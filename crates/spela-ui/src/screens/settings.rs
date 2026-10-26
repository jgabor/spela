//! 4 Settings: groups and options in List, the option in Detail.
//!
//! Every config key the TUI shows comes from
//! [`spela_core::config::OPTIONS`], by section (Display, Startup, Paths,
//! DLL policy, Logging), so Settings, `config set` and the palette list the
//! same keys with the same labels and checks.
//!
//! - **List**: the groups stack in one column and the current group opens
//!   below its name (mockups, screen 6). Up and Down choose an option and
//!   Left and Right a group; both wrap, and a new group starts on its first
//!   option, as in Go. Enter opens the option in Detail.
//! - **Detail**: the option's label, value, save state, description, the
//!   `Choices` line for a cyclic option and its default. Space picks the
//!   next value of a bool, choice or number option. Enter opens the editor
//!   for a path, text, URL or list option (TUI 10). There is no Up or Down
//!   here, as in Go.
//!
//! Saving is local first: a change shows in the same frame, through
//! [`AppState::set_config`], so the theme, glyphs and mouse follow at once.
//! The write goes to the write queue as a [`Write::Config`], which edits
//! `config.toml` with toml_edit. "Saved: <Label> <value>" follows; a
//! failure puts the stored value back, unless a newer change of the same
//! key is still on its way, and says "Not saved: <error>".
//!
//! Not here: Go applies `log_level` to the running TUI after a save. The
//! Rust logger has no reload handle yet, so a new level takes effect at the
//! next start.

use std::collections::BTreeMap;

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::config::{Config, ConfigOption, OptionKind, SettingsSection};

use super::{Action as AnyAction, Dest, Owner, Row, RowKind, ScreenAction};
use crate::editor::{Editor, EditorInput, EditorOutcome};
use crate::event::{Cmd as AnyCmd, Write};
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::{Pane, PaneArea};
use crate::paint::{Painter, Span};
use crate::state::{AppState, Level};
use crate::text::width;
use crate::theme::Role;

/// The Settings screen's state.
#[derive(Clone, Debug, Default)]
pub struct State {
    /// The current group, an index into [`SettingsSection::ALL`].
    pub group: usize,
    /// The current option within the group.
    pub cursor: usize,
    /// The open editor, for a path, text, URL or list option.
    pub editor: Option<Editor>,
    /// The config as last confirmed on disk. `None` until the first
    /// change, when it is the config the TUI started with.
    pub stored: Option<Config>,
    /// Saves on their way, by token: the key and the value text sent.
    pub pending: BTreeMap<u64, (&'static str, String)>,
    /// The last failed save of a key, shown in Detail until it saves.
    pub errors: BTreeMap<&'static str, String>,
}

/// What the Settings keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// The previous option, wrapping.
    Up,
    /// The next option, wrapping.
    Down,
    /// The previous group, wrapping.
    PrevGroup,
    /// The next group, wrapping.
    NextGroup,
    /// Opens a group from a click on its name.
    SelectGroup(usize),
    /// Moves to an option of a group, from a click.
    Select(usize, usize),
    /// Opens the option in Detail.
    Open,
    /// Space: the next value of a bool, choice or number option.
    Next,
    /// Enter: the editor of a path, text, URL or list option.
    Edit,
    /// A key for the open editor.
    Editor(EditorInput),
    /// Shows an option by its `config set` key, from the palette.
    Jump(&'static str),
}

/// Settings has no worker results of its own: saves answer through
/// [`saved`].
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {}

/// Settings asks the shell for nothing beyond [`AnyCmd::Save`] and
/// [`AnyCmd::SetMouse`].
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {}

fn act(a: Action) -> AnyAction {
    AnyAction::Screen(ScreenAction::Settings(a))
}

fn editor_input(input: EditorInput) -> AnyAction {
    act(Action::Editor(input))
}

fn editor_typed(c: char) -> AnyAction {
    act(Action::Editor(EditorInput::Type(c)))
}

/// The options of a group, in display order.
pub fn options(group: usize) -> Vec<&'static ConfigOption> {
    SettingsSection::ALL
        .get(group)
        .map(|s| s.options().collect())
        .unwrap_or_default()
}

/// The option under the cursor.
pub fn current(state: &AppState) -> Option<&'static ConfigOption> {
    options(state.settings.group)
        .get(state.settings.cursor)
        .copied()
}

