//! 1 Library, Detail: the game's DLLs section, which the profile page
//! draws when DLLs is its section (TUI 5 and 12).
//!
//! The section shows where the game lives, its DLL versions with the
//! Catalog's marks, whether a backup exists, the deny list line, the last
//! result, and three actions under `DLL actions: <game>`:
//!
//! - **Install DLL** opens a picker in the section, `Select a supported DLL
//!   family` (the game's families, or every family when it has none), then
//!   `Select <family> version` (every version the manifest lists, the
//!   latest first). Picking one opens the confirmation. Esc closes the
//!   picker and says the install was cancelled.
//! - **Update DLLs (N files)** updates the DLLs the stale function marks
//!   stale (decision 38) to the manifest's latest, after the confirmation
//!   `confirm_destructive` can skip.
//! - **Restore originals (N files)** puts the backed up originals back and
//!   removes what spela installed where nothing was (decision 37).
//!
//! An action that cannot run stays, dimmed, with `Unavailable: <reason>`
//! under it. Install and update wait while a deny list names the game
//! (`DLL swaps denied (<reason>)`); restore never does. While the game's
//! job runs, the section reads `Work is running. Controls return when it
//! finishes.` Up and Down choose an action and Enter runs it; the same
//! three rows are in Actions and the palette. The jobs themselves are
//! [`crate::dll_jobs`]'s.

use ratatui::layout::Rect;
use spela_core::dll::detect::{CATALOGUE, family};
use spela_core::games::{DllType, Game};
use spela_core::profile::Scope;

use super::{Action as AnyAction, Row, ScreenAction, confirm, denylist, dlls, library, profile};
use crate::dll_jobs::{self, DllOp, Origin};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::Pane;
use crate::paint::Painter;
use crate::picker::{Choice, Picker, PickerInput, PickerOutcome};
use crate::state::{AppState, Level};
use crate::text::wrap;
use crate::theme::Role;

/// The widest picker box, as the profile page's editors.
const PICKER_W: u16 = 56;

/// One of the three actions, in their order.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Kind {
    /// Install DLL.
    Install,
    /// Update DLLs.
    Update,
    /// Restore originals.
    Restore,
}

impl Kind {
    /// The three, in order.
    pub const ALL: [Kind; 3] = [Kind::Install, Kind::Update, Kind::Restore];

    /// The action's name, as in Go.
    pub fn name(self) -> &'static str {
        match self {
            Kind::Install => "Install DLL",
            Kind::Update => "Update DLLs",
            Kind::Restore => "Restore originals",
        }
    }

    fn explanation(self) -> &'static str {
        match self {
            Kind::Install => "Choose a family and version, then review the target.",
            Kind::Update => "Review each detected stale DLL before updating.",
            Kind::Restore => "Restore backed-up files. DLLs spela added are removed.",
        }
    }
}

/// The install picker.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Chooser {
    /// Choosing the family.
    Family {
        /// The game.
        app_id: u64,
        /// The picker.
        picker: Picker,
        /// Each choice's family.
        families: Vec<DllType>,
    },
    /// Choosing the version.
    Version {
        /// The game.
        app_id: u64,
        /// The family.
        dll_type: DllType,
        /// The picker.
        picker: Picker,
        /// Each choice's version.
        versions: Vec<String>,
    },
}

impl Chooser {
    fn app_id(&self) -> u64 {
        match self {
            Chooser::Family { app_id, .. } | Chooser::Version { app_id, .. } => *app_id,
        }
    }

    fn picker(&self) -> &Picker {
        match self {
            Chooser::Family { picker, .. } | Chooser::Version { picker, .. } => picker,
        }
    }
}

/// The section's state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The action under the cursor.
    pub cursor: usize,
    /// The open install picker.
    pub chooser: Option<Chooser>,
}

/// What the section's keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// The previous action.
    Up,
    /// The next action.
    Down,
    /// A click on an action row.
    Select(usize),
    /// Enter: runs the action under the cursor.
    Enter,
    /// Runs one action, from Enter, a row, Actions or the palette.
    Run(Kind),
    /// A key for the install picker.
    Pick(PickerInput),
}

