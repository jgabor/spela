//! `spela launch`: what launching a game would do, as a dry run only.
//! Only the wrapper (`spela %command%`) launches a game (decision 40).

use std::io::Write;

use clap::{ArgGroup, Args};

use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};

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
    _ctx: &mut Context,
    _out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    dry_run_only(args)?;
    Err(CliError::not_implemented("launch --dry-run"))
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
    .with_hint("set the game's Steam launch options to `spela %command%` instead")
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
