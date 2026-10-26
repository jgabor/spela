//! A choice picker, shared by the profile editors, Settings and the
//! Actions popup's governor picker.
//!
//! Adapted from mejl-ui's `picker.rs` for spela's in-pane form (mockups,
//! screen 2): a box titled `Edit <Label>  jump: <typed>`, a `Value before
//! edit` line, the choices with the stored one marked ✓, and `[ Apply ]
//! [ Cancel ]`. Type-to-jump follows Go (TUI 8): a typed letter jumps to
//! the next choice that starts with what was typed, the same letter again
//! moves to the next match, and a letter that matches nothing starts the
//! jump over from that letter. Up and Down wrap, Tab moves between the
//! choices and the buttons, Enter applies or cancels by focus, Esc
//! cancels.
//!
//! A screen keeps a [`Picker`] in its own state, returns
//! [`Picker::bindings`] from its `bindings` while it is open (and `true`
//! from `editing`), feeds each [`PickerInput`] to [`Picker::handle`] and
//! draws it with [`Picker::render`].

use ratatui::layout::Rect;

use crate::keymap::{Binding, Key};
use crate::paint::Painter;
use crate::screens::Action;
use crate::theme::Role;

/// What has focus inside the picker.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PickerFocus {
    /// The choices.
    Choices,
    /// `[ Apply ]`.
    Apply,
    /// `[ Cancel ]`.
    Cancel,
}

/// One choice.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Choice {
    /// What it shows, such as `Follow All games (balanced)` or `quality`.
    pub label: String,
    /// It is the stored value, marked ✓.
    pub current: bool,
}

/// A key the picker takes.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PickerInput {
    /// The previous choice, wrapping.
    Up,
    /// The next choice, wrapping.
    Down,
    /// The next control.
    Tab,
    /// The previous control.
    BackTab,
    /// Left or Right on the buttons.
    Switch,
    /// Apply or cancel, by focus.
    Enter,
    /// Cancel.
    Esc,
    /// A typed character, for type-to-jump.
    Type(char),
}

/// What a key did.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PickerOutcome {
    /// Only the picker changed.
    Moved,
    /// Apply this choice, by index.
    Apply(usize),
    /// Close without a change.
    Cancel,
}

/// An open picker.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Picker {
    /// The title, such as `Edit Super resolution`.
    pub title: String,
    /// The `Value before edit` text, when there is one.
    pub before: Option<String>,
    /// The choices.
    pub choices: Vec<Choice>,
    /// The cursor choice.
    pub cursor: usize,
    /// What was typed for the jump.
    pub jump: String,
    /// What has focus.
    pub focus: PickerFocus,
}

impl Picker {
    /// A picker with the cursor on the stored choice, or the first.
    pub fn new(title: impl Into<String>, before: Option<String>, choices: Vec<Choice>) -> Picker {
        let cursor = choices.iter().position(|c| c.current).unwrap_or(0);
        Picker {
            title: title.into(),
            before,
            choices,
            cursor,
            jump: String::new(),
            focus: PickerFocus::Choices,
        }
    }

    /// Choices from labels, with `current` marked.
    pub fn choices<S: AsRef<str>>(labels: &[S], current: Option<&str>) -> Vec<Choice> {
        labels
            .iter()
            .map(|l| Choice {
                label: l.as_ref().to_owned(),
                current: Some(l.as_ref()) == current,
            })
            .collect()
    }

    /// The picker's keys. `map` turns an input into the screen's action;
    /// `typed` does the same for a typed character, because a binding for
    /// typed keys takes a plain function.
    pub fn bindings(
        &self,
        map: fn(PickerInput) -> Action,
        typed: fn(char) -> Action,
    ) -> Vec<Binding> {
        let enter = match self.focus {
            PickerFocus::Cancel => "cancel",
            _ => "apply",
        };
        let mut out = vec![
            Binding::typed(typed, map(PickerInput::Esc), "to jump").hint_as("Type", 100),
            Binding::new(Key::UP, map(PickerInput::Up), "choose").hint_as("Up/Down", 95),
            Binding::new(Key::DOWN, map(PickerInput::Down), "Next choice"),
            Binding::new(Key::TAB, map(PickerInput::Tab), "control").hint(80),
            Binding::new(Key::BACK_TAB, map(PickerInput::BackTab), "Previous control"),
            Binding::new(Key::ENTER, map(PickerInput::Enter), enter).hint(90),
            Binding::new(Key::ESC, map(PickerInput::Esc), "cancel").hint(85),
        ];
        if self.focus != PickerFocus::Choices {
            out.push(Binding::new(
                Key::LEFT,
                map(PickerInput::Switch),
                "Choose button",
            ));
            out.push(Binding::new(
                Key::RIGHT,
                map(PickerInput::Switch),
                "Choose button",
            ));
        }
        out
    }

