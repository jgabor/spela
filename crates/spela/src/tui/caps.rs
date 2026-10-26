//! The terminal capability cache, `<cache>/caps.toml`.
//!
//! Adapted from mejl's `terminal/cache.rs`. A slow terminal, such as one
//! over SSH, may not answer the probe in time. When an earlier start on
//! the same terminal got full answers, they fill those gaps.
//!
//! - Kept: kitty keyboard, synchronized output and the XTVERSION name.
//!   mejl's theme answer is gone with its query.
//! - Keyed by `TERM`, `TERM_PROGRAM`, `TERM_PROGRAM_VERSION`, the
//!   multiplexer and SSH. A differing XTVERSION drops an entry.
//! - Only a complete probe with definite kitty and sync answers writes,
//!   and only on a change, atomically and without fsync. Newest first, at
//!   most [`MAX_ENTRIES`].
//! - A file that does not load counts as empty. `--demo` uses none.

use std::path::Path;

use serde::{Deserialize, Serialize};
use spela_core::env::{Env, var};
use spela_core::fs::{Durability, write_atomic};

use super::probe::{Capabilities, Multiplexer, ProbeStatus, Support};

/// The file format version.
pub const VERSION: u32 = 1;

/// The most terminals remembered.
pub const MAX_ENTRIES: usize = 16;

/// The whole file.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
pub struct CapsCache {
    /// [`VERSION`].
    pub version: u32,
    /// Newest first.
    #[serde(default, rename = "terminal")]
    pub entries: Vec<Entry>,
}

/// What one terminal answered.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Entry {
    /// See [`key`].
    pub key: String,
    /// The XTVERSION reply.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub terminal_version: Option<String>,
    /// The kitty keyboard protocol.
    pub kitty_keyboard: Answer,
    /// Synchronized output.
    pub synchronized_output: Answer,
}

/// A definite answer.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Answer {
    /// Supported.
    Yes,
    /// Not supported.
    No,
}

impl Answer {
    fn from_support(s: Support) -> Option<Answer> {
        match s {
            Support::Yes => Some(Answer::Yes),
            Support::No => Some(Answer::No),
            Support::Unknown => None,
        }
    }

    fn support(self) -> Support {
        match self {
            Answer::Yes => Support::Yes,
            Answer::No => Support::No,
        }
    }
}

/// The cache key, from environment values only.
pub fn key(caps: &Capabilities, env: &Env) -> String {
    let mux = match caps.multiplexer {
        Some(Multiplexer::Tmux) => "tmux",
        Some(Multiplexer::Zellij) => "zellij",
        Some(Multiplexer::Screen) => "screen",
        None => "none",
    };
    format!(
        "term={};program={};version={};mux={mux};ssh={}",
        caps.term.as_deref().unwrap_or(""),
        caps.term_program.as_deref().unwrap_or(""),
        env.non_empty(var::TERM_PROGRAM_VERSION).unwrap_or(""),
        if caps.ssh { "yes" } else { "no" },
    )
}

impl CapsCache {
    /// Reads the file. Anything wrong with it gives an empty cache.
    pub fn load(path: &Path) -> CapsCache {
        let empty = CapsCache {
            version: VERSION,
            entries: Vec::new(),
        };
        let Ok(text) = std::fs::read_to_string(path) else {
            return empty;
        };
        match toml::from_str::<CapsCache>(&text) {
            Ok(cache) if cache.version == VERSION => cache,
            _ => empty,
        }
    }

    /// The entry for `key`.
    pub fn get(&self, key: &str) -> Option<&Entry> {
        self.entries.iter().find(|e| e.key == key)
    }

    /// Records a complete probe's answers, newest first. Returns whether
    /// anything changed.
    pub fn record(&mut self, key: &str, caps: &Capabilities) -> bool {
        if caps.probe != ProbeStatus::Complete {
            return false;
        }
        let (Some(kitty_keyboard), Some(synchronized_output)) = (
            Answer::from_support(caps.kitty_keyboard),
            Answer::from_support(caps.synchronized_output),
        ) else {
            return false;
        };
        let entry = Entry {
            key: key.to_owned(),
            terminal_version: caps.terminal_version.clone(),
            kitty_keyboard,
            synchronized_output,
        };
        if self.entries.first() == Some(&entry) {
            return false;
        }
        self.entries.retain(|e| e.key != key);
        self.entries.insert(0, entry);
        self.entries.truncate(MAX_ENTRIES);
        true
    }

