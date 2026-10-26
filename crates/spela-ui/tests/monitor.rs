//! 3 Monitor: the sections, the alert list, the top bar's alert word and
//! the hardware actions with their confirmations (terminal-ux.md, hardware
//! actions; mockups.html, screen 5).

use spela_core::alerts::{self, AlertInput, Thresholds};
use spela_core::backend::{
    ApplyReport, GpuMetrics, PrivilegeError, SettingValue, Step, StepOutcome, StepReport,
    ThrottleReasons,
};
use spela_core::metrics::CpuMetrics;
use spela_ui::event::{Cmd, Event, HardwareOp, MetricsSample, Msg};
use spela_ui::layout::Pane;
use spela_ui::screens::monitor::{self, Section};
use spela_ui::screens::{Action, Popup, ScreenAction};
use spela_ui::state::Level;
use spela_ui::testing::{key, render_text, state_with_games};
use spela_ui::{AppState, update};

fn gpu(temp: u32, util: u32, power: u32) -> GpuMetrics {
    GpuMetrics {
        temperature_c: temp,
        utilization_percent: util,
        memory_used_mib: 12_083,
        memory_total_mib: 16_384,
        power_draw_w: power,
        power_limit_w: Some(320),
        power_max_limit_w: Some(360),
        graphics_clock_mhz: 2745,
        memory_clock_mhz: 11201,
        fan_percent: Some(64),
        ..GpuMetrics::default()
    }
}

fn cpu(util: u32, smt: Option<bool>) -> CpuMetrics {
    CpuMetrics {
        utilization_percent: Some(util),
        mhz: Some(3910),
        memory_used_mib: Some(10_444),
        memory_total_mib: Some(31_948),
        governor: Some("powersave".into()),
        governors: vec!["performance".into(), "powersave".into(), "schedutil".into()],
        smt_active: smt,
    }
}

/// A sample with the evaluator's alerts, as the metrics thread sends it.
fn sample(g: GpuMetrics, c: CpuMetrics) -> Event {
    let input = AlertInput {
        temperature_c: g.temperature_c,
        power_draw_w: f64::from(g.power_draw_w),
        power_limit_w: g.power_limit_w.map_or(0.0, f64::from),
        power_max_limit_w: g.power_max_limit_w.map_or(0.0, f64::from),
        fan_percent: g.fan_percent.unwrap_or(0),
        throttle_thermal: g.throttle.thermal(),
        throttle_power: g.throttle.power(),
    };
    let alerts = alerts::evaluate(&input, &Thresholds::default());
    Event::Msg(Msg::Metrics(Box::new(MetricsSample {
        gpu: Ok(g),
        cpu: Ok(c),
        alerts,
    })))
}

/// Monitor after the mockup's 20 hot samples: 62 °C up to 71 °C, 98 %
/// use and 318 W of 320 W, so "Power limited" is active.
fn hot(width: u16, height: u16) -> AppState {
    let mut s = state_with_games(width, height, 12);
    let temps = [
        62, 63, 63, 64, 65, 65, 66, 66, 67, 67, 68, 68, 69, 69, 70, 70, 70, 71, 71, 71,
    ];
    let utils = [
        71, 78, 84, 90, 93, 95, 96, 97, 97, 98, 98, 97, 98, 99, 98, 98, 97, 98, 98, 98,
    ];
    let power = [
        240, 262, 281, 296, 305, 310, 313, 315, 316, 317, 318, 317, 318, 319, 318, 318, 317, 318,
        318, 318,
    ];
    for i in 0..20 {
        update(
            &mut s,
            sample(gpu(temps[i], utils[i], power[i]), cpu(34, Some(true))),
        );
    }
    update(&mut s, key("3"));
    s
}

fn hardware_done(token: u64, result: Result<ApplyReport, PrivilegeError>) -> Event {
    Event::Msg(Msg::HardwareDone { token, result })
}

fn applied(step: Step, target: SettingValue) -> ApplyReport {
    ApplyReport {
        steps: vec![StepReport {
            step,
            target,
            outcome: StepOutcome::AppliedNoRestore,
        }],
    }
}

fn row_reason(s: &AppState, label: &str) -> Option<String> {
    let rows = spela_ui::screens::actions(s, s.focus);
    rows.into_iter()
        .find(|r| r.label == label)
        .unwrap_or_else(|| panic!("no row {label}"))
        .unavailable
}

