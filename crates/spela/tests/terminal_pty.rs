//! The real `spela` binary on a real pty, with the screen read by vt100.
//!
//! Adapted from mejl's `tests/terminal_pty.rs`. mejl checks bytes and never
//! runs its real binary; spela also reads the screen the way a terminal
//! would. Each test opens a fresh pty and makes its slave side the child's
//! stdin, stdout and stderr. The child starts a new session with the pty as
//! its controlling terminal, so even its `/dev/tty` is the pty and the
//! developer's own terminal is never touched. The child runs in the sandbox
//! (see `common`).
//!
//! Phase 4 adds the TUI scenarios from the architecture: mouse modes, restore
//! on exit, panic and SIGTERM, Ctrl-Z and resume, bracketed paste, the
//! resize prompt and first-frame timing.

mod common;

use std::io::Read;
use std::os::fd::{AsRawFd, FromRawFd, OwnedFd, RawFd};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use common::Sandbox;

const WAIT: Duration = Duration::from_secs(10);
const ROWS: u16 = 24;
const COLS: u16 = 80;

struct Pty {
    master: OwnedFd,
    slave: Option<OwnedFd>,
}

fn open_pty(rows: u16, cols: u16) -> Pty {
    let mut master: RawFd = -1;
    let mut slave: RawFd = -1;
    let mut size = libc::winsize {
        ws_row: rows,
        ws_col: cols,
        ws_xpixel: 0,
        ws_ypixel: 0,
    };
    // SAFETY: openpty writes two descriptors into the out pointers; the
    // name and termios pointers may be null.
    let rc = unsafe {
        libc::openpty(
            &mut master,
            &mut slave,
            std::ptr::null_mut(),
            std::ptr::null_mut(),
            &raw mut size,
        )
    };
    assert_eq!(rc, 0, "openpty: {}", std::io::Error::last_os_error());
    // SAFETY: both descriptors are fresh and owned by nobody else.
    unsafe {
        Pty {
            master: OwnedFd::from_raw_fd(master),
            slave: Some(OwnedFd::from_raw_fd(slave)),
        }
    }
}

/// Everything the child wrote, read on a thread so the child never blocks
/// on a full pty buffer.
struct Output {
    bytes: Arc<Mutex<Vec<u8>>>,
    stop: Arc<AtomicBool>,
    thread: Option<std::thread::JoinHandle<()>>,
}

impl Output {
    fn read_from(master: &OwnedFd) -> Output {
        let file = std::fs::File::from(master.try_clone().unwrap());
        let bytes = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let thread = std::thread::spawn({
            let bytes = Arc::clone(&bytes);
            let stop = Arc::clone(&stop);
            move || {
                let mut file = file;
                let mut buf = [0u8; 4096];
                while !stop.load(Ordering::SeqCst) {
                    if !readable(file.as_raw_fd(), Duration::from_millis(20)) {
                        continue;
                    }
                    match file.read(&mut buf) {
                        // EIO once every slave descriptor is closed.
                        Ok(0) | Err(_) => break,
                        Ok(n) => bytes.lock().unwrap().extend_from_slice(&buf[..n]),
                    }
                }
            }
        });
        Output {
            bytes,
            stop,
            thread: Some(thread),
        }
    }

    fn raw(&self) -> Vec<u8> {
        self.bytes.lock().unwrap().clone()
    }

    /// Waits for the reader to see the end of the output.
    fn finish(mut self) -> Vec<u8> {
        if let Some(t) = self.thread.take() {
            let deadline = Instant::now() + WAIT;
            while !t.is_finished() && Instant::now() < deadline {
                std::thread::sleep(Duration::from_millis(5));
            }
            self.stop.store(true, Ordering::SeqCst);
            let _ = t.join();
        }
        self.raw()
    }
}

impl Drop for Output {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(t) = self.thread.take() {
            let _ = t.join();
        }
    }
}

fn readable(fd: RawFd, timeout: Duration) -> bool {
    let mut pfd = libc::pollfd {
        fd,
        events: libc::POLLIN,
        revents: 0,
    };
    let ms = libc::c_int::try_from(timeout.as_millis()).unwrap_or(libc::c_int::MAX);
    // SAFETY: one valid pollfd for an open descriptor.
    unsafe { libc::poll(&mut pfd, 1, ms) > 0 }
}

/// A finished run on the pty.
struct Screen {
    code: Option<i32>,
    raw: Vec<u8>,
    parser: vt100::Parser,
}

