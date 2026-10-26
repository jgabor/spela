//! `spela denylist`: the anti-cheat deny list for DLL swaps.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::commands::denylist_write;
use crate::error::CliError;

/// `denylist` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DenylistCommand {
    /// Show the built-in and user lists.
    Show(Rest),
    /// Check whether a game's DLLs may be swapped.
    Check(Rest),
    /// Allow DLL swaps for a game.
    Allow(denylist_write::GameArgs),
    /// Deny DLL swaps for a game.
    Deny(denylist_write::DenyArgs),
    /// Remove your allow or deny for a game.
    Clear(denylist_write::GameArgs),
}

/// Runs one `denylist` subcommand.
pub fn run(
    command: &DenylistCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DenylistCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Check(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Allow(a) => denylist_write::allow(a, ctx, out),
        DenylistCommand::Deny(a) => denylist_write::deny(a, ctx, out),
        DenylistCommand::Clear(a) => denylist_write::clear(a, ctx, out),
    }
}
