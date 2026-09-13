# Basic-key TUI navigation and audit remediation

Status: completed and verified on 2026-09-13. Acceptance is recorded below.
Prepared on 2026-09-13 against
`95cf340` (v0.6.1). Planning task: `spela-uzqw`. Implementation epic: `spela-fi78`.

## Objective and scope

Make every currently supported TUI workflow discoverable and usable through
arrows, Tab, Enter, Space, and numbers. Ctrl+S is an optional save shortcut; a
selectable Save action must provide the same result. Every active key hint must
work in the destination, pane, mode, and overlay where it appears.

This is a command and navigation requirement. Search terms, paths, and profile
values still accept ordinary text, including spaces, punctuation, non-English
characters, and negative numbers. Normal text editing remains local to the input.
Typing content must never invoke a command. No workflow may require Escape,
Shift+Tab, a letter shortcut, a punctuation shortcut, or a function key.

Preserve the existing profile/configuration formats, DLL safety and backup rules,
save serialization, and shared destination/selection meanings. Use the existing
action model and dependencies. This plan does not add game launching, expose new
Settings options, change GUI shortcuts, or change GPU/CPU privilege behavior.

The deliverable is a working TUI, updated navigation and testing documentation,
targeted regression tests, and rendered termctrl acceptance evidence. This file records the approved implementation scope;
[the navigation contract](tui-navigation.md) describes the implemented behavior. Older completed task records do not prove the audited behavior works.

## Audit baseline and required fixes

The preceding audit used the compiled executable, termctrl 0.3.1, isolated XDG
state, three fixture games, and temporary DLL files. It recorded 245 termctrl
commands and 86 successful nonblank rendered screens. The evidence remains at
`/home/jgabor/.cache/spela-tui-audit/spela-visible-keys-6uhotcdf/`, including
`report.md`, `commands.jsonl`, and numbered screen files. The summary below is
kept here so the plan does not depend on retaining that cache.

| Finding | Observed behavior and source | Required result | Work |
| --- | --- | --- | --- |
| F1: hints ignore focus | Library view brackets and Catalog update-all appear with List focus but require Detail. Settings List arrows change groups while the Detail hint says change value. `resource_view.go`, `options_modal.go`, `keymap.go`. Screens 004, 020, 022, 044, 047, 071. | Display only executable shortcuts for the active pane. Describe group/view movement separately from value editing. Expose catalog update-all as an explicit destination action. | A, C, D |
| F2: root reset and discard are blocked | Default-profile reset and Browse cancellation are displayed but rejected by a game-only availability gate. `libraryProfileAvailable` in `keymap.go`. Screens 031–033. | Root reset uses system defaults; per-game reset uses inheritance. Both scopes can discard their own draft. | A, B |
| F3: reopening loses a dirty draft | A root draft survives a destination detour, then Enter on the same root item reloads saved values. `refreshDefaultsDetail` and `ContentModel.SetGame` also expose a broader replacement risk. Screens 035–036. | Reopening the same item preserves its draft. Any transition that would replace a dirty draft must resolve Save, Discard, or Cancel first. | B |
| F4: Settings Ctrl+S is a no-op during path edit | The hint promises applying input, but the Settings editor handles only Enter. Profile Ctrl+S currently means apply to draft, not persist. `options_modal.go`, `editor_host.go`, `keymap.go`. Screens 064–067. | Ctrl+S validates the active input and persists its owning draft. Enter applies a field to the draft. Labels distinguish the two. | B |
| F5: Monitor Enter is a no-op | Generic Open selection is displayed, but the Monitor List handles only movement. `list_pane.go`, `keymap.go`. Screens 012–015. | Enter opens the selected source in Detail, with useful inspection/scrolling and a visible return path. | C |
| F6: read-only panes advertise editing | DLL Detail shows field navigation, Edit, and Save with no corresponding action. Generic Detail bindings lack capability gates. Screens 045–047. | Read-only panes expose inspection, scrolling when needed, and applicable DLL actions. No Edit or Save hint. | A, C, D |
| F7: controls are missing or clipped | Status drops most globals and uses half the width; Help lists only its own close/quit keys. Tab, search, and layout controls are undiscoverable. At 80×24, values, Help/Quit access, and confirmation policy are lost. `layout.go`, `help.go`. Screens 003, 007, 016, 036, 048, 085, 088. | Reserve essential controls, provide full scoped Help, and keep every workflow operative at 80×24, including with verbose hints disabled. | A, C, D |
| F8: repeated rendering becomes corrupt | Metric digits concatenate, rows mix, and the header can disappear after repeated navigation/resizing. A fresh session recovers. Screens 020, 035, 055, 059. Ownership between app rendering and termctrl is unproven. | Reproduce actual terminal resizes, isolate the cause, correct it at the owning layer, and retain rendered regression evidence. | E, F |

