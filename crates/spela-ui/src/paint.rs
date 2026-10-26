//! Drawing helpers that every screen uses, on top of ratatui's buffer.
//!
//! A [`Painter`] writes text by display width, cut with the theme's
//! ellipsis so nothing ever spills past a pane edge, and draws the shared
//! chrome: boxes with titles in the border, the cursor row, the scroll
//! thumb and popups over a dimmed backdrop. Every string it draws goes
//! through [`crate::text::clean`] first, because game names and paths come
//! from the disk.

use ratatui::buffer::Buffer;
use ratatui::layout::Rect;
use ratatui::style::Style;

use crate::hit::{HitMap, HitTarget};
use crate::text::{clean, sanitize, truncate, width};
use crate::theme::{Glyphs, Role, Theme};

/// A piece of text in a role.
pub type Span<'a> = (&'a str, Role);

/// Draws on a buffer with a theme and a glyph set.
pub struct Painter<'a> {
    /// The frame's buffer.
    pub buf: &'a mut Buffer,
    /// The theme.
    pub theme: Theme,
    /// The glyphs.
    pub glyphs: Glyphs,
}

impl<'a> Painter<'a> {
    /// A painter over `buf`.
    pub fn new(buf: &'a mut Buffer, theme: Theme, glyphs: Glyphs) -> Painter<'a> {
        Painter { buf, theme, glyphs }
    }

    /// The style for a role.
    pub fn style(&self, role: Role) -> Style {
        self.theme.style(role)
    }

    /// Writes `text` at `(x, y)`, at most `max` cells, cut with the
    /// ellipsis. Returns the column after it.
    pub fn text(&mut self, x: u16, y: u16, max: u16, text: &str, role: Role) -> u16 {
        let style = self.style(role);
        self.styled(x, y, max, text, style)
    }

    /// [`Painter::text`] with an explicit style.
    pub fn styled(&mut self, x: u16, y: u16, max: u16, text: &str, style: Style) -> u16 {
        if max == 0 || y >= self.buf.area.bottom() {
            return x;
        }
        let safe = clean(text);
        let (cut, w) = truncate(&safe, usize::from(max), self.glyphs.ellipsis);
        self.buf.set_stringn(x, y, &cut, usize::from(max), style);
        x + u16::try_from(w).unwrap_or(max)
    }

    /// Writes spans one after another from `(x, y)`, all within `max`
    /// cells. Returns the column after the last.
    pub fn spans(&mut self, x: u16, y: u16, max: u16, spans: &[Span<'_>]) -> u16 {
        let end = x.saturating_add(max);
        let mut at = x;
        for (text, role) in spans {
            if at >= end {
                break;
            }
            at = self.text(at, y, end - at, text, *role);
        }
        at
    }

    /// Writes spans so they end at `right` (exclusive), as the top bar's
    /// metrics and the status line's work state do. Returns where they
    /// start.
    pub fn spans_right(&mut self, right: u16, y: u16, spans: &[Span<'_>]) -> u16 {
        let total: usize = spans.iter().map(|(t, _)| width(&clean(t))).sum();
        let start = right.saturating_sub(u16::try_from(total).unwrap_or(u16::MAX));
        self.spans(start, y, right - start, spans);
        start
    }

    /// Patches the cursor style over a row.
    pub fn cursor_row(&mut self, area: Rect) {
        let style = self.theme.cursor();
        self.buf.set_style(area, style);
    }

    /// Clears an area to blanks.
    pub fn clear(&mut self, area: Rect) {
        let area = area.intersection(self.buf.area);
        for y in area.top()..area.bottom() {
            for x in area.left()..area.right() {
                if let Some(cell) = self.buf.cell_mut((x, y)) {
                    cell.reset();
                }
            }
        }
    }

    /// Draws a box border with a title in its top border at `x + 2`, as
    /// ` Title `.
    pub fn frame(&mut self, area: Rect, title: &str, border: Role, title_role: Role) {
        if area.width < 2 || area.height < 2 {
            return;
        }
        let b = self.glyphs.border;
        let style = self.style(border);
        let (l, r, t, bot) = (area.left(), area.right() - 1, area.top(), area.bottom() - 1);
        for x in l + 1..r {
            self.buf.set_string(x, t, b.horizontal_top, style);
            self.buf.set_string(x, bot, b.horizontal_bottom, style);
        }
        for y in t + 1..bot {
            self.buf.set_string(l, y, b.vertical_left, style);
            self.buf.set_string(r, y, b.vertical_right, style);
        }
        self.buf.set_string(l, t, b.top_left, style);
        self.buf.set_string(r, t, b.top_right, style);
        self.buf.set_string(l, bot, b.bottom_left, style);
        self.buf.set_string(r, bot, b.bottom_right, style);
        if !title.is_empty() {
            self.title(l + 2, t, area.width.saturating_sub(4), title, title_role);
        }
    }

    /// Writes ` text ` into a border row.
    pub fn title(&mut self, x: u16, y: u16, max: u16, text: &str, role: Role) {
        if max < 3 {
            return;
        }
        let end = self.text(x + 1, y, max - 2, text, role);
        let style = self.style(role);
        self.buf.set_string(x, y, " ", style);
        self.buf.set_string(end, y, " ", style);
    }

    /// A horizontal rule across a box at row `y`, joined to its sides.
    pub fn rule(&mut self, area: Rect, y: u16, border: Role) {
        let style = self.style(border);
        let b = self.glyphs.border;
        for x in area.left() + 1..area.right() - 1 {
            self.buf.set_string(x, y, b.horizontal_top, style);
        }
        self.buf
            .set_string(area.left(), y, self.glyphs.joins[0], style);
        self.buf
            .set_string(area.right() - 1, y, self.glyphs.joins[1], style);
    }

    /// The scroll thumb on a border column, for `visible` of `total` rows
    /// starting at `offset`, over the rows of `track`. Nothing when it all
    /// fits. Size and place round as the mockups do, and at the end of the
    /// content the thumb reaches the last track row.
    pub fn thumb(&mut self, x: u16, track: Rect, offset: usize, visible: usize, total: usize) {
        let Some((from, size)) = thumb_span(usize::from(track.height), offset, visible, total)
        else {
            return;
        };
        let style = self.style(Role::Muted);
        for i in from..from + size {
            let y = track.y + u16::try_from(i).unwrap_or(0);
            self.buf.set_string(x, y, self.glyphs.thumb, style);
        }
    }

    /// Dims everything below the top bar, as behind a popup.
    pub fn dim_backdrop(&mut self, area: Rect) {
        let style = self.theme.backdrop();
        for y in area.top().max(1)..area.bottom() {
            for x in area.left()..area.right() {
                if let Some(cell) = self.buf.cell_mut((x, y)) {
                    cell.set_style(Style::reset().patch(style));
                }
            }
        }
    }

    /// A popup: clears its area, draws a focus-coloured box with the title,
    /// and records the backdrop and the popup as click targets. Returns the
    /// area inside the borders.
    pub fn popup(&mut self, area: Rect, title: &str, hits: &mut HitMap) -> Rect {
        hits.push(self.buf.area, HitTarget::Backdrop);
        hits.push(area, HitTarget::Popup);
        self.clear(area);
        self.frame(area, title, Role::Focus, Role::Key);
        Rect::new(
            area.x + 1,
            area.y + 1,
            area.width.saturating_sub(2),
            area.height.saturating_sub(2),
        )
    }

    /// A gauge of `width` cells for `used` of `total`.
    pub fn gauge(&mut self, x: u16, y: u16, width: u16, used: f64, total: f64) -> u16 {
        let ratio = if total > 0.0 {
            (used / total).clamp(0.0, 1.0)
        } else {
            0.0
        };
        let full = (ratio * f64::from(width)).round() as u16;
        let fs = self.style(Role::Focus);
        let ms = self.style(Role::Muted);
        for i in 0..width {
            let (g, s) = if i < full {
                (self.glyphs.gauge_full, fs)
            } else {
                (self.glyphs.gauge_empty, ms)
            };
            self.buf.set_string(x + i, y, g, s);
        }
        x + width
    }
}

/// The first row and the length of a scroll thumb on a track of `h` rows,
/// or `None` when everything fits.
pub fn thumb_span(h: usize, offset: usize, visible: usize, total: usize) -> Option<(usize, usize)> {
    if total <= visible || h == 0 {
        return None;
    }
    let round = |n: usize| (n + total / 2) / total;
    let size = round(visible * h).clamp(1, h);
    let from = if offset + visible >= total {
        h - size
    } else {
        round(offset * h).min(h - size)
    };
    Some((from, size))
}

/// Pads or cuts `text` to exactly `cols` cells with the ellipsis.
pub fn fit(text: &str, cols: usize, glyphs: &Glyphs) -> String {
    crate::text::fit(&sanitize(text), cols, glyphs.ellipsis)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_thumb_reaches_the_last_row_at_the_end() {
        // 16 rows of 37 on a 16-row track: the floor would stop one short.
        let (from, size) = thumb_span(16, 21, 16, 37).unwrap();
        assert_eq!(from + size, 16);
        assert_eq!(thumb_span(16, 0, 16, 37), Some((0, 7)));
        assert_eq!(thumb_span(16, 0, 16, 16), None, "it all fits");
        for total in 17..80 {
            for offset in 0..=total - 16 {
                let (from, size) = thumb_span(16, offset, 16, total).unwrap();
                assert!(from + size <= 16 && size >= 1);
                if offset + 16 == total {
                    assert_eq!(from + size, 16, "total {total}");
                }
                if offset == 0 {
                    assert_eq!(from, 0);
                }
            }
        }
    }
}
