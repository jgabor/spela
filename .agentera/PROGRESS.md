# Progress

## Cycle 38 — 2026-04-10 (plan summary: TUI End-to-End Test Harness)

**Phase**: build
**What**: Completed 7-task plan — TUI end-to-end test harness with injectable Services struct, model factories, key-sequence helpers, and 99 state machine tests across 6 test files covering layout (30 tests), sidebar (19), content/DLL (24), profile widget/disabled fields (16), plus factory smoke tests (10). Services DI for synchronous I/O enables pure state-machine testing without filesystem or hardware.
**Commits**: ab97dba refactor(tui): introduce Services struct for dependency injection · 996519c test(tui): add test helpers and model factories · 2b4c351 test(tui): add layout, navigation, and modal state machine tests · ce5a138 test(tui): add sidebar state machine tests · 3ea0a4e test(tui): add content and DLL operation state machine tests · b7e3c3c test(tui): add profile widget and disabled field contract tests · 9a1a816 chore: plan-level freshness checkpoint
**Discovered**: HEALTH.md Tests: C finding for TUI partially resolved — TUI package now has 99 model/Update tests (up from 0). Worktree agents consistently branched from pre-Services revision; direct implementation was more reliable for test code. SidebarModel and ContentModel aren't tea.Model implementations — tests call Update directly. Structural disabled-field iteration pattern is resilient to field additions.
**Verified**: N/A: docs-only
**Next**: Vision-driven work. GUI DLL progress indicator (degraded), overlay CLI commands (annoying), or Vulkan layer PoC (vision direction).
**Context**: intent — freshness checkpoint: CHANGELOG, TODO, PROGRESS updates for plan completion · constraints — none · unknowns — none · scope — operational artifacts only

## Cycle 37 — 2026-04-10

**Phase**: build
**What**: 16 profile widget tests — structural iteration over all 9 disabled fields (nav skip, input rejection, "Coming soon" rendering), edge cases (all-disabled Overlay group, last-enabled CPU field clamp), value cycling, save/esc, grid-enter first-non-disabled.
**Commit**: b7e3c3c test(tui): add profile widget and disabled field contract tests
**Inspiration**: none
**Discovered**: The structural test pattern (iterate groups/fields, test only `disabled: true`) is resilient to field additions — if a new disabled field is added, it gets tested automatically. The Overlay group being entirely disabled creates a clean edge case: enter editing lands on field 0, and all navigation stays put.
**Verified**: N/A: test-only
**Next**: Task 7 (freshness checkpoint) is the final task — all feature tasks are complete.
**Context**: intent — test disabled field contracts per Task 6 acceptance · constraints — structural iteration, proportionality + 3 edge cases · unknowns — none · scope — profile_widget_test.go (new, 319 LOC)

## Cycle 36 — 2026-04-10

**Phase**: build
**What**: 24 content state machine tests — no-game guards, tab switching, launch (with duplication prevention), DLL update/restore confirmation flow (Y/cancel/skip-confirmation), install wizard 3-step state machine (start/type-nav/type→version/version→download/esc/q cancel), and message handlers.
**Commit**: 3ea0a4e test(tui): add content and DLL operation state machine tests
**Inspiration**: none
**Discovered**: ContentModel.Update returns `(ContentModel, tea.Cmd)` not `(tea.Model, tea.Cmd)` — direct call pattern works cleanly without type assertions. The `dll.DLL` type needed importing for install wizard version tests. Confirmation flow has a nice property: any key except Y/y cancels, which makes boundary testing trivial.
**Verified**: N/A: test-only
**Next**: Task 6 (profile widget/disabled fields) is the last feature task before the freshness checkpoint.
**Context**: intent — test content DLL operations per Task 5 acceptance criteria · constraints — proportionality, Cmd-return level · unknowns — none · scope — content_test.go (new, 403 LOC)

## Cycle 35 — 2026-04-10

**Phase**: build
**What**: 19 sidebar state machine tests — navigation (up/down/j/k with boundary clamping), DLL filter, profile filter with injected fake, sort cycling, clear filters, multi-select (space/a/A/esc), enter confirm/batch, search activation/blur.
**Commit**: ce5a138 test(tui): add sidebar state machine tests
**Inspiration**: none
**Discovered**: SidebarModel is not a tea.Model (no Init method) — can't use sendKey helper. Tests call m.Update(keyMsg(...)) directly. This pattern also applies to ContentModel and ProfileWidgetModel for Tasks 5-6. Should add component-level send helpers to testhelpers if the pattern repeats.
**Verified**: N/A: test-only
**Next**: Tasks 5 and 6 remain. Task 5 (content/DLL) is the larger one.
**Context**: intent — test sidebar key bindings per Task 4 acceptance criteria · constraints — proportionality 1+1, injected profile fakes · unknowns — none · scope — sidebar_test.go (new, 324 LOC)

