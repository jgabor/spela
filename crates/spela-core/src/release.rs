//! The release check: is a newer spela tagged on GitHub? Go's
//! `internal/release` (parity inventory DLL 5).
//!
//! - It asks GitHub's tags API with `Accept: application/vnd.github+json`, a
//!   5 s timeout and a 1 MiB body cap, and picks the highest tag that looks
//!   like `vX.Y.Z` by the shared version compare.
//! - The `dll-*` tags share the namespace, so the newest release can fall off
//!   the first page. The check follows the `Link` header's `next` page, on
//!   the same host, up to [`MAX_PAGES`], until a page holds a release tag.
//!   Go read only the first page.
//! - It asks at most once a day, and again when the stored time is in the
//!   future. Every attempt is recorded in `<cache>/release-check.json`; only
//!   a success changes `latest`. A state file that cannot be read counts as
//!   never checked.
//! - Only a release or packaged build checks: one whose version came
//!   through `SPELA_VERSION` at build time, from release CI as `X.Y.Z` or
//!   from the AUR `spela-git` package as its pkgver, such as
//!   `0.7.0.r6.g087d1c4` (decision 68). A packaged build compares its
//!   release part, `0.7.0`, with the tags. Every other build reports
//!   `<crate version>-dev` and never asks. Go skipped `-git` builds.
//! - Only the TUI checks, gated by `check_updates`; the CLI stays quiet
//!   (decision 39). The URL is a parameter, so tests point it at the fake.

use std::io;
use std::sync::LazyLock;

use jiff::{SignedDuration, Timestamp};
use regex_lite::Regex;
use serde::{Deserialize, Serialize};

use crate::backend::http::{Http, HttpRequest, RELEASE_BODY_CAP, RELEASE_TIMEOUT};
use crate::clock::Clock;
use crate::dll::version;
use crate::fs::{Durability, write_atomic};
use crate::paths::Paths;

/// GitHub's tag list for spela, Go's URL.
pub const TAGS_URL: &str = "https://api.github.com/repos/jgabor/spela/tags?per_page=100";

/// The `Accept` header GitHub's API asks for.
pub const ACCEPT: &str = "application/vnd.github+json";

/// The time between two attempts, successful or not.
pub const CHECK_INTERVAL: SignedDuration = SignedDuration::from_hours(24);

/// The most tag pages one check reads.
pub const MAX_PAGES: usize = 5;

static RELEASE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^v?\d+\.\d+\.\d+$").expect("the release pattern compiles"));

/// An AUR VCS pkgver: the tag's release, the commits since it and the
/// commit, as `pkg/aur/PKGBUILD`'s `pkgver()` writes it.
static PACKAGED: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^(v?\d+\.\d+\.\d+)\.r\d+\.g[0-9a-f]+$").expect("the pkgver pattern compiles")
});

/// Whether `version` is a plain release, `1.2.3` or `v1.2.3`. Development
/// builds and `git describe` output are not.
pub fn is_release(version: &str) -> bool {
    RELEASE.is_match(version)
}

/// The release a version is built from: a plain release as it is, and a
/// `spela-git` pkgver such as `0.7.0.r6.g087d1c4` as `0.7.0` (decision 68).
/// `None` for every other version, which never checks.
pub fn release_base(version: &str) -> Option<&str> {
    if is_release(version) {
        return Some(version);
    }
    PACKAGED
        .captures(version)
        .and_then(|c| c.get(1))
        .map(|m| m.as_str())
}

/// The running version: the version release CI or the AUR package built
/// in, or `<crate version>-dev`.
pub fn running_version() -> String {
    version_from(option_env!("SPELA_VERSION"))
}

/// The running version for a build-time `SPELA_VERSION` of `built`.
fn version_from(built: Option<&str>) -> String {
    match built {
        Some(v) if release_base(v).is_some() => v.to_owned(),
        _ => format!("{}-dev", crate::VERSION),
    }
}

/// A newer release than the running one.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Notice {
    /// The running version.
    pub current: String,
    /// The newest release tag.
    pub latest: String,
}

impl Notice {
    /// The status line's text: `spela vX is available`.
    pub fn text(&self) -> String {
        format!("spela {} is available", with_v(&self.latest))
    }
}

fn with_v(version: &str) -> String {
    if version.starts_with('v') {
        version.to_owned()
    } else {
        format!("v{version}")
    }
}

/// `release-check.json`, Go's `update-check.json` shape.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
struct State {
    #[serde(default, with = "go_time")]
    checked_at: Option<Timestamp>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    latest: String,
}

mod go_time {
    use jiff::Timestamp;
    use jiff::tz::TimeZone;
    use serde::{Deserialize, Deserializer, Serializer};

