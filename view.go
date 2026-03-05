package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	lgtable "charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
)

// watBanner is the 5-line full-width block-art banner "WOW ADDON TRACKER".
var watBanner = []string{
	"█   █  ███  █   █      ███  ████  ████   ███  █   █     █████ ████   ███   ████ █  █  █████ ████  ",
	"█   █ █   █ █   █     █   █ █   █ █   █ █   █ ██  █       █   █   █ █   █ █     █ █   █     █   █ ",
	"█ █ █ █   █ █ █ █     █████ █   █ █   █ █   █ █ █ █       █   ████  █████ █     ██    ████  ████  ",
	"██ ██ █   █ ██ ██     █   █ █   █ █   █ █   █ █  ██       █   █  █  █   █ █     █ █   █     █  █  ",
	"█   █  ███  █   █     █   █ ████  ████   ███  █   █       █   █   █ █   █  ████ █  █  █████ █   █ ",
}

func (m model) View() tea.View {
	return tea.View{
		Content:   zone.Scan(m.render()),
		AltScreen: true,
		MouseMode: tea.MouseModeCellMotion,
	}
}

func (m model) render() string {
	// Before the first WindowSizeMsg we have no terminal dimensions — show a
	// blank initializing screen to avoid a scrunched layout flash.
	if !m.viewportReady {
		return "  Initializing...\n"
	}

	var sb strings.Builder

	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	if m.loading && !m.installing && !m.updatingSingle && !m.updatingAll && !m.checkingUpdates && !m.addRepoFetching {
		sb.WriteString(fmt.Sprintf("  %s Loading...\n", m.spinner.View()))
		sb.WriteString(m.renderFooter(nil))
		return sb.String()
	}

	switch {
	case m.viewBrowseDetail:
		sb.WriteString(m.renderBrowseDetail())
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
	case m.viewBrowseDetail:
		km = browseDetailKeys
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
	result := sb.String()
	if m.confirmQuit {
		return placeCentered(result, m.renderQuitConfirm(), m.terminalWidth, m.terminalHeight)
	}
	return result
}

// placeCentered composites a popup string centered over the base screen content.
// Each line that the popup occupies is rebuilt as:
//
//	left_bg_chars + popup_line + right_bg_chars
//
// using ANSI-aware truncation so existing styling on the background is preserved.
func placeCentered(base, popup string, termW, termH int) string {
	baseLines := strings.Split(base, "\n")
	popLines := strings.Split(strings.TrimRight(popup, "\n"), "\n")

	popH := len(popLines)
	popW := 0
	for _, l := range popLines {
		if w := lipgloss.Width(l); w > popW {
			popW = w
		}
	}

	startRow := (termH - popH) / 2
	startCol := (termW - popW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	for len(baseLines) <= startRow+popH {
		baseLines = append(baseLines, "")
	}

	for i, popLine := range popLines {
		row := startRow + i
		bg := baseLines[row]
		// Left: background up to startCol, right: background from startCol+popW onward.
		left := ansi.Truncate(bg, startCol, "")
		right := ansi.TruncateLeft(bg, startCol+popW, "")
		baseLines[row] = left + popLine + right
	}

	return strings.Join(baseLines, "\n")
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
		// Large terminal: 5-line full-width block-art "WoW Addon Tracker" banner.
		logoStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Primary))
		var out strings.Builder
		for _, line := range watBanner {
			out.WriteString(logoStyle.Render(line) + "\n")
		}
		// Stats line below the banner, right-aligned
		if statsStr != "" {
			statsW := lipgloss.Width(statsStr)
			gap := w - statsW - 2
			if gap < 1 {
				gap = 1
			}
			out.WriteString(strings.Repeat(" ", gap) + statsStr + "\n")
		}
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

	// Content overhead: large header (h>=26) = 5 banner + 1 stats + 1 divider + 1 blank + 2 footer = 10
	// Small header = 2 header + 1 blank + 2 footer = 5
	overhead := 5
	if h >= 26 {
		overhead = 10
	}
	contentH := h - overhead
	if contentH < 10 {
		contentH = 10
	}

	// Width split: installed panel takes 3/4, info panel takes 1/4.
	infoOuterW := w / 4
	if infoOuterW < 22 {
		infoOuterW = 22
	}
	if infoOuterW > 36 {
		infoOuterW = 36
	}
	installedOuterW := w - infoOuterW

	// Height split: top row (installed + info panel) and bottom browse panel.
	// tabRowH = 2: one row per tab strip.
	// Three tab strips total: flavor (above installed), browse flavor (Retail/Classic),
	// and browse kind (All/Hot/New) — the latter two are above the browse panel.
	const tabRowH = 2
	topRowH := contentH / 2
	if topRowH < 6 {
		topRowH = 6
	}
	browseOuterH := contentH - topRowH - tabRowH*3
	if browseOuterH < 4 {
		browseOuterH = 4
	}

	flavorTabs := m.renderFlavorTabs(installedOuterW)
	installed := m.renderInstalledPanel(installedOuterW, topRowH)
	info := m.renderSidebar(infoOuterW, topRowH+tabRowH)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Top, flavorTabs, installed),
		info)
	browseFlavorTabs := m.renderBrowseFlavorTabs(w)
	browseTabs := m.renderBrowseTabs(w)
	browse := m.renderBrowsePanel(w, browseOuterH)

	return lipgloss.JoinVertical(lipgloss.Top, topRow, browseFlavorTabs, browseTabs, browse) + "\n"
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

