//! 1 Library, Detail pane: the profile page of the selected game or of All
//! games.
//!
//! The game line, own settings, the section strip `[Proton] DLSS GPU CPU
//! DLLs`, rows of label, value and source with the focused row's
//! description, the editors and pickers under their field, Space to follow
//! or own a value, the VKD3D notice, and saves through [`AppState::save`]
//! with a [`crate::event::Write::Profile`] (terminal-ux.md, profile page
//! details). The DLLs section draws [`super::dlls`] and
//! [`super::denylist`].
//!
//! A placeholder for now: it draws the selected scope's game line and a
//! line that says what comes here. It has the full screen contract of
//! [`super::library`], so its owner fills this file alone.

use ratatui::Frame;
use spela_core::profile::Scope;

use super::{Row, library};
use crate::event::Cmd as AnyCmd;
use crate::hit::HitMap;
use crate::keymap::Binding;
use crate::layout::{Pane, PaneArea};
use crate::paint::Painter;
use crate::state::AppState;
use crate::theme::Role;

/// The profile page's state.
#[derive(Clone, Debug, Default)]
pub struct State {}

/// What the profile page's keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {}

/// The profile page's worker results, such as a loaded profile or the
/// VKD3D notice.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {}

/// The profile page's requests to the shell, run in the binary's
/// `tui/exec/profile.rs`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {}

/// The pane title: the game's name, or `All games`.
pub fn title(state: &AppState, _pane: Pane) -> String {
    match library::selected_scope(state) {
        Some(Scope::AllGames) => "All games".to_owned(),
        Some(Scope::Game(_)) => library::selected_game(state)
            .map(|g| g.name.clone())
            .unwrap_or_default(),
        None => "Profile".to_owned(),
    }
}

/// Whether an editor or picker is open.
pub fn editing(_state: &AppState, _pane: Pane) -> bool {
    false
}

/// The page's keys.
pub fn bindings(_state: &AppState, _pane: Pane) -> Vec<Binding> {
    Vec::new()
}

/// The Actions rows of the page.
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

/// The mouse wheel over the page.
pub fn wheel(_state: &mut AppState, _pane: Pane, _down: bool) -> Vec<AnyCmd> {
    Vec::new()
}

/// Draws the page.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, _hits: &mut HitMap) {
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = area.inner;
    if inner.height < 3 || inner.width < 3 {
        return;
    }
    let (x, w) = (inner.x + 1, inner.width - 2);
    let dot = state.glyphs.dot;
    match (
        library::selected_scope(state),
        library::selected_game(state),
    ) {
        (Some(Scope::Game(_)), Some(game)) => {
            let line = format!("{} {dot} App ID {}", game.name, game.app_id);
            p.text(x, inner.y, w, &line, Role::Title);
        }
        (Some(Scope::AllGames), _) => {
            p.text(x, inner.y, w, "All games", Role::Title);
        }
        _ => {
            p.text(
                x,
                inner.y,
                w,
                "Select a game from the scope list",
                Role::Muted,
            );
            return;
        }
    }
    p.text(
        x,
        inner.y + 2,
        w,
        "The profile page comes here: Proton, DLSS, GPU, CPU and DLLs.",
        Role::Muted,
    );
}
