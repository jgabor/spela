//! Effective values and where they come from.
//!
//! Every field resolves the same way, as Go's `ResolveForApply` does: the
//! game's own value when its key is present, else the All games value when
//! that key is present, else the system default. With no game profile at
//! all, launch applies All games directly, which is resolving with no game
//! values. The origin words are Go's (`internal/profile/explanation.go`).

use serde::Serialize;

use super::field::{FieldKey, Section};
use super::value::Value;
use super::{OwnValues, Scope};

/// Where a shown value comes from. The JSON strings are Go's.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Origin {
    /// The game sets it, and the value differs from the All games value.
    ThisGame,
    /// The game sets it, and the value equals the All games value, so it
    /// will not follow later changes there. When All games does not set
    /// it, the value is the zero of its kind (SMT Off counts, as in Go).
    ThisGameSame,
    /// The game follows, and All games sets it.
    AllGames,
    /// Nothing sets it.
    SystemDefault,
    /// All games scope only: All games sets it.
    SetHere,
}

impl Origin {
    /// Go's origin name, as `--json` prints it.
    pub fn as_str(self) -> &'static str {
        match self {
            Origin::ThisGame => "this_game",
            Origin::ThisGameSame => "this_game_same",
            Origin::AllGames => "all_games",
            Origin::SystemDefault => "system_default",
            Origin::SetHere => "set_here",
        }
    }

    /// The coarse source: the scope's own value, All games, or the default.
    pub fn source(self) -> Source {
        match self {
            Origin::ThisGame | Origin::ThisGameSame | Origin::SetHere => Source::Own,
            Origin::AllGames => Source::AllGames,
            Origin::SystemDefault => Source::SystemDefault,
        }
    }
}

/// Which layer a value comes from, without Go's finer split of own values.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Source {
    /// The scope sets it itself: a game's own value, or a value All games
    /// sets on its own page. The ⚑ marker shows it.
    Own,
    /// The game follows All games, which sets it.
    AllGames,
    /// Nothing sets it: the built-in default.
    SystemDefault,
}

/// One field, resolved in one scope. It carries both the stored and the
/// resolved view, which is what `--json` prints for a field.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Resolved {
    /// The field.
    pub key: FieldKey,
    /// The effective value.
    pub value: Value,
    /// The scope's own value, or `None` when it follows. On All games, the
    /// value set there. SMT left unchanged on purpose is
    /// `Some(Value::OptionalBool(None))` (decision 65).
    pub own: Option<Value>,
    /// The All games value, or `None` when All games does not set it.
    /// Always `None` on the All games scope itself.
    pub all_games: Option<Value>,
    /// Where the value comes from.
    pub origin: Origin,
}

impl Resolved {
    /// Whether the scope sets this field itself.
    pub fn is_own(&self) -> bool {
        self.own.is_some()
    }
}

/// Resolves one field of a game. `game` is `None` when the game has no
/// profile file, and `all_games` is `None` when there is no All games file.
pub fn resolve(game: Option<&OwnValues>, all_games: Option<&OwnValues>, key: FieldKey) -> Resolved {
    let field = key.field();
    let own = game.and_then(|g| g.get(key)).cloned();
    let all = all_games.and_then(|a| a.get(key)).cloned();
    let (value, origin) = match (&own, &all) {
        (Some(own), Some(all)) if own == all => (own.clone(), Origin::ThisGameSame),
        (Some(own), Some(_)) => (own.clone(), Origin::ThisGame),
        // Go compares with the zero value here, so SMT Off counts as same.
        (Some(own), None) if own.is_zero() => (own.clone(), Origin::ThisGameSame),
        (Some(own), None) => (own.clone(), Origin::ThisGame),
        (None, Some(all)) => (all.clone(), Origin::AllGames),
        (None, None) => (field.default_value(), Origin::SystemDefault),
    };
    Resolved {
        key,
        value,
        own,
        all_games: all,
        origin,
    }
}

/// Resolves one field of the All games profile itself: set here, or the
/// system default.
pub fn resolve_all_games(all_games: Option<&OwnValues>, key: FieldKey) -> Resolved {
    let own = all_games.and_then(|a| a.get(key)).cloned();
    let (value, origin) = match &own {
        Some(v) => (v.clone(), Origin::SetHere),
        None => (key.field().default_value(), Origin::SystemDefault),
    };
    Resolved {
        key,
        value,
        own,
        all_games: None,
        origin,
    }
}

/// Every field of one scope, resolved, in table order. Launch planning,
/// the profile page, `profile show` and `--json` all read this.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Effective {
    /// The scope these values belong to.
    pub scope: Scope,
    fields: Vec<Resolved>,
}

