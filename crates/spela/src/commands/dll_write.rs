//! The write side of `spela dll`: `install`, `update` and `restore`.
//!
//! Each command resolves its games from the game cache as it is, as Go did,
//! and hands the work to [`spela_core::dll::ops`]: the manifest under its
//! refresh policy, the checksum-verified download outside `games.lock`, the
//! deny list guard, the backup, the swap, the rescan and the save. The CLI
//! never prompts, as in Go.
//!
//! Text keeps Go's lines where Go had a command (`dll update <game> <type>`
//! and `dll restore`). The forms Go's CLI lacked (decision 20) print one
//! block per game. Download progress goes to stderr. With `--json` a run
//! prints one object; a run with a failed item exits 1 and prints the error
//! object with the same object as its `details`.

use std::collections::BTreeSet;
use std::fmt::Write as _;
use std::io::Write;
use std::path::{Path, PathBuf};

use clap::Args;
use serde::Serialize;
use spela_core::denylist;
use spela_core::dll::fetch::{self, ManifestSource};
use spela_core::dll::manifest::{self, Manifest, RefreshPolicy};
use spela_core::dll::ops::{
    self, BatchItem, BatchResult, DllContext, FailKind, OpError, Outcome, ProgressEvent, Stage,
};
use spela_core::games::{DllType, Game, GameCache};

use super::read::{self, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};
use crate::json::SCHEMA_VERSION;

/// Parses a DLL family name in any case, as Go lowercased it.
pub fn parse_dll_type(value: &str) -> Result<DllType, String> {
    DllType::from_name(&value.to_ascii_lowercase())
        .ok_or_else(|| format!("unknown DLL type {value:?} (use dlss, dlssg, dlssd, xess or fsr)"))
}

/// `dll install` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct InstallArgs {
    /// The game: a Steam AppID or its exact name.
    pub game: String,
    /// The DLL family: dlss, dlssg, dlssd, xess or fsr.
    #[arg(value_name = "TYPE", value_parser = parse_dll_type)]
    pub dll_type: DllType,
    /// The version to install, older ones included. Default: the latest.
    #[arg(long, value_name = "VERSION")]
    pub version: Option<String>,
}

/// `dll update` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct UpdateArgs {
    /// The game: a Steam AppID or its exact name. Not with --all.
    #[arg(required_unless_present = "all", conflicts_with = "all")]
    pub game: Option<String>,
    /// Only this DLL family of the game. Default: every family it has.
    #[arg(value_name = "TYPE", value_parser = parse_dll_type, conflicts_with = "type_filter")]
    pub dll_type: Option<DllType>,
    /// Update every game with a DLSS, XeSS or FSR DLL. Deny-listed games are
    /// skipped.
    #[arg(long)]
    pub all: bool,
    /// Only this DLL family, with --all.
    #[arg(long = "type", value_name = "TYPE", value_parser = parse_dll_type)]
    pub type_filter: Option<DllType>,
    /// The version to update to. Default: the latest. An update never goes
    /// to an older version; use 'spela dll install' for that.
    #[arg(long, value_name = "VERSION")]
    pub version: Option<String>,
    /// Use only DLLs already in the DLL cache: no network at all. Without
    /// --version, the newest cached version the manifest lists.
    #[arg(long)]
    pub cached_only: bool,
}

/// `dll restore` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct RestoreArgs {
    /// The game: a Steam AppID or its exact name.
    pub game: String,
}

/// The pieces an operation needs, owned for the run.
struct Run {
    config: spela_core::config::Config,
    http: std::sync::Arc<dyn spela_core::backend::Http>,
}

impl Run {
    fn new(ctx: &Context) -> Result<Run, CliError> {
        Ok(Run {
            config: read::config(ctx)?,
            http: ctx.backends.http.clone(),
        })
    }

    fn cx<'a>(&'a self, ctx: &'a Context) -> DllContext<'a> {
        DllContext {
            paths: &ctx.paths,
            config: &self.config,
            http: self.http.as_ref(),
            clock: ctx.clock.as_ref(),
        }
    }

