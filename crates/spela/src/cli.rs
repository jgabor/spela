//! The command line.
//!
//! [`main`] is the process: it reads the arguments and the environment
//! once, routes them (see [`crate::router`]), sets up paths, logging and the
//! backends, and maps the result to an exit code. [`run`] is the seam that
//! runs one parsed command with injected paths, backends and streams.
//!
//! Every command in the architecture's Commands table is registered here,
//! and each group's arguments and code live in its own file under
//! [`crate::commands`]. `overlay` and `gui` are reserved: they say the
//! command was removed in 0.7.0 and exit 2, and a program on PATH with
//! either name is never launched for them (decision 45). Their arguments,
//! like the hidden `apply-profile` helper's, never reach clap, so no form
//! of them prints a help page or a clap usage error.
//!
//! The reserved commands take a catch-all [`Rest`]. A `Rest` would swallow
//! a global flag that comes after a positional, so [`hoist_globals`] moves `--json`, `--demo`
//! and `--log-level` to the front before clap parses.
//!
//! Exit codes are in [`crate::error::exit`]: 0, 1, 2, 3 and 5, plus the
//! game's own status in wrapper mode.

use std::ffi::OsString;
use std::io::{self, IsTerminal, Write};
use std::process::ExitCode;

use clap::{ArgAction, ArgMatches, Args, CommandFactory, FromArgMatches, Parser, Subcommand};
use clap_complete::Shell;
use std::sync::Arc;

use spela_core::backend::Backends;
use spela_core::clock::{Clock, SystemClock};
use spela_core::env::{Env, var};
use spela_core::migrate::Report;
use spela_core::paths::{Paths, PathsError};

use crate::commands::config::ConfigCommand;
use crate::commands::cpu::CpuCommand;
use crate::commands::denylist::DenylistCommand;
use crate::commands::dll::DllCommand;
use crate::commands::dlss::DlssCommand;
use crate::commands::gpu::GpuCommand;
use crate::commands::launch::LaunchArgs;
use crate::commands::profile::ProfileCommand;
use crate::commands::proton::ProtonCommand;
use crate::commands::{self, removed};
use crate::demo::DemoSession;
use crate::error::{CliError, exit};
use crate::helper::{self, HelperArgs};
use crate::logging::{self, LogMode};
use crate::output::{self, Colors};
use crate::router::{self, Route, WrapperInvocation};
use crate::{backends, locks, migration};

const ABOUT: &str = "Linux gaming optimization tool";

const LONG_ABOUT: &str = "\
Linux gaming optimization tool for NVIDIA GPUs: per-game profiles, DLSS and
Proton settings, DLL management with backups, and GPU and CPU tuning around
Steam launches.

To launch a game with its profile, set its Steam launch options to
`spela %command%`.";

const EXIT_HELP: &str = "\
Exit codes:
  0  success
  1  failure
  2  usage error
  3  config or profile file invalid
  5  another spela holds the lock
  In wrapper mode (spela %command%), the game's own exit code.

Environment:
  SPELA_LOG  log filter, such as debug or spela=trace
  NO_COLOR   turn colour off";

/// spela's arguments.
#[derive(Debug, Clone, Parser)]
#[command(
    name = "spela",
    version = spela_core::release::shown_version(),
    about = ABOUT,
    long_about = LONG_ABOUT,
    after_help = EXIT_HELP,
    disable_version_flag = true
)]
pub struct Cli {
    /// Print the version.
    #[arg(short = 'v', short_alias = 'V', long = "version", action = ArgAction::Version)]
    version: Option<bool>,

    // The global flags sort after every command's own flags, so each help
    // page lists them together at the end.
    /// Print results as JSON (schema version 1).
    #[arg(long, global = true, display_order = 900)]
    pub json: bool,

    /// Run on built-in fictional data. Your files are never written.
    #[arg(long, global = true, display_order = 901)]
    pub demo: bool,

    /// Log level. Overrides SPELA_LOG.
    #[arg(
        long,
        global = true,
        display_order = 902,
        value_name = "LEVEL",
        value_parser = ["off", "error", "warn", "info", "debug", "trace"]
    )]
    pub log_level: Option<String>,

    /// What to do. Without one, spela prints this help.
    #[command(subcommand)]
    pub command: Option<Command>,
}

