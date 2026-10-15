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
| `dlss.sr_mode` | string | Super resolution | unset, off, ultra_performance, performance, balanced, quality, dlaa | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.sr_preset` | string | Resolution preset | unset, default, auto, A-F, J-M | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.sr_override` | bool | Resolution override | true, false | environment | ephemeral launch | CLI (set with --sr-mode or --sr-preset)/TUI/GUI |
| `dlss.rr_mode` | string | Ray reconstruction | DLSS modes | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.rr_preset` | string | Ray preset | unset, default, A-F, J-M | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.rr_override` | bool | Ray override | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.fg_enabled` | bool | Frame generation | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.fg_override` | bool | Generation override | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.multi_frame` | int | Multi-frame | 0-4 | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.indicator` | bool | SR indicator | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `dlss.fg_indicator` | bool | FG indicator | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `gpu.clock_offset` | int | Clock offset | unbounded, MHz | system state | restorable mutation | CLI/TUI/GUI |
| `gpu.vrr` | string | KDE VRR | unset, automatic, always, never | system state | restorable mutation | CLI show only/TUI/GUI |
| `gpu.memory_offset` | int | Memory offset | unbounded, MHz | system state | restorable mutation | CLI/TUI/GUI |
| `gpu.power_limit` | int | Power limit | unbounded, W | system state | restorable mutation | CLI/TUI/GUI |
| `gpu.fan_speed` | int | Fan speed | 0-100 %, shown and not enforced | system state | restorable mutation | CLI/TUI/GUI |
| `gpu.power_mizer` | string | Power mode | unset, adaptive, max, legacy prefer-max spellings | system state | not applicable | CLI/TUI/GUI |
| `gpu.shader_cache` | bool | Shader cache | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `gpu.shader_cache_path` | string | Shader cache path | unbounded | environment | ephemeral launch | CLI/TUI/GUI |
| `gpu.threaded_optimization` | bool | Threaded optimization | true, false | environment | ephemeral launch | CLI/TUI/GUI |
| `cpu.governor` | string | Governor | performance, powersave, schedutil, ondemand | system state | restorable mutation | CLI/TUI/GUI |
| `cpu.smt` | optional bool | SMT | unset, true, false | system state | restorable mutation | CLI/TUI/GUI |
| `cpu.affinity` | string | Affinity | unbounded | system state | not applicable | CLI show only/TUI/GUI |
| `overlay.enabled` | bool | Enabled | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.position` | string | Position | top-left, top-right, bottom-left, bottom-right | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_fps` | bool | Show FPS | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_frametime` | bool | Show frametime | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_cpu` | bool | Show CPU | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_gpu` | bool | Show GPU | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.show_vram` | bool | Show VRAM | true, false | overlay | not applicable | CLI/TUI/GUI |
| `overlay.toggle_key` | string | Toggle key | unbounded | overlay | not applicable | CLI/TUI/GUI |

## Shared words

`internal/profile/vocabulary.go` holds the words every surface prints, and
`internal/profile/fields.go` holds each field's label, description, unit, and
display range. A surface formats layout; it does not invent its own words.

| Idea | Words |
|---|---|
| The value comes from All games | `from All games`; action **Follow All games** |
| The game has its own value | `this game · All games: Off` |
| Own value equal to All games | `this game · same as All games` (it will not follow later changes) |
| Nothing sets the value | `system default`; a number nothing sets reads `none` |
| A value All games sets (All games page) | `set here`; action **Use system default** |
| Booleans, empty values | `On`, `Off`, `none`; stored as `true`, `false`, and empty |
| Clear every setting of a game | **Follow All games for all settings** (`spela profile reset`) |

Display ranges (fan speed 0 to 100%) are shown, never enforced: a stored value
outside the range still loads and saves. Retired wording: `(default)`,
"Override for this game", "Inherited from defaults", "Default for all games",
"defaults profile", "root profile".

## Mutation semantics

### Terminal editing contract

The profile field table is the terminal editor catalog. Every entry exposes a
stable key, subsystem, primitive type, label, description, constraints, and an
editor kind. Presentation code may format values, but it must not maintain a
second list of profile fields or invent constraints that are absent here.

