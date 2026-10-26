//! 3 Monitor: GPU, CPU and Alerts in List, sparklines, gauges and the
//! alert list in Detail, and the hardware actions (decision 44).
//!
//! It reads `AppState::metrics`, the same samples and rings the top bar
//! shows. The 2 s poll is the shell's; this screen only draws the result
//! (mockups.html, screen 5). The alerts come from the evaluator on the
//! metrics thread ([`spela_core::alerts`]).
//!
//! Its Actions rows, in either pane and in the palette, are the hardware
//! actions of terminal-ux.md: `Reset GPU clocks`, `Set CPU governor` and
//! `Turn SMT off` or `Turn SMT on`. Each one confirms first, with focus on
//! Cancel, and `confirm_destructive = false` does not skip it. Confirm
//! sends [`crate::Cmd::Hardware`], which the shell runs as one job on its
//! pool; the result comes back as [`crate::Msg::HardwareDone`] to
//! [`hardware_done`], which says it on the status line in the CLI's words.
//! While a job runs, the rows are unavailable with `wait for pending
//! work`, the status line shows `Applying...` and Quit waits.
//!
//! `Set CPU governor` opens a picker in the Actions popup's place, titled
//! like it: the governors sysfs lists, the current one marked ✓. Esc goes
//! back to the Actions rows, and a pick opens the confirmation. The
//! governor list, the current governor and the SMT state come with each
//! metrics sample, read on the metrics thread, so a change shows at the
//! next tick.

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::alerts::{self, Severity};
use spela_core::backend::{ApplyReport, GpuError, GpuMetrics, PrivilegeError, StepOutcome};
use spela_core::metrics::CpuMetrics;

use super::actions as actions_popup;
use super::{Action as AnyAction, Dest, Popup, PopupAction, Row, ScreenAction, confirm};
use crate::event::{Cmd as AnyCmd, HardwareOp};
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::{Layout, Pane, PaneArea};
use crate::paint::{Painter, fit};
use crate::picker::{Choice, Picker, PickerInput, PickerOutcome};
use crate::state::{AppState, Level, Ring};
use crate::theme::{Glyphs, Role, temperature_ratio, thermal_step};

/// Cells a label takes in Detail, as in the mockup.
pub const LABEL_W: usize = 7;
/// Cells a value takes before its sparkline, as in the mockup.
pub const VALUE_W: usize = 16;
/// Cells a gauge takes.
pub const GAUGE_W: usize = 20;
/// The CPU block's label width: `Governor` needs 9 with its space.
pub const CPU_LABEL_W: usize = 9;

/// Shown before the first sample (Go's `Loading metrics…`).
pub const LOADING: &str = "Loading metrics...";
/// Shown when the backend says GPU or CPU reads are not possible here.
pub const UNSUPPORTED: &str = "Unsupported on this system";
/// Shown when the first read failed.
pub const FAILED: &str = "Metrics failed to load";
/// Shown when a read failed after an earlier one worked. Go's text had an
/// em dash, which is outside the glyph set.
pub const STALE: &str = "Stale metrics, refresh failed";
/// The Alerts section with nothing active.
pub const NO_ALERTS: &str = "No active alerts";

/// The two lines every hardware confirmation ends with.
pub const LASTS: &str = "This change stays until you change it again. spela does not undo it.";
/// The approval line of every hardware confirmation.
pub const APPROVAL: &str = "Needs administrator approval through polkit.";

/// What the privileged helper's error says when no polkit agent can ask
/// (unverified: the user's live check settles how polkit reports it).
const NO_AGENT: &str = "No authentication agent";

/// A section of the List.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Section {
    /// GPU 0.
    #[default]
    Gpu,
    /// The CPU.
    Cpu,
    /// The active alerts.
    Alerts,
}

impl Section {
    /// Every section, in List order.
    pub const ALL: [Section; 3] = [Section::Gpu, Section::Cpu, Section::Alerts];

    /// The List label and the Detail title.
    pub fn label(self) -> &'static str {
        match self {
            Section::Gpu => "GPU",
            Section::Cpu => "CPU",
            Section::Alerts => "Alerts",
        }
    }

    fn index(self) -> usize {
        Section::ALL.iter().position(|s| *s == self).unwrap_or(0)
    }
}

