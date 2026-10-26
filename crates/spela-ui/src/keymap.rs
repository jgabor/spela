//! Keys and the binding engine.
//!
//! Adapted from mejl-ui's `keymap/key.rs` and `keymap/mod.rs`, without key
//! sequences, presets, counts or a user `[keys]` table: spela's keys are a
//! compile-time table (terminal-ux.md, keys).
//!
//! [`Key::from_event`] folds every terminal's way of reporting a key into
//! one canonical form, so a binding written for `?` or Shift-Tab matches on
//! every terminal. A [`Binding`] ties keys to an [`Action`] with the label
//! the hint bar and Help show and, when it cannot run now, the reason.
//! [`resolve`] walks the composed bindings in order and the first one that
//! can run wins. The hint bar, Help and dispatch all read the same list
//! ([`crate::screens::bindings`]), so a key never shows unless it works.

use std::fmt;
use std::str::FromStr;

use crossterm::event::{KeyCode, KeyEvent, KeyEventKind, KeyModifiers};

use crate::screens::Action;

/// A key without modifiers.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum KeyName {
    /// A printable character. Shift is already folded into it, so `Q`
    /// means Shift+q. Space is `Char(' ')`.
    Char(char),
    /// Enter or Return.
    Enter,
    /// Escape.
    Esc,
    /// Tab.
    Tab,
    /// Shift+Tab.
    BackTab,
    /// Backspace.
    Backspace,
    /// Delete.
    Delete,
    /// Arrow up.
    Up,
    /// Arrow down.
    Down,
    /// Arrow left.
    Left,
    /// Arrow right.
    Right,
    /// Home.
    Home,
    /// End.
    End,
    /// Page up.
    PageUp,
    /// Page down.
    PageDown,
}

/// A key with modifiers, in canonical form.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct Key {
    /// The key.
    pub name: KeyName,
    /// Control held.
    pub ctrl: bool,
    /// Alt (Meta) held.
    pub alt: bool,
    /// Shift held, only for keys where Shift is not already part of the
    /// character: named keys, Space, and Ctrl+letter.
    pub shift: bool,
}

impl Key {
    /// A plain character key.
    pub const fn char(c: char) -> Key {
        Key {
            name: KeyName::Char(c),
            ctrl: false,
            alt: false,
            shift: false,
        }
    }

    /// A named key without modifiers.
    pub const fn named(name: KeyName) -> Key {
        Key {
            name,
            ctrl: false,
            alt: false,
            shift: false,
        }
    }

    /// Ctrl plus a character.
    pub const fn ctrl(c: char) -> Key {
        Key {
            name: KeyName::Char(c),
            ctrl: true,
            alt: false,
            shift: false,
        }
    }

    /// Enter.
    pub const ENTER: Key = Key::named(KeyName::Enter);
    /// Escape.
    pub const ESC: Key = Key::named(KeyName::Esc);
    /// Tab.
    pub const TAB: Key = Key::named(KeyName::Tab);
    /// Shift+Tab.
    pub const BACK_TAB: Key = Key::named(KeyName::BackTab);
    /// Arrow up.
    pub const UP: Key = Key::named(KeyName::Up);
    /// Arrow down.
    pub const DOWN: Key = Key::named(KeyName::Down);
    /// Arrow left.
    pub const LEFT: Key = Key::named(KeyName::Left);
    /// Arrow right.
    pub const RIGHT: Key = Key::named(KeyName::Right);
    /// Space.
    pub const SPACE: Key = Key::char(' ');
    /// Backspace.
    pub const BACKSPACE: Key = Key::named(KeyName::Backspace);
    /// Page up.
    pub const PAGE_UP: Key = Key::named(KeyName::PageUp);
    /// Page down.
    pub const PAGE_DOWN: Key = Key::named(KeyName::PageDown);
    /// Home.
    pub const HOME: Key = Key::named(KeyName::Home);
    /// End.
    pub const END: Key = Key::named(KeyName::End);

