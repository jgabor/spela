//! DLL jobs: install, update, restore and the startup auto-update, shared
//! by the game's DLLs section ([`crate::screens::game_dlls`]), the DLL
//! Catalog's update-all ([`crate::screens::dlls`]) and the Library's batch
//! update ([`crate::screens::library`]).
//!
//! One job runs at a time, on the shell's job pool ([`Cmd::Dll`]), and
//! [`crate::state::Jobs::dll`] holds its token while it runs, so Quit is
//! refused and the DLL rows say why they wait. A scan and a DLL job each
//! replace the game cache, so neither starts while the other runs.
//!
//! - **Asking first.** Install, restore and every update open the
//!   confirmation with Cancel focused: a target list (`1 DLL target(s)`,
//!   one `game · family · from -> to` line and its path per target) and a
//!   `Backup:` line, Go's words (TUI 12). `confirm_destructive = false`
//!   skips only the update's (decision 4). Cancel says `<op> cancelled, no
//!   operation ran`.
//! - **Progress** comes as one [`Msg::DllProgress`] per stage and shows as
//!   words on the right of the status line, such as `Downloading DLSS
//!   310.5.3...`. No spinner, no percentage (terminal-ux.md).
//! - **Results** go to the status line in Go's words, and the game's DLLs
//!   section and the Catalog keep their last one. The job sends the game
//!   cache as it is afterwards, so every view shows the rescanned DLLs.
//! - **Update targets** are the DLLs the one stale function marks stale
//!   (decision 38), so a row's count, the confirmation and the requests come
//!   from one list.
//! - **Restore targets** come from the backup index, read on a worker. The
//!   job checks them again before it changes anything and answers
//!   [`DllResult::BackupChanged`] when they moved, so the confirmation
//!   opens again as `Backup changed: review restore`.
//! - **Quitting.** A stop signal, a closed terminal or Ctrl-C during a job
//!   ([`stop`]) lets the job finish its current step: the shell sets the
//!   job's stop flag, a download ends, a batch stops between items and
//!   saves what changed, and spela quits when the job answers. A second
//!   signal exits at once.

use std::collections::{BTreeMap, BTreeSet};
use std::path::PathBuf;

use spela_core::dll::auto::AutoUpdate;
use spela_core::dll::backup::{RestoreTarget, clean};
use spela_core::dll::detect::{dll_type_of, family};
use spela_core::dll::ops::{
    BatchResult, Installed, OpError, Outcome, ProgressEvent, Restored, Stage, UpdateRequest,
};
use spela_core::games::{DllType, Game, GameCache};
use spela_core::profile::Scope;

use crate::event::Cmd;
use crate::screens::{Action, confirm, dlls, library, profile};
use crate::state::{AppState, Level};
use crate::theme::Role;

/// What the status line says while spela waits for a DLL job to quit.
pub const QUITTING: &str = "Quitting after this DLL step...";

/// A DLL operation for the job pool.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum DllOp {
    /// Install one version of a family into a game.
    Install {
        /// The game.
        app_id: u64,
        /// The family.
        dll_type: DllType,
        /// The manifest version.
        version: String,
    },
    /// Update these detected DLLs, downloading as needed.
    Update {
        /// One per DLL.
        requests: Vec<UpdateRequest>,
    },
    /// Put a game's originals back, if the backup still lists `expected`.
    Restore {
        /// The game.
        app_id: u64,
        /// The targets the confirmation showed.
        expected: Vec<RestoreTarget>,
    },
    /// The startup auto-update (decision 39).
    AutoUpdate,
}

/// Where a job started, which decides its words and where its result
/// stays.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Origin {
    /// The game's DLLs section, or its Actions rows.
    Game(u64),
    /// The Catalog's update-all.
    Catalog,
    /// Library's batch update of this many selected games.
    Batch {
        /// The games asked for.
        requested: usize,
    },
    /// The startup auto-update.
    Auto,
}

