//! The metrics thread: it owns the GPU backend and polls GPU 0 and the
//! CPU, one poll at a time.
//!
//! nvml-wrapper's `Device` borrows `Nvml`, so one owner is simpler than
//! sharing (architecture.md, TUI process). The UI asks for a poll on each
//! 2 s tick and never while one is in flight; the thread also drops extra
//! requests that queued up. Each sweep runs the alert evaluator and sends
//! the alerts with the metrics. NVML never runs on the UI thread.

use std::path::PathBuf;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::mpsc;

use spela_core::alerts::{self, AlertInput, Thresholds};
use spela_core::backend::{Gpu, GpuError, GpuInfo, GpuMetrics};
use spela_core::metrics::{CpuMetrics, CpuTimes, parse_cpuinfo_mhz, parse_meminfo};
use spela_ui::Msg;
use spela_ui::event::MetricsSample;

use super::events::LoopSender;

/// CPU and memory reads for the metrics thread.
pub trait CpuLoad: Send {
    /// One sample.
    fn sample(&mut self) -> Result<CpuMetrics, String>;
}

/// The real reads: `stat`, `cpuinfo` and `meminfo` under a procfs root,
/// `/proc` outside tests.
#[derive(Debug)]
pub struct ProcCpu {
    root: PathBuf,
    prev: Option<CpuTimes>,
}

impl ProcCpu {
    /// Reads under `root`.
    pub fn new(root: impl Into<PathBuf>) -> ProcCpu {
        ProcCpu {
            root: root.into(),
            prev: None,
        }
    }
}

impl CpuLoad for ProcCpu {
    fn sample(&mut self) -> Result<CpuMetrics, String> {
        let read = |name: &str| std::fs::read_to_string(self.root.join(name));
        let stat =
            read("stat").map_err(|e| format!("cannot read {}/stat: {e}", self.root.display()))?;
        let times = CpuTimes::parse(&stat).ok_or_else(|| "no cpu line in stat".to_owned())?;
        let utilization_percent = self.prev.and_then(|p| times.utilization_since(&p));
        self.prev = Some(times);
        let mhz = read("cpuinfo").ok().and_then(|t| parse_cpuinfo_mhz(&t));
        let mem = read("meminfo").ok().and_then(|t| parse_meminfo(&t));
        Ok(CpuMetrics {
            utilization_percent,
            mhz,
            memory_used_mib: mem.map(|m| m.0),
            memory_total_mib: mem.map(|m| m.1),
        })
    }
}

/// Scripted CPU reads for `--demo` and `SPELA_BACKEND=fake`, so a test or
/// a demo never shows this machine's numbers.
#[derive(Debug, Default)]
pub struct FakeCpuLoad {
    n: u32,
}

impl CpuLoad for FakeCpuLoad {
    fn sample(&mut self) -> Result<CpuMetrics, String> {
        const LOAD: [u32; 6] = [8, 12, 34, 61, 47, 19];
        let i = (self.n as usize) % LOAD.len();
        self.n = self.n.wrapping_add(1);
        Ok(CpuMetrics {
            utilization_percent: Some(LOAD[i]),
            mhz: Some(3_400 + LOAD[i] * 15),
            memory_used_mib: Some(9_800 + u64::from(LOAD[i]) * 40),
            memory_total_mib: Some(32_000),
        })
    }
}

/// A GPU that is not there, left behind when the metrics thread takes the
/// real one.
#[derive(Debug)]
pub struct NoGpu;

impl Gpu for NoGpu {
    fn info(&mut self) -> Result<GpuInfo, GpuError> {
        Err(GpuError::Unavailable(
            "the metrics thread owns the GPU".into(),
        ))
    }

    fn metrics(&mut self) -> Result<GpuMetrics, GpuError> {
        self.info().map(|_| unreachable!())
    }
}

/// One sweep: GPU 0, the CPU and the alerts.
pub fn sweep(gpu: &mut dyn Gpu, cpu: &mut dyn CpuLoad) -> MetricsSample {
    let gpu = gpu.metrics().map_err(|e| e.to_string());
    let alerts = match &gpu {
        Ok(m) => alerts::evaluate(
            &AlertInput {
                temperature_c: m.temperature_c,
                power_draw_w: f64::from(m.power_draw_w),
                power_limit_w: 0.0,
                fan_percent: m.fan_percent.unwrap_or(0),
                throttle_thermal: false,
                throttle_power: false,
            },
            &Thresholds::default(),
        ),
        Err(_) => Vec::new(),
    };
    MetricsSample {
        gpu,
        cpu: cpu.sample(),
        alerts,
    }
}

/// The running metrics thread. It lives until the process exits.
#[derive(Debug)]
pub struct MetricsThread {
    tx: mpsc::Sender<()>,
    polls: Arc<AtomicUsize>,
}

impl MetricsThread {
    /// Starts the thread with the backends it owns from now on.
    pub fn spawn(
        mut gpu: Box<dyn Gpu>,
        mut cpu: Box<dyn CpuLoad>,
        loop_tx: LoopSender<Msg>,
    ) -> std::io::Result<MetricsThread> {
        let (tx, rx) = mpsc::channel::<()>();
        let polls = Arc::new(AtomicUsize::new(0));
        let counter = polls.clone();
        std::thread::Builder::new()
            .name("spela-metrics".into())
            .spawn(move || {
                while rx.recv().is_ok() {
                    // At most one poll in flight: extra asks are one ask.
                    while rx.try_recv().is_ok() {}
                    counter.fetch_add(1, Ordering::SeqCst);
                    let sample = sweep(gpu.as_mut(), cpu.as_mut());
                    if loop_tx.done(Msg::Metrics(Box::new(sample))).is_err() {
                        return;
                    }
                }
            })?;
        Ok(MetricsThread { tx, polls })
    }

    /// Asks for one sweep.
    pub fn poll(&self) {
        let _ = self.tx.send(());
    }

    /// Sweeps run so far.
    pub fn polls(&self) -> usize {
        self.polls.load(Ordering::SeqCst)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::fake::FakeGpu;

    #[test]
    fn proc_reads_need_two_samples_for_use() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(dir.path().join("stat"), "cpu  100 0 100 700 100 0 0 0\n").unwrap();
        std::fs::write(dir.path().join("cpuinfo"), "cpu MHz\t: 3000.0\n").unwrap();
        std::fs::write(
            dir.path().join("meminfo"),
            "MemTotal: 2048000 kB\nMemAvailable: 1024000 kB\n",
        )
        .unwrap();
        let mut cpu = ProcCpu::new(dir.path());
        let first = cpu.sample().unwrap();
        assert_eq!(first.utilization_percent, None);
        assert_eq!(first.mhz, Some(3000));
        assert_eq!(first.memory_total_mib, Some(2000));
        std::fs::write(dir.path().join("stat"), "cpu  200 0 200 1300 100 0 0 0\n").unwrap();
        assert_eq!(cpu.sample().unwrap().utilization_percent, Some(25));
        assert!(ProcCpu::new(dir.path().join("none")).sample().is_err());
    }

    #[test]
    fn a_sweep_runs_the_evaluator() {
        let mut gpu = FakeGpu::demo();
        let mut cpu = FakeCpuLoad::default();
        let samples: Vec<MetricsSample> = (0..6).map(|_| sweep(&mut gpu, &mut cpu)).collect();
        assert!(samples.iter().all(|s| s.gpu.is_ok() && s.cpu.is_ok()));
        assert!(
            samples.iter().any(|s| !s.alerts.is_empty()),
            "the demo walks into alerts"
        );
        assert!(samples[0].alerts.is_empty());
    }
}