    /// The manifest: the cache only for cached-only runs, else under the
    /// refresh policy. A fallback to the cache is a warning.
    fn manifest(&self, ctx: &Context, cached_only: bool) -> Result<ManifestSource, CliError> {
        let got = if cached_only {
            fetch::cached_manifest(&ctx.paths)
        } else {
            fetch::get_manifest(
                &ctx.paths,
                &RefreshPolicy::from_config(&self.config),
                self.http.as_ref(),
                ctx.clock.as_ref(),
                false,
            )
        };
        let source = got.map_err(|e| {
            let hint = if cached_only {
                "run an update without --cached-only once to fetch the manifest"
            } else {
                "check the network, or dll_manifest_url in 'spela config show'"
            };
            CliError::new(exit::FAILURE, e.to_string())
                .with_hint(hint)
                .with_kind("no_manifest")
        })?;
        if let Some(warning) = &source.warning {
            tracing::warn!("{warning}");
        }
        Ok(source)
    }
}

/// Prints `Downloading <type> <version>...` to stderr the first time bytes
/// arrive for a payload.
struct DownloadNotes<'e> {
    err: &'e mut dyn Write,
    seen: BTreeSet<(DllType, String)>,
}

impl DownloadNotes<'_> {
    fn on(&mut self, event: &ProgressEvent) {
        if event.stage != Stage::Downloading || event.downloaded == 0 {
            return;
        }
        let Some(dll_type) = event.dll_type else {
            return;
        };
        if self.seen.insert((dll_type, event.version.clone())) {
            let _ = writeln!(self.err, "Downloading {dll_type} {}...", event.version);
        }
    }
}

/// Runs `spela dll install`.
pub fn install(
    args: &InstallArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    let cache = read::games(ctx, Rescan::Never)?;
    let game = read::find_game(&cache, &args.game)?.clone();
    let run = Run::new(ctx)?;
    let source = run.manifest(ctx, false)?;
    let mut notes = DownloadNotes {
        err,
        seen: BTreeSet::new(),
    };
    let done = ops::install(
        &run.cx(ctx),
        source.manifest(),
        game.app_id,
        args.dll_type,
        args.version.as_deref(),
        &mut |e| notes.on(e),
    );
    let item = match &done {
        Ok(i) => ItemJson {
            app_id: game.app_id,
            name: game.name.clone(),
            dll_type: args.dll_type,
            path: Some(i.path.clone()),
            from_version: i.from_version.clone(),
            to_version: Some(i.version.clone()),
            outcome: "changed",
            error: None,
        },
        Err(e) => ItemJson {
            app_id: game.app_id,
            name: game.name.clone(),
            dll_type: args.dll_type,
            path: None,
            from_version: None,
            to_version: args.version.clone(),
            outcome: "failed",
            error: Some(ErrorJson::of(e)),
        },
    };
    let report = Report {
        schema_version: SCHEMA_VERSION,
        items: vec![item],
        skipped: Vec::new(),
        updated: usize::from(done.is_ok()),
        unchanged: 0,
        failed: usize::from(done.is_err()),
    };
    let installed = done.map_err(|e| op_error(&e).with_details(details(&report)))?;
    if inv.cli.json {
        return read::write_json(out, &report);
    }
    read::write_out(
        out,
        &format!(
            "Installed {} {} for {} at {}\n",
            args.dll_type,
            installed.version,
            game.name,
            installed.path.display()
        ),
    )
}

/// Runs `spela dll update`.
pub fn update(
    args: &UpdateArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    let dll_type = args.dll_type.or(args.type_filter);
    if args.version.is_some() && dll_type.is_none() {
        return Err(CliError::new(
            exit::USAGE,
            "--version needs a DLL type, such as 'spela dll update <game> dlss --version 310.5.3'",
        ));
    }
    let cache = read::games(ctx, Rescan::Never)?;
    let (games, skipped) = match &args.game {
        Some(query) => (vec![read::find_game(&cache, query)?.clone()], Vec::new()),
        None => all_games(ctx, &cache, dll_type),
    };
    let run = Run::new(ctx)?;
    let app_ids: Vec<u64> = games.iter().map(|g| g.app_id).collect();
    let mut requests = ops::requests_for(
        &cache,
        &app_ids,
        dll_type,
        args.version.as_deref(),
        args.cached_only,
    );
    if args.game.is_some() && requests.is_empty() {
        let what = dll_type.map_or_else(|| "DLSS, XeSS or FSR".to_owned(), |t| t.to_string());
        return Err(
            CliError::new(exit::FAILURE, format!("game does not have a {what} DLL"))
                .with_kind("not_found"),
        );
    }
    let batch = if requests.is_empty() {
        BatchResult::default()
    } else {
        let source = run.manifest(ctx, args.cached_only)?;
        if args.cached_only && args.version.is_none() {
            let dir = run.config.dll_cache_dir(&ctx.paths);
            for request in &mut requests {
                request.version = newest_cached(&dir, request.dll_type, source.manifest());
            }
        }
        let mut notes = DownloadNotes {
            err,
            seen: BTreeSet::new(),
        };
        ops::update(&run.cx(ctx), source.manifest(), requests, &mut |e| {
            notes.on(e)
        })
    };
    let report = Report::of(&batch, &games, skipped);
    let failed = (batch.failed > 0).then(|| batch_error(&batch).with_details(details(&report)));
    if inv.cli.json {
        return match failed {
            Some(e) => Err(e),
            None => read::write_json(out, &report),
        };
    }
    let text = match (&args.game, args.dll_type) {
        (Some(_), Some(t)) if !args.all => go_update_text(&batch, t),
        _ => block_text(&report),
    };
    read::write_out(out, &text)?;
    failed.map_or(Ok(()), Err)
}

