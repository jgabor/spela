# spela Rust rewrite: research docs

## Summary

spela is being rewritten in Rust: a clean rewrite with full feature parity for the CLI and the TUI, and no GUI. The architecture and terminal UX follow mejl (`~/git/mejl`) wherever that fits. This folder holds the research done on 2026-10-25. The decisions are recorded below, and the architecture doc comes next.

The short version:

- mejl's reusable part is its terminal shell, not its data stack. The event loop, timer, input thread, terminal manager, signal watcher, capability probe, lock, logger, keymap engine, layout and theme code carry over. JMAP, SQLite, the sync engine and the outbox do not.
- The keyboard model keeps spela's basic keys and adds mejl's letters, palette, `q` and `?` on top (decision 1).
- Several current Go behaviours are bugs that should not be ported as they are: wrapper mode hijacks subcommands, restore has holes, and the "clock offset" is really a clock lock.
- The Rust crates are settled for every non-UI need. A starter set builds in 18.5 s and gives a 5.8 MB binary.

## Docs in this folder

| File | What it covers |
|---|---|
| [mejl-study.md](mejl-study.md) | How mejl was researched and documented, its mockups, mejl-ui as built, its binary, terminal layer, tests and infrastructure, and every place mejl's docs and code disagree |
| [parity-inventory.md](parity-inventory.md) | The parity contract: every CLI command, flag, output and exit code, the launch and privilege path, the profile schema and inheritance, storage, env vars, Steam, Proton, display, GPU and CPU, DLLs, the denylist, the overlay, the release check, and the whole TUI. Each item is marked contract, bug or GUI only |
| [terminal-ux.md](terminal-ux.md) | The TUI's frame, layout, keys, popups, themes, glyphs, mouse, terminal handling, CLI output and budgets, and every change from the Go TUI |
| [mockups.html](mockups.html) | Self-contained mock-ups of the main screens on exact cell grids, with a self-check |
| [rust-crates.md](rust-crates.md) | Crates for every non-UI need, with versions, licences and MSRV from crates.io on 2026-10-25, probe builds against real Steam and DLL files, traps, and a starter `Cargo.toml` |

Every research doc ends its sections with the conflicts and open questions its author found.

## Top findings

1. **mejl's shell carries over, its data stack does not.** These parts are generic or close to it:
   - the one-channel main loop, the on-demand timer thread and the input thread that pauses for `$EDITOR`;
   - the terminal mode manager with its panic hook, the signal watcher, and the capability probe with its cache and quirks table;
   - the flock instance lock, the redacting logger and the `Host` trait with a recording fake;
   - in mejl-ui, the keymap engine, layout, hit testing, text width, theme, clock seam, perf ring buffer and the TestBackend harness.

   About 3,600 lines of mejl-ui are generic and 15,100 are adaptable ([mejl-study.md](mejl-study.md)). spela needs plain worker threads for NVML, Steam scans, downloads and `pkexec`. It does not need tokio, a store thread or SQLite (inferred).
2. **mejl's docs drifted from its code.** These are verified in [mejl-study.md](mejl-study.md):
   - there is no dirty flag, and an idle mejl redraws every second;
   - there is no BufWriter, and OSC 8 links are never written;
   - the pty tests check bytes, not screens;
   - CI runs tests only on release tags;
   - the fuzzy matcher is in-house, not nucleo-matcher.

   Use mejl's code as the reference, not its docs.
3. **spela's keyboard contract conflicts with mejl's on most points.** spela uses:
   - digits, arrows, Tab, Enter and Space;
   - Esc only in editors;
   - Help and Quit as Actions rows;
   - no `q`, no palette and no letter commands.

   It has two panes and an 80x24 floor, with one breakpoint at 100 columns or 30 rows. Confirmations start on Cancel. mejl differs on each of these. PLAN.md bans a palette and `q` as back.
4. **spela's own docs disagree about the shipped TUI.** `docs/design/tui-navigation.md`, `docs/tui-testing.md` and the code agree: autosave, Space toggles a field between following All games and its own value, and there are no drafts. PLAN.md phases 2 and 3 and some acceptance docs still describe the retired draft model. Some tests pin views that production never renders.
5. **The wrapper and launch path has real bugs.**
   - Wrapper detection runs before command parsing, so `spela overlay` runs `/usr/bin/overlay` on this machine.
   - A direct `spela launch` cannot launch at all.
   - Game detection can match the wrong game by install-dir substring.
   - Exit codes are only 0 or 1, and the game's exit status is lost.
6. **Restore after privileged changes has holes.**
   - A failed apply registers no restore.
   - Restore is a second `pkexec` prompt at game exit.
   - Clocks reset to driver defaults, not their prior values.
   - An unreadable SMT state restores SMT off.
   - "Clock offset" locks clocks to 2100 plus N MHz, and "memory offset" is a maximum.
   - The polkit action is not scoped to `apply-profile`, and a source install never installs the policy.
7. **Some contracts reach past this machine.**
   - Installed Go clients fetch the DLL manifest at `raw.githubusercontent.com/jgabor/spela/main/data/manifest.json`, so it must not move.
   - The release check reads GitHub tags.
   - The backup layout and `backup.json` must stay readable, because they are how users get their original DLLs back.
