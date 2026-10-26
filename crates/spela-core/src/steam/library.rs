//! Steam library folders: the Steam install, `libraryfolders.vdf` and the configured extra libraries.
//!
//! The Steam root is the configured `steam_path`, else the first folder
//! that exists of `~/.steam/steam`, `~/.local/share/Steam` and the Flatpak
//! path (Go's `FindSteamPath`). The root's `steamapps/libraryfolders.vdf`
//! lists the libraries. They come in file order, then the configured extra
//! paths with the label `additional`, without repeating a path.
//!
//! Two things differ from Go:
//!
//! - A broken `libraryfolders.vdf` is reported, and the extra paths are
//!   still scanned. Go aborted the whole scan.
//! - Libraries keep their order in the file. Go took them from a map, so
//!   their order was random.

use std::path::{Path, PathBuf};

use super::manifest::{COMPATDATA, steamapps_dir};
use super::vdf::{self, VdfFileError};

/// Where Steam may be installed, relative to the home folder, in the order
/// they are tried.
pub const STEAM_ROOT_CANDIDATES: [&str; 3] = [
    ".steam/steam",
    ".local/share/Steam",
    ".var/app/com.valvesoftware.Steam/.steam/steam",
];

/// The library list in the Steam root's `steamapps` folder.
pub const LIBRARY_FOLDERS: &str = "libraryfolders.vdf";

/// The label of a library that comes from `additional_library_paths`.
pub const ADDITIONAL_LABEL: &str = "additional";

/// One Steam library.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Library {
    /// The library folder, which holds `steamapps`.
    pub path: PathBuf,
    /// Steam's label, often empty, or `additional` for a configured path.
    pub label: String,
}

/// The first Steam install found under `home`.
pub fn find_steam_root(home: &Path) -> Option<PathBuf> {
    STEAM_ROOT_CANDIDATES
        .iter()
        .map(|candidate| home.join(candidate))
        .find(|path| path.is_dir())
}

/// The Steam root to use: `configured` when it is not empty, as Go took it,
/// else the first install found under `home`. `None` when there is no home
/// and nothing configured, or when no install is found.
pub fn resolve_steam_root(configured: &str, home: Option<&Path>) -> Option<PathBuf> {
    if !configured.is_empty() {
        return Some(PathBuf::from(configured));
    }
    home.and_then(find_steam_root)
}

/// The libraries `<root>/steamapps/libraryfolders.vdf` lists, in file
/// order. Entries with an empty path are skipped. A file with no
/// `libraryfolders` node lists no library.
pub fn read_library_folders(steam_root: &Path) -> Result<Vec<Library>, VdfFileError> {
    let root = vdf::read(&steamapps_dir(steam_root).join(LIBRARY_FOLDERS))?;
    let Some(folders) = root.node("libraryfolders") else {
        return Ok(Vec::new());
    };
    Ok(folders
        .children()
        .into_iter()
        .filter_map(|(_, item)| item.as_node())
        .filter_map(|node| {
            let path = node.text("path").unwrap_or_default();
            (!path.is_empty()).then(|| Library {
                path: PathBuf::from(path),
                label: node.text("label").unwrap_or_default().to_owned(),
            })
        })
        .collect())
}

/// A Steam install as found: its root, its libraries with the configured
/// extra ones added, and the problems met on the way.
#[derive(Debug, Default)]
pub struct SteamInstall {
    /// The Steam root, when one is configured or found.
    pub root: Option<PathBuf>,
    /// The libraries to scan, in order, each path once.
    pub libraries: Vec<Library>,
    /// What went wrong, such as a `libraryfolders.vdf` that does not parse.
    /// Each problem is also logged.
    pub problems: Vec<VdfFileError>,
}

impl SteamInstall {
    /// Finds Steam and its libraries. `steam_path` and `additional` are the
    /// config's `steam_path` and `additional_library_paths`; `home` is the
    /// user's home folder, from `HOME`. Nothing here fails: a problem is
    /// recorded and the rest is still found.
    pub fn discover(steam_path: &str, additional: &[String], home: Option<&Path>) -> SteamInstall {
        let root = resolve_steam_root(steam_path, home);
        let mut install = SteamInstall {
            root: root.clone(),
            ..SteamInstall::default()
        };
        if let Some(root) = &root {
            match read_library_folders(root) {
                Ok(libraries) => {
                    for library in libraries {
                        install.add(library);
                    }
                }
                Err(e) => {
                    tracing::warn!("cannot list the Steam libraries: {e}");
                    install.problems.push(e);
                }
            }
        }
        for path in additional.iter().filter(|p| !p.is_empty()) {
            install.add(Library {
                path: PathBuf::from(path),
                label: ADDITIONAL_LABEL.to_owned(),
            });
        }
        install
    }

    /// Adds a library unless one with the same path, compared as written,
    /// is already listed.
    fn add(&mut self, library: Library) {
        if !self.libraries.iter().any(|l| l.path == library.path) {
            self.libraries.push(library);
        }
    }

    /// Whether there is any library to scan. With none, there is no Steam
    /// installation to speak of.
    pub fn has_libraries(&self) -> bool {
        !self.libraries.is_empty()
    }

    /// The library folders, in order.
    pub fn library_paths(&self) -> impl Iterator<Item = &Path> {
        self.libraries.iter().map(|l| l.path.as_path())
    }
}

