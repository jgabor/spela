//! The migration notice: what this run migrated from Go spela, shown once
//! inside the TUI on the run that migrates. The CLI prints the same lines
//! to stderr for its commands; `tui` shows them here instead, because its
//! logs go to the file only.

use ratatui::Frame;
use ratatui::layout::Rect;

use super::{Action as AnyAction, Popup, PopupAction};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::Layout;
use crate::paint::Painter;
use crate::state::AppState;
use crate::theme::Role;

/// The popup's widest width.
pub const WIDTH: u16 = 96;

/// The notice.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct State {
    /// The notice lines, as `migrate::Report::notice` gives them.
    pub lines: Vec<String>,
    /// The first line shown.
    pub scroll: usize,
}

impl State {
    /// A notice of these lines.
    pub fn new(lines: Vec<String>) -> State {
        State { lines, scroll: 0 }
    }
}

/// What the notice's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// Scroll up.
    Up,
    /// Scroll down.
    Down,
    /// Close the notice.
    Close,
}

fn act(a: Action) -> AnyAction {
    AnyAction::Popup(PopupAction::Notice(a))
}

/// The notice's keys.
pub fn bindings(_state: &AppState, _popup: &State) -> Vec<Binding> {
    vec![
        Binding::new(Key::UP, act(Action::Up), "Scroll").hint_as("Up/Down", 50),
        Binding::new(Key::DOWN, act(Action::Down), "Scroll down"),
        Binding::new(Key::ENTER, act(Action::Close), "Close").hint(90),
        Binding::new(Key::ESC, act(Action::Close), "Close").hint(80),
    ]
}

/// Runs a notice action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let Some(Popup::Notice(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    match action {
        Action::Up => popup.scroll = popup.scroll.saturating_sub(1),
        Action::Down => popup.scroll = (popup.scroll + 1).min(popup.lines.len().saturating_sub(1)),
        Action::Close => state.close_popup(),
    }
    Vec::new()
}

/// Draws the notice.
pub fn render(
    state: &AppState,
    popup: &State,
    frame: &mut Frame,
    layout: &Layout,
    hits: &mut HitMap,
) {
    let inner_w = usize::from(WIDTH.min(layout.workspace().width).saturating_sub(6));
    let wrapped: Vec<String> = popup
        .lines
        .iter()
        .flat_map(|l| crate::text::wrap(l, inner_w))
        .collect();
    let h = u16::try_from(wrapped.len())
        .unwrap_or(u16::MAX)
        .saturating_add(6);
    let area = layout.centered(WIDTH, h);
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = p.popup(area, "Migrated from Go spela", hits);
    if inner.height < 4 || inner.width < 12 {
        return;
    }
    let visible = usize::from(inner.height - 4);
    for (n, line) in wrapped.iter().skip(popup.scroll).take(visible).enumerate() {
        let y = inner.y + 1 + u16::try_from(n).unwrap_or(0);
        p.text(inner.x + 2, y, inner.width - 4, line, Role::Text);
    }
    let close = "[ Close ]";
    let bx = inner.right().saturating_sub(2 + 9);
    let by = inner.bottom() - 2;
    p.text(bx, by, 9, close, Role::Key);
    p.cursor_row(Rect::new(bx, by, 9, 1));
    hits.push(
        Rect::new(bx, by, 9, 1),
        HitTarget::Action(act(Action::Close)),
    );
}
