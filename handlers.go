package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// inDashboard returns true when the app is showing the main dashboard (no overlays).
func (m model) inDashboard() bool {
	return !m.confirmQuit &&
		!m.viewBrowseDetail &&
		!m.browseInstallConfirm &&
		!m.inputAddRepo &&
		!m.addRepoConfirm &&
		!m.installing &&
		!m.updatingSingle &&
		!m.confirmDelete &&
		!m.viewAddonDetail &&
		!m.updatingAll &&
		!m.inputNewProfile &&
		!m.selectModeProfileAddons &&
		!m.viewProfileDetail &&
		!m.viewProfiles &&
		!m.inputSettingsRetail &&
		!m.inputSettingsClassic &&
		!m.inputSettingsToken &&
		!m.viewSettings
}

// handleDashboard handles input for the split-panel dashboard (default view).
func (m model) handleDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Browse filter active: forward all input to the textinput
	if m.dashboardFocus == "browse" && m.browseDBFilterActive {
		keyMsg, isKey := msg.(tea.KeyPressMsg)
		if isKey {
			switch keyMsg.String() {
			case "esc":
				m.browseDBFilterActive = false
				m.browseDBFilter = ""
				m.textInputBrowseFilter.Blur()
				m.browseDBIndices = m.browseCurIndices()
				m.browseDBCursor = 0
				return m, nil
			case "enter":
				m.browseDBFilterActive = false
				m.textInputBrowseFilter.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.textInputBrowseFilter, cmd = m.textInputBrowseFilter.Update(msg)
		m.browseDBFilter = m.textInputBrowseFilter.Value()
		m.browseDBIndices = m.browseCurIndices()
		m.browseDBCursor = 0
		return m, cmd
	}

	// Installed filter active: forward all input to the textinput
	if m.dashboardFocus == "installed" && m.addonFilterActive {
		keyMsg, isKey := msg.(tea.KeyPressMsg)
		if isKey {
			switch keyMsg.String() {
			case "esc":
				m.addonFilterActive = false
				m.addonListFilter = ""
				m.textInputAddonFilter.Blur()
				m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, "", m.config.Addons)
				if m.addonListCursor >= len(m.addonFilteredIndices) {
					m.addonListCursor = 0
				}
				return m, nil
			case "enter":
				m.addonFilterActive = false
				m.textInputAddonFilter.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.textInputAddonFilter, cmd = m.textInputAddonFilter.Update(msg)
		m.addonListFilter = m.textInputAddonFilter.Value()
		m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, m.addonListFilter, m.config.Addons)
		m.addonListCursor = 0
		return m, cmd
	}

	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return m, nil
	}

	m.errorMsg = ""
	m.successMsg = ""

	switch keyMsg.String() {
	case "tab":
		if m.dashboardFocus == "installed" {
			m.dashboardFocus = "browse"
		} else {
			m.dashboardFocus = "installed"
		}

	case "j", "down":
		if m.dashboardFocus == "installed" {
			if m.addonListCursor < len(m.addonFilteredIndices)-1 {
				m.addonListCursor++
			}
		} else {
			if m.browseDBCursor < len(m.browseDBIndices)-1 {
				m.browseDBCursor++
			}
		}

	case "k", "up":
		if m.dashboardFocus == "installed" {
			if m.addonListCursor > 0 {
				m.addonListCursor--
			}
		} else {
			if m.browseDBCursor > 0 {
				m.browseDBCursor--
			}
		}

	case "enter":
		if m.dashboardFocus == "installed" {
			if len(m.addonFilteredIndices) > 0 {
				m.selectedAddonIdx = m.addonFilteredIndices[m.addonListCursor]
				m.viewAddonDetail = true
				m = setupChangelogViewport(m)
			}
		} else {
			if len(m.browseDBIndices) == 0 {
				break
			}
			if len(m.browseDBSelected) > 0 {
				m.browseInstallConfirm = true
				m.browseInstallFlavor = m.defaultFlavor
			} else {
				idx := m.browseDBIndices[m.browseDBCursor]
				entry := m.addonDB[idx]
				return m.startInstallFromEntry(entry)
			}
		}

	case "space":
		if m.dashboardFocus == "browse" && len(m.browseDBIndices) > 0 {
			idx := m.browseDBIndices[m.browseDBCursor]
			if _, ok := m.browseDBSelected[idx]; ok {
				delete(m.browseDBSelected, idx)
			} else {
				m.browseDBSelected[idx] = struct{}{}
			}
		}

	case "/":
		if m.dashboardFocus == "installed" {
			m.addonFilterActive = true
			m.textInputAddonFilter.SetValue("")
			m.textInputAddonFilter.Focus()
		} else {
			m.browseDBFilterActive = true
			m.textInputBrowseFilter.SetValue("")
			m.textInputBrowseFilter.Focus()
		}

	case "1", "2", "3":
		if m.dashboardFocus == "browse" {
			tabs := []string{"all", "hot", "new"}
			idx := int(keyMsg.String()[0] - '1')
			if idx < len(tabs) {
				m.browseTab = tabs[idx]
				m.browseDBIndices = m.browseCurIndices()
				m.browseDBCursor = 0
			}
		}

	case "f":
		if m.defaultFlavor == "retail" {
			m.defaultFlavor = "classic"
		} else {
			m.defaultFlavor = "retail"
		}
		m.pendingFlavor = m.defaultFlavor
		m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, m.addonListFilter, m.config.Addons)
		m.addonListCursor = 0

	case "a":
		m.inputAddRepo = true
		m.textInputRepo.SetValue("")
		m.textInputRepo.Focus()
		m.pendingFlavor = m.defaultFlavor

	case "c":
		if m.checkingUpdates {
			return m, nil
		}
		if len(m.config.Addons) > 0 {
			m.checkingUpdates = true
			m.loading = true
			return m, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken)
		}
		m.errorMsg = "No addons tracked. Press [a] to add one."

	case "U":
		if m.checkingUpdates {
			return m, nil
		}
		if len(m.config.Addons) > 0 {
			m.updatingAll = true
			m.loading = true
			return m, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken)
		}
		m.errorMsg = "No addons tracked. Press [a] to add one."

	case "r":
		m.successMsg = ""
		m.errorMsg = ""
		m.dbRefreshing = true
		m.rssHotLoaded = false
		m.rssNewLoaded = false
		return m, tea.Batch(refreshWoWInterfaceDB(m.addonDB), fetchWoWIRSS("hot"), fetchWoWIRSS("new"))

	case "p":
		m.viewProfiles = true
		m.profileListCursor = 0

	case "s":
		m.viewSettings = true
		m.settingsCursor = 0

	case "esc":
		// Reset state: clear any filters, selections, and messages.
		// If already in a clean state, show the quit confirmation popup.
		anythingCleared := false
		if m.addonListFilter != "" {
			m.addonListFilter = ""
			m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, "", m.config.Addons)
			anythingCleared = true
		}
		if m.browseDBFilter != "" {
			m.browseDBFilter = ""
			m.browseDBIndices = m.browseCurIndices()
			anythingCleared = true
		}
		if len(m.browseDBSelected) > 0 {
			m.browseDBSelected = make(map[int]struct{})
			anythingCleared = true
		}
		if m.errorMsg != "" || m.successMsg != "" {
			m.errorMsg = ""
			m.successMsg = ""
			anythingCleared = true
		}
		if !anythingCleared {
			m.confirmQuit = true
			m.quitConfirmFocus = "no"
		}

	case "q":
		m.confirmQuit = true
		m.quitConfirmFocus = "no"
	}

	return m, nil
}

