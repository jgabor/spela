# spela Rust rewrite: research docs

## Summary

spela is being rewritten in Rust: a clean rewrite with full feature parity for the CLI and the TUI, and no GUI. The architecture and terminal UX follow mejl (`~/git/mejl`) wherever that fits. This folder holds the research done on 2026-10-25 and the architecture built on it. The decisions are recorded below: 1 to 29 by the user, and 30 to 66 by Claude on the user's delegation.

The short version:

- mejl's reusable part is its terminal shell, not its data stack. The event loop, timer, input thread, terminal manager, signal watcher, capability probe, lock, logger, keymap engine, layout and theme code carry over. JMAP, SQLite, the sync engine and the outbox do not.
- The keyboard model keeps spela's basic keys and adds mejl's letters, palette, `q` and `?` on top (decision 1).
- Several current Go behaviours are bugs that should not be ported as they are: wrapper mode hijacks subcommands, restore has holes, and the "clock offset" is really a clock lock.
- The Rust crates are settled for every non-UI need. A starter set builds in 18.5 s and gives a 5.8 MB binary.

## Docs in this folder

| File | What it covers |
|---|---|
| [architecture.md](architecture.md) | The synthesis: principles, what carries over from mejl, threads, crates, files and migration, launch and privilege, testing, budgets, CI, the phased roadmap, risks, where the sources disagree, and the live checks left for the user |
| [phase0-checks.md](phase0-checks.md) | The read-only Phase 0 checks Claude runs on this machine, with their results (decision 63) |
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
11. **`~/git/spela.old` is an older Rust spela** with no DLL code. Decision 19 says to ignore it.

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

## Decisions made by Claude on the user's delegation, 2026-10-26

The user delegated every open question in [architecture.md](architecture.md) to Claude. These answers are binding like the ones above. Where one of them settles an earlier decision, it says so: decision 30 ends the investigation that decision 22 asked for, and decision 32 carries out decision 9 on fakes until the user's root test.

