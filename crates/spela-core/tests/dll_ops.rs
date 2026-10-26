//! DLL install, update, batch update and restore, end to end over fixture
//! libraries in temp dirs and the fake HTTP backend (Go's
//! `operations_test.go`, ported to the fixed behaviour).
//!
//! Every test hashes the whole temp tree before and after, and checks that
//! each changed file is one the operation may write: the game's own folder,
//! the backups, the DLL cache, the game cache or a lock. No request leaves
//! the fake: every URL is a loopback one the test serves.

mod common;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use common::{copy_fixture, fixtures};

use spela_core::clock::{Clock, ManualClock};
use spela_core::config::Config;
use spela_core::denylist;
use spela_core::dll::backup;
use spela_core::dll::detect;
use spela_core::dll::manifest::{Manifest, ManifestEntry};
use spela_core::dll::ops::{
    self, DllContext, FailKind, OpError, Outcome, ProgressEvent, Stage, UpdateRequest,
};
use spela_core::fake::FakeHttp;
use spela_core::games::{DllType, Game, cache};
use spela_core::hex;
use spela_core::paths::Paths;
use spela_core::pe;

/// A temp root with XDG dirs, one Steam library and a fake network.
struct Lab {
    _dir: tempfile::TempDir,
    root: PathBuf,
    paths: Paths,
    library: PathBuf,
    config: Config,
    http: FakeHttp,
    clock: ManualClock,
    manifest: Manifest,
}

/// Version 3.7.10 of `nvngx_dlss.dll`, a PE stub Go's goref wrote.
fn pe_3_7_10() -> Vec<u8> {
    std::fs::read(
        fixtures().join("games/database/library/steamapps/common/Harbor Lights/nvngx_dlss.dll"),
    )
    .unwrap()
}

/// Version 310.2.1 of `nvngx_dlss.dll`, a PE stub.
fn pe_310_2_1() -> Vec<u8> {
    std::fs::read(
        fixtures()
            .join("games/database/library/steamapps/common/Glimmer Run/bin/x64/nvngx_dlss.dll"),
    )
    .unwrap()
}

impl Lab {
    fn new() -> Lab {
        let dir = tempfile::tempdir().unwrap();
        let root = dir.path().to_owned();
        let library = root.join("library");
        std::fs::create_dir_all(library.join("steamapps/common")).unwrap();
        Lab {
            paths: Paths::under(&root.join("xdg")),
            _dir: dir,
            root,
            library,
            config: Config::default(),
            http: FakeHttp::new(),
            clock: ManualClock::new("2026-10-26T15:02:10Z".parse().unwrap()),
            manifest: Manifest::default(),
        }
    }

    fn cx(&self) -> DllContext<'_> {
        DllContext {
            paths: &self.paths,
            config: &self.config,
            http: &self.http,
            clock: &self.clock,
            stop: None,
        }
    }

    /// A game with these files, scanned into the game cache.
    fn game(&self, app_id: u64, name: &str, files: &[(&str, &[u8])]) -> PathBuf {
        let dir = self.library.join("steamapps/common").join(name);
        std::fs::create_dir_all(&dir).unwrap();
        for (rel, bytes) in files {
            let path = dir.join(rel);
            std::fs::create_dir_all(path.parent().unwrap()).unwrap();
            std::fs::write(path, bytes).unwrap();
        }
        let game = Game {
            app_id,
            name: name.into(),
            install_dir: dir.clone(),
            library_path: self.library.clone(),
            prefix_path: None,
            scanned_at: self.clock.now(),
            dlls: detect::scan_directory(&dir, pe::version_string).unwrap(),
        };
        cache::update(&self.paths, |c| c.upsert(game)).unwrap();
        dir
    }

    /// Adds a manifest entry whose payload the fake serves.
    fn offer(
        &mut self,
        dll_type: DllType,
        version: &str,
        filename: &str,
        body: &[u8],
    ) -> ManifestEntry {
        let url = format!("http://127.0.0.1:9/{dll_type}/{version}/{filename}");
        self.http.serve(&url, body.to_vec());
        let entry = ManifestEntry {
            version: version.into(),
            filename: filename.into(),
            url,
            sha256: hex::sha256_hex(body),
            size: body.len() as i64,
            ..ManifestEntry::default()
        };
        self.manifest
            .dlls
            .entry(dll_type.as_str().into())
            .or_default()
            .push(entry.clone());
        entry
    }

    fn game_of(&self, app_id: u64) -> Game {
        cache::load(&self.paths).get(app_id).unwrap().clone()
    }

    fn update_game(&self, app_id: u64, dll_type: Option<DllType>) -> ops::BatchResult {
        let requests =
            ops::requests_for(&cache::load(&self.paths), &[app_id], dll_type, None, false);
        ops::update(&self.cx(), &self.manifest, requests, &mut |_| {})
    }

    /// Every file under the root with its SHA-256; a symlink by its target.
    fn tree(&self) -> BTreeMap<PathBuf, String> {
        let mut out = BTreeMap::new();
        walk(&self.root, &mut out);
        out
    }

    /// The files that changed since `before`, checked against where an
    /// operation may write.
    fn changed_since(&self, before: &BTreeMap<PathBuf, String>) -> Vec<PathBuf> {
        let after = self.tree();
        let mut changed: Vec<PathBuf> = after
            .iter()
            .filter(|(p, h)| before.get(*p) != Some(h))
            .map(|(p, _)| p.clone())
            .chain(before.keys().filter(|p| !after.contains_key(*p)).cloned())
            .collect();
        changed.sort();
        changed.dedup();
        let allowed = [
            self.library.join("steamapps/common"),
            self.paths.backups_dir(),
            self.config.dll_cache_dir(&self.paths),
            self.paths.game_cache(),
            self.paths.locks_dir(),
        ];
        for path in &changed {
            assert!(
                allowed.iter().any(|a| path.starts_with(a)),
                "a write outside the library, backups, caches and locks: {}",
                path.display()
            );
        }
        changed
    }

    /// Only loopback URLs were asked for.
    fn no_remote_host(&self) {
        for url in self.http.urls() {
            assert!(url.starts_with("http://127.0.0.1:"), "{url}");
        }
    }
}

