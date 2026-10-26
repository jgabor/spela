//! The writes of `spela profile`: `create`, `reset`, `delete` and `follow`.
//!
//! - `create <game>` makes an empty profile named after the game. A game
//!   that has one already is an error, as in Go.
//! - `reset <game>` makes every setting follow All games and keeps the file
//!   and its name. `reset --all-games` puts every All games setting back to
//!   the system default ("Use system default for all settings").
//! - `delete <game>` removes the game's profile. When there was none it
//!   says so, where Go reported a deletion. The migration record keeps the
//!   Go profile from coming back at the next start.
//! - `follow <field>... [--games <list>]` is the TUI's "Apply to games with
//!   their own value" (decisions 20 and 51): each listed game, or every game
//!   with a profile, that sets a field itself follows All games for it. One
//!   locked write per game, so a game that cannot be written is named and
//!   the others still change; the command then exits 1.

use std::io::Write;

use clap::Args;
use spela_core::migrate::SourceKind;
use spela_core::profile::store;
use spela_core::profile::words::{ALL_GAMES, follow_outcome_text};
use spela_core::profile::{FieldError, FieldKey, Scope, Section};

use crate::cli::Context;
use crate::commands::read::{self, Rescan};
use crate::commands::target::{
    clear_all_message, field_error, find_game, line, profile_error, target,
};
use crate::error::{CliError, exit};
use crate::migration;

/// `profile create` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct CreateArgs {
    /// The game: its Steam AppID or its exact name.
    pub game: String,
}

/// `profile reset` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct ResetArgs {
    /// The game: its Steam AppID or its exact name.
    #[arg(required_unless_present = "all_games")]
    pub game: Option<String>,
    /// Put every All games setting back to the system default.
    #[arg(long, conflicts_with = "game")]
    pub all_games: bool,
}

/// `profile delete` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct DeleteArgs {
    /// The game: its Steam AppID or its exact name.
    pub game: String,
}

/// `profile follow` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
#[command(after_help = "\
Fields are written as section.name, such as dlss.sr_mode, gpu.fan_speed or
proton.hdr. A game whose profile does not set a field is left alone.")]
pub struct FollowArgs {
    /// The fields that games should follow All games for.
    #[arg(required = true, value_name = "FIELD")]
    pub fields: Vec<String>,
    /// Only these games, by AppID or exact name, separated by commas.
    /// Without it, every game with a profile.
    #[arg(long, value_name = "GAMES", value_delimiter = ',')]
    pub games: Vec<String>,
}

/// Runs `profile create`.
pub fn create(args: &CreateArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let target = target(ctx, Some(&args.game), false)?;
    let Scope::Game(app_id) = target.scope else {
        unreachable!("a game target")
    };
    if !store::create(&ctx.paths, app_id, &target.name).map_err(profile_error)? {
        return Err(CliError::new(
            exit::FAILURE,
            format!("profile already exists for {}", target.name),
        ));
    }
    line(out, &format!("Created profile for {}", target.name))
}

/// Runs `profile reset`.
pub fn reset(args: &ResetArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let target = target(ctx, args.game.as_deref(), args.all_games)?;
    let cleared = store::follow_all(&ctx.paths, target.scope).map_err(profile_error)?;
    line(out, &clear_all_message(&target, "all settings", cleared))
}

/// Runs `profile delete`.
pub fn delete(args: &DeleteArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let target = target(ctx, Some(&args.game), false)?;
    let Scope::Game(app_id) = target.scope else {
        unreachable!("a game target")
    };
    let text = if store::delete(&ctx.paths, app_id).map_err(profile_error)? {
        format!("Deleted profile for {}", target.name)
    } else {
        format!("No profile for {}", target.name)
    };
    line(out, &text)
}

/// The field a `profile follow` argument names: `<section>.<name>`, with
/// dashes for underscores and the short names `reset` takes.
pub fn field_key(input: &str) -> Result<FieldKey, FieldError> {
    input
        .split_once('.')
        .and_then(|(section, _)| Section::from_name(section))
        .and_then(|section| section.lookup(input))
        .map(|f| f.id)
        .ok_or_else(|| FieldError::Unknown(input.to_owned()))
}

