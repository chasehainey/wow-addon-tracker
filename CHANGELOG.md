# Changelog

All notable changes to WoW Addon Tracker are documented here.

## [v0.0.2] - 2026-03-05

### Added

- **Browse panel** — searchable list of 7,500+ WoWInterface addons embedded directly in the binary
- **Retail / Classic flavor tabs** — separate tabs in both the Installed and Browse panels; Retail = addons compatible with `12.x.x`, Classic = everything else
- **Browse detail view** — clicking an addon in the browse list opens a full-screen scrollable page with description, changelog, downloads, favorites, compatibility, and page link
- **Addon detail overhaul** — installed addon detail now uses a scrollable viewport with glamour-rendered Markdown, including description and changelog pulled from the embedded DB
- **Description in install confirm** — addon description is shown in the confirm screen before installing from the browse list
- **Source column** — installed addon list now shows `GH` (GitHub) or `WoW:I` (WoWInterface) as a Source column
- **Clickable tabs** — All / Hot / New browse tabs and Retail / Classic flavor tabs are mouse-clickable
- **Hot / New RSS feeds** — Hot and New browse tabs are populated from live WoWInterface RSS feeds (ISO-8859-1 encoding handled correctly)
- **Sorted browse list** — All tab sorted by total downloads descending
- **Full addon database fields** — database now includes `description`, `changelog`, `downloads_monthly`, `favorites`, `compatibility`, `file_info_url`, and `download_url` for every entry
- **BBCode → Markdown conversion** — all WoWInterface descriptions and changelogs are converted to Markdown at DB build time
- **Mouse wheel scrolling** — mouse wheel now scrolls the viewport in both the addon detail and browse detail views
- **Initializing screen** — a clean "Initializing..." message is shown on startup before terminal dimensions are known, preventing a scrunched layout flash
- **`cmd/bootstrap-db` tool** — standalone tool to rebuild `assets/addon-db.json` from the MMOUI API with full metadata and BBCode-to-Markdown conversion

### Fixed

- Hot and New browse tabs were empty due to ISO-8859-1 XML encoding declaration; fixed by rewriting to UTF-8 before parsing
- BBCode URLs with quoted attributes (e.g. `[url="https://..."]`) were being rendered with literal quotes; stripped correctly now
- Viewport height calculations corrected to prevent content overflowing below the terminal edge
- Browse addon detail description now appears immediately on open (moved before metadata) rather than requiring the user to scroll past stats
- Non-key messages (window resize, mouse events) are now correctly forwarded to the viewport in the addon detail view

## [v0.0.1] - 2026-03-04

### Added

- Initial release
- Track addons from GitHub releases and WoWInterface
- Automatic update checks with 30-minute recurring background polling
- Batch update all outdated addons
- Install addons from GitHub repos — fetches latest release, downloads zip, extracts into AddOns directory
- Addon profiles — group addons by character or role
- Retail / Classic game flavor support with separate AddOns paths
- Settings screen for configuring paths and GitHub token
- Dracula and Dracula Light color themes
- Full keyboard navigation with context-sensitive help bar
- Mouse support via BubbleZone
- Spinner and progress bar during downloads
- Persistent config stored as JSON
