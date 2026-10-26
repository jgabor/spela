//! 2 DLL Catalog, read only, and the DLL rows the profile page's DLLs
//! section can reuse ([`game_dll_lines`]).
//!
//! List has two groups, Library and Deployment, switched with Left and
//! Right as in Go (TUI 3):
//!
//! - **Library**: the five DLL families in catalogue order. Detail shows
//!   the family's file name, the manifest's latest version and every
//!   version it lists, the versions in the DLL cache, and every deployment
//!   of the family, marked current, stale or unknown.
//! - **Deployment**: the games with detected DLLs. Detail shows each DLL
//!   with its version and mark.
//!
//! Stale is the one function the Library, the Catalog and the CLI share
//! ([`spela_core::dll::stale`], decision 38): an installed version that is
//! known and lower than the highest manifest version of its family. A
//! version that cannot be read, a family the manifest does not list, and
//! no manifest at all read "unknown", never "current".
//!
//! The manifest comes from the cache (`manifest-v1.json`) and the cached
//! versions from the DLL cache folder. Both are read once at start, on the
//! shell's job pool ([`Cmd::Load`]); this crate never reads a file.
//! Nothing here fetches, installs, updates or restores: those come with
//! the DLL operations, so `Update all stale deployments` is listed but
//! unavailable.

use std::collections::BTreeMap;

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::dll::detect::{CATALOGUE, Family, family};
use spela_core::dll::manifest::{CachedManifest, Manifest};
use spela_core::dll::stale::{self, Freshness};
use spela_core::games::{DetectedDll, DllType, Game};

use super::{Action as AnyAction, Row, ScreenAction};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::{Pane, PaneArea};
use crate::paint::{Painter, fit};
use crate::state::AppState;
use crate::theme::{Glyphs, Role};

/// Why `Update all stale deployments` cannot run in this build.
pub const NOT_BUILT: &str = "not built yet";

/// A group of the List.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Group {
    /// The DLL families.
    #[default]
    Library,
    /// The games with DLLs.
    Deployment,
}

impl Group {
    /// The group's name, as in Go.
    pub fn label(self) -> &'static str {
        match self {
            Group::Library => "Library",
            Group::Deployment => "Deployment",
        }
    }

    fn other(self) -> Group {
        match self {
            Group::Library => Group::Deployment,
            Group::Deployment => Group::Library,
        }
    }
}

/// What the load found.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Catalog {
    /// The manifest cache: `None` when there is none yet, or why it could
    /// not be read.
    pub manifest: Result<Option<CachedManifest>, String>,
    /// The versions in the DLL cache by family, newest first, or why the
    /// folder could not be read.
    pub cached: BTreeMap<DllType, Result<Vec<String>, String>>,
}

/// The Catalog's state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The List group.
    pub group: Group,
    /// The cursor among the families.
    pub family: usize,
    /// The cursor among the deployments.
    pub game: usize,
    /// The first List row shown in Deployment.
    pub list_scroll: usize,
    /// The first Detail line shown.
    pub scroll: usize,
    /// What the load found; `None` while it runs.
    pub catalog: Option<Catalog>,
    /// The load in flight.
    pub request: Option<u64>,
}

/// What the Catalog's keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// The previous row.
    Up,
    /// The next row.
    Down,
    /// The other group.
    Group,
    /// Moves the cursor to a row of the current group.
    Select(usize),
    /// Opens the row in Detail.
    Open,
    /// Scrolls Detail up a line.
    ScrollUp,
    /// Scrolls Detail down a line.
    ScrollDown,
    /// `Update all stale deployments`: listed, not built.
    UpdateAll,
}

/// The Catalog's worker results.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {
    /// The load finished.
    Loaded {
        /// The [`Cmd::Load`] this answers.
        request: u64,
        /// What it found, or why the job failed.
        result: Result<Catalog, String>,
    },
}

/// The Catalog's requests, run in the binary's `tui/exec/dlls.rs`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {
    /// Read the manifest cache and the DLL cache folder.
    Load {
        /// The id the result carries.
        request: u64,
    },
}

