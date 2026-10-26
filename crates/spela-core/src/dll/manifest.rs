//! The DLL manifest: its types, the URL, the cache at `manifest-v1.json`
//! and the DLL cache.
//!
//! The remote manifest keeps Go's URL and schema, because every installed Go
//! client reads it (decision 14). Its `dlls` map goes from a manifest key,
//! the word of a [`DllType`], to a list of entries. Keys spela does not know
//! are kept, so the file reads whatever it gains.
//!
//! What changes from Go (decision 8):
//!
//! - The latest entry of a family is the one with the highest version by
//!   the shared compare, not the first in the list.
//! - An entry whose version is not 1 to 4 numbers joined by dots is never
//!   picked and never names a cache file, so a version cannot leave the
//!   DLL cache folder.
//! - [`ManifestEntry::check`] also requires a file name of the entry's own
//!   family, so `dlss.dll` is never installed and an FSR payload only goes
//!   to the file it was made for.
//!
//! The cache is `<cache>/manifest-v1.json` (decision 53): the manifest plus
//! `fetched_at` and `source_url`. While it is missing, Go's
//! `<cache>/manifest.json` is read in its place, as a seed. A cache without
//! the two extra keys counts as fetched from the default URL at the file's
//! modification time, as in Go. Fetching is not here: [`RefreshPolicy`]
//! and [`CachedManifest::is_current`] say when a fetch is due.

use std::collections::BTreeMap;
use std::io;
use std::path::{Path, PathBuf};

use jiff::{SignedDuration, Timestamp};
use serde::{Deserialize, Deserializer, Serialize};

use super::detect::{family, is_file_name_of};
use super::go_json::{self, null_default};
use super::version;
use crate::config::{Config, check_manifest_url};
use crate::games::DllType;
use crate::paths::Paths;

/// The manifest every Go client fetches. It must not move (decision 14).
pub const DEFAULT_MANIFEST_URL: &str =
    "https://raw.githubusercontent.com/jgabor/spela/main/data/manifest.json";

/// The refresh interval when `manifest_refresh_hours` is 0 or less.
pub const DEFAULT_REFRESH_HOURS: i64 = 24;

/// The DLL manifest, Go's schema version 1.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Manifest {
    /// The schema version, `"1"`.
    #[serde(default, deserialize_with = "null_default")]
    pub version: String,
    /// When the manifest was last changed. Optional when reading.
    #[serde(
        default,
        with = "go_json::time",
        skip_serializing_if = "Option::is_none"
    )]
    pub updated_at: Option<Timestamp>,
    /// The repository the DLLs come from. Informational only.
    #[serde(default, deserialize_with = "null_default")]
    pub repository: String,
    /// The entries of each manifest key, newest first by convention.
    #[serde(default, deserialize_with = "null_lists")]
    pub dlls: BTreeMap<String, Vec<ManifestEntry>>,
}

/// One DLL release in the manifest.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ManifestEntry {
    /// The DLL's version, such as `310.5.3`.
    #[serde(default, deserialize_with = "null_default")]
    pub version: String,
    /// The file name in the game folder, such as `nvngx_dlss.dll`.
    #[serde(default, deserialize_with = "null_default")]
    pub filename: String,
    /// Where the raw DLL is downloaded from.
    #[serde(default, deserialize_with = "null_default")]
    pub url: String,
    /// The SHA-256 of the DLL, 64 hex digits.
    #[serde(default, deserialize_with = "null_default")]
    pub sha256: String,
    /// The size in bytes. Informational: the download checks the hash.
    #[serde(default, deserialize_with = "null_default")]
    pub size: i64,
    /// When the DLL was released. Optional when reading.
    #[serde(
        default,
        with = "go_json::time",
        skip_serializing_if = "Option::is_none"
    )]
    pub release_date: Option<Timestamp>,
    /// Where the DLL came from, such as `From techpowerup.com`.
    #[serde(
        default,
        deserialize_with = "null_default",
        skip_serializing_if = "String::is_empty"
    )]
    pub notes: String,
}

