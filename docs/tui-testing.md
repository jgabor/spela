# TUI testing

Use [Terminal Control](https://github.com/anomalyco/terminal-control) for every
change that can affect Spela's interactive terminal. Build a compiled executable,
run it in a named session, wait for visible content, and read `termctrl show`.
Unit tests and raw terminal logs do not establish visual correctness.

Profile changes are saved as they are made, so there is no Save step to test.
Focus Detail and use its displayed Space control: on a boolean it switches
between following and the opposite value, and the same key undoes it. After each
press, wait for `Saved:` on the message line and inspect the isolated profile
file. Check both All games and a game with a nonempty inherited default. Space
must stay with Library selection while List has focus. Enter opens a picker for
booleans and choices (type a letter to jump, Enter applies, Esc cancels) and an
editor for numbers and text, where Space is text only inside the editor. Left and
Right jump between the sections of the page and reach the DLLs section; a game
opens on its Profile, and the section and the field under the cursor stay when
the game changes.

The game's DLLs section lists the DLL versions with the Catalog's stale marks,
the deny list line when a list names the game, and in Actions the Allow, Deny
and Clear deny list rows. From Phase 5 it also has the Install DLL, Update DLLs
and Restore originals rows: use them directly with Up/Down and Enter, verify
disabled reasons and that Enter is absent for an unavailable selection, review
both pane focus states and both sizes, and confirm fixture file contents after
Update and Restore originals. Merely opening a dialog does not verify the
mutation.

## Terminal Control version

Use Terminal Control 1.2.1 or a newer version that passes this compatibility
check. Version 0.3.1 ignores the cursor backward tab sequence (`CSI Z`), so it
can show a corrupt screen even when the application's output is valid:

```bash
termctrl --version
termctrl start tc-check --cols 20 --rows 3 -- sh -c 'printf "abcdefghij\033[Zx"; sleep 3'
termctrl wait tc-check "x" --timeout 5000
termctrl show tc-check    # passes when it reads abcdefghxj, fails on abcdefghijx
termctrl stop tc-check
```

The 2026-09-13 acceptance review used the unmodified upstream 1.2.1 commit
`c1d4f95e4f1b7638f6229e9bfc9599a955f95ce2`, built under the local cache. To use
that existing review build on the review machine:

```bash
export PATH="/home/jgabor/.cache/spela-termctrl-build/release:$PATH"
termctrl --version
```

This path is a local verification tool, not an application dependency or a
portable installation instruction.

## Start

Build the executable, then run a named session:

```bash
cargo build
termctrl start spela-tui-review --cols 120 --rows 40 -- ./target/debug/spela tui
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
mkdir -p "$review_root/config" "$review_root/data" "$review_root/cache" "$review_root/state" "$review_root/runtime"

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
      XDG_STATE_HOME="$review_root/state" \
      XDG_RUNTIME_DIR="$review_root/runtime" \
      ./target/debug/spela tui
termctrl wait spela-tui-isolated "Library" --timeout 20000
termctrl show spela-tui-isolated
```

Copied game paths still point to real installations. Profile and Settings writes
stay in `review_root`, but DLL and launch operations can touch real game files.
Use deterministic fixtures with temporary game paths for those operations. Never
approve privileged or destructive actions against real game files during review.

`spela --demo` is the safest review: it runs on fictional games and the fake
GPU, polkit and display backends, under a temp dir it removes on quit, so no
real file or setting can change:

```bash
termctrl start spela-tui-demo --cols 120 --rows 40 -- ./target/debug/spela --demo tui
```

To review a migration or a store written by Go spela, copy the fixture sandbox
`tests/fixtures/go/sandbox/main` to a temp dir, point `HOME` and every `XDG_*`
variable into it, and set `SPELA_BACKEND=fake` so no real GPU, CPU, polkit or
display backend is built.

The Go E2E suite checks the Go TUI only and is not kept for the rewrite
(decision 54). Its journeys are pty scenarios on the Rust binary in
`crates/spela/tests/terminal_pty.rs`: mode bytes, restore on quit, on SIGTERM
and on a panic, bracketed paste into Find, the resize prompt, Ctrl-Z and resume,
and the first frame's time.

An empty game database must remain interactive and show `No games found` with
Actions and Settings recovery guidance. For manual review, use an isolated empty
Steam library. Use the displayed Actions menu to choose `Rescan games`; inspect
the outcome and recovery controls. Wait for the visible scan completion before
starting another scan. While a scan, save, or DLL mutation is pending, normal
Quit must wait for a save and stay unavailable with a reason during a scan or DLL
mutation. Do not infer success from the process
running.

## Use displayed controls

The keys are option B (decision 1): digits 0 to 4, arrows, Tab and Shift-Tab,
Enter and Space, plus `?` Help, `:` or Ctrl-K the palette, `/` Find, `q` back
then quit, and `Q` quit now. Esc backs out one level: it closes a popup, an
editor or a search, then moves Detail to List, and it never quits (decision 2).
Ctrl-C quits at once and Ctrl-Z suspends, anywhere. Send a command only when
its key is visible in the current frame or listed in Help.
Text entry is distinct: a focused input accepts ordinary text, including digits,
letters such as `q`, spaces, non-ASCII characters, and punctuation.

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
Enter hint; use Tab to reach Close. Help and Quit are Actions rows too, besides
`?` and `Q`.

For search, focus the Library List and press `/` or choose `Search games`. After
the `/` input appears, send text separately from the key that applies it:

```bash
termctrl send spela-tui-isolated 'text:Cyber'
termctrl show spela-tui-isolated
termctrl send spela-tui-isolated enter
termctrl wait spela-tui-isolated "Cyberpunk 2077" --timeout 20000
termctrl show spela-tui-isolated
```

Search also has selectable Apply, Clear search, and Cancel controls. Every editor
box has the input and `[ Apply ]  [ Cancel ]`. Tab moves between local controls;
Enter activates the focused one. Enter on the input or Apply validates the input
and saves it at once. Check the persisted TOML (`config.toml`,
`profiles/*.toml`, decision 6) and quit through a displayed control before
opening another process on that state.

While an editor, search input, Help, Actions, chooser, or confirmation owns
input, destination numbers must not be shown as active commands. Typing `01234`
in a text field must leave the destination unchanged. In Browse, check the
active pane marker before using arrows or Enter. Read-only Detail views must not
advertise editing. Verify that the essential controls and Help stay
reachable at both sizes.

Confirmations start on Cancel. Inspect target paths and backup policy, use Tab
to reach scrollable details when needed, then return to the buttons. Confirm
requires an explicit selection. Results have a selectable Close control. With
Confirm destructive actions off, DLL updates start without a confirmation;
restore and install still ask.

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
The top bar holds the brand, the destinations and the live metrics (decision 3).
At 100 columns and 30 rows or more both panes show. Below that, down to 80x24,
one pane shows, titled `▸ List` or `▸ Detail`, and Tab reveals the other. Below
80x24 a bounded resize prompt replaces the workspace and only Ctrl-C and Ctrl-Z
work. Confirm that recovery restores the top bar, closed borders, complete
metrics units, and current alerts without stale cells.

The pty scenario `below_80x24_only_the_resize_prompt_shows` in
`crates/spela/tests/terminal_pty.rs` covers the prompt on the real binary; the
snapshots in `crates/spela-ui/tests` pin the frames at 120x40 and 80x24.

## Record results and clean up

Record fixture identity, viewport sizes, displayed key journeys, active frames,
persisted changes, and the important `termctrl show` results. Cover startup,
loading, empty, error, cancellation, and exit states as relevant. Check search,
filters, sort, selection, autosaved profile changes and their undo, Settings,
read-only destinations, and
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