fn walk(dir: &Path, out: &mut BTreeMap<PathBuf, String>) {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        let kind = entry.file_type().unwrap();
        if kind.is_symlink() {
            out.insert(
                path.clone(),
                format!("link {:?}", std::fs::read_link(&path).unwrap()),
            );
        } else if kind.is_dir() {
            walk(&path, out);
        } else {
            out.insert(
                path.clone(),
                hex::sha256_hex(&std::fs::read(&path).unwrap()),
            );
        }
    }
}

fn read(path: &Path) -> Vec<u8> {
    std::fs::read(path).unwrap()
}

#[test]
fn update_backs_up_every_dll_swaps_bytes_and_restore_puts_them_back() {
    let mut lab = Lab::new();
    let dir = lab.game(
        9100501,
        "Glimmer Run",
        &[
            ("bin/nvngx_dlss.dll", &pe_3_7_10()),
            ("bin/nvngx_dlssg.dll", b"original framegen"),
        ],
    );
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    let before = lab.tree();

    let mut stages = Vec::new();
    let requests = ops::requests_for(
        &cache::load(&lab.paths),
        &[9100501],
        Some(DllType::Dlss),
        None,
        false,
    );
    let batch = ops::update(
        &lab.cx(),
        &lab.manifest,
        requests,
        &mut |e: &ProgressEvent| {
            if stages.last() != Some(&e.stage) {
                stages.push(e.stage);
            }
        },
    );
    assert_eq!(
        (batch.updated, batch.unchanged, batch.failed),
        (1, 0, 0),
        "{batch:?}"
    );
    assert_eq!(
        stages,
        [
            Stage::Resolving,
            Stage::Downloading,
            Stage::Updating,
            Stage::Scanning,
            Stage::Saving
        ]
    );
    let item = &batch.items[0];
    assert_eq!(item.from_version.as_deref(), Some("3.7.10"));
    assert_eq!(item.to_version.as_deref(), Some("310.2.1"));
    let result = item.result.as_ref().unwrap();
    assert!(result.files_changed && result.metadata_persisted);
    assert_eq!(read(&dir.join("bin/nvngx_dlss.dll")), pe_310_2_1());
    assert_eq!(read(&dir.join("bin/nvngx_dlssg.dll")), b"original framegen");
    let dlss = lab.game_of(9100501).dlls[0].clone();
    assert_eq!(
        dlss.version.as_deref(),
        Some("310.2.1"),
        "rescanned and saved"
    );

    // Every detected DLL is backed up on the first swap, by Go's names.
    let index = backup::load(&lab.paths, 9100501).unwrap().unwrap();
    assert_eq!(index.files.len(), 2);
    for f in &index.files {
        assert_eq!(
            f.backup_path.file_name().unwrap(),
            backup::hashed_backup_name(&f.original_path)
        );
    }
    assert_eq!(read(&index.files[0].backup_path), pe_3_7_10());
    assert_eq!(
        index.installed_sha256(&dir.join("bin/nvngx_dlss.dll")),
        Some(hex::sha256_hex(&pe_310_2_1()).as_str())
    );
    assert!(index.created_files.is_empty());
    let changed = lab.changed_since(&before);
    assert!(changed.contains(&dir.join("bin/nvngx_dlss.dll")));
    assert!(!changed.contains(&dir.join("bin/nvngx_dlssg.dll")));

    // Current now: a no-op with no request, scan or save.
    let requests_before = lab.http.requests().len();
    let again = lab.update_game(9100501, Some(DllType::Dlss));
    assert_eq!((again.updated, again.unchanged, again.failed), (0, 1, 0));
    assert_eq!(
        again.items[0].result.as_ref().unwrap().outcome,
        Outcome::NoOp
    );
    assert_eq!(lab.http.requests().len(), requests_before);

    // Restore puts the original bytes back and forgets the install.
    let restored = ops::restore(&lab.cx(), 9100501, &mut |_| {}).unwrap();
    assert_eq!(restored.restored.len(), 2);
    assert_eq!(read(&dir.join("bin/nvngx_dlss.dll")), pe_3_7_10());
    assert_eq!(
        lab.game_of(9100501).dlls[0].version.as_deref(),
        Some("3.7.10")
    );
    let index = backup::load(&lab.paths, 9100501).unwrap().unwrap();
    assert!(index.installed.is_empty());
    assert_eq!(index.files.len(), 2, "backups stay, as in Go");
    let game_files: Vec<PathBuf> = lab
        .changed_since(&before)
        .into_iter()
        .filter(|p| p.starts_with(&dir))
        .collect();
    assert!(
        game_files.is_empty(),
        "the game is byte for byte as before: {game_files:?}"
    );
    lab.no_remote_host();
}