/// Monitor's state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The List cursor.
    pub section: Section,
    /// The first Detail line shown.
    pub scroll: usize,
    /// The hardware job that runs, with its command, for the result.
    pub running: Option<(u64, HardwareOp)>,
}

/// What Monitor's keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// The previous section.
    Up,
    /// The next section.
    Down,
    /// Moves the cursor to a section.
    Select(Section),
    /// Opens the section in Detail.
    Open,
    /// Scrolls Detail up a line.
    ScrollUp,
    /// Scrolls Detail down a line.
    ScrollDown,
    /// The `Reset GPU clocks` row: asks first.
    AskGpuReset,
    /// The `Set CPU governor` row: opens the governor picker.
    PickGovernor,
    /// The `Turn SMT off` or `on` row: asks first.
    AskSmt(bool),
    /// A confirmed hardware command.
    Run(HardwareOp),
}

/// Monitor's worker results. Hardware results come as the shell's
/// [`crate::Msg::HardwareDone`].
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {}

/// Monitor's requests. Hardware jobs are the shell's
/// [`crate::Cmd::Hardware`].
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {}

/// The governor picker, open in the Actions popup's place.
#[derive(Clone, Debug, PartialEq)]
pub struct GovernorPopup {
    /// The Actions popup's title, `Actions · Monitor · list`.
    pub title: String,
    /// The picker over the governors sysfs lists.
    pub picker: Picker,
    /// The governor now, for the confirmation.
    pub current: Option<String>,
    /// The Actions popup it replaced, for Esc. `None` when it opened from
    /// the palette, so Esc closes it.
    pub back: Option<actions_popup::State>,
}

/// What the governor picker's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum GovernorAction {
    /// A key for the picker.
    Input(PickerInput),
}

fn act(a: Action) -> AnyAction {
    AnyAction::Screen(ScreenAction::Monitor(a))
}

fn governor_act(input: PickerInput) -> AnyAction {
    AnyAction::Popup(PopupAction::Governor(GovernorAction::Input(input)))
}

fn governor_typed(c: char) -> AnyAction {
    governor_act(PickerInput::Type(c))
}

/// The pane title: `Monitor` on List, the section on Detail.
pub fn title(state: &AppState, pane: Pane) -> String {
    match pane {
        Pane::List => "Monitor".to_owned(),
        Pane::Detail => state.monitor.section.label().to_owned(),
    }
}

/// Monitor has no editor or picker in its panes.
pub fn editing(_state: &AppState, _pane: Pane) -> bool {
    false
}

/// The keys of a pane. The List has no groups, so Left and Right are not
/// bound (mockups.html, screen 5).
pub fn bindings(state: &AppState, pane: Pane) -> Vec<Binding> {
    match pane {
        Pane::List => vec![
            Binding::new(Key::UP, act(Action::Up), "Item").hint_as("Up/Down", 100),
            Binding::new(Key::DOWN, act(Action::Down), "Next item"),
            Binding::new(Key::ENTER, act(Action::Open), "Open").hint(90),
        ],
        Pane::Detail => {
            if detail_overflows(state) {
                vec![
                    Binding::new(Key::UP, act(Action::ScrollUp), "Scroll").hint_as("Up/Down", 100),
                    Binding::new(Key::DOWN, act(Action::ScrollDown), "Scroll down"),
                ]
            } else {
                Vec::new()
            }
        }
    }
}

/// The hardware rows, the same in both panes.
pub fn actions(state: &AppState, _pane: Pane) -> Vec<Row> {
    hardware_rows(state)
}

/// The palette lists the hardware rows from every destination.
pub fn palette(state: &AppState) -> Vec<Row> {
    hardware_rows(state)
}

fn busy(state: &AppState) -> Result<(), String> {
    match state.jobs.hardware {
        Some(_) => Err("wait for pending work".into()),
        None => Ok(()),
    }
}

