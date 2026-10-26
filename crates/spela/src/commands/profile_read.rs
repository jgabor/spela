//! The read side of the profile commands: `profile list`, `profile show`
//! and the section shows `dlss show`, `gpu show`, `cpu show` and
//! `proton show`. The write side lives in [`super::profile`] and the
//! section files.
//!
//! A show resolves every field the way launch does and prints each one as
//! `<label>  <value>  <source>` with aligned columns, in the words of
//! [`spela_core::profile::words`]. It works without a profile file, where
//! Go's `profile show --json` failed. `--all-games` shows the All games
//! profile itself (decision 20). `--json` prints the schema version 1
//! profile object, with only the section's fields for a section show.
//!
//! The Overlay section, `gpu.power_mizer` and `cpu.affinity` are gone
//! (decision 10).

use std::fmt::Write as _;
use std::io::Write;

use clap::{ArgGroup, Args};
use spela_core::backend::GpuError;
use spela_core::profile::store;
use spela_core::profile::words::{self, ALL_GAMES};
use spela_core::profile::{FieldKey, Origin, Scope, Section, Value};
use spela_core::proton;

use super::read::{self, ProfileView, Rescan};
use crate::cli::{Context, Invocation};
use crate::error::CliError;
use crate::text::Paint;

/// The scope a show reads: a game, or All games.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
#[command(group(ArgGroup::new("target").required(true).args(["game", "all_games"])))]
pub struct ShowArgs {
    /// The game: a Steam AppID or its exact name.
    pub game: Option<String>,
    /// Show the All games profile, which every game follows.
    #[arg(long)]
    pub all_games: bool,
}

/// Runs `spela profile list`: the games with their own profile, by name.
pub fn list(
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let listing = store::list(&ctx.paths).map_err(crate::commands::target::profile_error)?;
    for (app_id, e) in &listing.broken {
        tracing::warn!(app_id, "skipping a profile that does not load: {e}");
    }
    // A Go profile that did not migrate is not listed; the migration
    // already warned about it on stderr.
    if listing.profiles.is_empty() {
        return read::write_out(out, "No profiles found.\n");
    }
    let cache = read::games(ctx, Rescan::Never)?;
    let mut rows: Vec<(String, u64, String)> = listing
        .profiles
        .iter()
        .map(|loaded| {
            let p = &loaded.profile;
            let shown = cache
                .get(p.app_id)
                .map_or_else(|| p.app_id.to_string(), |g| g.name.clone());
            let name = if p.name.is_empty() {
                "custom".to_owned()
            } else {
                p.name.clone()
            };
            (shown, p.app_id, name)
        })
        .collect();
    rows.sort();
    let paint = Paint::new(ctx.colors.out);
    let mut text = String::new();
    for (shown, app_id, name) in rows {
        let _ = writeln!(
            text,
            "{} {}: {}",
            paint.primary(&shown),
            paint.dim(&format!("({app_id})")),
            paint.secondary(&name)
        );
    }
    read::write_out(out, &text)
}

/// Runs `spela profile show`: every section.
pub fn show(
    args: &ShowArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let view = load(args, ctx)?;
    if inv.cli.json {
        return read::write_json(out, &json(&view, None));
    }
    let paint = Paint::new(ctx.colors.out);
    let mut text = format!("Profile for {}:\n\n", paint.primary(&view.name));
    text.push_str(&no_profile_note(&view));
    for section in Section::ALL {
        let _ = writeln!(text, "{}", paint.primary(section.title()));
        for line in rows(&view, section, paint) {
            let _ = writeln!(text, "  {line}");
        }
        text.push('\n');
    }
    read::write_out(out, &text)
}

/// Runs a section show: `dlss show`, `gpu show`, `cpu show` or
/// `proton show`. `proton show` adds the descriptor heap notice when
/// vkd3d_heap resolves on.
pub fn section_show(
    section: Section,
    args: &ShowArgs,
    inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let view = load(args, ctx)?;
    if inv.cli.json {
        return read::write_json(out, &json(&view, Some(section)));
    }
    let paint = Paint::new(ctx.colors.out);
    let mut text = format!("{} for {}:\n\n", section_heading(section), view.name);
    text.push_str(&no_profile_note(&view));
    for line in rows(&view, section, paint) {
        let _ = writeln!(text, "{line}");
    }
    if section == Section::Proton
        && let Some(notice) = heap_notice(ctx, &view)?
    {
        let _ = writeln!(text, "    {}", paint.dim(&notice));
    }
    read::write_out(out, &text)
}

