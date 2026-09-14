---
# spela-cife
title: FSR release automation
status: todo
type: epic
created_at: 2026-01-16T20:47:34Z
updated_at: 2026-01-16T20:47:34Z
parent: spela-csr0
---

Automate detection and publishing of new AMD FSR versions.

## Scope

- Create fsr-updater tool to check GPUOpen-LibrariesAndSDKs/FidelityFX-SDK releases
- Create workflow to download, validate, and hash new DLLs
- Auto-create GitHub Releases with DLL assets
- Update manifest.json and create PR for review

## Source

AMD publishes FSR SDK at: <https://github.com/GPUOpen-LibrariesAndSDKs/FidelityFX-SDK/releases>
FSR 4 uses multiple DLLs: amd_fidelityfx_framegeneration_dx12.dll, amd_fidelityfx_loader_dx12.dll, amd_fidelityfx_upscaler_dx12.dll

## Deliverables

- tools/fsr-updater/ - Go tool to check for updates  
- .github/workflows/fsr-manifest.yml - Daily check workflow