    /// Takes one input.
    pub fn handle(&mut self, input: PickerInput) -> PickerOutcome {
        let n = self.choices.len();
        match input {
            PickerInput::Up if n > 0 => {
                self.focus = PickerFocus::Choices;
                self.cursor = (self.cursor + n - 1) % n;
                self.jump.clear();
            }
            PickerInput::Down if n > 0 => {
                self.focus = PickerFocus::Choices;
                self.cursor = (self.cursor + 1) % n;
                self.jump.clear();
            }
            PickerInput::Up | PickerInput::Down => {}
            PickerInput::Tab => {
                self.focus = match self.focus {
                    PickerFocus::Choices => PickerFocus::Apply,
                    PickerFocus::Apply => PickerFocus::Cancel,
                    PickerFocus::Cancel => PickerFocus::Choices,
                };
            }
            PickerInput::BackTab => {
                self.focus = match self.focus {
                    PickerFocus::Choices => PickerFocus::Cancel,
                    PickerFocus::Apply => PickerFocus::Choices,
                    PickerFocus::Cancel => PickerFocus::Apply,
                };
            }
            PickerInput::Switch => {
                self.focus = match self.focus {
                    PickerFocus::Apply => PickerFocus::Cancel,
                    _ => PickerFocus::Apply,
                };
            }
            PickerInput::Enter => {
                return match self.focus {
                    PickerFocus::Cancel => PickerOutcome::Cancel,
                    _ if n == 0 => PickerOutcome::Cancel,
                    _ => PickerOutcome::Apply(self.cursor),
                };
            }
            PickerInput::Esc => return PickerOutcome::Cancel,
            PickerInput::Type(c) => self.type_jump(c),
        }
        PickerOutcome::Moved
    }

    fn type_jump(&mut self, c: char) {
        if c == ' ' {
            return;
        }
        self.focus = PickerFocus::Choices;
        let n = self.choices.len();
        let starts = |i: usize, prefix: &str| {
            self.choices[i]
                .label
                .to_lowercase()
                .starts_with(&prefix.to_lowercase())
        };
        // The same letter again moves to the next match of that letter.
        let repeat = self.jump.chars().count() == 1 && self.jump.starts_with(c);
        let candidate = if repeat {
            self.jump.clone()
        } else {
            format!("{}{c}", self.jump)
        };
        let from = if repeat { self.cursor + 1 } else { self.cursor };
        let find = |prefix: &str| (0..n).map(|k| (from + k) % n).find(|i| starts(*i, prefix));
        if let Some(i) = find(&candidate) {
            self.cursor = i;
            self.jump = candidate;
            return;
        }
        // Nothing matches: the jump starts over from this letter.
        let single = c.to_string();
        self.jump = single.clone();
        if let Some(i) = (0..n)
            .map(|k| (self.cursor + 1 + k) % n)
            .find(|i| starts(*i, &single))
        {
            self.cursor = i;
        }
    }

    /// Rows the boxed form takes: the box, the before line and a blank,
    /// the choices, a blank and the buttons.
    pub fn height(&self) -> u16 {
        let before = u16::from(self.before.is_some()) * 2;
        u16::try_from(self.choices.len())
            .unwrap_or(u16::MAX)
            .saturating_add(4 + before)
    }

