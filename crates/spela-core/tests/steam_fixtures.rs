//! The Steam scan, the PE versions and the Proton resolver against the
//! fixtures Go spela wrote (`tests/fixtures/go`, fictional games). Each test
//! copies a case into a temp folder and replaces `/fixture-root` with it,
//! so nothing reads or writes a real Steam library or XDG folder.

mod common;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use common::{copy_fixture, fixtures};

use serde::Deserialize;
use spela_core::clock::ManualClock;
use spela_core::env::Env;
use spela_core::games::{DllType, GameCache, cache, scan};
use spela_core::paths::Paths;
use spela_core::proton::{self, ResolveError};
use spela_core::steam::detect::{self, Clue};
use spela_core::steam::library::SteamInstall;

fn fixture(case: &str) -> PathBuf {
    fixtures().join(case)
}

/// Copies a case and points its paths at the copy. Only text files are
/// rewritten; the PE stubs are copied byte for byte.
fn relocate(case: &str) -> tempfile::TempDir {
    let dir = tempfile::tempdir().unwrap();
    copy_fixture(&fixture(case), dir.path(), dir.path().to_str().unwrap());
    dir
}

#[derive(Debug, Deserialize)]
struct GoDatabase {
    games: BTreeMap<u64, GoGame>,
}

#[derive(Debug, Deserialize)]
struct GoGame {
    app_id: u64,
    name: String,
    install_dir: PathBuf,
    library_path: PathBuf,
    #[serde(default)]
    prefix_path: Option<PathBuf>,
    #[serde(default)]
    dlls: Vec<GoDll>,
}

#[derive(Debug, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
struct GoDll {
    path: PathBuf,
    name: String,
    #[serde(rename = "type")]
    dll_type: String,
    #[serde(default)]
    version: Option<String>,
}

fn go_database(root: &Path) -> GoDatabase {
    let text = std::fs::read_to_string(root.join("data/spela/games.yaml")).unwrap();
    serde_saphyr::from_str(&text).unwrap()
}

fn clock() -> ManualClock {
    ManualClock::new("2026-10-26T15:02:10Z".parse().unwrap())
}

fn sandbox_install(root: &Path) -> SteamInstall {
    SteamInstall::discover("", &[], Some(&root.join("home")))
}

#[test]
fn the_sandbox_scan_matches_go_and_keeps_the_pending_update() {
    let dir = relocate("sandbox/main");
    let root = dir.path();
    let install = sandbox_install(root);
    let steam_root = root.join("home/.steam/steam");
    assert_eq!(install.root.as_deref(), Some(steam_root.as_path()));
    let libraries: Vec<&Path> = install.library_paths().collect();
    assert_eq!(
        libraries,
        [steam_root.as_path(), root.join("library2").as_path()]
    );

    let found = scan::scan(&install, &clock());
    assert!(found.problems.is_empty(), "{:?}", found.problems);
    let ids: Vec<u64> = found.games.iter().map(|g| g.app_id).collect();
    // Go dropped 9100050, whose StateFlags 6 means an update is pending.
    // 9100040 is a tool: a toolmanifest.vdf and a Proton name.
    assert_eq!(ids, [9100010, 9100020, 9100030, 9100050, 9100060, 9100070]);

    let go = go_database(root);
    for game in found.games.iter().filter(|g| g.app_id != 9100050) {
        let want = &go.games[&game.app_id];
        assert_eq!(game.app_id, want.app_id);
        assert_eq!(game.name, want.name);
        assert_eq!(game.install_dir, want.install_dir, "{}", game.name);
        assert_eq!(game.library_path, want.library_path, "{}", game.name);
        assert_eq!(game.prefix_path, want.prefix_path, "{}", game.name);
        let mut dlls: Vec<GoDll> = game
            .dlls
            .iter()
            .map(|d| GoDll {
                path: d.path.clone(),
                name: d.name.clone(),
                dll_type: d.dll_type.as_str().to_owned(),
                version: d.version.clone(),
            })
            .collect();
        let mut want_dlls: Vec<&GoDll> = want.dlls.iter().collect();
        // Go kept walk order; the cache sorts by family, then path.
        want_dlls.sort();
        dlls.sort();
        assert_eq!(dlls.iter().collect::<Vec<_>>(), want_dlls, "{}", game.name);
    }
    let types: Vec<DllType> = found.games[0].dlls.iter().map(|d| d.dll_type).collect();
    assert_eq!(types, [DllType::Dlss, DllType::Dlssg, DllType::Dlssd]);
    assert_eq!(found.games[0].dlls[0].version.as_deref(), Some("310.2.1"));
    let xess = &found.games[1].dlls[1];
    assert_eq!(
        (xess.dll_type, xess.version.as_deref()),
        (DllType::Xess, Some("1.3.1.33"))
    );

    let pending = &found.games[3];
    assert_eq!(pending.name, "Pending Patch");
    assert_eq!(pending.dlls.len(), 1);
    assert_eq!(pending.prefix_path, None);
}

