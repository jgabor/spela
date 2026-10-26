//! What comes over the network for DLLs: the manifest, under its refresh
//! policy, and the DLL payloads, into the checksum-verified DLL cache.
//!
//! Both go through the [`Http`] backend, so tests use the fake or a loopback
//! server and never a real host.
//!
//! The manifest (Go's `GetManifest`, parity inventory DLL 2.4):
//!
//! - The URL is the policy's: the configured one when it is https or http to
//!   this machine, otherwise the default ([`RefreshPolicy::from_config`]).
//! - A cache that [`CachedManifest::is_current`] accepts is used as it is.
//!   Otherwise the manifest is fetched, with Go's timeouts and a 4 MiB body
//!   cap Go lacked, and saved to `manifest-v1.json` with `fetched_at` and
//!   `source_url`.
//! - A failed fetch falls back to the cache, whatever its source or age, and
//!   says so. With no cache it is an error.
//! - A cache that does not parse counts as no cache, as in Go.
//!
//! A payload (Go's `acquireDLL`, DLL 2.5): the cached file is used when its
//! SHA-256 matches the entry. Otherwise it is downloaded through a hasher
//! into a temp file in the cache folder, and renamed into place only when
//! the hash matches, so the cache never holds a file with the wrong bytes.
//! A cached-only acquire never downloads. No lock is held: the rename is
//! atomic, and the download runs before `games.lock` is taken.

use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};

use sha2::{Digest, Sha256};

use super::manifest::{self, CachedManifest, Manifest, ManifestEntry, RefreshPolicy};
use crate::backend::http::{Http, HttpError, HttpRequest, MANIFEST_BODY_CAP};
use crate::clock::Clock;
use crate::fs::{Durability, write_atomic};
use crate::games::DllType;
use crate::hex;
use crate::paths::Paths;

/// The manifest an operation uses, and where it came from.
#[derive(Debug, Clone, PartialEq)]
pub struct ManifestSource {
    /// The manifest with its fetch time and URL.
    pub cached: CachedManifest,
    /// Whether it was fetched now.
    pub fetched: bool,
    /// Why the cache stands in for a fetch that failed, or why a fetched
    /// manifest could not be saved.
    pub warning: Option<String>,
}

impl ManifestSource {
    /// The manifest itself.
    pub fn manifest(&self) -> &Manifest {
        &self.cached.manifest
    }
}

/// No manifest to use.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum ManifestError {
    /// The fetch failed and there is no cache to fall back to.
    #[error("failed to fetch the DLL manifest from {url}: {reason}")]
    Fetch {
        /// The URL asked.
        url: String,
        /// Why.
        reason: String,
    },
    /// Cached only: there is no usable cache.
    #[error("no DLL manifest in the cache")]
    NoCache,
}

/// The manifest under the refresh policy. `force` fetches even when the
/// cache is current. Nothing in Go forced a fetch; `dll check-updates
/// --refresh` does.
pub fn get_manifest(
    paths: &Paths,
    policy: &RefreshPolicy,
    http: &dyn Http,
    clock: &dyn Clock,
    force: bool,
) -> Result<ManifestSource, ManifestError> {
    let cached = load_usable_cache(paths);
    let now = clock.now();
    if !force
        && let Some(cached) = &cached
        && cached.is_current(policy, now)
    {
        return Ok(ManifestSource {
            cached: cached.clone(),
            fetched: false,
            warning: None,
        });
    }
    match fetch_manifest(&policy.url, http) {
        Ok(manifest) => {
            let fresh = CachedManifest {
                manifest,
                fetched_at: Some(now),
                source_url: policy.url.clone(),
            };
            let warning = save_cache(paths, &fresh)
                .err()
                .map(|e| format!("cannot save the DLL manifest cache: {e}"));
            Ok(ManifestSource {
                cached: fresh,
                fetched: true,
                warning,
            })
        }
        Err(reason) => match cached {
            Some(cached) => Ok(ManifestSource {
                warning: Some(format!(
                    "using the cached DLL manifest: cannot fetch {}: {reason}",
                    policy.url
                )),
                cached,
                fetched: false,
            }),
            None => Err(ManifestError::Fetch {
                url: policy.url.clone(),
                reason,
            }),
        },
    }
}

