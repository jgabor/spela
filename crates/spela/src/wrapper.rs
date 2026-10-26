//! Wrapper mode: `spela %command%` runs a game with its profile.
//!
//! The sequence is the architecture's ("Launch and privilege", Sequence):
//!
//! 1. Refuse to run as root (decision 35). Watch SIGINT, SIGTERM, SIGHUP
//!    and SIGQUIT from here on.
//! 2. Load the config, the game cache and the profiles, migrating Go files
//!    first. Detect the game, build the plan and print it. Nothing here may
//!    stop the game: bad data warns and falls back (decision 49).
//! 3. When the plan changes VRR or hardware, try the launch lock. Another
//!    launch holding it means an env-only launch with a warning
//!    (decision 33).
//! 4. Apply KDE VRR in the user session and register its restore, even
//!    when the apply fails. Skipped when a stop signal already came.
//! 5. Register the hardware restore, then start the root helper with the
//!    batch. A dismissed prompt or a failed step only warns, and a step the
//!    helper skipped is logged as skipped with its reason. Skipped when
//!    a stop signal already came, so no prompt shows for a launch that is
//!    already stopping.
//! 6. A stop signal before this point: run the restores and exit 128 plus
//!    the signal, without starting the game.
//! 7. Print the launch message and start the game with inherited streams.
//!    Every later stop signal goes to the game.
//! 8. Wait for it, run the restores in reverse (hardware, then KDE VRR),
//!    release the lock, and exit with the game's status, or 128 plus the
//!    signal that ended it.
//!
//! The launch body runs under `catch_unwind`, so a panic still restores.
//! A restore that fails is logged and never changes the exit code.

use std::ffi::OsString;
use std::io::{self, Write};
use std::os::unix::process::ExitStatusExt;
use std::panic::{AssertUnwindSafe, catch_unwind};
use std::path::Path;
use std::process::{Child, Command, ExitStatus, Stdio};
use std::sync::{Arc, Mutex, MutexGuard};

use spela_core::backend::{Backends, PrivilegedSession, StepOutcome};
use spela_core::clock::Clock;
use spela_core::config::{self, Config};
use spela_core::env::{Env, var};
use spela_core::games::{Game, cache};
use spela_core::launch::summary::{self, Mode};
use spela_core::launch::vrr::{self, VrrRestore};
use spela_core::launch::{LaunchPlan, ProfileState, detect};
use spela_core::lock::{InstanceLock, LockError};
use spela_core::migrate::Report;
use spela_core::paths::Paths;
use spela_core::profile::{Effective, store};
use spela_core::steam::library::SteamInstall;

use crate::error::{CliError, exit};
use crate::plan::SystemProbes;
use crate::router::WrapperInvocation;

/// The warning when no game matches, as Go printed it.
pub const NOT_DETECTED: &str = "Warning: could not detect game for wrapper invocation";

/// Where stop signals go: held until the game starts, then forwarded to
/// it. The signal thread and the launch share it.
#[derive(Debug, Default)]
pub struct StopSlot {
    state: Mutex<StopState>,
}

#[derive(Debug, Default)]
struct StopState {
    /// The first stop signal that came before the game started.
    pending: Option<i32>,
    /// The running game's process id.
    child: Option<u32>,
}

impl StopSlot {
    /// A slot with nothing pending.
    pub fn new() -> StopSlot {
        StopSlot::default()
    }

    fn lock(&self) -> MutexGuard<'_, StopState> {
        self.state.lock().unwrap_or_else(|e| e.into_inner())
    }

    /// A stop signal arrived: forward it to the game when it runs, else
    /// remember it. Every signal is forwarded, not only the first.
    pub fn signal(&self, number: i32) {
        let mut state = self.lock();
        match state.child {
            Some(pid) => {
                tracing::info!(signal = number, pid, "forwarding a signal to the game");
                // SAFETY: kill has no memory effects; a stale pid is only
                // an error return.
                let _ = unsafe { libc::kill(pid as libc::pid_t, number) };
            }
            None => {
                state.pending.get_or_insert(number);
            }
        }
    }

    /// The stop signal that came before the game started, if any.
    pub fn pending(&self) -> Option<i32> {
        self.lock().pending
    }

    /// Starts the game unless a stop signal already came, holding the slot
    /// so a signal during the start waits and then reaches the game.
    fn start(&self, spawn: impl FnOnce() -> io::Result<Child>) -> Result<Child, Started> {
        let mut state = self.lock();
        if let Some(signal) = state.pending {
            return Err(Started::Stopped(signal));
        }
        let child = spawn().map_err(Started::Failed)?;
        state.child = Some(child.id());
        Ok(child)
    }

    /// The game ended: later signals are only remembered.
    fn finished(&self) {
        self.lock().child = None;
    }

    /// Waits for the game to exit. The exit is first seen without reaping
    /// the game (`WNOWAIT`), then the slot forgets its PID, and only then
    /// is the game reaped. So a signal that comes as the game exits never
    /// reaches another process that got the freed PID.
    fn wait(&self, child: &mut Child) -> io::Result<ExitStatus> {
        let pid = child.id() as libc::id_t;
        loop {
            // SAFETY: an all-zero siginfo_t is a valid value for waitid to
            // fill in.
            let mut info: libc::siginfo_t = unsafe { std::mem::zeroed() };
            // SAFETY: `info` is a valid out pointer, and WNOWAIT leaves
            // the child to be reaped below.
            let waited =
                unsafe { libc::waitid(libc::P_PID, pid, &mut info, libc::WEXITED | libc::WNOWAIT) };
            if waited == 0 || io::Error::last_os_error().kind() != io::ErrorKind::Interrupted {
                break;
            }
        }
        self.finished();
        child.wait()
    }
}

