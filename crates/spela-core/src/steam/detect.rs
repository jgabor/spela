//! Which game a wrapped command or a running process belongs to.
//!
//! Wrapper mode (`spela %command%`) finds its game in this order
//! (architecture, game detection):
//!
//! 1. The variables `SteamAppId`, then `SteamGameId`, trimmed.
//! 2. An argument `AppId=<id>` or `SteamAppId=<id>`, the form Steam's
//!    launch wrapper passes.
//! 3. An argument that is a game's install folder or sits under it,
//!    compared by path components. Empty folders are skipped, the longest
//!    folder wins, and a tie means no game.
//!
//! Each step first looks in the game cache. When the cache has no match,
//! because it is missing, corrupt or older than the game, a launch still
//! finds its game (decision 49): an AppID from step 1 or 2 is taken as it
//! is, and step 3 reads the app manifests of the library that the
//! argument's `steamapps/common/<folder>` path names, without a scan.
//!
//! Go matched any argument that contained an install folder as a
//! substring, so an empty folder matched every argument and
//! `common/Portal` matched `common/Portal 2`.
//!
//! [`running_games`] uses the same folder rule on `/proc`, as Go's
//! `RunningGames` does.

use std::collections::BTreeSet;
use std::ffi::{OsStr, OsString};
use std::os::unix::ffi::OsStrExt;
use std::path::{Component, Path, PathBuf};

use crate::env::Env;
use crate::games::{Game, GameCache};

use super::manifest::{self, COMMON, STEAMAPPS};
use super::tools;

/// The variable Steam sets to the game's AppID.
pub const STEAM_APP_ID: &str = crate::env::var::STEAM_APP_ID;
/// The variable Steam sets to the game's ID, which is the AppID for a
/// Steam game.
pub const STEAM_GAME_ID: &str = crate::env::var::STEAM_GAME_ID;

/// What found the game.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Clue {
    /// `SteamAppId` or `SteamGameId`.
    Environment,
    /// An `AppId=<id>` or `SteamAppId=<id>` argument.
    Argument,
    /// An argument inside the game's install folder.
    InstallFolder,
}

/// A detected game.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Detected<'a> {
    /// The AppID.
    pub app_id: u64,
    /// The game in the cache, when it is there.
    pub game: Option<&'a Game>,
    /// The name, from the cache or the app manifest, when known.
    pub name: Option<String>,
    /// What found it.
    pub clue: Clue,
}

/// The game a wrapped command launches, or `None`. `command` is the
/// command after the wrapper's own `KEY=VALUE` tokens.
pub fn detect<'a, S: AsRef<OsStr>>(
    cache: &'a GameCache,
    env: &Env,
    command: &[S],
) -> Option<Detected<'a>> {
    let env_ids = env_app_ids(env);
    let arg_ids = argument_app_ids(command);
    let cached = |app_id: u64, clue: Clue| {
        cache.get(app_id).map(|game| Detected {
            app_id,
            game: Some(game),
            name: Some(game.name.clone()),
            clue,
        })
    };
    let as_is = |app_id: u64, clue: Clue| Detected {
        app_id,
        game: None,
        name: None,
        clue,
    };
    env_ids
        .iter()
        .find_map(|&id| cached(id, Clue::Environment))
        .or_else(|| arg_ids.iter().find_map(|&id| cached(id, Clue::Argument)))
        .or_else(|| {
            let folders = cache
                .games
                .iter()
                .map(|g| (g.app_id, g.install_dir.as_path()));
            let app_id =
                longest_folder_match(folders, command.iter().map(|a| Path::new(a.as_ref())))?;
            cached(app_id, Clue::InstallFolder)
        })
        .or_else(|| env_ids.first().map(|&id| as_is(id, Clue::Environment)))
        .or_else(|| arg_ids.first().map(|&id| as_is(id, Clue::Argument)))
        .or_else(|| detect_from_manifests(command))
}

