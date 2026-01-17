# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

### Documentation

- Add README with features and installation guide

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