fn act(a: Action) -> AnyAction {
    AnyAction::Screen(ScreenAction::GameDlls(a))
}

fn pick(input: PickerInput) -> AnyAction {
    act(Action::Pick(input))
}

fn pick_typed(c: char) -> AnyAction {
    act(Action::Pick(PickerInput::Type(c)))
}

/// The game the section shows.
fn game(state: &AppState) -> Option<&Game> {
    match library::selected_scope(state)? {
        Scope::Game(_) => library::selected_game(state),
        Scope::AllGames => None,
    }
}

/// The open picker, when it belongs to the game the section shows.
fn open_chooser(state: &AppState) -> Option<&Chooser> {
    let chooser = state.game_dlls.chooser.as_ref()?;
    (profile::aspect(state) == profile::Aspect::Dlls
        && game(state).is_some_and(|g| g.app_id == chooser.app_id()))
    .then_some(chooser)
}

/// Whether the install picker is open, so its keys hold.
pub fn editing(state: &AppState) -> bool {
    open_chooser(state).is_some()
}

/// The families Install offers: the game's own, or every family when it
/// has none, each only when the manifest lists a version of it.
fn install_families(state: &AppState, game: &Game) -> Result<Vec<DllType>, String> {
    let Some(manifest) = dlls::manifest(state) else {
        return Err("DLL manifest unavailable".into());
    };
    let found: Vec<DllType> = CATALOGUE
        .iter()
        .map(|f| f.dll_type)
        .filter(|t| game.dlls.is_empty() || game.has(*t))
        .filter(|t| manifest.valid_entries(*t).next().is_some())
        .collect();
    if found.is_empty() {
        Err("no DLL in the manifest for this game's families".into())
    } else {
        Ok(found)
    }
}

/// Whether an action can run for `game`, or why not.
fn available(state: &AppState, game: &Game, kind: Kind) -> Result<(), String> {
    dll_jobs::can_start(state)?;
    match kind {
        Kind::Install => {
            denylist::swap_allowed(state, game.app_id)?;
            install_families(state, game).map(|_| ())
        }
        Kind::Update => {
            denylist::swap_allowed(state, game.app_id)?;
            if dll_jobs::stale_targets(state, [game]).is_empty() {
                Err("no stale DLLs for this game".into())
            } else {
                Ok(())
            }
        }
        Kind::Restore => match dll_jobs::backup(state, game.app_id) {
            None => Err("reading the backup".into()),
            Some(Err(e)) => Err(format!("backup unreadable: {e}")),
            Some(Ok(t)) if t.is_empty() => Err("no backup for this game".into()),
            Some(Ok(_)) => Ok(()),
        },
    }
}

fn files(n: usize) -> String {
    format!("{n} {}", if n == 1 { "file" } else { "files" })
}

/// The restore's file count, once the backup is read.
fn restore_count(state: &AppState, app_id: u64) -> Option<usize> {
    match dll_jobs::backup(state, app_id) {
        Some(Ok(t)) => Some(t.len()),
        _ => None,
    }
}

/// An action's title in the section, with its count.
fn title(state: &AppState, game: &Game, kind: Kind) -> String {
    match kind {
        Kind::Install => kind.name().to_owned(),
        Kind::Update => {
            let n = dll_jobs::stale_targets(state, [game]).len();
            format!("{} ({})", kind.name(), files(n))
        }
        Kind::Restore => match restore_count(state, game.app_id) {
            Some(n) => format!("{} ({})", kind.name(), files(n)),
            None => kind.name().to_owned(),
        },
    }
}

/// An action's Actions and palette row label, as in Go.
fn row_label(state: &AppState, game: &Game, kind: Kind) -> String {
    let name = &game.name;
    match kind {
        Kind::Install => format!("Install DLL: {name}"),
        Kind::Update => {
            let n = dll_jobs::stale_targets(state, [game]).len();
            format!("Update stale DLLs: {name} ({n})")
        }
        Kind::Restore => match restore_count(state, game.app_id) {
            Some(n) => format!("Restore originals: {name} ({})", files(n)),
            None => format!("Restore originals: {name}"),
        },
    }
}

