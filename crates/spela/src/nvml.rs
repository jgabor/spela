//! GPU 0 through NVML: the reads every process may do, and the setters only
//! the root helper calls.
//!
//! nvml-wrapper loads `libnvidia-ml.so.1` at run time, so a machine with no
//! NVIDIA driver gets an error value, never a crash, and the binary starts
//! anywhere. NVML is opened lazily, on the first read or setter, never at
//! startup: init costs 4 to 17 ms. There is no `nvidia-smi` fallback
//! (decision 58). Only GPU 0 is used (decision 25). The device is looked up
//! per call, because `Device` borrows `Nvml`.
//!
//! The clock and memory offsets are NVML's GPC and memory VF offsets
//! (decisions 9 and 32). They are live-unverified until the user's root
//! test.

use nvml_wrapper::Nvml;
use nvml_wrapper::bitmasks::device::ThrottleReasons as NvmlThrottle;
use nvml_wrapper::enum_wrappers::device::{Clock, TemperatureSensor};
use nvml_wrapper::enums::device::FanControlPolicy;
use nvml_wrapper::error::NvmlError;

use spela_core::backend::{
    FanSetting, Gpu, GpuControl, GpuError, GpuInfo, GpuMetrics, GpuPrior, Prior, ThrottleReasons,
};

/// Opens NVML and checks there is a GPU 0.
pub fn open() -> Result<Nvml, GpuError> {
    let nvml = Nvml::init().map_err(gpu_error)?;
    match nvml.device_count().map_err(gpu_error)? {
        0 => Err(GpuError::Unavailable("NVML found no GPU".into())),
        _ => Ok(nvml),
    }
}

/// An NVML error as a [`GpuError`]: a missing library or driver is
/// unavailable, an unsupported call is not supported, the rest fail.
pub fn gpu_error(e: NvmlError) -> GpuError {
    match e {
        NvmlError::NotSupported => GpuError::NotSupported(e.to_string()),
        NvmlError::LibloadingError(_)
        | NvmlError::FailedToLoadSymbol(_)
        | NvmlError::LibraryNotFound
        | NvmlError::DriverNotLoaded
        | NvmlError::Uninitialized
        | NvmlError::LibRmVersionMismatch => GpuError::Unavailable(e.to_string()),
        other => GpuError::Failed(other.to_string()),
    }
}

/// GPU 0 reads, opened on the first call. A failed open is kept, so a
/// machine without NVIDIA pays for one attempt, not one per poll.
#[derive(Default)]
pub struct NvmlGpu {
    nvml: Option<Result<Nvml, GpuError>>,
}

impl std::fmt::Debug for NvmlGpu {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("NvmlGpu")
            .field("opened", &self.nvml.as_ref().map(Result::is_ok))
            .finish()
    }
}

impl NvmlGpu {
    /// A reader that opens NVML on its first call.
    pub fn lazy() -> NvmlGpu {
        NvmlGpu::default()
    }

    fn nvml(&mut self) -> Result<&Nvml, GpuError> {
        match self.nvml.get_or_insert_with(open) {
            Ok(nvml) => Ok(nvml),
            Err(e) => Err(e.clone()),
        }
    }
}

impl Gpu for NvmlGpu {
    fn info(&mut self) -> Result<GpuInfo, GpuError> {
        let nvml = self.nvml()?;
        let driver_version = nvml.sys_driver_version().map_err(gpu_error)?;
        let device = nvml.device_by_index(0).map_err(gpu_error)?;
        Ok(GpuInfo {
            name: device.name().unwrap_or_default(),
            driver_version,
            memory_total_mib: device
                .memory_info()
                .map(|m| m.total / (1024 * 1024))
                .unwrap_or(0),
            power_draw_mw: device.power_usage().ok(),
            power_limit_range_w: device
                .power_management_limit_constraints()
                .ok()
                .map(|c| (c.min_limit / 1000, c.max_limit / 1000)),
        })
    }

    /// One sweep, as Go's: each read that fails leaves its value at zero or
    /// missing, so one unsupported sensor never hides the rest.
    fn metrics(&mut self) -> Result<GpuMetrics, GpuError> {
        let nvml = self.nvml()?;
        let device = nvml.device_by_index(0).map_err(gpu_error)?;
        let memory = device.memory_info().ok();
        let throttle = device
            .current_throttle_reasons()
            .map(|r| ThrottleReasons {
                thermal_hardware: r.contains(NvmlThrottle::HW_THERMAL_SLOWDOWN),
                thermal_software: r.contains(NvmlThrottle::SW_THERMAL_SLOWDOWN),
                power_cap: r.contains(NvmlThrottle::SW_POWER_CAP),
                power_brake: r.contains(NvmlThrottle::HW_POWER_BRAKE_SLOWDOWN),
            })
            .unwrap_or_default();
        Ok(GpuMetrics {
            temperature_c: device.temperature(TemperatureSensor::Gpu).unwrap_or(0),
            utilization_percent: device.utilization_rates().map_or(0, |u| u.gpu),
            memory_used_mib: memory.as_ref().map_or(0, |m| m.used / (1024 * 1024)),
            memory_total_mib: memory.as_ref().map_or(0, |m| m.total / (1024 * 1024)),
            power_draw_w: device.power_usage().map_or(0, |mw| mw / 1000),
            power_limit_w: device.power_management_limit().ok().map(|mw| mw / 1000),
            power_max_limit_w: device
                .power_management_limit_constraints()
                .ok()
                .map(|c| c.max_limit / 1000),
            throttle,
            graphics_clock_mhz: device.clock_info(Clock::Graphics).unwrap_or(0),
            memory_clock_mhz: device.clock_info(Clock::Memory).unwrap_or(0),
            fan_percent: device.fan_speed(0).ok(),
        })
    }
}

