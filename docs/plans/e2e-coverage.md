# Plan: e2e-rmux.sh interaction coverage

## What

Close the six largest coverage gaps in `scripts/e2e-rmux.sh`: value mutation
(`h`/`l`), pin (`p`), root defaults editing, sidebar filters/sort, multi-select
batch actions, and options modal save. Every core user interaction that
mutates state should have at least one e2e assertion verifying the TUI
rendered the change and the YAML persisted.

## Why

The current suite tests rendering (smoke, search, empty state, help, nav)
and two reset bindings (`r`, `R`). It never tests *changing a value* — the
primary reason a user opens the TUI. The root defaults editor (just refactored
to reflection-based bool helpers in 71f665f) has zero e2e coverage. Sidebar
filters, multi-select, and options save are all wired but unexercised.

## Scope

### Included

- `h`/`l` cycling on game profile bool fields
- `h`/`l` cycling on root defaults bool fields (three-state: default→true→false)
- `p` pin binding (inherited → override)
- Root defaults `r` (reset focused) and `R` (reset all)
- Sidebar filters: `d` (hasDLLs), `P` (hasProfile), `C` (clear)
- Sidebar sort: `s` (cycle 4 modes)
- Multi-select: `space` (toggle), `a` (select all), `enter` (batch menu)
- Options modal: `j`/`k` navigate, `h`/`l` change, `s` save, verify config.yaml

### Excluded

- DLL install (`i`) / update (`u`) — requires mock HTTP server, deferred
- DLSS preset modal — requires navigating to `sr_preset` field and interacting
  with a modal list picker; separate effort
- Rail destinations 2 (DLL Catalog) and 3 (Monitor) smoke tests — filed in TODO.md
- Rail `j`/`k` + `Enter` navigation — filed in TODO.md
- Context nav `j`/`k` subsystem cycling — filed in TODO.md
- `q` back navigation — filed in TODO.md
- F5/F11 density toggle — filed in TODO.md
- Ctrl+R rescan — filed in TODO.md
- Search edge cases — filed in TODO.md
- Structural refactors (send_and_capture helper, session isolation) — filed in TODO.md

## Design

Each new test follows the existing `qa_mutation` / `qa_reset_all` pattern:
fresh rmux session, navigate to the target scope, send keys, poll the YAML
file for the expected change, assert on the captured screen. Reuse the
existing `run_tui`, `capture`, `count_overrides` helpers. Add a
`count_overrides_for` helper that counts entries matching a specific field
key, so assertions can verify *which* field changed, not just that the count
shifted.

For root defaults tests, navigate to All games → Profile aspect (the path
exercised in UX-7 but never mutated). The `h`/`l` bindings there route through
`resource_view.go:225-236` (the code shipped in 71f665f).

For options modal tests, navigate to Settings (`4`), enter the modal, change
`check_updates` from `false` to `true`, save with `s`, and grep
`config.yaml` for the updated value.

## Tasks

### Task 1: Game profile value mutation (h/l)

**Acceptance:**

- GIVEN a game profile with `dlss.fg_override: false` WHEN user opens the
  profile aspect, focuses `fg_override`, and presses `l` THEN the TUI shows
  `true` and `1091500.yaml` has `fg_override: true` under `overrides:`
- GIVEN the same field WHEN user presses `h` THEN the value cycles back to
  `false` and the YAML reflects the change
- GIVEN a non-bool field (e.g. `sr_mode`) WHEN user presses `h`/`l` THEN
  the value cycles through its enum options and persists

### Task 2: Root defaults value mutation (h/l)

**Acceptance:**

- GIVEN the root defaults profile (All games → Profile) WHEN user focuses
  `proton.vkd3d_heap` and presses `l` THEN the value cycles from
  `(default)` to `true` and `default.yaml` has `vkd3d_heap: true`
- GIVEN the same field at `true` WHEN user presses `l` THEN it cycles to
  `false` and the YAML reflects `vkd3d_heap: false`
- GIVEN the same field at `false` WHEN user presses `l` THEN it cycles back
  to `(default)` and the `vkd3d_heap` key is removed from `default.yaml`

### Task 3: Pin binding (p)

**Acceptance:**

- GIVEN a game profile with an inherited field (not in `overrides:`) WHEN
  user focuses it and presses `p` THEN a new entry appears in `overrides:`
  and the TUI shows the override marker
- GIVEN an already-overridden field WHEN user presses `p` THEN no duplicate
  entry is created (idempotent)

### Task 4: Root defaults reset (r, R)

**Acceptance:**

- GIVEN root defaults with `proton.enable_hdr: true` WHEN user focuses that
  field and presses `r` THEN the value returns to `(default)` and the key
  is removed from `default.yaml`
- GIVEN root defaults with multiple non-default fields WHEN user presses `R`
  THEN all fields return to `(default)` and `default.yaml` contains no
  explicit values

### Task 5: Sidebar filters and sort

**Acceptance:**

- GIVEN the game list with 3 games (2 with DLLs, 1 without) WHEN user
  presses `d` THEN only games with DLLs are visible (Elden Ring hidden)
- GIVEN filtered list WHEN user presses `C` THEN all 3 games reappear
- GIVEN the game list WHEN user presses `s` THEN the sort order changes
  (verify by capturing before/after and asserting the first line differs)

### Task 6: Multi-select and batch actions

**Acceptance:**

- GIVEN the game list WHEN user presses `space` on a game THEN the select
  indicator appears and select mode activates
- GIVEN select mode with 2 games selected WHEN user presses `a` THEN all
  filterable games are selected
- GIVEN select mode active WHEN user presses `enter` THEN the batch action
  menu appears (verify "Update all DLLs" text)
- GIVEN batch menu visible WHEN user presses `esc` THEN the menu closes

### Task 7: Options modal save

**Acceptance:**

- GIVEN Settings destination open WHEN user enters the options modal,
  navigates to `check_updates`, presses `l` to toggle to `true`, and
  presses `s` THEN `config.yaml` contains `check_updates: true`
- GIVEN the saved config WHEN the TUI is restarted THEN the options modal
  shows `check_updates` as `true`

## Overall acceptance

- GIVEN `scripts/e2e-rmux.sh` WHEN executed THEN all existing and new checks
  pass without manual intervention
- GIVEN any core mutation binding (`h`, `l`, `r`, `R`, `p`) WHEN exercised in
  e2e THEN the persisted YAML reflects the expected state
- GIVEN the root defaults editor (shipped in 71f665f) WHEN exercised in e2e
  THEN three-state bool cycling, reset, and reset-all all verify correctly
