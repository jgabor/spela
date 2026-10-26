//! The words every surface prints for profile values.
//!
//! The CLI and the TUI print these and nothing else, so a value reads the
//! same everywhere. Layout and keys stay with the surfaces. The strings are
//! Go's, pinned by its tests (`internal/profile/vocabulary.go`). Display text
//! never goes into `--json`.

use super::FollowOutcome;
use super::field::Field;
use super::resolve::{Origin, Resolved};
use super::value::{Kind, Value};

/// The name of the profile every game follows.
pub const ALL_GAMES: &str = "All games";

/// A game's field goes back to following All games.
pub const FOLLOW_ALL_GAMES: &str = "Follow All games";
/// An All games field goes back to the system default.
pub const USE_SYSTEM_DEFAULT: &str = "Use system default";
/// Every field of a game follows All games (`spela profile reset`).
pub const FOLLOW_ALL_GAMES_FOR_ALL_SETTINGS: &str = "Follow All games for all settings";
/// Every All games field goes back to the system default.
pub const USE_SYSTEM_DEFAULT_FOR_ALL_SETTINGS: &str = "Use system default for all settings";
/// The games that set a field themselves follow All games for it.
pub const APPLY_TO_GAMES_WITH_OWN_VALUE: &str = "Apply to games with their own value";
/// A followed game field gets its own value.
pub const SET_FOR_THIS_GAME: &str = "Set for this game";
/// An All games field at the system default gets a value.
pub const SET_HERE: &str = "Set here";

/// A true bool.
pub const ON: &str = "On";
/// A false bool.
pub const OFF: &str = "Off";
/// An empty text, SMT's none, or a number nothing sets.
pub const NONE: &str = "none";
/// SMT's none when a scope stores it: SMT is left as it is at launch, even
/// when All games sets it (decision 65).
pub const UNCHANGED: &str = "unchanged";

/// Words no surface may print any more. Surface tests check that none of
/// them appears.
pub const RETIRED: &[&str] = &[
    "(default)",
    "Override for this game",
    "Inherited from defaults",
    "Default for all games",
    "defaults profile",
    "root profile",
    "This change does not reach",
    "Effect:",
];

/// How many game names a sentence lists before it says "and N more".
const NAMES_SHOWN: usize = 6;

/// A value for people: On and Off, none for empty text and SMT's none, and
/// numbers with their unit (`150 MHz`, `250 W`, `60%`).
pub fn display_value(field: &Field, value: &Value) -> String {
    match value {
        Value::Bool(true) | Value::OptionalBool(Some(true)) => ON.to_owned(),
        Value::Bool(false) | Value::OptionalBool(Some(false)) => OFF.to_owned(),
        Value::OptionalBool(None) => NONE.to_owned(),
        Value::Int(n) => match field.unit {
            None => n.to_string(),
            Some("%") => format!("{n}%"),
            Some(unit) => format!("{n} {unit}"),
        },
        Value::Text(t) if t.is_empty() => NONE.to_owned(),
        Value::Text(t) => t.clone(),
    }
}

/// The effective value of a resolved field. A number that nothing sets
/// reads none, not 0, because zero means "leave it alone". SMT's none that
/// a scope stores reads unchanged, so it is not taken for "nothing sets it".
pub fn value_text(field: &Field, resolved: &Resolved) -> String {
    if resolved.origin == Origin::SystemDefault {
        if field.kind == Kind::Int {
            return NONE.to_owned();
        }
        return display_value(field, &resolved.value);
    }
    stored_value_text(field, &resolved.value)
}

/// A value a scope stores: as [`display_value`], except that SMT's none
/// reads unchanged.
fn stored_value_text(field: &Field, value: &Value) -> String {
    match value {
        Value::OptionalBool(None) => UNCHANGED.to_owned(),
        other => display_value(field, other),
    }
}

/// Where a value comes from. A game's own value also shows the All games
/// value it replaced, so the choice can be judged on one line.
pub fn source_text(field: &Field, resolved: &Resolved) -> String {
    match resolved.origin {
        Origin::ThisGame => format!(
            "this game \u{b7} {ALL_GAMES}: {}",
            all_games_value_text(field, resolved)
        ),
        Origin::ThisGameSame => format!("this game \u{b7} same as {ALL_GAMES}"),
        Origin::AllGames => format!("from {ALL_GAMES}"),
        Origin::SetHere => "set here".to_owned(),
        Origin::SystemDefault => "system default".to_owned(),
    }
}

