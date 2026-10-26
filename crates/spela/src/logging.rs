//! Logs: a rolling file for TUI and wrapper runs, stderr for the CLI
//! (decision 12).
//!
//! - The TUI logs only to `<state>/log/spela.log.<date>`, because anything
//!   on the terminal would corrupt the screen.
//! - The wrapper logs to the file and also sends warnings to stderr, which
//!   Steam captures.
//! - CLI commands send warnings to stderr and write no file.
//!
//! Files roll daily, the last [`KEEP_FILES`] are kept, and they never hold
//! ANSI codes. stderr gets ANSI only when it is a terminal and `NO_COLOR` is
//! unset.
//!
//! The level comes from `--log-level`, then `SPELA_LOG` (full
//! `tracing_subscriber::EnvFilter` syntax, such as `spela=debug`), then the
//! default: info for the file and warn for stderr. The TUI puts the
//! config's `log_level` between `SPELA_LOG` and the default through
//! [`LevelHandle`]: at start and after each save in Settings, as Go applies
//! it after each save (parity-inventory.md, TUI 10). The filter is parsed
//! from the [`Env`] snapshot, never read from the process environment.

use std::io::Write;
use std::path::Path;

use spela_core::env::{Env, var};
use spela_core::paths::create_private_dir;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::{EnvFilter, Layer, Registry, reload};

/// The file level when nothing sets one.
pub const DEFAULT_FILE_LEVEL: &str = "info";
/// The stderr level when nothing sets one.
pub const DEFAULT_STDERR_LEVEL: &str = "warn";
/// Log file name prefix.
pub const FILE_PREFIX: &str = "spela.log";
/// Daily log files kept.
pub const KEEP_FILES: usize = 7;

/// Where a run's logs go.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LogMode {
    /// The TUI: the file only.
    File,
    /// The wrapper: the file, plus warnings on stderr.
    FileAndStderr,
    /// CLI commands: stderr only.
    Stderr,
}

/// Errors setting up logging.
#[derive(Debug, thiserror::Error)]
pub enum LogError {
    /// The filter text is not valid.
    #[error("invalid log filter {filter:?}: {reason}")]
    Filter {
        /// What was given.
        filter: String,
        /// Why it failed.
        reason: String,
    },
    /// The log folder cannot be created.
    #[error("cannot create log folder {}: {source}", path.display())]
    Dir {
        /// The folder.
        path: std::path::PathBuf,
        /// The cause.
        source: std::io::Error,
    },
    /// The rolling appender cannot start.
    #[error("cannot open the log file: {0}")]
    Appender(String),
}

/// Keeps the background file writer alive. Lines logged just before exit
/// are flushed when this drops, so `main` holds it to the end.
#[derive(Debug)]
pub struct LogGuard {
    _file: Option<tracing_appender::non_blocking::WorkerGuard>,
    /// Changes the file's level while the process runs.
    pub level: LevelHandle,
}

/// Sets the log file's level to the config's `log_level` while the process
/// runs. It does nothing when `--log-level` or `SPELA_LOG` chose the level,
/// because they come first, or when there is no file.
#[derive(Debug, Clone, Default)]
pub struct LevelHandle {
    file: Option<reload::Handle<EnvFilter, Registry>>,
    explicit: bool,
}

impl LevelHandle {
    /// A reloadable filter for the file layer, starting at `filter`, and
    /// its handle. `explicit` is whether a flag or `SPELA_LOG` chose it.
    pub fn reloadable(
        filter: EnvFilter,
        explicit: bool,
    ) -> (reload::Layer<EnvFilter, Registry>, LevelHandle) {
        let (layer, handle) = reload::Layer::new(filter);
        (
            layer,
            LevelHandle {
                file: Some(handle),
                explicit,
            },
        )
    }

    /// Sets the file's level, such as `debug`, unless a flag or
    /// `SPELA_LOG` chose it.
    pub fn apply(&self, level: &str) {
        let Some(handle) = &self.file else {
            return;
        };
        if self.explicit {
            return;
        }
        match parse_filter(level) {
            Ok(filter) => {
                if let Err(e) = handle.reload(filter) {
                    tracing::warn!("cannot change the log level: {e}");
                }
            }
            Err(e) => tracing::warn!("{e}"),
        }
    }
}

