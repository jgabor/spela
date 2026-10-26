//! `spela proton`: proton settings in a profile.
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

/// `proton` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProtonCommand {
    /// Change Proton settings in a profile.
    Set(Rest),
    /// Show Proton settings in a profile.
    Show(ShowArgs),
    /// Make Proton settings follow All games.
    Reset(Rest),
}

/// Runs one `proton` subcommand.
pub fn run(
    command: &ProtonCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ProtonCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ProtonCommand::Show(args) => {
            profile_read::section_show(Section::Proton, args, inv, ctx, out, err)
        }
        ProtonCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
