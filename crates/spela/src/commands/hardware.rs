//! The live hardware commands: `gpu info` and `cpu info`, which read, and
//! `gpu reset`, `cpu governor <g>` and `cpu smt <on|off>`, which change the
//! machine now.
//!
//! The changes go through one `pkexec` and the one-shot helper
//! (`apply-profile --once`), never `pkexec tee` per CPU or `pkexec
//! nvidia-smi` as in Go (parity-inventory.md, CLI 3.9 and 3.10). They take
//! the launch lock without waiting, so they never run into a game's
//! restore; a held lock exits 5. A live change is meant to last, so
//! nothing restores it.
//!
//! The lines and messages are Go's, with these fixes: `cpu smt` takes only
//! the bool words, so a typo is a usage error instead of turning SMT off;
//! `gpu reset` also sets both clock offsets to 0 (decision 32); and the
//! errors no longer guess "(may need root)".

use std::io::Write;

use spela_core::backend::{ApplyReport, HardwareRequest, StepOutcome};
use spela_core::profile::value::parse_bool;

use super::read;
use crate::cli::Context;
use crate::error::{CliError, exit};
use crate::text::Paint;

/// Runs `spela gpu info`.
pub fn gpu_info(ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let gpu = &mut ctx.backends.gpu;
    let info = gpu
        .info()
        .map_err(|e| CliError::new(exit::FAILURE, format!("failed to get GPU info: {e}")))?;
    let metrics = gpu.metrics();
    if let Err(e) = &metrics {
        tracing::info!("no GPU metrics for gpu info: {e}");
    }
    let metrics = metrics.ok();
    let p = Paint::new(ctx.colors.out);
    let line = |label: &str, value: String| format!("{}  {value}\n", p.dim(label));
    let mut text = String::new();
    text.push_str(&line("GPU:", p.primary(&info.name)));
    text.push_str(&line("Driver:", info.driver_version.clone()));
    text.push_str(&line(
        "VRAM:",
        p.accent(&format!("{} MB", info.memory_total_mib)),
    ));
    let temp = metrics
        .as_ref()
        .map(|m| format!("{}°C", m.temperature_c))
        .unwrap_or_default();
    text.push_str(&line("Temp:", p.accent(&temp)));
    if let Some(mw) = info.power_draw_mw {
        let mut power = p.accent(&format!("{:.1} W", f64::from(mw) / 1000.0));
        if let Some(limit) = metrics.as_ref().and_then(|m| m.power_limit_w) {
            power.push_str(&format!(" / {limit} W limit"));
        }
        if let Some((min, max)) = info.power_limit_range_w {
            power.push_str(&format!(" ({min}–{max} W range)"));
        }
        text.push_str(&line("Power:", power));
    }
    if let Some(m) = &metrics {
        text.push_str(&line(
            "Core:",
            p.accent(&format!("{} MHz", m.graphics_clock_mhz)),
        ));
        text.push_str(&line(
            "Mem:",
            p.accent(&format!("{} MHz", m.memory_clock_mhz)),
        ));
        if let Some(fan) = m.fan_percent {
            text.push_str(&line("Fan:", p.accent(&format!("{fan}%"))));
        }
        text.push_str(&line(
            "Load:",
            p.accent(&format!("{}%", m.utilization_percent)),
        ));
    }
    read::write_out(out, &text)
}

