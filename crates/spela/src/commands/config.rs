//! `spela config`: show or change spela's settings.
//!
//! `show` is in [`super::config_read`] and `set` in [`super::config_write`].

use std::io::Write;

use clap::Subcommand;

use super::config_read;
use crate::cli::{Context, Invocation};
use crate::commands::config_write;
use crate::error::CliError;

/// `config` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum ConfigCommand {
    /// Print the config.
    Show,
    /// Change one setting.
    Set(config_write::SetArgs),
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
        ConfigCommand::Set(a) => config_write::set(a, ctx, out),
    }
}
