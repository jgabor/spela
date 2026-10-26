//! `spela profile`: manage per-game profiles and All games.
//!
//! `list` and `show` are in [`super::profile_read`], and the writes in
//! [`super::profile_write`].

use std::io::Write;

use clap::Subcommand;

use super::profile_read::{self, ShowArgs};
use crate::cli::{Context, Invocation};
use crate::commands::profile_write;
use crate::error::CliError;

/// `profile` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProfileCommand {
    /// List games with their own profile.
    List,
    /// Create a profile for a game.
    Create(profile_write::CreateArgs),
    /// Show a game's or All games' settings.
    Show(ShowArgs),
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
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ProfileCommand::List => profile_read::list(inv, ctx, out, err),
        ProfileCommand::Create(a) => profile_write::create(a, ctx, out),
        ProfileCommand::Show(args) => profile_read::show(args, inv, ctx, out, err),
        ProfileCommand::Reset(a) => profile_write::reset(a, ctx, out),
        ProfileCommand::Delete(a) => profile_write::delete(a, ctx, out),
        ProfileCommand::Follow(a) => profile_write::follow(a, ctx, out),
    }
}
