//! The read side of `spela denylist`: `denylist show` and `denylist check`.
//!
//! The text is Go's, with the corrected `show` layout: `User-denied games:`
//! only when there are some, after a blank line, and `[FORCE-ALLOWED]` only
//! on anti-cheat entries. A user `denylist.toml` adds to the built-in list
//! (decision 36).
//!
//! The lists fail closed (decision 8): a list file that does not load, or a
//! Go list that did not migrate, is an error that names the file and exits
//! 3, never a quiet answer from the built-in list alone.

use std::fmt::Write as _;
use std::io::Write;

use clap::Args;
use spela_core::denylist::{self, DenyLists, ListError};
use spela_core::migrate::SourceKind;

use super::read::{self, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};

/// `denylist check` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
pub struct CheckArgs {
    /// The game: a Steam AppID or its exact name. An AppID that is not in
    /// the game list is checked as it is.
    pub game: String,
}

/// Runs `spela denylist show`.
pub fn show(
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let lists = load(ctx)?;
    let mut text = String::from("Denied games (anti-cheat):\n");
    for listed in lists.anti_cheat() {
        let e = listed.entry;
        let status = if lists.is_force_allowed(e.app_id) {
            " [FORCE-ALLOWED]"
        } else {
            ""
        };
        let _ = writeln!(text, "  {} ({}): {}{status}", e.name, e.app_id, e.reason);
    }
    if !lists.user_denied().is_empty() {
        text.push_str("\nUser-denied games:\n");
        for e in lists.user_denied() {
            let _ = writeln!(text, "  {} ({}): {}", e.name, e.app_id, e.reason);
        }
    }
    read::write_out(out, &text)
}

/// Runs `spela denylist check <game>`.
pub fn check(
    args: &CheckArgs,
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let cache = read::games(ctx, Rescan::Never)?;
    let (app_id, name) = read::game_or_app_id(&cache, &args.game)?;
    let lists = load(ctx)?;
    let status = lists.check(app_id);
    let text = match status.reason() {
        Some(reason) => format!(
            "{name} is DENIED: {reason}\nUse 'spela denylist allow' to force-allow (not recommended)\n"
        ),
        None => format!("{name} is allowed for DLL swapping\n"),
    };
    read::write_out(out, &text)
}

/// The lists, failing closed on a Go list that did not migrate or a list
/// file that does not load.
fn load(ctx: &Context) -> Result<DenyLists, CliError> {
    for failed in ctx.migration.failed() {
        if matches!(
            failed.kind,
            SourceKind::DenyOverrides | SourceKind::UserDenylist
        ) {
            return Err(read::not_migrated(&failed.entry.path, &failed.entry.error)
                .with_kind("list_invalid"));
        }
    }
    denylist::load(&ctx.paths).map_err(list_error)
}

fn list_error(e: ListError) -> CliError {
    let code = match e {
        ListError::Invalid { .. } | ListError::Newer { .. } => exit::CONFIG,
        ListError::Read { .. } | ListError::Write { .. } | ListError::Lock(_) => exit::FAILURE,
    };
    CliError::new(code, e.to_string()).with_kind("list_invalid")
}