/// The values Space steps through, when the option has a fixed set.
pub fn choices(option: &ConfigOption) -> Option<&'static [&'static str]> {
    match option.kind {
        OptionKind::Bool => Some(&["true", "false"]),
        OptionKind::Choice(c) | OptionKind::Int(c) => Some(c),
        OptionKind::Text | OptionKind::Folder | OptionKind::Url | OptionKind::FolderList => None,
    }
}

/// Whether Enter opens an editor for the option: paths, text, URLs and
/// lists, as in Go.
pub fn editable(option: &ConfigOption) -> bool {
    choices(option).is_none()
}

/// A value as Settings shows it: On and Off for a bool, `none` for an
/// empty value, Go's words (TUI 10).
pub fn shown(option: &ConfigOption, raw: &str) -> String {
    match option.kind {
        OptionKind::Bool if raw == "true" => "On".to_owned(),
        OptionKind::Bool => "Off".to_owned(),
        _ if raw.is_empty() => "none".to_owned(),
        _ => raw.to_owned(),
    }
}

/// The value Space moves to: the next choice after the current one,
/// wrapping. A value that is not a choice moves on from the first, as in
/// Go's `cycleValue`.
pub fn next_value(option: &ConfigOption, current: &str) -> Option<&'static str> {
    let list = choices(option)?;
    let i = list.iter().position(|c| *c == current).unwrap_or(0);
    list.get((i + 1) % list.len()).copied()
}

fn group_title(group: usize) -> &'static str {
    SettingsSection::ALL
        .get(group)
        .map_or("Settings", |s| s.title())
}

/// The pane titles: `Settings · <group>` and the option's label.
pub fn title(state: &AppState, pane: Pane) -> String {
    match pane {
        Pane::List => format!(
            "Settings {} {}",
            state.glyphs.dot,
            group_title(state.settings.group)
        ),
        Pane::Detail => current(state).map_or_else(|| "Setting".to_owned(), |o| o.label.to_owned()),
    }
}

/// Whether the editor is open: then it takes typed keys.
pub fn editing(state: &AppState, _pane: Pane) -> bool {
    state.settings.editor.is_some()
}

/// The keys of a pane.
pub fn bindings(state: &AppState, pane: Pane) -> Vec<Binding> {
    if let Some(editor) = &state.settings.editor {
        return editor.bindings(editor_input, editor_typed);
    }
    match pane {
        Pane::List => vec![
            Binding::new(Key::UP, act(Action::Up), "Option").hint_as("Up/Down", 100),
            Binding::new(Key::DOWN, act(Action::Down), "Next option"),
            Binding::new(Key::LEFT, act(Action::PrevGroup), "Group").hint_as("Left/Right", 95),
            Binding::new(Key::RIGHT, act(Action::NextGroup), "Next group"),
            Binding::new(Key::ENTER, act(Action::Open), "Open").hint(90),
        ],
        Pane::Detail => {
            let Some(option) = current(state) else {
                return Vec::new();
            };
            if editable(option) {
                vec![Binding::new(Key::ENTER, act(Action::Edit), "Edit value").hint(90)]
            } else {
                vec![Binding::new(Key::SPACE, act(Action::Next), "Next value").hint(90)]
            }
        }
    }
}

/// The Actions rows: `Edit value` on a path, text, URL or list option in
/// Detail, as in Go (TUI 7).
pub fn actions(state: &AppState, pane: Pane) -> Vec<Row> {
    match (pane, current(state)) {
        (Pane::Detail, Some(option)) if editable(option) => {
            vec![Row::new("Edit value", act(Action::Edit)).key(Key::ENTER)]
        }
        _ => Vec::new(),
    }
}

/// One palette row per setting, which opens it in Detail.
pub fn palette(_state: &AppState) -> Vec<Row> {
    SettingsSection::ALL
        .iter()
        .flat_map(|s| s.options())
        .map(|o| Row::new(o.label, act(Action::Jump(o.key))).kind(RowKind::Setting))
        .collect()
}

