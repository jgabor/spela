//! CPU state reads: the frequency governor, SMT and the `cpu info` lines.
//!
//! The architecture's seam for CPU sysfs and procfs is a root path,
//! [`SystemRoots`]. The read side also sits behind the [`Cpu`] trait, so
//! `--demo` and the binary tests, which run the real binary with
//! `SPELA_BACKEND=fake`, read the fake and never this machine. [`SysfsCpu`]
//! is the real reader over the roots; a test points them at a temp dir.
//! Writes happen only in the root helper ([`crate::helper`]).

use std::io;
use std::path::{Path, PathBuf};

/// The CPU folder under the sysfs root.
pub const CPU_DIR: &str = "devices/system/cpu";
/// cpu0's current governor, under [`CPU_DIR`].
pub const GOVERNOR_FILE: &str = "cpu0/cpufreq/scaling_governor";
/// cpu0's available governors, under [`CPU_DIR`].
pub const AVAILABLE_GOVERNORS_FILE: &str = "cpu0/cpufreq/scaling_available_governors";
/// Whether SMT is active, under [`CPU_DIR`].
pub const SMT_ACTIVE_FILE: &str = "smt/active";
/// The SMT switch the helper writes, under [`CPU_DIR`].
pub const SMT_CONTROL_FILE: &str = "smt/control";
/// The online CPUs as a range list such as `0-7,9`, under [`CPU_DIR`].
pub const ONLINE_FILE: &str = "online";

/// Where the kernel's file systems are: `/sys` and `/proc` on a real
/// system, folders under a temp dir in a test.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SystemRoots {
    /// The sysfs root, `/sys`.
    pub sys: PathBuf,
    /// The procfs root, `/proc`.
    pub proc: PathBuf,
}

impl SystemRoots {
    /// This machine's `/sys` and `/proc`.
    pub fn system() -> SystemRoots {
        SystemRoots {
            sys: PathBuf::from("/sys"),
            proc: PathBuf::from("/proc"),
        }
    }

    /// `sys` and `proc` folders under `dir`, for tests and the demo.
    pub fn under(dir: &Path) -> SystemRoots {
        SystemRoots {
            sys: dir.join("sys"),
            proc: dir.join("proc"),
        }
    }

    /// The CPU folder, `/sys/devices/system/cpu`.
    pub fn cpu_dir(&self) -> PathBuf {
        self.sys.join(CPU_DIR)
    }
}

/// What `cpu info` shows besides the governor and SMT, which have their own
/// reads. Each value can be missing on its own.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CpuInfo {
    /// The first `model name` in `/proc/cpuinfo`.
    pub model: Option<String>,
    /// Online CPUs. Go counted `runtime.NumCPU()`, which is the same unless
    /// the process runs with a narrower affinity.
    pub cores: usize,
    /// The average `scaling_cur_freq` of the online CPUs, in MHz.
    pub average_mhz: Option<u32>,
    /// The one-minute load average divided by the CPU count, as a percent
    /// capped at 100. This is a load figure, not CPU time: Go's comment
    /// that it came from `/proc/stat` was wrong.
    pub load_percent: Option<u32>,
    /// RAM in use (total minus available) and in total, in MiB.
    pub memory_mib: Option<(u64, u64)>,
    /// Whether `scx.service` is active, or `None` when `scx_loader` is not
    /// installed and the line is left out, as in Go.
    pub scx_active: Option<bool>,
}

/// Why a CPU read failed. Nothing panics.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("cannot read {path}: {reason}")]
pub struct CpuError {
    /// The file that could not be read.
    pub path: String,
    /// Why.
    pub reason: String,
}

/// CPU state reads. Every read can fail.
pub trait Cpu: Send {
    /// cpu0's current governor, such as `powersave`.
    fn governor(&mut self) -> Result<String, CpuError>;
    /// The governors cpu0 offers, in the order sysfs lists them.
    fn available_governors(&mut self) -> Result<Vec<String>, CpuError>;
    /// Whether SMT is active.
    fn smt_active(&mut self) -> Result<bool, CpuError>;
    /// The other `cpu info` lines. Fails only when `/proc/cpuinfo` cannot
    /// be read, as Go's did. Readers that only serve the TUI keep this
    /// default.
    fn info(&mut self) -> Result<CpuInfo, CpuError> {
        Err(CpuError {
            path: "/proc/cpuinfo".into(),
            reason: "this reader has no procfs".into(),
        })
    }
}

/// The real reader: files under a sysfs root, `/sys` on a real system, and
/// for [`Cpu::info`] a procfs root.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SysfsCpu {
    root: PathBuf,
    proc: Option<PathBuf>,
}

