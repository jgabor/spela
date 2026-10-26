//! `spela profile`: manage per-game profiles and All games.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::commands::profile_write;
use crate::error::CliError;

/// `profile` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProfileCommand {
    /// List games with their own profile.
    List(Rest),
    /// Create a profile for a game.
    Create(profile_write::CreateArgs),
    /// Show a game's or All games' settings.
    Show(Rest),
    /// Make every setting follow All games, or reset All games.
    Reset(profile_write::ResetArgs),
    /// Delete a game's profile.
    Delete(profile_write::DeleteArgs),
    /// Make games that set a field follow All games for it.
    Follow(profile_write::FollowArgs),
}

/// Runs one `profile` subcommand.
pub fn run(
    command: &ProfileCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ProfileCommand::List(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Create(a) => profile_write::create(a, ctx, out),
        ProfileCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Reset(a) => profile_write::reset(a, ctx, out),
        ProfileCommand::Delete(a) => profile_write::delete(a, ctx, out),
        ProfileCommand::Follow(a) => profile_write::follow(a, ctx, out),
    }
}
