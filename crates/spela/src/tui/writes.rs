//! The write queue: one thread runs profile, settings and deny list writes
//! in send order, after mejl's store thread.
//!
//! Writes for the same file that queue up merge by key, newest value per
//! key ([`spela_ui::event::Write::merge`]), and every token merged into
//! one write gets the same [`Msg::Saved`]. Each write goes through the
//! core store function, which takes `config.lock` on a fresh descriptor,
//! so flock excludes even within this process (architecture.md, TUI
//! process). On quit the loop exits, the terminal is restored, and then
//! [`WriteQueue::finish`] joins the thread off the UI path; a failed save
//! is printed after the restore.

use std::collections::BTreeMap;
use std::sync::mpsc;
use std::thread::JoinHandle;

use spela_core::denylist;
use spela_core::paths::Paths;
use spela_core::profile::{FieldKey, Scope, Value, store};
use spela_ui::Msg;
use spela_ui::event::{DenyChange, Write};

use super::events::LoopSender;

/// The running queue.
#[derive(Debug)]
pub struct WriteQueue {
    tx: Option<mpsc::Sender<(u64, Write)>>,
    thread: Option<JoinHandle<Vec<String>>>,
}

impl WriteQueue {
    /// Starts the writer thread. A profile write for a scope in `refused`
    /// fails with that text: its Go file did not migrate, and a TOML file
    /// written now would be adopted in its place, losing the Go values
    /// (see [`crate::migration::refuse_profile`]).
    pub fn spawn(
        paths: Paths,
        refused: BTreeMap<Scope, String>,
        loop_tx: LoopSender<Msg>,
    ) -> std::io::Result<WriteQueue> {
        let (tx, rx) = mpsc::channel::<(u64, Write)>();
        let thread = std::thread::Builder::new()
            .name("spela-writes".into())
            .spawn(move || {
                let mut failures = Vec::new();
                while let Ok(first) = rx.recv() {
                    let mut pending = vec![first];
                    pending.extend(rx.try_iter());
                    for (tokens, write) in merge(pending) {
                        let result = match &write {
                            Write::Profile { scope, .. } if refused.contains_key(scope) => {
                                Err(refused[scope].clone())
                            }
                            _ => apply(&paths, &write),
                        };
                        if let Err(e) = &result {
                            tracing::warn!("not saved: {e}");
                            failures.push(e.clone());
                        }
                        for token in tokens {
                            // The loop may be gone after quit; the
                            // failure list still reports it.
                            let _ = loop_tx.done(Msg::Saved {
                                token,
                                result: result.clone(),
                            });
                        }
                    }
                }
                failures
            })?;
        Ok(WriteQueue {
            tx: Some(tx),
            thread: Some(thread),
        })
    }

    /// Queues a write.
    pub fn submit(&self, token: u64, write: Write) {
        if let Some(tx) = &self.tx {
            let _ = tx.send((token, write));
        }
    }

    /// A handle that queues writes, for the command runner. The queue
    /// finishes once every handle and the queue itself let go.
    pub fn sender(&self) -> WriteSender {
        WriteSender {
            tx: self.tx.clone(),
        }
    }

    /// Lets the queue finish what it has and returns the writes that
    /// failed.
    pub fn finish(mut self) -> Vec<String> {
        self.tx.take();
        self.thread
            .take()
            .and_then(|t| t.join().ok())
            .unwrap_or_default()
    }
}

/// Queues writes on a [`WriteQueue`].
#[derive(Debug, Clone)]
pub struct WriteSender {
    tx: Option<mpsc::Sender<(u64, Write)>>,
}

impl WriteSender {
    /// Queues a write.
    pub fn submit(&self, token: u64, write: Write) {
        if let Some(tx) = &self.tx {
            let _ = tx.send((token, write));
        }
    }
}

