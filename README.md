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

Build prerequisites are Git, Rust 1.89 or newer, and
[just](https://github.com/casey/just).

```bash
git clone https://github.com/jgabor/spela.git
cd spela
just build-release     # builds target/release/spela
sudo just install      # installs /usr/bin/spela and the polkit policy
```

Steam runs launch options with `PATH=/usr/bin:/bin`, so `spela %command%` needs
the binary in `/usr/bin`. The polkit policy describes the action spela asks for
when it tunes the GPU and CPU around a launch.

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
spela profile show 1091500          # every setting: effective value and where it comes from
spela dlss show 1091500             # effective DLSS values and their sources
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

# Make a game follow All games for every setting.
spela profile reset 1091500
```

`dlss set --sr-mode` enables the SR override; `--fg true` enables the FG override
as well as frame generation. Resetting a field makes the game follow All games again; it does not turn the setting off.
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
spela dll update 1091500             # update every stale DLL of the game, backing up originals
spela dll update 1091500 dlss        # only DLSS Super Resolution
spela dll install 1091500 dlss --version 310.2.1   # pin a version, older ones included
spela dll update --all               # every game a deny list does not name
spela dll restore 1091500            # put the originals back, remove DLLs spela added
spela dll cache                      # the versions in the DLL cache
```

A type is one of `dlss`, `dlssg`, `dlssd`, `xess` or `fsr`. Restore works on the
game's whole backup, not just the most recently updated type, and removes a DLL
that an install created where none existed. DLL updates persist until restored;
wrapper cleanup does not undo them.

In the TUI, the same operations are on a game's DLLs section (the last section
of its page, or its Actions), on the Library's batch actions for selected games,
and on the DLL Catalog's Deployment group as `Update all stale deployments`.
Each one asks first, with Cancel focused. With `auto_update_dlls` on, the TUI
updates stale DLLs at start, skipping running and deny-listed games, and with
`check_updates` on it says when a newer spela is released. Only release
binaries check: the `spela-linux-amd64` download from GitHub Releases does,
while AUR `spela-git` and source builds never do.

Use `spela --help` or `spela <command> --help` for the full command reference.

## Profiles and inheritance

Game profiles live in `~/.config/spela/profiles/<app-id>.toml`. The file
`profiles/all-games.toml` holds the All games profile. A game follows All games
for every setting it does not set itself. CLI setters give a game its own
value; reset commands make it follow All games again. The `show` commands print
each setting with its value and its source: `from All games`, `system default`,
`this game · All games: <value>` for a value the game set itself, or
`this game · same as All games` for one that will not follow later changes to
All games. `--json` prints the stored values only.

A profile file stores only the values its scope sets itself: a key that is
present is the game's own value, and a key that is absent follows All games.
For example, after `spela dlss set 1091500 --sr-mode quality` and
`spela gpu set 1091500 --shader-cache true`,
`~/.config/spela/profiles/1091500.toml` reads:

```toml
version = 1
name = "Cyberpunk 2077"

[dlss]
sr_mode = "quality"
sr_override = true

[gpu]
shader_cache = true
```

`dlss.sr_override` activates the DLSS override. An explicit Off or 0 is written
as it is, such as `threaded_optimization = false`, and still counts as the
game's own value. spela edits these files in place, keeping comments and key
order, so they can also be edited by hand. Review the effective settings and
dry-run output before launching, especially if you have customized All games.
The profile fields and their effects are described in the
[architecture's profile section](docs/architecture.md#profiles-and-inheritance);
[docs/profile-fields.md](docs/profile-fields.md) still describes Go spela 0.6.

### Coming from Go spela 0.6

The first time it runs, spela migrates Go's YAML files to TOML once:
`config.yaml`, `profiles/*.yaml` (with `default.yaml` becoming
`all-games.toml`), `lists/*.yaml` and the game database. The originals stay in
place as backups and are never read again. A value that has no place in the new
files, such as the removed overlay settings, is recorded in
`~/.local/share/spela/migration.toml` and in the header of the migrated file.
DLL backups written by Go are used where they are.

## Launch and cleanup limits

Environment settings apply to the child process. Hardware settings (GPU clocks,
power limit and fan, CPU governor and SMT) are applied by a root helper that
reads each prior value first and stays running while the game runs. It restores
them when the game exits, on a handled stop signal, and also when the wrapper
itself is killed, because the helper sees its input close. This last case is
tested on fakes but not yet confirmed on real hardware. A setting whose prior
value cannot be read is skipped, unsupported settings are skipped with
warnings, and a restore can fail; each case is logged. KDE VRR and the
environment settings need the wrapper itself, so a killed wrapper leaves VRR as
the game had it. Power loss, or killing the root helper with SIGKILL, skips
every restore; a reboot clears the hardware settings, and `spela gpu reset`
resets the GPU clocks.

A direct Steam URI launch cannot track the real game lifetime or provide the
same cleanup coverage. `spela launch <game>` without `--dry-run` always exits
with code 2 and points to `spela %command%`: use Steam with `spela %command%`
for play and `spela launch --dry-run 1091500` for inspection. Launch
preparation reports DLL denylist, backup, and path write-state information
without planning a DLL swap.

## Configuration locations

Spela follows the XDG Base Directory specification. These are the default
locations; XDG environment variables can move them.

| Default path | Contents |
|---|---|
| `~/.config/spela/config.toml` | Global configuration |
| `~/.config/spela/profiles/all-games.toml` | The All games profile |
| `~/.config/spela/profiles/<app-id>.toml` | Per-game profiles |
| `~/.config/spela/lists/` | Deny list overrides and your own deny list |
| `~/.local/share/spela/backups/` | DLL backups per game |
| `~/.local/share/spela/migration.toml` | What the one-time migration from Go spela did |
| `~/.cache/spela/games.json` | Scanned game database |
| `~/.cache/spela/` | Downloaded DLLs, the version manifest and the release check |
| `~/.local/state/spela/log/` | Log files |
| `~/.local/state/spela/locks/` | The TUI, launch and store locks |

`spela config show` prints every key with its value, and
`spela config set <key> <value>` changes one. The keys and their defaults are
listed in the [architecture's config.toml section](docs/architecture.md#configtoml);
[docs/configuration.md](docs/configuration.md) still describes Go spela 0.6.

## License

MIT
