# TUI profile and settings UX review plan

Status: approved on 2026-10-14 and tracked as Agentera plan `mnfroqoiwu`. The review is
complete as of 2026-10-14 and awaits the user's choice among the alternatives. The report is
[tui-profile-settings-ux-review.md](tui-profile-settings-ux-review.md), with mock frames in
[tui-profile-settings-ux-review-mocks.md](tui-profile-settings-ux-review-mocks.md). Nothing has
been implemented.
Prepared against `e0d293d` (v0.6.2-4). The working tree has an uncommitted
dependency update. It changes no TUI source, but it does bump Bubble Tea,
Bubbles, Lipgloss, and Ultraviolet, so it can change what the TUI renders.

## Why another review

Two earlier rounds fixed the TUI's correctness. The Agentera plan `ajcnhmbmoh`
("TUI clarity, safety, and operability", 2026-09-02) and the
[basic-key plan](tui-basic-key-navigation-plan.md) (2026-09-13) made hints
truthful, made drafts safe, made every workflow reachable with basic keys, and
bounded the layout. Both passed acceptance.

Neither round measured task cost, comprehension, or information architecture.
Some of their fixes add friction by design: dirty-draft guards, reset and discard
only in Actions, a four-control field editor, and the removal of letter and
Escape accelerators. Profile and settings management still feels clunky.

**Objective:** find out why managing profiles and settings feels clunky,
measure it, and produce two or three simpler interaction models for the user to
choose from. This plan covers the review only. Implementation gets its own plan
after the user decides.

## Scope

In scope:

- Library profile editing for All games and for single games.
- How inheritance and effective values are shown.
- The Settings destination.
- The Actions overlay where it serves profiles or Settings.
- The save, discard, reset, and field-editor model.
- Field vocabulary across the TUI, GUI, CLI, YAML, and
  [profile-fields.md](../profile-fields.md).
- The GUI's profile and Settings flows, measured on the same tasks, because
  TUI and GUI parity is a priority.
- Both 120x40 and 80x24, with hints on and off.

Out of scope:

- DLL, Monitor, and Catalog workflows, except shell patterns they share with
  profiles.
- Code changes of any kind during the review, in the TUI or the GUI.

Constraints:

- Use isolated HOME/XDG state and fixture games.
- Never approve privileged actions or touch real game files.
- Follow [TUI testing](../tui-testing.md) for sessions, evidence, and cleanup.

## Recon findings (2026-10-14)

This was a short hands-on pass, not the review. Setup:

- A prebuilt binary, `v0.6.2-4-ge0d293d-dirty`, built on 2026-09-14 from a tree
  whose uncommitted state is unknown.
- Terminal Control 1.2.1.
- An isolated copy of the real configuration: 45 games plus the All games row,
  and 16 profiles.

### Contract defects

These are mismatches between documentation, code, and behavior. Fix them
whichever redesign the user picks. Agentera tracks them as TODOs: D1
`kjutocdjaz`, D2 `srryekxvmq`, D3 `hpuwnhwspr`, D4 `hxvtaqzzfg`, and D5
`bdnaynbvan`.

| ID | Defect | Evidence |
| --- | --- | --- |
| D1 | `profile-fields.md` omits `gpu.vrr` (KDE VRR). | `internal/profile/fields.go:66` has the descriptor; the doc table has no row. |
| D2 | The doc's "Shared label" column contradicts the code. Examples: SR mode vs Super resolution, SR preset vs Resolution preset, RR mode vs Ray reconstruction, FG enabled vs Frame generation, Threaded opt vs Threaded optimization. Commit `72f5b0d` simplified the code labels on purpose, so by default the doc gets updated. Confirm against the vocabulary matrix (Phase 2). | `profile-fields.md` table vs `fields.go:48-86`. |
| D3 | TUI Settings shows five groups for three options. DLL policy and Logging are empty and show "No setting selected". | The rendered Settings screens. `config.Sections(VisibilityTUI)` keeps empty sections. |
| D4 | Only 1 of 35 field descriptors has a description (KDE VRR). Numeric fields show no units or ranges: clock offset, memory offset, power limit, fan speed. The "Effect:" line is generic per impact class. | `fields.go`, and the Profile view. |
| D5 | Fan speed 0 has a hidden meaning. It saves as an override and shows "0 · Override for this game". But apply skips values of 0 or less, so 0 means "leave the fan alone" and also blocks any inherited fan speed. Nothing on screen says so. Clock offset, memory offset, and power limit were not checked; Phase 2 covers them. | `internal/profile/apply.go:107`, and the saved fixture YAML. |

