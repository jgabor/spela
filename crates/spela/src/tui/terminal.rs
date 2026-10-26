//! Terminal modes: enter, restore, suspend and the panic hook.
//!
//! Adopted from mejl's `terminal/mod.rs`. [`enter`] switches on raw mode,
//! the alternate screen, bracketed paste, mouse buttons and drag with SGR
//! coordinates (modes 1000, 1002 and 1006, never 1003) when the mouse is
//! on, and kitty keyboard flags only for disambiguation when the probe
//! found them. [`restore`] undoes exactly what was turned on, in reverse
//! order, and is safe to call more than once and from any thread.
//!
//! The terminal is restored on every way out: a normal quit when the
//! [`TerminalGuard`] drops, a panic that ends the process
//! ([`install_panic_hook`]), and SIGTERM, SIGINT or SIGHUP
//! ([`super::signals`]).
//!
//! The panic hook fixes mejl's gap (binary 5.3): a panic on any thread
//! that does not catch its own is fatal. It restores the terminal, prints
//! the message and exits, because a UI that looks alive with a dead thread
//! behind it is worse. Job pool threads catch their panics
//! ([`mark_thread_catches_panics`]).

use std::cell::Cell;
use std::fmt;
use std::io::{self, Write};
use std::sync::{Mutex, PoisonError};

use crossterm::event::{
    DisableBracketedPaste, DisableMouseCapture, EnableBracketedPaste, KeyboardEnhancementFlags,
    PopKeyboardEnhancementFlags, PushKeyboardEnhancementFlags,
};
use crossterm::terminal::{EnterAlternateScreen, LeaveAlternateScreen};
use crossterm::{Command, cursor, execute, terminal};

/// What to switch on when entering the TUI.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TerminalOptions {
    /// Mouse buttons, wheel and drag (modes 1000, 1002, 1006). Off with
    /// `ui.mouse = false`.
    pub mouse: bool,
    /// Bracketed paste (2004), so a pasted path never fires keys.
    pub bracketed_paste: bool,
    /// Push kitty keyboard flags. Only when the probe found support.
    pub kitty_keyboard: bool,
    /// Frames use synchronized output (mode 2026), so the way out ends it
    /// in case a frame stopped halfway.
    pub synchronized_output: bool,
}

impl Default for TerminalOptions {
    fn default() -> Self {
        TerminalOptions {
            mouse: true,
            bracketed_paste: true,
            kitty_keyboard: false,
            synchronized_output: false,
        }
    }
}

/// The kitty flags spela asks for: unambiguous Esc and modified keys.
/// Nothing depends on them; every binding works without them.
pub const KITTY_FLAGS: KeyboardEnhancementFlags =
    KeyboardEnhancementFlags::DISAMBIGUATE_ESCAPE_CODES;

/// Mouse buttons and drag with SGR coordinates: modes 1000, 1002 and 1006.
/// crossterm's `EnableMouseCapture` also turns on 1003 (every motion),
/// which floods the input thread.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct EnableMouseButtons;

impl Command for EnableMouseButtons {
    fn write_ansi(&self, f: &mut impl fmt::Write) -> fmt::Result {
        f.write_str("\x1b[?1000h\x1b[?1002h\x1b[?1006h")
    }
}

static ACTIVE: Mutex<Option<TerminalOptions>> = Mutex::new(None);

/// Restores the terminal when dropped.
#[derive(Debug)]
pub struct TerminalGuard {
    _private: (),
}

impl Drop for TerminalGuard {
    fn drop(&mut self) {
        let _ = restore();
    }
}

/// Switches the terminal into TUI mode. If a step fails, what was already
/// switched on is undone before the error returns.
pub fn enter(options: TerminalOptions) -> io::Result<TerminalGuard> {
    let mut active = ACTIVE.lock().unwrap_or_else(PoisonError::into_inner);
    if active.is_some() {
        return Err(io::Error::other("the terminal is already in TUI mode"));
    }
    *active = Some(options);
    drop(active);
    if let Err(e) = switch_on(options) {
        let _ = restore();
        return Err(e);
    }
    Ok(TerminalGuard { _private: () })
}

fn switch_on(options: TerminalOptions) -> io::Result<()> {
    terminal::enable_raw_mode()?;
    let mut out = io::stdout();
    execute!(out, EnterAlternateScreen, cursor::Hide)?;
    if options.bracketed_paste {
        execute!(out, EnableBracketedPaste)?;
    }
    if options.mouse {
        execute!(out, EnableMouseButtons)?;
    }
    if options.kitty_keyboard {
        execute!(out, PushKeyboardEnhancementFlags(KITTY_FLAGS))?;
    }
    out.flush()
}

/// Puts the terminal back as it was. Does nothing when the TUI is not
/// active. Every step runs even if an earlier one fails; the first error
/// is returned.
pub fn restore() -> io::Result<()> {
    let options = ACTIVE.lock().unwrap_or_else(PoisonError::into_inner).take();
    match options {
        Some(options) => switch_off(options),
        None => Ok(()),
    }
}

/// The panic path: never blocks on the state lock.
fn restore_after_panic() {
    let options = match ACTIVE.try_lock() {
        Ok(mut guard) => guard.take(),
        Err(std::sync::TryLockError::Poisoned(p)) => p.into_inner().take(),
        Err(std::sync::TryLockError::WouldBlock) => Some(TerminalOptions {
            mouse: true,
            bracketed_paste: true,
            kitty_keyboard: true,
            synchronized_output: true,
        }),
    };
    if let Some(options) = options {
        let _ = switch_off(options);
    }
}

