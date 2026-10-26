//! The migration from Go spela: reading its YAML files and writing the TOML ones, with the record in `migration.toml`.
//!
//! [`run`] is called once at startup, before anything reads the store. It
//! lists Go's files and migrates each one the record does not list as
//! migrated or skipped:
//!
//! | Go file | Becomes |
//! |---|---|
//! | `<config>/config.yaml` | `<config>/config.toml`, only values that differ from Go's defaults |
//! | `<config>/profiles/default.yaml` | `<config>/profiles/all-games.toml` |
//! | `<config>/profiles/<appid>.yaml` | `<config>/profiles/<appid>.toml` |
//! | `<config>/lists/overrides.yaml` | `<config>/lists/overrides.toml` |
//! | `<config>/lists/denylist.yaml` | `<config>/lists/denylist.toml` |
//! | `<data>/games.yaml` | `<cache>/games.json` |
//! | `<cache>/manifest.json`, `<cache>/update-check.json` | `<cache>/manifest-v1.json`, `<cache>/release-check.json`, copied once as seeds (decision 53) |
//!
//! The rules:
//!
//! - Go files are only read, never renamed, rewritten or deleted. The
//!   record keeps each one's SHA-256.
//! - Each new file starts with a comment that names its Go source and the
//!   values it did not carry over. The record, `<data>/migration.toml`,
//!   lists every one of them, defaults included.
//! - A source is migrated at most once: "migrated" means listed in the
//!   record, not "the target exists". A source missing from the record
//!   whose target exists and loads is adopted and the target is left alone.
//! - Each source succeeds or fails on its own. A failed source writes no
//!   target, is recorded with its error and is tried again on every start.
//!   A broken `default.yaml` also fails every game profile that has no
//!   `overrides` key, because Go's legacy load needed it.
//! - The config, profile and list steps run under `config.lock`. The game
//!   cache step then runs under `games.lock` alone, and records its result
//!   under `config.lock` again, so no step holds both locks.
//!
//! Only this module reads YAML, through the one wrapper in `yaml.rs`.

mod config;
mod games;
mod go;
mod lists;
mod profiles;
pub mod record;
mod yaml;

use std::io;
use std::path::{Path, PathBuf};

use jiff::Timestamp;
use jiff::tz::TimeZone;

use crate::clock::Clock;
use crate::fs::{Durability, write_atomic};
use crate::hex::sha256_hex;
use crate::lock::{LockError, StoreLock};
use crate::paths::Paths;
use crate::profile::store;

pub use record::{Dropped, Outcome, Record, RecordError, Source, Why};

use go::{GoLoadError, GoProfile};
use profiles::GoScope;

/// The advice that goes with a Go `config.yaml` that does not migrate. It
/// is Go's text.
pub const CONFIG_HINT: &str = "Fix the YAML or remove the file to restore defaults.";

/// Which Go file a source is.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SourceKind {
    /// `config.yaml`.
    Config,
    /// `profiles/default.yaml`.
    AllGames,
    /// `profiles/<appid>.yaml`.
    Game(u64),
    /// A file in the profile folder that Go never loaded, such as a `.yml`.
    NotAProfile,
    /// `lists/overrides.yaml`.
    DenyOverrides,
    /// `lists/denylist.yaml`.
    UserDenylist,
    /// `<data>/games.yaml`.
    GameDatabase,
    /// `<cache>/manifest.json`.
    ManifestCache,
    /// `<cache>/update-check.json`.
    UpdateCheck,
}

impl SourceKind {
    /// Whether the source holds user intent, as opposed to a rebuildable
    /// cache.
    pub fn is_user_intent(self) -> bool {
        !matches!(
            self,
            SourceKind::GameDatabase | SourceKind::ManifestCache | SourceKind::UpdateCheck
        )
    }
}

/// One source this run looked at.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Handled {
    /// Which Go file it is.
    pub kind: SourceKind,
    /// Its entry in the record.
    pub entry: Source,
    /// Its target already existed and was adopted, not written.
    pub adopted: bool,
}

