# TODO

## ⇶ Critical

- [x] ~~GUI/TUI launch bypasses cleanup pipeline~~ — fixed: both now register RestorePoint and cleanup closures
- [x] ~~Competing logging patterns~~ — fixed: all log.Printf replaced with centralized slog logging
- [x] ~~Global mutable state in TUI styles~~ — fixed: Styles struct threaded by pointer through all models

## ⇉ Degraded

- [x] ~~DLSS-D column missing from GUI DLL display~~ — already present in GameDetail.svelte (stale entry)
- [x] ~~No DLL operation progress indicator in GUI~~ — fixed in c013f1d: backend emits `dll:progress` events at each stage; frontend shows current stage next to busy button
- [x] ~~DLL operation error messages incomplete in GUI~~ — fixed in c013f1d: all DLL ops wrap errors with stage context; failures shown in persistent dismissible banner instead of 3s toast

## ⇢ Annoying

- [x] ~~No CLI commands for overlay profile settings~~ — added `spela overlay set/show` with 8 flags in 6a54b25

## Resolved

- [x] ~~DLL database not persisted after operations~~ — fixed in prior refactoring
- [x] ~~Game launch bypasses launcher package~~ — fixed in prior refactoring
- [x] ~~Missing profile fields~~ — fixed in prior refactoring
- [x] ~~Incomplete DLSS set command flags~~ — fixed in prior refactoring
- [x] ~~NVML setter privilege model undecided~~ — migrated to batched pkexec apply-profile with go-nvml setters
- [x] ~~Ludusavi save game integration~~ — removed entirely in 39a8cc6; feature to be rethought
- [x] ~~Dead code accumulation (16.6%)~~ — swept to 14.3% in 0687920; 49 functions, 3 files, 1 dep removed
- [x] ~~TUI test coverage (Tests: C in HEALTH.md)~~ — 99 state machine tests added across 6 test files (ab97dba..b7e3c3c); Services DI, model factories, layout/sidebar/content/profile widget coverage
