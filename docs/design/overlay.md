# Overlay technical design

This document describes the architecture for Spela's built-in game overlay — a next-generation in-game HUD for Linux/NVIDIA that replaces MangoHud.

## North star goals

1. **Visually beautiful** — SDF fonts, custom GPU-rendered charts with anti-aliased lines and glow, NVIDIA App-style semi-transparent panels. The overlay should look so much better than MangoHud that screenshots sell the product.
2. **Smart insights** — Actionable recommendations ("GPU thermal throttling — reduce power limit by 10W"), not just raw numbers. Adaptive coaching in v1.x.
3. **Immediately responsive** — In-game configuration via emulated interactivity. No more exit-game-edit-config-relaunch loops.

## Architecture: "Visually smart, logically dumb"

The overlay layer renders beautifully but makes no decisions. All intelligence lives in the Go process. Input is captured by Go via Wayland's XDG Desktop Portal and translated into UI state updates pushed to the layer via shared memory.

```
┌─────────────────────────────────────────────────────────┐
│                      GAME PROCESS                        │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  VK_LAYER_SPELA_overlay (Zig / ImGui)              │  │
│  │  - Reads mmap (display data, UI state from Go)     │  │
│  │  - Renders SDF text, custom GPU charts, ImGui      │  │
│  │  - Measures frame times → writes to mmap           │  │
│  │  - Resolution-adaptive scaling                     │  │
│  │  - Zero input handling                             │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  [Other layers: Steam overlay, VK_NV_present, etc.]      │
└──────────┬───────────────────────────────────────────────┘
           │ mmap (single file, io_uring-inspired ring buffers)
           │ Go→Layer: state, events, metric history
           │ Layer→Go: frame times, commands
┌──────────▼───────────────────────────────────────────────┐
│                   SPELA GO PROCESS                        │
│                                                           │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │ Metrics      │  │ UI State     │  │ Input          │  │
│  │ (go-nvml,   │  │ Machine      │  │ (XDG Global-   │  │
│  │  sysfs)     │  │ (menus,      │  │  Shortcuts     │  │
│  │             │  │  sliders)    │  │  Portal/DBus)  │  │
│  └──────┬──────┘  └──────┬───────┘  └───────┬────────┘  │
│         │                │                   │            │
│  ┌──────▼────────────────▼───────────────────▼─────────┐ │
│  │              Intelligence Layer                      │ │
│  │  - Smart alerts (throttle detection, recommendations)│ │
│  │  - Pre-renders display strings for overlay           │ │
│  │  - Executes runtime tuning (NVML setters via polkit) │ │
│  │  - Manages profiles and launch configs               │ │
│  │  - Session comparison (v1.x)                         │ │
│  └──────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────┘
```

### Input handling: emulated interactivity

The layer has zero input handling. Interactivity is emulated:

1. Go registers global shortcuts via `org.freedesktop.portal.GlobalShortcuts` (XDG Desktop Portal, DBus)
2. When the user presses a hotkey (e.g. toggle overlay, navigate menu), Go receives the event via DBus
3. Go updates UI state (menu open/closed, highlighted item, slider value) and writes it to the mmap state section
4. The layer renders the updated state — it *looks* interactive but the layer never touched input

This approach works on all Wayland compositors that support the XDG Desktop Portal (KDE, GNOME, wlroots-based).

**X11 is out of scope.** Wayland only.

## Components

### Zig Vulkan layer (`overlay/`)

The overlay is implemented as a Vulkan implicit layer written in Zig.

**Key dependencies:**

- `vulkan-zig` — Vulkan bindings (auto-generated from spec)
- `cimgui` — ImGui C bindings (Zig calls C directly via `@cImport`)
- Custom SDF font renderer (MSDF atlas + fragment shader)
- Custom chart renderer (AA line strips + gradients + glow)

**Layer behavior:**

1. Intercepts `vkQueuePresentKHR` to inject overlay rendering
2. Measures frame time between present calls
3. Reads display state and metric history from mmap ring buffer
4. Renders overlay using SDF text, custom charts, and ImGui widgets
5. Adapts rendering quality based on current FPS (adaptive performance budget)
6. Watchdog: skips overlay rendering if previous frame took >2ms

