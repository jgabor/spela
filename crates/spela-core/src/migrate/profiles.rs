//! Go profiles to own values: which leaves a scope set in Go, and what
//! becomes of each.
//!
//! - A game sets a field when its key is marked `true`, with its stored
//!   value even when that is zero. A leaf with no marker had no effect.
//! - All games sets a field when it is marked or not zero (Go's
//!   `setsField`), so a non-nil `smt: false` counts.
//!
//! A set field is kept as an own value, except that decision 10 drops the
//! retired fields, decision 31 drops Go's clock and memory offsets, and a
//! value spela cannot hold is dropped with a note. Each drop is recorded.

use crate::profile::{FieldKey, Kind, OwnValues, Value};

use super::go::{GO_FIELDS, GoProfile, GoValue, is_go_field};
use super::record::{Dropped, Why};

/// A Go profile in spela's terms.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Converted {
    /// The game's name; `None` for All games.
    pub name: Option<String>,
    /// The own values to write.
    pub values: OwnValues,
    /// What was not carried over.
    pub dropped: Vec<Dropped>,
}

/// Which profile a Go file is.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GoScope {
    /// `default.yaml`.
    AllGames,
    /// `<appid>.yaml`, after legacy inheritance.
    Game,
}

/// Converts a loaded Go profile. `unknown_keys` are the file's keys Go had
/// no place for, from [`super::go::unknown_profile_keys`].
pub fn convert(go: &GoProfile, scope: GoScope, unknown_keys: &[(String, String)]) -> Converted {
    let mut values = OwnValues::new();
    let mut dropped = Vec::new();
    for (key, _) in GO_FIELDS {
        let leaf = go.leaf(key);
        let set = match scope {
            GoScope::Game => go.is_marked(key),
            GoScope::AllGames => go.sets_field(key),
        };
        if !set {
            if scope == GoScope::Game && !leaf.is_zero() {
                dropped.push(
                    Dropped::new(key, leaf.display(), Why::NoMarker, false)
                        .with_note("Go applied the All games value instead"),
                );
            }
            continue;
        }
        let Some(field) = FieldKey::from_key(key) else {
            dropped.push(Dropped::new(
                key,
                leaf.display(),
                Why::Retired,
                leaf.is_zero(),
            ));
            continue;
        };
        if matches!(field, FieldKey::GpuClockOffset | FieldKey::GpuMemoryOffset) {
            dropped.push(Dropped::new(
                key,
                leaf.display(),
                Why::OffsetMeaningChanged,
                leaf.is_zero(),
            ));
            continue;
        }
        let result = match to_value(field, leaf) {
            Some(value) => values.set(field, value).map(|_| ()),
            None => Err(crate::profile::FieldError::WrongKind {
                key: field.as_str(),
                expected: field.field().kind,
            }),
        };
        if let Err(e) = result {
            let note = match (field, scope) {
                (FieldKey::CpuSmt, GoScope::Game) => {
                    "Go left SMT unchanged for this game; it now follows All games".to_owned()
                }
                (FieldKey::CpuSmt, GoScope::AllGames) => {
                    "none is the default, so nothing changes".to_owned()
                }
                _ => e.to_string(),
            };
            let default = scope == GoScope::AllGames && leaf.is_zero();
            dropped.push(
                Dropped::new(key, leaf.display(), Why::NotSupported, default).with_note(note),
            );
        }
    }
    for (marker, on) in go.overrides.iter().flatten() {
        if !is_go_field(marker) {
            dropped.push(
                Dropped::new(marker, on.to_string(), Why::UnknownMarker, !on)
                    .with_note("a marker in `overrides`"),
            );
        }
    }
    for (key, value) in unknown_keys {
        dropped.push(Dropped::new(key, value, Why::UnknownKey, false));
    }
    let name = match scope {
        GoScope::Game => Some(go.name.clone()),
        GoScope::AllGames => {
            if !go.name.is_empty() {
                dropped.push(Dropped::new(
                    "name",
                    super::go::quote(&go.name),
                    Why::AllGamesName,
                    false,
                ));
            }
            None
        }
    };
    Converted {
        name,
        values,
        dropped,
    }
}

