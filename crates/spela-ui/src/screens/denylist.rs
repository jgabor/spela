//! The deny list surface: one game's state line and the Allow, Deny and
//! Clear actions (terminal-ux.md, denylist; decisions 20 and 36).
//!
//! It draws no pane of its own. The game's DLLs section shows
//! [`state_line`] under "DLL versions" and lists [`actions`] in its Actions
//! popup, after the DLL rows; the palette lists them too. The DLL Install
//! and Update rows dim with [`swap_allowed`].
//!
//! - The lists load on the job pool ([`Cmd::Load`]) at start and again
//!   after every change, because spela-ui reads no files. A list that does
//!   not load shows `Denylist: unreadable (<file>)`, and the rows wait.
//! - Allow opens the confirmation with Go's warning and Cancel focused.
//!   `confirm_destructive = false` never skips it.
//! - Deny opens a one-line editor in Detail for the reason, prefilled
//!   `user-specified`, as the CLI's default. While it is open,
//!   [`crate::screens`] gives it Library Detail's keys and draws it there.
//! - Clear saves at once, because it only removes the user's own entry.
//!
//! Each row saves a [`Write::Deny`] through the write queue, which calls
//! the same core function as `denylist allow`, `deny` and `clear`.

use std::collections::BTreeMap;

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::denylist::{DEFAULT_REASON, DenyLists, Source, Status};
use spela_core::games::Game;

use super::{Action as AnyAction, Dest, Owner, Row, ScreenAction, confirm, library};
use crate::editor::{Editor, EditorInput, EditorOutcome};
use crate::event::{Cmd as AnyCmd, DenyChange, Write};
use crate::hit::HitMap;
use crate::keymap::Binding;
use crate::layout::{Pane, PaneArea};
use crate::paint::Painter;
use crate::state::{AppState, Level};
use crate::theme::Role;

/// Go's warning on `denylist allow`.
pub const ALLOW_WARNING: &str = "WARNING: Swapping DLLs in anti-cheat games may result in bans!";

/// A list file that did not load.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Unreadable {
    /// The file's name, for the state line.
    pub file: String,
    /// The whole error.
    pub error: String,
}

/// The open reason editor.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct DenyEditor {
    /// The game.
    pub app_id: u64,
    /// Its name.
    pub name: String,
    /// The editor.
    pub editor: Editor,
}

/// A save on its way.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Pending {
    /// The game.
    pub app_id: u64,
    /// Its name.
    pub name: String,
    /// The change.
    pub change: DenyChange,
}

/// The deny list surface's state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The lists, or why they did not load. `None` while loading.
    pub lists: Option<Result<DenyLists, Unreadable>>,
    /// The load on its way.
    pub request: Option<u64>,
    /// The reason editor of Deny.
    pub editor: Option<DenyEditor>,
    /// Saves on their way, by token.
    pub pending: BTreeMap<u64, Pending>,
}

/// What the rows and the editor's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// Opens the Allow confirmation.
    Allow {
        /// The game.
        app_id: u64,
        /// Its name.
        name: String,
    },
    /// Opens the reason editor.
    Deny {
        /// The game.
        app_id: u64,
        /// Its name.
        name: String,
    },
    /// Saves a change: Confirm on Allow, Apply on Deny, and Clear.
    Apply {
        /// The game.
        app_id: u64,
        /// Its name.
        name: String,
        /// The change.
        change: DenyChange,
    },
    /// A key for the reason editor.
    Editor(EditorInput),
}

/// The lists, loaded.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {
    /// A load finished.
    Loaded {
        /// The request this answers.
        request: u64,
        /// The lists, or why they did not load.
        result: Result<DenyLists, Unreadable>,
    },
}

/// What the deny list asks the shell for, run in the binary's
/// `tui/exec/denylist.rs`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {
    /// Load the built-in list, the user list and the overrides.
    Load {
        /// The request id the result carries.
        request: u64,
    },
}

