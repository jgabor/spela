//! `spela profile`: manage per-game profiles and All games.
//!
//! The read subcommands are in [`super::profile_read`]. The others still
//! take a placeholder [`Rest`] and say they are not built yet until their
//! phase declares their arguments here.

use std::io::Write;

use clap::Subcommand;

use super::profile_read::{self, ShowArgs};
use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `profile` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProfileCommand {
    /// List games with their own profile.
    List,
    /// Create a profile for a game.
    Create(Rest),
    /// Show a game's or All games' settings.
    Show(ShowArgs),
    /// Make every setting follow All games, or reset All games.
    Reset(Rest),
    /// Delete a game's profile.
    Delete(Rest),
    /// Make games that set a field follow All games for it.
    Follow(Rest),
}

/// Runs one `profile` subcommand.
pub fn run(
    command: &ProfileCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ProfileCommand::List => profile_read::list(inv, ctx, out, err),
        ProfileCommand::Create(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Show(args) => profile_read::show(args, inv, ctx, out, err),
        ProfileCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Delete(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Follow(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
