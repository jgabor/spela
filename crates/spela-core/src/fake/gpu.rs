//! A scripted GPU and a recording GPU control.

use std::sync::{Arc, Mutex, MutexGuard};

use crate::backend::gpu::{
    FanSetting, Gpu, GpuControl, GpuError, GpuInfo, GpuMetrics, GpuPrior, Prior,
};

/// A fault the fake GPU can show.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GpuFault {
    /// NVML is missing: every call is [`GpuError::Unavailable`].
    Missing,
    /// Every call is [`GpuError::NotSupported`].
    NotSupported,
}

impl GpuFault {
    fn error(self) -> GpuError {
        match self {
            GpuFault::Missing => GpuError::Unavailable("libnvidia-ml.so.1 not found (fake)".into()),
            GpuFault::NotSupported => GpuError::NotSupported("fake".into()),
        }
    }
}

#[derive(Debug)]
struct GpuState {
    info: GpuInfo,
    script: Vec<GpuMetrics>,
    next: usize,
    polls: usize,
    fault: Option<GpuFault>,
}

/// GPU reads from a script. Each metrics call returns the next sample and
/// wraps at the end.
#[derive(Debug, Clone)]
pub struct FakeGpu {
    state: Arc<Mutex<GpuState>>,
}

impl FakeGpu {
    /// A GPU that returns `script` in turn. An empty script reads as
    /// unsupported.
    pub fn new(info: GpuInfo, script: Vec<GpuMetrics>) -> Self {
        FakeGpu {
            state: Arc::new(Mutex::new(GpuState {
                info,
                script,
                next: 0,
                polls: 0,
                fault: None,
            })),
        }
    }

    /// The demo GPU. Its samples walk from idle through warm and hot to
    /// critical and back, so the alert states show up in turn.
    pub fn demo() -> Self {
        let sample = |temperature_c, utilization_percent, power_draw_w, fan| GpuMetrics {
            temperature_c,
            utilization_percent,
            memory_used_mib: 2_048 + u64::from(utilization_percent) * 100,
            memory_total_mib: 16_376,
            power_draw_w,
            graphics_clock_mhz: 210 + utilization_percent * 25,
            memory_clock_mhz: 10_501,
            fan_percent: Some(fan),
        };
        FakeGpu::new(
            GpuInfo {
                name: "Demo GPU 4080".into(),
                driver_version: "580.95.05".into(),
                memory_total_mib: 16_376,
            },
            vec![
                sample(41, 3, 22, 0),
                sample(64, 71, 248, 42),
                sample(79, 97, 312, 68),
                sample(86, 99, 318, 81),
                sample(93, 100, 320, 100),
                sample(72, 64, 230, 55),
            ],
        )
    }

    /// Turns a fault on, or off with `None`.
    pub fn set_fault(&self, fault: Option<GpuFault>) {
        self.lock().fault = fault;
    }

    /// How many metrics calls were made, faulted ones included.
    pub fn polls(&self) -> usize {
        self.lock().polls
    }

    fn lock(&self) -> MutexGuard<'_, GpuState> {
        self.state.lock().unwrap_or_else(|e| e.into_inner())
    }
}

impl Gpu for FakeGpu {
    fn info(&mut self) -> Result<GpuInfo, GpuError> {
        let state = self.lock();
        match state.fault {
            Some(f) => Err(f.error()),
            None => Ok(state.info.clone()),
        }
    }

    fn metrics(&mut self) -> Result<GpuMetrics, GpuError> {
        let mut state = self.lock();
        state.polls += 1;
        if let Some(f) = state.fault {
            return Err(f.error());
        }
        if state.script.is_empty() {
            return Err(GpuError::NotSupported("no samples (fake)".into()));
        }
        let at = state.next % state.script.len();
        state.next = state.next.wrapping_add(1);
        Ok(state.script[at].clone())
    }
}

/// One setter call on [`FakeGpuControl`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GpuCall {
    /// `set_clock_offset`.
    ClockOffset(i32),
    /// `set_memory_offset`.
    MemoryOffset(i32),
    /// `reset_locked_clocks`.
    ResetLockedClocks,
    /// `set_power_limit`.
    PowerLimit(u32),
    /// `set_fan_speed`.
    FanSpeed(u8),
    /// `set_fan_auto`.
    FanAuto,
}

#[derive(Debug)]
struct ControlState {
    clock_offset_mhz: i32,
    memory_offset_mhz: i32,
    power_limit_w: u32,
    fan: FanSetting,
    calls: Vec<GpuCall>,
    fail_at_call: Option<usize>,
    prior_unreadable: bool,
}

/// GPU setters that change values in memory and record every call.
#[derive(Debug, Clone)]
pub struct FakeGpuControl {
    state: Arc<Mutex<ControlState>>,
}

impl Default for FakeGpuControl {
    fn default() -> Self {
        Self::new()
    }
}

impl FakeGpuControl {
    /// Driver defaults: no offsets, a 320 W limit, automatic fan.
    pub fn new() -> Self {
        FakeGpuControl {
            state: Arc::new(Mutex::new(ControlState {
                clock_offset_mhz: 0,
                memory_offset_mhz: 0,
                power_limit_w: 320,
                fan: FanSetting::Auto,
                calls: Vec::new(),
                fail_at_call: None,
                prior_unreadable: false,
            })),
        }
    }

    /// Makes setter call number `n` (counting from 0) fail.
    pub fn fail_at_call(&self, n: Option<usize>) {
        self.lock().fail_at_call = n;
    }

