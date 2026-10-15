# TUI navigation and shell contract

This is the behavioral contract for the terminal shell. The completed
[basic-key implementation plan](tui-basic-key-navigation-plan.md) records the
2026-09-13 audit, decisions, and acceptance criteria. The TUI edits profiles and
configuration, manages DLLs, and displays metrics. Game launching and privileged
GPU/CPU tuning remain outside its interactive workflows.

## Navigation state and controls

The shell owns a destination, its selection, visible List or Detail focus, and
one active input layer. The destination bar is not focusable. Each destination
retains its own cursor, group, and filters when revisited.

| Control | Browse behavior |
| --- | --- |
| 0 | Open Actions for the current destination, pane, and selection |
| 1, 2, 3, 4 | Library, DLL Catalog, Monitor, Settings; return focus to List |
| Tab | Move between eligible List and Detail panes |
| Up, Down | Move the focused list or profile field; scroll read-only detail when needed |
| Left, Right | Change groups in Catalog/Settings List; jump between the sections of a profile page and its game's DLLs section |
| Enter | Open List selection in Detail; edit a profile value (a picker for choices, an editor for numbers and text); run a selected DLL action |
| Space | Toggle a Library game's selection; in Detail, switch a profile row between following and its own value |

Every workflow is reachable with arrows, Tab, Enter, Space, and numbers. Escape
closes an open editor or picker, as the Cancel control does. Commands do not
require Shift+Tab, letters, punctuation, or function keys. There is no Save key:
every change is written as it is made. Ctrl+C remains an emergency terminal
interruption, outside the normal Quit workflow.

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
executable keys, and names Space for the row under the cursor (Set for this game,
Follow All games, Set here, or Use system default). Actions may show a relevant
unavailable operation with its reason; Enter is advertised only when the
selected row can run.

The active pane has a focus label and border. Editors and dialogs use a visible
selection marker or focused control styling. Essential navigation and dismissal
controls are always visible.

## Actions and Help

Actions uses Up/Down to choose a row, Enter to run an available action, and Tab
to reach Close. Contextual rows expose search, filters, explicit sort choices,
visible selection operations, the Follow and Use system default rows for a field
or the whole page, and applicable DLL operations. Every Browse context also exposes the three layout choices, Help,
and guarded Quit.

Help distinguishes reference information from active controls. It lists the
captured Browse context and describes other scoped controls as reference only.
Up/Down scroll its body when it overflows. Tab reaches Close; Enter closes Help
only while Close has focus. Dismissing either overlay retains the invoking pane.

## Library and profiles

Library starts on the first game, or an empty state with Actions rescan and
Settings path recovery. The pinned All games item edits the default profile.
Enter opens a game on its Profile with Detail focus. The page you were on and the
field under the cursor stay when the game changes, so the same setting can be
changed on the next game without finding it again. A game left on its DLLs
section reopens on DLLs. All games keeps its own cursor.

A profile page has three header lines: the game, its App ID, its DLLs, and the
save state (`Saved at once`, `Saving…`, or `Not saved: <reason>`); the settings
the game sets itself, by name and value; and the section strip (Proton, DLSS,
GPU, CPU, Overlay, and for a game DLLs) with the current section marked. Left
and Right jump to the first field of the previous or next section on a game page
and on All games. Right from the last section opens the game's DLLs section;
Left returns to the first Overlay field. Nothing wraps.

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

Moving between games, filtering, searching, and rescanning never ask a question:
every change has already been written. A write still running when the game
changes finishes in the background, and its message names the game when it is no
longer on screen.

Use system default (a field on All games) clears the value. Follow All games (a
field on a game) clears the game's own value. Their "for all settings" forms need
one confirmation that says how many settings they clear. Semantically empty
override maps are not different from the saved file, but an explicit override pin
remains meaningful even when its value is false.

## Field editing and persistence

Every change is saved when it is made. The message line says what was saved
(`Saved: HDR On`, `Saved: Clock offset follows All games`), the header shows
`Saving…` while a write runs, and a failed write returns the row to the stored
value and shows `Not saved: <reason>` in the header and on the message line.
There is no draft, no Save command, and no Discard command.

Space in Profile Detail switches the focused row between following and its own
value. Following to own, a boolean takes the opposite of the value it follows;
any other field puts back the value it had before, or opens its editor or picker
when it has none. Own to following remembers the value, so the same key undoes
either move. On All games the same key switches between the system default and a
value set there. The row always shows the effective value and its source
(`from All games`, `this game · All games: Off`, `set here`, or `system
default`). Explicit Off, zero, empty text, and optional no-value choices remain
distinct from inheritance.

Enter edits the focused row. Booleans, optional booleans, and fields with a fixed
set of values open a picker: the choices as a list, the first being Follow All
games (or Use system default) with the value it would give. Typing letters or
digits jumps to the first choice that starts with them; Up and Down move; Enter
applies and saves; Esc cancels. The picker is also how a game pins the value it
already follows. Numbers and text open an editor with Input, Apply, and Cancel
controls; Tab cycles through them and Left/Right choose buttons while a button
owns focus. Typing replaces a prefilled number. Text and number inputs accept
their normal content and cursor controls, and Space is literal text only while an
input owns focus.

Enter on Input or Apply validates the field, applies it, and saves it. Cancel and
Esc drop only the current edit. A rejected value stays in the editor with its
error, for correction.

Writes for one profile are queued and merged in order, so an earlier result never
marks a later change saved. Moving on while a write runs is safe. Settings
follow the same rules: Space and the path editor save at once, a change made
during a write is written by the next one, and a failed write returns the option
to its stored value.

Normal Quit waits for writes in flight and then quits (`Saving, then quitting`);
it is refused while a scan or DLL operation runs. Ctrl+C quits at once.

## DLL Catalog and mutations

Catalog List owns its Library/Deployment group and row. Detail displays cached
versions or deployments. Read-only views expose scrolling only when needed and
do not advertise field editing or profile saving. Update all stale deployments
is an explicit Catalog operation, available from either pane when targets exist.
If invoked from List, its dialog reveals Detail and returns to List on dismissal.

The Library game's DLLs section starts with where the game lives (install
directory, Proton prefix, and the `spela %command%` Steam launch option, each on
one line, shown when they fit) and shows a direct action list below its versions:
Install DLL, Update DLLs, and Restore originals. With Detail focused, Up/Down
selects an action and Enter starts it. All three rows remain visible, including
target counts and reasons for unavailable actions, at the minimum terminal size.
Unavailable selections have no Enter hint.
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
and saves it; Enter opens a text or free-number editor whose Apply saves.

A scan captures its configuration snapshot and has one request identity. A
pending scan disables another scan and normal Quit. Obsolete completions cannot
replace newer state. Results that arrive during an editor or overlay wait until
Browse can apply them.

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
