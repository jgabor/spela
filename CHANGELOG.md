# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Thermal color gradient system for TUI metrics — continuous blue-to-red coloring based on metric severity
- Expanded theme token system with surface palette, text hierarchy, and metric-specific colors
- `spela proton set/show` commands for per-game HDR, Wayland, NGX updater settings
- Per-game GPU power limit profile support with apply-at-launch and restore-on-exit
- Enhanced `spela gpu info` with power limit range, clocks, fan speed, and GPU utilization
- Enhanced `spela cpu info` with frequencies, load, RAM, available governors, and SCX status
- GPU profile flags: `--shader-cache`, `--shader-cache-path`, `--threaded-opt`

### Changed

- Update transitive dependencies (x/crypto, x/net, x/text, wails, bubbles, echo)
- Consolidate launch orchestration into single entry point for CLI, GUI, and TUI
- TUI theme and styles passed through model tree instead of global state
- Profile widget fields use declarative registry with apply closures instead of switch

### Fixed

- Restore GPU clocks, CPU governor, and environment variables on game exit when launched from GUI or TUI
- Overlay collector now starts for GUI and TUI launches (previously CLI-only)
- All log output now routes through centralized slog logging for consistent level control
- Test coverage added for config, cpu, and launcher packages

## [0.2.1] - 2026-03-13

### Fixed

- Use `LoadEffective` in wrapper mode so the default profile is applied for games without a game-specific profile
- Treat profile load errors as non-fatal warnings instead of aborting the game launch
- Replace `.gitkeep`-based frontend dist directory with build-tag-guarded embed (`embed_assets`) so CI builds no longer depend on a tracked placeholder file

## [0.2.0] - 2026-03-02

### Added

- Add missing profile fields: DLSS ray reconstruction, frame generation, GPU clock offsets, CPU settings, overlay settings, and Ludusavi restore (TUI and GUI)
- Add missing `dlss set` CLI flags: `--sr-model-preset`, `--rr-preset`, `--rr-override`, `--fg-override`, `--fg-indicator`
- Add distinct dark and light themes with theme switcher in options modal
- Add "Coming soon" label for profile settings not yet wired to backend
- Add initial game auto-selection on TUI startup
- Add unknown profile value indicator (? prefix) for non-standard values

### Changed

- Remap game launch key from `l` to `L` (Shift+L) to free vim-style navigation
- Change `q` key to always back-navigate (content → sidebar → quit)
- Use `sort.SliceStable` for deterministic game list ordering
- Pre-compute profile existence map before sorting to reduce filesystem calls
- Scope deselect-all (`A`) to current filter view instead of clearing all selections
- Preserve cursor position on filter changes instead of resetting to top
- Replace hardcoded colors with theme references throughout TUI
- Use launcher package for game launching in TUI and GUI (enables Ludusavi backup and signal handling)
- Deduplicate DLSS preset metadata using canonical profile package source

### Fixed

- Fix messages dropped when switching focus during async DLL load
- Fix data race on game.DLLs mutation from background goroutines
- Fix DLL install/update/restore errors silently swallowed with no user feedback
- Fix `dllOperating` guard never set, allowing concurrent DLL operations
- Fix DLSS-G versions never displayed due to column key mismatch
- Fix DLSS-D type detected but missing from display columns (TUI and GUI)
- Fix launch errors not shown to user
- Fix options save error silently treated as cancel
- Fix options modal centering off by padding amount
- Fix profile widget grid navigation (left/right now moves between columns)
- Fix zombie process from `cmd.Start()` without `cmd.Wait()`
- Fix DLL version metadata not persisted to database after update/install/restore (TUI, GUI, CLI)
- Fix DLL update progress not shown to user in TUI
- Fix GUI theme selector not accepting "light" theme

### Removed

- Remove dead `profile_editor.go` (~430 lines of unused code)

## [0.1.0] - 2026-01-23