/// The listed games with DLLs, of one family when given, and the ones the
/// deny list refuses, skipped with their reason. A deny list that does not
/// load skips nothing, so each item fails closed with its error.
fn all_games(
    ctx: &Context,
    cache: &GameCache,
    dll_type: Option<DllType>,
) -> (Vec<Game>, Vec<SkippedJson>) {
    let lists = denylist::load(&ctx.paths).ok();
    let mut games = Vec::new();
    let mut skipped = Vec::new();
    for game in read::games_with_dlls(cache) {
        if dll_type.is_some_and(|t| !game.has(t)) {
            continue;
        }
        if let Some(reason) = lists
            .as_ref()
            .and_then(|l| l.check(game.app_id).reason().map(str::to_owned))
        {
            skipped.push(SkippedJson {
                app_id: game.app_id,
                name: game.name.clone(),
                reason: format!("DLL swap denied: {reason}"),
            });
            continue;
        }
        games.push(game.clone());
    }
    (games, skipped)
}

/// The newest version of a family in the DLL cache that the manifest lists.
fn newest_cached(dir: &Path, dll_type: DllType, m: &Manifest) -> Option<String> {
    manifest::cached_versions(dir, dll_type)
        .ok()?
        .into_iter()
        .find(|v| m.entries(dll_type).iter().any(|e| &e.version == v))
}

/// Runs `spela dll restore`.
pub fn restore(
    args: &RestoreArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
) -> Result<(), CliError> {
    let cache = read::games(ctx, Rescan::Never)?;
    let game = read::find_game(&cache, &args.game)?.clone();
    let run = Run::new(ctx)?;
    let done = ops::restore(&run.cx(ctx), game.app_id, &mut |_| {}).map_err(|e| op_error(&e))?;
    if inv.cli.json {
        return read::write_json(
            out,
            &RestoreJson {
                schema_version: SCHEMA_VERSION,
                app_id: game.app_id,
                name: &game.name,
                restored: &done.restored,
                deleted: &done.deleted,
            },
        );
    }
    read::write_out(out, &format!("Restored original DLLs for {}\n", game.name))
}

/// An operation's error for the CLI, with a `--json` code.
fn op_error(e: &OpError) -> CliError {
    let kind = match e {
        OpError::Partial { .. } => "partial_failure",
        OpError::Failed { kind, .. } => match kind {
            FailKind::GameNotFound => "game_not_found",
            FailKind::NotFound => "not_found",
            FailKind::Invalid => "invalid",
            FailKind::Denied => "denied",
            FailKind::Download => "download_failed",
            FailKind::Backup => "backup_failed",
            FailKind::Write => "write_failed",
        },
    };
    CliError::new(exit::FAILURE, e.to_string()).with_kind(kind)
}

/// Go's error after a batch with failures.
fn batch_error(batch: &BatchResult) -> CliError {
    let joined: Vec<String> = batch
        .items
        .iter()
        .filter_map(|i| i.result.as_ref().err().map(ToString::to_string))
        .collect();
    CliError::new(
        exit::FAILURE,
        format!(
            "{} DLL update(s) failed: {}",
            batch.failed,
            joined.join("\n")
        ),
    )
    .with_kind("dll_update_failed")
}

