//! The real privilege backend: `pkexec /usr/bin/spela apply-profile`.
//!
//! This is the only module that may name `pkexec` or `sudo`. A safety test
//! fails when another source file does. `pkexec` is found by a PATH lookup,
//! never by an absolute path, so a test's PATH stub always catches a call.
//!
//! Phase 6 builds the round trip (decision 30): one long-lived helper per
//! launch with JSON lines on its stdin and stdout, and `apply-profile
//! --once` for the live commands. Until then every call says it is not
//! built, and nothing is spawned.

use spela_core::backend::{
    ApplyReport, HardwareRequest, Privilege, PrivilegeError, PrivilegedSession,
};

/// The polkit agent program, looked up on PATH.
pub const PKEXEC: &str = "pkexec";
/// Where the polkit policy expects spela (its `exec.path` annotation).
pub const INSTALL_PATH: &str = "/usr/bin/spela";
/// The polkit action id, a contract with installed policies.
pub const ACTION_ID: &str = "io.github.spela.apply-profile";

/// The real backend. Spawns nothing yet.
#[derive(Debug, Default)]
pub struct PolkitPrivilege;

impl PolkitPrivilege {
    fn not_built() -> PrivilegeError {
        PrivilegeError::Unavailable(format!(
            "running {INSTALL_PATH} through {PKEXEC} is not yet implemented in this build"
        ))
    }
}

impl Privilege for PolkitPrivilege {
    fn start(
        &mut self,
        _request: &HardwareRequest,
    ) -> Result<Box<dyn PrivilegedSession>, PrivilegeError> {
        Err(Self::not_built())
    }

    fn apply_once(&mut self, _request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError> {
        Err(Self::not_built())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn nothing_runs_yet() {
        let mut p = PolkitPrivilege;
        let request = HardwareRequest {
            smt: Some(false),
            ..HardwareRequest::default()
        };
        assert!(matches!(
            p.start(&request).err(),
            Some(PrivilegeError::Unavailable(_))
        ));
        assert!(p.apply_once(&request).is_err());
        assert_eq!(ACTION_ID, "io.github.spela.apply-profile");
    }
}
