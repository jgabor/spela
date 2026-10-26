//! 1 Library, List pane: the Find row, the pinned All games row and the
//! games, with ▪ for managed DLLs and ⚑ for an own profile.
//!
//! The first frame comes from the game cache, and one background scan
//! refreshes it when `rescan_on_startup` is on or the cache is empty
//! (decision 24). The selection keeps its game across a rescan, and a
//! rescan drops selected games that are gone.
//!
//! The Find row is a text input with Go's four controls, Input, Apply,
//! Clear search and Cancel, which Tab cycles. While it has the cursor
//! every typed key goes into the query, digits included (decision 8), and
//! the list filters as you type, as Go's Search does (case-insensitive
//! substring, at most 30 characters). Enter runs the focused control:
//! Apply keeps the query, Clear search empties it, Cancel and Esc put back
//! the query Find opened with. Down moves into the results.
//!
//! The DLL and profile filters, the four sorts and multi-select come from
//! Actions, as in Go (TUI 11). Space selects a game. Select mode hides the
//! All games and Find rows and freezes Detail on the scope it showed.
//! Enter with visible selected games opens the batch actions, whose
//! `Update selected games (N visible)` updates the stale DLLs of the
//! listed selected games to the manifest's latest, downloading as needed,
//! after the confirmation (TUI 12; the job is [`crate::dll_jobs`]'s).
//! Hidden selections never join a batch.
//!
//! This module is also the model of the screen contract described in
//! [`crate::screens`]: every public function here with a `state` and a
//! `pane` has the signature the other screens use.

use std::collections::BTreeSet;

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::games::{Game, GameCache};
use spela_core::profile::Scope;

use super::{
    Action as AnyAction, Popup, Row, RowKind, ScreenAction, actions as actions_popup, profile,
};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::input::LineInput;
use crate::keymap::{Binding, Key, KeyName};
use crate::layout::{Pane, PaneArea};
use crate::paint::{Painter, fit};
use crate::state::{AppState, Level};
use crate::theme::Role;

/// The longest query, as in Go.
pub const MAX_QUERY: usize = 30;

/// How the games are ordered, chosen from Actions.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Sort {
    /// By name, A to Z. The default.
    #[default]
    AToZ,
    /// By name, Z to A.
    ZToA,
    /// Games with DLLs first, then A to Z.
    DllsFirst,
    /// Games with their own profile first, then A to Z.
    ProfilesFirst,
}

impl Sort {
    /// Every sort, in Actions order.
    pub const ALL: [Sort; 4] = [Sort::AToZ, Sort::ZToA, Sort::DllsFirst, Sort::ProfilesFirst];

    /// The Actions row, as in Go.
    pub fn label(self) -> &'static str {
        match self {
            Sort::AToZ => "Sort: A-Z",
            Sort::ZToA => "Sort: Z-A",
            Sort::DllsFirst => "Sort: DLLs first",
            Sort::ProfilesFirst => "Sort: profiles first",
        }
    }

    /// The tag in the list title, `Scope [Z-A]`. None for the default.
    pub fn tag(self) -> Option<&'static str> {
        match self {
            Sort::AToZ => None,
            Sort::ZToA => Some("Z-A"),
            Sort::DllsFirst => Some("DLLs"),
            Sort::ProfilesFirst => Some("Profile"),
        }
    }
}

/// The Find row's controls, which Tab cycles.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum FindFocus {
    /// The query.
    #[default]
    Input,
    /// Apply: keep the query and show the results.
    Apply,
    /// Clear search: empty the query.
    Clear,
    /// Cancel: put back the query Find opened with.
    Cancel,
}

impl FindFocus {
    fn next(self) -> FindFocus {
        match self {
            FindFocus::Input => FindFocus::Apply,
            FindFocus::Apply => FindFocus::Clear,
            FindFocus::Clear => FindFocus::Cancel,
            FindFocus::Cancel => FindFocus::Input,
        }
    }

    fn prev(self) -> FindFocus {
        match self {
            FindFocus::Input => FindFocus::Cancel,
            FindFocus::Apply => FindFocus::Input,
            FindFocus::Clear => FindFocus::Apply,
            FindFocus::Cancel => FindFocus::Clear,
        }
    }

    /// What Enter does on it, for the hint bar.
    fn enter_label(self) -> &'static str {
        match self {
            FindFocus::Input | FindFocus::Apply => "Apply search",
            FindFocus::Clear => "Clear search",
            FindFocus::Cancel => "Cancel search",
        }
    }
}