Existing passes to preserve include destination selection, filtering/sorting,
Space selection, profile and Settings persistence, empty-library rescan recovery,
live Monitor views, and fixture DLL install/update/restore with cancellation.
Real game launch, privileged tuning, network downloads, and real DLL version
parsing were not validated by that audit.

## Proposed interaction contract

### Shell and focus

Keep the two shell panes, List and Detail. The destination bar remains outside
the focus cycle. Menus, editors, and dialogs own a local focus cycle while open;
they do not add a shell pane. Exactly one input target has visible focus.

| Key | Browse behavior | Editor or overlay behavior |
| --- | --- | --- |
| `1`–`4` | Library, DLL Catalog, Monitor, Settings. Display the mapping in the destination bar. | Input where applicable; never switch destinations. |
| `0` | Open **Actions** for the current context, from either pane. Always display this entry point. | Input in text fields; otherwise no global action. Close the current layer before opening another. |
| `Tab` | Switch List/Detail when the other pane supports interaction. | Cycle visible input areas and buttons, wrapping within the current layer. |
| `↑` `↓` | Move through List rows or editable Profile fields; scroll read-only Detail when it overflows. | Move menu/choice rows or scroll the focused body. Text fields retain local cursor behavior. |
| `←` `→` | Change a named List group in Settings/Catalog, or Overview/Profile/DLLs in Library Detail. No Browse value adjustment. | Adjust an explicit choice editor, move a text cursor, or move between dialog buttons when the button row has focus. |
| `Enter` | Open the List selection in Detail, or start editing an editable field. With selected games, open the clearly labelled batch-actions menu. | Activate the focused control. From an input, apply the field to the draft; from a menu, execute the selected action. |
| `Space` | Toggle game selection in Library List. | Toggle the focused checkbox/boolean or selection row. In text input, insert a space. Never implicitly confirm a mutation. |
| `Ctrl+S` | Save the active Profile or Settings document where its save action is available. | Validate the current field, apply it, and save its owning document. |

In Search, Edit, and overlays, remove active `1`–`4` shortcut labels from the
destination bar and hide the Browse `0 Actions` hint. Keep the destination names
for orientation. The visible key hints must describe the owning layer, including
its local Tab cycle, rather than suggesting that underlying commands still run.

For Library, the save context is the focused Profile Detail, including the
default profile, or its active field editor. List, Overview, DLLs, Catalog, and
Monitor do not advertise Ctrl+S. Settings has one configuration draft, so save is
available from either pane and its editor. An unrelated overlay does not pass
Ctrl+S through to an underlying document.

Opening a game still starts at Overview. Opening the default profile starts at
its Profile fields. Opening Monitor or a Catalog selection focuses its Detail.
An unavailable pane is skipped; an empty Library must still offer rescan and
Actions. At the supported compact size, Tab reveals and focuses the other pane
as one transition. Hidden panes receive no input.

Remove the old punctuation, letter, function-key, and extra modifier command
bindings when their replacements land. Text entry remains a separate input
path. Preserve terminal interrupt/restoration behavior as an emergency lifecycle
path, outside the normal workflow and acceptance key set.

