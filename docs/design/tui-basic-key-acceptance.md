# Basic-key TUI acceptance

Implementation date: 2026-09-13. Historical Beans epic: `spela-fi78`.
This closed ID is retained as acceptance provenance, not a live tracker link.
Current work uses Agentera; the [migration record](../migrations/beans-to-agentera.md)
documents recovery of the historical source. The evidence below is unchanged.

The implementation follows the [approved plan](tui-basic-key-navigation-plan.md)
and [navigation contract](tui-navigation.md). Commands use arrows, Tab, Enter,
Space, and numbers, with optional Ctrl+S. Ordinary text entry remains unrestricted.

## Audit resolution

| Finding | Implemented behavior and evidence |
| --- | --- |
| F1: focus-blind hints | One contextual action registry drives dispatch, Actions, Help, and active footer labels. Library view arrows require Detail; Settings group and value controls have different scopes. Catalog mutations reveal their dialog and restore the invoking focus. |
| F2: blocked root reset/discard | All games uses system defaults; games use inheritance. Both scopes support reset and discard, with explicit confirmation for reset-all/discard. |
| F3: draft replacement | Same-item reopen and destination detours preserve drafts. Save/Discard/Cancel resolves replacement before publishing selection. Search cancellation deep-copies its original text. Save snapshots preserve later edits. |
| F4: inconsistent save | Profile and Settings editors expose Input/Apply/Cancel/Save. Enter applies to draft; Save or Ctrl+S validates and persists. Save failures retain the draft and editor state. |
| F5: Monitor Enter | Enter opens the selected source in Detail. The focused List row has a visible marker; Tab returns to List. |
| F6: read-only edit hints | Overview, Monitor, and Catalog inspection have no profile Edit/Save hints. Scrolling is offered when needed. DLL Actions states its target and availability. |
| F7: hidden/clipped controls | The 80x24 layout displays the active pane, with pinned input/dialog/result controls. Actions exposes all workflows and full scoped Help. Essential controls remain visible with Show hints=false. Long batch failures remain scrollable. |
| F8: render corruption | A minimal cursor backward-tab sequence isolated a Terminal Control 0.3.1 defect. Unmodified Terminal Control 1.2.1 renders it correctly. Spela also pins the upstream Ultraviolet resize-buffer fix. Eleven actual resize phases pass with live metrics. |

Additional integration fixes refresh shared game records after DLL mutations,
reject obsolete installer and update-check results, serialize scans, capture scan
configuration, and reject older scan completions. Normal Quit stays unavailable
during pending work. Deferred scan results wait for active editors/overlays and
any dirty-profile decision.

## Verification

The final build and broad checks passed with isolated XDG state:

```bash
env MAGEFILE_CACHE=/home/jgabor/.cache/spela-mage mage lint
env MAGEFILE_CACHE=/home/jgabor/.cache/spela-mage mage test
env MAGEFILE_CACHE=/home/jgabor/.cache/spela-mage mage build

env PATH="/home/jgabor/.cache/spela-termctrl-build/release:$PATH" \
  SPELA_E2E_EDITOR_ARTIFACTS=/home/jgabor/.cache/spela-editor-kinds \
  SPELA_E2E_RESIZE_ARTIFACTS=/home/jgabor/.cache/spela-basic-key-final-resize \
  go test -tags=e2e ./tests/e2e -count=1 -v

env PATH="/home/jgabor/.cache/spela-termctrl-build/release:$PATH" \
  go test -tags=e2e ./tests/e2e \
  -run '^TestTUIFiltersSortAndVisibleSelection$' -count=1 -v
```

`mage test` includes all default Go tests and the GUI backend tests. Lint reports
zero issues. All 13 compiled termctrl E2E tests passed in 36.289 seconds. A
subsequent targeted check of the zero-target batch also passed at both sizes.
Together they cover search, filters, sorting, selection, root drafts, Settings
persistence and restart,
unsaved Quit decisions, read-only destinations, DLL pre-dispatch cancellation,
empty-library scan recovery, startup errors, and live resize recovery. Choice,
integer, and text editors additionally validate exact YAML, inheritance, local
Apply/Cancel, invalid integer recovery, and paths containing `01234 /åäö/[cache]`.
Starting an editor clears earlier transient save feedback.

Focused regressions additionally cover editor validation, exact draft ownership,
serialized saves and scans, obsolete completions, pending-work Quit, long result
scrolling, modal focus restoration, and DLL file/database persistence. Selected
batches with no concrete targets show an unavailable reason and do not advertise
Enter for Update.

Manual reviews read `termctrl show` before choosing displayed controls. Temporary
games, cached DLL payloads, backup metadata, and isolated HOME/XDG directories
allowed file hash and persisted YAML comparisons. A filtered two-game selection
updated only the visible game; the hidden selected game retained its DLL and
profile hashes and had no new backup. All four sort choices were exercised.
Root/profile, Help, Monitor, Actions, and resize checks were also repeated with verbose hints disabled.

## Evidence and limits

Durable regression coverage is under `internal/tui` and `tests/e2e`. Local rendered
screens, command receipts, and logs are retained at:

- `/home/jgabor/.cache/spela-basic-key-final-{lint,test,build}.log`
- `/home/jgabor/.cache/spela-basic-key-final-e2e.log`
- `/home/jgabor/.cache/spela-basic-key-zero-target-e2e.log`
- `/home/jgabor/.cache/spela-editor-kinds.log`
- `/home/jgabor/.cache/spela-editor-kinds/`
- `/home/jgabor/.cache/spela-basic-key-final-resize/`
- `/home/jgabor/.cache/spela-tui-audit/spela-basic-root-y6oxbbhz/`
- `/home/jgabor/.cache/spela-tui-audit/spela-dll-basic-u4ippn9d/`
- `/home/jgabor/.cache/spela-tui-audit/spela-dll-final-wpkqvfn7/`
- `/home/jgabor/.cache/spela-tui-audit/spela-dll-selected-wpkqvfn7/`
- `/home/jgabor/.cache/spela-resize-report.md`

The verified Terminal Control version is 1.2.1, built from upstream commit
`c1d4f95e4f1b7638f6229e9bfc9599a955f95ce2`. The user's installed 0.3.1 executable
was not replaced. The scoped PATH above selects the corrected review tool.

DLL fixtures use synthetic payloads, so the observed parsed version can be
`unknown`; correctness was checked against their exact hashes and backup files.
These checks do not qualify real-game launch, privileged GPU/CPU changes, network
download availability, vendor DLL version parsing, or hardware performance.

All named review sessions were stopped. Their temporary game/HOME/XDG fixture
directories were removed after retaining the listed evidence.
