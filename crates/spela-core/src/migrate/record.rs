//! The migration record, `<data>/migration.toml`: one entry per Go source,
//! with what became of it and every value it did not carry over.
//!
//! ```toml
//! version = 1
//!
//! [[source]]
//! path = "/home/me/.config/spela/profiles/1091500.yaml"
//! sha256 = "9f2c..."
//! target = "/home/me/.config/spela/profiles/1091500.toml"
//! result = "migrated"
//! at = 2026-10-26T17:04:11Z
//! by = "spela 0.7.0"
//!
//! [[source.dropped]]
//! key = "gpu.clock_offset"
//! value = "150"
//! why = "offset_meaning_changed"
//! ```
//!
//! A source listed as migrated or skipped is never read again. A failed
//! source is tried again on every start. The record is user intent: losing
//! it could bring back a deleted profile, so it is written whole, atomically
//! and with fsync, under `config.lock`.

use std::fmt;
use std::io;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::fs::{Durability, write_atomic};

/// The record format version.
pub const RECORD_VERSION: i64 = 1;

/// What became of one Go source.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Outcome {
    /// Its target was written, or already existed and was adopted.
    Migrated,
    /// Go never used it, so there is nothing to migrate.
    Skipped,
    /// It could not be read or written. It is tried again on every start.
    Failed,
}

impl Outcome {
    /// The record's word: `migrated`, `skipped` or `failed`.
    pub fn as_str(self) -> &'static str {
        match self {
            Outcome::Migrated => "migrated",
            Outcome::Skipped => "skipped",
            Outcome::Failed => "failed",
        }
    }
}

/// Why a value was not carried over.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Why {
    /// A field or key spela 0.7 removed: the overlay fields,
    /// `gpu.power_mizer` and `cpu.affinity` (decision 10), and
    /// `compact_mode` (decision 3).
    Retired,
    /// Go's `gpu.clock_offset` and `gpu.memory_offset` were clock locks;
    /// the fields now mean real offsets (decision 31).
    OffsetMeaningChanged,
    /// Go's GUI `theme` (decision 46).
    GuiOnly,
    /// `show_hints` and `preferred_dll_source`, which Go already ignored
    /// (decision 46).
    RetiredKey,
    /// A key Go spela did not know, so it ignored it.
    UnknownKey,
    /// A game profile value with no `true` marker, which had no effect in
    /// Go.
    NoMarker,
    /// A marker for a field Go did not know.
    UnknownMarker,
    /// A value Go ignored at use and replaced with its default, such as an
    /// unknown log level or a relative folder.
    IgnoredByGo,
    /// A config value equal to Go's default, which carries no choice
    /// (decision 43).
    Default,
    /// A value spela cannot hold, such as SMT marked with no value.
    NotSupported,
    /// The All games profile's `name`, which spela does not keep.
    AllGamesName,
    /// A deny entry for a game that is also allowed. Allow wins
    /// (decision 36).
    AllowedAndDenied,
}

impl Why {
    /// The words the file header uses.
    pub fn describe(self) -> &'static str {
        match self {
            Why::Retired => "removed in spela 0.7",
            Why::OffsetMeaningChanged => "Go locked clocks here; the field is now a real offset",
            Why::GuiOnly => "the GUI theme is gone",
            Why::RetiredKey => "Go spela no longer used it",
            Why::UnknownKey => "Go spela did not know this key",
            Why::NoMarker => "had no effect in Go spela",
            Why::UnknownMarker => "Go spela did not know this field",
            Why::IgnoredByGo => "Go spela ignored it and used the default",
            Why::Default => "Go spela's default",
            Why::NotSupported => "spela cannot store it",
            Why::AllGamesName => "All games has no name",
            Why::AllowedAndDenied => "the game is also allowed, and allow wins",
        }
    }
}

