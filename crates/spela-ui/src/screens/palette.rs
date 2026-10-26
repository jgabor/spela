//! The command palette (`:` or Ctrl-K): one fuzzy list over every
//! destination, Actions row, game and setting, from every destination.
//!
//! The rows are the destinations, every screen's Actions rows (the deny
//! list rows for the game selected in Library among them), every game and
//! every setting. Typing filters with [`crate::fuzzy`], matched letters are
//! highlighted, Up and Down move, Enter runs, Esc closes. A row shows its
//! key when it
//! has one, and an unavailable row shows its reason, as in Actions
//! (terminal-ux.md, option B). The rows come from the screens' `palette`
//! through [`crate::screens::palette`].

use ratatui::Frame;
use ratatui::layout::Rect;

use super::{Action as AnyAction, Popup, PopupAction, Row};
use crate::event::Cmd as AnyCmd;
use crate::fuzzy;
use crate::hit::{HitMap, HitTarget};
use crate::input::{InputResult, LineInput};
use crate::keymap::{Binding, Key, KeyName};
use crate::layout::Layout;
use crate::paint::Painter;
use crate::state::{AppState, Level};
use crate::theme::Role;

/// The popup's widest width.
pub const WIDTH: u16 = 84;
/// Rows shown at most.
pub const MAX_ROWS: usize = 10;

/// The palette.
#[derive(Clone, Debug, PartialEq)]
pub struct State {
    /// The filter.
    pub input: LineInput,
    /// Every row.
    pub rows: Vec<Row>,
    /// The rows that match, best first, with the matched char positions.
    pub matches: Vec<(usize, Vec<usize>)>,
    /// The cursor among the matches.
    pub cursor: usize,
}

impl State {
    /// A palette over `rows`, unfiltered.
    pub fn new(rows: Vec<Row>) -> State {
        let mut s = State {
            input: LineInput::default(),
            rows,
            matches: Vec::new(),
            cursor: 0,
        };
        s.refilter();
        s
    }

    /// Filters the rows by the input: every word must match, best score
    /// first, then the rows' own order.
    pub fn refilter(&mut self) {
        let q = self.input.text();
        let mut scored: Vec<(i32, usize, Vec<usize>)> = self
            .rows
            .iter()
            .enumerate()
            .filter_map(|(i, r)| fuzzy::score(q, &r.label).map(|m| (m.score, i, m.positions)))
            .collect();
        scored.sort_by(|a, b| b.0.cmp(&a.0).then(a.1.cmp(&b.1)));
        self.matches = scored.into_iter().map(|(_, i, p)| (i, p)).collect();
        self.cursor = 0;
    }
}

/// The status line text while the palette is open.
pub fn status_text(popup: &State) -> String {
    let shown = popup.matches.len().min(MAX_ROWS);
    if popup.input.is_empty() {
        format!("{} commands", popup.rows.len())
    } else {
        let noun = if shown == 1 { "match" } else { "matches" };
        format!("{shown} best {noun} for \"{}\"", popup.input.text())
    }
}

/// What the palette's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// A typed character for the filter.
    Type(char),
    /// An editing key for the filter.
    Edit(Key),
    /// The previous match.
    Up,
    /// The next match.
    Down,
    /// Runs the cursor row.
    Run,
    /// Runs this match, from a click.
    RunMatch(usize),
    /// Closes the palette.
    Close,
}

fn act(a: Action) -> AnyAction {
    AnyAction::Popup(PopupAction::Palette(a))
}

fn typed(c: char) -> AnyAction {
    act(Action::Type(c))
}

/// The palette's keys. Every typed key filters, letters and digits too.
pub fn bindings(_state: &AppState, _popup: &State) -> Vec<Binding> {
    let mut out = vec![
        Binding::new(Key::UP, act(Action::Up), "Move").hint_as("Up/Down", 100),
        Binding::new(Key::DOWN, act(Action::Down), "Next match"),
        Binding::new(Key::ENTER, act(Action::Run), "Run").hint(90),
        Binding::new(Key::ESC, act(Action::Close), "Close").hint(80),
    ];
    for name in [
        KeyName::Backspace,
        KeyName::Delete,
        KeyName::Left,
        KeyName::Right,
        KeyName::Home,
        KeyName::End,
    ] {
        let key = Key::named(name);
        out.push(Binding::new(key, act(Action::Edit(key)), "Edit the filter"));
    }
    out.push(Binding::typed(typed, act(Action::Close), "Filter"));
    out
}

/// Runs a palette action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let Some(Popup::Palette(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    let n = popup.matches.len().min(MAX_ROWS);
    match action {
        Action::Type(c) => {
            popup.input.handle(&Key::char(c));
            popup.refilter();
        }
        Action::Edit(key) => {
            if popup.input.handle(&key) == InputResult::Changed {
                popup.refilter();
            }
        }
        Action::Up if n > 0 => popup.cursor = (popup.cursor + n - 1) % n,
        Action::Down if n > 0 => popup.cursor = (popup.cursor + 1) % n,
        Action::Up | Action::Down => {}
        Action::Close => state.close_popup(),
        Action::Run => {
            let i = popup.cursor;
            return run_match(state, i);
        }
        Action::RunMatch(i) => return run_match(state, i),
    }
    Vec::new()
}

fn run_match(state: &mut AppState, i: usize) -> Vec<AnyCmd> {
    let Some(Popup::Palette(popup)) = &state.popup else {
        return Vec::new();
    };
    let Some(row) = popup
        .matches
        .get(i)
        .and_then(|(r, _)| popup.rows.get(*r))
        .cloned()
    else {
        return Vec::new();
    };
    state.close_popup();
    if let Some(reason) = row.unavailable {
        state.say(Level::Error, format!("{}: {reason}", row.label));
        return Vec::new();
    }
    crate::update::run(state, row.action)
}

