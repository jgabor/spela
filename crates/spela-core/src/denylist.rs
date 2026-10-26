//! The anti-cheat deny list: the built-in list, the user list that adds to
//! it, and the allow and deny overrides.
//!
//! A DLL install or update into a game with kernel anti-cheat can get the
//! player banned, so spela refuses one for a denied game. Restore is never
//! refused: it puts the game's own files back.
//!
//! The files are TOML in `<config>/lists/`:
//!
//! - `overrides.toml`: `allowed`, a list of AppIDs, and `[[denied]]` tables
//!   with `app_id`, `name` and `reason`. spela writes it for `denylist
//!   allow`, `deny` and `clear`.
//! - `denylist.toml`, optional: `[[entries]]` tables of the same shape. The
//!   migration writes it once from Go's `denylist.yaml`; after that only the
//!   user does.
//!
//! What changes from Go (`internal/denylist`):
//!
//! - The user list adds to the built-in list instead of replacing it
//!   (decision 36). `denylist allow` is how a built-in entry is lifted.
//! - Allow and deny for one game are exclusive: allowing removes the game
//!   from `denied`, denying removes it from `allowed`, so the latest action
//!   wins (decision 36). A file that lists a game in both, as Go could
//!   write, still reads as allowed.
//! - A list that cannot be read or does not parse fails closed: install and
//!   update are refused with an error naming the file (decision 8). Go
//!   ignored it, so every swap was allowed. An unknown key is an error too,
//!   so a misspelt `[[denid]]` cannot drop a denial unnoticed.
//!
//! A game is matched by AppID only, in this order: `allowed`, then
//! `denied`, then the user list, then the built-in list.

use std::io;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use toml_edit::{DocumentMut, InlineTable, Item, Table, value};

use crate::fs::{Durability, write_atomic};
use crate::lock::{LockError, StoreLock};
use crate::paths::Paths;

/// The version a list file carries.
pub const LIST_VERSION: i64 = 1;

/// The reason `denylist deny` records when none is given, Go's.
pub const DEFAULT_REASON: &str = "user-specified";

/// The built-in list, Go's (`internal/denylist/denylist.go:29-42`): AppID,
/// name and reason.
pub const BUILT_IN: [(u64, &str, &str); 10] = [
    (1172470, "Apex Legends", "Easy Anti-Cheat"),
    (1938090, "Call of Duty: HQ", "RICOCHET Anti-Cheat"),
    (1517290, "Battlefield 2042", "Easy Anti-Cheat"),
    (1063730, "New World", "Easy Anti-Cheat"),
    (1599340, "Lost Ark", "Easy Anti-Cheat"),
    (594650, "Hunt: Showdown", "Easy Anti-Cheat"),
    (578080, "PUBG: Battlegrounds", "Anti-Cheat"),
    (1085660, "Destiny 2", "BattlEye"),
    (252490, "Rust", "Easy Anti-Cheat"),
    (1623660, "Escape from Tarkov", "BattlEye"),
];

/// One denied game. The shape of Go's YAML entries and of the TOML tables.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DenyEntry {
    /// The Steam AppID.
    pub app_id: u64,
    /// The game's name, for display.
    #[serde(default)]
    pub name: String,
    /// Why it is denied, such as the anti-cheat's name.
    #[serde(default)]
    pub reason: String,
}

/// The overrides, `overrides.toml`, in Go's shape (`overrides.yaml` reads
/// into this too).
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Overrides {
    /// Games allowed whatever the lists say.
    #[serde(default)]
    pub allowed: Vec<u64>,
    /// Games the user denied.
    #[serde(default)]
    pub denied: Vec<DenyEntry>,
}

/// The user's deny list, `denylist.toml`, in Go's shape (`denylist.yaml`
/// reads into this too).
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct UserList {
    /// The denied games.
    #[serde(default)]
    pub entries: Vec<DenyEntry>,
}

