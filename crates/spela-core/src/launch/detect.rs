//! Which game a wrapped command runs: Go's `DetectGameFromCommand`, fixed
//! (architecture, "Game detection").
//!
//! In order:
//!
//! 1. `SteamAppId`, then `SteamGameId`, from the environment, trimmed.
//! 2. An argument `AppId=<id>` or `SteamAppId=<id>`, as Steam's reaper
//!    passes it.
//! 3. An argument that is a game's install folder or sits under it,
//!    compared by path, not by substring. Empty folders and Steam tools such
//!    as Proton never match. The longest folder wins, and a tie between two
//!    games means no game. Go took any substring, so `common/Portal`
//!    matched `common/Portal 2`, and an empty folder matched everything.
//!
//! With a game cache, steps 1 and 2 count only an AppID the cache knows.
//! Without one (decision 49), an AppID is taken as it is, and step 3 reads
//! the app manifests of the library the path names, in place, with no scan.
//! The wrapper also takes the second way when the cache is not empty but
//! does not know the game, so a game newer than the last scan still gets
//! its profile.

use std::ffi::OsString;
use std::path::{Component, Path, PathBuf};

use crate::env::{Env, var};
use crate::games::{Game, GameCache};
use crate::steam::library::SteamInstall;
use crate::steam::manifest::{self, AppManifest};
use crate::steam::tools;

/// The AppIDs steps 1 and 2 find, in the order they are tried.
fn app_ids(env: &Env, args: &[OsString]) -> Vec<u64> {
    let mut ids = Vec::new();
    for name in [var::STEAM_APP_ID, var::STEAM_GAME_ID] {
        if let Some(id) = env.non_empty(name).and_then(|v| v.parse().ok()) {
            ids.push(id);
        }
    }
    for arg in args.iter().filter_map(|a| a.to_str()) {
        let value = arg
            .strip_prefix("AppId=")
            .or_else(|| arg.strip_prefix("SteamAppId="));
        if let Some(id) = value.and_then(|v| v.trim().parse().ok()) {
            ids.push(id);
        }
    }
    ids
}

/// The game in the cache, or `None` when nothing matches.
pub fn from_cache<'a>(cache: &'a GameCache, env: &Env, args: &[OsString]) -> Option<&'a Game> {
    if let Some(game) = app_ids(env, args).into_iter().find_map(|id| cache.get(id)) {
        return Some(game);
    }
    let candidates = cache
        .games
        .iter()
        .filter(|g| !g.install_dir.as_os_str().is_empty())
        .filter(|g| !tools::is_tool(g.app_id, &g.name, &g.install_dir))
        .map(|g| (g, g.install_dir.as_path()));
    longest_match(candidates, args)
}

/// Picks the game whose folder holds an argument, the longest folder
/// winning. A tie between two games is no game.
fn longest_match<'a, T>(
    candidates: impl Iterator<Item = (&'a T, &'a Path)>,
    args: &[OsString],
) -> Option<&'a T> {
    let mut best: Option<(&T, usize)> = None;
    let mut tied = false;
    for (item, folder) in candidates {
        let depth = folder.components().count();
        let hit = args
            .iter()
            .map(Path::new)
            .any(|arg| arg.is_absolute() && arg.starts_with(folder));
        if !hit {
            continue;
        }
        match best {
            Some((current, d)) if depth == d && !std::ptr::eq(current, item) => tied = true,
            Some((_, d)) if depth <= d => {}
            _ => {
                best = Some((item, depth));
                tied = false;
            }
        }
    }
    if tied {
        None
    } else {
        best.map(|(item, _)| item)
    }
}

