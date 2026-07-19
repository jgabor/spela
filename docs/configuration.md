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
| `LogLevel` | `log_level` | `logLevel` | `info` | `debug`, `info`, `warn`, `error` | Logging | yes | yes | yes | Used when logging is initialized. |
| `ShaderCache` | `shader_cache` | `shaderCache` | `$XDG_CACHE_HOME/spela/nvidia` | path/string, including empty | Paths | yes | no | no | Classified legacy global cache path; no settings control or live effect. |
| `CheckUpdates` | `check_updates` | `checkUpdates` | `true` | boolean (`1`/`0` also accepted by CLI) | Startup | yes | yes | yes | Used by startup update checks. |
| `ShowHints` | `show_hints` | `showHints` | `true` | boolean | Display | no | yes | yes | TUI hint rendering changes immediately, before save. |
| `RescanOnStartup` | `rescan_on_startup` | `rescanOnStartup` | `true` | boolean | Startup | no | yes | yes | Used on the next startup. |
| `AutoUpdateDLLs` | `auto_update_dlls` | `autoUpdateDLLs` | `false` | boolean | Startup | no | yes | yes | Startup policy; no live settings effect. |
| `SteamPath` | `steam_path` | `steamPath` | empty (auto-detect) | path/string | Paths | no | yes | yes | Used by subsequent Steam lookups and launches. |
| `AdditionalLibraryPaths` | `additional_library_paths` | `additionalLibraryPaths` | omitted/nil | string list | Paths | no | no | no | Wails/YAML compatibility field; no settings control. |
| `DLLCachePath` | `dll_cache_path` | `dllCachePath` | empty (XDG cache) | path/string | Paths | no | yes | yes | Used by subsequent DLL cache operations. |
| `BackupPath` | `backup_path` | `backupPath` | empty (XDG data) | path/string | Paths | no | yes | yes | Used by subsequent backup operations. |
| `DLLManifestURL` | `dll_manifest_url` | `dllManifestURL` | empty (built-in endpoint) | URL/string | DLL policy | no | no | no | Wails/YAML compatibility field; no settings control. |
| `AutoRefreshManifest` | `auto_refresh_manifest` | `autoRefreshManifest` | `true` | boolean | DLL policy | no | yes | yes | Used by subsequent manifest operations. |
| `ManifestRefreshHours` | `manifest_refresh_hours` | `manifestRefreshHours` | `24` | integer; UI choices `1`, `6`, `12`, `24`, `48`, `168` | DLL policy | no | yes | yes | Used by subsequent manifest refresh checks; explicit `0` is valid. |
| `PreferredDLLSource` | `preferred_dll_source` | `preferredDLLSource` | `techpowerup` | `techpowerup`, `github` | DLL policy | no | yes | yes | Used by subsequent DLL downloads. |
| `Theme` | `theme` | `theme` | `default` | `default`, `dark`, `light` | Display | no | yes | yes | GUI theme changes immediately; all three historically accepted values remain catalogued. |
| `CompactMode` | `compact_mode` | `compactMode` | `false` | boolean | Display | no | yes | yes | Persisted display preference; no additional live adapter action. |
| `ConfirmDestructive` | `confirm_destructive` | `confirmDestructive` | `true` | boolean | Display | no | yes | yes | Used when destructive-action presentation is initialized. |

`shader_cache`, `additional_library_paths`, and `dll_manifest_url` are fully
classified contract fields, not deprecated fields. Their lack of new TUI/GUI
controls is intentional compatibility behavior.
