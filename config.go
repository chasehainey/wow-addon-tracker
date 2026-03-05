package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "wow-addon-tracker")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create config dir: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func loadConfig() tea.Cmd {
	return func() tea.Msg {
		path, err := configPath()
		if err != nil {
			return configLoadedMsg{err: err}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Return defaults
				home, _ := os.UserHomeDir()
				cfg := Config{
					RetailPath:  filepath.Join(home, "Dropbox (Maestral)", "Games", "WoW", "Retail", "Interface", "Addons"),
					ClassicPath: filepath.Join(home, "Dropbox (Maestral)", "Games", "WoW", "Classic", "Interface", "Addons"),
					Addons:      []TrackedAddon{},
					Profiles:    []Profile{},
				}
				return configLoadedMsg{config: cfg}
			}
			return configLoadedMsg{err: err}
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return configLoadedMsg{err: fmt.Errorf("could not parse config: %w", err)}
		}

		// Ensure slices are non-nil
		if cfg.Addons == nil {
			cfg.Addons = []TrackedAddon{}
		}
		if cfg.Profiles == nil {
			cfg.Profiles = []Profile{}
		}

		// Backfill defaults for empty paths
		if cfg.RetailPath == "" {
			home, _ := os.UserHomeDir()
			cfg.RetailPath = filepath.Join(home, "Dropbox (Maestral)", "Games", "WoW", "Retail", "Interface", "Addons")
		}
		if cfg.ClassicPath == "" {
			home, _ := os.UserHomeDir()
			cfg.ClassicPath = filepath.Join(home, "Dropbox (Maestral)", "Games", "WoW", "Classic", "Interface", "Addons")
		}

		return configLoadedMsg{config: cfg}
	}
}

func saveConfig(cfg Config) tea.Cmd {
	return func() tea.Msg {
		path, err := configPath()
		if err != nil {
			return configSavedMsg{err: err}
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return configSavedMsg{err: fmt.Errorf("could not marshal config: %w", err)}
		}

		// Atomic write via tmp+rename
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return configSavedMsg{err: fmt.Errorf("could not write config: %w", err)}
		}
		if err := os.Rename(tmp, path); err != nil {
			return configSavedMsg{err: fmt.Errorf("could not rename config: %w", err)}
		}

		return configSavedMsg{}
	}
}