/// Runs an action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    let n = options(state.settings.group).len();
    let groups = SettingsSection::ALL.len();
    let s = &mut state.settings;
    match action {
        Action::Up if n > 0 => s.cursor = (s.cursor + n - 1) % n,
        Action::Down if n > 0 => s.cursor = (s.cursor + 1) % n,
        Action::Up | Action::Down => {}
        Action::PrevGroup => {
            s.group = (s.group + groups - 1) % groups;
            s.cursor = 0;
        }
        Action::NextGroup => {
            s.group = (s.group + 1) % groups;
            s.cursor = 0;
        }
        Action::SelectGroup(g) => {
            s.editor = None;
            if g != s.group && g < groups {
                s.group = g;
                s.cursor = 0;
            }
        }
        Action::Select(g, o) => {
            s.editor = None;
            if g < groups && o < options(g).len() {
                s.group = g;
                s.cursor = o;
            }
        }
        Action::Open => state.focus = Pane::Detail,
        Action::Next => return cycle(state),
        Action::Edit => open_editor(state),
        Action::Editor(input) => return editor_key(state, input),
        Action::Jump(key) => {
            for g in 0..groups {
                if let Some(o) = options(g).iter().position(|o| o.key == key) {
                    s.group = g;
                    s.cursor = o;
                }
            }
            s.editor = None;
            state.dest = Dest::Settings;
            state.focus = Pane::Detail;
        }
    }
    Vec::new()
}

fn cycle(state: &mut AppState) -> Vec<AnyCmd> {
    let Some(option) = current(state) else {
        return Vec::new();
    };
    let now = state.config.get(option.key).unwrap_or_default();
    match next_value(option, &now) {
        Some(next) => change(state, option, next),
        None => Vec::new(),
    }
}

fn open_editor(state: &mut AppState) {
    let Some(option) = current(state) else {
        return;
    };
    if !editable(option) {
        return;
    }
    let value = state.config.get(option.key).unwrap_or_default();
    let hint = match option.kind {
        OptionKind::FolderList => "Folders separated by colons. Empty clears the list.",
        OptionKind::Folder => "An absolute folder. Empty uses the default.",
        OptionKind::Url => "An https URL. Empty uses spela's own manifest.",
        _ => "Empty finds it.",
    };
    let editor = Editor::new(
        format!("Edit {}", option.label),
        Some(value.clone()),
        &value,
    )
    .hint(hint);
    state.settings.editor = Some(editor);
    state.focus = Pane::Detail;
}

fn editor_key(state: &mut AppState, input: EditorInput) -> Vec<AnyCmd> {
    let Some(editor) = &mut state.settings.editor else {
        return Vec::new();
    };
    match editor.handle(input) {
        EditorOutcome::Moved => Vec::new(),
        EditorOutcome::Cancel => {
            state.settings.editor = None;
            Vec::new()
        }
        EditorOutcome::Apply(text) => {
            let Some(option) = current(state) else {
                state.settings.editor = None;
                return Vec::new();
            };
            // Check the value on a copy first, so a refused value keeps the
            // editor open with Go's text, as in Go.
            let mut check = state.config.clone();
            if let Err(e) = check.set(option.key, &text) {
                if let Some(editor) = &mut state.settings.editor {
                    editor.fail(e.to_string());
                }
                return Vec::new();
            }
            state.settings.editor = None;
            change(state, option, &text)
        }
    }
}

/// Sets one option on screen at once and queues its save.
fn change(state: &mut AppState, option: &'static ConfigOption, raw: &str) -> Vec<AnyCmd> {
    let mut next = state.config.clone();
    if let Err(e) = next.set(option.key, raw) {
        state.say(Level::Error, format!("Not saved: {e}"));
        return Vec::new();
    }
    if next == state.config {
        return Vec::new();
    }
    if state.settings.stored.is_none() {
        state.settings.stored = Some(state.config.clone());
    }
    let mut cmds = state.set_config(next);
    let (token, cmd) = state.save(
        Owner::Settings,
        Write::Config {
            changes: BTreeMap::from([(option.key.to_owned(), raw.to_owned())]),
        },
    );
    state
        .settings
        .pending
        .insert(token, (option.key, raw.to_owned()));
    state.settings.errors.remove(option.key);
    cmds.push(cmd);
    cmds
}

/// Copies one setting from `from` to `to`. A list is copied whole, so a
/// folder with a `:` in its name survives.
fn copy_setting(from: &Config, to: &mut Config, key: &str) {
    if key == "additional_library_paths" {
        to.additional_library_paths = from.additional_library_paths.clone();
    } else if let Some(raw) = from.get(key) {
        // The value was valid when it was stored, so it sets again.
        let _ = to.set(key, &raw);
    }
}

