//! Changing DLL files on disk: the backup before a swap, the swap itself and
//! the containment check every write passes. Go's `internal/dll/swap.go`.
//!
//! The backup ([`prepare_backup`], then [`Prepared::commit`]):
//!
//! - The index is loaded first. One that does not load fails closed, so
//!   nothing changes (Go's `CreateBackup`).
//! - Every existing backup file must still be a regular file.
//! - Every DLL of the game that has no entry yet is copied to
//!   `<backup files dir>/<appid>/<sha256 of the path>-<basename>`, synced, and
//!   added at the end of `files`. A file already there without an entry is
//!   never overwritten. The first backup of a path is kept.
//! - Unless Steam replaced it (decision 37): when the file on disk matches
//!   neither its backup nor the hash spela recorded installing there, its
//!   backup file is refreshed in place with the file Steam shipped. Only an
//!   index with an `installed` record can say that, so a Go-era entry is
//!   never refreshed.
//! - The install records its target in `installed`, and in `created_files`
//!   when no file was there. The index is written, durably, before the
//!   game's DLL is touched. If the commit fails, the new backup files go.
//!
//! The swap ([`replace_file`]) writes a temp file in the target's folder,
//! syncs it, renames it over the target and syncs the folder. It never
//! follows a symlink at the target: a link is refused, so a write cannot
//! leave the game folder through it. [`check_inside`] refuses any target
//! outside the game's install folder, by path and by its real folder.

use std::io::{self, Write};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};

use jiff::Timestamp;
use jiff::tz::TimeZone;

use super::backup::{self, BackedUpFile, BackupError, BackupIndex, clean, hashed_backup_name};
use crate::fs::{Durability, NEW_FILE_MODE, write_atomic};
use crate::games::DetectedDll;
use crate::hex;
use crate::paths::Paths;

