//! Stale DLLs: one answer to "is there an update for this DLL" (decision 38).
//!
//! A DLL is stale when its installed version is known and lower than the
//! highest valid manifest version for its family. The Library, the Catalog,
//! `dll check-updates` and the startup auto-update all call [`freshness`],
//! where Go had five meanings (parity inventory, DLL 2.10). An unknown
//! version is its own answer: the screens show it as "unknown", an explicit
//! update still swaps it, and auto-update skips it.

use super::manifest::{Manifest, ManifestEntry};
use super::version;
use crate::games::{DetectedDll, DllType};

/// How an installed DLL compares with the manifest.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Freshness<'a> {
    /// The installed version is the latest, or newer.
    Current {
        /// The manifest's latest entry for the family.
        latest: &'a ManifestEntry,
    },
    /// A newer version is in the manifest.
    Stale {
        /// The manifest's latest entry for the family.
        latest: &'a ManifestEntry,
    },
    /// The installed version could not be read.
    Unknown {
        /// The manifest's latest entry for the family, if it has one.
        latest: Option<&'a ManifestEntry>,
    },
    /// The manifest has no valid entry for the family.
    NotInManifest,
}

impl<'a> Freshness<'a> {
    /// Whether the DLL is stale. An unknown version is not.
    pub fn is_stale(self) -> bool {
        matches!(self, Freshness::Stale { .. })
    }

    /// The manifest's latest entry, when it has one.
    pub fn latest(self) -> Option<&'a ManifestEntry> {
        match self {
            Freshness::Current { latest } | Freshness::Stale { latest } => Some(latest),
            Freshness::Unknown { latest } => latest,
            Freshness::NotInManifest => None,
        }
    }
}

/// How an installed version of a family compares with the manifest. An
/// empty version counts as unknown.
pub fn freshness<'a>(
    dll_type: DllType,
    installed: Option<&str>,
    manifest: &'a Manifest,
) -> Freshness<'a> {
    let latest = manifest.latest(dll_type);
    let installed = installed.filter(|v| !v.is_empty());
    match (installed, latest) {
        (None, latest) => Freshness::Unknown { latest },
        (Some(_), None) => Freshness::NotInManifest,
        (Some(v), Some(latest)) if version::is_newer(v, &latest.version) => {
            Freshness::Stale { latest }
        }
        (Some(_), Some(latest)) => Freshness::Current { latest },
    }
}

/// [`freshness`] of a detected DLL.
pub fn of_dll<'a>(dll: &DetectedDll, manifest: &'a Manifest) -> Freshness<'a> {
    freshness(dll.dll_type, dll.version.as_deref(), manifest)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn manifest() -> Manifest {
        let entry = |v: &str| ManifestEntry {
            version: v.into(),
            filename: "nvngx_dlss.dll".into(),
            ..ManifestEntry::default()
        };
        let mut m = Manifest::default();
        // The older one first: the latest is the highest, not the first.
        m.dlls
            .insert("dlss".into(), vec![entry("310.5.0"), entry("310.5.3")]);
        m.dlls.insert("xess".into(), vec![entry("not a version")]);
        m
    }

    #[test]
    fn stale_only_when_known_and_lower() {
        let m = manifest();
        let f = |v: Option<&str>| freshness(DllType::Dlss, v, &m);
        assert!(f(Some("3.7.10")).is_stale());
        assert!(f(Some("310.5.0")).is_stale());
        assert_eq!(f(Some("310.5.0")).latest().unwrap().version, "310.5.3");
        assert!(matches!(f(Some("310.5.3")), Freshness::Current { .. }));
        assert!(matches!(f(Some("310.5.3.0")), Freshness::Current { .. }));
        assert!(matches!(f(Some("311")), Freshness::Current { .. }));
        assert!(matches!(f(None), Freshness::Unknown { latest: Some(_) }));
        assert!(matches!(f(Some("")), Freshness::Unknown { .. }));
        assert!(!f(None).is_stale());
    }

    #[test]
    fn a_family_with_no_valid_entry_is_not_in_the_manifest() {
        let m = manifest();
        assert_eq!(
            freshness(DllType::Xess, Some("1.3"), &m),
            Freshness::NotInManifest
        );
        assert_eq!(
            freshness(DllType::Fsr, Some("1.0"), &m),
            Freshness::NotInManifest
        );
        assert_eq!(
            freshness(DllType::Fsr, None, &m),
            Freshness::Unknown { latest: None }
        );
    }

    #[test]
    fn of_dll_reads_the_detected_version() {
        let m = manifest();
        let dll = DetectedDll {
            path: "/g/nvngx_dlss.dll".into(),
            name: "nvngx_dlss.dll".into(),
            dll_type: DllType::Dlss,
            version: Some("310.4".into()),
        };
        assert!(of_dll(&dll, &m).is_stale());
    }
}
