//! `spela cpu`: cPU information, profile settings, governor and SMT.
//!
//! The read subcommands are in [`super::cpu_read`] and [`super::profile_read`]. The others still
//! take a placeholder [`Rest`] and say they are not built yet until their
//! phase declares their arguments here.

use std::io::Write;

use clap::Subcommand;

use spela_core::profile::Section;

use super::cpu_read::{self, GovernorArgs, SmtArgs};
use super::profile_read::{self, ShowArgs};
use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `cpu` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum CpuCommand {
    /// Show the CPU, governor and SMT state.
    Info(Rest),
    /// Show or set the CPU governor now.
    Governor(GovernorArgs),
    /// Show or set SMT now.
    Smt(SmtArgs),
    /// Change CPU settings in a profile.
    Set(Rest),
    /// Show CPU settings in a profile.
    Show(ShowArgs),
    /// Make CPU settings follow All games.
    ProfileReset(Rest),
}

/// Runs one `cpu` subcommand.
pub fn run(
    command: &CpuCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        CpuCommand::Info(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Governor(GovernorArgs { governor: None }) => {
            cpu_read::governor(inv, ctx, out, err)
        }
        CpuCommand::Governor(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Smt(SmtArgs { value: None }) => cpu_read::smt(inv, ctx, out, err),
        CpuCommand::Smt(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        CpuCommand::Show(args) => {
            profile_read::section_show(Section::Cpu, args, inv, ctx, out, err)
        }
        CpuCommand::ProfileReset(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
