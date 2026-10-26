//! Help (`?`, or the Help row in Actions): "Keys here" for the context it
//! was opened from, the Actions rows of that context, then a reference per
//! mode, all built from the keymap.
//!
//! "Keys here" is the composed binding list of that context, the same one
//! the hint bar shows and dispatch reads, so every key in it works there.
//! A key bound twice shows the binding dispatch would run: the first one
//! that can run. A reason shows only for a key that cannot run, which
//! fixes Go's Help printing a reason for keys that work (TUI conflicts).
//! The reference tables come from the bindings of Browse, of the popups
//! and of the editors and pickers, so they cannot drift from the keys
//! either (Go's four "Reference: <mode> controls" tables, TUI 6).

use ratatui::Frame;
use ratatui::layout::Rect;

use super::{Action as AnyAction, Popup, PopupAction, Row, actions, confirm};
use crate::editor::Editor;
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::Layout;
use crate::paint::Painter;
use crate::picker::Picker;
use crate::state::AppState;
use crate::theme::Role;

/// The popup's widest width.
pub const WIDTH: u16 = 100;

/// One Help line: a heading, a key with its label, an Actions row, or
/// plain text.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum HelpLine {
    /// A section heading.
    Heading(String),
    /// A key, what it does, and why it cannot run now, if it cannot.
    Key {
        /// The key text, such as `Up/Down` or `Enter`.
        keys: String,
        /// What it does.
        label: String,
        /// Why it cannot run now.
        unavailable: Option<String>,
    },
    /// An Actions row: its label, its own key if it has one, and why it
    /// cannot run now.
    Row {
        /// The row's key outside the popup, such as `/`.
        key: Option<String>,
        /// The label.
        label: String,
        /// Why it cannot run now.
        unavailable: Option<String>,
    },
    /// Plain text.
    Text(String),
    /// A blank line.
    Blank,
}

/// The Help popup.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct State {
    /// The lines.
    pub lines: Vec<HelpLine>,
    /// The first line shown.
    pub scroll: usize,
    /// The Close button has focus.
    pub on_close: bool,
}

const MOUSE: &[&str] = &[
    "Click a row to move there; double click opens it, like Enter.",
    "Click a destination, a section, a hint or a button to run it.",
    "Right click opens Actions for what is under the pointer.",
    "The wheel scrolls the pane under the pointer.",
    "Hold Shift to select text in most terminals.",
];

fn capital(label: &str) -> String {
    let mut c = label.chars();
    match c.next() {
        Some(first) => first.to_uppercase().chain(c).collect(),
        None => String::new(),
    }
}

/// Key lines for `bindings`, one per key text, in first-appearance order.
/// Each shows the binding dispatch runs for it: the first available one,
/// or the first one with its reason when none can run. `reasons` off
/// leaves the reasons out, for the reference tables.
pub fn key_lines(bindings: &[Binding], reasons: bool) -> Vec<HelpLine> {
    let mut order: Vec<String> = Vec::new();
    for b in bindings {
        let text = b.key_text();
        if b.label.is_empty() || text.is_empty() || order.contains(&text) {
            continue;
        }
        order.push(text);
    }
    order
        .into_iter()
        .map(|text| {
            let same: Vec<&Binding> = bindings
                .iter()
                .filter(|b| !b.label.is_empty() && b.key_text() == text)
                .collect();
            let shown = same
                .iter()
                .find(|b| b.available())
                .or_else(|| same.first())
                .expect("the key text came from these bindings");
            HelpLine::Key {
                keys: text,
                label: capital(&shown.label),
                unavailable: if reasons {
                    shown.unavailable.clone()
                } else {
                    None
                },
            }
        })
        .collect()
}

fn reference_popups(state: &AppState) -> Vec<Binding> {
    let menu = actions::State::new(String::new(), Vec::new());
    let mut out = actions::bindings(state, &menu);
    let ask = confirm::State::new(String::new(), Vec::new(), AnyAction::Back);
    out.extend(confirm::bindings(state, &ask));
    out
}

fn reference_editors() -> Vec<Binding> {
    let picker = Picker::new(String::new(), None, Vec::new());
    let mut out = picker.bindings(|_| AnyAction::Back, |_| AnyAction::Back);
    let editor = Editor::new(String::new(), None, "");
    out.extend(editor.bindings(|_| AnyAction::Back, |_| AnyAction::Back));
    out
}