/// A backup or swap step that failed before or while changing a file.
#[derive(Debug, thiserror::Error)]
pub enum SwapError {
    /// The index does not load, so spela does not know what is backed up.
    #[error("load existing backup metadata: {0}")]
    Index(#[from] BackupError),
    /// A file cannot be backed up or a backup is unusable. Go's texts.
    #[error("{0}")]
    Backup(String),
    /// The index cannot be written.
    #[error("failed to save backup metadata: {0}")]
    Save(io::Error),
}

/// What [`prepare_backup`] did, ready to write.
#[derive(Debug)]
pub struct Prepared {
    /// The index with the new entries. The caller adds `installed` and
    /// `created_files` before [`Prepared::commit`].
    pub index: BackupIndex,
    /// Backup files this step created, removed again if the commit fails.
    new_files: Vec<PathBuf>,
    /// Entries whose backup Steam made stale and that were refreshed.
    pub refreshed: Vec<PathBuf>,
}

impl Prepared {
    /// Writes the index durably. On failure the backup files this step made
    /// are removed, as Go removes them.
    pub fn commit(&self, paths: &Paths) -> Result<(), SwapError> {
        save_index(paths, &self.index).map_err(|e| {
            remove_files(&self.new_files);
            SwapError::Save(e)
        })
    }
}

/// Backs up every DLL in `dlls` that has no backup yet, and refreshes the
/// backups Steam made stale. Writes backup files, not the index.
/// `files_dir` is [`crate::config::Config::backup_files_dir`].
pub fn prepare_backup(
    paths: &Paths,
    files_dir: &Path,
    app_id: u64,
    game_name: &str,
    dlls: &[DetectedDll],
    now: Timestamp,
) -> Result<Prepared, SwapError> {
    let dir = files_dir.join(app_id.to_string());
    let mut index = match backup::load(paths, app_id)? {
        Some(index) => index,
        None => BackupIndex {
            app_id,
            game_name: game_name.to_owned(),
            created_at: Some(go_time_text(now, &TimeZone::system())),
            backup_path: dir.clone(),
            ..BackupIndex::default()
        },
    };
    for file in &index.files {
        check_regular(&file.backup_path).map_err(|e| {
            SwapError::Backup(format!(
                "existing backup for {} is unreadable: {e}",
                file.original_path.display()
            ))
        })?;
    }
    let mut refreshed = Vec::new();
    for dll in dlls {
        let Some(entry) = index.entry(&dll.path) else {
            continue;
        };
        if index.installed_sha256(&dll.path).is_none() {
            continue;
        }
        let (Ok(current), Ok(saved)) = (sha_of(&dll.path), sha_of(&entry.backup_path)) else {
            continue;
        };
        if !index.steam_replaced(&dll.path, &current, &saved) {
            continue;
        }
        let backup_path = entry.backup_path.clone();
        copy_durable(&dll.path, &backup_path).map_err(|e| {
            SwapError::Backup(format!("failed to refresh the backup of {}: {e}", dll.name))
        })?;
        if let Some(entry) = index
            .files
            .iter_mut()
            .find(|f| clean(&f.original_path) == clean(&dll.path))
        {
            entry.version = dll.version.clone().unwrap_or_default();
        }
        tracing::info!(
            path = %dll.path.display(),
            "Steam replaced this DLL since spela installed it, so its backup now holds Steam's file"
        );
        refreshed.push(dll.path.clone());
    }
    // A created file that Steam now ships is the game's own: back it up.
    let shipped: Vec<PathBuf> = dlls
        .iter()
        .filter(|d| index.is_created(&d.path))
        .filter(|d| {
            sha_of(&d.path)
                .ok()
                .zip(index.installed_sha256(&d.path))
                .is_some_and(|(current, installed)| !same_sha(&current, installed))
        })
        .map(|d| d.path.clone())
        .collect();
    index
        .created_files
        .retain(|p| !shipped.iter().any(|s| clean(s) == clean(p)));

    // Every DLL with no entry that spela did not create is an original.
    let new: Vec<&DetectedDll> = dlls
        .iter()
        .filter(|d| index.entry(&d.path).is_none() && !index.is_created(&d.path))
        .collect();
    for dll in &new {
        check_regular(&dll.path)
            .map_err(|e| SwapError::Backup(format!("cannot backup {}: {e}", dll.name)))?;
    }
    let mut new_files = Vec::new();
    if !new.is_empty() {
        std::fs::create_dir_all(&dir)
            .map_err(|e| SwapError::Backup(format!("failed to create backup directory: {e}")))?;
    }
    for dll in new {
        let backup_path = dir.join(hashed_backup_name(&dll.path));
        match std::fs::symlink_metadata(&backup_path) {
            Ok(_) => {
                remove_files(&new_files);
                return Err(SwapError::Backup(format!(
                    "backup destination already exists without metadata: {}",
                    backup_path.display()
                )));
            }
            Err(e) if e.kind() == io::ErrorKind::NotFound => {}
            Err(e) => {
                remove_files(&new_files);
                return Err(SwapError::Backup(format!(
                    "inspect backup destination: {e}"
                )));
            }
        }
        if let Err(e) = copy_durable(&dll.path, &backup_path) {
            remove_files(&new_files);
            return Err(SwapError::Backup(format!(
                "failed to backup {}: {e}",
                dll.name
            )));
        }
        new_files.push(backup_path.clone());
        index.files.push(BackedUpFile {
            original_path: dll.path.clone(),
            backup_path,
            dll_name: dll.name.clone(),
            version: dll.version.clone().unwrap_or_default(),
        });
        // Whatever spela recorded installing there is not this original.
        index.installed.retain(|p, _| clean(p) != clean(&dll.path));
    }
    Ok(Prepared {
        index,
        new_files,
        refreshed,
    })
}

/// Records an install at `target` of a file with `sha256`: in `installed`,
/// and in `created_files` when `created`.
pub fn record_install(index: &mut BackupIndex, target: &Path, sha256: &str, created: bool) {
    index.installed.retain(|p, _| clean(p) != clean(target));
    index
        .installed
        .insert(target.to_owned(), sha256.to_ascii_lowercase());
    if created && !index.is_created(target) {
        index.created_files.push(target.to_owned());
    }
}

/// Forgets what spela installed at paths a restore put back or deleted, so
/// a later Steam file there is never taken for spela's. Returns whether the
/// index changed.
pub fn record_restore(index: &mut BackupIndex, restored: &[PathBuf], deleted: &[PathBuf]) -> bool {
    let before = (index.installed.len(), index.created_files.len());
    let touched = |p: &Path| restored.iter().chain(deleted).any(|t| clean(t) == clean(p));
    index.installed.retain(|p, _| !touched(p));
    index
        .created_files
        .retain(|p| !deleted.iter().any(|d| clean(d) == clean(p)));
    before != (index.installed.len(), index.created_files.len())
}

/// Writes `backup.json` durably in Go's layout: two-space indent, no
/// newline at the end, Go's keys first.
pub fn save_index(paths: &Paths, index: &BackupIndex) -> io::Result<()> {
    let path = paths.backup_index(index.app_id);
    if let Some(dir) = path.parent() {
        std::fs::create_dir_all(dir)?;
    }
    let text = serde_json::to_vec_pretty(index).map_err(io::Error::other)?;
    write_atomic(&path, &text, Durability::Durable)
}

/// Removes a game's index, for a restore that leaves nothing to restore.
pub fn remove_index(paths: &Paths, app_id: u64) -> io::Result<()> {
    match std::fs::remove_file(paths.backup_index(app_id)) {
        Err(e) if e.kind() != io::ErrorKind::NotFound => Err(e),
        _ => Ok(()),
    }
}

/// A time as Go's JSON writes `time.Time`: RFC 3339 with the nanoseconds
/// trimmed of trailing zeros, in the given zone, `Z` for UTC.
pub fn go_time_text(time: Timestamp, tz: &TimeZone) -> String {
    let zoned = time.to_zoned(tz.clone());
    let offset = zoned.offset().seconds();
    let mut text = zoned.strftime("%Y-%m-%dT%H:%M:%S").to_string();
    let nanos = zoned.subsec_nanosecond();
    if nanos != 0 {
        let digits = format!("{nanos:09}");
        text.push('.');
        text.push_str(digits.trim_end_matches('0'));
    }
    if offset == 0 {
        text.push('Z');
    } else {
        let sign = if offset < 0 { '-' } else { '+' };
        let abs = offset.unsigned_abs();
        text.push_str(&format!("{sign}{:02}:{:02}", abs / 3600, abs % 3600 / 60));
    }
    text
}

/// Refuses a write target outside the game's install folder: by its path,
/// as Go's `pathWithin`, and by the real folder it lands in, so a symlinked
/// folder inside the game cannot carry the write elsewhere.
pub fn check_inside(install_dir: &Path, target: &Path) -> Result<(), String> {
    let escapes = || format!("DLL target escapes install directory: {}", target.display());
    if install_dir.as_os_str().is_empty() || !install_dir.is_absolute() {
        return Err("install directory is required".to_owned());
    }
    let (root, path) = (clean(install_dir), clean(target));
    if path == root || !path.starts_with(&root) {
        return Err(escapes());
    }
    let real_root = install_dir.canonicalize().map_err(|e| {
        format!(
            "install directory {} is unavailable: {e}",
            install_dir.display()
        )
    })?;
    let parent = target.parent().unwrap_or(target);
    let real_parent = parent
        .canonicalize()
        .map_err(|e| format!("cannot reach {}: {e}", parent.display()))?;
    if !real_parent.starts_with(&real_root) {
        return Err(escapes());
    }
    Ok(())
}

/// Refuses a target that is a symlink or a folder. A missing target is
/// fine: an install may create it.
pub fn check_target(target: &Path) -> Result<(), String> {
    match std::fs::symlink_metadata(target) {
        Ok(m) if m.file_type().is_symlink() => Err(format!(
            "{} is a symbolic link; spela replaces only regular files",
            target.display()
        )),
        Ok(m) if m.is_dir() => Err(format!("{} is a directory", target.display())),
        Ok(_) => Ok(()),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(()),
        Err(e) => Err(format!("inspect {}: {e}", target.display())),
    }
}

/// Replaces `target` with a copy of `source` atomically and durably. The
/// target keeps its mode; a new one takes the source's. A target that is a
/// symlink is refused, never followed.
pub fn replace_file(source: &Path, target: &Path) -> io::Result<()> {
    check_target(target).map_err(io::Error::other)?;
    copy_durable(source, target)
}

/// Copies `source` to `target` through a synced temp file in the target's
/// folder, then syncs the folder.
fn copy_durable(source: &Path, target: &Path) -> io::Result<()> {
    let mut input = std::fs::File::open(source)?;
    let mode = match std::fs::metadata(target) {
        Ok(m) => m.permissions().mode() & 0o7777,
        Err(_) => input
            .metadata()
            .map(|m| m.permissions().mode() & 0o777)
            .unwrap_or(NEW_FILE_MODE),
    };
    let dir = target
        .parent()
        .ok_or_else(|| io::Error::other("the target has no folder"))?;
    let mut prefix = std::ffi::OsString::from(".");
    prefix.push(target.file_name().unwrap_or_default());
    prefix.push(".spela-");
    let mut tmp = tempfile::Builder::new()
        .prefix(&prefix)
        .suffix(".tmp")
        .tempfile_in(dir)?;
    io::copy(&mut input, tmp.as_file_mut())?;
    tmp.as_file_mut().flush()?;
    tmp.as_file()
        .set_permissions(std::fs::Permissions::from_mode(mode))?;
    tmp.as_file().sync_all()?;
    tmp.persist(target).map_err(|e| e.error)?;
    std::fs::File::open(dir)?.sync_all()
}

/// Go's `preflightRegularFile`.
fn check_regular(path: &Path) -> Result<(), String> {
    let file = std::fs::File::open(path).map_err(|e| format!("open {}: {e}", path.display()))?;
    let meta = file
        .metadata()
        .map_err(|e| format!("stat {}: {e}", path.display()))?;
    if meta.is_file() {
        Ok(())
    } else {
        Err(format!("{} is not a regular file", path.display()))
    }
}

/// The SHA-256 of a file's bytes.
pub fn sha_of(path: &Path) -> io::Result<String> {
    hex::sha256_reader(std::fs::File::open(path)?)
}

fn same_sha(a: &str, b: &str) -> bool {
    hex::parse_sha256(a).is_some_and(|a| hex::parse_sha256(b) == Some(a))
}

fn remove_files(paths: &[PathBuf]) {
    for path in paths {
        let _ = std::fs::remove_file(path);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::games::DllType;

    fn dll(path: &Path, version: Option<&str>) -> DetectedDll {
        DetectedDll {
            path: path.to_owned(),
            name: path.file_name().unwrap().to_string_lossy().into_owned(),
            dll_type: crate::dll::detect::dll_type_of(&path.file_name().unwrap().to_string_lossy())
                .unwrap_or(DllType::Dlss),
            version: version.map(str::to_owned),
        }
    }

    fn now() -> Timestamp {
        "2026-10-26T15:02:10.1234Z".parse().unwrap()
    }

    struct Setup {
        _base: tempfile::TempDir,
        paths: Paths,
        game: PathBuf,
        files: PathBuf,
    }

    fn setup() -> Setup {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(&base.path().join("xdg"));
        let game = base.path().join("lib/steamapps/common/Glimmer Run");
        std::fs::create_dir_all(game.join("bin")).unwrap();
        let files = paths.backups_dir();
        Setup {
            _base: base,
            paths,
            game,
            files,
        }
    }

    #[test]
    fn go_s_time_text() {
        let utc = TimeZone::UTC;
        assert_eq!(
            go_time_text("2026-10-26T15:02:10.123456789Z".parse().unwrap(), &utc),
            "2026-10-26T15:02:10.123456789Z"
        );
        assert_eq!(go_time_text(now(), &utc), "2026-10-26T15:02:10.1234Z");
        assert_eq!(
            go_time_text("2026-10-26T15:02:10Z".parse().unwrap(), &utc),
            "2026-10-26T15:02:10Z"
        );
        let plus2 = TimeZone::fixed(jiff::tz::offset(2));
        assert_eq!(
            go_time_text("2026-10-26T15:02:10.123456789Z".parse().unwrap(), &plus2),
            "2026-10-26T17:02:10.123456789+02:00"
        );
        let minus = TimeZone::fixed(jiff::tz::Offset::from_seconds(-(3 * 3600 + 1800)).unwrap());
        assert_eq!(
            go_time_text("2026-10-26T15:02:10Z".parse().unwrap(), &minus),
            "2026-10-26T11:32:10-03:30"
        );
    }

    #[test]
    fn the_first_backup_of_each_path_is_kept_and_written_as_go_names_it() {
        let s = setup();
        let a = s.game.join("bin/nvngx_dlss.dll");
        let b = s.game.join("nvngx_dlssg.dll");
        std::fs::write(&a, b"original a").unwrap();
        let p = prepare_backup(
            &s.paths,
            &s.files,
            7,
            "Glimmer Run",
            &[dll(&a, Some("3.7"))],
            now(),
        )
        .unwrap();
        p.commit(&s.paths).unwrap();
        let first = backup::load(&s.paths, 7).unwrap().unwrap();
        assert_eq!(first.files.len(), 1);
        let entry = &first.files[0];
        assert_eq!(
            entry.backup_path,
            s.files.join("7").join(hashed_backup_name(&a))
        );
        assert_eq!(std::fs::read(&entry.backup_path).unwrap(), b"original a");
        assert_eq!(entry.version, "3.7");
        assert_eq!(first.backup_path, s.files.join("7"));
        let raw = std::fs::read_to_string(s.paths.backup_index(7)).unwrap();
        assert!(
            raw.starts_with(
                "{\n  \"app_id\": 7,\n  \"game_name\": \"Glimmer Run\",\n  \"created_at\": \""
            ),
            "{raw}"
        );
        assert!(!raw.ends_with('\n'), "Go's MarshalIndent has no newline");

        // A second swap keeps the first backup and adds the new file.
        std::fs::write(&a, b"spela's swap").unwrap();
        std::fs::write(&b, b"original b").unwrap();
        let p = prepare_backup(
            &s.paths,
            &s.files,
            7,
            "Glimmer Run",
            &[dll(&a, Some("310.5")), dll(&b, None)],
            now(),
        )
        .unwrap();
        p.commit(&s.paths).unwrap();
        let second = backup::load(&s.paths, 7).unwrap().unwrap();
        assert_eq!(second.files.len(), 2);
        assert_eq!(second.created_at, first.created_at);
        assert_eq!(
            std::fs::read(&second.files[0].backup_path).unwrap(),
            b"original a"
        );
        assert_eq!(second.files[0].version, "3.7");
        assert_eq!(second.files[1].version, "");
    }

    #[test]
    fn a_broken_index_or_an_orphan_backup_fails_closed() {
        let s = setup();
        let a = s.game.join("nvngx_dlss.dll");
        std::fs::write(&a, b"original").unwrap();
        std::fs::create_dir_all(s.paths.backup_index(7).parent().unwrap()).unwrap();
        std::fs::write(s.paths.backup_index(7), "{").unwrap();
        let err = prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&a, None)], now()).unwrap_err();
        assert!(
            err.to_string()
                .starts_with("load existing backup metadata: parse backup metadata"),
            "{err}"
        );
        std::fs::remove_file(s.paths.backup_index(7)).unwrap();