fn act(a: Action) -> AnyAction {
    AnyAction::Screen(ScreenAction::Dlls(a))
}

/// The first load, from [`AppState::start`].
pub fn start(state: &mut AppState) -> Vec<AnyCmd> {
    let request = state.request();
    state.dlls.request = Some(request);
    vec![AnyCmd::Dlls(Cmd::Load { request })]
}

/// The cached manifest, when the load found one.
pub fn manifest(state: &AppState) -> Option<&Manifest> {
    match &state.dlls.catalog {
        Some(Catalog {
            manifest: Ok(Some(m)),
            ..
        }) => Some(&m.manifest),
        _ => None,
    }
}

/// How a detected DLL compares with the manifest.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Mark {
    /// The latest, or newer.
    Current,
    /// A newer version is in the manifest.
    Stale {
        /// The manifest's latest version.
        latest: String,
    },
    /// The version, the manifest or its entry for the family is missing.
    Unknown,
}

impl Mark {
    /// The word Detail and List show.
    pub fn word(&self) -> &'static str {
        match self {
            Mark::Current => "current",
            Mark::Stale { .. } => "stale",
            Mark::Unknown => "unknown",
        }
    }

    /// The role the word is drawn in.
    pub fn role(&self) -> Role {
        match self {
            Mark::Current => Role::Ok,
            Mark::Stale { .. } => Role::Warn,
            Mark::Unknown => Role::Muted,
        }
    }
}

/// The mark of a detected DLL, from the shared stale function.
pub fn mark(state: &AppState, dll: &DetectedDll) -> Mark {
    let Some(m) = manifest(state) else {
        return Mark::Unknown;
    };
    match stale::of_dll(dll, m) {
        Freshness::Current { .. } => Mark::Current,
        Freshness::Stale { latest } => Mark::Stale {
            latest: latest.version.clone(),
        },
        Freshness::Unknown { .. } | Freshness::NotInManifest => Mark::Unknown,
    }
}

/// A game's mark: stale when any DLL is, then unknown, then current.
pub fn game_mark(state: &AppState, game: &Game) -> Mark {
    let marks: Vec<Mark> = game.dlls.iter().map(|d| mark(state, d)).collect();
    marks
        .iter()
        .find(|m| matches!(m, Mark::Stale { .. }))
        .or_else(|| marks.iter().find(|m| **m == Mark::Unknown))
        .cloned()
        .unwrap_or(Mark::Current)
}

/// The games with detected DLLs, by index into the game cache, sorted by
/// name as the Library sorts them. Steam tools are left out.
pub fn deployments(state: &AppState) -> Vec<usize> {
    let mut out: Vec<usize> = state
        .games
        .games
        .iter()
        .enumerate()
        .filter(|(_, g)| !g.dlls.is_empty() && !spela_core::steam::tools::is_tool_name(&g.name))
        .map(|(i, _)| i)
        .collect();
    out.sort_by(|a, b| {
        let (ga, gb) = (&state.games.games[*a], &state.games.games[*b]);
        ga.name
            .to_lowercase()
            .cmp(&gb.name.to_lowercase())
            .then(ga.app_id.cmp(&gb.app_id))
    });
    out
}

/// Every stale DLL over all deployments.
pub fn stale_count(state: &AppState) -> usize {
    deployments(state)
        .into_iter()
        .flat_map(|i| state.games.games[i].dlls.iter())
        .filter(|d| matches!(mark(state, d), Mark::Stale { .. }))
        .count()
}

fn update_all_row(state: &AppState) -> Row {
    let n = stale_count(state);
    let unit = if n == 1 { "DLL" } else { "DLLs" };
    let reason = if n == 0 {
        "no stale deployments"
    } else {
        NOT_BUILT
    };
    Row::new(
        format!("Update all stale deployments ({n} {unit})"),
        act(Action::UpdateAll),
    )
    .when(Err(reason.to_owned()))
}

/// The selected family, in Library.
pub fn selected_family(state: &AppState) -> &'static Family {
    &CATALOGUE[state.dlls.family.min(CATALOGUE.len() - 1)]
}

