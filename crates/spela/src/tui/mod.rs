//! The TUI shell: the terminal, the threads and the loop around
//! [`spela_ui`].
//!
//! | Module | Role |
//! |---|---|
//! | [`terminal`] | Modes, reverse restore, suspend, the panic hook |
//! | [`signals`] | SIGTERM, SIGINT, SIGHUP and SIGCONT |
//! | [`probe`], [`caps`], [`quirks`] | The capability probe, its cache and its quirks |
//! | [`events`] | The loop's one channel |
//! | [`input`] | The blocking input thread |
//! | [`timer`] | The metrics tick and wake-ups |
//! | [`host`] | Suspend, with a recording fake |
//! | [`app`] | The loop |
//! | [`metrics`] | The metrics thread |
//! | [`jobs`] | The job pool |
//! | [`writes`] | The write queue |
//! | [`exec`] | Running commands, one file per area |
//!
//! Startup order (architecture.md, the event loop): the TUI lock (taken by
//! [`crate::cli`] before this runs), a terminal check, signals, the
//! migration and the config (errors reported before the terminal changes),
//! the capability probe with its cache, terminal enter, the input, timer,
//! metrics, job and write threads, the first frame from the game cache,
//! then the background scan (decision 24).
//!
//! On quit the loop exits, the terminal is restored, and then the write
//! queue is joined, off the UI path. A save that failed is printed after
//! the restore.

pub mod app;
pub mod caps;
pub mod events;
pub mod exec;
pub mod host;
pub mod input;
pub mod jobs;
pub mod metrics;
pub mod probe;
pub mod quirks;
pub mod signals;
pub mod terminal;
pub mod timer;
pub mod writes;

use std::collections::BTreeMap;
use std::io::{self, BufWriter, IsTerminal, Write};

use ratatui::Terminal;
use ratatui::backend::CrosstermBackend;
use spela_core::backend::BackendKind;
use spela_core::env::var;
use spela_core::games::cache;
use spela_core::lock::InstanceLock;
use spela_core::migrate::{CONFIG_HINT, Report, SourceKind};
use spela_core::profile::{Scope, store};
use spela_ui::{AppState, Startup};

use crate::cli::Context;
use crate::error::{CliError, exit};

use self::app::App;
use self::events::{LoopEvent, channel};
use self::exec::{SteamSource, Workers};
use self::host::TerminalHost;
use self::jobs::Pool;
use self::metrics::{CpuLoad, FakeCpuLoad, MetricsThread, NoCpu, NoGpu, ProcCpu};
use self::probe::Support;
use self::signals::SignalWatcher;
use self::terminal::TerminalOptions;
use self::timer::{METRICS_EVERY, Timer};
use self::writes::WriteQueue;

/// The scopes whose Go profile did not migrate, with the CLI's text for
/// them. Neither the TUI nor its write queue edits them.
pub fn broken_profiles(report: &Report) -> BTreeMap<Scope, String> {
    let mut out = BTreeMap::new();
    for h in report.failed() {
        let scope = match h.kind {
            SourceKind::Game(app_id) => Scope::Game(app_id),
            SourceKind::AllGames => Scope::AllGames,
            _ => continue,
        };
        if let Err(e) = crate::migration::refuse_profile(report, scope) {
            let text = match &e.hint {
                Some(hint) => format!("{} ({hint})", e.message),
                None => e.message.clone(),
            };
            out.insert(scope, text);
        }
    }
    out
}

/// The error when stdin or stdout is not a terminal.
pub const NOT_A_TERMINAL: &str = "spela tui needs a terminal";

