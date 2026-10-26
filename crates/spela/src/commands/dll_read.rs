//! The read side of `spela dll`: `dll list [game]` and `dll check-updates`.
//!
//! Both read the game cache as it is, as Go did, and never rescan. Without
//! a game, `dll list` shows every listed game with a DLSS, XeSS or FSR DLL,
//! by name; Go showed only DLSS games, in map order, against its own label
//! (decision 8). Lines are Go's and carry no colour, as in Go.
//!
//! `dll check-updates` reads only the cached manifest: Phase 2 fetches
//! nothing. A DLL needs an update when [`stale::freshness`] says it is
//! stale (decision 38), or when its version is unknown and the manifest has
//! a version to offer, which Go also listed and which `dll update` swaps.

use std::fmt::Write as _;
use std::io::Write;

use clap::Args;
use spela_core::dll::manifest;
use spela_core::dll::stale::{self, Freshness};
use spela_core::games::Game;

use super::read::{self, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};

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

/// Runs `spela dll check-updates` on the cached manifest.
pub fn check_updates(
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let cached = manifest::load_cache(&ctx.paths)
        .map_err(|e| {
            CliError::new(
                exit::FAILURE,
                format!("failed to read the DLL manifest: {e}"),
            )
        })?
        .ok_or_else(|| {
            CliError::new(
                exit::FAILURE,
                "no DLL manifest in the cache, so spela cannot check for updates",
            )
            .with_kind("no_manifest")
        })?;
    let cache = read::games(ctx, Rescan::Never)?;
    let games = read::games_with_dlls(&cache);
    if games.is_empty() {
        return read::write_out(out, &format!("{NO_DLL_GAMES}\n"));
    }
    let mut text = String::new();
    for game in games {
        let mut lines = String::new();
        for dll in read::dlls_in_path_order(game) {
            let (current, latest) = match stale::of_dll(dll, &cached.manifest) {
                Freshness::Stale { latest } => (dll.version.as_deref(), latest),
                Freshness::Unknown {
                    latest: Some(latest),
                } => (None, latest),
                _ => continue,
            };
            let _ = writeln!(
                lines,
                "  {}: {} -> {}",
                dll.name,
                read::version_text(current),
                latest.version
            );
        }
        if !lines.is_empty() {
            let _ = writeln!(text, "{} ({})", game.name, game.app_id);
            text.push_str(&lines);
        }
    }
    if text.is_empty() {
        text = "All DLLs are up to date.\n".to_owned();
    }
    read::write_out(out, &text)
}