**File structure:**

```
overlay/
├── build.zig
├── src/
│   ├── main.zig          # Layer entry point and Vulkan dispatch
│   ├── layer.zig         # Vulkan layer implementation
│   ├── renderer.zig      # ImGui setup and composite rendering
│   ├── sdf.zig           # SDF font atlas loading and text rendering
│   ├── charts.zig        # Custom GPU-rendered charts (AA lines, glow)
│   ├── metrics.zig       # Frame timing, FPS calculation
│   ├── ipc.zig           # Ring buffer reader/writer
│   ├── scaling.zig       # Resolution-adaptive scaling
│   └── ui/
│       ├── hud.zig       # Main overlay HUD
│       ├── menu.zig      # Settings menu panel (rendered from Go state)
│       ├── alerts.zig    # Alert card rendering
│       └── widgets.zig   # Reusable UI components
├── shaders/
│   ├── sdf.frag          # SDF text fragment shader
│   ├── chart_line.vert   # AA line vertex shader
│   ├── chart_line.frag   # AA line + glow fragment shader
│   └── panel.frag        # Semi-transparent panel shader
├── assets/
│   └── atlas.msdf        # Pre-generated MSDF font atlas
└── manifest/
    └── spela_overlay.json
```

### Go components (`internal/overlay/`)

The Go side handles all intelligence, metrics, input, and IPC.

**Responsibilities:**

- Poll GPU stats via NVML, CPU stats via sysfs
- Run alert logic (thermal throttling, power limit detection)
- Maintain UI state machine (menu navigation, slider positions, panel states)
- Listen for XDG GlobalShortcuts events via DBus
- Write display state, events, and metric history to mmap ring buffer
- Read frame times and user commands from mmap ring buffer
- Execute NVML setter commands (power limit, fan speed, clock offsets) via polkit

**File structure:**

```
internal/overlay/
├── config.go         # Overlay configuration
├── stats.go          # System stats collector
├── ipc.go            # Ring buffer writer/reader
├── alerts.go         # Throttle detection and recommendations
├── input.go          # XDG GlobalShortcuts portal integration
├── uistate.go        # UI state machine (menus, sliders, panels)
└── theme.go          # Theme presets and color overrides
```

## IPC protocol

Communication uses a single memory-mapped file at `$XDG_RUNTIME_DIR/spela-overlay-<pid>.dat`.

The design is inspired by io_uring's ring buffer architecture but implemented entirely in userspace — no kernel involvement, zero syscalls.

### File layout

```
┌────────────────────────────────────────────────────────────┐
│ Header (64 bytes)                                          │
│ ├─ magic: u32 = 0x5350454C ("SPEL")                       │
│ ├─ version: u16                                            │
│ ├─ flags: u16                                              │
│ ├─ state_offset: u32                                       │
│ ├─ state_size: u32                                         │
│ ├─ event_ring_offset: u32                                  │
│ ├─ command_ring_offset: u32                                │
│ ├─ data_ring_offset: u32                                   │
│ └─ reserved: [32]u8                                        │
├────────────────────────────────────────────────────────────┤
│ State Section (Go → Layer, overwrite, ~4KB)                │
│ ├─ seqlock: u32 (atomic, even=stable, odd=writing)         │
│ ├─ Display strings (pre-rendered text lines with colors)   │
│ ├─ UI state (menu open, selected item, slider values)      │
│ ├─ Alert state (type, message, suggestion, icon)           │
│ ├─ Profile info (name, badge indicator for pending changes)│
│ ├─ Config (visibility, position, opacity, layout preset)   │
│ └─ GPU tuning current values (for before/after display)    │
├────────────────────────────────────────────────────────────┤
│ Event Ring — Go → Layer (FIFO, 256 slots × 64B = 16KB)    │
│ ├─ head: u32 (atomic, Go increments after write)           │
│ ├─ tail: u32 (atomic, Layer increments after read)         │
│ └─ entries[256]: EventEntry                                │
│     ├─ type: u8 (graph_marker, alert, config_change, etc.) │
│     ├─ timestamp: u64                                      │
│     └─ payload: [55]u8                                     │
├────────────────────────────────────────────────────────────┤
│ Command Ring — Layer → Go (FIFO, 256 slots × 64B = 16KB)  │
│ ├─ head: u32 (atomic, Layer increments after write)        │
│ ├─ tail: u32 (atomic, Go increments after read)            │
│ └─ entries[256]: CommandEntry                              │
│     ├─ type: u8 (frame_time_batch, user_command, etc.)     │
│     └─ payload: [63]u8                                     │
├────────────────────────────────────────────────────────────┤
│ Data Ring — Go → Layer (circular, 4096 samples × 8B = 32KB)│
│ ├─ write_pos: u32 (atomic)                                 │
│ └─ samples[4096]: MetricSample                             │
│     ├─ timestamp: u32 (relative, ms)                       │
│     └─ value: f32 (metric value)                           │
└────────────────────────────────────────────────────────────┘
```

