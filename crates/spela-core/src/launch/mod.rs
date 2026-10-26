//! Launch planning: one plan for the dry run, the wrapper and the TUI's
//! Preview launch.
//!
//! `spela launch --dry-run` prints a [`LaunchPlan`], wrapper mode prints the
//! same plan and then carries it out, and the TUI shows it read only. All
//! three build it with [`plan`] and print it with [`summary`], so the dry
//! run shows every variable the launch sets, `PROTON_VKD3D_HEAP` included,
//! which Go's dry run left out (architecture, "One plan for dry run and
//! launch").
//!
//! | Module | Role |
//! |---|---|
//! | [`env`] | The game's environment, Go's variables with last-wins keys |
//! | [`hardware`] | The privileged request and the VRR policy from a profile |
//! | [`summary`] | The dry-run summary and the launch message |
//! | [`detect`] | Which game a wrapped command is |
//! | [`vrr`] | KDE VRR: picking the primary output, applying and restoring |
//!
//! Building a plan reads files only: the game's DLLs, and, when vkd3d_heap
//! resolves on, the Proton build and the driver version through
//! [`Probes`]. It never runs pkexec or `kscreen-doctor` and takes no lock.

pub mod detect;
pub mod env;
pub mod hardware;
pub mod summary;
pub mod vrr;

use std::os::unix::fs::PermissionsExt;
use std::path::Path;

pub use env::LaunchEnv;

use crate::backend::{HardwareRequest, VrrPolicy};
use crate::games::Game;
use crate::profile::field::Effect;
use crate::profile::{Effective, FieldKey, Origin, Resolved, Value};
use crate::proton::{self, Build, Compatibility, ResolveError};

/// Which profile a launch applies.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ProfileUse {
    /// The game has its own profile file, which follows All games where it
    /// sets nothing.
    Own,
    /// The game has no profile file, and the All games file applies.
    AllGames,
    /// Neither file exists: nothing but system defaults.
    None,
}

/// A resolved profile and which files it came from.
#[derive(Debug, Clone)]
pub struct ProfileState {
    /// Every field, resolved.
    pub effective: Effective,
    /// Whether the game's own profile file exists.
    pub game_file: bool,
    /// Whether the All games file exists.
    pub all_games_file: bool,
}

impl ProfileState {
    /// Which profile applies.
    pub fn applies(&self) -> ProfileUse {
        if self.game_file {
            ProfileUse::Own
        } else if self.all_games_file {
            ProfileUse::AllGames
        } else {
            ProfileUse::None
        }
    }

    /// No profile at all: the wrapper's fallback when spela's own files do
    /// not load.
    pub fn none(app_id: u64) -> ProfileState {
        ProfileState {
            effective: Effective::of_game(app_id, None, None),
            game_file: false,
            all_games_file: false,
        }
    }
}

/// What the deny list says about the game.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DenyState {
    /// DLL swaps are allowed.
    Allowed,
    /// A list denies DLL swaps, with the reason.
    Denied(String),
    /// The lists could not be read, with the error.
    Unknown(String),
}

/// Everything the plan reads apart from the probes.
#[derive(Debug, Clone)]
pub struct PlanInputs<'a> {
    /// The game.
    pub game: &'a Game,
    /// Its resolved profile.
    pub profile: &'a ProfileState,
    /// The shader cache folder for a profile that names none: the config's
    /// `shader_cache`, else `<cache>/nvidia`.
    pub shader_cache_dir: &'a Path,
    /// The deny list's answer for the game.
    pub deny: DenyState,
    /// Whether the game has a DLL backup.
    pub backup: bool,
    /// Whether spela runs from `/usr/bin/spela`, where the polkit policy
    /// expects it. Elsewhere pkexec asks with its generic action.
    pub at_install_path: bool,
}

/// The two lookups the descriptor heap gate needs. They run only when
/// vkd3d_heap resolves on.
pub trait Probes {
    /// The Proton build Steam runs the game with.
    fn proton(&mut self, app_id: u64) -> Result<Build, ResolveError>;
    /// The NVIDIA driver version: `Some("")` with no NVIDIA driver, `None`
    /// when the driver could not be asked.
    fn driver(&mut self) -> Option<String>;
}

/// One line of the Profile impacts block.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ImpactItem {
    /// The field.
    pub key: FieldKey,
    /// The value as the summary prints it: `unset` for an empty value.
    pub value: String,
    /// Where it comes from: `this_game`, `all_games` or `system_default`.
    pub source: &'static str,
    /// Go's restore class.
    pub restore: &'static str,
}

