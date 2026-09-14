# GUI integration technical design

This document describes the architecture for embedding the Wails GUI into the main spela binary.

## Overview

The goal is to create a single binary that supports three interfaces:

- **CLI**: Direct commands like `spela list`, `spela show`
- **TUI**: Terminal UI via `spela tui`
- **GUI**: Desktop GUI via `spela gui`

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     cmd/spela/main.go                       │
│                    (Cobra root command)                     │
├─────────────────────────────────────────────────────────────┤
│  commands/   │  commands/   │  commands/   │  commands/     │
│  list.go     │  tui.go      │  gui.go      │  ...           │
└──────┬───────┴──────┬───────┴──────┬───────┴────────────────┘
       │              │              │
       ▼              ▼              ▼
┌──────────────┐ ┌──────────┐ ┌──────────────────────────────┐
│ internal/*   │ │ internal │ │ internal/gui/                │
│ (shared)     │ │ /tui     │ │ ├── app.go                   │
│              │ │          │ │ ├── run.go                   │
│ - game       │ │          │ │ ├── assets_prod.go (embed)   │
│ - profile    │ │          │ │ ├── assets_dev.go (http)     │
│ - gpu        │ │          │ │ └── frontend/                │
│ - cpu        │ │          │ │     ├── src/ (Svelte)        │
│ - dll        │ │          │ │     └── dist/ (built)        │
│ - ...        │ │          │ │                              │
└──────────────┘ └──────────┘ └──────────────────────────────┘
```

## Key components

### 1. GUI command (`cmd/spela/commands/gui.go`)

```go
var GUICmd = &cobra.Command{
    Use:   "gui",
    Short: "Launch the desktop GUI",
    RunE:  runGUI,
}

func runGUI(cmd *cobra.Command, args []string) error {
    // Acquire instance lock
    if err := lock.Acquire(); err != nil {
        return fmt.Errorf("spela is already running: %w", err)
    }
    defer lock.Release()

    // Attempt to launch GUI
    if err := gui.Run(); err != nil {
        // Fallback to TUI silently
        return tui.Run()
    }
    return nil
}
```

### 2. GUI package (`internal/gui/`)

**run.go** - Main entry point:

```go
package gui

func Run() error {
    app := NewApp()

    return wails.Run(&options.App{
        Title:  "Spela",
        Width:  1024,
        Height: 768,
        AssetServer: &assetserver.Options{
            Assets: assets, // from assets_*.go
        },
        OnStartup:  app.startup,
        OnShutdown: app.shutdown,
        Menu:       buildMenu(app),
        Bind:       []interface{}{app},
    })
}
```

**assets_prod.go** - Production embedded assets:

```go
//go:build !dev

package gui

import "embed"

//go:embed all:frontend/dist
var assets embed.FS
```

**assets_dev.go** - Development proxy mode:

```go
//go:build dev

package gui

import "net/http"

var assets http.FileSystem = nil // nil triggers dev server mode
```

### 3. Instance locking (`internal/lock/`)

```go
package lock

import (
    "os"
    "path/filepath"
    "strconv"
    "syscall"
)

func pidFilePath() string {
    return filepath.Join(xdg.RuntimeDir(), "spela.pid")
}

func Acquire() error {
    // Check for existing lock
    if data, err := os.ReadFile(pidFilePath()); err == nil {
        pid, _ := strconv.Atoi(string(data))
        if processExists(pid) {
            return fmt.Errorf("PID %d", pid)
        }
        // Stale lock, remove it
        os.Remove(pidFilePath())
    }

    // Create new lock
    return os.WriteFile(pidFilePath(),
        []byte(strconv.Itoa(os.Getpid())), 0600)
}

func Release() {
    os.Remove(pidFilePath())
}

func processExists(pid int) bool {
    return syscall.Kill(pid, 0) == nil
}
```

## Build system

### Makefile targets

```makefile
BUN := $(shell command -v bun 2>/dev/null)

check-bun:
ifndef BUN
 $(error bun is required but not installed. Install from https://bun.sh)
endif

frontend-deps: check-bun
 cd internal/gui/frontend && bun install

frontend-build: frontend-deps
 cd internal/gui/frontend && bun run build

build: frontend-build
 go build -ldflags "-s -w -X main.version=$(VERSION)" -o spela ./cmd/spela

dev: frontend-deps
 cd internal/gui/frontend && bun run dev &
 go build -tags dev -o spela ./cmd/spela
 ./spela gui

clean:
 rm -f spela
 rm -rf internal/gui/frontend/dist
 rm -rf internal/gui/frontend/node_modules
```

### CI workflow updates

```yaml
- name: Install GUI dependencies
  run: |
    sudo apt-get update
    sudo apt-get install -y libwebkit2gtk-4.1-dev build-essential libgtk-3-dev

- name: Setup Bun
  uses: oven-sh/setup-bun@v1
  with:
    bun-version: latest

- name: Build
  run: make build

- name: Test frontend
  run: make test-frontend
```

## Development workflow

### Fast iteration on GUI

1. Start dev mode: `make dev`
   - Vite dev server runs on :5173 with HMR
   - Go binary built with `-tags dev` proxies to Vite
   - Svelte changes reflect instantly
   - Go changes require `make dev` restart

2. Stop dev mode: `make dev-stop` or Ctrl+C

### Production build

```bash
make build
./spela gui
```

## Testing strategy

### Unit tests

- Go: `go test ./...` (includes internal/gui/ App methods)
- Svelte: `bun run test` (Vitest + Testing Library)

### E2E tests

- Playwright tests launch full binary
- Verify critical flows: app launch, game list, profile editing
- Run in CI with xvfb for headless display

## Migration checklist

1. [ ] Create internal/gui/ package structure
2. [ ] Move and refactor gui/ code
3. [ ] Implement assets_prod.go and assets_dev.go
4. [ ] Create internal/lock/ package
5. [ ] Create cmd/spela/commands/gui.go
6. [ ] Update Makefile with new targets
7. [ ] Update CI workflow
8. [ ] Add frontend tests
9. [ ] Add e2e tests
10. [ ] Remove standalone gui/ directory
11. [ ] Remove wails.json
12. [ ] Update documentation
