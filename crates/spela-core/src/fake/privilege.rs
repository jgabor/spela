//! A privilege runner that needs no root.
//!
//! It plays the root helper in process: GPU steps go to a
//! [`FakeGpuControl`], and CPU steps write a sysfs tree under a temp dir
//! that the test passes in. It follows the helper's rules: prior values are
//! read first, a step whose prior cannot be read is skipped, a failed step
//! does not stop the next one, and a session dropped without a restore
//! restores, as the helper does at end of input.
//!
//! These rules are written out here because the `--serve` loop does not
//! exist yet. When Phase 6 builds it, this fake drives that loop over
//! in-memory pipes instead, so its tests check the helper the wrapper
//! really runs (architecture.md, Backends and fakes).

use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, MutexGuard};

use crate::backend::gpu::{FanSetting, GpuControl, Prior};
use crate::backend::privilege::{
    ApplyReport, HardwareRequest, Privilege, PrivilegeError, PrivilegedSession, RestoreReport,
    SettingValue, Step, StepOutcome, StepReport,
};

use super::gpu::FakeGpuControl;

/// A fault the fake privilege runner can show.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PrivilegeFault {
    /// The user dismisses the prompt. Nothing changes.
    Dismissed,
    /// polkit is missing. Nothing changes.
    Unavailable,
    /// Step number `n` of the request, counting from 0, fails.
    FailAtStep(usize),
    /// The wrapper goes away before step number `n`, counting from 0. The
    /// helper sees the end of its input, restores the steps it applied and
    /// exits, so the caller gets an error and no session.
    ParentGone(usize),
    /// No polkit agent can ask for the password. Nothing changes, and the
    /// error carries pkexec's own text.
    NoAgent,
}

/// What pkexec prints when no polkit agent can ask (unverified: the
/// user's live check settles how pkexec reports it).
pub const NO_AGENT_MESSAGE: &str =
    "Error executing command as another user: No authentication agent found.";

/// What the fake was asked to do, for assertions.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct PrivilegeLog {
    /// Prompts shown: one per `start` or `apply_once`, dismissed ones too.
    pub prompts: usize,
    /// Sessions restored on request.
    pub restores: usize,
    /// Sessions dropped without a request, which restore anyway.
    pub restores_on_drop: usize,
    /// Helpers that lost their parent mid-apply and restored on their own.
    pub restores_on_parent_gone: usize,
}

/// The machine the fake changes.
#[derive(Debug, Clone)]
struct Machine {
    control: FakeGpuControl,
    sys_root: Option<PathBuf>,
}

/// Runs the helper's steps in process.
#[derive(Debug, Clone)]
pub struct FakePrivilege {
    machine: Machine,
    fault: Option<PrivilegeFault>,
    log: Arc<Mutex<PrivilegeLog>>,
}

impl FakePrivilege {
    /// A runner over `control` and, for CPU steps, a sysfs tree at
    /// `sys_root` (the folder that stands for `/sys`). Without one, CPU
    /// steps are skipped.
    pub fn new(control: FakeGpuControl, sys_root: Option<PathBuf>) -> Self {
        FakePrivilege {
            machine: Machine { control, sys_root },
            fault: None,
            log: Arc::new(Mutex::new(PrivilegeLog::default())),
        }
    }

    /// Turns a fault on, or off with `None`.
    pub fn set_fault(&mut self, fault: Option<PrivilegeFault>) {
        self.fault = fault;
    }

    /// What happened so far. Clones of this runner share the log.
    pub fn log(&self) -> PrivilegeLog {
        lock(&self.log).clone()
    }

    fn prompt(&self) -> Result<(), PrivilegeError> {
        lock(&self.log).prompts += 1;
        match self.fault {
            Some(PrivilegeFault::Dismissed) => Err(PrivilegeError::Dismissed),
            Some(PrivilegeFault::Unavailable) => Err(PrivilegeError::Unavailable(
                "polkit is missing (fake)".into(),
            )),
            Some(PrivilegeFault::NoAgent) => Err(PrivilegeError::Helper(NO_AGENT_MESSAGE.into())),
            _ => Ok(()),
        }
    }

