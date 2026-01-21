---
# spela-am43
title: Allow TUI DLL install without detected DLLs
status: completed
type: bug
priority: normal
created_at: 2026-01-21T19:15:22Z
updated_at: 2026-01-21T19:48:38Z
parent: spela-6cx4
---

Match GUI install behavior by allowing DLL install even when a game has no detected DLLs.\n\n## Checklist\n- [x] list installable DLL types from manifest regardless of detected DLLs\n- [x] use install semantics instead of swap-only when no DLLs are detected\n- [x] refresh game DLL inventory after install
