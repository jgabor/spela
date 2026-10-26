//! DLL operations: install, update, batch update and restore. Go's
//! `internal/dll/operations.go`, with its contract (parity inventory DLL
//! 2.6 and `docs/design/dll-operations.md`): the domain resolves, acquires,
//! backs up, replaces, rescans and saves, emits progress with Go's stages,
//! and returns a typed partial failure. Adapters own confirmation and
//! rendering.
//!
//! What changes from Go:
//!
//! - The payload is downloaded into the checksum-verified DLL cache before
//!   `games.lock` is taken. The lock covers only the backup, the replace,
//!   the rescan and the save, so a slow download never holds up a scan
//!   (architecture, Locks). Under the lock the game is read fresh and its
//!   DLL path checked again.
//! - The deny list guard runs before the download and again under the lock,
//!   and a list that does not load refuses the swap (fails closed).
//! - The latest version is the highest, not the first entry (decision 8).
//! - An update writes only a payload whose file name is the detected file's
//!   (any case), so an FSR dx12 file never gets the vk payload. An install
//!   writes over a detected file of the same name in any case instead of
//!   making a second file.
//! - Every write is checked to stay inside the game's install folder, by
//!   path and by real folder, and a DLL that is a symlink is refused.
//!   Restore checks only the real folder, because Go recorded each original
//!   path by whatever route reached the game, and replaces a symlink at the
//!   target with the file, as Go's rename did.
//! - Install records what it wrote and what it created in the backup index
//!   (decision 37). Restore deletes the created files and forgets both.
//!
//! The error texts are Go's where Go had one.

use std::fmt;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};

use super::backup::{self, clean};
use super::detect;
use super::fetch::{self, AcquireError};
use super::manifest::{Manifest, ManifestEntry};
use super::swap::{self, Prepared};
use super::version;
use crate::backend::http::Http;
use crate::clock::Clock;
use crate::config::Config;
use crate::denylist;
use crate::games::{DetectedDll, DllType, Game, GameCache, cache};
use crate::paths::Paths;
use crate::pe;

/// What an operation needs from outside.
#[derive(Clone, Copy)]
pub struct DllContext<'a> {
    /// spela's directories.
    pub paths: &'a Paths,
    /// The config, for the DLL cache and backup folders.
    pub config: &'a Config,
    /// The network.
    pub http: &'a dyn Http,
    /// The clock, for scan and backup times.
    pub clock: &'a dyn Clock,
    /// Set when spela is quitting: a download stops, and a batch stops
    /// between items and saves what it already changed. `None` never stops.
    pub stop: Option<&'a AtomicBool>,
}

impl DllContext<'_> {
    /// Whether the operation should stop at the next safe point.
    pub fn stopped(&self) -> bool {
        self.stop.is_some_and(|s| s.load(Ordering::SeqCst))
    }
}

impl fmt::Debug for DllContext<'_> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("DllContext")
            .field("paths", &self.paths)
            .finish_non_exhaustive()
    }
}

/// Where an operation is, Go's stages.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Stage {
    /// Picking the manifest entry.
    Resolving,
    /// Getting the payload into the DLL cache.
    Downloading,
    /// Writing a new DLL.
    Installing,
    /// Writing over a detected DLL.
    Updating,
    /// Putting the originals back.
    Restoring,
    /// Reading the game folder again.
    Scanning,
    /// Writing the game cache.
    Saving,
}

impl Stage {
    /// Go's word for the stage.
    pub fn as_str(self) -> &'static str {
        match self {
            Stage::Resolving => "resolving",
            Stage::Downloading => "downloading",
            Stage::Installing => "installing",
            Stage::Updating => "updating",
            Stage::Restoring => "restoring",
            Stage::Scanning => "scanning",
            Stage::Saving => "saving",
        }
    }
}

