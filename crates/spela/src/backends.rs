//! Picks the backends for a run: the fake for `--demo` and for
//! `SPELA_BACKEND=fake`, otherwise the real ones.
//!
//! Built: the GPU reads on NVML ([`NvmlGpu`]), the CPU reads over `/sys`
//! and `/proc` with the SCX status ([`SystemCpu`]), the KDE display through
//! `kscreen-doctor` ([`KscreenDisplay`]), HTTP through ureq with the proxy
//! from the environment snapshot ([`UreqHttp`]), and the polkit helper
//! ([`PolkitPrivilege`]). Nothing opens at startup, so a command that never
//! asks pays nothing; NVML in particular stays lazy (it costs 4 to 17 ms per
//! process).

use std::path::Path;
use std::process::{Command, Stdio};
use std::sync::Arc;
use std::time::{Duration, Instant};

use spela_core::backend::{BackendKind, Backends, Cpu, CpuError, CpuInfo, SysfsCpu};
use spela_core::env::{Env, var};

use crate::display::KscreenDisplay;
use crate::error::{CliError, exit};
use crate::http::UreqHttp;
use crate::nvml::NvmlGpu;
use crate::privilege::PolkitPrivilege;

/// The backends for this run. `--demo` always gets the fake.
pub fn select(env: &Env, demo: bool) -> Result<Backends, CliError> {
    Ok(match kind(env, demo)? {
        BackendKind::Fake => spela_core::fake::backends(),
        BackendKind::Real => real(env),
    })
}

/// Which backends a run gets: the fake for `--demo` and
/// `SPELA_BACKEND=fake`, an error for an unknown `SPELA_BACKEND`, the real
/// ones otherwise.
pub fn kind(env: &Env, demo: bool) -> Result<BackendKind, CliError> {
    if demo {
        return Ok(BackendKind::Fake);
    }
    BackendKind::from_env(env).map_err(|m| CliError::new(exit::FAILURE, m))
}

/// The real backends.
///
/// A unit test that builds them panics here, so no in-process test reaches
/// NVML, `kscreen-doctor`, polkit or the network. Binary tests run with
/// `SPELA_BACKEND=fake`, and `tests/safety.rs` checks every command logs
/// `backend=Fake`.
pub fn real(env: &Env) -> Backends {
    if cfg!(test) {
        panic!("a unit test built the real backends; use the fake");
    }
    Backends {
        kind: BackendKind::Real,
        gpu: Box::new(NvmlGpu::lazy()),
        cpu: Box::new(SystemCpu::new(
            SysfsCpu::system(),
            env.non_empty(var::PATH).map(str::to_owned),
        )),
        display: Box::new(KscreenDisplay::system()),
        privilege: Box::new(PolkitPrivilege::cli()),
        http: Arc::new(UreqHttp::from_env(env)),
    }
}

/// The SCX loader, looked up on PATH.
const SCX_LOADER: &str = "scx_loader";
/// systemd's control program, asked whether `scx.service` runs.
const SYSTEMCTL: &str = "systemctl";
/// How long `systemctl is-active` may take before SCX counts as inactive.
const SCX_TIMEOUT: Duration = Duration::from_secs(2);

/// The real CPU reader: [`SysfsCpu`] plus the SCX line of `cpu info`, which
/// needs a PATH lookup and a process.
#[derive(Debug)]
pub struct SystemCpu {
    sysfs: SysfsCpu,
    path_var: Option<String>,
}

impl SystemCpu {
    /// Reads `sysfs`, and looks for `scx_loader` on `path_var`, the
    /// environment snapshot's `PATH`.
    pub fn new(sysfs: SysfsCpu, path_var: Option<String>) -> SystemCpu {
        SystemCpu { sysfs, path_var }
    }
}

impl Cpu for SystemCpu {
    fn governor(&mut self) -> Result<String, CpuError> {
        self.sysfs.governor()
    }

    fn available_governors(&mut self) -> Result<Vec<String>, CpuError> {
        self.sysfs.available_governors()
    }

    fn smt_active(&mut self) -> Result<bool, CpuError> {
        self.sysfs.smt_active()
    }

    fn info(&mut self) -> Result<CpuInfo, CpuError> {
        let mut info = self.sysfs.info()?;
        info.scx_active = scx_status(self.path_var.as_deref(), &systemctl_is_active);
        Ok(info)
    }
}