enum Started {
    Stopped(i32),
    Failed(io::Error),
}

/// Watches SIGINT, SIGTERM, SIGHUP and SIGQUIT and hands each to a
/// [`StopSlot`]. SIGQUIT (Ctrl-backslash in a terminal) is a stop like the
/// others, so it no longer ends spela without its restores.
pub struct SignalForwarder {
    handle: signal_hook::iterator::Handle,
    thread: Option<std::thread::JoinHandle<()>>,
}

impl SignalForwarder {
    /// Starts watching. From now on these signals no longer end spela.
    pub fn spawn(slot: Arc<StopSlot>) -> io::Result<SignalForwarder> {
        let mut signals = signal_hook::iterator::Signals::new([
            libc::SIGINT,
            libc::SIGTERM,
            libc::SIGHUP,
            libc::SIGQUIT,
        ])?;
        let handle = signals.handle();
        let thread = std::thread::Builder::new()
            .name("spela-signals".into())
            .spawn(move || {
                for number in signals.forever() {
                    slot.signal(number);
                }
            })?;
        Ok(SignalForwarder {
            handle,
            thread: Some(thread),
        })
    }
}

impl Drop for SignalForwarder {
    fn drop(&mut self) {
        self.handle.close();
        if let Some(thread) = self.thread.take() {
            let _ = thread.join();
        }
    }
}

/// One registered restore.
enum Restore {
    /// KDE VRR on one output.
    Vrr(VrrRestore),
    /// The waiting root helper. `None` until it started; it stays `None`
    /// when the start failed, because then nothing changed.
    Hardware(Option<Box<dyn PrivilegedSession>>),
}

impl Restore {
    fn area(&self) -> &'static str {
        match self {
            Restore::Vrr(_) => "KDE VRR",
            Restore::Hardware(_) => "hardware",
        }
    }
}

/// The restores, run once, in reverse registration order.
#[derive(Default)]
struct Cleanup {
    steps: Vec<Restore>,
}

impl Cleanup {
    fn areas(&self) -> Vec<&'static str> {
        self.steps.iter().map(Restore::area).collect()
    }

    fn run(&mut self, backends: &mut Backends) {
        while let Some(step) = self.steps.pop() {
            let area = step.area();
            let result = match step {
                Restore::Vrr(restore) => restore
                    .run(backends.display.as_mut())
                    .map_err(|e| e.to_string()),
                Restore::Hardware(None) => continue,
                Restore::Hardware(Some(session)) => restore_hardware(session),
            };
            let area = slog_value(area);
            match result {
                Ok(()) => tracing::info!(area = %area, "restore succeeded"),
                Err(error) => tracing::warn!(area = %area, error, "restore failed"),
            }
        }
    }
}

/// A value as Go's slog text handler writes it: bare, or quoted when it
/// has a space, so the lines read `area=hardware` and `area="KDE VRR"`.
fn slog_value(value: &str) -> String {
    if value.contains(char::is_whitespace) || value.is_empty() {
        format!("{value:?}")
    } else {
        value.to_owned()
    }
}

fn restore_hardware(session: Box<dyn PrivilegedSession>) -> Result<(), String> {
    let report = session.restore().map_err(|e| e.to_string())?;
    let failed: Vec<String> = report
        .steps
        .iter()
        .filter_map(|(step, result)| result.as_ref().err().map(|e| format!("{step}: {e}")))
        .collect();
    for (step, result) in &report.steps {
        if result.is_ok() {
            tracing::info!(step = %step, "hardware step restored");
        }
    }
    if failed.is_empty() {
        Ok(())
    } else {
        Err(failed.join("; "))
    }
}

/// What a launch needs from the outside world. Tests build one over temp
/// dirs and the fake backends.
pub struct Launch<'a> {
    /// The environment snapshot.
    pub env: &'a Env,
    /// spela's directories.
    pub paths: &'a Paths,
    /// Display, privilege and GPU.
    pub backends: &'a mut Backends,
    /// The clock, for the migration.
    pub clock: &'a dyn Clock,
    /// Where stop signals land.
    pub stop: &'a StopSlot,
    /// The log file's level, set to the config's once it loads.
    pub log: crate::logging::LevelHandle,
    /// The summary and the launch message.
    pub out: &'a mut dyn Write,
    /// Warnings.
    pub err: &'a mut dyn Write,
}

