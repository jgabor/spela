//! The keymap's contract: keys you can see are keys that work
//! (terminal-ux.md, principles 2 and 3).
//!
//! In every destination, pane, size and popup: no key is bound twice in
//! one table, every key can be typed with legacy encoding, every key the
//! hint bar shows runs the action it names, and Help lists the same keys
//! the context dispatches.

mod common;

use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
use spela_core::profile::FieldKey;
use spela_ui::AppState;
use spela_ui::event::Cmd;
use spela_ui::hit::HitTarget;
use spela_ui::keymap::{self, Binding, Key, KeyName, Resolution};
use spela_ui::layout::Pane;
use spela_ui::screens::{self, Dest, Popup, help};
use spela_ui::testing::{key, render_text, state_with_games};
use spela_ui::update;
use spela_ui::view::fitted_hints;

/// The crossterm event a terminal sends for `key` in legacy encoding:
/// uppercase letters carry Shift, Shift-Tab is BackTab with Shift.
fn legacy(key: &Key) -> KeyEvent {
    let (code, mut mods) = match key.name {
        KeyName::Char(c) if key.ctrl => (KeyCode::Char(c), KeyModifiers::CONTROL),
        KeyName::Char(c) if c.is_ascii_uppercase() => (KeyCode::Char(c), KeyModifiers::SHIFT),
        KeyName::Char(c) => (KeyCode::Char(c), KeyModifiers::NONE),
        KeyName::Enter => (KeyCode::Enter, KeyModifiers::NONE),
        KeyName::Esc => (KeyCode::Esc, KeyModifiers::NONE),
        KeyName::Tab => (KeyCode::Tab, KeyModifiers::NONE),
        KeyName::BackTab => (KeyCode::BackTab, KeyModifiers::SHIFT),
        KeyName::Backspace => (KeyCode::Backspace, KeyModifiers::NONE),
        KeyName::Delete => (KeyCode::Delete, KeyModifiers::NONE),
        KeyName::Up => (KeyCode::Up, KeyModifiers::NONE),
        KeyName::Down => (KeyCode::Down, KeyModifiers::NONE),
        KeyName::Left => (KeyCode::Left, KeyModifiers::NONE),
        KeyName::Right => (KeyCode::Right, KeyModifiers::NONE),
        KeyName::Home => (KeyCode::Home, KeyModifiers::NONE),
        KeyName::End => (KeyCode::End, KeyModifiers::NONE),
        KeyName::PageUp => (KeyCode::PageUp, KeyModifiers::NONE),
        KeyName::PageDown => (KeyCode::PageDown, KeyModifiers::NONE),
    };
    if key.alt {
        mods |= KeyModifiers::ALT;
    }
    KeyEvent::new(code, mods)
}

/// Every context worth checking, as a state, with a name for failures.
fn contexts() -> Vec<(String, AppState)> {
    let mut out = Vec::new();
    for (w, h) in [(120, 40), (80, 24)] {
        for dest in ["1", "2", "3", "4"] {
            for pane in [Pane::List, Pane::Detail] {
                let mut s = state_with_games(w, h, 12);
                update(&mut s, key(dest));
                s.focus = pane;
                out.push((format!("{w}x{h} dest {dest} {pane:?}"), s));
            }
        }
        let mut find = state_with_games(w, h, 12);
        update(&mut find, key("/"));
        out.push((format!("{w}x{h} Find"), find));
        for popup in ["0", "?", ":"] {
            let mut s = state_with_games(w, h, 12);
            update(&mut s, key(popup));
            out.push((format!("{w}x{h} popup {popup}"), s));
        }
        // The inputs: the profile picker and number editor, Settings'
        // shared editor, and Find with a query.
        for (field, name) in [
            (FieldKey::DlssSrMode, "picker"),
            (FieldKey::GpuClockOffset, "number editor"),
        ] {
            let mut s = common::loaded(w, h);
            update(&mut s, key("enter"));
            s.profile.field = field;
            update(&mut s, key("enter"));
            assert!(s.profile.editor.is_some(), "{name}");
            out.push((format!("{w}x{h} {name}"), s));
        }
        let mut find = state_with_games(w, h, 12);
        update(&mut find, key("/"));
        update(&mut find, key("e"));
        out.push((format!("{w}x{h} Find with a query"), find));
        let mut confirm = state_with_games(w, h, 12);
        confirm.confirm(screens::confirm::State::new(
            "Confirm",
            Vec::new(),
            screens::Action::Quit,
        ));
        out.push((format!("{w}x{h} confirm"), confirm));
    }
    out
}

