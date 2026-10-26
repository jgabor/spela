//! `spela denylist`: the anti-cheat deny list for DLL swaps.
//!
//! The read subcommands are in [`super::denylist_read`]. The others still
//! take a placeholder [`Rest`] and say they are not built yet until their
//! phase declares their arguments here.

use std::io::Write;

use clap::Subcommand;

use super::denylist_read::{self, CheckArgs};
use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `denylist` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DenylistCommand {
    /// Show the built-in and user lists.
    Show,
    /// Check whether a game's DLLs may be swapped.
    Check(CheckArgs),
    /// Allow DLL swaps for a game.
    Allow(Rest),
    /// Deny DLL swaps for a game.
    Deny(Rest),
    /// Remove your allow or deny for a game.
    Clear(Rest),
}

/// Runs one `denylist` subcommand.
pub fn run(
    command: &DenylistCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DenylistCommand::Show => denylist_read::show(inv, ctx, out, err),
        DenylistCommand::Check(args) => denylist_read::check(args, inv, ctx, out, err),
        DenylistCommand::Allow(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Deny(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Clear(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