/// The AppIDs in `SteamAppId` and `SteamGameId`, in that order. A value
/// that is empty, 0 or not a number is skipped.
pub fn env_app_ids(env: &Env) -> Vec<u64> {
    [STEAM_APP_ID, STEAM_GAME_ID]
        .iter()
        .filter_map(|name| env.non_empty(name)?.parse::<u64>().ok())
        .filter(|&id| id != 0)
        .collect()
}

/// The AppIDs in `AppId=<id>` and `SteamAppId=<id>` arguments, in order.
pub fn argument_app_ids<S: AsRef<OsStr>>(command: &[S]) -> Vec<u64> {
    command
        .iter()
        .filter_map(|arg| {
            let arg = arg.as_ref().to_str()?;
            let id = arg
                .strip_prefix("AppId=")
                .or_else(|| arg.strip_prefix("SteamAppId="))?;
            id.parse::<u64>().ok().filter(|&id| id != 0)
        })
        .collect()
}

/// Whether `path` is `folder` or inside it, compared by components.
pub fn is_within(path: &Path, folder: &Path) -> bool {
    !folder.as_os_str().is_empty() && path.starts_with(folder)
}

/// The app whose folder holds one of `paths`. The folder with the most
/// components wins; two different apps tied at the top give `None`.
fn longest_folder_match<'p>(
    folders: impl IntoIterator<Item = (u64, &'p Path)>,
    paths: impl Iterator<Item = &'p Path> + Clone,
) -> Option<u64> {
    let mut best: Option<(usize, u64)> = None;
    let mut tied = false;
    for (app_id, folder) in folders {
        if !paths.clone().any(|p| is_within(p, folder)) {
            continue;
        }
        let depth = folder.components().count();
        match best {
            Some((d, id)) if d > depth || (d == depth && id == app_id) => {}
            Some((d, _)) if d == depth => tied = true,
            _ => {
                best = Some((depth, app_id));
                tied = false;
            }
        }
    }
    best.filter(|_| !tied).map(|(_, id)| id)
}

/// Step 3 without the cache: for each argument under
/// `<library>/steamapps/common/<folder>`, the installed apps of that
/// library whose `installdir` is the folder. Tools such as Proton, whose
/// script is also in the command, are skipped.
fn detect_from_manifests<'a, S: AsRef<OsStr>>(command: &[S]) -> Option<Detected<'a>> {
    let mut candidates: Vec<(u64, PathBuf, String)> = Vec::new();
    for arg in command {
        let Some((library, folder)) = library_and_folder(Path::new(arg.as_ref())) else {
            continue;
        };
        let Ok(manifests) = manifest::list_manifests(&library) else {
            continue;
        };
        for path in manifests {
            let Ok(Some(app)) = manifest::read(&path) else {
                continue;
            };
            if app.install_dir != folder {
                continue;
            }
            let Some(install_dir) = app.full_install_dir() else {
                continue;
            };
            if !tools::is_tool(app.app_id, &app.name, &install_dir)
                && !candidates.iter().any(|(id, _, _)| *id == app.app_id)
            {
                candidates.push((app.app_id, install_dir, app.name));
            }
        }
    }
    let folders = candidates.iter().map(|(id, dir, _)| (*id, dir.as_path()));
    let app_id = longest_folder_match(folders, command.iter().map(|a| Path::new(a.as_ref())))?;
    let (_, _, name) = candidates.into_iter().find(|(id, _, _)| *id == app_id)?;
    Some(Detected {
        app_id,
        game: None,
        name: Some(name),
        clue: Clue::InstallFolder,
    })
}

/// For an absolute path under `<library>/steamapps/common/<folder>`, the
/// library and the folder name.
fn library_and_folder(path: &Path) -> Option<(PathBuf, String)> {
    if !path.is_absolute() {
        return None;
    }
    let components: Vec<Component<'_>> = path.components().collect();
    components.windows(3).enumerate().find_map(|(at, window)| {
        let [
            Component::Normal(steamapps),
            Component::Normal(common),
            Component::Normal(folder),
        ] = window
        else {
            return None;
        };
        if *steamapps != STEAMAPPS || *common != COMMON {
            return None;
        }
        let library: PathBuf = components[..at].iter().collect();
        Some((library, folder.to_str()?.to_owned()))
    })
}