fn switch_off(options: TerminalOptions) -> io::Result<()> {
    let mut out = io::stdout();
    let mut first: Option<io::Error> = None;
    let mut keep = |r: io::Result<()>| {
        if let Err(e) = r
            && first.is_none()
        {
            first = Some(e);
        }
    };
    if options.synchronized_output {
        // A frame cut short by a panic or an error may have begun a
        // synchronized update and never ended it.
        keep(out.write_all(b"\x1b[?2026l"));
    }
    if options.kitty_keyboard {
        keep(execute!(out, PopKeyboardEnhancementFlags));
    }
    if options.mouse {
        keep(execute!(out, DisableMouseCapture));
    }
    if options.bracketed_paste {
        keep(execute!(out, DisableBracketedPaste));
    }
    keep(execute!(out, LeaveAlternateScreen, cursor::Show));
    keep(out.flush());
    keep(terminal::disable_raw_mode());
    first.map_or(Ok(()), Err)
}

/// Turns the mouse modes on or off while the TUI runs, after `ui.mouse`
/// changed, and records it so [`restore`] undoes the right ones. Does
/// nothing when the TUI is not active.
pub fn set_mouse(on: bool) -> io::Result<()> {
    let mut active = ACTIVE.lock().unwrap_or_else(PoisonError::into_inner);
    let Some(options) = active.as_mut() else {
        return Ok(());
    };
    if options.mouse == on {
        return Ok(());
    }
    let mut out = io::stdout();
    if on {
        execute!(out, EnableMouseButtons)?;
    } else {
        execute!(out, DisableMouseCapture)?;
    }
    options.mouse = on;
    out.flush()
}

/// Whether the TUI mode is on.
pub fn is_active() -> bool {
    ACTIVE
        .lock()
        .unwrap_or_else(PoisonError::into_inner)
        .is_some()
}

/// Leaves TUI mode, runs `f` on the normal screen, and enters TUI mode
/// again with the same options. When the TUI is not active, `f` just runs.
/// The caller redraws the whole screen afterwards.
pub fn suspend_for<T>(f: impl FnOnce() -> T) -> io::Result<T> {
    let options = ACTIVE.lock().unwrap_or_else(PoisonError::into_inner).take();
    let Some(options) = options else {
        return Ok(f());
    };
    switch_off(options)?;
    let result = f();
    *ACTIVE.lock().unwrap_or_else(PoisonError::into_inner) = Some(options);
    if let Err(e) = switch_on(options) {
        let _ = restore();
        return Err(e);
    }
    Ok(result)
}

/// Stops the process as Ctrl-Z does in a normal program, with the terminal
/// restored while stopped. Returns once the shell resumes it with `fg`.
pub fn suspend_process() -> io::Result<()> {
    suspend_for(|| {
        // SAFETY: raise only sends a signal to this process. SIGTSTP keeps
        // its default action, which stops the process until SIGCONT.
        unsafe {
            libc::raise(libc::SIGTSTP);
        }
    })
}

thread_local! {
    static CATCHES_PANICS: Cell<bool> = const { Cell::new(false) };
}

/// Marks the current thread as one that catches its own panics, as job
/// pool threads do. A panic there is logged and the TUI keeps running.
pub fn mark_thread_catches_panics() {
    CATCHES_PANICS.with(|c| c.set(true));
}

/// Whether a panic on the current thread ends the process.
pub fn panic_is_fatal() -> bool {
    !CATCHES_PANICS.with(Cell::get)
}

/// Installs a panic hook that logs every panic and, for a fatal one,
/// restores the terminal, prints the message and, off the main thread,
/// exits with 101.
pub fn install_panic_hook() {
    let previous = std::panic::take_hook();
    std::panic::set_hook(Box::new(move |info| {
        let thread = std::thread::current();
        let fatal = panic_is_fatal();
        tracing::error!(
            thread = thread.name().unwrap_or("unnamed"),
            fatal,
            "panic: {info}"
        );
        if fatal {
            restore_after_panic();
            previous(info);
            if thread.name() != Some("main") {
                std::process::exit(101);
            }
        }
    }));
}

/// The terminal size in columns and rows, if it can be read.
pub fn size() -> Option<(u16, u16)> {
    terminal::size().ok().filter(|&(c, r)| c > 0 && r > 0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_mouse_command_has_no_motion_tracking() {
        let mut s = String::new();
        EnableMouseButtons.write_ansi(&mut s).unwrap();
        assert_eq!(s, "\x1b[?1000h\x1b[?1002h\x1b[?1006h");
        assert!(!s.contains("1003"));
    }

    #[test]
    fn restore_without_enter_is_a_no_op() {
        assert!(restore().is_ok());
        assert!(!is_active());
        assert_eq!(suspend_for(|| 7).unwrap(), 7);
    }

    #[test]
    fn marking_is_per_thread() {
        let marked = std::thread::spawn(|| {
            mark_thread_catches_panics();
            panic_is_fatal()
        })
        .join()
        .unwrap();
        assert!(!marked);
        assert!(panic_is_fatal());
    }
}
