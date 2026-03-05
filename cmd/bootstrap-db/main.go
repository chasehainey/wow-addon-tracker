// bootstrap-db builds assets/addon-db.json by merging four sources:
//
//  1. The community addon catalogue CSV (layday/github-wow-addon-catalogue)
//  2. All repos tagged with the `world-of-warcraft` GitHub topic
//  3. CurseForge top-200 addons by downloads (requires CURSEFORGE_API_KEY)
//  4. WoWInterface top-200 addons by downloads (no auth required)
//
// Sources 3 and 4 contribute only entries where the addon page links to a
// GitHub repository — any addon that doesn't publish its source on GitHub is
// silently skipped.
//
// Usage (from repo root):
//
//	go run ./cmd/bootstrap-db
//
// Requires a GitHub token in GITHUB_TOKEN env var or saved in the app config
// at ~/.config/wow-addon-tracker/config.json.
// Optional: set CURSEFORGE_API_KEY to include CurseForge results.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	catalogueURL  = "https://raw.githubusercontent.com/layday/github-wow-addon-catalogue/refs/heads/main/addons.csv"
	apiBase       = "https://api.github.com"
	enrichWorkers = 20
)

type AddonDBEntry struct {
	Repo        string   `json:"repo"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Stars       int      `json:"stars"`
	Language    string   `json:"language"`
	Topics      []string `json:"topics"`
	UpdatedAt   string   `json:"updated_at"`
}

type appConfig struct {
	GithubToken string `json:"github_token"`
}

type ghSearchResponse struct {
	TotalCount int            `json:"total_count"`
	Items      []ghSearchItem `json:"items"`
}

type ghSearchItem struct {
	FullName        string   `json:"full_name"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	StargazersCount int      `json:"stargazers_count"`
	Language        string   `json:"language"`
	Topics          []string `json:"topics"`
	UpdatedAt       string   `json:"updated_at"`
}

type ghRepoResp struct {
	StargazersCount int    `json:"stargazers_count"`
	Language        string `json:"language"`
}

// ── CurseForge types ─────────────────────────────────────────────────────────

type cfResponse struct {
	Data []cfMod `json:"data"`
}

type cfMod struct {
	Name  string  `json:"name"`
	Links cfLinks `json:"links"`
}