/// Settings has no worker results.
pub fn on_msg(_state: &mut AppState, msg: Msg) -> Vec<AnyCmd> {
    match msg {}
}

/// A save finished: "Saved: <Label> <value>", or the stored value back and
/// "Not saved: <error>".
pub fn saved(state: &mut AppState, token: u64, result: Result<(), String>) -> Vec<AnyCmd> {
    let Some((key, raw)) = state.settings.pending.remove(&token) else {
        return Vec::new();
    };
    let Some(option) = spela_core::config::option(key) else {
        return Vec::new();
    };
    let mut stored = state
        .settings
        .stored
        .take()
        .unwrap_or_else(|| state.config.clone());
    let cmds = match result {
        Ok(()) => {
            let _ = stored.set(key, &raw);
            state.settings.errors.remove(key);
            let value = stored.get(key).unwrap_or_default();
            state.say(
                Level::Ok,
                format!("Saved: {} {}", option.label, shown(option, &value)),
            );
            Vec::new()
        }
        Err(e) => {
            state.settings.errors.insert(key, e.clone());
            state.say(Level::Error, format!("Not saved: {e}"));
            // A newer change of this key is still on its way: it decides
            // what is stored, so it stays on screen.
            let newer = state.settings.pending.values().any(|(k, _)| *k == key);
            if newer {
                Vec::new()
            } else {
                let mut back = state.config.clone();
                copy_setting(&stored, &mut back, key);
                state.set_config(back)
            }
        }
    };
    state.settings.stored = Some(stored);
    cmds
}

/// The wheel moves the List cursor.
pub fn wheel(state: &mut AppState, pane: Pane, down: bool) -> Vec<AnyCmd> {
    if pane == Pane::List && state.settings.editor.is_none() {
        return update(state, if down { Action::Down } else { Action::Up });
    }
    Vec::new()
}

/// Draws a pane.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = area.inner;
    if inner.height == 0 || inner.width < 6 {
        return;
    }
    match area.pane {
        Pane::List => render_list(state, &mut p, inner, hits),
        Pane::Detail => render_detail(state, &mut p, inner),
    }
}

fn render_list(state: &AppState, p: &mut Painter<'_>, inner: Rect, hits: &mut HitMap) {
    let s = &state.settings;
    let focused = state.focus == Pane::List;
    let x = inner.x + 1;
    let w = inner.width - 2;
    let mut y = inner.y;
    for (g, section) in SettingsSection::ALL.iter().enumerate() {
        if y >= inner.bottom() {
            return;
        }
        let open = g == s.group;
        let role = if open { Role::Title } else { Role::Text };
        p.text(x, y, w, section.title(), role);
        hits.push(
            Rect::new(inner.x, y, inner.width, 1),
            HitTarget::Row {
                select: act(Action::SelectGroup(g)),
                open: act(Action::Open),
            },
        );
        y += 1;
        if !open {
            continue;
        }
        for (o, option) in section.options().enumerate() {
            if y >= inner.bottom() {
                return;
            }
            let line = Rect::new(inner.x, y, inner.width, 1);
            let cursor = o == s.cursor;
            if cursor {
                let role = if focused { Role::Key } else { Role::Muted };
                p.text(x, y, 1, state.glyphs.cursor, role);
            }
            let role = if cursor { Role::Strong } else { Role::Text };
            p.text(x + 2, y, w.saturating_sub(2), option.label, role);
            if cursor {
                p.cursor_row(line);
            }
            hits.push(
                line,
                HitTarget::Row {
                    select: act(Action::Select(g, o)),
                    open: act(Action::Open),
                },
            );
            y += 1;
        }
    }
}

