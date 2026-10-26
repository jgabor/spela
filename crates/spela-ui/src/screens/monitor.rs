//! 3 Monitor: GPU, CPU and Alerts in List, sparklines and gauges in Detail.
//!
//! It reads `AppState::metrics`, the same samples the top bar shows. The 2 s poll is the shell's. Its Actions rows hold the hardware actions (decision 44), in Phase 6.
//!
//! A placeholder for now: it draws its pane titles and a line that says
//! what comes here. It has the full screen contract of
//! [`super::library`], so its owner fills this file alone.

use ratatui::Frame;

use super::Row;
use crate::event::Cmd as AnyCmd;
use crate::hit::HitMap;
use crate::keymap::Binding;
use crate::layout::{Pane, PaneArea};
use crate::paint::Painter;
use crate::state::AppState;
use crate::theme::Role;

/// This screen's state.
#[derive(Clone, Debug, Default)]
pub struct State {}

/// What this screen's keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {}

/// This screen's worker results.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {}

/// This screen's requests to the shell, run in the binary's
/// `tui/exec/monitor.rs`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {}

/// The pane title.
pub fn title(_state: &AppState, pane: Pane) -> String {
    match pane {
        Pane::List => "Monitor".to_owned(),
        Pane::Detail => "GPU".to_owned(),
    }
}

/// Whether an editor or picker is open.
pub fn editing(_state: &AppState, _pane: Pane) -> bool {
    false
}

/// The keys of a pane.
pub fn bindings(_state: &AppState, _pane: Pane) -> Vec<Binding> {
    Vec::new()
}

/// The Actions rows of a pane.
pub fn actions(_state: &AppState, _pane: Pane) -> Vec<Row> {
    Vec::new()
}

/// The palette rows.
pub fn palette(_state: &AppState) -> Vec<Row> {
    Vec::new()
}

/// Runs an action.
pub fn update(_state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    match action {}
}

/// Takes a worker result.
pub fn on_msg(_state: &mut AppState, msg: Msg) -> Vec<AnyCmd> {
    match msg {}
}

/// Takes a save's result.
pub fn saved(_state: &mut AppState, _token: u64, _result: Result<(), String>) -> Vec<AnyCmd> {
    Vec::new()
}

/// The mouse wheel over a pane.
pub fn wheel(_state: &mut AppState, _pane: Pane, _down: bool) -> Vec<AnyCmd> {
    Vec::new()
}

/// Draws a pane.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, _hits: &mut HitMap) {
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = area.inner;
    if inner.height == 0 || inner.width < 3 {
        return;
    }
    let text = match area.pane {
        Pane::List => "GPU, CPU and Alerts come here.",
        Pane::Detail => "Sparklines, gauges and the alert list come here.",
    };
    for (i, line) in crate::text::wrap(text, usize::from(inner.width - 2))
        .iter()
        .enumerate()
    {
        let y = inner.y + u16::try_from(i).unwrap_or(u16::MAX);
        if y >= inner.bottom() {
            break;
        }
        p.text(inner.x + 1, y, inner.width - 2, line, Role::Muted);
    }
}
