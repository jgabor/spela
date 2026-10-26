//! The DLL commands, run off the UI thread: the Catalog's reads, the
//! manifest fetch, a game's backup read, the DLL jobs and the release
//! check.
//!
//! - The Catalog's load reads the manifest cache (`manifest-v1.json`) and
//!   lists each family's folder in the DLL cache. The refresh fetches the
//!   manifest under its refresh policy first. Both read the config fresh.
//! - A DLL job ([`job`]) reads the config fresh, gets the manifest under
//!   its policy (a restore needs none), runs the core operation with the
//!   shared stop flag, and answers with the game cache as it is after. It
//!   sends one progress message per stage, not one per downloaded chunk.
//! - The release check asks only in a release build, and only from the TUI
//!   (decision 39). The demo asks its fake, as if it were a release.

use std::collections::BTreeMap;
use std::sync::atomic::Ordering;

use spela_core::denylist;
use spela_core::dll::auto::{self, AutoUpdate};
use spela_core::dll::backup;
use spela_core::dll::detect::CATALOGUE;
use spela_core::dll::fetch;
use spela_core::dll::manifest::{self, RefreshPolicy};
use spela_core::dll::ops::{self, DllContext, FailKind, OpError, ProgressEvent};
use spela_core::games::cache;
use spela_core::paths::Paths;
use spela_core::release;
use spela_ui::Msg;
use spela_ui::dll_jobs::{DllDone, DllOp, DllResult};
use spela_ui::screens::dlls::{Catalog, Cmd, Msg as DllsMsg};

use super::Workers;

fn config(paths: &Paths) -> spela_core::config::Config {
    spela_core::config::load(&paths.config_file())
        .map(|l| l.config)
        .unwrap_or_default()
}

fn cached_versions(
    paths: &Paths,
    config: &spela_core::config::Config,
) -> BTreeMap<spela_core::games::DllType, Result<Vec<String>, String>> {
    let dir = config.dll_cache_dir(paths);
    CATALOGUE
        .iter()
        .map(|f| {
            let versions = manifest::cached_versions(&dir, f.dll_type).map_err(|e| {
                format!(
                    "cannot read {}: {e}",
                    manifest::cache_dir(&dir, f.dll_type).display()
                )
            });
            (f.dll_type, versions)
        })
        .collect()
}

/// Reads what the Catalog shows: the manifest cache and the cached
/// versions of every family.
pub fn read_catalog(paths: &Paths) -> Catalog {
    let config = config(paths);
    Catalog {
        manifest: manifest::load_cache(paths).map_err(|e| e.to_string()),
        cached: cached_versions(paths, &config),
        note: None,
    }
}

/// Fetches the manifest under the config's refresh policy, or at once when
/// `force`, then reads the cached versions. A fetch that fails falls back
/// to the cache, with a note; with no cache, the manifest is the error.
pub fn refresh_catalog(
    paths: &Paths,
    http: &dyn spela_core::backend::Http,
    clock: &dyn spela_core::clock::Clock,
    force: bool,
) -> Catalog {
    let config = config(paths);
    let policy = RefreshPolicy::from_config(&config);
    let (manifest, note) = match fetch::get_manifest(paths, &policy, http, clock, force) {
        Ok(source) => (Ok(Some(source.cached)), source.warning),
        Err(e) => (Err(e.to_string()), None),
    };
    if let Some(note) = &note {
        tracing::warn!("{note}");
    }
    Catalog {
        manifest,
        cached: cached_versions(paths, &config),
        note,
    }
}

/// What a restore of the game would change, from its backup index: empty
/// when it has none.
pub fn read_backup(paths: &Paths, app_id: u64) -> Result<Vec<backup::RestoreTarget>, String> {
    match backup::load(paths, app_id) {
        Ok(Some(index)) => Ok(index.restore_targets()),
        Ok(None) => Ok(Vec::new()),
        Err(e) => Err(e.to_string()),
    }
}

