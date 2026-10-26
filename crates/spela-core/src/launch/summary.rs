//! The dry-run summary and the launch message.
//!
//! The order and wording are Go's `PrintLaunchSummary`
//! (`cmd/spela/commands/launch.go:109-136`), with the changes the
//! architecture lists in "One plan for dry run and launch":
//!
//! - the `overlay:` impacts group and the Overlay block are gone
//!   (decision 10);
//! - sources are the origin names `this_game`, `all_games` and
//!   `system_default`, in place of Go's `override`, `default` and `unset`;
//! - the Environment block shows every variable the launch sets,
//!   `PROTON_VKD3D_HEAP` included, and `__GL_THREADED_OPTIMIZATION` only
//!   when a profile sets it (decision 47);
//! - a Preflight block shows the descriptor heap notice;
//! - the Hardware block lists KDE VRR, says which fields the profile names
//!   but leaves unchanged, so it agrees with the impacts on zero values,
//!   and says when pkexec will use its generic action.
//!
//! The summary is lines of tagged spans. The CLI paints them, and the
//! wrapper and the TUI take the plain text, so all three print one text.

use super::{DenyState, LaunchPlan, ProfileUse};
use crate::profile::FieldKey;

/// The shell's install path, which the polkit policy names.
pub const INSTALL_PATH: &str = "/usr/bin/spela";

/// The last line of the TUI's Preview launch and the hint of a launch
/// without `--dry-run` (decision 40).
pub const STEAM_HINT: &str = "set the game's Steam launch options to `spela %command%` instead";

/// How a span is shown. The CLI maps each to a colour role.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tone {
    /// Ordinary text.
    Plain,
    /// The game's name.
    Name,
    /// A block heading.
    Heading,
    /// A line that asks for attention.
    Accent,
    /// Labels and plan lines.
    Dim,
}

/// One piece of a line.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Span {
    /// How it is shown.
    pub tone: Tone,
    /// The text.
    pub text: String,
}

/// One line of the summary.
pub type Line = Vec<Span>;

/// Where the summary comes from, which sets its Command line.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Mode {
    /// `spela launch --dry-run` and the TUI's Preview launch: the Steam URI
    /// and the note that a direct launch cannot clean up.
    DryRun,
    /// Wrapper mode, with the command Steam passed.
    Wrapper(String),
}

/// The printed summary.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Summary {
    /// Every line, in order. An empty line is a blank line.
    pub lines: Vec<Line>,
}

impl Summary {
    /// The text without colour, each line ended by a newline.
    pub fn plain(&self) -> String {
        self.render(|_, text| text.to_owned())
    }

    /// The text with each span passed through `paint`.
    pub fn render(&self, paint: impl Fn(Tone, &str) -> String) -> String {
        let mut out = String::new();
        for line in &self.lines {
            for span in line {
                if span.tone == Tone::Plain {
                    out.push_str(&span.text);
                } else {
                    out.push_str(&paint(span.tone, &span.text));
                }
            }
            out.push('\n');
        }
        out
    }
}

struct Builder {
    lines: Vec<Line>,
}

impl Builder {
    fn line(&mut self, spans: &[(Tone, &str)]) {
        self.lines.push(
            spans
                .iter()
                .map(|(tone, text)| Span {
                    tone: *tone,
                    text: (*text).to_owned(),
                })
                .collect(),
        );
    }

    fn blank(&mut self) {
        self.lines.push(Vec::new());
    }

    fn heading(&mut self, text: &str) {
        self.blank();
        self.line(&[(Tone::Heading, text)]);
    }

    /// `  <dim label> <rest>`.
    fn labelled(&mut self, label: &str, rest: &str) {
        self.line(&[(Tone::Plain, "  "), (Tone::Dim, label), (Tone::Plain, rest)]);
    }

    fn dim(&mut self, text: &str) {
        self.line(&[(Tone::Plain, "  "), (Tone::Dim, text)]);
    }
}

