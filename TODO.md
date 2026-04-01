# TODO

## ⇶ Critical

- [x] ~~GUI/TUI launch bypasses cleanup pipeline~~ — fixed: both now register RestorePoint and cleanup closures
- [x] ~~Competing logging patterns~~ — fixed: all log.Printf replaced with centralized slog logging
- [x] ~~Global mutable state in TUI styles~~ — fixed: Styles struct threaded by pointer through all models

## ⇉ Degraded

- [ ] DLSS-D column missing from GUI DLL display — `internal/gui/frontend/src/lib/GameDetail.svelte:404-416`
- [ ] No DLL operation progress indicator in GUI — `internal/gui/frontend/src/lib/GameDetail.svelte`
- [ ] DLL operation error messages incomplete in GUI — database save errors not surfaced

## ⇢ Annoying

- [ ] No CLI commands for GPU/CPU/Overlay/Ludusavi profile settings — fields exist but no subcommands (also "coming soon" in TUI)

## Resolved

- [x] ~~DLL database not persisted after operations~~ — fixed in prior refactoring
- [x] ~~Game launch bypasses launcher package~~ — fixed in prior refactoring
- [x] ~~Missing profile fields~~ — fixed in prior refactoring
- [x] ~~Incomplete DLSS set command flags~~ — fixed in prior refactoring
- [x] ~~NVML setter privilege model undecided~~ — migrated to batched pkexec apply-profile with go-nvml setters
