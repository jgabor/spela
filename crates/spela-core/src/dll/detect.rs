//! DLL detection: which DLL families a game folder holds, by file name.
//!
//! A scan walks the whole install folder without following links, as Go's
//! `filepath.WalkDir` does, and matches each file name without regard to
//! case (`internal/dll/detect.go`). The stored name keeps the case on disk.
//! Each match gets its version from [`crate::pe`].
//!
//! Two things differ from Go. A folder that cannot be read is skipped and
//! logged, where Go stopped the walk there. And the DLLs come sorted by
//! family in catalogue order, then by path, where Go kept walk order.

use std::path::Path;

use walkdir::WalkDir;

use crate::games::{DetectedDll, DllType};
use crate::pe;

/// Every file name a scan recognises, lowercase, with its family.
/// `amd_fidelityfx_dx12.dll` is detected as FSR too, although the DLL
/// catalogue swaps only `amd_fidelityfx_vk.dll`.
pub const KNOWN_FILES: [(&str, DllType); 6] = [
    ("nvngx_dlss.dll", DllType::Dlss),
    ("nvngx_dlssg.dll", DllType::Dlssg),
    ("nvngx_dlssd.dll", DllType::Dlssd),
    ("libxess.dll", DllType::Xess),
    ("amd_fidelityfx_vk.dll", DllType::Fsr),
    ("amd_fidelityfx_dx12.dll", DllType::Fsr),
];

/// The family a file name belongs to, matched without regard to case.
pub fn dll_type_of(file_name: &str) -> Option<DllType> {
    KNOWN_FILES
        .iter()
        .find(|(name, _)| name.eq_ignore_ascii_case(file_name))
        .map(|&(_, dll_type)| dll_type)
}

/// The known DLLs under `dir`, with their versions, sorted by family and
/// then by path. A `dir` that cannot be read gives an empty list, which
/// is logged.
pub fn scan_dir(dir: &Path) -> Vec<DetectedDll> {
    let mut found = Vec::new();
    for entry in WalkDir::new(dir).sort_by_file_name() {
        let entry = match entry {
            Ok(entry) => entry,
            Err(e) => {
                tracing::warn!("skipping part of {} in the DLL scan: {e}", dir.display());
                continue;
            }
        };
        if entry.file_type().is_dir() {
            continue;
        }
        let Some(name) = entry.file_name().to_str() else {
            continue;
        };
        if let Some(dll_type) = dll_type_of(name) {
            found.push(DetectedDll {
                version: pe::version_string(entry.path()),
                path: entry.path().to_owned(),
                name: name.to_owned(),
                dll_type,
            });
        }
    }
    found.sort_by(|a, b| (a.dll_type, &a.path).cmp(&(b.dll_type, &b.path)));
    found
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn file_names_match_without_case() {
        assert_eq!(dll_type_of("nvngx_dlss.dll"), Some(DllType::Dlss));
        assert_eq!(dll_type_of("NVNGX_DLSS.DLL"), Some(DllType::Dlss));
        assert_eq!(dll_type_of("nvngx_dlssg.dll"), Some(DllType::Dlssg));
        assert_eq!(dll_type_of("nvngx_dlssd.dll"), Some(DllType::Dlssd));
        assert_eq!(dll_type_of("LibXeSS.dll"), Some(DllType::Xess));
        assert_eq!(dll_type_of("amd_fidelityfx_dx12.dll"), Some(DllType::Fsr));
        assert_eq!(dll_type_of("amd_fidelityfx_upscaler_dx12.dll"), None);
        assert_eq!(dll_type_of("nvngx_dlss.dll.bak"), None);
    }

    #[test]
    fn finds_nested_files_sorted_by_family_then_path_and_skips_folders() {
        let dir = tempfile::tempdir().unwrap();
        let root = dir.path();
        std::fs::create_dir_all(root.join("bin/x64")).unwrap();
        std::fs::create_dir_all(root.join("nvngx_dlss.dll")).unwrap();
        for name in [
            "bin/x64/nvngx_dlssg.dll",
            "bin/x64/nvngx_dlss.dll",
            "bin/x64/NVNGX_DLSSD.DLL",
            "a/amd_fidelityfx_vk.dll",
            "readme.txt",
        ] {
            let path = root.join(name);
            std::fs::create_dir_all(path.parent().unwrap()).unwrap();
            std::fs::write(path, b"not a PE file").unwrap();
        }
        let found = scan_dir(root);
        let names: Vec<(&str, DllType)> = found
            .iter()
            .map(|d| (d.name.as_str(), d.dll_type))
            .collect();
        assert_eq!(
            names,
            [
                ("nvngx_dlss.dll", DllType::Dlss),
                ("nvngx_dlssg.dll", DllType::Dlssg),
                ("NVNGX_DLSSD.DLL", DllType::Dlssd),
                ("amd_fidelityfx_vk.dll", DllType::Fsr),
            ]
        );
        assert!(
            found.iter().all(|d| d.version.is_none()),
            "stubs have no version"
        );
        assert_eq!(found[0].path, root.join("bin/x64/nvngx_dlss.dll"));
    }

    #[test]
    fn a_missing_folder_gives_nothing() {
        let dir = tempfile::tempdir().unwrap();
        assert!(scan_dir(&dir.path().join("gone")).is_empty());
    }

    #[test]
    fn links_are_not_followed() {
        let dir = tempfile::tempdir().unwrap();
        let outside = dir.path().join("outside");
        std::fs::create_dir_all(&outside).unwrap();
        std::fs::write(outside.join("libxess.dll"), b"x").unwrap();
        let game = dir.path().join("game");
        std::fs::create_dir_all(&game).unwrap();
        std::os::unix::fs::symlink(&outside, game.join("linked")).unwrap();
        assert!(scan_dir(&game).is_empty());
    }
}
