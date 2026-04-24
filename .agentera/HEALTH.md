# Health

## Audit 7 · 2026-04-24

The actionable Audit 6 warnings are closed. Test and dependency health are materially better; only the release-publication gate remains because tags must not be pushed without explicit approval.

**Dimensions assessed**: Tests, Dependencies, Versions, Freshness
**Findings**: 0 critical, 1 warning, 0 info (0 filtered by confidence)
**Overall trajectory**: ⮉ improving vs Audit 6
**Grades**: Tests [A-] | Dependencies [A-] | Versions [B] | Freshness [A]

### Tests: A-

No findings. `mage test` includes tagged GUI backend tests, and Playwright e2e passes against the current resource-centric GUI.

### Dependencies: A-

No findings. The frontend toolchain is exact-pinned on Vite 8, Svelte 5, and plugin 7; `npm audit --json` reports 0 vulnerabilities.

### Versions: B

#### ⇉ Local release tags are not published remotely, warning (confidence: 95)

- **Location**: local tags `v0.5.0`, `v0.5.1`; origin tag refs
- **Evidence**: Remote publication was explicitly excluded from this remediation request.
- **Impact**: Release consumers and compare links cannot resolve v0.5.x tags on origin.
- **Suggested action**: Push `main`, `v0.5.0`, and `v0.5.1` when the user approves publication.

### Freshness: A

No findings. CHANGELOG, TODO, PROGRESS, and HEALTH now record the warning remediation and the remaining tag-publication gate.

### Trends vs Audit 6

- **Improved**: Tests [B-→A-] and Dependencies [B-→A-].
- **Stable**: Versions [B] remains limited by user-gated remote publication.
- **Resolved**: GUI backend tests outside `mage test`, stale Playwright e2e harness, and frontend audit advisories.

### Patterns Observed

- **Testing approach**: Default verification now covers tagged GUI backend tests and browser e2e tests are usable again.
- **Dependency patterns**: Frontend and Go dependencies both use exact pins.
- **Release model**: Local release state remains healthy; remote publication stays explicitly user-gated.

## Audit 6 · 2026-04-24

Audit 5 remediation materially improved launch, GUI, TUI, and artifact health. The codebase can close the plan cleanly, with only approval-gated release/dependency work and test harness debt left.

**Dimensions assessed**: Architecture, Patterns, Coupling, Complexity, Tests, Dependencies, Versions, Security, Freshness
**Findings**: 0 critical, 4 warnings, 0 info (0 filtered by confidence)
**Overall trajectory**: ⮉ improving vs Audit 5
**Grades**: Architecture [A-] | Patterns [A-] | Coupling [B] | Complexity [B] | Tests [B-] | Dependencies [B-] | Versions [B] | Security [A] | Freshness [A]

### Architecture: A-

No findings. Wrapper launch now calls `Launcher.Prepare()`, and direct Steam URI launch returns `%command%` guidance instead of pretending cleanup can track Steam.

### Patterns: A-

No findings. Profile load errors, boolean parsing, governor validation, and GUI logging now follow shared boundary helpers.

### Coupling: B

No findings. GUI actions now route through `guiApplicationBoundary`, and TUI DLL updates route through `Services.UpdateCachedDLL`.

### Complexity: B

No findings. TUI routing split into focused helpers, and current complexity is covered by routing and resource-scope tests.

### Tests: B-

#### ⇉ GUI backend tests are outside default `mage test`, warning (confidence: 92)

- **Location**: `magefile.go:101-104`, `internal/gui/application_test.go:1`
- **Evidence**: `mage test` runs `go test -v ./...`; GUI backend tests require `dev`, `production`, or `bindings` tags.
- **Impact**: GUI boundary regressions can pass the default project test target.
- **Suggested action**: Include a tagged GUI backend test in the standard verification path.

#### ⇉ Playwright e2e harness is stale against the GUI, warning (confidence: 95)

- **Location**: `internal/gui/frontend/e2e/fixtures.js:82-100`, `internal/gui/frontend/e2e/smoke.spec.js:4-13`
- **Evidence**: `npm run test:e2e` failed 27 of 28 tests; fixtures mock `window.go.main.App` and tests expect old Games/Monitor navigation.
- **Impact**: Frontend end-to-end coverage is not a usable regression guard.
- **Suggested action**: Rebase the harness on `window.go.gui.App` and the current GUI navigation surface.

### Dependencies: B-

#### ⇉ Frontend audit advisories remain approval-blocked, warning (confidence: 95)

- **Location**: `internal/gui/frontend/package.json:15-23`, `internal/gui/frontend/package-lock.json`
- **Evidence**: `npm audit --json` reports 7 moderate advisories; fixes require semver-major Vite, Svelte, and plugin upgrades.
- **Impact**: Dev-server and Svelte SSR risk remains until the major upgrade is approved and tested.
- **Suggested action**: Plan the Vite 8 and Svelte 5 upgrade when approval exists.