/// Starts the game: the production spawn. The registered restore areas are
/// for tests.
pub fn spawn_game(command: &mut Command, _registered: &[&'static str]) -> io::Result<Child> {
    command.spawn()
}

/// Runs one wrapped game and returns the exit code: the game's own, or
/// 128 plus its signal, or 1 when it could not start.
pub fn launch(
    l: &mut Launch<'_>,
    invocation: &WrapperInvocation,
    spawn: &mut dyn FnMut(&mut Command, &[&'static str]) -> io::Result<Child>,
) -> Result<u8, CliError> {
    let mut cleanup = Cleanup::default();
    let mut lock: Option<InstanceLock> = None;
    let body = catch_unwind(AssertUnwindSafe(|| {
        body(l, invocation, &mut cleanup, &mut lock, spawn)
    }));
    cleanup.run(l.backends);
    drop(lock);
    match body {
        Ok(result) => result,
        Err(_) => Err(CliError::new(
            exit::FAILURE,
            "spela stopped on an internal error; the changes it made were restored",
        )),
    }
}

fn body(
    l: &mut Launch<'_>,
    invocation: &WrapperInvocation,
    cleanup: &mut Cleanup,
    lock: &mut Option<InstanceLock>,
    spawn: &mut dyn FnMut(&mut Command, &[&'static str]) -> io::Result<Child>,
) -> Result<u8, CliError> {
    let (config, migration) = load_store(l);
    let game = find_game(l, &config, invocation);
    let mut plan = game
        .as_ref()
        .map(|game| build_plan(l, &config, &migration, game));
    match &plan {
        Some(p) => {
            let command = invocation
                .command
                .iter()
                .map(|a| a.to_string_lossy())
                .collect::<Vec<_>>()
                .join(" ");
            let text = summary::summary(p, &Mode::Wrapper(command), crate::privilege::PKEXEC);
            // A closed stdout must never stop the game or the restores.
            let _ = l.out.write_all(text.plain().as_bytes());
            if let Some(preflight) = &p.preflight {
                preflight.log(p.app_id);
            }
        }
        None => {
            let _ = writeln!(l.err, "{NOT_DETECTED}");
            tracing::info!("could not detect the game; launching with no profile");
        }
    }

    let mut skipped = false;
    if let Some(p) = &plan
        && p.changes_system()
    {
        match crate::locks::launch(l.paths) {
            Ok(held) => *lock = Some(held),
            Err(e) => {
                let why = match &e {
                    LockError::Held { .. } => e.to_string(),
                    LockError::Io { .. } => format!("cannot take the launch lock: {e}"),
                };
                warn(
                    l.err,
                    &format!("{why}; launching without VRR and hardware changes"),
                );
                skipped = true;
                plan = Some(p.env_only());
            }
        }
    }

    // A stop signal that already came means the game never starts, so
    // making changes, or showing a prompt, would be wasted.
    if let Some(p) = &plan {
        if l.stop.pending().is_none() {
            apply_vrr(l, p, cleanup);
        }
        if l.stop.pending().is_none() {
            apply_hardware(l, p, cleanup);
        }
    }

    let mut command = game_command(invocation, plan.as_ref());
    let message = summary::launch_message(plan.as_ref(), skipped);
    let registered = cleanup.areas();
    let started = l.stop.start(|| {
        let _ = writeln!(l.out, "{message}");
        let _ = l.out.flush();
        spawn(&mut command, &registered)
    });
    let mut child = match started {
        Ok(child) => child,
        Err(Started::Stopped(signal)) => {
            tracing::warn!(signal, "stopped before the game started; restoring");
            return Ok(signal_code(signal));
        }
        Err(Started::Failed(e)) => {
            return Err(CliError::new(
                exit::FAILURE,
                format!(
                    "cannot start {}: {e}",
                    invocation.command[0].to_string_lossy()
                ),
            ));
        }
    };
    let status = l
        .stop
        .wait(&mut child)
        .map_err(|e| CliError::new(exit::FAILURE, format!("cannot wait for the game: {e}")))?;
    let code = match (status.code(), status.signal()) {
        (Some(code), _) => u8::try_from(code).unwrap_or(exit::FAILURE),
        (None, Some(signal)) => signal_code(signal),
        (None, None) => exit::FAILURE,
    };
    tracing::info!(code, "the game exited");
    Ok(code)
}

/// `128 + signal`, the shell's convention.
fn signal_code(signal: i32) -> u8 {
    u8::try_from(128 + signal).unwrap_or(exit::FAILURE)
}

/// The config and the migration report. A config or migration that fails
/// warns, and the launch goes on with the default config and no profile.
fn load_store(l: &mut Launch<'_>) -> (Config, Option<Report>) {
    let migration = match crate::migration::run(l.paths, l.clock, l.err, true) {
        Ok(report) => Some(report),
        Err(e) => {
            warn(l.err, &format!("{}; launching with no profile", e.message));
            None
        }
    };
    let config = match config::load(&l.paths.config_file()) {
        Ok(loaded) => {
            for warning in &loaded.warnings {
                tracing::warn!("{}: {warning}", l.paths.config_file().display());
            }
            // A config the migration just wrote counts from here on.
            l.log.apply(loaded.config.log_level.name());
            loaded.config
        }
        Err(e) => {
            warn(
                l.err,
                &format!("{e}; launching with the default config and no profile"),
            );
            return (Config::default(), None);
        }
    };
    (config, migration)
}

/// A `Warning:` line on stderr, as Go printed its wrapper warnings, and the
/// same text in the log file. It is logged at info so the stderr layer,
/// which shows warnings, does not print it twice.
fn warn(err: &mut dyn Write, message: &str) {
    let _ = writeln!(err, "Warning: {message}");
    tracing::info!("warning: {message}");
}

/// The Steam install the config and `HOME` point at.
fn steam(env: &Env, config: &Config) -> SteamInstall {
    let home = env.non_empty(var::HOME).map(Path::new);
    SteamInstall::discover(&config.steam_path, &config.additional_library_paths, home)
}

/// The game, from the cache when it knows the game, else from the Steam
/// AppID in the environment or arguments, or Steam's own files, without a
/// scan (decision 49). A cache that lacks the game, such as one written
/// before the game was installed, must not cost it its profile.
fn find_game(l: &mut Launch<'_>, config: &Config, invocation: &WrapperInvocation) -> Option<Game> {
    let mut args: Vec<OsString> = invocation
        .env
        .iter()
        .map(|(k, v)| OsString::from(format!("{k}={v}")))
        .collect();
    args.extend(invocation.command.iter().cloned());
    let games = cache::load(l.paths);
    if games.is_empty() {
        warn(
            l.err,
            "the game cache is empty or unreadable; finding the game from Steam's files",
        );
    } else if let Some(game) = detect::from_cache(&games, l.env, &args) {
        return Some(game.clone());
    } else {
        tracing::info!("the game is not in the game cache; finding it from Steam's files");
    }
    detect::without_cache(&steam(l.env, config), l.env, &args)
}

/// The profile of the game, falling back on bad files: a broken game
/// profile falls back to All games, a broken All games to no profile.
fn profile(l: &mut Launch<'_>, migration: &Option<Report>, app_id: u64) -> ProfileState {
    let Some(report) = migration else {
        return ProfileState::none(app_id);
    };
    let all_games = if report.failed_all_games().is_some() {
        Err("All games did not migrate from Go spela".to_owned())
    } else {
        store::load_all_games(l.paths).map_err(|e| e.to_string())
    };
    let all_games = match all_games {
        Ok(all) => all,
        Err(e) => {
            warn(l.err, &format!("{e}; launching with no profile"));
            return ProfileState::none(app_id);
        }
    };
    let all_values = all_games.as_ref().map(|a| &a.profile.values);
    let game = if report.failed_profile(app_id).is_some() {
        Err("the game's profile did not migrate from Go spela".to_owned())
    } else {
        store::load_game(l.paths, app_id).map_err(|e| e.to_string())
    };
    let game = match game {
        Ok(game) => game,
        Err(e) => {
            let fallback = if all_games.is_some() {
                "launching with All games settings"
            } else {
                "launching with no profile"
            };
            warn(l.err, &format!("{e}; {fallback}"));
            None
        }
    };
    ProfileState {
        effective: Effective::of_game(app_id, game.as_ref().map(|g| &g.profile.values), all_values),
        game_file: game.is_some(),
        all_games_file: all_games.is_some(),
    }
}

fn build_plan(
    l: &mut Launch<'_>,
    config: &Config,
    migration: &Option<Report>,
    game: &Game,
) -> LaunchPlan {
    let profile = profile(l, migration, game.app_id);
    let env = l.env;
    let gpu = &mut l.backends.gpu;
    let mut install = || steam(env, config);
    let mut driver = || crate::plan::driver_version(gpu.as_mut());
    let mut probes = SystemProbes {
        install: &mut install,
        driver: &mut driver,
    };
    crate::plan::build(l.paths, config, game, &profile, &mut probes)
}

fn apply_vrr(l: &mut Launch<'_>, plan: &LaunchPlan, cleanup: &mut Cleanup) {
    let Some(policy) = plan.vrr else {
        return;
    };
    let (restore, result) = vrr::apply(l.backends.display.as_mut(), l.env, policy);
    // Kept even when the set failed: it may still have changed the display.
    if let Some(restore) = restore {
        cleanup.steps.push(Restore::Vrr(restore));
    }
    match result {
        Ok(()) => tracing::info!(policy = %policy, "KDE VRR policy applied"),
        // Go's slog warning, on stderr and in the file.
        Err(e) => tracing::warn!(error = %e, "failed to apply KDE VRR policy"),
    }
}

fn apply_hardware(l: &mut Launch<'_>, plan: &LaunchPlan, cleanup: &mut Cleanup) {
    if plan.hardware.is_empty() {
        return;
    }
    // Registered before the helper starts (architecture, "Restore
    // registered before the game starts").
    cleanup.steps.push(Restore::Hardware(None));
    match l.backends.privilege.start(&plan.hardware) {
        Ok(session) => {
            let report = session.report();
            for step in &report.steps {
                match &step.outcome {
                    StepOutcome::Applied { prior } => {
                        tracing::info!(step = %step.step, target = %step.target, prior = %prior, "hardware step applied");
                    }
                    StepOutcome::AppliedNoRestore => {
                        tracing::info!(step = %step.step, "hardware step applied");
                    }
                    StepOutcome::Failed(error) => {
                        tracing::warn!(step = %step.step, error, "hardware step failed");
                    }
                    StepOutcome::Skipped(reason) => {
                        tracing::warn!(step = %step.step, reason, "hardware step skipped");
                    }
                }
            }
            // A skipped step, such as a value the machine does not offer,
            // is reported above as skipped with its reason, not as a
            // failure: the other steps applied and restore after the game.
            let failed = failed_steps(&report.steps);
            if !failed.is_empty() {
                tracing::warn!(error = %failed, "failed to apply hardware settings");
            }
            if let Some(Restore::Hardware(slot)) = cleanup.steps.last_mut() {
                *slot = Some(session);
            }
        }
        Err(e) => tracing::warn!(error = %e, "failed to apply hardware settings"),
    }
}

/// The steps that failed, as `step: error; step: error`. Skipped steps are
/// not failures.
fn failed_steps(steps: &[spela_core::backend::StepReport]) -> String {
    steps
        .iter()
        .filter_map(|s| match &s.outcome {
            StepOutcome::Failed(error) => Some(format!("{}: {error}", s.step)),
            _ => None,
        })
        .collect::<Vec<_>>()
        .join("; ")
}

/// The game's command: inherited streams, the plan's variables, then the
/// wrapper's own `KEY=VALUE` tokens, which beat every spela value.
fn game_command(invocation: &WrapperInvocation, plan: Option<&LaunchPlan>) -> Command {
    let mut command = Command::new(&invocation.command[0]);
    command
        .args(&invocation.command[1..])
        .stdin(Stdio::inherit())
        .stdout(Stdio::inherit())
        .stderr(Stdio::inherit());
    if let Some(p) = plan {
        command.envs(p.env.iter());
    }
    command.envs(invocation.env.iter().map(|(k, v)| (k.as_str(), v.as_str())));
    command
}

/// Wrapper mode refuses to run as root (decision 35): a game run as root
/// writes root-owned files into the user's home, and the polkit policy
/// assumes the wrapper is the user. It runs before anything else, so
/// nothing is written.
pub fn refuse_root(euid: u32) -> Result<(), CliError> {
    if euid != 0 {
        return Ok(());
    }
    Err(
        CliError::new(exit::FAILURE, "spela does not run games as root")
            .with_hint("set the game's Steam launch options to `spela %command%` as your own user")
            .with_kind("root"),
    )
}

/// Wrapper mode as the process: refuses root, watches signals, sets up
/// paths, logging and the backends, and runs the game.
pub fn main(
    invocation: &WrapperInvocation,
    env: Env,
    paths: &dyn Fn() -> Result<Paths, spela_core::paths::PathsError>,
    stderr_color: bool,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<u8, CliError> {
    // SAFETY: geteuid has no preconditions and cannot fail.
    refuse_root(unsafe { libc::geteuid() })?;
    let stop = Arc::new(StopSlot::new());
    let _signals = SignalForwarder::spawn(Arc::clone(&stop))
        .map_err(|e| CliError::new(exit::FAILURE, format!("cannot watch signals: {e}")))?;
    let paths = paths().map_err(|e| CliError::new(exit::FAILURE, e.to_string()))?;
    let log = crate::logging::init(
        crate::logging::LogMode::FileAndStderr,
        None,
        crate::logging::config_level(&paths.config_file()),
        &env,
        &paths.log_dir(),
        stderr_color,
        err,
    );
    let mut backends = crate::backends::select(&env, false)?;
    tracing::info!(
        version = spela_core::release::shown_version(),
        command = ?invocation.command,
        backend = ?backends.kind,
        "spela wrapping a game"
    );
    let clock = spela_core::clock::SystemClock;
    let mut l = Launch {
        env: &env,
        paths: &paths,
        backends: &mut backends,
        clock: &clock,
        stop: &stop,
        log: log.level.clone(),
        out,
        err,
    };
    launch(&mut l, invocation, &mut spawn_game)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Writes an executable script through `/bin/cp`, so this process never
    /// holds a write descriptor to it that a parallel test's fork could
    /// inherit, which would make running it fail with "text file busy".
    fn write_script(path: &std::path::Path, text: impl AsRef<[u8]>) {
        let source = path.with_file_name(format!(
            "{}.text",
            path.file_name().unwrap().to_string_lossy()
        ));
        std::fs::write(&source, text).unwrap();
        let copied = std::process::Command::new("/bin/cp")
            .arg(&source)
            .arg(path)
            .status()
            .unwrap();
        assert!(copied.success());
        std::fs::remove_file(&source).unwrap();
        std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o755)).unwrap();
    }
    use std::os::unix::fs::PermissionsExt;

    use spela_core::backend::VrrPolicy;
    use spela_core::fake::{self, FakeDisplay, FakeGpuControl, FakePrivilege, PrivilegeFault};
    use spela_core::games::GameCache;
    use spela_core::profile::{FieldKey, OwnValues, Scope, Value};

    struct World {
        _dir: tempfile::TempDir,
        root: std::path::PathBuf,
        paths: Paths,
        env: Env,
        display: FakeDisplay,
        privilege: FakePrivilege,
        game: std::path::PathBuf,
    }

    const APP_ID: u64 = 9100010;
    const PRIMARY: &str = "6a1c4e1e-0000-4000-8000-000000000001";

    /// A game cache with one game whose install folder holds `run.sh`, a
    /// script that records its environment and exits with `$EXIT`, and an
    /// All games profile with VRR and a governor.
    fn world() -> World {
        let dir = tempfile::tempdir().unwrap();
        let root = dir.path().to_owned();
        let paths = Paths::under(&root.join("xdg"));
        let install = root.join("library/steamapps/common/Glimmer Run");
        std::fs::create_dir_all(&install).unwrap();
        let game = install.join("run.sh");
        write_script(
            &game,
            format!(
                "#!/bin/sh\n/usr/bin/env > '{}'\nexit ${{EXIT:-0}}\n",
                root.join("game-env").display()
            ),
        );
        cache::update(&paths, |c: &mut GameCache| {
            c.upsert(Game {
                app_id: APP_ID,
                name: "Glimmer Run".into(),
                install_dir: install.clone(),
                library_path: root.join("library"),
                prefix_path: None,
                scanned_at: jiff::Timestamp::UNIX_EPOCH,
                dlls: Vec::new(),
            });
        })
        .unwrap();
        let mut all = OwnValues::new();
        all.set(FieldKey::GpuVrr, Value::Text("always".into()))
            .unwrap();
        all.set(FieldKey::GpuPowerLimit, Value::Int(250)).unwrap();
        all.set(FieldKey::ProtonEnableHdr, Value::Bool(true))
            .unwrap();
        save(&paths, Scope::AllGames, &all);
        World {
            _dir: dir,
            root,
            paths,
            env: Env::from_pairs([
                (var::XDG_SESSION_TYPE, "wayland"),
                (var::XDG_CURRENT_DESKTOP, "KDE"),
            ]),
            display: FakeDisplay::demo(),
            privilege: FakePrivilege::new(FakeGpuControl::new(), None),
            game,
        }
    }

    fn save(paths: &Paths, scope: Scope, values: &OwnValues) {
        let pairs: Vec<(FieldKey, Value)> = values.iter().map(|(k, v)| (k, v.clone())).collect();
        store::edit(paths, scope, "Glimmer Run", &pairs, &[]).unwrap();
    }

    struct Ran {
        code: Result<u8, CliError>,
        out: String,
        err: String,
        registered: Option<Vec<&'static str>>,
    }

    fn run(w: &World, extra_env: &[(&str, &str)], stop: &StopSlot) -> Ran {
        let mut backends = fake::backends();
        backends.display = Box::new(w.display.clone());
        backends.privilege = Box::new(w.privilege.clone());
        let mut out = Vec::new();
        let mut err = Vec::new();
        let mut registered = None;
        let invocation = WrapperInvocation {
            env: extra_env
                .iter()
                .map(|(k, v)| ((*k).to_owned(), (*v).to_owned()))
                .collect(),
            command: vec![w.game.clone().into_os_string()],
        };
        let code = {
            let mut l = Launch {
                env: &w.env,
                paths: &w.paths,
                backends: &mut backends,
                clock: &spela_core::clock::SystemClock,
                stop,
                log: crate::logging::LevelHandle::default(),
                out: &mut out,
                err: &mut err,
            };
            launch(&mut l, &invocation, &mut |cmd, areas| {
                registered = Some(areas.to_vec());
                cmd.spawn()
            })
        };
        Ran {
            code,
            out: String::from_utf8(out).unwrap(),
            err: String::from_utf8(err).unwrap(),
            registered,
        }
    }

    fn game_env(w: &World) -> String {
        std::fs::read_to_string(w.root.join("game-env")).unwrap_or_default()
    }

    #[test]
    fn applies_registers_restores_before_spawn_and_restores_after() {
        let w = world();
        let ran = run(
            &w,
            &[("EXIT", "7"), ("PROTON_ENABLE_HDR", "0")],
            &StopSlot::new(),
        );
        assert_eq!(ran.code.unwrap(), 7, "the game's own status: {}", ran.err);
        assert_eq!(ran.registered.unwrap(), ["KDE VRR", "hardware"]);
        assert_eq!(
            w.display.sets(),
            [
                (PRIMARY.to_owned(), VrrPolicy::Always),
                (PRIMARY.to_owned(), VrrPolicy::Automatic)
            ]
        );
        let log = w.privilege.log();
        assert_eq!((log.prompts, log.restores, log.restores_on_drop), (1, 1, 0));
        assert!(
            ran.out.contains("Game: Glimmer Run (AppID 9100010)"),
            "{}",
            ran.out
        );
        assert!(ran.out.contains("Command: "), "{}", ran.out);
        assert!(
            ran.out
                .ends_with("Launching Glimmer Run with All games settings...\n"),
            "{}",
            ran.out
        );
        // The wrapper's own KEY=VALUE beats the profile.
        assert!(
            game_env(&w).contains("PROTON_ENABLE_HDR=0\n"),
            "{}",
            game_env(&w)
        );
        assert!(
            !crate::locks::launch(&w.paths).is_err(),
            "the lock is free again"
        );
    }

    #[test]
    fn a_failed_or_dismissed_apply_still_launches_and_restores_what_applied() {
        let mut w = world();
        let mut own = OwnValues::new();
        own.set(FieldKey::GpuFanSpeed, Value::Int(70)).unwrap();
        save(&w.paths, Scope::Game(APP_ID), &own);
        w.privilege.set_fault(Some(PrivilegeFault::FailAtStep(1)));
        let ran = run(&w, &[], &StopSlot::new());
        assert_eq!(ran.code.unwrap(), 0, "a failed step still launches");
        assert_eq!(
            w.privilege.log().restores,
            1,
            "the applied power limit is restored"
        );
        assert!(ran.out.contains("Launching Glimmer Run with profile..."));

        let mut w = world();
        w.privilege.set_fault(Some(PrivilegeFault::Dismissed));
        let ran = run(&w, &[], &StopSlot::new());
        assert_eq!(ran.code.unwrap(), 0, "a dismissed prompt still launches");
        assert_eq!(w.privilege.log().prompts, 1);
        assert_eq!(w.privilege.log().restores, 0);
        assert_eq!(w.display.sets().len(), 2, "VRR is still restored");
    }

    #[test]
    fn only_failed_steps_count_as_a_failure() {
        use spela_core::backend::{SettingValue, Step, StepReport};
        let report = |step, outcome| StepReport {
            step,
            target: SettingValue::Watts(250),
            outcome,
        };
        let skipped = [
            report(Step::PowerLimit, StepOutcome::Skipped("not offered".into())),
            report(Step::Fan, StepOutcome::AppliedNoRestore),
        ];
        assert_eq!(failed_steps(&skipped), "", "a skipped step is no failure");
        let mixed = [
            report(Step::PowerLimit, StepOutcome::Failed("refused".into())),
            report(Step::Smt, StepOutcome::Skipped("not supported".into())),
            report(Step::Fan, StepOutcome::Failed("busy".into())),
        ];
        assert_eq!(
            failed_steps(&mixed),
            format!("{}: refused; {}: busy", Step::PowerLimit, Step::Fan)
        );
    }

    #[test]
    fn a_skipped_step_still_launches_and_restores_the_applied_ones() {
        // The power limit cannot be read, so the helper skips that step;
        // the fan has a safe prior and applies.
        let mut w = world();
        let control = FakeGpuControl::new();
        control.set_prior_unreadable(true);
        w.privilege = FakePrivilege::new(control, None);
        let mut own = OwnValues::new();
        own.set(FieldKey::GpuFanSpeed, Value::Int(70)).unwrap();
        save(&w.paths, Scope::Game(APP_ID), &own);
        let ran = run(&w, &[], &StopSlot::new());
        assert_eq!(ran.code.unwrap(), 0, "{}", ran.err);
        assert_eq!(w.privilege.log().restores, 1, "the applied steps restore");
    }

    #[test]
    fn a_signal_before_the_game_starts_restores_and_exits_128_plus_n() {
        let w = world();
        let stop = StopSlot::new();
        stop.signal(libc::SIGTERM);
        stop.signal(libc::SIGINT);
        let ran = run(&w, &[], &stop);
        assert_eq!(ran.code.unwrap(), 143, "the first signal counts");
        assert!(game_env(&w).is_empty(), "the game never started");
        assert_eq!(
            (w.privilege.log().prompts, w.privilege.log().restores),
            (0, 0),
            "no prompt for a launch that is already stopping"
        );
        assert!(w.display.sets().is_empty(), "VRR was never changed");
        assert!(!ran.out.contains("Launching"), "{}", ran.out);
        assert!(
            crate::locks::launch(&w.paths).is_ok(),
            "the lock is released"
        );
    }

    #[test]
    fn a_held_lock_launches_with_env_only_settings() {
        let w = world();
        let held = crate::locks::launch(&w.paths).unwrap();
        let ran = run(&w, &[], &StopSlot::new());
        drop(held);
        assert_eq!(ran.code.unwrap(), 0);
        assert!(
            ran.err.contains("spela is already running (PID: "),
            "{}",
            ran.err
        );
        assert!(
            ran.err
                .contains("launching without VRR and hardware changes")
        );
        assert!(w.display.sets().is_empty());
        assert_eq!(w.privilege.log().prompts, 0);
        assert!(ran.out.contains("skipped: another launch holds the lock"));
        assert!(
            game_env(&w).contains("PROTON_ENABLE_HDR=1\n"),
            "env still applies"
        );
    }

    #[test]
    fn bad_data_never_blocks_a_launch() {
        // An unknown game: no profile, a warning, the game still runs.
        let w = world();
        let mut backends = fake::backends();
        let mut out = Vec::new();
        let mut err = Vec::new();
        let script = w.root.join("other.sh");
        write_script(&script, "#!/bin/sh\nexit 3\n");
        let invocation = WrapperInvocation {
            env: vec![],
            command: vec![script.into_os_string()],
        };
        let stop = StopSlot::new();
        let mut l = Launch {
            env: &Env::default(),
            paths: &w.paths,
            backends: &mut backends,
            clock: &spela_core::clock::SystemClock,
            stop: &stop,
            log: crate::logging::LevelHandle::default(),
            out: &mut out,
            err: &mut err,
        };
        let code = launch(&mut l, &invocation, &mut spawn_game).unwrap();
        assert_eq!(code, 3);
        assert!(String::from_utf8(err).unwrap().contains(NOT_DETECTED));
        assert_eq!(String::from_utf8(out).unwrap(), "Launching command...\n");

        // A corrupt game cache and a broken game profile: detected from
        // SteamAppId, and All games applies.
        let w = world();
        std::fs::write(w.paths.game_cache(), "{ not json").unwrap();
        std::fs::write(w.paths.game_profile(APP_ID), "[gpu\nbroken").unwrap();
        let mut env = w.env.clone();
        env = env.with(var::STEAM_APP_ID, "9100010");
        let w = World { env, ..w };
        let ran = run(&w, &[], &StopSlot::new());
        assert_eq!(ran.code.unwrap(), 0, "{}", ran.err);
        assert!(
            ran.err.contains("the game cache is empty or unreadable"),
            "{}",
            ran.err
        );
        assert!(
            ran.err.contains("launching with All games settings"),
            "{}",
            ran.err
        );
        assert!(
            ran.out.contains("Game: 9100010 (AppID 9100010)"),
            "{}",
            ran.out
        );

        // A broken game profile and no All games file: the warning says
        // what the launch really uses.
        let w = world();
        std::fs::remove_file(w.paths.all_games_profile()).unwrap();
        std::fs::write(w.paths.game_profile(APP_ID), "[gpu\nbroken").unwrap();
        let ran = run(&w, &[], &StopSlot::new());
        assert_eq!(ran.code.unwrap(), 0, "{}", ran.err);
        assert!(
            ran.err.contains("; launching with no profile"),
            "{}",
            ran.err
        );
        assert!(!ran.err.contains("All games settings"), "{}", ran.err);
        assert!(ran.out.contains("(no profile)"), "{}", ran.out);
    }

    #[test]
    fn a_cache_that_lacks_the_game_still_loads_its_profile_by_app_id() {
        // The cache knows only Glimmer Run. Steam starts another game, newer
        // than the last scan, whose own profile sets HDR off.
        const OTHER: u64 = 9100020;
        let mut w = world();
        let script = w.root.join("other.sh");
        write_script(
            &script,
            format!(
                "#!/bin/sh\n/usr/bin/env > '{}'\n",
                w.root.join("game-env").display()
            ),
        );
        w.game = script;
        let mut own = OwnValues::new();
        own.set(FieldKey::ProtonEnableHdr, Value::Bool(false))
            .unwrap();
        save(&w.paths, Scope::Game(OTHER), &own);
        let base = w.env.clone();
        for (name, value) in [
            (var::STEAM_APP_ID, "9100020"),
            (var::STEAM_GAME_ID, "9100020"),
        ] {
            w.env = base.clone().with(name, value);
            let ran = run(&w, &[], &StopSlot::new());
            assert_eq!(ran.code.unwrap(), 0, "{}", ran.err);
            assert!(!ran.err.contains(NOT_DETECTED), "{name}: {}", ran.err);
            assert!(
                !ran.err.contains("the game cache is empty"),
                "the cache is not empty: {}",
                ran.err
            );
            assert!(
                ran.out.contains("Game: 9100020 (AppID 9100020)"),
                "{name}: {}",
                ran.out
            );
            assert!(
                ran.out.ends_with("Launching 9100020 with profile...\n"),
                "{name}: {}",
                ran.out
            );
            // All games turns HDR on; the game's own profile turns it off.
            assert!(
                !game_env(&w).contains("PROTON_ENABLE_HDR=1\n"),
                "{name}: the game's own value applies"
            );
        }

        // The reaper's AppId argument works the same way.
        w.env = base;
        let ran = run(&w, &[("AppId", "9100020")], &StopSlot::new());
        assert!(
            ran.out.contains("Game: 9100020 (AppID 9100020)"),
            "{}",
            ran.out
        );
    }

    #[test]
    fn a_start_failure_restores_and_fails() {
        let mut w = world();
        w.game = w.root.join("missing.sh");
        let env = w.env.clone().with(var::STEAM_APP_ID, "9100010");
        let w = World { env, ..w };
        let ran = run(&w, &[], &StopSlot::new());
        let e = ran.code.unwrap_err();
        assert_eq!(e.code, exit::FAILURE);
        assert!(e.message.starts_with("cannot start "), "{}", e.message);
        assert_eq!(w.privilege.log().restores, 1);
        assert_eq!(w.display.sets().len(), 2);
    }

    #[test]
    fn root_is_refused() {
        let e = refuse_root(0).unwrap_err();
        assert_eq!((e.code, e.kind), (exit::FAILURE, "root"));
        assert!(e.hint.unwrap().contains("as your own user"));
        assert!(refuse_root(1000).is_ok());
    }

    #[test]
    fn a_panic_still_restores() {
        let w = world();
        let mut backends = fake::backends();
        backends.display = Box::new(w.display.clone());
        backends.privilege = Box::new(w.privilege.clone());
        let (mut out, mut err) = (Vec::new(), Vec::new());
        let invocation = WrapperInvocation {
            env: vec![],
            command: vec![w.game.clone().into_os_string()],
        };
        let stop = StopSlot::new();
        let mut l = Launch {
            env: &w.env,
            paths: &w.paths,
            backends: &mut backends,
            clock: &spela_core::clock::SystemClock,
            stop: &stop,
            log: crate::logging::LevelHandle::default(),
            out: &mut out,
            err: &mut err,
        };
        let e = launch(&mut l, &invocation, &mut |_, _| {
            panic!("a bug before the game")
        })
        .unwrap_err();
        assert_eq!(e.code, exit::FAILURE);
        assert_eq!(w.privilege.log().restores, 1);
        assert_eq!(w.display.sets().len(), 2, "VRR is restored too");
        assert!(
            crate::locks::launch(&w.paths).is_ok(),
            "the lock is released"
        );
    }

    #[test]
    fn signals_reach_the_running_game() {
        let slot = StopSlot::new();
        let child = slot
            .start(|| {
                Command::new("/bin/sh")
                    .args(["-c", "exec /bin/sleep 30"])
                    .spawn()
            })
            .ok()
            .unwrap();
        let mut child = child;
        slot.signal(libc::SIGHUP);
        let status = slot.wait(&mut child).unwrap();
        assert_eq!(status.signal(), Some(libc::SIGHUP));
        assert_eq!(slot.pending(), None, "a forwarded signal is not pending");
        slot.signal(libc::SIGTERM);
        assert_eq!(slot.pending(), Some(libc::SIGTERM));
        assert_eq!(signal_code(libc::SIGHUP), 129);
    }
}
