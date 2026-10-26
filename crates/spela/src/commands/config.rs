//! `spela config`: show or change spela's settings.
//!
//! The read subcommands are in [`super::config_read`]. The others still
//! take a placeholder [`Rest`] and say they are not built yet until their
//! phase declares their arguments here.

use std::io::Write;

use clap::Subcommand;

use super::config_read;
use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `config` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ConfigCommand {
    /// Print the config.
    Show,
    /// Change one setting.
    Set(Rest),
}

/// Runs one `config` subcommand.
pub fn run(
    command: &ConfigCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        ConfigCommand::Show => config_read::show(inv, ctx, out, err),
        ConfigCommand::Set(_) => Err(CliError::not_implemented(&inv.path.join(" "))),
    }
}