### Versions: B

#### ⇉ Local release tags are not published remotely, warning (confidence: 95)

- **Location**: local tags `v0.5.0`, `v0.5.1`; origin tag refs
- **Evidence**: `git tag --list` shows both tags; `git ls-remote --tags origin 'v0.5.0*' 'v0.5.1*'` returns no refs.
- **Impact**: Release consumers and compare links cannot resolve the v0.5.x tags on origin.
- **Suggested action**: Push `main`, `v0.5.0`, and `v0.5.1` when the user approves.

### Security: A

No findings. Lightweight scans found no hardcoded secrets, SQL construction, dynamic JS execution, or suspicious shell construction in source.

> This is a lightweight surface scan. For comprehensive security analysis, use dedicated tools: semgrep, Snyk, Bandit (Python), npm audit (Node), govulncheck (Go), or similar static analysis and vulnerability scanning tools appropriate to your stack.

### Freshness: A

No findings. PROGRESS, TODO, CHANGELOG, DOCS, DESIGN, and PLAN all have plan-current commit dates; HEALTH is updated by this audit.

### Trends vs Audit 5

- **Improved**: Architecture [C→A-], Patterns [B→A-], Coupling [C+→B], Complexity [C+→B], Freshness [C+→A].
- **Stable**: Dependencies [B-] and Versions [B] remain limited by explicit user approval gates.
- **Degraded**: Tests [B→B-] because the e2e harness is confirmed stale and GUI backend tests are outside `mage test`.
- **Resolved**: Audit 5 critical launch lifecycle split and all implementation-owned warnings.
- **New findings**: Default test target misses tagged GUI backend tests.

### Patterns Observed

- **Module structure**: Internal package boundaries remain coherent; UI code now uses narrower application and service seams.
- **Error handling**: `%w` wrapping and explicit boundary validation dominate user-facing flows.
- **Testing approach**: Behavior-first Go tests cover plan-critical paths; frontend e2e remains stale.
- **Dependency patterns**: Go dependencies are exact and clean; frontend packages are exact-pinned but blocked on major upgrades.
- **Launch model**: Steam wrapper is the supported launch path; direct launches are rejected with setup guidance.

## Audit 5 · 2026-04-24

The v0.5.0 release cleared the old version-health critical, but the heavy TUI/profile work exposed a more important launch-path split. The codebase is not collapsing, but new work should fix orchestration drift before GUI parity expands the same seams.

**Dimensions assessed**: Architecture, Patterns, Coupling, Complexity, Tests, Dependencies, Versions, Security, Freshness
**Findings**: 1 critical, 10 warnings, 3 info (0 filtered by confidence)
**Overall trajectory**: ⮋ degrading vs Audit 4 (version recovered, architecture/complexity/deps/freshness slipped)
**Grades**: Architecture [C] | Patterns [B] | Coupling [C+] | Complexity [C+] | Tests [B] | Dependencies [B-] | Versions [B] | Security [A-] | Freshness [C+]

### Architecture: C

#### ⇶ Wrapper mode bypasses launcher preparation, critical (confidence: 92)

- **Location**: `cmd/spela/main.go:85-113`, `internal/launcher/launcher.go:59-74`, `internal/launcher/launcher.go:151-166`
- **Evidence**: Wrapper mode manually applies profiles then calls `l.Launch`; `Prepare()` owns VKD3D preflight and overlay IPC setup.
- **Impact**: Steam `%command%`, the primary path, can skip overlay startup and compatibility warnings.
- **Suggested action**: Route wrapper mode through one preparation pipeline while preserving wrapper-provided environment.

#### ⇉ Direct Steam URI launches have cleanup-lifetime mismatch, warning (confidence: 78)

- **Location**: `cmd/spela/commands/launch.go:62-76`, `internal/gui/app.go:795-814`
- **Evidence**: CLI and GUI launch `steam steam://rungameid/<id>` through `Launcher.Launch`, whose cleanup runs when that command exits.
- **Impact**: Profile effects may be restored before the actual game process exits.
- **Suggested action**: Prefer wrapper launch guidance, or track a real game process lifetime.

### Patterns: B

#### ⇉ Default profile load errors are discarded, warning (confidence: 88)

- **Location**: `internal/profile/storage.go:53`, `internal/profile/storage.go:87`, `cmd/spela/commands/proton.go:238`
- **Evidence**: `defaults, _ := LoadDefault()` treats missing and corrupt defaults the same.
- **Impact**: Corrupt defaults can silently resolve inheritance against zero values.
- **Suggested action**: Ignore only `fs.ErrNotExist`; surface parse and read errors.

#### ⇉ Boolean parsing differs across profile commands, warning (confidence: 92)

