//! Go spela's profile model and the two in-memory steps Go ran on load.
//!
//! [`GoProfile`] holds what Go's `Profile` struct held after
//! `unmarshalProfileYAML`: all 35 leaves, the name and the `overrides` map.
//! The functions here are ports of Go's own rules, so the migration decides
//! from the same state Go launched from:
//!
//! - `migrateSRPreset`, run on every load (`internal/profile/migrate_sr_preset.go`).
//! - `migrateInheritance`, run on a game profile with no `overrides` key
//!   (`internal/profile/inheritance.go:275-325`).
//! - `setsField`, Go's rule for what All games sets
//!   (`internal/profile/explanation.go:129-138`).
//!
//! Go compares with `reflect.IsZero` on the field itself, so a non-nil
//! `smt: false` is not zero here, unlike [`crate::profile::Value::is_zero`].

use std::collections::BTreeMap;

use serde::Deserialize;

use super::yaml::{self, YamlError};

/// One leaf of Go's profile, typed as Go typed it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum GoValue {
    /// A Go `bool`.
    Bool(bool),
    /// A Go `int`.
    Int(i64),
    /// A Go `string`, or a string type such as `DLSSMode`.
    Text(String),
    /// Go's `*bool` for SMT. `None` is a nil pointer.
    Smt(Option<bool>),
}

impl GoValue {
    /// `reflect.Value.IsZero`: false, 0, empty, or a nil pointer.
    pub fn is_zero(&self) -> bool {
        match self {
            GoValue::Bool(b) => !b,
            GoValue::Int(n) => *n == 0,
            GoValue::Text(t) => t.is_empty(),
            GoValue::Smt(b) => b.is_none(),
        }
    }

    /// The value as the record and the file header write it: JSON-like, so
    /// text is quoted and SMT's nil is `none`.
    pub fn display(&self) -> String {
        match self {
            GoValue::Bool(b) | GoValue::Smt(Some(b)) => b.to_string(),
            GoValue::Int(n) => n.to_string(),
            GoValue::Text(t) => quote(t),
            GoValue::Smt(None) => "none".to_owned(),
        }
    }
}

/// A string in double quotes with JSON escapes, which TOML basic strings
/// share.
pub fn quote(text: &str) -> String {
    serde_json::to_string(text).unwrap_or_else(|_| format!("\"{text}\""))
}

/// The kind of a Go leaf.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GoKind {
    /// `bool`.
    Bool,
    /// `int`.
    Int,
    /// `string`.
    Text,
    /// `*bool`.
    Smt,
}

impl GoKind {
    fn zero(self) -> GoValue {
        match self {
            GoKind::Bool => GoValue::Bool(false),
            GoKind::Int => GoValue::Int(0),
            GoKind::Text => GoValue::Text(String::new()),
            GoKind::Smt => GoValue::Smt(None),
        }
    }
}

/// Go's 35 field keys with their kinds, in Go's descriptor order
/// (`internal/profile/fields.go:61-101`).
pub const GO_FIELDS: [(&str, GoKind); 35] = [
    ("proton.enable_hdr", GoKind::Bool),
    ("proton.enable_wayland", GoKind::Bool),
    ("proton.enable_ngx_updater", GoKind::Bool),
    ("proton.vkd3d_heap", GoKind::Bool),
    ("dlss.sr_mode", GoKind::Text),
    ("dlss.sr_preset", GoKind::Text),
    ("dlss.sr_override", GoKind::Bool),
    ("dlss.rr_mode", GoKind::Text),
    ("dlss.rr_preset", GoKind::Text),
    ("dlss.rr_override", GoKind::Bool),
    ("dlss.fg_enabled", GoKind::Bool),
    ("dlss.fg_override", GoKind::Bool),
    ("dlss.multi_frame", GoKind::Int),
    ("dlss.indicator", GoKind::Bool),
    ("dlss.fg_indicator", GoKind::Bool),
    ("gpu.clock_offset", GoKind::Int),
    ("gpu.vrr", GoKind::Text),
    ("gpu.memory_offset", GoKind::Int),
    ("gpu.power_limit", GoKind::Int),
    ("gpu.fan_speed", GoKind::Int),
    ("gpu.power_mizer", GoKind::Text),
    ("gpu.shader_cache", GoKind::Bool),
    ("gpu.shader_cache_path", GoKind::Text),
    ("gpu.threaded_optimization", GoKind::Bool),
    ("cpu.governor", GoKind::Text),
    ("cpu.smt", GoKind::Smt),
    ("cpu.affinity", GoKind::Text),
    ("overlay.enabled", GoKind::Bool),
    ("overlay.position", GoKind::Text),
    ("overlay.show_fps", GoKind::Bool),
    ("overlay.show_frametime", GoKind::Bool),
    ("overlay.show_cpu", GoKind::Bool),
    ("overlay.show_gpu", GoKind::Bool),
    ("overlay.show_vram", GoKind::Bool),
    ("overlay.toggle_key", GoKind::Text),
];