/// The games among `games` that look like they are running, read from
/// `proc_root` (`/proc` outside tests). A game runs when a process other
/// than `own_pid` has the argument `AppId=<id>`, or when its working
/// folder, its executable or one of its arguments is inside the game's
/// install folder or that folder with links resolved. Processes that
/// cannot be read are skipped.
pub fn running_games(proc_root: &Path, own_pid: u32, games: &[Game]) -> BTreeSet<u64> {
    let mut running = BTreeSet::new();
    if games.is_empty() {
        return running;
    }
    let folders: Vec<Vec<PathBuf>> = games
        .iter()
        .map(|g| install_folders(&g.install_dir))
        .collect();
    let Ok(entries) = std::fs::read_dir(proc_root) else {
        return running;
    };
    for entry in entries.filter_map(Result::ok) {
        let Some(pid) = entry
            .file_name()
            .to_str()
            .and_then(|n| n.parse::<u32>().ok())
        else {
            continue;
        };
        if pid == own_pid {
            continue;
        }
        let dir = entry.path();
        let links: Vec<PathBuf> = ["cwd", "exe"]
            .iter()
            .filter_map(|link| std::fs::read_link(dir.join(link)).ok())
            .collect();
        let arguments = command_line(&dir.join("cmdline"));
        for (game, folders) in games.iter().zip(&folders) {
            if running.contains(&game.app_id) {
                continue;
            }
            let launch = format!("AppId={}", game.app_id);
            let launched = arguments
                .iter()
                .any(|a| a.as_os_str() == OsStr::new(&launch));
            let inside = folders.iter().any(|folder| {
                links.iter().any(|p| is_within(p, folder))
                    || arguments.iter().any(|a| is_within(Path::new(a), folder))
            });
            if launched || inside {
                running.insert(game.app_id);
            }
        }
    }
    running
}

/// The install folder as stored and, when links lead to it, the real
/// folder that `/proc` reports.
fn install_folders(install_dir: &Path) -> Vec<PathBuf> {
    if install_dir.as_os_str().is_empty() {
        return Vec::new();
    }
    let folder: PathBuf = install_dir.components().collect();
    let mut folders = vec![folder.clone()];
    if let Ok(real) = std::fs::canonicalize(&folder)
        && real != folder
    {
        folders.push(real);
    }
    folders
}

