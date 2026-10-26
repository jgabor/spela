//! The migration against the files Go spela wrote, in
//! `tests/fixtures/go`, and what Go made of them, in each case's
//! `oracle.json`.
//!
//! Each case is copied into a temp dir with `/fixture-root` replaced by that
//! dir, and the migration runs on it with paths built by hand, because the
//! fixtures keep Go's `<home>/spela` layout. Every case checks that:
//!
//! - every value Go loaded comes out the same in the migrated TOML, or is
//!   listed as dropped in the record;
//! - a file Go failed to load fails here too, and writes nothing;
//! - the Go files are byte-identical afterwards;
//! - a second run changes no file.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use serde_json::Value as Json;
use spela_core::clock::ManualClock;
use spela_core::config;
use spela_core::games::cache;
use spela_core::migrate::{self, Dropped, Outcome, Record, Report, SourceKind, Why};
use spela_core::paths::Paths;
use spela_core::profile::resolve::{resolve, resolve_all_games};
use spela_core::profile::{FieldKey, OwnValues, store};

const ROOT_TOKEN: &str = "/fixture-root";

struct Case {
    _dir: tempfile::TempDir,
    base: PathBuf,
    paths: Paths,
    originals: BTreeMap<PathBuf, Vec<u8>>,
}

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../tests/fixtures/go")
}

/// Copies a case into a temp dir, with `/fixture-root` replaced in text
/// files only. `rename` moves a file inside the copy, from and to paths
/// relative to the case.
fn case_with(name: &str, rename: &[(&str, &str)]) -> Case {
    let src = fixtures().join(name);
    let dir = tempfile::tempdir().unwrap();
    let base = dir.path().to_owned();
    let root = base.to_str().unwrap();
    for entry in walkdir::WalkDir::new(&src) {
        let entry = entry.unwrap();
        let dst = base.join(entry.path().strip_prefix(&src).unwrap());
        if entry.file_type().is_dir() {
            std::fs::create_dir_all(&dst).unwrap();
            continue;
        }
        let bytes = std::fs::read(entry.path()).unwrap();
        let bytes = match String::from_utf8(bytes) {
            Ok(text) => text.replace(ROOT_TOKEN, root).into_bytes(),
            Err(e) => e.into_bytes(),
        };
        std::fs::write(&dst, bytes).unwrap();
    }
    for (from, to) in rename {
        std::fs::rename(base.join(from), base.join(to)).unwrap();
    }
    let paths = Paths {
        config_dir: base.join("config/spela"),
        data_dir: base.join("data/spela"),
        cache_dir: base.join("cache/spela"),
        state_dir: base.join("state/spela"),
    };
    let originals = snapshot(&base);
    Case {
        _dir: dir,
        base,
        paths,
        originals,
    }
}

fn case(name: &str) -> Case {
    case_with(name, &[])
}

/// Every file under `base` except the lock files, with its bytes.
fn snapshot(base: &Path) -> BTreeMap<PathBuf, Vec<u8>> {
    walkdir::WalkDir::new(base)
        .into_iter()
        .map(Result::unwrap)
        .filter(|e| e.file_type().is_file())
        .filter(|e| !e.path().starts_with(base.join("state")))
        .map(|e| (e.path().to_owned(), std::fs::read(e.path()).unwrap()))
        .collect()
}

fn clock() -> ManualClock {
    ManualClock::new("2026-10-26T17:04:11Z".parse().unwrap())
}

impl Case {
    fn oracle(&self, name: &str) -> Json {
        serde_json::from_slice(&std::fs::read(self.base.join(name)).unwrap()).unwrap()
    }

    fn record(&self) -> Record {
        Record::load(&self.paths.migration_record()).unwrap()
    }

    fn dropped(&self, source: &Path) -> Vec<Dropped> {
        self.record()
            .get(source)
            .map(|s| s.dropped.clone())
            .unwrap_or_default()
    }