/// What a job did.
#[derive(Clone, Debug)]
pub enum DllResult {
    /// An install.
    Installed(Result<Installed, OpError>),
    /// An update, or why no manifest could be had.
    Updated(Result<BatchResult, String>),
    /// A restore.
    Restored(Result<Restored, OpError>),
    /// The backup no longer lists what the confirmation showed: its
    /// targets now, or why the index cannot be read. Nothing changed.
    BackupChanged(Result<Vec<RestoreTarget>, String>),
    /// The startup auto-update.
    Auto(AutoUpdate),
    /// The job failed before it could run, or panicked.
    Failed(String),
}

/// A finished job.
#[derive(Clone, Debug)]
pub struct DllDone {
    /// What it did.
    pub result: DllResult,
    /// The game cache after it, when it could be read.
    pub games: Option<GameCache>,
}

/// The job that runs.
#[derive(Clone, Debug, PartialEq)]
pub struct Running {
    /// Its token.
    pub token: u64,
    /// Where it started.
    pub origin: Origin,
    /// Its words before the first progress, such as `Updating DLLs...`.
    pub label: &'static str,
    /// Its last progress.
    pub progress: Option<ProgressEvent>,
}

/// A game's backup, as a worker read it.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Backup {
    /// Being read, by request.
    Loading(u64),
    /// What a restore would change, empty for no backup, or why the index
    /// cannot be read.
    Ready(Result<Vec<RestoreTarget>, String>),
}

/// The DLL jobs' state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The job that runs.
    pub running: Option<Running>,
    /// spela quits when the running job answers.
    pub stopping: bool,
    /// Each game's last result, for its DLLs section.
    pub results: BTreeMap<u64, (String, Level)>,
    /// The Catalog's last update-all result.
    pub catalog_result: Option<(String, Level)>,
    /// Backups read so far, by game. Cleared after every job.
    pub backups: BTreeMap<u64, Backup>,
    /// The startup auto-update waits for the startup scan.
    pub auto_pending: bool,
}

/// What Confirm on a DLL confirmation starts.
#[derive(Clone, Debug, PartialEq)]
pub struct Start {
    /// The operation.
    pub op: DllOp,
    /// Where it comes from.
    pub origin: Origin,
}

/// One DLL an update would change.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Target {
    /// The game.
    pub app_id: u64,
    /// Its name.
    pub game: String,
    /// The family.
    pub dll_type: DllType,
    /// The detected file.
    pub path: PathBuf,
    /// Its version, when known.
    pub from: Option<String>,
    /// The manifest's latest version.
    pub to: String,
}

/// Whether a DLL job may start now, or why not.
pub fn can_start(state: &AppState) -> Result<(), String> {
    if state.jobs.dll.is_some() {
        Err("finish the current DLL operation".into())
    } else if state.jobs.scan.is_some() {
        Err("wait for pending work".into())
    } else {
        Ok(())
    }
}

/// The stale DLLs of `games`, by the Catalog's marks (decision 38), in game
/// order, then catalogue order, then path.
pub fn stale_targets<'g>(
    state: &AppState,
    games: impl IntoIterator<Item = &'g Game>,
) -> Vec<Target> {
    let mut out = Vec::new();
    for game in games {
        let mut found: Vec<_> = game.dlls.iter().collect();
        found.sort_by(|a, b| (a.dll_type, &a.path).cmp(&(b.dll_type, &b.path)));
        for dll in found {
            if let dlls::Mark::Stale { latest } = dlls::mark(state, dll) {
                out.push(Target {
                    app_id: game.app_id,
                    game: game.name.clone(),
                    dll_type: dll.dll_type,
                    path: dll.path.clone(),
                    from: dll.version.clone().filter(|v| !v.is_empty()),
                    to: latest,
                });
            }
        }
    }
    out
}

/// The update requests for `targets`, each to its latest version.
pub fn update_requests(targets: &[Target]) -> Vec<UpdateRequest> {
    targets
        .iter()
        .map(|t| UpdateRequest {
            app_id: t.app_id,
            dll_type: t.dll_type,
            path: t.path.clone(),
            version: Some(t.to.clone()),
            cached_only: false,
        })
        .collect()
}

