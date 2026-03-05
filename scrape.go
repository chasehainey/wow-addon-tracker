package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

const (
	wowiHotURL    = "https://www.wowinterface.com/rss/hot.xml"
	wowiLatestURL = "https://www.wowinterface.com/rss/latest.xml"
)

//go:embed assets/addon-db.json
var embeddedAddonDB []byte

const wowiFilelistURL = "https://api.mmoui.com/v4/game/WOW/filelist.json"

// ── Embedded DB load ──────────────────────────────────────────────────────────

// loadAddonDB loads the addon database bundled at build time.
func loadAddonDB() tea.Cmd {
	return func() tea.Msg {
		var entries []AddonDBEntry
		if err := json.Unmarshal(embeddedAddonDB, &entries); err != nil {
			return dbLoadedMsg{err: fmt.Errorf("parse embedded DB: %w", err)}
		}
		if entries == nil {
			entries = []AddonDBEntry{}
		}
		return dbLoadedMsg{entries: entries}
	}
}

// ── Background filelist refresh ([r] action) ──────────────────────────────────

// refreshWoWInterfaceDB fetches the MMOUI filelist to update version/date info
// for every entry. Descriptions and download URLs from the embedded DB are
// preserved — only version, date, downloads, and favorites are updated.
func refreshWoWInterfaceDB(existing []AddonDBEntry) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(wowiFilelistURL)
		if err != nil {
			return dbLoadedMsg{err: fmt.Errorf("filelist fetch: %w", err)}
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return dbLoadedMsg{err: fmt.Errorf("filelist HTTP %d", resp.StatusCode)}
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return dbLoadedMsg{err: fmt.Errorf("filelist read: %w", err)}
		}

		type filelistEntry struct {
			ID               int      `json:"id"`
			Version          string   `json:"version"`
			LastUpdate       int64    `json:"lastUpdate"`
			Title            string   `json:"title"`
			Author           string   `json:"author"`
			Downloads        int      `json:"downloads"`
			DownloadsMonthly int      `json:"downloadsMonthly"`
			Favorites        int      `json:"favorites"`
			CategoryID       int      `json:"categoryId"`
			GameVersions     []string `json:"gameVersions"`
			FileInfoURI      string   `json:"fileInfoUri"`
		}
		var fresh []filelistEntry
		if err := json.Unmarshal(body, &fresh); err != nil {
			return dbLoadedMsg{err: fmt.Errorf("filelist parse: %w", err)}
		}

		// Build lookup maps.
		freshByID := make(map[int]filelistEntry, len(fresh))
		for _, e := range fresh {
			freshByID[e.ID] = e
		}
		existingByID := make(map[int]*AddonDBEntry, len(existing))
		for i := range existing {
			if existing[i].WoWInterfaceID > 0 {
				existingByID[existing[i].WoWInterfaceID] = &existing[i]
			}
		}

		// Update existing entries; append new ones.
		updated := make([]AddonDBEntry, len(existing))
		copy(updated, existing)
		for i := range updated {
			if updated[i].WoWInterfaceID == 0 {
				continue
			}
			f, ok := freshByID[updated[i].WoWInterfaceID]
			if !ok {
				continue
			}
			updated[i].LatestVersion = f.Version
			updated[i].LatestDate = unixMsToDate(f.LastUpdate)
			updated[i].Downloads = f.Downloads
			updated[i].DownloadsMonthly = f.DownloadsMonthly
			updated[i].Favorites = f.Favorites
			if len(f.GameVersions) > 0 {
				updated[i].Compatibility = f.GameVersions
			}
			if f.FileInfoURI != "" && updated[i].FileInfoURL == "" {
				updated[i].FileInfoURL = f.FileInfoURI
			}
			if updated[i].Name == "" {
				updated[i].Name = f.Title
			}
			if updated[i].Author == "" {
				updated[i].Author = f.Author
			}
		}
		// Append entries in the fresh list that aren't in the existing DB.
		for _, f := range fresh {
			if _, exists := existingByID[f.ID]; !exists {
				updated = append(updated, AddonDBEntry{
					WoWInterfaceID:   f.ID,
					Name:             f.Title,
					Author:           f.Author,
					LatestVersion:    f.Version,
					LatestDate:       unixMsToDate(f.LastUpdate),
					Downloads:        f.Downloads,
					DownloadsMonthly: f.DownloadsMonthly,
					Favorites:        f.Favorites,
					CategoryID:       f.CategoryID,
					DownloadURL:      wowiDownloadURL(f.ID),
					FileInfoURL:      f.FileInfoURI,
					Compatibility:    f.GameVersions,
				})
			}
		}
		return dbLoadedMsg{entries: updated}
	}
}

