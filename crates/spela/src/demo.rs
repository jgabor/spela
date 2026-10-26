//! `spela --demo`: the fake backends under a private temp dir.
//!
//! Every path comes from [`Paths::under`] a fresh temp dir that is removed
//! when the session drops. Only `<config>/config.toml` is copied in, so
//! theme and mouse settings carry over and the real file is never written.
//! The copy's `dll_cache_path` and `backup_path` are cleared, because the
//! demo installs, updates and restores DLLs: its payloads and backups stay
//! in the temp dir, never in the user's own DLL cache or backup folder.
//! The demo never reads Go YAML and never migrates (decision 18).
//!
//! Its network is [`spela_core::fake::dlls`]: a manifest, payloads that
//! match it and a tag list, so DLL flows run end to end and no request
//! leaves the process.
//!
//! The session also writes the fictional Steam library and a fictional
//! sysfs tree into the temp dir, so the TUI and every CLI command (`scan`,
//! `list`, `show`, `launch --dry-run`, the `dll` commands and the live CPU
//! commands) run on the same built-in games and the same fake hardware.
//!
//! The temp dir goes on every exit the process controls: the session's
//! drop on a normal quit or a main-thread panic, and [`remove_now`] on the
//! exits that skip drops, the TUI's second stop signal and a fatal panic
//! on another thread. A kill that cannot be caught, such as SIGKILL, leaves
//! it behind, so each new session removes the demo dirs whose process is
//! gone. A dir counts only when it has the [`MARKER`] file with that
//! process's PID and belongs to this user.

use std::io;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, MutexGuard};

/// The temp dir's name prefix.
const PREFIX: &str = "spela-demo-";

/// The file in a demo dir that names the process that made it.
pub const MARKER: &str = ".spela-demo-pid";

/// The running session's temp dir, for [`remove_now`].
static LIVE: Mutex<Option<PathBuf>> = Mutex::new(None);

fn live() -> MutexGuard<'static, Option<PathBuf>> {
    LIVE.lock().unwrap_or_else(|e| e.into_inner())
}

/// Removes the running demo's temp dir at once, for an exit that skips the
/// session's drop. A no-op when no demo runs or it is already gone.
pub fn remove_now() {
    let dir = live().take();
    if let Some(dir) = dir {
        let _ = std::fs::remove_dir_all(dir);
    }
}

/// Whether process `pid` still runs. A PID this user may not signal is
/// someone else's process, and counts as running.
fn running(pid: u32) -> bool {
    let Ok(pid) = libc::pid_t::try_from(pid) else {
        return true;
    };
    // SAFETY: signal 0 only checks that the process exists.
    if unsafe { libc::kill(pid, 0) } == 0 {
        return true;
    }
    io::Error::last_os_error().raw_os_error() != Some(libc::ESRCH)
}

/// Removes the demo dirs under `tmp`, other than `own`, that a process left
/// behind when it could not clean up: each needs this user's ownership, a
/// [`MARKER`] with a PID, and that process gone.
fn sweep_stale(tmp: &Path, own: &Path) {
    use std::os::unix::fs::MetadataExt;
    let Ok(entries) = std::fs::read_dir(tmp) else {
        return;
    };
    // SAFETY: geteuid has no preconditions and cannot fail.
    let uid = unsafe { libc::geteuid() };
    for entry in entries.flatten() {
        let path = entry.path();
        let named = entry
            .file_name()
            .to_str()
            .is_some_and(|n| n.starts_with(PREFIX));
        if !named || path == own {
            continue;
        }
        // Not a symlink: the metadata of the entry itself.
        let Ok(meta) = std::fs::symlink_metadata(&path) else {
            continue;
        };
        if !meta.is_dir() || meta.uid() != uid {
            continue;
        }
        let pid = std::fs::read_to_string(path.join(MARKER))
            .ok()
            .and_then(|text| text.trim().parse::<u32>().ok());
        if let Some(pid) = pid
            && !running(pid)
        {
            match std::fs::remove_dir_all(&path) {
                Ok(()) => tracing::info!(dir = %path.display(), pid, "removed a stale demo dir"),
                Err(e) => {
                    tracing::info!(dir = %path.display(), "cannot remove a stale demo dir: {e}")
                }
            }
        }
    }
}

