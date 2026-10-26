//! The frame's geometry, a pure function of the terminal size.
//!
//! Adapted from mejl-ui's `layout.rs`. update and render both call
//! [`Layout::new`], so a key handled before a frame is drawn sees the same
//! pane heights the frame will have. spela's contract (decision 3):
//!
//! | Size | Layout |
//! |---|---|
//! | 100 columns or more, and 30 rows or more | Two panes. List width `min(32, max(w/3, 24))` |
//! | Under 100 columns or under 30 rows, down to 80x24 | One pane, titled `▸ List` or `▸ Detail` |
//! | Under 80x24 | The resize prompt |
//!
//! Rows: 0 is the top bar, 1 to `h-3` the outer box with the panes, `h-2`
//! the status line and `h-1` the hint bar (terminal-ux.md, frame).

use ratatui::layout::Rect;

/// The smallest usable width.
pub const MIN_WIDTH: u16 = 80;
/// The smallest usable height.
pub const MIN_HEIGHT: u16 = 24;
/// Two panes from this width...
pub const TWO_PANE_WIDTH: u16 = 100;
/// ...and this height.
pub const TWO_PANE_HEIGHT: u16 = 30;

/// A pane of a destination.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum Pane {
    /// The left pane: games, groups or items.
    List,
    /// The right pane: the profile page, an option, a sparkline.
    Detail,
}

impl Pane {
    /// The other pane.
    pub fn other(self) -> Pane {
        match self {
            Pane::List => Pane::Detail,
            Pane::Detail => Pane::List,
        }
    }

    /// The lower-case word Actions titles use: `list` or `detail`.
    pub fn word(self) -> &'static str {
        match self {
            Pane::List => "list",
            Pane::Detail => "detail",
        }
    }
}

/// A pane's rectangles.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct PaneArea {
    /// Which pane.
    pub pane: Pane,
    /// Inside the borders, where content goes.
    pub inner: Rect,
    /// The column of the right border, where the scroll thumb goes.
    pub border_x: u16,
    /// The top border row, where the title goes, and its first column.
    pub title_at: (u16, u16),
}

/// How the frame is split.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Mode {
    /// Smaller than 80x24: only the resize prompt.
    TooSmall,
    /// One pane at a time; Tab shows the other.
    Single,
    /// List and Detail side by side.
    Two,
}

/// Where everything goes for one terminal size.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Layout {
    /// The whole terminal.
    pub area: Rect,
    /// How it is split.
    pub mode: Mode,
    /// Row 0.
    pub top: Rect,
    /// The outer box, borders included.
    pub frame: Rect,
    /// The List pane, when shown in two-pane mode.
    pub list: PaneArea,
    /// The Detail pane in two-pane mode, or the one pane in single mode.
    pub detail: PaneArea,
    /// The status line.
    pub status: Rect,
    /// The hint bar.
    pub hints: Rect,
}

/// The List width at a terminal width: `min(32, max(w/3, 24))`.
pub fn list_width(width: u16) -> u16 {
    (width / 3).clamp(24, 32)
}

impl Layout {
    /// The layout for a terminal of `width` by `height` cells.
    pub fn new(width: u16, height: u16) -> Layout {
        let area = Rect::new(0, 0, width, height);
        let mode = if width < MIN_WIDTH || height < MIN_HEIGHT {
            Mode::TooSmall
        } else if width < TWO_PANE_WIDTH || height < TWO_PANE_HEIGHT {
            Mode::Single
        } else {
            Mode::Two
        };
        let h = height.max(4);
        let top = Rect::new(0, 0, width, 1);
        let frame = Rect::new(0, 1, width, h - 3);
        let status = Rect::new(0, h - 2, width, 1);
        let hints = Rect::new(0, h - 1, width, 1);
        let inner_h = frame.height.saturating_sub(2);
        let (list, detail) = match mode {
            Mode::Two => {
                let lw = list_width(width);
                (
                    PaneArea {
                        pane: Pane::List,
                        inner: Rect::new(1, 2, lw, inner_h),
                        border_x: lw + 1,
                        title_at: (1, 2),
                    },
                    PaneArea {
                        pane: Pane::Detail,
                        inner: Rect::new(lw + 2, 2, width.saturating_sub(lw + 3), inner_h),
                        border_x: width.saturating_sub(1),
                        title_at: (1, lw + 3),
                    },
                )
            }
            Mode::Single | Mode::TooSmall => {
                let one = |pane| PaneArea {
                    pane,
                    inner: Rect::new(1, 2, width.saturating_sub(2), inner_h),
                    border_x: width.saturating_sub(1),
                    title_at: (1, 2),
                };
                (one(Pane::List), one(Pane::Detail))
            }
        };
        Layout {
            area,
            mode,
            top,
            frame,
            list,
            detail,
            status,
            hints,
        }
    }