#[test]
fn install_into_a_game_without_dlls_is_undone_by_restore() {
    let mut lab = Lab::new();
    let dir = lab.game(9100502, "Harbor Lights", &[("readme.txt", b"hello")]);
    lab.offer(DllType::Xess, "1.3.1", "libxess.dll", b"xess payload");
    let before = lab.tree();

    let installed = ops::install(
        &lab.cx(),
        &lab.manifest,
        9100502,
        DllType::Xess,
        None,
        &mut |_| {},
    )
    .unwrap();
    assert_eq!(installed.path, dir.join("libxess.dll"));
    assert_eq!(installed.from_version, None);
    assert_eq!(read(&dir.join("libxess.dll")), b"xess payload");
    let index = backup::load(&lab.paths, 9100502).unwrap().unwrap();
    assert!(index.files.is_empty());
    assert_eq!(index.created_files, [dir.join("libxess.dll")]);
    assert!(
        backup::has_backup(&lab.paths, 9100502),
        "Go's 'no backup found' is fixed"
    );

    let restored = ops::restore(&lab.cx(), 9100502, &mut |_| {}).unwrap();
    assert_eq!(restored.deleted, [dir.join("libxess.dll")]);
    assert!(!dir.join("libxess.dll").exists());
    assert!(
        !lab.paths.backup_index(9100502).exists(),
        "nothing left to restore"
    );
    assert!(lab.game_of(9100502).dlls.is_empty());
    let changed = lab.changed_since(&before);
    assert!(changed.iter().all(|p| !p.starts_with(&dir)), "{changed:?}");
}

#[test]
fn install_writes_over_a_detected_file_in_any_case_and_pins_versions() {
    let mut lab = Lab::new();
    let dir = lab.game(
        9100503,
        "Paper Comet",
        &[("Bin/NVNGX_DLSS.DLL", &pe_3_7_10())],
    );
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    lab.offer(DllType::Dlss, "3.5.0", "nvngx_dlss.dll", b"older");

    let installed = ops::install(
        &lab.cx(),
        &lab.manifest,
        9100503,
        DllType::Dlss,
        Some("3.5.0"),
        &mut |_| {},
    )
    .unwrap();
    assert_eq!(installed.path, dir.join("Bin/NVNGX_DLSS.DLL"));
    assert_eq!(installed.version, "3.5.0", "install takes an older version");
    assert_eq!(read(&dir.join("Bin/NVNGX_DLSS.DLL")), b"older");
    assert!(
        !dir.join("nvngx_dlss.dll").exists(),
        "no second, lowercase file"
    );
    assert!(!dir.join("Bin/nvngx_dlss.dll").exists());

    let err = ops::install(
        &lab.cx(),
        &lab.manifest,
        9100503,
        DllType::Dlss,
        Some("9.9"),
        &mut |_| {},
    )
    .unwrap_err();
    assert_eq!(
        err.to_string(),
        "no dlss version \"9.9\" available in manifest"
    );
}

