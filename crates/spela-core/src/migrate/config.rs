//! Go's `config.yaml` to `config.toml`.
//!
//! Go wrote every key on each save, so the file cannot show which values the
//! user chose, and a default carries no choice. A key is written only when
//! its value differs from Go's default; every other Go key goes in the
//! record (decision 43). A value Go ignored at use, such as an unknown log
//! level, a relative folder or an unusable manifest URL, is recorded
//! instead of written, because Go used the default and spela would refuse
//! the file. Go's `theme`, `compact_mode`, the retired keys and unknown keys
//! are recorded (decisions 3 and 46). The `[ui]` keys start at their
//! defaults.

use std::path::{Path, PathBuf};

use serde::Deserialize;
use toml_edit::{Array, DocumentMut};

use crate::config::{LogLevel, check_manifest_url};
use crate::paths::Paths;

use super::go::{json_text, quote};
use super::record::{Dropped, Why};
use super::yaml::{self, YamlError};

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct ConfigYaml {
    log_level: Option<String>,
    shader_cache: Option<String>,
    check_updates: Option<bool>,
    rescan_on_startup: Option<bool>,
    auto_update_dlls: Option<bool>,
    steam_path: Option<String>,
    additional_library_paths: Option<Vec<String>>,
    dll_cache_path: Option<String>,
    backup_path: Option<String>,
    dll_manifest_url: Option<String>,
    auto_refresh_manifest: Option<bool>,
    manifest_refresh_hours: Option<i64>,
    theme: Option<String>,
    compact_mode: Option<bool>,
    confirm_destructive: Option<bool>,
}

/// The keys of Go's config struct.
const GO_KEYS: [&str; 15] = [
    "log_level",
    "shader_cache",
    "check_updates",
    "rescan_on_startup",
    "auto_update_dlls",
    "steam_path",
    "additional_library_paths",
    "dll_cache_path",
    "backup_path",
    "dll_manifest_url",
    "auto_refresh_manifest",
    "manifest_refresh_hours",
    "theme",
    "compact_mode",
    "confirm_destructive",
];

/// A Go config in spela's terms.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Converted {
    /// The `config.toml` text without its header.
    pub text: String,
    /// The Go keys not written, defaults included.
    pub dropped: Vec<Dropped>,
}

/// One kept setting, ready to write.
enum Kept {
    Bool(bool),
    Int(i64),
    Text(String),
    List(Vec<String>),
}

/// Converts the text of Go's `config.yaml`. `paths` gives Go's computed
/// default for `shader_cache` and the default folders.
pub fn convert(text: &str, paths: &Paths) -> Result<Converted, YamlError> {
    let go: ConfigYaml = yaml::from_str(text)?;
    let mut kept: Vec<(&str, Kept)> = Vec::new();
    let mut dropped = Vec::new();
    let mut default = |key: &str, value: String| {
        dropped.push(Dropped::new(key, value, Why::Default, true));
    };

    let log_level = go.log_level.unwrap_or_else(|| "info".to_owned());
    let mut ignored = Vec::new();
    match LogLevel::from_name(&log_level) {
        Some(LogLevel::Info) => default("log_level", quote(&log_level)),
        Some(_) => kept.push(("log_level", Kept::Text(log_level))),
        None => ignored.push(("log_level", quote(&log_level), "unsupported log level")),
    }

    for (key, value, go_default) in [
        ("check_updates", go.check_updates, true),
        ("rescan_on_startup", go.rescan_on_startup, true),
        ("auto_update_dlls", go.auto_update_dlls, false),
        ("confirm_destructive", go.confirm_destructive, true),
        ("auto_refresh_manifest", go.auto_refresh_manifest, true),
    ] {
        let value = value.unwrap_or(go_default);
        if value == go_default {
            default(key, value.to_string());
        } else {
            kept.push((key, Kept::Bool(value)));
        }
    }

    let steam_path = go.steam_path.unwrap_or_default();
    if steam_path.is_empty() {
        default("steam_path", quote(""));
    } else {
        kept.push(("steam_path", Kept::Text(steam_path)));
    }

    let libraries = go.additional_library_paths.unwrap_or_default();
    if libraries.is_empty() {
        default("additional_library_paths", "[]".to_owned());
    } else {
        kept.push(("additional_library_paths", Kept::List(libraries)));
    }

    for (key, value, default_dir) in [
        (
            "dll_cache_path",
            go.dll_cache_path.unwrap_or_default(),
            paths.default_dll_cache_dir(),
        ),
        (
            "backup_path",
            go.backup_path.unwrap_or_default(),
            paths.backups_dir(),
        ),
        (
            "shader_cache",
            go.shader_cache
                .unwrap_or_else(|| paths.default_shader_cache_dir().display().to_string()),
            paths.default_shader_cache_dir(),
        ),
    ] {
        if value.is_empty() || clean(&value) == Some(default_dir) {
            default(key, quote(&value));
        } else if Path::new(&value).is_absolute() {
            kept.push((key, Kept::Text(value)));
        } else {
            ignored.push((key, quote(&value), "not an absolute path"));
        }
    }

    let url = go.dll_manifest_url.unwrap_or_default();
    if url.is_empty() {
        default("dll_manifest_url", quote(""));
    } else if check_manifest_url(&url).is_ok() {
        kept.push(("dll_manifest_url", Kept::Text(url)));
    } else {
        ignored.push(("dll_manifest_url", quote(&url), "not an https URL"));
    }

    let hours = go.manifest_refresh_hours.unwrap_or(24);
    if hours == 24 {
        default("manifest_refresh_hours", hours.to_string());
    } else if hours > 0 {
        kept.push(("manifest_refresh_hours", Kept::Int(hours)));
    } else {
        ignored.push((
            "manifest_refresh_hours",
            hours.to_string(),
            "Go used 24 hours",
        ));
    }

    for (key, value, note) in ignored {
        dropped.push(Dropped::new(key, value, Why::IgnoredByGo, false).with_note(note));
    }
    let theme = go.theme.unwrap_or_else(|| "default".to_owned());
    dropped.push(Dropped::new(
        "theme",
        quote(&theme),
        Why::GuiOnly,
        theme == "default",
    ));
    let compact = go.compact_mode.unwrap_or(false);
    dropped.push(Dropped::new(
        "compact_mode",
        compact.to_string(),
        Why::Retired,
        !compact,
    ));
    for (key, value) in unknown_keys(text) {
        let why = if key == "show_hints" || key == "preferred_dll_source" {
            Why::RetiredKey
        } else {
            Why::UnknownKey
        };
        dropped.push(Dropped::new(key, value, why, false));
    }

    // Written in the order of the architecture's key list.
    let order = [
        "log_level",
        "check_updates",
        "rescan_on_startup",
        "auto_update_dlls",
        "confirm_destructive",
        "steam_path",
        "additional_library_paths",
        "dll_cache_path",
        "backup_path",
        "shader_cache",
        "dll_manifest_url",
        "auto_refresh_manifest",
        "manifest_refresh_hours",
    ];
    kept.sort_by_key(|(k, _)| order.iter().position(|o| o == k));
    let mut doc = DocumentMut::new();
    doc.insert("version", toml_edit::value(1));
    for (key, value) in kept {
        let value: toml_edit::Value = match value {
            Kept::Bool(b) => b.into(),
            Kept::Int(n) => n.into(),
            Kept::Text(t) => t.into(),
            Kept::List(list) => {
                let mut array = Array::new();
                for item in list {
                    array.push(item);
                }
                array.into()
            }
        };
        doc.insert(key, toml_edit::Item::Value(value));
    }
    Ok(Converted {
        text: doc.to_string(),
        dropped,
    })
}