/// The game without a cache (decision 49): an AppID from steps 1 and 2 as
/// it is, else the install folder a path argument names, from that
/// library's app manifests. The Steam libraries give the name of an AppID
/// game when one has its manifest.
pub fn without_cache(install: &SteamInstall, env: &Env, args: &[OsString]) -> Option<Game> {
    if let Some(id) = app_ids(env, args).into_iter().next() {
        let manifest = install.library_paths().find_map(|library| {
            manifest::read(&manifest::manifest_path(library, id))
                .ok()
                .flatten()
        });
        return Some(match manifest {
            Some(m) => game_from(&m),
            None => Game {
                app_id: id,
                name: id.to_string(),
                install_dir: PathBuf::new(),
                library_path: PathBuf::new(),
                prefix_path: None,
                scanned_at: jiff::Timestamp::UNIX_EPOCH,
                dlls: Vec::new(),
            },
        });
    }
    let mut manifests: Vec<(AppManifest, PathBuf)> = Vec::new();
    for library in args.iter().filter_map(|a| library_of(Path::new(a))) {
        if manifests.iter().any(|(m, _)| m.library_path == library) {
            continue;
        }
        for path in manifest::list_manifests(&library).unwrap_or_default() {
            if let Ok(Some(m)) = manifest::read(&path)
                && let Some(dir) = m.full_install_dir()
                && !tools::is_tool(m.app_id, &m.name, &dir)
            {
                manifests.push((m, dir));
            }
        }
    }
    let candidates = manifests.iter().map(|(m, dir)| (m, dir.as_path()));
    longest_match(candidates, args).map(game_from)
}

/// The library a path under `<library>/steamapps/common/<folder>` sits in.
fn library_of(path: &Path) -> Option<PathBuf> {
    if !path.is_absolute() {
        return None;
    }
    let parts: Vec<Component<'_>> = path.components().collect();
    let at = parts.windows(3).position(|w| {
        w[0].as_os_str() == "steamapps"
            && w[1].as_os_str() == "common"
            && matches!(w[2], Component::Normal(_))
    })?;
    Some(parts[..at].iter().collect())
}