/// A hardware or VRR field the profile names that changes nothing at
/// launch, so the Hardware block and the impacts agree.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Unchanged {
    /// The field.
    pub key: FieldKey,
    /// Why nothing changes, such as `left unchanged (gpu.fan_speed=0)`.
    pub why: String,
}

/// One detected DLL in the DLL block.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DllLine {
    /// The family word, such as `dlssg`.
    pub family: &'static str,
    /// The file name as found.
    pub name: String,
    /// The version, or `none`.
    pub version: String,
    /// Go's write check on the file: `write bit present`, `no write bit` or
    /// `unknown (path not accessible)`.
    pub write: &'static str,
}

/// The descriptor heap check, when vkd3d_heap resolves on.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Preflight {
    /// The check's outcome on each axis.
    pub compatibility: Compatibility,
    /// The notice to show, if any.
    pub notice: Option<String>,
}

impl Preflight {
    /// Logs the check with Go's messages. It never blocks a launch.
    pub fn log(&self, app_id: u64) {
        let c = &self.compatibility;
        if let Some(reason) = c.proton_skip {
            tracing::info!(
                reason,
                app_id,
                "vkd3d_heap: could not resolve Proton for appID; skipping Proton compatibility check"
            );
        }
        if let Some(reason) = c.driver_skip {
            tracing::info!(
                reason,
                app_id,
                "vkd3d_heap: NVIDIA driver not detected; skipping driver compatibility check"
            );
        }
        if !c.proton_ok && c.proton_skip.is_none() {
            tracing::warn!(
                detected = %c.proton_detected,
                minimum = proton::MIN_PROTON_CACHYOS_BUILD,
                "vkd3d_heap: Proton build does not support descriptor_heap"
            );
        }
        if !c.driver_ok && c.driver_skip.is_none() {
            tracing::warn!(
                detected = %c.driver_detected,
                minimum = proton::MIN_DRIVER_VERSION,
                "vkd3d_heap: NVIDIA driver below minimum"
            );
        }
    }
}

/// What a launch of one game does.
#[derive(Debug, Clone)]
pub struct LaunchPlan {
    /// The game's AppID.
    pub app_id: u64,
    /// The game's name.
    pub name: String,
    /// Which profile applies.
    pub profile: ProfileUse,
    /// How many settings the game's own profile holds.
    pub own_count: usize,
    /// The Profile impacts lines, in table order.
    pub impacts: Vec<ImpactItem>,
    /// The game's environment: the profile's variables and the launcher's.
    pub env: LaunchEnv,
    /// The KDE VRR policy to set, if any.
    pub vrr: Option<VrrPolicy>,
    /// The privileged changes.
    pub hardware: HardwareRequest,
    /// Hardware and VRR fields the profile names that change nothing.
    pub unchanged: Vec<Unchanged>,
    /// The descriptor heap check, when vkd3d_heap resolves on.
    pub preflight: Option<Preflight>,
    /// The deny list's answer.
    pub deny: DenyState,
    /// Whether a DLL backup exists.
    pub backup: bool,
    /// The detected DLLs, by family word and then name, as Go sorted them.
    pub dlls: Vec<DllLine>,
    /// Whether pkexec uses the spela policy (spela at `/usr/bin/spela`).
    pub at_install_path: bool,
}

impl LaunchPlan {
    /// Whether the plan changes the display or the hardware, which needs
    /// the launch lock.
    pub fn changes_system(&self) -> bool {
        self.vrr.is_some() || !self.hardware.is_empty()
    }

    /// The same plan with VRR and the privileged changes dropped: what a
    /// second launch does while another holds the launch lock (decision 33).
    pub fn env_only(&self) -> LaunchPlan {
        LaunchPlan {
            vrr: None,
            hardware: HardwareRequest::default(),
            ..self.clone()
        }
    }
}

