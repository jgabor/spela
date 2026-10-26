//! How errors and colour reach the user.
//!
//! Results go to stdout, and warnings and errors to stderr. Colour goes only
//! to a terminal, and `NO_COLOR` turns it off. Errors print `error:
//! <message>` and, where one helps, `hint: <one line>`, never the full usage
//! text. With `--json` a failure prints only an error object on stdout.

use std::io::{self, Write};

use anstyle::{AnsiColor, Style};
use serde::Serialize;
use spela_core::env::Env;

use crate::error::CliError;

/// Whether each stream gets colour.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct Colors {
    /// stdout.
    pub out: bool,
    /// stderr.
    pub err: bool,
}

impl Colors {
    /// Colour for each stream that is a terminal, unless `NO_COLOR` is set.
    pub fn detect(env: &Env, out_is_tty: bool, err_is_tty: bool) -> Colors {
        let allowed = !env.no_color();
        Colors {
            out: allowed && out_is_tty,
            err: allowed && err_is_tty,
        }
    }
}

const ERROR: Style = Style::new()
    .bold()
    .fg_color(Some(anstyle::Color::Ansi(AnsiColor::Red)));
const HINT: Style = Style::new().bold();

/// Writes `error: <message>` and the hint, if any.
pub fn write_error(err: &mut dyn Write, e: &CliError, color: bool) -> io::Result<()> {
    let paint = |s: Style, text: &str| {
        if color {
            format!("{}{text}{}", s.render(), s.render_reset())
        } else {
            text.to_owned()
        }
    };
    writeln!(err, "{} {}", paint(ERROR, "error:"), e.message)?;
    if let Some(hint) = &e.hint {
        writeln!(err, "{} {hint}", paint(HINT, "hint:"))?;
    }
    Ok(())
}

#[derive(Serialize)]
struct JsonError<'a> {
    schema_version: u32,
    error: JsonErrorBody<'a>,
}

#[derive(Serialize)]
struct JsonErrorBody<'a> {
    code: &'a str,
    message: &'a str,
}

/// Writes the `--json` error object, schema version 1, with a newline.
pub fn write_json_error(out: &mut dyn Write, e: &CliError) -> io::Result<()> {
    let value = JsonError {
        schema_version: 1,
        error: JsonErrorBody {
            code: e.kind,
            message: &e.message,
        },
    };
    serde_json::to_writer_pretty(&mut *out, &value).map_err(io::Error::other)?;
    writeln!(out)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::exit;

    #[test]
    fn colour_needs_a_terminal_and_no_no_color() {
        let env = Env::default();
        assert_eq!(
            Colors::detect(&env, true, false),
            Colors {
                out: true,
                err: false
            }
        );
        let off = env.with("NO_COLOR", "1");
        assert_eq!(Colors::detect(&off, true, true), Colors::default());
    }

    #[test]
    fn plain_error_and_hint() {
        let e = CliError::new(exit::USAGE, "bad thing").with_hint("do the other thing");
        let mut buf = Vec::new();
        write_error(&mut buf, &e, false).unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "error: bad thing\nhint: do the other thing\n"
        );
    }

    #[test]
    fn coloured_error_has_ansi() {
        let mut buf = Vec::new();
        write_error(&mut buf, &CliError::new(exit::FAILURE, "x"), true).unwrap();
        let text = String::from_utf8(buf).unwrap();
        assert!(text.contains("\x1b["), "{text:?}");
        assert!(text.ends_with("error:\x1b[0m x\n"), "{text:?}");
    }

    #[test]
    fn json_error_has_the_schema_version_first() {
        let mut buf = Vec::new();
        write_json_error(&mut buf, &CliError::not_implemented("scan")).unwrap();
        let text = String::from_utf8(buf).unwrap();
        assert!(
            text.starts_with("{\n  \"schema_version\": 1,\n  \"error\": {"),
            "{text}"
        );
        assert!(text.ends_with("}\n"));
        let v: serde_json::Value = serde_json::from_str(&text).unwrap();
        assert_eq!(v["error"]["code"], "not_implemented");
    }
}