8. **Parity is the union of the CLI and the TUI.** Some features exist on one side only:
   - TUI only: editing All games, DLL install and version pinning, cached-only updates and update all.
   - CLI only: denylist management. Removing an allow or a deny is wired nowhere.
9. **The overlay IPC has no consumer.** The 80-byte mmap file is written for nothing, and `docs/design/overlay.md` describes a different, unbuilt protocol. Six overlay fields do nothing. Only the alert evaluator is live, in the TUI header. Eight profile fields do nothing in total.
10. **The crates are settled** ([rust-crates.md](rust-crates.md)):
    - serde-saphyr reads and writes YAML that yaml.v3 accepts, byte-identical for profiles;
    - nvml-wrapper 0.13 has every setter spela uses;
    - keyvalues-parser handles the VDF files, with parity differences listed;
    - `object` plus a port of the Go scan reads PE versions, matching on 1,224 real DLLs;
    - blocking ureq does HTTP, and `std::fs::File::lock` and memmap2 cover the rest.

    The MSRV is 1.89. The starter set is 177 crates, an 18.5 s clean release build and a 5.8 MB binary.
11. **`~/git/spela.old` is an older Rust spela** with no DLL code. Whether it matters is open.

## Decisions

Answered by the user on 2026-10-25. The architecture builds on these.

| # | Decision | Answer |
|---|---|---|
| 1 | Keyboard model | Basic keys plus letters (option B in [terminal-ux.md](terminal-ux.md)): today's digits, arrows, Tab, Enter, Space and Ctrl-C, plus `?` help, `:` palette, `/` find, `q`/`Q` quit. This reverses the 2026-10-14 basic-key rule |
| 2 | Esc | Back one level everywhere. It never quits and never confirms |
| 3 | Layout | spela's contract stays: 80x24 floor, one breakpoint at 100 columns or 30 rows. The Standard, Compact and Focused layouts and `compact_mode` are dropped |
| 4 | Undo or confirm | Keep confirms for DLL and hardware changes, Cancel focused first. Space undoes a single field |
| 5 | Mouse | On by default, with a key for every clickable label |
| 6 | Config and profile format | TOML, migrated from the Go YAML on first read |
| 7 | `--json` output | A new, versioned snake_case schema |
| 8 | Known bugs | Port the fixed behaviour |
| 9 | GPU clock and memory offsets | Real NVML offsets, after one live root test by the user |
| 10 | Overlay and inert fields | Drop the IPC writer, every overlay field (enabled, position, show_*, toggle_key), the Overlay section and the `overlay` command, plus power_mizer and cpu.affinity, until needed. Keep the alert evaluator. The migration notes what it dropped |
| 11 | Concurrency | std threads and blocking ureq |
| 12 | Logs | A rolling file under `XDG_STATE_HOME` for TUI and wrapper runs, plus stderr for CLI warnings |
| 13 | Living with the Go binary | Move on: new file names, the Go files stay as migration backups, no side-by-side support |
| 14 | DLL manifest | Keep the URL. Port `tools/dlss-updater` to Rust |
| 15 | Licence | MIT. MPL-2.0 dependencies are allowed when unmodified |
| 16 | MSRV | 1.89 (recommendation, not asked) |
| 17 | CI | Gate every push and pull request. Release builds on tags |
| 18 | Demo mode | Yes: `spela --demo` on a fake backend, which the tests share |
| 19 | `~/git/spela.old` | Ignore it |
| 20 | CLI and TUI features | Equal on both, including All games editing, DLL install and pinning, and denylist removal |
| 21 | Wrapper detection | Known subcommands parse first. Anything else that is a path or a program on PATH is a game launch |
| 22 | Restore after the game exits | Investigate first: prototype a long-lived privileged helper against two `pkexec` calls, and count prompts under Steam, gamescope and Big Picture |
| 23 | XDG paths | Spec-strict, as etcetera does |
| 24 | Steam scanning | Scan once, in the background. The TUI renders from the game cache at once. The CLI rescans only on an empty cache or `spela scan` |
| 25 | GPUs | GPU 0 only |
| 26 | `spela gui` | Removed |
| 27 | Theme | Terminal ANSI 16 by default, with `neon` and `mono` |
| 28 | Own-value glyph | ⚑ |
| 29 | Locks | A TUI lock as today, plus a launch lock around apply and restore. Both use flock and keep the "already running (PID: N)" message |

## What is not done yet

Each of these is a gap against mejl's method:

- No adversarial fact-check pass ran over these docs. The completeness critic checked 30 claims and found 4 wrong ones, which [parity-inventory.md](parity-inventory.md) corrects at the top.
- No live probes ran. NVML setters, `kscreen-doctor`, `pkexec` and real launches need you, on this machine, and the open questions in each doc say which.
- The probe crates are in a temporary folder (`/tmp/claude-1000/-home-jgabor-git-spela/c455c1a5-ee6c-4ad4-a8a6-b6b0f16f5d64/scratchpad/crate-probe`). They will not survive a reboot.

## Next steps

1. `architecture.md`: the synthesis, in mejl's shape. Principles, an adopt, adapt or reject table for mejl's parts, threads, crate split, file migration, testing, budgets, a phased roadmap where each phase says what it proves, a risk register, and a table of where the sources disagree.
2. A plan in `docs/plans/`, then the build.
