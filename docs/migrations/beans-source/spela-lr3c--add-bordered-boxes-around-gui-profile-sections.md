---
# spela-lr3c
title: Add bordered boxes around GUI profile sections
status: todo
type: task
created_at: 2026-01-18T19:40:54Z
updated_at: 2026-01-18T19:40:54Z
parent: spela-achk
---

Wrap each profile settings group (DLSS, GPU, Proton) in bordered box with rounded corners. Use --border-default (velvet orchid) for unfocused, --border-focus (amethyst) for focused.

## Files
- GameDetail.svelte

## Reference
`internal/tui/profile_widget.go` lines 500-529