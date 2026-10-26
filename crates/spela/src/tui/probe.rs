//! What the terminal can do: one probe with a DA1 sentinel.
//!
//! Adapted from mejl's `terminal/probe.rs`. Two sources:
//!
//! - The environment ([`from_env`]): `TERM`, `TERM_PROGRAM`, the
//!   multiplexer (`TMUX`, `ZELLIJ`, `STY`) and `SSH_TTY`, from the env
//!   snapshot.
//! - One burst of queries written to `/dev/tty` ([`probe_tty`]): the kitty
//!   keyboard flags (`CSI ? u`), synchronized output (DECRQM 2026) and the
//!   terminal's name (XTVERSION), then DA1 last. Terminals answer in order
//!   and almost all answer DA1, so once its reply arrives any reply still
//!   missing means "not supported". A 150 ms timeout bounds a slow
//!   terminal. mejl's theme and OSC 11 queries are dropped: spela has no
//!   automatic theme.
//!
//! The probe reads the replies itself, so it runs before the input thread
//! starts. After a timeout [`drain_late_replies`] keeps reading until the
//! terminal goes quiet, with a hard cap, so late replies never reach the
//! input as keys. [`super::quirks`] can skip the probe or give it more
//! time, and [`super::caps`] fills answers a timed-out probe left unknown.

use std::time::{Duration, Instant};

use spela_core::env::{Env, var};

/// Whether a feature works.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Support {
    /// The terminal said so.
    Yes,
    /// The terminal answered DA1 but not this query.
    No,
    /// No answer, or no probe ran.
    #[default]
    Unknown,
}

/// A terminal multiplexer between spela and the terminal.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Multiplexer {
    /// tmux.
    Tmux,
    /// Zellij.
    Zellij,
    /// GNU screen.
    Screen,
}

/// How the probe went.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub enum ProbeStatus {
    /// No probe ran.
    #[default]
    NotRun,
    /// DA1 arrived, so every answer is final.
    Complete,
    /// The time ran out before DA1.
    TimedOut,
    /// The terminal could not be queried.
    Failed(String),
}

/// Everything spela knows about the terminal.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Capabilities {
    /// The kitty keyboard protocol.
    pub kitty_keyboard: Support,
    /// Synchronized output (mode 2026).
    pub synchronized_output: Support,
    /// The XTVERSION reply, such as `kitty(0.49.2)`.
    pub terminal_version: Option<String>,
    /// `TERM`.
    pub term: Option<String>,
    /// `TERM_PROGRAM`.
    pub term_program: Option<String>,
    /// The multiplexer in between, if any.
    pub multiplexer: Option<Multiplexer>,
    /// Running over SSH.
    pub ssh: bool,
    /// How the probe went.
    pub probe: ProbeStatus,
}

impl Capabilities {
    /// Adds probe answers to environment-based capabilities.
    pub fn apply_probe(&mut self, replies: &ProbeReplies, status: ProbeStatus) {
        let complete = status == ProbeStatus::Complete;
        let answered = |yes: Option<bool>| match (yes, complete) {
            (Some(true), _) => Support::Yes,
            (Some(false), _) => Support::No,
            (None, true) => Support::No,
            (None, false) => Support::Unknown,
        };
        self.kitty_keyboard = answered(replies.kitty_flags.map(|_| true));
        self.synchronized_output = answered(replies.sync_output);
        if replies.terminal_version.is_some() {
            self.terminal_version = replies.terminal_version.clone();
        }
        self.probe = status;
    }
}

/// Capabilities from the environment alone.
pub fn from_env(env: &Env) -> Capabilities {
    let multiplexer = if env.non_empty(var::TMUX).is_some() {
        Some(Multiplexer::Tmux)
    } else if env.non_empty(var::ZELLIJ).is_some() {
        Some(Multiplexer::Zellij)
    } else if env.non_empty(var::STY).is_some() {
        Some(Multiplexer::Screen)
    } else {
        None
    };
    Capabilities {
        term: env.non_empty(var::TERM).map(str::to_owned),
        term_program: env.non_empty(var::TERM_PROGRAM).map(str::to_owned),
        multiplexer,
        ssh: env.non_empty(var::SSH_TTY).is_some(),
        ..Capabilities::default()
    }
}