/// The three rows for Actions and the palette.
pub fn rows(state: &AppState) -> Vec<Row> {
    let Some(game) = game(state) else {
        return Vec::new();
    };
    Kind::ALL
        .iter()
        .map(|k| {
            Row::new(row_label(state, game, *k), act(Action::Run(*k)))
                .when(available(state, game, *k))
        })
        .collect()
}

fn selected(state: &AppState) -> Kind {
    Kind::ALL[state.game_dlls.cursor.min(Kind::ALL.len() - 1)]
}

/// The section's keys, before the profile page's Left and Right.
pub fn bindings(state: &AppState) -> Vec<Binding> {
    if let Some(chooser) = open_chooser(state) {
        return chooser.picker().bindings(pick, pick_typed);
    }
    let Some(game) = game(state) else {
        return Vec::new();
    };
    // While the game's own job runs, the section has no controls.
    if running_here(state, game.app_id).is_some() {
        return Vec::new();
    }
    let kind = selected(state);
    vec![
        Binding::new(Key::UP, act(Action::Up), "Action").hint_as("Up/Down", 100),
        Binding::new(Key::DOWN, act(Action::Down), "Next action"),
        Binding::new(Key::ENTER, act(Action::Enter), kind.name())
            .hint(90)
            .when(available(state, game, kind)),
    ]
}

/// Runs an action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    let mut cmds = match action {
        Action::Up => {
            state.game_dlls.cursor = state.game_dlls.cursor.saturating_sub(1);
            Vec::new()
        }
        Action::Down => {
            state.game_dlls.cursor = (state.game_dlls.cursor + 1).min(Kind::ALL.len() - 1);
            Vec::new()
        }
        Action::Select(i) => {
            state.game_dlls.cursor = i.min(Kind::ALL.len() - 1);
            Vec::new()
        }
        Action::Enter => run(state, selected(state)),
        Action::Run(kind) => {
            state.game_dlls.cursor = Kind::ALL.iter().position(|k| *k == kind).unwrap_or(0);
            run(state, kind)
        }
        Action::Pick(input) => pick_input(state, input),
    };
    cmds.extend(dll_jobs::want_backup(state));
    cmds
}

fn run(state: &mut AppState, kind: Kind) -> Vec<AnyCmd> {
    let Some(game) = game(state).cloned() else {
        return Vec::new();
    };
    // A row from Actions or the palette shows the section it acts on.
    state.focus = Pane::Detail;
    state.profile.aspect = profile::Aspect::Dlls;
    state.profile.editor = None;
    if available(state, &game, kind).is_err() {
        return Vec::new();
    }
    match kind {
        Kind::Install => {
            open_families(state, &game);
            Vec::new()
        }
        Kind::Update => {
            let targets = dll_jobs::stale_targets(state, [&game]);
            let items = dll_jobs::update_items(state, &targets);
            dll_jobs::ask(
                state,
                confirm::Guard::DllUpdate,
                "Confirm DLL update",
                dll_jobs::body(&items, "current DLL is backed up before replacement"),
                "DLL update",
                DllOp::Update {
                    requests: dll_jobs::update_requests(&targets),
                },
                Origin::Game(game.app_id),
            )
        }
        Kind::Restore => {
            let Some(Ok(targets)) = dll_jobs::backup(state, game.app_id).cloned() else {
                return Vec::new();
            };
            let items = dll_jobs::restore_items(state, &game, &targets);
            dll_jobs::ask(
                state,
                confirm::Guard::DllRestore,
                "Confirm DLL restore",
                dll_jobs::body(&items, dll_jobs::RESTORE_BACKUP),
                "DLL restore",
                DllOp::Restore {
                    app_id: game.app_id,
                    expected: targets,
                },
                Origin::Game(game.app_id),
            )
        }
    }
}

