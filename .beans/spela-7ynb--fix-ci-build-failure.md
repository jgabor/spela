---
# spela-7ynb
title: Fix CI build failure
status: completed
type: bug
priority: normal
created_at: 2026-01-18T16:51:35Z
updated_at: 2026-01-19T20:03:46Z
---

Build job fails on GitHub Actions because Wails defaults to webkit2gtk-4.0 but ubuntu-latest (24.04) only has webkit2gtk-4.1. Need to add webkit2_41 build tag.