#[test]
fn checksum_denylist_and_validation_failures_change_nothing() {
    let mut lab = Lab::new();
    let dir = lab.game(9100504, "Lantern Keep", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    let mut entry = lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    // The served bytes are not the manifest's.
    lab.http.serve(&entry.url, b"tampered".to_vec());
    let before = lab.tree();
    let batch = lab.update_game(9100504, None);
    assert_eq!(batch.failed, 1);
    let err = batch.items[0].result.as_ref().unwrap_err();
    assert!(
        err.to_string()
            .starts_with("acquire dlss 310.2.1: cache DLL: checksum mismatch"),
        "{err}"
    );
    assert!(!err.files_changed());
    assert!(
        lab.changed_since(&before).is_empty(),
        "no backup, no cache file, no swap"
    );

    // A deny-listed game: refused before the download.
    lab.http.serve(&entry.url, pe_310_2_1());
    denylist::deny(&lab.paths, 9100504, "Lantern Keep", "Easy Anti-Cheat").unwrap();
    let before = lab.tree();
    let requests_before = lab.http.requests().len();
    let batch = lab.update_game(9100504, None);
    let err = batch.items[0].result.as_ref().unwrap_err();
    assert_eq!(
        err.to_string(),
        "DLL swap denied for app 9100504: Easy Anti-Cheat"
    );
    assert!(matches!(
        err,
        OpError::Failed {
            kind: FailKind::Denied,
            ..
        }
    ));
    assert_eq!(lab.http.requests().len(), requests_before);
    assert!(lab.changed_since(&before).is_empty());

    // A deny list that does not load fails closed.
    std::fs::write(lab.paths.deny_overrides(), "allowed = [").unwrap();
    let batch = lab.update_game(9100504, None);
    let err = batch.items[0].result.as_ref().unwrap_err().to_string();
    assert!(
        err.starts_with("DLL install and update are refused until the deny list loads"),
        "{err}"
    );
    std::fs::remove_file(lab.paths.deny_overrides()).unwrap();

    // A manifest file name that is a path, or another family's: refused
    // before any request.
    for bad in ["../nvngx_dlss.dll", "dlss.dll", "nvngx_dlssg.dll"] {
        lab.manifest.dlls.clear();
        entry.filename = bad.into();
        lab.manifest.dlls.insert("dlss".into(), vec![entry.clone()]);
        let requests_before = lab.http.requests().len();
        let batch = lab.update_game(9100504, None);
        let err = batch.items[0].result.as_ref().unwrap_err().to_string();
        assert!(err.contains("manifest filename"), "{bad}: {err}");
        let err = ops::install(
            &lab.cx(),
            &lab.manifest,
            9100504,
            DllType::Dlss,
            None,
            &mut |_| {},
        )
        .unwrap_err()
        .to_string();
        assert!(err.contains("manifest filename"), "{bad}: {err}");
        assert_eq!(lab.http.requests().len(), requests_before, "{bad}");
    }
    assert_eq!(read(&dir.join("nvngx_dlss.dll")), pe_3_7_10());
}

#[test]
fn an_fsr_payload_only_goes_to_the_file_it_was_made_for() {
    let mut lab = Lab::new();
    let dir = lab.game(
        9100505,
        "Ironleaf",
        &[("amd_fidelityfx_dx12.dll", b"dx12 original")],
    );
    lab.offer(DllType::Fsr, "1.1", "amd_fidelityfx_vk.dll", b"vk payload");
    let batch = lab.update_game(9100505, Some(DllType::Fsr));
    let err = batch.items[0].result.as_ref().unwrap_err().to_string();
    assert_eq!(
        err,
        "the fsr 1.1 manifest file amd_fidelityfx_vk.dll is not amd_fidelityfx_dx12.dll"
    );
    assert_eq!(read(&dir.join("amd_fidelityfx_dx12.dll")), b"dx12 original");
    assert!(lab.http.requests().is_empty());
}

#[test]
fn cached_only_needs_a_version_and_never_downloads() {
    let mut lab = Lab::new();
    let dir = lab.game(9100506, "Quarry", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    let entry = lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    let request = |version: Option<&str>| UpdateRequest {
        app_id: 9100506,
        dll_type: DllType::Dlss,
        path: dir.join("nvngx_dlss.dll"),
        version: version.map(str::to_owned),
        cached_only: true,
    };
    let batch = ops::update(&lab.cx(), &lab.manifest, vec![request(None)], &mut |_| {});
    assert_eq!(
        batch.items[0].result.as_ref().unwrap_err().to_string(),
        "cached DLL version is required"
    );
    let batch = ops::update(
        &lab.cx(),
        &lab.manifest,
        vec![request(Some("310.2.1"))],
        &mut |_| {},
    );
    let err = batch.items[0].result.as_ref().unwrap_err().to_string();
    assert!(
        err.starts_with("acquire dlss 310.2.1: cached DLL not found: "),
        "{err}"
    );

    let cached = spela_core::dll::manifest::cache_file(
        &lab.config.dll_cache_dir(&lab.paths),
        DllType::Dlss,
        "310.2.1",
    )
    .unwrap();
    std::fs::create_dir_all(cached.parent().unwrap()).unwrap();
    std::fs::write(&cached, pe_310_2_1()).unwrap();
    let batch = ops::update(
        &lab.cx(),
        &lab.manifest,
        vec![request(Some("310.2.1"))],
        &mut |_| {},
    );
    assert_eq!(batch.updated, 1, "{batch:?}");
    assert_eq!(read(&dir.join("nvngx_dlss.dll")), pe_310_2_1());
    assert!(
        lab.http.requests().is_empty(),
        "nothing asked of {}",
        entry.url
    );
}

#[test]
fn a_batch_carries_on_after_a_failure() {
    let mut lab = Lab::new();
    let a = lab.game(9100507, "Alpha", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    let b = lab.game(
        9100508,
        "Beta",
        &[("nvngx_dlss.dll", &pe_3_7_10()), ("libxess.dll", b"xess")],
    );
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    denylist::deny(&lab.paths, 9100507, "Alpha", "anti-cheat").unwrap();
    let requests = ops::requests_for(
        &cache::load(&lab.paths),
        &[9100507, 9100508, 42],
        None,
        None,
        false,
    );
    assert_eq!(
        requests.len(),
        4,
        "Alpha's dlss, Beta's dlss and xess, and the unknown game"
    );
    let batch = ops::update(&lab.cx(), &lab.manifest, requests, &mut |_| {});
    assert_eq!(
        (batch.updated, batch.unchanged, batch.failed),
        (1, 0, 3),
        "{batch:#?}"
    );
    assert_eq!(read(&a.join("nvngx_dlss.dll")), pe_3_7_10());
    assert_eq!(read(&b.join("nvngx_dlss.dll")), pe_310_2_1());
    let errors: Vec<String> = batch
        .items
        .iter()
        .filter_map(|i| i.result.as_ref().err().map(|e| e.to_string()))
        .collect();
    assert_eq!(
        errors,
        [
            "DLL swap denied for app 9100507: anti-cheat",
            "no xess version \"\" available in manifest",
            "game not found: 42",
        ]
    );
}

#[test]
fn a_save_failure_is_a_partial_failure_and_files_stay_changed() {
    use std::os::unix::fs::PermissionsExt;
    let mut lab = Lab::new();
    lab.config.dll_cache_path = lab.root.join("dll-cache").display().to_string();
    let dir = lab.game(9100509, "Gamma", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    // The game cache's folder is read only: the cache loads but cannot save.
    let mode = |m| std::fs::Permissions::from_mode(m);
    std::fs::set_permissions(&lab.paths.cache_dir, mode(0o555)).unwrap();
    let batch = lab.update_game(9100509, None);
    std::fs::set_permissions(&lab.paths.cache_dir, mode(0o755)).unwrap();
    assert_eq!(batch.failed, 1, "{batch:#?}");
    let err = batch.items[0].result.as_ref().unwrap_err();
    assert!(
        matches!(
            err,
            OpError::Partial {
                stage: Stage::Saving,
                ..
            }
        ),
        "{err:?}"
    );
    assert!(
        err.to_string()
            .starts_with("DLL files changed but metadata did not fully persist during saving: save game database: "),
        "{err}"
    );
    assert!(err.files_changed());
    assert_eq!(
        read(&dir.join("nvngx_dlss.dll")),
        pe_310_2_1(),
        "no roll back"
    );
    assert_eq!(
        lab.game_of(9100509).dlls[0].version.as_deref(),
        Some("3.7.10"),
        "the cache is stale"
    );
}

#[test]
fn a_steam_update_refreshes_the_backup_before_the_next_swap() {
    let mut lab = Lab::new();
    let dir = lab.game(9100510, "Delta", &[("nvngx_dlss.dll", b"steam 1")]);
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    assert_eq!(lab.update_game(9100510, None).updated, 1);
    // Steam ships its own update, older than spela's but a new original.
    std::fs::write(dir.join("nvngx_dlss.dll"), pe_3_7_10()).unwrap();
    lab.game(9100510, "Delta", &[]);
    assert_eq!(lab.update_game(9100510, None).updated, 1);
    ops::restore(&lab.cx(), 9100510, &mut |_| {}).unwrap();
    assert_eq!(
        read(&dir.join("nvngx_dlss.dll")),
        pe_3_7_10(),
        "restore gives Steam's file, not the first original"
    );
}

#[test]
fn restore_without_a_backup_or_game_says_so() {
    let lab = Lab::new();
    lab.game(9100511, "Echo", &[("nvngx_dlss.dll", b"x")]);
    let err = ops::restore(&lab.cx(), 9100511, &mut |_| {}).unwrap_err();
    assert_eq!(
        err.to_string(),
        "restore backup: no backup found for app 9100511"
    );
    assert!(matches!(
        err,
        OpError::Failed {
            kind: FailKind::NotFound,
            ..
        }
    ));
    assert_eq!(
        ops::restore(&lab.cx(), 1, &mut |_| {})
            .unwrap_err()
            .to_string(),
        "game not found: 1"
    );
}

#[test]
fn a_symlinked_dll_or_folder_is_never_written_through() {
    let mut lab = Lab::new();
    let outside = lab.root.join("outside");
    std::fs::create_dir_all(&outside).unwrap();
    std::fs::write(outside.join("nvngx_dlss.dll"), b"not the game's").unwrap();
    let dir = lab.library.join("steamapps/common/Foxtrot");
    std::fs::create_dir_all(&dir).unwrap();
    std::os::unix::fs::symlink(outside.join("nvngx_dlss.dll"), dir.join("nvngx_dlss.dll")).unwrap();
    lab.game(9100512, "Foxtrot", &[]);
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    let before = lab.tree();
    let batch = lab.update_game(9100512, None);
    assert_eq!(batch.failed, 1, "{batch:?}");
    let err = ops::install(
        &lab.cx(),
        &lab.manifest,
        9100512,
        DllType::Dlss,
        None,
        &mut |_| {},
    )
    .unwrap_err();
    assert!(err.to_string().contains("symbolic link"), "{err}");
    assert_eq!(read(&outside.join("nvngx_dlss.dll")), b"not the game's");
    let changed = lab.changed_since(&before);
    assert!(
        changed
            .iter()
            .all(|p| !p.starts_with(&dir) && !p.starts_with(&outside)),
        "{changed:?}"
    );
}

/// The Phase 5 proof: each Go-era backup restores byte-exact, Go's index
/// stays byte-identical, and every write lands inside the fixture library.
#[test]
fn go_era_backups_restore_byte_exact() {
    let cases = [
        ("backups/sha256-names", 9100401, "Glimmer Run"),
        ("backups/basename-names", 9100402, "Harbor Lights"),
        ("backups/mixed-names", 9100403, "Lantern Keep"),
        ("backups/custom-backup-path", 9100404, "Paper Comet"),
    ];
    for (name, app_id, game_name) in cases {
        let lab = Lab::new();
        let case = lab.root.join("case");
        copy_fixture(&fixtures().join(name), &case, case.to_str().unwrap());
        let paths = Paths {
            config_dir: case.join("config/spela"),
            data_dir: case.join("data/spela"),
            cache_dir: lab.root.join("cache"),
            state_dir: lab.root.join("state"),
        };
        let library = case.join("library");
        let dir = library.join("steamapps/common").join(game_name);
        let game = Game {
            app_id,
            name: game_name.into(),
            install_dir: dir.clone(),
            library_path: library.clone(),
            prefix_path: None,
            scanned_at: lab.clock.now(),
            dlls: detect::scan_directory(&dir, pe::version_string).unwrap(),
        };
        cache::update(&paths, |c| c.upsert(game)).unwrap();
        let index_bytes = read(&paths.backup_index(app_id));
        let index = backup::load(&paths, app_id).unwrap().unwrap();
        // Every game file differs from its backup before the restore, so
        // the restore has to bring each one back.
        for file in &index.files {
            if read(&file.original_path) == read(&file.backup_path) {
                std::fs::write(&file.original_path, b"swapped by the test").unwrap();
            }
            assert_ne!(read(&file.original_path), read(&file.backup_path), "{name}");
        }
        let mut before = BTreeMap::new();
        walk(&lab.root, &mut before);

        let cx = DllContext {
            paths: &paths,
            config: &lab.config,
            http: &lab.http,
            clock: &lab.clock,
            stop: None,
        };
        let restored = ops::restore(&cx, app_id, &mut |_| {}).unwrap();
        assert_eq!(restored.restored.len(), index.files.len(), "{name}");
        for file in &index.files {
            assert_eq!(
                read(&file.original_path),
                read(&file.backup_path),
                "{name}: {}",
                file.dll_name
            );
        }
        assert_eq!(
            read(&paths.backup_index(app_id)),
            index_bytes,
            "{name}: backup.json untouched"
        );

        let mut after = BTreeMap::new();
        walk(&lab.root, &mut after);
        for (path, hash) in &after {
            if before.get(path) == Some(hash) {
                continue;
            }
            assert!(
                path.starts_with(&dir)
                    || path.starts_with(&paths.cache_dir)
                    || path.starts_with(&paths.state_dir),
                "{name}: wrote {}",
                path.display()
            );
        }
        assert!(lab.http.requests().is_empty());
    }
}

/// A Go-era backup survives a Rust update: the first original is kept,
/// restore brings it back, and `backup.json` ends as Go wrote it.
#[test]
fn a_rust_update_over_a_go_backup_restores_go_s_original() {
    let mut lab = Lab::new();
    let case = lab.root.join("case");
    copy_fixture(
        &fixtures().join("backups/basename-names"),
        &case,
        case.to_str().unwrap(),
    );
    lab.paths = Paths {
        config_dir: case.join("config/spela"),
        data_dir: case.join("data/spela"),
        cache_dir: lab.root.join("cache"),
        state_dir: lab.root.join("state"),
    };
    let dir = case.join("library/steamapps/common/Harbor Lights");
    let game = Game {
        app_id: 9100402,
        name: "Harbor Lights".into(),
        install_dir: dir.clone(),
        library_path: case.join("library"),
        prefix_path: None,
        scanned_at: lab.clock.now(),
        dlls: detect::scan_directory(&dir, pe::version_string).unwrap(),
    };
    cache::update(&lab.paths, |c| c.upsert(game)).unwrap();
    let go_index = read(&lab.paths.backup_index(9100402));
    let original = read(&backup::load(&lab.paths, 9100402).unwrap().unwrap().files[0].backup_path);
    lab.offer(DllType::Dlss, "310.9.9", "nvngx_dlss.dll", b"rust payload");

    let batch = lab.update_game(9100402, None);
    assert_eq!(batch.updated, 1, "{batch:#?}");
    let index = backup::load(&lab.paths, 9100402).unwrap().unwrap();
    assert_eq!(
        index.files.len(),
        1,
        "Go's entry is kept, nothing new in files"
    );
    assert!(
        index
            .installed_sha256(&dir.join("nvngx_dlss.dll"))
            .is_some()
    );
    let raw: serde_json::Value =
        serde_json::from_slice(&read(&lab.paths.backup_index(9100402))).unwrap();
    let go: serde_json::Value = serde_json::from_slice(&go_index).unwrap();
    for key in ["app_id", "game_name", "created_at", "backup_path", "files"] {
        assert_eq!(raw[key], go[key], "{key}");
    }

    ops::restore(&lab.cx(), 9100402, &mut |_| {}).unwrap();
    assert_eq!(read(&dir.join("nvngx_dlss.dll")), original);
    assert_eq!(
        String::from_utf8(read(&lab.paths.backup_index(9100402))).unwrap(),
        String::from_utf8(go_index).unwrap(),
        "after the restore, backup.json is byte for byte Go's again"
    );
}

/// Go recorded the original paths by one route to the game; the game cache
/// may reach it by another, such as a symlinked library. Restore still puts
/// the originals back, byte-exact.
#[test]
fn a_go_backup_restores_through_another_route_to_the_library() {
    let lab = Lab::new();
    let case = lab.root.join("case");
    copy_fixture(
        &fixtures().join("backups/sha256-names"),
        &case,
        case.to_str().unwrap(),
    );
    let alias = lab.root.join("alias");
    std::os::unix::fs::symlink(case.join("library"), &alias).unwrap();
    let paths = Paths {
        config_dir: case.join("config/spela"),
        data_dir: case.join("data/spela"),
        cache_dir: lab.root.join("cache"),
        state_dir: lab.root.join("state"),
    };
    let dir = alias.join("steamapps/common/Glimmer Run");
    let game = Game {
        app_id: 9100401,
        name: "Glimmer Run".into(),
        install_dir: dir.clone(),
        library_path: alias.clone(),
        prefix_path: None,
        scanned_at: lab.clock.now(),
        dlls: detect::scan_directory(&dir, pe::version_string).unwrap(),
    };
    cache::update(&paths, |c| c.upsert(game)).unwrap();
    let index = backup::load(&paths, 9100401).unwrap().unwrap();
    for file in &index.files {
        assert!(file.original_path.starts_with(case.join("library")));
        std::fs::write(&file.original_path, b"swapped by the test").unwrap();
    }
    let cx = DllContext {
        paths: &paths,
        config: &lab.config,
        http: &lab.http,
        clock: &lab.clock,
        stop: None,
    };
    let restored = ops::restore(&cx, 9100401, &mut |_| {}).unwrap();
    assert_eq!(restored.restored.len(), 3);
    for file in &index.files {
        assert_eq!(
            read(&file.original_path),
            read(&file.backup_path),
            "{}",
            file.dll_name
        );
    }

    // A game that moved to another library is still refused.
    let elsewhere = lab.root.join("moved/steamapps/common/Glimmer Run");
    std::fs::create_dir_all(&elsewhere).unwrap();
    cache::update(&paths, |c| {
        let mut g = c.get(9100401).unwrap().clone();
        g.install_dir = elsewhere.clone();
        c.upsert(g);
    })
    .unwrap();
    let err = ops::restore(&cx, 9100401, &mut |_| {})
        .unwrap_err()
        .to_string();
    assert!(err.contains("escapes install directory"), "{err}");
}

#[test]
fn a_stop_ends_a_batch_between_items_and_saves_what_changed() {
    use std::sync::atomic::{AtomicBool, Ordering};
    let mut lab = Lab::new();
    let first = lab.game(9100511, "Amber Reach", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    let second = lab.game(9100512, "Birch Hollow", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    let requests = ops::requests_for(
        &cache::load(&lab.paths),
        &[9100511, 9100512],
        None,
        None,
        false,
    );
    let stop = AtomicBool::new(false);
    let cx = DllContext {
        stop: Some(&stop),
        ..lab.cx()
    };
    // The signal comes while the first game's file is being swapped.
    let batch = ops::update(&cx, &lab.manifest, requests, &mut |e: &ProgressEvent| {
        if e.stage == Stage::Updating {
            stop.store(true, Ordering::SeqCst);
        }
    });
    assert_eq!((batch.updated, batch.failed), (1, 1), "{batch:?}");
    assert!(batch.items[0].result.as_ref().unwrap().metadata_persisted);
    let Err(OpError::Failed { kind, message }) = &batch.items[1].result else {
        panic!("{:?}", batch.items[1].result)
    };
    assert_eq!((*kind, message.as_str()), (FailKind::Stopped, ops::STOPPED));
    assert_eq!(read(&first.join("nvngx_dlss.dll")), pe_310_2_1());
    assert_eq!(read(&second.join("nvngx_dlss.dll")), pe_3_7_10());
    assert_eq!(
        lab.game_of(9100511).dlls[0].version.as_deref(),
        Some("310.2.1"),
        "the swap that ran is saved in the game cache"
    );
    assert!(backup::load(&lab.paths, 9100512).unwrap().is_none());

    // A stop during a download keeps nothing and touches no game.
    lab.offer(
        DllType::Dlss,
        "310.5.3",
        "nvngx_dlss.dll",
        &vec![7u8; 100_000],
    );
    let before = lab.tree();
    let stop = AtomicBool::new(false);
    let cx = DllContext {
        stop: Some(&stop),
        ..lab.cx()
    };
    let err = ops::install(
        &cx,
        &lab.manifest,
        9100512,
        DllType::Dlss,
        Some("310.5.3"),
        &mut |e| {
            if e.stage == Stage::Downloading && e.downloaded > 0 {
                stop.store(true, Ordering::SeqCst);
            }
        },
    )
    .unwrap_err();
    assert!(
        matches!(
            err,
            OpError::Failed {
                kind: FailKind::Stopped,
                ..
            }
        ),
        "{err}"
    );
    let changed = lab.changed_since(&before);
    assert!(
        changed.is_empty(),
        "no game file and no cache file: {changed:?}"
    );
    lab.no_remote_host();
}

#[test]
fn auto_update_skips_unknown_denied_and_running_games() {
    use spela_core::dll::auto;
    let mut lab = Lab::new();
    lab.game(9100521, "Amber Reach", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    lab.game(9100522, "Birch Hollow", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    lab.game(
        9100523,
        "Cobalt Drift",
        &[("nvngx_dlss.dll", b"no version")],
    );
    lab.game(9100524, "Dune Sea", &[("nvngx_dlss.dll", &pe_3_7_10())]);
    lab.offer(DllType::Dlss, "310.2.1", "nvngx_dlss.dll", &pe_310_2_1());
    denylist::deny(&lab.paths, 9100524, "Dune Sea", "fixture anti-cheat").unwrap();
    let lists = denylist::load(&lab.paths).unwrap();
    let cache = cache::load(&lab.paths);
    let mut asked = Vec::new();
    let done = auto::auto_update(
        &lab.cx(),
        &cache,
        &lab.manifest,
        &lists,
        &mut |games| {
            asked = games.iter().map(|g| g.app_id).collect();
            [9100522].into()
        },
        &mut |_| {},
    );
    assert_eq!(
        asked,
        [9100521, 9100522],
        "only stale, allowed games are asked about"
    );
    assert_eq!(done.running, ["Birch Hollow"]);
    assert_eq!((done.batch.updated, done.batch.failed), (1, 0));
    assert_eq!(
        done.notice(),
        "DLL auto-update: updated 1 DLL; skipped Birch Hollow while running"
    );
    assert_eq!(
        lab.game_of(9100521).dlls[0].version.as_deref(),
        Some("310.2.1")
    );
    for id in [9100522, 9100523, 9100524] {
        assert!(backup::load(&lab.paths, id).unwrap().is_none(), "{id}");
    }
    lab.no_remote_host();
}
