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

#[cfg(test)]
mod tests {
    use super::*;

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
