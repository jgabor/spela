//! The DLL read side and the deny list against files Go spela wrote:
//! `tests/fixtures/go/{backups,lists,caches,games,sandbox}`. Each case is
//! copied into a temp folder with `/fixture-root` replaced, as the
//! fixtures README says, so no test reads or writes outside it.

use std::path::{Path, PathBuf};

use serde::Deserialize;
use serde_json::Value;

use spela_core::clock::ManualClock;
use spela_core::denylist::{self, DenyLists, Overrides, UserList};
use spela_core::dll::backup::{self, NameScheme};
use spela_core::dll::detect;
use spela_core::dll::manifest::{self, DEFAULT_MANIFEST_URL};
use spela_core::games::DllType;
use spela_core::migrate;
use spela_core::paths::Paths;

const FIXTURE_ROOT: &str = "/fixture-root";

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../tests/fixtures/go")
}

/// goref's text files: the ones that got the `/fixture-root` swap.
fn is_text(path: &Path, data: &[u8]) -> bool {
    let ext = path
        .extension()
        .map(|e| e.to_string_lossy().to_lowercase())
        .unwrap_or_default();
    let text_ext = [
        "yaml", "yml", "json", "vdf", "acf", "txt", "md", "out", "err", "",
    ];
    text_ext.contains(&ext.as_str()) && !data.contains(&0)
}

fn copy_relocated(from: &Path, to: &Path, root: &str) {
    std::fs::create_dir_all(to).unwrap();
    for entry in std::fs::read_dir(from).unwrap() {
        let entry = entry.unwrap();
        let (src, dst) = (entry.path(), to.join(entry.file_name()));
        if entry.file_type().unwrap().is_dir() {
            copy_relocated(&src, &dst, root);
            continue;
        }
        let data = std::fs::read(&src).unwrap();
        if is_text(&src, &data) {
            let text = String::from_utf8(data).unwrap().replace(FIXTURE_ROOT, root);
            std::fs::write(&dst, text).unwrap();
        } else {
            std::fs::write(&dst, data).unwrap();
        }
    }
}

/// A fixture case in a temp folder, with spela's paths over Go's XDG
/// layout (`data/spela`, not `data`).
struct Case {
    _dir: tempfile::TempDir,
    root: PathBuf,
    paths: Paths,
}

fn case(name: &str) -> Case {
    let dir = tempfile::tempdir().unwrap();
    let root = dir.path().join("case");
    copy_relocated(&fixtures().join(name), &root, root.to_str().unwrap());
    let paths = Paths {
        config_dir: root.join("config/spela"),
        data_dir: root.join("data/spela"),
        cache_dir: root.join("cache/spela"),
        state_dir: dir.path().join("state"),
    };
    Case {
        _dir: dir,
        root,
        paths,
    }
}

/// Runs the migration, which copies Go's caches once as seeds.
fn seed(c: &Case) {
    let clock = ManualClock::new("2026-10-26T17:04:11Z".parse().unwrap());
    migrate::run(&c.paths, &clock).unwrap();
}

fn oracles(c: &Case) -> Vec<Value> {
    let text = std::fs::read_to_string(c.root.join("oracle.json")).unwrap();
    match serde_json::from_str(&text).unwrap() {
        Value::Array(list) => list,
        one => vec![one],
    }
}

const BACKUP_CASES: [&str; 5] = [
    "backups/sha256-names",
    "backups/basename-names",
    "backups/mixed-names",
    "backups/custom-backup-path",
    "backups/invalid-index",
];

