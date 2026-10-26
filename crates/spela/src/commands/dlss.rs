//! `spela dlss`: dLSS settings in a profile.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `dlss` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DlssCommand {
    /// Show DLSS settings.
    Show(Rest),
    /// Change DLSS settings.
    Set(Rest),
    /// Make DLSS settings follow All games.
    Reset(Rest),
}

/// Runs one `dlss` subcommand.
pub fn run(
    command: &DlssCommand,
    inv: &Invocation,
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DlssCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DlssCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DlssCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
