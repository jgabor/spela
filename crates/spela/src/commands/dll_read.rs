//! The read side of `spela dll`: `dll list [game]`, `dll check-updates` and
//! `dll cache [type]`.
//!
//! They read the game cache as it is, as Go did, and never rescan. Without
//! a game, `dll list` shows every listed game with a DLSS, XeSS or FSR DLL,
//! by name; Go showed only DLSS games, in map order, against its own label
//! (decision 8). Lines are Go's and carry no colour, as in Go.
//!
//! `dll check-updates` gets the manifest under its refresh policy, as Go's
//! did: the cache while it is current, otherwise a fetch, and the cache
//! again, with a warning, when the fetch fails. `--refresh` fetches even a
//! current cache. A DLL needs an update when [`stale::freshness`] says it
//! is stale (decision 38), or when its version is unknown and the manifest
//! has a version to offer, which Go also listed and which `dll update`
//! swaps.
//!
//! `dll cache` lists the versions in the DLL cache, newest first: what a
//! `dll update --cached-only` can use without the network.

use std::fmt::Write as _;
use std::io::Write;

use clap::Args;
use serde::Serialize;
use spela_core::dll::detect::CATALOGUE;
use spela_core::dll::fetch;
use spela_core::dll::manifest::{self, RefreshPolicy};
use spela_core::dll::stale::{self, Freshness};
use spela_core::games::{DllType, Game};

use super::dll_write::parse_dll_type;
use super::read::{self, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};
use crate::json::SCHEMA_VERSION;

/// The text when no game has a known DLL.
pub const NO_DLL_GAMES: &str = "No games with DLSS/FSR/XeSS DLLs found.";

/// `dll list` arguments.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct ListArgs {
    /// The game: a Steam AppID or its exact name. Without one, every game
    /// with a DLSS, XeSS or FSR DLL.
    pub game: Option<String>,
}

/// Runs `spela dll list`.
pub fn list(
    args: &ListArgs,
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let cache = read::games(ctx, Rescan::Never)?;
    let games: Vec<&Game> = match &args.game {
        Some(query) => vec![read::find_game(&cache, query)?],
        None => read::games_with_dlls(&cache),
    };
    if games.is_empty() {
        return read::write_out(out, &format!("{NO_DLL_GAMES}\n"));
    }
    let mut text = String::new();
    for game in games {
        let _ = writeln!(text, "{} ({})", game.name, game.app_id);
        for dll in read::dlls_in_path_order(game) {
            let _ = writeln!(
                text,
                "  {}: {}",
                dll.name,
                read::version_text(dll.version.as_deref())
            );
            let _ = writeln!(text, "    {}", dll.path.display());
        }
    }
    read::write_out(out, &text)
}

/// `dll check-updates` arguments.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct CheckUpdatesArgs {
    /// Fetch the manifest even when the cached one is current.
    #[arg(long)]
    pub refresh: bool,
}

/// `dll cache` arguments.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct CacheArgs {
    /// Only this DLL family: dlss, dlssg, dlssd, xess or fsr.
    #[arg(value_name = "TYPE", value_parser = parse_dll_type)]
    pub dll_type: Option<DllType>,
}

