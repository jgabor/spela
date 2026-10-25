# spela terminal UX

Date: 2026-10-25. Status: decided. This doc adapts mejl's terminal UX (`~/git/mejl/docs/terminal-ux.md` and its code, see [mejl-study.md](mejl-study.md)) to spela's TUI contract ([parity-inventory.md](parity-inventory.md), section 4). The decisions are recorded in [README.md](README.md). The user chose option B: the basic keys of option A plus mejl's letter keys (decision 1), with the mouse on by default (decision 5). [mockups.html](mockups.html) draws the screens.

Labels follow mejl's method: (verified: source), (inferred), (unverified). Design choices here are (inferred) unless marked.

## Summary

- **Take mejl's frame, keep spela's keys.** One top bar, framed panes with titles in the border, a status line and a hint bar. The 7-row logo header and the separate destination bar go away. The destinations and live GPU metrics move into the top bar. That gives the workspace 8 more rows at 120x40 and 4 more at 80x24.
- **Basic keys, plus letters (option B, chosen).** Today's basic keys all stay: digits 0 to 4, arrows, Tab, Enter, Space and Ctrl-C. Esc backs out one level, and Shift-Tab moves to the previous pane. mejl-style letters sit on top: `?` help, `:` palette, `/` find, `q` back then quit, and `Q` quit now. Help and Quit stay as Actions rows too. This reverses the 2026-10-14 basic-key rule, by the user's decision.
- **The mouse is on by default.** Every clickable label shows its key, and no action needs the mouse.
- **Overlays float.** Actions, Help and confirmations open as bordered popups over a dimmed workspace, the way mejl draws its palette and which-key boxes. They no longer replace the whole screen. You can still see what you are acting on.
- **Saving is local first.** A field change shows in the same frame. The write runs on a worker, and the status line goes from "Saving…" to "Saved". A failure puts the stored value back and says why. This is today's behaviour, now with mejl's frame budget.
- **Colour is a theme, not the meaning.** The default theme uses the 16 ANSI colours, so the user's terminal theme carries over. spela's neon palette ships as the `neon` theme. Every state also has a word or a glyph, and `NO_COLOR` gives the `mono` theme.

## Principles

