---
# spela-3upe
title: Add sidebar + content split layout to GUI
status: completed
type: task
priority: high
created_at: 2026-01-18T19:40:53Z
updated_at: 2026-01-19T15:41:22Z
parent: spela-achk
---

Replace single-column view switching with permanent sidebar (30%) + content (70%) layout. Sidebar shows game list always visible, content area shows selected game details.

## Files

- App.svelte
- GameList.svelte
- GameDetail.svelte

## Reference

`internal/tui/layout.go` (30% sidebar ratio, 25-50 char width)
