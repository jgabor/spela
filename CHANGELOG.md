# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-04-19

Version 0.3.0 brings significant GPU monitoring improvements including an NVML backend for fast metric reading, GPU alert detection with throttle reason identification, and live in-game metrics via mmap IPC with sparkline and gauge visualizations. The TUI gains tab-based navigation (DLLs, Profile, Launch), density modes, modal overlays, and confirmation prompts for destructive DLL operations, while GPU profiles now support per-game power limits and fan speed control.

### Added

- Upgrade to bubbletea v2 and apply crush UI patterns
- Add NVML backend for fast GPU metric reading
- Add GPU alert detection system
- Integrate GPU alerts into header with colored metrics
- Add NVML throttle reason detection for precise alerts
- Add mmap IPC protocol with seqlock synchronization
- Add stats collector for periodic IPC writes
- Wire collector into launcher for live game metrics
- Batched pkexec apply-profile with NVML setters
- Add proton profile commands and complete GPU profile flags
- Add per-game GPU power limit to profile system
- Enhance gpu info with power limit range, clocks, fan, and utilization
- Enhance cpu info with frequencies, load, RAM, available governors, and SCX status
- Add thermal gradient system and expanded theme tokens
- Add thermal gradient system and expanded theme tokens
- Add sparkline and gauge renderers with metrics buffer
- Add sparkline and gauge renderers with metrics buffer
- Add sparklines and gauges to header metrics display
- Add context-sensitive keybinding bar with disabled reasons
- Add sparklines/gauges to header and context-sensitive keybinding bar
- Add tab-based content views with DLLs, Profile, and Launch tabs
- Add tab-based content views with DLLs, Profile, and Launch tabs
- Add compositor-based modal overlays with cascading stack
- Add density modes (standard/compact/focused) and jump-key panel titles
- Add density modes, compositor modals, and jump-key panel titles
- Add color flash animation on message bar for operation feedback
- Add confirmation prompts for destructive DLL operations
- Add overlay set/show commands for per-game overlay profiles
- Enable overlay profile fields in TUI widget
- Improve profile show with formatted output and --json flag
- Add fan speed control to profile system
- Add fan speed field to GPU profile widget
- Add --dry-run flag to launch command
- Add --json flag to gpu show and overlay show commands
- Add --json flag to proton, cpu, and dlss show commands

### Changed

- Small code quality improvements
- Deduplicate tool name patterns between game and steam
- Extract Database.FindGame to eliminate duplicated lookups
- Remove dlssModeToEnv identity function
- Use FindGame in denylist commands
- Use CutPrefix and consolidate privilege exec helpers
- Use GetGame accessor and add HTTP timeouts to dlss-updater
- Fix deprecated API, use WalkDir, and add sentinel error
- Improve error handling and extract gameInfoFromGame helper
- Sort ListDLLNames at source, fix double map allocation
- Modernize loops and wrap SetSMT error in cpu package
- Extract GameDLLsFromDetected helper, sort profile list output
- Migrate sort to slices package, add version comparison tests
- Add comprehensive tests for deny list operations
- Modernize idioms, add tests, and fix minor issues
- Use strconv and strings.FieldsSeq for efficiency
- Migrate to .agentera/ artifact layout
- Convert docs/ISSUES.md to TODO.md
- Consolidate launch orchestration into Prepare()
- Replace global mutable styles with threaded *Styles
- Add persistence tests for YAML roundtrip, missing file, and malformed input
- Add tests with mock sysfs for governor, SMT, metrics, and affinity
- Add tests for cleanup order, signal forwarding, overlay IPC, and wrapper parsing
- Replace profile widget 54-case switch with field registry
- Update transitive dependencies
- Replace binary focus with navigation stack and breadcrumbs
- Merge branch 'worktree-agent-a1a635f0'
- Replace binary focus with navigation stack and breadcrumbs
- Merge branch 'worktree-agent-a5700507'
- Merge branch 'worktree-agent-a92ba1ae'
- Remove ludusavi save game integration
- Remove dead code and unused dependencies
- Plan-level freshness checkpoint and archive
- Introduce Services struct for dependency injection
- Add test helpers and model factories for state machine testing
- Add layout, navigation, and modal state machine tests
- Add sidebar state machine tests
- Add content and DLL operation state machine tests
- Add profile widget and disabled field contract tests
- Plan-level freshness checkpoint for TUI test harness plan
- Plan-level freshness checkpoint and archive
- Archive completed TUI test harness plan
- Reduce complexity by splitting three oversized files
- Archive TUI complexity plan and update artifacts

