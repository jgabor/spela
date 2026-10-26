//! The read side of `spela config`: `config show`.
//!
//! It prints every setting as TOML, the new format (decision 6), with
//! defaults filled in, in the order of the architecture's `config.toml`
//! listing. Go printed its YAML the same way, defaults included. An empty
//! `shader_cache` prints the folder it means, `<cache>/nvidia`, as Go
//! printed its computed default.

use std::io::Write;

use super::read;
use crate::cli::{Context, Invocation};
use crate::error::{CliError, exit};

/// Runs `spela config show`.
pub fn show(
    _inv: &Invocation,
    ctx: &mut Context,
    out: &mut dyn Write,
    _err: &mut dyn Write,
) -> Result<(), CliError> {
    let config = read::config(ctx)?;
    let mut shown = config.clone();
    if shown.shader_cache.is_empty() {
        shown.shader_cache = config.shader_cache_dir(&ctx.paths).display().to_string();
    }
    let text = toml::to_string(&shown).map_err(|e| {
        CliError::new(
            exit::FAILURE,
            format!("cannot write the config as TOML: {e}"),
        )
    })?;
    read::write_out(out, &text)
}
