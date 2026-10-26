//! The UI's state: [`AppState`].
//!
//! One struct holds what every screen shares (size, destination, focus,
//! the popup, the status line, the config, the game cache, the metrics,
//! running jobs, pending saves) and one field per screen for that screen's
//! own state. Screens change the shared parts through the helpers here:
//! [`AppState::say`], [`AppState::open`], [`AppState::confirm`],
//! [`AppState::save`] and [`AppState::request`].

use std::collections::{BTreeMap, VecDeque};
use std::sync::Arc;

use jiff::{SignedDuration, Timestamp};
use spela_core::alerts::Alert;
use spela_core::backend::GpuMetrics;
use spela_core::clock::Clock;
use spela_core::config::Config;
use spela_core::games::GameCache;
use spela_core::metrics::CpuMetrics;
use spela_core::profile::Scope;

use crate::event::{Cmd, MetricsSample, Write};
use crate::hit::{HitMap, HitTarget};
use crate::layout::{Layout, Mode, Pane};
use crate::perf::PerfStats;
use crate::screens::{
    self, Dest, Owner, Popup, confirm, denylist, dlls, library, monitor, notice, profile, settings,
};
use crate::theme::{Glyphs, Theme};

/// How long a status message stays (Go's `messagebar.go`).
pub const STATUS_FOR: SignedDuration = SignedDuration::from_secs(5);

/// Samples each metrics ring keeps, as Go's header does.
pub const HISTORY: usize = 20;

/// How a status message reads.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Level {
    /// Plain.
    Info,
    /// Green, with ✓.
    Ok,
    /// Red, with ✗.
    Error,
}

/// A message on the left of the status line.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct StatusMessage {
    /// The text, without its glyph.
    pub text: String,
    /// How it reads.
    pub level: Level,
    /// When it clears.
    pub until: Timestamp,
}

/// The newest samples of one metric, oldest first.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Ring {
    values: VecDeque<u32>,
}

impl Ring {
    /// Adds a sample, dropping the oldest past [`HISTORY`].
    pub fn push(&mut self, value: u32) {
        if self.values.len() == HISTORY {
            self.values.pop_front();
        }
        self.values.push_back(value);
    }

    /// The samples, oldest first.
    pub fn values(&self) -> impl DoubleEndedIterator<Item = u32> + ExactSizeIterator + '_ {
        self.values.iter().copied()
    }

    /// The newest `n` samples, oldest first.
    pub fn last(&self, n: usize) -> impl Iterator<Item = u32> + '_ {
        self.values
            .iter()
            .skip(self.values.len().saturating_sub(n))
            .copied()
    }

    /// How many samples there are.
    pub fn len(&self) -> usize {
        self.values.len()
    }

    /// Whether there are none yet.
    pub fn is_empty(&self) -> bool {
        self.values.is_empty()
    }
}

/// The live metrics the top bar and Monitor share. There is one poll; the
/// UI only draws its result (terminal-ux.md, budgets).
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Metrics {
    /// The newest GPU sample, or why it failed. `None` before the first.
    pub gpu: Option<Result<GpuMetrics, String>>,
    /// The newest CPU sample, or why it failed.
    pub cpu: Option<Result<CpuMetrics, String>>,
    /// The active alerts.
    pub alerts: Vec<Alert>,
    /// GPU temperature in °C.
    pub temperature: Ring,
    /// GPU use in percent.
    pub utilization: Ring,
    /// GPU power in W.
    pub power: Ring,
    /// CPU use in percent.
    pub cpu_utilization: Ring,
    /// A poll is running; no other is asked for until it answers.
    pub polling: bool,
    /// Samples received.
    pub samples: u64,
}

impl Metrics {
    /// Takes a sample in.
    pub fn record(&mut self, sample: MetricsSample) {
        if let Ok(g) = &sample.gpu {
            self.temperature.push(g.temperature_c);
            self.utilization.push(g.utilization_percent);
            self.power.push(g.power_draw_w);
        }
        if let Ok(Some(u)) = sample.cpu.as_ref().map(|c| c.utilization_percent) {
            self.cpu_utilization.push(u);
        }
        self.gpu = Some(sample.gpu);
        self.cpu = Some(sample.cpu);
        self.alerts = sample.alerts;
        self.polling = false;
        self.samples += 1;
    }

    /// Whether neither GPU nor CPU can be read, so polling stops.
    pub fn unavailable(&self) -> bool {
        matches!((&self.gpu, &self.cpu), (Some(Err(_)), Some(Err(_))))
    }
}

/// Jobs that block Quit while they run, by request id or token.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Jobs {
    /// The background scan.
    pub scan: Option<u64>,
    /// A DLL install, update or restore.
    pub dll: Option<u64>,
    /// A hardware command.
    pub hardware: Option<u64>,
}

