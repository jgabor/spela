# TUI navigation and shell contract

This document is the behavioral source of truth for the terminal shell. It describes the
replacement navigation model, not the legacy primary/context/content implementation. Widgets,
pane models, cursors, and rendered tabs may implement this contract, but they are not additional
navigation state.

## Navigation state

The shell state has exactly these user-visible dimensions:

- **Destination**: `Library`, `DLL Catalog`, `Monitor`, or `Settings`.
- **Selection**: the destination-owned item or items that drive the Detail pane. A selection is
  tagged with its destination and cannot be reused by another destination.
- **Visible focus**: either `List` or `Detail`. There is one visible focus target. The destination
  bar is never focusable.
- **Input mode**: `Browse`, `Search`, or `Edit`.
- **Overlay**: absent, or one identified overlay with its own local state and return focus.

Destination-local presentation state (cursor, filters, sorting, scrolling, multi-selection, and
the chosen detail subsection) belongs to the corresponding List or Detail pane. It must not create
another shell focus zone. A destination change restores valid local state when it exists and
otherwise uses that destination's defaults. Stale selections are cleared or moved to the nearest
valid item before rendering.

`Tab` and reverse-`Tab` move focus only between visible panes that can currently accept input. If
only one pane can accept input, they are explicit no-ops. A destination change chooses the List
when it can accept input; otherwise it chooses Detail. Opening a game always selects that game and
opens its `Overview`, regardless of the detail subsection used for the previous game.

## Input routing and precedence

Input is routed in this order:

1. Terminal lifecycle messages, including resize, update the viewport regardless of mode.
2. An open Overlay receives applicable keys first.
3. In `Edit`, the active editor receives applicable keys first.
4. In `Search`, the search control receives applicable keys first.
5. Documented global actions are considered.
6. The visibly focused pane receives the key.

“Applicable” includes text entry, cursor movement, confirmation, and mode-specific cancellation.
Once a layer handles a key, routing stops. A layer may deliberately decline a non-applicable key,
allowing the next layer to consider it. Printable destination shortcuts (`1` through `4`) are
global only in `Browse`; in Search or Edit they are input, and an Overlay may consume them. This
rule also applies to every other printable shortcut: typing must never trigger a hidden action.

`Escape` closes or cancels only the highest active layer: Overlay first, then Edit (discarding the
uncommitted edit), then Search (clearing and leaving search). In Browse it performs only an action
explicitly registered for the current scope; it does not silently move focus or change selection.

Hidden panes and hidden controls receive no keys. In particular, profile subsection controls
cannot consume List movement, and a search result update changes the displayed search scope in the
same state transition. `Enter` may accept a result, but is never needed to repair stale scope text.

## Canonical keymap

Behavior, the status bar, and full help are projections of one ordered keymap. A binding contains:

| Field | Meaning |
| --- | --- |
| Mode | Modes in which the binding can be considered |
| Scope | Global, Overlay, List, Detail, Search, or Edit, plus any destination/action qualifier |
| Action | Stable semantic action identifier handled by routing |
| Keys | Normalized input keys used for matching |
| Labels | User-facing key and action labels used by status and help |
| Availability | Predicate and disabled reason derived from current state |

Key handling must resolve a semantic action from this model; views must not maintain parallel help
bindings. The compact status bar includes only currently relevant bindings, while full help may
also show unavailable bindings with their reason. Neither surface may advertise a key that routing
cannot execute in the shown mode and scope.

At minimum, the model registers `1`–`4` as Browse-only destination actions, pane traversal as a
shell action, help as a global action, and each destination's List and Detail actions in their
own scope. Exact labels and alternate keys live in the keymap rather than in this design document.

Any input that does not resolve to an available action is an explicit no-op: it leaves destination,
selection, focus, mode, overlay, search scope, cursor, and draft state unchanged and launches no
command. Unsupported keys must never fall through to a hidden widget.

