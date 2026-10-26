//! Exit codes and the error every command returns.

use std::io;

use spela_core::lock::LockError;

/// Exit codes, after mejl's. 4 stays free so the two tools agree.
pub mod exit {
    /// Success, help and version.
    pub const OK: u8 = 0;
    /// Any other failure.
    pub const FAILURE: u8 = 1;
    /// A usage error, from clap or from spela's own checks.
    pub const USAGE: u8 = 2;
    /// A config or profile file is invalid.
    pub const CONFIG: u8 = 3;
    /// Another spela holds the lock.
    pub const LOCKED: u8 = 5;
}

/// A failure with its exit code. The message and hint are for the user.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CliError {
    /// The exit code.
    pub code: u8,
    /// What went wrong, in plain words.
    pub message: String,
    /// One line on what to do next, when one helps.
    pub hint: Option<String>,
    /// A stable snake_case id for the `--json` error object.
    pub kind: &'static str,
}

impl CliError {
    /// A failure with a code. The `--json` id follows the code.
    pub fn new(code: u8, message: impl Into<String>) -> Self {
        let kind = match code {
            exit::USAGE => "usage",
            exit::CONFIG => "config_invalid",
            exit::LOCKED => "locked",
            _ => "failure",
        };
        CliError {
            code,
            message: message.into(),
            hint: None,
            kind,
        }
    }

    /// Adds a one-line hint.
    pub fn with_hint(mut self, hint: impl Into<String>) -> Self {
        self.hint = Some(hint.into());
        self
    }

    /// Sets the `--json` id.
    pub fn with_kind(mut self, kind: &'static str) -> Self {
        self.kind = kind;
        self
    }

    /// A command a later phase fills.
    pub fn not_implemented(command: &str) -> Self {
        CliError::new(
            exit::FAILURE,
            format!("'spela {command}' is not yet implemented in this build"),
        )
        .with_kind("not_implemented")
    }

    /// stdout was closed, as in `spela list --json | head`. This is a quiet
    /// exit 0, never a panic.
    pub fn broken_pipe() -> Self {
        CliError::new(exit::OK, "").with_kind("broken_pipe")
    }

    /// Whether this is [`CliError::broken_pipe`].
    pub fn is_broken_pipe(&self) -> bool {
        self.kind == "broken_pipe"
    }
}

impl CliError {
    /// Maps a failed write to stdout. A closed stdout is a quiet exit 0;
    /// any other error is a failure. Only stdout gets this: a closed pipe
    /// to the root helper is a real failure.
    pub fn from_stdout(e: io::Error) -> Self {
        if e.kind() == io::ErrorKind::BrokenPipe {
            CliError::broken_pipe()
        } else {
            CliError::new(exit::FAILURE, format!("cannot write the output: {e}"))
        }
    }
}

impl From<io::Error> for CliError {
    fn from(e: io::Error) -> Self {
        CliError::new(exit::FAILURE, e.to_string())
    }
}

impl From<LockError> for CliError {
    fn from(e: LockError) -> Self {
        let code = match e {
            LockError::Held { .. } => exit::LOCKED,
            LockError::Io { .. } => exit::FAILURE,
        };
        CliError::new(code, e.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn kinds_follow_codes() {
        assert_eq!(CliError::new(exit::FAILURE, "x").kind, "failure");
        assert_eq!(CliError::new(exit::USAGE, "x").kind, "usage");
        assert_eq!(CliError::new(exit::CONFIG, "x").kind, "config_invalid");
        assert_eq!(CliError::new(exit::LOCKED, "x").kind, "locked");
        let e = CliError::not_implemented("scan");
        assert_eq!((e.code, e.kind), (exit::FAILURE, "not_implemented"));
        assert_eq!(
            e.message,
            "'spela scan' is not yet implemented in this build"
        );
    }

    #[test]
    fn only_a_closed_stdout_is_a_quiet_success() {
        let pipe = || io::Error::from(io::ErrorKind::BrokenPipe);
        let e = CliError::from_stdout(pipe());
        assert!(e.is_broken_pipe());
        assert_eq!(e.code, exit::OK);
        let full = CliError::from_stdout(io::Error::other("disk full"));
        assert!(!full.is_broken_pipe());
        assert_eq!(full.code, exit::FAILURE);
        // Any other closed pipe, such as the root helper's, is a failure.
        let helper = CliError::from(pipe());
        assert!(!helper.is_broken_pipe());
        assert_eq!(helper.code, exit::FAILURE);
    }

    #[test]
    fn held_lock_exits_5() {
        let held = LockError::Held {
            path: "/tmp/x".into(),
            pid: Some(42),
        };
        let e = CliError::from(held);
        assert_eq!(e.code, exit::LOCKED);
        assert_eq!(e.message, "spela is already running (PID: 42)");
    }
}
