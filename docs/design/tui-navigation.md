# TUI navigation and shell contract

This is the behavioral contract for the terminal shell. The completed
[basic-key implementation plan](tui-basic-key-navigation-plan.md) records the
2026-09-13 audit, decisions, and acceptance criteria. The TUI edits profiles and
configuration, manages DLLs, and displays metrics. Game launching and privileged
GPU/CPU tuning remain outside its interactive workflows.

## Navigation state and controls

The shell owns a destination, its selection, visible List or Detail focus, and
one active input layer. The destination bar is not focusable. Each destination
retains its own cursor, group, filters, and applicable draft when revisited.

| Control | Browse behavior |
| --- | --- |
| 0 | Open Actions for the current destination, pane, and selection |
| 1, 2, 3, 4 | Library, DLL Catalog, Monitor, Settings; return focus to List |
| Tab | Move between eligible List and Detail panes |
| Up, Down | Move the focused list or profile field; scroll read-only detail when needed |
| Left, Right | Change groups in Catalog/Settings List; change a selected game's Detail view |
| Enter | Open List selection in Detail; edit text or a free number; run a selected DLL action |
| Space | Toggle a Library game's selection; cycle a focused Detail choice |
| Ctrl+S | Save the owning profile or Settings draft when that save context is available |

Every workflow is reachable with arrows, Tab, Enter, Space, and numbers. Ctrl+S
is an optional alternative to selectable Save controls. Commands do not require
Escape, Shift+Tab, letters, punctuation, or function keys. Ctrl+C remains an
emergency terminal interruption, outside the normal guarded Quit workflow.

Text inputs accept ordinary content, including digits, punctuation, spaces,
non-English text, and negative numbers. Cursor and deletion keys remain local
to text inputs. The restriction above applies to commands, not entered data.

## Input ownership and hints

Resize and asynchronous completion messages always update their owning state.
Keys go to exactly one active layer: decision dialog, Actions, Help, batch or DLL
flow, Search, field editor, then Browse. A rejected key is a no-op in that layer;
it cannot reach a hidden pane or invoke a Browse command.

The destination numbers and global Actions shortcut appear only in Browse.
During Search or Edit, digits belong to the focused text input. When a button
owns focus, typing cannot alter the input behind it. Overlays hide underlying
shortcuts and expose their own focus, scroll, confirm, and dismissal controls.

`CanonicalKeymap` resolves semantic actions and their availability. Actions,
Help, and status labels use the same context, including destination, pane,
selection, editor kind, target count, and pending work. The footer shows only
executable keys. Actions may show a relevant unavailable operation with its
reason; Enter is advertised only when the selected row can run.

The active pane has a focus label and border. Editors and dialogs use a visible
selection marker or focused control styling. Essential navigation and dismissal
controls remain visible when verbose hints are disabled.

## Actions and Help

Actions uses Up/Down to choose a row, Enter to run an available action, and Tab
to reach Close. Contextual rows expose search, filters, explicit sort choices,
visible selection operations, profile reset/discard/save, and applicable DLL
operations. Every Browse context also exposes the three layout choices, Help,
and guarded Quit.

Help distinguishes reference information from active controls. It lists the
captured Browse context and describes other scoped controls as reference only.
Up/Down scroll its body when it overflows. Tab reaches Close; Enter closes Help
only while Close has focus. Dismissing either overlay retains the invoking pane.

## Library and drafts

Library starts on the first game, or an empty state with Actions rescan and
Settings path recovery. The pinned All games item edits the default profile.
Enter opens a game in Overview with Detail focus. Left/Right change between
Overview, Profile, and DLLs only while game Detail has focus.

The List owns search, DLL/profile filters, four explicit sort choices, and game
selection. Search has Input, Apply, Clear, and Cancel controls. Results and Detail
selection update together. Apply retains the query; Cancel restores the query
and selection captured when Search opened. Clear removes the query. Actions
Clear filters also resets filters and sort to A-Z.

Space toggles a game; All games cannot be selected for a DLL batch. Selection
counts distinguish total selected from selected within the current filter.
Select all and Clear selected affect filtered games. Enter with visible selected
games opens their batch actions. Hidden selections never silently join that
batch. Rescanning removes selections for games that no longer exist.

Root and per-game profile drafts are separate. Reopening the same item or making
a destination detour preserves its draft. A transition that would replace a
dirty owner, including cursor movement, filtering, search, or rescan results,
requires Save and continue, Discard and continue, or Cancel. Cancel is selected
first. List selection and Detail scope change only after the decision succeeds.

Reset field on All games means the system default. Reset field on a game means
inheritance. Reset all and Discard draft require explicit confirmation. Discard
restores the saved baseline. Semantically empty override maps are not dirty, but
an explicit override pin remains meaningful even when its value is false.