/// Runs one of the Catalog's commands.
pub fn run(w: &mut Workers, cmd: Cmd) {
    let paths = w.paths.clone();
    match cmd {
        Cmd::Load { request } => w.jobs.submit(
            &w.tx,
            move || {
                Msg::Dlls(DllsMsg::Loaded {
                    request,
                    result: Ok(read_catalog(&paths)),
                })
            },
            move |e| {
                Msg::Dlls(DllsMsg::Loaded {
                    request,
                    result: Err(e),
                })
            },
        ),
        Cmd::Refresh { request, force } => {
            let http = w.http.clone();
            let clock = w.clock.clone();
            w.jobs.submit(
                &w.tx,
                move || {
                    Msg::Dlls(DllsMsg::Refreshed {
                        request,
                        result: Ok(refresh_catalog(
                            &paths,
                            http.as_ref(),
                            clock.as_ref(),
                            force,
                        )),
                    })
                },
                move |e| {
                    Msg::Dlls(DllsMsg::Refreshed {
                        request,
                        result: Err(e),
                    })
                },
            );
        }
        Cmd::Backup { request, app_id } => w.jobs.submit(
            &w.tx,
            move || {
                Msg::Dlls(DllsMsg::Backup {
                    request,
                    app_id,
                    result: read_backup(&paths, app_id),
                })
            },
            move |e| {
                Msg::Dlls(DllsMsg::Backup {
                    request,
                    app_id,
                    result: Err(e),
                })
            },
        ),
    }
}

/// The manifest for an install or update, under the refresh policy.
fn manifest_for(
    paths: &Paths,
    config: &spela_core::config::Config,
    http: &dyn spela_core::backend::Http,
    clock: &dyn spela_core::clock::Clock,
) -> Result<spela_core::dll::manifest::Manifest, String> {
    let source = fetch::get_manifest(
        paths,
        &RefreshPolicy::from_config(config),
        http,
        clock,
        false,
    )
    .map_err(|e| e.to_string())?;
    if let Some(w) = &source.warning {
        tracing::warn!("{w}");
    }
    Ok(source.cached.manifest)
}

/// Runs a DLL job on the pool.
pub fn job(w: &mut Workers, token: u64, op: DllOp) {
    let paths = w.paths.clone();
    let http = w.http.clone();
    let clock = w.clock.clone();
    let stop = w.stop.clone();
    let proc_root = w.proc_root.clone();
    let tx = w.tx.clone();
    w.jobs.submit(
        &w.tx,
        move || {
            let config = config(&paths);
            let cx = DllContext {
                paths: &paths,
                config: &config,
                http: http.as_ref(),
                clock: clock.as_ref(),
                stop: Some(&stop),
            };
            // One message per stage: a download reports every chunk.
            let mut last = None;
            let mut progress = |e: &ProgressEvent| {
                let key = (e.stage, e.app_id, e.dll_type, e.version.clone());
                if last.as_ref() != Some(&key) {
                    last = Some(key);
                    let _ = tx.done(Msg::DllProgress {
                        token,
                        event: e.clone(),
                    });
                }
            };
            let result = run_op(&cx, op, proc_root.as_deref(), &mut progress);
            if stop.load(Ordering::SeqCst) {
                tracing::info!("a DLL job finished its step before spela quit");
            }
            Msg::DllDone {
                token,
                done: Box::new(DllDone {
                    result,
                    games: Some(cache::load(&paths)),
                }),
            }
        },
        move |e| Msg::DllDone {
            token,
            done: Box::new(DllDone {
                result: DllResult::Failed(e),
                games: None,
            }),
        },
    );
}

