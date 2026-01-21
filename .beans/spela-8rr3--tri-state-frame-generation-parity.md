---
# spela-8rr3
title: Tri-state frame generation parity
status: completed
type: feature
priority: normal
created_at: 2026-01-21T19:11:53Z
updated_at: 2026-01-21T19:17:16Z
parent: spela-6cx4
---

Upgrade GUI profile editing to represent frame generation override tri-state, and fix TUI to display and clear FG override correctly so both match.\n\n## Checklist\n- [x] update GUI ProfileInfo to include fgOverride and honor it on load/save\n- [x] replace GUI frame generation checkbox with tri-state selector matching TUI values\n- [x] update GUI e2e fixtures/tests for fgOverride behavior\n- [x] fix TUI frame generation display and override clearing for (default)