fn act(a: Action) -> AnyAction {
    AnyAction::Screen(ScreenAction::Denylist(a))
}

fn editor_input(input: EditorInput) -> AnyAction {
    act(Action::Editor(input))
}

fn editor_typed(c: char) -> AnyAction {
    act(Action::Editor(EditorInput::Type(c)))
}

/// Loads the lists on a worker. [`crate::AppState::start`] calls it, and
/// so does every finished change.
pub fn start(state: &mut AppState) -> Vec<AnyCmd> {
    let request = state.request();
    state.denylist.request = Some(request);
    vec![AnyCmd::Denylist(Cmd::Load { request })]
}

fn lists(state: &AppState) -> Result<&DenyLists, String> {
    match &state.denylist.lists {
        None => Err("wait for the deny list".into()),
        Some(Err(u)) => Err(format!("deny list unreadable ({})", u.file)),
        Some(Ok(l)) => Ok(l),
    }
}

/// The game's line under "DLL versions", only when a list names the game
/// or the user allowed it, or when a list does not load. Most games show
/// nothing (terminal-ux.md, denylist).
pub fn state_line(state: &AppState, app_id: u64) -> Option<(String, Role)> {
    let l = match &state.denylist.lists {
        None => return None,
        Some(Err(u)) => return Some((format!("Denylist: unreadable ({})", u.file), Role::Error)),
        Some(Ok(l)) => l,
    };
    match l.check(app_id) {
        Status::Allowed => None,
        Status::ForceAllowed { over: Some(_) } => {
            Some(("Denylist: force-allowed".to_owned(), Role::Warn))
        }
        Status::ForceAllowed { over: None } => {
            Some(("Denylist: allowed by you".to_owned(), Role::Muted))
        }
        Status::Denied(listed) => {
            let reason = &listed.entry.reason;
            let text = match listed.source {
                Source::UserDenied => format!("Denylist: denied by you ({reason})"),
                Source::BuiltIn | Source::UserList => format!("Denylist: denied ({reason})"),
            };
            Some((text, Role::Warn))
        }
    }
}

/// Whether DLL install and update may run for the game, or why not, for
/// the DLL rows: `DLL swaps denied (<reason>)`. A list that does not load
/// fails closed. Restore never asks.
pub fn swap_allowed(state: &AppState, app_id: u64) -> Result<(), String> {
    let l = lists(state)?;
    match l.check(app_id) {
        Status::Denied(listed) => Err(format!("DLL swaps denied ({})", listed.entry.reason)),
        _ => Ok(()),
    }
}

/// Whether a loaded deny list denies the game.
pub fn is_denied(state: &AppState, app_id: u64) -> bool {
    lists(state).is_ok_and(|l| l.check(app_id).is_denied())
}

/// The three rows for a game, with their reasons when they cannot run.
pub fn rows(state: &AppState, game: &Game) -> Vec<Row> {
    let (app_id, name) = (game.app_id, game.name.clone());
    let l = lists(state);
    let allow = l.clone().and_then(|l| {
        if l.is_force_allowed(app_id) {
            Err("already allowed".to_owned())
        } else {
            Ok(())
        }
    });
    // A game a list already denies needs no deny of its own; Clear or
    // Allow are the ways out.
    let deny = l.clone().and_then(|l| {
        if l.check(app_id).is_denied() {
            Err("already denied".to_owned())
        } else {
            Ok(())
        }
    });
    let clear = l.and_then(|l| {
        let own = l.is_force_allowed(app_id) || l.user_denied().iter().any(|e| e.app_id == app_id);
        if own {
            Ok(())
        } else {
            Err("no override".to_owned())
        }
    });
    vec![
        Row::new(
            format!("Allow DLL swaps: {name}"),
            act(Action::Allow {
                app_id,
                name: name.clone(),
            }),
        )
        .when(allow),
        Row::new(
            format!("Deny DLL swaps: {name}"),
            act(Action::Deny {
                app_id,
                name: name.clone(),
            }),
        )
        .when(deny),
        Row::new(
            format!("Clear denylist override: {name}"),
            act(Action::Apply {
                app_id,
                name,
                change: DenyChange::Clear,
            }),
        )
        .when(clear),
    ]
}

