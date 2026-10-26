//! `--json` output, schema version 1: the objects `scan`, `list`, `show`
//! and the profile shows print (decision 7).
//!
//! The rules, from the architecture's "JSON output, schema version 1":
//!
//! - every output is one object with `schema_version` as its first key;
//! - keys are snake_case and come in a fixed order, the field order of the
//!   structs here;
//! - a list is always present, `[]` when empty;
//! - a value that can be missing is present as `null`;
//! - values are raw: numbers are numbers, a unit goes in its own key, times
//!   are RFC 3339 in UTC and app ids are numbers;
//! - games sort by name, then app id; DLLs in catalogue order, then path;
//!   profile fields in table order;
//! - origins are Go's origin names, never display text.
//!
//! A new key keeps version 1. Removing or renaming a key, or changing a
//! value's type or meaning, raises [`SCHEMA_VERSION`] for every command.

use std::path::Path;

use jiff::Timestamp;
use serde::Serialize;

use crate::games::{DetectedDll, DllType, Game};
use crate::profile::{Effective, Origin, Resolved, Scope, Section, Value};

/// The schema version every `--json` object starts with.
pub const SCHEMA_VERSION: u32 = 1;

/// `scan --json` and `list --json`: every listed game.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct GameList<'a> {
    /// [`SCHEMA_VERSION`].
    pub schema_version: u32,
    /// The games, by name, then app id.
    pub games: Vec<GameJson<'a>>,
}

/// `show <game> --json`: one game.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct GameShow<'a> {
    /// [`SCHEMA_VERSION`].
    pub schema_version: u32,
    /// The game.
    pub game: GameJson<'a>,
}

/// A game: the cache fields plus `has_profile`.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct GameJson<'a> {
    /// The Steam AppID.
    pub app_id: u64,
    /// The name from the app manifest.
    pub name: &'a str,
    /// The install folder.
    pub install_dir: &'a Path,
    /// The Steam library the game is in.
    pub library_path: &'a Path,
    /// The Proton prefix, or `null`.
    pub prefix_path: Option<&'a Path>,
    /// When the scan read the game, in UTC.
    pub scanned_at: Timestamp,
    /// Whether the game has its own profile.
    pub has_profile: bool,
    /// The DLLs, in catalogue order, then path.
    pub dlls: Vec<DllJson<'a>>,
}

/// One DLL of a game.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct DllJson<'a> {
    /// The absolute path.
    pub path: &'a Path,
    /// The file name as found.
    pub name: &'a str,
    /// The family: `dlss`, `dlssg`, `dlssd`, `xess` or `fsr`.
    #[serde(rename = "type")]
    pub dll_type: DllType,
    /// The version, or `null` when the file has none that reads.
    pub version: Option<&'a str>,
}

/// `profile show --json` and the section shows: a scope's fields, each
/// with its stored and resolved view.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct ProfileJson<'a> {
    /// [`SCHEMA_VERSION`].
    pub schema_version: u32,
    /// `game` or `all_games`.
    pub scope: &'static str,
    /// The game's AppID, `null` for All games.
    pub app_id: Option<u64>,
    /// The game's name, `null` for All games.
    pub name: Option<&'a str>,
    /// Whether the scope has its own profile file.
    pub has_profile: bool,
    /// The fields, in table order.
    pub fields: Vec<FieldJson<'a>>,
}

/// One profile field.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct FieldJson<'a> {
    /// The full key, such as `gpu.power_limit`.
    pub key: &'static str,
    /// The section.
    pub section: Section,
    /// `bool`, `int`, `string` or `optional_bool`. It fixes the type of
    /// the three values.
    pub kind: &'static str,
    /// The unit of a number, such as `W`, or `null`.
    pub unit: Option<&'static str>,
    /// The effective value. `null` only for an `optional_bool` at none.
    pub value: &'a Value,
    /// The scope's own value, `null` when it follows.
    pub own_value: Option<&'a Value>,
    /// The All games value, `null` when All games does not set it, and
    /// always `null` on the All games scope.
    pub all_games_value: Option<&'a Value>,
    /// Where the value comes from: Go's origin name.
    pub origin: Origin,
}

