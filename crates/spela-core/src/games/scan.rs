//! The scan: Steam libraries to games and their DLLs, with Steam tools dropped, written to the game cache.
//!
//! A rescan replaces the whole cache, as Go's `Rescan` does: games that are
//! no longer installed drop out. It runs under `games.lock`, so a DLL swap
//! never interleaves with it (Go's DLL updater waits for the rescan lock
//! too). Each library's app manifests are read in file name order:
//!
//! - an app counts when the installed bit is set in StateFlags, so a game
//!   with an update pending stays (Go dropped it);
//! - Steam tools are dropped ([`crate::steam::tools`]);
//! - the install folder is `<library>/steamapps/common/<installdir>`, and
//!   a manifest whose `installdir` is not one plain folder name is skipped;
//! - the prefix is `compatdata/<appid>` when its `pfx/drive_c` exists;
//! - the DLLs come from [`crate::dll::detect`], with versions from
//!   [`crate::pe`]. A walk error ends that game's walk and keeps what was
//!   found before it, as Go's `Rescan` did.
//!
//! When two libraries hold the same AppID, the first library wins and the
//! second is logged. Go kept whichever its map order gave last.

use std::io;
use std::path::{Path, PathBuf};

use crate::clock::Clock;
use crate::dll::detect;
use crate::paths::Paths;
use crate::pe;
use crate::steam::library::{Library, SteamInstall, proton_prefix};
use crate::steam::manifest::{self, AppManifest};
use crate::steam::tools;
use crate::steam::vdf::VdfFileError;

use super::{CACHE_VERSION, DetectedDll, DllType, Game, GameCache, cache};

/// The text when there is no library to scan, Go's wording.
pub const NO_STEAM: &str = "could not find Steam installation";