/// Runs `profile follow`.
pub fn follow(args: &FollowArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    let mut keys: Vec<FieldKey> = Vec::new();
    for input in &args.fields {
        let key = field_key(input).map_err(field_error)?;
        if !keys.contains(&key) {
            keys.push(key);
        }
    }
    let games = follow_games(ctx, &args.games)?;
    // A game whose Go profile did not migrate is not written: the retry at
    // the next start would adopt the new file in its place.
    let (refused, writable): (Vec<_>, Vec<_>) = games.into_iter().partition(|(app_id, _)| {
        migration::refuse_profile(&ctx.migration, Scope::Game(*app_id)).is_err()
    });
    let mut failed = false;
    for key in keys {
        let mut outcome = store::follow_all_games_in(&ctx.paths, &writable, &[key]);
        for (app_id, name) in &refused {
            let why = ctx
                .migration
                .failed()
                .find(|h| h.kind == SourceKind::Game(*app_id))
                .map(|h| format!("cannot migrate {}", h.entry.path.display()))
                .unwrap_or_default();
            outcome.failed.push(name.clone());
            outcome.errors.insert(name.clone(), why);
        }
        outcome.failed.sort();
        failed |= !outcome.failed.is_empty();
        line(out, &follow_outcome_text(key.field().label, &outcome))?;
    }
    if failed {
        return Err(CliError::new(
            exit::FAILURE,
            format!("some games could not be changed to follow {ALL_GAMES}"),
        ));
    }
    Ok(())
}

/// The games `profile follow` looks at, by app id and name: the listed
/// ones, or every game with a profile file, the broken ones included so
/// they are reported.
fn follow_games(ctx: &Context, queries: &[String]) -> Result<Vec<(u64, String)>, CliError> {
    let mut out: Vec<(u64, String)> = Vec::new();
    if !queries.is_empty() {
        for query in queries {
            let game = find_game(ctx, query)?;
            if !out.iter().any(|(id, _)| *id == game.app_id) {
                out.push((game.app_id, game.name));
            }
        }
        return Ok(out);
    }
    let listing = store::list(&ctx.paths).map_err(profile_error)?;
    let cache = read::games(ctx, Rescan::IfEmptyQuietly).unwrap_or_default();
    let name = |app_id: u64, stored: &str| -> String {
        cache
            .get(app_id)
            .map(|g| g.name.clone())
            .filter(|n| !n.is_empty())
            .or_else(|| (!stored.is_empty()).then(|| stored.to_owned()))
            .unwrap_or_else(|| app_id.to_string())
    };
    for loaded in &listing.profiles {
        let p = &loaded.profile;
        out.push((p.app_id, name(p.app_id, &p.name)));
    }
    for (app_id, _) in &listing.broken {
        out.push((*app_id, name(*app_id, "")));
    }
    for h in ctx.migration.failed() {
        if let SourceKind::Game(app_id) = h.kind
            && !out.iter().any(|(id, _)| *id == app_id)
        {
            out.push((app_id, name(app_id, "")));
        }
    }
    out.sort();
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn follow_fields_are_full_keys_with_go_s_spellings() {
        assert_eq!(field_key("dlss.sr_mode"), Ok(FieldKey::DlssSrMode));
        assert_eq!(field_key("dlss.sr-mode"), Ok(FieldKey::DlssSrMode));
        assert_eq!(field_key("proton.hdr"), Ok(FieldKey::ProtonEnableHdr));
        assert_eq!(
            field_key("gpu.threaded_opt"),
            Ok(FieldKey::GpuThreadedOptimization)
        );
        assert_eq!(field_key("dlss.fg"), Ok(FieldKey::DlssFgEnabled));
        for bad in [
            "sr_mode",
            "gpu.sr_mode",
            "overlay.enabled",
            "gpu.power_mizer",
            "",
        ] {
            assert_eq!(
                field_key(bad),
                Err(FieldError::Unknown(bad.to_owned())),
                "{bad}"
            );
        }
        assert_eq!(
            field_error(FieldError::Unknown("x".into())).message,
            "unknown profile field: \"x\""
        );
    }
}
