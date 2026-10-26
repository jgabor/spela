# Global configuration contract

Spela loads `$XDG_CONFIG_HOME/spela/config.yaml` over `config.Default()`. Missing
keys retain their defaults; explicit `false`, `0`, and empty string values are
retained. For compatibility, an explicit empty `additional_library_paths` list
is omitted by `omitempty` and loads as nil rather than remaining distinct from
nil. `config show` prints every YAML field. The **CLI** column below means
editable through `config set`, and the **TUI** column means a visible Settings
control.

| Go field | YAML key | Default | Accepted value | Section | CLI | TUI | Effect |
|---|---|---|---|---|:---:|:---:|---|
| `LogLevel` | `log_level` | `info` | `debug`, `info`, `warn`, `error` | Logging | yes | yes | Applied at startup and after each TUI save. |
| `ShaderCache` | `shader_cache` | `$XDG_CACHE_HOME/spela/nvidia` | absolute folder or empty | Paths | yes | yes | Shader cache folder for a profile that turns Shader cache on and leaves its own path empty. Empty or relative uses the default. Used by the next launch. |
| `CheckUpdates` | `check_updates` | `true` | boolean (`1`/`0` also accepted by CLI) | Startup | yes | yes | When the TUI starts, asks GitHub at most once a day for the newest `vX.Y.Z` tag and shows a notice when it is newer (TUI header). Every attempt, including a failed one, is recorded in `$XDG_CACHE_HOME/spela/update-check.json`. Builds whose version is not a plain `X.Y.Z` (development, `git describe`, AUR `-git`) never ask. |
| `RescanOnStartup` | `rescan_on_startup` | `true` | boolean | Startup | no | yes | Used on the next startup. |
| `AutoUpdateDLLs` | `auto_update_dlls` | `false` | boolean | Startup | no | yes | When the TUI starts (after a startup scan), updates each DLL that is older than the manifest's latest version, with the usual backup and checksum checks. Skips deny-listed games, games that look running (a process in or naming the install folder, or a Steam launch for the app ID), and DLLs whose version is unknown. Shows a notice only when something was updated, failed, or skipped while running. |
| `SteamPath` | `steam_path` | empty (auto-detect) | path/string | Paths | no | yes | Used by subsequent Steam lookups and launches. |
| `AdditionalLibraryPaths` | `additional_library_paths` | omitted/nil | string list; edited as colon-separated text | Paths | no | yes | Used by subsequent Steam scans. |
| `DLLCachePath` | `dll_cache_path` | empty (`$XDG_CACHE_HOME/spela/dlls`) | absolute folder or empty | Paths | no | yes | Used by subsequent DLL downloads and cache lookups. Earlier downloads are not moved. The manifest cache stays in the XDG cache. |
| `BackupPath` | `backup_path` | empty (`$XDG_DATA_HOME/spela/backups`) | absolute folder or empty | Paths | no | yes | Receives new DLL backup files. The backup index stays in `$XDG_DATA_HOME/spela/backups/<appid>/backup.json` with absolute file paths, so earlier backups stay restorable after a change. |
| `DLLManifestURL` | `dll_manifest_url` | empty (built-in endpoint) | https URL, http only to this machine, or empty | DLL policy | no | yes | Source of the DLL list and checksums. A changed URL is fetched at next manifest use, even with auto-refresh off. A stored URL that fails the check is ignored and the built-in endpoint applies. |
| `AutoRefreshManifest` | `auto_refresh_manifest` | `true` | boolean | DLL policy | no | yes | On fetches a new manifest when the cached one is older than the refresh interval. Off keeps the cached manifest at any age; Spela still fetches when nothing is cached or the URL changed. |
| `ManifestRefreshHours` | `manifest_refresh_hours` | `24` | integer; UI choices `1`, `6`, `12`, `24`, `48`, `168` | DLL policy | no | yes | Age, measured from the last fetch, at which the cached manifest counts as old. An explicit `0` or negative value is kept in the file and treated as `24`. |
| `Theme` | `theme` | `default` | `default`, `dark`, `light` | Display | no | no | Kept in the file and the option catalog, but no interface shows it now that the GUI is gone: the TUI draws with the terminal's own colours. |
| `CompactMode` | `compact_mode` | `false` | boolean | Display | no | yes | The TUI starts in the Compact layout and switches when the setting changes; Actions still pick a layout for the session. |
| `ConfirmDestructive` | `confirm_destructive` | `true` | boolean | Display | no | yes | On asks before every DLL update (TUI game, catalog and selected-game updates). Off starts updates at once. Restore and install always ask, whatever the setting, so a restore keeps its backup re-check. Takes effect at once. |

## Settings groups

The TUI's Settings show the options in these groups, in this order.
`internal/config/docs_contract_test.go` checks the **CLI** and **TUI** columns
above against the option catalog.

| Group | Options |
|---|---|
| Display | Compact mode, Confirm destructive actions |
| Startup | Re-scan on startup, Auto-update DLLs, Check for updates |
| Paths | Steam path, Additional library paths, DLL cache path, Backup path, Default shader cache path |
| DLL policy | Auto-refresh manifest, Refresh interval, DLL manifest URL |
| Logging | Log level |

Settings opens on Display. Two keys are retired and appear in no interface:
`show_hints` and `preferred_dll_source`.

Folder options (`shader_cache`, `dll_cache_path`, `backup_path`) reject a
relative path or `~` when edited. A relative value already in `config.yaml`
is ignored at use time and the default folder applies.

The manifest cache (`$XDG_CACHE_HOME/spela/manifest.json`) records when and
from which URL it was fetched. A cache written by an older Spela has neither;
it counts as fetched from the built-in endpoint at its file time.

`preferred_dll_source` is retired: Spela has one DLL source. The key in an
existing `config.yaml` is ignored and dropped on the next save.