// unixMsToDate converts a Unix timestamp in milliseconds to "YYYY-MM-DD".
func unixMsToDate(ms int64) string {
	if ms == 0 {
		return ""
	}
	sec := ms / 1000
	// Simple date extraction without importing time in scrape.go.
	// Use a Sprintf approach via the standard library epoch math.
	// seconds since epoch → year/month/day
	// Delegate to a small inline calc.
	days := sec / 86400
	// Days since 1970-01-01.
	y, m, d := julianToGregorian(days + 2440588) // Julian day for 1970-01-01 = 2440588
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

// julianToGregorian converts a Julian Day Number to (year, month, day).
// Algorithm: Fliegel & Van Flandern (CACM 1968).
func julianToGregorian(jd int64) (int64, int64, int64) {
	l := jd + 68569
	n := (4 * l) / 146097
	l = l - (146097*n+3)/4
	i := (4000 * (l + 1)) / 1461001
	l = l - (1461*i)/4 + 31
	j := (80 * l) / 2447
	d := l - (2447*j)/80
	l = j / 11
	m := j + 2 - 12*l
	y := 100*(n-49) + i + l
	return y, m, d
}

// ── Pending install helpers ───────────────────────────────────────────────────

// pendingDBEntry looks up the AddonDBEntry for the current pending install repo.
// Returns nil if not found.
func (m model) pendingDBEntry() *AddonDBEntry {
	return dbEntryForRepo(m.pendingRepo, m.addonDB)
}

// installedAddonDBEntry looks up the AddonDBEntry for an installed addon.
// Returns nil if not found.
func (m model) installedAddonDBEntry(addon TrackedAddon) *AddonDBEntry {
	return dbEntryForRepo(addon.GithubRepo, m.addonDB)
}

// dbEntryForRepo finds an AddonDBEntry by pseudo-repo key (wowinterface:ID or Owner/Repo).
func dbEntryForRepo(repo string, db []AddonDBEntry) *AddonDBEntry {
	if id := wowiIDFromKey(repo); id > 0 {
		for i := range db {
			if db[i].WoWInterfaceID == id {
				return &db[i]
			}
		}
	} else if repo != "" {
		for i := range db {
			if db[i].Repo == repo {
				return &db[i]
			}
		}
	}
	return nil
}

// ── WoWInterface install helpers ──────────────────────────────────────────────

// wowiKeyFromID returns the pseudo-repo key used to identify a WoWInterface addon.
func wowiKeyFromID(id int) string {
	return fmt.Sprintf("wowinterface:%d", id)
}

// wowiIDFromKey extracts the WoWInterface addon ID from a pseudo-repo key.
// Returns 0 if the key is not a WoWInterface key.
func wowiIDFromKey(key string) int {
	if !strings.HasPrefix(key, "wowinterface:") {
		return 0
	}
	id := 0
	s := strings.TrimPrefix(key, "wowinterface:")
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		id = id*10 + int(c-'0')
	}
	return id
}

// wowiDownloadURL returns the CDN download URL for a WoWInterface addon.
func wowiDownloadURL(id int) string {
	return fmt.Sprintf("https://cdn.wowinterface.com/downloads/getfile.php?id=%d", id)
}

