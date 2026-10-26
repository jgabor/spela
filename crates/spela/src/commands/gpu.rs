//! `spela gpu`: gPU information, profile settings and reset.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

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
    Show(Rest),
    /// Make GPU settings follow All games.
    ProfileReset(Rest),
}

/// Runs one `gpu` subcommand.
pub fn run(
    command: &GpuCommand,
    inv: &Invocation,
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        GpuCommand::Info(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        GpuCommand::Reset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        GpuCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        GpuCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        GpuCommand::ProfileReset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
