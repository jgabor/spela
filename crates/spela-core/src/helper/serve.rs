//! The helper's two loops: `--serve` for a launch and `--once` for a live
//! command (decision 30; architecture.md, Protocol).
//!
//! Both read no file, resolve no path and log nothing: the parent logs the
//! lines it reads. The `--serve` loop takes its input as [`Event`]s, so the
//! binary can feed it stdin lines from a reader thread and SIGTERM from its
//! signal thread, and a test can feed it anything.
//!
//! `--serve` restores on every way out once it has a batch: a restore
//! request, the end of its input, a line it does not expect (a broken
//! parent), SIGTERM, and a closed output pipe (EPIPE: the parent is gone).
//! An event that arrives while it applies stops it before the next step.
//! `--once` never restores: a live change is meant to last.

use std::io::{self, BufRead, Read, Write};
use std::sync::mpsc::{Receiver, Sender, TryRecvError};

use crate::backend::{SettingValue, Step, StepOutcome, StepReport};

use super::machine::{Machine, Saved};
use super::protocol::{self, MAX_LINE, Mode, Reply};

/// What the `--serve` loop reacts to.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Event {
    /// One line from the parent, without its newline.
    Line(String),
    /// The parent closed the helper's input: it exited, panicked or was
    /// killed. Also sent for a read error or a line over [`MAX_LINE`].
    Eof,
    /// SIGTERM: restore and exit, as for [`Event::Eof`].
    Stop,
}

/// How the helper exits.
pub mod exit {
    /// Every step applied (`--once`), or every restore worked (`--serve`).
    pub const OK: u8 = 0;
    /// A step was not applied (`--once`), or a restore failed (`--serve`).
    pub const FAILURE: u8 = 1;
    /// The request was refused before any change.
    pub const REFUSED: u8 = 2;
}

/// Reads `input` line by line on its own thread and sends each line, then
/// [`Event::Eof`] at the end. The thread ends there, or when the loop has
/// gone.
pub fn spawn_reader<R: BufRead + Send + 'static>(
    mut input: R,
    tx: Sender<Event>,
) -> io::Result<std::thread::JoinHandle<()>> {
    std::thread::Builder::new()
        .name("spela-helper-input".into())
        .spawn(move || {
            loop {
                let event = match read_line(&mut input) {
                    Ok(Some(line)) => Event::Line(line),
                    Ok(None) | Err(_) => Event::Eof,
                };
                let last = event == Event::Eof;
                if tx.send(event).is_err() || last {
                    return;
                }
            }
        })
}

/// One line of at most [`MAX_LINE`] bytes, or `None` at the end. A longer
/// line is an error.
pub fn read_line(input: &mut dyn BufRead) -> io::Result<Option<String>> {
    let mut buf = Vec::new();
    let read = input
        .take(MAX_LINE as u64 + 1)
        .read_until(b'\n', &mut buf)?;
    if read == 0 {
        return Ok(None);
    }
    if buf.last() == Some(&b'\n') {
        buf.pop();
    } else if buf.len() > MAX_LINE {
        return Err(io::Error::new(io::ErrorKind::InvalidData, "line too long"));
    }
    String::from_utf8(buf)
        .map(Some)
        .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))
}

/// The `--serve` loop. The first event must be the batch. It applies the
/// batch, reports each step, writes `ready` and waits; then it restores
/// what it applied, in reverse order, and returns the exit code.
pub fn serve(events: &Receiver<Event>, out: &mut dyn Write, machine: &mut dyn Machine) -> u8 {
    let batch = match events.recv() {
        Ok(Event::Line(line)) => line,
        // The parent went before it sent anything: nothing changed.
        Ok(Event::Eof | Event::Stop) | Err(_) => return exit::OK,
    };
    let steps = match protocol::parse_request(&batch, Mode::Serve) {
        Ok(steps) => steps,
        Err(e) => {
            let _ = send(out, &Reply::Error(e));
            return exit::REFUSED;
        }
    };
    let mut applied: Vec<(Step, Saved)> = Vec::new();
    let mut waiting = true;
    for (step, target) in steps {
        let checked = machine.check(step, &target);
        if interrupted(events) {
            waiting = false;
            break;
        }
        let (report, saved) = run_step(machine, step, target, checked);
        if let Some(saved) = saved {
            applied.push((step, saved));
        }
        if send(out, &Reply::Step(report)).is_err() {
            // The parent is gone. The step is done; the restore below puts
            // it back with the rest.
            waiting = false;
            break;
        }
    }
    if waiting && send(out, &Reply::Ready).is_ok() {
        // A restore request, the end of input, SIGTERM or any other line:
        // each one restores. A second batch is never accepted.
        let _ = events.recv();
    }
    restore(&applied, out, machine)
}