fn all_games_value_text(field: &Field, resolved: &Resolved) -> String {
    match &resolved.all_games {
        Some(v) => stored_value_text(field, v),
        None if field.kind == Kind::Int => NONE.to_owned(),
        None => display_value(field, &field.default_value()),
    }
}

/// The action that clears one value in a scope.
pub fn reset_action(all_games_scope: bool) -> &'static str {
    if all_games_scope {
        USE_SYSTEM_DEFAULT
    } else {
        FOLLOW_ALL_GAMES
    }
}

/// The action that clears every setting a page holds itself.
pub fn clear_all_action(all_games_scope: bool) -> &'static str {
    if all_games_scope {
        USE_SYSTEM_DEFAULT_FOR_ALL_SETTINGS
    } else {
        FOLLOW_ALL_GAMES_FOR_ALL_SETTINGS
    }
}

/// How many settings the whole-page action clears, and where they go.
pub fn clear_all_body(all_games_scope: bool, count: usize) -> String {
    let (owner, target) = if all_games_scope {
        (format!("{ALL_GAMES} sets"), "the system default")
    } else {
        ("this game sets".to_owned(), ALL_GAMES)
    };
    match count {
        0 => format!("Nothing to clear: every value already follows {target}."),
        1 => format!(
            "This clears the 1 setting that {owner} itself. It follows {target} afterwards. It is saved at once."
        ),
        n => format!(
            "This clears the {n} settings that {owner} itself. They follow {target} afterwards. It is saved at once."
        ),
    }
}

/// How many games set their own value for a field, on the All games page.
/// Empty for none.
pub fn own_value_suffix(count: usize) -> String {
    match count {
        0 => String::new(),
        1 => "1 game has its own value".to_owned(),
        n => format!("{n} games have their own value"),
    }
}

/// A count of games: `1 game`, `4 games`.
pub fn games_text(count: usize) -> String {
    if count == 1 {
        "1 game".to_owned()
    } else {
        format!("{count} games")
    }
}

/// The question before All games is applied to the games that set their
/// own value. It names the games and the value they will follow, which is
/// left out when `value_text` is empty.
pub fn apply_to_own_value_body(label: &str, value_text: &str, names: &[String]) -> String {
    let mut list = names[..names.len().min(NAMES_SHOWN)].join(", ");
    if names.len() > NAMES_SHOWN {
        list.push_str(&format!(" and {} more", names.len() - NAMES_SHOWN));
    }
    let (subject, verb) = if names.len() == 1 {
        ("1 game has".to_owned(), "It")
    } else {
        (format!("{} games have", names.len()), "They")
    };
    let mut follow = ALL_GAMES.to_owned();
    if !value_text.is_empty() {
        follow.push_str(&format!(" ({value_text})"));
    }
    format!(
        "{label}: {subject} their own value: {list}. {verb} will follow {follow}. It is saved at once and cannot be undone."
    )
}

/// The games that could not be written, each with its reason, such as
/// `Beta (disk full)`. Empty when every write worked.
pub fn follow_failure_text(outcome: &FollowOutcome) -> String {
    outcome
        .failed
        .iter()
        .map(|name| {
            let why = outcome.errors.get(name).map_or("", String::as_str);
            format!("{name} ({why})")
        })
        .collect::<Vec<_>>()
        .join(", ")
}

/// What an applied follow did. A failure comes first, then the games that
/// did follow.
pub fn follow_outcome_text(label: &str, outcome: &FollowOutcome) -> String {
    let n = outcome.followed.len();
    let verb = if n == 1 { "follows" } else { "follow" };
    let text = format!("{label}: {} {verb} {ALL_GAMES}", games_text(n));
    let failure = follow_failure_text(outcome);
    if failure.is_empty() {
        text
    } else {
        format!("Not saved for {failure}. {text}")
    }
}

