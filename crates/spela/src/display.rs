//! The real KDE display backend: `kscreen-doctor` in the user session.
//!
//! As in Go (`internal/display/vrr.go`): each call runs with `LC_ALL=C` and
//! a 10 s timeout, reads stdout only, and a set whose stdout says
//! `applying config failed!` is an error even though kscreen-doctor exits
//! 0. kscreen-doctor needs the desktop session's Wayland socket and aborts
//! with SIGABRT without one (`docs/phase0-checks.md`), which reads as
//! unavailable. The program is found on PATH, so a test's PATH stub always
//! catches a call. Tests use the fake, or a script in a temp dir in place
//! of the program, and never run kscreen-doctor.

use std::io::Read;
use std::os::unix::process::ExitStatusExt;
use std::path::PathBuf;
use std::process::{Command, Stdio};
use std::time::Duration;

use wait_timeout::ChildExt;

use spela_core::backend::{Display, DisplayError, Output, VrrPolicy};
use spela_core::kscreen;

/// The program, looked up on PATH.
const PROGRAM: &str = "kscreen-doctor";

/// How long one call may take.
pub const TIMEOUT: Duration = Duration::from_secs(10);

/// KDE VRR through `kscreen-doctor`.
#[derive(Debug, Clone)]
pub struct KscreenDisplay {
    program: PathBuf,
    timeout: Duration,
}

impl KscreenDisplay {
    /// `kscreen-doctor` from PATH, with the 10 s timeout.
    pub fn system() -> KscreenDisplay {
        KscreenDisplay {
            program: PathBuf::from(PROGRAM),
            timeout: TIMEOUT,
        }
    }

    /// Runs the program with `args` and returns its stdout.
    fn run(&self, args: &[&str]) -> Result<String, DisplayError> {
        let what = format!("{PROGRAM} {}", args.join(" "));
        let mut child = Command::new(&self.program)
            .args(args)
            .env("LC_ALL", "C")
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => {
                    DisplayError::Unavailable(format!("{PROGRAM} is not installed"))
                }
                _ => DisplayError::Failed(format!("{what}: {e}")),
            })?;
        // Read on a thread, so a full pipe never stalls the wait.
        let mut stdout = child.stdout.take().expect("stdout is piped");
        let reader = std::thread::spawn(move || {
            let mut text = Vec::new();
            let _ = stdout.read_to_end(&mut text);
            text
        });
        let status = match child.wait_timeout(self.timeout) {
            Ok(Some(status)) => status,
            Ok(None) => {
                let _ = child.kill();
                let _ = child.wait();
                let _ = reader.join();
                return Err(DisplayError::TimedOut);
            }
            Err(e) => {
                let _ = child.kill();
                let _ = child.wait();
                return Err(DisplayError::Failed(format!("{what}: {e}")));
            }
        };
        let text = String::from_utf8_lossy(&reader.join().unwrap_or_default()).into_owned();
        if let Some(signal) = status.signal() {
            return Err(DisplayError::Unavailable(format!(
                "{what} ended by signal {signal}; is this a KDE Wayland session?"
            )));
        }
        if !status.success() {
            return Err(DisplayError::Failed(format!("{what}: {status}")));
        }
        Ok(text)
    }
}

impl Display for KscreenDisplay {
    fn outputs(&mut self) -> Result<Vec<Output>, DisplayError> {
        kscreen::parse_outputs(&self.run(&["-o"])?)
    }

    fn set_vrr(&mut self, uuid: &str, policy: VrrPolicy) -> Result<(), DisplayError> {
        let stdout = self.run(&[&format!("output.{uuid}.vrrpolicy.{policy}")])?;
        if kscreen::set_failed(&stdout) {
            return Err(DisplayError::Failed(format!(
                "{PROGRAM}: {}",
                stdout.trim()
            )));
        }
        Ok(())
    }
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

    /// A stand-in program: a shell script that logs its arguments and
    /// `LC_ALL`, then runs `body`.
    fn stand_in(dir: &std::path::Path, body: &str) -> KscreenDisplay {
        let log = dir.join("calls");
        let path = dir.join("doctor-stand-in");
        write_script(
            &path,
            format!(
                "#!/bin/sh\necho \"$LC_ALL $*\" >> '{}'\n{body}\n",
                log.display()
            ),
        );
        KscreenDisplay {
            program: path,
            timeout: Duration::from_secs(5),
        }
    }

    fn calls(dir: &std::path::Path) -> String {
        std::fs::read_to_string(dir.join("calls")).unwrap_or_default()
    }

    #[test]
    fn reads_outputs_and_sets_with_lc_all_c() {
        let dir = tempfile::tempdir().unwrap();
        let fixture = concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../tests/fixtures/kscreen/doctor-o.txt"
        );
        let mut d = stand_in(
            dir.path(),
            &format!("[ \"$1\" = -o ] && cat '{fixture}'\nexit 0"),
        );
        let outputs = d.outputs().unwrap();
        assert_eq!(outputs[0].name, "DP-3");
        assert_eq!(outputs[0].vrr_policy(), Some(VrrPolicy::Never));
        d.set_vrr(&outputs[0].uuid, VrrPolicy::Always).unwrap();
        assert_eq!(
            calls(dir.path()),
            "C -o\nC output.4f1e2d3c-5b6a-4789-8abc-def012345601.vrrpolicy.always\n"
        );
    }

    #[test]
    fn a_zero_exit_failure_on_stdout_is_an_error() {
        let dir = tempfile::tempdir().unwrap();
        let mut d = stand_in(
            dir.path(),
            "echo 'applying config failed! Backend rejected configuration'",
        );
        assert!(matches!(
            d.set_vrr("u", VrrPolicy::Never),
            Err(DisplayError::Failed(m)) if m.contains("applying config failed!")
        ));
    }

    #[test]
    fn failures_map_to_their_errors() {
        let dir = tempfile::tempdir().unwrap();
        let mut aborts = stand_in(dir.path(), "kill -ABRT $$");
        assert!(matches!(
            aborts.outputs(),
            Err(DisplayError::Unavailable(_))
        ));
        let mut fails = stand_in(dir.path(), "exit 3");
        assert!(matches!(fails.outputs(), Err(DisplayError::Failed(_))));
        let mut hangs = stand_in(dir.path(), "exec /bin/sleep 30");
        hangs.timeout = Duration::from_millis(200);
        assert_eq!(hangs.outputs(), Err(DisplayError::TimedOut));
        let mut missing = KscreenDisplay {
            program: dir.path().join("not-there"),
            timeout: TIMEOUT,
        };
        assert!(matches!(
            missing.outputs(),
            Err(DisplayError::Unavailable(_))
        ));
    }
}