#[test]
fn every_backups_case_reads_as_go_read_it() {
    let mut checked = 0;
    for name in BACKUP_CASES {
        let c = case(name);
        for oracle in oracles(&c) {
            let app_id = oracle["app_id"].as_u64().unwrap();
            let at = format!("{name} {app_id}");
            assert_eq!(
                c.paths.backup_index(app_id),
                Path::new(oracle["index_path"].as_str().unwrap()),
                "{at}: the index stays in data/spela/backups"
            );
            assert_eq!(
                backup::has_backup(&c.paths, app_id),
                oracle["backup_exists"].as_bool().unwrap(),
                "{at}"
            );
            let loaded = backup::load(&c.paths, app_id);
            match oracle["load_error"].as_str() {
                Some(go_error) => {
                    let ours = loaded.unwrap_err().to_string();
                    if go_error.starts_with("parse backup metadata: ") {
                        // serde's detail differs from Go's.
                        assert!(ours.starts_with("parse backup metadata: "), "{at}: {ours}");
                    } else {
                        assert_eq!(ours, go_error, "{at}");
                    }
                    assert!(backup::plan_restore(&c.paths, app_id).is_err(), "{at}");
                }
                None => {
                    let index = loaded.unwrap().unwrap();
                    let files = oracle["files"].as_array().unwrap();
                    assert_eq!(index.files.len(), files.len(), "{at}");
                    for (file, want) in index.files.iter().zip(files) {
                        assert_eq!(
                            file.original_path,
                            Path::new(want["original_path"].as_str().unwrap()),
                            "{at}"
                        );
                        assert_eq!(
                            file.backup_path,
                            Path::new(want["backup_path"].as_str().unwrap()),
                            "{at}"
                        );
                        let scheme = match file.name_scheme() {
                            NameScheme::Sha256 => "sha256-basename",
                            NameScheme::Basename | NameScheme::Other => "basename",
                        };
                        assert_eq!(scheme, want["name_scheme"].as_str().unwrap(), "{at}");
                        assert_eq!(
                            file.backup_path.is_file(),
                            want["backup_file_exists"].as_bool().unwrap(),
                            "{at}: {}",
                            file.backup_path.display()
                        );
                        assert!(file.backup_path.starts_with(&c.root), "{at}");
                    }
                    let plan = backup::plan_restore(&c.paths, app_id).unwrap();
                    assert_eq!(plan.copies, index.files, "{at}");
                    assert!(plan.deletes.is_empty(), "{at}");
                    assert_eq!(
                        index.created_at.as_deref(),
                        Some("2026-10-26T17:02:10.123456789+02:00"),
                        "{at}"
                    );
                }
            }
            checked += 1;
        }
    }
    assert_eq!(checked, 8, "four valid indexes and four Go refuses");
}

#[test]
fn custom_backup_path_files_sit_outside_the_index_folder() {
    let c = case("backups/custom-backup-path");
    let index = backup::load(&c.paths, 9100404).unwrap().unwrap();
    assert_eq!(index.backup_path, c.root.join("dll-backups/9100404"));
    assert!(
        index.files[0]
            .backup_path
            .starts_with(c.root.join("dll-backups"))
    );
    assert!(c.paths.backup_index(9100404).is_file());
}

#[test]
fn mixed_names_keep_the_first_backup_of_a_path() {
    let c = case("backups/mixed-names");
    let index = backup::load(&c.paths, 9100403).unwrap().unwrap();
    let schemes: Vec<NameScheme> = index.files.iter().map(|f| f.name_scheme()).collect();
    assert_eq!(schemes, [NameScheme::Basename, NameScheme::Sha256]);
    let dlss = index
        .entry(
            &c.root
                .join("library/steamapps/common/Lantern Keep/nvngx_dlss.dll"),
        )
        .unwrap();
    assert_eq!(dlss.version, "3.1.30", "the basename entry Go wrote first");
}

/// The goref fix: in the committed fixtures, every sha256 backup name is the
/// hash of its original path under /fixture-root, the file is there, and
/// oracle.json names the same file as backup.json.
#[test]
fn committed_hashed_names_match_their_original_paths() {
    let mut hashed = 0;
    let mut cases: Vec<&str> = BACKUP_CASES.to_vec();
    cases.extend(["sandbox/main", "sandbox/errors"]);
    for name in cases {
        let dir = fixtures().join(name);
        let indexes = std::fs::read_dir(dir.join("data/spela/backups")).unwrap();
        // Only the backups cases have a backup oracle.
        let oracle: Option<Value> = std::fs::read_to_string(dir.join("oracle.json"))
            .ok()
            .filter(|_| name.starts_with("backups/"))
            .map(|t| serde_json::from_str(&t).unwrap());
        for index in indexes {
            let path = index.unwrap().path().join("backup.json");
            let Ok(index) = serde_json::from_str::<backup::BackupIndex>(
                &std::fs::read_to_string(&path).unwrap(),
            ) else {
                continue;
            };
            for file in &index.files {
                let on_disk = dir.join(
                    file.backup_path
                        .strip_prefix(FIXTURE_ROOT)
                        .unwrap_or(&file.backup_path),
                );
                if file.name_scheme() == NameScheme::Sha256 {
                    assert_eq!(
                        file.backup_path.file_name().unwrap(),
                        backup::hashed_backup_name(&file.original_path),
                        "{name}: {}",
                        file.backup_path.display()
                    );
                    assert!(on_disk.is_file(), "{name}: {}", on_disk.display());
                    hashed += 1;
                }
                if let Some(Value::Object(o)) = &oracle {
                    let listed = o["files"].as_array().unwrap().iter().any(|f| {
                        f["original_path"] == file.original_path.to_str().unwrap()
                            && f["backup_path"] == file.backup_path.to_str().unwrap()
                    });
                    assert!(
                        listed,
                        "{name}: oracle.json disagrees with backup.json on {}",
                        file.backup_path.display()
                    );
                }
            }
        }
    }
    assert_eq!(
        hashed,
        3 + 1 + 1 + 3 + 3,
        "sha256, mixed, custom and both sandboxes"
    );
}

