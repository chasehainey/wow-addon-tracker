package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	lgtable "charm.land/lipgloss/v2/table"
	zone "github.com/lrstanley/bubblezone/v2"
)

// watLogo is the 6-line block-art logo shown in the header on large terminals.
var watLogo = []string{
	" ██╗    ██╗ █████╗ ████████╗",
	" ██║    ██║██╔══██╗╚══██╔══╝",
	" ██║ █╗ ██║███████║   ██║   ",
	" ██║███╗██║██╔══██║   ██║   ",
	" ╚███╔███╔╝██║  ██║   ██║   ",
	"  ╚══╝╚══╝ ╚═╝  ╚═╝   ╚═╝  ",
}

func (m model) View() tea.View {
	return tea.View{
		Content:   zone.Scan(m.render()),
		AltScreen: true,
		MouseMode: tea.MouseModeCellMotion,
	}
}

func (m model) render() string {
	var sb strings.Builder

	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	if m.loading && !m.installing && !m.updatingSingle && !m.updatingAll && !m.checkingUpdates && !m.addRepoFetching {
		sb.WriteString(fmt.Sprintf("  %s Loading...\n", m.spinner.View()))
		sb.WriteString(m.renderFooter(nil))
		return sb.String()
	}

	switch {
	case m.browseInstallConfirm:
		sb.WriteString(m.renderBrowseInstallConfirm())
	case m.inputAddRepo:
		sb.WriteString(m.renderAddRepoInput())
	case m.addRepoConfirm:
		sb.WriteString(m.renderAddRepoConfirm())
	case m.installing || m.updatingSingle:
		sb.WriteString(m.renderInstallProgress())
	case m.confirmDelete:
		sb.WriteString(m.renderConfirmDelete())
	case m.viewAddonDetail:
		sb.WriteString(m.renderAddonDetail())
	case m.updatingAll:
		sb.WriteString(m.renderUpdatingAll())
	case m.inputNewProfile:
		sb.WriteString(m.renderNewProfileInput())
	case m.selectModeProfileAddons:
		sb.WriteString(m.renderProfileAddonSelect())
	case m.viewProfileDetail:
		sb.WriteString(m.renderProfileDetail())
	case m.viewProfiles:
		sb.WriteString(m.renderProfileList())
	case m.inputSettingsRetail || m.inputSettingsClassic || m.inputSettingsToken:
		sb.WriteString(m.renderSettingsInput())
	case m.viewSettings:
		sb.WriteString(m.renderSettings())
	default:
		sb.WriteString(m.renderDashboard())
	}

	// Determine which keymap to show in the footer
	var km help.KeyMap
	switch {
	case m.viewAddonDetail:
		km = addonDetailKeys
	case m.viewProfiles && !m.viewProfileDetail && !m.inputNewProfile:
		km = profileListKeys
	case m.selectModeProfileAddons:
		km = profileAddonKeys
	case m.viewSettings:
		km = settingsKeys
	case m.inputAddRepo, m.inputSettingsRetail, m.inputSettingsClassic, m.inputSettingsToken, m.inputNewProfile:
		km = inputKeys
	case m.inDashboard():
		km = dashboardKeys
	}
	sb.WriteString(m.renderFooter(km))
	return sb.String()
}

// ── Header ───────────────────────────────────────────────────────────────────

func (m model) renderHeader() string {
	s := m.getStyles()
	w := m.terminalWidth
	if w < 20 {
		w = 80
	}

	// Build stats string shared between both modes
	var statsStr string
	if len(m.config.Addons) > 0 {
		updates := 0
		for _, aws := range m.addonsWithStatus {
			if aws.Status == StatusUpdateAvail {
				updates++
			}
		}
		statsStr = s.Muted.Render(fmt.Sprintf("%d addons", len(m.config.Addons)))
		if updates > 0 {
			statsStr += "  " + s.StatusWarn.Render(fmt.Sprintf("·  %d update%s available", updates, pluralS(updates)))
		}
	}
	if m.checkingUpdates {
		statsStr += "  " + s.Muted.Render(m.spinner.View()+" checking...")
	}

	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Border)).
		Render("  " + strings.Repeat("─", max(0, w-4)))

	if m.terminalHeight >= 26 {
		// Large terminal: 6-line block-art WAT logo.
		// "WoW Addon Tracker" appears inline on the last logo line; stats right-aligned.
		logoStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Primary))
		subtitleSty := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Secondary))
		var out strings.Builder
		// Lines 0–4: plain logo
		for _, line := range watLogo[:5] {
			out.WriteString(logoStyle.Render(line) + "\n")
		}
		// Line 5: last logo glyph + subtitle inline + right-aligned stats
		lastLogo := logoStyle.Render(watLogo[5])
		subtitle := subtitleSty.Render("  WoW Addon Tracker")
		logoW := lipgloss.Width(lastLogo)
		subW := lipgloss.Width(subtitle)
		statsW := lipgloss.Width(statsStr)
		gap := w - logoW - subW - statsW - 2
		if gap < 1 {
			gap = 1
		}
		out.WriteString(lastLogo + subtitle + strings.Repeat(" ", gap) + statsStr + "\n")
		out.WriteString(divider)
		return out.String()
	}

	// Compact header for small terminals
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Primary)).
		Render("⚔  W·A·T — WoW Addon Tracker")
	titleWidth := lipgloss.Width(title)
	statsWidth := lipgloss.Width(statsStr)
	gap := w - 2 - titleWidth - statsWidth - 2
	if gap < 1 {
		gap = 1
	}
	return "  " + title + strings.Repeat(" ", gap) + statsStr + "\n" + divider
}

// ── Footer ───────────────────────────────────────────────────────────────────