/// Whether `key` is one of Go's 35 field keys.
pub fn is_go_field(key: &str) -> bool {
    GO_FIELDS.iter().any(|(k, _)| *k == key)
}

/// The values Go's `gpu.vrr` accepted. It is the only field Go checked on
/// load.
const VRR_VALUES: &[&str] = &["", "unset", "automatic", "always", "never"];

/// A Go profile after `unmarshalProfileYAML`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GoProfile {
    /// `name`, empty when the file has none.
    pub name: String,
    /// Every one of the 35 leaves, zero when the file leaves it out.
    pub leaves: BTreeMap<&'static str, GoValue>,
    /// The `overrides` map, `None` when the file has no `overrides` key.
    pub overrides: Option<BTreeMap<String, bool>>,
}

impl GoProfile {
    /// The leaf with this key.
    pub fn leaf(&self, key: &str) -> &GoValue {
        &self.leaves[key]
    }

    /// Go's `IsOverridden`: the key is marked `true`.
    pub fn is_marked(&self, key: &str) -> bool {
        self.overrides
            .as_ref()
            .is_some_and(|m| m.get(key) == Some(&true))
    }

    /// Go's `setsField` for All games: marked, or not zero.
    pub fn sets_field(&self, key: &str) -> bool {
        self.is_marked(key) || !self.leaf(key).is_zero()
    }
}

/// Why a Go profile did not load, in Go's words where Go has them.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum GoLoadError {
    /// The YAML does not parse into Go's shape.
    #[error("{0}")]
    Yaml(#[from] YamlError),
    /// `gpu.vrr` holds a value Go refused.
    #[error("gpu.vrr: unsupported value {value:?} (valid: {})", VRR_VALUES.join(", "))]
    Vrr {
        /// The stored value.
        value: String,
    },
    /// A legacy game profile needs `default.yaml`, which did not load.
    #[error("load default profile for migration: {0}")]
    Default(Box<GoLoadError>),
}

