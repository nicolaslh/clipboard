# 📋 Clipboard Manager

A lightweight, fast, local-first clipboard manager for macOS built with Wails v3.

## Features

### Core
- 🔄 **Auto capture** — Monitors system clipboard in real-time (text + images)
- 🖼️ **Image support** — Screenshots and copied images saved automatically with thumbnail preview
- 🔍 **Full-text search** — Fuzzy content matching with keyword highlighting
- 📅 **Date filter** — Date picker to view records from a specific day
- 📌 **Pin important items** — Pinned items are never cleared or expired
- 🗑️ **Bulk clear** — One-click clear of all unpinned records (with confirmation)

### Menu Bar Resident
- 🖥️ **System tray** — Lives in the menu bar, runs in background
- 🔒 **Close = hide** — Closing the window doesn't quit the app; clipboard monitoring continues
- 👆 **Click to toggle** — Click tray icon to show/hide the window
- 📋 **Right-click menu** — "Show Window" and "Quit" options

### Smart Categories
- Auto-detects content type: **Text**, **URL**, **Code**, **File Path**, **Email**, **Image**
- Color-coded tags for quick identification
- Filter by category

### Snippet Library
- Organize frequently used content into named groups
- Dedicated snippet view for managing reusable text

### Auto Expiry
- Default 30-day retention, expired unpinned items auto-deleted
- Runs cleanup on startup and hourly
- Configurable retention period (1–365 days) in settings
- Pinned items never expire

### Large Text Preview
- Records over 200 characters are truncated in the list
- "Expand" button opens a full-content modal with scroll and copy

### Search Highlighting
- Matching keywords highlighted in yellow
- Adapts to light/dark mode

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Framework | [Wails v3](https://v3.wails.io/) |
| Backend | Go 1.23 |
| Frontend | Vanilla JS + Vite |
| Storage | SQLite (WAL mode) |
| Clipboard | [golang.design/x/clipboard](https://pkg.go.dev/golang.design/x/clipboard) |
| Build | [Task](https://taskfile.dev/) |

## Getting Started

### Prerequisites

- Go 1.23+
- Node.js 18+
- [Wails v3 CLI](https://v3.wails.io/getting-started/installation/)
- [Task](https://taskfile.dev/)

### Install Wails CLI

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

### Development

```bash
wails3 dev
```

Frontend hot-reload + backend auto-rebuild.

### Build

```bash
wails3 task build
```

Output: `bin/clipboard`

### Package as .app

```bash
wails3 task darwin:package
```

Output: `bin/clipboard.app` — ready to run or drag into Applications.

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Cmd+Shift+V` | Toggle window visibility |
| `Cmd+F` | Focus search input |
| `Escape` | Clear search / close modal |
| Click text item | Copy text to clipboard |
| Click image item | Copy image to clipboard |

## Project Structure

```
.
├── main.go                  # App entry, window, tray, keybindings
├── clipboard_service.go     # Clipboard polling (text+image), categories, snippets, cleanup, image HTTP
├── store.go                 # SQLite layer, migrations, category detection, time queries
├── frontend/
│   ├── index.html           # Main page (filters, date picker, settings modal)
│   ├── src/
│   │   ├── main.js          # Frontend logic (search highlight, image render, snippets)
│   │   └── style.css        # Flat UI styles (light/dark mode)
│   ├── bindings/            # Wails auto-generated bindings (gitignored)
│   ├── vite.config.js
│   └── package.json
├── build/
│   ├── config.yml           # Wails build & dev_mode config
│   ├── appicon.png          # App icon 1024x1024
│   ├── trayicon.png         # Menu bar icon (template icon)
│   └── darwin/
│       ├── Info.plist        # macOS app metadata
│       └── icons.icns        # macOS icon bundle
├── Taskfile.yml             # Build task definitions
├── go.mod
└── go.sum
```

## Design Decisions

- **Polling over Watch** — `golang.design/x/clipboard`'s Watch requires the main thread Cocoa event loop on macOS, which conflicts with Wails WebView. Switched to 500ms polling.
- **Images stored as base64** — Small images stored directly in SQLite as data URIs, served via HTTP route on demand. No filesystem management needed.
- **Image deduplication** — Short SHA256 hash comparison prevents saving duplicate images.
- **SQLite WAL mode** — Non-blocking concurrent reads/writes, suitable for frequent inserts.
- **Text deduplication** — Consecutive identical content is not re-saved; self-writes are skipped.
- **Auto migration** — Old databases get new columns added automatically on startup.
- **Menu bar resident** — Window close is intercepted via RegisterHook + Cancel(), app stays alive.
- **Flat UI** — Follows system light/dark mode with minimal decoration.

## License

MIT
