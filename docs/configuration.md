# Global configuration contract

Spela loads `$XDG_CONFIG_HOME/spela/config.yaml` over `config.Default()`. Missing
keys retain their defaults; explicit `false`, `0`, and empty string values are
retained. For compatibility, an explicit empty `additional_library_paths` list
is omitted by `omitempty` and loads as nil rather than remaining distinct from
nil. `config show` prints every YAML field. The **CLI** column below means
editable through `config set`; Wails transports every field, while the **TUI**
and **GUI** columns describe visible settings controls.

| Go field | YAML key | Wails JSON key | Default | Accepted value | Section | CLI | TUI | GUI | Effect |
|---|---|---|---|---|---|:---:|:---:|:---:|---|
| `LogLevel` | `log_level` | `logLevel` | `info` | `debug`, `info`, `warn`, `error` | Logging | yes | yes | yes | Applied at startup and after each TUI or GUI save. |
| `ShaderCache` | `shader_cache` | `shaderCache` | `$XDG_CACHE_HOME/spela/nvidia` | absolute folder or empty | Paths | yes | yes | yes | Shader cache folder for a profile that turns Shader cache on and leaves its own path empty. Empty or relative uses the default. Used by the next launch. |
| `CheckUpdates` | `check_updates` | `checkUpdates` | `true` | boolean (`1`/`0` also accepted by CLI) | Startup | yes | yes | yes | When the TUI or GUI starts, asks GitHub at most once a day for the newest `vX.Y.Z` tag and shows a notice when it is newer (TUI header, GUI notice). Every attempt, including a failed one, is recorded in `$XDG_CACHE_HOME/spela/update-check.json`. Builds whose version is not a plain `X.Y.Z` (development, `git describe`, AUR `-git`) never ask. |
| `RescanOnStartup` | `rescan_on_startup` | `rescanOnStartup` | `true` | boolean | Startup | no | yes | yes | Used on the next startup. |
| `AutoUpdateDLLs` | `auto_update_dlls` | `autoUpdateDLLs` | `false` | boolean | Startup | no | yes | yes | When the TUI or GUI starts (in the TUI after a startup scan), updates each DLL that is older than the manifest's latest version, with the usual backup and checksum checks. Skips deny-listed games, games that look running (a process in or naming the install folder, or a Steam launch for the app ID), and DLLs whose version is unknown. Updates nothing, and says so, when the deny list or its overrides exist but cannot be read. Shows a notice only when something was updated, failed, skipped while running, or skipped for an unreadable deny list. |
| `SteamPath` | `steam_path` | `steamPath` | empty (auto-detect) | path/string | Paths | no | yes | yes | Used by subsequent Steam lookups and launches. |
| `AdditionalLibraryPaths` | `additional_library_paths` | `additionalLibraryPaths` | omitted/nil | string list; edited as colon-separated text | Paths | no | yes | yes | Used by subsequent Steam scans. |
| `DLLCachePath` | `dll_cache_path` | `dllCachePath` | empty (`$XDG_CACHE_HOME/spela/dlls`) | absolute folder or empty | Paths | no | yes | yes | Used by subsequent DLL downloads and cache lookups. Earlier downloads are not moved. The manifest cache stays in the XDG cache. |
| `BackupPath` | `backup_path` | `backupPath` | empty (`$XDG_DATA_HOME/spela/backups`) | absolute folder or empty | Paths | no | yes | yes | Receives new DLL backup files. The backup index stays in `$XDG_DATA_HOME/spela/backups/<appid>/backup.json` with absolute file paths, so earlier backups stay restorable after a change. |
| `DLLManifestURL` | `dll_manifest_url` | `dllManifestURL` | empty (built-in endpoint) | https URL, http only to this machine, or empty | DLL policy | no | yes | yes | Source of the DLL list and checksums. A changed URL is fetched at next manifest use, even with auto-refresh off. A stored URL that fails the check is ignored and the built-in endpoint applies. |
| `AutoRefreshManifest` | `auto_refresh_manifest` | `autoRefreshManifest` | `true` | boolean | DLL policy | no | yes | yes | On fetches a new manifest when the cached one is older than the refresh interval. Off keeps the cached manifest at any age; Spela still fetches when nothing is cached or the URL changed. |
| `ManifestRefreshHours` | `manifest_refresh_hours` | `manifestRefreshHours` | `24` | integer; UI choices `1`, `6`, `12`, `24`, `48`, `168` | DLL policy | no | yes | yes | Age, measured from the last fetch, at which the cached manifest counts as old. An explicit `0` or negative value is kept in the file and treated as `24`. |
| `Theme` | `theme` | `theme` | `default` | `default`, `dark`, `light` | Display | no | no | yes | GUI theme changes immediately; all three historically accepted values remain catalogued. GUI only: the TUI draws with the terminal's own colours. |
| `CompactMode` | `compact_mode` | `compactMode` | `false` | boolean | Display | no | yes | yes | The TUI starts in the Compact layout and switches when the setting changes; Actions still pick a layout for the session. The GUI shrinks its spacing and text by an eighth at once. |
| `ConfirmDestructive` | `confirm_destructive` | `confirmDestructive` | `true` | boolean | Display | no | yes | yes | On asks before every DLL update (TUI game, catalog and selected-game updates; GUI Update all DLLs). Off starts updates at once. Restore and install always ask, whatever the setting, so a restore keeps its backup re-check. Takes effect at once. |

## Settings parity

The TUI and GUI Settings show the same options in the same groups, in this
order, with one exception. `internal/config/docs_contract_test.go` checks the
**CLI**, **TUI**, and **GUI** columns above and this table against the option
catalog.

| Group | In the TUI and GUI | GUI only, and why |
|---|---|---|
| Display | Compact mode, Confirm destructive actions | Theme: the TUI draws with the terminal's own colours. |
| Startup | Re-scan on startup, Auto-update DLLs, Check for updates | |
| Paths | Steam path, Additional library paths, DLL cache path, Backup path, Default shader cache path | |
| DLL policy | Auto-refresh manifest, Refresh interval, DLL manifest URL | |
| Logging | Log level | |

Settings opens on Display. A group without an option for that interface is not
shown; today every group has at least one in both. Two keys are retired and
appear in neither: `show_hints` and `preferred_dll_source`.

Folder options (`shader_cache`, `dll_cache_path`, `backup_path`) reject a
relative path or `~` when edited. A relative value already in `config.yaml`
is ignored at use time and the default folder applies.

The manifest cache (`$XDG_CACHE_HOME/spela/manifest.json`) records when and
from which URL it was fetched. A cache written by an older Spela has neither;
it counts as fetched from the built-in endpoint at its file time.

`preferred_dll_source` is retired: Spela has one DLL source. The key in an
existing `config.yaml` is ignored and dropped on the next save.