    /// The area a pane draws in. In single-pane mode both panes share it.
    pub fn pane(&self, pane: Pane) -> PaneArea {
        match pane {
            Pane::List => self.list,
            Pane::Detail => self.detail,
        }
    }

    /// Whether both panes show.
    pub fn two_panes(&self) -> bool {
        self.mode == Mode::Two
    }

    /// Where popups may go: inside the outer box, with a one-cell margin.
    pub fn workspace(&self) -> Rect {
        let f = self.frame;
        Rect::new(
            f.x + 2,
            f.y + 1,
            f.width.saturating_sub(4),
            f.height.saturating_sub(2),
        )
    }

    /// A popup of at most `width` by `height`, clamped to the workspace and
    /// centred in it.
    pub fn centered(&self, width: u16, height: u16) -> Rect {
        let ws = self.workspace();
        let w = width.min(ws.width);
        let h = height.min(ws.height);
        Rect::new(ws.x + (ws.width - w) / 2, ws.y + (ws.height - h) / 2, w, h)
    }

    /// A popup anchored to the right of the workspace, as Actions is, so
    /// the list it acts on stays visible.
    pub fn right_anchored(&self, width: u16, height: u16) -> Rect {
        let ws = self.workspace();
        let w = width.min(ws.width);
        let h = height.min(ws.height);
        Rect::new(ws.x + ws.width - w, ws.y + 1.min(ws.height - h), w, h)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_breakpoints_follow_the_contract() {
        assert_eq!(Layout::new(120, 40).mode, Mode::Two);
        assert_eq!(Layout::new(99, 40).mode, Mode::Single);
        assert_eq!(Layout::new(120, 29).mode, Mode::Single);
        assert_eq!(Layout::new(80, 24).mode, Mode::Single);
        assert_eq!(Layout::new(79, 24).mode, Mode::TooSmall);
        assert_eq!(Layout::new(80, 23).mode, Mode::TooSmall);
    }

    #[test]
    fn panes_match_the_mockup_grid() {
        // mockups.html, screen 1: List inner columns 1 to 32, the shared
        // border at 33, Detail inner from 34 to 118, rows 2 to 36.
        let l = Layout::new(120, 40);
        assert_eq!(l.list.inner, Rect::new(1, 2, 32, 35));
        assert_eq!(l.list.border_x, 33);
        assert_eq!(l.detail.inner, Rect::new(34, 2, 85, 35));
        assert_eq!(l.status, Rect::new(0, 38, 120, 1));
        assert_eq!(l.hints, Rect::new(0, 39, 120, 1));
        let narrow = Layout::new(80, 24);
        assert_eq!(narrow.detail.inner, Rect::new(1, 2, 78, 19));
        assert_eq!(list_width(100), 32);
        assert_eq!(list_width(80), 26);
    }

    #[test]
    fn popups_keep_a_margin() {
        let l = Layout::new(120, 40);
        let ws = l.workspace();
        let p = l.right_anchored(80, 20);
        assert!(p.x >= ws.x && p.right() <= ws.right());
        assert_eq!(p.right(), 118);
        let c = l.centered(200, 100);
        assert_eq!(c, ws);
    }
}
