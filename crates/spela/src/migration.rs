//! The migration from Go spela, run once at startup.
//!
//! [`run`] runs [`spela_core::migrate::run`] before a command reads the
//! config, the profiles or the lists, so every command that reads them sees
//! the migrated TOML files ([`needs_store`] says which commands those are).
//! Write commands run it too: a Go source that is not settled in the record
//! is retried, and a retry adopts any target that already exists, so a write
//! that made `config.toml` or `<appid>.toml` first would keep the Go values
//! from ever being imported.
//!
//! What the CLI does with the report:
//!
//! - The one-time notice goes to stderr on the run that migrates, before the
//!   command runs, even when the command then fails and under `--json`.
//!   `tui` logs to the file only, so it runs the migration itself, after its
//!   terminal check, and shows the notice inside the TUI.
//! - A Go `config.yaml` that does not migrate exits 3 with Go's advice.
//! - A write to a scope whose Go file failed is refused with exit 3
//!   ([`refuse_profile`]): the Go file is tried again at the next start, and
//!   a TOML file written now would be adopted in its place. The deny list
//!   module refuses its own writes the same way.

use std::io::Write;
use std::path::Path;

use spela_core::clock::Clock;
use spela_core::migrate::{self, CONFIG_HINT, Handled, MigrateError, Report, SourceKind};
use spela_core::paths::Paths;
use spela_core::profile::Scope;

use crate::cli::Command;
use crate::commands::cpu::CpuCommand;
use crate::commands::gpu::GpuCommand;
use crate::error::{CliError, exit};

/// The advice for a Go profile that does not migrate.
pub const SOURCE_HINT: &str = "Fix the YAML or remove the file; spela migrates it on the next run, and the Go file stays as a backup.";

/// The advice for a Go deny list file that does not migrate. Removing it
/// would throw away the user's allows and denials, so it never says to.
pub const LIST_HINT: &str = "Fix the YAML or the folder's permissions; spela migrates the file on the next run. Removing it would lose the games it lists.";

/// Whether a command reads or writes spela's config, profiles or lists, so
/// the migration must run before it. Help, completions, the removed names,
/// the root helper and the commands that only read or change live hardware
/// do not. `--demo` never does: it runs on its own temp files and never
/// reads Go YAML (decision 18). `tui` runs it itself, after its terminal
/// check, so the notice shows inside the TUI and a run with no terminal
/// migrates nothing.
pub fn needs_store(command: Option<&Command>, demo: bool) -> bool {
    if demo {
        return false;
    }
    let Some(command) = command else {
        return false;
    };
    !matches!(
        command,
        Command::Completion { .. }
            | Command::Tui
            | Command::Overlay(_)
            | Command::Gui(_)
            | Command::ApplyProfile(_)
            | Command::Profile { command: None }
            | Command::Config { command: None }
            | Command::Dll { command: None }
            | Command::Dlss { command: None }
            | Command::Gpu { command: None }
            | Command::Cpu { command: None }
            | Command::Proton { command: None }
            | Command::Denylist { command: None }
            | Command::Gpu {
                command: Some(GpuCommand::Info | GpuCommand::Reset),
            }
            | Command::Cpu {
                command: Some(CpuCommand::Info | CpuCommand::Governor(_) | CpuCommand::Smt(_)),
            }
    )
}

/// Runs the migration and handles its report: the notice on `err` when
/// `notice` is set, and exit 3 when Go's `config.yaml` did not migrate.
/// Returns the report for the command, which refuses writes to a scope
/// whose Go file failed.
pub fn run(
    paths: &Paths,
    clock: &dyn Clock,
    err: &mut dyn Write,
    notice: bool,
) -> Result<Report, CliError> {
    let report = migrate::run(paths, clock).map_err(migrate_error)?;
    if notice {
        for line in report.notice() {
            // A closed stderr must not stop the command.
            let _ = writeln!(err, "{line}");
        }
    }
    if let Some(message) = report.config_failure() {
        return Err(CliError::new(exit::CONFIG, message).with_hint(CONFIG_HINT));
    }
    Ok(report)
}

