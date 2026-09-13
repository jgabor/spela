# TUI testing

Use [Terminal Control](https://github.com/anomalyco/terminal-control) for every
change that can affect Spela's interactive terminal. Build a compiled executable,
run it in a named session, wait for visible content, and read `termctrl show`.
Unit tests and raw terminal logs do not establish visual correctness.

For profile choices, focus Detail and use its displayed Space control. Verify
HDR and VKD3D heap through `(default)` to `true` to `false` to `(default)`, then
save an explicit value with Ctrl+S or Actions > Save and inspect the isolated
profile. Check both All games and a game with a nonempty inherited default.
Space must stay with Library selection while List has focus. Text and free-number
fields must show Enter in Detail and accept Space as text only inside the editor.

For game DLLs, use the visible Install DLL, Update DLLs, and Restore originals
rows directly with Up/Down and Enter. Verify disabled reasons and that Enter is
absent for an unavailable selection. Review both pane focus states, both terminal
sizes, and `show_hints: false`. Confirm fixture file contents after Update and
Restore originals; merely opening a dialog does not verify the mutation.

## Terminal Control version

Use Terminal Control 1.2.1 or a newer version that passes the compatibility test:

```bash
termctrl --version
go test -tags=e2e ./tests/e2e -run '^TestTerminalControlCursorBackwardTabCompatibility$' -count=1
```

Version 0.3.1 ignores the cursor backward tab sequence (`CSI Z`) used by Bubble
Tea. It can show corrupt metrics even when the application emits valid terminal
output. The compatibility test checks this behavior directly.

The 2026-09-13 acceptance review used the unmodified upstream 1.2.1 commit
`c1d4f95e4f1b7638f6229e9bfc9599a955f95ce2`, built under the local cache. To use
that existing review build on the review machine:

```bash
export PATH="/home/jgabor/.cache/spela-termctrl-build/release:$PATH"
termctrl --version
```

This path is a local verification tool, not an application dependency or a
portable installation instruction. The installed user tool was left unchanged.
Spela also includes the separate upstream
[Ultraviolet resize correction](https://github.com/charmbracelet/ultraviolet/commit/d38ea0f8aa5cefbde40e02dc7da8c834504bd76e),
which fixes stale comparison cells after terminal growth. Both corrections are
needed to reproduce and verify live resize reliably.

## Start

Build the executable, then run a named session:

```bash
mage build
termctrl start spela-tui-review --cols 120 --rows 40 -- ./spela tui
termctrl wait spela-tui-review "Library" --timeout 20000
termctrl status spela-tui-review
termctrl show spela-tui-review
```

Use a unique name when reviews run concurrently, and use that name in every
later command. Check the command, working directory, size, and lifecycle with
`termctrl status`.

The example uses normal XDG state. Spela may update the game database when it is
empty or `rescan_on_startup` is enabled. Use isolated state whenever startup or
the reviewed journey can write files.

## Isolate writable state

Profile editing, Settings changes, persistence checks, errors, and empty states
require an isolated XDG environment. Exploratory review can copy configuration
and database files into a temporary root:

```bash
review_root="$(mktemp -d /tmp/spela-termctrl-XXXXXX)"
mkdir -p "$review_root/config" "$review_root/data" "$review_root/cache" "$review_root/runtime"

config_source="${XDG_CONFIG_HOME:-$HOME/.config}/spela"
data_source="${XDG_DATA_HOME:-$HOME/.local/share}/spela"
cache_source="${XDG_CACHE_HOME:-$HOME/.cache}/spela"

if [ -d "$config_source" ]; then cp -a "$config_source" "$review_root/config/"; fi
if [ -d "$data_source" ]; then cp -a "$data_source" "$review_root/data/"; fi
if [ -d "$cache_source" ]; then cp -a "$cache_source" "$review_root/cache/"; fi

termctrl start spela-tui-isolated --cols 120 --rows 40 -- \
  env XDG_CONFIG_HOME="$review_root/config" \
      XDG_DATA_HOME="$review_root/data" \
      XDG_CACHE_HOME="$review_root/cache" \
      XDG_RUNTIME_DIR="$review_root/runtime" \
      ./spela tui
termctrl wait spela-tui-isolated "Library" --timeout 20000
termctrl show spela-tui-isolated
```

Copied game paths still point to real installations. Profile and Settings writes
stay in `review_root`, but DLL and launch operations can touch real game files.
Use deterministic fixtures with temporary game paths for those operations. Never
approve privileged or destructive actions against real game files during review.

The E2E suite creates fresh fixture games, profiles, a manifest, isolated HOME
and XDG paths, and its own Terminal Control runtime directory. It builds the
executable and cleans up its unique sessions and temporary files:

```bash
go test -tags=e2e ./tests/e2e -count=1 -v
```

An empty game database must remain interactive and show `No games found` with
Actions and Settings recovery guidance. For manual review, use an isolated empty
Steam library. Use the displayed Actions menu to choose `Rescan games`; inspect
the outcome and recovery controls. Wait for the visible scan completion before
starting another scan. While a scan, save, or DLL mutation is pending, normal
Quit must remain unavailable with a reason. Do not infer success from the process
running.

## Use displayed controls

Navigation and commands need only numbers, arrows, Tab, Enter, Space, and Ctrl+S
for saving. Send a command only when its key is visible in the current frame.
Text entry is distinct: a focused input accepts ordinary text, including digits,
spaces, non-ASCII characters, and punctuation.

Open Actions from Browse, inspect the selected row, then close it through its
visible control:

```bash
termctrl send spela-tui-review 'text:0'
termctrl wait spela-tui-review "Actions ·" --timeout 20000
termctrl show spela-tui-review
termctrl send spela-tui-review tab
termctrl wait spela-tui-review "▸ Close" --timeout 20000
termctrl show spela-tui-review
termctrl send spela-tui-review enter
termctrl show spela-tui-review
```

To run an action, use the displayed Up or Down key and inspect each selected row
until the requested label is selected, then press Enter. Do not derive a key or
row count from the source code. Unavailable actions explain why and omit the
Enter hint; use Tab to reach Close. Help and Quit are Actions rows.

For search, focus the Library List and choose `Search games`. After the Input
control appears, send text separately from the key that applies it:

```bash
termctrl send spela-tui-isolated 'text:Cyber'
termctrl show spela-tui-isolated
termctrl send spela-tui-isolated enter
termctrl wait spela-tui-isolated "Cyberpunk 2077" --timeout 20000
termctrl show spela-tui-isolated
```

Search also has selectable Apply, Clear search, and Cancel controls. Field
editors have Input, Apply, Cancel, and Save controls. Tab moves between local
controls; Enter activates the focused one. Apply changes the draft, while Ctrl+S
validates the current input and saves. Check the persisted YAML and restart
through the displayed Quit action before opening another process on that state.

While an editor, search input, Help, Actions, chooser, or confirmation owns
input, destination numbers must not be shown as active commands. Typing `01234`
in a text field must leave the destination unchanged. In Browse, check the
active pane marker before using arrows or Enter. Read-only Detail views must not
advertise editing or saving. Hide optional hints in Settings and verify that the
essential controls and Help remain reachable.

Confirmations start on Cancel. Inspect target paths and backup policy, use Tab
to reach scrollable details when needed, then return to the buttons. Confirm
requires an explicit selection. Results have a selectable Close control.

## Resize and inspect

Use actual resize operations on one running session, including shrink, growth,
and recovery from below the minimum:

```bash
termctrl resize spela-tui-review --cols 80 --rows 24
termctrl wait spela-tui-review "Library" --timeout 20000
termctrl show spela-tui-review
termctrl resize spela-tui-review --cols 72 --rows 20
termctrl wait spela-tui-review "Resize terminal" --timeout 20000
termctrl show spela-tui-review
termctrl resize spela-tui-review --cols 120 --rows 40
termctrl wait spela-tui-review "Library" --timeout 20000
termctrl show spela-tui-review
```

Wait for a new visible metrics sample after each supported resize and inspect
that frame. A fresh process at each size does not test resize recovery.
At 80x24 the compact header and active pane must fit; Tab reveals the other
pane. At 120x40 Standard shows both panes. Below 80x24 a bounded resize prompt
replaces the workspace. Confirm that recovery restores one destination bar,
closed borders, complete metrics units, and current alerts without stale cells.

The resize regression uses deterministic changing metrics and one session through
120x40, 80x24, 100x32, and 72x20. To retain diagnostic artifacts:

```bash
resize_evidence="$(mktemp -d /tmp/spela-resize-evidence-XXXXXX)"
SPELA_E2E_RESIZE_ARTIFACTS="$resize_evidence" \
  go test -tags=e2e ./tests/e2e \
  -run '^TestTUIRepeatedLiveResizePreservesFrames$' -count=1 -v
```

Read the rendered `last-screen.txt` and the visible phase captures in test output.
`terminal.ansi` is diagnostic evidence, not a substitute for rendered review.

## Record results and clean up

Record fixture identity, viewport sizes, displayed key journeys, active frames,
persisted changes, and the important `termctrl show` results. Cover startup,
loading, empty, error, cancellation, and exit states as relevant. Check search,
filters, sort, selection, profile drafts, Settings, read-only destinations, and
DLL operations against the changed scope. State anything not verified.

Inspect the final screen before stopping a failed or exited session. Stop each
named session and verify no review sessions remain:

```bash
termctrl show spela-tui-review
termctrl stop spela-tui-review
termctrl stop spela-tui-isolated
termctrl list
```

Remove only the temporary roots created for this review after checking their
paths. A stop error for a session that was never started does not replace the
final session-list check. Preserve requested evidence before removing fixtures.
