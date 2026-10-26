//! The one-channel event loop.
//!
//! The shape is mejl's `app/mod.rs` (`run`, `handle_batch`, `after_turn`,
//! `draw`) with the fixes architecture.md lists. One turn:
//!
//! 1. Block for one event, then take everything already waiting.
//! 2. Coalesce runs of resize and drag events, keeping the first stamp.
//! 3. Feed each event to [`spela_ui::update`]. A stop signal quits.
//!    SIGCONT clears the screen and feeds a resize.
//! 4. Run each [`Cmd`]: the metrics thread, the job pool, the write queue,
//!    or the [`Host`] for suspend.
//! 5. `after_turn`: ticking on or off from `wants_metrics`, and one wake-up
//!    per new deadline from `next_wakeup`.
//! 6. Draw at most once, and only when the turn changed something visible
//!    (`state.dirty`). mejl draws after every batch, ticks included.
//! 7. Draw into the backend's BufWriter, with the synchronized-output
//!    markers through the same writer when the probe found support, then
//!    flush. Nothing else writes to stdout while the TUI runs.
//! 8. Record one key-to-flush sample per input event of the batch.
//!
//! [`App`] is generic over the backend and the command runner, so the
//! deterministic counters (draws, turns, polls) run in-process on a
//! `TestBackend` and a recorder.

use std::io::{self, Write};
use std::time::Instant;

use ratatui::Terminal;
use ratatui::backend::{Backend, CrosstermBackend, TestBackend};
use spela_ui::perf::FrameSample;
use spela_ui::{AppState, Cmd, Event, Msg};

use super::events::{LoopEvent, LoopReceiver, TimedEvent};
use super::exec::Exec;
use super::host::Host;
use super::input::coalesce;
use super::timer::Timer;

/// The begin and end markers of synchronized output (mode 2026).
pub const SYNC_BEGIN: &[u8] = b"\x1b[?2026h";
/// The end marker.
pub const SYNC_END: &[u8] = b"\x1b[?2026l";

/// A backend the loop can write the synchronized-output markers to.
pub trait Markers {
    /// Writes raw bytes into the backend's own writer.
    fn write_raw(&mut self, bytes: &[u8]) -> io::Result<()>;
}

impl<W: Write> Markers for CrosstermBackend<W> {
    fn write_raw(&mut self, bytes: &[u8]) -> io::Result<()> {
        self.write_all(bytes)
    }
}

impl Markers for TestBackend {
    fn write_raw(&mut self, _bytes: &[u8]) -> io::Result<()> {
        Ok(())
    }
}

/// Counts that gate every push: they do not depend on machine speed
/// (decision 55).
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct Counters {
    /// Loop turns: batches handled.
    pub turns: u64,
    /// Frames drawn.
    pub draws: u64,
    /// Ticks received.
    pub ticks: u64,
    /// Wake-ups received.
    pub wakes: u64,
}

/// The loop: the UI state, the terminal, and what runs commands.
pub struct App<B: Backend + Markers> {
    /// The UI state.
    pub state: AppState,
    /// The terminal.
    pub terminal: Terminal<B>,
    /// Suspend.
    pub host: Box<dyn Host>,
    /// Runs commands.
    pub exec: Box<dyn Exec>,
    /// The timer; `None` in tests, which feed ticks themselves.
    pub timer: Option<Timer>,
    /// Write the synchronized-output markers.
    pub synchronized: bool,
    /// The deterministic counters.
    pub counters: Counters,
    /// Why the loop stopped on its own, for a line after the restore.
    pub exit_note: Option<String>,
    quit: bool,
    last_wake: Option<jiff::Timestamp>,
}

impl<B: Backend + Markers> App<B> {
    /// A loop over `state` and `terminal`.
    pub fn new(
        state: AppState,
        terminal: Terminal<B>,
        host: Box<dyn Host>,
        exec: Box<dyn Exec>,
    ) -> App<B> {
        App {
            state,
            terminal,
            host,
            exec,
            timer: None,
            synchronized: false,
            counters: Counters::default(),
            exit_note: None,
            quit: false,
            last_wake: None,
        }
    }

    /// Whether the loop is done.
    pub fn quitting(&self) -> bool {
        self.quit || self.state.quit
    }

    /// Runs the first commands and draws the first frame, from the game
    /// cache as it is.
    pub fn start(&mut self) -> io::Result<()> {
        let cmds = self.state.start();
        for c in cmds {
            self.exec_cmd(c);
        }
        self.after_turn();
        self.draw(&[])
    }

    /// Runs turns until the UI quits or every sender is gone.
    pub fn run(&mut self, rx: &LoopReceiver<Msg>) -> io::Result<()> {
        while !self.quitting() {
            let batch = rx.recv_batch();
            if batch.is_empty() {
                break;
            }
            self.handle_batch(batch)?;
        }
        Ok(())
    }