/// Which list denies a game.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Source {
    /// spela's built-in list.
    BuiltIn,
    /// The user's `denylist.toml`.
    UserList,
    /// `denylist deny`, the `denied` list of `overrides.toml`.
    UserDenied,
}

/// A list entry and the list it is in.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Listed<'a> {
    /// The entry.
    pub entry: &'a DenyEntry,
    /// Its list.
    pub source: Source,
}

/// What the lists say about one game.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Status<'a> {
    /// No list names it.
    Allowed,
    /// `denylist allow` allows it. `over` is the entry the allow lifts,
    /// when a list names the game.
    ForceAllowed {
        /// The lifted entry.
        over: Option<Listed<'a>>,
    },
    /// A list denies it.
    Denied(Listed<'a>),
}

impl Status<'_> {
    /// Whether DLL swaps are refused.
    pub fn is_denied(&self) -> bool {
        matches!(self, Status::Denied(_))
    }

    /// The reason of a denial.
    pub fn reason(&self) -> Option<&str> {
        match self {
            Status::Denied(listed) => Some(&listed.entry.reason),
            _ => None,
        }
    }
}

/// A list file that cannot be used.
#[derive(Debug, thiserror::Error)]
pub enum ListError {
    /// The file is there but cannot be read.
    #[error("cannot read {}: {source}", path.display())]
    Read {
        /// The list file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
    /// The file does not parse or has a value spela does not accept.
    #[error("{} is not a valid deny list: {message}", path.display())]
    Invalid {
        /// The list file.
        path: PathBuf,
        /// What is wrong, with the line and column when TOML knows them.
        message: String,
    },
    /// The file was written by a newer spela, so this one leaves it alone.
    #[error(
        "{} was written by a newer spela (version {version}), so this one does not change it",
        path.display()
    )]
    Newer {
        /// The list file.
        path: PathBuf,
        /// The file's version.
        version: i64,
    },
    /// The file cannot be written.
    #[error("cannot write {}: {source}", path.display())]
    Write {
        /// The list file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
    /// `config.lock` cannot be taken.
    #[error(transparent)]
    Lock(#[from] LockError),
}

/// Why a DLL install or update is refused.
#[derive(Debug, thiserror::Error)]
pub enum SwapRefused {
    /// A list denies the game. The text is Go's.
    #[error("DLL swap denied for app {app_id}: {reason}")]
    Denied {
        /// The game.
        app_id: u64,
        /// The list's reason.
        reason: String,
    },
    /// A list does not load, so spela cannot tell (decision 8).
    #[error("DLL install and update are refused until the deny list loads: {0}")]
    Unreadable(#[from] ListError),
}

/// The built-in list, the user list and the overrides, loaded.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DenyLists {
    built_in: Vec<DenyEntry>,
    user: UserList,
    overrides: Overrides,
}

/// The built-in list as entries.
pub fn built_in() -> Vec<DenyEntry> {
    BUILT_IN
        .iter()
        .map(|(app_id, name, reason)| DenyEntry {
            app_id: *app_id,
            name: (*name).to_owned(),
            reason: (*reason).to_owned(),
        })
        .collect()
}

impl DenyLists {
    /// The lists from a user list and overrides, with the built-in list.
    pub fn new(user: UserList, overrides: Overrides) -> DenyLists {
        DenyLists {
            built_in: built_in(),
            user,
            overrides,
        }
    }

    /// What the lists say about a game.
    pub fn check(&self, app_id: u64) -> Status<'_> {
        let denied = self.denied_entry(app_id);
        if self.is_force_allowed(app_id) {
            return Status::ForceAllowed { over: denied };
        }
        denied.map_or(Status::Allowed, Status::Denied)
    }

    /// Refuses a swap into a denied game, with Go's text.
    pub fn guard(&self, app_id: u64) -> Result<(), SwapRefused> {
        match self.check(app_id) {
            Status::Denied(listed) => Err(SwapRefused::Denied {
                app_id,
                reason: listed.entry.reason.clone(),
            }),
            _ => Ok(()),
        }
    }

    /// Whether `denylist allow` allows the game.
    pub fn is_force_allowed(&self, app_id: u64) -> bool {
        self.overrides.allowed.contains(&app_id)
    }

    /// The anti-cheat list `denylist show` prints first: the built-in
    /// entries in their order, each replaced in place by a user list entry
    /// for the same game, then the user list's other entries in file order.
    /// A game appears once.
    pub fn anti_cheat(&self) -> Vec<Listed<'_>> {
        let mut out: Vec<Listed<'_>> = Vec::new();
        for entry in &self.built_in {
            let listed = match self.user_entry(entry.app_id) {
                Some(user) => Listed {
                    entry: user,
                    source: Source::UserList,
                },
                None => Listed {
                    entry,
                    source: Source::BuiltIn,
                },
            };
            out.push(listed);
        }
        for entry in &self.user.entries {
            if !out.iter().any(|l| l.entry.app_id == entry.app_id) {
                out.push(Listed {
                    entry,
                    source: Source::UserList,
                });
            }
        }
        out
    }

    /// The games `denylist deny` denied, in file order.
    pub fn user_denied(&self) -> &[DenyEntry] {
        &self.overrides.denied
    }

    /// The overrides as loaded.
    pub fn overrides(&self) -> &Overrides {
        &self.overrides
    }

    fn user_entry(&self, app_id: u64) -> Option<&DenyEntry> {
        self.user.entries.iter().find(|e| e.app_id == app_id)
    }

    fn denied_entry(&self, app_id: u64) -> Option<Listed<'_>> {
        if let Some(entry) = self.overrides.denied.iter().find(|e| e.app_id == app_id) {
            return Some(Listed {
                entry,
                source: Source::UserDenied,
            });
        }
        self.anti_cheat()
            .into_iter()
            .find(|l| l.entry.app_id == app_id)
    }
}