/// The SCX line, as Go read it: `None` (no line) when `scx_loader` is not
/// on PATH, otherwise whether `systemctl is-active scx.service` succeeds.
/// `is_active` runs that check, so a test passes a fake.
pub fn scx_status(path_var: Option<&str>, is_active: &dyn Fn() -> bool) -> Option<bool> {
    let installed = path_var
        .into_iter()
        .flat_map(|p| p.split(':'))
        .filter(|dir| !dir.is_empty())
        .any(|dir| is_executable(&Path::new(dir).join(SCX_LOADER)));
    installed.then(is_active)
}

fn is_executable(path: &Path) -> bool {
    use std::os::unix::fs::PermissionsExt;
    std::fs::metadata(path).is_ok_and(|m| m.is_file() && m.permissions().mode() & 0o111 != 0)
}

/// `systemctl is-active scx.service`, with a timeout Go did not have.
fn systemctl_is_active() -> bool {
    let child = Command::new(SYSTEMCTL)
        .args(["is-active", "scx.service"])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn();
    let Ok(mut child) = child else {
        return false;
    };
    let started = Instant::now();
    loop {
        match child.try_wait() {
            Ok(Some(status)) => return status.success(),
            Ok(None) if started.elapsed() < SCX_TIMEOUT => {
                std::thread::sleep(Duration::from_millis(10));
            }
            _ => {
                let _ = child.kill();
                let _ = child.wait();
                return false;
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::env::var;

    #[test]
    fn demo_and_fake_select_the_fake() {
        let fake = Env::default().with(var::SPELA_BACKEND, "fake");
        assert_eq!(select(&fake, false).unwrap().kind, BackendKind::Fake);
        assert_eq!(
            select(&Env::default(), true).unwrap().kind,
            BackendKind::Fake
        );
        // --demo wins even over a typo, because it never needs the real ones.
        let typo = Env::default().with(var::SPELA_BACKEND, "fkae");
        assert_eq!(select(&typo, true).unwrap().kind, BackendKind::Fake);
        assert_eq!(select(&typo, false).unwrap_err().code, exit::FAILURE);
    }

    #[test]
    fn nothing_set_means_real_but_a_test_never_builds_them() {
        assert_eq!(kind(&Env::default(), false).unwrap(), BackendKind::Real);
        let built = std::panic::catch_unwind(|| real(&Env::default()));
        assert!(built.is_err(), "real() refuses to run in a unit test");
    }

    #[test]
    fn scx_shows_only_when_its_loader_is_installed() {
        use std::os::unix::fs::PermissionsExt;
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().display().to_string();
        let asked = std::cell::Cell::new(0);
        let active = || {
            asked.set(asked.get() + 1);
            true
        };
        assert_eq!(scx_status(Some(&path), &active), None);
        assert_eq!(scx_status(None, &active), None);
        assert_eq!(
            asked.get(),
            0,
            "systemctl is never asked without the loader"
        );
        let loader = dir.path().join(SCX_LOADER);
        std::fs::write(&loader, "").unwrap();
        assert_eq!(scx_status(Some(&path), &active), None, "not executable");
        std::fs::set_permissions(&loader, std::fs::Permissions::from_mode(0o755)).unwrap();
        assert_eq!(scx_status(Some(&format!(":{path}")), &active), Some(true));
        assert_eq!(scx_status(Some(&path), &|| false), Some(false));
    }

    #[test]
    fn the_system_cpu_reads_its_roots_and_adds_no_scx_without_the_loader() {
        let dir = tempfile::tempdir().unwrap();
        let roots = spela_core::backend::SystemRoots::under(dir.path());
        spela_core::fake::cpu::write_sysfs(&roots.sys).unwrap();
        std::fs::create_dir_all(&roots.proc).unwrap();
        std::fs::write(roots.proc.join("cpuinfo"), "model name\t: Test CPU\n").unwrap();
        let mut cpu = SystemCpu::new(SysfsCpu::with_roots(&roots), None);
        let info = cpu.info().unwrap();
        assert_eq!(info.model.as_deref(), Some("Test CPU"));
        assert_eq!(info.cores, 4);
        assert_eq!(info.scx_active, None);
        assert_eq!(cpu.governor().unwrap(), "powersave");
    }
}
