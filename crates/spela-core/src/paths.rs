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
/// The store lock for config, profiles, lists and the migration record.
pub const CONFIG_LOCK: &str = "config.lock";
/// The store lock for the game cache, DLL swaps and backups.
pub const GAMES_LOCK: &str = "games.lock";
/// The profile folder in the config directory.
pub const PROFILES_DIR: &str = "profiles";
/// The All games profile in the profile folder.
pub const ALL_GAMES_PROFILE: &str = "all-games.toml";
/// The list folder in the config directory.
pub const LISTS_DIR: &str = "lists";
/// The user's allow and deny overrides in the list folder.
pub const DENY_OVERRIDES: &str = "overrides.toml";
/// The optional user deny list in the list folder.
pub const USER_DENYLIST: &str = "denylist.toml";
/// The migration record in the data directory.
pub const MIGRATION_RECORD: &str = "migration.toml";
/// The backup folder in the data directory.
pub const BACKUPS_DIR: &str = "backups";
/// The backup index in each game's backup folder.
pub const BACKUP_INDEX: &str = "backup.json";
/// The game cache in the cache directory (decision 43).
pub const GAME_CACHE: &str = "games.json";
/// The DLL manifest cache in the cache directory (decision 53).
pub const MANIFEST_CACHE: &str = "manifest-v1.json";
/// The DLL cache folder in the cache directory, shared with Go (decision 53).
pub const DLL_CACHE_DIR: &str = "dlls";
/// The release check in the cache directory (decision 53).
pub const RELEASE_CHECK: &str = "release-check.json";
/// The terminal capability cache in the cache directory.
pub const CAPS_CACHE: &str = "caps.toml";
/// The NVIDIA shader cache folder in the cache directory, the default of
/// the `shader_cache` setting. The driver writes it, not spela.
pub const SHADER_CACHE_DIR: &str = "nvidia";

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

    /// The lock every write to the config, profiles, lists and migration
    /// record takes.
    pub fn config_lock(&self) -> PathBuf {
        self.locks_dir().join(CONFIG_LOCK)
    }

    /// The lock every write to the game cache, DLLs and backups takes.
    pub fn games_lock(&self) -> PathBuf {
        self.locks_dir().join(GAMES_LOCK)
    }

    /// Where the profiles live.
    pub fn profiles_dir(&self) -> PathBuf {
        self.config_dir.join(PROFILES_DIR)
    }

    /// The All games profile.
    pub fn all_games_profile(&self) -> PathBuf {
        self.profiles_dir().join(ALL_GAMES_PROFILE)
    }

    /// One game's profile.
    pub fn game_profile(&self, app_id: u64) -> PathBuf {
        self.profiles_dir().join(format!("{app_id}.toml"))
    }

    /// Where the deny lists live.
    pub fn lists_dir(&self) -> PathBuf {
        self.config_dir.join(LISTS_DIR)
    }

    /// The user's allow and deny overrides.
    pub fn deny_overrides(&self) -> PathBuf {
        self.lists_dir().join(DENY_OVERRIDES)
    }

    /// The optional user deny list, which adds to the built-in one.
    pub fn user_denylist(&self) -> PathBuf {
        self.lists_dir().join(USER_DENYLIST)
    }

    /// The record of what the migration from Go spela did.
    pub fn migration_record(&self) -> PathBuf {
        self.data_dir.join(MIGRATION_RECORD)
    }

    /// The folder of backup indexes, and the default folder for backup
    /// files when `backup_path` is empty.
    pub fn backups_dir(&self) -> PathBuf {
        self.data_dir.join(BACKUPS_DIR)
    }

    /// One game's backup index. It stays here even when `backup_path`
    /// moves the backup files elsewhere.
    pub fn backup_index(&self, app_id: u64) -> PathBuf {
        self.backups_dir()
            .join(app_id.to_string())
            .join(BACKUP_INDEX)
    }

    /// The game cache.
    pub fn game_cache(&self) -> PathBuf {
        self.cache_dir.join(GAME_CACHE)
    }

    /// The DLL manifest cache.
    pub fn manifest_cache(&self) -> PathBuf {
        self.cache_dir.join(MANIFEST_CACHE)
    }

    /// The default DLL cache folder, used when `dll_cache_path` is empty.
    pub fn default_dll_cache_dir(&self) -> PathBuf {
        self.cache_dir.join(DLL_CACHE_DIR)
    }

    /// The release check.
    pub fn release_check(&self) -> PathBuf {
        self.cache_dir.join(RELEASE_CHECK)
    }

    /// The terminal capability cache.
    pub fn caps_cache(&self) -> PathBuf {
        self.cache_dir.join(CAPS_CACHE)
    }

    /// The default shader cache folder, used when `shader_cache` is empty.
    pub fn default_shader_cache_dir(&self) -> PathBuf {
        self.cache_dir.join(SHADER_CACHE_DIR)
    }

    /// The files Go spela wrote, which the migration reads. Each stays in
    /// place as a backup.
    pub fn go(&self) -> GoPaths<'_> {
        GoPaths { paths: self }
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

