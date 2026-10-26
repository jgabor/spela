//! `spela denylist`: the anti-cheat deny list for DLL swaps.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `denylist` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DenylistCommand {
    /// Show the built-in and user lists.
    Show(Rest),
    /// Check whether a game's DLLs may be swapped.
    Check(Rest),
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
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DenylistCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Check(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Allow(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Deny(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DenylistCommand::Clear(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