func (m model) renderFooter(km help.KeyMap) string {
	var sb strings.Builder
	s := m.getStyles()

	sb.WriteString("\n")
	if m.errorMsg != "" {
		sb.WriteString(s.Error.Render("  ✗  "+m.errorMsg) + "\n")
	}
	if m.successMsg != "" {
		sb.WriteString(s.Success.Render("  ✓  "+m.successMsg) + "\n")
	}

	if km != nil {
		h := help.New()
		h.SetWidth(m.terminalWidth - 4)
		sb.WriteString(s.Muted.Render("  ") + h.View(km) + "\n")
	}
	return sb.String()
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (m model) renderDashboard() string {
	w := m.terminalWidth
	if w < 40 {
		w = 80
	}
	h := m.terminalHeight
	if h < 15 {
		h = 24
	}

	// Content overhead: large header (h>=26) = 7 header + 1 blank + 2 footer = 10
	// Small header = 2 header + 1 blank + 2 footer = 5
	overhead := 5
	if h >= 26 {
		overhead = 10
	}
	contentH := h - overhead
	if contentH < 10 {
		contentH = 10
	}

	// Sidebar width
	sidebarOuterW := 26
	if w > 110 {
		sidebarOuterW = 30
	}
	leftOuterW := w - sidebarOuterW
	if leftOuterW < 30 {
		leftOuterW = 30
	}

	// The tab row above the installed panel occupies 2 lines (border top + content).
	// Subtract those from the available height before splitting installed vs browse.
	const tabRowH = 2
	availH := contentH - tabRowH
	if availH < 8 {
		availH = 8
	}
	installedOuterH := availH / 3
	if installedOuterH < 4 {
		installedOuterH = 4
	}
	browseOuterH := availH - installedOuterH
	if browseOuterH < 4 {
		browseOuterH = 4
	}

	tabs := m.renderFlavorTabs(leftOuterW)
	installed := m.renderInstalledPanel(leftOuterW, installedOuterH)
	browse := m.renderBrowsePanel(leftOuterW, browseOuterH)
	left := lipgloss.JoinVertical(lipgloss.Top, tabs, installed, browse)
	sidebar := m.renderSidebar(sidebarOuterW, contentH)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sidebar) + "\n"
}

// renderFlavorTabs renders a 2-line tab strip (Retail | Classic) above the
// installed panel.  The active tab has a rounded top border with no bottom so
// it visually connects to the panel below; the inactive tab is text-only.
func (m model) renderFlavorTabs(w int) string {
	// Active tab: rounded top + sides, NO bottom border (open-bottom)
	activeSty := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.theme.Accent)).
		Border(lipgloss.RoundedBorder(), true, true, false, true).
		BorderForeground(lipgloss.Color(m.theme.Accent)).
		Padding(0, 1)
	// Inactive tab: plain text, muted, bottom-aligned with content row
	inactiveSty := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Muted)).
		Padding(0, 2)

	var retailSty, classicSty lipgloss.Style
	if m.defaultFlavor == "retail" {
		retailSty = activeSty
		classicSty = inactiveSty
	} else {
		retailSty = inactiveSty
		classicSty = activeSty
	}

	retailTab := zone.Mark("tab-retail", retailSty.Render("Retail"))
	classicTab := zone.Mark("tab-classic", classicSty.Render("Classic"))

	// Bottom-align so the content rows of both tabs sit on the same line
	return lipgloss.JoinHorizontal(lipgloss.Bottom, retailTab, classicTab)
}

func (m model) renderInstalledPanel(outerW, outerH int) string {
	s := m.getStyles()
	innerW := outerW - 2
	innerH := outerH - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	borderColor := m.theme.Border
	if m.dashboardFocus == "installed" {
		borderColor = m.theme.Accent
	}

	count := len(m.config.Addons)

	// Column widths: Name | Creator | Version | Status
	// Fixed: cursor(1) + sep(2) + sep(2) + cCreator + sep(2) + cVer + sep(2) + status(1) = 10+cCreator+cVer
	const cVer = 10
	const cCreator = 14
	cNm := innerW - 10 - cCreator - cVer
	if cNm > 26 {
		cNm = 26
	}
	if cNm < 8 {
		cNm = 8
	}

	var lines []string

	// Optional filter line
	if m.addonFilterActive {
		lines = append(lines, " Filter: "+m.textInputAddonFilter.View())
	} else if m.addonListFilter != "" {
		lines = append(lines, s.Muted.Render(fmt.Sprintf(" Filter: %q  (%d)", m.addonListFilter, len(m.addonFilteredIndices))))
	}

	// Column header row
	lines = append(lines, s.Muted.Render(
		" "+lipgloss.NewStyle().Width(cNm).Render("Name")+
			"  "+lipgloss.NewStyle().Width(cCreator).Render("Creator")+
			"  "+lipgloss.NewStyle().Width(cVer).Render("Version")+
			"  Status"))

	headerLines := len(lines)
	listH := innerH - headerLines
	if listH < 0 {
		listH = 0
	}

	if count == 0 {
		lines = append(lines, s.Muted.Render(" No addons tracked. Press [a] to add."))
	} else if len(m.addonFilteredIndices) == 0 {
		lines = append(lines, s.Muted.Render(" No addons match filter."))
	} else {
		start := m.addonListCursor - listH/2
		if start < 0 {
			start = 0
		}
		end := start + listH
		if end > len(m.addonFilteredIndices) {
			end = len(m.addonFilteredIndices)
			start = end - listH
			if start < 0 {
				start = 0
			}
		}

		for listPos := start; listPos < end; listPos++ {
			addonIdx := m.addonFilteredIndices[listPos]
			aws := m.addonsWithStatus[addonIdx]

			// Split GithubRepo into creator and addon name.
			creator := ""
			addonName := aws.Addon.Name
			if parts := strings.SplitN(aws.Addon.GithubRepo, "/", 2); len(parts) == 2 {
				creator = parts[0]
				addonName = parts[1]
			}

			name := truncateStr(addonName, cNm)
			cre := truncateStr(creator, cCreator)
			ver := truncateStr(displayVersion(aws.Addon.InstalledVersion), cVer)
			if ver == "" {
				ver = "—"
			}

			var stsGlyph, stsColor string
			switch aws.Status {
			case StatusUpToDate:
				stsGlyph, stsColor = "✓", m.theme.Success
			case StatusUpdateAvail:
				stsGlyph, stsColor = "↑", m.theme.Warning
			case StatusNotInstalled:
				stsGlyph, stsColor = "✗", m.theme.Error
			default:
				stsGlyph, stsColor = "?", m.theme.Muted
			}
			stsStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(stsColor)).Render(stsGlyph)

			// Selected row: name+creator+ver highlighted, status keeps its own color
			var line string
			if listPos == m.addonListCursor {
				line = s.SelectedItem.Render(
					">"+lipgloss.NewStyle().Width(cNm).Render(name)+
						"  "+lipgloss.NewStyle().Width(cCreator).Render(cre)+
						"  "+lipgloss.NewStyle().Width(cVer).Render(ver)) +
					"  " + stsStyled
			} else {
				line = " " +
					lipgloss.NewStyle().Width(cNm).Render(name) +
					"  " + s.Muted.Width(cCreator).Render(cre) +
					"  " + s.Value.Width(cVer).Render(ver) +
					"  " + stsStyled
			}
			lines = append(lines, zone.Mark(fmt.Sprintf("inst-row-%d", listPos), line))
		}
	}

	// Pad / clip to innerH
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(innerW).
		Height(innerH).
		Render(content)
}