/// A `dlls` map whose lists may be `null`, as Go writes a nil slice.
fn null_lists<'de, D: Deserializer<'de>>(
    deserializer: D,
) -> Result<BTreeMap<String, Vec<ManifestEntry>>, D::Error> {
    let raw: Option<BTreeMap<String, Option<Vec<ManifestEntry>>>> =
        Option::deserialize(deserializer)?;
    Ok(raw
        .unwrap_or_default()
        .into_iter()
        .map(|(key, list)| (key, list.unwrap_or_default()))
        .collect())
}

impl Manifest {
    /// Reads a manifest's JSON text.
    pub fn from_json(text: &str) -> Result<Manifest, serde_json::Error> {
        serde_json::from_str(text)
    }

    /// Every entry of a family as the file lists it, valid or not.
    pub fn entries(&self, dll_type: DllType) -> &[ManifestEntry] {
        self.dlls
            .get(dll_type.as_str())
            .map(Vec::as_slice)
            .unwrap_or_default()
    }

    /// The entries of a family whose version is valid, in file order.
    pub fn valid_entries(&self, dll_type: DllType) -> impl Iterator<Item = &ManifestEntry> {
        self.entries(dll_type)
            .iter()
            .filter(|e| version::is_valid(&e.version))
    }

    /// The entry with the highest valid version of a family. When two
    /// entries have equal versions, the first one in the file wins.
    pub fn latest(&self, dll_type: DllType) -> Option<&ManifestEntry> {
        self.valid_entries(dll_type).reduce(|best, e| {
            if version::compare(&e.version, &best.version).is_gt() {
                e
            } else {
                best
            }
        })
    }

    /// The entry of a family with exactly this version, if it is valid.
    pub fn find(&self, dll_type: DllType, version: &str) -> Option<&ManifestEntry> {
        self.valid_entries(dll_type).find(|e| e.version == version)
    }
}

/// Why a manifest entry may not be installed.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum EntryError {
    /// The version is empty, Go's text.
    #[error("invalid {dll_type} manifest entry: version is required")]
    NoVersion {
        /// The family.
        dll_type: DllType,
    },
    /// The version is not 1 to 4 numbers joined by dots.
    #[error(
        "invalid {dll_type} manifest entry: version {version:?} is not 1 to 4 numbers joined by dots"
    )]
    BadVersion {
        /// The family.
        dll_type: DllType,
        /// The version as the manifest has it.
        version: String,
    },
    /// The file name is not a DLL name of the entry's family.
    #[error(
        "invalid {dll_type} {version} manifest filename {filename:?}: not a {} DLL name such as {}",
        family(*dll_type).label,
        family(*dll_type).filename
    )]
    BadFilename {
        /// The family.
        dll_type: DllType,
        /// The entry's version.
        version: String,
        /// The file name as the manifest has it.
        filename: String,
    },
    /// The checksum is not 64 hex digits, Go's text.
    #[error("invalid {dll_type} {version} manifest checksum")]
    BadChecksum {
        /// The family.
        dll_type: DllType,
        /// The entry's version.
        version: String,
    },
}

impl ManifestEntry {
    /// Checks the entry before an install or update uses it: a valid
    /// version, a file name of its own family (any case), and a SHA-256.
    pub fn check(&self, dll_type: DllType) -> Result<(), EntryError> {
        if self.version.is_empty() {
            return Err(EntryError::NoVersion { dll_type });
        }
        if !version::is_valid(&self.version) {
            return Err(EntryError::BadVersion {
                dll_type,
                version: self.version.clone(),
            });
        }
        if !is_file_name_of(dll_type, &self.filename) {
            return Err(EntryError::BadFilename {
                dll_type,
                version: self.version.clone(),
                filename: self.filename.clone(),
            });
        }
        if crate::hex::parse_sha256(&self.sha256).is_none() {
            return Err(EntryError::BadChecksum {
                dll_type,
                version: self.version.clone(),
            });
        }
        Ok(())
    }
}