/// The selected deployment, in Deployment.
pub fn selected_game(state: &AppState) -> Option<&Game> {
    let rows = deployments(state);
    let i = *rows.get(state.dlls.game.min(rows.len().saturating_sub(1)))?;
    state.games.games.get(i)
}

/// The pane title: `DLL Catalog · <group>` on List; the family or the game
/// on Detail.
pub fn title(state: &AppState, pane: Pane) -> String {
    match pane {
        Pane::List => format!(
            "DLL Catalog {} {}",
            state.glyphs.dot,
            state.dlls.group.label()
        ),
        Pane::Detail => match state.dlls.group {
            Group::Library => selected_family(state).label.to_owned(),
            Group::Deployment => {
                selected_game(state).map_or_else(|| "Deployment".to_owned(), |g| g.name.clone())
            }
        },
    }
}

/// The Catalog has no editor.
pub fn editing(_state: &AppState, _pane: Pane) -> bool {
    false
}

fn row_count(state: &AppState) -> usize {
    match state.dlls.group {
        Group::Library => CATALOGUE.len(),
        Group::Deployment => deployments(state).len(),
    }
}

/// The keys of a pane.
pub fn bindings(state: &AppState, pane: Pane) -> Vec<Binding> {
    match pane {
        Pane::List => {
            let rows = if row_count(state) == 0 {
                Err("no items".to_owned())
            } else {
                Ok(())
            };
            vec![
                Binding::new(Key::UP, act(Action::Up), "Item")
                    .hint_as("Up/Down", 100)
                    .when(rows.clone()),
                Binding::new(Key::DOWN, act(Action::Down), "Next item").when(rows.clone()),
                Binding::new(Key::LEFT, act(Action::Group), "Group").hint_as("Left/Right", 60),
                Binding::new(Key::RIGHT, act(Action::Group), "Next group"),
                Binding::new(Key::ENTER, act(Action::Open), "Open")
                    .hint(90)
                    .when(rows),
            ]
        }
        Pane::Detail => {
            if detail_lines(state).len() > detail_height(state) {
                vec![
                    Binding::new(Key::UP, act(Action::ScrollUp), "Scroll").hint_as("Up/Down", 100),
                    Binding::new(Key::DOWN, act(Action::ScrollDown), "Scroll down"),
                ]
            } else {
                Vec::new()
            }
        }
    }
}

/// The Actions rows: `Update all stale deployments` in Deployment, as in
/// Go; nothing of its own in Library.
pub fn actions(state: &AppState, _pane: Pane) -> Vec<Row> {
    match state.dlls.group {
        Group::Library => Vec::new(),
        Group::Deployment => vec![update_all_row(state)],
    }
}

/// The palette's Catalog row.
pub fn palette(state: &AppState) -> Vec<Row> {
    vec![update_all_row(state)]
}

/// Runs an action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    let n = row_count(state);
    let list_h = list_height(state);
    let d = &mut state.dlls;
    let cursor = match d.group {
        Group::Library => &mut d.family,
        Group::Deployment => &mut d.game,
    };
    match action {
        Action::Up => {
            *cursor = cursor.saturating_sub(1);
            d.scroll = 0;
        }
        Action::Down => {
            *cursor = (*cursor + 1).min(n.saturating_sub(1));
            d.scroll = 0;
        }
        Action::Select(i) => {
            if *cursor != i {
                d.scroll = 0;
            }
            *cursor = i.min(n.saturating_sub(1));
        }
        Action::Group => {
            d.group = d.group.other();
            d.scroll = 0;
        }
        Action::Open => {
            if n > 0 {
                state.focus = Pane::Detail;
            }
        }
        Action::ScrollUp => d.scroll = d.scroll.saturating_sub(1),
        Action::ScrollDown => {
            let max = max_scroll(state);
            state.dlls.scroll = (state.dlls.scroll + 1).min(max);
        }
        // Unavailable everywhere it is listed, so it never runs.
        Action::UpdateAll => {}
    }
    keep_cursor_visible(state, list_h);
    Vec::new()
}

