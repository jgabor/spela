# TUI testing

Use [Terminal Control](https://github.com/anomalyco/terminal-control) for every
change that can affect Spela's interactive terminal. It provides a persistent
PTY, keyboard input, terminal resizing, lifecycle inspection, and the rendered
screen an agent or reviewer must actually read.

Unit and state-machine tests remain necessary, but they do not prove that a
fullscreen terminal is readable or usable. Visual review means reading the
visible terminal returned by `termctrl show`. Do not substitute process logs,
raw stdout, or screenshot generation unless the task specifically requires an
artifact.

## Start

Build the compiled executable, then run it in a named terminal session:

```bash
mage build
termctrl start spela-tui-review --cols 120 --rows 40 -- ./spela tui
termctrl wait spela-tui-review "Library" --timeout 20000
termctrl status spela-tui-review
termctrl show spela-tui-review
```

Use a unique session name when another review may run concurrently. Persistent
sessions survive between shell commands, so use the same name for every
operation. `termctrl status` must show the intended command, working directory,
terminal size, and lifecycle state.

The command above uses the caller's normal XDG state. Use it only when
inspection and a possible startup rescan are acceptable: Spela may update the
game database when it is empty or `rescan_on_startup` is enabled. Do not mutate
profiles, settings, or game files in a normal user environment. Use the
isolated setup below whenever startup or the reviewed journey may write state.

## Isolate writable state

Profile editing, settings changes, persistence checks, failure paths, and
empty-state review require an isolated XDG environment. A practical exploratory
review can clone the current Spela configuration and database into a temporary
root:

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

The copied game database still contains paths to real game installations.
Profile and settings writes stay under `review_root`, but DLL install, update,
restore, and launch operations can touch those real paths. Do not exercise
those operations with cloned user data. Use deterministic test fixtures for
game-file mutations, and never approve a privileged or destructive action
against real data through `termctrl send`.

Repeatable acceptance journeys must start from a fresh deterministic fixture,
not a clone of mutable user state. Record the fixture, viewport size, actions,
expected persisted changes, and final visible view. Fixture game paths must be
temporary directories whenever the journey can mutate game files.

To review the no-games path, use an empty home with a minimal empty Steam
library. The command exits after printing the recovery message, so render it as
a disposable terminal command rather than starting a persistent session:

```bash
empty_root="$(mktemp -d /tmp/spela-termctrl-empty-XXXXXX)"
spela_binary="$PWD/spela"
mkdir -p "$empty_root/config" "$empty_root/data" "$empty_root/cache" \
  "$empty_root/runtime" "$empty_root/home/.steam/steam/steamapps"

cat > "$empty_root/home/.steam/steam/steamapps/libraryfolders.vdf" <<EOF
"libraryfolders"
{
  "0"
  {
    "path" "$empty_root/home/.steam/steam"
    "label" ""
    "apps"
    {
    }
  }
}
EOF

termctrl show -- \
  env HOME="$empty_root/home" \
      XDG_CONFIG_HOME="$empty_root/config" \
      XDG_DATA_HOME="$empty_root/data" \
      XDG_CACHE_HOME="$empty_root/cache" \
      XDG_RUNTIME_DIR="$empty_root/runtime" \
      "$spela_binary" tui
```

The visible result must include `No games found. Run 'spela scan' first.`

## Interact

Send text and keys as separate arguments. Wait for visible text instead of
using arbitrary sleeps, then read the updated screen:

```bash
termctrl send spela-tui-review 'text:?'
termctrl wait spela-tui-review "Keyboard shortcuts" --timeout 20000
termctrl show spela-tui-review
termctrl send spela-tui-review escape
```

For a search review, use a title that exists in the isolated database. With the
standard Cyberpunk 2077 fixture, for example:

```bash
termctrl send spela-tui-isolated tab 'text:/' 'text:Cyber' enter
termctrl wait spela-tui-isolated "Cyberpunk 2077" --timeout 20000
termctrl show spela-tui-isolated
termctrl send spela-tui-isolated escape
```

Supported keys include `enter`, `escape`, `tab`, `shift-tab`, arrows, paging
keys, and `ctrl-a` through `ctrl-z`. Use `ctrl-c` when interruption or terminal
restoration is part of the behavior under test. Add `--pace-ms 35` for text
input when a human-readable recording matters.

Read fullscreen TUI state with `termctrl show`, not `termctrl logs`. Logs are
diagnostic evidence only. Resize and read the screen again whenever layout can
change:

```bash
termctrl resize spela-tui-review --cols 80 --rows 24
termctrl show spela-tui-review
termctrl resize spela-tui-review --cols 72 --rows 20
termctrl show spela-tui-review
termctrl resize spela-tui-review --cols 120 --rows 40
termctrl show spela-tui-review
```

## Review

Exercise the paths affected by the change. For a broad TUI review, read each
`termctrl show` result and verify:

- Startup, no-games, loading, empty, error, cancellation, and exit states use
  Spela's current terminology and explain recovery where action is required.
- The header, destination navigation, List pane, Detail pane, status bar,
  message bar, overlays, dialogs, and key hints are visible and do not overlap.
- Focus movement, destination switching, game search, list filtering and sort,
  profile inspection and editing, help, settings, and cancellation work from
  the keyboard where relevant.
- Profile and settings changes made against isolated state are visible, survive
  restart when persistence is expected, and leave the source user state
  unchanged.
- Library, DLL Catalog, Monitor, and Settings views render the selected
  destination. Destination navigation does not enter the pane focus cycle, and
  exactly one eligible List or Detail pane has visible focus.
- At 120x40 and the supported 80x24 minimum, output is bounded and operative,
  status and help remain reachable, and there is no clipped actionable text,
  stale rows, broken borders, or leaked ANSI sequences. Below 80x24, a bounded
  resize prompt appears and the TUI recovers after resizing larger.
- Failure, interruption, and cancellation restore a usable terminal and do not
  leave a stale Spela lock or unexplained partial state.

Do not claim visual verification from unit tests alone. Verification evidence
must include the relevant visible text or a concise description of what each
important `termctrl show` displayed, the terminal sizes reviewed, and the user
journeys exercised.

## Clean up

Inspect the final screen before stopping a crashed or exited session. Always
stop named sessions and remove isolated state when finished:

```bash
termctrl show spela-tui-review
termctrl stop spela-tui-review
termctrl stop spela-tui-isolated
rm -rf "$review_root" "$empty_root"
termctrl list
```

Stopping a session that was never started may report an error; that does not
replace checking `termctrl list` for sessions left behind.
