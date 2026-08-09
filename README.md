<div align="center">

<img src="applogo.png" alt="Clip Logo" width="128" />

# Clip

**A simple, cross-platform RSS reader** — for macOS / Windows, built for fast browsing and distraction-free reading.

<img alt="Static Badge" src="https://img.shields.io/badge/Go-1.25.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/Wails-v3.0.0--alpha.98-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/React-18.2-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/TypeScript-6.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/Vite-8.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/Tailwind%20CSS-4.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/SQLite-1.52%20%28modernc%29-blue.svg">

[简体中文](README.zh.md) | English

</div>

## Screenshots

<div align="center">

<img src="screenshot/screenshot_1.png" alt="Clip screenshot 1" width="49%" />
<img src="screenshot/screenshot_2.png" alt="Clip screenshot 2" width="49%" />

</div>

## Features

- **Three-column layout**: source/folder tree · article list · reading view
- **Subscription management**: add feeds, organize into folders, OPML import/export
- **Scheduled fetching**: RSS / Atom parsing with background auto-refresh
- **Focused reading**: full-text reading view + distraction-free focus mode
- **Fast search**: global full-text search across titles, summaries, and notes
- **Notes**: attach notes to articles for later review
- **Desktop notifications**: new-article alerts, macOS Dock / Windows taskbar badges
- **Keyboard driven**: a complete set of shortcuts
- **Theme & i18n**: light / dark / follow-system, Chinese & English
- **Performance & offline**: lightweight list queries, local cache, readable offline

## Building from Source

### 1. Prepare the Development Environment

| Dependency | Version | Install |
|:-----------|:--------|:--------|
| Go | ≥ 1.25 | https://go.dev/dl/ |
| Node.js | ≥ 20 | https://nodejs.org/ |
| pnpm | ≥ 9 | `npm install -g pnpm` |
| Wails v3 CLI | latest | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` |

Platform-specific requirements:

- **macOS**: install Xcode Command Line Tools — `xcode-select --install`
- **Windows**: WebView2 Runtime (preinstalled on Windows 11)

### 2. Clone and Install Dependencies

```bash
# Clone the repository
git clone https://github.com/clip-rss/clip.git
cd clip

# Install frontend dependencies
cd frontend
pnpm install
cd ..

# Pull Go dependencies
go mod tidy
```

### 3. Development Mode (Hot Reload)

```bash
wails3 task dev
```

> Starts the Vite dev server and Go backend; changes apply live.

### 4. Build & Package

```bash
# Build the binary for the current platform
wails3 task build

# Package a distributable installer (macOS → .app / Windows → installer)
wails3 task package
```

Artifacts are output to the `bin/` directory.

### 5. Tests

```bash
# Go backend tests
go test ./...

# Frontend type check + unit tests
cd frontend
pnpm typecheck
pnpm test
```

### Extra: Regenerate Frontend Bindings

After modifying bound methods or Go models under `api/`, regenerate the frontend TS bindings:

```bash
wails3 generate bindings
```

Do not hand-edit files under `frontend/bindings/`.

## License

[MIT](LICENSE)
