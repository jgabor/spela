# Plan: TUI Visual Transformation

<!-- Level: full | Created: 2026-04-01 | Status: active -->
<!-- Reviewed: 2026-04-01 | Critic issues: 11 found, 9 addressed, 2 dismissed -->

## What

Transform spela's TUI from a functional config editor into a polished gaming HUD. Replace
static metric numbers with live sparklines and thermally-colored gauges, restructure the
content pane with tabs, replace binary focus with a navigation stack, add a context-sensitive
keybinding bar, adopt lipgloss compositor for modal stacking, and introduce information
density modes.

## Why

Spela's TUI is architecturally sound but visually static. The header shows plain text
numbers (`GPU: 72C 85% 280W`), the content pane stacks sections vertically with fixed
heights that clip on small terminals, the binary focus system (`FocusSidebar`/`FocusContent`)
can't represent deep drill-down, and the modal system uses an ad-hoc `activeDialog != nil`
guard that blocks extension. These limitations compound — they make spela feel like a config
file editor rather than the real-time gaming control surface described in VISION.md.

The INSPIRERA analysis of lazygit, k9s, btop, and the Charm v2 ecosystem identified
concrete patterns that address each limitation. The DESIGN.md visual identity system
codifies the target aesthetic. This plan decomposes the transformation into ordered tasks
that each deliver a visible improvement while maintaining a working TUI at every commit.

## Constraints

- TUI must remain functional after every task — no multi-task broken states
- All visual tokens from DESIGN.md — never hardcode ANSI colors directly
- Tests for new pure-function components (sparkline, gauge, thermal gradient)
- Layout.go's Update() must shrink, not grow — each context owns its message handling
- No changes to non-TUI packages (gpu, cpu, profile, etc.) — this is a UI-only transformation
- No emoji — Unicode symbols only per DESIGN.md constraints
- No existing TUI tests exist — regression detection for existing behavior is visual. New
  pure-function components (Tasks 1, 2) get unit tests.

## Scope

**In**: header sparklines, block gauges, thermal gradients, tab-based content, navigation
stack, breadcrumbs, context-sensitive keybinding bar, compositor modals, density modes,
jump-key panel titles

**Out**: GUI (Wails/Svelte) changes, overlay changes, new CLI commands, profile field
additions, new features beyond visual transformation, baseline characterization tests for
existing TUI behavior

**Deferred**: mouse click zones via compositor Hit(), animated transitions (flash-success,
alert-pulse), huh-based form replacement for profile widget, glamour markdown help

## Design

Eight tasks across four layers. Foundation (thermal + renderers) can proceed in parallel
with the structural pivot (navigation stack). The component layer (tabs, context bar,
header, modals) builds on both. Density modes and jump-keys integrate everything.

The navigation stack is the architectural centerpiece — it replaces binary focus routing with
a LIFO context stack that unifies panel focus, tab switching, and layout-level modal opening
into a single mechanism. Layout.go's Update() shrinks because each context handles its own
messages. Content-level modal patterns (DLL install state machine, DLSS preset modal, batch
overlay) are migrated to the compositor in Task 7 — Task 3 only handles layout-level routing.

Content.go's vertical stacking (game info + DLLs + profile at fixed heights) must decompose
into per-tab sub-models in Task 4. Each tab owns its Update()/View() and gets the full panel
height. The existing `ContentModel` becomes a tab coordinator that routes messages to the
active tab's sub-model while preserving state across tab switches.

The Theme struct expands in Task 1 from 12 fields to the full DESIGN.md token set (surface
palette, text hierarchy, thermal gradient, metric tokens). All downstream tasks build on
these tokens.

Modules affected: `internal/tui/` exclusively (12 existing files, 3-4 new files).

## Tasks

### Task 1: Thermal gradient system and expanded theme tokens

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN a metric value and its min/max range WHEN rendered THEN the color shifts continuously from cool-blue through green to amber to red
▸ GIVEN a theme switch at runtime WHEN the user changes theme THEN all thermal-colored metrics update immediately without restart
▸ GIVEN the three themes (default, dark, light) WHEN each is active THEN brand colors remain identical and only surface/text tokens change
▸ GIVEN the Theme struct WHEN inspected THEN it contains all DESIGN.md token groups: surface palette, text hierarchy, thermal stops, and metric-specific tokens

### Task 2: Sparkline and gauge renderers

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN a buffer of 20 metric samples WHEN rendered as a sparkline THEN the output is exactly 20 characters wide using eighth-block characters (▁▂▃▄▅▆▇█)
▸ GIVEN a gauge at 53% with width 12 WHEN rendered THEN the output shows filled and empty block characters proportional to the value with percentage label
▸ GIVEN an empty buffer with zero samples WHEN rendered as a sparkline THEN the output shows baseline characters (▁) at the specified width, not blank
▸ GIVEN a value exceeding the maximum range WHEN rendered THEN the output caps at full block (█) without panicking

### Task 3: Navigation stack replacing binary focus

**Depends on**: none
**Status**: □ pending
**Scope note**: This task migrates layout-level focus routing only (FocusSidebar/FocusContent
enum, activeDialog guard, help overlay, batch menu intercept). Content-level modal patterns
(DLL install state machine, DLSS preset modal, profileWidget.Editing()) are migrated to the
compositor in Task 7.
**Acceptance**:
▸ GIVEN the TUI starts WHEN the sidebar is visible THEN a breadcrumb trail shows "spela > Games" in the status bar
▸ GIVEN the user selects a game and presses enter WHEN the content pane focuses THEN the breadcrumb updates to "spela > Games > [Game Name]"
▸ GIVEN a layout-level modal is open on top of content WHEN the user presses Escape THEN the modal closes and focus returns to the previous context without losing game selection
▸ GIVEN three layout contexts are stacked (sidebar → game detail → options modal) WHEN Escape is pressed twice THEN focus returns to sidebar with breadcrumb matching
▸ GIVEN a message intended for the sidebar context WHEN the content pane is focused THEN the sidebar does not receive it — each context handles only its own messages