/// Any arguments of a reserved command. Caught here so the command runs and
/// says it was removed, instead of failing on an unknown argument.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct Rest {
    /// Anything.
    #[arg(
        trailing_var_arg = true,
        allow_hyphen_values = true,
        hide = true,
        value_name = "ARGS"
    )]
    pub args: Vec<String>,
}

/// Subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum Command {
    /// Scan Steam libraries for games and their DLLs.
    Scan(commands::scan::ScanArgs),
    /// List the games found in Steam libraries.
    List(commands::list::ListArgs),
    /// Show one game and its DLLs.
    Show(commands::show::ShowArgs),
    /// Show what launching a game would do (dry run only).
    Launch(LaunchArgs),
    /// Manage per-game profiles and All games.
    Profile {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<ProfileCommand>,
    },
    /// Show or change spela's settings.
    Config {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<ConfigCommand>,
    },
    /// Manage DLSS, XeSS and FSR DLLs: list, update, install, restore.
    Dll {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<DllCommand>,
    },
    /// DLSS settings in a profile.
    Dlss {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<DlssCommand>,
    },
    /// GPU information, profile settings and reset.
    Gpu {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<GpuCommand>,
    },
    /// CPU information, profile settings, governor and SMT.
    Cpu {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<CpuCommand>,
    },
    /// Proton settings in a profile.
    Proton {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<ProtonCommand>,
    },
    /// The anti-cheat deny list for DLL swaps.
    Denylist {
        /// The subcommand. Without one, its help is printed.
        #[command(subcommand)]
        command: Option<DenylistCommand>,
    },
    /// Open the interactive terminal UI.
    Tui,
    /// Print a shell completion script.
    Completion {
        /// The shell.
        shell: Shell,
    },
    /// The root helper, run by polkit. Not for direct use.
    #[command(hide = true, disable_help_flag = true)]
    ApplyProfile(HelperArgs),
    /// Removed in 0.7.0.
    #[command(hide = true, disable_help_flag = true)]
    Overlay(Rest),
    /// Removed in 0.7.0.
    #[command(hide = true, disable_help_flag = true)]
    Gui(Rest),
}

/// A parsed command and the names that led to it, such as
/// `["profile", "show"]`.
#[derive(Debug, Clone)]
pub struct Invocation {
    /// The parsed arguments.
    pub cli: Cli,
    /// The subcommand names, outermost first.
    pub path: Vec<String>,
}

/// What the arguments ask for.
#[derive(Debug)]
pub enum Parsed {
    /// Run a command.
    Run(Box<Invocation>),
    /// Wrap a game.
    Wrapper(WrapperInvocation),
    /// Print this and exit: help, version or a usage error.
    Exit {
        /// The exit code: 0 for help and version, 2 for usage errors.
        code: u8,
        /// The text, already coloured or plain.
        text: String,
        /// Whether it goes to stdout. Usage errors go to stderr.
        to_stdout: bool,
    },
}

/// What a command needs from the outside world. Tests build one over temp
/// dirs and the fake backends.
pub struct Context {
    /// The environment snapshot.
    pub env: Env,
    /// spela's directories, under a temp dir for tests and `--demo`.
    pub paths: Paths,
    /// GPU, display, privilege and HTTP.
    pub backends: Backends,
    /// Colour for stdout and stderr.
    pub colors: Colors,
    /// The clock: the system clock, a manual one in tests.
    pub clock: Arc<dyn Clock>,
    /// What this run's migration from Go spela did. Empty under `--demo`,
    /// when the command reads no store, or when there was nothing to
    /// migrate.
    pub migration: Report,
    /// Whether this run is `--demo`, which never reads the user's Steam.
    pub demo: bool,
    /// Changes the log file's level, for the config's `log_level`.
    pub log: logging::LevelHandle,
}

/// Every subcommand name and alias clap knows, hidden ones and `help`
/// included. The router checks these before it looks for a game.
pub fn known_subcommands() -> Vec<String> {
    let cmd = Cli::command();
    let mut names: Vec<String> = cmd
        .get_subcommands()
        .flat_map(|c| {
            std::iter::once(c.get_name().to_owned()).chain(c.get_all_aliases().map(str::to_owned))
        })
        .collect();
    names.push("help".to_owned());
    names
}