func (m model) renderBrowsePanel(outerW, outerH int) string {
	s := m.getStyles()
	innerW := outerW - 2
	innerH := outerH - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	borderColor := m.theme.Border
	if m.dashboardFocus == "browse" {
		borderColor = m.theme.Accent
	}

	count := len(m.addonDB)

	var lines []string
	// Title line
	titleText := fmt.Sprintf(" Browse (%s) ", formatCount(count))
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Secondary)).Render(titleText))

	// Filter line
	if m.browseDBFilterActive {
		lines = append(lines, " Filter: "+m.textInputBrowseFilter.View())
	} else if m.browseDBFilter != "" {
		lines = append(lines, s.Muted.Render(fmt.Sprintf(" Filter: %q  (%d)", m.browseDBFilter, len(m.browseDBIndices))))
	}

	// Selected count
	if sel := len(m.browseDBSelected); sel > 0 {
		lines = append(lines, s.Success.Render(fmt.Sprintf(" %d selected — enter to install", sel)))
	}

	headerLines := len(lines)
	listH := innerH - headerLines
	if listH < 0 {
		listH = 0
	}

	if count == 0 {
		lines = append(lines, s.Muted.Render(" No database loaded."))
	} else if len(m.browseDBIndices) == 0 {
		lines = append(lines, s.Muted.Render(" No addons match filter."))
	} else {
		// Column widths: " " + cb(3) + "  " + name(nameW) + "  " + creator(creatorW) + "  " + desc(descW)
		// fixed overhead = 1 + 3 + 2 + 2 + 2 = 10
		const bCreatorW = 14
		available := innerW - 10 - bCreatorW - 2
		if available < 10 {
			available = 10
		}
		nameW := available * 2 / 5
		if nameW > 24 {
			nameW = 24
		}
		if nameW < 10 {
			nameW = 10
		}
		descW := available - nameW
		if descW < 0 {
			descW = 0
		}

		// Column header row
		hdr := s.Muted.Render(
			"    " + lipgloss.NewStyle().Width(nameW).Render("Name") +
				"  " + lipgloss.NewStyle().Width(bCreatorW).Render("Creator") +
				"  " + "Description")
		lines = append(lines, hdr)
		listH--
		if listH < 0 {
			listH = 0
		}

		start := m.browseDBCursor - listH/2
		if start < 0 {
			start = 0
		}
		end := start + listH
		if end > len(m.browseDBIndices) {
			end = len(m.browseDBIndices)
			start = end - listH
			if start < 0 {
				start = 0
			}
		}

		for listPos := start; listPos < end; listPos++ {
			idx := m.browseDBIndices[listPos]
			e := m.addonDB[idx]
			_, isSelected := m.browseDBSelected[idx]
			checkbox := "[ ]"
			if isSelected {
				checkbox = "[✓]"
			}

			// Split "Creator/RepoName" into separate columns.
			addonName := e.Repo
			creator := ""
			if parts := strings.SplitN(e.Repo, "/", 2); len(parts) == 2 {
				creator = parts[0]
				addonName = parts[1]
			}
			name := truncateStr(addonName, nameW)
			cre := truncateStr(creator, bCreatorW)
			desc := truncateStr(e.Description, descW)

			var line string
			if listPos == m.browseDBCursor {
				row := ">" + lipgloss.NewStyle().Width(3).Render(checkbox) +
					"  " + lipgloss.NewStyle().Width(nameW).Render(name) +
					"  " + lipgloss.NewStyle().Width(bCreatorW).Render(cre)
				if descW > 0 {
					row += "  " + lipgloss.NewStyle().Width(descW).Render(desc)
				}
				line = s.SelectedItem.Render(row)
			} else {
				cbStyle := s.Muted
				if isSelected {
					cbStyle = s.Success
				}
				line = " " + cbStyle.Width(3).Render(checkbox) +
					"  " + s.Value.Width(nameW).Render(name) +
					"  " + s.Muted.Width(bCreatorW).Render(cre)
				if descW > 0 {
					line += "  " + s.Muted.Width(descW).Render(desc)
				}
			}
			lines = append(lines, zone.Mark(fmt.Sprintf("browse-row-%d", listPos), line))
		}
	}

	// Pad / clip to innerH
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(innerW).
		Height(innerH).
		Render(content)
}

func (m model) renderSidebar(outerW, outerH int) string {
	s := m.getStyles()
	innerW := outerW - 2
	innerH := outerH - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	div := s.Muted.Render(" " + strings.Repeat("─", max(0, innerW-1)))

	var lines []string

	// Stats
	installed := len(m.config.Addons)
	updates := 0
	for _, aws := range m.addonsWithStatus {
		if aws.Status == StatusUpdateAvail {
			updates++
		}
	}
	flavorLabel := "Retail"
	if m.defaultFlavor == "classic" {
		flavorLabel = "Classic"
	}
	lines = append(lines, s.Label.Render(" Game: ")+s.Value.Render(flavorLabel))
	lines = append(lines, s.Muted.Render(fmt.Sprintf(" %d installed", installed)))
	if updates > 0 {
		lines = append(lines, s.StatusWarn.Render(fmt.Sprintf(" %d update%s avail", updates, pluralS(updates))))
	} else if installed > 0 {
		lines = append(lines, s.StatusOK.Render(" all up to date"))
	}
	lines = append(lines, div)

	// Action buttons
	lines = append(lines, s.Label.Render(" Actions"))
	type action struct{ key, label string }
	actions := []action{
		{"c", "Check updates"},
		{"U", "Update all"},
		{"a", "Add addon"},
		{"r", "Refresh DB"},
		{"p", "Profiles"},
		{"s", "Settings"},
		{"q", "Quit"},
	}
	for i, act := range actions {
		label := truncateStr(fmt.Sprintf(" [%s] %s", act.key, act.label), innerW)
		lines = append(lines, zone.Mark(fmt.Sprintf("sidebar-action-%d", i), s.MenuItem.Render(label)))
	}

	// Pad / clip to innerH
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Border)).
		Width(innerW).
		Render(content)
}