/// Reads a Go profile file as `unmarshalProfileYAML` does: the leaves, the
/// `sr_model_preset` fold and the `gpu.vrr` check. Legacy inheritance is
/// separate, see [`migrate_inheritance`].
pub fn parse_profile(text: &str) -> Result<GoProfile, GoLoadError> {
    let doc: ProfileYaml = yaml::from_str(text)?;
    let mut leaves: BTreeMap<&'static str, GoValue> = GO_FIELDS
        .iter()
        .map(|(k, kind)| (*k, kind.zero()))
        .collect();
    let mut put = |key: &'static str, value: GoValue| {
        leaves.insert(key, value);
    };
    let b = |v: Option<bool>| GoValue::Bool(v.unwrap_or(false));
    let i = |v: Option<i64>| GoValue::Int(v.unwrap_or(0));
    let t = |v: Option<String>| GoValue::Text(v.unwrap_or_default());

    let proton = doc.proton.unwrap_or_default();
    put("proton.enable_hdr", b(proton.enable_hdr));
    put("proton.enable_wayland", b(proton.enable_wayland));
    put("proton.enable_ngx_updater", b(proton.enable_ngx_updater));
    put("proton.vkd3d_heap", b(proton.vkd3d_heap));

    let dlss = doc.dlss.unwrap_or_default();
    put("dlss.sr_mode", t(dlss.sr_mode));
    put("dlss.sr_preset", t(dlss.sr_preset));
    put("dlss.sr_override", b(dlss.sr_override));
    put("dlss.rr_mode", t(dlss.rr_mode));
    put("dlss.rr_preset", t(dlss.rr_preset));
    put("dlss.rr_override", b(dlss.rr_override));
    put("dlss.fg_enabled", b(dlss.fg_enabled));
    put("dlss.fg_override", b(dlss.fg_override));
    put("dlss.multi_frame", i(dlss.multi_frame));
    put("dlss.indicator", b(dlss.indicator));
    put("dlss.fg_indicator", b(dlss.fg_indicator));

    let gpu = doc.gpu.unwrap_or_default();
    let vrr = gpu.vrr.unwrap_or_default();
    if !VRR_VALUES.contains(&vrr.as_str()) {
        return Err(GoLoadError::Vrr { value: vrr });
    }
    put("gpu.vrr", GoValue::Text(vrr));
    put("gpu.shader_cache", b(gpu.shader_cache));
    put("gpu.shader_cache_path", t(gpu.shader_cache_path));
    put("gpu.threaded_optimization", b(gpu.threaded_optimization));
    put("gpu.clock_offset", i(gpu.clock_offset));
    put("gpu.memory_offset", i(gpu.memory_offset));
    put("gpu.power_limit", i(gpu.power_limit));
    put("gpu.power_mizer", t(gpu.power_mizer));
    put("gpu.fan_speed", i(gpu.fan_speed));

    let cpu = doc.cpu.unwrap_or_default();
    put("cpu.governor", t(cpu.governor));
    put("cpu.smt", GoValue::Smt(cpu.smt));
    put("cpu.affinity", t(cpu.affinity));

    let overlay = doc.overlay.unwrap_or_default();
    put("overlay.enabled", b(overlay.enabled));
    put("overlay.position", t(overlay.position));
    put("overlay.show_fps", b(overlay.show_fps));
    put("overlay.show_frametime", b(overlay.show_frametime));
    put("overlay.show_cpu", b(overlay.show_cpu));
    put("overlay.show_gpu", b(overlay.show_gpu));
    put("overlay.show_vram", b(overlay.show_vram));
    put("overlay.toggle_key", t(overlay.toggle_key));

    let overrides = doc.overrides.map(|m| {
        m.into_iter()
            .map(|(k, v)| (k, v.unwrap_or(false)))
            .collect::<BTreeMap<_, _>>()
    });
    let mut profile = GoProfile {
        name: doc.name.unwrap_or_default(),
        leaves,
        overrides,
    };
    migrate_sr_preset(
        &mut profile,
        dlss.sr_model_preset.unwrap_or_default().as_str(),
    );
    Ok(profile)
}

/// Go's `migrateSRPreset`: the retired `sr_model_preset` wins over
/// `sr_preset` when it names a preset, `sr_preset` is normalised, and a
/// `true` marker on the old key moves to `dlss.sr_preset`.
fn migrate_sr_preset(profile: &mut GoProfile, legacy: &str) {
    let mapped = match legacy.trim().to_ascii_lowercase().as_str() {
        "auto" => "auto",
        "k" => "K",
        "l" => "L",
        "m" => "M",
        _ => "",
    };
    if !mapped.is_empty() {
        profile
            .leaves
            .insert("dlss.sr_preset", GoValue::Text(mapped.to_owned()));
    }
    if let GoValue::Text(preset) = profile.leaf("dlss.sr_preset").clone()
        && !preset.is_empty()
    {
        profile.leaves.insert(
            "dlss.sr_preset",
            GoValue::Text(crate::profile::field::normalize_preset(&preset)),
        );
    }
    const LEGACY: &str = "dlss.sr_model_preset";
    if let Some(map) = profile.overrides.as_mut()
        && map.get(LEGACY) == Some(&true)
    {
        map.insert("dlss.sr_preset".to_owned(), true);
        map.remove(LEGACY);
    }
}

