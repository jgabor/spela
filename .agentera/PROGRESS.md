# Progress

## Cycle 22 — 2026-04-01 20:00

**What**: Added sparkline and gauge renderers with metrics ring buffer — per-character thermal coloring via lipgloss StyleRanges, sub-character gauge precision, 26 tests
**Commit**: 6dd98fc feat(tui): add sparkline and gauge renderers with metrics buffer
**Inspiration**: INSPIRERA deep dive on ntcharts sparkline algorithm, lipgloss StyleRanges API, left-block Unicode sub-character precision
**Discovered**: Worktree branched before Task 1 merge — created duplicate thermal.go/styles.go changes. Resolved by keeping main's versions. Also duplicate TestNormalizeAndClamp between files — removed from sparkline_test.go.
**Next**: Task 3 (navigation stack) is the architectural centerpiece — unblocked and highest priority. Task 6 (header metrics) is also unblocked now that Tasks 1+2 are done.

## Cycle 21 — 2026-04-01 19:30

**What**: Added thermal gradient system and expanded Theme struct from 14 to 37 fields — surface palette, text hierarchy, 6 thermal stops, metric tokens. Header now renders GPU temp/power/fan with continuous thermal coloring.
**Commit**: 50bdd0c feat(tui): add thermal gradient system and expanded theme tokens
**Inspiration**: INSPIRERA analysis of btop (braille graphs, thermal gradients), k9s (Oklch perceptual color), DESIGN.md thermal gradient specification
**Discovered**: Worktree agent generated Go 1.21-style loop variable copies (`theme := theme`) — Go 1.25 doesn't need these. Fixed post-merge. Otherwise clean implementation.
**Next**: Task 2 (sparkline/gauge renderers) and Task 3 (navigation stack) are both unblocked

## Cycle 1 — 2026-03-17 20:15

**What**: Added go-nvml backend for GPU metrics, replacing nvidia-smi CLI shelling with direct NVML API calls (~50x faster)
**Commit**: 58d79bb feat(gpu): add NVML backend for fast GPU metric reading
**Inspiration**: go-nvml library patterns from NVIDIA/go-nvml examples; overlay design docs (docs/design/overlay.md, overlay-review.md) which require NVML for the planned in-game overlay
**Discovered**: The agent worktree introduced regressions (broke ResetClocks error handling, regressed SplitSeq to Split) — applied changes manually to avoid these. Worktree approach needs careful diff review.
**Next**: The overlay IPC protocol (mmap ring buffers) or the Go-side stats collector that will feed metrics to the overlay layer at high frequency

## Cycle 2 — 2026-03-17 21:00

**What**: Added GPU alert detection system — pure function evaluating metrics for thermal throttling, power limit saturation, and fan maximum conditions
**Commit**: 7a2a6da feat(overlay): add GPU alert detection system
**Inspiration**: NVML throttle reason bitmask API (nvmlClocksThrottleReasonHwThermalSlowdown); research on GPU monitoring best practices suggesting tighter alert thresholds (80°C warning, 85°C critical rather than waiting for driver thermal limit)
**Discovered**: ISSUES.md was heavily stale — issues #1, #2, #3, #7 all already fixed in prior refactoring commits. Marked resolved and added new issue #9 for NVML setter privilege model decision (per-command pkexec vs privileged daemon). docs/ directory is in .gitignore — needed `git add -f`.
**Next**: Integrate alerts into the TUI header (show warning indicators when GPU is throttling), or build the NVML throttle reason detection for more precise alerts

## Cycle 3 — 2026-03-17 21:30

**What**: Integrated GPU alerts into TUI header — temperature/power colored by severity, fan speed shown via NVML, compact alert indicators for throttling/power limit
**Commit**: 8bf8bfe feat(tui): integrate GPU alerts into header with colored metrics
**Inspiration**: None needed (UI integration of existing components)
**Discovered**: Sonnet worktree agent regressed bubbletea/lipgloss v2 imports to v1 and changed async metric fetching to synchronous (would block UI). Applied changes manually again. Agent worktrees consistently introduce regressions in areas outside their direct task scope.
**Next**: NVML throttle reason detection (GetCurrentClocksThrottleReasons) for precise alerts, or overlay IPC protocol (mmap shared memory) as the next overlay foundation piece

## Cycle 4 — 2026-03-17 22:00

**What**: Added NVML throttle reason detection — driver-reported causes (thermal HW/SW, power cap, power brake) now inform alerts precisely instead of relying solely on temperature/power thresholds
**Commit**: 9774163 feat(gpu): add NVML throttle reason detection for precise alerts
**Inspiration**: NVML ClocksThrottleReason bitmask API; confirmed exact constants via go doc (ClocksThrottleReasonHwThermalSlowdown=64, SwPowerCap=4, etc.)
**Discovered**: Implementing directly instead of via worktree agent avoided all regressions from cycles 1-3. For focused single-file changes, direct implementation is faster and more reliable than worktree dispatch.
**Next**: Overlay IPC protocol (mmap shared memory Go→Layer writer) or overlay configuration model (presets, colors, position) for profile integration

