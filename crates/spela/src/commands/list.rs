//! `spela list`: the games in the game cache.
//!
//! The cache is scanned first only when it is empty (decision 24; Go
//! rescanned on every run while `rescan_on_startup` was on). Steam tools
//! are dropped and games come by name, then app id. An empty cache prints
//! `No games found. Run 'spela scan' first.`, and with `--json` an empty
//! list, where Go printed the text even then (decision 8).

use std::fmt::Write as _;
use std::io::Write;

use clap::Args;
use spela_core::games::{Game, GameCache};

use super::read::{self, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::CliError;
use crate::text::Paint;

/// The text when there are no games.
pub const NO_GAMES: &str = "No games found. Run 'spela scan' first.";

/// `list` arguments.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct ListArgs {
    /// Show each game's DLLs and their versions.
    #[arg(long)]
    pub with_dlls: bool,
}

/// Runs `spela list`.
pub fn run(
    args: &ListArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let cache = read::games(ctx, Rescan::IfEmpty)?;
    if inv.cli.json {
        let list = spela_core::json::game_list(cache.listed(), |id| read::has_profile(ctx, id));
        return read::write_json(out, &list);
    }
    read::write_out(
        out,
        &text(&cache, args.with_dlls, Paint::new(ctx.colors.out)),
    )
}

/// The text list.
pub fn text(cache: &GameCache, with_dlls: bool, paint: Paint) -> String {
    let mut games: Vec<&Game> = cache.listed().collect();
    if games.is_empty() {
        return format!("{NO_GAMES}\n");
    }
    read::sort_by_name(&mut games);
    let mut text = String::new();
    for game in games {
        let _ = writeln!(
            text,
            "{} {}",
            paint.primary(&game.name),
            paint.dim(&format!("({})", game.app_id))
        );
        if with_dlls {
            for dll in read::dlls_in_path_order(game) {
                let _ = writeln!(
                    text,
                    "  {}: {}",
                    paint.secondary(&dll.name),
                    paint.accent(read::version_text(dll.version.as_deref()))
                );
            }
        }
    }
    text
}