impl fmt::Display for Stage {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// One progress event. Events describe work, not success.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProgressEvent {
    /// The stage.
    pub stage: Stage,
    /// The game.
    pub app_id: u64,
    /// The family, when the stage has one.
    pub dll_type: Option<DllType>,
    /// The version being installed, when known.
    pub version: String,
    /// The file name being written, when known.
    pub filename: String,
    /// Bytes downloaded so far.
    pub downloaded: u64,
    /// The download's size, when the server sent it.
    pub total: Option<u64>,
}

impl ProgressEvent {
    fn new(stage: Stage, app_id: u64) -> ProgressEvent {
        ProgressEvent {
            stage,
            app_id,
            dll_type: None,
            version: String::new(),
            filename: String::new(),
            downloaded: 0,
            total: None,
        }
    }
}

/// The progress callback.
pub type Progress<'a> = &'a mut dyn FnMut(&ProgressEvent);

/// Whether an operation changed files.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Outcome {
    /// Files changed.
    Changed,
    /// Nothing to do: the installed version is current.
    NoOp,
}

/// What an operation did.
#[derive(Debug, Clone, PartialEq)]
pub struct OpResult {
    /// Changed or no-op.
    pub outcome: Outcome,
    /// Whether a game file changed.
    pub files_changed: bool,
    /// Whether the game cache holds the result.
    pub metadata_persisted: bool,
    /// The game as rescanned, or as it was for a no-op.
    pub game: Option<Game>,
}

impl OpResult {
    fn changed() -> OpResult {
        OpResult {
            outcome: Outcome::Changed,
            files_changed: true,
            metadata_persisted: false,
            game: None,
        }
    }
}

/// What kind of failure, for a `--json` code.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FailKind {
    /// The game is not in the game cache.
    GameNotFound,
    /// The game has no such DLL, or no backup.
    NotFound,
    /// The request or the manifest entry is not usable.
    Invalid,
    /// The deny list refuses the swap, or does not load.
    Denied,
    /// The payload could not be had.
    Download,
    /// A backup could not be made or read.
    Backup,
    /// A game file or the lock failed.
    Write,
    /// spela is quitting, so the operation stopped before it changed
    /// anything.
    Stopped,
}

/// The message of an operation that stopped because spela is quitting.
pub const STOPPED: &str = "stopped because spela is quitting";

fn stopped() -> OpError {
    OpError::failed(FailKind::Stopped, STOPPED)
}

/// Why an operation failed.
#[derive(Debug, Clone, PartialEq, thiserror::Error)]
pub enum OpError {
    /// Nothing changed.
    #[error("{message}")]
    Failed {
        /// What kind.
        kind: FailKind,
        /// Go's text where Go had one.
        message: String,
    },
    /// Files changed but the game cache or backup index does not say so.
    #[error("DLL files changed but metadata did not fully persist during {stage}: {message}")]
    Partial {
        /// Where it stopped.
        stage: Stage,
        /// Why.
        message: String,
        /// What was done.
        result: Box<OpResult>,
    },
}

impl OpError {
    fn failed(kind: FailKind, message: impl Into<String>) -> OpError {
        OpError::Failed {
            kind,
            message: message.into(),
        }
    }

    fn partial(stage: Stage, message: impl Into<String>, result: OpResult) -> OpError {
        OpError::Partial {
            stage,
            message: message.into(),
            result: Box::new(result),
        }
    }

    /// Whether files changed.
    pub fn files_changed(&self) -> bool {
        matches!(self, OpError::Partial { .. })
    }
}

fn game_not_found(app_id: u64) -> OpError {
    OpError::failed(FailKind::GameNotFound, format!("game not found: {app_id}"))
}

