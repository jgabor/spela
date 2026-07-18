# Spela

Linux gaming optimization tool for NVIDIA GPUs with DLSS/DLL management and per-game profiles.

## Build

```bash
mage build    # Build binary with embedded frontend
mage test     # Run tests
mage lint     # Run linter
mage install  # Install to GOPATH/bin
```

## Project structure

- `cmd/spela/` - CLI entry point, cobra commands, wrapper mode detection
- `internal/config/` - Global YAML configuration
- `internal/cpu/` - CPU governor, SMT, affinity via sysfs
- `internal/denylist/` - Anti-cheat DLL swap deny list
- `internal/dll/` - DLL detection, download, backup, restore, version parsing
- `internal/env/` - Environment variable builder for game launch
- `internal/game/` - Game database, Steam manifest parsing
- `internal/gpu/` - NVIDIA GPU metrics and control via NVML and nvidia-smi
- `internal/gui/` - Wails v2 + Svelte graphical interface
- `internal/launcher/` - Game launch orchestration, signal forwarding, cleanup
- `internal/lock/` - File-based process locking
- `internal/logging/` - Centralized slog-based logging
- `internal/overlay/` - Overlay IPC protocol (mmap + seqlock), alert detection, stats collector
- `internal/privilege/` - Polkit (pkexec) privilege escalation, batched apply-profile
- `internal/profile/` - Per-game YAML profiles, apply/restore with cleanup closures
- `internal/steam/` - Steam library detection, VDF parsing
- `internal/tui/` - Bubbletea v2 interactive terminal UI
- `internal/xdg/` - XDG Base Directory path resolution
- `data/polkit/` - Polkit policy for privileged operations
- `docs/` - Design documents and reference material

## Code style

- Go 1.25+
- Use `gopkg.in/yaml.v3` for YAML
- Use `github.com/spf13/cobra` for CLI
- XDG Base Directory compliance for all paths
- No abbreviations in identifiers
- Centralized logging via `internal/logging` (not `log.Printf`)

## Privilege model

GPU clocks, power limits, CPU governor, and SMT are applied via a single batched
`pkexec` round-trip using `privilege.ExecSelf("apply-profile", ...)`. The hidden
`apply-profile` command runs with root privileges and uses go-nvml setters directly.
Profile.Apply() returns cleanup closures that restore previous state on game exit.

## Testing

Primary test target: Cyberpunk 2077 (AppID 1091500)

```bash
mage test            # run all tests
go test ./...        # alternative
```

### TUI testing

Use [Terminal Control](https://github.com/anomalyco/terminal-control) for every
change that can affect Spela's interactive terminal. Build the compiled
executable, run it in a named session, wait for visible content instead of
sleeping, and read the rendered terminal with `termctrl show`:

```bash
mage build
termctrl start spela-tui-review --cols 120 --rows 40 -- ./spela tui
termctrl wait spela-tui-review "Library" --timeout 20000
termctrl status spela-tui-review
termctrl show spela-tui-review
```

Use a unique session name when reviews may run concurrently, and use that name
for every later command. Send text and keys as separate arguments:

```bash
termctrl send spela-tui-review 'text:?'
termctrl wait spela-tui-review "Keyboard shortcuts" --timeout 20000
termctrl show spela-tui-review
termctrl send spela-tui-review escape
```

Visual review means reading the visible screen returned by `termctrl show`.
Do not substitute process logs, raw stdout, screenshots, or unit tests. Exercise
relevant keyboard paths, then inspect standard and constrained layouts:

```bash
termctrl resize spela-tui-review --cols 80 --rows 24
termctrl show spela-tui-review
termctrl resize spela-tui-review --cols 120 --rows 40
termctrl show spela-tui-review
```

Inspect the final screen before stopping a failed or exited session. Always stop
named sessions when finished:

```bash
termctrl stop spela-tui-review
termctrl list
```

Use isolated XDG state when startup may rescan, and for profile mutations,
empty-state testing, and failure paths. Never approve privileged or destructive
actions against real game files during visual review. See
[`docs/tui-testing.md`](docs/tui-testing.md) for the complete setup, review
checklist, evidence requirements, and cleanup procedure.

## Workflow

- Use beans for task tracking (not TodoWrite)
- Commit after completing each epic
- Use conventional commit format (feat:, fix:, refactor:, etc.)