// handleQuitConfirm handles the quit confirmation popup.
func (m model) handleQuitConfirm(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "tab", "left", "right", "h", "l":
		if m.quitConfirmFocus == "no" {
			m.quitConfirmFocus = "yes"
		} else {
			m.quitConfirmFocus = "no"
		}
	case "enter":
		if m.quitConfirmFocus == "yes" {
			return m, tea.Quit
		}
		m.confirmQuit = false
	case "esc":
		m.confirmQuit = false
	}
	return m, nil
}

// handleAddonList handles key events on the addon list view
func (m model) handleAddonList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.addonFilterActive {
		keyMsg, isKey := msg.(tea.KeyPressMsg)
		if isKey {
			switch keyMsg.String() {
			case "esc":
				m.addonFilterActive = false
				m.addonListFilter = ""
				m.textInputAddonFilter.Blur()
				m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, "", m.config.Addons)
				if m.addonListCursor >= len(m.addonFilteredIndices) {
					m.addonListCursor = 0
				}
				return m, nil
			case "enter":
				m.addonFilterActive = false
				m.textInputAddonFilter.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.textInputAddonFilter, cmd = m.textInputAddonFilter.Update(msg)
		m.addonListFilter = m.textInputAddonFilter.Value()
		m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, m.addonListFilter, m.config.Addons)
		m.addonListCursor = 0
		return m, cmd
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "j", "down":
		if m.addonListCursor < len(m.addonFilteredIndices)-1 {
			m.addonListCursor++
		}
	case "k", "up":
		if m.addonListCursor > 0 {
			m.addonListCursor--
		}
	case "enter", "space":
		if len(m.addonFilteredIndices) > 0 {
			m.selectedAddonIdx = m.addonFilteredIndices[m.addonListCursor]
			m.viewAddonDetail = true
			m = setupChangelogViewport(m)
		}
	case "/":
		m.addonFilterActive = true
		m.textInputAddonFilter.SetValue("")
		m.textInputAddonFilter.Focus()
	case "c":
		if !m.checkingUpdates && len(m.config.Addons) > 0 {
			m.checkingUpdates = true
			m.loading = true
			return m, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken)
		}
	case "U":
		if !m.checkingUpdates && len(m.config.Addons) > 0 {
			m.updatingAll = true
			m.loading = true
			return m, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken)
		}
	case "a":
		m.viewAddons = false
		m.inputAddRepo = true
		m.textInputRepo.SetValue("")
		m.textInputRepo.Focus()
	case "esc", "q":
		m.viewAddons = false
		m.addonFilterActive = false
		m.addonListFilter = ""
	}
	return m, nil
}

