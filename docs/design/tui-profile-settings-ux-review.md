# TUI profile and settings UX review

Status: review complete, awaiting your choice. Part 1 (sections 1 to 8) holds the findings and
baseline metrics. Part 2 (sections 9 to 14) holds the alternatives and the decision package.
Nothing has been implemented.

- Date: 2026-10-14. Prepared against commit `e0d293d` (v0.6.2-4). Every stream measured
  build (a), built from that commit in a separate worktree. The working tree's uncommitted
  dependency update was not measured; it also breaks `mage build` (see the last section).
- Plan: [tui-profile-settings-ux-review-plan.md](tui-profile-settings-ux-review-plan.md),
  tracked as Agentera plan `mnfroqoiwu`. IDs used here: defects D1 to D5, observations O1
  to O11, inherited decisions G1 to G7, tasks T1 to T10.
- Evidence: `~/.local/state/spela-ux-review/2026-10-14/` (outside the repository). This
  report quotes the key frames, so it stands on its own. Paths below that do not start with
  `internal/`, `cmd/`, `docs/`, or `tests/` are relative to that folder.
- Source paths are repo-relative at `e0d293d`.

## 1. Summary

1. **A correctness bug sits under the clunkiness (F1, critical).** If a game profile has no
   overrides left, the save writes the file without an `overrides` map. The next load
   treats it as a legacy file and pins every field that differs from a non-zero default.
   The screen says "Profile saved!" and shows the fields as inherited. Launch then follows
   the pins, not the defaults: a dry run for the legacy `{}` game sets
   `__GL_THREADED_OPTIMIZATION=0` and plans no hardware change, although the fixture defaults
   say threaded optimization on, clock offset 150, and fan speed 60
   (`audit/cli/launch-dry-run-1601580.txt`). It happens through the TUI (reset the last override, or
   Reset all), the GUI (nine Resets), the CLI (`proton reset` on the last override), and
   `profile create` plus a first edit. Four of the five measuring streams reproduced it.
   By a read-only simulation against the code, your real `1091500.yaml` (Cyberpunk 2077)
   and `1601580.yaml` are `{}` and load with eight phantom pins each. DrainSim's five
   zero-valued pins are exactly the set that migration creates. Nothing in the redesign
   options should ship before this is fixed. Autosave in particular makes it worse.
2. **The friction is structural, and I measured it.**
   - HDR on for one game: 4 keys when the game is under the cursor, 40 keys when it is far
     down the list (37 of them arrows). The same far game takes 9 keys through Search, a
     route that exists in the Actions menu and that nothing on screen points to.
   - HDR on for three games in a row: 16 keys. Ten are navigation and six are Enter plus
     Right, because every game change returns to Overview and puts the field cursor back on
     HDR. The recon path takes 25 keys against the recon's 19.
   - Reset and discard live only in Actions: 6 to 8 keys and 2 to 3 modals each time.
   - Settings: 13 keys for three changes, 7 of which only move focus or switch group. Two of
     the five groups are empty.
3. **Screens leave out what a user needs to decide.** A pinned row never prints the default
   it replaced. The source is never "All games". The "Unsaved changes" label scrolls out of
   view at field 15 (120x40) and field 5 (80x24). The Effect line is cut before the restore
   sentence. Numeric hardware fields show no unit or range, and 99999 W was saved.
4. **The terminal and graphical interfaces use different edit models.** The TUI blocks
   browsing with a draft dialog that names the profile but not the field. The GUI has no
   draft marker and no guard, and seven of ten ways of leaving a screen drop a draft
   silently. Neither is right, and they need one model.
5. **Optional accelerators are not justified.** Each large cut and each recurring slip can
   be handled with displayed basic keys. Your bar (at least 30 percent fewer keys on T1 or
   T7, or a slip seen in two walkthroughs, with the basic-key path complete) is not met.
6. **Show hints does nothing visible.** Twenty frame pairs with hints on and off are
   identical except for the setting's own value lines. Where a task was run both ways, the key
   counts, changes, and modals are the same (T9 differs only because the task itself turns
   hints off).

7. **What I recommend (section 12).** Fix F1 first, on its own. Then take alternative B
   (profile-first, autosave, one shared vocabulary), delivered in two steps: the display and
   vocabulary fixes first, the navigation and save changes second. Estimated cost: T1 near
   4 to 2 keys, T7 16 to 10, T5 47 to 21, T10 44 to 19. If you decline autosave, alternative A
   is the safe choice.

## 2. Method and limits

- **Builds and fixtures.** Build (a), the committed tree. Isolated HOME and XDG roots made by
  `baseline/make-fixture.sh`: 46 games plus All games (47 rows), a defaults profile with
  concrete values, an ordinary override game, a game with explicit false and empty pins
  that oppose the defaults (the DrainSim pattern), a legacy empty `{}` game, an untouched
  game, a game that inherits non-zero numbers, and a non-default Steam path.
- **Isolation.** Every Spela process (TUI, CLI, GUI bridge) ran in a bubblewrap jail with
  `/` read-only and only the fixture root writable (`baseline/isolation/jail-proof.txt`).
  Your real Spela directories hashed the same before and after the whole run. No privileged
  action was approved and no game was launched.
- **Terminal runs.** Terminal Control 1.2.1 at 120x40 and 80x24, one key at a time, only keys
  the current frame displayed (exceptions are logged as exploratory with a slip). Each key
  was recorded with its frame, row tally, view and mode changes, modals, and the four
  orientation and four walkthrough answers.
- **GUI runs.** The shipped frontend over the real Go backend through a review bridge, in
  headless Chromium at 1280x800, one action at a time with a screenshot and a snapshot.
- **Counts.** Minimal-path keys exclude exploratory and failed rows. Arrows count one by
  one. A typed value is one text row. Tables below are generated from `count-log.py` output
  over the four logs (`synthesis/build-report.py`).
- **Raters.** One model rater per stream, working from shared anchors
  (`tools/judging-anchors.md`). Severity ratings in section 3 are mine, from a single
  evaluator; Nielsen suggests several.
- **Limits.**
  - Headless Chromium is not the WebKitGTK window the app opens (1024x768).
  - Hardware and launch behavior were read from code, not run.
  - The defaults stream copied its ratings for hints-off, replay, and 80x24 T5 cells from the
    120x40 judgments by script. The 120x40 hints-on ratings are the measured ones.
  - Several stream agents could not save their own report files. The parent re-validated
    every log, frame set, and cleanup, and spot-checked the key claims
    (`synthesis/stream-verification.md`). Claims not re-checked are marked there.
  - Estimates for options that do not exist yet are labelled as estimates.

