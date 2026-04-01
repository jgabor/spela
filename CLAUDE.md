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
- `internal/ludusavi/` - Ludusavi save game backup integration
- `internal/overlay/` - Overlay IPC protocol (mmap + seqlock), alert detection, stats collector
- `internal/privilege/` - Polkit (pkexec) privilege escalation, batched apply-profile
- `internal/profile/` - Per-game YAML profiles, apply/restore with cleanup closures
- `internal/steam/` - Steam library detection, VDF parsing
- `internal/tui/` - Bubbletea v2 interactive terminal UI
- `internal/update/` - Self-update checking
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

## Workflow

- Use beans for task tracking (not TodoWrite)
- Commit after completing each epic
- Use conventional commit format (feat:, fix:, refactor:, etc.)