fn run_op(
    cx: &DllContext<'_>,
    op: DllOp,
    proc_root: Option<&std::path::Path>,
    progress: &mut dyn FnMut(&ProgressEvent),
) -> DllResult {
    let manifest = || manifest_for(cx.paths, cx.config, cx.http, cx.clock);
    match op {
        DllOp::Install {
            app_id,
            dll_type,
            version,
        } => DllResult::Installed(match manifest() {
            Ok(m) => ops::install(cx, &m, app_id, dll_type, Some(&version), progress),
            Err(message) => Err(OpError::Failed {
                kind: FailKind::Download,
                message,
            }),
        }),
        DllOp::Update { requests } => {
            DllResult::Updated(manifest().map(|m| ops::update(cx, &m, requests, progress)))
        }
        DllOp::Restore { app_id, expected } => {
            // The confirmation showed `expected`; a backup that moved since
            // asks again and changes nothing.
            match read_backup(cx.paths, app_id) {
                Ok(now) if now == expected => {
                    DllResult::Restored(ops::restore(cx, app_id, progress))
                }
                now => DllResult::BackupChanged(now),
            }
        }
        DllOp::AutoUpdate => {
            let m = match manifest() {
                Ok(m) => m,
                Err(e) => return DllResult::Auto(AutoUpdate::skipped(e)),
            };
            let lists = match denylist::load(cx.paths) {
                Ok(l) => l,
                Err(e) => return DllResult::Auto(AutoUpdate::skipped(e.to_string())),
            };
            let games = cache::load(cx.paths);
            let mut busy = |candidates: &[&spela_core::games::Game]| match proc_root {
                Some(root) => {
                    let games: Vec<_> = candidates.iter().map(|g| (*g).clone()).collect();
                    spela_core::steam::detect::running_games(root, std::process::id(), &games)
                }
                None => Default::default(),
            };
            DllResult::Auto(auto::auto_update(
                cx, &games, &m, &lists, &mut busy, progress,
            ))
        }
    }
}