/// Builds the plan for one game.
pub fn plan(inputs: &PlanInputs<'_>, probes: &mut dyn Probes) -> LaunchPlan {
    let effective = &inputs.profile.effective;
    let game = inputs.game;
    let mut env = env::profile_env(effective, inputs.shader_cache_dir);
    let preflight = if env::vkd3d_heap_on(effective) {
        let build = probes.proton(game.app_id);
        // The launcher's variable, after the profile's (architecture,
        // "Environment and precedence").
        if proton::needs_heap_env(&build) {
            env.set(env::PROTON_VKD3D_HEAP, "1");
        }
        let driver = probes.driver();
        let compatibility = proton::check(&build, driver.as_deref());
        let notice = compatibility.notice();
        Some(Preflight {
            compatibility,
            notice,
        })
    } else {
        None
    };
    let (hardware, refused) = hardware::hardware_request(effective);
    let vrr = hardware::vrr_policy(effective);
    let impacts: Vec<ImpactItem> = effective.iter().filter_map(impact).collect();
    let unchanged = unchanged(effective, &impacts, &hardware, vrr, &refused);
    let own_count = if inputs.profile.game_file {
        effective.own_count()
    } else {
        0
    };
    LaunchPlan {
        app_id: game.app_id,
        name: game.name.clone(),
        profile: inputs.profile.applies(),
        own_count,
        impacts,
        env,
        vrr,
        hardware,
        unchanged,
        preflight,
        deny: inputs.deny.clone(),
        backup: inputs.backup,
        dlls: dll_lines(game),
        at_install_path: inputs.at_install_path,
    }
}

/// One field's impacts line: shown when the game sets it, or when its
/// value is not the zero, as Go's summary filtered them.
fn impact(r: &Resolved) -> Option<ImpactItem> {
    if !r.is_own() && r.value.is_zero() {
        return None;
    }
    let field = r.key.field();
    Some(ImpactItem {
        key: r.key,
        value: summary_value(r),
        source: match r.origin {
            Origin::ThisGame | Origin::ThisGameSame | Origin::SetHere => "this_game",
            Origin::AllGames => "all_games",
            Origin::SystemDefault => "system_default",
        },
        restore: field.effect.restore(),
    })
}

/// Go's `summaryValue`: the value as text, `unset` when empty. SMT left
/// unchanged on purpose reads `unchanged` (decision 65).
fn summary_value(r: &Resolved) -> String {
    match &r.value {
        Value::OptionalBool(None) if r.is_own() => crate::profile::words::UNCHANGED.to_owned(),
        v => {
            let text = v.canonical();
            if text.trim().is_empty() {
                "unset".to_owned()
            } else {
                text
            }
        }
    }
}

/// The hardware and VRR fields the impacts list that the plan does not
/// change: an own zero, an empty governor, SMT left unchanged, VRR
/// `unset`, or a value the helper cannot take.
fn unchanged(
    effective: &Effective,
    impacts: &[ImpactItem],
    request: &HardwareRequest,
    vrr: Option<VrrPolicy>,
    refused: &[(FieldKey, String)],
) -> Vec<Unchanged> {
    let mut out = Vec::new();
    for item in impacts {
        let field = item.key.field();
        if field.effect == Effect::Environment {
            continue;
        }
        if let Some((_, why)) = refused.iter().find(|(k, _)| *k == item.key) {
            out.push(Unchanged {
                key: item.key,
                why: format!("not applied: {why}"),
            });
            continue;
        }
        let applied = match item.key {
            FieldKey::GpuClockOffset => request.clock_offset_mhz.is_some(),
            FieldKey::GpuMemoryOffset => request.memory_offset_mhz.is_some(),
            FieldKey::GpuPowerLimit => request.power_limit_w.is_some(),
            FieldKey::GpuFanSpeed => request.fan.is_some(),
            FieldKey::CpuGovernor => request.cpu_governor.is_some(),
            FieldKey::CpuSmt => request.smt.is_some(),
            FieldKey::GpuVrr => vrr.is_some(),
            _ => true,
        };
        if !applied && effective.get(item.key).is_own() {
            out.push(Unchanged {
                key: item.key,
                why: format!("left unchanged ({}={})", field.key, item.value),
            });
        }
    }
    out
}

/// The game's DLLs as Go's summary listed them: by family word, then name.
fn dll_lines(game: &Game) -> Vec<DllLine> {
    let mut lines: Vec<DllLine> = game
        .dlls
        .iter()
        .map(|d| DllLine {
            family: d.dll_type.as_str(),
            name: d.name.clone(),
            version: d
                .version
                .clone()
                .filter(|v| !v.is_empty())
                .unwrap_or_else(|| crate::profile::words::NONE.to_owned()),
            write: write_bit(&d.path),
        })
        .collect();
    lines.sort_by(|a, b| a.family.cmp(b.family).then_with(|| a.name.cmp(&b.name)));
    lines
}