/// Picks the manifest entry: the latest for `None` or `latest`, else the
/// exact version.
pub fn resolve<'m>(
    manifest: &'m Manifest,
    dll_type: DllType,
    version: Option<&str>,
) -> Result<&'m ManifestEntry, OpError> {
    let found = match version.filter(|v| !v.is_empty() && *v != "latest") {
        None => manifest.latest(dll_type),
        Some(v) => manifest.entries(dll_type).iter().find(|e| e.version == v),
    };
    found.ok_or_else(|| {
        let message = match version.filter(|v| !v.is_empty() && *v != "latest") {
            Some(v) => format!("no {dll_type} version {v:?} available in manifest"),
            None => format!("the DLL manifest lists no {dll_type} versions"),
        };
        OpError::failed(FailKind::Invalid, message)
    })
}

fn guard(paths: &Paths, app_id: u64) -> Result<(), OpError> {
    denylist::guard_swap(paths, app_id)
        .map_err(|e| OpError::failed(FailKind::Denied, e.to_string()))
}

fn acquire(
    cx: &DllContext<'_>,
    app_id: u64,
    dll_type: DllType,
    entry: &ManifestEntry,
    allow_download: bool,
    progress: Progress<'_>,
) -> Result<PathBuf, OpError> {
    let event = |downloaded, total| ProgressEvent {
        dll_type: Some(dll_type),
        version: entry.version.clone(),
        downloaded,
        total,
        ..ProgressEvent::new(Stage::Downloading, app_id)
    };
    progress(&event(0, None));
    fetch::acquire(
        cx.http,
        &cx.config.dll_cache_dir(cx.paths),
        dll_type,
        entry,
        allow_download,
        cx.stop,
        &mut |n, t| progress(&event(n, t)),
    )
    .map_err(|e: AcquireError| match e {
        AcquireError::Stopped => stopped(),
        e => OpError::failed(
            FailKind::Download,
            format!("acquire {dll_type} {}: {e}", entry.version),
        ),
    })
}

/// A detected DLL of `game` at exactly `path`, of `dll_type`.
fn detected<'g>(
    game: &'g Game,
    dll_type: DllType,
    path: &Path,
) -> Result<&'g DetectedDll, OpError> {
    let want = clean(path);
    let found = game
        .dlls
        .iter()
        .find(|d| d.dll_type == dll_type && clean(&d.path) == want)
        .ok_or_else(|| {
            OpError::failed(
                FailKind::NotFound,
                format!("detected {dll_type} DLL path not found: {}", path.display()),
            )
        })?;
    swap::check_inside(&game.install_dir, &found.path)
        .map_err(|m| OpError::failed(FailKind::Invalid, m))?;
    Ok(found)
}

/// Where an install writes `filename`: the detected DLL of that name, in
/// any case, or a new file at the top of the install folder.
fn install_target(game: &Game, filename: &str) -> Result<(PathBuf, Option<String>), OpError> {
    let mut matches = game
        .dlls
        .iter()
        .filter(|d| d.name.eq_ignore_ascii_case(filename));
    let target = match (matches.next(), matches.next()) {
        (Some(_), Some(_)) => {
            return Err(OpError::failed(
                FailKind::Invalid,
                format!("multiple DLLs named {filename} found; an exact path is required"),
            ));
        }
        (Some(d), None) => (d.path.clone(), d.version.clone()),
        (None, _) => (game.install_dir.join(filename), None),
    };
    swap::check_inside(&game.install_dir, &target.0)
        .map_err(|m| OpError::failed(FailKind::Invalid, m))?;
    Ok(target)
}

/// Rescans a game's install folder into `game`.
fn rescan(cx: &DllContext<'_>, game: &mut Game, progress: Progress<'_>) -> Result<(), String> {
    progress(&ProgressEvent::new(Stage::Scanning, game.app_id));
    let found = detect::scan_directory(&game.install_dir, pe::version_string)
        .map_err(|e| format!("scan install directory: {e}"))?;
    game.dlls = found;
    game.scanned_at = cx.clock.now();
    Ok(())
}