/// The query burst, DA1 last.
pub const QUERIES: &str = concat!(
    "\x1b[?u",      // kitty keyboard flags
    "\x1b[?2026$p", // DECRQM synchronized output
    "\x1b[>q",      // XTVERSION
    "\x1b[c",       // DA1, the sentinel
);

/// The answers found in the terminal's reply bytes.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct ProbeReplies {
    /// The kitty flags reply.
    pub kitty_flags: Option<u8>,
    /// DECRPM for 2026: `Some(true)` for set or reset, `Some(false)` for
    /// not recognised or permanently reset.
    pub sync_output: Option<bool>,
    /// XTVERSION text, control characters removed.
    pub terminal_version: Option<String>,
    /// The DA1 reply arrived.
    pub da1: bool,
}

/// Parses everything recognisable in `bytes`. Unknown sequences are
/// skipped; an unfinished one at the end is ignored.
pub fn parse_replies(bytes: &[u8]) -> ProbeReplies {
    let mut out = ProbeReplies::default();
    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] != 0x1b || i + 1 >= bytes.len() {
            i += 1;
            continue;
        }
        match bytes[i + 1] {
            b'[' => match parse_csi(&bytes[i + 2..]) {
                Some((params, inter, fin, used)) => {
                    apply_csi(params, inter, fin, &mut out);
                    i += 2 + used;
                }
                None => break,
            },
            b']' | b'P' => match string_end(&bytes[i + 2..]) {
                Some((body, used)) => {
                    if bytes[i + 1] == b'P'
                        && let Some(name) = body.strip_prefix(b">|")
                    {
                        let text: String = String::from_utf8_lossy(name)
                            .chars()
                            .filter(|c| !c.is_control())
                            .take(128)
                            .collect();
                        if !text.trim().is_empty() {
                            out.terminal_version = Some(text.trim().to_owned());
                        }
                    }
                    i += 2 + used;
                }
                None => break,
            },
            _ => i += 1,
        }
    }
    out
}

/// Splits a CSI body (after `ESC [`): parameters, intermediates, the
/// final byte (0 when it is not a valid CSI) and the bytes used.
fn parse_csi(rest: &[u8]) -> Option<(&[u8], &[u8], u8, usize)> {
    let params_end = rest
        .iter()
        .position(|b| !(0x30..=0x3f).contains(b))
        .unwrap_or(rest.len());
    let inter_end = params_end
        + rest[params_end..]
            .iter()
            .position(|b| !(0x20..=0x2f).contains(b))
            .unwrap_or(rest.len() - params_end);
    let final_byte = *rest.get(inter_end)?;
    if !(0x40..=0x7e).contains(&final_byte) {
        return Some((&[], &[], 0, 0));
    }
    Some((
        &rest[..params_end],
        &rest[params_end..inter_end],
        final_byte,
        inter_end + 1,
    ))
}

fn apply_csi(params: &[u8], inter: &[u8], fin: u8, out: &mut ProbeReplies) {
    let Some(params) = params.strip_prefix(b"?") else {
        return;
    };
    let text = std::str::from_utf8(params).unwrap_or("");
    let nums: Vec<Option<u32>> = text.split(';').map(|p| p.parse().ok()).collect();
    match (fin, inter) {
        (b'u', b"") => {
            if let Some(Some(flags)) = nums.first() {
                out.kitty_flags = u8::try_from(*flags).ok();
            }
        }
        (b'y', b"$") => {
            if let [Some(2026), Some(ps)] = nums.as_slice() {
                out.sync_output = Some(matches!(ps, 1 | 2));
            }
        }
        (b'c', b"") => out.da1 = true,
        _ => {}
    }
}

/// Finds the end of an OSC or DCS string: BEL or ST (`ESC \`).
fn string_end(rest: &[u8]) -> Option<(&[u8], usize)> {
    for (i, &b) in rest.iter().enumerate() {
        if b == 0x07 {
            return Some((&rest[..i], i + 1));
        }
        if b == 0x1b {
            return match rest.get(i + 1) {
                Some(b'\\') => Some((&rest[..i], i + 2)),
                Some(_) => Some((&rest[..i], i)),
                None => None,
            };
        }
    }
    None
}