// handleAddonDetail handles key events on the addon detail view
func (m model) handleAddonDetail(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "u":
		if m.selectedAddonIdx < len(m.config.Addons) {
			addon := m.config.Addons[m.selectedAddonIdx]
			if id := wowiIDFromKey(addon.GithubRepo); id > 0 {
				for _, e := range m.addonDB {
					if e.WoWInterfaceID == id {
						release := wowiMakeRelease(e)
						path := addonPath(m.config, addon.GameFlavor)
						m.updatingSingle = true
						m.loading = true
						m.downloadProgress = 0.1
						return m, tea.Batch(installAddon(addon.GithubRepo, release, path, "", addon.ExtractAs), downloadTick())
					}
				}
				return m, nil
			}
			m.updatingSingle = true
			m.loading = true
			return m, fetchLatestRelease(addon.GithubRepo, m.config.GithubToken)
		}
	case "d":
		m.confirmDelete = true
	case "esc", "q":
		m.viewAddonDetail = false
	case "up", "k":
		m.viewport.ScrollUp(1)
	case "down", "j":
		m.viewport.ScrollDown(1)
	case "pgup", "b":
		m.viewport.PageUp()
	case "pgdown", "f", " ":
		m.viewport.PageDown()
	case "home", "g":
		m.viewport.GotoTop()
	case "end", "G":
		m.viewport.GotoBottom()
	}
	return m, nil
}

// handleBrowseDB handles the Browse Addons view
func (m model) handleBrowseDB(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.browseDBFilterActive {
		keyMsg, isKey := msg.(tea.KeyPressMsg)
		if isKey {
			switch keyMsg.String() {
			case "esc":
				m.browseDBFilterActive = false
				m.browseDBFilter = ""
				m.textInputBrowseFilter.Blur()
				m.browseDBIndices = m.browseCurIndices()
				m.browseDBCursor = 0
				return m, nil
			case "enter":
				m.browseDBFilterActive = false
				m.textInputBrowseFilter.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.textInputBrowseFilter, cmd = m.textInputBrowseFilter.Update(msg)
		m.browseDBFilter = m.textInputBrowseFilter.Value()
		m.browseDBIndices = m.browseCurIndices()
		m.browseDBCursor = 0
		return m, cmd
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "j", "down":
		if m.browseDBCursor < len(m.browseDBIndices)-1 {
			m.browseDBCursor++
		}
	case "k", "up":
		if m.browseDBCursor > 0 {
			m.browseDBCursor--
		}
	case "space":
		if len(m.browseDBIndices) > 0 && m.browseDBCursor < len(m.browseDBIndices) {
			idx := m.browseDBIndices[m.browseDBCursor]
			if _, ok := m.browseDBSelected[idx]; ok {
				delete(m.browseDBSelected, idx)
			} else {
				m.browseDBSelected[idx] = struct{}{}
			}
		}
	case "/":
		m.browseDBFilterActive = true
		m.textInputBrowseFilter.SetValue("")
		m.textInputBrowseFilter.Focus()
	case "enter":
		if len(m.browseDBIndices) == 0 {
			return m, nil
		}
		if len(m.browseDBSelected) > 0 {
			// Batch install selected addons
			m.browseInstallConfirm = true
			m.browseInstallFlavor = "retail"
		} else {
			// Open detail view for the current cursor item
			idx := m.browseDBIndices[m.browseDBCursor]
			m.selectedBrowseDBIdx = idx
			m.viewBrowseDetail = true
			m = setupBrowseDetailViewport(m)
		}
	case "1", "2", "3":
		tabs := []string{"all", "hot", "new"}
		idx := int(keyMsg.String()[0] - '1')
		if idx < len(tabs) {
			m.browseTab = tabs[idx]
			m.browseDBIndices = m.browseCurIndices()
			m.browseDBCursor = 0
		}
	case "esc", "q":
		m.viewBrowseDB = false
		m.browseDBFilter = ""
		m.browseDBFilterActive = false
		m.browseDBSelected = make(map[int]struct{})
	}
	return m, nil
}

// handleBrowseDetail handles input in the Browse addon detail view.
func (m model) handleBrowseDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Pass scroll events to viewport.
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	switch keyMsg.String() {
	case "i", "enter":
		if m.selectedBrowseDBIdx < len(m.addonDB) {
			entry := m.addonDB[m.selectedBrowseDBIdx]
			m.viewBrowseDetail = false
			return m.startInstallFromEntry(entry)
		}
	case "k", "up":
		m.viewport.ScrollUp(1)
	case "j", "down":
		m.viewport.ScrollDown(1)
	case "esc", "q":
		m.viewBrowseDetail = false
	}
	return m, nil
}

// handleBrowseInstallConfirm handles the flavor picker before batch installing from Browse
func (m model) handleBrowseInstallConfirm(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "tab":
		if m.browseInstallFlavor == "retail" {
			m.browseInstallFlavor = "classic"
		} else {
			m.browseInstallFlavor = "retail"
		}
	case "enter":
		// Build install queue from selected db indices
		m.browseInstallQueue = nil
		for idx := range m.browseDBSelected {
			e := m.addonDB[idx]
			if e.WoWInterfaceID > 0 {
				m.browseInstallQueue = append(m.browseInstallQueue, wowiKeyFromID(e.WoWInterfaceID))
			} else if e.Repo != "" {
				m.browseInstallQueue = append(m.browseInstallQueue, e.Repo)
			}
		}
		if len(m.browseInstallQueue) == 0 {
			m.browseInstallConfirm = false
			return m, nil
		}
		m.browseInstallConfirm = false
		m.browseInstalling = true
		m.browseInstallIdx = 0
		m.loading = true
		return m, m.nextBrowseInstallCmd()
	case "esc":
		m.browseInstallConfirm = false
	}
	return m, nil
}