## 3. Findings, ranked

Severity is Nielsen's 0 to 4 scale (0 not a problem, 1 cosmetic, 2 minor, 3 major, 4
catastrophe). Heuristics: H1 visibility of system status, H2 match with the real world, H3
user control and freedom, H4 consistency and standards, H5 error prevention, H6 recognition
over recall, H7 efficiency of use, H8 minimalist design, H9 error recovery, H10 help.
Frequency is the number of the five measuring streams (single-game, defaults, settings,
GUI, audit) that saw it independently, then the tasks. Class: P = presentation only, M = a
model change (storage, navigation, state, or field meaning). Order: severity, then
frequency. The full owner-and-contract map is in `synthesis/root-causes.md`.

| # | ID | Finding | Heuristic | Sev | Frequency | Class | Evidence |
|---:|---|---|---|---:|---|---|---|
| 1 | F1 | A profile returned to all-inherited is saved without an `overrides` map and re-pins fields on reload. The screen says saved and inherited. | H1, H5 | 4 | 4 streams: T5, T10, T3 | M | `single-game/frames/T10-120x40-on-r1-044/055/075.txt`; `defaults/state/T5-120-on-r2-after-part2/`; `gui/t5/`; `audit/cli/experiment-reset-last-override-no-mans-sky.txt` |
| 2 | F10 | Profile is the second view. Every game change resets the view to Overview and the field cursor to HDR. | H7 | 3 | 4 streams: every game task | M | `single-game/frames/RECON-120x40-r1-020/025.txt`; T7 |
| 3 | F5 | The unsaved cue is a label at the top of a scrolling list. It scrolls out at field 15 (120x40) and field 5 (80x24). The List pane shows no cue. | H1 | 3 | 4 streams: T1, T2, T5, T6, T8, T9 | P | `single-game/frames/T6-120x40-on-r3-068/069.txt`; `T1-80x24-on-r3-007/008.txt` |
| 4 | F3 | The value column is inconsistent. A pinned row never prints the default it replaced. The source is "Default", never "All games". | H2, H4 | 3 | 4 streams: T1, T3, T4, T5, T6 | P | `single-game/frames/T1-120x40-on-r1-003.txt`; `defaults/frames/T3-120x40-on-r1-018.txt` |
| 5 | F18 | Four label sets and different state words per surface. `profile show` prints raw values. | H4 | 3 | 4 streams | P | `audit/vocabulary-matrix.md` |
| 6 | F8, F20 | Numeric hardware fields show no unit or range, accept any integer, and a saved 0 silently blocks the inherited value. | H5, H10 | 3 | 4 streams: T6, T10 | M, P | `single-game/frames/T6-120x40-on-r5-025.txt`; `audit/field-audit.md` |
| 7 | F9 | Reset and discard live only in Actions. The first row and the reset row move with the field type. Wording differs by scope. | H6, H4 | 3 | 3 streams: T3, T5, T8, T9, T10 | M | `defaults/frames/T5-120x40-on-r1-042.txt`; `T4-120x40-on-r1-009.txt` |
| 8 | F7 | The field editor appends to a prefilled value, shows no Backspace, no unit or range, and its Cancel keeps the surrounding draft. | H5, H6 | 3 | 3 streams: T6, T8, T9 | M, P | `single-game/frames/T6-120x40-on-r1-020.txt`; `T8-120x40-r3-023.txt` |
| 9 | F11 | No jump, find, or wrap in the List. Search exists but is not signposted. | H6, H7 | 3 | 3 streams: T1, T3, T4, T10 | P | `single-game/frames/T1-120x40-on-r2-036.txt`; `audit/commands-parent.jsonl` |
| 10 | F2 | Migration pins look like the user's own choices, and Overview counts them. | H1, H2 | 3 | 3 streams: T3, T5, T10 | M, P | `audit/migration-origin.md`; `defaults/frames/T5-120x40-on-r3-025/026.txt` |
| 11 | F13 | The Effect line is cut at both sizes, dropping the restore sentence. | H1, H8 | 3 | 2 streams: every field row | P | `single-game/frames/T6-120x40-on-r2-024.txt` |
| 12 | F6 | The draft guard fires on any List move, names the profile but not the field, offers no keep-and-move, and misses digit keys. | H3, H5 | 3 | 2 streams: T8 | M | `single-game/frames/T8-120x40-on-r1-005.txt` |
| 13 | F17 | The GUI has no draft marker or guard, loses a draft on 7 of 10 leave routes, has no whole-game reset, and lacks fan speed and power limit controls. | H3, H1 | 3 | 1 stream: T5, T8 | M | `gui/notes.md` (T8 table); `gui/t5/` |
| 14 | F19 | Mode, preset, and frame-generation fields do nothing without their override field; 8 fields have no launch consumer. | H2, H5 | 3 | 1 stream (code inspection) | M | `audit/field-audit.md` |
| 15 | F16 | Show hints changes nothing visible. | H4 | 2 | 4 streams | P | `settings/frames/T9-120x40-on-r13-013.txt` and its pair |
| 16 | F14 | At 80x24 the Profile view and editor name no game, and Settings lists no values. | H1 | 2 | 4 streams | P | `single-game/frames/T1-80x24-on-r1-002.txt` |
| 17 | F12 | The 35 fields are one flat list. Clock offset is 15 Down from HDR. There is no section jump or overrides-only view. | H7, H8 | 2 | 3 streams: T4, T6, T10 | M | `single-game/frames/T6-120x40-on-r1-018.txt` |
| 18 | F15 | Settings has five fixed groups for three options, two empty. At 80x24 an empty group is a blank box. | H8 | 2 | 2 streams: T9 | M, P | `settings/frames/T9-120x40-on-r11-004.txt` |
| 19 | F4 | In All games scope every row reads "Default for all games", and `(default)` means unset there. | H2 | 2 | 2 streams: T3 | P | `defaults/frames/T4-120x40-on-r1-004.txt` |
| 20 | F21 | Out of scope: `spela overlay show` runs a PATH executable named `overlay`. | H5 | 2 | 1 stream | M | `audit/cli/overlay-show-hijacked-by-PATH-executable.txt` |

## 4. Baseline metrics

Terminal keys are minimal-path keys. "Not run" means that cell was not part of the plan for
that task. The same fixture and the same keys were used at both
sizes, so most counts match. A key count that looks equal at 120x40 and 80x24 does not mean
the screens are equal: see F14.