    /// Canonical form of a crossterm key event, or `None` for a release or
    /// repeat event and for keys spela never binds (media keys, function
    /// keys, modifier-only presses).
    pub fn from_event(event: &KeyEvent) -> Option<Key> {
        if event.kind == KeyEventKind::Release {
            return None;
        }
        let m = event.modifiers;
        let ctrl = m.contains(KeyModifiers::CONTROL);
        let alt = m.contains(KeyModifiers::ALT);
        let shift = m.contains(KeyModifiers::SHIFT);
        let (name, shift) = match event.code {
            KeyCode::Char(c) if ctrl && c.is_ascii_alphabetic() => (
                KeyName::Char(c.to_ascii_lowercase()),
                shift || c.is_ascii_uppercase(),
            ),
            KeyCode::Char(' ') => (KeyName::Char(' '), shift),
            KeyCode::Char(c) => (KeyName::Char(c), false),
            KeyCode::Enter => (KeyName::Enter, shift),
            KeyCode::Esc => (KeyName::Esc, shift),
            KeyCode::Tab if shift => (KeyName::BackTab, false),
            KeyCode::Tab => (KeyName::Tab, false),
            KeyCode::BackTab => (KeyName::BackTab, false),
            KeyCode::Backspace => (KeyName::Backspace, shift),
            KeyCode::Delete => (KeyName::Delete, shift),
            KeyCode::Up => (KeyName::Up, shift),
            KeyCode::Down => (KeyName::Down, shift),
            KeyCode::Left => (KeyName::Left, shift),
            KeyCode::Right => (KeyName::Right, shift),
            KeyCode::Home => (KeyName::Home, shift),
            KeyCode::End => (KeyName::End, shift),
            KeyCode::PageUp => (KeyName::PageUp, shift),
            KeyCode::PageDown => (KeyName::PageDown, shift),
            _ => return None,
        };
        Some(Key {
            name,
            ctrl,
            alt,
            shift,
        })
    }

    /// The character typed, for text inputs: a printable key with no Ctrl
    /// or Alt. Space counts.
    pub fn text(&self) -> Option<char> {
        match self.name {
            KeyName::Char(c) if !self.ctrl && !self.alt => Some(c),
            _ => None,
        }
    }

    /// Why legacy key encoding cannot deliver this key, if it cannot. No
    /// default binding may have one (terminal-ux.md, principle 3).
    pub fn legacy_problem(&self) -> Option<&'static str> {
        if self.alt {
            return Some(
                "Alt arrives as Esc followed by the key without the kitty keyboard protocol",
            );
        }
        match (self.name, self.ctrl, self.shift) {
            (KeyName::Char('i'), true, false) => Some("Ctrl-i is the same byte as Tab"),
            (KeyName::Char('m'), true, false) => Some("Ctrl-m is the same byte as Enter"),
            (KeyName::Char('h'), true, false) => {
                Some("Ctrl-h is sent as Backspace by many terminals")
            }
            (KeyName::Char('['), true, _) => Some("Ctrl-[ is the same byte as Esc"),
            (KeyName::Char(c), true, true) if c.is_ascii_alphabetic() => {
                Some("Ctrl+Shift+letter cannot be told apart from Ctrl+letter")
            }
            (KeyName::Char(c), true, _) if !c.is_ascii_alphabetic() => {
                Some("legacy encoding has no code for Ctrl with this character")
            }
            (KeyName::Char(' '), false, true) => Some("Shift-Space is sent as Space"),
            (KeyName::Char(_), _, _) => None,
            (_, true, _) => Some("Ctrl with this key needs the kitty keyboard protocol"),
            (KeyName::Enter | KeyName::Backspace | KeyName::Esc, false, true) => {
                Some("Shift with this key needs the kitty keyboard protocol")
            }
            _ => None,
        }
    }
}

