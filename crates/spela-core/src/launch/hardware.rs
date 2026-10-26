//! What a launch asks the root helper and the display to change, read from
//! the resolved profile.
//!
//! A privileged step is requested only for a `clock_offset` other than 0, a
//! `memory_offset` other than 0, a `power_limit` above 0, a `fan_speed`
//! above 0, a governor, or a set SMT, as Go's `needsHardwareApply` decides
//! (`internal/profile/apply.go:65-72`). SMT left unchanged (decision 65)
//! adds no SMT step, even when All games sets one. A value that does not
//! fit the helper's type adds no step either, and the plan says so.

use crate::backend::{FanSetting, HardwareRequest, VrrPolicy};
use crate::profile::{Effective, FieldKey, Value};

/// The privileged request for a launch, and the values that could not be
/// sent, by field.
pub fn hardware_request(effective: &Effective) -> (HardwareRequest, Vec<(FieldKey, String)>) {
    let mut request = HardwareRequest::default();
    let mut refused = Vec::new();
    let int = |key: FieldKey| effective.value(key).as_int().unwrap_or(0);
    let mut out_of_range = |key: FieldKey, value: i64| {
        refused.push((key, format!("{value} is out of range")));
    };

    let clock = int(FieldKey::GpuClockOffset);
    if clock != 0 {
        match i32::try_from(clock) {
            Ok(v) => request.clock_offset_mhz = Some(v),
            Err(_) => out_of_range(FieldKey::GpuClockOffset, clock),
        }
    }
    let memory = int(FieldKey::GpuMemoryOffset);
    if memory != 0 {
        match i32::try_from(memory) {
            Ok(v) => request.memory_offset_mhz = Some(v),
            Err(_) => out_of_range(FieldKey::GpuMemoryOffset, memory),
        }
    }
    let power = int(FieldKey::GpuPowerLimit);
    if power > 0 {
        match u32::try_from(power) {
            Ok(v) => request.power_limit_w = Some(v),
            Err(_) => out_of_range(FieldKey::GpuPowerLimit, power),
        }
    }
    let fan = int(FieldKey::GpuFanSpeed);
    if fan > 0 {
        match u8::try_from(fan) {
            Ok(v) if v <= 100 => request.fan = Some(FanSetting::Percent(v)),
            _ => out_of_range(FieldKey::GpuFanSpeed, fan),
        }
    }
    if let Some(governor) = effective.value(FieldKey::CpuGovernor).as_text()
        && !governor.is_empty()
    {
        request.cpu_governor = Some(governor.to_owned());
    }
    if let Value::OptionalBool(Some(on)) = effective.value(FieldKey::CpuSmt) {
        request.smt = Some(*on);
    }
    (request, refused)
}

/// The KDE VRR policy a launch sets, or `None` for empty and `unset`, which
/// change nothing. Loading checks the value, so anything else is a policy.
pub fn vrr_policy(effective: &Effective) -> Option<VrrPolicy> {
    match effective.value(FieldKey::GpuVrr).as_text() {
        None | Some("" | "unset") => None,
        Some(text) => VrrPolicy::parse(text),
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
        Effective::of_game(1, Some(&g), Some(&a))
    }

    #[test]
    fn go_s_rules_for_each_step() {
        let e = effective(
            &[
                (FieldKey::GpuClockOffset, Value::Int(150)),
                (FieldKey::GpuMemoryOffset, Value::Int(-500)),
                (FieldKey::GpuPowerLimit, Value::Int(250)),
                (FieldKey::GpuFanSpeed, Value::Int(60)),
                (FieldKey::CpuSmt, Value::OptionalBool(Some(false))),
            ],
            &[(FieldKey::CpuGovernor, Value::Text("performance".into()))],
        );
        let (r, refused) = hardware_request(&e);
        assert_eq!(
            r,
            HardwareRequest {
                reset_locked_clocks: false,
                clock_offset_mhz: Some(150),
                memory_offset_mhz: Some(-500),
                power_limit_w: Some(250),
                fan: Some(FanSetting::Percent(60)),
                cpu_governor: Some("performance".into()),
                smt: Some(false),
            }
        );
        assert!(refused.is_empty());
    }

    #[test]
    fn zeros_and_unchanged_smt_add_no_step() {
        let e = effective(
            &[
                (FieldKey::GpuFanSpeed, Value::Int(0)),
                (FieldKey::GpuPowerLimit, Value::Int(0)),
                (FieldKey::CpuSmt, Value::OptionalBool(None)),
                (FieldKey::CpuGovernor, Value::Text(String::new())),
            ],
            &[
                (FieldKey::GpuFanSpeed, Value::Int(80)),
                (FieldKey::CpuSmt, Value::OptionalBool(Some(true))),
                (FieldKey::CpuGovernor, Value::Text("powersave".into())),
            ],
        );
        let (r, refused) = hardware_request(&e);
        assert!(r.is_empty(), "{r:?}");
        assert!(refused.is_empty());
    }

    #[test]
    fn values_the_helper_cannot_take_are_refused() {
        let e = effective(
            &[
                (FieldKey::GpuFanSpeed, Value::Int(140)),
                (FieldKey::GpuClockOffset, Value::Int(i64::MAX)),
                (FieldKey::GpuPowerLimit, Value::Int(-5)),
            ],
            &[],
        );
        let (r, refused) = hardware_request(&e);
        assert!(r.is_empty());
        let keys: Vec<FieldKey> = refused.iter().map(|(k, _)| *k).collect();
        assert_eq!(keys, [FieldKey::GpuClockOffset, FieldKey::GpuFanSpeed]);
    }

    #[test]
    fn vrr_unset_and_empty_do_nothing() {
        let vrr = |v: &str| {
            vrr_policy(&effective(
                &[(FieldKey::GpuVrr, Value::Text(v.into()))],
                &[],
            ))
        };
        assert_eq!(vrr(""), None);
        assert_eq!(vrr("unset"), None);
        assert_eq!(vrr("always"), Some(VrrPolicy::Always));
        assert_eq!(vrr("never"), Some(VrrPolicy::Never));
        assert_eq!(vrr("automatic"), Some(VrrPolicy::Automatic));
    }
}