/// The cached manifest only, for cached-only updates: no request at all.
pub fn cached_manifest(paths: &Paths) -> Result<ManifestSource, ManifestError> {
    let cached = load_usable_cache(paths).ok_or(ManifestError::NoCache)?;
    Ok(ManifestSource {
        cached,
        fetched: false,
        warning: None,
    })
}

fn load_usable_cache(paths: &Paths) -> Option<CachedManifest> {
    match manifest::load_cache(paths) {
        Ok(cached) => cached,
        Err(e) => {
            tracing::warn!("{e}; fetching it again");
            None
        }
    }
}

/// Fetches and parses the manifest at `url`.
fn fetch_manifest(url: &str, http: &dyn Http) -> Result<Manifest, String> {
    let uri: http::Uri = url.parse().map_err(|e| format!("bad URL: {e}"))?;
    let request = HttpRequest::new(uri).with_body_cap(MANIFEST_BODY_CAP);
    let mut body = Vec::new();
    http.get(&request, &mut body, &mut |_, _| {})
        .map_err(|e| e.to_string())?;
    Manifest::from_json(&String::from_utf8_lossy(&body))
        .map_err(|e| format!("the manifest does not parse: {e}"))
}

/// Writes the manifest cache atomically, without fsync: it is rebuildable.
pub fn save_cache(paths: &Paths, cached: &CachedManifest) -> io::Result<()> {
    let text = serde_json::to_vec_pretty(cached).map_err(io::Error::other)?;
    std::fs::create_dir_all(&paths.cache_dir)?;
    write_atomic(&paths.manifest_cache(), &text, Durability::Cache)
}

/// Why a payload could not be had. The texts are Go's where Go had one.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum AcquireError {
    /// The version cannot name a cache file.
    #[error(transparent)]
    Version(#[from] manifest::InvalidVersion),
    /// Cached only, and the cached file is missing.
    #[error("cached DLL not found: {}", .0.display())]
    NotCached(PathBuf),
    /// Cached only, and the cached file has other bytes.
    #[error("cached DLL checksum mismatch: expected {expected}, got {actual}")]
    CachedMismatch {
        /// The manifest's SHA-256.
        expected: String,
        /// The file's.
        actual: String,
    },
    /// The cached file cannot be read.
    #[error("read cached DLL: {0}")]
    Read(String),
    /// The download failed.
    #[error("download DLL: {0}")]
    Download(HttpError),
    /// The downloaded bytes are not the manifest's.
    #[error("cache DLL: checksum mismatch: expected {expected}, got {actual}")]
    Mismatch {
        /// The manifest's SHA-256.
        expected: String,
        /// The download's.
        actual: String,
    },
    /// The entry's URL does not parse.
    #[error("download DLL: bad URL {0:?}")]
    BadUrl(String),
    /// The cache folder or file cannot be written.
    #[error("cache DLL: {0}")]
    Write(String),
    /// spela is quitting, so the download stopped. Nothing was kept.
    #[error("download DLL: stopped because spela is quitting")]
    Stopped,
}

