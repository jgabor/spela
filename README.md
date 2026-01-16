![Spela](assets/spela.png)

<p align="center">
  <i>Linux gaming optimization tool</i>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#usage">Usage</a> •
  <a href="#configuration">Configuration</a>
</p>

---

**Spela** (Swedish for "play") is a Linux gaming optimization tool that combines DLSS/DLL management with comprehensive per-game configuration. It solves the pain of maintaining game-specific settings and manually updating DLSS DLLs.

## Features

### 📦 DLL management

- **Automatic DLSS updates:** Download and swap DLSS/XeSS/FSR DLLs with a single command
- **Safe backups:** Original DLLs are backed up before any swap
- **One-click restore:** Revert to original DLLs at any time
- **Version tracking:** See current vs available DLL versions

### 🎮 Per-game profiles

- **Presets:** Performance, Balanced, and Quality presets out of the box
- **Full DLSS control:** Configure DLSS-SR, DLSS-RR (Ray Reconstruction), and DLSS-FG (Frame Generation)
- **Environment variables:** Automatically sets DXVK-NVAPI, Proton, and HDR variables
- **Auto-restore:** Settings are restored when the game exits

### ⚡ System tuning

- **GPU:** Clock offsets, power limits, shader cache configuration
- **CPU:** Governor control, SMT toggle, core affinity, SCX scheduler integration
- **HDR:** Automatic HDR environment setup for Wayland

### 🖥️ Multiple interfaces

- **CLI:** Full-featured command-line interface
- **TUI:** Interactive terminal UI with real-time monitoring
- **GUI:** Native desktop application (Wails + Svelte)

## Installation

### AUR (Arch Linux)

```bash
yay -S spela
```

### Go install

```bash
go install github.com/jgabor/spela/cmd/spela@latest
```

### From source

```bash
git clone https://github.com/jgabor/spela.git
cd spela
make build
sudo make install
```

### Requirements

- Steam with Proton
- **GPU support:**
  - NVIDIA: fully supported

_AMD and Intel support planned._

## Usage

### Scan for games

```bash
spela scan
spela list --with-dlls
```

### Update DLLs

```bash
# Update all DLLs for a game
spela dll update "Cyberpunk 2077"

# Restore original DLLs
spela dll restore "Cyberpunk 2077"
```

### Manage profiles

```bash
# Create a profile from preset
spela profile create "Cyberpunk 2077" --preset performance

# Edit profile settings
spela profile edit "Cyberpunk 2077"

# Apply profile manually
spela profile apply "Cyberpunk 2077"
```

### Launch games

```bash
# Launch with profile applied
spela launch "Cyberpunk 2077"

# Or use as Steam launch option
# Set launch options to: spela %command%
```

### Interactive TUI

```bash
spela tui
```

## Configuration

Configuration files are stored following XDG Base Directory specification:

```
~/.config/spela/
├── config.yaml           # Global settings
└── profiles/
    └── <app-id>.yaml     # Per-game profiles

~/.local/share/spela/
└── backups/              # DLL backups per game

~/.cache/spela/
├── dlls/                 # Downloaded DLL cache
└── manifest.json         # DLL version manifest
```

### Profile example

```yaml
name: Cyberpunk 2077
preset: performance
dlss:
  sr_mode: performance
  sr_override: true
  frame_gen: true
gpu:
  clock_offset: 100
  shader_cache: true
cpu:
  governor: performance
  smt: true
hdr: true
wayland: true
```

## 🔧 Environment variables

Spela configures these environment variables when launching games:

| Variable                                     | Description                     |
| -------------------------------------------- | ------------------------------- |
| `DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE`            | DLSS Super Resolution mode      |
| `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE`        | Force DLSS-SR override          |
| `DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE`        | Force Frame Generation override |
| `DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT` | Multi-frame generation count    |
| `PROTON_ENABLE_WAYLAND`                      | Enable Wayland support          |
| `PROTON_ENABLE_HDR`                          | Enable HDR support              |
| `PROTON_ENABLE_NGX_UPDATER`                  | Use NVIDIA's built-in updater   |
| `__GL_SHADER_DISK_CACHE_PATH`                | Custom shader cache location    |

## 📄 License

MIT
