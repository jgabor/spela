//! The TOML profile files: `profiles/all-games.toml` and
//! `profiles/<appid>.toml`, read into [`super::OwnValues`] and written with
//! toml_edit under `config.lock`.
//!
//! A file stores only the values its scope sets itself: a key that is
//! present is an own value, a key that is absent follows. Every file spela
//! writes carries `version = 1`; a game file also carries `name`.
//!
//! Reading validates every value against the field table and reports each
//! problem with its line and column. An unknown key is a warning and stays
//! in the file. A file with a higher `version` is read only.
//!
//! Every write is one transaction: it takes `config.lock` on a fresh
//! descriptor, reads the file fresh, changes only the keys it means to
//! change with toml_edit, so comments, order and unknown keys survive,
//! parses the new text again, and replaces the file atomically with fsync.
//! A file that does not load is never written over.

use std::io;
use std::ops::Range;
use std::path::{Path, PathBuf};

use toml::de::{DeTable, DeValue};
use toml_edit::{DocumentMut, Item, Key, Table, TableLike};

use super::field::{FieldError, FieldKey, Section};
use super::value::{Kind, Value};
use super::{AllGamesProfile, FollowOutcome, GameProfile, OwnValues, Scope};
use crate::config::{Diagnostic, closest, line_col};
use crate::fs::{Durability, write_atomic};
use crate::lock::{LockError, StoreLock};
use crate::paths::Paths;

/// The profile format version this spela writes and reads fully.
pub const PROFILE_VERSION: i64 = 1;

/// Whether a game has its own profile, the ⚑ marker: its file exists, even
/// when it holds no own values.
pub fn has_profile(paths: &Paths, app_id: u64) -> bool {
    paths.game_profile(app_id).is_file()
}

/// A profile read from its file, with what the reader noticed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Loaded<P> {
    /// The profile.
    pub profile: P,
    /// Unknown keys and other things that do not stop spela, in file order.
    pub warnings: Vec<Diagnostic>,
    /// The file has a newer `version` than this spela knows, so spela must
    /// not write it.
    pub read_only: bool,
}