impl State {
    /// Help for a context: its composed bindings and its Actions rows,
    /// then the reference tables for every mode.
    pub fn new(state: &AppState, context: &str, keys: &[Binding], rows: &[Row]) -> State {
        let mut lines = vec![HelpLine::Heading(format!("Keys here: {context}"))];
        lines.extend(key_lines(keys, true));
        if !rows.is_empty() {
            lines.push(HelpLine::Blank);
            lines.push(HelpLine::Heading(
                "Actions here: open them with 0 after closing Help".into(),
            ));
            lines.extend(rows.iter().map(|r| HelpLine::Row {
                key: r.key.map(|k| k.to_string()),
                label: r.label.clone(),
                unavailable: r.unavailable.clone(),
            }));
        }
        let (lead, tail) = super::global_bindings(state);
        let browse: Vec<Binding> = lead.into_iter().chain(tail).collect();
        for (heading, bindings) in [
            ("Reference: Browse controls", browse),
            ("Reference: popup controls", reference_popups(state)),
            ("Reference: editor and picker controls", reference_editors()),
        ] {
            lines.push(HelpLine::Blank);
            lines.push(HelpLine::Heading(heading.to_owned()));
            lines.extend(key_lines(&bindings, false));
        }
        lines.push(HelpLine::Blank);
        lines.push(HelpLine::Heading("Mouse".into()));
        if state.config.ui.mouse {
            lines.extend(MOUSE.iter().map(|t| HelpLine::Text((*t).to_owned())));
        } else {
            lines.push(HelpLine::Text(
                "The mouse is off. Turn it on in Settings, Display, Mouse.".into(),
            ));
        }
        State {
            lines,
            scroll: 0,
            on_close: false,
        }
    }
}

/// Opens Help for the current context. From the Actions popup, Help
/// describes the context behind it.
pub fn open(state: &mut AppState) {
    let behind = matches!(state.popup, Some(Popup::Actions(_)));
    let popup = if behind { state.popup.take() } else { None };
    let keys = super::bindings(state);
    let rows = if state.popup.is_none() {
        super::actions(state, state.focus)
    } else {
        Vec::new()
    };
    let context = format!(
        "{} {} {}",
        state.dest.name(),
        state.glyphs.dot,
        state.focus.word()
    );
    let help = State::new(state, &context, &keys, &rows);
    if behind {
        state.popup = popup;
    }
    state.open(Popup::Help(help));
}

/// What the popup's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// Scroll up.
    Up,
    /// Scroll down.
    Down,
    /// A page up.
    PageUp,
    /// A page down.
    PageDown,
    /// Between the body and Close.
    Tab,
    /// Enter: closes when Close has focus.
    Enter,
    /// Closes Help.
    Close,
}

fn act(a: Action) -> AnyAction {
    AnyAction::Popup(PopupAction::Help(a))
}

/// The popup's keys.
pub fn bindings(_state: &AppState, popup: &State) -> Vec<Binding> {
    let mut out = vec![
        Binding::new(Key::UP, act(Action::Up), "Scroll").hint_as("Up/Down", 100),
        Binding::new(Key::DOWN, act(Action::Down), "Scroll down"),
        Binding::new(Key::PAGE_UP, act(Action::PageUp), "Page up"),
        Binding::new(Key::PAGE_DOWN, act(Action::PageDown), "Page down"),
        Binding::new(Key::TAB, act(Action::Tab), "Close button")
            .also(Key::BACK_TAB)
            .hint(50),
    ];
    if popup.on_close {
        out.push(Binding::new(Key::ENTER, act(Action::Enter), "Close").hint(90));
    }
    out.push(Binding::new(Key::ESC, act(Action::Close), "Close").hint(40));
    out
}

