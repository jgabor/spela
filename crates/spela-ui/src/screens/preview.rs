//! Preview launch: the read-only dry-run summary in a large popup
//! (decision 44). Phase 6 fills it.
//!
//! The body is the text `spela launch <game> --dry-run` prints, built on a
//! worker as [`crate::Cmd::Preview`] and answered with
//! [`crate::Msg::PreviewReady`]. While it builds, the body reads `Building
//! the launch plan...`. It never takes a lock and never runs pkexec.

use ratatui::Frame;

use super::{Action as AnyAction, Popup, PopupAction};
use crate::event::Cmd as AnyCmd;
use crate::hit::HitMap;
use crate::keymap::{Binding, Key};
use crate::layout::Layout;
use crate::paint::Painter;
use crate::state::AppState;
use crate::theme::Role;

/// The last line of every preview (decision 40).
pub const READ_ONLY: &str = "Read only. To launch with these settings, set the game's Steam launch options to: spela %command%";

/// The Preview launch popup.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct State {
    /// The request the summary answers.
    pub request: u64,
    /// The game's name, for the title.
    pub name: String,
    /// The summary, once built, or why it failed.
    pub summary: Option<Result<String, String>>,
    /// The first line shown.
    pub scroll: usize,
}

/// What the popup's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// Scroll up.
    Up,
    /// Scroll down.
    Down,
    /// Close the popup.
    Close,
}

fn act(a: Action) -> AnyAction {
    AnyAction::Popup(PopupAction::Preview(a))
}

/// A summary arrived. A stale one is dropped.
pub fn ready(state: &mut AppState, request: u64, result: Result<String, String>) {
    if let Some(Popup::Preview(p)) = &mut state.popup
        && p.request == request
    {
        p.summary = Some(result);
        state.dirty = true;
    }
}

/// The popup's keys.
pub fn bindings(_state: &AppState, _popup: &State) -> Vec<Binding> {
    vec![
        Binding::new(Key::UP, act(Action::Up), "Scroll").hint_as("Up/Down", 100),
        Binding::new(Key::DOWN, act(Action::Down), "Scroll down"),
        Binding::new(Key::ESC, act(Action::Close), "Close").hint(80),
    ]
}

/// Runs a popup action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let Some(Popup::Preview(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    match action {
        Action::Up => popup.scroll = popup.scroll.saturating_sub(1),
        Action::Down => popup.scroll += 1,
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
    let area = layout.centered(ws.width, ws.height);
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let title = format!("Preview launch {} {}", state.glyphs.dot, popup.name);
    let inner = p.popup(area, &title, hits);
    if inner.height < 3 || inner.width < 10 {
        return;
    }
    let (text, role) = match &popup.summary {
        None => ("Building the launch plan...".to_owned(), Role::Muted),
        Some(Ok(s)) => (format!("{s}\n\n{READ_ONLY}"), Role::Text),
        Some(Err(e)) => (format!("Unavailable: {e}"), Role::Error),
    };
    let lines: Vec<&str> = text.lines().collect();
    for (n, line) in lines
        .iter()
        .skip(popup.scroll)
        .take(usize::from(inner.height - 2))
        .enumerate()
    {
        let y = inner.y + 1 + u16::try_from(n).unwrap_or(0);
        p.text(inner.x + 2, y, inner.width - 4, line, role);
    }
}
