//! Settings, the deny list surface, Help, the palette and the status
//! line's notices, on ratatui's TestBackend at 120x40 and 80x24.
//!
//! Read a changed snapshot against `docs/mockups.html`, screens 6 and 8:
//! the groups stack in the List with the current one open, Detail shows the
//! label, value, save state, description, `Choices` and `Default`, and the
//! hint bar reads `0 Actions  Space Next value  Tab Pane  Esc List  ? Help`.

use std::time::Instant;

use crossterm::event::{KeyModifiers, MouseButton, MouseEvent, MouseEventKind};
use spela_core::config::Theme;
use spela_core::denylist::{DenyLists, Overrides, UserList};
use spela_ui::event::{Cmd, Event, Msg, Write};
use spela_ui::keymap::{self, Key};
use spela_ui::layout::Pane;
use spela_ui::screens::{self, Dest, Owner, Popup, denylist, library, settings};
use spela_ui::state::{AppState, Level};
use spela_ui::testing::{games, key, render_text, state_with_games};
use spela_ui::update;

fn typed(state: &mut AppState, text: &str) {
    for c in text.chars() {
        update(state, Event::Key(Key::char(c)));
    }
}

fn last_token(cmds: &[Cmd]) -> u64 {
    match cmds.iter().rev().find(|c| matches!(c, Cmd::Save { .. })) {
        Some(Cmd::Save { token, .. }) => *token,
        _ => panic!("no save in {cmds:?}"),
    }
}

fn jump(state: &mut AppState, option: &'static str) {
    screens::run(
        state,
        screens::ScreenAction::Settings(settings::Action::Jump(option)),
    );
}

fn deny_lists(state: &mut AppState, overrides: Overrides) {
    let cmds = denylist::start(state);
    let Cmd::Denylist(denylist::Cmd::Load { request }) = cmds[0] else {
        panic!("{cmds:?}")
    };
    update(
        state,
        Event::Msg(Msg::Denylist(denylist::Msg::Loaded {
            request,
            result: Ok(DenyLists::new(UserList::default(), overrides)),
        })),
    );
}

