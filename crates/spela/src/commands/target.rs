//! What the write commands share: the profile a command changes, the game a
//! `<game>` argument names, and how store errors become exit codes.
//!
//! A command that edits a profile takes `<game>` or `--all-games`, which
//! edits All games (decision 20). A game resolves through the game cache as
//! every command does: a numeric AppID first, then an exact name. When the
//! cache is empty the CLI scans once first (decision 24).
//!
//! Exit codes for store errors: a file that does not load, or that a newer
//! spela wrote, exits 3; a value a field or setting cannot hold exits 2; a
//! file that cannot be read or written exits 1.

use std::io::Write;

use spela_core::config::store::ConfigStoreError;
use spela_core::denylist::ListError;
use spela_core::games::scan::rescan;
use spela_core::games::{Game, GameCache, cache};
use spela_core::profile::store::ProfileError;
use spela_core::profile::words::ALL_GAMES;
use spela_core::profile::{FieldError, Scope};

use super::read;
pub use super::read::SCAN_HINT;
use crate::cli::Context;
use crate::error::{CliError, exit};
use crate::migration;

/// The profile a write command changes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Target {
    /// All games or one game.
    pub scope: Scope,
    /// The game's name, which fills an empty profile name. Empty for All
    /// games.
    pub name: String,
}

impl Target {
    /// The name messages use: the game's, or `All games`.
    pub fn label(&self) -> &str {
        match self.scope {
            Scope::AllGames => ALL_GAMES,
            Scope::Game(_) => &self.name,
        }
    }

    /// Whether this is All games.
    pub fn is_all_games(&self) -> bool {
        self.scope == Scope::AllGames
    }
}

/// The profile `<game>` or `--all-games` names. A write to a scope whose Go
/// profile did not migrate is refused (exit 3), so the retry at the next
/// start still imports it.
pub fn target(ctx: &Context, game: Option<&str>, all_games: bool) -> Result<Target, CliError> {
    let target = match (game, all_games) {
        (_, true) => Target {
            scope: Scope::AllGames,
            name: String::new(),
        },
        (Some(query), false) => {
            let game = find_game(ctx, query)?;
            Target {
                scope: Scope::Game(game.app_id),
                name: game.name,
            }
        }
        (None, false) => {
            return Err(CliError::new(
                exit::USAGE,
                "name a game, or pass --all-games to change All games",
            ));
        }
    };
    migration::refuse_profile(&ctx.migration, target.scope)?;
    Ok(target)
}

/// The game cache, scanned first when it is empty (decision 24). A scan
/// that cannot run leaves the cache empty; the lookup then says the game
/// was not found. The Steam install is the read commands' one, so `--demo`
/// never scans the user's Steam.
pub fn games(ctx: &Context) -> Result<GameCache, CliError> {
    let found = cache::load(&ctx.paths);
    if !found.is_empty() {
        return Ok(found);
    }
    let config = read::config(ctx)?;
    let install = read::steam(ctx, &config);
    match rescan(&ctx.paths, &install, ctx.clock.as_ref()) {
        Ok(_) => Ok(cache::load(&ctx.paths)),
        Err(e) => {
            tracing::info!("no scan before the lookup: {e}");
            Ok(found)
        }
    }
}

/// The game a `<game>` argument names, with Go's `game not found: <q>`.
pub fn find_game(ctx: &Context, query: &str) -> Result<Game, CliError> {
    let games = games(ctx)?;
    games
        .find(query)
        .cloned()
        .map_err(|e| CliError::from(e).with_hint(SCAN_HINT))
}

/// A profile store error with its exit code.
pub fn profile_error(e: ProfileError) -> CliError {
    let code = match &e {
        ProfileError::Invalid { .. } | ProfileError::ReadOnly { .. } => exit::CONFIG,
        ProfileError::Field(_) => exit::USAGE,
        ProfileError::Io { .. } | ProfileError::Lock(_) => exit::FAILURE,
    };
    match e {
        ProfileError::Lock(lock) => CliError::from(lock),
        other => CliError::new(code, other.to_string()),
    }
}

/// A value a field cannot hold: a usage error.
pub fn field_error(e: FieldError) -> CliError {
    CliError::new(exit::USAGE, e.to_string())
}

/// A config store error with its exit code.
pub fn config_store_error(e: ConfigStoreError) -> CliError {
    match e {
        ConfigStoreError::Set(e) => CliError::new(exit::USAGE, e.to_string()),
        ConfigStoreError::Config(e) => CliError::from(e),
        e @ ConfigStoreError::ReadOnly { .. } => CliError::new(exit::CONFIG, e.to_string()),
        e @ ConfigStoreError::Io { .. } => CliError::new(exit::FAILURE, e.to_string()),
        ConfigStoreError::Lock(e) => CliError::from(e),
    }
}

/// A deny list error with its exit code.
pub fn list_error(e: ListError) -> CliError {
    match e {
        e @ (ListError::Invalid { .. } | ListError::Newer { .. }) => {
            CliError::new(exit::CONFIG, e.to_string())
        }
        e @ (ListError::Read { .. } | ListError::Write { .. }) => {
            CliError::new(exit::FAILURE, e.to_string())
        }
        ListError::Lock(e) => CliError::from(e),
    }
}

/// Writes one line of results to stdout.
pub fn line(out: &mut dyn Write, text: &str) -> Result<(), CliError> {
    writeln!(out, "{text}").map_err(CliError::from_stdout)
}

