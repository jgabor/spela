//! The fake behind every backend trait.
//!
//! Tests and `spela --demo` share it (decision 18). It is plain code with no
//! test-only dependencies, because the release binary ships it for the
//! demo. Each fake keeps its state behind an `Arc`, so a test can keep a
//! clone, hand the other to the code under test and then check what was
//! called. Faults are switches: NVML missing, a dismissed prompt, a failure
//! at apply step N, an HTTP status or timeout, a display that never answers.
//!
//! Demo data is fictional (decision 43).

pub mod cpu;
pub mod display;
pub mod gpu;
pub mod http;
pub mod privilege;

use std::sync::Arc;

pub use cpu::{CpuFault, FakeCpu};
pub use display::{DisplayFault, FakeDisplay};
pub use gpu::{FakeGpu, FakeGpuControl, GpuCall, GpuFault};
pub use http::{FakeHttp, FakeRoute};
pub use privilege::{FakePrivilege, NO_AGENT_MESSAGE, PrivilegeFault, PrivilegeLog};

use crate::backend::{BackendKind, Backends};

/// A full set of fakes with demo data. The privilege fake has no sysfs
/// tree, so CPU steps are skipped; tests that need one build their own.
pub fn backends() -> Backends {
    Backends {
        kind: BackendKind::Fake,
        gpu: Box::new(FakeGpu::demo()),
        cpu: Box::new(FakeCpu::demo()),
        display: Box::new(FakeDisplay::demo()),
        privilege: Box::new(FakePrivilege::new(FakeGpuControl::new(), None)),
        http: Arc::new(FakeHttp::new()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn backends_are_the_fake() {
        let mut b = backends();
        assert_eq!(b.kind, BackendKind::Fake);
        assert!(b.gpu.info().is_ok());
        assert!(!b.display.outputs().unwrap().is_empty());
    }
}