/// Something a scan could not read. The scan carries on without it.
#[derive(Debug, thiserror::Error)]
pub enum ScanProblem {
    /// `libraryfolders.vdf` or an app manifest cannot be read or parsed.
    #[error(transparent)]
    File(#[from] VdfFileError),
    /// A library's `steamapps` folder cannot be listed.
    #[error("cannot list the app manifests in {}: {source}", path.display())]
    Library {
        /// The library.
        path: PathBuf,
        /// Why.
        source: io::Error,
    },
}

/// What a scan found in the libraries.
#[derive(Debug, Default)]
pub struct Scan {
    /// The installed games, sorted by app id.
    pub games: Vec<Game>,
    /// What could not be read.
    pub problems: Vec<ScanProblem>,
}

/// A rescan that wrote nothing, because there is no Steam library.
#[derive(Debug, thiserror::Error)]
pub enum RescanError {
    /// No Steam root was found and no extra library is configured, or the
    /// root lists no library. [`SteamInstall::problems`] says why, such as
    /// a broken `libraryfolders.vdf`.
    #[error("{NO_STEAM}")]
    NoSteam,
    /// The cache could not be locked or written.
    #[error("cannot save the game cache: {0}")]
    Io(#[from] io::Error),
}

/// Scans every library of `install`. Reads only; writes nothing. The
/// problems of finding the libraries stay in [`SteamInstall::problems`].
pub fn scan(install: &SteamInstall, clock: &dyn Clock) -> Scan {
    let mut out = Scan::default();
    let mut cache = GameCache::default();
    for library in &install.libraries {
        for game in scan_library(library, clock, &mut out.problems) {
            if let Some(kept) = cache.get(game.app_id) {
                tracing::info!(
                    app_id = game.app_id,
                    "{} is in two libraries; keeping {}",
                    game.name,
                    kept.library_path.display()
                );
                continue;
            }
            cache.upsert(game);
        }
    }
    out.games = cache.games;
    out
}

/// The games in one library.
fn scan_library(
    library: &Library,
    clock: &dyn Clock,
    problems: &mut Vec<ScanProblem>,
) -> Vec<Game> {
    let manifests = match manifest::list_manifests(&library.path) {
        Ok(found) => found,
        Err(source) => {
            tracing::warn!(
                "cannot list the app manifests in {}: {source}",
                library.path.display()
            );
            problems.push(ScanProblem::Library {
                path: library.path.clone(),
                source,
            });
            return Vec::new();
        }
    };
    let mut games = Vec::new();
    for path in manifests {
        match manifest::read(&path) {
            Ok(Some(app)) => {
                if let Some(game) = game_from(library, &app, clock) {
                    games.push(game);
                }
            }
            Ok(None) => {}
            Err(e) => {
                tracing::warn!("skipping an app manifest: {e}");
                problems.push(ScanProblem::File(e));
            }
        }
    }
    games
}

/// The game an installed app is, or `None` for an app that is not
/// installed, is a tool, or has no usable install folder.
fn game_from(library: &Library, app: &AppManifest, clock: &dyn Clock) -> Option<Game> {
    if !app.is_installed() {
        return None;
    }
    let Some(install_dir) = app.full_install_dir() else {
        tracing::warn!(
            app_id = app.app_id,
            "skipping {}: its installdir {:?} is not a folder name",
            app.name,
            app.install_dir
        );
        return None;
    };
    if tools::is_tool(app.app_id, &app.name, &install_dir) {
        return None;
    }
    Some(Game {
        app_id: app.app_id,
        name: app.name.clone(),
        prefix_path: proton_prefix(&library.path, app.app_id),
        dlls: dlls_in(&install_dir),
        install_dir,
        library_path: library.path.clone(),
        scanned_at: clock.now(),
    })
}

/// The known DLLs under a game's install folder, with their PE versions. A
/// walk error, such as a missing install folder, ends the walk there; the
/// game keeps the DLLs found before it, as Go's `Rescan` did, and the error
/// is logged.
fn dlls_in(install_dir: &Path) -> Vec<DetectedDll> {
    match detect::scan_directory(install_dir, pe::version_string) {
        Ok(found) => found,
        Err(e) => {
            tracing::warn!("{e}; keeping the {} DLLs found before it", e.found.len());
            e.found
        }
    }
}

/// Scans Steam and replaces the cache with what it found, under
/// `games.lock`. With no library to scan, the cache is left as it is and
/// the error says `could not find Steam installation`, as in Go.
///
/// The CLI calls this only for `spela scan` or when the cache is empty;
/// `rescan_on_startup` switches only the TUI's background scan
/// (decision 24).
pub fn rescan(
    paths: &Paths,
    install: &SteamInstall,
    clock: &dyn Clock,
) -> Result<Scan, RescanError> {
    if !install.has_libraries() {
        return Err(RescanError::NoSteam);
    }
    let found = cache::update(paths, |cache| {
        let found = scan(install, clock);
        *cache = GameCache {
            version: CACHE_VERSION,
            updated_at: Some(clock.now()),
            games: found.games.clone(),
        };
        found
    })?;
    Ok(found)
}

/// The read-side views of the cache, which drop tool names as Go's `List`
/// and `GamesWithDLSS` do, as a second guard behind the scan.
impl GameCache {
    /// The games to list: every game whose name is not a tool name, sorted
    /// by app id.
    pub fn listed(&self) -> impl Iterator<Item = &Game> {
        self.games.iter().filter(|g| !tools::is_tool_name(&g.name))
    }

