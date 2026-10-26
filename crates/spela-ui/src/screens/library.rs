//! 1 Library, List pane: the Find row, the pinned All games row and the
//! games, with ▪ for managed DLLs and ⚑ for an own profile.
//!
//! The first frame comes from the game cache, and one background scan
//! refreshes it when `rescan_on_startup` is on or the cache is empty
//! (decision 24). The selection keeps its game across a rescan.
//!
//! The Find row is a text input: while the cursor is on it, every typed
//! key goes into the query, digits included (decision 8), and the list
//! filters as you type, as Go's Search does (case-insensitive substring,
//! at most 30 characters). Enter shows the results, Esc clears the query,
//! Down moves into the results.
//!
//! Not here yet (TUI 11): the DLL and profile filters, the four sorts and
//! multi-select with batch actions. They belong in this file.
//!
//! This module is also the model of the screen contract described in
//! [`crate::screens`]: every public function here with a `state` and a
//! `pane` has the signature the other screens use.

use std::collections::BTreeSet;

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::games::{Game, GameCache};
use spela_core::profile::Scope;

use super::{Action as AnyAction, Row, RowKind, ScreenAction};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::input::LineInput;
use crate::keymap::{Binding, Key};
use crate::layout::{Pane, PaneArea};
use crate::paint::{Painter, fit};
use crate::state::{AppState, Level};
use crate::text::width;
use crate::theme::Role;

/// The longest query, as in Go.
pub const MAX_QUERY: usize = 30;

/// The Library list's state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The cursor is on the Find row, which then takes typed keys.
    pub finding: bool,
    /// The query. The list shows only games whose name contains it.
    pub query: LineInput,
    /// The cursor row among [`State::rows`] when not finding.
    pub cursor: usize,
    /// The first row shown.
    pub scroll: usize,
    /// The rows: All games, when shown, then the games.
    pub rows: Vec<ListRow>,
    /// Games with their own profile file.
    pub profiles: BTreeSet<u64>,
    /// The last scan's error, shown on an empty list.
    pub scan_error: Option<String>,
}

/// One row of the list.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ListRow {
    /// The pinned All games row.
    AllGames,
    /// A game, by index into the game cache.
    Game(usize),
}

/// What the Library list's keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// The previous row, or the Find row from the first.
    Up,
    /// The next row, or the first result from the Find row.
    Down,
    /// A page up.
    PageUp,
    /// A page down.
    PageDown,
    /// The first row.
    Home,
    /// The last row.
    End,
    /// Moves the cursor to this row.
    Select(usize),
    /// Opens the selection in Detail.
    Open,
    /// Puts the cursor on the Find row.
    Find,
    /// A typed character for the query.
    Type(char),
    /// An editing key for the query: Backspace, Delete, Left, Right, Home,
    /// End.
    Edit(Key),
    /// Leaves the Find row for the results, keeping the query.
    ShowResults,
    /// Clears the query and leaves the Find row.
    ClearFind,
    /// Starts a scan.
    Rescan,
    /// Shows a game: clears the query and puts the cursor on it.
    Jump(u64),
}

/// The Library list's worker results. It has none of its own: the scan
/// answers with [`crate::Msg::ScanDone`].
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {}

/// The Library list's requests. The scan is the shell's
/// [`crate::Cmd::Scan`].
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {}

fn act(a: Action) -> AnyAction {
    AnyAction::Screen(ScreenAction::Library(a))
}

fn typed(c: char) -> AnyAction {
    act(Action::Type(c))
}

/// The game a list row shows.
pub fn game_at(state: &AppState, row: ListRow) -> Option<&Game> {
    match row {
        ListRow::Game(i) => state.games.games.get(i),
        ListRow::AllGames => None,
    }
}

/// The row under the cursor. On the Find row, the first result, so
/// Detail follows the query as it is typed.
pub fn current_row(state: &AppState) -> Option<ListRow> {
    let lib = &state.library;
    if lib.finding {
        lib.rows.first().copied()
    } else {
        lib.rows.get(lib.cursor).copied()
    }
}

/// The profile scope Detail shows: All games or the selected game.
pub fn selected_scope(state: &AppState) -> Option<Scope> {
    match current_row(state)? {
        ListRow::AllGames => Some(Scope::AllGames),
        ListRow::Game(i) => state.games.games.get(i).map(|g| Scope::Game(g.app_id)),
    }
}

/// The game Detail shows, when a game is selected.
pub fn selected_game(state: &AppState) -> Option<&Game> {
    current_row(state).and_then(|r| game_at(state, r))
}