Editing a game field to a concrete value gives the game its own value; reset
makes it **Follow All games** again. An All games field instead resets to **Use
system default**. Every applied change is written at once, merged into the
stored profile as a diff of the changed fields, and then becomes the new visible
baseline. Cancel and Esc drop an edit that was not applied. A rejected value
stays in the editor with its error. A failed write returns the row to the stored
value and keeps the error visible. Settings uses the same editor host and saves
at once while retaining its separate settings catalog; neither editor may mutate
live configuration before the write succeeds.

- **set** validates and writes the supplied primitive exactly and marks the
  leaf in `Overrides`; false, zero, empty string, and nil SMT are explicit
  values. Bounded descriptor values are the validation source for CLI, TUI,
  and GUI mutations.
- **pin** copies the currently effective value from live defaults into the raw
  game profile and marks only that leaf. It is invalid for the defaults root.
- **reset** zeros only the named raw leaf and removes only its override marker,
  so the live default becomes effective immediately.
- The GUI saves every change as it is made: a control change sends `set`, Follow
  All games sends `reset`, and Set for this game sends `set` with the value the
  field had before it followed (or `pin` when there is none), so the same button
  undoes itself. Each change is one ordered `[]ProfilePatch` batch. The backend
  serializes profile transactions, loads raw storage once, applies every patch
  to an independent copy, saves once only if all patches succeed, and returns a
  refreshed projection. A later invalid patch therefore cannot partially save.
- The GUI shows **Set for this game** for inherited game fields, **Follow All
  games** for the game's own values, and on the All games page **Use system
  default** for a value set there and **Set here** for one that is not (Set here
  sets the value shown, or the one Use system default removed). The projection carries these words, each field's label, description,
  unit, number hint, value text, and source text (for example `this game · All
  games: 150 MHz`), so the GUI prints what the terminal and the command line
  print. Editing a control always sends `set`, including zero, false, empty
  string, and unset SMT. These explicit values use literal labels such as
  `No mode` and `No SMT value`, never `(default)`. A stored value that a list
  does not offer is shown as it is stored. Frame generation treats `fg_enabled`
  and `fg_override` as one inheritance action sent in one batch. Number fields
  (clock offset, memory offset, power limit, fan speed) take a whole number; the
  range is shown and not enforced.
- Saves for one page run one after the other, and only the newest response
  replaces the controls. A failed save returns the controls to the last saved
  copy and shows `Not saved` with the reason. The target scope and AppID are
  captured before awaiting the transaction; refresh and feedback are
  suppressed if the user navigates away meanwhile.
- The projection also carries `clearAll` (the whole-page action word, how many
  settings the page holds itself, and the confirmation sentence), and on the All
  games page each row's `ownValueSuffix`, `ownValueCount` and `ownValueAction`
  (`4 games have their own value`). Follow All games for all settings confirms,
  then sends one batch of `reset` patches for every own field.
- On All games, a field some games set themselves offers **Apply to games with
  their own value**. `PreviewApplyToGamesWithOwnValue(fields)` returns the
  sentence that names the games and the value; after the user confirms,
  `ApplyToGamesWithOwnValue(fields)` makes those games follow All games for the
  field (the frame generation control passes both of its fields) and returns the
  message, the games that could not be written, and the refreshed All games
  page. The TUI offers the same action in the Actions menu on that row, and a line
  under the focused row says so. Each
  game is its own locked write, so a failure names the game and the rest still
  change; other fields of those games are not touched.
- Every one of the 35 fields has a GUI control, including `cpu.affinity`, which
  Spela does not apply yet; its description says so.

Every saved profile carries the `overrides` key, written as `overrides: {}`
when nothing is pinned, so a profile whose last override was reset reloads as
fully inherited. Only files without the key are legacy: they migrate by
comparing every descriptor leaf with the current defaults. A legacy file that
was saved without the key before this rule keeps migrating, and its first edit
freezes the migrated values as overrides. Existing `overrides`, YAML keys and
grouping, and default/override/unset explanation semantics are unchanged.
