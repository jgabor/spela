//! A scripted CPU: fixed governor and SMT answers, with a fault switch.

use std::sync::{Arc, Mutex, MutexGuard};

use crate::backend::cpu::{Cpu, CpuError};

/// A fault the fake CPU can show.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CpuFault {
    /// sysfs has no cpufreq or SMT files: every read fails.
    Unreadable,
}

#[derive(Debug)]
struct CpuState {
    governor: String,
    available: Vec<String>,
    smt_active: bool,
    fault: Option<CpuFault>,
}

/// CPU reads from fixed values. Clones share their state, so a test can
/// keep one and change what the other answers.
#[derive(Debug, Clone)]
pub struct FakeCpu {
    state: Arc<Mutex<CpuState>>,
}

impl FakeCpu {
    /// A CPU on `governor`, offering `available`, with SMT on or off.
    pub fn new(governor: &str, available: &[&str], smt_active: bool) -> FakeCpu {
        FakeCpu {
            state: Arc::new(Mutex::new(CpuState {
                governor: governor.to_owned(),
                available: available.iter().map(|g| (*g).to_owned()).collect(),
                smt_active,
                fault: None,
            })),
        }
    }

    /// The demo CPU: powersave, with performance and powersave offered, and
    /// SMT on.
    pub fn demo() -> FakeCpu {
        FakeCpu::new("powersave", &["performance", "powersave"], true)
    }

    /// Turns a fault on, or off with `None`.
    pub fn set_fault(&self, fault: Option<CpuFault>) {
        self.lock().fault = fault;
    }

    fn lock(&self) -> MutexGuard<'_, CpuState> {
        self.state.lock().unwrap_or_else(|e| e.into_inner())
    }

    fn check(state: &CpuState, file: &str) -> Result<(), CpuError> {
        match state.fault {
            Some(CpuFault::Unreadable) => Err(CpuError {
                path: format!("/sys/devices/system/cpu/{file}"),
                reason: "no such file or directory (fake)".into(),
            }),
            None => Ok(()),
        }
    }
}

impl Cpu for FakeCpu {
    fn governor(&mut self) -> Result<String, CpuError> {
        let state = self.lock();
        Self::check(&state, crate::backend::cpu::GOVERNOR_FILE)?;
        Ok(state.governor.clone())
    }

    fn available_governors(&mut self) -> Result<Vec<String>, CpuError> {
        let state = self.lock();
        Self::check(&state, crate::backend::cpu::AVAILABLE_GOVERNORS_FILE)?;
        Ok(state.available.clone())
    }

    fn smt_active(&mut self) -> Result<bool, CpuError> {
        let state = self.lock();
        Self::check(&state, crate::backend::cpu::SMT_ACTIVE_FILE)?;
        Ok(state.smt_active)
    }
}

/// The governors the demo sysfs tree offers, as acpi-cpufreq lists them.
pub const DEMO_GOVERNORS: &str = "conservative ondemand userspace powersave performance schedutil";

/// Writes a demo sysfs tree under `root`, the folder that stands for
/// `/sys`: four CPUs on `powersave` and SMT on. `spela --demo` reads it
/// with [`crate::backend::SysfsCpu`] and the fake privilege runner writes
/// it, so a governor or SMT change shows at the next metrics tick. Only
/// ever called on a temp dir.
pub fn write_sysfs(root: &std::path::Path) -> std::io::Result<()> {
    let cpu = root.join(crate::backend::cpu::CPU_DIR);
    for n in 0..4 {
        let dir = cpu.join(format!("cpu{n}/cpufreq"));
        std::fs::create_dir_all(&dir)?;
        std::fs::write(dir.join("scaling_governor"), "powersave\n")?;
        std::fs::write(
            dir.join("scaling_available_governors"),
            format!("{DEMO_GOVERNORS}\n"),
        )?;
    }
    std::fs::create_dir_all(cpu.join("smt"))?;
    std::fs::write(cpu.join("smt/control"), "on\n")?;
    std::fs::write(cpu.join("smt/active"), "1\n")?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_demo_tree_reads_back_and_follows_the_fake_runner() {
        use crate::backend::{HardwareRequest, Privilege, SysfsCpu};
        use crate::fake::{FakeGpuControl, FakePrivilege};
        let root = tempfile::tempdir().unwrap();
        write_sysfs(root.path()).unwrap();
        let mut cpu = SysfsCpu::new(root.path());
        assert_eq!(cpu.governor().unwrap(), "powersave");
        assert_eq!(cpu.available_governors().unwrap().len(), 6);
        assert!(cpu.smt_active().unwrap());
        let mut runner = FakePrivilege::new(FakeGpuControl::new(), Some(root.path().to_owned()));
        let report = runner
            .apply_once(&HardwareRequest {
                cpu_governor: Some("performance".into()),
                smt: Some(false),
                ..HardwareRequest::default()
            })
            .unwrap();
        assert!(report.all_applied(), "{report:?}");
        assert_eq!(cpu.governor().unwrap(), "performance");
        assert!(!cpu.smt_active().unwrap());
    }

    #[test]
    fn answers_and_faults() {
        let mut cpu = FakeCpu::demo();
        assert_eq!(cpu.governor().unwrap(), "powersave");
        assert_eq!(
            cpu.available_governors().unwrap(),
            ["performance", "powersave"]
        );
        assert!(cpu.smt_active().unwrap());
        let probe = cpu.clone();
        probe.set_fault(Some(CpuFault::Unreadable));
        assert!(cpu.governor().is_err());
        assert!(cpu.smt_active().is_err());
        probe.set_fault(None);
        assert!(cpu.available_governors().is_ok());
    }
}
