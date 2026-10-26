//! Games: the game cache, `<cache>/games.json`, and finding a game in it.
//!
//! The cache holds only what a scan finds, so it is rebuildable. The fields
//! are Go's (`internal/game/game.go`). Games are an array sorted by app id.
//!
//! | Module | Role |
//! |---|---|
//! | [`cache`] | Reading and writing `games.json` under `games.lock` |
//! | [`scan`] | Building the cache from the Steam libraries |

pub mod cache;
pub mod scan;

use std::fmt;
use std::path::PathBuf;

use jiff::Timestamp;
use serde::{Deserialize, Serialize};

/// The cache format version written in `games.json`.
pub const CACHE_VERSION: u32 = 1;

/// A DLL family, in catalogue order: dlss, dlssg, dlssd, xess, fsr. Lists
/// of DLLs sort in this order (Go's, `internal/game/game.go:10-16`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DllType {
    /// DLSS super resolution, `nvngx_dlss.dll`.
    Dlss,
    /// DLSS frame generation, `nvngx_dlssg.dll`.
    Dlssg,
    /// DLSS ray reconstruction, `nvngx_dlssd.dll`.
    Dlssd,
    /// Intel XeSS.
    Xess,
    /// AMD FSR.
    Fsr,
}

impl DllType {
    /// Every family, in catalogue order.
    pub const ALL: [DllType; 5] = [
        DllType::Dlss,
        DllType::Dlssg,
        DllType::Dlssd,
        DllType::Xess,
        DllType::Fsr,
    ];

    /// The word in files, flags and JSON, such as `dlssg`.
    pub fn as_str(self) -> &'static str {
        match self {
            DllType::Dlss => "dlss",
            DllType::Dlssg => "dlssg",
            DllType::Dlssd => "dlssd",
            DllType::Xess => "xess",
            DllType::Fsr => "fsr",
        }
    }

    /// The family with this word.
    pub fn from_name(name: &str) -> Option<DllType> {
        DllType::ALL.into_iter().find(|t| t.as_str() == name)
    }
}

impl fmt::Display for DllType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// One DLL a scan found in a game folder.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DetectedDll {
    /// The absolute path.
    pub path: PathBuf,
    /// The file name as found, such as `nvngx_dlss.dll`.
    pub name: String,
    /// The family.
    #[serde(rename = "type")]
    pub dll_type: DllType,
    /// The version as Go's formatter writes it, such as `310.4.0`, or
    /// `None` when the file has no readable version.
    #[serde(default)]
    pub version: Option<String>,
}

/// One installed game.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Game {
    /// The Steam AppID.
    pub app_id: u64,
    /// The name from the app manifest.
    pub name: String,
    /// The install folder, absolute.
    pub install_dir: PathBuf,
    /// The Steam library the game is in.
    pub library_path: PathBuf,
    /// The Proton prefix, `steamapps/compatdata/<appid>`, when it exists.
    #[serde(default)]
    pub prefix_path: Option<PathBuf>,
    /// When the scan read this game.
    pub scanned_at: Timestamp,
    /// The DLLs found, in catalogue order and then by path.
    #[serde(default)]
    pub dlls: Vec<DetectedDll>,
}

impl Game {
    /// Whether the game has a DLL of this family.
    pub fn has(&self, dll_type: DllType) -> bool {
        self.dlls.iter().any(|d| d.dll_type == dll_type)
    }
}

/// The game cache, `<cache>/games.json`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GameCache {
    /// The format version, [`CACHE_VERSION`].
    #[serde(default = "cache_version")]
    pub version: u32,
    /// When a scan last wrote the cache. `None` for a cache never written.
    #[serde(default)]
    pub updated_at: Option<Timestamp>,
    /// The games, sorted by app id.
    #[serde(default)]
    pub games: Vec<Game>,
}

fn cache_version() -> u32 {
    CACHE_VERSION
}

impl Default for GameCache {
    fn default() -> GameCache {
        GameCache {
            version: CACHE_VERSION,
            updated_at: None,
            games: Vec::new(),
        }
    }
}

/// No game matches a query. The text is Go's (`game not found: <q>`).
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("game not found: {0}")]
pub struct GameNotFound(pub String);

impl GameCache {
    /// Reads the JSON text of a cache.
    pub fn from_json(text: &str) -> Result<GameCache, serde_json::Error> {
        serde_json::from_str(text)
    }

    /// The JSON text of the cache, pretty printed, ending with a newline.
    /// Fails only on a path that is not UTF-8.
    pub fn to_json(&self) -> Result<String, serde_json::Error> {
        let mut text = serde_json::to_string_pretty(self)?;
        text.push('\n');
        Ok(text)
    }

    /// Whether the cache has no games, which makes the CLI rescan
    /// (decision 24).
    pub fn is_empty(&self) -> bool {
        self.games.is_empty()
    }

