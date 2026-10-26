//! The demo's DLL world: a manifest, payloads whose SHA-256 matches it, and
//! a tag list for the release check, all served by [`FakeHttp`] under
//! `spela --demo` only (decision 18). Every payload is a [`super::pe`] stub
//! with the entry's version, so a swap shows the new version after the
//! rescan. The hosts are under `.invalid`, so nothing here could reach a
//! real server even outside the fake.

use crate::dll::manifest::{DEFAULT_MANIFEST_URL, Manifest, ManifestEntry};
use crate::games::DllType;
use crate::hex;

use super::http::FakeHttp;

/// Where the demo's payloads live.
pub const HOST: &str = "https://dlls.spela-demo.invalid";

/// The demo manifest's entries: family, version and file name. DLSS has an
/// older release too, so Install can pin one.
pub const ENTRIES: &[(DllType, &str, &str)] = &[
    (DllType::Dlss, "310.5.3", "nvngx_dlss.dll"),
    (DllType::Dlss, "310.4.0", "nvngx_dlss.dll"),
    (DllType::Dlss, "310.2.1", "nvngx_dlss.dll"),
    (DllType::Dlssg, "310.5.3", "nvngx_dlssg.dll"),
    (DllType::Dlssd, "310.5.3", "nvngx_dlssd.dll"),
    (DllType::Xess, "2.0.1", "libxess.dll"),
    (DllType::Xess, "1.3.1", "libxess.dll"),
    (DllType::Fsr, "1.1.0", "amd_fidelityfx_vk.dll"),
];

/// The tags the demo's release check reads: a DLL tag, this version and
/// the next patch release, so the demo shows the release notice.
pub fn tags() -> String {
    let current = crate::VERSION;
    let mut parts: Vec<u64> = current.split('.').filter_map(|p| p.parse().ok()).collect();
    parts.resize(3, 0);
    parts[2] += 1;
    format!(
        r#"[{{"name":"dll-2026-10-20"}},{{"name":"v{}.{}.{}"}},{{"name":"v{current}"}}]"#,
        parts[0], parts[1], parts[2]
    )
}

/// A payload's bytes.
pub fn payload(version: &str) -> Vec<u8> {
    super::pe::stub(version)
}

fn url(dll_type: DllType, version: &str, filename: &str) -> String {
    format!("{HOST}/{dll_type}/{version}/{filename}")
}

/// The demo manifest, with each payload's URL, size and SHA-256.
pub fn manifest() -> Manifest {
    let mut m = Manifest {
        version: "1".into(),
        repository: "spela demo".into(),
        ..Manifest::default()
    };
    for (dll_type, version, filename) in ENTRIES {
        let body = payload(version);
        m.dlls
            .entry(dll_type.as_str().to_owned())
            .or_default()
            .push(ManifestEntry {
                version: (*version).to_owned(),
                filename: (*filename).to_owned(),
                url: url(*dll_type, version, filename),
                sha256: hex::sha256_hex(&body),
                size: body.len() as i64,
                notes: "spela demo stub, not a real DLL".into(),
                ..ManifestEntry::default()
            });
    }
    m
}

/// A fake that serves the demo manifest at `manifest_url` and at the
/// default URL, every payload, and [`tags`] at the release check's URL.
pub fn http(manifest_url: &str) -> FakeHttp {
    let fake = FakeHttp::new();
    let text = serde_json::to_string(&manifest()).unwrap_or_default();
    fake.serve(DEFAULT_MANIFEST_URL, text.clone());
    fake.serve(manifest_url, text);
    for (dll_type, version, filename) in ENTRIES {
        fake.serve(&url(*dll_type, version, filename), payload(version));
    }
    fake.serve(crate::release::TAGS_URL, tags());
    fake
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::backend::http::Http;
    use crate::dll::fetch;

    fn serves(http: &dyn Http, url: &str) -> bool {
        let Ok(uri) = url.parse() else {
            return false;
        };
        let request = crate::backend::http::HttpRequest::new(uri);
        http.get(&request, &mut std::io::sink(), &mut |_, _| {})
            .is_ok()
    }

    #[test]
    fn every_entry_checks_and_its_payload_matches() {
        let m = manifest();
        for (dll_type, _, _) in ENTRIES {
            for e in m.entries(*dll_type) {
                e.check(*dll_type).unwrap();
            }
        }
        assert_eq!(m.latest(DllType::Dlss).unwrap().version, "310.5.3");
        assert_eq!(m.latest(DllType::Xess).unwrap().version, "2.0.1");
        let fake = http("http://127.0.0.1:9/custom.json");
        let dir = tempfile::tempdir().unwrap();
        let paths = crate::paths::Paths::under(dir.path());
        let clock = crate::clock::ManualClock::new("2026-10-26T17:00:00Z".parse().unwrap());
        let checked = crate::release::check(
            &paths,
            &fake,
            &clock,
            crate::VERSION,
            crate::release::TAGS_URL,
        );
        let notice = checked.notice.expect("the demo has a newer release").text();
        assert!(
            notice.starts_with("spela v") && notice.ends_with(" is available"),
            "{notice}"
        );
        assert!(serves(&fake, DEFAULT_MANIFEST_URL));
        assert!(serves(&fake, "http://127.0.0.1:9/custom.json"));
        assert!(!serves(&fake, "https://example.invalid/other"));
        let dir = tempfile::tempdir().unwrap();
        for (dll_type, version, _) in ENTRIES {
            let entry = m
                .entries(*dll_type)
                .iter()
                .find(|e| e.version == *version)
                .unwrap();
            let path = fetch::acquire(
                &fake,
                dir.path(),
                *dll_type,
                entry,
                true,
                None,
                &mut |_, _| {},
            )
            .unwrap();
            let read = crate::pe::version_string(&path).unwrap();
            assert!(
                crate::dll::version::compare(&read, version).is_eq(),
                "{read} {version}"
            );
        }
    }
}