/// The manifest cache: the manifest plus when and where it was fetched.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct CachedManifest {
    /// The manifest as fetched.
    #[serde(flatten)]
    pub manifest: Manifest,
    /// When it was fetched. A cache without it counts as fetched at the
    /// file's modification time; `None` only when that is unknown too.
    #[serde(
        default,
        with = "go_json::time",
        skip_serializing_if = "Option::is_none"
    )]
    pub fetched_at: Option<Timestamp>,
    /// The URL it was fetched from. Empty means [`DEFAULT_MANIFEST_URL`].
    #[serde(
        default,
        deserialize_with = "null_default",
        skip_serializing_if = "String::is_empty"
    )]
    pub source_url: String,
    /// Whether this was read from Go's `manifest.json` because
    /// `manifest-v1.json` does not exist yet. Not in the file.
    #[serde(skip)]
    pub seeded_from_go: bool,
}

/// When the cached manifest is used and when it is fetched again: Go's
/// policy, read from the config.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RefreshPolicy {
    /// The manifest URL.
    pub url: String,
    /// Fetch again when the cache is older than `max_age`.
    pub auto_refresh: bool,
    /// How old a cache may be while `auto_refresh` is on.
    pub max_age: SignedDuration,
}

impl Default for RefreshPolicy {
    fn default() -> RefreshPolicy {
        RefreshPolicy {
            url: DEFAULT_MANIFEST_URL.to_owned(),
            auto_refresh: true,
            max_age: SignedDuration::from_hours(DEFAULT_REFRESH_HOURS),
        }
    }
}

impl RefreshPolicy {
    /// The policy a config sets. The configured URL counts only when it is
    /// https or http to this machine; otherwise the default URL applies,
    /// with no warning, as in Go. A refresh interval of 0 or less means 24
    /// hours.
    pub fn from_config(config: &Config) -> RefreshPolicy {
        let url = &config.dll_manifest_url;
        let hours = match config.manifest_refresh_hours {
            h if h > 0 => h,
            _ => DEFAULT_REFRESH_HOURS,
        };
        RefreshPolicy {
            url: if !url.is_empty() && check_manifest_url(url).is_ok() {
                url.clone()
            } else {
                DEFAULT_MANIFEST_URL.to_owned()
            },
            auto_refresh: config.auto_refresh_manifest,
            max_age: hours
                .checked_mul(3600)
                .map_or(SignedDuration::MAX, SignedDuration::from_secs),
        }
    }
}

impl CachedManifest {
    /// The URL the cache came from, the default URL for a cache that does
    /// not say.
    pub fn source(&self) -> &str {
        if self.source_url.is_empty() {
            DEFAULT_MANIFEST_URL
        } else {
            &self.source_url
        }
    }

    /// Whether the cache may be used as it is at `now`: it came from the
    /// policy's URL, and auto refresh is off or the cache is younger than
    /// the interval. A cache from another URL is fetched again even with
    /// auto refresh off. A cache with no fetch time is never current while
    /// auto refresh is on.
    pub fn is_current(&self, policy: &RefreshPolicy, now: Timestamp) -> bool {
        if self.source() != policy.url {
            return false;
        }
        if !policy.auto_refresh {
            return true;
        }
        self.fetched_at
            .is_some_and(|at| now.duration_since(at) < policy.max_age)
    }
}

