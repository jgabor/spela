//! Picks the backends for a run: the fake for `--demo` and for
//! `SPELA_BACKEND=fake`, otherwise the real ones.
//!
//! The real GPU, display and HTTP backends arrive in later phases (NVML in
//! Phase 2, `kscreen-doctor` and polkit in Phase 6, ureq with the manifest
//! in Phase 5). Until then they report that they are not built. They open
//! nothing when built, so a command that never asks pays nothing; NVML in
//! particular stays lazy (it costs 4 to 17 ms per process).

use std::sync::Arc;

use spela_core::backend::{
    BackendKind, Backends, Display, DisplayError, Gpu, GpuError, GpuInfo, GpuMetrics, Http,
    HttpError, HttpRequest, HttpResponse, Output, VrrPolicy,
};
use spela_core::env::Env;

use crate::error::{CliError, exit};
use crate::privilege::PolkitPrivilege;

/// The backends for this run. `--demo` always gets the fake.
pub fn select(env: &Env, demo: bool) -> Result<Backends, CliError> {
    Ok(match kind(env, demo)? {
        BackendKind::Fake => spela_core::fake::backends(),
        BackendKind::Real => real(),
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
pub fn real() -> Backends {
    if cfg!(test) {
        panic!("a unit test built the real backends; use the fake");
    }
    Backends {
        kind: BackendKind::Real,
        gpu: Box::new(NvmlGpu),
        display: Box::new(KscreenDisplay),
        privilege: Box::new(PolkitPrivilege),
        http: Arc::new(UreqHttp),
    }
}

/// GPU 0 through NVML. Not built yet.
#[derive(Debug)]
pub struct NvmlGpu;

impl NvmlGpu {
    fn not_built() -> GpuError {
        GpuError::Unavailable("NVML support is not yet implemented in this build".into())
    }
}

impl Gpu for NvmlGpu {
    fn info(&mut self) -> Result<GpuInfo, GpuError> {
        Err(Self::not_built())
    }

    fn metrics(&mut self) -> Result<GpuMetrics, GpuError> {
        Err(Self::not_built())
    }
}

/// KDE VRR through `kscreen-doctor`. Not built yet.
#[derive(Debug)]
pub struct KscreenDisplay;

impl Display for KscreenDisplay {
    fn outputs(&mut self) -> Result<Vec<Output>, DisplayError> {
        Err(DisplayError::Unavailable(
            "KDE VRR is not yet implemented in this build".into(),
        ))
    }

    fn set_vrr(&mut self, _uuid: &str, _policy: VrrPolicy) -> Result<(), DisplayError> {
        self.outputs().map(|_| ())
    }
}

/// Blocking HTTP through ureq. Not built yet, so nothing leaves the machine.
#[derive(Debug)]
pub struct UreqHttp;

impl Http for UreqHttp {
    fn get(
        &self,
        _request: &HttpRequest,
        _body: &mut dyn std::io::Write,
        _progress: &mut dyn FnMut(u64, Option<u64>),
    ) -> Result<HttpResponse, HttpError> {
        Err(HttpError::Network(
            "HTTP is not yet implemented in this build".into(),
        ))
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
        let built = std::panic::catch_unwind(real);
        assert!(built.is_err(), "real() refuses to run in a unit test");
    }

    /// The stubs' read paths only. Remove each check when its backend is
    /// built: a real `NvmlGpu` would open NVML here. `set_vrr` and
    /// `UreqHttp::get` are never called from a test.
    #[test]
    fn unbuilt_stubs_report_unavailable() {
        assert!(matches!(NvmlGpu.info(), Err(GpuError::Unavailable(_))));
        assert!(matches!(NvmlGpu.metrics(), Err(GpuError::Unavailable(_))));
        assert!(matches!(
            KscreenDisplay.outputs(),
            Err(DisplayError::Unavailable(_))
        ));
    }
}
