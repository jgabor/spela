//! DLL detection: which DLL families a game folder holds, by file name.
//!
//! This is Go's `ScanDirectory` (`internal/dll/detect.go`):
//!
//! - The whole folder is walked without following symlinks, the root
//!   included, as Go's `filepath.WalkDir` does.
//! - A file is a DLL when its lowercased name is in [`FILE_NAMES`]. The name
//!   is stored as found on disk, so `NVNGX_DLSS.DLL` keeps its case.
//! - Any walk error, even in a nested folder, ends the scan. The DLLs found
//!   before it come back with the error.
//! - A version that cannot be read is `None`, which shows as "unknown".
//!
//! The version itself comes from the PE reader in [`crate::pe`], passed in
//! by the caller, so this module never opens a DLL.
//!
//! The list comes back in catalogue order and then by path, the order of
//! [`crate::games::Game::dlls`]. Go kept the walk order.

use std::fmt;
use std::io;
use std::path::{Path, PathBuf};

use walkdir::WalkDir;

use crate::games::{DetectedDll, DllType};

/// A DLL family as the catalogue lists it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Family {
    /// The family. Its word is also the manifest key and the DLL cache
    /// folder name.
    pub dll_type: DllType,
    /// The file name an install creates when the game has none, Go's
    /// catalogue `Filename`. For FSR that is the Vulkan file only.
    pub filename: &'static str,
    /// The short label: DLSS, DLSS-G, DLSS-D, XeSS or FSR.
    pub label: &'static str,
}

/// The DLL families in catalogue order, which the TUI's deployment table
/// follows (Go's `KnownDLLTypes`).
pub static CATALOGUE: [Family; 5] = [
    Family {
        dll_type: DllType::Dlss,
        filename: "nvngx_dlss.dll",
        label: "DLSS",
    },
    Family {
        dll_type: DllType::Dlssg,
        filename: "nvngx_dlssg.dll",
        label: "DLSS-G",
    },
    Family {
        dll_type: DllType::Dlssd,
        filename: "nvngx_dlssd.dll",
        label: "DLSS-D",
    },
    Family {
        dll_type: DllType::Xess,
        filename: "libxess.dll",
        label: "XeSS",
    },
    Family {
        dll_type: DllType::Fsr,
        filename: "amd_fidelityfx_vk.dll",
        label: "FSR",
    },
];

/// Every file name detection knows, lowercase, with its family. FSR has two.
pub static FILE_NAMES: [(&str, DllType); 6] = [
    ("nvngx_dlss.dll", DllType::Dlss),
    ("nvngx_dlssg.dll", DllType::Dlssg),
    ("nvngx_dlssd.dll", DllType::Dlssd),
    ("libxess.dll", DllType::Xess),
    ("amd_fidelityfx_vk.dll", DllType::Fsr),
    ("amd_fidelityfx_dx12.dll", DllType::Fsr),
];

/// The catalogue entry of a family.
pub fn family(dll_type: DllType) -> &'static Family {
    // CATALOGUE lists every DllType in declaration order.
    &CATALOGUE[dll_type as usize]
}

/// The family of a DLL file name, ignoring case: `NVNGX_DLSS.DLL` is DLSS.
pub fn dll_type_of(file_name: &str) -> Option<DllType> {
    let lower = file_name.to_lowercase();
    FILE_NAMES
        .iter()
        .find(|(name, _)| *name == lower)
        .map(|(_, t)| *t)
}

/// Whether `file_name` is a name of `dll_type`, ignoring case. An install or
/// update only writes a manifest entry whose file name passes this.
pub fn is_file_name_of(dll_type: DllType, file_name: &str) -> bool {
    dll_type_of(file_name) == Some(dll_type)
}

/// A scan that stopped on an error.
#[derive(Debug)]
pub struct ScanError {
    /// Where the walk failed.
    pub path: PathBuf,
    /// Why.
    pub source: io::Error,
    /// The DLLs found before the error, in the same order as a full scan.
    pub found: Vec<DetectedDll>,
}

impl fmt::Display for ScanError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "cannot scan {}: {}", self.path.display(), self.source)
    }
}

impl std::error::Error for ScanError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        Some(&self.source)
    }
}