### Actions, Help, and cancellation

Use one Actions overlay backed by the existing semantic action registry. It has
contextual rows followed by global rows, a stable order, a visible selected row,
and a pinned Close button. Up/Down moves rows, Enter invokes a row, and Tab moves
between the list and Close. A child chooser replaces the overlay and provides
Back and Cancel buttons; it does not stack shell focus zones.

Contextual rows name their target and, for batch actions, their scope and count.
Pane-specific actions belong to the invoking pane. Destination-wide actions,
such as Catalog Deployment update-all, are available from either pane when
their prerequisites hold. Disabled rows explain the reason and cannot dispatch;
unrelated commands are omitted. Do not show an active Enter-to-run hint on a
disabled row. Recheck availability before executing an action
or committing a confirmation because scans and background operations can change
the target state.

The menu must expose the following existing capabilities:

| Context | Actions or selectable controls |
| --- | --- |
| Global | Help, layout choices (Standard, Compact, Focused), Quit |
| Library List | Search, DLL filter, profile filter, explicit choice of all four current sort modes, Clear filters, Select all filtered games, Clear selected filtered games, Update selected games, Rescan games |
| Library Profile | Edit, Save, Discard draft, Reset selected field, Reset all fields; root labels say System default, game labels say Inherit |
| Library DLLs | Install with family/version selection, Update stale DLLs for the named game, Restore available backups |
| Catalog | Library/Deployment group selection and current metadata/cache/deployment inspection; Update all stale deployments from the Deployment group |
| Monitor | GPU, CPU, Alerts selection and inspection; no invented edit/save actions |
| Settings | Existing group/setting selection, Edit, Save settings, Discard settings draft |

Select all must enter selection mode itself. It acts on filtered games, never
the default-profile row. Clear selected filtered games retains any selection
outside that scope. Show the total selected count separately from the visible
count, and name the full target set in batch confirmation. Opening a batch menu
must not open a game or begin an update. Explicit sort choices replace cycling
an undiscoverable key; preserve the current clear-filter/sort behavior and label
any sort reset it performs.

Search has a focused text input and visible Apply, Clear search, and Cancel
controls. Results and the scope label update together. Apply retains the query
and returns to List Browse. Cancel restores the query/scope from before entering
Search; Clear search removes the filter explicitly. Apply any dirty-profile
transition guard before a result change replaces the selected profile.

Help shows controls for its own scrolling and closing separately from reference
tables for destinations, panes, editors, and dialogs. Reference bindings state
their required focus/mode and any unavailable reason; they are not presented as
active Help shortcuts. Capture the context that opened Help so it can explain
the user's current actions. Tab then Enter must always close Help.

Every editor, chooser, result overlay, confirmation, and Help view has selectable
Cancel, Back, or Close controls as applicable. Tab always reaches them. Closing
returns to the captured pane and row, normalizing only if the target disappeared.
Quit is reachable through Actions and uses the pending-operation and dirty-draft
guards below. In-progress work is distinct from a cancellable chooser.

### Editing, saving, and draft ownership

Use the same visible editor lifecycle for profiles and Settings:

1. Enter starts a field edit without changing the saved value. The local cycle
   is Input, Apply, Cancel, Save. Omit Save only where persistence is not a
   capability. Choices and booleans use arrows/Space; text and numeric fields
   accept their normal content. Input widgets may expose a Clear control for
   replacing text, with a visible label.
2. Enter from Input, or Apply, validates and applies only that field to the
   owning draft. Return to Browse and label the document Unsaved.
3. Cancel discards only the active field edit and restores its value from the
   draft at edit entry. It must not discard other previously edited fields.
4. Save or Ctrl+S validates the input, applies it, and persists the owning draft.
   Invalid input stays open with its error and performs no write. Saved feedback
   appears only after persistence succeeds. Save failure keeps the draft and a
   usable retry/cancel path.
