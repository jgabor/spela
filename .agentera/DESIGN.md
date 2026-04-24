# Spela Design System

<!-- Maintained by visualisera. Created: 2026-04-01 -->

## Philosophy

Spela's visual identity is a **HUD for your gaming rig** — not a dashboard you glance at,
but an instrument panel you master. Every pixel of terminal real estate earns its keep by
showing live data, actionable controls, or navigational context.

Three principles drive the aesthetic:

**Precision over decoration.** No ornamental elements. Borders exist to delineate focus, not
to look pretty. Color exists to encode meaning, not to create ambiance. Every visual choice
answers the question: "what information does this communicate?"

**Density over whitespace.** Gamers are used to HUDs that pack information into constrained
spaces. Spela respects that instinct. Sparklines next to numbers, gauges instead of raw
values, inline indicators instead of modal alerts. Information should be *felt* before it
is *read*.

**Thermal as metaphor.** Temperature is the universal language of GPU monitoring — cold blues
for idle, warm ambers for load, angry reds for throttle. This gradient extends beyond
temperature: it becomes the visual vocabulary for any metric that has a healthy/warning/critical
range. The user's eye learns one color language and applies it everywhere.

The overall impression in 3 seconds: **control room at night** — dark, precise, alive with
data, purple-accented, unapologetically technical.

## Colors

### Brand Palette

Derived from the spela logo. The purple-to-pink gradient is the visual signature. Royal Blue
provides a cooler counterpoint for secondary elements and data visualization.

<!-- design:colors-brand -->
```yaml
# Brand identity — the spela signature
brand-primary:       "#9C41AA"  # Amethyst (ANSI 133) — primary accent, focused borders, titles
brand-secondary:     "#566EDC"  # Royal Blue (ANSI 69) — secondary accent, DLL indicators, links
brand-accent:        "#FA76C2"  # Pink Carnation (ANSI 212) — highlights, sparkline peaks, notifications
brand-dark:          "#200748"  # Dark Amethyst (ANSI 53) — selection backgrounds, deep accents
brand-deep:          "#64297D"  # Velvet Orchid (ANSI 91) — unfocused borders, subtle accents
```

### Surface Palette

Three tiers of background depth create visual layering without shadows.

<!-- design:colors-surface -->
```yaml
# Surfaces — depth via darkness, not shadow
surface-base:        "#000000"  # ANSI 16 — terminal background, deepest layer
surface-raised:      "#1C1C1C"  # ANSI 234 — panels, cards, raised content
surface-overlay:     "#262626"  # ANSI 235 — modals, popups, floating layers
surface-highlight:   "#303030"  # ANSI 236 — hover states, subtle emphasis
```

### Text Palette

Four tiers of text prominence. Primary for content, secondary for labels, dim for hints,
muted for disabled/decorative.

<!-- design:colors-text -->
```yaml
text-primary:        "#F5F5FD"  # ANSI 255 — Ghost White, primary content
text-secondary:      "#A8A8B3"  # ANSI 145 — labels, metadata
text-dim:            "#6C6C7A"  # ANSI 242 — hints, disabled hints, timestamps
text-muted:          "#4A4A55"  # ANSI 239 — decorative borders, deep background text
```

### Semantic Palette

Status colors that carry universal meaning. Used consistently across all contexts — never
for decoration.

<!-- design:colors-semantic -->
```yaml
# Status — fixed meaning, never decorative
status-success:      "#87D787"  # ANSI 114 — operation complete, healthy state
status-error:        "#FF5F5F"  # ANSI 203 — failure, critical alert, destructive action
status-warning:      "#FFAF5F"  # ANSI 215 — caution, elevated state, needs attention
status-info:         "#5F87FF"  # ANSI 69 — informational, neutral notification
```

### Thermal Gradient

The signature visual language. A continuous gradient from cool to critical that applies to
any metric with a range: temperature, utilization, power draw, fan speed.