/// The form the hint bar and Help show: `Up`, `Enter`, `Space`,
/// `Shift-Tab`, `Ctrl-K`, `?`. Arrow keys are words, because the arrow
/// glyphs are outside the safe glyph set (terminal-ux.md, frame).
impl fmt::Display for Key {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.ctrl {
            f.write_str("Ctrl-")?;
        }
        if self.alt {
            f.write_str("Alt-")?;
        }
        if self.shift && self.name != KeyName::BackTab {
            f.write_str("Shift-")?;
        }
        match self.name {
            KeyName::Char(' ') => f.write_str("Space"),
            KeyName::Char(c) if self.ctrl => write!(f, "{}", c.to_ascii_uppercase()),
            KeyName::Char(c) => write!(f, "{c}"),
            KeyName::Enter => f.write_str("Enter"),
            KeyName::Esc => f.write_str("Esc"),
            KeyName::Tab => f.write_str("Tab"),
            KeyName::BackTab => f.write_str("Shift-Tab"),
            KeyName::Backspace => f.write_str("Backspace"),
            KeyName::Delete => f.write_str("Delete"),
            KeyName::Up => f.write_str("Up"),
            KeyName::Down => f.write_str("Down"),
            KeyName::Left => f.write_str("Left"),
            KeyName::Right => f.write_str("Right"),
            KeyName::Home => f.write_str("Home"),
            KeyName::End => f.write_str("End"),
            KeyName::PageUp => f.write_str("PgUp"),
            KeyName::PageDown => f.write_str("PgDn"),
        }
    }
}

/// A key string that did not parse.
#[derive(Clone, Debug, PartialEq, Eq, thiserror::Error)]
#[error("{0}")]
pub struct KeyParseError(pub String);

/// Parses `q`, `Q`, `?`, `space`, `enter`, `esc`, `tab`, `shift-tab`,
/// `ctrl-k`, `up` and the like. Tests use it to write keys plainly.
impl FromStr for Key {
    type Err = KeyParseError;

    fn from_str(s: &str) -> Result<Key, KeyParseError> {
        let mut rest = s;
        let mut ctrl = false;
        if let Some(r) = rest.strip_prefix("ctrl-").filter(|r| !r.is_empty()) {
            ctrl = true;
            rest = r;
        }
        let mut chars = rest.chars();
        let name = match (chars.next(), chars.next()) {
            (Some(c), None) => KeyName::Char(c),
            _ => match rest {
                "space" => KeyName::Char(' '),
                "enter" => KeyName::Enter,
                "esc" => KeyName::Esc,
                "tab" => KeyName::Tab,
                "shift-tab" => KeyName::BackTab,
                "backspace" => KeyName::Backspace,
                "delete" => KeyName::Delete,
                "up" => KeyName::Up,
                "down" => KeyName::Down,
                "left" => KeyName::Left,
                "right" => KeyName::Right,
                "home" => KeyName::Home,
                "end" => KeyName::End,
                "pgup" => KeyName::PageUp,
                "pgdn" => KeyName::PageDown,
                other => return Err(KeyParseError(format!("unknown key {other:?}"))),
            },
        };
        let name = match name {
            KeyName::Char(c) if ctrl => KeyName::Char(c.to_ascii_lowercase()),
            n => n,
        };
        Ok(Key {
            name,
            ctrl,
            alt: false,
            shift: false,
        })
    }
}

/// How the hint bar shows a binding.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Hint {
    /// Higher stays longer when the bar runs out of space.
    /// [`Hint::ALWAYS`] is never dropped.
    pub priority: u8,
    /// The key text, when it is not the first key itself: `Up/Down`,
    /// `Left/Right`, `Type`.
    pub keys: Option<&'static str>,
}

impl Hint {
    /// The priority of a hint the bar never drops, such as `0 Actions`.
    pub const ALWAYS: u8 = u8::MAX;
}

/// One binding: keys, what they run, and how the hint bar and Help show
/// it.
#[derive(Clone, Debug)]
pub struct Binding {
    /// Any of these keys runs it.
    pub keys: Vec<Key>,
    /// Any typed character runs this instead, for text inputs, Find and
    /// type-to-jump. Checked after `keys`.
    pub typed: Option<fn(char) -> Action>,
    /// What the keys run.
    pub action: Action,
    /// The label, such as `Edit value` or `Follow All games`.
    pub label: String,
    /// Shown in the hint bar, and how.
    pub hint: Option<Hint>,
    /// Why it cannot run now. A binding with a reason never runs, and the
    /// hint bar leaves it out; Help shows the reason.
    pub unavailable: Option<String>,
}

