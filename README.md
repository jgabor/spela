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

Spela detects the game, loads its profile (or the default profile), applies DLSS
settings, GPU clocks, CPU governor, and environment variables, then launches the game.
Everything is restored on exit.

### CLI

```bash
spela scan                          # scan Steam libraries
spela list                          # list detected games
spela show 1091500                  # show game details (Cyberpunk 2077)
spela dll check-updates             # check for newer DLSS DLLs
spela dll update 1091500            # update DLLs for a game
spela profile create 1091500        # create a game profile
spela dlss set 1091500 --sr-mode quality --fg-enabled
spela launch 1091500                # launch with profile applied
```

### TUI / GUI

```bash
spela tui    # interactive terminal UI
spela gui    # graphical interface (Wails + Svelte)
```

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

Profiles compose with a default profile (`~/.config/spela/default.yaml`). Game-specific
settings override defaults. `spela launch` applies the effective profile and restores
everything on exit.

## Configuration

Files follow XDG Base Directory specification:

```
~/.config/spela/
├── config.yaml           # Global settings
├── default.yaml          # Default profile
└── profiles/
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