/// Go's `dllPathWriteOutcome`.
fn write_bit(path: &Path) -> &'static str {
    match std::fs::metadata(path) {
        Err(_) => "unknown (path not accessible)",
        Ok(m) if m.permissions().mode() & 0o222 == 0 => "no write bit",
        Ok(_) => "write bit present",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::backend::FanSetting;
    use crate::games::{DetectedDll, DllType};
    use crate::profile::OwnValues;

    struct NoProbes {
        proton: Option<Build>,
        driver: Option<String>,
        calls: usize,
    }

    impl Probes for NoProbes {
        fn proton(&mut self, _: u64) -> Result<Build, ResolveError> {
            self.calls += 1;
            self.proton.clone().ok_or(ResolveError::NoSteamRoot)
        }
        fn driver(&mut self) -> Option<String> {
            self.calls += 1;
            self.driver.clone()
        }
    }

    fn unresolved() -> NoProbes {
        NoProbes {
            proton: None,
            driver: Some("590.48.01".into()),
            calls: 0,
        }
    }

    fn game(dlls: Vec<DetectedDll>) -> Game {
        Game {
            app_id: 9100010,
            name: "Glimmer Run".into(),
            install_dir: "/lib/steamapps/common/Glimmer".into(),
            library_path: "/lib".into(),
            prefix_path: None,
            scanned_at: jiff::Timestamp::UNIX_EPOCH,
            dlls,
        }
    }

    fn state(game: &[(FieldKey, Value)], all: &[(FieldKey, Value)]) -> ProfileState {
        let mut g = OwnValues::new();
        for (k, v) in game {
            g.set(*k, v.clone()).unwrap();
        }
        let mut a = OwnValues::new();
        for (k, v) in all {
            a.set(*k, v.clone()).unwrap();
        }
        ProfileState {
            effective: Effective::of_game(9100010, Some(&g), Some(&a)),
            game_file: true,
            all_games_file: true,
        }
    }

    fn build(profile: &ProfileState, probes: &mut NoProbes) -> LaunchPlan {
        let g = game(vec![]);
        plan(
            &PlanInputs {
                game: &g,
                profile,
                shader_cache_dir: Path::new("/cache/nvidia"),
                deny: DenyState::Allowed,
                backup: false,
                at_install_path: true,
            },
            probes,
        )
    }

    #[test]
    fn the_heap_gate_runs_only_when_vkd3d_heap_is_on() {
        let mut probes = unresolved();
        let p = build(&state(&[], &[]), &mut probes);
        assert_eq!(probes.calls, 0, "no Proton or driver lookup");
        assert!(p.preflight.is_none());
        assert_eq!(p.env.get(env::PROTON_VKD3D_HEAP), None);

        let on = state(&[(FieldKey::ProtonVkd3dHeap, Value::Bool(true))], &[]);
        let p = build(&on, &mut probes);
        assert_eq!(probes.calls, 2);
        assert_eq!(
            p.env.get(env::PROTON_VKD3D_HEAP),
            Some("1"),
            "unresolved Proton"
        );
        assert_eq!(p.env.get("VKD3D_CONFIG"), Some("descriptor_heap"));
        let notice = p.preflight.unwrap().notice.unwrap();
        assert!(
            notice.contains("could not resolve active Proton"),
            "{notice}"
        );
    }

    #[test]
    fn a_modern_build_needs_no_heap_variable() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(dir.path().join("proton"), "#!/bin/sh\n").unwrap();
        let mut probes = NoProbes {
            proton: Some(Build {
                name: "proton-cachyos-11.0-20260601-slr".into(),
                path: dir.path().to_owned(),
            }),
            driver: Some("590.48.01".into()),
            calls: 0,
        };
        let on = state(&[(FieldKey::ProtonVkd3dHeap, Value::Bool(true))], &[]);
        let p = build(&on, &mut probes);
        assert_eq!(p.env.get(env::PROTON_VKD3D_HEAP), None);
        assert_eq!(p.preflight.unwrap().notice, None, "all is well");
        // The legacy gate adds it.
        std::fs::write(dir.path().join("proton"), "export PROTON_VKD3D_HEAP=1\n").unwrap();
        let p = build(&on, &mut probes);
        assert_eq!(p.env.get(env::PROTON_VKD3D_HEAP), Some("1"));
    }

    #[test]
    fn impacts_show_own_and_non_zero_values_with_three_sources() {
        let s = state(
            &[
                (FieldKey::ProtonEnableHdr, Value::Bool(true)),
                (FieldKey::GpuFanSpeed, Value::Int(0)),
                (FieldKey::CpuSmt, Value::OptionalBool(None)),
                (FieldKey::GpuVrr, Value::Text("unset".into())),
            ],
            &[
                (FieldKey::CpuGovernor, Value::Text("performance".into())),
                (FieldKey::GpuFanSpeed, Value::Int(70)),
                (FieldKey::CpuSmt, Value::OptionalBool(Some(false))),
                (FieldKey::ProtonEnableWayland, Value::Bool(false)),
            ],
        );
        let p = build(&s, &mut unresolved());
        let shown: Vec<(&str, &str, &str)> = p
            .impacts
            .iter()
            .map(|i| (i.key.as_str(), i.value.as_str(), i.source))
            .collect();
        assert_eq!(
            shown,
            [
                ("proton.enable_hdr", "true", "this_game"),
                ("gpu.vrr", "unset", "this_game"),
                ("gpu.fan_speed", "0", "this_game"),
                ("cpu.governor", "performance", "all_games"),
                ("cpu.smt", "unchanged", "this_game"),
            ]
        );
        assert_eq!(p.own_count, 4);
        assert_eq!(p.hardware.fan, None);
        assert_eq!(p.hardware.smt, None, "decision 65: no SMT step");
        assert_eq!(p.hardware.cpu_governor.as_deref(), Some("performance"));
        let why: Vec<(&str, &str)> = p
            .unchanged
            .iter()
            .map(|u| (u.key.as_str(), u.why.as_str()))
            .collect();
        assert_eq!(
            why,
            [
                ("gpu.vrr", "left unchanged (gpu.vrr=unset)"),
                ("gpu.fan_speed", "left unchanged (gpu.fan_speed=0)"),
                ("cpu.smt", "left unchanged (cpu.smt=unchanged)"),
            ]
        );
        assert!(p.changes_system());
        let env_only = p.env_only();
        assert!(!env_only.changes_system());
        assert_eq!(env_only.env, p.env);
    }

    #[test]
    fn profile_use_and_counts() {
        let mut s = state(&[(FieldKey::GpuPowerLimit, Value::Int(250))], &[]);
        assert_eq!(s.applies(), ProfileUse::Own);
        let p = build(&s, &mut unresolved());
        assert_eq!(p.hardware.power_limit_w, Some(250));
        assert_eq!(p.own_count, 1);
        s.game_file = false;
        assert_eq!(s.applies(), ProfileUse::AllGames);
        s.all_games_file = false;
        assert_eq!(s.applies(), ProfileUse::None);
        assert_eq!(ProfileState::none(1).applies(), ProfileUse::None);
        let fan = state(&[(FieldKey::GpuFanSpeed, Value::Int(40))], &[]);
        assert_eq!(
            build(&fan, &mut unresolved()).hardware.fan,
            Some(FanSetting::Percent(40))
        );
    }

    #[test]
    fn dll_lines_sort_by_family_word_then_name_with_go_s_write_check() {
        let dir = tempfile::tempdir().unwrap();
        let ro = dir.path().join("ro.dll");
        std::fs::write(&ro, "x").unwrap();
        std::fs::set_permissions(&ro, std::fs::Permissions::from_mode(0o444)).unwrap();
        let rw = dir.path().join("nvngx_dlss.dll");
        std::fs::write(&rw, "x").unwrap();
        let dll = |path: &Path, name: &str, t: DllType, v: Option<&str>| DetectedDll {
            path: path.to_owned(),
            name: name.into(),
            dll_type: t,
            version: v.map(str::to_owned),
        };
        let g = game(vec![
            dll(
                &dir.path().join("gone.dll"),
                "nvngx_dlssg.dll",
                DllType::Dlssg,
                Some("3.7.0"),
            ),
            dll(&ro, "libxess.dll", DllType::Xess, None),
            dll(&rw, "nvngx_dlss.dll", DllType::Dlss, Some("310.2.1")),
            dll(&rw, "nvngx_dlssd.dll", DllType::Dlssd, Some("310.5.3")),
        ]);
        let lines: Vec<String> = dll_lines(&g)
            .iter()
            .map(|l| format!("{}: {} {}; {}", l.family, l.name, l.version, l.write))
            .collect();
        assert_eq!(
            lines,
            [
                "dlss: nvngx_dlss.dll 310.2.1; write bit present",
                "dlssd: nvngx_dlssd.dll 310.5.3; write bit present",
                "dlssg: nvngx_dlssg.dll 3.7.0; unknown (path not accessible)",
                "xess: libxess.dll none; no write bit",
            ]
        );
    }
}