/// Whether a game has its own profile, for ⚑.
pub fn has_profile(state: &AppState, app_id: u64) -> bool {
    state.library.profiles.contains(&app_id)
}

/// Replaces the set of games with their own profile.
pub fn set_profiles(state: &mut AppState, profiles: Vec<u64>) {
    state.library.profiles = profiles.into_iter().collect();
    state.dirty = true;
}

fn listed_sorted(cache: &GameCache, query: &str) -> Vec<usize> {
    let q = query.to_lowercase();
    let mut out: Vec<usize> = cache
        .games
        .iter()
        .enumerate()
        .filter(|(_, g)| !spela_core::steam::tools::is_tool_name(&g.name))
        .filter(|(_, g)| q.is_empty() || g.name.to_lowercase().contains(&q))
        .map(|(i, _)| i)
        .collect();
    out.sort_by(|a, b| {
        let (ga, gb) = (&cache.games[*a], &cache.games[*b]);
        ga.name
            .to_lowercase()
            .cmp(&gb.name.to_lowercase())
            .then(ga.app_id.cmp(&gb.app_id))
    });
    out
}

/// Rebuilds the rows after the games or the query changed, keeping the
/// cursor on the same game when it is still listed.
pub fn games_changed(state: &mut AppState) {
    // A list that had no games yet starts on the first game, as Go does,
    // not on All games.
    let had_games = state
        .library
        .rows
        .iter()
        .any(|r| matches!(r, ListRow::Game(_)));
    let keep = selected_scope(state).filter(|_| had_games);
    let query = state.library.query.text().to_owned();
    let mut rows = Vec::new();
    if query.is_empty() {
        rows.push(ListRow::AllGames);
    }
    rows.extend(
        listed_sorted(&state.games, &query)
            .into_iter()
            .map(ListRow::Game),
    );
    let first_game = usize::from(query.is_empty());
    let lib = &mut state.library;
    lib.rows = rows;
    let found = keep.and_then(|scope| {
        lib.rows.iter().position(|r| match (r, scope) {
            (ListRow::AllGames, Scope::AllGames) => true,
            (ListRow::Game(i), Scope::Game(id)) => state.games.games[*i].app_id == id,
            _ => false,
        })
    });
    // Go starts on the first game, not on All games.
    lib.cursor = found
        .unwrap_or(first_game)
        .min(lib.rows.len().saturating_sub(1));
    state.dirty = true;
}

/// Starts the background scan, unless one runs.
pub fn start_scan(state: &mut AppState) -> Vec<AnyCmd> {
    if state.jobs.scan.is_some() {
        return Vec::new();
    }
    let request = state.request();
    state.jobs.scan = Some(request);
    state.dirty = true;
    vec![AnyCmd::Scan { request }]
}

/// A scan finished. A stale result is dropped.
pub fn scan_done(
    state: &mut AppState,
    request: u64,
    result: Result<GameCache, String>,
    profiles: Vec<u64>,
) -> Vec<AnyCmd> {
    if state.jobs.scan != Some(request) {
        return Vec::new();
    }
    state.jobs.scan = None;
    match result {
        Ok(cache) => {
            let n = cache.listed().count();
            state.games = cache;
            state.library.scan_error = None;
            set_profiles(state, profiles);
            games_changed(state);
            state.say(Level::Info, format!("Rescan complete: {n} games found"));
        }
        Err(e) => {
            state.library.scan_error = Some(e.clone());
            state.say(Level::Error, format!("Rescan failed: {e}"));
        }
    }
    Vec::new()
}

/// Whether `/` can find: there are games to find in.
pub fn can_find(state: &AppState) -> Result<(), String> {
    if state.games.games.is_empty() {
        Err("no games".into())
    } else {
        Ok(())
    }
}

/// `/`: the List gets focus with the cursor on the Find row.
pub fn find(state: &mut AppState) -> Vec<AnyCmd> {
    state.focus = Pane::List;
    update(state, Action::Find)
}

/// The Rescan games row, shared by both Library panes.
pub fn rescan_row(state: &AppState) -> Row {
    Row::new("Rescan games", act(Action::Rescan)).when(match state.jobs.scan {
        Some(_) => Err("wait for pending work".into()),
        None => Ok(()),
    })
}

/// The pane title: `Scope`, as in Go.
pub fn title(_state: &AppState, _pane: Pane) -> String {
    "Scope".to_owned()
}

/// The legend in the bottom border.
pub fn footer(state: &AppState) -> Option<String> {
    let g = state.glyphs;
    Some(format!("{} DLLs  {} profile", g.dlls, g.own))
}