fn keep_cursor_visible(state: &mut AppState, height: usize) {
    let n = deployments(state).len();
    let d = &mut state.dlls;
    d.game = d.game.min(n.saturating_sub(1));
    if d.game < d.list_scroll {
        d.list_scroll = d.game;
    } else if d.game >= d.list_scroll + height {
        d.list_scroll = d.game + 1 - height;
    }
    d.list_scroll = d.list_scroll.min(n.saturating_sub(height));
}

/// Takes a worker result. A stale load is dropped.
pub fn on_msg(state: &mut AppState, msg: Msg) -> Vec<AnyCmd> {
    match msg {
        Msg::Loaded { request, result } => {
            if state.dlls.request != Some(request) {
                return Vec::new();
            }
            state.dlls.request = None;
            state.dlls.catalog = Some(result.unwrap_or_else(|e| {
                Catalog {
                    manifest: Err(e.clone()),
                    cached: CATALOGUE
                        .iter()
                        .map(|f| (f.dll_type, Err(e.clone())))
                        .collect(),
                }
            }));
            state.dirty = true;
        }
    }
    Vec::new()
}

/// The Catalog saves nothing.
pub fn saved(_state: &mut AppState, _token: u64, _result: Result<(), String>) -> Vec<AnyCmd> {
    Vec::new()
}

/// The wheel moves the List cursor, or scrolls Detail.
pub fn wheel(state: &mut AppState, pane: Pane, down: bool) -> Vec<AnyCmd> {
    let action = match (pane, down) {
        (Pane::List, true) => Action::Down,
        (Pane::List, false) => Action::Up,
        (Pane::Detail, true) => Action::ScrollDown,
        (Pane::Detail, false) => Action::ScrollUp,
    };
    update(state, action)
}

/// One Detail line: spans of text in roles.
pub type Line = Vec<(String, Role)>;

fn plain(text: impl Into<String>, role: Role) -> Line {
    vec![(text.into(), role)]
}

/// The version text of a detected DLL: its version, or `unknown`.
pub fn version_text(dll: &DetectedDll) -> &str {
    dll.version
        .as_deref()
        .filter(|v| !v.is_empty())
        .unwrap_or("unknown")
}

/// One line per detected DLL of `game`: the family, the version, the
/// mark, and the file under the install folder. The profile page's DLLs
/// section can show these too, so both say the same.
pub fn game_dll_lines(state: &AppState, game: &Game) -> Vec<Line> {
    let g = &state.glyphs;
    let mut dlls: Vec<&DetectedDll> = game.dlls.iter().collect();
    // Catalogue order, then by path, whatever order the cache kept.
    dlls.sort_by(|a, b| (a.dll_type, &a.path).cmp(&(b.dll_type, &b.path)));
    dlls.into_iter()
        .map(|dll| {
            let m = mark(state, dll);
            let mut line = vec![
                (fit(family(dll.dll_type).label, 8, g), Role::Text),
                (fit(version_text(dll), 14, g), Role::Strong),
                (fit(m.word(), 9, g), m.role()),
            ];
            if let Mark::Stale { latest } = &m {
                line.push((fit(&format!("latest {latest}"), 18, g), Role::Muted));
            }
            let rel = dll
                .path
                .strip_prefix(&game.install_dir)
                .unwrap_or(&dll.path);
            line.push((rel.display().to_string(), Role::Muted));
            line
        })
        .collect()
}

