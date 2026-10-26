//! The parent's side of the helper protocol, shared by the real polkit
//! backend and the fake: send the batch, read the report lines, keep the
//! waiting helper as a [`PrivilegedSession`], and turn a helper that never
//! answered into a [`PrivilegeError`].
//!
//! Every line read is logged here, because the helper writes no log of its
//! own (architecture.md, Protocol).

use std::io::{BufRead, Write};

use crate::backend::{
    ApplyReport, HardwareRequest, PrivilegeError, PrivilegedSession, RestoreReport,
};

use super::protocol::{self, Reply};
use super::serve::read_line;

/// How the helper process, or the fake's thread, ended.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct HelperExit {
    /// The exit code, or `None` when a signal ended it.
    pub code: Option<i32>,
    /// Everything it wrote to stderr: the polkit agent's own errors.
    pub stderr: String,
}

/// Waits for the helper to end. Called once, after its input is closed.
pub type Finish = Box<dyn FnOnce() -> HelperExit + Send>;

/// Why a helper did not get to `ready` (or, for `--once`, did not report
/// every step).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StartError {
    /// It ended without a protocol line: the prompt was dismissed, polkit
    /// refused, or the helper could not run.
    Exit(HelperExit),
    /// It refused the request, with its reason. Nothing changed.
    Refused(String),
    /// It stopped partway. `restored` is what it put back on its own, when
    /// it did: the `--serve` helper restores when its parent goes.
    Stopped {
        /// The steps it reported.
        report: ApplyReport,
        /// Its restore report, when it restored before it stopped.
        restored: Option<RestoreReport>,
        /// How it ended.
        exit: HelperExit,
    },
}

impl StartError {
    /// The error the trait reports.
    pub fn into_privilege_error(self) -> PrivilegeError {
        match self {
            StartError::Exit(exit) => exit_error(&exit),
            StartError::Refused(why) => PrivilegeError::Helper(why),
            StartError::Stopped {
                report,
                restored,
                exit,
            } => {
                let mut text = format!("the helper stopped after {} step(s)", report.steps.len());
                if let Some(r) = restored {
                    text.push_str(&format!(" and put back {}", r.steps.len()));
                }
                let stderr = exit.stderr.trim();
                if !stderr.is_empty() {
                    text.push_str(&format!(": {stderr}"));
                }
                PrivilegeError::Helper(text)
            }
        }
    }
}

/// The error for a helper that ended before it wrote a protocol line. The
/// polkit agent exits 126 when the user dismisses the prompt, or says it
/// was cancelled or dismissed, as Go read it (parity-inventory.md, CLI
/// 7.1). Anything else is the agent's own text.
pub fn exit_error(exit: &HelperExit) -> PrivilegeError {
    let text = exit.stderr.trim();
    let lower = text.to_lowercase();
    if exit.code == Some(126)
        || lower.contains("authentication cancelled")
        || lower.contains("canceled")
        || lower.contains("dismissed")
    {
        return PrivilegeError::Dismissed;
    }
    if text.is_empty() {
        return PrivilegeError::Helper(match exit.code {
            Some(code) => format!("the privileged helper exited with code {code}"),
            None => "the privileged helper was killed".to_owned(),
        });
    }
    PrivilegeError::Helper(text.to_owned())
}

/// The session for a request with no steps: nothing was asked, so nothing
/// is restored and no helper runs.
pub fn empty_session() -> Box<dyn PrivilegedSession> {
    Box::new(NoChange(ApplyReport::default()))
}

struct NoChange(ApplyReport);

impl PrivilegedSession for NoChange {
    fn report(&self) -> &ApplyReport {
        &self.0
    }

    fn restore(self: Box<Self>) -> Result<RestoreReport, PrivilegeError> {
        Ok(RestoreReport::default())
    }
}

/// A waiting `--serve` helper.
///
/// [`HelperSession::finish_restore`] sends the restore request and reads
/// the helper's answer. Dropping the session without it closes the
/// helper's input, which makes the helper restore on its own, and then
/// waits for it to finish, so the restore is over before a caller lets go
/// of the launch lock.
pub struct HelperSession {
    report: ApplyReport,
    input: Option<Box<dyn Write + Send>>,
    output: Box<dyn BufRead + Send>,
    finish: Option<Finish>,
}

impl std::fmt::Debug for HelperSession {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("HelperSession")
            .field("report", &self.report)
            .finish_non_exhaustive()
    }
}

