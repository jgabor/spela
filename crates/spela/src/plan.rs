//! What the dry run, the wrapper and the TUI's Preview launch share when
//! they build a launch plan: the Proton and driver probes, the deny list's
//! answer, the backup check and where spela runs from. The plan itself is
//! [`spela_core::launch::plan`].

use std::path::Path;

use spela_core::backend::{Gpu, GpuError};
use spela_core::config::Config;
use spela_core::denylist;
use spela_core::dll::backup;
use spela_core::games::Game;
use spela_core::launch::summary::INSTALL_PATH;
use spela_core::launch::{self, DenyState, LaunchPlan, PlanInputs, Probes, ProfileState};
use spela_core::paths::Paths;
use spela_core::proton::{self, Build, ResolveError};
use spela_core::steam::library::SteamInstall;

/// The probes over a Steam install and a driver lookup. Both are asked
/// only when vkd3d_heap resolves on, so a plan without it never looks for
/// Steam or opens NVML.
pub struct SystemProbes<'a> {
    /// Finds the Steam install the Proton build is looked for in.
    pub install: &'a mut dyn FnMut() -> SteamInstall,
    /// The driver version, as [`Probes::driver`] answers it.
    pub driver: &'a mut dyn FnMut() -> Option<String>,
}

impl Probes for SystemProbes<'_> {
    fn proton(&mut self, app_id: u64) -> Result<Build, ResolveError> {
        proton::resolve(&(self.install)(), app_id)
    }

    fn driver(&mut self) -> Option<String> {
        (self.driver)()
    }
}

/// The driver version from the GPU backend: `Some("")` when NVML is
/// missing, which takes Go's no-driver path, and `None` when the driver
/// could not be asked.
pub fn driver_version(gpu: &mut dyn Gpu) -> Option<String> {
    match gpu.info() {
        Ok(info) => Some(info.driver_version),
        Err(GpuError::Unavailable(why)) => {
            tracing::info!("no NVIDIA driver for the descriptor heap check: {why}");
            Some(String::new())
        }
        Err(e) => {
            tracing::info!("the NVIDIA driver could not be asked: {e}");
            None
        }
    }
}

/// What the deny lists say about a game. Lists that do not load are
/// reported, not guessed.
pub fn deny_state(paths: &Paths, app_id: u64) -> DenyState {
    match denylist::load(paths) {
        Ok(lists) => {
            let status = lists.check(app_id);
            match status.reason() {
                Some(reason) if status.is_denied() => DenyState::Denied(reason.to_owned()),
                _ => DenyState::Allowed,
            }
        }
        Err(e) => {
            tracing::warn!("cannot read the deny lists: {e}");
            DenyState::Unknown(e.to_string())
        }
    }
}

/// Whether this process runs from `/usr/bin/spela`, where the polkit
/// policy expects it.
pub fn at_install_path() -> bool {
    spela_core::env::current_exe().is_some_and(|p| p == Path::new(INSTALL_PATH))
}

/// Builds the plan for one game.
pub fn build(
    paths: &Paths,
    config: &Config,
    game: &Game,
    profile: &ProfileState,
    probes: &mut dyn Probes,
) -> LaunchPlan {
    let shader_cache_dir = config.shader_cache_dir(paths);
    let inputs = PlanInputs {
        game,
        profile,
        shader_cache_dir: &shader_cache_dir,
        deny: deny_state(paths, game.app_id),
        backup: backup::has_backup(paths, game.app_id),
        at_install_path: at_install_path(),
    };
    launch::plan(&inputs, probes)
}
