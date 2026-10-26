//! `dll install`, `update`, `restore`, `cache` and `check-updates` as the
//! real binary in the sandbox.
//!
//! The sandbox's fake network answers nothing, so these runs need no
//! download: the manifest cache is seeded with a loopback source and auto
//! refresh off, and the DLL cache with payloads whose SHA-256 the manifest
//! names. Downloads are tested in process, against the fake with routes
//! (`src/commands/dll_write.rs`), and in `spela-core`'s `tests/dll_ops.rs`.
//! Every run checks that each file it changed is the game's DLL, a backup,
//! a cache or a lock under the sandbox.

mod common;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use common::Sandbox;
use serde_json::Value;

const PAYLOAD: &[u8] = b"spela fixture payload 310.5.3";
const ORIGINAL: &[u8] = b"not a real DLL";
const MANIFEST_URL: &str = "http://127.0.0.1:9/manifest.json";

fn sha(bytes: &[u8]) -> String {
    spela_core::hex::sha256_hex(bytes)
}

/// A fictional Steam install with two games, scanned, a seeded manifest
/// and a seeded DLL cache.
struct Lab {
    s: Sandbox,
    common: PathBuf,
}

impl Lab {
    fn new() -> Lab {
        let s = Sandbox::new();
        let root = s.home().join(".steam/steam");
        write(
            &root.join("steamapps/libraryfolders.vdf"),
            &format!(
                "\"libraryfolders\"\n{{\n\t\"0\"\n\t{{\n\t\t\"path\"\t\t\"{}\"\n\t}}\n}}\n",
                root.display()
            ),
        );
        let lab = Lab {
            common: root.join("steamapps/common"),
            s,
        };
        lab.game(
            &root,
            9100610,
            "Amber Reach",
            "Amber",
            &["bin/nvngx_dlss.dll"],
        );
        lab.game(
            &root,
            9100620,
            "Birch Hollow",
            "Birch",
            &["nvngx_dlss.dll", "libxess.dll"],
        );
        lab.game(&root, 9100630, "Cinder Gate", "Cinder", &[]);
        write(
            &lab.s.config_dir().join("config.toml"),
            &format!("dll_manifest_url = \"{MANIFEST_URL}\"\nauto_refresh_manifest = false\n"),
        );
        write(
            &lab.cache().join("manifest-v1.json"),
            &format!(
                r#"{{"version":"1","dlls":{{"dlss":[
                    {{"version":"310.5.3","filename":"nvngx_dlss.dll","url":"http://127.0.0.1:9/dlss.dll","sha256":"{}"}},
                    {{"version":"310.4.0","filename":"nvngx_dlss.dll","url":"http://127.0.0.1:9/old.dll","sha256":"{}"}}],
                    "xess":[{{"version":"1.3.1","filename":"libxess.dll","url":"http://127.0.0.1:9/xess.dll","sha256":"{}"}}]}},
                    "fetched_at":"2026-10-26T15:00:00Z","source_url":"{MANIFEST_URL}"}}"#,
                sha(PAYLOAD),
                sha(b"old"),
                sha(b"xess")
            ),
        );
        let dlls = lab.cache().join("dlls");
        std::fs::create_dir_all(dlls.join("dlss")).unwrap();
        std::fs::write(dlls.join("dlss/310.5.3.dll"), PAYLOAD).unwrap();
        std::fs::write(dlls.join("dlss/310.4.0.dll"), b"old").unwrap();
        lab.s.run(&["scan"]).expect_code(0);
        lab
    }

    fn game(&self, root: &Path, app_id: u64, name: &str, dir: &str, files: &[&str]) {
        write(
            &root.join(format!("steamapps/appmanifest_{app_id}.acf")),
            &format!(
                "\"AppState\"\n{{\n\t\"appid\"\t\t\"{app_id}\"\n\t\"name\"\t\t\"{name}\"\n\t\"StateFlags\"\t\t\"4\"\n\t\"installdir\"\t\t\"{dir}\"\n}}\n"
            ),
        );
        std::fs::create_dir_all(self.common.join(dir)).unwrap();
        for file in files {
            write(&self.common.join(dir).join(file), "not a real DLL");
        }
    }

    fn cache(&self) -> PathBuf {
        self.s.root().join("xdg-cache/spela")
    }

    fn tree(&self) -> BTreeMap<PathBuf, Vec<u8>> {
        let mut out = BTreeMap::new();
        walk(self.s.root(), &mut out);
        out
    }

    /// Runs spela and checks where it wrote.
    fn run(&self, args: &[&str]) -> common::Run {
        let before = self.tree();
        let run = self.s.run(args);
        let after = self.tree();
        let allowed = [
            self.common.clone(),
            self.s.root().join("xdg-data/spela/backups"),
            self.cache().join("games.json"),
            self.cache().join("dlls"),
            self.s.state_dir().join("locks"),
            self.s.config_dir().join("lists"),
        ];
        for (path, bytes) in &after {
            if before.get(path) != Some(bytes) {
                assert!(
                    allowed.iter().any(|a| path.starts_with(a)),
                    "{args:?} wrote {}",
                    path.display()
                );
            }
        }
        run
    }
}