    /// Applies the request's steps. With [`PrivilegeFault::ParentGone`] it
    /// stops before that step and returns the steps applied so far as the
    /// error.
    fn apply_batch(&mut self, request: &HardwareRequest) -> Result<ApplyReport, ApplyReport> {
        let (fail_at, gone_at) = match self.fault {
            Some(PrivilegeFault::FailAtStep(n)) => (Some(n), None),
            Some(PrivilegeFault::ParentGone(n)) => (None, Some(n)),
            _ => (None, None),
        };
        let steps = request
            .steps()
            .into_iter()
            .enumerate()
            .take_while(|(i, _)| gone_at != Some(*i))
            .map(|(i, (step, target))| {
                if !step.is_restorable() {
                    let outcome = if fail_at == Some(i) {
                        StepOutcome::Failed(format!("fake failure at step {i}"))
                    } else {
                        match self.machine.set(step, &target) {
                            Ok(()) => StepOutcome::AppliedNoRestore,
                            Err(e) => StepOutcome::Failed(e),
                        }
                    };
                    return StepReport {
                        step,
                        target,
                        outcome,
                    };
                }
                let outcome = match self.machine.read(step) {
                    Err(why) => StepOutcome::Skipped(format!("prior value not readable: {why}")),
                    Ok(_) if fail_at == Some(i) => {
                        StepOutcome::Failed(format!("fake failure at step {i}"))
                    }
                    Ok(prior) => match self.machine.set(step, &target) {
                        Ok(()) => StepOutcome::Applied { prior },
                        Err(e) => StepOutcome::Failed(e),
                    },
                };
                StepReport {
                    step,
                    target,
                    outcome,
                }
            })
            .collect();
        let report = ApplyReport { steps };
        if gone_at.is_some_and(|n| n < request.steps().len()) {
            Err(report)
        } else {
            Ok(report)
        }
    }

    /// What the helper does when its input ends mid-apply: put back what
    /// it applied, then exit.
    fn parent_gone(&mut self, partial: &ApplyReport, restore: bool) -> PrivilegeError {
        if restore {
            for s in partial.restore_plan().steps {
                let _ = self.machine.set(s.step, &s.value);
            }
            lock(&self.log).restores_on_parent_gone += 1;
        }
        PrivilegeError::Helper(format!(
            "the helper lost its parent after {} step(s) (fake)",
            partial.steps.len()
        ))
    }
}

impl Privilege for FakePrivilege {
    fn start(
        &mut self,
        request: &HardwareRequest,
    ) -> Result<Box<dyn PrivilegedSession>, PrivilegeError> {
        self.prompt()?;
        let report = match self.apply_batch(request) {
            Ok(report) => report,
            Err(partial) => return Err(self.parent_gone(&partial, true)),
        };
        Ok(Box::new(FakeSession {
            report,
            machine: self.machine.clone(),
            log: Arc::clone(&self.log),
            done: false,
        }))
    }

    fn apply_once(&mut self, request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError> {
        self.prompt()?;
        // A one-shot helper never restores, even when its parent goes.
        self.apply_batch(request)
            .map_err(|partial| self.parent_gone(&partial, false))
    }
}

struct FakeSession {
    report: ApplyReport,
    machine: Machine,
    log: Arc<Mutex<PrivilegeLog>>,
    done: bool,
}

impl FakeSession {
    fn run_restore(&mut self) -> RestoreReport {
        self.done = true;
        let steps = self
            .report
            .restore_plan()
            .steps
            .into_iter()
            .map(|s| (s.step, self.machine.set(s.step, &s.value)))
            .collect();
        RestoreReport { steps }
    }
}

impl PrivilegedSession for FakeSession {
    fn report(&self) -> &ApplyReport {
        &self.report
    }

    fn restore(mut self: Box<Self>) -> Result<RestoreReport, PrivilegeError> {
        let report = self.run_restore();
        lock(&self.log).restores += 1;
        Ok(report)
    }
}

impl Drop for FakeSession {
    fn drop(&mut self) {
        if !self.done {
            self.run_restore();
            lock(&self.log).restores_on_drop += 1;
        }
    }
}

impl Machine {
    fn read(&mut self, step: Step) -> Result<SettingValue, String> {
        let gpu = self.control.read_prior();
        let known = |p: Prior<SettingValue>| match p {
            Prior::Known(v) => Ok(v),
            Prior::Unknown(why) => Err(why),
        };
        match step {
            Step::LockedClocks => Err("locked clocks have no prior value".into()),
            Step::ClockOffset => known(gpu.clock_offset_mhz.map(SettingValue::Mhz)),
            Step::MemoryOffset => known(gpu.memory_offset_mhz.map(SettingValue::Mhz)),
            Step::PowerLimit => known(gpu.power_limit_w.map(SettingValue::Watts)),
            // A fan whose mode cannot be read goes back to automatic.
            Step::Fan => Ok(SettingValue::Fan(
                gpu.fan.known().copied().unwrap_or(FanSetting::Auto),
            )),
            Step::CpuGovernor => {
                let cpu0 = self.cpu_dir()?.join("cpu0/cpufreq/scaling_governor");
                read_trimmed(&cpu0).map(SettingValue::Governor)
            }
            Step::Smt => match read_trimmed(&self.cpu_dir()?.join("smt/control"))?.as_str() {
                "on" => Ok(SettingValue::Smt(true)),
                "off" => Ok(SettingValue::Smt(false)),
                other => Err(format!("SMT control is {other:?}")),
            },
        }
    }