/// The pane title. The deny list draws inside the game's DLLs section.
pub fn title(_state: &AppState, _pane: Pane) -> String {
    "Deny list".to_owned()
}

/// Whether the reason editor is open: then it takes typed keys.
pub fn editing(state: &AppState, _pane: Pane) -> bool {
    state.denylist.editor.is_some()
}

/// The reason editor's keys, while it is open.
pub fn bindings(state: &AppState, _pane: Pane) -> Vec<Binding> {
    match &state.denylist.editor {
        Some(e) => e.editor.bindings(editor_input, editor_typed),
        None => Vec::new(),
    }
}

/// The rows for the game selected in Library, on its Detail pane. The DLLs
/// section lists them after its DLL rows.
pub fn actions(state: &AppState, pane: Pane) -> Vec<Row> {
    match (pane, library::selected_game(state)) {
        (Pane::Detail, Some(game)) => rows(state, game),
        _ => Vec::new(),
    }
}

/// The rows for the game selected in Library.
pub fn palette(state: &AppState) -> Vec<Row> {
    library::selected_game(state)
        .map(|g| rows(state, g))
        .unwrap_or_default()
}

/// Runs an action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    match action {
        Action::Allow { app_id, name } => {
            let mut body = vec![vec![(
                format!("{name} {} App ID {app_id}", state.glyphs.dot),
                Role::Strong,
            )]];
            if let Some(line) = state_line(state, app_id) {
                body.push(vec![line]);
            }
            body.push(Vec::new());
            body.push(vec![(
                "DLL install and update will run for this game.".to_owned(),
                Role::Text,
            )]);
            body.push(vec![(ALLOW_WARNING.to_owned(), Role::Warn)]);
            let apply = act(Action::Apply {
                app_id,
                name: name.clone(),
                change: DenyChange::Allow,
            });
            // The allow always asks: confirm_destructive covers DLL
            // updates only.
            if !confirm::asks(state, confirm::Guard::AllowDllSwaps) {
                return crate::update::run(state, apply);
            }
            state.confirm(
                confirm::State::new("Allow DLL swaps", body, apply)
                    .cancel_message("DLL swaps not allowed, nothing changed"),
            );
            Vec::new()
        }
        Action::Deny { app_id, name } => {
            let editor = Editor::new(format!("Deny DLL swaps: {name}"), None, DEFAULT_REASON)
                .hint(format!("The reason. Empty uses {DEFAULT_REASON}."));
            state.denylist.editor = Some(DenyEditor {
                app_id,
                name,
                editor,
            });
            state.close_popup();
            state.dest = Dest::Library;
            state.focus = Pane::Detail;
            // The editor opens in the game's DLLs section, under its state
            // line, where the swaps it guards are.
            if library::selected_game(state).is_some_and(|g| g.app_id == app_id) {
                state.profile.aspect = super::profile::Aspect::Dlls;
            }
            Vec::new()
        }
        Action::Apply {
            app_id,
            name,
            change,
        } => {
            let (token, cmd) = state.save(
                Owner::Denylist,
                Write::Deny {
                    app_id,
                    name: name.clone(),
                    change: change.clone(),
                },
            );
            state.denylist.pending.insert(
                token,
                Pending {
                    app_id,
                    name,
                    change,
                },
            );
            vec![cmd]
        }
        Action::Editor(input) => {
            let Some(open) = &mut state.denylist.editor else {
                return Vec::new();
            };
            match open.editor.handle(input) {
                EditorOutcome::Moved => Vec::new(),
                EditorOutcome::Cancel => {
                    state.denylist.editor = None;
                    Vec::new()
                }
                EditorOutcome::Apply(text) => {
                    let Some(open) = state.denylist.editor.take() else {
                        return Vec::new();
                    };
                    let reason = match text.trim() {
                        "" => DEFAULT_REASON.to_owned(),
                        t => t.to_owned(),
                    };
                    update(
                        state,
                        Action::Apply {
                            app_id: open.app_id,
                            name: open.name,
                            change: DenyChange::Deny { reason },
                        },
                    )
                }
            }
        }
    }
}