/// Why a profile could not be read or written.
#[derive(Debug, thiserror::Error)]
pub enum ProfileError {
    /// The file or its folder cannot be read or written.
    #[error("profile {}: {source}", path.display())]
    Io {
        /// The file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
    /// The file is not a valid profile. Nothing is written over it.
    #[error("{}", format_invalid(path, errors))]
    Invalid {
        /// The file.
        path: PathBuf,
        /// Every problem found, in file order.
        errors: Vec<Diagnostic>,
    },
    /// The file has a newer `version`, so this spela does not change it.
    #[error("profile {} was written by a newer spela, so this spela does not change it", path.display())]
    ReadOnly {
        /// The file.
        path: PathBuf,
    },
    /// A value the field cannot hold.
    #[error(transparent)]
    Field(#[from] FieldError),
    /// `config.lock` could not be taken.
    #[error(transparent)]
    Lock(#[from] LockError),
}

fn format_invalid(path: &Path, errors: &[Diagnostic]) -> String {
    let mut out = format!("profile {} has errors:", path.display());
    for e in errors {
        out.push_str("\n  ");
        out.push_str(&e.to_string());
    }
    out
}

fn io_error(path: &Path) -> impl FnOnce(io::Error) -> ProfileError + '_ {
    move |source| ProfileError::Io {
        path: path.to_owned(),
        source,
    }
}

// Reading.

/// What a profile file holds before it is tied to a scope.
#[derive(Debug, Clone, PartialEq, Eq)]
struct Parsed {
    name: String,
    values: OwnValues,
    warnings: Vec<Diagnostic>,
    read_only: bool,
}

/// Parses the text of a game's profile.
pub fn parse_game(app_id: u64, text: &str) -> Result<Loaded<GameProfile>, Vec<Diagnostic>> {
    let parsed = parse(text, true)?;
    Ok(Loaded {
        profile: GameProfile {
            app_id,
            name: parsed.name,
            values: parsed.values,
        },
        warnings: parsed.warnings,
        read_only: parsed.read_only,
    })
}

/// Parses the text of the All games profile.
pub fn parse_all_games(text: &str) -> Result<Loaded<AllGamesProfile>, Vec<Diagnostic>> {
    let parsed = parse(text, false)?;
    Ok(Loaded {
        profile: AllGamesProfile {
            values: parsed.values,
        },
        warnings: parsed.warnings,
        read_only: parsed.read_only,
    })
}

fn parse(text: &str, game: bool) -> Result<Parsed, Vec<Diagnostic>> {
    let doc = DeTable::parse(text).map_err(|e| vec![from_toml_error(text, &e)])?;
    let doc = doc.get_ref();
    let mut errors = Vec::new();
    let mut warnings = Vec::new();
    let mut parsed = Parsed {
        name: String::new(),
        values: OwnValues::new(),
        warnings: Vec::new(),
        read_only: false,
    };
    let mut known_top: Vec<&str> = vec!["version"];
    if game {
        known_top.push("name");
    }
    known_top.extend(Section::ALL.iter().map(|s| s.name()));

    for (key, value) in doc.iter() {
        let name: &str = key.get_ref();
        let span = value.span();
        match (name, value.get_ref()) {
            ("version", DeValue::Integer(n)) => match i64::from_str_radix(n.as_str(), n.radix()) {
                Ok(v) if v > PROFILE_VERSION => {
                    parsed.read_only = true;
                    warnings.push(at(
                            text,
                            span,
                            format!(
                                "profile version {v} is newer than this spela knows ({PROFILE_VERSION}), so spela reads it but never writes it"
                            ),
                        ));
                }
                Ok(v) if v >= 1 => {}
                _ => errors.push(at(
                    text,
                    span,
                    format!("`version` is {n}, but must be 1 or higher"),
                )),
            },
            ("version", _) => errors.push(at(text, span, "`version` must be a number".into())),
            ("name", DeValue::String(s)) if game => parsed.name = s.to_string(),
            ("name", _) if game => errors.push(at(text, span, "`name` must be text".into())),
            (section_name, inner) => match Section::from_name(section_name) {
                Some(section) => match inner {
                    DeValue::Table(table) => {
                        read_section(
                            text,
                            section,
                            table,
                            &mut parsed.values,
                            &mut errors,
                            &mut warnings,
                        );
                    }
                    _ => errors.push(at(
                        text,
                        span,
                        format!("`{section_name}` must be a table of settings"),
                    )),
                },
                None => warnings.push(unknown(text, key.span(), "", section_name, &known_top)),
            },
        }
    }
    if !errors.is_empty() {
        errors.sort_by_key(|d| (d.line, d.column));
        return Err(errors);
    }
    warnings.sort_by_key(|d| (d.line, d.column));
    parsed.warnings = warnings;
    Ok(parsed)
}

fn read_section(
    text: &str,
    section: Section,
    table: &DeTable<'_>,
    values: &mut OwnValues,
    errors: &mut Vec<Diagnostic>,
    warnings: &mut Vec<Diagnostic>,
) {
    let known: Vec<&str> = section.fields().map(|f| f.name()).collect();
    for (key, value) in table.iter() {
        let name: &str = key.get_ref();
        let Some(field) = section.fields().find(|f| f.name() == name) else {
            let prefix = format!("{}.", section.name());
            warnings.push(unknown(text, key.span(), &prefix, name, &known));
            continue;
        };
        let typed = match (field.kind, value.get_ref()) {
            (Kind::Bool, DeValue::Boolean(b)) => Some(Value::Bool(*b)),
            (Kind::OptionalBool, DeValue::Boolean(b)) => Some(Value::OptionalBool(Some(*b))),
            (Kind::Int, DeValue::Integer(n)) => i64::from_str_radix(n.as_str(), n.radix())
                .ok()
                .map(Value::Int),
            (Kind::Text, DeValue::String(s)) => Some(Value::Text(s.to_string())),
            _ => None,
        };
        let result = match typed {
            Some(v) => values.set(field.id, v).map(|_| ()),
            None => Err(FieldError::WrongKind {
                key: field.key,
                expected: field.kind,
            }),
        };
        if let Err(e) = result {
            errors.push(at(text, value.span(), e.to_string()));
        }
    }
}

fn unknown(text: &str, span: Range<usize>, prefix: &str, name: &str, known: &[&str]) -> Diagnostic {
    let mut message = format!("unknown key `{prefix}{name}` is ignored");
    if let Some(guess) = closest(name, known) {
        message.push_str(&format!(" (did you mean `{prefix}{guess}`?)"));
    }
    at(text, span, message)
}

fn at(text: &str, span: Range<usize>, message: String) -> Diagnostic {
    let (line, column) = line_col(text, span.start);
    Diagnostic {
        line,
        column,
        message,
    }
}

fn from_toml_error(text: &str, e: &toml::de::Error) -> Diagnostic {
    let (line, column) = e.span().map(|s| line_col(text, s.start)).unwrap_or((0, 0));
    Diagnostic {
        line,
        column,
        message: e.message().trim().to_owned(),
    }
}

fn read_text(path: &Path) -> Result<Option<String>, ProfileError> {
    match std::fs::read_to_string(path) {
        Ok(text) => Ok(Some(text)),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(e) => Err(io_error(path)(e)),
    }
}

/// A game's profile, or `None` when it has no file. Readers never lock.
pub fn load_game(paths: &Paths, app_id: u64) -> Result<Option<Loaded<GameProfile>>, ProfileError> {
    let path = paths.game_profile(app_id);
    match read_text(&path)? {
        None => Ok(None),
        Some(text) => parse_game(app_id, &text)
            .map(Some)
            .map_err(|errors| ProfileError::Invalid { path, errors }),
    }
}

/// The All games profile, or `None` when it has no file.
pub fn load_all_games(paths: &Paths) -> Result<Option<Loaded<AllGamesProfile>>, ProfileError> {
    let path = paths.all_games_profile();
    match read_text(&path)? {
        None => Ok(None),
        Some(text) => parse_all_games(&text)
            .map(Some)
            .map_err(|errors| ProfileError::Invalid { path, errors }),
    }
}

/// Every game profile on disk.
#[derive(Debug, Default)]
pub struct Listing {
    /// The profiles that load, sorted by app id.
    pub profiles: Vec<Loaded<GameProfile>>,
    /// The game profiles that do not load, sorted by app id. Go skipped
    /// these silently; spela names them.
    pub broken: Vec<(u64, ProfileError)>,
}

/// Lists the game profiles: files named `<appid>.toml`, where the stem is
/// the app id as spela writes it. `all-games.toml`, other names and folders
/// are not games. A missing profile folder is an empty list.
pub fn list(paths: &Paths) -> Result<Listing, ProfileError> {
    let dir = paths.profiles_dir();
    let entries = match std::fs::read_dir(&dir) {
        Ok(entries) => entries,
        Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(Listing::default()),
        Err(e) => return Err(io_error(&dir)(e)),
    };
    let mut ids = Vec::new();
    for entry in entries {
        let entry = entry.map_err(io_error(&dir))?;
        let name = entry.file_name();
        let Some(app_id) = name.to_str().and_then(game_file_app_id) else {
            continue;
        };
        if entry.path().is_file() {
            ids.push(app_id);
        }
    }
    ids.sort_unstable();
    let mut listing = Listing::default();
    for app_id in ids {
        match load_game(paths, app_id) {
            Ok(Some(loaded)) => listing.profiles.push(loaded),
            Ok(None) => {}
            Err(e) => listing.broken.push((app_id, e)),
        }
    }
    Ok(listing)
}

/// The app id of a game profile file name, `<appid>.toml`. A leading zero
/// or any other spelling is not a game profile, because spela only ever
/// opens the canonical name.
pub fn game_file_app_id(file_name: &str) -> Option<u64> {
    let stem = file_name.strip_suffix(".toml")?;
    let app_id: u64 = stem.parse().ok()?;
    (app_id.to_string() == stem).then_some(app_id)
}

// Writing.

/// The text of a new profile file: an optional header comment, `version`,
/// the game's `name` when it has one, and each own value in table order.
pub fn render(name: Option<&str>, values: &OwnValues, header: &[String]) -> String {
    let mut doc = DocumentMut::new();
    doc.insert("version", toml_edit::value(PROFILE_VERSION));
    if let Some(name) = name.filter(|n| !n.is_empty()) {
        doc.insert("name", toml_edit::value(name));
    }
    for section in Section::ALL {
        let mut own = values.in_section(section).peekable();
        if own.peek().is_none() {
            continue;
        }
        let mut table = Table::new();
        for (key, value) in own {
            table.insert(key.field().name(), Item::Value(toml_value(value)));
        }
        doc.insert(section.name(), Item::Table(table));
    }
    let mut text = String::new();
    for line in header {
        if line.is_empty() {
            text.push_str("#\n");
        } else {
            text.push_str("# ");
            text.push_str(line);
            text.push('\n');
        }
    }
    text.push_str(&doc.to_string());
    text
}

fn toml_value(value: &Value) -> toml_edit::Value {
    match value {
        Value::Bool(b) | Value::OptionalBool(Some(b)) => (*b).into(),
        Value::Int(n) => (*n).into(),
        Value::Text(t) => t.as_str().into(),
        // OwnValues never holds none; an empty string keeps TOML valid.
        Value::OptionalBool(None) => "".into(),
    }
}

/// Writes profile text after checking that it reads back. The folder is
/// created when missing.
pub(crate) fn write_checked(path: &Path, text: &str, game: bool) -> Result<(), ProfileError> {
    if let Err(errors) = parse(text, game) {
        return Err(ProfileError::Invalid {
            path: path.to_owned(),
            errors,
        });
    }
    if let Some(dir) = path.parent() {
        std::fs::create_dir_all(dir).map_err(io_error(dir))?;
    }
    write_atomic(path, text.as_bytes(), Durability::Durable).map_err(io_error(path))
}

/// An open profile document inside one write transaction.
struct Editor {
    doc: DocumentMut,
    name: String,
    values: OwnValues,
    changed: bool,
}

impl Editor {
    fn set(&mut self, key: FieldKey, value: Value) -> Result<Option<Value>, ProfileError> {
        let mut values = self.values.clone();
        let previous = values.set(key, value.clone())?;
        if previous.as_ref() == Some(&value) {
            return Ok(previous);
        }
        self.values = values;
        let field = key.field();
        let section = field.section.name();
        if self
            .doc
            .get(section)
            .and_then(Item::as_table_like)
            .is_none()
        {
            self.doc.remove(section);
            insert_section(&mut self.doc, field.section);
        }
        let new = toml_value(&value);
        match self.doc.get_mut(section) {
            Some(Item::Table(table)) => {
                if !replace_value(table, field.name(), &new) {
                    let index = field_index(table.iter().map(|(k, _)| k), key);
                    if table.is_dotted() {
                        table.insert(field.name(), Item::Value(new));
                    } else {
                        insert_at(
                            table,
                            index,
                            Key::new(field.name()),
                            Item::Value(new),
                            false,
                        );
                    }
                }
            }
            Some(Item::Value(toml_edit::Value::InlineTable(table))) => {
                if !replace_value(table, field.name(), &new) {
                    table.insert(field.name(), new);
                }
            }
            _ => unreachable!("the section table was just made"),
        }
        self.changed = true;
        Ok(previous)
    }

    fn follow(&mut self, key: FieldKey) -> Option<Value> {
        let previous = self.values.follow(key)?;
        let field = key.field();
        let section = field.section.name();
        let empty = match self.doc.get_mut(section).and_then(Item::as_table_like_mut) {
            Some(table) => {
                table.remove(field.name());
                table.is_empty()
            }
            None => false,
        };
        if empty && !has_comment(self.doc.get(section)) {
            self.doc.remove(section);
        }
        self.changed = true;
        Some(previous)
    }

    fn fill_name(&mut self, name: &str) {
        if !self.name.is_empty() || name.is_empty() {
            return;
        }
        self.name = name.to_owned();
        let root = self.doc.as_table_mut();
        let new: toml_edit::Value = name.into();
        if !replace_value(root, "name", &new) {
            let index = usize::from(root.contains_key("version"));
            insert_at(root, index, Key::new("name"), Item::Value(new), true);
        }
        self.changed = true;
    }
}

/// Replaces the value of an existing key, keeping the comment after it.
/// Returns `false` when the key is not there.
fn replace_value(table: &mut dyn TableLike, name: &str, new: &toml_edit::Value) -> bool {
    match table.get_mut(name).and_then(Item::as_value_mut) {
        Some(old) => {
            let decor = old.decor().clone();
            *old = new.clone();
            *old.decor_mut() = decor;
            true
        }
        None => false,
    }
}

fn has_comment(item: Option<&Item>) -> bool {
    let decor = match item {
        Some(Item::Table(t)) => t.decor(),
        _ => return false,
    };
    [decor.prefix(), decor.suffix()]
        .into_iter()
        .flatten()
        .any(|raw| raw.as_str().is_some_and(|s| s.contains('#')))
}

/// Adds an empty section table so it prints between the sections before
/// and after it in [`Section::ALL`]. Every root table gets a position in
/// the new order; the rest of the file is untouched.
fn insert_section(doc: &mut DocumentMut, section: Section) {
    let root = doc.as_table_mut();
    // The display order toml_edit uses: a table without a position takes
    // the position of the one before it.
    let mut order: Vec<(isize, usize, String)> = Vec::new();
    let mut last = 0;
    for (i, (key, item)) in root.iter().enumerate() {
        if let Item::Table(t) = item {
            if let Some(p) = t.position() {
                last = p;
            }
            order.push((last, i, key.to_owned()));
        }
    }
    order.sort();
    let mut names: Vec<String> = order.into_iter().map(|(_, _, k)| k).collect();
    let rank = |name: &str| Section::from_name(name).map(|s| s as usize);
    let me = section as usize;
    let index = names
        .iter()
        .rposition(|n| rank(n).is_some_and(|r| r < me))
        .map(|i| i + 1)
        .or_else(|| names.iter().position(|n| rank(n).is_some_and(|r| r > me)))
        .unwrap_or(names.len());
    names.insert(index, section.name().to_owned());
    root.insert(section.name(), Item::Table(Table::new()));
    for (position, name) in names.iter().enumerate() {
        if let Some(Item::Table(t)) = root.get_mut(name) {
            t.set_position(Some(position as isize + 1));
        }
    }
}

/// Where a field's key goes among the keys of its section: after the last
/// field before it in table order, else before the first field after it.
/// Unknown keys keep their places.
fn field_index<'a>(keys: impl Iterator<Item = &'a str>, key: FieldKey) -> usize {
    let section = key.field().section;
    let rank = |k: &str| section.fields().find(|f| f.name() == k).map(|f| f.id);
    let keys: Vec<&str> = keys.collect();
    keys.iter()
        .rposition(|k| rank(k).is_some_and(|r| r < key))
        .map(|i| i + 1)
        .or_else(|| keys.iter().position(|k| rank(k).is_some_and(|r| r > key)))
        .unwrap_or(keys.len())
}

/// Inserts `key` at `index` in `table`, moving the entries after it so
/// each keeps its formatting and comments. With `header`, a key put first
/// takes over the comment above the old first entry, so a file's header
/// comment stays on top.
fn insert_at(table: &mut Table, index: usize, mut key: Key, item: Item, header: bool) {
    let names: Vec<String> = table.iter().map(|(k, _)| k.to_owned()).collect();
    let mut moved = Vec::new();
    for name in names.iter().skip(index) {
        if let Some(entry) = table.remove_entry(name) {
            moved.push(entry);
        }
    }
    if header
        && index == 0
        && let Some((first_key, first_item)) = moved.first_mut()
    {
        let decor = match first_item {
            Item::Table(t) => t.decor_mut(),
            _ => first_key.leaf_decor_mut(),
        };
        if let Some(prefix) = decor.prefix().cloned()
            && prefix.as_str().is_some_and(|s| s.contains('#'))
        {
            decor.set_prefix("");
            key.leaf_decor_mut().set_prefix(prefix);
        }
    }
    table.insert_formatted(&key, item);
    for (k, v) in moved {
        table.insert_formatted(&k, v);
    }
}

/// Runs one transaction on the profile at `path`: read fresh, change,
/// check, replace. `create` makes a missing file; otherwise a missing file
/// gives `None` and nothing is written. The file is written only when
/// `change` changed something, or when it is new. The caller holds
/// `config.lock`.
fn transact<T>(
    path: &Path,
    game: Option<(u64, &str)>,
    create: bool,
    change: impl FnOnce(&mut Editor) -> Result<T, ProfileError>,
) -> Result<Option<(T, bool)>, ProfileError> {
    let is_game = game.is_some();
    let (mut editor, existed) = match read_text(path)? {
        Some(text) => {
            let parsed = parse(&text, is_game).map_err(|errors| ProfileError::Invalid {
                path: path.to_owned(),
                errors,
            })?;
            if parsed.read_only {
                return Err(ProfileError::ReadOnly {
                    path: path.to_owned(),
                });
            }
            let doc: DocumentMut =
                text.parse()
                    .map_err(|e: toml_edit::TomlError| ProfileError::Invalid {
                        path: path.to_owned(),
                        errors: vec![Diagnostic {
                            line: 0,
                            column: 0,
                            message: e.message().to_owned(),
                        }],
                    })?;
            let editor = Editor {
                doc,
                name: parsed.name,
                values: parsed.values,
                changed: false,
            };
            (editor, true)
        }
        None if !create => return Ok(None),
        None => {
            let text = render(None, &OwnValues::new(), &[]);
            let editor = Editor {
                doc: text.parse().expect("a new profile parses"),
                name: String::new(),
                values: OwnValues::new(),
                changed: true,
            };
            (editor, false)
        }
    };
    if let Some((_, name)) = game {
        editor.fill_name(name);
    }
    let result = change(&mut editor)?;
    if existed && !editor.changed {
        return Ok(Some((result, false)));
    }
    let root = editor.doc.as_table_mut();
    if !root.contains_key("version") {
        insert_at(
            root,
            0,
            Key::new("version"),
            toml_edit::value(PROFILE_VERSION),
            true,
        );
    }
    write_checked(path, &editor.doc.to_string(), is_game)?;
    Ok(Some((result, true)))
}

fn scope_path(paths: &Paths, scope: Scope) -> PathBuf {
    match scope {
        Scope::AllGames => paths.all_games_profile(),
        Scope::Game(app_id) => paths.game_profile(app_id),
    }
}

fn lock(paths: &Paths) -> Result<StoreLock, ProfileError> {
    Ok(StoreLock::acquire(&paths.config_lock())?)
}

/// What a write did.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Written {
    /// The file did not exist and was made.
    pub created: bool,
    /// The file was written. `false` when nothing changed.
    pub written: bool,
    /// Each changed field with the own value it had before, `None` where
    /// it followed. Fields whose value did not change are left out.
    pub previous: Vec<(FieldKey, Option<Value>)>,
}