/// The unit and shown range of a free number. Empty for any other field.
pub fn number_hint(field: &Field) -> String {
    if field.kind != Kind::Int || !field.choices().is_empty() {
        return String::new();
    }
    let mut hint = "Whole number".to_owned();
    if let Some(unit) = field.unit {
        hint.push_str(&format!(" in {unit}"));
    }
    if let Some((min, max)) = field.range {
        hint.push_str(&format!(", shown range {min} to {max}"));
    }
    hint.push('.');
    hint
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::profile::{FieldKey, OwnValues, resolve::resolve};

    fn f(key: FieldKey) -> &'static Field {
        key.field()
    }

    #[test]
    fn values_read_as_go_prints_them() {
        let d = |k, v: Value| display_value(f(k), &v);
        assert_eq!(d(FieldKey::GpuClockOffset, Value::Int(150)), "150 MHz");
        assert_eq!(d(FieldKey::GpuPowerLimit, Value::Int(250)), "250 W");
        assert_eq!(d(FieldKey::GpuFanSpeed, Value::Int(60)), "60%");
        assert_eq!(d(FieldKey::DlssMultiFrame, Value::Int(2)), "2");
        assert_eq!(d(FieldKey::ProtonEnableHdr, Value::Bool(true)), "On");
        assert_eq!(d(FieldKey::ProtonEnableHdr, Value::Bool(false)), "Off");
        assert_eq!(d(FieldKey::CpuSmt, Value::OptionalBool(None)), "none");
        assert_eq!(d(FieldKey::CpuSmt, Value::OptionalBool(Some(false))), "Off");
        assert_eq!(d(FieldKey::DlssSrMode, Value::Text(String::new())), "none");
        assert_eq!(d(FieldKey::DlssSrMode, Value::Text("dlaa".into())), "dlaa");
    }

    #[test]
    fn value_and_source_texts() {
        let mut all = OwnValues::new();
        all.set(FieldKey::ProtonEnableHdr, Value::Bool(false))
            .unwrap();
        let mut game = OwnValues::new();
        game.set(FieldKey::ProtonEnableHdr, Value::Bool(true))
            .unwrap();
        game.set(FieldKey::GpuClockOffset, Value::Int(150)).unwrap();
        game.set(FieldKey::GpuPowerLimit, Value::Int(0)).unwrap();
        let r = |k| resolve(Some(&game), Some(&all), k);
        let hdr = r(FieldKey::ProtonEnableHdr);
        assert_eq!(
            source_text(f(FieldKey::ProtonEnableHdr), &hdr),
            "this game · All games: Off"
        );
        let clock = r(FieldKey::GpuClockOffset);
        assert_eq!(
            source_text(f(FieldKey::GpuClockOffset), &clock),
            "this game · All games: none"
        );
        assert_eq!(value_text(f(FieldKey::GpuClockOffset), &clock), "150 MHz");
        let power = r(FieldKey::GpuPowerLimit);
        assert_eq!(value_text(f(FieldKey::GpuPowerLimit), &power), "0 W");
        assert_eq!(
            source_text(f(FieldKey::GpuPowerLimit), &power),
            "this game · same as All games"
        );
        let fan = r(FieldKey::GpuFanSpeed);
        assert_eq!(value_text(f(FieldKey::GpuFanSpeed), &fan), "none");
        assert_eq!(
            source_text(f(FieldKey::GpuFanSpeed), &fan),
            "system default"
        );
        let game_none = resolve(None, Some(&all), FieldKey::ProtonEnableHdr);
        assert_eq!(
            source_text(f(FieldKey::ProtonEnableHdr), &game_none),
            "from All games"
        );
        let sr = r(FieldKey::DlssSrMode);
        assert!(!source_text(f(FieldKey::DlssSrMode), &sr).contains('\u{2014}'));
    }

    #[test]
    fn smt_left_unchanged_reads_unchanged_where_a_scope_stores_it() {
        let smt = f(FieldKey::CpuSmt);
        let mut all = OwnValues::new();
        all.set(FieldKey::CpuSmt, Value::OptionalBool(Some(true)))
            .unwrap();
        let mut game = OwnValues::new();
        game.set(FieldKey::CpuSmt, Value::OptionalBool(None))
            .unwrap();
        let own = resolve(Some(&game), Some(&all), FieldKey::CpuSmt);
        assert_eq!(value_text(smt, &own), "unchanged");
        assert_eq!(source_text(smt, &own), "this game · All games: On");
        // A game that follows an All games "unchanged".
        let follows = resolve(None, Some(&game), FieldKey::CpuSmt);
        assert_eq!(value_text(smt, &follows), "unchanged");
        assert_eq!(source_text(smt, &follows), "from All games");
        // All games On against a game that leaves it unchanged.
        let reverse = resolve(Some(&all), Some(&game), FieldKey::CpuSmt);
        assert_eq!(
            source_text(smt, &reverse),
            "this game · All games: unchanged"
        );
        // Nothing sets it: still none.
        let nothing = resolve(None, None, FieldKey::CpuSmt);
        assert_eq!(value_text(smt, &nothing), "none");
        assert!(!RETIRED.contains(&UNCHANGED));
    }

    #[test]
    fn page_words() {
        assert_eq!(reset_action(false), "Follow All games");
        assert_eq!(reset_action(true), "Use system default");
        assert_eq!(
            clear_all_action(true),
            "Use system default for all settings"
        );
        assert_eq!(
            clear_all_body(false, 0),
            "Nothing to clear: every value already follows All games."
        );
        assert_eq!(
            clear_all_body(true, 0),
            "Nothing to clear: every value already follows the system default."
        );
        assert_eq!(
            clear_all_body(false, 1),
            "This clears the 1 setting that this game sets itself. It follows All games afterwards. It is saved at once."
        );
        assert_eq!(
            clear_all_body(true, 3),
            "This clears the 3 settings that All games sets itself. They follow the system default afterwards. It is saved at once."
        );
        assert_eq!(own_value_suffix(0), "");
        assert_eq!(own_value_suffix(1), "1 game has its own value");
        assert_eq!(own_value_suffix(4), "4 games have their own value");
        assert_eq!(games_text(1), "1 game");
        assert_eq!(games_text(0), "0 games");
    }

    #[test]
    fn apply_and_follow_sentences() {
        let names: Vec<String> = ["A"].iter().map(|s| (*s).to_owned()).collect();
        assert_eq!(
            apply_to_own_value_body("HDR", "On", &names),
            "HDR: 1 game has their own value: A. It will follow All games (On). It is saved at once and cannot be undone."
        );
        let many: Vec<String> = (1..=8).map(|i| format!("G{i}")).collect();
        assert_eq!(
            apply_to_own_value_body("Frame generation", "", &many),
            "Frame generation: 8 games have their own value: G1, G2, G3, G4, G5, G6 and 2 more. They will follow All games. It is saved at once and cannot be undone."
        );
        let mut outcome = FollowOutcome {
            followed: vec!["Alpha".into()],
            ..FollowOutcome::default()
        };
        assert_eq!(
            follow_outcome_text("HDR", &outcome),
            "HDR: 1 game follows All games"
        );
        outcome.followed.push("Gamma".into());
        outcome.failed.push("Beta".into());
        outcome.errors.insert("Beta".into(), "disk full".into());
        assert_eq!(
            follow_outcome_text("HDR", &outcome),
            "Not saved for Beta (disk full). HDR: 2 games follow All games"
        );
    }

    #[test]
    fn number_hints() {
        assert_eq!(
            number_hint(f(FieldKey::GpuClockOffset)),
            "Whole number in MHz."
        );
        assert_eq!(
            number_hint(f(FieldKey::GpuFanSpeed)),
            "Whole number in %, shown range 0 to 100."
        );
        assert_eq!(number_hint(f(FieldKey::DlssMultiFrame)), "");
        assert_eq!(number_hint(f(FieldKey::ProtonEnableHdr)), "");
    }

    #[test]
    fn no_field_text_uses_a_retired_word() {
        for field in &crate::profile::FIELDS {
            for word in RETIRED {
                assert!(!field.label.contains(word), "{} label", field.key);
                assert!(
                    !field.description.contains(word),
                    "{} description",
                    field.key
                );
            }
        }
    }
}
