//! Go's `<data>/games.yaml` to the game cache, `<cache>/games.json`.
//!
//! Every game and DLL is copied, Steam tools included, because the read
//! side filters tool names as Go's did. Times are parsed from Go's RFC 3339
//! with nanoseconds. A DLL of a family spela does not know is left out with
//! a log line: the cache is rebuildable, so the next scan finds it again.

use std::collections::BTreeMap;
use std::path::PathBuf;

use jiff::Timestamp;
use serde::Deserialize;

use crate::games::{CACHE_VERSION, DetectedDll, DllType, Game, GameCache};

use super::yaml::{self, YamlError};

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct DatabaseYaml {
    games: Option<BTreeMap<u64, GameYaml>>,
    updated_at: Option<String>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct GameYaml {
    app_id: Option<u64>,
    name: Option<String>,
    install_dir: Option<String>,
    library_path: Option<String>,
    prefix_path: Option<String>,
    dlls: Option<Vec<DllYaml>>,
    scanned_at: Option<String>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
struct DllYaml {
    path: Option<String>,
    name: Option<String>,
    #[serde(rename = "type")]
    dll_type: Option<String>,
    version: Option<String>,
}

/// Go's zero `time.Time`, which a game with no `scanned_at` holds.
const GO_ZERO_TIME: &str = "0001-01-01T00:00:00Z";

/// Converts the text of Go's `games.yaml`.
pub fn convert(text: &str) -> Result<GameCache, YamlError> {
    let go: DatabaseYaml = yaml::from_str(text)?;
    let mut cache = GameCache {
        version: CACHE_VERSION,
        updated_at: match go.updated_at.as_deref() {
            None | Some(GO_ZERO_TIME) => None,
            Some(t) => Some(time(t)?),
        },
        games: Vec::new(),
    };
    for (key, game) in go.games.unwrap_or_default() {
        let app_id = game.app_id.filter(|id| *id != 0).unwrap_or(key);
        let mut dlls = Vec::new();
        for dll in game.dlls.unwrap_or_default() {
            let kind = dll.dll_type.unwrap_or_default();
            let Some(dll_type) = DllType::from_name(&kind) else {
                tracing::warn!(
                    app_id,
                    kind,
                    "a DLL of a family spela does not know is left out of the game cache"
                );
                continue;
            };
            dlls.push(DetectedDll {
                path: PathBuf::from(dll.path.unwrap_or_default()),
                name: dll.name.unwrap_or_default(),
                dll_type,
                version: dll.version.filter(|v| !v.is_empty()),
            });
        }
        cache.upsert(Game {
            app_id,
            name: game.name.unwrap_or_default(),
            install_dir: PathBuf::from(game.install_dir.unwrap_or_default()),
            library_path: PathBuf::from(game.library_path.unwrap_or_default()),
            prefix_path: game
                .prefix_path
                .filter(|p| !p.is_empty())
                .map(PathBuf::from),
            scanned_at: time(game.scanned_at.as_deref().unwrap_or(GO_ZERO_TIME))?,
            dlls,
        });
    }
    Ok(cache)
}

fn time(text: &str) -> Result<Timestamp, YamlError> {
    text.parse()
        .map_err(|e| yaml::error(format!("invalid time {text:?}: {e}")))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn lines(parts: &[&str]) -> String {
        let mut text = parts.join("\n");
        text.push('\n');
        text
    }

    #[test]
    fn games_dlls_and_times_are_copied() {
        let text = lines(&[
            "games:",
            "    9100302:",
            "        app_id: 9100302",
            "        name: Harbor Lights",
            "        install_dir: /lib/steamapps/common/Harbor Lights",
            "        library_path: /lib",
            "        prefix_path: /lib/steamapps/compatdata/9100302",
            "        dlls:",
            "            - path: /lib/steamapps/common/Harbor Lights/libxess.dll",
            "              name: libxess.dll",
            "              type: xess",
            "              version: 1.3.1.33",
            "            - path: /lib/x.dll",
            "              name: x.dll",
            "              type: optiscaler",
            "        scanned_at: 2026-10-26T17:02:11.123456789+02:00",
            "    9100301:",
            "        name: Glimmer Run",
            "        install_dir: /lib/a",
            "        library_path: /lib",
            "updated_at: 2026-10-26T02:45:16.375232404+02:00",
        ]);
        let cache = convert(&text).unwrap();
        let ids: Vec<u64> = cache.games.iter().map(|g| g.app_id).collect();
        assert_eq!(ids, [9100301, 9100302]);
        let harbor = cache.get(9100302).unwrap();
        assert_eq!(harbor.dlls.len(), 1, "an unknown family is left out");
        assert_eq!(harbor.dlls[0].version.as_deref(), Some("1.3.1.33"));
        assert_eq!(
            harbor.scanned_at,
            "2026-10-26T15:02:11.123456789Z"
                .parse::<Timestamp>()
                .unwrap()
        );
        let glimmer = cache.get(9100301).unwrap();
        assert_eq!(glimmer.prefix_path, None);
        assert_eq!(
            glimmer.scanned_at,
            GO_ZERO_TIME.parse::<Timestamp>().unwrap()
        );
        assert!(cache.updated_at.is_some());
        assert!(convert("games: [\n").is_err());
        assert!(convert("games:\n    1:\n        scanned_at: soon\n").is_err());
    }
}