<!-- design:colors-thermal -->
```yaml
# Thermal — the HUD's visual pulse. Continuous gradient for ranged metrics.
thermal-cold:        "#5F87FF"  # ANSI 69  — idle, below 30% of range
thermal-cool:        "#5FAFFF"  # ANSI 75  — light load, 30-45%
thermal-warm:        "#87D787"  # ANSI 114 — normal operating range, 45-65%
thermal-hot:         "#FFD75F"  # ANSI 221 — elevated, 65-80%
thermal-critical:    "#FF875F"  # ANSI 209 — high, 80-90%
thermal-throttle:    "#FF5F5F"  # ANSI 203 — danger zone, 90%+

# Gradient stops for lipgloss.Blend1D:
# [cold, cool, warm, hot, critical, throttle] at [0.0, 0.30, 0.50, 0.70, 0.85, 1.0]
```

### Metric-Specific Tokens

Purpose-built colors for recurring data types in the GPU monitoring context.

<!-- design:colors-metrics -->
```yaml
# GPU metrics
metric-gpu-temp:     thermal    # use thermal gradient based on value/range
metric-gpu-util:     thermal    # use thermal gradient
metric-gpu-power:    thermal    # use thermal gradient (power_draw / power_limit)
metric-gpu-vram:     thermal    # use thermal gradient (used / total)
metric-gpu-fan:      thermal    # use thermal gradient
metric-gpu-clock:    "#5FAFFF"  # ANSI 75 — static, informational

# CPU metrics
metric-cpu-util:     thermal    # use thermal gradient
metric-cpu-freq:     "#AF87FF"  # ANSI 141 — purple tone, differentiates from GPU
metric-ram:          thermal    # use thermal gradient

# DLL indicators
metric-dll-current:  "#566EDC"  # brand-secondary — installed version
metric-dll-update:   "#FFD75F"  # thermal-hot — update available
metric-dll-missing:  "#6C6C7A"  # text-dim — not installed
```

### Neon Dark Theme

Spela has one TUI theme: dark terminal surfaces with neon semantic accents. Magenta marks
profile overrides and stale deployment cells. Cyan marks focus and active navigation.

<!-- design:theme -->
```yaml
theme:
  mode: single-neon-dark
  background:        surface-base
  foreground:        text-primary
  foreground-muted:  text-dim
  border:            brand-deep
  accent-override:   "#FA76C2"  # magenta — overrides, stale DLL deployments
  accent-focus:      "#5FAFFF"  # cyan — focused rail item, active field, active pane
  accent-brand:      brand-primary
```

## Typography

Terminal monospace is the only typeface. Hierarchy is created through weight (bold), color,
and Unicode symbols — never through font size (impossible in terminals) or font choice.

<!-- design:typography -->
```yaml
# Text hierarchy — monospace only, differentiated by style and color
heading:
  weight: bold
  color: brand-primary
  decoration: none
  usage: panel titles, section headers, resource labels

subheading:
  weight: bold
  color: text-primary
  decoration: none
  usage: group titles within panels, field group names

label:
  weight: normal
  color: text-secondary
  decoration: none
  usage: metric labels ("GPU:", "VRAM:"), field names, column headers

value:
  weight: normal
  color: text-primary
  decoration: none
  usage: metric values, field values, data content

value-emphasis:
  weight: bold
  color: text-primary
  decoration: none
  usage: selected values, active states, current selection

hint:
  weight: normal
  color: text-dim
  decoration: none
  usage: keybinding hints, placeholder text, secondary information

disabled:
  weight: normal
  color: text-muted
  decoration: none
  usage: unavailable options, coming soon features, inactive elements
```

## Spacing

Terminal spacing is measured in character cells. The base unit is 1 character. All spacing
follows a consistent scale to prevent visual drift.

<!-- design:spacing -->
```yaml
# Character-cell spacing scale
space-0:    0     # no space — tight inline elements
space-1:    1     # minimal — between label and value ("GPU: 72C")
space-2:    2     # standard — between metric groups on same line
space-4:    4     # comfortable — between sections within a panel
space-pad:  1     # panel interior padding (left/right)
space-gap:  0     # gap between adjacent panels (border-to-border)

# Vertical spacing (lines)
line-0:     0     # no vertical gap — consecutive fields in same group
line-1:     1     # standard — between groups within a panel
line-2:     2     # section break — between major sections
```

## Borders

Borders are functional, not decorative. They delineate panels and encode focus state.

