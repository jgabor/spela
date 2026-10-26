//! CPU state reads: the frequency governor and SMT.
//!
//! The architecture's seam for CPU sysfs is a root path. The read side also
//! sits behind the [`Cpu`] trait, so `--demo` and the binary tests, which
//! run the real binary with `SPELA_BACKEND=fake`, read the fake and never
//! this machine's sysfs. [`SysfsCpu`] is the real reader over a root path;
//! a test points it at a temp dir. Writes happen only in the root helper.

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
}

/// The real reader: files under a sysfs root, `/sys` on a real system.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SysfsCpu {
    root: PathBuf,
}

impl SysfsCpu {
    /// A reader over the sysfs tree at `root`.
    pub fn new(root: impl Into<PathBuf>) -> SysfsCpu {
        SysfsCpu { root: root.into() }
    }

    /// The reader over this machine's `/sys`.
    pub fn system() -> SysfsCpu {
        SysfsCpu::new("/sys")
    }

    fn read(&self, file: &str) -> Result<String, CpuError> {
        let path = self.root.join(CPU_DIR).join(file);
        read_trimmed(&path).map_err(|e| CpuError {
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
}
