package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
)

// getStyles returns lipgloss styles based on the current theme
func (m model) getStyles() struct {
	Title        lipgloss.Style
	Header       lipgloss.Style
	MenuItem     lipgloss.Style
	SelectedItem lipgloss.Style
	Success      lipgloss.Style
	Error        lipgloss.Style
	Warning      lipgloss.Style
	Info         lipgloss.Style
	Muted        lipgloss.Style
	Divider      lipgloss.Style
	Value        lipgloss.Style
	Label        lipgloss.Style
	Highlight    lipgloss.Style
	StatusOK     lipgloss.Style
	StatusWarn   lipgloss.Style
	StatusMuted  lipgloss.Style
} {
	return struct {
		Title        lipgloss.Style
		Header       lipgloss.Style
		MenuItem     lipgloss.Style
		SelectedItem lipgloss.Style
		Success      lipgloss.Style
		Error        lipgloss.Style
		Warning      lipgloss.Style
		Info         lipgloss.Style
		Muted        lipgloss.Style
		Divider      lipgloss.Style
		Value        lipgloss.Style
		Label        lipgloss.Style
		Highlight    lipgloss.Style
		StatusOK     lipgloss.Style
		StatusWarn   lipgloss.Style
		StatusMuted  lipgloss.Style
	}{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(m.theme.Primary)).
			MarginBottom(1),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(m.theme.Secondary)).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(m.theme.Border)).
			Padding(0, 1),
		MenuItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Foreground)),
		SelectedItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Accent)).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Success)).
			Bold(true),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Error)).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Warning)).
			Bold(true),
		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Primary)),
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Muted)),
		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Border)),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Highlight)),
		Label: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Secondary)),
		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Highlight)).
			Bold(true),
		StatusOK: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Success)),
		StatusWarn: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Warning)),
		StatusMuted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Muted)),
	}
}

// detectTerminalBackground tries to determine if the terminal has a dark background
func detectTerminalBackground() bool {
	colorfgbg := os.Getenv("COLORFGBG")
	if colorfgbg != "" {
		parts := strings.Split(colorfgbg, ";")
		if len(parts) >= 2 {
			if bgNum, err := strconv.Atoi(parts[1]); err == nil {
				return bgNum < 8 || bgNum == 0
			}
		}
	}
	return true
}

// newGlamourRenderer creates a glamour TermRenderer sized to the given content
// width using the auto dark/light style.  Falls back gracefully on error.
func newGlamourRenderer(width int) *glamour.TermRenderer {
	if width < 20 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	return r
}

func initialModel() model {
	tiRepo := textinput.New()
	tiRepo.Placeholder = "Owner/Repo (e.g. WeakAuras2/WeakAuras2)"
	tiRepo.CharLimit = 200
	tiRepo.SetWidth(60)

	tiAddonFilter := textinput.New()
	tiAddonFilter.Placeholder = "Filter addons..."
	tiAddonFilter.CharLimit = 100
	tiAddonFilter.SetWidth(40)

	tiProfileName := textinput.New()
	tiProfileName.Placeholder = "Profile name..."
	tiProfileName.CharLimit = 100
	tiProfileName.SetWidth(40)

	tiSettingsRetail := textinput.New()
	tiSettingsRetail.Placeholder = "Path to Retail AddOns directory..."
	tiSettingsRetail.CharLimit = 500
	tiSettingsRetail.SetWidth(80)

	tiSettingsClassic := textinput.New()
	tiSettingsClassic.Placeholder = "Path to Classic AddOns directory..."
	tiSettingsClassic.CharLimit = 500
	tiSettingsClassic.SetWidth(80)

	tiSettingsToken := textinput.New()
	tiSettingsToken.Placeholder = "GitHub personal access token (optional)..."
	tiSettingsToken.CharLimit = 200
	tiSettingsToken.SetWidth(80)
	tiSettingsToken.EchoMode = textinput.EchoPassword

	tiBrowseFilter := textinput.New()
	tiBrowseFilter.Placeholder = "Filter addons..."
	tiBrowseFilter.CharLimit = 100
	tiBrowseFilter.SetWidth(60)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	pb := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(50),
	)

	isDark := detectTerminalBackground()
	selectedTheme := Dracula
	if !isDark {
		selectedTheme = DraculaLight
	}

	return model{
		dashboardFocus: "installed",
		defaultFlavor:  "retail",

		addonsWithStatus:     []AddonWithStatus{},
		addonFilteredIndices: []int{},
		profileAddonSelected: make(map[int]struct{}),

		textInputRepo:            tiRepo,
		textInputAddonFilter:     tiAddonFilter,
		textInputProfileName:     tiProfileName,
		textInputSettingsRetail:  tiSettingsRetail,
		textInputSettingsClassic: tiSettingsClassic,
		textInputSettingsToken:   tiSettingsToken,
		textInputBrowseFilter:    tiBrowseFilter,

		spinner:         s,
		progressBar:     pb,
		theme:           selectedTheme,
		glamourRenderer: newGlamourRenderer(80),
		updateQueue:     []string{},
		updateAllErrors: []string{},
		dbSuggestionIdx:  -1,
		browseFlavor:        "retail",
		browseDBIndices:     []int{},
		browseDBSelected:    make(map[int]struct{}),
		browseInstallFlavor: "retail",
		browseTab:           "all",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadConfig(), loadAddonDB(),
		fetchWoWIRSS("hot"), fetchWoWIRSS("new"))
}

