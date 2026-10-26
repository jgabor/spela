//! The seams between spela and the machine.
//!
//! Every backend that touches hardware, privilege, the display or the
//! network sits behind a trait with two implementations: the real one in the
//! `spela` binary and the fake in [`crate::fake`]. Tests and `spela --demo`
//! share the fake (decision 18). Backends that are only files have no trait:
//! their seam is a root path that tests point at a temp dir. CPU state is
//! the exception: its reads are files under a sysfs root, and they also sit
//! behind [`Cpu`], so the fake answers for `--demo` and the binary tests.
//!
//! [`BackendKind::from_env`] reads the hidden `SPELA_BACKEND` selector.
//! `fake` swaps these backends for the fake and changes nothing else, so the
//! [`crate::paths::Paths`] a test injected stay.

pub mod cpu;
pub mod display;
pub mod gpu;
pub mod http;
pub mod privilege;

use std::sync::Arc;

pub use cpu::{Cpu, CpuError, SysfsCpu};
pub use display::{Display, DisplayError, Output, VrrPolicy};
pub use gpu::{
    FanSetting, Gpu, GpuControl, GpuError, GpuInfo, GpuMetrics, GpuPrior, Prior, ThrottleReasons,
};
pub use http::{Http, HttpError, HttpRequest, HttpResponse, USER_AGENT};
pub use privilege::{
    ApplyReport, HardwareRequest, Privilege, PrivilegeError, PrivilegedSession, RestorePlan,
    RestoreReport, RestoreStep, SettingValue, Step, StepOutcome, StepReport,
};

use crate::env::{Env, var};

/// Which implementations a process uses.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BackendKind {
    /// NVML, `kscreen-doctor`, the polkit helper and ureq.
    Real,
    /// [`crate::fake`], for tests and `--demo`.
    Fake,
}

impl BackendKind {
    /// Reads `SPELA_BACKEND`: unset or empty means real, `fake` means the
    /// fake. Anything else is refused, so a typo in a test sandbox never
    /// falls back to real hardware.
    pub fn from_env(env: &Env) -> Result<BackendKind, String> {
        match env.non_empty(var::SPELA_BACKEND) {
            None | Some("real") => Ok(BackendKind::Real),
            Some("fake") => Ok(BackendKind::Fake),
            Some(other) => Err(format!(
                "unknown {} value {other:?}; use \"fake\" or leave it unset",
                var::SPELA_BACKEND
            )),
        }
    }
}

/// One process's backends.
pub struct Backends {
    /// Which set this is.
    pub kind: BackendKind,
    /// GPU 0 reads.
    pub gpu: Box<dyn Gpu>,
    /// CPU governor and SMT reads.
    pub cpu: Box<dyn Cpu>,
    /// KDE VRR.
    pub display: Box<dyn Display>,
    /// The one privileged round trip.
    pub privilege: Box<dyn Privilege>,
    /// Blocking HTTP, shared with workers.
    pub http: Arc<dyn Http>,
}

impl std::fmt::Debug for Backends {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Backends")
            .field("kind", &self.kind)
            .finish()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn selector_defaults_to_real_and_refuses_typos() {
        assert_eq!(
            BackendKind::from_env(&Env::default()),
            Ok(BackendKind::Real)
        );
        let with = |v: &str| BackendKind::from_env(&Env::default().with(var::SPELA_BACKEND, v));
        assert_eq!(with(""), Ok(BackendKind::Real));
        assert_eq!(with("real"), Ok(BackendKind::Real));
        assert_eq!(with("fake"), Ok(BackendKind::Fake));
        assert_eq!(with(" fake "), Ok(BackendKind::Fake));
        let err = with("fkae").unwrap_err();
        assert!(
            err.contains("SPELA_BACKEND") && err.contains("fkae"),
            "{err}"
        );
    }
}