| Task or variant | 120x40 | 80x24 | 120x40, hints off | 80x24, hints off | Keys by type (120x40) | Changes (120x40) | Modals | Note |
|---|---:|---:|---:|---:|---|---|---|---|
| T1 near: HDR on for 3DMark (0 Down) | 4 | 4 | 4 | 4 | arrow 1, ctrl-s 1, enter 1, space 1 | pane 1, view 1 | none |  |
| T1 far: Satisfactory by scrolling (36 Down) | 40 | 40 | not run | not run | arrow 37, ctrl-s 1, enter 1, space 1 | pane 1, view 1 | none |  |
| T1 far: Satisfactory through Search (parent run) | 9 | not run | not run | not run | arrow 2, ctrl-s 1, digit 1, enter 3, space 1, text 1 | mode 2, pane 1, view 1 | actions 1 | measured, but the route is not signposted |
| T2: preset K and override on (Alchemy Factory) | 22 | 22 | not run | not run | arrow 9, ctrl-s 1, enter 1, space 11 | pane 1, view 1 | none | K costs 10 Space presses |
| T3 by scanning the games | 66 | 66 | 66 | 66 | arrow 54, ctrl-s 1, enter 4, space 1, tab 6 | pane 10, view 8 | none | 46 of 66 keys are Down |
| T3 through the profile filter | 26 | 26 | 26 | 26 | arrow 13, ctrl-s 1, digit 1, enter 5, space 1, tab 5 | pane 9, view 7 | actions 1 | through a 17-row Actions list |
| T4: Cyberpunk 2077, read 7 fields | 29 | 29 | not run | not run | arrow 28, enter 1 | pane 1, view 1 | none |  |
| T4: DrainSim, read 7 fields | 36 | 36 | not run | not run | arrow 35, enter 1 | pane 1, view 1 | none |  |
| T5: one field reset, then a whole game reset | 47 | 47 | not run | not run | arrow 37, ctrl-s 2, digit 2, enter 5, tab 1 | pane 3, view 3 | actions 2, confirm 1 | the result does not persist (F1) |
| T5: Reset all on the legacy Frostpunk 2 game | 24 | 24 | not run | not run | arrow 19, ctrl-s 1, digit 1, enter 3 | pane 1, view 1 | actions 1, confirm 1 |  |
| T6: clock offset 200 and power limit 250 | 28 | 28 | not run | not run | arrow 20, ctrl-s 1, enter 5, text 2 | mode 4, pane 1, view 1 | none | plus 3 undisplayed Backspaces per number field |
| T7: HDR on for 3 games in a row | 16 | 16 | 16 | 16 | arrow 5, ctrl-s 3, enter 3, space 3, tab 2 | pane 5, view 5 | none |  |
| T8 method 1: List move, Discard and continue | 5 | 5 | not run | not run | arrow 3, enter 1, tab 1 | pane 1, view 1 | unsaved-changes 1 | leaves the game |
| T8 method 2: Actions, Discard draft | 6 | 6 | not run | not run | arrow 3, digit 1, enter 2 | none | actions 1, confirm 1 | stays on the field |
| T8 method 3: editor Cancel | 3 | 3 | not run | not run | enter 1, tab 2 | mode 1 | none | does not discard the draft |
| T9: hints off, Steam path, rescan on startup | 13 | 13 | 10 | 10 | arrow 2, ctrl-s 1, digit 1, enter 4, space 2, tab 2, text 1 | destination 1, group 2, mode 1, pane 5 | none |  |
| T10: notice and undo a wrong saved value | 44 | 44 | not run | not run | arrow 38, ctrl-s 1, digit 2, enter 3 | pane 1, view 1 | actions 2 | the undo does not persist (F1) |
| Recon path: change a field, be ready on the next game | 25 | 25 | not run | not run | arrow 20, ctrl-s 1, enter 2, space 1, tab 1 | pane 3, view 3 | none | recon baseline was 19 |

Reading the table:

- T6 costs 28 displayed keys plus 3 undisplayed Backspaces for each number field edit.
- T8 methods 1 and 2 remove the draft. Method 3 (editor Cancel) does not, so its 3 keys
  are not a finished abandon.
- T9 with hints on costs 13 keys and with hints off 10, because the hints step costs 0 keys
  once the setting is already off.
- The hints-off columns differ from the hints-on columns only where the task changes the
  setting itself. Hints on and off made no other difference to any count (F16).
- T1 far through Search is a parent run on a fresh fixture (`audit/commands-parent.jsonl`,
  task T1 run 3). The route is measured and not discoverable: the List frame does not
  mention it, and the walkthrough answers are `partial`.

Graphical interface (1280x800, actions are clicks, scrolls, presses, and text):

| Task | Actions | By type | Changes | Modals | Note |
|---|---:|---|---|---|---|
| T1: HDR on for 3DMark | 6 | click 4, other 2 | pane 1, view 2 | none |  |
| T1: HDR on for Satisfactory | 7 | click 4, other 1, tab 1, text 1 | pane 1, view 2 | none |  |
| T2: preset K and override on | 12 | click 7, other 3, tab 1, text 1 | group 1, pane 1, view 2 | chooser 1 | the dropdown shows 7 of 13 options and needs a wheel |
| T3: VKD3D heap off for all games, find the games that keep it | 12 | click 11, other 1 | pane 4, view 8 | none | found through the Profile chip and 4 Profile clicks |
| T4: read Cyberpunk 2077 and DrainSim | 21 | click 13, other 5, tab 1, text 2 | group 9, pane 2, view 4 | none | Frame generation reads wrong for Cyberpunk 2077 |
| T5: one field reset, then nine Resets for a whole game | 25 | click 19, other 3, tab 1, text 2 | group 3, pane 2, view 4 | none | no whole-game action; the result does not persist |
| T6: clock offset 200 | 10 | click 6, other 2, tab 1, text 1 | group 1, pane 1, view 2 | chooser 1 | no power limit or fan speed control |
| T7: HDR on for 3 games in a row | 19 | click 12, other 3, tab 1, text 3 | pane 3, view 6 | none |  |
| T8: abandon a draft, 10 leave routes | 34 | click 31, digit 2, other 1 | destination 8, group 3, pane 4, view 12 | none | 7 routes drop the draft silently |
| T9: hints off, Steam path, rescan on startup | 7 | click 5, enter 1, text 1 | destination 1, group 2 | none |  |
| T10: notice and undo a wrong value | 7 | click 6, other 1 | group 1, pane 1, view 2 | none | the fan speed half is invisible |

The GUI has no hints concept and no unsaved marker. Its T6 and T10 are partial because the
control for fan speed and power limit does not exist.

## 5. Orientation answers

The four frame questions, as the share of frames in the baseline run that answer `yes`.
Treat these with care:

