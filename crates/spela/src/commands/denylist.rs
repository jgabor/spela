//! `spela denylist`: the anti-cheat deny list for DLL swaps.
//!
//! `show` and `check` are in [`super::denylist_read`], and `allow`, `deny`
//! and `clear` in [`super::denylist_write`].

use std::io::Write;

use clap::Subcommand;

use super::denylist_read::{self, CheckArgs};
use crate::cli::{Context, Invocation};
use crate::commands::denylist_write;
use crate::error::CliError;

/// `denylist` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DenylistCommand {
    /// Show the built-in and user lists.
    Show,
    /// Check whether a game's DLLs may be swapped.
    Check(CheckArgs),
    /// Allow DLL swaps for a game.
    Allow(denylist_write::GameArgs),
    /// Deny DLL swaps for a game.
    Deny(denylist_write::DenyArgs),
    /// Remove your allow or deny for a game.
    Clear(denylist_write::GameArgs),
}

/// Runs one `denylist` subcommand.
pub fn run(
    command: &DenylistCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DenylistCommand::Show => denylist_read::show(inv, ctx, out, err),
        DenylistCommand::Check(args) => denylist_read::check(args, inv, ctx, out, err),
        DenylistCommand::Allow(a) => denylist_write::allow(a, ctx, out),
        DenylistCommand::Deny(a) => denylist_write::deny(a, ctx, out),
        DenylistCommand::Clear(a) => denylist_write::clear(a, ctx, out),
    }
}