/// Sends `request` to a `--serve` helper over `input` and reads its report
/// from `output` until it is ready.
pub fn start(
    request: &HardwareRequest,
    mut input: Box<dyn Write + Send>,
    mut output: Box<dyn BufRead + Send>,
    finish: Finish,
) -> Result<HelperSession, StartError> {
    let line = protocol::encode_request(&request.steps());
    // A helper that is already gone shows as the end of its output below.
    let _ = writeln!(input, "{line}").and_then(|()| input.flush());
    let mut report = ApplyReport::default();
    loop {
        match next_reply(output.as_mut()) {
            Some(Reply::Step(s)) => report.steps.push(s),
            Some(Reply::Ready) => {
                return Ok(HelperSession {
                    report,
                    input: Some(input),
                    output,
                    finish: Some(finish),
                });
            }
            Some(Reply::Restored { step, result }) => {
                // The helper lost its input before it was ready and put
                // back what it applied.
                let mut restored = RestoreReport::default();
                restored.steps.push((step, result));
                read_restored(output.as_mut(), &mut restored);
                drop(input);
                return Err(StartError::Stopped {
                    report,
                    restored: Some(restored),
                    exit: finish(),
                });
            }
            Some(Reply::Done) => {
                drop(input);
                return Err(StartError::Stopped {
                    report,
                    restored: Some(RestoreReport::default()),
                    exit: finish(),
                });
            }
            Some(Reply::Error(why)) => {
                drop(input);
                drain(output.as_mut());
                let _ = finish();
                return Err(StartError::Refused(why));
            }
            None => {
                drop(input);
                let exit = finish();
                return Err(if report.steps.is_empty() {
                    StartError::Exit(exit)
                } else {
                    StartError::Stopped {
                        report,
                        restored: None,
                        exit,
                    }
                });
            }
        }
    }
}

/// Reads a `--once` helper's report to the end of its output. The helper
/// writes one line per step; a report with fewer lines than `request` has
/// steps means it stopped partway.
pub fn read_once(
    request: &HardwareRequest,
    output: &mut dyn BufRead,
    finish: Finish,
) -> Result<ApplyReport, StartError> {
    let expected = request.steps().len();
    let mut report = ApplyReport::default();
    let mut refused = None;
    while let Some(reply) = next_reply(output) {
        match reply {
            Reply::Step(s) => report.steps.push(s),
            Reply::Error(why) => refused = Some(why),
            other => tracing::warn!(?other, "unexpected line from the one-shot helper"),
        }
    }
    let exit = finish();
    if let Some(why) = refused {
        return Err(StartError::Refused(why));
    }
    if report.steps.is_empty() {
        return Err(StartError::Exit(exit));
    }
    if report.steps.len() < expected {
        return Err(StartError::Stopped {
            report,
            restored: None,
            exit,
        });
    }
    Ok(report)
}

impl HelperSession {
    /// What the helper applied.
    pub fn report(&self) -> &ApplyReport {
        &self.report
    }

    /// Asks the helper to restore, reads one result per restored step and
    /// waits for it to exit. A step the helper never answered for is a
    /// failure in the report.
    pub fn finish_restore(mut self) -> Result<RestoreReport, PrivilegeError> {
        let mut input = self.input.take();
        let sent = input.as_mut().is_some_and(|w| {
            writeln!(w, "{}", protocol::encode_restore())
                .and_then(|()| w.flush())
                .is_ok()
        });
        // Closing the input restores too, if the request was lost.
        drop(input);
        let mut restored = RestoreReport::default();
        read_restored(self.output.as_mut(), &mut restored);
        let exit = self.finish.take().map(|f| f()).unwrap_or_default();
        for planned in self.report.restore_plan().steps {
            if !restored.steps.iter().any(|(s, _)| *s == planned.step) {
                restored
                    .steps
                    .push((planned.step, Err("the helper did not answer".into())));
            }
        }
        if !sent && restored.steps.iter().all(|(_, r)| r.is_err()) {
            return Err(exit_error(&exit));
        }
        Ok(restored)
    }
}

impl PrivilegedSession for HelperSession {
    fn report(&self) -> &ApplyReport {
        &self.report
    }

    fn restore(self: Box<Self>) -> Result<RestoreReport, PrivilegeError> {
        (*self).finish_restore()
    }
}

impl Drop for HelperSession {
    fn drop(&mut self) {
        let Some(finish) = self.finish.take() else {
            return;
        };
        // End of input: the helper restores on its own.
        drop(self.input.take());
        let mut restored = RestoreReport::default();
        read_restored(self.output.as_mut(), &mut restored);
        tracing::info!(
            steps = restored.steps.len(),
            "the privileged helper restored when its session was dropped"
        );
        let _ = finish();
    }
}

/// Reads `restored` lines until `done` or the end of output.
fn read_restored(output: &mut dyn BufRead, into: &mut RestoreReport) {
    while let Some(reply) = next_reply(output) {
        match reply {
            Reply::Restored { step, result } => into.steps.push((step, result)),
            Reply::Done => {
                drain(output);
                return;
            }
            other => tracing::warn!(?other, "unexpected line from the helper during restore"),
        }
    }
}

