//! The DLL flows (TUI 12): the game's DLLs section with its Install,
//! Update and Restore, the Library's batch update, the Catalog's
//! update-all, the startup auto-update and release notice, progress words,
//! confirmations that open on Cancel, Quit refused while a job runs, and a
//! stop signal that waits for the job's step. Frames at 120x40 and 80x24.

mod common;

use std::collections::BTreeMap;
use std::path::PathBuf;

use common::{check_keys, loaded};
use spela_core::dll::auto::AutoUpdate;
use spela_core::dll::backup::RestoreTarget;
use spela_core::dll::manifest::{CachedManifest, Manifest, ManifestEntry};
use spela_core::dll::ops::{
    BatchItem, BatchResult, Installed, OpResult, Outcome, ProgressEvent, Restored, Stage,
};
use spela_core::games::{DllType, GameCache};
use spela_ui::AppState;
use spela_ui::dll_jobs::{DllDone, DllOp, DllResult, QUITTING};
use spela_ui::event::{Cmd, Event, Msg};
use spela_ui::layout::Pane;
use spela_ui::screens::confirm::Focus;
use spela_ui::screens::{Popup, denylist, dlls, game_dlls};
use spela_ui::testing::{empty_denylist, key, render_text};
use spela_ui::update;

fn entry(version: &str) -> ManifestEntry {
    ManifestEntry {
        version: version.into(),
        filename: "nvngx_dlss.dll".into(),
        url: format!("https://dlls.spela-demo.invalid/dlss/{version}/nvngx_dlss.dll"),
        sha256: "ab".repeat(32),
        ..ManifestEntry::default()
    }
}

fn catalog() -> dlls::Catalog {
    let mut m = Manifest::default();
    m.dlls
        .insert("dlss".into(), vec![entry("310.4.0"), entry("310.5.3")]);
    dlls::Catalog {
        manifest: Ok(Some(CachedManifest {
            manifest: m,
            ..CachedManifest::default()
        })),
        cached: BTreeMap::from([(DllType::Dlss, Ok(vec!["310.4.0".to_owned()]))]),
        note: None,
    }
}

fn answer_catalog(s: &mut AppState) {
    let cmds = dlls::reload(s);
    let Cmd::Dlls(dlls::Cmd::Load { request }) = cmds[0] else {
        panic!("{cmds:?}")
    };
    update(
        s,
        Event::Msg(Msg::Dlls(dlls::Msg::Loaded {
            request,
            result: Ok(catalog()),
        })),
    );
}

/// Answers every backup read in `cmds` with `targets`.
fn answer_backups(s: &mut AppState, cmds: &[Cmd], targets: &[RestoreTarget]) {
    for c in cmds {
        if let Cmd::Dlls(dlls::Cmd::Backup { request, app_id }) = c {
            update(
                s,
                Event::Msg(Msg::Dlls(dlls::Msg::Backup {
                    request: *request,
                    app_id: *app_id,
                    result: Ok(targets.to_vec()),
                })),
            );
        }
    }
}

fn backed_up() -> Vec<RestoreTarget> {
    vec![RestoreTarget {
        path: PathBuf::from("/fixture/steamapps/common/Ashfall Courier/nvngx_dlss.dll"),
        dll_name: "nvngx_dlss.dll".into(),
        version: "310.1".into(),
        deletes: false,
    }]
}

/// Twelve games with profiles, the manifest (DLSS 310.5.3 latest, so the
/// even games' 310.4.0 is stale), an empty deny list and no scan.
fn ready(width: u16, height: u16) -> AppState {
    let mut s = loaded(width, height);
    answer_catalog(&mut s);
    let cmds = denylist::start(&mut s);
    empty_denylist(&mut s, &cmds);
    s
}

/// The first game's DLLs section, focused, with its backup read.
fn section(width: u16, height: u16, backup: &[RestoreTarget]) -> AppState {
    let mut s = ready(width, height);
    update(&mut s, key("enter"));
    s.profile.field = spela_core::profile::FieldKey::CpuSmt;
    let cmds = update(&mut s, key("right"));
    answer_backups(&mut s, &cmds, backup);
    assert_eq!(s.focus, Pane::Detail);
    s
}

