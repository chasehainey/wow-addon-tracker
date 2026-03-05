package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// 1. Window resize — handle first in all modes
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalHeight = msg.Height
		m.terminalWidth = msg.Width
		if !m.viewportReady {
			m.viewport = viewport.New(viewport.WithWidth(msg.Width), viewport.WithHeight(msg.Height-6))
			m.viewportReady = true
		} else {
			m.viewport.SetWidth(msg.Width)
			m.viewport.SetHeight(msg.Height - 6)
		}
		m.glamourRenderer = newGlamourRenderer(msg.Width - 8)
		if m.viewAddonDetail {
			m = setupChangelogViewport(m)
		}
		if m.viewBrowseDetail {
			m = setupBrowseDetailViewport(m)
		}
		return m, cmd
	}

	// 2. Global ctrl+c
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	// 3. Spinner tick — keep alive while loading or RSS feeds are in-flight.
	rssInFlight := !m.rssHotLoaded || !m.rssNewLoaded
	if m.loading || m.dbRefreshing || rssInFlight {
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		if _, ok := msg.(spinner.TickMsg); ok {
			return m, spinnerCmd
		}
		cmd = spinnerCmd
	}

	// 4. Message type switch
	switch msg := msg.(type) {
	case configLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = "Config error: " + msg.err.Error()
			return m, autoCheckTick()
		}
		m.config = msg.config
		m.addonsWithStatus = make([]AddonWithStatus, len(m.config.Addons))
		for i, a := range m.config.Addons {
			m.addonsWithStatus[i] = AddonWithStatus{Addon: a, Status: StatusUnknown}
		}
		m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, "", m.config.Addons)
		// Kick off an immediate check then schedule the 30-minute recurring tick.
		if len(m.config.Addons) > 0 {
			m.checkingUpdates = true
			m.loading = true
			return m, tea.Batch(cmd, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken), autoCheckTick())
		}
		return m, tea.Batch(cmd, autoCheckTick())

	case configSavedMsg:
		if msg.err != nil {
			m.errorMsg = "Save error: " + msg.err.Error()
		}
		return m, cmd

	case releaseCheckMsg:
		m.loading = false
		m.addRepoFetching = false

		if msg.err != nil {
			m.updatingSingle = false
			if m.browseInstalling {
				// Skip failed fetch, advance to next in queue
				m.browseInstallIdx++
				if m.browseInstallIdx < len(m.browseInstallQueue) {
					return m, fetchLatestRelease(m.browseInstallQueue[m.browseInstallIdx], m.config.GithubToken)
				}
				m.browseInstalling = false
				m.installing = false
				m.errorMsg = "Some installs failed"
				return m, cmd
			}
			m.errorMsg = "Fetch error: " + msg.err.Error()
			// If updateAll queue had an error, advance to next
			if m.updatingAll {
				m.updateAllErrors = append(m.updateAllErrors, msg.repo+": "+msg.err.Error())
				m.updateQueueIdx++
				if m.updateQueueIdx < len(m.updateQueue) {
					m.loading = true
					return m, tea.Batch(cmd, m.startNextQueuedUpdate())
				}
				m.updatingAll = false
				m.installing = false
			}
			return m, cmd
		}

		if m.browseInstalling {
			path := addonPath(m.config, m.browseInstallFlavor)
			m.installing = true
			m.loading = true
			m.downloadProgress = 0.1
			ea := addonExtractAs(msg.repo, m.config.Addons, m.addonDB)
			return m, tea.Batch(cmd, installAddon(msg.repo, msg.release, path, m.config.GithubToken, ea), downloadTick())
		}

		if m.updatingSingle {
			// Triggered from addon detail "u" key
			m.updatingSingle = false
			if m.selectedAddonIdx < len(m.config.Addons) {
				addon := m.config.Addons[m.selectedAddonIdx]
				path := addonPath(m.config, addon.GameFlavor)
				m.installing = true
				m.loading = true
				m.downloadProgress = 0.1
				return m, tea.Batch(cmd, installAddon(msg.repo, msg.release, path, m.config.GithubToken, addon.ExtractAs), downloadTick())
			}
			return m, cmd
		}

		if m.updatingAll {
			// Triggered from update queue — directly install without showing confirm
			if m.updateQueueIdx < len(m.updateQueue) {
				repo := m.updateQueue[m.updateQueueIdx]
				// Find flavor and extractAs for this addon
				flavor := "retail"
				ea := ""
				for _, a := range m.config.Addons {
					if a.GithubRepo == repo {
						flavor = a.GameFlavor
						ea = a.ExtractAs
						break
					}
				}
				path := addonPath(m.config, flavor)
				m.installing = true
				m.loading = true
				m.downloadProgress = 0.1
				return m, tea.Batch(cmd, installAddon(repo, msg.release, path, m.config.GithubToken, ea), downloadTick())
			}
			return m, cmd
		}

		// Triggered from add-addon / browse-click flow.
		// Update the GitHub DB entry with fresh release data (WoWI entries
		// are already populated from the embedded DB).
		if msg.release.TagName != "" && msg.release.TagName != "HEAD" {
			for i, e := range m.addonDB {
				if e.Repo != "" && e.Repo == msg.repo {
					m.addonDB[i].LatestVersion = msg.release.TagName
					m.addonDB[i].LatestDate = releaseDate(msg.release)
					if msg.release.Body != "" {
						m.addonDB[i].Changelog = msg.release.Body
					}
					break
				}
			}
		}
		release := msg.release
		m.pendingRelease = &release
		m.pendingRepo = msg.repo
		m.pendingFlavor = "retail"
		m.pendingExtractAs = addonExtractAs(msg.repo, m.config.Addons, m.addonDB)
		m.inputAddRepo = false
		m.addRepoConfirm = true
		return m, cmd

	case batchCheckCompleteMsg:
		m.loading = false
		m.checkingUpdates = false

		if msg.err != nil {
			m.errorMsg = "Check error: " + msg.err.Error()
			m.updatingAll = false
			return m, cmd
		}

		m.addonsWithStatus = msg.results
		for i, aws := range m.addonsWithStatus {
			if i < len(m.config.Addons) {
				m.config.Addons[i] = aws.Addon
			}
		}

		if m.updatingAll {
			m.updateQueue = []string{}
			m.updateQueueIdx = 0
			m.updateAllErrors = []string{}
			for _, aws := range m.addonsWithStatus {
				if aws.Status == StatusUpdateAvail || aws.Status == StatusNotInstalled {
					m.updateQueue = append(m.updateQueue, aws.Addon.GithubRepo)
				}
			}
			if len(m.updateQueue) == 0 {
				m.updatingAll = false
				m.successMsg = "All addons are up to date!"
				return m, cmd
			}
			m.loading = true
			return m, tea.Batch(cmd, m.startNextQueuedUpdate())
		}

		m.successMsg = "Update check complete"
		return m, tea.Batch(cmd, saveConfig(m.config))

	case installCompleteMsg:
		m.loading = false
		m.installing = false
		m.downloadProgress = 0.1

		if msg.err != nil {
			m.errorMsg = "Install error: " + msg.err.Error()
			if m.updatingAll {
				m.updateAllErrors = append(m.updateAllErrors, msg.repo+": "+msg.err.Error())
				m.updateQueueIdx++
				if m.updateQueueIdx < len(m.updateQueue) {
					m.loading = true
					m.installing = true
					return m, tea.Batch(cmd, m.startNextQueuedUpdate())
				}
				m.updatingAll = false
			}
			return m, cmd
		}

		installedAt := time.Now().Format(time.RFC3339)

		// Check if addon already exists in config
		found := false
		for i, a := range m.config.Addons {
			if a.GithubRepo == msg.repo {
				found = true
				m.config.Addons[i].InstalledVersion = msg.version
				m.config.Addons[i].InstalledDate = installedAt
				if msg.changelog != "" {
					m.config.Addons[i].Changelog = msg.changelog
				}
				if msg.extractAs != "" {
					m.config.Addons[i].ExtractAs = msg.extractAs
				}
				if len(msg.directories) > 0 {
					m.config.Addons[i].Directories = msg.directories
				}
				if i < len(m.addonsWithStatus) {
					m.addonsWithStatus[i].Addon = m.config.Addons[i]
					m.addonsWithStatus[i].Status = StatusUpToDate
				}
				break
			}
		}

		if !found {
			name := msg.repo
			if id := wowiIDFromKey(msg.repo); id > 0 {
				// WoWInterface addon — look up display name from the DB.
				for _, e := range m.addonDB {
					if e.WoWInterfaceID == id {
						name = e.Name
						break
					}
				}
			} else if parts := strings.SplitN(msg.repo, "/", 2); len(parts) == 2 {
				name = parts[1]
			}
			// When the addon ships without a folder and we extract into a named
			// directory, that directory name is the canonical addon name.
			if msg.extractAs != "" {
				name = msg.extractAs
			}
			flavor := m.pendingFlavor
			if m.browseInstalling {
				flavor = m.browseInstallFlavor
			}
			newAddon := TrackedAddon{
				Name:             name,
				GithubRepo:       msg.repo,
				WoWInterfaceID:   wowiIDFromKey(msg.repo),
				InstalledVersion: msg.version,
				InstalledDate:    installedAt,
				Changelog:        msg.changelog,
				ExtractAs:        msg.extractAs,
				Directories:      msg.directories,
				Profiles:         []string{},
				GameFlavor:       flavor,
			}
			m.config.Addons = append(m.config.Addons, newAddon)
			m.addonsWithStatus = append(m.addonsWithStatus, AddonWithStatus{
				Addon:  newAddon,
				Status: StatusUpToDate,
			})
		}

		m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, m.addonListFilter, m.config.Addons)

		if m.browseInstalling {
			m.browseInstallIdx++
			if m.browseInstallIdx < len(m.browseInstallQueue) {
				m.loading = true
				m.installing = true
				return m, tea.Batch(saveConfig(m.config), m.nextBrowseInstallCmd())
			}
			m.browseInstalling = false
			count := len(m.browseInstallQueue)
			m.browseInstallQueue = nil
			m.browseDBSelected = make(map[int]struct{})
			m.successMsg = fmt.Sprintf("Installed %d addons", count)
			return m, tea.Batch(cmd, saveConfig(m.config))
		}

		if m.updatingAll {
			m.updateQueueIdx++
			if m.updateQueueIdx < len(m.updateQueue) {
				m.loading = true
				m.installing = true
				return m, tea.Batch(saveConfig(m.config), m.startNextQueuedUpdate())
			}
			m.updatingAll = false
			if len(m.updateAllErrors) > 0 {
				m.errorMsg = m.updateAllErrors[0]
				if len(m.updateAllErrors) > 1 {
					m.errorMsg += fmt.Sprintf(" (+%d more)", len(m.updateAllErrors)-1)
				}
			} else {
				m.successMsg = "All updates complete!"
			}
			return m, tea.Batch(cmd, saveConfig(m.config))
		}

		m.successMsg = "Installed " + msg.version
		return m, tea.Batch(cmd, saveConfig(m.config))

	case downloadTickMsg:
		if m.installing {
			if m.downloadProgress < 0.90 {
				m.downloadProgress += 0.025
			}
			return m, downloadTick()
		}
		return m, cmd

	case autoCheckTickMsg:
		// Schedule the next tick regardless, then start a check if idle.
		next := autoCheckTick()
		if len(m.config.Addons) > 0 && !m.checkingUpdates && !m.installing && !m.updatingAll {
			m.checkingUpdates = true
			m.loading = true
			return m, tea.Batch(next, checkAllAddons(m.config.Addons, m.addonDB, m.config.GithubToken))
		}
		return m, next

	case addonDeletedMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = "Delete error: " + msg.err.Error()
			return m, cmd
		}
		for i, a := range m.config.Addons {
			if a.Name == msg.name {
				m.config.Addons = append(m.config.Addons[:i], m.config.Addons[i+1:]...)
				if i < len(m.addonsWithStatus) {
					m.addonsWithStatus = append(m.addonsWithStatus[:i], m.addonsWithStatus[i+1:]...)
				}
				break
			}
		}
		m.viewAddonDetail = false
		m.addonFilteredIndices = computeAddonFilter(m.defaultFlavor, m.addonListFilter, m.config.Addons)
		if m.addonListCursor >= len(m.addonFilteredIndices) && m.addonListCursor > 0 {
			m.addonListCursor--
		}
		m.successMsg = "Deleted " + msg.name
		return m, tea.Batch(cmd, saveConfig(m.config))

	case dbLoadedMsg:
		m.dbRefreshing = false
		if msg.err != nil {
			if len(m.addonDB) == 0 {
				m.errorMsg = "DB load error: " + msg.err.Error()
			} else {
				m.errorMsg = "DB refresh failed: " + msg.err.Error()
			}
			return m, cmd
		}
		m.addonDB = msg.entries
		m.browseDBIndices = m.browseCurIndices()
		// Update status for WoWInterface-installed addons from the loaded DB.
		wowiIdx := make(map[int]AddonDBEntry, len(m.addonDB))
		for _, e := range m.addonDB {
			if e.WoWInterfaceID > 0 {
				wowiIdx[e.WoWInterfaceID] = e
			}
		}
		for i, aws := range m.addonsWithStatus {
			id := wowiIDFromKey(aws.Addon.GithubRepo)
			if id == 0 {
				continue
			}
			e, ok := wowiIdx[id]
			if !ok || e.LatestVersion == "" {
				continue
			}
			m.addonsWithStatus[i].LatestVersion = e.LatestVersion
			m.addonsWithStatus[i].LatestDate = e.LatestDate
			if aws.Addon.InstalledVersion == "" {
				m.addonsWithStatus[i].Status = StatusNotInstalled
			} else if normalizeVersion(aws.Addon.InstalledVersion) == normalizeVersion(e.LatestVersion) {
				m.addonsWithStatus[i].Status = StatusUpToDate
			} else {
				m.addonsWithStatus[i].Status = StatusUpdateAvail
			}
		}
		return m, cmd

	case rssLoadedMsg:
		if msg.feedType == "hot" {
			m.rssHotLoaded = true
			if msg.err == nil {
				m.hotIDs = msg.ids
			}
		} else {
			m.rssNewLoaded = true
			if msg.err == nil {
				m.latestIDs = msg.ids
			}
		}
		// Recompute browse indices if we're on the affected tab.
		if (msg.feedType == "hot" && m.browseTab == "hot") ||
			(msg.feedType == "new" && m.browseTab == "new") {
			m.browseDBIndices = m.browseCurIndices()
			m.browseDBCursor = 0
		}
		return m, cmd

	case progress.FrameMsg:
		var progressCmd tea.Cmd
		m.progressBar, progressCmd = m.progressBar.Update(msg)
		return m, progressCmd

	case tea.MouseReleaseMsg:
		if msg.Button == tea.MouseLeft && !m.loading {
			return m.handleMouseClick(msg)
		}
		return m, cmd

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)

	}

	// 5. Mode if-chain (most specific first)
	if m.viewBrowseDetail {
		return m.handleBrowseDetail(msg)
	}

	if m.browseInstallConfirm {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleBrowseInstallConfirm(keyMsg)
		}
		return m, cmd
	}

	if m.inputAddRepo {
		return m.handleAddRepoInput(msg)
	}

	if m.addRepoConfirm {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleAddRepoConfirm(keyMsg)
		}
		return m, cmd
	}

	if m.confirmDelete {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleConfirmDelete(keyMsg)
		}
		return m, cmd
	}

	if m.viewAddonDetail {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleAddonDetail(keyMsg)
		}
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		return m, tea.Batch(cmd, vpCmd)
	}

	if m.inputNewProfile {
		return m.handleNewProfileInput(msg)
	}

	if m.selectModeProfileAddons {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleProfileAddonSelect(keyMsg)
		}
		return m, cmd
	}

	if m.viewProfileDetail {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleProfileDetail(keyMsg)
		}
		return m, cmd
	}

	if m.viewProfiles {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleProfileList(keyMsg)
		}
		return m, cmd
	}

	if m.inputSettingsRetail {
		return m.handleSettingsRetailInput(msg)
	}

	if m.inputSettingsClassic {
		return m.handleSettingsClassicInput(msg)
	}

	if m.inputSettingsToken {
		return m.handleSettingsTokenInput(msg)
	}

	if m.viewSettings {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleSettings(keyMsg)
		}
		return m, cmd
	}

	if m.confirmQuit {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleQuitConfirm(keyMsg)
		}
		return m, cmd
	}

	// Default: dashboard
	if !m.loading {
		return m.handleDashboard(msg)
	}

	return m, cmd
}