    /// Draws the boxed form at `area`'s top, as wide as `area`.
    pub fn render(&self, p: &mut Painter<'_>, area: Rect) {
        let h = self.height().min(area.height);
        if h < 3 || area.width < 16 {
            return;
        }
        let r = Rect::new(area.x, area.y, area.width, h);
        let title = if self.jump.is_empty() {
            self.title.clone()
        } else {
            format!("{}  jump: {}", self.title, self.jump)
        };
        p.clear(r);
        p.frame(r, &title, Role::Focus, Role::Key);
        let x = r.x + 2;
        let w = r.width.saturating_sub(4);
        let mut y = r.y + 1;
        if let Some(before) = &self.before {
            p.text(
                x,
                y,
                w,
                &format!("Value before edit: {before}"),
                Role::Muted,
            );
            y += 2;
        }
        let last = r.bottom().saturating_sub(4);
        for (i, choice) in self.choices.iter().enumerate() {
            if y > last {
                break;
            }
            let cursor = i == self.cursor;
            let focused = cursor && self.focus == PickerFocus::Choices;
            if cursor {
                p.text(x, y, 1, p.glyphs.cursor, Role::Key);
            }
            let role = if focused { Role::Key } else { Role::Text };
            let end = p.text(x + 2, y, w.saturating_sub(2), &choice.label, role);
            if choice.current {
                let ok = p.glyphs.ok;
                p.text(end + 2, y, w.saturating_sub(end + 2 - x), ok, Role::Ok);
            }
            if focused {
                p.cursor_row(Rect::new(x, y, w, 1));
            }
            y += 1;
        }
        let by = r.bottom().saturating_sub(2);
        let apply = "[ Apply ]";
        let cancel = "[ Cancel ]";
        let (ar, cr) = match self.focus {
            PickerFocus::Apply => (Role::Key, Role::Text),
            PickerFocus::Cancel => (Role::Text, Role::Key),
            PickerFocus::Choices => (Role::Text, Role::Text),
        };
        p.text(x + 2, by, 9, apply, ar);
        p.text(x + 13, by, 10, cancel, cr);
        match self.focus {
            PickerFocus::Apply => p.cursor_row(Rect::new(x + 2, by, 9, 1)),
            PickerFocus::Cancel => p.cursor_row(Rect::new(x + 13, by, 10, 1)),
            PickerFocus::Choices => {}
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sr() -> Picker {
        let labels = [
            "Follow All games (balanced)",
            "none",
            "off",
            "ultra_performance",
            "performance",
            "balanced",
            "quality",
            "dlaa",
        ];
        Picker::new(
            "Edit Super resolution",
            Some("quality".into()),
            Picker::choices(&labels, Some("quality")),
        )
    }

    #[test]
    fn the_cursor_starts_on_the_stored_value() {
        assert_eq!(sr().cursor, 6);
    }

    #[test]
    fn typing_jumps_as_go_does() {
        let mut p = sr();
        p.handle(PickerInput::Type('p'));
        assert_eq!(p.choices[p.cursor].label, "performance");
        assert_eq!(p.jump, "p");
        let mut p = sr();
        p.handle(PickerInput::Type('o'));
        assert_eq!(p.choices[p.cursor].label, "off");
        p.handle(PickerInput::Type('o'));
        assert_eq!(
            p.cursor, 2,
            "o again: the next match is off itself, wrapping"
        );
        p.handle(PickerInput::Type('u'));
        assert_eq!(
            p.choices[p.cursor].label, "ultra_performance",
            "ou matches nothing, u restarts"
        );
        assert_eq!(p.jump, "u");
        let mut p = sr();
        p.handle(PickerInput::Type('n'));
        assert_eq!(p.choices[p.cursor].label, "none");
        p.handle(PickerInput::Type('n'));
        assert_eq!(p.choices[p.cursor].label, "none", "the only n");
    }

    #[test]
    fn enter_applies_by_focus_and_esc_cancels() {
        let mut p = sr();
        p.handle(PickerInput::Down);
        assert_eq!(p.handle(PickerInput::Enter), PickerOutcome::Apply(7));
        p.handle(PickerInput::Tab);
        p.handle(PickerInput::Tab);
        assert_eq!(p.focus, PickerFocus::Cancel);
        assert_eq!(p.handle(PickerInput::Enter), PickerOutcome::Cancel);
        assert_eq!(p.handle(PickerInput::Esc), PickerOutcome::Cancel);
        assert_eq!(p.height(), 8 + 6);
    }
}
