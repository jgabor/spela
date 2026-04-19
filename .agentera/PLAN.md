# Plan: DX12 descriptor_heap per-game toggle

<!-- Level: full | Created: 2026-04-19 | Status: active -->
<!-- Reviewed: 2026-04-19 | Critic issues: 10 found, 10 addressed, 0 dismissed -->

## What

Add a per-game Proton toggle `vkd3d_heap` that emits `PROTON_VKD3D_HEAP=1` and `VKD3D_CONFIG=descriptor_heap` when enabled. Detect the active Proton build for each game and the current NVIDIA driver version. Surface incompatibility warnings at toggle time (TUI/CLI) and at launch time (slog), so users never silently set flags that do nothing.

## Why

`VK_EXT_descriptor_heap` (Vulkan 1.4.340, Jan 2026) fixes a long-standing DX12-on-Linux performance cliff on NVIDIA and resolves Xid 109 hard crashes on Blackwell. Proton-CachyOS 10.0-20260321+ ships the prototype implementation behind both env vars. Users currently discover this via Reddit/forum threads and paste env vars manually — exactly the fragmentation VISION.md targets.

Setting env vars without verifying the environment can honor them violates the transparency principle ("every env var set, every knob visible") and the correctness principle ("if Spela sets a value, it's the right value"). The preflight detection makes the feature honest: enabling it on incompatible setups produces immediate, visible feedback.

## Constraints

- Additive only: existing ProtonSettings YAML layout preserved (`omitempty`)
- No rebuild of vkd3d-proton, DXVK, or Proton — out of scope
- No Proton installer
- Preflight warnings non-blocking: never prevent launch
- Proton build detection is best-effort filesystem scan; ambiguity logs info and proceeds
- Minimum-version constants live in one named file (not scattered)
- User-facing incompatibility feedback fires at toggle time, not only at launch

## Scope

**In**: profile field, env emission, Proton build resolver package, CLI + TUI user surface with toggle-time compatibility notice, launcher preflight warnings, CHANGELOG + release cut.

**Out**: GUI toggle parity, runtime driver upgrade, Proton installation, DXVK builds, rebuilding vkd3d-proton from source, VKD3D_CONFIG compositional helper (YAGNI until a second flag exists).

**Deferred**: community profile intelligence (cross-referencing what descriptor_heap setting works per-game) — depends on game-intelligence infra not yet built.

## Design

ProtonSettings gains a `VKD3DHeap bool`. `applyProton` emits both env vars unconditionally when true. No merging — if the user has `VKD3D_CONFIG` in their shell, spela overwrites it (consistent with how `PROTON_ENABLE_HDR` etc. are handled).

A new `internal/proton` package exposes:

- `ResolveForAppID(appID uint64) (ProtonBuild, error)` — walks Steam's compat-tool resolution to identify which Proton the game will actually launch with
- `SupportsVKD3DHeap(build ProtonBuild) bool` — best-effort filesystem scan of the build directory for a known marker (grep for `PROTON_VKD3D_HEAP` string in the Proton launch script)
- `MinimumDriverVersion() string` and related constants live in `internal/proton/requirements.go` as named constants

Driver version gate lives next to the NVML driver query (or in the proton package, consuming it). Parses the NVIDIA driver string with awareness that it may be two-component (`580.94`), three-component (`580.94.16`), with a beta suffix (`585.0.0`), or empty on non-NVIDIA systems.

**Toggle-time feedback** (user surface): when the user enables `vkd3d_heap` in the TUI profile widget or runs `spela proton show`, if the resolver reports incompatibility with the current Proton build or driver, a short `⚠ notice` line appears inline (not a modal, not a hard error).

**Launch-time feedback** (launcher): `launcher.Prepare()` calls the resolver after the profile is loaded; if `vkd3d_heap=true` and requirements are unmet, `slog.Warn` records the mismatch with the detected versions and minimum versions. Launch continues.

## Tasks

### Task 1: Profile field + env emission

**Depends on**: none
**Status**: ■ complete
**Acceptance**:

- GIVEN a profile with `vkd3d_heap: true` WHEN applied THEN `PROTON_VKD3D_HEAP=1` is set in the environment
- GIVEN `vkd3d_heap: true` WHEN applied THEN `VKD3D_CONFIG=descriptor_heap` is set in the environment
- GIVEN `vkd3d_heap: false` WHEN applied THEN neither env var is set
- GIVEN an existing profile YAML without the `vkd3d_heap` key WHEN loaded and re-saved THEN it round-trips without error and the field defaults to false
- Test proportionality: 1 pass + 1 fail per new apply branch (2 tests total); round-trip covered by extending the existing profile YAML test

### Task 2: Proton build resolver

**Depends on**: none
**Status**: ■ complete
**Acceptance**:

- GIVEN a Steam-managed game AppID AND the user's Steam library is detected WHEN `ResolveForAppID` is called THEN it returns the Proton build directory path and a non-empty name
- GIVEN a Proton build directory containing the `PROTON_VKD3D_HEAP` marker string in its launch script WHEN `SupportsVKD3DHeap` is called THEN it returns true
- GIVEN a Proton build directory without the marker WHEN `SupportsVKD3DHeap` is called THEN it returns false
- GIVEN the AppID cannot be mapped to a Proton build (no config, not installed) WHEN resolver is called THEN it returns a sentinel error that callers distinguish from "resolved as unsupported"
- GIVEN a three-component NVIDIA driver string "580.94.16" WHEN parsed and compared to the minimum THEN it satisfies the requirement
- GIVEN a two-component string "580.94", a beta string "585.0.0", and a whitespace-padded string from nvidia-smi fallback WHEN parsed THEN each is handled without panic and compared correctly
- GIVEN an empty driver string (non-NVIDIA system) WHEN parsed THEN the comparison returns a clearly-typed "unavailable" result
- Version constants (minimum Proton build date, minimum driver version) live in a single file with comments citing the issue/PR they came from
- Test proportionality: 1 pass + 1 fail per resolver path (detection success, detection failure, marker present, marker absent), plus edge-case expansion on driver version parsing (4 shape cases named above)

### Task 3: User-facing surface (CLI + TUI)

**Depends on**: Task 1, Task 2
**Status**: ■ complete
**Acceptance**:

- GIVEN `spela proton set --vkd3d-heap=true <appid>` WHEN run THEN the profile persists `vkd3d_heap: true`
- GIVEN `spela proton show <appid>` WHEN run THEN the `vkd3d_heap` value is displayed in both formatted and `--json` output
- GIVEN `spela proton show <appid>` AND `vkd3d_heap=true` AND the resolver detects an incompatible Proton or driver WHEN run THEN a short notice line appears under the setting naming the specific incompatibility (Proton vs driver)
- GIVEN `spela proton show <appid>` AND `vkd3d_heap=true` AND the environment is compatible WHEN run THEN no notice appears
- GIVEN the TUI profile widget is opened AND the Proton section is visible WHEN navigating THEN a `VKD3D Heap` field is present
- GIVEN the TUI field is focused WHEN the user toggles it THEN the in-memory profile state reflects the change
- GIVEN the TUI field is enabled AND the resolver reports incompatibility WHEN the widget renders THEN a short notice is visible inline with the field
- Test proportionality: 1 pass + 1 fail for the CLI flag handler; 1 new test for the TUI toggle behavior; 1 new test each for CLI-notice and TUI-notice rendering (4 new tests total)

### Task 4: Launch-time preflight warnings

**Depends on**: Task 1, Task 2
**Status**: □ pending
**Acceptance**:

- GIVEN `vkd3d_heap=true` AND the resolved Proton build lacks the marker WHEN the launcher prepares THEN `slog.Warn` is emitted naming the detected build and the minimum supported build
- GIVEN `vkd3d_heap=true` AND the NVIDIA driver version is below the minimum WHEN the launcher prepares THEN `slog.Warn` is emitted naming the detected driver and minimum version
- GIVEN `vkd3d_heap=true` AND both Proton and driver satisfy the requirements WHEN the launcher prepares THEN no warning is emitted
- GIVEN `vkd3d_heap=true` AND the resolver returns an error WHEN the launcher prepares THEN an info-level log records the detection failure and launch proceeds
- GIVEN `vkd3d_heap=false` WHEN the launcher prepares THEN preflight checks are skipped entirely
- Test proportionality: 1 pass + 1 fail per branch (5 branches, 10 tests); covered by extending existing launcher test harness

