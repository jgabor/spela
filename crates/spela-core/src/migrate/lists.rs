//! Go's deny list files to TOML.
//!
//! - `lists/overrides.yaml` becomes `lists/overrides.toml` with Go's two
//!   lists. A game in both lists stays allowed, as in Go, and its deny entry
//!   goes in the record (decision 36).
//! - `lists/denylist.yaml` becomes `lists/denylist.toml`, entries as they
//!   are. In Go this file replaced the built-in list; now it adds to it
//!   (decision 36), so the file says so.
//!
//! The shapes are the ones the architecture sets for the deny list module:
//!
//! ```toml
//! version = 1
//! allowed = [1172470]
//!
//! [[denied]]
//! app_id = 252490
//! name = "Rust"
//! reason = "user-specified"
//! ```
//!
//! and `[[entries]]` with the same three keys for `denylist.toml`.

use serde::{Deserialize, Serialize};

use super::go::quote;
use super::record::{Dropped, Why};
use super::yaml::{self, YamlError};

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct EntryYaml {
    app_id: Option<u64>,
    name: Option<String>,
    reason: Option<String>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct OverridesYaml {
    allowed: Option<Vec<u64>>,
    denied: Option<Vec<EntryYaml>>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct DenyListYaml {
    entries: Option<Vec<EntryYaml>>,
}

#[derive(Debug, Serialize)]
struct Entry {
    app_id: u64,
    name: String,
    reason: String,
}

impl From<EntryYaml> for Entry {
    fn from(e: EntryYaml) -> Entry {
        Entry {
            app_id: e.app_id.unwrap_or(0),
            name: e.name.unwrap_or_default(),
            reason: e.reason.unwrap_or_default(),
        }
    }
}

#[derive(Debug, Serialize)]
struct OverridesToml {
    version: i64,
    allowed: Vec<u64>,
    denied: Vec<Entry>,
}

#[derive(Debug, Serialize)]
struct DenyListToml {
    version: i64,
    entries: Vec<Entry>,
}

/// A converted list file: its TOML text without the header, and what was
/// not carried over.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Converted {
    /// The TOML text.
    pub text: String,
    /// What was not carried over.
    pub dropped: Vec<Dropped>,
}

/// Converts Go's `overrides.yaml`.
pub fn overrides(text: &str) -> Result<Converted, YamlError> {
    let go: OverridesYaml = yaml::from_str(text)?;
    let allowed = go.allowed.unwrap_or_default();
    let mut denied = Vec::new();
    let mut dropped = Vec::new();
    for entry in go.denied.unwrap_or_default().into_iter().map(Entry::from) {
        if allowed.contains(&entry.app_id) {
            dropped.push(Dropped::new(
                format!("denied.{}", entry.app_id),
                format!(
                    "{{ name = {}, reason = {} }}",
                    quote(&entry.name),
                    quote(&entry.reason)
                ),
                Why::AllowedAndDenied,
                false,
            ));
        } else {
            denied.push(entry);
        }
    }
    let out = OverridesToml {
        version: 1,
        allowed,
        denied,
    };
    Ok(Converted {
        text: toml::to_string(&out).map_err(|e| yaml_like(e.to_string()))?,
        dropped,
    })
}

/// Converts Go's `denylist.yaml`.
pub fn denylist(text: &str) -> Result<Converted, YamlError> {
    let go: DenyListYaml = yaml::from_str(text)?;
    let out = DenyListToml {
        version: 1,
        entries: go
            .entries
            .unwrap_or_default()
            .into_iter()
            .map(Entry::from)
            .collect(),
    };
    Ok(Converted {
        text: toml::to_string(&out).map_err(|e| yaml_like(e.to_string()))?,
        dropped: Vec::new(),
    })
}

/// A TOML encoding failure, which only an app id above `i64::MAX` causes,
/// reported like a bad source file.
fn yaml_like(message: String) -> YamlError {
    yaml::error(message)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn lines(parts: &[&str]) -> String {
        let mut text = parts.join("\n");
        text.push('\n');
        text
    }

    #[test]
    fn allow_wins_and_the_deny_entry_is_recorded() {
        let text = lines(&[
            "allowed:",
            "    - 1172470",
            "    - 9100203",
            "denied:",
            "    - app_id: 9100201",
            "      name: Frostline Arena",
            "      reason: kernel anti-cheat",
            "    - app_id: 9100203",
            "      name: Both Lists",
            "      reason: user-specified",
        ]);
        let c = overrides(&text).unwrap();
        assert_eq!(
            c.text,
            lines(&[
                "version = 1",
                "allowed = [1172470, 9100203]",
                "",
                "[[denied]]",
                "app_id = 9100201",
                "name = \"Frostline Arena\"",
                "reason = \"kernel anti-cheat\"",
            ])
        );
        assert_eq!(c.dropped.len(), 1);
        assert_eq!(c.dropped[0].key, "denied.9100203");
        assert_eq!(
            c.dropped[0].value,
            "{ name = \"Both Lists\", reason = \"user-specified\" }"
        );
    }

    #[test]
    fn empty_lists_stay_empty() {
        let c = overrides("allowed: []\ndenied: []\n").unwrap();
        assert_eq!(c.text, "version = 1\nallowed = []\ndenied = []\n");
        let d = denylist(&lines(&[
            "entries:",
            "    - app_id: 9100201",
            "      name: Frostline Arena",
            "      reason: kernel anti-cheat",
        ]))
        .unwrap();
        assert!(
            d.text.contains("[[entries]]\napp_id = 9100201\n"),
            "{}",
            d.text
        );
        assert!(overrides("allowed: [x\n").is_err());
    }
}
