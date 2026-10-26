//! `spela dlss`: dLSS settings in a profile.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::commands::section_write;
use crate::error::CliError;

/// `dlss` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DlssCommand {
    /// Show DLSS settings.
    Show(Rest),
    /// Change DLSS settings.
    Set(section_write::DlssSetArgs),
    /// Make DLSS settings follow All games.
    Reset(section_write::DlssResetArgs),
}

/// Runs one `dlss` subcommand.
pub fn run(
    command: &DlssCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DlssCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DlssCommand::Set(a) => section_write::set(a, ctx, out),
        DlssCommand::Reset(a) => section_write::reset(a, ctx, out),
    }
}
