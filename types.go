package main

import (
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/glamour"
)

// Theme represents a color theme
type Theme struct {
	Name       string
	Background string
	Foreground string
	Primary    string
	Secondary  string
	Accent     string
	Success    string
	Warning    string
	Error      string
	Muted      string
	Highlight  string
	Border     string
}

// Available themes (Dracula)
var (
	Dracula = Theme{
		Name:       "Dracula",
		Background: "#282a36",
		Foreground: "#f8f8f2",
		Primary:    "#bd93f9",
		Secondary:  "#8be9fd",
		Accent:     "#ff79c6",
		Success:    "#50fa7b",
		Warning:    "#f1fa8c",
		Error:      "#ff5555",
		Muted:      "#6272a4",
		Highlight:  "#ffb86c",
		Border:     "#44475a",
	}

	DraculaLight = Theme{
		Name:       "Dracula Light",
		Background: "#f8f8f2",
		Foreground: "#282a36",
		Primary:    "#7c3aed",
		Secondary:  "#0891b2",
		Accent:     "#db2777",
		Success:    "#16a34a",
		Warning:    "#ea580c",
		Error:      "#dc2626",
		Muted:      "#6b7280",
		Highlight:  "#d97706",
		Border:     "#d1d5db",
	}
)

// --- Persisted types (serialize to config.json) ---

type TrackedAddon struct {
	Name             string   `json:"name"`
	GithubRepo       string   `json:"github_repo"`            // "Owner/Repo" or "wowinterface:{ID}"
	WoWInterfaceID   int      `json:"wowi_id,omitempty"`      // set when installed from WoWInterface
	InstalledVersion string   `json:"installed_version"` // "v5.14.2" or ""
	InstalledDate    string   `json:"installed_date"`    // RFC3339
	LatestVersion    string   `json:"latest_version,omitempty"`  // last known latest release tag
	LatestDate       string   `json:"latest_date,omitempty"`     // last known latest release date
	Changelog        string   `json:"changelog,omitempty"`       // release notes / CHANGELOG.md content
	ExtractAs        string   `json:"extract_as,omitempty"`      // when set, all repo content goes into this folder name
	Directories      []string `json:"directories"`       // top-level folders in AddOns
	Profiles         []string `json:"profiles"`
	GameFlavor       string   `json:"game_flavor"` // "retail" | "classic"
}

type Profile struct {
	Name   string   `json:"name"`
	Addons []string `json:"addons"` // TrackedAddon.Name values
}

type Config struct {
	RetailPath  string         `json:"retail_path"`
	ClassicPath string         `json:"classic_path"`
	GithubToken string         `json:"github_token,omitempty"`
	Addons      []TrackedAddon `json:"addons"`
	Profiles    []Profile      `json:"profiles"`
}

// --- GitHub API response types ---

