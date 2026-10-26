//! A privilege runner that needs no root.
//!
//! It runs the root helper's real loops ([`crate::helper::serve`]) in
//! process, on a thread, over pipe pairs: the `--serve` loop for
//! [`Privilege::start`] and the `--once` loop for
//! [`Privilege::apply_once`]. The parent's side is the real client
//! ([`crate::helper::client`]), so these tests check the helper the wrapper
//! starts and the code that talks to it. GPU steps go to a
//! [`FakeGpuControl`], and CPU steps write a sysfs tree under a temp dir
//! that the test passes in.
//!
//! The faults stand in for what only polkit or a dying parent can do: a
//! dismissed prompt, no polkit, no agent, a failure at step N, and a parent
//! that goes away at step N.

use std::io::{self, BufReader, Write};
use std::path::PathBuf;
use std::sync::mpsc::{self, Sender};
use std::sync::{Arc, Mutex, MutexGuard};

use crate::backend::cpu::{CPU_DIR, SMT_ACTIVE_FILE};
use crate::backend::gpu::GpuControl;
use crate::backend::privilege::{
    ApplyReport, HardwareRequest, Privilege, PrivilegeError, PrivilegedSession, RestoreReport,
    SettingValue, Step,
};
use crate::helper::client::{self, Finish, HelperExit, HelperSession, StartError};
use crate::helper::machine::{Machine, Saved, SystemMachine};
use crate::helper::protocol;
use crate::helper::serve::{self, Event};

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
    /// The wrapper goes away at step number `n`, counting from 0. A
    /// `--serve` helper sees the end of its input before it changes step
    /// `n`, restores the steps it applied and exits, so the caller gets an
    /// error and no session. A `--once` helper sees its output close when
    /// it reports step `n`: that step and the ones before it stay, as a
    /// live change does, and the caller gets an error.
    ParentGone(usize),
    /// No polkit agent can ask for the password. Nothing changes, and the
    /// error carries the agent's own text.
    NoAgent,
}

/// What the polkit agent prints when no agent can ask (unverified: the
/// user's live check settles how it reports it).
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

/// Runs the helper's loops in process.
#[derive(Debug, Clone)]
pub struct FakePrivilege {
    control: FakeGpuControl,
    sys_root: Option<PathBuf>,
    fault: Option<PrivilegeFault>,
    log: Arc<Mutex<PrivilegeLog>>,
}