## Field editing and persistence

In Profile Detail, Space cycles a focused boolean or finite choice directly in
the draft. Ordinary booleans use `(default)`, `true`, `false`, then `(default)`.
For a game, `(default)` means inheritance, with the effective value shown beside
it. For All games it means the system default. Explicit false, zero, empty text,
and optional no-value choices remain distinct from inheritance. Finite numeric
choices, such as Multi-frame, also cycle with Space. Returning to the original
value and override state clears the draft's unsaved status.

Settings boolean and finite choices cycle between their configured values with
Space. They have no inheritance state. Space never saves. Save in Actions or
Ctrl+S persists the draft. Choice fields do not advertise or open an Enter editor.

Free text and numeric fields use Enter to open an editor with Input, Apply,
Cancel, and Save controls. Tab cycles through them; Left/Right choose buttons
while a button owns focus. Text and number inputs accept their normal content
and cursor controls. Space is literal text only while an input owns focus.

Enter on Input or Apply validates the field and applies it to the draft. Cancel
drops only the current raw edit and retains earlier draft changes. Save and
Ctrl+S validate the field, apply it, and persist the owning document. Validation
and save failures remain visible and preserve input for correction or retry.

Browse Save on Library belongs to the focused Profile Detail, including All
games. Settings Save belongs to its configuration draft from either pane. A
successful save updates only the captured snapshot's baseline; edits made after
that snapshot remain dirty. Existing save serialization is preserved.

Normal Quit waits for pending writes/scans. It resolves both profile and Settings
drafts before exiting. Cancel returns to the invoking state. Failure to save
keeps the dialog and drafts available for retry.

## DLL Catalog and mutations

Catalog List owns its Library/Deployment group and row. Detail displays cached
versions or deployments. Read-only views expose scrolling only when needed and
do not advertise field editing or profile saving. Update all stale deployments
is an explicit Catalog operation, available from either pane when targets exist.
If invoked from List, its dialog reveals Detail and returns to List on dismissal.

The Library game's DLLs view shows a direct action list below its versions:
Install DLL, Update DLLs, and Restore originals. With Detail focused, Up/Down
selects an action and Enter starts it. All three rows remain visible, including
target counts and reasons for unavailable actions, at the minimum terminal size
and with verbose hints disabled. Unavailable selections have no Enter hint.
With List focused, the DLL view shows Tab to focus its controls. The same
workflows remain available in Actions.

Install uses selectable type and version lists plus Back/Cancel.
All mutations show concrete files, versions, and backup/restore policy before
execution. Cancel is the default. Tab accesses long details; arrows scroll them
or select confirmation buttons according to focus. Busy operations expose a
wait state until completion. Results have a scrollable body and pinned Close.

Completion messages update all views' game references. Request identities reject
obsolete installer and update-check responses. Restore targets come from the
actual backup metadata, including files absent from the current detection list.
Restore originals restores Spela's recorded backups. It does not delete newly
installed DLLs that have no original backup.
Existing deny-list, backup, mutation, and persistence services remain authoritative.

## Monitor, Settings, and scans

Monitor List owns GPU, CPU, and Alerts selection. Enter opens the source in
Detail. Detail provides inspection and scrolling, with Tab returning to List.
It exposes no edit/save action.

Settings List owns the configured option groups and fields. Left/Right changes
group, Up/Down selects a setting, and Enter opens Detail. Space cycles a choice
there; Enter opens a text or free-number editor. Destination changes preserve
its existing configuration draft.

A scan captures its configuration snapshot and has one request identity. A
pending scan disables another scan and normal Quit. Obsolete completions cannot
replace newer state. Results that arrive during an editor or overlay wait until
Browse can apply them, including any required dirty-profile decision.

## Layout and terminal verification

The shell allocates bounded rectangles before rendering content. It reserves
orientation, active controls, and overlay dismissal before allocating body space.

- At 120x40, Standard shows the header, destination bar, List and Detail side by
  side, message line, and two footer rows.
- At 80x24, or below the split layout's width/height threshold, the active pane
  fills the content area. Tab changes the displayed pane. Selected field cards,
  editor controls, confirmation policy, and result dismissal remain reachable.
- Compact reduces the header; Focused removes it and uses one active pane.
- Below 80x24, the bounded resize prompt replaces all interactive content.
  Workspace keys are suspended; resizing restores the prior state. Pending
  completions still update their owners. Emergency Ctrl+C remains available.

Live terminal review uses compiled executables, isolated XDG fixtures, and
`termctrl show`. Terminal Control 1.2.1 is the verified baseline; 0.3.1 mishandles
cursor backward tabs and cannot validate these rendered frames. The pinned
Ultraviolet renderer includes the upstream fix for expanded resize columns.
See [the testing guide](../tui-testing.md) for commands, evidence, and cleanup.
