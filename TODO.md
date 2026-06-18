# TODO

## ⇶ Critical

- [x] ~~GUI/TUI launch bypasses cleanup pipeline~~ — fixed: both now register RestorePoint and cleanup closures
- [x] ~~Competing logging patterns~~ — fixed: all log.Printf replaced with centralized slog logging
- [x] ~~Global mutable state in TUI styles~~ — fixed: Styles struct threaded by pointer through all models

## ⇉ Degraded

- [x] ~~**`GameDetail.svelte` god-component split**~~ — extracted `GameDLLPane.svelte`, `GameProfilePane.svelte`, and `profileFieldOptions.js`; orchestrator now 401 lines
- [ ] **TUI stale game DB on launch** — `list`/`show` fail until manual `spela scan`; Satisfactory (custom Steam root) missing from cold DB; auto-scan or rescan prompt when DB empty/stale (rmux e2e 2026-06-18)
- [x] ~~**TUI status bar shows `u:update` when no updates exist**~~ — fixed: nav.ContentHints gating, message bar feedback, aspect forward from Content — `nav.contentZoneKeys()` always emits update hint; inline DLL hints correctly omit it; `u` silently no-ops with no message bar feedback (rmux e2e 2026-06-18)
- [ ] **Go→JS nav binding drift risk** — `navState.js` hand-ports `internal/nav`; add Wails export or codegen so GUI transitions cannot silently diverge (`.agentera/plan.yaml` deferred)
- [ ] **GUI DLL Catalog parity** — `DLLCatalogPane.svelte` is a thin stub; TUI has full library inventory + deployment matrix + update-all
- [ ] **`SupportsVKD3DHeap` detection stale for Proton-CachyOS 11.0+** — greps for removed `PROTON_VKD3D_HEAP` marker; Proton-CachyOS now needs only `VKD3D_CONFIG=descriptor_heap`, so compatible builds may false-negative
- [x] ~~TUI focus state too subtle~~ — zone indicator in status bar, column focus markers (▸), profile row cursor (`>`)
- [x] ~~TUI Tab model inconsistent~~ — help documents Tab/Esc zone flow; status bar shows active zone
- [x] ~~DLSS-D column missing from GUI DLL display~~ — already present in GameDetail.svelte (stale entry)
- [x] ~~No DLL operation progress indicator in GUI~~ — fixed in c013f1d: backend emits `dll:progress` events at each stage; frontend shows current stage next to busy button
- [x] ~~DLL operation error messages incomplete in GUI~~ — fixed in c013f1d: all DLL ops wrap errors with stage context; failures shown in persistent dismissible banner instead of 3s toast

## ⇢ Annoying

- [ ] **TUI command palette** — help lists `:` as deferred; no palette implementation yet
- [x] ~~**TUI no feedback when `u` pressed with DLLs up to date**~~ — fixed: nav.ContentHints gating, message bar feedback, aspect forward from Content — show message bar hint (e.g. "DLLs already up to date") when `hasUpdates=false`
- [x] ~~**TUI aspect hotkeys silently ignored outside Context zone**~~ — fixed: nav.ContentHints gating, message bar feedback, aspect forward from Content — `1`/`2`/`3` only work in Context; forwarded from Content zone
- [ ] **TUI search filter does not commit game scope until Enter** — `/` filters sidebar but breadcrumb stays on prior scope until confirmation (rmux e2e 2026-06-18)
- [ ] **rmux ANSI focus verification** — rmux e2e validated Library→game→DLLs flow and letter keys; arrow escape sequences and help overlay layout in 120×40 panes still unchecked
- [ ] **Frontend-owned option/DLL labels** — DLSS presets and DLL type labels live in `GameDetail.svelte` while semantics come from backend; promote to catalog metadata if option churn increases (HEALTH.md Audit 9)
- [ ] **Table-driven Go↔JS nav parity tests** — expand beyond spot checks so every `SelectDestination` / `SelectScope` / `SelectAspect` rule in `internal/nav` has a matching JS assertion
- [x] ~~TUI profile rows lack action affordances~~ — focused-row hints for reset/pin on game profiles
- [x] ~~Add rmux arrow-key regression coverage~~ — detail test uses KeyDown code path documented for rmux escape sequences
- [x] ~~Align default-profile path docs with implementation~~ — README uses `profiles/default.yaml`
- [x] ~~No CLI commands for overlay profile settings~~ — added `spela overlay set/show` with 8 flags in 6a54b25

## Resolved

- [x] ~~Three-zone navigation wiring~~ — `internal/nav` + settings catalog + TUI/GUI shell parity; P0 hotkey gate and DLL section filtering; rail syncs from `navState` (`.agentera/plan.yaml`, thermo review fixes)
- [x] ~~Default-profile semantics in GUI boundary~~ — `getDefaultProfile()` now supplies `Explain` metadata; `TestGUIBoundaryDefaultProfileSemanticsPass`
- [x] ~~Gova-inspired GUI seam coupling and behavior coverage~~ — delivered in 5743044..98ad89a; Task 7 found no open GUI-seam follow-up.
- [x] ~~Trusted profile loop shared semantics~~ — delivered in fed4ac0; effective values report source, launch impact, and restore coverage.
- [x] ~~Trusted profile loop launch preparation visibility~~ — delivered in 60152a4; CLI and wrapper summaries show planned environment, DLL, hardware, overlay, and compatibility impacts.
- [x] ~~Trusted profile loop restore confidence~~ — delivered in 32ce2dc; cleanup outcomes and DLL restore readiness are visible without hiding launch results.
- [x] ~~Trusted profile loop TUI and GUI inspection parity~~ — delivered in 80ab1c9 and 4b3d68c; both surfaces use shared profile semantics.
- [x] ~~Trusted profile loop guidance and release state~~ — delivered in 040c8c8 and 9b5f81f; README guidance is current and local `v0.6.0` release state exists.
- [x] ~~Audit 6 GUI backend tests outside default `mage test`~~ — fixed by running tagged GUI backend tests from `mage test`
- [x] ~~Audit 6 stale Playwright e2e harness~~ — fixed by rebasing fixtures on `window.go.gui.App` and current GUI resource/detail views
- [x] ~~Audit 6 frontend audit advisories~~ — fixed by upgrading to exact-pinned Vite 8, Svelte 5, and plugin 7; `npm audit` reports 0 vulnerabilities
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
