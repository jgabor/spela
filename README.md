# Spela

Spela is a Linux gaming configuration tool for NVIDIA GPUs. Keep per-game
settings in one place and apply them when you launch from Steam.

- **Per-game profiles** with shared defaults and explicit overrides.
- **DLSS and Proton configuration** without maintaining long launch options.
- **Explicit DLL management** with backups for updates and restoration.
- **Interactive terminal UI** for browsing games, editing profiles, and inspecting settings.
- **Temporary GPU/CPU settings** applied during tracked Steam launches, with cleanup
  when the wrapped process exits.

## Requirements

- Linux, an NVIDIA GPU, and the proprietary NVIDIA driver (`nvidia-utils` on Arch).
- Steam with Proton-enabled games. DLSS features depend on support in the GPU,
  game, driver, and Proton version; configuration cannot add missing support.
- polkit for privileged hardware tuning, such as GPU power limits and CPU governor changes.

## Install

### AUR (Arch Linux)

```bash
yay -S spela-git
```

### Build from source

Build prerequisites are Git, Go 1.25.5 or newer, Bun 1.3.14 (the pinned package
manager version), a C toolchain, `pkg-config`, and the GTK 3 and WebKit2GTK 4.1
development headers. These dependencies are needed for the integrated binary,
even when you only use its CLI or TUI. Installation also uses `sudo`.

```bash
git clone https://github.com/jgabor/spela.git
cd spela
go tool mage build     # builds ./spela
go tool mage install   # installs to GOPATH/bin, then copies to /usr/bin with sudo
```

## First use

Scan your Steam libraries, inspect a game, and create a profile. These examples
use Cyberpunk 2077 (`1091500`); substitute an AppID from your own library.

```bash
spela scan
spela list
spela tui                          # browse and configure in the terminal; exit to continue
spela show 1091500                  # game details and detected DLLs
spela profile create 1091500
spela dlss set 1091500 --sr-mode quality
spela launch --dry-run 1091500      # inspect preparation without applying changes or launching
```

Then set the game's **Steam launch options** to:

```text
spela %command%
```

Launch the game from Steam. Spela receives Steam's real command, loads the
effective profile, reports preparation, applies supported settings, and tracks
the wrapped process for cleanup. The TUI is for configuration and inspection;
it does not replace this launch path.

## Useful CLI examples

### Inspect effective settings

```bash
spela profile show 1091500          # stored profile, not merged defaults
spela dlss show 1091500             # effective DLSS values and inheritance sources
spela gpu show 1091500              # effective GPU profile settings
spela proton show 1091500           # effective Proton settings
spela dlss show 1091500 --json       # effective DLSS values as JSON
```

### Change settings and return to defaults

These commands edit the game profile; they do not tune live hardware or launch
the game. Enable frame generation only for a compatible game and GPU.

```bash
spela dlss set 1091500 --sr-mode quality --fg true
spela gpu set 1091500 --shader-cache true
spela proton set 1091500 --ngx-updater false
spela launch --dry-run 1091500

# Return the fields changed above to inheritance, including DLSS activation flags.
spela dlss reset 1091500 sr_mode
spela dlss reset 1091500 sr_override
spela dlss reset 1091500 fg_enabled
spela dlss reset 1091500 fg_override
spela gpu profile-reset 1091500 shader_cache
spela proton reset 1091500 ngx_updater
```

`dlss set --sr-mode` enables the SR override; `--fg true` enables the FG override
as well as frame generation. Resetting a field means “inherit,” not “turn off.”
Use profile commands or the TUI for launch-time hardware settings: live controls
such as `cpu governor`, `cpu smt`, and `gpu reset` are separate operations.

### Inspect, update, and restore DLLs

DLL changes are **explicit file operations**, not automatic launch-time swaps.
Close the game before updating or restoring. Check compatibility first,
especially for games with anti-cheat: the denylist blocks known unsafe swaps,
but absence from it is not proof of safety. A backup helps restore files; it
does not protect against anti-cheat penalties.

```bash
spela dll list 1091500              # inspect detected DLLs
spela denylist check 1091500        # check DLL swap policy
spela dll check-updates             # fetch/check available versions

# Optional file changes, only after reviewing compatibility:
spela dll update 1091500 dlss        # update DLSS Super Resolution, backing up originals
spela dll restore 1091500            # restore the game's backed-up DLLs
```

The update command requires a DLL type (`dlss`, `dlssg`, `dlssd`, `xess`, or
`fsr`). Restore operates on the game's backups, not just the most recently
updated type. DLL updates persist until restored; wrapper cleanup does not undo them.

Use `spela --help` or `spela <command> --help` for the full command reference.

## Profiles and inheritance

Game profiles live in `~/.config/spela/profiles/<app-id>.yaml`. Fields inherit
from `profiles/default.yaml` unless explicitly pinned in the game profile.
CLI setters pin fields; reset commands return them to the current defaults.
The subsystem `show` commands mark values as inherited or overridden, so you
can see which choices are specific to a game.

For example, `~/.config/spela/profiles/1091500.yaml` can request DLSS Quality
with the default preset and enable shader caching, without choosing hardware
clocks or enabling frame generation:

```yaml
name: Cyberpunk 2077 — Quality
dlss:
  sr_mode: quality
  sr_preset: default
  sr_override: true
gpu:
  shader_cache: true
overrides:
  dlss.sr_mode: true
  dlss.sr_preset: true
  dlss.sr_override: true
  gpu.shader_cache: true
```

`dlss.sr_override` activates the DLSS override. The top-level `overrides` map
has a different purpose: it pins each chosen field against changes to defaults.
Unlisted fields still inherit, so review the effective settings and dry-run
output before launching, especially if you have customized defaults. See the
[profile field reference](docs/profile-fields.md) for settings and their effects.

## Launch and cleanup limits

Environment settings apply to the child process. Restorable hardware settings
are restored after the tracked process exits or on handled termination, where
the prior state could be captured and the change applied. Unsupported settings
may be skipped with warnings, and restoration can fail. SIGKILL, a crash, or
power loss cannot run cleanup.

A direct Steam URI launch cannot track the real game lifetime or provide the
same cleanup coverage. A normal direct `spela launch 1091500` is rejected when
cleanup cannot be tracked; use Steam with `spela %command%` for play and
`spela launch --dry-run 1091500` for inspection. Launch preparation reports DLL
denylist, backup, and path write-state information without planning a DLL swap.

## Configuration locations

Spela follows the XDG Base Directory specification. These are the default
locations; XDG environment variables can move them.

| Default path | Contents |
|---|---|
| `~/.config/spela/config.yaml` | Global configuration |
| `~/.config/spela/profiles/` | Default and per-game profiles |
| `~/.local/share/spela/games.yaml` | Scanned game database |
| `~/.local/share/spela/backups/` | DLL backups per game |
| `~/.cache/spela/` | Downloaded DLLs and version manifest |

See the [global configuration reference](docs/configuration.md) for keys and defaults.

## License

MIT
