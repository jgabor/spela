//! Monitor's commands and the hardware jobs, run off the UI thread.
//!
//! A hardware job (decision 44) runs on the job pool: it takes the launch
//! lock without waiting, as the live CLI commands do, then hands one
//! request to the privilege backend's one-shot apply, and answers with
//! `Msg::HardwareDone`. A held lock fails the job with the lock's own
//! `spela is already running (PID: N)` and changes nothing. The job never
//! runs on the UI thread.
//!
//! The backend is the one the run selected. Under `--demo` and
//! `SPELA_BACKEND=fake` it is the in-process fake, so nothing on this
//! machine changes. Otherwise it is the polkit runner built for the TUI:
//! `pkexec --disable-internal-agent` with a null stdin, so pkexec never
//! prompts on the TUI's tty, and stdout and stderr on pipes, so nothing is
//! written over the frame (decision 44).

use std::path::Path;
use std::sync::{Arc, Mutex, PoisonError};

use spela_core::backend::{
    ApplyReport, Cpu, HardwareRequest, Privilege, PrivilegeError, PrivilegedSession, SysfsCpu,
};
use spela_core::fake::{FakeGpuControl, FakePrivilege};
use spela_core::paths::Paths;
use spela_ui::Msg;
use spela_ui::event::HardwareOp;
use spela_ui::screens::monitor::Cmd;

use super::Workers;

/// The privilege backend, shared with the job pool. One hardware job runs
/// at a time (the UI refuses a second), so the lock is never contended.
#[derive(Clone)]
pub struct SharedPrivilege(Arc<Mutex<Box<dyn Privilege>>>);

impl SharedPrivilege {
    /// Shares `backend`.
    pub fn new(backend: Box<dyn Privilege>) -> SharedPrivilege {
        SharedPrivilege(Arc::new(Mutex::new(backend)))
    }

    fn apply_once(&self, request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError> {
        self.0
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .apply_once(request)
    }
}

impl std::fmt::Debug for SharedPrivilege {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("SharedPrivilege")
    }
}

/// A privilege backend that is not there, left behind when the TUI's job
/// pool takes the real one.
#[derive(Debug)]
pub struct NoPrivilege;

impl Privilege for NoPrivilege {
    fn start(
        &mut self,
        _request: &HardwareRequest,
    ) -> Result<Box<dyn PrivilegedSession>, PrivilegeError> {
        Err(PrivilegeError::Unavailable(
            "the TUI's job pool owns the privilege backend".into(),
        ))
    }

    fn apply_once(&mut self, _request: &HardwareRequest) -> Result<ApplyReport, PrivilegeError> {
        Err(PrivilegeError::Unavailable(
            "the TUI's job pool owns the privilege backend".into(),
        ))
    }
}

/// The request a hardware command sends, the same as its CLI command's:
/// `gpu reset`, `cpu governor <g>` or `cpu smt <on|off>`.
pub fn request(op: &HardwareOp) -> HardwareRequest {
    match op {
        HardwareOp::GpuReset => HardwareRequest::gpu_reset(),
        HardwareOp::Governor(g) => HardwareRequest {
            cpu_governor: Some(g.clone()),
            ..HardwareRequest::default()
        },
        HardwareOp::Smt(on) => HardwareRequest {
            smt: Some(*on),
            ..HardwareRequest::default()
        },
    }
}

/// Runs a hardware command on the pool.
pub fn hardware(w: &mut Workers, token: u64, op: HardwareOp) {
    let paths = w.paths.clone();
    let privilege = w.privilege.clone();
    w.jobs.submit(
        &w.tx,
        move || Msg::HardwareDone {
            token,
            result: apply(&paths, &privilege, &op),
        },
        move |e| Msg::HardwareDone {
            token,
            result: Err(PrivilegeError::Helper(e)),
        },
    );
}

/// One hardware command: the launch lock without waiting, then one
/// one-shot apply. A held lock answers with the lock's own message and
/// asks the backend for nothing. Blocking: it runs on the job pool.
pub fn apply(
    paths: &Paths,
    privilege: &SharedPrivilege,
    op: &HardwareOp,
) -> Result<ApplyReport, PrivilegeError> {
    let _lock =
        crate::locks::launch(paths).map_err(|e| PrivilegeError::Unavailable(e.to_string()))?;
    let report = privilege.apply_once(&request(op));
    tracing::info!(?op, ok = report.is_ok(), "hardware command");
    if let Ok(r) = &report {
        for s in &r.steps {
            tracing::info!(step = %s.step, target = %s.target, outcome = ?s.outcome, "hardware step");
        }
    }
    report
}

/// The CPU reader and privilege runner for `spela --demo`: a fictional
/// sysfs tree under `root`, read by the real sysfs reader and written by
/// the fake runner, so a governor or SMT change shows at the next tick.
/// `root` is inside the demo's private temp dir.
pub fn demo_hardware(root: &Path) -> std::io::Result<(Box<dyn Cpu>, Box<dyn Privilege>)> {
    spela_core::fake::cpu::write_sysfs(root)?;
    Ok((
        Box::new(SysfsCpu::new(root)),
        Box::new(FakePrivilege::new(
            FakeGpuControl::new(),
            Some(root.to_owned()),
        )),
    ))
}

/// Runs one of the screen's commands. It has none of its own: hardware
/// jobs come as the shell's `Cmd::Hardware`.
pub fn run(_w: &mut Workers, cmd: Cmd) {
    match cmd {}
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn each_command_sends_its_cli_request() {
        assert_eq!(request(&HardwareOp::GpuReset), HardwareRequest::gpu_reset());
        assert_eq!(
            request(&HardwareOp::Governor("performance".into())).cpu_governor,
            Some("performance".into())
        );
        let smt = request(&HardwareOp::Smt(false));
        assert_eq!(smt.smt, Some(false));
        assert_eq!(smt.steps().len(), 1);
        assert!(NoPrivilege.apply_once(&smt).is_err());
    }

    #[test]
    fn a_held_launch_lock_changes_nothing() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let fake = FakePrivilege::new(FakeGpuControl::new(), None);
        let shared = SharedPrivilege::new(Box::new(fake.clone()));
        let held = crate::locks::launch(&paths).unwrap();
        let err = apply(&paths, &shared, &HardwareOp::GpuReset).unwrap_err();
        let PrivilegeError::Unavailable(text) = &err else {
            panic!("{err:?}")
        };
        assert!(
            text.starts_with("spela is already running (PID: "),
            "{text}"
        );
        assert_eq!(fake.log().prompts, 0, "nothing was asked");
        drop(held);
        let report = apply(&paths, &shared, &HardwareOp::GpuReset).unwrap();
        assert!(report.all_applied(), "{report:?}");
        assert_eq!(fake.log().prompts, 1);
        // The job let the lock go.
        assert!(crate::locks::launch(&paths).is_ok());
    }

    #[test]
    fn the_demo_hardware_changes_only_its_temp_tree() {
        let dir = tempfile::tempdir().unwrap();
        let (mut cpu, mut privilege) = demo_hardware(dir.path()).unwrap();
        assert_eq!(cpu.governor().unwrap(), "powersave");
        let report = privilege
            .apply_once(&request(&HardwareOp::Governor("performance".into())))
            .unwrap();
        assert!(report.all_applied());
        assert_eq!(cpu.governor().unwrap(), "performance");
        let report = privilege
            .apply_once(&request(&HardwareOp::Smt(false)))
            .unwrap();
        assert!(report.all_applied());
        assert!(!cpu.smt_active().unwrap());
    }
}
