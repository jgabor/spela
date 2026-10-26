//! `spela list`: list the games found in Steam libraries.
//!
//! Its arguments are [`ListArgs`], a placeholder [`Rest`] until its phase
//! replaces the alias with its own struct here. Until then it says it is
//! not built yet.

use std::io::Write;

use crate::cli::{Context, Invocation, Rest};
use crate::error::CliError;

/// `list` arguments. Replace the alias with a clap `Args` struct.
pub type ListArgs = Rest;

/// Runs `spela list`.
pub fn run(
    _args: &ListArgs,
    inv: &Invocation,
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    Err(CliError::not_implemented(&inv.path.join(" ")))
}
