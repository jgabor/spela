//! The input thread.
//!
//! Adapted from mejl's `input.rs`. A std thread blocks on a terminal read,
//! stamps each event with the time it was read, takes whatever else is
//! already waiting (at most [`MAX_BATCH`]), drops terminal replies that
//! crossterm turned into keys, folds runs of drag and resize into their
//! last event ([`coalesce`]), and sends the batch to the main loop.
//!
//! mejl waits on crossterm's `EventStream` so it can pause for `$EDITOR`.
//! spela has no editor handoff, and a stopped process stops its input
//! thread with it, so a blocking `crossterm::event::read` is enough
//! (architecture.md, the event loop; the suspend pty scenario in
//! `tests/terminal_pty.rs` checks it). The thread is never joined: it
//! reads until the process exits. Nothing else in the process may call
//! crossterm's event reader while it runs.

use std::io;
use std::time::{Duration, Instant};

use crossterm::event::{
    Event, KeyCode, KeyEvent, KeyEventKind, KeyModifiers, MouseEvent, MouseEventKind,
};

use super::events::{LoopEvent, LoopSender, TimedEvent};

/// At most this many waiting events are read in one go, so one huge paste
/// of keys cannot starve the main loop.
pub const MAX_BATCH: usize = 1024;

/// The longest terminal reply [`drop_leaked_replies`] looks for, in keys.
pub const MAX_LEAKED_REPLY: usize = 512;

/// Folds runs of events where only the last one matters: mouse motion,
/// drag with the same button and modifiers, and resizes. The merged event
/// keeps the first event's timestamp. Everything else passes in order.
pub fn coalesce(events: Vec<TimedEvent>) -> Vec<TimedEvent> {
    let mut out: Vec<TimedEvent> = Vec::with_capacity(events.len());
    for ev in events {
        if let Some(last) = out.last_mut()
            && mergeable(&last.event, &ev.event)
        {
            last.event = ev.event;
            continue;
        }
        out.push(ev);
    }
    out
}

fn mergeable(a: &Event, b: &Event) -> bool {
    match (a, b) {
        (Event::Resize(..), Event::Resize(..)) => true,
        (Event::Mouse(a), Event::Mouse(b)) => same_motion(a, b),
        _ => false,
    }
}

fn same_motion(a: &MouseEvent, b: &MouseEvent) -> bool {
    a.modifiers == b.modifiers
        && match (a.kind, b.kind) {
            (MouseEventKind::Moved, MouseEventKind::Moved) => true,
            (MouseEventKind::Drag(x), MouseEventKind::Drag(y)) => x == y,
            _ => false,
        }
}

/// Removes terminal replies that crossterm turned into keys: an OSC or
/// DCS string arrives as Alt+`]` or Alt+Shift+P, its characters, and
/// Alt+`\` or Ctrl+G. Only a whole run inside one batch is removed, so a
/// user's own Alt+`]` passes.
pub fn drop_leaked_replies(events: Vec<TimedEvent>) -> Vec<TimedEvent> {
    if !events.iter().any(|e| is_string_start(&e.event)) {
        return events;
    }
    let mut out = Vec::with_capacity(events.len());
    let mut i = 0;
    while i < events.len() {
        if is_string_start(&events[i].event)
            && let Some(len) = leaked_reply_len(&events[i + 1..])
        {
            tracing::debug!(keys = len + 1, "dropped a terminal reply read as keys");
            i += len + 1;
            continue;
        }
        out.push(events[i].clone());
        i += 1;
    }
    out
}

fn leaked_reply_len(rest: &[TimedEvent]) -> Option<usize> {
    for (n, ev) in rest.iter().take(MAX_LEAKED_REPLY).enumerate() {
        let Event::Key(key) = &ev.event else {
            return None;
        };
        if key.kind != KeyEventKind::Press {
            return None;
        }
        if is_string_end(key) {
            return Some(n + 1);
        }
        let plain = key.modifiers.difference(KeyModifiers::SHIFT).is_empty();
        match key.code {
            KeyCode::Char(c) if plain && (' '..='~').contains(&c) => {}
            _ => return None,
        }
    }
    None
}

fn is_string_start(event: &Event) -> bool {
    let Event::Key(key) = event else {
        return false;
    };
    key.kind == KeyEventKind::Press
        && match key.code {
            KeyCode::Char(']') => key.modifiers == KeyModifiers::ALT,
            KeyCode::Char('P') => key.modifiers == KeyModifiers::ALT | KeyModifiers::SHIFT,
            _ => false,
        }
}

fn is_string_end(key: &KeyEvent) -> bool {
    match key.code {
        KeyCode::Char('\\') => key.modifiers == KeyModifiers::ALT,
        KeyCode::Char('g') => key.modifiers == KeyModifiers::CONTROL,
        _ => false,
    }
}

/// Where the input thread reads events. The real one is the terminal;
/// tests use a script.
pub trait EventSource: Send + 'static {
    /// Blocks for the next event.
    fn read(&mut self) -> io::Result<Event>;
    /// Whether an event is waiting, without blocking.
    fn ready(&mut self) -> io::Result<bool>;
}

/// The terminal, through crossterm's blocking reader.
#[derive(Debug, Default)]
pub struct TerminalSource;

impl EventSource for TerminalSource {
    fn read(&mut self) -> io::Result<Event> {
        crossterm::event::read()
    }

