# Profile field contract

`internal/profile.Fields` is the bounded domain contract for every persisted
leaf below `Profile`. `Profile` remains typed and YAML remains grouped by
`dlss`, `gpu`, `cpu`, `proton`, and `overlay`. The table order is the descriptor
order within each subsystem; CLI, TUI, and GUI retain their own layout.

CLI reset names are the YAML leaf with either `_` or `-`. Historical aliases
remain: Proton accepts `hdr`, `wayland`, and `ngx-updater`; DLSS accepts `fg`;
GPU accepts `threaded-opt`. Set-command flags and the deprecated
`--sr-model-preset` flag are unchanged.

| Field / YAML storage | Type | Shared label | Allowed values (when bounded) | Impact | Restore | Current surfaces |
|---|---|---|---|---|---|---|
| `proton.enable_hdr` | bool | HDR | true, false | compatibility | ephemeral launch | CLI/TUI/GUI |
| `proton.enable_wayland` | bool | Wayland | true, false | compatibility | ephemeral launch | CLI/TUI/GUI |
| `proton.enable_ngx_updater` | bool | NGX updater | true, false | compatibility | ephemeral launch | CLI/TUI/GUI |
| `proton.vkd3d_heap` | bool | VKD3D heap | true, false | compatibility | ephemeral launch | CLI/TUI/GUI |
| `dlss.sr_mode` | string | SR mode | unset, off, ultra_performance, performance, balanced, quality, dlaa | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.sr_preset` | string | SR preset | unset, default, auto, A-F, J-M | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.sr_override` | bool | SR override | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.rr_mode` | string | RR mode | DLSS modes | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.rr_preset` | string | RR preset | unset, default, A-F, J-M | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.rr_override` | bool | RR override | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.fg_enabled` | bool | FG enabled | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.fg_override` | bool | FG override | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.multi_frame` | int | Multi-frame | 0-4 | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.indicator` | bool | SR indicator | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.fg_indicator` | bool | FG indicator | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `gpu.clock_offset` | int | Clock offset | unbounded | system state | restorable mutation | CLI/TUI/GUI |
| `gpu.memory_offset` | int | Memory offset | unbounded | system state | restorable mutation | CLI/TUI/GUI |
| `gpu.power_limit` | int | Power limit | unbounded | system state | restorable mutation | CLI/TUI |
| `gpu.fan_speed` | int | Fan speed | unbounded | system state | restorable mutation | CLI/TUI |
| `gpu.power_mizer` | string | Power mode | unset, adaptive, max, legacy prefer-max spellings | system state | not applicable | CLI/TUI/GUI |
| `gpu.shader_cache` | bool | Shader cache | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `gpu.shader_cache_path` | string | Shader cache path | unbounded | environment | ephemeral launch | CLI/TUI/GUI |
| `gpu.threaded_optimization` | bool | Threaded opt | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `cpu.governor` | string | Governor | performance, powersave, schedutil, ondemand | system state | restorable mutation | CLI/TUI/GUI |
| `cpu.smt` | optional bool | SMT | unset, true, false | system state | restorable mutation | CLI/TUI/GUI |
| `cpu.affinity` | string | Affinity | unbounded | system state | not applicable | CLI/TUI |
| `overlay.enabled` | bool | Enabled | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.position` | string | Position | top-left, top-right, bottom-left, bottom-right | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_fps` | bool | Show FPS | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_frametime` | bool | Show frametime | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_cpu` | bool | Show CPU | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_gpu` | bool | Show GPU | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_vram` | bool | Show VRAM | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.toggle_key` | string | Toggle key | unbounded | overlay | not applicable | CLI/TUI/GUI |

## Mutation semantics

- **set** validates and writes the supplied primitive exactly and marks the
  leaf in `Overrides`; false, zero, empty string, and nil SMT are explicit
  values. Bounded descriptor values are the validation source for CLI, TUI,
  and GUI mutations.
- **pin** copies the currently effective value from live defaults into the raw
  game profile and marks only that leaf. It is invalid for the defaults root.
- **reset** zeros only the named raw leaf and removes only its override marker,
  so the live default becomes effective immediately.
- A GUI Save sends one ordered `[]ProfilePatch` batch. The backend serializes
  profile transactions, loads raw storage once, applies every patch to an
  independent copy, saves once only if all patches succeed, and returns a
  refreshed projection. A later invalid patch therefore cannot partially save.
- The GUI shows **Pin** for inherited game fields, **Reset** for overrides, and
  **Unset** for defaults. Editing a control always queues `set`, including zero,
  false, empty string, and unset SMT. These explicit values use literal labels
  such as `0 MHz`, `No mode`, and `No SMT value`, never `(default)`. Actions
  queue explicit `pin` or `reset`, toggle off when selected again, and are
  cleared when that field is edited so the last interaction wins. Frame
  generation treats `fg_enabled` and `fg_override` as one inheritance action.
  The target scope and AppID are captured before awaiting the transaction;
  refresh and feedback are suppressed if the user navigates away meanwhile.
- Fields without GUI controls (`gpu.power_limit`, `gpu.fan_speed`, and
  `cpu.affinity`) are never reconstructed or changed by GUI saves.

Legacy files without `overrides` still migrate by comparing every descriptor
leaf with the current defaults. Existing `overrides`, YAML keys and grouping,
and default/override/unset explanation semantics are unchanged.
