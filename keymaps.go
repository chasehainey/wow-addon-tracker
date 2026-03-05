package main

import "charm.land/bubbles/v2/key"

// Each key map type implements help.KeyMap so renderFooter() can call m.help.View(km).

// dashboardKeyMap is the key map for the main dashboard view.
type dashboardKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Tab    key.Binding
	Enter  key.Binding
	Filter key.Binding
	Add    key.Binding
	Check  key.Binding
	Update key.Binding
	Quit   key.Binding
}

func (k dashboardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Enter, k.Filter, k.Add, k.Quit}
}
func (k dashboardKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Tab, k.Enter},
		{k.Filter, k.Add, k.Check, k.Update, k.Quit},
	}
}

var dashboardKeys = dashboardKeyMap{
	Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch panel")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/install")),
	Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Check:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "check")),
	Update: key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "update all")),
	Quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
}

// addonListKeyMap is the key map for the addon list.
type addonListKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Filter key.Binding
	Add    key.Binding
	Check  key.Binding
	Update key.Binding
	Back   key.Binding
}

func (k addonListKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Filter, k.Add, k.Back}
}
func (k addonListKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Filter},
		{k.Add, k.Check, k.Update, k.Back},
	}
}

var addonListKeys = addonListKeyMap{
	Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
	Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Check:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "check")),
	Update: key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "update all")),
	Back:   key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "back")),
}

// addonDetailKeyMap is the key map for the addon detail view.
type addonDetailKeyMap struct {
	Update key.Binding
	Delete key.Binding
	Back   key.Binding
}

func (k addonDetailKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Update, k.Delete, k.Back}
}
func (k addonDetailKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Update, k.Delete, k.Back}}
}

var addonDetailKeys = addonDetailKeyMap{
	Update: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "update")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Back:   key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", "back")),
}

// browseDBKeyMap is the key map for the Browse Addons view.
type browseDBKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Filter key.Binding
	Enter  key.Binding
	Back   key.Binding
}

func (k browseDBKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Filter, k.Enter, k.Back}
}
func (k browseDBKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Select, k.Filter, k.Enter, k.Back}}
}

var browseDBKeys = browseDBKeyMap{
	Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Select: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select")),
	Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
	Back:   key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "back")),
}

// browseDetailKeyMap is the key map for the Browse addon detail view.
type browseDetailKeyMap struct {
	Install key.Binding
	ScrollU key.Binding
	ScrollD key.Binding
	Back    key.Binding
}

func (k browseDetailKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Install, k.ScrollU, k.ScrollD, k.Back}
}
func (k browseDetailKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Install, k.ScrollU, k.ScrollD, k.Back}}
}

var browseDetailKeys = browseDetailKeyMap{
	Install: key.NewBinding(key.WithKeys("i", "enter"), key.WithHelp("i/enter", "install")),
	ScrollU: key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "scroll up")),
	ScrollD: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "scroll down")),
	Back:    key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "back")),
}

// profileListKeyMap is the key map for the profile list.
type profileListKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	New   key.Binding
	Back  key.Binding
}

func (k profileListKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.New, k.Back}
}
func (k profileListKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Enter, k.New, k.Back}}
}

var profileListKeys = profileListKeyMap{
	Up:    key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:  key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
	New:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Back:  key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "back")),
}

// profileAddonKeyMap is the key map for the profile addon multi-select.
type profileAddonKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Toggle key.Binding
	Save   key.Binding
	Cancel key.Binding
}

func (k profileAddonKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Save, k.Cancel}
}
func (k profileAddonKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Toggle, k.Save, k.Cancel}}
}

var profileAddonKeys = profileAddonKeyMap{
	Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Toggle: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
	Save:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
	Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

// settingsKeyMap is the key map for the settings menu.
type settingsKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Back  key.Binding
}

func (k settingsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Back}
}
func (k settingsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Enter, k.Back}}
}

var settingsKeys = settingsKeyMap{
	Up:    key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:  key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
	Back:  key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "back")),
}

// inputKeyMap is the key map for text input screens.
type inputKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
}

func (k inputKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Confirm, k.Cancel}
}
func (k inputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Confirm, k.Cancel}}
}

var inputKeys = inputKeyMap{
	Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}