    fn ready(&mut self) -> io::Result<bool> {
        crossterm::event::poll(Duration::ZERO)
    }
}

/// Starts the input thread on `source`. It is never joined: it ends when
/// the loop hangs up or the read fails.
pub fn spawn<S: EventSource, M: Send + 'static>(
    mut source: S,
    tx: LoopSender<M>,
) -> io::Result<()> {
    std::thread::Builder::new()
        .name("spela-input".into())
        .spawn(move || {
            loop {
                let first = match source.read() {
                    Ok(e) => e,
                    Err(e) => {
                        let _ = tx.send(LoopEvent::InputClosed(e.to_string()));
                        return;
                    }
                };
                let mut batch = vec![TimedEvent {
                    event: first,
                    at: Instant::now(),
                }];
                let mut failed = None;
                while batch.len() < MAX_BATCH {
                    match source.ready() {
                        Ok(true) => match source.read() {
                            Ok(event) => batch.push(TimedEvent {
                                event,
                                at: Instant::now(),
                            }),
                            Err(e) => {
                                failed = Some(e);
                                break;
                            }
                        },
                        Ok(false) => break,
                        Err(e) => {
                            failed = Some(e);
                            break;
                        }
                    }
                }
                // Events read before a failure still count.
                let batch = coalesce(drop_leaked_replies(batch));
                if !batch.is_empty() && tx.send(LoopEvent::Input(batch)).is_err() {
                    return;
                }
                if let Some(e) = failed {
                    let _ = tx.send(LoopEvent::InputClosed(e.to_string()));
                    return;
                }
            }
        })?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crossterm::event::MouseButton;
    use std::collections::VecDeque;
    use std::sync::{Arc, Condvar, Mutex};

    use crate::tui::events::channel;

    fn key(c: char) -> Event {
        Event::Key(KeyEvent::new(KeyCode::Char(c), KeyModifiers::NONE))
    }

    fn drag(col: u16) -> Event {
        Event::Mouse(MouseEvent {
            kind: MouseEventKind::Drag(MouseButton::Left),
            column: col,
            row: 1,
            modifiers: KeyModifiers::NONE,
        })
    }

    fn timed(events: Vec<Event>) -> Vec<TimedEvent> {
        let base = Instant::now();
        events
            .into_iter()
            .enumerate()
            .map(|(i, event)| TimedEvent {
                event,
                at: base + Duration::from_millis(i as u64),
            })
            .collect()
    }

    #[test]
    fn drags_and_resizes_fold_and_keys_pass() {
        let events = timed(vec![
            drag(1),
            drag(2),
            key('j'),
            Event::Resize(80, 24),
            Event::Resize(120, 40),
        ]);
        let first = events[0].at;
        let out = coalesce(events);
        assert_eq!(out.len(), 3);
        assert_eq!(out[0].event, drag(2));
        assert_eq!(out[0].at, first, "the first stamp stays");
        assert_eq!(out[2].event, Event::Resize(120, 40));
    }

    #[test]
    fn a_late_osc_reply_is_dropped() {
        let alt = |c| Event::Key(KeyEvent::new(KeyCode::Char(c), KeyModifiers::ALT));
        let mut events = vec![alt(']')];
        events.extend("11;rgb:0/0/0".chars().map(key));
        events.push(alt('\\'));
        events.push(key('j'));
        let out: Vec<Event> = drop_leaked_replies(timed(events))
            .into_iter()
            .map(|e| e.event)
            .collect();
        assert_eq!(out, vec![key('j')]);
        let alone = vec![alt(']')];
        assert_eq!(drop_leaked_replies(timed(alone.clone())).len(), 1);
    }

    /// A blocking source fed from the test.
    #[derive(Clone, Default)]
    struct Script(Arc<(Mutex<VecDeque<io::Result<Event>>>, Condvar)>);

    impl Script {
        fn push(&self, items: Vec<io::Result<Event>>) {
            self.0.0.lock().unwrap().extend(items);
            self.0.1.notify_all();
        }
    }

    impl EventSource for Script {
        fn read(&mut self) -> io::Result<Event> {
            let mut q = self.0.0.lock().unwrap();
            loop {
                if let Some(e) = q.pop_front() {
                    return e;
                }
                q = self.0.1.wait(q).unwrap();
            }
        }

        fn ready(&mut self) -> io::Result<bool> {
            Ok(!self.0.0.lock().unwrap().is_empty())
        }
    }

    #[test]
    fn the_thread_sends_batches_and_closes_on_an_error() {
        let script = Script::default();
        let (tx, rx) = channel::<()>();
        spawn(script.clone(), tx).unwrap();
        script.push(vec![
            Ok(key('a')),
            Ok(key('b')),
            Err(io::Error::other("tty gone")),
        ]);
        let deadline = Instant::now() + Duration::from_secs(5);
        let mut keys = Vec::new();
        let mut closed = None;
        while closed.is_none() && Instant::now() < deadline {
            for ev in rx.recv_batch_until(deadline) {
                match ev {
                    LoopEvent::Input(batch) => keys.extend(batch.into_iter().map(|t| t.event)),
                    LoopEvent::InputClosed(why) => closed = Some(why),
                    other => panic!("{other:?}"),
                }
            }
        }
        assert_eq!(keys, vec![key('a'), key('b')]);
        assert!(closed.unwrap().contains("tty gone"));
    }
}
