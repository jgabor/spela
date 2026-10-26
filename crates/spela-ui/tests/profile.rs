//! The profile page with its profiles loaded: the keymap contract in every
//! state of the page, and frames at 120x40 and 80x24 to read against
//! `docs/mockups.html` screens 1, 2 and 7.

mod common;

use common::{check_keys, loaded};
use spela_core::profile::{FieldKey, Scope};
use spela_ui::AppState;
use spela_ui::event::{Cmd, Event, Msg};
use spela_ui::layout::Pane;
use spela_ui::screens::{Action, Popup, ScreenAction, library, profile};
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
    assert_eq!(e.editor.input.text(), "250q01");
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
    // A click on a row focuses the List and runs the row's select.
    s.focus = Pane::List;
    let next = s.library.cursor + 1;
    spela_ui::update::run(
        &mut s,
        Action::Screen(ScreenAction::Library(library::Action::Select(next))),
    );
    assert!(s.profile.editor.is_none());
}

#[test]
fn the_editor_keeps_its_keys_after_a_click_on_the_list() {
    for field in [FieldKey::DlssSrMode, FieldKey::GpuClockOffset] {
        let mut s = on_field(120, 40, field);
        update(&mut s, key("enter"));
        // A click on the current game's row keeps the scope and the editor.
        s.focus = Pane::List;
        let here = s.library.cursor;
        spela_ui::update::run(
            &mut s,
            Action::Screen(ScreenAction::Library(library::Action::Select(here))),
        );
        assert!(s.profile.editor.is_some(), "{field:?}");
        update(&mut s, key("q"));
        update(&mut s, key("2"));
        assert!(!s.quit, "{field:?}: q typed or picked, not quit");
        assert_eq!(s.dest, spela_ui::screens::Dest::Library, "{field:?}");
        assert!(s.profile.editor.is_some(), "{field:?}");
    }
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
fn the_picker_at_80x24() {
    let mut s = on_field(80, 24, FieldKey::DlssSrMode);
    update(&mut s, key("enter"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn the_number_editor_at_80x24() {
    let mut s = on_field(80, 24, FieldKey::GpuClockOffset);
    update(&mut s, key("enter"));
    update(&mut s, key("x"));
    update(&mut s, key("enter"));
    let text = render_text(&mut s);
    assert!(text.contains("Invalid value: invalid integer: x"), "{text}");
    insta::assert_snapshot!(text);
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

#[test]
fn the_section_hint_reads_the_same_on_the_first_section() {
    let mut s = on_field(120, 40, FieldKey::ProtonEnableHdr);
    let text = render_text(&mut s);
    let bar = text.lines().nth(39).unwrap();
    assert!(bar.contains("Left/Right Section"), "{bar}");
    assert!(bar.contains("Esc List"), "{bar}");
}

#[test]
fn a_click_on_another_field_closes_the_editor() {
    let mut s = on_field(120, 40, FieldKey::DlssSrMode);
    update(&mut s, key("enter"));
    assert!(s.profile.editor.is_some());
    spela_ui::update::run(
        &mut s,
        spela_ui::screens::Action::Screen(spela_ui::screens::ScreenAction::Profile(
            profile::Action::SelectField(FieldKey::GpuVrr),
        )),
    );
    assert!(s.profile.editor.is_none());
    assert_eq!(s.profile.field, FieldKey::GpuVrr);
}

#[test]
fn ascii_glyphs_reach_every_part_of_the_page() {
    let ascii = |s: &mut AppState| {
        let mut config = s.config.clone();
        config.ui.ascii_glyphs = true;
        s.set_config(config);
    };
    let mut s = on_field(80, 24, FieldKey::DlssSrMode);
    ascii(&mut s);
    let text = render_text(&mut s);
    assert!(text.is_ascii(), "{text}");
    update(&mut s, key("enter"));
    let text = render_text(&mut s);
    assert!(text.is_ascii(), "the picker: {text}");
    let mut s = common::loaded(120, 40);
    ascii(&mut s);
    update(&mut s, key("space"));
    let text = render_text(&mut s);
    assert!(text.is_ascii(), "select mode: {text}");
    update(&mut s, key("tab"));
    s.profile.field = FieldKey::GpuClockOffset;
    update(&mut s, key("enter"));
    let text = render_text(&mut s);
    assert!(text.is_ascii(), "the number editor: {text}");
}

#[test]
fn the_dlls_section_lists_the_deny_rows_in_actions() {
    let mut s = on_field(120, 40, FieldKey::CpuSmt);
    update(&mut s, key("right"));
    assert_eq!(profile::aspect(&s), profile::Aspect::Dlls);
    update(&mut s, key("0"));
    let Some(Popup::Actions(p)) = &s.popup else {
        panic!("Actions")
    };
    let labels: Vec<&str> = p.rows.iter().map(|r| r.label.as_str()).collect();
    let game = &s.games.games[0].name;
    for row in [
        "Allow DLL swaps",
        "Deny DLL swaps",
        "Clear denylist override",
    ] {
        assert!(
            labels.contains(&format!("{row}: {game}").as_str()),
            "{row} in {labels:?}"
        );
    }
}

#[test]
fn an_all_games_save_never_names_a_game() {
    let mut s = loaded(120, 40);
    s.library.cursor = 0;
    s.focus = Pane::Detail;
    s.profile.field = FieldKey::ProtonEnableHdr;
    let cmds = update(&mut s, key("space"));
    // The user moves to a game before the save answers.
    s.library.cursor = 1;
    saved(&mut s, &cmds);
    let text = &s.status.as_ref().unwrap().text;
    assert!(text.starts_with("Saved: HDR"), "{text}");
    assert!(text.ends_with("(All games)"), "{text}");
}

#[test]
fn the_header_keeps_the_dll_summary_and_the_save_state() {
    use spela_core::games::{DetectedDll, DllType};
    let mut s = on_field(120, 40, FieldKey::DlssSrMode);
    let game = &mut s.games.games[0];
    game.dlls = [
        (DllType::Dlss, "nvngx_dlss.dll"),
        (DllType::Dlssg, "nvngx_dlssg.dll"),
        (DllType::Dlssd, "nvngx_dlssd.dll"),
    ]
    .into_iter()
    .map(|(dll_type, name)| DetectedDll {
        path: game.install_dir.join(name),
        name: name.into(),
        dll_type,
        version: None,
    })
    .collect();
    let text = render_text(&mut s);
    let header = text.lines().nth(2).unwrap();
    assert!(header.contains("DLLs: DLSS unknown"), "{header}");
    assert!(header.contains("Saved at once"), "{header}");
}