// browseCurIndices returns browse DB indices for the current tab and filter,
// narrowed to the current browseFlavor (retail/classic).
func (m model) browseCurIndices() []int {
	var raw []int
	switch m.browseTab {
	case "hot":
		raw = computeTabIndices(m.browseDBFilter, m.addonDB, m.hotIDs)
	case "new":
		raw = computeTabIndices(m.browseDBFilter, m.addonDB, m.latestIDs)
	default:
		raw = computeBrowseFilter(m.browseDBFilter, m.addonDB)
	}
	return filterByFlavor(raw, m.addonDB, m.browseFlavor)
}

// computeAddonFilter returns indices into addons that match the flavor and
// query (case-insensitive substring). An empty flavor matches all flavors.
func computeAddonFilter(flavor, query string, addons []TrackedAddon) []int {
	q := strings.ToLower(query)
	var out []int
	for i, a := range addons {
		if flavor != "" && a.GameFlavor != flavor {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(a.Name), q) &&
			!strings.Contains(strings.ToLower(a.GithubRepo), q) {
			continue
		}
		out = append(out, i)
	}
	return out
}

// displayVersion formats a version/tag string for compact display in the UI.
//
// Some addons (e.g. Details-Damage-Meter) publish tags like
// "Details.20260304.14718.170" instead of semver releases.  For those we
// extract and return the YYYYMMDD date segment, which is the most human-
// readable part and fits in the 12-char version column.
//
// For normal semver tags ("v15.08", "12.0.28") the leading "v" is stripped
// and the rest is returned unchanged.
func displayVersion(v string) string {
	if v == "" {
		return v
	}
	v = strings.TrimPrefix(v, "v")
	// Walk the alphanumeric segments of the tag looking for an 8-digit value
	// that resembles a YYYYMMDD date.
	for _, seg := range splitTagSegments(v) {
		n, ok := parseTagUint(seg)
		if ok && len(seg) == 8 && isLikelyDate(n) {
			return seg
		}
	}
	return v
}

// isLikelyDate returns true when n looks like a valid YYYYMMDD integer:
// year 2000–2099, month 01–12, day 01–31.
func isLikelyDate(n uint64) bool {
	day := n % 100
	month := (n / 100) % 100
	year := n / 10000
	return year >= 2000 && year <= 2099 && month >= 1 && month <= 12 && day >= 1 && day <= 31
}

// statusSymbol returns a display symbol for an UpdateStatus
func statusSymbol(s UpdateStatus) string {
	switch s {
	case StatusUpToDate:
		return "✓"
	case StatusUpdateAvail:
		return "!"
	case StatusNotInstalled:
		return "✗"
	default:
		return "?"
	}
}

// addonExtractAs resolves the extraction folder override for a repo.
// The tracked addon config takes priority (may have been auto-detected on a
// previous install); the addon DB entry is the fallback for first installs.
func addonExtractAs(repo string, addons []TrackedAddon, db []AddonDBEntry) string {
	for _, a := range addons {
		if a.GithubRepo == repo && a.ExtractAs != "" {
			return a.ExtractAs
		}
	}
	for _, e := range db {
		if e.Repo == repo {
			return e.ExtractAs
		}
	}
	return ""
}