/// The default probe timeout. DA1 ends the wait early.
pub const DEFAULT_TIMEOUT: Duration = Duration::from_millis(150);

/// The capabilities for this terminal: the environment, then the probe
/// unless a quirk skips it, then the cache at `cache` (none under
/// `--demo`).
pub fn detect(env: &Env, timeout: Duration, cache: Option<&std::path::Path>) -> Capabilities {
    let mut caps = from_env(env);
    let quirk = super::quirks::lookup(caps.term.as_deref(), caps.term_program.as_deref());
    if let Some(q) = quirk {
        tracing::debug!(quirk = q.name, "terminal quirk applies");
    }
    let timeout = match quirk {
        Some(q) if q.skip_probe => None,
        Some(q) => Some(q.probe_timeout.map_or(timeout, |qt| qt.max(timeout))),
        None => Some(timeout),
    };
    if let Some(timeout) = timeout {
        match probe_tty(timeout) {
            Ok((replies, status)) => caps.apply_probe(&replies, status),
            Err(e) => caps.probe = ProbeStatus::Failed(e.to_string()),
        }
    }
    if let Some(path) = cache {
        super::caps::apply(&mut caps, env, path);
    }
    caps
}

/// Where the probe reads reply bytes. The real one polls `/dev/tty`;
/// tests use a script with a clock of its own.
pub trait ReplySource {
    /// The current time on this source's clock.
    fn now(&self) -> Instant;

    /// Waits up to `timeout` for bytes. `Ok(None)` when the time ran out,
    /// `Ok(Some(0))` at the end of input.
    fn read_timeout(&mut self, buf: &mut [u8], timeout: Duration)
    -> std::io::Result<Option<usize>>;
}

/// The most reply bytes kept.
pub const MAX_REPLY_BYTES: usize = 8192;

/// How long [`drain_late_replies`] reads after a timed-out probe.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DrainLimits {
    /// Stop after this long with no bytes, unless a reply is half read.
    pub quiet: Duration,
    /// Never read longer than this.
    pub cap: Duration,
}

/// The drain after a timed-out probe.
pub const DRAIN: DrainLimits = DrainLimits {
    quiet: Duration::from_millis(50),
    cap: Duration::from_millis(500),
};

/// Reads replies until DA1 arrives or `timeout` passes, then drains late
/// replies within [`DRAIN`]. A DA1 found while draining still completes
/// the probe.
pub fn run_probe<S: ReplySource>(
    src: &mut S,
    timeout: Duration,
) -> std::io::Result<(ProbeReplies, ProbeStatus)> {
    let deadline = src.now() + timeout;
    let mut buf = Vec::with_capacity(512);
    let mut chunk = [0u8; 512];
    loop {
        let left = deadline.saturating_duration_since(src.now());
        if left.is_zero() {
            break;
        }
        match src.read_timeout(&mut chunk, left)? {
            None => continue,
            Some(0) => return Ok((parse_replies(&buf), ProbeStatus::TimedOut)),
            Some(n) => buf.extend_from_slice(&chunk[..n]),
        }
        let replies = parse_replies(&buf);
        if replies.da1 {
            return Ok((replies, ProbeStatus::Complete));
        }
        if buf.len() > MAX_REPLY_BYTES {
            return Ok((replies, ProbeStatus::TimedOut));
        }
    }
    let da1 = drain_late_replies(src, &mut buf, DRAIN)?;
    let status = if da1 {
        ProbeStatus::Complete
    } else {
        ProbeStatus::TimedOut
    };
    Ok((parse_replies(&buf), status))
}