fn library_lines(state: &AppState, width: usize) -> Vec<Line> {
    let g = &state.glyphs;
    let fam = selected_family(state);
    let mut lines = vec![
        plain(fam.label, Role::Title),
        plain(format!("File: {}", fam.filename), Role::Text),
    ];
    let catalog = state.dlls.catalog.as_ref();
    let latest = match catalog.map(|c| &c.manifest) {
        None => "loading...".to_owned(),
        Some(Ok(Some(m))) => m
            .manifest
            .latest(fam.dll_type)
            .map_or_else(|| "unavailable".to_owned(), |e| e.version.clone()),
        Some(_) => "unavailable".to_owned(),
    };
    lines.push(plain(format!("Latest: {latest}"), Role::Text));
    let listed = match catalog.map(|c| &c.manifest) {
        None => "loading...".to_owned(),
        Some(Ok(Some(m))) => {
            let mut versions: Vec<&str> = m
                .manifest
                .valid_entries(fam.dll_type)
                .map(|e| e.version.as_str())
                .collect();
            versions.sort_by(|a, b| spela_core::dll::version::compare(b, a));
            versions.dedup();
            if versions.is_empty() {
                "unavailable".to_owned()
            } else {
                versions.join(", ")
            }
        }
        Some(_) => "unavailable".to_owned(),
    };
    lines.push(plain(format!("Manifest: {listed}"), Role::Text));
    let cached = match catalog.and_then(|c| c.cached.get(&fam.dll_type)) {
        None if catalog.is_none() => "loading...".to_owned(),
        Some(Ok(v)) if !v.is_empty() => v.join(", "),
        _ => "unavailable".to_owned(),
    };
    lines.push(plain(format!("Cached: {cached}"), Role::Text));
    match catalog.map(|c| &c.manifest) {
        Some(Ok(None)) => lines.push(plain("No cached manifest yet", Role::Muted)),
        Some(Err(e)) => lines.push(plain(e.clone(), Role::Error)),
        _ => {}
    }
    if let Some(Err(e)) = catalog.and_then(|c| c.cached.get(&fam.dll_type)) {
        lines.push(plain(e.clone(), Role::Error));
    }
    lines.push(Vec::new());
    let mut rows = Vec::new();
    for i in deployments(state) {
        let game = &state.games.games[i];
        for dll in game.dlls.iter().filter(|d| d.dll_type == fam.dll_type) {
            let m = mark(state, dll);
            let name_w = width.saturating_sub(26).max(12);
            rows.push(vec![
                (fit(&game.name, name_w, g), Role::Text),
                ("  ".to_owned(), Role::Text),
                (fit(version_text(dll), 14, g), Role::Strong),
                (m.word().to_owned(), m.role()),
            ]);
        }
    }
    lines.push(plain(format!("Deployments: {}", rows.len()), Role::Strong));
    if rows.is_empty() {
        lines.push(plain("No deployed DLLs", Role::Muted));
    }
    lines.extend(rows);
    lines
}

fn deployment_lines(state: &AppState) -> Vec<Line> {
    let Some(game) = selected_game(state) else {
        return vec![plain("No deployed DLL selected", Role::Muted)];
    };
    let mut lines = vec![
        plain("Deployment", Role::Title),
        plain(game.name.clone(), Role::Strong),
        Vec::new(),
    ];
    lines.extend(game_dll_lines(state, game));
    let stale = game
        .dlls
        .iter()
        .filter(|d| matches!(mark(state, d), Mark::Stale { .. }))
        .count();
    if stale > 0 {
        lines.push(Vec::new());
        lines.push(plain(format!("{stale} stale DLL target(s)"), Role::Muted));
    }
    if manifest(state).is_none() {
        lines.push(Vec::new());
        let why = match state.dlls.catalog.as_ref().map(|c| &c.manifest) {
            None => "Loading the manifest cache...",
            _ => "No cached manifest, so no DLL can be compared",
        };
        lines.push(plain(why, Role::Muted));
    }
    lines
}

/// The Detail lines for the selection.
pub fn detail_lines(state: &AppState) -> Vec<Line> {
    let width = usize::from(
        state
            .layout()
            .pane(Pane::Detail)
            .inner
            .width
            .saturating_sub(2),
    );
    match state.dlls.group {
        Group::Library => library_lines(state, width),
        Group::Deployment => deployment_lines(state),
    }
}

fn detail_height(state: &AppState) -> usize {
    usize::from(state.layout().pane(Pane::Detail).inner.height)
}

/// List rows for items: one less in single-pane mode, where the first row
/// shows the groups, because the pane title cannot.
fn list_height(state: &AppState) -> usize {
    let layout = state.layout();
    let strip = u16::from(!layout.two_panes());
    usize::from(layout.pane(Pane::List).inner.height.saturating_sub(strip)).max(1)
}

