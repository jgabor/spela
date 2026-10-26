//! `spela proton`: Proton settings in a profile.
//!
//! `show` is in [`super::profile_read`], and `set` and `reset` in
//! [`super::section_write`].

use std::io::Write;

use clap::Subcommand;

use spela_core::profile::Section;

use super::profile_read::{self, ShowArgs};
use crate::cli::{Context, Invocation};
use crate::commands::section_write;
use crate::error::CliError;

/// `proton` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ProtonCommand {
    /// Change Proton settings in a profile.
    Set(section_write::ProtonSetArgs),
    /// Show Proton settings in a profile.
    Show(ShowArgs),
    /// Make Proton settings follow All games.
    Reset(section_write::ProtonResetArgs),
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
        ProtonCommand::Set(a) => section_write::set(a, ctx, out),
        ProtonCommand::Show(args) => {
            profile_read::section_show(Section::Proton, args, inv, ctx, out, err)
        }
        ProtonCommand::Reset(a) => section_write::reset(a, ctx, out),
    }
}
