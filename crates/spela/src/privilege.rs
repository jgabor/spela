//! The real privilege backend: `pkexec /usr/bin/spela apply-profile`.
//!
//! This is the only module that may name `pkexec` or `sudo`. A safety test
//! fails when another source file does. `pkexec` is found by a PATH lookup,
//! never by an absolute path, so a test's PATH stub always catches a call.
//!
//! One `pkexec` per call (decision 30):
//!
//! - [`Privilege::start`] runs `apply-profile --serve` with pipes on its
//!   stdin and stdout, sends the batch, reads one line per step and keeps
//!   the waiting helper as the session. The session's restore, or its drop,
//!   makes the helper restore (see [`spela_core::helper::client`]).
//! - [`Privilege::apply_once`] runs `apply-profile --once <request>` for the
//!   live `gpu reset`, `cpu governor` and `cpu smt`, and reads its report.
//!   The CLI leaves stdin on the terminal, so pkexec can fall back to its
//!   text agent there. The TUI passes `--disable-internal-agent` and a null
//!   stdin, so pkexec never prompts on the TUI's tty (decision 44).
//!
//! pkexec's own failures map as Go's did (parity-inventory.md, CLI 7.1):
//! not on PATH is unavailable; exit 126 or a cancel message is a dismissed
//! prompt; anything else is the agent's or the helper's text.

use std::ffi::OsString;
use std::io::{BufReader, Read};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};

use spela_core::backend::{
    ApplyReport, HardwareRequest, Privilege, PrivilegeError, PrivilegedSession,
};
use spela_core::helper::client::{self, Finish, HelperExit, StartError};
use spela_core::helper::protocol;

/// The polkit agent program, looked up on PATH.
pub const PKEXEC: &str = "pkexec";
/// Where the polkit policy expects spela (its `exec.path` annotation).
/// Steam runs launches with `PATH=/usr/bin:/bin`, so this is also where
/// the wrapper is found.
pub const INSTALL_PATH: &str = "/usr/bin/spela";
/// The polkit action id, a contract with installed policies.
pub const ACTION_ID: &str = "io.github.spela.apply-profile";
/// pkexec's flag that keeps it from prompting on a terminal.
pub const NO_TEXT_AGENT: &str = "--disable-internal-agent";
/// Go's error when pkexec is missing.
pub const NO_POLKIT: &str = "pkexec not available: install polkit for privileged operations";

/// The program pkexec runs, and whether it is the path the policy names.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HelperProgram {
    /// The path passed to pkexec.
    pub path: PathBuf,
    /// Whether it is [`INSTALL_PATH`], so polkit uses spela's action with
    /// `auth_admin_keep`. Any other path gets the generic action, which
    /// asks every time.
    pub policy: bool,
}

/// The program for a running binary at `current`: [`INSTALL_PATH`] when
/// both are the same file, otherwise `current` with its symlinks resolved,
/// as Go's `ExecSelf` did (parity-inventory.md, CLI 7.1).
pub fn helper_program(current: &Path, install: &Path) -> HelperProgram {
    let resolved = current
        .canonicalize()
        .unwrap_or_else(|_| current.to_owned());
    if install
        .canonicalize()
        .is_ok_and(|installed| installed == resolved)
    {
        return HelperProgram {
            path: install.to_owned(),
            policy: true,
        };
    }
    HelperProgram {
        path: resolved,
        policy: false,
    }
}

/// This binary's helper program, or `None` when the running binary cannot
/// be found. `/proc/self/exe` is what `current_exe` reads on Linux.
pub fn this_helper() -> Option<HelperProgram> {
    let current = std::fs::read_link("/proc/self/exe").ok()?;
    Some(helper_program(&current, Path::new(INSTALL_PATH)))
}

/// Where a call comes from, which decides how pkexec may prompt.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Caller {
    /// The CLI or the wrapper: pkexec may use its text agent.
    Cli,
    /// The TUI: no text agent and a null stdin (decision 44).
    Tui,
}

/// The helper mode a call runs.
#[derive(Debug, Clone, PartialEq, Eq)]
enum HelperMode {
    Serve,
    Once(String),
}

/// The program and arguments for one call. As root, the helper runs
/// directly, as Go's `Exec` did; otherwise through pkexec.
fn command_line(
    program: &Path,
    mode: &HelperMode,
    caller: Caller,
    root: bool,
) -> (OsString, Vec<OsString>) {
    let mut args: Vec<OsString> = Vec::new();
    if !root && caller == Caller::Tui {
        args.push(NO_TEXT_AGENT.into());
    }
    if !root {
        args.push(program.into());
    }
    args.push("apply-profile".into());
    match mode {
        HelperMode::Serve => args.push("--serve".into()),
        HelperMode::Once(request) => {
            args.push("--once".into());
            args.push(request.into());
        }
    }
    let launcher = if root {
        program.as_os_str().to_owned()
    } else {
        PKEXEC.into()
    };
    (launcher, args)
}