/// Creates an empty profile for a game, with its name, when it has none.
/// Returns `false` and writes nothing when one exists.
pub fn create(paths: &Paths, app_id: u64, name: &str) -> Result<bool, ProfileError> {
    let _lock = lock(paths)?;
    let path = paths.game_profile(app_id);
    if path.exists() {
        return Ok(false);
    }
    let text = render(Some(name), &OwnValues::new(), &[]);
    write_checked(&path, &text, true)?;
    Ok(true)
}

/// Deletes a game's profile. Returns `false` when there was none, so the
/// caller can say so instead of reporting a deletion.
pub fn delete(paths: &Paths, app_id: u64) -> Result<bool, ProfileError> {
    let _lock = lock(paths)?;
    let path = paths.game_profile(app_id);
    match std::fs::remove_file(&path) {
        Ok(()) => Ok(true),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(false),
        Err(e) => Err(io_error(&path)(e)),
    }
}

/// Sets own values in one write. A game's file is created when missing, as
/// Go's setters do, and its `name` is filled with `name` when empty. The
/// All games file is created when missing. Every value is checked first;
/// on an error nothing is written.
pub fn set(
    paths: &Paths,
    scope: Scope,
    name: &str,
    values: &[(FieldKey, Value)],
) -> Result<Written, ProfileError> {
    let _lock = lock(paths)?;
    let path = scope_path(paths, scope);
    let existed = path.exists();
    let game = match scope {
        Scope::Game(app_id) => Some((app_id, name)),
        Scope::AllGames => None,
    };
    let (previous, written) = transact(&path, game, true, |editor| {
        let mut previous = Vec::new();
        for (key, value) in values {
            let before = editor.values.get(*key).cloned();
            if before.as_ref() != Some(value) {
                editor.set(*key, value.clone())?;
                previous.push((*key, before));
            }
        }
        Ok(previous)
    })?
    .expect("set creates the file");
    Ok(Written {
        created: !existed,
        written,
        previous,
    })
}

