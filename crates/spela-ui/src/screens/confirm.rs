//! The confirmation popup: a body, then `[ Cancel ]  [ Confirm ]`, with
//! focus on Cancel first (decision 4).
//!
//! Left and Right choose a button, Enter activates it, Tab moves between
//! the buttons and the body, which Up and Down scroll. Esc cancels; it
//! never confirms (decision 2). DLL install, update and restore, the
//! whole-page resets, Apply to games with their own value, the deny list
//! allow and the hardware actions all use it.
//!
//! A screen opens one with [`AppState::confirm`]:
//!
//! ```ignore
//! state.confirm(
//!     confirm::State::new("Confirm DLL update", body, action)
//!         .confirm_label("Confirm")
//!         .cancel_message("DLL update cancelled, no operation ran"),
//! );
//! ```
//!
//! Confirm closes the popup and runs `action` through
//! [`crate::update::run`]. [`asks`] says which changes ask first: only the
//! DLL update follows `confirm_destructive`.

use ratatui::Frame;
use ratatui::layout::Rect;

use super::{Action as AnyAction, Popup, PopupAction};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::Layout;
use crate::paint::Painter;
use crate::state::{AppState, Level};
use crate::text::width;
use crate::theme::Role;

/// The popup's widest width.
pub const WIDTH: u16 = 88;

/// One body line: spans of text in roles.
pub type Line = Vec<(String, Role)>;

/// Where focus is.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Focus {
    /// The body, which Up and Down scroll.
    Body,
    /// `[ Cancel ]`, where focus starts.
    Cancel,
    /// `[ Confirm ]`.
    Confirm,
}

/// A confirmation.
#[derive(Clone, Debug, PartialEq)]
pub struct State {
    /// The title in the border.
    pub title: String,
    /// The body lines.
    pub body: Vec<Line>,
    /// The confirm button's word.
    pub confirm_label: String,
    /// What Confirm runs.
    pub on_confirm: AnyAction,
    /// The status message on Cancel, such as `DLL update cancelled, no
    /// operation ran`.
    pub cancelled: Option<String>,
    /// Where focus is.
    pub focus: Focus,
    /// The first body line shown.
    pub scroll: usize,
}

impl State {
    /// A confirmation with focus on Cancel.
    pub fn new(title: impl Into<String>, body: Vec<Line>, on_confirm: AnyAction) -> State {
        State {
            title: title.into(),
            body,
            confirm_label: "Confirm".into(),
            on_confirm,
            cancelled: None,
            focus: Focus::Cancel,
            scroll: 0,
        }
    }

    /// Another word on the confirm button.
    pub fn confirm_label(mut self, label: impl Into<String>) -> State {
        self.confirm_label = label.into();
        self
    }

    /// The status message when the user cancels.
    pub fn cancel_message(mut self, text: impl Into<String>) -> State {
        self.cancelled = Some(text.into());
        self
    }
}

/// What a confirmation guards, for [`asks`].
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Guard {
    /// A DLL update, one game's or a batch.
    DllUpdate,
    /// A DLL install.
    DllInstall,
    /// A DLL restore.
    DllRestore,
    /// A whole-page "for all settings" reset.
    PageReset,
    /// Apply to games with their own value.
    ApplyToGames,
    /// GPU reset, CPU governor or SMT.
    Hardware,
    /// Allowing DLL swaps for a game on a deny list.
    AllowDllSwaps,
}

/// Whether a change asks first. `confirm_destructive = false` skips only
/// the DLL update confirmation, as in Go (TUI 10). Install, restore, the
/// whole-page resets, Apply to games with their own value, the hardware
/// actions and the deny list allow always ask (decisions 4 and 44).
pub fn asks(state: &AppState, guard: Guard) -> bool {
    match guard {
        Guard::DllUpdate => state.config.confirm_destructive,
        Guard::DllInstall
        | Guard::DllRestore
        | Guard::PageReset
        | Guard::ApplyToGames
        | Guard::Hardware
        | Guard::AllowDllSwaps => true,
    }
}

/// Plain body lines from text, one role for all.
pub fn lines(text: &[&str], role: Role) -> Vec<Line> {
    text.iter().map(|t| vec![((*t).to_owned(), role)]).collect()
}