/// The real backend.
#[derive(Debug)]
pub struct PolkitPrivilege {
    caller: Caller,
}

impl PolkitPrivilege {
    /// For the CLI and the wrapper.
    pub fn cli() -> PolkitPrivilege {
        PolkitPrivilege {
            caller: Caller::Cli,
        }
    }

    /// For the TUI's hardware jobs.
    pub fn tui() -> PolkitPrivilege {
        PolkitPrivilege {
            caller: Caller::Tui,
        }
    }

    fn spawn(&self, mode: &HelperMode, stdin: Stdio) -> Result<Child, PrivilegeError> {
        let program = this_helper().ok_or_else(|| {
            PrivilegeError::Unavailable("cannot find the spela binary to run as root".into())
        })?;
        // SAFETY: geteuid has no preconditions and cannot fail.
        let root = unsafe { libc::geteuid() == 0 };
        let (launcher, args) = command_line(&program.path, mode, self.caller, root);
        tracing::info!(
            program = %program.path.display(),
            policy = program.policy,
            caller = ?self.caller,
            mode = if matches!(mode, HelperMode::Serve) { "serve" } else { "once" },
            "starting the privileged helper"
        );
        Command::new(launcher)
            .args(args)
            .stdin(stdin)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => PrivilegeError::Unavailable(NO_POLKIT.into()),
                _ => PrivilegeError::Helper(format!("cannot start {PKEXEC}: {e}")),
            })
    }

    fn stdin(&self) -> Stdio {
        match self.caller {
            Caller::Cli => Stdio::inherit(),
            Caller::Tui => Stdio::null(),
        }
    }
}

impl Privilege for PolkitPrivilege {
    fn start(
        &mut self,
        request: &HardwareRequest,
    ) -> Result<Box<dyn PrivilegedSession>, PrivilegeError> {
        if request.is_empty() {
            return Ok(client::empty_session());
        }
        let mut child = self.spawn(&HelperMode::Serve, Stdio::piped())?;
        let (Some(input), Some(output)) = (child.stdin.take(), child.stdout.take()) else {
            let _ = child.kill();
            let _ = child.wait();
            return Err(PrivilegeError::Helper("the helper has no pipes".into()));
        };
        let finish = finish(child);
        client::start(
            request,
            Box::new(input),
            Box::new(BufReader::new(output)),
            finish,
        )
        .map(|s| Box::new(s) as Box<dyn PrivilegedSession>)
        .map_err(StartError::into_privilege_error)
    }