fn dll_cmd(cmds: &[Cmd]) -> (u64, DllOp) {
    cmds.iter()
        .find_map(|c| match c {
            Cmd::Dll { token, op } => Some((*token, op.clone())),
            _ => None,
        })
        .unwrap_or_else(|| panic!("no DLL job in {cmds:?}"))
}

fn confirm_now(s: &mut AppState) -> Vec<Cmd> {
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(c.focus, Focus::Cancel, "it opens on Cancel");
    update(s, key("right"));
    update(s, key("enter"))
}

fn status(s: &AppState) -> String {
    s.status
        .as_ref()
        .map(|m| m.text.clone())
        .unwrap_or_default()
}

fn changed() -> OpResult {
    OpResult {
        outcome: Outcome::Changed,
        files_changed: true,
        metadata_persisted: true,
        game: None,
    }
}

fn batch_of(s: &AppState, op: &DllOp) -> BatchResult {
    let DllOp::Update { requests } = op else {
        panic!("{op:?}")
    };
    let items: Vec<BatchItem> = requests
        .iter()
        .map(|r| BatchItem {
            request: r.clone(),
            game_name: s.games.get(r.app_id).unwrap().name.clone(),
            from_version: Some("310.4.0".into()),
            to_version: r.version.clone(),
            result: Ok(changed()),
        })
        .collect();
    BatchResult {
        updated: items.len(),
        items,
        unchanged: 0,
        failed: 0,
    }
}

/// The game cache after a job that brought every DLSS to 310.5.3.
fn updated_cache(s: &AppState) -> GameCache {
    let mut cache = s.games.clone();
    for g in &mut cache.games {
        for d in &mut g.dlls {
            d.version = Some("310.5.3".into());
        }
    }
    cache
}

fn finish(s: &mut AppState, token: u64, result: DllResult, games: Option<GameCache>) -> Vec<Cmd> {
    update(
        s,
        Event::Msg(Msg::DllDone {
            token,
            done: Box::new(DllDone { result, games }),
        }),
    )
}

#[test]
fn the_dlls_section_shows_versions_and_actions_at_both_sizes() {
    for (w, h) in [(120, 40), (80, 24)] {
        let mut s = section(w, h, &backed_up());
        let text = render_text(&mut s);
        assert!(text.contains("DLL actions: Ashfall Courier"), "{text}");
        assert!(text.contains("Update DLLs (1 file)"), "{text}");
        assert!(text.contains("Restore originals (1 file)"), "{text}");
        assert!(text.contains("stale"), "{text}");
        check_keys(&format!("{w}x{h} DLLs section"), &s);
        insta::assert_snapshot!(format!("section_{w}x{h}"), text);
    }
}

