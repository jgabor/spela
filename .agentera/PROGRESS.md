# Progress

## Cycle 53 — 2026-04-19

**Phase**: build
**What**: Task 4 of the descriptor_heap plan — wired launch-time preflight warnings for `vkd3d_heap=true`. Refactored `internal/proton/notice.go` to extract `CheckCompatibility(appID, NoticeDeps) CompatibilityResult` as the structured primitive; `CompatibilityNotice` now consumes that result rather than running evaluation inline. The struct carries `{ProtonOK, ProtonDetected, ProtonSkip, DriverOK, DriverDetected, DriverSkip}` — exactly the fields both surfaces need, no more (YAGNI on expansion). Extended `internal/logging` with `Info(msg, args...)` and a `SetHandler(h) func()` hook that returns a cleanup closure so tests can redirect the slog handler to a bytes.Buffer without leaking into other tests. Added `Launcher.VKD3DCompatibilityCheck VKD3DCompatibilityCheckFunc` (injectable hook) with a `defaultVKD3DCompatibilityCheck` that wires production Steam config + NVML driver probe; preflight runs from `Prepare()` only when `l.Profile.Proton.VKD3DHeap == true` and `l.Game != nil`. Two hard-incompatibility Warns (separate per axis: `"vkd3d_heap: Proton build does not support descriptor_heap"` with `detected`/`minimum` keys and `"vkd3d_heap: NVIDIA driver below minimum"` with the same key names) and two info-skip paths (resolver couldn't run, driver unavailable). Non-blocking: preflight never returns an error, never panics, never blocks Launch.
**Commit**: (pending) feat(launcher): preflight warnings when vkd3d_heap enabled on incompatible env
**Inspiration**: The existing `protonCompatibilityNotice` CLI indirection pattern — moved one layer deeper by having `CompatibilityNotice` consume a `CompatibilityResult` rather than compute in place. Now the three surfaces (CLI notice, TUI notice, launcher preflight) share identical evaluation semantics while rendering independently: string glyphs for humans, key/value slog for logs.
**Discovered**: The notice helper's precedence rule ("hard incompatibility on either axis beats an info skip on the other") translates cleanly to the launcher: each axis logs independently. A resolver skip + driver-too-old condition emits both one `INFO` and one `WARN` — verified by `TestPrepare_VKD3DHeap_ResolverError_DriverHardStillWarns`. Launcher importing `config`/`gpu`/`steam`/`proton` introduced no cycles (verified via `grep -r jgabor/spela/internal/launcher` — only TUI and GUI import launcher).
**Verified**: `mage test` → all packages PASS. `mage lint` → 0 issues. Ran `go test ./internal/launcher/ -run VKD3D -v` → 10/10 PASS in 0.002s:

- `TestPrepare_VKD3DHeap_ProtonIncompat_LogsWarn` — captured text handler line: `level=WARN msg="vkd3d_heap: Proton build does not support descriptor_heap" detected=GE-Proton10-34 minimum=10.0-20260321`, zero driver-warn lines
- `TestPrepare_VKD3DHeap_ProtonIncompat_NoWarnWhenDisabled` — vkd3d_heap=false with same incompat result; injected checker set to `t.Fatal` on invocation — never called, captured buffer contains no `vkd3d_heap` substring
- `TestPrepare_VKD3DHeap_DriverIncompat_LogsWarn` — captured line: `level=WARN msg="vkd3d_heap: NVIDIA driver below minimum" detected=570.86.0 minimum=580.94.16`, no proton-warn line
- `TestPrepare_VKD3DHeap_DriverOK_NoDriverWarn` — vkd3d_heap=true, ProtonOK+DriverOK → captured buffer contains no `NVIDIA driver below minimum`
- `TestPrepare_VKD3DHeap_BothCompatible_NoLogs` — ProtonOK+DriverOK happy path → captured buffer contains no `vkd3d_heap` substring at all
- `TestPrepare_VKD3DHeap_BothIncompat_LogsTwoWarns` — both axes bad → `strings.Count(out, "level=WARN") == 2` and both expected messages present
- `TestPrepare_VKD3DHeap_ResolverError_LogsInfo` — ProtonSkip set, DriverOK → captured line `level=INFO msg="vkd3d_heap: could not resolve Proton for appID; skipping Proton compatibility check" reason="could not resolve active Proton for this game; skipping compatibility check" appID=1091500`, zero `level=WARN` lines
- `TestPrepare_VKD3DHeap_ResolverError_DriverHardStillWarns` — ProtonSkip + DriverOK=false → captured buffer contains both `level=INFO` (resolver skip) and `level=WARN` (driver below minimum), proving precedence: axes log independently
- `TestPrepare_VKD3DHeap_Disabled_NoLogs` — vkd3d_heap=false with `t.Fatal` sentinel checker → preflight never invoked checker, captured buffer has zero `vkd3d_heap` lines
- `TestPrepare_VKD3DHeap_EnabledInvokesChecker` — vkd3d_heap=true → invoked flag flipped to true, AppID 1091500 propagated
- Also: `go test ./internal/proton/ -run CompatibilityNotice -v` → 9/9 PASS unchanged after the `CheckCompatibility` refactor (existing TUI+CLI notice tests unmodified)

**Next**: Task 5 (cut v0.4.0 release) — now unblocked with Tasks 1–4 complete. Task 6 (plan-level freshness checkpoint) follows Task 5.
**Context**: intent — honest preflight at launch time so users who set `vkd3d_heap=true` on incompatible environments see immediate, structured feedback in logs rather than a silent no-op · constraints — non-blocking (launch proceeds regardless), no GUI/CLI/TUI changes in this task, reuse Task-3 compat helpers rather than duplicating · unknowns — none · scope — `internal/proton/notice.go` (refactor + `CompatibilityResult` + `CheckCompatibility`), `internal/logging/logging.go` (+Info, +SetHandler), `internal/launcher/launcher.go` (+vkd3dPreflight, +VKD3DCompatibilityCheck field, +defaultVKD3DCompatibilityCheck), `internal/launcher/preflight_test.go` (new, 10 tests), `.agentera/PLAN.md` (Task 4 → complete)

## Cycle 52 — 2026-04-19

**Phase**: build
**What**: Task 3 of the descriptor_heap plan — exposed `vkd3d_heap` through the CLI (`spela proton set --vkd3d-heap=<bool>`, `spela proton show` formatted + `--json`) and the TUI profile widget (new `VKD3D heap` toggle in the Proton group). Added `internal/proton/notice.go` with a `CompatibilityNotice(appID, NoticeDeps)` helper that wires the resolver + driver parser + minimum-version constants and returns a single ready-to-render string: `⚠` for hard incompatibility (Proton too old, driver too old, or both), `ⓘ` for info-level skips (resolver error, driver unavailable), empty for compatible. Hard incompatibility on either axis wins over an info-level skip on the other. Both surfaces call the helper only when the toggle is true — disabled profiles pay zero cost. Exposed `gpu.DriverVersionString()` (NVML-preferred, nvidia-smi fallback) as the smallest public entry point into the existing `getInfoNVML()` driver read, keeping the NVML map untouched. CLI uses a package-level `protonCompatibilityNotice` indirection so tests stub out filesystem+NVML; TUI threads the notice through `Services.VKD3DNotice` (new field) and `ProfileWidgetModel.SetVKD3DNoticeSource(func() string)` (new method), consulted only inside `renderWidgetBox` directly below the `vkd3d_heap` row.
**Commit**: (pending) feat(proton): expose vkd3d_heap in CLI and TUI with compatibility notice
**Inspiration**: Existing `--hdr`/`--wayland`/`--ngx-updater` flag pattern in `cmd/spela/commands/proton.go` and the declarative `WidgetField` template in `internal/tui/profile_fields.go` — VKD3D heap follows both exactly (bool toggle with `(default)`/`true`/`false` options). Services-DI pattern already established in `internal/tui/services.go` made the TUI notice test straightforward.
**Discovered**: `runProtonSet` originally mapped any non-`"true"`/`"1"` flag value silently to false, swallowing typos like `--hdr=yse`. Added `parseBoolFlag` with an explicit error for unknowns and used it for `--vkd3d-heap` only (didn't retrofit the older flags since that's outside this task's scope — logged as implicit follow-up). The `--json` output for `proton show` automatically picks up `VKD3DHeap` via struct marshaling since `ProtonSettings` already had the field from Task 1 — no CLI-side plumbing needed. `ParseDriverVersion("570.86")` normalizes to `"570.86.0"` via `DriverVersion.String()`, which is what the notice surfaces — deliberate: callers see the parsed canonical form.
**Verified**: `mage test` → all packages PASS (cached after change). `mage lint` → 0 issues. Four new test functions in `internal/tui/profile_widget_test.go` plus four in `cmd/spela/commands/proton_test.go` plus nine in `internal/proton/notice_test.go`. Explicit commands and results:

- `go test ./internal/proton/ -run CompatibilityNotice -v` → 9/9 PASS in 0.002s
  - `TestCompatibilityNotice_AllCompatible` — Proton `cachyos-10.0-20260410-slr` + driver `580.94.16` → `""`
  - `TestCompatibilityNotice_ProtonIncompatible` — Proton `GE-Proton10-34` (marker absent) + driver `580.94.16` → `"⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ (detected: GE-Proton10-34)"`
  - `TestCompatibilityNotice_DriverIncompatible` — compatible Proton + driver `570.86` → `"⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: 570.86.0)"`
  - `TestCompatibilityNotice_BothIncompatible` — Proton `GE-Proton10-34` + driver `570.86` → combined `⚠` mentioning both minimums and both detected values
  - `TestCompatibilityNotice_ResolverErrorAndDriverOK` — `ErrProtonNotResolved` + compatible driver → info `ⓘ could not resolve active Proton for this game; skipping compatibility check`
  - `TestCompatibilityNotice_DriverUnavailableAndProtonOK` — compatible Proton + empty driver string → info `ⓘ NVIDIA driver not detected; skipping driver compatibility check`
  - `TestCompatibilityNotice_ResolverErrorButDriverTooOld` — resolver error + driver `570.86` → still surfaces `⚠` for driver (hard beats info)
  - `TestCompatibilityNotice_ProtonProbeError_InfoSkip` — non-sentinel resolver error + compatible driver → info-level skip
  - `TestCompatibilityNotice_NilDeps_ReturnsEmpty` — zero-value `NoticeDeps` → `""`
- `go test ./cmd/spela/commands/ -run Proton -v` → 4/4 PASS in 0.002s
  - `TestRunProtonSet_VKD3DHeap_Persists` — `--vkd3d-heap=true` on seeded AppID 1091500 → `profile.Load(1091500).Proton.VKD3DHeap == true`
  - `TestRunProtonSet_VKD3DHeap_InvalidValue` — `--vkd3d-heap=maybe` → error containing `"vkd3d-heap"`, profile not persisted
  - `TestRunProtonShow_RendersNoticeWhenIncompatible` — seeded profile with `VKD3DHeap=true` + stubbed `protonCompatibilityNotice` → captured stdout contains `"VKD3D heap:"` and `"requires NVIDIA driver 580.94.16"`
  - `TestRunProtonShow_NoNoticeWhenToggleDisabled` — `VKD3DHeap=false` → notice source never called, notice text absent from stdout
- `go test ./internal/tui/ -run VKD3D -v` → 3/3 PASS in 0.002s
  - `TestProfileWidget_VKD3DHeap_FieldPresent` — focuses the new field in the Proton group and sends `right` → `profile.Proton.VKD3DHeap == true`, `modified == true`
  - `TestProfileWidget_VKD3DHeap_NoticeRendered` — profile with `VKD3DHeap=true` + injected notice source → `View()` output contains `"VKD3D heap"` and `"580.94.16"`
  - `TestProfileWidget_VKD3DHeap_NoticeSkippedWhenDisabled` — `VKD3DHeap=false` → notice source never invoked, notice text absent from rendered output
**Next**: Task 4 (launcher preflight warnings) — still unblocked, independent of this task. Task 5 (release) waits on Task 4.
**Context**: intent — surface the vkd3d_heap toggle through every pre-launch human touchpoint (CLI set + show, TUI widget) with honest inline compatibility feedback that matches the vision principle "every env var set, every knob visible" · constraints — additive only, no GUI parity this cycle, no launcher changes, no external deps, TUI field strictly after NGX, hard incompatibility wins over info skips · unknowns — none · scope — `cmd/spela/commands/proton.go` + `proton_test.go` (new), `internal/proton/notice.go` + `notice_test.go` (new), `internal/gpu/nvidia.go` (+1 public fn), `internal/tui/profile_fields.go` (+1 field), `internal/tui/profile_widget.go` (+2 methods, +1 render block), `internal/tui/profile_widget_test.go` (+3 tests), `internal/tui/services.go` (+1 field, +1 fn), `internal/tui/content.go` (wire notice), `internal/tui/testhelpers_test.go` (stub field)

## Cycle 51 — 2026-04-19

**Phase**: build
**What**: Task 2 of the descriptor_heap plan — created new `internal/proton` package exposing `ResolveForAppID`, `SupportsVKD3DHeap`, `ParseDriverVersion` + `DriverVersion.Compare`/`MeetsMinimum`/`String`, sentinel errors `ErrProtonNotResolved` and `ErrDriverUnavailable`, and constants `MinDriverVersion = "580.94.16"` / `MinProtonCachyOSBuild = "10.0-20260321"` in a dedicated `requirements.go` with PR/release citations. Resolver walks `config/config.vdf` → `InstallConfigStore → Software → Valve → Steam → CompatToolMapping` with per-AppID → global `"0"` fallback, then locates the build directory under `compatibilitytools.d/<name>` (community) or `steamapps/common/<name>` (built-in). Marker detection is a literal byte scan of the top-level `proton` script for `PROTON_VKD3D_HEAP`; missing script returns `(false, nil)` per the "unsupported but not error" contract. Driver parser trims whitespace, handles 1–4 components, returns typed `ErrDriverUnavailable` on empty input and a descriptive wrapped error on garbage. No consumers wired — Tasks 3 and 4 will consume this surface.
**Commit**: feat(proton): add build resolver, marker detection, and driver version gate
**Inspiration**: `internal/dll/version.go` as the template for version comparison semantics (kept the idiom of splitting on `.` and tolerating missing components).
**Discovered**: `internal/gpu` has no public `DriverVersion()` function — the driver string is read inline inside `getInfoNVML()` into a map. Rather than refactor that out in this task (which would conflate Task 2 with a gpu-package change), `ParseDriverVersion` takes the raw string as input and the launcher preflight (Task 4) will be responsible for fetching and passing it in. `steam.VDFNode.GetNode` safely chains on nil-map receivers since the method operates on a map type, so `root.GetNode(…).GetNode(…)` is a clean way to walk the nested config. Community tool names in `config.vdf` typically match the directory name under `compatibilitytools.d/` exactly (verified against local `~/.steam/root/compatibilitytools.d/` which contains `cachyos-10.0-20260410-slr`, `GE-Proton10-34`, etc.), and built-in Proton uses names like `"Proton Hotfix"` that match `steamapps/common/Proton Hotfix/` — the two-candidate locate-strategy covers both.
**Verified**: `mage test` exit 0 (all packages). `mage lint` → 0 issues after `gofumpt -w internal/proton/`. `go test ./internal/proton/... -v` → 14 subtests PASS in 0.002s:

- `TestParseDriverVersion_Shapes/three-component_at_minimum` — "580.94.16" → {580,94,16, Available:true}, `MeetsMinimum()=true`
- `TestParseDriverVersion_Shapes/two-component_below_minimum_patch` — "580.94" → {580,94,0}, `MeetsMinimum()=false` (patch 0 < 16)
- `TestParseDriverVersion_Shapes/beta_with_zero_minor_above_minimum_major` — "585.0.0" → {585,0,0}, `MeetsMinimum()=true`
- `TestParseDriverVersion_Shapes/whitespace-padded_from_nvidia-smi_fallback` — "  580.94.16  " → parses and meets minimum
- `TestParseDriverVersion_Empty_ReturnsUnavailable` — `""` and `"   "` both → `errors.Is(err, ErrDriverUnavailable)`, `String()="unavailable"`, `MeetsMinimum()=false`
- `TestParseDriverVersion_Garbage_ReturnsTypedError` — "not-a-version" returns non-nil error that is NOT `ErrDriverUnavailable`
- `TestDriverVersion_Compare` — six pairs including `unavailable.Compare(available) = -1` and `unavailable.Compare(unavailable) = 0`
- `TestResolveForAppID_PerGameOverride_Found` — fake `config.vdf` with `CompatToolMapping["1091500"]="GE-Proton10-34"` + fake tool dir → returns `{Name:"GE-Proton10-34", Path:<tmp>/compatibilitytools.d/GE-Proton10-34}`
- `TestResolveForAppID_GlobalDefault_Found` — only `"0"="cachyos-10.0-20260410-slr"` mapping → AppID 1091500 resolves via fallback
- `TestResolveForAppID_NoMapping_ReturnsSentinel` — AppID not in map AND no `"0"` default → `errors.Is(err, ErrProtonNotResolved)`
- `TestResolveForAppID_MappingPresentButDirMissing_ReturnsSentinel` — mapping refers to `GE-Proton99-99` that isn't installed → sentinel
- `TestResolveForAppID_BuiltInCommonFallback` — `"Proton Hotfix"` under `steamapps/common/` (not `compatibilitytools.d/`) → resolves via second candidate path
- `TestResolveForAppID_EmptyRoot_ReturnsSentinel` — `ResolveForAppID("", 1091500)` → sentinel
- `TestSupportsVKD3DHeap_MarkerPresent` — `proton` script containing `PROTON_VKD3D_HEAP` literal → `true`
- `TestSupportsVKD3DHeap_MarkerAbsent` — `proton` script without the string → `false`
- `TestSupportsVKD3DHeap_ScriptMissing_ReturnsFalseNoError` — directory exists, no `proton` script → `(false, nil)`
- `TestSupportsVKD3DHeap_EmptyBuildPath` — zero-value `Build{}` → `(false, nil)`
**Next**: Task 3 (CLI + TUI user surface) and Task 4 (launcher preflight) now both unblocked — can run in parallel since they touch different subsystems.
**Context**: intent — create resolver/marker/driver-parser substrate that Tasks 3/4 layer UX and preflight on · constraints — no new deps, no consumer wiring, constants centralized · unknowns — whether built-in Proton ever uses an internal key that differs from its `steamapps/common/` subdir name (current assumption: name == dirname for the cases we care about; if violated, adds an `appinfo.vdf` fallback in a later cycle) · scope — new package `internal/proton/` with 3 source files + 2 test files (driver.go, resolver.go, requirements.go, driver_test.go, resolver_test.go)

## Cycle 50 — 2026-04-19

**Phase**: build
**What**: Task 1 of the descriptor_heap plan — added `VKD3DHeap bool` field to `ProtonSettings` with `yaml:"vkd3d_heap,omitempty"`, introduced `Environment.EnableVKD3DHeap()` that sets both `PROTON_VKD3D_HEAP=1` and `VKD3D_CONFIG=descriptor_heap` together (no compositional helper — YAGNI until a second VKD3D_CONFIG flag exists), and wired the helper into `applyProton`. Three new tests: enabled-branch asserts both env vars present, disabled-branch asserts both absent, YAML round-trip covers legacy-YAML load (field defaults false, omitempty keeps key out of re-marshaled zero output, re-save preserves true when flipped).
**Commit**: (pending)
**Inspiration**: plan cycle — follows the existing `EnableWayland`/`EnableHDR`/`EnableNGXUpdater` pattern exactly.
**Discovered**: `internal/profile` had no storage test file before this cycle, and `internal/env` had none at all. The round-trip coverage went into `apply_test.go` alongside the apply-branch tests rather than a new storage_test.go since the field's behavior is what matters and that file is the natural home for the profile-struct tests. YAML marshaling collapses empty structs via `omitempty` on the parent, so when only `VKD3DHeap=true` is set under an otherwise-populated `proton:` block the key appears; when the whole ProtonSettings is zero-valued the `proton:` parent disappears — the test explicitly uses legacy YAML with other proton fields set to exercise the key-level `omitempty` on `vkd3d_heap`.
**Verified**: `mage test` → all packages PASS (ok on internal/profile). `mage lint` → 0 issues. `go test -run 'TestApplyProton_VKD3DHeap|TestProtonSettings_YAMLRoundTrip' -v` → 3/3 PASS. Test assertions inspect the applied env directly: `PROTON_VKD3D_HEAP=1` and `VKD3D_CONFIG=descriptor_heap` both present when `vkd3d_heap=true`; both empty string when false. Round-trip test confirms legacy YAML without the key loads with `VKD3DHeap=false`, `omitempty` keeps `vkd3d_heap` out of re-marshaled output when false, and the key appears + round-trips when flipped to true.
**Next**: Task 2 (Proton build resolver) — independent of Task 1, can start immediately.
**Context**: intent — add the persistence + emission substrate that Tasks 3/4 layer UI and preflight on top of · constraints — additive only, omitempty, no CLI/TUI/launcher touches · unknowns — none · scope — profile.go (+1 field), env.go (+1 method), apply.go (+3 lines), apply_test.go (+tests)

## Cycle 49 — 2026-04-19

**Phase**: build
**What**: Closed both remaining `## Degraded` GUI items in one focused increment. Backend (`internal/gui/app.go`) emits `dll:progress` events from InstallDLL/UpdateDLLs/RestoreDLLs at every stage (manifest resolve, download, install/swap/restore, scan, save) and wraps every error with stage context; previously-swallowed ScanDirectory errors now log via slog.Warn. Frontend (`GameDetail.svelte`) subscribes to the event and shows the active stage next to the busy button; failures now route to a persistent dismissible error banner instead of a 3-second toast.
**Commit**: c013f1d fix(gui): surface DLL operation progress and complete error context
**Inspiration**: none — vision principles (transparency, no silent failures) drove the design directly.
**Discovered**: The GUI build was silently broken on main — `dll.GetOrDownloadDLL` was removed in 0687920 (dead-code sweep) but its callers in `internal/gui/app.go` were missed because the file is build-tagged `dev || production || bindings` and the analyzer couldn't see them. Fixed inline with a small `ensureDLLCached` helper using `DownloadDLLWithProgress`. Worktree subagent branched from a stale commit (f2227d6, pre-ludusavi-removal), so the worktree's modifications had to be cherry-picked manually rather than merged. Models.ts regen also dropped a stale `backupOnLaunch` field left over from the ludusavi removal.
**Verified**: `mage build` produced a 12MB binary at `cmd/spela/build/bin/spela`; `mage test` exit 0; `strings` confirmed all 11 new stage labels and error-wrap strings ("Resolving manifest", "Downloading %s %s", "Installing %s", "Scanning install directory", "Saving database", "Restoring backup", "save game database after install/update/restore", "scan after update/restore failed", etc.) plus 3 occurrences of `dll:progress` (one per emitting function) embedded in the binary; Svelte bundle includes the new `dllProgressStage` and `error-banner` code. GUI session itself cannot be exercised in this headless environment (no X/Wayland display).
**Next**: With both degraded items closed, the natural next move is the v0.3.0 version bump (HEALTH Audit 3 noted v0.2.1 is 28 days old with substantial unreleased content), or the Vulkan overlay layer PoC which still needs `/planera`. Could also tackle the remaining Coupling/Complexity warnings if a maintenance pass is preferred.
**Context**: intent — close both degraded GUI feedback items per vision transparency principle in one increment · constraints — Svelte component changes only, backend behavior preserved (only error messages and event emissions added) · unknowns — none, though the worktree-base drift was an unexpected friction · scope — internal/gui/app.go (3 functions wrapped + helpers), internal/gui/frontend/src/lib/GameDetail.svelte (subscription + error banner + CSS), models.ts (regen)

## Cycle 48 — 2026-04-10

**Phase**: build
**What**: TUI complexity reduction — split profile_widget.go (994→455 lines) into profile_fields.go, extracted Layout.Update() (366→74 lines) into layout_handlers.go, extracted tab/view renderers from content.go (945→680 lines) into content_views.go. All 99 TUI tests pass unchanged.
**Commit**: 0e00765 refactor(tui): reduce complexity by splitting three oversized files
**Inspiration**: none — mechanical extraction following file size guidance from HEALTH.md Audit 3
**Discovered**: Worktree agent correctly created all 3 new files but merge diagnostics showed false duplicate errors (stale); build was actually clean. The 74-line Update() is well under 80 line limit. Complexity grade should improve from C+ to B in next audit.
**Verified**: N/A: refactor-no-behavior-change
**Next**: Vision-driven work — GUI DLL progress indicator (degraded), or Vulkan overlay layer PoC (vision direction — needs /planera).
**Context**: intent — reduce TUI complexity per HEALTH.md Audit 3 warnings · constraints — 99 tests unchanged, same package · unknowns — none · scope — 3 TUI files split into 6

## Cycle 47 — 2026-04-10

**Phase**: build
**What**: Added `--json` to proton, cpu, and dlss show commands — completing machine-readable output parity across all 6 profile section show commands.
**Commit**: 16d4bd8 feat(cli): add --json flag to proton, cpu, and dlss show commands
**Inspiration**: none — finishing consistency sweep from Cycle 46
**Discovered**: All 6 show commands now follow the same pattern: formatted output by default, `--json` for machine-readable. The pattern is mechanical but important for scripting workflows.
**Verified**: `spela help {proton,cpu,dlss} show` all display `--json` flag
**Next**: `/inspektera` for health audit (17+ cycles since last). Then: GUI DLL progress indicator (degraded), Vulkan layer PoC (vision).
**Context**: intent — complete --json parity across all show commands · constraints — existing output unchanged · unknowns — none · scope — proton.go, cpu_set.go, dlss.go

## Cycle 46 — 2026-04-10

**Phase**: build
**What**: Added `--json` flag to `gpu show` and `overlay show` commands for machine-readable output, matching the `profile show --json` pattern.
**Commit**: 9e8c176 feat(cli): add --json flag to gpu show and overlay show commands
**Inspiration**: none — consistency with existing profile show --json
**Discovered**: The remaining show commands (proton, cpu, dlss) also lack --json — defer to future cycle. 16 cycles this session; remaining work needs Svelte (GUI) or C (Vulkan) — recommend fresh session.
**Verified**: `spela help gpu show` and `spela help overlay show` both display `--json` flag
**Next**: Recommend `/inspektera` — HEALTH.md is 15+ cycles stale. After audit: GUI DLL progress indicator (degraded) or Vulkan layer PoC (vision).
**Context**: intent — add --json to show commands for scripting/composability · constraints — preserve existing formatted output · unknowns — none · scope — gpu_set.go, overlay.go

## Cycle 45 — 2026-04-10

**Phase**: build
**What**: Added `--dry-run` flag to `spela launch` — shows env vars, hardware settings (clock/memory/power/fan/governor/SMT), overlay config, and launch command without executing anything. Extracted `Profile.ApplyEnv()` for env-only application.
**Commit**: 4f69e93 feat(cli): add --dry-run flag to launch command
**Inspiration**: none — the vision's transparency principle ("show everything Spela does") demanded this
**Discovered**: Needed to split `Profile.Apply()` into `ApplyEnv()` (env-only, safe for dry-run) and the full `Apply()` (env + hardware). The linter collapsed the `hasHW` check into the if statement.
**Verified**: `spela help launch` shows `--dry-run` flag; build and all tests pass
**Next**: GUI DLL progress indicator (degraded), or Vulkan layer PoC (vision direction). This session shipped 15 cycles (32-45) — strongly suggest pausing for user review.
**Context**: intent — add launch dry-run per vision transparency principle · constraints — no hardware changes in dry-run · unknowns — none · scope — launch.go, apply.go (new ApplyEnv method)

## Cycle 44 — 2026-04-10

**Phase**: build
**What**: Added fan speed field to TUI GPU profile widget — percentage presets 30-100% with (default)=auto, completing CLI→TUI parity for NVML fan speed control.
**Commit**: 496dc5d feat(tui): add fan speed field to GPU profile widget
**Inspiration**: none
**Discovered**: Nothing unexpected. The field follows the exact power-limit pattern. Existing tests pass because the structural disabled-field iteration only checks `disabled: true` fields, and the new field is enabled.
**Verified**: Profile widget renders fan speed field with value cycling that persists to the profile struct, verified by build+test passing and code inspection matching the power-limit field pattern
**Next**: GUI DLL progress indicator (degraded), or Vulkan layer PoC (vision direction). Session has been highly productive — 13 cycles today covering TUI test harness (7 tasks), overlay CLI, TUI overlay enablement, profile show, fan speed (NVML + CLI + TUI). Good stopping point.
**Context**: intent — complete fan speed CLI→TUI parity · constraints — follow power-limit pattern · unknowns — none · scope — profile_widget.go GPU settings group

## Cycle 43 — 2026-04-10

**Phase**: build
**What**: GPU fan speed control — NVML SetFanSpeed_v2/SetDefaultFanSpeed_v2 integrated into profile system with batched pkexec apply-profile, auto-reset on game exit, CLI `gpu set --fan-speed`, and formatted display in `gpu show` and `profile show`.
**Commit**: c6282a4 feat(gpu): add fan speed control to profile system
**Inspiration**: none — follows exact power-limit integration pattern from Cycle 18
**Discovered**: go-nvml exposes `GetNumFans()` for multi-fan GPUs — the setter loops over all fans. `SetDefaultFanSpeed_v2` cleanly restores automatic fan control. The `nvml` import was needed in nvidia.go for `nvml.SUCCESS` in `GetNumFans` check.
**Verified**: `spela help gpu set` shows `--fan-speed int` flag with correct description; build and all tests pass
**Next**: Add fan speed to TUI profile widget, or GUI DLL progress indicator (degraded), or Vulkan layer PoC
**Context**: intent — add fan speed control per NVIDIA depth vision direction · constraints — batched pkexec pattern, auto-reset on exit · unknowns — none · scope — gpu/nvml.go, gpu/nvidia.go, profile struct, apply.go, apply_profile.go, gpu_set.go, profile.go

## Cycle 42 — 2026-04-10

**Phase**: build
**What**: Improved `spela profile show` — replaced raw JSON dump with formatted, human-readable output covering all 5 profile sections (DLSS, GPU, CPU, Proton, Overlay) with styled labels and (default) placeholders. JSON output preserved via `--json` flag.
**Commit**: 301d814 feat(cli): improve profile show with formatted output and --json flag
**Inspiration**: none — follows existing per-section show command styling patterns
**Discovered**: Nothing unexpected. The `strconv` import was already present from `profileList`. CLI styling helpers (CLIDim, CLIPrimary) are simple wrappers around lipgloss — clean and reusable.
**Verified**: `spela help profile show` shows --json flag; both formatted and JSON paths reach game lookup correctly (error expected without Steam library on dev machine)
**Next**: GUI DLL progress indicator (last degraded item), or Vulkan layer PoC (vision direction)
**Context**: intent — improve profile show per vision transparency principle · constraints — preserve JSON output as --json · unknowns — none · scope — profile.go show command

## Cycle 41 — 2026-04-10

**Phase**: build
**What**: Enabled 7 overlay profile fields in TUI widget (Enabled, Position, ShowFPS/Frametime/CPU/GPU/VRAM) — removed `disabled: true` flags. ToggleKey and CPU Affinity remain disabled. Updated edge-case test for partially-enabled Overlay group.
**Commit**: b239876 feat(tui): enable overlay profile fields in TUI widget
**Inspiration**: none
**Discovered**: The structural disabled-field test automatically adapted — it now finds only 2 disabled fields (cpu_affinity, overlay_toggle_key) instead of 9. This validates the adversarial review's recommendation to use structural iteration rather than hard-coding a count.
**Verified**: Profile widget tests pass with 2 disabled fields; `TestProfileWidget_EnterEditing_LandsOnFirstEnabled` verifies navigation correctly handles the partially-enabled Overlay group
**Next**: GUI DLL progress indicator (degraded), or Vulkan layer PoC (vision direction)
**Context**: intent — enable TUI overlay fields now that CLI backend exists · constraints — ToggleKey stays disabled (needs textinput), tests must pass · unknowns — none · scope — profile_widget.go, profile_widget_test.go

## Cycle 40 — 2026-04-10

**Phase**: build
**What**: Added `spela overlay set/show` CLI commands — 8 flags covering all OverlaySettings fields (enabled, position, show-fps/frametime/cpu/gpu/vram, toggle-key). Closes the last gap in CLI profile management.
**Commit**: 6a54b25 feat(cli): add overlay set/show commands for per-game overlay profiles
**Inspiration**: none — follows existing proton set/show pattern exactly
**Discovered**: NVIDIA's framebuffer overlay tool intercepts bare `overlay` subcommand on NVIDIA systems (exits with "Couldn't get an overlay visual"). The `help overlay` form works. Not a code bug — environment collision with the NVIDIA driver's overlay binary in PATH.
**Verified**: `spela help overlay` outputs correct command tree; `spela help overlay set` shows all 8 flags with correct descriptions and types
**Next**: GUI DLL progress indicator (degraded in TODO.md), or start enabling the TUI overlay fields now that CLI commands exist
**Context**: intent — add overlay CLI commands to close profile management gap per TODO.md annoying item · constraints — follow proton.go pattern · unknowns — none · scope — overlay.go (new), main.go registration

## Cycle 38 — 2026-04-10 (plan summary: TUI End-to-End Test Harness)

**Phase**: build
**What**: Completed 7-task plan — TUI end-to-end test harness with injectable Services struct, model factories, key-sequence helpers, and 99 state machine tests across 6 test files covering layout (30 tests), sidebar (19), content/DLL (24), profile widget/disabled fields (16), plus factory smoke tests (10). Services DI for synchronous I/O enables pure state-machine testing without filesystem or hardware.
**Commits**: ab97dba refactor(tui): introduce Services struct for dependency injection · 996519c test(tui): add test helpers and model factories · 2b4c351 test(tui): add layout, navigation, and modal state machine tests · ce5a138 test(tui): add sidebar state machine tests · 3ea0a4e test(tui): add content and DLL operation state machine tests · b7e3c3c test(tui): add profile widget and disabled field contract tests · 9a1a816 chore: plan-level freshness checkpoint
**Discovered**: HEALTH.md Tests: C finding for TUI partially resolved — TUI package now has 99 model/Update tests (up from 0). Worktree agents consistently branched from pre-Services revision; direct implementation was more reliable for test code. SidebarModel and ContentModel aren't tea.Model implementations — tests call Update directly. Structural disabled-field iteration pattern is resilient to field additions.
**Verified**: N/A: docs-only
**Next**: Vision-driven work. GUI DLL progress indicator (degraded), overlay CLI commands (annoying), or Vulkan layer PoC (vision direction).
**Context**: intent — freshness checkpoint: CHANGELOG, TODO, PROGRESS updates for plan completion · constraints — none · unknowns — none · scope — operational artifacts only

## Cycle 37 — 2026-04-10

**Phase**: build
**What**: 16 profile widget tests — structural iteration over all 9 disabled fields (nav skip, input rejection, "Coming soon" rendering), edge cases (all-disabled Overlay group, last-enabled CPU field clamp), value cycling, save/esc, grid-enter first-non-disabled.
**Commit**: b7e3c3c test(tui): add profile widget and disabled field contract tests
**Inspiration**: none
**Discovered**: The structural test pattern (iterate groups/fields, test only `disabled: true`) is resilient to field additions — if a new disabled field is added, it gets tested automatically. The Overlay group being entirely disabled creates a clean edge case: enter editing lands on field 0, and all navigation stays put.
**Verified**: N/A: test-only
**Next**: Task 7 (freshness checkpoint) is the final task — all feature tasks are complete.
**Context**: intent — test disabled field contracts per Task 6 acceptance · constraints — structural iteration, proportionality + 3 edge cases · unknowns — none · scope — profile_widget_test.go (new, 319 LOC)

## Cycle 36 — 2026-04-10

**Phase**: build
**What**: 24 content state machine tests — no-game guards, tab switching, launch (with duplication prevention), DLL update/restore confirmation flow (Y/cancel/skip-confirmation), install wizard 3-step state machine (start/type-nav/type→version/version→download/esc/q cancel), and message handlers.
**Commit**: 3ea0a4e test(tui): add content and DLL operation state machine tests
**Inspiration**: none
**Discovered**: ContentModel.Update returns `(ContentModel, tea.Cmd)` not `(tea.Model, tea.Cmd)` — direct call pattern works cleanly without type assertions. The `dll.DLL` type needed importing for install wizard version tests. Confirmation flow has a nice property: any key except Y/y cancels, which makes boundary testing trivial.
**Verified**: N/A: test-only
**Next**: Task 6 (profile widget/disabled fields) is the last feature task before the freshness checkpoint.
**Context**: intent — test content DLL operations per Task 5 acceptance criteria · constraints — proportionality, Cmd-return level · unknowns — none · scope — content_test.go (new, 403 LOC)

## Cycle 35 — 2026-04-10

**Phase**: build
**What**: 19 sidebar state machine tests — navigation (up/down/j/k with boundary clamping), DLL filter, profile filter with injected fake, sort cycling, clear filters, multi-select (space/a/A/esc), enter confirm/batch, search activation/blur.
**Commit**: ce5a138 test(tui): add sidebar state machine tests
**Inspiration**: none
**Discovered**: SidebarModel is not a tea.Model (no Init method) — can't use sendKey helper. Tests call m.Update(keyMsg(...)) directly. This pattern also applies to ContentModel and ProfileWidgetModel for Tasks 5-6. Should add component-level send helpers to testhelpers if the pattern repeats.
**Verified**: N/A: test-only
**Next**: Tasks 5 and 6 remain. Task 5 (content/DLL) is the larger one.
**Context**: intent — test sidebar key bindings per Task 4 acceptance criteria · constraints — proportionality 1+1, injected profile fakes · unknowns — none · scope — sidebar_test.go (new, 324 LOC)

## Cycle 34 — 2026-04-10

**Phase**: build
**What**: 30 layout state machine tests covering all global key bindings, help overlay, density modes, jump keys (with/without game, focused-mode boundary), nav stack (single entry, deep pop, root protection), modal interception, batch menu (esc/nav/enter/block), and window sizing.
**Commit**: 2b4c351 test(tui): add layout, navigation, and modal state machine tests
**Inspiration**: none
**Discovered**: Implemented directly instead of worktree dispatch — cleaner when test code needs access to internal types and current API. The `rescanCompleteMsg` type doesn't exist; actual type is `rescanGamesMsg`. Batch menu only has one action so cursor boundary testing is minimal.
**Verified**: N/A: test-only
**Next**: Tasks 4, 5, 6 all unblocked. Can parallelize in next cycles.
**Context**: intent — test all Layout model key bindings and navigation per Task 3 acceptance criteria · constraints — proportionality 1+1 per binding · unknowns — none · scope — layout_test.go (new, 455 LOC)

## Cycle 33 — 2026-04-10

**Phase**: build
**What**: Test helpers and model factories — testServices (fake deps), testGame/testDLL/testDatabase (data), testLayout/testLayoutWithGame/testContent/testSidebar (models), keyMsg/sendKey/sendKeys (key simulation), execCmd (Cmd asserter). 10 smoke subtests validate all helpers including keyMsg round-trip for 12 key patterns.
**Commit**: 996519c test(tui): add test helpers and model factories for state machine testing
**Inspiration**: none — standard Go test helper patterns
**Discovered**: Bubbletea v2's `KeyPressMsg.String()` returns `"space"` not `" "` for space key — the worktree agent's test expected `" "`. Worktree branched from pre-Services revision, required full adaptation of model factories to use Services API. `sendKeys` triggered unused-function lint — resolved by adding a smoke subtest.
**Verified**: N/A: test-only
**Next**: Tasks 3-6 are all unblocked. Task 3 (layout/nav/modal tests) is the broadest; Tasks 4-6 can run in parallel.
**Context**: intent — build test infrastructure factories and key helpers for subsequent state machine tests · constraints — no filesystem, bubbletea v2 key types · unknowns — none · scope — testhelpers_test.go (new, 377 LOC)

## Cycle 32 — 2026-04-10

**Phase**: build
**What**: Introduced Services struct for TUI dependency injection — function fields for config loading, profile loading/existence, and backup existence threaded through Layout, Content, and Sidebar constructors. Production uses DefaultServices(); tests can substitute fakes.
**Commit**: ab97dba refactor(tui): introduce Services struct for dependency injection
**Inspiration**: none — standard Go function-field injection pattern
**Discovered**: Synchronous I/O was spread across 12 call sites in 3 files. `loadEffectiveProfile` converted from package-level function to ContentModel method. Async tea.Cmd closures (DLL download, GPU metrics, game launch) intentionally left as direct calls — they're tested at Cmd-return level in later tasks.
**Verified**: N/A: refactor-no-behavior-change
**Next**: Task 2 — test helpers and model factories (depends on this task)
**Context**: intent — enable pure state-machine testing of TUI models by replacing synchronous I/O with injectable fakes · constraints — no behavior change, async Cmd closures untouched · unknowns — none · scope — services.go (new), layout.go, content.go, sidebar.go

## Cycle 31 — 2026-04-09 (plan summary: Ludusavi Removal and Dead Code Sweep)

**What**: Completed 3-task plan — removed ludusavi save game integration (package, profile, launcher, TUI, GUI, Svelte, docs, packaging) and swept 49 dead functions across 16 files plus unused mangohud config, self-updater package, and samber/lo dependency. Net -1,000 lines, -4 files. Dead code 16.6% → 14.3%.
**Commits**: 39a8cc6 refactor: remove ludusavi save game integration · 0687920 refactor: remove dead code and unused dependencies
**Discovered**: Worktree agents unreliable for this codebase (Cycle 29 agent deleted 3,584 lines of unrelated code). logging.go collapsed to single Warn function after ludusavi removal killed last Info caller. Architecture grade improved C → B. HEALTH.md Audit 1 dead code finding (16.6%) partially resolved — now 14.3%.
**Verified**: N/A: chore-build-config
**Next**: Vision-driven work. GUI DLL progress indicator (degraded), overlay CLI commands (annoying), or Vulkan layer PoC (vision direction).

## Cycle 30 — 2026-04-09

**What**: Dead code sweep — removed 49 unreachable functions across 16 files, deleted mangohud.go and entire update package, removed samber/lo unused dep. Dead code 16.4% → 14.3% (sentrux), 49 → 0 (deadcode -test).
**Commit**: 0687920 refactor: remove dead code and unused dependencies
**Inspiration**: none
**Discovered**: nvidia_test.go and sparkline_test.go had tests for dead functions — removed too. GPUGeneration type/constants kept but methods and parser removed. logging.go collapsed to just Warn. Architecture grade improved C → B (sentrux).
**Verified**: N/A: refactor-no-behavior-change
**Next**: Task 3 — plan-level freshness checkpoint
**Context**: intent — reduce dead code from 16.6% baseline per HEALTH.md Audit 1 · constraints — preserve test-only exports · unknowns — none · scope — 16 files across 13 packages + go.mod

## Cycle 29 — 2026-04-09

**What**: Removed ludusavi save game integration — package, profile struct field, launcher pre-launch hook, TUI backup settings widget, GUI backup checkbox, Svelte component section, docs, and AUR packaging references
**Commit**: 39a8cc6 refactor: remove ludusavi save game integration
**Inspiration**: none
**Discovered**: Svelte frontend and docs/gamemode.sh had ludusavi references that the initial integration map missed. Worktree agent went rogue — deleted sparklines, thermal gradients, navstack, help system, and operational artifacts (3,584 lines of unrelated deletion). Caught in diff review; implemented directly instead.
**Verified**: N/A: refactor-no-behavior-change
**Next**: Task 2 — dead code and unused dependency sweep
**Context**: intent — remove ludusavi to clear path for rethought save game approach · constraints — old profiles must still load · unknowns — none · scope — 13 files across 7 packages + docs + packaging

## Archived Cycles

Cycle 28 (2026-04-01): feat — TUI confirmation dialogs for destructive DLL ops
Cycle 27 (2026-04-01): feat — color flash animation on message bar
Cycle 26 (2026-04-01): feat — compositor modals, density modes, jump-key titles
Cycle 25 (2026-04-01): feat — tab-based content views (DLLs/Profile/Launch)
Cycle 24 (2026-04-01): feat — header sparklines/gauges, context keybinding bar
Cycle 23 (2026-04-01): refactor — nav stack replacing binary focus
Cycle 22 (2026-04-01): feat — sparkline/gauge renderers with thermal coloring
Cycle 21 (2026-04-01): feat — thermal gradient system, expanded theme tokens
Cycle 20 (2026-04-01): feat — enhanced cpu info CLI command
Cycle 19 (2026-04-01): feat — enhanced gpu info with NVML metrics
Cycle 18 (2026-04-01): feat — GPU power limit in profile system
Cycle 17 (2026-04-01): chore — transitive dependency update
Cycle 16 (2026-04-01): feat — proton CLI commands, GPU profile flags
Cycle 15 (2026-04-01): refactor — profile widget field registry
Cycle 14 (2026-04-01): test — launcher tests (cleanup, signals, overlay)
Cycle 13 (2026-04-01): test — CPU package tests with mock sysfs
Cycle 12 (2026-04-01): test — config persistence tests
Cycle 11 (2026-04-01): refactor — TUI threaded Styles replacing globals
Cycle 10 (2026-04-01): fix — centralized slog logging
Cycle 9 (2026-04-01): refactor — consolidated launch orchestration
Cycle 8 (2026-04-01): fix — GUI/TUI launch cleanup closures
Cycle 7 (2026-03-17): feat — overlay collector wired into launcher
Cycle 6 (2026-03-17): feat — stats collector goroutine
Cycle 5 (2026-03-17): feat — overlay mmap IPC with seqlock
Cycle 4 (2026-03-17): feat — NVML throttle reason detection
Cycle 3 (2026-03-17): feat — GPU alerts in TUI header
Cycle 2 (2026-03-17): feat — GPU alert detection system
Cycle 1 (2026-03-17): feat — go-nvml backend for GPU metrics