/// Sets some own values and makes other fields follow, in one write, as a
/// `set` command with several flags does. The file is created only when
/// there is a value to set; following alone never creates one. A game's
/// `name` is filled with `name` when empty. Every value is checked first;
/// on an error nothing is written. A key in both lists ends up following.
pub fn edit(
    paths: &Paths,
    scope: Scope,
    name: &str,
    values: &[(FieldKey, Value)],
    follow: &[FieldKey],
) -> Result<Written, ProfileError> {
    let _lock = lock(paths)?;
    let path = scope_path(paths, scope);
    let existed = path.exists();
    let game = match scope {
        Scope::Game(app_id) => Some((app_id, name)),
        Scope::AllGames => None,
    };
    let done = transact(&path, game, !values.is_empty(), |editor| {
        let mut previous: Vec<(FieldKey, Option<Value>)> = Vec::new();
        let mut note = |key: FieldKey, before: Option<Value>| {
            if !previous.iter().any(|(k, _)| *k == key) {
                previous.push((key, before));
            }
        };
        for (key, value) in values {
            let before = editor.values.get(*key).cloned();
            if before.as_ref() != Some(value) {
                editor.set(*key, value.clone())?;
                note(*key, before);
            }
        }
        for key in follow {
            if let Some(before) = editor.follow(*key) {
                note(*key, Some(before));
            }
        }
        // A key set and then followed in one call did not change.
        let after = editor.values.clone();
        previous.retain(|(key, before)| after.get(*key) != before.as_ref());
        Ok(previous)
    })?;
    Ok(match done {
        Some((previous, written)) => Written {
            created: !existed && written,
            written,
            previous,
        },
        None => Written {
            created: false,
            written: false,
            previous: Vec::new(),
        },
    })
}