/// Backs up the game's DLLs, records the install at `target` and writes
/// the index, before the game's file is touched.
fn back_up_for(
    cx: &DllContext<'_>,
    game: &Game,
    extra: Option<DetectedDll>,
    target: &Path,
    sha256: &str,
    created: bool,
) -> Result<(Prepared, Option<backup::BackupIndex>), OpError> {
    let mut dlls = game.dlls.clone();
    dlls.extend(extra);
    let before = backup::load(cx.paths, game.app_id).ok().flatten();
    let mut prepared = swap::prepare_backup(
        cx.paths,
        &cx.config.backup_files_dir(cx.paths),
        game.app_id,
        &game.name,
        &dlls,
        cx.clock.now(),
    )
    .map_err(|e| OpError::failed(FailKind::Backup, e.to_string()))?;
    swap::record_install(&mut prepared.index, target, sha256, created);
    prepared
        .commit(cx.paths)
        .map_err(|e| OpError::failed(FailKind::Backup, e.to_string()))?;
    Ok((prepared, before))
}

/// Puts the index back as it was before a swap that failed to write.
fn undo_record(
    cx: &DllContext<'_>,
    prepared: &Prepared,
    before: Option<backup::BackupIndex>,
    target: &Path,
) {
    let mut index = prepared.index.clone();
    match before {
        Some(old) => {
            index.installed = old.installed;
            index.created_files = old.created_files;
        }
        None => {
            index.installed.retain(|p, _| clean(p) != clean(target));
            index.created_files.retain(|p| clean(p) != clean(target));
        }
    }
    if let Err(e) = swap::save_index(cx.paths, &index) {
        tracing::warn!("cannot undo the install record in the backup index: {e}");
    }
}

/// What an install did.
#[derive(Debug, Clone, PartialEq)]
pub struct Installed {
    /// The result.
    pub result: OpResult,
    /// The file written.
    pub path: PathBuf,
    /// The version it had before, when known.
    pub from_version: Option<String>,
    /// The version installed.
    pub version: String,
}

/// Installs `version` of `dll_type` into a game: over the detected DLL of
/// the entry's file name, or as a new file in the install folder. Any
/// version, older ones included.
pub fn install(
    cx: &DllContext<'_>,
    manifest: &Manifest,
    app_id: u64,
    dll_type: DllType,
    version: Option<&str>,
    progress: Progress<'_>,
) -> Result<Installed, OpError> {
    let snapshot = cache::load(cx.paths);
    let game = snapshot.get(app_id).ok_or_else(|| game_not_found(app_id))?;
    progress(&ProgressEvent {
        dll_type: Some(dll_type),
        version: version.unwrap_or_default().to_owned(),
        ..ProgressEvent::new(Stage::Resolving, app_id)
    });
    let entry = resolve(manifest, dll_type, version)?;
    entry
        .check(dll_type)
        .map_err(|e| OpError::failed(FailKind::Invalid, e.to_string()))?;
    install_target(game, &entry.filename)?;
    guard(cx.paths, app_id)?;
    let payload = acquire(cx, app_id, dll_type, entry, true, progress)?;
    if cx.stopped() {
        return Err(stopped());
    }

    let _lock =
        cache::lock(cx.paths).map_err(|e| OpError::failed(FailKind::Write, e.to_string()))?;
    let mut fresh = cache::load(cx.paths);
    let game = fresh
        .get(app_id)
        .ok_or_else(|| game_not_found(app_id))?
        .clone();
    let (target, from_version) = install_target(&game, &entry.filename)?;
    swap::check_target(&target).map_err(|m| OpError::failed(FailKind::Invalid, m))?;
    guard(cx.paths, app_id)?;
    progress(&ProgressEvent {
        dll_type: Some(dll_type),
        version: entry.version.clone(),
        filename: entry.filename.clone(),
        ..ProgressEvent::new(Stage::Installing, app_id)
    });
    let created = !target.exists();
    let undetected = (!created && !game.dlls.iter().any(|d| clean(&d.path) == clean(&target)))
        .then(|| DetectedDll {
            path: target.clone(),
            name: target
                .file_name()
                .map(|n| n.to_string_lossy().into_owned())
                .unwrap_or_default(),
            dll_type,
            version: None,
        });
    let (prepared, before) = back_up_for(cx, &game, undetected, &target, &entry.sha256, created)?;
    if let Err(e) = swap::replace_file(&payload, &target) {
        undo_record(cx, &prepared, before, &target);
        return Err(OpError::failed(
            FailKind::Write,
            format!("install DLL: failed to install DLL: {e}"),
        ));
    }
    let result = finish_one(cx, &mut fresh, game, progress)?;
    Ok(Installed {
        result,
        path: target,
        from_version,
        version: entry.version.clone(),
    })
}