/// `game · family · from -> to`, or `game · family · to` when the version
/// is not known, as Go's target list reads.
pub fn target_line(
    state: &AppState,
    game: &str,
    fam: &str,
    from: Option<&str>,
    to: &str,
) -> String {
    let dot = state.glyphs.dot;
    match from {
        Some(f) => format!("{game} {dot} {fam} {dot} {f} -> {to}"),
        None => format!("{game} {dot} {fam} {dot} {to}"),
    }
}

/// An update's target lines with their paths.
pub fn update_items(state: &AppState, targets: &[Target]) -> Vec<(String, PathBuf)> {
    targets
        .iter()
        .map(|t| {
            (
                target_line(
                    state,
                    &t.game,
                    family(t.dll_type).label,
                    t.from.as_deref(),
                    &t.to,
                ),
                t.path.clone(),
            )
        })
        .collect()
}

/// A restore's target lines: each original goes back to its backup, and
/// each file spela created is removed (decision 37).
pub fn restore_items(
    state: &AppState,
    game: &Game,
    targets: &[RestoreTarget],
) -> Vec<(String, PathBuf)> {
    targets
        .iter()
        .map(|t| {
            let fam = dll_type_of(&t.dll_name).map_or(t.dll_name.as_str(), |d| family(d).label);
            let current = game
                .dlls
                .iter()
                .find(|d| clean(&d.path) == clean(&t.path))
                .and_then(|d| d.version.as_deref());
            let to = if t.deletes {
                "removed".to_owned()
            } else if t.version.is_empty() {
                "backup".to_owned()
            } else {
                format!("backup {}", t.version)
            };
            (
                target_line(state, &game.name, fam, current, &to),
                t.path.clone(),
            )
        })
        .collect()
}

/// The widest body line: what the confirmation shows at 80x24, so a path
/// wraps instead of being cut.
const BODY_W: usize = 70;

/// A DLL confirmation's body: the target count, each target and its path,
/// and the backup policy.
pub fn body(items: &[(String, PathBuf)], backup: &str) -> Vec<confirm::Line> {
    let wrapped = |text: &str, role: Role| -> Vec<confirm::Line> {
        crate::text::wrap(text, BODY_W)
            .into_iter()
            .map(|l| vec![(l, role)])
            .collect()
    };
    let mut lines = vec![
        vec![(format!("{} DLL target(s)", items.len()), Role::Strong)],
        Vec::new(),
    ];
    for (line, path) in items {
        lines.extend(wrapped(line, Role::Text));
        lines.extend(wrapped(&path.display().to_string(), Role::Muted));
        lines.push(Vec::new());
    }
    lines.extend(wrapped(&format!("Backup: {backup}"), Role::Muted));
    lines
}

/// Opens a DLL confirmation, or starts at once when `guard` does not ask.
pub fn ask(
    state: &mut AppState,
    guard: confirm::Guard,
    title: &str,
    body: Vec<confirm::Line>,
    what: &str,
    op: DllOp,
    origin: Origin,
) -> Vec<Cmd> {
    let action = Action::Dll(Start { op, origin });
    if confirm::asks(state, guard) {
        state.confirm(
            confirm::State::new(title, body, action)
                .cancel_message(format!("{what} cancelled, no operation ran")),
        );
        Vec::new()
    } else {
        crate::update::run(state, action)
    }
}