/// Finds the DLLs under `dir`. `read_version` gives a file's version as
/// Go's formatter writes it, or `None`; an empty version counts as `None`.
/// A missing `dir` is an error.
pub fn scan_directory(
    dir: &Path,
    mut read_version: impl FnMut(&Path) -> Option<String>,
) -> Result<Vec<DetectedDll>, ScanError> {
    let mut found = Vec::new();
    let walk = WalkDir::new(dir)
        .follow_links(false)
        .follow_root_links(false)
        .sort_by_file_name();
    for entry in walk {
        let entry = match entry {
            Ok(entry) => entry,
            Err(e) => {
                sort(&mut found);
                return Err(ScanError {
                    path: e.path().unwrap_or(dir).to_owned(),
                    source: e.into(),
                    found,
                });
            }
        };
        if entry.file_type().is_dir() {
            continue;
        }
        let Some(name) = entry.file_name().to_str().map(str::to_owned) else {
            continue;
        };
        if let Some(dll_type) = dll_type_of(&name) {
            let path = entry.into_path();
            let version = read_version(&path).filter(|v| !v.is_empty());
            found.push(DetectedDll {
                name,
                path,
                dll_type,
                version,
            });
        }
    }
    sort(&mut found);
    Ok(found)
}

fn sort(dlls: &mut [DetectedDll]) {
    dlls.sort_by(|a, b| {
        a.dll_type
            .cmp(&b.dll_type)
            .then_with(|| a.path.cmp(&b.path))
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    fn touch(path: &Path) {
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, b"stub").unwrap();
    }

    fn version_of_name(path: &Path) -> Option<String> {
        // A stand-in for the PE reader: the parent folder name.
        let parent = path.parent()?.file_name()?.to_str()?;
        parent.strip_prefix("v").map(str::to_owned)
    }

    #[test]
    fn catalogue_order_and_names_are_go_s() {
        let types: Vec<DllType> = CATALOGUE.iter().map(|f| f.dll_type).collect();
        assert_eq!(types, DllType::ALL);
        let labels: Vec<&str> = CATALOGUE.iter().map(|f| f.label).collect();
        assert_eq!(labels, ["DLSS", "DLSS-G", "DLSS-D", "XeSS", "FSR"]);
        assert_eq!(family(DllType::Fsr).filename, "amd_fidelityfx_vk.dll");
        for f in &CATALOGUE {
            assert_eq!(family(f.dll_type), f);
            assert!(is_file_name_of(f.dll_type, f.filename));
        }
    }

    #[test]
    fn names_match_without_case_and_fsr_has_two() {
        assert_eq!(dll_type_of("NVNGX_DLSS.DLL"), Some(DllType::Dlss));
        assert_eq!(dll_type_of("nvngx_dlssg.dll"), Some(DllType::Dlssg));
        assert_eq!(dll_type_of("Nvngx_Dlssd.dll"), Some(DllType::Dlssd));
        assert_eq!(dll_type_of("libxess.dll"), Some(DllType::Xess));
        assert_eq!(dll_type_of("amd_fidelityfx_vk.dll"), Some(DllType::Fsr));
        assert_eq!(dll_type_of("AMD_FIDELITYFX_DX12.dll"), Some(DllType::Fsr));
        // Not detected, as in Go: FSR 4 and XeSS frame generation files.
        for name in [
            "amd_fidelityfx_upscaler_dx12.dll",
            "libxess_fg.dll",
            "libxell.dll",
            "dlss.dll",
            "nvngx_dlss.dll.bak",
        ] {
            assert_eq!(dll_type_of(name), None, "{name}");
        }
        assert!(is_file_name_of(DllType::Fsr, "amd_fidelityfx_dx12.dll"));
        assert!(!is_file_name_of(DllType::Dlss, "nvngx_dlssg.dll"));
        assert!(!is_file_name_of(DllType::Dlss, "dlss.dll"));
    }

    #[test]
    fn scan_finds_every_family_keeps_case_and_sorts() {
        let dir = tempfile::tempdir().unwrap();
        let root = dir.path();
        touch(&root.join("plugins/v3.5/nvngx_dlss.dll"));
        touch(&root.join("bin/x64/v310.2.1/NVNGX_DLSS.DLL"));
        touch(&root.join("bin/x64/v310.2.1/nvngx_dlssg.dll"));
        touch(&root.join("libxess.dll"));
        touch(&root.join("fsr/amd_fidelityfx_dx12.dll"));
        touch(&root.join("fsr/amd_fidelityfx_vk.dll"));
        touch(&root.join("bin/other.dll"));
        touch(&root.join("nvngx_dlss.dll.bak"));
        // A folder with a DLL's name is not a DLL.
        std::fs::create_dir_all(root.join("nvngx_dlssd.dll")).unwrap();

        let found = scan_directory(root, version_of_name).unwrap();
        let got: Vec<(DllType, String, Option<&str>)> = found
            .iter()
            .map(|d| {
                let rel = d.path.strip_prefix(root).unwrap().display().to_string();
                (d.dll_type, rel, d.version.as_deref())
            })
            .collect();
        assert_eq!(
            got,
            [
                (
                    DllType::Dlss,
                    "bin/x64/v310.2.1/NVNGX_DLSS.DLL".into(),
                    Some("310.2.1")
                ),
                (
                    DllType::Dlss,
                    "plugins/v3.5/nvngx_dlss.dll".into(),
                    Some("3.5")
                ),
                (
                    DllType::Dlssg,
                    "bin/x64/v310.2.1/nvngx_dlssg.dll".into(),
                    Some("310.2.1")
                ),
                (DllType::Xess, "libxess.dll".into(), None),
                (DllType::Fsr, "fsr/amd_fidelityfx_dx12.dll".into(), None),
                (DllType::Fsr, "fsr/amd_fidelityfx_vk.dll".into(), None),
            ]
        );
        assert_eq!(found[0].name, "NVNGX_DLSS.DLL");
        assert!(found.iter().all(|d| d.path.is_absolute()));
    }

    #[test]
    fn an_empty_version_is_unknown() {
        let dir = tempfile::tempdir().unwrap();
        touch(&dir.path().join("nvngx_dlss.dll"));
        let found = scan_directory(dir.path(), |_| Some(String::new())).unwrap();
        assert_eq!(found[0].version, None);
    }

    #[test]
    fn symlinks_are_not_followed() {
        let dir = tempfile::tempdir().unwrap();
        let outside = dir.path().join("outside");
        touch(&outside.join("nvngx_dlss.dll"));
        let game = dir.path().join("game");
        touch(&game.join("libxess.dll"));
        std::os::unix::fs::symlink(&outside, game.join("linked")).unwrap();
        let found = scan_directory(&game, |_| None).unwrap();
        assert_eq!(found.len(), 1);
        assert_eq!(found[0].dll_type, DllType::Xess);

        // A root that is a symlink is not followed either, as Go's WalkDir.
        let link = dir.path().join("root-link");
        std::os::unix::fs::symlink(&outside, &link).unwrap();
        assert!(scan_directory(&link, |_| None).unwrap().is_empty());

        // A symlink to a file with a DLL's name counts, as in Go.
        std::os::unix::fs::symlink(outside.join("nvngx_dlss.dll"), game.join("nvngx_dlss.dll"))
            .unwrap();
        let found = scan_directory(&game, |_| None).unwrap();
        assert_eq!(found.len(), 2);
    }

    #[test]
    fn a_missing_folder_is_an_error() {
        let dir = tempfile::tempdir().unwrap();
        let missing = dir.path().join("missing");
        let err = scan_directory(&missing, |_| None).unwrap_err();
        assert_eq!(err.path, missing);
        assert_eq!(err.source.kind(), io::ErrorKind::NotFound);
        assert!(err.found.is_empty());
        assert!(err.to_string().starts_with("cannot scan "), "{err}");
    }

    #[test]
    fn a_nested_walk_error_ends_the_scan_with_what_was_found() {
        use std::os::unix::fs::PermissionsExt;
        let dir = tempfile::tempdir().unwrap();
        let root = dir.path();
        touch(&root.join("a/nvngx_dlss.dll"));
        touch(&root.join("b/nvngx_dlssg.dll"));
        touch(&root.join("c/libxess.dll"));
        let locked = root.join("b");
        std::fs::set_permissions(&locked, std::fs::Permissions::from_mode(0o000)).unwrap();
        let result = scan_directory(root, |_| None);
        std::fs::set_permissions(&locked, std::fs::Permissions::from_mode(0o755)).unwrap();
        // Root can read a 0000 folder; then the walk simply succeeds.
        let Err(err) = result else {
            return;
        };
        assert_eq!(err.path, locked);
        assert_eq!(err.source.kind(), io::ErrorKind::PermissionDenied);
        let found: Vec<DllType> = err.found.iter().map(|d| d.dll_type).collect();
        assert_eq!(found, [DllType::Dlss], "c is never reached");
    }
}
