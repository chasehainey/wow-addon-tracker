package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

const githubAPIBase = "https://api.github.com"

func githubRequest(url, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "wow-addon-tracker/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

// fetchLatestReleaseSync fetches both the best formal release and the best
// tag-based release, then returns whichever is more recent.  This correctly
// handles repos like Details-Damage-Meter that have old formal releases but
// now publish all updates as tags — the tag always wins when it is newer.
func fetchLatestReleaseSync(repo, token string) (GitHubRelease, error) {
	formal, formalErr := fetchBestFormalRelease(repo, token)
	tag, tagErr := fetchLatestTagRelease(repo, token)

	if formalErr != nil && tagErr != nil {
		// No releases and no tags — fall back to the default branch zipball.
		// Pin the download to the specific commit SHA so we don't always pull
		// HEAD, and use the commit date as the displayed version string.
		sha, date := fetchLatestCommit(repo, token)
		tagName := date
		if tagName == "" {
			tagName = "HEAD"
		}
		zipURL := fmt.Sprintf("%s/repos/%s/zipball", githubAPIBase, repo)
		if sha != "" {
			zipURL = fmt.Sprintf("%s/repos/%s/zipball/%s", githubAPIBase, repo, sha)
		}
		return GitHubRelease{TagName: tagName, ZipballURL: zipURL}, nil
	}
	if formalErr != nil {
		return tag, nil
	}
	if tagErr != nil {
		return formal, nil
	}
	// Both found — compare using dates embedded in the tag names, with a
	// careful fallback to PublishedAt when only one side has a dated tag.
	//
	// Repos like Details-Damage-Meter use date-stamped tags ("Details.20260304…")
	// as their canonical versioning while also publishing formal "Release_N"
	// releases.  The formal release's PublishedAt may be one day after the tag
	// date (packaging delay), but the date-stamped tag IS the right version.
	// Conversely, repos that switched FROM date tags TO semver should use the
	// semver formal release, which will have a PublishedAt years after the old
	// date tags.
	//
	// Decision rules (tagTagDate / formalTagDate are YYYYMMDD from tag names):
	//  1. Both have date tags  → compare YYYYMMDD; newer wins.
	//  2. Only tag has date    → if formal.PublishedAt is from the same year or
	//                            an earlier year, the date-tagged style is canonical:
	//                            prefer tag.  If formal is from a later year, the
	//                            repo switched versioning → prefer formal.
	//  3. Only formal has date → mirror of case 2.
	//  4. Neither has date     → fall back to compareTagVersions.
	tagTagDate := extractTagDate(tag.TagName)
	formalTagDate := extractTagDate(formal.TagName)

	switch {
	case tagTagDate != "" && formalTagDate != "":
		// Both date-stamped: the later date wins.
		if tagTagDate >= formalTagDate {
			return tag, nil
		}
		return formal, nil

	case tagTagDate != "" && formalTagDate == "":
		// Tag is date-stamped; formal is not (e.g. "Release_1234", "v3.0.0").
		// If the formal's publication year is strictly later than the tag's year,
		// the repo has switched away from date-stamped versioning → prefer formal.
		// Otherwise the date-stamped tag is the canonical version → prefer tag.
		formalPubYear := ""
		if len(formal.PublishedAt) >= 4 {
			formalPubYear = formal.PublishedAt[:4]
		}
		if formalPubYear != "" && formalPubYear > tagTagDate[:4] {
			return formal, nil
		}
		return tag, nil

	case tagTagDate == "" && formalTagDate != "":
		// Formal is date-stamped; tag is not.  Mirror of the case above.
		tagPubYear := ""
		if len(tag.PublishedAt) >= 4 {
			tagPubYear = tag.PublishedAt[:4]
		}
		if tagPubYear != "" && tagPubYear > formalTagDate[:4] {
			return tag, nil
		}
		return formal, nil

	default:
		// Neither has a date tag — fall back to tag name version comparison.
		if tagIsNewer(tag.TagName, formal.TagName) {
			return tag, nil
		}
		return formal, nil
	}
}

// fetchBestFormalRelease returns the best formal GitHub release (via
// /releases/latest, cross-checked against /releases?per_page=1).
// Returns an error when the repo has no formal releases at all.
func fetchBestFormalRelease(repo, token string) (GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, repo)
	resp, err := githubRequest(url, token)
	if err != nil {
		return GitHubRelease{}, fmt.Errorf("request failed: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 200 {
		var latest GitHubRelease
		if err := json.Unmarshal(body, &latest); err == nil {
			// The "latest" marker may lag; compare with the most-recently
			// published release and use whichever is newer — but never let
			// a pre-release displace the stable "latest".
			if recent, err := fetchMostRecentRelease(repo, token); err == nil {
				if !recent.Prerelease && recent.PublishedAt > latest.PublishedAt {
					return recent, nil
				}
			}
			return latest, nil
		}
	}
	// 404 or unparseable — fall back to the releases list.
	return fetchMostRecentRelease(repo, token)
}

// fetchMostRecentRelease fetches /releases?per_page=1 and returns the most
// recently published release.  Returns an error if the list is empty.
func fetchMostRecentRelease(repo, token string) (GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=1", githubAPIBase, repo)
	resp, err := githubRequest(url, token)
	if err != nil {
		return GitHubRelease{}, fmt.Errorf("request failed: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return GitHubRelease{}, fmt.Errorf("repo %q not found or has no releases", repo)
	}
	var releases []GitHubRelease
	if err := json.Unmarshal(body, &releases); err != nil || len(releases) == 0 {
		return GitHubRelease{}, fmt.Errorf("no formal releases")
	}
	return releases[0], nil
}

// releaseDate returns a YYYY-MM-DD comparable string for a release.
// Uses PublishedAt when present; otherwise extracts a YYYYMMDD date embedded
// in the tag name (e.g. "Details.20260304.14718.170" → "2026-03-04").
func releaseDate(r GitHubRelease) string {
	if len(r.PublishedAt) >= 10 {
		return r.PublishedAt[:10]
	}
	for _, seg := range splitTagSegments(r.TagName) {
		n, ok := parseTagUint(seg)
		if ok && len(seg) == 8 && isLikelyDate(n) {
			return seg[:4] + "-" + seg[4:6] + "-" + seg[6:8]
		}
	}
	return ""
}

// ghTag is the GitHub /repos/{repo}/tags response element.
type ghTag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
}

// fetchLatestTagRelease finds the newest tag for a repo and returns a
// GitHubRelease for it. It first tries to find a formal release attached to
// that tag (which may have real addon-ZIP assets). If none exists it
// synthesises a release whose ZipballURL points at the tag's source archive.
//
// The GitHub tags API does not guarantee sort order — for repos like
// Details-Damage-Meter that switched from semver tags ("DetailsRetail.v9…")
// to date-stamped tags ("Details.20260304…"), the old semver tags sort
// alphabetically after the new date-stamped ones and may fill the first
// several pages entirely.  We paginate up to maxTagPages pages so that the
// date-stamped tags are always reachable.
func fetchLatestTagRelease(repo, token string) (GitHubRelease, error) {
	const maxTagPages = 50
	var allTags []ghTag
	foundDateTag := false

	for page := 1; page <= maxTagPages; page++ {
		url := fmt.Sprintf("%s/repos/%s/tags?per_page=100&page=%d", githubAPIBase, repo, page)
		resp, err := githubRequest(url, token)
		if err != nil {
			if len(allTags) == 0 {
				return GitHubRelease{}, fmt.Errorf("tags request failed: %w", err)
			}
			break
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != 200 {
			if len(allTags) == 0 {
				return GitHubRelease{}, fmt.Errorf("tags API returned %d", resp.StatusCode)
			}
			break
		}
		var pageTags []ghTag
		if err := json.Unmarshal(body, &pageTags); err != nil || len(pageTags) == 0 {
			break
		}
		allTags = append(allTags, pageTags...)

		pageHasDateTag := false
		for _, t := range pageTags {
			if extractTagDate(t.Name) != "" {
				pageHasDateTag = true
				foundDateTag = true
				break
			}
		}
		// Once we've found date-stamped tags and this page has none, we've
		// passed the date-stamped zone in the API's sort order — stop early.
		if foundDateTag && !pageHasDateTag {
			break
		}

		if len(pageTags) < 100 {
			break // last page — no more tags
		}
	}

	if len(allTags) == 0 {
		return GitHubRelease{}, fmt.Errorf("no tags found")
	}

	best := pickNewestTag(allTags)

	// Try to get a formal release for this tag — it may carry actual addon ZIPs.
	releaseURL := fmt.Sprintf("%s/repos/%s/releases/tags/%s", githubAPIBase, repo, best.Name)
	rResp, err := githubRequest(releaseURL, token)
	if err == nil {
		rBody, rErr := io.ReadAll(rResp.Body)
		rResp.Body.Close()
		if rErr == nil && rResp.StatusCode == 200 {
			var rel GitHubRelease
			if json.Unmarshal(rBody, &rel) == nil && rel.TagName != "" {
				return rel, nil
			}
		}
	}

	// No formal release — synthesise one from the tag's source zipball.
	// install.go will set stripTopLevel=true and find addon folders inside.
	return GitHubRelease{
		TagName:    best.Name,
		Name:       best.Name,
		ZipballURL: best.ZipballURL,
	}, nil
}

// pickNewestTag returns the tag with the highest version from the slice.
//
// Strategy:
//  1. Tags that contain a YYYYMMDD date segment beat tags that don't — repos
//     like Details-Damage-Meter switched from semver to date-stamped tags, so
//     the date-stamped ones are always newer than the old semver ones.
//  2. Among date-stamped tags, the later date wins.
//  3. Among non-date tags, fall back to compareTagVersions (numeric-segment sort).
func pickNewestTag(tags []ghTag) ghTag {
	best := tags[0]
	for _, t := range tags[1:] {
		if tagIsNewer(t.Name, best.Name) {
			best = t
		}
	}
	return best
}

// tagIsNewer reports whether tag name a is newer than b.
func tagIsNewer(a, b string) bool {
	dateA := extractTagDate(a)
	dateB := extractTagDate(b)
	if dateA != "" && dateB != "" {
		// Both date-stamped: later date wins.
		return dateA > dateB
	}
	if dateA != "" {
		// Only a has a date — it's the newer style.
		return true
	}
	if dateB != "" {
		// Only b has a date — b is the newer style.
		return false
	}
	// Neither has a date: compare by numeric version segments.
	return compareTagVersions(a, b) > 0
}

// extractTagDate returns the first YYYYMMDD segment found in a tag name,
// or "" if none is present.
func extractTagDate(name string) string {
	for _, seg := range splitTagSegments(name) {
		n, ok := parseTagUint(seg)
		if ok && len(seg) == 8 && isLikelyDate(n) {
			return seg
		}
	}
	return ""
}

// tagNumericSeq extracts the numeric components of a tag name, treating
// v/V-prefixed segments (e.g. "v9") as their numeric value and skipping
// purely alphabetic segments (e.g. "DetailsRetail").
// "v9.1.0.8812.145"          → [9, 1, 0, 8812, 145]
// "DetailsRetail.v9.1.0.8888.145" → [9, 1, 0, 8888, 145]
func tagNumericSeq(tag string) []uint64 {
	var seq []uint64
	for _, seg := range splitTagSegments(tag) {
		if n, ok := parseTagUint(seg); ok {
			seq = append(seq, n)
			continue
		}
		// Strip a leading v/V and try again.
		if len(seg) > 1 && (seg[0] == 'v' || seg[0] == 'V') {
			if n, ok := parseTagUint(seg[1:]); ok {
				seq = append(seq, n)
			}
		}
		// Skip purely alphabetic segments like "DetailsRetail", "alpha".
	}
	return seq
}

// compareTagVersions returns >0 if a is newer than b, <0 if older, 0 if equal.
// Only numeric segments are compared; alphabetic-only segments (e.g.
// "DetailsRetail") are skipped so that prefix differences don't affect the
// result.
func compareTagVersions(a, b string) int {
	seqA := tagNumericSeq(a)
	seqB := tagNumericSeq(b)
	n := len(seqA)
	if len(seqB) < n {
		n = len(seqB)
	}
	for i := 0; i < n; i++ {
		if seqA[i] != seqB[i] {
			if seqA[i] > seqB[i] {
				return 1
			}
			return -1
		}
	}
	return len(seqA) - len(seqB)
}

// splitTagSegments splits a tag name on any non-alphanumeric character.
// e.g. "Details.20260304.14718.170" → ["Details","20260304","14718","170"]
//
//	"v2.5.3-release"             → ["v2","5","3","release"]  (v stays)
func splitTagSegments(tag string) []string {
	var segs []string
	var cur strings.Builder
	for _, ch := range tag {
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			cur.WriteRune(ch)
		} else if cur.Len() > 0 {
			segs = append(segs, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		segs = append(segs, cur.String())
	}
	return segs
}

// parseTagUint parses an all-digit string as uint64. Returns false for anything
// that contains non-digit characters (e.g. "Details", "v2", "alpha").
func parseTagUint(s string) (uint64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	var n uint64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + uint64(ch-'0')
	}
	return n, true
}

// fetchChangelogVersion tries common changelog filenames via the GitHub
// Contents API and returns the first version number found inside the file.
// Returns "" if nothing useful is found.
func fetchChangelogVersion(repo, token string) string {
	candidates := []string{
		"CHANGELOG.md", "Changelog.md", "changelog.md",
		"CHANGES.md", "HISTORY.md", "RELEASES.md",
	}
	for _, name := range candidates {
		url := fmt.Sprintf("%s/repos/%s/contents/%s", githubAPIBase, repo, name)
		resp, err := githubRequest(url, token)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			continue
		}

		var file struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if err := json.Unmarshal(body, &file); err != nil {
			continue
		}

		var content string
		if file.Encoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(
				strings.ReplaceAll(file.Content, "\n", ""),
			)
			if err != nil {
				continue
			}
			content = string(decoded)
		} else {
			content = file.Content
		}

		if v := extractVersionFromChangelog(content); v != "" {
			return v
		}
	}
	return ""
}