/// Starts a job, checking again that one may start.
pub fn start(state: &mut AppState, start: Start) -> Vec<Cmd> {
    if let Err(reason) = can_start(state) {
        state.say(Level::Error, format!("Not started: {reason}"));
        return Vec::new();
    }
    let token = state.request();
    state.jobs.dll = Some(token);
    let label = match &start.op {
        DllOp::Install { .. } => "Installing DLL...",
        DllOp::Restore { .. } => "Restoring DLLs...",
        DllOp::Update { .. } | DllOp::AutoUpdate => "Updating DLLs...",
    };
    match start.origin {
        Origin::Game(id) => {
            state.dll_jobs.results.remove(&id);
        }
        Origin::Catalog => state.dll_jobs.catalog_result = None,
        Origin::Batch { .. } | Origin::Auto => {}
    }
    // A Catalog or Library batch changes other games' DLLs: their last
    // per-game result no longer describes the files.
    if !matches!(start.origin, Origin::Game(_))
        && let DllOp::Update { requests } = &start.op
    {
        for request in requests {
            state.dll_jobs.results.remove(&request.app_id);
        }
    }
    state.dll_jobs.running = Some(Running {
        token,
        origin: start.origin,
        label,
        progress: None,
    });
    state.dirty = true;
    vec![Cmd::Dll {
        token,
        op: start.op,
    }]
}

/// The startup auto-update: now, or after the startup scan when one runs.
pub fn start_auto(state: &mut AppState) -> Vec<Cmd> {
    if state.jobs.scan.is_some() {
        state.dll_jobs.auto_pending = true;
        return Vec::new();
    }
    start(
        state,
        Start {
            op: DllOp::AutoUpdate,
            origin: Origin::Auto,
        },
    )
}

/// After a scan answered: the auto-update that waited for it starts.
pub fn scan_finished(state: &mut AppState) -> Vec<Cmd> {
    if !std::mem::take(&mut state.dll_jobs.auto_pending) {
        return Vec::new();
    }
    start_auto(state)
}

/// The words for where a job is.
pub fn words(running: &Running) -> String {
    let Some(e) = &running.progress else {
        return running.label.to_owned();
    };
    let what = match (e.dll_type, e.version.is_empty()) {
        (Some(t), false) => format!("{} {}", family(t).label, e.version),
        (Some(t), true) => family(t).label.to_owned(),
        (None, _) => "DLLs".to_owned(),
    };
    match e.stage {
        Stage::Resolving => running.label.to_owned(),
        Stage::Downloading => format!("Downloading {what}..."),
        Stage::Installing => format!("Installing {what}..."),
        Stage::Updating => format!("Updating {what}..."),
        Stage::Restoring => "Restoring DLLs...".to_owned(),
        Stage::Scanning => "Scanning the game folder...".to_owned(),
        Stage::Saving => "Saving the game list...".to_owned(),
    }
}

/// The status line's work words while a job runs.
pub fn work_words(state: &AppState) -> Option<String> {
    if state.dll_jobs.stopping {
        return Some(QUITTING.to_owned());
    }
    state.dll_jobs.running.as_ref().map(words)
}

/// A job moved on. Only a change of words draws.
pub fn progress(state: &mut AppState, token: u64, event: ProgressEvent) {
    let Some(running) = state.dll_jobs.running.as_mut().filter(|r| r.token == token) else {
        return;
    };
    let before = words(running);
    running.progress = Some(event);
    if words(running) != before {
        state.dirty = true;
    }
}

/// Go's batch line: `DLL update: U updated, N current, F failed`, then
/// each failure with its path.
pub fn batch_text(prefix: &str, batch: &BatchResult) -> String {
    let mut text = format!(
        "{prefix}: {} updated, {} current, {} failed",
        batch.updated, batch.unchanged, batch.failed
    );
    for item in &batch.items {
        if let Err(e) = &item.result {
            text.push_str(&format!("; {}: {e}", item.request.path.display()));
        }
    }
    text
}

