//! The environment a launch gives the game.
//!
//! The variables and their values are Go's, exactly (decisions 47 and 59;
//! `internal/profile/apply.go` and `internal/env/env.go`). Go appended them
//! to the parent environment and relied on exec keeping the last duplicate.
//! Here every key is set once, last value wins, and the child gets the
//! parent environment plus these through `Command::env`, which replaces an
//! inherited key. Precedence, lowest first: the parent environment, the
//! profile variables, the launcher's `PROTON_VKD3D_HEAP`, then the wrapper's
//! own `KEY=VALUE` tokens.

use std::collections::BTreeMap;
use std::path::Path;

use crate::profile::{Effective, FieldKey, Value};

/// `PROTON_VKD3D_HEAP`: the legacy descriptor heap gate the launcher adds.
pub const PROTON_VKD3D_HEAP: &str = "PROTON_VKD3D_HEAP";

/// The variables spela sets for one launch. A key is set once: setting it
/// again replaces the value, so the last one wins.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LaunchEnv {
    vars: BTreeMap<String, String>,
}

impl LaunchEnv {
    /// No variables.
    pub fn new() -> LaunchEnv {
        LaunchEnv::default()
    }

    /// Sets `key`, replacing an earlier value.
    pub fn set(&mut self, key: impl Into<String>, value: impl Into<String>) {
        self.vars.insert(key.into(), value.into());
    }

    /// The value of `key`, if set.
    pub fn get(&self, key: &str) -> Option<&str> {
        self.vars.get(key).map(String::as_str)
    }

    /// Whether nothing is set.
    pub fn is_empty(&self) -> bool {
        self.vars.is_empty()
    }

    /// The number of variables.
    pub fn len(&self) -> usize {
        self.vars.len()
    }

    /// Every variable, by key.
    pub fn iter(&self) -> impl Iterator<Item = (&str, &str)> {
        self.vars.iter().map(|(k, v)| (k.as_str(), v.as_str()))
    }

    /// Sets every pair in order, so a later pair beats an earlier one and
    /// every pair beats what was set before.
    pub fn extend<K: Into<String>, V: Into<String>>(
        &mut self,
        pairs: impl IntoIterator<Item = (K, V)>,
    ) {
        for (k, v) in pairs {
            self.set(k, v);
        }
    }

    /// `KEY=VALUE` lines sorted as whole strings, byte by byte, as Go's
    /// `sort.Strings` sorted the dry run's lines.
    pub fn sorted_lines(&self) -> Vec<String> {
        let mut lines: Vec<String> = self.iter().map(|(k, v)| format!("{k}={v}")).collect();
        lines.sort();
        lines
    }
}