    /// Runs the migration, then checks that the Go files are unchanged and
    /// that a second run changes nothing.
    fn migrate(&self) -> Report {
        let report = migrate::run(&self.paths, &clock()).unwrap();
        for (path, bytes) in &self.originals {
            assert_eq!(
                &std::fs::read(path).unwrap(),
                bytes,
                "{} changed",
                path.display()
            );
        }
        let after = snapshot(&self.base);
        let again = migrate::run(&self.paths, &clock()).unwrap();
        assert!(
            again
                .handled
                .iter()
                .all(|h| h.entry.result == Outcome::Failed),
            "a second run only retries failures: {:?}",
            again.handled
        );
        assert!(again.notice().is_empty());
        let second = snapshot(&self.base);
        assert_eq!(
            after.keys().collect::<Vec<_>>(),
            second.keys().collect::<Vec<_>>(),
            "a second run makes no file"
        );
        for (path, bytes) in &after {
            assert_eq!(
                &second[path],
                bytes,
                "{} changed on the second run",
                path.display()
            );
        }
        report
    }
}

/// A value in the record's form: JSON-like, with Go's nil SMT as `none`.
fn shown(value: &Json) -> String {
    match value {
        Json::Null => "none".to_owned(),
        other => other.to_string(),
    }
}

fn is_zero(value: &Json) -> bool {
    match value {
        Json::Null => true,
        Json::Bool(b) => !b,
        Json::Number(n) => n.as_i64() == Some(0),
        Json::String(s) => s.is_empty(),
        _ => false,
    }
}

fn find<'a>(dropped: &'a [Dropped], key: &str) -> Option<&'a Dropped> {
    dropped.iter().find(|d| d.key == key)
}

fn is_offset(key: &str) -> bool {
    key == "gpu.clock_offset" || key == "gpu.memory_offset"
}

