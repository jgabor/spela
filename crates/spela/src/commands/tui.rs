//! `spela tui`: the interactive terminal UI.

use spela_core::lock::InstanceLock;

use crate::cli::Context;
use crate::error::CliError;

/// Runs the TUI. [`crate::cli`] has already taken the TUI lock, which is
/// held until this returns. Next in the startup order: the config and the
/// terminal (Phase 4).
pub fn run(_ctx: &mut Context, _lock: InstanceLock) -> Result<(), CliError> {
    Err(CliError::not_implemented("tui"))
}