    pub(super) fn serialize<S: Serializer>(t: &Option<Timestamp>, s: S) -> Result<S::Ok, S::Error> {
        let text = t.map_or_else(
            || "0001-01-01T00:00:00Z".to_owned(),
            |t| crate::dll::swap::go_time_text(t, &TimeZone::system()),
        );
        s.serialize_str(&text)
    }

    pub(super) fn deserialize<'de, D: Deserializer<'de>>(
        d: D,
    ) -> Result<Option<Timestamp>, D::Error> {
        let text = Option::<String>::deserialize(d)?.unwrap_or_default();
        let t: Timestamp = text.parse().map_err(serde::de::Error::custom)?;
        // Go's zero time is "never".
        Ok((t.as_second() != -62_135_596_800).then_some(t))
    }
}

/// What a check found: a notice when a newer release is known, and the
/// lookup's error when it failed. Both can be set: the notice then comes
/// from an earlier success.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Checked {
    /// The notice, if any.
    pub notice: Option<Notice>,
    /// Why asking failed, if it did.
    pub error: Option<String>,
    /// Whether GitHub was asked this time.
    pub asked: bool,
}

/// Checks for a newer release than `current`, asking `tags_url` when the
/// last attempt is a day old or more. A build that is not a release never
/// asks.
pub fn check(
    paths: &Paths,
    http: &dyn Http,
    clock: &dyn Clock,
    current: &str,
    tags_url: &str,
) -> Checked {
    let mut out = Checked::default();
    let Some(base) = release_base(current) else {
        return out;
    };
    let mut state = load_state(paths);
    let now = clock.now();
    let due = match state.checked_at {
        None => true,
        Some(at) => at > now || now.duration_since(at) >= CHECK_INTERVAL,
    };
    if due {
        out.asked = true;
        state.checked_at = Some(now);
        match latest_tag(http, tags_url) {
            Ok(latest) => state.latest = latest,
            Err(e) => out.error = Some(e),
        }
        if let Err(e) = save_state(paths, &state)
            && out.error.is_none()
        {
            out.error = Some(format!(
                "cannot save {}: {e}",
                paths.release_check().display()
            ));
        }
    }
    if is_release(&state.latest) && version::is_newer(base, &state.latest) {
        out.notice = Some(Notice {
            current: current.to_owned(),
            latest: state.latest,
        });
    }
    out
}

fn load_state(paths: &Paths) -> State {
    std::fs::read(paths.release_check())
        .ok()
        .and_then(|bytes| serde_json::from_slice(&bytes).ok())
        .unwrap_or_default()
}

fn save_state(paths: &Paths, state: &State) -> io::Result<()> {
    let text = serde_json::to_vec(state).map_err(io::Error::other)?;
    std::fs::create_dir_all(&paths.cache_dir)?;
    write_atomic(&paths.release_check(), &text, Durability::Cache)
}

#[derive(Deserialize)]
struct Tag {
    #[serde(default)]
    name: String,
}

/// The highest release tag, reading pages until one holds a release tag.
/// The texts are Go's.
pub fn latest_tag(http: &dyn Http, tags_url: &str) -> Result<String, String> {
    let mut url = tags_url.to_owned();
    for _ in 0..MAX_PAGES {
        let uri: http::Uri = url
            .parse()
            .map_err(|_| format!("look up releases: bad URL {url:?}"))?;
        let request = HttpRequest::new(uri.clone())
            .with_accept(ACCEPT)
            .with_timeouts(RELEASE_TIMEOUT, RELEASE_TIMEOUT)
            .with_body_cap(RELEASE_BODY_CAP);
        let mut body = Vec::new();
        let response = http
            .get(&request, &mut body, &mut |_, _| {})
            .map_err(|e| format!("look up releases: {e}"))?;
        let tags: Vec<Tag> =
            serde_json::from_slice(&body).map_err(|e| format!("read releases: {e}"))?;
        let best = tags
            .into_iter()
            .map(|t| t.name)
            .filter(|n| is_release(n))
            .reduce(|best, n| {
                if version::is_newer(&best, &n) {
                    n
                } else {
                    best
                }
            });
        if let Some(best) = best {
            return Ok(best);
        }
        match response.link.as_deref().and_then(|l| next_link(l, &uri)) {
            Some(next) => url = next,
            None => break,
        }
    }
    Err("read releases: no release tags".to_owned())
}