// setupChangelogViewport renders the full installed addon detail into m.viewport.
// Call this whenever viewAddonDetail becomes true or the terminal is resized.
func setupChangelogViewport(m model) model {
	var rendered string
	if m.selectedAddonIdx < len(m.config.Addons) {
		addon := m.config.Addons[m.selectedAddonIdx]
		var aws AddonWithStatus
		if m.selectedAddonIdx < len(m.addonsWithStatus) {
			aws = m.addonsWithStatus[m.selectedAddonIdx]
		} else {
			aws = AddonWithStatus{Addon: addon, Status: StatusUnknown}
		}
		dbEntry := m.installedAddonDBEntry(addon)
		md := buildInstalledAddonMarkdown(addon, aws, dbEntry)
		if m.glamourRenderer != nil {
			if out, err := m.glamourRenderer.Render(md); err == nil {
				rendered = out
			} else {
				rendered = md
			}
		} else {
			rendered = md
		}
	}
	// Overhead: header(8) + name+blank(2) + scroll-indicator(2) + blank+action(2) + footer(2) = 16
	h := m.terminalHeight - 16
	if h < 5 {
		h = 5
	}
	m.viewport.SetHeight(h)
	m.viewport.SetWidth(m.terminalWidth - 4)
	m.viewport.SetContent(rendered)
	m.viewport.GotoTop()
	return m
}

// buildBrowseDetailMarkdown constructs a Markdown document containing all
// scraped information for an AddonDBEntry. Description comes first so it is
// immediately visible without scrolling; metadata and changelog follow.
func buildBrowseDetailMarkdown(e AddonDBEntry) string {
	var sb strings.Builder

	sb.WriteString("# " + e.Name + "\n\n")

	// Compact single-line header: author · version · date
	var headerParts []string
	if e.Author != "" {
		headerParts = append(headerParts, "by **"+e.Author+"**")
	}
	if e.LatestVersion != "" {
		headerParts = append(headerParts, displayVersion(e.LatestVersion))
	}
	if e.LatestDate != "" {
		headerParts = append(headerParts, formatDate(e.LatestDate))
	}
	if len(headerParts) > 0 {
		sb.WriteString(strings.Join(headerParts, "  ·  ") + "\n\n")
	}

	// Description immediately after the title so it is visible on first open.
	if e.Description != "" {
		sb.WriteString(e.Description + "\n\n")
	}

	// Details section below the description.
	sb.WriteString("---\n\n")

	var dlParts []string
	if e.Downloads > 0 {
		dlParts = append(dlParts, "**Downloads:** "+formatInt(e.Downloads))
	}
	if e.DownloadsMonthly > 0 {
		dlParts = append(dlParts, formatInt(e.DownloadsMonthly)+"/month")
	}
	if e.Favorites > 0 {
		dlParts = append(dlParts, "★ "+formatInt(e.Favorites)+" favorites")
	}
	if len(dlParts) > 0 {
		sb.WriteString(strings.Join(dlParts, "  ·  ") + "\n\n")
	}

	if len(e.Compatibility) > 0 {
		sb.WriteString("**Compatible with:** " + strings.Join(e.Compatibility, " · ") + "\n\n")
	}

	if e.FileInfoURL != "" {
		sb.WriteString("**Page:** [" + e.FileInfoURL + "](" + e.FileInfoURL + ")\n\n")
	}

	if e.Changelog != "" && e.Changelog != "None" {
		sb.WriteString("---\n\n## Changelog\n\n")
		sb.WriteString(e.Changelog + "\n\n")
	}

	return sb.String()
}

// formatInt formats an integer with thousands separators.
func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}
	out = append(out, s[:rem]...)
	for i := rem; i < len(s); i += 3 {
		out = append(out, ',')
		out = append(out, s[i:i+3]...)
	}
	return string(out)
}