impl FakePrivilege {
    /// A runner over `control` and, for CPU steps, a sysfs tree at
    /// `sys_root` (the folder that stands for `/sys`). Without one, CPU
    /// steps are skipped.
    pub fn new(control: FakeGpuControl, sys_root: Option<PathBuf>) -> Self {
        FakePrivilege {
            control,
            sys_root,
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

    /// The machine the helper thread changes, with this runner's faults.
    fn machine(&self, events: Option<Sender<Event>>) -> FakeMachine {
        let control = self.control.clone();
        FakeMachine {
            inner: SystemMachine::new(
                Box::new(move || Ok(Box::new(control.clone()) as Box<dyn GpuControl>)),
                self.sys_root.clone(),
            ),
            sys_root: self.sys_root.clone(),
            fail_at: match self.fault {
                Some(PrivilegeFault::FailAtStep(n)) => Some(n),
                _ => None,
            },
            gone_at: events.and_then(|tx| match self.fault {
                Some(PrivilegeFault::ParentGone(n)) => Some((n, tx)),
                _ => None,
            }),
            next: 0,
            current: 0,
        }
    }

    fn gone_at(&self) -> Option<usize> {
        match self.fault {
            Some(PrivilegeFault::ParentGone(n)) => Some(n),
            _ => None,
        }
    }
}

impl Privilege for FakePrivilege {
    fn start(
        &mut self,
        request: &HardwareRequest,
    ) -> Result<Box<dyn PrivilegedSession>, PrivilegeError> {
        if request.is_empty() {
            return Ok(client::empty_session());
        }
        self.prompt()?;
        let (to_helper_r, to_helper_w) = io::pipe().map_err(pipe_error)?;
        let (from_helper_r, from_helper_w) = io::pipe().map_err(pipe_error)?;
        let (tx, rx) = mpsc::channel();
        let mut machine = self.machine(Some(tx.clone()));
        serve::spawn_reader(BufReader::new(to_helper_r), tx).map_err(pipe_error)?;
        let helper = std::thread::Builder::new()
            .name("spela-fake-helper".into())
            .spawn(move || {
                let mut out = from_helper_w;
                serve::serve(&rx, &mut out, &mut machine)
            })
            .map_err(pipe_error)?;
        match client::start(
            request,
            Box::new(to_helper_w),
            Box::new(BufReader::new(from_helper_r)),
            joined(helper),
        ) {
            Ok(session) => Ok(Box::new(FakeSession {
                inner: Some(session),
                log: Arc::clone(&self.log),
            })),
            Err(e) => {
                if matches!(
                    e,
                    StartError::Stopped {
                        restored: Some(_),
                        ..
                    }
                ) {
                    lock(&self.log).restores_on_parent_gone += 1;
                }
                Err(e.into_privilege_error())
            }
        }
    }

    fn apply_once(&mut self, request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError> {
        if request.is_empty() {
            return Ok(ApplyReport::default());
        }
        self.prompt()?;
        let line = protocol::encode_request(&request.steps());
        let (from_helper_r, from_helper_w) = io::pipe().map_err(pipe_error)?;
        let mut machine = self.machine(None);
        let gone_at = self.gone_at();
        let helper = std::thread::Builder::new()
            .name("spela-fake-helper".into())
            .spawn(move || {
                let mut out = ClosesAfter {
                    inner: from_helper_w,
                    lines_left: gone_at,
                };
                serve::once(&line, &mut out, &mut machine)
            })
            .map_err(pipe_error)?;
        client::read_once(request, &mut BufReader::new(from_helper_r), joined(helper))
            .map_err(StartError::into_privilege_error)
    }
}

/// Waits for the helper thread and passes on its exit code.
fn joined(helper: std::thread::JoinHandle<u8>) -> Finish {
    Box::new(move || HelperExit {
        code: Some(helper.join().map_or(101, i32::from)),
        stderr: String::new(),
    })
}

fn pipe_error(e: io::Error) -> PrivilegeError {
    PrivilegeError::Helper(format!("cannot start the fake helper: {e}"))
}

/// The helper's machine with the fake's faults, and the one thing the
/// kernel does that a temp tree cannot: `smt/active` follows
/// `smt/control`.
struct FakeMachine {
    inner: SystemMachine,
    sys_root: Option<PathBuf>,
    fail_at: Option<usize>,
    gone_at: Option<(usize, Sender<Event>)>,
    /// The index the next step gets.
    next: usize,
    /// The step being applied.
    current: usize,
}

impl FakeMachine {
    fn follow_smt(&self, on: bool) {
        if let Some(root) = &self.sys_root {
            let active = root.join(CPU_DIR).join(SMT_ACTIVE_FILE);
            if active.exists() {
                let _ = std::fs::write(active, if on { "1" } else { "0" });
            }
        }
    }
}

impl Machine for FakeMachine {
    fn check(&mut self, step: Step, target: &SettingValue) -> Result<(), String> {
        self.current = self.next;
        self.next += 1;
        if let Some((n, tx)) = &self.gone_at
            && *n == self.current
        {
            // The helper reads its input before it changes this step.
            let _ = tx.send(Event::Eof);
        }
        self.inner.check(step, target)
    }

    fn read_prior(&mut self, step: Step) -> Result<Saved, String> {
        self.inner.read_prior(step)
    }

    fn apply(&mut self, step: Step, target: &SettingValue) -> Result<(), String> {
        if self.fail_at == Some(self.current) {
            return Err(format!("fake failure at step {}", self.current));
        }
        self.inner.apply(step, target)?;
        if let SettingValue::Smt(on) = target {
            self.follow_smt(*on);
        }
        Ok(())
    }

    fn restore(&mut self, step: Step, saved: &Saved) -> Result<(), String> {
        self.inner.restore(step, saved)?;
        if let Saved::Value(SettingValue::Smt(on)) = saved {
            self.follow_smt(*on);
        }
        Ok(())
    }
}

/// The `--once` helper's stdout: it closes, like a pipe whose reader went
/// away, once `lines_left` lines are through.
struct ClosesAfter<W: Write> {
    inner: W,
    lines_left: Option<usize>,
}

impl<W: Write> Write for ClosesAfter<W> {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        if self.lines_left == Some(0) {
            return Err(io::Error::from(io::ErrorKind::BrokenPipe));
        }
        let n = self.inner.write(buf)?;
        if buf[..n].contains(&b'\n')
            && let Some(left) = &mut self.lines_left
        {
            *left -= 1;
        }
        Ok(n)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.inner.flush()
    }
}

/// A session over the fake helper thread, counting restores for the log.
struct FakeSession {
    inner: Option<HelperSession>,
    log: Arc<Mutex<PrivilegeLog>>,
}

impl PrivilegedSession for FakeSession {
    fn report(&self) -> &ApplyReport {
        self.inner
            .as_ref()
            .map(HelperSession::report)
            .expect("a session is restored only once")
    }

    fn restore(mut self: Box<Self>) -> Result<RestoreReport, PrivilegeError> {
        let session = self.inner.take().expect("a session is restored only once");
        lock(&self.log).restores += 1;
        session.finish_restore()
    }
}

impl Drop for FakeSession {
    fn drop(&mut self) {
        if let Some(session) = self.inner.take() {
            lock(&self.log).restores_on_drop += 1;
            drop(session);
        }
    }
}

fn lock(log: &Mutex<PrivilegeLog>) -> MutexGuard<'_, PrivilegeLog> {
    log.lock().unwrap_or_else(|e| e.into_inner())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    use crate::backend::{FanSetting, StepOutcome};
    use crate::fake::GpuCall;

    fn read_trimmed(path: &Path) -> Result<String, String> {
        std::fs::read_to_string(path)
            .map(|s| s.trim().to_owned())
            .map_err(|e| e.to_string())
    }

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
    fn unavailable_governor_skips_that_step_only() {
        let sys = sysfs();
        let mut privilege = FakePrivilege::new(FakeGpuControl::new(), Some(sys.path().into()));
        let report = privilege
            .apply_once(&HardwareRequest {
                cpu_governor: Some("schedutil".into()),
                smt: Some(false),
                ..HardwareRequest::default()
            })
            .unwrap();
        // A governor sysfs does not list fails its check: skipped, with the
        // reason.
        assert!(
            matches!(&report.steps[0].outcome, StepOutcome::Skipped(why) if why.contains("not available"))
        );
        assert!(matches!(
            report.steps[1].outcome,
            StepOutcome::Applied { .. }
        ));
        assert_eq!(governor(sys.path(), 0), "powersave");
    }
}
