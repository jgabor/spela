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
//! The TUI scenarios from the architecture:
//!
//! - the real binary under `--demo`: mouse modes 1000, 1002 and 1006 but
//!   never 1003, and none with `ui.mouse = false`; restore on Ctrl-C and on
//!   SIGTERM; bracketed paste into Find; the resize prompt; the first frame,
//!   with its time reported;
//! - a child of this test binary (`pty_child`) for Ctrl-Z: spela's blocking
//!   input thread survives a stop and a resume, and the next key arrives
//!   once with no ghost keys (architecture.md, the event loop).
//!
//! The harness answers the DA1 probe at once, so timings do not include a
//! probe timeout.

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

/// A live run of the real binary on a pty, for scenarios that type keys.
struct Live {
    child: Option<Child>,
    output: Output,
    master: OwnedFd,
    answered: bool,
    started: Instant,
    rows: u16,
    cols: u16,
}

impl Live {
    fn start(sandbox: &Sandbox, args: &[&str], rows: u16, cols: u16, env: &[(&str, &str)]) -> Live {
        let mut cmd = sandbox.command();
        cmd.args(args).env("TERM", "xterm-256color");
        for (k, v) in env {
            cmd.env(k, v);
        }
        let mut pty = open_pty(rows, cols);
        let output = Output::read_from(&pty.master);
        let slave = pty.slave.take().unwrap();
        let stdio = || Stdio::from(slave.try_clone().unwrap());
        cmd.stdin(stdio()).stdout(stdio()).stderr(stdio());
        {
            use std::os::unix::process::CommandExt;
            // SAFETY: only async-signal-safe calls between fork and exec: a
            // new session with the pty as its controlling terminal.
            unsafe {
                cmd.pre_exec(|| {
                    if libc::setsid() < 0 || libc::ioctl(0, libc::TIOCSCTTY as _, 0) < 0 {
                        return Err(std::io::Error::last_os_error());
                    }
                    Ok(())
                });
            }
        }
        let started = Instant::now();
        let child = cmd.spawn().unwrap();
        drop(cmd);
        drop(slave);
        Live {
            child: Some(child),
            output,
            master: pty.master,
            answered: false,
            started,
            rows,
            cols,
        }
    }

    fn raw(&self) -> Vec<u8> {
        self.output.raw()
    }

    fn text(&self) -> String {
        String::from_utf8_lossy(&self.raw()).into_owned()
    }

    fn type_bytes(&self, bytes: &[u8]) {
        use std::io::Write;
        let mut master = std::fs::File::from(self.master.try_clone().unwrap());
        master.write_all(bytes).unwrap();
    }

    /// Answers the probe's DA1 the moment it shows, as a fast terminal
    /// does.
    fn pump(&mut self) {
        if !self.answered && self.text().contains("\x1b[c") {
            self.type_bytes(b"\x1b[?62;22c");
            self.answered = true;
        }
    }

    fn screen(&self) -> String {
        let size = |n: u16| std::num::NonZeroU16::new(n).unwrap();
        let mut parser = vt100::Parser::new(size(self.rows), size(self.cols), 0);
        parser.process(&self.raw());
        parser.screen().contents()
    }

    /// Waits for `needle` on the screen and returns the time since spawn.
    fn wait_screen(&mut self, needle: &str) -> Duration {
        let deadline = Instant::now() + WAIT;
        loop {
            self.pump();
            if self.screen().contains(needle) {
                return self.started.elapsed();
            }
            assert!(
                Instant::now() < deadline,
                "no {needle:?} on the screen:\n{}\nraw: {:?}",
                self.screen(),
                self.text()
            );
            std::thread::sleep(Duration::from_millis(5));
        }
    }

    fn pid(&self) -> libc::pid_t {
        libc::pid_t::try_from(self.child.as_ref().unwrap().id()).unwrap()
    }

    /// Waits for the exit and returns the code and everything written.
    fn finish(mut self) -> (Option<i32>, String) {
        let mut child = self.child.take().unwrap();
        let code = wait(&mut child);
        let raw = std::mem::replace(&mut self.output, Output::empty()).finish();
        (code, String::from_utf8_lossy(&raw).into_owned())
    }
}

