//! SIGTERM, SIGINT, SIGHUP and SIGCONT for the TUI.
//!
//! Adopted from mejl's `terminal/signals.rs`, and installed first, before
//! the probe (architecture.md, startup order). In raw mode Ctrl-C is a
//! key, so these come from `kill`, a closed terminal or a resumed job. The
//! first stop signal goes to the main loop, which quits cleanly. If a
//! second one comes, the terminal is restored at once and the process
//! exits with `128 + signal`, with raw mode off even when the capability
//! probe had it on, and the demo's temp dir removed. SIGCONT goes to the
//! loop too, which repaints. Wrapper mode has its own policy and does not
//! use this.

/// A signal the TUI reacts to.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SignalKind {
    /// SIGTERM.
    Terminate,
    /// SIGINT, from `kill -INT`.
    Interrupt,
    /// SIGHUP: the terminal went away.
    Hangup,
    /// SIGCONT: resumed after a stop.
    Continue,
}

impl SignalKind {
    /// Every kind.
    pub const ALL: [SignalKind; 4] = [
        SignalKind::Terminate,
        SignalKind::Interrupt,
        SignalKind::Hangup,
        SignalKind::Continue,
    ];

    /// Whether this signal asks spela to quit.
    pub fn is_stop(self) -> bool {
        !matches!(self, SignalKind::Continue)
    }

    /// The signal number.
    pub fn number(self) -> i32 {
        match self {
            SignalKind::Terminate => libc::SIGTERM,
            SignalKind::Interrupt => libc::SIGINT,
            SignalKind::Hangup => libc::SIGHUP,
            SignalKind::Continue => libc::SIGCONT,
        }
    }

    fn from_number(n: i32) -> Option<SignalKind> {
        SignalKind::ALL.into_iter().find(|k| k.number() == n)
    }
}

/// What to do with a signal.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SignalAction {
    /// Tell the main loop.
    Forward,
    /// Restore the terminal and exit with this code now.
    ExitNow(i32),
}

/// What to do with a signal, given how many stop signals came before.
pub fn signal_action(kind: SignalKind, earlier_stops: u32) -> SignalAction {
    if kind.is_stop() && earlier_stops >= 1 {
        SignalAction::ExitNow(128 + kind.number())
    } else {
        SignalAction::Forward
    }
}

/// A running signal watcher. Dropping it stops watching.
#[derive(Debug)]
pub struct SignalWatcher {
    handle: signal_hook::iterator::Handle,
    thread: Option<std::thread::JoinHandle<()>>,
}

impl SignalWatcher {
    /// Watches the four signals on a thread and calls `forward` for each
    /// one the loop should see.
    pub fn spawn(forward: impl Fn(SignalKind) + Send + 'static) -> std::io::Result<SignalWatcher> {
        let numbers: Vec<i32> = SignalKind::ALL.iter().map(|k| k.number()).collect();
        let mut signals = signal_hook::iterator::Signals::new(&numbers)?;
        let handle = signals.handle();
        let thread = std::thread::Builder::new()
            .name("spela-signals".into())
            .spawn(move || {
                let mut stops = 0;
                for number in signals.forever() {
                    let Some(kind) = SignalKind::from_number(number) else {
                        continue;
                    };
                    match signal_action(kind, stops) {
                        SignalAction::Forward => forward(kind),
                        SignalAction::ExitNow(code) => {
                            let _ = super::terminal::restore();
                            // The probe turns raw mode on before TUI mode
                            // is recorded, so restore cannot know of it.
                            // This is a no-op when raw mode is off.
                            let _ = crossterm::terminal::disable_raw_mode();
                            // The exit skips every drop, the demo's too.
                            crate::demo::remove_now();
                            std::process::exit(code);
                        }
                    }
                    if kind.is_stop() {
                        stops += 1;
                    }
                }
            })?;
        Ok(SignalWatcher {
            handle,
            thread: Some(thread),
        })
    }
}

impl Drop for SignalWatcher {
    fn drop(&mut self) {
        self.handle.close();
        if let Some(thread) = self.thread.take() {
            let _ = thread.join();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_first_stop_is_forwarded_and_the_second_exits() {
        assert_eq!(
            signal_action(SignalKind::Terminate, 0),
            SignalAction::Forward
        );
        assert_eq!(
            signal_action(SignalKind::Hangup, 1),
            SignalAction::ExitNow(129)
        );
        assert_eq!(
            signal_action(SignalKind::Interrupt, 3),
            SignalAction::ExitNow(130)
        );
        assert_eq!(
            signal_action(SignalKind::Continue, 5),
            SignalAction::Forward
        );
        for k in SignalKind::ALL {
            assert_eq!(SignalKind::from_number(k.number()), Some(k));
        }
    }
}
