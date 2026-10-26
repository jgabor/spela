//! `spela config`: show or change spela's settings.
//!
//! Each subcommand still takes a placeholder [`Rest`] and says it is not
//! built yet until its phase declares its arguments here.

use std::io::Write;

use clap::Subcommand;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `config` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ConfigCommand {
    /// Print the config.
    Show(Rest),
    /// Change one setting.
    Set(Rest),
}

/// Runs one `config` subcommand.
pub fn run(
    command: &ConfigCommand,
    inv: &Invocation,
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ConfigCommand::Show(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
        ConfigCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
