//! DLL backups, read side: `backup.json` and both backup file name schemes,
//! read in place (decision 50).
//!
//! Backups are how users get their original DLLs back, so Go's files are
//! read as Go wrote them and never moved or reshaped
//! (`internal/dll/swap.go`):
//!
//! - The index is `<data>/backups/<appid>/backup.json`. It never moves, even
//!   when `backup_path` sends new backup files elsewhere.
//! - Each entry records the absolute original path and the absolute path of
//!   its backup file. A backup file is named `<sha256>-<basename>`, the
//!   SHA-256 of the cleaned original path, or by Go's older scheme the plain
//!   `<basename>`. Restore uses the recorded path, so both schemes work and
//!   nothing is renamed.
//! - An index that does not parse, whose app id is wrong, that lists
//!   nothing, or that has an empty path fails closed: restore changes
//!   nothing. The error texts are Go's.
//!
//! Two top-level keys are new (decision 37). Go ignores them, and nothing
//! new ever goes into `files`:
//!
//! - `created_files`: DLLs an install created where none existed. Restore
//!   deletes them. An index may list only these, after an install into a
//!   game that had no DLLs, so "lists nothing" means no files and no
//!   created files.
//! - `installed`: each path spela wrote, mapped to the SHA-256 of what it
//!   last installed there. It tells a Steam update apart from spela's own
//!   install ([`BackupIndex::steam_replaced`]).
//!
//! Every other key is kept as it is, so a later write cannot drop one.

use std::collections::BTreeMap;
use std::ffi::OsString;
use std::io;
use std::os::unix::ffi::{OsStrExt, OsStringExt};
use std::path::{Path, PathBuf};

use jiff::Timestamp;
use serde::{Deserialize, Serialize};

use super::go_json::{self, null_default};
use crate::hex;
use crate::paths::Paths;

/// A game's backup index, `backup.json`, in Go's shape plus the new keys.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct BackupIndex {
    /// The game's AppID. Must match the folder the index is in.
    #[serde(default, deserialize_with = "null_default")]
    pub app_id: u64,
    /// The game's name when the first backup was made.
    #[serde(default, deserialize_with = "null_default")]
    pub game_name: String,
    /// When the first backup was made, exactly as written: Go writes RFC
    /// 3339 with nanoseconds and the local offset. Kept as text so a later
    /// write leaves it unchanged. See [`BackupIndex::created`].
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub created_at: Option<String>,
    /// The folder the first backup files went to. Informational: each
    /// file records its own path.
    #[serde(default, deserialize_with = "null_default")]
    pub backup_path: PathBuf,
    /// The backed up DLLs. The first backup of a path is kept.
    #[serde(default, deserialize_with = "null_default")]
    pub files: Vec<BackedUpFile>,
    /// DLLs an install created where none existed (decision 37).
    #[serde(
        default,
        deserialize_with = "null_default",
        skip_serializing_if = "Vec::is_empty"
    )]
    pub created_files: Vec<PathBuf>,
    /// The SHA-256 of what spela last installed at each path (decision 37).
    #[serde(
        default,
        deserialize_with = "null_default",
        skip_serializing_if = "BTreeMap::is_empty"
    )]
    pub installed: BTreeMap<PathBuf, String>,
    /// Keys this version does not know, kept as they are.
    #[serde(flatten)]
    pub other: serde_json::Map<String, serde_json::Value>,
}

/// One backed up DLL.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct BackedUpFile {
    /// Where the DLL was, absolute.
    #[serde(default, deserialize_with = "null_default")]
    pub original_path: PathBuf,
    /// The backup file, absolute, under either name scheme.
    #[serde(default, deserialize_with = "null_default")]
    pub backup_path: PathBuf,
    /// The DLL's file name, as detected.
    #[serde(default, deserialize_with = "null_default")]
    pub dll_name: String,
    /// The version backed up, empty when unknown.
    #[serde(default, deserialize_with = "null_default")]
    pub version: String,
}