## Destinations and ownership

### Library

Default entry: all games, List focus, no filter, the configured/default sort, no multi-selection,
and the first game selected when one exists. The Detail pane opens the selected game's `Overview`.

- **List owns** game cursor and scrolling, search/filter scope, sorting, multi-selection, and batch
  actions. Search results and the visible scope label are one state update.
- **Detail owns** the selected game's Overview, Profile, and DLL views and all actions on that game.
  Profile filters/subsections are Detail-local and cannot intercept List keys.

With no games, List renders the empty-library explanation and discovery/rescan action; Detail
renders “No game selected.” During a scan, List renders bounded loading progress and keeps any
still-valid results usable. A scan failure is a List error with retry guidance. Game-specific data
that cannot be read is an inline Detail error; a feature unavailable for the selected game is a
Detail unavailable state, not an empty list.

### DLL Catalog

Default entry: catalog Library section, List focus, default DLL-type/profile filters, no
multi-selection, and the first available catalog item selected.

- **List owns** the Library/Deployment collection being browsed, its cursor, scrolling, DLL and
  profile filters, multi-selection, and batch download/update/deployment actions.
- **Detail owns** metadata, cached versions, affected deployments, and actions for the selected DLL
  or deployment row.

An empty catalog or deployment result is explained in List and leaves Detail with “No DLL
selected.” Manifest/cache loading appears in List without stale rows masquerading as current data.
Manifest or cache failures are retryable List errors. Network-dependent operations render a clear
unavailable state when offline; cached information remains browsable where possible.

### Monitor

Default entry: GPU section, List focus, with the first available source selected.

- **List owns** the GPU, CPU, and Alerts source/device collection, cursor, scrolling, and any
  monitor filter.
- **Detail owns** live readings, history, alerts, and actions for the selected source.

Before the first sample, Detail shows loading rather than zero-valued measurements. No compatible
device is an unavailable state with the detected limitation. Collector failures are Detail errors
with retry/recovery guidance; stale last-known values must be labeled stale. “No alerts” is an
empty success state, not an error.

### Settings

Default entry: Display group, List focus, first setting selected, and Browse mode.

- **List owns** the Display, Startup, Paths, DLL policy, and Logging groups, the setting cursor,
  scrolling, and filtering.
- **Detail owns** the selected setting's description, effective/saved value, validation feedback,
  and edit/save/cancel actions.

An empty group renders an explanatory List state and no Detail selection. Loading is used while
configuration is read. Read or persistence failures are inline errors that preserve visible focus
and any recoverable draft. A setting unsupported on the current host is shown as unavailable and
cannot enter Edit. Entering Edit must not mutate the saved or live value; cancel restores the saved
display, and only a successful save replaces it.

## Layout contract

Terminal size is measured once by the shell and passed down as bounded rectangles. Every pane,
table, status line, help view, and overlay clamps both width and height to its rectangle; content
may truncate or scroll but may not expand the application beyond the viewport.

- **120×40 (normal):** destination bar, side-by-side List and Detail, and status hints are visible.
  Full labels and useful Detail context should be preferred.
- **80×24 (supported minimum):** the same navigation state remains operative. List and Detail may
  use narrower proportions, condensed labels, or a single-pane presentation, provided the active
  pane is obvious and `Tab`/reverse-`Tab`, status hints, and help remain reachable. Nothing is
  clipped outside the 80×24 viewport.
- **Below 80×24:** replace the workspace and overlays with a bounded resize prompt stating the
  80×24 minimum and current size. Resize events are still handled. No hidden workspace receives
  keys, no domain action runs, and returning to a supported size restores a valid destination,
  selection, and visible focus without panic.

Overlays never assume the normal fixture. At supported sizes they fit within the current viewport,
retain a visible dismissal path, and return to the focus captured when opened if that pane remains
available; otherwise focus is normalized using the destination rules above.