/// Rescans and saves after one changed game, Go's `refresh` and
/// `finalize`.
fn finish_one(
    cx: &DllContext<'_>,
    fresh: &mut GameCache,
    mut game: Game,
    progress: Progress<'_>,
) -> Result<OpResult, OpError> {
    let app_id = game.app_id;
    let mut result = OpResult::changed();
    if let Err(e) = rescan(cx, &mut game, progress) {
        result.game = Some(game);
        return Err(OpError::partial(Stage::Scanning, e, result));
    }
    result.game = Some(game.clone());
    fresh.upsert(game);
    progress(&ProgressEvent::new(Stage::Saving, app_id));
    if let Err(e) = cache::save(cx.paths, fresh) {
        return Err(OpError::partial(
            Stage::Saving,
            format!("save game database: {e}"),
            result,
        ));
    }
    result.metadata_persisted = true;
    Ok(result)
}

/// One DLL to update: an exact detected path (Go's `UpdateRequest`).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct UpdateRequest {
    /// The game.
    pub app_id: u64,
    /// The family.
    pub dll_type: DllType,
    /// The detected DLL's path. Never a base name.
    pub path: PathBuf,
    /// The version to update to; `None` for the latest.
    pub version: Option<String>,
    /// Use only the DLL cache: no download. Needs a version.
    pub cached_only: bool,
}

/// One request's outcome in a batch.
#[derive(Debug, Clone, PartialEq)]
pub struct BatchItem {
    /// The request.
    pub request: UpdateRequest,
    /// The game's name, when the game is known.
    pub game_name: String,
    /// The installed version before, when known.
    pub from_version: Option<String>,
    /// The version the manifest offered, once resolved.
    pub to_version: Option<String>,
    /// What happened.
    pub result: Result<OpResult, OpError>,
}

/// A batch's items and counts. An item counts as updated only when its
/// files changed and the game cache saved.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct BatchResult {
    /// One per request, in order.
    pub items: Vec<BatchItem>,
    /// Items that changed files and saved.
    pub updated: usize,
    /// Items that were current already.
    pub unchanged: usize,
    /// Items that failed, partial failures included.
    pub failed: usize,
}

/// The requests for every detected DLL of `app_ids`, of one family or of
/// all, from the game cache as it is (Go's `UpdateGames`). An unknown game
/// gives a request that fails with "game not found".
pub fn requests_for(
    cache: &GameCache,
    app_ids: &[u64],
    dll_type: Option<DllType>,
    version: Option<&str>,
    cached_only: bool,
) -> Vec<UpdateRequest> {
    let mut out = Vec::new();
    for &app_id in app_ids {
        let request = |dll_type, path: PathBuf| UpdateRequest {
            app_id,
            dll_type,
            path,
            version: version.map(str::to_owned),
            cached_only,
        };
        match cache.get(app_id) {
            None => out.push(request(dll_type.unwrap_or(DllType::Dlss), PathBuf::new())),
            Some(game) => out.extend(
                game.dlls
                    .iter()
                    .filter(|d| dll_type.is_none_or(|t| d.dll_type == t))
                    .map(|d| request(d.dll_type, d.path.clone())),
            ),
        }
    }
    out
}

/// A request that passed the checks and has its payload.
struct Ready {
    index: usize,
    payload: PathBuf,
    sha256: String,
}