/// The status line's words for a finished job, and the line its DLLs
/// section or the Catalog keeps.
fn result_words(origin: &Origin, result: &DllResult) -> Option<((Level, String), Option<String>)> {
    let out = match (origin, result) {
        (_, DllResult::Failed(e)) => (Level::Error, format!("DLL operation failed: {e}")),
        (_, DllResult::Installed(Ok(i))) if i.result.outcome == Outcome::NoOp => {
            return Some((
                (Level::Info, "Install: already current".into()),
                Some("DLL install: already current".into()),
            ));
        }
        (_, DllResult::Installed(Ok(_))) => {
            return Some((
                (Level::Ok, "DLL installed successfully!".into()),
                Some("DLL install completed".into()),
            ));
        }
        (_, DllResult::Installed(Err(e))) => {
            return Some((
                (Level::Error, format!("Install failed: {e}")),
                Some(format!("DLL install failed: {e}")),
            ));
        }
        (_, DllResult::Restored(Ok(r))) if !r.kept.is_empty() => {
            // A file spela created that Steam now ships stays, as the game's.
            let kept = format!("kept {} file(s) Steam replaced", r.kept.len());
            return Some((
                (Level::Ok, format!("Original DLLs restored! {kept}")),
                Some(format!("DLL restore completed; {kept}")),
            ));
        }
        (_, DllResult::Restored(Ok(_))) => {
            return Some((
                (Level::Ok, "Original DLLs restored!".into()),
                Some("DLL restore completed".into()),
            ));
        }
        (_, DllResult::Restored(Err(e))) => {
            return Some((
                (Level::Error, format!("Restore failed: {e}")),
                Some(format!("DLL restore failed: {e}")),
            ));
        }
        (_, DllResult::Updated(Err(e))) => (Level::Error, format!("Update failed: {e}")),
        (Origin::Catalog, DllResult::Updated(Ok(b))) => {
            if b.failed > 0 {
                (Level::Error, batch_text("Update-all", b))
            } else if b.updated == 0 {
                (Level::Info, "Update-all: already current".into())
            } else {
                (Level::Ok, batch_text("Update-all", b))
            }
        }
        (Origin::Batch { requested }, DllResult::Updated(Ok(b))) => {
            if b.failed > 0 {
                (Level::Error, batch_text("DLL update", b))
            } else if b.updated == 0 {
                (Level::Info, "Selected games are already current".into())
            } else {
                let games: BTreeSet<u64> = b
                    .items
                    .iter()
                    .filter(|i| matches!(&i.result, Ok(r) if r.outcome == Outcome::Changed))
                    .map(|i| i.request.app_id)
                    .collect();
                (
                    Level::Ok,
                    format!("Updated DLLs for {}/{requested} games", games.len()),
                )
            }
        }
        (_, DllResult::Updated(Ok(b))) => {
            if b.failed > 0 {
                (Level::Error, batch_text("DLL update", b))
            } else if b.updated > 0 {
                (Level::Ok, batch_text("DLL update", b))
            } else {
                (Level::Info, "DLLs already up to date".into())
            }
        }
        (_, DllResult::Auto(a)) => {
            let notice = a.notice();
            if notice.is_empty() {
                return None;
            }
            let level = if a.error.is_some() || a.batch.failed > 0 {
                Level::Error
            } else if a.batch.updated == 0 {
                Level::Info
            } else {
                Level::Ok
            };
            (level, notice)
        }
        (_, DllResult::BackupChanged(_)) => return None,
    };
    let line = out.1.clone();
    Some((out, Some(line)))
}

/// Drops the per-game result of every game a Catalog, Library or startup
/// batch went through, so no DLLs section names an earlier operation that
/// no longer describes its files.
fn forget_touched(state: &mut AppState, origin: &Origin, result: &DllResult) {
    if matches!(origin, Origin::Game(_)) {
        return;
    }
    let batch = match result {
        DllResult::Updated(Ok(b)) => b,
        DllResult::Auto(a) => &a.batch,
        _ => return,
    };
    for item in &batch.items {
        state.dll_jobs.results.remove(&item.request.app_id);
    }
}

