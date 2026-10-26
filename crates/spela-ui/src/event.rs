//! What reaches [`crate::update`] and what it asks the shell to do.
//!
//! [`Event`] is everything the UI reacts to: keys, the mouse, pastes,
//! resizes, the metrics tick, a due wake-up and worker results ([`Msg`]).
//! [`Cmd`] is everything the UI asks of the shell: polls, scans, saves and
//! jobs. update does no I/O itself (architecture.md, channels and message
//! types).
//!
//! The shell-level variants are the ones architecture.md sketches. A
//! screen's own results and requests travel in its own variant, such as
//! [`Msg::Profile`] and [`Cmd::Profile`], whose types live in that
//! screen's file, so the screens can grow without touching this one.
//! Results carry the request id or token they answer, and a screen drops a
//! result whose id is stale.

use std::collections::BTreeMap;

use crossterm::event::{Event as TermEvent, MouseEvent};
use spela_core::alerts::Alert;
use spela_core::backend::{ApplyReport, GpuMetrics, PrivilegeError};
use spela_core::games::GameCache;
use spela_core::metrics::CpuMetrics;
use spela_core::profile::{FieldKey, Scope, Value};

use crate::keymap::Key;
use crate::screens;

/// One thing the UI reacts to.
#[derive(Clone, Debug)]
pub enum Event {
    /// A key, in canonical form.
    Key(Key),
    /// A mouse event.
    Mouse(MouseEvent),
    /// Pasted text, from bracketed paste. It never fires keys.
    Paste(String),
    /// The terminal is now this many columns and rows.
    Resize(u16, u16),
    /// The metrics cadence, every 2 s while [`crate::AppState::wants_metrics`].
    Tick,
    /// A wake-up asked for with [`crate::AppState::next_wakeup`] is due.
    Wake,
    /// A worker or the write queue finished.
    Msg(Msg),
}

impl Event {
    /// The UI event for a terminal event, if the UI cares about it. Focus
    /// changes are dropped: the metrics tick does not pause on focus lost
    /// (decision 42).
    pub fn from_terminal(event: TermEvent) -> Option<Event> {
        match event {
            TermEvent::Key(k) => Key::from_event(&k).map(Event::Key),
            TermEvent::Mouse(m) => Some(Event::Mouse(m)),
            TermEvent::Paste(s) => Some(Event::Paste(s)),
            TermEvent::Resize(w, h) => Some(Event::Resize(w, h)),
            TermEvent::FocusGained | TermEvent::FocusLost => None,
        }
    }
}

/// One metrics sweep: GPU 0, the CPU, and the alerts the evaluator found.
#[derive(Clone, Debug, PartialEq)]
pub struct MetricsSample {
    /// GPU 0, or why it could not be read.
    pub gpu: Result<GpuMetrics, String>,
    /// The CPU, or why it could not be read.
    pub cpu: Result<CpuMetrics, String>,
    /// The active alerts, in the evaluator's order.
    pub alerts: Vec<Alert>,
}

/// A live hardware command (decision 44). Phase 6 runs it.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum HardwareOp {
    /// `spela gpu reset`.
    GpuReset,
    /// `spela cpu governor <g>`.
    Governor(String),
    /// `spela cpu smt on` or `off`.
    Smt(bool),
}

/// What to do with a deny list entry.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum DenyChange {
    /// `denylist allow`.
    Allow,
    /// `denylist deny --reason <reason>`.
    Deny {
        /// The reason.
        reason: String,
    },
    /// `denylist clear`.
    Clear,
}

/// One write for the write queue: one file and the keys it changes.
/// Queued writes for the same file merge by key, newest value per key
/// (architecture.md, TUI process).
#[derive(Clone, Debug, PartialEq)]
pub enum Write {
    /// Fields of one profile: `Some` sets an own value, `None` makes the
    /// field follow.
    Profile {
        /// The profile.
        scope: Scope,
        /// The game's name, filled into a new game file.
        name: String,
        /// The changes, by field.
        changes: BTreeMap<FieldKey, Option<Value>>,
    },
    /// Settings, by `config set` key, with the raw value text.
    Config {
        /// The changes, by key.
        changes: BTreeMap<String, String>,
    },
    /// One game's deny list override.
    Deny {
        /// The game.
        app_id: u64,
        /// Its name, for a new deny entry.
        name: String,
        /// The change.
        change: DenyChange,
    },
}

impl Write {
    /// Which file the write goes to. Two writes with the same file merge.
    pub fn file(&self) -> WriteFile {
        match self {
            Write::Profile { scope, .. } => WriteFile::Profile(*scope),
            Write::Config { .. } => WriteFile::Config,
            Write::Deny { app_id, .. } => WriteFile::Deny(*app_id),
        }
    }