/// Loads the lists. A missing file is an empty one. A file that cannot be
/// read or does not parse is an error that names it.
pub fn load(paths: &Paths) -> Result<DenyLists, ListError> {
    let overrides = match read(&paths.deny_overrides())? {
        Some((path, text)) => parse_overrides(&path, &text)?.overrides,
        None => Overrides::default(),
    };
    let user = match read(&paths.user_denylist())? {
        Some((path, text)) => parse_user_list(&path, &text)?,
        None => UserList::default(),
    };
    Ok(DenyLists::new(user, overrides))
}

/// Refuses a DLL install or update into `app_id` when a list denies it, or
/// when a list does not load.
pub fn guard_swap(paths: &Paths, app_id: u64) -> Result<(), SwapRefused> {
    load(paths)?.guard(app_id)
}

/// `denylist allow`: adds the game to `allowed` and removes it from
/// `denied`. Returns whether the file changed.
pub fn allow(paths: &Paths, app_id: u64) -> Result<bool, ListError> {
    let id = toml_app_id(paths, app_id)?;
    edit_overrides(paths, |doc| {
        let removed = remove_denied(doc, id);
        let allowed = allowed_array(doc);
        let added = !allowed.iter().any(|v| v.as_integer() == Some(id));
        if added {
            allowed.push(id);
        }
        removed || added
    })
}

/// `denylist deny`: adds the game to `denied`, or updates its name and
/// reason in place, and removes it from `allowed`. Returns whether the file
/// changed.
pub fn deny(paths: &Paths, app_id: u64, name: &str, reason: &str) -> Result<bool, ListError> {
    let id = toml_app_id(paths, app_id)?;
    edit_overrides(paths, |doc| {
        let removed = remove_allowed(doc, id);
        removed | upsert_denied(doc, id, name, reason)
    })
}