/// Makes fields follow in one write: on a game they follow All games, on
/// All games the system default. Never creates a file: `None` when the
/// scope has no file. Returns the own values that were removed.
pub fn follow(
    paths: &Paths,
    scope: Scope,
    keys: &[FieldKey],
) -> Result<Option<Vec<(FieldKey, Value)>>, ProfileError> {
    let _lock = lock(paths)?;
    follow_unlocked(&scope_path(paths, scope), scope, keys)
}

fn follow_unlocked(
    path: &Path,
    scope: Scope,
    keys: &[FieldKey],
) -> Result<Option<Vec<(FieldKey, Value)>>, ProfileError> {
    let game = match scope {
        Scope::Game(app_id) => Some((app_id, "")),
        Scope::AllGames => None,
    };
    let done = transact(path, game, false, |editor| {
        Ok(keys
            .iter()
            .filter_map(|k| editor.follow(*k).map(|v| (*k, v)))
            .collect::<Vec<_>>())
    })?;
    Ok(done.map(|(removed, _)| removed))
}

/// Makes every field of a scope follow, as `profile reset` does. The file
/// and its name stay. `None` when the scope has no file; otherwise how many
/// own values were cleared.
pub fn follow_all(paths: &Paths, scope: Scope) -> Result<Option<usize>, ProfileError> {
    let _lock = lock(paths)?;
    let path = scope_path(paths, scope);
    let all: Vec<FieldKey> = FieldKey::all().collect();
    Ok(follow_unlocked(&path, scope, &all)?.map(|removed| removed.len()))
}

/// "Apply to games with their own value": makes `keys` follow All games in
/// each listed game that sets one of them itself. One locked write per
/// game, so it is not atomic across games: a game that cannot be written is
/// named in the outcome and the others still change. Games are given as
/// app id and name; the outcome lists names.
pub fn follow_all_games_in(
    paths: &Paths,
    games: &[(u64, String)],
    keys: &[FieldKey],
) -> FollowOutcome {
    let mut outcome = FollowOutcome::default();
    for (app_id, name) in games {
        let result = lock(paths).and_then(|_lock| {
            let path = paths.game_profile(*app_id);
            follow_unlocked(&path, Scope::Game(*app_id), keys)
        });
        match result {
            Ok(Some(removed)) if !removed.is_empty() => outcome.followed.push(name.clone()),
            Ok(_) => {}
            Err(e) => {
                outcome.failed.push(name.clone());
                outcome.errors.insert(name.clone(), e.to_string());
            }
        }
    }
    outcome.followed.sort();
    outcome.failed.sort();
    outcome
}