| # | Decision | Answer |
|---|---|---|
| 30 | Restore design (decision 22) | A long-lived root helper. The wrapper spawns `pkexec /usr/bin/spela apply-profile --serve` once, with stdin and stdout pipes carrying JSON lines. The helper reads prior values as root, applies the batch, reports each step, then waits. On a restore request, or on stdin EOF because the wrapper died, it restores the applied steps from their prior values and exits. One prompt per launch. The two-pkexec design is not built. The prompt-count spike stays a user follow-up, confirmed by the user's live launch check |
| 31 | Migrated GPU offsets | Go's `gpu.clock_offset` and `gpu.memory_offset` are not copied into the new offset keys. They are kept in `migration.toml` and the profile file header, and the migration notice says so |
| 32 | Real NVML offsets (decision 9) | The helper wires `set_gpc_clock_vf_offset` and `set_mem_clock_vf_offset`, with `read_prior` and restore, behind `GpuControl`, tested on fakes only. The docs mark them live-unverified until the user's root test |
| 33 | Second launch under the launch lock | It launches with env-only settings, skips VRR and hardware, and warns |
| 34 | Go's `apply-profile --reset` | Not accepted. The helper prints a clear error |
| 35 | Polkit policy | Keeps the action id `io.github.spela.apply-profile` and annotates `org.freedesktop.policykit.exec.path` `/usr/bin/spela` and `org.freedesktop.policykit.exec.argv1` `apply-profile`. Wrapper mode refuses to run as root |
| 36 | Deny lists | The user list adds to the built-in list. Allow and deny for one game are exclusive: setting one clears the other. When a Go file lists a game in both, allow wins |
| 37 | DLL install and restore | Install records `created_files`, and restore deletes them. Before an install, if the current DLL's hash matches neither the backup nor the file spela last installed, the backup is refreshed, because Steam updated it |
| 38 | Stale DLLs | A DLL is stale when it is lower than the highest manifest version for its type. Library and Catalog call one function |
| 39 | Release notice and startup auto-update | TUI only. The CLI stays quiet. `dll update --all` is the CLI operation |
| 40 | `spela launch <game>` without `--dry-run` | Exits 2 with a hint to put `spela %command%` in Steam's launch options |
| 41 | HTTP | Honours the proxy env, as ureq does by default, and sends `User-Agent: spela/<version>` |
| 42 | Metrics and job pool | No metrics pause on focus lost. The job pool has 2 threads |
| 43 | Paths and file details | Relative `XDG_*` values are ignored per the spec, with no warning. TOML files carry a `version = 1` key. The game cache is `<cache>/spela/games.json`. The config migration writes only values that differ from Go's defaults and lists the rest in the record. No warning when a Go file changes after migration. Fixtures and demo data use fictional games and app ids |
| 44 | Live hardware commands in the TUI (row 66) | They come to the TUI (decisions 20 and 4). TUI Actions gain GPU reset, CPU governor and SMT, each behind a confirmation that opens on Cancel, run as a `Cmd::Hardware` job on the pool, taking the launch lock. pkexec runs with `--disable-internal-agent` and a null stdin, so it never prompts on the TUI's tty. With no graphical polkit agent the job fails with a message pointing to the CLI. The profile page gains a read-only Preview launch, the dry-run summary |
| 45 | `overlay` and `gui` | Reserved names. They print that the command was removed in 0.7.0 and exit 2. A program on PATH named `overlay` is never launched for them |
| 46 | Go's `theme`, `show_hints`, `preferred_dll_source` and unknown config keys | Dropped and recorded in `migration.toml` and the notice |
| 47 | `__GL_THREADED_OPTIMIZATION` | Exported only when the game or All games sets the field |
| 48 | Version output | clap's `spela 0.7.0`. `-V`, `--version` and a `-v` alias all work |
| 49 | Corrupt or missing game cache in wrapper mode | Never blocks a launch. The wrapper warns, detects the game from the Steam env and path, and launches |
| 50 | DLL backups | `backup.json` and both name schemes are read in place and never moved or reshaped. New data goes in new top-level keys that Go ignores. The [../AGENTS.md](../AGENTS.md) hard rule is reworded to say so |
| 51 | New command names | `profile follow <field>... [--games]` and `denylist clear <game>` |
| 52 | VDF | keyvalues-parser, last duplicate wins. The other differences from Go's line parser are accepted |
| 53 | Manifest cache and update check | Renamed to `manifest-v1.json` and `release-check.json`, seeded once from Go's files. The DLL cache folder stays shared with Go |
| 54 | Go e2e oracle | Not kept. Rust has its own pty and termctrl checks, plus the Phase 0 transcripts |
| 55 | Perf gate | Deterministic counters are the gate. Wall-clock p99 tests run in release builds with one rerun and report their numbers |
| 56 | CI checks | CI adds cargo-deny for licences and advisories. No CodeQL in v1 |
| 57 | Completions | Static completions from clap_complete are accepted |
| 58 | `nvidia-smi` fallback | None: NVML only. When NVML is missing, GPU features say so |
| 59 | dxvk-nvapi and the DLSS indicator | The values and the indicator env stay exactly as Go emits them |
| 60 | Visible string changes | "with All games settings" and "High temp" at critical are accepted |
| 61 | Accessible mode | Not in v1 |
| 62 | Deleting the Go code | The GUI is deleted in Phase 1. The rest of the Go tree stays untouched as the parity reference until the user's live launch checks pass, then goes in one commit. This replaces the per-phase Go deletions in the roadmap |
| 63 | Read-only Phase 0 checks | Proton tool dirs, `config.vdf` key casing, StateFlags, `kscreen-doctor -o` and the PE versions of installed DLLs are run by Claude and recorded in [phase0-checks.md](phase0-checks.md). Checks that change hardware or launch games stay with the user |
| 64 | MSRV toolchain | The 1.89 toolchain is installed with rustup for the MSRV check |
| 65 | Go profile values with no stored form that launches the same (the migration's known gap) | (a) A game may own `cpu.smt` with no value, which leaves SMT unchanged even when All games sets it. It is stored as `cpu.smt = "unchanged"`, `OwnValues` accepts it, resolve and the shared words show it as `unchanged`, and the migration turns Go's `smt` mark with no value into it. (b) A relative `gpu.shader_cache_path`, and an `sr_mode`, `rr_mode` or `multi_frame` outside the field's list, are dropped, so the field follows. Each one is recorded in `migration.toml` with reason `invalid_in_go` and named on its own line in the migration notice |
| 66 | TUI details accepted as built in Phase 4 | Search games is unavailable in select mode; a palette jump to a game clears filters and selection; the batch row reports its reason; an empty number is refused; Apply to games with their own value writes each game; multi-frame uses a picker; the migration notice is a popup; info alerts stay out of the top bar; failures read 'Not applied: ...' |

## What is not done yet

Each of these is a gap against mejl's method, or a check only the user may run:

- No adversarial fact-check pass ran over these docs. The completeness critic checked 30 claims and found 4 wrong ones, which [parity-inventory.md](parity-inventory.md) corrects at the top.
- The read-only Phase 0 checks are Claude's to run and record in [phase0-checks.md](phase0-checks.md) (decision 63). Until that file has a result for a check, the docs keep that claim (unverified).
- Every check that changes hardware, runs `pkexec` or launches a game is the user's, on this machine. [architecture.md](architecture.md), open questions, lists them. The main ones:
  - the live launch check under desktop Steam, gamescope and Big Picture, which confirms one prompt per launch and that the `--serve` helper restores on every exit path (decision 30);
  - the NVML root test of the clock and memory offsets, which are wired but live-unverified (decision 32);
  - a real KDE VRR apply, SMT and governor restores, real DLL swaps and restores, and the dxvk-nvapi values (decision 59);
  - the network checks: the GitHub tags API, the TLS platform verifier and the daily dlss-updater job.
- The Go code stays until the user's live launch check passes, then goes in one commit (decision 62). The other checks do not gate it.
- The probe crates are in a temporary folder (`/tmp/claude-1000/-home-jgabor-git-spela/c455c1a5-ee6c-4ad4-a8a6-b6b0f16f5d64/scratchpad/crate-probe`). They will not survive a reboot.

## Next steps

1. A plan in `docs/plans/` that follows the roadmap in [architecture.md](architecture.md).
2. The build, phase by phase. Each phase is done when its proof holds.
