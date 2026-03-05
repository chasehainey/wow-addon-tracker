package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func selectZipAsset(release GitHubRelease) (GitHubAsset, bool) {
	// Prefer non-nolib .zip
	for _, a := range release.Assets {
		lower := strings.ToLower(a.Name)
		if strings.HasSuffix(lower, ".zip") && !strings.Contains(lower, "nolib") {
			return a, true
		}
	}
	// Fallback: any .zip
	for _, a := range release.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
			return a, true
		}
	}
	return GitHubAsset{}, false
}

// changelogVersionRe matches the first version-like heading in a changelog.
// Handles formats:
//
//	## [1.2.3]  ## [v1.2.3]  ## v1.2.3  ## 1.2.3
//	## Version 1.2.3  ## Release 1.2.3  # 1.2.3.4
var changelogVersionRe = regexp.MustCompile(
	`(?im)^#+\s*(?:\[v?|v|version\s+|release\s+)?(\d+\.\d+[\d.]*)`,
)

func extractVersionFromChangelog(changelog string) string {
	if m := changelogVersionRe.FindStringSubmatch(changelog); m != nil {
		return m[1]
	}
	return ""
}

func isChangelogFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == "changelog.md" || lower == "changelog" ||
		lower == "changes.md" || lower == "changes" ||
		lower == "history.md" || lower == "releases.md"
}

func downloadToTemp(url, token string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "wow-addon-tracker/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "wow-addon-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}

	return tmp.Name(), nil
}

// zipStripPrefix returns the common top-level directory prefix produced by
// GitHub source archives (e.g. "owner-repo-sha123/"), or "" if none.
func zipStripPrefix(files []*zip.File) string {
	if len(files) == 0 {
		return ""
	}
	first := files[0].Name
	if idx := strings.Index(first, "/"); idx >= 0 {
		prefix := first[:idx+1]
		// Confirm every entry starts with this prefix (real source archive)
		for _, f := range files {
			if !strings.HasPrefix(f.Name, prefix) {
				return ""
			}
		}
		return prefix
	}
	return ""
}

// detectTocFolder scans a zip (after stripping any top-level prefix) and
// returns the base name of the first root-level .toc file found, or "".
// This is used to auto-name the target folder when all addon files are at
// the root of the archive rather than inside a subdirectory.
func detectTocFolder(r *zip.ReadCloser, stripPrefix string) string {
	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, stripPrefix)
		// Root-level file: no "/" and ends in .toc
		if !strings.Contains(name, "/") && strings.HasSuffix(strings.ToLower(name), ".toc") {
			return strings.TrimSuffix(name, filepath.Ext(name))
		}
	}
	return ""
}

