//! The one-channel event loop.
//!
//! The shape is mejl's `app/mod.rs` (`run`, `handle_batch`, `after_turn`,
//! `draw`) with the fixes architecture.md lists. One turn:
//!
//! 1. Block for one event, then take everything already waiting.
//! 2. Coalesce runs of resize and drag events, keeping the first stamp.
//! 3. Feed each event to [`spela_ui::update`]. A stop signal quits, but
//!    a running DLL job first finishes its current step, and so does
//!    Ctrl-C or a terminal that went away during one. SIGCONT clears the
//!    screen and feeds a resize.
//! 4. Run each [`Cmd`]: the metrics thread, the job pool, the write queue,
//!    or the [`Host`] for suspend.
//! 5. `after_turn`: ticking on or off from `wants_metrics`, and one wake-up
//!    per new deadline from `next_wakeup`.
//! 6. Draw at most once, and only when the turn changed something visible
//!    (`state.dirty`). mejl draws after every batch, ticks included.
//! 7. Render into ratatui's buffer while the backend's BufWriter is empty,
//!    then write the diff into it, with the synchronized-output markers
//!    through the same writer when the probe found support, and flush.
//!    So a panic inside render leaves no bytes behind that a later drop
//!    could flush after the panic hook restored the screen. Nothing else
//!    writes to stdout while the TUI runs.
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