/// The manifest cache cannot be read.
#[derive(Debug, thiserror::Error)]
pub enum ManifestCacheError {
    /// The file is there but cannot be read.
    #[error("cannot read the manifest cache {}: {source}", path.display())]
    Read {
        /// The cache file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
    /// The file does not parse.
    #[error("the manifest cache {} does not parse: {source}", path.display())]
    Parse {
        /// The cache file.
        path: PathBuf,
        /// The cause.
        source: serde_json::Error,
    },
}

/// The cached manifest: `manifest-v1.json`, or Go's `manifest.json` while
/// that is missing. `None` when neither exists. Nothing is written.
pub fn load_cache(paths: &Paths) -> Result<Option<CachedManifest>, ManifestCacheError> {
    if let Some(cached) = read_cache(&paths.manifest_cache())? {
        return Ok(Some(cached));
    }
    Ok(
        read_cache(&paths.go().manifest_cache())?.map(|cached| CachedManifest {
            seeded_from_go: true,
            ..cached
        }),
    )
}

fn read_cache(path: &Path) -> Result<Option<CachedManifest>, ManifestCacheError> {
    let text = match std::fs::read_to_string(path) {
        Ok(text) => text,
        Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(None),
        Err(source) => {
            return Err(ManifestCacheError::Read {
                path: path.to_owned(),
                source,
            });
        }
    };
    let mut cached: CachedManifest =
        serde_json::from_str(&text).map_err(|source| ManifestCacheError::Parse {
            path: path.to_owned(),
            source,
        })?;
    if cached.fetched_at.is_none() {
        // A cache from before fetch times were recorded was written when
        // it was fetched.
        cached.fetched_at = std::fs::metadata(path)
            .and_then(|m| m.modified())
            .ok()
            .and_then(|t| Timestamp::try_from(t).ok());
    }
    Ok(Some(cached))
}

/// A version that may not name a DLL cache file.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("invalid DLL version {0:?}: a version is 1 to 4 numbers joined by dots")]
pub struct InvalidVersion(pub String);

/// The DLL cache folder of one family: `<dll cache>/<manifest key>`.
/// `dll_cache_dir` is [`Config::dll_cache_dir`].
pub fn cache_dir(dll_cache_dir: &Path, dll_type: DllType) -> PathBuf {
    dll_cache_dir.join(dll_type.as_str())
}

/// Where one version of a family is cached: `<dll cache>/<key>/<version>.dll`.
/// The version is checked first, so it can never leave the folder.
pub fn cache_file(
    dll_cache_dir: &Path,
    dll_type: DllType,
    version: &str,
) -> Result<PathBuf, InvalidVersion> {
    if !version::is_valid(version) {
        return Err(InvalidVersion(version.to_owned()));
    }
    Ok(cache_dir(dll_cache_dir, dll_type).join(format!("{version}.dll")))
}

/// The versions of a family in the DLL cache, newest first: the `.dll`
/// files of its folder with the suffix taken off, as Go lists them, but
/// only names that are valid versions. A missing folder is an empty list.
pub fn cached_versions(dll_cache_dir: &Path, dll_type: DllType) -> io::Result<Vec<String>> {
    let entries = match std::fs::read_dir(cache_dir(dll_cache_dir, dll_type)) {
        Ok(entries) => entries,
        Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(Vec::new()),
        Err(e) => return Err(e),
    };
    let mut versions = Vec::new();
    for entry in entries {
        let entry = entry?;
        if entry.file_type()?.is_dir() {
            continue;
        }
        let name = entry.file_name();
        let Some(version) = name.to_str().and_then(|n| n.strip_suffix(".dll")) else {
            continue;
        };
        if version::is_valid(version) {
            versions.push(version.to_owned());
        }
    }
    versions.sort_by(|a, b| version::compare(b, a).then_with(|| a.cmp(b)));
    Ok(versions)
}

#[cfg(test)]
mod tests {
    use super::*;

    const SHA: &str = "8707e53b26c68c606b98bf31c223485ff30d310a261b1a36d48b2eaabc1507ec";

    fn entry(version: &str, filename: &str) -> ManifestEntry {
        ManifestEntry {
            version: version.into(),
            filename: filename.into(),
            url: format!("https://example.test/{version}/{filename}"),
            sha256: SHA.into(),
            ..ManifestEntry::default()
        }
    }

    fn manifest(dlss: Vec<ManifestEntry>) -> Manifest {
        let mut m = Manifest::default();
        m.dlls.insert("dlss".into(), dlss);
        m
    }

