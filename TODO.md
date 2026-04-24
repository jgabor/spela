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

- [x] ~~Audit 5 launch lifecycle split~~ — fixed in 51bc71f; wrapper launches use shared preparation and cleanup
- [x] ~~Audit 5 direct Steam URI lifetime mismatch~~ — fixed in 51bc71f and GUI boundary commits; users get `%command%` guidance
- [x] ~~Audit 5 profile and privileged input gaps~~ — fixed in 5a3d3e8; default errors, bool parsing, governors, and env behavior covered
- [x] ~~Audit 5 GUI domain/logging seam~~ — fixed in 8c5a0ed and 6e52e3f; GUI actions route through a boundary and shared logging
- [x] ~~Audit 5 TUI DLL workflow and profile-field drift~~ — fixed in 1778e67; services own DLL workflows and field display coverage exists
- [x] ~~Audit 5 TUI routing hotspot~~ — fixed in f2e2584; routing helpers preserve modal, resource, message, and DLL behavior
- [x] ~~Audit 5 frontend dependency health~~ — addressed in ab10940; exact pins added, semver-major advisory fix remains approval-blocked in PROGRESS.md
- [x] ~~Audit 5 release freshness~~ — addressed by local `v0.5.1` in a6ce00a and d76d253; remote push remains user-gated
- [x] ~~Audit 5 stale DESIGN/DOCS artifacts~~ — resolved by Task 8 checkpoint; DESIGN and DOCS reflect resource-centric neon TUI contracts
- [x] ~~DLL database not persisted after operations~~ — fixed in prior refactoring
- [x] ~~Game launch bypasses launcher package~~ — fixed in prior refactoring
- [x] ~~Missing profile fields~~ — fixed in prior refactoring
- [x] ~~Incomplete DLSS set command flags~~ — fixed in prior refactoring
- [x] ~~NVML setter privilege model undecided~~ — migrated to batched pkexec apply-profile with go-nvml setters
- [x] ~~Ludusavi save game integration~~ — removed entirely in 39a8cc6; feature to be rethought
- [x] ~~Dead code accumulation (16.6%)~~ — swept to 14.3% in 0687920; 49 functions, 3 files, 1 dep removed
- [x] ~~TUI test coverage (Tests: C in HEALTH.md)~~ — 99 state machine tests added across 6 test files (ab97dba..b7e3c3c); Services DI, model factories, layout/sidebar/content/profile widget coverage
- [x] ~~Launch-tab UX (99% of users never use it; launches go through Steam `%command%`)~~ — removed in v0.5.0 TUI redesign (8b4907f); launches stay in CLI
- [x] ~~Duplicate DLSS model presets in picker~~ — fixed in v0.5.0 (c694b47) via `dedupePresets` helper
- [x] ~~No profile reset / unclear default-vs-game relationship~~ — resolved in v0.5.0 by live inheritance (5788b6c, c694b47): inherited vs overridden markers, `r`/`shift+R` reset, `p` pin bindings, `spela <subsystem> reset` CLI verbs
- [x] ~~Profile grid misalignment and sequential navigation~~ — replaced in v0.5.0 (b7ffbe6) with single-column grouped-by-subsystem detail renderer and j/k field-by-field navigation across group boundaries
- [x] ~~Theme variant selector (Default/Dark/Light triad)~~ — collapsed in v0.5.0 (96b907b) to a single neon-accent dark palette with canonical tokens; legacy `theme:` values stripped on load