// handleAddRepoInput handles the add-addon repo input screen
func (m model) handleAddRepoInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch keyMsg.String() {
		case "esc":
			m.inputAddRepo = false
			m.textInputRepo.Blur()
			m.dbSuggestions = nil
			m.dbSuggestionIdx = -1
			return m, nil
		case "tab":
			if len(m.dbSuggestions) > 0 {
				m.dbSuggestionIdx = (m.dbSuggestionIdx + 1) % len(m.dbSuggestions)
				e := m.dbSuggestions[m.dbSuggestionIdx]
				if e.WoWInterfaceID > 0 {
					return m.startInstallFromEntry(e)
				}
				m.textInputRepo.SetValue(e.Repo)
				m.dbSuggestions = computeDBSuggestions(m.textInputRepo.Value(), m.addonDB)
			}
			return m, nil
		case "enter":
			repo := strings.TrimSpace(m.textInputRepo.Value())
			if repo == "" {
				m.errorMsg = "Please enter a GitHub repo (Owner/Repo)"
				return m, nil
			}
			parts := strings.SplitN(repo, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				m.errorMsg = "Invalid format. Use Owner/Repo (e.g. WeakAuras2/WeakAuras2)"
				return m, nil
			}
			m.pendingRepo = repo
			m.addRepoFetching = true
			m.loading = true
			m.errorMsg = ""
			m.dbSuggestions = nil
			m.dbSuggestionIdx = -1
			return m, fetchLatestRelease(repo, m.config.GithubToken)
		}
	}
	var cmd tea.Cmd
	m.textInputRepo, cmd = m.textInputRepo.Update(msg)
	m.dbSuggestions = computeDBSuggestions(m.textInputRepo.Value(), m.addonDB)
	m.dbSuggestionIdx = -1
	return m, cmd
}

// handleAddRepoConfirm handles the add-addon confirmation screen
func (m model) handleAddRepoConfirm(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "esc":
		m.addRepoConfirm = false
		m.pendingRelease = nil
		m.pendingRepo = ""
	case "tab":
		if m.pendingFlavor == "retail" {
			m.pendingFlavor = "classic"
		} else {
			m.pendingFlavor = "retail"
		}
	case "enter":
		if m.pendingRelease == nil {
			return m, nil
		}
		release := *m.pendingRelease
		repo := m.pendingRepo
		flavor := m.pendingFlavor
		path := addonPath(m.config, flavor)

		m.addRepoConfirm = false
		m.installing = true
		m.loading = true
		m.downloadProgress = 0.1
		m.pendingRelease = nil
		return m, tea.Batch(installAddon(repo, release, path, m.config.GithubToken, m.pendingExtractAs), downloadTick())
	}
	return m, nil
}

// handleConfirmDelete handles the delete confirmation prompt
func (m model) handleConfirmDelete(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "y", "Y":
		if m.selectedAddonIdx < len(m.config.Addons) {
			addon := m.config.Addons[m.selectedAddonIdx]
			m.confirmDelete = false
			m.loading = true
			path := addonPath(m.config, addon.GameFlavor)
			return m, deleteAddonFolders(addon, path)
		}
	case "n", "N", "esc":
		m.confirmDelete = false
	}
	return m, nil
}

// handleProfileList handles key events on the profile list view
func (m model) handleProfileList(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "j", "down":
		if m.profileListCursor < len(m.config.Profiles)-1 {
			m.profileListCursor++
		}
	case "k", "up":
		if m.profileListCursor > 0 {
			m.profileListCursor--
		}
	case "enter":
		if len(m.config.Profiles) > 0 {
			m.selectedProfileIdx = m.profileListCursor
			m.viewProfileDetail = true
		}
	case "n":
		m.inputNewProfile = true
		m.textInputProfileName.SetValue("")
		m.textInputProfileName.Focus()
	case "esc", "q":
		m.viewProfiles = false
	}
	return m, nil
}

