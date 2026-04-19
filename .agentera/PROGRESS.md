# Progress

## Cycle 60 — 2026-04-19

**Phase**: release
**What**: Task 7 of the TUI ground-up redesign plan — version bump to v0.5.0 per the `git-cliff + magefile` release policy in `.agentera/DOCS.md`. Promoted the curated `## [Unreleased]` block in `CHANGELOG.md` to `## [0.5.0] - 2026-04-19` in place (preserving every Added/Changed/Fixed bullet that cycles 54-59 accumulated at plan level), inserted a plan-level prose summary paragraph under the version heading that frames the redesign (resource-centric spine, left rail of four resources, live inheritance, neon-accent dark palette), restored an empty `## [Unreleased]` heading above it, and appended `[0.5.0]: https://github.com/jgabor/spela/compare/v0.4.0..v0.5.0` to the link reference block. Committed `chore(release): v0.5.0` (commit hash d36a6b5) and created the annotated tag `v0.5.0` locally. Did NOT invoke `mage release:release` because that target is interactive (reads stdin for confirm/edit/abort), depends on `opencode` for an LLM summary that would overwrite the curated plan-level paragraph, and calls `git push` + `git push origin v0.5.0` unconditionally — all three conflict with the orkestrera conductor's push-gated constraint and with AC2's plan-level-summary requirement. Instead replicated the magefile's non-interactive steps by hand (`updateChangelogFile` equivalent: promote heading + preserve curated content; `commitTagPush` equivalent minus the push: `git add CHANGELOG.md`, `git commit -m "chore(release): v0.5.0"`, `git tag -a v0.5.0 -m "Release v0.5.0"`).

**Commit**: d36a6b5 chore(release): v0.5.0

**Inspiration**: `.agentera/DOCS.md` `versioning` block (`version_files: [CHANGELOG.md]`, `policy: semver, bump on release via git-cliff + magefile`) is the authoritative contract. `magefile.go` functions `updateChangelogFile` (lines 420-454) and `commitTagPush` (lines 476-494) document the exact release-commit shape and the `git tag -a <version> -m "Release <version>"` annotation form, which I mirrored verbatim. The curated Unreleased content itself was already plan-level by cycles 54-59 so the promotion preserved every bullet 1:1 — no git-cliff raw-commit output needed to be incorporated.

**Discovered**: The `mage release:release` target as written cannot produce the v0.5.0 release this plan requires without either (a) modifying the magefile to separate the "bump" from the "push" and the "LLM summary" from the core flow, or (b) running the curated manual steps used this cycle. For future plan-level releases where the Unreleased section is already curated, a non-interactive `mage release:bump` target would be a clean YAGNI-respecting addition — logged as a followup. Additionally: the PLAN.md Task 7 AC3 ("tag exists on origin") is structurally gated on the user pushing; the orkestrera conductor protocol handed this task explicitly push-deferred, so the task stays `□ pending` with an inline note rather than being marked complete.

**Verified**: Structural verification — `mage test` green across all 15 packages (cached + fresh runs both pass), `mage lint` reports 0 issues, `mage build` produces a 12 MB binary plus the Wails GUI bundle (`dist/index.html 2.48 kB`, `dist/assets/index-DHYszOwN.css 32.43 kB`, `dist/assets/index-CVtoQoVE.js 72.27 kB`). CHANGELOG.md post-bump inspection: `## [Unreleased]` heading present on line 8 with zero content lines beneath it (next heading `## [0.5.0] - 2026-04-19` on line 10); v0.5.0 section contains the plan-level summary paragraph, Added (1 bullet: live inheritance + reset verbs + show markers), Changed (5 bullets: theme collapse, shell rewrite, shared detail renderer, inheritance rendering + reset/pin bindings, DLLs + Metrics resources), and Fixed (1 bullet: DLSS preset dedup). Tag list — `git tag --list v0.5.0` → `v0.5.0`. Commit log — `git log --oneline -1` → `d36a6b5 chore(release): v0.5.0`. AC3 (tag exists on origin): **pending user push — local only; awaiting `git push && git push origin v0.5.0` from user**.

**Next**: Task 8 (plan-level freshness checkpoint) — the final task of the TUI ground-up redesign plan. Closes out the plan by writing a "Plan Summary — TUI ground-up redesign" entry to PROGRESS.md, moving superseded TODO.md items to Resolved, marking all PLAN.md tasks complete, and archiving PLAN.md to `.agentera/archive/PLAN-2026-04-19-tui-redesign.md`. Task 8 is the orkestrera conductor's responsibility after the user confirms the push has completed (since AC for Task 8 reference a fully-released v0.5.0).

**Context**: intent — cut v0.5.0 per the documented git-cliff + magefile policy while honoring the orkestrera push-deferred constraint and preserving the curated plan-level CHANGELOG content · constraints — no push; no overwrite of curated plan-level Added/Changed/Fixed; no new dependencies; `mage test`/`lint`/`build` must be clean post-bump; commit message must match `chore(release): v0.5.0` per magefile convention · unknowns — none at release time; the only pending state is AC3 user push · scope — CHANGELOG.md (promote Unreleased to v0.5.0 + add summary paragraph + link reference), .agentera/PLAN.md (Task 7 status annotation), .agentera/PROGRESS.md (this entry); git commit + annotated tag produced locally

## Cycle 59 — 2026-04-19

