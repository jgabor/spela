//! The hidden `apply-profile` command: the root helper under polkit.
//!
//! It runs as root with root's environment, so it does no per-user setup:
//! no XDG paths, no migration, no log file. `cli::main` dispatches it before
//! any of those start. Phase 6 builds its two modes (decision 30):
//! `--serve` for a launch and `--once <request>` for the live commands.
//! Go's flag form is refused with a clear error and exit 2 (decision 34).

use std::io::Write;

use clap::Args;

use crate::error::{CliError, exit};

/// What the helper prints for any argument form it does not speak.
pub const GO_FLAGS_MESSAGE: &str = concat!(
    "apply-profile: this is the spela ",
    env!("CARGO_PKG_VERSION_MAJOR"),
    ".",
    env!("CARGO_PKG_VERSION_MINOR"),
    " helper; the Go helper's flags are not supported"
);

/// `apply-profile` arguments, kept as raw strings. clap never validates
/// them and the command has no help flag, so every form the helper does
/// not speak, Go's `--reset` and flag forms, `--help` and a bare
/// `--once` included, gets [`GO_FLAGS_MESSAGE`] and exit 2.
#[derive(Debug, Clone, Default, PartialEq, Eq, Args)]
pub struct HelperArgs {
    /// Everything after `apply-profile`.
    #[arg(
        trailing_var_arg = true,
        allow_hyphen_values = true,
        hide = true,
        value_name = "ARGS"
    )]
    pub args: Vec<String>,
}

/// The two forms the helper speaks (decision 30).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Mode {
    /// `--serve`: apply the batch read from stdin, then wait to restore it.
    Serve,
    /// `--once <request>`: apply this one JSON request and exit without
    /// restoring.
    Once(String),
}

impl HelperArgs {
    /// The mode these arguments ask for, or `None` for any other form.
    pub fn mode(&self) -> Option<Mode> {
        match self.args.as_slice() {
            [serve] if serve == "--serve" => Some(Mode::Serve),
            [once, request] if once == "--once" && !request.starts_with('-') => {
                Some(Mode::Once(request.clone()))
            }
            [once] => once
                .strip_prefix("--once=")
                .filter(|r| !r.is_empty())
                .map(|r| Mode::Once(r.to_owned())),
            _ => None,
        }
    }
}

/// What the helper does with `args`. In this build that is always an
/// error: other forms exit 2, and the two modes are not built yet.
pub fn check(args: &HelperArgs) -> CliError {
    match args.mode() {
        None => CliError::new(exit::USAGE, GO_FLAGS_MESSAGE).with_kind("go_helper_flags"),
        Some(_) => CliError::not_implemented("apply-profile"),
    }
}

/// Runs the helper and returns its exit code. The helper's lines go to
/// `err` as they are, without the CLI's `error:` prefix.
pub fn run(args: &HelperArgs, err: &mut dyn Write) -> u8 {
    let e = check(args);
    let message = if e.kind == "not_implemented" {
        "apply-profile: not yet implemented in this build"
    } else {
        e.message.as_str()
    };
    let _ = writeln!(err, "{message}");
    e.code
}

#[cfg(test)]
mod tests {
    use super::*;

    fn args(raw: &[&str]) -> HelperArgs {
        HelperArgs {
            args: raw.iter().map(|s| (*s).to_owned()).collect(),
        }
    }

    fn run_text(a: &HelperArgs) -> (u8, String) {
        let mut err = Vec::new();
        let code = run(a, &mut err);
        (code, String::from_utf8(err).unwrap())
    }

    #[test]
    fn every_other_form_is_refused_with_exit_2() {
        for raw in [
            &["--reset"][..],
            &["--clock-offset", "100"][..],
            &[][..],
            &["--serve", "--governor=performance"][..],
            &["--serve", "--once", "{}"][..],
            &["--once"][..],
            &["--once="][..],
            &["--once", "--serve"][..],
            &["--help"][..],
            &["-h"][..],
            &["--serve", "--reset"][..],
        ] {
            let (code, text) = run_text(&args(raw));
            assert_eq!(code, exit::USAGE, "{raw:?}");
            assert_eq!(
                text,
                "apply-profile: this is the spela 0.7 helper; the Go helper's flags are not supported\n",
                "{raw:?}"
            );
        }
    }

    #[test]
    fn the_two_modes_parse_but_are_not_built_yet() {
        assert_eq!(args(&["--serve"]).mode(), Some(Mode::Serve));
        assert_eq!(
            args(&["--once", "{}"]).mode(),
            Some(Mode::Once("{}".into()))
        );
        assert_eq!(args(&["--once={}"]).mode(), Some(Mode::Once("{}".into())));
        for raw in [&["--serve"][..], &["--once", "{}"][..]] {
            assert_eq!(run_text(&args(raw)).0, exit::FAILURE, "{raw:?}");
        }
    }
}