### Synchronization

All ring buffers use SPSC (single-producer single-consumer) lockfree synchronization:

- **State section:** Seqlock pattern — Go increments seqlock to odd before writing, even after. Layer retries read if seqlock is odd or changed during read.
- **Event/Command rings:** Atomic head/tail with acquire/release memory ordering. Power-of-2 sizing for efficient modulo (bitwise AND with mask).
- **Data ring:** Atomic write position. Layer reads from `write_pos - N` to `write_pos` for the last N samples.

## Rendering

### SDF font rendering

Fonts are pre-processed into a Multi-channel Signed Distance Field (MSDF) atlas using `msdf-atlas-gen` at build time. The atlas contains glyph distance fields that the GPU samples at runtime.

**Advantages:**

- Resolution-independent — crisp text at 1080p, 1440p, and 4K from a single atlas
- GPU-accelerated — fragment shader computes anti-aliasing per-pixel
- No FreeType runtime dependency
- No atlas regeneration on resolution change

**Pipeline:**

1. Build time: `msdf-atlas-gen` processes font → generates atlas texture + glyph metrics JSON
2. Runtime: Layer loads atlas as a Vulkan texture
3. Per-frame: Vertex shader positions quads per glyph, fragment shader samples MSDF and applies anti-aliasing

### Custom chart rendering

Frame time graphs and GPU metric charts use custom vertex-based rendering:

- **Line rendering:** Triangle strips generated from line segments with proper mitering for anti-aliased edges
- **Fill:** Gradient-filled area under the line (vertex colors on fill triangles)
- **Glow:** Additive blending pass on the line geometry for a subtle glow effect
- **Graph markers:** Vertical lines at tuning change points with labels

### Adaptive performance budget

The layer scales rendering quality based on current FPS:

- High FPS (>120): Reduce chart sample count, simplify glow
- Medium FPS (60-120): Full quality rendering
- Low FPS (<60): Minimal rendering to avoid adding to the problem

**Watchdog:** If overlay rendering exceeds 2ms, skip the overlay entirely for that frame.

**Kill switch:** `SPELA_OVERLAY_DISABLE=1` env var completely disables the layer.

## Visual design

### Style: NVIDIA App-inspired

- Clean, flat, semi-transparent panels
- Smooth SDF-rendered text (no bitmap artifacts)
- Subtle rounded corners on panel backgrounds
- Color-coded metrics (green/yellow/red thresholds)
- Prominent alert cards with icon, message, and suggested action

### Layout presets

Three built-in presets:

1. **Minimal** — FPS counter + frame time, top-left corner
2. **Compact** — FPS, GPU temp/power/clock, CPU usage, small frame time graph
3. **Full** — All metrics, large frame time graph, profile name, alert area

### Color configuration

Colors are overridable via simple YAML in `~/.config/spela/overlay.yaml`:

```yaml
overlay:
  preset: compact
  position: top-left
  opacity: 0.85
  colors:
    background: "#1a1a2e80"
    text: "#eeeeeeff"
    accent: "#4a9fffff"
    warning: "#ffc107ff"
    critical: "#dc3545ff"
    graph_line: "#4a9fffff"
    graph_glow: "#4a9fff40"
```

## Smart insights

### v1: Actionable recommendations

The Go process monitors NVML telemetry and generates alerts:

| Condition | Alert |
|-----------|-------|
| GPU temp > throttle threshold | "GPU thermal throttling — consider improving airflow or reducing power limit" |
| GPU hitting power limit | "GPU power-limited at {current}W — increase to {suggested}W for +{estimated}% FPS" |
| GPU clock below boost | "GPU not reaching boost clock — check power limit and thermals" |
| Fan speed at 100% | "Fans at maximum — GPU temp: {temp}C" |

Alerts appear as prominent cards with an icon, message, and suggested action. They auto-dismiss after 10 seconds or can be dismissed via hotkey.

### v1.x: Adaptive coaching

- Session comparison: "FPS 15% lower than last session at this location"
- Profile suggestions: "Switch to Performance preset to hit your 60 FPS target"
- Stutter detection: "Stutter detected: 45ms spike, 3 occurrences in last minute"

### Tuning feedback

When the user changes a GPU parameter:

- **Graph marker:** Vertical line on the frame time graph at the change point
- **Before/after comparison:** Small card showing the old and new values with the performance delta

## Privilege escalation

NVML setter functions (power limit, fan speed, clock offsets) require elevated privileges. Spela uses its existing polkit infrastructure:

1. User authenticates once via polkit agent when Spela starts a tuning session
2. Polkit policy (`data/polkit/`) authorizes NVML operations for the session duration
3. All subsequent NVML setter calls execute without additional prompts

## MangoHud coexistence

When the overlay is active and MangoHud is detected:

- Show a one-time notification: "MangoHud detected — disable it for best experience?"
- If user chooses to disable: set `DISABLE_MANGOHUD=1` for future launches
- If user chooses to keep: both overlays run (with a note about potential performance impact)
- Preference is stored in the game's profile

## Vulkan layer stacking

A game may have multiple implicit layers active:

1. `VK_LAYER_VALVE_steam_overlay`
2. `VK_LAYER_NV_present` (Smooth Motion)
3. `VK_LAYER_SPELA_overlay`

**Mitigations:**

- Use `enable_environment` in the layer manifest (opt-in via `SPELA_OVERLAY=1`)
- Spela sets this env var when launching games with the overlay enabled
- Test layer interactions with Steam overlay and NV_present
- Document known conflicts

## Build integration

Mage orchestrates both Go and Zig builds:

```go
// magefile.go

func Build() error {
    mg.Deps(BuildGo, BuildOverlay)
    return nil
}

func BuildOverlay() error {
    return sh.Run("zig", "build", "-Doptimize=ReleaseFast",
        "--build-file", "overlay/build.zig")
}
```

The overlay `.so` is installed to `/usr/lib/spela/` with the layer manifest in `/usr/share/vulkan/implicit_layer.d/`.

## Implementation phases

| Phase | Focus | Key deliverable |
|-------|-------|-----------------|
| 0 | Proof of concept | Zig layer renders SDF text on vkcube |
| 1 | Foundation | Always-on HUD with live metrics via mmap |
| 2 | Visual polish | Custom GPU charts, resolution scaling, panels |
| 3 | Smart insights | Throttle detection, alert cards, graph markers |
| 4 | Interactivity | XDG GlobalShortcuts, emulated UI, GPU tuning |
| 5 | Polish & compat | 6+ game compatibility, watchdog, MangoHud handling |

## Platform support

- **Target:** Linux with Wayland compositor
- **GPU:** NVIDIA (GeForce, via NVML)
- **Vulkan implementations:** DXVK (DX11 games), vkd3d-proton (DX12 games), native Vulkan
- **Compositors:** KDE Plasma, GNOME, Hyprland, Sway (any compositor supporting XDG Desktop Portal GlobalShortcuts)
- **X11:** Out of scope