/// The summary of a plan. `agent` names the program that asks for the
/// password, which only the binary's privilege module names.
pub fn summary(plan: &LaunchPlan, mode: &Mode, agent: &str) -> Summary {
    let mut b = Builder { lines: Vec::new() };
    b.line(&[
        (Tone::Dim, "Game:"),
        (Tone::Plain, " "),
        (Tone::Name, &plan.name),
        (Tone::Plain, &format!(" (AppID {})", plan.app_id)),
    ]);
    b.blank();
    match mode {
        Mode::DryRun => {
            b.line(&[
                (Tone::Dim, "Command:"),
                (
                    Tone::Plain,
                    &format!(" steam steam://rungameid/{}", plan.app_id),
                ),
            ]);
            b.line(&[
                (Tone::Accent, "Launch option:"),
                (
                    Tone::Plain,
                    " direct Steam URI launch cannot track cleanup coverage; use Steam launch options: spela %command%",
                ),
            ]);
        }
        Mode::Wrapper(command) => {
            b.line(&[
                (Tone::Dim, "Command:"),
                (Tone::Plain, &format!(" {command}")),
            ]);
        }
    }
    let profile = if plan.own_count == 0 {
        " no profile-specific mutation planned".to_owned()
    } else {
        format!(" {} profile-specific override(s) planned", plan.own_count)
    };
    b.line(&[(Tone::Dim, "Profile:"), (Tone::Plain, &profile)]);
    if plan.profile == ProfileUse::None {
        b.blank();
        b.line(&[(
            Tone::Dim,
            "No profile or defaults will affect launch preparation",
        )]);
    }

    impacts(&mut b, plan);

    b.heading("Environment");
    let env = plan.env.sorted_lines();
    if env.is_empty() {
        b.dim("no launch environment variables planned");
    }
    for line in &env {
        b.dim(line);
    }

    if let Some(notice) = plan.preflight.as_ref().and_then(|p| p.notice.as_deref()) {
        b.heading("Preflight");
        b.line(&[(Tone::Plain, "  "), (Tone::Accent, notice)]);
    }

    b.heading("DLL");
    b.dim("no launch-time DLL file mutation planned");
    match &plan.deny {
        DenyState::Allowed => b.labelled("denylist:", " allowed"),
        DenyState::Denied(reason) => {
            let reason = if reason.is_empty() {
                crate::profile::words::NONE
            } else {
                reason
            };
            b.labelled("denylist:", &format!(" denied ({reason})"));
        }
        DenyState::Unknown(e) => b.labelled("denylist:", &format!(" unknown ({e})")),
    }
    let backup = if plan.backup {
        " available"
    } else {
        " not created yet"
    };
    b.labelled("backup:", backup);
    for dll in &plan.dlls {
        b.labelled(
            &format!("{}:", dll.family),
            &format!(" {} {}; path write: {}", dll.name, dll.version, dll.write),
        );
    }

    hardware(&mut b, plan, agent);
    Summary { lines: b.lines }
}

fn impacts(b: &mut Builder, plan: &LaunchPlan) {
    b.heading("Profile impacts");
    // game_file stays as Go printed it, though no field touches game files.
    for group in ["compatibility", "environment", "game_file", "system_state"] {
        let items: Vec<_> = plan
            .impacts
            .iter()
            .filter(|i| i.key.field().impact.name() == group)
            .collect();
        let label = format!("{group}:");
        if items.is_empty() {
            b.labelled(&label, " none");
            continue;
        }
        b.line(&[(Tone::Plain, "  "), (Tone::Dim, &label)]);
        for item in items {
            b.line(&[(
                Tone::Plain,
                &format!(
                    "    {}={} source={} restore={}",
                    item.key.as_str(),
                    item.value,
                    item.source,
                    item.restore
                ),
            )]);
        }
    }
}