/// Whether the Find row has the cursor: then it takes typed keys.
pub fn editing(state: &AppState, _pane: Pane) -> bool {
    state.library.finding
}

/// The list's keys.
pub fn bindings(state: &AppState, _pane: Pane) -> Vec<Binding> {
    let lib = &state.library;
    if lib.finding {
        let mut out = vec![
            Binding::typed(typed, act(Action::Find), "Find games").hint_as("Type", 100),
            Binding::new(Key::ENTER, act(Action::ShowResults), "Show results").hint(90),
            Binding::new(Key::DOWN, act(Action::ShowResults), "Results"),
            Binding::new(Key::ESC, act(Action::ClearFind), "Clear search").hint(80),
            Binding::new(Key::TAB, AnyAction::NextPane, "Pane").hint(30),
            Binding::new(Key::BACK_TAB, AnyAction::PrevPane, "Previous pane"),
        ];
        for key in [
            Key::BACKSPACE,
            Key::named(crate::keymap::KeyName::Delete),
            Key::LEFT,
            Key::RIGHT,
            Key::HOME,
            Key::END,
        ] {
            out.push(Binding::new(key, act(Action::Edit(key)), "Edit the query"));
        }
        return out;
    }
    let has_rows = if lib.rows.is_empty() {
        Err("no items".to_owned())
    } else {
        Ok(())
    };
    let mut out = vec![
        Binding::new(Key::UP, act(Action::Up), "Game").hint_as("Up/Down", 100),
        Binding::new(Key::DOWN, act(Action::Down), "Next game").when(has_rows.clone()),
        Binding::new(Key::ENTER, act(Action::Open), "Open selection")
            .hint(90)
            .when(has_rows.clone()),
        Binding::new(Key::PAGE_UP, act(Action::PageUp), "Page up"),
        Binding::new(Key::PAGE_DOWN, act(Action::PageDown), "Page down"),
        Binding::new(Key::HOME, act(Action::Home), "First game"),
        Binding::new(Key::END, act(Action::End), "Last game"),
    ];
    if !lib.query.is_empty() {
        out.push(Binding::new(Key::ESC, act(Action::ClearFind), "Clear search").hint(15));
    }
    out
}

/// The Actions rows for the list. The Detail pane's are the profile
/// page's; [`crate::screens::actions`] adds Rescan games to both.
pub fn actions(state: &AppState, _pane: Pane) -> Vec<Row> {
    vec![
        Row::new("Search games", act(Action::Find))
            .key(Key::char('/'))
            .when(can_find(state)),
        rescan_row(state),
    ]
}

/// The palette's game rows and the list's Actions rows.
pub fn palette(state: &AppState) -> Vec<Row> {
    let mut rows = vec![
        Row::new("Search games", act(Action::Find))
            .key(Key::char('/'))
            .when(can_find(state)),
        rescan_row(state),
    ];
    for i in listed_sorted(&state.games, "") {
        let g = &state.games.games[i];
        rows.push(Row::new(g.name.clone(), act(Action::Jump(g.app_id))).kind(RowKind::Game));
    }
    rows
}

fn page(state: &AppState) -> usize {
    usize::from(state.layout().list.inner.height.saturating_sub(1)).max(1)
}

/// Runs a list action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    let last = state.library.rows.len().saturating_sub(1);
    let page = page(state);
    let lib = &mut state.library;
    match action {
        Action::Up => {
            if lib.cursor == 0 && !state.games.games.is_empty() {
                lib.finding = true;
            } else {
                lib.cursor = lib.cursor.saturating_sub(1);
            }
        }
        Action::Down => lib.cursor = (lib.cursor + 1).min(last),
        Action::PageUp => lib.cursor = lib.cursor.saturating_sub(page),
        Action::PageDown => lib.cursor = (lib.cursor + page).min(last),
        Action::Home => lib.cursor = 0,
        Action::End => lib.cursor = last,
        Action::Select(i) => {
            lib.finding = false;
            lib.cursor = i.min(last);
        }
        Action::Open => {
            if !lib.rows.is_empty() {
                state.focus = Pane::Detail;
            }
        }
        Action::Find => {
            if !state.games.games.is_empty() {
                state.library.finding = true;
                state.focus = Pane::List;
            }
        }
        Action::Type(c) => {
            if lib.query.text().chars().count() < MAX_QUERY {
                lib.query.handle(&Key::char(c));
                games_changed(state);
            }
        }
        Action::Edit(key) => {
            if lib.query.handle(&key) == crate::input::InputResult::Changed {
                games_changed(state);
            }
        }
        Action::ShowResults => {
            lib.finding = false;
            lib.cursor = 0;
        }
        Action::ClearFind => {
            lib.finding = false;
            if !lib.query.is_empty() {
                lib.query.clear();
                games_changed(state);
            }
        }
        Action::Rescan => {
            let cmds = start_scan(state);
            if !cmds.is_empty() {
                state.say(Level::Info, "Rescanning games...");
            }
            return cmds;
        }
        Action::Jump(app_id) => {
            state.library.finding = false;
            state.library.query.clear();
            games_changed(state);
            let lib = &mut state.library;
            if let Some(i) = lib.rows.iter().position(|r| match r {
                ListRow::Game(i) => state.games.games[*i].app_id == app_id,
                ListRow::AllGames => false,
            }) {
                lib.cursor = i;
            }
            state.dest = super::Dest::Library;
            state.focus = Pane::List;
        }
    }
    keep_cursor_visible(state);
    Vec::new()
}