/// Updates each request independently and carries on after a failure.
/// Every payload is acquired first, outside the lock; then one hold of
/// `games.lock` backs up, swaps and rescans each game, and the cache is
/// saved once. When spela is quitting ([`DllContext::stop`]), the items not
/// started yet fail with [`STOPPED`], and what already changed is saved.
pub fn update(
    cx: &DllContext<'_>,
    manifest: &Manifest,
    requests: Vec<UpdateRequest>,
    progress: Progress<'_>,
) -> BatchResult {
    let snapshot = cache::load(cx.paths);
    let mut items = Vec::with_capacity(requests.len());
    let mut ready = Vec::new();
    for request in requests {
        let mut item = BatchItem {
            game_name: snapshot
                .get(request.app_id)
                .map(|g| g.name.clone())
                .unwrap_or_default(),
            from_version: None,
            to_version: None,
            result: Ok(OpResult::changed()),
            request,
        };
        if cx.stopped() {
            item.result = Err(stopped());
            items.push(item);
            continue;
        }
        match prepare_update(cx, manifest, &snapshot, &mut item, progress) {
            Ok(Some((payload, sha256))) => ready.push(Ready {
                index: items.len(),
                payload,
                sha256,
            }),
            Ok(None) => {}
            Err(e) => item.result = Err(e),
        }
        items.push(item);
    }
    if !ready.is_empty() {
        swap_all(cx, &mut items, ready, progress);
    }
    let mut batch = BatchResult {
        items,
        ..BatchResult::default()
    };
    for item in &batch.items {
        match &item.result {
            Err(_) => batch.failed += 1,
            Ok(r) if r.outcome == Outcome::NoOp => batch.unchanged += 1,
            Ok(_) => batch.updated += 1,
        }
    }
    batch
}

/// The checks before the lock. `Ok(None)` is a no-op, already in `item`.
fn prepare_update(
    cx: &DllContext<'_>,
    manifest: &Manifest,
    snapshot: &GameCache,
    item: &mut BatchItem,
    progress: Progress<'_>,
) -> Result<Option<(PathBuf, String)>, OpError> {
    let request = &item.request;
    let game = snapshot
        .get(request.app_id)
        .ok_or_else(|| game_not_found(request.app_id))?;
    let installed = detected(game, request.dll_type, &request.path)?;
    item.from_version = installed.version.clone();
    // `--cached-only` picks the newest cached version the manifest lists
    // (decision 67); none means nothing usable is cached.
    if request.cached_only && request.version.is_none() {
        return Err(OpError::failed(
            FailKind::Invalid,
            format!(
                "no cached {} version; run without --cached-only to download it",
                request.dll_type
            ),
        ));
    }
    progress(&ProgressEvent {
        dll_type: Some(request.dll_type),
        version: request.version.clone().unwrap_or_default(),
        ..ProgressEvent::new(Stage::Resolving, request.app_id)
    });
    let entry = resolve(manifest, request.dll_type, request.version.as_deref())?;
    item.to_version = Some(entry.version.clone());
    // Go's order: the entry and its file name, the no-op rule, then the
    // checksum (parity-inventory.md, DLL 2.6).
    entry
        .check_listing(request.dll_type)
        .map_err(|e| OpError::failed(FailKind::Invalid, e.to_string()))?;
    if !entry.filename.eq_ignore_ascii_case(&installed.name) {
        return Err(OpError::failed(
            FailKind::Invalid,
            format!(
                "the {} {} manifest file {} is not {}",
                request.dll_type, entry.version, entry.filename, installed.name
            ),
        ));
    }
    if let Some(current) = installed.version.as_deref()
        && !version::is_newer(current, &entry.version)
    {
        item.result = Ok(OpResult {
            outcome: Outcome::NoOp,
            files_changed: false,
            metadata_persisted: true,
            game: Some(game.clone()),
        });
        return Ok(None);
    }
    entry
        .check_checksum(request.dll_type)
        .map_err(|e| OpError::failed(FailKind::Invalid, e.to_string()))?;
    guard(cx.paths, request.app_id)?;
    let payload = acquire(
        cx,
        request.app_id,
        request.dll_type,
        entry,
        !request.cached_only,
        progress,
    )?;
    Ok(Some((payload, entry.sha256.clone())))
}