#[test]
fn the_sandbox_backup_loads_and_plans_a_restore() {
    for name in ["sandbox/main", "sandbox/errors"] {
        let c = case(name);
        let plan = backup::plan_restore(&c.paths, 9100020).unwrap();
        let types: Vec<Option<DllType>> = plan
            .copies
            .iter()
            .map(|f| detect::dll_type_of(&f.dll_name))
            .collect();
        assert_eq!(types.len(), 3, "{name}");
        assert!(types.iter().all(Option::is_some), "{name}: {types:?}");
    }
}

#[test]
fn go_lists_read_into_the_shared_types_and_check_as_decision_36_says() {
    for (name, user_list_case) in [
        ("lists/overrides-empty", false),
        ("lists/overrides-full", false),
        ("lists/denylist-user", true),
    ] {
        let c = case(name);
        let read = |path: &Path| std::fs::read_to_string(path).ok();
        let overrides: Overrides = read(&c.paths.go().deny_overrides())
            .map(|t| serde_saphyr::from_str(&t).unwrap())
            .unwrap_or_default();
        let user: UserList = read(&c.paths.go().user_denylist())
            .map(|t| serde_saphyr::from_str(&t).unwrap())
            .unwrap_or_default();
        let lists = DenyLists::new(user.clone(), overrides.clone());
        let oracle = &oracles(&c)[0];
        for check in oracle["is_denied"].as_array().unwrap() {
            let app_id = check["app_id"].as_u64().unwrap();
            let builtin = denylist::BUILT_IN.iter().find(|(id, _, _)| *id == app_id);
            let status = lists.check(app_id);
            if user_list_case && !lists.is_force_allowed(app_id) && builtin.is_some() {
                // Go: the user's denylist.yaml replaced the built-in list,
                // so these read as allowed. Decision 36: it adds to it.
                assert_eq!(check["denied"], false, "{name} {app_id}");
                assert_eq!(status.reason(), builtin.map(|b| b.2), "{name} {app_id}");
                continue;
            }
            assert_eq!(
                status.is_denied(),
                check["denied"].as_bool().unwrap(),
                "{name} {app_id}"
            );
            let reason = check["reason"].as_str().unwrap();
            assert_eq!(status.reason().unwrap_or(""), reason, "{name} {app_id}");
        }

        // The same lists as the migration writes them read back the same.
        std::fs::create_dir_all(c.paths.lists_dir()).unwrap();
        std::fs::write(
            c.paths.deny_overrides(),
            denylist::overrides_toml(&overrides),
        )
        .unwrap();
        let loaded = denylist::load(&c.paths).unwrap();
        assert_eq!(loaded.overrides(), &overrides, "{name}");
        if user_list_case {
            let toml_list: String = user
                .entries
                .iter()
                .map(|e| {
                    format!(
                        "[[entries]]\napp_id = {}\nname = {:?}\nreason = {:?}\n",
                        e.app_id, e.name, e.reason
                    )
                })
                .collect();
            std::fs::write(c.paths.user_denylist(), format!("version = 1\n{toml_list}")).unwrap();
            let loaded = denylist::load(&c.paths).unwrap();
            assert_eq!(loaded.check(9100204).reason(), Some("Easy Anti-Cheat"));
            assert!(loaded.check(1172470).is_denied());
            assert_eq!(loaded.anti_cheat().len(), 12);
        }
    }
}

