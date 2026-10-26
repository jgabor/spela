//! The on-demand timer: the metrics tick and one-off wake-ups.
//!
//! Adopted from mejl's `timer.rs`. An idle spela does no work: a single
//! thread sleeps on a condvar until the earliest deadline. It sends
//! [`LoopEvent::Tick`] every period while ticking is on (the 2 s metrics
//! tick) and [`LoopEvent::Wake`] at the one asked-for instant (a status
//! message that is due to clear). There is one wake-up slot: a new deadline
//! replaces the one before, so a status message that replaces another
//! never leaves the old one's wake-up queued. The loop asks for a
//! wall-clock deadline only when it changes, which fixes mejl's many
//! instants for one deadline.

use std::sync::{Arc, Condvar, Mutex, MutexGuard, PoisonError};
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

use super::events::{LoopEvent, LoopSender};

/// The metrics cadence, Go's interval (`internal/tui/header.go:81`).
pub const METRICS_EVERY: Duration = Duration::from_secs(2);

#[derive(Debug)]
struct State {
    ticking: bool,
    next_tick: Option<Instant>,
    wake: Option<Instant>,
    stop: bool,
}

#[derive(Debug)]
struct Shared {
    state: Mutex<State>,
    changed: Condvar,
}

/// The timer thread. Dropping it stops the thread.
#[derive(Debug)]
pub struct Timer {
    shared: Arc<Shared>,
    period: Duration,
    thread: Option<JoinHandle<()>>,
}

impl Timer {
    /// Starts a timer that ticks every `period` while ticking is on.
    /// Ticking starts off.
    pub fn spawn<M: Send + 'static>(tx: LoopSender<M>, period: Duration) -> std::io::Result<Timer> {
        let shared = Arc::new(Shared {
            state: Mutex::new(State {
                ticking: false,
                next_tick: None,
                wake: None,
                stop: false,
            }),
            changed: Condvar::new(),
        });
        let worker = shared.clone();
        let thread = std::thread::Builder::new()
            .name("spela-timer".into())
            .spawn(move || run(&worker, &tx, period))?;
        Ok(Timer {
            shared,
            period,
            thread: Some(thread),
        })
    }

    fn lock(&self) -> MutexGuard<'_, State> {
        self.shared
            .state
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
    }

    /// Turns ticking on or off. The same value again changes nothing, so
    /// the loop calls this after every turn.
    pub fn set_ticking(&self, on: bool) {
        let mut state = self.lock();
        if state.ticking == on {
            return;
        }
        state.ticking = on;
        state.next_tick = on.then(|| Instant::now() + self.period);
        drop(state);
        self.shared.changed.notify_one();
    }

    /// Whether ticks are on.
    pub fn is_ticking(&self) -> bool {
        self.lock().ticking
    }

    /// Asks for one wake-up at `deadline`, in place of any one not yet
    /// sent. A deadline in the past fires at once.
    pub fn wake_at(&self, deadline: Instant) {
        self.lock().wake = Some(deadline);
        self.shared.changed.notify_one();
    }

    /// Asks for a wake-up at a wall-clock time, as the UI's
    /// `next_wakeup` gives it.
    pub fn wake_at_timestamp(&self, at: jiff::Timestamp, now: jiff::Timestamp) {
        let delay = if at > now {
            Duration::try_from(at.duration_since(now)).unwrap_or(Duration::ZERO)
        } else {
            Duration::ZERO
        };
        self.wake_at(Instant::now() + delay);
    }

    /// Pending wake-ups: zero or one.
    pub fn pending_wakes(&self) -> usize {
        usize::from(self.lock().wake.is_some())
    }
}

impl Drop for Timer {
    fn drop(&mut self) {
        self.lock().stop = true;
        self.shared.changed.notify_one();
        if let Some(thread) = self.thread.take() {
            let _ = thread.join();
        }
    }
}

fn run<M>(shared: &Shared, tx: &LoopSender<M>, period: Duration) {
    let mut state = shared.state.lock().unwrap_or_else(PoisonError::into_inner);
    loop {
        if state.stop {
            return;
        }
        let now = Instant::now();
        let mut due = Vec::new();
        if let Some(at) = state.wake
            && at <= now
        {
            state.wake = None;
            due.push(LoopEvent::Wake(at));
        }
        if let Some(tick) = state.next_tick
            && tick <= now
        {
            due.push(LoopEvent::Tick);
            // Missed ticks after a stall or a suspend are skipped.
            let mut next = tick + period;
            if next <= now {
                next = now + period;
            }
            state.next_tick = Some(next);
        }
        if !due.is_empty() {
            drop(state);
            for event in due {
                if tx.send(event).is_err() {
                    return;
                }
            }
            state = shared.state.lock().unwrap_or_else(PoisonError::into_inner);
            continue;
        }
        let next = [state.next_tick, state.wake].into_iter().flatten().min();
        state = match next {
            Some(at) => {
                let wait = at.saturating_duration_since(Instant::now());
                shared
                    .changed
                    .wait_timeout(state, wait)
                    .unwrap_or_else(PoisonError::into_inner)
                    .0
            }
            None => shared
                .changed
                .wait(state)
                .unwrap_or_else(PoisonError::into_inner),
        };
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tui::events::{LoopReceiver, channel};

    fn next(rx: &LoopReceiver<()>, within: Duration) -> Option<LoopEvent<()>> {
        rx.recv_batch_until(Instant::now() + within)
            .into_iter()
            .next()
    }

    #[test]
    fn a_new_wake_replaces_the_pending_one_and_fires_once() {
        let (tx, rx) = channel::<()>();
        let timer = Timer::spawn(tx, METRICS_EVERY).unwrap();
        let now = Instant::now();
        let (a, b) = (
            now + Duration::from_millis(20),
            now + Duration::from_millis(40),
        );
        timer.wake_at(a);
        timer.wake_at(b);
        timer.wake_at(b);
        assert_eq!(timer.pending_wakes(), 1, "b replaced a");
        assert!(matches!(next(&rx, Duration::from_secs(5)), Some(LoopEvent::Wake(at)) if at == b));
        assert_eq!(timer.pending_wakes(), 0);
        assert!(next(&rx, Duration::from_millis(60)).is_none());
    }

    #[test]
    fn ticks_start_and_stop() {
        let (tx, rx) = channel::<()>();
        let timer = Timer::spawn(tx, Duration::from_millis(15)).unwrap();
        assert!(
            next(&rx, Duration::from_millis(40)).is_none(),
            "off by default"
        );
        timer.set_ticking(true);
        timer.set_ticking(true);
        assert!(timer.is_ticking());
        for _ in 0..3 {
            assert!(matches!(
                next(&rx, Duration::from_secs(5)),
                Some(LoopEvent::Tick)
            ));
        }
        timer.set_ticking(false);
        let _ = rx.try_recv();
        assert!(next(&rx, Duration::from_millis(60)).is_none());
    }
}