- `what_is_unsaved` is `no` by rule on any frame with no cue at all, so it is near zero
  on read-only tasks (T3, T4, T10) and is shown here only for tasks where a draft exists.
- The GUI figures are left out on purpose. The GUI stream used its own policies (the GUI
  has no unsaved marker; "launch" is `partial` for raw enum text), so they are not a
  measured parity gap.
- The ratings come from model raters working from shared anchors. Use them to compare
  screens within the terminal interface, not as absolute scores.

| Task (120x40, hints on, baseline run) | Where am I | What is unsaved | Inherited or own, and source | What launch applies |
|---|---|---|---|---|
| T1 near | 4/4 | 1/4 | 2/4 | 3/4 |
| T1 far | 40/40 | 1/40 | 2/40 | 3/40 |
| T2 | 22/22 | 1/22 | 11/22 | 12/22 |
| T6 | 31/31 | 1/31 | 7/31 | 0/31 |
| T7 | 16/16 | 3/16 | 8/16 | 11/16 |
| T8 method 1 | 5/8 | 0/8 | 2/8 | 3/8 |
| T5 | 35/47 | 12/47 | 5/47 | 0/47 |
| T9 | 13/13 | 5/13 | 13/13 | 11/13 |

What the shares say: "where am I" is mostly `yes` at 120x40 and drops at 80x24 because the
game name is not shown. "What is unsaved" is `yes` only on the frame that says "Profile
saved!". "Inherited or own, and the source" never reaches `yes` on an inherited row, because
the source is "Default" and not "All games". "What launch applies" is limited by the cut
Effect line.

## 6. Status of every defect, observation, and decision

One status each: confirmed, revised, or rejected. "Revised" means the finding holds in a
changed form. The deciding evidence is the first item cited.

| ID | Status | What the evidence says |
|---|---|---|
| D1 | confirmed | `docs/profile-fields.md` has no `gpu.vrr` row (`internal/profile/fields.go:66`). The same table also wrongly lists CLI setters for two fields and omits `unset` for two. |
| D2 | revised | Nine labels differ, not five, and they are the labels commit `72f5b0d` changed. The doc column still equals what the CLI prints, and the GUI uses a fourth set, so updating the doc alone leaves three surfaces out of step. `audit/vocabulary-matrix.md` |
| D3 | confirmed | Five groups hold 3 options, split 1/1/1/0/0. `settings/frames/T9-120x40-on-r11-004.txt`. At 80x24 an empty group is a blank box. `internal/config/options.go:111-128` keeps empty groups. |
| D4 | revised | One of 35 descriptors has a description, and no surface shows even that one. Numeric fields have no unit or range, and 99999 W was saved. `single-game/frames/T6-120x40-on-r5-025.txt` |
| D5 | revised | A saved 0 is skipped at launch for fan speed, clock offset, memory offset, and power limit, and blocks the inherited value. No screen says so. `internal/profile/apply.go:65-72,98-109` |
| O1 | confirmed | Three steps deep, and the view resets on every game change. The field cursor also returns to HDR. `single-game/frames/RECON-120x40-r1-020/025.txt` |
| O2 | confirmed | The dialog names "3DMark profile", not the field, and offers no keep-and-move. `single-game/frames/T8-120x40-on-r1-005.txt` |
| O3 | confirmed | The label leaves the view at field 15 (120x40) and field 5 (80x24). `single-game/frames/T6-120x40-on-r3-068/069.txt` |
| O4 | revised | Inherited booleans and enums put `(default)` in the value column and the value in the note. Numbers that inherit a non-zero value show the number. Only the selected overridden row shifts two columns. In All games every row repeats "Default for all games". `single-game/frames/T1-120x40-on-r1-003.txt`, `T6-120x40-on-r1-018.txt`; `defaults/frames/T4-120x40-on-r1-004.txt` |
| O5 | confirmed | Cycling Multi-frame (inherited 0) creates `0 / Override for this game`. The file stores the override with no value. `single-game/yaml/T2-120x40-on-r1-O5-after-3669570.yaml` |
| O6 | revised | The list is 40 rows (5 headers, 35 fields) with no filter or jump. The GUI already has a subsystem rail. "About ten" overrides in real files is a union over five files, and only about six are deliberate. `audit/report.md` |
| O7 | revised | The Actions rows depend on scope and field type: 11 to 17 rows, the first row is `Edit field` or a disabled `Save`, and reset wording is "system default" in All games and "inherit" in a game. `defaults/frames/T5-120x40-on-r1-042.txt`, `T4-120x40-on-r1-009.txt` |
| O8 | revised | The editor shows the effective value as "Value before edit" when it is non-zero, and 0 where the row shows `(default)`. Instructions appear twice. Three Tabs from Input reach Save (a stop-count trap, not a hidden reorder). Enter on Save writes a GPU field with no confirmation. `single-game/yaml/T6-O8-120x40-on-r3-after-762220.yaml` |
| O9 | revised | Each group with options holds exactly one option. An option costs 2 to 5 keys. One Ctrl+S saves every group. `settings/frames/T9-120x40-on-r2-005.txt` |
| O10 | confirmed | Dialogs show 4 content rows (30 of 40 rows empty at 120x40, 14 of 24 at 80x24). Actions leaves 21 to 22 rows empty at 120x40 and 5 to 6 at 80x24. `single-game/frames/T8-120x40-on-r1-005.txt` |
| O11 | revised | Pinned rows look different from inherited ones but do not show the default they oppose. The origin is migration of an emptied profile (F1), not necessarily user edits: DrainSim's five zero pins match what migration creates. `audit/migration-simulation.txt` |

The inherited decisions are priced, not judged: the status is "priced" once the cost is measured, and the user decides at the gate (details in `synthesis/root-causes.md`):

| ID | Decision | Status | Measured cost |
|---|---|---|---|
| G1 | Basic keys only | priced | Arrows are 37 of 40 keys in T1 far, 54 of 66 in T3 scan. A signposted Search would cut T1 far to 9 keys. Backspace is required but shown nowhere (3 per number edit). |
| G2 | Explicit save, guard on any List move | priced | Save is 25 percent of T1 near and 19 percent of T7. The guard costs 3 keys inside the dialog and has no keep-and-move. The unsaved cue scrolls away. The GUI has no guard. |
| G3 | Reset and discard only in Actions | priced | 6 keys for a field reset, 8 plus a confirm for a whole game, 6 and 2 modals for a discard. 30 percent of the T5 keys. Row positions shift with field type. |
| G4 | Four-control editor | priced | Cancel is 3 keys and Save 3 Tabs. A number edit is 3 displayed keys plus 3 Backspaces. Editor Cancel keeps the draft. |
| G5 | Profile is the second view | priced | 2 of 4 keys in T1 near and 6 of 16 in T7. The recon path adds 9 keys to be ready on the next game. |
| G6 | Five fixed Settings groups | priced | 7 of 13 T9 keys move focus or group. Two of five groups are empty. The terminal exposes 3 of 17 options. |
| G7 | Fields ordered by subsystem, all shown | priced | Clock offset is 15 Down from HDR. The first wrong value in T10 takes 27 keys to find. 14 of 35 Effect lines claim an effect that does not happen. |