    /// The game with this AppID.
    pub fn get(&self, app_id: u64) -> Option<&Game> {
        self.games.iter().find(|g| g.app_id == app_id)
    }

    /// The game a command's `<game>` argument names: a numeric AppID in the
    /// cache first, then an exact, case-sensitive name. When two games share
    /// a name, the lowest AppID wins, where Go took map order. There is no
    /// fuzzy, prefix or case-insensitive match, and tools are not filtered.
    pub fn find(&self, query: &str) -> Result<&Game, GameNotFound> {
        if let Ok(app_id) = query.parse::<u64>()
            && let Some(game) = self.get(app_id)
        {
            return Ok(game);
        }
        self.games
            .iter()
            .filter(|g| g.name == query)
            .min_by_key(|g| g.app_id)
            .ok_or_else(|| GameNotFound(query.to_owned()))
    }

    /// Adds or replaces a game, keeping the list sorted by app id.
    pub fn upsert(&mut self, game: Game) {
        match self.games.binary_search_by_key(&game.app_id, |g| g.app_id) {
            Ok(i) => self.games[i] = game,
            Err(i) => self.games.insert(i, game),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn game(app_id: u64, name: &str) -> Game {
        Game {
            app_id,
            name: name.to_owned(),
            install_dir: PathBuf::from(format!("/lib/steamapps/common/{name}")),
            library_path: PathBuf::from("/lib"),
            prefix_path: None,
            scanned_at: "2026-10-26T15:02:10Z".parse().unwrap(),
            dlls: Vec::new(),
        }
    }

    #[test]
    fn find_takes_the_app_id_then_the_exact_name() {
        let mut cache = GameCache::default();
        cache.upsert(game(9100030, "Hollow Crown 3"));
        cache.upsert(game(9100010, "Chrome Avenue 2081"));
        cache.upsert(game(9100020, "Hollow Crown 3"));
        cache.upsert(game(42, "9100010"));
        let ids: Vec<u64> = cache.games.iter().map(|g| g.app_id).collect();
        assert_eq!(ids, [42, 9100010, 9100020, 9100030], "sorted by app id");
        assert_eq!(cache.find("9100010").unwrap().name, "Chrome Avenue 2081");
        assert_eq!(cache.find("Hollow Crown 3").unwrap().app_id, 9100020);
        assert_eq!(cache.find("42").unwrap().name, "9100010");
        assert_eq!(
            cache.find("hollow crown 3").unwrap_err().to_string(),
            "game not found: hollow crown 3"
        );
        assert!(cache.find("Hollow").is_err());
        // A number that is no AppID still matches a name.
        cache.upsert(game(7, "123"));
        assert_eq!(cache.find("123").unwrap().app_id, 7);
    }

    #[test]
    fn json_round_trips_in_the_architecture_s_shape() {
        let mut g = game(9100010, "Chrome Avenue 2081");
        g.prefix_path = Some("/lib/steamapps/compatdata/9100010".into());
        g.dlls.push(DetectedDll {
            path: "/lib/steamapps/common/Chrome Avenue 2081/nvngx_dlss.dll".into(),
            name: "nvngx_dlss.dll".into(),
            dll_type: DllType::Dlss,
            version: Some("310.4.0".into()),
        });
        let cache = GameCache {
            version: CACHE_VERSION,
            updated_at: Some("2026-10-26T17:02:11Z".parse().unwrap()),
            games: vec![g],
        };
        let text = cache.to_json().unwrap();
        assert!(text.contains("\"type\": \"dlss\""), "{text}");
        assert!(
            text.contains("\"scanned_at\": \"2026-10-26T15:02:10Z\""),
            "{text}"
        );
        assert!(text.ends_with("}\n"));
        assert_eq!(GameCache::from_json(&text).unwrap(), cache);
        assert!(cache.games[0].has(DllType::Dlss));
        assert!(!cache.games[0].has(DllType::Xess));
        // Optional keys may be missing.
        let bare = r#"{"games":[{"app_id":1,"name":"x","install_dir":"/a","library_path":"/b","scanned_at":"2026-10-26T15:02:10Z"}]}"#;
        let cache = GameCache::from_json(bare).unwrap();
        assert_eq!(cache.version, CACHE_VERSION);
        assert_eq!(cache.games[0].prefix_path, None);
    }

    #[test]
    fn dll_types_are_in_catalogue_order() {
        let mut types = vec![
            DllType::Fsr,
            DllType::Dlssd,
            DllType::Dlss,
            DllType::Xess,
            DllType::Dlssg,
        ];
        types.sort();
        assert_eq!(types, DllType::ALL);
        for t in DllType::ALL {
            assert_eq!(DllType::from_name(t.as_str()), Some(t));
        }
    }
}
