//! Help (`?`, or the Help row in Actions): "Keys here" for the context it
//! was opened from, then a reference per mode, all from the keymap.
//!
//! "Keys here" is the composed binding list of that context, the same one
//! the hint bar shows and dispatch reads, so every key in it works there.
//! A reason shows only for a key that cannot run, which fixes Go's Help
//! printing a reason for keys that work (TUI conflicts).

use ratatui::Frame;
use ratatui::layout::Rect;

use super::{Action as AnyAction, Popup, PopupAction};
use crate::event::Cmd as AnyCmd;
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::Layout;
use crate::paint::Painter;
use crate::state::AppState;
use crate::theme::Role;

/// The popup's widest width.
pub const WIDTH: u16 = 100;

/// One Help line: a heading, a key with its label, or plain text.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum HelpLine {
    /// A section heading.
    Heading(String),
    /// A key, what it does, and why it cannot run now, if it cannot.
    Key {
        /// The key text, such as `Up/Down` or `Enter`.
        keys: String,
        /// What it does.
        label: String,
        /// Why it cannot run now.
        unavailable: Option<String>,
    },
    /// Plain text.
    Text(String),
    /// A blank line.
    Blank,
}

/// The Help popup.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct State {
    /// The lines.
    pub lines: Vec<HelpLine>,
    /// The first line shown.
    pub scroll: usize,
    /// The Close button has focus.
    pub on_close: bool,
}

/// The reference sections after "Keys here".
const REFERENCE: &[(&str, &[(&str, &str)])] = &[
    (
        "Everywhere",
        &[
            ("0", "Actions for this context"),
            ("1 to 4", "Library, DLL Catalog, Monitor, Settings"),
            ("Tab", "Next pane"),
            ("Shift-Tab", "Previous pane"),
            ("?", "Help"),
            (": or Ctrl-K", "Commands: every action, game and setting"),
            ("/", "Find in the current list"),
            ("Esc", "Back one level"),
            ("q", "Back one level, then quit"),
            ("Q", "Quit now, after pending saves"),
            ("Ctrl-C", "Quit at once"),
            ("Ctrl-Z", "Suspend to the shell"),
        ],
    ),
    (
        "In popups",
        &[
            ("Up/Down", "Choose"),
            ("Enter", "Run or activate"),
            ("Tab", "Next control"),
            ("Esc", "Close or cancel"),
        ],
    ),
    (
        "In editors, pickers and Find",
        &[
            ("Type", "Enter text, or jump to a choice"),
            ("Enter", "Apply"),
            ("Tab", "Next control"),
            ("Esc", "Cancel"),
        ],
    ),
];

const MOUSE: &[&str] = &[
    "Click a row to move there; double click opens it, like Enter.",
    "Click a destination, a section, a hint or a button to run it.",
    "Right click opens Actions for what is under the pointer.",
    "The wheel scrolls the pane under the pointer.",
    "Hold Shift to select text in most terminals.",
];

impl State {
    /// Help for a context, from its composed bindings.
    pub fn new(context: String, keys: &[Binding]) -> State {
        let mut lines = vec![HelpLine::Heading(format!("Keys here: {context}"))];
        let mut seen: Vec<String> = Vec::new();
        for b in keys {
            let text = b.key_text();
            if b.label.is_empty() || seen.contains(&text) {
                continue;
            }
            seen.push(text.clone());
            lines.push(HelpLine::Key {
                keys: text,
                label: b.label.clone(),
                unavailable: b.unavailable.clone(),
            });
        }
        for (heading, rows) in REFERENCE {
            lines.push(HelpLine::Blank);
            lines.push(HelpLine::Heading((*heading).to_owned()));
            for (k, l) in *rows {
                lines.push(HelpLine::Key {
                    keys: (*k).to_owned(),
                    label: (*l).to_owned(),
                    unavailable: None,
                });
            }
        }
        lines.push(HelpLine::Blank);
        lines.push(HelpLine::Heading("Mouse".into()));
        lines.extend(MOUSE.iter().map(|t| HelpLine::Text((*t).to_owned())));
        State {
            lines,
            scroll: 0,
            on_close: false,
        }
    }
}

/// What the popup's keys run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// Scroll up.
    Up,
    /// Scroll down.
    Down,
    /// A page up.
    PageUp,
    /// A page down.
    PageDown,
    /// Between the body and Close.
    Tab,
    /// Enter: closes when Close has focus.
    Enter,
    /// Closes Help.
    Close,
}

