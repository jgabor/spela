//! Adding a DLL release to `data/manifest.json`.
//!
//! Both DLL workflows used to edit the manifest with inline Python that put
//! the new entry first. Go clients take the first entry of a family as the
//! latest, so an older release added later became their latest, and the
//! manual workflow wrote `<type>.dll` as the file name. Here:
//!
//! - the entry's file name is its family's real name, such as
//!   `nvngx_dlss.dll`, and its URL must end in that name;
//! - the SHA-256 and size are read from the DLL itself;
//! - an entry with the same version is replaced;
//! - the new entry goes before the first entry with a lower version, so the
//!   list stays newest first and the first entry stays the highest for Go
//!   clients, while spela 0.7 picks the highest anyway.
//!
//! The manifest keeps Go's schema version 1 and its URL (decision 14). It is
//! read and written with `spela_core`'s types, so the family keys come out
//! in name order and an empty `notes` is left out.

use std::path::{Path, PathBuf};

use jiff::Timestamp;
use spela_core::dll::detect::family;
use spela_core::dll::manifest::{Manifest, ManifestEntry};
use spela_core::dll::version;
use spela_core::fs::{Durability, write_atomic};
use spela_core::games::DllType;
use spela_core::hex::sha256_reader;

/// One release to add.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Release {
    /// The family.
    pub dll_type: DllType,
    /// The version, 1 to 4 numbers joined by dots.
    pub version: String,
    /// The DLL on disk.
    pub file: PathBuf,
    /// Where clients download it. https, ending in the family's file name.
    pub url: String,
    /// A note such as `From techpowerup.com`. Empty leaves it out.
    pub notes: String,
}

/// What [`add`] did.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Added {
    /// Where the entry went in its family's list. 0 is the latest.
    pub position: usize,
    /// Whether an entry with the same version was replaced.
    pub replaced: bool,
    /// The highest version of the family after the change.
    pub latest: String,
}

/// Why a release could not be added.
#[derive(Debug, thiserror::Error)]
pub enum AddError {
    /// The version could not name a manifest entry.
    #[error("version {0:?} is not 1 to 4 numbers joined by dots")]
    BadVersion(String),
    /// The URL is not https or does not end in the file name.
    #[error("the URL {url:?} must be https and end in /{filename}")]
    BadUrl {
        /// The URL as given.
        url: String,
        /// The family's file name.
        filename: &'static str,
    },
    /// The DLL cannot be read.
    #[error("cannot read the DLL {}: {source}", path.display())]
    ReadDll {
        /// The DLL.
        path: PathBuf,
        /// The cause.
        source: std::io::Error,
    },
    /// The manifest cannot be read.
    #[error("cannot read the manifest {}: {source}", path.display())]
    ReadManifest {
        /// The manifest.
        path: PathBuf,
        /// The cause.
        source: std::io::Error,
    },
    /// The manifest does not parse.
    #[error("the manifest {} does not parse: {source}", path.display())]
    Parse {
        /// The manifest.
        path: PathBuf,
        /// The cause.
        source: serde_json::Error,
    },
    /// The manifest cannot be written.
    #[error("cannot write the manifest {}: {source}", path.display())]
    Write {
        /// The manifest.
        path: PathBuf,
        /// The cause.
        source: std::io::Error,
    },
}