/// The locked part of a batch: back up, swap and rescan each ready item,
/// then save once.
fn swap_all(
    cx: &DllContext<'_>,
    items: &mut [BatchItem],
    ready: Vec<Ready>,
    progress: Progress<'_>,
) {
    let _lock = match cache::lock(cx.paths) {
        Ok(lock) => lock,
        Err(e) => {
            for r in &ready {
                items[r.index].result = Err(OpError::failed(FailKind::Write, e.to_string()));
            }
            return;
        }
    };
    let mut fresh = cache::load(cx.paths);
    let mut changed = Vec::new();
    for r in ready {
        let item = &mut items[r.index];
        if cx.stopped() {
            item.result = Err(stopped());
            continue;
        }
        match swap_one(cx, &mut fresh, item, &r, progress) {
            Ok(result) => {
                item.result = Ok(result);
                changed.push(r.index);
            }
            Err(e) => item.result = Err(e),
        }
    }
    if changed.is_empty() {
        return;
    }
    progress(&ProgressEvent::new(
        Stage::Saving,
        items[changed[0]].request.app_id,
    ));
    let saved = cache::save(cx.paths, &fresh);
    for index in changed {
        let item = &mut items[index];
        let Ok(result) = &mut item.result else {
            continue;
        };
        match &saved {
            Ok(()) => result.metadata_persisted = true,
            Err(e) => {
                let result = result.clone();
                item.result = Err(OpError::partial(
                    Stage::Saving,
                    format!("save game database: {e}"),
                    result,
                ));
            }
        }
    }
}

fn swap_one(
    cx: &DllContext<'_>,
    fresh: &mut GameCache,
    item: &BatchItem,
    ready: &Ready,
    progress: Progress<'_>,
) -> Result<OpResult, OpError> {
    let request = &item.request;
    let game = fresh
        .get(request.app_id)
        .ok_or_else(|| game_not_found(request.app_id))?
        .clone();
    let installed = detected(&game, request.dll_type, &request.path)?.clone();
    swap::check_target(&installed.path).map_err(|m| OpError::failed(FailKind::Invalid, m))?;
    guard(cx.paths, request.app_id)?;
    progress(&ProgressEvent {
        dll_type: Some(request.dll_type),
        version: item.to_version.clone().unwrap_or_default(),
        filename: installed.name.clone(),
        ..ProgressEvent::new(Stage::Updating, request.app_id)
    });
    let (prepared, before) = back_up_for(cx, &game, None, &installed.path, &ready.sha256, false)?;
    if let Err(e) = swap::replace_file(&ready.payload, &installed.path) {
        undo_record(cx, &prepared, before, &installed.path);
        return Err(OpError::failed(
            FailKind::Write,
            format!("swap {}: failed to swap DLL: {e}", request.dll_type),
        ));
    }
    let mut game = game;
    let mut result = OpResult::changed();
    if let Err(e) = rescan(cx, &mut game, progress) {
        result.game = Some(game);
        return Err(OpError::partial(Stage::Scanning, e, result));
    }
    result.game = Some(game.clone());
    fresh.upsert(game);
    Ok(result)
}

/// What a restore did.
#[derive(Debug, Clone, PartialEq)]
pub struct Restored {
    /// The result.
    pub result: OpResult,
    /// Originals put back.
    pub restored: Vec<PathBuf>,
    /// Files an install created, deleted.
    pub deleted: Vec<PathBuf>,
    /// Files an install created that Steam has since replaced with its own:
    /// kept, and no longer counted as spela's.
    pub kept: Vec<PathBuf>,
}

