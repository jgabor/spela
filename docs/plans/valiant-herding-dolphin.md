# Wire up inert Settings options, then expose them in the TUI and GUI

## Context

Agentera TODO `hpuwnhwspr` (TUI/GUI Settings parity). Step 1 already hid empty
TUI groups. What remains is parity: the TUI shows 2 options
(`rescan_on_startup`, `steam_path`; `show_hints` is a retired key) and the GUI
shows 13. Research found that 11 of the GUI's options are read by no runtime
code. The user chose to wire them up first, then expose them everywhere. This
plan is for a new Agentera plan (full level), executed in phases. Each phase
is one build cycle and ends with the option working and visible on both
interfaces. No commits or pushes without the user's say.

## Findings that shape the plan

- `logging` has no level control (`internal/logging/logging.go`); CLI `main`,
  wrapper mode and the GUI never load config at startup.
- DLL cache and backup both funnel through two or one helpers
  (`dll/download.go:18,22`, `dll/swap.go:35`). `backup.json` stores absolute
  backup paths (`swap.go:24,30`), so changing `backup_path` hides old backups.
- The manifest freshness check uses the manifest's publish date, not the fetch
  time (`dll/manifest.go:133`), so it hits the network on every call once the
  published manifest is a day old. A refresh interval cannot work until fixed.
- `preferred_dll_source` has no second source in the running app; TechPowerUp
  exists only in the offline generator (`tools/dlss-updater`).
- `check_updates` has no code at all (no release lookup, no notice).
- `auto_update_dlls`: `dll.UpdateGames` runs headless, with deny list, backup,
  and checksum safeguards, but it downloads and swaps game files unattended.
- `confirm_destructive`: the TUI always confirms; the GUI never confirms DLL
  update or restore, so the option text is false today. Honouring `false` in
  the TUI would skip the restore "backup changed" re-check.
- `theme` works in the GUI only. `compact_mode` is read nowhere; the TUI has an
  unrelated density mode.
- UI gaps: no TUI editor for `KindStringList`; the GUI renders a list option as
  an empty select (`internal/config/wails.go:32-41`).

## Phases (in order)

1. **Foundation and logging.** Add `logging.SetLevel`; load config and apply
   `log_level` in a cobra `PersistentPreRunE`, in wrapper mode, in TUI and GUI
   startup. Add `VisibilityTUI` to `log_level`. Fix the config-to-UI wiring
   needed by later phases: TUI list editor and GUI list widget for
   `additional_library_paths` (and expose it on both; it already has a reader).
2. **Paths.** `dll_cache_path`: honour in `GetDLLCachePath` and
   `GetDLLCacheDir` (manifest cache stays put). `backup_path`: honour in
   `GetBackupDir` for new backup files only. The backup index
   (`backup.json`) always stays in the default data folder and records
   absolute file paths, so earlier backups stay restorable after any number
   of path changes. (Changed during build from "fall back to the default
   location": that fallback lost backups made under an earlier custom path.)
   Fix its description (no "save backups").
   Global `shader_cache`: pass the config value into `profile.Apply` and
   `ApplyEnv` as the fallback before the xdg default, empty falls back too.
   Expose all three on TUI and GUI.
3. **Manifest.** Fix the freshness bug by recording the fetch time. Honour
   `auto_refresh_manifest`, `manifest_refresh_hours` and `dll_manifest_url`
   through one resolver used by the CLI, TUI and GUI call sites. Expose on
   both. `preferred_dll_source`: see decision 1.
4. **Update check.** Implement `check_updates`: a release lookup with a short
   timeout and stored last-check time, skipped for `dev` builds, reusing
   `dll.CompareVersions`, with a non-blocking notice in the TUI header and a
   GUI toast. Expose on both.
5. **Auto-update DLLs.** At TUI `Init` and GUI `startup`, run the update path
   when enabled; see decision 2 for the safety rules.
6. **Behaviour options.** `confirm_destructive` and `compact_mode`: see
   decisions 3 and 4. `theme` stays GUI-only with a stated reason.
7. **Close out.** Update docs (`docs/configuration.md`, navigation contract),
   resolve TODO `hpuwnhwspr` with the per-option reasons, add a short "settings
   parity" table.

## Decisions I need from you (my recommendations)

1. **`preferred_dll_source`** has one source today. Recommend retiring the
   option (as `show_hints` was) rather than building a second source.
2. **`auto_update_dlls`** changes game files unattended. Recommend: default
   off (as now), skip a game whose process is running, never touch deny-listed
   games, and show a result notice. Alternative: update only DLLs already in
   the cache.
3. **`confirm_destructive`.** Recommend: GUI gets real confirmations for DLL
   update and restore; the TUI keeps its checks and honours `false` only for
   update, never for restore.
4. **`compact_mode`.** Recommend: map it to the TUI's existing compact density
   and add compact CSS in the GUI. Alternative: retire it.

## Tests to change

`internal/config/options_test.go:19-34,126-135` (visibility),
`internal/tui/options_modal_test.go:47` and the group-0 assumptions at
`:58,116,139,212`, `internal/gui/app_supported_test.go:133-144`, GUI fixtures
`frontend/e2e/fixtures.js`, `tests/e2e/tui_journeys_test.go:328`. New tests per
option: one pass and one fail (path override, missing path, stale manifest,
no network, dev build).

## Verification

`mage test`, `mage lint`, `mage build`; frontend `bun run test` in a scratch
checkout; `go test -tags e2e ./tests/e2e/...`. TUI checks with termctrl 1.2.1
in isolated XDG state at 120x40 and 80x24, using fixture games only (no real
DLLs, no network beyond a local stub server). Verify backup restore still
works after changing `backup_path`.