// extractZip extracts the zip at zipPath into destDir and returns:
//   - the top-level directory names written
//   - any changelog text found
//   - the effective extractAs folder (may differ from the argument when auto-detected)
//   - an error
//
// Behaviour depends on the parameters:
//
//	extractAs != "":  All non-hidden content (including root files) is placed
//	                  under destDir/extractAs/.  The GitHub source-archive
//	                  top-level prefix is always stripped first.
//
//	stripTopLevel:    Strip the common prefix; then skip root-level files
//	                  (but read changelog ones) and hidden directories.
//
//	Neither:          Extract everything as-is (standard release zip).
func extractZip(zipPath, destDir string, stripTopLevel bool, extractAs string) (dirs []string, changelog string, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	// Determine prefix to strip.
	stripPrefix := ""
	if stripTopLevel || extractAs != "" {
		stripPrefix = zipStripPrefix(r.File)
	}

	topDirs := make(map[string]struct{})

	for _, f := range r.File {
		name := f.Name

		// Strip prefix when operating on source archives.
		if stripPrefix != "" {
			if !strings.HasPrefix(name, stripPrefix) {
				continue
			}
			name = strings.TrimPrefix(name, stripPrefix)
			if name == "" {
				continue
			}
		}

		// Determine the top-level component (directory or file at root).
		topComp := name
		if idx := strings.Index(name, "/"); idx != -1 {
			topComp = name[:idx]
		}

		// In source-archive modes skip anything starting with ".".
		if (stripTopLevel || extractAs != "") && strings.HasPrefix(topComp, ".") {
			continue
		}

		isRootLevel := !strings.Contains(name, "/") && !f.FileInfo().IsDir()

		if extractAs != "" {
			// ── ExtractAs mode ──────────────────────────────────────────────
			// Everything (root files included) goes into destDir/extractAs/.
			// Read changelog before redirecting path.
			if isRootLevel && isChangelogFile(name) && changelog == "" {
				if rc, e := f.Open(); e == nil {
					data, _ := io.ReadAll(rc)
					rc.Close()
					changelog = string(data)
				}
			}
			name = extractAs + "/" + name

		} else if stripTopLevel {
			// ── Zipball mode ─────────────────────────────────────────────────
			// Skip root-level files; read changelog among them.
			if isRootLevel {
				if isChangelogFile(name) && changelog == "" {
					if rc, e := f.Open(); e == nil {
						data, _ := io.ReadAll(rc)
						rc.Close()
						changelog = string(data)
					}
				}
				continue
			}
		}

		// Sanitize: reject path traversal.
		if strings.Contains(name, "..") {
			return nil, "", fmt.Errorf("unsafe path in zip: %s", f.Name)
		}

		// Track top-level directory names.
		parts := strings.SplitN(name, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			topDirs[parts[0]] = struct{}{}
		}

		destPath := filepath.Join(destDir, filepath.FromSlash(name))

		// Ensure destPath stays within destDir.
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return nil, "", fmt.Errorf("path escape: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return nil, "", fmt.Errorf("mkdir %s: %w", destPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return nil, "", fmt.Errorf("mkdir parent %s: %w", destPath, err)
		}

		rc, err := f.Open()
		if err != nil {
			return nil, "", fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return nil, "", fmt.Errorf("create %s: %w", destPath, err)
		}

		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return nil, "", fmt.Errorf("extract %s: %w", f.Name, copyErr)
		}
	}

	result := make([]string, 0, len(topDirs))
	for d := range topDirs {
		result = append(result, d)
	}
	return result, changelog, nil
}

func installAddon(repo string, release GitHubRelease, addonPath, token, extractAs string) tea.Cmd {
	return func() tea.Msg {
		var downloadURL string
		stripTopLevel := false

		if asset, ok := selectZipAsset(release); ok {
			downloadURL = asset.BrowserDownloadURL
		} else if release.ZipballURL != "" {
			downloadURL = release.ZipballURL
			stripTopLevel = true
		} else {
			return installCompleteMsg{repo: repo, err: fmt.Errorf("no downloadable assets for release %s", release.TagName)}
		}

		tmpPath, err := downloadToTemp(downloadURL, token)
		if err != nil {
			return installCompleteMsg{repo: repo, err: err}
		}
		defer os.Remove(tmpPath)

		// Auto-detect extractAs when not explicitly set:
		// if the zip contains root-level .toc files, the addon ships without
		// a wrapping folder and needs one created.
		if extractAs == "" {
			if r, e := zip.OpenReader(tmpPath); e == nil {
				prefix := zipStripPrefix(r.File)
				extractAs = detectTocFolder(r, prefix)
				r.Close()
			}
		}

		dirs, changelog, err := extractZip(tmpPath, addonPath, stripTopLevel, extractAs)
		if err != nil {
			return installCompleteMsg{repo: repo, err: err}
		}

		// For release-based installs use the GitHub release body as changelog
		// when nothing was found inside the zip.
		if changelog == "" && release.Body != "" {
			changelog = release.Body
		}

		// For repos with no releases the tag is "HEAD"; try to pull a real
		// version from the changelog text instead.
		version := release.TagName
		if version == "HEAD" && changelog != "" {
			if v := extractVersionFromChangelog(changelog); v != "" {
				version = v
			}
		}

		return installCompleteMsg{
			repo:        repo,
			version:     version,
			directories: dirs,
			changelog:   changelog,
			extractAs:   extractAs,
			err:         nil,
		}
	}
}

func deleteAddonFolders(addon TrackedAddon, addonPath string) tea.Cmd {
	return func() tea.Msg {
		for _, dir := range addon.Directories {
			fullPath := filepath.Join(addonPath, dir)
			if err := os.RemoveAll(fullPath); err != nil {
				return addonDeletedMsg{name: addon.Name, err: fmt.Errorf("remove %s: %w", dir, err)}
			}
		}
		return addonDeletedMsg{name: addon.Name}
	}
}
