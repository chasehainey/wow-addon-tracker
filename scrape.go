package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const remoteDBURL = "https://raw.githubusercontent.com/chasehainey/wow-addon-tracker/main/assets/addon-db.json"

//go:embed assets/addon-db.json
var embeddedAddonDB []byte

// addonDBPath returns the path to the local addon database JSON file
func addonDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "wow-addon-tracker")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create config dir: %w", err)
	}
	return filepath.Join(dir, "addon-db.json"), nil
}

// loadAddonDB loads the addon database. Uses the local override if present,
// otherwise falls back to the DB bundled at build time.
func loadAddonDB() tea.Cmd {
	return func() tea.Msg {
		path, err := addonDBPath()
		if err != nil {
			return dbLoadedMsg{entries: []AddonDBEntry{}, err: err}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				data = embeddedAddonDB
			} else {
				return dbLoadedMsg{entries: []AddonDBEntry{}, err: err}
			}
		}

		var entries []AddonDBEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return dbLoadedMsg{entries: []AddonDBEntry{}, err: fmt.Errorf("could not parse addon db: %w", err)}
		}
		if entries == nil {
			entries = []AddonDBEntry{}
		}
		return dbLoadedMsg{entries: entries}
	}
}

// saveAddonDB saves the addon database to the local override path atomically.
func saveAddonDB(entries []AddonDBEntry) tea.Cmd {
	return func() tea.Msg {
		path, err := addonDBPath()
		if err != nil {
			return configSavedMsg{err: err}
		}

		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return configSavedMsg{err: fmt.Errorf("could not marshal addon db: %w", err)}
		}

		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return configSavedMsg{err: fmt.Errorf("could not write addon db: %w", err)}
		}
		if err := os.Rename(tmp, path); err != nil {
			return configSavedMsg{err: fmt.Errorf("could not rename addon db: %w", err)}
		}
		return configSavedMsg{}
	}
}

// fetchRemoteDB downloads the curated addon database from the hosted GitHub
// repo and returns a dbLoadedMsg with save=true so the result is cached locally.
func fetchRemoteDB() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(remoteDBURL)
		if err != nil {
			return dbLoadedMsg{err: fmt.Errorf("remote DB fetch: %w", err)}
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return dbLoadedMsg{err: fmt.Errorf("remote DB HTTP %d", resp.StatusCode)}
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return dbLoadedMsg{err: fmt.Errorf("remote DB read: %w", err)}
		}
		var entries []AddonDBEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return dbLoadedMsg{err: fmt.Errorf("remote DB parse: %w", err)}
		}
		if entries == nil {
			entries = []AddonDBEntry{}
		}
		return dbLoadedMsg{entries: entries, save: true}
	}
}

// computeDBSuggestions returns up to 8 entries matching query (case-insensitive substring on Name or Repo).
func computeDBSuggestions(query string, db []AddonDBEntry) []AddonDBEntry {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var out []AddonDBEntry
	for _, e := range db {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Repo), q) {
			out = append(out, e)
			if len(out) >= 8 {
				break
			}
		}
	}
	return out
}

// computeBrowseFilter returns indices into db matching query (case-insensitive on Name, Repo, Description, Language).
func computeBrowseFilter(query string, db []AddonDBEntry) []int {
	if query == "" {
		return allDBIndices(db)
	}
	q := strings.ToLower(query)
	var out []int
	for i, e := range db {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Repo), q) ||
			strings.Contains(strings.ToLower(e.Description), q) ||
			strings.Contains(strings.ToLower(e.Language), q) {
			out = append(out, i)
		}
	}
	return out
}

// allDBIndices returns a slice [0, 1, ..., len(db)-1].
func allDBIndices(db []AddonDBEntry) []int {
	out := make([]int, len(db))
	for i := range db {
		out[i] = i
	}
	return out
}
