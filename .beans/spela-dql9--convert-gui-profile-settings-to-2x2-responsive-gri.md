---
# spela-dql9
title: Convert GUI profile settings to 2x2 responsive grid
status: completed
type: task
priority: normal
created_at: 2026-01-18T19:40:54Z
updated_at: 2026-01-19T14:41:35Z
parent: spela-achk
---

Arrange profile groups in 2x2 CSS Grid when width allows (>80ch), single column otherwise. Each section is a bordered box within the grid.

## Files

- GameDetail.svelte

## Reference

`internal/tui/profile_widget.go` lines 394-399, 481-498