<!-- design:borders -->
```yaml
# Border styles and their semantic meaning
border-panel:
  style: rounded          # lipgloss.RoundedBorder()
  color-focused: border-focus
  color-unfocused: border-default
  color-inactive: text-muted
  usage: main panels (sidebar, content, header)

border-group:
  style: rounded          # lipgloss.RoundedBorder()
  color-focused: brand-primary
  color-unfocused: text-dim
  usage: profile widget groups, settings sections

border-modal:
  style: rounded
  color: brand-primary
  usage: floating overlays, dialogs, modals

border-separator:
  style: normal           # lipgloss.NormalBorder(), bottom only
  color: text-muted
  usage: header bottom edge, section dividers
```

## Components

### Layout Shell

```
┌─────────────────────────────────────────────────┐
│  Header: Logo + Live Metrics + Sparklines       │
│─────────────────────────────────────────────────│
│ [1] Games                                        │
│ [2] DLLs                                         │
│ [3] Defaults                                     │
│ [4] Metrics                                      │
│          │                                       │
│ Left Rail│  Resource Pane                        │
│  fixed   │  remaining width                      │
│          │                                       │
│          │                                       │
├─────────────────────────────────────────────────┤
│  Message Bar (transient, 1 line)                │
├─────────────────────────────────────────────────┤
│  Status Bar: Breadcrumbs · Context Keys · Mode  │
└─────────────────────────────────────────────────┘
```

<!-- design:components-layout -->
```yaml
header:
  height: 7           # fixed — logo (6) + border (1)
  content: logo (left), metrics with sparklines (right)
  border: border-separator (bottom only)

sidebar:
  width-min: 18
  width-max: 28
  border: border-panel
  title-format: "Resources"

content:
  width: remaining after sidebar
  border: border-panel
  title-format: "{active-resource}"
  resources: [Games, DLLs, Defaults, Metrics]

message-bar:
  height: 1
  position: below main panels
  types: [info, success, error]
  auto-clear: 5 seconds

status-bar:
  height: 1
  position: bottom
  layout: "[breadcrumbs]  ·  [context-keys]  ·  [mode]"
  breadcrumb-separator: " > "
  breadcrumb-active: brand-primary, bold
  breadcrumb-trail: text-dim
  key-format: "{key}:{action}"
  key-separator: "  "
```

### Sparkline

Inline braille or eighth-block character rendering of time-series data. The visual heartbeat
of the HUD.

<!-- design:components-sparkline -->
```yaml
sparkline:
  renderer: eighth-block    # ▁▂▃▄▅▆▇█ — 8 vertical levels per cell
  width: 20                 # characters — ~40s of history at 2s interval
  color: thermal-gradient   # value-mapped through thermal palette
  empty-char: "▁"           # minimum level shown as baseline, not blank
  overflow: "█"             # maximum level — capped, not clipped
  direction: left-to-right  # newest value on right
  usage: inline after metric values in header

sparkline-braille:
  renderer: braille         # U+2800-U+28FF — 8 pseudo-pixels per cell
  width: 20                 # characters — higher resolution than eighth-block
  color: single             # one color (brand-secondary), no gradient
  usage: optional high-res mode, toggled per preference
```

### Gauge

Inline block-character progress bar. Replaces raw fractions with visual proportion.

<!-- design:components-gauge -->
```yaml
gauge:
  format: "[{filled}{empty}] {percent}%"
  filled-char: "█"
  empty-char: "░"
  width: 12                 # characters for the bar portion
  color-filled: thermal-gradient   # value-mapped through thermal palette
  color-empty: text-muted
  usage: VRAM usage, power draw ratio, fan speed

gauge-mini:
  format: "{filled}{empty}"
  width: 8
  usage: compact mode inline gauges, table cells
```

### Resource Navigation

Permanent left rail with four peer resources. `tab` moves focus into the active pane;
`1`-`4` switch resources globally. Steam `%command%` remains the launch path.