/// Whether GPU 0 can take a reset, or why not (decision 58).
pub fn gpu_available(state: &AppState) -> Result<(), String> {
    match &state.metrics.gpu {
        None => Err("loading metrics".into()),
        Some(Err(e)) => Err(format!("NVML unavailable: {}", nvml_reason(e))),
        Some(Ok(_)) => Ok(()),
    }
}

/// The CPU sample, or why the CPU state is not known.
fn cpu_state(state: &AppState) -> Result<&CpuMetrics, String> {
    match &state.metrics.cpu {
        None => Err("loading metrics".into()),
        Some(Err(e)) => Err(e.clone()),
        Some(Ok(c)) => Ok(c),
    }
}

/// The governors the picker offers, or why there are none.
pub fn governors(state: &AppState) -> Result<&[String], String> {
    let c = cpu_state(state)?;
    if c.governors.is_empty() {
        Err("no cpufreq governors".into())
    } else {
        Ok(&c.governors)
    }
}

/// Whether SMT is on, or why it cannot change.
pub fn smt(state: &AppState) -> Result<bool, String> {
    cpu_state(state)?
        .smt_active
        .ok_or_else(|| "SMT not supported".into())
}

fn hardware_rows(state: &AppState) -> Vec<Row> {
    let smt = smt(state);
    let smt_label = match smt {
        Ok(false) => "Turn SMT on",
        _ => "Turn SMT off",
    };
    let smt_target = !matches!(smt, Ok(false));
    vec![
        Row::new("Reset GPU clocks", act(Action::AskGpuReset))
            .when(gpu_available(state).and_then(|()| busy(state))),
        Row::new("Set CPU governor", act(Action::PickGovernor))
            .when(governors(state).map(|_| ()).and_then(|()| busy(state))),
        Row::new(smt_label, act(Action::AskSmt(!smt_target)))
            .when(smt.map(|_| ()).and_then(|()| busy(state))),
    ]
}

/// The reason without the error's own "not available" prefix, so the row
/// reads `NVML unavailable: <reason>`.
fn nvml_reason(e: &str) -> &str {
    let prefix = GpuError::Unavailable(String::new()).to_string();
    e.strip_prefix(prefix.as_str()).unwrap_or(e)
}

/// Runs an action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    let n = Section::ALL.len();
    match action {
        Action::Up => {
            let i = state.monitor.section.index();
            state.monitor.section = Section::ALL[(i + n - 1) % n];
            state.monitor.scroll = 0;
        }
        Action::Down => {
            let i = state.monitor.section.index();
            state.monitor.section = Section::ALL[(i + 1) % n];
            state.monitor.scroll = 0;
        }
        Action::Select(s) => {
            if state.monitor.section != s {
                state.monitor.scroll = 0;
            }
            state.monitor.section = s;
        }
        Action::Open => state.focus = Pane::Detail,
        Action::ScrollUp => state.monitor.scroll = state.monitor.scroll.saturating_sub(1),
        Action::ScrollDown => {
            let max = max_scroll(state);
            state.monitor.scroll = (state.monitor.scroll + 1).min(max);
        }
        Action::AskGpuReset => ask(
            state,
            "Reset GPU clocks",
            "Resets GPU 0's locked clocks and clock offsets to the driver defaults.".to_owned(),
            HardwareOp::GpuReset,
            "GPU reset cancelled, nothing changed",
        ),
        Action::PickGovernor => open_governor_picker(state),
        Action::AskSmt(on) => {
            let (word, from, to) = if on {
                ("on", "off", "on")
            } else {
                ("off", "on", "off")
            };
            ask(
                state,
                &format!("Turn SMT {word}"),
                format!(
                    "Turns simultaneous multithreading {word} for the whole CPU: {from} -> {to}"
                ),
                HardwareOp::Smt(on),
                "SMT change cancelled, nothing changed",
            );
        }
        Action::Run(op) => return run(state, op),
    }
    Vec::new()
}

/// Opens the confirmation for a hardware command, with focus on Cancel.
fn ask(state: &mut AppState, title: &str, change: String, op: HardwareOp, cancelled: &str) {
    let body = vec![
        vec![(change, Role::Text)],
        Vec::new(),
        vec![(LASTS.to_owned(), Role::Muted)],
        vec![(APPROVAL.to_owned(), Role::Muted)],
    ];
    state.confirm(confirm::State::new(title, body, act(Action::Run(op))).cancel_message(cancelled));
}