        let orphan = s.files.join("7").join(hashed_backup_name(&a));
        std::fs::write(&orphan, b"someone else's").unwrap();
        let err = prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&a, None)], now()).unwrap_err();
        assert_eq!(
            err.to_string(),
            format!(
                "backup destination already exists without metadata: {}",
                orphan.display()
            )
        );
        assert_eq!(std::fs::read(&orphan).unwrap(), b"someone else's");
        assert!(!s.paths.backup_index(7).exists());
    }

    #[test]
    fn a_failed_commit_removes_the_new_backup_files() {
        let s = setup();
        let a = s.game.join("nvngx_dlss.dll");
        std::fs::write(&a, b"original").unwrap();
        let files = s.game.parent().unwrap().join("custom");
        let p = prepare_backup(&s.paths, &files, 8, "G", &[dll(&a, None)], now()).unwrap();
        let made = p.index.files[0].backup_path.clone();
        assert!(made.is_file());
        // A file where the index's folder goes makes the save fail.
        std::fs::create_dir_all(s.paths.backups_dir()).unwrap();
        std::fs::write(s.paths.backups_dir().join("8"), b"blocks the folder").unwrap();
        assert!(matches!(p.commit(&s.paths), Err(SwapError::Save(_))));
        assert!(!made.exists());
    }

    #[test]
    fn created_files_are_not_originals_until_steam_ships_one() {
        let s = setup();
        let x = s.game.join("libxess.dll");
        let mut p = prepare_backup(&s.paths, &s.files, 7, "G", &[], now()).unwrap();
        record_install(&mut p.index, &x, &hex::sha256_hex(b"spela xess"), true);
        p.commit(&s.paths).unwrap();
        std::fs::write(&x, b"spela xess").unwrap();
        let p = prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&x, None)], now()).unwrap();
        assert!(p.index.files.is_empty(), "spela's own file is no original");
        assert_eq!(p.index.created_files, std::slice::from_ref(&x));

        std::fs::write(&x, b"steam xess").unwrap();
        let p = prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&x, None)], now()).unwrap();
        assert!(p.index.created_files.is_empty());
        assert_eq!(p.index.files.len(), 1);
        assert_eq!(
            std::fs::read(&p.index.files[0].backup_path).unwrap(),
            b"steam xess"
        );
        assert_eq!(p.index.installed_sha256(&x), None);
    }

    #[test]
    fn a_steam_update_refreshes_the_backup_and_a_go_index_never_does() {
        let s = setup();
        let a = s.game.join("nvngx_dlss.dll");
        std::fs::write(&a, b"original").unwrap();
        let mut p =
            prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&a, Some("1.0"))], now()).unwrap();
        record_install(&mut p.index, &a, &hex::sha256_hex(b"spela 310"), false);
        p.commit(&s.paths).unwrap();
        std::fs::write(&a, b"spela 310").unwrap();

        // Still spela's file: nothing refreshed.
        let p = prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&a, Some("310"))], now()).unwrap();
        assert!(p.refreshed.is_empty());

        // Steam shipped 2.0: the backup now holds it.
        std::fs::write(&a, b"steam 2.0").unwrap();
        let p = prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&a, Some("2.0"))], now()).unwrap();
        assert_eq!(p.refreshed, std::slice::from_ref(&a));
        assert_eq!(
            std::fs::read(&p.index.files[0].backup_path).unwrap(),
            b"steam 2.0"
        );
        assert_eq!(p.index.files[0].version, "2.0");

        // Without an installed record, as Go wrote it, the backup stays.
        let mut go = p.index.clone();
        go.installed.clear();
        save_index(&s.paths, &go).unwrap();
        std::fs::write(&a, b"steam 3.0").unwrap();
        let p = prepare_backup(&s.paths, &s.files, 7, "G", &[dll(&a, Some("3.0"))], now()).unwrap();
        assert!(p.refreshed.is_empty());
        assert_eq!(
            std::fs::read(&p.index.files[0].backup_path).unwrap(),
            b"steam 2.0"
        );
    }

    #[test]
    fn records_and_forgets_installs() {
        let mut index = BackupIndex::default();
        let a = Path::new("/g/nvngx_dlss.dll");
        let b = Path::new("/g/libxess.dll");
        let sha = hex::sha256_hex(b"x");
        record_install(&mut index, a, &sha.to_uppercase(), false);
        record_install(&mut index, b, &sha, true);
        record_install(&mut index, b, &sha, true);
        assert_eq!(index.installed_sha256(a), Some(sha.as_str()));
        assert_eq!(index.created_files, [b.to_owned()]);
        assert!(record_restore(&mut index, &[a.to_owned()], &[b.to_owned()]));
        assert!(index.installed.is_empty() && index.created_files.is_empty());
        assert!(!record_restore(&mut index, &[a.to_owned()], &[]));
    }

    #[test]
    fn containment_and_targets() {
        let s = setup();
        let inside = s.game.join("bin/nvngx_dlss.dll");
        assert_eq!(check_inside(&s.game, &inside), Ok(()));
        assert_eq!(check_inside(&s.game, &s.game.join("x.dll")), Ok(()));
        for bad in [
            s.game.join("../Other/nvngx_dlss.dll"),
            s.game.parent().unwrap().join("nvngx_dlss.dll"),
            s.game.clone(),
        ] {
            assert!(
                check_inside(&s.game, &bad).unwrap_err().contains("escapes"),
                "{bad:?}"
            );
        }
        // A symlinked folder inside the game that points outside it.
        let outside = s.game.parent().unwrap().join("outside");
        std::fs::create_dir_all(&outside).unwrap();
        std::os::unix::fs::symlink(&outside, s.game.join("link")).unwrap();
        assert!(check_inside(&s.game, &s.game.join("link/nvngx_dlss.dll")).is_err());
        assert!(check_inside(Path::new(""), &inside).is_err());

        // A DLL that is a symlink is refused, and its target is untouched.
        let real = outside.join("real.dll");
        std::fs::write(&real, b"keep").unwrap();
        let link = s.game.join("nvngx_dlss.dll");
        std::os::unix::fs::symlink(&real, &link).unwrap();
        let payload = s.game.parent().unwrap().join("payload.dll");
        std::fs::write(&payload, b"new").unwrap();
        assert!(replace_file(&payload, &link).is_err());
        assert_eq!(std::fs::read(&real).unwrap(), b"keep");
        assert!(check_target(&s.game.join("bin")).is_err());
    }

    #[test]
    fn replace_keeps_the_target_mode_and_leaves_no_temp_file() {
        let s = setup();
        let target = s.game.join("nvngx_dlss.dll");
        std::fs::write(&target, b"old").unwrap();
        std::fs::set_permissions(&target, std::fs::Permissions::from_mode(0o600)).unwrap();
        let payload = s.game.parent().unwrap().join("payload.dll");
        std::fs::write(&payload, b"new").unwrap();
        replace_file(&payload, &target).unwrap();
        assert_eq!(std::fs::read(&target).unwrap(), b"new");
        assert_eq!(
            std::fs::metadata(&target).unwrap().permissions().mode() & 0o777,
            0o600
        );
        let names: Vec<_> = std::fs::read_dir(&s.game)
            .unwrap()
            .map(|e| e.unwrap().file_name())
            .collect();
        assert_eq!(names.len(), 2, "{names:?}");
    }
}