// renderBrowseFlavorTabs renders the Retail / Classic tab strip above the browse
// panel, mirroring the installed flavor tabs.
func (m model) renderBrowseFlavorTabs(w int) string {
	focused := m.dashboardFocus == "browse"
	accentCol := lipgloss.Color(m.theme.Accent)
	if !focused {
		accentCol = lipgloss.Color(m.theme.Border)
	}
	activeSty := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentCol).
		Border(lipgloss.RoundedBorder(), true, true, false, true).
		BorderForeground(accentCol).
		Padding(0, 1)
	inactiveSty := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Muted)).
		Padding(0, 2)

	var retailSty, classicSty lipgloss.Style
	if m.browseFlavor == "retail" {
		retailSty = activeSty
		classicSty = inactiveSty
	} else {
		retailSty = inactiveSty
		classicSty = activeSty
	}

	retailTab := zone.Mark("browse-flavor-tab-retail", retailSty.Render("Retail"))
	classicTab := zone.Mark("browse-flavor-tab-classic", classicSty.Render("Classic"))
	return lipgloss.JoinHorizontal(lipgloss.Bottom, retailTab, classicTab)
}

// renderBrowseTabs renders the All / Hot / New tab strip above the browse panel,
// mirroring the style of renderFlavorTabs.
func (m model) renderBrowseTabs(w int) string {
	focused := m.dashboardFocus == "browse"
	accentCol := lipgloss.Color(m.theme.Accent)
	if !focused {
		accentCol = lipgloss.Color(m.theme.Border)
	}
	activeSty := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentCol).
		Border(lipgloss.RoundedBorder(), true, true, false, true).
		BorderForeground(accentCol).
		Padding(0, 1)
	inactiveSty := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Muted)).
		Padding(0, 2)

	type tabDef struct{ id, label string }
	defs := []tabDef{{"all", "All"}, {"hot", "Hot"}, {"new", "New"}}
	var tabs []string
	for _, td := range defs {
		var rendered string
		if m.browseTab == td.id {
			rendered = zone.Mark("browse-tab-"+td.id, activeSty.Render(td.label))
		} else {
			rendered = zone.Mark("browse-tab-"+td.id, inactiveSty.Render(td.label))
		}
		tabs = append(tabs, rendered)
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
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

	// Column widths: Name | Creator | Version | Source | Status
	// Fixed: cursor(1) + 5×sep(2) + cCreator + cVer + cSource + status(1) = 12+cCreator+cVer+cSource
	const cVer = 10
	const cCreator = 14
	const cSource = 6
	cNm := innerW - 12 - cCreator - cVer - cSource
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
			"  "+lipgloss.NewStyle().Width(cSource).Render("Source")+
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
			src := addonSource(aws.Addon)

			// Selected row: name+creator+ver+source highlighted, status keeps its own color
			var line string
			if listPos == m.addonListCursor {
				line = s.SelectedItem.Render(
					">"+lipgloss.NewStyle().Width(cNm).Render(name)+
						"  "+lipgloss.NewStyle().Width(cCreator).Render(cre)+
						"  "+lipgloss.NewStyle().Width(cVer).Render(ver)+
						"  "+lipgloss.NewStyle().Width(cSource).Render(src)) +
					"  " + stsStyled
			} else {
				line = " " +
					lipgloss.NewStyle().Width(cNm).Render(name) +
					"  " + s.Muted.Width(cCreator).Render(cre) +
					"  " + s.Value.Width(cVer).Render(ver) +
					"  " + s.Muted.Width(cSource).Render(src) +
					"  " + stsStyled
			}
			lines = append(lines, zone.Mark(fmt.Sprintf("inst-row-%d", listPos), line))
		}
	}

	// Clamp every line to innerW then pad/clip to innerH.
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, innerW, "")
	}
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
		Width(outerW).
		MaxWidth(outerW).
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
		var emptyMsg string
		switch {
		case m.browseTab == "hot" && !m.rssHotLoaded:
			emptyMsg = fmt.Sprintf(" %s Fetching hot list…", m.spinner.View())
		case m.browseTab == "new" && !m.rssNewLoaded:
			emptyMsg = fmt.Sprintf(" %s Fetching new releases…", m.spinner.View())
		default:
			emptyMsg = " No addons match filter."
		}
		lines = append(lines, s.Muted.Render(emptyMsg))
	} else {
		// Column widths:
		// " "(1) + cb(3) + "  " + name(nameW) + "  " + creator(14) + "  " + version(10) + "  " + desc(descW)
		// fixed overhead = 1+3+2+2+14+2+10+2 = 36
		const bCreatorW = 14
		const bVersionW = 10
		available := innerW - 36
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
				"  " + lipgloss.NewStyle().Width(bVersionW).Render("Latest") +
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

			name := truncateStr(e.Name, nameW)
			cre := truncateStr(e.Author, bCreatorW)
			ver := displayVersion(e.LatestVersion)
			if ver == "" {
				ver = "—"
			}
			ver = truncateStr(ver, bVersionW)
			desc := truncateStr(e.Description, descW)

			var line string
			if listPos == m.browseDBCursor {
				row := ">" + lipgloss.NewStyle().Width(3).MaxWidth(3).Render(checkbox) +
					"  " + lipgloss.NewStyle().Width(nameW).MaxWidth(nameW).Render(name) +
					"  " + lipgloss.NewStyle().Width(bCreatorW).MaxWidth(bCreatorW).Render(cre) +
					"  " + lipgloss.NewStyle().Width(bVersionW).MaxWidth(bVersionW).Render(ver)
				if descW > 0 {
					row += "  " + lipgloss.NewStyle().Width(descW).MaxWidth(descW).Render(desc)
				}
				line = s.SelectedItem.Render(row)
			} else {
				cbStyle := s.Muted
				if isSelected {
					cbStyle = s.Success
				}
				line = " " + cbStyle.Width(3).MaxWidth(3).Render(checkbox) +
					"  " + s.Value.Width(nameW).MaxWidth(nameW).Render(name) +
					"  " + s.Muted.Width(bCreatorW).MaxWidth(bCreatorW).Render(cre) +
					"  " + s.Info.Width(bVersionW).MaxWidth(bVersionW).Render(ver)
				if descW > 0 {
					line += "  " + s.Muted.Width(descW).MaxWidth(descW).Render(desc)
				}
			}
			lines = append(lines, zone.Mark(fmt.Sprintf("browse-row-%d", listPos), ansi.Truncate(line, innerW, "")))
		}
	}

	// Clamp every line then pad/clip to innerH.
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, innerW, "")
	}
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
		Width(outerW).
		MaxWidth(outerW).
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
	if m.dbRefreshing {
		lines = append(lines, s.Muted.Render(fmt.Sprintf(" %s refreshing DB…", m.spinner.View())))
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

	// Clamp every line then pad/clip to innerH.
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, innerW, "")
	}
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
		Width(outerW).
		MaxWidth(outerW).
		Height(innerH).
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
	const cSt, cIn, cLt, cFl, cSrc = 3, 12, 12, 7, 5
	// 2 (indent) + cSt + 5×2 (separators) + cIn + cLt + cFl + cSrc = 2+3+10+12+12+7+5 = 51
	w := m.terminalWidth
	if w <= 0 {
		w = 80
	}
	cNm := w - 51
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
		hdr.Width(cFl).Render("Flavor") + "  " +
		hdr.Width(cSrc).Render("Src")
	sb.WriteString(colHdr + "\n")
	divLine := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Border)).
		Render("  " + strings.Repeat("─", cSt+cNm+cIn+cLt+cFl+cSrc+10))
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
		src := addonSource(aws.Addon)

		var lineStr string
		if listPos == m.addonListCursor {
			lineStr = s.SelectedItem.Render(
				"> " +
					lipgloss.NewStyle().Width(cSt).Render(sym) + "  " +
					lipgloss.NewStyle().Width(cNm).Render(name) + "  " +
					lipgloss.NewStyle().Width(cIn).Render(installed) + "  " +
					lipgloss.NewStyle().Width(cLt).Render(latest) + "  " +
					lipgloss.NewStyle().Width(cFl).Render(aws.Addon.GameFlavor) + "  " +
					lipgloss.NewStyle().Width(cSrc).Render(src),
			)
		} else {
			nameCell := lipgloss.NewStyle().Width(cNm).Render(name)
			installedCell := s.Value.Width(cIn).Render(installed)
			latestCell := s.StatusWarn.Width(cLt).Render(latest)
			flavorCell := s.Muted.Width(cFl).Render(aws.Addon.GameFlavor)
			srcCell := s.Muted.Width(cSrc).Render(src)
			lineStr = "  " + symCell + "  " + nameCell + "  " + installedCell + "  " + latestCell + "  " + flavorCell + "  " + srcCell
		}
		sb.WriteString(zone.Mark(fmt.Sprintf("addon-%d", listPos), lineStr) + "\n")
	}
	sb.WriteString(divLine + "\n")
	return sb.String()
}

