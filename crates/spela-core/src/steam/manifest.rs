//! App manifests: `appmanifest_<appid>.acf`, the installed games and their state flags.
//!
//! Each library's `steamapps` folder holds one manifest per app. Its
//! `AppState` node gives the AppID, name, install folder and state flags
//! (Go's `internal/steam/appmanifest.go`). StateFlags is a bit field, and an
//! app counts as installed when the fully installed bit (4) is set. Go
//! required StateFlags to equal 4, which dropped every game with a pending
//! update (StateFlags 6) from the library.

use std::io;
use std::path::{Component, Path, PathBuf};

use super::vdf::{self, VdfFileError};

/// The fully installed bit in StateFlags.
pub const STATE_FULLY_INSTALLED: u32 = 4;

/// The folder under a library that holds manifests, `common` and
/// `compatdata`.
pub const STEAMAPPS: &str = "steamapps";

/// The folder under `steamapps` that holds install folders.
pub const COMMON: &str = "common";

/// The folder under `steamapps` that holds Proton prefixes.
pub const COMPATDATA: &str = "compatdata";

/// The app manifest file name prefix.
const MANIFEST_PREFIX: &str = "appmanifest_";

/// The app manifest file name suffix.
const MANIFEST_SUFFIX: &str = ".acf";

/// What an app manifest says about one app.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AppManifest {
    /// The AppID, never 0.
    pub app_id: u64,
    /// The name, such as `Glimmer Run`.
    pub name: String,
    /// The install folder's name under `steamapps/common`.
    pub install_dir: String,
    /// Steam's state bits. 0 when missing or not a number.
    pub state_flags: u32,
    /// When Steam last updated the app, in Unix seconds. 0 when missing.
    pub last_updated: i64,
    /// The size on disk in bytes. 0 when missing.
    pub size_on_disk: u64,
    /// The library the manifest is in: the folder above `steamapps`.
    pub library_path: PathBuf,
}

impl AppManifest {
    /// Whether Steam says the app is fully installed (bit 4 set). An
    /// update may still be pending.
    pub fn is_installed(&self) -> bool {
        self.state_flags & STATE_FULLY_INSTALLED != 0
    }

    /// The install folder, `<library>/steamapps/common/<installdir>`, or
    /// `None` when `installdir` is not one plain folder name. An empty,
    /// absolute or `..` value would point outside `common`, so such a
    /// manifest has no install folder.
    pub fn full_install_dir(&self) -> Option<PathBuf> {
        let mut components = Path::new(&self.install_dir).components();
        match (components.next(), components.next()) {
            (Some(Component::Normal(name)), None) => {
                Some(common_dir(&self.library_path).join(name))
            }
            _ => None,
        }
    }
}

/// `<library>/steamapps`.
pub fn steamapps_dir(library: &Path) -> PathBuf {
    library.join(STEAMAPPS)
}

/// `<library>/steamapps/common`.
pub fn common_dir(library: &Path) -> PathBuf {
    steamapps_dir(library).join(COMMON)
}

/// The manifest of one app in a library.
pub fn manifest_path(library: &Path, app_id: u64) -> PathBuf {
    steamapps_dir(library).join(format!("{MANIFEST_PREFIX}{app_id}{MANIFEST_SUFFIX}"))
}

/// The app manifests in a library, sorted by file name. A missing
/// `steamapps` folder is an error.
pub fn list_manifests(library: &Path) -> io::Result<Vec<PathBuf>> {
    let mut found: Vec<PathBuf> = std::fs::read_dir(steamapps_dir(library))?
        .filter_map(Result::ok)
        .filter(|entry| {
            entry.file_name().to_str().is_some_and(|name| {
                name.len() > MANIFEST_PREFIX.len() + MANIFEST_SUFFIX.len()
                    && name.starts_with(MANIFEST_PREFIX)
                    && name.ends_with(MANIFEST_SUFFIX)
            })
        })
        .map(|entry| entry.path())
        .collect();
    found.sort();
    Ok(found)
}

/// Reads the manifest text of an app in `library`. `None` when it has no
/// `AppState` node or its AppID is 0 or not a number, as in Go.
pub fn parse(text: &str, library: &Path) -> Result<Option<AppManifest>, vdf::ParseError> {
    let root = vdf::parse(text)?;
    Ok(from_root(&root, library))
}

/// Reads an app manifest file. The library is the folder two levels up,
/// as in Go.
pub fn read(path: &Path) -> Result<Option<AppManifest>, VdfFileError> {
    let root = vdf::read(path)?;
    let library = path
        .parent()
        .and_then(Path::parent)
        .unwrap_or(Path::new(""));
    Ok(from_root(&root, library))
}