/// Adds `release` to the manifest at `path`, stamped with `now`.
pub fn add(path: &Path, release: &Release, now: Timestamp) -> Result<Added, AddError> {
    if !version::is_valid(&release.version) {
        return Err(AddError::BadVersion(release.version.clone()));
    }
    let filename = family(release.dll_type).filename;
    if !release.url.starts_with("https://") || !release.url.ends_with(&format!("/{filename}")) {
        return Err(AddError::BadUrl {
            url: release.url.clone(),
            filename,
        });
    }
    let read_dll = |source| AddError::ReadDll {
        path: release.file.clone(),
        source,
    };
    let dll = std::fs::File::open(&release.file).map_err(read_dll)?;
    let size = dll.metadata().map_err(read_dll)?.len();
    let sha256 = sha256_reader(dll).map_err(read_dll)?;

    let text = std::fs::read_to_string(path).map_err(|source| AddError::ReadManifest {
        path: path.to_owned(),
        source,
    })?;
    let mut manifest = Manifest::from_json(&text).map_err(|source| AddError::Parse {
        path: path.to_owned(),
        source,
    })?;

    // Whole seconds, as the Python step wrote.
    let now = Timestamp::from_second(now.as_second()).expect("a second of a valid time");
    let entry = ManifestEntry {
        version: release.version.clone(),
        filename: filename.to_owned(),
        url: release.url.clone(),
        sha256,
        size: i64::try_from(size).unwrap_or(i64::MAX),
        release_date: Some(now),
        notes: release.notes.clone(),
    };
    let list = manifest
        .dlls
        .entry(release.dll_type.as_str().to_owned())
        .or_default();
    let before = list.len();
    list.retain(|e| version::compare(&e.version, &release.version).is_ne());
    let replaced = list.len() != before;
    let position = list
        .iter()
        .position(|e| version::compare(&e.version, &release.version).is_lt())
        .unwrap_or(list.len());
    list.insert(position, entry);
    manifest.updated_at = Some(now);
    let latest = manifest
        .latest(release.dll_type)
        .map(|e| e.version.clone())
        .unwrap_or_default();

    let mut json = serde_json::to_string_pretty(&manifest).expect("a manifest serializes");
    json.push('\n');
    write_atomic(path, json.as_bytes(), Durability::Durable).map_err(|source| AddError::Write {
        path: path.to_owned(),
        source,
    })?;
    Ok(Added {
        position,
        replaced,
        latest,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    const NOW: &str = "2026-10-26T12:34:56.789Z";

    struct Fixture {
        _dir: tempfile::TempDir,
        manifest: PathBuf,
        dll: PathBuf,
    }

    fn fixture(manifest: &str) -> Fixture {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("manifest.json");
        std::fs::write(&path, manifest).unwrap();
        let dll = dir.path().join("downloaded.dll");
        std::fs::write(&dll, b"MZ fixture dll").unwrap();
        Fixture {
            manifest: path,
            dll,
            _dir: dir,
        }
    }

    fn release(f: &Fixture, dll_type: DllType, version: &str) -> Release {
        let filename = family(dll_type).filename;
        Release {
            dll_type,
            version: version.into(),
            file: f.dll.clone(),
            url: format!(
                "https://github.com/jgabor/spela/releases/download/dll-{dll_type}-v{version}/{filename}"
            ),
            notes: "From techpowerup.com".into(),
        }
    }

    fn versions(path: &Path, dll_type: DllType) -> Vec<String> {
        let m = Manifest::from_json(&std::fs::read_to_string(path).unwrap()).unwrap();
        m.entries(dll_type)
            .iter()
            .map(|e| e.version.clone())
            .collect()
    }

    const TWO: &str = r#"{"version":"1","updated_at":"2026-02-03T06:24:32Z","repository":"jgabor/spela-dlls",
        "dlls":{"dlss":[{"version":"310.5.3","filename":"nvngx_dlss.dll","url":"https://x/nvngx_dlss.dll",
        "sha256":"8707e53b26c68c606b98bf31c223485ff30d310a261b1a36d48b2eaabc1507ec","size":1,
        "release_date":"2026-02-03T06:24:32Z","notes":"From techpowerup.com"},
        {"version":"310.5.0","filename":"nvngx_dlss.dll","url":"https://y/nvngx_dlss.dll",
        "sha256":"22c4660249da5b352cde0ce4d95178c3761223dd6ac096d0b88e5c03d7a5cdd1","size":2}],
        "xess":[],"fsr":[]}}"#;

    #[test]
    fn a_newer_release_goes_first_with_the_dll_s_hash_and_size() {
        let f = fixture(TWO);
        let added = add(
            &f.manifest,
            &release(&f, DllType::Dlss, "310.6.0"),
            NOW.parse().unwrap(),
        )
        .unwrap();
        assert_eq!(
            added,
            Added {
                position: 0,
                replaced: false,
                latest: "310.6.0".into()
            }
        );
        assert_eq!(
            versions(&f.manifest, DllType::Dlss),
            ["310.6.0", "310.5.3", "310.5.0"]
        );
        let text = std::fs::read_to_string(&f.manifest).unwrap();
        let m = Manifest::from_json(&text).unwrap();
        let new = &m.entries(DllType::Dlss)[0];
        assert_eq!(new.filename, "nvngx_dlss.dll");
        assert_eq!(new.sha256, spela_core::hex::sha256_hex(b"MZ fixture dll"));
        assert_eq!(new.size, 14);
        assert_eq!(new.notes, "From techpowerup.com");
        assert_eq!(
            new.release_date,
            Some("2026-10-26T12:34:56Z".parse().unwrap())
        );
        assert_eq!(m.updated_at, new.release_date, "whole seconds");
        assert_eq!(m.repository, "jgabor/spela-dlls", "kept");
        // Python's json.dump(indent=2) layout, plus a final newline.
        assert!(
            text.starts_with("{\n  \"version\": \"1\",\n  \"updated_at\": \"2026-10-26T12:34:56Z\",\n  \"repository\": \"jgabor/spela-dlls\",\n  \"dlls\": {\n    \"dlss\": [\n      {\n        \"version\": \"310.6.0\",\n        \"filename\": \"nvngx_dlss.dll\",\n"),
            "{text}"
        );
        assert!(
            text.contains("    \"fsr\": [],\n    \"xess\": []\n"),
            "{text}"
        );
        assert!(text.ends_with("}\n"));
        // The old entries are as they were.
        assert_eq!(
            m.entries(DllType::Dlss)[2].sha256,
            "22c4660249da5b352cde0ce4d95178c3761223dd6ac096d0b88e5c03d7a5cdd1"
        );
    }

    #[test]
    fn an_older_release_never_becomes_the_first_entry() {
        let f = fixture(TWO);
        let added = add(
            &f.manifest,
            &release(&f, DllType::Dlss, "310.4.1"),
            NOW.parse().unwrap(),
        )
        .unwrap();
        assert_eq!(
            added,
            Added {
                position: 2,
                replaced: false,
                latest: "310.5.3".into()
            }
        );
        assert_eq!(
            versions(&f.manifest, DllType::Dlss),
            ["310.5.3", "310.5.0", "310.4.1"]
        );
        add(
            &f.manifest,
            &release(&f, DllType::Dlss, "310.5.1"),
            NOW.parse().unwrap(),
        )
        .unwrap();
        assert_eq!(
            versions(&f.manifest, DllType::Dlss),
            ["310.5.3", "310.5.1", "310.5.0", "310.4.1"]
        );
    }

    #[test]
    fn the_same_version_is_replaced_and_a_new_family_gets_a_list() {
        let f = fixture(TWO);
        let added = add(
            &f.manifest,
            &release(&f, DllType::Dlss, "310.5.3.0"),
            NOW.parse().unwrap(),
        )
        .unwrap();
        assert!(added.replaced);
        assert_eq!(added.position, 0);
        assert_eq!(
            versions(&f.manifest, DllType::Dlss),
            ["310.5.3.0", "310.5.0"]
        );

        let added = add(
            &f.manifest,
            &release(&f, DllType::Dlssd, "310.5.3"),
            NOW.parse().unwrap(),
        )
        .unwrap();
        assert_eq!((added.position, added.replaced), (0, false));
        assert_eq!(versions(&f.manifest, DllType::Dlssd), ["310.5.3"]);

        let f = fixture(TWO);
        add(
            &f.manifest,
            &release(&f, DllType::Fsr, "1.0"),
            NOW.parse().unwrap(),
        )
        .unwrap();
        let m = Manifest::from_json(&std::fs::read_to_string(&f.manifest).unwrap()).unwrap();
        assert_eq!(
            m.entries(DllType::Fsr)[0].filename,
            "amd_fidelityfx_vk.dll",
            "the catalogue's FSR file"
        );
    }

    #[test]
    fn bad_input_changes_nothing() {
        let f = fixture(TWO);
        let now: Timestamp = NOW.parse().unwrap();
        let mut r = release(&f, DllType::Dlss, "310.6.0");
        r.version = "../310".into();
        assert!(matches!(
            add(&f.manifest, &r, now),
            Err(AddError::BadVersion(_))
        ));
        for url in [
            "https://github.com/x/dlss.dll",
            "http://github.com/x/nvngx_dlss.dll",
            "https://github.com/x/nvngx_dlssg.dll",
            "https://github.com/x/nvngx_dlss.dll?raw=1",
        ] {
            let mut r = release(&f, DllType::Dlss, "310.6.0");
            r.url = url.into();
            let err = add(&f.manifest, &r, now).unwrap_err();
            assert!(matches!(err, AddError::BadUrl { .. }), "{url}");
            assert!(
                err.to_string()
                    .ends_with("must be https and end in /nvngx_dlss.dll")
            );
        }
        let mut r = release(&f, DllType::Dlss, "310.6.0");
        r.file = f.dll.with_file_name("missing.dll");
        assert!(matches!(
            add(&f.manifest, &r, now),
            Err(AddError::ReadDll { .. })
        ));
        assert_eq!(std::fs::read_to_string(&f.manifest).unwrap(), TWO);

        let broken = fixture("{");
        let err = add(
            &broken.manifest,
            &release(&broken, DllType::Dlss, "1.0"),
            now,
        )
        .unwrap_err();
        assert!(matches!(err, AddError::Parse { .. }), "{err}");
        let missing = broken.manifest.with_file_name("none.json");
        assert!(matches!(
            add(&missing, &release(&broken, DllType::Dlss, "1.0"), now),
            Err(AddError::ReadManifest { .. })
        ));
    }

    #[test]
    fn the_shipped_manifest_round_trips_its_entries() {
        let shipped = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data/manifest.json");
        let before = Manifest::from_json(&std::fs::read_to_string(&shipped).unwrap()).unwrap();
        let f = fixture(&std::fs::read_to_string(&shipped).unwrap());
        add(
            &f.manifest,
            &release(&f, DllType::Dlssg, "310.6.0"),
            NOW.parse().unwrap(),
        )
        .unwrap();
        let after = Manifest::from_json(&std::fs::read_to_string(&f.manifest).unwrap()).unwrap();
        for dll_type in DllType::ALL {
            let mut new = after.entries(dll_type).to_vec();
            if dll_type == DllType::Dlssg {
                assert_eq!(new.remove(0).version, "310.6.0");
            }
            assert_eq!(before.entries(dll_type), new, "{dll_type}");
        }
        assert_eq!(after.version, "1");
        assert_eq!(after.repository, before.repository);
    }
}