#[test]
fn a_rescan_writes_the_cache_that_find_reads() {
    let dir = relocate("sandbox/main");
    let root = dir.path();
    let paths = Paths::under(&root.join("xdg"));
    scan::rescan(&paths, &sandbox_install(root), &clock()).unwrap();
    let cached = cache::load(&paths);
    assert_eq!(cached.games.len(), 6);
    assert_eq!(
        cached.find("Paper Comet: Director's Cut").unwrap().app_id,
        9100060
    );
    assert_eq!(cached.find("9100070").unwrap().name, "Frostline Arena");
    let text = std::fs::read_to_string(paths.game_cache()).unwrap();
    assert_eq!(GameCache::from_json(&text).unwrap(), cached);
    let version = cached.get(9100060).unwrap().dlls[0].version.clone();
    assert_eq!(version, None, "NVNGX_DLSS.DLL has no readable version");
}

#[test]
fn an_empty_home_has_no_steam() {
    let dir = relocate("sandbox/empty");
    let install = SteamInstall::discover("", &[], Some(&dir.path().join("home")));
    assert_eq!(install.root, None);
    let paths = Paths::under(&dir.path().join("xdg"));
    let err = scan::rescan(&paths, &install, &clock()).unwrap_err();
    assert_eq!(err.to_string(), "could not find Steam installation");
}

#[test]
fn pe_versions_match_go_in_every_formatter_shape() {
    let dir = relocate("games/database");
    let go = go_database(dir.path());
    let mut checked = 0;
    for game in go.games.values() {
        for dll in &game.dlls {
            assert_eq!(
                spela_core::pe::version_string(&dll.path),
                dll.version,
                "{}",
                dll.path.display()
            );
            checked += 1;
        }
        if game.install_dir.is_dir() {
            let found = spela_core::dll::detect::scan_directory(
                &game.install_dir,
                spela_core::pe::version_string,
            )
            .unwrap();
            assert_eq!(found.len(), game.dlls.len(), "{}", game.name);
        }
    }
    assert_eq!(checked, 7);
}

#[test]
fn proton_resolves_where_go_could_and_where_it_could_not() {
    let dir = relocate("sandbox/main");
    let root = dir.path();
    let install = sandbox_install(root);
    let tools = root.join("home/.steam/steam/compatibilitytools.d");

    let cachyos = proton::resolve(&install, 9100010).unwrap();
    assert_eq!(cachyos.name, "cachyos-10.0-20260330-slr");
    assert_eq!(cachyos.path, tools.join("cachyos-10.0-20260330-slr"));
    assert!(proton::uses_legacy_heap_gate(&cachyos).unwrap());

    let ge = proton::resolve(&install, 9100060).unwrap();
    assert_eq!(ge.name, "GE-Proton10-34");
    assert_eq!(
        proton::check(&Ok(ge), Some("580.94.16")).notice().unwrap(),
        "⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ (detected: GE-Proton10-34)"
    );

    // Go could not resolve this one: the internal name differs from the
    // folder name.
    let internal = proton::resolve(&install, 9100070).unwrap();
    assert_eq!(internal.name, "GE-Proton11-7-x86_64");
    assert_eq!(internal.path, tools.join("GE-Proton11-7"));

    // The global default is the built-in short name.
    let default = proton::resolve(&install, 9100030).unwrap();
    assert_eq!(default.name, "proton_experimental");
    assert_eq!(
        default.path,
        root.join("home/.steam/steam/steamapps/common/Proton - Experimental")
    );

    let no_root = SteamInstall::default();
    let unresolved = proton::resolve(&no_root, 9100010);
    assert!(matches!(unresolved, Err(ResolveError::NoSteamRoot)));
    assert_eq!(
        proton::check(&unresolved, Some("580.94.16"))
            .notice()
            .unwrap(),
        "ⓘ could not resolve active Proton for this game; skipping compatibility check"
    );
}

#[test]
fn the_wrapper_finds_a_game_without_the_cache() {
    let dir = relocate("sandbox/main");
    let root = dir.path();
    let proton = root.join("home/.steam/steam/steamapps/common/Proton - Experimental/proton");
    let exe = root.join("library2/steamapps/common/Frostline Arena/game/arena.exe");
    let command = [
        proton.into_os_string(),
        "waitforexitandrun".into(),
        exe.into_os_string(),
    ];
    let empty = GameCache::default();
    let found = detect::detect(&empty, &Env::default(), &command).unwrap();
    assert_eq!(found.app_id, 9100070);
    assert_eq!(found.name.as_deref(), Some("Frostline Arena"));
    assert_eq!(found.clue, Clue::InstallFolder);

    let paths = Paths::under(&root.join("xdg"));
    scan::rescan(&paths, &sandbox_install(root), &clock()).unwrap();
    let cached = cache::load(&paths);
    let found = detect::detect(&cached, &Env::default(), &command).unwrap();
    assert_eq!(found.game.unwrap().app_id, 9100070);
}