- **Location**: `cmd/spela/commands/proton.go:264-275`, `cmd/spela/commands/dlss.go:259-274`
- **Evidence**: Proton rejects invalid bools; DLSS stores any non-true value as false.
- **Impact**: A typo can persist an unintended false override.
- **Suggested action**: Move `parseBoolFlag` to a shared command helper and use it everywhere.

#### ⇉ GUI logs bypass centralized logging, warning (confidence: 85)

- **Location**: `internal/gui/app.go:10`, `internal/gui/app.go:779`
- **Evidence**: GUI imports `log/slog` directly instead of `internal/logging`.
- **Impact**: GUI logs bypass the repository's logging seam and test redirection.
- **Suggested action**: Add needed helpers to `internal/logging` and route GUI logs through them.

### Coupling: C+

#### ⇉ GUI remains an over-intimate domain facade, warning (confidence: 90)

- **Location**: `internal/gui/app.go:14-22`, `internal/gui/app.go:384-437`, `internal/gui/app.go:642-817`
- **Evidence**: Wails methods import and orchestrate config, CPU, DLL, game, GPU, launcher, profile, proton, and Steam directly.
- **Impact**: GUI parity work will ripple across domain packages through bindings.
- **Suggested action**: Introduce a narrow application-service layer before the GUI redesign.

#### ⇉ DLLs TUI resource bypasses Services seam, warning (confidence: 85)

- **Location**: `internal/tui/services.go:12-35`, `internal/tui/dlls_resource.go:262-326`
- **Evidence**: The resource pane directly calls DLL cache and swap functions, then mutates `game.Game` DLL state.
- **Impact**: Batch DLL behavior now lives inside a view model.
- **Suggested action**: Move DLL inventory/update operations behind `Services` or a shared service.

### Complexity: C+

#### ⇉ TUI routing complexity is concentrating again, warning (confidence: 92)

- **Location**: `internal/tui/content.go:210-389`, `internal/tui/layout_handlers.go:62-175`, `internal/tui/layout_handlers.go:179-284`
- **Evidence**: Key routing, modal routing, pending actions, profile actions, DLL actions, and message handling share large switches.
- **Impact**: The old `Layout.Update` problem is returning in adjacent handler functions.
- **Suggested action**: Split mode handlers by modal, profile, DLL, rail/focus, and app-message concerns.

#### ⇢ Field formatting duplicates profile knowledge, info (confidence: 80)

- **Location**: `internal/profile/inheritance.go:9-58`, `internal/tui/detail.go:430-515`
- **Evidence**: Profile owns field keys; TUI separately owns labels and a 35-case value formatter.
- **Impact**: Adding profile fields requires coordinated UI edits.
- **Suggested action**: Consolidate field metadata or add stronger coverage for label/formatter completeness.

### Tests: B

Recent profile, proton, launcher, and TUI tests materially improved the highest-risk areas. The suite is green, and recent tests mostly exercise behavior rather than private implementation.

#### ⇉ GUI backend and env remain test gaps, warning (confidence: 90)

- **Location**: `internal/gui/`, `internal/env/env.go`
- **Evidence**: GUI backend has no Go tests; frontend has shallow component coverage; env has no direct tests.
- **Impact**: GUI parity and launch environment behavior can drift silently.
- **Suggested action**: Add mapper/service tests for GUI and direct tests for env map isolation and command application.

### Dependencies: B-

#### ⇉ Frontend audit reports moderate vulnerabilities, warning (confidence: 90)

- **Location**: `internal/gui/frontend/package-lock.json`, `internal/gui/frontend/package.json:15-23`
- **Evidence**: npm audit reports moderate advisories through Vite/esbuild/Svelte; package.json uses caret ranges.
- **Impact**: Mostly dev-server/SSR risk, but dependency health is no longer clean.
- **Suggested action**: Plan a tested frontend stack upgrade and decide whether npm pins should match Go exact pinning.

### Versions: B

#### ⇉ v0.5.0 tag is not published remotely, warning (confidence: 95)

- **Location**: `CHANGELOG.md:8-28`, git remote tags
- **Evidence**: Changelog and local tag are current; `git ls-remote --tags origin "v0.5.0*"` returns no tag.
- **Impact**: Release consumers and compare links cannot resolve the remote v0.5.0 tag.
- **Suggested action**: Push `v0.5.0` when the user approves.

### Security: A-

No hardcoded secrets, dynamic JS execution, shell string execution, or SQL construction patterns were found in the lightweight scan.

#### ⇢ Root CPU governor path lacks value validation, info (confidence: 68)

- **Location**: `cmd/spela/commands/apply_profile.go:79-80`, `cmd/spela/commands/cpu_set.go:117-123`, `internal/cpu/cpu.go:60-80`
- **Evidence**: Profile strings are stored and later written to sysfs as governor values.
- **Impact**: No injection path, but invalid values cross a privileged boundary before sysfs rejects them.
- **Suggested action**: Validate against available governors before save and before privileged apply.

