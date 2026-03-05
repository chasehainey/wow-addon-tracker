package main

/*
WoW Addon Tracker - GitHub-based WoW addon manager TUI

This is a Bubble Tea v2 TUI application organized across 9 files for maintainability.
Below is a guide to where specific functionality lives:

FILE GUIDE:
-----------

main.go (this file)
  - Application entry point
  - Initializes and runs the Bubble Tea program

types.go
  - All type definitions and data structures
  - Theme definitions (Dracula, DraculaLight)
  - Persisted types: TrackedAddon, Profile, Config
  - GitHub API response types: GitHubRelease, GitHubAsset
  - Runtime view types: AddonWithStatus, UpdateStatus
  - Message types for Bubble Tea communication (*Msg structs)
  - The main model struct with all state fields
  → Look here when: Adding new data types, modifying state structure

config.go
  - Config load/save (~/.config/wow-addon-tracker/config.json)
  - configPath(), loadConfig(), saveConfig(), expandPath()
  → Look here when: Changing config structure, default paths

api.go
  - GitHub API calls: fetchLatestRelease(), fetchLatestReleaseSync()
  - Concurrent batch check: checkAllAddons()
  - normalizeVersion()
  → Look here when: Adding API calls, changing GitHub request logic

install.go
  - installAddon(): download ZIP + extract to AddOns dir
  - deleteAddonFolders(): remove addon directories
  → Look here when: Changing install/delete behavior

utils.go
  - getStyles() - Theme-based lipgloss styling
  - detectTerminalBackground() - Theme auto-detection
  - initialModel() - Creates the initial application state
  - Init() - Bubble Tea initialization
  - Helper functions: computeAddonFilter, statusSymbol, addonPath
  → Look here when: Adding helpers, modifying initialization, changing styling

handlers.go
  - Mode-specific handler methods
  - handleDashboard, handleAddonList, handleAddonDetail
  - handleAddRepoInput, handleAddRepoConfirm, handleConfirmDelete
  - handleProfileList, handleProfileDetail, handleProfileAddonSelect
  - handleNewProfileInput, handleSettings, handleSettings*Input
  → Look here when: Adding new modes, modifying user interactions

update.go
  - Main Update() method - the Bubble Tea update loop
  - Routes all tea.Msg events to appropriate handlers
  - Handles message types, window resize, spinner
  → Look here when: Adding keyboard shortcuts, changing event flow

view.go
  - Main View() method - the Bubble Tea view/render loop
  - All UI rendering for every mode and state
  → Look here when: Changing UI layout, modifying display text

ARCHITECTURE NOTES:
-------------------
- All files use `package main`
- The model struct is defined in types.go but its methods are spread across files:
  * model.getStyles() → utils.go
  * model.Init() → utils.go
  * model.Update() → update.go
  * model.View() → view.go
  * model.handle*() → handlers.go
*/

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

func main() {
	zone.NewGlobal()
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
