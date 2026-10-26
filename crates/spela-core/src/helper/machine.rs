//! What the root helper changes: GPU 0 through a [`GpuControl`] and the
//! CPU through sysfs.
//!
//! [`Machine`] is the seam between the helper's loops and the hardware.
//! [`SystemMachine`] is the real one; the fake privilege runner wraps it to
//! fail at a chosen step. The rules (architecture.md, Partial-failure
//! rules):
//!
//! - Every target is checked before it is applied. A target that does not
//!   suit the machine skips its step only.
//! - The prior value is read as root right before the change. In `--serve`
//!   a step whose prior cannot be read is skipped, because spela never
//!   restores a guess. The fan is the one exception: a fan whose mode cannot
//!   be read goes back to automatic. `--once` never restores, so there the
//!   prior is only shown in the report (see [`super::serve::once`]).
//! - A step that writes several things, the per-CPU governor and the
//!   per-fan setting, puts back the ones it wrote when a later one fails,
//!   so a failed step changed nothing.
//! - The power limit prior is kept in mW, exactly as read, so its restore
//!   never writes a rounded value outside the card's range.

use std::path::{Path, PathBuf};

use crate::backend::cpu::{self, CPU_DIR, SMT_CONTROL_FILE};
use crate::backend::{FanSetting, GpuControl, GpuError, GpuPrior, Prior, SettingValue, Step};

/// The graphics clock offsets the helper accepts, in MHz. NVML reports no
/// range through nvml-wrapper 0.13, so these are spela's own bounds until
/// the user's root test checks what the driver takes (decision 32).
pub const CLOCK_OFFSET_RANGE: (i32, i32) = (-1000, 1000);
/// The memory clock offsets the helper accepts, in MHz. The same caveat as
/// [`CLOCK_OFFSET_RANGE`].
pub const MEMORY_OFFSET_RANGE: (i32, i32) = (-3000, 3000);

/// A value read before a change: what the report shows, and what a restore
/// writes back.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Saved {
    /// One value.
    Value(SettingValue),
    /// The governor of each CPU the step wrote, by CPU number.
    Governors(Vec<(u32, String)>),
    /// The power limit in mW, as read.
    PowerLimitMw(u32),
    /// Each fan's mode, by fan index. A fan whose mode could not be read
    /// is automatic here.
    Fans(Vec<FanSetting>),
}

impl Saved {
    /// The prior value the report shows. For governors, each distinct name
    /// in CPU order, joined with `, `: usually just one. The power limit is
    /// rounded to the nearest watt. For fans it is fan 0's mode, the fan
    /// Monitor shows; the restore still puts each fan back to its own.
    pub fn shown(&self) -> SettingValue {
        match self {
            Saved::Value(v) => v.clone(),
            Saved::PowerLimitMw(mw) => SettingValue::Watts(mw.saturating_add(500) / 1000),
            Saved::Fans(fans) => {
                SettingValue::Fan(fans.first().copied().unwrap_or(FanSetting::Auto))
            }
            Saved::Governors(list) => {
                let mut names: Vec<&str> = Vec::new();
                for (_, g) in list {
                    if !names.contains(&g.as_str()) {
                        names.push(g);
                    }
                }
                SettingValue::Governor(names.join(", "))
            }
        }
    }
}

/// What the helper changes. Every error is a plain reason for the report.
pub trait Machine: Send {
    /// Checks a target before anything changes. An error skips the step.
    fn check(&mut self, step: Step, target: &SettingValue) -> Result<(), String>;
    /// Reads what a restore needs, right before the change. An error skips
    /// the step.
    fn read_prior(&mut self, step: Step) -> Result<Saved, String>;
    /// Makes the change. A failure leaves the step as it was.
    fn apply(&mut self, step: Step, target: &SettingValue) -> Result<(), String>;
    /// Puts a saved value back.
    fn restore(&mut self, step: Step, saved: &Saved) -> Result<(), String>;
}

