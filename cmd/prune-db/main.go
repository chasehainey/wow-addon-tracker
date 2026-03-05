// prune-db audits every repo in assets/addon-db.json against the GitHub API
// and removes entries that haven't had a code push in over a year, have been
// archived/disabled, or no longer exist.
//
// Two files are written:
//
//	reports/stale-YYYY-MM-DD.csv   — every removed entry with the reason
//	assets/addon-db.json           — overwritten with only the healthy entries
//
// Usage (from repo root):
//
//	go run ./cmd/prune-db
//
// Requires GITHUB_TOKEN env var or a token in the app config.
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
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	workers    = 20
	apiBase    = "https://api.github.com"
	staleCutoff = 365 * 24 * time.Hour
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

type ghRepoDetail struct {
	PushedAt        string `json:"pushed_at"`
	StargazersCount int    `json:"stargazers_count"`
	Language        string `json:"language"`
	Archived        bool   `json:"archived"`
	Disabled        bool   `json:"disabled"`
}

type result struct {
	entry    AddonDBEntry
	detail   ghRepoDetail
	reason   string // "" = keep, "stale", "archived", "not_found", "error"
	lastPush string // YYYY-MM-DD
}

func readToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", "wow-addon-tracker", "config.json"))
	if err != nil {
		return ""
	}
	var cfg appConfig
	json.Unmarshal(data, &cfg)
	return cfg.GithubToken
}

func apiGet(url, token string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "wow-addon-tracker-prune/1.0")
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

func check(entry AddonDBEntry, token string, cutoff time.Time) result {
	body, status, err := apiGet(fmt.Sprintf("%s/repos/%s", apiBase, entry.Repo), token)

	r := result{entry: entry}

	if err != nil {
		r.reason = "error"
		return r
	}
	if status == 404 || status == 451 {
		r.reason = "not_found"
		return r
	}
	if status != 200 {
		r.reason = "error"
		return r
	}

	var d ghRepoDetail
	if err := json.Unmarshal(body, &d); err != nil {
		r.reason = "error"
		return r
	}
	r.detail = d

	// Update star / language in case they've changed.
	r.entry.Stars = d.StargazersCount
	r.entry.Language = d.Language

	if d.Archived || d.Disabled {
		r.reason = "archived"
		if len(d.PushedAt) >= 10 {
			r.lastPush = d.PushedAt[:10]
		}
		return r
	}

	if d.PushedAt == "" {
		// No push date — treat as stale.
		r.reason = "stale"
		return r
	}

	pushed, err := time.Parse(time.RFC3339, d.PushedAt)
	if err != nil {
		// Try date-only format.
		pushed, err = time.Parse("2006-01-02", d.PushedAt[:10])
		if err != nil {
			r.reason = "stale"
			return r
		}
	}
	if len(d.PushedAt) >= 10 {
		r.lastPush = d.PushedAt[:10]
	}

	if pushed.Before(cutoff) {
		r.reason = "stale"
	}
	return r
}

