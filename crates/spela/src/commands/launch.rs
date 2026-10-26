//! `spela launch`: what launching a game would do, as a dry run only.
//! Only the wrapper (`spela %command%`) launches a game (decision 40).
//!
//! The dry run prints the plan wrapper mode would carry out, built by the
//! same function (architecture, "One plan for dry run and launch"). It
//! reads the game cache as it is, never runs pkexec or `kscreen-doctor`,
//! and takes no lock. A profile that does not load exits 3 and names the
//! file.

use std::io::Write;

use clap::{ArgGroup, Args};

use spela_core::games::GameNotFound;
use spela_core::launch::summary::{self, Mode, STEAM_HINT, Tone};
use spela_core::launch::{LaunchPlan, ProfileState};
use spela_core::profile::Scope;

use super::read::{self, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};
use crate::plan::SystemProbes;
use crate::text::Paint;

/// `launch` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
#[command(group(ArgGroup::new("target").required(true).args(["game", "game_id"])))]
pub struct LaunchArgs {
    /// The game: a Steam AppID or its exact name.
    pub game: Option<String>,
    /// The game by Steam AppID.
    #[arg(long, value_name = "N")]
    pub game_id: Option<u64>,
    /// Print what a launch would do, and change nothing.
    #[arg(long)]
    pub dry_run: bool,
}

/// Runs `spela launch`. Without `--dry-run` it cannot track the game, so it
/// exits 2 with a hint (decision 40).
pub fn run(
    args: &LaunchArgs,
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    dry_run_only(args)?;
    let plan = dry_run_plan(args, ctx)?;
    let paint = Paint::new(ctx.colors.out);
    let text =
        summary::summary(&plan, &Mode::DryRun, crate::privilege::PKEXEC).render(|tone, text| {
            match tone {
                Tone::Name => paint.primary(text),
                Tone::Heading => paint.secondary(text),
                Tone::Accent => paint.accent(text),
                Tone::Dim => paint.dim(text),
                Tone::Plain => text.to_owned(),
            }
        });
    read::write_out(out, &text)
}

/// The plan for the game the arguments name.
fn dry_run_plan(args: &LaunchArgs, ctx: &mut Context) -> Result<LaunchPlan, CliError> {
    let cache = read::games(ctx, Rescan::Never)?;
    let game = match (args.game_id, &args.game) {
        (Some(id), _) => cache.get(id).ok_or_else(|| {
            CliError::from(GameNotFound(id.to_string())).with_hint(read::SCAN_HINT)
        })?,
        (None, Some(query)) => read::find_game(&cache, query)?,
        (None, None) => {
            return Err(CliError::new(exit::USAGE, "name a game or pass --game-id"));
        }
    };
    let config = read::config(ctx)?;
    let view = read::profile(ctx, Scope::Game(game.app_id), &game.name)?;
    let profile = ProfileState {
        effective: view.effective,
        game_file: view.has_profile,
        all_games_file: view.all_games_file,
    };
    let (env, demo) = (&ctx.env, ctx.demo);
    let gpu = &mut ctx.backends.gpu;
    let mut install = || read::steam_install(env, demo, &config);
    let mut driver = || crate::plan::driver_version(gpu.as_mut());
    let mut probes = SystemProbes {
        install: &mut install,
        driver: &mut driver,
    };
    Ok(crate::plan::build(
        &ctx.paths,
        &config,
        game,
        &profile,
        &mut probes,
    ))
}

/// A launch without `--dry-run` cannot track the game (decision 40).
fn dry_run_only(args: &LaunchArgs) -> Result<(), CliError> {
    if args.dry_run {
        return Ok(());
    }
    Err(CliError::new(
        exit::USAGE,
        "direct Steam URI launch cannot track the game lifetime or cleanup coverage",
    )
    .with_hint(STEAM_HINT)
    .with_kind("launch_needs_wrapper"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn launch_without_dry_run_points_at_steam() {
        let e = dry_run_only(&LaunchArgs {
            game: Some("x".into()),
            game_id: None,
            dry_run: false,
        })
        .unwrap_err();
        assert_eq!(e.code, exit::USAGE);
        assert!(
            e.message
                .contains("cannot track the game lifetime or cleanup coverage")
        );
        assert!(e.hint.unwrap().contains("spela %command%"));
    }
}