fn game_from(m: &AppManifest) -> Game {
    Game {
        app_id: m.app_id,
        name: m.name.clone(),
        install_dir: m.full_install_dir().unwrap_or_default(),
        library_path: m.library_path.clone(),
        prefix_path: None,
        scanned_at: jiff::Timestamp::UNIX_EPOCH,
        dlls: Vec::new(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn game(app_id: u64, name: &str, dir: &str) -> Game {
        Game {
            app_id,
            name: name.into(),
            install_dir: dir.into(),
            library_path: "/lib".into(),
            prefix_path: None,
            scanned_at: jiff::Timestamp::UNIX_EPOCH,
            dlls: Vec::new(),
        }
    }

    fn cache() -> GameCache {
        let mut c = GameCache::default();
        for g in [
            game(9100200, "Portal Run", "/lib/steamapps/common/Portal"),
            game(9100201, "Portal Run 2", "/lib/steamapps/common/Portal 2"),
            game(9100202, "Empty Folder", ""),
            game(
                1493710,
                "Proton Experimental",
                "/lib/steamapps/common/Proton - Experimental",
            ),
        ] {
            c.upsert(g);
        }
        c
    }

    fn os(args: &[&str]) -> Vec<OsString> {
        args.iter().map(OsString::from).collect()
    }

    fn found(env: &Env, args: &[&str]) -> Option<u64> {
        let c = cache();
        from_cache(&c, env, &os(args)).map(|g| g.app_id)
    }

    #[test]
    fn environment_first_then_arguments() {
        let env = Env::from_pairs([(var::STEAM_APP_ID, " 9100201 ")]);
        assert_eq!(
            found(&env, &["/lib/steamapps/common/Portal/run"]),
            Some(9100201)
        );
        let env = Env::from_pairs([(var::STEAM_APP_ID, "42"), (var::STEAM_GAME_ID, "9100200")]);
        assert_eq!(found(&env, &[]), Some(9100200), "an unknown id is skipped");
        let none = Env::default();
        assert_eq!(
            found(&none, &["reaper", "AppId=9100201", "--"]),
            Some(9100201)
        );
        assert_eq!(found(&none, &["SteamAppId=9100200"]), Some(9100200));
        assert_eq!(found(&none, &["AppId=nope"]), None);
    }

    #[test]
    fn install_folders_match_by_path_not_substring() {
        let none = Env::default();
        assert_eq!(
            found(&none, &["/lib/steamapps/common/Portal 2/portal2.exe"]),
            Some(9100201),
            "Go's substring rule matched Portal here"
        );
        assert_eq!(
            found(&none, &["/lib/steamapps/common/Portal"]),
            Some(9100200)
        );
        assert_eq!(found(&none, &["/lib/steamapps/common/Portally/x"]), None);
        assert_eq!(found(&none, &["relative/steamapps/common/Portal/x"]), None);
        // The empty folder and the Proton tool never match.
        assert_eq!(
            found(
                &none,
                &[
                    "/lib/steamapps/common/Proton - Experimental/proton",
                    "waitforexitandrun",
                    "/lib/steamapps/common/Portal/hl2.exe"
                ]
            ),
            Some(9100200)
        );
        assert_eq!(found(&none, &["/somewhere/else"]), None);
    }

    #[test]
    fn the_longest_folder_wins_and_a_tie_is_no_game() {
        let mut c = cache();
        c.upsert(game(
            9100203,
            "Portal Mods",
            "/lib/steamapps/common/Portal/mods",
        ));
        let args = os(&["/lib/steamapps/common/Portal/mods/x.exe"]);
        assert_eq!(
            from_cache(&c, &Env::default(), &args).map(|g| g.app_id),
            Some(9100203)
        );
        c.upsert(game(
            9100204,
            "Portal Mods Twin",
            "/lib/steamapps/common/Portal/mods",
        ));
        assert_eq!(from_cache(&c, &Env::default(), &args), None);
    }

    fn library() -> (tempfile::TempDir, SteamInstall) {
        let dir = tempfile::tempdir().unwrap();
        let steamapps = dir.path().join("steamapps");
        std::fs::create_dir_all(steamapps.join("common/Quiet Orchard")).unwrap();
        std::fs::write(
            steamapps.join("appmanifest_9100030.acf"),
            "\"AppState\"\n{\n\t\"appid\"\t\t\"9100030\"\n\t\"name\"\t\t\"Quiet Orchard\"\n\t\"StateFlags\"\t\t\"4\"\n\t\"installdir\"\t\t\"Quiet Orchard\"\n}\n",
        )
        .unwrap();
        let install = SteamInstall {
            root: None,
            libraries: vec![crate::steam::library::Library {
                path: dir.path().to_owned(),
                label: "test".into(),
            }],
            problems: Vec::new(),
        };
        (dir, install)
    }

    #[test]
    fn without_a_cache_ids_are_taken_as_they_are() {
        let (_dir, install) = library();
        let env = Env::from_pairs([(var::STEAM_APP_ID, "9100030")]);
        let g = without_cache(&install, &env, &[]).unwrap();
        assert_eq!((g.app_id, g.name.as_str()), (9100030, "Quiet Orchard"));
        let g = without_cache(&install, &Env::default(), &os(&["AppId=777"])).unwrap();
        assert_eq!((g.app_id, g.name.as_str()), (777, "777"));
    }

    #[test]
    fn without_a_cache_paths_read_that_library_s_manifests() {
        let (dir, _) = library();
        let exe = dir
            .path()
            .join("steamapps/common/Quiet Orchard/orchard.exe");
        let args = vec![OsString::from("waitforexitandrun"), exe.into_os_string()];
        let g = without_cache(&SteamInstall::default(), &Env::default(), &args).unwrap();
        assert_eq!(g.app_id, 9100030);
        assert_eq!(
            g.install_dir,
            dir.path().join("steamapps/common/Quiet Orchard")
        );
        let other = os(&["/nowhere/steamapps/common/Quiet Orchard/x.exe"]);
        assert!(without_cache(&SteamInstall::default(), &Env::default(), &other).is_none());
        assert_eq!(
            library_of(Path::new("/a/b/steamapps/common/G/x")),
            Some(PathBuf::from("/a/b"))
        );
        assert_eq!(library_of(Path::new("/a/steamapps/common")), None);
    }
}