/// Merges queued writes for the same file into the first of them, keeping
/// the order of first appearance. Each merged write lists every token it
/// answers.
pub fn merge(pending: Vec<(u64, Write)>) -> Vec<(Vec<u64>, Write)> {
    let mut out: Vec<(Vec<u64>, Write)> = Vec::new();
    for (token, write) in pending {
        let file = write.file();
        match out.iter_mut().find(|(_, w)| w.file() == file) {
            Some((tokens, merged)) => {
                tokens.push(token);
                // Same file, so the merge cannot fail.
                let _ = merged.merge(write);
            }
            None => out.push((vec![token], write)),
        }
    }
    out
}

/// Runs one write through the core store.
pub fn apply(paths: &Paths, write: &Write) -> Result<(), String> {
    match write {
        Write::Profile {
            scope,
            name,
            changes,
        } => {
            let values: Vec<(FieldKey, Value)> = changes
                .iter()
                .filter_map(|(k, v)| v.clone().map(|v| (*k, v)))
                .collect();
            let follow: Vec<FieldKey> = changes
                .iter()
                .filter(|(_, v)| v.is_none())
                .map(|(k, _)| *k)
                .collect();
            store::edit(paths, *scope, name, &values, &follow)
                .map(|_| ())
                .map_err(|e| e.to_string())
        }
        Write::Config { changes } => {
            // One write for every merged key: a bad value or a failed
            // write saves none of them, so the UI's put-back of every
            // merged token matches the disk.
            let pairs: Vec<(&str, &str)> = changes
                .iter()
                .map(|(k, v)| (k.as_str(), v.as_str()))
                .collect();
            spela_core::config::store::set_many(paths, &pairs)
                .map(|_| ())
                .map_err(|e| e.to_string())
        }
        Write::Deny {
            app_id,
            name,
            change,
        } => match change {
            DenyChange::Allow => denylist::allow(paths, *app_id),
            DenyChange::Deny { reason } => denylist::deny(paths, *app_id, name, reason),
            DenyChange::Clear => denylist::clear(paths, *app_id),
        }
        .map(|_| ())
        .map_err(|e| e.to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tui::events::{LoopEvent, channel};
    use std::time::{Duration, Instant};

    fn config(key: &str, value: &str) -> Write {
        Write::Config {
            changes: BTreeMap::from([(key.to_owned(), value.to_owned())]),
        }
    }

    #[test]
    fn writes_for_one_file_merge_and_answer_every_token() {
        let merged = merge(vec![
            (1, config("ui.mouse", "false")),
            (
                2,
                Write::Deny {
                    app_id: 9,
                    name: "x".into(),
                    change: DenyChange::Allow,
                },
            ),
            (3, config("ui.mouse", "true")),
        ]);
        assert_eq!(merged.len(), 2);
        assert_eq!(merged[0].0, vec![1, 3]);
        assert_eq!(merged[0].1, config("ui.mouse", "true"));
    }

    fn wait_saved(rx: &crate::tui::events::LoopReceiver<Msg>) -> (u64, Result<(), String>) {
        let deadline = Instant::now() + Duration::from_secs(5);
        while Instant::now() < deadline {
            for ev in rx.recv_batch_until(deadline) {
                if let LoopEvent::Done(Msg::Saved { token, result }) = ev {
                    return (token, result);
                }
            }
        }
        panic!("no save result");
    }

    #[test]
    fn the_queue_saves_and_reports_each_token() {
        let dir = tempfile::tempdir().unwrap();
        let paths = Paths::under(dir.path());
        let (tx, rx) = channel::<Msg>();
        let queue = WriteQueue::spawn(paths.clone(), BTreeMap::new(), tx).unwrap();
        queue.submit(1, config("check_updates", "false"));
        assert_eq!(wait_saved(&rx), (1, Ok(())));
        let text = std::fs::read_to_string(paths.config_file()).unwrap();
        assert!(text.contains("check_updates = false"), "{text}");
        queue.submit(2, config("manifest_refresh_hours", "not a number"));
        let (token, result) = wait_saved(&rx);
        assert_eq!(token, 2);
        assert!(result.is_err());
        assert_eq!(
            queue.finish().len(),
            1,
            "the failure is reported after quit"
        );
    }
}