// ── Addon list ───────────────────────────────────────────────────────────────

func (m model) renderAddonList() string {
	s := m.getStyles()
	var sb strings.Builder

	count := len(m.addonFilteredIndices)
	header := fmt.Sprintf("  Addons (%d)", len(m.config.Addons))
	if m.addonListFilter != "" {
		header += fmt.Sprintf(" — %d match", count)
		if count != 1 {
			header += "es"
		}
	}
	sb.WriteString(s.Label.Render(header) + "\n")

	if m.addonFilterActive {
		sb.WriteString(fmt.Sprintf("  Filter: %s\n", m.textInputAddonFilter.View()))
	} else if m.addonListFilter != "" {
		sb.WriteString(s.Muted.Render(fmt.Sprintf("  Filter: %q", m.addonListFilter)) + "\n")
	}
	sb.WriteString("\n")

	if len(m.config.Addons) == 0 {
		sb.WriteString(s.Muted.Render("  No addons tracked yet.") + "\n")
		sb.WriteString(s.Info.Render("  Press [a] to add your first addon.") + "\n")
		return sb.String()
	}
	if len(m.addonFilteredIndices) == 0 {
		sb.WriteString(s.Muted.Render("  No addons match the filter.") + "\n")
		return sb.String()
	}

	// Column widths — name column fills available terminal space.
	const cSt, cIn, cLt, cFl = 3, 12, 12, 7
	// 2 (indent) + cSt + 4×2 (separators) + cIn + cLt + cFl = 2+3+8+12+12+7 = 44
	w := m.terminalWidth
	if w <= 0 {
		w = 80
	}
	cNm := w - 44
	if cNm < 16 {
		cNm = 16
	}
	if cNm > 40 {
		cNm = 40
	}

	// Header row
	hdr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Secondary))
	colHdr := "  " +
		hdr.Width(cSt).Render("ST") + "  " +
		hdr.Width(cNm).Render("Name") + "  " +
		hdr.Width(cIn).Render("Installed") + "  " +
		hdr.Width(cLt).Render("Latest") + "  " +
		hdr.Width(cFl).Render("Flavor")
	sb.WriteString(colHdr + "\n")
	divLine := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Border)).
		Render("  " + strings.Repeat("─", cSt+cNm+cIn+cLt+cFl+8))
	sb.WriteString(divLine + "\n")

	for listPos, addonIdx := range m.addonFilteredIndices {
		aws := m.addonsWithStatus[addonIdx]

		sym, symColor := statusGlyph(aws.Status, m.theme)
		symCell := lipgloss.NewStyle().Foreground(lipgloss.Color(symColor)).Width(cSt).Render(sym)

		installed := truncateStr(displayVersion(aws.Addon.InstalledVersion), cIn)
		if installed == "" {
			installed = "—"
		}
		// Use runtime LatestVersion when available; fall back to persisted value.
		latestVer := aws.LatestVersion
		if latestVer == "" {
			latestVer = aws.Addon.LatestVersion
		}
		latest := ""
		if aws.Status == StatusUpdateAvail && latestVer != "" {
			latest = truncateStr("→ "+displayVersion(latestVer), cLt)
		}

		name := truncateStr(aws.Addon.Name, cNm)

		var lineStr string
		if listPos == m.addonListCursor {
			lineStr = s.SelectedItem.Render(
				"> " +
					lipgloss.NewStyle().Width(cSt).Render(sym) + "  " +
					lipgloss.NewStyle().Width(cNm).Render(name) + "  " +
					lipgloss.NewStyle().Width(cIn).Render(installed) + "  " +
					lipgloss.NewStyle().Width(cLt).Render(latest) + "  " +
					lipgloss.NewStyle().Width(cFl).Render(aws.Addon.GameFlavor),
			)
		} else {
			nameCell := lipgloss.NewStyle().Width(cNm).Render(name)
			installedCell := s.Value.Width(cIn).Render(installed)
			latestCell := s.StatusWarn.Width(cLt).Render(latest)
			flavorCell := s.Muted.Width(cFl).Render(aws.Addon.GameFlavor)
			lineStr = "  " + symCell + "  " + nameCell + "  " + installedCell + "  " + latestCell + "  " + flavorCell
		}
		sb.WriteString(zone.Mark(fmt.Sprintf("addon-%d", listPos), lineStr) + "\n")
	}
	sb.WriteString(divLine + "\n")
	return sb.String()
}

// ── Addon detail ─────────────────────────────────────────────────────────────