impl Drop for Live {
    fn drop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

impl Output {
    fn empty() -> Output {
        Output {
            bytes: Arc::new(Mutex::new(Vec::new())),
            stop: Arc::new(AtomicBool::new(true)),
            thread: None,
        }
    }
}

/// Asserts that `needles` appear in `text` in this order.
fn in_order(text: &str, needles: &[&str]) {
    let mut from = 0;
    for n in needles {
        match text[from..].find(n) {
            Some(at) => from += at + n.len(),
            None => panic!("{n:?} missing or out of order in {text:?}"),
        }
    }
}

#[test]
fn the_tui_enters_and_restores_every_mode() {
    let s = Sandbox::new();
    let mut live = Live::start(&s, &["--demo", "tui"], 40, 120, &[]);
    let first = live.wait_screen("Library");
    live.wait_screen("Rescan complete: 12 games found");
    let screen = live.screen();
    assert!(screen.contains("Ashfall Courier"), "{screen}");
    eprintln!("first frame after {first:?} (debug build, report only)");
    live.type_bytes(b"\x03");
    let (code, out) = live.finish();
    assert_eq!(code, Some(0), "{out:?}");
    in_order(
        &out,
        &[
            "\x1b[?1049h",
            "\x1b[?2004h",
            "\x1b[?1000h\x1b[?1002h\x1b[?1006h",
            "\x1b[?1000l",
            "\x1b[?2004l",
            "\x1b[?1049l",
            "\x1b[?25h",
        ],
    );
    assert!(!out.contains("1003h"), "no motion tracking");
}

#[test]
fn mouse_off_leaves_selection_to_the_terminal() {
    let s = Sandbox::new();
    std::fs::create_dir_all(s.config_dir()).unwrap();
    std::fs::write(s.config_dir().join("config.toml"), "[ui]\nmouse = false\n").unwrap();
    let mut live = Live::start(&s, &["--demo", "tui"], 24, 80, &[]);
    live.wait_screen("Library");
    live.type_bytes(b"Q");
    // The startup scan may still run, which refuses Q; Ctrl-C always works.
    live.type_bytes(b"\x03");
    let (code, out) = live.finish();
    assert_eq!(code, Some(0));
    assert!(out.contains("\x1b[?1049h"));
    assert!(!out.contains("\x1b[?1000h"), "{out:?}");
}

#[test]
fn sigterm_quits_and_restores() {
    let s = Sandbox::new();
    let mut live = Live::start(&s, &["--demo", "tui"], 24, 80, &[]);
    live.wait_screen("Library");
    // SAFETY: SIGTERM to our own child.
    assert_eq!(unsafe { libc::kill(live.pid(), libc::SIGTERM) }, 0);
    let (code, out) = live.finish();
    assert_eq!(code, Some(0));
    let after = &out[out.find("\x1b[?1049h").unwrap()..];
    in_order(after, &["\x1b[?1049l", "\x1b[?25h"]);
}

#[test]
fn a_paste_types_into_find_and_fires_no_keys() {
    let s = Sandbox::new();
    let mut live = Live::start(&s, &["--demo", "tui"], 24, 80, &[]);
    live.wait_screen("Rescan complete");
    live.type_bytes(b"/");
    // `q` and `2` would quit or switch destination if they fired as keys.
    live.type_bytes(b"\x1b[200~q2 Lantern\x1b[201~");
    live.wait_screen("/ q2 Lantern");
    let screen = live.screen();
    assert!(screen.contains("No games match"), "{screen}");
    assert!(screen.contains("1 Library") || screen.contains("Library"));
    live.type_bytes(b"\x03");
    assert_eq!(live.finish().0, Some(0));
}

#[test]
fn below_80x24_only_the_resize_prompt_shows() {
    let s = Sandbox::new();
    let mut live = Live::start(&s, &["--demo", "tui"], 20, 72, &[]);
    live.wait_screen("Resize terminal");
    let screen = live.screen();
    assert!(screen.contains("spela needs at least 80x24"), "{screen}");
    assert!(screen.contains("Current size: 72x20"), "{screen}");
    live.type_bytes(b"q0");
    std::thread::sleep(Duration::from_millis(100));
    assert!(
        live.screen().contains("Resize terminal"),
        "q and 0 do nothing here"
    );
    live.type_bytes(b"\x03");
    assert_eq!(live.finish().0, Some(0));
}

#[test]
fn without_a_terminal_the_tui_says_so() {
    let s = Sandbox::new();
    let run = s.run(&["tui"]);
    run.expect_code(1);
    assert!(
        run.stderr()
            .starts_with("error: spela tui needs a terminal"),
        "{}",
        run.stderr()
    );
}

// Ctrl-Z with spela's blocking input thread, on a child of this binary.

const SCENARIO: &str = "SPELA_PTY_SCENARIO";

struct Running {
    child: Option<Child>,
    output: Output,
    pty: Pty,
    slave: OwnedFd,
}

impl Running {
    fn start(scenario: &str) -> Running {
        let mut pty = open_pty(ROWS, COLS);
        let output = Output::read_from(&pty.master);
        let slave = pty.slave.take().unwrap();
        let stdio = || Stdio::from(slave.try_clone().unwrap());
        let mut cmd = Command::new(std::env::current_exe().unwrap());
        cmd.args(["--exact", "pty_child", "--nocapture", "--test-threads=1"])
            .env(SCENARIO, scenario)
            .stdin(stdio())
            .stdout(stdio())
            .stderr(stdio());
        let child = cmd.spawn().unwrap();
        Running {
            child: Some(child),
            output,
            pty,
            slave,
        }
    }

