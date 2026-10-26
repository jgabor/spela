//! `spela proton`: proton settings in a profile.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `proton` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProtonCommand {
    /// Change Proton settings in a profile.
    Set(Rest),
    /// Show Proton settings in a profile.
    Show(Rest),
    /// Make Proton settings follow All games.
    Reset(Rest),
}

/// Runs one `proton` subcommand.
pub fn run(
    command: &ProtonCommand,
    inv: &Invocation,
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ProtonCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProtonCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProtonCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