impl Binding {
    /// A binding for one key.
    pub fn new(key: Key, action: Action, label: impl Into<String>) -> Binding {
        Binding {
            keys: vec![key],
            typed: None,
            action,
            label: label.into(),
            hint: None,
            unavailable: None,
        }
    }

    /// A binding that takes every typed character, such as a search input.
    /// `action` is what a click on its hint runs.
    pub fn typed(map: fn(char) -> Action, action: Action, label: impl Into<String>) -> Binding {
        Binding {
            keys: Vec::new(),
            typed: Some(map),
            action,
            label: label.into(),
            hint: None,
            unavailable: None,
        }
    }

    /// More keys for the same action, such as Ctrl-K beside `:`.
    pub fn also(mut self, key: Key) -> Binding {
        self.keys.push(key);
        self
    }

    /// Shows it in the hint bar with this priority.
    pub fn hint(mut self, priority: u8) -> Binding {
        self.hint = Some(Hint {
            priority,
            keys: None,
        });
        self
    }

    /// Shows it in the hint bar under other key text, such as `Up/Down`.
    pub fn hint_as(mut self, keys: &'static str, priority: u8) -> Binding {
        self.hint = Some(Hint {
            priority,
            keys: Some(keys),
        });
        self
    }

    /// Marks it unavailable with a reason, or available with `Ok`.
    pub fn when(mut self, available: Result<(), String>) -> Binding {
        self.unavailable = available.err();
        self
    }

    /// Marks it unavailable with a reason.
    pub fn unavailable(mut self, reason: impl Into<String>) -> Binding {
        self.unavailable = Some(reason.into());
        self
    }

    /// Whether it can run now.
    pub fn available(&self) -> bool {
        self.unavailable.is_none()
    }

    /// The key text the hint bar and Help show.
    pub fn key_text(&self) -> String {
        if let Some(Hint {
            keys: Some(keys), ..
        }) = self.hint
        {
            return keys.to_owned();
        }
        match (self.keys.first(), self.typed) {
            (Some(k), _) => k.to_string(),
            (None, Some(_)) => "Type".to_owned(),
            (None, None) => String::new(),
        }
    }

    /// The action this binding runs for `key`, if it matches at all.
    pub fn matches(&self, key: &Key) -> Option<Action> {
        if self.keys.contains(key) {
            return Some(self.action.clone());
        }
        let c = key.text()?;
        self.typed.map(|map| map(c))
    }
}

/// What a key resolved to.
#[derive(Clone, Debug, PartialEq)]
pub enum Resolution {
    /// Run this action.
    Run(Action),
    /// A binding matched but cannot run now, for this reason. Nothing
    /// happens: a key that cannot run is a no-op, as in Go.
    Unavailable(String),
    /// No binding matches.
    Unbound,
}

/// Finds what `key` does: the first available binding that matches, in
/// order; otherwise the first unavailable one; otherwise nothing.
pub fn resolve(bindings: &[Binding], key: &Key) -> Resolution {
    let mut blocked = None;
    for b in bindings {
        if let Some(action) = b.matches(key) {
            match &b.unavailable {
                None => return Resolution::Run(action),
                Some(reason) if blocked.is_none() => blocked = Some(reason.clone()),
                Some(_) => {}
            }
        }
    }
    blocked.map_or(Resolution::Unbound, Resolution::Unavailable)
}

/// A problem [`check`] found.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Conflict {
    /// The key.
    pub key: Key,
    /// What is wrong.
    pub message: String,
}