/// One path a restore would change: an original put back, or a file an
/// install created, deleted.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RestoreTarget {
    /// The game's file.
    pub path: PathBuf,
    /// Its file name, as backed up.
    pub dll_name: String,
    /// The version backed up, empty when unknown or when the file is
    /// deleted.
    pub version: String,
    /// The restore deletes the file, because an install created it.
    pub deletes: bool,
}

/// How a backup file is named.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum NameScheme {
    /// `<sha256 of the cleaned original path>-<basename>`, Go's current
    /// scheme. A test that moves a fixture keeps the old hash, so only the
    /// shape is checked here, not the hash.
    Sha256,
    /// The plain `<basename>`, Go's older scheme.
    Basename,
    /// Any other name. Restore still uses the recorded path.
    Other,
}

impl BackedUpFile {
    /// The name scheme of the backup file.
    pub fn name_scheme(&self) -> NameScheme {
        let (Some(name), Some(base)) =
            (self.backup_path.file_name(), self.original_path.file_name())
        else {
            return NameScheme::Other;
        };
        let (name, base) = (name.as_bytes(), base.as_bytes());
        if name == base {
            return NameScheme::Basename;
        }
        let hex_len = hex::SHA256_HEX_LEN;
        let hashed = name.len() == hex_len + 1 + base.len()
            && name[..hex_len]
                .iter()
                .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(b))
            && name[hex_len] == b'-'
            && &name[hex_len + 1..] == base;
        if hashed {
            NameScheme::Sha256
        } else {
            NameScheme::Other
        }
    }
}

/// The name Go gives a new backup file: the SHA-256 of the cleaned
/// original path, a `-`, and the original's basename.
pub fn hashed_backup_name(original_path: &Path) -> OsString {
    let cleaned = clean(original_path);
    let mut name = hex::sha256_hex(cleaned.as_os_str().as_bytes()).into_bytes();
    name.push(b'-');
    name.extend_from_slice(
        original_path
            .file_name()
            .map(OsStrExt::as_bytes)
            .unwrap_or_default(),
    );
    OsString::from_vec(name)
}

/// Go's `filepath.Clean`: one `/` between parts, no `.` parts, and each
/// `..` taken back lexically. An empty result is `.`.
pub fn clean(path: &Path) -> PathBuf {
    let bytes = path.as_os_str().as_bytes();
    let rooted = bytes.first() == Some(&b'/');
    let mut parts: Vec<&[u8]> = Vec::new();
    for part in bytes.split(|b| *b == b'/') {
        match part {
            b"" | b"." => {}
            b".." => match parts.last() {
                Some(last) if *last != b".." => {
                    parts.pop();
                }
                _ if rooted => {}
                _ => parts.push(part),
            },
            _ => parts.push(part),
        }
    }
    let mut out = Vec::with_capacity(bytes.len());
    if rooted {
        out.push(b'/');
    }
    out.extend_from_slice(&parts.join(&b'/'));
    if out.is_empty() {
        out.push(b'.');
    }
    PathBuf::from(OsString::from_vec(out))
}

impl BackupIndex {
    /// When the first backup was made, or `None` when the index does not
    /// say or says Go's zero time.
    pub fn created(&self) -> Option<Timestamp> {
        self.created_at
            .as_deref()
            .and_then(|t| go_json::parse_time(t).ok().flatten())
    }

    /// The backup entry of an original path, compared after cleaning.
    pub fn entry(&self, original_path: &Path) -> Option<&BackedUpFile> {
        let want = clean(original_path);
        self.files.iter().find(|f| clean(&f.original_path) == want)
    }

    /// What a restore would do to each path, from the index alone, in
    /// [`plan_restore`]'s order: the backed up originals first, then the
    /// files an install created. The TUI shows these before it asks, and
    /// checks them again before the restore runs.
    pub fn restore_targets(&self) -> Vec<RestoreTarget> {
        let copies = self
            .files
            .iter()
            .filter(|f| !self.is_created(&f.original_path))
            .map(|f| RestoreTarget {
                path: f.original_path.clone(),
                dll_name: f.dll_name.clone(),
                version: f.version.clone(),
                deletes: false,
            });
        let deletes = self.created_files.iter().map(|p| RestoreTarget {
            path: p.clone(),
            dll_name: p
                .file_name()
                .map(|n| n.to_string_lossy().into_owned())
                .unwrap_or_default(),
            version: String::new(),
            deletes: true,
        });
        copies.chain(deletes).collect()
    }

