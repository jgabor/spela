//! Writing a file so a reader sees the old file or the new one, never half
//! of one.
//!
//! Every file spela writes goes through [`write_atomic`]. It follows Go
//! spela's sequence, a temp file in the same folder, chmod, write, close and
//! rename, and adds three things:
//!
//! - For user intent and backups ([`Durability::Durable`]) the file is
//!   synced before the rename and the folder after it. Caches skip both.
//! - A replaced file keeps its mode. A new file gets 0644.
//! - When the target is a symlink, as in a dotfiles setup, the link's target
//!   is replaced and the link survives.

use std::fs::{File, Permissions};
use std::io::{self, Write};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};

/// The mode of a file spela creates.
pub const NEW_FILE_MODE: u32 = 0o644;

/// How hard a write tries to survive a crash.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Durability {
    /// User intent and backups: fsync the file and its folder.
    Durable,
    /// Rebuildable data: no fsync.
    Cache,
}

/// Symlinks followed before giving up, as the kernel's own limit.
const MAX_LINKS: usize = 40;

/// Replaces `path` with `contents` atomically. The folder must exist.
pub fn write_atomic(path: &Path, contents: &[u8], durability: Durability) -> io::Result<()> {
    let target = resolve_links(path)?;
    let dir = match target.parent() {
        Some(d) if !d.as_os_str().is_empty() => d.to_owned(),
        _ => PathBuf::from("."),
    };
    let name = target
        .file_name()
        .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidInput, "path has no file name"))?;
    let mode = match std::fs::metadata(&target) {
        Ok(meta) => meta.permissions().mode() & 0o7777,
        Err(e) if e.kind() == io::ErrorKind::NotFound => NEW_FILE_MODE,
        Err(e) => return Err(e),
    };

    let mut prefix = std::ffi::OsString::from(".");
    prefix.push(name);
    prefix.push(".");
    let mut tmp = tempfile::Builder::new()
        .prefix(&prefix)
        .suffix(".tmp")
        .tempfile_in(&dir)?;
    tmp.as_file()
        .set_permissions(Permissions::from_mode(mode))?;
    tmp.write_all(contents)?;
    if durability == Durability::Durable {
        tmp.as_file().sync_all()?;
    }
    tmp.persist(&target).map_err(|e| e.error)?;
    if durability == Durability::Durable {
        File::open(&dir)?.sync_all()?;
    }
    Ok(())
}

/// Follows symlinks at `path` to the file a write should replace. A missing
/// file, or a link to one, resolves to where that file would be.
fn resolve_links(path: &Path) -> io::Result<PathBuf> {
    let mut current = path.to_owned();
    for _ in 0..MAX_LINKS {
        match std::fs::symlink_metadata(&current) {
            Ok(meta) if meta.file_type().is_symlink() => {
                let link = std::fs::read_link(&current)?;
                current = match current.parent() {
                    Some(parent) if link.is_relative() => parent.join(link),
                    _ => link,
                };
            }
            Ok(_) => return Ok(current),
            Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(current),
            Err(e) => return Err(e),
        }
    }
    Err(io::Error::new(
        io::ErrorKind::InvalidInput,
        format!("too many symlinks at {}", path.display()),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn mode(path: &Path) -> u32 {
        std::fs::metadata(path).unwrap().permissions().mode() & 0o7777
    }

    fn names(dir: &Path) -> Vec<String> {
        let mut v: Vec<String> = std::fs::read_dir(dir)
            .unwrap()
            .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
            .collect();
        v.sort();
        v
    }

    #[test]
    fn new_file_gets_0644_and_no_temp_file_is_left() {
        let dir = tempfile::tempdir().unwrap();
        let file = dir.path().join("config.toml");
        write_atomic(&file, b"version = 1\n", Durability::Durable).unwrap();
        assert_eq!(std::fs::read(&file).unwrap(), b"version = 1\n");
        assert_eq!(mode(&file), 0o644);
        assert_eq!(names(dir.path()), ["config.toml"]);
    }

    #[test]
    fn replaced_file_keeps_its_mode() {
        let dir = tempfile::tempdir().unwrap();
        let file = dir.path().join("games.json");
        std::fs::write(&file, b"old").unwrap();
        std::fs::set_permissions(&file, Permissions::from_mode(0o600)).unwrap();
        write_atomic(&file, b"new", Durability::Cache).unwrap();
        assert_eq!(std::fs::read(&file).unwrap(), b"new");
        assert_eq!(mode(&file), 0o600);
    }

    #[test]
    fn symlink_survives_and_its_target_changes() {
        let dir = tempfile::tempdir().unwrap();
        let dotfiles = dir.path().join("dotfiles");
        std::fs::create_dir(&dotfiles).unwrap();
        let real = dotfiles.join("config.toml");
        std::fs::write(&real, b"old").unwrap();
        let link = dir.path().join("config.toml");
        std::os::unix::fs::symlink("dotfiles/config.toml", &link).unwrap();

        write_atomic(&link, b"new", Durability::Durable).unwrap();
        assert!(
            std::fs::symlink_metadata(&link)
                .unwrap()
                .file_type()
                .is_symlink()
        );
        assert_eq!(std::fs::read(&real).unwrap(), b"new");
        assert_eq!(names(&dotfiles), ["config.toml"]);
    }

    #[test]
    fn dangling_link_creates_its_target() {
        let dir = tempfile::tempdir().unwrap();
        let link = dir.path().join("link");
        std::os::unix::fs::symlink(dir.path().join("made"), &link).unwrap();
        write_atomic(&link, b"x", Durability::Cache).unwrap();
        assert_eq!(std::fs::read(dir.path().join("made")).unwrap(), b"x");
    }

    #[test]
    fn link_loop_is_an_error() {
        let dir = tempfile::tempdir().unwrap();
        let a = dir.path().join("a");
        let b = dir.path().join("b");
        std::os::unix::fs::symlink(&b, &a).unwrap();
        std::os::unix::fs::symlink(&a, &b).unwrap();
        let err = write_atomic(&a, b"x", Durability::Cache).unwrap_err();
        assert!(err.to_string().contains("too many symlinks"), "{err}");
    }

    #[test]
    fn missing_folder_is_an_error_and_writes_nothing() {
        let dir = tempfile::tempdir().unwrap();
        let file = dir.path().join("missing/config.toml");
        assert!(write_atomic(&file, b"x", Durability::Durable).is_err());
        assert!(names(dir.path()).is_empty());
    }
}