impl Jobs {
    /// Whether a job runs that Quit must wait for.
    pub fn busy(&self) -> bool {
        self.scan.is_some() || self.dll.is_some() || self.hardware.is_some()
    }
}

/// What the shell knows at start, before the first frame.
#[derive(Debug)]
pub struct Startup {
    /// The terminal size.
    pub size: (u16, u16),
    /// The loaded config.
    pub config: Config,
    /// The game cache as it is on disk. The first frame is drawn from it.
    pub games: GameCache,
    /// Games with their own profile file.
    pub profiles: Vec<u64>,
    /// Scopes whose Go profile did not migrate, with the error to show.
    /// The TUI shows it on that scope and does not edit it; the write
    /// queue refuses it too.
    pub broken_profiles: BTreeMap<Scope, String>,
    /// The migration notice, when this run migrated Go files.
    pub notice: Vec<String>,
    /// Config warnings, such as unknown keys.
    pub warnings: Vec<String>,
    /// `NO_COLOR` is set.
    pub no_color: bool,
    /// `--demo`.
    pub demo: bool,
    /// The clock.
    pub clock: Arc<dyn Clock>,
}

/// Everything the UI holds.
pub struct AppState {
    /// The terminal size in columns and rows.
    pub size: (u16, u16),
    /// The current destination.
    pub dest: Dest,
    /// The focused pane.
    pub focus: Pane,
    /// The open popup.
    pub popup: Option<Popup>,
    /// The status line's message.
    pub status: Option<StatusMessage>,
    /// The theme, after `NO_COLOR`.
    pub theme: Theme,
    /// The glyph set.
    pub glyphs: Glyphs,
    /// `NO_COLOR` is set, so the theme stays mono whatever the config says.
    pub no_color: bool,
    /// The config as last loaded or saved.
    pub config: Config,
    /// The game cache.
    pub games: GameCache,
    /// Scopes whose Go profile did not migrate, with the error to show.
    /// Never edit them: the write queue refuses them.
    pub broken_profiles: BTreeMap<Scope, String>,
    /// The live metrics.
    pub metrics: Metrics,
    /// Running jobs.
    pub jobs: Jobs,
    /// Pending saves, by token, with the screen that owns each.
    pub saves: BTreeMap<u64, Owner>,
    /// A save reached disk in this session, so the status line can say
    /// "Saved".
    pub saved_once: bool,
    /// The release notice, `spela vX is available`.
    pub release: Option<String>,
    /// `--demo`.
    pub demo: bool,
    /// The clock.
    pub clock: Arc<dyn Clock>,
    /// The last frame's clickable regions.
    pub hits: HitMap,
    /// The last click, for double clicks.
    pub last_click: Option<(HitTarget, Timestamp)>,
    /// Something visible changed since the last frame.
    pub dirty: bool,
    /// The UI asked to quit.
    pub quit: bool,
    /// Key-to-flush timings.
    pub perf: PerfStats,
    next_id: u64,
    /// [`library`]'s state.
    pub library: library::State,
    /// [`profile`]'s state.
    pub profile: profile::State,
    /// [`settings`]'s state.
    pub settings: settings::State,
    /// [`monitor`]'s state.
    pub monitor: monitor::State,
    /// [`dlls`]'s state.
    pub dlls: dlls::State,
    /// [`denylist`]'s state.
    pub denylist: denylist::State,
}

impl std::fmt::Debug for AppState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AppState")
            .field("size", &self.size)
            .field("dest", &self.dest)
            .field("focus", &self.focus)
            .finish_non_exhaustive()
    }
}

impl AppState {
    /// The state for a start. Nothing is drawn or asked for yet; call
    /// [`AppState::start`].
    pub fn new(startup: Startup) -> AppState {
        let theme = Theme::new(startup.config.ui.theme, startup.no_color);
        let glyphs = Glyphs::new(startup.config.ui.ascii_glyphs);
        let mut state = AppState {
            size: startup.size,
            dest: Dest::Library,
            focus: Pane::List,
            popup: None,
            status: None,
            theme,
            glyphs,
            no_color: startup.no_color,
            config: startup.config,
            games: startup.games,
            broken_profiles: startup.broken_profiles,
            metrics: Metrics::default(),
            jobs: Jobs::default(),
            saves: BTreeMap::new(),
            saved_once: false,
            release: None,
            demo: startup.demo,
            clock: startup.clock,
            hits: HitMap::new(),
            last_click: None,
            dirty: true,
            quit: false,
            perf: PerfStats::default(),
            next_id: 1,
            library: library::State::default(),
            profile: profile::State::default(),
            settings: settings::State::default(),
            monitor: monitor::State::default(),
            dlls: dlls::State::default(),
            denylist: denylist::State::default(),
        };
        library::set_profiles(&mut state, startup.profiles);
        library::games_changed(&mut state);
        if let Some(first) = startup.warnings.first() {
            let more = startup.warnings.len() - 1;
            let text = if more > 0 {
                format!("{first} (and {more} more in the log)")
            } else {
                first.clone()
            };
            state.say(Level::Error, text);
        }
        if !startup.notice.is_empty() {
            state.open(Popup::Notice(notice::State::new(startup.notice)));
        }
        state
    }