/// A job finished. A stale answer is dropped. The game cache it sends
/// replaces the shown one, the backups and the Catalog are read again, and
/// a quit that waited for it goes ahead.
pub fn done(state: &mut AppState, token: u64, done: DllDone) -> Vec<Cmd> {
    let Some(running) = state.dll_jobs.running.take_if(|r| r.token == token) else {
        return Vec::new();
    };
    state.jobs.dll = None;
    state.dirty = true;
    let mut cmds = Vec::new();
    if let Some(games) = done.games {
        library::replace_games(state, games);
        cmds.extend(profile::follow_selection(state));
    }
    forget_touched(state, &running.origin, &done.result);
    if let (Origin::Game(app_id), DllResult::BackupChanged(changed)) =
        (&running.origin, &done.result)
    {
        reopen_restore(state, *app_id, changed.clone());
    } else if let Some(((level, text), line)) = result_words(&running.origin, &done.result) {
        state.say(level, text);
        if let Some(line) = line {
            match running.origin {
                Origin::Game(id) => {
                    state.dll_jobs.results.insert(id, (line, level));
                }
                Origin::Catalog => state.dll_jobs.catalog_result = Some((line, level)),
                Origin::Batch { .. } | Origin::Auto => {}
            }
        }
    }
    state.dll_jobs.backups.clear();
    cmds.extend(want_backup(state));
    cmds.extend(dlls::reload(state));
    if state.dll_jobs.stopping {
        state.quit = true;
        cmds.push(Cmd::Quit);
    }
    cmds
}

fn reopen_restore(state: &mut AppState, app_id: u64, changed: Result<Vec<RestoreTarget>, String>) {
    let targets = match changed {
        Err(e) => {
            state.say(Level::Error, format!("Cannot read DLL backup: {e}"));
            return;
        }
        Ok(t) if t.is_empty() => {
            state.say(Level::Error, "DLL restore unavailable: no backup files");
            return;
        }
        Ok(t) => t,
    };
    let Some(game) = state.games.get(app_id).cloned() else {
        return;
    };
    let items = restore_items(state, &game, &targets);
    let _ = ask(
        state,
        confirm::Guard::DllRestore,
        "Backup changed: review restore",
        body(&items, RESTORE_BACKUP),
        "DLL restore",
        DllOp::Restore {
            app_id,
            expected: targets,
        },
        Origin::Game(app_id),
    );
}

/// The restore's backup line, Go's.
pub const RESTORE_BACKUP: &str = "existing backup is restored; current DLL is not backed up again";

/// A stop signal, a closed terminal, or Ctrl-C while a job runs: the job
/// finishes its current step first. Otherwise, and the second time, quit.
pub fn stop(state: &mut AppState) -> Vec<Cmd> {
    if state.jobs.dll.is_some() && !state.dll_jobs.stopping {
        state.dll_jobs.stopping = true;
        state.say(Level::Info, "Finishing the current DLL step, then quitting");
        return vec![Cmd::StopDll];
    }
    state.quit = true;
    vec![Cmd::Quit]
}

/// The game whose DLLs section shows, if any.
fn shown_game(state: &AppState) -> Option<u64> {
    match library::selected_scope(state)? {
        Scope::Game(id) if profile::aspect(state) == profile::Aspect::Dlls => Some(id),
        _ => None,
    }
}

/// Reads the shown game's backup on a worker, unless it is read already.
pub fn want_backup(state: &mut AppState) -> Vec<Cmd> {
    let Some(app_id) = shown_game(state) else {
        return Vec::new();
    };
    if state.dll_jobs.backups.contains_key(&app_id) {
        return Vec::new();
    }
    let request = state.request();
    state
        .dll_jobs
        .backups
        .insert(app_id, Backup::Loading(request));
    vec![Cmd::Dlls(dlls::Cmd::Backup { request, app_id })]
}

/// A backup read answered. A stale one is dropped.
pub fn backup_read(
    state: &mut AppState,
    request: u64,
    app_id: u64,
    result: Result<Vec<RestoreTarget>, String>,
) {
    if state.dll_jobs.backups.get(&app_id) == Some(&Backup::Loading(request)) {
        state.dll_jobs.backups.insert(app_id, Backup::Ready(result));
        state.dirty = true;
    }
}

/// A game's restore targets, once read.
pub fn backup(state: &AppState, app_id: u64) -> Option<&Result<Vec<RestoreTarget>, String>> {
    match state.dll_jobs.backups.get(&app_id)? {
        Backup::Ready(r) => Some(r),
        Backup::Loading(_) => None,
    }
}
