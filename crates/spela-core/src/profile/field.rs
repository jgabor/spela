//! The field table: the one list of profile fields.
//!
//! Every surface reads this table: the CLI's flags and reset names, the
//! TUI's rows and editors, the TOML reader and writer, the JSON output and
//! the launch plan. None of them keeps a second list. It holds the 25 fields
//! that are left after decision 10 drops Go's eight overlay fields,
//! `gpu.power_mizer` and `cpu.affinity`: four Proton, eleven DLSS, eight GPU
//! and two CPU fields, in Go's descriptor order (`internal/profile/fields.go`).
//!
//! The default of every field is the zero of its kind: Off, 0, empty, or
//! none for SMT. Nothing else is a built-in default.

use std::fmt;

use serde::Serialize;

use super::value::{Kind, Value, parse_bool};

/// A profile section, in the order files, the TUI and the docs show them.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Section {
    /// Proton settings.
    Proton,
    /// DLSS settings.
    Dlss,
    /// GPU settings.
    Gpu,
    /// CPU settings.
    Cpu,
}

impl Section {
    /// Every section, in order.
    pub const ALL: [Section; 4] = [Section::Proton, Section::Dlss, Section::Gpu, Section::Cpu];

    /// The TOML table and key prefix, such as `dlss`.
    pub fn name(self) -> &'static str {
        match self {
            Section::Proton => "proton",
            Section::Dlss => "dlss",
            Section::Gpu => "gpu",
            Section::Cpu => "cpu",
        }
    }

    /// The heading people see, such as `DLSS`.
    pub fn title(self) -> &'static str {
        match self {
            Section::Proton => "Proton",
            Section::Dlss => "DLSS",
            Section::Gpu => "GPU",
            Section::Cpu => "CPU",
        }
    }

    /// The section with this TOML name.
    pub fn from_name(name: &str) -> Option<Section> {
        Section::ALL.into_iter().find(|s| s.name() == name)
    }

    /// This section's fields, in table order.
    pub fn fields(self) -> impl Iterator<Item = &'static Field> {
        FIELDS.iter().filter(move |f| f.section == self)
    }

    /// The field a command-line name means in this section. Dashes count as
    /// underscores, a `<section>.` prefix is allowed, and Go's short names
    /// work: `hdr`, `wayland`, `ngx-updater`, `fg` and `threaded-opt`.
    pub fn lookup(self, input: &str) -> Option<&'static Field> {
        let leaf = input.replace('-', "_");
        let leaf = leaf
            .strip_prefix(self.name())
            .and_then(|rest| rest.strip_prefix('.'))
            .unwrap_or(&leaf);
        self.fields()
            .find(|f| f.name() == leaf || f.aliases.contains(&leaf))
    }
}

impl fmt::Display for Section {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.name())
    }
}

/// The control a surface shows to edit a field.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Editor {
    /// On or Off.
    Toggle,
    /// A pick from a list.
    Choice,
    /// A whole number.
    Integer,
    /// Free text.
    Text,
}

/// What launch changes for a field. Go's impact classes, kept for the
/// field docs (`internal/profile/explanation.go:20-26`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Impact {
    /// A Proton compatibility variable.
    Compatibility,
    /// Another launch environment variable.
    Environment,
    /// GPU, CPU or display state outside the game.
    SystemState,
}

impl Impact {
    /// Go's word, such as `system_state`.
    pub fn name(self) -> &'static str {
        match self {
            Impact::Compatibility => "compatibility",
            Impact::Environment => "environment",
            Impact::SystemState => "system_state",
        }
    }
}

/// How launch applies a field, which says who restores it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Effect {
    /// An environment variable for the game. Nothing to restore.
    Environment,
    /// A hardware change in the root helper: GPU clocks, power and fans,
    /// the CPU governor and SMT. Restored at exit from the prior value.
    Hardware,
    /// The KDE display policy, changed in the user session. Restored at exit.
    Display,
}