/// The next reply, logged. Lines that do not parse are logged and skipped;
/// the end of output is `None`.
fn next_reply(output: &mut dyn BufRead) -> Option<Reply> {
    loop {
        let line = match read_line(output) {
            Ok(Some(line)) => line,
            Ok(None) => return None,
            Err(e) => {
                tracing::warn!("cannot read the privileged helper's output: {e}");
                return None;
            }
        };
        match protocol::parse_reply(&line) {
            Ok(reply) => {
                log_reply(&reply);
                return Some(reply);
            }
            Err(e) => tracing::warn!(line, "privileged helper: {e}"),
        }
    }
}

fn log_reply(reply: &Reply) {
    match reply {
        Reply::Step(s) => {
            tracing::info!(step = %s.step, target = %s.target, outcome = ?s.outcome, "privileged helper step");
        }
        Reply::Restored { step, result } => match result {
            Ok(()) => tracing::info!(step = %step, "privileged helper restored"),
            Err(e) => tracing::warn!(step = %step, error = %e, "privileged helper restore failed"),
        },
        Reply::Error(e) => tracing::warn!(error = %e, "privileged helper refused the request"),
        Reply::Ready | Reply::Done => tracing::info!(?reply, "privileged helper"),
    }
}

/// Reads and drops the rest of the output, so the helper never blocks on a
/// full pipe.
fn drain(output: &mut dyn BufRead) {
    while let Ok(Some(_)) = read_line(output) {}
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{self, Cursor};

    use crate::backend::{FanSetting, SettingValue, Step, StepOutcome, StepReport};

    fn lines(replies: &[Reply]) -> Box<dyn BufRead + Send> {
        let text: String = replies
            .iter()
            .map(|r| format!("{}\n", protocol::encode_reply(r)))
            .collect();
        Box::new(Cursor::new(text.into_bytes()))
    }

    fn applied(step: Step, target: SettingValue, prior: SettingValue) -> Reply {
        Reply::Step(StepReport {
            step,
            target,
            outcome: StepOutcome::Applied { prior },
        })
    }

    fn exit(code: i32, stderr: &str) -> Finish {
        let e = HelperExit {
            code: Some(code),
            stderr: stderr.into(),
        };
        Box::new(move || e)
    }

    fn request() -> HardwareRequest {
        HardwareRequest {
            power_limit_w: Some(250),
            smt: Some(false),
            ..HardwareRequest::default()
        }
    }

    #[test]
    fn dismissed_and_agent_errors_map_like_go() {
        let e = |code, text: &str| {
            exit_error(&HelperExit {
                code: Some(code),
                stderr: text.into(),
            })
        };
        assert_eq!(e(126, ""), PrivilegeError::Dismissed);
        assert_eq!(
            e(
                127,
                "Error executing command as another user: Request dismissed"
            ),
            PrivilegeError::Dismissed
        );
        assert_eq!(e(1, "Authentication CANCELLED"), PrivilegeError::Dismissed);
        assert_eq!(
            e(
                127,
                "Error executing command as another user: No authentication agent found.\n"
            ),
            PrivilegeError::Helper(
                "Error executing command as another user: No authentication agent found.".into()
            )
        );
        assert_eq!(
            e(1, ""),
            PrivilegeError::Helper("the privileged helper exited with code 1".into())
        );
    }

    #[test]
    fn start_reads_steps_until_ready_and_restore_reads_the_answer() {
        let sent: std::sync::Arc<std::sync::Mutex<Vec<u8>>> = Default::default();
        let input = SharedWriter(sent.clone());
        let output = lines(&[
            applied(
                Step::PowerLimit,
                SettingValue::Watts(250),
                SettingValue::Watts(320),
            ),
            applied(Step::Smt, SettingValue::Smt(false), SettingValue::Smt(true)),
            Reply::Ready,
            Reply::Restored {
                step: Step::Smt,
                result: Ok(()),
            },
            Reply::Restored {
                step: Step::PowerLimit,
                result: Err("busy".into()),
            },
            Reply::Done,
        ]);
        let session = start(&request(), Box::new(input), output, exit(1, "")).unwrap();
        assert_eq!(session.report().steps.len(), 2);
        let restored = session.finish_restore().unwrap();
        assert_eq!(
            restored.steps,
            [(Step::Smt, Ok(())), (Step::PowerLimit, Err("busy".into()))]
        );
        let text = String::from_utf8(sent.lock().unwrap().clone()).unwrap();
        let sent_lines: Vec<&str> = text.lines().collect();
        assert_eq!(sent_lines.len(), 2);
        assert_eq!(
            protocol::parse_request(sent_lines[0], protocol::Mode::Serve).unwrap(),
            request().steps()
        );
        assert!(protocol::is_restore(sent_lines[1]));
    }

    #[test]
    fn a_helper_that_never_answers_maps_its_exit() {
        let err = start(&request(), Box::new(io::sink()), lines(&[]), exit(126, "")).unwrap_err();
        assert_eq!(err.into_privilege_error(), PrivilegeError::Dismissed);
        let err = start(
            &request(),
            Box::new(io::sink()),
            lines(&[Reply::Error("the request has no steps".into())]),
            exit(2, ""),
        )
        .unwrap_err();
        assert_eq!(err, StartError::Refused("the request has no steps".into()));
    }

    #[test]
    fn a_helper_that_restores_before_ready_is_a_stop_with_its_restore() {
        let output = lines(&[
            applied(
                Step::PowerLimit,
                SettingValue::Watts(250),
                SettingValue::Watts(320),
            ),
            Reply::Restored {
                step: Step::PowerLimit,
                result: Ok(()),
            },
            Reply::Done,
        ]);
        let err = start(&request(), Box::new(io::sink()), output, exit(0, "")).unwrap_err();
        let StartError::Stopped {
            report, restored, ..
        } = &err
        else {
            panic!("{err:?}")
        };
        assert_eq!(report.steps.len(), 1);
        assert_eq!(restored.as_ref().unwrap().steps.len(), 1);
        assert_eq!(
            err.into_privilege_error(),
            PrivilegeError::Helper("the helper stopped after 1 step(s) and put back 1".into())
        );
    }

    #[test]
    fn a_restore_with_missing_answers_reports_them_as_failures() {
        let output = lines(&[
            applied(
                Step::PowerLimit,
                SettingValue::Watts(250),
                SettingValue::Watts(320),
            ),
            applied(Step::Smt, SettingValue::Smt(false), SettingValue::Smt(true)),
            Reply::Ready,
            Reply::Restored {
                step: Step::Smt,
                result: Ok(()),
            },
        ]);
        let session = start(&request(), Box::new(io::sink()), output, exit(0, "")).unwrap();
        let restored = session.finish_restore().unwrap();
        assert_eq!(
            restored.steps,
            [
                (Step::Smt, Ok(())),
                (Step::PowerLimit, Err("the helper did not answer".into()))
            ]
        );
    }

    #[test]
    fn read_once_needs_every_step() {
        let all = [
            applied(
                Step::PowerLimit,
                SettingValue::Watts(250),
                SettingValue::Watts(320),
            ),
            Reply::Step(StepReport {
                step: Step::Smt,
                target: SettingValue::Smt(false),
                outcome: StepOutcome::Failed("busy".into()),
            }),
        ];
        let report = read_once(&request(), lines(&all).as_mut(), exit(1, "")).unwrap();
        assert!(!report.all_applied());
        let err = read_once(&request(), lines(&all[..1]).as_mut(), exit(1, "")).unwrap_err();
        assert!(matches!(err, StartError::Stopped { .. }), "{err:?}");
        let err = read_once(&request(), lines(&[]).as_mut(), exit(126, "")).unwrap_err();
        assert_eq!(err.into_privilege_error(), PrivilegeError::Dismissed);
        let fan = HardwareRequest {
            fan: Some(FanSetting::Auto),
            ..HardwareRequest::default()
        };
        let err = read_once(
            &fan,
            lines(&[Reply::Error("bad".into())]).as_mut(),
            exit(2, ""),
        )
        .unwrap_err();
        assert_eq!(err, StartError::Refused("bad".into()));
    }

    #[test]
    fn a_dropped_session_closes_the_input_and_waits() {
        let finished = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
        let flag = finished.clone();
        let output = lines(&[
            applied(Step::Smt, SettingValue::Smt(false), SettingValue::Smt(true)),
            Reply::Ready,
            Reply::Restored {
                step: Step::Smt,
                result: Ok(()),
            },
            Reply::Done,
        ]);
        let session = start(
            &request(),
            Box::new(io::sink()),
            output,
            Box::new(move || {
                flag.store(true, std::sync::atomic::Ordering::SeqCst);
                HelperExit::default()
            }),
        )
        .unwrap();
        drop(session);
        assert!(finished.load(std::sync::atomic::Ordering::SeqCst));
    }

    struct SharedWriter(std::sync::Arc<std::sync::Mutex<Vec<u8>>>);

    impl Write for SharedWriter {
        fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
            self.0.lock().unwrap().extend_from_slice(buf);
            Ok(buf.len())
        }

        fn flush(&mut self) -> io::Result<()> {
            Ok(())
        }
    }
}