    /// Whether an install created this path where no DLL existed.
    pub fn is_created(&self, path: &Path) -> bool {
        let want = clean(path);
        self.created_files.iter().any(|p| clean(p) == want)
    }

    /// The SHA-256 of what spela last installed at this path, if recorded.
    pub fn installed_sha256(&self, path: &Path) -> Option<&str> {
        let want = clean(path);
        self.installed
            .iter()
            .find(|(p, _)| clean(p) == want)
            .map(|(_, sha)| sha.as_str())
    }

    /// Whether Steam replaced the DLL at `path` since spela last installed
    /// there, so its backup should be refreshed before the next swap
    /// (decision 37): the file on disk matches neither its backup nor what
    /// spela recorded installing. With no record of an install, as for every
    /// Go-era backup, the answer is no: the file on disk may be what Go
    /// installed, and refreshing would lose the real original.
    pub fn steam_replaced(&self, path: &Path, current_sha256: &str, backup_sha256: &str) -> bool {
        let Some(installed) = self.installed_sha256(path) else {
            return false;
        };
        let same = |a: &str, b: &str| {
            hex::parse_sha256(a).is_some_and(|a| hex::parse_sha256(b) == Some(a))
        };
        !same(current_sha256, installed) && !same(current_sha256, backup_sha256)
    }

    /// Checks an index read from `<data>/backups/<app_id>/backup.json`, in
    /// Go's order and with Go's texts, plus the new keys.
    fn validate(&self, app_id: u64) -> Result<(), BackupError> {
        if let Some(text) = &self.created_at
            && let Err(e) = go_json::parse_time(text)
        {
            return Err(BackupError::Parse(format!("created_at {text:?}: {e}")));
        }
        let invalid = |m: String| Err(BackupError::Invalid(m));
        if self.app_id != app_id {
            return invalid(format!("app ID is {}, want {app_id}", self.app_id));
        }
        if self.files.is_empty() && self.created_files.is_empty() {
            return invalid("no files".to_owned());
        }
        for (i, file) in self.files.iter().enumerate() {
            let paths = [&file.original_path, &file.backup_path];
            if paths.iter().any(|p| p.as_os_str().is_empty()) {
                return invalid(format!("file {i} has an empty path"));
            }
            if paths.iter().any(|p| !p.is_absolute()) {
                return invalid(format!("file {i} has a relative path"));
            }
        }
        for (i, path) in self.created_files.iter().enumerate() {
            if path.as_os_str().is_empty() {
                return invalid(format!("created file {i} has an empty path"));
            }
            if !path.is_absolute() {
                return invalid(format!("created file {i} has a relative path"));
            }
        }
        for (path, sha) in &self.installed {
            if !path.is_absolute() {
                return invalid(format!("installed path {:?} is not absolute", path));
            }
            if hex::parse_sha256(sha).is_none() {
                return invalid(format!(
                    "installed hash of {} is not a SHA-256",
                    path.display()
                ));
            }
        }
        Ok(())
    }
}