## Cycle 34 — 2026-04-10

**Phase**: build
**What**: 30 layout state machine tests covering all global key bindings, help overlay, density modes, jump keys (with/without game, focused-mode boundary), nav stack (single entry, deep pop, root protection), modal interception, batch menu (esc/nav/enter/block), and window sizing.
**Commit**: 2b4c351 test(tui): add layout, navigation, and modal state machine tests
**Inspiration**: none
**Discovered**: Implemented directly instead of worktree dispatch — cleaner when test code needs access to internal types and current API. The `rescanCompleteMsg` type doesn't exist; actual type is `rescanGamesMsg`. Batch menu only has one action so cursor boundary testing is minimal.
**Verified**: N/A: test-only
**Next**: Tasks 4, 5, 6 all unblocked. Can parallelize in next cycles.
**Context**: intent — test all Layout model key bindings and navigation per Task 3 acceptance criteria · constraints — proportionality 1+1 per binding · unknowns — none · scope — layout_test.go (new, 455 LOC)

## Cycle 33 — 2026-04-10

**Phase**: build
**What**: Test helpers and model factories — testServices (fake deps), testGame/testDLL/testDatabase (data), testLayout/testLayoutWithGame/testContent/testSidebar (models), keyMsg/sendKey/sendKeys (key simulation), execCmd (Cmd asserter). 10 smoke subtests validate all helpers including keyMsg round-trip for 12 key patterns.
**Commit**: 996519c test(tui): add test helpers and model factories for state machine testing
**Inspiration**: none — standard Go test helper patterns
**Discovered**: Bubbletea v2's `KeyPressMsg.String()` returns `"space"` not `" "` for space key — the worktree agent's test expected `" "`. Worktree branched from pre-Services revision, required full adaptation of model factories to use Services API. `sendKeys` triggered unused-function lint — resolved by adding a smoke subtest.
**Verified**: N/A: test-only
**Next**: Tasks 3-6 are all unblocked. Task 3 (layout/nav/modal tests) is the broadest; Tasks 4-6 can run in parallel.
**Context**: intent — build test infrastructure factories and key helpers for subsequent state machine tests · constraints — no filesystem, bubbletea v2 key types · unknowns — none · scope — testhelpers_test.go (new, 377 LOC)

## Cycle 32 — 2026-04-10

**Phase**: build
**What**: Introduced Services struct for TUI dependency injection — function fields for config loading, profile loading/existence, and backup existence threaded through Layout, Content, and Sidebar constructors. Production uses DefaultServices(); tests can substitute fakes.
**Commit**: ab97dba refactor(tui): introduce Services struct for dependency injection
**Inspiration**: none — standard Go function-field injection pattern
**Discovered**: Synchronous I/O was spread across 12 call sites in 3 files. `loadEffectiveProfile` converted from package-level function to ContentModel method. Async tea.Cmd closures (DLL download, GPU metrics, game launch) intentionally left as direct calls — they're tested at Cmd-return level in later tasks.
**Verified**: N/A: refactor-no-behavior-change
**Next**: Task 2 — test helpers and model factories (depends on this task)
**Context**: intent — enable pure state-machine testing of TUI models by replacing synchronous I/O with injectable fakes · constraints — no behavior change, async Cmd closures untouched · unknowns — none · scope — services.go (new), layout.go, content.go, sidebar.go

## Cycle 31 — 2026-04-09 (plan summary: Ludusavi Removal and Dead Code Sweep)

**What**: Completed 3-task plan — removed ludusavi save game integration (package, profile, launcher, TUI, GUI, Svelte, docs, packaging) and swept 49 dead functions across 16 files plus unused mangohud config, self-updater package, and samber/lo dependency. Net -1,000 lines, -4 files. Dead code 16.6% → 14.3%.
**Commits**: 39a8cc6 refactor: remove ludusavi save game integration · 0687920 refactor: remove dead code and unused dependencies
**Discovered**: Worktree agents unreliable for this codebase (Cycle 29 agent deleted 3,584 lines of unrelated code). logging.go collapsed to single Warn function after ludusavi removal killed last Info caller. Architecture grade improved C → B. HEALTH.md Audit 1 dead code finding (16.6%) partially resolved — now 14.3%.
**Verified**: N/A: chore-build-config
**Next**: Vision-driven work. GUI DLL progress indicator (degraded), overlay CLI commands (annoying), or Vulkan layer PoC (vision direction).