fn render_detail(state: &AppState, p: &mut Painter<'_>, inner: Rect) {
    let s = &state.settings;
    let x = inner.x + 1;
    let w = inner.width - 2;
    let bottom = inner.bottom();
    let Some(option) = current(state) else {
        p.text(x, inner.y, w, "No setting selected", Role::Muted);
        return;
    };
    let mut y = inner.y;
    let line = |p: &mut Painter<'_>, y: &mut u16, spans: &[Span<'_>]| {
        if *y < bottom {
            p.spans(x, *y, w, spans);
        }
        *y += 1;
    };
    line(p, &mut y, &[(option.label, Role::Title)]);
    let raw = state.config.get(option.key).unwrap_or_default();
    for part in wrap_value(&shown(option, &raw), usize::from(w)) {
        line(p, &mut y, &[(part.as_str(), Role::Strong)]);
    }
    let status = match s.errors.get(option.key) {
        Some(e) => (format!("Not saved: {e}"), Role::Error),
        None if s.pending.values().any(|(k, _)| *k == option.key) => {
            ("Saving...".to_owned(), Role::Warn)
        }
        None => ("Saved at once".to_owned(), Role::Muted),
    };
    line(p, &mut y, &[(status.0.as_str(), status.1)]);
    y += 1;
    if let Some(editor) = &s.editor
        && y < bottom
    {
        let h = editor.height().min(bottom - y);
        editor.render(p, Rect::new(inner.x + 1, y, inner.width - 2, h));
        y += h + 1;
    }
    for part in crate::text::wrap(option.description, usize::from(w)) {
        line(p, &mut y, &[(part.as_str(), Role::Muted)]);
    }
    if let Some(list) = choices(option) {
        y += 1;
        let words: Vec<String> = list
            .iter()
            .map(|c| {
                let word = shown(option, c);
                if *c == raw { format!("[{word}]") } else { word }
            })
            .collect();
        let mut spans: Vec<Span<'_>> = vec![("Choices: ", Role::Muted)];
        for (i, (word, c)) in words.iter().zip(list.iter()).enumerate() {
            if i > 0 {
                spans.push(("  ", Role::Text));
            }
            let role = if *c == raw { Role::Key } else { Role::Muted };
            spans.push((word.as_str(), role));
        }
        line(p, &mut y, &spans);
    }
    y += 1;
    let default = Config::default().get(option.key).unwrap_or_default();
    let default = shown(option, &default);
    line(
        p,
        &mut y,
        &[("Default: ", Role::Muted), (default.as_str(), Role::Text)],
    );
}