/// A process's arguments, split on NUL.
fn command_line(path: &Path) -> Vec<OsString> {
    let Ok(bytes) = std::fs::read(path) else {
        return Vec::new();
    };
    let trimmed = bytes.strip_suffix(b"\0").unwrap_or(&bytes);
    if trimmed.is_empty() {
        return Vec::new();
    }
    trimmed
        .split(|&b| b == 0)
        .map(|arg| OsStr::from_bytes(arg).to_owned())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn game(app_id: u64, name: &str, install_dir: &str) -> Game {
        Game {
            app_id,
            name: name.into(),
            install_dir: install_dir.into(),
            library_path: "/lib".into(),
            prefix_path: None,
            scanned_at: "2026-10-26T15:02:10Z".parse().unwrap(),
            dlls: Vec::new(),
        }
    }

    fn cache(games: Vec<Game>) -> GameCache {
        let mut cache = GameCache::default();
        for g in games {
            cache.upsert(g);
        }
        cache
    }

    #[test]
    fn the_environment_comes_first_then_arguments_then_folders() {
        let c = cache(vec![
            game(9100010, "Glimmer Run", "/lib/steamapps/common/Glimmer Run"),
            game(
                9100020,
                "Harbor Lights",
                "/lib/steamapps/common/HarborLights",
            ),
            game(
                9100030,
                "Quiet Orchard",
                "/lib/steamapps/common/Quiet Orchard",
            ),
        ]);
        let cmd = [
            "AppId=9100020",
            "--",
            "/lib/steamapps/common/Quiet Orchard/orchard.exe",
        ];
        let env = Env::from_pairs([(STEAM_APP_ID, " 9100010 ")]);
        let d = detect(&c, &env, &cmd).unwrap();
        assert_eq!((d.app_id, d.clue), (9100010, Clue::Environment));
        assert_eq!(d.game.unwrap().name, "Glimmer Run");

        let d = detect(&c, &Env::default(), &cmd).unwrap();
        assert_eq!((d.app_id, d.clue), (9100020, Clue::Argument));

        let d = detect(&c, &Env::default(), &cmd[1..]).unwrap();
        assert_eq!((d.app_id, d.clue), (9100030, Clue::InstallFolder));

        // SteamGameId when SteamAppId is unusable; a bad value is skipped.
        let env = Env::from_pairs([(STEAM_APP_ID, "zero"), (STEAM_GAME_ID, "9100020")]);
        assert_eq!(detect(&c, &env, &cmd[1..]).unwrap().app_id, 9100020);
        assert_eq!(
            argument_app_ids(&["SteamAppId=7", "AppId=0", "AppId=x"]),
            [7]
        );
    }

    #[test]
    fn folders_match_by_component_and_the_longest_wins() {
        let c = cache(vec![
            game(1, "Portal", "/lib/steamapps/common/Portal"),
            game(2, "Portal 2", "/lib/steamapps/common/Portal 2"),
            game(3, "Broken", ""),
            game(4, "Nested", "/lib/steamapps/common/Portal/extra"),
        ]);
        let none = Env::default();
        let find = |arg: &str| detect(&c, &none, &[arg]).map(|d| d.app_id);
        assert_eq!(find("/lib/steamapps/common/Portal 2/portal2.sh"), Some(2));
        assert_eq!(find("/lib/steamapps/common/Portal/hl2.sh"), Some(1));
        assert_eq!(find("/lib/steamapps/common/Portal"), Some(1));
        assert_eq!(find("/lib/steamapps/common/Portal/extra/x.exe"), Some(4));
        assert_eq!(find("/lib/steamapps/common/Portalx/a"), None);
        assert_eq!(find("-novid"), None, "an empty folder matches nothing");
    }

    #[test]
    fn two_games_in_one_folder_are_a_tie() {
        let c = cache(vec![
            game(1, "A", "/lib/steamapps/common/Shared"),
            game(2, "B", "/lib/steamapps/common/Shared/"),
        ]);
        assert_eq!(
            detect(&c, &Env::default(), &["/lib/steamapps/common/Shared/x"]),
            None
        );
    }

    fn write_app(lib: &Path, app_id: u64, name: &str, dir: &str) {
        std::fs::create_dir_all(lib.join("steamapps/common").join(dir)).unwrap();
        let text = format!(
            "\"AppState\" {{ \"appid\" \"{app_id}\" \"name\" \"{name}\" \"StateFlags\" \"4\" \"installdir\" \"{dir}\" }}"
        );
        std::fs::write(manifest::manifest_path(lib, app_id), text).unwrap();
    }

    #[test]
    fn without_the_cache_ids_are_taken_as_they_are_and_folders_use_manifests() {
        let dir = tempfile::tempdir().unwrap();
        let lib = dir.path().join("library2");
        write_app(&lib, 9100070, "Frostline Arena", "Frostline Arena");
        write_app(
            &lib,
            1493710,
            "Proton - Experimental",
            "Proton - Experimental",
        );
        let empty = GameCache::default();
        let env = Env::from_pairs([(STEAM_GAME_ID, "9100070")]);
        let d = detect::<&str>(&empty, &env, &[]).unwrap();
        assert_eq!(
            (d.app_id, d.game, d.clue),
            (9100070, None, Clue::Environment)
        );
        let d = detect(&empty, &Env::default(), &["SteamAppId=9100070"]).unwrap();
        assert_eq!((d.app_id, d.clue), (9100070, Clue::Argument));

        let proton = lib.join("steamapps/common/Proton - Experimental/proton");
        let exe = lib.join("steamapps/common/Frostline Arena/game/arena.exe");
        let cmd = [
            proton.as_os_str(),
            OsStr::new("waitforexitandrun"),
            exe.as_os_str(),
        ];
        let d = detect(&empty, &Env::default(), &cmd).unwrap();
        assert_eq!(d.app_id, 9100070);
        assert_eq!(d.name.as_deref(), Some("Frostline Arena"));
        assert_eq!(d.clue, Clue::InstallFolder);
        // Only the Proton script: a tool, so no game.
        assert_eq!(detect(&empty, &Env::default(), &cmd[..2]), None);
    }

    #[test]
    fn a_cached_folder_match_beats_an_uncached_id() {
        let c = cache(vec![game(1, "Cached", "/lib/steamapps/common/Cached")]);
        let env = Env::from_pairs([(STEAM_APP_ID, "99")]);
        let d = detect(&c, &env, &["/lib/steamapps/common/Cached/run"]).unwrap();
        assert_eq!(d.app_id, 1);
        let d = detect(&c, &env, &["/elsewhere/run"]).unwrap();
        assert_eq!((d.app_id, d.game), (99, None));
    }

    #[test]
    fn library_and_folder_needs_an_absolute_common_path() {
        assert_eq!(
            library_and_folder(Path::new("/mnt/g/steamapps/common/Game/bin/x.exe")),
            Some((PathBuf::from("/mnt/g"), "Game".to_owned()))
        );
        assert_eq!(library_and_folder(Path::new("steamapps/common/Game")), None);
        assert_eq!(
            library_and_folder(Path::new("/mnt/g/steamapps/common")),
            None
        );
    }

    fn process(root: &Path, pid: u32, cwd: &Path, exe: &Path, cmdline: &[u8]) {
        let dir = root.join(pid.to_string());
        std::fs::create_dir_all(&dir).unwrap();
        std::os::unix::fs::symlink(cwd, dir.join("cwd")).unwrap();
        std::os::unix::fs::symlink(exe, dir.join("exe")).unwrap();
        std::fs::write(dir.join("cmdline"), cmdline).unwrap();
    }

    #[test]
    fn running_games_read_a_fake_proc() {
        let dir = tempfile::tempdir().unwrap();
        let proc_root = dir.path().join("proc");
        std::fs::create_dir_all(dir.path().join("real/Harbor Lights")).unwrap();
        // /proc reports real paths.
        let real = std::fs::canonicalize(dir.path().join("real/Harbor Lights")).unwrap();
        let linked = dir.path().join("linked");
        std::os::unix::fs::symlink(dir.path().join("real"), &linked).unwrap();
        let games = vec![
            game(1, "By id", "/lib/steamapps/common/By id"),
            game(2, "By arg", "/lib/steamapps/common/By arg"),
            game(3, "By link", linked.join("Harbor Lights").to_str().unwrap()),
            game(4, "Idle", "/lib/steamapps/common/Idle"),
            game(5, "Own", "/lib/steamapps/common/Own"),
        ];
        process(
            &proc_root,
            100,
            Path::new("/"),
            Path::new("/usr/bin/reaper"),
            b"reaper\0SteamLaunch\0AppId=1\0--\0",
        );
        process(
            &proc_root,
            101,
            Path::new("/"),
            Path::new("/usr/bin/wine"),
            b"wine\0/lib/steamapps/common/By arg/x.exe\0",
        );
        process(&proc_root, 102, &real, Path::new("/usr/bin/wine"), b"");
        process(
            &proc_root,
            103,
            Path::new("/lib/steamapps/common/Own"),
            Path::new("/x"),
            b"x\0",
        );
        std::fs::create_dir_all(proc_root.join("self")).unwrap();
        std::fs::create_dir_all(proc_root.join("104")).unwrap();
        let running = running_games(&proc_root, 103, &games);
        assert_eq!(running.into_iter().collect::<Vec<_>>(), [1, 2, 3]);
        assert!(running_games(&dir.path().join("none"), 1, &games).is_empty());
        assert!(running_games(&proc_root, 1, &[]).is_empty());
    }
}
