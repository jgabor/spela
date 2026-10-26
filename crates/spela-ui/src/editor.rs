//! A one-line text editor with Input, Apply and Cancel controls, shared by
//! every text editor: the profile page's numbers and path, the Settings
//! path and list editors, and the deny list reason.
//!
//! It is Go's field editor in spela's boxed form (parity-inventory.md, TUI
//! 8): a box titled `Edit <Label>`, a `Value before edit` line, the input,
//! an optional hint, `Invalid value: <err>` after a refused value, and
//! `[ Apply ]  [ Cancel ]`, the mockups' button row. The hint and the error
//! wrap inside the box. Tab and Shift-Tab move between the input and the
//! buttons, Left and Right switch buttons, Enter applies or cancels by
//! focus (Enter in the input applies), and Esc cancels. Typing on a button
//! does nothing, as in Go. An invalid value keeps the editor open with
//! focus on the input, as in Go. A number editor replaces its value on the
//! first typed key until the cursor moves (Go's integer editors).
//!
//! A screen keeps an [`Editor`] in its state, returns
//! [`Editor::bindings`] from its `bindings` while it is open (and `true`
//! from `editing`), feeds each [`EditorInput`] to [`Editor::handle`] and
//! draws it with [`Editor::render`]. This is the same shape as
//! [`crate::picker::Picker`].

use ratatui::layout::Rect;

use crate::input::{InputResult, LineInput};
use crate::keymap::{Binding, Key, KeyName};
use crate::paint::Painter;
use crate::screens::Action;
use crate::text::wrap;
use crate::theme::Role;

/// The longest value, as Go's text input allows.
pub const MAX_CHARS: usize = 256;

/// What has focus inside the editor.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum EditorFocus {
    /// The text input.
    Input,
    /// `[ Apply ]`.
    Apply,
    /// `[ Cancel ]`.
    Cancel,
}

/// A key the editor takes.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum EditorInput {
    /// A typed character.
    Type(char),
    /// An editing key for the input: Backspace, Delete, Left, Right, Home,
    /// End, Ctrl-u, Ctrl-w.
    Edit(Key),
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
}

/// What a key did.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum EditorOutcome {
    /// Only the editor changed.
    Moved,
    /// Apply this text. The screen checks it and calls [`Editor::fail`]
    /// when it is refused.
    Apply(String),
    /// Close without a change.
    Cancel,
}

/// An open editor.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Editor {
    /// The title, such as `Edit Backup path`.
    pub title: String,
    /// The `Value before edit` text, when there is one.
    pub before: Option<String>,
    /// The text.
    pub input: LineInput,
    /// A muted line under the input, such as what an empty value means.
    pub hint: Option<String>,
    /// Why the last value was refused.
    pub error: Option<String>,
    /// What has focus.
    pub focus: EditorFocus,
    /// The first typed key replaces the value, until the cursor moves.
    pub replace: bool,
}

impl Editor {
    /// An editor holding `value`, with focus on the input and the cursor at
    /// the end.
    pub fn new(title: impl Into<String>, before: Option<String>, value: &str) -> Editor {
        Editor {
            title: title.into(),
            before,
            input: LineInput::new(value),
            hint: None,
            error: None,
            focus: EditorFocus::Input,
            replace: false,
        }
    }

    /// Makes the first typed key replace the value, as a number editor
    /// does.
    pub fn replace_on_first_key(mut self) -> Editor {
        self.replace = true;
        self
    }

    /// Adds a hint line under the input.
    pub fn hint(mut self, text: impl Into<String>) -> Editor {
        self.hint = Some(text.into());
        self
    }

    /// Shows why a value was refused and puts focus back on the input.
    pub fn fail(&mut self, error: impl Into<String>) {
        self.error = Some(error.into());
        self.focus = EditorFocus::Input;
    }