impl Effect {
    /// Go's restore class for this effect
    /// (`internal/profile/explanation.go:28-35`).
    pub fn restore(self) -> &'static str {
        match self {
            Effect::Environment => "ephemeral_launch_environment",
            Effect::Hardware | Effect::Display => "restorable_mutation",
        }
    }
}

/// Which values a field accepts beyond its kind.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Rule {
    /// Any value of the kind.
    Any,
    /// One of these, compared with [`Value::canonical`]. `""` is in each
    /// list of a text field: an empty own value is a real value, such as
    /// `No mode`, not "follows".
    OneOf(&'static [&'static str]),
    /// Empty, or an absolute path.
    AbsolutePathOrEmpty,
    /// Any text. The choices come from the system at run time, as the CPU
    /// governors from sysfs `scaling_available_governors`. A stored value
    /// the system does not list is kept and reported, never refused here.
    SystemList,
}

/// One profile field.
#[derive(Debug, PartialEq, Eq)]
pub struct Field {
    /// The typed key.
    pub id: FieldKey,
    /// The full key, `<section>.<name>`, as in JSON, the migration record
    /// and error messages.
    pub key: &'static str,
    /// The section.
    pub section: Section,
    /// The value kind.
    pub kind: Kind,
    /// The short label every surface shows.
    pub label: &'static str,
    /// The help text: what the field does at launch.
    pub description: &'static str,
    /// The values it accepts.
    pub rule: Rule,
    /// The editor a surface shows.
    pub editor: Editor,
    /// The unit of a number, such as `MHz`.
    pub unit: Option<&'static str>,
    /// The shown range of a number. Shown, never enforced.
    pub range: Option<(i64, i64)>,
    /// The long flag of the section's `set` command, without the dashes.
    /// `None` when the field has no flag of its own.
    pub flag: Option<&'static str>,
    /// Other names that `reset` accepts, after dashes become underscores.
    pub aliases: &'static [&'static str],
    /// Go's impact class.
    pub impact: Impact,
    /// How launch applies it.
    pub effect: Effect,
}

impl Field {
    /// The key within its section, such as `sr_mode`. It is the TOML key
    /// in the section's table.
    pub fn name(&self) -> &'static str {
        &self.key[self.section.name().len() + 1..]
    }

    /// The built-in default: the zero of the kind.
    pub fn default_value(&self) -> Value {
        self.kind.zero()
    }

    /// The listed choices, for a [`Rule::OneOf`] field.
    pub fn choices(&self) -> &'static [&'static str] {
        match self.rule {
            Rule::OneOf(list) => list,
            _ => &[],
        }
    }

    /// Checks that `value` is one this field may hold.
    pub fn check(&self, value: &Value) -> Result<(), FieldError> {
        if value.kind() != self.kind {
            return Err(FieldError::WrongKind {
                key: self.key,
                expected: self.kind,
            });
        }
        match self.rule {
            Rule::Any | Rule::SystemList => Ok(()),
            Rule::OneOf(list) => {
                let text = value.canonical();
                if list.contains(&text.as_str()) {
                    Ok(())
                } else {
                    Err(FieldError::Unsupported {
                        key: self.key,
                        value: text,
                        valid: list,
                    })
                }
            }
            Rule::AbsolutePathOrEmpty => match value {
                Value::Text(t) if !t.is_empty() && !t.starts_with('/') => {
                    Err(FieldError::NotAbsolute {
                        key: self.key,
                        value: t.clone(),
                    })
                }
                _ => Ok(()),
            },
        }
    }

    /// Reads a value as a person types it, on the command line or in a TUI
    /// editor, and checks it. Booleans take spela's one bool vocabulary
    /// ([`parse_bool`]). The two presets are normalised as Go does: `default`
    /// and `auto` in lower case, a single letter in upper case. `default`
    /// for any other field is the caller's to handle, as "follow All games".
    pub fn parse(&self, raw: &str) -> Result<Value, FieldError> {
        let value = match self.kind {
            Kind::Bool => Value::Bool(parse_bool(raw).ok_or_else(|| FieldError::InvalidBool {
                raw: raw.to_owned(),
            })?),
            Kind::OptionalBool => Value::OptionalBool(Some(parse_bool(raw).ok_or_else(|| {
                FieldError::InvalidBool {
                    raw: raw.to_owned(),
                }
            })?)),
            Kind::Int => {
                Value::Int(raw.trim().parse().map_err(|_| FieldError::InvalidInteger {
                    key: self.key,
                    raw: raw.to_owned(),
                })?)
            }
            Kind::Text => match self.id {
                FieldKey::DlssSrPreset | FieldKey::DlssRrPreset => {
                    Value::Text(normalize_preset(raw))
                }
                _ => Value::Text(raw.to_owned()),
            },
        };
        self.check(&value)?;
        Ok(value)
    }
}

