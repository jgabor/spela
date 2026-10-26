//! The one privileged round trip.
//!
//! Decision 30 sets the shape: the wrapper starts one long-lived root helper
//! per launch, behind one polkit prompt. The helper reads each prior value
//! as root, applies the batch and reports every step. Then it waits. A
//! restore request, or the end of its input because the wrapper died, makes
//! it restore the applied steps from their prior values and exit.
//!
//! In this trait, [`Privilege::start`] starts that helper and returns a
//! [`PrivilegedSession`]. [`PrivilegedSession::restore`] asks for the
//! restore. Dropping a session without it closes the helper's input, and
//! the helper restores as it does when the wrapper dies.
//!
//! [`Privilege::apply_once`] is the one-shot form for the live `gpu reset`,
//! `cpu governor` and `cpu smt` commands. Their change is meant to last, so
//! it never restores.
//!
//! In a launch, a setting whose prior value cannot be read is skipped,
//! never applied: spela never restores a guess. A failed step does not stop
//! the steps after it. A one-shot change is never restored, so there a
//! prior that cannot be read does not stop the change: the step is applied
//! and reported as [`StepOutcome::AppliedNoRestore`]. [`Step::LockedClocks`],
//! the reset that `gpu reset` sends, reads no prior at all and is always
//! reported that way.

use std::fmt;

use super::gpu::FanSetting;

/// One privileged setting, in the order the helper applies them.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Step {
    /// GPU locked clocks, reset to the driver default. Only `gpu reset`
    /// asks for it, and nothing restores it.
    LockedClocks,
    /// GPU graphics clock offset.
    ClockOffset,
    /// GPU memory clock offset.
    MemoryOffset,
    /// GPU power limit.
    PowerLimit,
    /// GPU fan.
    Fan,
    /// CPU frequency governor, on every online CPU.
    CpuGovernor,
    /// CPU simultaneous multithreading.
    Smt,
}

impl Step {
    /// Every step, in apply order.
    pub const ALL: [Step; 7] = [
        Step::LockedClocks,
        Step::ClockOffset,
        Step::MemoryOffset,
        Step::PowerLimit,
        Step::Fan,
        Step::CpuGovernor,
        Step::Smt,
    ];

    /// The name used in reports and logs.
    pub fn as_str(self) -> &'static str {
        match self {
            Step::LockedClocks => "locked_clocks",
            Step::ClockOffset => "clock_offset",
            Step::MemoryOffset => "memory_offset",
            Step::PowerLimit => "power_limit",
            Step::Fan => "fan",
            Step::CpuGovernor => "governor",
            Step::Smt => "smt",
        }
    }

    /// Whether a restore can put this step back. A step that cannot is
    /// applied without reading a prior value and never restored.
    pub fn is_restorable(self) -> bool {
        self != Step::LockedClocks
    }
}

impl fmt::Display for Step {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A value one step sets or restores.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SettingValue {
    /// The driver's own setting, the target of a reset.
    DriverDefault,
    /// A clock offset in MHz.
    Mhz(i32),
    /// A power limit in W.
    Watts(u32),
    /// A fan setting.
    Fan(FanSetting),
    /// A CPU governor name.
    Governor(String),
    /// SMT on or off.
    Smt(bool),
}

impl fmt::Display for SettingValue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SettingValue::DriverDefault => f.write_str("driver default"),
            SettingValue::Mhz(v) => write!(f, "{v} MHz"),
            SettingValue::Watts(v) => write!(f, "{v} W"),
            SettingValue::Fan(v) => write!(f, "{v}"),
            SettingValue::Governor(v) => f.write_str(v),
            SettingValue::Smt(true) => f.write_str("on"),
            SettingValue::Smt(false) => f.write_str("off"),
        }
    }
}

/// What a launch or a live command asks the helper to change. `None` leaves
/// a setting alone.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct HardwareRequest {
    /// Reset the GPU's locked clocks to the driver default.
    pub reset_locked_clocks: bool,
    /// GPU graphics clock offset in MHz.
    pub clock_offset_mhz: Option<i32>,
    /// GPU memory clock offset in MHz.
    pub memory_offset_mhz: Option<i32>,
    /// GPU power limit in W.
    pub power_limit_w: Option<u32>,
    /// GPU fan.
    pub fan: Option<FanSetting>,
    /// CPU governor for every online CPU.
    pub cpu_governor: Option<String>,
    /// CPU SMT.
    pub smt: Option<bool>,
}

