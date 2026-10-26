//! `spela cpu`: CPU information, profile settings, governor and SMT.
//!
//! `governor` and `smt` with no argument are in [`super::cpu_read`]; `info`,
//! and `governor` and `smt` with a value, in [`super::hardware`]; `show` in
//! [`super::profile_read`]; and `set` and `profile-reset` in
//! [`super::section_write`].

use std::io::Write;

use clap::Subcommand;

use spela_core::profile::Section;

use super::cpu_read::{self, GovernorArgs, SmtArgs};
use super::profile_read::{self, ShowArgs};
use crate::cli::{Context, Invocation};
use crate::commands::section_write;
use crate::error::CliError;

/// `cpu` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum CpuCommand {
    /// Show the CPU, governor and SMT state.
    Info,
    /// Show or set the CPU governor now.
    Governor(GovernorArgs),
    /// Show or set SMT now.
    Smt(SmtArgs),
    /// Change CPU settings in a profile.
    Set(section_write::CpuSetArgs),
    /// Show CPU settings in a profile.
    Show(ShowArgs),
    /// Make CPU settings follow All games.
    ProfileReset(section_write::CpuResetArgs),
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
        CpuCommand::Info => super::hardware::cpu_info(ctx, out),
        CpuCommand::Governor(GovernorArgs { governor: None }) => {
            cpu_read::governor(inv, ctx, out, err)
        }
        CpuCommand::Governor(GovernorArgs { governor: Some(g) }) => {
            super::hardware::set_governor(ctx, g, out)
        }
        CpuCommand::Smt(SmtArgs { value: None }) => cpu_read::smt(inv, ctx, out, err),
        CpuCommand::Smt(SmtArgs { value: Some(v) }) => super::hardware::set_smt(ctx, v, out),
        CpuCommand::Set(a) => section_write::set(a, ctx, out),
        CpuCommand::Show(args) => {
            profile_read::section_show(Section::Cpu, args, inv, ctx, out, err)
        }
        CpuCommand::ProfileReset(a) => section_write::reset(a, ctx, out),
    }
}