/// Runs the TUI until the user quits. `_lock` is the TUI lock, held to the
/// end.
pub fn run(ctx: &mut Context, _lock: InstanceLock, err: &mut dyn Write) -> Result<(), CliError> {
    // Before any thread, migration or log line: a TUI without a terminal
    // fails at once and touches nothing.
    if !(io::stdin().is_terminal() && io::stdout().is_terminal()) {
        return Err(CliError::new(exit::FAILURE, NOT_A_TERMINAL)
            .with_hint("run it in a terminal; every operation is also a CLI command"));
    }
    let (tx, rx) = channel::<spela_ui::Msg>();
    let signal_tx = tx.clone();
    let _signals = SignalWatcher::spawn(move |kind| {
        let _ = signal_tx.send(LoopEvent::Signal(kind));
    })
    .map_err(|e| CliError::new(exit::FAILURE, format!("cannot watch signals: {e}")))?;

    // The migration, with its notice shown inside the TUI. `--demo` never
    // reads Go YAML.
    let mut notice = Vec::new();
    if !ctx.demo {
        let report = crate::migration::run(&ctx.paths, ctx.clock.as_ref(), err, false)?;
        notice = report.notice();
        ctx.migration = report;
    }
    let loaded = spela_core::config::load(&ctx.paths.config_file())
        .map_err(|e| CliError::new(exit::CONFIG, e.to_string()).with_hint(CONFIG_HINT))?;
    let warnings: Vec<String> = loaded
        .warnings
        .iter()
        .map(|w| format!("{}: {w}", ctx.paths.config_file().display()))
        .collect();
    for w in &warnings {
        tracing::warn!("{w}");
    }
    let config = loaded.config;
    // The config's level, unless a flag or SPELA_LOG chose one.
    ctx.log.apply(config.log_level.name());
    let broken = broken_profiles(&ctx.migration);

    // The demo scans its own fictional library.
    let steam = if ctx.demo {
        let root = ctx.paths.cache_dir.join("demo-steam");
        spela_core::fake::library::write(&root).map_err(|e| {
            CliError::new(
                exit::FAILURE,
                format!("cannot set up the demo library: {e}"),
            )
        })?;
        // A fictional sysfs tree, so the hardware actions change only it.
        let (cpu, privilege) = exec::monitor::demo_hardware(&ctx.paths.cache_dir.join("demo-sys"))
            .map_err(|e| {
                CliError::new(
                    exit::FAILURE,
                    format!("cannot set up the demo hardware: {e}"),
                )
            })?;
        ctx.backends.cpu = cpu;
        ctx.backends.privilege = privilege;
        SteamSource::Demo(root.join("SteamLibrary"))
    } else {
        SteamSource::System {
            home: ctx.env.non_empty(var::HOME).map(Into::into),
        }
    };
    let games = cache::load(&ctx.paths);
    let profiles: Vec<u64> = games
        .games
        .iter()
        .map(|g| g.app_id)
        .filter(|id| store::has_profile(&ctx.paths, *id))
        .collect();

    let cache_path = (!ctx.demo).then(|| ctx.paths.caps_cache());
    let caps = probe::detect(&ctx.env, probe::DEFAULT_TIMEOUT, cache_path.as_deref());
    tracing::info!(
        probe = ?caps.probe,
        kitty = ?caps.kitty_keyboard,
        sync = ?caps.synchronized_output,
        terminal = caps.terminal_version.as_deref().unwrap_or(""),
        "terminal capabilities"
    );

    terminal::install_panic_hook();
    let guard = terminal::enter(TerminalOptions {
        mouse: config.ui.mouse,
        bracketed_paste: true,
        kitty_keyboard: caps.kitty_keyboard == Support::Yes,
        synchronized_output: caps.synchronized_output == Support::Yes,
    })
    .map_err(|e| CliError::new(exit::FAILURE, format!("cannot set up the terminal: {e}")))?;
    let backend = CrosstermBackend::new(BufWriter::new(io::stdout()));
    let term = Terminal::new(backend)
        .map_err(|e| CliError::new(exit::FAILURE, format!("cannot set up the terminal: {e}")))?;
    let size = terminal::size().unwrap_or((80, 24));

    let failure = |what: &str, e: io::Error| {
        CliError::new(exit::FAILURE, format!("cannot start the {what}: {e}"))
    };
    input::spawn(input::TerminalSource, tx.clone()).map_err(|e| failure("input thread", e))?;
    let gpu = std::mem::replace(&mut ctx.backends.gpu, Box::new(NoGpu));
    let cpu: Box<dyn CpuLoad> = match ctx.backends.kind {
        BackendKind::Fake => Box::new(FakeCpuLoad::default()),
        BackendKind::Real => Box::new(ProcCpu::new("/proc")),
    };
    // One CPU reader, shared by the metrics thread (governor and SMT on
    // Monitor) and the jobs (the profile page's governor picker).
    let cpu_reader =
        exec::profile::CpuReader::new(std::mem::replace(&mut ctx.backends.cpu, Box::new(NoCpu)));
    let metrics = MetricsThread::spawn(gpu, cpu, Box::new(cpu_reader.clone()), tx.clone())
        .map_err(|e| failure("metrics thread", e))?;
    let driver = metrics.driver();
    // The TUI's own polkit runner: no text agent and a null stdin, so
    // pkexec never prompts on this tty (decision 44).
    if ctx.backends.kind == BackendKind::Real {
        ctx.backends.privilege = Box::new(crate::privilege::PolkitPrivilege::tui());
    }
    let privilege = std::mem::replace(
        &mut ctx.backends.privilege,
        Box::new(exec::monitor::NoPrivilege),
    );
    let jobs = Pool::spawn(jobs::THREADS).map_err(|e| failure("job pool", e))?;
    let writes = WriteQueue::spawn(ctx.paths.clone(), broken.clone(), tx.clone())
        .map_err(|e| failure("write queue", e))?;
    let timer = Timer::spawn(tx.clone(), METRICS_EVERY).map_err(|e| failure("timer", e))?;
    let workers = Workers {
        paths: ctx.paths.clone(),
        tx: tx.clone(),
        metrics,
        jobs,
        writes: writes.sender(),
        clock: ctx.clock.clone(),
        steam,
        cpu: cpu_reader,
        driver,
        privilege: exec::monitor::SharedPrivilege::new(privilege),
        log: ctx.log.clone(),
    };
    drop(tx);

    let state = AppState::new(Startup {
        size,
        config,
        games,
        profiles,
        broken_profiles: broken,
        notice,
        warnings,
        no_color: ctx.env.no_color(),
        demo: ctx.demo,
        clock: ctx.clock.clone(),
    });
    let mut app = App::new(state, term, Box::new(TerminalHost), Box::new(workers));
    app.timer = Some(timer);
    app.synchronized = caps.synchronized_output == Support::Yes;
    let result = app.start().and_then(|()| app.run(&rx));
    let perf = app.state.perf.summary();
    tracing::info!(
        draws = app.counters.draws,
        turns = app.counters.turns,
        keys = perf.total,
        p99_us = perf.p99.as_micros() as u64,
        "TUI closed"
    );
    let exit_note = app.exit_note.take();
    // Restore first, then join the write queue off the UI path.
    drop(app);
    drop(guard);
    for f in writes.finish() {
        let _ = writeln!(err, "warning: not saved: {f}");
    }
    if let Some(note) = exit_note {
        tracing::info!("{note}");
    }
    result.map_err(|e| CliError::new(exit::FAILURE, format!("the terminal failed: {e}")))
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::clock::ManualClock;
    use spela_core::paths::Paths;

    #[test]
    fn a_go_profile_that_did_not_migrate_is_broken() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let go = paths.go().game_profile(9100030);
        std::fs::create_dir_all(go.parent().unwrap()).unwrap();
        std::fs::write(&go, "name: [\n").unwrap();
        let clock = ManualClock::new("2026-10-26T17:04:11Z".parse().unwrap());
        let report = crate::migration::run(&paths, &clock, &mut Vec::new(), false).unwrap();
        let broken = broken_profiles(&report);
        let text = &broken[&Scope::Game(9100030)];
        assert!(text.contains("9100030.yaml"), "{text}");
        assert!(!broken.contains_key(&Scope::AllGames));
    }
}