## Cycle 5 — 2026-03-17 22:30

**What**: Implemented overlay mmap IPC protocol — shared memory file with SPEL header, 64-byte state section, and seqlock synchronization for lock-free Go→Layer communication
**Commit**: 9c759b2 feat(overlay): add mmap IPC protocol with seqlock synchronization
**Inspiration**: io_uring ring buffer design (from overlay design doc); seqlock pattern from Linux kernel for single-writer/single-reader synchronization
**Discovered**: Binary protocol with `unsafe` and atomics requires careful attention to field alignment and byte ordering. Power draw stored as milliwatts (uint32) on wire to avoid float in shared memory. Temperature as int32 to handle negative values.
**Next**: Stats collector goroutine that periodically writes GPU+CPU metrics to the IPC file, or the C++ Vulkan layer proof of concept that reads from the mmap

## Cycle 6 — 2026-03-17 23:00

**What**: Added stats collector goroutine — periodic `CollectFunc` → `WriteState` loop with immediate-write-on-start, clean shutdown, and domain-isolated callback design
**Commit**: ce7b202 feat(overlay): add stats collector for periodic IPC writes
**Inspiration**: None needed (standard goroutine + ticker pattern)
**Discovered**: Domain isolation via `CollectFunc` callback keeps overlay package free of gpu/cpu imports while still allowing the caller to compose the full metrics→alerts→IPC pipeline. 4 timing-based tests all stable.
**Next**: Wire the collector into the launcher (start collector when launching a game with overlay enabled, stop on exit), or pivot to a different vision direction (parity feature like Smooth Motion profile support)

## Cycle 20 — 2026-04-01 17:50

**What**: Enhanced `spela cpu info` — shows available governors, average frequency, CPU load, RAM usage, SCX status with colored output
**Commit**: 4773a28 feat(cli): enhance cpu info with frequencies, load, RAM, available governors, and SCX status
**Inspiration**: None — mirrors gpu info enhancement pattern
**Discovered**: Linter auto-converted if/else chain to tagged switch on SMT value. GetCPUMetrics reads from /proc/loadavg for utilization.
**Next**: Profile export/import, fan speed control, or GUI DLL progress indicator

## Cycle 19 — 2026-04-01 17:35

**What**: Enhanced `spela gpu info` — shows power draw/limit/range, clocks, fan speed, utilization via NVML
**Commit**: 1837027 feat(gpu): enhance gpu info with power limit range, clocks, fan, and utilization
**Inspiration**: None — NVML API provides all data, just needed wiring
**Discovered**: GetPowerManagementLimitConstraints returns min/max in milliwatts — useful for validating --power-limit values
**Next**: Fan speed control, profile export/import, or HEALTH.md re-audit after 18 cycles of changes

## Cycle 18 — 2026-04-01 17:20

**What**: Added GPU power limit to profile system — apply at launch, restore on exit, CLI flag, TUI field, GPU show display
**Commit**: 7dd494c feat(profile): add per-game GPU power limit to profile system
**Inspiration**: None — followed existing clock offset pattern through all layers
**Discovered**: apply-profile already had --gpu-power-limit flag but profile struct didn't have the field. Reset path also existed but wasn't handling power limit restore.
**Next**: Fan speed control via NVML, or GPU info improvements (show current power limit/range)

## Cycle 17 — 2026-04-01 17:00

**What**: Updated all transitive dependencies (x/crypto +16, x/net +17, wails v2.12, bubbles v2.1). Marked stale TODO entry.
**Commit**: 03e273e chore(deps): update transitive dependencies
**Inspiration**: None — security hygiene
**Discovered**: TODO "DLSS-D column missing" was stale — column already exists in GameDetail.svelte
**Next**: Vulkan overlay layer PoC, or GUI DLL progress indicator, or overlay/ludusavi CLI commands

## Cycle 16 — 2026-04-01 16:45

**What**: Added proton set/show CLI commands (HDR, Wayland, NGX updater) and completed GPU set flags (shader-cache, threaded-opt)
**Commit**: 64e0181 feat(cli): add proton profile commands and complete GPU profile flags
**Inspiration**: None — followed existing dlss/gpu/cpu set command patterns
**Discovered**: GPU set had a subtle bug: --power-mizer didn't support "default" to clear — fixed while adding flags
**Next**: Overlay/ludusavi CLI commands, or start on Vulkan overlay layer PoC

## Cycle 15 — 2026-04-01 16:30

**What**: Replaced profile widget 54-case switch with field registry — 33 apply closures inline with field definitions
**Commit**: b455621 refactor(tui): replace profile widget 54-case switch with field registry
**Inspiration**: None — standard declarative registry pattern
**Discovered**: Plan complete. All 8 foundation hardening tasks done. PLAN.md archived.
**Next**: Vision-driven work — overlay CLI commands, Vulkan layer PoC, or remaining HEALTH.md warnings

