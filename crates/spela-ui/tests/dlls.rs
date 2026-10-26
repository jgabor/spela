//! 2 DLL Catalog: the Library and Deployment groups, the manifest from the
//! cache, cached versions, and marks from the one stale function
//! (decision 38). Read only: no row installs, updates or restores.

use std::collections::BTreeMap;

use spela_core::dll::manifest::{CachedManifest, Manifest, ManifestEntry};
use spela_core::games::{DetectedDll, DllType};
use spela_ui::AppState;
use spela_ui::event::{Cmd, Event, Msg};
use spela_ui::layout::Pane;
use spela_ui::screens::dlls::{self, Catalog, Group, Mark};
use spela_ui::testing::{key, render_text, state_with_games};
use spela_ui::update;

fn entry(version: &str, filename: &str) -> ManifestEntry {
    ManifestEntry {
        version: version.into(),
        filename: filename.into(),
        ..ManifestEntry::default()
    }
}

fn catalog() -> Catalog {
    let mut m = Manifest::default();
    // The older one first: the latest is the highest, not the first.
    m.dlls.insert(
        "dlss".into(),
        vec![
            entry("310.4.0", "nvngx_dlss.dll"),
            entry("310.5.3", "nvngx_dlss.dll"),
        ],
    );
    m.dlls
        .insert("dlssg".into(), vec![entry("310.5.3", "nvngx_dlssg.dll")]);
    Catalog {
        manifest: Ok(Some(CachedManifest {
            manifest: m,
            ..CachedManifest::default()
        })),
        cached: BTreeMap::from([
            (
                DllType::Dlss,
                Ok(vec!["310.5.3".to_owned(), "310.4.0".to_owned()]),
            ),
            (DllType::Dlssg, Ok(Vec::new())),
        ]),
    }
}

/// Twelve games: every other one has DLSS 310.4.0; the first also has a
/// current DLSS-G and an XeSS with no readable version.
fn state(width: u16, height: u16, c: Option<Catalog>) -> AppState {
    let mut s = state_with_games(width, height, 12);
    let first = &mut s.games.games[0];
    let dir = first.install_dir.clone();
    first.dlls.push(DetectedDll {
        path: dir.join("bin/nvngx_dlssg.dll"),
        name: "nvngx_dlssg.dll".into(),
        dll_type: DllType::Dlssg,
        version: Some("310.5.3".into()),
    });
    first.dlls.push(DetectedDll {
        path: dir.join("libxess.dll"),
        name: "libxess.dll".into(),
        dll_type: DllType::Xess,
        version: None,
    });
    let cmds = s.start();
    let request = cmds
        .iter()
        .find_map(|c| match c {
            Cmd::Dlls(dlls::Cmd::Load { request }) => Some(*request),
            _ => None,
        })
        .expect("start loads the catalog");
    if let Some(c) = c {
        update(
            &mut s,
            Event::Msg(Msg::Dlls(dlls::Msg::Loaded {
                request,
                result: Ok(c),
            })),
        );
    }
    update(&mut s, key("2"));
    s
}

#[test]
fn catalog_library_120x40() {
    let mut s = state(120, 40, Some(catalog()));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn catalog_deployment_120x40() {
    let mut s = state(120, 40, Some(catalog()));
    update(&mut s, key("right"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn catalog_list_80x24_names_the_groups() {
    let mut s = state(80, 24, Some(catalog()));
    let text = render_text(&mut s);
    assert!(text.contains("[Library]  Deployment"), "{text}");
    update(&mut s, key("right"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn catalog_deployment_80x24() {
    let mut s = state(80, 24, Some(catalog()));
    update(&mut s, key("right"));
    update(&mut s, key("enter"));
    assert_eq!(s.focus, Pane::Detail);
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn while_loading_and_without_a_manifest_nothing_reads_current() {
    let mut s = state(120, 40, None);
    let text = render_text(&mut s);
    assert!(text.contains("Latest: loading..."), "{text}");
    let dll = s.games.games[0].dlls[0].clone();
    assert_eq!(dlls::mark(&s, &dll), Mark::Unknown);

    let none = Catalog {
        manifest: Ok(None),
        cached: BTreeMap::new(),
    };
    let mut s = state(120, 40, Some(none));
    let text = render_text(&mut s);
    assert!(text.contains("Latest: unavailable"), "{text}");
    assert!(text.contains("Cached: unavailable"), "{text}");
    assert!(text.contains("No cached manifest yet"), "{text}");
    update(&mut s, key("right"));
    let text = render_text(&mut s);
    assert!(!text.contains("current"), "{text}");
    assert!(text.contains("unknown"), "{text}");
}

#[test]
fn marks_and_the_update_all_row() {
    let mut s = state(120, 40, Some(catalog()));
    let game = s.games.games[0].clone();
    let marks: Vec<Mark> = game.dlls.iter().map(|d| dlls::mark(&s, d)).collect();
    assert_eq!(
        marks,
        [
            Mark::Stale {
                latest: "310.5.3".into()
            },
            Mark::Current,
            Mark::Unknown,
        ]
    );
    assert_eq!(
        dlls::game_mark(&s, &game),
        Mark::Stale {
            latest: "310.5.3".into()
        }
    );
    assert_eq!(dlls::stale_count(&s), 6);

    // Library has no row of its own; Deployment lists update-all, which
    // cannot run in this build.
    let rows = spela_ui::screens::actions(&s, Pane::List);
    assert!(!rows.iter().any(|r| r.label.starts_with("Update all")));
    update(&mut s, key("right"));
    assert_eq!(s.dlls.group, Group::Deployment);
    let rows = spela_ui::screens::actions(&s, Pane::List);
    let row = rows
        .iter()
        .find(|r| r.label == "Update all stale deployments (6 DLLs)")
        .unwrap();
    assert_eq!(row.unavailable.as_deref(), Some(dlls::NOT_BUILT));
}

#[test]
fn the_list_scrolls_with_the_cursor_and_the_detail_follows() {
    let mut s = state_with_games(80, 24, 40);
    update(&mut s, key("2"));
    update(&mut s, key("right"));
    for _ in 0..19 {
        update(&mut s, key("down"));
    }
    let names = dlls::deployments(&s);
    assert_eq!(names.len(), 20);
    let selected = dlls::selected_game(&s).unwrap().name.clone();
    assert_eq!(selected, s.games.games[names[19]].name);
    let text = render_text(&mut s);
    assert!(text.contains(&selected), "{text}");
    assert!(s.dlls.list_scroll > 0);
}