/// Runs `spela cpu info`.
pub fn cpu_info(ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let cpu = &mut ctx.backends.cpu;
    let info = cpu
        .info()
        .map_err(|e| CliError::new(exit::FAILURE, e.to_string()))?;
    let p = Paint::new(ctx.colors.out);
    let line = |label: &str, value: String| format!("{}  {value}\n", p.dim(label));
    let mut text = String::new();
    text.push_str(&line(
        "CPU:",
        p.primary(info.model.as_deref().unwrap_or("")),
    ));
    text.push_str(&line("Cores:", p.accent(&info.cores.to_string())));
    let mut governor = p.accent(&cpu.governor().unwrap_or_default());
    if let Ok(available) = cpu.available_governors()
        && !available.is_empty()
    {
        governor.push_str(&format!(" ({} available)", available.join(", ")));
    }
    text.push_str(&line("Governor:", governor));
    let smt = match cpu.smt_active() {
        Ok(true) => "enabled",
        Ok(false) => "disabled",
        Err(_) => "",
    };
    text.push_str(&line("SMT:", p.accent(smt)));
    if let Some(mhz) = info.average_mhz.filter(|m| *m > 0) {
        text.push_str(&line("Freq:", p.accent(&format!("{mhz} MHz avg"))));
    }
    if let Some(load) = info.load_percent.filter(|l| *l > 0) {
        text.push_str(&line("Load:", p.accent(&format!("{load}%"))));
    }
    if let Some((used, total)) = info.memory_mib.filter(|(_, t)| *t > 0) {
        text.push_str(&line("RAM:", p.accent(&format!("{used} / {total} MB"))));
    }
    if let Some(active) = info.scx_active {
        let status = if active { "active" } else { "inactive" };
        text.push_str(&line("SCX:", p.accent(status)));
    }
    read::write_out(out, &text)
}

/// Runs `spela gpu reset`: locked clocks back to the driver, both offsets
/// to 0.
pub fn gpu_reset(ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    apply(ctx, &HardwareRequest::gpu_reset(), "failed to reset clocks")?;
    let p = Paint::new(ctx.colors.out);
    read::write_out(
        out,
        &format!("{}\n", p.success("GPU clocks reset to default")),
    )
}

/// Runs `spela cpu governor <governor>`. A governor sysfs does not list is
/// refused before the prompt, with the list.
pub fn set_governor(
    ctx: &mut Context,
    governor: &str,
    out: &mut dyn Write,
) -> Result<(), CliError> {
    const FAILED: &str = "failed to set governor";
    if let Ok(available) = ctx.backends.cpu.available_governors()
        && !available.iter().any(|g| g == governor)
    {
        let list = if available.is_empty() {
            "none".to_owned()
        } else {
            available.join(", ")
        };
        return Err(CliError::new(
            exit::FAILURE,
            format!("{FAILED}: CPU governor {governor:?} is not available (available: {list})"),
        ));
    }
    let request = HardwareRequest {
        cpu_governor: Some(governor.to_owned()),
        ..HardwareRequest::default()
    };
    apply(ctx, &request, FAILED)?;
    read::write_out(out, &format!("Governor set to {governor}\n"))
}

/// Runs `spela cpu smt <value>`. Only the bool words count: anything else
/// is a usage error, where Go turned SMT off.
pub fn set_smt(ctx: &mut Context, value: &str, out: &mut dyn Write) -> Result<(), CliError> {
    let Some(on) = parse_bool(value) else {
        return Err(CliError::new(
            exit::USAGE,
            format!("invalid SMT value: {value} (use on or off)"),
        ));
    };
    let request = HardwareRequest {
        smt: Some(on),
        ..HardwareRequest::default()
    };
    apply(ctx, &request, "failed to set SMT")?;
    read::write_out(
        out,
        if on {
            "SMT enabled\n"
        } else {
            "SMT disabled\n"
        },
    )
}

/// Takes the launch lock without waiting, runs the one-shot helper, and
/// fails with `context` and the reason when a step did not apply.
fn apply(ctx: &mut Context, request: &HardwareRequest, context: &str) -> Result<(), CliError> {
    let lock = crate::locks::launch(&ctx.paths).map_err(CliError::from)?;
    tracing::info!(lock = %lock.path().display(), "took the launch lock");
    let report = ctx
        .backends
        .privilege
        .apply_once(request)
        .map_err(|e| CliError::new(exit::FAILURE, format!("{context}: {e}")))?;
    drop(lock);
    match not_applied(&report) {
        None => Ok(()),
        Some(why) => Err(CliError::new(exit::FAILURE, format!("{context}: {why}"))),
    }
}