/// Starts a confirmed hardware command, unless one runs.
fn run(state: &mut AppState, op: HardwareOp) -> Vec<AnyCmd> {
    if let Err(reason) = busy(state) {
        state.say(Level::Error, reason);
        return Vec::new();
    }
    let token = state.request();
    state.jobs.hardware = Some(token);
    state.monitor.running = Some((token, op.clone()));
    state.dirty = true;
    vec![AnyCmd::Hardware { token, op }]
}

/// A hardware job finished: the status line says how, in the CLI's words
/// (terminal-ux.md, hardware actions). A stale token is dropped.
pub fn hardware_done(
    state: &mut AppState,
    token: u64,
    result: Result<ApplyReport, PrivilegeError>,
) -> Vec<AnyCmd> {
    if state.jobs.hardware != Some(token) {
        return Vec::new();
    }
    state.jobs.hardware = None;
    state.dirty = true;
    let Some((_, op)) = state.monitor.running.take().filter(|(t, _)| *t == token) else {
        return Vec::new();
    };
    let (level, text) = result_text(&op, &result);
    state.say(level, text);
    Vec::new()
}

/// The status line's words for a hardware result.
pub fn result_text(
    op: &HardwareOp,
    result: &Result<ApplyReport, PrivilegeError>,
) -> (Level, String) {
    match result {
        Ok(report) if report.all_applied() => (
            Level::Ok,
            match op {
                HardwareOp::GpuReset => "GPU clocks reset to default".to_owned(),
                HardwareOp::Governor(g) => format!("Governor set to {g}"),
                HardwareOp::Smt(true) => "SMT enabled".to_owned(),
                HardwareOp::Smt(false) => "SMT disabled".to_owned(),
            },
        ),
        Ok(report) => {
            let why = report
                .steps
                .iter()
                .find_map(|s| match &s.outcome {
                    StepOutcome::Failed(e) | StepOutcome::Skipped(e) => Some(e.clone()),
                    _ => None,
                })
                .unwrap_or_else(|| "the helper reported no result".to_owned());
            (Level::Error, format!("Not applied: {why}"))
        }
        Err(PrivilegeError::Dismissed) => (
            Level::Error,
            "Not applied: the polkit prompt was dismissed".to_owned(),
        ),
        Err(PrivilegeError::Helper(text)) if text.contains(NO_AGENT) => {
            let (what, command) = what_and_command(op);
            (
                Level::Error,
                format!("{what} needs a polkit agent. Run '{command}' in a terminal"),
            )
        }
        // How pkexec reports a missing agent is a user live check, so any
        // other helper failure gets the same pointer to the CLI
        // (architecture.md, TUI process).
        Err(PrivilegeError::Helper(text)) => {
            let (_, command) = what_and_command(op);
            (
                Level::Error,
                format!(
                    "{}. Run '{command}' in a terminal",
                    text.trim().trim_end_matches('.')
                ),
            )
        }
        Err(PrivilegeError::Unavailable(text)) => (Level::Error, text.clone()),
    }
}

/// A hardware command's name for the status line, and its CLI command.
fn what_and_command(op: &HardwareOp) -> (&'static str, String) {
    match op {
        HardwareOp::GpuReset => ("GPU reset", "spela gpu reset".to_owned()),
        HardwareOp::Governor(g) => ("Governor change", format!("spela cpu governor {g}")),
        HardwareOp::Smt(on) => (
            "SMT change",
            format!("spela cpu smt {}", if *on { "on" } else { "off" }),
        ),
    }
}