**Phase**: build
**What**: Task 6 of the TUI ground-up redesign plan — the DLLs and Metrics resource panes stop rendering stubs and carry real content. The DLLs pane renders two always-visible sections: a library section enumerating every DLL family Spela recognises (DLSS, DLSS-G, DLSS-D, XeSS, FSR) paired with its latest manifest version and newest cached payload plus count, and a deployment section whose games × DLL-types matrix shows the installed version in each cell. Cells older than the newest cached version carry a magenta `◆` marker via the existing `AccentOverride` token (same token used for inheritance overrides per Task 5). Rows for DLL types with zero installed games are omitted from the matrix; the edge case where no tracked game installs any managed DLL renders an informative message. A new `U` (also `ctrl+u`) binding triggers a batched update-all that walks every stale cell, copies its newest cached payload into the game install dir via `dll.SwapDLL`, and reflects per-cell outcome ("ok" / "err:") in `lastBatchResult` once the batch completes. The Metrics pane relocates the existing thermal-gradient, sparkline, and gauge widgets into the resource body with no change in underlying data or rendering — `HeaderModel` retains the 2-second polling loop and rolling `MetricsBuffer`s, while the layout now threads the header's state into a `MetricsResourceModel` via `SetMetricsData` each render so the Metrics pane draws the same thermal-coloured temperature / utilisation / power sparklines, VRAM / RAM gauges, clock / fan / governor lines, and overlay alerts. Two tiny dll-package additions expose the inventory: `dll.KnownDLLTypes()` (ordered catalogue) and `dll.ListCachedVersions(key)` (per-type cache-dir scan). Help screen documents the new DLLs-resource bindings (`j`/`k` row focus, `U` / `ctrl+u` update-all) and Metrics-resource tab-only affordance.

**Commit**: 7e7752d feat(tui): fill DLLs and Metrics resources with real content

**Inspiration**: .agentera/DECISIONS.md Decision 1 (firm) — two-section DLL view and metrics-as-resource both flow from the resource-centric spine. Task 5's `AccentOverride` magenta token reused verbatim as the stale-cell marker to stay visually consistent with inheritance-override rendering. Existing `RenderSparkline` / `RenderGauge` / `ThermalColor` primitives consumed unchanged so the state-machine test suite for them continues to pass bit-for-bit.

**Discovered**: The existing batch-update path in `layout.go` operated over selected games in the sidebar, downloading manifest entries on the fly. For the DLLs resource we prefer the already-cached path (the newest locally cached payload) — no network round-trip and no manifest lookup is required because `ListCachedVersions` enumerates what's downloaded. This keeps the batched action deterministic and offline-friendly. The `dllsUpdateAllCompleteMsg` has to be routed unconditionally by the resource pane — the user may have navigated away during the batch, so tying delivery to `active == ResourceDLLs` would drop messages.

**Verified**: Behaviour change: DLLs and Metrics resource panes. Structural verification — `mage test` green (all packages pass including the unchanged TUI state-machine suite), `mage lint` reports 0 issues, `mage build` produces a binary. Behavioural verification — the Task 6 render-probe test (`SPELA_RENDER_PROBE=1 go test ./internal/tui -run TestRenderProbe_Task6 -v`) emits ANSI output showing both panes. The DLLs-resource render shows: library rows for DLSS / DLSS-G / DLSS-D / XeSS / FSR (with DLSS-G carrying `3.8.10` / `3.8.10` / `2` and DLSS-D / XeSS / FSR carrying `-` / `-` / `0`), the deployment matrix omitting the DLSS-D / XeSS / FSR rows (acceptance: zero-install rows absent), and Alpha's `3.7.0` DLSS cell carrying the magenta `◆` marker ahead of the version string while Beta's `3.8.10` DLSS cell and Alpha's `1.0.3` DLSS-G cell render without the marker (acceptance: stale marker present exactly where expected). The Metrics-resource render shows thermal-coloured Temp (72°C yellow), Util (78%), Power (250W / 350W orange) sparklines, VRAM/RAM gauges at 50% / 38%, GPU clock 2400 MHz, Fan 65%, CPU Util 55% sparkline, Freq 4200 MHz, Governor "performance", and the "⚠ High temp" alert (acceptance: existing widgets render inside the pane unchanged). The 9 new DLLs-resource state-machine tests pass (`TestDLLsResource_*`) and cover library coverage, row omission for zero-install DLL types, stale-cell marker, up-to-date absence, zero-game edge case, j/k navigation with clamping, U no-op when nothing is stale, and batch-complete cache refresh.

**Next**: Task 7 — version bump per DOCS.md convention (v0.5.0) with CHANGELOG and git tag. Then Task 8 (plan-level freshness checkpoint) closes out the TUI redesign plan.

**Context**: intent — fill the two remaining resource panes and relocate metrics widgets into the new Metrics pane · constraints — no regression in metrics state-machine tests, minimal dll-package surface, reuse AccentOverride token for stale marker · unknowns — whether cached-first update-all might bypass a newer manifest entry (accepted: the cache is refreshed on start-up when the manifest is older than 24h, matching existing `GetManifest` semantics) · scope — internal/tui/dlls_resource.go (new), internal/tui/metrics_resource.go (new), resource_view.go router, layout.go + layout_handlers.go wiring, help.go keybinding docs, dll/detect.go KnownDLLTypes, dll/download.go ListCachedVersions

## Cycle 58 — 2026-04-19

**Phase**: build
**What**: Task 5 of the TUI ground-up redesign plan — the densest cycle: inheritance rendering in the Games detail view (inherited fields in `FgMuted` with no marker, overridden fields in `Fg` with a magenta `◆` marker in `AccentOverride`), three new Games-resource bindings (`r` resets the focused field, `shift+R` resets the whole game profile, `p` pins the currently-resolved value on an inherited field as an override), a displacement of the Restore DLLs binding from `R` to `ctrl+shift+r`, a new `profile.PinField(field, defaults)` primitive that captures the resolved value at pin time so the pinned field survives subsequent defaults edits, and a defensive dedup of the DLSS preset picker via a new `dedupePresets` helper so each preset renders at most once even if the raw order gains a duplicate. Help screen updated to document every new binding plus the displaced restore. Isolation is enforced three ways: (1) `DetailModel.IsOverridden/ResetFocused/ResetAll/PinFocused` all short-circuit when `isRoot=true`, (2) the `r`/`R`/`p` handlers in `ContentModel.Update` guard on `m.game != nil`, and (3) the `updateDefaults` path in `resourcePaneModel` does not route those keys into any reset/pin handler. Tests: 11 new profile-package tests around `PinField` and 13 new TUI tests around detail rendering/reset/pin/DLSS-dedup, each with pass+fail pairs per new helper.

