//! Mouse hit testing.
//!
//! Adopted from mejl-ui's `hit.rs`, with spela's targets. ratatui is
//! immediate mode and has no widget tree, so every clickable element
//! records its rectangle and a [`HitTarget`] while it is drawn. The frame's
//! [`HitMap`] is kept until the next frame. A mouse event is tested against
//! it from the top-most (last drawn) region down, and the target becomes
//! the same action its key gives (terminal-ux.md, mouse).

use ratatui::layout::{Position, Rect};

use crate::layout::Pane;
use crate::screens::Action;

/// What a region of the screen does when clicked.
#[derive(Clone, Debug, PartialEq)]
pub enum HitTarget {
    /// A pane's area, for the wheel and for right click.
    Pane(Pane),
    /// A list row. A click runs `select`, a double click runs `select`
    /// and then `open`, like Enter.
    Row {
        /// Moves the cursor to the row.
        select: Action,
        /// What Enter does on the row.
        open: Action,
    },
    /// Runs an action: destinations, hints, buttons, section tabs.
    Action(Action),
    /// Inside a popup but not on an entry: swallows the click.
    Popup,
    /// Everything outside a popup: a click closes it.
    Backdrop,
}

/// The regions recorded while drawing one frame.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct HitMap {
    regions: Vec<(Rect, HitTarget)>,
}

impl HitMap {
    /// An empty map.
    pub fn new() -> HitMap {
        HitMap::default()
    }

    /// Records a region. Later regions are on top of earlier ones.
    pub fn push(&mut self, area: Rect, target: HitTarget) {
        if area.width > 0 && area.height > 0 {
            self.regions.push((area, target));
        }
    }

    /// Forgets every region, at the start of a frame.
    pub fn clear(&mut self) {
        self.regions.clear();
    }

    /// The top-most target at a cell.
    pub fn hit(&self, x: u16, y: u16) -> Option<&HitTarget> {
        let p = Position::new(x, y);
        self.regions
            .iter()
            .rev()
            .find(|(r, _)| r.contains(p))
            .map(|(_, t)| t)
    }

    /// The pane under a cell, whatever is drawn on top of it.
    pub fn pane_at(&self, x: u16, y: u16) -> Option<Pane> {
        let p = Position::new(x, y);
        self.regions.iter().rev().find_map(|(r, t)| match t {
            HitTarget::Pane(pane) if r.contains(p) => Some(*pane),
            _ => None,
        })
    }

    /// The rectangle of the first region with this target.
    pub fn find(&self, target: &HitTarget) -> Option<Rect> {
        self.regions
            .iter()
            .find(|(_, t)| t == target)
            .map(|(r, _)| *r)
    }

    /// All recorded regions, bottom first.
    pub fn regions(&self) -> &[(Rect, HitTarget)] {
        &self.regions
    }

    /// Number of regions.
    pub fn len(&self) -> usize {
        self.regions.len()
    }

    /// True when nothing was recorded.
    pub fn is_empty(&self) -> bool {
        self.regions.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn top_most_region_wins() {
        let mut map = HitMap::new();
        map.push(Rect::new(0, 0, 10, 10), HitTarget::Pane(Pane::List));
        map.push(Rect::new(0, 2, 10, 1), HitTarget::Action(Action::Help));
        map.push(Rect::new(0, 0, 0, 5), HitTarget::Popup);
        assert_eq!(map.len(), 2);
        assert_eq!(map.hit(3, 2), Some(&HitTarget::Action(Action::Help)));
        assert_eq!(map.hit(3, 3), Some(&HitTarget::Pane(Pane::List)));
        assert_eq!(map.pane_at(3, 2), Some(Pane::List));
        assert_eq!(map.hit(10, 3), None);
        assert_eq!(
            map.find(&HitTarget::Action(Action::Help)),
            Some(Rect::new(0, 2, 10, 1))
        );
    }
}