func (m model) renderAddonDetail() string {
	s := m.getStyles()
	var sb strings.Builder

	if m.selectedAddonIdx >= len(m.config.Addons) {
		return s.Error.Render("  Invalid selection") + "\n"
	}

	addon := m.config.Addons[m.selectedAddonIdx]
	var aws AddonWithStatus
	if m.selectedAddonIdx < len(m.addonsWithStatus) {
		aws = m.addonsWithStatus[m.selectedAddonIdx]
	} else {
		aws = AddonWithStatus{Addon: addon, Status: StatusUnknown}
	}

	sb.WriteString(s.Highlight.Render("  "+addon.Name) + "  " + s.Muted.Render(addon.GithubRepo) + "\n\n")

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Secondary))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Highlight))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))

	tableWidth := m.terminalWidth - 8
	if tableWidth < 40 {
		tableWidth = 40
	}

	// Build rows
	type row struct{ label, value string }
	rows := []row{
		{"Repository", addon.GithubRepo},
		{"Flavor", addon.GameFlavor},
	}
	if addon.InstalledVersion != "" {
		iv := displayVersion(addon.InstalledVersion)
		if addon.InstalledDate != "" {
			iv += "  ·  " + formatDate(addon.InstalledDate)
		}
		rows = append(rows, row{"Installed", iv})
	} else {
		rows = append(rows, row{"Installed", "not installed"})
	}
	// Use runtime value when available; fall back to persisted value after restart.
	detailLatestVer := aws.LatestVersion
	if detailLatestVer == "" {
		detailLatestVer = addon.LatestVersion
	}
	detailLatestDate := aws.LatestDate
	if detailLatestDate == "" {
		detailLatestDate = addon.LatestDate
	}
	if detailLatestVer != "" {
		lv := displayVersion(detailLatestVer)
		if detailLatestDate != "" {
			lv += "  ·  " + formatDate(detailLatestDate)
		}
		rows = append(rows, row{"Latest", lv})
	}
	if len(addon.Directories) > 0 {
		rows = append(rows, row{"Folders", strings.Join(addon.Directories, ",  ")})
	}
	if len(addon.Profiles) > 0 {
		rows = append(rows, row{"Profiles", strings.Join(addon.Profiles, ",  ")})
	}

	// Status row
	switch aws.Status {
	case StatusUpToDate:
		rows = append(rows, row{"Status", "✓  Up to date"})
	case StatusUpdateAvail:
		rows = append(rows, row{"Status", fmt.Sprintf("!  Update: %s  →  %s", displayVersion(addon.InstalledVersion), displayVersion(detailLatestVer))})
	case StatusNotInstalled:
		rows = append(rows, row{"Status", "✗  Not installed"})
	default:
		rows = append(rows, row{"Status", "?  Unknown"})
	}

	// Render as lipgloss table
	t := lgtable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Border))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == 0 {
				return labelStyle
			}
			return valueStyle
		}).
		Width(tableWidth)

	for _, r := range rows {
		t.Row(r.label, r.value)
	}
	for _, line := range strings.Split(t.Render(), "\n") {
		sb.WriteString("  " + line + "\n")
	}

	// Changelog / release notes
	if addon.Changelog != "" {
		sb.WriteString("\n")
		sb.WriteString("  " + labelStyle.Render("Changelog") + "\n")
		lines := strings.Split(strings.TrimRight(addon.Changelog, "\n\r"), "\n")
		// How many lines fit below: header(2) + name(2) + table(~12) + this header(2) + actions(2)
		maxLines := m.terminalHeight - 20
		if maxLines < 3 {
			maxLines = 3
		}
		if maxLines > 12 {
			maxLines = 12
		}
		shown := lines
		truncated := false
		if len(lines) > maxLines {
			shown = lines[:maxLines]
			truncated = true
		}
		for _, ln := range shown {
			ln = truncateStr(strings.TrimRight(ln, "\r"), m.terminalWidth-6)
			sb.WriteString("  " + mutedStyle.Render(ln) + "\n")
		}
		if truncated {
			sb.WriteString("  " + mutedStyle.Render(fmt.Sprintf("… %d more lines", len(lines)-maxLines)) + "\n")
		}
	}

	sb.WriteString("\n")
	var actionParts []string
	if aws.Status != StatusUpToDate {
		actionParts = append(actionParts, zone.Mark("detail-update", s.StatusWarn.Render("[u] Update")))
	}
	actionParts = append(actionParts,
		zone.Mark("detail-delete", s.Error.Render("[d] Delete")),
		mutedStyle.Render("esc back"),
	)
	sb.WriteString("  " + strings.Join(actionParts, mutedStyle.Render("  ·  ")) + "\n")
	return sb.String()
}

// ── Add addon input ───────────────────────────────────────────────────────────

func (m model) renderAddRepoInput() string {
	s := m.getStyles()
	var sb strings.Builder

	sb.WriteString(s.Label.Render("  Add Addon") + "\n\n")
	sb.WriteString(s.Info.Render("  GitHub repository (Owner/Repo):") + "\n\n")
	sb.WriteString("  " + m.textInputRepo.View() + "\n\n")

	if len(m.dbSuggestions) > 0 {
		sb.WriteString(s.Muted.Render("  Suggestions — Tab to cycle, click to select:") + "\n")

		const cRp, cSt, cLn = 42, 8, 12
		for i, e := range m.dbSuggestions {
			stars := "★ " + formatStars(e.Stars)
			lang := e.Language
			if lang == "" {
				lang = "?"
			}
			desc := e.Description
			if len(desc) > 45 {
				desc = desc[:42] + "..."
			}

			var lineStr string
			if i == m.dbSuggestionIdx {
				lineStr = s.SelectedItem.Render(
					"> " +
						lipgloss.NewStyle().Width(cRp).MaxWidth(cRp).Render(e.Repo) + "  " +
						lipgloss.NewStyle().Width(cSt).MaxWidth(cSt).Render(stars) + "  " +
						lipgloss.NewStyle().Width(cLn).MaxWidth(cLn).Render(lang) + "  " +
						desc,
				)
			} else {
				lineStr = s.Muted.Render(
					"  " +
						lipgloss.NewStyle().Width(cRp).MaxWidth(cRp).Render(e.Repo) + "  " +
						lipgloss.NewStyle().Width(cSt).MaxWidth(cSt).Render(stars) + "  " +
						lipgloss.NewStyle().Width(cLn).MaxWidth(cLn).Render(lang) + "  " +
						desc,
				)
			}
			sb.WriteString(zone.Mark(fmt.Sprintf("suggest-%d", i), lineStr) + "\n")
		}
		sb.WriteString("\n")
	}

	if m.addRepoFetching {
		sb.WriteString(fmt.Sprintf("  %s Fetching release info...\n", m.spinner.View()))
	}
	return sb.String()
}

// ── Add addon confirm ─────────────────────────────────────────────────────────

