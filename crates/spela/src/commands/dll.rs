//! `spela dll`: manage DLSS, XeSS and FSR DLLs: list, update, install, restore.
//!
//! The read subcommands are in [`super::dll_read`]. The others still
//! take a placeholder [`Rest`] and say they are not built yet until their
//! phase declares their arguments here.

use std::io::Write;

use clap::Subcommand;

use super::dll_read::{self, ListArgs};
use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `dll` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DllCommand {
    /// List a game's DLLs, or every game with DLSS, XeSS or FSR.
    List(ListArgs),
    /// Check for newer DLL versions.
    CheckUpdates,
    /// Update a game's DLL, or every game's.
    Update(Rest),
    /// Install a DLL into a game.
    Install(Rest),
    /// List cached DLL versions.
    Cache(Rest),
    /// Put a game's original DLLs back.
    Restore(Rest),
}

/// Runs one `dll` subcommand.
pub fn run(
    command: &DllCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DllCommand::List(args) => dll_read::list(args, inv, ctx, out, err),
        DllCommand::CheckUpdates => dll_read::check_updates(inv, ctx, out, err),
        DllCommand::Update(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::Install(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::Cache(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::Restore(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