### Task 5: Cut v0.4.0 release

**Depends on**: Tasks 1, 2, 3, 4
**Status**: □ pending
**Acceptance**:

- GIVEN tasks 1-4 are complete AND their commits use conventional `feat`/`fix` prefixes WHEN `mage release:release` runs THEN git-cliff produces a CHANGELOG entry covering the descriptor_heap feature
- GIVEN git-cliff computes the bumped version WHEN the release runs THEN the bump follows semver (minor for feat; patch if only fixes)
- The release commit, tag, and GitHub release all reflect the new version

### Task 6: Plan-level freshness checkpoint

**Depends on**: Task 5
**Status**: □ pending
**Acceptance**:

- GIVEN the plan is nominally complete WHEN the checkpoint runs THEN CHANGELOG.md contains a plan-level Added entry summarizing descriptor_heap support (written by git-cliff during Task 5 — this step verifies presence, doesn't duplicate)
- GIVEN the plan is complete WHEN the checkpoint runs THEN PROGRESS.md has a plan-level summary entry separate from per-cycle entries
- GIVEN the plan is complete WHEN the checkpoint runs THEN TODO.md contains no pending items referencing descriptor_heap or `vkd3d_heap` work
- GIVEN the plan is complete WHEN the checkpoint runs THEN PLAN.md is archived to `.agentera/archive/PLAN-2026-04-19-descriptor-heap.md` and removed from `.agentera/PLAN.md`

## Overall Acceptance

- GIVEN a user with Proton-CachyOS 10.0-20260321+ AND NVIDIA driver 580.94.16+ WHEN they enable `vkd3d_heap` via CLI or TUI and launch a DX12 game THEN both env vars are set and no preflight warning fires
- GIVEN a user with stock Valve Proton or Proton-GE WHEN they enable `vkd3d_heap` in the TUI or run `proton show` THEN a visible notice names the Proton build requirement at toggle/show time, and a `slog.Warn` also fires at launch
- GIVEN a user with an older NVIDIA driver WHEN they enable `vkd3d_heap` THEN a visible notice names the minimum driver version at toggle/show time, and a `slog.Warn` also fires at launch
- GIVEN the resolver encounters a genuinely unexpected Proton build (unknown marker outcome) WHEN the user enables or launches THEN launch is not blocked and the ambiguity is logged as info

## Risks

- **Version-constant rot**: Minimum driver version (580.94.16) and minimum Proton-CachyOS build date (20260321) are moving targets. NVIDIA may backport, or the required versions may shift after upstream vkd3d-proton merges PR #2943. Mitigation: constants in one file with PR/issue comments. Acceptance at next inspektera audit: verify constants still match upstream state.
- **Proton detection fragility**: Grep-for-marker is brittle. Proton-CachyOS may rename, compile out, or relocate the marker at any release. Mitigation: if detection becomes noisy, switch to toolmanifest or CHANGELOG parsing. The "unknown → info log, launch proceeds" path ensures fragility never blocks users.
- **GUI parity gap**: GUI has no toggle this cycle. A GUI-only user cannot enable `vkd3d_heap` or see its warnings. This is a deferred UX inconsistency, not a graceful limitation. Mitigation: add a follow-up TODO entry after plan completes; target the next GUI-focused cycle.
- **User expectation mismatch**: Users may enable `vkd3d_heap` expecting performance gains that depend on game-specific behavior (some DX12 titles benefit dramatically; others don't). Spela can't know per-game deltas without the community profile layer. Mitigation: the `spela proton show` notice describes what the flag does, not what performance change to expect.

## Surprises
<!-- Populated by realisera during execution when reality diverges from plan. -->
