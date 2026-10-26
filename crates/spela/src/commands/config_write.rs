//! `spela config set <key> <value>`: every setting the TUI's Settings
//! shows, not only Go's three (decision 20).
//!
//! The keys keep Go's names, and the `[ui]` settings are `ui.theme`,
//! `ui.mouse` and `ui.ascii_glyphs`. Bools take spela's one bool
//! vocabulary, and `additional_library_paths` takes folders joined by `:`
//! or a JSON list. The write changes only that key in `config.toml`, under
//! `config.lock`, and replaces the file atomically with fsync, where Go
//! rewrote `config.yaml` in place. The output is Go's
//! `Set <key> = <value>`.

use std::io::Write;

use clap::Args;
use spela_core::config::{OPTIONS, store};

use crate::cli::Context;
use crate::commands::target::{config_store_error, line};
use crate::error::CliError;

/// `config set` arguments.
#[derive(Debug, Clone, PartialEq, Eq, Args)]
#[command(after_help = keys_help())]
pub struct SetArgs {
    /// The setting, such as check_updates or ui.theme.
    pub key: String,
    /// The new value. Bools take true, false, yes, no, on, off, 1 or 0.
    #[arg(allow_hyphen_values = true)]
    pub value: String,
}

fn keys_help() -> String {
    let keys: Vec<&str> = OPTIONS.iter().map(|o| o.key).collect();
    format!("Keys: {}.", keys.join(", "))
}

/// Runs `config set`.
pub fn set(args: &SetArgs, ctx: &mut Context, out: &mut dyn Write) -> Result<(), CliError> {
    store::set(&ctx.paths, &args.key, &args.value).map_err(config_store_error)?;
    line(out, &format!("Set {} = {}", args.key, args.value))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_help_lists_every_key() {
        let help = keys_help();
        for option in &OPTIONS {
            assert!(help.contains(option.key), "{}", option.key);
        }
        assert!(
            help.starts_with("Keys: confirm_destructive, ui.theme, "),
            "{help}"
        );
    }
}