The accelerator bar (at least 30 percent fewer keys on T1 or T7, or removal of a slip seen in
two or more walkthroughs, while the basic-key path stays complete) is **not met**. Section 4
of `synthesis/root-causes.md` shows why: the measured Search route already cuts T1 far by
77.5 percent, model changes inside the rule are estimated to cut T1 near by 50 percent and T7
by 37.5 percent, and the Backspace slip is fixed by showing the key.

## 7. Key frames

**F1: the screen after the undo says saved and inherited** (`single-game/frames/T10-120x40-on-r1-044.txt`):

```text
    Clock offset          150           ↳ Inherited from defaults
 Profile saved!
```

The stored file is `name: Dome Keeper` and nothing else
(`single-game/yaml/T10-120x40-r1-after-1637320.yaml`). After a restart:

```text
  Profile overrides: 9
  DLLs: none detected
  ◆ Clock offset          0             Override for this game
  > ◆ Fan speed             0             Override for this game
```

**O2: the draft guard dialog** (`single-game/frames/T8-120x40-on-r1-005.txt`; it replaces the whole screen):

```text
  │Unsaved changes                                                                                              
  │                                                                                                             
  │                                                                                                             
  │Continue with changes to:                                                                                    
  │3DMark profile                                                                                               
  │                                                                                                             
  │▸ Cancel    Save and continue    Discard and continue                                                        
  │Tab: next control  ← →: choose  Enter: Cancel                                                                
```

**D3: an empty Settings group** (`settings/frames/T9-120x40-on-r11-004.txt`):

```text
╭ ▸ List ──────────────────────╮╭ Detail ───────────────────────────────────────────────────────────────────────
│DLL policy                    ││Settings                                                                       
│                              ││                                                                               
│←/→ group                     ││                                                                               
│                              ││No setting selected                                                            
│                              ││                                                                               
```

**O8: the editor appends to the prefilled value** (`single-game/frames/T6-120x40-on-r1-020.txt`; the intent was 200):

```text
│> Afterparty                  ││Value before edit: 150                                                         
│  Alchemy Factory             ││150200▏                                                                        
│  BALL x PIT                  ││> Input  Apply  Cancel  Save                                                   
```

**F11: the Search route, filtered by typing "Sat"** (`audit/frames/T1-120x40-on-r3-003.txt`):

```text
╭ ▸ List ──────────────────────╮╭ Detail ───────────────────────────────────────────────────────────────────────
│Scope                         ││[Overview]  Profile  DLLs                                                      
│                              ││                                                                               
│> Sat                         ││Satisfactory Modeler                                                           
│  Satisfactory ● ◆            ││                                                                               
```

## 8. Outside the scope of this review

- **Real data may be affected now.** By the read-only simulation, `~/.config/spela/profiles/1091500.yaml`
  (Cyberpunk 2077, your primary test target) and `1601580.yaml` are `{}`, and `1631270.yaml`
  holds only a name. Each loads with eight phantom pins. Files already saved with pins,
  like DrainSim, cannot be told apart from deliberate choices; only you can say how to
  treat them. A possible workaround for the `{}` and name-only files is
  `spela profile delete <appid>`: on a jailed fixture it returned every field of a `{}`
  profile to `[inherited]` (`audit/cli/parent-profile-delete-workaround.txt`). It would also
  delete deliberate overrides, so it only suits files that hold none. I did not run it on
  real state.
- **The uncommitted dependency update breaks `mage build`.** Wails 2.16.0 with clir 1.7.0
  loses its flag defaults, so the bindings step fails (`baseline/builds.md`). Frames from a
  hand-built binary with the update matched the committed build on one journey.
- **The `termctrl` on PATH is 0.3.1** and corrupts frames without an error. Every stream used the
  1.2.1 build by absolute path.
- **`spela overlay show <id>`** runs `/usr/bin/overlay` (mesa-demos) when it exists, because
  the wrapper-mode check runs before the command parser (`cmd/spela/main.go:113-130`).

## 9. Before any alternative

### 9.1 F1 is a precondition, not part of an option

Autosave plus a revert that does not persist is worse than today, so the fix comes first in
every implementation plan and can ship alone.

- **What to change.** Always write the `overrides` key, even when it is empty
  (`internal/profile/profile.go:40-46` drops an empty map through `omitempty`), and run legacy
  migration only for files that lack the key (`internal/profile/storage.go:50-60`). Keep
  `migrateInheritance` (`internal/profile/inheritance.go:264-325`) for genuine legacy files.
- **Tests to add.** Reset the last override, then reload under a non-empty default; `profile
  create`, then a first edit, then reload; Reset all, then reload. No current test does any of
  these (`synthesis/root-causes.md`, section 4).
- **Files already damaged.** A saved file with phantom pins cannot be told from deliberate
  pins (DrainSim has four non-zero values beside five zero pins). Only you can decide
  how to treat them: leave them, or offer a one-time repair that lists the suspected phantom
  pins and asks. By the read-only simulation, `1091500.yaml` (Cyberpunk 2077) and
  `1601580.yaml` are `{}` and `1631270.yaml` holds a name only, so each loads with eight
  phantom pins today. `spela profile delete <appid>` returned every field of a `{}` fixture
  profile to `[inherited]` (`audit/cli/parent-profile-delete-workaround.txt`). It is only safe
  for a file that holds no deliberate overrides, and I did not run it on real state.

### 9.2 One vocabulary and inheritance model, for the TUI, GUI, and CLI

Direction 3 asks for one profile vocabulary on both surfaces. This table is shared by A, B, and
C. It replaces `(default)`, "Override for this game", "Inherited from defaults", "Default for
all games", and the GUI's Pin, Unset, and Reset. Field labels are the descriptor labels in
`internal/profile/fields.go`, and the document, the GUI, and the CLI conform to them (D2).