/// Go's text for `dll update <game> <type>`.
fn go_update_text(batch: &BatchResult, dll_type: DllType) -> String {
    let mut text = String::new();
    for item in &batch.items {
        if let Err(e) = &item.result {
            let _ = writeln!(text, "Failed {}: {e}", item.request.path.display());
        }
    }
    let result_line = |t: &mut String| {
        let _ = writeln!(
            t,
            "DLL update result: {} updated, {} unchanged, {} failed",
            batch.updated, batch.unchanged, batch.failed
        );
    };
    if batch.failed > 0 {
        result_line(&mut text);
        return text;
    }
    let version = batch
        .items
        .iter()
        .find(|i| matches!(&i.result, Ok(r) if r.outcome == Outcome::Changed))
        .and_then(|i| i.to_version.clone())
        .unwrap_or_default();
    match batch.updated {
        0 => {
            let _ = writeln!(text, "{dll_type} DLLs are already at the latest version");
        }
        1 => {
            let _ = writeln!(text, "Updated {dll_type} DLL to version {version}");
        }
        n => {
            let _ = writeln!(text, "Updated {n} {dll_type} DLLs to version {version}");
        }
    }
    result_line(&mut text);
    text
}

/// One block per game: each DLL with what happened to it.
fn block_text(report: &Report) -> String {
    let mut text = String::new();
    let mut current: Option<u64> = None;
    for item in &report.items {
        if current != Some(item.app_id) {
            let _ = writeln!(text, "{} ({})", item.name, item.app_id);
            current = Some(item.app_id);
        }
        let name = item.path.as_deref().and_then(Path::file_name).map_or_else(
            || item.dll_type.to_string(),
            |n| n.to_string_lossy().into_owned(),
        );
        let from = read::version_text(item.from_version.as_deref());
        let to = item.to_version.as_deref().unwrap_or("?");
        let _ = match (&item.error, item.outcome) {
            (Some(e), _) => writeln!(text, "  {name}: failed: {}", e.message),
            (None, "unchanged") => writeln!(text, "  {name}: {from}, already the latest"),
            (None, _) => writeln!(text, "  {name}: {from} -> {to}"),
        };
    }
    for s in &report.skipped {
        let _ = writeln!(text, "Skipped {} ({}): {}", s.name, s.app_id, s.reason);
    }
    if report.items.is_empty() && report.skipped.is_empty() {
        text.push_str("No games with DLSS/FSR/XeSS DLLs found.\n");
    }
    let _ = writeln!(
        text,
        "DLL update result: {} updated, {} unchanged, {} failed",
        report.updated, report.unchanged, report.failed
    );
    text
}

fn details(report: &Report) -> serde_json::Value {
    serde_json::to_value(report).unwrap_or(serde_json::Value::Null)
}

/// `dll install --json` and `dll update --json`.
#[derive(Debug, Clone, Serialize)]
pub struct Report {
    /// [`SCHEMA_VERSION`].
    pub schema_version: u32,
    /// One per DLL, by game name, then catalogue order, then path.
    pub items: Vec<ItemJson>,
    /// Games `--all` left out.
    pub skipped: Vec<SkippedJson>,
    /// DLLs changed and saved.
    pub updated: usize,
    /// DLLs already current.
    pub unchanged: usize,
    /// DLLs that failed.
    pub failed: usize,
}

impl Report {
    fn of(batch: &BatchResult, games: &[Game], skipped: Vec<SkippedJson>) -> Report {
        let order = |app_id: u64| games.iter().position(|g| g.app_id == app_id);
        let mut items: Vec<(Option<usize>, &BatchItem)> = batch
            .items
            .iter()
            .map(|i| (order(i.request.app_id), i))
            .collect();
        items.sort_by(|(a, x), (b, y)| {
            a.cmp(b)
                .then(x.request.dll_type.cmp(&y.request.dll_type))
                .then(x.request.path.cmp(&y.request.path))
        });
        Report {
            schema_version: SCHEMA_VERSION,
            items: items.into_iter().map(|(_, i)| ItemJson::of(i)).collect(),
            skipped,
            updated: batch.updated,
            unchanged: batch.unchanged,
            failed: batch.failed,
        }
    }
}