### UX observations to confirm

These are hypotheses. The review must confirm, revise, or reject each one with
evidence.

| ID | Observation |
| --- | --- |
| O1 | Profile fields are three steps deep: select a game, press Enter (Overview), press Right (Profile). Moving to another game resets Detail to Overview, so editing several games repeats the steps. |
| O2 | With an unsaved field, a plain List cursor move opens a full-screen "Unsaved changes" dialog. It names the profile but not the changed fields. One edit blocks browsing. |
| O3 | The "Unsaved changes" label sits at the top of the scrolling field list (`detail.go:651`). When editing lower fields it scrolls out of view. The only visible cue is then a ◆ beside the field and a Ctrl+S hint. |
| O4 | The value column is ambiguous. Game rows show `(default)` plus `↳ Default: true`, or `↳ Inherited from defaults` with no value for numbers. All games rows repeat `○ Default for all games` on every row and never say what the system default is. The effective value is never the main column. Overridden rows shift the label two characters right. |
| O5 | Space can create an override that changes nothing. Multi-frame cycled from `(default)` to `0` while the inherited value was already 0. |
| O6 | All 35 fields appear in one flat list grouped by subsystem. Real profiles override about ten fields: HDR, Wayland, VKD3D heap, SR preset, SR override, SR indicator, power mode, shader cache, threaded optimization, and VRR. There is no way to show only overrides, jump to a section, or find a field. |
| O7 | Actions mixes profile commands with Rescan, three Layout rows, Help, and Quit. It fills the screen for about ten rows. Its first row is a disabled Save. "Reset field to system default" does not name the field. |
| O8 | The field editor replaces the field list. For an inherited field it shows "Value before edit: 0" rather than the effective value. Its instructions appear twice. Observed slip: three Tabs from Input reached Save, not Cancel. Enter then saved a system-state GPU field to disk with no confirmation. |
| O9 | Settings needs Left/Right per group, Enter, Space, then a save, for one or two options per group. Detail shows a single option as "Saved value" and "Draft value" blocks. |
| O10 | Dialogs and Actions hide the header and use the full screen for a few lines of content. |
| O11 | In real data, the DrainSim profile pins false or empty values against the defaults: Wayland, shader cache, threaded optimization, power mode, and SR indicator. Check whether the user set them or legacy migration created them. Migration compares absent leaves with the current defaults. Either way, "explicitly off" must look different from "inherits on". |

First baseline data point: change one field on one game, then move to the next
game. From List on 3DMark, the minimal path is 19 keys (12 of them Down presses),
three view changes, and one modal.

## Inherited decisions and user direction

Earlier plans approved the decisions below. The review measures what each one
costs. On 2026-10-14 the user gave direction on three points:

1. **Basic keys only stays the rule.** Commands keep using only arrows, Tab,
   Enter, Space, and numbers. The review may argue for optional accelerators,
   but only with strong measured evidence. The bar: accelerators would cut keys
   by at least 30 percent on T1 or T7, or remove a slip seen in two or more
   walkthroughs, while the basic-key path stays complete.
2. **Autosave is acceptable** for profile fields. Alternatives may drop
   explicit saving and the draft guard if reverting a field stays easy.
3. **TUI and GUI parity is a priority.** Alternatives must define one profile
   vocabulary and one inheritance model for both surfaces.

The other decisions remain open for the user at the gate.

