//! Themes and glyphs.
//!
//! Adapted from mejl-ui's `theme.rs`. Views never pick colours: they ask
//! for a [`Role`], and the theme decides how it looks, so a theme cannot
//! drop a state distinction by accident. State never depends on colour
//! alone: every state also has a word or a glyph (terminal-ux.md, theme and
//! glyphs).
//!
//! - `terminal` (the default): the 16 ANSI colours, so the user's terminal
//!   theme carries over. The cursor row is reverse video.
//! - `neon`: spela's own truecolor palette.
//! - `mono`: bold, dim and reverse only. `NO_COLOR` forces it.
//!
//! No theme paints the terminal background.
//!
//! [`Glyphs`] are the Unicode Neutral width set, or plain ASCII when
//! `ui.ascii_glyphs` is on.

use ratatui::style::{Color, Modifier, Style};
use ratatui::symbols::border;
use spela_core::config::Theme as ThemeName;

/// What a piece of text means. A theme maps each one to a style.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Role {
    /// Plain text.
    Text,
    /// Bold text: a focused row's name, a value.
    Strong,
    /// Following values, hints, descriptions.
    Muted,
    /// Pane borders.
    Border,
    /// The focused pane title, the cursor, keys.
    Focus,
    /// A key in the hint bar or a popup.
    Key,
    /// An own value, the ⚑ marker.
    Own,
    /// Section titles and the game line: own colour, bold.
    Title,
    /// Saved, current, success.
    Ok,
    /// A warning or "Saving...".
    Warn,
    /// An error, "Not saved", a critical alert.
    Error,
    /// The brand tag in the top bar.
    Brand,
    /// Matched letters in the palette.
    Match,
    /// A step on the thermal scale, 0 (cool) to 5 (hot).
    Thermal(u8),
}

/// A built theme: its name and whether colour is on.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Theme {
    /// The theme in effect, after `NO_COLOR`.
    pub name: ThemeName,
}

const NEON_TEXT: Color = Color::Rgb(0xe8, 0xe8, 0xf0);
const NEON_MUTED: Color = Color::Rgb(0x6a, 0x6a, 0x80);
const NEON_BORDER: Color = Color::Rgb(0x3a, 0x3a, 0x58);
const NEON_FOCUS: Color = Color::Rgb(0x5f, 0xf0, 0xff);
const NEON_OWN: Color = Color::Rgb(0xff, 0x5f, 0xd2);
const NEON_OK: Color = Color::Rgb(0x7c, 0xe3, 0x8b);
const NEON_WARN: Color = Color::Rgb(0xff, 0xd8, 0x66);
const NEON_ERROR: Color = Color::Rgb(0xff, 0x6e, 0x6e);
const NEON_CURSOR: Color = Color::Rgb(0x1c, 0x1c, 0x30);
/// The six stops of Go's `thermal.go`.
const NEON_THERMAL: [Color; 6] = [
    Color::Rgb(0x5f, 0xf0, 0xff),
    Color::Rgb(0x5f, 0xba, 0xff),
    Color::Rgb(0x7c, 0xe3, 0x8b),
    Color::Rgb(0xff, 0xd8, 0x66),
    Color::Rgb(0xff, 0x9e, 0x5f),
    Color::Rgb(0xff, 0x6e, 0x6e),
];
const ANSI_THERMAL: [Color; 6] = [
    Color::Cyan,
    Color::Blue,
    Color::Green,
    Color::Yellow,
    Color::Red,
    Color::Red,
];

impl Theme {
    /// The theme for a config value. `NO_COLOR` forces mono.
    pub fn new(name: ThemeName, no_color: bool) -> Theme {
        Theme {
            name: if no_color { ThemeName::Mono } else { name },
        }
    }

