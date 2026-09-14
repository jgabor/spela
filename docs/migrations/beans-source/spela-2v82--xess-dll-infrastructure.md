---
# spela-2v82
title: XeSS DLL infrastructure
status: draft
type: milestone
created_at: 2026-01-16T19:40:51Z
updated_at: 2026-01-16T19:40:51Z
---

Set up Intel XeSS DLL hosting using GitHub Releases.

## Goals

- Host Intel XeSS DLLs as GitHub Release assets
- Extend manifest.json to include XeSS versions
- Automate detection of new XeSS releases
- Enable XeSS update flow in spela

## Prerequisites

- DLSS infrastructure complete (spela-x1hf)
- Manifest schema supports multiple DLL types

## Success criteria

1. `spela dll update` handles XeSS DLLs
2. Manifest includes XeSS versions
3. CI workflow detects and publishes new XeSS versions