    /// The editor's keys. `map` turns an input into the screen's action;
    /// `typed` does the same for a typed character, because a binding for
    /// typed keys takes a plain function.
    pub fn bindings(
        &self,
        map: fn(EditorInput) -> Action,
        typed: fn(char) -> Action,
    ) -> Vec<Binding> {
        // Go's words: `Type to edit  Tab control  Enter apply|cancel edit
        // Backspace delete  Esc cancel`.
        let enter = match self.focus {
            EditorFocus::Cancel => "cancel edit",
            _ => "apply",
        };
        let mut out = Vec::new();
        if self.focus == EditorFocus::Input {
            out.push(Binding::typed(typed, map(EditorInput::Esc), "to edit").hint_as("Type", 100));
        }
        out.push(Binding::new(Key::TAB, map(EditorInput::Tab), "control").hint(80));
        out.push(Binding::new(Key::ENTER, map(EditorInput::Enter), enter).hint(90));
        out.push(Binding::new(
            Key::BACK_TAB,
            map(EditorInput::BackTab),
            "Previous control",
        ));
        if self.focus == EditorFocus::Input {
            out.push(
                Binding::new(
                    Key::BACKSPACE,
                    map(EditorInput::Edit(Key::BACKSPACE)),
                    "delete",
                )
                .hint(70),
            );
        }
        out.push(Binding::new(Key::ESC, map(EditorInput::Esc), "cancel").hint(85));
        if self.focus == EditorFocus::Input {
            for key in [
                Key::named(KeyName::Delete),
                Key::LEFT,
                Key::RIGHT,
                Key::HOME,
                Key::END,
                Key::ctrl('u'),
                Key::ctrl('w'),
            ] {
                out.push(Binding::new(
                    key,
                    map(EditorInput::Edit(key)),
                    "Edit the text",
                ));
            }
        } else {
            out.push(Binding::new(
                Key::LEFT,
                map(EditorInput::Switch),
                "Choose button",
            ));
            out.push(Binding::new(
                Key::RIGHT,
                map(EditorInput::Switch),
                "Choose button",
            ));
        }
        out
    }

    /// Takes one input.
    pub fn handle(&mut self, input: EditorInput) -> EditorOutcome {
        match input {
            EditorInput::Type(c) => {
                if self.focus == EditorFocus::Input {
                    if self.replace {
                        self.input.clear();
                        self.replace = false;
                    }
                    if self.input.text().chars().count() < MAX_CHARS {
                        self.input.handle(&Key::char(c));
                        self.error = None;
                    }
                }
            }
            EditorInput::Edit(key) => {
                if self.focus == EditorFocus::Input {
                    // Moving the cursor or deleting first edits in place,
                    // as Go does.
                    self.replace = false;
                    if self.input.handle(&key) == InputResult::Changed {
                        self.error = None;
                    }
                }
            }
            EditorInput::Tab => {
                self.focus = match self.focus {
                    EditorFocus::Input => EditorFocus::Apply,
                    EditorFocus::Apply => EditorFocus::Cancel,
                    EditorFocus::Cancel => EditorFocus::Input,
                };
            }
            EditorInput::BackTab => {
                self.focus = match self.focus {
                    EditorFocus::Input => EditorFocus::Cancel,
                    EditorFocus::Apply => EditorFocus::Input,
                    EditorFocus::Cancel => EditorFocus::Apply,
                };
            }
            EditorInput::Switch => {
                self.focus = match self.focus {
                    EditorFocus::Apply => EditorFocus::Cancel,
                    _ => EditorFocus::Apply,
                };
            }
            EditorInput::Enter => {
                return match self.focus {
                    EditorFocus::Cancel => EditorOutcome::Cancel,
                    _ => EditorOutcome::Apply(self.input.text().to_owned()),
                };
            }
            EditorInput::Esc => return EditorOutcome::Cancel,
        }
        EditorOutcome::Moved
    }

    /// The hint and the error, wrapped to the text width of a box
    /// `width` wide.
    fn notes(&self, width: u16) -> Vec<(String, Role)> {
        let cols = usize::from(width.saturating_sub(4)).max(1);
        let mut out = Vec::new();
        if let Some(hint) = &self.hint {
            out.extend(wrap(hint, cols).into_iter().map(|l| (l, Role::Muted)));
        }
        if let Some(error) = &self.error {
            let text = format!("Invalid value: {error}");
            out.extend(wrap(&text, cols).into_iter().map(|l| (l, Role::Error)));
        }
        out
    }

    /// Rows the boxed form takes in a box `width` wide: the box, the before
    /// line and a blank, the input, the wrapped hint and error, a blank and
    /// the buttons.
    pub fn height(&self, width: u16) -> u16 {
        let before = u16::from(self.before.is_some()) * 2;
        let notes = u16::try_from(self.notes(width).len()).unwrap_or(u16::MAX);
        (5 + before).saturating_add(notes)
    }