/// A game as JSON. `has_profile` comes from the caller, which knows the
/// profile folder.
pub fn game(game: &Game, has_profile: bool) -> GameJson<'_> {
    let mut dlls: Vec<&DetectedDll> = game.dlls.iter().collect();
    dlls.sort_by(|a, b| {
        a.dll_type
            .cmp(&b.dll_type)
            .then_with(|| a.path.cmp(&b.path))
    });
    GameJson {
        app_id: game.app_id,
        name: &game.name,
        install_dir: &game.install_dir,
        library_path: &game.library_path,
        prefix_path: game.prefix_path.as_deref(),
        scanned_at: game.scanned_at,
        has_profile,
        dlls: dlls
            .into_iter()
            .map(|d| DllJson {
                path: &d.path,
                name: &d.name,
                dll_type: d.dll_type,
                version: d.version.as_deref().filter(|v| !v.is_empty()),
            })
            .collect(),
    }
}

/// The game list object, sorted by name, then app id.
pub fn game_list<'a>(
    games: impl IntoIterator<Item = &'a Game>,
    has_profile: impl Fn(u64) -> bool,
) -> GameList<'a> {
    let mut games: Vec<&Game> = games.into_iter().collect();
    games.sort_by(|a, b| a.name.cmp(&b.name).then(a.app_id.cmp(&b.app_id)));
    GameList {
        schema_version: SCHEMA_VERSION,
        games: games
            .into_iter()
            .map(|g| game(g, has_profile(g.app_id)))
            .collect(),
    }
}

/// The `show` object.
pub fn game_show(g: &Game, has_profile: bool) -> GameShow<'_> {
    GameShow {
        schema_version: SCHEMA_VERSION,
        game: game(g, has_profile),
    }
}

/// One resolved field as JSON.
pub fn field(resolved: &Resolved) -> FieldJson<'_> {
    let field = resolved.key.field();
    FieldJson {
        key: field.key,
        section: field.section,
        kind: field.kind.name(),
        unit: field.unit,
        value: &resolved.value,
        own_value: resolved.own.as_ref(),
        all_games_value: resolved.all_games.as_ref(),
        origin: resolved.origin,
    }
}

/// The profile object of a scope: every field, or one section's. `name`
/// is the game's name and is left out on All games.
pub fn profile<'a>(
    effective: &'a Effective,
    name: &'a str,
    has_profile: bool,
    section: Option<Section>,
) -> ProfileJson<'a> {
    let (scope, app_id, name) = match effective.scope {
        Scope::AllGames => ("all_games", None, None),
        Scope::Game(app_id) => ("game", Some(app_id), Some(name)),
    };
    ProfileJson {
        schema_version: SCHEMA_VERSION,
        scope,
        app_id,
        name,
        has_profile,
        fields: effective
            .iter()
            .filter(|r| section.is_none_or(|s| r.key.field().section == s))
            .map(field)
            .collect(),
    }
}

