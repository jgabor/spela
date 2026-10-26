//! The readers against what the migration writes: Go's fixtures are
//! migrated first, then read back through the deny list and manifest cache
//! modules, as spela does at startup. Each side is tested alone in
//! `go_fixtures.rs` and `go_dll_fixtures.rs`; this checks that they agree.

mod common;

use std::path::PathBuf;

use common::{copy_fixture, fixtures};
use serde_json::Value;
use spela_core::clock::ManualClock;
use spela_core::denylist;
use spela_core::dll::manifest::{self, DEFAULT_MANIFEST_URL};
use spela_core::games::DllType;
use spela_core::migrate;
use spela_core::paths::Paths;

struct Case {
    _dir: tempfile::TempDir,
    base: PathBuf,
    paths: Paths,
}

/// Copies a case into a temp dir, with `/fixture-root` replaced in text
/// files, and builds its paths by hand, because the fixtures keep Go's
/// `<home>/spela` layout.
fn case(name: &str) -> Case {
    let src = fixtures().join(name);
    let dir = tempfile::tempdir().unwrap();
    let base = dir.path().to_owned();
    copy_fixture(&src, &base, base.to_str().unwrap());
    let paths = Paths {
        config_dir: base.join("config/spela"),
        data_dir: base.join("data/spela"),
        cache_dir: base.join("cache/spela"),
        state_dir: base.join("state/spela"),
    };
    Case {
        _dir: dir,
        base,
        paths,
    }
}

fn clock() -> ManualClock {
    ManualClock::new("2026-10-26T17:04:11Z".parse().unwrap())
}

fn oracle(c: &Case) -> Value {
    let text = std::fs::read_to_string(c.base.join("oracle.json")).unwrap();
    match serde_json::from_str(&text).unwrap() {
        Value::Array(mut list) => list.remove(0),
        one => one,
    }
}

#[test]
fn migrated_lists_load_and_check_as_go_did_but_for_decision_36() {
    for (name, user_list_case) in [
        ("lists/overrides-empty", false),
        ("lists/overrides-full", false),
        ("lists/denylist-user", true),
    ] {
        let c = case(name);
        let report = migrate::run(&c.paths, &clock()).unwrap();
        assert!(report.failed().next().is_none(), "{name}: {report:?}");
        assert!(c.paths.deny_overrides().exists(), "{name}");
        let lists = denylist::load(&c.paths).unwrap();
        let oracle = oracle(&c);
        for check in oracle["is_denied"].as_array().unwrap() {
            let app_id = check["app_id"].as_u64().unwrap();
            let status = lists.check(app_id);
            let builtin = denylist::BUILT_IN.iter().find(|(id, _, _)| *id == app_id);
            if user_list_case && !lists.is_force_allowed(app_id) && builtin.is_some() {
                // Go's denylist.yaml replaced the built-in list; now it adds.
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
        assert!(denylist::guard_swap(&c.paths, 1).is_ok(), "{name}");
    }
}

#[test]
fn the_deny_list_commands_edit_a_migrated_overrides_file() {
    for name in ["lists/overrides-empty", "lists/overrides-full"] {
        let c = case(name);
        migrate::run(&c.paths, &clock()).unwrap();
        assert!(denylist::deny(&c.paths, 9100299, "New Game", "user-specified").unwrap());
        assert!(denylist::load(&c.paths).unwrap().check(9100299).is_denied());
        assert!(denylist::allow(&c.paths, 9100299).unwrap());
        let lists = denylist::load(&c.paths).unwrap();
        assert!(!lists.check(9100299).is_denied(), "{name}");
        assert!(lists.is_force_allowed(9100299), "{name}");
        assert!(denylist::clear(&c.paths, 9100299).unwrap());
        let text = std::fs::read_to_string(c.paths.deny_overrides()).unwrap();
        assert!(text.starts_with('#'), "{name}: the header stays\n{text}");
    }
}

#[test]
fn migrated_manifest_caches_load() {
    let c = case("caches/manifest-with-fetch-time");
    migrate::run(&c.paths, &clock()).unwrap();
    assert!(c.paths.manifest_cache().exists());
    let cached = manifest::load_cache(&c.paths).unwrap().unwrap();
    assert_eq!(
        cached.fetched_at,
        Some("2026-10-26T02:45:16.376726614+02:00".parse().unwrap())
    );
    assert_eq!(cached.source(), DEFAULT_MANIFEST_URL);
    assert_eq!(
        cached.manifest.latest(DllType::Dlss).unwrap().version,
        "310.5.3"
    );

    let c = case("caches/manifest-without-fetch-time");
    let go = c.paths.go().manifest_cache();
    let go_mtime =
        jiff::Timestamp::try_from(std::fs::metadata(&go).unwrap().modified().unwrap()).unwrap();
    migrate::run(&c.paths, &clock()).unwrap();
    let cached = manifest::load_cache(&c.paths).unwrap().unwrap();
    assert_eq!(
        cached.fetched_at,
        Some(go_mtime),
        "the seed keeps Go's time"
    );
}