    /// The style for a role.
    pub fn style(&self, role: Role) -> Style {
        let bold = Style::new().add_modifier(Modifier::BOLD);
        match self.name {
            ThemeName::Mono => match role {
                Role::Text | Role::Own | Role::Ok | Role::Thermal(_) => Style::new(),
                Role::Strong | Role::Focus | Role::Key | Role::Title | Role::Error | Role::Warn => {
                    bold
                }
                Role::Muted | Role::Border => Style::new().add_modifier(Modifier::DIM),
                Role::Brand => bold.add_modifier(Modifier::REVERSED),
                Role::Match => Style::new().add_modifier(Modifier::UNDERLINED),
            },
            ThemeName::Terminal => match role {
                Role::Text => Style::new(),
                Role::Strong => bold,
                Role::Muted | Role::Border => Style::new().fg(Color::DarkGray),
                Role::Focus | Role::Key => bold.fg(Color::Blue),
                Role::Own => Style::new().fg(Color::Magenta),
                Role::Title => bold.fg(Color::Magenta),
                Role::Ok => Style::new().fg(Color::Green),
                Role::Warn => Style::new().fg(Color::Yellow),
                Role::Error => Style::new().fg(Color::Red),
                Role::Brand => bold.add_modifier(Modifier::REVERSED),
                Role::Match => bold.fg(Color::Yellow),
                Role::Thermal(n) => Style::new().fg(ANSI_THERMAL[usize::from(n.min(5))]),
            },
            ThemeName::Neon => match role {
                Role::Text => Style::new().fg(NEON_TEXT),
                Role::Strong => bold.fg(NEON_TEXT),
                Role::Muted => Style::new().fg(NEON_MUTED),
                Role::Border => Style::new().fg(NEON_BORDER),
                Role::Focus | Role::Key => bold.fg(NEON_FOCUS),
                Role::Own => Style::new().fg(NEON_OWN),
                Role::Title => bold.fg(NEON_OWN),
                Role::Ok => Style::new().fg(NEON_OK),
                Role::Warn => Style::new().fg(NEON_WARN),
                Role::Error => Style::new().fg(NEON_ERROR),
                Role::Brand => bold.fg(Color::Black).bg(NEON_FOCUS),
                Role::Match => bold.fg(NEON_WARN),
                Role::Thermal(n) => Style::new().fg(NEON_THERMAL[usize::from(n.min(5))]),
            },
        }
    }

    /// The style patched over the cursor row: reverse video in `terminal`
    /// and `mono`, a dark background in `neon`.
    pub fn cursor(&self) -> Style {
        match self.name {
            ThemeName::Neon => Style::new().bg(NEON_CURSOR),
            _ => Style::new().add_modifier(Modifier::REVERSED),
        }
    }

    /// The style a popup's backdrop gets: everything behind it dimmed.
    pub fn backdrop(&self) -> Style {
        match self.name {
            ThemeName::Neon => Style::new().fg(NEON_MUTED),
            ThemeName::Terminal => Style::new().fg(Color::DarkGray),
            ThemeName::Mono => Style::new().add_modifier(Modifier::DIM),
        }
    }
}

/// The thermal step, 0 to 5, for a ratio from 0 to 1 (Go's stops at 0,
/// .3, .5, .7, .85 and 1).
pub fn thermal_step(ratio: f64) -> u8 {
    const STOPS: [f64; 6] = [0.0, 0.3, 0.5, 0.7, 0.85, 1.0];
    STOPS.iter().rposition(|s| ratio >= *s).unwrap_or(0) as u8
}

/// The temperature's ratio on the thermal scale: 30 °C to 95 °C.
pub fn temperature_ratio(celsius: u32) -> f64 {
    ((f64::from(celsius) - 30.0) / 65.0).clamp(0.0, 1.0)
}

