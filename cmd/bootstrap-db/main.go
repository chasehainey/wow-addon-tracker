// bootstrap-db builds assets/addon-db.json by fetching all WoW addons from
// the public MMOUI / WoWInterface API.
//
// Strategy:
//  1. Fetch the full addon list (one request, ~7 500 addons).
//  2. Batch-fetch details in groups of 100 to get descriptions and exact
//     download URIs (~80 requests total, run concurrently).
//  3. Merge, strip BBCode markup from descriptions, sort by downloads, write.
//
// Usage (from repo root):
//
//	go run ./cmd/bootstrap-db
//
// Output: assets/addon-db.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	filelistURL = "https://api.mmoui.com/v4/game/WOW/filelist.json"
	detailsBase = "https://api.mmoui.com/v4/game/WOW/filedetails/"
	batchSize   = 100
	workers     = 15
)

// ── MMOUI API types ──────────────────────────────────────────────────────────

type filelistEntry struct {
	ID               int      `json:"id"`
	CategoryID       int      `json:"categoryId"`
	Version          string   `json:"version"`
	LastUpdate       int64    `json:"lastUpdate"` // Unix ms
	Title            string   `json:"title"`
	Author           string   `json:"author"`
	Downloads        int      `json:"downloads"`
	DownloadsMonthly int      `json:"downloadsMonthly"`
	Favorites        int      `json:"favorites"`
	GameVersions     []string `json:"gameVersions"`
	FileInfoURI      string   `json:"fileInfoUri"`
}

type detailEntry struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Author           string `json:"author"`
	Description      string `json:"description"`
	ChangeLog        string `json:"changeLog"`
	Version          string `json:"version"`
	LastUpdate       int64  `json:"lastUpdate"`
	DownloadURI      string `json:"downloadUri"`
	CategoryID       int    `json:"categoryId"`
	DownloadsMonthly int    `json:"downloadsMonthly"`
}

// ── Output type (must match app's types.go AddonDBEntry JSON tags) ───────────

