//! The two instance locks (decision 29), both flock under `<state>/locks`.
//!
//! - The TUI lock: one `spela tui` at a time.
//! - The launch lock: held from the first hardware or VRR change of a
//!   launch until its last restore, and by the live hardware commands. A
//!   second launch while it is held launches with env-only settings
//!   (decision 33).
//!
//! A held lock prints `spela is already running (PID: N)` and exits 5.

use spela_core::lock::{InstanceLock, LockError};
use spela_core::paths::Paths;

use crate::error::CliError;

/// Takes the TUI lock, or fails with exit code 5 when another TUI has it.
pub fn tui(paths: &Paths) -> Result<InstanceLock, CliError> {
    InstanceLock::acquire(&paths.tui_lock()).map_err(CliError::from)
}

/// Takes the launch lock. The caller decides what a held lock means: the
/// CLI exits 5, a second launch goes on without hardware changes.
pub fn launch(paths: &Paths) -> Result<InstanceLock, LockError> {
    InstanceLock::acquire(&paths.launch_lock())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::exit;

    #[test]
    fn locks_live_under_state_and_exclude() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let tui_lock = tui(&paths).unwrap();
        assert!(tui_lock.path().starts_with(base.path().join("state/locks")));
        let err = tui(&paths).unwrap_err();
        assert_eq!(err.code, exit::LOCKED);
        assert!(err.message.starts_with("spela is already running (PID: "));

        // The launch lock is separate from the TUI lock.
        let launch_lock = launch(&paths).unwrap();
        assert!(matches!(launch(&paths), Err(LockError::Held { .. })));
        drop(launch_lock);
        drop(tui_lock);
        assert!(tui(&paths).is_ok());
    }
}
