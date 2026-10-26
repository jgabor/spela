//! What runs after logging and the backends and before a command reads the
//! store: the migration from Go spela's files.
//!
//! The architecture's "When it runs": at startup, before anything reads the
//! store, spela migrates each Go source the record does not list as done.
//! The pass is cheap when there is nothing to do. It never runs under
//! `--demo` (decision 18), for the root helper, or for commands that do not
//! touch the store, such as `completion` and a parent's help. The TUI does
//! its own in Phase 4, where the notice goes to the status line.
//!
//! A Go `config.yaml` that does not migrate stops the command with exit 3
//! and Go's advice. Other failures are left in [`Context::migration`] for
//! the commands that read those files: a failed profile exits 3 where it is
//! read, and a failed list makes the deny list commands fail closed.

use std::io::Write;

use spela_core::migrate::{self, CONFIG_HINT};

use crate::cli::{Command, Context, Invocation};
use crate::error::{CliError, exit};

/// Whether `inv` reads or writes the store, so the migration runs first.
pub fn needs_store(inv: &Invocation) -> bool {
    match &inv.cli.command {
        Some(
            Command::Scan(_)
            | Command::List(_)
            | Command::Show(_)
            | Command::Launch(_)
            | Command::Profile { command: Some(_) }
            | Command::Config { command: Some(_) }
            | Command::Dll { command: Some(_) }
            | Command::Dlss { command: Some(_) }
            | Command::Gpu { command: Some(_) }
            | Command::Cpu { command: Some(_) }
            | Command::Proton { command: Some(_) }
            | Command::Denylist { command: Some(_) },
        ) => true,
        Some(_) | None => false,
    }
}

/// Runs the migration for a command that needs the store, prints its
/// notice on `err`, and keeps its report in `ctx.migration`.
pub fn migrate(inv: &Invocation, ctx: &mut Context, err: &mut dyn Write) -> Result<(), CliError> {
    if ctx.demo || !needs_store(inv) {
        return Ok(());
    }
    let report = migrate::run(&ctx.paths, ctx.clock.as_ref()).map_err(|e| {
        CliError::new(
            exit::FAILURE,
            format!("cannot migrate Go spela's files: {e}"),
        )
    })?;
    for line in report.notice() {
        // A closed stderr loses only the notice; the record keeps it all.
        let _ = writeln!(err, "{line}");
    }
    // Each failed source is already a warning on stderr, from the migration.
    if let Some(message) = report.config_failure() {
        return Err(CliError::new(exit::CONFIG, message).with_hint(CONFIG_HINT));
    }
    ctx.migration = report;
    Ok(())
}
