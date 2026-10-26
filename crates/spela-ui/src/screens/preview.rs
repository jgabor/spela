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

/// The body text and its role.
fn text(popup: &State) -> (String, Role) {
    match &popup.summary {
        None => ("Building the launch plan...".to_owned(), Role::Muted),
        Some(Ok(s)) => (format!("{s}\n\n{READ_ONLY}"), Role::Text),
        Some(Err(e)) => (format!("Unavailable: {e}"), Role::Error),
    }
}

/// The body lines wrapped to `cols` cells. A long line, such as an
/// environment variable or an impacts item, continues on the next row,
/// indented two cells past its own indent, so no part of the plan is cut.
fn wrapped(popup: &State, cols: usize) -> (Vec<String>, Role) {
    let (text, role) = text(popup);
    let mut out = Vec::new();
    for line in text.lines() {
        let body = line.trim_start();
        if body.is_empty() {
            out.push(String::new());
            continue;
        }
        let indent = line.len() - body.len();
        if crate::text::width(line) <= cols {
            out.push(line.to_owned());
            continue;
        }
        let pieces = crate::text::wrap(body, cols.saturating_sub(indent + 2).max(1));
        for (n, piece) in pieces.into_iter().enumerate() {
            let pad = if n == 0 { indent } else { indent + 2 };
            out.push(format!("{}{piece}", " ".repeat(pad)));
        }
    }
    (out, role)
}

/// The cells a body line may take: the popup less its borders and margins.
fn body_cols(state: &AppState) -> usize {
    let ws = state.layout().workspace();
    let area = state.layout().centered(ws.width, ws.height);
    usize::from(area.width.saturating_sub(6))
}

/// The body rows the popup shows: it fills the workspace.
fn visible_rows(state: &AppState) -> usize {
    let ws = state.layout().workspace();
    let area = state.layout().centered(ws.width, ws.height);
    usize::from(area.height.saturating_sub(4))
}

/// How far the body can scroll: zero when it all fits.
fn max_scroll(state: &AppState, popup: &State) -> usize {
    wrapped(popup, body_cols(state))
        .0
        .len()
        .saturating_sub(visible_rows(state))
}

/// The popup's keys.
pub fn bindings(state: &AppState, popup: &State) -> Vec<Binding> {
    let scroll = if max_scroll(state, popup) > 0 {
        Ok(())
    } else {
        Err("nothing to scroll".to_owned())
    };
    vec![
        Binding::new(Key::UP, act(Action::Up), "Scroll")
            .hint_as("Up/Down", 100)
            .when(scroll.clone()),
        Binding::new(Key::DOWN, act(Action::Down), "Scroll down").when(scroll),
        Binding::new(Key::ESC, act(Action::Close), "Close").hint(80),
    ]
}

/// Runs a popup action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let max = match &state.popup {
        Some(Popup::Preview(popup)) => max_scroll(state, popup),
        _ => return Vec::new(),
    };
    let Some(Popup::Preview(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    match action {
        Action::Up => popup.scroll = popup.scroll.saturating_sub(1),
        Action::Down => popup.scroll = (popup.scroll + 1).min(max),
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
    let (lines, role) = wrapped(popup, usize::from(inner.width.saturating_sub(4)));
    for (n, line) in lines
        .iter()
        .skip(popup.scroll)
        .take(visible_rows(state))
        .enumerate()
    {
        let y = inner.y + 1 + u16::try_from(n).unwrap_or(0);
        p.text(inner.x + 2, y, inner.width - 4, line, role);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn long_lines_wrap_under_their_indent_and_nothing_is_cut() {
        let popup = State {
            request: 1,
            name: "Glimmer Run".into(),
            summary: Some(Ok(
                "Environment\n  DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_k\n  short".into(),
            )),
            scroll: 0,
        };
        let (lines, role) = wrapped(&popup, 40);
        assert_eq!(role, Role::Text);
        assert_eq!(lines[0], "Environment");
        let short = lines.iter().position(|l| l == "  short").unwrap();
        let var = &lines[1..short];
        assert!(var.len() > 1, "{lines:?}");
        assert!(var[0].starts_with("  DXVK_"), "{var:?}");
        assert!(var[1..].iter().all(|l| l.starts_with("    ")), "{var:?}");
        assert!(
            lines.iter().all(|l| crate::text::width(l) <= 40),
            "{lines:?}"
        );
        let joined: String = var.iter().map(|l| l.trim()).collect();
        assert_eq!(
            joined,
            "DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_k"
        );
    }
}