// buildInstalledAddonMarkdown constructs a Markdown document for an installed addon,
// combining live status with any description/changelog from the addon DB.
func buildInstalledAddonMarkdown(addon TrackedAddon, aws AddonWithStatus, dbEntry *AddonDBEntry) string {
	var sb strings.Builder

	sb.WriteString("# " + addon.Name + "\n\n")

	if dbEntry != nil && dbEntry.Author != "" {
		sb.WriteString("**Author:** " + dbEntry.Author + "\n\n")
	}

	sb.WriteString("**Source:** " + addonSource(addon) + "\n\n")
	sb.WriteString("**Flavor:** " + addon.GameFlavor + "\n\n")

	var verParts []string
	if addon.InstalledVersion != "" {
		v := "**Installed:** " + displayVersion(addon.InstalledVersion)
		if addon.InstalledDate != "" {
			v += "  ·  " + formatDate(addon.InstalledDate)
		}
		verParts = append(verParts, v)
	}
	latestVer := aws.LatestVersion
	if latestVer == "" {
		latestVer = addon.LatestVersion
	}
	latestDate := aws.LatestDate
	if latestDate == "" {
		latestDate = addon.LatestDate
	}
	if latestVer != "" {
		v := "**Latest:** " + displayVersion(latestVer)
		if latestDate != "" {
			v += "  ·  " + formatDate(latestDate)
		}
		verParts = append(verParts, v)
	}
	for _, p := range verParts {
		sb.WriteString(p + "\n\n")
	}

	switch aws.Status {
	case StatusUpToDate:
		sb.WriteString("**Status:** ✓ Up to date\n\n")
	case StatusUpdateAvail:
		sb.WriteString(fmt.Sprintf("**Status:** ! Update available: %s → %s\n\n",
			displayVersion(addon.InstalledVersion), displayVersion(latestVer)))
	case StatusNotInstalled:
		sb.WriteString("**Status:** ✗ Not installed\n\n")
	}

	if len(addon.Directories) > 0 {
		sb.WriteString("**Folders:** " + strings.Join(addon.Directories, ", ") + "\n\n")
	}
	if len(addon.Profiles) > 0 {
		sb.WriteString("**Profiles:** " + strings.Join(addon.Profiles, ", ") + "\n\n")
	}

	if dbEntry != nil {
		var dlParts []string
		if dbEntry.Downloads > 0 {
			dlParts = append(dlParts, "**Downloads:** "+formatInt(dbEntry.Downloads))
		}
		if dbEntry.DownloadsMonthly > 0 {
			dlParts = append(dlParts, formatInt(dbEntry.DownloadsMonthly)+"/month")
		}
		if dbEntry.Favorites > 0 {
			dlParts = append(dlParts, "★ "+formatInt(dbEntry.Favorites)+" favorites")
		}
		if len(dlParts) > 0 {
			sb.WriteString(strings.Join(dlParts, "  ·  ") + "\n\n")
		}
		if len(dbEntry.Compatibility) > 0 {
			sb.WriteString("**Compatible with:** " + strings.Join(dbEntry.Compatibility, " · ") + "\n\n")
		}
		if dbEntry.FileInfoURL != "" {
			sb.WriteString("**Page:** [" + dbEntry.FileInfoURL + "](" + dbEntry.FileInfoURL + ")\n\n")
		}
		if dbEntry.Description != "" {
			sb.WriteString("---\n\n## Description\n\n")
			sb.WriteString(dbEntry.Description + "\n\n")
		}
	}

	changelog := addon.Changelog
	if changelog == "" && dbEntry != nil {
		changelog = dbEntry.Changelog
	}
	if changelog != "" && changelog != "None" {
		sb.WriteString("---\n\n## Changelog\n\n")
		sb.WriteString(changelog + "\n\n")
	}

	return sb.String()
}

// setupBrowseDetailViewport renders the selected browse addon's info into
// m.viewport and sizes it to fill the available terminal space.
func setupBrowseDetailViewport(m model) model {
	if m.selectedBrowseDBIdx >= len(m.addonDB) {
		return m
	}
	e := m.addonDB[m.selectedBrowseDBIdx]
	md := buildBrowseDetailMarkdown(e)

	var rendered string
	if m.glamourRenderer != nil {
		if out, err := m.glamourRenderer.Render(md); err == nil {
			rendered = out
		} else {
			rendered = md
		}
	} else {
		rendered = md
	}

	// Overhead: header(8) + name+blank(2) + scroll-indicator(2) + blank+action(2) + footer(2) = 16
	h := m.terminalHeight - 16
	if h < 5 {
		h = 5
	}
	m.viewport.SetHeight(h)
	m.viewport.SetWidth(m.terminalWidth - 4)
	m.viewport.SetContent(rendered)
	m.viewport.GotoTop()
	return m
}

// addonPath returns the AddOns directory for the given flavor
func addonPath(cfg Config, flavor string) string {
	if flavor == "classic" {
		return cfg.ClassicPath
	}
	return cfg.RetailPath
}
