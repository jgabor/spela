# Progress

## Cycle 1 — 2026-03-17 20:15

**What**: Added go-nvml backend for GPU metrics, replacing nvidia-smi CLI shelling with direct NVML API calls (~50x faster)
**Commit**: 58d79bb feat(gpu): add NVML backend for fast GPU metric reading
**Inspiration**: go-nvml library patterns from NVIDIA/go-nvml examples; overlay design docs (docs/design/overlay.md, overlay-review.md) which require NVML for the planned in-game overlay
**Discovered**: The agent worktree introduced regressions (broke ResetClocks error handling, regressed SplitSeq to Split) — applied changes manually to avoid these. Worktree approach needs careful diff review.
**Next**: The overlay IPC protocol (mmap ring buffers) or the Go-side stats collector that will feed metrics to the overlay layer at high frequency
