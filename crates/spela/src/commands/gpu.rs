//! `spela gpu`: GPU information, profile settings and reset.
//!
//! `info` and `reset` are in [`super::hardware`], `show` in
//! [`super::profile_read`], and `set` and `profile-reset` in
//! [`super::section_write`].

use std::io::Write;

use clap::Subcommand;

use spela_core::profile::Section;

use super::profile_read::{self, ShowArgs};
use crate::cli::{Context, Invocation};
use crate::commands::section_write;
use crate::error::CliError;

/// `gpu` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum GpuCommand {
    /// Show the GPU, driver and limits.
    Info,
    /// Reset GPU 0's locked clocks and clock offsets to the driver defaults now.
    Reset,
    /// Change GPU settings in a profile.
    Set(section_write::GpuSetArgs),
    /// Show GPU settings in a profile.
    Show(ShowArgs),
    /// Make GPU settings follow All games.
    ProfileReset(section_write::GpuResetArgs),
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
        GpuCommand::Info => super::hardware::gpu_info(ctx, out),
        GpuCommand::Reset => super::hardware::gpu_reset(ctx, out),
        GpuCommand::Set(a) => section_write::set(a, ctx, out),
        GpuCommand::Show(args) => {
            profile_read::section_show(Section::Gpu, args, inv, ctx, out, err)
        }
        GpuCommand::ProfileReset(a) => section_write::reset(a, ctx, out),
    }
}