impl HardwareRequest {
    /// What `gpu reset` sends: locked clocks reset and both VF offsets at
    /// 0.
    pub fn gpu_reset() -> Self {
        HardwareRequest {
            reset_locked_clocks: true,
            clock_offset_mhz: Some(0),
            memory_offset_mhz: Some(0),
            ..HardwareRequest::default()
        }
    }

    /// Whether the request changes nothing, so no prompt is needed.
    pub fn is_empty(&self) -> bool {
        self.steps().is_empty()
    }

    /// The requested steps with their targets, in apply order.
    pub fn steps(&self) -> Vec<(Step, SettingValue)> {
        let mut out = Vec::new();
        if self.reset_locked_clocks {
            out.push((Step::LockedClocks, SettingValue::DriverDefault));
        }
        if let Some(v) = self.clock_offset_mhz {
            out.push((Step::ClockOffset, SettingValue::Mhz(v)));
        }
        if let Some(v) = self.memory_offset_mhz {
            out.push((Step::MemoryOffset, SettingValue::Mhz(v)));
        }
        if let Some(v) = self.power_limit_w {
            out.push((Step::PowerLimit, SettingValue::Watts(v)));
        }
        if let Some(v) = self.fan {
            out.push((Step::Fan, SettingValue::Fan(v)));
        }
        if let Some(v) = &self.cpu_governor {
            out.push((Step::CpuGovernor, SettingValue::Governor(v.clone())));
        }
        if let Some(v) = self.smt {
            out.push((Step::Smt, SettingValue::Smt(v)));
        }
        out
    }
}

/// What happened to one step.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StepOutcome {
    /// Changed. `prior` is what was there, read as root before the change.
    Applied {
        /// The value a restore puts back.
        prior: SettingValue,
    },
    /// Changed, and never restored: a step that is not
    /// [`Step::is_restorable`].
    AppliedNoRestore,
    /// The change failed, so nothing needs restoring for this step.
    Failed(String),
    /// Not attempted, with the reason.
    Skipped(String),
}

/// One line of the helper's report.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StepReport {
    /// The setting.
    pub step: Step,
    /// What it was set to, or was meant to be.
    pub target: SettingValue,
    /// What happened.
    pub outcome: StepOutcome,
}

/// The helper's report on an apply: one entry per requested step.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ApplyReport {
    /// In apply order.
    pub steps: Vec<StepReport>,
}

impl ApplyReport {
    /// Whether every step was applied.
    pub fn all_applied(&self) -> bool {
        self.steps.iter().all(|s| {
            matches!(
                s.outcome,
                StepOutcome::Applied { .. } | StepOutcome::AppliedNoRestore
            )
        })
    }

    /// What a restore puts back: every applied step, in reverse apply
    /// order, so SMT comes back before the governor and re-onlined CPUs get
    /// the restored governor.
    pub fn restore_plan(&self) -> RestorePlan {
        RestorePlan {
            steps: self
                .steps
                .iter()
                .rev()
                .filter_map(|s| match &s.outcome {
                    StepOutcome::Applied { prior: value } => Some(RestoreStep {
                        step: s.step,
                        value: value.clone(),
                    }),
                    _ => None,
                })
                .collect(),
        }
    }
}

/// One setting to put back.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RestoreStep {
    /// The setting.
    pub step: Step,
    /// The value read before the change.
    pub value: SettingValue,
}

/// Everything a restore puts back, in the order it runs.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct RestorePlan {
    /// Reverse apply order.
    pub steps: Vec<RestoreStep>,
}

/// The helper's report on a restore: one result per planned step.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct RestoreReport {
    /// In restore order. A failure does not stop the next step.
    pub steps: Vec<(Step, Result<(), String>)>,
}

/// Why the privileged round trip failed.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum PrivilegeError {
    /// The user dismissed or cancelled the prompt. Nothing changed.
    #[error("authentication was dismissed")]
    Dismissed,
    /// The prompt cannot run, for example because polkit is missing.
    #[error("privileged changes are not available: {0}")]
    Unavailable(String),
    /// The helper failed. The text is the helper's.
    #[error("the privileged helper failed: {0}")]
    Helper(String),
}

/// The root helper, behind one polkit prompt per call.
pub trait Privilege: Send {
    /// Starts a helper that applies one batch and waits to restore it. The
    /// session's report has one entry per step, including failed and
    /// skipped ones, so a partial apply still restores what it changed.
    fn start(
        &mut self,
        request: &HardwareRequest,
    ) -> Result<Box<dyn PrivilegedSession>, PrivilegeError>;