/// Reads what the terminal still sends after a timed-out probe. Stops at
/// DA1 (and returns `true`), after `limits.quiet` with no bytes while no
/// reply is half read, at `limits.cap`, or at the end of input.
pub fn drain_late_replies<S: ReplySource>(
    src: &mut S,
    buf: &mut Vec<u8>,
    limits: DrainLimits,
) -> std::io::Result<bool> {
    let start = src.now();
    let hard_stop = start + limits.cap;
    let mut scanner = ReplyScanner::default();
    scanner.feed(buf);
    let mut quiet_until = start + limits.quiet;
    let mut drained = 0usize;
    let mut chunk = [0u8; 512];
    loop {
        let stop = if scanner.in_sequence() {
            hard_stop
        } else {
            quiet_until.min(hard_stop)
        };
        let left = stop.saturating_duration_since(src.now());
        if left.is_zero() {
            return Ok(false);
        }
        let n = match src.read_timeout(&mut chunk, left)? {
            None => continue,
            Some(0) => return Ok(false),
            Some(n) => n,
        };
        let from = buf.len();
        buf.extend_from_slice(&chunk[..n]);
        scanner.feed(&chunk[..n]);
        drained += n;
        quiet_until = src.now() + limits.quiet;
        if parse_replies(&buf[from.saturating_sub(64)..]).da1 {
            return Ok(true);
        }
        if drained > MAX_REPLY_BYTES {
            return Ok(false);
        }
    }
}

/// Tracks whether a byte stream is inside an escape sequence, so the drain
/// never stops inside a reply.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct ReplyScanner {
    state: ScanState,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
enum ScanState {
    #[default]
    Ground,
    Escape,
    Csi,
    Text,
    TextEscape,
}

impl ReplyScanner {
    /// Advances over `bytes`.
    pub fn feed(&mut self, bytes: &[u8]) {
        for &b in bytes {
            self.state = match (self.state, b) {
                (ScanState::Ground, 0x1b) => ScanState::Escape,
                (ScanState::Ground, _) => ScanState::Ground,
                (ScanState::Escape, b'[') => ScanState::Csi,
                (ScanState::Escape, b']' | b'P' | b'_' | b'^' | b'X') => ScanState::Text,
                (ScanState::Escape, 0x1b) => ScanState::Escape,
                (ScanState::Escape, _) => ScanState::Ground,
                (ScanState::Csi, 0x40..=0x7e) => ScanState::Ground,
                (ScanState::Csi, 0x1b) => ScanState::Escape,
                (ScanState::Csi, 0x20..=0x3f) => ScanState::Csi,
                (ScanState::Csi, _) => ScanState::Ground,
                (ScanState::Text, 0x07) => ScanState::Ground,
                (ScanState::Text, 0x1b) => ScanState::TextEscape,
                (ScanState::Text, _) => ScanState::Text,
                (ScanState::TextEscape, b'\\') => ScanState::Ground,
                (ScanState::TextEscape, b'[') => ScanState::Csi,
                (ScanState::TextEscape, b']' | b'P' | b'_' | b'^' | b'X') => ScanState::Text,
                (ScanState::TextEscape, 0x1b) => ScanState::Escape,
                (ScanState::TextEscape, _) => ScanState::Ground,
            };
        }
    }

    /// Whether the bytes so far end inside a sequence.
    pub fn in_sequence(&self) -> bool {
        self.state != ScanState::Ground
    }
}

/// Sends [`QUERIES`] to `/dev/tty` and reads the replies with
/// [`run_probe`]. Raw mode is on for the probe.
pub fn probe_tty(timeout: Duration) -> std::io::Result<(ProbeReplies, ProbeStatus)> {
    use std::io::Write;

    let was_raw = crossterm::terminal::is_raw_mode_enabled()?;
    if !was_raw {
        crossterm::terminal::enable_raw_mode()?;
    }
    let result = (|| {
        let mut tty = std::fs::OpenOptions::new()
            .read(true)
            .write(true)
            .open("/dev/tty")?;
        tty.write_all(QUERIES.as_bytes())?;
        tty.flush()?;
        run_probe(&mut TtySource { tty }, timeout)
    })();
    if !was_raw {
        let _ = crossterm::terminal::disable_raw_mode();
    }
    result
}

struct TtySource {
    tty: std::fs::File,
}

impl ReplySource for TtySource {
    fn now(&self) -> Instant {
        Instant::now()
    }