// handleProfileDetail handles key events on the profile detail view
func (m model) handleProfileDetail(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "e":
		m.selectModeProfileAddons = true
		m.profileAddonCursor = 0
		m.profileAddonSelected = make(map[int]struct{})
		if m.selectedProfileIdx < len(m.config.Profiles) {
			profile := m.config.Profiles[m.selectedProfileIdx]
			for i, addon := range m.config.Addons {
				for _, name := range profile.Addons {
					if name == addon.Name {
						m.profileAddonSelected[i] = struct{}{}
					}
				}
			}
		}
	case "d":
		if m.selectedProfileIdx < len(m.config.Profiles) {
			m.config.Profiles = append(
				m.config.Profiles[:m.selectedProfileIdx],
				m.config.Profiles[m.selectedProfileIdx+1:]...,
			)
			m.viewProfileDetail = false
			if m.profileListCursor >= len(m.config.Profiles) && m.profileListCursor > 0 {
				m.profileListCursor--
			}
			return m, saveConfig(m.config)
		}
	case "esc", "q":
		m.viewProfileDetail = false
	}
	return m, nil
}

// handleProfileAddonSelect handles the addon multi-select for profile editing
func (m model) handleProfileAddonSelect(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "j", "down":
		if m.profileAddonCursor < len(m.config.Addons)-1 {
			m.profileAddonCursor++
		}
	case "k", "up":
		if m.profileAddonCursor > 0 {
			m.profileAddonCursor--
		}
	case "space":
		if _, ok := m.profileAddonSelected[m.profileAddonCursor]; ok {
			delete(m.profileAddonSelected, m.profileAddonCursor)
		} else {
			m.profileAddonSelected[m.profileAddonCursor] = struct{}{}
		}
	case "enter":
		if m.selectedProfileIdx < len(m.config.Profiles) {
			var addonNames []string
			for i := range m.config.Addons {
				if _, ok := m.profileAddonSelected[i]; ok {
					addonNames = append(addonNames, m.config.Addons[i].Name)
				}
			}
			if addonNames == nil {
				addonNames = []string{}
			}
			m.config.Profiles[m.selectedProfileIdx].Addons = addonNames
			m.selectModeProfileAddons = false
			return m, saveConfig(m.config)
		}
	case "esc":
		m.selectModeProfileAddons = false
	}
	return m, nil
}

// handleNewProfileInput handles the new profile name input
func (m model) handleNewProfileInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch keyMsg.String() {
		case "esc":
			m.inputNewProfile = false
			m.textInputProfileName.Blur()
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.textInputProfileName.Value())
			if name == "" {
				m.errorMsg = "Profile name cannot be empty"
				return m, nil
			}
			for _, p := range m.config.Profiles {
				if p.Name == name {
					m.errorMsg = "A profile with that name already exists"
					return m, nil
				}
			}
			newProfile := Profile{Name: name, Addons: []string{}}
			m.config.Profiles = append(m.config.Profiles, newProfile)
			m.inputNewProfile = false
			m.textInputProfileName.Blur()
			m.selectedProfileIdx = len(m.config.Profiles) - 1
			m.viewProfileDetail = true
			m.errorMsg = ""
			return m, saveConfig(m.config)
		}
	}
	var cmd tea.Cmd
	m.textInputProfileName, cmd = m.textInputProfileName.Update(msg)
	return m, cmd
}

// handleSettings handles key events on the settings view
func (m model) handleSettings(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	settingsItems := []string{"Retail Path", "Classic Path", "GitHub Token", "Back"}
	switch keyMsg.String() {
	case "j", "down":
		if m.settingsCursor < len(settingsItems)-1 {
			m.settingsCursor++
		}
	case "k", "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "enter":
		switch settingsItems[m.settingsCursor] {
		case "Retail Path":
			m.inputSettingsRetail = true
			m.textInputSettingsRetail.SetValue(m.config.RetailPath)
			m.textInputSettingsRetail.Focus()
		case "Classic Path":
			m.inputSettingsClassic = true
			m.textInputSettingsClassic.SetValue(m.config.ClassicPath)
			m.textInputSettingsClassic.Focus()
		case "GitHub Token":
			m.inputSettingsToken = true
			m.textInputSettingsToken.SetValue(m.config.GithubToken)
			m.textInputSettingsToken.Focus()
		case "Back":
			m.viewSettings = false
		}
	case "esc", "q":
		m.viewSettings = false
	}
	return m, nil
}