/// Routes and parses `args`, which exclude the program name.
pub fn parse(args: &[OsString], env: &Env, colors: Colors) -> Parsed {
    let known = known_subcommands();
    let path_var = env.get(var::PATH);
    let route = router::route(
        args,
        |name| known.iter().any(|k| k == name),
        |token| router::is_launchable(token, path_var),
    );
    if let Route::Wrapper(w) = route {
        return Parsed::Wrapper(w);
    }
    if let Some(exit) = demo_launch(args, &known, path_var, colors) {
        return exit;
    }
    if let Some(inv) = raw_command(args) {
        return Parsed::Run(Box::new(inv));
    }
    let hoisted = hoist_globals(args);
    let json = hoisted
        .iter()
        .take_while(|a| *a != "--")
        .any(|a| a == "--json");
    let argv = std::iter::once(OsString::from("spela")).chain(hoisted);
    let matches = match Cli::command().try_get_matches_from(argv) {
        Ok(m) => m,
        Err(e) => return clap_exit(&e, colors, json),
    };
    match Cli::from_arg_matches(&matches) {
        Ok(cli) => Parsed::Run(Box::new(Invocation {
            cli,
            path: command_path(&matches),
        })),
        Err(e) => clap_exit(&e, colors, json),
    }
}

/// `--demo` never runs a game (architecture, "Backends and fakes"): global
/// flags followed by what would be a wrapped game is refused with exit 2,
/// instead of clap's unknown-command error.
fn demo_launch(
    args: &[OsString],
    known: &[String],
    path_var: Option<&str>,
    colors: Colors,
) -> Option<Parsed> {
    let hoisted = hoist_globals(args);
    let mut rest = hoisted.as_slice();
    let mut demo = false;
    while let Some(first) = rest.first().and_then(|a| a.to_str()) {
        match first {
            "--demo" => demo = true,
            "--json" => {}
            "--log-level" => rest = rest.get(1..).unwrap_or_default(),
            a if a.starts_with("--log-level=") => {}
            _ => break,
        }
        rest = rest.get(1..).unwrap_or_default();
    }
    if !demo || rest.is_empty() {
        return None;
    }
    let route = router::route(
        rest,
        |name| known.iter().any(|k| k == name),
        |token| router::is_launchable(token, path_var),
    );
    if !matches!(route, Route::Wrapper(_)) {
        return None;
    }
    let mut text = Vec::new();
    let e = CliError::new(exit::USAGE, "--demo never runs a game")
        .with_hint("run the game through Steam with `spela %command%`, without --demo");
    let _ = output::write_error(&mut text, &e, colors.err);
    Some(Parsed::Exit {
        code: exit::USAGE,
        text: String::from_utf8_lossy(&text).into_owned(),
        to_stdout: false,
    })
}

/// The hidden commands whose arguments clap never sees: `apply-profile`,
/// whose helper checks its own forms (decision 34), and the removed
/// `overlay` and `gui`, which print their removal error for any arguments,
/// `--help` and `help overlay` included (decision 45). `None` for anything
/// else.
fn raw_command(args: &[OsString]) -> Option<Invocation> {
    let (name, rest) = match args.first()?.to_str()? {
        "help" => (args.get(1)?.to_str()?, &args[2..]),
        first => (first, &args[1..]),
    };
    let rest: Vec<String> = rest
        .iter()
        .map(|a| a.to_string_lossy().into_owned())
        .collect();
    // An error object for `overlay --json`, as for `--json overlay`.
    let json = rest
        .iter()
        .take_while(|a| *a != "--")
        .any(|a| a == "--json");
    let command = match name {
        "apply-profile" => Command::ApplyProfile(HelperArgs { args: rest }),
        "overlay" => Command::Overlay(Rest { args: rest }),
        "gui" => Command::Gui(Rest { args: rest }),
        _ => return None,
    };
    Some(Invocation {
        cli: Cli {
            version: None,
            json,
            demo: false,
            log_level: None,
            command: Some(command),
        },
        path: vec![name.to_owned()],
    })
}