/// The pretty printed text of an object, ending with a newline.
pub fn to_text(value: &impl Serialize) -> Result<String, serde_json::Error> {
    let mut text = serde_json::to_string_pretty(value)?;
    text.push('\n');
    Ok(text)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::profile::{FieldKey, OwnValues};

    fn sample_game() -> Game {
        let dll = |name: &str, dll_type, version: Option<&str>| DetectedDll {
            path: format!("/lib/steamapps/common/G/bin/{name}").into(),
            name: name.to_owned(),
            dll_type,
            version: version.map(str::to_owned),
        };
        Game {
            app_id: 9100010,
            name: "Glimmer Run".into(),
            install_dir: "/lib/steamapps/common/G".into(),
            library_path: "/lib".into(),
            prefix_path: None,
            scanned_at: "2026-10-26T17:02:10.5+02:00".parse().unwrap(),
            dlls: vec![
                dll("nvngx_dlssd.dll", DllType::Dlssd, Some("310.5.3")),
                dll("libxess.dll", DllType::Xess, None),
                dll("nvngx_dlssg.dll", DllType::Dlssg, Some("")),
                dll("nvngx_dlss.dll", DllType::Dlss, Some("310.2.1")),
            ],
        }
    }

    #[test]
    fn a_game_has_every_key_in_order_with_nulls_and_utc_times() {
        let g = sample_game();
        let text = to_text(&game_show(&g, true)).unwrap();
        assert!(text.starts_with("{\n  \"schema_version\": 1,"), "{text}");
        let v: serde_json::Value = serde_json::from_str(&text).unwrap();
        let game = &v["game"];
        assert_eq!(game.as_object().unwrap().len(), 8, "{text}");
        assert_eq!(game["prefix_path"], serde_json::Value::Null);
        assert_eq!(game["scanned_at"], "2026-10-26T15:02:10.5Z");
        assert_eq!(game["has_profile"], true);
        let types: Vec<&str> = game["dlls"]
            .as_array()
            .unwrap()
            .iter()
            .map(|d| d["type"].as_str().unwrap())
            .collect();
        assert_eq!(types, ["dlss", "dlssg", "dlssd", "xess"]);
        assert_eq!(game["dlls"][1]["version"], serde_json::Value::Null);
        assert_eq!(game["dlls"][3]["version"], serde_json::Value::Null);
        assert!(text.ends_with("}\n"));
        let order: Vec<usize> = [
            "\"app_id\"",
            "\"name\"",
            "\"install_dir\"",
            "\"library_path\"",
            "\"prefix_path\"",
            "\"scanned_at\"",
            "\"has_profile\"",
            "\"dlls\"",
        ]
        .iter()
        .map(|k| text.find(k).unwrap())
        .collect();
        assert!(order.is_sorted(), "keys in the fixed order: {text}");
    }

    #[test]
    fn an_empty_list_is_an_empty_array() {
        let text = to_text(&game_list([], |_| false)).unwrap();
        assert_eq!(text, "{\n  \"schema_version\": 1,\n  \"games\": []\n}\n");
    }

    #[test]
    fn games_sort_by_name_then_app_id() {
        let mut a = sample_game();
        a.name = "Beta".into();
        a.app_id = 2;
        let mut b = sample_game();
        b.name = "Alpha".into();
        b.app_id = 3;
        let mut c = sample_game();
        c.name = "Beta".into();
        c.app_id = 1;
        let list = game_list([&a, &b, &c], |id| id == 1);
        let ids: Vec<u64> = list.games.iter().map(|g| g.app_id).collect();
        assert_eq!(ids, [3, 1, 2]);
        assert!(list.games[1].has_profile);
    }

    #[test]
    fn fields_carry_both_views_and_go_s_origin_names() {
        let mut own = OwnValues::new();
        own.set(FieldKey::GpuPowerLimit, Value::Int(250)).unwrap();
        let mut all = OwnValues::new();
        all.set(FieldKey::ProtonEnableWayland, Value::Bool(true))
            .unwrap();
        let effective = Effective::of_game(9100010, Some(&own), Some(&all));
        let json = profile(&effective, "Glimmer Run", true, None);
        assert_eq!(json.fields.len(), 25);
        let v = serde_json::to_value(&json).unwrap();
        let find = |key: &str| {
            v["fields"]
                .as_array()
                .unwrap()
                .iter()
                .find(|f| f["key"] == key)
                .unwrap()
                .clone()
        };
        assert_eq!(
            find("gpu.power_limit"),
            serde_json::json!({
                "key": "gpu.power_limit", "section": "gpu", "kind": "int", "unit": "W",
                "value": 250, "own_value": 250, "all_games_value": null, "origin": "this_game"
            })
        );
        let wayland = find("proton.enable_wayland");
        assert_eq!(wayland["own_value"], serde_json::Value::Null);
        assert_eq!(wayland["all_games_value"], true);
        assert_eq!(wayland["origin"], "all_games");
        let smt = find("cpu.smt");
        assert_eq!(smt["kind"], "optional_bool");
        assert_eq!(smt["value"], serde_json::Value::Null);
        assert_eq!(smt["origin"], "system_default");
        let dlss = profile(&effective, "Glimmer Run", true, Some(Section::Dlss));
        assert!(dlss.fields.iter().all(|f| f.section == Section::Dlss));
        assert_eq!(dlss.fields.len(), 11);
    }

    #[test]
    fn all_games_has_null_identity() {
        let effective = Effective::of_all_games(None);
        let v = serde_json::to_value(profile(&effective, "ignored", false, None)).unwrap();
        assert_eq!(v["scope"], "all_games");
        assert_eq!(v["app_id"], serde_json::Value::Null);
        assert_eq!(v["name"], serde_json::Value::Null);
        assert!(
            v["fields"]
                .as_array()
                .unwrap()
                .iter()
                .all(|f| f["origin"] == "system_default")
        );
    }
}