    /// Makes every prior read unknown.
    pub fn set_prior_unreadable(&self, unreadable: bool) {
        self.lock().prior_unreadable = unreadable;
    }

    /// Every setter call so far, failed ones included.
    pub fn calls(&self) -> Vec<GpuCall> {
        self.lock().calls.clone()
    }

    /// The current values: clock offset, memory offset, power limit, fan.
    pub fn current(&self) -> (i32, i32, u32, FanSetting) {
        let s = self.lock();
        (
            s.clock_offset_mhz,
            s.memory_offset_mhz,
            s.power_limit_w,
            s.fan,
        )
    }

    fn lock(&self) -> MutexGuard<'_, ControlState> {
        self.state.lock().unwrap_or_else(|e| e.into_inner())
    }

    fn call(&mut self, call: GpuCall) -> Result<(), GpuError> {
        let mut s = self.lock();
        let index = s.calls.len();
        s.calls.push(call);
        if s.fail_at_call == Some(index) {
            return Err(GpuError::Failed(format!("fake failure at call {index}")));
        }
        match call {
            GpuCall::ClockOffset(v) => s.clock_offset_mhz = v,
            GpuCall::MemoryOffset(v) => s.memory_offset_mhz = v,
            // The fake never locks clocks, so a reset only shows in `calls`.
            GpuCall::ResetLockedClocks => {}
            GpuCall::PowerLimit(v) => s.power_limit_w = v,
            GpuCall::FanSpeed(v) => s.fan = FanSetting::Percent(v),
            GpuCall::FanAuto => s.fan = FanSetting::Auto,
        }
        Ok(())
    }
}

impl GpuControl for FakeGpuControl {
    fn read_prior(&mut self) -> GpuPrior {
        let s = self.lock();
        if s.prior_unreadable {
            let why = || "not readable (fake)".to_owned();
            return GpuPrior {
                clock_offset_mhz: Prior::Unknown(why()),
                memory_offset_mhz: Prior::Unknown(why()),
                power_limit_w: Prior::Unknown(why()),
                fan: Prior::Unknown(why()),
            };
        }
        GpuPrior {
            clock_offset_mhz: Prior::Known(s.clock_offset_mhz),
            memory_offset_mhz: Prior::Known(s.memory_offset_mhz),
            power_limit_w: Prior::Known(s.power_limit_w),
            fan: Prior::Known(s.fan),
        }
    }

    fn set_clock_offset(&mut self, mhz: i32) -> Result<(), GpuError> {
        self.call(GpuCall::ClockOffset(mhz))
    }

    fn set_memory_offset(&mut self, mhz: i32) -> Result<(), GpuError> {
        self.call(GpuCall::MemoryOffset(mhz))
    }

    fn reset_locked_clocks(&mut self) -> Result<(), GpuError> {
        self.call(GpuCall::ResetLockedClocks)
    }

    fn set_power_limit(&mut self, watts: u32) -> Result<(), GpuError> {
        self.call(GpuCall::PowerLimit(watts))
    }

    fn set_fan_speed(&mut self, percent: u8) -> Result<(), GpuError> {
        self.call(GpuCall::FanSpeed(percent))
    }

    fn set_fan_auto(&mut self) -> Result<(), GpuError> {
        self.call(GpuCall::FanAuto)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn script_wraps_and_counts_polls() {
        let mut gpu = FakeGpu::demo();
        let watcher = gpu.clone();
        let first = gpu.metrics().unwrap();
        for _ in 0..5 {
            gpu.metrics().unwrap();
        }
        assert_eq!(gpu.metrics().unwrap(), first);
        assert_eq!(watcher.polls(), 7);
    }

    #[test]
    fn demo_reaches_critical_heat() {
        let mut gpu = FakeGpu::demo();
        let hottest = (0..6)
            .map(|_| gpu.metrics().unwrap().temperature_c)
            .max()
            .unwrap();
        assert!(hottest >= 90, "{hottest}");
    }

    #[test]
    fn missing_nvml_is_an_error_not_a_panic() {
        let mut gpu = FakeGpu::demo();
        gpu.set_fault(Some(GpuFault::Missing));
        assert!(matches!(gpu.info(), Err(GpuError::Unavailable(_))));
        assert!(matches!(gpu.metrics(), Err(GpuError::Unavailable(_))));
        gpu.set_fault(None);
        assert!(gpu.metrics().is_ok());
        let mut empty = FakeGpu::new(FakeGpu::demo().info().unwrap(), vec![]);
        assert!(matches!(empty.metrics(), Err(GpuError::NotSupported(_))));
    }

    #[test]
    fn control_records_and_fails_at_the_chosen_call() {
        let mut control = FakeGpuControl::new();
        let watcher = control.clone();
        control.fail_at_call(Some(1));
        control.set_clock_offset(150).unwrap();
        assert!(control.set_power_limit(250).is_err());
        control.set_fan_speed(70).unwrap();
        assert_eq!(
            watcher.calls(),
            [
                GpuCall::ClockOffset(150),
                GpuCall::PowerLimit(250),
                GpuCall::FanSpeed(70)
            ]
        );
        assert_eq!(watcher.current(), (150, 0, 320, FanSetting::Percent(70)));
        assert_eq!(control.read_prior().clock_offset_mhz, Prior::Known(150));
        control.set_prior_unreadable(true);
        assert!(control.read_prior().fan.known().is_none());
    }
}