/// A Go leaf as the value of a kept field.
fn to_value(field: FieldKey, leaf: &GoValue) -> Option<Value> {
    Some(match (field.field().kind, leaf) {
        (Kind::Bool, GoValue::Bool(b)) => Value::Bool(*b),
        (Kind::Int, GoValue::Int(n)) => Value::Int(*n),
        (Kind::Text, GoValue::Text(t)) => Value::Text(t.clone()),
        (Kind::OptionalBool, GoValue::Smt(b)) => Value::OptionalBool(*b),
        _ => return None,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::migrate::go::{migrate_inheritance, parse_profile};

    fn lines(parts: &[&str]) -> String {
        let mut text = parts.join("\n");
        text.push('\n');
        text
    }

    fn keys(dropped: &[Dropped]) -> Vec<(&str, Why)> {
        dropped.iter().map(|d| (d.key.as_str(), d.why)).collect()
    }

    #[test]
    fn a_game_keeps_marked_values_even_when_zero() {
        let go = parse_profile(&lines(&[
            "name: Rook Harbor",
            "gpu:",
            "    power_limit: 180",
            "    clock_offset: 150",
            "proton:",
            "    enable_hdr: true",
            "overlay:",
            "    enabled: true",
            "overrides:",
            "    gpu.power_limit: true",
            "    gpu.clock_offset: true",
            "    gpu.threaded_optimization: true",
            "    proton.enable_hdr: false",
            "    overlay.enabled: true",
            "    made.up: true",
        ]))
        .unwrap();
        let c = convert(&go, GoScope::Game, &[]);
        assert_eq!(c.name.as_deref(), Some("Rook Harbor"));
        let own: Vec<(FieldKey, Value)> = c.values.iter().map(|(k, v)| (k, v.clone())).collect();
        assert_eq!(
            own,
            [
                (FieldKey::GpuPowerLimit, Value::Int(180)),
                (FieldKey::GpuThreadedOptimization, Value::Bool(false)),
            ]
        );
        assert_eq!(
            keys(&c.dropped),
            [
                ("proton.enable_hdr", Why::NoMarker),
                ("gpu.clock_offset", Why::OffsetMeaningChanged),
                ("overlay.enabled", Why::Retired),
                ("made.up", Why::UnknownMarker),
            ]
        );
    }

    #[test]
    fn all_games_sets_what_is_marked_or_not_zero() {
        let go = parse_profile(&lines(&[
            "name: Shared",
            "cpu:",
            "    smt: false",
            "dlss:",
            "    multi_frame: 3",
            "overrides:",
            "    gpu.shader_cache: true",
            "    cpu.governor: false",
        ]))
        .unwrap();
        let c = convert(&go, GoScope::AllGames, &[]);
        assert_eq!(c.name, None);
        assert_eq!(c.values.len(), 3);
        assert_eq!(
            c.values.get(FieldKey::CpuSmt),
            Some(&Value::OptionalBool(Some(false)))
        );
        assert_eq!(
            c.values.get(FieldKey::GpuShaderCache),
            Some(&Value::Bool(false))
        );
        assert_eq!(keys(&c.dropped), [("name", Why::AllGamesName)]);
    }

    #[test]
    fn values_spela_cannot_hold_are_recorded() {
        let go = parse_profile(&lines(&[
            "dlss:",
            "    rr_preset: auto",
            "overrides:",
            "    cpu.smt: true",
            "    dlss.rr_preset: true",
        ]))
        .unwrap();
        let c = convert(&go, GoScope::Game, &[]);
        assert!(c.values.is_empty());
        assert_eq!(
            keys(&c.dropped),
            [
                ("dlss.rr_preset", Why::NotSupported),
                ("cpu.smt", Why::NotSupported)
            ]
        );
        assert_eq!(c.dropped[1].value, "none");
        assert!(c.dropped[0].note.contains("unsupported value"));
    }

    #[test]
    fn a_legacy_profile_without_all_games_keeps_every_value() {
        let mut go = parse_profile(&lines(&[
            "name: Ember Coast",
            "gpu:",
            "    vrr: unset",
            "    power_limit: 250",
            "cpu:",
            "    smt: false",
        ]))
        .unwrap();
        migrate_inheritance(&mut go, None);
        let c = convert(&go, GoScope::Game, &[]);
        assert_eq!(c.values.len(), 3);
        assert!(c.dropped.is_empty());
    }
}