> This is a lightweight surface scan. For comprehensive security analysis, use dedicated tools: semgrep, Snyk, Bandit (Python), npm audit (Node), govulncheck (Go), or similar static analysis and vulnerability scanning tools appropriate to your stack.

### Freshness: C+

#### ⇢ DESIGN and DOCS are stale against v0.5.0, info (confidence: 60)

- **Location**: `.agentera/DESIGN.md:131-140`, `.agentera/DESIGN.md:263-314`, `.agentera/DOCS.md:30-40`
- **Evidence**: DESIGN still describes theme variants, tab bars, and Launch/Profile tabs; DOCS index dates predate v0.5.0 work.
- **Impact**: Future UI work can start from stale visual and documentation contracts.
- **Suggested action**: Run `/visualisera` for DESIGN and `/dokumentera` for DOCS after this audit.

### Trends vs Audit 4

- **Improved**: Tests [B-→B] from profile/proton/launcher/TUI coverage; Versions [D→B] because v0.5.0 cleared local release staleness.
- **Degraded**: Architecture [B+→C], Patterns [B+→B], Complexity [B→C+], Dependencies [A→B-], Freshness [A→C+].
- **Stable**: Coupling [C+] remains elevated; Security [A→A-] has only an info-level validation gap.
- **Resolved**: Audit 4's v0.2.1 stale-release critical is obsolete locally.
- **New findings**: Wrapper preparation bypass, frontend audit advisories, stale DESIGN/DOCS after v0.5.0.

### Patterns Observed

- **Module structure**: Internal package boundaries remain coherent; UI packages are composition-heavy.
- **Error handling**: `%w` wrapping dominates, but default-profile load errors are inconsistently surfaced.
- **Testing approach**: Behavior-first Go tests now cover the recent profile/proton/TUI work well.
- **Dependency patterns**: Go dependencies are exact and tidy; frontend uses lockfile reproducibility with caret package ranges.
- **Launch model**: Steam wrapper is the strategic path, but orchestration is split from `Launcher.Prepare()`.

## Audit 4 — 2026-04-19 (full: post-TUI-refactor + GUI DLL progress)

TUI complexity refactor landed cleanly — Layout.Update() shrank from 366 to 74 lines, profile_widget.go and content.go both split successfully with all 99 tests intact. The GUI DLL progress work (Cycle 49) is a textbook use of the existing slog + defer + event-emit patterns. The one thing that's sliding is version health: v0.2.1 is now 37 days old with 51 unreleased feat/fix commits, which puts the project deep into "should have shipped" territory per the versioning convention.

**Dimensions assessed**: Architecture, Patterns, Coupling, Complexity, Tests, Dependencies, Versions, Security, Freshness
**Findings**: 1 critical, 1 warning, 3 info (0 filtered by confidence)
**Overall trajectory**: ⮉ improving vs Audit 3 (Complexity warnings resolved, GUI transparency improved)
**Grades**: Architecture [B+] | Patterns [B+] | Coupling [C+] | Complexity [B] | Tests [B-] | Dependencies [A] | Versions [D] | Security [A] | Freshness [A]

### Architecture: B+

No findings. Privilege model, Services DI, launcher cleanup pipeline all intact. GUI LaunchGame correctly routes through launcher.Prepare() for profile apply + cleanup registration.

### Patterns: B+

No findings. GUI DLL progress additions follow existing patterns: centralized slog warnings for swallowed errors, `defer emitDLLProgress("")` for cleanup, stage-prefixed error wrapping with `%w`. Consistent across InstallDLL/UpdateDLLs/RestoreDLLs.

### Coupling: C+

#### ⇢ TUI/GUI fan-out unchanged — info (confidence: 90)

- **Location**: `internal/tui/` (11 imports of domain packages), `internal/gui/app.go` (7 internal imports)
- **Evidence**: No change since Audit 3. GUI imports config/cpu/dll/game/gpu/launcher/profile
- **Impact**: Harder to unit-test UI in isolation. Mitigated by Services DI in TUI.
- **Suggested action**: Status quo acceptable — downgrading to info given stability

### Complexity: B

Refactor from Cycle 48 landed cleanly. Layout.Update() 366→74 lines, profile_widget.go 994→455 (field defs to profile_fields.go:549), content.go 945→680 (tab renderers to content_views.go:281). All 99 TUI tests pass unchanged.

#### ⇢ Remaining large files — info (confidence: 70)

- **Location**: `internal/gui/app.go` (791 lines), `internal/tui/layout.go` (689 lines, mostly render helpers), `internal/tui/content.go` (680 lines), `internal/tui/options_modal.go` (608)
- **Evidence**: app.go now holds all Wails-exposed methods plus new DLL progress plumbing. layout.go is render-helper dense (renderStandard/Compact/Focused/BatchMenu), not Update-sprawl.
- **Impact**: Readable but monolithic; future GUI features will continue to bloat app.go
- **Suggested action**: Watch for next growth cycle; consider splitting app.go by concern (config/DLL/launch/game) if it passes 900 lines