use spela_core::backend::{Backends, Cpu, Http, Privilege, SysfsCpu};
use spela_core::dll::manifest::RefreshPolicy;
use spela_core::fake::{FakeGpuControl, FakePrivilege};
use spela_core::paths::Paths;

/// A demo run's private directories. Dropping it removes them.
#[derive(Debug)]
pub struct DemoSession {
    dir: tempfile::TempDir,
    paths: Paths,
}

impl DemoSession {
    /// Makes the temp dir and copies the user's config into it, if there is
    /// one.
    pub fn start(real: &Paths) -> io::Result<DemoSession> {
        let dir = tempfile::Builder::new().prefix(PREFIX).tempdir()?;
        std::fs::write(dir.path().join(MARKER), std::process::id().to_string())?;
        if let Some(tmp) = dir.path().parent() {
            sweep_stale(tmp, dir.path());
        }
        let paths = Paths::under(dir.path());
        let config = real.config_file();
        if config.is_file() {
            std::fs::create_dir_all(&paths.config_dir)?;
            std::fs::copy(&config, paths.config_file())?;
            let own_folders = spela_core::config::load(&paths.config_file()).is_ok_and(|l| {
                !l.config.dll_cache_path.is_empty() || !l.config.backup_path.is_empty()
            });
            if own_folders {
                spela_core::config::store::set_many(
                    &paths,
                    &[("dll_cache_path", ""), ("backup_path", "")],
                )
                .map_err(io::Error::other)?;
            }
        }
        spela_core::fake::library::write(&paths.cache_dir.join("demo-steam"))?;
        spela_core::fake::cpu::write_sysfs(&sys_dir(&paths))?;
        *live() = Some(dir.path().to_owned());
        Ok(DemoSession { dir, paths })
    }

    /// The demo's directories.
    pub fn paths(&self) -> &Paths {
        &self.paths
    }

    /// The temp dir everything lives under.
    pub fn root(&self) -> &std::path::Path {
        self.dir.path()
    }

    /// Scans the fictional library into the demo's game cache, so a CLI
    /// command that never scans, such as `dll list`, still sees the demo
    /// games. The TUI scans on its own at startup, so it does not call this.
    pub fn scan(&self) -> Result<(), String> {
        let install = spela_core::fake::library::install(&library_dir(&self.paths));
        spela_core::games::scan::rescan(&self.paths, &install, &spela_core::clock::SystemClock)
            .map(|_| ())
            .map_err(|e| e.to_string())
    }
}

impl Drop for DemoSession {
    /// Forgets the dir for [`remove_now`]; the temp dir removes itself.
    fn drop(&mut self) {
        let mut live = live();
        if live.as_deref() == Some(self.dir.path()) {
            *live = None;
        }
    }
}

/// The demo's fictional Steam library under its paths.
pub fn library_dir(paths: &Paths) -> PathBuf {
    paths.cache_dir.join("demo-steam").join("SteamLibrary")
}

/// The demo's fictional sysfs tree under its paths.
pub fn sys_dir(paths: &Paths) -> PathBuf {
    paths.cache_dir.join("demo-sys")
}

/// Points `backends` at the demo: its network, and a CPU reader and
/// privilege runner over the fictional sysfs tree, so a governor or SMT
/// change shows at the next read and nothing on the machine changes.
pub fn use_demo_backends(backends: &mut Backends, paths: &Paths) {
    let (cpu, privilege) = hardware(&sys_dir(paths));
    backends.cpu = cpu;
    backends.privilege = privilege;
    backends.http = http(paths);
}