#[test]
fn update_asks_on_cancel_then_runs_and_reports_progress_as_words() {
    let mut s = section(120, 40, &[]);
    let text = render_text(&mut s);
    assert!(text.contains("Restore originals (0 files)"), "{text}");
    assert!(text.contains("no backup for this game"), "{text}");
    update(&mut s, key("down"));
    update(&mut s, key("enter"));
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(c.title, "Confirm DLL update");
    insta::assert_snapshot!("confirm_update_120x40", render_text(&mut s));
    // Enter on Cancel runs nothing.
    let cmds = update(&mut s, key("enter"));
    assert!(cmds.iter().all(|c| !matches!(c, Cmd::Dll { .. })));
    assert_eq!(status(&s), "DLL update cancelled, no operation ran");

    update(&mut s, key("enter"));
    let cmds = confirm_now(&mut s);
    let (token, op) = dll_cmd(&cmds);
    let DllOp::Update { requests } = &op else {
        panic!("{op:?}")
    };
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0].version.as_deref(), Some("310.5.3"));
    assert!(!requests[0].cached_only, "a game's update downloads");
    assert_eq!(s.jobs.dll, Some(token));

    // Quit waits for the job, and so do Rescan and every DLL row.
    update(&mut s, key("Q"));
    assert!(!s.quit);
    assert_eq!(status(&s), "wait for pending operations");
    let rows = spela_ui::screens::actions(&s, Pane::Detail);
    let rescan = rows.iter().find(|r| r.label == "Rescan games").unwrap();
    assert_eq!(rescan.unavailable.as_deref(), Some("wait for pending work"));

    // Progress is a word on the status line, one per stage.
    let event = ProgressEvent {
        stage: Stage::Downloading,
        app_id: s.games.games[0].app_id,
        dll_type: Some(DllType::Dlss),
        version: "310.5.3".into(),
        filename: String::new(),
        downloaded: 0,
        total: None,
    };
    s.dirty = false;
    update(
        &mut s,
        Event::Msg(Msg::DllProgress {
            token,
            event: event.clone(),
        }),
    );
    assert!(s.dirty);
    s.dirty = false;
    update(
        &mut s,
        Event::Msg(Msg::DllProgress {
            token,
            event: ProgressEvent {
                downloaded: 4096,
                ..event.clone()
            },
        }),
    );
    assert!(!s.dirty, "the same words draw nothing");
    // A stale token is dropped.
    update(
        &mut s,
        Event::Msg(Msg::DllProgress {
            token: token + 99,
            event: ProgressEvent {
                stage: Stage::Saving,
                ..event
            },
        }),
    );
    let text = render_text(&mut s);
    assert!(text.contains("Downloading DLSS 310.5.3..."), "{text}");
    assert!(
        text.contains("Work is running. Controls return when it finishes."),
        "{text}"
    );
    insta::assert_snapshot!("update_running_120x40", text);

    let batch = batch_of(&s, &op);
    let games = updated_cache(&s);
    finish(
        &mut s,
        token + 1,
        DllResult::Updated(Ok(batch.clone())),
        None,
    );
    assert_eq!(s.jobs.dll, Some(token), "a stale result is dropped");
    let cmds = finish(&mut s, token, DllResult::Updated(Ok(batch)), Some(games));
    assert_eq!(s.jobs.dll, None);
    assert_eq!(status(&s), "DLL update: 1 updated, 0 current, 0 failed");
    assert!(
        cmds.iter()
            .any(|c| matches!(c, Cmd::Dlls(dlls::Cmd::Load { .. }))),
        "the Catalog reads its caches again"
    );
    answer_backups(&mut s, &cmds, &backed_up());
    let text = render_text(&mut s);
    assert!(text.contains("310.5.3"), "the new version shows: {text}");
    assert!(text.contains("Update DLLs (0 files)"), "{text}");
    insta::assert_snapshot!("update_done_120x40", text);
}

#[test]
fn install_picks_a_family_then_a_version_then_asks() {
    let mut s = section(120, 40, &[]);
    update(&mut s, key("enter"));
    assert!(game_dlls::editing(&s));
    check_keys("family picker", &s);
    let text = render_text(&mut s);
    assert!(text.contains("Select a supported DLL family"), "{text}");
    // Letters type into the picker; q never quits there.
    update(&mut s, key("q"));
    assert!(!s.quit && game_dlls::editing(&s));
    update(&mut s, key("enter"));
    let text = render_text(&mut s);
    assert!(text.contains("Select DLSS version"), "{text}");
    assert!(text.contains("310.5.3 (latest)"), "{text}");
    insta::assert_snapshot!("install_versions_120x40", text);
    check_keys("version picker", &s);
    update(&mut s, key("enter"));
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(c.title, "Confirm DLL install");
    let body: Vec<String> = c
        .body
        .iter()
        .map(|l| l.iter().map(|(t, _)| t.as_str()).collect())
        .collect();
    assert_eq!(body[0], "1 DLL target(s)");
    assert_eq!(body[2], "Ashfall Courier · DLSS · 310.4.0 -> 310.5.3");
    assert!(body.last().unwrap().starts_with("Backup: "));
    let cmds = confirm_now(&mut s);
    let (token, op) = dll_cmd(&cmds);
    assert_eq!(
        op,
        DllOp::Install {
            app_id: s.games.games[0].app_id,
            dll_type: DllType::Dlss,
            version: "310.5.3".into(),
        }
    );
    let installed = Installed {
        result: changed(),
        path: s.games.games[0].dlls[0].path.clone(),
        from_version: Some("310.4.0".into()),
        version: "310.5.3".into(),
    };
    finish(&mut s, token, DllResult::Installed(Ok(installed)), None);
    assert_eq!(status(&s), "DLL installed successfully!");
    assert!(render_text(&mut s).contains("DLL install completed"));

    // Esc closes the picker and says so.
    update(&mut s, key("enter"));
    assert!(game_dlls::editing(&s));
    update(&mut s, key("esc"));
    assert!(!game_dlls::editing(&s));
    assert_eq!(status(&s), "DLL install cancelled, no operation ran");
}