/// The profile's variables: Proton, then DLSS, then GPU, as Go's
/// `Profile.ApplyEnv` sets them. `shader_cache_dir` is the folder used
/// when the shader cache is on and the profile names none: the config's
/// `shader_cache`, else `<cache>/nvidia`.
pub fn profile_env(effective: &Effective, shader_cache_dir: &Path) -> LaunchEnv {
    let mut env = LaunchEnv::new();
    let on = |key: FieldKey| effective.value(key).as_bool().unwrap_or(false);
    let text = |key: FieldKey| effective.value(key).as_text().unwrap_or("").to_owned();

    // Proton.
    if on(FieldKey::ProtonEnableWayland) {
        env.set("PROTON_ENABLE_WAYLAND", "1");
    }
    if on(FieldKey::ProtonEnableHdr) {
        env.set("PROTON_ENABLE_HDR", "1");
    }
    if on(FieldKey::ProtonEnableNgxUpdater) {
        env.set("PROTON_ENABLE_NGX_UPDATER", "1");
    }
    if on(FieldKey::ProtonVkd3dHeap) {
        env.set("VKD3D_CONFIG", "descriptor_heap");
    }

    // DLSS, with dxvk-nvapi's values exactly as Go emits them (decision 59).
    if on(FieldKey::DlssSrOverride) {
        env.set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE", "on");
        let mode = text(FieldKey::DlssSrMode);
        if !mode.is_empty() {
            env.set("DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE", mode.clone());
        }
        let preset = text(FieldKey::DlssSrPreset);
        if !preset.is_empty() {
            env.set(
                "DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION",
                sr_preset_env(&preset, &mode),
            );
        }
    }
    if on(FieldKey::DlssRrOverride) {
        env.set("DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE", "on");
        let mode = text(FieldKey::DlssRrMode);
        if !mode.is_empty() {
            env.set("DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE", mode);
        }
        let preset = text(FieldKey::DlssRrPreset);
        if !preset.is_empty() {
            env.set(
                "DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION",
                preset_env(&preset),
            );
        }
    }
    if on(FieldKey::DlssFgOverride) {
        env.set("DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE", "on");
        if on(FieldKey::DlssFgEnabled) {
            let frames = effective
                .value(FieldKey::DlssMultiFrame)
                .as_int()
                .unwrap_or(0);
            env.set(
                "DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT",
                frames.to_string(),
            );
        }
    }
    let mut debug = Vec::new();
    if on(FieldKey::DlssIndicator) {
        debug.push("DLSSIndicator=1024");
    }
    if on(FieldKey::DlssFgIndicator) {
        debug.push("DLSSGIndicator=2");
    }
    if !debug.is_empty() {
        env.set("DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS", debug.join(","));
    }

    // GPU.
    if on(FieldKey::GpuShaderCache) {
        let own = text(FieldKey::GpuShaderCachePath);
        let path = if own.is_empty() {
            shader_cache_dir.display().to_string()
        } else {
            own
        };
        env.set("__GL_SHADER_DISK_CACHE", "1");
        env.set("__GL_SHADER_DISK_CACHE_PATH", path);
    }
    // Only when the game or All games sets it (decision 47). Go exported 0
    // for every game left at the system default.
    if effective.is_set(FieldKey::GpuThreadedOptimization) {
        let value = if on(FieldKey::GpuThreadedOptimization) {
            "1"
        } else {
            "0"
        };
        env.set("__GL_THREADED_OPTIMIZATION", value);
    }
    env
}

/// Whether vkd3d_heap resolves on, which gates the Proton heap variable and
/// the preflight check.
pub fn vkd3d_heap_on(effective: &Effective) -> bool {
    effective.value(FieldKey::ProtonVkd3dHeap) == &Value::Bool(true)
}

/// Go's `dlssSRPresetToEnv`: `auto` picks L for ultra performance, M for
/// performance and K otherwise.
fn sr_preset_env(preset: &str, mode: &str) -> String {
    if preset == "auto" {
        let picked = match mode {
            "ultra_performance" => "L",
            "performance" => "M",
            _ => "K",
        };
        return preset_env(picked);
    }
    preset_env(preset)
}

