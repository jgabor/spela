//! Running the UI's commands: one file per area, so the screens' owners
//! each fill their own.
//!
//! | File | Commands |
//! |---|---|
//! | this file | the shell's: metrics polls, saves, and the DLL job's stop flag |
//! | [`monitor`] | also `Cmd::Hardware`, the hardware jobs |
//! | [`library`] | `Cmd::Scan` and `library::Cmd` |
//! | [`profile`] | `profile::Cmd` |
//! | [`settings`] | `settings::Cmd` |
//! | [`monitor`] | `monitor::Cmd` |
//! | [`preview`] | `Cmd::Preview`, the dry-run summary |
//! | [`dlls`] | `dlls::Cmd`, and also `Cmd::Dll` and `Cmd::CheckRelease` |
//! | [`denylist`] | `denylist::Cmd` |
//!
//! A command never blocks the UI thread: polls go to the metrics thread,
//! writes to the write queue, and everything else to the job pool, which
//! answers through the loop channel.

pub mod denylist;
pub mod dlls;
pub mod library;
pub mod monitor;
pub mod preview;
pub mod profile;
pub mod settings;

use std::path::PathBuf;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use spela_core::backend::Http;
use spela_core::clock::Clock;
use spela_core::paths::Paths;
use spela_core::steam::library::SteamInstall;
use spela_ui::{Cmd, Msg};

use super::events::LoopSender;
use super::jobs::Pool;
use super::metrics::MetricsThread;
use super::writes::WriteSender;

/// What runs the UI's commands. The loop's tests use a recording one.
pub trait Exec {
    /// Runs one command. `Suspend`, `Quit` and `SetMouse` never reach it:
    /// the loop handles them.
    fn run(&mut self, cmd: Cmd);
}

/// Where the scan looks.
#[derive(Debug, Clone)]
pub enum SteamSource {
    /// The config's `steam_path` and `additional_library_paths`, and
    /// `HOME`, read fresh at each scan.
    System {
        /// `HOME`.
        home: Option<std::path::PathBuf>,
    },
    /// The demo's fictional library.
    Demo(std::path::PathBuf),
}

impl SteamSource {
    /// The Steam install to scan now.
    pub fn install(&self, paths: &Paths) -> SteamInstall {
        match self {
            SteamSource::Demo(library) => spela_core::fake::library::install(library),
            SteamSource::System { home } => {
                let config = spela_core::config::load(&paths.config_file())
                    .map(|l| l.config)
                    .unwrap_or_default();
                SteamInstall::discover(
                    &config.steam_path,
                    &config.additional_library_paths,
                    home.as_deref(),
                )
            }
        }
    }
}

/// The real workers.
pub struct Workers {
    /// spela's directories.
    pub paths: Paths,
    /// The loop channel, for job results.
    pub tx: LoopSender<Msg>,
    /// The metrics thread.
    pub metrics: MetricsThread,
    /// The job pool.
    pub jobs: Pool,
    /// The write queue.
    pub writes: WriteSender,
    /// The clock.
    pub clock: Arc<dyn Clock>,
    /// Where scans look.
    pub steam: SteamSource,
    /// The CPU reader of this run's backends (the fake under `--demo` and
    /// `SPELA_BACKEND=fake`), shared with the metrics thread and jobs: the
    /// governor picker reads the governors sysfs lists.
    pub cpu: profile::CpuReader,
    /// The NVIDIA driver version, for the VKD3D notice.
    pub driver: super::metrics::DriverSlot,
    /// The privilege backend, for the hardware jobs.
    pub privilege: monitor::SharedPrivilege,
    /// Sets the log file's level after `log_level` was saved.
    pub log: crate::logging::LevelHandle,
    /// The network: ureq, or the fake (the demo's under `--demo`).
    pub http: Arc<dyn Http>,
    /// Set when spela quits during a DLL job, which then stops at its next
    /// safe point.
    pub stop: Arc<AtomicBool>,
    /// Where the auto-update looks for running games: `/proc`, or `None`
    /// under the fake backend, where no game runs.
    pub proc_root: Option<PathBuf>,
    /// `--demo`: the release check asks the demo's fake as a release.
    pub demo: bool,
}

impl std::fmt::Debug for Workers {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Workers")
            .field("paths", &self.paths)
            .field("steam", &self.steam)
            .field("proc_root", &self.proc_root)
            .field("demo", &self.demo)
            .finish_non_exhaustive()
    }
}

impl Exec for Workers {
    fn run(&mut self, cmd: Cmd) {
        match cmd {
            Cmd::PollMetrics => self.metrics.poll(),
            Cmd::Scan { request } => library::scan(self, request),
            Cmd::Save { token, write } => self.writes.submit(token, write),
            Cmd::CheckRelease => dlls::check_release(self),
            Cmd::Dll { token, op } => dlls::job(self, token, op),
            Cmd::StopDll => self.stop.store(true, Ordering::SeqCst),
            Cmd::Hardware { token, op } => monitor::hardware(self, token, op),
            Cmd::Preview { request, app_id } => preview::run(self, request, app_id),
            Cmd::SetLogLevel(level) => self.log.apply(level.name()),
            Cmd::Suspend | Cmd::Quit | Cmd::SetMouse(_) => {}
            Cmd::Library(c) => library::run(self, c),
            Cmd::Profile(c) => profile::run(self, c),
            Cmd::Settings(c) => settings::run(self, c),
            Cmd::Monitor(c) => monitor::run(self, c),
            Cmd::Dlls(c) => dlls::run(self, c),
            Cmd::Denylist(c) => denylist::run(self, c),
        }
    }
}

/// Commands recorded instead of run, for the loop's tests.
#[derive(Debug, Default, Clone)]
pub struct Recorder {
    /// Every command, in order.
    pub cmds: Arc<std::sync::Mutex<Vec<Cmd>>>,
}

impl Exec for Recorder {
    fn run(&mut self, cmd: Cmd) {
        self.cmds
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner)
            .push(cmd);
    }
}