fn hardware(b: &mut Builder, plan: &LaunchPlan, agent: &str) {
    b.heading("Hardware");
    let request = &plan.hardware;
    if request.is_empty() {
        b.dim("no privileged hardware mutation planned");
    } else {
        b.dim(&format!(
            "privileged changes apply via {agent} and restore on cleanup"
        ));
        if !plan.at_install_path {
            b.dim(&format!(
                "spela is not at {INSTALL_PATH}, so {agent} asks with its generic action"
            ));
        }
    }
    let rows: [(FieldKey, &str, Option<String>); 6] = [
        (
            FieldKey::GpuClockOffset,
            "GPU clock offset:",
            request.clock_offset_mhz.map(|v| format!("{v} MHz")),
        ),
        (
            FieldKey::GpuMemoryOffset,
            "GPU memory offset:",
            request.memory_offset_mhz.map(|v| format!("{v} MHz")),
        ),
        (
            FieldKey::GpuPowerLimit,
            "GPU power limit:",
            request.power_limit_w.map(|v| format!("{v} W")),
        ),
        (
            FieldKey::GpuFanSpeed,
            "GPU fan speed:",
            request.fan.map(|f| match f {
                crate::backend::FanSetting::Percent(p) => format!("{p}%"),
                crate::backend::FanSetting::Auto => "auto".to_owned(),
            }),
        ),
        (
            FieldKey::CpuGovernor,
            "CPU governor:",
            request.cpu_governor.clone(),
        ),
        (
            FieldKey::CpuSmt,
            "CPU SMT:",
            request.smt.map(|on| on.to_string()),
        ),
    ];
    for (key, label, value) in rows {
        if let Some(value) = value {
            b.labelled(label, &format!("  {value}"));
        } else if let Some(u) = plan.unchanged.iter().find(|u| u.key == key) {
            b.labelled(label, &format!("  {}", u.why));
        }
    }
    if let Some(policy) = plan.vrr {
        b.labelled(
            "KDE VRR:",
            &format!("  {policy} in the desktop session, restored on cleanup"),
        );
    } else if let Some(u) = plan.unchanged.iter().find(|u| u.key == FieldKey::GpuVrr) {
        b.labelled("KDE VRR:", &format!("  {}", u.why));
    }
}