/// The release check: `spela vX is available` when a newer release is
/// tagged. A failed lookup stays quiet, as in Go, and is logged.
pub fn check_release(w: &mut Workers) {
    let paths = w.paths.clone();
    let http = w.http.clone();
    let clock = w.clock.clone();
    let current = if w.demo {
        // The demo's fake answers as for a release build.
        spela_core::VERSION.to_owned()
    } else {
        release::running_version()
    };
    w.jobs.submit(
        &w.tx,
        move || {
            let checked = release::check(
                &paths,
                http.as_ref(),
                clock.as_ref(),
                &current,
                release::TAGS_URL,
            );
            if let Some(e) = &checked.error {
                tracing::info!("release check: {e}");
            }
            Msg::ReleaseNotice(checked.notice.map(|n| n.text()))
        },
        |e| {
            tracing::warn!("release check: {e}");
            Msg::ReleaseNotice(None)
        },
    );
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::games::DllType;

    #[test]
    fn reads_the_manifest_cache_and_the_cached_versions() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let empty = read_catalog(&paths);
        assert_eq!(empty.manifest, Ok(None));
        assert_eq!(empty.cached[&DllType::Dlss], Ok(Vec::new()));

        let config = spela_core::config::Config::default();
        let dlss = manifest::cache_dir(&config.dll_cache_dir(&paths), DllType::Dlss);
        std::fs::create_dir_all(&dlss).unwrap();
        for v in ["310.4.0", "310.5.3", "not-a-version"] {
            std::fs::write(dlss.join(format!("{v}.dll")), b"x").unwrap();
        }
        let cache = paths.manifest_cache();
        std::fs::create_dir_all(cache.parent().unwrap()).unwrap();
        std::fs::write(
            &cache,
            r#"{"version":"1","dlls":{"dlss":[{"version":"310.5.3","filename":"nvngx_dlss.dll"}]}}"#,
        )
        .unwrap();
        let c = read_catalog(&paths);
        assert_eq!(
            c.cached[&DllType::Dlss],
            Ok(vec!["310.5.3".to_owned(), "310.4.0".to_owned()])
        );
        let m = c.manifest.unwrap().unwrap();
        assert_eq!(m.manifest.latest(DllType::Dlss).unwrap().version, "310.5.3");

        std::fs::write(&cache, "{").unwrap();
        assert!(read_catalog(&paths).manifest.is_err());
    }

    #[test]
    fn the_refresh_fetches_and_falls_back_with_a_note() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let clock = spela_core::clock::ManualClock::new("2026-10-26T17:00:00Z".parse().unwrap());
        let none = spela_core::fake::FakeHttp::new();
        let c = refresh_catalog(&paths, &none, &clock, false);
        assert!(
            c.manifest
                .as_ref()
                .unwrap_err()
                .starts_with("failed to fetch the DLL manifest"),
            "{:?}",
            c.manifest
        );
        let demo = spela_core::fake::dlls::http(manifest::DEFAULT_MANIFEST_URL);
        let c = refresh_catalog(&paths, &demo, &clock, false);
        let m = c.manifest.unwrap().unwrap();
        assert_eq!(m.manifest.latest(DllType::Dlss).unwrap().version, "310.5.3");
        assert!(paths.manifest_cache().is_file(), "saved for the next start");
        // Stale and unreachable: the cache stands in, with a note.
        clock.advance(jiff::SignedDuration::from_hours(48));
        let c = refresh_catalog(&paths, &none, &clock, false);
        assert!(c.manifest.unwrap().is_some());
        assert!(c.note.unwrap().starts_with("using the cached DLL manifest"));
    }

    #[test]
    fn a_backup_reads_as_its_restore_targets() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        assert_eq!(read_backup(&paths, 9100110), Ok(Vec::new()));
        let index = paths.backup_index(9100110);
        std::fs::create_dir_all(index.parent().unwrap()).unwrap();
        std::fs::write(&index, "{").unwrap();
        assert!(read_backup(&paths, 9100110).is_err());
    }

    #[test]
    fn the_demo_flows_run_end_to_end_on_the_demo_fake() {
        use spela_core::clock::ManualClock;
        use spela_core::games::DllType;
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(&base.path().join("xdg"));
        let install = spela_core::fake::library::write(&base.path().join("steam")).unwrap();
        let clock = ManualClock::new("2026-10-26T17:00:00Z".parse().unwrap());
        spela_core::games::scan::rescan(&paths, &install, &clock).unwrap();
        let config = spela_core::config::Config::default();
        let http = spela_core::fake::dlls::http(manifest::DEFAULT_MANIFEST_URL);
        let cx = DllContext {
            paths: &paths,
            config: &config,
            http: &http,
            clock: &clock,
            stop: None,
        };
        let version = |app_id: u64, t: DllType| {
            cache::load(&paths)
                .get(app_id)
                .unwrap()
                .dlls
                .iter()
                .find(|d| d.dll_type == t)
                .unwrap()
                .version
                .clone()
        };
        // The auto-update brings every stale demo DLL to the latest; the
        // one with no readable version stays.
        let DllResult::Auto(a) = run_op(&cx, DllOp::AutoUpdate, None, &mut |_| {}) else {
            panic!()
        };
        assert_eq!((a.batch.updated, a.batch.failed), (8, 0), "{a:?}");
        assert_eq!(version(9100110, DllType::Dlss).as_deref(), Some("310.5.3"));
        assert_eq!(version(9100150, DllType::Xess).as_deref(), Some("2.0.1"));
        assert_eq!(version(9100170, DllType::Dlss), None);

        // Restore puts the original back.
        let expected = read_backup(&paths, 9100110).unwrap();
        assert_eq!(expected.len(), 2, "both DLLs were backed up");
        let DllResult::Restored(r) = run_op(
            &cx,
            DllOp::Restore {
                app_id: 9100110,
                expected,
            },
            None,
            &mut |_| {},
        ) else {
            panic!()
        };
        r.unwrap();
        assert_eq!(version(9100110, DllType::Dlss).as_deref(), Some("310.2.1"));

        // Install pins an older version.
        let DllResult::Installed(i) = run_op(
            &cx,
            DllOp::Install {
                app_id: 9100130,
                dll_type: DllType::Dlss,
                version: "310.2.1".into(),
            },
            None,
            &mut |_| {},
        ) else {
            panic!()
        };
        i.unwrap();
        assert_eq!(version(9100130, DllType::Dlss).as_deref(), Some("310.2.1"));

        // A backup that moved since the confirmation asks again.
        let DllResult::BackupChanged(now) = run_op(
            &cx,
            DllOp::Restore {
                app_id: 9100130,
                expected: Vec::new(),
            },
            None,
            &mut |_| {},
        ) else {
            panic!()
        };
        assert_eq!(now.unwrap().len(), 1);
        assert_eq!(version(9100130, DllType::Dlss).as_deref(), Some("310.2.1"));
        for url in http.urls() {
            assert!(
                url.contains(".invalid/") || url.starts_with(manifest::DEFAULT_MANIFEST_URL),
                "{url}"
            );
        }
    }
}