fn tables(state: &AppState) -> Vec<Vec<Binding>> {
    if let Some(popup) = &state.popup {
        let _ = popup;
        // A popup's table is everything after Ctrl-C.
        return vec![screens::bindings(state).split_off(1)];
    }
    let (lead, tail) = screens::global_bindings(state);
    let mut global = lead;
    global.extend(tail);
    vec![screens::screen_bindings(state, state.focus), global]
}

#[test]
fn no_table_binds_a_key_twice_or_needs_kitty() {
    for (name, state) in contexts() {
        for table in tables(&state) {
            let problems = keymap::check(&table);
            assert!(problems.is_empty(), "{name}: {problems:?}");
        }
    }
}

#[test]
fn every_binding_survives_legacy_encoding() {
    for (name, state) in contexts() {
        for b in screens::bindings(&state) {
            for k in &b.keys {
                assert_eq!(
                    Key::from_event(&legacy(k)).as_ref(),
                    Some(k),
                    "{name}: {}",
                    b.label
                );
            }
        }
    }
}

#[test]
fn every_hinted_key_runs_what_its_hint_says() {
    for (name, state) in contexts() {
        let bindings = screens::bindings(&state);
        for hint in fitted_hints(&bindings, usize::from(state.size.0 - 2)) {
            let Some(k) = hint.keys.first() else {
                assert!(hint.typed.is_some(), "{name}: a hint with no key");
                continue;
            };
            assert_eq!(
                keymap::resolve(&bindings, k),
                Resolution::Run(hint.action.clone()),
                "{name}: {} {}",
                hint.key_text(),
                hint.label
            );
        }
    }
}

#[test]
fn every_hint_click_runs_what_its_key_runs_and_typing_hints_do_nothing() {
    for (name, mut state) in contexts() {
        let bindings = screens::bindings(&state);
        let text = render_text(&mut state);
        let y = state.size.1 - 1;
        let row: Vec<char> = text.lines().nth(usize::from(y)).unwrap().chars().collect();
        for x in 0..state.size.0 {
            let Some(HitTarget::Action(a)) = state.hits.hit(x, y).cloned() else {
                continue;
            };
            let by_key = bindings.iter().any(|b| {
                b.action == a
                    && b.keys.first().is_some_and(|k| {
                        keymap::resolve(&bindings, k) == Resolution::Run(a.clone())
                    })
            });
            assert!(
                by_key,
                "{name}: a hint click at {x} runs {a:?}, which no key runs"
            );
            let at: String = row.iter().skip(usize::from(x)).take(5).collect();
            assert_ne!(at, "Type ", "{name}: the Type hint is clickable");
        }
    }
}

#[test]
fn ctrl_z_suspends_anywhere() {
    let mut cases: Vec<(String, AppState)> = contexts();
    cases.push(("72x20".into(), state_with_games(72, 20, 3)));
    for (name, mut state) in cases {
        assert_eq!(
            update(&mut state, key("ctrl-z")),
            vec![Cmd::Suspend],
            "{name}"
        );
    }
}

#[test]
fn q_clears_an_applied_search_before_it_quits() {
    let mut s = state_with_games(120, 40, 12);
    update(&mut s, key("/"));
    for k in ["g", "l", "i", "m"] {
        update(&mut s, key(k));
    }
    update(&mut s, key("enter"));
    assert_eq!(s.library.query.text(), "glim");
    let q = screens::bindings(&s)
        .into_iter()
        .find(|b| b.keys.contains(&Key::char('q')))
        .unwrap();
    assert_eq!(q.label, "Back");
    assert!(update(&mut s, key("q")).is_empty());
    assert!(
        !s.quit && s.library.query.is_empty(),
        "q cleared the search"
    );
    assert_eq!(update(&mut s, key("q")), vec![Cmd::Quit]);
}