/// Opens the governor picker in the Actions popup's place.
fn open_governor_picker(state: &mut AppState) {
    let list = match governors(state).and_then(|g| busy(state).map(|()| g)) {
        Ok(list) => list.to_vec(),
        Err(reason) => {
            state.say(Level::Error, reason);
            return;
        }
    };
    let current = cpu_state(state).ok().and_then(|c| c.governor.clone());
    let pane = state.focus;
    let g = state.glyphs;
    let title = format!(
        "Actions {} {} {} {}",
        g.dot,
        state.dest.name(),
        g.dot,
        pane.word()
    );
    let back = (state.dest == Dest::Monitor).then(|| {
        let rows = super::actions(state, pane);
        let mut popup = actions_popup::State::new(title.clone(), rows);
        if let Some(i) = popup
            .rows
            .iter()
            .position(|r| r.action == act(Action::PickGovernor))
        {
            popup.cursor = i;
        }
        popup
    });
    let choices: Vec<Choice> = Picker::choices(&list, current.as_deref());
    state.open(Popup::Governor(GovernorPopup {
        title,
        picker: Picker::new("Set CPU governor", None, choices),
        current,
        back,
    }));
}

/// The governor picker's keys.
pub fn governor_bindings(_state: &AppState, popup: &GovernorPopup) -> Vec<Binding> {
    popup.picker.bindings(governor_act, governor_typed)
}