/// Wraps a value to `cols` cells, breaking after a `/` where it can, so a
/// long path stays readable, as Go's Settings does.
pub fn wrap_value(value: &str, cols: usize) -> Vec<String> {
    let cols = cols.max(1);
    let mut out = Vec::new();
    let mut rest: Vec<char> = value.chars().collect();
    while width(&rest.iter().collect::<String>()) > cols {
        let mut used = 0;
        let mut fit = 0;
        for c in &rest {
            let cw = width(c.encode_utf8(&mut [0u8; 4]));
            if used + cw > cols {
                break;
            }
            used += cw;
            fit += 1;
        }
        let fit = fit.max(1);
        let cut = rest[..fit]
            .iter()
            .rposition(|c| *c == '/')
            .filter(|i| *i > 0)
            .map_or(fit, |i| i + 1);
        out.push(rest[..cut].iter().collect());
        rest.drain(..cut);
    }
    out.push(rest.into_iter().collect());
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;

    fn open(state: &mut AppState, key_name: &str) {
        update(
            state,
            Action::Jump(spela_core::config::option(key_name).unwrap().key),
        );
    }

    #[test]
    fn space_cycles_and_a_number_off_the_list_moves_on_from_the_first() {
        let o = spela_core::config::option("manifest_refresh_hours").unwrap();
        assert_eq!(next_value(o, "24"), Some("48"));
        assert_eq!(next_value(o, "168"), Some("1"));
        assert_eq!(next_value(o, "30"), Some("6"), "Go's cycleValue");
        let b = spela_core::config::option("ui.mouse").unwrap();
        assert_eq!(next_value(b, "true"), Some("false"));
        assert_eq!(shown(b, "false"), "Off");
        let f = spela_core::config::option("backup_path").unwrap();
        assert_eq!(next_value(f, ""), None);
        assert_eq!(shown(f, ""), "none");
    }

    #[test]
    fn groups_and_options_wrap_and_a_new_group_starts_at_the_top() {
        let mut s = state_with_games(120, 40, 2);
        step(&mut s, key("4"));
        assert_eq!(current(&s).unwrap().key, "ui.theme");
        step(&mut s, key("up"));
        assert_eq!(current(&s).unwrap().key, "confirm_destructive");
        step(&mut s, key("left"));
        assert_eq!(group_title(s.settings.group), "Logging");
        assert_eq!(s.settings.cursor, 0);
        step(&mut s, key("right"));
        step(&mut s, key("right"));
        assert_eq!(current(&s).unwrap().key, "rescan_on_startup");
    }

    #[test]
    fn a_fast_second_change_survives_the_first_ones_failure() {
        let mut s = state_with_games(120, 40, 2);
        open(&mut s, "manifest_refresh_hours");
        let first = update(&mut s, Action::Next);
        let second = update(&mut s, Action::Next);
        assert_eq!(s.config.manifest_refresh_hours, 168, "shown at once");
        let token = |cmds: &[AnyCmd]| match cmds.last() {
            Some(AnyCmd::Save { token, .. }) => *token,
            other => panic!("{other:?}"),
        };
        let (t1, t2) = (token(&first), token(&second));
        crate::screens::saved(&mut s, Owner::Settings, t1, Err("disk full".into()));
        assert_eq!(
            s.config.manifest_refresh_hours, 168,
            "the newer change stays on screen"
        );
        crate::screens::saved(&mut s, Owner::Settings, t2, Ok(()));
        assert_eq!(
            s.settings.stored.as_ref().unwrap().manifest_refresh_hours,
            168
        );
        assert_eq!(
            s.status.as_ref().unwrap().text,
            "Saved: Refresh interval 168"
        );
        assert!(s.settings.errors.is_empty());
    }

    #[test]
    fn a_failed_mouse_save_puts_the_value_back_and_turns_the_mouse_on_again() {
        let mut s = state_with_games(120, 40, 2);
        open(&mut s, "ui.mouse");
        let cmds = update(&mut s, Action::Next);
        assert!(cmds.contains(&AnyCmd::SetMouse(false)), "{cmds:?}");
        let Some(AnyCmd::Save { token, .. }) = cmds.last() else {
            panic!("{cmds:?}")
        };
        let back = saved(&mut s, *token, Err("config file is read only".into()));
        assert_eq!(back, vec![AnyCmd::SetMouse(true)]);
        assert!(s.config.ui.mouse);
        assert_eq!(
            s.status.as_ref().unwrap().text,
            "Not saved: config file is read only"
        );
        assert_eq!(s.settings.errors["ui.mouse"], "config file is read only");
    }

    #[test]
    fn the_editor_refuses_a_relative_path_and_saves_an_absolute_one() {
        let mut s = state_with_games(120, 40, 2);
        open(&mut s, "backup_path");
        assert!(
            bindings(&s, Pane::Detail)
                .iter()
                .any(|b| b.label == "Edit value")
        );
        step(&mut s, key("enter"));
        assert!(editing(&s, Pane::Detail));
        for c in "rel".chars() {
            step(&mut s, crate::event::Event::Key(Key::char(c)));
        }
        let cmds = step(&mut s, key("enter"));
        assert!(cmds.is_empty());
        let e = s.settings.editor.as_ref().unwrap();
        assert_eq!(
            e.error.as_deref(),
            Some("backup_path must be an absolute path: rel")
        );
        step(&mut s, key("home"));
        step(&mut s, crate::event::Event::Key(Key::char('/')));
        let cmds = step(&mut s, key("enter"));
        assert!(s.settings.editor.is_none());
        assert_eq!(s.config.backup_path, "/rel");
        assert!(
            matches!(&cmds[..], [AnyCmd::Save { write: Write::Config { changes }, .. }] if changes["backup_path"] == "/rel"),
            "{cmds:?}"
        );
    }

    #[test]
    fn letters_type_into_the_editor_and_esc_cancels() {
        let mut s = state_with_games(120, 40, 2);
        open(&mut s, "steam_path");
        step(&mut s, key("enter"));
        for k in ["q", "1", "?", "Q"] {
            step(&mut s, key(k));
        }
        assert!(!s.quit && s.dest == Dest::Settings && s.popup.is_none());
        assert_eq!(s.settings.editor.as_ref().unwrap().input.text(), "q1?Q");
        step(&mut s, key("esc"));
        assert!(s.settings.editor.is_none());
        assert_eq!(s.config.steam_path, "", "nothing saved");
    }

    #[test]
    fn long_paths_wrap_after_a_slash() {
        assert_eq!(
            wrap_value("/home/user/games/steam", 12),
            ["/home/user/", "games/steam"]
        );
        assert_eq!(wrap_value("abcdefgh", 3), ["abc", "def", "gh"]);
        assert_eq!(wrap_value("", 3), [""]);
    }
}
