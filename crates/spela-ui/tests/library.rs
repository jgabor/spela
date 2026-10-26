//! The Library list's filters, sorts, multi-select and Find controls: the
//! keymap contract in each, and frames to read against `docs/mockups.html`.

mod common;

use common::{check_keys, loaded};
use spela_ui::screens::library::{self, Action, Sort};
use spela_ui::screens::{Action as AnyAction, Popup, ScreenAction};
use spela_ui::testing::{key, render_text};
use spela_ui::update;

fn run(s: &mut spela_ui::AppState, a: Action) {
    spela_ui::update::run(s, AnyAction::Screen(ScreenAction::Library(a)));
}

#[test]
fn every_list_state_keeps_the_keymap_contract() {
    for (w, h) in [(120, 40), (80, 24)] {
        let s = loaded(w, h);
        check_keys(&format!("{w}x{h} list"), &s);
        let mut s = loaded(w, h);
        update(&mut s, key("/"));
        check_keys(&format!("{w}x{h} Find"), &s);
        update(&mut s, key("tab"));
        check_keys(&format!("{w}x{h} Find Apply"), &s);
        let mut s = loaded(w, h);
        update(&mut s, key("space"));
        check_keys(&format!("{w}x{h} select mode"), &s);
        update(&mut s, key("enter"));
        assert!(matches!(s.popup, Some(Popup::Actions(_))));
        check_keys(&format!("{w}x{h} batch actions"), &s);
        let mut s = loaded(w, h);
        run(&mut s, Action::ToggleDllFilter);
        run(&mut s, Action::SetSort(Sort::ZToA));
        check_keys(&format!("{w}x{h} filtered"), &s);
    }
}

#[test]
fn the_palette_reaches_the_list_actions_and_the_games() {
    let mut s = loaded(120, 40);
    update(&mut s, key(":"));
    let Some(Popup::Palette(p)) = &s.popup else {
        panic!("the palette")
    };
    let has = |label: &str| p.rows.iter().any(|r| r.label == label);
    assert!(has("Sort: profiles first"));
    assert!(has("Toggle DLL filter"));
    assert!(has("Ashfall Courier"));
    assert!(has("Preview launch: Ashfall Courier"));
}

#[test]
fn select_mode_at_120x40() {
    let mut s = loaded(120, 40);
    update(&mut s, key("space"));
    update(&mut s, key("down"));
    update(&mut s, key("down"));
    update(&mut s, key("space"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn filters_and_sort_at_120x40() {
    let mut s = loaded(120, 40);
    run(&mut s, Action::ToggleDllFilter);
    run(&mut s, Action::SetSort(Sort::ZToA));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn nothing_matches_the_filters_or_the_search() {
    let mut s = loaded(120, 40);
    s.library.profiles.clear();
    run(&mut s, Action::ToggleProfileFilter);
    let text = render_text(&mut s);
    assert!(text.contains("No games match filters"), "{text}");
    assert!(text.contains("Clear filters in Actions"), "{text}");
    run(&mut s, Action::ClearFilters);
    update(&mut s, key("/"));
    for c in ["z", "z"] {
        update(&mut s, key(c));
    }
    let text = render_text(&mut s);
    assert!(text.contains("No games match \"zz\""), "{text}");
    assert!(text.contains("Use Clear search or"), "{text}");
    update(&mut s, key("esc"));
    assert!(s.library.query.is_empty());
    assert_eq!(s.library.rows[0], library::ListRow::AllGames);
}

#[test]
fn select_mode_at_80x24() {
    let mut s = loaded(80, 24);
    update(&mut s, key("space"));
    let text = render_text(&mut s);
    assert!(text.contains("Selected: 1 total, 1 visible"), "{text}");
    insta::assert_snapshot!(text);
}

#[test]
fn filters_and_sort_at_80x24() {
    let mut s = loaded(80, 24);
    run(&mut s, Action::ToggleDllFilter);
    run(&mut s, Action::SetSort(Sort::ZToA));
    let text = render_text(&mut s);
    assert!(text.contains("Scope [Z-A]"), "{text}");
    insta::assert_snapshot!(text);
}

/// With no games, the List shows Go's empty state: no All games row, no
/// Find row, and the scan error stays under the recovery hint after the
/// status line clears.
#[test]
fn an_empty_library_shows_the_recovery_state() {
    for (w, h) in [(120, 40), (80, 24)] {
        for error in [None, Some("could not find Steam installation")] {
            let mut s = spela_ui::testing::state_with_games(w, h, 0);
            s.library.scan_error = error.map(str::to_owned);
            assert!(s.library.rows.is_empty(), "{w}x{h}: {:?}", s.library.rows);
            check_keys(&format!("{w}x{h} empty"), &s);
            let text = render_text(&mut s);
            assert!(text.contains("No games found"), "{text}");
            assert!(text.contains("Use Actions to rescan or"), "{text}");
            assert!(text.contains("Settings for paths"), "{text}");
            assert!(!text.contains("Find games"), "{text}");
            assert!(!text.contains("All games"), "{text}");
            if let Some(e) = error {
                assert!(text.contains(&e[..20]), "{text}");
            }
            let name = format!(
                "empty_library_{w}x{h}{}",
                if error.is_some() { "_scan_error" } else { "" }
            );
            insta::assert_snapshot!(name, text);
        }
    }
}