func (m model) renderAddRepoConfirm() string {
	s := m.getStyles()
	var sb strings.Builder

	if m.pendingRelease == nil {
		return s.Error.Render("  No release data") + "\n"
	}
	rel := m.pendingRelease

	sb.WriteString(s.Label.Render("  Confirm Install") + "\n\n")

	boxWidth := m.terminalWidth - 8
	if boxWidth < 40 {
		boxWidth = 40
	}
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Border)).
		Padding(0, 1).
		Width(boxWidth)

	var content strings.Builder
	content.WriteString(s.Highlight.Render(m.pendingRepo) + "  " + s.Value.Render(rel.TagName) + "\n")
	if rel.PublishedAt != "" {
		content.WriteString(s.Muted.Render("Released: "+formatDate(rel.PublishedAt)) + "\n")
	}

	if len(rel.Assets) > 0 {
		content.WriteString("\n" + s.Label.Render("Assets:") + "\n")
		for _, a := range rel.Assets {
			content.WriteString("  " + s.Value.Render(a.Name) + "  " + s.Muted.Render(formatBytes(a.Size)) + "\n")
		}
	}

	if rel.Body != "" {
		content.WriteString("\n" + s.Label.Render("Release Notes:") + "\n")
		lines := strings.Split(rel.Body, "\n")
		maxLines := 6
		for i, line := range lines {
			if i >= maxLines {
				content.WriteString(s.Muted.Render("  ...") + "\n")
				break
			}
			content.WriteString(s.Muted.Render("  "+line) + "\n")
		}
	}

	sb.WriteString(boxStyle.Render(content.String()) + "\n\n")

	// Flavor selector
	var retailLabel, classicLabel string
	if m.pendingFlavor == "retail" {
		retailLabel = zone.Mark("flavor-retail", s.SelectedItem.Render("[Retail]"))
		classicLabel = zone.Mark("flavor-classic", s.Muted.Render("Classic"))
	} else {
		retailLabel = zone.Mark("flavor-retail", s.Muted.Render("Retail"))
		classicLabel = zone.Mark("flavor-classic", s.SelectedItem.Render("[Classic]"))
	}
	sb.WriteString(fmt.Sprintf("  Game: %s  %s\n\n", retailLabel, classicLabel))

	installBtn := zone.Mark("action-install", s.Success.Render("[ Install ]"))
	cancelBtn := zone.Mark("action-cancel", s.Error.Render("[ Cancel ]"))
	sb.WriteString(fmt.Sprintf("  %s  %s\n", installBtn, cancelBtn))
	sb.WriteString(s.Muted.Render("  Tab toggle retail/classic  ·  Esc cancel") + "\n")
	return sb.String()
}

// ── Install progress ─────────────────────────────────────────────────────────

func (m model) renderInstallProgress() string {
	s := m.getStyles()
	var sb strings.Builder

	repo := m.pendingRepo
	if m.updatingAll && m.updateQueueIdx < len(m.updateQueue) {
		repo = m.updateQueue[m.updateQueueIdx]
	}
	if m.browseInstalling && m.browseInstallIdx < len(m.browseInstallQueue) {
		repo = m.browseInstallQueue[m.browseInstallIdx]
	}
	if repo == "" && m.selectedAddonIdx < len(m.config.Addons) {
		repo = m.config.Addons[m.selectedAddonIdx].GithubRepo
	}

	action := "Installing"
	if m.updatingSingle || (m.updatingAll && m.updateQueueIdx < len(m.updateQueue)) {
		action = "Updating"
	}

	sb.WriteString(fmt.Sprintf("  %s %s %s...\n\n",
		m.spinner.View(),
		s.Info.Render(action),
		s.Value.Render(repo),
	))

	pbWidth := m.terminalWidth - 8
	if pbWidth < 20 {
		pbWidth = 20
	}
	if pbWidth > 60 {
		pbWidth = 60
	}

	pb := m.progressBar
	pb.SetWidth(pbWidth)
	if m.updatingAll && len(m.updateQueue) > 0 {
		done := m.updateQueueIdx
		total := len(m.updateQueue)
		pct := float64(done) / float64(total)
		sb.WriteString(s.Muted.Render(fmt.Sprintf("  %d / %d addons", done, total)) + "\n")
		sb.WriteString("  " + pb.ViewAs(pct) + "\n")
	} else if m.browseInstalling && len(m.browseInstallQueue) > 0 {
		done := m.browseInstallIdx
		total := len(m.browseInstallQueue)
		pct := float64(done) / float64(total)
		sb.WriteString(s.Muted.Render(fmt.Sprintf("  %d / %d addons", done, total)) + "\n")
		sb.WriteString("  " + pb.ViewAs(pct) + "\n")
	} else {
		sb.WriteString(s.Muted.Render("  Downloading...") + "\n")
		sb.WriteString("  " + pb.ViewAs(m.downloadProgress) + "\n")
	}
	return sb.String()
}

func (m model) renderCheckingUpdates() string {
	s := m.getStyles()
	return fmt.Sprintf("  %s %s\n", m.spinner.View(), s.Info.Render("Checking for updates..."))
}

func (m model) renderUpdatingAll() string {
	s := m.getStyles()
	var sb strings.Builder
	sb.WriteString(s.Label.Render("  Update All") + "\n\n")
	if m.loading {
		sb.WriteString(fmt.Sprintf("  %s %s\n", m.spinner.View(), s.Info.Render("Preparing updates...")))
	}
	return sb.String()
}

// ── Delete confirm ────────────────────────────────────────────────────────────

func (m model) renderConfirmDelete() string {
	s := m.getStyles()
	var sb strings.Builder

	name := ""
	if m.selectedAddonIdx < len(m.config.Addons) {
		name = m.config.Addons[m.selectedAddonIdx].Name
	}

	boxWidth := m.terminalWidth - 12
	if boxWidth < 36 {
		boxWidth = 36
	}

	var content strings.Builder
	content.WriteString(s.Warning.Render("Delete Addon") + "\n\n")
	content.WriteString("Delete " + s.Highlight.Render(name) + s.Muted.Render(" and all its folders?") + "\n")

	if m.selectedAddonIdx < len(m.config.Addons) {
		dirs := m.config.Addons[m.selectedAddonIdx].Directories
		if len(dirs) > 0 {
			content.WriteString("\n" + s.Muted.Render("Folders:") + "\n")
			for _, d := range dirs {
				content.WriteString("  " + s.Error.Render("−  "+d) + "\n")
			}
		}
	}

	content.WriteString("\n")
	yesBtn := zone.Mark("delete-yes", s.Success.Render("[ Yes, Delete ]"))
	noBtn := zone.Mark("delete-no", s.Error.Render("[ No, Cancel ]"))
	content.WriteString(yesBtn + "    " + noBtn + "\n")

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Error)).
		Padding(1, 2).
		Width(boxWidth)

	sb.WriteString(boxStyle.Render(content.String()) + "\n")
	return sb.String()
}

// ── Profile list ─────────────────────────────────────────────────────────────