/// Scrolls so the cursor row shows.
fn keep_cursor_visible(state: &mut AppState) {
    let height = usize::from(state.layout().list.inner.height.saturating_sub(1)).max(1);
    let lib = &mut state.library;
    if lib.cursor < lib.scroll {
        lib.scroll = lib.cursor;
    } else if lib.cursor >= lib.scroll + height {
        lib.scroll = lib.cursor + 1 - height;
    }
    lib.scroll = lib.scroll.min(lib.rows.len().saturating_sub(height));
}

/// The list's worker results: none yet.
pub fn on_msg(_state: &mut AppState, msg: Msg) -> Vec<AnyCmd> {
    match msg {}
}

/// The list saves nothing.
pub fn saved(_state: &mut AppState, _token: u64, _result: Result<(), String>) -> Vec<AnyCmd> {
    Vec::new()
}

/// The wheel moves the cursor three rows.
pub fn wheel(state: &mut AppState, _pane: Pane, down: bool) -> Vec<AnyCmd> {
    let mut cmds = Vec::new();
    for _ in 0..3 {
        cmds.extend(update(state, if down { Action::Down } else { Action::Up }));
    }
    state.library.finding = false;
    cmds
}

/// Draws the list.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    let lib = &state.library;
    let inner = area.inner;
    let focused = state.focus == Pane::List;
    let g = state.glyphs;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, g);
    if inner.width < 10 || inner.height < 2 {
        return;
    }
    let x = inner.x + 1;
    let w = inner.width - 2;
    // The Find row.
    let y = inner.y;
    let find_area = Rect::new(inner.x, y, inner.width, 1);
    hits.push(find_area, HitTarget::Action(act(Action::Find)));
    if lib.finding {
        let at = p.text(x, y, w, "/ ", Role::Key);
        let room = usize::from(w.saturating_sub(at - x).saturating_sub(1));
        let (shown, cursor_x) = lib.query.view(room);
        p.text(at, y, w - (at - x), &shown, Role::Strong);
        let cx = at + u16::try_from(cursor_x).unwrap_or(0);
        if focused && cx < inner.right() {
            p.cursor_row(Rect::new(cx, y, 1, 1));
        }
    } else if lib.query.is_empty() {
        p.spans(x, y, w, &[("/", Role::Key), (" Find games", Role::Muted)]);
    } else {
        let q = format!(" {}", lib.query.text());
        p.spans(x, y, w, &[("/", Role::Key), (q.as_str(), Role::Text)]);
    }
    let rows_area = Rect::new(inner.x, inner.y + 1, inner.width, inner.height - 1);
    let height = usize::from(rows_area.height);
    if lib.rows.is_empty() {
        empty_state(state, &mut p, rows_area);
        return;
    }
    let name_w = usize::from(inner.width.saturating_sub(8));
    for (n, (i, row)) in lib
        .rows
        .iter()
        .enumerate()
        .skip(lib.scroll)
        .take(height)
        .enumerate()
    {
        let y = rows_area.y + u16::try_from(n).unwrap_or(0);
        let line = Rect::new(inner.x, y, inner.width, 1);
        let is_cursor = !lib.finding && i == lib.cursor;
        let cursor_role = if focused { Role::Key } else { Role::Muted };
        if is_cursor {
            p.text(x, y, 1, g.cursor, cursor_role);
        }
        match row {
            ListRow::AllGames => {
                p.text(x + 2, y, w - 2, "All games", Role::Own);
            }
            ListRow::Game(gi) => {
                let game = &state.games.games[*gi];
                let role = if is_cursor && focused {
                    Role::Strong
                } else {
                    Role::Text
                };
                p.text(x + 2, y, w - 2, &fit(&game.name, name_w, &g), role);
                if !game.dlls.is_empty() {
                    p.text(inner.x + inner.width - 4, y, 1, g.dlls, Role::Focus);
                }
                if lib.profiles.contains(&game.app_id) {
                    p.text(inner.x + inner.width - 2, y, 1, g.own, Role::Own);
                }
            }
        }
        if is_cursor {
            p.cursor_row(line);
        }
        hits.push(
            line,
            HitTarget::Row {
                select: act(Action::Select(i)),
                open: act(Action::Open),
            },
        );
    }
    p.thumb(area.border_x, rows_area, lib.scroll, height, lib.rows.len());
}