/// Checks a profile case against its oracle: every field's effective value
/// and origin, in All games and in every game, including a game with no
/// profile file.
fn check_profiles(case: &Case, report: &Report) {
    let oracle = case.oracle("oracle.json");
    let paths = &case.paths;
    let all = &oracle["all_games"];
    let all_source = paths.go().all_games_profile();
    let all_dropped = case.dropped(&all_source);
    let all_values: Option<OwnValues> = if all["load_error"].is_string() {
        assert!(report.failed_all_games().is_some(), "All games must fail");
        assert!(!paths.all_games_profile().exists());
        None
    } else if all["exists"] == true {
        let loaded = store::load_all_games(paths)
            .unwrap()
            .expect("all-games.toml");
        assert!(loaded.warnings.is_empty(), "{:?}", loaded.warnings);
        Some(loaded.profile.values)
    } else {
        assert!(!paths.all_games_profile().exists());
        None
    };

    if let Some(values) = &all_values {
        for f in all["fields"].as_array().unwrap() {
            let key = f["field"].as_str().unwrap();
            let set_here = f["origin"] == "set_here";
            let dropped = find(&all_dropped, key);
            if f["kept"] == false || is_offset(key) {
                if set_here {
                    let d = dropped.unwrap_or_else(|| panic!("All games {key} must be recorded"));
                    assert_eq!(d.value, shown(&f["stored"]), "{key}");
                    let why = if is_offset(key) {
                        Why::OffsetMeaningChanged
                    } else {
                        Why::Retired
                    };
                    assert_eq!(d.why, why, "{key}");
                }
                if let Some(k) = FieldKey::from_key(key) {
                    assert!(!values.contains(k), "{key} is not copied (decision 31)");
                }
                continue;
            }
            if dropped.is_some() {
                continue;
            }
            let r = resolve_all_games(Some(values), FieldKey::from_key(key).unwrap());
            assert_eq!(r.origin.as_str(), f["origin"], "All games {key}");
            let want = if set_here {
                f["stored"].clone()
            } else {
                serde_json::to_value(r.key.field().default_value()).unwrap()
            };
            assert_eq!(
                serde_json::to_value(&r.value).unwrap(),
                want,
                "All games {key}"
            );
        }
    }

    for g in oracle["games"].as_array().unwrap() {
        let app_id = g["app_id"].as_u64().unwrap();
        let source = paths.go().game_profile(app_id);
        let target = paths.game_profile(app_id);
        if let Some(error) = g["load_error"].as_str() {
            assert!(
                report.failed_profile(app_id).is_some(),
                "{app_id} must fail like Go: {error}"
            );
            assert!(!target.exists(), "a failed source writes nothing");
            if error.starts_with("gpu.vrr") {
                assert_eq!(report.failed_profile(app_id), Some(error), "Go's text");
            }
            continue;
        }
        let game_values = if g["exists"] == true {
            let loaded = store::load_game(paths, app_id).unwrap().expect("game toml");
            assert_eq!(loaded.profile.name, g["name"].as_str().unwrap());
            assert!(loaded.warnings.is_empty(), "{:?}", loaded.warnings);
            Some(loaded.profile.values)
        } else {
            assert!(!target.exists());
            None
        };
        let dropped = case.dropped(&source);
        let Some(fields) = g["fields"].as_array() else {
            // Go could not resolve this game, because All games did not
            // load. Its own values still migrate: every marked field.
            let Some(values) = game_values.as_ref() else {
                continue;
            };
            for (key, on) in g["overrides_after_load"].as_object().unwrap() {
                match FieldKey::from_key(key) {
                    Some(k) if *on == true && !is_offset(key) => {
                        assert!(values.contains(k), "{app_id} {key} is own")
                    }
                    _ if *on == true => assert!(find(&dropped, key).is_some(), "{key}"),
                    _ => {}
                }
            }
            continue;
        };
        for f in fields {
            let key = f["field"].as_str().unwrap();
            let marked = f["overridden"] == true;
            let stored = &f["stored"];
            let here = find(&dropped, key);
            if !marked && game_values.is_some() && !is_zero(stored) {
                let d =
                    here.unwrap_or_else(|| panic!("{app_id} {key}: an unmarked value is recorded"));
                assert_eq!(d.why, Why::NoMarker, "{app_id} {key}");
            }
            if f["kept"] == false || is_offset(key) {
                if marked {
                    let d = here.unwrap_or_else(|| panic!("{app_id} {key} must be recorded"));
                    assert_eq!(d.value, shown(stored), "{app_id} {key}");
                }
                if let (Some(k), Some(values)) = (FieldKey::from_key(key), &game_values) {
                    assert!(!values.contains(k), "{key} is not copied (decision 31)");
                }
                continue;
            }
            if here.is_some_and(|d| d.why != Why::NoMarker) {
                continue;
            }
            let k = FieldKey::from_key(key).unwrap();
            let r = resolve(game_values.as_ref(), all_values.as_ref(), k);
            assert_eq!(
                serde_json::to_value(&r.value).unwrap(),
                f["effective"],
                "{app_id} {key} effective"
            );
            if find(&all_dropped, key).is_none() {
                assert_eq!(r.origin.as_str(), f["origin"], "{app_id} {key} origin");
            }
        }
    }

    let listing = store::list(paths).unwrap();
    assert!(listing.broken.is_empty(), "{:?}", listing.broken);
    let ids: Vec<u64> = listing.profiles.iter().map(|p| p.profile.app_id).collect();
    let want: Vec<u64> = oracle["list"]
        .as_array()
        .unwrap()
        .iter()
        .map(|v| v.as_u64().unwrap())
        .collect();
    assert_eq!(ids, want, "the games that load");
}

#[test]
fn every_profile_case_matches_its_oracle() {
    let cases = std::fs::read_dir(fixtures().join("profiles")).unwrap();
    let mut names: Vec<String> = cases
        .map(|e| e.unwrap().file_name().into_string().unwrap())
        .collect();
    names.sort();
    assert_eq!(names.len(), 20, "{names:?}");
    for name in names
        .iter()
        .map(|n| format!("profiles/{n}"))
        .chain(["sandbox/main".to_owned(), "e2e/library".to_owned()])
    {
        let case = case(&name);
        let report = case.migrate();
        check_profiles(&case, &report);
    }
}

