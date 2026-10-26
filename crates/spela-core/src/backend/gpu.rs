//! GPU 0 reads, and the setters only the root helper calls.
//!
//! spela uses device 0 only (decision 25), and NVML only (decision 58).

use std::fmt;

/// A value read before a change. `Unknown` means "do not restore": spela
/// never restores a guess.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Prior<T> {
    /// The value that was there.
    Known(T),
    /// Why it could not be read.
    Unknown(String),
}

impl<T> Prior<T> {
    /// The value, when it was read.
    pub fn known(&self) -> Option<&T> {
        match self {
            Prior::Known(v) => Some(v),
            Prior::Unknown(_) => None,
        }
    }

    /// Changes the value inside a `Known`.
    pub fn map<U>(self, f: impl FnOnce(T) -> U) -> Prior<U> {
        match self {
            Prior::Known(v) => Prior::Known(f(v)),
            Prior::Unknown(why) => Prior::Unknown(why),
        }
    }
}

/// What the driver says about the card.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GpuInfo {
    /// The product name, such as `NVIDIA GeForce RTX 4080`.
    pub name: String,
    /// The driver version, such as `580.95.05`.
    pub driver_version: String,
    /// Total VRAM in MiB.
    pub memory_total_mib: u64,
    /// The board's power draw when asked, in mW, for `gpu info`'s one
    /// decimal. `None` when the card does not report it.
    pub power_draw_mw: Option<u32>,
    /// The lowest and highest power limit the card accepts, in W.
    pub power_limit_range_w: Option<(u32, u32)>,
}

/// Why the driver holds the clocks back: NVML's four throttle reasons that
/// the alert evaluator reads (Go's `ThrottleReasons`).
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct ThrottleReasons {
    /// Hardware thermal slowdown.
    pub thermal_hardware: bool,
    /// Software thermal slowdown.
    pub thermal_software: bool,
    /// The software power cap.
    pub power_cap: bool,
    /// The hardware power brake.
    pub power_brake: bool,
}

impl ThrottleReasons {
    /// A thermal reason: hardware or software thermal slowdown.
    pub fn thermal(self) -> bool {
        self.thermal_hardware || self.thermal_software
    }

    /// A power reason: the power cap or the power brake.
    pub fn power(self) -> bool {
        self.power_cap || self.power_brake
    }
}

/// One metrics sample.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct GpuMetrics {
    /// Core temperature in °C.
    pub temperature_c: u32,
    /// GPU utilisation in percent.
    pub utilization_percent: u32,
    /// VRAM in use, in MiB.
    pub memory_used_mib: u64,
    /// Total VRAM in MiB.
    pub memory_total_mib: u64,
    /// Board power draw in W.
    pub power_draw_w: u32,
    /// The power limit in force in W, when the card reports one.
    pub power_limit_w: Option<u32>,
    /// The highest power limit the card accepts in W, when it reports one.
    pub power_max_limit_w: Option<u32>,
    /// The throttle reasons. All false when the driver does not report
    /// them.
    pub throttle: ThrottleReasons,
    /// Graphics clock in MHz.
    pub graphics_clock_mhz: u32,
    /// Memory clock in MHz.
    pub memory_clock_mhz: u32,
    /// Fan speed in percent, when the card reports one.
    pub fan_percent: Option<u32>,
}

/// A fan setting: the driver's automatic curve or a fixed speed.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FanSetting {
    /// The driver controls the fan.
    Auto,
    /// A fixed speed in percent.
    Percent(u8),
}

/// Values read as root before the helper changes anything: each one a
/// restore can put back. Locked clocks are not here. `gpu reset` only ever
/// resets them and nothing restores them, and NVML has setters and a reset
/// for locked clocks but no getter (inferred from the NVML API, unverified
/// on nvml-wrapper).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GpuPrior {
    /// The graphics clock offset in MHz.
    pub clock_offset_mhz: Prior<i32>,
    /// The memory clock offset in MHz.
    pub memory_offset_mhz: Prior<i32>,
    /// The power limit in W.
    pub power_limit_w: Prior<u32>,
    /// The fan policy.
    pub fan: Prior<FanSetting>,
}

/// Why a GPU call failed. Nothing panics.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum GpuError {
    /// NVML is missing or did not start, so GPU features are off.
    #[error("NVIDIA management library not available: {0}")]
    Unavailable(String),
    /// The card or driver does not support this.
    #[error("not supported by this GPU or driver: {0}")]
    NotSupported(String),
    /// Any other failure.
    #[error("{0}")]
    Failed(String),
}

/// GPU 0 reads. Every read can fail.
pub trait Gpu: Send {
    /// Name, driver and VRAM.
    fn info(&mut self) -> Result<GpuInfo, GpuError>;
    /// One metrics sample.
    fn metrics(&mut self) -> Result<GpuMetrics, GpuError>;
}

/// The setters, used only inside the root helper.
///
/// The offsets are NVML's GPC and memory VF offsets (decisions 9 and 32).
/// They are live-unverified until the user's root test.
pub trait GpuControl: Send {
    /// Reads every value the setters below can change.
    fn read_prior(&mut self) -> GpuPrior;
    /// The lowest and highest power limit the card accepts, in W. The
    /// helper checks a requested limit against it before any change.
    fn power_limit_range(&mut self) -> Result<(u32, u32), GpuError>;
    /// Sets the graphics clock offset.
    fn set_clock_offset(&mut self, mhz: i32) -> Result<(), GpuError>;
    /// Sets the memory clock offset.
    fn set_memory_offset(&mut self, mhz: i32) -> Result<(), GpuError>;
    /// Gives the graphics clocks back to the driver, for `gpu reset`.
    fn reset_locked_clocks(&mut self) -> Result<(), GpuError>;
    /// Sets the power limit.
    fn set_power_limit(&mut self, watts: u32) -> Result<(), GpuError>;
    /// Fixes the fan speed.
    fn set_fan_speed(&mut self, percent: u8) -> Result<(), GpuError>;
    /// Gives the fan back to the driver.
    fn set_fan_auto(&mut self) -> Result<(), GpuError>;
}

impl fmt::Display for FanSetting {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            FanSetting::Auto => f.write_str("auto"),
            FanSetting::Percent(p) => write!(f, "{p}%"),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn prior_known_and_map() {
        let p: Prior<i32> = Prior::Known(150);
        assert_eq!(p.known(), Some(&150));
        assert_eq!(p.map(|v| v * 2), Prior::Known(300));
        let u: Prior<i32> = Prior::Unknown("not readable".into());
        assert_eq!(u.known(), None);
        assert_eq!(u.map(|v| v * 2), Prior::Unknown("not readable".into()));
    }

    #[test]
    fn fan_setting_text() {
        assert_eq!(FanSetting::Auto.to_string(), "auto");
        assert_eq!(FanSetting::Percent(60).to_string(), "60%");
    }
}