impl SysfsCpu {
    /// A reader over the sysfs tree at `root`, with no procfs: enough for
    /// the governor and SMT, which is all the TUI asks.
    pub fn new(root: impl Into<PathBuf>) -> SysfsCpu {
        SysfsCpu {
            root: root.into(),
            proc: None,
        }
    }

    /// A reader over both roots.
    pub fn with_roots(roots: &SystemRoots) -> SysfsCpu {
        SysfsCpu {
            root: roots.sys.clone(),
            proc: Some(roots.proc.clone()),
        }
    }

    /// The reader over this machine's `/sys` and `/proc`.
    pub fn system() -> SysfsCpu {
        SysfsCpu::with_roots(&SystemRoots::system())
    }

    fn read(&self, file: &str) -> Result<String, CpuError> {
        let path = self.root.join(CPU_DIR).join(file);
        read_trimmed(&path).map_err(|e| CpuError {
            path: path.display().to_string(),
            reason: e.to_string(),
        })
    }

    fn read_proc(&self, file: &str) -> Result<String, CpuError> {
        let Some(proc) = &self.proc else {
            return Err(CpuError {
                path: format!("/proc/{file}"),
                reason: "this reader has no procfs root".into(),
            });
        };
        let path = proc.join(file);
        std::fs::read_to_string(&path).map_err(|e| CpuError {
            path: path.display().to_string(),
            reason: e.to_string(),
        })
    }
}

fn read_trimmed(path: &Path) -> io::Result<String> {
    std::fs::read_to_string(path).map(|text| text.trim().to_owned())
}

impl Cpu for SysfsCpu {
    fn governor(&mut self) -> Result<String, CpuError> {
        self.read(GOVERNOR_FILE)
    }

    fn available_governors(&mut self) -> Result<Vec<String>, CpuError> {
        Ok(parse_governors(&self.read(AVAILABLE_GOVERNORS_FILE)?))
    }

    fn smt_active(&mut self) -> Result<bool, CpuError> {
        Ok(parse_smt_active(&self.read(SMT_ACTIVE_FILE)?))
    }

    fn info(&mut self) -> Result<CpuInfo, CpuError> {
        let model = parse_model(&self.read_proc("cpuinfo")?);
        let cpu_dir = self.root.join(CPU_DIR);
        let cpus = online_cpus(&cpu_dir);
        let cores = cpus.len();
        let khz: Vec<u64> = cpus
            .iter()
            .filter_map(|n| {
                let file = cpu_dir.join(format!("cpu{n}/cpufreq/scaling_cur_freq"));
                read_trimmed(&file).ok()?.parse().ok()
            })
            .collect();
        // Go divided each CPU's kHz by 1000 before it averaged them.
        let average_mhz = (!khz.is_empty())
            .then(|| khz.iter().map(|k| k / 1000).sum::<u64>() / khz.len() as u64)
            .and_then(|m| u32::try_from(m).ok());
        let load_percent = self
            .read_proc("loadavg")
            .ok()
            .and_then(|t| load_percent(&t, cores));
        let memory_mib = self
            .read_proc("meminfo")
            .ok()
            .and_then(|t| crate::metrics::parse_meminfo(&t));
        Ok(CpuInfo {
            model,
            cores,
            average_mhz,
            load_percent,
            memory_mib,
            scx_active: None,
        })
    }
}

/// The online CPUs under a CPU folder, by number. They are listed from the
/// `cpuN` folders, so offline and non-contiguous CPUs are seen. A CPU is
/// online when its `online` file says `1` or it has none, as cpu0 often
/// does not. Go looped over 0 to `NumCPU - 1` instead.
pub fn online_cpus(cpu_dir: &Path) -> Vec<u32> {
    let Ok(entries) = std::fs::read_dir(cpu_dir) else {
        return Vec::new();
    };
    let mut cpus: Vec<u32> = entries
        .flatten()
        .filter_map(|e| {
            let name = e.file_name();
            let n: u32 = name.to_str()?.strip_prefix("cpu")?.parse().ok()?;
            let online = match read_trimmed(&e.path().join("online")) {
                Ok(text) => text == "1",
                Err(_) => true,
            };
            (online && e.path().is_dir()).then_some(n)
        })
        .collect();
    cpus.sort_unstable();
    cpus
}

/// The first `model name` of a `/proc/cpuinfo` text.
pub fn parse_model(cpuinfo: &str) -> Option<String> {
    cpuinfo
        .lines()
        .filter(|l| l.starts_with("model name"))
        .find_map(|l| l.split_once(':').map(|(_, v)| v.trim().to_owned()))
}