/// The Proton prefix of an app, `<library>/steamapps/compatdata/<appid>`,
/// when its `pfx/drive_c` is a folder (Go's `ScanProtonPrefix`).
pub fn proton_prefix(library: &Path, app_id: u64) -> Option<PathBuf> {
    let prefix = steamapps_dir(library)
        .join(COMPATDATA)
        .join(app_id.to_string());
    prefix
        .join("pfx")
        .join("drive_c")
        .is_dir()
        .then_some(prefix)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn mkdir(path: &Path) {
        std::fs::create_dir_all(path).unwrap();
    }

    #[test]
    fn the_first_candidate_that_exists_is_the_root() {
        let home = tempfile::tempdir().unwrap();
        let home = home.path();
        assert_eq!(find_steam_root(home), None);
        let flatpak = home.join(STEAM_ROOT_CANDIDATES[2]);
        mkdir(&flatpak);
        assert_eq!(find_steam_root(home), Some(flatpak));
        let local = home.join(".local/share/Steam");
        mkdir(&local);
        assert_eq!(find_steam_root(home), Some(local.clone()));
        // ~/.steam/steam is usually a link to ~/.local/share/Steam.
        mkdir(&home.join(".steam"));
        std::os::unix::fs::symlink(&local, home.join(".steam/steam")).unwrap();
        assert_eq!(find_steam_root(home), Some(home.join(".steam/steam")));
    }

    #[test]
    fn a_file_is_not_a_root_and_the_configured_path_wins() {
        let home = tempfile::tempdir().unwrap();
        let home = home.path();
        mkdir(&home.join(".steam"));
        std::fs::write(home.join(".steam/steam"), "not a folder").unwrap();
        mkdir(&home.join(".local/share/Steam"));
        assert_eq!(
            resolve_steam_root("", Some(home)),
            Some(home.join(".local/share/Steam"))
        );
        assert_eq!(
            resolve_steam_root("/opt/steam", Some(home)),
            Some(PathBuf::from("/opt/steam"))
        );
        assert_eq!(resolve_steam_root("", None), None);
    }

    fn write_folders(root: &Path, text: &str) {
        mkdir(&steamapps_dir(root));
        std::fs::write(steamapps_dir(root).join(LIBRARY_FOLDERS), text).unwrap();
    }

    #[test]
    fn libraries_come_in_file_order_then_the_extra_paths_once() {
        let dir = tempfile::tempdir().unwrap();
        let root = dir.path().join("steam");
        write_folders(
            &root,
            r#"
            "libraryfolders"
            {
                "contentstatsid" "1"
                "0" { "path" "/steam" "label" "" "apps" { "9100010" "1" } }
                "10" { "path" "/ten" }
                "2" { "path" "/two" "label" "Games" }
                "3" { "path" "" }
                "1" { "path" "/one" }
            }
            "#,
        );
        let extra = vec![
            "/two".to_owned(),
            String::new(),
            "/more".to_owned(),
            "/more".to_owned(),
        ];
        let install = SteamInstall::discover(root.to_str().unwrap(), &extra, None);
        assert!(install.problems.is_empty());
        let paths: Vec<&Path> = install.library_paths().collect();
        assert_eq!(
            paths,
            ["/steam", "/one", "/two", "/ten", "/more"].map(Path::new)
        );
        assert_eq!(install.libraries[2].label, "Games");
        assert_eq!(install.libraries[4].label, ADDITIONAL_LABEL);
    }

    #[test]
    fn a_broken_library_list_still_scans_the_extra_paths() {
        let dir = tempfile::tempdir().unwrap();
        let root = dir.path().join("steam");
        write_folders(&root, "\"libraryfolders\" {");
        let install = SteamInstall::discover(root.to_str().unwrap(), &["/more".to_owned()], None);
        assert_eq!(install.problems.len(), 1);
        assert!(
            install.problems[0].to_string().contains(LIBRARY_FOLDERS),
            "{}",
            install.problems[0]
        );
        assert_eq!(
            install.library_paths().collect::<Vec<_>>(),
            [Path::new("/more")]
        );

        // A missing list is a problem too, as Go's open error was.
        let none = SteamInstall::discover(dir.path().join("nowhere").to_str().unwrap(), &[], None);
        assert!(none.problems[0].is_not_found());
        assert!(!none.has_libraries());
    }

    #[test]
    fn no_steam_and_no_extra_paths_means_no_libraries() {
        let home = tempfile::tempdir().unwrap();
        let install = SteamInstall::discover("", &[], Some(home.path()));
        assert_eq!(install.root, None);
        assert!(!install.has_libraries());
        assert!(install.problems.is_empty());
    }

    #[test]
    fn a_prefix_needs_its_drive_c_folder() {
        let dir = tempfile::tempdir().unwrap();
        let lib = dir.path();
        assert_eq!(proton_prefix(lib, 9100010), None);
        let prefix = steamapps_dir(lib).join("compatdata/9100010");
        mkdir(&prefix.join("pfx"));
        assert_eq!(proton_prefix(lib, 9100010), None);
        mkdir(&prefix.join("pfx/drive_c"));
        assert_eq!(proton_prefix(lib, 9100010), Some(prefix));
    }
}