### Documentation

- Log utvecklarn cycles 6-7
- Refine vision, audit health, rewrite README, update CLAUDE.md
- Update plan, progress, and changelog for launch cleanup fix
- Update plan, progress, and changelog for launch consolidation
- Update plan, progress, and changelog for logging fix
- Update plan, progress, and changelog for TUI styles refactor
- Update plan and progress for config tests
- Update plan and progress for CPU tests
- Update plan, progress, and changelog for launcher tests
- Archive foundation hardening plan — all 8 tasks complete
- Update progress, todo, and changelog for CLI profile commands
- Update progress, todo, and changelog for dependency update
- Archive power limit plan, update progress and changelog
- Update progress and changelog for gpu info enhancement
- Update progress and changelog for cpu info enhancement
- Update progress and changelog for sparkline/gauge renderers
- Update progress and changelog for Tasks 5 and 6
- Log cycle 49 (GUI DLL progress + error context)
- Log inspektera audit 4

### Fixed

- Use XDG state directory instead of ~/logs
- Add HTTP client timeouts to prevent indefinite hangs
- Check download close error, use XDG for MangoHud logs, precompile regex
- Guard against nil logger and invalid appID in manifest parsing
- Batch menu bounds, env var restore correctness, test resource leaks
- Clean up partial file on copy error, remove stale +build lines
- Use LoadEffective in launch command, propagate ResetClocks errors
- Update name when modifying denied entry
- Gate Wails code behind build tags, remove TUI fallback
- Include bindings tag in build constraints
- Register cleanup closures in GUI and TUI launch paths
- Replace log.Printf with centralized slog logging
- Make jump keys 2/3/4 global and fix F5/F11 key matching
- Surface DLL operation progress and complete error context

## [0.2.1] - 2026-03-13

### Fixed

- Use LoadEffective and treat profile errors as non-fatal
- Add tests and update generated files

## [0.2.0] - 2026-03-02

### Added

- Add pkexec-based privilege escalation

### Changed

- Add DLSS Frame Generation 310.5.3 to manifest
- Add DLSS 310.5.3 to manifest
- Update editor, linter, and git hooks configuration
- Optimize workflows and fix AUR publish

### Fixed

- Restore frontend dist .gitkeep lost during rebase

## [dll-dlssg-v310.5.3] - 2026-01-31

### Changed

- Add DLSS Ray Reconstruction 310.5.3 to manifest

## [dll-dlssd-v310.5.3] - 2026-01-23

### Fixed

- Install wails cli for builds
- Avoid e2e on failed builds
- Ensure frontend dist for embed

## [0.1.0] - 2026-01-23

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

[0.3.0]: https://github.com/jgabor/spela/compare/v0.2.1..v0.3.0
[0.2.1]: https://github.com/jgabor/spela/compare/v0.2.0..v0.2.1
[0.2.0]: https://github.com/jgabor/spela/compare/dll-dlssg-v310.5.3..v0.2.0
[dll-dlssg-v310.5.3]: https://github.com/jgabor/spela/compare/dll-dlssd-v310.5.3..dll-dlssg-v310.5.3
[dll-dlssd-v310.5.3]: https://github.com/jgabor/spela/compare/v0.1.0..dll-dlssd-v310.5.3
[0.1.0]: https://github.com/jgabor/spela/tree/v0.1.0