#[test]
fn a_migrated_game_profile_reads_as_documented() {
    let case = case("profiles/every-field");
    case.migrate();
    let text = std::fs::read_to_string(case.paths.game_profile(9100101)).unwrap();
    let (header, body): (Vec<&str>, Vec<&str>) = text.lines().partition(|l| l.starts_with('#'));
    assert!(
        header[0].starts_with("# Migrated from 9100101.yaml (Go spela) on "),
        "{text}"
    );
    assert_eq!(header[1], "# Not carried over:");
    assert!(header.contains(
        &"#   gpu.clock_offset = 120 (Go locked clocks here; the field is now a real offset)"
    ));
    assert!(header.contains(&"#   overlay.toggle_key = \"F12\" (removed in spela 0.7)"));
    assert_eq!(
        header.last().unwrap(),
        &format!("# Full record: {}", case.paths.migration_record().display())
    );
    assert_eq!(
        body,
        [
            "version = 1",
            "name = \"Glimmer Run\"",
            "",
            "[proton]",
            "enable_hdr = true",
            "enable_wayland = true",
            "enable_ngx_updater = true",
            "vkd3d_heap = true",
            "",
            "[dlss]",
            "sr_mode = \"quality\"",
            "sr_preset = \"K\"",
            "sr_override = true",
            "rr_mode = \"dlaa\"",
            "rr_preset = \"L\"",
            "rr_override = true",
            "fg_enabled = true",
            "fg_override = true",
            "multi_frame = 4",
            "indicator = true",
            "fg_indicator = true",
            "",
            "[gpu]",
            "vrr = \"automatic\"",
            "power_limit = 300",
            "fan_speed = 70",
            "shader_cache = true",
            "shader_cache_path = \"/shader-cache\"",
            "threaded_optimization = true",
            "",
            "[cpu]",
            "governor = \"performance\"",
            "smt = false",
        ]
    );
}

#[test]
fn explicit_zeros_keep_own_zero_values_and_record_the_nil_smt() {
    let case = case("profiles/explicit-zeros");
    case.migrate();
    let game = store::load_game(&case.paths, 9100102).unwrap().unwrap();
    // 25 kept fields, minus the two offsets and SMT's nil.
    assert_eq!(game.profile.values.len(), 22);
    assert!(game.profile.values.iter().all(|(_, v)| v.is_zero()));
    let dropped = case.dropped(&case.paths.go().game_profile(9100102));
    let smt = find(&dropped, "cpu.smt").unwrap();
    assert_eq!((smt.value.as_str(), smt.why), ("none", Why::NotSupported));
    assert!(
        !smt.default,
        "the notice counts it: SMT now follows All games"
    );
}

#[test]
fn profile_delete_then_a_restart_does_not_bring_it_back() {
    let case = case("profiles/own-values");
    case.migrate();
    assert!(store::delete(&case.paths, 9100103).unwrap());
    let report = migrate::run(&case.paths, &clock()).unwrap();
    assert!(report.is_empty());
    assert!(!case.paths.game_profile(9100103).exists());
}

#[test]
fn a_lost_record_adopts_the_targets_and_imports_only_what_is_gone() {
    let case = case("profiles/own-values");
    case.migrate();
    let kept = std::fs::read(case.paths.game_profile(9100104)).unwrap();
    std::fs::remove_file(case.paths.migration_record()).unwrap();
    std::fs::remove_file(case.paths.game_profile(9100103)).unwrap();
    store::set(
        &case.paths,
        spela_core::profile::Scope::Game(9100104),
        "",
        &[(
            FieldKey::GpuPowerLimit,
            spela_core::profile::Value::Int(123),
        )],
    )
    .unwrap();
    let edited = std::fs::read(case.paths.game_profile(9100104)).unwrap();
    assert_ne!(edited, kept);
    let report = migrate::run(&case.paths, &clock()).unwrap();
    let adopted: Vec<SourceKind> = report
        .handled
        .iter()
        .filter(|h| h.adopted)
        .map(|h| h.kind)
        .collect();
    assert!(adopted.contains(&SourceKind::Game(9100104)), "{adopted:?}");
    assert_eq!(
        std::fs::read(case.paths.game_profile(9100104)).unwrap(),
        edited
    );
    assert!(case.paths.game_profile(9100103).exists(), "imported again");
    assert_eq!(report.migrated().count(), 1);
}

#[test]
fn a_target_that_does_not_load_is_left_alone() {
    let case = case("profiles/two-space");
    std::fs::write(case.paths.game_profile(9100114), "[gpu\n").unwrap();
    let report = migrate::run(&case.paths, &clock()).unwrap();
    let error = report.failed_profile(9100114).unwrap();
    assert!(
        error.contains("already exists but does not load"),
        "{error}"
    );
    assert_eq!(
        std::fs::read_to_string(case.paths.game_profile(9100114)).unwrap(),
        "[gpu\n"
    );
}