/// The checksum-verified payload of `entry` in the DLL cache, downloading it
/// unless `allow_download` is false. `progress` gets the bytes so far and
/// the total when the server sent one. When `stop` is set, a download in
/// progress ends at its next chunk and its temp file is removed.
pub fn acquire(
    http: &dyn Http,
    dll_cache_dir: &Path,
    dll_type: DllType,
    entry: &ManifestEntry,
    allow_download: bool,
    stop: Option<&AtomicBool>,
    progress: &mut dyn FnMut(u64, Option<u64>),
) -> Result<PathBuf, AcquireError> {
    let path = manifest::cache_file(dll_cache_dir, dll_type, &entry.version)?;
    match std::fs::File::open(&path).and_then(hex::sha256_reader) {
        Ok(actual) if same_sha(&actual, &entry.sha256) => return Ok(path),
        Ok(actual) if !allow_download => {
            return Err(AcquireError::CachedMismatch {
                expected: entry.sha256.clone(),
                actual,
            });
        }
        Err(e) if e.kind() == io::ErrorKind::NotFound && !allow_download => {
            return Err(AcquireError::NotCached(path));
        }
        Err(e) if e.kind() != io::ErrorKind::NotFound && !allow_download => {
            return Err(AcquireError::Read(e.to_string()));
        }
        _ => {}
    }
    let uri: http::Uri = entry
        .url
        .parse()
        .map_err(|_| AcquireError::BadUrl(entry.url.clone()))?;
    let dir = manifest::cache_dir(dll_cache_dir, dll_type);
    std::fs::create_dir_all(&dir).map_err(|e| AcquireError::Write(e.to_string()))?;
    let mut prefix = std::ffi::OsString::from(".");
    prefix.push(path.file_name().unwrap_or_default());
    prefix.push(".");
    let mut tmp = tempfile::Builder::new()
        .prefix(&prefix)
        .suffix(".tmp")
        .tempfile_in(&dir)
        .map_err(|e| AcquireError::Write(e.to_string()))?;
    let stopped = || stop.is_some_and(|s| s.load(Ordering::SeqCst));
    if stopped() {
        return Err(AcquireError::Stopped);
    }
    let mut hashing = HashingWriter {
        inner: tmp.as_file_mut(),
        hasher: Sha256::new(),
        stop,
    };
    let got = http.get(&HttpRequest::new(uri), &mut hashing, progress);
    if stopped() {
        // Dropping the temp file deletes it.
        return Err(AcquireError::Stopped);
    }
    got.map_err(AcquireError::Download)?;
    let actual = hex::encode(&hashing.hasher.finalize());
    if !same_sha(&actual, &entry.sha256) {
        // Dropping the temp file deletes it: the cache never holds it.
        return Err(AcquireError::Mismatch {
            expected: entry.sha256.clone(),
            actual,
        });
    }
    {
        use std::os::unix::fs::PermissionsExt;
        tmp.as_file()
            .set_permissions(std::fs::Permissions::from_mode(crate::fs::NEW_FILE_MODE))
            .map_err(|e| AcquireError::Write(e.to_string()))?;
    }
    tmp.persist(&path)
        .map_err(|e| AcquireError::Write(e.error.to_string()))?;
    Ok(path)
}

fn same_sha(a: &str, b: &str) -> bool {
    hex::parse_sha256(a).is_some_and(|a| hex::parse_sha256(b) == Some(a))
}

/// Writes through to a file and hashes what it wrote. It refuses to write
/// once `stop` is set, which ends the download.
struct HashingWriter<'a> {
    inner: &'a mut std::fs::File,
    hasher: Sha256,
    stop: Option<&'a AtomicBool>,
}