fn migrate_error(e: MigrateError) -> CliError {
    match e {
        MigrateError::Record(e) => CliError::new(exit::CONFIG, e.to_string()),
        MigrateError::Lock(e) => CliError::from(e),
    }
}

/// The failed source of this kind in the report, if any.
fn failed(report: &Report, kind: SourceKind) -> Option<&Handled> {
    report.failed().find(|h| h.kind == kind)
}

/// The error for a Go profile that did not migrate: exit 3, naming the
/// file. A Go deny list that did not migrate is the core's
/// `ListError::NotMigrated`, with [`LIST_HINT`].
pub fn not_migrated(path: &Path, error: &str) -> CliError {
    CliError::new(
        exit::CONFIG,
        format!(
            "cannot migrate Go spela's profile {}: {error}",
            path.display()
        ),
    )
    .with_hint(SOURCE_HINT)
}

/// Refuses a write to a profile whose Go file did not migrate: a game's
/// `<appid>.yaml`, or `default.yaml` for All games. Exit 3, naming the file.
pub fn refuse_profile(report: &Report, scope: Scope) -> Result<(), CliError> {
    let kind = match scope {
        Scope::AllGames => SourceKind::AllGames,
        Scope::Game(app_id) => SourceKind::Game(app_id),
    };
    match failed(report, kind) {
        Some(h) => Err(not_migrated(&h.entry.path, &h.entry.error)),
        None => Ok(()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::clock::ManualClock;

    fn clock() -> ManualClock {
        ManualClock::new("2026-10-26T17:04:11Z".parse().unwrap())
    }

    fn write(path: &Path, text: &str) {
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, text).unwrap();
    }

    #[test]
    fn nothing_to_do_prints_nothing() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let mut err = Vec::new();
        let report = run(&paths, &clock(), &mut err, true).unwrap();
        assert!(report.is_empty());
        assert!(err.is_empty());
    }

    #[test]
    fn the_notice_goes_to_stderr_once() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        write(&paths.go().config(), "log_level: debug\n");
        let mut err = Vec::new();
        run(&paths, &clock(), &mut err, true).unwrap();
        let text = String::from_utf8(err).unwrap();
        assert!(text.starts_with("Migrated 1 file from Go spela"), "{text}");
        let mut again = Vec::new();
        run(&paths, &clock(), &mut again, true).unwrap();
        assert!(again.is_empty(), "the second run is quiet");
        let mut quiet = Vec::new();
        write(&paths.go().game_profile(9100010), "name: X\n");
        run(&paths, &clock(), &mut quiet, false).unwrap();
        assert!(quiet.is_empty(), "no notice when asked for none");
    }

    #[test]
    fn a_broken_go_config_exits_3_with_go_s_advice() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        write(&paths.go().config(), "log_level: [\n");
        let e = run(&paths, &clock(), &mut Vec::new(), true).unwrap_err();
        assert_eq!(e.code, exit::CONFIG);
        assert!(e.message.contains("config.yaml"), "{}", e.message);
        assert_eq!(e.hint.as_deref(), Some(CONFIG_HINT));
    }

    #[test]
    fn writes_to_a_failed_source_are_refused() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        write(&paths.go().game_profile(9100030), "name: [\n");
        let report = run(&paths, &clock(), &mut Vec::new(), true).unwrap();
        let e = refuse_profile(&report, Scope::Game(9100030)).unwrap_err();
        assert_eq!(e.code, exit::CONFIG);
        assert!(e.message.contains("9100030.yaml"), "{}", e.message);
        assert_eq!(e.hint.as_deref(), Some(SOURCE_HINT));
        assert!(refuse_profile(&report, Scope::Game(9100010)).is_ok());
        assert!(refuse_profile(&report, Scope::AllGames).is_ok());
    }
}