/// Opens GPU 0's setters. The helper calls it only when a GPU step comes,
/// so a CPU-only batch never loads NVML.
pub type GpuOpener = Box<dyn FnMut() -> Result<Box<dyn GpuControl>, GpuError> + Send>;

/// The real machine: GPU 0 and the CPU files under a sysfs root.
pub struct SystemMachine {
    open_gpu: GpuOpener,
    gpu: Option<Result<Box<dyn GpuControl>, String>>,
    prior: Option<GpuPrior>,
    sys: Option<PathBuf>,
}

impl std::fmt::Debug for SystemMachine {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SystemMachine")
            .field("sys", &self.sys)
            .finish_non_exhaustive()
    }
}

impl SystemMachine {
    /// A machine whose GPU comes from `open_gpu` and whose CPU files are
    /// under the sysfs root `sys` (`/sys` in the helper). Without a sysfs
    /// root, CPU steps are skipped.
    pub fn new(open_gpu: GpuOpener, sys: Option<PathBuf>) -> SystemMachine {
        SystemMachine {
            open_gpu,
            gpu: None,
            prior: None,
            sys,
        }
    }

    fn gpu(&mut self) -> Result<&mut dyn GpuControl, String> {
        if self.gpu.is_none() {
            self.gpu = Some((self.open_gpu)().map_err(|e| e.to_string()));
        }
        match self.gpu.as_mut() {
            Some(Ok(gpu)) => Ok(gpu.as_mut()),
            Some(Err(e)) => Err(e.clone()),
            None => unreachable!("opened above"),
        }
    }

    /// GPU 0's prior values, read once per helper, before its first GPU
    /// change.
    fn gpu_prior(&mut self) -> Result<GpuPrior, String> {
        if let Some(p) = &self.prior {
            return Ok(p.clone());
        }
        let prior = self.gpu()?.read_prior();
        self.prior = Some(prior.clone());
        Ok(prior)
    }

    fn cpu_dir(&self) -> Result<PathBuf, String> {
        self.sys
            .as_ref()
            .map(|root| root.join(CPU_DIR))
            .ok_or_else(|| "no sysfs tree".to_owned())
    }

    /// The online CPUs with a governor file: the ones a governor step
    /// writes.
    fn governed_cpus(&self) -> Result<Vec<u32>, String> {
        let dir = self.cpu_dir()?;
        let cpus: Vec<u32> = cpu::online_cpus(&dir)
            .into_iter()
            .filter(|n| governor_file(&dir, *n).is_file())
            .collect();
        if cpus.is_empty() {
            return Err("no online CPU has a frequency governor".into());
        }
        Ok(cpus)
    }