impl Write for HashingWriter<'_> {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        if self.stop.is_some_and(|s| s.load(Ordering::SeqCst)) {
            // Not `Interrupted`: `write_all` would retry it.
            return Err(io::Error::other("stopped"));
        }
        let n = self.inner.write(buf)?;
        self.hasher.update(&buf[..n]);
        Ok(n)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.inner.flush()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::clock::ManualClock;
    use crate::fake::{FakeHttp, FakeRoute};
    use jiff::SignedDuration;

    const URL: &str = "http://127.0.0.1:9/manifest.json";

    fn manifest_json(version: &str) -> String {
        format!(
            r#"{{"version":"1","repository":"fixture","dlls":{{"dlss":[{{"version":"{version}","filename":"nvngx_dlss.dll","url":"http://127.0.0.1:9/dlss.dll","sha256":"{}"}}]}}}}"#,
            hex::sha256_hex(b"payload")
        )
    }

    fn policy(auto_refresh: bool) -> RefreshPolicy {
        RefreshPolicy {
            url: URL.into(),
            auto_refresh,
            max_age: SignedDuration::from_hours(24),
        }
    }

    fn clock() -> ManualClock {
        ManualClock::new("2026-10-26T15:00:00Z".parse().unwrap())
    }

    #[test]
    fn fetches_saves_then_reuses_a_current_cache() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let http = FakeHttp::new();
        http.serve(URL, manifest_json("310.5.3"));
        let clock = clock();
        let got = get_manifest(&paths, &policy(true), &http, &clock, false).unwrap();
        assert!(got.fetched);
        assert_eq!(got.warning, None);
        assert_eq!(
            got.manifest().latest(DllType::Dlss).unwrap().version,
            "310.5.3"
        );
        let saved = manifest::load_cache(&paths).unwrap().unwrap();
        assert_eq!(saved.source_url, URL);
        assert_eq!(saved.fetched_at, Some(clock.now()));
        assert_eq!(http.requests()[0].body_cap, MANIFEST_BODY_CAP);

        // Current: no second request.
        clock.advance(SignedDuration::from_hours(2));
        let again = get_manifest(&paths, &policy(true), &http, &clock, false).unwrap();
        assert!(!again.fetched);
        assert_eq!(http.requests().len(), 1);
        // Forced: fetched again.
        get_manifest(&paths, &policy(true), &http, &clock, true).unwrap();
        assert_eq!(http.requests().len(), 2);
        // Stale: fetched again.
        clock.advance(SignedDuration::from_hours(30));
        http.serve(URL, manifest_json("310.6.0"));
        let fresh = get_manifest(&paths, &policy(true), &http, &clock, false).unwrap();
        assert!(fresh.fetched);
        assert_eq!(
            fresh.manifest().latest(DllType::Dlss).unwrap().version,
            "310.6.0"
        );
    }

    #[test]
    fn a_failed_fetch_falls_back_to_any_cache_and_says_so() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let http = FakeHttp::new();
        http.route(URL, FakeRoute::Fault(HttpError::Timeout));
        let err = get_manifest(&paths, &policy(true), &http, &clock(), false).unwrap_err();
        assert_eq!(
            err.to_string(),
            format!("failed to fetch the DLL manifest from {URL}: the request timed out")
        );
        assert!(!paths.manifest_cache().exists());

        // A cache from another source is still the fallback.
        let old = CachedManifest {
            manifest: Manifest::from_json(&manifest_json("310.1.0")).unwrap(),
            fetched_at: None,
            source_url: "https://example.invalid/other.json".into(),
        };
        save_cache(&paths, &old).unwrap();
        let got = get_manifest(&paths, &policy(false), &http, &clock(), false).unwrap();
        assert!(!got.fetched);
        assert_eq!(
            got.manifest().latest(DllType::Dlss).unwrap().version,
            "310.1.0"
        );
        let warning = got.warning.unwrap();
        assert!(
            warning.starts_with("using the cached DLL manifest: cannot fetch"),
            "{warning}"
        );

        // A body that does not parse is a failed fetch too.
        http.serve(URL, "{not json");
        let got = get_manifest(&paths, &policy(true), &http, &clock(), true).unwrap();
        assert!(got.warning.unwrap().contains("does not parse"));
    }

    #[test]
    fn a_broken_cache_counts_as_none_and_cached_only_never_asks() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        std::fs::create_dir_all(&paths.cache_dir).unwrap();
        std::fs::write(paths.manifest_cache(), "{").unwrap();
        assert_eq!(cached_manifest(&paths), Err(ManifestError::NoCache));
        let http = FakeHttp::new();
        http.serve(URL, manifest_json("310.5.3"));
        let got = get_manifest(&paths, &policy(false), &http, &clock(), false).unwrap();
        assert!(got.fetched, "rebuilt from the network");
        assert!(cached_manifest(&paths).is_ok());
    }

    fn entry(body: &[u8]) -> ManifestEntry {
        ManifestEntry {
            version: "310.5.3".into(),
            filename: "nvngx_dlss.dll".into(),
            url: "http://127.0.0.1:9/dlss.dll".into(),
            sha256: hex::sha256_hex(body).to_uppercase(),
            ..ManifestEntry::default()
        }
    }

    fn names(dir: &Path) -> Vec<String> {
        let mut v: Vec<String> = std::fs::read_dir(dir)
            .map(|d| {
                d.map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
                    .collect()
            })
            .unwrap_or_default();
        v.sort();
        v
    }

    #[test]
    fn acquire_downloads_checks_and_reuses_the_cache() {
        let base = tempfile::tempdir().unwrap();
        let dir = base.path().join("dlls");
        let body = vec![9u8; 50_000];
        let http = FakeHttp::new();
        http.serve("http://127.0.0.1:9/dlss.dll", body.clone());
        let mut seen = Vec::new();
        let path = acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(&body),
            true,
            None,
            &mut |n, t| seen.push((n, t)),
        )
        .unwrap();
        assert_eq!(path, dir.join("dlss/310.5.3.dll"));
        assert_eq!(std::fs::read(&path).unwrap(), body);
        assert_eq!(seen.last(), Some(&(50_000, Some(50_000))));
        assert_eq!(names(&dir.join("dlss")), ["310.5.3.dll"]);

        // A valid cache makes no request, even when downloads are allowed.
        acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(&body),
            true,
            None,
            &mut |_, _| {},
        )
        .unwrap();
        acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(&body),
            false,
            None,
            &mut |_, _| {},
        )
        .unwrap();
        assert_eq!(http.requests().len(), 1);

        // A corrupt cache is downloaded again, or refused when cached only.
        std::fs::write(&path, b"corrupt").unwrap();
        let err = acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(&body),
            false,
            None,
            &mut |_, _| {},
        )
        .unwrap_err();
        assert!(matches!(err, AcquireError::CachedMismatch { .. }), "{err}");
        assert!(
            err.to_string()
                .starts_with("cached DLL checksum mismatch: expected ")
        );
        acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(&body),
            true,
            None,
            &mut |_, _| {},
        )
        .unwrap();
        assert_eq!(std::fs::read(&path).unwrap(), body);
        assert_eq!(http.requests().len(), 2);
    }

    #[test]
    fn acquire_never_keeps_a_bad_download() {
        let base = tempfile::tempdir().unwrap();
        let dir = base.path().join("dlls");
        let http = FakeHttp::new();
        http.serve("http://127.0.0.1:9/dlss.dll", b"tampered".to_vec());
        let err = acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(b"payload"),
            true,
            None,
            &mut |_, _| {},
        )
        .unwrap_err();
        assert!(matches!(err, AcquireError::Mismatch { .. }), "{err}");
        assert!(
            names(&dir.join("dlss")).is_empty(),
            "no file and no temp file"
        );

        http.route(
            "http://127.0.0.1:9/dlss.dll",
            FakeRoute::Body {
                status: 404,
                body: Vec::new(),
                link: None,
            },
        );
        let err = acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(b"payload"),
            true,
            None,
            &mut |_, _| {},
        )
        .unwrap_err();
        assert_eq!(
            err.to_string(),
            "download DLL: the server answered HTTP 404"
        );
        assert!(names(&dir.join("dlss")).is_empty());

        let missing = acquire(
            &http,
            &dir,
            DllType::Dlss,
            &entry(b"payload"),
            false,
            None,
            &mut |_, _| {},
        )
        .unwrap_err();
        assert_eq!(
            missing.to_string(),
            format!(
                "cached DLL not found: {}",
                dir.join("dlss/310.5.3.dll").display()
            )
        );
        let mut bad = entry(b"payload");
        bad.version = "../../x".into();
        assert!(matches!(
            acquire(&http, &dir, DllType::Dlss, &bad, true, None, &mut |_, _| {}),
            Err(AcquireError::Version(_))
        ));
        assert_eq!(http.requests().len(), 2, "no request for a bad version");
    }
}
