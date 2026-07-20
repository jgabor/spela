# TUI Redesign Delivery Plan

Deliver a predictable terminal interface built around non-focusable destination navigation and a
two-pane **List / Detail** workspace. Follow Document → Test → Code within each phase and leave the
TUI usable at every phase boundary.

## 1. Lock down broken behavior

Define the replacement contract before changing the shell. Protect intended outcomes, not current
broken bindings or three-zone mechanics.

- [ ] Document navigation state as destination, selection, visible focus, input mode, and optional
  overlay; shell widgets are not domain state.
- [ ] Define input precedence: **Search**, **Edit**, and **Overlay** consume applicable input before
  global shortcuts. Printable destination shortcuts are active only in **Browse** mode.
- [ ] Route keys only to the visibly focused pane, except documented global actions.
- [ ] Define one keymap model for behavior and rendered help, including mode, scope, action, key
  labels, and availability.
- [ ] Define layout fixtures: 120x40 is normal and 80x24 is the supported minimum. At or above
  80x24 output is bounded and operative with status/help reachable; below it, render a bounded
  resize prompt and never panic.
- [ ] Add state-machine tests for mode precedence, focus visibility, destinations, selection,
  detail actions, overlays, and resize. Explicitly regress reserved printable characters in Search,
  Settings save visibility/live mutation, and hidden components handling keys.
- [ ] Add regressions proving game selection lands on Overview, search updates displayed scope, and
  profile navigation cannot steal list movement.
- [ ] Add layout regressions at 120x40, 80x24, and below-minimum dimensions for those contracts.
- [ ] Confirm new regressions fail on current bugs, then introduce the canonical keymap/router and
  compatibility wiring in the legacy shell until they pass; do not require the Phase 2 shell.
- [ ] Make unsupported keys explicit no-ops with no hidden focus or scope changes.

**Acceptance criteria**

- [ ] Behavior and help use one keymap source.
- [ ] Tests encode the new routing and layout contract without rail focus, Context subsystem
  cycling, or three-zone transitions.
- [ ] Known failures are fixed and the full relevant suite is green at the phase boundary; tests
  assert user-visible behavior rather than legacy shell mechanics.

## 2. Replace the shell

Replace primary/context/content focus with destination navigation above a two-pane workspace.
Preserve domain operations while deleting obsolete shell state and routing.

- [ ] Document the destination set and default entry state for **Library**, **DLL Catalog**,
  **Monitor**, and **Settings**.
- [ ] Specify List / Detail ownership for each destination, including empty, loading, error, and
  unavailable states; preserve Library sorting, DLL/profile filters, multi-select, and batch actions.
- [ ] Add tests proving the top bar responds to global shortcuts but never enters the focus cycle.
- [ ] Add tests for List ↔ Detail focus, selection-driven details, mode-specific Escape behavior,
  and destination changes from every pane.
- [ ] Implement the non-focusable top destination bar with a clear selected state.
- [ ] Keep one visible focus target across List / Detail; Tab and reverse-Tab move only between
  panes that can accept input.
- [ ] Make each list own its cursor, filtering, scrolling, and selection; make each detail pane own
  actions for that selection.
- [ ] Open a selected game on Overview rather than inheriting a prior profile subsection.
- [ ] Update search results and displayed scope together; Enter may accept a result but is not
  required to repair stale scope text.
- [ ] Centralize terminal measurement, enforce the 80x24 contract, and clamp every pane, overlay,
  table, and help view to the available viewport.
- [ ] Render mode- and focus-aware status hints from the canonical keymap.
- [ ] Remove three-zone state, rail/context focus handlers, and adapters after active routes migrate.

**Acceptance criteria**

- [ ] Every destination is reachable and usable without focusing the destination bar.
- [ ] Visible focus, input mode, help, and actual key routing agree.
- [ ] Supported resizes preserve bounded output and a recoverable navigation state.

## 3. Consolidate editing

Profile uses one typed profile field catalog; Settings may retain its domain settings catalog.
Both feed a shared typed editor host/value-kind contract and use the same transactional pattern:
entering edit creates a draft, save persists it, and cancel discards it.

- [ ] Document the field catalog contract: stable key, subsystem, type, label, description,
  inherited/effective value, constraints, and editor. A concrete game value creates an override;
  reset returns it to **Inherited**. Root/default fields use **System default** instead.