fn open_families(state: &mut AppState, game: &Game) {
    let Ok(families) = install_families(state, game) else {
        return;
    };
    let choices = families
        .iter()
        .map(|t| Choice {
            label: format!("{} ({})", family(*t).label, family(*t).filename),
            current: false,
        })
        .collect();
    // One input at a time: the picker closes the Find row.
    library::stop_finding(state);
    state.game_dlls.chooser = Some(Chooser::Family {
        app_id: game.app_id,
        picker: Picker::new("Select a supported DLL family", None, choices),
        families,
    });
}

fn open_versions(state: &mut AppState, app_id: u64, dll_type: DllType) {
    let Some(manifest) = dlls::manifest(state) else {
        return;
    };
    let mut versions: Vec<String> = manifest
        .valid_entries(dll_type)
        .map(|e| e.version.clone())
        .collect();
    versions.sort_by(|a, b| spela_core::dll::version::compare(b, a));
    versions.dedup();
    let installed = state
        .games
        .get(app_id)
        .and_then(|g| g.dlls.iter().find(|d| d.dll_type == dll_type))
        .and_then(|d| d.version.clone());
    let latest = manifest.latest(dll_type).map(|e| e.version.clone());
    let choices = versions
        .iter()
        .map(|v| Choice {
            label: if Some(v) == latest.as_ref() {
                format!("{v} (latest)")
            } else {
                v.clone()
            },
            current: installed
                .as_deref()
                .is_some_and(|i| spela_core::dll::version::compare(i, v).is_eq()),
        })
        .collect();
    let label = family(dll_type).label;
    let before = Some(installed.unwrap_or_else(|| "not installed".to_owned()));
    let mut picker = Picker::new(format!("Select {label} version"), before, choices);
    // The latest first, as Go's list starts, whatever is installed.
    picker.cursor = 0;
    state.game_dlls.chooser = Some(Chooser::Version {
        app_id,
        dll_type,
        picker,
        versions,
    });
}

fn cancel_install(state: &mut AppState) -> Vec<AnyCmd> {
    state.game_dlls.chooser = None;
    state.say(Level::Info, "DLL install cancelled, no operation ran");
    Vec::new()
}

fn pick_input(state: &mut AppState, input: PickerInput) -> Vec<AnyCmd> {
    let Some(chooser) = state.game_dlls.chooser.as_mut() else {
        return Vec::new();
    };
    let outcome = match chooser {
        Chooser::Family { picker, .. } | Chooser::Version { picker, .. } => picker.handle(input),
    };
    match (outcome, chooser.clone()) {
        (PickerOutcome::Moved, _) => Vec::new(),
        (PickerOutcome::Cancel, _) => cancel_install(state),
        (
            PickerOutcome::Apply(i),
            Chooser::Family {
                app_id, families, ..
            },
        ) => {
            match families.get(i) {
                Some(t) => open_versions(state, app_id, *t),
                None => {
                    state.game_dlls.chooser = None;
                }
            }
            Vec::new()
        }
        (
            PickerOutcome::Apply(i),
            Chooser::Version {
                app_id,
                dll_type,
                versions,
                ..
            },
        ) => {
            state.game_dlls.chooser = None;
            match versions.get(i) {
                Some(v) => confirm_install(state, app_id, dll_type, v),
                None => Vec::new(),
            }
        }
    }
}

fn confirm_install(
    state: &mut AppState,
    app_id: u64,
    dll_type: DllType,
    version: &str,
) -> Vec<AnyCmd> {
    let Some(game) = state.games.get(app_id).cloned() else {
        return Vec::new();
    };
    let filename = dlls::manifest(state)
        .and_then(|m| m.valid_entries(dll_type).find(|e| e.version == version))
        .map_or_else(
            || family(dll_type).filename.to_owned(),
            |e| e.filename.clone(),
        );
    let detected = game
        .dlls
        .iter()
        .find(|d| d.name.eq_ignore_ascii_case(&filename));
    let path = detected.map_or_else(|| game.install_dir.join(&filename), |d| d.path.clone());
    let from = detected.and_then(|d| d.version.clone());
    let line = dll_jobs::target_line(
        state,
        &game.name,
        family(dll_type).label,
        from.as_deref(),
        version,
    );
    dll_jobs::ask(
        state,
        confirm::Guard::DllInstall,
        "Confirm DLL install",
        dll_jobs::body(
            &[(line, path)],
            "original DLL is backed up before replacement",
        ),
        "DLL install",
        DllOp::Install {
            app_id,
            dll_type,
            version: version.to_owned(),
        },
        Origin::Game(app_id),
    )
}