/// Draws the palette.
pub fn render(
    state: &AppState,
    popup: &State,
    frame: &mut Frame,
    layout: &Layout,
    hits: &mut HitMap,
) {
    let shown = popup.matches.len().min(MAX_ROWS);
    let rows = u16::try_from(shown.max(1)).unwrap_or(1);
    let ws = layout.workspace();
    let w = WIDTH.min(ws.width);
    let h = (rows + 8).min(ws.height);
    let area = Rect::new(ws.x + (ws.width - w) / 2, ws.y + 2.min(ws.height - h), w, h);
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = p.popup(area, "Commands", hits);
    if inner.height < 6 || inner.width < 30 {
        return;
    }
    let hint = " : or Ctrl-K ";
    p.spans_right(
        area.right().saturating_sub(2),
        area.y,
        &[(hint, Role::Muted)],
    );
    let x = inner.x + 1;
    let w = inner.width - 2;
    let at = p.text(x, inner.y, w, "> ", Role::Key);
    let (text, cursor) = popup.input.view(usize::from(w.saturating_sub(3)));
    p.text(at, inner.y, w - 2, &text, Role::Strong);
    let cx = at + u16::try_from(cursor).unwrap_or(0);
    p.cursor_row(Rect::new(cx, inner.y, 1, 1));
    p.rule(area, inner.y + 1, Role::Focus);
    let kind_x = inner.right().saturating_sub(21);
    let key_x = inner.right().saturating_sub(31);
    let label_w = key_x.saturating_sub(inner.x + 4);
    if popup.matches.is_empty() {
        p.text(inner.x + 3, inner.y + 2, label_w, "No matches", Role::Muted);
    }
    for (n, (row_i, positions)) in popup.matches.iter().take(shown).enumerate() {
        let row = &popup.rows[*row_i];
        let y = inner.y + 2 + u16::try_from(n).unwrap_or(0);
        let line = Rect::new(inner.x, y, inner.width, 1);
        let cursor = n == popup.cursor;
        if cursor {
            p.text(inner.x + 1, y, 1, state.glyphs.cursor, Role::Key);
        }
        let base = if row.unavailable.is_some() {
            Role::Muted
        } else if cursor {
            Role::Strong
        } else {
            Role::Text
        };
        let mut lx = inner.x + 3;
        let end = inner.x + 3 + label_w;
        for (ci, ch) in row.label.chars().enumerate() {
            if lx >= end {
                break;
            }
            let role = if positions.contains(&ci) {
                Role::Match
            } else {
                base
            };
            let mut buf = [0u8; 4];
            lx = p.text(lx, y, end - lx, ch.encode_utf8(&mut buf), role);
        }
        if let Some(reason) = &row.unavailable {
            let text = format!(" {} {reason}", state.glyphs.dot);
            p.text(lx, y, end.saturating_sub(lx), &text, Role::Muted);
        }
        if let Some(key) = row.key {
            p.text(key_x, y, 8, &key.to_string(), Role::Key);
        }
        p.text(kind_x, y, 18, row.kind.word(), Role::Muted);
        if cursor {
            p.cursor_row(line);
        }
        hits.push(line, HitTarget::Action(act(Action::RunMatch(n))));
    }
    let sep = inner.y + 2 + rows;
    if sep < inner.bottom() {
        p.rule(area, sep, Role::Focus);
        p.spans(
            x,
            sep + 1,
            w,
            &[
                ("Type to filter. ", Role::Muted),
                ("Enter", Role::Key),
                (" runs, ", Role::Muted),
                ("Esc", Role::Key),
                (" closes.", Role::Muted),
            ],
        );
        p.spans(
            x,
            sep + 2,
            w,
            &[
                (
                    "A row shows its key when it has one. Actions rows are reached with ",
                    Role::Muted,
                ),
                ("0", Role::Key),
                (".", Role::Muted),
            ],
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::screens::Dest;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;

    #[test]
    fn typing_filters_and_enter_runs_the_best_match() {
        let mut s = state_with_games(120, 40, 3);
        step(&mut s, key(":"));
        assert!(matches!(s.popup, Some(Popup::Palette(_))));
        for c in "settings".chars() {
            step(&mut s, crate::event::Event::Key(Key::char(c)));
        }
        let Some(Popup::Palette(p)) = &s.popup else {
            unreachable!()
        };
        assert_eq!(p.rows[p.matches[0].0].label, "Go to Settings");
        step(&mut s, key("enter"));
        assert!(s.popup.is_none());
        assert_eq!(s.dest, Dest::Settings);
    }

    #[test]
    fn the_status_counts_matches_in_words() {
        let mut s = state_with_games(120, 40, 3);
        step(&mut s, key(":"));
        for c in "refresh interval".chars() {
            step(&mut s, crate::event::Event::Key(Key::char(c)));
        }
        let Some(Popup::Palette(p)) = &s.popup else {
            unreachable!()
        };
        assert_eq!(status_text(p), "1 best match for \"refresh interval\"");
    }

    #[test]
    fn letters_type_and_never_quit() {
        let mut s = state_with_games(120, 40, 3);
        step(&mut s, key("ctrl-k"));
        for k in ["q", "Q", "1", "?"] {
            step(&mut s, key(k));
        }
        assert!(!s.quit);
        let Some(Popup::Palette(p)) = &s.popup else {
            unreachable!()
        };
        assert_eq!(p.input.text(), "qQ1?");
    }
}
