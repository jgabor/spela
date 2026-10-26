//! `spela cpu`: cPU information, profile settings, governor and SMT.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `cpu` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum CpuCommand {
    /// Show the CPU, governor and SMT state.
    Info(Rest),
    /// Show or set the CPU governor now.
    Governor(Rest),
    /// Show or set SMT now.
    Smt(Rest),
    /// Change CPU settings in a profile.
    Set(Rest),
    /// Show CPU settings in a profile.
    Show(Rest),
    /// Make CPU settings follow All games.
    ProfileReset(Rest),
}

/// Runs one `cpu` subcommand.
pub fn run(
    command: &CpuCommand,
    inv: &Invocation,
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        CpuCommand::Info(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Governor(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Smt(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::ProfileReset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