| Idea | Words on screen (TUI and GUI) | CLI words | Stored in YAML |
|---|---|---|---|
| The value comes from All games | source column: `from All games`; action: **Follow All games** | `follows All games` | key absent from `overrides` |
| The game has its own value | marker ◆, source `this game · All games: Off` | `this game` | key in `overrides`, value stored |
| Own value that equals All games | `this game · same as All games` (it will not follow later changes) | `this game (same as All games)` | key in `overrides`, equal value |
| Explicitly off or empty | `Off` or `none`, never `(default)` | `off`, `none` | `false` or `""` in `overrides` |
| The defaults profile | **All games** everywhere (never "defaults profile" or "root profile") | `All games` | `default.yaml` |
| No value in All games | `system default` | `system default` | key absent |
| Clear a game's value | **Follow All games** | `reset` prints `now follows All games` | remove the key from `overrides` |
| Clear an All games value | **Use system default** | `reset` prints `now uses the system default` | remove the key |
| Clear a whole game | **Follow All games for all settings**, with a confirm that counts the settings | `profile reset <game>` (new) | `overrides: {}` stays |
| Booleans | `On` and `Off` | `on`, `off` | `true`, `false` |

`profile show` prints the effective value and its source, like `gpu show`. Retired words:
`(default)`, "Override for this game", "Inherited from defaults", "Default for all games",
Pin, Unset, "defaults profile", "root profile".

## 10. The three alternatives

All three use the vocabulary above and the F1 fix. All three keep commands to arrows, Tab,
Enter, Space, digits, Ctrl+S, and typed text inside an input. None adds an accelerator. They
form a ladder: A fixes what the screens say, B also changes how you move and save, C also
unifies Settings with the profile form. All mock frames are in
[tui-profile-settings-ux-review-mocks.md](tui-profile-settings-ux-review-mocks.md); each
section quotes the 80x24 profile frame.

### 10.1 A: Clarify the current model

- **Idea.** Keep Overview first, the views, explicit save, and the draft guard. Fix what the
  screens say and add the missing signposts.
- **TUI changes.**
  - Value column shows the effective value and its source in every row, with units for
    numbers (F3, F8), for example `this game · All games: Off`.
  - A pinned status line under the tab strip names the unsaved fields, so the cue cannot
    scroll away (F5). The draft dialog names the fields, offers Keep editing, and is sized to
    its content so the page stays visible (F6, O10).
  - A "Find games" row at the top of the List makes Search discoverable (F11).
  - All games rows show which games set their own value (T3, F4), so the answer is on the
    same screen as the edit.
  - Reset rows come first in Actions (F9). Typed text replaces the prefilled number and
    Backspace is shown (F7). The Effect line wraps instead of being cut (F13). The 80x24
    Profile view names the game (F14).
  - Settings: one list, empty groups hidden, options edited in place (F15).
  - Show hints is wired to the key bar or removed (F16).
- **GUI changes.** The same words and source column. A dirty marker and the pane's save
  status where the user can see them. A confirm on leaving with a draft that lists the
  fields. "Follow All games for all settings" as one action (F17).
- **Contracts that change.** Profile storage marker (F1), value and source text, draft
  dialog text and choices, Actions row order, Settings group list, List top row.
- **Test churn (estimate).** About 17 files: `detail_test.go`, `draft_navigation_test.go`,
  `options_modal_test.go`, `editor_actions_test.go`, `help_test.go`, `layout_test.go`,
  `space_controls_test.go`, the two e2e journey files, the profile reload tests, and the
  GUI `GameDetail.test.js` and `supportedFlows.test.js`.
- **Risks.** Low. The cost is that T7 and the recon path stay as they are: A removes no
  navigation keys. The blocking dialog stays, though it now names the fields.
- **Patterns borrowed.** Source markers (nmtui `(default)`, git `--show-scope`), a key bar
  that follows context (lazygit).

```text
 GPU  30°C   0%   5W  CPU   3% 2997MHz
 VRAM 11.0/12.0G  RAM 17.4/30.4G
────────────────────────────────────────────────────────────────────────────────
◆ 1 Library  2 DLL Catalog  3 Monitor  4 Settings
╭ ▸ Detail ────────────────────────────────────────────────────────────────────╮
│Overview  [Profile]  DLLs                                                     │
│Cyberpunk 2077 · Saved · nothing unsaved                                      │
│Proton                                                                        │
│    ◆ HDR                   On          this game · All games: Off            │
│      Wayland               On          from All games                        │
│      NGX updater           Off         from All games                        │
│  > ◆ VKD3D heap            On          this game · same as All games         │
│      Applied at game start, undone at exit.                                  │
│DLSS                                                                          │
│    ◆ Super resolution      quality     this game · All games: none           │
│      Resolution preset     L           from All games                        │
│      Resolution override   Off         from All games                        │
│      Ray reconstruction    none        from All games                        │
│      Ray preset            none        from All games                        │
│                                                                              │
╰──────────────────────────────────────────────────────────────────────────────╯

0: Actions  Tab: Switch pane  ←: Previous view  →: Next view  ↑: Previous field
↓: Next field  Space: Next value  Ctrl+S: Save
```

### 10.2 B: Profile-first, autosave, per-field revert

- **Idea.** Everything in A, plus a different flow. Enter on a game opens the Profile list
  directly; the view and the field cursor stay when you change games; every applied edit is
  saved at once.
- **TUI changes.**
  - Overview is gone as a view. Its facts become a two-line header, and the header lists the
    game's own settings by name and value (replaces "Profile overrides: 5"; F2, T4, T10).
  - Left and Right jump to the previous or next section (replaces the view keys). A number
    field is edited with Enter; Space on it switches between **Follow All games** and the last
    own value, so a revert is one key and the undo is the same key.
  - Enum fields open a picker with type-ahead (`K` is Enter, `K`, Enter). Booleans cycle:
    follow, then the opposite of the effective value, then the same value (pinned).
  - A status line says what was saved ("Saved: HDR On"). There is no draft, no Ctrl+S, and no
    draft dialog. The one half-made state is an open editor, and it has Cancel.
  - A whole-game reset is a confirmed Actions row that counts the settings it will clear.
- **GUI changes.** The profile pane saves each control as it changes (the backend already
  takes patches), the Profile tab opens first, the Save button goes away, Reset becomes
  **Follow All games**, Pin becomes "Set for this game", and the same own-settings header
  appears. The section rail keeps its place across games.
- **Contracts that change.** Everything in A, plus the navigation contract (`internal/nav`
  opens a game on Profile and keeps the view), the save model (per-field atomic writes; the
  draft guard and decision dialogs are removed), the editor controls, and the meaning of Left,
  Right, and Space.
- **Test churn (estimate).** About 34 files: A's files plus `nav_test.go`, `gui_test.go`,
  `shell_contract_test.go`, `active_state_contract_test.go`, `rescan_lifecycle_test.go`,
  `editor_host_test.go`, `profile_editor_contract_test.go`, `keymap_test.go`, all three e2e
  files, and the GUI `App.test.js`, `profileAndListFlows.test.js`, and three Playwright specs.