fn max_scroll(state: &AppState) -> usize {
    detail_lines(state)
        .len()
        .saturating_sub(detail_height(state))
}

/// Draws a pane.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    match area.pane {
        Pane::List => render_list(state, frame, area, hits),
        Pane::Detail => render_detail(state, frame, area),
    }
}

/// The right-hand text of a List row and its role. `deps` is
/// [`deployments`], computed once per frame.
fn list_tag(state: &AppState, deps: &[usize], row: usize) -> Option<(String, Role)> {
    match state.dlls.group {
        Group::Library => {
            let t = CATALOGUE[row].dll_type;
            let games: Vec<&Game> = deps
                .iter()
                .map(|i| &state.games.games[*i])
                .filter(|g| g.has(t))
                .collect();
            let stale = games
                .iter()
                .flat_map(|g| g.dlls.iter())
                .filter(|d| d.dll_type == t && matches!(mark(state, d), Mark::Stale { .. }))
                .count();
            match (stale, games.len()) {
                (0, 0) => None,
                (0, 1) => Some(("1 game".to_owned(), Role::Muted)),
                (0, n) => Some((format!("{n} games"), Role::Muted)),
                (s, _) => Some((format!("{s} stale"), Role::Warn)),
            }
        }
        Group::Deployment => {
            let i = *deps.get(row)?;
            let m = game_mark(state, &state.games.games[i]);
            Some((m.word().to_owned(), m.role()))
        }
    }
}

