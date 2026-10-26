# spela architecture and product direction

Date: 2026-10-26. This doc synthesizes the research docs in this folder, the 29 decisions the user answered on 2026-10-25, and decisions 30 to 68, which Claude made on the user's delegation on 2026-10-26. It adds no new research. Claims carried over from a research doc keep the label they had there, (verified: <doc or path:line>), (inferred) or (unverified), and link the doc they came from. Design choices made here are (inferred) unless a decision in [README.md](README.md) makes them. A decision is cited as "decision N". The sources are:

- [README.md](README.md): the summary, the top findings and decisions 1 to 68. The decisions are binding. Decisions 30 to 68 override any proposal or open question in this doc that they contradict.
- [terminal-ux.md](terminal-ux.md): the decided TUI spec, with [mockups.html](mockups.html) for the screens.
- [mejl-study.md](mejl-study.md): mejl's method, mejl-ui and the mejl binary as built, and where mejl's docs and code disagree. mejl's code facts are the reference, not its docs.
- [parity-inventory.md](parity-inventory.md): the Go behaviour contract. The corrections block at the top overrides its body, and this doc follows the corrections.
- [rust-crates.md](rust-crates.md): crate picks, probes, traps and a starter `Cargo.toml`.
- [../AGENTS.md](../AGENTS.md): the hard rules. mejl's own synthesis, `~/git/mejl/docs/architecture.md`, gave this doc its shape.

Citation scheme. [parity-inventory.md](parity-inventory.md) has five parts with their own numbering. This doc cites them as "CLI 3.4" (CLI surface and launch path), "domain 3" (domain packages), "DLL 2.4" (DLL subsystem, denylist, overlay and release check), "TUI 13" (TUI parity contract) and "additions B.4" (completeness critic), plus "corrections" for the block at the top. Each part also has a conflicts list and an open questions list, cited as "CLI conflicts", "domain open questions" and so on. [mejl-study.md](mejl-study.md) is cited by its parts: "method 5.3" (research method), "mejl-ui keymap" (mejl-ui as built, by section name) and "binary 4.2" (the mejl binary). Paths such as `internal/tui/header.go:81` are in spela's Go code, which still lives in this repo.

## Summary

- One Rust binary, `spela`, replaces the Go CLI and TUI with full parity between the two, and drops the GUI (decisions 20 and 26). It also replaces the Steam wrapper (`spela %command%`), which applies a profile, runs the game and restores what it changed.
- mejl's terminal shell carries over and its data stack does not. The event loop, timer, input thread, terminal manager, signals, probe, lock, logger, keymap, layout, hit map, theme and test harness are adopted or adapted. JMAP, SQLite, the sync engine and the outbox are rejected ([README.md](README.md), finding 1). Plain std threads and blocking ureq do the rest, with no tokio (decision 11).
- User data moves from Go's YAML to TOML on first read. Every value the user set is kept, and the Go files stay as backups. The migration is tested against fixtures written by the Go code (decisions 6 and 13; [../AGENTS.md](../AGENTS.md)).
- Decision 10 drops the overlay IPC writer, every overlay field, the Overlay section, the `overlay` command, `gpu.power_mizer` and `cpu.affinity`. The migration records what it dropped. The profile goes from 35 fields to 25 (inferred count). The alert evaluator stays.
- The Go bugs that [parity-inventory.md](parity-inventory.md) lists are ported in their fixed form (decision 8). The biggest are in the launch path: wrapper detection that hijacks subcommands, restore holes after a privileged apply, and a "clock offset" that is really a clock lock.
- The restore after the game exits runs in a long-lived root helper, `apply-profile --serve`, started by one `pkexec` per launch (decision 30). Real NVML offsets are wired behind `GpuControl` and tested on fakes (decision 32). Both stay live-unverified until the user's live launch check and root test, which only the user may run.

| Goal | Verdict | One line |
|---|---|---|
| CLI and TUI parity, no GUI | Yes | Every operation is a core function that both front ends call (decision 20). The live hardware commands come to TUI Actions (decision 44). The release notice and the startup DLL auto-update stay TUI only (decision 39) |
| Migration from the Go files | Yes, gated on fixtures | Fixtures are captured from the Go code, which stays until the user's live checks pass (decision 62). Go offsets are recorded, not copied (decision 31) |
| Restore after the game exits | Designed, live-unverified | The `--serve` helper restores on request or when its stdin closes (decision 30). The user's live launch check confirms the prompt count |
| Real NVML offsets | Wired, live-unverified | The setters exist in nvml-wrapper (verified: [rust-crates.md](rust-crates.md), NVML). The helper wires them with prior reads and restore, tested on fakes until the user's root test (decisions 9 and 32) |
| Instant TUI | Yes by design, unproven | The UI thread never blocks. Deterministic counters gate every push, and the wall-clock [terminal-ux.md](terminal-ux.md) budgets run in release builds and report their numbers (decision 55) |
| Hard safety rules | Yes by construction | Every hardware, privilege, network and path seam has a fake, plus PATH stubs and grep gates that fail a test when a rule breaks |

## Guiding principles