    /// One turn: applies the batch, runs the commands, draws at most once.
    pub fn handle_batch(&mut self, batch: Vec<LoopEvent<Msg>>) -> io::Result<()> {
        self.counters.turns += 1;
        let mut stamps: Vec<Instant> = Vec::new();
        for item in coalesce_batch(batch) {
            match item {
                LoopEvent::Input(events) => {
                    for t in events {
                        stamps.push(t.at);
                        if let Some(ev) = Event::from_terminal(t.event) {
                            self.feed(ev);
                        }
                    }
                }
                LoopEvent::InputClosed(reason) => {
                    self.exit_note = Some(format!("the terminal stopped sending input: {reason}"));
                    self.quit = true;
                }
                LoopEvent::Signal(kind) => {
                    if kind.is_stop() {
                        self.quit = true;
                    } else {
                        self.repaint_all()?;
                    }
                }
                LoopEvent::Tick => {
                    self.counters.ticks += 1;
                    self.feed(Event::Tick);
                }
                LoopEvent::Wake(_) => {
                    self.counters.wakes += 1;
                    self.last_wake = None;
                    self.feed(Event::Wake);
                }
                LoopEvent::Done(msg) => self.feed(Event::Msg(msg)),
            }
            if self.quitting() {
                return Ok(());
            }
        }
        self.after_turn();
        if self.state.dirty {
            self.draw(&stamps)?;
        }
        Ok(())
    }

    fn feed(&mut self, ev: Event) {
        let cmds = spela_ui::update(&mut self.state, ev);
        for c in cmds {
            self.exec_cmd(c);
        }
    }

    fn exec_cmd(&mut self, cmd: Cmd) {
        match cmd {
            Cmd::Quit => self.quit = true,
            Cmd::Suspend => {
                if let Err(e) = self.suspend() {
                    tracing::warn!("{e}");
                }
            }
            other => self.exec.run(other),
        }
    }

    fn suspend(&mut self) -> io::Result<()> {
        // Nothing may sit in the BufWriter when the modes switch.
        self.terminal.backend_mut().flush().map_err(backend_error)?;
        if let Some(t) = &self.timer {
            t.set_ticking(false);
        }
        if let Err(e) = self.host.suspend() {
            spela_ui::state::AppState::say(&mut self.state, spela_ui::state::Level::Error, e);
        }
        self.repaint_all()
    }

    /// After a resume or SIGCONT: the shell may have reset the terminal,
    /// so clear and redraw everything at the current size.
    fn repaint_all(&mut self) -> io::Result<()> {
        self.terminal.clear().map_err(backend_error)?;
        let size = self.terminal.size().map_err(backend_error)?;
        self.feed(Event::Resize(size.width, size.height));
        self.state.dirty = true;
        Ok(())
    }

    fn after_turn(&mut self) {
        let Some(timer) = &self.timer else {
            return;
        };
        timer.set_ticking(self.state.wants_metrics());
        if let Some(at) = self.state.next_wakeup()
            && self.last_wake != Some(at)
        {
            timer.wake_at_timestamp(at, self.state.clock.now());
            self.last_wake = Some(at);
        }
    }

    fn draw(&mut self, stamps: &[Instant]) -> io::Result<()> {
        if self.synchronized {
            self.terminal.backend_mut().write_raw(SYNC_BEGIN)?;
        }
        let state = &mut self.state;
        self.terminal
            .draw(|f| spela_ui::render(state, f))
            .map_err(backend_error)?;
        if self.synchronized {
            self.terminal.backend_mut().write_raw(SYNC_END)?;
        }
        self.terminal.backend_mut().flush().map_err(backend_error)?;
        self.counters.draws += 1;
        self.state.dirty = false;
        for at in stamps {
            self.state.perf.record(FrameSample {
                input_to_flush: at.elapsed(),
            });
        }
        Ok(())
    }
}

fn backend_error(e: impl std::fmt::Display) -> io::Error {
    io::Error::other(e.to_string())
}

