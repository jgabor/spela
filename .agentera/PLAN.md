# Plan: TUI ground-up redesign

<!-- Level: full | Created: 2026-04-19 | Status: active -->
<!-- Reviewed: 2026-04-19 | Critic issues: 7 found, 7 addressed, 0 dismissed -->

## What

Rebuild the TUI as a resource-centric configuration and inspection console. Replace the current sidebar-of-games + tabbed-content + Launch-tab shell with a left rail of four peer resources (games, dlls, defaults, metrics) plus a single-column, always-expanded, inheritance-aware profile detail view. Refresh the theme to a single neon-accent dark palette with two semantic accents.

## Why

The current TUI carries accreted UX debt: a Launch tab that 99% of users do not use (launches go through Steam's `%command%`), an opaque default-vs-game profile relationship, no profile reset, a misaligned grid with sequential navigation, and duplicate DLSS model presets. A coherent redesign resolves all of these at once and unblocks the GUI parity work that follows in a later plan.

## Constraints

- Launch workflows stay in the CLI (`spela %command%`); the TUI never launches a game again.
- Charm stack only (bubbletea v2, lipgloss v2). Keyboard-first. Sentence case. No em-dashes.
- Existing profile YAML files must continue to load. Migration policy: fields whose values equal the current default are stripped (treated as inherited); fields whose values differ become overridden.
- Existing 99-test TUI state-machine suite is triaged and realigned inside Task 3, not silently rewritten later.
- No new dependencies. GUI untouched in this plan; GUI parity is a follow-on plan.

## Scope

**In**: full TUI rewrite (shell + all resource views), profile inheritance primitives in `internal/profile/`, single-theme token refresh in `internal/tui/styles.go` plus CLI helpers, CLI `show`/`reset` inheritance-aware output, duplicate-presets bug fix.

**Out**: GUI changes, overlay changes, new metrics, new DLL types, CLI command surface changes beyond inheritance-aware display and a `reset` verb.

**Deferred**: community profiles, per-hardware recommendations, command palette (`:` reserved in keymap but not implemented).

## Design

Four layers, bottom to top.

**Inheritance layer** (`internal/profile/`): game profiles store only overridden fields; unset fields resolve from defaults at apply time. API: `IsOverridden(field)`, `Reset(field)`, `ResetAll()`, `ResolveForApply(defaults)`. Load-time migration strips fields equal to current defaults.

**Theme layer** (`internal/tui/styles.go` + CLI helpers): single theme. Tokens: `bg`, `fg`, `fg-muted` (inherited rows), `accent-override` (magenta #ff5fd2), `accent-focus` (cyan #5ff0ff), `border`. Config `theme` variant selector dropped.

**Shell layer** (`internal/tui/layout.go` + resource router): permanent left rail of four resources with 1-4 hotkeys and j/k; right pane renders the focused resource. Tab concept and Launch surface removed everywhere.

**Resource views**: Games (list + inheritance-aware detail, grouped single-column fields always expanded), DLLs (library section + deployment table), Defaults (reuses detail renderer as root — no markers), Metrics (relocates existing thermal/sparkline widgets).

Risks the plan accepts: Task 5 is the densest cycle (inheritance rendering + three keybindings + one bug fix); the 99-test suite will require substantial realignment before Task 4; the YAML migration policy must handle zero-valued fields correctly or live inheritance is neutered on day one.

## Tasks

### Task 1: Theme refresh to neon-accent dark

**Depends on**: none
**Status**: ■ complete
**Acceptance**:
▸ GIVEN the TUI is launched THEN a single theme is active with the neon-accent dark palette (magenta override, cyan focus, muted greys).
▸ GIVEN any config file with `theme: dark` or `theme: light` WHEN the TUI loads THEN it resolves to the single theme without error and does not re-persist the legacy value.
▸ GIVEN the token set WHEN inspected THEN it includes distinct tokens for `fg-muted` (inherited rows), `accent-override`, and `accent-focus`, so Tasks 4-5 do not need to extend it.
▸ GIVEN CLI helpers (CLIPrimary/CLISecondary/etc.) WHEN rendered THEN they reference the new tokens and no legacy amethyst/royal-blue colors remain.
▸ GIVEN theme-related TUI tests WHEN run THEN they pass against the new tokens (1 pass + 1 fail per new theme helper).

### Task 2: Profile inheritance primitives + CLI inheritance output

**Depends on**: none
**Status**: □ pending
**Acceptance**:
▸ GIVEN a default profile and a game profile WHEN a field is unset on the game profile THEN `ResolveForApply` returns the default's value.
▸ GIVEN a game profile with a pinned field WHEN the default changes THEN `ResolveForApply` returns the game profile's pinned value (override persists through default edits).
▸ GIVEN a game profile field that is overridden WHEN `Reset(field)` is called THEN `IsOverridden(field)` returns false and `ResolveForApply` falls back to the default's value.
▸ GIVEN any existing profile YAML WHEN loaded THEN fields whose values equal the current defaults are stripped to inherited; fields whose values differ are preserved as overridden; no profile field is silently discarded.
▸ GIVEN `spela proton show` / `spela dlss show` / equivalents WHEN run against a game profile THEN each field's output clearly marks whether the value is inherited or overridden.
▸ GIVEN `spela proton reset <field>` (or equivalent per-subsystem verb) WHEN run THEN the named field becomes inherited and subsequent `show` reflects that.
▸ GIVEN the profile package WHEN tested THEN 1 pass + 1 fail per testable unit covers `IsOverridden`, `Reset`, `ResetAll`, `ResolveForApply`, and the load-time migration.

### Task 3: Shell redesign with left rail, keymap audit, test triage

**Depends on**: Task 1
**Status**: □ pending
**Acceptance**:
▸ GIVEN the TUI starts THEN the shell shows a permanent left rail listing exactly four resources (games, dlls, defaults, metrics) with 1-4 hotkeys and j/k focus movement.
▸ GIVEN the left rail is focused WHEN the user presses 1-4, or j/k followed by enter THEN the right pane swaps to the selected resource without losing rail focus.
▸ GIVEN the redesigned TUI code WHEN searched (`rg -i launch internal/tui/`) THEN no matches appear outside test fixtures or historical comments.
▸ GIVEN the redesigned TUI code WHEN inspected THEN the previous ContentTab enum and Launch state are removed; no "tab" concept remains in the shell.
▸ GIVEN the project keymap WHEN audited THEN `r`, `shift+r`, `p`, `:`, and the rail hotkeys do not collide with any existing global binding without the displaced binding being documented in the help screen.
▸ GIVEN the 99-test TUI suite WHEN triaged inside this task THEN every test is categorized as keep, rewrite, or delete, and new state-machine coverage for the rail router lands in the same commit as the shell rewrite.

### Task 4: Games and Defaults resource scaffold

**Depends on**: Task 2, Task 3
**Status**: □ pending
**Acceptance**:
▸ GIVEN the games resource is selected THEN a game list renders on the left of the resource pane and the selected game's detail renders on the right.
▸ GIVEN a game's detail view WHEN rendered THEN profile fields appear in a single column, grouped by subsystem (proton, dlss, gpu, cpu, overlay), with every group always visible (no collapse/expand toggle).
▸ GIVEN the defaults resource is selected THEN the same detail renderer displays the default profile as root, with no inherited or overridden markers and no reset/pin keybindings offered.
▸ GIVEN navigation WHEN j/k is pressed THEN focus moves field-by-field across the entire grouped list, including across group-header boundaries.

### Task 5: Games inheritance rendering, reset/pin bindings, duplicate-preset fix

**Depends on**: Task 4
**Status**: □ pending
**Acceptance**:
▸ GIVEN a game profile field WHEN it is inherited THEN it renders with the `fg-muted` token and no override marker.
▸ GIVEN a game profile field WHEN it is overridden THEN it renders with the `fg` token and a visible override marker using the `accent-override` token.
▸ GIVEN a focused field WHEN the user presses `r` THEN the field resets to inherited; WHEN `shift+r` is pressed THEN every field on the current game's profile resets to inherited; WHEN `p` is pressed on an inherited field THEN the currently-resolved value pins as an override.
▸ GIVEN the DLSS model selector WHEN opened in a game's profile THEN every preset appears at most once.
▸ GIVEN the help screen WHEN opened in the games resource THEN `r`, `shift+r`, and `p` are documented with their effects.

### Task 6: DLLs resource and Metrics resource relocation

**Depends on**: Task 3
**Status**: □ pending
**Acceptance**:
▸ GIVEN the dlls resource is selected THEN two sections are visible at once: a library section listing DLL types with latest and cached versions, and a deployment section showing a games × DLL types table with the installed version per cell.
▸ GIVEN a game has an installed DLL older than the latest cached version WHEN the deployment table renders THEN the cell visibly marks the stale state.
▸ GIVEN a DLL type has zero installed games WHEN the deployment table renders THEN its row is omitted.
▸ GIVEN the user triggers update-all from the dlls resource THEN every stale cell updates to the latest cached version in a single batched action, with per-cell success/failure reflected afterward.
▸ GIVEN the metrics resource is selected THEN the existing thermal and sparkline widgets render inside its pane with no regression in data or rendering (existing metrics state-machine tests pass unchanged).

### Task 7: Version bump per DOCS.md convention

**Depends on**: Task 1, Task 2, Task 3, Task 4, Task 5, Task 6
**Status**: □ pending
**Acceptance**:
▸ GIVEN all prior tasks are complete WHEN the release is cut THEN the version bumps to v0.5.0 per the git-cliff + magefile policy documented in DOCS.md.
▸ GIVEN CHANGELOG.md WHEN inspected post-bump THEN a v0.5.0 section summarizes the redesign under Added/Changed/Fixed, and the Unreleased section is empty.
▸ GIVEN the tag v0.5.0 WHEN fetched THEN it exists on origin per the project's explicit-push tag policy.

### Task 8: Plan-level freshness checkpoint

**Depends on**: Task 7
**Status**: □ pending
**Acceptance**:
▸ GIVEN CHANGELOG.md WHEN inspected THEN the v0.5.0 entry describes the redesign at plan level (spine, inheritance, theme, resources), not as a sequence of per-cycle diffs.
▸ GIVEN PROGRESS.md WHEN inspected THEN a "Plan Summary — TUI ground-up redesign" entry exists following the PROGRESS.md format.
▸ GIVEN TODO.md WHEN inspected THEN every item superseded or resolved by the redesign is moved to the Resolved section.
▸ GIVEN PLAN.md WHEN inspected THEN all tasks are marked complete and the plan is archived to `.agentera/archive/PLAN-2026-04-19-tui-redesign.md`.

## Overall Acceptance

▸ GIVEN a user opens the redesigned TUI THEN the shell is a left rail of four resources (games, dlls, defaults, metrics) with no tabs, no launch surface, and a single neon-accent dark theme.
▸ GIVEN a game profile WHEN viewed THEN inherited and overridden fields are visually distinct and the user can reset or pin any field with a single keystroke.
▸ GIVEN the default profile is edited WHEN a game has fields inheriting from it THEN those fields reflect the new default values at apply time without the user touching the game's profile.
▸ GIVEN any existing profile YAML file WHEN loaded THEN it continues to apply with the same practical behavior as before the redesign, with inheritance correctly reconstructed per the migration policy.
▸ GIVEN v0.5.0 is tagged WHEN CHANGELOG.md is read THEN the redesign is summarized at plan level under the version heading.

## Surprises

(empty; populated by realisera during execution when reality diverges from the plan)