// handleSettingsRetailInput handles the retail path input
func (m model) handleSettingsRetailInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch keyMsg.String() {
		case "esc":
			m.inputSettingsRetail = false
			m.textInputSettingsRetail.Blur()
			return m, nil
		case "enter":
			m.config.RetailPath = expandPath(strings.TrimSpace(m.textInputSettingsRetail.Value()))
			m.inputSettingsRetail = false
			m.textInputSettingsRetail.Blur()
			m.successMsg = "Retail path saved"
			return m, saveConfig(m.config)
		}
	}
	var cmd tea.Cmd
	m.textInputSettingsRetail, cmd = m.textInputSettingsRetail.Update(msg)
	return m, cmd
}

// handleSettingsClassicInput handles the classic path input
func (m model) handleSettingsClassicInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch keyMsg.String() {
		case "esc":
			m.inputSettingsClassic = false
			m.textInputSettingsClassic.Blur()
			return m, nil
		case "enter":
			m.config.ClassicPath = expandPath(strings.TrimSpace(m.textInputSettingsClassic.Value()))
			m.inputSettingsClassic = false
			m.textInputSettingsClassic.Blur()
			m.successMsg = "Classic path saved"
			return m, saveConfig(m.config)
		}
	}
	var cmd tea.Cmd
	m.textInputSettingsClassic, cmd = m.textInputSettingsClassic.Update(msg)
	return m, cmd
}