    #[test]
    fn reads_go_s_schema_with_optional_times_and_unknown_keys() {
        let text = r#"{"version":"1","updated_at":"2026-02-03T06:24:32Z","repository":"jgabor/spela-dlls",
            "dlls":{"dlss":[{"version":"310.5.3","filename":"nvngx_dlss.dll","url":"https://x/a","sha256":"ab",
            "size":54779504,"release_date":"2026-02-03T06:24:32Z","notes":"From techpowerup.com"}],
            "fsr":[],"xess":null,"future_dll":[{"version":"1"}]},"extra":true}"#;
        let m = Manifest::from_json(text).unwrap();
        assert_eq!(m.version, "1");
        assert_eq!(m.updated_at, Some("2026-02-03T06:24:32Z".parse().unwrap()));
        let dlss = &m.entries(DllType::Dlss)[0];
        assert_eq!(dlss.size, 54779504);
        assert_eq!(dlss.notes, "From techpowerup.com");
        assert!(m.entries(DllType::Xess).is_empty());
        assert!(m.entries(DllType::Dlssd).is_empty());
        assert_eq!(m.dlls["future_dll"][0].version, "1");

        // Go's policy test fixture: no times at all.
        let bare =
            Manifest::from_json(r#"{"version":"1","repository":"/primary","dlls":{}}"#).unwrap();
        assert_eq!(bare.updated_at, None);
        // A Go zero time is no time.
        let zero =
            Manifest::from_json(r#"{"updated_at":"0001-01-01T00:00:00Z","dlls":null}"#).unwrap();
        assert_eq!(zero, Manifest::default());
    }

    #[test]
    fn serializes_in_go_s_field_order() {
        let mut m = manifest(vec![entry("310.5.3", "nvngx_dlss.dll")]);
        m.version = "1".into();
        m.updated_at = Some("2026-02-03T06:24:32Z".parse().unwrap());
        let text = serde_json::to_string(&m).unwrap();
        assert!(
            text.starts_with(r#"{"version":"1","updated_at":"2026-02-03T06:24:32Z","repository":"","dlls":{"dlss":[{"version":"310.5.3","filename":"nvngx_dlss.dll","url":"#),
            "{text}"
        );
        assert!(
            !text.contains("notes"),
            "notes is omitted when empty: {text}"
        );
        assert_eq!(Manifest::from_json(&text).unwrap(), m);
    }

    #[test]
    fn latest_is_the_highest_valid_version_not_the_first() {
        let m = manifest(vec![
            entry("310.1.0", "nvngx_dlss.dll"),
            entry("310.5.3", "nvngx_dlss.dll"),
            entry("../../../evil", "nvngx_dlss.dll"),
            entry("999.0.0-beta", "nvngx_dlss.dll"),
            entry("310.5.3.0", "nvngx_dlss.dll"),
            entry("310.4", "nvngx_dlss.dll"),
        ]);
        let latest = m.latest(DllType::Dlss).unwrap();
        assert_eq!(latest.version, "310.5.3", "the first of two equal versions");
        assert!(std::ptr::eq(latest, &m.entries(DllType::Dlss)[1]));
        assert_eq!(m.latest(DllType::Xess), None);
        assert_eq!(m.find(DllType::Dlss, "310.4").unwrap().version, "310.4");
        assert_eq!(m.find(DllType::Dlss, "../../../evil"), None);
        assert_eq!(m.find(DllType::Dlss, "310.9"), None);
        assert_eq!(m.valid_entries(DllType::Dlss).count(), 4);
        assert_eq!(m.entries(DllType::Dlss).len(), 6);
    }

    #[test]
    fn check_refuses_what_install_must_not_write() {
        assert_eq!(
            entry("310.5.3", "nvngx_dlss.dll").check(DllType::Dlss),
            Ok(())
        );
        assert_eq!(
            entry("310.5.3", "NVNGX_DLSS.DLL").check(DllType::Dlss),
            Ok(())
        );
        assert_eq!(
            entry("1.0", "amd_fidelityfx_dx12.dll").check(DllType::Fsr),
            Ok(())
        );
        let err = |e: ManifestEntry, t| e.check(t).unwrap_err().to_string();
        assert_eq!(
            err(entry("", "nvngx_dlss.dll"), DllType::Dlss),
            "invalid dlss manifest entry: version is required"
        );
        assert_eq!(
            err(entry("../x", "nvngx_dlss.dll"), DllType::Dlss),
            r#"invalid dlss manifest entry: version "../x" is not 1 to 4 numbers joined by dots"#
        );
        // dll-release.yml's `<type>.dll`, a path, and another family's name.
        for bad in [
            "dlss.dll",
            "../nvngx_dlss.dll",
            "sub/nvngx_dlss.dll",
            "nvngx_dlssg.dll",
            "",
        ] {
            assert!(
                matches!(
                    entry("310.5.3", bad).check(DllType::Dlss),
                    Err(EntryError::BadFilename { .. })
                ),
                "{bad}"
            );
        }
        assert_eq!(
            err(entry("310.5.3", "dlss.dll"), DllType::Dlss),
            r#"invalid dlss 310.5.3 manifest filename "dlss.dll": not a DLSS DLL name such as nvngx_dlss.dll"#
        );
        let mut short = entry("310.5.3", "nvngx_dlss.dll");
        short.sha256 = "abc".into();
        assert_eq!(
            err(short, DllType::Dlss),
            "invalid dlss 310.5.3 manifest checksum"
        );
        let mut upper = entry("310.5.3", "nvngx_dlss.dll");
        upper.sha256 = SHA.to_uppercase();
        assert_eq!(upper.check(DllType::Dlss), Ok(()));
    }

    #[test]
    fn policy_follows_the_config() {
        let mut config = Config::default();
        assert_eq!(
            RefreshPolicy::from_config(&config),
            RefreshPolicy::default()
        );
        config.dll_manifest_url = "http://example.test/manifest.json".into();
        config.manifest_refresh_hours = 12;
        let p = RefreshPolicy::from_config(&config);
        assert_eq!(p.url, DEFAULT_MANIFEST_URL, "an unsafe URL is ignored");
        assert_eq!(p.max_age, SignedDuration::from_hours(12));
        config.dll_manifest_url = "https://example.test/manifest.json".into();
        assert_eq!(
            RefreshPolicy::from_config(&config).url,
            "https://example.test/manifest.json"
        );
        config.dll_manifest_url = "http://127.0.0.1:8080/m.json".into();
        assert_eq!(
            RefreshPolicy::from_config(&config).url,
            "http://127.0.0.1:8080/m.json"
        );
        for hours in [0, -5] {
            config.manifest_refresh_hours = hours;
            assert_eq!(
                RefreshPolicy::from_config(&config).max_age,
                SignedDuration::from_hours(24)
            );
        }
        config.manifest_refresh_hours = i64::MAX;
        assert_eq!(
            RefreshPolicy::from_config(&config).max_age,
            SignedDuration::MAX
        );
        config.auto_refresh_manifest = false;
        assert!(!RefreshPolicy::from_config(&config).auto_refresh);
    }

    #[test]
    fn is_current_follows_go_s_policy_test() {
        // internal/dll/manifest_policy_test.go, TestGetManifestFollowsRefreshSettings.
        let now: Timestamp = "2026-10-26T15:00:00Z".parse().unwrap();
        let hours = |h: i64| now - SignedDuration::from_hours(h);
        let primary = "http://127.0.0.1:9/primary";
        let cache = |source: &str, at| CachedManifest {
            source_url: source.into(),
            fetched_at: Some(at),
            ..CachedManifest::default()
        };
        let policy = |url: &str, auto_refresh, max_hours| RefreshPolicy {
            url: url.into(),
            auto_refresh,
            max_age: SignedDuration::from_hours(max_hours),
        };
        assert!(cache(primary, now).is_current(&policy(primary, true, 6), now));
        assert!(cache(primary, hours(2)).is_current(&policy(primary, true, 6), now));
        assert!(!cache(primary, hours(2)).is_current(&policy(primary, true, 1), now));
        assert!(cache(primary, hours(720)).is_current(&policy(primary, false, 1), now));
        assert!(
            !cache(primary, now).is_current(&policy("http://127.0.0.1:9/other", false, 24), now),
            "a changed source is fetched even with auto refresh off"
        );
        // A cache from the future counts as fresh, as Go's time.Since does.
        assert!(
            cache(primary, now + SignedDuration::from_hours(5))
                .is_current(&policy(primary, true, 1), now)
        );
        // An old cache with no source came from the default URL.
        let legacy = CachedManifest::default();
        assert_eq!(legacy.source(), DEFAULT_MANIFEST_URL);
        assert!(legacy.is_current(&policy(DEFAULT_MANIFEST_URL, false, 24), now));
        assert!(
            !legacy.is_current(&policy(DEFAULT_MANIFEST_URL, true, 24), now),
            "no time"
        );
    }

    #[test]
    fn load_cache_prefers_v1_seeds_from_go_and_falls_back_to_the_file_time() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        assert!(load_cache(&paths).unwrap().is_none());

        std::fs::create_dir_all(&paths.cache_dir).unwrap();
        let go = paths.go().manifest_cache();
        std::fs::write(&go, r#"{"version":"1","updated_at":"2020-01-01T00:00:00Z","repository":"legacy","dlls":{}}"#).unwrap();
        let seeded = load_cache(&paths).unwrap().unwrap();
        assert!(seeded.seeded_from_go);
        assert_eq!(seeded.manifest.repository, "legacy");
        assert_eq!(seeded.source(), DEFAULT_MANIFEST_URL);
        let mtime =
            Timestamp::try_from(std::fs::metadata(&go).unwrap().modified().unwrap()).unwrap();
        assert_eq!(seeded.fetched_at, Some(mtime), "the file time stands in");
        assert!(!paths.manifest_cache().exists(), "reading writes nothing");

        std::fs::write(
            paths.manifest_cache(),
            r#"{"version":"1","repository":"v1","dlls":{},"fetched_at":"2026-10-26T02:45:16.376726614+02:00","source_url":"https://example.test/m.json"}"#,
        )
        .unwrap();
        let v1 = load_cache(&paths).unwrap().unwrap();
        assert!(!v1.seeded_from_go);
        assert_eq!(v1.manifest.repository, "v1");
        assert_eq!(
            v1.fetched_at,
            Some("2026-10-26T00:45:16.376726614Z".parse().unwrap())
        );
        assert_eq!(v1.source(), "https://example.test/m.json");

        // A broken v1 file is an error, not a reason to read Go's.
        std::fs::write(paths.manifest_cache(), "{").unwrap();
        let err = load_cache(&paths).unwrap_err();
        assert!(matches!(err, ManifestCacheError::Parse { .. }), "{err}");
        assert!(
            err.to_string().contains("manifest-v1.json does not parse"),
            "{err}"
        );
    }

    #[test]
    fn cache_files_are_checked_and_listed_newest_first() {
        let base = tempfile::tempdir().unwrap();
        let dir = base.path().join("dlls");
        assert_eq!(
            cache_file(&dir, DllType::Dlssg, "310.5.3").unwrap(),
            dir.join("dlssg/310.5.3.dll")
        );
        for bad in ["../../x", "", "310.5.3/../../y", "v310"] {
            assert_eq!(
                cache_file(&dir, DllType::Dlss, bad),
                Err(InvalidVersion(bad.into())),
                "{bad}"
            );
        }
        assert!(cached_versions(&dir, DllType::Dlss).unwrap().is_empty());
        let dlss = cache_dir(&dir, DllType::Dlss);
        std::fs::create_dir_all(dlss.join("310.9.9.dll")).unwrap();
        for name in [
            "310.5.0.dll",
            "310.5.3.dll",
            "3.7.10.dll",
            "evil.dll",
            "310.6.0.txt",
            "310.10.dll",
        ] {
            std::fs::write(dlss.join(name), b"x").unwrap();
        }
        assert_eq!(
            cached_versions(&dir, DllType::Dlss).unwrap(),
            ["310.10", "310.5.3", "310.5.0", "3.7.10"]
        );
    }
}