#[test]
fn settings_dll_policy_saved_120x40() {
    // Mockup 6: DLL policy open, Refresh interval in Detail, saved.
    let mut s = state_with_games(120, 40, 12);
    let mut cfg = s.config.clone();
    cfg.manifest_refresh_hours = 12;
    s.set_config(cfg);
    jump(&mut s, "manifest_refresh_hours");
    let cmds = update(&mut s, key("space"));
    assert_eq!(s.config.manifest_refresh_hours, 24, "shown at once");
    let token = last_token(&cmds);
    let saving = render_text(&mut s);
    assert!(saving.contains("Saving..."), "{saving}");
    update(
        &mut s,
        Event::Msg(Msg::Saved {
            token,
            result: Ok(()),
        }),
    );
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn settings_detail_80x24() {
    let mut s = state_with_games(80, 24, 12);
    jump(&mut s, "dll_cache_path");
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn settings_editor_with_a_refused_value_120x40() {
    let mut s = state_with_games(120, 40, 12);
    jump(&mut s, "backup_path");
    update(&mut s, key("enter"));
    typed(&mut s, "backups");
    assert!(
        update(&mut s, key("enter")).is_empty(),
        "refused, nothing saved"
    );
    let text = render_text(&mut s);
    assert!(
        text.contains("Invalid value: backup_path must be an absolute path: backups"),
        "{text}"
    );
    insta::assert_snapshot!(text);
}

#[test]
fn every_config_key_is_in_settings_once() {
    let mut seen: Vec<&str> = Vec::new();
    for g in 0..spela_core::config::SettingsSection::ALL.len() {
        seen.extend(settings::options(g).iter().map(|o| o.key));
    }
    let mut all: Vec<&str> = spela_core::config::OPTIONS.iter().map(|o| o.key).collect();
    all.sort_unstable();
    let mut sorted = seen.clone();
    sorted.sort_unstable();
    assert_eq!(sorted, all);
    assert_eq!(seen.len(), 16);
}

#[test]
fn the_theme_and_glyphs_follow_at_once() {
    let mut s = state_with_games(120, 40, 2);
    jump(&mut s, "ui.theme");
    update(&mut s, key("space"));
    assert_eq!(s.config.ui.theme, Theme::Neon);
    assert_eq!(s.theme, spela_ui::theme::Theme::new(Theme::Neon, false));
    jump(&mut s, "ui.ascii_glyphs");
    let cmds = update(&mut s, key("space"));
    assert!(s.config.ui.ascii_glyphs);
    assert!(render_text(&mut s).contains("> ASCII glyphs"));
    assert!(matches!(
        &cmds[..],
        [Cmd::Save { write: Write::Config { changes }, .. }] if changes["ui.ascii_glyphs"] == "true"
    ));
}

#[test]
fn a_click_on_a_group_opens_it() {
    let mut s = state_with_games(120, 40, 2);
    update(&mut s, key("4"));
    let text = render_text(&mut s);
    let row = text.lines().position(|l| l.contains("│ Paths")).unwrap();
    update(
        &mut s,
        Event::Mouse(MouseEvent {
            kind: MouseEventKind::Down(MouseButton::Left),
            column: 4,
            row: u16::try_from(row).unwrap(),
            modifiers: KeyModifiers::NONE,
        }),
    );
    assert_eq!(settings::current(&s).unwrap().key, "steam_path");
}

#[test]
fn the_editor_keys_are_safe_on_every_terminal() {
    let mut s = state_with_games(80, 24, 2);
    jump(&mut s, "additional_library_paths");
    update(&mut s, key("enter"));
    for _ in 0..3 {
        let table = screens::bindings(&s).split_off(1);
        assert!(
            keymap::check(&table).is_empty(),
            "{:?}",
            keymap::check(&table)
        );
        update(&mut s, key("tab"));
    }
    // Back on the input: a list is typed with colons.
    typed(&mut s, "/a:/b");
    let cmds = update(&mut s, key("enter"));
    assert_eq!(s.config.additional_library_paths, ["/a", "/b"]);
    assert_eq!(cmds.len(), 1);
}

#[test]
fn the_palette_opens_a_setting_in_detail() {
    let mut s = state_with_games(120, 40, 3);
    update(&mut s, key(":"));
    typed(&mut s, "cache path");
    update(&mut s, key("enter"));
    assert_eq!(s.dest, Dest::Settings);
    assert_eq!(s.focus, Pane::Detail);
    assert_eq!(settings::current(&s).unwrap().key, "dll_cache_path");
}

#[test]
fn the_palette_filters_a_thousand_games_quickly() {
    let mut s = state_with_games(120, 40, 1);
    s.games = games(1000);
    library::games_changed(&mut s);
    update(&mut s, key(":"));
    let started = Instant::now();
    typed(&mut s, "fixture 9");
    let per_key = started.elapsed() / 9;
    let Some(Popup::Palette(p)) = &s.popup else {
        panic!("the palette is open")
    };
    assert!(p.rows.len() > 1000);
    assert!(!p.matches.is_empty());
    // The release budget is 1 ms (architecture.md, performance budgets).
    // This is a debug build, so it only catches a blow-up.
    assert!(per_key.as_millis() < 100, "{per_key:?} per key");
}

#[test]
fn the_deny_rows_reach_the_allow_confirmation_from_the_palette() {
    let mut s = state_with_games(120, 40, 12);
    deny_lists(&mut s, Overrides::default());
    update(&mut s, key(":"));
    typed(&mut s, "allow dll");
    update(&mut s, key("enter"));
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("Allow confirms first")
    };
    assert_eq!(c.title, "Allow DLL swaps");
    insta::assert_snapshot!(render_text(&mut s));
    update(&mut s, key("esc"));
    assert!(s.popup.is_none());
    assert!(s.saves.is_empty(), "Esc never confirms");
}

#[test]
fn deny_reason_editor_in_library_detail_120x40() {
    let mut s = state_with_games(120, 40, 12);
    deny_lists(&mut s, Overrides::default());
    update(&mut s, key(":"));
    typed(&mut s, "deny dll");
    update(&mut s, key("enter"));
    assert!(s.denylist.editor.is_some());
    insta::assert_snapshot!(render_text(&mut s));
    let cmds = update(&mut s, key("enter"));
    let token = last_token(&cmds);
    let reload = screens::saved(&mut s, Owner::Denylist, token, Ok(()));
    assert!(matches!(
        reload[..],
        [Cmd::Denylist(denylist::Cmd::Load { .. })]
    ));
    let status = s.status.as_ref().unwrap();
    assert_eq!(status.level, Level::Ok);
    assert!(status.text.ends_with("(user-specified)"), "{}", status.text);
}

#[test]
fn help_from_settings_lists_its_keys_and_rows() {
    let mut s = state_with_games(120, 40, 2);
    jump(&mut s, "steam_path");
    update(&mut s, key("?"));
    let Some(Popup::Help(h)) = &s.popup else {
        panic!("? opens Help")
    };
    assert!(h.lines.iter().any(|l| matches!(l,
        screens::help::HelpLine::Row { label, .. } if label == "Edit value")));
    assert!(h.lines.iter().any(|l| matches!(l,
        screens::help::HelpLine::Key { keys, label, .. } if keys == "Enter" && label == "Edit value")));
}

#[test]
fn the_release_notice_sits_right_of_a_long_message_at_80x24() {
    let mut s = state_with_games(80, 24, 2);
    update(
        &mut s,
        Event::Msg(Msg::ReleaseNotice(Some("spela v0.9.0 is available".into()))),
    );
    s.say(
        Level::Info,
        "Migrated 14 files from Go spela, kept as backups only. Dropped 3 settings. See /home/user/.local/share/spela/migration.toml",
    );
    let text = render_text(&mut s);
    let status = text.lines().nth(22).unwrap();
    assert!(status.ends_with("spela v0.9.0 is available"), "{status:?}");
    assert!(status.contains("Migrated 14 files"), "{status:?}");
    assert!(
        status.chars().count() <= 79,
        "nothing past the last column: {status:?}"
    );
    let mut wide = state_with_games(120, 40, 2);
    update(
        &mut wide,
        Event::Msg(Msg::ReleaseNotice(Some("spela v0.9.0 is available".into()))),
    );
    let text = render_text(&mut wide);
    assert!(
        text.lines()
            .nth(38)
            .unwrap()
            .ends_with("spela v0.9.0 is available")
    );
}

#[test]
fn the_migration_notice_opens_over_the_first_frame_and_closes() {
    let mut s = AppState::new(spela_ui::Startup {
        size: (80, 24),
        config: spela_core::config::Config::default(),
        games: games(3),
        profiles: Vec::new(),
        broken_profiles: Default::default(),
        notice: vec![
            "Migrated 14 files from Go spela, kept as backups only. Dropped 3 settings. See /home/user/.local/share/spela/migration.toml".into(),
        ],
        warnings: Vec::new(),
        no_color: false,
        demo: false,
        clock: spela_ui::testing::clock(),
    });
    let cmds = s.start();
    assert!(cmds.iter().any(|c| matches!(c, Cmd::Denylist(_))));
    let text = render_text(&mut s);
    assert!(text.contains("Migrated from Go spela"), "{text}");
    assert!(
        text.contains("migration.toml"),
        "the whole path shows: {text}"
    );
    // The startup scan's message does not hide it.
    let Some(Cmd::Scan { request }) = cmds.iter().find(|c| matches!(c, Cmd::Scan { .. })) else {
        panic!("{cmds:?}")
    };
    update(
        &mut s,
        Event::Msg(Msg::ScanDone {
            request: *request,
            result: Ok(games(3)),
            profiles: Vec::new(),
        }),
    );
    assert!(matches!(s.popup, Some(Popup::Notice(_))));
    update(&mut s, key("enter"));
    assert!(s.popup.is_none());
}

#[test]
fn a_saved_log_level_sets_the_log_file_level_at_once() {
    let mut s = state_with_games(120, 40, 3);
    update(&mut s, key("4"));
    jump(&mut s, "log_level");
    s.focus = Pane::Detail;
    let cmds = update(&mut s, key("space"));
    let token = last_token(&cmds);
    let level = s.config.log_level;
    let after = update(
        &mut s,
        Event::Msg(Msg::Saved {
            token,
            result: Ok(()),
        }),
    );
    assert_eq!(after, vec![Cmd::SetLogLevel(level)]);
    // A failed save leaves the level alone.
    let cmds = update(&mut s, key("space"));
    let token = last_token(&cmds);
    let after = update(
        &mut s,
        Event::Msg(Msg::Saved {
            token,
            result: Err("disk full".into()),
        }),
    );
    assert!(!after.iter().any(|c| matches!(c, Cmd::SetLogLevel(_))));
}