type AddonDBEntry struct {
	WoWInterfaceID   int      `json:"wowi_id"`
	Name             string   `json:"name"`
	Author           string   `json:"author,omitempty"`
	Description      string   `json:"description,omitempty"`
	Changelog        string   `json:"changelog,omitempty"`
	LatestVersion    string   `json:"latest_version,omitempty"`
	LatestDate       string   `json:"latest_date,omitempty"`
	Downloads        int      `json:"downloads,omitempty"`
	DownloadsMonthly int      `json:"downloads_monthly,omitempty"`
	Favorites        int      `json:"favorites,omitempty"`
	CategoryID       int      `json:"category_id,omitempty"`
	DownloadURL      string   `json:"download_url,omitempty"`
	FileInfoURL      string   `json:"file_info_url,omitempty"`
	Compatibility    []string `json:"compatibility,omitempty"`
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// BBCode → Markdown conversion regexps (compiled once at startup).
var (
	reListBlock = regexp.MustCompile(`(?is)\[list(=\d+)?\](.*?)\[/list\]`)
	reListItem  = regexp.MustCompile(`(?i)\[\*\]`)
	reSizeTag   = regexp.MustCompile(`(?is)\[size=(\d+)\](.*?)\[/size\]`)
	reBoldTag   = regexp.MustCompile(`(?is)\[b\](.*?)\[/b\]`)
	reItalicTag = regexp.MustCompile(`(?is)\[i\](.*?)\[/i\]`)
	reUnderTag  = regexp.MustCompile(`(?is)\[u\](.*?)\[/u\]`)
	reStrikeTag = regexp.MustCompile(`(?is)\[s\](.*?)\[/s\]`)
	reCodeTag   = regexp.MustCompile(`(?is)\[code\](.*?)\[/code\]`)
	reURLTag1   = regexp.MustCompile(`(?is)\[url=([^\]]+)\](.*?)\[/url\]`)
	reURLTag2   = regexp.MustCompile(`(?is)\[url\](.*?)\[/url\]`)
	reIMGTag    = regexp.MustCompile(`(?is)\[img\](.*?)\[/img\]`)
	reColorTag  = regexp.MustCompile(`(?is)\[color=[^\]]+\](.*?)\[/color\]`)
	reQuoteTag  = regexp.MustCompile(`(?is)\[quote(?:=[^\]]*)?\](.*?)\[/quote\]`)
	reIndentTag = regexp.MustCompile(`(?is)\[indent\](.*?)\[/indent\]`)
	reAnyTag    = regexp.MustCompile(`\[/?[^\]\s=]+(?:=[^\]]*)?\]`)
)

// bbcodeToMarkdown converts WoWInterface BBCode markup to Markdown.
func bbcodeToMarkdown(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Lists (multiline — must run before inline tags).
	s = reListBlock.ReplaceAllStringFunc(s, func(m string) string {
		sub := reListBlock.FindStringSubmatch(m)
		ordered := sub[1] != ""
		items := reListItem.Split(sub[2], -1)
		var lines []string
		num := 1
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if ordered {
				lines = append(lines, fmt.Sprintf("%d. %s", num, item))
				num++
			} else {
				lines = append(lines, "- "+item)
			}
		}
		if len(lines) == 0 {
			return ""
		}
		return "\n" + strings.Join(lines, "\n") + "\n"
	})
	// Standalone [*] outside a list block → bullet.
	s = reListItem.ReplaceAllString(s, "\n- ")

	// Block quotes.
	s = reQuoteTag.ReplaceAllStringFunc(s, func(m string) string {
		sub := reQuoteTag.FindStringSubmatch(m)
		inner := strings.TrimSpace(sub[1])
		lines := strings.Split(inner, "\n")
		for i, l := range lines {
			lines[i] = "> " + l
		}
		return "\n" + strings.Join(lines, "\n") + "\n"
	})

	// Size tags → headings.
	s = reSizeTag.ReplaceAllStringFunc(s, func(m string) string {
		sub := reSizeTag.FindStringSubmatch(m)
		size, _ := strconv.Atoi(sub[1])
		text := strings.TrimSpace(sub[2])
		switch {
		case size >= 5:
			return "\n## " + text + "\n"
		case size >= 4:
			return "\n### " + text + "\n"
		case size >= 3:
			return "\n#### " + text + "\n"
		default:
			return text
		}
	})

	// Inline formatting.
	s = reBoldTag.ReplaceAllString(s, "**$1**")
	s = reItalicTag.ReplaceAllString(s, "*$1*")
	s = reUnderTag.ReplaceAllString(s, "_$1_")
	s = reStrikeTag.ReplaceAllString(s, "~~$1~~")
	s = reCodeTag.ReplaceAllString(s, "`$1`")
	s = reURLTag1.ReplaceAllStringFunc(s, func(m string) string {
		sub := reURLTag1.FindStringSubmatch(m)
		url := strings.Trim(sub[1], `"'`)
		text := strings.TrimSpace(sub[2])
		if text == "" {
			return url
		}
		return "[" + text + "](" + url + ")"
	})
	s = reURLTag2.ReplaceAllString(s, "$1")
	s = reIMGTag.ReplaceAllString(s, "") // drop inline images
	s = reColorTag.ReplaceAllString(s, "$1")
	s = reIndentTag.ReplaceAllString(s, "$1")

	// Strip any remaining unrecognised tags.
	s = reAnyTag.ReplaceAllString(s, "")

	// Collapse excessive blank lines.
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

func unixMsToDate(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.Unix(ms/1000, 0).UTC().Format("2006-01-02")
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func fetch(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "wow-addon-tracker-scraper/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ── Fetch filelist ────────────────────────────────────────────────────────────

func fetchFilelist() ([]filelistEntry, error) {
	fmt.Print("Fetching addon filelist... ")
	body, err := fetch(filelistURL)
	if err != nil {
		return nil, fmt.Errorf("filelist: %w", err)
	}
	var entries []filelistEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("filelist parse: %w", err)
	}
	fmt.Printf("%d addons\n", len(entries))
	return entries, nil
}

// ── Fetch details in batches ─────────────────────────────────────────────────

func fetchDetailsBatch(ids []int) ([]detailEntry, error) {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	url := detailsBase + strings.Join(parts, ",") + ".json"
	body, err := fetch(url)
	if err != nil {
		return nil, err
	}
	// API returns {"ERROR":"..."} when all IDs are invalid.
	if len(body) > 0 && body[0] == '{' {
		return nil, nil
	}
	var entries []detailEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("details parse: %w", err)
	}
	return entries, nil
}

func fetchAllDetails(ids []int) (map[int]detailEntry, int) {
	// Split into batches.
	var batches [][]int
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batches = append(batches, ids[i:end])
	}

	fmt.Printf("Fetching details (%d batches, %d workers)...\n", len(batches), workers)

	type result struct {
		entries []detailEntry
		err     error
	}
	results := make([]result, len(batches))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var done int64

	for i, batch := range batches {
		wg.Add(1)
		go func(i int, batch []int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			entries, err := fetchDetailsBatch(batch)
			results[i] = result{entries: entries, err: err}
			done++
			if done%10 == 0 || int(done) == len(batches) {
				fmt.Printf("  %d/%d batches\n", done, len(batches))
			}
		}(i, batch)
	}
	wg.Wait()

	byID := make(map[int]detailEntry)
	errCount := 0
	for _, r := range results {
		if r.err != nil {
			errCount++
			continue
		}
		for _, e := range r.entries {
			byID[e.ID] = e
		}
	}
	return byID, errCount
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	output := flag.String("out", "assets/addon-db.json", "output path")
	flag.Parse()

	// 1. Filelist.
	filelistEntries, err := fetchFilelist()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Build ID slice and lookup map.
	ids := make([]int, len(filelistEntries))
	byID := make(map[int]filelistEntry, len(filelistEntries))
	for i, e := range filelistEntries {
		ids[i] = e.ID
		byID[e.ID] = e
	}

	// 2. Details.
	detailsByID, errCount := fetchAllDetails(ids)
	fmt.Printf("  %d details fetched, %d batch errors\n", len(detailsByID), errCount)

	// 3. Merge.
	out := make([]AddonDBEntry, 0, len(filelistEntries))
	for _, fl := range filelistEntries {
		entry := AddonDBEntry{
			WoWInterfaceID:   fl.ID,
			Name:             fl.Title,
			Author:           fl.Author,
			LatestVersion:    fl.Version,
			LatestDate:       unixMsToDate(fl.LastUpdate),
			Downloads:        fl.Downloads,
			DownloadsMonthly: fl.DownloadsMonthly,
			Favorites:        fl.Favorites,
			CategoryID:       fl.CategoryID,
			DownloadURL:      fmt.Sprintf("https://cdn.wowinterface.com/downloads/getfile.php?id=%d", fl.ID),
			FileInfoURL:      fl.FileInfoURI,
			Compatibility:    fl.GameVersions,
		}
		if d, ok := detailsByID[fl.ID]; ok {
			entry.Description = bbcodeToMarkdown(d.Description)
			entry.Changelog = bbcodeToMarkdown(d.ChangeLog)
			// Prefer monthly downloads from details when available.
			if d.DownloadsMonthly > 0 {
				entry.DownloadsMonthly = d.DownloadsMonthly
			}
			// Use the exact downloadUri from the API when available; it
			// resolves correctly for the current file version.
			if d.DownloadURI != "" {
				entry.DownloadURL = d.DownloadURI
			}
		}
		// Omit placeholder changelog value from the API.
		if entry.Changelog == "None" {
			entry.Changelog = ""
		}
		out = append(out, entry)
	}

	// Sort by downloads descending so the most-used addons appear first.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Downloads > out[j].Downloads
	})

	// 4. Write.
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d addons to %s (%.1f MB)\n", len(out), *output, float64(len(data))/1e6)
}