func (m model) renderProfileList() string {
	s := m.getStyles()
	var sb strings.Builder

	sb.WriteString(s.Label.Render(fmt.Sprintf("  Profiles (%d)", len(m.config.Profiles))) + "\n\n")

	if len(m.config.Profiles) == 0 {
		sb.WriteString(s.Muted.Render("  No profiles yet.") + "\n")
		sb.WriteString(s.Info.Render("  Press [n] to create one.") + "\n")
	} else {
		for i, p := range m.config.Profiles {
			count := fmt.Sprintf("  %s", s.Muted.Render(fmt.Sprintf("(%d addons)", len(p.Addons))))
			var lineStr string
			if i == m.profileListCursor {
				lineStr = s.SelectedItem.Render("  ▶  "+p.Name) + count
			} else {
				lineStr = s.MenuItem.Render("     "+p.Name) + count
			}
			sb.WriteString(zone.Mark(fmt.Sprintf("profile-%d", i), lineStr) + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// ── Profile detail ────────────────────────────────────────────────────────────

func (m model) renderProfileDetail() string {
	s := m.getStyles()
	var sb strings.Builder

	if m.selectedProfileIdx >= len(m.config.Profiles) {
		return s.Error.Render("  Invalid profile") + "\n"
	}
	profile := m.config.Profiles[m.selectedProfileIdx]

	sb.WriteString(s.Highlight.Render("  "+profile.Name) + "\n\n")

	if len(profile.Addons) == 0 {
		sb.WriteString(s.Muted.Render("  No addons assigned. Press [e] to edit.") + "\n")
	} else {
		tableWidth := m.terminalWidth - 8
		if tableWidth < 30 {
			tableWidth = 30
		}
		t := lgtable.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Border))).
			StyleFunc(func(row, col int) lipgloss.Style {
				return lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Highlight))
			}).
			Width(tableWidth)
		for i, name := range profile.Addons {
			t.Row(fmt.Sprintf("%d", i+1), name)
		}
		for _, line := range strings.Split(t.Render(), "\n") {
			sb.WriteString("  " + line + "\n")
		}
	}

	sb.WriteString("\n")
	editBtn := s.Info.Render("[e] Edit addons")
	delBtn := s.Error.Render("[d] Delete profile")
	sb.WriteString("  " + editBtn + "  ·  " + delBtn + "  ·  " + s.Muted.Render("esc back") + "\n")
	return sb.String()
}

// ── Profile addon select ──────────────────────────────────────────────────────

