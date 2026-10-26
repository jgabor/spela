//! The profile page's commands, run on the job pool, never on the UI
//! thread: reading every profile, the VKD3D notice (the game's Proton
//! build and the NVIDIA driver) and the governors sysfs lists. Results go
//! back as `Msg::Profile`. Saves go through the write queue as
//! `Cmd::Save`, like every other screen's.

use std::sync::{Arc, Mutex, PoisonError};
use std::time::Duration;

use spela_core::backend::{Cpu, CpuError};
use spela_core::paths::Paths;
use spela_core::profile::{Scope, store};
use spela_core::proton;
use spela_ui::Msg;
use spela_ui::screens::profile::{Cmd, Loaded, LoadedGame, Msg as ProfileMsg};

use super::Workers;
use crate::tui::metrics::DriverSlot;

/// How long a notice job waits for the metrics thread's driver read.
const DRIVER_WAIT: Duration = Duration::from_secs(5);

/// This run's CPU reader, shared by the jobs that read it.
#[derive(Clone)]
pub struct CpuReader(Arc<Mutex<Box<dyn Cpu>>>);

impl CpuReader {
    /// Shares `cpu` with the jobs.
    pub fn new(cpu: Box<dyn Cpu>) -> CpuReader {
        CpuReader(Arc::new(Mutex::new(cpu)))
    }

    /// The governors the system lists.
    pub fn available_governors(&self) -> Result<Vec<String>, CpuError> {
        self.0
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .available_governors()
    }
}

impl std::fmt::Debug for CpuReader {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("CpuReader")
    }
}

/// The CPU reader left behind when the workers take this run's reader.
#[derive(Debug)]
pub struct NoCpu;

impl NoCpu {
    fn error() -> CpuError {
        CpuError {
            path: String::new(),
            reason: "the TUI's workers own the CPU reader".into(),
        }
    }
}

impl Cpu for NoCpu {
    fn governor(&mut self) -> Result<String, CpuError> {
        Err(Self::error())
    }

    fn available_governors(&mut self) -> Result<Vec<String>, CpuError> {
        Err(Self::error())
    }

    fn smt_active(&mut self) -> Result<bool, CpuError> {
        Err(Self::error())
    }
}

/// Runs one of the page's commands on the pool.
pub fn run(w: &mut Workers, cmd: Cmd) {
    match cmd {
        Cmd::Load { request } => {
            let paths = w.paths.clone();
            w.jobs.submit(
                &w.tx,
                move || {
                    Msg::Profile(ProfileMsg::Loaded {
                        request,
                        result: load(&paths),
                    })
                },
                move |e| {
                    Msg::Profile(ProfileMsg::Loaded {
                        request,
                        result: Err(e),
                    })
                },
            );
        }
        Cmd::Notice { request, app_id } => {
            let paths = w.paths.clone();
            let steam = w.steam.clone();
            let driver = w.driver.clone();
            w.jobs.submit(
                &w.tx,
                move || {
                    let install = steam.install(&paths);
                    let build = proton::resolve(&install, app_id);
                    let driver = wait_driver(&driver);
                    Msg::Profile(ProfileMsg::Notice {
                        request,
                        app_id,
                        notice: proton::check(&build, driver.as_deref()).notice(),
                    })
                },
                move |_| {
                    Msg::Profile(ProfileMsg::Notice {
                        request,
                        app_id,
                        notice: None,
                    })
                },
            );
        }
        Cmd::Governors { request } => {
            let cpu = w.cpu.clone();
            w.jobs.submit(
                &w.tx,
                move || {
                    let result = cpu.available_governors().map_err(|e| e.to_string());
                    Msg::Profile(ProfileMsg::Governors { request, result })
                },
                move |e| {
                    Msg::Profile(ProfileMsg::Governors {
                        request,
                        result: Err(e),
                    })
                },
            );
        }
    }
}

/// The driver version the metrics thread read, waiting for it a little. A
/// thread that never answers counts as a driver that could not be asked.
fn wait_driver(slot: &DriverSlot) -> Option<String> {
    let step = Duration::from_millis(50);
    let mut waited = Duration::ZERO;
    loop {
        if let Some(driver) = slot.get() {
            return driver.clone();
        }
        if waited >= DRIVER_WAIT {
            return None;
        }
        std::thread::sleep(step);
        waited += step;
    }
}

/// Reads every profile. A file that does not load is reported for its
/// scope; only an unreadable profile folder fails the whole load.
pub fn load(paths: &Paths) -> Result<Loaded, String> {
    let mut loaded = Loaded::default();
    match store::load_all_games(paths) {
        Ok(Some(l)) => loaded.all_games = Some((l.profile.values, l.read_only)),
        Ok(None) => {}
        Err(e) => loaded.broken.push((Scope::AllGames, e.to_string())),
    }
    let listing = store::list(paths).map_err(|e| e.to_string())?;
    for l in listing.profiles {
        loaded.games.push(LoadedGame {
            app_id: l.profile.app_id,
            values: l.profile.values,
            read_only: l.read_only,
        });
    }
    for (app_id, e) in listing.broken {
        loaded.broken.push((Scope::Game(app_id), e.to_string()));
    }
    Ok(loaded)
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::profile::{FieldKey, Value};
    use std::sync::OnceLock;

    #[test]
    fn the_load_reads_every_profile_and_names_broken_ones() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        store::set(
            &paths,
            Scope::AllGames,
            "",
            &[(FieldKey::ProtonEnableHdr, Value::Bool(true))],
        )
        .unwrap();
        store::set(
            &paths,
            Scope::Game(9100010),
            "Ashfall Courier",
            &[(FieldKey::GpuPowerLimit, Value::Int(250))],
        )
        .unwrap();
        std::fs::write(paths.game_profile(9100020), "version = \"x\"\n").unwrap();
        let loaded = load(&paths).unwrap();
        let (all, read_only) = loaded.all_games.unwrap();
        assert_eq!(all.get(FieldKey::ProtonEnableHdr), Some(&Value::Bool(true)));
        assert!(!read_only);
        assert_eq!(loaded.games.len(), 1);
        assert_eq!(loaded.games[0].app_id, 9100010);
        assert_eq!(loaded.broken.len(), 1);
        assert_eq!(loaded.broken[0].0, Scope::Game(9100020));
        // No profile folder is an empty load, not an error.
        let empty = tempfile::tempdir().unwrap();
        let none = load(&Paths::under(empty.path())).unwrap();
        assert!(none.all_games.is_none() && none.games.is_empty());
    }

    #[test]
    fn the_driver_wait_takes_what_the_metrics_thread_read() {
        let slot: DriverSlot = Arc::new(OnceLock::new());
        slot.set(Some("580.95.05".into())).unwrap();
        assert_eq!(wait_driver(&slot).as_deref(), Some("580.95.05"));
        assert!(NoCpu.available_governors().is_err());
    }
}