- [ ] Define profile and Settings draft behavior for modify, save, cancel, validation failure, and
  persistence failure; Settings drafts cannot mutate live config, successful saves remain visible,
  and failed saves retain a recoverable draft.
- [ ] Add catalog coverage for every profile field and editor-contract tests across Profile and
  Settings for supported value kinds, invalid input, and cancellation.
- [ ] Add persistence tests for exact changed keys and values, Inherited/System default semantics,
  Settings isolation, post-save visibility, and draft retention after failed saves.
- [ ] Feed the typed profile catalog and domain settings catalog into one editor host/value-kind
  contract; share controls and transactions without merging unlike domain metadata.
- [ ] Keep profile changes in memory until save, show dirty state, and restore persisted values on
  cancel; concrete game values create overrides, while reset returns to Inherited or System default.
- [ ] After concrete-value override editing is available, remove the TUI pin action, binding, help
  entry, and pin-specific shell tests while preserving stored profile override semantics.
- [ ] Apply the draft boundary to Settings without premature live mutation or replacing visible
  saved values; show failures in place while preserving focus and the draft.
- [ ] Remove `ProfileWidget`, duplicate subsystem editors, and obsolete preset-modal routing after
  every profile field uses the catalog.

**Acceptance criteria**

- [ ] Every profile field uses the typed profile catalog; Profile and Settings both use the shared
  editor host/value-kind contract while retaining their appropriate domain catalogs.
- [ ] Profile and Settings saves survive reload and remain visible; cancel writes nothing;
  validation and I/O failures retain the draft without changing live state.
- [ ] No production route references `ProfileWidget` or Context subsystem navigation, and no TUI
  pin action, binding, help entry, or pin-specific shell test remains.

## 4. Enforce user journeys

Test the redesign as users experience it in a real terminal. Prefer isolated journeys over
duplicated key-by-key shell checks.

- [ ] Document each journey's fixture, size, actions, persisted outcome, and final view; use fresh
  deterministic state and bounded waits rather than Escape cleanup from a previous scenario.
- [ ] Cover Library search → game → Overview → profile edit → save → reload, plus cancel and invalid
  edit correction; assert the exact override, no cancel write, and draft retention after failure.
- [ ] Cover Settings edit → save → reload, plus cancel and save-failure outcomes; assert no live
  mutation before save, saved values remain visible, and failed drafts stay recoverable.
- [ ] Cover Library sorting, DLL/profile filters, multi-select, and batch actions in replacement
  journeys before deleting any legacy fixtures that protect those outcomes.
- [ ] Cover Library, DLL Catalog, Monitor, and Settings switching; assert destination selection
  never becomes pane focus.
- [ ] Cover traversal, input precedence, help, and resize behavior at 120x40, 80x24, and below the
  supported minimum.
- [ ] Cover refresh and available DLL actions with deterministic service or HTTP fixtures; absent
  actions must be hidden or explain why they cannot run.
- [ ] Assert help against the canonical keymap in each mode instead of duplicating binding lists.
- [ ] Run focused TUI tests, terminal journeys, then the repository test target; fix flakes before
  declaring the redesign complete.
- [ ] Delete superseded three-zone and rail-focus fixtures after replacement journeys cover their
  user-visible outcomes.

**Acceptance criteria**

- [ ] Terminal coverage proves navigation, search, editing, persistence, cancel, error recovery,
  help accuracy, destination access, and bounded resize.
- [ ] Journeys are isolated and validate outcomes rather than ANSI details or obsolete focus states.
- [ ] The full repository test target passes without legacy shell fixtures.

**Non-goals and removed contracts**

- The three-focus-zone shell, focusable destination rail, rail cursor, and rail-focus tests are not
  preserved.
- `q` is not back. Escape owns mode-specific back/cancel behavior; quit is a distinct keymap action.
- Context does not navigate profile subsystems; fields use List / Detail and the typed catalog.
- A command palette is out of scope and must not remain as a deferred binding or help entry.
- Broken bindings, hidden routing, rmux mechanics, ANSI snapshots, GUI parity, and unrelated
  maintenance are not permanent TUI contracts.
- Launch redesign, new profile fields, and new DLL capabilities are out of scope unless needed to
  preserve an existing operation during migration.
