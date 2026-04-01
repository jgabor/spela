# Progress

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