This release introduces Spela as a comprehensive Linux gaming optimization tool for NVIDIA GPUs, featuring DLSS/DLL management and per-game profiles. It includes multiple interfaces (CLI, TUI with bubbletea, and Wails desktop app) along with game scanning, launch wrapper, and tuning capabilities. The release also adds DLSS Frame Generation and Ray Reconstruction support, unified theme with light/dark modes, and extensive CI/CD automation with AUR package publishing.

### Added

- Add CLI with game scanning, profiles, launch wrapper, and tuning
- Add bubbletea interface with game browser and monitoring
- Add Wails desktop app with game browser and monitoring
- Add repository manifest, downloader, backup, and swap
- Add CI/CD, AUR packages, ludusavi backup, and mangohud overlay
- Redesign layout with header, sidebar, and unified content
- Implement DLSS DLL infrastructure
- Use TechPowerUp as primary DLSS source with GitHub fallback
- Add model preset selection (K, L, M)
- Add GUI integration with Wails
- Enhance build system with frontend compilation and dev mode
- Add TUI enhancements
- Add DLSS-G and DLSS-D DLL support
- Add release automation and preparation for v0.1.0
- Add Playwright e2e tests
- Include LLM summary in CHANGELOG.md
- Automate AUR package publishing
- Polish layout and profile settings UI
- Filter out Proton and Steam tools from game lists
- Add options modal for global configuration
- Add unified spela theme with light/dark mode support
- Improve GUI feature parity with TUI
- Keep options and bindings in sync
- Keep game list sidebar visible
- Add header metrics panel
- Simplify header and options panel
- Add default profiles and dll install
- Align default profile and fg override state
- Align gui and tui parity behaviors
- Align visual language with tui

### Changed

- Gofumpt
- Add lefthook pre-commit config
- Add Wails generated bindings and dependencies
- Optimize spela.png
- Trigger DLSS manifest update on push to main
- Commit manifest updates directly to main
- Add DLSS 310.5.0 to manifest
- Make DLSS update workflow idempotent
- Disable Go cache to avoid tar warnings
- Fix Go version and golangci-lint compatibility
- Exclude GUI package and build golangci-lint from source
- Fix test paths and golangci-lint config
- Use v2 exclusions syntax for golangci-lint
- Lint only specific packages to avoid gui build issues
- Remove old standalone gui directory
- Add testing and CI enhancements for unified binary
- Add DLSS Frame Generation 310.5.0 to manifest
- Add DLSS Ray Reconstruction 310.5.0 to manifest
- Move DLL releases to separate spela-dlls repository
- Remove preset system, add DLSS 4/4.5 presets
- Ignore Wails generated artifacts
- Ignore beans tracking and tweak dll headers

### Documentation

- Add README with features and installation guide
- Add screenshot to README

### Fixed

- Update golangci-lint config for v2
- Resolve all golangci-lint errors
- Use official NVIDIA GitHub for DLSS updates
- Use DLL type instead of filename for manifest lookup
- Output compact JSON for CI parsing
- Handle parallel job race conditions in DLL manifest workflow
- DLL version detection and CI webkit package
- TUI install dialog shows error for empty DLL version lists
- Check fmt.Sscanf return value to satisfy linter
- Use dev build tag for lint job to skip embed directive
- Exclude gui package from linting (requires wails/embed)
- Correct golangci config location for exclude-dirs
- Use v2 linters.exclusions.paths for gui directory
- Create stub frontend/dist directory for CI linting
- Add explicit permissions to CI workflow
- Show reason when GUI falls back to TUI
- Find git-cliff in common install locations
- Add production build tag for Wails GUI
- Run go vet on whole project instead of individual files
- Add path editing support in options modal
- Only dim profile widget border, not content
- Remove duplicate Model setting from profile widget
- Use webkit2gtk-4.1 for Ubuntu 24.04 compatibility
- Correct Wails binding package name from main to gui
- Fix filter reactivity and dropdown styling
- Use custom Dropdown component for styled sort menu
- Use custom Dropdown component for game detail settings
- Adjust profile grid responsiveness
- Add webkit2_41 tag to bindings
- Allow dll install without detection
- Restore interactive redo flow
- Change summary model

[0.2.0]: https://github.com/jgabor/spela/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/jgabor/spela/tree/v0.1.0
