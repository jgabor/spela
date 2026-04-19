# Decisions

## Decision 1 · 2026-04-19

**Question**: How to overhaul the TUI (and then the GUI) to fix the accreted UX debt (duplicate model presets, no profile reset, unclear default-vs-game relationship, grid misalignment, sequential navigation, Launch-as-tab) and refresh the theme.

**Context**: Ground-up rethink using ASCII mockups. Keystone reframe: 99% of launches go through Steam's `%command%` (`spela %command%`), not the TUI, so the TUI is not a launcher.

**Alternatives** (rejected per axis):

- Spine: launcher-first / tune-then-launch / resource-centric
- Profile model: live inheritance / snapshot copy / explicit fields only
- Field layout: grouped single column / responsive 2D grid
- Theme: terminal brutalist / neon-accent dark / warm phosphor
- Shell nav: left rail / top tabs / command palette only
- DLL surface: library-with-per-game / per-game only / master-detail split / two-section

**Choice**: TUI becomes a resource-centric configuration and inspection console.

- Shell: permanent left rail (games · dlls · defaults · metrics), 1-4 hotkeys, right side fills with the active resource view.
- Launch: removed from the TUI entirely. `spela %command%` in Steam is the only launch path.
- Profile model: live inheritance. Each field is inherited (tracks defaults) or overridden (pinned). `r` resets field, `shift+r` resets whole profile, `p` pins current value.
- Field layout: single-column grouped by subsystem (proton, dlss, gpu, cpu, overlay), groups always visible (no collapse), j/k navigates across the whole list.
- Theme: neon-accent dark. Magenta (#ff5fd2) = override, cyan (#5ff0ff) = focus. Muted greys elsewhere (fg #e8e8f0, fg-muted #6a6a80, border #202030, bg #0a0a14). Tokens only; no hardcoded hex in components.
- DLLs resource: two always-visible sections. Library (inventory of types, latest + cached versions) + Deployment (games × DLL types table with per-cell version and stale markers). Per-game DLL state also surfaces inside the Game detail view.

**Reasoning**: Launcher-removal is the keystone. Once the TUI is not a launcher, the Launch-tab bug disappears, "resource-centric" becomes the obvious spine, and "live inheritance" becomes the obvious default-vs-game model. Grouped-column-no-collapse eliminates the 2D-nav problem without losing scannability. Two accents let the theme encode meaning, not ornament. Two-section DLL view separates inventory from deployment so neither degenerates at 10+ games.

**Confidence**: firm
**Feeds into**: standalone (decomposition via /planera; GUI parity follows TUI)
