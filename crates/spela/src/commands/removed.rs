//! The reserved names `overlay` and `gui`. They print that the command was
//! removed in 0.7.0 and exit 2, and a program on PATH with either name is
//! never launched for them (decision 45).

use crate::error::{CliError, exit};

/// `spela overlay ...`: the overlay had no reader (decision 10).
pub fn overlay() -> CliError {
    removed(
        "overlay",
        "the overlay had no reader, so its settings were dropped",
    )
}

/// `spela gui ...`: the GUI is gone (decision 26).
pub fn gui() -> CliError {
    removed("gui", "use 'spela tui' for the interactive interface")
}

fn removed(name: &str, hint: &str) -> CliError {
    CliError::new(
        exit::USAGE,
        format!("the {name} command was removed in 0.7.0"),
    )
    .with_hint(hint)
    .with_kind("removed")
}