/// The line printed just before the game starts. `skipped` says another
/// launch holds the launch lock, so VRR and hardware changes were dropped
/// (decision 33).
pub fn launch_message(plan: Option<&LaunchPlan>, skipped: bool) -> String {
    let mut text = match plan {
        None => "Launching command...".to_owned(),
        Some(p) => match p.profile {
            ProfileUse::Own => format!("Launching {} with profile...", p.name),
            ProfileUse::AllGames => format!("Launching {} with All games settings...", p.name),
            ProfileUse::None => format!("Launching {} (no profile)...", p.name),
        },
    };
    if skipped {
        text.push_str(" VRR and hardware changes are skipped: another launch holds the lock.");
    }
    text
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Any program name will do: only the binary names the real one.
    const AGENT: &str = "the agent";

    use crate::backend::{FanSetting, HardwareRequest, VrrPolicy};
    use crate::launch::{DllLine, ImpactItem, LaunchEnv, Preflight, Unchanged};

    fn plan() -> LaunchPlan {
        let mut env = LaunchEnv::new();
        env.set("__GL_SHADER_DISK_CACHE", "1");
        env.set("PROTON_ENABLE_HDR", "1");
        LaunchPlan {
            app_id: 9100010,
            name: "Glimmer Run".into(),
            profile: ProfileUse::Own,
            own_count: 2,
            impacts: vec![
                ImpactItem {
                    key: FieldKey::ProtonEnableHdr,
                    value: "true".into(),
                    source: "this_game",
                    restore: "ephemeral_launch_environment",
                },
                ImpactItem {
                    key: FieldKey::GpuFanSpeed,
                    value: "0".into(),
                    source: "this_game",
                    restore: "restorable_mutation",
                },
                ImpactItem {
                    key: FieldKey::CpuGovernor,
                    value: "performance".into(),
                    source: "all_games",
                    restore: "restorable_mutation",
                },
            ],
            env,
            vrr: Some(VrrPolicy::Always),
            hardware: HardwareRequest {
                power_limit_w: Some(250),
                cpu_governor: Some("performance".into()),
                ..HardwareRequest::default()
            },
            unchanged: vec![Unchanged {
                key: FieldKey::GpuFanSpeed,
                why: "left unchanged (gpu.fan_speed=0)".into(),
            }],
            preflight: Some(Preflight {
                compatibility: crate::proton::check(
                    &Err(crate::proton::ResolveError::NoSteamRoot),
                    Some("590.1.2"),
                ),
                notice: Some("ⓘ could not resolve".into()),
            }),
            deny: DenyState::Denied("kernel anti-cheat".into()),
            backup: true,
            dlls: vec![DllLine {
                family: "dlss",
                name: "nvngx_dlss.dll".into(),
                version: "310.2.1".into(),
                write: "write bit present",
            }],
            at_install_path: false,
        }
    }

    #[test]
    fn the_whole_dry_run_text() {
        let text = summary(&plan(), &Mode::DryRun, AGENT).plain();
        let expected = [
            "Game: Glimmer Run (AppID 9100010)",
            "",
            "Command: steam steam://rungameid/9100010",
            "Launch option: direct Steam URI launch cannot track cleanup coverage; use Steam launch options: spela %command%",
            "Profile: 2 profile-specific override(s) planned",
            "",
            "Profile impacts",
            "  compatibility:",
            "    proton.enable_hdr=true source=this_game restore=ephemeral_launch_environment",
            "  environment: none",
            "  game_file: none",
            "  system_state:",
            "    gpu.fan_speed=0 source=this_game restore=restorable_mutation",
            "    cpu.governor=performance source=all_games restore=restorable_mutation",
            "",
            "Environment",
            "  PROTON_ENABLE_HDR=1",
            "  __GL_SHADER_DISK_CACHE=1",
            "",
            "Preflight",
            "  ⓘ could not resolve",
            "",
            "DLL",
            "  no launch-time DLL file mutation planned",
            "  denylist: denied (kernel anti-cheat)",
            "  backup: available",
            "  dlss: nvngx_dlss.dll 310.2.1; path write: write bit present",
            "",
            "Hardware",
            "  privileged changes apply via the agent and restore on cleanup",
            "  spela is not at /usr/bin/spela, so the agent asks with its generic action",
            "  GPU power limit:  250 W",
            "  GPU fan speed:  left unchanged (gpu.fan_speed=0)",
            "  CPU governor:  performance",
            "  KDE VRR:  always in the desktop session, restored on cleanup",
        ];
        assert_eq!(text.lines().collect::<Vec<_>>(), expected);
    }

    #[test]
    fn wrapper_mode_names_the_command_and_an_empty_plan_says_so() {
        let mut p = plan();
        p.own_count = 0;
        p.profile = ProfileUse::None;
        p.impacts.clear();
        p.env = LaunchEnv::new();
        p.vrr = None;
        p.hardware = HardwareRequest {
            fan: Some(FanSetting::Percent(60)),
            smt: Some(false),
            ..HardwareRequest::default()
        };
        p.unchanged.clear();
        p.preflight = None;
        p.at_install_path = true;
        p.deny = DenyState::Allowed;
        let text = summary(&p, &Mode::Wrapper("/games/run.sh -dx12".into()), AGENT).plain();
        assert!(text.contains("Command: /games/run.sh -dx12\nProfile: no profile-specific mutation planned\n\nNo profile or defaults will affect launch preparation\n"), "{text}");
        assert!(!text.contains("Launch option:"));
        assert!(text.contains("  compatibility: none\n"));
        assert!(text.contains("Environment\n  no launch environment variables planned\n"));
        assert!(!text.contains("Preflight"));
        assert!(
            text.contains("  GPU fan speed:  60%\n  CPU SMT:  false\n"),
            "{text}"
        );
        assert!(!text.contains("generic action"));
        assert!(text.contains("denylist: allowed"));
    }

    #[test]
    fn painting_touches_only_tagged_spans() {
        let painted = summary(&plan(), &Mode::DryRun, AGENT).render(|tone, text| match tone {
            Tone::Name => format!("<{text}>"),
            _ => text.to_owned(),
        });
        assert!(painted.starts_with("Game: <Glimmer Run> (AppID 9100010)\n"));
    }

    #[test]
    fn launch_messages() {
        let mut p = plan();
        assert_eq!(
            launch_message(Some(&p), false),
            "Launching Glimmer Run with profile..."
        );
        p.profile = ProfileUse::AllGames;
        assert_eq!(
            launch_message(Some(&p), false),
            "Launching Glimmer Run with All games settings..."
        );
        p.profile = ProfileUse::None;
        assert_eq!(
            launch_message(Some(&p), true),
            "Launching Glimmer Run (no profile)... VRR and hardware changes are skipped: another launch holds the lock."
        );
        assert_eq!(launch_message(None, false), "Launching command...");
    }
}