/// Go's `dlssPresetToEnv`: a known letter is `render_preset_<letter>`, and
/// anything else, `default` included, is `render_preset_default`.
fn preset_env(preset: &str) -> String {
    match preset {
        "A" | "B" | "C" | "D" | "E" | "F" | "J" | "K" | "L" | "M" => {
            format!("render_preset_{}", preset.to_ascii_lowercase())
        }
        _ => "render_preset_default".to_owned(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::profile::OwnValues;

    fn effective(game: &[(FieldKey, Value)], all: &[(FieldKey, Value)]) -> Effective {
        let mut g = OwnValues::new();
        for (k, v) in game {
            g.set(*k, v.clone()).unwrap();
        }
        let mut a = OwnValues::new();
        for (k, v) in all {
            a.set(*k, v.clone()).unwrap();
        }
        Effective::of_game(9100010, Some(&g), Some(&a))
    }

    fn t(s: &str) -> Value {
        Value::Text(s.into())
    }

    #[test]
    fn go_s_variables_for_glimmer_run() {
        // sandbox/main 9100010, as Go's dry run printed it.
        let e = effective(
            &[
                (FieldKey::ProtonEnableHdr, Value::Bool(true)),
                (FieldKey::ProtonVkd3dHeap, Value::Bool(true)),
                (FieldKey::DlssSrMode, t("quality")),
                (FieldKey::DlssSrPreset, t("K")),
                (FieldKey::DlssSrOverride, Value::Bool(true)),
                (FieldKey::DlssFgEnabled, Value::Bool(true)),
                (FieldKey::DlssFgOverride, Value::Bool(true)),
                (FieldKey::DlssMultiFrame, Value::Int(3)),
                (FieldKey::DlssIndicator, Value::Bool(true)),
                (FieldKey::GpuThreadedOptimization, Value::Bool(false)),
            ],
            &[
                (FieldKey::ProtonEnableWayland, Value::Bool(true)),
                (FieldKey::GpuShaderCache, Value::Bool(true)),
            ],
        );
        let env = profile_env(&e, Path::new("/cache/spela/nvidia"));
        assert_eq!(
            env.sorted_lines(),
            [
                "DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT=3",
                "DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE=on",
                "DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE=quality",
                "DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE=on",
                "DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_k",
                "DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS=DLSSIndicator=1024",
                "PROTON_ENABLE_HDR=1",
                "PROTON_ENABLE_WAYLAND=1",
                "VKD3D_CONFIG=descriptor_heap",
                "__GL_SHADER_DISK_CACHE=1",
                "__GL_SHADER_DISK_CACHE_PATH=/cache/spela/nvidia",
                "__GL_THREADED_OPTIMIZATION=0",
            ]
        );
        assert!(vkd3d_heap_on(&e));
    }

    #[test]
    fn threaded_optimization_only_when_set() {
        // Decision 47: nothing sets it, nothing is exported.
        let e = effective(&[], &[]);
        assert!(profile_env(&e, Path::new("/x")).is_empty());
        let on = effective(
            &[],
            &[(FieldKey::GpuThreadedOptimization, Value::Bool(true))],
        );
        assert_eq!(
            profile_env(&on, Path::new("/x")).get("__GL_THREADED_OPTIMIZATION"),
            Some("1")
        );
    }

    #[test]
    fn presets_modes_and_indicators_follow_go() {
        let e = effective(
            &[
                (FieldKey::DlssSrOverride, Value::Bool(true)),
                (FieldKey::DlssSrPreset, t("auto")),
                (FieldKey::DlssSrMode, t("ultra_performance")),
                (FieldKey::DlssRrOverride, Value::Bool(true)),
                (FieldKey::DlssRrPreset, t("default")),
                (FieldKey::DlssFgOverride, Value::Bool(true)),
                (FieldKey::DlssIndicator, Value::Bool(true)),
                (FieldKey::DlssFgIndicator, Value::Bool(true)),
                (FieldKey::GpuShaderCache, Value::Bool(true)),
                (FieldKey::GpuShaderCachePath, t("/games/shaders")),
            ],
            &[],
        );
        let env = profile_env(&e, Path::new("/x"));
        assert_eq!(
            env.get("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION"),
            Some("render_preset_l")
        );
        assert_eq!(
            env.get("DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION"),
            Some("render_preset_default")
        );
        assert_eq!(env.get("DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE"), None);
        // FG override without FG on sets no frame count.
        assert_eq!(env.get("DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT"), None);
        assert_eq!(
            env.get("DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS"),
            Some("DLSSIndicator=1024,DLSSGIndicator=2")
        );
        assert_eq!(
            env.get("__GL_SHADER_DISK_CACHE_PATH"),
            Some("/games/shaders")
        );
        assert_eq!(sr_preset_env("auto", "performance"), "render_preset_m");
        assert_eq!(sr_preset_env("auto", "quality"), "render_preset_k");
        // Without the override nothing of SR is exported.
        let off = effective(&[(FieldKey::DlssSrMode, t("dlaa"))], &[]);
        assert!(profile_env(&off, Path::new("/x")).is_empty());
    }

    #[test]
    fn last_value_wins() {
        let mut env = LaunchEnv::new();
        env.set("A", "1");
        env.extend([("A", "2"), ("B", "3")]);
        env.set("B", "4");
        assert_eq!(env.sorted_lines(), ["A=2", "B=4"]);
        assert_eq!(env.len(), 2);
    }
}
