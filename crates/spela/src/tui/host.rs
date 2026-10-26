//! What the loop needs from the machine around it: suspending to the
//! shell.
//!
//! Adapted from mejl's `app/host.rs`. The synchronized-output markers are
//! not here: mejl writes them through separate `io::stdout()` handles,
//! which would reorder bytes behind spela's BufWriter, so the loop writes
//! them through the backend (architecture.md, what carries over). `open`,
//! `copy` and `edit` are dropped: no spela action uses them.

/// The loop's side effects outside the UI.
pub trait Host {
    /// Stops the process as Ctrl-Z does in a shell, with the terminal
    /// restored while stopped, and returns once resumed with the TUI
    /// entered again. The caller then redraws everything.
    fn suspend(&mut self) -> Result<(), String>;

    /// Turns the terminal's mouse modes on or off.
    fn set_mouse(&mut self, on: bool) -> Result<(), String>;
}

/// The real terminal.
#[derive(Debug, Default)]
pub struct TerminalHost;

impl Host for TerminalHost {
    fn suspend(&mut self) -> Result<(), String> {
        super::terminal::suspend_process().map_err(|e| format!("cannot suspend: {e}"))
    }

    fn set_mouse(&mut self, on: bool) -> Result<(), String> {
        super::terminal::set_mouse(on).map_err(|e| format!("cannot change the mouse modes: {e}"))
    }
}

/// A host that counts what it was asked, for in-process tests.
#[derive(Debug, Default, Clone)]
pub struct RecordingHost {
    /// Times the loop asked to suspend.
    pub suspends: std::sync::Arc<std::sync::atomic::AtomicUsize>,
    /// The mouse modes asked for, in order.
    pub mouse: std::sync::Arc<std::sync::Mutex<Vec<bool>>>,
}

impl Host for RecordingHost {
    fn suspend(&mut self) -> Result<(), String> {
        self.suspends
            .fetch_add(1, std::sync::atomic::Ordering::SeqCst);
        Ok(())
    }

    fn set_mouse(&mut self, on: bool) -> Result<(), String> {
        self.mouse
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner)
            .push(on);
        Ok(())
    }
}