#[cfg(test)]
mod tests {
    use super::*;

    fn paths() -> (tempfile::TempDir, Paths) {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        (base, paths)
    }

    fn lines(parts: &[&str]) -> String {
        let mut text = parts.join("\n");
        text.push('\n');
        text
    }

    fn read(path: &Path) -> String {
        std::fs::read_to_string(path).unwrap()
    }

    #[test]
    fn an_empty_file_still_counts() {
        let (_base, paths) = paths();
        assert!(!has_profile(&paths, 9100010));
        std::fs::create_dir_all(paths.profiles_dir()).unwrap();
        std::fs::write(paths.game_profile(9100010), "").unwrap();
        assert!(has_profile(&paths, 9100010));
        let loaded = load_game(&paths, 9100010).unwrap().unwrap();
        assert!(loaded.profile.values.is_empty());
    }

    #[test]
    fn render_writes_version_name_and_sections_in_order() {
        let mut own = OwnValues::new();
        own.set(FieldKey::CpuSmt, Value::OptionalBool(Some(false)))
            .unwrap();
        own.set(FieldKey::GpuThreadedOptimization, Value::Bool(false))
            .unwrap();
        own.set(FieldKey::DlssSrMode, Value::Text("quality".into()))
            .unwrap();
        own.set(FieldKey::ProtonEnableHdr, Value::Bool(true))
            .unwrap();
        own.set(FieldKey::GpuPowerLimit, Value::Int(250)).unwrap();
        let text = render(
            Some("Chrome Avenue 2081"),
            &own,
            &["Migrated from 9100010.yaml.".to_owned()],
        );
        assert_eq!(
            text,
            lines(&[
                "# Migrated from 9100010.yaml.",
                "version = 1",
                "name = \"Chrome Avenue 2081\"",
                "",
                "[proton]",
                "enable_hdr = true",
                "",
                "[dlss]",
                "sr_mode = \"quality\"",
                "",
                "[gpu]",
                "power_limit = 250",
                "threaded_optimization = false",
                "",
                "[cpu]",
                "smt = false",
            ])
        );
        let loaded = parse_game(9100010, &text).unwrap();
        assert_eq!(loaded.profile.values, own);
        assert_eq!(loaded.profile.name, "Chrome Avenue 2081");
    }

    #[test]
    fn reading_checks_every_value_with_its_place() {
        let text = lines(&[
            "version = 1",
            "name = \"X\"",
            "[dlss]",
            "sr_mode = \"fast\"",
            "multi_frame = \"two\"",
            "[gpu]",
            "vrr = \"Always\"",
            "power_limt = 3",
            "[cpu]",
            "smt = true",
            "[overlay]",
            "enabled = true",
        ]);
        let errors: Vec<String> = parse_game(1, &text)
            .unwrap_err()
            .iter()
            .map(ToString::to_string)
            .collect();
        assert_eq!(
            errors,
            [
                "line 4, column 11: dlss.sr_mode: unsupported value \"fast\" (valid: , off, ultra_performance, performance, balanced, quality, dlaa)",
                "line 5, column 15: dlss.multi_frame: expected integer",
                "line 7, column 7: gpu.vrr: unsupported value \"Always\" (valid: , unset, automatic, always, never)",
            ]
        );
        let ok = lines(&[
            "[gpu]",
            "power_limt = 3",
            "[overlay]",
            "enabled = true",
            "[cpu]",
            "governor = \"conservative\"",
        ]);
        let loaded = parse_game(1, &ok).unwrap();
        let w: Vec<String> = loaded.warnings.iter().map(ToString::to_string).collect();
        assert_eq!(
            w,
            [
                "line 2, column 1: unknown key `gpu.power_limt` is ignored (did you mean `gpu.power_limit`?)",
                "line 3, column 2: unknown key `overlay` is ignored",
            ]
        );
        assert_eq!(
            loaded.profile.values.get(FieldKey::CpuGovernor),
            Some(&Value::Text("conservative".into())),
            "governors come from the system"
        );
        assert!(
            parse_all_games("name = \"X\"\n").unwrap().warnings[0]
                .message
                .contains("unknown key `name`")
        );
        assert!(parse_game(1, "version = 0\n").is_err());
        assert!(parse_game(1, "proton = 3\n").is_err());
        assert!(parse_game(1, "[proton\n").is_err());
        let newer = parse_game(1, "version = 2\n").unwrap();
        assert!(newer.read_only);
    }

    #[test]
    fn set_creates_fills_the_name_and_writes_only_on_change() {
        let (_base, paths) = paths();
        let scope = Scope::Game(9100010);
        let w = set(
            &paths,
            scope,
            "Chrome Avenue 2081",
            &[(FieldKey::GpuPowerLimit, Value::Int(250))],
        )
        .unwrap();
        assert!(w.created && w.written);
        assert_eq!(w.previous, [(FieldKey::GpuPowerLimit, None)]);
        let path = paths.game_profile(9100010);
        assert_eq!(
            read(&path),
            lines(&[
                "version = 1",
                "name = \"Chrome Avenue 2081\"",
                "",
                "[gpu]",
                "power_limit = 250",
            ])
        );
        let w = set(
            &paths,
            scope,
            "Other",
            &[(FieldKey::GpuPowerLimit, Value::Int(250))],
        )
        .unwrap();
        assert!(!w.created && !w.written, "same value, no write");
        assert!(w.previous.is_empty());
        let err = set(
            &paths,
            scope,
            "",
            &[
                (FieldKey::GpuPowerLimit, Value::Int(300)),
                (FieldKey::GpuVrr, Value::Text("sometimes".into())),
            ],
        )
        .unwrap_err();
        assert!(err.to_string().contains("gpu.vrr"), "{err}");
        assert!(read(&path).contains("power_limit = 250"), "nothing written");
    }