#[test]
fn a_library_row_from_the_palette_shows_library() {
    let mut s = state_with_games(120, 40, 12);
    update(&mut s, key("4"));
    update(&mut s, key(":"));
    for c in "search games".chars() {
        update(&mut s, spela_ui::event::Event::Key(Key::char(c)));
    }
    update(&mut s, key("enter"));
    assert_eq!(s.dest, Dest::Library);
    assert!(s.library.finding, "Find is open");
    // A sort from the palette shows Library too.
    update(&mut s, key("esc"));
    update(&mut s, key("3"));
    update(&mut s, key(":"));
    for c in "sort: z-a".chars() {
        update(&mut s, spela_ui::event::Event::Key(Key::char(c)));
    }
    update(&mut s, key("enter"));
    assert_eq!(s.dest, Dest::Library);
    assert_eq!(s.library.sort, screens::library::Sort::ZToA);
}

#[test]
fn help_lists_the_keys_dispatch_reads() {
    for (name, mut state) in contexts()
        .into_iter()
        .filter(|(_, s)| s.popup.is_none() && s.browsing())
    {
        let keys = screens::bindings(&state);
        update(&mut state, key("?"));
        let Some(Popup::Help(h)) = &state.popup else {
            panic!("{name}: ? opens Help");
        };
        for b in keys.iter().filter(|b| !b.label.is_empty()) {
            let listed = h
                .lines
                .iter()
                .any(|l| matches!(l, help::HelpLine::Key { keys, .. } if *keys == b.key_text()));
            assert!(listed, "{name}: Help misses {}", b.key_text());
        }
        // A key that works never shows a reason.
        for line in &h.lines {
            if let help::HelpLine::Key {
                keys: k,
                unavailable: Some(_),
                ..
            } = line
            {
                let b = keys.iter().find(|b| b.key_text() == *k).unwrap();
                assert!(!b.available(), "{name}: {k} works but shows a reason");
            }
        }
    }
}

#[test]
fn letters_do_nothing_in_browse_that_they_should_not() {
    // Esc never quits and never confirms; Ctrl-C quits from anywhere.
    for (name, mut state) in contexts() {
        let cmds = update(&mut state, key("esc"));
        assert!(!state.quit && cmds.is_empty(), "{name}: Esc quit");
        let mut again = state;
        update(&mut again, key("ctrl-c"));
        assert!(again.quit, "{name}: Ctrl-C");
    }
}

#[test]
fn q_quits_from_every_list_with_nothing_to_back_out_of() {
    for dest in ["1", "2", "3", "4"] {
        let mut s = state_with_games(120, 40, 12);
        update(&mut s, key(dest));
        assert_eq!(update(&mut s, key("q")), vec![Cmd::Quit], "dest {dest}");
    }
}

#[test]
fn a_key_shown_in_the_actions_popup_runs_its_row() {
    // `Q` quits from inside the popup, as everywhere (terminal-ux.md).
    for dest in ["1", "2", "3", "4"] {
        let mut s = state_with_games(120, 40, 12);
        update(&mut s, key(dest));
        update(&mut s, key("0"));
        assert!(
            matches!(s.popup, Some(screens::Popup::Actions(_))),
            "dest {dest}"
        );
        assert_eq!(update(&mut s, key("Q")), vec![Cmd::Quit], "dest {dest}");
    }
    // `?` opens Help from the popup.
    let mut s = state_with_games(80, 24, 12);
    update(&mut s, key("0"));
    update(&mut s, key("?"));
    assert!(matches!(s.popup, Some(screens::Popup::Help(_))));
    // Lower-case `q` is not a row's key, so it does nothing there.
    let mut s = state_with_games(80, 24, 12);
    update(&mut s, key("0"));
    assert!(update(&mut s, key("q")).is_empty());
    assert!(matches!(s.popup, Some(screens::Popup::Actions(_))));
}