/// The `--once` loop: applies `request`, reports each step and returns 0
/// when every step applied, 1 otherwise. It never restores.
pub fn once(request: &str, out: &mut dyn Write, machine: &mut dyn Machine) -> u8 {
    let steps = match protocol::parse_request(request, Mode::Once) {
        Ok(steps) => steps,
        Err(e) => {
            let _ = send(out, &Reply::Error(e));
            return exit::REFUSED;
        }
    };
    let mut all_applied = true;
    for (step, target) in steps {
        let checked = machine.check(step, &target);
        let (report, _) = run_step(machine, step, target, checked);
        all_applied &= matches!(
            report.outcome,
            StepOutcome::Applied { .. } | StepOutcome::AppliedNoRestore
        );
        if send(out, &Reply::Step(report)).is_err() {
            // The parent is gone. What applied stays, as a live change
            // does; the rest is left alone.
            return exit::FAILURE;
        }
    }
    if all_applied { exit::OK } else { exit::FAILURE }
}

/// One step: the check's result, then the prior value, then the change.
/// Returns the report line and, when the step changed something a restore
/// can put back, what it saved.
fn run_step(
    machine: &mut dyn Machine,
    step: Step,
    target: SettingValue,
    checked: Result<(), String>,
) -> (StepReport, Option<Saved>) {
    let report = |outcome| StepReport {
        step,
        target: target.clone(),
        outcome,
    };
    if let Err(why) = checked {
        return (report(StepOutcome::Skipped(why)), None);
    }
    if !step.is_restorable() {
        let outcome = match machine.apply(step, &target) {
            Ok(()) => StepOutcome::AppliedNoRestore,
            Err(e) => StepOutcome::Failed(e),
        };
        return (report(outcome), None);
    }
    let saved = match machine.read_prior(step) {
        Ok(saved) => saved,
        Err(why) => {
            return (
                report(StepOutcome::Skipped(format!(
                    "prior value not readable: {why}"
                ))),
                None,
            );
        }
    };
    match machine.apply(step, &target) {
        Ok(()) => (
            report(StepOutcome::Applied {
                prior: saved.shown(),
            }),
            Some(saved),
        ),
        Err(e) => (report(StepOutcome::Failed(e)), None),
    }
}

/// Puts back every applied step in reverse order, each on its own, so SMT
/// comes back before the governor and one failure does not stop the rest.
/// Write errors are ignored: the parent may be gone, and the restore runs
/// anyway.
fn restore(applied: &[(Step, Saved)], out: &mut dyn Write, machine: &mut dyn Machine) -> u8 {
    let mut code = exit::OK;
    for (step, saved) in applied.iter().rev() {
        let result = machine.restore(*step, saved);
        if result.is_err() {
            code = exit::FAILURE;
        }
        let _ = send(
            out,
            &Reply::Restored {
                step: *step,
                result,
            },
        );
    }
    let _ = send(out, &Reply::Done);
    code
}

/// Whether anything came from the parent or a signal while the batch is
/// applied. Each counts as a reason to stop: a restore request, the end of
/// input, SIGTERM, or a line that should not come yet.
fn interrupted(events: &Receiver<Event>) -> bool {
    match events.try_recv() {
        Ok(_) | Err(TryRecvError::Disconnected) => true,
        Err(TryRecvError::Empty) => false,
    }
}