    /// Writes the file atomically, without fsync: it is a cache.
    pub fn save(&self, path: &Path) -> std::io::Result<()> {
        let text = toml::to_string(self).map_err(std::io::Error::other)?;
        if let Some(dir) = path.parent() {
            std::fs::create_dir_all(dir)?;
        }
        write_atomic(path, text.as_bytes(), Durability::Cache)
    }
}

/// Fills answers the probe left unknown from `entry`, unless this run's
/// XTVERSION shows another terminal. Returns whether any answer came from
/// the entry.
pub fn fill_unknown(caps: &mut Capabilities, entry: &Entry) -> bool {
    if caps.terminal_version.is_some() && caps.terminal_version != entry.terminal_version {
        return false;
    }
    let mut used = false;
    for (field, cached) in [
        (&mut caps.kitty_keyboard, entry.kitty_keyboard),
        (&mut caps.synchronized_output, entry.synchronized_output),
    ] {
        if *field == Support::Unknown {
            *field = cached.support();
            used = true;
        }
    }
    if caps.terminal_version.is_none() {
        caps.terminal_version = entry.terminal_version.clone();
    }
    used
}

/// After a complete probe the file is updated; otherwise its entry fills
/// what is missing. A failed write is logged: the cache is a shortcut.
pub fn apply(caps: &mut Capabilities, env: &Env, path: &Path) {
    let key = key(caps, env);
    let mut cache = CapsCache::load(path);
    let changed = if caps.probe == ProbeStatus::Complete {
        cache.record(&key, caps)
    } else {
        match cache.get(&key).cloned() {
            Some(entry) if fill_unknown(caps, &entry) => false,
            Some(entry)
                if caps.terminal_version.is_some()
                    && caps.terminal_version != entry.terminal_version =>
            {
                cache.entries.retain(|e| e.key != key);
                true
            }
            _ => false,
        }
    };
    if changed && let Err(e) = cache.save(path) {
        tracing::warn!(path = %path.display(), "cannot write caps.toml: {e}");
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tui::probe::from_env;

    fn kitty() -> Env {
        Env::from_pairs([
            ("TERM", "xterm-kitty"),
            ("TERM_PROGRAM", "kitty"),
            ("TERM_PROGRAM_VERSION", "0.49.2"),
        ])
    }

    fn complete(env: &Env) -> Capabilities {
        Capabilities {
            kitty_keyboard: Support::Yes,
            synchronized_output: Support::Yes,
            terminal_version: Some("kitty(0.49.2)".into()),
            probe: ProbeStatus::Complete,
            ..from_env(env)
        }
    }

    #[test]
    fn a_complete_probe_fills_a_later_timeout() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("cache/caps.toml");
        let env = kitty();
        apply(&mut complete(&env), &env, &path);
        let text = std::fs::read_to_string(&path).unwrap();
        assert!(text.contains("kitty_keyboard = \"yes\""), "{text}");
        let mut slow = Capabilities {
            probe: ProbeStatus::TimedOut,
            ..from_env(&env)
        };
        apply(&mut slow, &env, &path);
        assert_eq!(slow.synchronized_output, Support::Yes);
        assert_eq!(slow.probe, ProbeStatus::TimedOut);
        let other = kitty().with("TERM_PROGRAM_VERSION", "0.50.0");
        let mut fresh = Capabilities {
            probe: ProbeStatus::TimedOut,
            ..from_env(&other)
        };
        apply(&mut fresh, &other, &path);
        assert_eq!(fresh.kitty_keyboard, Support::Unknown, "another terminal");
    }

    #[test]
    fn bad_files_count_as_empty_and_entries_are_capped() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("caps.toml");
        std::fs::write(&path, "not [[[ toml").unwrap();
        assert!(CapsCache::load(&path).entries.is_empty());
        let mut cache = CapsCache::default();
        for i in 0..MAX_ENTRIES + 3 {
            let env = kitty().with("TERM_PROGRAM_VERSION", &i.to_string());
            let caps = complete(&env);
            assert!(cache.record(&key(&caps, &env), &caps));
        }
        assert_eq!(cache.entries.len(), MAX_ENTRIES);
    }
}