### Task 4: Tab-based content views

**Depends on**: Task 3
**Status**: □ pending
**Scope note**: ContentModel decomposes into a tab coordinator with per-tab sub-models.
Each tab sub-model owns its Update()/View(). The coordinator routes messages to the active
tab and preserves inactive tab state.
**Acceptance**:
▸ GIVEN a game is selected WHEN the content pane shows THEN a tab bar displays "[2]DLLs [3]Profile [4]Launch" with the active tab visually distinguished
▸ GIVEN the DLLs tab is active WHEN the user presses 3 THEN the view switches to the Profile tab and the tab bar updates
▸ GIVEN a small terminal (80x24) WHEN viewing any content tab THEN each tab gets the full panel height instead of sharing vertical space
▸ GIVEN the Profile tab is active with unsaved changes WHEN switching to DLLs tab and back THEN the unsaved changes are preserved

### Task 5: Context-sensitive keybinding bar

**Depends on**: Task 3
**Status**: □ pending
**Scope note**: Requires a keybinding metadata model — each keybinding carries an
active/dimmed/hidden state and an optional reason string. The navigation stack context
provides which keybindings are relevant; domain state (e.g., "has backup") provides the
enable/disable condition.
**Acceptance**:
▸ GIVEN the sidebar is focused WHEN the status bar renders THEN it shows only keybindings valid for sidebar context (navigate, search, filter, sort)
▸ GIVEN a game with no DLL backup WHEN viewing the content pane THEN the restore keybinding appears dimmed with reason "(no backup)" instead of being hidden
▸ GIVEN the user opens a modal WHEN the status bar renders THEN it shows only keybindings valid for the modal (navigate, select, cancel)
▸ GIVEN a narrow terminal WHEN the keybinding bar would exceed available width THEN it truncates with "..." and prioritizes the most common actions

### Task 6: Header metrics with sparklines and gauges

**Depends on**: Task 1, Task 2
**Status**: □ pending
**Scope note**: A rolling sample buffer (ring buffer) stores recent metric values inside the
TUI package. The buffer is populated on each 2-second metrics tick. At startup, the buffer
is empty and sparklines render baseline characters until samples accumulate.
**Acceptance**:
▸ GIVEN the TUI is running for 40 seconds WHEN viewing the header THEN GPU temperature shows the current value followed by a 20-character sparkline of recent history
▸ GIVEN VRAM usage at 8.2/12.0 GB WHEN viewing the header THEN a block gauge shows proportional fill alongside the text value
▸ GIVEN GPU temperature at 85°C (in the warning range) WHEN viewing the header THEN both the temperature number and its sparkline render in amber/orange thermal color
▸ GIVEN no GPU is detected WHEN viewing the header THEN metrics show "N/A" without sparklines or gauges and without errors
▸ GIVEN the TUI just started (0 samples) WHEN viewing the header THEN sparklines show baseline characters at full width, not blank space

### Task 7: Compositor-based modal system

**Depends on**: Task 3
**Status**: □ pending
**Scope note**: Migrates all three modal patterns to the compositor: (1) layout-level dialog
guard from Task 3's navigation stack, (2) content-level modals (DLL install state machine,
DLSS preset modal), and (3) the batch overlay. All become compositor layers with z-ordering.
**Acceptance**:
▸ GIVEN a modal is open WHEN rendered THEN it appears as a centered overlay with the main content visible but inactive behind it
▸ GIVEN modal A is open WHEN another modal triggers (e.g., DLSS preset from within options) THEN modal B appears offset from modal A, showing the stack visually
▸ GIVEN a modal is open WHEN the terminal resizes THEN the modal repositions to remain centered without rendering artifacts
▸ GIVEN the transformation is complete WHEN inspecting the codebase THEN no ad-hoc dialog guard patterns remain — all modals use the compositor

### Task 8: Information density modes and jump-key panel titles

**Depends on**: Task 3, Task 4, Task 6, Task 7
**Status**: □ pending
**Acceptance**:
▸ GIVEN standard density mode WHEN viewing the TUI THEN all panels are visible: header with logo and sparklines, sidebar, tabbed content, status bar
▸ GIVEN the user presses the compact mode toggle WHEN the layout updates THEN the header condenses to metrics-only (no logo) and sparklines hide, reclaiming vertical space
▸ GIVEN the user presses the focused mode toggle WHEN the layout updates THEN only the current content tab is visible at full screen width with a minimal status bar
▸ GIVEN the sidebar panel WHEN rendered THEN its border title shows "[1] Games" where "1" is styled as a jump key
▸ GIVEN any density mode WHEN the user presses a jump key (1-4) THEN focus moves to the corresponding panel or tab regardless of current mode

## Overall Acceptance

▸ GIVEN the complete TUI transformation WHEN a user opens spela THEN the header shows live sparklines and thermally-colored metrics, the content uses tabs, breadcrumbs show navigation context, and the keybinding bar updates dynamically
▸ GIVEN a terminal at 80x24 minimum WHEN all features are active THEN the layout renders without clipping, overflow, or visual artifacts
▸ GIVEN the existing test suite WHEN run after transformation THEN all tests pass with no regressions
▸ GIVEN any point during the 8-task execution WHEN the TUI is launched THEN it is functional — no task leaves the TUI in a broken intermediate state

## Surprises