/// Takes the loaded lists. A stale load is dropped.
pub fn on_msg(state: &mut AppState, msg: Msg) -> Vec<AnyCmd> {
    match msg {
        Msg::Loaded { request, result } => {
            if state.denylist.request == Some(request) {
                state.denylist.request = None;
                state.denylist.lists = Some(result);
                state.dirty = true;
            }
            Vec::new()
        }
    }
}

/// A change finished: the status line says so, and the lists load again.
pub fn saved(state: &mut AppState, token: u64, result: Result<(), String>) -> Vec<AnyCmd> {
    let Some(p) = state.denylist.pending.remove(&token) else {
        return Vec::new();
    };
    match result {
        Ok(()) => {
            let text = match &p.change {
                DenyChange::Allow => format!("Saved: DLL swaps allowed for {}", p.name),
                DenyChange::Deny { reason } => {
                    format!("Saved: DLL swaps denied for {} ({reason})", p.name)
                }
                DenyChange::Clear => format!("Saved: denylist override cleared for {}", p.name),
            };
            state.say(Level::Ok, text);
        }
        Err(e) => state.say(Level::Error, format!("Not saved: {e}")),
    }
    start(state)
}

/// The deny list scrolls nothing.
pub fn wheel(_state: &mut AppState, _pane: Pane, _down: bool) -> Vec<AnyCmd> {
    Vec::new()
}

/// The open reason editor, when it is for `app_id`.
pub fn editor_for(state: &AppState, app_id: u64) -> Option<&crate::editor::Editor> {
    state
        .denylist
        .editor
        .as_ref()
        .filter(|open| open.app_id == app_id)
        .map(|open| &open.editor)
}

/// Whether Detail shows the editor's game's DLLs section, which draws the
/// editor under its state line itself.
fn drawn_inline(state: &AppState) -> bool {
    let Some(open) = &state.denylist.editor else {
        return false;
    };
    state.dest == Dest::Library
        && super::profile::aspect(state) == super::profile::Aspect::Dlls
        && library::selected_game(state).is_some_and(|g| g.app_id == open.app_id)
}

/// Draws the reason editor over Library's Detail, with the game's state
/// line above it, while it is open and the game's DLLs section is not in
/// view. The DLLs section draws it under its own state line otherwise.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, _hits: &mut HitMap) {
    if state.denylist.editor.is_none() || area.pane != Pane::Detail || drawn_inline(state) {
        return;
    }
    let inner = area.inner;
    if inner.height < 4 || inner.width < 20 {
        return;
    }
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let top = inner.y + 2.min(inner.height - 1);
    let rest = Rect::new(inner.x, top, inner.width, inner.bottom() - top);
    render_editor(state, &mut p, rest);
}