/// What a whole-page reset says, Go's `profile reset` texts. `what` is
/// `all settings` or, for one section, `all DLSS settings`. `cleared` is
/// `None` when the scope has no profile file.
pub fn clear_all_message(target: &Target, what: &str, cleared: Option<usize>) -> String {
    let name = target.label();
    match (target.is_all_games(), cleared) {
        (true, None | Some(0)) => format!("{name} already uses the system default for {what}."),
        (true, Some(n)) => format!("{name} now uses the system default for {what} ({n} cleared)."),
        (false, None) => {
            format!("No profile for {name}; it already follows {ALL_GAMES} for {what}.")
        }
        (false, Some(0)) => format!("{name} already follows {ALL_GAMES} for {what}."),
        (false, Some(n)) => format!("{name} now follows {ALL_GAMES} for {what} ({n} cleared)."),
    }
}

/// What a one-field reset says, Go's texts. `exists` is whether the scope
/// has a profile file; `cleared` whether the field had an own value.
pub fn follow_message(target: &Target, label: &str, exists: bool, cleared: bool) -> String {
    let name = target.label();
    match (target.is_all_games(), exists, cleared) {
        (true, _, true) => format!("{name}: {label} now uses the system default."),
        (true, _, false) => format!("{name}: {label} already uses the system default."),
        (false, false, _) => format!("No profile for {name}; {label} already follows {ALL_GAMES}."),
        (false, true, false) => format!("{name}: {label} already follows {ALL_GAMES}."),
        (false, true, true) => format!("{name}: {label} now follows {ALL_GAMES}."),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::config;
    use spela_core::lock::LockError;
    use spela_core::profile::FieldKey;

    #[test]
    fn store_errors_get_their_exit_codes() {
        let invalid = ProfileError::Invalid {
            path: "/c/p.toml".into(),
            errors: Vec::new(),
        };
        assert_eq!(profile_error(invalid).code, exit::CONFIG);
        let field = ProfileError::Field(FieldError::InvalidBool { raw: "x".into() });
        assert_eq!(profile_error(field).code, exit::USAGE);
        let io = ProfileError::Io {
            path: "/c/p.toml".into(),
            source: std::io::Error::other("disk full"),
        };
        assert_eq!(profile_error(io).code, exit::FAILURE);
        let held = ProfileError::Lock(LockError::Held {
            path: "/s/config.lock".into(),
            pid: Some(7),
        });
        assert_eq!(profile_error(held).code, exit::LOCKED);
        let unknown = FieldError::Unknown("x".into());
        assert_eq!(field_error(unknown).code, exit::USAGE);
        let bad = ListError::Invalid {
            path: "/c/lists/overrides.toml".into(),
            message: "no".into(),
        };
        assert_eq!(list_error(bad).code, exit::CONFIG);
        let set = ConfigStoreError::Set(config::ConfigSetError("unknown config key: x".into()));
        let e = config_store_error(set);
        assert_eq!(
            (e.code, e.message.as_str()),
            (exit::USAGE, "unknown config key: x")
        );
        assert_eq!(
            FieldKey::GpuFanSpeed
                .field()
                .parse("x")
                .map_err(field_error)
                .unwrap_err()
                .code,
            exit::USAGE
        );
    }

    #[test]
    fn labels_name_the_game_or_all_games() {
        let game = Target {
            scope: Scope::Game(1),
            name: "Hollow Crown 3".into(),
        };
        assert_eq!(game.label(), "Hollow Crown 3");
        assert!(!game.is_all_games());
        let all = Target {
            scope: Scope::AllGames,
            name: String::new(),
        };
        assert_eq!(all.label(), "All games");
    }

    #[test]
    fn reset_texts_are_go_s() {
        let game = Target {
            scope: Scope::Game(1),
            name: "Hollow Crown 3".into(),
        };
        let all = Target {
            scope: Scope::AllGames,
            name: String::new(),
        };
        let what = "all settings";
        assert_eq!(
            clear_all_message(&game, what, None),
            "No profile for Hollow Crown 3; it already follows All games for all settings."
        );
        assert_eq!(
            clear_all_message(&game, what, Some(0)),
            "Hollow Crown 3 already follows All games for all settings."
        );
        assert_eq!(
            clear_all_message(&game, what, Some(3)),
            "Hollow Crown 3 now follows All games for all settings (3 cleared)."
        );
        assert_eq!(
            clear_all_message(&all, "all GPU settings", Some(2)),
            "All games now uses the system default for all GPU settings (2 cleared)."
        );
        assert_eq!(
            clear_all_message(&all, what, None),
            "All games already uses the system default for all settings."
        );
        assert_eq!(
            follow_message(&game, "HDR", false, false),
            "No profile for Hollow Crown 3; HDR already follows All games."
        );
        assert_eq!(
            follow_message(&game, "HDR", true, false),
            "Hollow Crown 3: HDR already follows All games."
        );
        assert_eq!(
            follow_message(&game, "HDR", true, true),
            "Hollow Crown 3: HDR now follows All games."
        );
        assert_eq!(
            follow_message(&all, "HDR", true, true),
            "All games: HDR now uses the system default."
        );
        assert_eq!(
            follow_message(&all, "HDR", false, false),
            "All games: HDR already uses the system default."
        );
    }
}
