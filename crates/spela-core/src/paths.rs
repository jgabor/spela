//! Where spela keeps its files.
//!
//! The four base directories are spec-strict XDG, as etcetera resolves them
//! (decision 23). A relative `XDG_*` value is ignored, as the spec says, and
//! the default under `$HOME` is used instead (decision 43). The app name
//! `spela` is joined onto each base, so with nothing set the directories are
//! `~/.config/spela`, `~/.local/share/spela`, `~/.cache/spela` and
//! `~/.local/state/spela`. These are the folders Go spela used for config,
//! data and cache, so the migration finds the Go files where Go left them.
//!
//! The runtime directory is never used. Locks live under `<state>/locks`,
//! so every process finds the same lock file whether or not
//! `XDG_RUNTIME_DIR` is set.
//!
//! Tests and demo mode use [`Paths::under`] with a temporary base, so they
//! never read or write the user's real directories.

use std::io;
use std::path::{Path, PathBuf};

use etcetera::app_strategy::{AppStrategy, AppStrategyArgs, Xdg};

/// The name joined onto every base directory.
pub const APP_NAME: &str = "spela";
/// The config file in the config directory.
pub const CONFIG_FILE: &str = "config.toml";
/// The log folder in the state directory.
pub const LOG_DIR: &str = "log";
/// The lock folder in the state directory.
pub const LOCKS_DIR: &str = "locks";
/// The TUI instance lock in the lock folder.
pub const TUI_LOCK: &str = "tui.lock";
/// The launch lock in the lock folder, held around hardware changes.
pub const LAUNCH_LOCK: &str = "launch.lock";

/// Failure to find the user's home directory.
#[derive(Debug, thiserror::Error)]
#[error("cannot find your home directory, so spela does not know where to keep its files")]
pub struct PathsError;

/// spela's directories, each with `spela` already joined.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Paths {
    /// User intent: config, profiles and lists.
    pub config_dir: PathBuf,
    /// Durable data: DLL backups and the migration record.
    pub data_dir: PathBuf,
    /// Rebuildable data: the game cache, the manifest and the DLL cache.
    pub cache_dir: PathBuf,
    /// Logs and locks.
    pub state_dir: PathBuf,
}

impl Paths {
    /// The user's directories, from `HOME` and the `XDG_*` variables.
    /// Nothing is created. Call this once, in `main`: etcetera reads the
    /// process environment itself.
    pub fn from_system() -> Result<Paths, PathsError> {
        let xdg = Xdg::new(AppStrategyArgs {
            top_level_domain: "io.github".into(),
            author: "spela".into(),
            app_name: APP_NAME.into(),
        })
        .map_err(|_| PathsError)?;
        let data_dir = xdg.data_dir();
        Ok(Paths {
            config_dir: xdg.config_dir(),
            cache_dir: xdg.cache_dir(),
            // The XDG strategy always has a state directory.
            state_dir: xdg.state_dir().unwrap_or_else(|| data_dir.join("state")),
            data_dir,
        })
    }

    /// Directories under one base: `<base>/config`, `<base>/data`,
    /// `<base>/cache` and `<base>/state`. Used by tests and demo mode.
    pub fn under(base: &Path) -> Paths {
        Paths {
            config_dir: base.join("config"),
            data_dir: base.join("data"),
            cache_dir: base.join("cache"),
            state_dir: base.join("state"),
        }
    }

    /// The config file.
    pub fn config_file(&self) -> PathBuf {
        self.config_dir.join(CONFIG_FILE)
    }

    /// Where log files go. Mode 0700.
    pub fn log_dir(&self) -> PathBuf {
        self.state_dir.join(LOG_DIR)
    }

    /// Where lock files go. Mode 0700.
    pub fn locks_dir(&self) -> PathBuf {
        self.state_dir.join(LOCKS_DIR)
    }

    /// The TUI instance lock.
    pub fn tui_lock(&self) -> PathBuf {
        self.locks_dir().join(TUI_LOCK)
    }

    /// The launch lock.
    pub fn launch_lock(&self) -> PathBuf {
        self.locks_dir().join(LAUNCH_LOCK)
    }

    /// The four base directories, for checks that walk them.
    pub fn bases(&self) -> [&Path; 4] {
        [
            &self.config_dir,
            &self.data_dir,
            &self.cache_dir,
            &self.state_dir,
        ]
    }
}

/// Creates a directory and its parents. The last component gets mode 0700,
/// and an existing one is tightened to 0700. Used for the log and lock
/// folders.
pub fn create_private_dir(dir: &Path) -> io::Result<()> {
    use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
    if let Some(parent) = dir.parent() {
        std::fs::create_dir_all(parent)?;
    }
    match std::fs::DirBuilder::new().mode(0o700).create(dir) {
        Ok(()) => Ok(()),
        Err(e) if e.kind() == io::ErrorKind::AlreadyExists && dir.is_dir() => {
            std::fs::set_permissions(dir, std::fs::Permissions::from_mode(0o700))
        }
        Err(e) => Err(e),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn under_puts_everything_below_the_base() {
        let base = tempfile::tempdir().unwrap();
        let p = Paths::under(base.path());
        for dir in p.bases() {
            assert!(dir.starts_with(base.path()), "{dir:?}");
        }
        assert_eq!(p.config_file(), base.path().join("config/config.toml"));
        assert_eq!(p.log_dir(), base.path().join("state/log"));
        assert_eq!(p.tui_lock(), base.path().join("state/locks/tui.lock"));
        assert_eq!(p.launch_lock(), base.path().join("state/locks/launch.lock"));
    }

    #[test]
    fn private_dir_is_0700_and_tightened() {
        use std::os::unix::fs::PermissionsExt;
        let base = tempfile::tempdir().unwrap();
        let dir = base.path().join("a/b");
        create_private_dir(&dir).unwrap();
        let mode = |d: &Path| std::fs::metadata(d).unwrap().permissions().mode() & 0o777;
        assert_eq!(mode(&dir), 0o700);
        std::fs::set_permissions(&dir, std::fs::Permissions::from_mode(0o755)).unwrap();
        create_private_dir(&dir).unwrap();
        assert_eq!(mode(&dir), 0o700);
    }

    #[test]
    fn private_dir_refuses_a_file_in_the_way() {
        let base = tempfile::tempdir().unwrap();
        let file = base.path().join("taken");
        std::fs::write(&file, b"x").unwrap();
        assert!(create_private_dir(&file).is_err());
    }
}