/// The filter spec chosen by the precedence rules, or `None` for the
/// default.
pub fn chosen_spec<'a>(cli_level: Option<&'a str>, env: &'a Env) -> Option<&'a str> {
    cli_level
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .or_else(|| env.non_empty(var::SPELA_LOG))
}

/// Parses a filter spec.
pub fn parse_filter(spec: &str) -> Result<EnvFilter, LogError> {
    EnvFilter::builder()
        .parse(spec)
        .map_err(|e| LogError::Filter {
            filter: spec.to_owned(),
            reason: e.to_string(),
        })
}

/// Installs logging for the whole process. Problems are reported on `err`
/// and logging carries on without the part that failed, so a broken log
/// folder never stops a launch.
pub fn init(
    mode: LogMode,
    cli_level: Option<&str>,
    env: &Env,
    log_dir: &Path,
    stderr_color: bool,
    err: &mut dyn Write,
) -> LogGuard {
    let spec = chosen_spec(cli_level, env);
    let filter_for = |default: &str, err: &mut dyn Write| match spec.map(parse_filter) {
        Some(Ok(f)) => f,
        Some(Err(e)) => {
            let _ = writeln!(err, "warning: {e}; using {default}");
            EnvFilter::new(default)
        }
        None => EnvFilter::new(default),
    };

    let (file_layer, guard, level) = match mode {
        LogMode::Stderr => (None, None, LevelHandle::default()),
        LogMode::File | LogMode::FileAndStderr => match file_writer(log_dir) {
            Ok((writer, guard)) => {
                let (filter, level) =
                    LevelHandle::reloadable(filter_for(DEFAULT_FILE_LEVEL, err), spec.is_some());
                let layer = tracing_subscriber::fmt::layer()
                    .with_ansi(false)
                    .with_target(true)
                    .with_thread_names(true)
                    .with_writer(writer)
                    .with_filter(filter);
                (Some(layer), Some(guard), level)
            }
            Err(e) => {
                let _ = writeln!(err, "warning: {e}; logging to a file is off");
                (None, None, LevelHandle::default())
            }
        },
    };

    let stderr_layer = match mode {
        LogMode::File => None,
        LogMode::FileAndStderr => Some(stderr_layer(
            stderr_color,
            EnvFilter::new(DEFAULT_STDERR_LEVEL),
        )),
        LogMode::Stderr => Some(stderr_layer(
            stderr_color,
            filter_for(DEFAULT_STDERR_LEVEL, err),
        )),
    };

    let subscriber = tracing_subscriber::registry()
        .with(file_layer)
        .with(stderr_layer);
    // A second install fails and the first one stays, which is what tests
    // that call this twice want.
    let _ = tracing::subscriber::set_global_default(subscriber);
    LogGuard {
        _file: guard,
        level,
    }
}

fn stderr_layer<S>(color: bool, filter: EnvFilter) -> impl Layer<S>
where
    S: tracing::Subscriber + for<'a> tracing_subscriber::registry::LookupSpan<'a>,
{
    tracing_subscriber::fmt::layer()
        .with_ansi(color)
        .with_target(false)
        .without_time()
        .with_writer(std::io::stderr)
        .with_filter(filter)
}

/// The daily rolling file writer, with the folder made private.
pub fn file_writer(
    log_dir: &Path,
) -> Result<
    (
        tracing_appender::non_blocking::NonBlocking,
        tracing_appender::non_blocking::WorkerGuard,
    ),
    LogError,