/// Moves `--json`, `--demo` and `--log-level <LEVEL>` (or
/// `--log-level=<LEVEL>`) to the front, keeping their order, so a global
/// flag after a positional still sets the flag instead of landing in a
/// placeholder [`Rest`]. Nothing after a bare `--` moves. clap still checks
/// the level, so a bad one stays a usage error.
pub fn hoist_globals(args: &[OsString]) -> Vec<OsString> {
    let mut globals = Vec::new();
    let mut others = Vec::new();
    let mut iter = args.iter();
    while let Some(arg) = iter.next() {
        match arg.to_str() {
            Some("--") => {
                others.push(arg.clone());
                others.extend(iter.cloned());
                break;
            }
            Some("--json" | "--demo") => globals.push(arg.clone()),
            Some(a) if a.starts_with("--log-level=") => globals.push(arg.clone()),
            Some("--log-level") => match iter.next() {
                Some(level) => {
                    globals.push(arg.clone());
                    globals.push(level.clone());
                }
                None => others.push(arg.clone()),
            },
            _ => others.push(arg.clone()),
        }
    }
    globals.extend(others);
    globals
}

/// What a clap error prints. A value that one of spela's own value parsers
/// refuses, such as `--sr-mode bogus`, is spela's check, so under `--json`
/// it is the JSON error object on stdout, exit 2. clap's own usage errors,
/// help and version keep clap's format.
fn clap_exit(e: &clap::Error, colors: Colors, json: bool) -> Parsed {
    if json && e.kind() == clap::error::ErrorKind::ValueValidation {
        let rendered = e.render().to_string();
        let first = rendered.lines().next().unwrap_or_default();
        let message = first.strip_prefix("error: ").unwrap_or(first);
        let mut text = Vec::new();
        let _ = output::write_json_error(&mut text, &CliError::new(exit::USAGE, message));
        return Parsed::Exit {
            code: exit::USAGE,
            text: String::from_utf8_lossy(&text).into_owned(),
            to_stdout: true,
        };
    }
    let to_stdout = !e.use_stderr();
    let color = if to_stdout { colors.out } else { colors.err };
    let rendered = e.render();
    let text = if color {
        rendered.ansi().to_string()
    } else {
        rendered.to_string()
    };
    Parsed::Exit {
        code: u8::try_from(e.exit_code()).unwrap_or(exit::USAGE),
        text,
        to_stdout,
    }
}

fn command_path(matches: &ArgMatches) -> Vec<String> {
    let mut path = Vec::new();
    let mut m = matches;
    while let Some((name, sub)) = m.subcommand() {
        path.push(name.to_owned());
        m = sub;
    }
    path
}

/// Runs spela as the process and maps the result to an exit code.
pub fn main() -> ExitCode {
    let args: Vec<OsString> = std::env::args_os().skip(1).collect();
    let env = Env::from_process();
    let ttys = (io::stdout().is_terminal(), io::stderr().is_terminal());
    let code = process(
        &args,
        env,
        ttys,
        &Paths::from_system,
        &mut io::stdout(),
        &mut io::stderr(),
    );
    ExitCode::from(code)
}