/// Go's `NormalizeSRPreset`: trimmed, `default` and `auto` lower case, a
/// single letter upper case, anything else as typed.
pub fn normalize_preset(raw: &str) -> String {
    let s = raw.trim();
    let lower = s.to_ascii_lowercase();
    if lower == "default" || lower == "auto" {
        lower
    } else if s.chars().count() == 1 {
        s.to_uppercase()
    } else {
        s.to_owned()
    }
}

/// A value a field cannot hold, or a name no field has.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum FieldError {
    /// No field has this key.
    #[error("unknown profile field: {0:?}")]
    Unknown(String),
    /// The value has the wrong kind.
    #[error("{key}: expected {}", expected.describe())]
    WrongKind {
        /// The field.
        key: &'static str,
        /// The kind it takes.
        expected: Kind,
    },
    /// The value is not in the field's list.
    #[error("{key}: unsupported value {value:?} (valid: {})", valid.join(", "))]
    Unsupported {
        /// The field.
        key: &'static str,
        /// The value, as [`Value::canonical`] writes it.
        value: String,
        /// The values it takes.
        valid: &'static [&'static str],
    },
    /// A path field holds a relative path.
    #[error("{key} must be an absolute path: {value}")]
    NotAbsolute {
        /// The field.
        key: &'static str,
        /// The path.
        value: String,
    },
    /// Typed text that is not a bool. Go's text
    /// (`cmd/spela/commands/bool_flag.go`).
    #[error("invalid bool {raw:?} (use true/false)")]
    InvalidBool {
        /// What was typed.
        raw: String,
    },
    /// Typed text that is not a whole number.
    #[error("{key}: invalid integer {raw:?}")]
    InvalidInteger {
        /// The field.
        key: &'static str,
        /// What was typed.
        raw: String,
    },
}

/// The typed key of each field. The order is the table order, so a
/// `BTreeMap<FieldKey, _>` iterates in it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize)]
#[serde(into = "&'static str")]
pub enum FieldKey {
    /// `proton.enable_hdr`
    ProtonEnableHdr,
    /// `proton.enable_wayland`
    ProtonEnableWayland,
    /// `proton.enable_ngx_updater`
    ProtonEnableNgxUpdater,
    /// `proton.vkd3d_heap`
    ProtonVkd3dHeap,
    /// `dlss.sr_mode`
    DlssSrMode,
    /// `dlss.sr_preset`
    DlssSrPreset,
    /// `dlss.sr_override`
    DlssSrOverride,
    /// `dlss.rr_mode`
    DlssRrMode,
    /// `dlss.rr_preset`
    DlssRrPreset,
    /// `dlss.rr_override`
    DlssRrOverride,
    /// `dlss.fg_enabled`
    DlssFgEnabled,
    /// `dlss.fg_override`
    DlssFgOverride,
    /// `dlss.multi_frame`
    DlssMultiFrame,
    /// `dlss.indicator`
    DlssIndicator,
    /// `dlss.fg_indicator`
    DlssFgIndicator,
    /// `gpu.clock_offset`
    GpuClockOffset,
    /// `gpu.vrr`
    GpuVrr,
    /// `gpu.memory_offset`
    GpuMemoryOffset,
    /// `gpu.power_limit`
    GpuPowerLimit,
    /// `gpu.fan_speed`
    GpuFanSpeed,
    /// `gpu.shader_cache`
    GpuShaderCache,
    /// `gpu.shader_cache_path`
    GpuShaderCachePath,
    /// `gpu.threaded_optimization`
    GpuThreadedOptimization,
    /// `cpu.governor`
    CpuGovernor,
    /// `cpu.smt`
    CpuSmt,
}

