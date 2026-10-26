//! Profiles: what a game sets itself, what All games sets, and what a
//! field resolves to.
//!
//! A profile stores only the values its scope sets itself. A key that is
//! present is an own value; a key that is absent follows. A game follows
//! All games, and All games follows the built-in default, the zero of each
//! field's kind. This replaces Go's `overrides` map: an explicit zero is
//! just a stored value.
//!
//! | Module | Role |
//! |---|---|
//! | [`field`] | The field table of 25 fields, with keys, flags and help |
//! | [`value`] | [`Value`], [`Kind`] and the one bool parser |
//! | [`resolve`] | Effective values with their [`Origin`] |
//! | [`words`] | The words every surface prints for values and origins |
//! | [`store`] | The TOML profile files |

pub mod field;
pub mod resolve;
pub mod store;
pub mod value;
pub mod words;

use std::collections::BTreeMap;

pub use field::{FIELDS, Field, FieldError, FieldKey, Section};
pub use resolve::{Effective, Origin, Resolved, Source};
pub use value::{Kind, SMT_UNCHANGED, Value, parse_bool};

/// Which profile a value belongs to.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Scope {
    /// The All games profile, which every game follows.
    AllGames,
    /// One game's profile, by Steam AppID.
    Game(u64),
}

/// The values one profile sets itself. Every value has passed its field's
/// check. Iteration is in table order.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct OwnValues {
    values: BTreeMap<FieldKey, Value>,
}

impl OwnValues {
    /// No own values: everything follows.
    pub fn new() -> OwnValues {
        OwnValues::default()
    }

    /// The own value of `key`, or `None` when it follows.
    pub fn get(&self, key: FieldKey) -> Option<&Value> {
        self.values.get(&key)
    }

    /// Whether the scope sets `key` itself.
    pub fn contains(&self, key: FieldKey) -> bool {
        self.values.contains_key(&key)
    }

    /// Sets an own value after checking it, and returns the one it
    /// replaced. SMT's none is a value too: it leaves SMT unchanged even
    /// when All games sets it (decision 65). [`OwnValues::follow`] removes
    /// the key instead.
    pub fn set(&mut self, key: FieldKey, value: Value) -> Result<Option<Value>, FieldError> {
        key.field().check(&value)?;
        Ok(self.values.insert(key, value))
    }

    /// Makes `key` follow again: on a game it follows All games, on All
    /// games it follows the system default. Returns the value it held, so
    /// a surface can offer it back.
    pub fn follow(&mut self, key: FieldKey) -> Option<Value> {
        self.values.remove(&key)
    }

    /// Makes every field follow. Returns how many own values there were.
    pub fn follow_all(&mut self) -> usize {
        let count = self.values.len();
        self.values.clear();
        count
    }

    /// The number of own values.
    pub fn len(&self) -> usize {
        self.values.len()
    }

    /// Whether every field follows.
    pub fn is_empty(&self) -> bool {
        self.values.is_empty()
    }

    /// The own values in table order.
    pub fn iter(&self) -> impl Iterator<Item = (FieldKey, &Value)> {
        self.values.iter().map(|(k, v)| (*k, v))
    }

    /// The own values of one section, in table order.
    pub fn in_section(&self, section: Section) -> impl Iterator<Item = (FieldKey, &Value)> {
        self.iter()
            .filter(move |(k, _)| k.field().section == section)
    }
}

/// The All games profile, `profiles/all-games.toml`. Every game follows it.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct AllGamesProfile {
    /// What All games sets.
    pub values: OwnValues,
}

/// One game's profile, `profiles/<appid>.toml`. A game "has its own
/// profile", the ⚑ marker, when its file exists, even with no own values.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GameProfile {
    /// The Steam AppID, from the file name.
    pub app_id: u64,
    /// The game's name, as the setters fill it and `profile list` prints it.
    /// Empty when nothing filled it.
    pub name: String,
    /// What the game sets itself.
    pub values: OwnValues,
}

impl GameProfile {
    /// An empty profile: every field follows All games.
    pub fn new(app_id: u64, name: impl Into<String>) -> GameProfile {
        GameProfile {
            app_id,
            name: name.into(),
            values: OwnValues::new(),
        }
    }
}

/// What "Apply to games with their own value" did: one locked write per
/// game, so some can fail while the rest change.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct FollowOutcome {
    /// The games that now follow, by name, sorted.
    pub followed: Vec<String>,
    /// The games that could not be written, by name, sorted.
    pub failed: Vec<String>,
    /// Why each failed game failed, by name.
    pub errors: BTreeMap<String, String>,
}

/// The names of the games that set `key` themselves, sorted, as the All
/// games page counts them (Go's `CollectOwnValueGames`).
pub fn own_value_games<'a>(
    games: impl IntoIterator<Item = (&'a str, &'a OwnValues)>,
    key: FieldKey,
) -> Vec<String> {
    let mut names: Vec<String> = games
        .into_iter()
        .filter(|(_, values)| values.contains(key))
        .map(|(name, _)| name.to_owned())
        .collect();
    names.sort();
    names
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn own_values_check_and_follow() {
        let mut own = OwnValues::new();
        assert!(own.is_empty());
        assert_eq!(own.set(FieldKey::GpuPowerLimit, Value::Int(250)), Ok(None));
        assert_eq!(
            own.set(FieldKey::GpuPowerLimit, Value::Int(0)),
            Ok(Some(Value::Int(250))),
            "an explicit zero is a value"
        );
        assert!(
            own.set(FieldKey::DlssSrMode, Value::Text("fast".into()))
                .is_err()
        );
        assert_eq!(
            own.set(FieldKey::CpuSmt, Value::OptionalBool(None)),
            Ok(None),
            "a game may leave SMT unchanged (decision 65)"
        );
        assert_eq!(
            own.set(FieldKey::CpuSmt, Value::OptionalBool(Some(false))),
            Ok(Some(Value::OptionalBool(None)))
        );
        own.set(FieldKey::ProtonEnableHdr, Value::Bool(true))
            .unwrap();
        let keys: Vec<FieldKey> = own.iter().map(|(k, _)| k).collect();
        assert_eq!(
            keys,
            [
                FieldKey::ProtonEnableHdr,
                FieldKey::GpuPowerLimit,
                FieldKey::CpuSmt
            ],
            "table order"
        );
        assert_eq!(own.in_section(Section::Gpu).count(), 1);
        assert_eq!(own.follow(FieldKey::GpuPowerLimit), Some(Value::Int(0)));
        assert!(!own.contains(FieldKey::GpuPowerLimit));
        assert_eq!(own.follow_all(), 2);
        assert!(own.is_empty());
    }

    #[test]
    fn own_value_games_are_sorted_names() {
        let mut a = OwnValues::new();
        a.set(FieldKey::DlssSrMode, Value::Text("dlaa".into()))
            .unwrap();
        let b = OwnValues::new();
        let mut c = OwnValues::new();
        c.set(FieldKey::DlssSrMode, Value::Text(String::new()))
            .unwrap();
        let games = [("Zeta", &a), ("Beta", &b), ("Alpha", &c)];
        assert_eq!(
            own_value_games(games, FieldKey::DlssSrMode),
            ["Alpha", "Zeta"]
        );
    }
}
