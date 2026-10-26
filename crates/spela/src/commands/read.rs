//! What the read commands share: the config, the Steam install, the game
//! cache with its rescan rule, finding a game, and a scope's profile.
//!
//! The rescan rule is decision 24: the CLI rescans only on an empty cache or
//! `spela scan`. `list` and `show` rescan an empty cache; every other read
//! command takes the cache as it is, as in Go, where only `list`, `show` and
//! the TUI refreshed it.
//!
//! A profile that does not load, or whose Go file did not migrate, exits 3
//! and names the file (architecture, "Profiles and inheritance").

use std::path::Path;

use spela_core::config::{self, Config};
use spela_core::env::var;
use spela_core::games::scan::{self, RescanError};
use spela_core::games::{Game, GameCache, cache};
use spela_core::migrate::CONFIG_HINT;
use spela_core::profile::store::{self, ProfileError};
use spela_core::profile::{Effective, Scope};
use spela_core::steam::library::SteamInstall;

use crate::cli::Context;
use crate::error::{CliError, exit};

/// The hint after a game that is not in the cache.
pub const SCAN_HINT: &str = "run 'spela scan' to refresh the game list";

/// Whether a command rescans an empty game cache before it reads it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Rescan {
    /// `list` and `show`: an empty cache is scanned first.
    IfEmpty,
    /// Every other read command: the cache as it is.
    Never,
}

/// The config, with defaults for a missing file. A file that does not load
/// exits 3. Unknown keys are warnings on stderr.
pub fn config(ctx: &Context) -> Result<Config, CliError> {
    let loaded = config::load(&ctx.paths.config_file())?;
    for warning in &loaded.warnings {
        tracing::warn!("{}: {warning}", ctx.paths.config_file().display());
    }
    Ok(loaded.config)
}

/// The Steam install the config and `HOME` point at. `--demo` never looks
/// at the user's Steam, so it gets no install.
pub fn steam(ctx: &Context, config: &Config) -> SteamInstall {
    if ctx.demo {
        return SteamInstall::default();
    }
    let home = ctx.env.non_empty(var::HOME).map(Path::new);
    let install =
        SteamInstall::discover(&config.steam_path, &config.additional_library_paths, home);
    for problem in &install.problems {
        if !problem.is_not_found() {
            tracing::warn!("{problem}");
        }
    }
    install
}

/// The game cache. With [`Rescan::IfEmpty`], an empty cache is rebuilt
/// from Steam first; with no Steam to scan that is Go's error.
pub fn games(ctx: &Context, rescan: Rescan) -> Result<GameCache, CliError> {
    let cache = cache::load(&ctx.paths);
    if rescan == Rescan::Never || !cache.is_empty() {
        return Ok(cache);
    }
    let config = config(ctx)?;
    let install = steam(ctx, &config);
    tracing::info!("the game cache is empty, so spela scans Steam first");
    match scan::rescan(&ctx.paths, &install, ctx.clock.as_ref()) {
        Ok(_) => Ok(cache::load(&ctx.paths)),
        Err(RescanError::NoSteam) => Err(no_steam("failed to refresh game database")),
        Err(e) => Err(CliError::new(
            exit::FAILURE,
            format!("failed to refresh game database: {e}"),
        )),
    }
}

/// Go's error when there is no Steam library to scan, after `context`.
pub fn no_steam(context: &str) -> CliError {
    CliError::new(exit::FAILURE, format!("{context}: {}", scan::NO_STEAM))
        .with_hint("set steam_path with 'spela config set steam_path <folder>'")
        .with_kind("no_steam")
}

/// The game a `<game>` argument names: an AppID, then an exact name.
pub fn find_game<'a>(cache: &'a GameCache, query: &str) -> Result<&'a Game, CliError> {
    cache
        .find(query)
        .map_err(|e| CliError::from(e).with_hint(SCAN_HINT))
}

/// The game a `<game>` argument names, or, for an unknown number, that
/// number with itself as the name, as the deny list commands accept it.
pub fn game_or_app_id(cache: &GameCache, query: &str) -> Result<(u64, String), CliError> {
    match cache.find(query) {
        Ok(game) => Ok((game.app_id, game.name.clone())),
        Err(e) => match query.parse::<u64>() {
            Ok(app_id) => Ok((app_id, query.to_owned())),
            Err(_) => Err(CliError::from(e).with_hint(SCAN_HINT)),
        },
    }
}

/// Whether a game has its own profile: its TOML file exists, or its Go
/// profile is there but did not migrate, which is a broken profile.
pub fn has_profile(ctx: &Context, app_id: u64) -> bool {
    store::has_profile(&ctx.paths, app_id) || ctx.migration.failed_profile(app_id).is_some()
}