1. The UI thread never waits. NVML, `nvidia-smi`, `pkexec`, Steam scans, the manifest, downloads and file writes run off it, and their results come back as messages ([../AGENTS.md](../AGENTS.md) hard rule; [terminal-ux.md](terminal-ux.md) principle 4). This is stricter than mejl as built: its UI thread writes the config file with `sync_all` and blocks on engine shutdown (verified: [mejl-study.md](mejl-study.md), binary 2.4).
2. One core, two front ends. Every operation is a function in the core crate. The CLI calls it on the main thread, the TUI calls it on a worker, and neither has a feature the other lacks. That includes All games editing, DLL install and version pinning, cached-only updates, update all, denylist removal, the live hardware commands and the dry-run summary (decision 20; the current split is verified in [parity-inventory.md](parity-inventory.md), DLL conflicts). The live `gpu reset`, `cpu governor <g>` and `cpu smt <v>` become TUI Actions, each behind a confirmation that opens on Cancel and run as a `Cmd::Hardware` job (decision 44). `launch --dry-run` becomes the profile page's read-only Preview launch (decision 44). Three exceptions are decided: the release notice and the startup DLL auto-update run in the TUI only, and the CLI's operation is `dll update --all` (decision 39); and only the wrapper launches a game, because the TUI never does and `spela launch` without `--dry-run` exits 2 (decision 40).
3. User intent survives the move. Config, profiles, denylist overrides and DLL backups hold what the user chose. The Go config, profile and list files are migrated to TOML the first time they are read. Every value the user set is kept, or, where decisions 10, 31 and 46 drop it, recorded in `migration.toml`, and the Go files stay where they are as backups (decisions 6 and 13; [../AGENTS.md](../AGENTS.md) hard rule). DLL backups are read in place and never reshaped (decision 50). The game cache is different: deleting it costs one scan (inferred, after mejl's split between a deletable cache and durable state, verified: [mejl-study.md](mejl-study.md), binary 14).
4. Restore what was there, or say that you cannot. Each privileged change registers its restore before the game starts ([../AGENTS.md](../AGENTS.md) hard rule). The restore uses the value read before the change. When that value could not be read, spela restores nothing for that setting and logs why. It never restores a guess. Today an unreadable SMT state restores SMT off, and clocks always reset to driver defaults (verified: [parity-inventory.md](parity-inventory.md), CLI 7.2). The restore runs in the same root helper that applied the change, which reads the prior values as root and restores them on request or when the wrapper dies (decision 30).
5. Fakes first. Every backend that touches hardware, privilege, the display or the network sits behind a seam with two implementations, the real one and a fake. Tests and `spela --demo` share the fake (decision 18). No test runs `pkexec`, changes hardware, writes game files, contacts a remote host or touches the real XDG dirs ([../AGENTS.md](../AGENTS.md) hard rules).
6. Port the contract, fix the bugs. Behaviour that the parity inventory marks as a bug is ported in its fixed form. Known subcommands parse before wrapper detection, the wrapper returns the game's exit status, a partial apply restores what it changed, and launch signals are handled from the start (decisions 8 and 21; bugs verified in [parity-inventory.md](parity-inventory.md), CLI 1, 5.1, 6.7 and 7.2).
7. mejl's code is the reference, not its docs. mejl's docs promise a dirty flag, a BufWriter and an idle loop, and its code has none of them (verified: [mejl-study.md](mejl-study.md), binary 16; [README.md](README.md), finding 2). Where this doc wants what mejl's docs describe, it says so and builds it.
8. Build only what spela uses. No key presets, key editor, macros, theme files, OSC 8 links or clipboard writes ([terminal-ux.md](terminal-ux.md), keys and terminal handling). No overlay IPC writer, no overlay fields, no Overlay section, no `overlay` command, no `power_mizer` and no `cpu.affinity` (decision 10). Each of these comes back only with a real use.
9. Keys you can see are keys that work. The hint bar, Help and the palette read the same keymap that dispatches keys. Every default binding works with legacy key encoding, in stock tmux and over SSH ([terminal-ux.md](terminal-ux.md) principles 2 and 3).
10. Idle means idle, and speed is measured. The loop wakes for input, worker results, a status message that is due to clear, and the 2 s metrics tick. Deterministic counters of draws, wake-ups, polls and NVML init gate every push. The wall-clock budgets in [terminal-ux.md](terminal-ux.md) run as release-mode tests in CI and report their numbers (decision 55). mejl claims its budgets are enforced but only asserts loose debug bounds (verified: [mejl-study.md](mejl-study.md), mejl-ui tests).

## Product direction and non-goals

spela is one Linux binary for NVIDIA and Proton tuning around Steam launches. It has three faces: a CLI for scripts and quick changes, a TUI for browsing the library and editing profiles, and a Steam wrapper (`spela %command%`) that applies a profile, runs the game and restores what it changed. The rewrite covers the CLI and the TUI with full parity, which means the union of what either does today, and drops the GUI (decisions 20 and 26). The architecture and terminal UX follow mejl where that fits ([README.md](README.md), summary).

What has to keep working for people who used the Go version:

- the DLL manifest at `raw.githubusercontent.com/jgabor/spela/main/data/manifest.json`, kept by a Rust port of `tools/dlss-updater` (decision 14; the URL is verified in [parity-inventory.md](parity-inventory.md), DLL 1.1);
- plain `vX.Y.Z` release tags newer than the Go ones (up to v0.6.2), because the Go release check reads them (verified: [parity-inventory.md](parity-inventory.md), DLL 1.2);
- the DLL backup index `backup.json` and both backup file name schemes, because they are how users get original DLLs back (verified: [parity-inventory.md](parity-inventory.md), DLL 1.3);
- the polkit action id `io.github.spela.apply-profile` and the install path `/usr/bin/spela` (verified: [parity-inventory.md](parity-inventory.md), CLI 7.3).

| Non-goal | Why | Source |
|---|---|---|
| A GUI, and `spela gui` | Removed | decision 26 |
| The overlay IPC writer, `SPELA_OVERLAY_IPC`, every overlay field, the Overlay section, the `overlay` command, and the `overlay:` impact group and Overlay block of the dry-run summary | No consumer exists (verified: [README.md](README.md), finding 9). The migration records what it dropped. The alert evaluator stays, under its own name, because the TUI uses it | decision 10; [parity-inventory.md](parity-inventory.md), DLL 4 and CLI 6.9 |
| `gpu.power_mizer` and `cpu.affinity` | Stored but never applied (verified: [parity-inventory.md](parity-inventory.md), domain 1) | decision 10 |
| GPUs other than device 0 | Go uses index 0 only (verified: [parity-inventory.md](parity-inventory.md), domain 10) | decision 25 |
| Running beside the Go binary | New file names. The Go files stay only as migration backups | decision 13 |
| YAML on disk, and Go's `--json` shape | TOML on disk, and a new versioned snake_case `--json` | decisions 6 and 7 |
| tokio, an async HTTP stack, SQLite, a sync engine | Plain threads and blocking ureq are enough | decision 11; [README.md](README.md), finding 1 |
| Key presets, a key editor, macros, theme files | The Go TUI has none of them | [terminal-ux.md](terminal-ux.md), keys and theme |
| OSC 8, OSC 52, terminal notifications, live theme switching, `$EDITOR` handoff | spela has no action that needs them | [terminal-ux.md](terminal-ux.md), terminal handling (inferred for `$EDITOR`) |
| macOS and Windows | spela is Linux and NVIDIA only (inferred from [../AGENTS.md](../AGENTS.md)). mejl's darwin release target goes | [mejl-study.md](mejl-study.md), method 5.5 |

Later, only on evidence: see [Later](#later-only-on-evidence) in the roadmap.

## What carries over from mejl

Every part [mejl-study.md](mejl-study.md) names gets a verdict. Adopt means the code lifts as is or with the fixes listed. Adapt means the mechanism carries over and the content is spela's. Reject means spela does not need it. Paths are relative to `~/git/mejl`. Facts about mejl keep their label from [mejl-study.md](mejl-study.md). Verdicts are (inferred) unless a decision makes them.

| Part | mejl path | Verdict | Reason |
|---|---|---|---|
| Event loop | `crates/mejl/src/app/mod.rs` (`run`, `handle_batch`, `after_turn`, `draw`), `crates/mejl/src/events.rs` | Adopt, with fixes | Block for one event, drain the rest, then draw once (verified: binary 4.1). Add the dirty flag and BufWriter its docs promise and its code lacks (verified: binary 4.2) |
| Timer thread | `crates/mejl/src/timer.rs` | Adopt, with two changes | Generic and on demand (verified: binary 15). Start it with `Timer::spawn_with(tx, TickMode::Every(2 s))` for the metrics tick. `Timer::spawn` defaults to `TickMode::WallSecond`, one tick per wall-clock second, and mejl documents `Every` as used by tests (verified: `~/git/mejl/crates/mejl/src/timer.rs:23-27, 68-69`). Dedupe wakes by deadline: mejl turns one deadline into many instants on busy turns (inferred there: binary 4.4) |
| Input thread | `crates/mejl/src/input.rs` | Adapt | Keep the read-time stamp, the 1,024-event batch cap, coalescing and the leaked-reply filter (verified: binary 8). Use a blocking `crossterm::event::read` instead of `EventStream` with pause, because spela has no `$EDITOR` handoff. See [The event loop](#the-event-loop) |
| Terminal manager | `crates/mejl/src/terminal/mod.rs` | Adopt, with fixes | Mode order, reverse restore, panic hook and idempotent restore are generic (verified: binary 5). Fix the panic classification of spawned threads, which leaves a live UI with no input (inferred there: binary 5.3) |
| Signals | `crates/mejl/src/terminal/signals.rs` | Adopt for the TUI only | First stop signal quits, a second exits 128 plus n, SIGCONT repaints (verified: binary 6). Install it before the probe, which mejl does not (inferred there: binary 5.2). Wrapper mode gets its own forwarding policy |
| Probe and caps cache | `crates/mejl/src/terminal/probe.rs`, `terminal/cache.rs` | Adapt | One write with a DA1 sentinel, 150 ms timeout, late-reply drain, and a cache that only a complete probe writes (verified: binary 7.1 to 7.3). Query kitty flags, DECRQM 2026 and XTVERSION. Drop the theme and OSC 11 queries: mejl uses them only in doctor and the cache (verified: binary 7.6), and spela has no automatic theme |
| Quirks | `crates/mejl/src/terminal/quirks.rs` | Adopt | `TERM=dumb` skips the probe, vscode gets 400 ms, each entry says why (verified: binary 7.4) |
| OSC 52 | `crates/mejl/src/terminal/osc.rs` | Reject | No copy action in the spec. OSC 8 goes too ([terminal-ux.md](terminal-ux.md), terminal handling) |
| Lock | `crates/mejl/src/lock.rs` | Adapt | flock, a PID written only for the message, close-on-exec (verified: binary 11). Use std `File::try_lock`, which mejl avoided only because of its 1.88 MSRV (verified: same). Two instance locks, the TUI lock and the launch lock (decision 29), plus the two store locks `config.lock` and `games.lock` (see [Locks](#locks)) |
| Logger | `crates/mejl/src/logging.rs` | Adapt | Rolling file, level precedence, no ANSI in files (verified: binary 11). File under `XDG_STATE_HOME` for TUI and wrapper runs, stderr for CLI warnings (decision 12). Drop token redaction: spela holds no secrets |
| Host trait | `crates/mejl/src/app/host.rs:17-41` | Adapt | Keep `suspend` plus the recording fake. The sync markers move off `Host`: mejl's `Host` writes them through its own `io::stdout()` handles (verified: binary 2.3), and spela writes them through `terminal.backend_mut()`, the one BufWriter (inferred). Drop `open_url`, `open_path`, `copy` and `edit`, which no spela action uses (verified: host.rs:17-41) |
| Keymap engine | `crates/mejl-ui/src/keymap/key.rs`, `keymap/mod.rs` | Adapt | Keep `Key` parsing and display, `legacy_problem`, contexts where the first one wins, `keys_for` and `check` (verified: mejl-ui keymap). No prefixes, no counts and no user `[keys]` table ([terminal-ux.md](terminal-ux.md): prefixes and which-key are not needed) |
| Presets and key editor | `crates/mejl-ui/src/keymap/preset.rs`, `keys_screen.rs`, `update/keys.rs`, `view/keys.rs` | Reject | About 4,600 lines (inferred there) for something the Go TUI does not have ([terminal-ux.md](terminal-ux.md), keys) |
| Macros | `crates/mejl-ui/src/macros.rs`, `update/macros.rs` | Reject | Mail-change steps only (verified: mejl-ui macros) |
| Palette and fuzzy matcher | `crates/mejl-ui/src/fuzzy.rs`, `picker.rs`, `update/overlays.rs` | Adopt the matcher, adapt the picker | `fuzzy.rs` is 165 generic lines (verified: mejl-ui palette). Decision 15 now allows MPL-2.0 nucleo-matcher, but reusing `fuzzy.rs` needs no dependency. Palette rows are Actions rows, games and settings ([terminal-ux.md](terminal-ux.md), option B) |
| Layout | `crates/mejl-ui/src/layout.rs` | Adapt | A pure function of size shared by update and render (verified: mejl-ui layout). spela's breakpoints: 80x24 floor, one breakpoint at 100 columns or 30 rows (decision 3) |
| Hit testing | `crates/mejl-ui/src/hit.rs` | Adopt | `HitMap` of `(Rect, HitTarget)` in draw order (verified: mejl-ui hit testing). spela's own targets |
| Theme | `crates/mejl-ui/src/theme.rs` | Adapt | Semantic style names, `NO_COLOR` forces mono, an ASCII glyph set (verified: mejl-ui theme). Themes `terminal`, `neon` and `mono` (decision 27). No theme files |
| Text width | `crates/mejl-ui/src/text.rs` | Adopt | Grapheme width, sanitising of bidi and control characters, truncate and wrap (verified: mejl-ui text). Game names come from Steam |
| Clock seam | `crates/mejl-ui/src/clock.rs` | Adopt | `Clock` with `SystemClock` and `ManualClock` (verified: mejl-ui text) |
| Perf ring buffer | `crates/mejl-ui/src/perf.rs` | Adopt | Last 1,000 samples, percentiles, buckets (verified: mejl-ui text). Add the release benchmark mejl lacks |
| TestBackend harness | `crates/mejl-ui/tests/common/mod.rs` | Adopt the pattern | ManualClock, fake data, a loop that plays the shell, insta snapshots (verified: mejl-ui tests). Sizes 80x24 and 120x40 ([../AGENTS.md](../AGENTS.md)) |
| pty tests | `crates/mejl/tests/terminal_pty.rs` | Adopt, and extend | `openpty` scenarios for mode sequences, suspend and the probe (verified: binary 12.1). They check bytes, not screens. spela adds screen reads of the real binary with vt100 (see [Testing strategy](#testing-strategy)) |
| Transport trait and fake | `crates/mejl-jmap/src/transport.rs`, `crates/mejl-fake` | Adapt | The seam with a fake as the second implementation carries over (verified: binary 14). spela's is a blocking `Http` trait with an in-process fake. No axum and no loopback server in the release binary, which mejl compiles in (verified: binary conflicts) |
| Demo mode | `crates/mejl/src/demo.rs`, `cli.rs` `run_demo` | Adapt | Fake backend, private temp dir removed on drop, config copied so settings never touch the real file (verified: binary 9.4 and 11). In process rather than over loopback (decision 18) |
| Store thread | `crates/mejl-sync/src/store_thread.rs` | Adapt the pattern only | One thread runs writes in send order (verified: binary 14). spela's write queue does this for profiles, settings and the denylist, with no SQLite |
| SQLite cache and state | `crates/mejl-store` | Reject | Files are enough ([README.md](README.md), finding 1). The disposable-cache principle carries over (principle 3) |
| Sync engine | `crates/mejl-sync/src/engine.rs` | Reject | No server to sync with ([README.md](README.md), finding 1) |
| Outbox | `crates/mejl-store` outbox tables | Reject | Saves are local. A failed save puts the stored value back ([terminal-ux.md](terminal-ux.md), summary) |
| Render pool | `crates/mejl/src/render_pool.rs` | Reject | spela renders no HTML. Its `catch_unwind` per job carries over to the job pool |
| toml_edit config writer | `crates/mejl/src/app/settings.rs` | Adopt | Keeps comments, re-parses before replacing, writes atomically with `sync_all` (verified: binary 10.3). TOML is now spela's format (decision 6). Runs on the write queue, not the UI thread |
| Doctor | `crates/mejl/src/doctor.rs` | Reject for v1 | No Go counterpart. Later, only on evidence |
| CLI context and exit codes | `crates/mejl/src/cli.rs` | Adopt | `run(cli, ctx, out, err)` as the test seam and a `CliError { code, message }` (verified: binary 9.2 and 9.3). Codes 0, 1, 2, 3 and 5 plus the game's status in wrapper mode ([terminal-ux.md](terminal-ux.md), CLI output) |
| CodeQL filter | `.github/workflows/codeql.yml`, `.github/codeql/filter-tests.py` | Reject for v1 | No CodeQL in v1 (decision 56). The filter drops results in test code, relying on inline tests last (verified: method 5.3). If CodeQL comes later, ban out-of-line `mod tests;` modules, which the filter misses (verified there). See [CI](#ci) |
| justfile and CI | `justfile`, `.github/workflows/release.yml` | Adapt | mejl gates only release tags and never checks its MSRV in CI (verified: binary 13.4). spela gates every push and pull request (decision 17), checks MSRV 1.89 (decision 16), and releases Linux targets only |
| Env snapshot | `crates/mejl/src/env.rs` | Adopt | Read the environment once. Nothing below the CLI calls `std::env::var` (verified: binary 11). Fix mejl's own `HOME` read (verified there) |
| Paths | `crates/mejl/src/paths.rs` | Adapt | etcetera spec-strict XDG and `Paths::under(base)` for tests (verified: binary 11; decision 23) |
| Config validation | `crates/mejl/src/config.rs` | Adopt the pattern | Errors with line and column, all at once. Unknown keys warn with a suggestion. An example printer (verified: binary 10.2) |
| `LineInput` | `crates/mejl-ui/src/input.rs` | Adopt | One-line editing for Find and text fields (verified: mejl-ui text) |
| External editor, compose editor, dates and density | `crates/mejl/src/terminal/editor.rs`, `mejl-ui/src/compose/editor.rs`, `dates.rs`, `state.rs` | Reject | No spela screen needs them |
| Theme detection (mode 2031) and notifications | `crates/mejl/src/terminal/theme.rs` | Reject | Dormant in mejl, and notifications were never built (verified: binary 7.5 and 7.6) |
| Accessible mode | `crates/mejl-ui/src/view/mod.rs` | Reject for v1 | No accessible mode in v1 (decision 61). Only the cursor parking was built in mejl (verified: mejl-ui conflicts) |

## System architecture

spela runs in four shapes from one binary: the TUI, the wrapper, the CLI and the hidden root helper. The TUI is the only one with threads of its own. The wrapper has a signal thread. It talks to the root helper on the main thread, because each exchange is a request followed by report lines. The CLI and the root helper are single threaded (inferred).

### TUI process

```
 tty ------> [input thread]  blocking read, stamp, coalesce ---- Input(batch) ----+
 signals --> [signal thread] TERM, INT, HUP, CONT -------------- Signal ---------+
 [timer thread] condvar on the next deadline ------------------- Tick / Wake ----+
                                                                                  v
                    [UI thread: owns the terminal, App and AppState]
                     recv one event, drain the rest, coalesce
                     update(&mut AppState, Event) -> Vec<Cmd>
                     exec each Cmd; after_turn sets ticking and the next wake
                     if dirty: draw once into BufWriter<Stdout>, inside sync markers
                          |  Poll           |  Job                 |  Write
                          v                 v                      v
                  [metrics thread]   [job pool, 2 threads]   [write queue, 1 thread]
                   owns the Nvml      Steam scan, manifest,   profile fields, All games,
                   handle; GPU 0      DLL install, update,    follow All games, settings,
                   plus /proc, /sys   restore, auto-update,   denylist overrides; in send
                   CPU reads, and     release check, Proton   order; flock per write
                   the alert          resolve, Preview
                   evaluator          launch, hardware
                                      (one-shot pkexec)
                          |                 |                      |
                          +--------- Done(Msg) into the UI channel +
```

- The metrics thread owns NVML. nvml-wrapper's `Device` borrows `Nvml`, so one owner is simpler than sharing (inferred from [rust-crates.md](rust-crates.md), NVML). A 12-query sweep takes about 7 ms in the driver (verified: same), and a poll that is still running is never queued again. It runs the alert evaluator on each sweep and sends the alerts with the metrics (see [Alert evaluator](#alert-evaluator)).
- The job pool is separate so a two-minute DLL download (verified: [parity-inventory.md](parity-inventory.md), DLL 2.6) never delays a metrics poll. It has 2 threads, so a scan can run beside a download (decision 42).
- Hardware jobs. The Go TUI never calls the privilege package (verified: grep of spela `internal/tui`, no hits). The Rust TUI does, for the live hardware commands (decision 44):
  - Actions offers GPU reset, CPU governor and SMT. Each opens a confirmation that starts on Cancel (decision 4). Confirm sends `Cmd::Hardware`, and the result comes back as `Msg::HardwareDone`.
  - The job takes the launch lock without blocking, as the CLI commands do (see [Launch lock](#launch-lock)). A held lock fails the job with `spela is already running (PID: N)`, and nothing is changed.
  - It runs the one-shot helper, `pkexec --disable-internal-agent /usr/bin/spela apply-profile --once <request>`, with a null stdin, so pkexec can never start a text agent on the TUI's tty (decision 44). The one-shot request travels as one JSON argument for that reason (see [Protocol](#protocol)). Stdout and stderr are pipes, so nothing is written over the frame.
  - With no graphical polkit agent, pkexec cannot ask, and the job fails. The status line says so and points to the CLI, for example `✗ GPU reset needs a polkit agent. Run 'spela gpu reset' in a terminal`. How pkexec reports a missing agent, by exit code or by message, is a live check for the user (see [Open questions](#open-questions)). Until then any failure that is not a dismissal shows the helper's error text plus that hint (inferred).
  - Only one hardware job runs at a time. While it runs, the other hardware rows are unavailable with `wait for pending work`, and Quit is refused with `wait for pending operations`, as for a DLL job (inferred).
- Preview launch. The profile page's read-only dry-run summary (decision 44) is built on the job pool as `Cmd::Preview`, because building the plan reads `config.vdf` for Proton and NVML for the driver version. It never takes a lock and never runs pkexec or `kscreen-doctor`, as the CLI dry run does not (see [One plan for dry run and launch](#one-plan-for-dry-run-and-launch)).
- The write queue adapts mejl's store thread: one writer, jobs in send order (verified: [mejl-study.md](mejl-study.md), binary 14). Queued writes for the same file merge by key: the newest value wins per key, and a deletion counts as an entry. The merged write keeps the oldest stored values, so a failure puts back every merged key. Go's queue does the same by keeping the oldest `before` and the newest `desired` and running `MergeChanges` (verified: [parity-inventory.md](parity-inventory.md), TUI 9; `internal/tui/content.go:105-120`). After a failure, the next write for that file starts from the failed write's stored values (inferred). Each write opens a fresh descriptor for its flock, because Go's flock on separate opens excludes within one process too (verified: [parity-inventory.md](parity-inventory.md), domain 3 and 15). A failed write sends the error back, and the UI puts the stored value back ([terminal-ux.md](terminal-ux.md), summary).
- Quit. `q`, `Q`, Quit and the palette's Quit are refused with `wait for pending operations` while a scan, DLL or hardware job runs, as today for scans and DLL jobs (verified: [parity-inventory.md](parity-inventory.md), TUI 6; `internal/tui/layout_actions.go:10-14`). Pending saves do not refuse Quit. On quit the loop exits and the terminal is restored, and then `main` joins the write queue. A failed save is printed to stderr after the restore. mejl shuts its engine down after its terminal guard drops in the same way (verified: [mejl-study.md](mejl-study.md), binary 3 step 8). So the join is off the UI path (inferred).
- A first SIGTERM, SIGINT or SIGHUP during a DLL job lets the job finish its current backup, replace, rescan and save before the process exits. A second signal exits at once, and the next start's fail-closed backup check covers that case (inferred). As built in Phase 5: the stop sets a flag the job shares (`DllContext::stop`). A download ends at its next chunk and its temp file goes; a batch stops between items, the items not started fail as `stopped because spela is quitting`, and the save-once still runs for the swaps that happened. The loop keeps running until the job answers, logging rather than failing on a draw error, because after SIGHUP the terminal is gone. Ctrl-C during a DLL job and a terminal that stops sending input take the same path; a second Ctrl-C quits at once. The status line says `Quitting after this DLL step...`.
- Every job-pool job runs under `catch_unwind` and turns a panic into an error message, as mejl's render pool does (verified: [mejl-study.md](mejl-study.md), binary 4.5). A panic on a long-lived thread (input, timer, signal, metrics or the write queue) is fatal: the hook restores the terminal and the process exits, because a UI that looks alive with a dead thread behind it is worse (inferred; mejl's gap is verified in [mejl-study.md](mejl-study.md), binary 5.3).

### Channels and message types

| Path | Mechanism | Source |
|---|---|---|
| Every thread to the UI | One unbounded `std::sync::mpsc` of `LoopEvent<Msg>`, generic over the message type | mejl's `events.rs` (verified: [mejl-study.md](mejl-study.md), binary 2.2) |
| UI to the timer | `Mutex` plus `Condvar`, wakes in a `BTreeSet<Instant>` | mejl's `timer.rs` (verified: same) |
| UI to the metrics thread | `std::sync::mpsc` of `Poll`, at most one in flight | (inferred) |
| UI to the job pool | `std::sync::mpsc` of boxed jobs behind a shared receiver | (inferred) |
| UI to the write queue | `std::sync::mpsc` of `Write`, one consumer | (inferred) |
| UI to the input thread | None. The thread reads until the process exits | (inferred) |

```rust
// spela binary: everything that reaches the UI thread
enum LoopEvent<M> {
    Input(Vec<TimedEvent>), // stamped at read time, coalesced, at most 1,024
    InputClosed,
    Signal(Signal),         // Stop(TERM | INT | HUP) or Continue
    Tick,                   // the metrics cadence, only while ticking is on
    Wake(Instant),          // one-off deadlines
    Done(M),                // a worker or the write queue finished
}

// spela-ui: results the UI understands
enum Msg {
    Metrics { gpu: Result<GpuMetrics, String>, cpu: Result<CpuMetrics, String>, alerts: Vec<Alert> },
    ScanDone { request: u64, result: Result<GameCache, String> },
    ManifestLoaded { request: u64, result: Result<Manifest, String> },
    DllProgress { token: u64, event: ProgressEvent },
    DllDone { token: u64, result: Result<OpResult, OpError> },
    Saved { token: u64, result: Result<(), String> },
    ReleaseNotice(Option<String>),
    AutoUpdateNotice(String),
    ProtonNotice { app_id: u64, notice: Notice },
    HardwareDone { token: u64, result: Result<ApplyReport, PrivilegeError> },
    PreviewReady { request: u64, result: Result<LaunchSummary, String> },
}

// spela-ui: requests to the shell; update does no I/O itself
enum Cmd {
    PollMetrics,
    Scan { request: u64 },
    LoadManifest { request: u64 },
    Dll { token: u64, op: DllOp },        // install, update, batch update, restore
    Save { token: u64, write: Write },    // one file: a map of key -> Set(value) | Remove, plus the stored values it replaces
    CheckRelease,
    AutoUpdate,
    ResolveProton { app_id: u64 },
    Hardware { token: u64, op: HardwareOp }, // GPU reset, governor or SMT, one-shot helper (decision 44)
    Preview { request: u64, app_id: u64 },   // read-only dry-run summary (decision 44)
    Suspend,
    Quit,
}
```

Results carry the request id or token they answer, and update drops a result whose id is stale. The Go TUI already does this for rescans, installer results and update checks (verified: [parity-inventory.md](parity-inventory.md), TUI 12 and 13). The type names are a sketch (inferred).

### The event loop

The shape is mejl's code, not its docs: update takes raw events and resolves keys itself through the keymap (verified: [mejl-study.md](mejl-study.md), mejl-ui conflicts). One turn:

1. Block on the channel for one event, then take everything already waiting without blocking (verified: mejl's `recv_batch`, binary 4.1).
2. Coalesce runs of resize and drag events, keeping the first timestamp and the last event (verified: binary 8).
3. Feed each event to `update(&mut AppState, Event) -> Vec<Cmd>`. Worker results arrive as `Event::Msg`. A stop signal sets quit. SIGCONT clears the screen and feeds a synthetic resize, as mejl does (verified: binary 6).
4. `exec` each `Cmd`: send it to the metrics thread, the job pool or the write queue, or call the `Host` for suspend.
5. `after_turn`: turn ticking on or off from `state.wants_metrics()`, and set the earliest wake from `state.next_wakeup()`. Wakes are deduped by deadline (inferred, fixing mejl's duplicates).
6. Draw at most once, and only when the turn changed something visible. mejl draws after every batch, including a batch that only holds a Tick (verified: binary 4.2). update sets `state.dirty`. A resize or SIGCONT always sets it. A metrics sample sets it only when a shown value changes at display precision (inferred).
7. Draw into `CrosstermBackend<BufWriter<Stdout>>`. The synchronized-output markers (`CSI ? 2026 h` and `l`, only when the probe found support) go through the same writer with `terminal.backend_mut()`, before and after `terminal.draw`, and then the writer is flushed. Nothing else writes to stdout while the TUI runs. mejl writes its markers through separate `io::stdout()` handles (verified: binary 2.3), which would reorder bytes once a BufWriter sits in front. ratatui's `draw` flushes the backend before it returns: `try_draw` calls `apply_buffer_with_cursor`, which calls `backend.flush()` (verified: ratatui-core-0.1.2 `src/terminal/render.rs:189, 204, 308`, the version ratatui 0.30.2 uses, in the local cargo registry). So the begin marker and the frame leave in one write, and the end marker leaves in a second small one, which is still correct (inferred).
8. Record one `input_to_flush` sample per input event in the batch, from its read-time stamp to the second flush, the one after the end marker. mejl records one sample per frame from the oldest stamp (verified: binary 4.3). The [terminal-ux.md](terminal-ux.md) budget is per key, so spela samples per event (inferred).

Ctrl-Z: restore the terminal, raise SIGTSTP, re-enter on resume and repaint (verified: mejl's `suspend_for`, binary 6). mejl also pauses its input thread first, which is why it needs `EventStream` and the futures executor (verified: binary 6 and 8). spela has no external editor, and a stopped process stops its input thread with it, so a blocking `read` may be enough (inferred). This drops crossterm's `event-stream` feature and the `futures` crate. A new pty scenario decides it: spawn spela's blocking input thread, send a key, call `suspend_process()`, check WIFSTOPPED and cooked mode, send SIGCONT, then assert that the next key arrives once and no ghost keys do (inferred). It extends mejl's `suspend_process` scenario, which runs no input thread: only mejl's `input_pause` scenario spawns `InputThread` (verified: `~/git/mejl/crates/mejl/tests/terminal_pty.rs:439-459`). If the test fails, spela takes mejl's `EventStream` input thread as is. The test passes, so spela keeps the blocking read (as built in Phase 4: `a_blocking_read_survives_ctrl_z_and_resume` in `crates/spela/tests/terminal_pty.rs`).

Startup order: signals, TUI lock, config load and migration (errors reported before the terminal changes), capability probe with the cache, terminal enter, input and timer threads, first frame from the game cache, then the background scan (decision 24). It follows mejl's order (verified: binary 3) with the signal watcher moved first, and takes the lock before anything writes, as Go's `tui` does (verified: [parity-inventory.md](parity-inventory.md), CLI 3.14). The order is (inferred).

### Wake-up policy

- The metrics tick runs every 2 s, the Go interval (verified: `internal/tui/header.go:81`). The top bar shows live metrics on every destination ([terminal-ux.md](terminal-ux.md), frame; [mockups.html](mockups.html), every screen), so in practice the tick runs whenever a normal frame is up. It stops when NVML and the CPU reads are both unavailable, below 80x24 where only the resize prompt shows, and while suspended (inferred). It does not pause when the terminal reports focus lost, so the Monitor sparklines have no gaps (decision 42).
- A status message clears after 5 s through a one-off wake (verified: `internal/tui/messagebar.go:18`). A double-click window uses the clock, not a wake.
- No spinner, no clock and no animation. Progress is a word ([terminal-ux.md](terminal-ux.md), popups). The Go TUI's 100 ms flash tick (verified: `internal/tui/messagebar.go:20`) is not carried over (inferred).
- Nothing else wakes the UI. Scans, downloads and saves report through the channel.
- A tick whose sample shows nothing new is polled but not drawn. This is the difference from mejl, whose default screen wakes, updates and redraws every second at rest (verified: [mejl-study.md](mejl-study.md), binary 4.4).

### Wrapper process

`spela %command%` runs the game. Known subcommands parse first, and anything else that is a path or a program on PATH is a launch (decision 21). The full sequence is in [Launch and privilege](#launch-and-privilege). Its shape:

```
 main thread                                   signal thread (installed first)
 -----------                                   -------------------------------
 route argv: subcommands first                 SIGINT, SIGTERM, SIGHUP:
 refuse to run as root (decision 35)             before the child starts: set cancelled
 load config, game cache and profiles;           after it starts: forward to the child
   detect the game; build the plan
 if the plan has VRR or privileged changes:
   try the launch lock (flock, no wait)
   held? warn, drop VRR and hardware (decision 33)
 apply KDE VRR in the user session; register its restore
 register the hardware restore; start the helper:
   pkexec /usr/bin/spela apply-profile --serve, one prompt (decision 30)
   send the batch, read one report line per step
 cancelled?  -> run restores, release the lock, exit 128 + n
 spawn the game, wait for it
 run restores in reverse order: "restore" to the helper, then VRR
 release the launch lock
 return ExitCode: the game's status, or 128 + its signal
```

- Signal handling starts before `pkexec` and VRR. Go registers it only after both, so a signal during the polkit prompt kills spela with no cleanup, and Go forwards only the first signal and never SIGHUP (verified: [parity-inventory.md](parity-inventory.md), CLI 6.7).
- The TUI's quit-on-signal policy does not apply here, because exiting early would skip the restore (inferred, answering the open question in [mejl-study.md](mejl-study.md), binary open questions; also binary 17).
- The launch lock is held from the first change until the last restore ends (see [Launch lock](#launch-lock)).
- `main` returns an `ExitCode` instead of calling `std::process::exit`, so destructors run and the log writer's guard flushes the restore lines to the file (inferred).
- Logs go to the rolling file under `XDG_STATE_HOME` and warnings to stderr, which Steam captures (decision 12).

### Root helper and CLI

- The hidden `apply-profile` command runs as root under `pkexec`, in one of two modes (see [Protocol](#protocol)):
  - `apply-profile --serve` is the long-lived helper for a launch (decision 30). It reads one batch from stdin, reads each prior value as root, applies, reports each step on stdout, and then waits. A restore request, or EOF on stdin because the wrapper died, makes it restore what it applied and exit.
  - `apply-profile --once <request>` serves the live hardware commands in the CLI and the TUI. It applies, reports and exits, with nothing to restore, because a live command is meant to last, as in Go (verified: [parity-inventory.md](parity-inventory.md), CLI 3.9 and 3.10).
  - Anything else, including Go's `--reset` and flag forms, prints a clear error that the Go helper protocol is not supported, and exits 2 (decision 34).
- The helper does no per-user setup: no XDG paths, no migration, no log file and no TUI hooks. It runs with root's environment, so anything it wrote would land in the wrong home or be owned by root. It reports each step's result to its parent as JSON lines, and the parent logs them (inferred).
- The CLI runs each command on the main thread. A DLL download blocks with progress on stderr, which is fine with no UI to keep alive (inferred). The live `gpu` and `cpu` commands run the one-shot helper with one `pkexec`. They leave stdin on the terminal, so pkexec can fall back to its text agent there when no graphical agent runs (unverified, part of the user's live checks).
- NVML is opened lazily in every shape: by the metrics thread in the TUI, on first use in the CLI, and inside the helper for setters. Go opens it in every invocation, including wrapper mode and `config show` (verified: [parity-inventory.md](parity-inventory.md), CLI 2.1). There is no `nvidia-smi` fallback (decision 58).

## Cargo workspace

Four crates. mejl has eight (verified: [mejl-study.md](mejl-study.md), binary 13.1), and most of its splits serve parts spela rejects: the JMAP client, the fake server, the store, the sync engine and the renderer.

| Crate | Role |
|---|---|
| `spela-core` | The domain and its seams: profiles, config, migration, games, Steam, DLLs, the denylist, alerts, launch planning, the backend traits and the fake |
| `spela-ui` | TUI state, update and view. No I/O |
| `spela` | The binary: CLI, wrapper, root helper, terminal shell, threads and the real backends |
| `dlss-updater` | The CI tool that keeps the DLL manifest current, ported from Go (decision 14). `publish = false` and not in `default-members` |

Dependency direction. `spela-core` depends on no internal crate.

```
 spela ---------> spela-ui ---------> spela-core
   |                                       ^
   +---------------------------------------+
 dlss-updater ----------------------------->
```

| Crate | Owns | External dependencies |
|---|---|---|
| `spela-core` | The field table (25 fields) and its vocabulary; profiles and All games inheritance; config schema and validation; reading the Go YAML files and migrating them; TOML read and comment-keeping write; the game cache; Steam library and Proton scanning over filesystem roots; VDF; PE version reading; version compare; manifest, DLL operations and backups; the denylist; the release check; the alert evaluator (`alerts`); launch planning (environment, dry-run summary, hardware request, restore plan); atomic writes and flock helpers; `Paths`, roots, `Env` and `Clock`; the backend traits; the fake | serde, serde_json, toml, toml_edit, serde-saphyr (read only), keyvalues-parser, object, sha2, http, jiff, regex-lite, walkdir, tempfile, etcetera, thiserror, tracing |
| `spela-ui` | `AppState`, `update`, `render`, keymap, layout, hit map, theme and glyphs, text width, picker and fuzzy matcher, `LineInput`, perf ring | spela-core, ratatui, crossterm (event types), unicode-width, unicode-segmentation, thiserror |
| `spela` | `main`; clap commands and CLI output; wrapper mode; the root helper; terminal mode manager, probe, caps cache, quirks, signals, input thread, timer, loop and `Host`; metrics thread, job pool and write queue; logging; the real backends (nvml-wrapper, ureq, sysfs writes, `pkexec`, `kscreen-doctor`) | the internal crates plus nvml-wrapper, ureq, clap, clap_complete, anstyle, rustix, libc, signal-hook, wait-timeout, tracing-subscriber, tracing-appender, anyhow. Colour is anstyle styles with detection from the env snapshot, and the router's PATH lookup is a few lines over the snapshot's `PATH`, so Phase 1 takes neither anstream nor which (as built) |
| `dlss-updater` | Scraping, the release step, manifest editing | spela-core, ureq, regex-lite, serde_json |

Why each split pays:

- `spela-ui` stands alone so its no-I/O rule is a fact a tool can check. mejl-ui has no file, network or process crates, and its only environment read is `NO_COLOR` (verified for the dependency list, inferred for calls: [mejl-study.md](mejl-study.md), mejl-ui size). mejl-ui's `theme::no_color_from_env` reads `NO_COLOR` itself (verified: `~/git/mejl/crates/mejl-ui/src/theme.rs:684-686`). spela-ui gets `NO_COLOR` from the `Env` snapshot instead and has no environment reads (inferred). spela's check: `cargo tree -p spela-ui` lists neither nvml-wrapper nor ureq, and a CI grep finds no `std::fs`, `std::process` or `std::net` in its source (inferred). It depends on `spela-core`, which does file I/O, so the grep carries the rule the tree cannot. The TestBackend tests then need no system at all.
- `spela-core` stands alone because two binaries need the same manifest types and version compare: `spela` and `dlss-updater` (decision 14). Compile time is weak evidence for any split: the starter set rebuilds in 0.3 s in debug (verified: [rust-crates.md](rust-crates.md), compile time).
- `dlss-updater` stands alone because it is CI infrastructure, not part of the shipped binary (inferred in [parity-inventory.md](parity-inventory.md), DLL 7), and `cargo install` should not install it.

What is not split, and why (inferred):

- No `spela-system` crate, which [rust-crates.md](rust-crates.md) proposed with NVML, sysfs, pkexec, kscreen-doctor, IPC and the launcher. Only the binary uses the real backends, and the IPC part is gone (decision 10).
- No fake crate. The fake lives in `spela-core::fake` as plain code with no test-only dependencies, because the release binary ships it for `--demo`. mejl's fake pulls axum into release builds (verified: [mejl-study.md](mejl-study.md), binary conflicts).
- Parsers of external output (`kscreen-doctor -o`, `nvidia-smi` CSV, sysfs and procfs files) live in `spela-core` as pure functions. Only the process calls live in the binary.

Changes to the starter set in [rust-crates.md](rust-crates.md):

| Change | Why |
|---|---|
| Add etcetera 0.11.0 | Spec-strict XDG (decision 23). rust-crates.md left it out because it changes Go's semantics |
| tracing-appender 0.2.5 moves from Later to Pick | The rolling log file (decision 12) |
| Add toml 1.1.8 and toml_edit 0.25.17 | TOML config and profiles (decision 6). Versions from mejl's workspace (verified: mejl `Cargo.toml:56-58`). Both declare rust-version 1.85 (verified: their `Cargo.toml` line 14 in the local cargo registry). Not built with spela's starter set (unverified) |
| Add unicode-width 0.2 and unicode-segmentation 1.13 | mejl's `text.rs` (verified: mejl `Cargo.toml:27-28`) |
| Remove memmap2 | Its only use was the overlay IPC writer (decision 10) |
| Keep serde-saphyr 2.0.0, pinned exactly, for reading only | Migration from the Go YAML. Nothing is written as YAML |
| crossterm 0.29.0 without `event-stream`, and no futures | The blocking input thread. Revisit if the suspend pty test fails |
| Keep signal-hook 0.3 and rustix | crossterm already carries both (verified: [rust-crates.md](rust-crates.md), processes) |
| The workspace version starts above 0.6.2, for example 0.7.0 | Release tags must be newer than the Go ones (verified: [parity-inventory.md](parity-inventory.md), DLL 1.2). The starter sets `0.1.0` (inferred fix) |
| `[workspace.package] license = "MIT"` | Decision 15. The starter already sets it (verified: [rust-crates.md](rust-crates.md), starter `Cargo.toml`). MPL-2.0 crates are allowed only when unmodified, and no GPL or LGPL crates come in (verified traps: [rust-crates.md](rust-crates.md), version and license traps) |
| Add libc 0.2.191 to `spela`, as a normal and a dev dependency | The adopted mejl terminal code and pty tests call libc for `poll` on `/dev/tty`, `raise(SIGTSTP)`, `kill`, `openpty`, `setsid` and `waitpid` (verified: mejl uses it in `terminal/mod.rs`, `terminal/probe.rs`, `terminal/signals.rs`, `lock.rs` and `tests/terminal_pty.rs`, and pins `libc = "0.2"` at `~/git/mejl/Cargo.toml:75`). It is likely in the tree already through crossterm and signal-hook (inferred). The alternative is rustix's `termios`, `event` and `pty` features |
| vt100 0.18.1 and proptest 1.12.0 move from Later to Pick, as dev dependencies | The pty screen reads and the property tests in [Testing strategy](#testing-strategy) use them (versions verified: [rust-crates.md](rust-crates.md), testing) |
| Add http 1 to `spela-core` | Manifest URL validation uses `http::Uri`, the type ureq re-exports as `ureq::http::Uri` (verified pick: [rust-crates.md](rust-crates.md), what spela needs). The `HttpRequest` types use it too. So `spela-core` and `spela-ui` stay free of ureq, and the `cargo tree -p spela-ui` check holds (inferred) |
| Keep sha2 0.11.1, with a small hex helper in `spela-core` | The helper encodes a digest and checks a 32-byte hex string. Never `format!("{:x}")` on a digest: 0.11's output has no `LowerHex` (verified trap: [rust-crates.md](rust-crates.md), version and license traps). The alternative is pinning 0.10.9 to match mejl (inferred choice) |
| Release profile keeps `panic = "unwind"` | An abort would skip the restore. rust-crates.md lists `panic = "abort"` only as a size option (verified: [rust-crates.md](rust-crates.md), summary) (inferred) |

## Backends and fakes

Each seam has the real implementation and a fake. The fake is the second implementation that justifies the trait (inferred, after mejl's `Transport`, verified: [mejl-study.md](mejl-study.md), binary 14). Where the real backend is only files, the seam is a root path, not a trait: a test points the root at a temp dir (inferred).

| Seam | Real | Fake | Kind |
|---|---|---|---|
| GPU reads | nvml-wrapper on device 0. It loads `libnvidia-ml.so.1` at runtime, so a missing driver is an error value, not a crash (verified: [rust-crates.md](rust-crates.md), NVML) | Scripted metrics. For the demo, a generator that walks through the alert states | trait `Gpu` |
| GPU control, root side only | nvml-wrapper setters inside the root helper | Records every call, fails at a chosen step | trait `GpuControl` |
| CPU sysfs | Files under `/sys`. The reads (governor, available governors, SMT) also sit behind trait `Cpu` as `SysfsCpu`, so `--demo` and the binary tests read the fake (as built in Phase 2). `Cpu::info` adds the other `cpu info` lines over `SystemRoots` (`/sys` and `/proc`); the binary's `SystemCpu` adds the SCX line (as built in Phase 6) | A temp-dir tree for `SysfsCpu`; `FakeCpu` for the trait | root path, plus trait `Cpu` for reads |
| procfs | Files under `SystemRoots.proc` (`/proc`): running games, cpuinfo, meminfo, loadavg | A temp-dir tree with fake pid folders | root path |
| Display (KDE VRR) | `kscreen-doctor` with a 10 s timeout and `LC_ALL=C` (verified: [parity-inventory.md](parity-inventory.md), CLI 6.5) | Scripted outputs and replies | trait `Display` |
| Privilege | `pkexec` running this binary's `apply-profile --serve` for a launch (decision 30), or `apply-profile --once` for a live command | Runs the helper loop in process over in-memory pipes, against the `GpuControl` fake and the sysfs tree. Can be dismissed, find no polkit agent, fail at step N, or lose its parent at step N. It runs the real `--serve` and `--once` loops from `spela-core::helper` on a thread over `std::io::pipe` pairs, with the real parent-side client (as built in Phase 6) | trait `Privilege` |
| Steam | Files under the home root: `.steam/steam`, `.local/share/Steam`, the Flatpak path (verified: [parity-inventory.md](parity-inventory.md), domain 7) | A fixture library in a temp dir ([../AGENTS.md](../AGENTS.md)) | root path |
| HTTP | ureq, blocking, on a worker | A map from URL to a response or a fault | trait `Http` |
| Clock | `SystemClock` | `ManualClock` with `set` and `advance` | trait `Clock` (mejl's) |
| Filesystem roots | `Paths` from etcetera | `Paths::under(tempdir)` | value |
| Environment | `Env::from_process`, read once | `Env::from_pairs` | value. `std::env::set_var` is unsafe in edition 2024 (verified: [rust-crates.md](rust-crates.md), traps) |

The game process itself is spawned with `std::process::Command` and has no trait. Wrapper tests launch a small script from a temp dir instead of a game (inferred). There is no general command-runner trait until a second user of one appears.

```rust
/// GPU 0 only (decision 25). Every read can fail; nothing panics.
pub trait Gpu: Send {
    fn info(&mut self) -> Result<GpuInfo, GpuError>;
    fn metrics(&mut self) -> Result<GpuMetrics, GpuError>;
}

/// A value read before a change. Unknown means "do not restore".
pub enum Prior<T> { Known(T), Unknown(String) }

/// Used only inside the root helper.
pub trait GpuControl {
    fn read_prior(&mut self) -> GpuPrior; // clock and memory VF offsets, power limit in mW, each fan's policy
    fn power_limit_range(&mut self) -> Result<(u32, u32), GpuError>;  // checks a power limit (as built)
    fn set_clock_offset(&mut self, mhz: i32) -> Result<(), GpuError>;  // set_gpc_clock_vf_offset (decision 32)
    fn set_memory_offset(&mut self, mhz: i32) -> Result<(), GpuError>; // set_mem_clock_vf_offset (decision 32)
    fn reset_locked_clocks(&mut self) -> Result<(), GpuError>;         // for gpu reset
    fn set_power_limit_mw(&mut self, milliwatts: u32) -> Result<(), GpuError>;
    fn set_fan(&mut self, fan: u32, setting: FanSetting) -> Result<(), GpuError>; // a percent or automatic, per fan
}

pub trait Display {
    fn outputs(&mut self) -> Result<Vec<Output>, DisplayError>;
    fn set_vrr(&mut self, uuid: &str, policy: VrrPolicy) -> Result<(), DisplayError>;
}

/// One pkexec per call. Each report has one entry per step.
pub trait Privilege: Send {
    /// A launch: starts `apply-profile --serve`, applies, and keeps the helper waiting (decision 30).
    fn start(&mut self, request: &HardwareRequest) -> Result<Box<dyn PrivilegedSession>, PrivilegeError>;
    /// A live command: `apply-profile --once`, nothing to restore.
    fn apply_once(&mut self, request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError>;
}

/// The waiting helper. Dropping it closes the helper's stdin, which also restores.
pub trait PrivilegedSession: Send {
    fn report(&self) -> &ApplyReport; // what the apply did, one entry per step
    fn restore(self: Box<Self>) -> Result<RestoreReport, PrivilegeError>;
}

pub trait Http: Send + Sync {
    fn get(
        &self,
        request: &HttpRequest,                 // URL, Accept, timeouts, body cap
        body: &mut dyn std::io::Write,
        progress: &mut dyn FnMut(u64, Option<u64>),
    ) -> Result<HttpResponse, HttpError>;
}
```

Notes on the shapes (inferred unless marked):

- `set_clock_offset` and `set_memory_offset` are real NVML offsets through nvml-wrapper's `set_gpc_clock_vf_offset` and `set_mem_clock_vf_offset` (verified: [rust-crates.md](rust-crates.md), NVML). The helper wires them, with `read_prior` and restore, and they are tested on fakes only (decision 32). They are live-unverified until the user's root test (decision 9; see [GPU offsets](#gpu-offsets)). There is no `set_power_mizer_mode` (decision 10).
- `read_prior` runs as root inside the helper, right before the change, so the restore gets real values. Go reads them unprivileged beforehand and ignores read errors (verified: [parity-inventory.md](parity-inventory.md), domain 4).
- The power limit prior is kept in mW, exactly as NVML reports it, and the restore writes that value back. A prior rounded to whole watts could land above the card's maximum, and NVML would refuse the restore. The report still shows whole watts.
- The fan prior is read per fan, for every fan from `num_fans`, or fan 0 alone when the count cannot be read, as Go did. A fan whose mode cannot be read restores to automatic. The fan step sets each fan in turn, and when one fails it puts the fans it already set back to their own priors, so a failed step changed nothing, as the governor step does. The restore tries every fan and joins the errors. The report's `prior` shows fan 0's mode.
- `PrivilegedSession` restores on an explicit `restore`, and also when it is dropped, because the drop closes the helper's stdin and the helper restores on EOF (decision 30). A panic or an early return in the wrapper therefore still restores (inferred).
- `ApplyReport` lists each step as applied with its prior value, failed with its error, or skipped. The restore plan is built only from applied steps whose prior value is known (principle 4).
- `gpu reset` sends `HardwareRequest::gpu_reset()`: a `LockedClocks` step first in apply order, then both VF offsets at 0. NVML has setters and a reset for locked clocks but no getter (inferred from the NVML API; the root test checks it), so `read_prior` does not cover them. The reset step reads no prior value, and it is reported as applied with nothing to restore, because only the one-shot helper sends it and nothing ever restores it (as built in Phase 1). Since nothing restores any `--once` step, an offset whose prior cannot be read is still set, and it is reported the same way (as built; see [Protocol](#protocol)).
- `PrivilegeError` keeps Go's split: dismissed when `pkexec` exits 126 or reports that authentication was cancelled or dismissed, unavailable when `pkexec` is not on PATH, otherwise the helper's error text (verified: [parity-inventory.md](parity-inventory.md), CLI 7.1).
- The real backend is `crates/spela/src/http.rs` (as built in Phase 5). It reads the proxy variables and `NO_PROXY` from the `Env` snapshot, in ureq's order (`ALL_PROXY`, `HTTPS_PROXY`, `HTTP_PROXY`, each upper case first), and turns ureq's own process lookup off, so the snapshot stays the only environment read and a test can give a proxy without `set_var`. Its tests run only against loopback listeners: the User-Agent and Accept headers, a 404, a redirect, both body caps, the header and total timeouts, a proxy that carries the request, and `NO_PROXY`.
- `HttpRequest` sets an explicit `User-Agent: spela/<version>` (decision 41), because GitHub's API may reject a request without one (unverified: [parity-inventory.md](parity-inventory.md), DLL 5). The real backend keeps ureq's default proxy support, so `HTTPS_PROXY` and the other proxy variables are honoured (decision 41). Timeouts follow Go: 30 s to the response header and 120 s in total for the manifest and DLLs, 5 s and a 1 MiB cap for the tags (verified: [parity-inventory.md](parity-inventory.md), DLL 2.4 and 5). The manifest gets a body cap too, which Go lacks (verified: [parity-inventory.md](parity-inventory.md), DLL conflicts).

The fake lives in `spela-core::fake`. It implements every trait above and builds a fixture tree in a temp dir: a Steam library with fictional games and app ids (decision 43), a sysfs and procfs tree, a manifest, and DLL payloads whose SHA-256 matches the manifest (inferred).

- Tests use `ManualClock` and fixed data, so snapshots stay stable. Faults are switches: NVML missing, a scan error, a download timeout, a checksum mismatch, a dismissed prompt, no polkit agent, a failure at apply step N, a helper whose parent goes away at step N (inferred, after mejl-fake's quirk switches, verified: [mejl-study.md](mejl-study.md), binary 14). The privilege switches are built in Phase 1 (`PrivilegeFault`): no polkit agent gives the helper error with pkexec's text, and a lost parent restores the steps applied so far and returns no session.
- A hidden backend selector, `SPELA_BACKEND=fake`, is read from the env snapshot (inferred). It swaps the hardware, privilege, display and HTTP backends for the fake and changes nothing else: the `Paths` the test injected stay. Every binary test sets it in its sandbox env, including the CLI, migration and wrapper tests that run against fixture XDG trees, Go fixtures or a temp launch script (see [Hard safety rules as tests](#hard-safety-rules-as-tests)).
- `--demo` is the user-facing mode built on top of it. It is a global flag (inferred). It runs the TUI and every CLI command on the fake with `SystemClock`, under `Paths::under` a private temp dir that is removed on quit, on the TUI's second stop signal and on a fatal panic. A kill that cannot be caught, such as SIGKILL from `termctrl stop`, leaves it, so each new demo removes the `spela-demo-*` dirs whose `.spela-demo-pid` names a process that is gone (inferred). It copies only `<config>/config.toml` in, so theme and mouse settings carry over and the real file is never written, as mejl's demo copies its config (verified: [mejl-study.md](mejl-study.md), binary 9.4). It never reads Go YAML and never runs the migration (inferred). Under `--demo`, wrapper mode and any game launch are refused (inferred).
- The fake `Http` serves the unit tests and the demo. The real ureq backend is tested separately against a loopback `std::net::TcpListener` thread, which was enough in the probe (verified: [rust-crates.md](rust-crates.md), testing). Neither ships a server in the release binary.

## Data and files

The rewrite keeps three kinds of file apart, the way mejl keeps its cache apart from user intent (verified: `~/git/mejl/docs/architecture.md:33, 180-186`). User intent covers config, profiles, lists, DLL backups and the migration record. These files are durable, and losing one loses something the user chose. Rebuildable data covers the game cache, the manifest cache, the DLL cache, the update check and the terminal capability cache. Deleting any of them costs a rescan, a download or a probe, never a setting. Ephemeral files are the locks and the logs (inferred).

Three rules hold for every file (inferred):

- Readers never lock. Every writer replaces files atomically, so a reader sees the old file or the new one, never half of one. Only read, modify and write cycles take a lock.
- The root helper never resolves an XDG path and never writes a user file or a log (see [Root helper and CLI](#root-helper-and-cli)).
- Tests and `spela --demo` build every path from `Paths::under(base)` with a temp dir (tests set it themselves, `--demo` makes its own), as mejl does (verified: `~/git/mejl/crates/mejl/src/paths.rs:79-90`). No test touches the real XDG dirs ([../AGENTS.md](../AGENTS.md) hard rules).

### Base directories

Paths are spec-strict XDG, as etcetera resolves them (decision 23). spela uses etcetera 0.11.0's XDG app strategy with the app name `spela`. The strategy joins the lowercased app name onto each base, so the dirs are `~/.config/spela`, `~/.local/share/spela`, `~/.cache/spela` and `~/.local/state/spela` when the variables are unset (verified: etcetera-0.11.0 `src/app_strategy.rs:59-61` and `src/app_strategy/xdg.rs:197-221` in the local cargo registry). These are the same folders the Go version uses for config, data and cache (verified: [parity-inventory.md](parity-inventory.md), CLI 10). So the migration finds the Go files where Go left them.

Three things change from Go:

- etcetera ignores a relative `XDG_*` value and returns no runtime dir when `XDG_RUNTIME_DIR` is unset. Go accepted relative values and fell back to `/tmp/runtime-$USER/spela` (verified: [rust-crates.md](rust-crates.md), XDG paths). spela follows the spec and prints no warning for a relative value (decision 43).
- Go has no state dir (verified: [parity-inventory.md](parity-inventory.md), CLI 10). The rewrite uses `<state>` for logs and locks.
- The runtime dir is not used at all (inferred). Go keeps its PID file, its games lock and the overlay IPC file there (verified: [parity-inventory.md](parity-inventory.md), CLI 10). The IPC file is gone (decision 10). The locks move to `<state>/locks/`, because every process must find the same lock file. With a runtime dir that may be missing, a process started with `XDG_RUNTIME_DIR` and one started without it would lock different files and no longer exclude each other. mejl also keeps its flock next to its data, not in the runtime dir (verified: [mejl-study.md](mejl-study.md), binary 11). flock leaves no stale lock, so lock files left behind are harmless.

Below, `<config>`, `<data>`, `<cache>` and `<state>` mean those dirs with `spela` already joined. Directories keep Go's modes: config and data 0755 (verified: [parity-inventory.md](parity-inventory.md), CLI 10), and cache 0755 (inferred). `<state>/locks` and `<state>/log` are 0700, and lock files are 0600 as Go's PID file is (verified mode: [parity-inventory.md](parity-inventory.md), CLI 8; the rest inferred).

### Files

| File | Path | Format | Owner | Durability | Write | Lock |
|---|---|---|---|---|---|---|
| Config | `<config>/config.toml` | TOML | The user, and spela through `config set` and Settings | User intent | toml_edit in place, atomic, fsync | `config.lock` |
| All games profile | `<config>/profiles/all-games.toml` | TOML | The user, and spela through the CLI and TUI | User intent | Same | `config.lock` |
| Game profiles | `<config>/profiles/<appid>.toml` | TOML | Same | User intent | Same | `config.lock` |
| Deny overrides | `<config>/lists/overrides.toml` | TOML | The user, and spela through `denylist` | User intent | Same | `config.lock` |
| User deny list (optional) | `<config>/lists/denylist.toml` | TOML | The user, and spela once when it migrates Go's `lists/denylist.yaml` | User intent | Written once by the migration, then never by spela | none |
| Migration record | `<data>/migration.toml` | TOML | spela | User intent: losing it can bring back deleted profiles | Whole file, atomic, fsync | `config.lock` |
| Backup index | `<data>/backups/<appid>/backup.json` | JSON, Go's schema plus new top-level keys Go ignores (decisions 37 and 50) | The DLL worker | Safety critical | Atomic, fsync | `games.lock` |
| Backup files | `<backup_path or data/backups>/<appid>/<sha256>-<basename>`, and Go's legacy `<basename>` | Raw DLL | The DLL worker | Safety critical | Copy, fsync before the game's DLL is touched | `games.lock` |
| Game cache | `<cache>/games.json` (decision 43) | JSON | The scan worker and the DLL worker | Rebuildable | Atomic, no fsync | `games.lock` |
| Manifest cache | `<cache>/manifest-v1.json` (decision 53) | JSON, Go's shape | The manifest worker | Rebuildable | Atomic, no fsync | none |
| DLL cache | `<dll_cache_path or cache/dlls>/<manifest key>/<version>.dll` | Raw DLL | The DLL worker | Rebuildable, checked by SHA-256 on use | Streamed to a temp file, hash checked, then renamed | none: the rename is atomic, and the download runs outside `games.lock` |
| Update check | `<cache>/release-check.json` (decision 53) | JSON, Go's shape | The release worker | Rebuildable | Atomic, no fsync | none |
| Capability cache | `<cache>/caps.toml` | TOML, mejl's format | The terminal module | Rebuildable | Atomic, no fsync | none |
| Logs | `<state>/log/spela.log.<date>` | Text | The logger | Disposable | Append | none |
| TUI lock | `<state>/locks/tui.lock` | PID text | The TUI | Ephemeral | flock | itself |
| Launch lock | `<state>/locks/launch.lock` | PID text | The wrapper and the live hardware commands in the CLI and the TUI | Ephemeral | flock | itself |
| Store locks | `<state>/locks/config.lock`, `<state>/locks/games.lock` | Empty | The store | Ephemeral | flock | itself |

The default shader cache folder `<cache>/nvidia` is written by the NVIDIA driver, not by spela (verified: [parity-inventory.md](parity-inventory.md), domain 1, `gpu.shader_cache_path`). The Go lock and IPC files `spela.pid`, `games.lock`, `profiles/.mutation.lock` and `overlay-<appid>.dat` are not reused (inferred, from decision 13).

### Writing a file

One helper writes every file (inferred). It follows Go's sequence of a temp file in the same folder, chmod, write, close and rename (verified: [parity-inventory.md](parity-inventory.md), DLL 2.8), through tempfile's `NamedTempFile::new_in` and `persist` (verified: [rust-crates.md](rust-crates.md), atomic file writes). Then it adds three things:

- For user intent and backups, it calls `sync_all` on the file before the rename and fsyncs the folder after it. Go never fsyncs (verified: [rust-crates.md](rust-crates.md), atomic file writes). mejl calls `sync_all` on its config (verified: [mejl-study.md](mejl-study.md), binary 10.3), and writes its caps cache without fsync (verified: binary 7.3). Caches skip both syncs.
- A replaced file keeps its old mode, as mejl's writer does (verified: [mejl-study.md](mejl-study.md), binary 10.3). A new file gets 0644, as Go uses (verified: [parity-inventory.md](parity-inventory.md), domain 3).
- If the target is a symlink, as it is in a dotfiles setup, the helper writes next to the link's target and replaces the target, so the link survives (inferred). Go's rename replaces the link itself (inferred from the write path in [parity-inventory.md](parity-inventory.md), domain 3).

Go's `config.yaml` and `lists/overrides.yaml` are written with a plain `os.WriteFile`, and a crash can truncate them (verified: [rust-crates.md](rust-crates.md), conflicts). The rewrite has no such exception.

The TOML user files, profiles included, go through toml_edit, so a write changes only the keys it means to change and keeps the user's comments, order and unknown keys. Before the replace, the new text is parsed again, and a text that does not parse is never written. mejl does both (verified: [mejl-study.md](mejl-study.md), binary 10.3). Reading uses the toml crate into typed structs. Go re-marshals a whole profile on every save (verified: [rust-crates.md](rust-crates.md), YAML), so keeping comments is new (inferred).

### Locks

All four locks are advisory flock, taken through std `File::lock` and `try_lock`, which are stable since Rust 1.89 (verified: [rust-crates.md](rust-crates.md), file locking). The kernel drops a lock when its process exits, so a stale lock cannot exist, and PID reuse cannot fake one (verified: [mejl-study.md](mejl-study.md), binary 11). A lock file stays in place after release, as mejl's does (verified: `~/git/mejl/crates/mejl/src/lock.rs:128-173`). Unlinking it would let two processes hold locks on different inodes at once (inferred). A tool that needs to know whether spela runs probes the lock with a non-blocking flock.

- The TUI lock replaces Go's PID file `spela.pid` (verified: [parity-inventory.md](parity-inventory.md), CLI 8). The TUI takes it without blocking and writes its PID into the file, only so that a second TUI can print `spela is already running (PID: N)` (decision 29; mejl writes its PID for the same reason, verified: [mejl-study.md](mejl-study.md), binary 11). A held lock is retried for 100 ms, as mejl does for descriptors inherited across a fork (verified: same). The descriptor is close-on-exec, so a game started from the TUI never inherits it (inferred). Only `spela tui` takes it, because `spela gui` is gone (decision 26). A held lock exits with code 5 ([terminal-ux.md](terminal-ux.md), CLI output).
- The launch lock covers applying and restoring hardware and display changes (decision 29). Its scope is set in [Launch lock](#launch-lock).
- `config.lock` serializes every write to `config.toml`, the profiles, the lists and the migration record. Go has a profile lock and no config or list lock (verified: [parity-inventory.md](parity-inventory.md), domain 3; [rust-crates.md](rust-crates.md), conflicts). One lock is enough, because these writes are small and rare (inferred).
- `games.lock` serializes the game cache, DLL replacement and backups. Go holds its games lock through a download of up to 2 minutes (verified: [parity-inventory.md](parity-inventory.md), DLL 2.6). spela downloads into the checksum-verified DLL cache first and takes `games.lock` only for the backup, replace, rescan and save (inferred). Keeping it apart from `config.lock` means a DLL operation never delays a profile save. No code path holds both locks (inferred).

### config.toml

The keys keep their Go names, so `spela config set check_updates false` still works (verified key list: [parity-inventory.md](parity-inventory.md), CLI 11). Every key can be set from the CLI, because the CLI and the TUI have the same features (decision 20). Go shows only three keys to the CLI (verified: same). The new TUI settings, which are the theme, the mouse and the ASCII glyphs, go in a `[ui]` table as in mejl ([terminal-ux.md](terminal-ux.md), theme and mouse; verified for mejl: [mejl-study.md](mejl-study.md), binary 10.3). Their labels and descriptions are settled in [terminal-ux.md](terminal-ux.md#settled-questions). A missing key means its default, as in Go (verified: `internal/config/config.go:76-79`).

Every key with its default:

```toml
version = 1

log_level = "info"                # debug, info, warn, error
check_updates = true
rescan_on_startup = true          # the TUI's background scan at start (decision 24)
auto_update_dlls = false
confirm_destructive = true

steam_path = ""
additional_library_paths = []
dll_cache_path = ""               # empty means <cache>/dlls
backup_path = ""                  # empty means <data>/backups
shader_cache = "/home/me/.cache/spela/nvidia"   # computed default

dll_manifest_url = ""             # empty means the built-in URL (decision 14)
auto_refresh_manifest = true
manifest_refresh_hours = 24

[ui]
theme = "terminal"                # terminal, neon or mono. NO_COLOR forces mono
mouse = true
ascii_glyphs = false
```

The defaults match Go's (verified: `internal/config/config.go:48-63`). The rewrite validates on load and reports every error with its line and column, as mejl does (verified: [mejl-study.md](mejl-study.md), binary 10.2). Go validates nothing at load (verified: [parity-inventory.md](parity-inventory.md), CLI 11). Unknown keys give a warning with a did-you-mean, not an error, and stay in the file (verified for mejl: [mejl-study.md](mejl-study.md), binary 10.2). A config that does not load exits with code 3 ([terminal-ux.md](terminal-ux.md), CLI output). The value rules and error texts come from the Go option table (verified: [parity-inventory.md](parity-inventory.md), CLI 11).

`confirm_destructive = false` skips only the DLL update confirmation, as in Go (verified: [parity-inventory.md](parity-inventory.md), TUI 10). Install, restore and whole-page resets always confirm (decision 4; [terminal-ux.md](terminal-ux.md), principle 6). So do the TUI's hardware actions, GPU reset, CPU governor and SMT (decisions 4 and 44).

`config set additional_library_paths` takes colon-separated paths or a JSON list, as Go's option parser does (verified: [parity-inventory.md](parity-inventory.md), CLI 11). The `[ui]` keys are set as `ui.theme`, `ui.mouse` and `ui.ascii_glyphs` (inferred).

`rescan_on_startup` keeps its name but narrows. It now switches the TUI's one background scan at start. The CLI rescans only on an empty game cache or `spela scan`, whatever it says (decision 24). In Go, `list` and `show` also rescan when it is on (verified: [parity-inventory.md](parity-inventory.md), CLI 3.2 and 3.3).

`version` is new, and every TOML file spela writes carries `version = 1` (decision 43). A missing `version` reads as 1. A file with a higher version is read only, with a warning (inferred). The Go clock offset shows why this helps: the meaning of a stored field is about to change (see [Migration from the Go files](#migration-from-the-go-files)).

### Profiles and inheritance

A profile has 25 fields after decision 10 drops ten of Go's 35. The ten are the eight overlay fields (`enabled`, `position`, the five `show_*` fields and `toggle_key`), `gpu.power_mizer` and `cpu.affinity` (verified field list: [parity-inventory.md](parity-inventory.md), domain 1). The 25 are four Proton fields, eleven DLSS fields, eight GPU fields and two CPU fields (inferred count from the same table and `internal/profile/fields.go:61-101`).

A profile file stores only the values its scope sets itself. A key that is present is an own value, and a key that is absent follows. This replaces Go's `overrides` map, where a stored leaf counts only when its key is marked true, and an explicit zero is carried by the marker alone (verified: [parity-inventory.md](parity-inventory.md), domain 2 and 3). In TOML an explicit zero is just written, as `threaded_optimization = false` or `clock_offset = 0`. TOML has no null, so `cpu.smt` is `true`, `false`, `"unchanged"` or absent. Absent follows. `"unchanged"` is an own "none": the scope leaves SMT as it is at launch, even when the scope above sets it, as a Go game profile with `smt` marked and no value did (decision 65; verified Go behaviour: the `profiles/explicit-zeros` oracle). The shared words show it as `unchanged`, and a "none" that nothing sets still reads `none`.

Sections and keys are written in descriptor order: proton, dlss, gpu, cpu (verified order: [parity-inventory.md](parity-inventory.md), domain conflicts, YAML byte contract). Go's YAML used struct order instead (verified: same). This matches the order the TUI and the docs show.

A game profile:

```toml
# Migrated from 1091500.yaml (Go spela) on 2026-10-26. The original is kept.
# Not carried over: overlay.enabled = true.
# Full record: ~/.local/share/spela/migration.toml
version = 1
name = "Cyberpunk 2077"

[proton]
enable_hdr = true

[dlss]
sr_mode = "quality"
sr_preset = "auto"
sr_override = true

[gpu]
power_limit = 250
threaded_optimization = false     # an own value of Off, not "follows"

[cpu]
smt = false
```

The All games profile has the same shape without `name`. It is `profiles/all-games.toml`, a new name for Go's `profiles/default.yaml` (decision 13). The stem is not a number, so it never shows up as a game profile, because only numeric stems are games (verified: `internal/profile/storage.go:260-297`).

```toml
version = 1

[proton]
enable_wayland = true

[dlss]
sr_preset = "auto"

[cpu]
governor = "performance"
```

`name` keeps Go's meaning. The setters fill it with the game name, and `profile list` prints it (verified: [parity-inventory.md](parity-inventory.md), CLI 3.5 and 3.16). A game "has its own profile", which the ⚑ marker shows (decision 28), when its file exists. An empty file still counts, because `profile reset` keeps the file (verified: [parity-inventory.md](parity-inventory.md), CLI 3.5).

Resolution is the same for every field (inferred, from Go's `ResolveForApply`, verified: [parity-inventory.md](parity-inventory.md), domain 2). The effective value is the game's own value when its key is present. Otherwise it is the All games value when that key is present. Otherwise it is the system default: Off, 0, empty or none. With no game file, launch applies All games directly, as Go does (verified: same).

The origin and the source words are Go's, unchanged (verified: [parity-inventory.md](parity-inventory.md), domain 2, explanation and vocabulary):

| Origin | When | Source text |
|---|---|---|
| `this_game` | The game sets it, and the value differs from the All games value | `this game · All games: <value>` |
| `this_game_same` | The game sets it, and the value equals the All games value. If All games does not set it, the value equals the system default | `this game · same as All games` |
| `all_games` | The game follows, and All games sets it | `from All games` |
| `system_default` | Nothing sets it | `system default` |
| `set_here` | All games scope only: All games sets it | `set here` |

The action words, value words, units and the ClearAll and ApplyToOwnValue texts are kept as listed in [parity-inventory.md](parity-inventory.md), domain 2 (exact vocabulary strings). The retired words stay retired.

Space switches one field between its own value and following (verified: [parity-inventory.md](parity-inventory.md), TUI 9; decision 4). In file terms:

- Own to following deletes the key from the scope's file. The TUI remembers the deleted value for that game and field.
- Following to own writes a key. A bool gets the opposite of its effective value. Another field gets its remembered value back, or opens its editor when nothing is remembered.
- On All games the same switch moves between `set here` and `system default`.
- Go keeps the remembered value in one detail view and loses it when the game changes (inferred bug: [parity-inventory.md](parity-inventory.md), TUI conflicts). The rewrite keys the memory by scope and field for the whole session (decision 8, inferred fix).

Each change is one write. The writer takes `config.lock`, reads the file fresh, sets or removes only the changed keys with toml_edit, and replaces the file. This is Go's `MergeChanges` inside `Mutate`, now on a document (verified for Go: [parity-inventory.md](parity-inventory.md), domain 3 and TUI 9). "Apply to games with their own value" stays one locked write per game. It is not atomic across games and reports the games it followed and the ones it failed (verified: [parity-inventory.md](parity-inventory.md), domain 3).

A profile that fails to load is handled per surface (inferred, from [parity-inventory.md](parity-inventory.md), CLI conflicts, "Profile load error handling differs by path"):

- CLI commands that read it, `launch --dry-run` included, fail with an error that names the file and exit 3. Go's `launch` fails the same way with `failed to load profile` (verified: [parity-inventory.md](parity-inventory.md), CLI 3.4 and additions B.4).
- The TUI shows the error on that game and does not edit it.
- Wrapper mode never refuses to run the game because spela's own data is bad. It warns and falls back (see [Game detection](#game-detection)). Go's wrapper also only warns (verified: same conflict).

Every enum is validated on load, and `cpu.governor` against the sysfs list (see [CPU](#cpu)). Go validates only `gpu.vrr` (verified: [parity-inventory.md](parity-inventory.md), domain 3), so this is a fix under decision 8.

### Deny lists

`lists/overrides.toml` keeps Go's two lists (verified shape: [parity-inventory.md](parity-inventory.md), DLL 1.3):

```toml
version = 1
allowed = [1172470]

[[denied]]
app_id = 252490
name = "Rust"
reason = "user-specified"
```

`lists/denylist.toml` is optional. spela writes it only once, when it migrates Go's `lists/denylist.yaml`, and never again; after that only the user edits it. It has `[[entries]]` with `app_id`, `name` and `reason`. Removal of an allow and of a deny is wired on both surfaces, as `denylist clear <game>` and a TUI action (decisions 20 and 51).

Three semantics change from Go:

- A list file that does not parse. Go ignores it, so DLL swaps fail open (verified: [parity-inventory.md](parity-inventory.md), DLL 3). The rewrite fails closed. While a list file does not load, install and update refuse with an error that names the file, and restore still works (decision 8, inferred fix). The same holds while a Go list file exists that the migration record does not list as migrated or skipped. The deny list module checks the record itself, so every caller fails closed, and its writes refuse too, because a new `overrides.toml` would be adopted in place of the Go file.
- A user `denylist.toml` adds to the built-in list of ten games (decision 36). In Go it replaces the built-in list, and a test pins that (verified: [parity-inventory.md](parity-inventory.md), DLL 3), while the DLL conflicts flag it as a hazard. `denylist allow` is how a user lifts a built-in entry.
- Allow and deny for one game are exclusive: `denylist allow` removes that game from `denied`, and `denylist deny` removes it from `allowed` (decision 36). So the most recent action wins. In Go a game can be in both lists, and allow always wins (verified: [parity-inventory.md](parity-inventory.md), DLL 3). A file that lists a game in both, written by Go or by hand, still resolves to allowed, because allow wins when both are present (decision 36). The next write for that game puts the file back to one list.

### Game cache

`games.yaml` in `<data>` becomes `<cache>/games.json` (decision 43). Clearing `~/.cache` then means an empty Library until the background scan ends, which is accepted. It holds only what a scan finds, so it is rebuildable, and the TUI can draw from it at once (decision 24). JSON fits a file that only spela writes. serde_json is already in the tree for backups, the manifest and `--json` (verified: [rust-crates.md](rust-crates.md), JSON).

```json
{
  "version": 1,
  "updated_at": "2026-10-26T17:02:11Z",
  "games": [
    {
      "app_id": 1091500,
      "name": "Cyberpunk 2077",
      "install_dir": "/mnt/games/SteamLibrary/steamapps/common/Cyberpunk 2077",
      "library_path": "/mnt/games/SteamLibrary",
      "prefix_path": "/mnt/games/SteamLibrary/steamapps/compatdata/1091500",
      "scanned_at": "2026-10-26T17:02:10Z",
      "dlls": [
        { "path": "/mnt/games/.../bin/x64/nvngx_dlss.dll", "name": "nvngx_dlss.dll", "type": "dlss", "version": "310.4.0" }
      ]
    }
  ]
}
```

The fields are Go's (verified: `internal/game/game.go:18-33`). Games are an array sorted by app id, not a map. The scan drops Steam tools before saving, as in Go (verified: [parity-inventory.md](parity-inventory.md), domain 7; `internal/steam/library.go:101`). The read-side name filter stays as a second guard (verified: [parity-inventory.md](parity-inventory.md), domain 6). DLL versions are the strings from Go's version formatter (verified: [parity-inventory.md](parity-inventory.md), DLL 1.3). Every write is a transaction under `games.lock` that loads the file fresh and saves only on change, as Go's `Transaction` does (verified: [parity-inventory.md](parity-inventory.md), domain 6). A file that does not parse counts as empty, so the next scan rebuilds it, and the problem is logged (inferred).

### DLL backups

Backups are read in place and never moved or reshaped (decision 50). They are how users get their original DLLs back (verified: [README.md](README.md), finding 7; [parity-inventory.md](parity-inventory.md), DLL 1.3). The [../AGENTS.md](../AGENTS.md) hard rule now says so: "DLL backups written by the Go version (`backup.json` and both backup file name schemes) are read in place and never moved or reshaped. New data goes in new top-level keys that Go ignores, never in `files[]`." The rewrite reads and writes Go's index as it is:

- The index is `<data>/backups/<appid>/backup.json`, with `app_id`, `game_name`, `created_at`, `backup_path` and `files[]` of `original_path`, `backup_path`, `dll_name` and `version`. All paths are absolute (verified: `internal/dll/swap.go:20-33`; [parity-inventory.md](parity-inventory.md), DLL 1.3).
- The index never moves, even when `backup_path` changes (verified: same).
- Restore finds files under both name schemes: `<sha256 of the cleaned original path>-<basename>`, and Go's older `<basename>` (verified: same).
- An index whose app id does not match, whose file list is empty or that has an empty path fails closed, so nothing is changed (verified: [parity-inventory.md](parity-inventory.md), DLL 2.7).
- The first backup of a path is kept (verified: same), unless Steam later replaces the file. Then it is refreshed before the next swap (decision 37; see [Restore](#restore)).
- `created_at` is written as Go writes it, RFC 3339 with nanoseconds and the local offset, through jiff (verified: [rust-crates.md](rust-crates.md), time).

The backup file and the index are synced to disk before the game's DLL is replaced (inferred).

The rewrite adds two top-level keys for decision 37 (the names are inferred):

- `created_files`: the absolute paths of DLLs that install created where none existed. Restore deletes them.
- `installed`: a map from each absolute path spela wrote to the SHA-256 of the file it last installed there. It tells a Steam update apart from spela's own install (see [Restore](#restore)).

Go's JSON decoder ignores unknown keys, so an index stays readable by Go (inferred). Go's `LoadBackup` rejects an entry with an empty path (verified: [parity-inventory.md](parity-inventory.md), DLL 2.7), so new data never goes into `files[]` (decision 50).

### Manifest cache, DLL cache and update check

These three keep Go's shapes. Decision 13 asks for new file names, so that an old Go binary that is still installed does not write the same files. The two JSON caches are renamed, and Go's files are read once, as seeds, by the migration (decision 53). Readers read only the new files, so a deleted cache stays empty until the next fetch. Backups keep Go's names (finding 7; decision 50), and the DLL cache folder stays shared with Go (decision 53).

- The manifest cache is `<cache>/manifest-v1.json`, seeded once from Go's `<cache>/manifest.json`. It is the remote manifest plus `fetched_at` and `source_url`. An old cache without those two keys counts as fetched from the default URL at the file's modification time (verified: [parity-inventory.md](parity-inventory.md), DLL 1.3). The remote URL and its schema do not change (decision 14). A manifest version is checked before it goes into a cache path (see [DLLs](#dlls)).
- The DLL cache is `<dll_cache_path or cache/dlls>/<manifest key>/<version>.dll`, holding raw DLLs (verified: [parity-inventory.md](parity-inventory.md), DLL 1.3). Every use checks the SHA-256 against the manifest, so a Go-era file is either used or downloaded again (verified: [parity-inventory.md](parity-inventory.md), DLL 2.5). The folder keeps Go's name (decision 53), because the user may have set `dll_cache_path` and every file is checked by hash, so a shared file cannot be wrong (inferred).
- The update check is `<cache>/release-check.json`, seeded once from Go's `<cache>/update-check.json`. Its shape is `{"checked_at": "<RFC 3339>", "latest": "vX.Y.Z"}`, with `latest` left out until a check succeeds (verified: `internal/release/release.go:58-61`; [parity-inventory.md](parity-inventory.md), DLL 5).

### Capability cache

The terminal capability cache follows mejl's (verified: [mejl-study.md](mejl-study.md), binary 7.3) with one change: it stores kitty keyboard, synchronized output and the XTVERSION name, and a probe counts as complete when the kitty and sync answers are definite. mejl writes only a probe whose kitty, sync and theme answers are all definite (verified: same), so the format and "complete" are spela's own (inferred). It is `<cache>/caps.toml`, format version 1, newest first, with at most 16 entries. The key is built from `TERM`, `TERM_PROGRAM`, `TERM_PROGRAM_VERSION`, the multiplexer and SSH, and a differing XTVERSION throws an entry away. Only a complete probe is written, only when it changed, atomically and without fsync. The cache only fills answers a timed-out probe left unknown, and demo mode uses none. The theme queries are dropped (see the probe row in [What carries over from mejl](#what-carries-over-from-mejl)). The TUI takes its lock before the probe runs (see [The event loop](#the-event-loop)), so there is one writer at a time and the cache needs no lock of its own (inferred).

### Logs

TUI and wrapper runs log to a daily file under `<state>/log/`. CLI commands write their warnings to stderr (decision 12). Wrapper runs also send warnings to stderr, which Steam captures (inferred). Go has no log file and writes to stderr in every mode, even while the TUI owns the terminal (verified: [parity-inventory.md](parity-inventory.md), CLI 9; [rust-crates.md](rust-crates.md), logging). Under Steam that stderr goes to Steam's own log, so restore results are hard to find (inferred: same).

- tracing and tracing-subscriber, as mejl uses (verified picks: [rust-crates.md](rust-crates.md), logging). The file setup follows mejl: tracing-appender's daily rolling files, 7 kept, no ANSI (verified: [mejl-study.md](mejl-study.md), binary 11).
- The TUI never writes logs to the terminal (inferred). ANSI goes to stderr only when stderr is a terminal (verified trap: [rust-crates.md](rust-crates.md), logging).
- The level comes from `--log-level`, then `SPELA_LOG`, then the config's `log_level`, then info (inferred, after mejl's order in [mejl-study.md](mejl-study.md), binary 11). `SPELA_LOG=debug spela %command%` in Steam's launch options then works for one game (inferred). An unknown stored level keeps the default and logs `ignoring configured log level` (verified: [parity-inventory.md](parity-inventory.md), CLI 9).
- The messages that Go's tests pin keep their wording: `restore succeeded` and `restore failed` with `area=`, the four vkd3d_heap lines, `failed to apply KDE VRR policy` and `failed to apply hardware settings` (verified: [parity-inventory.md](parity-inventory.md), CLI 9). The record layout changes from slog's text handler to tracing's (inferred in [rust-crates.md](rust-crates.md), logging).
- No redaction layer: spela handles no secrets, so mejl's redacting writer is not needed (inferred; mejl's is verified in [mejl-study.md](mejl-study.md), binary 11).
- The root helper never logs to a file. Its JSON result lines reach the parent, which logs them.
- A TUI and a wrapper run can append to the same daily file at the same time. Their lines do not interleave: tracing-appender opens the file for append and writes each event in one call. A two-process test runs 12 TUI and 12 wrapper processes at once on one log folder and checks that every line is whole (verified: `crates/spela/tests/safety.rs`, `two_processes_never_interleave_lines_in_one_log_file`). It covers lines of the length spela writes, not events larger than a pipe or page buffer.

## Migration from the Go files

The rewrite reads the Go files once and writes its own (decision 6). The Go files stay where they are as backups, and the two versions are not meant to run side by side (decision 13). The hard rules ask for every value the user set to be kept, the original left in place, and tests against files the Go version wrote ([../AGENTS.md](../AGENTS.md), hard rules).

One principle settles the edge cases: the migration keeps the effective launch behaviour (inferred). The Go bug fixes from decision 8 change what new writes do. They never reinterpret stored data. For example, `gpu set --shader-cache-path default` stores an empty own value instead of following (verified: [parity-inventory.md](parity-inventory.md), corrections). The migration keeps that empty own value, because it is what Go applies at launch. The same goes for an own value of 0 stored by the integer flags (verified: [parity-inventory.md](parity-inventory.md), CLI 3.9).

Every enum is checked on load (see [Profiles and inheritance](#profiles-and-inheritance)), and Go checked only `gpu.vrr`, so a Go file can hold a value no field allows. Where a stored value launches exactly as Go's did, the migration rewrites to it and records the rewrite: a DLSS preset Go did not know, such as `quality`, is stored as `default`, because Go's `dlssPresetToEnv` exported `render_preset_default` for every preset it did not know (verified: `internal/profile/apply.go`). The record lists it as `stored_as_default`, and the notice does not count it.

Known gap, resolved by decision 65: some Go values had no stored form that launches the same, so an earlier build recorded them as `not_supported` and let the scope above apply instead. Decision 65 closes each case. A game that marks `cpu.smt` with no value, which left SMT unchanged in Go (the `profiles/explicit-zeros` fixture has one), is stored as `smt = "unchanged"` and launches as it did. A relative `gpu.shader_cache_path`, which Go passed on as it was although its docs ask for an absolute folder, and an `sr_mode`, `rr_mode` or `multi_frame` outside the field's list, which Go's own setters refuse but its loader exported as it was, are dropped, so the field follows. Each such drop is recorded as `invalid_in_go`, and the notice names it on its own line (the `profiles/invalid-in-go` fixture has them in All games and two games). `not_supported` is still read in old records. The fixture test keeps an empty `KNOWN_GAPS` list, so a new drop that changes launch behaviour without a decision fails it.

The decisions make four exceptions to "every value is kept", and each dropped value is recorded instead (see [What is dropped, and how spela says so](#what-is-dropped-and-how-spela-says-so)):

- The fields dropped by decision 10 change nothing a user can see: the overlay IPC file had no reader, and the other fields had no effect (verified: [README.md](README.md), finding 9).
- `gpu.clock_offset` and `gpu.memory_offset` change meaning under decision 9, so their values are not carried over into the new keys (decision 31; see the table below).
- Go's GUI `theme`, the retired `show_hints` and `preferred_dll_source`, and unknown config keys are dropped (decision 46).
- A relative `gpu.shader_cache_path`, and an `sr_mode`, `rr_mode` or `multi_frame` outside the field's list, are dropped, so the field follows (decision 65). Go's setters refuse the three list values (verified: `internal/profile/fields.go:203-204`), and Go's docs ask for absolute folders, but its loader checked neither and exported them as they were (verified: [parity-inventory.md](parity-inventory.md), "Profile shader cache path is not checked like the Settings path").

### When it runs

At startup, before anything reads the store, spela lists the Go sources: `config.yaml`, `profiles/default.yaml`, `profiles/<appid>.yaml`, `lists/overrides.yaml`, `lists/denylist.yaml` and `<data>/games.yaml`. It migrates each one the record does not list as migrated or skipped (inferred). This costs a few `stat` calls and one directory read. The pass runs the config, profile and list steps under `config.lock`, so a TUI and a wrapper run that start together do not both migrate. It then releases `config.lock` and runs the game cache step under `games.lock` alone. The root helper never migrates. `--demo` starts from an empty temp dir with only `config.toml` copied in, so it never sees a Go file. Migration tests put Go fixtures into their temp dirs on purpose, and other tests start empty.

The order matters: config, then All games, then game profiles, then lists, then the game cache. A legacy game profile needs the All games values to work out its own (verified: `internal/profile/storage.go:56-62`).

### What each file becomes

Only `<numeric>.yaml` profiles are migrated. Go's `List` accepts `.yml` too, but `Load` always opens `<appid>.yaml`, so a `.yml` profile never took effect in Go (verified: `internal/profile/storage.go:29-31, 260-297`). `.yml` files and non-numeric stems are recorded as skipped.

| Go file | Becomes | Mapping |
|---|---|---|
| `<config>/config.yaml` | `<config>/config.toml` | Kept keys keep their names. A key is written only when its value differs from Go's default, and every key not written is listed in the record (decision 43). Go writes every key on each save, because most tags have no `omitempty` (verified: `internal/config/config.go:22-46, 84-96`). So the file cannot show which values the user chose, and a default value carries no choice (inferred). `shader_cache` counts as default when it equals `<cache>/nvidia` as Go computed it (verified default: [parity-inventory.md](parity-inventory.md), CLI 11). A value that Go itself ignored at use, such as an unknown log level, is dropped with a note, because Go used the default (verified: [parity-inventory.md](parity-inventory.md), CLI 9). The `[ui]` keys start at their defaults |
| `<config>/profiles/default.yaml` | `<config>/profiles/all-games.toml` | Go's `sr_model_preset` step runs first (verified: [parity-inventory.md](parity-inventory.md), domain 3). A kept field is written when its key is marked in `overrides` or its value is not zero, which is Go's rule for what All games sets (verified: `internal/profile/explanation.go:129-138`). A non-nil `smt`, including `false`, counts as set, and so does a marked `smt` with no value, which is written as `smt = "unchanged"` (decision 65). `name` and the `overrides` map are not written. A non-empty `name` goes in the record |
| `<config>/profiles/<appid>.yaml` | `<config>/profiles/<appid>.toml` | Go's two in-memory steps run first, with the same rules Go uses: legacy inheritance when the `overrides` key is missing, measured against the raw `default.yaml`, and `sr_model_preset` (verified: `internal/profile/storage.go:56-62`; `internal/profile/inheritance.go:275-325`; [parity-inventory.md](parity-inventory.md), domain 3). A kept field is written when its key is marked true, with its stored value even if that is zero. A marked `smt` with no value is written as `smt = "unchanged"`, and a value outside Go's own rules is dropped as `invalid_in_go` (decision 65). `name` is kept. A leaf with no marker had no effect in Go, so it is dropped and recorded. A `false` marker means follows and writes nothing. An unknown marker key is recorded |
| `<config>/lists/overrides.yaml` | `<config>/lists/overrides.toml` | `allowed` and `denied` are copied as they are, except for an app in both lists. Allow wins, as it does in Go (verified: [parity-inventory.md](parity-inventory.md), DLL 3; decision 36): the app goes in `allowed` only, and its `denied` entry, with name and reason, goes in the record |
| `<config>/lists/denylist.yaml` | `<config>/lists/denylist.toml` | `entries` copied as they are |
| `<data>/games.yaml` | `<cache>/games.json` | Every game and DLL copied, with times parsed from Go's RFC 3339. A failure here is not an error: the cache is rebuilt by the next scan |
| `<data>/backups/`, `<cache>/dlls/` | Themselves | Read in place. Nothing is rewritten |
| `<cache>/manifest.json`, `<cache>/update-check.json` | `<cache>/manifest-v1.json`, `<cache>/release-check.json` (decision 53) | Copied once as seeds when the new file is missing. A failure is not an error: the cache refills |
| `spela.pid`, `games.lock`, `profiles/.mutation.lock`, `overlay-<appid>.dat` | Nothing | Ignored |

Two GPU fields need their own rows, because their meaning changes:

| Go field | Go meaning | Rust meaning | Mapping (decision 31) |
|---|---|---|---|
| `gpu.clock_offset` | Locks graphics clocks to 0 to 2100+N MHz (verified: [parity-inventory.md](parity-inventory.md), domain 1 and CLI 7.2) | A real NVML clock offset in MHz (decisions 9 and 32) | Not copied into `clock_offset`. A lock to 2250 MHz caps the clock, while a +150 MHz offset raises it. The value goes in the record and the file's header comment, and the notice says so |
| `gpu.memory_offset` | Locks memory clocks to at most N MHz (verified: same) | A real NVML memory offset in MHz (decisions 9 and 32) | Not copied. A stored 9501 would ask for +9501 MHz. Recorded and reported the same way |

[../AGENTS.md](../AGENTS.md)'s migration rule names this case: a value that decisions 10, 31, 46 or 65 drop is recorded in `migration.toml` and the file's header instead of kept.

### What is dropped, and how spela says so

Decision 10 drops these profile fields with every value they hold: `overlay.enabled`, `overlay.position`, `overlay.show_fps`, `overlay.show_frametime`, `overlay.show_cpu`, `overlay.show_gpu`, `overlay.show_vram`, `overlay.toggle_key`, `gpu.power_mizer` and `cpu.affinity`. The config drops `compact_mode`, because the Compact and Focused layouts are gone (decision 3).

Decision 46 drops three more kinds of config key, each recorded in the migration record and counted in the notice:

- the GUI's `theme` (verified GUI only: [parity-inventory.md](parity-inventory.md), CLI 11). `spela gui` is removed (decision 26), and Go's values `default`, `dark` and `light` mean nothing to the terminal themes `terminal`, `neon` and `mono` (verified values: same; decision 27). It is never mapped to `ui.theme`;
- the retired keys `show_hints` and `preferred_dll_source`, which Go already ignores (verified: same);
- any other unknown key.

Decision 31 drops Go's `gpu.clock_offset` and `gpu.memory_offset` values from the new keys (see [What each file becomes](#what-each-file-becomes)). Decision 65 drops a relative `gpu.shader_cache_path` and an `sr_mode`, `rr_mode` or `multi_frame` outside the field's list, each recorded as `invalid_in_go`. The migration of `lists/overrides.yaml` drops the `denied` entry of a game that is also allowed (decision 36).

The migration says what it dropped in three places (decision 10 asks for a note; the places are inferred):

1. The migration record, `<data>/migration.toml`, is the full and lasting account. It has one entry per Go source:

   ```toml
   version = 1

   [[source]]
   path = "/home/me/.config/spela/profiles/1091500.yaml"
   sha256 = "9f2c..."
   target = "/home/me/.config/spela/profiles/1091500.toml"
   result = "migrated"                 # migrated, skipped or failed
   at = 2026-10-26T17:04:11Z
   by = "spela 0.7.0"

   [[source.dropped]]
   key = "overlay.enabled"
   value = "true"
   why = "retired"
   default = false

   [[source.dropped]]
   key = "gpu.clock_offset"
   value = "150"
   why = "offset_meaning_changed"    # Go's clock lock, not copied (decision 31)
   default = false
   ```

   The record keeps every dropped key and value, defaults included. Comments in a TOML file can be lost when the file is edited by hand, and log files roll over, so the record is the one copy that lasts. Each dropped value says why (`retired`, `offset_meaning_changed`, `gui_only`, `retired_key`, `unknown_key`, `no_marker`, `unknown_marker`, `ignored_by_go`, `default`, `not_supported`, `invalid_in_go`, `stored_as_default`, `all_games_name` or `allowed_and_denied`) and whether it is Go's default or zero, which the notice leaves out. A skipped source has a `note`, and a failed one an `error`. `at` is in UTC (as built: `crates/spela-core/src/migrate/record.rs`).
2. Each new TOML file starts with a short comment that names its Go source and the values it did not carry over, as in the game profile example above.
3. On the run that migrates, the CLI prints one line to stderr and the TUI shows the same lines in a popup over its first frame, titled `Migrated from Go spela`, which Enter or Esc closes (as built in Phase 4). A status line message would not do: it clears after 5 s, the startup scan's `Rescan complete` replaces it, and at 80 columns it cannot hold the record's path. The first line reads `Migrated 14 files from Go spela, kept as backups only. Dropped 3 settings. See ~/.local/share/spela/migration.toml`. When a Go profile set a clock or memory offset, a second line says that those values were not carried over because the fields now mean real offsets (decision 31), for example `GPU clock and memory offsets from Go spela were not carried over. See ~/.local/share/spela/migration.toml`. Then each value dropped as `invalid_in_go` gets a line of its own that names the key, the Go file and the value, and says what the field does now (decision 65), for example `dlss.sr_mode in 1091500.yaml was not carried over: "ultra" is not valid in Go spela either, so the field now follows All games.` The log gets one line per file. The notice counts only dropped values that differ from Go's defaults, so a user who never touched the overlay or `compact_mode` is not told about them.

### The originals stay as backups

The migration only reads Go files. It never renames, rewrites or deletes one ([../AGENTS.md](../AGENTS.md), hard rules). The record holds each source's SHA-256, and the tests check that the bytes are unchanged after a migration. After a successful migration, spela never imports that Go file again. `default.yaml` is still read as the base for any legacy game profile migrated later, by a retry or because the file is new. If a Go file changes after its migration, because the Go binary is still installed and saved over it, spela does not import it again and does not warn (decision 43). The migration notice already says that the Go files are backups only.

### Idempotence

A source is migrated at most once, and "migrated" means "listed in the record as migrated or skipped", not "the target exists" (inferred). If a missing target triggered a migration, `spela profile delete <game>` would bring the game's YAML profile back at the next start. Removing `config.toml` to get the defaults back, as Go's error text tells users to do, would do the same (verified text: [parity-inventory.md](parity-inventory.md), CLI 3.14).

Writing the target and updating the record are two steps, so a crash or a failed record write can fall between them. The record is saved after each source, so at most one target is left unrecorded. On the next start, a source that is missing from the record but whose target exists and parses is adopted. It is recorded as migrated and the target is not touched (inferred). The Go file is still converted, without writing, so the values it did not carry over go into the record all the same, and the notice counts them, because the run that wrote the target never showed one. A second run over the same files writes nothing. If the record is deleted, spela adopts every target that still exists and imports again only the sources whose targets are gone.

### Failure handling

Each source succeeds or fails on its own. A failed source writes no target, leaves the Go file alone, and is recorded as `failed` with the error. It is tried again on every start until it works (inferred). The rebuildable caches are the exception: a `games.yaml`, `manifest.json` or `update-check.json` that cannot be read or parsed is recorded as `skipped` with the error in its note, so it is logged once and never read again.

- A YAML that does not parse, a file that cannot be read, or an invalid `gpu.vrr` fails that file. Go would fail to load the same file (verified: [parity-inventory.md](parity-inventory.md), domain 3).
- A `default.yaml` that fails also fails All games and every legacy game profile, because they need it. Go's legacy `Load` fails the same way (verified: same). Game profiles that have an `overrides` key do not depend on it and still migrate.
- A failed `config.yaml` is a config error with exit code 3 in the CLI and the TUI. The message names the YAML file and keeps Go's advice to fix it or remove it (verified text: [parity-inventory.md](parity-inventory.md), CLI 3.14). Once the user removes it, there is no source and the defaults apply. In wrapper mode, a config that fails to load or migrate warns on stderr and in the log, and the game launches with the default config and no profile (inferred, so a wrapper launch never fails on spela's own data, see [Game detection](#game-detection)).
- A failed profile counts as a broken profile, with the per-surface rule in [Profiles and inheritance](#profiles-and-inheritance): the CLI and `launch --dry-run` report it and exit 3, and wrapper mode warns and falls back.
- A failed list file blocks DLL install and update, which is the fail-closed rule under [Deny lists](#deny-lists).
- A failed `games.yaml` is logged once and skipped, and the scan rebuilds the cache.
- A failed target write, such as a full disk, leaves the target absent and the record unchanged, so the next start tries again. A failed record write stops the run with exit 3; the target written just before it is adopted on the next start (see [Idempotence](#idempotence)).

### Reading YAML

Only the migration reads YAML, and nothing writes it any more, so yaml.v3 byte compatibility no longer matters (inferred from decision 6). The reader is serde-saphyr 2.0.0, pinned exactly and wrapped behind one module, because it had two major releases in three months (verified: [rust-crates.md](rust-crates.md), YAML and traps). What it does that the migration needs (verified: [rust-crates.md](rust-crates.md), YAML):

- It reads the 4-space files yaml.v3 writes.
- It reads YAML 1.1 bools such as `smt: yes`, as yaml.v3 does, and is the only serde YAML crate tested that does.
- It rejects duplicate keys and tab indentation, as yaml.v3 does.
- It tells a missing `overrides` key from `overrides: {}` through `Option<BTreeMap<String, bool>>`.
- Two shapes yaml.v3 loads need help: only the first document is read, as yaml.v3's `Unmarshal` reads it, so a second `---` document is ignored, and a Go `int` leaf takes a float cut toward zero, so `power_limit: 250.0` reads as 250 and `fan_speed: 40.7` as 40 (verified: the `profiles/yaml-shapes` fixture and its oracle).

Go's tests also require 2-space files to load (verified: [parity-inventory.md](parity-inventory.md), domain 3). The probe covered only yaml.v3's 4-space output, so 2-space reading is (unverified) until the fixture case runs. To report unknown and retired keys, the reader has to keep the keys it does not know. Whether serde-saphyr supports `#[serde(flatten)]` into a map for that was not probed (unverified).

### Fixture tests

The fixtures come from the real Go code, not from hand-written YAML (inferred). They use fictional games and app ids (decision 43). A small Go generator is committed at `tools/goref`, and the fixtures it writes are in `tests/fixtures/go`. It is new code beside the Go tree and leaves the existing Go code untouched (decision 62). It calls Go's own `config.Save`, `profile.Save`, `profile.SaveDefault`, the game database save, `denylist.SaveOverrides` and the backup code against a temp XDG tree. The `goref` program from the crate research did this from copied structs in a scratch folder (verified: [rust-crates.md](rust-crates.md), how this was gathered). Phase 0 rebuilt it in `tools/goref` on Go's own code. Go cannot write the older shapes any more, so those cases are written as literal YAML in the generator. The fixtures and the expected results are committed in Phase 0, so they outlive the Go code when it is deleted in the final step ([../AGENTS.md](../AGENTS.md), layout; decision 62).

The cases:

- a default config, and a config with every key set, including `theme`, `compact_mode`, a library path list and explicit zeros;
- a config with the retired keys and an unknown log level;
- a profile with an `overrides` map, explicit zeros and `smt: false`;
- a legacy profile with no `overrides` key, against a `default.yaml` that has values, and against no `default.yaml`;
- a `default.yaml` with set values that are not marked;
- `sr_model_preset`, `smt: yes`, 2-space indentation, `overrides: {}`, `false` markers and unknown marker keys;
- every dropped field set, and a non-zero `clock_offset` and `memory_offset`;
- an invalid `gpu.vrr`, a YAML that does not parse, and a `.yml` profile;
- `games.yaml` with tools, times and DLL versions;
- `overrides.yaml` empty (`allowed: []`), full, and with one game in both lists, and a `denylist.yaml`;
- backup indexes with both file name schemes, a manifest cache with and without `fetched_at`, and `update-check.json`.

The generator also writes a value oracle, which is separate from the Go e2e journeys that decision 54 drops: for each profile fixture, Go's effective value and origin for every field, from `ResolveForApply` and `ExplainField`. The Rust tests check that:

- the migrated TOML resolves to the same effective value and origin for all 25 kept fields;
- every dropped value appears in the record;
- the new files match insta snapshots;
- the source bytes are unchanged;
- a second run writes nothing;
- a failed source writes nothing;
- `profile delete` followed by a restart does not bring the profile back;
- a restore works from Go-written backups under both name schemes.

All of it runs under `Paths::under` with temp dirs and no network ([../AGENTS.md](../AGENTS.md), hard rules).

## CLI

### Shape

- One binary, `spela`, built with clap derive (verified pick: [rust-crates.md](rust-crates.md), CLI). `main.rs` only calls a library entry point, as mejl's does, so every command can be tested without a terminal (verified: [mejl-study.md](mejl-study.md), binary 1). The entry point takes injected stdout, stderr, environment and paths, like mejl's `run(cli, ctx, out, err)` seam (verified: [mejl-study.md](mejl-study.md), binary 9.3). The paths come in as a resolver that `main` sets to `Paths::from_system`, which reads HOME and the `XDG_*` variables through etcetera. It runs only after the root helper is dispatched, so the helper never resolves an XDG path (as built in Phase 1).
- Before clap runs, a small router decides between a subcommand and a wrapped game. The rules are in [Routing](#routing-subcommand-or-wrapped-game). Known subcommands always win (decision 21).
- `spela` with no arguments prints help and exits 0, as today (verified: [parity-inventory.md](parity-inventory.md), CLI 2.2).
- Global flags work before or after the subcommand, as mejl's `--log-level` does (verified: [mejl-study.md](mejl-study.md), binary 9.1): `--log-level <level>`, `--demo`, `--json`, `--help`, `--version`. Unlike mejl, where `--demo` conflicts with subcommands (verified: same), spela's `--demo` is global (inferred; see [Backends and fakes](#backends-and-fakes)). `--json` is global so a failure in any command can print the JSON error object; only the commands in [JSON output](#json-output-schema-version-1) print JSON results. Until each command's own arguments are declared, the parser moves these global flags to the front before clap runs, so a flag after a game name still counts (as built in Phase 1).
- Numeric flags set `allow_negative_numbers`, so `--clock-offset -100` still parses as it does under pflag (verified trap: [rust-crates.md](rust-crates.md), CLI).
- Flags whose presence matters are `Option<T>`, which reproduces cobra's `Changed()` (verified: [rust-crates.md](rust-crates.md), CLI).
- One bool parser for every flag and config value: true, 1, yes, on and false, 0, no, off, in any case (inferred). Today config takes only true/false/1/0, profile flags take more, and `cpu set --smt` takes a third set (verified: [parity-inventory.md](parity-inventory.md), CLI conflicts, "Inconsistent bool vocabularies").
- Field names keep today's normalisation and aliases: `-` becomes `_`, and proton.hdr, proton.wayland, proton.ngx_updater, gpu.threaded_opt and dlss.fg map to their full keys (verified: [parity-inventory.md](parity-inventory.md), CLI 3.16).
- Every setter accepts `default`, which resets the field so it follows All games. The exceptions are `--sr-preset` and `--rr-preset`. There `default` stays the stored preset value that exports `render_preset_default` (verified: `internal/profile/fields.go:68, 71` list `default` among their values). Those two follow All games through `dlss reset <game> sr_preset|rr_preset` (inferred). Today only `cpu set --governor default` and `--smt default` reset, and `--shader-cache-path default` stores an empty override instead (verified: [parity-inventory.md](parity-inventory.md), corrections). Using reset for every `default` is (inferred).

### Commands

The game argument resolves as today: a numeric AppID first, then an exact, case-sensitive name (verified: [parity-inventory.md](parity-inventory.md), CLI 4). When two games share a name, the lowest AppID wins instead of Go's map order (inferred fix of the non-determinism noted in CLI 4). Commands that edit a profile take `--all-games` in place of `<game>`, which edits All games (decision 20; the CLI cannot do this today, verified: [parity-inventory.md](parity-inventory.md), additions B.1).

| Go command | Rust command | Change, with the current behaviour it replaces |
|---|---|---|
| `spela` | `spela` | Help, exit 0. Unchanged |
| `--version`, `-v` | `--version`, `-v` and `-V` | `-v` stays as an alias, through `#[arg(short = 'v', short_alias = 'V', long = "version", action = ArgAction::Version)]` with `disable_version_flag = true`, which removes clap's own flag (inferred). The text is clap's `spela 0.7.0` (decision 48). cobra printed `spela version <v>` (verified Go form: [parity-inventory.md](parity-inventory.md), CLI 2.2) |
| `help`, `completion` | `help`, `completion <shell>` | Static scripts from clap_complete in place of cobra's dynamic `__complete` (decision 57; verified pick and trade-off: [rust-crates.md](rust-crates.md), CLI and open questions) |
| `scan [--json]` | `scan [--json]` | The count line and the list include XeSS-only and FSR-only games, which the label already promises (verified: [parity-inventory.md](parity-inventory.md), corrections). `--json` drops Steam tools, like `list` (CLI 3.1 and CLI conflicts) |
| `list [--with-dlls] [--json]` | same | No rescan unless the game cache is empty (decision 24; today it rescans on every run, CLI 3.2). An empty cache under `--json` prints an empty list, not text (bug: CLI 3.2) |
| `show <game> [--json]` | same | Same rescan rule as `list` |
| `launch <game> [--game-id N] [--dry-run]` | `launch <game> --dry-run`, `launch --game-id N --dry-run` | Dry run only. Without `--dry-run` it exits 2 instead of Go's 1, with a hint to put `spela %command%` in the game's Steam launch options (decision 40). The two-argument form that executed the game query is removed (bug: CLI 3.4). `game not found: <q>` replaces the bare `game not found` (CLI 4) |
| `profile list`, `create`, `show [--json]`, `reset` | same, plus `--all-games` on `show` and `reset` | `reset --all-games` is "Use system default for all settings" |
| `profile delete <game>` | same | Prints `No profile for <Name>` when none existed, instead of a false success (bug: CLI 3.5) |
| none | `profile follow <field>... [--games <list>]` | "Apply to games with their own value": every listed game, or every game, that sets the field follows All games (decisions 20 and 51; TUI only today, additions B.1) |
| `config show` | same | Prints TOML, the new format (decision 6) |
| `config set <key> <value>` | same | Every key the TUI's Settings shows, not only log_level, shader_cache and check_updates (decision 20; CLI 3.6 and 11) |
| `dll list [game]` | same | Without a game, lists every game with DLSS, FSR or XeSS (bug: corrections). Sorted by name (DLL 2.11 notes map order) |
| `dll check-updates` | same | Includes XeSS and FSR. One meaning of "update available", the same function Library and Catalog call (decision 38; see [DLLs](#dlls)) |
| `dll update <game> <type>` | `dll update <game> <type> [--version V] [--cached-only]`, `dll update --all [--type T]` | Pinning, cached-only and update all come from the TUI (decision 20; DLL 2.11) |
| none | `dll install <game> <type> [--version V]` | TUI only today (DLL 2.11) |
| none | `dll cache [type]` | Lists cached versions, TUI only today (DLL 2.11) |
| `dll restore <game>` | same | See [Restore](#restore) |
| `dlss show`, `set`, `reset` | same | `--sr-preset` help lists the real values, default, auto, A to F and J to M. `--rr-preset auto` fails at parse time. Both were help-text mismatches (corrections). The deprecated `--sr-model-preset` flag is dropped (inferred). The migration still maps the old key (domain 3). `--sr-override` sets `dlss.sr_override` itself, as the TUI can (decision 20). `reset`, like `gpu` and `cpu profile-reset`, takes any number of fields, and with none resets the whole section (as built in Phase 3) |
| `gpu info` | same | NVML only, with no `nvidia-smi` fallback (decision 58; see [NVML](#nvml)) |
| `gpu reset` | same | One pkexec through the one-shot helper, instead of two `pkexec nvidia-smi` calls (CLI 3.9; domain conflicts, "Per-CPU pkexec outside apply-profile"). It resets locked clocks and sets both VF offsets to 0 (inferred, with decision 32). Also a TUI action (decision 44) |
| `gpu set <game>` | same, plus `--vrr` | `--power-mizer` is removed (decision 10). `--vrr` comes from the TUI (decision 20; there is no flag today, CLI 3.9). `--fan-speed 0` stores 0, which means no fan change at launch, and the help says so (see [Partial-failure rules](#partial-failure-rules)). `default` follows All games. Today the help says "0 to clear" and stores an override of 0 (bug: CLI 3.9). The offset flags change meaning (see [GPU offsets](#gpu-offsets)) |
| `gpu show`, `profile-reset` | same | `power_mizer` is no longer a field |
| `cpu info` | same | Unchanged lines (CLI 3.10) |
| `cpu governor [g]` | same | One pkexec through the one-shot helper, not one `pkexec tee` per CPU (bug: CLI 3.10). Also a TUI action (decision 44) |
| `cpu smt [on\|off]` | same | Any value outside the bool vocabulary is a usage error, exit 2. Today a typo disables SMT (bug: CLI 3.10). Also a TUI action (decision 44) |
| `cpu set`, `show`, `profile-reset` | same | `affinity` is no longer a field (decision 10) |
| `proton set`, `show`, `reset` | same | Unchanged (CLI 3.11) |
| `overlay set`, `show`, `reset` | reserved name | Removed (decision 10). The name stays reserved: any `spela overlay ...` prints that the command was removed in 0.7.0 and exits 2, so it never reaches wrapper detection and a program on PATH named `overlay` is never launched for it (decision 45; `/usr/bin/overlay` from mesa-demos exists on this machine, verified: [rust-crates.md](rust-crates.md), conflicts) |
| `denylist show`, `check`, `allow`, `deny [--reason]` | same | Unchanged text, including the corrected `denylist show` layout and the `PUBG: Battlegrounds` name (corrections) |
| none | `denylist clear <game>` | Removes the user's allow or deny for that game (decisions 20 and 51; removal is wired nowhere today, DLL 3). `allow` and `deny` now clear each other (decision 36; see [Deny lists](#deny-lists)) |
| `tui` | same | Takes the TUI lock (decision 29). A bad config exits 3 with today's two-line recovery message, now naming the TOML file or the YAML file whose migration failed, instead of exiting 0 (CLI 3.14) |
| `gui` | reserved name | Removed (decision 26). Reserved like `overlay`: it prints that the command was removed in 0.7.0 and exits 2 (decision 45) |
| `apply-profile` (hidden) | `apply-profile --serve`, `apply-profile --once <request>` (hidden) | New protocol, see [Protocol](#protocol). Go's flag form, `--reset` included, is not accepted and prints a clear error (decisions 13 and 34) |

Command citations in the last column are to [parity-inventory.md](parity-inventory.md).

### Output, errors and exit codes

- Results go to stdout. Warnings, progress and errors go to stderr (inferred, after mejl's seam in [mejl-study.md](mejl-study.md), binary 9.3).
- Colour goes only to a terminal, as anstyle styles that spela writes only when the stream is a terminal and `NO_COLOR` is unset in the env snapshot ([terminal-ux.md](terminal-ux.md), CLI output). Go always writes ANSI, even to a pipe, with `TERM=dumb` and with `NO_COLOR=1` (verified: [phase0-checks.md](phase0-checks.md), Go behaviour found while writing the fixtures). The CLI shares the TUI's theme roles and vocabulary ([terminal-ux.md](terminal-ux.md), CLI output).
- Errors print `error: <message>` and, where one helps, a one-line hint such as `hint: run 'spela scan' to refresh the game list`. The full usage text is no longer printed after a runtime error ([terminal-ux.md](terminal-ux.md), CLI output; today cobra prints `Error:` plus usage to stderr, verified: [parity-inventory.md](parity-inventory.md), CLI 2.3). clap's own usage errors keep clap's format.
- Message text stays where tests pin it today, for example `unknown config key: not_a_key`, `unsupported log level: verbose` and `game not found: missing` (verified: [parity-inventory.md](parity-inventory.md), CLI 2.3).
- A closed stdout pipe, as in `spela list --json | head`, must not end in Rust's broken-pipe panic (verified trap: [rust-crates.md](rust-crates.md), version and license traps). The handling is (inferred):
  - spela keeps Rust's ignored SIGPIPE and never resets it to the default. A default SIGPIPE would let a closed stdout kill the wrapper, which prints the summary to Steam's stdout, or the root helper, which writes JSON lines to a parent that may be gone, before any restore runs.
  - All output uses fallible writes (`writeln!` with the error handled), never `println!`.
  - The CLI treats `BrokenPipe` on stdout as a quiet exit 0.
  - The wrapper ignores stdout errors and still runs cleanup.
  - The helper treats EPIPE on its report pipe as "parent gone". It finishes or rolls back the current step, and the `--serve` helper then restores what it applied.

Exit codes follow mejl's pattern ([terminal-ux.md](terminal-ux.md), CLI output; mejl's values verified: [mejl-study.md](mejl-study.md), binary 9.2):

| Code | Meaning |
|---|---|
| 0 | Success, help and version |
| 1 | Any other failure |
| 2 | Usage error, from clap or from spela's own argument checks |
| 3 | Config or profile file invalid |
| 4 | Unused. mejl uses it for auth. spela leaves it free so the two tools agree (inferred) |
| 5 | Locked: `spela is already running (PID: N)` (decision 29) |
| game's code | Wrapper mode, after the game ran: the game's own exit code, or 128 plus the signal number if a signal ended it (decision 8; today it is 0 or 1 and the status is lost, verified: [parity-inventory.md](parity-inventory.md), CLI 5.2) |

In wrapper mode, spela's own codes apply only when spela fails before the game starts. A restore failure after the game exits is logged and does not change the exit code, as today a cleanup failure never hides the launch result (verified: [parity-inventory.md](parity-inventory.md), CLI 6.8).

`--help` ends with an exit code and environment block, as mejl's does (verified: [mejl-study.md](mejl-study.md), binary 9.1): the codes above, `SPELA_LOG` and `NO_COLOR`.

### JSON output, schema version 1

`--json` gets a new, versioned snake_case schema (decision 7). Go's output is not kept. It uses Go field names such as `AppID` and `SRMode`, prints `null` for empty lists, escapes HTML characters, and mixes stored and resolved values (verified: [rust-crates.md](rust-crates.md), JSON; [parity-inventory.md](parity-inventory.md), additions B.5 and CLI conflicts). The Go-escaping formatter from [rust-crates.md](rust-crates.md) is not needed.

The rules (inferred unless marked):

- Every output is one JSON object with `"schema_version": 1` as its first key. There are no bare arrays at the top level, so keys can be added later.
- Keys are snake_case and come in a fixed order.
- A list is always present, and an empty one is `[]`, never `null` and never left out.
- A value that can be missing is present as `null`. No key is left out, so every object of a kind has the same keys.
- Values are raw: numbers are numbers, so `150` and not `"150 MHz"`, and units go in a separate `unit` key. Times are RFC 3339 in UTC. App ids are numbers.
- Lists are sorted: games by name and then app id, DLLs in catalogue order (dlss, dlssg, dlssd, xess, fsr) and then path, profile fields in descriptor order. The catalogue order is Go's (verified: [parity-inventory.md](parity-inventory.md), DLL 2.1).
- The origin strings are Go's origin names (verified: [parity-inventory.md](parity-inventory.md), domain 2). Display text such as `from All games` stays out of JSON, so rewording the UI never breaks a script.
- Output is pretty printed and ends with a newline. Colour never goes into JSON, and logs and warnings go to stderr.
- With `--json`, a failure prints only an error object on stdout and exits with the usual code. An example is `{"schema_version": 1, "error": {"code": "game_not_found", "message": "game not found: missing"}}`. A value that spela's own flag check refuses, such as `--sr-mode bogus`, counts as such a failure, with code `usage` and exit 2, even though the check runs inside clap. clap's own usage errors, such as an unknown flag, keep clap's text on stderr (see [Output, errors and exit codes](#output-errors-and-exit-codes)).
- A new key or a new command keeps version 1. Removing or renaming a key, or changing a value's type or meaning, raises the version for every command. It is noted in the changelog. Only the current version is printed. Snapshot tests pin the version 1 output.

The commands are Go's `--json` set without `overlay show` (verified set: [rust-crates.md](rust-crates.md), what spela needs; [parity-inventory.md](parity-inventory.md), CLI 3.1 to 3.11):

| Command | Object |
|---|---|
| `scan`, `list` | `{"schema_version": 1, "games": [game, ...]}`. `scan` filters Steam tools like `list` does. In Go, `scan --json` includes them as a map keyed by app id (verified: [parity-inventory.md](parity-inventory.md), CLI conflicts and additions B.5). An empty cache gives `"games": []` |
| `show <game>` | `{"schema_version": 1, "game": game}` |
| `profile show <game>` | `{"schema_version": 1, "scope": "game", "app_id": 1091500, "name": "Cyberpunk 2077", "has_profile": true, "fields": [field, ...]}` with all 25 fields. It works without a profile file. Go errors there (verified: [parity-inventory.md](parity-inventory.md), CLI 3.5) |
| `dlss show`, `gpu show`, `cpu show`, `proton show` | The same object, with `fields` limited to that section |

With `--all-games`, `scope` is `"all_games"`, `app_id` and `name` are `null`, and the origins are `set_here` or `system_default`.

A `game` has the cache fields plus `has_profile`:

```json
{
  "app_id": 1091500,
  "name": "Cyberpunk 2077",
  "install_dir": "/mnt/games/SteamLibrary/steamapps/common/Cyberpunk 2077",
  "library_path": "/mnt/games/SteamLibrary",
  "prefix_path": null,
  "scanned_at": "2026-10-26T16:02:10Z",
  "has_profile": true,
  "dlls": [
    { "path": "/mnt/games/.../nvngx_dlss.dll", "name": "nvngx_dlss.dll", "type": "dlss", "version": null }
  ]
}
```

A `field` has both the stored and the resolved view. Go split these: `profile show --json` printed the raw stored profile, and the section shows printed resolved values (verified: [parity-inventory.md](parity-inventory.md), CLI conflicts).

```json
{
  "key": "gpu.power_limit",
  "section": "gpu",
  "kind": "int",
  "unit": "W",
  "value": 250,
  "own_value": 250,
  "all_games_value": null,
  "origin": "this_game"
}
```

`kind` is `bool`, `int`, `string` or `optional_bool`, and it fixes the type of the three values. `own_value` is `null` when the scope follows. A scope that leaves SMT unchanged on purpose (decision 65) also has `own_value` `null`, because "none" is `null` in every value; its `origin` names the scope. On All games, `own_value` is the value set there and `all_games_value` is always `null`. `all_games_value` is `null` when All games does not set the field. `value` is the effective value, and it is `null` only for an `optional_bool` at its "none" default.

Other read commands, such as `dll list`, `denylist show` and `config show`, can gain `--json` results later without a version change. Version 1 does not add them. They accept the global `--json` flag only so that a failure prints the error object (see [Shape](#shape)).

The DLL commands gained objects in Phase 5, as new commands under version 1 (as built):

| Command | Object |
|---|---|
| `dll install`, `dll update` | `{"schema_version": 1, "items": [item, ...], "skipped": [{"app_id", "name", "reason"}, ...], "updated": n, "unchanged": n, "failed": n}`. An item has `app_id`, `name`, `type`, `path`, `from_version`, `to_version`, `outcome` (`changed`, `unchanged` or `failed`) and `error` (`null`, or `code`, `message` and `files_changed`). Items sort by game name, then catalogue order, then path. `skipped` lists the deny-listed games `--all` left out |
| `dll restore` | `{"schema_version": 1, "app_id", "name", "restored": [path, ...], "deleted": [path, ...], "kept": [path, ...]}`. `kept` lists files an install created that Steam has since replaced, which restore keeps (as built) |
| `dll cache` | `{"schema_version": 1, "cache_dir", "types": [{"type", "versions": [newest first]}, ...]}` |
| `dll check-updates` | `{"schema_version": 1, "manifest": {"source_url", "fetched_at", "fetched_now"}, "games": [{"app_id", "name", "dlls": [{"path", "name", "type", "version", "latest"}]}]}` with only the DLLs that need an update |

A batch with a failed item still exits 1 and prints only the error object, code `dll_update_failed`. Its new `details` key holds the whole object above, so a script sees what changed before the failure. Other error objects have no `details` key (as built).

## Launch and privilege

### Routing: subcommand or wrapped game

The router runs on the raw arguments before clap (inferred design for decision 21):

1. If the first argument starts with `-` and is not a bare `--`, clap parses it. This keeps `--help`, `--version` and global flags.
2. If the first argument is a known subcommand name, clap parses it. Known names are every command in the [Commands](#commands) table, `help`, `completion`, the hidden `apply-profile`, and the reserved `overlay` and `gui` (decision 45).
3. Otherwise, including a leading bare `--`, the arguments are read as a wrapper invocation, using today's rules: leading `KEY=VALUE` tokens with keys matching `[A-Za-z_][A-Za-z0-9_]*` are environment, and a bare `--`, first or after the env tokens, ends them. A leading bare `--` starts the command with no environment tokens, so `spela -- %command%` launches as it does in Go (verified: [parity-inventory.md](parity-inventory.md), CLI 5.1; `internal/launcher/launcher.go:297-320`).
4. The first remaining token is a game launch when it is an absolute path that exists, contains `/`, or is found on PATH with no extension, `.exe` or `.sh`. The extension check ignores case, as Go's `strings.ToLower` does (verified rules: [parity-inventory.md](parity-inventory.md), CLI 5.1; `internal/launcher/launcher.go:288`).
5. Anything else goes to clap, which reports an unknown command with exit 2.

This fixes the pre-emption bug where a subcommand whose name is also a program on PATH launched that program, as `spela overlay` ran `/usr/bin/overlay`. The removed `overlay` and `gui` stay reserved, so the fix covers them too (decision 45; verified bug: [parity-inventory.md](parity-inventory.md), CLI 1 and 5.1; [README.md](README.md), finding 5). Steam's `%command%` usually starts with an absolute path, so step 2 never sees it (inferred).

### Game detection

`DetectGameFromCommand` is kept and fixed (inferred design, fixing the bug in [parity-inventory.md](parity-inventory.md), CLI 5.2):

1. The env vars `SteamAppId`, then `SteamGameId`, trimmed and parsed as an AppID in the game cache (verified as today's first step: CLI 5.2). That Steam sets them in the launch environment is (unverified, a user live check).
2. An argument `AppId=<id>` or `SteamAppId=<id>`. spela's running-game scan already matches `AppId=<id>` in process arguments (verified: [parity-inventory.md](parity-inventory.md), domain 6), so Steam's reaper argument is the likely form (inferred).
3. Install folder match by path, not substring: an argument equals a game's install folder or sits under `folder/`, the rule the running-game scan uses (verified: domain 6). Empty install folders are skipped. The longest match wins, and a tie means no game. Today any substring matches, an empty folder matches everything, and map order picks among collisions such as `common/Portal` and `common/Portal 2` (bug: CLI 5.2).

If no game is found, spela prints `Warning: could not detect game for wrapper invocation` and runs the command with no profile, as today (verified: CLI 5.2). Wrapper mode never rescans Steam, even on an empty cache, so a scan never delays a game start (inferred; decision 24 names only the CLI and the TUI).

A wrapper launch never fails because spela's own data is bad (inferred). A config that fails to load or migrate warns and launches with the default config and no profile (see [Failure handling](#failure-handling)). A corrupt or missing game cache never blocks a launch either (decision 49). The wrapper warns, and detects the game without the cache: an AppID from step 1 or 2 is taken as it is, and step 3 matches the `steamapps/common/<folder>` path in the command against the app manifests of that Steam library, read in place without a full scan (inferred). It then loads `profiles/<appid>.toml` and launches. The wrapper detects the same way when the cache loads but does not know the game, for example a game installed after the last scan: the AppID from step 1 or 2 still finds its profile, so the game never launches with no profile only because the cache is behind (inferred, under decision 49). Go aborts here with `failed to load game database` and exit 1 (verified: CLI 5.2; `cmd/spela/main.go:65`), which decision 49 replaces. A corrupt game profile warns and falls back to All games. A corrupt All games profile warns and falls back to no profile. Today a corrupt game profile also drops valid defaults (verified: CLI conflicts, "Profile load error handling differs by path").

### One plan for dry run and launch

Both `launch --dry-run` and wrapper mode build one `LaunchPlan` from the game, the resolved profile, the config and the detected Proton build. The dry run prints it. The real launch prints it and then executes it (inferred). Because both read one value, the dry run shows every environment variable the launch sets. Today the dry run omits `PROTON_VKD3D_HEAP`, which Prepare adds (bug: [parity-inventory.md](parity-inventory.md), CLI 6.9 and domain conflicts, "Dry-run environment omits PROTON_VKD3D_HEAP").

The plan holds the environment as an ordered map, the privileged settings with their target values, the KDE VRR policy, the preflight notices, and the DLL status lines (denylist, backup, per-DLL write state).

The summary keeps today's order and wording (verified: [parity-inventory.md](parity-inventory.md), CLI 6.9), with these changes:

- The `overlay:` impact group and the Overlay block are gone (decision 10).
- Profile impacts print `source=<origin>` with the origin names `this_game`, `all_games` and `system_default`, in place of Go's second vocabulary `default`, `override` and `unset` (verified: CLI conflicts). `<n> profile-specific override(s)` counts the game's own keys, because there is no overrides map to count (inferred, under decision 8).
- The Hardware block and Profile impacts agree on zero values, and the Hardware block lists VRR (bug: CLI 6.9).
- The launch message says `Launching <Name> with profile...` only when the game has its own profile, `Launching <Name> with All games settings...` when only All games applies, and `Launching <Name> (no profile)...` otherwise. Today "with profile" also covers All games only (verified: CLI conflicts, "'with profile' wording"). The new wording is accepted (decision 60).
- When spela is not at `/usr/bin/spela`, the Hardware block says pkexec will use the generic action (inferred; the fallback itself is inferred in CLI 7.3).
- Clock and memory offsets show in the Hardware block as real offsets, because the helper applies them (decision 32; see [GPU offsets](#gpu-offsets)).
- While another launch holds the launch lock, the launch message adds that VRR and hardware are skipped (decision 33; see [Launch lock](#launch-lock)).

The dry run never runs pkexec or `kscreen-doctor`, as today (verified: CLI 6.5). Wrapper mode still prints the summary to stdout before each launch (verified: CLI 5.2). The TUI's Preview launch shows the same summary, built by the same function (decision 44).

### Environment and precedence

The child gets the parent environment plus spela's variables, set with `Command::env`, which replaces an inherited key instead of adding a duplicate (inferred, std behaviour). Today `BuildEnv` appends (verified: [parity-inventory.md](parity-inventory.md), domain 5) and relies on Go keeping the last duplicate (inferred: domain 5). Precedence, lowest first (inferred from today's order in CLI 6.2):

1. The parent environment.
2. Profile variables: Proton, DLSS and GPU, built as today (verified variable set: domain 5 and CLI 6.1).
3. Launcher variables: `PROTON_VKD3D_HEAP=1` when vkd3d_heap is on and Proton resolution fails or the build uses the legacy gate (verified rule: CLI 6.1).
4. The `KEY=VALUE` tokens from the wrapper arguments. They beat every spela value, as today (verified: CLI 5.2, pinned by `SPELA_USER_SETTING=keep`).

`SPELA_OVERLAY_IPC` is never set (decision 10).

`__GL_THREADED_OPTIMIZATION` is exported only when the game or All games sets the field (decision 47). Today any applied profile exports it, so a system default turns threaded optimisation off (verified: domain conflicts, "Threaded optimization is forced to 0"). This changes the launch environment of every game left at the system default, which decision 47 accepts.

The dxvk-nvapi values and the DLSS indicator variables stay exactly as Go emits them (decision 59; verified variable set: [parity-inventory.md](parity-inventory.md), domain 5). Whether dxvk-nvapi accepts every value is a user live check, and it changes nothing in the env builder.

A relative profile `gpu.shader_cache_path` is rejected on write and on load, like the config folder options. Today it is exported unchecked (verified: [parity-inventory.md](parity-inventory.md), domain conflicts). The fix is (inferred) under decision 8.

### Preflight

The vkd3d_heap check runs only when vkd3d_heap resolves on. It logs and never blocks, with today's messages (verified: [parity-inventory.md](parity-inventory.md), CLI 6.3 and 9). Its notices also go into the plan, so the dry run shows them.

### Sequence

Wrapper mode runs these steps in order (inferred, keeping today's order of effects from [parity-inventory.md](parity-inventory.md), CLI 6.1 and 6.8):

1. Refuse to run as root, with an error and exit 1 (decision 35; the exit code is inferred). Install the signal watcher. SIGINT, SIGTERM, SIGHUP and SIGQUIT are caught from here on (SIGQUIT as built, so Ctrl-backslash in a terminal no longer skips the restores). Today they are caught only after Prepare's pkexec and VRR steps (bug: CLI 6.7).
2. Load the config, the game cache and the profiles, running the migration if a Go file is new. Detect the game. Build the plan. Print the summary.
3. If the plan has privileged or VRR changes, try the launch lock without blocking (see [Launch lock](#launch-lock)). If another launch holds it, drop VRR and the privileged changes from the plan, warn, and go on with the env-only settings (decision 33).
4. Apply KDE VRR in the user session. Push its restore onto the cleanup stack, even when the apply returns an error, as today (verified: domain 4).
5. Push the hardware restore onto the cleanup stack, then start the `--serve` helper with one `pkexec` and send it the batch (see [Privilege model](#privilege-model) and [Protocol](#protocol)). The helper applies and reports each step, then waits.
6. If a stop signal arrived during steps 2 to 5, do not start the game. Run cleanup, release the lock, and exit with 128 plus the signal number (inferred). As built, a stop signal that is already pending also skips steps 4 and 5, so no polkit prompt shows for a launch that is already stopping.
7. Print the launch message and spawn the child with inherited stdin, stdout and stderr (verified: CLI 6.7). The child inherits neither the lock descriptor nor the write end of the helper's stdin pipe, because std opens files and pipes with close-on-exec (inferred). That matters: if the game held the write end, the helper would never see EOF when the wrapper dies.
8. Forward every SIGINT, SIGTERM, SIGHUP and SIGQUIT to the child, not only the first. Today later signals are swallowed and SIGHUP is not handled (bug: CLI 6.7). spela signals the direct child only, because Steam's reaper sits in `%command%` and handles the tree (inferred in CLI 6.7). process-wrap stays a later pick ([rust-crates.md](rust-crates.md), processes).
9. Wait for the child and keep its exit status. As built, the wrapper first waits with `waitid(WNOWAIT)`, then stops forwarding, then reaps, so a signal that comes as the game exits never reaches another process that got the freed PID.
10. Run cleanup, release the launch lock, and exit with the game's status. The hardware step of cleanup sends `restore` to the helper and reads its report lines until it exits.

A failure to spawn runs cleanup and returns the error, as today (verified: CLI 6.7). A panic must not skip the restore: the launch body runs under `catch_unwind` with cleanup after it, which is why the release profile keeps `panic = "unwind"` (inferred).

### Cleanup

- Steps carry an area label and run in reverse registration order. The order is now hardware, then KDE VRR, because the overlay step is gone (verified order: [parity-inventory.md](parity-inventory.md), CLI 6.8; decision 10).
- Each step logs `restore succeeded area=<a>` or `restore failed area=<a> error=<e>`, and the next step still runs (verified: CLI 6.8).
- The list is cleared afterwards, so cleanup runs once (verified: CLI 6.8).

### Launch lock

Decision 29 adds a launch lock around apply and restore. It is an flock on `<state>/locks/launch.lock` (see [Locks](#locks)), with the PID written in it for the "already running (PID: N)" message (verified pattern: [mejl-study.md](mejl-study.md), binary 11; [rust-crates.md](rust-crates.md), file locking). Today no launch path takes any lock (verified: [parity-inventory.md](parity-inventory.md), CLI 5.2 and 8).

- The lock is per user: it lives in the caller's own XDG state dir, while the hardware it guards is machine-wide. A second user, the same user with another `XDG_STATE_HOME`, or `sudo spela gpu reset` (which runs the helper directly as root, with root's own lock) does not see it, so those launches can still interleave. This gap is accepted. A machine-wide lock taken by the root helper, such as `/run/spela/hardware.lock`, would close it, but the helper writes no file (see [Protocol](#protocol)), and only the user's live check can show how pkexec and the helper behave with it. A `sudo -E` run can also leave root-owned files in the user's state dir, the lock among them, after which a wrapper launch cannot open the lock and goes on env-only with `cannot take the launch lock`.
- Wrapper mode takes it before the first VRR or privileged change and holds it until the last restore step ends (inferred reading of decision 29). One gap is accepted: if the wrapper is killed with SIGKILL, the kernel drops the lock at once while the helper's EOF restore may still run, so a launch that starts in that moment can read a half-restored state (inferred). Two short holds, one around apply and one around restore, would let a second launch read the first launch's applied state as its prior state, and the restores would then undo each other in the wrong order (inferred). [parity-inventory.md](parity-inventory.md) (CLI open questions) asks the lock to stop exactly that interleaving.
- The live hardware commands (`gpu reset`, `cpu governor <g>`, `cpu smt <v>`) take it too, so they cannot interleave with a running game's restore (inferred). If it is held, they exit 5.
- The TUI's hardware jobs take it for the length of the job, as the CLI commands do (decision 44). If it is held, the job fails with `spela is already running (PID: N)` and changes nothing.
- `launch --dry-run` and the TUI's Preview launch do not take it.
- A second wrapper launch while the lock is held never blocks. It launches with the env-only settings, skips VRR and the privileged changes, and warns on stderr and in the log, for example `spela is already running (PID: N); launching without VRR and hardware changes` (decision 33; the text is inferred).

### Privilege model

- Privileged changes go through one batched `pkexec /usr/bin/spela apply-profile` call ([../AGENTS.md](../AGENTS.md) hard rule; today's shape, verified: [parity-inventory.md](parity-inventory.md), CLI 7.1 and 7.2). For a launch that call is the long-lived `--serve` helper, which also does the restore, so a launch shows one prompt at most (decision 30). For a live command it is the one-shot `--once` helper.
- The hidden `apply-profile` command is the only code that calls NVML setters, writes CPU sysfs files or needs root (inferred). The unprivileged process never runs `pkexec tee` or `pkexec nvidia-smi` again, as today's live commands do (verified: domain conflicts, "Per-CPU pkexec outside apply-profile").
- The polkit action id `io.github.spela.apply-profile`, its message, its defaults and the `/usr/bin/spela` path stay (contract, verified: CLI 7.3). `/usr/bin` matters because Steam runs launches with `PATH=/usr/bin:/bin` (verified: CLI 7.3).
- The policy is scoped to the `apply-profile` argument. The action keeps its id and annotates `org.freedesktop.policykit.exec.path` as `/usr/bin/spela` and `org.freedesktop.policykit.exec.argv1` as `apply-profile` (decision 35). Today the action annotates only the executable path, so with `auth_admin_keep` it matches wrapper mode running any program (verified: CLI 7.3 and CLI conflicts). That pkexec honours `argv1` on the user's polkit version, so `pkexec /usr/bin/spela <anything else>` falls back to the generic action, is a user live check (unverified: [parity-inventory.md](parity-inventory.md) names the annotation only as an example). Wrapper mode refuses to run as root as a further guard (decision 35).
- The source install recipe installs the policy to `/usr/share/polkit-1/actions/` next to the binary in `/usr/bin`. Today only the PKGBUILD installs it, so a source install gets the generic action without `auth_admin_keep` (verified: CLI 7.3 and CLI conflicts). The user runs that recipe. No agent runs it ([../AGENTS.md](../AGENTS.md) hard rule).

### Protocol

The Go flag form may change (verified: [parity-inventory.md](parity-inventory.md), CLI 7.2), and Rust does not accept it (decisions 13 and 34). Today results come back only as the exit code and stderr, with cobra's usage text mixed in (verified: CLI 7.2). JSON lines replace that. Decision 30 sets the shape. The field names below are a sketch (inferred).

`apply-profile --serve`, for a launch:

1. The parent spawns `pkexec /usr/bin/spela apply-profile --serve` with stdin and stdout as pipes. pkexec passes stdin through, and today's live CPU path relies on that with `pkexec tee` (verified: CLI 7.1).
2. The parent writes one line, the batch: `{"version": 1, "type": "apply", "steps": [{"step": "clock_offset", "value": 150}, {"step": "governor", "value": "performance"}]}`.
3. The helper checks that it runs as root and validates every value, then works through the steps in the order in [Partial-failure rules](#partial-failure-rules). For each one it reads the prior value as root, applies, and writes one line: `{"type": "step", "step": "clock_offset", "prior": 0, "outcome": "applied"}`. The outcome is `applied`, `failed` with an `error`, or `skipped` with a `reason`, for example when the prior value could not be read. The parent reads each line as it comes, so it knows what was applied even if the helper dies partway.
4. The helper writes `{"type": "ready"}` and waits on stdin. The game runs.
5. At game exit the parent writes `{"type": "restore"}`. The helper restores each applied step from its prior value, in reverse order, writes one `{"type": "restored", "step": ..., "outcome": ...}` line per step and then `{"type": "done"}`, and exits 0.
6. EOF on stdin at any point after the batch means the wrapper is gone: it exited, panicked or was killed. The helper restores what it applied, as in step 5, and exits. EOF before the batch exits with nothing changed. A line that does not parse, or any request other than `restore`, counts as a broken parent and is handled like EOF, so the helper never accepts a second batch.
7. EPIPE on stdout also means the parent is gone. The helper finishes or rolls back the current step, then restores (see [Output, errors and exit codes](#output-errors-and-exit-codes)).

The waiting helper must outlive anything that could stop it before it restores (inferred):

- It ignores SIGINT and SIGHUP, so Ctrl-C in a terminal or a closed session does not kill it before the wrapper asks for the restore, or before EOF. As built, it also ignores SIGQUIT, SIGTSTP, SIGTTIN and SIGTTOU, and right after the root check it calls `setsid()`, so the signals a terminal sends its foreground group (Ctrl-C, Ctrl-backslash, Ctrl-Z, a hangup) no longer reach it at all. pkexec has finished its prompt by then, and the helper reads only its pipes. `--once` does the same and ignores SIGINT, SIGHUP and SIGQUIT, so a Ctrl-C at the CLI cannot stop a live change halfway through the per-CPU governor writes.
- A panic in the helper does not skip the restore: each machine call runs under `catch_unwind` while the list of applied steps stays outside it. A panic is reported as that step's failure, the batch stops there, and the steps applied so far are restored, the panicking step included when its prior was read.
- SIGTERM makes it restore and exit, as EOF does.
- SIGKILL cannot be caught. A root helper killed that way leaves the hardware as it is, which the next `spela gpu reset` or a reboot clears. This is the one case with no restore.
- EOF reaches the helper whether pkexec execs it or stays as its parent, because the helper reads the same pipe either way (inferred). Which of the two pkexec does is part of the user's live check.

`apply-profile --once <request>`, for the live `gpu reset`, `cpu governor <g>` and `cpu smt <v>` in the CLI and the TUI:

- The request is one JSON argument, of the same shape as the batch, because the TUI runs pkexec with a null stdin and the CLI leaves stdin on the terminal for pkexec's text agent (decision 44). The argument is not secret, so `ps` showing it is fine (inferred).
- The helper validates, applies, writes the same step lines, and exits 0 when every step applied, or 1 otherwise. It never restores: a live command is meant to last, as in Go (verified: CLI 3.9 and 3.10). So, as built, a prior that cannot be read never skips a `--once` step. It is read only for the report, and a step applied without one is reported as `applied_no_restore`. A card that cannot read its VF offsets still gets them zeroed by `gpu reset`.

In both modes:

- Exit code 126 or a cancel message from pkexec means the user dismissed the prompt, as today (verified: CLI 7.1). Then nothing changed and no restore is needed.
- Any other argument form, Go's `--reset` and flag forms, `--help`, `-h` and a bare `--once` included, prints `apply-profile: this is the spela 0.7 helper; the Go helper's flags are not supported` to stderr and exits 2, with no change made (decision 34; the text is inferred). clap never sees the helper's arguments. The helper runs only when `apply-profile` is the first argument, the form the polkit `exec.argv1` annotation scopes. `spela help apply-profile ...` and any global flag before it, `--demo` included, get the same message and exit 2 without running it, because the helper ignores `--demo` and `SPELA_BACKEND=fake` (as built). The removed `overlay` and `gui` get the same treatment: every form, `--help` and `spela help overlay` included, prints the removal error and exits 2 (decision 45).
- The helper writes no file and no log. The parent logs every line it reads.

As built in Phase 6 (`spela-core::helper`; the binary adds root, stdin, the signals, NVML and `/sys`):

- Lines: the batch is `{"version": 1, "type": "apply", "steps": [{"step": <name>, "value": <value>}]}`. Step names are `locked_clocks` (no value, `--once` only, because nothing restores it), `clock_offset`, `memory_offset`, `power_limit`, `fan` (a percent or `"auto"`), `governor` and `smt` (a bool). A step reply is `{"type": "step", "step", "target", "outcome", ...}` with `prior` for `applied`, `error` for `failed` and `reason` for `skipped`, plus `applied_no_restore` for the locked clocks reset. A refused request gets `{"type": "error", "error": ...}` and changes nothing.
- Parsing refuses the whole batch for an unknown key, an unknown or repeated step, a missing value or a value of the wrong JSON type or range. The per-step checks then skip a step only: the governor must be listed by every online CPU's `scaling_available_governors` and be a plain name, the power limit must sit in NVML's constraints (an unreadable range skips the step), the fan must be 0 to 100, and the offsets must sit in spela's own bounds, -1000 to 1000 MHz for the clock and -3000 to 3000 MHz for memory, because nvml-wrapper 0.13 reads no VF offset range. The root test settles the real bounds (decision 32).
- `--serve` reads stdin on a thread and SIGTERM on another, both into one channel. Anything that arrives while the batch applies stops it before the next step. Every way out after the batch restores. Exit codes: 0, 1 when a restore failed (or a `--once` step did not apply), 2 for a refused request.
- The governor prior is read and restored per CPU. The report shows the distinct names, joined with `, `. A governor write that fails on one CPU puts back the CPUs it already wrote, so a failed step changed nothing.
- The parent's side (`helper::client`) maps a helper that wrote no line to the pkexec rules above, treats a `restored` line before `ready` as a helper that lost its input and restored on its own, and on drop closes the helper's stdin and waits for its restore to end.

### Restore registered before the game starts

The hardware restore is pushed onto the cleanup stack before the helper starts. It sends `restore` to the waiting helper, which restores exactly the steps it reported as applied, to the prior values it read as root (decision 30). If the wrapper never gets to run that step, the helper restores on EOF instead. This fixes two Go bugs (verified: [parity-inventory.md](parity-inventory.md), CLI 7.2 and domain conflicts, "Failed privileged apply leaves partial state with no restore"):

- A failed or partial apply registers no restore today.
- Prior state is read unprivileged, and read errors become zero values.

### Partial-failure rules

These are (inferred), each fixing a behaviour listed in [parity-inventory.md](parity-inventory.md), domain 4 and the domain conflicts:

- Validate all values before any change. An invalid value skips that setting only, and the result says why. The governor must be in sysfs `scaling_available_governors` (today's check, verified: CLI 7.2). The power limit must be within the NVML power-limit constraints, which `gpu info` already reads (verified: domain 10). The fan speed must be 0 to 100. Offsets must be within a range NVML reports, if it reports one (unverified). Otherwise the live root test sets the bounds. Until then the helper uses its own bounds (see [Protocol](#protocol), as built). Today there are no range checks, and negative values wrap through uint32 (bug: domain conflicts, "Negative memory offset is not guarded").
- In a launch (`--serve`), a setting whose prior value cannot be read is not applied. spela never restores a value it did not read. A live `--once` change is never restored, so it applies without a readable prior (as built). Today an unreadable SMT state restores SMT off (bug: domain conflicts, "SMT restore defaults to off").
- Apply steps run in today's order: graphics, memory, power, fan, governor, SMT (verified: CLI 7.2). A failed step does not stop the independent steps after it. Today the helper stops at the first failure and leaves earlier changes in place (verified: domain 4).
- Restore steps run in reverse, each independently, so one failure does not block the rest. Today the reset returns at the first error (bug: domain conflicts, "Reset always resets clocks first and aborts at the first error").
- Restore touches only what was applied. Today every restore resets locked clocks first, even for a governor-only profile (bug: same conflict; [rust-crates.md](rust-crates.md), conflicts).
- SMT is restored before the governor, so re-onlined CPUs get the restored governor. Today the governor comes first (domain conflicts, "Governor writes assume contiguous online CPUs"; the effect needs a live test).
- The fan returns to its prior state when NVML can report it, otherwise to automatic. Today a prior manual fan speed is lost (bug: CLI 7.2). Whether NVML reports the fan control mode is (unverified).
- A dismissed prompt or a failed apply lets the game launch, with `failed to apply hardware settings` logged, as today (verified: CLI 6.1). The line names only the steps that failed. A step the helper skips, such as an invalid value (decision 67), is logged as `hardware step skipped` with its reason and is not a failure: the other steps apply and restore.
- The privileged apply runs only for `clock_offset` other than 0, `memory_offset` other than 0, `power_limit` above 0, `fan_speed` above 0, a governor, or a set SMT, as today (verified: [parity-inventory.md](parity-inventory.md), domain 4; `internal/profile/apply.go:65-72`). An own `fan_speed` of 0 adds no privileged step. It only stops All games' fan speed from applying, so the fan stays as it is, usually automatic (inferred).

### Restore at game exit: the long-lived helper

Decision 22 asked for a prototype of two designs. Decision 30 picks the long-lived helper, and the two-pkexec design is not built:

| Design | How it works | Why |
|---|---|---|
| Long-lived helper (built) | One `pkexec` starts `apply-profile --serve`. It applies, reports, then waits on its stdin. When the parent writes `restore`, or the pipe closes because the parent died, it restores and exits (see [Protocol](#protocol)) | One prompt per launch. The restore needs no second prompt, so it works after `auth_admin_keep` has expired and with no polkit agent at game exit. It also runs when the wrapper is killed or panics, because EOF triggers it |
| Two pkexec calls (not built) | One call applies at launch. A second call restores at game exit (today's design, verified: [parity-inventory.md](parity-inventory.md), CLI 7.2) | A second prompt at exit, which may fail silently under gamescope or Big Picture with no agent (inferred in CLI open questions), and no restore if the wrapper dies between the calls (inferred) |

The cost is a root process that lives for the whole session. It accepts no request but `restore`, writes no file, and holds nothing but the prior values (inferred).

The prompt-count spike stays a user follow-up. The user's live launch check confirms the design, under desktop Steam, a gamescope session and Big Picture (see [Open questions](#open-questions)):

- one launch shows one prompt at most, including after the `auth_admin_keep` window has expired;
- what happens with no polkit agent running, and whether pkexec falls back to a text agent on a tty (unverified);
- the restore runs after a normal game exit, after Steam's Stop button (unverified what Steam sends to the wrapper then), after a SIGKILL of the wrapper, and when the user logs out during the game.

Under the hard rules in [../AGENTS.md](../AGENTS.md), only the user runs this, on their machine. No agent or test runs pkexec. The check must run under the real polkit action, `io.github.spela.apply-profile`. That action annotates `/usr/bin/spela`, and a helper at any other path falls back to the generic `org.freedesktop.policykit.exec` action with no `auth_admin_keep` (verified: [parity-inventory.md](parity-inventory.md), CLI 7.3). So the user installs the binary at `/usr/bin/spela` with the policy from `data/polkit`, as root. A run under the generic action does not count (inferred).

### GPU offsets

Decision 9 makes `clock_offset` and `memory_offset` real NVML offsets. Today "clock offset" locks graphics clocks to 0 to 2100 plus N MHz, and "memory offset" is an absolute maximum memory clock (verified: [parity-inventory.md](parity-inventory.md), CLI 7.2 and domain 1). nvml-wrapper 0.13 has `set_gpc_clock_vf_offset`, `set_mem_clock_vf_offset` and `set_clock_offset` (verified: [rust-crates.md](rust-crates.md), NVML).

The helper wires `set_gpc_clock_vf_offset` and `set_mem_clock_vf_offset` behind `GpuControl`, reads the prior offsets as root, and restores them (decision 32). They are tested on fakes only, and this doc marks them live-unverified until the user's root test. That test checks, on the user's GPU and driver, that each setter succeeds, that the offset reads back, and that restoring the prior offset works. `gpu reset` sets both VF offsets to 0 and resets locked clocks (inferred, now that decision 32 wires the offsets). Today it only resets locked clocks (verified: CLI 3.9). Go's stored values are not migrated into the new keys (decision 31; see [What each file becomes](#what-each-file-becomes)).

## Hardware

### NVML

- nvml-wrapper 0.13.0 dlopens `libnvidia-ml.so.1` at runtime. A missing driver is a plain error, and the binary starts on systems without NVIDIA (verified: [rust-crates.md](rust-crates.md), NVML).
- GPU 0 only (decision 25; today's behaviour, verified: [parity-inventory.md](parity-inventory.md), domain 10).
- Opened lazily, with one owner per process (see [Root helper and CLI](#root-helper-and-cli)). The device is looked up per call, because `Device` borrows `Nvml` and setters take `&mut Device` (inferred in [rust-crates.md](rust-crates.md), NVML).
- Init took 4.1 to 16.6 ms, and a 12-query sweep took about 7 ms in the driver (verified: [rust-crates.md](rust-crates.md), NVML). So the metrics thread owns the reads, and the UI thread never calls NVML ([../AGENTS.md](../AGENTS.md) hard rule).
- The sweep reads what today's metrics read: temperature, power use, power limit, utilisation, memory used and total, graphics and memory clocks, fan speed and the four throttle reasons (verified: [parity-inventory.md](parity-inventory.md), domain 10). The same worker reads the CPU metrics.
- `gpu info` keeps its lines and formats, including the en dash in the power range (verified: [parity-inventory.md](parity-inventory.md), CLI 3.9 and domain 10).
- Setters run only inside the root helper (see [Privilege model](#privilege-model)).
- The driver version for the vkd3d_heap check comes from NVML's system driver version. The wrapper and `proton show` open NVML lazily for this one read (inferred). Go reads it with `SystemGetDriverVersion`, falls back to `nvidia-smi`, and returns an empty string when both fail (verified: `internal/gpu/driver_version.go`). If NVML fails, the check takes Go's no-driver path. It logs `vkd3d_heap: NVIDIA driver not detected; skipping driver compatibility check` (verified: `internal/launcher/launcher.go:153`), and the notice reads `NVIDIA driver not detected; skipping driver compatibility check` (verified: `internal/proton/notice.go:179`).
- There is no `nvidia-smi` fallback: NVML only (decision 58). It is in nvidia-utils with NVML, and fan control never had a fallback (verified: [parity-inventory.md](parity-inventory.md), domain 10). When NVML is missing, every GPU feature says so: `gpu info` reports the NVML error, the top bar and Monitor show GPU metrics as unavailable with the reason, GPU hardware actions are unavailable, and the vkd3d_heap check takes the no-driver path above (inferred wording).

### CPU

- sysfs and procfs are read with std fs (verified pick: [rust-crates.md](rust-crates.md), procfs and sysfs). Paths are today's: cpu0 `scaling_governor` and `scaling_available_governors`, per-CPU `scaling_governor`, `smt/active` and `smt/control` (verified: [parity-inventory.md](parity-inventory.md), domain 11).
- CPUs are listed from the `cpuN` folders in sysfs, and only online CPUs with a cpufreq folder are written. Today the loop runs 0 to `runtime.NumCPU()-1`, which is affinity-limited and misses offline or non-contiguous CPUs (bug: domain conflicts, "Governor writes assume contiguous online CPUs"; additions B.2). The prior governor is read per CPU, not from cpu0 only (inferred).
- A governor is valid when sysfs lists it. Today a second, hard-coded list also applies, so `conservative` passes one check and fails the other (verified: additions B.2). Dropping the hard-coded list is (inferred). `cpu.governor` is then validated on load and in the TUI picker against the sysfs list, read off the UI thread and cached. A stored value that sysfs does not list is kept and reported, not rejected (inferred).
- Writes happen only in the root helper.
- `cpu info` keeps its lines (verified: CLI 3.10). Its load figure stays load1 divided by CPU count, capped at 100, and the code says so. Today a comment claims `/proc/stat` (verified: domain conflicts, "CPU 'utilization' is load average").
- SCX status stays: `scx_loader` on PATH, then `systemctl is-active scx.service` (verified: domain 11).
- `cpu.affinity` is gone (decision 10).

### KDE VRR

Today's behaviour is a contract and stays (verified: [parity-inventory.md](parity-inventory.md), CLI 6.5 and domain 9; [kde-vrr.md](kde-vrr.md) matches the code):

- It runs in the user session, never under pkexec.
- Policies are automatic, always and never. Empty and `unset` do nothing.
- It needs `XDG_SESSION_TYPE=wayland` and KDE in `XDG_CURRENT_DESKTOP`.
- `kscreen-doctor -o` is parsed with ANSI stripped. Anything missing or duplicated fails closed. Exactly one output must have priority 1, enabled and connected.
- The set command is `output.<uuid>.vrrpolicy.<policy>`. `applying config failed!` in stdout is an error.
- Restore finds the output again by UUID and never re-targets the current primary.
- Each call has a 10 s timeout, through wait-timeout, with `LC_ALL=C` (verified pick: [rust-crates.md](rust-crates.md), processes).
- The dry run never calls it.

The restore is registered even when the apply returns an error (verified: domain 4). No D-Bus. zbus is avoided (verified: [rust-crates.md](rust-crates.md), D-Bus). The step runs before the privileged apply and restores after it, as today (verified order: CLI 6.8). Claude ran the read-only `kscreen-doctor -o` in Phase 0 (decision 63). It needs the desktop session's Wayland socket and aborts with SIGABRT without one, it writes ANSI codes even to a pipe, and its output has every line Go's parser reads (verified: [phase0-checks.md](phase0-checks.md), kscreen-doctor -o). The parser fixture is `tests/fixtures/kscreen/doctor-o.txt`. Applying a VRR policy changes the display, so that check is the user's.

### Alert evaluator

The overlay IPC writer, its 500 ms collector and `SPELA_OVERLAY_IPC` are gone, and the evaluator stays (decision 10). It moves to `spela-core::alerts`.

- `evaluate(input, thresholds)` stays a pure function with today's rules and messages (verified: [parity-inventory.md](parity-inventory.md), DLL 4.5): a thermal throttle reason is critical; otherwise 85 °C or more is critical and 80 °C or more is a warning; a power throttle reason, or draw at 95 % of the limit or more, is a warning; fans at 95 % or more is info.
- The severity and type strings and the labels "High temp", "Throttling", "Power limited" and "Fan max" stay (verified: DLL 4.5).
- The metrics thread runs it on each sweep. The top bar shows the highest alert as a word, and that word is never dropped at narrow widths ([terminal-ux.md](terminal-ux.md), frame). The Monitor screen's Alerts list uses the same output.
- Two fixes (inferred, and the visible change is accepted by decision 60):
  - "Throttling" is used only when NVML reports a thermal throttle reason. Reaching 85 °C without one reads "High temp" at critical severity. Today 86 °C with no throttle reason says "GPU thermal throttling" (verified: DLL conflicts, "Alert precedence comment vs behaviour").
  - The power suggestion is capped at the GPU's maximum power limit, which NVML reports. Today it always says limit x 1.1 (verified: DLL conflicts, "Power alert suggestion ignores the max limit").
- The unused graphics clock input is dropped (verified unused: DLL 4.5; dropping it is inferred).

## Steam and Proton

### Libraries and manifests

- The Steam root is the configured `steam_path`, else the first of `~/.steam/steam`, `~/.local/share/Steam` and the Flatpak path (verified: [parity-inventory.md](parity-inventory.md), domain 7).
- VDF files are parsed with keyvalues-parser 0.2.4. It parsed the real `libraryfolders.vdf`, all 59 real app manifests and the 105 KiB `config.vdf` on this machine (verified: [rust-crates.md](rust-crates.md), Steam VDF).
- Its grammar differs from spela's line parser: it unescapes `\\`, keeps a pair that has a trailing `//` comment, accepts `"key" {` on one line, and keeps every duplicate key (verified: same). spela takes the last value of a duplicate key, which matches Go (verified: same). The other three differences are accepted, because the new parser reads more real files correctly (decision 52).
- Libraries come in file order, then the configured extra paths, deduplicated by exact string (verified rule: domain 7). If the same AppID is in two libraries, the first library wins, with a log line (inferred). Today Go's map order decides (bug: domain conflicts, "Library order and duplicate app ids are not deterministic").
- A broken `libraryfolders.vdf` no longer stops the scan of the configured extra libraries. The error is reported and the extra paths are still scanned (inferred). Today it aborts both (verified: domain conflicts, "A broken libraryfolders.vdf aborts scans").
- An app counts as installed when the fully-installed bit (4) is set in StateFlags, not only when StateFlags equals 4 (inferred). Today an installed game with another flag set, such as an update pending, drops out of the library (verified: domain conflicts, "Installed check uses StateFlags equal to 4"). On this machine every manifest has StateFlags 4 (installed) or 6 (installed plus update required), and Go's equality test drops the three games with a pending update (verified: [phase0-checks.md](phase0-checks.md), StateFlags). Fixtures cover both, including the sandbox's `Pending Patch` with StateFlags 6.
- Tools are dropped as today: the 32 known AppIDs, a `toolmanifest.vdf` in the install folder, and the tool name patterns (verified: domain 6 and 7).
- The prefix is `compatdata/<appid>`, valid when `pfx/drive_c` is a folder (verified: domain 7).
- Scanning runs once in the background. The TUI renders from the game cache at once. The CLI rescans only on an empty cache or `spela scan` (decision 24). Today `list`, `show` and `tui` rescan, and the TUI scans twice at start (verified: CLI 3.2 and TUI 13). A missing Steam install gives the empty library instead of an error (verified Go bug: TUI conflicts).

### Proton

- The resolver reads `config/config.vdf` at `InstallConfigStore > Software > Valve > Steam > CompatToolMapping` and tries the AppID, then `"0"` (verified: [parity-inventory.md](parity-inventory.md), domain 8). The real file uses exactly that key casing, with one spelling (verified: [phase0-checks.md](phase0-checks.md), CompatToolMapping key casing). Keys are still matched without regard to case while walking the tree, which costs nothing (inferred).
- The mapped name is a tool's internal name, not its folder name. Go looks for `compatibilitytools.d/<name>` and `steamapps/common/<name>` under the Steam root only, and on this machine that resolves no mapped tool at all (verified: [phase0-checks.md](phase0-checks.md), Proton tool folders). The Rust resolver therefore:
  - reads the internal name from each `compatibilitytools.d/*/compatibilitytool.vdf` under the Steam root, so `GE-Proton11-7-x86_64` finds the `GE-Proton11-7` folder;
  - searches `steamapps/common` in every library, not only the Steam root's, because most tools sit in a second library;
  - maps the built-in short names such as `proton_experimental` and `proton_9` to their tools through the tools' app manifests, so `proton_experimental` finds `Proton - Experimental` (inferred, not tried on this machine).
- `/usr/share/steam/compatibilitytools.d` is not searched. It does not exist on this machine, and Go never looked there (verified: [phase0-checks.md](phase0-checks.md), Proton tool folders). A mapping with no folder anywhere, like the three Proton-CachyOS builds here, gives the "could not resolve active Proton" notice as today.
- Fixtures cover these shapes: a GE folder whose internal name differs, and a built-in mapping with a `Proton - Experimental` folder (`tests/fixtures/go/sandbox/main/home/.steam/steam/`).
- The legacy gate, the CachyOS tag pattern and comparison, the driver version parser, the `580.94.16` minimum and the notice strings stay (verified: domain 8). The vkd3d_heap field text is corrected to say 11.0-20260520 or later, matching the code (verified: domain conflicts, "VKD3D heap description overstates 11.0 support").

## DLLs

### What stays

- Detection by filename, case-insensitive, keeping the on-disk case: dlss, dlssg, dlssd, xess and fsr (both `amd_fidelityfx_vk.dll` and `amd_fidelityfx_dx12.dll`) (verified: [parity-inventory.md](parity-inventory.md), DLL 2.1). The catalogue order stays, because the TUI deployment table depends on it (verified: DLL 2.1).
- The walk does not follow symlinks, through walkdir (verified pick: [rust-crates.md](rust-crates.md), regex, PATH lookup and directory walking).
- Versions come from `VS_FIXEDFILEINFO` FileVersion, through a port of Go's `.rsrc` scan on `object`. It agreed with pelite and goblin on 1,224 real DLLs (verified: [rust-crates.md](rust-crates.md), PE version resources). The formatting rules stay (verified: DLL 2.2).
- One shared version module does the dotted compare: one leading `v` stripped, non-numeric parts read as 0, the shorter version padded (verified: DLL 2.3). The release check, the TUI and `dlss-updater` use it too.
- The domain owns resolve, acquire, back up, replace, rescan and save, emits progress events with today's stages, and returns a typed partial failure. Adapters own confirmation and rendering (verified contract: DLL 2.6 and 2.12).
- Backups: the first backup of a path is kept until Steam replaces the file (decision 37), every detected DLL of a game is backed up on its first swap, and a malformed index fails closed (verified: DLL 2.7). The on-disk shape is in [DLL backups](#dll-backups).
- Writes are atomic with an fsync of the file and its folder (see [Writing a file](#writing-a-file)). The download runs before `games.lock` is taken (see [Locks](#locks)).

### Hazards fixed

Each fixes an item in [parity-inventory.md](parity-inventory.md), DLL conflicts, under decision 8. The designs are (inferred).

| Hazard today | Rust behaviour |
|---|---|
| "Latest" is the first manifest entry, so an older version inserted at index 0 becomes latest (DLL 2.4) | Latest is the highest version by the shared compare |
| The denylist fails open on a malformed or unreadable `denylist.yaml` or `overrides.yaml` (DLL 3) | It fails closed (see [Deny lists](#deny-lists)) |
| A user deny list replaces the built-in list instead of adding to it (DLL 3 and DLL conflicts, "Denylist fails open") | The user list adds to the built-in list, and allow and deny for one game clear each other (decision 36; see [Deny lists](#deny-lists)) |
| The manifest version is joined into the cache path unchecked (DLL conflicts, "Unvalidated manifest version") | An entry's version must be 1 to 4 dot-separated numbers and its key a known type, or the entry is rejected before any path is built |
| Install matches the detected file name with exact case, so `NVNGX_DLSS.DLL` gets a second lowercase file (DLL conflicts, "Case handling") | Install matches without regard to case and writes over the detected file |
| A manifest filename that is not a real DLL name, such as `dlss.dll` from `dll-release.yml`, is installed as is (DLL conflicts, "dll-release.yml filename") | Install and update require the entry's filename to be a known name for its type |
| FSR maps two files to one manifest key, so a dx12 file could get vk bytes (DLL conflicts, "FSR maps two files") | A payload is only written to a file with the same name as the entry's filename |
| `dll list` and `check-updates` skip XeSS-only and FSR-only games (corrections) | Both include every type |
| Five meanings of "update available" (DLL 2.10) | One: a DLL is stale when its installed version is known and lower than the highest manifest version for its type. Library, Catalog, `dll check-updates` and auto-update call one function (decision 38). An unknown version shows as "unknown". An explicit update still swaps it, and auto-update skips it |
| The manifest body is read with no size cap (DLL conflicts, "Manifest fetch has no body cap") | A cap, as the release check has. The size is (inferred), for example 4 MiB |

### Restore

Restore keeps today's preflight, atomic replace, no denylist guard and `PartialFailure` on a failed later replace (verified: [parity-inventory.md](parity-inventory.md), DLL 2.6). Decision 37 settles the two semantics that [parity-inventory.md](parity-inventory.md) left open (DLL open questions):

- Files that install created are removed by restore. Install records each file it creates where none existed in the new top-level `created_files` key of the backup index (see [DLL backups](#dll-backups)), so an install into a game with no DLLs still leaves a backup entry. Today that install makes no backup, so restore fails with "no backup found" and cannot remove the file (bug: DLL conflicts, "Restore and backup refresh semantics"). As built, restore deletes a created file only while its SHA-256 still matches what spela recorded installing there. A file that Steam has since replaced is the game's own: restore keeps it, drops it from `created_files` and `installed`, and lists it as `kept` in `dll restore`'s output. A created file with no recorded hash is deleted, as before. A backed-up path whose file Steam updated after the install still gets its old original copied back, as in Go.
- Go's `LoadBackup` reads an index with the new keys: run on indexes the Rust binary wrote, it loads one with `installed` and its Go entries. An index that lists only `created_files`, written by an install into a game with no DLLs, has an empty `files` list, so Go refuses it as "no files" and changes nothing (verified with Go's `internal/dll` on 2026-10-26, Phase 5). A restore that leaves an index with nothing to restore removes it, and one that leaves Go's keys alone writes nothing, so a Go-era `backup.json` is byte-identical after a Rust update and restore (as built, `crates/spela-core/tests/dll_ops.rs`).
- Writes stay in the game: every target must be inside the game's install folder by path and by real folder, and a DLL that is a symlink is refused, never followed (as built).
- The backup is refreshed after Steam updates a DLL. spela records the SHA-256 of each file it installs, in the new top-level `installed` key. Before an install or update, if the file on disk matches neither the backup nor that hash, Steam replaced it, so spela backs it up as the new original first. Today restore rolls back to the older original (verified: same conflict).

### Manifest fetch

- The URL stays `https://raw.githubusercontent.com/jgabor/spela/main/data/manifest.json`, because every installed Go client fetches it (decision 14; verified: [parity-inventory.md](parity-inventory.md), DLL 1.1).
- The schema stays at version 1, and the time fields are optional when parsing (verified schema and advice: DLL 2.4 and 10).
- The fetch policy stays: the configured URL only if it is https or http to loopback, a cache that is reused when its source matches and it is younger than `manifest_refresh_hours`, and a fall back to the cache when a fetch fails (verified: DLL 2.4).
- HTTP is blocking ureq on a worker, with the timeouts in [Backends and fakes](#backends-and-fakes) (verified mapping: [rust-crates.md](rust-crates.md), HTTP and TLS; decision 11).
- Downloads stream through a SHA-256 hasher into an atomic temp file, and a mismatch aborts before the rename (verified: DLL 2.5).
- TLS uses ureq's platform verifier, so system CAs work as they do in Go (verified pick, untested at runtime: [rust-crates.md](rust-crates.md), HTTP and TLS). ureq honours proxy variables, while Go's DLL client ignores `HTTPS_PROXY` (verified: same). spela keeps ureq's behaviour and honours them (decision 41).
- Startup auto-update stays in the TUI only, gated by `auto_update_dlls` and skipping running and deny-listed games (verified: DLL 2.9; decision 39). The CLI's operation is `dll update --all`, run when the user asks.

## Release check and manifest updater

### Release check

- GET `https://api.github.com/repos/jgabor/spela/tags?per_page=100` with `Accept: application/vnd.github+json`, a 5 s timeout and a 1 MiB body cap. Pick the highest tag matching `^v?\d+\.\d+\.\d+$` with the shared version compare (verified: [parity-inventory.md](parity-inventory.md), DLL 5). Send `User-Agent: spela/<version>` (see [Backends and fakes](#backends-and-fakes)).
- Follow the `Link` header's next page, up to a few pages, until a release tag is found (inferred). Today only the first 100 tags are read, and the `dll-*` tags share the namespace, so the newest release could fall off page one (verified: DLL conflicts, "Release check pagination and shared tag namespace").
- The version is embedded at build time. Release CI passes the tag's version through an environment variable that `option_env!` reads: `SPELA_VERSION` (as built in `spela_core::release`, set by `release.yml`). The AUR `spela-git` package sets it to its pkgver, such as `0.7.0.r6.g087d1c4`, as Go's AUR build did; that build reports the pkgver and checks with its release part, `0.7.0` (decision 68). A build without it, or with any other value, reports `<crate version>-dev` and never checks (inferred). Go's dev, `git describe` and AUR `-git` builds all skip the check (verified: DLL 5), so decision 68 turns it on for the AUR package. The Cargo version alone cannot mark a release, because every build has one (inferred).
- Rust releases keep publishing plain `vX.Y.Z` tags newer than v0.6.2, or Go users never see the notice (verified: DLL 1.2).
- The state file keeps `checked_at` and `latest`. It asks once a day, again when the stored time is in the future, records every attempt and updates `latest` only on success (verified: DLL 5).
- `check_updates` gates it, as today. Only the TUI checks, and the CLI stays quiet (verified for Go: DLL 5; decision 39). The notice goes to the right of the status line as `spela vX is available` ([terminal-ux.md](terminal-ux.md), frame). Today's header text is `Spela vX is available (you have vY)` (verified: DLL 5).
- Tests point it at a loopback fake through the backend, never at GitHub ([../AGENTS.md](../AGENTS.md) hard rule). `--demo` returns a canned answer: its fake serves a tag list whose newest tag is the next patch release, and the demo checks as a release build, so the notice shows (as built in Phase 5, `spela_core::fake::dlls`). This replaces the Go `e2e` build tag and `SPELA_E2E_RELEASES_URL` (verified: DLL 5).

### dlss-updater in Rust

`tools/dlss-updater` is ported to a small Rust binary in the workspace, not shipped in `spela` (decision 14). It keeps today's behaviour (verified: [parity-inventory.md](parity-inventory.md), DLL 7):

- the TechPowerUp scrape, with the version from the page title, the file id from a hidden input and the download URL from the 302 `Location`;
- text or JSON output with type, version, download_url, filename, is_new and source;
- exit codes 0 for new, 2 for up to date and 1 for an error.

One proposed change: `is_new` means newer by the shared version compare, not "the strings differ" (inferred fix, DLL 7 and DLL conflicts, "Meaning of 'latest'"). Its tests use saved HTML pages served on loopback, never the network ([../AGENTS.md](../AGENTS.md) hard rule).

As built in Phase 7, three smaller differences from Go's tool, compared against Go's `run()` on the same loopback fake and manifests:

- The manifest's current version is the highest valid one, with one leading `v` dropped first as Go did, so `v310.5.3` equal to the listed version is up to date, not new.
- Words after the flags, or after `--`, are a usage error with exit 2. Go ignored them, so `-type nope extra` exited 1 in Go and exits 2 here: a mistyped command should not run a check.
- When the manifest already has a version newer than the one TechPowerUp lists, the text says so: `X is up to date: the manifest has A, newer than the listed B`. The JSON line is the same, apart from Go's escaping of `&`, which parses to the same value.

`dll-manifest.yml` builds the Rust tool, which removes the Go 1.22 against go.mod 1.25.5 mismatch (verified: DLL conflicts, "CI Go version vs go.mod"). `dll-release.yml` writes the real in-game filename (verified bug: DLL conflicts). The manifest's last bot commit was 2026-02-03, so the scraper may be failing (unverified: DLL 7). The user checks the workflow runs in Phase 0.

## Terminal UI

[terminal-ux.md](terminal-ux.md) is the decided TUI spec, and [mockups.html](mockups.html) shows its screens on exact cell grids. This doc does not repeat it. The decisions it rests on:

- Keys: spela's basic keys (digits, arrows, Tab, Enter, Space and Ctrl-C) plus `?` help, `:` palette, `/` find, and `q` and `Q` quit (decision 1, option B). The palette lists Actions rows, games and settings.
- Esc goes back one level everywhere. It never quits and never confirms (decision 2).
- spela's layout contract stays: an 80x24 floor and one breakpoint at 100 columns or 30 rows. The Standard, Compact and Focused layouts and `compact_mode` are dropped (decision 3).
- Confirmations stay for DLL install, update and restore, the whole-page "for all settings" resets, and Apply to games with their own value, with Cancel focused first (decision 4; [terminal-ux.md](terminal-ux.md), principle 6). `confirm_destructive = false` skips only the update confirmation, as today (verified: [parity-inventory.md](parity-inventory.md), TUI 10). The CLI never prompts, as in Go, whose command files never read stdin (verified: grep of `cmd/spela/commands`). Decision 4's hardware confirmations are the TUI's GPU reset, CPU governor and SMT actions, each opening on Cancel (decision 44). Space undoes a single field (decision 4).
- The live hardware commands and a read-only Preview launch come to the TUI (decision 44). See [TUI process](#tui-process) for the job, and [terminal-ux.md](terminal-ux.md) for the screens.
- No accessible mode in v1 (decision 61).
- Mouse on by default, with a key for every clickable label (decision 5). `ui.mouse = false` turns it off.
- Themes `terminal` (ANSI 16, the default), `neon` and `mono` (decision 27). `NO_COLOR` forces mono.
- ⚑ marks a field or game with its own value (decision 28).
- Autosave with no drafts, as the shipped Go TUI does ([README.md](README.md), finding 4). Saves run on the write queue.

TUI 19 in [parity-inventory.md](parity-inventory.md) is mostly overturned. The palette, `q`, letter and Esc bans are reversed by decisions 1 and 2. "No privileged tuning" is reversed by decision 44, which brings GPU reset, CPU governor and SMT to the TUI. "No launch" still binds: the TUI never starts a game, and Preview launch only shows the dry-run summary. "No pin action" still binds.

[terminal-ux.md](terminal-ux.md) now covers the items this doc used to list as missing from it:

- denylist state and management on the game's DLLs section, for Phase 5 (decisions 20 and 36);
- the VKD3D notice on the profile page, for Phase 4 (decision 20; in Go only the GUI renders it, verified: [parity-inventory.md](parity-inventory.md), TUI 14);
- the hardware actions, their confirmations and the no-agent failure message, and Preview launch, for Phase 6 (decision 44);
- the editor's empty prefill for a "none" row and digits typing on the Find row, both Go bug fixes under decision 8 (verified bugs: [parity-inventory.md](parity-inventory.md), TUI open questions and TUI conflicts);
- the section strip without `Overlay` in its list of changed e2e strings (decision 10).

The shell that runs it is in [System architecture](#system-architecture), and its budgets are in [Performance budgets](#performance-budgets). Review in a real terminal follows [../AGENTS.md](../AGENTS.md) and [tui-testing.md](tui-testing.md).

## Testing strategy

The rewrite is tested at every layer without root, without the GPU, without the network and without the user's files. That is a hard rule ([../AGENTS.md](../AGENTS.md)), so the only checks that touch real hardware are the live checks the user runs (last row). The Go tests are the spec to port: [parity-inventory.md](parity-inventory.md) lists the ones that pin behaviour (DLL 9; TUI 17 and 18). Where a test pins a Go bug, the Rust test pins the fixed behaviour instead (decision 8).

| Layer | What | How |
|---|---|---|
| Pure logic | Version compare, VDF tokenizing, manifest URL validation, the wrapper argv router, env precedence and last-wins dedupe, inheritance and the source vocabulary, the alert evaluator, the dry-run summary builder, and the field descriptor table | Inline `#[cfg(test)] mod tests`, last in each file ([../AGENTS.md](../AGENTS.md)). Go table tests are ported row for row, for example `version_test.go`, `alerts_test.go` and `migrate_sr_preset_test.go` (verified: [parity-inventory.md](parity-inventory.md), DLL 9 and domain 3). proptest for version compare, VDF and the router (verified pick as Later: [rust-crates.md](rust-crates.md), testing). The descriptor table guard counts 25 fields instead of 35 (inferred count from decision 10 and `internal/profile/fields.go:61-101`). It replaces the 35-field guard in [parity-inventory.md](parity-inventory.md), domain 15 |
| Doc tables | `docs/profile-fields.md` against the field descriptor table, and `docs/configuration.md` against the config option table | Tests that pin each doc table's keys, kinds and label order to the code table, as Go's doc-contract tests do (verified: `internal/profile/vocabulary_test.go:278-314` reads `docs/profile-fields.md`, and `internal/config/docs_contract_test.go:49-117` checks `docs/configuration.md`). Inline tests in `spela-core` (inferred) |
| Backends against fakes | NVML, sysfs (governor, SMT), procfs, the privilege runner, `kscreen-doctor`, HTTP, Steam libraries, the child launcher and the clock | The traits and fakes in [Backends and fakes](#backends-and-fakes). The same fake backs `spela --demo` (decision 18). The real ureq backend runs against a loopback `TcpListener` thread (verified: [rust-crates.md](rust-crates.md), testing). Steam libraries are fixture trees in temp dirs. Each fake has failure switches: pkexec exit 126 and "dismissed", NVML NotSupported, a `kscreen-doctor` that never exits, HTTP 404 and a slow body (inferred) |
| Migration fixtures from Go | Every Go-written file the migration reads | The generator and cases in [Fixture tests](#fixture-tests), captured in Phase 0 while the Go code still exists. Each test asserts that every value the user set is kept or listed in the migration record, that the Go file is byte-identical afterwards, that a second run changes nothing, and that a failed migration writes nothing (inferred, from the migration rule in [../AGENTS.md](../AGENTS.md)) |
| TUI update and view | Every destination, popup, editor and picker, and every empty, loading, error and unavailable state | ratatui `TestBackend` plus insta snapshots at 80x24 and 120x40, the two sizes in spela's layout contract (decision 3), plus the resize prompt at 72x20, a size `docs/tui-testing.md` already reviews (verified: [parity-inventory.md](parity-inventory.md), TUI 18). Snapshots cover the `terminal`, `neon` and `mono` themes and `ui.ascii_glyphs` (decision 27; [terminal-ux.md](terminal-ux.md)). Copy mejl's harness: a manual clock, the fake backend, and a loop that runs commands until they settle (verified: [mejl-study.md](mejl-study.md), mejl-ui tests). Also port its style checks per cell, its hostile-width names and its size sweeps (verified: same) |
| Keymap reachability and conflicts | Every binding in every context | Building the keymap fails on a conflict. Every default binding is reachable with legacy key encoding ([terminal-ux.md](terminal-ux.md), principle 3). Every key in the hint bar and in Help dispatches an action that is available in that frame ([terminal-ux.md](terminal-ux.md), principle 2). Letters do nothing in a text input, search or picker. Esc never quits and never confirms (decision 2). Ctrl-C quits even below 80x24. Confirmations open on Cancel (decision 4). Help never shows a reason for a key that works, which fixes a Go bug (verified: [parity-inventory.md](parity-inventory.md), TUI conflicts) |
| Mouse hit tests | Every clickable region | The frame's hit map gives each click the same action as its key. Every clickable label shows its key (decision 5). Also covered: the wheel over the pane under the pointer, double click as Enter, and right click opening Actions ([terminal-ux.md](terminal-ux.md), mouse). These are the cases mejl's `tests/mouse.rs` covers (verified: [mejl-study.md](mejl-study.md), mejl-ui tests) |
| Terminal pty tests | Mode bytes, and the real release binary's screen | Keep mejl's `openpty` byte checks for mode order (verified: [mejl-study.md](mejl-study.md), binary 12.1). Add runs of the real release binary in a pty, with the screen read by vt100 0.18.1 (verified version: [rust-crates.md](rust-crates.md), testing). mejl never reads a screen and never runs the real binary (verified: binary 12.1). Scenarios: entering and restoring mouse modes 1000, 1002 and 1006 but never 1003; restore on exit, on panic and on SIGTERM; Ctrl-Z and resume; bracketed paste; the resize prompt; first-frame timing. The harness answers the DA1 probe at once, so timings do not include a probe timeout (inferred) |
| CLI | Every command, flag, message, exit code and `--json` shape | assert_cmd with `env_clear()`, and HOME, the XDG dirs and PATH pointed into a temp dir (verified pattern: [mejl-study.md](mejl-study.md), binary 12.2). The child env is set per process because `std::env::set_var` is unsafe in edition 2024 (verified: [rust-crates.md](rust-crates.md), traps). insta covers text output and the version 1 `--json` (decision 7). Exit codes 0, 1, 2, 3 and 5, and the game's own status in wrapper mode ([terminal-ux.md](terminal-ux.md), CLI output). Every binary test sets `SPELA_BACKEND=fake` (see [Backends and fakes](#backends-and-fakes)). Also: `NO_COLOR`, output that is not a terminal, and `spela list --json \| head -c1` exiting 0 without a panic (verified trap: [rust-crates.md](rust-crates.md)). Wrapper routing runs with PATH shims named `overlay`, `gui` and `scan`. Known subcommands must win (decision 21), and the reserved names print their removal error and exit 2 (decision 45). Router cases include `spela -- <script>` and `spela FOO=1 -- <script>`, which launch as in Go (verified: [parity-inventory.md](parity-inventory.md), CLI 5.1). `dlss set --sr-preset default` and `--rr-preset default` store `default`, and `dlss reset` makes them follow (inferred) |
| termctrl visual review | The compiled binary, by eye | Per [../AGENTS.md](../AGENTS.md): Terminal Control 1.2.1 or newer, 120x40 and 80x24, `termctrl show`, keys sent only when visible, isolated XDG dirs, and always stop the session. Running `spela --demo` makes a review safe by default, because the fake backend has no real Steam library, GPU or pkexec behind it (inferred) |
| Black-box TUI parity | The TUI journeys that Go's `tests/e2e` covers | The Go e2e oracle is not kept (decision 54). It could not run unchanged anyway: it imports `internal/dll` and `internal/profile`, waits for `spela.pid` to disappear, and asserts strings that [terminal-ux.md](terminal-ux.md) changes (verified: grep of Go imports; `tests/e2e/tui_journeys_test.go:214-224`; [parity-inventory.md](parity-inventory.md), TUI 18 and 22). Its journeys become Rust pty scenarios on the real binary, read with vt100, plus the termctrl review above. The Phase 0 text transcripts of the Go commands are the CLI's black-box reference |
| Read-only checks on this machine | Proton tool dirs, `config.vdf` key casing, StateFlags, `kscreen-doctor -o` and the PE versions of installed DLLs | Run by Claude in Phase 0, reading only, and recorded in [phase0-checks.md](phase0-checks.md) (decision 63). Their outputs become fixtures |
| Live checks, by the user only | NVML setters and offsets, pkexec prompt counts and the `--serve` restore, `kscreen-doctor` applies, real launches, real DLL swaps and restores, the GitHub tags API and the manifest fetch | Never run by an agent or by CI ([../AGENTS.md](../AGENTS.md), hard rules). [Open questions](#open-questions) lists them. Phase 6 ends with real launches of Cyberpunk 2077, spela's primary test target (verified: `~/git/spela/AGENTS.md:55`) |

### Hard safety rules as tests

Each hard rule in [../AGENTS.md](../AGENTS.md) has a mechanism that keeps it true by default, plus a test that fails when someone breaks it. The mechanisms are (inferred) unless a source is given.

| Rule | Mechanism | Test that fails when it is broken |
|---|---|---|
| No `pkexec` or `sudo` | Privileged work goes through the `Privilege` trait. The only real implementation lives in one module, and only `main` builds it. It finds `pkexec` by a PATH lookup, as Go does (verified: [parity-inventory.md](parity-inventory.md), CLI 7.1), and never by an absolute path, so a PATH stub always catches it | CLI tests put stub `pkexec` and `sudo` first on PATH. Each stub writes a marker file in the temp root and exits non-zero, and every test asserts that no marker exists. A grep gate in `just check` and CI fails when `"pkexec"` or `"sudo"` appears outside that module, inline test modules included. The binary tests and any `build.rs` are scanned for a forbidden program named by an absolute path, such as `/usr/bin/pkexec`, or run by its bare name, which the PATH stubs cannot catch outside the sandbox. The production part of a file ends only at the column-0 `#[cfg(test)]` on its last `mod`, so an indented `#[cfg(test)]` item hides nothing (as built in Phase 1) |
| No hardware changes | NVML, sysfs and every system command sit behind traits. The sysfs root is injected. A cleared env does not stop the dynamic loader from finding the real `libnvidia-ml.so.1`, so every binary test selects the fake backend with `SPELA_BACKEND=fake` in its sandbox env. It swaps only the backends and keeps the test's `Paths`, so CLI, migration and wrapper tests run on their own fixture trees. `--demo` builds on the same selector (decision 18; the selector is inferred) | PATH stubs for `nvidia-smi`, `kscreen-doctor`, `systemctl` and `scx_loader` fail loudly in the same way. The fake NVML records every setter call. A unit test that calls `backends::real()` panics, and a binary test runs every command in the sandbox and fails unless each logs `backend=Fake` (as built in Phase 1: `crates/spela/src/backends.rs`, `tests/safety.rs`). A unit test points nvml-wrapper's builder at a missing library and expects a plain error, not a panic (verified behaviour: [rust-crates.md](rust-crates.md), NVML). sysfs write tests assert that only the injected temp root changed |
| No remote hosts | HTTP sits behind one trait, and only one module per binary builds a ureq agent: `crates/spela`'s http backend and `dlss-updater`'s fetch module. The gate checks both paths. A manifest URL may be http only to loopback (verified: [parity-inventory.md](parity-inventory.md), CLI 11). The release check gets a loopback-only override, as Go's `SPELA_E2E_RELEASES_URL` does (verified: [parity-inventory.md](parity-inventory.md), DLL 5) | Fixtures set `check_updates = false`, `auto_refresh_manifest = false` and a loopback manifest URL, like the Go e2e mock (verified: [parity-inventory.md](parity-inventory.md), DLL 9). The sandbox env sets `HTTPS_PROXY`, `HTTP_PROXY` and `ALL_PROXY` to a closed loopback port and `NO_PROXY=127.0.0.1,localhost`. ureq honours proxy variables (verified: [rust-crates.md](rust-crates.md), HTTP and TLS), so a stray remote request fails fast instead of leaving the machine. This tripwire covers only clients that keep proxy support on, so the grep gate stays. It fails on `ureq`, `std::net`, `TcpStream` or `UdpSocket` in any source outside the two HTTP modules (as built in Phase 1) |
| No real XDG dirs | All paths come from a `Paths` value. Tests build it with `Paths::under(base)` (verified pattern: [rust-crates.md](rust-crates.md), XDG paths). Env is read once into a snapshot, and nothing below the CLI calls `std::env::var` (verified rule: [mejl-study.md](mejl-study.md), binary 11) | Binary tests clear the env and set HOME, `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`, `XDG_STATE_HOME` and `XDG_RUNTIME_DIR` under the temp root. HOME is in the temp root too, so every fallback also lands there. A test asserts that the log file and lock paths the binary reports are under the temp root, as mejl's print-config test does (verified: [mejl-study.md](mejl-study.md), binary 12.2). A grep gate fails on `std::env::var`, `var_os`, `vars` or `set_var` outside the snapshot module, and on any other `std::env` path or `use std::env` import, so an imported `var` cannot slip past. `main`'s `std::env::args_os()` is the one allowed use (as built in Phase 1) |
| No real game files | Steam root candidates come from HOME and config, so in tests they point at fixture libraries. Every DLL write checks that its target is inside a known library root | The Go path-escape test is ported (verified: `operations_test.go:520-566` in [parity-inventory.md](parity-inventory.md), DLL 9). Every DLL test hashes the whole temp tree before and after, and compares the change with the expected file list |
| UI thread never blocks | Workers own NVML, `nvidia-smi`, pkexec, the network, Steam scans and file writes ([../AGENTS.md](../AGENTS.md); [terminal-ux.md](terminal-ux.md), principle 4) | A slow fake backend (NVML 500 ms per call, scan 2 s, a save that takes 1 s) still lets every key reach a flushed frame within budget. The fake records the calling thread and fails the test when it is the UI thread. No key is handled after quit, so the write-queue join at exit stays off the UI path |
| One batched pkexec round trip, restore registered first | The wrapper registers each restore before it spawns the game | Fake-runner tests assert one privileged apply per launch, and that each applied change has a registered restore before spawn. Restore must run on normal exit, SIGINT, SIGTERM, SIGHUP and after a failed or partial apply. That fixes the Go holes (verified: [parity-inventory.md](parity-inventory.md), CLI 7.2). Helper tests run the `--serve` loop on the fakes and check that it restores what it applied on a `restore` line, on stdin EOF at every step, on a closed stdout, on a malformed line and on SIGTERM, and that `--once` never restores (decision 30; inferred tests) |

## Performance budgets

The budgets come from [terminal-ux.md](terminal-ux.md) (budgets). They are measured on release builds, not debug builds. Deterministic counters are the gate. The wall-clock p99 and p95 tests run in release builds with one rerun and report their numbers (decision 55). mejl claims its budgets are enforced in CI, but its tests assert only loose debug bounds (100, 60, 30 and 250 ms), no UI benchmark exists, and its CI runs only on tags (verified: [mejl-study.md](mejl-study.md), mejl-ui tests and binary conflicts).

| Metric | Budget | Measured how | Gate |
|---|---|---|---|
| Key to bytes flushed, local action | p99 under 8 ms ([terminal-ux.md](terminal-ux.md)) | The input thread stamps each event at read time. The clock stops when the backend's flush returns. Samples are per input event (see [The event loop](#the-event-loop)). An in-process run of the real loop, with the crossterm backend writing to a byte sink, plays 1,000 scripted keys over Library, a profile edit, Settings and the palette on a 1,000-game fixture (inferred) | Every push, in a release build. Reports p99 and buckets. Over 8 ms gets one rerun, then a warning in the job summary (decision 55) |
| Warm start to first useful frame | p95 under 50 ms, from the cached game list ([terminal-ux.md](terminal-ux.md)) | The pty harness spawns the release binary with a cached game list and answers DA1 at once. The clock runs from spawn until vt100 shows "Library" and the first rows. 20 runs (inferred). NVML init stays off this path: Go does it eagerly in every process, and it takes 4 to 17 ms (verified: [rust-crates.md](rust-crates.md), compile time and binary size; [parity-inventory.md](parity-inventory.md), CLI 2.1) | Every push, release build. Reports, as above (decision 55) |
| Metrics tick | One worker poll every 2 s. The UI only draws the result ([terminal-ux.md](terminal-ux.md)) | A manual clock, as in mejl's harness. A fake NVML counts the polls per tick and records the calling thread. A real 12-query sweep took about 7 ms (verified: [rust-crates.md](rust-crates.md), NVML), which is why it never runs on the UI thread | Every push. Deterministic |
| Idle | No wake-ups beyond the 2 s tick, and at most one draw per tick. No draw when the shown metric text did not change. No wake-ups at all on the resize prompt or when no metrics are available (inferred restatement, see [Where the sources disagree](#where-the-sources-disagree)) | Count loop turns and draws over a 60 s idle window on the manual clock. mejl's own idle claim fails in its code: its default screen redraws every second (verified: [mejl-study.md](mejl-study.md), binary 4.4) | Every push. Deterministic |
| Draws per batch of events | At most one ([terminal-ux.md](terminal-ux.md), principle 4) | A draw counter in the harness. mejl draws after every batch with no dirty check (verified: [mejl-study.md](mejl-study.md), binary 4.2) | Every push. Deterministic |
| Palette match per keystroke | Under 1 ms over every Actions row, game and setting in a 1,000-game fixture (inferred, after mejl's target in its architecture doc) | A release-mode perf test, or divan (verified as Later: [rust-crates.md](rust-crates.md), testing) | Every push, release build. Reports, as above (decision 55) |
| Commands that do not touch the GPU | No NVML init. A release `--version` took 1 ms (verified: [rust-crates.md](rust-crates.md), compile time and binary size) | assert_cmd with a fake NVML that records init | Every push. Deterministic |
| Binary size and clean build time | Track only. The starter set gave 5.8 MB and an 18.5 s clean release build (verified: [rust-crates.md](rust-crates.md)) | Release job artifact size, and the build time in the job log | Report on tags |

How the gate stays honest on shared CI runners (decision 55):

- The deterministic counters (draws, wake-ups, polls, NVML init) are the gate, because they do not depend on machine speed. Any counter over its budget fails the job.
- The wall-clock tests keep their full budget as the target. A miss gets one rerun of that test only. A second miss is reported in the job summary with its numbers and does not fail the job, because a hosted runner's speed is not spela's (inferred reading of decision 55).
- Each run reports buckets (under 8, 16, 50 and 100 ms), as mejl's method does (verified: [mejl-study.md](mejl-study.md), method 1.5). The true p99 on the user's machine is measured locally and noted in the as-built notes.

## CI

Phase 1 added the Rust gate as its own workflow, `.github/workflows/rust.yml`, and cut `.github/workflows/ci.yml` down to the Go reference build and tests (step 8). The Go lint, coverage and e2e jobs, and the Wails, bun and Playwright steps, are gone. Phase 7 made `release.yml` the Rust release: on a `v*` tag it validates the tag, calls `rust.yml` for the full gate, builds with `SPELA_VERSION` set, and publishes `spela-linux-amd64` and `checksums.txt` with the changelog section as the notes. Only its build job has `contents: write`. `ci.yml` goes in the final step (decision 62) (verified: `.github/workflows/rust.yml`, `ci.yml`, `release.yml`). Every push and pull request is gated (decision 17), unlike mejl, which runs tests only on release tags (verified: [mejl-study.md](mejl-study.md), binary 13.4).

On every push and pull request:

1. `cargo fmt --all --check`.
2. `cargo clippy --workspace --all-targets --locked -- -D warnings`.
3. `cargo test --workspace --locked`, in debug.
4. The perf tests in a release build: `cargo test --release --locked -p spela --test perf -- --ignored`, and the pty first-frame scenario. The counters gate, and the wall-clock tests report (decision 55; see [Performance budgets](#performance-budgets)).
5. The MSRV check: `cargo +1.89 check --workspace --all-targets --locked`. The MSRV is 1.89 (decision 16). No 1.89 toolchain was installed during research (verified: [rust-crates.md](rust-crates.md), open questions). It is installed with rustup for this check, locally and in CI (decision 64). mejl's justfile passes `--all-targets` and its AGENTS.md does not (verified: [mejl-study.md](mejl-study.md), binary 13.3). spela uses `--all-targets`, so test code also builds on 1.89 (inferred).
6. The safety grep gates from [Hard safety rules as tests](#hard-safety-rules-as-tests), and the `spela-ui` no-I/O grep from [Cargo workspace](#cargo-workspace).
7. `cargo deny check`, all four checks (licenses, bans, sources and advisories), which enforces decision 15 (MIT, unmodified MPL-2.0 allowed, no GPL or LGPL) and the RustSec traps in [rust-crates.md](rust-crates.md) (decision 56). Its `deny.toml` lists the allowed licences.
8. While Go code remains: `go build ./...` and `go test ./...` in `ci.yml`, so the parity reference stays green. Nothing else runs on the Go code: its lint, its 90% coverage gate and its e2e job left CI in Phase 1, because the Go code is frozen and decision 54 drops the e2e oracle. The Go job goes in the final step with the Go tree (decision 62).

There is no CodeQL job in v1 (decision 56). The Go e2e oracle is not kept (decision 54), so no CI job runs Go's termctrl 0.3.1 any more: Phase 1 removed the e2e job that installed it from `ci.yml`. Any CI job that runs termctrl uses 1.2.1 or newer.

Steps 1 to 3 and 5 are the `just check` gate, which the pre-commit hook runs: `cargo fmt --all --check`, then clippy and test with `--locked`, then `just msrv`, which is step 5 (verified: `justfile`; `.githooks/pre-commit`). Steps 1 to 3, 5 and 7 run in `rust.yml` on every push and pull request; `just deny` runs step 7 offline, without advisories, because those need the RustSec download (verified: `.github/workflows/rust.yml`, `justfile`).

On tags matching `vX.Y.Z`:

1. Validate the tag with the regex the current `release.yml` uses, passing it through env, not shell interpolation (verified: `.github/workflows/release.yml`; [parity-inventory.md](parity-inventory.md), CLI 13.2).
2. Check that the Cargo version equals the tag, as mejl does (verified: [mejl-study.md](mejl-study.md), binary 13.4).
3. Run the full gate.
4. `cargo build --release --locked` for x86_64-unknown-linux-gnu only, with the tag's version embedded (see [Release check](#release-check)). spela is Linux and NVIDIA only, so mejl's aarch64 and macOS targets are not needed (inferred).
5. Publish `spela-linux-amd64` and `checksums.txt`, with notes from the CHANGELOG section (verified intent: [parity-inventory.md](parity-inventory.md), CLI 13.4).

Not on every push: the daily `dll-manifest.yml`, which keeps feeding the manifest URL. Since Phase 7 it builds and runs the Rust `dlss-updater` (`cargo build --locked -p dlss-updater`). The inline-tests-last rule stays, even without CodeQL, because it keeps the door open for mejl's filter later (inferred; the filter's reliance on it is verified: [mejl-study.md](mejl-study.md), method 5.3 and binary 13.4).

## Phased roadmap

Each phase says what it proves. A phase is done when its proof holds, not when its feature list is typed out (the rule from mejl's architecture doc). Parity areas use the citation scheme at the top of this doc.

Go code is deleted by decision 62, which replaces the per-phase deletions this roadmap used to list. Phase 1 deletes the Go GUI, which is not ported ([../AGENTS.md](../AGENTS.md), layout). Phases 2 to 7 delete no Go code. The rest of the Go tree stays untouched as the parity reference until the user's live launch checks pass, and then goes in one commit, the [final step](#final-step-remove-the-go-tree). So `go build ./...` and `go test ./...` stay green through every phase without any import-order bookkeeping, and the Go TUI, launcher and privilege code stay runnable for comparison until the live checks are done.

### Phase 0: verify (no app code)

- Capture the Go fixtures and commit them:
  - The generator in [Fixture tests](#fixture-tests). The `goref` fixtures must be regenerated, because the probe folder is temporary. Fixture games and app ids are fictional (decision 43).
  - The fixtures the Go tests build (4-space and 2-space profiles, legacy profiles, both backup name schemes) and the three-game e2e library (verified: [parity-inventory.md](parity-inventory.md), TUI 18). Where a Go test or the e2e harness pins a real game, the fixture keeps its shape with a fictional name and app id (decision 43).
  - Text transcripts of the read-only Go commands, run against fixture XDG roots. With the Go e2e oracle gone (decision 54), these transcripts are the black-box reference for the CLI.
  - The sandbox for the Go binary. It runs only in the same sandbox as the CLI tests. Its config sets `rescan_on_startup`, `check_updates` and `auto_refresh_manifest` to false and points `dll_manifest_url` at loopback, and the manifest cache is seeded fresh from a fixture. That matters because `dll check-updates` and `show` go through the manifest refresh policy (verified: [parity-inventory.md](parity-inventory.md), DLL 2.4; the TUI start does too, inferred in additions B.8). The Go build runs with `GOPROXY=off`, so a missing module fails instead of downloading (inferred).
- Read-only checks, run by Claude on this machine and recorded in [phase0-checks.md](phase0-checks.md) (decision 63). None of them changes hardware, writes a game file or contacts a remote host:
  - the Proton resolver: whether tools under `/usr/share/steam/compatibilitytools.d` are found, whether built-in tool names match their `steamapps/common` folders, and the key casing in the real `config.vdf` (open question in [parity-inventory.md](parity-inventory.md), domain open questions);
  - Steam `StateFlags` values in the real app manifests, including any game with a pending update;
  - `kscreen-doctor -o`, captured as a parser fixture;
  - whether the PE version of the installed DLLs formats to the manifest string.
- Install the 1.89 toolchain with rustup for the MSRV check (decision 64).
- Checks that change hardware, run pkexec, launch a game or use the network stay with the user ([../AGENTS.md](../AGENTS.md), hard rule 1). [Open questions](#open-questions) lists each one with the phase it gates. None of them blocks Phase 1, because every privileged path is built and tested on fakes first (decisions 30 and 32).
- The design questions this phase used to decide are settled by decisions 30 to 64.
- Closes no parity area. Deletes no Go code.
- Proves: the Go-written fixtures and transcripts are in the repo, and every read-only fact the parsers depend on is recorded.

### Phase 1: foundations

- The Cargo workspace with the crate split and the starter-set changes in [Cargo workspace](#cargo-workspace).
- `Paths` on etcetera, spec strict, with `Paths::under(base)`, and the env snapshot (inferred, after mejl's `paths.rs` and `env.rs`).
- Logging as in [Logs](#logs) (decision 12).
- The flock TUI lock and launch lock under `<state>/locks/`, keeping "spela is already running (PID: N)" (decision 29), with lock files that stay in place (see [Locks](#locks)).
- The exit-code module (0, 1, 2, 3, 5), `error: <message>` plus a one-line hint ([terminal-ux.md](terminal-ux.md), CLI output), and `BrokenPipe` handling on every write (SIGPIPE stays ignored, see [Output, errors and exit codes](#output-errors-and-exit-codes)).
- The clap root with `--version`, `-V` and `-v` printing `spela 0.7.0` (decision 48), help and static completion through clap_complete (decision 57), and the router with its tests, including a leading `--` and the reserved `overlay` and `gui` names (decision 45). Wrapper mode itself lands in Phase 6.
- The backend traits and the shared fake, with minimal demo data on fictional games (decisions 18 and 43), and the `SPELA_BACKEND=fake` selector.
- The test harnesses (assert_cmd sandbox, insta, the pty skeleton), the safety-rule tests and the CI gate with cargo-deny and the MSRV job (decisions 56 and 64). `--locked` is added to the justfile's clippy and test recipes.
- Settled here by tests: tracing-appender lines from TUI and wrapper runs do not interleave in the same daily file (the two-process test in `tests/safety.rs`), and toml 1.1.8 and toml_edit 0.25.17 build with spela's set at 1.89 (the MSRV job builds them through a dev-dependency probe, `crates/spela-core/tests/toml_probe.rs`, until the config code depends on them).
- Closes: CLI 2.1 to 2.3 (with the new exit codes and error format), CLI 3.15 (`gui` removed, decision 26), CLI 8 (lock, now flock), CLI 9 (logging, now file plus stderr), CLI 10 (XDG, now spec strict), CLI 12 (colours and `NO_COLOR`) and CLI 13.1 (build targets, now the justfile). CLI 5.1 closes in Phase 6, when wrapper mode runs behind the router built here (inferred).
- Go deleted: the GUI, which is not ported ([../AGENTS.md](../AGENTS.md)). It is the only Go deletion before the final step (decision 62). The list:
  - `internal/gui`, `cmd/spela/main_wails.go` and its test, `cmd/spela/wails.json`, `cmd/spela/commands/gui.go` with the `GUICmd` line in `cmd/spela/main.go:45`, `internal/nav/gui.go` and `tools/gen-nav-js`;
  - `internal/config/wails.go`, whose `WailsCatalog` has one caller, `internal/gui/app.go:46`, the GUI-only parts of `internal/config/options.go` (`VisibilityGUI`, `VisibilityWails` and `OptionByJSONKey`), and the json tags on `Config` in `internal/config/config.go`, which only the Wails bindings read (verified: grep);
  - the magefile's GUI targets `GenNavJS`, `FrontendBindings`, `FrontendBuild`, `TestFrontend`, `Dev` and `DevStop`, and the GUI parts of the others. `Build` and `Install` drop `mg.Deps(FrontendBuild)` and the `embed_assets,production,webkit2_41` tags, `Test` drops its `./internal/gui` run, `Coverage` drops its wails build, and `TestE2E` drops its GUI deps (verified: `magefile.go:44-51, 157-161, 188, 264, 277-284`). So `go tool mage build` keeps producing a CLI and TUI Go binary until the final step, which the Go TUI reference, the Phase 0 transcripts and the AUR package need (inferred);
  - `webkit2gtk-4.1` and `gtk3` from the PKGBUILD's `depends`, and `bun` from its `makedepends` (verified: `pkg/aur/PKGBUILD:10-11`);
  - the Wails, bun and Playwright steps in Go CI.

  With the `main.go`, config and magefile edits above, these are leaf consumers (verified: grep of Go imports). The Go lock, logging and XDG packages stay, because `cmd/spela/commands` imports them.
- Proves: the gate runs on every push, and breaking any safety rule fails a test.

### Phase 2: read-only CLI parity

- Phase 2 reads the Go YAML files in memory, with the readers Phase 3's migration will use, and writes no config or profile file. `spela scan` writes only the Rust game cache, under its new name (decision 13). This lets the read-only commands close without migrating anything yet (inferred).
- Steam libraries, app manifests and VDF on keyvalues-parser, last duplicate wins, with its listed parity differences accepted (decision 52; verified: [rust-crates.md](rust-crates.md), Steam VDF). Also the Proton resolver, AppID and name resolution, and the game cache at `<cache>/games.json` (decision 43). The CLI rescans only on an empty cache or `spela scan` (decision 24). The fixtures for StateFlags, Proton tool dirs and `config.vdf` casing come from [phase0-checks.md](phase0-checks.md).
- Profile reading, inheritance, the source vocabulary and the effective-value explanation, over the 25 remaining fields.
- Commands: `scan`, `list`, `show`, `profile list`, `profile show`, `config show`, `dlss|gpu|cpu|proton show`, the read-only `cpu governor` and `cpu smt` with no argument, `dll list`, `dll check-updates` on the cached manifest, and `denylist show` and `check`. GPU and CPU status read through the backend traits.
- `--json` in the version 1 schema (decision 7).
- Known bugs fixed here (decision 8): `dll list` and `check-updates` also count XeSS and FSR games, as their own labels say (verified: [parity-inventory.md](parity-inventory.md), corrections), and `list --json` on an empty database prints JSON.
- Closes:
  - CLI 3.1, 3.2 and 3.3, and the read parts of 3.5 to 3.11 and 3.13 (3.12 `overlay` is removed in Phase 3);
  - CLI 4 and the load side of CLI 11;
  - the read side of domain 1, 2 and 3, domain 6, 7 and 8, and the read side of domain 10 and 11;
  - DLL 2.1 to 2.3, the cache-read side of DLL 2.4 and the read side of DLL 3;
  - additions B.3, B.5 and B.7.
- Go deleted: nothing (decision 62).
- Proves: on the fixture libraries and Go-written files, Rust shows the same games, values and sources as the captured Go transcripts. The only differences are the fixed bugs and the new JSON schema.

### Phase 3: profiles and migration

- TOML config and profiles, migrated from Go YAML on first read (decision 6), as set in [Migration from the Go files](#migration-from-the-go-files). The migration record lists what was dropped (decision 10).
- Go's `gpu.clock_offset` and `gpu.memory_offset` are recorded, not copied, and the notice says so (decision 31). Go's `theme`, `show_hints`, `preferred_dll_source` and unknown config keys are dropped and recorded (decision 46). Every TOML file carries `version = 1`, and the config migration writes only values that differ from Go's defaults (decision 43).
- Deny lists: the user list adds to the built-in list, and allow and deny clear each other (decision 36). `profile follow` and `denylist clear` get their names (decision 51).
- Settled here by the fixture tests: whether serde-saphyr reads 2-space YAML and supports `#[serde(flatten)]` into a map for unknown keys.
- Writes: `set`, `reset`, `profile-reset`, follow All games, create and delete. Each transaction takes the flock on a new descriptor (verified need: [parity-inventory.md](parity-inventory.md), domain 15). Writes are atomic with fsync, which Go never does (verified: [rust-crates.md](rust-crates.md), atomic file writes).
- Equal features (decision 20): CLI editing of All games, which no Go command offers today (verified: [parity-inventory.md](parity-inventory.md), additions B.1). Also `denylist` allow and deny with their removals, which nothing wires today (verified: DLL 3), and `config set` for every key the TUI shows.
- Known bugs fixed (decision 8), from [parity-inventory.md](parity-inventory.md):
  - `--fan-speed 0` is documented as "0 to clear" but stores an own value of 0, and `default` does not follow All games. Rust keeps storing 0, now documented as "no fan change at launch", and `default` follows (see [Partial-failure rules](#partial-failure-rules));
  - the `--sr-preset` help says A-M, and `--rr-preset auto` is accepted then rejected;
  - `.yml` profiles are listed but skipped;
  - `profile delete` reports success when nothing existed;
  - the denylist fails open on a malformed file;
  - `config.yaml` and `overrides.yaml` are written non-atomically.
- The `profile show` text output loses its Overlay section, so its sections are Proton, DLSS, GPU and CPU (decision 10; today's order verified: [parity-inventory.md](parity-inventory.md), CLI 3.5).
- Port `docs/profile-fields.md` (25 fields) and `docs/configuration.md` (the TOML keys, including `[ui]`), each with a test that ties its table to the descriptor table or the config option table, as the Go doc-contract tests do (see [Testing strategy](#testing-strategy)). Keep `docs/kde-vrr.md` and `docs/NVAPI.md`. Drop `docs/gamemode.sh` (inferred; done). The two ports are blocked while the Go tree stays (decision 62): Go's doc-contract tests, `internal/profile/vocabulary_test.go` and `internal/config/docs_contract_test.go`, read these exact files and pin Go's 35 fields and option keys, so a Rust table there fails `go test ./...`. They wait for a decision: new paths for the Rust tables, or the port in the commit that removes the Go tree.
- Closes:
  - the stored-profile parts of CLI 3.5, 3.6 and 3.8 to 3.11, plus 3.12 (`overlay`, removed by decision 10), 3.13 and 3.16;
  - the save side of CLI 11;
  - domain 1 to 3 in full, and domain 14 (the in-scope docs);
  - DLL 3 except the swap guard on install and update and the TUI surface, which Phase 5 closes;
  - DLL 6 with CLI 3.8 (the `dlss` command and `docs/NVAPI.md`);
  - DLL 1.4, DLL 4.1 to 4.4 and DLL 4.6 to 4.8, closed by being dropped (decision 10);
  - additions B.1 and B.2.
- Go deleted: nothing (decision 62).
- Proves: every Go fixture migrates with each value kept or listed in the record, a second run changes nothing, and the Go originals stay byte-identical. CLI writes match the Go semantics except for the listed fixes.

### Phase 4: the TUI shell

- mejl's shell: the one-channel loop, the input thread, the on-demand timer, the terminal manager and panic hook, the signal watcher, the probe and its cache, the keymap engine, layout, hit map, theme and perf ring (verified as carry-over: [README.md](README.md), finding 1), with the fixes in [What carries over from mejl](#what-carries-over-from-mejl).
- The [terminal-ux.md](terminal-ux.md) frame and the decisions in [Terminal UI](#terminal-ui).
- Library with the profile page and All games, Settings with every config key, and Monitor. Monitor gets the 2 s metrics thread on GPU 0 only (decision 25), and the alert evaluator is ported from Go `internal/overlay/alerts.go` with `alerts_test.go` as its spec (decision 10 keeps it). Search, sort, filters, multi-select and the Find row (verified: [parity-inventory.md](parity-inventory.md), TUI 11 and conflicts).
- One background scan, with the first frame drawn from the cache (decision 24). That fixes the double startup scan (verified Go bug: [parity-inventory.md](parity-inventory.md), TUI conflicts).
- `spela --demo` on the shared fake (decision 18).
- The VKD3D notice on the profile page (CLI 3.11, decision 20), the editor's empty prefill for a "none" row and digits typing on the Find row, as [terminal-ux.md](terminal-ux.md) sets them.
- No metrics pause on focus lost (decision 42). No accessible mode (decision 61).
- Settled here by a test: whether a blocking `crossterm::event::read` survives Ctrl-Z and resume without mejl's input pause (the pty scenario in [The event loop](#the-event-loop)). Settled: it does.
- As built, the shell first: `crates/spela/src/tui/` holds the terminal, the threads and the loop, and `crates/spela-ui/src/screens/` holds one file per destination and popup, registered in `screens/mod.rs`. The TUI lock is taken before the terminal check and the signal watcher, because the CLI takes it before `tui` runs; nothing writes before either. Then each destination in its own file: the Library list with Find, filters, sorts and multi-select, and the profile page (`screens/library.rs`, `screens/profile.rs`); Settings, the deny list surface, Help built from the keymap and the shared editor (`screens/settings.rs`, `screens/denylist.rs`, `screens/help.rs`, `editor.rs`); Monitor with the alert evaluator (`spela-core/src/alerts.rs`) and its hardware actions, and the read-only DLL Catalog (`screens/monitor.rs`, `screens/dlls.rs`). The metrics thread and the profile page's governor picker share one CPU reader. The wall-clock budgets run in `crates/spela/tests/perf.rs` (key to flush over 1,000 keys and the palette match, on 1,000 games) and the pty first-frame scenario, as CI's release step that reports and never fails on time. Still waiting: the batch DLL update and the DLL actions in the DLLs section (Phase 5); the Preview launch planner and the real polkit path (Phase 6); and, of the benchmarks, the first frame as a p95 over 20 runs from the cached game list (one run reports it now) and the one-rerun-then-job-summary step of decision 55.
- Closes:
  - TUI 1 to 4, 6 to 10 and 14 to 16, except the Catalog parts of TUI 3;
  - TUI 5, 11, 13 and 17, except the release notice, the batch DLL update, the startup manifest, release and auto-update steps, and the DLL and Catalog strings, which Phase 5 closes;
  - the string changes [terminal-ux.md](terminal-ux.md) lists, plus the section strip without its Overlay entry and the Overlay rows removed (decision 10);
  - CLI 3.14 (`tui`), with the TUI lock;
  - domain 12 (destinations, scope and aspect). The breadcrumb, `ProfileSubsystem` and `nav/gui.go` are recorded as not ported;
  - DLL 4.5 (alerts only), and the TUI startup half of additions B.8. Its manifest half waits for Phase 5.
- TUI 19 is recorded as in [Terminal UI](#terminal-ui): the palette, `q`, letter and Esc bans are reversed by decisions 1 and 2, "no privileged tuning" is reversed by decision 44 in Phase 6, and "no launch" and "no pin action" still bind. TUI 20 is not ported.
- Go deleted: nothing (decision 62).
- Proves: the deterministic counters hold on the CI gate, and the key-to-flush and palette budgets report their numbers from a release build (decision 55). The first frame reports one release run, not yet a p95. Also the snapshots at 80x24 and 120x40, the keymap and mouse tests, and a termctrl review of `spela --demo` per [../AGENTS.md](../AGENTS.md).

### Phase 5: DLL operations

- Manifest fetch with its refresh policy and a body cap, into `manifest-v1.json` (decision 53). Download with SHA-256, the shared DLL cache, install, update, batch update, update all, cached-only updates, restore, startup auto-update in the TUI only, and the release check into `release-check.json`, also TUI only (decisions 39 and 53). Every request sends `User-Agent: spela/<version>` and honours the proxy env (decision 41). All network calls use blocking ureq on workers (decision 11).
- Backups: read `backup.json` and both backup name schemes from Go installs in place (decision 50; verified safety-critical: [parity-inventory.md](parity-inventory.md), DLL 1.3). Install records `created_files` and the `installed` hashes, restore deletes created files, and a Steam-updated DLL refreshes the backup first (decision 37).
- Equal features (decision 20): the CLI gains install, version pinning, cached-only, update all and `dll cache [type]`. The TUI gains denylist state and management, as [terminal-ux.md](terminal-ux.md) sets it. The swap guard on install and update lands here too.
- The hazards in [Hazards fixed](#hazards-fixed), with one stale function that Library, Catalog and the CLI call (decision 38).
- TUI 12 flows and the Catalog destination, with confirmations that open on Cancel.
- Closes:
  - CLI 3.7;
  - DLL 1.1 to 1.3, 2.4 to 2.12, the rest of DLL 3, and DLL 5;
  - TUI 12, the Catalog parts of TUI 3 and 5, and the remainders of TUI 5, 11, 13 and 17 from Phase 4;
  - the manifest half of additions B.8, and additions B.6.
- Go deleted: nothing (decision 62). `tests/e2e` is not adapted into a Rust oracle (decision 54); it goes with the Go tree in the final step.
- Proves: Go-era backups restore byte-exact from fixtures, and an index with the new keys still loads with Go's `LoadBackup`, called from the Phase 0 generator (inferred). Every write lands inside the fixture libraries. No remote host is contacted. The CLI and TUI offer the same DLL operations.
- As built, the TUI half. One DLL job runs at a time on the job pool: `Cmd::Dll { token, op }` answers with one `Msg::DllProgress` per stage (the shell drops the per-chunk events) and `Msg::DllDone`, which carries the game cache as it is after the job; a stale token is dropped. `spela_ui::dll_jobs` holds the jobs, their confirmations and their result words, shared by three places:
  - the game's DLLs section, `screens/game_dlls.rs`, which the profile page draws: Install (a family picker, then a version picker with the latest first), Update DLLs (the stale DLLs by the one stale function) and Restore originals (the targets of the backup index, read on a worker), each with an Actions and a palette row. While the game's job runs the section reads `Work is running. Controls return when it finishes.`, and its last result stays under the versions;
  - Library's batch, `Update selected games (N visible)`, over the stale DLLs of the listed selected games;
  - the Catalog's `Update all stale deployments (N DLLs)`, over every game no deny list names, downloading as needed, as `dll update --all` does. Go's was cached-only because its stale test compared with the newest cached version; under decision 38 a cached-only update-all would fail every uncached target.
  
  Every update, install and restore asks first with Cancel focused, in Go's words (`Confirm DLL update`, `1 DLL target(s)`, `Backup: ...`, `DLL update cancelled, no operation ran`), and `confirm_destructive = false` skips only the update's. The restore job reads the backup again and answers `BackupChanged` when its targets moved since the confirmation, which then opens again as `Backup changed: review restore`. A scan and a DLL job each replace the game cache, so neither starts while the other runs, and the rows say `wait for pending work`. Quit waits for a DLL job. The start runs the local reads first and then, on the pool's queue behind them, the manifest fetch under its refresh policy (the startup manifest of additions B.8), the release check when `check_updates` is on, and the auto-update when `auto_update_dlls` is on, after the startup scan when one runs (`spela_core::dll::auto`, Go's `AutoUpdate`, with the running-games check of `steam::detect`). Progress shows as words on the right of the status line, such as `Downloading DLSS 310.5.3...`, and the release notice sits there when nothing runs.
  
  `spela --demo` serves a manifest, payloads whose SHA-256 matches it and a tag list (`spela_core::fake::dlls`), and its library DLLs are PE stubs with versions (`spela_core::fake::pe`, a port of the Phase 0 generator's `peStub`), some older than the manifest's, so every flow runs end to end in the demo. The demo's copy of `config.toml` loses `dll_cache_path` and `backup_path`, so demo payloads and backups stay in its temp dir.

### Phase 6: launch and privilege

- Wrapper mode as set in [Launch and privilege](#launch-and-privilege): behind the router from Phase 1 (decision 21), path-based game detection, one plan for dry run and launch, env build with last-wins dedupe (verified need: [parity-inventory.md](parity-inventory.md), domain 15), preflight and KDE VRR.
- The privileged apply through the long-lived `apply-profile --serve` helper, one pkexec per launch, with each restore registered before the game starts ([../AGENTS.md](../AGENTS.md); decision 30). Restore returns prior values, including clocks and fan where NVML can read them, on a restore request or on stdin EOF. An unreadable SMT state is skipped, not set to off. A partial apply restores what did apply (verified Go bugs: [parity-inventory.md](parity-inventory.md), CLI 7.2).
- The launch lock from the first change to the last restore (decision 29). A second launch while it is held goes ahead with env-only settings and warns (decision 33).
- Signal handling starts before the privileged apply (verified Go bug: [parity-inventory.md](parity-inventory.md), CLI 6.7). The game's own exit status is passed on (decision 8; [terminal-ux.md](terminal-ux.md), CLI output). Real NVML VF offsets, wired and tested on fakes, live-unverified until the user's root test (decisions 9 and 32).
- The live `gpu` and `cpu` commands go through the one-shot `apply-profile --once` helper, which fixes the per-CPU pkexec and the `cpu smt` typo that disabled SMT (verified: [parity-inventory.md](parity-inventory.md), CLI 1 and 3.10).
- The TUI's hardware actions, GPU reset, CPU governor and SMT, as `Cmd::Hardware` jobs behind confirmations that open on Cancel, and the profile page's read-only Preview launch (decision 44; see [TUI process](#tui-process) and [terminal-ux.md](terminal-ux.md)). They land here because they need the privilege backend and the launch plan.
- The dry-run summary drops the `overlay:` impacts group and the Overlay block, shows `PROTON_VKD3D_HEAP` and lists VRR. Cleanup runs hardware, then KDE VRR (decision 10; [parity-inventory.md](parity-inventory.md), CLI 6.8). `SPELA_OVERLAY_IPC` is never set. `__GL_THREADED_OPTIMIZATION` is exported only when a profile sets it (decision 47). `spela launch` without `--dry-run` exits 2 with its hint (decision 40).
- The polkit policy annotates `exec.path` and `exec.argv1`, and wrapper mode refuses to run as root (decision 35). Go's `apply-profile --reset` gets a clear error (decision 34). A corrupt or missing game cache never blocks a launch (decision 49).
- Closes:
  - CLI 3.4, the live parts of 3.9 and 3.10, 3.17, 5.1, 5.2, 6 and 7;
  - domain 4, 5, 9 and 13, and the write side of domain 10 and 11 (NVML setters, the per-fan rule with its fan-0 fallback, and the sysfs governor and SMT writes);
  - additions B.4.
- Go deleted: nothing (decision 62).
- Proves, in tests against fakes: every applied change has a restore registered before spawn, and restore runs on exit, SIGINT, SIGTERM, SIGHUP, a failed apply and the wrapper's death (stdin EOF). The TUI hardware jobs take the launch lock and never let pkexec reach the tty.
- Proves, live and by the user only: one launch of Cyberpunk 2077 each under Steam, gamescope and Big Picture. Each shows one prompt at most, and the hardware reads back at its prior values after a normal exit, Steam's Stop button and a killed wrapper (decision 30). This is the live launch check that gates the final step.
- As built, the launch track (the privileged backend is a separate track):
  - `spela-core/src/launch/` holds the plan: `env.rs` (Go's variables, last value wins), `hardware.rs` (the `HardwareRequest` and the VRR policy from the profile; SMT left unchanged adds no step), `summary.rs` (the dry-run text as tagged spans, which the CLI paints and the wrapper and Preview print plain), `detect.rs` (game detection with and without the cache) and `vrr.rs` (KDE VRR over the `Display` trait). `spela-core/src/kscreen.rs` parses `kscreen-doctor -o`. The `Display` trait's `Output` gained `connected`, `priority` and the policy text, which the selection needs.
  - The binary has `wrapper.rs` (the sequence above, with the restores in a stack run under `catch_unwind`), `display.rs` (the real `kscreen-doctor` backend), `plan.rs` (what the dry run, the wrapper and Preview share) and `tui/exec/preview.rs`.
  - The summary names pkexec through the binary's privilege module, so `spela-core` never names it. A plan built outside `/usr/bin/spela` says pkexec asks with its generic action, which every test run shows.
  - Wrapper warnings that Go printed with `Warning:` keep that form on stderr and go to the log at info. Go's slog warnings, such as `failed to apply KDE VRR policy` and `restore failed area=<a>`, are tracing warnings, which the wrapper's stderr layer shows. Restore areas print as Go's slog did: `area=hardware`, `area="KDE VRR"`.
  - `launch --dry-run` is compared with every Go transcript in `crates/spela/tests/go_transcripts.rs`, with each difference listed with its reason. `cpu set --smt unchanged` and the profile picker's `unchanged` choice set decision 65's stored form.

### Phase 7: the dlss-updater port and release

- Port `tools/dlss-updater` to Rust (decision 14), as set in [dlss-updater in Rust](#dlss-updater-in-rust). Whether "new" compares versions is decided here.
- `dll-manifest.yml` builds the Rust tool. The manifest URL does not move (decision 14; verified contract: [parity-inventory.md](parity-inventory.md), DLL 1.1). The Go `tools/dlss-updater` stays in the tree, unused, until the final step.
- The release workflow from [CI](#ci) replaces the Go build in `release.yml`. The AUR package builds with cargo and installs the binary and the polkit policy, with its `exec.argv1` annotation (decision 35), to the same paths. The source install now installs the policy too (verified gap: [parity-inventory.md](parity-inventory.md), CLI 7.3 and 13.3).
- Port the intents of `tests/contracts`: the release workflow, tag-injection safety, notes from the changelog, and the dry-run-first AUR submit (verified: [parity-inventory.md](parity-inventory.md), CLI 13.4).
- Closes: CLI 13.2 to 13.4, DLL 7 and DLL 8 (coverage-inventory is replaced or dropped).
- Go deleted: nothing (decision 62).
- Proves: a tag builds the release artifact, and the updater passes its fixture tests. The manifest workflow also runs on GitHub with the Rust tool, which only the user can watch, because no test reaches the network.
- As built: `crates/dlss-updater` is the updater (`scrape.rs`, `fetch.rs`, `flags.rs`, `add.rs`), tested in `crates/dlss-updater/tests/fetch.rs` against saved pages on loopback, `tests/release.rs` (the release, manifest and DLL release workflows, tag validation, the notes script and the PKGBUILD and `just install` layout) and `tests/aur_submit.rs` (the dry-run-first AUR submit). `release.yml` and `dll-manifest.yml` run the Rust builds. `scripts/release-notes.sh` refuses a section whose heading is not dated, so a tag cannot publish an `Unreleased` section. The PKGBUILD and `just install` put `/usr/bin/spela` and the polkit policy in place.

### Final step: remove the Go tree

Decision 62 gates this on the user's live launch checks, the last proof of Phase 6, and it comes after Phase 7 so the manifest feeder never runs without an updater.

- One commit deletes the rest of the Go tree: `cmd/spela`; every package under `internal/`; `tools/dlss-updater`, `tools/coverage-inventory` and any other Go tool; `tests/e2e` (`tests/contracts` already went in Phase 7, ported to `crates/dlss-updater/tests`); the Phase 0 fixture generator in `tools/goref`; `magefile.go`, `go.mod`, `go.sum` and `.golangci.yml`; the coverage scripts and their Python test; and the Go jobs in `ci.yml`. The committed fixtures, the value oracle and the transcripts stay, because the Rust tests read them.
- Nothing in the Rust workspace imports Go, so the order inside the commit does not matter (inferred). After it, `cargo` is the only build, and the [../AGENTS.md](../AGENTS.md) layout row and Go reference test command are updated in the same commit.
- Proves: `just check` and CI pass with no Go toolchain installed.

### Later (only on evidence)

- The overlay IPC writer and the overlay fields, once a layer exists that reads them (decision 10, "until needed").
- `gpu.power_mizer`, through nvml-wrapper's `set_power_mizer_mode`, and `cpu.affinity` (verified to exist: [rust-crates.md](rust-crates.md), NVML and open questions).
- More than one GPU (decision 25).
- A `doctor` command. It has no Go counterpart. The strongest case is a missing polkit policy on source installs (verified: [parity-inventory.md](parity-inventory.md), CLI 7.3).
- Signalling the game's whole process group with process-wrap (verified as Later: [rust-crates.md](rust-crates.md), processes).
- KScreen over D-Bus instead of parsing `kscreen-doctor` (inferred in [rust-crates.md](rust-crates.md), D-Bus).
- reqwest and tokio, only if async work appears (decision 11).
- Key presets and a key editor (YAGNI: [terminal-ux.md](terminal-ux.md), keys).
- Man pages through clap_mangen.

## Risk register

| # | Risk | Likelihood | Impact | Mitigation | Trigger to act |
|---|---|---|---|---|---|
| 1 | The one prompt that the `--serve` helper needs cannot be answered under gamescope or Big Picture, for example with no polkit agent | Medium (unverified: [parity-inventory.md](parity-inventory.md), CLI open questions) | Hardware tuning is unusable in game mode | One prompt per launch, and the restore needs none (decision 30). A failed or dismissed prompt still launches with the env-only settings, as today (verified: CLI 6.1). The TUI's hardware jobs fail with a message pointing to the CLI (decision 44) | The user's live launch check |
| 2 | Restore fails after the game exits and leaves clocks, fan, governor or SMT changed | Medium (verified that Go's restore is a second, late pkexec call: [parity-inventory.md](parity-inventory.md), CLI 7.2) | High: the machine stays in a tuned state | Restore is registered before spawn, runs on every exit path and signal, and restores prior values. The helper also restores on stdin EOF when the wrapper dies (decision 30). Only a SIGKILL of the root helper itself skips it. A failed restore is logged to the state log file (decision 12) and shown on the next run (inferred) | Any "restore failed" line in tests or logs |
| 3 | Migrated `clock_offset` and `memory_offset` values change meaning under real NVML offsets (decision 9). In Go, `clock_offset: 200` locked the maximum clock at 2300 MHz, and `memory_offset` was an absolute maximum (verified: [parity-inventory.md](parity-inventory.md), CLI 7.2) | Low: the values are not copied (decision 31) | High: different hardware settings than the user chose | Not copied into the new keys. Kept in the migration record and the file header, and the notice says so (decision 31) | A fixture test that finds an offset copied |
| 4 | The Go fixtures are lost before they are captured, so the migration cannot be tested against real Go output | High (verified: the probe folder is temporary, [README.md](README.md)) | High: the migration guarantee in [../AGENTS.md](../AGENTS.md) cannot be proven | Phase 0 commits the fixtures. No Go package is deleted before its fixtures exist | Phase 3 starts without committed fixtures |
| 5 | The migration loses or misreads a value: legacy inheritance, `sr_model_preset`, 2-space files, `yes` and `no` booleans, unknown override keys | Medium (inferred; the cases are verified in [parity-inventory.md](parity-inventory.md), domain 3) | High: silent change to a user's profile | Fixture tests per case, originals kept byte-identical, idempotence, and the migration record | Any fixture diff |
| 6 | Go-era DLL backups cannot be restored from Rust | Low to medium (inferred) | High: users lose their original DLLs (verified: [parity-inventory.md](parity-inventory.md), DLL 1.3) | Fixtures for `backup.json` and both name schemes, byte-exact restore tests, and fail closed on a bad index (the Go test exists) | Any restore fixture failure |
| 7 | After `overlay` and `gui` are removed, `spela overlay ...` falls through to PATH and launches `/usr/bin/overlay` from mesa-demos as a game (decisions 10, 21 and 26) | Medium (verified that `/usr/bin/overlay` exists here: [rust-crates.md](rust-crates.md), conflicts) | Medium: an unrelated program runs, with any game profile the detector matches | `overlay` and `gui` stay reserved names that print that the command was removed in 0.7.0 and exit 2 (decision 45). A CLI test with a PATH shim named `overlay` | A test failure |
| 8 | A game launched by the Go binary is still running when the package switches to Rust. Go's restore then runs `pkexec /usr/bin/spela apply-profile --reset ...` against the Rust binary (verified mechanism: [parity-inventory.md](parity-inventory.md), CLI open questions) | Low (inferred) | High: the restore fails and the hardware stays tuned | The Go flags are not accepted, and the helper prints a clear error (decisions 13 and 34). The release notes tell users to exit Go-launched games before upgrading, and `spela gpu reset`, `cpu governor` and `cpu smt`, or a reboot, put the hardware back (inferred) | First Rust release |
| 9 | The polkit action is not scoped to `apply-profile`, so with `auth_admin_keep` any `pkexec /usr/bin/spela ...` matches it (inferred: [parity-inventory.md](parity-inventory.md), CLI 7.3) | Low once the policy annotates `exec.argv1` | High: privilege reaches further than needed | The policy annotates `exec.path` and `exec.argv1` `apply-profile`, the helper accepts only `--serve` and `--once`, and wrapper mode refuses to run as root (decisions 34 and 35) | The user's live check finds that polkit ignores `argv1` |
| 10 | Source installs lack the polkit policy, so users get the generic action and more prompts (verified: [parity-inventory.md](parity-inventory.md), CLI 7.3) | High for source installs | Medium | The install recipe installs the policy. The error text names the missing policy (inferred) | Phase 7 |
| 11 | The manifest feeder stops and no user gets new DLLs. The last bot commit was 2026-02-03, and the CI pins Go 1.22 against go.mod 1.25.5 (verified: [parity-inventory.md](parity-inventory.md), DLL 7 and DLL conflicts) | Medium (unverified whether it is failing now) | Medium: no new DLLs for Go or Rust users | Phase 0 check, Phase 7 port with fixture tests, and an alert when the workflow fails | A failed run, or no commit after a known DLSS release |
| 12 | Wall-clock perf tests flake on shared CI runners | Medium (inferred) | Low: wall-clock tests report and do not gate | Deterministic counters are the gate. Wall-clock tests get one rerun and report their numbers and buckets (decision 55) | Reported numbers drift past the budget for a week |
| 13 | NVML calls stall the metrics thread or the first frame. A sweep takes about 7 ms and init 4 to 17 ms (verified: [rust-crates.md](rust-crates.md), NVML) | Low to medium | Medium: late metrics, or a slow start | Metrics thread only, lazy init off the first-frame path, stale results dropped, and a slow-fake test | Budget failure on Monitor or at start |
| 14 | The Steam scan differs from Go: keyvalues-parser accepts more files, `StateFlags == 4` drops games with a pending update, and duplicate app ids and library order depend on map order (verified: [rust-crates.md](rust-crates.md), Steam VDF; [parity-inventory.md](parity-inventory.md), domain conflicts). Phase 0 found 8 of 59 manifests here at StateFlags 6, three of them games (verified: [phase0-checks.md](phase0-checks.md), StateFlags) | Medium | Medium: games missing or extra | A fixture library per edge case, including StateFlags 6, deterministic order, and the bit test | Transcript diff in Phase 2 |
| 15 | The Proton resolver misses tools, so the VKD3D heap gate misfires. Phase 0 showed it is worse than feared: Go resolves no mapped tool on this machine, because tools are mapped by internal or short names and most sit in a second library. `/usr/share/steam/compatibilitytools.d` does not exist here (verified: [phase0-checks.md](phase0-checks.md), Proton tool folders) | High | Medium: the notice always says it could not resolve Proton | The resolver in [Proton](#proton): internal names from `compatibilitytool.vdf`, every library's `steamapps/common`, built-in short names through app manifests, with fixture tests for each shape | Fired in Phase 0; closes with the Phase 2 resolver tests |
| 16 | `kscreen-doctor` output changes with a Plasma update and VRR parsing breaks (inferred) | Medium | Low: VRR is a warning only, and the launch continues (verified contract: [parity-inventory.md](parity-inventory.md), CLI 6.5) | `LC_ALL=C`, a 10 s timeout, fixture outputs from Phase 0, fail soft | A parse failure in a log |
| 17 | Two processes resolve different lock paths and stop excluding each other. Go kept locks in the runtime dir with a `/tmp` fallback, and spec-strict etcetera returns no runtime dir when `XDG_RUNTIME_DIR` is unset (verified: [rust-crates.md](rust-crates.md), XDG paths) | Low once locks live in `<state>/locks/` (inferred) | Medium: two launches can interleave apply and restore | Locks under `<state>/locks/`, never in the runtime dir. Lock files are never unlinked. A test checks that the TUI, wrapper and CLI resolve the same path with and without `XDG_RUNTIME_DIR` (inferred) | A lock test failure |
| 18 | A panic on a long-lived TUI thread leaves the UI drawing on a restored screen, or with no input. mejl has this gap (verified: [mejl-study.md](mejl-study.md), binary 5.3 and binary conflicts) | Low to medium | Medium: a broken session that looks alive | Job-pool panics are caught and reported as errors. A panic on the input, timer, signal, metrics or write-queue thread is fatal: restore the terminal, then exit. A pty test covers it (inferred) | A pty test failure, or a report |
| 19 | The new `--json` schema breaks scripts or AUR users that read the Go field names (verified current shape: [rust-crates.md](rust-crates.md), JSON) | Low (unverified who reads it) | Low to medium | A schema version field and a changelog note (decision 7) | A user report |
| 20 | TLS and proxy behaviour differ from Go: ureq honours proxy env, while Go's DLL client ignores `HTTPS_PROXY` (verified: [rust-crates.md](rust-crates.md), HTTP and TLS) | Low | Medium: downloads fail behind a custom CA or proxy | The platform verifier, as the starter set picks. Proxy variables are honoured (decision 41) | A failed fetch report |
| 21 | serde-saphyr churn: 2.0.0 is weeks old, with two majors in three months (verified: [rust-crates.md](rust-crates.md), traps) | Medium | Low: it now only reads Go YAML for the migration | Pin it exactly, wrap it in the migration module, never write YAML | A new major or an advisory |
| 22 | MSRV 1.89 has never been built exactly (verified: [rust-crates.md](rust-crates.md), open questions) | Low | Low | The MSRV job in CI from Phase 1, on a 1.89 toolchain installed with rustup (decision 64) | The MSRV job fails |
| 23 | Mouse capture on by default takes text selection away in some terminals (verified tension: [mejl-study.md](mejl-study.md), mejl-ui open questions) | Medium | Low | `ui.mouse = false`, and Help mentions Shift for selection ([terminal-ux.md](terminal-ux.md), mouse) | User report |
| 24 | Users run the old Go binary after migrating. Go keeps writing YAML, which Rust no longer reads, so the two states drift (inferred from decision 13) | Low to medium | Medium: changes seem lost | The migration notice says the Go files are backups only. There is no warning when a Go file changes later (decision 43) | A user report |

## Where the sources disagree

Each disagreement is resolved here. Nothing was picked silently. Rows that decisions 30 to 64 settled cite the decision. Rows marked (inferred) are design choices of this doc that no decision contradicts. "Drafts" means the section drafts this doc was merged from, where they gave different answers to the same question.

| # | Topic | What the sources say | Resolution |
|---|---|---|---|
| 1 | Idle against top-bar metrics | [terminal-ux.md](terminal-ux.md) principle 4 and its budget row: a screen without metrics has no wake-ups. The same doc's frame and every screen in [mockups.html](mockups.html) put live metrics in the top bar on every destination, so no normal screen lacks metrics | Resolved: tick every 2 s while the top bar is visible, none below 80x24, while suspended or with no metrics source, and no draw when the shown text is unchanged (inferred). No pause on focus lost (decision 42) |
| 2 | A write queue against "no store thread" | [README.md](README.md) finding 1: spela needs no store thread. [terminal-ux.md](terminal-ux.md): saves run on a worker and a failure puts the stored value back | Resolved (inferred): reject mejl's SQLite store thread, adapt only its single-writer, in-order pattern, with a fresh-descriptor flock per write, per-key merging of queued writes, and a join after the terminal is restored on quit |
| 3 | The overlay IPC writer, its reader and memmap2 | [rust-crates.md](rust-crates.md) picks memmap2 for the IPC, proposes a `spela-system` crate with IPC, and says a Rust writer must follow the code because an external Zig layer reads it. [parity-inventory.md](parity-inventory.md) DLL 1.4 and 4.8 say no consumer exists | Resolved by decision 10: the writer, every overlay field, the Overlay section and the command go. memmap2 leaves the dependency set. No `spela-system` crate. The alert evaluator moves to `spela-core::alerts` |
| 4 | XDG semantics | [parity-inventory.md](parity-inventory.md) CLI 10 and [rust-crates.md](rust-crates.md) (XDG paths, a hand-written module) keep Go's rules: relative values accepted and a `/tmp/runtime-$USER` fallback. mejl uses etcetera | Resolved by decision 23: etcetera, spec strict. Relative `XDG_*` values are ignored with no warning (decision 43) |
| 5 | Where locks live | Drafts: one put locks in `<state>/locks/` and never uses the runtime dir. One put them in `$XDG_RUNTIME_DIR/spela/` with a fallback under the cache dir. Two left it open with the state dir proposed. Go uses the runtime dir with a `/tmp` fallback ([parity-inventory.md](parity-inventory.md), CLI 10) | Resolved (inferred): `<state>/locks/`. A runtime dir that may be missing would let two processes lock different files. Risk 17 tracks it |
| 6 | Log destination | [parity-inventory.md](parity-inventory.md) CLI 9 makes stderr-only logging a contract. [rust-crates.md](rust-crates.md) lists "stderr or file" as open and marks tracing-appender Later. mejl logs to rolling files only | Resolved by decision 12: daily rolling files under `<state>/log/`, 7 kept, plus stderr for CLI warnings. tracing-appender moves to Pick. The root helper writes no file |
| 7 | YAML crate and byte contract against TOML | [parity-inventory.md](parity-inventory.md) domain 3 pins yaml.v3 bytes, and [rust-crates.md](rust-crates.md) tuned serde-saphyr to write them. Decision 6 makes TOML the format | Resolved by decision 6: YAML is only read during migration. serde-saphyr stays, pinned exactly behind one module. The byte layout is no longer a contract |
| 8 | TOML crates missing from the starter set | The [rust-crates.md](rust-crates.md) starter has no toml or toml_edit. [mejl-study.md](mejl-study.md) binary 10.3 says mejl's toml_edit writer does not transfer because spela used YAML. Decision 6 makes spela use TOML | Resolved (inferred): add toml 1.1.8 and toml_edit 0.25.17, as mejl pins them. Their MSRV of 1.85 is verified locally. Their build cost with spela's set is unmeasured |
| 9 | Crate split | [rust-crates.md](rust-crates.md) proposes `spela-core`, `spela-system` and `spela-ui` plus the binary. mejl has eight crates ([mejl-study.md](mejl-study.md), binary 13.1) | Resolved (inferred): four crates. Real backends live in the binary, the fake in `spela-core`, and `dlss-updater` is separate because decision 14 ports it and it shares the manifest types |
| 10 | Input thread design | mejl's architecture doc describes a blocking crossterm read. mejl's code uses `EventStream` under a futures executor so it can pause for `$EDITOR` and Ctrl-Z ([mejl-study.md](mejl-study.md), binary 2.4 and 8) | Resolved provisionally: a blocking read, because spela has no `$EDITOR` handoff. A new pty scenario that runs spela's input thread across Ctrl-Z decides. mejl's `suspend_process` scenario cannot, because it runs no input thread (verified: `~/git/mejl/crates/mejl/tests/terminal_pty.rs:439-459`). If the new test fails, spela takes mejl's `EventStream` thread |
| 11 | Dirty flag, BufWriter and idle in mejl | mejl's docs promise drawing only when dirty, a BufWriter, and no wake-ups at idle. Its code draws every batch, writes to plain stdout, and ticks at 1 Hz ([mejl-study.md](mejl-study.md), binary 4.2 and 4.4) | Resolved (inferred): follow the docs' intent and build all three |
| 12 | Sync markers and the BufWriter | mejl writes the synchronized-output markers and OSC 52 through separate `io::stdout()` handles ([mejl-study.md](mejl-study.md), binary 2.3), which would reorder bytes behind a BufWriter | Resolved: every terminal write goes through the one BufWriter, through `terminal.backend_mut()` (inferred). ratatui's `draw` flushes before it returns (verified: ratatui-core-0.1.2 `src/terminal/render.rs:189, 204, 308`), so the end marker leaves in a second small write, which is still correct (inferred) |
| 13 | The update signature | mejl's docs: keys go through the keymap into an Action before `update(&mut State, Action)`. Its code feeds raw events into update ([mejl-study.md](mejl-study.md), mejl-ui conflicts) | Resolved (inferred): the code shape, `update(&mut AppState, Event) -> Vec<Cmd>` |
| 14 | Fuzzy matcher licence | mejl wrote its own matcher because nucleo-matcher is MPL-2.0 ([mejl-study.md](mejl-study.md), mejl-ui palette). Decision 15 allows unmodified MPL-2.0 | Resolved (inferred): reuse `fuzzy.rs` anyway, because it needs no dependency |
| 15 | Sharing `games.lock` and files with the Go binary | [parity-inventory.md](parity-inventory.md) DLL 10 suggests keeping the `games.lock` path so both binaries can share `games.yaml` during a transition. Decision 13 rules out side-by-side support | Resolved by decision 13: the Rust game cache has its own name and lock, and `games.yaml` only seeds it once |
| 16 | pkexec in the TUI job pool | A draft listed pkexec round trips among the TUI's worker jobs. A grep of spela `internal/tui` finds no call into the privilege package | Resolved by decision 44: the pool carries a `Cmd::Hardware` job for GPU reset, CPU governor and SMT, which runs the one-shot helper with pkexec's internal agent off and a null stdin (see [TUI process](#tui-process)) |
| 17 | Launch lock scope | Decision 29: "around apply and restore". Drafts: one drew two short holds, one held it from apply to restore, two left it open. [parity-inventory.md](parity-inventory.md) CLI open questions want the lock to stop interleaved apply and restore, which two short holds do not do | Resolved: held from the first change until the last restore ends (inferred), and a second launch meanwhile goes ahead with env-only settings, skips VRR and hardware, and warns (decision 33) |
| 18 | VRR and privileged apply order | A draft's wrapper diagram applied the privileged step first. [parity-inventory.md](parity-inventory.md) CLI 6.8 verifies VRR applies first and restores last | Resolved by the verified order: VRR first, hardware second, cleanup in reverse |
| 19 | The Go TUI's 100 ms flash tick | The Go TUI has one for the message bar (`internal/tui/messagebar.go:20`). [terminal-ux.md](terminal-ux.md) keeps the 5 s clear and uses words, not spinners, and says nothing about the flash | Resolved (inferred): not carried over |
| 20 | UI thread disk writes in mejl | mejl's principles say the UI thread never blocks. Its settings screen writes the config with `sync_all` on the UI thread ([mejl-study.md](mejl-study.md), binary conflicts) | Resolved (inferred): the toml_edit writer is adopted but runs on the write queue |
| 21 | MSRV and CI | mejl pins 1.88, runs tests only on tags and never checks its MSRV ([mejl-study.md](mejl-study.md), binary 13.4). Its justfile passes `--all-targets` to the MSRV check and its AGENTS.md does not (binary 13.3) | Resolved by decisions 16 and 17: every push is gated, and `cargo +1.89 check --workspace --all-targets --locked` runs in CI and in `just msrv` |
| 22 | DLL backups: migrate or read in place | [../AGENTS.md](../AGENTS.md) lists the DLL backup layout among the Go files to migrate "to the rewrite's own shape". [README.md](README.md) finding 7 and [parity-inventory.md](parity-inventory.md) DLL 1.3 say `backup.json` and both name schemes must stay readable | Resolved by decision 50: read in place, never moved or reshaped. New data goes in new top-level keys that Go ignores, never in `files[]`. The [../AGENTS.md](../AGENTS.md) hard rule is reworded to say so |
| 23 | Meaning of `rescan_on_startup` | [parity-inventory.md](parity-inventory.md) CLI 3.2 and 3.3: `list` and `show` rescan when it is on. Decision 24: scan once in the background, and the CLI rescans only on an empty cache or `spela scan`. [mockups.html](mockups.html) still shows "Re-scan on startup" | Resolved: the key keeps its name and only switches the TUI's background scan. A draft had left this open |
| 24 | New file names against files kept in place | Decision 13: "new file names, the Go files stay as migration backups". Backups, the manifest cache, the DLL cache and `update-check.json` keep their Go names | Resolved by decision 53: the manifest cache and the update check become `<cache>/manifest-v1.json` and `<cache>/release-check.json`, seeded once from Go's files. Backups keep Go's names (finding 7; decision 50). The DLL cache folder stays shared with Go |
| 25 | Go's `theme` key | [rust-crates.md](rust-crates.md) conflicts: a rewrite must round-trip the GUI-only `theme` or lose it. Decision 27 adds a TUI theme with a different meaning. Go's values are default, dark and light ([parity-inventory.md](parity-inventory.md), CLI 11) | Resolved by decision 46: `theme`, `show_hints`, `preferred_dll_source` and unknown keys are dropped and recorded in `migration.toml` and the notice, never mapped to `ui.theme`. The round trip is moot, because the YAML is never rewritten |
| 26 | `compact_mode` | [terminal-ux.md](terminal-ux.md) frame: the migration drops it "with a note (open question)". Decision 3 drops it | Resolved by decision 3: dropped and recorded. The notice mentions it only when it was true. terminal-ux.md's frame section now cites decision 3 |
| 27 | Exit code for a bad config in the TUI | [parity-inventory.md](parity-inventory.md) CLI 3.14: `spela tui` prints a recovery message and exits 0, pinned by a test. [terminal-ux.md](terminal-ux.md): exit 3 for config errors | Resolved: exit 3, with the recovery message naming the TOML file, or the YAML file whose migration failed |
| 28 | Which profile files Go loads | [parity-inventory.md](parity-inventory.md) domain 3: `List()` reads `.yaml` and `.yml`. The code: `Load` always opens `<appid>.yaml` (`internal/profile/storage.go:29-31, 260-297`) | Resolved by the code: migrate only `<numeric>.yaml` and `default.yaml`. A `.yml` file had no effect and is recorded as skipped |
| 29 | Deny lists that do not parse | [parity-inventory.md](parity-inventory.md) DLL 3: a malformed list is ignored, so swaps fail open (contract, flagged as a hazard). Decision 8: port the fix | Resolved: fail closed for install and update. Restore still works |
| 30 | User deny list: replace or merge, and allow against deny | [parity-inventory.md](parity-inventory.md) DLL 3: a user list replaces the built-in list (pinned by a test), and allow always beats a user deny. Its DLL conflicts flag replace as a hazard. Drafts: one kept Go's semantics and left both open, one proposed merge and most-recent-wins | Resolved by decision 36: the user list adds to the built-in list. Allow and deny for one game are exclusive, so setting one clears the other. When a Go file lists a game in both, allow wins |
| 31 | `--json` shape | [parity-inventory.md](parity-inventory.md) CLI 14 marks the Go-name JSON (AppID, null slices, HTML escaping) as a contract, and [rust-crates.md](rust-crates.md) builds a Go-escaping formatter. `profile show --json` prints the stored profile and `<section> show --json` prints resolved values (CLI conflicts). `scan --json` is a map with tools, `list --json` an array without (additions B.5) | Resolved by decision 7: schema version 1. One `field` shape carries effective, own and All games values with the origin. Every profile view works without a profile file. `scan` and `list` both return a sorted `games` array without tools |
| 32 | Keeping every value when Go writes every config key | [../AGENTS.md](../AGENTS.md): a migration keeps every value the user set. Go's `config.Save` writes every key (`internal/config/config.go:22-46, 84-96`), so a config cannot show which values the user chose | Resolved by decision 43: write a key only when it differs from Go's default, and list every key not written in the record |
| 33 | Keeping every value against decision 10 | [../AGENTS.md](../AGENTS.md): keep every value. Decision 10: drop the overlay fields, `power_mizer` and `cpu.affinity`, and note what was dropped | Resolved by decision 10, the explicit exception: the values go into the record and the file header only. Nothing a user can see changes, because none had a reader |
| 34 | Offset migration | Decision 9 makes `clock_offset` and `memory_offset` real offsets, while Go stored a clock lock and an absolute maximum ([parity-inventory.md](parity-inventory.md), domain 1). [../AGENTS.md](../AGENTS.md) keeps every value. Drafts: two proposed not copying the values into the new keys, one proposed keeping them inert until confirmed | Resolved by decision 31: not copied into the new keys. Kept in `migration.toml` and the profile file header, and the migration notice says so |
| 35 | Meaning of Space on profile fields | `docs/design/tui-space-and-dll-controls-acceptance.md`: Space cycles default, true, false. The code, `tui-navigation.md` and [parity-inventory.md](parity-inventory.md) TUI 9: Space switches between own value and following | Resolved in [README.md](README.md) finding 4 for the code. The data model writes and deletes keys to match, and the remembered value now lasts the session |
| 36 | fsync policy | Go never fsyncs ([rust-crates.md](rust-crates.md), atomic file writes). mejl syncs its config but not `caps.toml` ([mejl-study.md](mejl-study.md), binary 10.3 and 7.3) | Resolved (inferred): user intent, the migration record and backups are synced with a folder fsync. Caches are not |
| 37 | Retired subcommands against PATH launch | Decisions 10 and 26 remove `overlay` and `gui`. Decision 21 makes anything on PATH that is not a known subcommand a launch, and `/usr/bin/overlay` exists here ([rust-crates.md](rust-crates.md), conflicts). Drafts: one reserved the names, one left it to Phase 0 | Resolved by decision 45: reserved names that print that the command was removed in 0.7.0 and exit 2. A program on PATH named `overlay` is never launched for them |
| 38 | Starter crate version against release tags | The [rust-crates.md](rust-crates.md) starter sets `version = "0.1.0"`. [parity-inventory.md](parity-inventory.md) DLL 1.2: Rust tags must be newer than v0.6.2 | Resolved (inferred): start above 0.6.2, for example 0.7.0, and embed the tag's version in release builds |
| 39 | `__GL_THREADED_OPTIMIZATION` export | [parity-inventory.md](parity-inventory.md) domain conflicts: Go exports 1 or 0 whenever any profile applies, so a system default forces it off. Domain open questions ask whether to keep that | Resolved by decision 47: export it only when the game or All games sets the field |
| 40 | `panic = "abort"` | [rust-crates.md](rust-crates.md) gives a 4.2 MB size with `panic = "abort"`. [../AGENTS.md](../AGENTS.md) requires the restore to run | Resolved (inferred): keep `panic = "unwind"` and run the launch body under `catch_unwind` |
| 41 | Single-instance lock mechanism | Go uses a PID file plus `kill(pid, 0)` ([parity-inventory.md](parity-inventory.md), CLI 8). [rust-crates.md](rust-crates.md) keeps that as default, with flock as the alternative | Resolved by decision 29: flock, with the PID written only for the message |
| 42 | Lock file removal | Go removes `spela.pid` on release, and the e2e quit check depends on it. mejl's flock lock never removes its file (`lock.rs:128-173`) | Resolved (inferred): the file stays in place, so two processes never lock different inodes. Tests probe the lock or wait for the process |
| 43 | Release notice text | [parity-inventory.md](parity-inventory.md) DLL 5: `Spela vX is available (you have vY)` in the header. [terminal-ux.md](terminal-ux.md): `spela vX is available` right of the status line | Resolved for [terminal-ux.md](terminal-ux.md), the decided spec |
| 44 | NVML setters against the `nvidia-smi` fallback | Go's AGENTS.md: `apply-profile` uses go-nvml setters. [parity-inventory.md](parity-inventory.md) CLI conflicts: the code falls back to `nvidia-smi`, and the live commands use `pkexec tee` or `nvidia-smi` | Resolved by decision 58: NVML only, with no `nvidia-smi` fallback, and every privileged write through the helper. When NVML is missing, GPU features say so |
| 45 | Signal policy: mejl against wrapper mode | mejl exits 128 plus n on a second stop signal ([mejl-study.md](mejl-study.md), binary 6). Its open questions say wrapper mode must not inherit that | Resolved (inferred): the wrapper forwards every signal and never exits early. The TUI keeps mejl's policy |
| 46 | VDF parser parity | [parity-inventory.md](parity-inventory.md) domain open questions ask whether to keep the line parser. [rust-crates.md](rust-crates.md) picks keyvalues-parser, which differs on escapes, comments, inline braces and duplicates | Resolved by decision 52: keyvalues-parser, taking the last duplicate as Go does, and accepting the other differences |
| 47 | Alert precedence | [parity-inventory.md](parity-inventory.md) DLL conflicts: the comment says thresholds apply only when every NVML flag is false, the code falls back to them whenever one is false, so 86 °C without a throttle reason says "Throttling" | Resolved: "Throttling" only on a thermal reason, otherwise "High temp" at critical (inferred). The visible change is accepted (decision 60) |
| 48 | Exit code for the launch rejection | [parity-inventory.md](parity-inventory.md) CLI 3.4: Go exits 1. [terminal-ux.md](terminal-ux.md) adopts mejl's codes, where 2 is usage | Resolved by decision 40: exit 2, with a hint to put `spela %command%` in Steam's launch options |
| 49 | Broken profile at launch | Drafts: one blocked the launch on a broken game or All games profile, as Go's `launch` does. One never let wrapper mode fail on spela's own data. [parity-inventory.md](parity-inventory.md) CLI conflicts: `launch` fails, the wrapper only warns but drops valid defaults | Resolved (inferred): per surface. The CLI and `launch --dry-run` exit 3. The wrapper warns and falls back to All games, or to no profile |
| 50 | Worker panics | Drafts: one caught panics in jobs and turned them into errors, one treated any worker panic as fatal | Resolved (inferred): job-pool panics become errors. A panic on a long-lived thread is fatal after a terminal restore (risk 18) |
| 51 | The Go `apply-profile --reset` handover | A draft said decision 13 settles it: the Go flags are not accepted. Another proposed accepting them once, for a game launched under Go before the switch ([parity-inventory.md](parity-inventory.md), CLI open questions) | Resolved by decision 34: not accepted. The helper prints a clear error. Risk 8 covers a game still running under Go at the switch |
| 52 | `--demo` scope | Drafts: one ran the demo as the TUI only, one made `--demo` a global flag for every command | Resolved (inferred): global, for the user. Binary tests select the fake with the hidden `SPELA_BACKEND=fake` instead, which keeps their own `Paths`. `--demo` builds on that selector. Wrapper mode and game launches are refused under `--demo` |
| 53 | Who owns NVML | Drafts: one kept `Nvml` in a `OnceLock` looked up per call, one gave the metrics thread sole ownership | Resolved (inferred): lazy and one owner per process. In the TUI that is the metrics thread |
| 54 | DLL cache and `games.lock` | Drafts: one put the DLL cache under `games.lock`, as Go holds it through downloads. One downloaded before taking the lock | Resolved (inferred): download outside the lock into the checksum-verified cache by atomic rename. `games.lock` covers backup, replace, rescan and save |
| 55 | HTTP fake: in-process or loopback | Drafts: one used an in-process URL map, one a loopback `TcpListener` | Resolved (inferred): both, at different layers. The map serves unit tests and the demo. Loopback tests the real ureq backend. Neither ships a server |
| 56 | Root helper logging and profile writes | Drafts: two left the helper's log open, two said it writes no file. One left toml_edit for profiles open, one used it | Resolved (inferred): the helper writes no file and the parent logs its JSON lines. Profiles are written with toml_edit, which keeps comments |
| 57 | Latency sampling granularity | mejl's architecture doc records `input_to_flush` per input event. Its code records one sample per frame from the oldest event ([mejl-study.md](mejl-study.md), binary 4.3) | Resolved (inferred): per input event, because the budget is per key |
| 58 | mejl's end-to-end terminal tests | mejl's architecture doc says the real binary runs in a pty and the screen is read with vt100. Its code re-runs the test binary and checks escape bytes ([mejl-study.md](mejl-study.md), binary 12.1) | Resolved (inferred): keep mejl's byte checks and add vt100 screen reads of the real release binary |
| 59 | Latency budgets: claimed against enforced | mejl's architecture doc says budgets are enforced in CI. Its tests assert only debug bounds of 100, 60, 30 and 250 ms, and CI runs only on tags | Resolved by decision 55: deterministic counters are the gate. The wall-clock budgets run in release builds on every push, with one rerun, and report their numbers |
| 60 | The Go e2e oracle | [parity-inventory.md](parity-inventory.md) TUI 22 calls the e2e journeys the best black-box oracle. They import `internal/dll` and `internal/profile`, wait for `spela.pid` to vanish, assert strings [terminal-ux.md](terminal-ux.md) changes, and use YAML fixtures | Resolved by decision 54: not kept. Rust has its own pty and termctrl checks, plus the Phase 0 transcripts. `tests/e2e` goes with the Go tree in the final step (decision 62) |
| 61 | Terminal Control version | [../AGENTS.md](../AGENTS.md) requires termctrl 1.2.1 or newer. Go CI installed 0.3.1 in its e2e job, which Phase 1 removed | Resolved (inferred): any CI job that runs termctrl uses 1.2.1 or newer |
| 62 | The 35-field descriptor guard | [parity-inventory.md](parity-inventory.md) domain 15 keeps a 35-field count as a guard. Decision 10 drops ten fields | Resolved (inferred): the guard counts 25 |
| 63 | Snapshot sizes | mejl snapshots at 80x24, 120x36 and 160x40. spela's [../AGENTS.md](../AGENTS.md) reviews at 120x40 and 80x24, and `docs/tui-testing.md` also uses 72x20 | Resolved (inferred): 80x24 and 120x40, plus the resize prompt at 72x20 |
| 64 | CLI and TUI parity against TUI-only checks | Decision 20 makes the features equal. The release notice and the startup DLL auto-update stay TUI only, as in Go ([parity-inventory.md](parity-inventory.md), DLL 5 and 2.9; DLL open questions) | Resolved by decision 39: the release notice and the startup DLL auto-update stay TUI only, and the CLI stays quiet. The operation is on both surfaces, because the CLI has `dll update --all` |
| 65 | Warning when a Go file changes after migration | A draft attributed a once-only warning to decision 13, which rules out side-by-side support but says nothing about a warning | Resolved by decision 43: no warning when a Go file changes after migration |
| 66 | Live hardware commands and the TUI | Decision 20: equal features on both surfaces. Decision 4: keep confirmations for DLL and hardware changes. The Go TUI contract: the TUI does not launch games or apply privileged tuning (verified doc and test: [parity-inventory.md](parity-inventory.md), TUI 19; `docs/design/tui-navigation.md:5-7`). A grep of `internal/tui` finds no live `gpu reset`, governor or SMT action. This doc keeps them CLI only, so decision 4's hardware confirmations have no screen | Resolved by decision 44: the live `gpu reset`, `cpu governor <g>` and `cpu smt <v>` come to TUI Actions as a `Cmd::Hardware` job, behind a confirmation that opens on Cancel, taking the launch lock. The profile page gains a read-only Preview launch. Decisions outrank the Go TUI doc, so TUI 19's "no privileged tuning" is reversed, and "no launch" still binds |

## Open questions

Decisions 30 to 64 settle every design question this section used to hold. What is left are checks that only the user may run, on this machine, because they change hardware, run pkexec, launch a game or use the network ([../AGENTS.md](../AGENTS.md), hard rules). Each row says what it settles and when it is needed. Read-only checks are Claude's and are recorded in [phase0-checks.md](phase0-checks.md) (decision 63). Checks that tests settle are listed in the phase that runs them: the Ctrl-Z pty scenario in Phase 4 and the serde-saphyr fixture cases in Phase 3. Phase 1 settled the log interleave and the toml build at 1.89.

| Live check, by the user | What it settles | When |
|---|---|---|
| The live launch check: one launch each under desktop Steam, a gamescope session and Big Picture, with the binary at `/usr/bin/spela` and the real polkit policy. Count the prompts, including after `auth_admin_keep` has expired and with no polkit agent running. Note whether pkexec execs the helper or stays as its parent, and whether it falls back to a text agent on a tty (unverified) | That the `--serve` helper shows one prompt per launch (decision 30) | End of Phase 6. It gates the final step (decision 62) |
| In the same launches: the restore after a normal exit, after Steam's Stop button (what Steam sends the wrapper is unverified), after a SIGKILL of the wrapper, and after a logout during the game. The hardware must read back at its prior values | That the restore request and the EOF restore cover every exit path (decision 30) | End of Phase 6 |
| The NVML root test on GPU 0: set, read back and restore `set_gpc_clock_vf_offset` and `set_mem_clock_vf_offset`, and the locked-clock, power and fan setters. Record the allowed offset range, what each reset restores, and whether NVML reads the prior locked-clock state and the fan control mode | That the wired offsets are safe, and the offset bounds in [Partial-failure rules](#partial-failure-rules) (decisions 9 and 32) | Before the user sets an offset |
| With the policy installed, `pkexec /usr/bin/spela <anything but apply-profile>` falls back to the generic action, so polkit honours `org.freedesktop.policykit.exec.argv1` on the user's version (unverified) | The policy scoping (decision 35; risk 9) | Phase 6 or 7 |
| A TUI hardware action with and without a graphical polkit agent. pkexec with `--disable-internal-agent` and a null stdin must never prompt on the tty. Note how pkexec reports the missing agent, by exit code or by message | The no-agent failure message (decision 44) | Phase 6 |
| `SteamAppId` and `SteamGameId` reach the launch environment, and the reaper's arguments carry `AppId=<id>` (unverified) | Game detection, with and without the game cache (decision 49) | Phase 6 |
| A real KDE VRR apply and restore through `kscreen-doctor` | The VRR step beyond the read-only parser fixture | Phase 6 |
| SMT off and on, then the governor restore: do re-onlined CPUs get the restored governor, and does a write to an offline CPU fail? | The SMT-before-governor restore order | Phase 6 |
| A real launch with dxvk-nvapi: does it accept SR mode `off`, `render_preset_default` and multi-frame 4, does the DLSS indicator persist, and does `PROTON_ENABLE_NGX_UPDATER=1` override a swapped DLL? | Help and doc text only. The env builder keeps Go's values either way (decision 59) | Any time |
| Real DLL swaps and restores on a game the user picks, including one with a Go-era backup | The DLL path on real files, beyond the fixtures | End of Phase 5 |
| The GitHub tags API with `User-Agent: spela/<version>`, the TLS platform verifier, and a fetch through a proxy | The release check and the manifest fetch on the real network (decision 41) | Phase 5 |
| Whether the daily dlss-updater job still succeeds, and the first run of the Rust updater on GitHub | The manifest feeder (risk 11) | Phase 0 for the Go job, Phase 7 for the Rust one |