#[test]
fn file_names_go_never_loaded_are_skipped() {
    let case = case("profiles/file-names");
    case.migrate();
    let record = case.record();
    let dir = case.paths.profiles_dir();
    for name in ["09100130.yaml", "9100129.yml", "default.yml", "notes.yaml"] {
        let entry = record
            .get(&dir.join(name))
            .unwrap_or_else(|| panic!("{name}"));
        assert_eq!(entry.result, Outcome::Skipped, "{name}");
        assert!(entry.target.is_none());
    }
    assert!(record.get(&dir.join("README.txt")).is_none());
    assert!(!dir.join("9100130.toml").exists() && !dir.join("9100129.toml").exists());
}

/// Go's effective config value of `key` as `Config::get` writes it.
fn go_text(value: &Json) -> String {
    match value {
        Json::String(s) => s.clone(),
        Json::Array(items) => items
            .iter()
            .map(|i| i.as_str().unwrap().to_owned())
            .collect::<Vec<_>>()
            .join(":"),
        other => other.to_string(),
    }
}

#[test]
fn every_config_case_matches_its_oracle() {
    let mut names: Vec<String> = std::fs::read_dir(fixtures().join("config"))
        .unwrap()
        .map(|e| e.unwrap().file_name().into_string().unwrap())
        .collect();
    names.sort();
    assert_eq!(names.len(), 7);
    let defaults = config::Config::default();
    for name in names {
        let case = case(&format!("config/{name}"));
        let report = case.migrate();
        let oracle = case.oracle("oracle.json");
        let paths = &case.paths;
        if oracle["load_error"].is_string() {
            let message = report.config_failure().expect("the config must fail");
            assert!(message.contains("config.yaml"), "{message}");
            assert!(!paths.config_file().exists());
            continue;
        }
        let loaded = config::load(&paths.config_file()).unwrap();
        assert!(
            loaded.from_file && loaded.warnings.is_empty(),
            "{name}: {:?}",
            loaded.warnings
        );
        let c = loaded.config;
        let dropped = case.dropped(&paths.go().config());
        for (key, value) in oracle["effective"].as_object().unwrap() {
            let d = find(&dropped, key);
            match key.as_str() {
                "theme" | "compact_mode" => {
                    let d = d.unwrap_or_else(|| panic!("{name}: {key} is recorded"));
                    assert_eq!(d.value, value.to_string(), "{name} {key}");
                    continue;
                }
                "shader_cache" | "dll_cache_path" | "backup_path" => continue,
                _ => {}
            }
            let got = c.get(key).unwrap();
            match d.map(|d| d.why) {
                Some(Why::IgnoredByGo) => {
                    assert_eq!(
                        got,
                        defaults.get(key).unwrap(),
                        "{name} {key}: Go used the default"
                    );
                }
                Some(Why::Default) | None => assert_eq!(got, go_text(value), "{name} {key}"),
                Some(other) => panic!("{name} {key}: unexpected {other:?}"),
            }
        }
        assert_eq!(
            c.dll_cache_dir(paths),
            Path::new(oracle["dll_cache_dir"].as_str().unwrap())
        );
        assert_eq!(
            c.backup_files_dir(paths),
            Path::new(oracle["backup_files_dir"].as_str().unwrap())
        );
        assert_eq!(
            c.shader_cache_dir(paths),
            Path::new(oracle["shader_cache_dir"].as_str().unwrap())
        );
        if oracle["validate_error"].is_string() {
            assert_eq!(find(&dropped, "log_level").unwrap().why, Why::IgnoredByGo);
        }
        let keys: Vec<&str> = dropped.iter().map(|d| d.key.as_str()).collect();
        if name == "retired-keys" {
            for key in ["show_hints", "preferred_dll_source", "fancy_mode"] {
                assert!(keys.contains(&key), "{key}");
            }
        }
        if name == "default" {
            let text = std::fs::read_to_string(paths.config_file()).unwrap();
            let body: Vec<&str> = text.lines().filter(|l| !l.starts_with('#')).collect();
            assert_eq!(
                body,
                ["version = 1"],
                "defaults are not written (decision 43)"
            );
            assert!(report.notable_dropped().next().is_none());
        }
    }
}