#[test]
fn a_game_in_both_go_lists_is_allowed_until_the_next_write() {
    let c = case("lists/overrides-full");
    let overrides: Overrides =
        serde_saphyr::from_str(&std::fs::read_to_string(c.paths.go().deny_overrides()).unwrap())
            .unwrap();
    std::fs::create_dir_all(c.paths.lists_dir()).unwrap();
    std::fs::write(
        c.paths.deny_overrides(),
        denylist::overrides_toml(&overrides),
    )
    .unwrap();
    assert!(!denylist::load(&c.paths).unwrap().check(9100203).is_denied());
    // The next deny puts the file back to one list, and the deny wins.
    assert!(denylist::deny(&c.paths, 9100203, "Both Lists", "user-specified").unwrap());
    let lists = denylist::load(&c.paths).unwrap();
    assert!(lists.check(9100203).is_denied());
    assert_eq!(lists.overrides().allowed, [1172470]);
}

#[derive(Deserialize)]
struct GoGames {
    games: std::collections::BTreeMap<u64, GoGame>,
}

#[derive(Deserialize)]
struct GoGame {
    install_dir: PathBuf,
    #[serde(default)]
    dlls: Vec<GoDll>,
}

#[derive(Deserialize)]
struct GoDll {
    path: PathBuf,
    name: String,
    #[serde(rename = "type")]
    dll_type: String,
}

#[test]
fn detection_finds_what_go_s_scan_found() {
    let c = case("games/database");
    let text = std::fs::read_to_string(c.paths.go().game_database()).unwrap();
    let db: GoGames = serde_saphyr::from_str(&text).unwrap();
    // Counts the games with DLLs. Quiet Orchard's install folder is empty,
    // which git does not keep, so whether it exists depends on the checkout.
    let mut scanned = 0;
    for (app_id, game) in db.games {
        if !game.install_dir.is_dir() {
            assert!(game.dlls.is_empty(), "{app_id}");
            continue;
        }
        let found = detect::scan_directory(&game.install_dir, |_| None).unwrap();
        let mut got: Vec<(PathBuf, String, String)> = found
            .into_iter()
            .map(|d| (d.path, d.name, d.dll_type.as_str().to_owned()))
            .collect();
        let mut want: Vec<(PathBuf, String, String)> = game
            .dlls
            .into_iter()
            .map(|d| (d.path, d.name, d.dll_type))
            .collect();
        got.sort();
        want.sort();
        assert_eq!(got, want, "{app_id}");
        if !want.is_empty() {
            scanned += 1;
        }
    }
    assert_eq!(scanned, 3);
}

#[test]
fn go_manifest_caches_seed_the_v1_cache() {
    let c = case("caches/manifest-with-fetch-time");
    assert!(
        manifest::load_cache(&c.paths).unwrap().is_none(),
        "Go's file is read only by the migration (decision 53)"
    );
    seed(&c);
    let cached = manifest::load_cache(&c.paths).unwrap().unwrap();
    assert_eq!(
        cached.fetched_at,
        Some("2026-10-26T02:45:16.376726614+02:00".parse().unwrap())
    );
    assert_eq!(cached.source(), DEFAULT_MANIFEST_URL);
    let m = &cached.manifest;
    assert_eq!(m.latest(DllType::Dlss).unwrap().version, "310.5.3");
    assert_eq!(m.latest(DllType::Dlssg).unwrap().version, "310.5.3");
    assert_eq!(m.latest(DllType::Xess), None);
    for t in [DllType::Dlss, DllType::Dlssg, DllType::Dlssd] {
        for entry in m.entries(t) {
            entry.check(t).unwrap();
        }
    }

    let c = case("caches/manifest-without-fetch-time");
    let go = c.paths.go().manifest_cache();
    let mtime =
        jiff::Timestamp::try_from(std::fs::metadata(&go).unwrap().modified().unwrap()).unwrap();
    seed(&c);
    let cached = manifest::load_cache(&c.paths).unwrap().unwrap();
    assert_eq!(cached.fetched_at, Some(mtime));
    assert_eq!(cached.source_url, "");
    assert_eq!(cached.source(), DEFAULT_MANIFEST_URL);
}