/// Where Go spela kept its files. The folders are the same as spela's,
/// because both use the XDG homes, but the names and formats differ.
#[derive(Debug, Clone, Copy)]
pub struct GoPaths<'a> {
    paths: &'a Paths,
}

impl GoPaths<'_> {
    /// Go's `config.yaml`.
    pub fn config(&self) -> PathBuf {
        self.paths.config_dir.join("config.yaml")
    }

    /// Go's All games profile, `profiles/default.yaml`.
    pub fn all_games_profile(&self) -> PathBuf {
        self.paths.profiles_dir().join("default.yaml")
    }

    /// Go's profile for one game, `profiles/<appid>.yaml`. Go's `Load` only
    /// ever opened this name, so a `.yml` file never took effect.
    pub fn game_profile(&self, app_id: u64) -> PathBuf {
        self.paths.profiles_dir().join(format!("{app_id}.yaml"))
    }

    /// Go's allow and deny overrides, `lists/overrides.yaml`.
    pub fn deny_overrides(&self) -> PathBuf {
        self.paths.lists_dir().join("overrides.yaml")
    }

    /// Go's user deny list, `lists/denylist.yaml`.
    pub fn user_denylist(&self) -> PathBuf {
        self.paths.lists_dir().join("denylist.yaml")
    }

    /// Go's game database, `<data>/games.yaml`.
    pub fn game_database(&self) -> PathBuf {
        self.paths.data_dir.join("games.yaml")
    }

    /// Go's manifest cache, the seed for [`Paths::manifest_cache`].
    pub fn manifest_cache(&self) -> PathBuf {
        self.paths.cache_dir.join("manifest.json")
    }

    /// Go's update check, the seed for [`Paths::release_check`].
    pub fn update_check(&self) -> PathBuf {
        self.paths.cache_dir.join("update-check.json")
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
    fn every_file_in_the_files_table_has_its_path() {
        let base = Path::new("/b");
        let p = Paths::under(base);
        let cases = [
            (p.config_lock(), "state/locks/config.lock"),
            (p.games_lock(), "state/locks/games.lock"),
            (p.all_games_profile(), "config/profiles/all-games.toml"),
            (p.game_profile(9100010), "config/profiles/9100010.toml"),
            (p.deny_overrides(), "config/lists/overrides.toml"),
            (p.user_denylist(), "config/lists/denylist.toml"),
            (p.migration_record(), "data/migration.toml"),
            (p.backups_dir(), "data/backups"),
            (p.backup_index(9100010), "data/backups/9100010/backup.json"),
            (p.game_cache(), "cache/games.json"),
            (p.manifest_cache(), "cache/manifest-v1.json"),
            (p.default_dll_cache_dir(), "cache/dlls"),
            (p.release_check(), "cache/release-check.json"),
            (p.caps_cache(), "cache/caps.toml"),
            (p.default_shader_cache_dir(), "cache/nvidia"),
            (p.go().config(), "config/config.yaml"),
            (p.go().all_games_profile(), "config/profiles/default.yaml"),
            (p.go().game_profile(9100010), "config/profiles/9100010.yaml"),
            (p.go().deny_overrides(), "config/lists/overrides.yaml"),
            (p.go().user_denylist(), "config/lists/denylist.yaml"),
            (p.go().game_database(), "data/games.yaml"),
            (p.go().manifest_cache(), "cache/manifest.json"),
            (p.go().update_check(), "cache/update-check.json"),
        ];
        for (got, want) in cases {
            assert_eq!(got, base.join(want));
        }
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
