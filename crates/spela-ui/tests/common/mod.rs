//! Shared by the Library and profile page tests: a state with every
//! profile loaded, as `spela tui` has it a moment after start, and the
//! keymap contract of `tests/keys.rs` for one context.

#![allow(dead_code)]

use spela_core::profile::{FieldKey, OwnValues, Value};
use spela_ui::AppState;
use spela_ui::event::{Cmd, Event, Msg};
use spela_ui::keymap::{self, Resolution};
use spela_ui::screens::{self, help, profile};
use spela_ui::testing::state_with_games;
use spela_ui::update;
use spela_ui::view::fitted_hints;

fn own(pairs: &[(FieldKey, Value)]) -> OwnValues {
    let mut v = OwnValues::new();
    for (k, val) in pairs {
        v.set(*k, val.clone()).expect("a valid value");
    }
    v
}

/// A state on 12 fictional games with the mockup's profiles loaded: All
/// games sets HDR, the DLSS modes, frame generation, VRR, the shader cache
/// and the governor; the first game sets its own super resolution, the
/// two overrides and a clock offset.
pub fn loaded(width: u16, height: u16) -> AppState {
    let mut s = state_with_games(width, height, 12);
    let request = s
        .start()
        .into_iter()
        .find_map(|c| match c {
            Cmd::Profile(profile::Cmd::Load { request }) => Some(request),
            _ => None,
        })
        .expect("the start loads the profiles");
    // The start also asked for a scan; the frames read as after it.
    s.jobs.scan = None;
    let text = |t: &str| Value::Text(t.to_owned());
    let all = own(&[
        (FieldKey::ProtonEnableHdr, Value::Bool(true)),
        (FieldKey::DlssSrMode, text("balanced")),
        (FieldKey::DlssSrPreset, text("K")),
        (FieldKey::DlssSrOverride, Value::Bool(false)),
        (FieldKey::DlssRrMode, text("quality")),
        (FieldKey::DlssRrOverride, Value::Bool(false)),
        (FieldKey::DlssFgEnabled, Value::Bool(true)),
        (FieldKey::DlssFgOverride, Value::Bool(true)),
        (FieldKey::DlssMultiFrame, Value::Int(1)),
        (FieldKey::GpuVrr, text("automatic")),
        (FieldKey::GpuShaderCache, Value::Bool(true)),
        (FieldKey::CpuGovernor, text("performance")),
    ]);
    let first = s.games.games[0].app_id;
    let game = own(&[
        (FieldKey::DlssSrMode, text("quality")),
        (FieldKey::DlssSrOverride, Value::Bool(true)),
        (FieldKey::DlssRrOverride, Value::Bool(true)),
        (FieldKey::GpuClockOffset, Value::Int(150)),
    ]);
    update(
        &mut s,
        Event::Msg(Msg::Profile(profile::Msg::Loaded {
            request,
            result: Ok(profile::Loaded {
                all_games: Some((all, false)),
                games: vec![profile::LoadedGame {
                    app_id: first,
                    values: game,
                    read_only: false,
                }],
                broken: Vec::new(),
            }),
        })),
    );
    s
}

/// The keymap contract for one context: no key bound twice in a table,
/// every hinted key runs what its hint says, Help lists every key and
/// shows no reason for a key that works, and Esc never quits.
pub fn check_keys(name: &str, state: &AppState) {
    let tables = if state.popup.is_some() {
        vec![screens::bindings(state).split_off(1)]
    } else {
        let (lead, tail) = screens::global_bindings(state);
        let mut global = lead;
        global.extend(tail);
        vec![screens::screen_bindings(state, state.focus), global]
    };
    for table in &tables {
        let problems = keymap::check(table);
        assert!(problems.is_empty(), "{name}: {problems:?}");
    }
    let bindings = screens::bindings(state);
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
    // Help is built from the same list `?` reads.
    if state.popup.is_none() && state.browsing() {
        let h = help::State::new(state, "", &bindings, &[]);
        for b in bindings.iter().filter(|b| !b.label.is_empty()) {
            let listed = h
                .lines
                .iter()
                .any(|l| matches!(l, help::HelpLine::Key { keys, .. } if *keys == b.key_text()));
            assert!(listed, "{name}: Help misses {}", b.key_text());
        }
        for line in &h.lines {
            if let help::HelpLine::Key {
                keys: k,
                unavailable: Some(_),
                ..
            } = line
            {
                let b = bindings.iter().find(|b| b.key_text() == *k).unwrap();
                assert!(!b.available(), "{name}: {k} works but shows a reason");
            }
        }
    }
}
