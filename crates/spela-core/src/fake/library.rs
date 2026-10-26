//! A fictional Steam library for `spela --demo` and the TUI's tests.
//!
//! [`write`] lays out app manifests and game folders under a temp dir, with
//! small DLL files where a game has managed DLLs, so a real scan of it
//! finds games, DLLs and Proton prefixes. The DLL files hold no PE version,
//! so their version reads as unknown. Every name and app id is fictional
//! (decision 43).

use std::io;
use std::path::{Path, PathBuf};

use crate::steam::library::{Library, SteamInstall};

/// One demo game: app id, name, install folder and the DLL files in it,
/// relative to the folder.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DemoGame {
    /// The Steam AppID, in the fictional 9100000 range.
    pub app_id: u64,
    /// The name in the app manifest.
    pub name: &'static str,
    /// The `installdir`.
    pub dir: &'static str,
    /// DLL files to create, relative to the install folder.
    pub dlls: &'static [&'static str],
    /// Whether the game has a Proton prefix.
    pub prefix: bool,
}

/// The demo library, by name.
pub const DEMO_GAMES: &[DemoGame] = &[
    DemoGame {
        app_id: 9100110,
        name: "Ashfall Courier",
        dir: "Ashfall Courier",
        dlls: &["bin/nvngx_dlss.dll", "bin/nvngx_dlssg.dll"],
        prefix: true,
    },
    DemoGame {
        app_id: 9100120,
        name: "Brightwater Keep",
        dir: "Brightwater Keep",
        dlls: &[],
        prefix: true,
    },
    DemoGame {
        app_id: 9100130,
        name: "Cinder Valley Racing",
        dir: "Cinder Valley Racing",
        dlls: &["nvngx_dlss.dll"],
        prefix: true,
    },
    DemoGame {
        app_id: 9100140,
        name: "Deepline Salvage",
        dir: "Deepline Salvage",
        dlls: &["Engine/nvngx_dlss.dll", "Engine/nvngx_dlssd.dll"],
        prefix: true,
    },
    DemoGame {
        app_id: 9100150,
        name: "Ember Tide",
        dir: "Ember Tide",
        dlls: &["libxess.dll"],
        prefix: true,
    },
    DemoGame {
        app_id: 9100160,
        name: "Glasshouse Detective",
        dir: "Glasshouse Detective",
        dlls: &[],
        prefix: false,
    },
    DemoGame {
        app_id: 9100170,
        name: "Hollow Crown Tactics: Definitive Edition",
        dir: "Hollow Crown Tactics",
        dlls: &["nvngx_dlss.dll"],
        prefix: true,
    },
    DemoGame {
        app_id: 9100180,
        name: "Ironleaf Expedition",
        dir: "Ironleaf Expedition",
        dlls: &["bin/x64/nvngx_dlss.dll", "bin/x64/amd_fidelityfx_vk.dll"],
        prefix: true,
    },
    DemoGame {
        app_id: 9100190,
        name: "Lantern Drift",
        dir: "Lantern Drift",
        dlls: &[],
        prefix: true,
    },
    DemoGame {
        app_id: 9100200,
        name: "Moonwell Farmstead",
        dir: "Moonwell Farmstead",
        dlls: &[],
        prefix: false,
    },
    DemoGame {
        app_id: 9100210,
        name: "Nightglass Protocol",
        dir: "Nightglass Protocol",
        dlls: &["nvngx_dlss.dll", "nvngx_dlssg.dll", "nvngx_dlssd.dll"],
        prefix: true,
    },
    DemoGame {
        app_id: 9100220,
        name: "Quarry of Stars",
        dir: "Quarry of Stars",
        dlls: &["nvngx_dlss.dll"],
        prefix: true,
    },
];

/// Writes the demo library under `root` and returns the Steam install that
/// lists it. `root` should be a fresh temp dir.
pub fn write(root: &Path) -> io::Result<SteamInstall> {
    let library = root.join("SteamLibrary");
    let steamapps = library.join("steamapps");
    std::fs::create_dir_all(steamapps.join("common"))?;
    for game in DEMO_GAMES {
        let manifest = format!(
            "\"AppState\"\n{{\n\t\"appid\"\t\t\"{}\"\n\t\"name\"\t\t\"{}\"\n\t\"StateFlags\"\t\t\"4\"\n\t\"installdir\"\t\t\"{}\"\n}}\n",
            game.app_id, game.name, game.dir
        );
        std::fs::write(
            steamapps.join(format!("appmanifest_{}.acf", game.app_id)),
            manifest,
        )?;
        let dir = steamapps.join("common").join(game.dir);
        std::fs::create_dir_all(&dir)?;
        for dll in game.dlls {
            let path = dir.join(dll);
            if let Some(parent) = path.parent() {
                std::fs::create_dir_all(parent)?;
            }
            std::fs::write(path, b"spela demo DLL, no PE version\n")?;
        }
        if game.prefix {
            std::fs::create_dir_all(
                steamapps
                    .join("compatdata")
                    .join(game.app_id.to_string())
                    .join("pfx/drive_c"),
            )?;
        }
    }
    Ok(install(&library))
}

/// The Steam install that lists only the demo library at `library`.
pub fn install(library: &Path) -> SteamInstall {
    SteamInstall {
        root: None,
        libraries: vec![Library {
            path: PathBuf::from(library),
            label: String::new(),
        }],
        problems: Vec::new(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::clock::ManualClock;
    use crate::games::scan;

    #[test]
    fn a_scan_finds_every_demo_game_and_its_dlls() {
        let dir = tempfile::tempdir().unwrap();
        let install = write(dir.path()).unwrap();
        let clock = ManualClock::new("2026-10-26T17:00:00Z".parse().unwrap());
        let found = scan::scan(&install, &clock);
        assert_eq!(found.games.len(), DEMO_GAMES.len());
        let nightglass = found.games.iter().find(|g| g.app_id == 9100210).unwrap();
        assert_eq!(nightglass.dlls.len(), 3);
        assert!(nightglass.prefix_path.is_some());
        let farm = found.games.iter().find(|g| g.app_id == 9100200).unwrap();
        assert!(farm.dlls.is_empty() && farm.prefix_path.is_none());
    }
}
