//! `spela gpu`: gPU information, profile settings and reset.
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

/// `gpu` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum GpuCommand {
    /// Show the GPU, driver and limits.
    Info(Rest),
    /// Reset GPU 0's locked clocks and clock offsets to the driver defaults now.
    Reset(Rest),
    /// Change GPU settings in a profile.
    Set(Rest),
    /// Show GPU settings in a profile.
    Show(ShowArgs),
    /// Make GPU settings follow All games.
    ProfileReset(Rest),
}

/// Runs one `gpu` subcommand.
pub fn run(
    command: &GpuCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        GpuCommand::Info(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        GpuCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        GpuCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        GpuCommand::Show(args) => {
            profile_read::section_show(Section::Gpu, args, inv, ctx, out, err)
        }
        GpuCommand::ProfileReset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
