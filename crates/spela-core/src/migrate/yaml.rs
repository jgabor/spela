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
//! - Only the first document is read, as yaml.v3's `Unmarshal` reads it, so
//!   a second `---` document is ignored, not an error.
//! - A Go `int` leaf takes a float, cut toward zero, as yaml.v3 does
//!   ([`go_int`]), so `power_limit: 250.0` reads as 250.

use std::fmt;

use serde::Deserializer;
use serde::de::{self, DeserializeOwned, Visitor};

/// YAML that does not parse into the expected shape.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct YamlError(String);

impl fmt::Display for YamlError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for YamlError {}

/// Reads the first YAML document. An empty document reads as the target's
/// default where the target allows it.
pub fn from_str<T: DeserializeOwned>(text: &str) -> Result<T, YamlError> {
    serde_saphyr::from_str(first_document(text)).map_err(|e| YamlError(first_line(&e.to_string())))
}

/// The text of the first document. yaml.v3 decodes only the first one.
/// A document ends at a line that starts the next one (`---`) or ends it
/// (`...`), at column 0, as YAML marks them; a `---` before any content
/// starts the first one. Go's writer never puts either inside a document.
fn first_document(text: &str) -> &str {
    let marker = |line: &str, m: &str| {
        line == m
            || line
                .strip_prefix(m)
                .is_some_and(|r| r.starts_with([' ', '\t']))
    };
    let mut offset = 0;
    let mut content = false;
    for line in text.split_inclusive('\n') {
        let bare = line.trim_end_matches(['\n', '\r']);
        let start = marker(bare, "---");
        if (start && content) || marker(bare, "...") {
            return &text[..offset];
        }
        let trimmed = bare.trim_start();
        if start || !(trimmed.is_empty() || trimmed.starts_with('#') || bare.starts_with('%')) {
            content = true;
        }
        offset += line.len();
    }
    text
}

/// Reads a Go `int` leaf as yaml.v3 does: a whole number, or a float cut
/// toward zero when it fits (`decode.go`, `reflect.Int`). Use it with
/// `#[serde(default, deserialize_with = "...")]` on an `Option<i64>`.
pub fn go_int<'de, D: Deserializer<'de>>(deserializer: D) -> Result<Option<i64>, D::Error> {
    struct GoInt;
    impl<'de> Visitor<'de> for GoInt {
        type Value = Option<i64>;
        fn expecting(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
            f.write_str("an integer")
        }
        fn visit_none<E: de::Error>(self) -> Result<Self::Value, E> {
            Ok(None)
        }
        fn visit_unit<E: de::Error>(self) -> Result<Self::Value, E> {
            Ok(None)
        }
        fn visit_some<D: Deserializer<'de>>(self, d: D) -> Result<Self::Value, D::Error> {
            d.deserialize_any(GoInt)
        }
        fn visit_i64<E: de::Error>(self, v: i64) -> Result<Self::Value, E> {
            Ok(Some(v))
        }
        fn visit_u64<E: de::Error>(self, v: u64) -> Result<Self::Value, E> {
            i64::try_from(v)
                .map(Some)
                .map_err(|_| E::custom(format!("{v} does not fit an integer")))
        }
        fn visit_f64<E: de::Error>(self, v: f64) -> Result<Self::Value, E> {
            // i64::MAX as f64 rounds up to 2^63, so the bound is exclusive.
            if v.is_finite() && v >= i64::MIN as f64 && v < i64::MAX as f64 {
                Ok(Some(v.trunc() as i64))
            } else {
                Err(E::custom(format!("{v} does not fit an integer")))
            }
        }
    }
    deserializer.deserialize_option(GoInt)
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
        #[serde(deserialize_with = "go_int")]
        go_number: Option<i64>,
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
    fn only_the_first_document_is_read() {
        let p = probe("text: one\n---\ntext: two\n").unwrap();
        assert_eq!(p.text.as_deref(), Some("one"));
        let p = probe("---\ntext: one\n...\nnot: [yaml\n").unwrap();
        assert_eq!(p.text.as_deref(), Some("one"), "the rest is not read");
        let p = probe("# notes\n---\ntext: one\n--- \ntext: two\n").unwrap();
        assert_eq!(p.text.as_deref(), Some("one"));
        let p = probe("text: |\n    a\n    ---b\nflag: true\n").unwrap();
        assert_eq!(
            p.text.as_deref(),
            Some("a\n---b\n"),
            "an indented --- is text"
        );
        assert_eq!(p.flag, Some(true));
    }

    #[test]
    fn go_int_leaves_take_floats_cut_toward_zero() {
        for (raw, want) in [
            ("250", Some(250)),
            ("250.0", Some(250)),
            ("250.9", Some(250)),
            ("-100.5", Some(-100)),
            ("1e3", Some(1000)),
            ("~", None),
            ("", None),
        ] {
            assert_eq!(
                probe(&format!("go_number: {raw}\n")).unwrap().go_number,
                want,
                "{raw}"
            );
        }
        assert_eq!(probe("text: x\n").unwrap().go_number, None);
        for bad in ["high", "\"250\"", ".nan", ".inf", "1e30", "true"] {
            assert!(probe(&format!("go_number: {bad}\n")).is_err(), "{bad}");
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
