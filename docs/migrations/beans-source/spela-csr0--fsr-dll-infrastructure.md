---
# spela-csr0
title: FSR DLL infrastructure
status: draft
type: milestone
created_at: 2026-01-16T19:40:55Z
updated_at: 2026-01-16T19:40:55Z
---

Set up AMD FSR DLL hosting using GitHub Releases.

## Goals

- Host AMD FSR DLLs as GitHub Release assets
- Extend manifest.json to include FSR versions
- Automate detection of new FSR releases (GPUOpen GitHub)
- Enable FSR update flow in spela

## Prerequisites

- DLSS infrastructure complete (spela-x1hf)
- Manifest schema supports multiple DLL types

## Success criteria

1. `spela dll update` handles FSR DLLs
2. Manifest includes FSR versions
3. CI workflow detects and publishes new FSR versions