/// One DLL in a report.
#[derive(Debug, Clone, Serialize)]
pub struct ItemJson {
    /// The game.
    pub app_id: u64,
    /// The game's name.
    pub name: String,
    /// The family.
    #[serde(rename = "type")]
    pub dll_type: DllType,
    /// The DLL's path, or `null` when the request never got one.
    pub path: Option<PathBuf>,
    /// The version before, or `null` when unknown.
    pub from_version: Option<String>,
    /// The version installed or offered, or `null` when not resolved.
    pub to_version: Option<String>,
    /// `changed`, `unchanged` or `failed`.
    pub outcome: &'static str,
    /// Why it failed, or `null`.
    pub error: Option<ErrorJson>,
}

impl ItemJson {
    fn of(item: &BatchItem) -> ItemJson {
        let (outcome, error) = match &item.result {
            Ok(r) if r.outcome == Outcome::NoOp => ("unchanged", None),
            Ok(_) => ("changed", None),
            Err(e) => ("failed", Some(ErrorJson::of(e))),
        };
        ItemJson {
            app_id: item.request.app_id,
            name: item.game_name.clone(),
            dll_type: item.request.dll_type,
            path: (!item.request.path.as_os_str().is_empty()).then(|| item.request.path.clone()),
            from_version: item.from_version.clone(),
            to_version: item.to_version.clone(),
            outcome,
            error,
        }
    }
}

/// An item's error.
#[derive(Debug, Clone, Serialize)]
pub struct ErrorJson {
    /// The code, as in the error object.
    pub code: &'static str,
    /// The message.
    pub message: String,
    /// Whether game files changed anyway (a partial failure).
    pub files_changed: bool,
}

impl ErrorJson {
    fn of(e: &OpError) -> ErrorJson {
        ErrorJson {
            code: op_error(e).kind,
            message: e.to_string(),
            files_changed: e.files_changed(),
        }
    }
}

/// A game `--all` left out.
#[derive(Debug, Clone, Serialize)]
pub struct SkippedJson {
    /// The game.
    pub app_id: u64,
    /// Its name.
    pub name: String,
    /// Why.
    pub reason: String,
}

/// `dll restore --json`.
#[derive(Debug, Clone, Serialize)]
struct RestoreJson<'a> {
    schema_version: u32,
    app_id: u64,
    name: &'a str,
    restored: &'a [PathBuf],
    deleted: &'a [PathBuf],
}

#[cfg(test)]
mod tests {
    //! Downloads through the CLI, in process, so the fake network can have
    //! routes. The sandbox tests in `tests/cli_dll.rs` cover the binary.

    use super::*;
    use std::sync::Arc;

    use spela_core::clock::ManualClock;
    use spela_core::env::Env;
    use spela_core::fake::FakeHttp;
    use spela_core::games::cache;
    use spela_core::hex;
    use spela_core::paths::Paths;

    use crate::cli::{self, Parsed};
    use crate::output::{self, Colors};

    const MANIFEST: &str = "http://127.0.0.1:9/manifest.json";

    struct Lab {
        _dir: tempfile::TempDir,
        paths: Paths,
        http: FakeHttp,
        game: PathBuf,
    }

    fn lab() -> Lab {
        let dir = tempfile::tempdir().unwrap();
        let paths = Paths::under(&dir.path().join("xdg"));
        let game = dir.path().join("library/steamapps/common/Ashfall");
        std::fs::create_dir_all(&game).unwrap();
        std::fs::write(game.join("nvngx_dlss.dll"), b"original").unwrap();
        cache::update(&paths, |c| {
            c.upsert(Game {
                app_id: 9100710,
                name: "Ashfall".into(),
                install_dir: game.clone(),
                library_path: dir.path().join("library"),
                prefix_path: None,
                scanned_at: "2026-10-26T15:00:00Z".parse().unwrap(),
                dlls: spela_core::dll::detect::scan_directory(&game, |_| None).unwrap(),
            })
        })
        .unwrap();
        std::fs::create_dir_all(&paths.config_dir).unwrap();
        std::fs::write(
            paths.config_file(),
            format!("dll_manifest_url = \"{MANIFEST}\"\n"),
        )
        .unwrap();
        let http = FakeHttp::new();
        let entry = |t: &str, file: &str, body: &[u8]| {
            format!(
                r#"[{{"version":"310.5.3","filename":"{file}","url":"http://127.0.0.1:9/{t}.dll","sha256":"{}"}}]"#,
                hex::sha256_hex(body)
            )
        };
        http.serve(
            MANIFEST,
            format!(
                r#"{{"version":"1","dlls":{{"dlss":{},"xess":{}}}}}"#,
                entry("dlss", "nvngx_dlss.dll", b"dlss payload"),
                entry("xess", "libxess.dll", b"xess payload")
            ),
        );
        http.serve("http://127.0.0.1:9/dlss.dll", b"dlss payload".to_vec());
        http.serve("http://127.0.0.1:9/xess.dll", b"xess payload".to_vec());
        Lab {
            _dir: dir,
            paths,
            http,
            game,
        }
    }

