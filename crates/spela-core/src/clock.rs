//! The injected clock.
//!
//! Code that needs the time asks one [`Clock`], so tests and snapshots use a
//! fixed or manually advanced time and give the same result on every
//! machine. Adapted from mejl-ui's `clock.rs`.

use std::fmt;
use std::sync::Mutex;

use jiff::{SignedDuration, Timestamp};

/// A source of the current time.
pub trait Clock: Send + Sync + fmt::Debug {
    /// The current instant.
    fn now(&self) -> Timestamp;
}

/// The real wall clock.
#[derive(Clone, Copy, Debug, Default)]
pub struct SystemClock;

impl Clock for SystemClock {
    fn now(&self) -> Timestamp {
        Timestamp::now()
    }
}

/// A clock that only moves when told to. Used by tests and snapshots.
#[derive(Debug)]
pub struct ManualClock {
    now: Mutex<Timestamp>,
}

impl ManualClock {
    /// A clock stopped at `now`.
    pub fn new(now: Timestamp) -> Self {
        ManualClock {
            now: Mutex::new(now),
        }
    }

    /// Moves the clock to `now`.
    pub fn set(&self, now: Timestamp) {
        *self.lock() = now;
    }

    /// Moves the clock forward, or back for a negative duration. Saturates
    /// at the ends of jiff's supported range.
    pub fn advance(&self, by: SignedDuration) {
        let mut guard = self.lock();
        *guard = guard.checked_add(by).unwrap_or(*guard);
    }

    fn lock(&self) -> std::sync::MutexGuard<'_, Timestamp> {
        // A panic while holding this lock cannot leave a Timestamp half
        // written, so a poisoned lock is still safe to use.
        self.now.lock().unwrap_or_else(|e| e.into_inner())
    }
}

impl Clock for ManualClock {
    fn now(&self) -> Timestamp {
        *self.lock()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn manual_clock_moves_only_when_told() {
        let start: Timestamp = "2026-10-26T14:02:31Z".parse().unwrap();
        let clock = ManualClock::new(start);
        assert_eq!(clock.now(), start);
        clock.advance(SignedDuration::from_millis(250));
        assert_eq!(clock.now(), start + SignedDuration::from_millis(250));
        clock.advance(SignedDuration::from_secs(-1));
        assert_eq!(clock.now(), start - SignedDuration::from_millis(750));
        clock.set(start);
        assert_eq!(clock.now(), start);
    }

    #[test]
    fn manual_clock_saturates_at_the_end_of_time() {
        let clock = ManualClock::new(Timestamp::MAX);
        clock.advance(SignedDuration::from_secs(1));
        assert_eq!(clock.now(), Timestamp::MAX);
    }

    #[test]
    fn system_clock_moves() {
        let a = SystemClock.now();
        let b = SystemClock.now();
        assert!(b >= a);
    }
}