- **Risks.**
  - A stray key now writes to a profile at once. The status line and the one-key undo are
    the mitigation, and nothing reaches hardware until launch.
  - No studied terminal tool offers per-field undo (`reference/reference-tools.md`), so the
    Space-follow idea has no precedent to copy.
  - The CLI and the TUI may write the same file; per-field writes need the existing atomic,
    locked update path.
  - F1 must land first.
- **Patterns borrowed.** Source markers; section jump (btop category tabs, nmtui Hide and
  Show); a context key bar (lazygit).

```text
 GPU  30°C   0%   5W  CPU   3% 2997MHz
 VRAM 11.0/12.0G  RAM 17.4/30.4G
────────────────────────────────────────────────────────────────────────────────
◆ 1 Library  2 DLL Catalog  3 Monitor  4 Settings
╭ ▸ Detail ────────────────────────────────────────────────────────────────────╮
│Cyberpunk 2077 · Saved at once                                                │
│Own (5): HDR, VKD3D heap, Super resolution, Frame generation, Clock offset    │
│Proton                                                                        │
│    ◆ HDR                   On          this game · All games: Off            │
│      Wayland               On          from All games                        │
│      NGX updater           Off         from All games                        │
│  > ◆ VKD3D heap            On          this game · same as All games         │
│      Applied at game start, undone at exit.                                  │
│DLSS                                                                          │
│    ◆ Super resolution      quality     this game · All games: none           │
│      Resolution preset     L           from All games                        │
│      Resolution override   Off         from All games                        │
│      Ray reconstruction    none        from All games                        │
│      Ray preset            none        from All games                        │
│      Ray override          Off         from All games                        │
╰──────────────────────────────────────────────────────────────────────────────╯

0: Actions  Tab: Switch pane  ↑ ↓: Field  ← →: Section
Space: Next value  Enter: Edit value
```

### 10.3 C: One form for profiles and Settings

- **Idea.** Everything in B, plus Settings and every profile scope use one form component,
  with sections, a "Changed here" filter, and one save model.
- **TUI changes.** The profile form for All games, for a game, and for Settings is one
  component. Settings groups become sections of one page; empty ones disappear. A filter row
  (Up from the first field, then Space) shows only the settings changed here, which turns T10
  into reading two rows. The filter is off by default, so every field stays reachable (G7).
- **GUI changes.** One Svelte form component for the profile pane and the Settings pane, the
  same filter chip, the same source column.
- **Contracts that change.** Everything in B, plus the form component API, the Settings
  section model (`internal/config/options.go` `Sections`), and per-option autosave.
- **Test churn (estimate).** About 40 files: B's plus `options_modal_test.go`,
  `navigation_settings_views_test.go`, the config option tests, the Settings e2e and GUI tests.
- **Risks.** The widest rewrite. A shared component can force compromises for the path
  editor and the profile editor. The gain over B is small: 3 keys on T10, and a consistent
  look for a screen that holds three options.
- **Patterns borrowed.** nmtui's single form with collapsible sections; the changed-only
  filter (k9s filters, the VS Code `@modified` filter).
- **Optional add-on, a decision for you.** Edit all selected games at once: the List already
  selects games with Space, and an edit would apply to the selection. T7 would cost 7 keys.
  It needs a rule for mixed values and for undo.

```text
 GPU  30°C   0%   5W  CPU   3% 2997MHz
 VRAM 11.0/12.0G  RAM 17.4/30.4G
────────────────────────────────────────────────────────────────────────────────
◆ 1 Library  2 DLL Catalog  3 Monitor  4 Settings
╭ ▸ Detail ────────────────────────────────────────────────────────────────────╮
│Cyberpunk 2077 · Saved at once                                                │
│Show:  All   [Changed here]                                                   │
│Proton                                                                        │
│    ◆ HDR                   On          this game · All games: Off            │
│    ◆ VKD3D heap            On          this game · same as All games         │
│DLSS                                                                          │
│    ◆ Super resolution      quality     this game · All games: none           │
│    ◆ Frame generation      On          this game · All games: Off            │
│GPU                                                                           │
│  > ◆ Clock offset          100 MHz     this game · All games: 150 MHz        │
│      -500 to +1000 MHz. 0 leaves the clock unchanged.                        │
│      Space: Follow All games (150 MHz)                                       │
│                                                                              │
│                                                                              │
│                                                                              │
╰──────────────────────────────────────────────────────────────────────────────╯

0: Actions  Tab: Switch pane  ↑ ↓: Field  ← →: Section
Space: Next value  Enter: Edit value
```

## 11. Estimated key counts

These are estimates from explicit key sequences (`synthesis/alt-metrics.py`), not
measurements. The "Now" column is the measured baseline, and the script checks that its
decomposition of each measured path reproduces the measured total, so only the changes are
estimated. Counts are the same at 120x40 and 80x24, because both sizes use the same keys;
the frames show how each option fits. Where an alternative changes nothing, the count equals
the baseline. T3 includes finding the games that keep the old value. T6 "Now" excludes the 3
undisplayed Backspaces for the clock offset. T8 compares the cost of a finished abandon: under
B and C no draft exists to abandon, because every applied edit is saved and undone with the same
key, so the only half-made state is an open editor, whose Cancel costs 3 keys.

| Task | Now | A clarify | B profile-first, autosave | B with explicit save | C one form |
|---|---:|---:|---:|---:|---:|
| T1 near | 4 | 4 (+0%) | 2 (-50%) | 3 (-25%) | 2 (-50%) |
| T1 far | 40 | 8 (-80%) | 6 (-85%) | 7 (-82%) | 6 (-85%) |
| T2 | 22 | 22 (+0%) | 10 (-55%) | 11 (-50%) | 10 (-55%) |
| T3 | 26 | 7 (-73%) | 6 (-77%) | 7 (-73%) | 6 (-77%) |
| T4 Cyberpunk | 29 | 29 (+0%) | 15 (-48%) | 15 (-48%) | 15 (-48%) |
| T4 DrainSim | 36 | 36 (+0%) | 22 (-39%) | 22 (-39%) | 22 (-39%) |
| T5 | 47 | 40 (-15%) | 21 (-55%) | 23 (-51%) | 21 (-55%) |
| T6 | 28 | 28 (+0%) | 13 (-54%) | 14 (-50%) | 13 (-54%) |
| T7 | 16 | 16 (+0%) | 10 (-38%) | 13 (-19%) | 10 (-38%) |
| T8 method 1 | 5 | 5 (+0%) | 3 (-40%) | 3 (-40%) | 3 (-40%) |
| T8 method 2 | 6 | 6 (+0%) | 3 (-50%) | 3 (-50%) | 3 (-50%) |
| T8 method 3 | 3 | 3 (+0%) | 3 (+0%) | 3 (+0%) | 3 (+0%) |
| T9 | 13 | 8 (-38%) | 8 (-38%) | 8 (-38%) | 8 (-38%) |
| T10 | 44 | 36 (-18%) | 19 (-57%) | 20 (-55%) | 16 (-64%) |
| RECON | 25 | 25 (+0%) | 14 (-44%) | 15 (-40%) | 14 (-44%) |
| T7 with bulk edit (add-on to C) | 16 | not applicable | not applicable | not applicable | 7 (-56%) |