    /// The first commands: the metrics poll, one background scan when
    /// `rescan_on_startup` is on or the cache is empty (decision 24), and
    /// the deny list load.
    pub fn start(&mut self) -> Vec<Cmd> {
        let mut cmds = Vec::new();
        if self.wants_metrics() {
            self.metrics.polling = true;
            cmds.push(Cmd::PollMetrics);
        }
        if self.config.rescan_on_startup || self.games.is_empty() {
            cmds.extend(library::start_scan(self));
        }
        cmds.extend(denylist::start(self));
        cmds
    }

    /// The layout for the current size.
    pub fn layout(&self) -> Layout {
        Layout::new(self.size.0, self.size.1)
    }

    /// A fresh id for a request or a token.
    pub fn request(&mut self) -> u64 {
        let id = self.next_id;
        self.next_id += 1;
        id
    }

    /// Shows a message on the status line for five seconds.
    pub fn say(&mut self, level: Level, text: impl Into<String>) {
        self.status = Some(StatusMessage {
            text: text.into(),
            level,
            until: self.clock.now() + STATUS_FOR,
        });
        self.dirty = true;
    }

    /// Opens a popup, replacing any open one.
    pub fn open(&mut self, popup: Popup) {
        self.popup = Some(popup);
        self.dirty = true;
    }

    /// Opens a confirmation, with focus on Cancel.
    pub fn confirm(&mut self, confirm: confirm::State) {
        self.open(Popup::Confirm(confirm));
    }

    /// Closes the popup.
    pub fn close_popup(&mut self) {
        if self.popup.take().is_some() {
            self.dirty = true;
        }
    }

    /// Queues a write for `owner` and returns its token and the command.
    /// The result comes back to `owner`'s `saved`.
    pub fn save(&mut self, owner: Owner, write: Write) -> (u64, Cmd) {
        let token = self.request();
        self.saves.insert(token, owner);
        self.dirty = true;
        (token, Cmd::Save { token, write })
    }

    /// Whether Quit can run now, or why not.
    pub fn can_quit(&self) -> Result<(), String> {
        if self.jobs.busy() {
            Err("wait for pending operations".into())
        } else {
            Ok(())
        }
    }

    /// Whether the metrics tick should run: a normal frame is up and the
    /// GPU or the CPU can be read. The tick does not stop on focus lost
    /// (decision 42).
    pub fn wants_metrics(&self) -> bool {
        self.layout().mode != Mode::TooSmall && !self.metrics.unavailable()
    }

    /// The next time the UI needs a wake-up: when the status message
    /// clears.
    pub fn next_wakeup(&self) -> Option<Timestamp> {
        self.status.as_ref().map(|m| m.until)
    }

    /// Applies a changed config: theme, glyphs and the rest. Returns
    /// [`Cmd::SetMouse`] when `ui.mouse` changed, so the terminal's mouse
    /// modes follow at once; run what it returns.
    pub fn set_config(&mut self, config: Config) -> Vec<Cmd> {
        let mouse = (config.ui.mouse != self.config.ui.mouse).then_some(config.ui.mouse);
        self.theme = Theme::new(config.ui.theme, self.no_color);
        self.glyphs = Glyphs::new(config.ui.ascii_glyphs);
        self.config = config;
        self.dirty = true;
        mouse.map(Cmd::SetMouse).into_iter().collect()
    }

    /// Whether the focused pane shows: always in two-pane mode, and the
    /// one pane in single mode.
    pub fn shows(&self, pane: Pane) -> bool {
        self.layout().two_panes() || self.focus == pane
    }

    /// Whether the UI is in Browse.
    pub fn browsing(&self) -> bool {
        screens::browsing(self)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rings_keep_the_newest_twenty() {
        let mut r = Ring::default();
        for v in 0..25 {
            r.push(v);
        }
        assert_eq!(r.len(), HISTORY);
        assert_eq!(r.values().next(), Some(5));
        assert_eq!(r.last(3).collect::<Vec<_>>(), [22, 23, 24]);
    }

    #[test]
    fn polling_stops_only_when_both_reads_fail() {
        let mut m = Metrics::default();
        assert!(!m.unavailable());
        m.record(MetricsSample {
            gpu: Err("no NVML".into()),
            cpu: Ok(CpuMetrics::default()),
            alerts: Vec::new(),
        });
        assert!(!m.unavailable());
        m.record(MetricsSample {
            gpu: Err("no NVML".into()),
            cpu: Err("no /proc".into()),
            alerts: Vec::new(),
        });
        assert!(m.unavailable());
    }
}
