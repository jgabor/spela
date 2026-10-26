//! CPU and memory metrics from procfs, as pure parsers.
//!
//! The TUI's metrics thread reads `/proc/stat`, `/proc/cpuinfo` and
//! `/proc/meminfo` under an injected root and passes the text here. Nothing
//! in this module touches a file, so tests feed it fixture text. The fields
//! are the ones the Go header shows: CPU use, the average clock and RAM.

/// One CPU sample, as the TUI's top bar and Monitor show it. Every field
/// can be missing on its own, because each comes from a different file.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CpuMetrics {
    /// Use of all CPUs since the previous sample, in percent. `None` on the
    /// first sample, which has nothing to compare with.
    pub utilization_percent: Option<u32>,
    /// The average current clock over all CPUs, in MHz.
    pub mhz: Option<u32>,
    /// RAM in use, in MiB: total minus available.
    pub memory_used_mib: Option<u64>,
    /// Total RAM, in MiB.
    pub memory_total_mib: Option<u64>,
}

/// The `cpu` line of `/proc/stat`: busy and total jiffies since boot.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct CpuTimes {
    /// Jiffies spent doing work.
    pub busy: u64,
    /// All jiffies, idle and iowait included.
    pub total: u64,
}

impl CpuTimes {
    /// Reads the first `cpu ` line of a `/proc/stat` text. Idle and iowait
    /// (the 4th and 5th numbers) count as not busy; guest time is already
    /// part of user time, so it is left out.
    pub fn parse(stat: &str) -> Option<CpuTimes> {
        let line = stat.lines().find(|l| l.starts_with("cpu "))?;
        let numbers: Vec<u64> = line
            .split_whitespace()
            .skip(1)
            .take(8)
            .map(str::parse)
            .collect::<Result<_, _>>()
            .ok()?;
        if numbers.len() < 4 {
            return None;
        }
        let total: u64 = numbers.iter().sum();
        let idle = numbers[3] + numbers.get(4).copied().unwrap_or(0);
        Some(CpuTimes {
            busy: total.saturating_sub(idle),
            total,
        })
    }

    /// Percent busy between an earlier sample and this one, or `None` when
    /// no time passed or the counters went backwards.
    pub fn utilization_since(&self, earlier: &CpuTimes) -> Option<u32> {
        let total = self.total.checked_sub(earlier.total)?;
        let busy = self.busy.checked_sub(earlier.busy)?;
        if total == 0 {
            return None;
        }
        let percent = (busy * 100 + total / 2) / total;
        u32::try_from(percent.min(100)).ok()
    }
}

/// The average of the `cpu MHz` lines of a `/proc/cpuinfo` text.
pub fn parse_cpuinfo_mhz(cpuinfo: &str) -> Option<u32> {
    let values: Vec<f64> = cpuinfo
        .lines()
        .filter(|l| l.starts_with("cpu MHz"))
        .filter_map(|l| l.split_once(':')?.1.trim().parse().ok())
        .collect();
    if values.is_empty() {
        return None;
    }
    let mean = values.iter().sum::<f64>() / values.len() as f64;
    // A clock is never negative or anywhere near u32::MAX MHz.
    Some(mean.round().clamp(0.0, f64::from(u32::MAX)) as u32)
}

/// RAM in use and in total, in MiB, from a `/proc/meminfo` text: total
/// minus `MemAvailable`.
pub fn parse_meminfo(meminfo: &str) -> Option<(u64, u64)> {
    let field = |name: &str| -> Option<u64> {
        let line = meminfo.lines().find(|l| l.starts_with(name))?;
        line.split_whitespace().nth(1)?.parse().ok()
    };
    let total_kib = field("MemTotal:")?;
    let available_kib = field("MemAvailable:")?;
    let used = total_kib.saturating_sub(available_kib);
    Some((used / 1024, total_kib / 1024))
}

#[cfg(test)]
mod tests {
    use super::*;

    const STAT_A: &str = "cpu  100 0 100 700 100 0 0 0 0 0\ncpu0 50 0 50 350 50 0 0 0 0 0\n";
    const STAT_B: &str = "cpu  200 0 200 1300 100 0 0 0 0 0\n";

    #[test]
    fn stat_lines_give_busy_and_total() {
        let a = CpuTimes::parse(STAT_A).unwrap();
        assert_eq!(
            a,
            CpuTimes {
                busy: 200,
                total: 1000
            }
        );
        let b = CpuTimes::parse(STAT_B).unwrap();
        assert_eq!(b.utilization_since(&a), Some(25));
        assert_eq!(a.utilization_since(&b), None, "counters went backwards");
        assert_eq!(a.utilization_since(&a), None, "no time passed");
        assert_eq!(CpuTimes::parse("intr 1 2 3\n"), None);
        assert_eq!(CpuTimes::parse("cpu 1 x 3 4\n"), None);
    }

    #[test]
    fn cpuinfo_averages_every_cpu() {
        let text = "processor : 0\ncpu MHz\t\t: 3000.000\nprocessor : 1\ncpu MHz\t\t: 4001.000\n";
        assert_eq!(parse_cpuinfo_mhz(text), Some(3501));
        assert_eq!(parse_cpuinfo_mhz("processor : 0\n"), None);
    }

    #[test]
    fn meminfo_counts_available_as_free() {
        let text = "MemTotal:       32768000 kB\nMemFree:  1000 kB\nMemAvailable:   16384000 kB\n";
        assert_eq!(parse_meminfo(text), Some((16000, 32000)));
        assert_eq!(parse_meminfo("MemTotal: 10 kB\n"), None);
    }
}