**Commit**: c694b47 feat(tui): games inheritance rendering, reset/pin bindings, DLSS dedup

**Inspiration**: .agentera/DECISIONS.md Decision 1 (firm) — live inheritance keystone. Task 2's `profile.Reset`/`MarkOverride`/`ResolveForApply` primitives; extended them with `PinField` that reads from defaults via the existing `fieldAccessor`+`deepCopyValue` plumbing so no parallel copy code was introduced. Task 1's `InheritedStyle`/`OverrideStyle`/`OverrideMarkerStyle`/`FocusStyle` helpers on `Styles` consumed as-is; no new tokens needed. Task 3's keymap audit identified the `r`/`p`/`shift+r` displacements but missed the collision between `R` and `shift+R` — cleared by moving Restore DLLs to `ctrl+shift+r` and logging the displacement in the same "Displaced bindings" help section.

**Discovered**: The Task 3 keymap audit declared "`shift+R` and bare `R` (restore DLLs) do not collide" — but Bubbletea v2 emits both as the literal `"R"` key string, so the collision was real. Fix was one more binding displacement, now documented. Another micro-discovery: the ContextKeys help bar had no entries for `r`/`R`/`p`; added them so the keybinding bar honestly advertises what's available when a game is selected. The `dedupePresets` helper also let the package drop the old bare `dlssPresetOrder` var in favor of a derived slice without breaking any consumer (they all referenced the same name). One false-positive in the first render probe — a value-vs-pointer mistake in the test harness (advancing cursor j-forward past the target field instead of rebuilding from zero) made `PinFocused` look broken; the production code was fine, probe fixed by rebuilding the DetailModel before walking to the next target. Test count went from 170 functions → ~194 (11 new profile tests + 13 new TUI tests).

**Verified**: `mage test` → 15 packages PASS (cmd/spela, cmd/spela/commands, internal/config, internal/cpu, internal/denylist, internal/dll, internal/game, internal/gpu, internal/launcher, internal/overlay, internal/privilege, internal/profile, internal/proton, internal/steam, internal/tui). `mage lint` → 0 issues. `mage build` → 12 MB binary rebuilt plus Wails GUI bundle (dist/index.html 2.48 kB, dist/assets/index.css 32.43 kB, dist/assets/index.js 72.27 kB). New tests run green:
`go test ./internal/profile/ -run PinField -v` → 4/4 PASS in 0.001s (`TestPinField_Pass_CopiesDefaultsValue`, `TestPinField_Fail_UnknownField`, `TestPinField_AlreadyOverriddenNoop`, `TestPinField_NilDefaults_PinsZero`).
`go test ./internal/tui/ -run 'TestDetail|TestDedupe|TestDLSSPreset' -v` → 28/28 PASS.

Behavioral render probe (scratch `TestRenderProbe_Task5` against a synthetic Cyberpunk 2077 profile with `Overrides={gpu.power_limit:true, proton.enable_hdr:true}` over defaults {PowerLimit=350, EnableWayland=true, VKD3DHeap=true, SRMode=quality}):

- **Before reset**: two `◆` markers in the view (HDR row and Power limit row — both magenta `\x1b[38;2;255;95;210m`). HDR row body renders in Fg `\x1b[38;2;232;232;240m true` (overridden). Every inherited field (Wayland, NGX updater, VKD3D heap, SR mode=quality, etc.) renders in FgMuted `\x1b[38;2;106;106;128m`. The focused Power limit row renders in focus cyan+bold `\x1b[1;38;2;95;240;255m` and shows the overridden value "400". Section headers (Proton, DLSS, GPU, CPU, Overlay) are bold magenta. Raw `Overrides={gpu.power_limit:true, proton.enable_hdr:true}`, raw.GPU.PowerLimit=400.
- **After `r` on PowerLimit**: ResetFocused returned `changed=true, err=<nil>`. Raw `Overrides={proton.enable_hdr:true}` (PowerLimit flag removed); raw.GPU.PowerLimit=0 (zeroed). View re-rendered: Power limit row now shows "350" in focus cyan+bold (resolved from defaults), NO `◆` marker in front of it; the HDR row still carries its `◆` marker and renders in Fg. Marker count dropped from 2 → 1.
- **After `p` on Wayland (inherited)**: PinFocused returned `changed=true`. Raw.Proton.EnableWayland=true (copied from defaults), `raw.IsOverridden(proton.enable_wayland)=true`. Persistence check: flipping `defaults.Proton.EnableWayland=false` then calling `raw.ResolveForApply(defaults)` returned Wayland=true — confirming the pin survives a subsequent defaults change (this is the whole reason `p` exists).
- **After `shift+R` (ResetAll)**: returned `changed=true`. Raw.Overrides=`map[]` (empty map — HDR override cleared). raw.Name="Cyberpunk 2077" preserved (not an inheritance-tracked field).
- **DLSS dedup**: `dlssPresetOrder` has `len=11` and contents `[default A B C D E F J K L M]` — each preset exactly once. Feeding an adversarial `[A A B C B A]` (len=6) through `dedupePresets` returns `[A B C]` (len=3) with first-seen order preserved.
- **Root suppression** re-verified: `TestDetail_RootSuppressesResetPinBindings` calls `ResetFocused`/`ResetAll`/`PinFocused` on a root DetailModel and asserts every one returns changed=false with the root profile's PowerLimit=350 untouched. `TestDetailView_RootSuppressesMarkersAndMuting` confirms the root view contains zero `◆` glyphs.

**Next**: Task 6 (DLLs + Metrics resources) — unblocked on Task 3 alone, independent of Task 5. Task 7 (version bump to v0.5.0) is the last gate, depending on all six content tasks. With Task 5 landed, the critical-path constraint on Task 7 now reduces to Task 6.

