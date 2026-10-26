//! The DLL Catalog's commands, run off the UI thread.
//!
//! The load reads the manifest cache (`manifest-v1.json`) and lists each
//! family's folder in the DLL cache, on the job pool, and answers with
//! `Msg::Dlls`. It reads the config fresh for the DLL cache folder, as the
//! scan does for the Steam paths. It fetches nothing and writes nothing.

use std::collections::BTreeMap;

use spela_core::dll::detect::CATALOGUE;
use spela_core::dll::manifest;
use spela_core::paths::Paths;
use spela_ui::Msg;
use spela_ui::screens::dlls::{Catalog, Cmd, Msg as DllsMsg};

use super::Workers;

/// Reads what the Catalog shows: the manifest cache and the cached
/// versions of every family.
pub fn read_catalog(paths: &Paths) -> Catalog {
    let config = spela_core::config::load(&paths.config_file())
        .map(|l| l.config)
        .unwrap_or_default();
    let dir = config.dll_cache_dir(paths);
    let cached: BTreeMap<_, _> = CATALOGUE
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
        .collect();
    Catalog {
        manifest: manifest::load_cache(paths).map_err(|e| e.to_string()),
        cached,
    }
}

/// Runs one of the Catalog's commands.
pub fn run(w: &mut Workers, cmd: Cmd) {
    match cmd {
        Cmd::Load { request } => {
            let paths = w.paths.clone();
            w.jobs.submit(
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
            );
        }
    }
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
}
