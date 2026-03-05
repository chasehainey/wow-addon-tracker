package main

import (
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
		updateQueue:     []string{},
		updateAllErrors: []string{},
		dbSuggestionIdx:  -1,
		browseDBIndices:  []int{},
		browseDBSelected: make(map[int]struct{}),
		browseInstallFlavor: "retail",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadConfig(), loadAddonDB(), fetchRemoteDB())
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

// addonPath returns the AddOns directory for the given flavor
func addonPath(cfg Config, flavor string) string {
	if flavor == "classic" {
		return cfg.ClassicPath
	}
	return cfg.RetailPath
}
