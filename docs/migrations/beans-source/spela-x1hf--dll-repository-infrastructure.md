---
# spela-x1hf
title: DLSS DLL infrastructure
status: completed
type: milestone
priority: normal
created_at: 2026-01-16T15:55:03Z
updated_at: 2026-01-16T20:31:19Z
blocking:
    - spela-2v82
    - spela-csr0
---

Build the DLL management foundation while implementing DLSS as the first supported vendor. This establishes the infrastructure that FSR and XeSS will later extend.

## Goals

### Foundation (generic infrastructure)

- Design manifest schema supporting multiple DLL types (DLSS, XeSS, FSR)
- Implement generic DLL download and version comparison logic
- Set up GitHub Releases infrastructure for DLL hosting
- Create reusable error handling and progress indication

### DLSS (first implementation)

- Host NVIDIA DLSS DLLs as GitHub Release assets
- Automate detection of new NVIDIA DLSS releases
- Enable end-to-end DLSS update flow in spela

## Approach

Use GitHub Releases on main repo:

- Single repo to manage
- No git history bloat (release assets aren't committed)
- GitHub Releases handles large binaries (2GB per asset limit)
- Tags like `dll-dlss-v3.7.20` distinguish DLL releases from code releases

## Success criteria

1. Manifest schema documented and validated
2. Generic download with progress indication works
3. Version comparison logic tested
4. `spela dll update` successfully downloads and swaps DLSS DLLs
5. CI workflow detects and publishes new DLSS versions