/// A scope's resolved profile and whether its file exists.
#[derive(Debug, Clone)]
pub struct ProfileView {
    /// The game's name, or `All games`.
    pub name: String,
    /// Whether the scope has its own profile file.
    pub has_profile: bool,
    /// Every field, resolved.
    pub effective: Effective,
}

/// Loads a scope's profile and All games, and resolves every field. A
/// profile that does not load, or did not migrate from Go, exits 3.
pub fn profile(ctx: &Context, scope: Scope, name: &str) -> Result<ProfileView, CliError> {
    if let Some(error) = ctx.migration.failed_all_games() {
        return Err(not_migrated(&ctx.paths.go().all_games_profile(), error));
    }
    let all_games = store::load_all_games(&ctx.paths).map_err(profile_error)?;
    if let Some(loaded) = &all_games {
        warn_profile(&ctx.paths.all_games_profile(), loaded.warnings.iter());
    }
    let all_values = all_games.as_ref().map(|l| &l.profile.values);
    match scope {
        Scope::AllGames => Ok(ProfileView {
            name: spela_core::profile::words::ALL_GAMES.to_owned(),
            has_profile: all_games.is_some(),
            effective: Effective::of_all_games(all_values),
        }),
        Scope::Game(app_id) => {
            if let Some(error) = ctx.migration.failed_profile(app_id) {
                return Err(not_migrated(&ctx.paths.go().game_profile(app_id), error));
            }
            let game = store::load_game(&ctx.paths, app_id).map_err(profile_error)?;
            if let Some(loaded) = &game {
                warn_profile(&ctx.paths.game_profile(app_id), loaded.warnings.iter());
            }
            Ok(ProfileView {
                name: name.to_owned(),
                has_profile: game.is_some(),
                effective: Effective::of_game(
                    app_id,
                    game.as_ref().map(|l| &l.profile.values),
                    all_values,
                ),
            })
        }
    }
}

fn warn_profile<'a>(path: &Path, warnings: impl Iterator<Item = &'a config::Diagnostic>) {
    for warning in warnings {
        tracing::warn!("{}: {warning}", path.display());
    }
}

/// The error for a Go file that did not migrate: exit 3, with Go's advice.
pub fn not_migrated(path: &Path, error: &str) -> CliError {
    CliError::new(
        exit::CONFIG,
        format!("cannot migrate Go spela's file {}: {error}", path.display()),
    )
    .with_hint(CONFIG_HINT)
}

/// A profile that does not load: exit 3 when the file is invalid, 1 when
/// it cannot be read.
pub fn profile_error(e: ProfileError) -> CliError {
    let code = match e {
        ProfileError::Io { .. } | ProfileError::Lock(_) => exit::FAILURE,
        ProfileError::Invalid { .. } | ProfileError::ReadOnly { .. } | ProfileError::Field(_) => {
            exit::CONFIG
        }
    };
    CliError::new(code, e.to_string())
}

/// Writes text to stdout, mapping a closed pipe to a quiet exit.
pub fn write_out(out: &mut dyn std::io::Write, text: &str) -> Result<(), CliError> {
    out.write_all(text.as_bytes())
        .map_err(CliError::from_stdout)
}

/// Writes a `--json` object to stdout.
pub fn write_json(
    out: &mut dyn std::io::Write,
    value: &impl serde::Serialize,
) -> Result<(), CliError> {
    let text = spela_core::json::to_text(value)
        .map_err(|e| CliError::new(exit::FAILURE, format!("cannot write the JSON output: {e}")))?;
    write_out(out, &text)
}

/// A game's DLLs in Go's text order: by path, which is the order Go's walk
/// found them. JSON uses the catalogue order instead.
pub fn dlls_in_path_order(game: &Game) -> Vec<&spela_core::games::DetectedDll> {
    let mut dlls: Vec<_> = game.dlls.iter().collect();
    dlls.sort_by(|a, b| a.path.cmp(&b.path));
    dlls
}

/// A DLL's version for text, `unknown` when it has none.
pub fn version_text(version: Option<&str>) -> &str {
    version.filter(|v| !v.is_empty()).unwrap_or("unknown")
}

/// The listed games with any DLL, by name and then app id: what `scan`
/// counts and `dll list` and `dll check-updates` show. Go counted only
/// DLSS games here, against its own DLSS/FSR/XeSS label (decision 8).
pub fn games_with_dlls(cache: &GameCache) -> Vec<&Game> {
    let mut games: Vec<&Game> = cache.listed().filter(|g| !g.dlls.is_empty()).collect();
    sort_by_name(&mut games);
    games
}

/// Sorts games by name, then app id.
pub fn sort_by_name(games: &mut [&Game]) {
    games.sort_by(|a, b| a.name.cmp(&b.name).then(a.app_id.cmp(&b.app_id)));
}
