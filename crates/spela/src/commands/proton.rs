//! `spela proton`: proton settings in a profile.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::commands::section_write;
use crate::error::CliError;

/// `proton` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProtonCommand {
    /// Change Proton settings in a profile.
    Set(section_write::ProtonSetArgs),
    /// Show Proton settings in a profile.
    Show(Rest),
    /// Make Proton settings follow All games.
    Reset(section_write::ProtonResetArgs),
}

/// Runs one `proton` subcommand.
pub fn run(
    command: &ProtonCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ProtonCommand::Set(a) => section_write::set(a, ctx, out),
        ProtonCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProtonCommand::Reset(a) => section_write::reset(a, ctx, out),
    }
}