**Context**: intent — decorate the Task 4 structural spine with the full Task 5 feature set in one cycle: per-row inheritance styling, three reset/pin bindings, DLSS dedup, and help-screen documentation · constraints — Defaults resource must not gain any marker rendering or reset/pin handler; `p` on an overridden field is a no-op; every new behavior has a 1 pass + 1 fail test pair; all YAGNI gates honored (no new Profile struct field, no new theme token, no new keymap globals beyond what Task 3 reserved + one displacement) · unknowns — none; Task 4 shipped the structural spine cleanly and Task 2 primitives covered everything except pinning, which was a clean extension · scope — internal/profile (inheritance.go +PinField, inheritance_test.go +4 tests), internal/tui (detail.go rewrites View + adds ResetFocused/ResetAll/PinFocused/RawProfile/rebuildResolved, detail_test.go +13 tests, dlss_preset_modal.go adds dedupePresets, content.go wires r/R/p/ctrl+shift+r, content_test.go updates Restore tests, help.go updates bindings + ContextKeys, help_test.go updates assertions); .agentera/PLAN.md (Task 5 → complete); CHANGELOG.md (Unreleased Changed + Fixed)

## Cycle 57 — 2026-04-19

**Phase**: build
**What**: Task 4 of the TUI ground-up redesign — shared single-column grouped-by-subsystem detail renderer wired into both the Games and Defaults resources. New `internal/tui/detail.go` exports `DetailModel` with `NewDetail(styles, rawGame, defaults)` (game view; resolves values through `profile.ResolveForApply`) and `NewRootDetail(styles, defaults)` (defaults view; renders raw defaults with `isRoot=true`). Renderer walks `detailSectionOrder = [proton, dlss, gpu, cpu, overlay]` per Decision 1; each section emits a bold header line plus one focusable row per field from `profile.SectionFields(section)`. `fieldLabels` maps every key in `profile.AllFields()` to a display label (a test asserts completeness so new fields cannot slip in without an entry). `Update(j/k/up/down)` walks the pre-flattened `focusableRows` slice so the cursor skips group headers transparently; other keys return `handled=false` for parent routing. `View()` renders `label  value` columns with `FocusStyle` (accent-focus cyan, bold) on the focused row. Wiring: `ContentModel` now holds both the legacy `ProfileWidget` (kept for the modal save pipeline Task 5 will re-home) and a new `DetailModel`; `renderProfile()` emits the DetailModel view. `resource_view.go` renders the Defaults resource via a dedicated `renderDefaults()` that drops the Task 3 stub entirely and draws a bordered box around the DetailModel, with `updateDefaults()` feeding j/k into the renderer. `layout_handlers.go`'s `tab` handler now flips `innerFocused=true` when tabbing into Defaults so the pane border picks up accent-focus. `handleProfileSaveMsg` calls `refreshDefaultsDetail()` after every save so the Defaults view reflects freshly-saved defaults without a reload. Help screen adds a new "Profile detail (Games + Defaults)" section documenting the j/k-crosses-groups and tab-transfers-focus contract.

**Commit**: b7ffbe6 feat(tui): shared grouped detail renderer for Games and Defaults resources

**Inspiration**: .agentera/DECISIONS.md Decision 1 (firm) — grouped single-column layout keystone. Built on Task 2's `profile.SectionFields` / `AllFields` primitives rather than introducing a parallel field registry. Preserved the legacy `ProfileWidget` value formatters (`displayBool`, `displayValue`, `displayInt`, `displayBoolPtr`) via `formatFieldValue` so the read-only detail matches the edit-mode widget visually and does not duplicate the per-type display logic.