impl FieldKey {
    /// Every key, in table order.
    pub fn all() -> impl Iterator<Item = FieldKey> {
        FIELDS.iter().map(|f| f.id)
    }

    /// The field's descriptor.
    pub fn field(self) -> &'static Field {
        &FIELDS[self as usize]
    }

    /// The full key, such as `dlss.sr_mode`.
    pub fn as_str(self) -> &'static str {
        self.field().key
    }

    /// The key with this full name, such as `dlss.sr_mode`.
    pub fn from_key(key: &str) -> Option<FieldKey> {
        field(key).map(|f| f.id)
    }
}

impl From<FieldKey> for &'static str {
    fn from(key: FieldKey) -> &'static str {
        key.as_str()
    }
}

impl fmt::Display for FieldKey {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// The field with this full key, such as `dlss.sr_mode`.
pub fn field(key: &str) -> Option<&'static Field> {
    FIELDS.iter().find(|f| f.key == key)
}

const DLSS_MODES: &[&str] = &[
    "",
    "off",
    "ultra_performance",
    "performance",
    "balanced",
    "quality",
    "dlaa",
];

/// The table, in Go's descriptor order. [`FieldKey`] indexes it.
pub static FIELDS: [Field; 25] = [
    // Proton
    Field {
        id: FieldKey::ProtonEnableHdr,
        key: "proton.enable_hdr",
        section: Section::Proton,
        kind: Kind::Bool,
        label: "HDR",
        description: "Enables HDR output in Proton (PROTON_ENABLE_HDR=1) when the game starts. Off leaves the variable unset.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("hdr"),
        aliases: &["hdr"],
        impact: Impact::Compatibility,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::ProtonEnableWayland,
        key: "proton.enable_wayland",
        section: Section::Proton,
        kind: Kind::Bool,
        label: "Wayland",
        description: "Runs the game on native Wayland in Proton (PROTON_ENABLE_WAYLAND=1) when the game starts. Off leaves the variable unset.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("wayland"),
        aliases: &["wayland"],
        impact: Impact::Compatibility,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::ProtonEnableNgxUpdater,
        key: "proton.enable_ngx_updater",
        section: Section::Proton,
        kind: Kind::Bool,
        label: "NGX updater",
        description: "Lets Proton update the DLSS DLLs (PROTON_ENABLE_NGX_UPDATER=1) when the game starts. Off leaves the variable unset.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("ngx-updater"),
        aliases: &["ngx_updater"],
        impact: Impact::Compatibility,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::ProtonVkd3dHeap,
        key: "proton.vkd3d_heap",
        section: Section::Proton,
        kind: Kind::Bool,
        label: "VKD3D heap",
        description: "Enables the VKD3D descriptor heap path (VKD3D_CONFIG=descriptor_heap) when the game starts. Needs Proton-CachyOS 10.0-20260321+ or 11.0+ and NVIDIA driver 580.94.16+.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("vkd3d-heap"),
        aliases: &[],
        impact: Impact::Compatibility,
        effect: Effect::Environment,
    },
    // DLSS
    Field {
        id: FieldKey::DlssSrMode,
        key: "dlss.sr_mode",
        section: Section::Dlss,
        kind: Kind::Text,
        label: "Super resolution",
        description: "DLSS super resolution quality mode. Applied only when Resolution override is On.",
        rule: Rule::OneOf(DLSS_MODES),
        editor: Editor::Choice,
        unit: None,
        range: None,
        flag: Some("sr-mode"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssSrPreset,
        key: "dlss.sr_preset",
        section: Section::Dlss,
        kind: Kind::Text,
        label: "Resolution preset",
        description: "DLSS super resolution render preset: auto follows the mode, A to F are the CNN presets (DLSS 2/3), J to M are the transformer presets (DLSS 4/4.5). Applied only when Resolution override is On.",
        rule: Rule::OneOf(&[
            "", "default", "auto", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M",
        ]),
        editor: Editor::Choice,
        unit: None,
        range: None,
        flag: Some("sr-preset"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssSrOverride,
        key: "dlss.sr_override",
        section: Section::Dlss,
        kind: Kind::Bool,
        label: "Resolution override",
        description: "Turns on the DLSS super resolution override. Mode and preset take effect only when this is On.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        // Go had no flag: --sr-mode and --sr-preset turn it on. The flag
        // lets the CLI turn it off, as the TUI can (decision 20).
        flag: Some("sr-override"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssRrMode,
        key: "dlss.rr_mode",
        section: Section::Dlss,
        kind: Kind::Text,
        label: "Ray reconstruction",
        description: "DLSS ray reconstruction quality mode. Applied only when Ray override is On.",
        rule: Rule::OneOf(DLSS_MODES),
        editor: Editor::Choice,
        unit: None,
        range: None,
        flag: Some("rr-mode"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssRrPreset,
        key: "dlss.rr_preset",
        section: Section::Dlss,
        kind: Kind::Text,
        label: "Ray preset",
        description: "DLSS ray reconstruction render preset: A to F are the CNN presets, J to M are the transformer presets. Applied only when Ray override is On.",
        rule: Rule::OneOf(&[
            "", "default", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M",
        ]),
        editor: Editor::Choice,
        unit: None,
        range: None,
        flag: Some("rr-preset"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssRrOverride,
        key: "dlss.rr_override",
        section: Section::Dlss,
        kind: Kind::Bool,
        label: "Ray override",
        description: "Turns on the DLSS ray reconstruction override. Mode and preset take effect only when this is On.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("rr-override"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssFgEnabled,
        key: "dlss.fg_enabled",
        section: Section::Dlss,
        kind: Kind::Bool,
        label: "Frame generation",
        description: "Requests DLSS frame generation. Applied only when Generation override is On.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("fg"),
        aliases: &["fg"],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssFgOverride,
        key: "dlss.fg_override",
        section: Section::Dlss,
        kind: Kind::Bool,
        label: "Generation override",
        description: "Turns on the DLSS frame generation override. Frame generation and multi-frame take effect only when this is On.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("fg-override"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssMultiFrame,
        key: "dlss.multi_frame",
        section: Section::Dlss,
        kind: Kind::Int,
        label: "Multi-frame",
        description: "Extra frames to generate, 0 to 4. Applied only when Generation override and Frame generation are both On.",
        rule: Rule::OneOf(&["0", "1", "2", "3", "4"]),
        editor: Editor::Integer,
        unit: None,
        range: None,
        flag: Some("multi-frame"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssIndicator,
        key: "dlss.indicator",
        section: Section::Dlss,
        kind: Kind::Bool,
        label: "SR indicator",
        description: "Shows the DLSS status indicator in the game.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("indicator"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::DlssFgIndicator,
        key: "dlss.fg_indicator",
        section: Section::Dlss,
        kind: Kind::Bool,
        label: "FG indicator",
        description: "Shows the DLSS frame generation indicator in the game.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("fg-indicator"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    // GPU
    Field {
        id: FieldKey::GpuClockOffset,
        key: "gpu.clock_offset",
        section: Section::Gpu,
        kind: Kind::Int,
        label: "Clock offset",
        description: "Raises or lowers the GPU core clock by this many MHz while the game runs, as an NVML clock offset. The previous offset returns at exit. 0 leaves the offset unchanged.",
        rule: Rule::Any,
        editor: Editor::Integer,
        unit: Some("MHz"),
        range: None,
        flag: Some("clock-offset"),
        aliases: &[],
        impact: Impact::SystemState,
        effect: Effect::Hardware,
    },
    Field {
        id: FieldKey::GpuVrr,
        key: "gpu.vrr",
        section: Section::Gpu,
        kind: Kind::Text,
        label: "KDE VRR",
        description: "KDE Wayland primary display only: unset leaves KDE unchanged; automatic uses fullscreen VRR; always includes the desktop; never disables VRR. Restored after a tracked launch.",
        rule: Rule::OneOf(&["", "unset", "automatic", "always", "never"]),
        editor: Editor::Choice,
        unit: None,
        range: None,
        flag: Some("vrr"),
        aliases: &[],
        impact: Impact::SystemState,
        effect: Effect::Display,
    },
    Field {
        id: FieldKey::GpuMemoryOffset,
        key: "gpu.memory_offset",
        section: Section::Gpu,
        kind: Kind::Int,
        label: "Memory offset",
        description: "Raises or lowers the GPU memory clock by this many MHz while the game runs, as an NVML memory clock offset. The previous offset returns at exit. 0 leaves the offset unchanged.",
        rule: Rule::Any,
        editor: Editor::Integer,
        unit: Some("MHz"),
        range: None,
        flag: Some("memory-offset"),
        aliases: &[],
        impact: Impact::SystemState,
        effect: Effect::Hardware,
    },
    Field {
        id: FieldKey::GpuPowerLimit,
        key: "gpu.power_limit",
        section: Section::Gpu,
        kind: Kind::Int,
        label: "Power limit",
        description: "Sets the GPU power limit while the game runs. The previous limit returns at exit when it could be read. 0 leaves the limit unchanged.",
        rule: Rule::Any,
        editor: Editor::Integer,
        unit: Some("W"),
        range: None,
        flag: Some("power-limit"),
        aliases: &[],
        impact: Impact::SystemState,
        effect: Effect::Hardware,
    },
    Field {
        id: FieldKey::GpuFanSpeed,
        key: "gpu.fan_speed",
        section: Section::Gpu,
        kind: Kind::Int,
        label: "Fan speed",
        description: "Sets every GPU fan to this speed while the game runs. Fans return to their previous state, or to automatic, at exit. 0 means no fan change at launch.",
        rule: Rule::Any,
        editor: Editor::Integer,
        unit: Some("%"),
        range: Some((0, 100)),
        flag: Some("fan-speed"),
        aliases: &[],
        impact: Impact::SystemState,
        effect: Effect::Hardware,
    },
    Field {
        id: FieldKey::GpuShaderCache,
        key: "gpu.shader_cache",
        section: Section::Gpu,
        kind: Kind::Bool,
        label: "Shader cache",
        description: "Turns on the NVIDIA shader disk cache (__GL_SHADER_DISK_CACHE=1) when the game starts. Off leaves the driver setting alone.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("shader-cache"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::GpuShaderCachePath,
        key: "gpu.shader_cache_path",
        section: Section::Gpu,
        kind: Kind::Text,
        label: "Shader cache path",
        description: "Folder for the NVIDIA shader cache, as an absolute path. Empty uses the default shader cache path from Settings. Used only when Shader cache is On.",
        rule: Rule::AbsolutePathOrEmpty,
        editor: Editor::Text,
        unit: None,
        range: None,
        flag: Some("shader-cache-path"),
        aliases: &[],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    Field {
        id: FieldKey::GpuThreadedOptimization,
        key: "gpu.threaded_optimization",
        section: Section::Gpu,
        kind: Kind::Bool,
        label: "Threaded optimization",
        description: "Sets __GL_THREADED_OPTIMIZATION to 1 (On) or 0 (Off) when the game starts. Unlike other toggles, Off is applied. When neither the game nor All games sets it, the variable is left alone.",
        rule: Rule::Any,
        editor: Editor::Toggle,
        unit: None,
        range: None,
        flag: Some("threaded-opt"),
        aliases: &["threaded_opt"],
        impact: Impact::Environment,
        effect: Effect::Environment,
    },
    // CPU
    Field {
        id: FieldKey::CpuGovernor,
        key: "cpu.governor",
        section: Section::Cpu,
        kind: Kind::Text,
        label: "Governor",
        description: "Sets the CPU frequency governor while the game runs. The choices are the governors the system lists. The previous governor returns at exit when it could be read. Empty leaves it unchanged.",
        rule: Rule::SystemList,
        editor: Editor::Choice,
        unit: None,
        range: None,
        flag: Some("governor"),
        aliases: &[],
        impact: Impact::SystemState,
        effect: Effect::Hardware,
    },
    Field {
        id: FieldKey::CpuSmt,
        key: "cpu.smt",
        section: Section::Cpu,
        kind: Kind::OptionalBool,
        label: "SMT",
        description: "Turns simultaneous multithreading (SMT) on or off while the game runs, then restores it at exit. When nothing sets it, SMT is left unchanged.",
        rule: Rule::OneOf(&["", "true", "false"]),
        editor: Editor::Choice,
        unit: None,
        range: None,
        flag: Some("smt"),
        aliases: &[],
        impact: Impact::SystemState,
        effect: Effect::Hardware,
    },
];

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_table_has_the_25_kept_fields() {
        assert_eq!(FIELDS.len(), 25);
        let count = |s: Section| s.fields().count();
        assert_eq!(count(Section::Proton), 4);
        assert_eq!(count(Section::Dlss), 11);
        assert_eq!(count(Section::Gpu), 8);
        assert_eq!(count(Section::Cpu), 2);
    }

    #[test]
    fn keys_index_the_table_in_order() {
        for (i, f) in FIELDS.iter().enumerate() {
            assert_eq!(f.id as usize, i, "{}", f.key);
            assert_eq!(f.id.field().key, f.key);
            assert_eq!(FieldKey::from_key(f.key), Some(f.id));
            assert!(f.key.starts_with(&format!("{}.", f.section.name())));
            assert!(!f.description.contains('\u{2014}'), "{}", f.key);
        }
        let keys: Vec<_> = FieldKey::all().collect();
        let mut sorted = keys.clone();
        sorted.sort();
        assert_eq!(keys, sorted);
        // Sections are contiguous and in Section order.
        let sections: Vec<Section> = FIELDS.iter().map(|f| f.section).collect();
        let mut s = sections.clone();
        s.sort();
        assert_eq!(sections, s);
    }

    #[test]
    fn dropped_fields_are_gone() {
        for key in [
            "gpu.power_mizer",
            "cpu.affinity",
            "overlay.enabled",
            "overlay.position",
            "overlay.show_fps",
            "overlay.toggle_key",
        ] {
            assert!(field(key).is_none(), "{key}");
        }
        assert!(FIELDS.iter().all(|f| f.flag != Some("power-mizer")));
    }

    #[test]
    fn flags_and_reset_names_keep_go_s_spellings() {
        let flag = |k: FieldKey| k.field().flag;
        assert_eq!(flag(FieldKey::ProtonEnableHdr), Some("hdr"));
        assert_eq!(flag(FieldKey::DlssFgEnabled), Some("fg"));
        assert_eq!(
            flag(FieldKey::GpuThreadedOptimization),
            Some("threaded-opt")
        );
        assert_eq!(flag(FieldKey::GpuVrr), Some("vrr"));
        assert_eq!(flag(FieldKey::DlssSrOverride), Some("sr-override"));
        let look = |s: Section, n: &str| s.lookup(n).map(|f| f.id);
        assert_eq!(
            look(Section::Proton, "hdr"),
            Some(FieldKey::ProtonEnableHdr)
        );
        assert_eq!(
            look(Section::Proton, "ngx-updater"),
            Some(FieldKey::ProtonEnableNgxUpdater)
        );
        assert_eq!(look(Section::Dlss, "fg"), Some(FieldKey::DlssFgEnabled));
        assert_eq!(look(Section::Dlss, "sr-mode"), Some(FieldKey::DlssSrMode));
        assert_eq!(
            look(Section::Dlss, "dlss.sr_mode"),
            Some(FieldKey::DlssSrMode)
        );
        assert_eq!(
            look(Section::Gpu, "threaded-opt"),
            Some(FieldKey::GpuThreadedOptimization)
        );
        // A field must belong to the named section.
        assert_eq!(look(Section::Gpu, "sr_mode"), None);
        assert_eq!(look(Section::Proton, "fg"), None);
        // Every flag also resolves as a reset name in its section.
        for f in &FIELDS {
            if let Some(flag) = f.flag {
                assert_eq!(look(f.section, flag), Some(f.id), "{}", f.key);
            }
        }
    }

    #[test]
    fn defaults_are_the_zero_of_each_kind() {
        assert_eq!(
            FieldKey::GpuShaderCache.field().default_value(),
            Value::Bool(false)
        );
        assert_eq!(
            FieldKey::GpuPowerLimit.field().default_value(),
            Value::Int(0)
        );
        assert_eq!(
            FieldKey::DlssSrMode.field().default_value(),
            Value::Text(String::new())
        );
        assert_eq!(
            FieldKey::CpuSmt.field().default_value(),
            Value::OptionalBool(None)
        );
        for f in &FIELDS {
            assert_eq!(f.check(&f.default_value()), Ok(()), "{}", f.key);
        }
    }

    #[test]
    fn check_follows_go_s_typed_write_rules() {
        let sr = FieldKey::DlssSrMode.field();
        assert_eq!(sr.check(&Value::Text("quality".into())), Ok(()));
        assert_eq!(
            sr.check(&Value::Text("fast".into()))
                .unwrap_err()
                .to_string(),
            "dlss.sr_mode: unsupported value \"fast\" (valid: , off, ultra_performance, performance, balanced, quality, dlaa)"
        );
        assert_eq!(
            sr.check(&Value::Bool(true)).unwrap_err().to_string(),
            "dlss.sr_mode: expected string"
        );
        let mf = FieldKey::DlssMultiFrame.field();
        assert_eq!(mf.check(&Value::Int(4)), Ok(()));
        assert!(mf.check(&Value::Int(5)).is_err());
        // Ranges are shown, never enforced.
        let fan = FieldKey::GpuFanSpeed.field();
        assert_eq!(fan.check(&Value::Int(150)), Ok(()));
        assert_eq!(fan.range, Some((0, 100)));
        // Offsets may be negative.
        assert_eq!(
            FieldKey::GpuClockOffset.field().check(&Value::Int(-100)),
            Ok(())
        );
        let path = FieldKey::GpuShaderCachePath.field();
        assert_eq!(path.check(&Value::Text(String::new())), Ok(()));
        assert_eq!(path.check(&Value::Text("/games/cache".into())), Ok(()));
        assert_eq!(
            path.check(&Value::Text("cache".into()))
                .unwrap_err()
                .to_string(),
            "gpu.shader_cache_path must be an absolute path: cache"
        );
        // A governor the built-in list never had is not refused here.
        let gov = FieldKey::CpuGovernor.field();
        assert_eq!(gov.check(&Value::Text("conservative".into())), Ok(()));
        let vrr = FieldKey::GpuVrr.field();
        assert!(vrr.check(&Value::Text("Always".into())).is_err());
    }

    #[test]
    fn parse_reads_what_people_type() {
        let hdr = FieldKey::ProtonEnableHdr.field();
        for (raw, want) in [("yes", true), ("ON", true), ("0", false), ("False", false)] {
            assert_eq!(hdr.parse(raw), Ok(Value::Bool(want)), "{raw}");
        }
        assert_eq!(
            hdr.parse("maybe").unwrap_err().to_string(),
            "invalid bool \"maybe\" (use true/false)"
        );
        assert_eq!(
            FieldKey::CpuSmt.field().parse("off"),
            Ok(Value::OptionalBool(Some(false)))
        );
        assert_eq!(
            FieldKey::GpuClockOffset.field().parse("-100"),
            Ok(Value::Int(-100))
        );
        assert!(FieldKey::GpuClockOffset.field().parse("fast").is_err());
        let sr = FieldKey::DlssSrPreset.field();
        assert_eq!(sr.parse("k"), Ok(Value::Text("K".into())));
        assert_eq!(sr.parse("AUTO"), Ok(Value::Text("auto".into())));
        assert_eq!(sr.parse("Default"), Ok(Value::Text("default".into())));
        assert!(sr.parse("G").is_err());
        // rr_preset has no auto, so it fails when it is typed.
        assert!(FieldKey::DlssRrPreset.field().parse("auto").is_err());
    }
}