fn send(out: &mut dyn Write, reply: &Reply) -> io::Result<()> {
    writeln!(out, "{}", protocol::encode_reply(reply))?;
    out.flush()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;
    use std::sync::mpsc;

    use crate::backend::{FanSetting, GpuControl, HardwareRequest};
    use crate::fake::{FakeGpuControl, GpuCall};
    use crate::helper::machine::SystemMachine;

    fn machine(control: &FakeGpuControl, sys: Option<&Path>) -> SystemMachine {
        let control = control.clone();
        SystemMachine::new(
            Box::new(move || Ok(Box::new(control.clone()) as Box<dyn GpuControl>)),
            sys.map(Path::to_owned),
        )
    }

    fn sysfs() -> tempfile::TempDir {
        let dir = tempfile::tempdir().unwrap();
        crate::fake::cpu::write_sysfs(dir.path()).unwrap();
        dir
    }

    fn governor(root: &Path, n: u32) -> String {
        std::fs::read_to_string(root.join(format!(
            "devices/system/cpu/cpu{n}/cpufreq/scaling_governor"
        )))
        .unwrap()
        .trim()
        .to_owned()
    }

    fn smt(root: &Path) -> String {
        std::fs::read_to_string(root.join("devices/system/cpu/smt/control"))
            .unwrap()
            .trim()
            .to_owned()
    }

    fn full_request() -> HardwareRequest {
        HardwareRequest {
            clock_offset_mhz: Some(150),
            power_limit_w: Some(250),
            fan: Some(FanSetting::Percent(70)),
            cpu_governor: Some("performance".into()),
            smt: Some(false),
            ..HardwareRequest::default()
        }
    }

    fn batch(request: &HardwareRequest) -> String {
        protocol::encode_request(&request.steps())
    }

    fn replies(out: &[u8]) -> Vec<Reply> {
        String::from_utf8(out.to_vec())
            .unwrap()
            .lines()
            .map(|l| protocol::parse_reply(l).unwrap())
            .collect()
    }

    fn types(replies: &[Reply]) -> Vec<&'static str> {
        replies
            .iter()
            .map(|r| match r {
                Reply::Step(_) => "step",
                Reply::Ready => "ready",
                Reply::Restored { .. } => "restored",
                Reply::Done => "done",
                Reply::Error(_) => "error",
            })
            .collect()
    }

    /// Runs `--serve` with `events` queued up front, so they arrive
    /// before the first step.
    fn serve_queued(events: Vec<Event>, m: &mut dyn Machine) -> (u8, Vec<Reply>) {
        let (tx, rx) = mpsc::channel();
        for e in events {
            tx.send(e).unwrap();
        }
        let mut out = Vec::new();
        let code = serve(&rx, &mut out, m);
        (code, replies(&out))
    }

    /// An output that sends each line it gets.
    struct LineSink {
        tx: mpsc::Sender<String>,
        buf: Vec<u8>,
    }

    impl Write for LineSink {
        fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
            for b in buf {
                if *b == b'\n' {
                    let line = String::from_utf8(std::mem::take(&mut self.buf)).unwrap();
                    let _ = self.tx.send(line);
                } else {
                    self.buf.push(*b);
                }
            }
            Ok(buf.len())
        }

        fn flush(&mut self) -> io::Result<()> {
            Ok(())
        }
    }

    /// Runs `--serve` on its own thread with `request`, waits for `ready`
    /// (or an error), then sends `after` and closes the input. Returns the
    /// exit code and every reply.
    fn serve_waiting(
        request: &HardwareRequest,
        after: Vec<Event>,
        mut m: SystemMachine,
    ) -> (u8, Vec<Reply>) {
        let (tx, rx) = mpsc::channel();
        let (line_tx, line_rx) = mpsc::channel();
        tx.send(Event::Line(batch(request))).unwrap();
        let helper = std::thread::spawn(move || {
            let mut out = LineSink {
                tx: line_tx,
                buf: Vec::new(),
            };
            serve(&rx, &mut out, &mut m)
        });
        let mut seen = Vec::new();
        for line in line_rx.iter() {
            let reply = protocol::parse_reply(&line).unwrap();
            let waiting = matches!(reply, Reply::Ready | Reply::Error(_) | Reply::Done);
            seen.push(reply);
            if waiting {
                break;
            }
        }
        for e in after {
            let _ = tx.send(e);
        }
        drop(tx);
        seen.extend(line_rx.iter().map(|l| protocol::parse_reply(&l).unwrap()));
        (helper.join().unwrap(), seen)
    }

    #[test]
    fn serve_applies_waits_and_restores_in_reverse_on_request() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let m = machine(&control, Some(sys.path()));
        let probe = control.clone();
        let root = sys.path().to_owned();
        // The check runs on the reply thread: once `ready` came, every
        // change is in place.
        let (tx, rx) = mpsc::channel();
        let (line_tx, line_rx) = mpsc::channel();
        tx.send(Event::Line(batch(&full_request()))).unwrap();
        let helper = std::thread::spawn(move || {
            let mut m = m;
            let mut out = LineSink {
                tx: line_tx,
                buf: Vec::new(),
            };
            serve(&rx, &mut out, &mut m)
        });
        let mut r: Vec<Reply> = Vec::new();
        for line in line_rx.iter() {
            r.push(protocol::parse_reply(&line).unwrap());
            if r.last() == Some(&Reply::Ready) {
                break;
            }
        }
        assert_eq!(probe.current(), (150, 0, 250, FanSetting::Percent(70)));
        assert_eq!(governor(&root, 3), "performance");
        assert_eq!(smt(&root), "off");
        tx.send(Event::Line(protocol::encode_restore())).unwrap();
        assert_eq!(helper.join().unwrap(), exit::OK);
        r.extend(line_rx.iter().map(|l| protocol::parse_reply(&l).unwrap()));
        assert_eq!(
            types(&r),
            [
                "step", "step", "step", "step", "step", "ready", "restored", "restored",
                "restored", "restored", "restored", "done"
            ]
        );
        let order: Vec<Step> = r
            .iter()
            .filter_map(|x| match x {
                Reply::Restored { step, result } => {
                    assert!(result.is_ok());
                    Some(*step)
                }
                _ => None,
            })
            .collect();
        assert_eq!(
            order,
            [
                Step::Smt,
                Step::CpuGovernor,
                Step::Fan,
                Step::PowerLimit,
                Step::ClockOffset
            ]
        );
        assert_eq!(control.current(), (0, 0, 320, FanSetting::Auto));
        assert_eq!(governor(sys.path(), 0), "powersave");
        assert_eq!(smt(sys.path()), "on");
    }

    #[test]
    fn end_of_input_after_the_batch_restores_like_a_request() {
        for last in [Event::Eof, Event::Stop, Event::Line("garbage".into())] {
            let sys = sysfs();
            let control = FakeGpuControl::new();
            let m = machine(&control, Some(sys.path()));
            let (code, r) = serve_waiting(&full_request(), vec![last.clone()], m);
            assert_eq!(code, exit::OK, "{last:?}");
            assert_eq!(
                types(&r).iter().filter(|t| **t == "restored").count(),
                5,
                "{last:?}"
            );
            assert_eq!(types(&r).last(), Some(&"done"), "{last:?}");
            assert_eq!(control.current(), (0, 0, 320, FanSetting::Auto), "{last:?}");
            assert_eq!(governor(sys.path(), 0), "powersave", "{last:?}");
            assert_eq!(smt(sys.path()), "on", "{last:?}");
        }
    }

    #[test]
    fn end_of_input_before_the_batch_changes_nothing() {
        let control = FakeGpuControl::new();
        let mut m = machine(&control, None);
        let (code, r) = serve_queued(vec![Event::Eof], &mut m);
        assert_eq!((code, r.len()), (exit::OK, 0));
        let (code, r) = serve_queued(vec![Event::Stop], &mut m);
        assert_eq!((code, r.len()), (exit::OK, 0));
        assert!(control.calls().is_empty());
    }

    #[test]
    fn a_bad_batch_is_refused_with_nothing_changed() {
        let control = FakeGpuControl::new();
        let mut m = machine(&control, None);
        for line in [
            "{}",
            r#"{"version":1,"type":"apply","steps":[{"step":"locked_clocks"}]}"#,
            r#"{"type":"restore"}"#,
        ] {
            let (code, r) = serve_queued(vec![Event::Line(line.into())], &mut m);
            assert_eq!(code, exit::REFUSED, "{line}");
            assert!(matches!(r.as_slice(), [Reply::Error(_)]), "{line}: {r:?}");
        }
        assert!(control.calls().is_empty());
    }

    #[test]
    fn a_failure_at_step_n_does_not_stop_the_rest_and_is_not_restored() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        // Setter call 1 is the power limit.
        control.fail_at_call(Some(1));
        let m = machine(&control, Some(sys.path()));
        let (_, r) = serve_waiting(&full_request(), vec![Event::Eof], m);
        let outcomes: Vec<(Step, bool)> = r
            .iter()
            .filter_map(|x| match x {
                Reply::Step(s) => Some((s.step, matches!(s.outcome, StepOutcome::Applied { .. }))),
                _ => None,
            })
            .collect();
        assert_eq!(
            outcomes,
            [
                (Step::ClockOffset, true),
                (Step::PowerLimit, false),
                (Step::Fan, true),
                (Step::CpuGovernor, true),
                (Step::Smt, true)
            ]
        );
        let restored: Vec<Step> = r
            .iter()
            .filter_map(|x| match x {
                Reply::Restored { step, .. } => Some(*step),
                _ => None,
            })
            .collect();
        assert!(!restored.contains(&Step::PowerLimit));
        assert_eq!(restored.len(), 4);
        assert_eq!(
            control
                .calls()
                .iter()
                .filter(|c| matches!(c, GpuCall::PowerLimit(_)))
                .count(),
            1,
            "the failed power limit is never restored"
        );
        assert_eq!(control.current(), (0, 0, 320, FanSetting::Auto));
        assert_eq!(smt(sys.path()), "on");
    }

    #[test]
    fn unknown_priors_are_skipped_and_never_restored() {
        let control = FakeGpuControl::new();
        control.set_prior_unreadable(true);
        let m = machine(&control, None);
        let (_, r) = serve_waiting(&full_request(), vec![Event::Eof], m);
        let skipped: Vec<Step> = r
            .iter()
            .filter_map(|x| match x {
                Reply::Step(s) if matches!(s.outcome, StepOutcome::Skipped(_)) => Some(s.step),
                _ => None,
            })
            .collect();
        // The power limit is skipped by its check (no range); the fan alone
        // has a fallback, automatic.
        assert_eq!(
            skipped,
            [
                Step::ClockOffset,
                Step::PowerLimit,
                Step::CpuGovernor,
                Step::Smt
            ]
        );
        assert_eq!(control.calls(), [GpuCall::FanSpeed(70), GpuCall::FanAuto]);
    }

    #[test]
    fn a_parent_that_goes_mid_apply_stops_the_batch_and_restores() {
        // An event queued behind the batch arrives before the first step.
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let mut m = machine(&control, Some(sys.path()));
        let (code, r) = serve_queued(
            vec![Event::Line(batch(&full_request())), Event::Eof],
            &mut m,
        );
        assert_eq!(code, exit::OK);
        assert_eq!(types(&r), ["done"], "stopped before any change");
        assert!(control.calls().is_empty());

        // EPIPE: the output closes after two lines.
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let mut m = machine(&control, Some(sys.path()));
        let (tx, rx) = mpsc::channel();
        tx.send(Event::Line(batch(&full_request()))).unwrap();
        let mut out = ClosesAfter {
            lines: 2,
            written: Vec::new(),
        };
        let code = serve(&rx, &mut out, &mut m);
        assert_eq!(code, exit::OK);
        // Steps 0 and 1 were reported, step 2 applied and its line failed:
        // all three are put back, and nothing after them ran.
        assert_eq!(
            control.calls(),
            [
                GpuCall::ClockOffset(150),
                GpuCall::PowerLimit(250),
                GpuCall::FanSpeed(70),
                GpuCall::FanAuto,
                GpuCall::PowerLimit(320),
                GpuCall::ClockOffset(0)
            ]
        );
        assert_eq!(governor(sys.path(), 0), "powersave");
        assert_eq!(smt(sys.path()), "on");
        drop(tx);
    }

    #[test]
    fn a_failed_restore_does_not_stop_the_others_and_exits_1() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        // Calls: clock 150, power 250, then the restore: power back fails.
        control.fail_at_call(Some(2));
        let m = machine(&control, Some(sys.path()));
        let request = HardwareRequest {
            clock_offset_mhz: Some(150),
            power_limit_w: Some(250),
            ..HardwareRequest::default()
        };
        let (code, r) = serve_waiting(&request, vec![Event::Eof], m);
        assert_eq!(code, exit::FAILURE);
        assert!(r.contains(&Reply::Restored {
            step: Step::ClockOffset,
            result: Ok(())
        }));
        assert_eq!(control.current().0, 0);
    }

    #[test]
    fn once_applies_reports_and_never_restores() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let mut m = machine(&control, Some(sys.path()));
        let mut out = Vec::new();
        let code = once(&batch(&HardwareRequest::gpu_reset()), &mut out, &mut m);
        assert_eq!(code, exit::OK);
        let r = replies(&out);
        assert_eq!(types(&r), ["step", "step", "step"]);
        assert_eq!(
            control.calls(),
            [
                GpuCall::ResetLockedClocks,
                GpuCall::ClockOffset(0),
                GpuCall::MemoryOffset(0)
            ]
        );

        // A governor sysfs does not list skips that step only, and exits 1.
        let request = HardwareRequest {
            cpu_governor: Some("turbo".into()),
            smt: Some(false),
            ..HardwareRequest::default()
        };
        let mut out = Vec::new();
        assert_eq!(once(&batch(&request), &mut out, &mut m), exit::FAILURE);
        let r = replies(&out);
        assert!(matches!(&r[0], Reply::Step(s) if matches!(s.outcome, StepOutcome::Skipped(_))));
        assert!(
            matches!(&r[1], Reply::Step(s) if matches!(s.outcome, StepOutcome::Applied { .. }))
        );
        assert_eq!(governor(sys.path(), 0), "powersave");
        assert_eq!(smt(sys.path()), "off", "a live change stays");

        let mut out = Vec::new();
        assert_eq!(once("{", &mut out, &mut m), exit::REFUSED);
        assert!(matches!(replies(&out).as_slice(), [Reply::Error(_)]));
    }

    #[test]
    fn the_reader_sends_lines_then_end_of_input() {
        let (tx, rx) = mpsc::channel();
        let input = io::Cursor::new(b"one\ntwo".to_vec());
        spawn_reader(input, tx).unwrap().join().unwrap();
        let got: Vec<Event> = rx.iter().collect();
        assert_eq!(
            got,
            [
                Event::Line("one".into()),
                Event::Line("two".into()),
                Event::Eof
            ]
        );
        let long = vec![b'x'; MAX_LINE + 5];
        let mut cursor = io::Cursor::new(long);
        assert!(read_line(&mut cursor).is_err());
    }

    #[test]
    fn a_pipe_pair_carries_the_whole_round_trip() {
        let sys = sysfs();
        let control = FakeGpuControl::new();
        let mut m = machine(&control, Some(sys.path()));
        let (to_helper_r, mut to_helper_w) = io::pipe().unwrap();
        let (mut from_helper_r, from_helper_w) = io::pipe().unwrap();
        let (tx, rx) = mpsc::channel();
        spawn_reader(io::BufReader::new(to_helper_r), tx).unwrap();
        let helper = std::thread::spawn(move || {
            let mut out = from_helper_w;
            serve(&rx, &mut out, &mut m)
        });
        writeln!(to_helper_w, "{}", batch(&full_request())).unwrap();
        let mut reader = io::BufReader::new(&mut from_helper_r);
        let mut seen = Vec::new();
        while let Some(line) = read_line(&mut reader).unwrap() {
            let reply = protocol::parse_reply(&line).unwrap();
            let ready = reply == Reply::Ready;
            seen.push(reply);
            if ready {
                break;
            }
        }
        assert_eq!(seen.len(), 6);
        assert_eq!(smt(sys.path()), "off");
        // The parent dies: its write end closes.
        drop(to_helper_w);
        assert_eq!(helper.join().unwrap(), exit::OK);
        assert_eq!(smt(sys.path()), "on");
        assert_eq!(control.current(), (0, 0, 320, FanSetting::Auto));
    }

    /// An output that takes `lines` lines and then fails like a closed
    /// pipe.
    struct ClosesAfter {
        lines: usize,
        written: Vec<u8>,
    }

    impl Write for ClosesAfter {
        fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
            let done = self.written.iter().filter(|b| **b == b'\n').count();
            if done >= self.lines {
                return Err(io::Error::from(io::ErrorKind::BrokenPipe));
            }
            self.written.extend_from_slice(buf);
            Ok(buf.len())
        }

        fn flush(&mut self) -> io::Result<()> {
            Ok(())
        }
    }
}