#[test]
fn every_list_case_matches_its_oracle() {
    for name in [
        "lists/denylist-user",
        "lists/overrides-empty",
        "lists/overrides-full",
    ] {
        let case = case(name);
        case.migrate();
        let oracle = case.oracle("oracle.json");
        let paths = &case.paths;
        let text = std::fs::read_to_string(paths.deny_overrides()).unwrap();
        let toml: toml::Table = toml::from_str(&text).unwrap();
        assert_eq!(toml["version"].as_integer(), Some(1));
        let allowed: Vec<u64> = toml["allowed"]
            .as_array()
            .unwrap()
            .iter()
            .map(|v| v.as_integer().unwrap() as u64)
            .collect();
        let want_allowed: Vec<u64> = oracle["overrides"]["Allowed"]
            .as_array()
            .unwrap()
            .iter()
            .map(|v| v.as_u64().unwrap())
            .collect();
        assert_eq!(allowed, want_allowed, "{name}");
        let denied: Vec<(u64, String, String)> = toml["denied"]
            .as_array()
            .unwrap()
            .iter()
            .map(|e| {
                (
                    e["app_id"].as_integer().unwrap() as u64,
                    e["name"].as_str().unwrap().to_owned(),
                    e["reason"].as_str().unwrap().to_owned(),
                )
            })
            .collect();
        let dropped = case.dropped(&paths.go().deny_overrides());
        let mut want_denied = Vec::new();
        for e in oracle["overrides"]["Denied"].as_array().unwrap() {
            let id = e["AppID"].as_u64().unwrap();
            if want_allowed.contains(&id) {
                let d = find(&dropped, &format!("denied.{id}")).expect("recorded");
                assert_eq!(d.why, Why::AllowedAndDenied);
                assert!(d.value.contains(e["Reason"].as_str().unwrap()));
                continue;
            }
            want_denied.push((
                id,
                e["Name"].as_str().unwrap().to_owned(),
                e["Reason"].as_str().unwrap().to_owned(),
            ));
        }
        assert_eq!(denied, want_denied, "{name}");
        let user_list = paths.go().user_denylist();
        if user_list.exists() {
            let text = std::fs::read_to_string(paths.user_denylist()).unwrap();
            assert!(text.contains("Now it adds"), "{text}");
            let toml: toml::Table = toml::from_str(&text).unwrap();
            let entries: Vec<u64> = toml["entries"]
                .as_array()
                .unwrap()
                .iter()
                .map(|e| e["app_id"].as_integer().unwrap() as u64)
                .collect();
            let want: Vec<u64> = oracle["denylist"]["Entries"]
                .as_array()
                .unwrap()
                .iter()
                .map(|e| e["AppID"].as_u64().unwrap())
                .collect();
            assert_eq!(entries, want);
        } else {
            assert!(!paths.user_denylist().exists());
        }
    }
}

#[test]
fn the_game_database_seeds_the_cache() {
    for (name, oracle_file) in [
        ("games/database", "oracle.json"),
        ("e2e/library", "oracle-games.json"),
    ] {
        let case = case(name);
        case.migrate();
        let oracle = case.oracle(oracle_file);
        let games = cache::load(&case.paths);
        let ids: Vec<u64> = games.games.iter().map(|g| g.app_id).collect();
        let want: Vec<u64> = oracle["all_app_ids"]
            .as_array()
            .unwrap()
            .iter()
            .map(|v| v.as_u64().unwrap())
            .collect();
        assert_eq!(ids, want, "{name}: every game, tools included");
        for (query, app_id) in oracle["find_game"].as_object().unwrap() {
            let got = games.find(query).map(|g| g.app_id).unwrap_or(0);
            assert_eq!(got, app_id.as_u64().unwrap(), "{name}: find {query}");
        }
        assert!(games.updated_at.is_some());
    }
    let case = case("games/database");
    case.migrate();
    let games = cache::load(&case.paths);
    let glimmer = games.get(9100301).unwrap();
    let versions: Vec<Option<&str>> = glimmer.dlls.iter().map(|d| d.version.as_deref()).collect();
    assert_eq!(versions, [Some("310.2.1"), Some("310.1"), Some("310.2.1")]);
    assert_eq!(
        glimmer.scanned_at,
        "2026-10-26T15:02:10.123456789Z".parse().unwrap()
    );
    let paper = games.get(9100303).unwrap();
    assert_eq!(paper.dlls[0].name, "NVNGX_DLSS.DLL");
    assert_eq!(paper.dlls[0].version, None);
    assert_eq!(paper.prefix_path, None);
}