func main() {
	token := readToken()
	if token == "" {
		fmt.Fprintln(os.Stderr, "No GitHub token found.")
		fmt.Fprintln(os.Stderr, "Set GITHUB_TOKEN=ghp_xxx or add a token via the app's Settings → GitHub Token.")
		os.Exit(1)
	}

	// Locate files relative to this source file.
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	dbPath := filepath.Join(repoRoot, "assets", "addon-db.json")
	reportsDir := filepath.Join(repoRoot, "reports")

	// Load DB.
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read %s: %v\n", dbPath, err)
		os.Exit(1)
	}
	var db []AddonDBEntry
	if err := json.Unmarshal(raw, &db); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse DB: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d entries from %s\n\n", len(db), dbPath)

	cutoff := time.Now().Add(-staleCutoff)
	fmt.Printf("Stale cutoff: %s (no push since this date = removed)\n\n", cutoff.Format("2006-01-02"))

	// Check every repo concurrently.
	results := make([]result, len(db))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var done, staleCount, notFound, archived, errCount atomic.Int32

	fmt.Printf("Checking %d repos (%d workers)...\n", len(db), workers)

	for i, entry := range db {
		wg.Add(1)
		go func(idx int, e AddonDBEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = check(e, token, cutoff)
			n := done.Add(1)
			switch results[idx].reason {
			case "stale":
				staleCount.Add(1)
			case "not_found":
				notFound.Add(1)
			case "archived":
				archived.Add(1)
			case "error":
				errCount.Add(1)
			}
			if n%100 == 0 || int(n) == len(db) {
				fmt.Printf("  %d / %d  (stale: %d  not_found: %d  archived: %d  errors: %d)\n",
					n, len(db), staleCount.Load(), notFound.Load(), archived.Load(), errCount.Load())
			}
		}(i, entry)
	}
	wg.Wait()

	// Split into keep / remove.
	var keep, remove []result
	for _, r := range results {
		if r.reason == "" {
			keep = append(keep, r)
		} else {
			remove = append(remove, r)
		}
	}

	fmt.Printf("\n── Summary ────────────────────────────────────────\n")
	fmt.Printf("  Total checked : %d\n", len(db))
	fmt.Printf("  Keeping       : %d\n", len(keep))
	fmt.Printf("  Removing      : %d\n", len(remove))
	fmt.Printf("    stale       : %d\n", staleCount.Load())
	fmt.Printf("    not found   : %d\n", notFound.Load())
	fmt.Printf("    archived    : %d\n", archived.Load())
	fmt.Printf("    errors      : %d\n", errCount.Load())
	fmt.Println()

	if len(remove) == 0 {
		fmt.Println("Nothing to remove. DB unchanged.")
		return
	}

	// Sort removed entries: not_found first, then archived, then stale, each
	// group sorted by stars descending so the report is easy to review.
	reasonOrder := map[string]int{"not_found": 0, "archived": 1, "stale": 2, "error": 3}
	sort.Slice(remove, func(i, j int) bool {
		oi, oj := reasonOrder[remove[i].reason], reasonOrder[remove[j].reason]
		if oi != oj {
			return oi < oj
		}
		return remove[i].entry.Stars > remove[j].entry.Stars
	})

	// Write CSV report.
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create reports dir: %v\n", err)
		os.Exit(1)
	}
	csvPath := filepath.Join(reportsDir, fmt.Sprintf("stale-%s.csv", time.Now().Format("2006-01-02")))
	cf, err := os.Create(csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create CSV: %v\n", err)
		os.Exit(1)
	}
	w := csv.NewWriter(cf)
	w.Write([]string{"repo", "name", "stars", "language", "last_pushed", "reason"})
	for _, r := range remove {
		w.Write([]string{
			r.entry.Repo,
			r.entry.Name,
			fmt.Sprintf("%d", r.entry.Stars),
			r.entry.Language,
			r.lastPush,
			r.reason,
		})
	}
	w.Flush()
	cf.Close()
	if err := w.Error(); err != nil {
		fmt.Fprintf(os.Stderr, "CSV write error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("CSV report → %s\n", csvPath)

	// Overwrite DB with healthy entries only, preserving star sort order.
	fresh := make([]AddonDBEntry, 0, len(keep))
	for _, r := range keep {
		fresh = append(fresh, r.entry)
	}
	sort.Slice(fresh, func(i, j int) bool {
		return fresh[i].Stars > fresh[j].Stars
	})

	out, err := json.MarshalIndent(fresh, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(dbPath, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot write DB: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("DB updated   → %s  (%d entries)\n", dbPath, len(fresh))

	// Preview first few removed entries.
	fmt.Println("\nFirst 10 removed entries:")
	header := fmt.Sprintf("  %-45s  %-10s  %s", "repo", "reason", "last_pushed")
	fmt.Println(header)
	fmt.Println("  " + strings.Repeat("─", len(header)-2))
	for i := 0; i < 10 && i < len(remove); i++ {
		r := remove[i]
		fmt.Printf("  %-45s  %-10s  %s\n", r.entry.Repo, r.reason, r.lastPush)
	}
	if len(remove) > 10 {
		fmt.Printf("  ... and %d more (see CSV)\n", len(remove)-10)
	}
}
