//! The main loop's one channel.
//!
//! Adopted from mejl's `events.rs`. The UI thread blocks on one unbounded
//! `std::sync::mpsc` channel that carries everything it reacts to: input
//! batches from the input thread, signals, the metrics tick, wake-ups and
//! worker results. Sending never blocks, and the UI drains everything
//! pending before it draws once.

use std::sync::mpsc;
use std::time::Instant;

use super::signals::SignalKind;

/// One terminal event with the time the input thread read it. That time
/// starts the key-to-flush measurement.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TimedEvent {
    /// The event.
    pub event: crossterm::event::Event,
    /// When it was read. For a coalesced run, when the first was read.
    pub at: Instant,
}

/// Everything the main loop receives.
#[derive(Debug)]
pub enum LoopEvent<M> {
    /// A batch of terminal events, stamped at read time, coalesced, at
    /// most [`super::input::MAX_BATCH`].
    Input(Vec<TimedEvent>),
    /// The input thread stopped because reading the terminal failed.
    InputClosed(String),
    /// A signal: quit on a stop signal, repaint after SIGCONT.
    Signal(SignalKind),
    /// The metrics cadence, only while ticking is on.
    Tick,
    /// A one-off deadline.
    Wake(Instant),
    /// A worker or the write queue finished.
    Done(M),
}

/// The sending side. Every producer gets a clone.
#[derive(Debug)]
pub struct LoopSender<M> {
    tx: mpsc::Sender<LoopEvent<M>>,
}

impl<M> Clone for LoopSender<M> {
    fn clone(&self) -> Self {
        LoopSender {
            tx: self.tx.clone(),
        }
    }
}

/// The main loop hung up.
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
#[error("the main loop has stopped")]
pub struct Disconnected;

impl<M> LoopSender<M> {
    /// Sends one event. Fails only when the loop is gone.
    pub fn send(&self, event: LoopEvent<M>) -> Result<(), Disconnected> {
        self.tx.send(event).map_err(|_| Disconnected)
    }

    /// Sends a worker result.
    pub fn done(&self, msg: M) -> Result<(), Disconnected> {
        self.send(LoopEvent::Done(msg))
    }
}

/// The receiving side, owned by the UI thread.
#[derive(Debug)]
pub struct LoopReceiver<M> {
    rx: mpsc::Receiver<LoopEvent<M>>,
}

impl<M> LoopReceiver<M> {
    /// The next event if one is waiting.
    pub fn try_recv(&self) -> Option<LoopEvent<M>> {
        self.rx.try_recv().ok()
    }

    /// Blocks for one event, then takes everything already waiting, in
    /// order: one turn of the loop. Empty once every sender is gone.
    pub fn recv_batch(&self) -> Vec<LoopEvent<M>> {
        let mut batch = Vec::new();
        if let Ok(first) = self.rx.recv() {
            batch.push(first);
            batch.extend(self.rx.try_iter());
        }
        batch
    }

    /// [`LoopReceiver::recv_batch`] that gives up at `deadline`.
    pub fn recv_batch_until(&self, deadline: Instant) -> Vec<LoopEvent<M>> {
        let mut batch = Vec::new();
        let wait = deadline.saturating_duration_since(Instant::now());
        if let Ok(first) = self.rx.recv_timeout(wait) {
            batch.push(first);
            batch.extend(self.rx.try_iter());
        }
        batch
    }
}

/// A new main loop channel.
pub fn channel<M>() -> (LoopSender<M>, LoopReceiver<M>) {
    let (tx, rx) = mpsc::channel();
    (LoopSender { tx }, LoopReceiver { rx })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_batch_takes_everything_waiting_in_order() {
        let (tx, rx) = channel::<u32>();
        tx.done(1).unwrap();
        tx.send(LoopEvent::Tick).unwrap();
        tx.done(2).unwrap();
        let batch = rx.recv_batch();
        assert_eq!(batch.len(), 3);
        assert!(matches!(batch[0], LoopEvent::Done(1)));
        assert!(matches!(batch[1], LoopEvent::Tick));
        assert!(rx.try_recv().is_none());
        drop(tx);
        assert!(rx.recv_batch().is_empty());
    }
}
