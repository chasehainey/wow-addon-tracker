# WoW Addon Tracker

A terminal UI for managing World of Warcraft addons from both GitHub and WoWInterface, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

```
█   █  ███  █   █      ███  ████  ████   ███  █   █     █████ ████   ███   ████ █  █  █████ ████
█   █ █   █ █   █     █   █ █   █ █   █ █   █ ██  █       █   █   █ █   █ █     █ █   █     █   █
█ █ █ █   █ █ █ █     █████ █   █ █   █ █   █ █ █ █       █   ████  █████ █     ██    ████  ████
██ ██ █   █ ██ ██     █   █ █   █ █   █ █   █ █  ██       █   █  █  █   █ █     █ █   █     █  █
█   █  ███  █   █     █   █ ████  ████   ███  █   █       █   █   █ █   █  ████ █  █  █████ █   █
```

## Features

- **Dual source support** — track addons installed from GitHub releases or WoWInterface
- **Browse & install** — search 7,500+ WoWInterface addons with full descriptions, changelogs, download counts, and compatibility info
- **Retail / Classic filtering** — separate flavor tabs for both the installed list and the browse panel
- **Automatic update checks** — detects new versions for GitHub addons and WoWInterface addons alike
- **Batch operations** — update all outdated addons in one keystroke
- **Profiles** — group addons into named profiles (e.g. by character or role)
- **Full addon detail** — scrollable markdown view with description, changelog, author, version history, and download stats
- **Mouse support** — click tabs, rows, and buttons; scroll with the mouse wheel
- **Hot / New feeds** — browse trending and recently updated WoWInterface addons via live RSS
- **Keyboard-first** — full keyboard navigation with a context-sensitive help bar

## Installation

### Pre-built binary

Download the latest release from the [Releases](../../releases) page and place the binary somewhere on your `$PATH`.

### Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/chasehainey/wow-addon-tracker.git
cd wow-addon-tracker
go build -o wow-addon-tracker .
```

## Usage

```bash
./wow-addon-tracker
```

On first run the app creates `~/.config/wow-addon-tracker/config.json` (or `$XDG_CONFIG_HOME/wow-addon-tracker/config.json`).

Open **Settings** (`s`) to configure your AddOns paths:

| Setting | Example |
|---|---|
| Retail path | `/Applications/World of Warcraft/_retail_/Interface/AddOns` |
| Classic path | `/Applications/World of Warcraft/_classic_/Interface/AddOns` |
| GitHub token | Optional — raises the API rate limit from 60 to 5,000 req/hr |

## Key Bindings

### Dashboard

| Key | Action |
|---|---|
| `a` | Add addon (GitHub repo or WoWInterface URL/ID) |
| `enter` | Open addon / browse item detail |
| `u` | Update selected addon |
| `U` | Update all outdated addons |
| `d` | Delete selected addon |
| `p` | Profiles |
| `s` | Settings |
| `Tab` | Switch focus between Installed and Browse panels |
| `j` / `k` or arrows | Move cursor |
| `/` | Filter installed addons |
| `q` / `Esc` | Quit |

### Detail Views

| Key | Action |
|---|---|
| `j` / `k` or arrows | Scroll |
| `PgDn` / `PgUp` | Page scroll |
| `i` / `enter` | Install (browse detail) |
| `u` | Update (addon detail) |
| `d` | Delete (addon detail) |
| `Esc` / `q` | Back |

### Browse Panel

| Key | Action |
|---|---|
| `Space` | Select / deselect addon for batch install |
| `enter` | Open detail or batch-install selected |
| `1` / `2` / `3` | Switch All / Hot / New tabs |

## Addon Sources

### GitHub

Paste a GitHub repo in the form `Owner/Repo` (e.g. `WeakAuras/WeakAuras2`). The tracker fetches the latest release, downloads the zip, and extracts it into your AddOns directory.

### WoWInterface

Paste a WoWInterface addon URL or numeric ID, or browse and install directly from the Browse panel. WoWInterface addons are sourced from the embedded database (7,500+ addons, updated via `go run ./cmd/bootstrap-db`).

## Rebuilding the Addon Database

The embedded `assets/addon-db.json` is built from the public MMOUI API. To refresh it:

```bash
go run ./cmd/bootstrap-db
```

This fetches the full addon list (~7,500 entries) and their details in parallel, converts BBCode descriptions to Markdown, and writes a new `assets/addon-db.json`. Re-build the binary afterwards to embed the updated database.

## Configuration

Config is stored in JSON at:

- **macOS / Linux**: `~/.config/wow-addon-tracker/config.json`
- **Windows**: `%APPDATA%\wow-addon-tracker\config.json`

```json
{
  "retail_path": "/path/to/_retail_/Interface/AddOns",
  "classic_path": "/path/to/_classic_/Interface/AddOns",
  "github_token": "",
  "addons": [],
  "profiles": []
}
```

## License

MIT