// handleMouseClick handles left-click events dispatched by bubblezone zone markers.
// It mirrors the mode routing in Update() and directly performs the relevant action.
func (m model) handleMouseClick(msg tea.MouseReleaseMsg) (tea.Model, tea.Cmd) {
	switch {
	// --- Browse detail view ---
	case m.viewBrowseDetail:
		if zone.Get("browse-detail-install").InBounds(msg) {
			if m.selectedBrowseDBIdx < len(m.addonDB) {
				entry := m.addonDB[m.selectedBrowseDBIdx]
				m.viewBrowseDetail = false
				return m.startInstallFromEntry(entry)
			}
		}

	// --- Quit confirm ---
	case m.confirmQuit:
		if zone.Get("quit-yes").InBounds(msg) {
			return m, tea.Quit
		} else if zone.Get("quit-no").InBounds(msg) {
			m.confirmQuit = false
		}

	// --- Browse batch install confirm ---
	case m.browseInstallConfirm:
		if zone.Get("browse-flavor-retail").InBounds(msg) {
			m.browseInstallFlavor = "retail"
		} else if zone.Get("browse-flavor-classic").InBounds(msg) {
			m.browseInstallFlavor = "classic"
		} else if zone.Get("browse-action-install").InBounds(msg) {
			return m.handleBrowseInstallConfirm(tea.KeyPressMsg{Code: tea.KeyEnter})
		} else if zone.Get("browse-action-cancel").InBounds(msg) {
			m.browseInstallConfirm = false
		}

	// --- Add Addon: confirm install ---
	case m.addRepoConfirm:
		if zone.Get("flavor-retail").InBounds(msg) {
			m.pendingFlavor = "retail"
		} else if zone.Get("flavor-classic").InBounds(msg) {
			m.pendingFlavor = "classic"
		} else if zone.Get("action-install").InBounds(msg) {
			if m.pendingRelease != nil {
				release := *m.pendingRelease
				path := addonPath(m.config, m.pendingFlavor)
				m.addRepoConfirm = false
				m.installing = true
				m.loading = true
				m.downloadProgress = 0.1
				m.pendingRelease = nil
				return m, tea.Batch(installAddon(m.pendingRepo, release, path, m.config.GithubToken, m.pendingExtractAs), downloadTick())
			}
		} else if zone.Get("action-cancel").InBounds(msg) {
			m.addRepoConfirm = false
			m.pendingRelease = nil
			m.pendingRepo = ""
		}

	// --- Delete confirm ---
	case m.confirmDelete:
		if zone.Get("delete-yes").InBounds(msg) {
			if m.selectedAddonIdx < len(m.config.Addons) {
				addon := m.config.Addons[m.selectedAddonIdx]
				m.confirmDelete = false
				m.loading = true
				return m, deleteAddonFolders(addon, addonPath(m.config, addon.GameFlavor))
			}
		} else if zone.Get("delete-no").InBounds(msg) {
			m.confirmDelete = false
		}

	// --- Addon detail buttons ---
	case m.viewAddonDetail:
		if zone.Get("detail-update").InBounds(msg) {
			if m.selectedAddonIdx < len(m.config.Addons) {
				addon := m.config.Addons[m.selectedAddonIdx]
				if id := wowiIDFromKey(addon.GithubRepo); id > 0 {
					for _, e := range m.addonDB {
						if e.WoWInterfaceID == id {
							release := wowiMakeRelease(e)
							path := addonPath(m.config, addon.GameFlavor)
							m.updatingSingle = true
							m.loading = true
							m.downloadProgress = 0.1
							return m, tea.Batch(installAddon(addon.GithubRepo, release, path, "", addon.ExtractAs), downloadTick())
						}
					}
					return m, nil
				}
				m.updatingSingle = true
				m.loading = true
				return m, fetchLatestRelease(addon.GithubRepo, m.config.GithubToken)
			}
		} else if zone.Get("detail-delete").InBounds(msg) {
			m.confirmDelete = true
		}

	// --- Profile addon multi-select ---
	case m.selectModeProfileAddons:
		for i := range m.config.Addons {
			if zone.Get(fmt.Sprintf("profile-addon-%d", i)).InBounds(msg) {
				m.profileAddonCursor = i
				if _, ok := m.profileAddonSelected[i]; ok {
					delete(m.profileAddonSelected, i)
				} else {
					m.profileAddonSelected[i] = struct{}{}
				}
				break
			}
		}

	// --- Profile list ---
	case m.viewProfiles && !m.viewProfileDetail && !m.inputNewProfile:
		for i := range m.config.Profiles {
			if zone.Get(fmt.Sprintf("profile-%d", i)).InBounds(msg) {
				m.selectedProfileIdx = i
				m.profileListCursor = i
				m.viewProfileDetail = true
				break
			}
		}

	// --- Settings ---
	case m.viewSettings:
		settingsItems := []string{"Retail Path", "Classic Path", "GitHub Token", "Back"}
		for i, item := range settingsItems {
			if zone.Get(fmt.Sprintf("settings-%d", i)).InBounds(msg) {
				m.settingsCursor = i
				switch item {
				case "Retail Path":
					m.inputSettingsRetail = true
					m.textInputSettingsRetail.SetValue(m.config.RetailPath)
					m.textInputSettingsRetail.Focus()
				case "Classic Path":
					m.inputSettingsClassic = true
					m.textInputSettingsClassic.SetValue(m.config.ClassicPath)
					m.textInputSettingsClassic.Focus()
				case "GitHub Token":
					m.inputSettingsToken = true
					m.textInputSettingsToken.SetValue(m.config.GithubToken)
					m.textInputSettingsToken.Focus()
				case "Back":
					m.viewSettings = false
				}
				break
			}
		}

	// --- Add Addon repo input: click suggestion to fill ---
	case m.inputAddRepo && !m.addRepoFetching:
		for i, e := range m.dbSuggestions {
			if zone.Get(fmt.Sprintf("suggest-%d", i)).InBounds(msg) {
				if e.WoWInterfaceID > 0 {
					return m.startInstallFromEntry(e)
				}
				m.textInputRepo.SetValue(e.Repo)
				m.dbSuggestionIdx = i
				m.dbSuggestions = computeDBSuggestions(e.Repo, m.addonDB)
				break
			}
		}

	// --- Dashboard: installed panel, browse panel, sidebar ---
	default:
		if m.inDashboard() {
			// Installed panel row clicks:
			// first click on a non-focused panel switches focus + moves cursor;
			// clicking when already focused opens the addon detail.
			if !m.addonFilterActive {
				for listPos := range m.addonFilteredIndices {
					if zone.Get(fmt.Sprintf("inst-row-%d", listPos)).InBounds(msg) {
						if m.dashboardFocus != "installed" {
							m.dashboardFocus = "installed"
							m.addonListCursor = listPos
						} else {
							m.selectedAddonIdx = m.addonFilteredIndices[listPos]
							m.addonListCursor = listPos
							m.viewAddonDetail = true
							m = setupChangelogViewport(m)
						}
						break
					}
				}
			}
			// Browse panel row clicks: first click focuses + moves cursor; second click opens detail.
			if !m.browseDBFilterActive {
				for listPos, idx := range m.browseDBIndices {
					if zone.Get(fmt.Sprintf("browse-row-%d", listPos)).InBounds(msg) {
						if m.dashboardFocus != "browse" || m.browseDBCursor != listPos {
							m.dashboardFocus = "browse"
							m.browseDBCursor = listPos
						} else {
							// Already on this row — open detail.
							m.selectedBrowseDBIdx = idx
							m.viewBrowseDetail = true
							m = setupBrowseDetailViewport(m)
						}
						break
					}
				}
			}
			// Flavor tab clicks (tab bar above installed list)
			if zone.Get("tab-retail").InBounds(msg) {
				m.defaultFlavor = "retail"
				m.addonFilteredIndices = computeAddonFilter("retail", m.addonListFilter, m.config.Addons)
				m.addonListCursor = 0
			} else if zone.Get("tab-classic").InBounds(msg) {
				m.defaultFlavor = "classic"
				m.addonFilteredIndices = computeAddonFilter("classic", m.addonListFilter, m.config.Addons)
				m.addonListCursor = 0
			}
			// Browse flavor tab clicks (Retail / Classic)
			if zone.Get("browse-flavor-tab-retail").InBounds(msg) {
				m.dashboardFocus = "browse"
				m.browseFlavor = "retail"
				m.browseDBIndices = m.browseCurIndices()
				m.browseDBCursor = 0
			} else if zone.Get("browse-flavor-tab-classic").InBounds(msg) {
				m.dashboardFocus = "browse"
				m.browseFlavor = "classic"
				m.browseDBIndices = m.browseCurIndices()
				m.browseDBCursor = 0
			}
			// Browse tab clicks (All / Hot / New)
			for _, tabID := range []string{"all", "hot", "new"} {
				if zone.Get("browse-tab-"+tabID).InBounds(msg) {
					m.dashboardFocus = "browse"
					m.browseTab = tabID
					m.browseDBIndices = m.browseCurIndices()
					m.browseDBCursor = 0
					break
				}
			}
			// Sidebar action buttons
			sidebarActions := []string{"c", "U", "a", "p", "s", "q"}
			for i := range sidebarActions {
				if zone.Get(fmt.Sprintf("sidebar-action-%d", i)).InBounds(msg) {
					switch sidebarActions[i] {
					case "c":
						if len(m.config.Addons) > 0 {
							m.checkingUpdates = true
							m.loading = true
							return m, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken)
						}
					case "U":
						if len(m.config.Addons) > 0 {
							m.updatingAll = true
							m.loading = true
							return m, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken)
						}
					case "a":
						m.inputAddRepo = true
						m.textInputRepo.SetValue("")
						m.textInputRepo.Focus()
						m.pendingFlavor = m.defaultFlavor
					case "p":
						m.viewProfiles = true
						m.profileListCursor = 0
					case "s":
						m.viewSettings = true
						m.settingsCursor = 0
					case "q":
						return m, tea.Quit
					}
					break
				}
			}
		}
	}
	return m, nil
}

