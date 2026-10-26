//! `spela dlss`: dLSS settings in a profile.
//!
//! The read subcommands are in [`super::profile_read`]. The others still
//! take a placeholder [`Rest`] and say they are not built yet until their
//! phase declares their arguments here.

use std::io::Write;

use clap::Subcommand;

use spela_core::profile::Section;

use super::profile_read::{self, ShowArgs};
use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `dlss` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DlssCommand {
    /// Show DLSS settings.
    Show(ShowArgs),
    /// Change DLSS settings.
    Set(Rest),
    /// Make DLSS settings follow All games.
    Reset(Rest),
}

/// Runs one `dlss` subcommand.
pub fn run(
    command: &DlssCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DlssCommand::Show(args) => {
            profile_read::section_show(Section::Dlss, args, inv, ctx, out, err)
        }
        DlssCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        DlssCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
