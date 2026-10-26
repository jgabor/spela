//! Writing `config.toml`: `config set` and Settings change one key with
//! toml_edit under `config.lock`, keeping comments, order and unknown keys.
//!
//! The write reads the file fresh under the lock and refuses when it does
//! not load or has a newer `version`. The value is checked by
//! [`Config::set`], so the texts are Go's. The new text is parsed again
//! before it replaces the file atomically with fsync.

use std::io;
use std::path::{Path, PathBuf};

use toml_edit::{Array, DocumentMut, Item, Table, TableLike, Value};

use super::{CONFIG_VERSION, Config, ConfigError, ConfigSetError, OptionKind, option, parse};
use crate::fs::{Durability, write_atomic};
use crate::lock::{LockError, StoreLock};
use crate::paths::Paths;

/// Why a setting could not be saved.
#[derive(Debug, thiserror::Error)]
pub enum ConfigStoreError {
    /// The key or the value is not valid. Go's texts.
    #[error(transparent)]
    Set(#[from] ConfigSetError),
    /// The file on disk does not load, so it is not written over.
    #[error(transparent)]
    Config(#[from] ConfigError),
    /// The file has a newer `version`, so this spela does not change it.
    #[error("config file {} was written by a newer spela, so this spela does not change it", path.display())]
    ReadOnly {
        /// The file.
        path: PathBuf,
    },
    /// The file or its folder cannot be written.
    #[error("cannot write config file {}: {source}", path.display())]
    Io {
        /// The file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
    /// `config.lock` could not be taken.
    #[error(transparent)]
    Lock(#[from] LockError),
}

/// Sets one setting by its `config set` key, such as `check_updates` or
/// `ui.theme`, and saves it. Returns the config as saved. A missing file is
/// created with `version = 1` and the one key.
pub fn set(paths: &Paths, key: &str, raw: &str) -> Result<Config, ConfigStoreError> {
    let _lock = StoreLock::acquire(&paths.config_lock())?;
    set_unlocked(&paths.config_file(), key, raw)
}

fn set_unlocked(path: &Path, key: &str, raw: &str) -> Result<Config, ConfigStoreError> {
    let io_error = |source| ConfigStoreError::Io {
        path: path.to_owned(),
        source,
    };
    let loaded = super::load(path)?;
    if loaded.read_only {
        return Err(ConfigStoreError::ReadOnly {
            path: path.to_owned(),
        });
    }
    let mut config = loaded.config;
    config.set(key, raw)?;
    let option = option(key).expect("set checked the key");

    let mut doc: DocumentMut = if loaded.from_file {
        std::fs::read_to_string(path)
            .map_err(io_error)?
            .parse()
            .map_err(|e: toml_edit::TomlError| ConfigError::Invalid {
                path: Some(path.to_owned()),
                errors: vec![super::Diagnostic {
                    line: 0,
                    column: 0,
                    message: e.message().to_owned(),
                }],
            })?
    } else {
        DocumentMut::new()
    };
    if !doc.contains_key("version") {
        doc.insert("version", toml_edit::value(i64::from(CONFIG_VERSION)));
    }
    let new = toml_value(&config, option.key);
    let (table, leaf): (&mut dyn TableLike, &str) = match option.key.split_once('.') {
        Some((table, leaf)) => {
            if doc.get(table).and_then(Item::as_table_like).is_none() {
                doc.insert(table, Item::Table(Table::new()));
            }
            let table = doc
                .get_mut(table)
                .and_then(Item::as_table_like_mut)
                .expect("the table was just made");
            (table, leaf)
        }
        None => (doc.as_table_mut(), option.key),
    };
    match table.get_mut(leaf).and_then(Item::as_value_mut) {
        Some(old) => {
            let decor = old.decor().clone();
            *old = new;
            *old.decor_mut() = decor;
        }
        None => {
            table.insert(leaf, Item::Value(new));
        }
    }

    let text = doc.to_string();
    let saved = parse(&text).map_err(|e| match e {
        ConfigError::Invalid { errors, .. } => ConfigError::Invalid {
            path: Some(path.to_owned()),
            errors,
        },
        other => other,
    })?;
    if let Some(dir) = path.parent() {
        std::fs::create_dir_all(dir).map_err(io_error)?;
    }
    write_atomic(path, text.as_bytes(), Durability::Durable).map_err(io_error)?;
    Ok(saved.config)
}

/// The TOML value of one setting, by its `config set` key.
pub(crate) fn toml_value(config: &Config, key: &str) -> Value {
    match key {
        "additional_library_paths" => {
            let mut list = Array::new();
            for path in &config.additional_library_paths {
                list.push(path.as_str());
            }
            Value::Array(list)
        }
        "manifest_refresh_hours" => config.manifest_refresh_hours.into(),
        _ => {
            let text = config.get(key).expect("every option has a value");
            match option(key).map(|o| o.kind) {
                Some(OptionKind::Bool) => (text == "true").into(),
                _ => text.into(),
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::{LogLevel, Theme};

    fn paths() -> (tempfile::TempDir, Paths) {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        (base, paths)
    }

    fn lines(parts: &[&str]) -> String {
        let mut text = parts.join("\n");
        text.push('\n');
        text
    }

    #[test]
    fn a_missing_file_is_created_with_the_one_key() {
        let (_base, paths) = paths();
        let saved = set(&paths, "check_updates", "off").unwrap();
        assert!(!saved.check_updates);
        assert_eq!(
            std::fs::read_to_string(paths.config_file()).unwrap(),
            "version = 1\ncheck_updates = false\n"
        );
        set(&paths, "ui.theme", "neon").unwrap();
        set(&paths, "additional_library_paths", "/a:/b c").unwrap();
        set(&paths, "manifest_refresh_hours", "48").unwrap();
        let loaded = crate::config::load(&paths.config_file()).unwrap();
        assert_eq!(loaded.config.ui.theme, Theme::Neon);
        assert_eq!(loaded.config.additional_library_paths, ["/a", "/b c"]);
        assert_eq!(loaded.config.manifest_refresh_hours, 48);
        assert!(!loaded.config.check_updates);
        assert!(loaded.warnings.is_empty(), "{:?}", loaded.warnings);
    }

    #[test]
    fn a_set_keeps_comments_order_and_unknown_keys() {
        let (_base, paths) = paths();
        std::fs::create_dir_all(&paths.config_dir).unwrap();
        let text = lines(&[
            "# spela settings",
            "version = 1",
            "log_level = \"info\"   # quiet",
            "my_key = 3",
            "",
            "[ui]",
            "# The colours.",
            "theme = \"mono\"",
        ]);
        std::fs::write(paths.config_file(), &text).unwrap();
        let saved = set(&paths, "log_level", "debug").unwrap();
        assert_eq!(saved.log_level, LogLevel::Debug);
        set(&paths, "ui.mouse", "no").unwrap();
        assert_eq!(
            std::fs::read_to_string(paths.config_file()).unwrap(),
            lines(&[
                "# spela settings",
                "version = 1",
                "log_level = \"debug\"   # quiet",
                "my_key = 3",
                "",
                "[ui]",
                "# The colours.",
                "theme = \"mono\"",
                "mouse = false",
            ])
        );
    }

    #[test]
    fn a_bad_value_or_file_writes_nothing() {
        let (_base, paths) = paths();
        assert_eq!(
            set(&paths, "compact_mode", "true").unwrap_err().to_string(),
            "unknown config key: compact_mode"
        );
        assert_eq!(
            set(&paths, "backup_path", "rel").unwrap_err().to_string(),
            "backup_path must be an absolute path: rel"
        );
        assert!(!paths.config_file().exists());
        std::fs::create_dir_all(&paths.config_dir).unwrap();
        std::fs::write(paths.config_file(), "log_level = \"loud\"\n").unwrap();
        let err = set(&paths, "check_updates", "true").unwrap_err();
        assert!(matches!(err, ConfigStoreError::Config(_)), "{err}");
        std::fs::write(paths.config_file(), "version = 2\n").unwrap();
        let err = set(&paths, "check_updates", "true").unwrap_err();
        assert!(matches!(err, ConfigStoreError::ReadOnly { .. }), "{err}");
        assert_eq!(
            std::fs::read_to_string(paths.config_file()).unwrap(),
            "version = 2\n"
        );
    }
}