    #[test]
    fn edits_keep_comments_order_and_unknown_keys() {
        let (_base, paths) = paths();
        std::fs::create_dir_all(paths.profiles_dir()).unwrap();
        let path = paths.game_profile(9100010);
        std::fs::write(
            &path,
            lines(&[
                "# My settings.",
                "version = 1",
                "name = \"Chrome Avenue 2081\"",
                "",
                "[gpu]",
                "# Quiet please.",
                "fan_speed = 40 # percent",
                "my_note = \"keep\"",
                "",
                "[cpu]",
                "smt = false",
            ]),
        )
        .unwrap();
        let scope = Scope::Game(9100010);
        set(
            &paths,
            scope,
            "",
            &[
                (FieldKey::GpuFanSpeed, Value::Int(55)),
                (FieldKey::GpuClockOffset, Value::Int(100)),
                (FieldKey::ProtonEnableHdr, Value::Bool(true)),
                (FieldKey::DlssSrMode, Value::Text("dlaa".into())),
                (FieldKey::GpuThreadedOptimization, Value::Bool(false)),
            ],
        )
        .unwrap();
        assert_eq!(
            read(&path),
            lines(&[
                "# My settings.",
                "version = 1",
                "name = \"Chrome Avenue 2081\"",
                "",
                "[proton]",
                "enable_hdr = true",
                "",
                "[dlss]",
                "sr_mode = \"dlaa\"",
                "",
                "[gpu]",
                "clock_offset = 100",
                "# Quiet please.",
                "fan_speed = 55 # percent",
                "threaded_optimization = false",
                "my_note = \"keep\"",
                "",
                "[cpu]",
                "smt = false",
            ])
        );
        let removed = follow(
            &paths,
            scope,
            &[FieldKey::CpuSmt, FieldKey::ProtonEnableHdr],
        )
        .unwrap()
        .unwrap();
        assert_eq!(removed.len(), 2);
        let text = read(&path);
        assert!(
            !text.contains("[cpu]") && !text.contains("[proton]"),
            "{text}"
        );
        assert!(text.contains("# Quiet please.\nfan_speed = 55 # percent\n"));
        assert_eq!(follow_all(&paths, scope).unwrap(), Some(4));
        let text = read(&path);
        assert!(text.contains("name = \"Chrome Avenue 2081\""), "{text}");
        assert!(text.contains("my_note = \"keep\""), "{text}");
        let loaded = load_game(&paths, 9100010).unwrap().unwrap();
        assert!(loaded.profile.values.is_empty());
        assert_eq!(follow_all(&paths, scope).unwrap(), Some(0));
        assert_eq!(follow_all(&paths, Scope::Game(1)).unwrap(), None);
        assert!(!paths.game_profile(1).exists(), "follow never creates");
    }

    #[test]
    fn a_missing_version_is_added_on_write_and_a_newer_one_refuses() {
        let (_base, paths) = paths();
        std::fs::create_dir_all(paths.profiles_dir()).unwrap();
        let path = paths.all_games_profile();
        std::fs::write(
            &path,
            lines(&["# Hand written.", "[cpu]", "governor = \"powersave\""]),
        )
        .unwrap();
        set(
            &paths,
            Scope::AllGames,
            "",
            &[(FieldKey::CpuGovernor, Value::Text("performance".into()))],
        )
        .unwrap();
        assert_eq!(
            read(&path),
            lines(&[
                "# Hand written.",
                "version = 1",
                "[cpu]",
                "governor = \"performance\"",
            ])
        );
        std::fs::write(&path, "version = 2\n").unwrap();
        let err = set(
            &paths,
            Scope::AllGames,
            "",
            &[(FieldKey::CpuGovernor, Value::Text("performance".into()))],
        )
        .unwrap_err();
        assert!(matches!(err, ProfileError::ReadOnly { .. }), "{err}");
        assert_eq!(read(&path), "version = 2\n");
    }

    #[test]
    fn a_broken_file_is_never_written_over() {
        let (_base, paths) = paths();
        std::fs::create_dir_all(paths.profiles_dir()).unwrap();
        let path = paths.game_profile(5);
        std::fs::write(&path, "[gpu\n").unwrap();
        let err = set(
            &paths,
            Scope::Game(5),
            "",
            &[(FieldKey::GpuFanSpeed, Value::Int(1))],
        )
        .unwrap_err();
        assert!(err.to_string().starts_with("profile "), "{err}");
        assert_eq!(read(&path), "[gpu\n");
        assert!(matches!(
            load_game(&paths, 5),
            Err(ProfileError::Invalid { .. })
        ));
    }

    #[test]
    fn create_delete_and_list() {
        let (_base, paths) = paths();
        assert!(list(&paths).unwrap().profiles.is_empty());
        assert!(create(&paths, 9100020, "Hollow Crown 3").unwrap());
        assert!(!create(&paths, 9100020, "Other").unwrap());
        assert!(create(&paths, 9100010, "Chrome Avenue 2081").unwrap());
        let dir = paths.profiles_dir();
        std::fs::write(dir.join("09100030.toml"), "").unwrap();
        std::fs::write(dir.join("notes.toml"), "").unwrap();
        std::fs::write(dir.join("9100040.toml"), "[gpu\n").unwrap();
        std::fs::create_dir(dir.join("9100050.toml")).unwrap();
        set(
            &paths,
            Scope::AllGames,
            "",
            &[(FieldKey::GpuFanSpeed, Value::Int(1))],
        )
        .unwrap();
        let listing = list(&paths).unwrap();
        let ids: Vec<u64> = listing.profiles.iter().map(|p| p.profile.app_id).collect();
        assert_eq!(ids, [9100010, 9100020]);
        assert_eq!(listing.profiles[1].profile.name, "Hollow Crown 3");
        assert_eq!(listing.broken.len(), 1);
        assert_eq!(listing.broken[0].0, 9100040);
        assert!(delete(&paths, 9100020).unwrap());
        assert!(!delete(&paths, 9100020).unwrap(), "nothing to delete");
        assert_eq!(game_file_app_id("9100010.toml"), Some(9100010));
        assert_eq!(game_file_app_id("09100010.toml"), None);
        assert_eq!(game_file_app_id("all-games.toml"), None);
    }