> {
    create_private_dir(log_dir).map_err(|source| LogError::Dir {
        path: log_dir.to_owned(),
        source,
    })?;
    let appender = tracing_appender::rolling::Builder::new()
        .rotation(tracing_appender::rolling::Rotation::DAILY)
        .filename_prefix(FILE_PREFIX)
        .max_log_files(KEEP_FILES)
        .build(log_dir)
        .map_err(|e| LogError::Appender(e.to_string()))?;
    Ok(tracing_appender::non_blocking(appender))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn flag_beats_env_beats_default() {
        let env = Env::default().with(var::SPELA_LOG, "spela=trace");
        assert_eq!(chosen_spec(Some("debug"), &env), Some("debug"));
        assert_eq!(chosen_spec(None, &env), Some("spela=trace"));
        assert_eq!(chosen_spec(None, &Env::default()), None);
        assert_eq!(chosen_spec(Some(" "), &Env::default()), None);
        let blank = Env::default().with(var::SPELA_LOG, "  ");
        assert_eq!(chosen_spec(None, &blank), None);
    }

    #[test]
    fn bad_filters_are_errors() {
        assert!(parse_filter("debug").is_ok());
        assert!(parse_filter("spela=trace,spela_core=debug").is_ok());
        let err = parse_filter("spela=notalevel").unwrap_err();
        assert!(err.to_string().contains("notalevel"), "{err}");
    }

    #[test]
    fn file_lines_have_no_ansi_and_the_folder_is_private() {
        use std::os::unix::fs::PermissionsExt;
        let dir = tempfile::tempdir().unwrap();
        let log_dir = dir.path().join("state/spela/log");
        let (writer, guard) = file_writer(&log_dir).unwrap();
        let layer = tracing_subscriber::fmt::layer()
            .with_ansi(false)
            .with_writer(writer)
            .with_filter(parse_filter("info").unwrap());
        let subscriber = tracing_subscriber::registry().with(layer);
        tracing::subscriber::with_default(subscriber, || {
            tracing::warn!(area = "hardware", "restore failed");
            tracing::debug!("filtered out");
        });
        drop(guard);
        let mut text = String::new();
        for entry in std::fs::read_dir(&log_dir).unwrap() {
            let entry = entry.unwrap();
            assert!(
                entry.file_name().to_string_lossy().starts_with(FILE_PREFIX),
                "{:?}",
                entry.file_name()
            );
            text.push_str(&std::fs::read_to_string(entry.path()).unwrap());
        }
        assert!(text.contains("restore failed"), "{text}");
        assert!(text.contains("area=\"hardware\""), "{text}");
        assert!(!text.contains("filtered out"));
        assert!(!text.contains('\x1b'), "no ANSI in files: {text:?}");
        let mode = std::fs::metadata(&log_dir).unwrap().permissions().mode();
        assert_eq!(mode & 0o777, 0o700);
    }

    #[test]
    fn the_level_handle_changes_the_file_level_unless_a_flag_chose_it() {
        use std::sync::{Arc, Mutex};
        let lines = Arc::new(Mutex::new(Vec::<u8>::new()));
        let sink = lines.clone();
        let make = move || Sink(sink.clone());
        let (filter, level) = LevelHandle::reloadable(parse_filter("info").unwrap(), false);
        let layer = tracing_subscriber::fmt::layer()
            .with_ansi(false)
            .with_writer(make)
            .with_filter(filter);
        let subscriber = tracing_subscriber::registry().with(layer);
        tracing::subscriber::with_default(subscriber, || {
            tracing::debug!("hidden at info");
            level.apply("debug");
            tracing::debug!("shown at debug");
        });
        let text = String::from_utf8(lines.lock().unwrap().clone()).unwrap();
        assert!(!text.contains("hidden at info"), "{text}");
        assert!(text.contains("shown at debug"), "{text}");
        // A flag or SPELA_LOG wins over the config.
        let (_, fixed) = LevelHandle::reloadable(parse_filter("trace").unwrap(), true);
        fixed.apply("error");
        LevelHandle::default().apply("debug");
    }

    struct Sink(std::sync::Arc<std::sync::Mutex<Vec<u8>>>);

    impl Write for Sink {
        fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
            self.0.lock().unwrap().extend_from_slice(buf);
            Ok(buf.len())
        }

        fn flush(&mut self) -> std::io::Result<()> {
            Ok(())
        }
    }
}
