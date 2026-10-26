//! Reading and writing `<cache>/games.json`.
//!
//! Readers never lock: every write replaces the file atomically. A write is
//! a transaction under `games.lock` that loads the file fresh and saves only
//! on change, as Go's `Transaction` does. A file that does not parse counts
//! as empty, so the next scan rebuilds it, and the problem is logged.

use std::io;

use crate::fs::{Durability, write_atomic};
use crate::lock::StoreLock;
use crate::paths::Paths;

use super::GameCache;

/// The cache, or an empty one when the file is missing, unreadable or does
/// not parse. Never fails: the cache is rebuildable.
pub fn load(paths: &Paths) -> GameCache {
    let path = paths.game_cache();
    match std::fs::read_to_string(&path) {
        Ok(text) => GameCache::from_json(&text).unwrap_or_else(|e| {
            tracing::warn!(path = %path.display(), "the game cache does not parse, so it counts as empty: {e}");
            GameCache::default()
        }),
        Err(e) if e.kind() == io::ErrorKind::NotFound => GameCache::default(),
        Err(e) => {
            tracing::warn!(path = %path.display(), "cannot read the game cache, so it counts as empty: {e}");
            GameCache::default()
        }
    }
}

/// Takes `games.lock`, for a caller that loads, changes and [`save`]s the
/// cache itself, such as a DLL swap.
pub fn lock(paths: &Paths) -> Result<StoreLock, crate::lock::LockError> {
    StoreLock::acquire(&paths.games_lock())
}

/// Writes the cache atomically. The caller holds [`lock`].
pub fn save(paths: &Paths, cache: &GameCache) -> io::Result<()> {
    let text = cache.to_json().map_err(io::Error::other)?;
    std::fs::create_dir_all(&paths.cache_dir)?;
    write_atomic(&paths.game_cache(), text.as_bytes(), Durability::Cache)
}

/// Changes the cache under `games.lock`. `change` gets the cache as it is on
/// disk now; the file is written only when the cache changed. Returns what
/// `change` returns.
pub fn update<T>(paths: &Paths, change: impl FnOnce(&mut GameCache) -> T) -> io::Result<T> {
    let _lock = lock(paths).map_err(io::Error::other)?;
    let before = load(paths);
    let mut cache = before.clone();
    let result = change(&mut cache);
    if cache != before {
        save(paths, &cache)?;
    }
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::games::Game;

    #[test]
    fn missing_and_broken_files_count_as_empty() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        assert!(load(&paths).is_empty());
        std::fs::create_dir_all(&paths.cache_dir).unwrap();
        std::fs::write(paths.game_cache(), "{not json").unwrap();
        assert!(load(&paths).is_empty());
    }

    #[test]
    fn update_writes_only_on_change() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        update(&paths, |_| ()).unwrap();
        assert!(!paths.game_cache().exists(), "no change, no file");
        update(&paths, |c| {
            c.upsert(Game {
                app_id: 9100010,
                name: "Chrome Avenue 2081".into(),
                install_dir: "/lib/a".into(),
                library_path: "/lib".into(),
                prefix_path: None,
                scanned_at: "2026-10-26T15:02:10Z".parse().unwrap(),
                dlls: Vec::new(),
            });
        })
        .unwrap();
        assert_eq!(
            load(&paths).find("9100010").unwrap().name,
            "Chrome Avenue 2081"
        );
    }
}
