//! The read side of `spela cpu governor` and `spela cpu smt`: with no
//! argument they print the live state through the [`spela_core::backend::Cpu`]
//! backend. With an argument they change it, which is the write side.
//!
//! The lines are Go's: `Current governor: <g>` and `Available: [a b]`, the
//! second in Go's `%v` slice format and left out when the list cannot be
//! read, and `SMT: true|false`.

use std::io::Write;

use clap::Args;

use super::read;
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};

/// `cpu governor` arguments.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct GovernorArgs {
    /// The governor to set now. Without one, the current governor is shown.
    pub governor: Option<String>,
}

/// `cpu smt` arguments.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct SmtArgs {
    /// on or off. Without one, the current state is shown.
    pub value: Option<String>,
}

/// Runs `spela cpu governor` with no argument.
pub fn governor(
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let cpu = &mut ctx.backends.cpu;
    let current = cpu
        .governor()
        .map_err(|e| CliError::new(exit::FAILURE, e.to_string()))?;
    let mut text = format!("Current governor: {current}\n");
    match cpu.available_governors() {
        Ok(available) => text.push_str(&format!("Available: [{}]\n", available.join(" "))),
        Err(e) => tracing::info!("cannot list the governors: {e}"),
    }
    read::write_out(out, &text)
}

/// Runs `spela cpu smt` with no argument.
pub fn smt(
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let active = ctx
        .backends
        .cpu
        .smt_active()
        .map_err(|e| CliError::new(exit::FAILURE, e.to_string()))?;
    read::write_out(out, &format!("SMT: {active}\n"))
}