/// The CPU reader and privilege runner over the sysfs tree at `root`: the
/// real sysfs reader, and the fake runner that writes the tree.
fn hardware(root: &Path) -> (Box<dyn Cpu>, Box<dyn Privilege>) {
    (
        Box::new(SysfsCpu::new(root)),
        Box::new(FakePrivilege::new(
            FakeGpuControl::new(),
            Some(root.to_owned()),
        )),
    )
}

/// The demo's network: the demo manifest at the URL the copied config
/// resolves to, as well as the default one, and its payloads.
pub fn http(paths: &Paths) -> Arc<dyn Http> {
    let config = spela_core::config::load(&paths.config_file())
        .map(|l| l.config)
        .unwrap_or_default();
    let url = RefreshPolicy::from_config(&config).url;
    Arc::new(spela_core::fake::dlls::http(&url))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn copies_only_the_config_and_cleans_up() {
        let base = tempfile::tempdir().unwrap();
        let real = Paths::under(base.path());
        std::fs::create_dir_all(&real.config_dir).unwrap();
        std::fs::write(real.config_file(), "[ui]\ntheme = \"mono\"\n").unwrap();
        std::fs::write(real.config_dir.join("config.yaml"), "go: true\n").unwrap();

        let session = DemoSession::start(&real).unwrap();
        let root = session.root().to_owned();
        assert_eq!(
            std::fs::read_to_string(session.paths().config_file()).unwrap(),
            "[ui]\ntheme = \"mono\"\n"
        );
        assert!(!session.paths().config_dir.join("config.yaml").exists());
        for dir in session.paths().bases() {
            assert!(dir.starts_with(&root));
        }
        drop(session);
        assert!(!root.exists());
    }

    #[test]
    fn the_copy_never_points_at_the_user_s_dll_cache_or_backups() {
        let base = tempfile::tempdir().unwrap();
        let real = Paths::under(base.path());
        std::fs::create_dir_all(&real.config_dir).unwrap();
        let text = "dll_cache_path = \"/home/x/dlls\"\nbackup_path = \"/home/x/backups\"\n\n[ui]\ntheme = \"neon\"\n";
        std::fs::write(real.config_file(), text).unwrap();
        let session = DemoSession::start(&real).unwrap();
        let config = spela_core::config::load(&session.paths().config_file())
            .unwrap()
            .config;
        assert_eq!(config.dll_cache_path, "");
        assert_eq!(config.backup_path, "");
        assert_eq!(config.ui.theme, spela_core::config::Theme::Neon);
        assert!(
            config
                .dll_cache_dir(session.paths())
                .starts_with(session.root())
        );
        assert_eq!(std::fs::read_to_string(real.config_file()).unwrap(), text);
    }

    #[test]
    fn no_config_is_fine() {
        let base = tempfile::tempdir().unwrap();
        let session = DemoSession::start(&Paths::under(base.path())).unwrap();
        assert!(!session.paths().config_file().exists());
    }

    #[test]
    fn the_session_writes_the_library_and_the_hardware_changes_only_its_tree() {
        use spela_core::backend::HardwareRequest;
        let base = tempfile::tempdir().unwrap();
        let session = DemoSession::start(&Paths::under(base.path())).unwrap();
        let library = library_dir(session.paths());
        assert!(library.starts_with(session.root()));
        assert!(library.join("steamapps").is_dir(), "{}", library.display());

        let (mut cpu, mut privilege) = hardware(&sys_dir(session.paths()));
        assert_eq!(cpu.governor().unwrap(), "powersave");
        let report = privilege
            .apply_once(&HardwareRequest {
                cpu_governor: Some("performance".into()),
                ..HardwareRequest::default()
            })
            .unwrap();
        assert!(report.all_applied());
        assert_eq!(cpu.governor().unwrap(), "performance");
        let report = privilege
            .apply_once(&HardwareRequest {
                smt: Some(false),
                ..HardwareRequest::default()
            })
            .unwrap();
        assert!(report.all_applied());
        assert!(!cpu.smt_active().unwrap());
    }
}