    fn check_governor(&self, governor: &str) -> Result<(), String> {
        let well_formed = !governor.is_empty()
            && governor.len() <= 32
            && governor
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'-');
        if !well_formed {
            return Err(format!("{governor:?} is not a governor name"));
        }
        let dir = self.cpu_dir()?;
        for n in self.governed_cpus()? {
            let file = dir.join(format!("cpu{n}/cpufreq/scaling_available_governors"));
            let text = read_trimmed(&file)?;
            let available = cpu::parse_governors(&text);
            if !available.iter().any(|g| g == governor) {
                let list = if available.is_empty() {
                    "none".to_owned()
                } else {
                    available.join(", ")
                };
                return Err(format!(
                    "CPU governor {governor:?} is not available (available: {list})"
                ));
            }
        }
        Ok(())
    }

    /// Writes each CPU's governor. When one write fails, the CPUs already
    /// written get their old governor back.
    fn write_governors(
        &self,
        values: &[(u32, String)],
        undo: &[(u32, String)],
    ) -> Result<(), String> {
        let dir = self.cpu_dir()?;
        for (i, (n, governor)) in values.iter().enumerate() {
            if let Err(e) = write_file(&governor_file(&dir, *n), governor) {
                for (m, old) in undo
                    .iter()
                    .filter(|(m, _)| values[..i].iter().any(|(w, _)| w == m))
                {
                    let _ = write_file(&governor_file(&dir, *m), old);
                }
                return Err(e);
            }
        }
        Ok(())
    }

    fn read_smt(&self) -> Result<bool, String> {
        let text = read_trimmed(&self.cpu_dir()?.join(SMT_CONTROL_FILE))?;
        match text.as_str() {
            "on" => Ok(true),
            "off" => Ok(false),
            other => Err(format!("SMT control is {other:?}")),
        }
    }

    fn write_smt(&self, on: bool) -> Result<(), String> {
        write_file(
            &self.cpu_dir()?.join(SMT_CONTROL_FILE),
            if on { "on" } else { "off" },
        )
    }

    fn set_value(&mut self, step: Step, value: &SettingValue) -> Result<(), String> {
        let gpu = |r: Result<(), GpuError>| r.map_err(|e| e.to_string());
        match (step, value) {
            (Step::LockedClocks, SettingValue::DriverDefault) => {
                gpu(self.gpu()?.reset_locked_clocks())
            }
            (Step::ClockOffset, SettingValue::Mhz(v)) => gpu(self.gpu()?.set_clock_offset(*v)),
            (Step::MemoryOffset, SettingValue::Mhz(v)) => gpu(self.gpu()?.set_memory_offset(*v)),
            (Step::PowerLimit, SettingValue::Watts(v)) => {
                let mw = v
                    .checked_mul(1000)
                    .ok_or_else(|| format!("{v} W is too large"))?;
                gpu(self.gpu()?.set_power_limit_mw(mw))
            }
            (Step::Fan, SettingValue::Fan(target)) => self.write_fans(*target),
            (Step::CpuGovernor, SettingValue::Governor(g)) => {
                let cpus = self.governed_cpus()?;
                let values: Vec<(u32, String)> = cpus.iter().map(|n| (*n, g.clone())).collect();
                let undo = self.read_governors(&cpus).unwrap_or_default();
                self.write_governors(&values, &undo)
            }
            (Step::Smt, SettingValue::Smt(on)) => self.write_smt(*on),
            (step, value) => Err(mismatch(step, value)),
        }
    }

    /// Each fan's prior mode, an unreadable one as automatic.
    fn fan_priors(&mut self) -> Result<Vec<FanSetting>, String> {
        let fans: Vec<FanSetting> = self
            .gpu_prior()?
            .fans
            .iter()
            .map(|f| f.known().copied().unwrap_or(FanSetting::Auto))
            .collect();
        if fans.is_empty() {
            Ok(vec![FanSetting::Auto])
        } else {
            Ok(fans)
        }
    }

    /// Sets every fan to `target`. When one fails, the fans already set go
    /// back to their prior modes, so a failed step changed nothing.
    fn write_fans(&mut self, target: FanSetting) -> Result<(), String> {
        let priors = self.fan_priors()?;
        let many = priors.len() > 1;
        let gpu = self.gpu()?;
        for fan in 0..priors.len() {
            if let Err(e) = gpu.set_fan(fan as u32, target) {
                for (back, old) in priors[..fan].iter().enumerate() {
                    let _ = gpu.set_fan(back as u32, *old);
                }
                return Err(fan_error(many, fan, &e));
            }
        }
        Ok(())
    }

    /// Puts each fan back to its own mode. Every fan is tried; the errors
    /// are joined.
    fn restore_fans(&mut self, fans: &[FanSetting]) -> Result<(), String> {
        let many = fans.len() > 1;
        let gpu = self.gpu()?;
        let failed: Vec<String> = fans
            .iter()
            .enumerate()
            .filter_map(|(fan, setting)| {
                gpu.set_fan(fan as u32, *setting)
                    .err()
                    .map(|e| fan_error(many, fan, &e))
            })
            .collect();
        if failed.is_empty() {
            Ok(())
        } else {
            Err(failed.join("; "))
        }
    }

    fn read_governors(&self, cpus: &[u32]) -> Result<Vec<(u32, String)>, String> {
        let dir = self.cpu_dir()?;
        cpus.iter()
            .map(|n| read_trimmed(&governor_file(&dir, *n)).map(|g| (*n, g)))
            .collect()
    }
}