#[test]
fn restore_asks_and_a_moved_backup_asks_again() {
    let mut s = section(80, 24, &backed_up());
    update(&mut s, key("down"));
    update(&mut s, key("down"));
    update(&mut s, key("enter"));
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(c.title, "Confirm DLL restore");
    insta::assert_snapshot!("confirm_restore_80x24", render_text(&mut s));
    let cmds = confirm_now(&mut s);
    let (token, op) = dll_cmd(&cmds);
    assert_eq!(
        op,
        DllOp::Restore {
            app_id: s.games.games[0].app_id,
            expected: backed_up()
        }
    );
    let mut moved = backed_up();
    moved[0].version = "310.2".into();
    finish(
        &mut s,
        token,
        DllResult::BackupChanged(Ok(moved.clone())),
        None,
    );
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("the confirmation opens again")
    };
    assert_eq!(c.title, "Backup changed: review restore");
    let cmds = confirm_now(&mut s);
    let (token, op) = dll_cmd(&cmds);
    assert_eq!(
        op,
        DllOp::Restore {
            app_id: s.games.games[0].app_id,
            expected: moved
        }
    );
    let restored = Restored {
        result: changed(),
        restored: vec![backed_up()[0].path.clone()],
        deleted: Vec::new(),
        kept: Vec::new(),
    };
    finish(&mut s, token, DllResult::Restored(Ok(restored)), None);
    assert_eq!(status(&s), "Original DLLs restored!");
}

#[test]
fn confirm_destructive_off_skips_only_the_update_question() {
    let mut s = section(120, 40, &backed_up());
    let mut config = s.config.clone();
    config.confirm_destructive = false;
    s.set_config(config);
    update(&mut s, key("down"));
    let cmds = update(&mut s, key("enter"));
    assert!(s.popup.is_none());
    let (token, _) = dll_cmd(&cmds);
    let cmds = finish(
        &mut s,
        token,
        DllResult::Updated(Err("no manifest".into())),
        None,
    );
    answer_backups(&mut s, &cmds, &backed_up());
    assert_eq!(status(&s), "Update failed: no manifest");
    update(&mut s, key("down"));
    update(&mut s, key("enter"));
    assert!(
        matches!(s.popup, Some(Popup::Confirm(_))),
        "restore still asks"
    );
}

#[test]
fn a_denied_game_keeps_restore_and_dims_install_and_update() {
    let mut s = section(120, 40, &backed_up());
    let app_id = s.games.games[0].app_id;
    let lists = spela_core::denylist::DenyLists::new(
        Default::default(),
        spela_core::denylist::Overrides {
            allowed: Vec::new(),
            denied: vec![spela_core::denylist::DenyEntry {
                app_id,
                name: "Ashfall Courier".into(),
                reason: "fixture anti-cheat".into(),
            }],
        },
    );
    let cmds = denylist::start(&mut s);
    let Cmd::Denylist(denylist::Cmd::Load { request }) = cmds[0] else {
        panic!()
    };
    update(
        &mut s,
        Event::Msg(Msg::Denylist(denylist::Msg::Loaded {
            request,
            result: Ok(lists),
        })),
    );
    let rows = game_dlls::rows(&s);
    assert_eq!(
        rows[0].unavailable.as_deref(),
        Some("DLL swaps denied (fixture anti-cheat)")
    );
    assert_eq!(
        rows[1].unavailable.as_deref(),
        Some("DLL swaps denied (fixture anti-cheat)")
    );
    assert_eq!(rows[2].unavailable, None, "restore is never blocked");
    let text = render_text(&mut s);
    assert!(
        text.contains("Denylist: denied by you (fixture anti-cheat)"),
        "{text}"
    );
}