/// The Library list's state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The cursor is on the Find row, which then takes typed keys.
    pub finding: bool,
    /// The query. The list shows only games whose name contains it.
    pub query: LineInput,
    /// The focused Find control.
    pub find_focus: FindFocus,
    /// The query and cursor Find opened with, which Cancel puts back.
    pub find_before: Option<(String, usize)>,
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
    /// The order of the games.
    pub sort: Sort,
    /// Only games with DLLs.
    pub dll_filter: bool,
    /// Only games with their own profile.
    pub profile_filter: bool,
    /// Selected games, by app id. Select mode is on while any is.
    pub selected: BTreeSet<u64>,
    /// The scope Detail keeps showing in select mode.
    pub frozen: Option<Scope>,
}

impl State {
    /// Whether select mode is on.
    pub fn selecting(&self) -> bool {
        !self.selected.is_empty()
    }

    /// Whether a filter is on.
    pub fn filtered(&self) -> bool {
        self.dll_filter || self.profile_filter
    }
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
    /// Opens the selection in Detail, or the batch actions in select mode.
    Open,
    /// Puts the cursor on the Find row.
    Find,
    /// A typed character for the query.
    Type(char),
    /// An editing key for the query: Backspace, Delete, Left, Right, Home,
    /// End, Ctrl-U and Ctrl-W.
    Edit(Key),
    /// Tab on the Find row: the next control.
    FindNext,
    /// Shift-Tab on the Find row: the previous control.
    FindPrev,
    /// Enter on the Find row: runs the focused control.
    FindEnter,
    /// Leaves the Find row for the results, keeping the query.
    ShowResults,
    /// Clears the query and leaves the Find row.
    ClearFind,
    /// Puts back the query Find opened with and leaves the Find row.
    CancelFind,
    /// Starts a scan.
    Rescan,
    /// Shows a game: clears the query and puts the cursor on it.
    Jump(u64),
    /// Only games with DLLs, or all again.
    ToggleDllFilter,
    /// Only games with their own profile, or all again.
    ToggleProfileFilter,
    /// Clears the filters and the search, and sorts A to Z.
    ClearFilters,
    /// Orders the games.
    SetSort(Sort),
    /// Space: selects or clears the game under the cursor.
    ToggleSelect,
    /// Selects every listed game.
    SelectAll,
    /// Clears the listed selected games.
    ClearSelected,
    /// Updates the listed selected games' stale DLLs, after asking.
    UpdateSelected,
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

fn row_scope(state: &AppState, row: ListRow) -> Option<Scope> {
    match row {
        ListRow::AllGames => Some(Scope::AllGames),
        ListRow::Game(i) => state.games.games.get(i).map(|g| Scope::Game(g.app_id)),
    }
}

/// The profile scope Detail shows: All games or the selected game. In
/// select mode, the scope Detail showed when select mode began.
pub fn selected_scope(state: &AppState) -> Option<Scope> {
    if let Some(scope) = state.library.frozen {
        return Some(scope);
    }
    row_scope(state, current_row(state)?)
}

/// The game Detail shows, when a game is selected.
pub fn selected_game(state: &AppState) -> Option<&Game> {
    match selected_scope(state)? {
        Scope::Game(app_id) => state.games.get(app_id),
        Scope::AllGames => None,
    }
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

fn sort_key(g: &Game) -> (String, u64) {
    (g.name.to_lowercase(), g.app_id)
}

/// The games the list shows, in its order: tools left out, then the
/// query, the filters and the sort.
fn listed(state: &AppState, query: &str) -> Vec<usize> {
    let lib = &state.library;
    let cache = &state.games;
    let q = query.to_lowercase();
    let mut out: Vec<usize> = cache
        .games
        .iter()
        .enumerate()
        .filter(|(_, g)| !spela_core::steam::tools::is_tool_name(&g.name))
        .filter(|(_, g)| q.is_empty() || g.name.to_lowercase().contains(&q))
        .filter(|(_, g)| !lib.dll_filter || !g.dlls.is_empty())
        .filter(|(_, g)| !lib.profile_filter || lib.profiles.contains(&g.app_id))
        .map(|(i, _)| i)
        .collect();
    let games = &cache.games;
    match lib.sort {
        Sort::AToZ => out.sort_by_key(|i| sort_key(&games[*i])),
        Sort::ZToA => out.sort_by_key(|i| std::cmp::Reverse(sort_key(&games[*i]))),
        Sort::DllsFirst => out.sort_by_key(|i| (games[*i].dlls.is_empty(), sort_key(&games[*i]))),
        Sort::ProfilesFirst => out.sort_by_key(|i| {
            (
                !lib.profiles.contains(&games[*i].app_id),
                sort_key(&games[*i]),
            )
        }),
    }
    out
}

/// Every game the cache lists, A to Z, for the palette.
fn all_sorted(cache: &GameCache) -> Vec<usize> {
    let mut out: Vec<usize> = cache
        .games
        .iter()
        .enumerate()
        .filter(|(_, g)| !spela_core::steam::tools::is_tool_name(&g.name))
        .map(|(i, _)| i)
        .collect();
    out.sort_by_key(|i| sort_key(&cache.games[*i]));
    out
}

/// Rebuilds the rows after the games, the query, a filter, the sort or
/// the selection changed, keeping the cursor on the same game when it is
/// still listed.
pub fn games_changed(state: &mut AppState) {
    let keep = kept_scope(state);
    rebuild_rows(state, keep);
}

/// The scope under the cursor, to keep across a change. A list that had no
/// games yet starts on the first game, as Go does, not on All games.
fn kept_scope(state: &AppState) -> Option<Scope> {
    let had_games = state
        .library
        .rows
        .iter()
        .any(|r| matches!(r, ListRow::Game(_)));
    current_row(state)
        .and_then(|r| row_scope(state, r))
        .filter(|_| had_games)
}

/// Takes a new game cache, from a scan or a DLL job: drops selected games
/// that are gone and keeps the cursor on its game. The rows index the old
/// cache, so the scope to keep is read before the swap.
pub fn replace_games(state: &mut AppState, cache: GameCache) {
    let keep = kept_scope(state);
    state.games = cache;
    let games = &state.games;
    state.library.selected.retain(|id| games.get(*id).is_some());
    if !state.library.selecting() {
        state.library.frozen = None;
    }
    if let Some(Scope::Game(id)) = state.library.frozen
        && state.games.get(id).is_none()
    {
        state.library.frozen = None;
    }
    rebuild_rows(state, keep);
}

fn rebuild_rows(state: &mut AppState, keep: Option<Scope>) {
    let query = state.library.query.text().to_owned();
    let lib = &state.library;
    let pinned = query.is_empty() && !lib.filtered() && !lib.selecting();
    let mut rows = Vec::new();
    if pinned {
        rows.push(ListRow::AllGames);
    }
    rows.extend(listed(state, &query).into_iter().map(ListRow::Game));
    let first_game = usize::from(pinned);
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

/// Starts the background scan, unless one or a DLL job runs.
pub fn start_scan(state: &mut AppState) -> Vec<AnyCmd> {
    if state.jobs.scan.is_some() || state.jobs.dll.is_some() {
        return Vec::new();
    }
    let request = state.request();
    state.jobs.scan = Some(request);
    state.dirty = true;
    vec![AnyCmd::Scan { request }]
}

/// A scan finished. A stale result is dropped. A fresh one replaces the
/// games, drops selected games that are gone, and reloads the profiles.
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
    let mut cmds = Vec::new();
    match result {
        Ok(cache) => {
            let n = cache.listed().count();
            state.library.scan_error = None;
            set_profiles(state, profiles);
            replace_games(state, cache);
            state.say(Level::Info, format!("Rescan complete: {n} games found"));
            cmds.extend(profile::rescanned(state));
        }
        Err(e) => {
            state.library.scan_error = Some(e.clone());
            state.say(Level::Error, format!("Rescan failed: {e}"));
        }
    }
    cmds.extend(profile::follow_selection(state));
    // The startup auto-update waits for the startup scan, as in Go.
    cmds.extend(crate::dll_jobs::scan_finished(state));
    cmds
}

/// Whether `/` can find: there are games to find in, and select mode is
/// off.
pub fn can_find(state: &AppState) -> Result<(), String> {
    if state.games.games.is_empty() {
        Err("no games".into())
    } else if state.library.selecting() {
        Err("clear the selection first".into())
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
    // A scan and a DLL job each replace the game cache, so one waits for
    // the other.
    let busy = state.jobs.scan.is_some() || state.jobs.dll.is_some();
    Row::new("Rescan games", act(Action::Rescan)).when(if busy {
        Err("wait for pending work".into())
    } else {
        Ok(())
    })
}

/// How many listed games are selected.
pub fn visible_selected(state: &AppState) -> usize {
    let lib = &state.library;
    lib.rows
        .iter()
        .filter_map(|r| game_at(state, *r))
        .filter(|g| lib.selected.contains(&g.app_id))
        .count()
}

/// The pane title: `Scope`, with the sort and the filters, or the
/// selection count in select mode, as in Go.
pub fn title(state: &AppState, _pane: Pane) -> String {
    let lib = &state.library;
    if lib.selecting() {
        return format!(
            "Selected: {} total, {} visible",
            lib.selected.len(),
            visible_selected(state)
        );
    }
    let mut title = "Scope".to_owned();
    if let Some(tag) = lib.sort.tag() {
        title.push_str(&format!(" [{tag}]"));
    }
    let g = state.glyphs;
    if lib.dll_filter {
        title.push_str(&format!(" {}DLLs", g.dlls));
    }
    if lib.profile_filter {
        title.push_str(&format!(" {}Profile", g.own));
    }
    title
}

/// The legend in the bottom border. In one pane the title reads `▸ List`,
/// so the sort, the filters and the selection count move here, before the
/// legend.
pub fn footer(state: &AppState) -> Option<String> {
    let g = state.glyphs;
    let legend = format!("{} DLLs  {} profile", g.dlls, g.own);
    let title = title(state, Pane::List);
    if !state.layout().two_panes() && title != "Scope" {
        return Some(format!("{title}  {legend}"));
    }
    Some(legend)
}

/// Whether the Find row has the cursor: then it takes typed keys.
pub fn editing(state: &AppState, _pane: Pane) -> bool {
    state.library.finding
}

fn find_bindings(state: &AppState) -> Vec<Binding> {
    let focus = state.library.find_focus;
    let mut out = vec![
        Binding::typed(typed, act(Action::Find), "Find games").hint_as("Type", 100),
        Binding::new(Key::ENTER, act(Action::FindEnter), focus.enter_label()).hint(90),
        Binding::new(Key::DOWN, act(Action::ShowResults), "Results"),
        Binding::new(Key::TAB, act(Action::FindNext), "Next control").hint(50),
        Binding::new(Key::BACK_TAB, act(Action::FindPrev), "Previous control"),
        Binding::new(Key::ESC, act(Action::CancelFind), "Cancel search").hint(80),
    ];
    for key in [
        Key::BACKSPACE,
        Key::named(KeyName::Delete),
        Key::LEFT,
        Key::RIGHT,
        Key::HOME,
        Key::END,
        Key::ctrl('u'),
        Key::ctrl('w'),
    ] {
        out.push(Binding::new(key, act(Action::Edit(key)), "Edit the query"));
    }
    out
}

/// The list's keys.
pub fn bindings(state: &AppState, _pane: Pane) -> Vec<Binding> {
    let lib = &state.library;
    if lib.finding {
        return find_bindings(state);
    }
    let has_rows = if lib.rows.is_empty() {
        Err("no items".to_owned())
    } else {
        Ok(())
    };
    let open_label = if lib.selecting() && visible_selected(state) > 0 {
        "Batch actions"
    } else {
        "Open selection"
    };
    let on_game = match current_row(state) {
        Some(ListRow::Game(_)) => Ok(()),
        _ => Err("select a game row".to_owned()),
    };
    let space_label = match current_row(state).and_then(|r| game_at(state, r)) {
        Some(g) if lib.selected.contains(&g.app_id) => "Clear game",
        _ => "Select game",
    };
    let mut out = vec![
        Binding::new(Key::UP, act(Action::Up), "Game")
            .hint_as("Up/Down", 100)
            .when(has_rows.clone()),
        Binding::new(Key::DOWN, act(Action::Down), "Next game").when(has_rows.clone()),
        Binding::new(Key::ENTER, act(Action::Open), open_label)
            .hint(90)
            .when(has_rows.clone()),
        Binding::new(Key::SPACE, act(Action::ToggleSelect), space_label)
            .hint(25)
            .when(on_game),
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

fn list_rows(state: &AppState) -> Vec<Row> {
    let lib = &state.library;
    let visible = visible_selected(state);
    let mut rows = vec![
        Row::new("Search games", act(Action::Find))
            .key(Key::char('/'))
            .when(can_find(state)),
        Row::new("Toggle DLL filter", act(Action::ToggleDllFilter)),
        Row::new("Toggle profile filter", act(Action::ToggleProfileFilter)),
        Row::new(
            "Clear filters (reset sort to A-Z)",
            act(Action::ClearFilters),
        ),
    ];
    for sort in Sort::ALL {
        rows.push(Row::new(sort.label(), act(Action::SetSort(sort))));
    }
    rows.push(Row::new(
        "Select all filtered games",
        act(Action::SelectAll),
    ));
    rows.push(
        Row::new("Clear selected filtered games", act(Action::ClearSelected)).when(
            if visible > 0 {
                Ok(())
            } else {
                Err("no selected filtered games".into())
            },
        ),
    );
    rows.push(update_selected_row(state));
    let _ = lib;
    rows.push(rescan_row(state));
    rows
}

/// The listed selected games.
fn visible_selected_games(state: &AppState) -> Vec<&Game> {
    let lib = &state.library;
    lib.rows
        .iter()
        .filter_map(|r| game_at(state, *r))
        .filter(|g| lib.selected.contains(&g.app_id))
        .collect()
}

fn update_selected_row(state: &AppState) -> Row {
    let targets = crate::dll_jobs::stale_targets(state, visible_selected_games(state));
    let available = if crate::dll_jobs::can_start(state).is_err() {
        Err("wait for pending work".to_owned())
    } else if targets.is_empty() {
        Err("no updates for selected filtered games".to_owned())
    } else {
        Ok(())
    };
    Row::new(
        format!("Update selected games ({} DLLs)", targets.len()),
        act(Action::UpdateSelected),
    )
    .when(available)
}

fn update_selected(state: &mut AppState) -> Vec<AnyCmd> {
    let visible = visible_selected(state);
    if update_selected_row(state).unavailable.is_some() {
        return Vec::new();
    }
    let targets = crate::dll_jobs::stale_targets(state, visible_selected_games(state));
    let items = crate::dll_jobs::update_items(state, &targets);
    crate::dll_jobs::ask(
        state,
        super::confirm::Guard::DllUpdate,
        "Confirm batch DLL update",
        crate::dll_jobs::body(&items, "each current DLL is backed up before replacement"),
        "DLL update",
        crate::dll_jobs::DllOp::Update {
            requests: crate::dll_jobs::update_requests(&targets),
        },
        crate::dll_jobs::Origin::Batch { requested: visible },
    )
}

/// The Actions rows for the list. The Detail pane's are the profile
/// page's; [`crate::screens::actions`] adds Rescan games to both.
pub fn actions(state: &AppState, _pane: Pane) -> Vec<Row> {
    list_rows(state)
}

/// The palette's game rows and the list's Actions rows.
pub fn palette(state: &AppState) -> Vec<Row> {
    let mut rows = list_rows(state);
    for i in all_sorted(&state.games) {
        let g = &state.games.games[i];
        rows.push(Row::new(g.name.clone(), act(Action::Jump(g.app_id))).kind(RowKind::Game));
    }
    rows
}

fn page(state: &AppState) -> usize {
    usize::from(rows_height(state)).max(1)
}

/// Rows the game list has below the Find row (and its controls).
fn rows_height(state: &AppState) -> u16 {
    let h = state.layout().list.inner.height;
    let lib = &state.library;
    let top = if lib.selecting() {
        0
    } else if lib.finding {
        2
    } else {
        1
    };
    h.saturating_sub(top)
}

/// Runs a list action. The ones that change what the list shows bring
/// Library up first, so a palette row run from another destination shows
/// what it did.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    if matches!(
        action,
        Action::Find
            | Action::ToggleDllFilter
            | Action::ToggleProfileFilter
            | Action::ClearFilters
            | Action::SetSort(_)
            | Action::SelectAll
            | Action::ClearSelected
    ) {
        state.dest = super::Dest::Library;
        state.focus = Pane::List;
    }
    let mut cmds = step(state, action);
    keep_cursor_visible(state);
    cmds.extend(profile::follow_selection(state));
    cmds
}

fn open_find(state: &mut AppState) {
    if state.games.games.is_empty() || state.library.selecting() {
        return;
    }
    // One input at a time: Find closes the profile page's editor.
    state.profile.editor = None;
    let lib = &mut state.library;
    if !lib.finding {
        lib.find_before = Some((lib.query.text().to_owned(), lib.cursor));
        lib.find_focus = FindFocus::Input;
    }
    lib.finding = true;
    state.focus = Pane::List;
}

/// Leaves the Find row, keeping the query, when another input opens.
pub fn stop_finding(state: &mut AppState) {
    if state.library.finding {
        leave_find(state);
    }
}

fn leave_find(state: &mut AppState) {
    let lib = &mut state.library;
    lib.finding = false;
    lib.find_before = None;
    lib.find_focus = FindFocus::Input;
}

fn step(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    let last = state.library.rows.len().saturating_sub(1);
    let page = page(state);
    let lib = &mut state.library;
    match action {
        Action::Up => {
            if lib.cursor == 0 && !state.games.games.is_empty() && !lib.selecting() {
                open_find(state);
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
            if lib.finding {
                leave_find(state);
            }
            state.library.cursor = i.min(last);
        }
        Action::Open => {
            if state.library.selecting() && visible_selected(state) > 0 {
                open_batch(state);
            } else if !state.library.rows.is_empty() {
                state.focus = Pane::Detail;
            }
        }
        Action::Find => open_find(state),
        Action::Type(c) => {
            lib.find_focus = FindFocus::Input;
            if lib.query.text().chars().count() < MAX_QUERY {
                lib.query.handle(&Key::char(c));
                games_changed(state);
            }
        }
        Action::Edit(key) => {
            lib.find_focus = FindFocus::Input;
            if lib.query.handle(&key) == crate::input::InputResult::Changed {
                games_changed(state);
            }
        }
        Action::FindNext => lib.find_focus = lib.find_focus.next(),
        Action::FindPrev => lib.find_focus = lib.find_focus.prev(),
        Action::FindEnter => {
            return match lib.find_focus {
                FindFocus::Input | FindFocus::Apply => step(state, Action::ShowResults),
                FindFocus::Clear => step(state, Action::ClearFind),
                FindFocus::Cancel => step(state, Action::CancelFind),
            };
        }
        Action::ShowResults => {
            leave_find(state);
            state.library.cursor = 0;
        }
        Action::ClearFind => {
            leave_find(state);
            if !state.library.query.is_empty() {
                state.library.query.clear();
                games_changed(state);
            }
        }
        Action::CancelFind => {
            let before = lib.find_before.take();
            leave_find(state);
            let (text, cursor) = before.unwrap_or_default();
            if state.library.query.text() != text {
                state.library.query.set(text);
                games_changed(state);
                state.library.cursor = cursor.min(state.library.rows.len().saturating_sub(1));
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
            leave_find(state);
            state.library.query.clear();
            state.library.frozen = None;
            state.library.selected.clear();
            state.library.dll_filter = false;
            state.library.profile_filter = false;
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
        Action::ToggleDllFilter => {
            lib.dll_filter = !lib.dll_filter;
            games_changed(state);
        }
        Action::ToggleProfileFilter => {
            lib.profile_filter = !lib.profile_filter;
            games_changed(state);
        }
        Action::ClearFilters => {
            lib.dll_filter = false;
            lib.profile_filter = false;
            lib.sort = Sort::AToZ;
            lib.query.clear();
            leave_find(state);
            games_changed(state);
        }
        Action::SetSort(sort) => {
            lib.sort = sort;
            games_changed(state);
        }
        Action::ToggleSelect => {
            if let Some(app_id) = current_row(state)
                .and_then(|r| game_at(state, r))
                .map(|g| g.app_id)
            {
                let before = selected_scope(state);
                let lib = &mut state.library;
                if !lib.selected.remove(&app_id) {
                    lib.selected.insert(app_id);
                }
                selection_changed(state, before);
            }
        }
        Action::SelectAll => {
            let before = selected_scope(state);
            let ids: Vec<u64> = state
                .library
                .rows
                .iter()
                .filter_map(|r| game_at(state, *r))
                .map(|g| g.app_id)
                .collect();
            state.library.selected.extend(ids);
            selection_changed(state, before);
        }
        Action::ClearSelected => {
            let before = selected_scope(state);
            let ids: Vec<u64> = state
                .library
                .rows
                .iter()
                .filter_map(|r| game_at(state, *r))
                .map(|g| g.app_id)
                .collect();
            for id in ids {
                state.library.selected.remove(&id);
            }
            selection_changed(state, before);
        }
        Action::UpdateSelected => return update_selected(state),
    }
    Vec::new()
}

/// After the selection changed: select mode freezes Detail on the scope
/// it showed before, and hides or shows the pinned rows.
fn selection_changed(state: &mut AppState, before: Option<Scope>) {
    let lib = &mut state.library;
    if lib.selecting() {
        if lib.frozen.is_none() {
            lib.frozen = before;
        }
    } else {
        lib.frozen = None;
    }
    games_changed(state);
}

/// The batch actions for the listed selected games, as an Actions popup
/// titled as Go's batch menu.
fn open_batch(state: &mut AppState) {
    let visible = visible_selected(state);
    let title = format!(
        "Batch action ({} visible {})",
        visible,
        if visible == 1 { "game" } else { "games" }
    );
    let rows = vec![update_selected_row(state)];
    state.open(Popup::Actions(actions_popup::State::new(title, rows)));
}

/// Scrolls so the cursor row shows.
fn keep_cursor_visible(state: &mut AppState) {
    let height = usize::from(rows_height(state)).max(1);
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
    if state.library.finding {
        leave_find(state);
    }
    let mut cmds = Vec::new();
    for _ in 0..3 {
        if !down && state.library.cursor == 0 {
            break;
        }
        cmds.extend(update(state, if down { Action::Down } else { Action::Up }));
    }
    cmds
}

fn find_row(state: &AppState, p: &mut Painter<'_>, inner: Rect, hits: &mut HitMap) -> u16 {
    let lib = &state.library;
    // An open Find row keeps the keys in either pane, so it shows its
    // cursor in either pane too.
    let focused = state.focus == Pane::List || lib.finding;
    let x = inner.x + 1;
    let w = inner.width - 2;
    let y = inner.y;
    if lib.selecting() {
        return y;
    }
    hits.push(
        Rect::new(inner.x, y, inner.width, 1),
        HitTarget::Action(act(Action::Find)),
    );
    if lib.finding {
        let at = p.text(x, y, w, "/ ", Role::Key);
        let room = usize::from(w.saturating_sub(at - x).saturating_sub(1));
        let (shown, cursor_x) = lib.query.view(room);
        p.text(at, y, w - (at - x), &shown, Role::Strong);
        let cx = at + u16::try_from(cursor_x).unwrap_or(0);
        if focused && lib.find_focus == FindFocus::Input && cx < inner.right() {
            p.cursor_row(Rect::new(cx, y, 1, 1));
        }
        // The controls, under the query.
        let cy = y + 1;
        let mut bx = x;
        for (control, label, a) in [
            (FindFocus::Apply, "Apply", Action::ShowResults),
            (FindFocus::Clear, "Clear search", Action::ClearFind),
            (FindFocus::Cancel, "Cancel", Action::CancelFind),
        ] {
            let on = lib.find_focus == control;
            let end = p.text(
                bx,
                cy,
                inner.right().saturating_sub(bx),
                label,
                if on { Role::Key } else { Role::Muted },
            );
            if on && focused {
                p.cursor_row(Rect::new(bx, cy, end - bx, 1));
            }
            hits.push(Rect::new(bx, cy, end - bx, 1), HitTarget::Action(act(a)));
            bx = end + 2;
            if bx >= inner.right() {
                break;
            }
        }
        return y + 2;
    }
    if lib.query.is_empty() {
        p.spans(x, y, w, &[("/", Role::Key), (" Find games", Role::Muted)]);
    } else {
        let q = format!(" {}", lib.query.text());
        p.spans(x, y, w, &[("/", Role::Key), (q.as_str(), Role::Text)]);
    }
    y + 1
}

/// Draws the list.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    let lib = &state.library;
    let inner = area.inner;
    let focused = state.focus == Pane::List;
    let g = state.glyphs;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, g);
    if inner.width < 10 || inner.height < 3 {
        return;
    }
    let x = inner.x + 1;
    let w = inner.width - 2;
    let top = find_row(state, &mut p, inner, hits);
    let rows_area = Rect::new(
        inner.x,
        top,
        inner.width,
        inner.bottom().saturating_sub(top),
    );
    let height = usize::from(rows_area.height);
    if lib.rows.is_empty() {
        empty_state(state, &mut p, rows_area);
        return;
    }
    let selecting = lib.selecting();
    let name_x = if selecting { x + 4 } else { x + 2 };
    let name_w = usize::from(inner.right().saturating_sub(name_x).saturating_sub(5));
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
                if selecting && lib.selected.contains(&game.app_id) {
                    p.text(x + 2, y, 1, g.ok, Role::Ok);
                }
                let role = if is_cursor && focused {
                    Role::Strong
                } else {
                    Role::Text
                };
                p.text(
                    name_x,
                    y,
                    u16::try_from(name_w).unwrap_or(0),
                    &fit(&game.name, name_w, &g),
                    role,
                );
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
        let hint = if lib.finding {
            "Use Clear search or Cancel below"
        } else {
            "Esc clears the search"
        };
        vec![
            (
                format!("No games match \"{}\"", lib.query.text()),
                Role::Text,
            ),
            (hint.to_owned(), Role::Muted),
        ]
    } else if lib.filtered() && !state.games.games.is_empty() {
        vec![
            ("No games match filters".to_owned(), Role::Text),
            ("Clear filters in Actions".to_owned(), Role::Muted),
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
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step_event;

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
        step_event(&mut s, key("up"));
        step_event(&mut s, key("up"));
        assert!(s.library.finding, "Up from the first row reaches Find");
        for k in ["1", "q", "?"] {
            step_event(&mut s, key(k));
        }
        assert_eq!(s.library.query.text(), "1q?");
        assert_eq!(
            s.dest,
            super::super::Dest::Library,
            "1 typed, not a destination"
        );
        assert!(!s.quit, "q typed, not quit");
        step_event(&mut s, key("esc"));
        assert!(!s.library.finding && s.library.query.is_empty());
    }

    #[test]
    fn ctrl_u_and_ctrl_w_edit_the_query() {
        let mut s = state_with_games(120, 40, 12);
        step_event(&mut s, key("/"));
        for c in "ember tide".chars() {
            step_event(&mut s, crate::event::Event::Key(Key::char(c)));
        }
        step_event(&mut s, key("ctrl-w"));
        assert_eq!(s.library.query.text(), "ember ");
        step_event(&mut s, key("ctrl-u"));
        assert!(s.library.query.is_empty());
        assert!(s.library.finding);
    }

    #[test]
    fn typing_filters_and_detail_follows() {
        let mut s = state_with_games(120, 40, 12);
        step_event(&mut s, key("/"));
        for c in "game 1".chars() {
            step_event(&mut s, crate::event::Event::Key(Key::char(c)));
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
        step_event(&mut s, key("enter"));
        assert!(!s.library.finding);
        assert_eq!(s.library.query.text(), "game 1", "Enter keeps the query");
    }

    #[test]
    fn the_find_controls_apply_clear_and_cancel() {
        let mut s = state_with_games(120, 40, 12);
        step_event(&mut s, key("/"));
        for c in "ember".chars() {
            step_event(&mut s, crate::event::Event::Key(Key::char(c)));
        }
        step_event(&mut s, key("enter"));
        assert_eq!(s.library.query.text(), "ember");
        // Open again, type more, then Cancel puts back "ember".
        step_event(&mut s, key("/"));
        step_event(&mut s, crate::event::Event::Key(Key::char('x')));
        assert_eq!(s.library.query.text(), "emberx");
        for _ in 0..3 {
            step_event(&mut s, key("tab"));
        }
        assert_eq!(s.library.find_focus, FindFocus::Cancel);
        step_event(&mut s, key("enter"));
        assert!(!s.library.finding);
        assert_eq!(s.library.query.text(), "ember", "Cancel restores");
        // Clear search empties it.
        step_event(&mut s, key("/"));
        step_event(&mut s, key("tab"));
        step_event(&mut s, key("tab"));
        assert_eq!(s.library.find_focus, FindFocus::Clear);
        step_event(&mut s, key("enter"));
        assert!(s.library.query.is_empty());
        assert_eq!(s.library.rows[0], ListRow::AllGames);
    }

    #[test]
    fn a_stale_scan_is_dropped_and_a_fresh_one_keeps_the_selection() {
        let mut s = state_with_games(120, 40, 5);
        step_event(&mut s, key("down"));
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

    fn names(s: &AppState) -> Vec<String> {
        s.library
            .rows
            .iter()
            .filter_map(|r| game_at(s, *r).map(|g| g.name.clone()))
            .collect()
    }

    #[test]
    fn sorts_and_filters_follow_go() {
        let mut s = state_with_games(120, 40, 6);
        update(&mut s, Action::SetSort(Sort::ZToA));
        assert_eq!(names(&s)[0], "Game 1 Remastered");
        assert_eq!(title(&s, Pane::List), "Scope [Z-A]");
        update(&mut s, Action::SetSort(Sort::DllsFirst));
        // Every other game has DLLs: indexes 0, 2 and 4.
        let n = names(&s);
        assert_eq!(
            &n[..3],
            ["Ashfall Courier", "Cinder Valley Racing", "Ember Tide"]
        );
        update(&mut s, Action::SetSort(Sort::ProfilesFirst));
        // Every third game has a profile: indexes 0 and 3.
        assert_eq!(&names(&s)[..2], ["Ashfall Courier", "Deepline Salvage"]);
        update(&mut s, Action::ToggleDllFilter);
        assert!(!s.library.rows.contains(&ListRow::AllGames));
        assert_eq!(names(&s).len(), 3);
        update(&mut s, Action::ToggleProfileFilter);
        assert_eq!(names(&s), ["Ashfall Courier"]);
        assert_eq!(title(&s, Pane::List), "Scope [Profile] ▪DLLs ⚑Profile");
        update(&mut s, Action::ClearFilters);
        assert_eq!(s.library.sort, Sort::AToZ);
        assert!(!s.library.filtered());
        assert_eq!(s.library.rows[0], ListRow::AllGames);
        assert_eq!(title(&s, Pane::List), "Scope");
    }

    #[test]
    fn select_mode_hides_pinned_rows_and_freezes_detail() {
        let mut s = state_with_games(120, 40, 6);
        let shown = selected_scope(&s);
        step_event(&mut s, key("space"));
        assert!(s.library.selecting());
        assert!(!s.library.rows.contains(&ListRow::AllGames));
        step_event(&mut s, key("down"));
        assert_eq!(selected_scope(&s), shown, "Detail is frozen");
        assert!(can_find(&s).is_err());
        step_event(&mut s, key("space"));
        assert_eq!(title(&s, Pane::List), "Selected: 2 total, 2 visible");
        update(&mut s, Action::ToggleDllFilter);
        assert_eq!(visible_selected(&s), 1, "a hidden selection stays out");
        // Enter opens the batch actions for the visible ones.
        step_event(&mut s, key("enter"));
        let Some(Popup::Actions(p)) = &s.popup else {
            panic!("batch actions")
        };
        assert_eq!(p.title, "Batch action (1 visible game)");
        assert!(p.rows[0].unavailable.is_some());
        s.close_popup();
        update(&mut s, Action::ToggleDllFilter);
        update(&mut s, Action::ClearSelected);
        assert!(!s.library.selecting());
        assert_eq!(s.library.frozen, None);
        assert_eq!(s.library.rows[0], ListRow::AllGames);
    }

    #[test]
    fn a_rescan_drops_selected_games_that_are_gone() {
        let mut s = state_with_games(120, 40, 4);
        update(&mut s, Action::SelectAll);
        assert_eq!(s.library.selected.len(), 4);
        let cmds = update(&mut s, Action::Rescan);
        let AnyCmd::Scan { request } = cmds[0] else {
            panic!("{cmds:?}")
        };
        let fewer = crate::testing::games(2);
        scan_done(&mut s, request, Ok(fewer), Vec::new());
        assert_eq!(s.library.selected.len(), 2);
    }
}