/// What one run of the migration did. Sources already settled in the record
/// are not listed.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Report {
    /// The sources this run read or tried, in the order it did them.
    pub handled: Vec<Handled>,
    /// The migration record.
    pub record: PathBuf,
}

impl Report {
    /// Whether this run had nothing to do.
    pub fn is_empty(&self) -> bool {
        self.handled.is_empty()
    }

    /// The sources this run migrated and wrote, adopted ones not included.
    pub fn migrated(&self) -> impl Iterator<Item = &Handled> {
        self.handled
            .iter()
            .filter(|h| h.entry.result == Outcome::Migrated && !h.adopted)
    }

    /// The sources that failed. Each is tried again at the next start.
    pub fn failed(&self) -> impl Iterator<Item = &Handled> {
        self.handled
            .iter()
            .filter(|h| h.entry.result == Outcome::Failed)
    }

    /// The error of a Go `config.yaml` that did not migrate, naming the
    /// file. The CLI and the TUI exit 3 on it, with [`CONFIG_HINT`].
    pub fn config_failure(&self) -> Option<String> {
        self.failed()
            .find(|h| h.kind == SourceKind::Config)
            .map(|h| {
                format!(
                    "cannot migrate Go spela's config file {}: {}",
                    h.entry.path.display(),
                    h.entry.error
                )
            })
    }

    /// Why a game's Go profile did not migrate, when it did not. Such a
    /// profile counts as broken: the CLI reports it and exits 3, and wrapper
    /// mode warns and falls back.
    pub fn failed_profile(&self, app_id: u64) -> Option<&str> {
        self.failed()
            .find(|h| h.kind == SourceKind::Game(app_id))
            .map(|h| h.entry.error.as_str())
    }

    /// Why the Go All games profile did not migrate, when it did not.
    pub fn failed_all_games(&self) -> Option<&str> {
        self.failed()
            .find(|h| h.kind == SourceKind::AllGames)
            .map(|h| h.entry.error.as_str())
    }

    /// The values this run did not carry over that differ from Go's
    /// defaults: what the notice counts.
    pub fn notable_dropped(&self) -> impl Iterator<Item = &Dropped> {
        self.migrated()
            .flat_map(|h| h.entry.dropped.iter())
            .filter(|d| !d.default)
    }

    /// The lines to show on the run that migrates: one line to stderr in
    /// the CLI, one status message in the TUI. Empty when nothing was
    /// migrated. A second line says when Go clock or memory offsets were
    /// not carried over (decision 31).
    pub fn notice(&self) -> Vec<String> {
        let files = self.migrated().count();
        if files == 0 {
            return Vec::new();
        }
        let plural = |n: usize, one: &str, many: &str| {
            if n == 1 {
                format!("{n} {one}")
            } else {
                format!("{n} {many}")
            }
        };
        let dropped = self.notable_dropped().count();
        let mut line = format!(
            "Migrated {} from Go spela, kept as backups only.",
            plural(files, "file", "files")
        );
        if dropped > 0 {
            line.push_str(&format!(
                " Dropped {}.",
                plural(dropped, "setting", "settings")
            ));
        }
        line.push_str(&format!(" See {}", self.record.display()));
        let mut lines = vec![line];
        if self
            .notable_dropped()
            .any(|d| d.why == Why::OffsetMeaningChanged)
        {
            lines.push(format!(
                "GPU clock and memory offsets from Go spela were not carried over. See {}",
                self.record.display()
            ));
        }
        lines
    }
}