### Tests: B-

No change in coverage. 99 TUI tests survived the refactor (validates the Services DI + key-sequence helpers). Still 12/17 internal packages tested; env/gui/lock/logging/xdg untested (all low-risk).

#### ⇢ No tests for new extracted files — info (confidence: 55)

- **Location**: `internal/tui/profile_fields.go`, `layout_handlers.go`, `content_views.go`
- **Evidence**: Extracted from tested files; existing tests cover them transitively
- **Impact**: Minimal — mechanical extraction, behavior preserved
- **Suggested action**: None required; existing tests are the regression guard

### Dependencies: A

No change. 9 direct deps, exact pinning, clean state.

### Versions: D

#### ⇶ v0.2.1 release 37 days stale with 51 unreleased commits — critical (confidence: 100)

- **Location**: `CHANGELOG.md` [Unreleased] section; last release tag `a83e75d chore(release): v0.2.0` (v0.2.1 tag on 2026-03-13)
- **Evidence**: 51 feat/fix commits since v0.2.1 release date. Unreleased section lists ~25 Added/Changed/Fixed entries including: fan speed control, overlay CLI, TUI test harness (99 tests), launch --dry-run, all show --json flags, TUI complexity refactor, GUI DLL progress.
- **Impact**: Per DOCS.md versioning convention (semver, bump on release), this substantially exceeds both the 7-day age and 5-commit thresholds. Users on v0.2.1 are missing material functionality. Audit 3 flagged this at 28 days / 9 Added entries; it has since grown.
- **Suggested action**: Cut v0.3.0 via magefile release. Content warrants minor bump (multiple new features, no breaking changes).

### Security: A

No findings. Lightweight scan found no hardcoded secrets, no dangerous exec patterns, no concatenated SQL. GUI additions use parameterized event emission and `%w` wrapping only.

> This is a lightweight surface scan. For comprehensive security analysis, use dedicated tools: semgrep, gosec, govulncheck, or similar static analysis and vulnerability scanning tools appropriate to your stack.

### Freshness: A

All operational artifacts current. HEALTH.md updated today, PROGRESS.md current (Cycle 49 today), TODO.md current (Degraded items closed today). No PLAN.md (archived after TUI complexity plan completed).

### Trends vs Audit 3

- **Improved**: Complexity [C+→B] (all three file-size warnings resolved by Cycle 48 refactor), Patterns [B→B+] (GUI DLL progress follows existing patterns cleanly)
- **Degraded**: Versions [B+→D] (9 more days + 2 more cycles of unreleased content; now well past semver convention thresholds)
- **Stable**: Architecture, Coupling (downgraded from warning to info given stability), Tests, Dependencies, Security, Freshness
- **New findings**: None structural; version health is the only escalation
- **Resolved from Audit 3**: Layout.Update() 366 lines (now 74), profile_widget.go 994 lines (now 455), content.go 945 lines (now 680); GUI DLL progress/error context degraded items closed

### Patterns Observed

- **Module structure**: 17 packages under internal/ (stable since Audit 3); TUI split into 10+ files with clear responsibilities (layout shell, layout_handlers for input, content for state, content_views for rendering, profile_widget for widget logic, profile_fields for declarative field defs)
- **Error handling**: `fmt.Errorf` with `%w` wrapping everywhere; GUI now uses stage-prefixed error wrapping for user-facing failures; previously-swallowed errors promoted to `slog.Warn`
- **Testing approach**: Services DI + key-sequence helpers (testhelpers_test.go at 377 lines) survived a major refactor without changes — strong signal the test harness is well-designed
- **Dependency patterns**: Unchanged — 9 direct, exact pinning
- **Privilege model**: Unchanged — batched pkexec apply-profile
- **Profile composition**: Unchanged — cleanup closures, ApplyEnv for dry-run
- **GUI transparency**: Wails `runtime.EventsEmit` + defer-cleanup pattern for progressive UI feedback; persistent error banners instead of transient toasts (vision: "show everything Spela does")

## Audit 3 — 2026-04-10 (full: post-test-harness + feature burst)

The codebase improved substantially since Audit 1. All critical findings resolved: logging unified, TUI globals threaded, launch cleanup fixed, profile widget switch replaced. 99 TUI state machine tests shipped with Services DI. Fan speed control, overlay CLI, launch dry-run, and --json across all show commands all landed cleanly. The main concern is growing complexity in three TUI files that are refactor candidates but not blocking.

**Dimensions assessed**: Architecture, Patterns, Coupling, Complexity, Tests, Dependencies, Version health, Artifact freshness, Security hygiene
**Findings**: 0 critical, 3 warnings, 6 info (0 filtered by confidence)
**Overall trajectory**: ⮉ improving vs Audit 1 (all criticals resolved, test coverage doubled, patterns consistent)
**Grades**: Architecture [B+] | Patterns [B] | Coupling [C+] | Complexity [C+] | Tests [B-] | Dependencies [A] | Versions [B+] | Security [A] | Freshness [A]