/// `denylist clear` (decision 51): removes the game from both `allowed`
/// and `denied`. Returns whether the file changed.
pub fn clear(paths: &Paths, app_id: u64) -> Result<bool, ListError> {
    let id = toml_app_id(paths, app_id)?;
    edit_overrides(paths, |doc| {
        remove_allowed(doc, id) | remove_denied(doc, id)
    })
}

/// The overrides file as spela writes it, for the migration from Go. It
/// writes the lists as given: the migration first keeps an app that Go
/// listed in both only in `allowed`, and records its `denied` entry.
pub fn overrides_toml(overrides: &Overrides) -> String {
    let mut doc = DocumentMut::new();
    doc.insert("version", value(LIST_VERSION));
    let mut allowed = toml_edit::Array::new();
    for id in &overrides.allowed {
        allowed.push(i64::try_from(*id).unwrap_or(i64::MAX));
    }
    doc.insert("allowed", value(allowed));
    for entry in &overrides.denied {
        let id = i64::try_from(entry.app_id).unwrap_or(i64::MAX);
        upsert_denied(&mut doc, id, &entry.name, &entry.reason);
    }
    doc.to_string()
}

fn read(path: &Path) -> Result<Option<(PathBuf, String)>, ListError> {
    match std::fs::read_to_string(path) {
        Ok(text) => Ok(Some((path.to_owned(), text))),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(source) => Err(ListError::Read {
            path: path.to_owned(),
            source,
        }),
    }
}

/// The `version` key alone, read before the rest.
#[derive(Deserialize)]
struct VersionOnly {
    #[serde(default)]
    version: Option<i64>,
}

/// A list entry in a TOML file: no keys beyond Go's.
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictEntry {
    app_id: u64,
    #[serde(default)]
    name: String,
    #[serde(default)]
    reason: String,
}