// ── Addon detail ─────────────────────────────────────────────────────────────

func (m model) renderAddonDetail() string {
	s := m.getStyles()
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

	var sb strings.Builder
	sb.WriteString(s.Highlight.Render("  "+addon.Name) + "\n\n")

	// Full detail content rendered via glamour in a scrollable viewport.
	sb.WriteString(m.viewport.View())
	if !m.viewport.AtTop() || !m.viewport.AtBottom() {
		pct := m.viewport.ScrollPercent()
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))
		sb.WriteString("\n  " + mutedStyle.Render(fmt.Sprintf("%.0f%%  ↑/↓ to scroll", pct*100)) + "\n")
	}

	sb.WriteString("\n")
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))
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

// ── Browse addon detail ───────────────────────────────────────────────────────

func (m model) renderBrowseDetail() string {
	s := m.getStyles()
	if m.selectedBrowseDBIdx >= len(m.addonDB) {
		return s.Error.Render("  Invalid selection") + "\n"
	}
	e := m.addonDB[m.selectedBrowseDBIdx]

	var sb strings.Builder
	sb.WriteString(s.Highlight.Render("  "+e.Name) + "\n\n")

	// Scrollable rendered markdown body.
	sb.WriteString(m.viewport.View())
	if !m.viewport.AtTop() || !m.viewport.AtBottom() {
		pct := m.viewport.ScrollPercent()
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))
		sb.WriteString("\n  " + mutedStyle.Render(fmt.Sprintf("%.0f%%  ↑/↓ to scroll", pct*100)) + "\n")
	}

	sb.WriteString("\n")
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted))
	installBtn := zone.Mark("browse-detail-install", s.Success.Render("[i] Install"))
	sb.WriteString("  " + installBtn + mutedStyle.Render("  ·  esc back") + "\n")
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

		const cNm, cAu = 28, 16
		for i, e := range m.dbSuggestions {
			name := e.Name
			if name == "" {
				name = e.Repo
			}
			author := e.Author
			desc := e.Description
			if len(desc) > 45 {
				desc = desc[:42] + "..."
			}

			var lineStr string
			if i == m.dbSuggestionIdx {
				lineStr = s.SelectedItem.Render(
					"> " +
						lipgloss.NewStyle().Width(cNm).MaxWidth(cNm).Render(name) + "  " +
						lipgloss.NewStyle().Width(cAu).MaxWidth(cAu).Render(author) + "  " +
						desc,
				)
			} else {
				lineStr = s.Muted.Render(
					"  " +
						lipgloss.NewStyle().Width(cNm).MaxWidth(cNm).Render(name) + "  " +
						lipgloss.NewStyle().Width(cAu).MaxWidth(cAu).Render(author) + "  " +
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

	// Description from the addon DB (WoWI or GitHub entries).
	if entry := m.pendingDBEntry(); entry != nil && entry.Description != "" {
		content.WriteString("\n" + s.Label.Render("Description:") + "\n")
		desc := entry.Description
		if m.glamourRenderer != nil {
			if out, err := m.glamourRenderer.Render(desc); err == nil {
				desc = out
			}
		}
		lines := strings.Split(strings.TrimSpace(desc), "\n")
		const maxDescLines = 10
		for i, line := range lines {
			if i >= maxDescLines {
				content.WriteString(s.Muted.Render("  ...") + "\n")
				break
			}
			content.WriteString(line + "\n")
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

func (m model) renderQuitConfirm() string {
	s := m.getStyles()

	noStyle := s.Muted
	yesStyle := s.Muted
	if m.quitConfirmFocus == "yes" {
		yesStyle = s.SelectedItem
	} else {
		noStyle = s.SelectedItem
	}

	yesBtn := zone.Mark("quit-yes", yesStyle.Render("[ Yes ]"))
	noBtn := zone.Mark("quit-no", noStyle.Render("[ No ]"))

	var content strings.Builder
	content.WriteString(s.Warning.Render("Quit WoW Addon Tracker?") + "\n\n")
	content.WriteString(yesBtn + "    " + noBtn + "\n")
	content.WriteString("\n" + s.Muted.Render("←/→ toggle · enter confirm · esc cancel"))

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Accent)).
		Padding(1, 3)

	return boxStyle.Render(content.String()) + "\n"
}

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

		displayName := e.Name
		if displayName == "" {
			displayName = e.Repo
		}
		var lineStr string
		if listPos == m.browseDBCursor {
			line := "> " + lipgloss.NewStyle().Width(3).MaxWidth(3).Render(checkbox) + "  " +
				lipgloss.NewStyle().Width(repoW).MaxWidth(repoW).Render(truncateStr(displayName, repoW))
			if descW > 0 {
				line += "  " + lipgloss.NewStyle().Width(descW).MaxWidth(descW).Render(truncateStr(e.Description, descW))
			}
			lineStr = s.SelectedItem.Render(line)
		} else {
			cbStyle := s.Muted
			if isSelected {
				cbStyle = s.Success
			}
			lineStr = "  " + cbStyle.Width(3).MaxWidth(3).Render(checkbox) + "  " +
				s.Value.Width(repoW).MaxWidth(repoW).Render(truncateStr(displayName, repoW))
			if descW > 0 {
				lineStr += "  " + s.Muted.Width(descW).MaxWidth(descW).Render(truncateStr(e.Description, descW))
			}
		}
		sb.WriteString(zone.Mark(fmt.Sprintf("browse-row-%d", visualRow), ansi.Truncate(lineStr, w, "")) + "\n")
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
		name := m.addonDB[idx].Name
		if name == "" {
			name = m.addonDB[idx].Repo
		}
		sb.WriteString(fmt.Sprintf("  %s  %s\n", s.Success.Render("•"), s.Value.Render(name)))
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

// addonSource returns the short source label for a tracked addon.
func addonSource(addon TrackedAddon) string {
	if strings.HasPrefix(addon.GithubRepo, "wowinterface:") {
		return "WoW:I"
	}
	return "GH"
}

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
