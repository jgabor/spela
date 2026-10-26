//! `spela dll`: manage DLSS, XeSS and FSR DLLs: list, update, install, restore.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `dll` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DllCommand {
    /// List a game's DLLs, or every game with DLSS, XeSS or FSR.
    List(Rest),
    /// Check for newer DLL versions.
    CheckUpdates(Rest),
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
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DllCommand::List(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::CheckUpdates(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::Update(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::Install(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::Cache(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DllCommand::Restore(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