impl Effective {
    /// A game's 25 values. `game` is `None` without a profile file.
    pub fn of_game(
        app_id: u64,
        game: Option<&OwnValues>,
        all_games: Option<&OwnValues>,
    ) -> Effective {
        Effective {
            scope: Scope::Game(app_id),
            fields: FieldKey::all()
                .map(|k| resolve(game, all_games, k))
                .collect(),
        }
    }

    /// The All games profile's 25 values.
    pub fn of_all_games(all_games: Option<&OwnValues>) -> Effective {
        Effective {
            scope: Scope::AllGames,
            fields: FieldKey::all()
                .map(|k| resolve_all_games(all_games, k))
                .collect(),
        }
    }

    /// One field.
    pub fn get(&self, key: FieldKey) -> &Resolved {
        &self.fields[key as usize]
    }

    /// The effective value of one field.
    pub fn value(&self, key: FieldKey) -> &Value {
        &self.get(key).value
    }

    /// Whether the game or All games sets `key`, as opposed to the system
    /// default. `__GL_THREADED_OPTIMIZATION` is exported only then
    /// (decision 47).
    pub fn is_set(&self, key: FieldKey) -> bool {
        self.get(key).origin != Origin::SystemDefault
    }

    /// Every field, in table order.
    pub fn iter(&self) -> impl Iterator<Item = &Resolved> {
        self.fields.iter()
    }

    /// One section's fields, in table order.
    pub fn in_section(&self, section: Section) -> impl Iterator<Item = &Resolved> {
        self.iter()
            .filter(move |r| r.key.field().section == section)
    }