/// Runs `spela dll check-updates`.
pub fn check_updates(
    args: &CheckUpdatesArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let config = read::config(ctx)?;
    let source = fetch::get_manifest(
        &ctx.paths,
        &RefreshPolicy::from_config(&config),
        ctx.backends.http.as_ref(),
        ctx.clock.as_ref(),
        args.refresh,
    )
    .map_err(|e| {
        CliError::new(exit::FAILURE, e.to_string())
            .with_hint("check the network, or dll_manifest_url in 'spela config show'")
            .with_kind("no_manifest")
    })?;
    if let Some(warning) = &source.warning {
        tracing::warn!("{warning}");
    }
    let cache = read::games(ctx, Rescan::Never)?;
    let games = read::games_with_dlls(&cache);
    let mut found: Vec<UpdatesGame> = Vec::new();
    for game in games {
        let mut dlls = Vec::new();
        for dll in read::dlls_in_path_order(game) {
            // Only stale DLLs, as the TUI marks them (decisions 38 and 70):
            // a DLL of unknown version is not offered.
            let (current, latest) = match stale::of_dll(dll, source.manifest()) {
                Freshness::Stale { latest } => (dll.version.clone(), latest),
                _ => continue,
            };
            dlls.push(UpdatesDll {
                path: dll.path.clone(),
                name: dll.name.clone(),
                dll_type: dll.dll_type,
                version: current,
                latest: latest.version.clone(),
            });
        }
        if !dlls.is_empty() {
            found.push(UpdatesGame {
                app_id: game.app_id,
                name: game.name.clone(),
                dlls,
            });
        }
    }
    if inv.cli.json {
        for g in &mut found {
            g.dlls
                .sort_by(|a, b| a.dll_type.cmp(&b.dll_type).then(a.path.cmp(&b.path)));
        }
        return read::write_json(
            out,
            &Updates {
                schema_version: SCHEMA_VERSION,
                manifest: ManifestJson {
                    source_url: source.cached.source().to_owned(),
                    fetched_at: source.cached.fetched_at,
                    fetched_now: source.fetched,
                },
                games: found,
            },
        );
    }
    if games_empty(&cache) {
        return read::write_out(out, &format!("{NO_DLL_GAMES}\n"));
    }
    let mut text = String::new();
    for game in &found {
        let _ = writeln!(text, "{} ({})", game.name, game.app_id);
        for dll in &game.dlls {
            let _ = writeln!(
                text,
                "  {}: {} -> {}",
                dll.name,
                read::version_text(dll.version.as_deref()),
                dll.latest
            );
        }
    }
    if text.is_empty() {
        text = "All DLLs are up to date.\n".to_owned();
    }
    read::write_out(out, &text)
}

fn games_empty(cache: &spela_core::games::GameCache) -> bool {
    read::games_with_dlls(cache).is_empty()
}

/// `dll check-updates --json`.
#[derive(Debug, Serialize)]
struct Updates {
    schema_version: u32,
    manifest: ManifestJson,
    games: Vec<UpdatesGame>,
}

#[derive(Debug, Serialize)]
struct ManifestJson {
    source_url: String,
    fetched_at: Option<jiff::Timestamp>,
    fetched_now: bool,
}

#[derive(Debug, Serialize)]
struct UpdatesGame {
    app_id: u64,
    name: String,
    dlls: Vec<UpdatesDll>,
}

#[derive(Debug, Serialize)]
struct UpdatesDll {
    path: std::path::PathBuf,
    name: String,
    #[serde(rename = "type")]
    dll_type: DllType,
    version: Option<String>,
    latest: String,
}

/// Runs `spela dll cache [type]`.
pub fn cache(
    args: &CacheArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
) -> Result<(), CliError> {
    let config = read::config(ctx)?;
    let dir = config.dll_cache_dir(&ctx.paths);
    let mut types = Vec::new();
    for family in CATALOGUE
        .iter()
        .filter(|f| args.dll_type.is_none_or(|t| t == f.dll_type))
    {
        let versions = manifest::cached_versions(&dir, family.dll_type).map_err(|e| {
            CliError::new(
                exit::FAILURE,
                format!(
                    "cannot read {}: {e}",
                    manifest::cache_dir(&dir, family.dll_type).display()
                ),
            )
        })?;
        types.push(CacheType {
            dll_type: family.dll_type,
            versions,
        });
    }
    if inv.cli.json {
        return read::write_json(
            out,
            &CacheJson {
                schema_version: SCHEMA_VERSION,
                cache_dir: &dir,
                types,
            },
        );
    }
    let mut text = format!("DLL cache: {}\n", dir.display());
    for t in &types {
        let list = if t.versions.is_empty() {
            "none".to_owned()
        } else {
            t.versions.join(", ")
        };
        let _ = writeln!(text, "  {}: {list}", t.dll_type);
    }
    read::write_out(out, &text)
}

/// `dll cache --json`.
#[derive(Debug, Serialize)]
struct CacheJson<'a> {
    schema_version: u32,
    cache_dir: &'a std::path::Path,
    types: Vec<CacheType>,
}

#[derive(Debug, Serialize)]
struct CacheType {
    #[serde(rename = "type")]
    dll_type: DllType,
    versions: Vec<String>,
}