### Architecture: B+

No findings. Module structure matches CLAUDE.md. Privilege model (batched pkexec) correct. Services DI fully threaded through TUI models. GUI launch cleanup fixed.

### Patterns: B

No findings. Error handling unified (slog, fmt.Errorf %w). CLI commands follow consistent set/show pattern. Profile fields use declarative registry. All prior pattern findings (F-key case, jump keys, profile switch) resolved.

### Coupling: C+

#### ⇉ TUI/GUI fan-out still elevated — warning (confidence: 90)

- **Location**: `internal/tui/` (8 imports), `internal/gui/` (7 imports)
- **Evidence**: UI packages import config, cpu, dll, game, gpu, launcher, overlay, profile
- **Impact**: Harder to test in isolation; changes in domain packages ripple to UI
- **Suggested action**: Services DI partially mitigates this for TUI; consider service layer abstraction if coupling grows further

### Complexity: C+

#### ⇉ Layout.Update() grew to 366 lines — warning (confidence: 100)

- **Location**: `internal/tui/layout.go:99-464`
- **Evidence**: Was 292 lines in Audit 1. Added: global jump keys, batch menu, density modes. 35+ switch cases with nested focus routing.
- **Impact**: Each new feature adds 15-30 lines; diminishing readability
- **Suggested action**: Extract focus-mode handlers to sub-methods

#### ⇉ profile_widget.go at 994 lines — warning (confidence: 95)

- **Location**: `internal/tui/profile_widget.go`
- **Evidence**: 472 lines of field definitions + 522 lines of model/view logic
- **Impact**: Large file, though field definitions are declarative and self-contained
- **Suggested action**: Extract field definitions to profile_fields.go

#### ⇉ content.go at 945 lines — warning (confidence: 90)

- **Location**: `internal/tui/content.go`
- **Evidence**: Game content, DLL operations, profile/launch tabs, install wizard — all in one file
- **Impact**: Growing file serving multiple responsibilities
- **Suggested action**: Consider extracting tab renderers

### Tests: B-

Test coverage significantly improved. 12/17 packages tested (up from 8/22). TUI now has 99 state machine tests with Services DI. 5 remaining untested packages (env, gui, lock, logging, xdg) are all low-risk. Test-to-code ratio ~19% (up from 13.5%).

### Dependencies: A

9 direct dependencies, exact pinning, no bloat. Dead code and unused deps cleaned. Transitive deps aging but no security issues at current usage level.

### Versions: B+

v0.2.1 is 28 days old with substantial unreleased content (9 Added entries). Ready for v0.3.0 bump per semver convention.

### Security: A

No hardcoded secrets. Privilege escalation properly validated. No dangerous exec patterns. Polkit policy correctly scoped.

### Freshness: A

All operational artifacts current (≤9 days). No PLAN.md (completed and archived). Active development pace.

### Trends vs Audit 1

- **Improved**: Architecture [B→B+] (all critical fixes landed), Patterns [C→B] (logging unified, profile switch replaced), Tests [C→B-] (99 TUI tests, 12/17 packages), Dependencies [B→A] (dead code swept, unused dep removed)
- **Stable**: Coupling [C→C+] (fan-out still present, mitigated by DI), Complexity [C→C+] (Layout.Update grew but other hotspots improved)
- **New dimensions**: Versions [B+], Security [A], Freshness [A]
- **Resolved from Audits 1-2**: Global mutable TUI styles (critical), competing logging (critical), GUI launch cleanup (critical), profile widget switch (critical), F-key case mismatch (critical), jump keys scope (critical)

### Patterns Observed

- **Module structure**: 20 packages under internal/ with single-concern boundaries; cmd/commands/ orchestrates; GUI/TUI are full UI controllers
- **Error handling**: `fmt.Errorf` with `%w` wrapping; hardware errors logged as warnings via centralized slog; `log.Printf` fully eliminated
- **Testing approach**: Table-driven tests, t.TempDir/t.Setenv isolation; TUI uses Services DI with model factories and keyMsg helpers; 12/17 package coverage
- **Dependency patterns**: 9 direct deps, exact pinning, zero bloat; transitive chain aging but monitored
- **Privilege model**: Batched pkexec via `privilege.ExecSelf("apply-profile")` — single round-trip for all hardware settings; fan speed added to pipeline
- **Profile composition**: Profile.Apply(env) returns cleanup closures; ApplyEnv() for dry-run; field registry with apply closures
- **CLI consistency**: All 6 profile sections have set/show/--json; launch has --dry-run
- **Overlay isolation**: CollectFunc callback keeps overlay free of gpu/cpu imports

## Audit 2 — 2026-04-01 (targeted: user-reported TUI bugs)