/// Draws the state line and the reason editor at the top of `area`.
/// Returns the rows used.
pub fn render_editor(state: &AppState, p: &mut Painter<'_>, area: Rect) -> u16 {
    let Some(open) = &state.denylist.editor else {
        return 0;
    };
    let (x, w) = (area.x + 1, area.width.saturating_sub(2));
    let h = (open.editor.height(w) + 2).min(area.height);
    p.clear(Rect::new(area.x, area.y, area.width, h));
    let mut y = area.y;
    if let Some((text, role)) = state_line(state, open.app_id) {
        p.text(x, y, w, &text, role);
    }
    y += 1;
    let rest = area.bottom().saturating_sub(y);
    open.editor
        .render(p, Rect::new(x, y, w, open.editor.height(w).min(rest)));
    h
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::screens::Popup;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;
    use spela_core::denylist::{DenyEntry, Overrides, UserList};

    fn loaded(s: &mut AppState, overrides: Overrides) {
        let cmds = start(s);
        let AnyCmd::Denylist(Cmd::Load { request }) = cmds[0] else {
            panic!("{cmds:?}")
        };
        on_msg(
            s,
            Msg::Loaded {
                request,
                result: Ok(DenyLists::new(UserList::default(), overrides)),
            },
        );
    }

    fn reasons(s: &AppState) -> Vec<Option<String>> {
        let game = library::selected_game(s).unwrap();
        rows(s, game).into_iter().map(|r| r.unavailable).collect()
    }

    #[test]
    fn the_rows_wait_for_the_lists_and_show_their_reasons() {
        let mut s = state_with_games(120, 40, 3);
        let id = library::selected_game(&s).unwrap().app_id;
        assert!(
            reasons(&s)
                .iter()
                .all(|r| r.as_deref() == Some("wait for the deny list"))
        );
        loaded(&mut s, Overrides::default());
        assert_eq!(state_line(&s, id), None, "most games show nothing");
        assert_eq!(reasons(&s), [None, None, Some("no override".into())]);
        loaded(
            &mut s,
            Overrides {
                allowed: vec![id],
                denied: Vec::new(),
            },
        );
        assert_eq!(state_line(&s, id).unwrap().0, "Denylist: allowed by you");
        assert_eq!(reasons(&s), [Some("already allowed".into()), None, None]);
        loaded(
            &mut s,
            Overrides {
                allowed: Vec::new(),
                denied: vec![DenyEntry {
                    app_id: id,
                    name: "x".into(),
                    reason: "my reason".into(),
                }],
            },
        );
        assert_eq!(
            state_line(&s, id).unwrap().0,
            "Denylist: denied by you (my reason)"
        );
        assert_eq!(
            swap_allowed(&s, id),
            Err("DLL swaps denied (my reason)".into())
        );
        assert_eq!(reasons(&s), [None, Some("already denied".into()), None]);
    }

    #[test]
    fn a_built_in_denial_counts_as_denied_and_an_allow_over_it_is_forced() {
        let mut s = state_with_games(120, 40, 1);
        let apex = 1172470;
        loaded(&mut s, Overrides::default());
        assert_eq!(
            state_line(&s, apex).unwrap().0,
            "Denylist: denied (Easy Anti-Cheat)"
        );
        loaded(
            &mut s,
            Overrides {
                allowed: vec![apex],
                denied: Vec::new(),
            },
        );
        assert_eq!(state_line(&s, apex).unwrap().0, "Denylist: force-allowed");
        assert_eq!(swap_allowed(&s, apex), Ok(()));
    }

    #[test]
    fn an_unreadable_list_fails_closed() {
        let mut s = state_with_games(120, 40, 2);
        let request = match start(&mut s)[0] {
            AnyCmd::Denylist(Cmd::Load { request }) => request,
            _ => unreachable!(),
        };
        on_msg(
            &mut s,
            Msg::Loaded {
                request: request + 1,
                result: Ok(DenyLists::new(UserList::default(), Overrides::default())),
            },
        );
        assert!(s.denylist.lists.is_none(), "a stale load is dropped");
        on_msg(
            &mut s,
            Msg::Loaded {
                request,
                result: Err(Unreadable {
                    file: "overrides.toml".into(),
                    error: "bad".into(),
                }),
            },
        );
        let id = library::selected_game(&s).unwrap().app_id;
        assert_eq!(
            state_line(&s, id).unwrap().0,
            "Denylist: unreadable (overrides.toml)"
        );
        assert_eq!(
            swap_allowed(&s, id),
            Err("deny list unreadable (overrides.toml)".into())
        );
    }

    #[test]
    fn allow_confirms_on_cancel_first_and_saves_on_confirm() {
        let mut s = state_with_games(120, 40, 2);
        loaded(&mut s, Overrides::default());
        let game = library::selected_game(&s).unwrap().clone();
        let mut cfg = s.config.clone();
        cfg.confirm_destructive = false;
        s.set_config(cfg);
        update(
            &mut s,
            Action::Allow {
                app_id: game.app_id,
                name: game.name.clone(),
            },
        );
        let Some(Popup::Confirm(c)) = &s.popup else {
            panic!("confirm_destructive = false never skips the allow")
        };
        assert_eq!(c.focus, confirm::Focus::Cancel);
        assert!(c.body.iter().flatten().any(|(t, _)| t == ALLOW_WARNING));
        assert!(step(&mut s, key("enter")).is_empty(), "Enter on Cancel");
        assert_eq!(
            s.status.as_ref().unwrap().text,
            "DLL swaps not allowed, nothing changed"
        );
        update(
            &mut s,
            Action::Allow {
                app_id: game.app_id,
                name: game.name.clone(),
            },
        );
        step(&mut s, key("right"));
        let cmds = step(&mut s, key("enter"));
        let [AnyCmd::Save { token, write }] = &cmds[..] else {
            panic!("{cmds:?}")
        };
        assert_eq!(
            write,
            &Write::Deny {
                app_id: game.app_id,
                name: game.name.clone(),
                change: DenyChange::Allow
            }
        );
        let reload = crate::screens::saved(&mut s, Owner::Denylist, *token, Ok(()));
        assert!(matches!(reload[..], [AnyCmd::Denylist(Cmd::Load { .. })]));
        assert_eq!(
            s.status.as_ref().unwrap().text,
            format!("Saved: DLL swaps allowed for {}", game.name)
        );
    }

    #[test]
    fn deny_opens_the_reason_editor_in_library_detail() {
        let mut s = state_with_games(120, 40, 2);
        loaded(&mut s, Overrides::default());
        let game = library::selected_game(&s).unwrap().clone();
        s.dest = Dest::Settings;
        update(
            &mut s,
            Action::Deny {
                app_id: game.app_id,
                name: game.name.clone(),
            },
        );
        assert_eq!((s.dest, s.focus), (Dest::Library, Pane::Detail));
        assert!(crate::screens::editing(&s, Pane::Detail));
        assert_eq!(
            s.denylist.editor.as_ref().unwrap().editor.input.text(),
            DEFAULT_REASON
        );
        step(&mut s, key("ctrl-u"));
        for c in "EAC".chars() {
            step(
                &mut s,
                crate::event::Event::Key(crate::keymap::Key::char(c)),
            );
        }
        let cmds = step(&mut s, key("enter"));
        assert!(
            matches!(&cmds[..], [AnyCmd::Save { write: Write::Deny { change: DenyChange::Deny { reason }, .. }, .. }] if reason == "EAC"),
            "{cmds:?}"
        );
        assert!(s.denylist.editor.is_none());
    }

    #[test]
    fn the_reason_editor_keeps_its_keys_after_a_click_on_the_list() {
        let mut s = state_with_games(120, 40, 2);
        loaded(&mut s, Overrides::default());
        let game = library::selected_game(&s).unwrap().clone();
        update(
            &mut s,
            Action::Deny {
                app_id: game.app_id,
                name: game.name,
            },
        );
        s.focus = Pane::List;
        step(&mut s, key("q"));
        step(&mut s, key("2"));
        assert!(!s.quit);
        assert_eq!(s.dest, Dest::Library);
        assert_eq!(
            s.denylist.editor.as_ref().unwrap().editor.input.text(),
            format!("{DEFAULT_REASON}q2")
        );
    }
}