    fn set(&mut self, step: Step, value: &SettingValue) -> Result<(), String> {
        let gpu = |r: Result<(), crate::backend::GpuError>| r.map_err(|e| e.to_string());
        match (step, value) {
            (Step::LockedClocks, SettingValue::DriverDefault) => {
                gpu(self.control.reset_locked_clocks())
            }
            (Step::ClockOffset, SettingValue::Mhz(v)) => gpu(self.control.set_clock_offset(*v)),
            (Step::MemoryOffset, SettingValue::Mhz(v)) => gpu(self.control.set_memory_offset(*v)),
            (Step::PowerLimit, SettingValue::Watts(v)) => gpu(self.control.set_power_limit(*v)),
            (Step::Fan, SettingValue::Fan(FanSetting::Percent(p))) => {
                gpu(self.control.set_fan_speed(*p))
            }
            (Step::Fan, SettingValue::Fan(FanSetting::Auto)) => gpu(self.control.set_fan_auto()),
            (Step::CpuGovernor, SettingValue::Governor(g)) => self.set_governor(g),
            (Step::Smt, SettingValue::Smt(on)) => {
                let smt = self.cpu_dir()?.join("smt");
                std::fs::write(smt.join("control"), if *on { "on" } else { "off" })
                    .map_err(|e| e.to_string())?;
                // The kernel updates `active` with `control`, so a reader of
                // the same tree sees the change.
                let active = smt.join("active");
                if active.exists() {
                    std::fs::write(active, if *on { "1" } else { "0" })
                        .map_err(|e| e.to_string())?;
                }
                Ok(())
            }
            (step, value) => Err(format!("{value} is not a value for {step}")),
        }
    }

    fn set_governor(&self, governor: &str) -> Result<(), String> {
        let cpu = self.cpu_dir()?;
        let available = cpu.join("cpu0/cpufreq/scaling_available_governors");
        if let Ok(list) = std::fs::read_to_string(&available)
            && !list.split_whitespace().any(|g| g == governor)
        {
            return Err(format!("governor {governor} is not available"));
        }
        let entries = std::fs::read_dir(&cpu).map_err(|e| e.to_string())?;
        let mut written = 0;
        for entry in entries.flatten() {
            let name = entry.file_name();
            let name = name.to_string_lossy();
            let is_cpu = name
                .strip_prefix("cpu")
                .is_some_and(|n| !n.is_empty() && n.bytes().all(|b| b.is_ascii_digit()));
            let file = entry.path().join("cpufreq/scaling_governor");
            if is_cpu && file.exists() {
                std::fs::write(&file, governor).map_err(|e| e.to_string())?;
                written += 1;
            }
        }
        if written == 0 {
            return Err("no CPU has a governor".into());
        }
        Ok(())
    }

    fn cpu_dir(&self) -> Result<PathBuf, String> {
        self.sys_root
            .as_ref()
            .map(|root| root.join("devices/system/cpu"))
            .ok_or_else(|| "no sysfs tree (fake)".to_owned())
    }
}

fn read_trimmed(path: &Path) -> Result<String, String> {
    std::fs::read_to_string(path)
        .map(|s| s.trim().to_owned())
        .map_err(|e| format!("{}: {e}", path.display()))
}

fn lock(log: &Mutex<PrivilegeLog>) -> MutexGuard<'_, PrivilegeLog> {
    log.lock().unwrap_or_else(|e| e.into_inner())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fake::GpuCall;

    /// A sysfs tree with two CPUs on powersave and SMT on.
    fn sysfs() -> tempfile::TempDir {
        let root = tempfile::tempdir().unwrap();
        let cpu = root.path().join("devices/system/cpu");
        for n in 0..2 {
            let dir = cpu.join(format!("cpu{n}/cpufreq"));
            std::fs::create_dir_all(&dir).unwrap();
            std::fs::write(dir.join("scaling_governor"), "powersave\n").unwrap();
            std::fs::write(
                dir.join("scaling_available_governors"),
                "performance powersave\n",
            )
            .unwrap();
        }
        std::fs::create_dir_all(cpu.join("smt")).unwrap();
        std::fs::write(cpu.join("smt/control"), "on\n").unwrap();
        root
    }

