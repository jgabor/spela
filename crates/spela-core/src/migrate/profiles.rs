//! Go profiles to own values: which leaves a scope set in Go, and what
//! becomes of each.
//!
//! - A game sets a field when its key is marked `true`, with its stored
//!   value even when that is zero. A leaf with no marker had no effect.
//! - All games sets a field when it is marked or not zero (Go's
//!   `setsField`), so a non-nil `smt: false` counts.
//!
//! A set field is kept as an own value, except that decision 10 drops the
//! retired fields, decision 31 drops Go's clock and memory offsets, and
//! decision 65 drops a value outside Go's own rules: an `sr_mode`,
//! `rr_mode` or `multi_frame` not in the field's list, or a relative
//! `gpu.shader_cache_path`. Each drop is recorded. SMT marked with no value
//! is kept: it is stored as `smt = "unchanged"` (decision 65), so the game
//! still leaves SMT alone when All games sets it.
//!
//! A DLSS preset that is not one of spela's, such as `quality`, is not
//! dropped: Go's `dlssPresetToEnv` exported `render_preset_default` for any
//! preset it did not know (`internal/profile/apply.go`), so it is stored as
//! `default`, which exports the same, and the rewrite is recorded. The
//! launch then stays as it was, whatever All games sets.

use crate::profile::{FieldError, FieldKey, Kind, OwnValues, Value};

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
            Some(Value::Text(preset))
                if is_preset(field)
                    && field.field().check(&Value::Text(preset.clone())).is_err() =>
            {
                values
                    .set(field, Value::Text("default".to_owned()))
                    .expect("default is a preset");
                dropped.push(Dropped::new(
                    key,
                    leaf.display(),
                    Why::StoredAsDefault,
                    false,
                ));
                continue;
            }
            Some(value) => values.set(field, value).map(|_| ()),
            None => Err(FieldError::WrongKind {
                key: field.as_str(),
                expected: field.field().kind,
            }),
        };
        if let Err(e) = result {
            let (why, note) = match e {
                FieldError::Unsupported { valid, .. } => {
                    let mut listed: Vec<&str> =
                        valid.iter().copied().filter(|v| !v.is_empty()).collect();
                    if valid.contains(&"") {
                        listed.push("empty");
                    }
                    (
                        Why::InvalidInGo,
                        format!(
                            "Go's setters accept only {}; Go passed it on at launch as it was",
                            listed.join(", ")
                        ),
                    )
                }
                FieldError::NotAbsolute { .. } => (
                    Why::InvalidInGo,
                    "a relative folder, where Go's docs ask for an absolute one; Go passed it on at launch as it was".to_owned(),
                ),
                other => (Why::NotSupported, other.to_string()),
            };
            let default = scope == GoScope::AllGames && leaf.is_zero();
            dropped.push(Dropped::new(key, leaf.display(), why, default).with_note(note));
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

/// Whether a field is one of the two DLSS presets.
fn is_preset(field: FieldKey) -> bool {
    matches!(field, FieldKey::DlssSrPreset | FieldKey::DlssRrPreset)
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
    fn smt_marked_with_no_value_is_kept_as_unchanged() {
        let go = parse_profile(&lines(&["overrides:", "    cpu.smt: true"])).unwrap();
        for scope in [GoScope::Game, GoScope::AllGames] {
            let c = convert(&go, scope, &[]);
            assert_eq!(
                c.values.get(FieldKey::CpuSmt),
                Some(&Value::OptionalBool(None)),
                "{scope:?}"
            );
            assert!(c.dropped.is_empty(), "{scope:?}: {:?}", c.dropped);
        }
    }

    #[test]
    fn values_outside_go_s_own_rules_are_dropped_as_invalid_in_go() {
        let text = lines(&[
            "dlss:",
            "    sr_mode: fast",
            "    rr_mode: Quality",
            "    multi_frame: 7",
            "gpu:",
            "    shader_cache: true",
            "    shader_cache_path: shaders/cache",
            "overrides:",
            "    dlss.sr_mode: true",
            "    dlss.rr_mode: true",
            "    dlss.multi_frame: true",
            "    gpu.shader_cache: true",
            "    gpu.shader_cache_path: true",
        ]);
        let go = parse_profile(&text).unwrap();
        for scope in [GoScope::Game, GoScope::AllGames] {
            let c = convert(&go, scope, &[]);
            let own: Vec<FieldKey> = c.values.iter().map(|(k, _)| k).collect();
            assert_eq!(own, [FieldKey::GpuShaderCache], "{scope:?}");
            assert_eq!(
                keys(&c.dropped),
                [
                    ("dlss.sr_mode", Why::InvalidInGo),
                    ("dlss.rr_mode", Why::InvalidInGo),
                    ("dlss.multi_frame", Why::InvalidInGo),
                    ("gpu.shader_cache_path", Why::InvalidInGo),
                ],
                "{scope:?}"
            );
            let values: Vec<&str> = c.dropped.iter().map(|d| d.value.as_str()).collect();
            assert_eq!(
                values,
                ["\"fast\"", "\"Quality\"", "7", "\"shaders/cache\""]
            );
            assert!(
                c.dropped.iter().all(|d| !d.default),
                "the notice counts them"
            );
            assert_eq!(
                c.dropped[2].note,
                "Go's setters accept only 0, 1, 2, 3, 4; Go passed it on at launch as it was"
            );
            assert_eq!(
                c.dropped[0].note,
                "Go's setters accept only off, ultra_performance, performance, balanced, quality, dlaa, empty; Go passed it on at launch as it was"
            );
        }
    }

    #[test]
    fn presets_go_did_not_know_are_stored_as_default() {
        // Go exported render_preset_default for each of these.
        let go = parse_profile(&lines(&[
            "dlss:",
            "    sr_preset: quality",
            "    rr_preset: auto",
            "overrides:",
            "    dlss.sr_preset: true",
            "    dlss.rr_preset: true",
        ]))
        .unwrap();
        let c = convert(&go, GoScope::Game, &[]);
        for key in [FieldKey::DlssSrPreset, FieldKey::DlssRrPreset] {
            assert_eq!(c.values.get(key), Some(&Value::Text("default".into())));
        }
        assert_eq!(
            keys(&c.dropped),
            [
                ("dlss.sr_preset", Why::StoredAsDefault),
                ("dlss.rr_preset", Why::StoredAsDefault)
            ]
        );
        assert_eq!(c.dropped[0].value, "\"quality\"");
        // A preset spela knows is kept as it is.
        let go = parse_profile(&lines(&[
            "dlss:",
            "    sr_preset: K",
            "overrides:",
            "    dlss.sr_preset: true",
        ]))
        .unwrap();
        let c = convert(&go, GoScope::Game, &[]);
        assert_eq!(
            c.values.get(FieldKey::DlssSrPreset),
            Some(&Value::Text("K".into()))
        );
        assert!(c.dropped.is_empty());
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