Assumptions behind the sequences:

- All options: F1 fixed; the Find row makes Search discoverable (Up from All games reaches
  it); the first match for the typed name is the target; All games rows list the games that
  set their own value.
- A: the baseline flow stays (Overview first, Right opens Profile, the view and the cursor
  reset on a game change, explicit save). Reset rows come first in Actions.
- B: Enter opens Profile with the cursor on the first field; the cursor stays on the same
  field across games; Right moves to the first field of the next section (Proton, DLSS, GPU);
  Space on a number row follows All games; an enum opens a picker with type-ahead; a whole-game
  reset is `0`, Enter, then Right and Enter on the confirm.
- B with explicit save adds one Ctrl+S per edited game and keeps a draft guard.
- C: B plus a filter row one Up above the first field, and the cursor lands on the next own
  row after a revert.
- Not modelled: the extra Down presses a long enum picker might need, and screen-size
  scrolling beyond the measured Down counts.

## 12. Recommendation

Take **B**, delivered in two steps, and fix F1 before either.

0. **F1 on its own.** It is a correctness bug that affects real files today.
1. **The A work first** (vocabulary, source column, named unsaved status, Find row, All games
   divergence note, Actions order, typed-replace editor, Settings list). It removes the
   comprehension problems without changing how anyone moves or saves.
2. **B's flow second** (Profile first, sticky view and cursor, section jump, autosave with
   Follow). This is where the key savings are.

Why B and not A alone: A removes no navigation keys. T7, the recon path, and the Actions-only
reset are structural, and only B changes them: T7 16 to 10, T5 47 to 21, T10 44 to 19, T6 28 to
13 (31 to 13 counting the undisplayed Backspaces). Why B and not C: C saves 3 more keys on T10
and gives one consistent form to a Settings screen with three options, for the largest
rewrite. The extra can wait until it is wanted.

If you decline autosave, the next best is B with explicit save (T7 13, T1 near 3) plus a
redesigned guard, or A alone. The GUI work belongs in the same plan whichever you choose,
because parity is a priority.

## 13. Decisions left to you

1. **Which alternative:** A, B (recommended), or C.
2. **Autosave for profile fields:** yes (B, C) or keep explicit save (A, B with explicit save).
3. **F1:** fix it now as its own change (recommended). And what to do about files already
   saved with phantom pins, including your `1091500.yaml`, `1601580.yaml`, `1631270.yaml`, and
   DrainSim: leave them, repair with a prompt, or use `spela profile delete` for the `{}`
   files.
4. **Show hints:** wire it to the key bar, or remove the option.
5. **Booleans:** `On` and `Off` instead of `true` and `false` on screen.
6. **Bulk edit for selected games** (the C add-on): wanted or not.
7. **Settings:** hide empty groups only (A), or one form (C).
8. **Commit** the review documents and Agentera state, leaving out the dependency work.
9. **Remove** the two build worktrees under `~/.cache/spela-ux-review-build/`.

## 14. Why there is no prototype

The measurable part of the choice is settled without one: the baseline is measured, each
estimate changes only what its contract changes (and the script rejects a decomposition that
does not reproduce the measured total), and every mock was checked to fit 120x40 and 80x24.
What is left is a preference: whether you accept autosave and the new meanings of Left, Right,
and Space. A prototype would restate the estimates, not settle that. If you pick B, the first
step of its implementation plan can be a throwaway prototype of the navigation to measure T1,
T7, and the recon path before the full build.

## 15. Backlog entries

Agentera TODO ids, readable with `npx -y agentera@next state todo get --id <id>`. The five
registered defects were updated with the review's findings; the others are new. Items whose
details came from a stream and were not re-checked say so in their requirements.

| Finding | TODO | Severity |
|---|---|---|
| F1 reset does not persist | `nsbxkoodkq` | critical |
| D1 `gpu.vrr` missing from the field document (updated) | `kjutocdjaz` | normal |
| D2 labels differ across surfaces (updated) | `srryekxvmq` | normal |
| D3 empty Settings groups (updated) | `hpuwnhwspr` | normal |
| D4 no descriptions, units, or ranges (updated) | `hxvtaqzzfg` | normal |
| D5 meaning of a saved 0 (updated) | `bdnaynbvan` | normal |
| F5 unsaved cue scrolls away | `kjlylvrrvm` | degraded |
| F6 draft dialog | `xafqoplfhu` | degraded |
| F7 field editor | `uzofpmrkiu` | degraded |
| F9 Actions menu | `ypsvpwcveu` | annoying |
| F13, F14 cut text and no game name at 80x24 | `lnxsjtjtrr` | degraded |
| F15 Settings safety and unread GUI settings | `wqnnfzjhpm` | annoying |
| F16 Show hints | `kjxiwkpgvj` | annoying |
| F17 GUI loses drafts | `hwddfdauhf` | degraded |
| F17 GUI dropdown hides stored values | `rpwrxivkcm` | annoying |
| F17 GUI lacks fan speed, power limit, whole-game reset | `piuftqpwst` | normal |
| F18 CLI default and clear words | `rgddfoguos` | annoying |
| F18 `profile show` is raw | `wzqekfgzdh` | degraded |
| F18 one set of state words | `wuaaeuetff` | degraded |
| F19 fields that do nothing | `xuyalcslma` | degraded |
| F20 GPU offsets act as clock caps (code inspection) | `wzpctclhaa` | degraded |
| F20 restore gaps after a failed apply (code inspection) | `iynrkonane` | degraded |
| F21 `overlay` program shadows the subcommand | `cridhyzjcj` | degraded |
| The redesign itself (waits for your choice) | `qiqoepaeuz` | normal |

Not filed as separate items: F2 (inside the F1 item), F3, F10, F11, and F12 (carried by the
redesign item), and F4 (the description and state-word items).