/// What draws one frame: [`spela_ui::render`], or a test's own.
pub type Render = fn(&mut AppState, &mut ratatui::Frame);

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
    /// Draws a frame. Tests swap it to check a render that panics.
    pub render: Render,
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
            render: spela_ui::render,
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
                    self.feed(Event::Stop);
                }
                LoopEvent::Signal(kind) => {
                    if kind.is_stop() {
                        self.feed(Event::Stop);
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
        if self.state.dirty
            && let Err(e) = self.draw(&stamps)
        {
            // While a DLL job finishes its step after a stop, a terminal
            // that went away must not end the loop before the job does.
            if !self.state.dll_jobs.stopping {
                return Err(e);
            }
            tracing::warn!("cannot draw while quitting: {e}");
        }
        Ok(())
    }

    fn feed(&mut self, ev: Event) {
        let cmds = spela_ui::update(&mut self.state, ev);
        for c in cmds {
            self.exec_cmd(c);
        }
    }

    /// Runs one command: quit, suspend and the mouse modes here, the rest
    /// on the workers.
    pub fn exec_cmd(&mut self, cmd: Cmd) {
        match cmd {
            // Ctrl-C during a DLL job: the job finishes its step first, as
            // for a stop signal. A second Ctrl-C quits at once.
            Cmd::Quit if self.state.jobs.dll.is_some() && !self.state.dll_jobs.stopping => {
                self.state.quit = false;
                self.feed(Event::Stop);
            }
            Cmd::Quit => self.quit = true,
            Cmd::Suspend => {
                if let Err(e) = self.suspend() {
                    tracing::warn!("{e}");
                }
            }
            Cmd::SetMouse(on) => {
                // The frame bytes go first, so the mode bytes never land
                // inside a frame.
                let flushed = self.terminal.backend_mut().flush();
                if let Err(e) = flushed
                    .map_err(|e| e.to_string())
                    .and_then(|()| self.host.set_mouse(on))
                {
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
        // ratatui's own `draw` with the steps split, so that nothing sits
        // in the BufWriter while the render code runs: a resize's clear is
        // flushed first, and the markers go in only after render returns.
        self.terminal.autoresize().map_err(backend_error)?;
        Backend::flush(self.terminal.backend_mut()).map_err(backend_error)?;
        {
            let mut frame = self.terminal.get_frame();
            (self.render)(&mut self.state, &mut frame);
        }
        if self.synchronized {
            self.terminal.backend_mut().write_raw(SYNC_BEGIN)?;
        }
        self.terminal.flush().map_err(backend_error)?;
        self.terminal.hide_cursor().map_err(backend_error)?;
        self.terminal.swap_buffers();
        if self.synchronized {
            self.terminal.backend_mut().write_raw(SYNC_END)?;
        }
        Backend::flush(self.terminal.backend_mut()).map_err(backend_error)?;
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
                ..GpuMetrics::default()
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

    fn idle_window(app: &mut App<TestBackend>) {
        // 30 ticks, each answered with the same sample: a 60 s idle window,
        // after the 20-sample rings are full.
        for _ in 0..30 {
            app.handle_batch(vec![LoopEvent::Tick]).unwrap();
            app.handle_batch(vec![LoopEvent::Done(steady())]).unwrap();
        }
    }

    #[test]
    fn idle_ticks_on_monitor_draw_nothing_once_the_rings_are_full() {
        let (mut app, _) = app();
        app.handle_batch(vec![key('3')]).unwrap();
        assert_eq!(app.state.dest, spela_ui::screens::Dest::Monitor);
        idle_window(&mut app);
        let before = app.counters.draws;
        idle_window(&mut app);
        assert_eq!(
            app.counters.draws, before,
            "no draw when Monitor shows the same"
        );
        // A changed value draws.
        let mut hot = steady();
        if let Msg::Metrics(s) = &mut hot
            && let Ok(g) = &mut s.gpu
        {
            g.temperature_c = 80;
        }
        app.handle_batch(vec![LoopEvent::Done(hot)]).unwrap();
        assert_eq!(app.counters.draws, before + 1);
    }

    #[test]
    fn below_80x24_nothing_but_a_resize_draws() {
        let rec = Recorder::default();
        let terminal = Terminal::new(TestBackend::new(72, 20)).unwrap();
        let mut app = App::new(
            state_with_games(72, 20, 12),
            terminal,
            Box::new(RecordingHost::default()),
            Box::new(rec.clone()),
        );
        app.start().unwrap();
        assert_eq!(app.counters.draws, 1, "the resize prompt");
        assert!(!app.state.wants_metrics(), "no tick below 80x24");
        // A late sample, a status wake, keys and a scan result change
        // nothing on the prompt.
        app.handle_batch(vec![LoopEvent::Done(steady())]).unwrap();
        app.state
            .say(spela_ui::state::Level::Info, "Rescan complete");
        app.state.dirty = false;
        app.handle_batch(vec![LoopEvent::Wake(Instant::now())])
            .unwrap();
        app.handle_batch(vec![key('q'), key('0')]).unwrap();
        assert_eq!(app.counters.draws, 1, "{:?}", app.counters);
        assert_eq!(app.counters.ticks, 0);
        assert!(
            !rec.cmds.lock().unwrap().contains(&Cmd::PollMetrics),
            "no poll below 80x24"
        );
        // A resize draws.
        app.terminal.backend_mut().resize(120, 40);
        app.handle_batch(vec![LoopEvent::Input(vec![TimedEvent {
            event: TermEvent::Resize(120, 40),
            at: Instant::now(),
        }])])
        .unwrap();
        assert_eq!(app.counters.draws, 2);
    }

    #[test]
    fn a_replaced_status_message_keeps_one_wake_queued() {
        use spela_ui::state::Level;
        let (tx, _rx) = crate::tui::events::channel::<Msg>();
        let (mut app, _) = app();
        app.timer = Some(Timer::spawn(tx, crate::tui::timer::METRICS_EVERY).unwrap());
        let pending = |app: &App<TestBackend>| app.timer.as_ref().unwrap().pending_wakes();
        app.state.say(Level::Info, "A");
        app.handle_batch(Vec::new()).unwrap();
        assert_eq!(pending(&app), 1);
        app.state.say(Level::Info, "B");
        app.handle_batch(Vec::new()).unwrap();
        assert_eq!(pending(&app), 1, "B's deadline replaced A's");
        // A wake that comes before B is due re-arms B's deadline once.
        app.handle_batch(vec![LoopEvent::Wake(Instant::now())])
            .unwrap();
        assert_eq!(pending(&app), 1);
    }

    /// A slow fake GPU and CPU reader that note every thread they run on.
    #[derive(Clone, Default)]
    struct Witness(std::sync::Arc<std::sync::Mutex<Vec<std::thread::ThreadId>>>);

    impl Witness {
        fn see(&self) {
            self.0.lock().unwrap().push(std::thread::current().id());
            std::thread::sleep(std::time::Duration::from_millis(50));
        }
    }

    struct SlowGpu(spela_core::fake::FakeGpu, Witness);

    impl spela_core::backend::Gpu for SlowGpu {
        fn info(&mut self) -> Result<spela_core::backend::GpuInfo, spela_core::backend::GpuError> {
            self.1.see();
            self.0.info()
        }

        fn metrics(&mut self) -> Result<GpuMetrics, spela_core::backend::GpuError> {
            self.1.see();
            self.0.metrics()
        }
    }

    struct SlowCpu(spela_core::fake::FakeCpu, Witness);

    impl spela_core::backend::Cpu for SlowCpu {
        fn governor(&mut self) -> Result<String, spela_core::backend::CpuError> {
            self.1.see();
            self.0.governor()
        }

        fn available_governors(&mut self) -> Result<Vec<String>, spela_core::backend::CpuError> {
            self.1.see();
            self.0.available_governors()
        }

        fn smt_active(&mut self) -> Result<bool, spela_core::backend::CpuError> {
            self.1.see();
            self.0.smt_active()
        }
    }

    #[test]
    fn slow_backends_never_run_on_the_loop_thread() {
        use crate::tui::exec::{SteamSource, Workers, monitor, profile};
        use crate::tui::metrics::{FakeCpuLoad, MetricsThread};
        use spela_core::fake::{FakeCpu, FakeGpu};

        let dir = tempfile::tempdir().unwrap();
        let paths = spela_core::paths::Paths::under(dir.path());
        let library = dir.path().join("steam");
        spela_core::fake::library::write(&library).unwrap();
        let (tx, rx) = crate::tui::events::channel::<Msg>();
        let seen = Witness::default();
        let gpu = SlowGpu(FakeGpu::demo(), seen.clone());
        let reader = profile::CpuReader::new(Box::new(SlowCpu(FakeCpu::demo(), seen.clone())));
        let metrics = MetricsThread::spawn(
            Box::new(gpu),
            Box::new(FakeCpuLoad::default()),
            Box::new(SlowCpu(FakeCpu::demo(), seen.clone())),
            tx.clone(),
        )
        .unwrap();
        let driver = metrics.driver();
        let writes = crate::tui::writes::WriteQueue::spawn(
            paths.clone(),
            std::collections::BTreeMap::new(),
            tx.clone(),
        )
        .unwrap();
        let workers = Workers {
            paths: paths.clone(),
            tx: tx.clone(),
            metrics,
            jobs: crate::tui::jobs::Pool::spawn(crate::tui::jobs::THREADS).unwrap(),
            writes: writes.sender(),
            clock: std::sync::Arc::new(spela_core::clock::SystemClock),
            steam: SteamSource::Demo(library.join("SteamLibrary")),
            cpu: reader,
            driver,
            privilege: monitor::SharedPrivilege::new(Box::new(monitor::NoPrivilege)),
            log: crate::logging::LevelHandle::default(),
            http: std::sync::Arc::new(spela_core::fake::FakeHttp::new()),
            stop: Default::default(),
            proc_root: None,
            demo: false,
        };
        drop(tx);
        let terminal = Terminal::new(TestBackend::new(120, 40)).unwrap();
        let mut app = App::new(
            state_with_games(120, 40, 12),
            terminal,
            Box::new(RecordingHost::default()),
            Box::new(workers),
        );
        app.start().unwrap();
        // The loop asks for the metrics, the governors and a scan; each
        // answer comes back on the channel.
        let deadline = Instant::now() + std::time::Duration::from_secs(20);
        while Instant::now() < deadline
            && !(app.state.metrics.gpu.is_some()
                && app.state.profile.governors.is_some()
                && app.state.jobs.scan.is_none())
        {
            let batch = rx.recv_batch_until(Instant::now() + std::time::Duration::from_millis(50));
            if !batch.is_empty() {
                app.handle_batch(batch).unwrap();
            }
        }
        assert!(app.state.metrics.gpu.is_some(), "a metrics sample arrived");
        assert!(
            app.state.profile.governors.is_some(),
            "the governors arrived"
        );
        let me = std::thread::current().id();
        let threads = seen.0.lock().unwrap().clone();
        assert!(!threads.is_empty());
        assert!(
            threads.iter().all(|t| *t != me),
            "a slow backend ran on the loop thread"
        );
        drop(app);
        writes.finish();
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

    /// A DLL update started from the first game's DLLs section, with the
    /// manifest and an empty deny list loaded.
    fn app_with_a_dll_job() -> (App<TestBackend>, u64) {
        use spela_core::dll::manifest::{CachedManifest, Manifest, ManifestEntry};
        use spela_ui::dll_jobs::{DllOp, Origin, Start};
        use spela_ui::screens::dlls;
        let (mut app, _) = app();
        let mut m = Manifest::default();
        m.dlls.insert(
            "dlss".into(),
            vec![ManifestEntry {
                version: "310.5.3".into(),
                filename: "nvngx_dlss.dll".into(),
                ..ManifestEntry::default()
            }],
        );
        app.state.jobs.scan = None;
        let request = app.state.dlls.request.unwrap();
        app.handle_batch(vec![LoopEvent::Done(Msg::Dlls(dlls::Msg::Loaded {
            request,
            result: Ok(dlls::Catalog {
                manifest: Ok(Some(CachedManifest {
                    manifest: m,
                    ..CachedManifest::default()
                })),
                cached: Default::default(),
                note: None,
            }),
        }))])
        .unwrap();
        let cmds = spela_ui::update::run(
            &mut app.state,
            spela_ui::screens::Action::Dll(Start {
                op: DllOp::AutoUpdate,
                origin: Origin::Auto,
            }),
        );
        let Some(Cmd::Dll { token, .. }) = cmds.into_iter().next() else {
            panic!("a DLL job")
        };
        (app, token)
    }

    fn dll_done(token: u64) -> LoopEvent<Msg> {
        use spela_ui::dll_jobs::{DllDone, DllResult};
        LoopEvent::Done(Msg::DllDone {
            token,
            done: Box::new(DllDone {
                result: DllResult::Auto(Default::default()),
                games: None,
            }),
        })
    }

    #[test]
    fn a_stop_signal_waits_for_the_dll_job_to_finish_its_step() {
        use crate::tui::signals::SignalKind;
        let (mut app, token) = app_with_a_dll_job();
        app.handle_batch(vec![LoopEvent::Signal(SignalKind::Terminate)])
            .unwrap();
        assert!(!app.quitting(), "the job finishes its step first");
        assert!(app.state.dll_jobs.stopping);
        app.handle_batch(vec![dll_done(token)]).unwrap();
        assert!(app.quitting());
    }

    #[test]
    fn ctrl_c_during_a_dll_job_waits_once_and_quits_on_the_second() {
        let ctrl_c = || {
            LoopEvent::Input(vec![TimedEvent {
                event: TermEvent::Key(KeyEvent::new(KeyCode::Char('c'), KeyModifiers::CONTROL)),
                at: Instant::now(),
            }])
        };
        let (mut app, _) = app_with_a_dll_job();
        app.handle_batch(vec![ctrl_c()]).unwrap();
        assert!(!app.quitting());
        app.handle_batch(vec![ctrl_c()]).unwrap();
        assert!(app.quitting(), "the second Ctrl-C quits at once");
        let (mut app, token2) = app_with_a_dll_job();
        app.handle_batch(vec![ctrl_c()]).unwrap();
        app.handle_batch(vec![dll_done(token2)]).unwrap();
        assert!(app.quitting());
        // A terminal that went away waits for the job too.
        let (mut app, token3) = app_with_a_dll_job();
        app.handle_batch(vec![LoopEvent::InputClosed("closed".into())])
            .unwrap();
        assert!(!app.quitting());
        app.handle_batch(vec![dll_done(token3)]).unwrap();
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
        app.exec_cmd(Cmd::SetMouse(false));
        assert_eq!(*host.mouse.lock().unwrap(), vec![false]);
        app.handle_batch(vec![key('q')]).unwrap();
        assert!(!app.quitting(), "the startup scan still runs");
        app.state.jobs.scan = None;
        app.handle_batch(vec![key('q')]).unwrap();
        assert!(app.quitting());
    }
}