#[test]
fn a_game_cache_a_scan_already_wrote_is_kept() {
    let case = case("games/database");
    cache::update(&case.paths, |c| {
        c.upsert(spela_core::games::Game {
            app_id: 1,
            name: "Scanned".into(),
            install_dir: "/a".into(),
            library_path: "/".into(),
            prefix_path: None,
            scanned_at: "2026-10-26T15:02:10Z".parse().unwrap(),
            dlls: Vec::new(),
        });
    })
    .unwrap();
    let report = case.migrate();
    let games = cache::load(&case.paths);
    assert_eq!(games.games.len(), 1);
    assert!(
        report
            .handled
            .iter()
            .any(|h| h.kind == SourceKind::GameDatabase && h.adopted)
    );
}

#[test]
fn caches_are_seeded_once_with_their_bytes_and_time() {
    for name in [
        "caches/manifest-with-fetch-time",
        "caches/manifest-without-fetch-time",
    ] {
        let case = case(name);
        let source = case.paths.go().manifest_cache();
        let old = std::time::SystemTime::UNIX_EPOCH + std::time::Duration::from_secs(1_760_000_000);
        std::fs::File::options()
            .write(true)
            .open(&source)
            .unwrap()
            .set_modified(old)
            .unwrap();
        case.migrate();
        let target = case.paths.manifest_cache();
        assert_eq!(
            std::fs::read(&target).unwrap(),
            std::fs::read(&source).unwrap()
        );
        assert_eq!(std::fs::metadata(&target).unwrap().modified().unwrap(), old);
        std::fs::remove_file(&target).unwrap();
        assert!(migrate::run(&case.paths, &clock()).unwrap().is_empty());
        assert!(!target.exists(), "seeded once only (decision 53)");
    }
    for which in ["ok", "failed"] {
        let from = format!("cache/spela/update-check.{which}.json");
        let case = case_with(
            "caches/update-check",
            &[(&from, "cache/spela/update-check.json")],
        );
        case.migrate();
        assert_eq!(
            std::fs::read(case.paths.release_check()).unwrap(),
            std::fs::read(case.paths.go().update_check()).unwrap()
        );
    }
}

#[test]
fn the_errors_sandbox_fails_only_the_broken_profiles() {
    let case = case("sandbox/errors");
    let report = case.migrate();
    let failed: Vec<SourceKind> = report.failed().map(|h| h.kind).collect();
    assert_eq!(
        failed,
        [SourceKind::Game(9100030), SourceKind::Game(9100060)]
    );
    assert!(
        report
            .failed_profile(9100060)
            .unwrap()
            .starts_with("gpu.vrr: unsupported value")
    );
    for app_id in [9100010, 9100020, 9100070] {
        assert!(case.paths.game_profile(app_id).exists(), "{app_id}");
    }
    assert!(report.config_failure().is_none());
    assert!(!report.notice().is_empty());
}

#[test]
fn the_notice_and_record_of_a_dropped_field_case() {
    let case = case("profiles/dropped-fields");
    let report = case.migrate();
    let notice = report.notice();
    assert_eq!(notice.len(), 2, "{notice:?}");
    assert!(
        notice[0].starts_with("Migrated 2 files from Go spela, kept as backups only. Dropped ")
    );
    assert!(
        notice[1].starts_with("GPU clock and memory offsets from Go spela were not carried over.")
    );
    let record = std::fs::read_to_string(case.paths.migration_record()).unwrap();
    for (key, value) in [
        ("gpu.clock_offset", "150"),
        ("gpu.memory_offset", "9501"),
        ("cpu.affinity", "\"0-7,16-23\""),
        ("overlay.toggle_key", "\"Shift+F12\""),
    ] {
        let needle = format!(
            "key = \"{key}\"\nvalue = {}\n",
            toml::Value::String(value.to_owned())
        );
        assert!(record.contains(&needle), "{needle}\n{record}");
    }
    let parsed = case.record();
    let game = parsed.get(&case.paths.go().game_profile(9100118)).unwrap();
    assert_eq!(game.result, Outcome::Migrated);
    assert_eq!(game.sha256.len(), 64);
    assert_eq!(
        game.target.as_deref(),
        Some(case.paths.game_profile(9100118).as_path())
    );
}