/// Runs a governor picker key.
pub fn governor_update(state: &mut AppState, action: GovernorAction) -> Vec<AnyCmd> {
    let GovernorAction::Input(input) = action;
    let Some(Popup::Governor(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    match popup.picker.handle(input) {
        PickerOutcome::Moved => Vec::new(),
        PickerOutcome::Cancel => {
            let back = popup.back.take();
            match back {
                Some(rows) => state.open(Popup::Actions(rows)),
                None => state.close_popup(),
            }
            Vec::new()
        }
        PickerOutcome::Apply(i) => {
            let choice = popup.picker.choices.get(i).map(|c| c.label.clone());
            let current = popup.current.clone();
            let Some(governor) = choice else {
                return Vec::new();
            };
            if current.as_deref() == Some(governor.as_str()) {
                state.close_popup();
                state.say(Level::Info, format!("The governor is already {governor}"));
                return Vec::new();
            }
            let from = current.unwrap_or_else(|| "unknown".to_owned());
            ask(
                state,
                "Set CPU governor",
                format!("Sets the CPU governor on every online CPU: {from} -> {governor}"),
                HardwareOp::Governor(governor),
                "Governor change cancelled, nothing changed",
            );
            Vec::new()
        }
    }
}

/// Draws the governor picker where the Actions popup was.
pub fn governor_render(
    state: &AppState,
    popup: &GovernorPopup,
    frame: &mut Frame,
    layout: &Layout,
    hits: &mut HitMap,
) {
    let area = layout.right_anchored(
        actions_popup::WIDTH,
        popup.picker.height().saturating_add(2),
    );
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = p.popup(area, &popup.title, hits);
    if inner.width < 20 || inner.height < 3 {
        return;
    }
    popup.picker.render(&mut p, inner);
}

/// Monitor saves nothing.
pub fn on_msg(_state: &mut AppState, msg: Msg) -> Vec<AnyCmd> {
    match msg {}
}

/// Monitor saves nothing.
pub fn saved(_state: &mut AppState, _token: u64, _result: Result<(), String>) -> Vec<AnyCmd> {
    Vec::new()
}

/// The wheel moves the List cursor, or scrolls Detail.
pub fn wheel(state: &mut AppState, pane: Pane, down: bool) -> Vec<AnyCmd> {
    let action = match (pane, down) {
        (Pane::List, true) => Action::Down,
        (Pane::List, false) => Action::Up,
        (Pane::Detail, true) => Action::ScrollDown,
        (Pane::Detail, false) => Action::ScrollUp,
    };
    update(state, action)
}

/// One Detail line: spans of text in roles.
pub type Line = Vec<(String, Role)>;

fn label(text: &str, w: usize, g: &Glyphs) -> (String, Role) {
    (fit(text, w, g), Role::Muted)
}

fn spark(
    ring: &Ring,
    lo: f64,
    hi: f64,
    g: &Glyphs,
    role: impl Fn(f64) -> Role,
) -> Vec<(String, Role)> {
    ring.values()
        .map(|v| {
            let r = if hi > lo {
                ((f64::from(v) - lo) / (hi - lo)).clamp(0.0, 1.0)
            } else {
                0.0
            };
            (g.spark_for(r).to_owned(), role(r))
        })
        .collect()
}

fn gauge(used: f64, total: f64, g: &Glyphs) -> Vec<(String, Role)> {
    let ratio = if total > 0.0 {
        (used / total).clamp(0.0, 1.0)
    } else {
        0.0
    };
    let full = (ratio * GAUGE_W as f64).round() as usize;
    vec![
        (g.gauge_full.repeat(full), Role::Focus),
        (g.gauge_empty.repeat(GAUGE_W - full), Role::Muted),
    ]
}

/// `°C` as the glyph set writes it: `C` alone with ASCII glyphs.
fn degrees(text: &str, g: &Glyphs) -> String {
    if g.degree == "°" {
        text.to_owned()
    } else {
        text.replace('°', "")
    }
}

fn gib(mib: u64) -> f64 {
    mib as f64 / 1024.0
}

/// What a failed read shows: Go's state words, then the reason.
fn failure_lines(error: &str, had_data: bool) -> Vec<Line> {
    let unsupported = [
        GpuError::Unavailable(String::new()).to_string(),
        GpuError::NotSupported(String::new()).to_string(),
    ];
    let word = if had_data {
        STALE
    } else if unsupported.iter().any(|p| error.starts_with(p.as_str())) {
        UNSUPPORTED
    } else {
        FAILED
    };
    vec![
        vec![(word.to_owned(), Role::Text)],
        vec![(error.to_owned(), Role::Muted)],
    ]
}

fn gpu_lines(state: &AppState) -> Vec<Line> {
    let m = &state.metrics;
    let g = &state.glyphs;
    let gpu: &GpuMetrics = match &m.gpu {
        None => return vec![vec![(LOADING.to_owned(), Role::Muted)]],
        Some(Err(e)) => return failure_lines(e, !m.temperature.is_empty()),
        Some(Ok(gpu)) => gpu,
    };
    let thermal = |r: f64| Role::Thermal(thermal_step(r));
    let mut lines = Vec::new();
    let temp = degrees(&format!("{}°C", gpu.temperature_c), g);
    let mut line = vec![
        label("Temp", LABEL_W, g),
        (
            fit(&temp, VALUE_W, g),
            thermal(temperature_ratio(gpu.temperature_c)),
        ),
    ];
    line.extend(spark(&m.temperature, 30.0, 95.0, g, thermal));
    lines.push(line);
    let mut line = vec![
        label("Util", LABEL_W, g),
        (
            fit(&format!("{}%", gpu.utilization_percent), VALUE_W, g),
            Role::Text,
        ),
    ];
    line.extend(spark(&m.utilization, 0.0, 100.0, g, |_| Role::Focus));
    lines.push(line);
    let (power, role, hi) = match gpu.power_limit_w.filter(|l| *l > 0) {
        Some(limit) => (
            format!("{}W / {limit}W", gpu.power_draw_w),
            thermal(f64::from(gpu.power_draw_w) / f64::from(limit)),
            f64::from(limit),
        ),
        None => (
            format!("{}W", gpu.power_draw_w),
            Role::Text,
            f64::from(m.power.values().max().unwrap_or(0).max(1)),
        ),
    };
    let mut line = vec![label("Power", LABEL_W, g), (fit(&power, VALUE_W, g), role)];
    line.extend(spark(&m.power, 0.0, hi, g, thermal));
    lines.push(line);
    let mut line = vec![label("VRAM", LABEL_W, g)];
    line.extend(gauge(
        gpu.memory_used_mib as f64,
        gpu.memory_total_mib as f64,
        g,
    ));
    line.push((
        format!(
            "  {:.1} / {:.1} GB",
            gib(gpu.memory_used_mib),
            gib(gpu.memory_total_mib)
        ),
        Role::Text,
    ));
    lines.push(line);
    if gpu.graphics_clock_mhz > 0 {
        let mut line = vec![
            label("Clock", LABEL_W, g),
            (format!("{} MHz", gpu.graphics_clock_mhz), Role::Text),
        ];
        if gpu.memory_clock_mhz > 0 {
            line.push(("   Mem  ".to_owned(), Role::Muted));
            line.push((format!("{} MHz", gpu.memory_clock_mhz), Role::Text));
        }
        lines.push(line);
    }
    if let Some(fan) = gpu.fan_percent {
        let mut line = vec![label("Fan", LABEL_W, g)];
        line.extend(gauge(f64::from(fan), 100.0, g));
        line.push((format!("  {fan}%"), Role::Text));
        lines.push(line);
    }
    lines
}

fn cpu_lines(state: &AppState) -> Vec<Line> {
    let m = &state.metrics;
    let g = &state.glyphs;
    let c = match &m.cpu {
        None => return vec![vec![(LOADING.to_owned(), Role::Muted)]],
        Some(Err(e)) => return failure_lines(e, !m.cpu_utilization.is_empty()),
        Some(Ok(c)) => c,
    };
    let w = CPU_LABEL_W;
    let mut lines = Vec::new();
    let (util, role) = match c.utilization_percent {
        Some(u) => (
            format!("{u}%"),
            Role::Thermal(thermal_step(f64::from(u) / 100.0)),
        ),
        None => ("measuring...".to_owned(), Role::Muted),
    };
    let mut line = vec![label("Util", w, g), (fit(&util, VALUE_W, g), role)];
    line.extend(spark(&m.cpu_utilization, 0.0, 100.0, g, |_| Role::Focus));
    lines.push(line);
    if let Some(mhz) = c.mhz {
        lines.push(vec![
            label("Freq", w, g),
            (format!("{mhz} MHz"), Role::Text),
        ]);
    }
    if let (Some(used), Some(total)) = (c.memory_used_mib, c.memory_total_mib) {
        let mut line = vec![label("RAM", w, g)];
        line.extend(gauge(used as f64, total as f64, g));
        line.push((
            format!("  {:.1} / {:.1} GB", gib(used), gib(total)),
            Role::Text,
        ));
        lines.push(line);
    }
    if let Some(governor) = &c.governor {
        lines.push(vec![
            label("Governor", w, g),
            (governor.clone(), Role::Text),
        ]);
    }
    if let Some(on) = c.smt_active {
        let word = if on { "on" } else { "off" };
        lines.push(vec![label("SMT", w, g), (word.to_owned(), Role::Text)]);
    }
    lines
}

fn alert_lines(state: &AppState) -> Vec<Line> {
    let g = &state.glyphs;
    if state.metrics.alerts.is_empty() {
        return vec![vec![(NO_ALERTS.to_owned(), Role::Muted)]];
    }
    let mut lines = Vec::new();
    for a in &state.metrics.alerts {
        let (glyph, role) = severity_look(a.severity, g);
        lines.push(vec![(degrees(&format!("{glyph} {}", a.message), g), role)]);
        if !a.suggestion.is_empty() {
            let text = format!("  Recovery: {}", a.suggestion);
            lines.push(vec![(degrees(&text, g), Role::Muted)]);
        }
    }
    lines
}

/// What Monitor draws from the metrics, as text: the Detail lines and the
/// alert badge in the List. A sample that leaves it the same is not drawn.
pub fn view_key(state: &AppState) -> String {
    let badge = alerts::highest(&state.metrics.alerts).map(|a| a.severity);
    format!(
        "{:?}{badge:?}{}",
        detail_lines(state),
        state.metrics.alerts.len()
    )
}

/// The glyph and role of a severity: ✗ critical, ⚠ warning, ⚠ muted info.
pub fn severity_look(severity: Severity, g: &Glyphs) -> (&'static str, Role) {
    match severity {
        Severity::Critical => (g.error, Role::Error),
        Severity::Warning => (g.warn, Role::Warn),
        Severity::Info => (g.warn, Role::Muted),
    }
}

/// The Detail lines of the selected section, headed by its name in
/// single-pane mode.
pub fn detail_lines(state: &AppState) -> Vec<Line> {
    let section = state.monitor.section;
    let mut lines = Vec::new();
    // In one pane the title reads `▸ Detail`, so the section heads it.
    if !state.layout().two_panes() {
        lines.push(vec![(section.label().to_owned(), Role::Title)]);
        lines.push(Vec::new());
    }
    lines.extend(match section {
        Section::Gpu => gpu_lines(state),
        Section::Cpu => cpu_lines(state),
        Section::Alerts => alert_lines(state),
    });
    lines
}

fn detail_height(state: &AppState) -> usize {
    usize::from(state.layout().pane(Pane::Detail).inner.height)
}

fn max_scroll(state: &AppState) -> usize {
    detail_lines(state)
        .len()
        .saturating_sub(detail_height(state))
}

fn detail_overflows(state: &AppState) -> bool {
    detail_lines(state).len() > detail_height(state)
}

/// Draws a pane.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    match area.pane {
        Pane::List => render_list(state, frame, area, hits),
        Pane::Detail => render_detail(state, frame, area),
    }
}