fn act(a: Action) -> AnyAction {
    AnyAction::Popup(PopupAction::Help(a))
}

/// The popup's keys.
pub fn bindings(_state: &AppState, popup: &State) -> Vec<Binding> {
    let mut out = vec![
        Binding::new(Key::UP, act(Action::Up), "Scroll").hint_as("Up/Down", 100),
        Binding::new(Key::DOWN, act(Action::Down), "Scroll down"),
        Binding::new(Key::PAGE_UP, act(Action::PageUp), "Page up"),
        Binding::new(Key::PAGE_DOWN, act(Action::PageDown), "Page down"),
        Binding::new(Key::TAB, act(Action::Tab), "Close button")
            .also(Key::BACK_TAB)
            .hint(50),
    ];
    if popup.on_close {
        out.push(Binding::new(Key::ENTER, act(Action::Enter), "Close").hint(90));
    }
    out.push(Binding::new(Key::ESC, act(Action::Close), "Close").hint(40));
    out
}

/// Runs a popup action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    let page = usize::from(state.layout().workspace().height.saturating_sub(6)).max(1);
    let Some(Popup::Help(popup)) = &mut state.popup else {
        return Vec::new();
    };
    state.dirty = true;
    let last = popup.lines.len().saturating_sub(1);
    match action {
        Action::Up => popup.scroll = popup.scroll.saturating_sub(1),
        Action::Down => popup.scroll = (popup.scroll + 1).min(last),
        Action::PageUp => popup.scroll = popup.scroll.saturating_sub(page),
        Action::PageDown => popup.scroll = (popup.scroll + page).min(last),
        Action::Tab => popup.on_close = !popup.on_close,
        Action::Enter if popup.on_close => state.close_popup(),
        Action::Enter => {}
        Action::Close => state.close_popup(),
    }
    Vec::new()
}

/// Draws the popup.
pub fn render(
    state: &AppState,
    popup: &State,
    frame: &mut Frame,
    layout: &Layout,
    hits: &mut HitMap,
) {
    let ws = layout.workspace();
    let area = layout.centered(WIDTH, ws.height);
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = p.popup(area, "Keyboard shortcuts", hits);
    if inner.height < 5 || inner.width < 20 {
        return;
    }
    let x = inner.x + 2;
    let w = inner.width.saturating_sub(4);
    p.text(
        x,
        inner.y,
        w,
        "Reference only. Close Help before using these controls.",
        Role::Muted,
    );
    let body = Rect::new(inner.x, inner.y + 2, inner.width, inner.height - 4);
    let visible = usize::from(body.height);
    let key_w = 14u16.min(w / 3);
    let scroll = popup.scroll.min(popup.lines.len().saturating_sub(visible));
    for (n, line) in popup.lines.iter().skip(scroll).take(visible).enumerate() {
        let y = body.y + u16::try_from(n).unwrap_or(0);
        match line {
            HelpLine::Heading(t) => {
                p.text(x, y, w, t, Role::Title);
            }
            HelpLine::Key {
                keys,
                label,
                unavailable,
            } => {
                p.text(x, y, key_w, keys, Role::Key);
                let text = match unavailable {
                    Some(r) => format!("{label} {} {r}", state.glyphs.dot),
                    None => label.clone(),
                };
                let role = if unavailable.is_some() {
                    Role::Muted
                } else {
                    Role::Text
                };
                p.text(x + key_w + 1, y, w.saturating_sub(key_w + 1), &text, role);
            }
            HelpLine::Text(t) => {
                p.text(x, y, w, t, Role::Text);
            }
            HelpLine::Blank => {}
        }
    }
    p.thumb(area.right() - 1, body, scroll, visible, popup.lines.len());
    let close_y = inner.bottom() - 1;
    let role = if popup.on_close {
        Role::Key
    } else {
        Role::Text
    };
    if popup.on_close {
        p.text(x - 1, close_y, 1, state.glyphs.cursor, Role::Key);
    }
    p.text(x + 1, close_y, 5, "Close", role);
    let close = Rect::new(inner.x, close_y, inner.width, 1);
    if popup.on_close {
        p.cursor_row(close);
    }
    hits.push(close, HitTarget::Action(act(Action::Close)));
}
