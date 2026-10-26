//! Colour roles for command output.
//!
//! The CLI shares the TUI's roles (`docs/terminal-ux.md`, CLI output):
//! primary for names, secondary for headings and DLL names, accent for
//! values that matter, dim for labels and sources, success for a yes. The
//! roles follow Go's `tui.CLI*` helpers. Colour goes only to a terminal and
//! `NO_COLOR` turns it off, which [`crate::output::Colors`] decides; with it
//! off every role is the plain text.

use anstyle::{AnsiColor, Color, Style};

const PRIMARY: Style = Style::new().bold();
const SECONDARY: Style = Style::new().fg_color(Some(Color::Ansi(AnsiColor::Blue)));
const ACCENT: Style = Style::new().fg_color(Some(Color::Ansi(AnsiColor::Cyan)));
const DIM: Style = Style::new().dimmed();
const SUCCESS: Style = Style::new().fg_color(Some(Color::Ansi(AnsiColor::Green)));

/// Paints text in a role, or leaves it plain when colour is off.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Paint {
    color: bool,
}

impl Paint {
    /// A painter for a stream that gets colour when `color` is true.
    pub fn new(color: bool) -> Paint {
        Paint { color }
    }

    fn style(self, style: Style, text: &str) -> String {
        if self.color {
            format!("{}{text}{}", style.render(), style.render_reset())
        } else {
            text.to_owned()
        }
    }

    /// Names: games, sections.
    pub fn primary(self, text: &str) -> String {
        self.style(PRIMARY, text)
    }

    /// Headings and DLL names.
    pub fn secondary(self, text: &str) -> String {
        self.style(SECONDARY, text)
    }

    /// Values that matter: counts, versions, a game's own setting.
    pub fn accent(self, text: &str) -> String {
        self.style(ACCENT, text)
    }

    /// Labels, ids, paths and sources.
    pub fn dim(self, text: &str) -> String {
        self.style(DIM, text)
    }

    /// A yes.
    pub fn success(self, text: &str) -> String {
        self.style(SUCCESS, text)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn plain_without_colour_and_ansi_with_it() {
        let plain = Paint::new(false);
        for f in [
            Paint::primary,
            Paint::secondary,
            Paint::accent,
            Paint::dim,
            Paint::success,
        ] {
            assert_eq!(f(plain, "x"), "x");
            let painted = f(Paint::new(true), "x");
            assert!(painted.starts_with("\x1b["), "{painted:?}");
            assert!(painted.ends_with("x\x1b[0m"), "{painted:?}");
        }
    }
}