/// Everything [`main`] does, with the streams, terminal checks and paths
/// passed in. Returns the exit code.
///
/// `paths` resolves spela's directories. `main` passes
/// [`Paths::from_system`], which reads HOME and the `XDG_*` variables
/// through etcetera; a test passes one that returns [`Paths::under`] a temp
/// dir. It is called only after the root helper is dispatched, because the
/// helper never resolves an XDG path.
pub fn process(
    args: &[OsString],
    env: Env,
    (out_is_tty, err_is_tty): (bool, bool),
    paths: &dyn Fn() -> Result<Paths, PathsError>,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> u8 {
    let colors = Colors::detect(&env, out_is_tty, err_is_tty);
    match parse(args, &env, colors) {
        Parsed::Exit {
            code,
            text,
            to_stdout,
        } => {
            let stream: &mut dyn Write = if to_stdout { out } else { err };
            match stream
                .write_all(text.as_bytes())
                .and_then(|()| stream.flush())
            {
                Err(e) if e.kind() == io::ErrorKind::BrokenPipe => exit::OK,
                _ => code,
            }
        }
        Parsed::Wrapper(wrapper) => {
            match crate::wrapper::main(&wrapper, env, paths, colors.err, out, err) {
                Ok(code) => code,
                Err(e) => report(Err(e), false, out, err, colors),
            }
        }
        Parsed::Run(inv) => {
            // The root helper runs as root with root's environment: no
            // XDG paths, no log file, nothing written. It runs only for
            // `apply-profile` as the first argument, the form the polkit
            // action's argv1 annotation scopes. Every other route to it,
            // `help apply-profile` or a global flag such as `--demo` first,
            // is refused before anything runs, because the helper ignores
            // `--demo` and the fake backend.
            if let Some(Command::ApplyProfile(a)) = &inv.cli.command {
                if args.first().and_then(|a| a.to_str()) != Some("apply-profile") {
                    let _ = writeln!(err, "{}", helper::GO_FLAGS_MESSAGE);
                    return exit::USAGE;
                }
                let code = helper::run(a, out, err);
                let _ = out.flush();
                return code;
            }
            let json = inv.cli.json;
            match setup_and_run(&inv, env, paths, colors, out, err) {
                Ok(()) => report(Ok(()), json, out, err, colors),
                Err(e) => report(Err(e), json, out, err, colors),
            }
        }
    }
}

fn setup_and_run(
    inv: &Invocation,
    env: Env,
    paths: &dyn Fn() -> Result<Paths, PathsError>,
    colors: Colors,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    let system = paths().map_err(|e| CliError::new(exit::FAILURE, e.to_string()))?;
    // Held to the end of the run: dropping it removes the demo's files.
    let demo =
        if inv.cli.demo {
            Some(DemoSession::start(&system).map_err(|e| {
                CliError::new(exit::FAILURE, format!("cannot set up the demo: {e}"))
            })?)
        } else {
            None
        };
    let paths = demo.as_ref().map_or(system, |d| d.paths().clone());
    let log = logging::init(
        log_mode(inv),
        inv.cli.log_level.as_deref(),
        &env,
        &paths.log_dir(),
        colors.err,
        err,
    );
    let mut backends = backends::select(&env, inv.cli.demo)?;
    if let Some(demo) = &demo {
        crate::demo::use_demo_backends(&mut backends, &paths);
        // The TUI scans at startup; every other command starts from the
        // demo games already in its cache.
        if !matches!(inv.cli.command, None | Some(Command::Tui)) {
            demo.scan().map_err(|e| {
                CliError::new(exit::FAILURE, format!("cannot set up the demo: {e}"))
            })?;
        }
    }
    tracing::info!(
        version = spela_core::release::shown_version(),
        command = inv.path.join(" "),
        demo = inv.cli.demo,
        backend = ?backends.kind,
        "spela starting"
    );
    let clock: Arc<dyn Clock> = Arc::new(SystemClock);
    let migration = if migration::needs_store(inv.cli.command.as_ref(), inv.cli.demo) {
        let notice = log_mode(inv) == LogMode::Stderr;
        migration::run(&paths, clock.as_ref(), err, notice)?
    } else {
        Report::default()
    };
    let mut ctx = Context {
        env,
        paths,
        backends,
        colors,
        clock,
        migration,
        demo: inv.cli.demo,
        log: log.level.clone(),
    };
    let result = run(inv, &mut ctx, out, err);
    if let Err(e) = &result
        && !e.is_broken_pipe()
    {
        tracing::info!(code = e.code, "command failed: {}", e.message);
    }
    result
}

/// Where a command's logs go: the TUI logs to the file only, other commands
/// to stderr.
pub fn log_mode(inv: &Invocation) -> LogMode {
    match &inv.cli.command {
        Some(Command::Tui) => LogMode::File,
        None if inv.cli.demo => LogMode::File,
        _ => LogMode::Stderr,
    }
}

fn report(
    result: Result<(), CliError>,
    json: bool,
    out: &mut dyn Write,
    err: &mut dyn Write,
    colors: Colors,
) -> u8 {
    let result = result.and_then(|()| out.flush().map_err(CliError::from_stdout));
    match result {
        Ok(()) => exit::OK,
        Err(e) if e.is_broken_pipe() => exit::OK,
        Err(e) => {
            if json {
                let _ = output::write_json_error(out, &e);
                let _ = out.flush();
            } else {
                let _ = output::write_error(err, &e, colors.err);
            }
            e.code
        }
    }
}

/// Runs one parsed command. `out` gets results, `err` gets warnings.
pub fn run(
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    let Some(command) = &inv.cli.command else {
        if inv.cli.demo {
            return run_tui(ctx, err);
        }
        return write_help(out, &[], ctx.colors.out);
    };
    match command {
        Command::Tui => run_tui(ctx, err),
        Command::Scan(args) => commands::scan::run(args, inv, ctx, out, err),
        Command::List(args) => commands::list::run(args, inv, ctx, out, err),
        Command::Show(args) => commands::show::run(args, inv, ctx, out, err),
        Command::Launch(args) => commands::launch::run(args, inv, ctx, out, err),
        Command::Profile { command: Some(c) } => commands::profile::run(c, inv, ctx, out, err),
        Command::Config { command: Some(c) } => commands::config::run(c, inv, ctx, out, err),
        Command::Dll { command: Some(c) } => commands::dll::run(c, inv, ctx, out, err),
        Command::Dlss { command: Some(c) } => commands::dlss::run(c, inv, ctx, out, err),
        Command::Gpu { command: Some(c) } => commands::gpu::run(c, inv, ctx, out, err),
        Command::Cpu { command: Some(c) } => commands::cpu::run(c, inv, ctx, out, err),
        Command::Proton { command: Some(c) } => commands::proton::run(c, inv, ctx, out, err),
        Command::Denylist { command: Some(c) } => commands::denylist::run(c, inv, ctx, out, err),
        // A parent with no subcommand, such as `spela dll`, prints its
        // help and exits 0, as in Go.
        Command::Profile { command: None }
        | Command::Config { command: None }
        | Command::Dll { command: None }
        | Command::Dlss { command: None }
        | Command::Gpu { command: None }
        | Command::Cpu { command: None }
        | Command::Proton { command: None }
        | Command::Denylist { command: None } => write_help(out, &inv.path, ctx.colors.out),
        Command::Completion { shell } => commands::completion::run(*shell, out),
        Command::Overlay(_) => Err(removed::overlay()),
        Command::Gui(_) => Err(removed::gui()),
        // `process` runs the helper before any setup; this seam never does.
        Command::ApplyProfile(a) => Err(helper::check(a).unwrap_or_else(|| {
            CliError::new(exit::FAILURE, "apply-profile runs only as its own process")
        })),
    }
}

/// Startup order: logging (done), the TUI lock, then the TUI itself. The
/// lock is taken here so its log line keeps the `spela::cli` target.
fn run_tui(ctx: &mut Context, err: &mut dyn Write) -> Result<(), CliError> {
    let lock = locks::tui(&ctx.paths)?;
    tracing::info!(lock = %lock.path().display(), "took the TUI lock");
    commands::tui::run(ctx, lock, err)
}

fn write_help(out: &mut dyn Write, path: &[String], color: bool) -> Result<(), CliError> {
    let mut root = Cli::command();
    // Building sets each subcommand's full name, such as `spela dll`.
    root.build();
    let mut target = &root;
    for name in path {
        match target.find_subcommand(name) {
            Some(sub) => target = sub,
            None => break,
        }
    }
    let help = target.clone().render_long_help();
    let text = if color {
        help.ansi().to_string()
    } else {
        help.to_string()
    };
    out.write_all(text.as_bytes())
        .map_err(CliError::from_stdout)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse_args(args: &[&str]) -> Parsed {
        let args: Vec<OsString> = args.iter().map(OsString::from).collect();
        parse(&args, &Env::default(), Colors::default())
    }

    fn path_of(args: &[&str]) -> Vec<String> {
        match parse_args(args) {
            Parsed::Run(inv) => inv.path,
            other => panic!("expected a command, got {other:?}"),
        }
    }

    fn exit_of(args: &[&str]) -> (u8, String, bool) {
        match parse_args(args) {
            Parsed::Exit {
                code,
                text,
                to_stdout,
            } => (code, text, to_stdout),
            other => panic!("expected an exit, got {other:?}"),
        }
    }

    #[test]
    fn clap_definitions_are_valid() {
        Cli::command().debug_assert();
    }

    #[test]
    fn every_architecture_command_is_registered() {
        let expected: &[&[&str]] = &[
            &["scan"],
            &["list"],
            &["show", "Some Game"],
            &["launch", "x", "--dry-run"],
            &["profile", "list"],
            &["profile", "create", "x"],
            &["profile", "show", "--all-games"],
            &["profile", "reset", "x"],
            &["profile", "delete", "x"],
            &["profile", "follow", "dlss.sr_mode", "--games", "1,2"],
            &["config", "show"],
            &["config", "set", "check_updates", "false"],
            &["dll", "list"],
            &["dll", "check-updates"],
            &["dll", "update", "--all"],
            &["dll", "install", "x", "dlss"],
            &["dll", "cache"],
            &["dll", "restore", "x"],
            &["dlss", "show", "x"],
            &["dlss", "set", "x", "--sr-preset", "default"],
            &["dlss", "reset", "x"],
            &["gpu", "info"],
            &["gpu", "reset"],
            &["gpu", "set", "x", "--clock-offset", "-100"],
            &["gpu", "show", "x"],
            &["gpu", "profile-reset", "x"],
            &["cpu", "info"],
            &["cpu", "governor", "performance"],
            &["cpu", "smt", "off"],
            &["cpu", "set", "x"],
            &["cpu", "show", "x"],
            &["cpu", "profile-reset", "x"],
            &["proton", "set", "x"],
            &["proton", "show", "x"],
            &["proton", "reset", "x"],
            &["denylist", "show"],
            &["denylist", "check", "x"],
            &["denylist", "allow", "x"],
            &["denylist", "deny", "x", "--reason", "anti-cheat"],
            &["denylist", "clear", "x"],
            &["tui"],
            &["completion", "bash"],
            &["apply-profile", "--serve"],
            &["overlay", "set"],
            &["gui"],
        ];
        for args in expected {
            let path = path_of(args);
            let names: Vec<&str> = args
                .iter()
                .copied()
                .take_while(|a| !a.starts_with('-'))
                .take(path.len())
                .collect();
            assert_eq!(path, names, "{args:?}");
        }
    }

    #[test]
    fn version_flags_all_print_clap_s_line() {
        for flag in ["--version", "-V", "-v"] {
            let (code, text, to_stdout) = exit_of(&[flag]);
            assert_eq!(code, exit::OK);
            assert!(to_stdout);
            assert_eq!(
                text,
                format!("spela {}\n", spela_core::release::shown_version()),
                "{flag}"
            );
        }
    }

    #[test]
    fn usage_errors_go_to_stderr_with_exit_2() {
        let (code, text, to_stdout) = exit_of(&["frobnicate"]);
        assert_eq!(code, exit::USAGE);
        assert!(!to_stdout);
        assert!(text.contains("frobnicate"), "{text}");
        let (code, _, _) = exit_of(&["launch"]);
        assert_eq!(code, exit::USAGE, "launch needs a game");
        let (code, _, _) = exit_of(&["--log-level", "loud"]);
        assert_eq!(code, exit::USAGE);
    }

    #[test]
    fn global_flags_work_before_and_after_the_command() {
        for args in [
            &["--json", "--demo", "scan"][..],
            &["scan", "--json", "--demo"][..],
            &["--log-level", "debug", "dll", "list"][..],
            &["dll", "--log-level", "debug", "list"][..],
        ] {
            match parse_args(args) {
                Parsed::Run(inv) => {
                    assert!(inv.cli.json || inv.cli.log_level.is_some(), "{args:?}");
                }
                other => panic!("{args:?}: {other:?}"),
            }
        }
    }

    #[test]
    fn global_flags_after_a_positional_still_count() {
        for args in [
            &["show", "x", "--demo"][..],
            &["show", "x", "--json"][..],
            &["profile", "show", "x", "--log-level", "debug"][..],
            &[
                "dlss",
                "set",
                "x",
                "--sr-preset",
                "default",
                "--log-level=info",
            ][..],
        ] {
            match parse_args(args) {
                Parsed::Run(inv) => {
                    let cli = &inv.cli;
                    assert!(
                        cli.demo || cli.json || cli.log_level.is_some(),
                        "{args:?}: {cli:?}"
                    );
                }
                other => panic!("{args:?}: {other:?}"),
            }
        }
        let (code, _, _) = exit_of(&["show", "x", "--log-level", "loud"]);
        assert_eq!(code, exit::USAGE, "the level is still checked");
    }

    #[test]
    fn hoisting_keeps_order_and_stops_at_a_double_dash() {
        let os = |a: &[&str]| -> Vec<OsString> { a.iter().map(OsString::from).collect() };
        assert_eq!(
            hoist_globals(&os(&[
                "show",
                "x",
                "--json",
                "--log-level",
                "debug",
                "--demo"
            ])),
            os(&["--json", "--log-level", "debug", "--demo", "show", "x"])
        );
        assert_eq!(
            hoist_globals(&os(&["show", "--", "x", "--demo"])),
            os(&["show", "--", "x", "--demo"])
        );
        assert_eq!(
            hoist_globals(&os(&["show", "x", "--log-level"])),
            os(&["show", "x", "--log-level"])
        );
    }

    #[test]
    fn reserved_and_helper_arguments_never_reach_clap() {
        for args in [
            &["overlay", "--help"][..],
            &["overlay", "-h"][..],
            &["gui", "-h"][..],
            &["gui", "--help"][..],
            &["help", "overlay"][..],
            &["help", "gui", "x"][..],
            &["overlay", "--log-level", "loud"][..],
            &["--demo", "overlay", "--help"][..],
            &["apply-profile", "--help"][..],
            &["apply-profile", "--once"][..],
            &["apply-profile", "--serve", "--once", "x"][..],
        ] {
            match parse_args(args) {
                Parsed::Run(inv) => assert!(
                    matches!(
                        inv.cli.command,
                        Some(Command::Overlay(_) | Command::Gui(_) | Command::ApplyProfile(_))
                    ),
                    "{args:?}"
                ),
                other => panic!("{args:?}: {other:?}"),
            }
        }
        match parse_args(&["overlay", "--json"]) {
            Parsed::Run(inv) => assert!(inv.cli.json),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn the_helper_runs_only_as_the_first_argument() {
        // Each of these used to reach the real helper, which ignores
        // `--demo` and the fake backend. Now each is refused before any
        // setup: no paths, no root check.
        for args in [
            &["help", "apply-profile", "--serve"][..],
            &["--demo", "apply-profile", "--serve"][..],
            &["--json", "apply-profile", "--once", "{}"][..],
            &["--log-level", "debug", "apply-profile", "--serve"][..],
            &["apply-profile", "--serve", "--demo"][..],
        ] {
            let args: Vec<OsString> = args.iter().map(OsString::from).collect();
            let (mut out, mut err) = (Vec::new(), Vec::new());
            let code = process(
                &args,
                Env::default(),
                (false, false),
                &|| panic!("the helper never resolves paths"),
                &mut out,
                &mut err,
            );
            let err = String::from_utf8(err).unwrap();
            assert_eq!(code, exit::USAGE, "{args:?}: {err}");
            assert_eq!(err, format!("{}\n", helper::GO_FLAGS_MESSAGE), "{args:?}");
            assert!(out.is_empty(), "{args:?}");
        }
    }

    #[test]
    fn known_names_include_hidden_and_help() {
        let names = known_subcommands();
        for n in ["scan", "apply-profile", "overlay", "gui", "help", "tui"] {
            assert!(names.iter().any(|k| k == n), "{n}");
        }
    }

    #[test]
    fn parents_without_a_subcommand_show_help() {
        match parse_args(&["dll"]) {
            Parsed::Run(inv) => {
                assert!(matches!(
                    inv.cli.command,
                    Some(Command::Dll { command: None })
                ));
                assert_eq!(inv.path, ["dll"]);
            }
            other => panic!("{other:?}"),
        }
        let mut out = Vec::new();
        write_help(&mut out, &["dll".to_owned()], false).unwrap();
        let text = String::from_utf8(out).unwrap();
        assert!(text.contains("Usage: spela dll"), "{text}");
        assert!(text.contains("check-updates"), "{text}");
    }
}