// fetchLatestCommit returns the SHA and committer date ("YYYY-MM-DD") of the
// most recent commit on the default branch.  Used as a last resort for repos
// that have no releases and no tags.  Both values are "" on any failure.
func fetchLatestCommit(repo, token string) (sha, date string) {
	url := fmt.Sprintf("%s/repos/%s/commits?per_page=1", githubAPIBase, repo)
	resp, err := githubRequest(url, token)
	if err != nil {
		return "", ""
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 {
		return "", ""
	}
	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Committer struct {
				Date string `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &commits); err != nil || len(commits) == 0 {
		return "", ""
	}
	sha = commits[0].SHA
	d := commits[0].Commit.Committer.Date
	if len(d) >= 10 {
		date = d[:10] // YYYY-MM-DD
	} else {
		date = d
	}
	return sha, date
}

func fetchLatestRelease(repo, token string) tea.Cmd {
	return func() tea.Msg {
		release, err := fetchLatestReleaseSync(repo, token)
		return releaseCheckMsg{repo: repo, release: release, err: err}
	}
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

func checkAllAddons(addons []TrackedAddon, db []AddonDBEntry, token string) tea.Cmd {
	return func() tea.Msg {
		if len(addons) == 0 {
			return batchCheckCompleteMsg{results: []AddonWithStatus{}}
		}

		// Build a WoWInterface ID → DB entry lookup for fast version checks.
		wowiMap := make(map[int]AddonDBEntry, len(db))
		for _, e := range db {
			if e.WoWInterfaceID > 0 {
				wowiMap[e.WoWInterfaceID] = e
			}
		}

		type result struct {
			idx     int
			release GitHubRelease
			err     error
		}

		results := make([]result, len(addons))
		sem := make(chan struct{}, 5) // max 5 concurrent requests
		var wg sync.WaitGroup

		for i, addon := range addons {
			wg.Add(1)
			go func(idx int, a TrackedAddon) {
				defer wg.Done()
				// WoWInterface addons: look up latest version from the DB
				// (already fetched at startup) — no GitHub API call needed.
				if id := wowiIDFromKey(a.GithubRepo); id > 0 {
					if e, ok := wowiMap[id]; ok && e.LatestVersion != "" {
						results[idx] = result{idx: idx, release: GitHubRelease{
							TagName:     e.LatestVersion,
							PublishedAt: e.LatestDate,
						}}
					}
					return
				}
				sem <- struct{}{}
				defer func() { <-sem }()
				release, err := fetchLatestReleaseSync(a.GithubRepo, token)
				results[idx] = result{idx: idx, release: release, err: err}
			}(i, addon)
		}
		wg.Wait()

		out := make([]AddonWithStatus, len(addons))
		for i, r := range results {
			addon := addons[i]
			aws := AddonWithStatus{
				Addon: addon,
			}
			if r.err != nil {
				aws.Status = StatusUnknown
			} else {
				latestTag := r.release.TagName
				// Only update persisted version data when we got a real tag.
				// "HEAD" means the repo has no releases/tags; don't overwrite
				// a previously-known good version with it.
				if latestTag != "" && latestTag != "HEAD" {
					aws.LatestVersion = latestTag
					aws.LatestDate = r.release.PublishedAt
					aws.Addon.LatestVersion = latestTag
					aws.Addon.LatestDate = r.release.PublishedAt
				}
				// Compare against the best version we know: freshly fetched,
				// or the value persisted from a previous successful check.
				effectiveLatest := aws.LatestVersion
				if effectiveLatest == "" {
					effectiveLatest = addon.LatestVersion
				}
				if effectiveLatest == "" || effectiveLatest == "HEAD" {
					aws.Status = StatusUnknown
				} else if addon.InstalledVersion == "" {
					aws.Status = StatusNotInstalled
				} else if normalizeVersion(addon.InstalledVersion) == normalizeVersion(effectiveLatest) {
					aws.Status = StatusUpToDate
				} else {
					aws.Status = StatusUpdateAvail
				}
			}
			out[i] = aws
		}

		return batchCheckCompleteMsg{results: out}
	}
}
