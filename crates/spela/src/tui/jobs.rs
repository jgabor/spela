//! The job pool: two threads for Steam scans, the manifest, DLL work, the
//! release check, Proton resolves, Preview launch and hardware jobs, so a
//! long download never delays a scan (decision 42).
//!
//! Every job runs under `catch_unwind` and turns a panic into an error
//! result, as mejl's render pool does. Its threads are marked as catching
//! panics, so the panic hook leaves the TUI running.

use std::panic::{AssertUnwindSafe, catch_unwind};
use std::sync::{Arc, Mutex, PoisonError, mpsc};

use super::events::LoopSender;

/// Threads in the pool.
pub const THREADS: usize = 2;

type Job = Box<dyn FnOnce() + Send>;

/// The pool. Jobs still running when the TUI quits are left to the
/// process exit.
#[derive(Debug)]
pub struct Pool {
    tx: mpsc::Sender<Job>,
}

impl Pool {
    /// Starts the threads.
    pub fn spawn(threads: usize) -> std::io::Result<Pool> {
        let (tx, rx) = mpsc::channel::<Job>();
        let rx = Arc::new(Mutex::new(rx));
        for i in 0..threads.max(1) {
            let rx = rx.clone();
            std::thread::Builder::new()
                .name(format!("spela-job-{i}"))
                .spawn(move || {
                    super::terminal::mark_thread_catches_panics();
                    loop {
                        let job = rx.lock().unwrap_or_else(PoisonError::into_inner).recv();
                        match job {
                            Ok(job) => job(),
                            Err(_) => return,
                        }
                    }
                })?;
        }
        Ok(Pool { tx })
    }

    /// Runs `job` on a pool thread and sends its result to the loop. A
    /// panic becomes `on_panic` of its message.
    pub fn submit<M: Send + 'static>(
        &self,
        tx: &LoopSender<M>,
        job: impl FnOnce() -> M + Send + 'static,
        on_panic: impl FnOnce(String) -> M + Send + 'static,
    ) {
        let tx = tx.clone();
        let run: Job = Box::new(move || {
            let msg = match catch_unwind(AssertUnwindSafe(job)) {
                Ok(msg) => msg,
                Err(payload) => {
                    let text = payload
                        .downcast_ref::<&str>()
                        .map(|s| (*s).to_owned())
                        .or_else(|| payload.downcast_ref::<String>().cloned())
                        .unwrap_or_else(|| "a worker failed".to_owned());
                    on_panic(format!("internal error: {text}"))
                }
            };
            let _ = tx.done(msg);
        });
        let _ = self.tx.send(run);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tui::events::{LoopEvent, channel};
    use std::time::{Duration, Instant};

    #[test]
    fn results_and_panics_both_come_back() {
        let pool = Pool::spawn(THREADS).unwrap();
        let (tx, rx) = channel::<Result<u32, String>>();
        pool.submit(&tx, || Ok(7), Err);
        pool.submit(&tx, || panic!("boom"), Err);
        let mut got = Vec::new();
        let deadline = Instant::now() + Duration::from_secs(5);
        while got.len() < 2 && Instant::now() < deadline {
            for ev in rx.recv_batch_until(deadline) {
                if let LoopEvent::Done(r) = ev {
                    got.push(r);
                }
            }
        }
        assert!(got.contains(&Ok(7)));
        assert!(
            got.iter()
                .any(|r| r.as_ref().is_err_and(|e| e.contains("boom")))
        );
    }
}
