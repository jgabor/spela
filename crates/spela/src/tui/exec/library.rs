//! The background scan (decision 24) and the Library list's commands.
//!
//! The scan runs on the job pool, reads the config fresh for the Steam
//! paths (or the demo library), replaces the game cache under
//! `games.lock` and answers with the new cache and the games that have
//! their own profile.

use spela_core::games::cache;
use spela_core::games::scan::{self, RescanError};
use spela_core::profile::store;
use spela_ui::Msg;
use spela_ui::screens::library::Cmd;

use super::Workers;

/// Starts a scan on the pool.
pub fn scan(w: &mut Workers, request: u64) {
    let paths = w.paths.clone();
    let clock = w.clock.clone();
    let steam = w.steam.clone();
    w.jobs.submit(
        &w.tx,
        move || {
            let install = steam.install(&paths);
            let result = match scan::rescan(&paths, &install, clock.as_ref()) {
                Ok(_) => Ok(cache::load(&paths)),
                Err(RescanError::NoSteam) => Err(scan::NO_STEAM.to_owned()),
                Err(e) => Err(e.to_string()),
            };
            let profiles = match &result {
                Ok(c) => c
                    .games
                    .iter()
                    .map(|g| g.app_id)
                    .filter(|id| store::has_profile(&paths, *id))
                    .collect(),
                Err(_) => Vec::new(),
            };
            Msg::ScanDone {
                request,
                result,
                profiles,
            }
        },
        move |e| Msg::ScanDone {
            request,
            result: Err(e),
            profiles: Vec::new(),
        },
    );
}

/// Runs a Library list command. It has none yet.
pub fn run(_w: &mut Workers, cmd: Cmd) {
    match cmd {}
}
