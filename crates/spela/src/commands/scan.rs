//! `spela scan`: scan Steam libraries for games and their DLLs, and write
//! the game cache. It is the one command that always rescans (decision 24).
//!
//! Text, as Go prints it: `Found <N> games`, then, when any game has a
//! DLSS, XeSS or FSR DLL, `Games with DLSS/FSR/XeSS: <M>` and each such
//! game with its DLLs. Go counted only DLSS games against that label; spela
//! counts every family (decision 8). Games come by name, not Go's map
//! order. `--json` prints the listed games in schema version 1, with Steam
//! tools dropped like `list`.

use std::fmt::Write as _;
use std::io::Write;

use clap::Args;
use spela_core::games::scan::{self, RescanError};
use spela_core::games::{GameCache, cache};

use super::read;
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};
use crate::text::Paint;

/// `scan` arguments: none of its own. `--json` is global.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct ScanArgs {}

/// Runs `spela scan`.
pub fn run(
    _args: &ScanArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let config = read::config(ctx)?;
    let install = read::steam(ctx, &config);
    match scan::rescan(&ctx.paths, &install, ctx.clock.as_ref()) {
        Ok(_) => {}
        Err(RescanError::NoSteam) => return Err(read::no_steam("scan failed")),
        Err(e) => {
            return Err(CliError::new(exit::FAILURE, format!("scan failed: {e}")));
        }
    }
    let cache = cache::load(&ctx.paths);
    if inv.cli.json {
        let list = spela_core::json::game_list(cache.listed(), |id| read::has_profile(ctx, id));
        return read::write_json(out, &list);
    }
    read::write_out(out, &text(&cache, Paint::new(ctx.colors.out)))
}

/// The text report of a scan.
pub fn text(cache: &GameCache, paint: Paint) -> String {
    let mut text = String::new();
    let _ = writeln!(
        text,
        "Found {} games",
        paint.primary(&cache.games.len().to_string())
    );
    let games = read::games_with_dlls(cache);
    if games.is_empty() {
        return text;
    }
    let _ = writeln!(
        text,
        "Games with DLSS/FSR/XeSS: {}",
        paint.accent(&games.len().to_string())
    );
    for game in games {
        let _ = writeln!(
            text,
            "  - {} {}",
            paint.primary(&game.name),
            paint.dim(&format!("({})", game.app_id))
        );
        for dll in read::dlls_in_path_order(game) {
            let _ = writeln!(
                text,
                "      {}: {}",
                paint.secondary(&dll.name),
                paint.accent(read::version_text(dll.version.as_deref()))
            );
        }
    }
    text
}