    /// Applies one batch that is meant to last, and never restores it.
    fn apply_once(&mut self, request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError>;
}

/// A running helper that applied a batch and waits.
pub trait PrivilegedSession: Send {
    /// What the helper did.
    fn report(&self) -> &ApplyReport;
    /// Asks the helper to restore [`ApplyReport::restore_plan`] and exit.
    fn restore(self: Box<Self>) -> Result<RestoreReport, PrivilegeError>;
}

#[cfg(test)]
mod tests {
    use super::*;

    fn applied(step: Step, target: SettingValue, prior: SettingValue) -> StepReport {
        StepReport {
            step,
            target,
            outcome: StepOutcome::Applied { prior },
        }
    }

    #[test]
    fn request_steps_follow_apply_order() {
        let request = HardwareRequest {
            smt: Some(false),
            power_limit_w: Some(250),
            clock_offset_mhz: Some(-100),
            cpu_governor: Some("performance".into()),
            ..HardwareRequest::default()
        };
        let steps: Vec<Step> = request.steps().into_iter().map(|(s, _)| s).collect();
        assert_eq!(
            steps,
            [
                Step::ClockOffset,
                Step::PowerLimit,
                Step::CpuGovernor,
                Step::Smt
            ]
        );
        assert!(!request.is_empty());
        assert!(HardwareRequest::default().is_empty());
        assert_eq!(Step::ALL.len(), 7);
        assert!(Step::ALL.windows(2).all(|w| w[0] < w[1]));
    }

    #[test]
    fn restore_plan_reverses_and_leaves_out_failed_and_skipped() {
        let report = ApplyReport {
            steps: vec![
                applied(
                    Step::ClockOffset,
                    SettingValue::Mhz(100),
                    SettingValue::Mhz(0),
                ),
                StepReport {
                    step: Step::PowerLimit,
                    target: SettingValue::Watts(250),
                    outcome: StepOutcome::Skipped("prior value not readable".into()),
                },
                StepReport {
                    step: Step::Fan,
                    target: SettingValue::Fan(FanSetting::Percent(70)),
                    outcome: StepOutcome::Failed("not supported".into()),
                },
                applied(
                    Step::CpuGovernor,
                    SettingValue::Governor("performance".into()),
                    SettingValue::Governor("powersave".into()),
                ),
                applied(Step::Smt, SettingValue::Smt(false), SettingValue::Smt(true)),
            ],
        };
        assert!(!report.all_applied());
        let plan = report.restore_plan();
        assert_eq!(
            plan.steps,
            [
                RestoreStep {
                    step: Step::Smt,
                    value: SettingValue::Smt(true),
                },
                RestoreStep {
                    step: Step::CpuGovernor,
                    value: SettingValue::Governor("powersave".into()),
                },
                RestoreStep {
                    step: Step::ClockOffset,
                    value: SettingValue::Mhz(0),
                },
            ]
        );
    }

    #[test]
    fn gpu_reset_resets_locked_clocks_first_and_zeroes_both_offsets() {
        let request = HardwareRequest::gpu_reset();
        assert_eq!(
            request.steps(),
            [
                (Step::LockedClocks, SettingValue::DriverDefault),
                (Step::ClockOffset, SettingValue::Mhz(0)),
                (Step::MemoryOffset, SettingValue::Mhz(0)),
            ]
        );
        assert!(!Step::LockedClocks.is_restorable());
        assert!(Step::ALL[1..].iter().all(|s| s.is_restorable()));
        let report = ApplyReport {
            steps: vec![
                StepReport {
                    step: Step::LockedClocks,
                    target: SettingValue::DriverDefault,
                    outcome: StepOutcome::AppliedNoRestore,
                },
                applied(
                    Step::ClockOffset,
                    SettingValue::Mhz(0),
                    SettingValue::Mhz(150),
                ),
            ],
        };
        assert!(report.all_applied());
        assert_eq!(
            report.restore_plan().steps,
            [RestoreStep {
                step: Step::ClockOffset,
                value: SettingValue::Mhz(150),
            }]
        );
    }

    #[test]
    fn value_and_step_text() {
        assert_eq!(SettingValue::Mhz(-50).to_string(), "-50 MHz");
        assert_eq!(SettingValue::Watts(300).to_string(), "300 W");
        assert_eq!(SettingValue::Smt(false).to_string(), "off");
        assert_eq!(SettingValue::Fan(FanSetting::Auto).to_string(), "auto");
        assert_eq!(Step::CpuGovernor.to_string(), "governor");
        assert_eq!(SettingValue::DriverDefault.to_string(), "driver default");
    }
}
