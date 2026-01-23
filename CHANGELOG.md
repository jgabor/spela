# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.1.0]: https://github.com/jgabor/spela/tree/v0.1.0