/// GPU 0's setters, for the root helper only.
pub struct NvmlControl {
    nvml: Nvml,
}

impl std::fmt::Debug for NvmlControl {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("NvmlControl")
    }
}

impl NvmlControl {
    /// Opens NVML for the helper.
    pub fn open() -> Result<NvmlControl, GpuError> {
        open().map(|nvml| NvmlControl { nvml })
    }

    fn device(&self) -> Result<nvml_wrapper::Device<'_>, GpuError> {
        self.nvml.device_by_index(0).map_err(gpu_error)
    }

    /// Runs `set` on every fan, or on fan 0 when the fan count cannot be
    /// read, as Go did (parity-inventory.md, domain 10).
    fn each_fan(
        &self,
        mut set: impl FnMut(&mut nvml_wrapper::Device<'_>, u32) -> Result<(), NvmlError>,
    ) -> Result<(), GpuError> {
        let mut device = self.device()?;
        let fans = device.num_fans().unwrap_or(1).max(1);
        for fan in 0..fans {
            set(&mut device, fan).map_err(gpu_error)?;
        }
        Ok(())
    }
}

impl GpuControl for NvmlControl {
    fn read_prior(&mut self) -> GpuPrior {
        fn unknown<T>(e: GpuError) -> Prior<T> {
            Prior::Unknown(e.to_string())
        }
        let device = match self.device() {
            Ok(d) => d,
            Err(e) => {
                return GpuPrior {
                    clock_offset_mhz: unknown(e.clone()),
                    memory_offset_mhz: unknown(e.clone()),
                    power_limit_w: unknown(e.clone()),
                    fan: unknown(e),
                };
            }
        };
        let read = |r: Result<i32, NvmlError>| match r {
            Ok(v) => Prior::Known(v),
            Err(e) => unknown(gpu_error(e)),
        };
        let fan = match device.fan_control_policy(0) {
            Ok(FanControlPolicy::TemperatureContinousSw) => Prior::Known(FanSetting::Auto),
            Ok(FanControlPolicy::Manual) => match device.fan_speed(0) {
                Ok(p) => Prior::Known(FanSetting::Percent(p.min(100) as u8)),
                Err(e) => unknown(gpu_error(e)),
            },
            Err(e) => unknown(gpu_error(e)),
        };
        GpuPrior {
            clock_offset_mhz: read(device.gpc_clock_vf_offset()),
            memory_offset_mhz: read(device.mem_clock_vf_offset()),
            // Rounded to the nearest watt: the restore sets whole watts.
            power_limit_w: match device.power_management_limit() {
                Ok(mw) => Prior::Known((mw + 500) / 1000),
                Err(e) => unknown(gpu_error(e)),
            },
            fan,
        }
    }

    fn power_limit_range(&mut self) -> Result<(u32, u32), GpuError> {
        let c = self
            .device()?
            .power_management_limit_constraints()
            .map_err(gpu_error)?;
        Ok((c.min_limit.div_ceil(1000), c.max_limit / 1000))
    }

    fn set_clock_offset(&mut self, mhz: i32) -> Result<(), GpuError> {
        self.device()?
            .set_gpc_clock_vf_offset(mhz)
            .map_err(gpu_error)
    }

    fn set_memory_offset(&mut self, mhz: i32) -> Result<(), GpuError> {
        self.device()?
            .set_mem_clock_vf_offset(mhz)
            .map_err(gpu_error)
    }

    /// Resets both locked clock ranges, graphics and memory, as Go's
    /// `ResetClocks` did. Both are tried; the first error is returned.
    fn reset_locked_clocks(&mut self) -> Result<(), GpuError> {
        let mut device = self.device()?;
        let gpu = device.reset_gpu_locked_clocks();
        let mem = device.reset_mem_locked_clocks();
        gpu.and(mem).map_err(gpu_error)
    }

    fn set_power_limit(&mut self, watts: u32) -> Result<(), GpuError> {
        let mw = watts
            .checked_mul(1000)
            .ok_or_else(|| GpuError::Failed(format!("{watts} W is too large")))?;
        self.device()?
            .set_power_management_limit(mw)
            .map_err(gpu_error)
    }

    fn set_fan_speed(&mut self, percent: u8) -> Result<(), GpuError> {
        self.each_fan(|d, fan| d.set_fan_speed(fan, u32::from(percent)))
    }

    fn set_fan_auto(&mut self) -> Result<(), GpuError> {
        self.each_fan(|d, fan| d.set_default_fan_speed(fan))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_missing_library_is_an_error_value_not_a_panic() {
        // Points the loader at a file that does not exist, so this never
        // reaches the real driver.
        let dir = tempfile::tempdir().unwrap();
        let missing = dir.path().join("libnvidia-ml.so.1");
        let err = Nvml::builder()
            .lib_path(missing.as_os_str())
            .init()
            .map(|_| ())
            .unwrap_err();
        assert!(
            matches!(gpu_error(err), GpuError::Unavailable(_)),
            "a missing library means NVML is unavailable"
        );
    }

    #[test]
    fn errors_map_to_their_kinds() {
        assert!(matches!(
            gpu_error(NvmlError::NotSupported),
            GpuError::NotSupported(_)
        ));
        assert!(matches!(
            gpu_error(NvmlError::DriverNotLoaded),
            GpuError::Unavailable(_)
        ));
        assert!(matches!(
            gpu_error(NvmlError::NoPermission),
            GpuError::Failed(_)
        ));
    }
}