fn render_list(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    let inner = area.inner;
    let g = state.glyphs;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, g);
    if inner.width < 10 || inner.height == 0 {
        return;
    }
    let focused = state.focus == Pane::List;
    let x = inner.x + 1;
    let w = inner.width - 2;
    for (i, section) in Section::ALL.iter().enumerate() {
        let y = inner.y + u16::try_from(i).unwrap_or(0);
        if y >= inner.bottom() {
            break;
        }
        let line = Rect::new(inner.x, y, inner.width, 1);
        let cursor = *section == state.monitor.section;
        let (glyph_role, label_role) = match (cursor, focused) {
            (true, true) => (Role::Key, Role::Key),
            (true, false) => (Role::Muted, Role::Text),
            _ => (Role::Muted, Role::Text),
        };
        if cursor {
            p.text(x, y, 1, g.cursor, glyph_role);
        }
        p.text(x + 2, y, w - 2, section.label(), label_role);
        if *section == Section::Alerts
            && let Some(top) = alerts::highest(&state.metrics.alerts)
        {
            let (glyph, role) = severity_look(top.severity, &g);
            let text = format!("{glyph} {}", state.metrics.alerts.len());
            let tw = u16::try_from(crate::text::width(&text)).unwrap_or(0);
            p.text(inner.right().saturating_sub(tw + 1), y, tw, &text, role);
        }
        if cursor {
            p.cursor_row(line);
        }
        hits.push(
            line,
            HitTarget::Row {
                select: act(Action::Select(*section)),
                open: act(Action::Open),
            },
        );
    }
}

