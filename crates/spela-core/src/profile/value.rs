//! Field values and the one bool vocabulary.

use std::fmt;

use serde::Serialize;

/// The kind of a field's value. It fixes the type of every value of the
/// field, in TOML and in JSON.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Kind {
    /// `true` or `false`.
    Bool,
    /// A whole number.
    Int,
    /// Text.
    #[serde(rename = "string")]
    Text,
    /// `true`, `false` or none. Only SMT has this kind. None leaves SMT
    /// unchanged at launch. It is the system default, and a scope may also
    /// store it as its own value, `smt = "unchanged"` (decision 65), so a
    /// game can leave SMT alone even when All games sets it.
    OptionalBool,
}

impl Kind {
    /// The JSON `kind` word: `bool`, `int`, `string` or `optional_bool`.
    pub fn name(self) -> &'static str {
        match self {
            Kind::Bool => "bool",
            Kind::Int => "int",
            Kind::Text => "string",
            Kind::OptionalBool => "optional_bool",
        }
    }

    /// The zero of the kind, which is every field's system default: Off, 0,
    /// empty or none.
    pub fn zero(self) -> Value {
        match self {
            Kind::Bool => Value::Bool(false),
            Kind::Int => Value::Int(0),
            Kind::Text => Value::Text(String::new()),
            Kind::OptionalBool => Value::OptionalBool(None),
        }
    }

    /// The words of Go's kind error, such as `boolean`.
    pub fn describe(self) -> &'static str {
        match self {
            Kind::Bool => "boolean",
            Kind::Int => "integer",
            Kind::Text => "string",
            Kind::OptionalBool => "optional boolean",
        }
    }
}

/// One field's value. In JSON it is the raw value: `true`, `150`, `"auto"`,
/// or `null` for SMT's none.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize)]
#[serde(untagged)]
pub enum Value {
    /// A [`Kind::Bool`] value.
    Bool(bool),
    /// A [`Kind::Int`] value.
    Int(i64),
    /// A [`Kind::Text`] value. Empty is a value, not "follows".
    Text(String),
    /// A [`Kind::OptionalBool`] value. `None` leaves SMT unchanged: the
    /// system default, or an own value stored as [`SMT_UNCHANGED`].
    OptionalBool(Option<bool>),
}

/// How a profile file stores SMT's none as an own value: `smt =
/// "unchanged"`. TOML has no null (decision 65).
pub const SMT_UNCHANGED: &str = "unchanged";

impl Value {
    /// The value's kind.
    pub fn kind(&self) -> Kind {
        match self {
            Value::Bool(_) => Kind::Bool,
            Value::Int(_) => Kind::Int,
            Value::Text(_) => Kind::Text,
            Value::OptionalBool(_) => Kind::OptionalBool,
        }
    }

    /// The value as Go's `fmt.Sprint` writes it, which is what allowed
    /// lists are checked against: `true`, `4`, `auto`, and `""` for none.
    pub fn canonical(&self) -> String {
        match self {
            Value::Bool(b) | Value::OptionalBool(Some(b)) => b.to_string(),
            Value::Int(n) => n.to_string(),
            Value::Text(t) => t.clone(),
            Value::OptionalBool(None) => String::new(),
        }
    }

    /// Whether this is the zero of its kind, as Go's `reflect.IsZero` sees
    /// a field's value. For SMT, Off counts as zero too, because Go
    /// compares the bool behind the pointer
    /// (`internal/profile/explanation.go:209-211`).
    pub fn is_zero(&self) -> bool {
        match self {
            Value::Bool(b) => !b,
            Value::Int(n) => *n == 0,
            Value::Text(t) => t.is_empty(),
            Value::OptionalBool(b) => !b.unwrap_or(false),
        }
    }

    /// The bool, for a [`Kind::Bool`] value.
    pub fn as_bool(&self) -> Option<bool> {
        match self {
            Value::Bool(b) => Some(*b),
            _ => None,
        }
    }

    /// The number, for a [`Kind::Int`] value.
    pub fn as_int(&self) -> Option<i64> {
        match self {
            Value::Int(n) => Some(*n),
            _ => None,
        }
    }

    /// The text, for a [`Kind::Text`] value.
    pub fn as_text(&self) -> Option<&str> {
        match self {
            Value::Text(t) => Some(t),
            _ => None,
        }
    }

    /// The optional bool, for a [`Kind::OptionalBool`] value.
    pub fn as_optional_bool(&self) -> Option<Option<bool>> {
        match self {
            Value::OptionalBool(b) => Some(*b),
            _ => None,
        }
    }
}

impl fmt::Display for Value {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.canonical())
    }
}

/// spela's one bool vocabulary, for every flag, editor and config value:
/// `true`, `1`, `yes`, `on` and `false`, `0`, `no`, `off`, in any case.
/// Surrounding spaces are ignored.
pub fn parse_bool(raw: &str) -> Option<bool> {
    match raw.trim().to_ascii_lowercase().as_str() {
        "true" | "1" | "yes" | "on" => Some(true),
        "false" | "0" | "no" | "off" => Some(false),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn one_bool_vocabulary() {
        for raw in ["true", "TRUE", "True", "1", "yes", "Yes", "on", " ON "] {
            assert_eq!(parse_bool(raw), Some(true), "{raw}");
        }
        for raw in ["false", "FALSE", "0", "no", "NO", "off", "Off"] {
            assert_eq!(parse_bool(raw), Some(false), "{raw}");
        }
        for raw in ["", "y", "n", "2", "enable", "default"] {
            assert_eq!(parse_bool(raw), None, "{raw}");
        }
    }

    #[test]
    fn json_values_are_raw() {
        let json = |v: Value| serde_json::to_string(&v).unwrap();
        assert_eq!(json(Value::Bool(true)), "true");
        assert_eq!(json(Value::Int(150)), "150");
        assert_eq!(json(Value::Text("auto".into())), "\"auto\"");
        assert_eq!(json(Value::OptionalBool(None)), "null");
        assert_eq!(json(Value::OptionalBool(Some(false))), "false");
        assert_eq!(serde_json::to_string(&Kind::Text).unwrap(), "\"string\"");
        assert_eq!(
            serde_json::to_string(&Kind::OptionalBool).unwrap(),
            "\"optional_bool\""
        );
    }

    #[test]
    fn zero_matches_go() {
        assert!(Value::OptionalBool(Some(false)).is_zero());
        assert!(Value::OptionalBool(None).is_zero());
        assert!(!Value::OptionalBool(Some(true)).is_zero());
        assert!(Value::Text(String::new()).is_zero());
        assert!(!Value::Int(-1).is_zero());
        for kind in [Kind::Bool, Kind::Int, Kind::Text, Kind::OptionalBool] {
            assert!(kind.zero().is_zero());
            assert_eq!(kind.zero().kind(), kind);
        }
    }
}