/// One value that a migration did not carry over.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Dropped {
    /// The key, such as `overlay.enabled` or `theme`.
    pub key: String,
    /// The value as Go held it, JSON-like: `true`, `150`, `"0-7"`, or
    /// `none` for SMT's nil.
    pub value: String,
    /// Why it was not carried over.
    pub why: Why,
    /// Whether the value is Go's default or zero. The notice counts only
    /// values that are not.
    #[serde(default)]
    pub default: bool,
    /// More detail, when it helps.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub note: String,
}

impl Dropped {
    /// A dropped value.
    pub fn new(
        key: impl Into<String>,
        value: impl Into<String>,
        why: Why,
        default: bool,
    ) -> Dropped {
        Dropped {
            key: key.into(),
            value: value.into(),
            why,
            default,
            note: String::new(),
        }
    }

    /// Adds a note.
    pub fn with_note(mut self, note: impl Into<String>) -> Dropped {
        self.note = note.into();
        self
    }
}

impl fmt::Display for Dropped {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} = {} ({}", self.key, self.value, self.why.describe())?;
        if !self.note.is_empty() {
            write!(f, ": {}", self.note)?;
        }
        f.write_str(")")
    }
}

/// One Go source in the record.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Source {
    /// The Go file.
    pub path: PathBuf,
    /// The SHA-256 of the Go file's bytes when it was read. Empty when it
    /// could not be read.
    #[serde(default)]
    pub sha256: String,
    /// The file it became, when there is one.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<PathBuf>,
    /// What became of it.
    pub result: Outcome,
    /// When, in UTC.
    pub at: toml::value::Datetime,
    /// The spela that did it, such as `spela 0.7.0`.
    pub by: String,
    /// Why it was skipped, or that its target already existed.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub note: String,
    /// Why it failed.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub error: String,
    /// The values it did not carry over.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub dropped: Vec<Dropped>,
}

impl Source {
    /// Whether this source is settled: migrated or skipped, so never read
    /// again.
    pub fn is_done(&self) -> bool {
        self.result != Outcome::Failed
    }
}

/// The whole record.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Record {
    /// The format version.
    pub version: i64,
    /// One entry per Go source, in the order they were first recorded.
    #[serde(default, rename = "source")]
    pub sources: Vec<Source>,
}

impl Default for Record {
    fn default() -> Record {
        Record {
            version: RECORD_VERSION,
            sources: Vec::new(),
        }
    }
}

/// The record could not be read or written.
#[derive(Debug, thiserror::Error)]
pub enum RecordError {
    /// The file cannot be read or written.
    #[error("migration record {}: {source}", path.display())]
    Io {
        /// The file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
    /// The file is not a valid record. spela does not write over it, because
    /// it says which Go files were already migrated.
    #[error("migration record {} does not load: {message}", path.display())]
    Invalid {
        /// The file.
        path: PathBuf,
        /// What is wrong.
        message: String,
    },
}

impl Record {
    /// Reads the record. A missing file is an empty record.
    pub fn load(path: &Path) -> Result<Record, RecordError> {
        match std::fs::read_to_string(path) {
            Ok(text) => toml::from_str(&text).map_err(|e| RecordError::Invalid {
                path: path.to_owned(),
                message: e.message().trim().to_owned(),
            }),
            Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(Record::default()),
            Err(source) => Err(RecordError::Io {
                path: path.to_owned(),
                source,
            }),
        }
    }

    /// The entry for a Go file.
    pub fn get(&self, path: &Path) -> Option<&Source> {
        self.sources.iter().find(|s| s.path == path)
    }

    /// Whether a Go file is settled and must not be read again.
    pub fn is_done(&self, path: &Path) -> bool {
        self.get(path).is_some_and(Source::is_done)
    }

    /// Records a source, replacing its old entry in place. Returns whether
    /// anything changed. A failed source that fails again the same way
    /// keeps its old entry, so a run that changes nothing writes nothing.
    pub fn put(&mut self, source: Source) -> bool {
        match self.sources.iter_mut().find(|s| s.path == source.path) {
            Some(old) => {
                let same_failure = old.result == Outcome::Failed
                    && source.result == Outcome::Failed
                    && old.error == source.error
                    && old.sha256 == source.sha256;
                if same_failure || *old == source {
                    return false;
                }
                *old = source;
                true
            }
            None => {
                self.sources.push(source);
                true
            }
        }
    }