fn from_root(root: &vdf::Node, library: &Path) -> Option<AppManifest> {
    let state = root.node("AppState")?;
    let text = |key: &str| state.text(key).unwrap_or_default();
    let app_id = text("appid")
        .trim()
        .parse::<u64>()
        .ok()
        .filter(|&id| id != 0)?;
    Some(AppManifest {
        app_id,
        name: text("name").to_owned(),
        install_dir: text("installdir").to_owned(),
        state_flags: text("StateFlags").trim().parse().unwrap_or(0),
        last_updated: text("LastUpdated").trim().parse().unwrap_or(0),
        size_on_disk: text("SizeOnDisk").trim().parse().unwrap_or(0),
        library_path: library.to_owned(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn manifest(body: &str) -> String {
        format!("\"AppState\"\n{{\n{body}\n}}\n")
    }

    #[test]
    fn reads_the_fields_go_reads() {
        let text = manifest(
            "  \"appid\" \"9100010\"\n  \"name\" \"Glimmer Run\"\n  \"StateFlags\" \"4\"\n  \"installdir\" \"Glimmer Run\"\n  \"LastUpdated\" \"1761490930\"\n  \"SizeOnDisk\" \"52428800\"",
        );
        let m = parse(&text, Path::new("/lib")).unwrap().unwrap();
        assert_eq!(m.app_id, 9100010);
        assert_eq!(m.name, "Glimmer Run");
        assert_eq!(m.state_flags, 4);
        assert_eq!(m.last_updated, 1761490930);
        assert_eq!(m.size_on_disk, 52428800);
        assert!(m.is_installed());
        assert_eq!(
            m.full_install_dir().unwrap(),
            Path::new("/lib/steamapps/common/Glimmer Run")
        );
    }

    #[test]
    fn the_installed_bit_decides_not_equality() {
        let flags = |f: &str| {
            let text = manifest(&format!(
                "  \"appid\" \"1\"\n  \"StateFlags\" \"{f}\"\n  \"installdir\" \"x\""
            ));
            parse(&text, Path::new("/lib"))
                .unwrap()
                .unwrap()
                .is_installed()
        };
        assert!(flags("4"));
        assert!(flags("6"), "installed with an update pending");
        assert!(flags("1030"));
        assert!(!flags("2"));
        assert!(!flags("0"));
        assert!(!flags("many"));
    }

    #[test]
    fn a_zero_or_bad_app_id_or_no_app_state_is_none() {
        let lib = Path::new("/lib");
        assert_eq!(parse(&manifest("  \"appid\" \"0\""), lib).unwrap(), None);
        assert_eq!(parse(&manifest("  \"appid\" \"abc\""), lib).unwrap(), None);
        assert_eq!(parse(&manifest("  \"name\" \"x\""), lib).unwrap(), None);
        assert_eq!(parse("\"Other\" { \"appid\" \"5\" }", lib).unwrap(), None);
        // Go's tests used one-line braces, which its parser never read.
        let inline = parse("\"AppState\" { \"appid\" \"5\" }", lib).unwrap();
        assert_eq!(inline.unwrap().app_id, 5);
        assert!(parse("\"AppState\" {", lib).is_err());
    }

    #[test]
    fn an_install_dir_must_be_one_folder_name() {
        let with = |dir: &str| AppManifest {
            app_id: 1,
            name: String::new(),
            install_dir: dir.to_owned(),
            state_flags: 4,
            last_updated: 0,
            size_on_disk: 0,
            library_path: "/lib".into(),
        };
        assert!(with("Game").full_install_dir().is_some());
        for bad in ["", "/etc", "..", "a/b", "."] {
            assert_eq!(with(bad).full_install_dir(), None, "{bad:?}");
        }
    }

    #[test]
    fn lists_and_reads_manifests_in_a_library() {
        let dir = tempfile::tempdir().unwrap();
        let lib = dir.path();
        assert!(list_manifests(lib).is_err(), "no steamapps folder");
        std::fs::create_dir_all(steamapps_dir(lib)).unwrap();
        for name in [
            "appmanifest_20.acf",
            "appmanifest_10.acf",
            "appmanifest_.acf",
            "libraryfolders.vdf",
            "appmanifest_30.acf.tmp",
        ] {
            std::fs::write(
                steamapps_dir(lib).join(name),
                manifest("  \"appid\" \"10\""),
            )
            .unwrap();
        }
        let found = list_manifests(lib).unwrap();
        assert_eq!(found, [manifest_path(lib, 10), manifest_path(lib, 20)]);
        let m = read(&found[0]).unwrap().unwrap();
        assert_eq!(m.library_path, lib);
    }
}
