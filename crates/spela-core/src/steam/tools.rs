//! Steam tools: Proton builds, runtimes and redistributables, which a scan
//! drops so they never show as games.
//!
//! An app is a tool when its AppID is a known tool, when its install folder
//! holds a `toolmanifest.vdf`, or when its name matches a tool pattern
//! (Go's `internal/steam/filter.go` and `internal/game/database.go`). The
//! name check also runs when a list is shown, as a second guard for games
//! that an older scan kept.

use std::path::Path;
use std::sync::LazyLock;

use regex_lite::Regex;

/// The file a Steam tool keeps in its install folder.
pub const TOOL_MANIFEST: &str = "toolmanifest.vdf";

/// AppIDs of Steam tools, the 32 entries of Go's list (`filter.go`).
pub const KNOWN_TOOL_APP_IDS: [u64; 32] = [
    // Proton builds.
    1493710, // Proton Experimental
    2805730, // Proton 9.0
    2348590, // Proton 8.0
    2180100, // Proton Hotfix
    1887720, // Proton 7.0
    1580130, // Proton 6.3
    1245040, // Proton 5.0
    858280,  // Proton 4.2
    1054830, // Proton 4.11
    961940,  // Proton 3.16
    930400,  // Proton 3.7
    // Steam Linux Runtimes.
    1628350, // Steam Linux Runtime - Sniper
    1070560, // Steam Linux Runtime - Soldier
    1391110, // Steam Linux Runtime - Scout
    // Steamworks and SDK tools.
    228980, // Steamworks Common Redistributables
    // Steam platform tools.
    250820,  // SteamVR
    1007,    // Steam Client
    1158310, // Steam Streaming Speakers
    1260320, // Steam Input Configurator
    1675200, // Steam Linux Runtime - Medic
    1826330, // Steam Proton EasyAntiCheat Runtime
    2394010, // Steam for Chrome OS
    2180060, // Steam Play None
    1161040, // Steam Linux Media Server
    1245020, // Proton BattlEye Runtime
    1826340, // Steam Runtime EasyAntiCheat
    3244930, // Steam Linux Runtime - Heavy
    3305210, // Steam Linux Runtime - Beryllium
    3283470, // Steam Linux Runtime 2.0
    3283410, // Steam Linux Runtime 3.0
    3316210, // Proton 10.0
    3026190, // Proton Next
];

/// Go's tool name patterns, matched without regard to case.
const TOOL_NAME_PATTERNS: [&str; 5] = [
    r"(?i)^proton(\s|$)",
    r"(?i)^steam\s+linux\s+runtime",
    r"(?i)^steamworks",
    r"(?i)redistributable",
    r"(?i)^steam\s+controller",
];

static TOOL_NAMES: LazyLock<Vec<Regex>> = LazyLock::new(|| {
    TOOL_NAME_PATTERNS
        .iter()
        .map(|p| Regex::new(p).expect("the tool name patterns are valid"))
        .collect()
});

/// Whether `app_id` is a known Steam tool.
pub fn is_known_tool_app_id(app_id: u64) -> bool {
    KNOWN_TOOL_APP_IDS.contains(&app_id)
}

/// Whether a name looks like a Steam tool, such as `Proton 9.0` or
/// `Steam Linux Runtime 3.0 (sniper)`. Surrounding space is ignored.
pub fn is_tool_name(name: &str) -> bool {
    let name = name.trim();
    TOOL_NAMES.iter().any(|re| re.is_match(name))
}

/// Whether an install folder holds a `toolmanifest.vdf` file.
pub fn has_tool_manifest(install_dir: &Path) -> bool {
    install_dir.join(TOOL_MANIFEST).is_file()
}

/// Whether an installed app is a Steam tool rather than a game.
pub fn is_tool(app_id: u64, name: &str, install_dir: &Path) -> bool {
    is_known_tool_app_id(app_id) || has_tool_manifest(install_dir) || is_tool_name(name)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn tool_names_match_go_s_cases() {
        let cases = [
            ("Proton Experimental", true),
            ("Proton 9.0", true),
            ("Proton", true),
            ("  proton 8.0 ", true),
            ("Steam Linux Runtime - Sniper", true),
            ("Steam Linux Runtime 3.0 (sniper)", true),
            ("Steamworks Common Redistributables", true),
            ("Some Redistributable", true),
            ("Steam Controller Driver", true),
            ("Chrome Avenue 2081", false),
            ("Protonic Drift", false),
            ("Glimmer Run", false),
            ("", false),
        ];
        for (name, want) in cases {
            assert_eq!(is_tool_name(name), want, "{name:?}");
        }
    }

    #[test]
    fn the_known_ids_are_go_s_list() {
        let mut ids = KNOWN_TOOL_APP_IDS.to_vec();
        ids.sort_unstable();
        ids.dedup();
        assert_eq!(ids.len(), 32, "no duplicates");
        assert!(is_known_tool_app_id(1493710));
        assert!(is_known_tool_app_id(228980));
        assert!(!is_known_tool_app_id(9100010));
    }

    #[test]
    fn a_tool_manifest_or_id_or_name_makes_a_tool() {
        let dir = tempfile::tempdir().unwrap();
        let game = dir.path().join("Glimmer Run");
        let tool = dir.path().join("Some Runtime");
        std::fs::create_dir_all(&game).unwrap();
        std::fs::create_dir_all(tool.join(TOOL_MANIFEST)).unwrap();
        assert!(!is_tool(9100010, "Glimmer Run", &game));
        assert!(
            !is_tool(9100011, "Some Runtime", &tool),
            "a folder named like the file does not count"
        );
        std::fs::remove_dir(tool.join(TOOL_MANIFEST)).unwrap();
        std::fs::write(tool.join(TOOL_MANIFEST), "\"manifest\" {}").unwrap();
        assert!(is_tool(9100011, "Some Runtime", &tool));
        assert!(is_tool(1628350, "Glimmer Run", &game));
        assert!(is_tool(9100012, "Proton Fixture 9.0", &game));
    }
}
