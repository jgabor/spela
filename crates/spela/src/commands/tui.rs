//! `spela tui`: the interactive terminal UI, and `--demo` with no command.
//!
//! The shell lives in [`crate::tui`]; this file is the command's entry.

use std::io::Write;

use spela_core::lock::InstanceLock;

use crate::cli::Context;
use crate::error::CliError;

/// Runs the TUI. [`crate::cli`] has already taken the TUI lock, which is
/// held until this returns.
pub fn run(ctx: &mut Context, lock: InstanceLock, err: &mut dyn Write) -> Result<(), CliError> {
    crate::tui::run(ctx, lock, err)
}