/// Puts a game's original DLLs back from its backup and deletes the DLLs
/// spela created. No deny list guard: restore puts the game's own files
/// back. The backup files and the index stay, as in Go, except that the
/// index forgets what spela installed at the restored paths, and an index
/// left with nothing to restore is removed.
pub fn restore(
    cx: &DllContext<'_>,
    app_id: u64,
    progress: Progress<'_>,
) -> Result<Restored, OpError> {
    let snapshot = cache::load(cx.paths);
    snapshot.get(app_id).ok_or_else(|| game_not_found(app_id))?;
    if cx.stopped() {
        return Err(stopped());
    }
    let _lock =
        cache::lock(cx.paths).map_err(|e| OpError::failed(FailKind::Write, e.to_string()))?;
    let mut fresh = cache::load(cx.paths);
    let game = fresh
        .get(app_id)
        .ok_or_else(|| game_not_found(app_id))?
        .clone();
    let plan = backup::plan_restore(cx.paths, app_id).map_err(|e| {
        let kind = match e {
            backup::RestoreError::NoBackup(_) => FailKind::NotFound,
            _ => FailKind::Backup,
        };
        OpError::failed(kind, format!("restore backup: {e}"))
    })?;
    for target in plan
        .copies
        .iter()
        .map(|c| &c.original_path)
        .chain(&plan.deletes)
    {
        let is_dir = std::fs::symlink_metadata(target).is_ok_and(|m| m.is_dir());
        swap::check_real_folder(&game.install_dir, target)
            .and_then(|()| {
                if is_dir {
                    Err(format!("{} is a directory", target.display()))
                } else {
                    Ok(())
                }
            })
            .map_err(|m| {
                OpError::failed(
                    FailKind::Invalid,
                    format!("restore backup: cannot restore {}: {m}", target.display()),
                )
            })?;
    }
    progress(&ProgressEvent::new(Stage::Restoring, app_id));
    let mut restored = Vec::new();
    let mut deleted = Vec::new();
    let partial_or_failed = |done: usize, message: String| {
        if done > 0 {
            OpError::partial(
                Stage::Restoring,
                format!("restore backup: {message}"),
                OpResult::changed(),
            )
        } else {
            OpError::failed(FailKind::Write, format!("restore backup: {message}"))
        }
    };
    for copy in &plan.copies {
        if let Err(e) = swap::restore_file(&copy.backup_path, &copy.original_path) {
            return Err(partial_or_failed(
                restored.len(),
                format!("failed to restore {}: {e}", copy.dll_name),
            ));
        }
        restored.push(copy.original_path.clone());
    }
    for path in &plan.deletes {
        if let Err(e) = std::fs::remove_file(path) {
            return Err(partial_or_failed(
                restored.len() + deleted.len(),
                format!("failed to remove {}: {e}", path.display()),
            ));
        }
        deleted.push(path.clone());
    }
    for path in &plan.kept {
        tracing::info!(
            path = %path.display(),
            "Steam ships this file now, so restore keeps it"
        );
    }
    let mut index = plan.index;
    // A kept file leaves the index as a deleted one does: it is the game's
    // own file now.
    let forgotten: Vec<PathBuf> = deleted.iter().chain(&plan.kept).cloned().collect();
    if swap::record_restore(&mut index, &restored, &forgotten) {
        let written = if index.files.is_empty() && index.created_files.is_empty() {
            swap::remove_index(cx.paths, app_id)
        } else {
            swap::save_index(cx.paths, &index)
        };
        if let Err(e) = written {
            return Err(OpError::partial(
                Stage::Restoring,
                format!("save backup metadata: {e}"),
                OpResult::changed(),
            ));
        }
    }
    let result = finish_one(cx, &mut fresh, game, progress)?;
    Ok(Restored {
        result,
        restored,
        deleted,
        kept: plan.kept,
    })
}