func (m model) renderProfileAddonSelect() string {
	s := m.getStyles()
	var sb strings.Builder

	sb.WriteString(s.Label.Render("  Select Addons for Profile") + "\n\n")

	if len(m.config.Addons) == 0 {
		sb.WriteString(s.Muted.Render("  No addons to select.") + "\n")
	} else {
		for i, addon := range m.config.Addons {
			_, checked := m.profileAddonSelected[i]
			checkStr := "[ ]"
			style := s.MenuItem
			if checked {
				checkStr = "[✓]"
				style = s.Success
			}
			line := fmt.Sprintf("  %s  %s", checkStr, addon.Name)
			var lineStr string
			if i == m.profileAddonCursor {
				lineStr = s.SelectedItem.Render("> " + checkStr + "  " + addon.Name)
			} else {
				lineStr = style.Render(line)
			}
			sb.WriteString(zone.Mark(fmt.Sprintf("profile-addon-%d", i), lineStr) + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// ── New profile input ─────────────────────────────────────────────────────────

func (m model) renderNewProfileInput() string {
	s := m.getStyles()
	var sb strings.Builder
	sb.WriteString(s.Label.Render("  New Profile") + "\n\n")
	sb.WriteString(s.Info.Render("  Profile name:") + "\n\n")
	sb.WriteString("  " + m.textInputProfileName.View() + "\n\n")
	return sb.String()
}

// ── Settings ──────────────────────────────────────────────────────────────────

func (m model) renderSettings() string {
	s := m.getStyles()
	var sb strings.Builder

	settingsItems := []string{"Retail Path", "Classic Path", "GitHub Token", "Back"}
	settingsValues := []string{
		m.config.RetailPath,
		m.config.ClassicPath,
		maskToken(m.config.GithubToken),
		"",
	}

	sb.WriteString(s.Label.Render("  Settings") + "\n\n")

	for i, item := range settingsItems {
		val := settingsValues[i]
		valStr := ""
		if val != "" {
			// Truncate long paths for display
			if len(val) > 48 {
				val = "..." + val[len(val)-45:]
			}
			valStr = s.Muted.Render("  " + val)
		}
		var lineStr string
		if i == m.settingsCursor {
			lineStr = s.SelectedItem.Render("  ▶  "+fmt.Sprintf("%-18s", item)) + valStr
		} else {
			lineStr = s.MenuItem.Render("     "+fmt.Sprintf("%-18s", item)) + valStr
		}
		sb.WriteString(zone.Mark(fmt.Sprintf("settings-%d", i), lineStr) + "\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

func (m model) renderSettingsInput() string {
	s := m.getStyles()
	var sb strings.Builder

	var label, inputView string
	switch {
	case m.inputSettingsRetail:
		label = "Retail AddOns Path"
		inputView = m.textInputSettingsRetail.View()
	case m.inputSettingsClassic:
		label = "Classic AddOns Path"
		inputView = m.textInputSettingsClassic.View()
	case m.inputSettingsToken:
		label = "GitHub Token"
		inputView = m.textInputSettingsToken.View()
	}

	sb.WriteString(s.Label.Render("  Settings — "+label) + "\n\n")
	sb.WriteString("  " + inputView + "\n\n")
	return sb.String()
}

// ── Browse Addon DB ───────────────────────────────────────────────────────────

func (m model) renderBrowseDB() string {
	s := m.getStyles()
	var sb strings.Builder

	count := len(m.addonDB)
	header := fmt.Sprintf("  Browse Addons (%s repos)", formatCount(count))
	sb.WriteString(s.Label.Render(header) + "\n")

	if count == 0 {
		sb.WriteString(s.Muted.Render("  No database loaded.") + "\n")
		return sb.String()
	}

	if m.browseDBFilterActive {
		sb.WriteString(fmt.Sprintf("  Filter: %s\n", m.textInputBrowseFilter.View()))
	} else if m.browseDBFilter != "" {
		sb.WriteString(s.Muted.Render(fmt.Sprintf("  Filter: %q  (%d matches)", m.browseDBFilter, len(m.browseDBIndices))) + "\n")
	}

	if sel := len(m.browseDBSelected); sel > 0 {
		sb.WriteString(s.Success.Render(fmt.Sprintf("  %d selected — press Enter to install", sel)) + "\n")
	}
	sb.WriteString("\n")

	if len(m.browseDBIndices) == 0 {
		sb.WriteString(s.Muted.Render("  No addons match the filter.") + "\n")
		return sb.String()
	}

	// Dynamic column widths — row: "  " + cb(3) + "  " + repo(repoW) + "  " + desc(descW)
	// total = 2 + 3 + 2 + repoW + 2 + descW = 9 + repoW + descW
	w := m.terminalWidth
	if w <= 0 {
		w = 80
	}
	available := w - 9 // chars for repo + "  " + desc
	if available < 15 {
		available = 15
	}
	const maxRepo = 35
	repoW := maxRepo
	if repoW > available-2 {
		repoW = available - 2
		if repoW < 10 {
			repoW = 10
		}
	}
	descW := available - repoW - 2
	if descW < 0 {
		descW = 0
	}

	hdr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Secondary))
	colHdr := "  " + hdr.Width(3).Render("") + "  " + hdr.Width(repoW).Render("Repository")
	if descW > 0 {
		colHdr += "  " + hdr.Width(descW).Render("Description")
	}
	sb.WriteString(colHdr + "\n")

	divWidth := 3 + 2 + repoW
	if descW > 0 {
		divWidth += 2 + descW
	}
	divLine := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Border)).
		Render("  " + strings.Repeat("─", divWidth))
	sb.WriteString(divLine + "\n")

	// Compute overhead lines precisely so the list never overflows the window.
	// render():        2  (header title+divider, "\n" from render() terminates divider)
	// renderBrowseDB:  1  (section header) + 1 (blank) + 1 (col header) + 1 (top div) + 1 (bot div) = 5
	// renderFooter():  1  (blank "\n") + 2  (help — allow up to 2 lines for wrapping) = 3
	overhead := 2 + 5 + 3 // = 10
	if m.browseDBFilterActive || m.browseDBFilter != "" {
		overhead++ // filter line
	}
	if len(m.browseDBSelected) > 0 {
		overhead++ // selected-count line
	}
	visibleHeight := m.terminalHeight - overhead
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	start := m.browseDBCursor - visibleHeight/2
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(m.browseDBIndices) {
		end = len(m.browseDBIndices)
		start = end - visibleHeight
		if start < 0 {
			start = 0
		}
	}

	for visualRow, listPos := 0, start; listPos < end; visualRow, listPos = visualRow+1, listPos+1 {
		idx := m.browseDBIndices[listPos]
		e := m.addonDB[idx]

		_, isSelected := m.browseDBSelected[idx]
		checkbox := "[ ]"
		if isSelected {
			checkbox = "[✓]"
		}

		var lineStr string
		if listPos == m.browseDBCursor {
			line := "> " + lipgloss.NewStyle().Width(3).MaxWidth(3).Render(checkbox) + "  " +
				lipgloss.NewStyle().Width(repoW).MaxWidth(repoW).Render(truncateStr(e.Repo, repoW))
			if descW > 0 {
				line += "  " + lipgloss.NewStyle().Width(descW).Render(truncateStr(e.Description, descW))
			}
			lineStr = s.SelectedItem.Render(line)
		} else {
			cbStyle := s.Muted
			if isSelected {
				cbStyle = s.Success
			}
			lineStr = "  " + cbStyle.Width(3).MaxWidth(3).Render(checkbox) + "  " +
				s.Value.Width(repoW).MaxWidth(repoW).Render(truncateStr(e.Repo, repoW))
			if descW > 0 {
				lineStr += "  " + s.Muted.Width(descW).Render(truncateStr(e.Description, descW))
			}
		}
		sb.WriteString(zone.Mark(fmt.Sprintf("browse-row-%d", visualRow), lineStr) + "\n")
	}
	sb.WriteString(divLine + "\n")
	return sb.String()
}

// ── Browse install confirm ────────────────────────────────────────────────────

func (m model) renderBrowseInstallConfirm() string {
	s := m.getStyles()
	var sb strings.Builder

	count := len(m.browseDBSelected)
	sb.WriteString(s.Label.Render(fmt.Sprintf("  Install %d Addon%s", count, pluralS(count))) + "\n\n")

	shown := 0
	for idx := range m.browseDBSelected {
		if shown >= 10 {
			remaining := count - shown
			sb.WriteString(s.Muted.Render(fmt.Sprintf("    ... and %d more", remaining)) + "\n")
			break
		}
		sb.WriteString(fmt.Sprintf("  %s  %s\n", s.Success.Render("•"), s.Value.Render(m.addonDB[idx].Repo)))
		shown++
	}

	sb.WriteString("\n")
	var browseRetailLabel, browseClassicLabel string
	if m.browseInstallFlavor == "retail" {
		browseRetailLabel = zone.Mark("browse-flavor-retail", s.SelectedItem.Render("[Retail]"))
		browseClassicLabel = zone.Mark("browse-flavor-classic", s.Muted.Render("Classic"))
	} else {
		browseRetailLabel = zone.Mark("browse-flavor-retail", s.Muted.Render("Retail"))
		browseClassicLabel = zone.Mark("browse-flavor-classic", s.SelectedItem.Render("[Classic]"))
	}
	sb.WriteString(fmt.Sprintf("  Game: %s  %s\n\n", browseRetailLabel, browseClassicLabel))

	installBtn := zone.Mark("browse-action-install", s.Success.Render("[ Install ]"))
	cancelBtn := zone.Mark("browse-action-cancel", s.Error.Render("[ Cancel ]"))
	sb.WriteString(fmt.Sprintf("  %s  %s\n", installBtn, cancelBtn))
	sb.WriteString(s.Muted.Render("  Tab toggle retail/classic  ·  Esc cancel") + "\n")
	return sb.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// statusGlyph returns the status symbol and its theme color hex.
func statusGlyph(status UpdateStatus, t Theme) (string, string) {
	switch status {
	case StatusUpToDate:
		return "✓", t.Success
	case StatusUpdateAvail:
		return "!", t.Warning
	case StatusNotInstalled:
		return "✗", t.Error
	default:
		return "?", t.Muted
	}
}

func formatDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Jan 2, 2006")
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "●●●●"
	}
	return token[:4] + "···" + token[len(token)-4:]
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func formatStars(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// truncateStr hard-truncates s to at most n visible runes, stripping newlines.
// Appends "…" if the string was cut.
func truncateStr(s string, n int) string {
	s = strings.SplitN(s, "\n", 2)[0] // take only first line
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}