fn render_list(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    let inner = area.inner;
    let g: Glyphs = state.glyphs;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, g);
    if inner.width < 12 || inner.height == 0 {
        return;
    }
    let focused = state.focus == Pane::List;
    let x = inner.x + 1;
    let w = inner.width - 2;
    let d = &state.dlls;
    let deps = deployments(state);
    // In one pane the title reads `▸ List`, so the first row names the
    // groups, the current one in brackets, as the section strip does.
    let mut top = inner.y;
    if !state.layout().two_panes() {
        let mut at = x;
        for group in [Group::Library, Group::Deployment] {
            let on = group == d.group;
            let text = if on {
                format!("[{}]", group.label())
            } else {
                group.label().to_owned()
            };
            let start = at;
            at = p.text(
                at,
                top,
                w.saturating_sub(at - x),
                &text,
                if on { Role::Key } else { Role::Muted },
            );
            if !on {
                hits.push(
                    Rect::new(start, top, at - start, 1),
                    HitTarget::Action(act(Action::Group)),
                );
            }
            at += 2;
        }
        top += 1;
    }
    let (labels, cursor, scroll): (Vec<String>, usize, usize) = match d.group {
        Group::Library => (
            CATALOGUE.iter().map(|f| f.label.to_owned()).collect(),
            d.family,
            0,
        ),
        Group::Deployment => (
            deps.iter()
                .map(|i| state.games.games[*i].name.clone())
                .collect(),
            d.game,
            d.list_scroll,
        ),
    };
    if labels.is_empty() {
        p.text(x, top, w, "No deployed DLLs", Role::Muted);
        return;
    }
    let rows_area = Rect::new(inner.x, top, inner.width, inner.bottom() - top);
    let height = usize::from(rows_area.height);
    for (n, (i, text)) in labels
        .iter()
        .enumerate()
        .skip(scroll)
        .take(height)
        .enumerate()
    {
        let y = top + u16::try_from(n).unwrap_or(0);
        let line = Rect::new(inner.x, y, inner.width, 1);
        let is_cursor = i == cursor;
        if is_cursor {
            let role = if focused { Role::Key } else { Role::Muted };
            p.text(x, y, 1, g.cursor, role);
        }
        let tag = list_tag(state, &deps, i);
        let tag_w = tag.as_ref().map_or(0, |(t, _)| {
            u16::try_from(crate::text::width(t)).unwrap_or(0)
        });
        let name_w = w.saturating_sub(2 + tag_w + u16::from(tag_w > 0));
        let role = if is_cursor && focused {
            Role::Strong
        } else {
            Role::Text
        };
        p.text(x + 2, y, name_w, text, role);
        if let Some((t, r)) = tag {
            p.text(inner.right().saturating_sub(tag_w + 1), y, tag_w, &t, r);
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
    p.thumb(area.border_x, rows_area, scroll, height, labels.len());
}

fn render_detail(state: &AppState, frame: &mut Frame, area: PaneArea) {
    let inner = area.inner;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    if inner.width < 4 || inner.height == 0 {
        return;
    }
    let lines = detail_lines(state);
    let height = usize::from(inner.height);
    let scroll = state.dlls.scroll.min(lines.len().saturating_sub(height));
    let x = inner.x + 1;
    let w = inner.width - 2;
    for (n, line) in lines.iter().skip(scroll).take(height).enumerate() {
        let y = inner.y + u16::try_from(n).unwrap_or(0);
        let spans: Vec<(&str, Role)> = line.iter().map(|(t, r)| (t.as_str(), *r)).collect();
        p.spans(x, y, w, &spans);
    }
    p.thumb(area.border_x, inner, scroll, height, lines.len());
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;
    use spela_core::dll::manifest::ManifestEntry;

    fn catalog(latest: &str) -> Catalog {
        let mut m = Manifest::default();
        m.dlls.insert(
            "dlss".into(),
            vec![ManifestEntry {
                version: latest.into(),
                filename: "nvngx_dlss.dll".into(),
                ..ManifestEntry::default()
            }],
        );
        Catalog {
            manifest: Ok(Some(CachedManifest {
                manifest: m,
                ..CachedManifest::default()
            })),
            cached: BTreeMap::from([(DllType::Dlss, Ok(vec!["310.5.3".to_owned()]))]),
        }
    }

    fn loaded(s: &mut AppState, c: Catalog) {
        let cmds = start(s);
        let AnyCmd::Dlls(Cmd::Load { request }) = cmds[0] else {
            panic!("{cmds:?}")
        };
        on_msg(
            s,
            Msg::Loaded {
                request,
                result: Ok(c),
            },
        );
    }

    #[test]
    fn marks_come_from_the_shared_stale_function() {
        let mut s = state_with_games(120, 40, 6);
        let dll = s.games.games[0].dlls[0].clone();
        assert_eq!(mark(&s, &dll), Mark::Unknown, "no manifest yet");
        loaded(&mut s, catalog("310.5.3"));
        assert_eq!(
            mark(&s, &dll),
            Mark::Stale {
                latest: "310.5.3".into()
            }
        );
        assert_eq!(stale_count(&s), 3);
        loaded(&mut s, catalog("310.4.0"));
        assert_eq!(mark(&s, &dll), Mark::Current);
        let mut unknown = dll.clone();
        unknown.version = None;
        assert_eq!(mark(&s, &unknown), Mark::Unknown);
    }

    #[test]
    fn a_stale_load_is_dropped() {
        let mut s = state_with_games(120, 40, 2);
        let cmds = start(&mut s);
        let AnyCmd::Dlls(Cmd::Load { request }) = cmds[0] else {
            panic!()
        };
        on_msg(
            &mut s,
            Msg::Loaded {
                request: request + 1,
                result: Ok(catalog("1")),
            },
        );
        assert!(s.dlls.catalog.is_none());
        on_msg(
            &mut s,
            Msg::Loaded {
                request,
                result: Err("boom".into()),
            },
        );
        assert!(manifest(&s).is_none());
        assert!(s.dlls.catalog.is_some());
    }

    #[test]
    fn left_and_right_switch_the_group() {
        let mut s = state_with_games(120, 40, 6);
        step(&mut s, key("2"));
        assert_eq!(title(&s, Pane::List), "DLL Catalog · Library");
        step(&mut s, key("right"));
        assert_eq!(s.dlls.group, Group::Deployment);
        assert_eq!(deployments(&s).len(), 3);
        step(&mut s, key("down"));
        assert_eq!(title(&s, Pane::Detail), s.games.games[2].name);
        step(&mut s, key("left"));
        assert_eq!(s.dlls.group, Group::Library);
    }
}