/// `filepath.Clean` of an absolute path; `None` for a relative one.
fn clean(value: &str) -> Option<PathBuf> {
    let path = Path::new(value);
    path.is_absolute().then(|| path.components().collect())
}

/// Top-level keys Go's config struct has no place for, with their values.
fn unknown_keys(text: &str) -> Vec<(String, String)> {
    let Ok(serde_json::Value::Object(root)) = yaml::from_str::<serde_json::Value>(text) else {
        return Vec::new();
    };
    root.iter()
        .filter(|(k, _)| !GO_KEYS.contains(&k.as_str()))
        .map(|(k, v)| (k.clone(), json_text(v)))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn lines(parts: &[&str]) -> String {
        let mut text = parts.join("\n");
        text.push('\n');
        text
    }

    fn paths() -> Paths {
        Paths::under(Path::new("/b"))
    }

    #[test]
    fn defaults_write_only_the_version() {
        let text = lines(&[
            "log_level: info",
            "shader_cache: /b/cache/nvidia",
            "check_updates: true",
            "rescan_on_startup: true",
            "auto_update_dlls: false",
            "auto_refresh_manifest: true",
            "manifest_refresh_hours: 24",
            "theme: default",
            "compact_mode: false",
            "confirm_destructive: true",
        ]);
        let c = convert(&text, &paths()).unwrap();
        assert_eq!(c.text, "version = 1\n");
        assert!(c.dropped.iter().all(|d| d.default), "{:?}", c.dropped);
        let keys: Vec<&str> = c.dropped.iter().map(|d| d.key.as_str()).collect();
        for key in GO_KEYS {
            assert!(keys.contains(&key), "{key} is recorded");
        }
    }

    #[test]
    fn values_go_ignored_are_recorded_not_written() {
        let text = lines(&[
            "log_level: verbose",
            "dll_cache_path: cache/dlls",
            "dll_manifest_url: http://example.com/m.json",
            "manifest_refresh_hours: 0",
            "theme: light",
            "show_hints: true",
            "fancy_mode: 3",
        ]);
        let c = convert(&text, &paths()).unwrap();
        assert_eq!(c.text, "version = 1\n");
        let notable: Vec<(&str, Why)> = c
            .dropped
            .iter()
            .filter(|d| !d.default)
            .map(|d| (d.key.as_str(), d.why))
            .collect();
        assert_eq!(
            notable,
            [
                ("log_level", Why::IgnoredByGo),
                ("dll_cache_path", Why::IgnoredByGo),
                ("dll_manifest_url", Why::IgnoredByGo),
                ("manifest_refresh_hours", Why::IgnoredByGo),
                ("theme", Why::GuiOnly),
                ("fancy_mode", Why::UnknownKey),
                ("show_hints", Why::RetiredKey),
            ]
        );
    }

    #[test]
    fn a_folder_equal_to_the_default_is_a_default() {
        let c = convert(
            "backup_path: /b/data/backups/\nshader_cache: \"\"\n",
            &paths(),
        )
        .unwrap();
        assert_eq!(c.text, "version = 1\n");
    }
}
