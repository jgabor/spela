//! `spela dll`: manage DLSS, XeSS and FSR DLLs: list, update, install, restore.
//!
//! The read subcommands are in [`super::dll_read`], the ones that change
//! files in [`super::dll_write`].

use std::io::Write;

use clap::Subcommand;

use super::dll_read::{self, CacheArgs, CheckUpdatesArgs, ListArgs};
use super::dll_write::{self, InstallArgs, RestoreArgs, UpdateArgs};
use crate::cli::{Context, Invocation};
use crate::error::CliError;

/// `dll` subcommands.
#[derive(Debug, Clone, Subcommand)]
pub enum DllCommand {
    /// List a game's DLLs, or every game with DLSS, XeSS or FSR.
    List(ListArgs),
    /// Check for newer DLL versions.
    CheckUpdates(CheckUpdatesArgs),
    /// Update a game's DLLs, or every game's, to the latest version.
    Update(UpdateArgs),
    /// Install a DLL into a game, any version.
    Install(InstallArgs),
    /// List the DLL versions in the DLL cache.
    Cache(CacheArgs),
    /// Put a game's original DLLs back and remove the ones spela added.
    Restore(RestoreArgs),
}

/// Runs one `dll` subcommand.
pub fn run(
    command: &DllCommand,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> Result<(), CliError> {
    match command {
        DllCommand::List(args) => dll_read::list(args, inv, ctx, out, err),
        DllCommand::CheckUpdates(args) => dll_read::check_updates(args, inv, ctx, out, err),
        DllCommand::Update(args) => dll_write::update(args, inv, ctx, out, err),
        DllCommand::Install(args) => dll_write::install(args, inv, ctx, out, err),
        DllCommand::Cache(args) => dll_read::cache(args, inv, ctx, out),
        DllCommand::Restore(args) => dll_write::restore(args, inv, ctx, out),
    }
}