## Cycle 14 — 2026-04-01 16:10

**What**: Added 11 launcher tests — cleanup order, signal forwarding, overlay IPC lifecycle, wrapper parsing, game detection
**Commit**: 9e8b4b3 test(launcher): add tests for cleanup order, signal forwarding, overlay IPC, and wrapper parsing
**Inspiration**: None — standard Go process testing with goroutines and signals
**Discovered**: Signal forwarding test works cleanly — signal.Notify intercepts SIGTERM in the test process without killing it
**Next**: Task 8 (profile field registry — last plan task)

## Cycle 13 — 2026-04-01 15:55

**What**: Added 10 CPU package tests with mock sysfs — governor read/write, SMT toggle, metrics, cpuinfo, affinity
**Commit**: 4460ca1 test(cpu): add tests with mock sysfs for governor, SMT, metrics, and affinity
**Inspiration**: None — standard Go sysfs mock pattern with sysRoot override
**Discovered**: Needed to add sysRoot var and sysPath() helper to make hardcoded /sys and /proc paths testable
**Next**: Task 7 (launcher tests) or Task 8 (profile field registry)

## Cycle 12 — 2026-04-01 15:40

**What**: Added config persistence tests — roundtrip, missing file defaults, malformed YAML error, deep clone
**Commit**: 8176d77 test(config): add persistence tests for YAML roundtrip, missing file, and malformed input
**Inspiration**: None — standard Go testing with t.TempDir and t.Setenv for XDG isolation
**Discovered**: Nothing unexpected — config package is clean and well-structured
**Next**: Task 6 (CPU tests) or Task 7 (launcher tests)

## Cycle 11 — 2026-04-01 15:25

**What**: Replaced all TUI global mutable styles with a *Styles struct threaded through 11 model files; CLI helpers made immutable
**Commit**: 8f442f0 refactor(tui): replace global mutable styles with threaded*Styles
**Inspiration**: None — standard bubbletea v2 pattern for shared state
**Discovered**: ContextHelp() needed a showHints parameter added since it previously read the global. Agent worktree worked well for this large mechanical refactoring.
**Next**: Tasks 5-7 (config/CPU/launcher tests) or Task 8 (profile field registry, now unblocked)

## Cycle 10 — 2026-04-01 15:05

**What**: Replaced all log.Printf with structured logging.Warn/Info in profile/apply.go and launcher/launcher.go
**Commit**: 47a4810 fix(logging): replace log.Printf with centralized slog logging
**Inspiration**: None — mechanical replacement
**Discovered**: Zero log.Printf remaining in entire codebase (verified via grep)
**Next**: Task 4 (TUI global state) or Tasks 5-6 (config/CPU tests)

## Cycle 9 — 2026-04-01 14:50

**What**: Consolidated launch orchestration into Launcher.Prepare() — single entry point for all interfaces with overlay IPC lifecycle
**Commit**: f5eda95 refactor(launcher): consolidate launch orchestration into Prepare()
**Inspiration**: None — structural refactoring applying existing patterns
**Discovered**: Overlay now works for GUI/TUI launches too (previously CLI-only). Net -90 lines of duplicated code.
**Next**: Task 3 (logging) or Tasks 4-6 (TUI styles, config/CPU tests)

## Cycle 8 — 2026-04-01 14:35

**What**: Fixed GUI and TUI launch paths to register cleanup closures — RestorePoint + p.Apply() cleanups now mirror CLI pattern
**Commit**: b9456e2 fix(launch): register cleanup closures in GUI and TUI launch paths
**Inspiration**: None — applied existing CLI pattern from commands/launch.go
**Discovered**: TUI had the identical bug to GUI (both dropped p.Apply() return value). HEALTH.md only flagged GUI; adversarial plan review caught TUI.
**Next**: Task 2 (consolidate launch orchestration) or parallel tasks 3-6 (logging, TUI styles, config/CPU tests)

## Cycle 7 — 2026-03-17 23:30

**What**: Wired overlay collector into launcher — when overlay is enabled in a game profile, the launcher creates an IPC file, starts a 500ms metrics collector, exports `SPELA_OVERLAY_IPC` env var, and cleans up on exit
**Commit**: dbf2ca4 feat(overlay): wire collector into launcher for live game metrics
**Inspiration**: None needed (integration of existing components)
**Discovered**: The composition function that bridges gpu→overlay lives naturally in the launch command (orchestration layer), preserving domain isolation. Position string→uint8 mapping added to overlay package as `ParsePosition`.
**Next**: Overlay configuration via profile CLI commands (`spela profile overlay --enabled --position top-right`), or the C++ Vulkan layer proof of concept that reads from the mmap IPC file