#[test]
fn the_actions_popup_lists_the_dll_rows_first() {
    let mut s = section(120, 40, &backed_up());
    update(&mut s, key("0"));
    let Some(Popup::Actions(a)) = &s.popup else {
        panic!("Actions")
    };
    let labels: Vec<&str> = a.rows.iter().map(|r| r.label.as_str()).collect();
    assert_eq!(
        &labels[..4],
        [
            "Install DLL: Ashfall Courier",
            "Update stale DLLs: Ashfall Courier (1)",
            "Restore originals: Ashfall Courier (1 file)",
            "Preview launch: Ashfall Courier",
        ]
    );
    check_keys("DLLs Actions", &s);
}

#[test]
fn a_scan_and_a_dll_job_never_overlap() {
    let mut s = section(120, 40, &backed_up());
    s.jobs.scan = Some(999);
    for row in game_dlls::rows(&s) {
        assert_eq!(row.unavailable.as_deref(), Some("wait for pending work"));
    }
}

#[test]
fn the_batch_updates_the_listed_selected_games() {
    let mut s = ready(120, 40);
    s.library.cursor = 1;
    update(&mut s, key("space"));
    update(&mut s, key("down"));
    update(&mut s, key("down"));
    update(&mut s, key("space"));
    assert_eq!(s.library.selected.len(), 2);
    update(&mut s, key("enter"));
    let Some(Popup::Actions(a)) = &s.popup else {
        panic!("the batch actions")
    };
    assert_eq!(a.rows[0].label, "Update selected games (2 visible)");
    assert_eq!(a.rows[0].unavailable, None);
    update(&mut s, key("enter"));
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(c.title, "Confirm DLL update");
    let cmds = confirm_now(&mut s);
    let (token, op) = dll_cmd(&cmds);
    let batch = batch_of(&s, &op);
    assert_eq!(batch.items.len(), 2);
    finish(&mut s, token, DllResult::Updated(Ok(batch)), None);
    assert_eq!(status(&s), "Updated DLLs for 2/2 games");
}

#[test]
fn the_catalog_updates_every_stale_deployment() {
    let mut s = ready(120, 40);
    update(&mut s, key("2"));
    update(&mut s, key("right"));
    update(&mut s, key("0"));
    let Some(Popup::Actions(a)) = &s.popup else {
        panic!("Actions")
    };
    assert_eq!(a.rows[0].label, "Update all stale deployments (6 DLLs)");
    update(&mut s, key("enter"));
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(c.title, "Confirm all stale DLL updates");
    assert_eq!(c.body[0][0].0, "6 DLL target(s)");
    let cmds = confirm_now(&mut s);
    let (token, op) = dll_cmd(&cmds);
    let DllOp::Update { requests } = &op else {
        panic!()
    };
    assert!(
        requests.iter().all(|r| !r.cached_only),
        "update-all downloads"
    );
    let batch = batch_of(&s, &op);
    let games = updated_cache(&s);
    finish(&mut s, token, DllResult::Updated(Ok(batch)), Some(games));
    assert_eq!(status(&s), "Update-all: 6 updated, 0 current, 0 failed");
    let text = render_text(&mut s);
    assert!(text.contains("Update-all: 6 updated"), "{text}");
    insta::assert_snapshot!("catalog_after_update_all_120x40", text);
}