fn render_detail(state: &AppState, frame: &mut Frame, area: PaneArea) {
    let inner = area.inner;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    if inner.width < 4 || inner.height == 0 {
        return;
    }
    let lines = detail_lines(state);
    let height = usize::from(inner.height);
    let scroll = state.monitor.scroll.min(lines.len().saturating_sub(height));
    let x = inner.x + 1;
    let w = inner.width - 2;
    for (n, line) in lines.iter().skip(scroll).take(height).enumerate() {
        let y = inner.y + u16::try_from(n).unwrap_or(0);
        let spans: Vec<(&str, Role)> = line.iter().map(|(t, r)| (t.as_str(), *r)).collect();
        p.spans(x, y, w, &spans);
    }
    p.thumb(area.border_x, inner, scroll, height, lines.len());
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;

    #[test]
    fn up_and_down_wrap_over_the_three_sections() {
        let mut s = state_with_games(120, 40, 3);
        step(&mut s, key("3"));
        assert_eq!(s.monitor.section, Section::Gpu);
        step(&mut s, key("up"));
        assert_eq!(s.monitor.section, Section::Alerts);
        step(&mut s, key("down"));
        step(&mut s, key("down"));
        assert_eq!(s.monitor.section, Section::Cpu);
        step(&mut s, key("enter"));
        assert_eq!(s.focus, Pane::Detail);
        assert_eq!(title(&s, Pane::Detail), "CPU");
    }

    #[test]
    fn the_nvml_reason_drops_the_error_prefix() {
        let e = GpuError::Unavailable("libnvidia-ml.so.1 not found".into()).to_string();
        assert_eq!(nvml_reason(&e), "libnvidia-ml.so.1 not found");
        assert_eq!(nvml_reason("other"), "other");
    }
}