| ID | Decision | Origin | Costs to measure |
| --- | --- | --- | --- |
| G1 | Commands use only basic keys. Letter, Escape, and punctuation accelerators were removed, not made optional. Kept by user direction 1. | Basic-key plan | Keys per task. Actions round-trips. No fast cancel or search. |
| G2 | Saving is always explicit, and the draft guard fires on any List cursor move. Autosave is allowed by user direction 2. | Basic-key plan, F3 | Modals per task. Browsing blocked by drafts. |
| G3 | Reset and discard exist only in Actions. | Basic-key plan | Keys to revert one field. |
| G4 | The editor has four controls: Input, Apply, Cancel, Save. | Basic-key plan, F4 | Slips. Apply vs Save confusion. |
| G5 | Profile is the second view tab inside game Detail; Overview opens first. | Shell contract | Steps to reach fields. Lost view on game change. |
| G6 | Settings keeps five fixed groups. Inert options are hidden. | Plan `ajcnhmbmoh` | Empty groups. Keys per option. |
| G7 | Fields are ordered by subsystem, and all of them are always shown. | Field descriptor order | Scrolling. Time to find the fields people actually use. |

## Method

### Phase 0: fixtures and baseline

1. Build outside the working tree, in disposable worktrees, because `mage build`
   runs code generation and a frontend install. Build the committed tree, and a
   second tree with the dependency update if it is still uncommitted. Compare
   one journey on both, record each build's version and commit, and name the
   build every stream uses. `git status` in the main checkout must not change.
2. Create a deterministic fixture with isolated HOME/XDG and temporary game
   paths, reusing `tests/e2e` helpers where possible. Use at least 40 games, so
   list scrolling matches the real library, and record each target game's list
   position. It must cover:
   - All games with concrete values, including a nonzero clock offset.
   - A game with ordinary overrides.
   - A game with explicit false or empty pins that oppose the defaults (the
     DrainSim pattern).
   - A game that inherits a nonzero number.
   - An empty game.
   - Settings with a non-default Steam path.
3. Create scratch GUI fixture data, outside the repository, with the same
   inheritance cases. Open the GUI frontend at a stated browser viewport and
   confirm the profile and Settings screens render.
4. Keep all evidence under `~/.local/state/spela-ux-review/2026-10-14/`. It is
   outside the repository and outside any cache, so it survives cleanup. Give
   each stream its own subdirectory, fixture copy, and termctrl session names.

### Phase 1: task walkthroughs and metrics

Run each task at 120x40 and at 80x24, using only displayed controls. Base the
tasks on real usage:

| Task | Goal |
| --- | --- |
| T1 | Turn HDR on for one game. |
| T2 | Set SR preset to K with SR override on for one game. |
| T3 | Turn VKD3D heap off for all games, then find which games will not follow because they override it. |
| T4 | Say what one game will actually get at launch, and which values are its own. |
| T5 | Return one field to inheritance, then reset a whole game. |
| T6 | Set a clock offset and power limit for one game, and understand units and launch behavior. |
| T7 | Apply the same change to three games in a row. |
| T8 | Abandon a half-made change. |
| T9 | In Settings, turn hints off, change the Steam path, and enable rescan on startup. |
| T10 | Notice and undo a wrong saved value. |

For each task, record:

- Keys by type.
- Pane, view, and mode changes.
- Modals.
- Slips and errors.
- Whether each frame answers four questions: where am I, what is unsaved, is
  this value inherited or overridden and from where, and what will launch
  apply.
- For each step, the cognitive-walkthrough questions: does the user know what to
  do, can they see the control, does it match their goal, and can they see
  progress?

Store the `termctrl show` frames and a `commands.jsonl` log as evidence.

Repeat the profile and Settings tasks in the GUI. Run its frontend dev server
in a browser with the scratch fixture data from Phase 0, and record clicks,
screens, and the same four questions. This shows the GUI's model and wording,
not its real backend. Record that limit.

### Phase 2: heuristic, vocabulary, and metadata audit

- Rate each issue on Nielsen's heuristics, adapted for a terminal, with severity
  from 0 to 4.
- Build a vocabulary matrix of every field label and state term ("default",
  "inherited", "override", "pin", "unset", "system default", "root profile",
  "All games") across the TUI, GUI, CLI `profile show`, YAML, and
  `profile-fields.md`.
- Audit each field for description, unit, range, meaning of zero or empty,
  impact, and restore behavior. Check the zero and negative semantics of clock
  offset, memory offset, and power limit against `apply.go`. Check that each
  overlay field has a runtime consumer.