/// Indicator glyphs. The Unicode set uses only the Neutral East Asian
/// width class, so each stays one cell in every locale.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Glyphs {
    /// The current destination, the focused pane, the menu cursor.
    pub cursor: &'static str,
    /// An own value, a game with its own profile.
    pub own: &'static str,
    /// A game with managed DLLs.
    pub dlls: &'static str,
    /// Selected, saved, current.
    pub ok: &'static str,
    /// Failed, critical, unavailable.
    pub error: &'static str,
    /// A warning.
    pub warn: &'static str,
    /// Truncated text.
    pub ellipsis: &'static str,
    /// The scroll thumb on a pane border.
    pub thumb: &'static str,
    /// A gauge's filled part.
    pub gauge_full: &'static str,
    /// A gauge's empty part.
    pub gauge_empty: &'static str,
    /// The separator in "this game · All games".
    pub dot: &'static str,
    /// The degree sign.
    pub degree: &'static str,
    /// Sparkline steps, low to high.
    pub spark: [&'static str; 8],
    /// Box drawing.
    pub border: border::Set<'static>,
    /// The joins where a line meets a box: left, right, top and bottom.
    pub joins: [&'static str; 4],
}

impl Glyphs {
    /// The Unicode set.
    pub const UNICODE: Glyphs = Glyphs {
        cursor: "▸",
        own: "⚑",
        dlls: "▪",
        ok: "✓",
        error: "✗",
        warn: "⚠",
        ellipsis: "⋯",
        thumb: "┃",
        gauge_full: "█",
        gauge_empty: "░",
        dot: "·",
        degree: "°",
        spark: ["▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"],
        border: border::PLAIN,
        joins: ["├", "┤", "┬", "┴"],
    };

    /// The plain ASCII set, for `ui.ascii_glyphs`.
    pub const ASCII: Glyphs = Glyphs {
        cursor: ">",
        own: "*",
        dlls: "+",
        ok: "x",
        error: "!",
        warn: "!",
        ellipsis: "...",
        thumb: "#",
        gauge_full: "#",
        gauge_empty: "-",
        dot: "-",
        degree: "C",
        spark: ["_", "_", ".", "-", "-", "=", "^", "^"],
        border: border::Set {
            top_left: "+",
            top_right: "+",
            bottom_left: "+",
            bottom_right: "+",
            vertical_left: "|",
            vertical_right: "|",
            horizontal_top: "-",
            horizontal_bottom: "-",
        },
        joins: ["+", "+", "+", "+"],
    };

    /// The set for `ui.ascii_glyphs`.
    pub fn new(ascii: bool) -> Glyphs {
        if ascii {
            Glyphs::ASCII
        } else {
            Glyphs::UNICODE
        }
    }

    /// The sparkline glyph for a ratio from 0 to 1.
    pub fn spark_for(&self, ratio: f64) -> &'static str {
        let i = (ratio.clamp(0.0, 1.0) * 7.0).round() as usize;
        self.spark[i.min(7)]
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::text::width;

    #[test]
    fn no_color_forces_mono() {
        assert_eq!(Theme::new(ThemeName::Neon, true).name, ThemeName::Mono);
        assert_eq!(Theme::new(ThemeName::Neon, false).name, ThemeName::Neon);
        let mono = Theme::new(ThemeName::Terminal, true);
        for role in [
            Role::Text,
            Role::Own,
            Role::Ok,
            Role::Error,
            Role::Thermal(5),
        ] {
            let s = mono.style(role);
            assert!(s.fg.is_none() && s.bg.is_none(), "{role:?}");
        }
    }

    #[test]
    fn every_glyph_is_one_cell() {
        for g in [Glyphs::UNICODE, Glyphs::ASCII] {
            for s in [
                g.cursor, g.own, g.dlls, g.ok, g.error, g.warn, g.thumb, g.dot, g.degree,
            ] {
                assert_eq!(width(s), 1, "{s:?}");
            }
            for s in g.spark {
                assert_eq!(width(s), 1, "{s:?}");
            }
            assert!(g.ellipsis.is_ascii() || width(g.ellipsis) == 1);
        }
    }

    #[test]
    fn thermal_steps_follow_go() {
        assert_eq!(thermal_step(0.0), 0);
        assert_eq!(thermal_step(0.31), 1);
        assert_eq!(thermal_step(0.69), 2);
        assert_eq!(thermal_step(0.99), 4);
        assert_eq!(thermal_step(1.0), 5);
        assert_eq!(thermal_step(temperature_ratio(54)), 1);
        assert_eq!(thermal_step(temperature_ratio(71)), 2);
    }
}