/// One line of the section: spans, or an action row with its index.
enum Line {
    Spans(Vec<(String, Role)>),
    Action(usize, Vec<(String, Role)>),
    /// The install picker, this many rows high.
    Picker(u16),
}

fn spans(text: impl Into<String>, role: Role) -> Line {
    Line::Spans(vec![(text.into(), role)])
}

/// The shortest picker box: its frame, the value before the edit and a
/// blank, one choice, a blank and the buttons.
const MIN_PICKER_H: u16 = 7;

fn picker_width(w: u16) -> u16 {
    PICKER_W.min(w.saturating_sub(4))
}

/// The versions, the backup, the deny list and the last result.
fn versions(state: &AppState, game: &Game, w: usize) -> Vec<Line> {
    let mut lines = vec![spans("DLL versions", Role::Title)];
    if game.dlls.is_empty() {
        lines.push(spans("No managed DLL installed", Role::Text));
        let families: Vec<&str> = CATALOGUE.iter().map(|f| f.label).collect();
        lines.push(spans(
            format!("Supported families: {}", families.join(", ")),
            Role::Muted,
        ));
    } else {
        // The Catalog's lines, with the same stale marks (decision 38).
        lines.extend(
            dlls::game_dll_lines(state, game)
                .into_iter()
                .map(Line::Spans),
        );
    }
    if restore_count(state, game.app_id).is_some_and(|n| n > 0) {
        lines.push(spans("Backup available", Role::Muted));
    }
    // The deny list line, only when a list names the game.
    if let Some((text, role)) = denylist::state_line(state, game.app_id) {
        lines.push(spans(text, role));
    }
    if let Some((text, level)) = state.dll_jobs.results.get(&game.app_id) {
        let role = match level {
            Level::Ok => Role::Ok,
            Level::Error => Role::Error,
            Level::Info => Role::Muted,
        };
        for l in wrap(text, w.max(20)) {
            lines.push(spans(l, role));
        }
    }
    lines
}

/// The job that runs for this game's section, if one does.
fn running_here(state: &AppState, app_id: u64) -> Option<&dll_jobs::Running> {
    state
        .dll_jobs
        .running
        .as_ref()
        .filter(|r| r.origin == Origin::Game(app_id))
}

/// The actions block, or the running job's words.
fn controls(state: &AppState, game: &Game) -> Vec<Line> {
    if let Some(r) = running_here(state, game.app_id) {
        return vec![
            spans("DLL operation", Role::Title),
            spans(dll_jobs::words(r), Role::Warn),
            spans(
                "Work is running. Controls return when it finishes.",
                Role::Muted,
            ),
        ];
    }
    let focused = state.focus == Pane::Detail;
    let cursor = selected(state);
    let mut lines = vec![spans(format!("DLL actions: {}", game.name), Role::Title)];
    let chooser = open_chooser(state);
    for (i, kind) in Kind::ALL.iter().enumerate() {
        let ok = available(state, game, *kind);
        let on = *kind == cursor && focused && chooser.is_none();
        let role = match (&ok, on) {
            (_, true) => Role::Key,
            (Ok(()), false) => Role::Text,
            (Err(_), false) => Role::Muted,
        };
        let mark = if on { state.glyphs.cursor } else { " " };
        lines.push(Line::Action(
            i,
            vec![(format!("{mark} {}", title(state, game, *kind)), role)],
        ));
        if *kind == Kind::Install
            && let Some(c) = chooser
        {
            lines.push(Line::Picker(c.picker().height()));
        }
        if let Err(reason) = ok {
            lines.push(spans(format!("    Unavailable: {reason}"), Role::Muted));
        }
    }
    lines.push(spans(cursor.explanation(), Role::Muted));
    lines
}