- Measure how much screen space goes to content, chrome, and empty dialog area.
- Settle O11 from migration code and tests.

### Phase 3: root causes

Map each finding to the code and contract that owns it, mainly `detail.go`,
`editor_host.go`, `options_modal.go`, `draft_navigation.go`, `actions_menu.go`,
`keymap.go`, `content_views.go`, `internal/profile/fields.go`, and
`internal/config/options.go`. Mark each fix as either presentation-only or a
model change. List the unit and E2E tests that encode current behavior and
would need to change.

### Phase 4: comparison

- GUI: `GameProfilePane.svelte` and `SettingsPane.svelte` cover the same domain
  with a different model, including Pin, Reset, and Unset actions and a paired
  frame-generation action. Each inconsistency between surfaces is a finding.
  List the Settings options each surface exposes, with a reason for every
  difference.
- CLI: `spela profile show` and `set` for vocabulary and effective-value output.
- Reference TUIs: take only patterns that apply. Look at inline editing and
  autosave with undo, a show-only-overrides filter, an effective-value column,
  single-page settings, and contextual key bars. Candidates include lazygit,
  k9s, btop options, htop setup, nmtui, and charmbracelet/huh forms.

### Phase 5: synthesis and alternatives

- Rank findings by severity and frequency, each with evidence.
- Build a baseline metrics table for T1 to T10 at both sizes.
- Write two or three alternatives, each with mock frames at 120x40 and 80x24,
  estimated task metrics, changed contracts, test churn, and risks. Every
  alternative may use autosave and defines one shared profile model for the TUI
  and GUI, with the GUI changes it implies. At least one alternative uses basic
  keys only. Optional accelerators appear only if Phase 3 met the bar under user
  direction 1. The
  sketches below are only starting points. Revise or replace them based on the
  findings.
  - **A, incremental:** keep the structure. Add an effective-value column, a
    pinned unsaved banner, field help with units, and one Settings list. Make
    the draft guard non-modal or save on leave.
  - **B, profile-first:** game Detail opens on Profile. The rows that matter
    (overrides) come first and sections can collapse. Edit inline, with
    autosave and per-field revert.
  - **C, one form model:** profiles and Settings share one scrolling form
    component with sections, a field filter, inline editors, and one save
    model.
- If frames do not settle the choice, build a throwaway prototype of the
  leading option.

### Phase 6: decision gate

Present the findings, metrics, the G1 to G7 costs, and the alternatives. The
user chooses. Then write a separate Agentera implementation plan whose
acceptance targets come from the measured baseline, for example fewer keys for
T1 and T7 and no modal for plain browsing.

## Deliverables

- A review report at `docs/design/tui-profile-settings-ux-review.md`.
- Rendered evidence, command logs, and final fixture state under
  `~/.local/state/spela-ux-review/2026-10-14/`. The report quotes the key
  frames, so it stands on its own.
- D1 to D5 as Agentera TODOs, updated with review findings, plus TODOs for any
  new defects the review confirms.

## Execution

The Agentera plan "Plan: TUI profile and settings UX review" splits this method
into nine tasks. Phase 0 is task 1. After it, five streams are independent:

- Task 2: single-game tasks T1, T2, T4, T6, T7, T8, and T10.
- Task 3: All games and inheritance tasks T3 and T5.
- Task 4: Settings task T9.
- Task 5: GUI walkthroughs and Settings parity.
- Task 6: the vocabulary, field metadata, and defect audit.

Run them in parallel with separate fixtures and session names. Task 7 covers
root causes and decision costs. Task 8 writes the report and metrics. Task 9
writes the alternatives and prepares the decision. Tasks 7 to 9 run in order.

## Completion criteria

- T1 to T10 are measured at both sizes, with frame evidence.
- Each of D1 to D5 and O1 to O11 is confirmed, revised, or rejected with
  evidence.
- Each of G1 to G7 has a measured cost.
- Each alternative has frames, estimated metrics, and named contract changes.
- Evidence is archived, then all named sessions are stopped and live fixtures
  removed. `termctrl list` shows no review sessions.