    /// The listed games with a DLSS, DLSS-G or DLSS-D DLL.
    pub fn with_dlss(&self) -> impl Iterator<Item = &Game> {
        self.listed().filter(|g| {
            [DllType::Dlss, DllType::Dlssg, DllType::Dlssd]
                .into_iter()
                .any(|t| g.has(t))
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::clock::ManualClock;
    use crate::steam::manifest::{manifest_path, steamapps_dir};

    fn clock() -> ManualClock {
        ManualClock::new("2026-10-26T15:02:10Z".parse().unwrap())
    }

    fn app(lib: &std::path::Path, app_id: u64, name: &str, dir: &str, flags: u32) {
        std::fs::create_dir_all(steamapps_dir(lib).join("common").join(dir)).unwrap();
        let text = format!(
            "\"AppState\"\n{{\n  \"appid\" \"{app_id}\"\n  \"name\" \"{name}\"\n  \"StateFlags\" \"{flags}\"\n  \"installdir\" \"{dir}\"\n}}\n"
        );
        std::fs::write(manifest_path(lib, app_id), text).unwrap();
    }

    fn library(path: &std::path::Path) -> Library {
        Library {
            path: path.to_owned(),
            label: String::new(),
        }
    }

    #[test]
    fn keeps_installed_games_and_drops_tools_and_bad_manifests() {
        let dir = tempfile::tempdir().unwrap();
        let lib = dir.path();
        app(lib, 9100010, "Glimmer Run", "Glimmer Run", 4);
        app(lib, 9100050, "Pending Patch", "Pending Patch", 6);
        app(lib, 9100080, "Half Downloaded", "Half Downloaded", 2);
        app(lib, 9100040, "Proton Fixture 9.0", "Proton Fixture 9.0", 4);
        app(lib, 1628350, "Some Runtime", "SteamLinuxRuntime_sniper", 4);
        app(lib, 9100090, "Escaping", "..", 4);
        std::fs::write(manifest_path(lib, 9100099), "\"AppState\" {").unwrap();
        let install = SteamInstall {
            root: None,
            libraries: vec![library(lib)],
            problems: Vec::new(),
        };
        let found = scan(&install, &clock());
        let ids: Vec<u64> = found.games.iter().map(|g| g.app_id).collect();
        assert_eq!(ids, [9100010, 9100050]);
        assert_eq!(found.problems.len(), 1, "{:?}", found.problems);
        let g = &found.games[0];
        assert_eq!(g.install_dir, lib.join("steamapps/common/Glimmer Run"));
        assert_eq!(g.library_path, lib);
        assert_eq!(g.prefix_path, None);
        assert_eq!(g.scanned_at, clock().now());
    }

    #[test]
    fn the_first_library_wins_a_duplicate_app_id() {
        let dir = tempfile::tempdir().unwrap();
        let (a, b) = (dir.path().join("a"), dir.path().join("b"));
        app(&a, 9100010, "Glimmer Run", "Glimmer Run", 4);
        app(&b, 9100010, "Glimmer Run", "Glimmer Run", 4);
        app(&b, 9100020, "Harbor Lights", "HarborLights", 4);
        let install = SteamInstall {
            root: None,
            libraries: vec![library(&b), library(&a), library(&dir.path().join("none"))],
            problems: Vec::new(),
        };
        let found = scan(&install, &clock());
        assert_eq!(found.games.len(), 2);
        assert_eq!(found.games[0].library_path, b);
        assert!(matches!(found.problems[..], [ScanProblem::Library { .. }]));
    }

    #[test]
    fn rescan_replaces_the_cache_and_no_steam_writes_nothing() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(&base.path().join("xdg"));
        let lib = base.path().join("lib");
        let none = SteamInstall::default();
        let err = rescan(&paths, &none, &clock()).unwrap_err();
        assert_eq!(err.to_string(), NO_STEAM);
        assert!(!paths.game_cache().exists());

        app(&lib, 9100010, "Glimmer Run", "Glimmer Run", 4);
        app(&lib, 9100020, "Harbor Lights", "HarborLights", 4);
        let install = SteamInstall {
            root: None,
            libraries: vec![library(&lib)],
            problems: Vec::new(),
        };
        rescan(&paths, &install, &clock()).unwrap();
        let cached = cache::load(&paths);
        assert_eq!(cached.games.len(), 2);
        assert_eq!(cached.updated_at, Some(clock().now()));

        std::fs::remove_file(manifest_path(&lib, 9100020)).unwrap();
        rescan(&paths, &install, &clock()).unwrap();
        let ids: Vec<u64> = cache::load(&paths).games.iter().map(|g| g.app_id).collect();
        assert_eq!(ids, [9100010], "a game that is gone drops out");
    }

    #[test]
    fn listed_views_drop_tool_names() {
        let mut cache = GameCache::default();
        let game = |app_id, name: &str, dlls: Vec<DllType>| Game {
            app_id,
            name: name.into(),
            install_dir: "/x".into(),
            library_path: "/".into(),
            prefix_path: None,
            scanned_at: clock().now(),
            dlls: dlls
                .into_iter()
                .map(|t| crate::games::DetectedDll {
                    path: "/x/a.dll".into(),
                    name: "a.dll".into(),
                    dll_type: t,
                    version: None,
                })
                .collect(),
        };
        cache.upsert(game(1, "Glimmer Run", vec![DllType::Dlssg]));
        cache.upsert(game(2, "Proton 9.0 (Fixture)", vec![DllType::Dlss]));
        cache.upsert(game(3, "Harbor Lights", vec![DllType::Xess]));
        let listed: Vec<u64> = cache.listed().map(|g| g.app_id).collect();
        assert_eq!(listed, [1, 3]);
        let dlss: Vec<u64> = cache.with_dlss().map(|g| g.app_id).collect();
        assert_eq!(dlss, [1]);
    }
}