/// The one-minute load of a `/proc/loadavg` text over `cores`, as a percent
/// capped at 100 and rounded to the nearest whole, as Go's `%.0f` printed
/// it.
pub fn load_percent(loadavg: &str, cores: usize) -> Option<u32> {
    let load1: f64 = loadavg.split_whitespace().next()?.parse().ok()?;
    if cores == 0 || !load1.is_finite() || load1 < 0.0 {
        return None;
    }
    let percent = (load1 / cores as f64 * 100.0).min(100.0);
    Some(percent.round() as u32)
}

/// The governors in a `scaling_available_governors` text, split on spaces.
pub fn parse_governors(text: &str) -> Vec<String> {
    text.split_whitespace().map(str::to_owned).collect()
}

/// Whether an `smt/active` text says SMT is on: `1`, as Go reads it.
pub fn parse_smt_active(text: &str) -> bool {
    text.trim() == "1"
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tree(governor: &str, available: &str, smt: &str) -> tempfile::TempDir {
        let dir = tempfile::tempdir().unwrap();
        let cpu = dir.path().join(CPU_DIR);
        std::fs::create_dir_all(cpu.join("cpu0/cpufreq")).unwrap();
        std::fs::create_dir_all(cpu.join("smt")).unwrap();
        std::fs::write(cpu.join(GOVERNOR_FILE), governor).unwrap();
        std::fs::write(cpu.join(AVAILABLE_GOVERNORS_FILE), available).unwrap();
        std::fs::write(cpu.join(SMT_ACTIVE_FILE), smt).unwrap();
        dir
    }

    #[test]
    fn reads_the_governor_the_list_and_smt() {
        let dir = tree("powersave\n", "performance powersave \n", "1\n");
        let mut cpu = SysfsCpu::new(dir.path());
        assert_eq!(cpu.governor().unwrap(), "powersave");
        assert_eq!(
            cpu.available_governors().unwrap(),
            ["performance", "powersave"]
        );
        assert!(cpu.smt_active().unwrap());
        let dir = tree("performance", "performance", "0");
        assert!(!SysfsCpu::new(dir.path()).smt_active().unwrap());
    }

    #[test]
    fn a_missing_file_names_its_path() {
        let dir = tempfile::tempdir().unwrap();
        let e = SysfsCpu::new(dir.path()).governor().unwrap_err();
        assert!(e.path.ends_with("cpu0/cpufreq/scaling_governor"), "{e}");
        assert!(e.to_string().starts_with("cannot read "), "{e}");
    }

    #[test]
    fn info_reads_procfs_and_the_online_cpus() {
        let dir = tempfile::tempdir().unwrap();
        let roots = SystemRoots::under(dir.path());
        let cpu = roots.cpu_dir();
        for (n, khz, online) in [
            (0, "3000500", None),
            (1, "4100999", Some("1")),
            (2, "800000", Some("0")),
        ] {
            let freq = cpu.join(format!("cpu{n}/cpufreq"));
            std::fs::create_dir_all(&freq).unwrap();
            std::fs::write(freq.join("scaling_cur_freq"), khz).unwrap();
            if let Some(o) = online {
                std::fs::write(cpu.join(format!("cpu{n}/online")), o).unwrap();
            }
        }
        std::fs::create_dir_all(cpu.join("cpufreq")).unwrap();
        std::fs::create_dir_all(&roots.proc).unwrap();
        std::fs::write(
            roots.proc.join("cpuinfo"),
            "processor\t: 0\nmodel name\t: Fictional 9000 CPU\nprocessor\t: 1\nmodel name\t: other\n",
        )
        .unwrap();
        std::fs::write(roots.proc.join("loadavg"), "1.50 0.80 0.40 2/900 1234\n").unwrap();
        std::fs::write(
            roots.proc.join("meminfo"),
            "MemTotal: 2048000 kB\nMemAvailable: 1024000 kB\n",
        )
        .unwrap();
        let info = SysfsCpu::with_roots(&roots).info().unwrap();
        assert_eq!(
            info,
            CpuInfo {
                model: Some("Fictional 9000 CPU".into()),
                cores: 2,
                // 3000 and 4100 MHz: kHz to MHz per CPU, then the mean.
                average_mhz: Some(3550),
                load_percent: Some(75),
                memory_mib: Some((1000, 2000)),
                scx_active: None,
            }
        );
        assert_eq!(online_cpus(&cpu), [0, 1]);
        // No procfs root, no info; the sysfs reads still work.
        let e = SysfsCpu::new(&roots.sys).info().unwrap_err();
        assert!(e.path.ends_with("cpuinfo"), "{e}");
    }

    #[test]
    fn load_is_capped_and_needs_cores() {
        assert_eq!(load_percent("64.0 1 1", 16), Some(100));
        assert_eq!(load_percent("0.04 1 1", 16), Some(0));
        assert_eq!(load_percent("1.0", 0), None);
        assert_eq!(load_percent("x", 4), None);
        assert_eq!(parse_model("processor: 0\n"), None);
    }
}