impl From<StrictEntry> for DenyEntry {
    fn from(e: StrictEntry) -> DenyEntry {
        DenyEntry {
            app_id: e.app_id,
            name: e.name,
            reason: e.reason,
        }
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictOverrides {
    #[serde(default, rename = "version")]
    _version: Option<i64>,
    #[serde(default)]
    allowed: Vec<u64>,
    #[serde(default)]
    denied: Vec<StrictEntry>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictUserList {
    #[serde(default, rename = "version")]
    _version: Option<i64>,
    #[serde(default)]
    entries: Vec<StrictEntry>,
}

struct ParsedOverrides {
    overrides: Overrides,
    newer: Option<i64>,
}

/// Reads the file's version: missing is 1, below 1 is an error, above 1 is
/// a newer spela's file, read leniently and never written.
fn file_version(path: &Path, text: &str) -> Result<Option<i64>, ListError> {
    let version = toml::from_str::<VersionOnly>(text)
        .map_err(|e| invalid(path, e))?
        .version
        .unwrap_or(LIST_VERSION);
    match version {
        LIST_VERSION => Ok(None),
        v if v > LIST_VERSION => {
            tracing::warn!(path = %path.display(), version = v, "a newer spela wrote this deny list; reading what this version knows");
            Ok(Some(v))
        }
        v => Err(ListError::Invalid {
            path: path.to_owned(),
            message: format!("unsupported version {v}"),
        }),
    }
}

fn parse_overrides(path: &Path, text: &str) -> Result<ParsedOverrides, ListError> {
    let newer = file_version(path, text)?;
    let overrides = if newer.is_some() {
        toml::from_str::<Overrides>(text).map_err(|e| invalid(path, e))?
    } else {
        let strict: StrictOverrides = toml::from_str(text).map_err(|e| invalid(path, e))?;
        Overrides {
            allowed: strict.allowed,
            denied: strict.denied.into_iter().map(DenyEntry::from).collect(),
        }
    };
    Ok(ParsedOverrides { overrides, newer })
}

fn parse_user_list(path: &Path, text: &str) -> Result<UserList, ListError> {
    if file_version(path, text)?.is_some() {
        return toml::from_str::<UserList>(text).map_err(|e| invalid(path, e));
    }
    let strict: StrictUserList = toml::from_str(text).map_err(|e| invalid(path, e))?;
    Ok(UserList {
        entries: strict.entries.into_iter().map(DenyEntry::from).collect(),
    })
}

fn invalid(path: &Path, error: impl std::fmt::Display) -> ListError {
    ListError::Invalid {
        path: path.to_owned(),
        message: error.to_string().trim_end().to_owned(),
    }
}

fn toml_app_id(paths: &Paths, app_id: u64) -> Result<i64, ListError> {
    i64::try_from(app_id).map_err(|_| ListError::Invalid {
        path: paths.deny_overrides(),
        message: format!("app id {app_id} is too large for TOML"),
    })
}

/// One locked edit of `overrides.toml`: read fresh, refuse a file that does
/// not load or that a newer spela wrote, change only what `change` changes
/// with toml_edit so comments and order stay, check the new text, and
/// replace the file. Writes nothing when `change` changes nothing.
fn edit_overrides(
    paths: &Paths,
    change: impl FnOnce(&mut DocumentMut) -> bool,
) -> Result<bool, ListError> {
    let path = paths.deny_overrides();
    let _lock = StoreLock::acquire(&paths.config_lock())?;
    let text = read(&path)?.map(|(_, text)| text).unwrap_or_default();
    if let Some(version) = parse_overrides(&path, &text)?.newer {
        return Err(ListError::Newer { path, version });
    }
    let mut doc: DocumentMut = text.parse().map_err(|e| invalid(&path, e))?;
    if text.trim().is_empty() {
        // A new file starts with its version.
        doc.insert("version", value(LIST_VERSION));
    }
    if !change(&mut doc) {
        return Ok(false);
    }
    // Every TOML file spela writes carries its version (decision 43).
    if !doc.contains_key("version") {
        doc.insert("version", value(LIST_VERSION));
    }
    let new_text = doc.to_string();
    parse_overrides(&path, &new_text)?;
    let write_err = |source| ListError::Write {
        path: path.clone(),
        source,
    };
    std::fs::create_dir_all(paths.lists_dir()).map_err(write_err)?;
    write_atomic(&path, new_text.as_bytes(), Durability::Durable).map_err(write_err)?;
    Ok(true)
}

/// The `allowed` array, created when missing or not an array.
fn allowed_array(doc: &mut DocumentMut) -> &mut toml_edit::Array {
    let item = doc
        .entry("allowed")
        .or_insert(value(toml_edit::Array::new()));
    if item.as_array().is_none() {
        *item = value(toml_edit::Array::new());
    }
    item.as_array_mut().expect("allowed was just made an array")
}

fn remove_allowed(doc: &mut DocumentMut, id: i64) -> bool {
    let Some(allowed) = doc.get_mut("allowed").and_then(Item::as_array_mut) else {
        return false;
    };
    let before = allowed.len();
    allowed.retain(|v| v.as_integer() != Some(id));
    allowed.len() != before
}

fn has_id(app_id: Option<i64>, id: i64) -> bool {
    app_id == Some(id)
}

fn remove_denied(doc: &mut DocumentMut, id: i64) -> bool {
    match doc.get_mut("denied") {
        Some(Item::ArrayOfTables(tables)) => {
            let before = tables.len();
            tables.retain(|t| !has_id(t.get("app_id").and_then(Item::as_integer), id));
            tables.len() != before
        }
        Some(item) => match item.as_array_mut() {
            Some(array) => {
                let before = array.len();
                array.retain(|v| {
                    !has_id(
                        v.as_inline_table()
                            .and_then(|t| t.get("app_id"))
                            .and_then(toml_edit::Value::as_integer),
                        id,
                    )
                });
                array.len() != before
            }
            None => false,
        },
        None => false,
    }
}

/// Adds a `[[denied]]` entry, or updates the first one for the game in
/// place, as Go's `Deny` does. An inline `denied = [...]` list stays inline.
fn upsert_denied(doc: &mut DocumentMut, id: i64, name: &str, reason: &str) -> bool {
    if let Some(array) = doc.get_mut("denied").and_then(Item::as_array_mut) {
        for v in array.iter_mut() {
            if let Some(t) = v.as_inline_table_mut()
                && has_id(t.get("app_id").and_then(toml_edit::Value::as_integer), id)
            {
                let same = t.get("name").and_then(toml_edit::Value::as_str) == Some(name)
                    && t.get("reason").and_then(toml_edit::Value::as_str) == Some(reason);
                t.insert("name", name.into());
                t.insert("reason", reason.into());
                return !same;
            }
        }
        let mut t = InlineTable::new();
        t.insert("app_id", id.into());
        t.insert("name", name.into());
        t.insert("reason", reason.into());
        array.push(t);
        return true;
    }
    if !matches!(doc.get("denied"), Some(Item::ArrayOfTables(_))) {
        doc.insert(
            "denied",
            Item::ArrayOfTables(toml_edit::ArrayOfTables::new()),
        );
    }
    let Some(Item::ArrayOfTables(tables)) = doc.get_mut("denied") else {
        unreachable!("denied was just made an array of tables");
    };
    for t in tables.iter_mut() {
        if has_id(t.get("app_id").and_then(Item::as_integer), id) {
            let same = t.get("name").and_then(Item::as_str) == Some(name)
                && t.get("reason").and_then(Item::as_str) == Some(reason);
            t["name"] = value(name);
            t["reason"] = value(reason);
            return !same;
        }
    }
    let mut t = Table::new();
    t["app_id"] = value(id);
    t["name"] = value(name);
    t["reason"] = value(reason);
    tables.push(t);
    true
}

#[cfg(test)]
mod tests {
    use super::*;

    fn setup() -> (tempfile::TempDir, Paths) {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        (base, paths)
    }

    fn write(path: &Path, text: &str) {
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, text).unwrap();
    }

    fn entry(app_id: u64, name: &str, reason: &str) -> DenyEntry {
        DenyEntry {
            app_id,
            name: name.into(),
            reason: reason.into(),
        }
    }

    #[test]
    fn with_no_files_the_built_in_list_applies() {
        let (_base, paths) = setup();
        let lists = load(&paths).unwrap();
        assert_eq!(lists.anti_cheat().len(), 10);
        for (app_id, _, reason) in BUILT_IN {
            assert_eq!(lists.check(app_id).reason(), Some(reason));
        }
        assert_eq!(lists.check(9100010), Status::Allowed);
        assert_eq!(
            guard_swap(&paths, 252490).unwrap_err().to_string(),
            "DLL swap denied for app 252490: Easy Anti-Cheat"
        );
        assert!(guard_swap(&paths, 9100010).is_ok());
        assert_eq!(lists.anti_cheat()[6].entry.name, "PUBG: Battlegrounds");
    }

    #[test]
    fn the_user_list_adds_to_the_built_in_list() {
        // Decision 36. Go replaced the built-in list with the user's.
        let user = UserList {
            entries: vec![
                entry(9100201, "Frostline Arena", "kernel anti-cheat"),
                entry(252490, "Rust", "my own reason"),
            ],
        };
        let lists = DenyLists::new(user, Overrides::default());
        assert!(lists.check(1172470).is_denied(), "built-in stays");
        assert_eq!(lists.check(9100201).reason(), Some("kernel anti-cheat"));
        let rust = lists.check(252490);
        assert_eq!(rust.reason(), Some("my own reason"));
        assert!(matches!(
            rust,
            Status::Denied(Listed {
                source: Source::UserList,
                ..
            })
        ));
        let shown = lists.anti_cheat();
        assert_eq!(shown.len(), 11, "Rust appears once");
        assert_eq!(shown[8].entry.reason, "my own reason");
        assert_eq!(shown[10].entry.app_id, 9100201);
    }

    #[test]
    fn allow_wins_over_every_list_then_denied_then_user_then_built_in() {
        let overrides = Overrides {
            allowed: vec![1172470, 9100203],
            denied: vec![
                entry(9100203, "Both Lists", "user-specified"),
                entry(1623660, "Escape from Tarkov", "user reason"),
            ],
        };
        let lists = DenyLists::new(UserList::default(), overrides);
        match lists.check(1172470) {
            Status::ForceAllowed { over: Some(lifted) } => {
                assert_eq!(lifted.source, Source::BuiltIn);
                assert_eq!(lifted.entry.name, "Apex Legends");
            }
            other => panic!("{other:?}"),
        }
        assert!(matches!(
            lists.check(9100203),
            Status::ForceAllowed {
                over: Some(Listed {
                    source: Source::UserDenied,
                    ..
                })
            }
        ));
        let tarkov = lists.check(1623660);
        assert_eq!(tarkov.reason(), Some("user reason"));
        assert!(lists.guard(1172470).is_ok());
        assert!(lists.guard(1623660).is_err());
    }

    #[test]
    fn a_broken_or_unreadable_list_fails_closed() {
        let (_base, paths) = setup();
        write(&paths.deny_overrides(), "allowed = [1172470\n");
        let err = guard_swap(&paths, 9100010).unwrap_err();
        assert!(matches!(
            err,
            SwapRefused::Unreadable(ListError::Invalid { .. })
        ));
        let text = err.to_string();
        assert!(
            text.starts_with("DLL install and update are refused until the deny list loads: ")
                && text.contains("overrides.toml is not a valid deny list"),
            "{text}"
        );

        write(
            &paths.deny_overrides(),
            "version = 1\n[[denid]]\napp_id = 1\n",
        );
        assert!(
            load(&paths)
                .unwrap_err()
                .to_string()
                .contains("unknown field `denid`")
        );
        write(&paths.deny_overrides(), "allowed = [-1]\n");
        assert!(load(&paths).is_err());
        write(&paths.deny_overrides(), "[[denied]]\nname = \"x\"\n");
        assert!(load(&paths).unwrap_err().to_string().contains("app_id"));
        write(&paths.deny_overrides(), "version = 0\n");
        assert!(
            load(&paths)
                .unwrap_err()
                .to_string()
                .contains("unsupported version 0")
        );

        std::fs::remove_file(paths.deny_overrides()).unwrap();
        write(&paths.user_denylist(), "[[entries]]\napp_id = \"x\"\n");
        assert!(
            load(&paths)
                .unwrap_err()
                .to_string()
                .contains("denylist.toml")
        );
        std::fs::remove_file(paths.user_denylist()).unwrap();
        std::fs::create_dir_all(paths.user_denylist()).unwrap();
        assert!(matches!(load(&paths), Err(ListError::Read { .. })));
    }

    #[test]
    fn a_newer_file_is_read_but_never_written() {
        let (_base, paths) = setup();
        write(
            &paths.deny_overrides(),
            "version = 2\nallowed = [1172470]\nsomething_new = true\n",
        );
        let lists = load(&paths).unwrap();
        assert!(lists.is_force_allowed(1172470));
        assert!(matches!(
            allow(&paths, 5),
            Err(ListError::Newer { version: 2, .. })
        ));
    }

    #[test]
    fn allow_and_deny_clear_each_other_and_keep_comments() {
        let (_base, paths) = setup();
        assert!(allow(&paths, 1172470).unwrap());
        assert!(!allow(&paths, 1172470).unwrap(), "idempotent");
        let text = std::fs::read_to_string(paths.deny_overrides()).unwrap();
        assert_eq!(text, "version = 1\nallowed = [1172470]\n");

        let mut text = text;
        text.insert_str(0, "# my notes\n");
        write(&paths.deny_overrides(), &text);
        assert!(deny(&paths, 1172470, "Apex Legends", DEFAULT_REASON).unwrap());
        assert!(deny(&paths, 9100202, "9100202", DEFAULT_REASON).unwrap());
        assert!(!deny(&paths, 9100202, "9100202", DEFAULT_REASON).unwrap());
        assert!(deny(&paths, 9100202, "Pocket Raid", "updated").unwrap());
        let text = std::fs::read_to_string(paths.deny_overrides()).unwrap();
        assert!(
            text.starts_with("# my notes\nversion = 1\nallowed = []\n"),
            "{text}"
        );
        // A hand-written file without a version gets one on its next write.
        let other = tempfile::tempdir().unwrap();
        let hand = Paths::under(other.path());
        write(&hand.deny_overrides(), "# mine\nallowed = [1]\n");
        assert!(!allow(&hand, 1).unwrap(), "no change, no write");
        assert_eq!(
            std::fs::read_to_string(hand.deny_overrides()).unwrap(),
            "# mine\nallowed = [1]\n"
        );
        assert!(allow(&hand, 2).unwrap());
        assert_eq!(
            std::fs::read_to_string(hand.deny_overrides()).unwrap(),
            "# mine\nallowed = [1, 2]\nversion = 1\n"
        );

        let lists = load(&paths).unwrap();
        assert!(lists.check(1172470).is_denied(), "deny cleared the allow");
        assert_eq!(
            lists.user_denied()[1],
            entry(9100202, "Pocket Raid", "updated")
        );

        assert!(allow(&paths, 9100202).unwrap());
        let lists = load(&paths).unwrap();
        assert_eq!(lists.check(9100202), Status::ForceAllowed { over: None });
        assert_eq!(lists.user_denied().len(), 1, "allow cleared the deny");

        assert!(clear(&paths, 9100202).unwrap());
        assert!(clear(&paths, 1172470).unwrap());
        assert!(!clear(&paths, 1172470).unwrap());
        let lists = load(&paths).unwrap();
        assert_eq!(lists.overrides(), &Overrides::default());
        assert!(
            lists.check(1172470).is_denied(),
            "back to the built-in entry"
        );
    }

    #[test]
    fn edits_refuse_a_broken_file_and_handle_inline_lists() {
        let (_base, paths) = setup();
        write(&paths.deny_overrides(), "allowed = [\n");
        assert!(matches!(
            deny(&paths, 1, "x", "y"),
            Err(ListError::Invalid { .. })
        ));
        assert_eq!(
            std::fs::read_to_string(paths.deny_overrides()).unwrap(),
            "allowed = [\n",
            "a broken file is left as it is"
        );

        write(
            &paths.deny_overrides(),
            "denied = [{ app_id = 7, name = \"A\", reason = \"r\" }]\n",
        );
        assert!(deny(&paths, 8, "B", "s").unwrap());
        assert!(deny(&paths, 7, "A", "new").unwrap());
        let lists = load(&paths).unwrap();
        assert_eq!(
            lists.user_denied(),
            [entry(7, "A", "new"), entry(8, "B", "s")]
        );
        assert!(allow(&paths, 7).unwrap());
        assert_eq!(load(&paths).unwrap().user_denied(), [entry(8, "B", "s")]);
    }

    #[test]
    fn overrides_toml_reads_back_as_written() {
        let overrides = Overrides {
            allowed: vec![1172470, 9100203],
            denied: vec![
                entry(9100201, "Frostline Arena", "kernel anti-cheat, updated"),
                entry(9100202, "9100202", "user-specified"),
            ],
        };
        let text = overrides_toml(&overrides);
        assert!(
            text.starts_with("version = 1\nallowed = [1172470, 9100203]\n\n[[denied]]\n"),
            "{text}"
        );
        let parsed = parse_overrides(Path::new("overrides.toml"), &text).unwrap();
        assert_eq!(parsed.overrides, overrides);
    }
}