#[test]
fn startup_asks_for_the_manifest_the_release_and_the_auto_update_after_the_scan() {
    let mut s = spela_ui::testing::state_with_games(80, 24, 4);
    let mut config = s.config.clone();
    config.auto_update_dlls = true;
    config.check_updates = true;
    s.set_config(config);
    let cmds = s.start();
    let scan = cmds
        .iter()
        .position(|c| matches!(c, Cmd::Scan { .. }))
        .unwrap();
    let refresh = cmds
        .iter()
        .position(|c| matches!(c, Cmd::Dlls(dlls::Cmd::Refresh { .. })))
        .unwrap();
    let load = cmds
        .iter()
        .position(|c| matches!(c, Cmd::Dlls(dlls::Cmd::Load { .. })))
        .unwrap();
    assert!(load < refresh, "local reads go first on the pool");
    assert!(cmds.contains(&Cmd::CheckRelease));
    assert!(
        !cmds.iter().any(|c| matches!(c, Cmd::Dll { .. })),
        "the auto-update waits for the scan"
    );
    let Cmd::Scan { request } = cmds[scan] else {
        panic!()
    };
    let games = s.games.clone();
    let cmds = update(
        &mut s,
        Event::Msg(Msg::ScanDone {
            request,
            result: Ok(games),
            profiles: Vec::new(),
        }),
    );
    let (token, op) = dll_cmd(&cmds);
    assert_eq!(op, DllOp::AutoUpdate);
    update(&mut s, key("Q"));
    assert!(!s.quit, "quit waits for the auto-update");
    let auto = AutoUpdate {
        running: vec!["Ember Tide".into()],
        ..AutoUpdate::default()
    };
    finish(&mut s, token, DllResult::Auto(auto), None);
    assert_eq!(
        status(&s),
        "DLL auto-update: skipped Ember Tide while running"
    );

    // A quiet auto-update says nothing.
    s.status = None;
    let cmds = spela_ui::dll_jobs::start_auto(&mut s);
    let (token, _) = dll_cmd(&cmds);
    finish(&mut s, token, DllResult::Auto(AutoUpdate::default()), None);
    assert!(s.status.is_none());

    // The release notice sits on the right of the status line.
    update(
        &mut s,
        Event::Msg(Msg::ReleaseNotice(Some("spela v0.7.1 is available".into()))),
    );
    let text = render_text(&mut s);
    assert!(text.contains("spela v0.7.1 is available"), "{text}");
    insta::assert_snapshot!("release_notice_80x24", text);
}

#[test]
fn a_stop_waits_for_the_dll_step_then_quits() {
    let mut s = section(120, 40, &backed_up());
    assert_eq!(update(&mut s, Event::Stop), vec![Cmd::Quit], "no job: quit");
    let mut s = section(120, 40, &backed_up());
    update(&mut s, key("down"));
    update(&mut s, key("enter"));
    let cmds = confirm_now(&mut s);
    let (token, op) = dll_cmd(&cmds);
    assert_eq!(update(&mut s, Event::Stop), vec![Cmd::StopDll]);
    assert!(!s.quit);
    assert!(render_text(&mut s).contains(QUITTING));
    let batch = batch_of(&s, &op);
    let cmds = finish(&mut s, token, DllResult::Updated(Ok(batch)), None);
    assert!(s.quit && cmds.contains(&Cmd::Quit));
}

#[test]
fn ascii_glyphs_reach_the_section_the_pickers_and_the_confirmations() {
    for (w, h) in [(120, 40), (80, 24)] {
        let mut s = section(w, h, &backed_up());
        let mut config = s.config.clone();
        config.ui.ascii_glyphs = true;
        s.set_config(config);
        let text = render_text(&mut s);
        assert!(text.is_ascii(), "{w}x{h} section:\n{text}");
        update(&mut s, key("enter"));
        update(&mut s, key("enter"));
        let text = render_text(&mut s);
        assert!(text.contains("Select DLSS version"), "{text}");
        assert!(text.is_ascii(), "{w}x{h} version picker:\n{text}");
        update(&mut s, key("enter"));
        let text = render_text(&mut s);
        assert!(text.contains("Confirm DLL install"), "{text}");
        assert!(text.is_ascii(), "{w}x{h} confirmation:\n{text}");
    }
}
