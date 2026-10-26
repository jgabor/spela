//! The hidden `apply-profile` command: the root helper under polkit.
//!
//! It runs as root with root's environment, so it does no per-user setup:
//! no XDG paths, no migration, no log file, and it reads no environment
//! variable. `cli::process` dispatches it before any of those start. Its two
//! modes (decision 30) are `--serve` for a launch, which reads one batch on
//! stdin, applies it, waits and restores, and `--once <request>` for the
//! live commands. Go's flag form is refused with a clear error and exit 2
//! (decision 34). The protocol, the checks and the loops are
//! [`spela_core::helper`]'s; this module adds root, stdin, the signals,
//! NVML and `/sys`.

use std::io::{self, Write};
use std::sync::mpsc::{self, Sender};

use clap::Args;
use signal_hook::consts::{SIGHUP, SIGINT, SIGQUIT, SIGTERM};
use signal_hook::iterator::Signals;

use spela_core::backend::{GpuControl, SystemRoots};
use spela_core::helper::machine::SystemMachine;
use spela_core::helper::serve::{self, Event};

use crate::error::{CliError, exit};
use crate::nvml::NvmlControl;

/// What the helper prints for any argument form it does not speak.
pub const GO_FLAGS_MESSAGE: &str = concat!(
    "apply-profile: this is the spela ",
    env!("CARGO_PKG_VERSION_MAJOR"),
    ".",
    env!("CARGO_PKG_VERSION_MINOR"),
    " helper; the Go helper's flags are not supported"
);

/// `apply-profile` arguments, kept as raw strings. clap never validates
/// them and the command has no help flag, so every form the helper does
/// not speak, Go's `--reset` and flag forms, `--help` and a bare
/// `--once` included, gets [`GO_FLAGS_MESSAGE`] and exit 2.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct HelperArgs {
    /// Everything after `apply-profile`.
    #[arg(
        trailing_var_arg = true,
        allow_hyphen_values = true,
        hide = true,
        value_name = "ARGS"
    )]
    pub args: Vec<String>,
}

/// The two forms the helper speaks (decision 30).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Mode {
    /// `--serve`: apply the batch read from stdin, then wait to restore it.
    Serve,
    /// `--once <request>`: apply this one JSON request and exit without
    /// restoring.
    Once(String),
}

impl HelperArgs {
    /// The mode these arguments ask for, or `None` for any other form.
    pub fn mode(&self) -> Option<Mode> {
        match self.args.as_slice() {
            [serve] if serve == "--serve" => Some(Mode::Serve),
            [once, request] if once == "--once" && !request.starts_with('-') => {
                Some(Mode::Once(request.clone()))
            }
            [once] => once
                .strip_prefix("--once=")
                .filter(|r| !r.is_empty())
                .map(|r| Mode::Once(r.to_owned())),
            _ => None,
        }
    }
}

/// What the helper prints when it does not run as root.
pub const NOT_ROOT_MESSAGE: &str = "apply-profile must be run with root privileges";

/// The usage error for `args`, or `None` when they name one of the two
/// modes.
pub fn check(args: &HelperArgs) -> Option<CliError> {
    match args.mode() {
        None => Some(CliError::new(exit::USAGE, GO_FLAGS_MESSAGE).with_kind("go_helper_flags")),
        Some(_) => None,
    }
}

/// Runs the helper and returns its exit code. Its protocol lines go to
/// `out`; its own errors go to `err` as they are, without the CLI's
/// `error:` prefix.
///
/// The order is fixed: the argument form first (exit 2), then root (exit
/// 1), and only then the terminal, stdin, the signals, NVML and `/sys`. So
/// a run that is not root touches nothing. NVML opens only when a GPU step
/// comes.
pub fn run(args: &HelperArgs, out: &mut dyn Write, err: &mut dyn Write) -> u8 {
    let Some(mode) = args.mode() else {
        let _ = writeln!(err, "{GO_FLAGS_MESSAGE}");
        return exit::USAGE;
    };
    if !is_root() {
        let _ = writeln!(err, "{NOT_ROOT_MESSAGE}");
        return exit::FAILURE;
    }
    leave_the_terminal();
    let mut machine = system_machine();
    match mode {
        Mode::Once(request) => {
            // A live change finishes its step: Ctrl-C mid-way through the
            // per-CPU governor writes must not leave CPUs on mixed
            // governors.
            ignore(&[SIGINT, SIGHUP, SIGQUIT]);
            serve::once(&request, out, &mut machine)
        }
        Mode::Serve => {
            let (tx, rx) = mpsc::channel();
            if let Err(e) = watch_signals(tx.clone()) {
                let _ = writeln!(err, "apply-profile: cannot watch signals: {e}");
                return exit::FAILURE;
            }
            if let Err(e) = serve::spawn_reader(io::BufReader::new(io::stdin()), tx) {
                let _ = writeln!(err, "apply-profile: cannot read stdin: {e}");
                return exit::FAILURE;
            }
            serve::serve(&rx, out, &mut machine)
        }
    }
}

/// The real machine: NVML GPU 0, opened on the first GPU step, and the CPU
/// files under `/sys`.
fn system_machine() -> SystemMachine {
    SystemMachine::new(
        Box::new(|| NvmlControl::open().map(|c| Box::new(c) as Box<dyn GpuControl>)),
        Some(SystemRoots::system().sys),
    )
}