    /// Draws the boxed form at `area`'s top, as wide as `area`.
    pub fn render(&self, p: &mut Painter<'_>, area: Rect) {
        let h = self.height(area.width).min(area.height);
        if h < 4 || area.width < 16 {
            return;
        }
        let r = Rect::new(area.x, area.y, area.width, h);
        p.clear(r);
        p.frame(r, &self.title, Role::Focus, Role::Key);
        let x = r.x + 2;
        let w = r.width.saturating_sub(4);
        let mut y = r.y + 1;
        let last = r.bottom().saturating_sub(3);
        if let Some(before) = &self.before {
            let shown = if before.is_empty() { "none" } else { before };
            p.text(x, y, w, &format!("Value before edit: {shown}"), Role::Muted);
            y += 2;
        }
        if y <= last {
            let (shown, cursor) = self.input.view(usize::from(w.saturating_sub(1)));
            p.text(x, y, w, &shown, Role::Strong);
            if self.focus == EditorFocus::Input {
                let cx = x + u16::try_from(cursor).unwrap_or(0);
                p.cursor_row(Rect::new(cx, y, 1, 1));
            }
            y += 1;
        }
        for (line, role) in self.notes(r.width) {
            if y > last {
                break;
            }
            p.text(x, y, w, &line, role);
            y += 1;
        }
        let by = r.bottom().saturating_sub(2);
        let apply = "[ Apply ]";
        let cancel = "[ Cancel ]";
        let (ar, cr) = match self.focus {
            EditorFocus::Apply => (Role::Key, Role::Text),
            EditorFocus::Cancel => (Role::Text, Role::Key),
            EditorFocus::Input => (Role::Text, Role::Text),
        };
        p.text(x, by, 9, apply, ar);
        p.text(x + 11, by, 10, cancel, cr);
        match self.focus {
            EditorFocus::Apply => p.cursor_row(Rect::new(x, by, 9, 1)),
            EditorFocus::Cancel => p.cursor_row(Rect::new(x + 11, by, 10, 1)),
            EditorFocus::Input => {}
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn typed(e: &mut Editor, s: &str) {
        for c in s.chars() {
            e.handle(EditorInput::Type(c));
        }
    }

    #[test]
    fn enter_applies_from_the_input_and_cancels_on_cancel() {
        let mut e = Editor::new("Edit Backup path", Some(String::new()), "");
        typed(&mut e, "/mnt/backups");
        assert_eq!(
            e.handle(EditorInput::Enter),
            EditorOutcome::Apply("/mnt/backups".into())
        );
        e.handle(EditorInput::Tab);
        assert_eq!(e.focus, EditorFocus::Apply);
        e.handle(EditorInput::Switch);
        assert_eq!(e.focus, EditorFocus::Cancel);
        assert_eq!(e.handle(EditorInput::Enter), EditorOutcome::Cancel);
        assert_eq!(e.handle(EditorInput::Esc), EditorOutcome::Cancel);
    }

    #[test]
    fn a_refused_value_keeps_the_editor_on_the_input() {
        let mut e = Editor::new("Edit Backup path", None, "rel");
        e.handle(EditorInput::Tab);
        e.fail("backup_path must be an absolute path: rel");
        assert_eq!(e.focus, EditorFocus::Input);
        assert_eq!(e.height(80), 6);
        // A reason longer than the box wraps inside it.
        assert_eq!(e.height(56), 7);
        typed(&mut e, "x");
        assert_eq!(e.error, None, "typing clears the error");
        assert_eq!(e.input.text(), "relx");
    }

    #[test]
    fn typing_stops_at_the_limit_and_only_in_the_input() {
        let mut e = Editor::new("Edit", None, &"a".repeat(MAX_CHARS));
        typed(&mut e, "b");
        assert_eq!(e.input.text().len(), MAX_CHARS);
        let mut e = Editor::new("Edit", None, "");
        e.handle(EditorInput::Tab);
        typed(&mut e, "z");
        assert!(e.input.is_empty(), "letters do nothing on the buttons");
    }

    #[test]
    fn a_number_editor_replaces_on_the_first_key_until_the_cursor_moves() {
        let mut e = Editor::new("Edit Clock offset", None, "150").replace_on_first_key();
        typed(&mut e, "2");
        assert_eq!(e.input.text(), "2");
        let mut e = Editor::new("Edit Clock offset", None, "150").replace_on_first_key();
        e.handle(EditorInput::Edit(Key::LEFT));
        typed(&mut e, "2");
        assert_eq!(e.input.text(), "1520");
    }

    #[test]
    fn ctrl_u_clears_and_ctrl_w_deletes_a_word() {
        let mut e = Editor::new("Edit", None, "");
        typed(&mut e, "/mnt/games extra");
        e.handle(EditorInput::Edit(Key::ctrl('w')));
        assert_eq!(e.input.text(), "/mnt/games ");
        e.handle(EditorInput::Edit(Key::ctrl('u')));
        assert!(e.input.is_empty());
    }
}