<!-- design:components-navigation -->
```yaml
resource-rail:
  breadcrumb-separator: " > "
  breadcrumb-root: "spela"
  breadcrumb-style-active: bold, brand-primary
  breadcrumb-style-trail: text-dim
  pop-key: "esc"

  # Example breadcrumb trails:
  # spela > Games > Cyberpunk 2077
  # spela > DLLs > Deployment
  # spela > Defaults
  # spela > Metrics
  # spela > Settings

jump-keys:
  games: "1"
  dlls: "2"
  defaults: "3"
  metrics: "4"
  format: "[{key}]{label}"
  style-key: accent-focus, bold
  style-label: text-primary
```

### Context Key Bar

Dynamic keybinding display in the status bar. Shows only what's available *right now*.

<!-- design:components-context-bar -->
```yaml
context-bar:
  position: status-bar center
  key-format: "{key}:{action}"
  key-style: brand-accent
  action-style: text-secondary
  separator: "  "
  disabled-format: "{key}:{action} ({reason})"
  disabled-style: text-muted
  truncation: ellipsis     # "..." when bar exceeds width
  max-items: 8             # prioritize by frequency of use

  # Destructive actions get warning color
  destructive-key-style: status-warning
```

### Modal / Dialog

Floating overlay with compositor-based stacking. Replaces the single-dialog guard.

<!-- design:components-modal -->
```yaml
modal:
  border: border-modal
  background: surface-overlay
  width-ratio: 0.60         # of terminal width
  width-min: 40
  width-max: 70
  position: center
  cascade-offset-x: 2       # stacked modals offset right
  cascade-offset-y: 1       # stacked modals offset down
  title-style: heading
  shadow: none               # no shadows in terminals — depth via border color

confirmation-dialog:
  border: border-modal
  border-color: status-warning    # orange border for destructive actions
  width: 45
  buttons: "[Y]es  [N]o"
  button-focus: bold, brand-primary
  button-default: text-secondary
```

### Sidebar List

Game list with indicators, search, and multi-select.

<!-- design:components-sidebar -->
```yaml
sidebar-item:
  format: "{indicators} {name}"
  style-normal: text-primary
  style-selected: bold, selection-fg on selection-bg
  style-focused: bold, text-primary        # cursor on this item, sidebar focused
  style-unfocused: text-secondary           # cursor on this item, sidebar not focused

  indicators:
    has-dlls: "●"
    has-dlls-color: brand-secondary
    has-profile: "◆"
    has-profile-color: brand-secondary
    has-update: "↑"
    has-update-color: status-warning
    multi-selected: "✓"
    multi-selected-color: brand-accent

search-bar:
  prompt: "/"
  prompt-style: brand-accent
  input-style: text-primary
  position: top of sidebar (replaces first item)
  
filter-legend:
  position: bottom of sidebar, above status bar
  format: "{active-filters} · {sort-mode} · {count} games"
  style: text-dim
```

### Profile Detail

Single-column grouped renderer shared by Games and Defaults. Games show live inheritance;
Defaults show root values without inheritance markers.

<!-- design:components-profile -->
```yaml
profile-group:
  border: border-group
  title-style: subheading
  padding: 1 horizontal

profile-field:
  format: "{label}: {value}"
  label-style: label
  value-style: value
  value-inherited-style: text-dim
  value-override-style: text-primary
  value-selected-style: value-emphasis, accent-focus background
  disabled-format: "{label}: {value} (coming soon)"
  disabled-style: disabled
  override-indicator: "◆"
  override-color: accent-override
  stale-dll-color: accent-override

profile-layout:
  columns: 1
  order: [proton, dlss, gpu, cpu, overlay]
  group-gap: 1 line
```

## Indicators & Icons

Consistent iconography across all components. Unicode symbols only — no emoji.

<!-- design:indicators -->
```yaml
# State indicators
state-modified:    "●"    # unsaved changes
state-active:      "▶"    # running, in progress
state-disabled:    "○"    # inactive, unavailable
state-loading:     "⟳"    # async operation in flight
state-error:       "✗"    # failure
state-success:     "✓"    # complete, healthy

# Metric indicators
alert-critical:    "✗"    # red — throttle, failure
alert-warning:     "⚠"    # amber — elevated, attention
alert-info:        "●"    # blue — informational

# Navigation
nav-breadcrumb:    ">"    # breadcrumb separator
nav-collapse:      "▸"    # collapsed section
nav-expand:        "▾"    # expanded section

# DLL status
dll-installed:     "●"    # blue — version present
dll-update:        "↑"    # amber — update available
dll-missing:       "─"    # dim — not installed
dll-backed-up:     "◆"    # blue — backup exists
```