/// Draws the section into `area`, below the profile page's header.
pub fn render(
    state: &AppState,
    p: &mut Painter<'_>,
    hits: &mut HitMap,
    area: Rect,
    x: u16,
    w: u16,
) {
    let Some(game) = game(state) else {
        return;
    };
    let h = usize::from(area.height);
    let mut bottom = controls(state, game);
    let height = |lines: &[Line]| -> usize {
        lines
            .iter()
            .map(|l| match l {
                Line::Picker(n) => usize::from(*n),
                _ => 1,
            })
            .sum()
    };
    let mut top = versions(state, game, usize::from(w));
    // While a picker is open, the versions it chooses between stay in view:
    // the rows under the picker give way first, then the picker shrinks to
    // a window on its choices, down to a box with one choice.
    let picker_at = bottom.iter().position(|l| matches!(l, Line::Picker(_)));
    if let Some(at) = picker_at
        && let Line::Picker(rows) = bottom[at]
    {
        let fit = h.saturating_sub(top.len() + 1 + height(&bottom[..at]));
        let fit = u16::try_from(fit).unwrap_or(u16::MAX);
        bottom[at] = Line::Picker(rows.min(fit.max(MIN_PICKER_H)));
    }
    let bottom_h = height(&bottom);
    let keep_h = match picker_at {
        Some(at) => height(&bottom[..=at]),
        None => bottom_h,
    };
    // The game's facts lead when there is room for them too; on a short
    // terminal the versions win, as in Go.
    let mut facts = vec![spans(
        format!("Install directory: {}", game.install_dir.display()),
        Role::Muted,
    )];
    if let Some(prefix) = &game.prefix_path {
        facts.push(spans(
            format!("Proton prefix: {}", prefix.display()),
            Role::Muted,
        ));
    }
    facts.push(spans("Steam launch option: spela %command%", Role::Muted));
    if facts.len() + 1 + top.len() + 1 + bottom_h <= h {
        facts.push(Line::Spans(Vec::new()));
        facts.append(&mut top);
        top = facts;
    }
    // The controls are kept first; the versions give way.
    let room = h.saturating_sub(keep_h + 1);
    if top.len() > room {
        top.truncate(room);
        if room > 0 {
            let n = game.dlls.len();
            let summary = if n == 0 {
                "No managed DLL installed".to_owned()
            } else {
                format!("{n} installed DLL files")
            };
            top[room - 1] = spans(summary, Role::Muted);
        }
    }
    let mut y = area.y;
    let end = area.bottom();
    let draw = |line: &Line, y: &mut u16, p: &mut Painter<'_>, hits: &mut HitMap| {
        if *y >= end {
            return;
        }
        match line {
            Line::Spans(s) => {
                let s: Vec<(&str, Role)> = s.iter().map(|(t, r)| (t.as_str(), *r)).collect();
                p.spans(x, *y, w, &s);
                *y += 1;
            }
            Line::Action(i, s) => {
                let s: Vec<(&str, Role)> = s.iter().map(|(t, r)| (t.as_str(), *r)).collect();
                p.spans(x, *y, w, &s);
                let row = Rect::new(x.saturating_sub(1), *y, w + 2, 1);
                if s.first().is_some_and(|(_, r)| *r == Role::Key) {
                    p.cursor_row(row);
                }
                hits.push(
                    row,
                    HitTarget::Row {
                        select: act(Action::Select(*i)),
                        open: act(Action::Enter),
                    },
                );
                *y += 1;
            }
            Line::Picker(rows) => {
                let room = end.saturating_sub(*y).min(*rows);
                if let Some(c) = open_chooser(state) {
                    c.picker()
                        .render(p, Rect::new(x + 4, *y, picker_width(w), room));
                }
                *y += room;
            }
        }
    };
    for line in &top {
        draw(line, &mut y, p, hits);
    }
    if !top.is_empty() {
        y += 1;
    }
    for line in &bottom {
        draw(line, &mut y, p, hits);
    }
}
