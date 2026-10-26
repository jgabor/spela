//! `spela profile`: manage per-game profiles and All games.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `profile` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProfileCommand {
    /// List games with their own profile.
    List(Rest),
    /// Create a profile for a game.
    Create(Rest),
    /// Show a game's or All games' settings.
    Show(Rest),
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
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ProfileCommand::List(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Create(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Delete(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProfileCommand::Follow(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