/// The heading of a section show, Go's words.
pub fn section_heading(section: Section) -> &'static str {
    match section {
        Section::Proton => "Proton profile",
        Section::Dlss => "DLSS configuration",
        Section::Gpu => "GPU profile",
        Section::Cpu => "CPU profile",
    }
}

fn load(args: &ShowArgs, ctx: &Context) -> Result<ProfileView, CliError> {
    if args.all_games {
        return read::profile(ctx, Scope::AllGames, ALL_GAMES);
    }
    let query = args.game.as_deref().unwrap_or_default();
    let cache = read::games(ctx, Rescan::Never)?;
    let game = read::find_game(&cache, query)?;
    read::profile(ctx, Scope::Game(game.app_id), &game.name)
}

fn json(view: &ProfileView, section: Option<Section>) -> crate::json::ProfileJson<'_> {
    crate::json::profile(&view.effective, &view.name, view.has_profile, section)
}

/// Go's note before the rows of a game without a profile file.
fn no_profile_note(view: &ProfileView) -> String {
    match view.effective.scope {
        Scope::Game(_) if !view.has_profile => format!(
            "No profile for {}; every value follows {ALL_GAMES} or the system default.\n\n",
            view.name
        ),
        _ => String::new(),
    }
}

/// One section's rows, `<label>  <value>  <source>`, with the columns
/// aligned over that section. The source is accented when the scope sets
/// the value itself.
pub fn rows(view: &ProfileView, section: Section, paint: Paint) -> Vec<String> {
    let cells: Vec<(&str, String, String, bool)> = view
        .effective
        .in_section(section)
        .map(|resolved| {
            let field = resolved.key.field();
            (
                field.label,
                words::value_text(field, resolved),
                words::source_text(field, resolved),
                matches!(
                    resolved.origin,
                    Origin::ThisGame | Origin::ThisGameSame | Origin::SetHere
                ),
            )
        })
        .collect();
    let width = |text: &str| text.chars().count();
    let label_width = cells.iter().map(|c| width(c.0)).max().unwrap_or(0);
    let value_width = cells.iter().map(|c| width(&c.1)).max().unwrap_or(0);
    cells
        .into_iter()
        .map(|(label, value, source, own)| {
            let source = if own {
                paint.accent(&source)
            } else {
                paint.dim(&source)
            };
            format!(
                "{}  {}  {source}",
                paint.dim(&pad(label, label_width)),
                pad(&value, value_width)
            )
        })
        .collect()
}

fn pad(text: &str, width: usize) -> String {
    let gap = width.saturating_sub(text.chars().count());
    format!("{text}{}", " ".repeat(gap))
}

/// The descriptor heap notice for a game whose vkd3d_heap resolves on:
/// the Proton build Steam would launch and the NVIDIA driver, each checked
/// on its own. `None` when all is well, or for All games, which has no
/// Proton build.
fn heap_notice(ctx: &mut Context, view: &ProfileView) -> Result<Option<String>, CliError> {
    let Scope::Game(app_id) = view.effective.scope else {
        return Ok(None);
    };
    if view.effective.value(FieldKey::ProtonVkd3dHeap) != &Value::Bool(true) {
        return Ok(None);
    }
    let config = read::config(ctx)?;
    let install = read::steam(ctx, &config);
    let build = proton::resolve(&install, app_id);
    // NVML missing takes Go's no-driver path; any other failure means the
    // driver could not be asked.
    let driver = match ctx.backends.gpu.info() {
        Ok(info) => Some(info.driver_version),
        Err(GpuError::Unavailable(why)) => {
            tracing::info!("no NVIDIA driver for the descriptor heap check: {why}");
            Some(String::new())
        }
        Err(e) => {
            tracing::info!("the NVIDIA driver could not be asked: {e}");
            None
        }
    };
    Ok(proton::check(&build, driver.as_deref()).notice())
}