/// Checks one context's bindings: no key bound twice, and no key that
/// legacy encoding cannot deliver. A table that fails this is a bug.
pub fn check(bindings: &[Binding]) -> Vec<Conflict> {
    let mut out = Vec::new();
    for (i, b) in bindings.iter().enumerate() {
        for key in &b.keys {
            if let Some(problem) = key.legacy_problem() {
                out.push(Conflict {
                    key: *key,
                    message: format!("{} cannot be typed on every terminal: {problem}", b.label),
                });
            }
            if let Some(other) = bindings[..i].iter().find(|o| o.keys.contains(key)) {
                out.push(Conflict {
                    key: *key,
                    message: format!("{key} is bound to both {} and {}", other.label, b.label),
                });
            }
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crossterm::event::KeyEventState;

    fn ev(code: KeyCode, modifiers: KeyModifiers) -> KeyEvent {
        KeyEvent {
            code,
            modifiers,
            kind: KeyEventKind::Press,
            state: KeyEventState::NONE,
        }
    }

    #[test]
    fn shift_folds_into_characters_on_every_encoding() {
        // Legacy encoding reports `Q` with Shift; kitty may not.
        let legacy = Key::from_event(&ev(KeyCode::Char('Q'), KeyModifiers::SHIFT));
        let kitty = Key::from_event(&ev(KeyCode::Char('Q'), KeyModifiers::NONE));
        assert_eq!(legacy, Some(Key::char('Q')));
        assert_eq!(kitty, legacy);
        let q = Key::from_event(&ev(KeyCode::Char('?'), KeyModifiers::SHIFT));
        assert_eq!(q, Some(Key::char('?')));
        let bt = Key::from_event(&ev(KeyCode::BackTab, KeyModifiers::SHIFT));
        assert_eq!(bt, Some(Key::BACK_TAB));
        let st = Key::from_event(&ev(KeyCode::Tab, KeyModifiers::SHIFT));
        assert_eq!(st, bt);
        let ck = Key::from_event(&ev(KeyCode::Char('k'), KeyModifiers::CONTROL));
        assert_eq!(ck, Some(Key::ctrl('k')));
        let release = KeyEvent {
            kind: KeyEventKind::Release,
            ..ev(KeyCode::Char('q'), KeyModifiers::NONE)
        };
        assert_eq!(Key::from_event(&release), None);
    }

    #[test]
    fn display_uses_words_for_arrows() {
        let shown: Vec<String> = ["up", "enter", "space", "shift-tab", "ctrl-k", "?", "0"]
            .iter()
            .map(|s| s.parse::<Key>().unwrap().to_string())
            .collect();
        assert_eq!(
            shown,
            ["Up", "Enter", "Space", "Shift-Tab", "Ctrl-K", "?", "0"]
        );
        assert!("bogus".parse::<Key>().is_err());
    }

    #[test]
    fn legacy_problems_are_found() {
        for s in ["ctrl-i", "ctrl-m", "ctrl-h", "ctrl-1"] {
            assert!(s.parse::<Key>().unwrap().legacy_problem().is_some(), "{s}");
        }
        for s in [
            "ctrl-k",
            "ctrl-c",
            "ctrl-z",
            "q",
            "?",
            "enter",
            "shift-tab",
            "space",
        ] {
            assert!(s.parse::<Key>().unwrap().legacy_problem().is_none(), "{s}");
        }
    }

    #[test]
    fn the_first_available_binding_wins() {
        let bindings = vec![
            Binding::new(Key::char('x'), Action::Help, "Help").unavailable("no help"),
            Binding::new(Key::char('x'), Action::Quit, "Quit"),
            Binding::new(Key::char('y'), Action::Palette, "Palette").unavailable("busy"),
        ];
        assert_eq!(
            resolve(&bindings, &Key::char('x')),
            Resolution::Run(Action::Quit)
        );
        assert_eq!(
            resolve(&bindings, &Key::char('y')),
            Resolution::Unavailable("busy".into())
        );
        assert_eq!(resolve(&bindings, &Key::char('z')), Resolution::Unbound);
        assert!(!check(&bindings).is_empty(), "x is bound twice");
    }
}