impl Machine for SystemMachine {
    fn check(&mut self, step: Step, target: &SettingValue) -> Result<(), String> {
        let within = |v: i32, (lo, hi): (i32, i32), what: &str| {
            if (lo..=hi).contains(&v) {
                Ok(())
            } else {
                Err(format!("{what} {v} MHz is outside {lo} to {hi} MHz"))
            }
        };
        match (step, target) {
            (Step::LockedClocks, SettingValue::DriverDefault) => Ok(()),
            (Step::ClockOffset, SettingValue::Mhz(v)) => {
                within(*v, CLOCK_OFFSET_RANGE, "a clock offset of")
            }
            (Step::MemoryOffset, SettingValue::Mhz(v)) => {
                within(*v, MEMORY_OFFSET_RANGE, "a memory offset of")
            }
            (Step::PowerLimit, SettingValue::Watts(w)) => {
                let (lo, hi) = self
                    .gpu()?
                    .power_limit_range()
                    .map_err(|e| format!("the power limit range cannot be read: {e}"))?;
                if (lo..=hi).contains(w) {
                    Ok(())
                } else {
                    Err(format!("a power limit of {w} W is outside {lo} to {hi} W"))
                }
            }
            (Step::Fan, SettingValue::Fan(FanSetting::Percent(p))) if *p > 100 => {
                Err(format!("a fan speed of {p}% is outside 0 to 100%"))
            }
            (Step::Fan, SettingValue::Fan(_)) => Ok(()),
            (Step::CpuGovernor, SettingValue::Governor(g)) => self.check_governor(g),
            (Step::Smt, SettingValue::Smt(_)) => Ok(()),
            (step, value) => Err(mismatch(step, value)),
        }
    }

    fn read_prior(&mut self, step: Step) -> Result<Saved, String> {
        let known = |p: Prior<SettingValue>| match p {
            Prior::Known(v) => Ok(Saved::Value(v)),
            Prior::Unknown(why) => Err(why),
        };
        match step {
            Step::LockedClocks => Err("locked clocks have no prior value".into()),
            Step::ClockOffset => known(self.gpu_prior()?.clock_offset_mhz.map(SettingValue::Mhz)),
            Step::MemoryOffset => known(self.gpu_prior()?.memory_offset_mhz.map(SettingValue::Mhz)),
            Step::PowerLimit => match self.gpu_prior()?.power_limit_mw {
                Prior::Known(mw) => Ok(Saved::PowerLimitMw(mw)),
                Prior::Unknown(why) => Err(why),
            },
            // A fan whose mode cannot be read goes back to automatic.
            Step::Fan => self.fan_priors().map(Saved::Fans),
            Step::CpuGovernor => {
                let cpus = self.governed_cpus()?;
                self.read_governors(&cpus).map(Saved::Governors)
            }
            Step::Smt => self
                .read_smt()
                .map(|on| Saved::Value(SettingValue::Smt(on))),
        }
    }

    fn apply(&mut self, step: Step, target: &SettingValue) -> Result<(), String> {
        self.set_value(step, target)
    }

    fn restore(&mut self, step: Step, saved: &Saved) -> Result<(), String> {
        match saved {
            Saved::Value(v) => self.set_value(step, v),
            Saved::PowerLimitMw(mw) => self
                .gpu()?
                .set_power_limit_mw(*mw)
                .map_err(|e| e.to_string()),
            Saved::Fans(fans) => self.restore_fans(fans),
            Saved::Governors(list) => {
                // A CPU that went offline since cannot take its governor
                // back; the others still do.
                let dir = self.cpu_dir()?;
                let mut failed = Vec::new();
                for (n, governor) in list {
                    if let Err(e) = write_file(&governor_file(&dir, *n), governor) {
                        failed.push(format!("cpu{n}: {e}"));
                    }
                }
                if failed.is_empty() {
                    Ok(())
                } else {
                    Err(failed.join("; "))
                }
            }
        }
    }
}

fn governor_file(cpu_dir: &Path, n: u32) -> PathBuf {
    cpu_dir.join(format!("cpu{n}/cpufreq/scaling_governor"))
}

