//! The keymap's contract: keys you can see are keys that work
//! (terminal-ux.md, principles 2 and 3).
//!
//! In every destination, pane, size and popup: no key is bound twice in
//! one table, every key can be typed with legacy encoding, every key the
//! hint bar shows runs the action it names, and Help lists the same keys
//! the context dispatches.

use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
use spela_ui::AppState;
use spela_ui::keymap::{self, Binding, Key, KeyName, Resolution};
use spela_ui::layout::Pane;
use spela_ui::screens::{self, Popup, help};
use spela_ui::testing::{key, state_with_games};
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