    fn apply_once(&mut self, request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError> {
        if request.is_empty() {
            return Ok(ApplyReport::default());
        }
        let line = protocol::encode_request(&request.steps());
        let mut child = self.spawn(&HelperMode::Once(line), self.stdin())?;
        let Some(output) = child.stdout.take() else {
            let _ = child.kill();
            let _ = child.wait();
            return Err(PrivilegeError::Helper(
                "the helper has no output pipe".into(),
            ));
        };
        let finish = finish(child);
        client::read_once(request, &mut BufReader::new(output), finish)
            .map_err(StartError::into_privilege_error)
    }
}

/// Reads the child's stderr on a thread, so a full pipe never blocks it,
/// and waits for it once its output is read.
fn finish(mut child: Child) -> Finish {
    let stderr = child.stderr.take().and_then(|mut pipe| {
        std::thread::Builder::new()
            .name("spela-helper-stderr".into())
            .spawn(move || {
                let mut text = String::new();
                let _ = pipe.read_to_string(&mut text);
                text
            })
            .ok()
    });
    Box::new(move || {
        let code = child.wait().ok().and_then(|s| s.code());
        let stderr = stderr.and_then(|t| t.join().ok()).unwrap_or_default();
        if !stderr.trim().is_empty() {
            tracing::info!(code, stderr = stderr.trim(), "privileged helper stderr");
        }
        HelperExit { code, stderr }
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn strings(args: &[OsString]) -> Vec<String> {
        args.iter()
            .map(|a| a.to_string_lossy().into_owned())
            .collect()
    }

    #[test]
    fn the_cli_and_the_tui_run_pkexec_with_their_own_flags() {
        let program = Path::new(INSTALL_PATH);
        let (launcher, args) = command_line(program, &HelperMode::Serve, Caller::Cli, false);
        assert_eq!(launcher, PKEXEC);
        assert_eq!(strings(&args), [INSTALL_PATH, "apply-profile", "--serve"]);
        let once = HelperMode::Once("{}".into());
        let (launcher, args) = command_line(program, &once, Caller::Tui, false);
        assert_eq!(launcher, PKEXEC);
        assert_eq!(
            strings(&args),
            [NO_TEXT_AGENT, INSTALL_PATH, "apply-profile", "--once", "{}"]
        );
        // As root the helper runs directly, without pkexec.
        let (launcher, args) = command_line(program, &once, Caller::Tui, true);
        assert_eq!(launcher, INSTALL_PATH);
        assert_eq!(strings(&args), ["apply-profile", "--once", "{}"]);
        assert_eq!(ACTION_ID, "io.github.spela.apply-profile");
    }

    #[test]
    fn the_policy_path_is_used_only_for_the_installed_binary() {
        let dir = tempfile::tempdir().unwrap();
        let installed = dir.path().join("usr-bin-spela");
        std::fs::write(&installed, "").unwrap();
        let link = dir.path().join("link");
        std::os::unix::fs::symlink(&installed, &link).unwrap();
        // Running the installed file, directly or through a link.
        for running in [&installed, &link] {
            assert_eq!(
                helper_program(running, &installed),
                HelperProgram {
                    path: installed.clone(),
                    policy: true
                }
            );
        }
        // A build elsewhere gets the generic action, by its real path.
        let build = dir.path().join("target-spela");
        std::fs::write(&build, "").unwrap();
        let other_link = dir.path().join("other");
        std::os::unix::fs::symlink(&build, &other_link).unwrap();
        assert_eq!(
            helper_program(&other_link, &installed),
            HelperProgram {
                path: build.canonicalize().unwrap(),
                policy: false
            }
        );
        assert!(!helper_program(&build, &dir.path().join("missing")).policy);
    }

    #[test]
    fn the_policy_names_the_action_the_path_and_the_helper_argument() {
        let path =
            Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data/polkit/io.github.spela.policy");
        let policy = std::fs::read_to_string(path).unwrap();
        for line in [
            format!("<action id=\"{ACTION_ID}\">"),
            format!(
                "<annotate key=\"org.freedesktop.policykit.exec.path\">{INSTALL_PATH}</annotate>"
            ),
            "<annotate key=\"org.freedesktop.policykit.exec.argv1\">apply-profile</annotate>"
                .to_owned(),
            "<message>Authentication is required to apply GPU/CPU performance settings</message>"
                .to_owned(),
            "<allow_any>auth_admin</allow_any>".to_owned(),
            "<allow_inactive>auth_admin</allow_inactive>".to_owned(),
            "<allow_active>auth_admin_keep</allow_active>".to_owned(),
        ] {
            assert!(policy.contains(&line), "{line}");
        }
    }

    /// Runs `sh -c script` with the pipes the real backend uses, and reads
    /// it as a one-shot helper.
    fn run_shell(script: &str, request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError> {
        let mut child = Command::new("sh")
            .args(["-c", script])
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .unwrap();
        let output = child.stdout.take().unwrap();
        client::read_once(request, &mut BufReader::new(output), finish(child))
            .map_err(StartError::into_privilege_error)
    }

    #[test]
    fn a_real_child_s_exit_and_stderr_map_like_pkexec_s() {
        let request = HardwareRequest {
            smt: Some(false),
            ..HardwareRequest::default()
        };
        assert_eq!(
            run_shell(
                "echo 'Error executing command as another user: Request dismissed' >&2; exit 126",
                &request
            ),
            Err(PrivilegeError::Dismissed)
        );
        assert_eq!(
            run_shell(
                "echo 'Error executing command as another user: Not authorized' >&2; exit 127",
                &request
            ),
            Err(PrivilegeError::Helper(
                "Error executing command as another user: Not authorized".into()
            ))
        );
        let line =
            r#"{"type":"step","step":"smt","target":false,"outcome":"failed","error":"busy"}"#;
        let report = run_shell(&format!("echo '{line}'; exit 1"), &request).unwrap();
        assert_eq!(report.steps.len(), 1);
        assert!(!report.all_applied());
    }

    #[test]
    fn an_empty_request_runs_nothing() {
        // No pkexec is spawned for nothing: the empty paths return first.
        let mut p = PolkitPrivilege::tui();
        assert_eq!(
            p.apply_once(&HardwareRequest::default()).unwrap(),
            ApplyReport::default()
        );
        let session = p.start(&HardwareRequest::default()).unwrap();
        assert!(session.report().steps.is_empty());
        assert!(session.restore().unwrap().steps.is_empty());
    }
}
