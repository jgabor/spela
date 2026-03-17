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