/// What the popup's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// Left: the previous button.
    Left,
    /// Right: the next button.
    Right,
    /// Tab: between the body and the buttons.
    Tab,
    /// Up: scroll the body.
    Up,
    /// Down: scroll the body.
    Down,
    /// Enter: activate the focused button.
    Activate,
    /// Esc, or a click on Cancel: cancel.
    Cancel,
    /// A click on Confirm.
    Confirm,
}

fn act(a: Action) -> AnyAction {
    AnyAction::Popup(PopupAction::Confirm(a))
}

/// The body rows the popup shows at the current size.
fn visible_rows(state: &AppState, popup: &State) -> usize {
    let body = u16::try_from(popup.body.len()).unwrap_or(u16::MAX);
    let area = state.layout().centered(WIDTH, body.saturating_add(6));
    usize::from(area.height.saturating_sub(6))
}

/// Whether Up and Down have anything to scroll, or why not.
fn can_scroll(state: &AppState, popup: &State) -> Result<(), String> {
    if popup.body.len() > visible_rows(state, popup) {
        Ok(())
    } else {
        Err("nothing to scroll".into())
    }
}

/// The popup's keys.
pub fn bindings(state: &AppState, popup: &State) -> Vec<Binding> {
    if popup.focus == Focus::Body {
        let scroll = can_scroll(state, popup);
        return vec![
            Binding::new(Key::UP, act(Action::Up), "Scroll")
                .hint_as("Up/Down", 100)
                .when(scroll.clone()),
            Binding::new(Key::DOWN, act(Action::Down), "Scroll down").when(scroll),
            Binding::new(Key::TAB, act(Action::Tab), "Buttons")
                .also(Key::BACK_TAB)
                .hint(50),
            Binding::new(Key::ESC, act(Action::Cancel), "Cancel").hint(40),
        ];
    }
    vec![
        Binding::new(Key::LEFT, act(Action::Left), "Choose").hint_as("Left/Right", 100),
        Binding::new(Key::RIGHT, act(Action::Right), "Next button"),
        Binding::new(Key::ENTER, act(Action::Activate), "Activate").hint(90),
        Binding::new(Key::TAB, act(Action::Tab), "Details")
            .also(Key::BACK_TAB)
            .hint(50),
        Binding::new(Key::ESC, act(Action::Cancel), "Cancel").hint(40),
    ]
}

/// Runs a popup action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let Some(Popup::Confirm(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    match action {
        Action::Left | Action::Right => {
            popup.focus = match popup.focus {
                Focus::Cancel => Focus::Confirm,
                Focus::Confirm | Focus::Body => Focus::Cancel,
            };
        }
        Action::Tab => {
            popup.focus = match popup.focus {
                Focus::Body => Focus::Cancel,
                Focus::Cancel | Focus::Confirm => Focus::Body,
            };
        }
        Action::Up => popup.scroll = popup.scroll.saturating_sub(1),
        Action::Down => popup.scroll = (popup.scroll + 1).min(popup.body.len().saturating_sub(1)),
        Action::Activate => match popup.focus {
            Focus::Confirm => return confirm(state),
            Focus::Cancel => return cancel(state),
            Focus::Body => {}
        },
        Action::Cancel => return cancel(state),
        Action::Confirm => return confirm(state),
    }
    Vec::new()
}

fn cancel(state: &mut AppState) -> Vec<AnyCmd> {
    if let Some(Popup::Confirm(popup)) = state.popup.take() {
        state.dirty = true;
        if let Some(text) = popup.cancelled {
            state.say(Level::Info, text);
        }
    }
    Vec::new()
}

fn confirm(state: &mut AppState) -> Vec<AnyCmd> {
    match state.popup.take() {
        Some(Popup::Confirm(popup)) => {
            state.dirty = true;
            crate::update::run(state, popup.on_confirm)
        }
        other => {
            state.popup = other;
            Vec::new()
        }
    }
}