/// Whether the process runs as root.
fn is_root() -> bool {
    // SAFETY: geteuid has no preconditions and cannot fail.
    unsafe { libc::geteuid() == 0 }
}

/// Moves the helper out of the caller's terminal session, so the signals a
/// terminal sends its foreground group (Ctrl-C, Ctrl-backslash, Ctrl-Z and a
/// hangup) no longer reach a root process that holds applied changes.
/// pkexec has finished its prompt by now, and the helper reads only its
/// pipes, so it needs no terminal. The job-control stops are ignored as
/// well, for the case where `setsid` fails because the helper already leads
/// its process group.
fn leave_the_terminal() {
    // SAFETY: setsid has no preconditions. Its only failure, EPERM for a
    // process group leader, leaves the process as it was.
    unsafe {
        libc::setsid();
    }
    ignore(&[libc::SIGTSTP, libc::SIGTTIN, libc::SIGTTOU]);
}

/// Sets `signals` to be ignored.
fn ignore(signals: &[i32]) {
    for signal in signals {
        // SAFETY: SIG_IGN installs no handler code, and every signal here
        // can be ignored.
        unsafe {
            libc::signal(*signal, libc::SIG_IGN);
        }
    }
}

/// The signals a waiting helper outlives: SIGINT, SIGHUP and SIGQUIT are
/// caught and ignored, so Ctrl-C or Ctrl-backslash in a terminal or a closed
/// session does not stop it before the restore; SIGTERM makes it restore
/// and exit, as the end of its input does (architecture.md, Protocol).
/// SIGKILL cannot be caught.
pub fn watch_signals(tx: Sender<Event>) -> io::Result<()> {
    watch(&[SIGTERM, SIGINT, SIGHUP, SIGQUIT], &[SIGTERM], tx)
}

/// Catches `signals` on a thread; the ones in `stop` send [`Event::Stop`],
/// the rest are dropped.
fn watch(signals: &[i32], stop: &[i32], tx: Sender<Event>) -> io::Result<()> {
    let mut caught = Signals::new(signals)?;
    let stop = stop.to_vec();
    std::thread::Builder::new()
        .name("spela-helper-signals".into())
        .spawn(move || {
            for signal in caught.forever() {
                if stop.contains(&signal) && tx.send(Event::Stop).is_err() {
                    return;
                }
            }
        })?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn args(raw: &[&str]) -> HelperArgs {
        HelperArgs {
            args: raw.iter().map(|s| (*s).to_owned()).collect(),
        }
    }

    fn run_text(a: &HelperArgs) -> (u8, String) {
        let mut out = Vec::new();
        let mut err = Vec::new();
        let code = run(a, &mut out, &mut err);
        assert!(out.is_empty(), "no protocol line before the checks pass");
        (code, String::from_utf8(err).unwrap())
    }

    #[test]
    fn every_other_form_is_refused_with_exit_2() {
        for raw in [
            &["--reset"][..],
            &["--clock-offset", "100"][..],
            &[][..],
            &["--serve", "--governor=performance"][..],
            &["--serve", "--once", "{}"][..],
            &["--once"][..],
            &["--once="][..],
            &["--once", "--serve"][..],
            &["--help"][..],
            &["-h"][..],
            &["--serve", "--reset"][..],
        ] {
            let (code, text) = run_text(&args(raw));
            assert_eq!(code, exit::USAGE, "{raw:?}");
            assert_eq!(
                text,
                "apply-profile: this is the spela 0.7 helper; the Go helper's flags are not supported\n",
                "{raw:?}"
            );
        }
    }

    #[test]
    fn the_two_modes_parse_and_refuse_to_run_without_root() {
        assert_eq!(args(&["--serve"]).mode(), Some(Mode::Serve));
        assert_eq!(
            args(&["--once", "{}"]).mode(),
            Some(Mode::Once("{}".into()))
        );
        assert_eq!(args(&["--once={}"]).mode(), Some(Mode::Once("{}".into())));
        assert!(check(&args(&["--serve"])).is_none());
        assert_eq!(check(&args(&["--reset"])).unwrap().code, exit::USAGE);
        // Tests never run as root, so both modes stop at the root check and
        // touch nothing. As root this would reach the hardware, so it is
        // skipped there.
        if is_root() {
            return;
        }
        for raw in [&["--serve"][..], &["--once", "{}"][..]] {
            assert_eq!(
                run_text(&args(raw)),
                (exit::FAILURE, format!("{NOT_ROOT_MESSAGE}\n")),
                "{raw:?}"
            );
        }
    }

    #[test]
    fn sigterm_stops_the_wait_and_the_others_are_ignored() {
        // SIGUSR1 and SIGUSR2 stand in for SIGTERM and SIGINT, so the test
        // never signals the test process with something that ends it.
        use signal_hook::consts::{SIGUSR1, SIGUSR2};
        let (tx, rx) = mpsc::channel();
        watch(&[SIGUSR1, SIGUSR2], &[SIGUSR1], tx).unwrap();
        signal_hook::low_level::raise(SIGUSR2).unwrap();
        signal_hook::low_level::raise(SIGUSR1).unwrap();
        let got = rx.recv_timeout(std::time::Duration::from_secs(5)).unwrap();
        assert_eq!(got, Event::Stop);
        assert!(
            rx.recv_timeout(std::time::Duration::from_millis(100))
                .is_err(),
            "the ignored signal sent nothing"
        );
    }
}