/// Runs a popup action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let page = usize::from(state.layout().workspace().height.saturating_sub(6)).max(1);
    let Some(Popup::Help(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    let last = popup.lines.len().saturating_sub(1);
    match action {
        Action::Up => popup.scroll = popup.scroll.saturating_sub(1),
        Action::Down => popup.scroll = (popup.scroll + 1).min(last),
        Action::PageUp => popup.scroll = popup.scroll.saturating_sub(page),
        Action::PageDown => popup.scroll = (popup.scroll + page).min(last),
        Action::Tab => popup.on_close = !popup.on_close,
        Action::Enter if popup.on_close => state.close_popup(),
        Action::Enter => {}
        Action::Close => state.close_popup(),
    }
    Vec::new()
}

/// Draws the popup.
pub fn render(
    state: &AppState,
    popup: &State,
    frame: &mut Frame,
    layout: &Layout,
    hits: &mut HitMap,
) {
    let ws = layout.workspace();
    let area = layout.centered(WIDTH, ws.height);
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = p.popup(area, "Keyboard shortcuts", hits);
    if inner.height < 5 || inner.width < 20 {
        return;
    }
    let x = inner.x + 2;
    let w = inner.width.saturating_sub(4);
    p.text(
        x,
        inner.y,
        w,
        "Reference only. Close Help before using these controls.",
        Role::Muted,
    );
    let body = Rect::new(inner.x, inner.y + 2, inner.width, inner.height - 4);
    let visible = usize::from(body.height);
    let key_w = 14u16.min(w / 3);
    let scroll = popup.scroll.min(popup.lines.len().saturating_sub(visible));
    for (n, line) in popup.lines.iter().skip(scroll).take(visible).enumerate() {
        let y = body.y + u16::try_from(n).unwrap_or(0);
        match line {
            HelpLine::Heading(t) => {
                p.text(x, y, w, t, Role::Title);
            }
            HelpLine::Key {
                keys,
                label,
                unavailable,
            } => {
                p.text(x, y, key_w, keys, Role::Key);
                let text = match unavailable {
                    Some(r) => format!("{label} {} {r}", state.glyphs.dot),
                    None => label.clone(),
                };
                let role = if unavailable.is_some() {
                    Role::Muted
                } else {
                    Role::Text
                };
                p.text(x + key_w + 1, y, w.saturating_sub(key_w + 1), &text, role);
            }
            HelpLine::Row {
                key,
                label,
                unavailable,
            } => {
                if let Some(k) = key {
                    p.text(x, y, key_w, k, Role::Key);
                }
                let (text, role) = match unavailable {
                    Some(r) => (format!("{label} {} {r}", state.glyphs.dot), Role::Muted),
                    None => (label.clone(), Role::Text),
                };
                p.text(x + key_w + 1, y, w.saturating_sub(key_w + 1), &text, role);
            }
            HelpLine::Text(t) => {
                p.text(x, y, w, t, Role::Text);
            }
            HelpLine::Blank => {}
        }
    }
    p.thumb(area.right() - 1, body, scroll, visible, popup.lines.len());
    let close_y = inner.bottom() - 1;
    let role = if popup.on_close {
        Role::Key
    } else {
        Role::Text
    };
    if popup.on_close {
        p.text(x - 1, close_y, 1, state.glyphs.cursor, Role::Key);
    }
    p.text(x + 1, close_y, 5, "Close", role);
    let close = Rect::new(inner.x, close_y, inner.width, 1);
    if popup.on_close {
        p.cursor_row(close);
    }
    hits.push(close, HitTarget::Action(act(Action::Close)));
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::screens::Action;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;

    #[test]
    fn a_key_bound_twice_shows_the_binding_that_runs() {
        let b = vec![
            Binding::new(Key::ESC, Action::Back, "Back").unavailable("nothing to close"),
            Binding::new(Key::ESC, Action::Help, "Clear search"),
            Binding::new(Key::char('/'), Action::Find, "Find").unavailable("Library only"),
        ];
        assert_eq!(
            key_lines(&b, true),
            [
                HelpLine::Key {
                    keys: "Esc".into(),
                    label: "Clear search".into(),
                    unavailable: None
                },
                HelpLine::Key {
                    keys: "/".into(),
                    label: "Find".into(),
                    unavailable: Some("Library only".into())
                },
            ]
        );
    }

    #[test]
    fn help_lists_the_actions_rows_and_the_reference_from_the_keymap() {
        let mut s = state_with_games(120, 40, 3);
        step(&mut s, key("0"));
        // The Help row of the Actions popup.
        crate::update::run(&mut s, Action::Help);
        let Some(Popup::Help(h)) = &s.popup else {
            panic!("the Help row opens Help")
        };
        assert_eq!(
            h.lines[0],
            HelpLine::Heading("Keys here: Library · list".into())
        );
        assert!(
            h.lines
                .iter()
                .any(|l| matches!(l, HelpLine::Row { label, .. } if label == "Rescan games"))
        );
        let headings: Vec<&str> = h
            .lines
            .iter()
            .filter_map(|l| match l {
                HelpLine::Heading(t) => Some(t.as_str()),
                _ => None,
            })
            .collect();
        assert_eq!(
            headings[1..],
            [
                "Actions here: open them with 0 after closing Help",
                "Reference: Browse controls",
                "Reference: popup controls",
                "Reference: editor and picker controls",
                "Mouse",
            ]
        );
        assert!(h.lines.iter().any(|l| matches!(l, HelpLine::Key { keys, label, .. } if keys == "Type" && label == "To jump")));
    }
}