fn empty_state(state: &AppState, p: &mut Painter<'_>, area: Rect) {
    let lib = &state.library;
    let x = area.x + 1;
    let w = area.width.saturating_sub(2);
    let lines: Vec<(String, Role)> = if !lib.query.is_empty() {
        vec![
            (
                format!("No games match \"{}\"", lib.query.text()),
                Role::Text,
            ),
            ("Esc clears the search".to_owned(), Role::Muted),
        ]
    } else if state.jobs.scan.is_some() {
        vec![("Scanning games...".to_owned(), Role::Muted)]
    } else {
        let mut v = vec![
            ("No games found".to_owned(), Role::Text),
            (
                "Use Actions to rescan or Settings for paths".to_owned(),
                Role::Muted,
            ),
        ];
        if let Some(e) = &lib.scan_error {
            v.push((e.clone(), Role::Error));
        }
        v
    };
    let mut y = area.y + 1;
    for (text, role) in lines {
        for line in crate::text::wrap(&text, usize::from(w)) {
            if y >= area.bottom() {
                return;
            }
            p.text(x, y, w, &line, role);
            y += 1;
        }
    }
    let _ = width;
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;

    #[test]
    fn starts_on_the_first_game_below_all_games() {
        let s = state_with_games(120, 40, 5);
        assert_eq!(s.library.rows[0], ListRow::AllGames);
        assert_eq!(s.library.cursor, 1);
        assert!(matches!(selected_scope(&s), Some(Scope::Game(_))));
    }

    #[test]
    fn the_find_row_takes_digits_and_letters() {
        let mut s = state_with_games(120, 40, 12);
        step(&mut s, key("up"));
        step(&mut s, key("up"));
        assert!(s.library.finding, "Up from the first row reaches Find");
        for k in ["1", "q", "?"] {
            step(&mut s, key(k));
        }
        assert_eq!(s.library.query.text(), "1q?");
        assert_eq!(
            s.dest,
            super::super::Dest::Library,
            "1 typed, not a destination"
        );
        assert!(!s.quit, "q typed, not quit");
        step(&mut s, key("esc"));
        assert!(!s.library.finding && s.library.query.is_empty());
    }

    #[test]
    fn typing_filters_and_detail_follows() {
        let mut s = state_with_games(120, 40, 12);
        step(&mut s, key("/"));
        for c in "game 1".chars() {
            step(&mut s, crate::event::Event::Key(Key::char(c)));
        }
        let names: Vec<&str> = s
            .library
            .rows
            .iter()
            .filter_map(|r| game_at(&s, *r).map(|g| g.name.as_str()))
            .collect();
        assert!(
            names.iter().all(|n| n.to_lowercase().contains("game 1")),
            "{names:?}"
        );
        assert!(!s.library.rows.contains(&ListRow::AllGames));
        assert_eq!(
            selected_game(&s).map(|g| g.name.as_str()),
            names.first().copied()
        );
        step(&mut s, key("enter"));
        assert!(!s.library.finding);
        assert_eq!(s.library.query.text(), "game 1", "Enter keeps the query");
    }

    #[test]
    fn a_stale_scan_is_dropped_and_a_fresh_one_keeps_the_selection() {
        let mut s = state_with_games(120, 40, 5);
        step(&mut s, key("down"));
        let before = selected_scope(&s);
        let cmds = update(&mut s, Action::Rescan);
        let AnyCmd::Scan { request } = cmds[0] else {
            panic!("{cmds:?}")
        };
        let games = s.games.clone();
        scan_done(&mut s, request + 1, Ok(GameCache::default()), Vec::new());
        assert_eq!(s.jobs.scan, Some(request), "stale result ignored");
        scan_done(&mut s, request, Ok(games), vec![]);
        assert_eq!(s.jobs.scan, None);
        assert_eq!(selected_scope(&s), before);
        assert_eq!(
            s.status.as_ref().unwrap().text,
            "Rescan complete: 5 games found"
        );
    }
}