type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	PublishedAt string        `json:"published_at"`
	Body        string        `json:"body"`
	Assets      []GitHubAsset `json:"assets"`
	HTMLURL     string        `json:"html_url"`
	ZipballURL  string        `json:"zipball_url"`
	Prerelease  bool          `json:"prerelease"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
}

// --- Runtime-only view model ---

type UpdateStatus int

const (
	StatusUnknown     UpdateStatus = iota
	StatusUpToDate
	StatusUpdateAvail
	StatusNotInstalled
)

type AddonWithStatus struct {
	Addon         TrackedAddon
	LatestVersion string
	LatestDate    string
	Status        UpdateStatus
}

// --- Message types ---

type configLoadedMsg struct {
	config Config
	err    error
}

type configSavedMsg struct {
	err error
}

type releaseCheckMsg struct {
	repo    string
	release GitHubRelease
	err     error
}

type batchCheckCompleteMsg struct {
	results []AddonWithStatus
	err     error
}

type installCompleteMsg struct {
	repo        string
	version     string
	directories []string
	changelog   string
	extractAs   string // persisted back so future updates use the same folder name
	err         error
}

type addonDeletedMsg struct {
	name string
	err  error
}

type downloadTickMsg struct{}
type autoCheckTickMsg struct{}
type wowiRefreshMsg struct{} // triggers a background filelist refresh

type dbLoadedMsg struct {
	entries []AddonDBEntry
	err     error
}

type rssLoadedMsg struct {
	feedType string // "hot" | "new"
	ids      []int
	err      error
}


// AddonDBEntry is a single entry in the addon database (WoWInterface or GitHub).
type AddonDBEntry struct {
	WoWInterfaceID   int      `json:"wowi_id,omitempty"`          // WoWInterface addon ID (>0 for WoWI entries)
	Repo             string   `json:"repo,omitempty"`             // GitHub "Owner/Repo" (non-empty for GitHub entries)
	Name             string   `json:"name"`
	Author           string   `json:"author,omitempty"`
	Description      string   `json:"description,omitempty"`
	Changelog        string   `json:"changelog,omitempty"`
	LatestVersion    string   `json:"latest_version,omitempty"`
	LatestDate       string   `json:"latest_date,omitempty"`
	Downloads        int      `json:"downloads,omitempty"`
	DownloadsMonthly int      `json:"downloads_monthly,omitempty"`
	Favorites        int      `json:"favorites,omitempty"`
	CategoryID       int      `json:"category_id,omitempty"`
	DownloadURL      string   `json:"download_url,omitempty"`
	FileInfoURL      string   `json:"file_info_url,omitempty"`   // WoWInterface addon page URL
	Compatibility    []string `json:"compatibility,omitempty"`   // game version strings e.g. ["12.0.0","4.4.0"]
	ExtractAs        string   `json:"extract_as,omitempty"`
}

// --- model struct ---

type model struct {
	config           Config
	addonsWithStatus []AddonWithStatus

	// Dashboard (default view — replaces main menu)
	dashboardFocus string // "installed" | "browse"
	defaultFlavor  string // "retail" | "classic" — sidebar flavor toggle

	// Addon list (viewAddons)
	viewAddons           bool
	addonListCursor      int
	addonListFilter      string
	addonFilterActive    bool
	textInputAddonFilter textinput.Model
	addonFilteredIndices []int

	// Addon detail (sub-state of viewAddons)
	viewAddonDetail  bool
	selectedAddonIdx int

	// Add addon flow
	inputAddRepo      bool
	textInputRepo     textinput.Model
	addRepoFetching   bool
	pendingRelease    *GitHubRelease
	pendingRepo       string
	pendingFlavor     string // "retail" | "classic"
	pendingExtractAs  string // non-empty when DB or user specifies a target folder name
	addRepoConfirm    bool

	// Installing
	installing       bool
	updatingSingle   bool
	downloadProgress float64 // 0.0–1.0; animated fake progress during download

	// Check/update all
	checkingUpdates bool
	updatingAll     bool
	updateQueue     []string // repos to update in sequence
	updateQueueIdx  int
	updateAllErrors []string

	// Delete confirm
	confirmDelete bool

	// Profiles
	viewProfiles            bool
	profileListCursor       int
	viewProfileDetail       bool
	selectedProfileIdx      int
	inputNewProfile         bool
	textInputProfileName    textinput.Model
	selectModeProfileAddons bool
	profileAddonCursor      int
	profileAddonSelected    map[int]struct{}

	// Settings
	viewSettings             bool
	settingsCursor           int
	inputSettingsRetail      bool
	inputSettingsClassic     bool
	inputSettingsToken       bool
	textInputSettingsRetail  textinput.Model
	textInputSettingsClassic textinput.Model
	textInputSettingsToken   textinput.Model

	// Addon DB
	addonDB        []AddonDBEntry
	dbRefreshing   bool // true while background filelist refresh is in-flight

	// Typeahead (add-addon flow)
	dbSuggestions   []AddonDBEntry
	dbSuggestionIdx int // -1 = none; Tab cycles 0..N-1

	// Browse Addons view
	viewBrowseDB          bool
	browseDBCursor        int
	browseDBFilterActive  bool
	browseDBFilter        string
	textInputBrowseFilter textinput.Model
	browseDBIndices       []int
	browseDBSelected      map[int]struct{} // db entry indices
	browseTab             string           // "all" | "hot" | "new"
	hotIDs                []int            // WoWI IDs from hot.xml, in order
	latestIDs             []int            // WoWI IDs from latest.xml, in order
	rssHotLoaded          bool             // true once hot.xml response has arrived
	rssNewLoaded          bool             // true once latest.xml response has arrived

	// Browse flavor filter (independent of installed flavor)
	browseFlavor string // "retail" | "classic"

	// Browse addon detail view
	viewBrowseDetail    bool
	selectedBrowseDBIdx int // index into addonDB

	// Browse batch install
	browseInstallConfirm bool
	browseInstallFlavor  string
	browseInstalling     bool
	browseInstallQueue   []string
	browseInstallIdx     int

	// Quit confirmation popup
	confirmQuit      bool
	quitConfirmFocus string // "no" | "yes"

	// Markdown renderer (glamour) — created once, width updated on resize.
	glamourRenderer *glamour.TermRenderer

	// Global UI
	loading        bool
	spinner        spinner.Model
	progressBar    progress.Model
	errorMsg       string
	successMsg     string
	viewport       viewport.Model
	viewportReady  bool
	terminalWidth  int
	terminalHeight int
	theme          Theme
}