fn write(path: &Path, text: &str) {
    std::fs::create_dir_all(path.parent().unwrap()).unwrap();
    std::fs::write(path, text).unwrap();
}

fn walk(dir: &Path, out: &mut BTreeMap<PathBuf, Vec<u8>>) {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if entry.file_type().unwrap().is_dir() {
            walk(&path, out);
        } else if let Ok(bytes) = std::fs::read(&path) {
            out.insert(path, bytes);
        }
    }
}

fn json(run: &common::Run) -> Value {
    serde_json::from_str(&run.stdout()).unwrap_or_else(|e| panic!("{e}: {}", run.stdout()))
}

#[test]
fn update_then_restore_puts_the_original_back() {
    let lab = Lab::new();
    let dll = lab.common.join("Amber/bin/nvngx_dlss.dll");
    let run = lab.run(&["dll", "update", "Amber Reach", "DLSS"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "Updated dlss DLL to version 310.5.3\nDLL update result: 1 updated, 0 unchanged, 0 failed\n"
    );
    assert_eq!(std::fs::read(&dll).unwrap(), PAYLOAD);
    let index: Value = serde_json::from_slice(
        &std::fs::read(
            lab.s
                .root()
                .join("xdg-data/spela/backups/9100610/backup.json"),
        )
        .unwrap(),
    )
    .unwrap();
    assert_eq!(index["files"][0]["original_path"], dll.to_str().unwrap());

    let run = lab.run(&["dll", "restore", "9100610"]);
    run.expect_code(0);
    assert_eq!(run.stdout(), "Restored original DLLs for Amber Reach\n");
    assert_eq!(std::fs::read(&dll).unwrap(), ORIGINAL);
}

#[test]
fn cached_only_takes_the_newest_cached_version_or_the_pinned_one() {
    let lab = Lab::new();
    let dll = lab.common.join("Amber/bin/nvngx_dlss.dll");
    let run = lab.run(&[
        "dll",
        "update",
        "Amber Reach",
        "dlss",
        "--cached-only",
        "--version",
        "310.4.0",
    ]);
    run.expect_code(0);
    assert_eq!(std::fs::read(&dll).unwrap(), b"old");
    // The PE-less payload has no version, so the next update swaps again.
    let run = lab.run(&["dll", "update", "Amber Reach", "dlss", "--cached-only"]);
    run.expect_code(0);
    assert_eq!(std::fs::read(&dll).unwrap(), PAYLOAD);
    // Removing the cache: cached only never downloads.
    std::fs::remove_file(lab.cache().join("dlls/dlss/310.5.3.dll")).unwrap();
    let run = lab.run(&[
        "dll",
        "update",
        "Amber Reach",
        "dlss",
        "--cached-only",
        "--version",
        "310.5.3",
    ]);
    run.expect_code(1);
    assert!(run.stdout().starts_with("Failed "), "{}", run.stdout());
    assert!(
        run.stderr().contains("cached DLL not found"),
        "{}",
        run.stderr()
    );
}

#[test]
fn install_creates_a_file_and_restore_removes_it() {
    let lab = Lab::new();
    // Cinder Gate has no DLL: install writes the manifest's file name.
    std::fs::write(lab.cache().join("dlls/dlss/310.5.3.dll"), PAYLOAD).unwrap();
    let run = lab.run(&[
        "dll",
        "install",
        "Cinder Gate",
        "dlss",
        "--version",
        "310.5.3",
    ]);
    run.expect_code(0);
    let created = lab.common.join("Cinder/nvngx_dlss.dll");
    assert_eq!(
        run.stdout(),
        format!(
            "Installed dlss 310.5.3 for Cinder Gate at {}\n",
            created.display()
        )
    );
    assert_eq!(std::fs::read(&created).unwrap(), PAYLOAD);
    let run = lab.run(&["--json", "dll", "restore", "Cinder Gate"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["schema_version"], 1);
    assert_eq!(v["deleted"][0], created.to_str().unwrap());
    assert_eq!(v["restored"], Value::Array(vec![]));
    assert!(!created.exists());
    assert!(
        !lab.s
            .root()
            .join("xdg-data/spela/backups/9100630/backup.json")
            .exists(),
        "an index with nothing left to restore goes"
    );
}

#[test]
fn update_all_skips_denied_games_and_prints_json() {
    let lab = Lab::new();
    lab.s
        .run(&[
            "denylist",
            "deny",
            "Birch Hollow",
            "--reason",
            "kernel anti-cheat",
        ])
        .expect_code(0);
    let run = lab.run(&["dll", "update", "--all", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["schema_version"], 1);
    assert_eq!(
        (
            v["updated"].clone(),
            v["unchanged"].clone(),
            v["failed"].clone()
        ),
        (1.into(), 0.into(), 0.into())
    );
    let item = &v["items"][0];
    assert_eq!(item["app_id"], 9100610);
    assert_eq!(item["type"], "dlss");
    assert_eq!(item["outcome"], "changed");
    assert_eq!(item["from_version"], Value::Null);
    assert_eq!(item["to_version"], "310.5.3");
    assert_eq!(item["error"], Value::Null);
    assert_eq!(v["skipped"][0]["app_id"], 9100620);
    assert_eq!(
        v["skipped"][0]["reason"],
        "DLL swap denied: kernel anti-cheat"
    );
    assert_eq!(
        std::fs::read(lab.common.join("Birch/nvngx_dlss.dll")).unwrap(),
        ORIGINAL,
        "the denied game is untouched"
    );

    // Text, with a type filter.
    let run = lab.run(&["dll", "update", "--all", "--type", "dlss"]);
    run.expect_code(0);
    assert!(
        run.stdout()
            .starts_with("Amber Reach (9100610)\n  nvngx_dlss.dll: unknown -> 310.5.3\n"),
        "{}",
        run.stdout()
    );
    assert!(
        run.stdout()
            .contains("Skipped Birch Hollow (9100620): DLL swap denied: kernel anti-cheat\n"),
        "{}",
        run.stdout()
    );
}

#[test]
fn a_failed_item_under_json_is_the_error_object_with_the_report() {
    let lab = Lab::new();
    let run = lab.run(&["dll", "update", "Birch Hollow", "--json"]);
    run.expect_code(1);
    assert!(run.stderr().is_empty(), "{}", run.stderr());
    let v = json(&run);
    assert_eq!(v["schema_version"], 1);
    assert_eq!(v["error"]["code"], "dll_update_failed");
    let details = &v["error"]["details"];
    assert_eq!(details["updated"], 1);
    assert_eq!(details["failed"], 1);
    let failed = details["items"]
        .as_array()
        .unwrap()
        .iter()
        .find(|i| i["outcome"] == "failed")
        .unwrap();
    assert_eq!(failed["type"], "xess");
    assert_eq!(failed["error"]["code"], "download_failed");
    assert_eq!(failed["error"]["files_changed"], false);
}

#[test]
fn bad_arguments_are_usage_errors() {
    let lab = Lab::new();
    for args in [
        &["dll", "update"][..],
        &["dll", "update", "Amber Reach", "--all"][..],
        &["dll", "update", "Amber Reach", "dlss", "--type", "xess"][..],
        &["dll", "update", "Amber Reach", "--version", "310.5.3"][..],
        &["dll", "install", "Amber Reach"][..],
        &["dll", "cache", "nope"][..],
    ] {
        let run = lab.run(args);
        run.expect_code(2);
        assert!(run.stdout().is_empty(), "{args:?}");
    }
    let run = lab.run(&["dll", "install", "Amber Reach", "bogus", "--json"]);
    run.expect_code(2);
    let v = json(&run);
    assert_eq!(v["error"]["code"], "usage");
    assert!(
        v["error"]["message"]
            .as_str()
            .unwrap()
            .contains("unknown DLL type"),
        "{v}"
    );
}

#[test]
fn missing_games_dlls_and_backups_say_so() {
    let lab = Lab::new();
    let run = lab.run(&["dll", "update", "Amber Reach", "xess"]);
    run.expect_code(1);
    assert_eq!(run.stderr(), "error: game does not have a xess DLL\n");
    let run = lab.run(&["dll", "restore", "Amber Reach"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: restore backup: no backup found for app 9100610\n"
    );
    let run = lab.run(&["dll", "install", "Nowhere", "dlss"]);
    run.expect_code(1);
    assert!(
        run.stderr().starts_with("error: game not found: Nowhere\n"),
        "{}",
        run.stderr()
    );
    let run = lab.run(&["dll", "install", "Cinder Gate", "dlss", "--version", "9.9"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: no dlss version \"9.9\" available in manifest\n"
    );
}

#[test]
fn dll_cache_lists_versions_newest_first() {
    let lab = Lab::new();
    let run = lab.run(&["dll", "cache"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        format!(
            "DLL cache: {}\n  dlss: 310.5.3, 310.4.0\n  dlssg: none\n  dlssd: none\n  xess: none\n  fsr: none\n",
            lab.cache().join("dlls").display()
        )
    );
    let run = lab.run(&["dll", "cache", "dlss", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["types"].as_array().unwrap().len(), 1);
    assert_eq!(v["types"][0]["type"], "dlss");
    assert_eq!(v["types"][0]["versions"][0], "310.5.3");
}

#[test]
fn check_updates_prints_json_and_refresh_forces_a_fetch() {
    let lab = Lab::new();
    let run = lab.run(&["dll", "check-updates", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["manifest"]["source_url"], MANIFEST_URL);
    assert_eq!(v["manifest"]["fetched_now"], false);
    let names: Vec<&str> = v["games"]
        .as_array()
        .unwrap()
        .iter()
        .map(|g| g["name"].as_str().unwrap())
        .collect();
    assert_eq!(names, ["Amber Reach", "Birch Hollow"]);
    assert_eq!(v["games"][1]["dlls"][0]["type"], "dlss", "catalogue order");
    assert_eq!(v["games"][1]["dlls"][1]["latest"], "1.3.1");

    // --refresh asks the fake network, which fails, so the cache stands in.
    let run = lab.run(&["dll", "check-updates", "--refresh"]);
    run.expect_code(0);
    assert!(
        run.stderr().contains("using the cached DLL manifest"),
        "{}",
        run.stderr()
    );
    assert!(
        run.stdout().starts_with("Amber Reach (9100610)\n"),
        "{}",
        run.stdout()
    );
}