/// Opens Actions and runs the row with this label.
fn run_row(s: &mut AppState, label: &str) -> Vec<Cmd> {
    update(s, key("0"));
    let Some(Popup::Actions(p)) = &s.popup else {
        panic!("0 opens Actions")
    };
    let i = p.rows.iter().position(|r| r.label == label).unwrap();
    let mut cmds = Vec::new();
    for _ in 0..i {
        cmds.extend(update(s, key("down")));
    }
    let Some(Popup::Actions(p)) = &s.popup else {
        panic!()
    };
    assert_eq!(p.rows[p.cursor].label, label);
    cmds.extend(update(s, key("enter")));
    cmds
}

#[test]
fn monitor_gpu_120x40() {
    let mut s = hot(120, 40);
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn monitor_gpu_80x24() {
    let mut s = hot(80, 24);
    update(&mut s, key("enter"));
    assert_eq!(s.focus, Pane::Detail);
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn monitor_cpu_and_alerts_120x40() {
    let mut s = hot(120, 40);
    update(&mut s, key("down"));
    let cpu = render_text(&mut s);
    assert!(cpu.contains("Governor powersave"), "{cpu}");
    assert!(cpu.contains("SMT      on"), "{cpu}");
    update(&mut s, key("down"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn the_list_counts_alerts_and_the_top_bar_names_the_highest() {
    let mut s = hot(120, 40);
    let text = render_text(&mut s);
    let first = text.lines().next().unwrap();
    assert!(first.ends_with("⚠ Power limited"), "{first}");
    assert!(text.contains("Alerts"));
    assert!(text.lines().nth(4).unwrap().contains("⚠ 1"), "{text}");

    // 86 °C with no throttle reason is High temp, critical (decision 60).
    update(&mut s, sample(gpu(86, 99, 200), cpu(40, Some(true))));
    let first = render_text(&mut s).lines().next().unwrap().to_owned();
    assert!(first.ends_with("✗ High temp"), "{first}");
    let throttled = GpuMetrics {
        throttle: ThrottleReasons {
            thermal_hardware: true,
            ..ThrottleReasons::default()
        },
        ..gpu(78, 99, 200)
    };
    update(&mut s, sample(throttled, cpu(40, Some(true))));
    let first = render_text(&mut s).lines().next().unwrap().to_owned();
    assert!(first.ends_with("✗ Throttling"), "{first}");

    // An info alert shows on Monitor only, as in Go.
    let fans = GpuMetrics {
        fan_percent: Some(100),
        ..gpu(60, 40, 100)
    };
    update(&mut s, sample(fans, cpu(40, Some(true))));
    s.monitor.section = Section::Alerts;
    let text = render_text(&mut s);
    assert!(!text.lines().next().unwrap().contains("Fan max"), "{text}");
    assert!(text.contains("⚠ GPU fans at 100%"), "{text}");
    assert!(text.contains("Recovery: GPU temperature: 60°C"), "{text}");
}

#[test]
fn reads_that_fail_keep_go_s_words() {
    let mut s = state_with_games(120, 40, 3);
    update(&mut s, key("3"));
    assert!(render_text(&mut s).contains(monitor::LOADING));
    let missing = spela_core::backend::GpuError::Unavailable("no driver (fake)".into());
    let failed = |e: String| {
        Event::Msg(Msg::Metrics(Box::new(MetricsSample {
            gpu: Err(e),
            cpu: Ok(cpu(5, None)),
            alerts: Vec::new(),
        })))
    };
    update(&mut s, failed(missing.to_string()));
    let text = render_text(&mut s);
    assert!(text.contains(monitor::UNSUPPORTED), "{text}");
    assert_eq!(
        row_reason(&s, "Reset GPU clocks").as_deref(),
        Some("NVML unavailable: no driver (fake)")
    );
    update(&mut s, sample(gpu(50, 10, 100), cpu(5, None)));
    update(&mut s, failed("read failed".into()));
    let text = render_text(&mut s);
    assert!(text.contains(monitor::STALE), "{text}");
    assert!(text.contains("read failed"), "{text}");
}

#[test]
fn the_rows_say_why_they_cannot_run() {
    let mut s = state_with_games(120, 40, 3);
    update(&mut s, key("3"));
    for label in ["Reset GPU clocks", "Set CPU governor", "Turn SMT off"] {
        assert_eq!(row_reason(&s, label).as_deref(), Some("loading metrics"));
    }
    let bare = CpuMetrics {
        governors: Vec::new(),
        smt_active: None,
        ..cpu(5, None)
    };
    update(&mut s, sample(gpu(50, 10, 100), bare));
    assert_eq!(row_reason(&s, "Reset GPU clocks"), None);
    assert_eq!(
        row_reason(&s, "Set CPU governor").as_deref(),
        Some("no cpufreq governors")
    );
    assert_eq!(
        row_reason(&s, "Turn SMT off").as_deref(),
        Some("SMT not supported")
    );
    update(&mut s, sample(gpu(50, 10, 100), cpu(5, Some(false))));
    assert_eq!(row_reason(&s, "Turn SMT on"), None);
    // The palette offers the same rows from another destination.
    update(&mut s, key("1"));
    let palette = spela_ui::screens::palette(&s);
    assert!(palette.iter().any(|r| r.label == "Turn SMT on"));
}

#[test]
fn a_gpu_reset_confirms_on_cancel_then_runs_once() {
    let mut s = hot(120, 40);
    assert!(run_row(&mut s, "Reset GPU clocks").is_empty());
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(c.focus, spela_ui::screens::confirm::Focus::Cancel);
    let body: Vec<String> = c
        .body
        .iter()
        .map(|l| l.iter().map(|(t, _)| t.as_str()).collect())
        .collect();
    assert_eq!(
        body,
        [
            "Resets GPU 0's locked clocks and clock offsets to the driver defaults.",
            "",
            monitor::LASTS,
            monitor::APPROVAL,
        ]
    );
    insta::assert_snapshot!("confirm_gpu_reset_120x40", render_text(&mut s));
    // Enter on Cancel cancels; nothing runs.
    assert!(update(&mut s, key("enter")).is_empty());
    assert_eq!(
        s.status.as_ref().unwrap().text,
        "GPU reset cancelled, nothing changed"
    );
    // Esc cancels too, and never confirms.
    run_row(&mut s, "Reset GPU clocks");
    assert!(update(&mut s, key("esc")).is_empty());
    assert!(s.popup.is_none());

    run_row(&mut s, "Reset GPU clocks");
    update(&mut s, key("right"));
    let cmds = update(&mut s, key("enter"));
    let [Cmd::Hardware { token, op }] = cmds.as_slice() else {
        panic!("{cmds:?}")
    };
    assert_eq!(*op, HardwareOp::GpuReset);
    assert_eq!(s.jobs.hardware, Some(*token));
    let text = render_text(&mut s);
    assert!(
        text.lines().nth(38).unwrap().ends_with("Applying..."),
        "{text}"
    );
    assert_eq!(
        row_reason(&s, "Reset GPU clocks").as_deref(),
        Some("wait for pending work")
    );
    assert!(update(&mut s, key("Q")).is_empty(), "Quit waits");
    assert!(!s.quit);

    // A stale token changes nothing; the real one says the CLI's words.
    update(
        &mut s,
        hardware_done(*token + 9, Ok(ApplyReport::default())),
    );
    assert_eq!(s.jobs.hardware, Some(*token));
    update(
        &mut s,
        hardware_done(
            *token,
            Ok(applied(Step::LockedClocks, SettingValue::DriverDefault)),
        ),
    );
    assert_eq!(s.jobs.hardware, None);
    let status = s.status.clone().unwrap();
    assert_eq!(
        (status.level, status.text.as_str()),
        (Level::Ok, "GPU clocks reset to default")
    );
}

#[test]
fn the_governor_picker_takes_the_actions_popup_s_place() {
    let mut s = hot(120, 40);
    run_row(&mut s, "Set CPU governor");
    let Some(Popup::Governor(g)) = &s.popup else {
        panic!("the governor picker")
    };
    assert_eq!(g.title, "Actions · Monitor · list");
    assert_eq!(g.picker.choices[g.picker.cursor].label, "powersave");
    assert!(g.picker.choices[g.picker.cursor].current);
    insta::assert_snapshot!("governor_picker_120x40", render_text(&mut s));

    // Esc goes back to the rows, on the governor row.
    update(&mut s, key("esc"));
    let Some(Popup::Actions(a)) = &s.popup else {
        panic!("back to the Actions rows")
    };
    assert_eq!(a.rows[a.cursor].label, "Set CPU governor");

    // Type-to-jump, then Enter opens the confirmation.
    update(&mut s, key("enter"));
    update(&mut s, key("s"));
    let Some(Popup::Governor(g)) = &s.popup else {
        panic!()
    };
    assert_eq!(g.picker.choices[g.picker.cursor].label, "schedutil");
    update(&mut s, key("up"));
    update(&mut s, key("up"));
    assert!(update(&mut s, key("enter")).is_empty());
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!("a confirmation")
    };
    assert_eq!(
        c.body[0][0].0,
        "Sets the CPU governor on every online CPU: powersave -> performance"
    );
    update(&mut s, key("right"));
    let cmds = update(&mut s, key("enter"));
    let [Cmd::Hardware { token, op }] = cmds.as_slice() else {
        panic!("{cmds:?}")
    };
    assert_eq!(*op, HardwareOp::Governor("performance".into()));
    update(
        &mut s,
        hardware_done(
            *token,
            Ok(applied(
                Step::CpuGovernor,
                SettingValue::Governor("performance".into()),
            )),
        ),
    );
    assert_eq!(
        s.status.as_ref().unwrap().text,
        "Governor set to performance"
    );
}

#[test]
fn picking_the_current_governor_changes_nothing() {
    let mut s = hot(120, 40);
    run_row(&mut s, "Set CPU governor");
    assert!(update(&mut s, key("enter")).is_empty());
    assert!(s.popup.is_none());
    assert_eq!(
        s.status.as_ref().unwrap().text,
        "The governor is already powersave"
    );
}

#[test]
fn smt_rows_name_the_opposite_state_and_failures_use_the_spec_s_words() {
    let mut s = hot(120, 40);
    run_row(&mut s, "Turn SMT off");
    let Some(Popup::Confirm(c)) = &s.popup else {
        panic!()
    };
    assert_eq!(c.title, "Turn SMT off");
    update(&mut s, key("right"));
    let cmds = update(&mut s, key("enter"));
    let [Cmd::Hardware { token, op }] = cmds.as_slice() else {
        panic!("{cmds:?}")
    };
    assert_eq!(*op, HardwareOp::Smt(false));
    let token = *token;
    update(
        &mut s,
        hardware_done(
            token,
            Err(PrivilegeError::Helper(
                spela_core::fake::NO_AGENT_MESSAGE.into(),
            )),
        ),
    );
    let status = s.status.clone().unwrap();
    assert_eq!(status.level, Level::Error);
    assert_eq!(
        status.text,
        "SMT change needs a polkit agent. Run 'spela cpu smt off' in a terminal"
    );

    let words =
        |op: HardwareOp, r: Result<ApplyReport, PrivilegeError>| monitor::result_text(&op, &r).1;
    assert_eq!(
        words(HardwareOp::GpuReset, Err(PrivilegeError::Dismissed)),
        "Not applied: the polkit prompt was dismissed"
    );
    assert_eq!(
        words(
            HardwareOp::GpuReset,
            Err(PrivilegeError::Helper(
                spela_core::fake::NO_AGENT_MESSAGE.into()
            ))
        ),
        "GPU reset needs a polkit agent. Run 'spela gpu reset' in a terminal"
    );
    assert_eq!(
        words(
            HardwareOp::Governor("performance".into()),
            Err(PrivilegeError::Unavailable(
                "spela is already running (PID: 42)".into()
            ))
        ),
        "spela is already running (PID: 42)"
    );
    let skipped = ApplyReport {
        steps: vec![StepReport {
            step: Step::Smt,
            target: SettingValue::Smt(true),
            outcome: StepOutcome::Skipped("prior value not readable: no smt control".into()),
        }],
    };
    assert_eq!(
        words(HardwareOp::Smt(true), Ok(skipped)),
        "Not applied: prior value not readable: no smt control"
    );
    assert_eq!(
        words(
            HardwareOp::Smt(true),
            Ok(applied(Step::Smt, SettingValue::Smt(true)))
        ),
        "SMT enabled"
    );
}

#[test]
fn clicks_and_the_wheel_move_between_sections() {
    use crossterm::event::{KeyModifiers, MouseButton, MouseEvent, MouseEventKind};
    let mut s = hot(120, 40);
    render_text(&mut s);
    let mouse = |kind, row| {
        Event::Mouse(MouseEvent {
            kind,
            column: 5,
            row,
            modifiers: KeyModifiers::NONE,
        })
    };
    update(&mut s, mouse(MouseEventKind::Down(MouseButton::Left), 4));
    assert_eq!(s.monitor.section, Section::Alerts);
    update(&mut s, mouse(MouseEventKind::ScrollDown, 4));
    assert_eq!(s.monitor.section, Section::Gpu, "the wheel wraps");
    let open = Action::Screen(ScreenAction::Monitor(monitor::Action::Open));
    spela_ui::update::run(&mut s, open);
    assert_eq!(s.focus, Pane::Detail);
}