    fn run(lab: &Lab, args: &[&str]) -> (Result<(), CliError>, String, String) {
        let argv: Vec<std::ffi::OsString> = args.iter().map(Into::into).collect();
        let Parsed::Run(inv) = cli::parse(&argv, &Env::default(), Colors::default()) else {
            panic!("{args:?} did not parse");
        };
        let mut backends = spela_core::fake::backends();
        backends.http = Arc::new(lab.http.clone());
        let mut ctx = Context {
            env: Env::default(),
            paths: lab.paths.clone(),
            backends,
            colors: Colors::default(),
            clock: Arc::new(ManualClock::new("2026-10-26T15:00:00Z".parse().unwrap())),
            migration: Default::default(),
            demo: false,
            log: Default::default(),
        };
        let (mut out, mut err) = (Vec::new(), Vec::new());
        let result = cli::run(&inv, &mut ctx, &mut out, &mut err);
        (
            result,
            String::from_utf8(out).unwrap(),
            String::from_utf8(err).unwrap(),
        )
    }

    #[test]
    fn update_downloads_through_the_backend_and_reports_go_s_lines() {
        let lab = lab();
        let (result, out, err) = run(&lab, &["dll", "update", "Ashfall", "dlss"]);
        result.unwrap();
        assert_eq!(err, "Downloading dlss 310.5.3...\n");
        assert_eq!(
            out,
            "Updated dlss DLL to version 310.5.3\nDLL update result: 1 updated, 0 unchanged, 0 failed\n"
        );
        assert_eq!(
            std::fs::read(lab.game.join("nvngx_dlss.dll")).unwrap(),
            b"dlss payload"
        );
        assert_eq!(
            lab.http.urls(),
            [MANIFEST, "http://127.0.0.1:9/dlss.dll"],
            "only the fixture's loopback URLs"
        );
        // The second run reuses the manifest cache and the DLL cache.
        let (result, _, err) = run(&lab, &["dll", "update", "Ashfall", "dlss"]);
        result.unwrap();
        assert!(err.is_empty(), "no download: {err}");
        assert_eq!(lab.http.requests().len(), 2);
    }

    #[test]
    fn install_json_reports_the_item() {
        let lab = lab();
        let (result, out, _) = run(&lab, &["dll", "install", "9100710", "XeSS", "--json"]);
        result.unwrap();
        let v: serde_json::Value = serde_json::from_str(&out).unwrap();
        assert_eq!(v["schema_version"], 1);
        assert_eq!(v["items"][0]["outcome"], "changed");
        assert_eq!(v["items"][0]["type"], "xess");
        assert_eq!(
            v["items"][0]["path"],
            lab.game.join("libxess.dll").to_str().unwrap()
        );
        assert_eq!(v["items"][0]["from_version"], serde_json::Value::Null);
        assert_eq!(
            std::fs::read(lab.game.join("libxess.dll")).unwrap(),
            b"xess payload"
        );
    }

    #[test]
    fn a_bad_download_fails_with_the_report_in_its_details() {
        let lab = lab();
        lab.http
            .serve("http://127.0.0.1:9/dlss.dll", b"tampered".to_vec());
        let (result, out, _) = run(&lab, &["--json", "dll", "update", "Ashfall"]);
        assert!(out.is_empty(), "nothing before the error object: {out}");
        let e = result.unwrap_err();
        assert_eq!(e.kind, "dll_update_failed");
        let mut text = Vec::new();
        output::write_json_error(&mut text, &e).unwrap();
        let v: serde_json::Value = serde_json::from_slice(&text).unwrap();
        let item = &v["error"]["details"]["items"][0];
        assert_eq!(item["error"]["code"], "download_failed");
        assert!(
            item["error"]["message"]
                .as_str()
                .unwrap()
                .contains("checksum mismatch"),
            "{item}"
        );
        assert_eq!(
            std::fs::read(lab.game.join("nvngx_dlss.dll")).unwrap(),
            b"original"
        );
    }
}