/// Joins the batch's input runs and coalesces them, keeping everything
/// else in order.
fn coalesce_batch(batch: Vec<LoopEvent<Msg>>) -> Vec<LoopEvent<Msg>> {
    let mut out = Vec::with_capacity(batch.len());
    let mut run: Vec<TimedEvent> = Vec::new();
    let flush = |run: &mut Vec<TimedEvent>, out: &mut Vec<LoopEvent<Msg>>| {
        if !run.is_empty() {
            out.push(LoopEvent::Input(coalesce(std::mem::take(run))));
        }
    };
    for e in batch {
        match e {
            LoopEvent::Input(events) => run.extend(events),
            other => {
                flush(&mut run, &mut out);
                out.push(other);
            }
        }
    }
    flush(&mut run, &mut out);
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tui::exec::Recorder;
    use crate::tui::host::RecordingHost;
    use crossterm::event::{Event as TermEvent, KeyCode, KeyEvent, KeyModifiers};
    use spela_core::backend::GpuMetrics;
    use spela_core::metrics::CpuMetrics;
    use spela_ui::event::MetricsSample;
    use spela_ui::testing::state_with_games;

    fn app() -> (App<TestBackend>, Recorder) {
        let rec = Recorder::default();
        let terminal = Terminal::new(TestBackend::new(120, 40)).unwrap();
        let mut app = App::new(
            state_with_games(120, 40, 12),
            terminal,
            Box::new(RecordingHost::default()),
            Box::new(rec.clone()),
        );
        app.start().unwrap();
        (app, rec)
    }

    fn key(c: char) -> LoopEvent<Msg> {
        LoopEvent::Input(vec![TimedEvent {
            event: TermEvent::Key(KeyEvent::new(KeyCode::Char(c), KeyModifiers::NONE)),
            at: Instant::now(),
        }])
    }

    fn steady() -> Msg {
        Msg::Metrics(Box::new(MetricsSample {
            gpu: Ok(GpuMetrics {
                temperature_c: 54,
                utilization_percent: 37,
                memory_used_mib: 2048,
                memory_total_mib: 16376,
                power_draw_w: 182,
                graphics_clock_mhz: 1800,
                memory_clock_mhz: 10501,
                fan_percent: Some(40),
            }),
            cpu: Ok(CpuMetrics {
                utilization_percent: Some(12),
                ..CpuMetrics::default()
            }),
            alerts: Vec::new(),
        }))
    }

    #[test]
    fn the_first_frame_polls_and_scans() {
        let (app, rec) = app();
        assert_eq!(app.counters.draws, 1);
        let cmds = rec.cmds.lock().unwrap().clone();
        assert_eq!(cmds[0], Cmd::PollMetrics);
        assert!(
            matches!(cmds[1], Cmd::Scan { .. }),
            "rescan_on_startup is on"
        );
    }

    #[test]
    fn a_batch_draws_once_and_an_unbound_key_draws_nothing() {
        let (mut app, _) = app();
        app.handle_batch(vec![key('2'), key('3'), key('4')])
            .unwrap();
        assert_eq!(app.counters.draws, 2, "one draw for three keys");
        app.handle_batch(vec![key('x')]).unwrap();
        assert_eq!(app.counters.draws, 2, "an unbound key changes nothing");
        assert_eq!(app.state.perf.summary().total, 3, "one sample per key");
    }

    #[test]
    fn idle_ticks_with_steady_metrics_draw_at_most_once() {
        let (mut app, rec) = app();
        app.handle_batch(vec![LoopEvent::Done(steady())]).unwrap();
        // A 60 s idle window: 30 ticks, each answered with the same sample.
        for _ in 0..30 {
            app.handle_batch(vec![LoopEvent::Tick]).unwrap();
            app.handle_batch(vec![LoopEvent::Done(steady())]).unwrap();
        }
        let polls = rec
            .cmds
            .lock()
            .unwrap()
            .iter()
            .filter(|c| **c == Cmd::PollMetrics)
            .count();
        assert_eq!(polls, 31, "one poll per tick, plus the first");
        // The first samples fill the sparkline; once the five shown are the
        // same, a sample draws nothing.
        assert!(app.counters.draws <= 1 + 6, "{:?}", app.counters);
        let before = app.counters.draws;
        for _ in 0..10 {
            app.handle_batch(vec![LoopEvent::Tick]).unwrap();
            app.handle_batch(vec![LoopEvent::Done(steady())]).unwrap();
        }
        assert_eq!(app.counters.draws, before, "no draw when nothing changed");
        assert_eq!(app.counters.ticks, 40);
    }

    #[test]
    fn a_stop_signal_quits_and_sigcont_repaints() {
        use crate::tui::signals::SignalKind;
        let (mut app, _) = app();
        app.handle_batch(vec![LoopEvent::Signal(SignalKind::Continue)])
            .unwrap();
        assert_eq!(app.counters.draws, 2);
        app.handle_batch(vec![LoopEvent::Signal(SignalKind::Terminate)])
            .unwrap();
        assert!(app.quitting());
    }

    #[test]
    fn q_quits_and_ctrl_z_suspends_through_the_host() {
        let host = RecordingHost::default();
        let terminal = Terminal::new(TestBackend::new(120, 40)).unwrap();
        let mut app = App::new(
            state_with_games(120, 40, 3),
            terminal,
            Box::new(host.clone()),
            Box::new(Recorder::default()),
        );
        app.start().unwrap();
        let ctrl_z = LoopEvent::Input(vec![TimedEvent {
            event: TermEvent::Key(KeyEvent::new(KeyCode::Char('z'), KeyModifiers::CONTROL)),
            at: Instant::now(),
        }]);
        app.handle_batch(vec![ctrl_z]).unwrap();
        assert_eq!(host.suspends.load(std::sync::atomic::Ordering::SeqCst), 1);
        app.handle_batch(vec![key('q')]).unwrap();
        assert!(!app.quitting(), "the startup scan still runs");
        app.state.jobs.scan = None;
        app.handle_batch(vec![key('q')]).unwrap();
        assert!(app.quitting());
    }
}