    /// Writes the record whole, atomically and with fsync. The folder is
    /// created when missing.
    pub fn save(&self, path: &Path) -> Result<(), RecordError> {
        let io_error = |source| RecordError::Io {
            path: path.to_owned(),
            source,
        };
        let text = toml::to_string(self).map_err(|e| RecordError::Invalid {
            path: path.to_owned(),
            message: e.to_string(),
        })?;
        if let Some(dir) = path.parent() {
            std::fs::create_dir_all(dir).map_err(io_error)?;
        }
        write_atomic(path, text.as_bytes(), Durability::Durable).map_err(io_error)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn source(path: &str, result: Outcome, error: &str) -> Source {
        Source {
            path: PathBuf::from(path),
            sha256: "ab".into(),
            target: Some(PathBuf::from("/c/profiles/1.toml")),
            result,
            at: "2026-10-26T17:04:11Z".parse().unwrap(),
            by: "spela 0.7.0".into(),
            note: String::new(),
            error: error.into(),
            dropped: vec![
                Dropped::new("gpu.clock_offset", "150", Why::OffsetMeaningChanged, false),
                Dropped::new("cpu.smt", "none", Why::NotSupported, false).with_note("why"),
            ],
        }
    }

    #[test]
    fn the_record_round_trips_in_its_documented_shape() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("data/migration.toml");
        let mut record = Record::load(&path).unwrap();
        assert!(record.sources.is_empty());
        assert!(record.put(source("/c/profiles/1.yaml", Outcome::Migrated, "")));
        record.save(&path).unwrap();
        let text = std::fs::read_to_string(&path).unwrap();
        assert!(text.starts_with("version = 1\n"), "{text}");
        assert!(text.contains("[[source]]\n"), "{text}");
        assert!(text.contains("result = \"migrated\"\n"), "{text}");
        assert!(text.contains("at = 2026-10-26T17:04:11Z\n"), "{text}");
        assert!(text.contains("[[source.dropped]]\n"), "{text}");
        assert!(
            text.contains("why = \"offset_meaning_changed\"\n"),
            "{text}"
        );
        let back = Record::load(&path).unwrap();
        assert_eq!(back, record);
        assert!(back.is_done(Path::new("/c/profiles/1.yaml")));
        assert!(!back.is_done(Path::new("/c/profiles/2.yaml")));
    }

    #[test]
    fn the_same_failure_twice_changes_nothing() {
        let mut record = Record::default();
        assert!(record.put(source("/a.yaml", Outcome::Failed, "bad")));
        let mut again = source("/a.yaml", Outcome::Failed, "bad");
        again.at = "2026-10-27T00:00:00Z".parse().unwrap();
        assert!(!record.put(again));
        assert!(record.put(source("/a.yaml", Outcome::Failed, "worse")));
        assert!(record.put(source("/a.yaml", Outcome::Migrated, "")));
        assert_eq!(record.sources.len(), 1);
        assert!(record.is_done(Path::new("/a.yaml")));
    }

    #[test]
    fn a_broken_record_is_an_error_not_an_empty_one() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("migration.toml");
        std::fs::write(&path, "[[source]\n").unwrap();
        assert!(matches!(
            Record::load(&path),
            Err(RecordError::Invalid { .. })
        ));
    }

    #[test]
    fn dropped_values_read_plainly() {
        let d = Dropped::new("overlay.enabled", "true", Why::Retired, false);
        assert_eq!(
            d.to_string(),
            "overlay.enabled = true (removed in spela 0.7)"
        );
        let d = d.with_note("x");
        assert_eq!(
            d.to_string(),
            "overlay.enabled = true (removed in spela 0.7: x)"
        );
    }
}