/// The migration could not run at all.
#[derive(Debug, thiserror::Error)]
pub enum MigrateError {
    /// The record does not load or cannot be written.
    #[error(transparent)]
    Record(#[from] RecordError),
    /// A store lock could not be taken.
    #[error(transparent)]
    Lock(#[from] LockError),
}

/// A Go file and where it goes.
#[derive(Debug, Clone)]
struct Candidate {
    kind: SourceKind,
    path: PathBuf,
    target: Option<PathBuf>,
    skip_note: &'static str,
}

/// Runs the migration. Cheap when there is nothing to do: a few `stat`
/// calls, one folder read and the record, with no lock taken.
pub fn run(paths: &Paths, clock: &dyn Clock) -> Result<Report, MigrateError> {
    let mut report = Report {
        handled: Vec::new(),
        record: paths.migration_record(),
    };
    let record_path = paths.migration_record();
    let pending = |record: &Record| -> Vec<Candidate> {
        candidates(paths)
            .into_iter()
            .filter(|c| !record.is_done(&c.path))
            .collect()
    };
    if pending(&Record::load(&record_path)?).is_empty() {
        return Ok(report);
    }
    let ctx = Context::new(paths, clock);

    let games_source = {
        let _lock = StoreLock::acquire(&paths.config_lock())?;
        let mut record = Record::load(&record_path)?;
        let todo = pending(&record);
        let mut games_source = None;
        let all_games = load_go_all_games(paths);
        for candidate in todo {
            let handled = match candidate.kind {
                SourceKind::GameDatabase => {
                    games_source = Some(candidate);
                    continue;
                }
                _ => ctx.migrate(&candidate, &all_games),
            };
            record.put(handled.entry.clone());
            report.handled.push(handled);
        }
        if !report.handled.is_empty() {
            record.save(&record_path)?;
        }
        games_source
    };

    if let Some(candidate) = games_source {
        let handled = ctx.migrate_games(&candidate);
        let _lock = StoreLock::acquire(&paths.config_lock())?;
        let mut record = Record::load(&record_path)?;
        if !record.is_done(&candidate.path) {
            if record.put(handled.entry.clone()) {
                record.save(&record_path)?;
            }
            report.handled.push(handled);
        }
    }
    for h in &report.handled {
        match h.entry.result {
            Outcome::Failed => tracing::warn!(
                path = %h.entry.path.display(),
                "cannot migrate a Go spela file: {}",
                h.entry.error
            ),
            outcome => tracing::info!(
                path = %h.entry.path.display(),
                result = outcome.as_str(),
                dropped = h.entry.dropped.len(),
                "migrated a Go spela file"
            ),
        }
    }
    Ok(report)
}

/// Every Go file that exists, in the order the migration needs: config,
/// All games, game profiles, lists, then the caches and the game cache.
fn candidates(paths: &Paths) -> Vec<Candidate> {
    let go = paths.go();
    let one = |kind, path: PathBuf, target: PathBuf| Candidate {
        kind,
        path,
        target: Some(target),
        skip_note: "",
    };
    let mut out = vec![
        one(SourceKind::Config, go.config(), paths.config_file()),
        one(
            SourceKind::AllGames,
            go.all_games_profile(),
            paths.all_games_profile(),
        ),
    ];
    let mut profiles: Vec<(String, PathBuf)> = std::fs::read_dir(paths.profiles_dir())
        .into_iter()
        .flatten()
        .flatten()
        .filter(|e| e.path().is_file())
        .filter_map(|e| Some((e.file_name().into_string().ok()?, e.path())))
        .collect();
    profiles.sort();
    for (name, path) in profiles {
        let (stem, yml) = match (name.strip_suffix(".yaml"), name.strip_suffix(".yml")) {
            (Some(stem), _) => (stem, false),
            (_, Some(stem)) => (stem, true),
            _ => continue,
        };
        if stem == "default" && !yml {
            continue;
        }
        let app_id = stem.parse::<u64>().ok();
        let canonical = app_id.is_some_and(|id| id.to_string() == stem);
        match app_id {
            Some(id) if canonical && !yml => {
                out.push(one(SourceKind::Game(id), path, paths.game_profile(id)));
            }
            _ => out.push(Candidate {
                kind: SourceKind::NotAProfile,
                path,
                target: None,
                skip_note: if yml {
                    "Go listed .yml profiles but only ever loaded <appid>.yaml"
                } else if app_id.is_some() {
                    "Go only ever loaded <appid>.yaml, and this name is not one"
                } else {
                    "not a game profile: the name is not an app id"
                },
            }),
        }
    }
    out.push(one(
        SourceKind::DenyOverrides,
        go.deny_overrides(),
        paths.deny_overrides(),
    ));
    out.push(one(
        SourceKind::UserDenylist,
        go.user_denylist(),
        paths.user_denylist(),
    ));
    out.push(one(
        SourceKind::ManifestCache,
        go.manifest_cache(),
        paths.manifest_cache(),
    ));
    out.push(one(
        SourceKind::UpdateCheck,
        go.update_check(),
        paths.release_check(),
    ));
    out.push(one(
        SourceKind::GameDatabase,
        go.game_database(),
        paths.game_cache(),
    ));
    out.retain(|c| c.path.is_file());
    out
}

/// Go's `default.yaml` as Go loaded it: `None` when there is none. A legacy
/// game profile is measured against it even after it was migrated.
fn load_go_all_games(paths: &Paths) -> Result<Option<GoProfile>, GoLoadError> {
    match std::fs::read(paths.go().all_games_profile()) {
        Ok(bytes) => {
            let text = String::from_utf8(bytes)
                .map_err(|_| GoLoadError::Yaml(yaml::error("the file is not UTF-8")))?;
            go::parse_profile(&text).map(Some)
        }
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(e) => Err(GoLoadError::Yaml(yaml::error(e.to_string()))),
    }
}

/// What every step of one run shares.
struct Context<'a> {
    paths: &'a Paths,
    at: toml::value::Datetime,
    date: String,
}

/// A converted source, ready to write.
struct Ready {
    text: String,
    dropped: Vec<Dropped>,
}

impl<'a> Context<'a> {
    fn new(paths: &'a Paths, clock: &dyn Clock) -> Context<'a> {
        let now = clock.now();
        let second = Timestamp::from_second(now.as_second()).unwrap_or(now);
        Context {
            paths,
            at: second
                .to_string()
                .parse()
                .expect("an RFC 3339 time is a TOML datetime"),
            date: now.to_zoned(TimeZone::system()).date().to_string(),
        }
    }

    fn entry(&self, candidate: &Candidate, sha256: String) -> Source {
        Source {
            path: candidate.path.clone(),
            sha256,
            target: candidate.target.clone(),
            result: Outcome::Migrated,
            at: self.at,
            by: format!("spela {}", crate::VERSION),
            note: String::new(),
            error: String::new(),
            dropped: Vec::new(),
        }
    }

    fn failed(&self, candidate: &Candidate, sha256: String, error: String) -> Handled {
        let mut entry = self.entry(candidate, sha256);
        entry.result = Outcome::Failed;
        entry.error = error;
        Handled {
            kind: candidate.kind,
            entry,
            adopted: false,
        }
    }

    /// Migrates one source under `config.lock`.
    fn migrate(
        &self,
        candidate: &Candidate,
        all_games: &Result<Option<GoProfile>, GoLoadError>,
    ) -> Handled {
        let (text, sha256) = match read_source(&candidate.path) {
            Ok(read) => read,
            Err((sha256, error)) => return self.failed(candidate, sha256, error),
        };
        let mut entry = self.entry(candidate, sha256.clone());
        if candidate.kind == SourceKind::NotAProfile {
            entry.result = Outcome::Skipped;
            entry.note = candidate.skip_note.to_owned();
            return Handled {
                kind: candidate.kind,
                entry,
                adopted: false,
            };
        }
        let target = candidate
            .target
            .clone()
            .expect("a migrated source has a target");
        if target.exists() {
            return match self.loads(candidate.kind, &target) {
                Ok(()) => {
                    entry.note = "its target already existed and was kept".to_owned();
                    Handled {
                        kind: candidate.kind,
                        entry,
                        adopted: true,
                    }
                }
                Err(e) => self.failed(
                    candidate,
                    sha256,
                    format!(
                        "{} already exists but does not load, so it is left alone: {e}",
                        target.display()
                    ),
                ),
            };
        }
        let ready = match self.convert(candidate, &text, all_games) {
            Ok(ready) => ready,
            Err(error) => return self.failed(candidate, sha256, error),
        };
        if let Err(error) = self.write(candidate, &target, &ready) {
            return self.failed(candidate, sha256, error);
        }
        entry.dropped = ready.dropped;
        Handled {
            kind: candidate.kind,
            entry,
            adopted: false,
        }
    }

    fn convert(
        &self,
        candidate: &Candidate,
        text: &str,
        all_games: &Result<Option<GoProfile>, GoLoadError>,
    ) -> Result<Ready, String> {
        let name = file_name(&candidate.path);
        let record = self.paths.migration_record();
        let header = |dropped: &[Dropped], extra: &[&str]| {
            header_lines(&name, &self.date, dropped, &record, extra)
        };
        match candidate.kind {
            SourceKind::Config => {
                let c = config::convert(text, self.paths).map_err(|e| e.to_string())?;
                let mut out = comment(&header(&c.dropped, &[]));
                out.push_str(&c.text);
                Ok(Ready {
                    text: out,
                    dropped: c.dropped,
                })
            }
            SourceKind::AllGames | SourceKind::Game(_) => {
                let scope = match candidate.kind {
                    SourceKind::AllGames => GoScope::AllGames,
                    _ => GoScope::Game,
                };
                let mut go = go::parse_profile(text).map_err(|e| e.to_string())?;
                if scope == GoScope::Game && go.overrides.is_none() {
                    let default = all_games
                        .as_ref()
                        .map_err(|e| GoLoadError::Default(Box::new(e.clone())).to_string())?;
                    go::migrate_inheritance(&mut go, default.as_ref());
                }
                let unknown = go::unknown_profile_keys(text);
                let c = profiles::convert(&go, scope, &unknown);
                Ok(Ready {
                    text: store::render(c.name.as_deref(), &c.values, &header(&c.dropped, &[])),
                    dropped: c.dropped,
                })
            }
            SourceKind::DenyOverrides => {
                let c = lists::overrides(text).map_err(|e| e.to_string())?;
                let mut out = comment(&header(&c.dropped, &[]));
                out.push_str(&c.text);
                Ok(Ready {
                    text: out,
                    dropped: c.dropped,
                })
            }
            SourceKind::UserDenylist => {
                let c = lists::denylist(text).map_err(|e| e.to_string())?;
                let note = [
                    "In Go spela this list replaced the built-in deny list. Now it adds",
                    "to it. `spela denylist allow <game>` lifts a built-in entry.",
                ];
                let mut out = comment(&header(&c.dropped, &note));
                out.push_str(&c.text);
                Ok(Ready {
                    text: out,
                    dropped: c.dropped,
                })
            }
            SourceKind::ManifestCache | SourceKind::UpdateCheck => {
                serde_json::from_str::<serde_json::Value>(text)
                    .map_err(|e| format!("not JSON: {e}"))?;
                Ok(Ready {
                    text: text.to_owned(),
                    dropped: Vec::new(),
                })
            }
            SourceKind::NotAProfile | SourceKind::GameDatabase => {
                unreachable!("handled elsewhere")
            }
        }
    }

    fn write(&self, candidate: &Candidate, target: &Path, ready: &Ready) -> Result<(), String> {
        match candidate.kind {
            SourceKind::AllGames => store::write_checked(target, &ready.text, false),
            SourceKind::Game(_) => store::write_checked(target, &ready.text, true),
            SourceKind::Config => {
                crate::config::parse(&ready.text).map_err(|e| e.to_string())?;
                return write_new(target, &ready.text, Durability::Durable);
            }
            SourceKind::DenyOverrides | SourceKind::UserDenylist => {
                toml::from_str::<toml::Table>(&ready.text).map_err(|e| e.to_string())?;
                return write_new(target, &ready.text, Durability::Durable);
            }
            SourceKind::ManifestCache | SourceKind::UpdateCheck => {
                write_new(target, &ready.text, Durability::Cache)?;
                // An old manifest cache without `fetched_at` is dated by
                // its file time, so the seed keeps the Go file's time.
                let modified = std::fs::metadata(&candidate.path)
                    .and_then(|m| m.modified())
                    .map_err(|e| e.to_string())?;
                return std::fs::File::options()
                    .write(true)
                    .open(target)
                    .and_then(|f| f.set_modified(modified))
                    .map_err(|e| e.to_string());
            }
            SourceKind::NotAProfile | SourceKind::GameDatabase => unreachable!("handled elsewhere"),
        }
        .map_err(|e| e.to_string())
    }

    /// Whether an existing target loads, so it can be adopted.
    fn loads(&self, kind: SourceKind, target: &Path) -> Result<(), String> {
        let text = std::fs::read_to_string(target).map_err(|e| e.to_string())?;
        let errors = |errors: Vec<crate::config::Diagnostic>| {
            errors
                .iter()
                .map(ToString::to_string)
                .collect::<Vec<_>>()
                .join("; ")
        };
        match kind {
            SourceKind::Config => crate::config::parse(&text)
                .map(|_| ())
                .map_err(|e| e.to_string()),
            SourceKind::AllGames => store::parse_all_games(&text).map(|_| ()).map_err(errors),
            SourceKind::Game(id) => store::parse_game(id, &text).map(|_| ()).map_err(errors),
            SourceKind::DenyOverrides | SourceKind::UserDenylist => {
                toml::from_str::<toml::Table>(&text)
                    .map(|_| ())
                    .map_err(|e| e.message().to_owned())
            }
            SourceKind::ManifestCache | SourceKind::UpdateCheck | SourceKind::GameDatabase => {
                serde_json::from_str::<serde_json::Value>(&text)
                    .map(|_| ())
                    .map_err(|e| e.to_string())
            }
            SourceKind::NotAProfile => Ok(()),
        }
    }

    /// Seeds the game cache under `games.lock`. A cache that already has
    /// games is adopted. A failure is not an error for spela: the next scan
    /// rebuilds the cache.
    fn migrate_games(&self, candidate: &Candidate) -> Handled {
        let (text, sha256) = match read_source(&candidate.path) {
            Ok(read) => read,
            Err((sha256, error)) => return self.failed(candidate, sha256, error),
        };
        let seed = match games::convert(&text) {
            Ok(seed) => seed,
            Err(e) => return self.failed(candidate, sha256, e.to_string()),
        };
        let adopted = crate::games::cache::update(self.paths, |cache| {
            if cache.is_empty() {
                *cache = seed;
                false
            } else {
                true
            }
        });
        let mut entry = self.entry(candidate, sha256.clone());
        match adopted {
            Ok(adopted) => {
                if adopted {
                    entry.note = "its target already existed and was kept".to_owned();
                }
                Handled {
                    kind: candidate.kind,
                    entry,
                    adopted,
                }
            }
            Err(e) => self.failed(candidate, sha256, e.to_string()),
        }
    }
}

/// Reads a Go file: its text and SHA-256, or the hash (empty when unread)
/// and the error.
fn read_source(path: &Path) -> Result<(String, String), (String, String)> {
    let bytes = std::fs::read(path).map_err(|e| (String::new(), e.to_string()))?;
    let sha256 = sha256_hex(&bytes);
    match String::from_utf8(bytes) {
        Ok(text) => Ok((text, sha256)),
        Err(_) => Err((sha256, "the file is not UTF-8".to_owned())),
    }
}

fn write_new(target: &Path, text: &str, durability: Durability) -> Result<(), String> {
    if let Some(dir) = target.parent() {
        std::fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    }
    write_atomic(target, text.as_bytes(), durability).map_err(|e| e.to_string())
}

fn file_name(path: &Path) -> String {
    path.file_name()
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_default()
}

/// The header of a migrated file: its Go source, the values it did not
/// carry over, and where the full record is. Go's defaults that a config
/// does not write are only in the record.
fn header_lines(
    source: &str,
    date: &str,
    dropped: &[Dropped],
    record: &Path,
    extra: &[&str],
) -> Vec<String> {
    let mut lines = vec![format!(
        "Migrated from {source} (Go spela) on {date}. The original is kept."
    )];
    let shown: Vec<&Dropped> = dropped.iter().filter(|d| d.why != Why::Default).collect();
    match shown.as_slice() {
        [] => {}
        [one] => lines.push(format!("Not carried over: {one}.")),
        many => {
            lines.push("Not carried over:".to_owned());
            lines.extend(many.iter().map(|d| format!("  {d}")));
        }
    }
    lines.extend(extra.iter().map(|l| (*l).to_owned()));
    lines.push(format!("Full record: {}", record.display()));
    lines
}

fn comment(lines: &[String]) -> String {
    let mut out = String::new();
    for line in lines {
        out.push_str("# ");
        out.push_str(line);
        out.push('\n');
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::clock::ManualClock;

    fn handled(kind: SourceKind, result: Outcome, dropped: Vec<Dropped>) -> Handled {
        Handled {
            kind,
            entry: Source {
                path: PathBuf::from("/c/x.yaml"),
                sha256: String::new(),
                target: None,
                result,
                at: "2026-10-26T17:04:11Z".parse().unwrap(),
                by: "spela 0.7.0".into(),
                note: String::new(),
                error: "bad".into(),
                dropped,
            },
            adopted: false,
        }
    }

    #[test]
    fn the_header_names_the_source_and_what_was_dropped() {
        let record = Path::new("/d/migration.toml");
        let one = [Dropped::new("overlay.enabled", "true", Why::Retired, false)];
        assert_eq!(
            header_lines("1091500.yaml", "2026-10-26", &one, record, &[]),
            [
                "Migrated from 1091500.yaml (Go spela) on 2026-10-26. The original is kept.",
                "Not carried over: overlay.enabled = true (removed in spela 0.7).",
                "Full record: /d/migration.toml",
            ]
        );
        let many = [
            Dropped::new("log_level", "\"info\"", Why::Default, true),
            Dropped::new("theme", "\"dark\"", Why::GuiOnly, false),
            Dropped::new("compact_mode", "true", Why::Retired, false),
        ];
        let lines = header_lines("config.yaml", "2026-10-26", &many, record, &["Extra."]);
        assert_eq!(lines[1], "Not carried over:");
        assert_eq!(lines[2], "  theme = \"dark\" (the GUI theme is gone)");
        assert_eq!(lines[4], "Extra.");
        assert_eq!(lines.len(), 6, "defaults stay in the record only");
        assert_eq!(comment(&lines[..1]), format!("# {}\n", lines[0]));
    }

    #[test]
    fn the_notice_counts_files_and_values_that_were_not_defaults() {
        let mut report = Report {
            handled: vec![
                handled(
                    SourceKind::Game(1),
                    Outcome::Migrated,
                    vec![
                        Dropped::new("gpu.clock_offset", "150", Why::OffsetMeaningChanged, false),
                        Dropped::new("overlay.show_fps", "false", Why::Retired, true),
                    ],
                ),
                handled(SourceKind::Config, Outcome::Failed, Vec::new()),
                handled(SourceKind::NotAProfile, Outcome::Skipped, Vec::new()),
            ],
            record: PathBuf::from("/d/migration.toml"),
        };
        assert_eq!(
            report.notice(),
            [
                "Migrated 1 file from Go spela, kept as backups only. Dropped 1 setting. See /d/migration.toml",
                "GPU clock and memory offsets from Go spela were not carried over. See /d/migration.toml",
            ]
        );
        assert_eq!(
            report.config_failure().unwrap(),
            "cannot migrate Go spela's config file /c/x.yaml: bad"
        );
        assert_eq!(report.failed_profile(1), None);
        report.handled.remove(0);
        assert!(report.notice().is_empty());
    }

    #[test]
    fn nothing_to_migrate_takes_no_lock() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        let clock = ManualClock::new("2026-10-26T17:04:11Z".parse().unwrap());
        let report = run(&paths, &clock).unwrap();
        assert!(report.is_empty());
        assert!(!paths.state_dir.exists(), "no lock file was made");
        assert!(!paths.migration_record().exists());
    }
}