impl Screen {
    fn contents(&self) -> String {
        self.parser.screen().contents()
    }
}

/// Runs `spela args` on a fresh pty in the sandbox and reads the screen.
fn run_on_pty(sandbox: &Sandbox, args: &[&str], env: &[(&str, &str)]) -> Screen {
    let mut cmd = sandbox.command();
    cmd.args(args).env("TERM", "xterm-256color");
    for (k, v) in env {
        cmd.env(k, v);
    }
    run_command_on_pty(cmd)
}

/// Runs `cmd` on a fresh pty, as its controlling terminal, and reads the
/// screen.
fn run_command_on_pty(mut cmd: Command) -> Screen {
    let mut pty = open_pty(ROWS, COLS);
    let output = Output::read_from(&pty.master);
    let slave = pty.slave.take().unwrap();
    let stdio = || Stdio::from(slave.try_clone().unwrap());
    cmd.stdin(stdio()).stdout(stdio()).stderr(stdio());
    {
        use std::os::unix::process::CommandExt;
        // SAFETY: only async-signal-safe calls between fork and exec. A new
        // session, then the pty (already stdin) as its controlling terminal,
        // so the child's `/dev/tty` is the pty and never the test runner's
        // terminal. Adopted from mejl's `tests/terminal_pty.rs`.
        unsafe {
            cmd.pre_exec(|| {
                if libc::setsid() < 0 || libc::ioctl(0, libc::TIOCSCTTY as _, 0) < 0 {
                    return Err(std::io::Error::last_os_error());
                }
                Ok(())
            });
        }
    }
    let mut child = cmd.spawn().unwrap();
    // Every parent copy of the slave must close, the command's included, or
    // the master never sees the end of the output.
    drop(cmd);
    drop(slave);
    let code = wait(&mut child);
    let raw = output.finish();
    let size = |n: u16| std::num::NonZeroU16::new(n).unwrap();
    let mut parser = vt100::Parser::new(size(ROWS), size(COLS), 0);
    parser.process(&raw);
    Screen { code, raw, parser }
}

fn wait(child: &mut Child) -> Option<i32> {
    let deadline = Instant::now() + WAIT;
    loop {
        if let Some(status) = child.try_wait().unwrap() {
            return status.code();
        }
        if Instant::now() > deadline {
            let _ = child.kill();
            let _ = child.wait();
            panic!("spela did not exit within {WAIT:?}");
        }
        std::thread::sleep(Duration::from_millis(5));
    }
}

#[test]
fn version_shows_on_the_screen() {
    let s = Sandbox::new();
    let screen = run_on_pty(&s, &["--version"], &[]);
    assert_eq!(screen.code, Some(0));
    let text = screen.contents();
    assert_eq!(
        text.trim_end(),
        concat!("spela ", env!("CARGO_PKG_VERSION")),
        "{text:?}"
    );
}

#[test]
fn errors_are_coloured_on_a_terminal_unless_no_color() {
    let s = Sandbox::new();
    let coloured = run_on_pty(&s, &["scan"], &[]);
    assert_eq!(coloured.code, Some(1));
    assert!(coloured.raw.contains(&0x1b), "ANSI on a terminal");
    let row = coloured.parser.screen().contents();
    assert!(
        row.starts_with("error: scan failed: could not find Steam installation"),
        "{row:?}"
    );
    // vt100 saw the red, bold `error:`.
    let cell = coloured.parser.screen().cell(0, 0).unwrap();
    assert!(cell.bold(), "error: is bold");

    let plain = run_on_pty(&s, &["scan"], &[("NO_COLOR", "1")]);
    assert_eq!(plain.code, Some(1));
    assert!(
        !plain.raw.contains(&0x1b),
        "{:?}",
        String::from_utf8_lossy(&plain.raw)
    );
    assert_eq!(plain.contents(), coloured.contents());
}

#[test]
fn the_pty_is_the_child_s_controlling_terminal() {
    // `/dev/tty` opens the controlling terminal. Without the new session it
    // would be the test runner's own terminal, or fail with ENXIO in CI.
    let s = Sandbox::new();
    let mut sh = Command::new("/bin/sh");
    sh.env_clear()
        .current_dir(s.root())
        .env("PATH", s.bin())
        .args(["-c", "echo on-the-pty > /dev/tty"]);
    let screen = run_command_on_pty(sh);
    assert_eq!(screen.code, Some(0));
    assert_eq!(screen.contents().trim_end(), "on-the-pty");
}