## Cycle 30 — 2026-04-09

**What**: Dead code sweep — removed 49 unreachable functions across 16 files, deleted mangohud.go and entire update package, removed samber/lo unused dep. Dead code 16.4% → 14.3% (sentrux), 49 → 0 (deadcode -test).
**Commit**: 0687920 refactor: remove dead code and unused dependencies
**Inspiration**: none
**Discovered**: nvidia_test.go and sparkline_test.go had tests for dead functions — removed too. GPUGeneration type/constants kept but methods and parser removed. logging.go collapsed to just Warn. Architecture grade improved C → B (sentrux).
**Verified**: N/A: refactor-no-behavior-change
**Next**: Task 3 — plan-level freshness checkpoint
**Context**: intent — reduce dead code from 16.6% baseline per HEALTH.md Audit 1 · constraints — preserve test-only exports · unknowns — none · scope — 16 files across 13 packages + go.mod

## Cycle 29 — 2026-04-09

**What**: Removed ludusavi save game integration — package, profile struct field, launcher pre-launch hook, TUI backup settings widget, GUI backup checkbox, Svelte component section, docs, and AUR packaging references
**Commit**: 39a8cc6 refactor: remove ludusavi save game integration
**Inspiration**: none
**Discovered**: Svelte frontend and docs/gamemode.sh had ludusavi references that the initial integration map missed. Worktree agent went rogue — deleted sparklines, thermal gradients, navstack, help system, and operational artifacts (3,584 lines of unrelated deletion). Caught in diff review; implemented directly instead.
**Verified**: N/A: refactor-no-behavior-change
**Next**: Task 2 — dead code and unused dependency sweep
**Context**: intent — remove ludusavi to clear path for rethought save game approach · constraints — old profiles must still load · unknowns — none · scope — 13 files across 7 packages + docs + packaging

## Archived Cycles

Cycle 28 (2026-04-01): feat — TUI confirmation dialogs for destructive DLL ops
Cycle 27 (2026-04-01): feat — color flash animation on message bar
Cycle 26 (2026-04-01): feat — compositor modals, density modes, jump-key titles
Cycle 25 (2026-04-01): feat — tab-based content views (DLLs/Profile/Launch)
Cycle 24 (2026-04-01): feat — header sparklines/gauges, context keybinding bar
Cycle 23 (2026-04-01): refactor — nav stack replacing binary focus
Cycle 22 (2026-04-01): feat — sparkline/gauge renderers with thermal coloring
Cycle 21 (2026-04-01): feat — thermal gradient system, expanded theme tokens
Cycle 20 (2026-04-01): feat — enhanced cpu info CLI command
Cycle 19 (2026-04-01): feat — enhanced gpu info with NVML metrics
Cycle 18 (2026-04-01): feat — GPU power limit in profile system
Cycle 17 (2026-04-01): chore — transitive dependency update
Cycle 16 (2026-04-01): feat — proton CLI commands, GPU profile flags
Cycle 15 (2026-04-01): refactor — profile widget field registry
Cycle 14 (2026-04-01): test — launcher tests (cleanup, signals, overlay)
Cycle 13 (2026-04-01): test — CPU package tests with mock sysfs
Cycle 12 (2026-04-01): test — config persistence tests
Cycle 11 (2026-04-01): refactor — TUI threaded Styles replacing globals
Cycle 10 (2026-04-01): fix — centralized slog logging
Cycle 9 (2026-04-01): refactor — consolidated launch orchestration
Cycle 8 (2026-04-01): fix — GUI/TUI launch cleanup closures
Cycle 7 (2026-03-17): feat — overlay collector wired into launcher
Cycle 6 (2026-03-17): feat — stats collector goroutine
Cycle 5 (2026-03-17): feat — overlay mmap IPC with seqlock
Cycle 4 (2026-03-17): feat — NVML throttle reason detection
Cycle 3 (2026-03-17): feat — GPU alerts in TUI header
Cycle 2 (2026-03-17): feat — GPU alert detection system
Cycle 1 (2026-03-17): feat — go-nvml backend for GPU metrics
