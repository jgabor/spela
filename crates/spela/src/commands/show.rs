//! `spela show <game>`: one game, its DLLs and whether it has a profile.
//!
//! The same rescan rule as `list`: only an empty cache is scanned first.
//! The text is Go's: `Name:`, `App ID:`, `Install:`, `Prefix:` when there
//! is one, `Detected DLLs:` with each DLL's version and path, and
//! `Profile: yes|no`. A game whose Go profile did not migrate still has a
//! profile, a broken one.

use std::fmt::Write as _;
use std::io::Write;

use clap::Args;
use spela_core::games::Game;

use super::read::{self, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::CliError;
use crate::text::Paint;

/// `show` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct ShowArgs {
    /// The game: a Steam AppID or its exact name.
    pub game: String,
}

/// Runs `spela show`.
pub fn run(
    args: &ShowArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let cache = read::games(ctx, Rescan::IfEmpty)?;
    let game = read::find_game(&cache, &args.game)?;
    let has_profile = read::has_profile(ctx, game.app_id);
    if inv.cli.json {
        return read::write_json(out, &spela_core::json::game_show(game, has_profile));
    }
    read::write_out(out, &text(game, has_profile, Paint::new(ctx.colors.out)))
}

/// The text of one game.
pub fn text(game: &Game, has_profile: bool, paint: Paint) -> String {
    let mut text = String::new();
    let _ = writeln!(
        text,
        "{}  {}",
        paint.dim("Name:"),
        paint.primary(&game.name)
    );
    let _ = writeln!(text, "{} {}", paint.dim("App ID:"), game.app_id);
    let _ = writeln!(
        text,
        "{}  {}",
        paint.dim("Install:"),
        game.install_dir.display()
    );
    if let Some(prefix) = &game.prefix_path {
        let _ = writeln!(text, "{}  {}", paint.dim("Prefix:"), prefix.display());
    }
    if !game.dlls.is_empty() {
        let _ = writeln!(text, "\n{}", paint.secondary("Detected DLLs:"));
        for dll in read::dlls_in_path_order(game) {
            let _ = writeln!(
                text,
                "  {}: {}",
                paint.primary(&dll.name),
                paint.accent(read::version_text(dll.version.as_deref()))
            );
            let _ = writeln!(text, "    {}", paint.dim(&dll.path.display().to_string()));
        }
    }
    let status = if has_profile {
        paint.success("yes")
    } else {
        paint.dim("no")
    };
    let _ = writeln!(text, "\n{} {status}", paint.dim("Profile:"));
    text
}