**Discovered**: `ResolveForApply(nil)` correctly returns an empty profile for any field NOT marked as `Overridden` on the source — which is the wrong behavior for a root defaults profile loaded directly from a struct literal (no `Overrides` map means every field reads as "(default)" despite being set). Fixed by having `buildDetail` do a shallow struct copy in the `isRoot` branch rather than running the resolution helper. The migration path in `storage.Load` populates `Overrides` from legacy YAML so production defaults loaded from disk would have worked through `ResolveForApply`, but the struct-copy path is the correct semantic (a root profile's values ARE the resolved values, by definition). `Task 3`'s stub test `TestRail_RouterRendersStub/Defaults` needed updating since Defaults is no longer a stub; kept it as the DLLs+Metrics case and added a new `TestResourcePane_DefaultsUsesRootDetail` that positively asserts the group headers appear and that no inheritance markers leak into the root rendering. Test count went from 153 functions → 170 (12 new detail tests + render probe + 4 pane tests).

**Verified**: `mage test` → 15 packages PASS. `mage lint` → 0 issues. `mage build` → 16 MB binary rebuilt plus the Wails GUI bundle. Behavioral rendering via `SPELA_RENDER_PROBE=1 go test ./internal/tui/ -run TestRenderProbe_Task4 -v`:

- **Games resource** (120×40 terminal, Cyberpunk 2077 selected with defaults profile setting Wayland=true, VKD3D_heap=true, SR mode=quality, SR preset=K, Power limit=350, Overlay enabled=true, position=top-left; game profile pinning HDR=true as an override): left sidebar renders the games list with "Cyberpunk 2077" highlighted in accent-focus cyan plus "● DLLs ◆ profile" legend; right detail renders the game header (name, AppID, install dir) above the "DLL versions" section ("No DLLs detected") followed by the five grouped subsystems. Proton: HDR=true (focused, cyan+bold — override), Wayland=true (inherited from defaults), NGX updater=(default), VKD3D heap=true (inherited). DLSS: SR mode=quality, SR preset=K (both inherited from defaults), remaining 10 fields=(default). GPU: Power limit=350 (inherited), remaining 7=(default). CPU: 3×(default). Overlay: Enabled=true, Position=top-left (both inherited), remaining 6=(default). Every subsystem header is bold magenta AccentOverride; every field row is present.

- **Defaults resource** (focused via `3` then `tab`): border renders in accent-focus cyan confirming the focus transfer into the pane. Title "Default profile" bold magenta, subtitle "Root profile — fields here feed games by inheritance." in fg-muted. Five groups in canonical order (Proton, DLSS, GPU, CPU, Overlay). Proton row state: HDR=(default) (focused, cyan+bold — HDR was set to false on the defaults struct, which displayBool renders as "(default)" in the Task 4 pre-marker UI), Wayland=true, NGX updater=(default), VKD3D heap=true. DLSS shows SR mode=quality, SR preset=K. GPU shows Power limit=350. Overlay shows Enabled=true, Position=top-left. Critically: no "[inherited]" or "[override]" strings appear anywhere in the output, and no AccentOverride marker glyph (◆ outside the rail) appears — confirming isRoot=true suppresses all markers per acceptance.

- **Focus crosses group boundaries**: with cursor at 0 (HDR), pressing `j` twice advances to NGX updater (cursor=2, focused field=`proton.enable_ngx_updater`). Pressing `j` 5 more times (total 7) advances cursor to 7, focused field=`dlss.sr_override` — confirming j has silently stepped across the Proton→DLSS header boundary without pausing on the header row. The render shows the accent-focus styling land on the correct field row each time, never on a group header.

- `TestDetail_FieldEnumeration_GroupOrder` walks j forward through all 35 fields and asserts each matches `profile.SectionFields(section)` in the canonical order.
- `TestDetail_FieldEnumeration_AllFieldsHaveLabels` asserts every key in `profile.AllFields()` has a non-empty entry in `fieldLabels` (adversarial — ensures adding a new field without a label fails loudly).
- `TestDetail_JKCrossesGroupHeaders` independently verifies 4× j from position 0 lands at the first DLSS field.
- `TestDetail_JKClampsAtEnds` confirms up-from-zero and down-past-end clamp correctly.
- `TestDetail_RootSuppressesMarkers` scans the rendered string for `[inherited]`, `[override]`, and the ◆ glyph and fails if any appear when isRoot=true.
- `TestDetail_GameDetailResolvesInheritance` asserts a game profile with no proton overrides renders HDR=true when defaults set it true.
- `TestDetail_GameDetailOverriddenValue` asserts an overridden false value on the game does NOT render defaults' true.
- `TestDetail_ViewRendersEveryGroupHeader` + `_FailNegative` (positive + negative pair) verify the five real headers appear and forbidden headers (Network, Audio, Physics) do not.
- `TestResourcePane_DefaultsUsesRootDetail` verifies the resource-pane-level wiring renders all five headers and no Task 3 stub text.
- `TestResourcePane_DefaultsJKMovesFieldFocus` verifies j/k through the pane's Update routes to the DetailModel cursor.
- `TestResourcePane_GamesSidebarPlusDetail` verifies the Games resource renders both the game list and the grouped detail headers.
- `TestLayoutHandlers_TabIntoDefaultsMovesInnerFocus` confirms the tab-from-rail-into-Defaults focus-transfer contract.

**Next**: Task 5 (inheritance rendering + r/shift+r/p bindings + DLSS duplicate-preset fix) — now unblocked on Task 4. Task 5 layers muted/bright row styling and the override marker on top of the existing DetailModel (fields already resolve via `ResolveForApply`, so marker rendering just reads `raw.IsOverridden(field)` per row), then wires `r` to `raw.Reset(field) + save`, `shift+r` to `raw.ResetAll() + save`, and `p` to pinning the currently-resolved value. Task 6 (DLLs + Metrics resources) is still gated on Task 3 alone.

**Context**: intent — ship the shared single-column grouped detail renderer so both Games and Defaults resources show real resolved profile values with j/k navigation across groups, laying the structural spine Task 5 decorates with inheritance markers · constraints — no inheritance markers / no r/shift+r/p bindings / no duplicate-preset fix (all Task 5); preserve the legacy ProfileWidget's save pipeline; reuse profile.SectionFields and ResolveForApply from Task 2 rather than introducing a parallel enumeration; every focusable row must correspond to a field present in both detailSectionOrder AND profile.AllFields() · unknowns — none; Decision 1 is firm and Tasks 2/3 primitives are live · scope — internal/tui (new detail.go, detail_test.go, detail_probe_test.go; modifications to content.go, content_views.go, layout.go, layout_handlers.go, resource_view.go, help.go, rail_test.go); .agentera/PLAN.md (Task 4 → complete); CHANGELOG.md (Unreleased addition)

## Cycle 56 — 2026-04-19

**Phase**: build
**What**: Task 3 of the TUI ground-up redesign plan — rewrote the shell around a permanent left rail of four peer resources (Games, DLLs, Defaults, Metrics). New `internal/tui/rail.go` owns the rail state machine (1-4 hotkeys, j/k cursor, enter confirm; hotkeys align cursor and active resource in one stroke). New `internal/tui/resource_view.go` routes the right pane to the active resource. Games resource preserves the existing games sidebar + per-game detail (ContentModel, now single-view after the tab removal); DLLs/Defaults/Metrics render placeholder stubs that Tasks 4-6 will fill in. Removed: `ContentTab` enum, `TabDLLs`/`TabProfile`/`TabLaunch` constants, `activeTab`, `launching` flag, `launchGame()`, `launchGameMsg`, `renderLaunch`, `renderTabBar`, `handleLaunchGameMsg`, and the `internal/launcher` import from the TUI package. Also removed `navstack.go` (unused after the shell rewrite). Default profile moved from the first row of the games sidebar to its own rail resource. `sidebarFocused` renamed to `railFocused` everywhere. Keymap audit: `r` reserved for Task 5 reset-field → rescan games rebound to `ctrl+r`; `p` reserved for Task 5 pin-field → profile filter rebound to `P` (shift+p); `shift+r`, `:` reserved without existing collisions; `1-4` rail hotkeys displace the old jump-to-pane keys. Every displacement documented in the help screen under a new "Displaced bindings (Task 3 keymap audit)" section.
**Commit**: 8b4907f refactor(tui): rewrite shell around four-resource left rail
**Inspiration**: .agentera/DECISIONS.md Decision 1 (firm) — resource-centric spine with launcher removal as the keystone. Also drew on Task 1's `FocusStyle`/`OverrideMarkerStyle` helpers so the rail focus + active-resource glyph consume canonical tokens rather than introducing new ones.
**Discovered**: The old shell's `navstack` was already single-entry in practice (only contentEntry ever got pushed/popped), so removing it simplified `LayoutModel` significantly. The `sidebarItemDefaultProfile` enum and its switch branches are now unreachable; I kept the enum member (and the `defaultProfileSelectedMsg`/`defaultProfileConfirmedMsg` types) as no-op compat surface because ripping them out would ripple into the pending Task 4 work. `options_modal.go` had one "launch" string ("Automatically update DLLs on launch") referring to game start, not the TUI Launch surface — rewrote to "on game start" for clarity. Test count went from 146 functions → 153 (deletions offset by new rail_test.go + smaller new tests).
**Verified**: `mage test` → all 15 packages PASS. `mage lint` → 0 issues. `mage build` → 12 MB binary rebuilt plus GUI dist bundle. Behavioral render probe (SPELA_RENDER_PROBE=1 via a scratch `TestRenderProbe` that simulates a 120x40 terminal and presses 1/2/3/4): captured concrete ANSI-rendered output confirming the shell behavior:

- After pressing `1`: `active=Games railFocused=true cursor=0`; the pane renders the games sidebar with "Cyberpunk 2077" highlighted by the accent-focus cyan SelectionBg plus the legend "● DLLs  ◆ profile", and a right-side placeholder "Select a game from the list".
- After pressing `2`: `active=DLLs railFocused=true cursor=1`; pane renders the DLLs stub ("Library and deployment table render here (Task 6).") under a magenta "DLLs" title.
- After pressing `3`: `active=Defaults railFocused=true cursor=2`; pane renders the Defaults stub ("Default profile renders through the shared detail renderer (Task 4).").
- After pressing `4`: `active=Metrics railFocused=true cursor=3`; pane renders the Metrics stub ("Thermal and sparkline widgets relocate here (Task 6).").

Rail focus is preserved across all four hotkey presses (verified in `TestRail_HotkeyFromLayoutStaysOnRail` and the probe both). `rg -i launch internal/tui/` has zero production-code matches — only two historical-comment references (`layout.go:35` and `content.go:55`, both explicitly documenting what was removed) plus test-file comments. New `rail_test.go` provides state-machine coverage: `TestRail_InitialState`, `TestRail_HotkeySelectsResource` (4 subtests), `TestRail_JKMovesCursor`, `TestRail_EnterConfirmsCursor`, `TestRail_HotkeyFromLayoutStaysOnRail` (4 subtests), `TestRail_RouterRendersStub` (3 subtests), `TestRail_GamesResourceRendersGamesList`. Triage document at `.agentera/work/tui-test-triage.md` categorizes every pre-existing TUI test as keep/rewrite/delete with a one-line reason.
**Next**: Task 4 (Games + Defaults resource scaffold) — now unblocked on both Task 2 (inheritance primitives) and Task 3 (shell). Task 4 fills in the Games resource's single-column grouped-field detail renderer and wires the Defaults resource to the shared renderer. Task 5 (inheritance rendering + r/shift+r/p bindings) depends on Task 4.
**Context**: intent — ship the rail-based shell skeleton so Tasks 4-6 have a stable spine to fill out, with all Launch/Tab surface removed and the keymap audited so Task 5's profile keystrokes have room to land · constraints — no new dependencies; GUI untouched; CLI launch path (`spela %command%`) preserved; pre-existing profile YAML and effective-profile behavior preserved; zero production-code `launch` references inside the TUI · unknowns — none; Decision 1 is firm and Task 1's theme tokens + Task 2's inheritance primitives are both live · scope — internal/tui only (18 files touched: rewrote layout.go, layout_handlers.go, content.go, content_views.go, help.go; added rail.go, resource_view.go, rail_test.go; deleted navstack.go; trimmed sidebar.go; updated options_modal.go for one launch-word hygiene fix; rewrote layout_test.go, content_test.go, help_test.go, sidebar_test.go, testhelpers_test.go; added triage artifact at .agentera/work/tui-test-triage.md)

## Cycle 55 — 2026-04-19

**Phase**: build
**What**: Task 2 of the TUI ground-up redesign plan — live profile inheritance primitives plus inheritance-aware CLI. Added `Overrides map[string]bool` sidecar to `Profile` (yaml: `overrides,omitempty`), 35 canonical field-key constants (one per leaf across proton/dlss/gpu/cpu/overlay), and reflection-driven `fieldAccessor` that walks yaml struct tags so new fields need no extra plumbing beyond a key constant. Primitives: `IsOverridden(field)`, `MarkOverride(field)`, `Reset(field)` (clears the flag AND zeros the backing value so ResolveForApply falls back), `ResetAll()` (nil-safe), `ResolveForApply(defaults)` (inherited → defaults, overridden → pinned, with pointer-safe deep copy for `*bool`), `AllFields()`, `SectionFields()`, `IsValidField()`. Load-time migration runs inside `storage.Load` when `Overrides` is nil: fields matching the current default get stripped (inherited); fields that differ become explicit overrides; critically, zero values that differ from non-zero defaults still count as overrides (e.g. a user-pinned `vkd3d_heap: false` when default is `true` survives migration). `LoadEffective` now returns `p.ResolveForApply(defaults)` so the launcher sees inherited-from-default values at apply time. CLI: every per-subsystem `set` marks overrides after mutating a field; every `show` renders resolved values with `[inherited]`/`[override]` markers via the new `renderField` helper in `cmd/spela/commands/inheritance_render.go`; five new reset verbs — `proton reset`, `dlss reset`, `overlay reset` (plus `gpu profile-reset` and `cpu profile-reset`, using `profile-reset` on the latter two to avoid colliding with the pre-existing `gpu reset` (hardware clocks) / cpu-set verbs) — each accepts both dash and underscore forms of the field name.
**Commit**: (pending) feat(profile): add live inheritance primitives and CLI inheritance output
**Inspiration**: .agentera/DECISIONS.md Decision 1 (firm) — live inheritance keystone. The simplest representation listed in PLAN.md Task 2 ("pointer-per-field") would have rippled through every apply site; instead chose the "Overrides map sidecar + reflection over yaml tags" path because it keeps the existing value-typed struct, lets the map serialize cleanly through yaml.v3, and avoids touching Apply/Save call sites beyond new MarkOverride calls in the CLI `set` handlers.
**Discovered**: `reflect.DeepEqual` on `any` interface values correctly dereferences pointers, which made `fieldValuesEqual` a one-liner rather than needing a per-type switch. The existing `TestProtonSettings_YAMLRoundTrip_VKD3DHeapAbsent` legacy-YAML test kept passing without modification, confirming migration does not alter round-trip semantics for profiles that were never touched. Three of the six `commands/*.go` files (gpu_set, cpu_set, overlay) lost their last `tui.` reference after `renderField` absorbed the styling, so the `internal/tui` import was dropped; lint caught this and gofumpt reformatted both files cleanly.
**Verified**: `mage test` → all 15 packages PASS. `mage lint` → 0 issues. `mage build` → binary rebuilt. New `internal/profile/inheritance_test.go` → 11 tests (1 pass + 1 fail per primitive for `IsOverridden`, `Reset`, `ResetAll`, `ResolveForApply`, plus `ResolveForApply_ZeroValueOverride` covering the zero-value-that-differs case, plus two load-time migration tests covering legacy-YAML migration and already-migrated-skip, plus YAML round-trip) → all 11 PASS in 0.002s. End-to-end behavioral verification against seeded `XDG_CONFIG_HOME=/tmp/spela-verify3/xdg-config XDG_DATA_HOME=/tmp/spela-verify3/xdg-data`:

- Legacy YAML (no `overrides:` key) with `enable_hdr: true` matching default, `enable_wayland: true` differing from default `false`, `vkd3d_heap: true` differing from default `false` → `proton show 1091500` rendered `HDR: true [inherited]`, `Wayland: true [override]`, `NGX updater: false [inherited]`, `VKD3D heap: true [override]` — migration correctly split matching from differing fields.
- `proton reset 1091500 vkd3d_heap` → stdout `Reset vkd3d_heap on Cyberpunk 2077 to inherited.` Subsequent `proton show` rendered `VKD3D heap: false [inherited]`. Saved profile YAML collapsed to `name: Cyberpunk 2077` + `proton.enable_wayland: true` + `overrides: proton.enable_wayland: true` — migration was one-way and persistent.
- `dlss set --sr-mode=quality` then `dlss show` → `Mode: quality [override]`, `Override: true [override]`, every other field `[inherited]`. `dlss reset sr_mode` → `Mode: [inherited]` on reshow.
- `gpu set --power-limit=400` → `Power limit: 400 [override]`; `gpu profile-reset power_limit` → `Power limit: (default) [inherited]`.
- `launch --dry-run 1091500` emitted `PROTON_ENABLE_HDR=1` (inherited from default) AND `PROTON_ENABLE_WAYLAND=1` (pinned override) AND `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE=on` (inherited through resolved), confirming `LoadEffective → ResolveForApply` threads through the launcher.
**Next**: Task 3 (shell redesign with left rail, keymap audit, test triage) — now unblocked on both Task 1 (theme) and Task 2 (inheritance primitives). Task 4 is gated on Task 3.
**Context**: intent — make live inheritance the data model so Tasks 4-5 can render muted/bright rows by consulting `IsOverridden` and reset via `Reset` without further primitive work · constraints — existing profile YAMLs must continue to apply with identical semantics; zero-value overrides must survive migration; no GUI/TUI behavior changed in this cycle (TUI continues to render via its existing path) · unknowns — none; Decision 1 is firm · scope — profile package (new inheritance.go + test file, profile.go struct extension, storage.go migration + LoadEffective resolve), commands package (6 files: proton.go, dlss.go, gpu_set.go, cpu_set.go, overlay.go for set+show+reset wiring; new inheritance_render.go helper), zero GUI/TUI touch

## Cycle 54 — 2026-04-19

**Phase**: build
**What**: Task 1 of the TUI ground-up redesign plan — collapsed the Default/Dark/Light theme triad into a single neon-accent dark palette keyed off six canonical tokens (`Bg` #0a0a14, `Fg` #e8e8f0, `FgMuted` #6a6a80, `AccentOverride` #ff5fd2 magenta, `AccentFocus` #5ff0ff cyan, `Border` #202030) per .agentera/DECISIONS.md Decision 1. Kept the legacy aliases (Primary/Secondary/Accent/Text/TextDim/Background/BorderFocus/SurfaceBase/…) pointing at the new tokens so header/sparkline/help/messagebar/modal consumers keep rendering while Tasks 3-6 migrate to canonical names. Added three theme helper methods on `Styles` — `InheritedStyle()` (FgMuted for inherited rows), `OverrideStyle()`+`OverrideMarkerStyle()` (Fg text plus AccentOverride marker), `FocusStyle()` (AccentFocus + bold) — which Task 5 consumes for inheritance rendering without extending the token set. `NewLayout` now ignores any `cfg.Theme` value, always uses `DefaultTheme`, and blanks `cfg.Theme` at load so subsequent saves do not re-persist `theme: dark` / `theme: light`. Dropped the Theme option from the TUI options modal and its getConfigValue/setConfigValue cases. CLI helpers (CLIPrimary/CLISecondary/CLIDim/CLISuccess/CLIAccent) rewired through canonical tokens — CLIPrimary now renders magenta (#ff5fd2 = AccentOverride), CLISecondary cyan (#5ff0ff = AccentFocus), CLIDim grey (#6a6a80 = FgMuted). Left `config.Config.Theme` field intact because the GUI still reads/writes it; the TUI simply neither reads nor writes it anymore.
**Commit**: 96b907b refactor(tui): collapse themes to single neon-accent dark palette
**Inspiration**: .agentera/DECISIONS.md Decision 1 (firm) — magenta=override, cyan=focus semantics drove the token naming; kept one hex per token by centralizing the six values in package-level `hex*` constants so no component reintroduces literals.
**Discovered**: GUI still consumes `config.Config.Theme` via `parseTheme` for its own `data-theme` CSS attribute, so removing the field entirely was out of scope. Stripping on TUI load (not on config.Save) is the narrowest contract that satisfies "does not re-persist the legacy value" without breaking the GUI. `lipgloss.Color("#ff5fd2")` is the supported v2 syntax — no other color construction needed.
**Verified**: `mage test` → all 15 packages PASS. `mage lint` → 0 issues. `mage build` → 12MB binary built. New tests (`go test ./internal/tui/ -run 'TestInheritedStyle|TestOverrideStyle|TestOverrideMarkerStyle|TestFocusStyle|TestTheme_|TestLayout_StripsLegacyTheme|TestCLIHelpers_NoLegacyColors' -v`) → 15 subtests PASS in 0.003s. Explicit observed output per helper:

- `CLIPrimary("PRIMARY")` rendered `"\x1b[38;2;255;95;210mPRIMARY\x1b[m"` — RGB 255,95,210 = #ff5fd2 = AccentOverride (magenta)
- `CLISecondary("SECONDARY")` rendered `"\x1b[38;2;95;240;255mSECONDARY\x1b[m"` — RGB 95,240,255 = #5ff0ff = AccentFocus (cyan)
- `CLIDim("DIM")` rendered `"\x1b[38;2;106;106;128mDIM\x1b[m"` — RGB 106,106,128 = #6a6a80 = FgMuted
- `CLISuccess("SUCCESS")` rendered `"\x1b[38;2;124;227;139mSUCCESS\x1b[m"` — #7ce38b = Success
- `CLIAccent("ACCENT")` rendered `"\x1b[38;2;255;95;210mACCENT\x1b[m"` — magenta AccentOverride
- `TestTheme_NeonPaletteTokens/Bg,Fg,FgMuted,AccentOverride,AccentFocus,Border` → six PASS asserting exact RGB match against Decision 1 spec
- `TestLayout_StripsLegacyTheme/dark,light,default,royal-blue` → four PASS confirming `NewLayout` blanks `config.Theme` regardless of input value and resolves to `neon-accent-dark`
- `TestCLIHelpers_NoLegacyColors_Smoke` → PASS confirming none of the legacy ANSI indices (133 Amethyst / 69 Royal Blue / 91 Velvet Orchid / 212 Pink Carnation) survived in CLI helper output
- `Grep` for `Amethyst|Royal Blue|Velvet Orchid|Pink Carnation|DarkTheme|LightTheme` outside tests/comments → zero production-code hits
**Next**: Task 2 (profile inheritance primitives + CLI inheritance output) — independent of this task; can start immediately. Task 3 (shell redesign with left rail) depends on this task and is now unblocked on the theme side.
**Context**: intent — lay down the token substrate Tasks 3-5 will render through, with inheritance (FgMuted), override marker (AccentOverride), and focus (AccentFocus) distinguishable from the start · constraints — GUI untouched per PLAN.md scope; legacy aliases preserved so existing TUI consumers keep rendering; config.Config.Theme field preserved for GUI · unknowns — none; Decision 1 is firm · scope — styles.go (rewritten, canonical tokens + aliases + helpers), layout.go (drop theme switch, strip legacy), options_modal.go (drop theme option + getter/setter cases), thermal_test.go (allThemes → single), styles_test.go (new, 11 helper tests), cli_smoke_test.go (new, legacy-color regression guard)

## Plan Summary — DX12 descriptor_heap — 2026-04-19

- **Plan**: DX12 descriptor_heap per-game toggle (6 tasks, completed 2026-04-19)
- **Delivered**: Users can enable `vkd3d_heap` per game from the CLI (`spela proton set --vkd3d-heap=<bool>`, visible in `spela proton show` formatted + `--json`) or the TUI profile widget (new `VKD3D heap` field in the Proton group). When enabled, spela emits `PROTON_VKD3D_HEAP=1` and `VKD3D_CONFIG=descriptor_heap` at launch. Toggle-time compatibility notices appear inline in both CLI and TUI when the resolved Proton build lacks the `PROTON_VKD3D_HEAP` marker or the NVIDIA driver is below 580.94.16. Launch-time `slog.Warn` preflight records structured `detected`/`minimum` fields per axis when requirements are unmet; launch is never blocked. Resolver error and missing-driver cases degrade to `slog.Info` skips rather than warnings.
- **Shipped in**: v0.4.0 (tag `ee7adfb`)
- **Cycles**: 50 (profile field + env emission), 51 (internal/proton resolver package: `ResolveForAppID`, `SupportsVKD3DHeap`, `ParseDriverVersion`, requirements constants), 52 (CLI + TUI surface with `CompatibilityNotice` helper), 53 (launcher preflight via `CheckCompatibility` + injectable `VKD3DCompatibilityCheck`)
- **Inspiration source**: inspirera analysis of `VK_EXT_descriptor_heap` (Vulkan 1.4.340, Jan 2026), Proton-CachyOS 10.0-20260321+, and the NVIDIA 580+ driver ecosystem — the plan was framed to turn fragmented Reddit/forum guidance into a first-class, honest toggle in spela
- **Follow-ups** (candidates for TODO filing):
  - GUI parity for `vkd3d_heap` toggle (currently a UX gap — CLI/TUI only)
  - Promote `gpu.DriverVersionString()` to a first-class public NVML driver-version API (minimal surface added in Cycle 52; worth rounding out)
  - Retrofit `parseBoolFlag` (introduced in Cycle 52 for `--vkd3d-heap`) to older proton flags `--hdr`, `--wayland`, `--ngx-updater` so typos stop silently falling through to false
  - Watch vkd3d-proton PR #2943 upstream merge → revisit `MinProtonCachyOSBuild` and `MinDriverVersion` constants in `internal/proton/requirements.go`
  - Consider flipping `vkd3d_heap` default-on once upstream vkd3d-proton stabilizes and NVIDIA 580.94.16+ is broadly shipped

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