// handleMouseWheel scrolls the active list view up or down.
func (m model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	down := msg.Button == tea.MouseWheelDown
	up := msg.Button == tea.MouseWheelUp

	switch {
	case m.viewAddonDetail || m.viewBrowseDetail:
		if down {
			m.viewport.ScrollDown(3)
		} else if up {
			m.viewport.ScrollUp(3)
		}
	case m.viewProfiles && !m.viewProfileDetail && !m.inputNewProfile:
		if down && m.profileListCursor < len(m.config.Profiles)-1 {
			m.profileListCursor++
		} else if up && m.profileListCursor > 0 {
			m.profileListCursor--
		}
	case m.selectModeProfileAddons:
		if down && m.profileAddonCursor < len(m.config.Addons)-1 {
			m.profileAddonCursor++
		} else if up && m.profileAddonCursor > 0 {
			m.profileAddonCursor--
		}
	case m.viewSettings:
		settingsCount := 4
		if down && m.settingsCursor < settingsCount-1 {
			m.settingsCursor++
		} else if up && m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case m.inDashboard():
		if m.dashboardFocus == "installed" && !m.addonFilterActive {
			if down && m.addonListCursor < len(m.addonFilteredIndices)-1 {
				m.addonListCursor++
			} else if up && m.addonListCursor > 0 {
				m.addonListCursor--
			}
		} else if m.dashboardFocus == "browse" && !m.browseDBFilterActive {
			if down && m.browseDBCursor < len(m.browseDBIndices)-1 {
				m.browseDBCursor++
			} else if up && m.browseDBCursor > 0 {
				m.browseDBCursor--
			}
		}
	}
	return m, nil
}

// startInstallFromEntry begins an install for a DB entry.
// WoWInterface entries go straight to the confirm screen.
// GitHub entries fetch the latest release first.
func (m model) startInstallFromEntry(entry AddonDBEntry) (model, tea.Cmd) {
	if entry.WoWInterfaceID > 0 {
		release := wowiMakeRelease(entry)
		m.pendingRepo = wowiKeyFromID(entry.WoWInterfaceID)
		m.pendingRelease = &release
		m.pendingFlavor = m.defaultFlavor
		m.pendingExtractAs = entry.ExtractAs
		m.inputAddRepo = false
		m.addRepoConfirm = true
		return m, nil
	}
	if entry.Repo != "" {
		m.pendingRepo = entry.Repo
		m.addRepoFetching = true
		m.loading = true
		m.errorMsg = ""
		m.inputAddRepo = false
		m.dbSuggestions = nil
		m.dbSuggestionIdx = -1
		return m, fetchLatestRelease(entry.Repo, m.config.GithubToken)
	}
	return m, nil
}

// handleSettingsTokenInput handles the GitHub token input
func (m model) handleSettingsTokenInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch keyMsg.String() {
		case "esc":
			m.inputSettingsToken = false
			m.textInputSettingsToken.Blur()
			return m, nil
		case "enter":
			m.config.GithubToken = strings.TrimSpace(m.textInputSettingsToken.Value())
			m.inputSettingsToken = false
			m.textInputSettingsToken.Blur()
			m.successMsg = "GitHub token saved"
			return m, saveConfig(m.config)
		}
	}
	var cmd tea.Cmd
	m.textInputSettingsToken, cmd = m.textInputSettingsToken.Update(msg)
	return m, cmd
}