    fn governor(root: &Path, n: u32) -> String {
        read_trimmed(&root.join(format!(
            "devices/system/cpu/cpu{n}/cpufreq/scaling_governor"
        )))
        .unwrap()
    }

    fn smt(root: &Path) -> String {
        read_trimmed(&root.join("devices/system/cpu/smt/control")).unwrap()
    }

    fn full_request() -> HardwareRequest {
        HardwareRequest {
            clock_offset_mhz: Some(150),
            power_limit_w: Some(250),
            fan: Some(FanSetting::Percent(70)),
            cpu_governor: Some("performance".into()),
            smt: Some(false),
            ..HardwareRequest::default()
        }
    }

    #[test]
    fn start_applies_and_restore_puts_back_in_reverse() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), Some(sys.path().to_owned()));
        let session = privilege.start(&full_request()).unwrap();
        assert!(session.report().all_applied());
        assert_eq!(control.current(), (150, 0, 250, FanSetting::Percent(70)));
        assert_eq!(governor(sys.path(), 0), "performance");
        assert_eq!(governor(sys.path(), 1), "performance");
        assert_eq!(smt(sys.path()), "off");

        let restored = session.restore().unwrap();
        let order: Vec<Step> = restored.steps.iter().map(|(s, _)| *s).collect();
        assert_eq!(
            order,
            [
                Step::Smt,
                Step::CpuGovernor,
                Step::Fan,
                Step::PowerLimit,
                Step::ClockOffset
            ]
        );
        assert!(restored.steps.iter().all(|(_, r)| r.is_ok()));
        assert_eq!(control.current(), (0, 0, 320, FanSetting::Auto));
        assert_eq!(governor(sys.path(), 1), "powersave");
        assert_eq!(smt(sys.path()), "on");
        assert_eq!(
            privilege.log(),
            PrivilegeLog {
                prompts: 1,
                restores: 1,
                restores_on_drop: 0,
                restores_on_parent_gone: 0,
            }
        );
    }

    #[test]
    fn dropped_session_restores_like_end_of_input() {
        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), None);
        let request = HardwareRequest {
            power_limit_w: Some(200),
            ..HardwareRequest::default()
        };
        drop(privilege.start(&request).unwrap());
        assert_eq!(control.current().2, 320);
        assert_eq!(privilege.log().restores_on_drop, 1);
    }

    #[test]
    fn apply_once_never_restores() {
        let sys = sysfs();
        let mut privilege = FakePrivilege::new(FakeGpuControl::new(), Some(sys.path().into()));
        let request = HardwareRequest {
            cpu_governor: Some("performance".into()),
            ..HardwareRequest::default()
        };
        assert!(privilege.apply_once(&request).unwrap().all_applied());
        assert_eq!(governor(sys.path(), 0), "performance");
        let log = privilege.log();
        assert_eq!((log.restores, log.restores_on_drop), (0, 0));
    }

    #[test]
    fn dismissed_or_unavailable_changes_nothing() {
        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), None);
        privilege.set_fault(Some(PrivilegeFault::Dismissed));
        assert_eq!(
            privilege.start(&full_request()).err(),
            Some(PrivilegeError::Dismissed)
        );
        privilege.set_fault(Some(PrivilegeFault::Unavailable));
        assert!(matches!(
            privilege.apply_once(&full_request()),
            Err(PrivilegeError::Unavailable(_))
        ));
        assert!(control.calls().is_empty());
        assert_eq!(privilege.log().prompts, 2);
    }

    #[test]
    fn failed_step_does_not_stop_the_rest_and_is_not_restored() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), Some(sys.path().into()));
        privilege.set_fault(Some(PrivilegeFault::FailAtStep(1)));
        let session = privilege.start(&full_request()).unwrap();
        let outcomes: Vec<bool> = session
            .report()
            .steps
            .iter()
            .map(|s| matches!(s.outcome, StepOutcome::Applied { .. }))
            .collect();
        assert_eq!(outcomes, [true, false, true, true, true]);
        session.restore().unwrap();
        assert!(!control.calls().contains(&GpuCall::PowerLimit(250)));
        assert_eq!(control.current(), (0, 0, 320, FanSetting::Auto));
    }

    #[test]
    fn unreadable_priors_are_skipped_except_the_fan() {
        let control = FakeGpuControl::new();
        control.set_prior_unreadable(true);
        let mut privilege = FakePrivilege::new(control.clone(), None);
        let session = privilege.start(&full_request()).unwrap();
        let skipped: Vec<Step> = session
            .report()
            .steps
            .iter()
            .filter(|s| matches!(s.outcome, StepOutcome::Skipped(_)))
            .map(|s| s.step)
            .collect();
        // No sysfs tree, and every GPU prior is unknown. The fan alone has a
        // safe fallback: automatic.
        assert_eq!(
            skipped,
            [
                Step::ClockOffset,
                Step::PowerLimit,
                Step::CpuGovernor,
                Step::Smt
            ]
        );
        session.restore().unwrap();
        assert_eq!(control.calls(), [GpuCall::FanSpeed(70), GpuCall::FanAuto]);
    }

    #[test]
    fn gpu_reset_resets_without_a_prior_and_restores_nothing_for_it() {
        let control = FakeGpuControl::new();
        control.set_prior_unreadable(true);
        let mut privilege = FakePrivilege::new(control.clone(), None);
        let report = privilege.apply_once(&HardwareRequest::gpu_reset()).unwrap();
        assert_eq!(report.steps[0].outcome, StepOutcome::AppliedNoRestore);
        // The offsets still need a readable prior.
        assert!(matches!(report.steps[1].outcome, StepOutcome::Skipped(_)));
        assert_eq!(control.calls(), [GpuCall::ResetLockedClocks]);
        assert!(report.restore_plan().steps.is_empty());

        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), None);
        assert!(
            privilege
                .apply_once(&HardwareRequest::gpu_reset())
                .unwrap()
                .all_applied()
        );
        assert_eq!(
            control.calls(),
            [
                GpuCall::ResetLockedClocks,
                GpuCall::ClockOffset(0),
                GpuCall::MemoryOffset(0)
            ]
        );
    }

    #[test]
    fn a_lost_parent_restores_what_was_applied_and_returns_no_session() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), Some(sys.path().into()));
        // Steps: clock, power, fan, governor, SMT. Gone before the governor.
        privilege.set_fault(Some(PrivilegeFault::ParentGone(3)));
        let err = privilege.start(&full_request()).err().unwrap();
        assert!(matches!(err, PrivilegeError::Helper(_)), "{err:?}");
        assert_eq!(control.current(), (0, 0, 320, FanSetting::Auto));
        assert_eq!(governor(sys.path(), 0), "powersave", "never applied");
        assert_eq!(smt(sys.path()), "on");
        assert_eq!(privilege.log().restores_on_parent_gone, 1);

        // A one-shot helper leaves its applied steps in place.
        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), None);
        privilege.set_fault(Some(PrivilegeFault::ParentGone(1)));
        assert!(privilege.apply_once(&full_request()).is_err());
        assert_eq!(control.current().0, 150);
        assert_eq!(privilege.log().restores_on_parent_gone, 0);

        // Gone after the last step is a normal apply.
        let mut privilege = FakePrivilege::new(FakeGpuControl::new(), None);
        privilege.set_fault(Some(PrivilegeFault::ParentGone(1)));
        let one = HardwareRequest {
            power_limit_w: Some(200),
            ..HardwareRequest::default()
        };
        assert!(privilege.start(&one).is_ok());
    }

    #[test]
    fn no_agent_changes_nothing_and_says_why() {
        let control = FakeGpuControl::new();
        let mut privilege = FakePrivilege::new(control.clone(), None);
        privilege.set_fault(Some(PrivilegeFault::NoAgent));
        assert_eq!(
            privilege.apply_once(&HardwareRequest::gpu_reset()).err(),
            Some(PrivilegeError::Helper(NO_AGENT_MESSAGE.into()))
        );
        assert!(control.calls().is_empty());
    }

    #[test]
    fn unavailable_governor_fails_that_step_only() {
        let sys = sysfs();
        let mut privilege = FakePrivilege::new(FakeGpuControl::new(), Some(sys.path().into()));
        let report = privilege
            .apply_once(&HardwareRequest {
                cpu_governor: Some("schedutil".into()),
                smt: Some(false),
                ..HardwareRequest::default()
            })
            .unwrap();
        assert!(matches!(report.steps[0].outcome, StepOutcome::Failed(_)));
        assert!(matches!(
            report.steps[1].outcome,
            StepOutcome::Applied { .. }
        ));
        assert_eq!(governor(sys.path(), 0), "powersave");
    }
}