/// The `rel="next"` URL of a `Link` header, when it is on the same scheme
/// and host as `base`.
pub fn next_link(link: &str, base: &http::Uri) -> Option<String> {
    link.split(',').find_map(|part| {
        let (target, params) = part.trim().split_once(';')?;
        let is_next = params.split(';').any(|p| {
            let p = p.trim().replace(' ', "");
            p == "rel=\"next\"" || p == "rel=next"
        });
        let url = target.trim().strip_prefix('<')?.strip_suffix('>')?;
        let uri: http::Uri = url.parse().ok()?;
        (is_next && uri.scheme() == base.scheme() && uri.authority() == base.authority())
            .then(|| url.to_owned())
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::backend::http::HttpError;
    use crate::clock::ManualClock;
    use crate::fake::{FakeHttp, FakeRoute};

    const URL: &str = "http://127.0.0.1:9/repos/jgabor/spela/tags?per_page=100";

    fn tags(names: &[&str]) -> String {
        serde_json::to_string(
            &names
                .iter()
                .map(|n| serde_json::json!({ "name": n }))
                .collect::<Vec<_>>(),
        )
        .unwrap()
    }

    fn setup() -> (tempfile::TempDir, Paths, FakeHttp, ManualClock) {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let clock = ManualClock::new("2026-10-26T15:00:00Z".parse().unwrap());
        (base, paths, FakeHttp::new(), clock)
    }

    #[test]
    fn release_versions() {
        for v in ["0.7.0", "v1.2.3", "v10.20.30"] {
            assert!(is_release(v), "{v}");
        }
        for v in [
            "dev",
            "0.7.0-dev",
            "v0.6.2-3-gabc",
            "0.7",
            "dll-dlss-v310.5.3",
            "",
        ] {
            assert!(!is_release(v), "{v}");
        }
        assert!(running_version().ends_with("-dev") || release_base(&running_version()).is_some());
    }

    #[test]
    fn packaged_versions_report_themselves_and_compare_their_release() {
        // pkg/aur's pkgver() turns `v0.7.0-6-g087d1c4` into this.
        assert_eq!(release_base("0.7.0.r6.g087d1c4"), Some("0.7.0"));
        assert_eq!(release_base("0.7.0.r0.gabcdef1"), Some("0.7.0"));
        assert_eq!(release_base("v1.2.3"), Some("v1.2.3"));
        for v in [
            "0.7.0-dev",
            "0.7.0.r6",
            "0.7.0.r6.gXYZ",
            "0.7.r6.g087d1c4",
            "dev",
        ] {
            assert_eq!(release_base(v), None, "{v}");
        }
        assert_eq!(version_from(Some("0.7.0.r6.g087d1c4")), "0.7.0.r6.g087d1c4");
        assert_eq!(version_from(Some("0.7.1")), "0.7.1");
        let dev = format!("{}-dev", crate::VERSION);
        assert_eq!(version_from(Some("0.7.0-6-g087d1c4")), dev);
        assert_eq!(version_from(None), dev);
    }

    #[test]
    fn a_packaged_build_checks_against_its_release() {
        let (_base, paths, http, clock) = setup();
        http.serve(URL, tags(&["v0.7.1", "v0.7.0"]));
        let checked = check(&paths, &http, &clock, "0.7.0.r6.g087d1c4", URL);
        assert!(checked.asked);
        assert_eq!(
            checked.notice,
            Some(Notice {
                current: "0.7.0.r6.g087d1c4".into(),
                latest: "v0.7.1".into(),
            })
        );
        // Built past the newest tag: nothing newer.
        let checked = check(&paths, &http, &clock, "0.7.1.r2.g1234567", URL);
        assert_eq!(checked.notice, None);
    }

    #[test]
    fn asks_once_a_day_and_records_every_attempt() {
        // Go's release_test.go: once a day, failures recorded, latest kept.
        let (_base, paths, http, clock) = setup();
        http.serve(
            URL,
            tags(&["v0.6.2", "dll-dlss-v310.5.3", "v0.8.1", "v0.10.0", "v0.9"]),
        );
        let got = check(&paths, &http, &clock, "0.7.0", URL);
        assert!(got.asked);
        assert_eq!(got.error, None);
        let notice = got.notice.unwrap();
        assert_eq!(notice.latest, "v0.10.0");
        assert_eq!(notice.text(), "spela v0.10.0 is available");
        let req = &http.requests()[0];
        assert_eq!(req.accept.as_deref(), Some(ACCEPT));
        assert_eq!(
            (req.total_timeout, req.body_cap),
            (RELEASE_TIMEOUT, RELEASE_BODY_CAP)
        );
        let saved = std::fs::read_to_string(paths.release_check()).unwrap();
        assert!(
            saved.starts_with(r#"{"checked_at":""#) && saved.ends_with(r#","latest":"v0.10.0"}"#),
            "{saved}"
        );

        // Within a day: the stored answer, no request.
        clock.advance(SignedDuration::from_hours(23));
        let got = check(&paths, &http, &clock, "0.7.0", URL);
        assert!(!got.asked && got.notice.is_some());
        assert_eq!(http.requests().len(), 1);

        // A day later the lookup fails: recorded, latest kept, error given.
        clock.advance(SignedDuration::from_hours(2));
        http.route(URL, FakeRoute::Fault(HttpError::Timeout));
        let got = check(&paths, &http, &clock, "0.7.0", URL);
        assert!(got.asked);
        assert_eq!(
            got.error.as_deref(),
            Some("look up releases: the request timed out")
        );
        assert_eq!(got.notice.unwrap().latest, "v0.10.0");
        clock.advance(SignedDuration::from_hours(1));
        assert!(
            !check(&paths, &http, &clock, "0.7.0", URL).asked,
            "the failure counts"
        );

        // A stored time in the future asks again.
        clock.set("2026-10-20T00:00:00Z".parse().unwrap());
        assert!(check(&paths, &http, &clock, "0.7.0", URL).asked);
    }

    #[test]
    fn dev_builds_never_ask_and_current_ones_get_no_notice() {
        let (_base, paths, http, clock) = setup();
        http.serve(URL, tags(&["v0.7.0"]));
        assert_eq!(
            check(&paths, &http, &clock, "0.7.0-dev", URL),
            Checked::default()
        );
        assert!(http.requests().is_empty());
        assert!(!paths.release_check().exists());
        let got = check(&paths, &http, &clock, "v0.7.0", URL);
        assert!(got.asked && got.notice.is_none());
    }

    #[test]
    fn unusable_tag_lists_are_errors() {
        let (_base, paths, http, clock) = setup();
        http.serve(URL, tags(&["dll-dlss-v310.5.3", "nightly"]));
        let got = check(&paths, &http, &clock, "0.7.0", URL);
        assert_eq!(got.error.as_deref(), Some("read releases: no release tags"));
        http.serve(URL, "{");
        clock.advance(SignedDuration::from_hours(25));
        let got = check(&paths, &http, &clock, "0.7.0", URL);
        assert!(got.error.unwrap().starts_with("read releases: "));
        http.route(
            URL,
            FakeRoute::Body {
                status: 403,
                body: Vec::new(),
                link: None,
            },
        );
        assert_eq!(
            latest_tag(&http, URL),
            Err("look up releases: the server answered HTTP 403".into())
        );
    }

    #[test]
    fn follows_next_pages_on_the_same_host_until_a_release() {
        let (_base, _paths, http, _clock) = setup();
        let page2 = "http://127.0.0.1:9/repos/jgabor/spela/tags?per_page=100&page=2";
        http.serve_with_link(
            URL,
            tags(&["dll-dlss-v310.5.3", "dll-dlssg-v310.5.3"]),
            &format!(r#"<{page2}>; rel="next", <http://127.0.0.1:9/last>; rel="last""#),
        );
        http.serve(page2, tags(&["v0.6.2", "v0.7.1"]));
        assert_eq!(latest_tag(&http, URL), Ok("v0.7.1".into()));
        assert_eq!(http.urls(), [URL, page2]);

        // Another host in the header is never followed.
        let other = FakeHttp::new();
        other.serve_with_link(URL, tags(&[]), r#"<https://example.invalid/x>; rel="next""#);
        assert_eq!(
            latest_tag(&other, URL),
            Err("read releases: no release tags".into())
        );
        assert_eq!(other.requests().len(), 1);

        // At most MAX_PAGES pages.
        let looping = FakeHttp::new();
        looping.serve_with_link(URL, tags(&[]), &format!(r#"<{URL}>; rel="next""#));
        assert!(latest_tag(&looping, URL).is_err());
        assert_eq!(looping.requests().len(), MAX_PAGES);
    }

    #[test]
    fn reads_go_s_state_file() {
        let (_base, paths, http, clock) = setup();
        std::fs::create_dir_all(&paths.cache_dir).unwrap();
        std::fs::write(
            paths.release_check(),
            r#"{"checked_at":"2026-10-26T14:02:10.123456789+02:00","latest":"v0.8.0"}"#,
        )
        .unwrap();
        let got = check(&paths, &http, &clock, "0.7.0", URL);
        assert!(!got.asked, "checked an hour ago");
        assert_eq!(got.notice.unwrap().latest, "v0.8.0");
        std::fs::write(paths.release_check(), "not json").unwrap();
        assert!(
            check(&paths, &http, &clock, "0.7.0", URL).asked,
            "unreadable is never checked"
        );
    }
}
