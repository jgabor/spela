//! The profile page with its profiles loaded: the keymap contract in every
//! state of the page, and frames at 120x40 and 80x24 to read against
//! `docs/mockups.html` screens 1, 2 and 7.

mod common;

use common::{check_keys, loaded};
use spela_core::profile::{FieldKey, Scope};
use spela_ui::AppState;
use spela_ui::event::{Cmd, Event, Msg};
use spela_ui::layout::Pane;
use spela_ui::screens::{Popup, profile};
use spela_ui::testing::{key, render_text};
use spela_ui::update;

/// Library with Detail focused on the first game's field `field`.
fn on_field(width: u16, height: u16, field: FieldKey) -> AppState {
    let mut s = loaded(width, height);
    update(&mut s, key("enter"));
    assert_eq!(s.focus, Pane::Detail);
    s.profile.field = field;
    s
}

fn saved(s: &mut AppState, cmds: &[Cmd]) {
    for c in cmds {
        if let Cmd::Save { token, .. } = c {
            update(
                s,
                Event::Msg(Msg::Saved {
                    token: *token,
                    result: Ok(()),
                }),
            );
        }
    }
}

#[test]
fn every_state_of_the_page_keeps_the_keymap_contract() {
    for (w, h) in [(120, 40), (80, 24)] {
        let s = on_field(w, h, FieldKey::DlssSrMode);
        check_keys(&format!("{w}x{h} game page"), &s);
        let s = on_field(w, h, FieldKey::ProtonEnableHdr);
        check_keys(&format!("{w}x{h} first section"), &s);
        let mut s = on_field(w, h, FieldKey::DlssSrMode);
        update(&mut s, key("enter"));
        assert!(s.profile.editor.is_some());
        check_keys(&format!("{w}x{h} picker"), &s);
        update(&mut s, key("tab"));
        check_keys(&format!("{w}x{h} picker buttons"), &s);
        let mut s = on_field(w, h, FieldKey::GpuPowerLimit);
        update(&mut s, key("enter"));
        check_keys(&format!("{w}x{h} number editor"), &s);
        update(&mut s, key("tab"));
        check_keys(&format!("{w}x{h} number editor buttons"), &s);
        let mut s = on_field(w, h, FieldKey::CpuSmt);
        update(&mut s, key("right"));
        check_keys(&format!("{w}x{h} DLLs"), &s);
        let mut s = loaded(w, h);
        s.library.cursor = 0;
        s.focus = Pane::Detail;
        check_keys(&format!("{w}x{h} All games"), &s);
        update(&mut s, key("0"));
        check_keys(&format!("{w}x{h} All games Actions"), &s);
    }
}

#[test]
fn letters_and_digits_type_in_an_editor_and_never_leave_it() {
    let mut s = on_field(120, 40, FieldKey::GpuPowerLimit);
    update(&mut s, key("enter"));
    for k in ["2", "5", "0", "q", "0", "1"] {
        update(&mut s, key(k));
    }
    assert!(!s.quit);
    assert_eq!(s.dest, spela_ui::screens::Dest::Library);
    let Some(profile::Editor::Text(e)) = &s.profile.editor else {
        panic!("the editor is open")
    };
    assert_eq!(e.input.text(), "250q01");
    // An invalid number keeps the editor open with the reason.
    update(&mut s, key("enter"));
    let text = render_text(&mut s);
    assert!(text.contains("Invalid value:"), "{text}");
}

#[test]
fn a_click_on_another_game_closes_the_editor() {
    let mut s = on_field(120, 40, FieldKey::DlssSrMode);
    update(&mut s, key("enter"));
    assert!(profile::editing(&s, Pane::Detail));
    s.focus = Pane::List;
    update(&mut s, key("down"));
    assert!(s.profile.editor.is_none());
}

#[test]
fn the_flag_and_the_own_line_follow_a_save() {
    let mut s = on_field(120, 40, FieldKey::GpuPowerLimit);
    update(&mut s, key("enter"));
    for k in ["2", "5", "0"] {
        update(&mut s, key(k));
    }
    let cmds = update(&mut s, key("enter"));
    let text = render_text(&mut s);
    assert!(text.contains("Saving..."), "{text}");
    assert!(
        text.contains("Own settings (5)") && text.contains("250 W"),
        "{text}"
    );
    saved(&mut s, &cmds);
    let text = render_text(&mut s);
    assert!(text.contains("Saved: Power limit 250 W"), "{text}");
    assert!(text.contains("Own settings (5)"), "{text}");
}

#[test]
fn the_page_at_120x40() {
    let mut s = on_field(120, 40, FieldKey::DlssSrMode);
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn the_picker_at_120x40() {
    let mut s = on_field(120, 40, FieldKey::DlssSrMode);
    update(&mut s, key("enter"));
    update(&mut s, key("p"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn the_number_editor_at_120x40() {
    let mut s = on_field(120, 40, FieldKey::GpuClockOffset);
    update(&mut s, key("enter"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn the_page_at_80x24() {
    let mut s = on_field(80, 24, FieldKey::DlssRrOverride);
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn all_games_at_120x40() {
    let mut s = loaded(120, 40);
    s.library.cursor = 0;
    update(&mut s, key("enter"));
    assert_eq!(profile::scope(&s), Some(Scope::AllGames));
    s.profile.field = FieldKey::DlssSrMode;
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn the_dlls_section_at_120x40() {
    let mut s = on_field(120, 40, FieldKey::CpuSmt);
    update(&mut s, key("right"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn the_detail_actions_follow_go_s_order() {
    let mut s = on_field(120, 40, FieldKey::DlssSrMode);
    update(&mut s, key("0"));
    let Some(Popup::Actions(a)) = &s.popup else {
        panic!("Actions")
    };
    let labels: Vec<&str> = a.rows.iter().map(|r| r.label.as_str()).collect();
    assert_eq!(
        labels,
        [
            "Follow All games",
            "Follow All games for all settings",
            "Edit value",
            "Preview launch: Ashfall Courier",
            "Rescan games",
            "Help",
            "Quit"
        ]
    );
    s.close_popup();
    s.profile.field = FieldKey::ProtonEnableWayland;
    update(&mut s, key("0"));
    let Some(Popup::Actions(a)) = &s.popup else {
        panic!("Actions")
    };
    assert_eq!(a.rows[0].unavailable.as_deref(), Some("nothing to clear"));
}
