//! The one place spela reads YAML.
//!
//! Only the migration reads Go spela's YAML, and nothing writes YAML any
//! more. serde-saphyr is pinned exactly in the workspace and every call to
//! it goes through this module, because it had two major releases in three
//! months. What the migration relies on, each checked by the fixture tests:
//!
//! - 2-space and 4-space files read the same.
//! - YAML 1.1 bools such as `yes`, `on`, `y` and `NO` read into bool targets,
//!   as Go's yaml.v3 reads them.
//! - Duplicate keys and tab indentation are errors, as in yaml.v3.
//! - A missing map key and `{}` stay apart through `Option`.

use std::fmt;

use serde::de::DeserializeOwned;

/// YAML that does not parse into the expected shape.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct YamlError(String);

impl fmt::Display for YamlError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for YamlError {}

/// Reads one YAML document. An empty document reads as the target's
/// default where the target allows it.
pub fn from_str<T: DeserializeOwned>(text: &str) -> Result<T, YamlError> {
    serde_saphyr::from_str(text).map_err(|e| YamlError(first_line(&e.to_string())))
}

/// A failure to report the way a bad source file is reported.
pub fn error(message: impl Into<String>) -> YamlError {
    YamlError(message.into())
}

/// serde-saphyr adds a source snippet after the first line. The record and
/// the CLI keep only the first line, without the `error: ` prefix.
fn first_line(message: &str) -> String {
    let line = message.lines().next().unwrap_or("").trim();
    line.strip_prefix("error: ").unwrap_or(line).to_owned()
}

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;

    use super::*;

    #[derive(Debug, Default, serde::Deserialize)]
    #[serde(default)]
    struct Probe {
        flag: Option<bool>,
        marks: Option<BTreeMap<String, bool>>,
        text: Option<String>,
        number: Option<i64>,
    }

    fn probe(text: &str) -> Result<Probe, YamlError> {
        from_str(text)
    }

    #[test]
    fn yaml_1_1_bools_read_as_go_reads_them() {
        for (word, want) in [
            ("y", true),
            ("Yes", true),
            ("on", true),
            ("TRUE", true),
            ("off", false),
            ("NO", false),
            ("n", false),
        ] {
            assert_eq!(
                probe(&format!("flag: {word}\n")).unwrap().flag,
                Some(want),
                "{word}"
            );
            let marks = probe(&format!("marks:\n    k: {word}\n"))
                .unwrap()
                .marks
                .unwrap();
            assert_eq!(marks["k"], want, "{word}");
        }
    }

    #[test]
    fn an_empty_map_is_not_a_missing_one() {
        assert_eq!(probe("marks: {}\n").unwrap().marks, Some(BTreeMap::new()));
        assert_eq!(probe("flag: true\n").unwrap().marks, None);
        assert_eq!(probe("").unwrap().flag, None);
    }

    #[test]
    fn version_strings_stay_text() {
        for raw in ["310.2.1", "1.3.1.33", "310", "1.0", "0-7"] {
            assert_eq!(
                probe(&format!("text: {raw}\n")).unwrap().text.as_deref(),
                Some(raw)
            );
        }
    }

    #[test]
    fn what_yaml_v3_refuses_is_refused() {
        let err = probe("number: high\n").unwrap_err().to_string();
        assert!(err.starts_with("line 1 column 9"), "{err}");
        assert!(!err.contains('\n'), "{err}");
        assert!(probe("flag: true\nflag: false\n").is_err());
        assert!(probe("marks:\n\tk: true\n").is_err());
        assert!(probe("not: [yaml").is_err());
    }
}
