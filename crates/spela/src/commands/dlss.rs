//! `spela dlss`: DLSS settings in a profile.
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

/// `dlss` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DlssCommand {
    /// Show DLSS settings.
    Show(ShowArgs),
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
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DlssCommand::Show(args) => {
            profile_read::section_show(Section::Dlss, args, inv, ctx, out, err)
        }
        DlssCommand::Set(a) => section_write::set(a, ctx, out),
        DlssCommand::Reset(a) => section_write::reset(a, ctx, out),
    }
}