5. Discard draft is a separate Actions command. Root reset restores system
   defaults; game reset removes the relevant override to inherit. Reset changes
   a draft, not storage. Reset-all and whole-draft discard name the affected
   document in a confirmation.

Reopening the same root/game must reuse its draft. Destination detours and detail
view changes retain drafts without prompting when they do not replace them.
Before a selection, rescan result, scope reload, or Quit would destroy dirty
state, show the affected document(s) and **Save and continue**, **Discard and
continue**, **Cancel**. Initially focus Cancel. Do not advance the selection or
replace the model until the decision succeeds. Cancel or a save failure leaves
the original draft, cursor, focus, and scope usable. If Quit involves both a
profile and Settings draft, list both, save each through its existing persistence
path, and do not quit on a failure.

Keep one current profile draft plus the existing Settings draft; no general
multi-document draft cache is required. Retain profile save snapshots and the
existing serialized/coalesced queue. An earlier save completion must not erase
later edits or mark them saved. Where saving blocks controls, show Saving and
disable those controls consistently. Repeated Ctrl+S must follow the existing
queue rules, not launch duplicate independent writes.

Normal Quit must also wait for all dispatched or queued saves and DLL mutations,
including a save owned by a previously viewed profile. While work is pending,
keep Quit unavailable with its reason and show the affected operation. Keep the
application running to receive completion/failure results; do not detach the
write or treat closing a view as completion. Once it settles, reevaluate dirty
drafts and failures before allowing Quit. An operation failure remains visible.

### Confirmations and small screens

Mutation dialogs initially focus **Cancel**. Tab moves through the scrollable
body and button row; left/right changes the focused button and Enter activates
it. Merely pressing Enter after opening a dialog must cancel, not mutate files.
The body shows the target games/files, versions where known, and backup policy.
The body can scroll; Cancel/Confirm remain visible before dispatch. Cancelling a
chooser or confirmation performs no mutation and records that outcome. After
Confirm dispatches work, show progress and disable further mutation/confirmation
input until the operation settles. Do not advertise Cancel when the underlying
operation cannot stop safely. Keep the progress view visible, explain that work
is running, and provide Close on its success/failure result. Result dismissal
does not undo completed work. Do not add rollback or a cancellation API here.

Reserve the destination bar, active-pane label, available `Tab` and `0 Actions` controls,
contextual Enter/Space/save guidance, and state/error feedback before allocating
content space. Use separate footer rows when needed. Shrink breadcrumbs and
decorative header space before dropping controls. Inactive panes show content
without actionable key hints. Show hints=false may hide explanatory shortcuts,
but cannot hide the navigation/Actions entry points or local dismissal controls.
If no other pane is eligible, omit the active Tab hint and explain that state in
Help instead of advertising a switch that cannot occur.

At 120×40, retain the side-by-side view. At widths below 100 columns or heights
below 30 rows, use a compact header and one full-width active pane, with Tab
revealing the other. This automatic constraint takes precedence over the user's
layout preference; preserve that preference for the return to a larger viewport.
Use the existing layout modes for explicit Standard/Compact/Focused choices.
At 80×24, selected and draft values, Help, validation, and confirmation policy
must be readable through bounded content scrolling with pinned controls.

Below 80×24, show the bounded resize prompt and current size. Suspend workspace
and overlay actions, preserve their state, and restore them on a supported
resize. Keep terminal lifecycle handling; do not add an unguarded Quit shortcut
that could discard a draft while the guard cannot fit.

## Implementation work and dependencies

All implementation beans start as `todo` under epic `spela-fi78`.

