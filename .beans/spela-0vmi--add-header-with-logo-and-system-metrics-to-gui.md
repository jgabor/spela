---
# spela-0vmi
title: Add header with logo and system metrics to GUI
status: completed
type: task
priority: high
created_at: 2026-01-18T19:40:54Z
updated_at: 2026-01-19T19:43:45Z
parent: spela-achk
---

Create header with ASCII SPELA logo (or SVG) on left, live GPU/VRAM/CPU/RAM metrics on right. Update metrics every 2 seconds using existing GetGPUInfo and GetCPUInfo backend calls.

## Files

- New Header.svelte
- App.svelte

## Reference

`internal/tui/header.go` lines 15-22 (logo), 78-119 (metrics)