## Animation Language

Terminal animations are state-driven, not frame-driven. Use bubbletea tick commands
for time-based transitions.

<!-- design:animation -->
```yaml
# All durations in milliseconds
flash-success:
  type: color-pulse
  color: status-success
  duration: 800
  pattern: bright → fade to normal over 4 ticks
  usage: profile saved, DLL installed, operation complete

flash-error:
  type: color-pulse
  color: status-error
  duration: 1200
  pattern: bright → fade to normal over 6 ticks
  usage: operation failed, validation error

alert-pulse:
  type: blink
  interval: 1000
  pattern: bright ↔ dim alternating
  max-cycles: 5
  usage: thermal throttle indicator, critical alerts

loading-spinner:
  frames: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]
  interval: 80
  color: brand-accent
  usage: async operations (DLL download, profile apply, resource update)

transition-modal:
  type: instant
  usage: modal open/close — no animation, instant render
  reason: perceived speed > visual smoothness in a power-user tool
```

## Information Density Modes

Three modes toggled at runtime. Each mode adjusts which panels and details are visible,
not the visual language itself. The design tokens remain constant.

<!-- design:density -->
```yaml
standard:
  header: visible (logo + metrics + sparklines)
  sidebar: visible (resource rail)
  content: visible (active resource pane)
  status-bar: visible
  sparklines: visible
  gauges: visible
  description: default layout — full information display

compact:
  header: condensed (metrics only, no logo, 2 lines)
  sidebar: visible (narrow resource rail)
  content: visible (active resource pane)
  status-bar: visible
  sparklines: hidden (values only)
  gauges: mini variant
  toggle-key: "F5"
  description: more vertical space for content, metrics still visible

focused:
  header: hidden
  sidebar: hidden
  content: fullscreen (100% width, current resource fills screen)
  status-bar: visible (minimal — breadcrumb + esc hint only)
  sparklines: hidden
  gauges: hidden
  toggle-key: "F11"
  description: maximum content area — profile editing or DLL management at full size
```

## Constraints

Rules that prevent visual drift. Every constraint has a reason.

<!-- design:constraints -->
```yaml
aesthetic:
  - property: emoji
    rule: prohibited
    reason: "Variable width, inconsistent rendering across terminals. Unicode symbols only."
  
  - property: box-shadow
    rule: prohibited
    reason: "Terminals have no shadow. Depth via border color intensity."
  
  - property: gradient-text
    rule: prohibited-except-sparklines
    reason: "Gradients on text reduce readability. Only sparklines and gauges use thermal gradient."
  
  - property: blinking-text
    rule: prohibited-except-critical-alerts
    reason: "Blink is hostile to usability. Reserved for genuine thermal throttle alerts."
  
  - property: color-without-semantic-meaning
    rule: prohibited
    reason: "Every color must answer 'what does this communicate?' Brand colors identify spela. Thermal colors encode state. Text colors encode hierarchy. No color is decorative."

structural:
  - pattern: hardcoded-ansi-color
    rule: prohibited
    scope: all TUI code
    reason: "All colors via theme tokens. Hardcoded values break the neon dark contract."
  
  - pattern: hardcoded-dimensions
    rule: prohibited-except-min-max
    scope: layout calculations
    reason: "Use ratios and constraints. Hardcoded widths break on terminal resize."
  
  - pattern: fmt-sprintf-for-styled-text
    rule: prohibited
    scope: View() functions
    reason: "Use lipgloss styles for all colored/formatted text. Sprintf bypasses theming."
  
  - pattern: nested-dialog-without-stack
    rule: prohibited
    scope: modal management
    reason: "All modals go through the compositor stack. No ad-hoc activeDialog guards."
  
  - pattern: sleep-in-animation
    rule: prohibited
    scope: animation
    reason: "Use tea.Tick for all time-based effects. Sleep blocks the event loop."
```