1. **The frame fits 80x24.** Everything an action needs is reachable at the minimum size. Below it, a bounded resize prompt replaces the screen and only Ctrl-C works (verified: parity-inventory.md, layout contract).
2. **Keys you can see are keys that work.** The hint bar and Help come from the same keymap as dispatch. A key never shows unless it works in the current context (verified: spela's `CanonicalKeymap` already does this; mejl does the same for help and which-key).
3. **Basic keys work everywhere.** Every binding works with legacy key encoding, in stock tmux and over SSH. No default needs the kitty keyboard protocol (verified: mejl terminal-ux.md, tmux defaults).
4. **The UI thread never waits.** NVML, `nvidia-smi`, `pkexec`, Steam scans, the manifest and downloads all run on workers. The loop draws at most once per batch of events. It wakes only for input, for worker results, and for the 2 s metrics tick on screens that show metrics.
5. **State is never colour alone.** "Saving…", "Not saved", "Unavailable: <reason>", ✓, ✗ and ⚠ always carry the meaning. Colour only reinforces it.
6. **Real side effects confirm; field edits undo.** DLL install, update, restore and whole-page resets ask first, with focus on Cancel. A single field is undone with Space, as today. mejl's undo-instead-of-confirm does not fit a DLL swap (inferred).

## Frame

```
row 0      top bar:   brand · destinations · live metrics
row 1      ┌ List title ─────┬ Detail title ──────────────────────┐
...        │ list pane       │ detail pane                        │
row h-3    └─────────────────┴────────────────────────────────────┘
row h-2    status line: message on the left, work and save state on the right
row h-1    hint bar: keys for the focused pane, from the keymap
```

- **Top bar.** It holds the reverse-video brand tag ` spela ` and then the destinations `1 Library  2 DLL Catalog  3 Monitor  4 Settings`. The active one is bold accent with a ▸. Digits show only in Browse, as today. Right-aligned, the live metrics `GPU 54° 37% 182W ▂▃▅▆▇ CPU 12%` follow, then the highest alert as a word (`⚠ Power limited`). When space runs out, the sparkline goes first, then CPU, then power, then utilisation and the `GPU` label. The temperature and the alert word are never dropped, so at 80 columns an alert reads `71° ⚠ Power limited`.
- **Panes.** One outer box, with a shared vertical border between List and Detail. A pane's title sits in its top border, in bold accent when the pane has focus and dim otherwise. A scroll thumb ┃ sits on the border when content overflows. This is mejl's frame (verified: mejl mockups.html `topBar`, `box`, `title`, `scrollbar`).
- **Status line.** Messages go on the left: info is plain, success is green with ✓, an error is red with ✗ and the word "Not saved" or "failed". It clears after 5 s, as today. Work and save state go on the right: "Saving…", "Saved", "Scanning games…", or the release notice "spela v0.9.0 is available".
- **Hint bar.** It shows `key label` pairs for the focused pane and mode, with keys in bold accent and labels dim. In Browse, `0 Actions` is always kept, and when the bar runs out of space, lower-priority hints drop first. Arrow keys are written `Up/Down` and `Left/Right`, because the arrow glyphs are outside the safe set. Help (from Actions) lists everything.

What changes from the Go TUI: the 7-row logo header and its 3-row compact form go. The destination bar becomes part of row 0. The message line and the 2-row footer become the status line plus a 1-row hint bar. Compact and Focused layouts are no longer needed for space. The `compact_mode` setting is dropped by the config migration with a note (decision 3).

## Layout

spela's layout contract stays as it is (decision 3):

| Size | Layout |
|---|---|
| 100 columns or more, and 30 rows or more | Two panes. List width is `min(32, max(w/3, 24))`, and Detail takes the rest |
| Under 100 columns or under 30 rows, down to 80x24 | One pane, titled `▸ List` or `▸ Detail`. Tab shows the other pane |
| Under 80x24 | "Resize terminal / spela needs at least 80x24 / Current size: WxH" |

mejl's 160-column breakpoint is not needed, because spela has two panes, not three. Popups clamp to the workspace and keep a one-cell margin.

## Destinations

| Destination | List | Detail |
|---|---|---|
| 1 Library | The Find row, then the pinned All games row, then games with markers (▪ has DLLs, ⚑ has its own profile). The title shows the sort and filters, and "Selected: N" in select mode | The profile page, with game line, own settings, section strip `[Proton] DLSS GPU CPU DLLs`, and rows of label, value and source. The focused row is followed by its description. Or the game's DLLs section |
| 2 DLL Catalog | The groups Library and Deployment (← → switch) | Per family: latest, cached versions and deployments marked current, stale or unknown |
| 3 Monitor | GPU, CPU and Alerts | Sparklines, gauges and the alert list, fed by the same metrics as the top bar |
| 4 Settings | Groups and options, labels only | The option's value, description and save state |

Every empty, loading, error and unavailable state keeps its current wording (verified: parity-inventory.md, destinations table).

## Keys

### Option A: basic keys (the base, all kept)

| Key | Where | Action |
|---|---|---|
| `0` | Browse | Actions popup for this context |
| `1` to `4` | Browse | Go to a destination. Focus returns to List |
| `↑` `↓` | List, Detail, popups | Previous or next item, field, action or line |
| `←` `→` | Detail | Previous or next section, and into DLLs |
| `←` `→` | List in Catalog and Settings | Previous or next group |
| `Tab` / `Shift-Tab` | Browse | Next or previous pane. Shift-Tab is new, as PLAN.md asked |
| `Enter` | List | Open the selection, start Find, or open batch actions |
| `Enter` | Detail | Edit the value, or run the selected DLL action |
| `Space` | List | Select or clear a game |
| `Space` | Detail | Switch the field between its own value and following All games. In Settings, the next value |
| `Esc` | Anywhere | Back one level: close the popup, editor or search, then Detail to List. In Browse on List it does nothing. It never quits and never confirms |
| `Ctrl-C` | Anywhere | Quit at once, even below the minimum size |
| `Ctrl-Z` | Anywhere | Suspend to the shell, redraw on resume (from mejl) |

Inside a text input every printable key, Space and the digits type, as today. Search and the editors keep their Input, Apply, Cancel controls with Tab between them.

Esc changes. Today it works only in editors, and tests pin it as a no-op in Help and in confirmations. Making it "back one level" follows mejl and PLAN.md's own wording ("Escape owns mode-specific back/cancel behavior"). This is decision 2.

### Option B: letter keys on top (chosen)

Everything in option A, plus mejl's habits:

| Key | Where | Action |
|---|---|---|
| `?` | Browse | Help for this context |
| `:` / `Ctrl-K` | Browse | A fuzzy command palette over every Actions row, game and setting. Each row shows its key |
| `/` | Browse | Find in the current list |
| `q` | Browse | Back one level, then quit at the top level. It waits for pending saves, as Quit does today |
| `Q` | Browse | Quit now, after pending saves |

Letters never act while a text input, search or picker has focus, because there they type. Prefixes and which-key popups are not needed at spela's size. This reverses the basic-key rule from 2026-10-14 (docs/design/tui-profile-settings-ux-review-plan.md:110-114), by the user's decision on 2026-10-25.

Not in either option: mejl's key presets, custom presets, key editor and macros. The Go TUI has none of them, so they are YAGNI.

## Popups and in-pane flows

- **Actions** (`0`): a bordered popup titled `Actions · <destination> · <pane>`, anchored to the right half of the workspace, with the screen behind it dimmed. The rows are the same as today. An unavailable row is dim and shows its reason. The cursor starts on the first available row. `Enter` runs the row and `Esc` closes. The Close button stays, so Tab users keep their path.
- **Help**: a large popup from Actions. It shows "Keys here" for the context it was opened from, then a reference table per mode, all from the keymap. It fixes today's bug where an available key shows an "unavailable" reason.
- **Decision and DLL confirmation**: a popup with a body, a target list and `[ Cancel ]  [ Confirm ]`. Cancel has focus first, and Esc cancels. The text stays the same ("1 DLL target(s)", "Backup: …").
- **Editors and pickers** stay inside Detail, under the field they edit, as today. They keep "Value before edit", type-to-jump and "Typing replaces the value".
- **Progress and results** for DLL work stay inside Detail. Progress uses a word ("Work is running. Controls return when it finishes.") and no spinner.

## Theme and glyphs

Semantic roles, as mejl names them, so a theme cannot lose a distinction:

| Role | terminal (default, ANSI 16) | neon (spela's palette, truecolor) |
|---|---|---|
| text, strong | default fg, bold | #e8e8f0 |
| muted: following values, borders, hints | bright black | #6a6a80, border #3a3a58 (brighter than the Go TUI's #202030, for contrast) |
| focus: focused pane title, cursor, keys | blue, bold | #5ff0ff |
| own value, titles | magenta | #ff5fd2 |
| ok, saved | green | #7ce38b |
| warning, saving | yellow | #ffd866 |
| error, not saved, critical | red | #ff6e6e |
| cursor row | reverse video, or a dark background | a dark background |
| thermal scale | cyan, blue, green, yellow, red | the six stops of `thermal.go` |

`mono` is used when `NO_COLOR` is set: bold, dim, reverse and the glyphs only. No theme paints the terminal background.

Glyphs come from the Unicode Neutral width class, so they stay one cell in every locale (method from mejl):

| Glyph | Meaning | ASCII fallback |
|---|---|---|
| ▸ | current destination, focused pane, menu cursor | `>` |
| ⚑ | this game sets its own value / has its own profile (was ◆) | `*` |
| ▪ | game has managed DLLs (was ●) | `+` |
| ✓ | selected, saved, current | `x` |
| ✗ | failed, critical alert, unavailable | `!` |
| ⚠ | warning alert | `!` |
| ⋯ | truncated text | `...` |

Accepted exceptions, each with an ASCII fallback set by `ui.ascii_glyphs`: box drawing (like mejl), including the scroll thumb ┃, the sparkline blocks ▁ to █ and ░ for gauges, `·` (U+00B7) because the source words "this game · All games" use it, and `°`.

## Mouse

On by default (decision 5), with modes 1000, 1002 and 1006 and no motion tracking, as mejl does. `ui.mouse = false` gives the terminal's own text selection back. Each frame records hit regions, as mejl's `HitMap` does, and a click becomes the same action its key gives:

- Click a list row to move the cursor there. Double click opens it, like Enter.
- Click a destination in the top bar, a section in the section strip, a hint in the hint bar, or a button in a popup.
- The wheel scrolls the pane under the pointer.
- Right click opens Actions for what is under the pointer.

Every clickable label shows its key, and no action needs the mouse. Help says that Shift lets the user select text in most terminals.

## Terminal handling

These come from mejl's binary (see [mejl-study.md](mejl-study.md), section 3):

- raw mode and the alternate screen, with restore on every exit path and in the panic hook;
- Ctrl-Z suspend and resume;
- synchronized output when the probe finds it;
- one capability probe with a DA1 sentinel, and a cached result;
- a BufWriter on stdout. mejl's docs ask for one, but its code does not have it.

Bracketed paste is on, so a pasted path never fires keys. Kitty keyboard flags are used only for disambiguation, and nothing depends on them. These parts of mejl stay out: OSC 8 links, notifications and theme switching, because mejl does not use them either.

## CLI output

The CLI shares the theme roles and the vocabulary with the TUI ("from All games", "system default", "this game · All games: X"). Colour only goes to a terminal, and `NO_COLOR` turns it off. Today the Go CLI always writes ANSI (unverified). Errors print `error: <message>` and a one-line hint, not the full usage. Exit codes follow mejl's pattern: 0 ok, 1 failure, 2 usage, 3 config, 5 locked, plus the game's own exit status in wrapper mode (decision 8).

## Budgets

These are taken from mejl and measured in release builds, not debug builds. mejl's own tests only check loose debug bounds.

| Metric | Budget |
|---|---|
| Key to bytes flushed, local action | p99 under 8 ms |
| Warm start to first useful frame (from the cached game list) | p95 under 50 ms |
| Metrics tick | one worker poll every 2 s. The UI only draws the result |
| Idle on a screen without metrics | no wake-ups |

## Changes from the Go TUI contract

| Today | Proposed | Why |
|---|---|---|
| 7-row logo header, 3-row compact header | Metrics in the top bar | Space, and mejl's frame |
| Destination bar on its own row, `◆ 2 DLL Catalog` | Destinations in row 0, `▸ 2 DLL Catalog` | Same |
| Message line plus 2-row footer | Status line plus 1-row hint bar | Same rows, clearer roles |
| Shell overlays replace the screen | Popups over a dimmed workspace | Context stays visible |
| Esc only in editors | Esc backs out one level | mejl, and PLAN.md's wording |
| No reverse Tab | Shift-Tab | PLAN.md asked for it |
| Basic keys only, Help and Quit only in Actions | Basic keys plus `?`, `:`, `/`, `q`, `Q`, and a palette | Decision 1, reversing the 2026-10-14 rule and PLAN.md's palette ban |
| No mouse | Mouse on by default | Decision 5 |
| ◆ and ● (Ambiguous width) | ⚑ and ▪ (Neutral) | Width safety in CJK locales and tmux |
| One neon truecolor theme | `terminal` default, `neon` and `mono` | The user's terminal theme, and `NO_COLOR` |
| Compact and Focused layouts | Not needed | The new frame fits 80x24 without them |

The e2e strings that change with this are `◆ 2 DLL Catalog`, the Compact/Focused layout rows, the `Key: Label` hint strings that become `key label` (`Enter: Edit value`, `Enter: Open selection`, `Enter: Batch actions`, `Enter: close`, `Tab: Focus DLL controls`), the filter tags `●DLLs` and `◆Profile` (now `▪` and `⚑`), and `Saving…` and `→`, which use ASCII (`...`, `->`) in the grids. Every other visible string in the parity inventory stays.

## Open questions

- Labels and descriptions for the new Display settings: Theme, Mouse and ASCII glyphs. The mock-ups use provisional ones.
- Can the cursor sit on the focused row for screen readers while staying hidden? This is open in mejl too.
- Does the top bar's metrics text stay readable when an alert is active at 80 columns? The narrow mockup tests one case only.

## Mock-ups

[mockups.html](mockups.html) draws these screens on exact cell grids, with the same self-check as mejl's:

1. Library with the profile page, 120x40.
2. Editing a field with the picker, 120x40.
3. The game's DLLs with an update confirmation, 120x40.
4. The Actions popup, 120x40.
5. Monitor, 120x40.
6. Settings, 120x40.
7. Narrow, 80x24.
8. The command palette, 120x40.
