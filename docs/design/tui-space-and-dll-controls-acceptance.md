# Space cycling and direct DLL controls acceptance

Verified on 2026-09-13 as a follow-up to the
[basic-key acceptance review](tui-basic-key-acceptance.md). The current behavior
is defined in the [navigation contract](tui-navigation.md).

## Accepted behavior

| Area | Observed result |
| --- | --- |
| HDR and VKD3D heap | Space cycles `(default)` → `true` → `false` → `(default)` without opening an editor. A full rotation returns to a clean draft. |
| Inheritance | A game displays `(default)` beside its effective inherited value. Explicit false, zero, empty text, and optional no-value pins survive saving. Returning to `(default)` removes the pin. |
| Other finite fields | String choices and Multi-frame cycle through their supported values with Space. Optional SMT values pass the profile API's bool/nil validation. |
| Settings | Space changes the selected boolean draft; the second Space returns to the saved value. Ctrl+S or Actions > Save persists it. |
| Free input | Enter opens text and free-number editors. Space remains literal text only in the input. Tab, Apply, Cancel, Save, and validation retain their existing behavior. |
| Focus and hints | List Space selects games. Detail Space changes choices. Cyclic fields hide Enter; free inputs hide Browse Space. Read-only views do not advertise field controls. |
| Direct DLL controls | Game DLLs displays Install DLL, Update DLLs, and Restore originals. Up/Down selects a row; Enter runs an available action. Unavailable reasons stay visible and suppress Enter. |
| DLL outcomes | Cancel leaves files unchanged. Update changes the selected stale file; Restore originals returns recorded backups to their original paths. Install into an empty fixture creates the expected DLL. Other game files remain unchanged. |
| Layout | Controls remain visible at 80×24 and 120×40 with hints disabled. The compact DLL view retains two installed versions alongside its actions. Results retain Close when resized. |

Restore originals means restoring Spela's recorded backups. Newly installed
DLLs with no original backup stay installed.

## Verification

The following commands passed:

- `go test ./internal/tui` and targeted cycle/focus/DLL regressions.
- `mage build`, including the embedded frontend.
- `mage lint`, with zero issues.
- `mage test`, including the default Go suite and GUI backend checks.
- `go test -tags=e2e ./tests/e2e -count=1 -v`, all 15 tests in 40.600 seconds.
- `git diff --check`.

The complete test run used a fresh writable `XDG_RUNTIME_DIR`. The sandbox's
normal `/run/user/1000` cannot hold test lock files. Terminal Control used the
scoped upstream 1.2.1 build described in [TUI testing](../tui-testing.md).

The compiled interactive review used only controls shown in the active view.
Manual `termctrl show` inspection covered game HDR inheritance, VKD3D heap
persistence, direct DLL controls, disabled selections, inactive Detail hints,
the installer and cancellation result, both viewport sizes, and hints disabled.
The E2E suite also covers root profiles, Settings, draft guards, filtering,
selection, other destinations, empty-state recovery, and repeated live resize.

Local evidence from this run:

- `/home/jgabor/.cache/spela-space-{build,lint,test,final-e2e}.log`
- `/home/jgabor/.cache/spela-space-e2e/`, rendered profile and Settings phases.
- `/home/jgabor/.cache/spela-space-manual/`, rendered manual phases, command log,
  saved profile, executable hash, and cleanup record.
- `/home/jgabor/.cache/spela-tui-audit/dll-direct-e2e-20260913/`, rendered direct
  DLL flows and fixture file hashes.

Profile and DLL writes used isolated state and temporary fixture game paths.
Actual game DLLs, privileged hardware changes, and live downloads were outside
this review. Named sessions and their temporary fixtures were cleaned up.