    fn wait_for(&self, needle: &str) -> String {
        let deadline = Instant::now() + WAIT;
        loop {
            let text = String::from_utf8_lossy(&self.output.raw()).into_owned();
            if let Some(at) = text.find(needle) {
                return text[..at + needle.len()].to_owned();
            }
            assert!(Instant::now() < deadline, "no {needle:?}; got {text:?}");
            std::thread::sleep(Duration::from_millis(5));
        }
    }

    fn type_keys(&self, bytes: &[u8]) {
        use std::io::Write;
        let mut master = std::fs::File::from(self.pty.master.try_clone().unwrap());
        master.write_all(bytes).unwrap();
    }

    fn raw_mode(&self) -> bool {
        // SAFETY: termios is plain data, filled in by tcgetattr.
        let mut t: libc::termios = unsafe { std::mem::zeroed() };
        // SAFETY: a valid descriptor and a valid termios pointer.
        let rc = unsafe { libc::tcgetattr(self.slave.as_raw_fd(), &mut t) };
        assert_eq!(rc, 0);
        t.c_lflag & (libc::ICANON | libc::ECHO) == 0
    }

    fn pid(&self) -> libc::pid_t {
        libc::pid_t::try_from(self.child.as_ref().unwrap().id()).unwrap()
    }

    fn finish(mut self) {
        let mut child = self.child.take().unwrap();
        let code = wait(&mut child);
        assert_eq!(
            code,
            Some(0),
            "child failed: {:?}",
            String::from_utf8_lossy(&self.output.raw())
        );
    }
}

impl Drop for Running {
    fn drop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

#[test]
fn a_blocking_read_survives_ctrl_z_and_resume() {
    let run = Running::start("suspend_blocking_read");
    run.wait_for("<entered>");
    assert!(run.raw_mode());
    run.type_keys(b"a");
    let pid = run.pid();
    let deadline = Instant::now() + WAIT;
    let status = loop {
        let mut status = 0;
        // SAFETY: waits on our own child; WNOHANG never blocks.
        let rc = unsafe { libc::waitpid(pid, &mut status, libc::WUNTRACED | libc::WNOHANG) };
        if rc == pid {
            break status;
        }
        assert!(Instant::now() < deadline, "the child did not stop");
        std::thread::sleep(Duration::from_millis(5));
    };
    assert!(libc::WIFSTOPPED(status), "stopped, not exited: {status}");
    assert!(
        !run.raw_mode(),
        "the shell gets a cooked terminal while stopped"
    );
    // SAFETY: SIGCONT to our own stopped child.
    assert_eq!(unsafe { libc::kill(pid, libc::SIGCONT) }, 0);
    run.wait_for("<resumed>");
    assert!(run.raw_mode(), "TUI mode again after fg");
    run.type_keys(b"b");
    run.wait_for("<done>");
    run.finish();
}

/// Runs the scenario the parent named; does nothing in a normal run.
#[test]
fn pty_child() {
    let Ok(scenario) = std::env::var(SCENARIO) else {
        return;
    };
    match scenario.as_str() {
        "suspend_blocking_read" => {
            use crossterm::event::{Event, KeyCode};
            use spela::tui::events::{LoopEvent, channel};
            use spela::tui::{input, terminal};

            // SAFETY: moves this process into a group of its own, so the
            // stop is real.
            assert_eq!(unsafe { libc::setpgid(0, 0) }, 0);
            let guard = terminal::enter(terminal::TerminalOptions {
                mouse: false,
                ..terminal::TerminalOptions::default()
            })
            .unwrap();
            let (tx, rx) = channel::<()>();
            input::spawn(input::TerminalSource, tx).unwrap();
            say("<entered>");
            let next_key = |within: Duration| -> Option<KeyCode> {
                let deadline = Instant::now() + within;
                while Instant::now() < deadline {
                    for ev in rx.recv_batch_until(deadline) {
                        if let LoopEvent::Input(batch) = ev {
                            for t in batch {
                                if let Event::Key(k) = t.event {
                                    return Some(k.code);
                                }
                            }
                        }
                    }
                }
                None
            };
            assert_eq!(next_key(WAIT), Some(KeyCode::Char('a')));
            terminal::suspend_process().unwrap();
            say("<resumed>");
            assert_eq!(
                next_key(WAIT),
                Some(KeyCode::Char('b')),
                "the next key arrives"
            );
            assert_eq!(
                next_key(Duration::from_millis(200)),
                None,
                "and no ghost key after it"
            );
            drop(guard);
            say("<done>");
        }
        other => panic!("unknown scenario {other:?}"),
    }
}

fn say(text: &str) {
    use std::io::Write;
    let mut out = std::io::stdout();
    out.write_all(text.as_bytes()).unwrap();
    out.flush().unwrap();
}