/// Go's `migrateInheritance` for a game profile with no `overrides` key:
/// a leaf equal to the All games leaf is zeroed and follows, a leaf that
/// differs is marked. With no All games file every non-zero leaf is
/// marked. `gpu.vrr` is marked whenever it is not empty.
pub fn migrate_inheritance(profile: &mut GoProfile, all_games: Option<&GoProfile>) {
    if profile.overrides.is_some() {
        return;
    }
    let mut marks = BTreeMap::new();
    for (key, kind) in GO_FIELDS {
        let value = profile.leaf(key).clone();
        if key == "gpu.vrr" || all_games.is_none() {
            if !value.is_zero() {
                marks.insert(key.to_owned(), true);
            }
            continue;
        }
        let default = all_games.map(|d| d.leaf(key));
        if default == Some(&value) {
            profile.leaves.insert(key, kind.zero());
        } else {
            marks.insert(key.to_owned(), true);
        }
    }
    profile.overrides = Some(marks);
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct ProfileYaml {
    name: Option<String>,
    dlss: Option<DlssYaml>,
    gpu: Option<GpuYaml>,
    cpu: Option<CpuYaml>,
    proton: Option<ProtonYaml>,
    overlay: Option<OverlayYaml>,
    overrides: Option<BTreeMap<String, Option<bool>>>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct DlssYaml {
    sr_mode: Option<String>,
    sr_preset: Option<String>,
    sr_model_preset: Option<String>,
    sr_override: Option<bool>,
    rr_mode: Option<String>,
    rr_preset: Option<String>,
    rr_override: Option<bool>,
    fg_enabled: Option<bool>,
    fg_override: Option<bool>,
    #[serde(deserialize_with = "super::yaml::go_int")]
    multi_frame: Option<i64>,
    indicator: Option<bool>,
    fg_indicator: Option<bool>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct GpuYaml {
    vrr: Option<String>,
    shader_cache: Option<bool>,
    shader_cache_path: Option<String>,
    threaded_optimization: Option<bool>,
    #[serde(deserialize_with = "super::yaml::go_int")]
    clock_offset: Option<i64>,
    #[serde(deserialize_with = "super::yaml::go_int")]
    memory_offset: Option<i64>,
    #[serde(deserialize_with = "super::yaml::go_int")]
    power_limit: Option<i64>,
    power_mizer: Option<String>,
    #[serde(deserialize_with = "super::yaml::go_int")]
    fan_speed: Option<i64>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct CpuYaml {
    governor: Option<String>,
    smt: Option<bool>,
    affinity: Option<String>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct ProtonYaml {
    enable_wayland: Option<bool>,
    enable_hdr: Option<bool>,
    enable_ngx_updater: Option<bool>,
    vkd3d_heap: Option<bool>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct OverlayYaml {
    enabled: Option<bool>,
    position: Option<String>,
    show_fps: Option<bool>,
    show_frametime: Option<bool>,
    show_cpu: Option<bool>,
    show_gpu: Option<bool>,
    show_vram: Option<bool>,
    toggle_key: Option<String>,
}

/// The YAML keys of Go's profile struct, by section. Anything else in a
/// profile file is a key Go ignored.
pub const PROFILE_KEYS: [(&str, &[&str]); 5] = [
    (
        "dlss",
        &[
            "sr_mode",
            "sr_preset",
            "sr_model_preset",
            "sr_override",
            "rr_mode",
            "rr_preset",
            "rr_override",
            "fg_enabled",
            "fg_override",
            "multi_frame",
            "indicator",
            "fg_indicator",
        ],
    ),
    (
        "gpu",
        &[
            "vrr",
            "shader_cache",
            "shader_cache_path",
            "threaded_optimization",
            "clock_offset",
            "memory_offset",
            "power_limit",
            "power_mizer",
            "fan_speed",
        ],
    ),
    ("cpu", &["governor", "smt", "affinity"]),
    (
        "proton",
        &[
            "enable_wayland",
            "enable_hdr",
            "enable_ngx_updater",
            "vkd3d_heap",
        ],
    ),
    (
        "overlay",
        &[
            "enabled",
            "position",
            "show_fps",
            "show_frametime",
            "show_cpu",
            "show_gpu",
            "show_vram",
            "toggle_key",
        ],
    ),
];

/// The keys in a Go profile file that Go's struct has no place for, with
/// their values, as `section.key` or `key`. Go ignored them on load.
pub fn unknown_profile_keys(text: &str) -> Vec<(String, String)> {
    let Ok(serde_json::Value::Object(root)) = yaml::from_str::<serde_json::Value>(text) else {
        return Vec::new();
    };
    let mut out = Vec::new();
    for (key, value) in &root {
        if key == "name" || key == "overrides" {
            continue;
        }
        let Some((_, known)) = PROFILE_KEYS.iter().find(|(s, _)| s == key) else {
            out.push((key.clone(), json_text(value)));
            continue;
        };
        if let serde_json::Value::Object(section) = value {
            for (leaf, v) in section {
                if !known.contains(&leaf.as_str()) {
                    out.push((format!("{key}.{leaf}"), json_text(v)));
                }
            }
        }
    }
    out
}

/// A generic YAML value as one line of JSON.
pub fn json_text(value: &serde_json::Value) -> String {
    serde_json::to_string(value).unwrap_or_default()
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
    fn leaves_default_to_zero_and_smt_keeps_its_pointer() {
        let p = parse_profile(&lines(&["name: X", "cpu:", "    smt: false"])).unwrap();
        assert_eq!(p.name, "X");
        assert_eq!(p.leaves.len(), 35);
        assert_eq!(p.leaf("cpu.smt"), &GoValue::Smt(Some(false)));
        assert!(
            !p.leaf("cpu.smt").is_zero(),
            "a non-nil pointer is not zero"
        );
        assert!(p.leaf("gpu.power_limit").is_zero());
        assert_eq!(p.overrides, None, "no key is legacy");
        let p = parse_profile("overrides: {}\n").unwrap();
        assert_eq!(p.overrides, Some(BTreeMap::new()));
    }

    #[test]
    fn sr_model_preset_folds_like_go() {
        let p = parse_profile(&lines(&[
            "dlss:",
            "  sr_preset: A",
            "  sr_model_preset: l",
            "overrides:",
            "  dlss.sr_model_preset: true",
        ]))
        .unwrap();
        assert_eq!(p.leaf("dlss.sr_preset"), &GoValue::Text("L".into()));
        assert!(p.is_marked("dlss.sr_preset"));
        assert!(
            !p.overrides
                .as_ref()
                .unwrap()
                .contains_key("dlss.sr_model_preset")
        );
        let p = parse_profile(&lines(&[
            "dlss:",
            "    sr_preset: k",
            "    sr_model_preset: transformer",
        ]))
        .unwrap();
        assert_eq!(p.leaf("dlss.sr_preset"), &GoValue::Text("K".into()));
        let p = parse_profile(&lines(&["overrides:", "    dlss.sr_model_preset: false"])).unwrap();
        assert_eq!(
            p.overrides.unwrap().get("dlss.sr_model_preset"),
            Some(&false)
        );
    }

    #[test]
    fn vrr_is_checked_with_go_s_text() {
        let err = parse_profile(&lines(&["gpu:", "    vrr: Always"])).unwrap_err();
        assert_eq!(
            err.to_string(),
            "gpu.vrr: unsupported value \"Always\" (valid: , unset, automatic, always, never)"
        );
    }

    #[test]
    fn legacy_inheritance_matches_go() {
        let all = parse_profile(&lines(&[
            "proton:",
            "    enable_hdr: true",
            "gpu:",
            "    power_limit: 300",
        ]))
        .unwrap();
        let mut game = parse_profile(&lines(&[
            "proton:",
            "  enable_hdr: true",
            "  enable_wayland: true",
            "gpu:",
            "  power_limit: 400",
            "  vrr: unset",
        ]))
        .unwrap();
        migrate_inheritance(&mut game, Some(&all));
        let marks = game.overrides.clone().unwrap();
        let marked: Vec<&str> = marks.keys().map(String::as_str).collect();
        assert_eq!(
            marked,
            ["gpu.power_limit", "gpu.vrr", "proton.enable_wayland"]
        );
        assert!(game.leaf("proton.enable_hdr").is_zero(), "equal is zeroed");
        let mut lone = parse_profile(&lines(&["cpu:", "    smt: false"])).unwrap();
        migrate_inheritance(&mut lone, None);
        assert!(lone.is_marked("cpu.smt"), "non-nil smt false is pinned");
    }

    #[test]
    fn keys_go_ignored_are_found() {
        let text = lines(&[
            "name: X",
            "fancy: 3",
            "dlss:",
            "    sr_model_preset: k",
            "    sharpness: 0.5",
            "proton:",
            "    enable_hdr: true",
        ]);
        assert_eq!(
            unknown_profile_keys(&text),
            [
                ("dlss.sharpness".to_owned(), "0.5".to_owned()),
                ("fancy".to_owned(), "3".to_owned())
            ]
        );
    }
}