| Work | Bean | Implementation and ownership | Depends on | Exit evidence |
| --- | --- | --- | --- | --- |
| A: action and focus foundation | `spela-kbvq` | Extend `keymap.go` and action context with draft/busy state, selected target/count, editor kind, and read-only/scroll capabilities. Dispatch semantic actions through `layout_handlers.go`, `list_pane.go`, `resource_view.go`, and affected widgets; only text editors receive raw text input. Add the Actions overlay and its local focus controls. Update `docs/design/tui-navigation.md` for the new contract. | None | Table-driven mode/focus/availability tests; menu and keyboard dispatch the same action; termctrl can reach a menu, Help, Close, and pane/destination navigation using displayed basic keys. |
| B: draft and save correctness | `spela-l7ij` | Fix root availability, draft replacement in `resource_view.go`/`content.go`, scope-change handling, field lifecycle in `detail.go`/`editor_host.go`/`options_modal.go`, and Ctrl+S. Preserve queue and failure semantics. | A | Root/game reset, field cancel, whole-draft discard, same-item reopen, guarded transitions, save failure, delayed saves, and restart persistence pass. Reproduce F2–F4 with termctrl. |
| C: complete workflow controls | `spela-7yoa` | Wire the action inventory through `sidebar.go`, content/DLL routing, `dlls_resource.go`, Monitor List/Detail, Settings, batch controls, and `dll_mutation.go`. Add basic-key choosers and safe confirmations. Remove obsolete command bindings and hard-coded raw-key dispatch as each path is replaced. | A, B | Every inventory row has a complete basic-key journey, including all cancellation paths, empty states, and a meaningful Monitor Enter. Fixture mutations verify both target scope and results. |
| D: truthful hints and bounded layout | `spela-809l` | Finish shared hint/help projections in `layout.go`, `help.go`, pane renderers, and dialogs. Remove parallel hint rules. Implement compact active-pane layout and pinned controls. | C | F1, F5–F7 pass in both pane focuses at 120×40 and 80×24. Essential controls remain visible with Show hints=false. Check layout thresholds and long content. |
| E: resize corruption diagnosis | `spela-cxhg` | Own the reproducer and live-frame diagnosis around `header.go`, viewport invalidation, and `tests/e2e/tui_journeys_test.go`. Use actual termctrl resize events, not only metric-width changes in a fixed viewport. Coordinate any shared layout edit with D. | None | Demonstrated root cause, scoped correction at its owning layer, and repeated rendered resize/navigation evidence. A suspected tool defect is not an application PASS. |
| F: acceptance and documentation | `spela-utx9` | Run the complete matrix below. Update `docs/tui-testing.md`, key examples in `AGENTS.md`, and existing tests that encode old shortcuts or incomplete Help. Correct the stale no-games test guidance. | B, C, D, E | Built binary, targeted and full required checks, rendered visible-key evidence, persistence/fixture checks, complete audit mapping, and cleanup. |

Start A and E independently if parallel work is useful. Keep B, C, and D ordered
because they share routing and pane models. Reconcile E's diagnosis with D before
editing shared layout code. Run targeted termctrl checks as each interactive
change lands; F is the final integrated pass. Commit the completed epic using
the repository's conventional commit format after acceptance. Release, install,
and remote publication are outside this plan.

## Verification and completion gate

Update meaningful behavior tests alongside each change, especially the
mode/focus/action matrix and draft/save state transitions. Do not retain tests
that assert Help must omit application controls or that an advertised no-op is
acceptable. Keep unavailable-key no-op tests: they must preserve navigation,
drafts, and files without reaching a hidden widget.

For each important state, record the pre-action `termctrl show`, the displayed
key/control and its declared scope, the key sent, and the observed result. Use
only currently displayed command keys. Free text is sent only after the UI
visibly enters a text field. Source inspection and tests can select the coverage
matrix; they cannot supply an undisplayed shortcut during the journey.

