//! `spela scan`: scan Steam libraries for games and their DLLs.
//!
//! Its arguments are [`ScanArgs`], a placeholder [`Rest`] until its phase
//! replaces the alias with its own struct here. Until then it says it is
//! not built yet.

use std::io::Write;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `scan` arguments. Replace the alias with a clap `Args` struct.
pub type ScanArgs = Rest;

/// Runs `spela scan`.
pub fn run(
    _args: &ScanArgs,
    inv: &Invocation,
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    Err(CliError::not_implemented(&inv.path.join(" ")))
}
