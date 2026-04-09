# TODO

## ⇶ Critical

- [x] ~~GUI/TUI launch bypasses cleanup pipeline~~ — fixed: both now register RestorePoint and cleanup closures
- [x] ~~Competing logging patterns~~ — fixed: all log.Printf replaced with centralized slog logging
- [x] ~~Global mutable state in TUI styles~~ — fixed: Styles struct threaded by pointer through all models

## ⇉ Degraded

- [x] ~~DLSS-D column missing from GUI DLL display~~ — already present in GameDetail.svelte (stale entry)
- [ ] No DLL operation progress indicator in GUI — `internal/gui/frontend/src/lib/GameDetail.svelte`
- [ ] DLL operation error messages incomplete in GUI — database save errors not surfaced

## ⇢ Annoying

- [ ] No CLI commands for overlay profile settings — overlay fields have no CLI subcommands yet

## Resolved

- [x] ~~DLL database not persisted after operations~~ — fixed in prior refactoring
- [x] ~~Game launch bypasses launcher package~~ — fixed in prior refactoring
- [x] ~~Missing profile fields~~ — fixed in prior refactoring
- [x] ~~Incomplete DLSS set command flags~~ — fixed in prior refactoring
- [x] ~~NVML setter privilege model undecided~~ — migrated to batched pkexec apply-profile with go-nvml setters
- [x] ~~Ludusavi save game integration~~ — removed entirely in 39a8cc6; feature to be rethought
- [x] ~~Dead code accumulation (16.6%)~~ — swept to 14.3% in 0687920; 49 functions, 3 files, 1 dep removed
