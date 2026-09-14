---
# spela-fw5t
title: 'Phase 1: Foundation — IPC and basic HUD'
status: todo
type: task
created_at: 2026-03-02T11:46:30Z
updated_at: 2026-03-02T11:46:30Z
parent: spela-s84z
blocked_by:
    - spela-ifu0
---

Implement io_uring-inspired mmap ring buffer IPC (Go + Zig). Go metrics collector writes to mmap. Layer reads and renders always-on HUD with FPS, GPU temp, power, VRAM.
