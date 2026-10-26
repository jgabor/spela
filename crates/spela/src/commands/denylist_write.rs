//! The writes of `spela denylist`: `allow`, `deny` and `clear`.
//!
//! They edit `lists/overrides.toml` under `config.lock`, keeping its
//! comments and order. Allow and deny for one game are exclusive: allowing
//! removes the game from `denied` and denying removes it from `allowed`, so
//! the latest action wins (decision 36). `clear` removes both, so the
//! built-in and user lists decide again (decision 51).
//!
//! The game argument is a game the cache knows, by AppID or exact name, or
//! else any numeric AppID. Such a game takes its name from the built-in
//! list, or else its digits stand for its name, as in Go.
//! The texts of `allow` and `deny` are Go's.

use std::io::Write;

use clap::Args;
use spela_core::denylist::{self, BUILT_IN, DEFAULT_REASON};

use crate::cli::Context;
use crate::commands::target::{SCAN_HINT, games, line, list_error};
use crate::error::CliError;
use crate::migration;

/// The warning `denylist allow` prints, Go's.
pub const ALLOW_WARNING: &str = "WARNING: Swapping DLLs in anti-cheat games may result in bans!";

/// `denylist allow` and `clear` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct GameArgs {
    /// The game: its Steam AppID or its exact name. Any AppID works, even
    /// one the game list does not know.
    pub game: String,
}

/// `denylist deny` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct DenyArgs {
    /// The game: its Steam AppID or its exact name. Any AppID works, even
    /// one the game list does not know.
    pub game: String,
    /// Why DLL swaps are denied.
    #[arg(long, default_value = DEFAULT_REASON)]
    pub reason: String,
}

/// The AppID and name a deny list argument names: a game the cache knows,
/// else any numeric AppID, named as the built-in list names it or, as in
/// Go, by its digits.
fn listed_game(ctx: &Context, query: &str) -> Result<(u64, String), CliError> {
    let games = games(ctx)?;
    match games.find(query) {
        Ok(game) => Ok((game.app_id, game.name.clone())),
        Err(not_found) => match query.parse::<u64>() {
            Ok(app_id) => {
                let name = BUILT_IN
                    .iter()
                    .find(|(id, _, _)| *id == app_id)
                    .map_or(query, |(_, name, _)| *name);
                Ok((app_id, name.to_owned()))
            }
            Err(_) => Err(CliError::from(not_found).with_hint(SCAN_HINT)),
        },
    }
}

/// Runs `denylist allow`.
pub fn allow(args: &GameArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let (app_id, name) = listed_game(ctx, &args.game)?;
    migration::refuse_overrides(&ctx.migration)?;
    denylist::allow(&ctx.paths, app_id).map_err(list_error)?;
    line(out, &format!("Force-allowed {name} ({app_id})"))?;
    line(out, ALLOW_WARNING)
}

/// Runs `denylist deny`.
pub fn deny(args: &DenyArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let (app_id, name) = listed_game(ctx, &args.game)?;
    migration::refuse_overrides(&ctx.migration)?;
    let reason = if args.reason.is_empty() {
        DEFAULT_REASON
    } else {
        &args.reason
    };
    denylist::deny(&ctx.paths, app_id, &name, reason).map_err(list_error)?;
    line(out, &format!("Denied {name} ({app_id}): {reason}"))
}

/// Runs `denylist clear`.
pub fn clear(args: &GameArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let (app_id, name) = listed_game(ctx, &args.game)?;
    migration::refuse_overrides(&ctx.migration)?;
    let text = if denylist::clear(&ctx.paths, app_id).map_err(list_error)? {
        format!("Cleared your allow or deny for {name} ({app_id})")
    } else {
        format!("{name} ({app_id}) has no allow or deny of yours")
    };
    line(out, &text)
}