type cfLinks struct {
	SourceUrl  string `json:"sourceUrl"`
	WebsiteUrl string `json:"websiteUrl"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func readToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "wow-addon-tracker", "config.json"))
	if err != nil {
		return ""
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.GithubToken
}

func apiGet(url, token string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "wow-addon-tracker-bootstrap/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// extractGitHubRepo extracts "Owner/Repo" from any string containing a
// github.com URL.  Returns "" if no valid repo path is found.
func extractGitHubRepo(s string) string {
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	idx := strings.Index(lower, "github.com/")
	if idx < 0 {
		return ""
	}
	rest := s[idx+len("github.com/"):]
	// Split into owner / repo / optional-extra
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return ""
	}
	owner := cleanSegment(parts[0])
	repo := strings.TrimSuffix(cleanSegment(parts[1]), ".git")
	if owner == "" || repo == "" {
		return ""
	}
	// Skip obviously non-repo paths like github.com/orgs/... or github.com/search
	if strings.EqualFold(owner, "orgs") || strings.EqualFold(owner, "search") ||
		strings.EqualFold(owner, "topics") || strings.EqualFold(owner, "explore") {
		return ""
	}
	return owner + "/" + repo
}

func cleanSegment(s string) string {
	for _, ch := range []string{"?", "#", " ", "\t", "\n", "\r", "\"", "'"} {
		if i := strings.Index(s, ch); i >= 0 {
			s = s[:i]
		}
	}
	return s
}

// ── Sources ───────────────────────────────────────────────────────────────────

// fetchCatalogue downloads and parses the community catalogue CSV.
func fetchCatalogue() ([]AddonDBEntry, error) {
	fmt.Println("→ Fetching community catalogue CSV...")
	resp, err := http.Get(catalogueURL)
	if err != nil {
		return nil, fmt.Errorf("fetch catalogue: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("catalogue HTTP %d", resp.StatusCode)
	}
	r := csv.NewReader(resp.Body)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	var out []AddonDBEntry
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 6 {
			continue
		}
		fullName := strings.TrimSpace(row[2])
		if fullName == "" || !strings.Contains(fullName, "/") {
			continue
		}
		out = append(out, AddonDBEntry{
			Repo:        fullName,
			Name:        row[1],
			Description: row[4],
			UpdatedAt:   row[5],
		})
	}
	fmt.Printf("   %d entries\n", len(out))
	return out, nil
}

// fetchTopicRepos scrapes all repos for a GitHub topic via the Search API.
func fetchTopicRepos(topic, token string) ([]AddonDBEntry, error) {
	fmt.Printf("→ Scraping topic:%s via GitHub Search API...\n", topic)
	var all []AddonDBEntry
	seen := make(map[string]bool)

	for page := 1; page <= 10; page++ {
		url := fmt.Sprintf("%s/search/repositories?q=topic:%s&sort=stars&order=desc&per_page=100&page=%d",
			apiBase, topic, page)
		body, status, err := apiGet(url, token)
		if err != nil {
			return nil, fmt.Errorf("search API (page %d): %w", page, err)
		}
		if status == 422 {
			break // past GitHub's 1000-result window
		}
		if status != 200 {
			return nil, fmt.Errorf("search API HTTP %d: %s", status, string(body))
		}
		var result ghSearchResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parse search response: %w", err)
		}
		if len(result.Items) == 0 {
			break
		}
		for _, item := range result.Items {
			if seen[item.FullName] {
				continue
			}
			seen[item.FullName] = true
			all = append(all, AddonDBEntry{
				Repo:        item.FullName,
				Name:        item.Name,
				Description: item.Description,
				Stars:       item.StargazersCount,
				Language:    item.Language,
				Topics:      item.Topics,
				UpdatedAt:   item.UpdatedAt,
			})
		}
		fmt.Printf("   page %d: %d results (total so far: %d)\n", page, len(result.Items), len(all))
		if len(result.Items) < 100 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	return all, nil
}

// fetchCurseForge queries the CurseForge API for the top-200 WoW addons by
// download count and returns entries for those that include a GitHub link in
// their sourceUrl or websiteUrl fields.
//
// Requires a CurseForge API key (free tier from console.curseforge.com).
func fetchCurseForge(apiKey string) ([]AddonDBEntry, error) {
	fmt.Println("→ Fetching CurseForge top-200 addons...")
	var out []AddonDBEntry
	seen := make(map[string]bool)

	// gameId=1 (World of Warcraft), classId=6 (AddOns), sortField=6 (TotalDownloads)
	for page := 0; page < 4; page++ {
		index := page * 50
		url := fmt.Sprintf(
			"https://api.curseforge.com/v1/mods/search?gameId=1&classId=6&sortField=6&sortOrder=desc&pageSize=50&index=%d",
			index)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "wow-addon-tracker-bootstrap/1.0")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("CurseForge page %d: %w", page+1, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("CurseForge API HTTP %d: %s", resp.StatusCode, string(body))
		}

		var result cfResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("CurseForge parse page %d: %w", page+1, err)
		}

		found := 0
		for _, mod := range result.Data {
			gh := extractGitHubRepo(mod.Links.SourceUrl)
			if gh == "" {
				gh = extractGitHubRepo(mod.Links.WebsiteUrl)
			}
			if gh == "" || seen[gh] {
				continue
			}
			seen[gh] = true
			out = append(out, AddonDBEntry{Repo: gh, Name: mod.Name})
			found++
		}
		fmt.Printf("   page %d: %d mods, %d with GitHub links (running total: %d)\n",
			page+1, len(result.Data), found, len(out))
		if len(result.Data) < 50 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return out, nil
}

// fetchWoWInterface fetches the top-200 WoWInterface addons by download count
// and returns entries for those whose detail page includes a GitHub link.
// No authentication required.
func fetchWoWInterface() ([]AddonDBEntry, error) {
	fmt.Println("→ Fetching WoWInterface top-200 addons...")

	resp, err := http.Get("https://api.mmoui.com/v4/game/WOW/filelist.json")
	if err != nil {
		return nil, fmt.Errorf("WoWInterface filelist: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("WoWInterface filelist HTTP %d", resp.StatusCode)
	}

	var files []map[string]interface{}
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("WoWInterface filelist parse: %w", err)
	}

	// Sort by download count descending, take top 200.
	sort.Slice(files, func(i, j int) bool {
		return wowiDownloads(files[i]) > wowiDownloads(files[j])
	})
	if len(files) > 200 {
		files = files[:200]
	}
	fmt.Printf("   %d total addons in DB, scanning top 200 for GitHub links...\n", len(files))

	var out []AddonDBEntry
	seen := make(map[string]bool)

	for i, f := range files {
		uid := wowiUID(f)
		if uid == "" {
			continue
		}
		dr, err := http.Get(fmt.Sprintf("https://api.mmoui.com/v4/game/WOW/filedetails/%s.json", uid))
		if err != nil {
			continue
		}
		detailBody, _ := io.ReadAll(dr.Body)
		dr.Body.Close()

		// Scan the entire JSON blob for any github.com URL.
		gh := extractGitHubRepo(string(detailBody))
		if gh == "" || seen[gh] {
			continue
		}
		seen[gh] = true
		name, _ := f["UIName"].(string)
		out = append(out, AddonDBEntry{Repo: gh, Name: name})

		if (i+1)%50 == 0 {
			fmt.Printf("   %d / 200 checked (%d GitHub links found)\n", i+1, len(out))
		}
		time.Sleep(50 * time.Millisecond) // polite pacing
	}
	fmt.Printf("   Found %d addons with GitHub links.\n", len(out))
	return out, nil
}

func wowiDownloads(e map[string]interface{}) int64 {
	for _, key := range []string{"UIDownloadTotal", "UIDownloads", "UIHitCount"} {
		if v, ok := e[key]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case string:
				n, _ := strconv.ParseInt(x, 10, 64)
				return n
			}
		}
	}
	return 0
}

func wowiUID(e map[string]interface{}) string {
	if v, ok := e["UID"]; ok {
		switch x := v.(type) {
		case float64:
			return strconv.FormatInt(int64(x), 10)
		case string:
			return x
		}
	}
	return ""
}

// enrichStars fetches star count and language for entries that don't have them yet.
func enrichStars(entries []AddonDBEntry, token string) []AddonDBEntry {
	var needEnrich []int
	for i, e := range entries {
		if e.Stars == 0 {
			needEnrich = append(needEnrich, i)
		}
	}
	if len(needEnrich) == 0 {
		fmt.Println("→ All entries already have star counts, skipping enrichment.")
		return entries
	}
	fmt.Printf("→ Enriching %d entries with star counts (%d workers)...\n",
		len(needEnrich), enrichWorkers)

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, enrichWorkers)
		done    atomic.Int32
		errored atomic.Int32
	)

	for _, idx := range needEnrich {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			body, status, err := apiGet(fmt.Sprintf("%s/repos/%s", apiBase, entries[i].Repo), token)
			n := done.Add(1)
			if err != nil || status != 200 {
				errored.Add(1)
			} else {
				var r ghRepoResp
				if err := json.Unmarshal(body, &r); err == nil {
					mu.Lock()
					entries[i].Stars = r.StargazersCount
					entries[i].Language = r.Language
					mu.Unlock()
				}
			}
			if n%100 == 0 || int(n) == len(needEnrich) {
				fmt.Printf("   %d / %d  (%d errors)\n", n, len(needEnrich), errored.Load())
			}
		}(idx)
	}
	wg.Wait()
	fmt.Printf("   Done. %d errors.\n", errored.Load())
	return entries
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	token := readToken()
	if token == "" {
		fmt.Fprintln(os.Stderr, "No GitHub token found.")
		fmt.Fprintln(os.Stderr, "Set GITHUB_TOKEN=ghp_xxx or add a token via the app's Settings → GitHub Token.")
		os.Exit(1)
	}
	fmt.Println("GitHub token: found")

	cfKey := os.Getenv("CURSEFORGE_API_KEY")
	if cfKey != "" {
		fmt.Println("CurseForge key: found")
	} else {
		fmt.Println("CurseForge key: not set (set CURSEFORGE_API_KEY to include CurseForge results)")
	}
	fmt.Println()

	seen := make(map[string]int) // repo → index in merged
	merged := make([]AddonDBEntry, 0, 3000)

	addSource := func(entries []AddonDBEntry, label string) {
		added := 0
		for _, e := range entries {
			if _, exists := seen[e.Repo]; !exists {
				seen[e.Repo] = len(merged)
				merged = append(merged, e)
				added++
			}
		}
		fmt.Printf("   +%d new  (%d total)\n", added, len(merged))
		_ = label
	}

	// 1. Community catalogue CSV
	catalogueEntries, err := fetchCatalogue()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 2. GitHub topic search (stars included in results — add first so they
	//    take precedence over catalogue entries for the same repo)
	fmt.Println()
	topicEntries, err := fetchTopicRepos("world-of-warcraft", token)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 3. CurseForge (optional)
	var cfEntries []AddonDBEntry
	if cfKey != "" {
		fmt.Println()
		cfEntries, err = fetchCurseForge(cfKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "CurseForge scrape error (continuing): %v\n", err)
		}
	}

	// 4. WoWInterface
	fmt.Println()
	wowiEntries, err := fetchWoWInterface()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WoWInterface scrape error (continuing): %v\n", err)
	}

	// Merge — topic search first (richest metadata), then the rest
	fmt.Println()
	fmt.Println("→ Merging sources...")
	fmt.Print("   GitHub topics:        ")
	addSource(topicEntries, "topics")
	fmt.Print("   Community catalogue:  ")
	addSource(catalogueEntries, "catalogue")
	if cfKey != "" {
		fmt.Print("   CurseForge:           ")
		addSource(cfEntries, "curseforge")
	}
	fmt.Print("   WoWInterface:         ")
	addSource(wowiEntries, "wowinterface")

	// 5. Enrich entries that don't have star counts yet
	fmt.Println()
	merged = enrichStars(merged, token)

	// 6. Sort by stars descending
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Stars > merged[j].Stars
	})

	// 7. Write output
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal failed: %v\n", err)
		os.Exit(1)
	}

	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	outPath := filepath.Join(repoRoot, "assets", "addon-db.json")

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Wrote %d entries → %s\n", len(merged), outPath)
	fmt.Println("\nTop 15 by stars:")
	for i := 0; i < 15 && i < len(merged); i++ {
		fmt.Printf("  %2d.  %-52s  %d ★\n", i+1, merged[i].Repo, merged[i].Stars)
	}
	fmt.Println("\nRun `go build ./...` to embed the updated database into the binary.")
}