    /// Merges a newer write for the same file into this one: the newer
    /// value wins per key. Returns the newer write unchanged when it is for
    /// another file.
    pub fn merge(&mut self, newer: Write) -> Result<(), Write> {
        match (self, newer) {
            (
                Write::Profile { scope, changes, .. },
                Write::Profile {
                    scope: s,
                    changes: c,
                    name,
                },
            ) if *scope == s => {
                changes.extend(c);
                let _ = name;
                Ok(())
            }
            (Write::Config { changes }, Write::Config { changes: c }) => {
                changes.extend(c);
                Ok(())
            }
            (
                Write::Deny { app_id, change, .. },
                Write::Deny {
                    app_id: a,
                    change: c,
                    ..
                },
            ) if *app_id == a => {
                *change = c;
                Ok(())
            }
            (_, newer) => Err(newer),
        }
    }
}

/// The file a [`Write`] changes.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum WriteFile {
    /// A profile.
    Profile(Scope),
    /// `config.toml`.
    Config,
    /// `lists/overrides.toml`, for one game's entry.
    Deny(u64),
}

/// A result the UI understands.
#[derive(Clone, Debug)]
pub enum Msg {
    /// A metrics sweep.
    Metrics(Box<MetricsSample>),
    /// A background scan finished. `profiles` lists the games with their
    /// own profile file after it.
    ScanDone {
        /// The request this answers.
        request: u64,
        /// The new game cache, or why the scan failed.
        result: Result<GameCache, String>,
        /// Games with their own profile.
        profiles: Vec<u64>,
    },
    /// A queued write finished. Every token merged into one write gets the
    /// same result.
    Saved {
        /// The token of the [`Cmd::Save`].
        token: u64,
        /// What happened.
        result: Result<(), String>,
    },
    /// The release check found a newer spela (`spela vX is available`).
    /// Phase 5.
    ReleaseNotice(Option<String>),
    /// The startup DLL auto-update changed something. Phase 5.
    AutoUpdateNotice(String),
    /// A hardware job finished. Phase 6.
    HardwareDone {
        /// The token of the [`Cmd::Hardware`].
        token: u64,
        /// The helper's report, or why it failed.
        result: Result<ApplyReport, PrivilegeError>,
    },
    /// A Preview launch summary is built. Phase 6.
    PreviewReady {
        /// The request this answers.
        request: u64,
        /// The summary text `launch --dry-run` prints, or why it failed.
        result: Result<String, String>,
    },
    /// A result for the Library list.
    Library(screens::library::Msg),
    /// A result for the profile page.
    Profile(screens::profile::Msg),
    /// A result for Settings.
    Settings(screens::settings::Msg),
    /// A result for Monitor.
    Monitor(screens::monitor::Msg),
    /// A result for the DLL Catalog and the game's DLLs.
    Dlls(screens::dlls::Msg),
    /// A result for the deny list surface.
    Denylist(screens::denylist::Msg),
}

/// A request to the shell.
#[derive(Clone, Debug, PartialEq)]
pub enum Cmd {
    /// Poll GPU 0 and the CPU once. The UI never asks while a poll is in
    /// flight.
    PollMetrics,
    /// Scan Steam in the background.
    Scan {
        /// The request id the result carries.
        request: u64,
    },
    /// Queue a write.
    Save {
        /// The token the result carries.
        token: u64,
        /// The write.
        write: Write,
    },
    /// Check for a newer release. Phase 5.
    CheckRelease,
    /// Update stale DLLs at startup. Phase 5.
    AutoUpdate,
    /// Run a live hardware command on the job pool (decision 44). Phase 6.
    Hardware {
        /// The token the result carries.
        token: u64,
        /// The command.
        op: HardwareOp,
    },
    /// Build the read-only Preview launch summary (decision 44). Phase 6.
    Preview {
        /// The request id the result carries.
        request: u64,
        /// The game.
        app_id: u64,
    },
    /// Stop to the shell, as Ctrl-Z does, and redraw on resume.
    Suspend,
    /// Leave the TUI.
    Quit,
    /// Work for the Library list.
    Library(screens::library::Cmd),
    /// Work for the profile page.
    Profile(screens::profile::Cmd),
    /// Work for Settings.
    Settings(screens::settings::Cmd),
    /// Work for Monitor.
    Monitor(screens::monitor::Cmd),
    /// Work for the DLL Catalog and the game's DLLs.
    Dlls(screens::dlls::Cmd),
    /// Work for the deny list surface.
    Denylist(screens::denylist::Cmd),
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn writes_merge_per_file_and_key() {
        let mut a = Write::Config {
            changes: BTreeMap::from([("ui.mouse".to_owned(), "false".to_owned())]),
        };
        let b = Write::Config {
            changes: BTreeMap::from([
                ("ui.mouse".to_owned(), "true".to_owned()),
                ("check_updates".to_owned(), "false".to_owned()),
            ]),
        };
        assert!(a.merge(b).is_ok());
        let Write::Config { changes } = &a else {
            unreachable!()
        };
        assert_eq!(changes["ui.mouse"], "true");
        assert_eq!(changes.len(), 2);

        let deny = Write::Deny {
            app_id: 1,
            name: "x".into(),
            change: DenyChange::Allow,
        };
        assert_eq!(a.merge(deny.clone()), Err(deny));
        assert_eq!(a.file(), WriteFile::Config);
    }
}
