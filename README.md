# Spela

Linux gaming optimization tool for NVIDIA GPUs. Manages DLSS DLLs, applies per-game
GPU/CPU profiles, and launches games with the right environment — one tool replacing
the MangoHud + LACT + DLSS Updater juggle.

## Install

### AUR (Arch Linux)

```bash
paru -S spela        # stable
paru -S spela-git    # development
```

### Build from source

Requires Go 1.25+, Bun (for GUI frontend), NVIDIA driver, polkit.

```bash
git clone https://github.com/jgabor/spela.git
cd spela
mage build     # builds binary with embedded frontend
mage install   # installs to GOPATH/bin
```

## Quick start

### As a Steam launch option (wrapper mode)

Set a game's launch options in Steam:

```
spela %command%
```

This is the trustworthy launch path. Spela receives Steam's real command, loads
the effective profile, shows the planned preparation, applies the changes it can
track, launches the game process, and runs cleanup when that process exits.

Direct Steam URI launch cannot track the real game lifetime or cleanup coverage.
For normal play, keep launching from Steam with `spela %command%` instead of using
Spela as a separate launcher.

### CLI

```bash
spela scan                          # scan Steam libraries
spela list                          # list detected games
spela show 1091500                  # show game details (Cyberpunk 2077)
spela dll check-updates             # check for newer DLSS DLLs
spela dll update 1091500            # update DLLs for a game
spela profile create 1091500        # create a game profile
spela dlss set 1091500 --sr-mode quality --fg-enabled
spela launch --dry-run 1091500      # show launch preparation without mutation
```

`spela launch` is primarily the wrapper entrypoint used by Steam. Dry runs are
safe for inspection. A normal direct launch by game name or AppID is rejected when
Spela cannot track cleanup; the command tells you to use `spela %command%`.

### TUI / GUI

```bash
spela tui    # interactive terminal UI
spela gui    # graphical interface (Wails + Svelte)
```

The TUI and GUI are configuration and inspection surfaces. Use them to edit
profiles, inspect inherited values, and review DLL or metric state. They do not
replace the Steam wrapper launch path.

## CLI commands

| Command | Description |
|---------|-------------|
| `scan` | Scan Steam libraries for games |
| `list` | List detected games |
| `show` | Show game details |
| `launch` | Launch a game with its profile |
| `profile` | Manage game profiles (list, create, show, delete) |
| `config` | Manage global configuration (show, set) |
| `dll` | Manage game DLLs (list, check-updates, update, restore) |
| `dlss` | Configure DLSS settings (show, set) |
| `gpu` | GPU tuning and information (info, reset) |
| `cpu` | CPU tuning and information (info, governor, smt) |
| `tui` | Launch interactive TUI |
| `gui` | Launch graphical interface |
| `denylist` | Manage the DLL swap deny list (show, check, allow, deny) |

## Profiles

Per-game YAML profiles stored in `~/.config/spela/profiles/`. A profile controls:

```yaml
name: "Cyberpunk 2077 — Quality"

dlss:
  sr_mode: quality
  sr_preset: E
  fg_enabled: true
  indicator: false

gpu:
  shader_cache: true
  clock_offset: 150
  memory_offset: 200

cpu:
  governor: performance
  smt: true

proton:
  enable_wayland: true
  enable_hdr: true

overlay:
  enabled: true
  position: top-left
```

Profiles compose with a default profile (`~/.config/spela/profiles/default.yaml`). The
effective value shown for a game has one source:

- `default`: inherited live from the default profile.
- `override`: pinned in the game profile.
- `unset`: configured by neither the game profile nor defaults.

Launch preparation groups effective values by impact: compatibility,
environment, game file, system state, or overlay. Environment values are
ephemeral child-process variables, not persistent mutations to restore. Hardware
changes such as GPU clocks, power limit, fan speed, CPU governor, and SMT are
restorable mutations when the wrapped game process exits.

DLL updates are explicit file operations through `spela dll`, not implicit
launch-time swaps. Launch preparation reports DLL safety state, including deny
list status, backup availability, and path write-state, without claiming a file
mutation is planned.

## Configuration

Files follow XDG Base Directory specification:

```
~/.config/spela/
├── config.yaml           # Global settings
└── profiles/
    ├── default.yaml      # Default profile
    └── <app-id>.yaml     # Per-game profiles

~/.local/share/spela/
└── backups/              # DLL backups per game

~/.cache/spela/
├── dlls/                 # Downloaded DLL cache
└── manifest.json         # DLL version manifest
```

## System requirements

- **NVIDIA GPU** with proprietary driver (`nvidia-utils`)
- **polkit** for privileged operations (GPU clocks, CPU governor)
- **Steam** with Proton-enabled games
- **Linux** (Wayland recommended, X11 supported)

## License

MIT