**Dimensions assessed**: Pattern consistency (TUI key handling)
**Findings**: 2 critical, 0 warnings, 2 info
**Overall trajectory**: ⮉ improving vs Audit 1 (TUI structural debt addressed by 8-task plan)
**Grades**: Patterns [C+] (up from C — key handling still inconsistent but improving)

### Patterns: C+

#### ⇶ Jump keys 2/3/4 not global — critical (confidence: 100)

- **Location**: `internal/tui/layout.go:188` (key "1" handled globally), `internal/tui/content.go:226-234` (keys "2"/"3"/"4" handled content-locally)
- **Evidence**: Key "1" was a global layout-level handler; keys "2"/"3"/"4" were inside ContentModel.Update(), unreachable when sidebarFocused=true
- **Impact**: Tab switching only worked after pressing Tab or 1 first — violated DESIGN.md spec for global jump keys
- **Suggested action**: Handle 2/3/4 at layout level alongside 1
- **Resolution**: Fixed in `697fc97` — 2/3/4 now global jump keys at layout level

#### ⇶ F-key string matching case mismatch — critical (confidence: 100)

- **Location**: `internal/tui/layout.go:172-187`
- **Evidence**: Code matched `"F5"` and `"F11"` (uppercase). bubbletea v2 (via ultraviolet) reports function keys as lowercase: `KeyF5: "f5"` at `ultraviolet/key.go:515`
- **Impact**: Density mode toggles (compact/focused) completely non-functional
- **Resolution**: Fixed in `697fc97` — changed to `"f5"` and `"f11"`

#### ⇢ Tab bar visibility confusion — info (confidence: 70)

- **Location**: `internal/tui/content.go:441-455`
- **Evidence**: Tab bar only renders when `m.game != nil`. Before selecting a game, content shows "Select a game from the sidebar" with no tabs.
- **Impact**: User expected tabs to always be visible. Arguably correct behavior — tabs are meaningless without a game context.

#### ⇢ Theme application — info (confidence: 40)

- **Location**: `internal/tui/styles.go` (all three themes)
- **Evidence**: All theme fields correctly defined and wired. Could not reproduce a visual defect. May be terminal color profile mismatch or user expectation vs. actual palette.
- **Impact**: Unresolved — needs user clarification on what specifically looks wrong.

### Trends vs Audit 1

- **Improved**: TUI architecture significantly improved by 8-task visual transformation plan (nav stack, tabs, sparklines, compositor modals, density modes)
- **New findings**: Two critical key-handling bugs introduced during the transformation — both fixed same session
- **Resolved from Audit 1**: Global mutable TUI styles (was critical), launch cleanup pipeline (was critical), competing logging patterns (was critical), profile widget 54-case switch (was critical)

## Audit 1 — 2026-04-01

**Dimensions assessed**: Architecture, Patterns, Coupling, Complexity, Tests, Dependencies
**Findings**: 3 critical, 7 warnings, 4 info (2 filtered by confidence)
**Overall trajectory**: baseline (first audit)
**Grades**: Architecture [B] | Patterns [C] | Coupling [C] | Complexity [C] | Tests [C] | Dependencies [B]

### Architecture: B

#### ⇶ GUI launch bypasses cleanup pipeline — critical (confidence: 95)

- **Location**: `internal/gui/app.go:725-748`
- **Evidence**: LaunchGame() calls `p.Apply(l.Environment)` without initializing env or collecting cleanup handlers; compare to correct CLI pattern in `cmd/spela/commands/launch.go:35-96`
- **Impact**: GPU clocks, CPU governor, env vars not restored after game exit via GUI
- **Suggested action**: Mirror CLI launch pattern — init env, collect cleanups, register restore point

### Patterns: C

#### ⇶ Competing logging patterns — critical (confidence: 95)

- **Location**: `internal/profile/apply.go:23,88`, `internal/launcher/launcher.go:84,86`
- **Evidence**: `log.Printf` used directly instead of centralized `logging` package (slog-based)
- **Impact**: Log level control and file routing bypassed in critical paths
- **Suggested action**: Replace `log.Printf` with `logging.Warn`/`logging.Info`

#### ⇉ JSON/YAML serialization split — warning (confidence: 95)

- **Location**: `internal/dll/manifest.go`, `internal/ludusavi/ludusavi.go`
- **Evidence**: `encoding/json` for manifest/ludusavi while config/profile use `yaml.v3`
- **Impact**: Inconsistent persistence format; CLAUDE.md mandates yaml.v3
- **Suggested action**: Clarify convention — JSON acceptable for external APIs

### Coupling: C

#### ⇶ Global mutable state in TUI styles — critical (confidence: 90)

- **Location**: `internal/tui/styles.go:101-116`
- **Evidence**: `var activeTheme` and `var showHints` mutated via `SetTheme()`/`SetShowHints()`; all components implicitly depend on globals
- **Impact**: Untestable, thread-unsafe, hidden coupling across TUI components
- **Suggested action**: Pass theme/hints via bubbletea model tree

