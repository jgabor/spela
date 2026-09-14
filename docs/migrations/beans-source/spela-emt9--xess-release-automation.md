---
# spela-emt9
title: XeSS release automation
status: todo
type: epic
created_at: 2026-01-16T20:47:27Z
updated_at: 2026-01-16T20:47:27Z
parent: spela-2v82
---

Automate detection and publishing of new Intel XeSS versions.

## Scope

- Create xess-updater tool to check github.com/intel/xess releases
- Create workflow to download, validate, and hash new DLLs
- Auto-create GitHub Releases with DLL assets
- Update manifest.json and create PR for review

## Source

Intel publishes XeSS SDK at: https://github.com/intel/xess/releases
The libxess.dll is included in the SDK zip.

## Deliverables

- tools/xess-updater/ - Go tool to check for updates
- .github/workflows/xess-manifest.yml - Daily check workflow