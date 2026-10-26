//! A snapshot of the environment variables spela reads.
//!
//! The process environment is read once, in `main`, with
//! [`Env::from_process`]. Everything else takes an [`Env`], so tests can pass
//! any set of variables without touching the process environment.
//! `std::env::set_var` is unsafe in edition 2024 and races with parallel
//! tests. A safety test fails when code outside this file reads `std::env`.

use std::collections::BTreeMap;
use std::ffi::OsString;

/// Variables spela reads, by name.
pub mod var {
    /// Selects the backends. `fake` swaps hardware, privilege, display and
    /// HTTP for the fake and keeps everything else. Hidden: tests set it.
    pub const SPELA_BACKEND: &str = "SPELA_BACKEND";
    /// Log filter, in `tracing_subscriber::EnvFilter` syntax.
    pub const SPELA_LOG: &str = "SPELA_LOG";
    /// Present and non-empty turns colour off (no-color.org).
    pub const NO_COLOR: &str = "NO_COLOR";
    /// Where programs are found, for telling a wrapped game from a typo.
    pub const PATH: &str = "PATH";
    /// The user's home folder, where Steam is looked for.
    pub const HOME: &str = "HOME";
    /// The variable Steam sets to the game's AppID.
    pub const STEAM_APP_ID: &str = "SteamAppId";
    /// The variable Steam sets to the game's ID, which is the AppID for a
    /// Steam game.
    pub const STEAM_GAME_ID: &str = "SteamGameId";

    /// Every name above, for [`super::Env::from_process`].
    pub const ALL: &[&str] = &[
        SPELA_BACKEND,
        SPELA_LOG,
        NO_COLOR,
        PATH,
        HOME,
        STEAM_APP_ID,
        STEAM_GAME_ID,
    ];
}

/// Environment variables captured once at startup.
///
/// Values that are not valid UTF-8 are left out: every variable spela reads
/// is text, and a garbled value would only cause confusing behaviour later.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Env {
    vars: BTreeMap<String, String>,
}

impl Env {
    /// Reads the variables in [`var::ALL`] from the process environment.
    pub fn from_process() -> Self {
        Self::from_os_pairs(
            var::ALL
                .iter()
                .filter_map(|name| std::env::var_os(name).map(|v| ((*name).into(), v))),
        )
    }

    /// Builds a snapshot from name and value pairs, for tests.
    pub fn from_pairs<K: Into<String>, V: Into<String>>(
        pairs: impl IntoIterator<Item = (K, V)>,
    ) -> Self {
        Env {
            vars: pairs
                .into_iter()
                .map(|(k, v)| (k.into(), v.into()))
                .collect(),
        }
    }

    fn from_os_pairs(pairs: impl IntoIterator<Item = (OsString, OsString)>) -> Self {
        Env {
            vars: pairs
                .into_iter()
                .filter_map(|(k, v)| Some((k.into_string().ok()?, v.into_string().ok()?)))
                .collect(),
        }
    }

    /// The value of a variable, if set.
    pub fn get(&self, name: &str) -> Option<&str> {
        self.vars.get(name).map(String::as_str)
    }

    /// The value of a variable if it is set and not empty after trimming.
    pub fn non_empty(&self, name: &str) -> Option<&str> {
        self.get(name).map(str::trim).filter(|v| !v.is_empty())
    }

    /// Whether a variable is set at all, even to an empty string.
    pub fn is_set(&self, name: &str) -> bool {
        self.vars.contains_key(name)
    }

    /// Whether `NO_COLOR` asks for no colour: set and not empty.
    pub fn no_color(&self) -> bool {
        self.get(var::NO_COLOR).is_some_and(|v| !v.is_empty())
    }

    /// Returns a copy with one variable set, for tests.
    pub fn with(mut self, name: &str, value: &str) -> Self {
        self.vars.insert(name.to_owned(), value.to_owned());
        self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn non_empty_ignores_blank_values() {
        let env = Env::from_pairs([("A", "  "), ("B", " x ")]);
        assert_eq!(env.non_empty("A"), None);
        assert_eq!(env.non_empty("B"), Some("x"));
        assert!(env.is_set("A"));
        assert!(!env.is_set("C"));
    }

    #[test]
    fn non_utf8_values_are_dropped() {
        use std::os::unix::ffi::OsStringExt;
        let env = Env::from_os_pairs([
            ("GOOD".into(), "yes".into()),
            ("BAD".into(), OsString::from_vec(vec![0xff, 0xfe])),
        ]);
        assert_eq!(env.get("GOOD"), Some("yes"));
        assert_eq!(env.get("BAD"), None);
    }

    #[test]
    fn no_color_needs_a_non_empty_value() {
        assert!(!Env::default().no_color());
        assert!(!Env::from_pairs([(var::NO_COLOR, "")]).no_color());
        assert!(Env::from_pairs([(var::NO_COLOR, "1")]).no_color());
        assert!(Env::default().with(var::NO_COLOR, "yes").no_color());
    }
}
