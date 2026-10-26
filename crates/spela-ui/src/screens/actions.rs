//! The Actions popup (`0`): every action of the current context as a row,
//! titled `Actions · <destination> · <pane>`, anchored right so the list
//! it acts on stays visible.
//!
//! An unavailable row stays, dimmed, with its reason after a dot. The
//! cursor starts on the first available row, Up and Down wrap, Enter runs
//! an available row, Tab moves to Close, Esc closes (terminal-ux.md,
//! popups). The rows come from the screens' `actions` through
//! [`crate::screens::actions`].

use ratatui::Frame;
use ratatui::layout::Rect;

use super::{Action as AnyAction, Popup, PopupAction, Row};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::Layout;
use crate::paint::Painter;
use crate::state::AppState;
use crate::text::width;
use crate::theme::Role;

/// The popup's width at 120 columns, as in the mockup.
pub const WIDTH: u16 = 80;

/// The Actions popup.
#[derive(Clone, Debug, PartialEq)]
pub struct State {
    /// `Actions · Library · list`.
    pub title: String,
    /// The rows, in the screens' order.
    pub rows: Vec<Row>,
    /// The cursor row.
    pub cursor: usize,
    /// The Close button has focus.
    pub on_close: bool,
}

impl State {
    /// A popup with the cursor on the first available row.
    pub fn new(title: String, rows: Vec<Row>) -> State {
        let cursor = rows
            .iter()
            .position(|r| r.unavailable.is_none())
            .unwrap_or(0);
        State {
            title,
            rows,
            cursor,
            on_close: false,
        }
    }
}

/// What the popup's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// The previous row, wrapping.
    Up,
    /// The next row, wrapping.
    Down,
    /// Between the rows and Close.
    Tab,
    /// Runs the cursor row, or closes on Close.
    Run,
    /// Runs this row, from a click.
    RunRow(usize),
    /// Closes the popup.
    Close,
}

fn act(a: Action) -> AnyAction {
    AnyAction::Popup(PopupAction::Actions(a))
}

/// The popup's keys.
pub fn bindings(_state: &AppState, popup: &State) -> Vec<Binding> {
    let run_label = if popup.on_close { "Close" } else { "Run" };
    vec![
        Binding::new(Key::UP, act(Action::Up), "Choose").hint_as("Up/Down", 100),
        Binding::new(Key::DOWN, act(Action::Down), "Next row"),
        Binding::new(Key::ENTER, act(Action::Run), run_label).hint(90),
        Binding::new(Key::TAB, act(Action::Tab), "Next control")
            .also(Key::BACK_TAB)
            .hint(50),
        Binding::new(Key::ESC, act(Action::Close), "Close").hint(40),
    ]
}

/// Runs a popup action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let Some(Popup::Actions(popup)) = &mut state.popup else {
        return Vec::new();
    };
    let n = popup.rows.len();
    match action {
        Action::Up if n > 0 => {
            popup.on_close = false;
            popup.cursor = (popup.cursor + n - 1) % n;
        }
        Action::Down if n > 0 => {
            popup.on_close = false;
            popup.cursor = (popup.cursor + 1) % n;
        }
        Action::Up | Action::Down => {}
        Action::Tab => popup.on_close = !popup.on_close,
        Action::Close => state.close_popup(),
        Action::Run if popup.on_close => state.close_popup(),
        Action::Run => {
            let i = popup.cursor;
            return run_row(state, i);
        }
        Action::RunRow(i) => return run_row(state, i),
    }
    state.dirty = true;
    Vec::new()
}

fn run_row(state: &mut AppState, i: usize) -> Vec<AnyCmd> {
    let Some(Popup::Actions(popup)) = &state.popup else {
        return Vec::new();
    };
    let Some(row) = popup.rows.get(i).cloned() else {
        return Vec::new();
    };
    if row.unavailable.is_some() {
        if let Some(Popup::Actions(popup)) = &mut state.popup {
            popup.cursor = i;
        }
        state.dirty = true;
        return Vec::new();
    }
    // Help describes the context behind the popup, so it takes the popup
    // over itself; every other row closes it first.
    if row.action != AnyAction::Help {
        state.close_popup();
    }
    crate::update::run(state, row.action)
}

/// Draws the popup.
pub fn render(
    state: &AppState,
    popup: &State,
    frame: &mut Frame,
    layout: &Layout,
    hits: &mut HitMap,
) {
    let rows = u16::try_from(popup.rows.len()).unwrap_or(u16::MAX);
    let area = layout.right_anchored(WIDTH, rows.saturating_add(6));
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = p.popup(area, &popup.title, hits);
    if inner.height < 4 || inner.width < 10 {
        return;
    }
    let visible = usize::from(inner.height - 4);
    let offset = popup.cursor.saturating_sub(visible.saturating_sub(1));
    let key_x = inner.right().saturating_sub(3);
    for (n, (i, row)) in popup
        .rows
        .iter()
        .enumerate()
        .skip(offset)
        .take(visible)
        .enumerate()
    {
        let y = inner.y + 1 + u16::try_from(n).unwrap_or(0);
        let line = Rect::new(inner.x, y, inner.width, 1);
        let cursor = !popup.on_close && i == popup.cursor;
        if cursor {
            p.text(inner.x + 1, y, 2, state.glyphs.cursor, Role::Key);
        }
        let label_w = key_x.saturating_sub(inner.x + 4);
        match &row.unavailable {
            Some(reason) => {
                let text = format!("{} {} {reason}", row.label, state.glyphs.dot);
                p.text(inner.x + 3, y, label_w, &text, Role::Muted);
            }
            None => {
                let role = if cursor { Role::Key } else { Role::Text };
                p.text(inner.x + 3, y, label_w, &row.label, role);
            }
        }
        if let Some(key) = row.key {
            let text = key.to_string();
            let w = u16::try_from(width(&text)).unwrap_or(1);
            p.text(key_x + 1 - w, y, w, &text, Role::Key);
        }
        if cursor {
            p.cursor_row(line);
        }
        hits.push(line, HitTarget::Action(act(Action::RunRow(i))));
    }
    let rule = inner.bottom() - 2;
    p.rule(area, rule, Role::Focus);
    let close_y = inner.bottom() - 1;
    if popup.on_close {
        p.text(inner.x + 1, close_y, 2, state.glyphs.cursor, Role::Key);
    }
    let role = if popup.on_close {
        Role::Key
    } else {
        Role::Text
    };
    p.text(inner.x + 3, close_y, 5, "Close", role);
    let close = Rect::new(inner.x, close_y, inner.width, 1);
    if popup.on_close {
        p.cursor_row(close);
    }
    hits.push(close, HitTarget::Action(act(Action::Close)));
}