// downloadTick fires a downloadTickMsg after 150ms to animate the fake progress bar.
func downloadTick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
		return downloadTickMsg{}
	})
}

// autoCheckTick fires an autoCheckTickMsg after 30 minutes for periodic update checks.
func autoCheckTick() tea.Cmd {
	return tea.Tick(30*time.Minute, func(time.Time) tea.Msg {
		return autoCheckTickMsg{}
	})
}

// startNextQueuedUpdate dispatches the next update in the queue.
// WoWInterface addons are installed directly from the DB without a GitHub fetch.
func (m model) startNextQueuedUpdate() tea.Cmd {
	if m.updateQueueIdx >= len(m.updateQueue) {
		return nil
	}
	repo := m.updateQueue[m.updateQueueIdx]
	if id := wowiIDFromKey(repo); id > 0 {
		for _, e := range m.addonDB {
			if e.WoWInterfaceID == id {
				release := wowiMakeRelease(e)
				flavor := "retail"
				ea := e.ExtractAs
				for _, a := range m.config.Addons {
					if a.GithubRepo == repo {
						flavor = a.GameFlavor
						if a.ExtractAs != "" {
							ea = a.ExtractAs
						}
						break
					}
				}
				path := addonPath(m.config, flavor)
				return tea.Batch(installAddon(repo, release, path, "", ea), downloadTick())
			}
		}
		return nil
	}
	return fetchLatestRelease(repo, m.config.GithubToken)
}

// nextBrowseInstallCmd returns the install command for the current browse queue item.
func (m model) nextBrowseInstallCmd() tea.Cmd {
	if m.browseInstallIdx >= len(m.browseInstallQueue) {
		return nil
	}
	repo := m.browseInstallQueue[m.browseInstallIdx]
	if id := wowiIDFromKey(repo); id > 0 {
		for _, e := range m.addonDB {
			if e.WoWInterfaceID == id {
				release := wowiMakeRelease(e)
				path := addonPath(m.config, m.browseInstallFlavor)
				ea := addonExtractAs(repo, m.config.Addons, m.addonDB)
				return tea.Batch(installAddon(repo, release, path, "", ea), downloadTick())
			}
		}
		return nil
	}
	return fetchLatestRelease(repo, m.config.GithubToken)
}