    #[test]
    fn follow_all_games_in_touches_only_games_with_an_own_value() {
        let (_base, paths) = paths();
        let key = FieldKey::DlssSrMode;
        set(
            &paths,
            Scope::Game(1),
            "Zeta",
            &[(key, Value::Text("dlaa".into()))],
        )
        .unwrap();
        set(
            &paths,
            Scope::Game(2),
            "Beta",
            &[(FieldKey::GpuFanSpeed, Value::Int(1))],
        )
        .unwrap();
        set(
            &paths,
            Scope::Game(3),
            "Alpha",
            &[(key, Value::Text(String::new()))],
        )
        .unwrap();
        std::fs::write(paths.game_profile(4), "[dlss\n").unwrap();
        let games: Vec<(u64, String)> = [
            (1, "Zeta"),
            (2, "Beta"),
            (3, "Alpha"),
            (4, "Broken"),
            (5, "None"),
        ]
        .iter()
        .map(|(id, n)| (*id, (*n).to_owned()))
        .collect();
        let before = read(&paths.game_profile(2));
        let outcome = follow_all_games_in(&paths, &games, &[key]);
        assert_eq!(outcome.followed, ["Alpha", "Zeta"]);
        assert_eq!(outcome.failed, ["Broken"]);
        assert!(outcome.errors["Broken"].contains("has errors"));
        assert_eq!(read(&paths.game_profile(2)), before, "untouched");
        assert!(!paths.game_profile(5).exists());
        let zeta = load_game(&paths, 1).unwrap().unwrap();
        assert!(!zeta.profile.values.contains(key));
    }

    #[test]
    fn edit_sets_and_follows_in_one_write_and_never_creates_to_follow() {
        let (_base, paths) = paths();
        let scope = Scope::Game(9100010);
        let w = edit(
            &paths,
            scope,
            "Chrome Avenue 2081",
            &[],
            &[FieldKey::CpuSmt],
        )
        .unwrap();
        assert_eq!(
            w,
            Written {
                created: false,
                written: false,
                previous: Vec::new()
            }
        );
        assert!(
            !paths.game_profile(9100010).exists(),
            "follow never creates"
        );
        let w = edit(
            &paths,
            scope,
            "Chrome Avenue 2081",
            &[
                (FieldKey::GpuFanSpeed, Value::Int(0)),
                (FieldKey::CpuSmt, Value::OptionalBool(Some(true))),
            ],
            &[],
        )
        .unwrap();
        assert!(w.created && w.written);
        let w = edit(
            &paths,
            scope,
            "Other",
            &[(FieldKey::GpuPowerLimit, Value::Int(200))],
            &[FieldKey::GpuFanSpeed, FieldKey::CpuGovernor],
        )
        .unwrap();
        assert!(!w.created && w.written);
        assert_eq!(
            w.previous,
            [
                (FieldKey::GpuPowerLimit, None),
                (FieldKey::GpuFanSpeed, Some(Value::Int(0))),
            ]
        );
        let loaded = load_game(&paths, 9100010).unwrap().unwrap().profile;
        assert_eq!(loaded.name, "Chrome Avenue 2081");
        assert!(!loaded.values.contains(FieldKey::GpuFanSpeed));
        assert_eq!(
            loaded.values.get(FieldKey::GpuPowerLimit),
            Some(&Value::Int(200))
        );
        let before = read(&paths.game_profile(9100010));
        let err = edit(
            &paths,
            scope,
            "",
            &[(FieldKey::GpuVrr, Value::Text("sometimes".into()))],
            &[FieldKey::GpuPowerLimit],
        )
        .unwrap_err();
        assert!(matches!(err, ProfileError::Field(_)), "{err}");
        assert_eq!(
            read(&paths.game_profile(9100010)),
            before,
            "nothing written"
        );
        let w = edit(&paths, scope, "", &[], &[FieldKey::GpuFanSpeed]).unwrap();
        assert!(!w.written && w.previous.is_empty(), "nothing to follow");
    }

    #[test]
    fn two_threads_writing_different_fields_both_land() {
        let (_base, paths) = paths();
        let a = {
            let paths = paths.clone();
            std::thread::spawn(move || {
                for n in 0..20 {
                    set(
                        &paths,
                        Scope::Game(7),
                        "G",
                        &[(FieldKey::GpuFanSpeed, Value::Int(n))],
                    )
                    .unwrap();
                }
            })
        };
        let b = {
            let paths = paths.clone();
            std::thread::spawn(move || {
                for n in 0..20 {
                    set(
                        &paths,
                        Scope::Game(7),
                        "G",
                        &[(FieldKey::GpuPowerLimit, Value::Int(n))],
                    )
                    .unwrap();
                }
            })
        };
        a.join().unwrap();
        b.join().unwrap();
        let loaded = load_game(&paths, 7).unwrap().unwrap().profile.values;
        assert_eq!(loaded.get(FieldKey::GpuFanSpeed), Some(&Value::Int(19)));
        assert_eq!(loaded.get(FieldKey::GpuPowerLimit), Some(&Value::Int(19)));
    }
}