    /// How many settings the page holds itself, which the whole-page
    /// action clears.
    pub fn own_count(&self) -> usize {
        self.iter().filter(|r| r.is_own()).count()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn own(pairs: &[(FieldKey, Value)]) -> OwnValues {
        let mut v = OwnValues::new();
        for (k, val) in pairs {
            v.set(*k, val.clone()).unwrap();
        }
        v
    }

    #[test]
    fn game_then_all_games_then_default() {
        let all = own(&[
            (FieldKey::ProtonEnableWayland, Value::Bool(true)),
            (FieldKey::GpuPowerLimit, Value::Int(250)),
        ]);
        let game = own(&[(FieldKey::GpuPowerLimit, Value::Int(300))]);
        let r = resolve(Some(&game), Some(&all), FieldKey::GpuPowerLimit);
        assert_eq!(
            (r.value.clone(), r.origin),
            (Value::Int(300), Origin::ThisGame)
        );
        assert_eq!(r.own, Some(Value::Int(300)));
        assert_eq!(r.all_games, Some(Value::Int(250)));
        let r = resolve(Some(&game), Some(&all), FieldKey::ProtonEnableWayland);
        assert_eq!(
            (r.value.clone(), r.origin),
            (Value::Bool(true), Origin::AllGames)
        );
        assert_eq!(r.own, None);
        let r = resolve(Some(&game), Some(&all), FieldKey::CpuSmt);
        assert_eq!(
            (r.value.clone(), r.origin),
            (Value::OptionalBool(None), Origin::SystemDefault)
        );
        // No game file: launch applies All games directly.
        let r = resolve(None, Some(&all), FieldKey::GpuPowerLimit);
        assert_eq!((r.value, r.origin), (Value::Int(250), Origin::AllGames));
        let r = resolve(None, None, FieldKey::GpuPowerLimit);
        assert_eq!((r.value, r.origin), (Value::Int(0), Origin::SystemDefault));
    }

    #[test]
    fn same_as_all_games_matches_go_s_oracle() {
        let all = own(&[
            (FieldKey::ProtonVkd3dHeap, Value::Bool(true)),
            (FieldKey::CpuSmt, Value::OptionalBool(Some(true))),
        ]);
        let game = own(&[
            (FieldKey::ProtonVkd3dHeap, Value::Bool(true)),
            (FieldKey::GpuClockOffset, Value::Int(0)),
            (FieldKey::DlssSrMode, Value::Text(String::new())),
            (FieldKey::GpuFanSpeed, Value::Int(60)),
            (FieldKey::CpuSmt, Value::OptionalBool(Some(false))),
        ]);
        let origin = |k| resolve(Some(&game), Some(&all), k).origin;
        assert_eq!(origin(FieldKey::ProtonVkd3dHeap), Origin::ThisGameSame);
        // All games does not set these: equal to the zero is "same".
        assert_eq!(origin(FieldKey::GpuClockOffset), Origin::ThisGameSame);
        assert_eq!(origin(FieldKey::DlssSrMode), Origin::ThisGameSame);
        assert_eq!(origin(FieldKey::GpuFanSpeed), Origin::ThisGame);
        // own-values game 0: smt false against All games true.
        assert_eq!(origin(FieldKey::CpuSmt), Origin::ThisGame);
        // two-space, legacy-no-default and sandbox/main game 0: smt false
        // with All games not setting it is this_game_same in Go.
        let lone = own(&[(FieldKey::CpuSmt, Value::OptionalBool(Some(false)))]);
        assert_eq!(
            resolve(Some(&lone), None, FieldKey::CpuSmt).origin,
            Origin::ThisGameSame
        );
        // yaml11-bools game 0: smt true with All games not setting it.
        let on = own(&[(FieldKey::CpuSmt, Value::OptionalBool(Some(true)))]);
        assert_eq!(
            resolve(Some(&on), None, FieldKey::CpuSmt).origin,
            Origin::ThisGame
        );
    }

    #[test]
    fn a_game_can_leave_smt_unchanged_against_all_games() {
        // explicit-zeros game 9100102: SMT marked with no value, All games
        // false. Go applied no SMT step and reported this_game.
        let all = own(&[(FieldKey::CpuSmt, Value::OptionalBool(Some(false)))]);
        let game = own(&[(FieldKey::CpuSmt, Value::OptionalBool(None))]);
        let r = resolve(Some(&game), Some(&all), FieldKey::CpuSmt);
        assert_eq!(r.value, Value::OptionalBool(None));
        assert_eq!(r.own, Some(Value::OptionalBool(None)));
        assert!(r.is_own());
        assert_eq!(r.origin, Origin::ThisGame);
        let e = Effective::of_game(9100102, Some(&game), Some(&all));
        assert!(e.is_set(FieldKey::CpuSmt));
        assert_eq!(e.own_count(), 1);
        // With All games not setting it, it equals the zero: same.
        assert_eq!(
            resolve(Some(&game), None, FieldKey::CpuSmt).origin,
            Origin::ThisGameSame
        );
        let r = resolve_all_games(Some(&game), FieldKey::CpuSmt);
        assert_eq!(
            (r.value, r.origin),
            (Value::OptionalBool(None), Origin::SetHere)
        );
    }

    #[test]
    fn all_games_scope_is_set_here_or_default() {
        let all = own(&[(FieldKey::DlssSrPreset, Value::Text("auto".into()))]);
        let r = resolve_all_games(Some(&all), FieldKey::DlssSrPreset);
        assert_eq!(r.origin, Origin::SetHere);
        assert_eq!(r.own, Some(Value::Text("auto".into())));
        assert_eq!(r.all_games, None);
        assert_eq!(
            resolve_all_games(Some(&all), FieldKey::DlssSrMode).origin,
            Origin::SystemDefault
        );
        let e = Effective::of_all_games(Some(&all));
        assert_eq!(e.scope, Scope::AllGames);
        assert_eq!(e.own_count(), 1);
        assert_eq!(e.iter().count(), 25);
    }

    #[test]
    fn effective_covers_every_field_in_order() {
        let all = own(&[(FieldKey::GpuThreadedOptimization, Value::Bool(false))]);
        let game = own(&[(FieldKey::GpuPowerLimit, Value::Int(250))]);
        let e = Effective::of_game(9100010, Some(&game), Some(&all));
        assert_eq!(e.scope, Scope::Game(9100010));
        let keys: Vec<FieldKey> = e.iter().map(|r| r.key).collect();
        assert_eq!(keys, FieldKey::all().collect::<Vec<_>>());
        assert_eq!(e.in_section(Section::Cpu).count(), 2);
        assert_eq!(e.value(FieldKey::GpuPowerLimit), &Value::Int(250));
        assert!(e.is_set(FieldKey::GpuThreadedOptimization));
        assert!(!e.is_set(FieldKey::GpuShaderCache));
        assert_eq!(e.own_count(), 1);
        assert_eq!(e.get(FieldKey::GpuPowerLimit).origin.source(), Source::Own);
    }

    #[test]
    fn origin_json_strings_are_go_s() {
        for (o, s) in [
            (Origin::ThisGame, "this_game"),
            (Origin::ThisGameSame, "this_game_same"),
            (Origin::AllGames, "all_games"),
            (Origin::SystemDefault, "system_default"),
            (Origin::SetHere, "set_here"),
        ] {
            assert_eq!(o.as_str(), s);
            assert_eq!(serde_json::to_string(&o).unwrap(), format!("\"{s}\""));
        }
    }
}