/// A backup index that cannot be used. Restore changes nothing.
#[derive(Debug, thiserror::Error)]
pub enum BackupError {
    /// The index is there but cannot be read.
    #[error("cannot read {}: {source}", path.display())]
    Read {
        /// The index file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
    /// The index does not parse. Go's prefix; the detail is serde's.
    #[error("parse backup metadata: {0}")]
    Parse(String),
    /// The index parses but is not valid, Go's text.
    #[error("invalid backup metadata: {0}")]
    Invalid(String),
}

/// A game's backup index, or `None` when it has none. Go's `LoadBackup`.
pub fn load(paths: &Paths, app_id: u64) -> Result<Option<BackupIndex>, BackupError> {
    let path = paths.backup_index(app_id);
    let text = match std::fs::read(&path) {
        Ok(text) => text,
        Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(None),
        Err(source) => return Err(BackupError::Read { path, source }),
    };
    let index: BackupIndex =
        serde_json::from_slice(&text).map_err(|e| BackupError::Parse(e.to_string()))?;
    index.validate(app_id)?;
    Ok(Some(index))
}

/// Whether a game has a usable backup, Go's `BackupExists`: an index that
/// loads. A broken index is no backup.
pub fn has_backup(paths: &Paths, app_id: u64) -> bool {
    matches!(load(paths, app_id), Ok(Some(_)))
}

/// What a restore of one game would do, after its read-only preflight.
#[derive(Debug, Clone, PartialEq)]
pub struct RestorePlan {
    /// The index the plan comes from.
    pub index: BackupIndex,
    /// Backup files to copy back over their originals, in index order.
    pub copies: Vec<BackedUpFile>,
    /// Files an install created, to delete. Only those that exist now and
    /// still hold what spela installed there.
    pub deletes: Vec<PathBuf>,
    /// Files an install created that now hold something else: Steam shipped
    /// its own file at that path since, so it is the game's file now. The
    /// restore keeps them and forgets that spela created them.
    pub kept: Vec<PathBuf>,
}

/// Why a restore cannot start. The texts are Go's.
#[derive(Debug, thiserror::Error)]
pub enum RestoreError {
    /// The game has no backup index.
    #[error("no backup found for app {0}")]
    NoBackup(u64),
    /// The index cannot be used.
    #[error(transparent)]
    Index(#[from] BackupError),
    /// A file of the index fails the preflight.
    #[error("cannot restore {name}: {reason}")]
    Preflight {
        /// The DLL's file name, or the path of a created file.
        name: String,
        /// Why.
        reason: String,
    },
}

/// Plans a restore of one game and runs Go's preflight on it without
/// changing anything: every backup file is a regular file, every target's
/// folder exists, and no target is a folder. Files an install created are
/// deleted rather than restored, even when a later backup lists them too,
/// because before spela there was no file there. A created file that is
/// already gone is left out; one that is a folder fails the preflight. A
/// created file whose SHA-256 no longer matches what spela recorded
/// installing there is kept: Steam put the game's own file at that path
/// since. With no record of the install, the file is taken to be spela's.
pub fn plan_restore(paths: &Paths, app_id: u64) -> Result<RestorePlan, RestoreError> {
    let index = load(paths, app_id)?.ok_or(RestoreError::NoBackup(app_id))?;
    let mut copies = Vec::new();
    for file in &index.files {
        if index.is_created(&file.original_path) {
            continue;
        }
        let fail = |reason: String| RestoreError::Preflight {
            name: file.dll_name.clone(),
            reason,
        };
        check_regular_file(&file.backup_path).map_err(fail)?;
        let parent_is_dir = file
            .original_path
            .parent()
            .and_then(|p| std::fs::metadata(p).ok())
            .is_some_and(|m| m.is_dir());
        if !parent_is_dir {
            return Err(fail("target directory is unavailable".to_owned()));
        }
        match std::fs::metadata(&file.original_path) {
            Ok(m) if m.is_dir() => return Err(fail("target is a directory".to_owned())),
            Ok(_) => {}
            Err(e) if e.kind() == io::ErrorKind::NotFound => {}
            Err(e) => return Err(fail(format!("{}: {e}", file.original_path.display()))),
        }
        copies.push(file.clone());
    }
    let mut deletes = Vec::new();
    let mut kept = Vec::new();
    for path in &index.created_files {
        match std::fs::symlink_metadata(path) {
            Ok(m) if m.is_dir() => {
                return Err(RestoreError::Preflight {
                    name: path.display().to_string(),
                    reason: "an installed file is now a directory".to_owned(),
                });
            }
            Ok(_) => match replaced_since_install(&index, path) {
                Ok(true) => kept.push(path.clone()),
                Ok(false) => deletes.push(path.clone()),
                Err(reason) => {
                    return Err(RestoreError::Preflight {
                        name: path.display().to_string(),
                        reason,
                    });
                }
            },
            Err(e) if e.kind() == io::ErrorKind::NotFound => {}
            Err(e) => {
                return Err(RestoreError::Preflight {
                    name: path.display().to_string(),
                    reason: e.to_string(),
                });
            }
        }
    }
    Ok(RestorePlan {
        index,
        copies,
        deletes,
        kept,
    })
}

/// Whether a file an install created now holds something other than what
/// spela recorded installing there. `false` when there is no record.
fn replaced_since_install(index: &BackupIndex, path: &Path) -> Result<bool, String> {
    let Some(installed) = index.installed_sha256(path) else {
        return Ok(false);
    };
    let current = std::fs::File::open(path)
        .and_then(hex::sha256_reader)
        .map_err(|e| format!("read {}: {e}", path.display()))?;
    let same = hex::parse_sha256(&current).is_some_and(|a| hex::parse_sha256(installed) == Some(a));
    Ok(!same)
}

/// Go's `preflightRegularFile`: the file opens and is a regular file.
fn check_regular_file(path: &Path) -> Result<(), String> {
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

#[cfg(test)]
mod tests {
    use super::*;

    const SHA_A: &str = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad";
    const SHA_B: &str = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855";
    const SHA_C: &str = "8707e53b26c68c606b98bf31c223485ff30d310a261b1a36d48b2eaabc1507ec";

    fn setup() -> (tempfile::TempDir, Paths) {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        (base, paths)
    }

    fn write_index(paths: &Paths, app_id: u64, text: &str) {
        let path = paths.backup_index(app_id);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, text).unwrap();
    }

    fn load_err(paths: &Paths, app_id: u64, text: &str) -> String {
        write_index(paths, app_id, text);
        load(paths, app_id).unwrap_err().to_string()
    }

    #[test]
    fn go_s_invalid_indexes_fail_with_go_s_texts() {
        // internal/dll/swap_test.go, TestBackupMetadataAndPathPrimitiveValidation.
        let (_base, paths) = setup();
        assert_eq!(
            load_err(
                &paths,
                1,
                r#"{"app_id":2,"files":[{"original_path":"/a","backup_path":"/b"}]}"#
            ),
            "invalid backup metadata: app ID is 2, want 1"
        );
        assert_eq!(
            load_err(&paths, 1, r#"{"app_id":1,"files":[]}"#),
            "invalid backup metadata: no files"
        );
        assert_eq!(
            load_err(&paths, 1, r#"{"app_id":1,"files":null}"#),
            "invalid backup metadata: no files"
        );
        assert_eq!(
            load_err(
                &paths,
                1,
                r#"{"app_id":1,"files":[{"original_path":"","backup_path":"/b"}]}"#
            ),
            "invalid backup metadata: file 0 has an empty path"
        );
        assert!(load_err(&paths, 1, "{").starts_with("parse backup metadata: "));
        assert!(load_err(&paths, 1, r#"{"app_id":-1}"#).starts_with("parse backup metadata: "));
        let err = load_err(&paths, 1, r#"{"app_id":1,"created_at":"soon","files":[]}"#);
        assert!(
            err.starts_with(r#"parse backup metadata: created_at "soon": "#),
            "{err}"
        );
        assert!(!has_backup(&paths, 1));
        assert!(load(&paths, 2).unwrap().is_none());
        assert!(!has_backup(&paths, 2));
    }

    #[test]
    fn paths_must_be_absolute_including_the_new_keys() {
        let (_base, paths) = setup();
        assert_eq!(
            load_err(
                &paths,
                1,
                r#"{"app_id":1,"files":[{"original_path":"a/x.dll","backup_path":"/b"}]}"#
            ),
            "invalid backup metadata: file 0 has a relative path"
        );
        assert_eq!(
            load_err(&paths, 1, r#"{"app_id":1,"created_files":[""]}"#),
            "invalid backup metadata: created file 0 has an empty path"
        );
        assert_eq!(
            load_err(&paths, 1, r#"{"app_id":1,"created_files":["x.dll"]}"#),
            "invalid backup metadata: created file 0 has a relative path"
        );
        let text = format!(
            r#"{{"app_id":1,"created_files":["/g/x.dll"],"installed":{{"x.dll":"{SHA_A}"}}}}"#
        );
        assert_eq!(
            load_err(&paths, 1, &text),
            "invalid backup metadata: installed path \"x.dll\" is not absolute"
        );
        assert_eq!(
            load_err(
                &paths,
                1,
                r#"{"app_id":1,"created_files":["/g/x.dll"],"installed":{"/g/x.dll":"nope"}}"#
            ),
            "invalid backup metadata: installed hash of /g/x.dll is not a SHA-256"
        );
    }

    #[test]
    fn an_index_of_only_created_files_is_valid() {
        let (_base, paths) = setup();
        let text = format!(
            r#"{{"app_id":7,"game_name":"Ashen Throne","backup_path":"/d/7","files":[],"created_files":["/g/nvngx_dlss.dll"],"installed":{{"/g/nvngx_dlss.dll":"{SHA_A}"}}}}"#
        );
        write_index(&paths, 7, &text);
        let index = load(&paths, 7).unwrap().unwrap();
        assert!(index.is_created(Path::new("/g/./nvngx_dlss.dll")));
        assert_eq!(
            index.installed_sha256(Path::new("/g//nvngx_dlss.dll")),
            Some(SHA_A)
        );
        assert!(has_backup(&paths, 7));
    }

    #[test]
    fn round_trips_go_s_keys_new_keys_and_unknown_keys() {
        let text = format!(
            r#"{{"app_id":9,"game_name":"G","created_at":"2026-10-26T17:02:10.123456789+02:00","backup_path":"/d/9","files":[{{"original_path":"/g/nvngx_dlss.dll","backup_path":"/d/9/nvngx_dlss.dll","dll_name":"nvngx_dlss.dll","version":"3.7.10"}}],"later":{{"x":1}},"installed":{{"/g/nvngx_dlss.dll":"{SHA_A}"}}}}"#
        );
        let index: BackupIndex = serde_json::from_str(&text).unwrap();
        assert_eq!(index.other["later"]["x"], 1);
        assert_eq!(
            index.created(),
            Some("2026-10-26T15:02:10.123456789Z".parse().unwrap())
        );
        let again = serde_json::to_string(&index).unwrap();
        assert!(
            again.starts_with(r#"{"app_id":9,"game_name":"G","created_at":"2026-10-26T17:02:10.123456789+02:00","backup_path":"/d/9","files":[{"original_path":"#),
            "Go's keys first, created_at unchanged: {again}"
        );
        assert!(
            !again.contains("created_files"),
            "empty new keys stay out: {again}"
        );
        assert_eq!(serde_json::from_str::<BackupIndex>(&again).unwrap(), index);
        // Go's zero time is no time.
        let zero: BackupIndex =
            serde_json::from_str(r#"{"created_at":"0001-01-01T00:00:00Z"}"#).unwrap();
        assert_eq!(zero.created(), None);
    }

    #[test]
    fn name_schemes() {
        let file = |backup: &str| BackedUpFile {
            original_path: "/g/bin/nvngx_dlss.dll".into(),
            backup_path: backup.into(),
            ..BackedUpFile::default()
        };
        let hashed = format!("/d/1/{SHA_C}-nvngx_dlss.dll");
        assert_eq!(file(&hashed).name_scheme(), NameScheme::Sha256);
        assert_eq!(
            file("/d/1/nvngx_dlss.dll").name_scheme(),
            NameScheme::Basename
        );
        assert_eq!(
            file(&hashed.to_uppercase()).name_scheme(),
            NameScheme::Other,
            "Go writes lowercase hex"
        );
        assert_eq!(
            file("/d/1/abc-nvngx_dlss.dll").name_scheme(),
            NameScheme::Other
        );
        assert_eq!(
            file(&format!("/d/1/{SHA_C}-nvngx_dlssg.dll")).name_scheme(),
            NameScheme::Other
        );
    }

    #[test]
    fn hashed_name_hashes_the_cleaned_path() {
        let path =
            Path::new("/fixture-root/library/steamapps/common/Glimmer Run/bin/x64/nvngx_dlss.dll");
        let name = hashed_backup_name(path);
        let want = format!(
            "{}-nvngx_dlss.dll",
            hex::sha256_hex(path.as_os_str().as_bytes())
        );
        assert_eq!(name, OsString::from(want));
        let messy = Path::new(
            "/fixture-root//library/./steamapps/common/x/../Glimmer Run/bin/x64/nvngx_dlss.dll",
        );
        assert_eq!(hashed_backup_name(messy), name);
    }

    #[test]
    fn clean_is_go_s_filepath_clean() {
        // Go's own path/filepath cleantests, Unix part.
        let cases = [
            ("abc", "abc"),
            ("abc/def", "abc/def"),
            ("a/b/c", "a/b/c"),
            (".", "."),
            ("..", ".."),
            ("../..", "../.."),
            ("../../abc", "../../abc"),
            ("/abc", "/abc"),
            ("/", "/"),
            ("", "."),
            ("abc/", "abc"),
            ("abc/def/", "abc/def"),
            ("/abc/", "/abc"),
            ("//abc", "/abc"),
            ("///abc", "/abc"),
            ("//abc//", "/abc"),
            ("abc//def//ghi", "abc/def/ghi"),
            ("abc/./def", "abc/def"),
            ("/./abc/def", "/abc/def"),
            ("abc/.", "abc"),
            ("abc/def/ghi/../jkl", "abc/def/jkl"),
            ("abc/def/../ghi/../jkl", "abc/jkl"),
            ("abc/def/..", "abc"),
            ("abc/def/../..", "."),
            ("/abc/def/../..", "/"),
            ("abc/def/../../..", ".."),
            ("/abc/def/../../..", "/"),
            ("abc/def/../../../ghi/jkl/../../../mno", "../../mno"),
            ("/../abc", "/abc"),
            ("abc/./../def", "def"),
            ("abc//./../def", "def"),
            ("abc/../../././../def", "../../def"),
        ];
        for (input, want) in cases {
            assert_eq!(clean(Path::new(input)), PathBuf::from(want), "{input:?}");
        }
    }

    #[test]
    fn steam_replaced_needs_a_record_of_spela_s_install() {
        let mut index = BackupIndex::default();
        let path = Path::new("/g/nvngx_dlss.dll");
        // A Go-era index: no record, so never refreshed.
        assert!(!index.steam_replaced(path, SHA_C, SHA_B));
        index.installed.insert(path.into(), SHA_A.into());
        assert!(
            index.steam_replaced(path, SHA_C, SHA_B),
            "Steam's own new file"
        );
        assert!(
            !index.steam_replaced(path, SHA_A, SHA_B),
            "still spela's install"
        );
        assert!(
            !index.steam_replaced(path, &SHA_A.to_uppercase(), SHA_B),
            "any case"
        );
        assert!(
            !index.steam_replaced(path, SHA_B, SHA_B),
            "back to the original"
        );
    }

    #[test]
    fn restore_preflight_has_go_s_texts_and_changes_nothing() {
        let (base, paths) = setup();
        let game = base.path().join("game");
        let backups = base.path().join("data/backups/5");
        std::fs::create_dir_all(&game).unwrap();
        std::fs::create_dir_all(&backups).unwrap();
        let original = game.join("nvngx_dlss.dll");
        let backup = backups.join("nvngx_dlss.dll");
        std::fs::write(&original, b"swapped").unwrap();
        std::fs::write(&backup, b"original").unwrap();
        let created = game.join("libxess.dll");
        std::fs::write(&created, b"installed").unwrap();
        let entry = |original: &Path, backup: &Path| serde_json::json!({"original_path": original, "backup_path": backup, "dll_name": "nvngx_dlss.dll", "version": "3.7.10"});
        let index = |files: serde_json::Value, created: serde_json::Value| {
            serde_json::json!({"app_id": 5, "files": files, "created_files": created}).to_string()
        };

        assert_eq!(
            plan_restore(&paths, 5).unwrap_err().to_string(),
            "no backup found for app 5"
        );

        let gone = game.join("gone.dll");
        write_index(
            &paths,
            5,
            &index(
                serde_json::json!([entry(&original, &backup), entry(&created, &backup)]),
                serde_json::json!([&created, &gone]),
            ),
        );
        let plan = plan_restore(&paths, 5).unwrap();
        assert_eq!(
            plan.copies.len(),
            1,
            "a created file is deleted, not restored"
        );
        assert_eq!(plan.copies[0].original_path, original);
        assert_eq!(
            plan.deletes,
            std::slice::from_ref(&created),
            "a gone file is left out"
        );
        assert_eq!(
            std::fs::read(&original).unwrap(),
            b"swapped",
            "nothing changed"
        );
        assert!(created.exists());

        let missing = backups.join("missing.dll");
        write_index(
            &paths,
            5,
            &index(
                serde_json::json!([entry(&original, &missing)]),
                serde_json::json!([]),
            ),
        );
        let err = plan_restore(&paths, 5).unwrap_err().to_string();
        assert!(
            err.starts_with(&format!(
                "cannot restore nvngx_dlss.dll: open {}: ",
                missing.display()
            )),
            "{err}"
        );

        write_index(
            &paths,
            5,
            &index(
                serde_json::json!([entry(&original, &backups)]),
                serde_json::json!([]),
            ),
        );
        assert_eq!(
            plan_restore(&paths, 5).unwrap_err().to_string(),
            format!(
                "cannot restore nvngx_dlss.dll: {} is not a regular file",
                backups.display()
            )
        );

        let elsewhere = base.path().join("moved/nvngx_dlss.dll");
        write_index(
            &paths,
            5,
            &index(
                serde_json::json!([entry(&elsewhere, &backup)]),
                serde_json::json!([]),
            ),
        );
        assert_eq!(
            plan_restore(&paths, 5).unwrap_err().to_string(),
            "cannot restore nvngx_dlss.dll: target directory is unavailable"
        );

        write_index(
            &paths,
            5,
            &index(
                serde_json::json!([entry(&game, &backup)]),
                serde_json::json!([]),
            ),
        );
        assert_eq!(
            plan_restore(&paths, 5).unwrap_err().to_string(),
            "cannot restore nvngx_dlss.dll: target is a directory"
        );

        write_index(
            &paths,
            5,
            &index(serde_json::json!([]), serde_json::json!([&game])),
        );
        assert_eq!(
            plan_restore(&paths, 5).unwrap_err().to_string(),
            format!(
                "cannot restore {}: an installed file is now a directory",
                game.display()
            )
        );

        write_index(&paths, 5, "{");
        assert!(matches!(
            plan_restore(&paths, 5),
            Err(RestoreError::Index(BackupError::Parse(_)))
        ));
    }

    #[test]
    fn restore_targets_put_originals_first_and_skip_created_entries() {
        let file = |path: &str, version: &str| BackedUpFile {
            original_path: path.into(),
            backup_path: format!("/b/{path}").into(),
            dll_name: "nvngx_dlss.dll".into(),
            version: version.into(),
        };
        let index = BackupIndex {
            app_id: 5,
            files: vec![file("/g/a/nvngx_dlss.dll", "310.1"), file("/g/new.dll", "")],
            created_files: vec!["/g/new.dll".into()],
            ..BackupIndex::default()
        };
        let targets = index.restore_targets();
        assert_eq!(targets.len(), 2);
        assert_eq!(targets[0].path, PathBuf::from("/g/a/nvngx_dlss.dll"));
        assert_eq!(
            (targets[0].version.as_str(), targets[0].deletes),
            ("310.1", false)
        );
        assert_eq!(targets[1].path, PathBuf::from("/g/new.dll"));
        assert_eq!(
            (targets[1].dll_name.as_str(), targets[1].deletes),
            ("new.dll", true)
        );
    }
}