/// A fan error, naming the fan when the card has more than one.
fn fan_error(many: bool, fan: usize, e: &GpuError) -> String {
    if many {
        format!("fan {fan}: {e}")
    } else {
        e.to_string()
    }
}

fn mismatch(step: Step, value: &SettingValue) -> String {
    format!("{value} is not a value for {step}")
}

fn read_trimmed(path: &Path) -> Result<String, String> {
    std::fs::read_to_string(path)
        .map(|s| s.trim().to_owned())
        .map_err(|e| format!("{}: {e}", path.display()))
}

/// Writes a sysfs file in place: sysfs files cannot be replaced, only
/// written.
fn write_file(path: &Path, value: &str) -> Result<(), String> {
    std::fs::write(path, value).map_err(|e| format!("{}: {e}", path.display()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fake::{FakeGpuControl, GpuCall};

    /// A sysfs tree: cpu0 to cpu3 on powersave, cpu2 offline, SMT on.
    fn sysfs() -> tempfile::TempDir {
        let root = tempfile::tempdir().unwrap();
        let cpu = root.path().join(CPU_DIR);
        for n in 0..4 {
            let dir = cpu.join(format!("cpu{n}/cpufreq"));
            std::fs::create_dir_all(&dir).unwrap();
            std::fs::write(dir.join("scaling_governor"), "powersave\n").unwrap();
            std::fs::write(
                dir.join("scaling_available_governors"),
                "performance powersave\n",
            )
            .unwrap();
        }
        std::fs::write(cpu.join("cpu2/online"), "0\n").unwrap();
        std::fs::write(cpu.join("cpu3/online"), "1\n").unwrap();
        std::fs::create_dir_all(cpu.join("smt")).unwrap();
        std::fs::write(cpu.join("smt/control"), "on\n").unwrap();
        root
    }

    fn machine(control: &FakeGpuControl, sys: Option<&Path>) -> SystemMachine {
        let control = control.clone();
        SystemMachine::new(
            Box::new(move || Ok(Box::new(control.clone()) as Box<dyn GpuControl>)),
            sys.map(Path::to_owned),
        )
    }

    fn governor(root: &Path, n: u32) -> String {
        read_trimmed(&governor_file(&root.join(CPU_DIR), n)).unwrap()
    }

    #[test]
    fn governors_go_to_online_cpus_only_and_come_back_per_cpu() {
        let sys = sysfs();
        std::fs::write(governor_file(&sys.path().join(CPU_DIR), 3), "schedutil\n").unwrap();
        let mut m = machine(&FakeGpuControl::new(), Some(sys.path()));
        let target = SettingValue::Governor("performance".into());
        m.check(Step::CpuGovernor, &target).unwrap();
        let saved = m.read_prior(Step::CpuGovernor).unwrap();
        assert_eq!(
            saved,
            Saved::Governors(vec![
                (0, "powersave".into()),
                (1, "powersave".into()),
                (3, "schedutil".into())
            ])
        );
        assert_eq!(
            saved.shown(),
            SettingValue::Governor("powersave, schedutil".into())
        );
        m.apply(Step::CpuGovernor, &target).unwrap();
        assert_eq!(governor(sys.path(), 0), "performance");
        assert_eq!(governor(sys.path(), 3), "performance");
        assert_eq!(governor(sys.path(), 2), "powersave", "offline, untouched");
        m.restore(Step::CpuGovernor, &saved).unwrap();
        assert_eq!(governor(sys.path(), 0), "powersave");
        assert_eq!(governor(sys.path(), 3), "schedutil");
    }

    #[test]
    fn a_governor_sysfs_does_not_list_is_refused() {
        let sys = sysfs();
        let mut m = machine(&FakeGpuControl::new(), Some(sys.path()));
        let e = m
            .check(
                Step::CpuGovernor,
                &SettingValue::Governor("conservative".into()),
            )
            .unwrap_err();
        assert_eq!(
            e,
            "CPU governor \"conservative\" is not available (available: performance, powersave)"
        );
        for bad in ["", "../../x", "per formance", &"a".repeat(33)] {
            assert!(
                m.check(Step::CpuGovernor, &SettingValue::Governor(bad.into()))
                    .is_err(),
                "{bad:?}"
            );
        }
        let mut none = machine(&FakeGpuControl::new(), None);
        assert!(none.read_prior(Step::Smt).is_err());
    }

    #[test]
    fn smt_reads_on_and_off_only() {
        let sys = sysfs();
        let mut m = machine(&FakeGpuControl::new(), Some(sys.path()));
        assert_eq!(
            m.read_prior(Step::Smt).unwrap(),
            Saved::Value(SettingValue::Smt(true))
        );
        m.apply(Step::Smt, &SettingValue::Smt(false)).unwrap();
        let control = sys.path().join(CPU_DIR).join(SMT_CONTROL_FILE);
        assert_eq!(read_trimmed(&control).unwrap(), "off");
        std::fs::write(&control, "notsupported\n").unwrap();
        let e = m.read_prior(Step::Smt).unwrap_err();
        assert!(e.contains("notsupported"), "{e}");
    }

    #[test]
    fn gpu_checks_use_the_card_range_and_spela_bounds() {
        let control = FakeGpuControl::new();
        let mut m = machine(&control, None);
        assert!(m.check(Step::PowerLimit, &SettingValue::Watts(250)).is_ok());
        let e = m
            .check(Step::PowerLimit, &SettingValue::Watts(400))
            .unwrap_err();
        assert_eq!(e, "a power limit of 400 W is outside 100 to 360 W");
        assert!(
            m.check(Step::ClockOffset, &SettingValue::Mhz(-1000))
                .is_ok()
        );
        assert!(
            m.check(Step::ClockOffset, &SettingValue::Mhz(1001))
                .is_err()
        );
        assert!(
            m.check(Step::MemoryOffset, &SettingValue::Mhz(-3001))
                .is_err()
        );
        assert!(
            m.check(Step::Fan, &SettingValue::Fan(FanSetting::Percent(101)))
                .is_err()
        );
        assert!(
            m.check(Step::Smt, &SettingValue::Mhz(1)).is_err(),
            "a value of the wrong kind"
        );
        control.set_prior_unreadable(true);
        let mut m = machine(&control, None);
        let e = m
            .check(Step::PowerLimit, &SettingValue::Watts(250))
            .unwrap_err();
        assert!(e.starts_with("the power limit range cannot be read"), "{e}");
        assert!(control.calls().is_empty(), "checks change nothing");
    }

    #[test]
    fn a_missing_gpu_is_a_reason_not_a_panic_and_opens_once() {
        let opened = std::sync::Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let count = opened.clone();
        let mut m = SystemMachine::new(
            Box::new(move || {
                count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                Err(GpuError::Unavailable("libnvidia-ml.so.1 not found".into()))
            }),
            None,
        );
        let e = m.read_prior(Step::ClockOffset).unwrap_err();
        assert!(e.contains("libnvidia-ml.so.1"), "{e}");
        assert!(m.read_prior(Step::Fan).is_err());
        assert!(
            m.apply(Step::PowerLimit, &SettingValue::Watts(200))
                .is_err()
        );
        assert_eq!(opened.load(std::sync::atomic::Ordering::SeqCst), 1);
    }

    #[test]
    fn gpu_steps_reach_the_setters_and_an_unknown_fan_goes_back_to_auto() {
        let control = FakeGpuControl::new();
        control.set_prior_unreadable(true);
        let mut m = machine(&control, None);
        assert!(m.read_prior(Step::PowerLimit).is_err());
        assert_eq!(
            m.read_prior(Step::Fan).unwrap(),
            Saved::Fans(vec![FanSetting::Auto])
        );
        m.apply(Step::LockedClocks, &SettingValue::DriverDefault)
            .unwrap();
        m.apply(Step::Fan, &SettingValue::Fan(FanSetting::Percent(60)))
            .unwrap();
        m.restore(Step::Fan, &Saved::Fans(vec![FanSetting::Auto]))
            .unwrap();
        assert_eq!(
            control.calls(),
            [
                GpuCall::ResetLockedClocks,
                GpuCall::FanSpeed(0, 60),
                GpuCall::FanAuto(0)
            ]
        );
    }

    #[test]
    fn each_fan_keeps_its_own_prior_and_a_failure_at_fan_1_undoes_fan_0() {
        let control = FakeGpuControl::new();
        control.set_fans(&[FanSetting::Auto, FanSetting::Percent(40), FanSetting::Auto]);
        let mut m = machine(&control, None);
        let saved = m.read_prior(Step::Fan).unwrap();
        assert_eq!(
            saved,
            Saved::Fans(vec![
                FanSetting::Auto,
                FanSetting::Percent(40),
                FanSetting::Auto
            ])
        );
        assert_eq!(saved.shown(), SettingValue::Fan(FanSetting::Auto));

        // Calls: fan 0 to 70 works, fan 1 fails, then fan 0 goes back.
        control.fail_at_call(Some(1));
        let e = m
            .apply(Step::Fan, &SettingValue::Fan(FanSetting::Percent(70)))
            .unwrap_err();
        assert!(e.starts_with("fan 1: "), "{e}");
        assert_eq!(
            control.calls(),
            [
                GpuCall::FanSpeed(0, 70),
                GpuCall::FanSpeed(1, 70),
                GpuCall::FanAuto(0)
            ]
        );
        assert_eq!(
            control.fans(),
            [FanSetting::Auto, FanSetting::Percent(40), FanSetting::Auto],
            "a failed step changed nothing"
        );

        // A restore tries every fan, even after one fails.
        control.fail_at_call(None);
        m.apply(Step::Fan, &SettingValue::Fan(FanSetting::Percent(70)))
            .unwrap();
        let calls = control.calls().len();
        control.fail_at_call(Some(calls));
        let e = m.restore(Step::Fan, &saved).unwrap_err();
        assert!(e.starts_with("fan 0: "), "{e}");
        assert_eq!(
            control.fans(),
            [
                FanSetting::Percent(70),
                FanSetting::Percent(40),
                FanSetting::Auto
            ]
        );
    }

    #[test]
    fn the_power_limit_prior_is_restored_in_exact_milliwatts() {
        let control = FakeGpuControl::new();
        // At the top of the card's range, which rounds up to 321 W.
        control.set_power_limit_now(320_500);
        let mut m = machine(&control, None);
        let saved = m.read_prior(Step::PowerLimit).unwrap();
        assert_eq!(saved, Saved::PowerLimitMw(320_500));
        assert_eq!(saved.shown(), SettingValue::Watts(321));
        m.apply(Step::PowerLimit, &SettingValue::Watts(250))
            .unwrap();
        m.restore(Step::PowerLimit, &saved).unwrap();
        assert_eq!(
            control.calls(),
            [
                GpuCall::PowerLimitMw(250_000),
                GpuCall::PowerLimitMw(320_500)
            ]
        );
        assert_eq!(control.power_limit_mw(), 320_500);
    }

    #[test]
    fn a_failed_governor_write_puts_back_the_cpus_it_wrote() {
        let sys = sysfs();
        let dir = sys.path().join(CPU_DIR);
        // cpu3's governor file becomes a folder: the write fails there.
        std::fs::remove_file(governor_file(&dir, 3)).unwrap();
        std::fs::create_dir(governor_file(&dir, 3)).unwrap();
        let m = machine(&FakeGpuControl::new(), Some(sys.path()));
        let values = vec![
            (0, "performance".to_owned()),
            (1, "performance".to_owned()),
            (3, "performance".to_owned()),
        ];
        let undo = vec![(0, "powersave".to_owned()), (1, "powersave".to_owned())];
        assert!(m.write_governors(&values, &undo).is_err());
        assert_eq!(governor(sys.path(), 0), "powersave");
        assert_eq!(governor(sys.path(), 1), "powersave");
    }
}