| Journey | Required assertions |
| --- | --- |
| Navigation and focus | Reach all four destinations and both panes; Enter opens root/game/catalog/Monitor/Settings selections; arrows name the active group/view/field correctly. An inactive pane never advertises a shortcut that needs its focus. In Search/Edit/overlays, global digit hints are absent and typing 0–4 never navigates. |
| Library browsing | Search and cancel/clear it, both filters, all four sorts, filter reset, Space selection, select-all from no selection, clear visible selection with hidden selected rows, and batch target/count confirmation. Root is never a batch game target. |
| Root and game profiles | Edit each supported field kind, cancel only one edit, apply, save by button and Ctrl+S, discard, reset one/all, reopen the same item, switch destination/view and return, change game/root with each guard choice, and restart. Check exact YAML changes and inheritance. |
| Settings | Test both focuses, bool/choice behavior where exposed, path entry, punctuation and digits as content, Enter-to-draft versus Ctrl+S-to-disk, cancel, discard, invalid/save-failure state, saving/busy feedback, and restart. |
| DLL workflows | Fixture install with family/version choice, selected-game update, filtered selection batch update, Catalog Deployment update-all from either pane, and restore. Every pre-dispatch stage has Back/Cancel; default confirmation Enter cancels. During delayed work, keys cannot dispatch twice or claim a false cancellation. Compare file hashes/backups and inspect final results and Close. |
| Read-only and availability | Catalog/Overview/Monitor have no edit/save hints. GPU/CPU/Alerts, empty/no-alerts, unavailable/missing prerequisites, no backups, no stale DLLs, and operation failures show usable explanations. Scrolling is available only when content overflows. |
| Help, menus, and recovery | Open Actions and Help from both panes of each destination; inspect all entries and scoped reference controls; close with Tab/Enter. Test every nested chooser, result, error, Cancel, Back, and Quit path, including dirty drafts. |
| Quit and pending work | Attempt normal Quit during delayed profile/Settings saves, queued saves from a previous profile, and DLL mutations. It stays unavailable until completion; failures and later edits remain visible and dirty drafts still require a decision. |
| Dimensions and hints | Repeat critical journeys at 120×40 and 80×24, with long names/paths and Show hints=false. Exercise 99/100-column and 29/30-row transitions. Inspect every visible value, focus mark, footer, confirmation policy, and dismissal control. |
| Actual resize and live updates | On the same running process, repeat 120×40 → 80×24 → 72×20 → 120×40 while metrics update. Include Browse, Edit, Help, and confirmation states; inspect every transition, then navigate again. Check complete metric units, no mixed rows, restored focus/drafts, and compare a fresh session. |
| Empty and failure states | Start with an empty isolated Steam library, recover through the visible rescan action, and inspect deterministic scan/read/save/operation failures without unsafe real-data actions. Invalid input and pre-dispatch cancellation leave files unchanged. For dispatched failures, verify the existing backup/partial-result contract and truthful outcome reporting. |

Use fresh deterministic fixtures with temporary game paths and isolated HOME/XDG
state. Never approve privileges or mutate real game files during review. Build
the compiled executable with `mage build`; if the local Mage cache is not
writable, use `MAGEFILE_CACHE=/home/jgabor/.cache/spela-mage mage build`. Use unique
named sessions, wait for visible content, inspect `termctrl status` and
`termctrl show`, and resize the running session explicitly. Do not substitute
logs or unit View strings for rendered-screen evidence.

Start with `go test ./internal/tui ./internal/nav` and the relevant existing E2E
selection after each change. For final acceptance, run `mage test`, `mage lint`,
and `go test -tags e2e -count=1 ./tests/e2e/...`, plus the manual
visible-key termctrl journeys. Record exact commands and versions. Inspect the
last rendered screen before stopping each session; remove isolated fixtures and
confirm that `termctrl list` contains no sessions from the review.

Epic completion requires F1–F7 and every supported action to pass the matrix,
plus an evidenced resolution of F8. If corruption is isolated to termctrl,
record the minimal reproducer, tested versions, and verified correction or
workaround before repeating acceptance. If it remains unresolved, keep that
gate open and report the limitation. Do not describe the TUI as fully verified
from targeted tests, fixture-only operation checks, or a fresh-session recovery.

## Completion report

The [acceptance report](tui-basic-key-acceptance.md) records the audit resolutions,
verification commands, rendered evidence, and validation limits. The navigation
contract and testing guide now describe the basic-key controls.