// wowiMakeRelease constructs a GitHubRelease-compatible install target from a
// WoWInterface DB entry. The BrowserDownloadURL drives the zip download.
func wowiMakeRelease(e AddonDBEntry) GitHubRelease {
	dlURL := e.DownloadURL
	if dlURL == "" {
		dlURL = wowiDownloadURL(e.WoWInterfaceID)
	}
	body := e.Changelog
	if body == "" {
		body = e.Description
	}
	return GitHubRelease{
		TagName: e.LatestVersion,
		Name:    e.Name,
		Body:    body,
		Assets: []GitHubAsset{{
			Name:               e.Name + ".zip",
			BrowserDownloadURL: dlURL,
			ContentType:        "application/zip",
		}},
	}
}

// ── Background detail enrichment (fetches descriptions for new entries) ───────

// enrichMissingDescriptions fetches details for DB entries that have no
// description yet, running up to batchSize IDs per request.
func enrichMissingDescriptions(entries []AddonDBEntry, token string) tea.Cmd {
	const batchSize = 100
	var toFetch []int
	for _, e := range entries {
		if e.WoWInterfaceID > 0 && e.Description == "" {
			toFetch = append(toFetch, e.WoWInterfaceID)
		}
	}
	if len(toFetch) == 0 {
		return nil
	}

	return func() tea.Msg {
		type detail struct {
			ID          int    `json:"id"`
			Description string `json:"description"`
			DownloadURI string `json:"downloadUri"`
		}
		byID := make(map[int]detail)
		sem := make(chan struct{}, 10)
		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < len(toFetch); i += batchSize {
			end := i + batchSize
			if end > len(toFetch) {
				end = len(toFetch)
			}
			batch := toFetch[i:end]
			wg.Add(1)
			go func(batch []int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				parts := make([]string, len(batch))
				for j, id := range batch {
					parts[j] = fmt.Sprintf("%d", id)
				}
				url := "https://api.mmoui.com/v4/game/WOW/filedetails/" +
					strings.Join(parts, ",") + ".json"
				resp, err := http.Get(url)
				if err != nil || resp.StatusCode != 200 {
					if resp != nil {
						resp.Body.Close()
					}
					return
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				var details []detail
				if json.Unmarshal(body, &details) != nil {
					return
				}
				mu.Lock()
				for _, d := range details {
					byID[d.ID] = d
				}
				mu.Unlock()
			}(batch)
		}
		wg.Wait()

		result := make([]AddonDBEntry, len(entries))
		copy(result, entries)
		for i, e := range result {
			if d, ok := byID[e.WoWInterfaceID]; ok {
				result[i].Description = bbcodeToMarkdown(d.Description)
				if result[i].DownloadURL == "" && d.DownloadURI != "" {
					result[i].DownloadURL = d.DownloadURI
				}
			}
		}
		return dbLoadedMsg{entries: result}
	}
}

// bbcodeToMarkdown converts WoWInterface BBCode markup to Markdown.
func bbcodeToMarkdown(raw string) string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	reList := regexp.MustCompile(`(?is)\[list(=\d+)?\](.*?)\[/list\]`)
	reItem := regexp.MustCompile(`(?i)\[\*\]`)
	s = reList.ReplaceAllStringFunc(s, func(m string) string {
		sub := reList.FindStringSubmatch(m)
		ordered := sub[1] != ""
		items := reItem.Split(sub[2], -1)
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
	s = reItem.ReplaceAllString(s, "\n- ")

	reQuote := regexp.MustCompile(`(?is)\[quote(?:=[^\]]*)?\](.*?)\[/quote\]`)
	s = reQuote.ReplaceAllStringFunc(s, func(m string) string {
		sub := reQuote.FindStringSubmatch(m)
		lines := strings.Split(strings.TrimSpace(sub[1]), "\n")
		for i, l := range lines {
			lines[i] = "> " + l
		}
		return "\n" + strings.Join(lines, "\n") + "\n"
	})

	reSize := regexp.MustCompile(`(?is)\[size=(\d+)\](.*?)\[/size\]`)
	s = reSize.ReplaceAllStringFunc(s, func(m string) string {
		sub := reSize.FindStringSubmatch(m)
		var sz int
		fmt.Sscanf(sub[1], "%d", &sz)
		text := strings.TrimSpace(sub[2])
		switch {
		case sz >= 5:
			return "\n## " + text + "\n"
		case sz >= 4:
			return "\n### " + text + "\n"
		case sz >= 3:
			return "\n#### " + text + "\n"
		default:
			return text
		}
	})

	s = regexp.MustCompile(`(?is)\[b\](.*?)\[/b\]`).ReplaceAllString(s, "**$1**")
	s = regexp.MustCompile(`(?is)\[i\](.*?)\[/i\]`).ReplaceAllString(s, "*$1*")
	s = regexp.MustCompile(`(?is)\[u\](.*?)\[/u\]`).ReplaceAllString(s, "_$1_")
	s = regexp.MustCompile(`(?is)\[s\](.*?)\[/s\]`).ReplaceAllString(s, "~~$1~~")
	s = regexp.MustCompile(`(?is)\[code\](.*?)\[/code\]`).ReplaceAllString(s, "`$1`")
	s = regexp.MustCompile(`(?is)\[url=([^\]]+)\](.*?)\[/url\]`).ReplaceAllStringFunc(s, func(m string) string {
		re := regexp.MustCompile(`(?is)\[url=([^\]]+)\](.*?)\[/url\]`)
		sub := re.FindStringSubmatch(m)
		url := strings.Trim(sub[1], `"'`)
		text := strings.TrimSpace(sub[2])
		if text == "" {
			return url
		}
		return "[" + text + "](" + url + ")"
	})
	s = regexp.MustCompile(`(?is)\[url\](.*?)\[/url\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?is)\[img\](.*?)\[/img\]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)\[color=[^\]]+\](.*?)\[/color\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?is)\[indent\](.*?)\[/indent\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`\[/?[^\]\s=]+(?:=[^\]]*)?\]`).ReplaceAllString(s, "")

	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// stripBBCode removes BBCode markup tags and normalises whitespace.
func stripBBCode(s string) string {
	var out strings.Builder
	inTag := false
	for _, c := range s {
		switch {
		case c == '[':
			inTag = true
		case c == ']':
			inTag = false
		case !inTag:
			out.WriteRune(c)
		}
	}
	// Collapse runs of whitespace-only lines.
	lines := strings.Split(out.String(), "\n")
	var result []string
	blanks := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blanks++
			if blanks <= 1 {
				result = append(result, "")
			}
		} else {
			blanks = 0
			result = append(result, l)
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// ── Compatibility helpers ─────────────────────────────────────────────────────

// isRetailAddon returns true when the addon lists any 12.x.x game version.
func isRetailAddon(e AddonDBEntry) bool {
	for _, v := range e.Compatibility {
		if strings.HasPrefix(v, "12.") {
			return true
		}
	}
	return false
}

// filterByFlavor removes indices whose addon doesn't match the given flavor.
// flavor "" passes everything through. "retail" keeps 12.x.x addons; "classic"
// keeps everything without a 12.x.x tag.
func filterByFlavor(indices []int, db []AddonDBEntry, flavor string) []int {
	if flavor == "" {
		return indices
	}
	out := indices[:0:0] // re-use backing array without aliasing
	for _, i := range indices {
		retail := isRetailAddon(db[i])
		if (flavor == "retail" && retail) || (flavor == "classic" && !retail) {
			out = append(out, i)
		}
	}
	return out
}

// ── Browse/search helpers ─────────────────────────────────────────────────────

// computeDBSuggestions returns up to 8 entries matching query
// (case-insensitive substring on Name or Author).
func computeDBSuggestions(query string, db []AddonDBEntry) []AddonDBEntry {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var out []AddonDBEntry
	for _, e := range db {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Author), q) {
			out = append(out, e)
			if len(out) >= 8 {
				break
			}
		}
	}
	return out
}

// computeBrowseFilter returns indices into db matching query
// (case-insensitive on Name, Author, Description).
func computeBrowseFilter(query string, db []AddonDBEntry) []int {
	if query == "" {
		return allDBIndices(db)
	}
	q := strings.ToLower(query)
	var out []int
	for i, e := range db {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Author), q) ||
			strings.Contains(strings.ToLower(e.Description), q) {
			out = append(out, i)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		return db[out[a]].Downloads > db[out[b]].Downloads
	})
	return out
}

// allDBIndices returns indices [0..len(db)-1] sorted by downloads descending.
func allDBIndices(db []AddonDBEntry) []int {
	out := make([]int, len(db))
	for i := range db {
		out[i] = i
	}
	sort.Slice(out, func(a, b int) bool {
		return db[out[a]].Downloads > db[out[b]].Downloads
	})
	return out
}

// ── RSS feed fetching ─────────────────────────────────────────────────────────

// fetchWoWIRSS fetches a WoWInterface RSS feed and returns the WoWInterface IDs
// in the order they appear in the feed. feedType is "hot" or "new".
func fetchWoWIRSS(feedType string) tea.Cmd {
	url := wowiHotURL
	if feedType == "new" {
		url = wowiLatestURL
	}
	return func() tea.Msg {
		resp, err := http.Get(url)
		if err != nil {
			return rssLoadedMsg{feedType: feedType, err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return rssLoadedMsg{feedType: feedType, err: fmt.Errorf("HTTP %d", resp.StatusCode)}
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return rssLoadedMsg{feedType: feedType, err: err}
		}

		// Go's xml package only handles UTF-8/UTF-16. WoWInterface RSS feeds
		// declare ISO-8859-1 but their content is ASCII-compatible, so we
		// can safely rewrite the encoding declaration before parsing.
		body = bytes.Replace(body, []byte(`encoding="ISO-8859-1"`), []byte(`encoding="UTF-8"`), 1)

		type rssItem struct {
			Link string `xml:"link"`
		}
		type rssChannel struct {
			Items []rssItem `xml:"item"`
		}
		type rssFeed struct {
			Channel rssChannel `xml:"channel"`
		}
		var feed rssFeed
		if err := xml.Unmarshal(body, &feed); err != nil {
			return rssLoadedMsg{feedType: feedType, err: fmt.Errorf("parse: %w", err)}
		}

		var ids []int
		for _, item := range feed.Channel.Items {
			if id := extractWoWIIDFromURL(item.Link); id > 0 {
				ids = append(ids, id)
			}
		}
		return rssLoadedMsg{feedType: feedType, ids: ids}
	}
}

// extractWoWIIDFromURL parses a WoWInterface URL like
// "https://www.wowinterface.com/downloads/info24954-WeakAuras2.html"
// and returns the numeric addon ID (24954), or 0 on failure.
func extractWoWIIDFromURL(link string) int {
	idx := strings.Index(link, "/info")
	if idx < 0 {
		return 0
	}
	s := link[idx+5:] // skip "/info"
	id := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + int(c-'0')
		} else {
			break
		}
	}
	return id
}

// computeTabIndices returns browse indices ordered by the given ID list.
// Only DB entries whose WoWInterfaceID appears in ids are included.
// Any active text filter is applied before ordering.
// Returns nil when ids is empty (RSS not yet loaded).
func computeTabIndices(query string, db []AddonDBEntry, ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	// Build wowiID → db index map (filtered by query).
	filtered := computeBrowseFilter(query, db)
	filtSet := make(map[int]int, len(filtered)) // wowiID → dbIndex
	for _, dbIdx := range filtered {
		if db[dbIdx].WoWInterfaceID > 0 {
			filtSet[db[dbIdx].WoWInterfaceID] = dbIdx
		}
	}
	var out []int
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if dbIdx, ok := filtSet[id]; ok && !seen[id] {
			out = append(out, dbIdx)
			seen[id] = true
		}
	}
	return out
}
