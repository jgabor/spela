# spela Rust rewrite: research docs

## Summary

spela is being rewritten in Rust: a clean rewrite with full feature parity for the CLI and the TUI, and no GUI. The architecture and terminal UX follow mejl (`~/git/mejl`) wherever that fits. This folder holds the research done on 2026-10-25. The architecture doc comes next, after the decisions below.

The short version:

- mejl's reusable part is its terminal shell, not its data stack. The event loop, timer, input thread, terminal manager, signal watcher, capability probe, lock, logger, keymap engine, layout and theme code carry over. JMAP, SQLite, the sync engine and the outbox do not.
- The biggest open choice is the keyboard model. spela's TUI uses basic keys only, a rule you restated on 2026-10-14. mejl uses letter keys, a palette, `q` and `?`. Adopting mejl's UX means reversing that rule, so it is your call.
- Several current Go behaviours are bugs that should not be ported as they are: wrapper mode hijacks subcommands, restore has holes, and the "clock offset" is really a clock lock.
- The Rust crates are settled for every non-UI need. A starter set builds in 18.5 s and gives a 5.8 MB binary.

## Docs in this folder

| File | What it covers |
|---|---|
| [mejl-study.md](mejl-study.md) | How mejl was researched and documented, its mockups, mejl-ui as built, its binary, terminal layer, tests and infrastructure, and every place mejl's docs and code disagree |
| [parity-inventory.md](parity-inventory.md) | The parity contract: every CLI command, flag, output and exit code, the launch and privilege path, the profile schema and inheritance, storage, env vars, Steam, Proton, display, GPU and CPU, DLLs, the denylist, the overlay, the release check, and the whole TUI. Each item is marked contract, bug or GUI only |
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

## Decisions you need to make

Recommendations are inferred. The architecture doc waits on rows 1 to 6.

| # | Decision | Options | Recommendation |
|---|---|---|---|
| 1 | Keyboard model | Keep spela's basic keys. Adopt mejl's letters, palette, `q`/`Q`, `?` and prefixes. Or both, through presets | Take mejl's keymap engine and chrome (top bar, framed panes, status line, hint bar from the keymap). Keep basic keys as the default bindings unless you reverse the 2026-10-14 rule. No key editor, custom presets or macros (not in spela today) |
| 2 | Esc | Editors only (today), or back one level everywhere (mejl) | Back one level everywhere, with confirmations still starting on Cancel |
| 3 | Layout | spela's 80x24 floor and one 100-column or 30-row threshold, or mejl's 160/100 breakpoints and 30x8 floor | Keep spela's contract. It has two panes, not three |
| 4 | Undo or confirm | Undo instead of confirm (mejl), or keep confirms | Keep confirms for DLL and hardware changes. Space already undoes a single field |
| 5 | Mouse | On by default (mejl), off, or later | Later. Every action already has a key, and the review procedure is keyboard only |
| 6 | Config and profile format after migration | YAML (serde-saphyr), or TOML (mejl, toml_edit keeps comments, errors with line and column) | TOML for `config.toml` and the profiles, migrated on first read. Read the old YAML with serde-saphyr only to migrate |
| 7 | `--json` output | Keep Go's PascalCase keys and nulls, or a new snake_case schema | A new, versioned schema, unless something you use parses the current one |
| 8 | Known bugs | Port today's behaviour, or the fixed one | Fixed: wrapper hijack, direct launch, restore holes, the exit code, the XeSS/FSR filter, the denylist failing open, "latest" meaning first in the list |
| 9 | GPU clock and memory "offsets" | Keep the clock locks, or use real NVML offsets | Real offsets, after you run one live root test on your GPU |
| 10 | Overlay and inert fields | Keep the IPC writer and the fields stored only, or drop them | Drop the IPC writer until a layer exists, and keep the alert evaluator. The migration drops the six inert overlay fields with a note |
| 11 | Concurrency | std threads with blocking ureq, or tokio with reqwest as mejl does | std threads and ureq (27 fewer crates, 3.3 MB smaller) |
| 12 | Logs | stderr only (today), or a rolling file under `XDG_STATE_HOME` as mejl does | Both: the file for the TUI and wrapper runs, so restore results under Steam are visible |
| 13 | Living with the Go binary | Share files with Go, or move on | New file names. The Go files stay as the migration backup, and Go is not supported side by side |
| 14 | DLL manifest and `tools/dlss-updater` | Keep the URL and the Go tool, port it, or replace it | Keep the URL. Leave the tool in Go until the rewrite ships |
| 15 | Licence and dependency licences | MIT with permissive deps only (mejl's code), or MPL-2.0 allowed | MIT. Allow MPL-2.0 only when a dependency is unmodified |
| 16 | MSRV | 1.89 or later | 1.89, checked once a 1.89 toolchain is installed |
| 17 | CI | Gate on every push, or on tags only (mejl) | Every push |
| 18 | Demo mode and a fake backend | Yes or no | Yes. A fake backend makes TUI reviews safe and is the second implementation that justifies the backend trait |
| 19 | `~/git/spela.old` | Ignore it, or study it | Ignore it, unless you know of something worth taking |

## What is not done yet

Each of these is a gap against mejl's method:

- No adversarial fact-check pass ran over these docs. The completeness critic checked 30 claims and found 4 wrong ones, which [parity-inventory.md](parity-inventory.md) corrects at the top.
- There is no spela-specific `terminal-ux.md`, and no mockups. Both depend on decisions 1 to 5.
- No live probes ran. NVML setters, `kscreen-doctor`, `pkexec` and real launches need you, on this machine, and the open questions in each doc say which.
- The probe crates are in a temporary folder (`/tmp/claude-1000/-home-jgabor-git-spela/c455c1a5-ee6c-4ad4-a8a6-b6b0f16f5d64/scratchpad/crate-probe`). They will not survive a reboot.

## Next steps

1. You answer the decisions above.
2. `architecture.md`: the synthesis, in mejl's shape. Principles, an adopt, adapt or reject table for mejl's parts, threads, crate split, file migration, testing, budgets, a phased roadmap where each phase says what it proves, a risk register, and a table of where the sources disagree.
3. `terminal-ux.md` and mockups for the chosen keyboard model.
4. A plan in `docs/plans/`, then the build.