/// Every step that did not apply, with its reason, or `None` when all did.
fn not_applied(report: &ApplyReport) -> Option<String> {
    let reasons: Vec<String> = report
        .steps
        .iter()
        .filter_map(|s| match &s.outcome {
            StepOutcome::Failed(why) | StepOutcome::Skipped(why) => {
                Some(format!("{}: {why}", s.step))
            }
            StepOutcome::Applied { .. } | StepOutcome::AppliedNoRestore => None,
        })
        .collect();
    (!reasons.is_empty()).then(|| reasons.join("; "))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    use spela_core::backend::Backends;
    use spela_core::env::Env;
    use spela_core::fake::{FakeGpuControl, FakePrivilege, GpuCall, PrivilegeFault};
    use spela_core::migrate::Report;
    use spela_core::paths::Paths;

    use crate::logging::LevelHandle;
    use crate::output::Colors;

    struct Setup {
        _dir: tempfile::TempDir,
        ctx: Context,
        sys: std::path::PathBuf,
        control: FakeGpuControl,
        privilege: FakePrivilege,
    }

    /// A context over temp dirs, the fake backends, a demo sysfs tree and
    /// a fake privilege runner that writes it.
    fn setup() -> Setup {
        let dir = tempfile::tempdir().unwrap();
        let sys = dir.path().join("sys");
        spela_core::fake::cpu::write_sysfs(&sys).unwrap();
        let control = FakeGpuControl::new();
        let privilege = FakePrivilege::new(control.clone(), Some(sys.clone()));
        let mut backends: Backends = spela_core::fake::backends();
        backends.privilege = Box::new(privilege.clone());
        backends.cpu = Box::new(spela_core::backend::SysfsCpu::new(&sys));
        let ctx = Context {
            env: Env::default(),
            paths: Paths::under(&dir.path().join("xdg")),
            backends,
            colors: Colors::default(),
            clock: Arc::new(spela_core::clock::SystemClock),
            migration: Report::default(),
            demo: false,
            log: LevelHandle::default(),
        };
        Setup {
            _dir: dir,
            ctx,
            sys,
            control,
            privilege,
        }
    }

    fn text(out: Vec<u8>) -> String {
        String::from_utf8(out).unwrap()
    }

    fn read_sys(sys: &std::path::Path, file: &str) -> String {
        std::fs::read_to_string(sys.join("devices/system/cpu").join(file))
            .unwrap()
            .trim()
            .to_owned()
    }

    #[test]
    fn gpu_info_prints_go_lines() {
        let mut s = setup();
        let mut out = Vec::new();
        gpu_info(&mut s.ctx, &mut out).unwrap();
        assert_eq!(
            text(out),
            "GPU:  Demo GPU 4080\n\
             Driver:  580.95.05\n\
             VRAM:  16376 MB\n\
             Temp:  41°C\n\
             Power:  22.4 W / 320 W limit (100–360 W range)\n\
             Core:  285 MHz\n\
             Mem:  10501 MHz\n\
             Fan:  0%\n\
             Load:  3%\n"
        );
        let gpu = spela_core::fake::FakeGpu::demo();
        gpu.set_fault(Some(spela_core::fake::GpuFault::Missing));
        s.ctx.backends.gpu = Box::new(gpu);
        let e = gpu_info(&mut s.ctx, &mut Vec::new()).unwrap_err();
        assert_eq!(e.code, exit::FAILURE);
        assert!(
            e.message
                .starts_with("failed to get GPU info: NVIDIA management library not available"),
            "{}",
            e.message
        );
    }

    #[test]
    fn cpu_info_prints_go_lines() {
        let mut s = setup();
        s.ctx.backends.cpu = Box::new(spela_core::fake::FakeCpu::demo());
        let mut out = Vec::new();
        cpu_info(&mut s.ctx, &mut out).unwrap();
        assert_eq!(
            text(out),
            "CPU:  Demo CPU 8-Core Processor\n\
             Cores:  16\n\
             Governor:  powersave (performance, powersave available)\n\
             SMT:  enabled\n\
             Freq:  3420 MHz avg\n\
             Load:  12%\n\
             RAM:  9800 / 32000 MB\n"
        );
    }

    #[test]
    fn the_live_commands_go_through_one_prompt_each() {
        let mut s = setup();
        let mut out = Vec::new();
        set_governor(&mut s.ctx, "performance", &mut out).unwrap();
        set_smt(&mut s.ctx, "off", &mut out).unwrap();
        gpu_reset(&mut s.ctx, &mut out).unwrap();
        assert_eq!(
            text(out),
            "Governor set to performance\nSMT disabled\nGPU clocks reset to default\n"
        );
        for n in 0..4 {
            assert_eq!(
                read_sys(&s.sys, &format!("cpu{n}/cpufreq/scaling_governor")),
                "performance"
            );
        }
        assert_eq!(read_sys(&s.sys, "smt/control"), "off");
        assert_eq!(
            s.control.calls(),
            [
                GpuCall::ResetLockedClocks,
                GpuCall::ClockOffset(0),
                GpuCall::MemoryOffset(0)
            ]
        );
        let log = s.privilege.log();
        assert_eq!((log.prompts, log.restores, log.restores_on_drop), (3, 0, 0));
    }

    #[test]
    fn a_typo_never_turns_smt_off() {
        let mut s = setup();
        for typo in ["of", "disable", ""] {
            let e = set_smt(&mut s.ctx, typo, &mut Vec::new()).unwrap_err();
            assert_eq!(e.code, exit::USAGE, "{typo:?}");
            assert_eq!(
                e.message,
                format!("invalid SMT value: {typo} (use on or off)")
            );
        }
        assert_eq!(read_sys(&s.sys, "smt/control"), "on");
        assert_eq!(s.privilege.log().prompts, 0);
        set_smt(&mut s.ctx, "ON", &mut Vec::new()).unwrap();
        assert_eq!(read_sys(&s.sys, "smt/control"), "on");
    }

    #[test]
    fn an_unlisted_governor_is_refused_before_the_prompt() {
        let mut s = setup();
        let e = set_governor(&mut s.ctx, "turbo", &mut Vec::new()).unwrap_err();
        assert_eq!(
            e.message,
            "failed to set governor: CPU governor \"turbo\" is not available (available: \
             conservative, ondemand, userspace, powersave, performance, schedutil)"
        );
        assert_eq!(s.privilege.log().prompts, 0);
    }

    #[test]
    fn a_held_launch_lock_exits_5_and_asks_nothing() {
        let mut s = setup();
        let held = crate::locks::launch(&s.ctx.paths).unwrap();
        let e = gpu_reset(&mut s.ctx, &mut Vec::new()).unwrap_err();
        assert_eq!(e.code, exit::LOCKED);
        assert!(e.message.starts_with("spela is already running (PID: "));
        assert_eq!(s.privilege.log().prompts, 0);
        drop(held);
        gpu_reset(&mut s.ctx, &mut Vec::new()).unwrap();
        // The command let the lock go.
        assert!(crate::locks::launch(&s.ctx.paths).is_ok());
    }

    #[test]
    fn a_dismissed_prompt_or_a_failed_step_says_why() {
        let mut s = setup();
        let mut privilege = s.privilege.clone();
        privilege.set_fault(Some(PrivilegeFault::Dismissed));
        s.ctx.backends.privilege = Box::new(privilege);
        let e = gpu_reset(&mut s.ctx, &mut Vec::new()).unwrap_err();
        assert_eq!(
            e.message,
            "failed to reset clocks: authentication was dismissed"
        );
        let mut privilege = s.privilege.clone();
        privilege.set_fault(Some(PrivilegeFault::FailAtStep(0)));
        s.ctx.backends.privilege = Box::new(privilege);
        let e = set_smt(&mut s.ctx, "off", &mut Vec::new()).unwrap_err();
        assert_eq!(e.message, "failed to set SMT: smt: fake failure at step 0");
        assert_eq!(read_sys(&s.sys, "smt/control"), "on");
    }
}
