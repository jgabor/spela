# Health

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