    fn read_timeout(
        &mut self,
        buf: &mut [u8],
        timeout: Duration,
    ) -> std::io::Result<Option<usize>> {
        use std::io::Read;
        use std::os::fd::AsRawFd;

        let mut pfd = libc::pollfd {
            fd: self.tty.as_raw_fd(),
            events: libc::POLLIN,
            revents: 0,
        };
        let ms = libc::c_int::try_from(timeout.as_millis().max(1)).unwrap_or(libc::c_int::MAX);
        // SAFETY: one valid pollfd for a descriptor `self.tty` keeps open.
        let rc = unsafe { libc::poll(&mut pfd, 1, ms) };
        if rc < 0 {
            let err = std::io::Error::last_os_error();
            if err.kind() == std::io::ErrorKind::Interrupted {
                return Ok(None);
            }
            return Err(err);
        }
        if rc == 0 {
            return Ok(None);
        }
        self.tty.read(buf).map(Some)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::VecDeque;

    #[test]
    fn replies_parse() {
        let r = parse_replies(b"\x1b[?1u\x1b[?2026;2$y\x1bP>|kitty(0.49.2)\x1b\\\x1b[?62;22c");
        assert_eq!(r.kitty_flags, Some(1));
        assert_eq!(r.sync_output, Some(true));
        assert_eq!(r.terminal_version.as_deref(), Some("kitty(0.49.2)"));
        assert!(r.da1);
        let none = parse_replies(b"\x1b[?2026;0$y\x1b[?6c");
        assert_eq!(none.sync_output, Some(false));
        let mut caps = Capabilities::default();
        caps.apply_probe(&none, ProbeStatus::Complete);
        assert_eq!(
            caps.kitty_keyboard,
            Support::No,
            "DA1 makes a missing reply final"
        );
        assert_eq!(caps.synchronized_output, Support::No);
    }

    #[test]
    fn the_environment_names_the_terminal() {
        let env = Env::from_pairs([
            ("TERM", "xterm-kitty"),
            ("TMUX", "x"),
            ("SSH_TTY", "/dev/pts/3"),
        ]);
        let caps = from_env(&env);
        assert_eq!(caps.term.as_deref(), Some("xterm-kitty"));
        assert_eq!(caps.multiplexer, Some(Multiplexer::Tmux));
        assert!(caps.ssh);
    }

    /// Bytes that arrive at given times on a clock the test moves.
    struct Script {
        now: Instant,
        chunks: VecDeque<(Duration, &'static [u8])>,
    }

    impl ReplySource for Script {
        fn now(&self) -> Instant {
            self.now
        }

        fn read_timeout(
            &mut self,
            buf: &mut [u8],
            timeout: Duration,
        ) -> std::io::Result<Option<usize>> {
            let start = self.now;
            match self.chunks.front() {
                Some((at, bytes)) if start + timeout >= start + *at => {
                    let (at, bytes) = (*at, *bytes);
                    self.chunks.pop_front();
                    self.now = start + at;
                    for c in &mut self.chunks {
                        c.0 = c.0.saturating_sub(at);
                    }
                    buf[..bytes.len()].copy_from_slice(bytes);
                    Ok(Some(bytes.len()))
                }
                _ => {
                    self.now = start + timeout;
                    for c in &mut self.chunks {
                        c.0 = c.0.saturating_sub(timeout);
                    }
                    Ok(None)
                }
            }
        }
    }

    #[test]
    fn da1_ends_the_wait_and_late_replies_are_drained() {
        let mut fast = Script {
            now: Instant::now(),
            chunks: VecDeque::from([(Duration::from_millis(2), &b"\x1b[?0u\x1b[?62c"[..])]),
        };
        let (r, status) = run_probe(&mut fast, DEFAULT_TIMEOUT).unwrap();
        assert_eq!(status, ProbeStatus::Complete);
        assert_eq!(r.kitty_flags, Some(0));

        let mut slow = Script {
            now: Instant::now(),
            chunks: VecDeque::from([
                (Duration::from_millis(40), &b"\x1b[?2026;"[..]),
                (Duration::from_millis(200), &b"2$y\x1b[?62c"[..]),
            ]),
        };
        let (r, status) = run_probe(&mut slow, DEFAULT_TIMEOUT).unwrap();
        assert_eq!(status, ProbeStatus::Complete, "DA1 found while draining");
        assert_eq!(r.sync_output, Some(true));
    }
}