/// Draws the popup.
pub fn render(
    state: &AppState,
    popup: &State,
    frame: &mut Frame,
    layout: &Layout,
    hits: &mut HitMap,
) {
    let body = u16::try_from(popup.body.len()).unwrap_or(u16::MAX);
    let area = layout.centered(WIDTH, body.saturating_add(6));
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = p.popup(area, &popup.title, hits);
    if inner.height < 4 || inner.width < 26 {
        return;
    }
    let x = inner.x + 2;
    let w = inner.width.saturating_sub(4);
    let visible = visible_rows(state, popup);
    for (n, line) in popup
        .body
        .iter()
        .skip(popup.scroll)
        .take(visible)
        .enumerate()
    {
        let y = inner.y + 1 + u16::try_from(n).unwrap_or(0);
        let spans: Vec<(&str, Role)> = line.iter().map(|(t, r)| (t.as_str(), *r)).collect();
        p.spans(x, y, w, &spans);
    }
    if popup.focus == Focus::Body {
        p.thumb(
            area.right() - 1,
            Rect::new(0, inner.y + 1, 1, inner.height - 4),
            popup.scroll,
            visible,
            popup.body.len(),
        );
    }
    let cancel = "[ Cancel ]";
    let confirm = format!("[ {} ]", popup.confirm_label);
    let total = u16::try_from(width(cancel) + 2 + width(&confirm)).unwrap_or(0);
    let bx = inner.right().saturating_sub(2 + total);
    let by = inner.bottom() - 2;
    let cancel_w = u16::try_from(width(cancel)).unwrap_or(0);
    let confirm_w = u16::try_from(width(&confirm)).unwrap_or(0);
    let (cancel_role, confirm_role) = match popup.focus {
        Focus::Cancel => (Role::Key, Role::Text),
        Focus::Confirm => (Role::Text, Role::Key),
        Focus::Body => (Role::Text, Role::Text),
    };
    p.text(bx, by, cancel_w, cancel, cancel_role);
    let cx = bx + cancel_w + 2;
    p.text(cx, by, confirm_w, &confirm, confirm_role);
    match popup.focus {
        Focus::Cancel => p.cursor_row(Rect::new(bx, by, cancel_w, 1)),
        Focus::Confirm => p.cursor_row(Rect::new(cx, by, confirm_w, 1)),
        Focus::Body => {}
    }
    hits.push(
        Rect::new(bx, by, cancel_w, 1),
        HitTarget::Action(act(Action::Cancel)),
    );
    hits.push(
        Rect::new(cx, by, confirm_w, 1),
        HitTarget::Action(act(Action::Confirm)),
    );
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;

    #[test]
    fn only_a_dll_update_follows_confirm_destructive() {
        let mut s = state_with_games(120, 40, 1);
        let mut cfg = s.config.clone();
        cfg.confirm_destructive = false;
        s.set_config(cfg);
        assert!(!asks(&s, Guard::DllUpdate));
        for g in [
            Guard::DllInstall,
            Guard::DllRestore,
            Guard::PageReset,
            Guard::ApplyToGames,
            Guard::Hardware,
            Guard::AllowDllSwaps,
        ] {
            assert!(asks(&s, g), "{g:?}");
        }
    }

    #[test]
    fn focus_starts_on_cancel_and_esc_never_confirms() {
        let mut s = state_with_games(120, 40, 2);
        s.confirm(
            State::new("Confirm", lines(&["Body"], Role::Text), AnyAction::Quit)
                .cancel_message("Nothing ran"),
        );
        let cmds = step(&mut s, key("enter"));
        assert!(cmds.is_empty() && !s.quit, "Enter on Cancel cancels");
        assert_eq!(s.status.as_ref().unwrap().text, "Nothing ran");
        s.confirm(State::new("Confirm", Vec::new(), AnyAction::Quit));
        step(&mut s, key("esc"));
        assert!(s.popup.is_none() && !s.quit);
        s.confirm(State::new("Confirm", Vec::new(), AnyAction::Quit));
        step(&mut s, key("right"));
        let cmds = step(&mut s, key("enter"));
        assert_eq!(cmds, vec![AnyCmd::Quit]);
    }

    #[test]
    fn scroll_works_only_when_the_body_does_not_fit() {
        let scroll_shown = |s: &AppState| {
            crate::screens::bindings(s)
                .iter()
                .any(|b| b.label == "Scroll" && b.available())
        };
        let mut s = state_with_games(120, 40, 3);
        s.confirm(State::new(
            "Short",
            lines(&["one"], Role::Text),
            AnyAction::Help,
        ));
        step(&mut s, key("tab"));
        assert!(!scroll_shown(&s), "one line fits");
        let long: Vec<String> = (0..80).map(|i| format!("line {i}")).collect();
        let long: Vec<&str> = long.iter().map(String::as_str).collect();
        s.confirm(State::new(
            "Long",
            lines(&long, Role::Text),
            AnyAction::Help,
        ));
        step(&mut s, key("tab"));
        assert!(scroll_shown(&s), "80 lines do not");
    }
}