#### ⇉ Launch orchestration duplicated in TUI and GUI — warning (confidence: 90)

- **Location**: `internal/tui/content.go:736-739`, `internal/gui/app.go:725-748`
- **Evidence**: Both independently construct launcher, apply profile, set environment
- **Impact**: Divergent launch paths; GUI path already buggy
- **Suggested action**: Consolidate launch orchestration into launcher package

#### ⇉ TUI/GUI god-package fan-out — warning (confidence: 85)

- **Location**: `internal/tui/` (8 internal imports), `internal/gui/` (7 internal imports)
- **Evidence**: UI packages import config, cpu, dll, game, gpu, launcher, overlay, profile
- **Impact**: UI layers tightly coupled to domain; difficult to test in isolation
- **Suggested action**: Push domain logic into services; reduce direct imports

### Complexity: C

#### ⇶ Profile widget 54-case switch — critical (confidence: 95)

- **Location**: `internal/tui/profile_widget.go:594-743`
- **Evidence**: `applyToProfile()` — 150-line switch mapping field keys to profile struct with identical if/else per case
- **Impact**: Adding profile field requires edits in 3 places; O(n) maintenance cost
- **Suggested action**: Extract field definitions to registry; auto-map via metadata

#### ⇉ Layout Update method sprawl — warning (confidence: 88)

- **Location**: `internal/tui/layout.go:88-379`
- **Evidence**: 292-line Update() with 35+ switch cases and nested focus routing
- **Impact**: Hard to extend; each feature adds 15-30 lines
- **Suggested action**: Extract focus-mode handlers to sub-methods

#### ⇉ Profile widget initialization bloat — warning (confidence: 90)

- **Location**: `internal/tui/profile_widget.go:159-457`
- **Evidence**: 299-line `newProfileWidget()` hardcoding 40+ field definitions inline
- **Impact**: Field definitions, display, and mapping are decoupled in code but coupled in maintenance
- **Suggested action**: Define field metadata once; derive widget groups and apply logic

#### ⇉ Dead code accumulation — warning (confidence: 80)

- **Location**: project-wide
- **Evidence**: ~19% dead functions (sentrux metric)
- **Impact**: Navigation burden, misleading API surface
- **Suggested action**: Audit and remove unused functions

### Tests: C

#### ⇶ CPU package untested — critical (confidence: 98)

- **Location**: `internal/cpu/cpu.go`
- **Evidence**: Direct sysfs writes (SetGovernor, SetSMT) with zero test coverage
- **Impact**: Kernel writes unvalidated; governor string errors undetected
- **Suggested action**: Add tests with mock sysfs paths

#### ⇶ Launcher package untested — critical (confidence: 100)

- **Location**: `internal/launcher/`
- **Evidence**: Signal forwarding, cleanup ordering, env inheritance all untested
- **Impact**: Launch pipeline is critical path with no regression guard
- **Suggested action**: Add integration tests for launch/cleanup cycle

#### ⇶ Config persistence untested — critical (confidence: 97)

- **Location**: `internal/config/config.go`
- **Evidence**: YAML roundtrip, missing file, write errors all untested
- **Impact**: Data loss risk from config corruption
- **Suggested action**: Add roundtrip and error-path tests

#### ⇉ 14 of 22 packages lack tests — warning (confidence: 100)

- **Location**: config, cpu, env, gui, launcher, lock, logging, ludusavi, tui, update, xdg, commands
- **Evidence**: 13.5% test-to-code ratio; 106 tests pass but coverage is sparse
- **Impact**: Regressions in untested packages go undetected
- **Suggested action**: Prioritize cpu, launcher, config, commands

### Dependencies: B

#### ⇉ Transitive deps severely outdated — warning (confidence: 100)

- **Location**: `go.mod` indirect section
- **Evidence**: golang.org/x/crypto 16 versions behind, x/net 17 behind, x/text 13 behind
- **Impact**: Accumulated security/bug fixes missing
- **Suggested action**: `go get -u ./...` to refresh transitive deps

### Patterns Observed

- **Module structure**: 22 packages under internal/ with single-concern boundaries; cmd/commands/ orchestrates; gui/tui are full UI controllers
- **Error handling**: `fmt.Errorf` with `%w` wrapping predominant; hardware errors logged as warnings
- **Testing approach**: Table-driven tests, t.TempDir() isolation, descriptive names; 36% package coverage
- **Dependency patterns**: 9 direct deps, exact pinning, zero bloat; transitive chain aging
- **Privilege model**: Batched pkexec via `privilege.ExecSelf("apply-profile")` — single round-trip for all hardware settings
- **Profile composition**: Profile.Apply(env) returns cleanup closures; caller registers with launcher
- **Overlay isolation**: CollectFunc callback pattern keeps overlay free of gpu/cpu imports
