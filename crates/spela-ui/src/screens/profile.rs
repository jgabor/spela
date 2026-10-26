//! 1 Library, Detail pane: the profile page of the selected game or of All
//! games.
//!
//! The page is Go's (TUI 5, profile page; terminal-ux.md, profile page
//! details):
//!
//! - Three header lines: the game line with the save state, "Own
//!   settings", and the section strip `[Proton]  DLSS  GPU  CPU  DLLs`
//!   (DLLs on a game only). Left and Right jump between sections; Right
//!   from CPU opens the game's DLLs and Left comes back.
//! - One scrolling page of rows: label, value and source, with ⚑ on an own
//!   value. The words come from [`spela_core::profile::words`]. The focused
//!   row is followed by its description, and on All games by the Apply to
//!   games with their own value note. The page scrolls to keep both in view.
//! - Enter edits: a picker for On/Off, SMT and the fields with a list of
//!   values (Multi-frame included, as Go does), the CPU governor picker with
//!   the governors sysfs lists, and a text editor for the free numbers and
//!   the shader cache path. A "none" row opens an empty editor, and a
//!   number editor replaces its value on the first key (decision 8).
//! - Space switches a field between its own value and following: own to
//!   following remembers the value, following to own flips a toggle or puts
//!   the remembered value back, and opens the editor otherwise.
//! - Actions: the one-field reset, the whole-page reset and Apply to games
//!   with their own value, each whole-page change behind a confirmation that
//!   opens on Cancel (decision 4), and Preview launch, which is read only.
//! - The VKD3D notice: when VKD3D heap resolves On for a game, Go's
//!   compatibility line shows under that row. It is resolved on a worker,
//!   so it appears when the answer arrives.
//!
//! Every change shows in the same frame and is saved through the write
//! queue ([`AppState::save`] with a [`Write::Profile`]). The page keeps two
//! copies of each profile: what it shows and what is known to be on disk.
//! A save that fails puts the stored value back, unless a newer save for
//! the same field is still on its way, and says why.
//!
//! Every profile is read once at start on a worker ([`Cmd::Load`]) and
//! again after each rescan. A scope whose file did not load or migrate, or
//! that a newer spela wrote, is shown and never edited.
//!
//! The DLLs section is [`game_dlls`]: the game's DLL versions with the
//! Catalog's stale marks, the deny list line, and Install, Update and
//! Restore, which are in Actions too, before the deny list rows.

use std::collections::{BTreeMap, BTreeSet};

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::games::Game;
use spela_core::profile::field::{Editor as EditorKind, FieldError, Rule};
use spela_core::profile::resolve::{resolve, resolve_all_games};
use spela_core::profile::{
    FIELDS, Field, FieldKey, FollowOutcome, Kind, OwnValues, Resolved, Scope, Section, Value, words,
};

use super::{
    Action as AnyAction, Owner, Popup, Row, ScreenAction, confirm, denylist, dlls, game_dlls,
    library, preview,
};
use crate::editor::{EditorInput, EditorOutcome};
use crate::event::{Cmd as AnyCmd, Write};
use crate::hit::{HitMap, HitTarget};
use crate::keymap::{Binding, Key};
use crate::layout::{Pane, PaneArea};
use crate::paint::Painter;
use crate::picker::{Choice, Picker, PickerInput, PickerOutcome};
use crate::state::{AppState, Level};
use crate::text::{width, wrap};
use crate::theme::Role;

/// The width of the value column, as Go's `%-12s`.
const VALUE_W: usize = 12;
/// The widest editor box, as in the mockup.
const EDITOR_W: u16 = 56;

/// What Detail shows of a game: its profile, or its DLLs.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Aspect {
    /// The profile page.
    #[default]
    Profile,
    /// The game's DLLs section.
    Dlls,
}

/// One profile as the page holds it.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Layer {
    /// What the page shows: the stored values plus every change not yet
    /// answered.
    pub shown: OwnValues,
    /// What is known to be on disk.
    pub stored: OwnValues,
    /// A newer spela wrote the file, so it is never written.
    pub read_only: bool,
}

/// Every profile, as loaded.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Profiles {
    /// The All games profile.
    pub all_games: Layer,
    /// Games with a profile file, and games changed since the load.
    pub games: BTreeMap<u64, Layer>,
    /// Profiles whose file does not load, with the error.
    pub broken: BTreeMap<Scope, String>,
}

/// One game profile the load read.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct LoadedGame {
    /// The app id.
    pub app_id: u64,
    /// What the game sets itself.
    pub values: OwnValues,
    /// A newer spela wrote it.
    pub read_only: bool,
}

/// What the load job read.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Loaded {
    /// The All games values and whether the file is read only, or `None`
    /// when there is no file.
    pub all_games: Option<(OwnValues, bool)>,
    /// The game profiles that load.
    pub games: Vec<LoadedGame>,
    /// The profiles that do not, with the error.
    pub broken: Vec<(Scope, String)>,
}

/// The VKD3D notice of one game.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Notice {
    /// Asked for, by request.
    Pending(u64),
    /// The line to show, or `None` when all is well.
    Ready(Option<String>),
}

/// A save on its way.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Pending {
    /// The profile.
    pub scope: Scope,
    /// The changes, by field: `None` follows.
    pub changes: BTreeMap<FieldKey, Option<Value>>,
    /// The status line's words, such as `HDR On`.
    pub summary: String,
    /// The Apply to games with their own value it belongs to.
    pub group: Option<u64>,
}

/// An Apply to games with their own value on its way: one save per game.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Group {
    /// The field's label.
    pub label: String,
    /// The tokens not yet answered.
    pub waiting: BTreeSet<u64>,
    /// What happened so far.
    pub outcome: FollowOutcome,
}

/// The editor for a number or a path, under its field: the shared
/// [`crate::editor::Editor`] with the profile and the field it edits.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct TextEditor {
    /// The profile it edits.
    pub scope: Scope,
    /// The field.
    pub key: FieldKey,
    /// The editor.
    pub editor: crate::editor::Editor,
}

/// A picker under its field, with the value each choice saves.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PickEditor {
    /// The profile it edits.
    pub scope: Scope,
    /// The field.
    pub key: FieldKey,
    /// The picker.
    pub picker: Picker,
    /// Each choice's value: `None` follows.
    pub values: Vec<Option<Value>>,
}

/// An open editor.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Editor {
    /// A picker.
    Pick(PickEditor),
    /// A text editor.
    Text(TextEditor),
}

impl Editor {
    /// The profile it edits.
    pub fn scope(&self) -> Scope {
        match self {
            Editor::Pick(e) => e.scope,
            Editor::Text(e) => e.scope,
        }
    }

    /// The field it edits.
    pub fn key(&self) -> FieldKey {
        match self {
            Editor::Pick(e) => e.key,
            Editor::Text(e) => e.key,
        }
    }
}

/// The profile page's state.
#[derive(Clone, Debug)]
pub struct State {
    /// Every profile, once loaded.
    pub profiles: Option<Profiles>,
    /// Why the load failed.
    pub load_error: Option<String>,
    /// The load on its way.
    pub load_request: Option<u64>,
    /// The field under the cursor. It stays when the game changes, as in
    /// Go.
    pub field: FieldKey,
    /// Profile or DLLs. It stays when the game changes.
    pub aspect: Aspect,
    /// The open editor.
    pub editor: Option<Editor>,
    /// The own value Space took away, so Space can put it back.
    pub last_own: BTreeMap<(Scope, FieldKey), Value>,
    /// Saves on their way, by token.
    pub pending: BTreeMap<u64, Pending>,
    /// Apply to games with their own value runs, by id.
    pub groups: BTreeMap<u64, Group>,
    /// The last failed save of each scope, until its next change.
    pub save_errors: BTreeMap<Scope, String>,
    /// The VKD3D notice of each game.
    pub notices: BTreeMap<u64, Notice>,
    /// The governors sysfs lists, or why they could not be read.
    pub governors: Option<Result<Vec<String>, String>>,
    /// The governor read on its way.
    pub governors_request: Option<u64>,
    /// Saves answered so far. A load that a save overtook is read again.
    pub saves_done: u64,
    /// [`State::saves_done`] when the load on its way was asked for.
    pub load_at: u64,
}

impl Default for State {
    fn default() -> State {
        State {
            profiles: None,
            load_error: None,
            load_request: None,
            field: FIELDS[0].id,
            aspect: Aspect::Profile,
            editor: None,
            last_own: BTreeMap::new(),
            pending: BTreeMap::new(),
            groups: BTreeMap::new(),
            save_errors: BTreeMap::new(),
            notices: BTreeMap::new(),
            governors: None,
            governors_request: None,
            saves_done: 0,
            load_at: 0,
        }
    }
}

/// What the profile page's keys and rows run.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Action {
    /// The previous field.
    Up,
    /// The next field.
    Down,
    /// Five fields up.
    PageUp,
    /// Five fields down.
    PageDown,
    /// The first field.
    Home,
    /// The last field.
    End,
    /// The first field of the previous section, or the profile from DLLs.
    PrevSection,
    /// The first field of the next section, or DLLs from CPU.
    NextSection,
    /// A click on a section in the strip.
    GoSection(Section),
    /// A click on DLLs in the strip.
    GoDlls,
    /// A click on a field row.
    SelectField(FieldKey),
    /// Enter: edit the field.
    Edit,
    /// Space: own value or following.
    Toggle,
    /// Follow All games, or Use system default, for the field.
    Reset,
    /// Asks before every field of the page follows.
    ResetAll,
    /// Every field of this scope follows, after the confirmation.
    ResetAllConfirmed(Scope),
    /// Asks before the games that set the field follow All games.
    ApplyToOwn,
    /// The games that set this field follow All games, after the
    /// confirmation.
    ApplyToOwnConfirmed(FieldKey),
    /// Opens Preview launch for the game.
    Preview,
    /// A picker key.
    Pick(PickerInput),
    /// A key of the text editor.
    Text(EditorInput),
}

/// The profile page's worker results.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Msg {
    /// Every profile, read.
    Loaded {
        /// The request this answers.
        request: u64,
        /// What was read, or why the folder could not be read.
        result: Result<Loaded, String>,
    },
    /// A game's VKD3D notice.
    Notice {
        /// The request this answers.
        request: u64,
        /// The game.
        app_id: u64,
        /// Go's line, or `None` when all is well.
        notice: Option<String>,
    },
    /// The governors sysfs lists.
    Governors {
        /// The request this answers.
        request: u64,
        /// The governors, or why they could not be read.
        result: Result<Vec<String>, String>,
    },
}

/// The profile page's requests to the shell, run in the binary's
/// `tui/exec/profile.rs`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Cmd {
    /// Read every profile.
    Load {
        /// The request id the result carries.
        request: u64,
    },
    /// Resolve a game's VKD3D notice: its Proton build and the driver.
    Notice {
        /// The request id the result carries.
        request: u64,
        /// The game.
        app_id: u64,
    },
    /// Read the governors sysfs lists.
    Governors {
        /// The request id the result carries.
        request: u64,
    },
}

fn act(a: Action) -> AnyAction {
    AnyAction::Screen(ScreenAction::Profile(a))
}

fn pick(input: PickerInput) -> AnyAction {
    act(Action::Pick(input))
}

fn pick_typed(c: char) -> AnyAction {
    act(Action::Pick(PickerInput::Type(c)))
}

fn text_typed(c: char) -> AnyAction {
    act(Action::Text(EditorInput::Type(c)))
}

fn text_key(input: EditorInput) -> AnyAction {
    act(Action::Text(input))
}

// Reading the page.

fn all_games_scope(scope: Scope) -> bool {
    scope == Scope::AllGames
}

/// The scope the page shows.
pub fn scope(state: &AppState) -> Option<Scope> {
    library::selected_scope(state)
}

/// The aspect in effect: All games has no DLLs section.
pub fn aspect(state: &AppState) -> Aspect {
    match scope(state) {
        Some(Scope::Game(_)) => state.profile.aspect,
        _ => Aspect::Profile,
    }
}

fn layer(state: &AppState, scope: Scope) -> Option<&Layer> {
    let p = state.profile.profiles.as_ref()?;
    match scope {
        Scope::AllGames => Some(&p.all_games),
        Scope::Game(id) => p.games.get(&id),
    }
}

/// One field of a scope, resolved from what the page shows.
pub fn resolved(state: &AppState, scope: Scope, key: FieldKey) -> Option<Resolved> {
    let p = state.profile.profiles.as_ref()?;
    Some(match scope {
        Scope::AllGames => resolve_all_games(Some(&p.all_games.shown), key),
        Scope::Game(id) => resolve(
            p.games.get(&id).map(|l| &l.shown),
            Some(&p.all_games.shown),
            key,
        ),
    })
}

/// Why a scope cannot be edited now, if it cannot.
pub fn editable(state: &AppState, scope: Scope) -> Result<(), String> {
    if state.profile.profiles.is_none() {
        return Err("loading profiles".into());
    }
    if broken(state, scope).is_some() {
        return Err("the profile did not load".into());
    }
    if layer(state, scope).is_some_and(|l| l.read_only) {
        return Err("written by a newer spela".into());
    }
    Ok(())
}

/// The error of a scope whose profile did not load or migrate.
pub fn broken(state: &AppState, scope: Scope) -> Option<&str> {
    state
        .broken_profiles
        .get(&scope)
        .or_else(|| state.profile.profiles.as_ref()?.broken.get(&scope))
        .map(String::as_str)
}

fn game_name(state: &AppState, scope: Scope) -> String {
    match scope {
        Scope::AllGames => words::ALL_GAMES.to_owned(),
        Scope::Game(id) => state
            .games
            .get(id)
            .map_or_else(|| format!("App ID {id}"), |g| g.name.clone()),
    }
}

/// The value a field would have if it followed: the All games value on a
/// game, the system default on All games.
fn follow_text(state: &AppState, scope: Scope, key: FieldKey) -> String {
    let field = key.field();
    let Some(p) = state.profile.profiles.as_ref() else {
        return String::new();
    };
    let r = match scope {
        Scope::AllGames => resolve_all_games(None, key),
        Scope::Game(_) => resolve(None, Some(&p.all_games.shown), key),
    };
    words::value_text(field, &r)
}

fn dotted(state: &AppState, text: &str) -> String {
    text.replace('\u{b7}', state.glyphs.dot)
}

/// The games that set `key` themselves, by app id and name, sorted by name.
fn own_value_games(state: &AppState, key: FieldKey) -> Vec<(u64, String)> {
    let Some(p) = state.profile.profiles.as_ref() else {
        return Vec::new();
    };
    let mut out: Vec<(u64, String)> = p
        .games
        .iter()
        .filter(|(_, l)| l.shown.contains(key))
        .map(|(id, _)| (*id, game_name(state, Scope::Game(*id))))
        .collect();
    out.sort_by(|a, b| a.1.cmp(&b.1).then(a.0.cmp(&b.0)));
    out
}

/// The fields in page order.
fn keys() -> impl Iterator<Item = FieldKey> {
    FieldKey::all()
}

fn index_of(key: FieldKey) -> usize {
    key as usize
}

fn key_at(i: usize) -> FieldKey {
    FIELDS[i.min(FIELDS.len() - 1)].id
}

fn first_of(section: Section) -> FieldKey {
    section.fields().next().map_or(FIELDS[0].id, |f| f.id)
}

fn section_index(section: Section) -> usize {
    Section::ALL.iter().position(|s| *s == section).unwrap_or(0)
}

/// Whether Left can go somewhere, or why not.
fn can_prev(state: &AppState) -> Result<(), String> {
    match aspect(state) {
        Aspect::Dlls => Ok(()),
        Aspect::Profile if section_index(state.profile.field.field().section) > 0 => Ok(()),
        Aspect::Profile => Err("first section".into()),
    }
}

/// Whether Right can go somewhere, or why not.
fn can_next(state: &AppState) -> Result<(), String> {
    let last = section_index(state.profile.field.field().section) + 1 >= Section::ALL.len();
    match (aspect(state), scope(state)) {
        (Aspect::Dlls, _) => Err("last section".into()),
        (Aspect::Profile, Some(Scope::Game(_))) => Ok(()),
        (Aspect::Profile, _) if last => Err("last section".into()),
        (Aspect::Profile, _) => Ok(()),
    }
}

// The screen contract.

/// The pane title: the game's name, or `All games`.
pub fn title(state: &AppState, _pane: Pane) -> String {
    match scope(state) {
        Some(Scope::AllGames) => words::ALL_GAMES.to_owned(),
        Some(Scope::Game(_)) => library::selected_game(state)
            .map(|g| g.name.clone())
            .unwrap_or_default(),
        None => "Profile".to_owned(),
    }
}

/// The editor open for the scope the page shows, if any.
fn open_editor(state: &AppState) -> Option<&Editor> {
    let editor = state.profile.editor.as_ref()?;
    (Some(editor.scope()) == scope(state) && aspect(state) == Aspect::Profile).then_some(editor)
}

/// Whether an editor or picker is open.
pub fn editing(state: &AppState, _pane: Pane) -> bool {
    open_editor(state).is_some()
}

/// The page's keys.
pub fn bindings(state: &AppState, _pane: Pane) -> Vec<Binding> {
    if let Some(editor) = open_editor(state) {
        return match editor {
            Editor::Pick(e) => e.picker.bindings(pick, pick_typed),
            Editor::Text(e) => e.editor.bindings(text_key, text_typed),
        };
    }
    let Some(scope) = scope(state) else {
        return Vec::new();
    };
    let mut left = Binding::new(Key::LEFT, act(Action::PrevSection), "Section");
    let mut right = Binding::new(Key::RIGHT, act(Action::NextSection), "Next section");
    let (l, r) = (can_prev(state), can_next(state));
    if l.is_ok() {
        left = left.hint_as("Left/Right", 40);
    } else if r.is_ok() {
        right =
            Binding::new(Key::RIGHT, act(Action::NextSection), "Section").hint_as("Left/Right", 40);
    }
    let left = left.when(l);
    let right = right.when(r);
    if aspect(state) == Aspect::Dlls {
        let mut out = game_dlls::bindings(state);
        out.extend([left, right]);
        return out;
    }
    let loaded = if state.profile.profiles.is_some() {
        Ok(())
    } else {
        Err("loading profiles".to_owned())
    };
    let edit = editable(state, scope);
    let space_label = space_label(state, scope);
    vec![
        Binding::new(Key::UP, act(Action::Up), "Field")
            .hint_as("Up/Down", 100)
            .when(loaded.clone()),
        Binding::new(Key::DOWN, act(Action::Down), "Next field").when(loaded.clone()),
        left,
        right,
        Binding::new(Key::ENTER, act(Action::Edit), "Edit value")
            .hint(90)
            .when(edit.clone()),
        Binding::new(Key::SPACE, act(Action::Toggle), space_label)
            .hint(80)
            .when(edit),
        Binding::new(Key::PAGE_UP, act(Action::PageUp), "Page up").when(loaded.clone()),
        Binding::new(Key::PAGE_DOWN, act(Action::PageDown), "Page down").when(loaded.clone()),
        Binding::new(Key::HOME, act(Action::Home), "First field").when(loaded.clone()),
        Binding::new(Key::END, act(Action::End), "Last field").when(loaded),
    ]
}

/// What Space does on the focused field, in Go's words.
fn space_label(state: &AppState, scope: Scope) -> &'static str {
    let own = resolved(state, scope, state.profile.field).is_some_and(|r| r.is_own());
    match (own, all_games_scope(scope)) {
        (true, false) => words::FOLLOW_ALL_GAMES,
        (true, true) => words::USE_SYSTEM_DEFAULT,
        (false, false) => words::SET_FOR_THIS_GAME,
        (false, true) => words::SET_HERE,
    }
}

fn preview_row(state: &AppState) -> Row {
    match (scope(state), library::selected_game(state)) {
        (Some(Scope::Game(_)), Some(game)) => Row::new(
            format!("Preview launch: {}", game.name),
            act(Action::Preview),
        ),
        _ => Row::new("Preview launch", act(Action::Preview)).when(Err("select a game".into())),
    }
}

/// The Actions rows of the page.
pub fn actions(state: &AppState, pane: Pane) -> Vec<Row> {
    if pane != Pane::Detail {
        return Vec::new();
    }
    page_rows(state)
}

fn page_rows(state: &AppState) -> Vec<Row> {
    let Some(scope) = scope(state) else {
        return Vec::new();
    };
    if aspect(state) == Aspect::Dlls {
        // The DLL rows, then Preview launch, then the deny list rows
        // (terminal-ux.md, denylist).
        let mut rows = game_dlls::rows(state);
        rows.push(preview_row(state));
        rows.extend(denylist::actions(state, Pane::Detail));
        return rows;
    }
    let all = all_games_scope(scope);
    let edit = editable(state, scope);
    let own = resolved(state, scope, state.profile.field).is_some_and(|r| r.is_own());
    let reset = edit.clone().and_then(|()| {
        if own {
            Ok(())
        } else {
            Err("nothing to clear".to_owned())
        }
    });
    let mut rows = vec![
        Row::new(words::reset_action(all), act(Action::Reset)).when(reset),
        Row::new(words::clear_all_action(all), act(Action::ResetAll)).when(edit.clone()),
    ];
    if all {
        rows.push(
            Row::new(
                words::APPLY_TO_GAMES_WITH_OWN_VALUE,
                act(Action::ApplyToOwn),
            )
            .when(apply_available(state)),
        );
    }
    rows.push(
        Row::new("Edit value", act(Action::Edit))
            .key(Key::ENTER)
            .when(edit),
    );
    rows.push(preview_row(state));
    rows
}

/// Whether Apply to games with their own value can run: on All games,
/// with no save on its way, when a game sets the field itself.
fn apply_available(state: &AppState) -> Result<(), String> {
    match scope(state) {
        Some(Scope::AllGames) => {}
        _ => return Err(format!("open {}", words::ALL_GAMES)),
    }
    editable(state, Scope::AllGames)?;
    if !state.profile.pending.is_empty() {
        return Err("saving".into());
    }
    if own_value_games(state, state.profile.field).is_empty() {
        return Err("no game has its own value".into());
    }
    Ok(())
}

/// The palette rows: Preview launch, and the whole-page reset of the page
/// Library shows. The one-field rows stay in Actions, where the focused
/// field is in view.
pub fn palette(state: &AppState) -> Vec<Row> {
    let mut rows = vec![preview_row(state)];
    if let Some(scope) = scope(state) {
        let label = format!(
            "{}: {}",
            words::clear_all_action(all_games_scope(scope)),
            game_name(state, scope)
        );
        rows.push(Row::new(label, act(Action::ResetAll)).when(editable(state, scope)));
    }
    rows
}

// Changing the page.

/// The first commands: read every profile and the governors.
pub fn start(state: &mut AppState) -> Vec<AnyCmd> {
    let mut cmds = load(state);
    cmds.extend(read_governors(state));
    cmds
}

fn load(state: &mut AppState) -> Vec<AnyCmd> {
    let request = state.request();
    state.profile.load_request = Some(request);
    state.profile.load_at = state.profile.saves_done;
    vec![AnyCmd::Profile(Cmd::Load { request })]
}

fn read_governors(state: &mut AppState) -> Vec<AnyCmd> {
    if state.profile.governors_request.is_some() {
        return Vec::new();
    }
    let request = state.request();
    state.profile.governors_request = Some(request);
    vec![AnyCmd::Profile(Cmd::Governors { request })]
}

/// After a rescan: read the profiles again and resolve the notices anew.
pub fn rescanned(state: &mut AppState) -> Vec<AnyCmd> {
    state.profile.notices.clear();
    load(state)
}

/// After the scope may have changed: an editor for another scope closes,
/// and a game whose VKD3D heap resolves On asks for its notice.
pub fn follow_selection(state: &mut AppState) -> Vec<AnyCmd> {
    let shown = scope(state);
    if state
        .profile
        .editor
        .as_ref()
        .is_some_and(|e| Some(e.scope()) != shown)
    {
        state.profile.editor = None;
        state.dirty = true;
    }
    if !game_dlls::editing(state) && state.game_dlls.chooser.take().is_some() {
        state.dirty = true;
    }
    let mut cmds = want_notice(state);
    cmds.extend(crate::dll_jobs::want_backup(state));
    cmds
}

fn want_notice(state: &mut AppState) -> Vec<AnyCmd> {
    let Some(Scope::Game(app_id)) = scope(state) else {
        return Vec::new();
    };
    if state.profile.notices.contains_key(&app_id) {
        return Vec::new();
    }
    let on = resolved(state, Scope::Game(app_id), FieldKey::ProtonVkd3dHeap)
        .is_some_and(|r| r.value == Value::Bool(true));
    if !on {
        return Vec::new();
    }
    let request = state.request();
    state
        .profile
        .notices
        .insert(app_id, Notice::Pending(request));
    vec![AnyCmd::Profile(Cmd::Notice { request, app_id })]
}

fn move_to(state: &mut AppState, key: FieldKey) {
    // A click on another row closes the editor of this one.
    if state
        .profile
        .editor
        .as_ref()
        .is_some_and(|e| e.key() != key)
    {
        state.profile.editor = None;
    }
    state.profile.field = key;
    state.profile.aspect = Aspect::Profile;
    state.dirty = true;
}

/// Runs an action.
pub fn update(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    state.dirty = true;
    let mut cmds = run(state, action);
    cmds.extend(want_notice(state));
    cmds.extend(crate::dll_jobs::want_backup(state));
    cmds
}

fn run(state: &mut AppState, action: Action) -> Vec<AnyCmd> {
    // A page action that arrives while an editor is open, from a click or
    // a right-click Actions row, closes the editor first.
    let editor_action = matches!(
        action,
        Action::Pick(_) | Action::Text(_) | Action::SelectField(_)
    );
    if !editor_action {
        state.profile.editor = None;
    }
    let i = index_of(state.profile.field);
    let last = FIELDS.len() - 1;
    match action {
        Action::Up => move_to(state, key_at(i.saturating_sub(1))),
        Action::Down => move_to(state, key_at((i + 1).min(last))),
        Action::PageUp => move_to(state, key_at(i.saturating_sub(5))),
        Action::PageDown => move_to(state, key_at((i + 5).min(last))),
        Action::Home => move_to(state, key_at(0)),
        Action::End => move_to(state, key_at(last)),
        Action::PrevSection => match aspect(state) {
            Aspect::Dlls => move_to(state, first_of(Section::Cpu)),
            Aspect::Profile => {
                let s = section_index(state.profile.field.field().section);
                if s > 0 {
                    move_to(state, first_of(Section::ALL[s - 1]));
                }
            }
        },
        Action::NextSection => {
            if aspect(state) == Aspect::Profile {
                let s = section_index(state.profile.field.field().section);
                if s + 1 < Section::ALL.len() {
                    move_to(state, first_of(Section::ALL[s + 1]));
                } else if matches!(scope(state), Some(Scope::Game(_))) {
                    state.profile.aspect = Aspect::Dlls;
                    state.profile.editor = None;
                }
            }
        }
        Action::GoSection(section) => {
            state.focus = Pane::Detail;
            move_to(state, first_of(section));
        }
        Action::GoDlls => {
            state.focus = Pane::Detail;
            if matches!(scope(state), Some(Scope::Game(_))) {
                state.profile.aspect = Aspect::Dlls;
                state.profile.editor = None;
            }
        }
        Action::SelectField(key) => move_to(state, key),
        Action::Edit => return begin_edit(state),
        Action::Toggle => return toggle(state),
        Action::Reset => return reset(state),
        Action::ResetAll => ask_reset_all(state),
        Action::ResetAllConfirmed(scope) => return reset_all(state, scope),
        Action::ApplyToOwn => ask_apply_to_own(state),
        Action::ApplyToOwnConfirmed(key) => return apply_to_own(state, key),
        Action::Preview => return preview(state),
        Action::Pick(input) => return pick_input(state, input),
        Action::Text(input) => return text_input(state, input),
    }
    Vec::new()
}

/// Every own value of `scope` the change would set, refusing a value the
/// field cannot hold.
fn apply_changes(
    values: &mut OwnValues,
    changes: &BTreeMap<FieldKey, Option<Value>>,
) -> Result<(), String> {
    for (key, value) in changes {
        match value {
            Some(v) => {
                values.set(*key, v.clone()).map_err(|e| e.to_string())?;
            }
            None => {
                values.follow(*key);
            }
        }
    }
    Ok(())
}

fn layer_mut(state: &mut AppState, scope: Scope) -> Option<&mut Layer> {
    let p = state.profile.profiles.as_mut()?;
    Some(match scope {
        Scope::AllGames => &mut p.all_games,
        Scope::Game(id) => p.games.entry(id).or_default(),
    })
}

/// Shows a change at once and queues its save. Nothing is saved when the
/// change changes nothing.
fn change(
    state: &mut AppState,
    scope: Scope,
    changes: BTreeMap<FieldKey, Option<Value>>,
    summary: String,
    group: Option<u64>,
) -> Vec<AnyCmd> {
    if let Err(reason) = editable(state, scope) {
        state.say(Level::Error, format!("Not saved: {reason}"));
        return Vec::new();
    }
    let Some(layer) = layer_mut(state, scope) else {
        return Vec::new();
    };
    let mut next = layer.shown.clone();
    if let Err(e) = apply_changes(&mut next, &changes) {
        state.say(Level::Error, format!("Not saved: {summary}: {e}"));
        return Vec::new();
    }
    if next == layer.shown {
        return Vec::new();
    }
    layer.shown = next;
    state.profile.save_errors.remove(&scope);
    let name = match scope {
        Scope::Game(id) => state
            .games
            .get(id)
            .map(|g| g.name.clone())
            .unwrap_or_default(),
        Scope::AllGames => String::new(),
    };
    let (token, cmd) = state.save(
        Owner::Profile,
        Write::Profile {
            scope,
            name,
            changes: changes.clone(),
        },
    );
    state.profile.pending.insert(
        token,
        Pending {
            scope,
            changes,
            summary,
            group,
        },
    );
    if let Some(g) = group.and_then(|g| state.profile.groups.get_mut(&g)) {
        g.waiting.insert(token);
    }
    state.dirty = true;
    vec![cmd]
}

/// The status line's words for one field after a change.
fn summary(state: &AppState, scope: Scope, key: FieldKey, value: Option<&Value>) -> String {
    let field = key.field();
    let mut text = match value {
        Some(v) => format!("{} {}", field.label, words::display_value(field, v)),
        None if all_games_scope(scope) => format!("{} uses the system default", field.label),
        None => format!("{} follows {}", field.label, words::ALL_GAMES),
    };
    if all_games_scope(scope) {
        text.push_str(&format!(" ({})", words::ALL_GAMES));
    }
    let _ = state;
    text
}

fn set_field(
    state: &mut AppState,
    scope: Scope,
    key: FieldKey,
    value: Option<Value>,
) -> Vec<AnyCmd> {
    let text = summary(state, scope, key, value.as_ref());
    change(state, scope, BTreeMap::from([(key, value)]), text, None)
}

fn toggle(state: &mut AppState) -> Vec<AnyCmd> {
    let Some(scope) = scope(state) else {
        return Vec::new();
    };
    if editable(state, scope).is_err() {
        return Vec::new();
    }
    let key = state.profile.field;
    let Some(r) = resolved(state, scope, key) else {
        return Vec::new();
    };
    if let Some(own) = r.own.clone() {
        state.profile.last_own.insert((scope, key), own);
        return set_field(state, scope, key, None);
    }
    let value = match (key.field().kind, &r.value) {
        (Kind::Bool, Value::Bool(b)) => Some(Value::Bool(!b)),
        _ => state.profile.last_own.get(&(scope, key)).cloned(),
    };
    match value {
        Some(v) => set_field(state, scope, key, Some(v)),
        None => begin_edit(state),
    }
}

fn reset(state: &mut AppState) -> Vec<AnyCmd> {
    let Some(scope) = scope(state) else {
        return Vec::new();
    };
    let key = state.profile.field;
    if !resolved(state, scope, key).is_some_and(|r| r.is_own()) {
        return Vec::new();
    }
    set_field(state, scope, key, None)
}

fn own_keys(state: &AppState, scope: Scope) -> Vec<FieldKey> {
    keys()
        .filter(|k| resolved(state, scope, *k).is_some_and(|r| r.is_own()))
        .collect()
}

fn ask_reset_all(state: &mut AppState) {
    let Some(scope) = scope(state) else {
        return;
    };
    if editable(state, scope).is_err() {
        return;
    }
    let all = all_games_scope(scope);
    let count = own_keys(state, scope).len();
    let title = words::clear_all_action(all);
    let mut body = Vec::new();
    let name = game_name(state, scope);
    body.push(vec![(name, Role::Strong)]);
    body.push(Vec::new());
    for line in wrap(&words::clear_all_body(all, count), 80) {
        body.push(vec![(line, Role::Text)]);
    }
    state.confirm(
        confirm::State::new(title, body, act(Action::ResetAllConfirmed(scope)))
            .confirm_label(title),
    );
}

fn reset_all(state: &mut AppState, scope: Scope) -> Vec<AnyCmd> {
    let changes: BTreeMap<FieldKey, Option<Value>> = own_keys(state, scope)
        .into_iter()
        .map(|k| (k, None))
        .collect();
    if changes.is_empty() {
        return Vec::new();
    }
    let text = if all_games_scope(scope) {
        format!("All settings use the system default ({})", words::ALL_GAMES)
    } else {
        format!("All settings follow {}", words::ALL_GAMES)
    };
    for key in changes.keys() {
        state.profile.last_own.remove(&(scope, *key));
    }
    change(state, scope, changes, text, None)
}

fn ask_apply_to_own(state: &mut AppState) {
    if apply_available(state).is_err() {
        return;
    }
    let key = state.profile.field;
    let field = key.field();
    let names: Vec<String> = own_value_games(state, key)
        .into_iter()
        .map(|(_, n)| n)
        .collect();
    let value = resolved(state, Scope::AllGames, key)
        .map(|r| words::value_text(field, &r))
        .unwrap_or_default();
    let text = words::apply_to_own_value_body(field.label, &value, &names);
    let body = wrap(&text, 80)
        .into_iter()
        .map(|l| vec![(l, Role::Text)])
        .collect();
    state.confirm(
        confirm::State::new(
            words::APPLY_TO_GAMES_WITH_OWN_VALUE,
            body,
            act(Action::ApplyToOwnConfirmed(key)),
        )
        .confirm_label(words::APPLY_TO_GAMES_WITH_OWN_VALUE),
    );
}

fn apply_to_own(state: &mut AppState, key: FieldKey) -> Vec<AnyCmd> {
    let games = own_value_games(state, key);
    if games.is_empty() {
        return Vec::new();
    }
    let label = key.field().label.to_owned();
    let id = state.request();
    state.profile.groups.insert(
        id,
        Group {
            label: label.clone(),
            ..Group::default()
        },
    );
    let mut cmds = Vec::new();
    for (app_id, _) in games {
        let scope = Scope::Game(app_id);
        let text = summary(state, scope, key, None);
        cmds.extend(change(
            state,
            scope,
            BTreeMap::from([(key, None)]),
            text,
            Some(id),
        ));
    }
    if state
        .profile
        .groups
        .get(&id)
        .is_some_and(|g| g.waiting.is_empty())
    {
        state.profile.groups.remove(&id);
    }
    cmds
}

fn preview(state: &mut AppState) -> Vec<AnyCmd> {
    let Some(game) = library::selected_game(state) else {
        return Vec::new();
    };
    let (app_id, name) = (game.app_id, game.name.clone());
    let request = state.request();
    state.open(Popup::Preview(preview::State {
        request,
        name,
        summary: None,
        scroll: 0,
    }));
    vec![AnyCmd::Preview { request, app_id }]
}

// Editors.

/// The choices of a field that edits with a picker, with the value each
/// saves; `None` for a field that edits as text.
fn choices(state: &AppState, field: &Field) -> Option<Vec<(String, Value)>> {
    let word = |v: &Value| words::display_value(field, v);
    let out: Vec<Value> = match (field.kind, field.rule) {
        (Kind::Bool, _) => vec![Value::Bool(true), Value::Bool(false)],
        // SMT's stored none ("unchanged", decision 65) is offered only
        // where a scope already stores it, as a stored value below.
        (Kind::OptionalBool, _) => vec![
            Value::OptionalBool(Some(true)),
            Value::OptionalBool(Some(false)),
        ],
        (Kind::Int, Rule::OneOf(list)) => list
            .iter()
            .filter_map(|s| s.parse().ok())
            .map(Value::Int)
            .collect(),
        (Kind::Text, Rule::OneOf(list)) => list
            .iter()
            // VRR has `unset` for that, as in Go.
            .filter(|s| !(s.is_empty() && field.id == FieldKey::GpuVrr))
            .map(|s| Value::Text((*s).to_owned()))
            .collect(),
        (Kind::Text, Rule::SystemList) => {
            let mut v = vec![Value::Text(String::new())];
            if let Some(Ok(governors)) = &state.profile.governors {
                v.extend(governors.iter().map(|g| Value::Text(g.clone())));
            }
            v
        }
        _ => return None,
    };
    Some(out.into_iter().map(|v| (word(&v), v)).collect())
}

fn begin_edit(state: &mut AppState) -> Vec<AnyCmd> {
    let Some(scope) = scope(state) else {
        return Vec::new();
    };
    if editable(state, scope).is_err() || aspect(state) != Aspect::Profile {
        return Vec::new();
    }
    let key = state.profile.field;
    let field = key.field();
    let Some(r) = resolved(state, scope, key) else {
        return Vec::new();
    };
    state.focus = Pane::Detail;
    // One input at a time: an editor closes the Find row.
    library::stop_finding(state);
    let before = words::value_text(field, &r);
    let mut cmds = Vec::new();
    if let Some(list) = choices(state, field) {
        let follow = format!(
            "{} ({})",
            words::reset_action(all_games_scope(scope)),
            follow_text(state, scope, key)
        );
        let mut labels = vec![Choice {
            label: follow,
            current: r.own.is_none(),
        }];
        let mut values = vec![None];
        for (label, value) in list {
            labels.push(Choice {
                current: r.own.as_ref() == Some(&value),
                label,
            });
            values.push(Some(value));
        }
        // A stored value the system does not list stays a choice. SMT's
        // stored none reads unchanged (decision 65).
        if let Some(own) = &r.own
            && !values.iter().any(|v| v.as_ref() == Some(own))
        {
            let label = match own {
                Value::OptionalBool(None) => words::UNCHANGED.to_owned(),
                other => words::display_value(field, other),
            };
            labels.push(Choice {
                label,
                current: true,
            });
            values.push(Some(own.clone()));
        }
        if field.rule == Rule::SystemList && !matches!(state.profile.governors, Some(Ok(_))) {
            state.profile.governors_request = None;
            cmds.extend(read_governors(state));
        }
        let picker = Picker::new(format!("Edit {}", field.label), Some(before), labels);
        state.profile.editor = Some(Editor::Pick(PickEditor {
            scope,
            key,
            picker,
            values,
        }));
    } else {
        // A "none" row opens empty (decision 8).
        let prefill = if before == words::NONE {
            String::new()
        } else {
            r.value.canonical()
        };
        let mut editor =
            crate::editor::Editor::new(format!("Edit {}", field.label), Some(before), &prefill);
        let hint = words::number_hint(field);
        if !hint.is_empty() {
            editor = editor.hint(format!("{hint} Typing replaces the value."));
        }
        if field.editor == EditorKind::Integer {
            editor = editor.replace_on_first_key();
        }
        state.profile.editor = Some(Editor::Text(TextEditor { scope, key, editor }));
    }
    state.dirty = true;
    cmds
}

fn pick_input(state: &mut AppState, input: PickerInput) -> Vec<AnyCmd> {
    let Some(Editor::Pick(e)) = &mut state.profile.editor else {
        return Vec::new();
    };
    match e.picker.handle(input) {
        PickerOutcome::Moved => Vec::new(),
        PickerOutcome::Cancel => {
            state.profile.editor = None;
            Vec::new()
        }
        PickerOutcome::Apply(i) => {
            let (scope, key) = (e.scope, e.key);
            let value = e.values.get(i).cloned().flatten();
            state.profile.editor = None;
            set_field(state, scope, key, value)
        }
    }
}

fn text_input(state: &mut AppState, input: EditorInput) -> Vec<AnyCmd> {
    let Some(Editor::Text(e)) = &mut state.profile.editor else {
        return Vec::new();
    };
    let text = match e.editor.handle(input) {
        EditorOutcome::Moved => return Vec::new(),
        EditorOutcome::Cancel => {
            state.profile.editor = None;
            return Vec::new();
        }
        EditorOutcome::Apply(text) => text,
    };
    let (scope, key) = (e.scope, e.key);
    // An empty Apply on a row that read "none" leaves it so: the editor
    // opened empty there (decision 8), so nothing was typed.
    if text.trim().is_empty() && e.editor.before.as_deref() == Some(words::NONE) {
        state.profile.editor = None;
        return Vec::new();
    }
    match key.field().parse(&text) {
        Ok(value) => {
            state.profile.editor = None;
            set_field(state, scope, key, Some(value))
        }
        Err(err) => {
            // The editor stays open with focus on the input.
            e.editor.fail(edit_error(&err));
            Vec::new()
        }
    }
}

/// Why a typed value was refused, in Go's TUI words where Go has them:
/// `invalid integer: <raw>` (parity-inventory.md, TUI 17).
fn edit_error(err: &FieldError) -> String {
    match err {
        FieldError::InvalidInteger { raw, .. } => format!("invalid integer: {raw}"),
        other => other.to_string(),
    }
}

// Results.

/// Takes a worker result.
pub fn on_msg(state: &mut AppState, msg: Msg) -> Vec<AnyCmd> {
    state.dirty = true;
    match msg {
        Msg::Loaded { request, result } => {
            if state.profile.load_request != Some(request) {
                return Vec::new();
            }
            state.profile.load_request = None;
            // A save answered after the read began: the read may miss it.
            if state.profile.saves_done != state.profile.load_at {
                return load(state);
            }
            match result {
                Ok(loaded) => {
                    state.profile.load_error = None;
                    state.profile.profiles = Some(adopt(state, loaded));
                }
                Err(e) => {
                    state.profile.load_error = Some(e);
                }
            }
            want_notice(state)
        }
        Msg::Notice {
            request,
            app_id,
            notice,
        } => {
            if state.profile.notices.get(&app_id) == Some(&Notice::Pending(request)) {
                state.profile.notices.insert(app_id, Notice::Ready(notice));
            }
            Vec::new()
        }
        Msg::Governors { request, result } => {
            if state.profile.governors_request == Some(request) {
                state.profile.governors = Some(result);
                refresh_governor_picker(state);
            }
            Vec::new()
        }
    }
}

/// Builds the page's profiles from a load, keeping every change whose
/// save is still on its way.
fn adopt(state: &AppState, loaded: Loaded) -> Profiles {
    let mut p = Profiles::default();
    if let Some((values, read_only)) = loaded.all_games {
        p.all_games = Layer {
            shown: values.clone(),
            stored: values,
            read_only,
        };
    }
    for g in loaded.games {
        p.games.insert(
            g.app_id,
            Layer {
                shown: g.values.clone(),
                stored: g.values,
                read_only: g.read_only,
            },
        );
    }
    p.broken = loaded.broken.into_iter().collect();
    for pending in state.profile.pending.values() {
        let layer = match pending.scope {
            Scope::AllGames => &mut p.all_games,
            Scope::Game(id) => p.games.entry(id).or_default(),
        };
        let _ = apply_changes(&mut layer.shown, &pending.changes);
    }
    p
}

/// An open governor picker gains the governors once they are read.
fn refresh_governor_picker(state: &mut AppState) {
    let Some(Editor::Pick(e)) = &state.profile.editor else {
        return;
    };
    if e.key.field().rule != Rule::SystemList {
        return;
    }
    let (cursor, focus) = (e.picker.cursor, e.picker.focus);
    state.profile.editor = None;
    let _ = begin_edit(state);
    if let Some(Editor::Pick(e)) = &mut state.profile.editor {
        e.picker.cursor = cursor.min(e.picker.choices.len().saturating_sub(1));
        e.picker.focus = focus;
    }
}

/// Takes a save's result. A failure puts the stored value back, unless a
/// newer save for the same field is on its way.
pub fn saved(state: &mut AppState, token: u64, result: Result<(), String>) -> Vec<AnyCmd> {
    let Some(p) = state.profile.pending.remove(&token) else {
        return Vec::new();
    };
    state.dirty = true;
    state.profile.saves_done += 1;
    // All games saves never name a game: their summary already ends in
    // `(All games)`, as in Go.
    let shown = p.scope == Scope::AllGames || scope(state) == Some(p.scope);
    let prefix = if shown {
        String::new()
    } else {
        format!("{}: ", game_name(state, p.scope))
    };
    let name = game_name(state, p.scope);
    match &result {
        Ok(()) => {
            if let Some(layer) = layer_mut(state, p.scope) {
                let _ = apply_changes(&mut layer.stored, &p.changes);
            }
            if let Scope::Game(id) = p.scope
                && p.changes.values().any(Option::is_some)
            {
                state.library.profiles.insert(id);
            }
        }
        Err(e) => {
            let newer: BTreeSet<FieldKey> = state
                .profile
                .pending
                .values()
                .filter(|q| q.scope == p.scope)
                .flat_map(|q| q.changes.keys().copied())
                .collect();
            if let Some(layer) = layer_mut(state, p.scope) {
                for key in p.changes.keys().filter(|k| !newer.contains(k)) {
                    match layer.stored.get(*key).cloned() {
                        Some(v) => {
                            let _ = layer.shown.set(*key, v);
                        }
                        None => {
                            layer.shown.follow(*key);
                        }
                    }
                }
            }
            state.profile.save_errors.insert(p.scope, e.clone());
        }
    }
    if let Some(id) = p.group {
        group_done(state, id, token, name, result);
    } else {
        match result {
            Ok(()) => state.say(Level::Ok, format!("Saved: {prefix}{}", p.summary)),
            Err(e) => state.say(
                Level::Error,
                format!("Not saved: {prefix}{}: {e}", p.summary),
            ),
        }
    }
    want_notice(state)
}

fn group_done(state: &mut AppState, id: u64, token: u64, name: String, result: Result<(), String>) {
    let Some(g) = state.profile.groups.get_mut(&id) else {
        return;
    };
    g.waiting.remove(&token);
    match result {
        Ok(()) => g.outcome.followed.push(name),
        Err(e) => {
            g.outcome.failed.push(name.clone());
            g.outcome.errors.insert(name, e);
        }
    }
    if !g.waiting.is_empty() {
        return;
    }
    let Some(mut g) = state.profile.groups.remove(&id) else {
        return;
    };
    g.outcome.followed.sort();
    g.outcome.failed.sort();
    let text = words::follow_outcome_text(&g.label, &g.outcome);
    if g.outcome.failed.is_empty() {
        state.say(Level::Ok, format!("Saved: {text}"));
    } else {
        state.say(Level::Error, text);
    }
}

/// The mouse wheel over the page moves the field cursor three rows.
pub fn wheel(state: &mut AppState, _pane: Pane, down: bool) -> Vec<AnyCmd> {
    if open_editor(state).is_some() || aspect(state) != Aspect::Profile {
        return Vec::new();
    }
    let mut cmds = Vec::new();
    for _ in 0..3 {
        cmds.extend(update(state, if down { Action::Down } else { Action::Up }));
    }
    cmds
}

// Drawing.

/// One line of the page body.
#[derive(Clone, Debug, PartialEq)]
enum Line {
    Blank,
    Section(Section),
    Field(FieldKey),
    Note(String, Role),
    /// The open editor's box, which takes this many rows from here.
    Editor(u16),
    /// A row the editor box above takes.
    Taken,
}

fn label_width() -> usize {
    FIELDS.iter().map(|f| width(f.label)).max().unwrap_or(0)
}

/// The VKD3D notice line of a game, as Go words it: a warning keeps ⚠ in
/// the warning role, an info line loses Go's ⓘ and is muted.
fn notice_line(state: &AppState, app_id: u64) -> Option<(String, Role)> {
    let Some(Notice::Ready(Some(text))) = state.profile.notices.get(&app_id) else {
        return None;
    };
    if let Some(rest) = text.strip_prefix(spela_core::proton::WARN_GLYPH) {
        return Some((format!("{} {rest}", state.glyphs.warn), Role::Warn));
    }
    let rest = text
        .strip_prefix(spela_core::proton::INFO_GLYPH)
        .unwrap_or(text);
    Some((rest.to_owned(), Role::Muted))
}

/// The width of the editor box in a body `cw` cells wide, as in the
/// mockup: indented under its field, at most [`EDITOR_W`].
fn editor_width(cw: usize) -> u16 {
    EDITOR_W.min(u16::try_from(cw).unwrap_or(u16::MAX).saturating_sub(4))
}

fn editor_height(editor: &Editor, cw: usize) -> u16 {
    match editor {
        Editor::Pick(e) => e.picker.height(),
        Editor::Text(e) => e.editor.height(editor_width(cw)),
    }
}

/// The body lines and where the focused row and its notes start and end.
fn body(state: &AppState, scope: Scope, cw: usize) -> (Vec<Line>, usize, usize) {
    let mut lines = Vec::new();
    let (mut start, mut end) = (0, 0);
    let focus = state.profile.field;
    let editor = open_editor(state);
    let note_w = cw.saturating_sub(6).max(8);
    for (si, section) in Section::ALL.iter().enumerate() {
        if si > 0 {
            lines.push(Line::Blank);
        }
        lines.push(Line::Section(*section));
        for field in section.fields() {
            lines.push(Line::Field(field.id));
            let focused = field.id == focus;
            if focused {
                start = lines.len() - 1;
                if let Some(e) = editor {
                    let h = editor_height(e, cw);
                    lines.push(Line::Editor(h));
                    for _ in 1..h {
                        lines.push(Line::Taken);
                    }
                } else {
                    for l in wrap(field.description, note_w) {
                        lines.push(Line::Note(l, Role::Muted));
                    }
                    if all_games_scope(scope) {
                        let n = own_value_games(state, field.id).len();
                        if n > 0 {
                            let text = format!(
                                "Actions (0): {} ({})",
                                words::APPLY_TO_GAMES_WITH_OWN_VALUE,
                                words::games_text(n)
                            );
                            for l in wrap(&text, note_w) {
                                lines.push(Line::Note(l, Role::Muted));
                            }
                        }
                    }
                }
            }
            if field.id == FieldKey::ProtonVkd3dHeap
                && let Scope::Game(app_id) = scope
                && resolved(state, scope, field.id).is_some_and(|r| r.value == Value::Bool(true))
                && let Some((text, role)) = notice_line(state, app_id)
            {
                for l in wrap(&text, note_w) {
                    lines.push(Line::Note(l, role));
                }
            }
            if focused {
                end = lines.len() - 1;
            }
        }
    }
    (lines, start, end)
}

fn status_word(state: &AppState, scope: Scope) -> (String, Role) {
    if let Some(e) = state.profile.save_errors.get(&scope) {
        return (format!("Not saved: {e}"), Role::Error);
    }
    if state.profile.pending.values().any(|p| p.scope == scope) {
        return ("Saving...".into(), Role::Warn);
    }
    if layer(state, scope).is_some_and(|l| l.read_only) {
        return ("Read only: written by a newer spela".into(), Role::Warn);
    }
    ("Saved at once".into(), Role::Muted)
}

fn dll_summary(game: &Game) -> String {
    let mut parts = Vec::new();
    for family in &spela_core::dll::detect::CATALOGUE {
        if let Some(d) = game.dlls.iter().find(|d| d.dll_type == family.dll_type) {
            parts.push(format!("{} {}", family.label, dlls::version_text(d)));
        }
    }
    if parts.is_empty() {
        "none detected".to_owned()
    } else {
        parts.join(", ")
    }
}

/// Draws the three header lines and a blank; returns the next row.
fn header(
    state: &AppState,
    p: &mut Painter<'_>,
    scope: Scope,
    x: u16,
    y: u16,
    w: u16,
    hits: &mut HitMap,
) -> u16 {
    let dot = state.glyphs.dot;
    let (status, status_role) = status_word(state, scope);
    let sep = format!(" {dot} ");
    match (scope, library::selected_game(state)) {
        (Scope::Game(_), Some(game)) => {
            let name = format!("{} {dot} App ID {}", game.name, game.app_id);
            // The DLL summary takes the room between the name and the save
            // state, cut with the ellipsis, so the state always shows.
            let room = usize::from(w).saturating_sub(width(&name) + width(&status));
            let lead = format!(" {dot} DLLs: ");
            let summary = dll_summary(game);
            let frame = width(&lead) + width(&sep);
            let dlls = if frame + width(&summary) <= room {
                format!("{lead}{summary}{sep}")
            } else if room >= frame + 8 {
                let (cut, _) = crate::text::truncate(&summary, room - frame, state.glyphs.ellipsis);
                format!("{lead}{cut}{sep}")
            } else {
                sep.clone()
            };
            let middle = dlls.as_str();
            p.spans(
                x,
                y,
                w,
                &[
                    (name.as_str(), Role::Title),
                    (middle, Role::Muted),
                    (status.as_str(), status_role),
                ],
            );
            let own: Vec<String> = keys()
                .filter_map(|k| {
                    let r = resolved(state, scope, k)?;
                    r.is_own().then(|| {
                        format!("{} {}", k.field().label, words::value_text(k.field(), &r))
                    })
                })
                .collect();
            let line = if state.profile.profiles.is_none() {
                String::new()
            } else if own.is_empty() {
                format!(
                    "Own settings: none, every value follows {}",
                    words::ALL_GAMES
                )
            } else {
                format!("Own settings ({}): {}", own.len(), own.join(&sep))
            };
            p.text(x, y + 1, w, &line, Role::Muted);
        }
        _ => {
            p.spans(
                x,
                y,
                w,
                &[
                    (words::ALL_GAMES, Role::Title),
                    (sep.as_str(), Role::Muted),
                    (status.as_str(), status_role),
                ],
            );
            p.text(
                x,
                y + 1,
                w,
                "Games follow these values unless they set their own.",
                Role::Muted,
            );
        }
    }
    // The section strip.
    let current = match aspect(state) {
        Aspect::Profile => Some(state.profile.field.field().section),
        Aspect::Dlls => None,
    };
    let mut at = x;
    let end = x + w;
    let mut entries: Vec<(String, bool, AnyAction)> = Section::ALL
        .iter()
        .map(|s| {
            (
                s.title().to_owned(),
                current == Some(*s),
                act(Action::GoSection(*s)),
            )
        })
        .collect();
    if matches!(scope, Scope::Game(_)) {
        entries.push((
            "DLLs".to_owned(),
            aspect(state) == Aspect::Dlls,
            act(Action::GoDlls),
        ));
    }
    for (i, (name, on, action)) in entries.into_iter().enumerate() {
        if i > 0 {
            at += 2;
        }
        if at >= end {
            break;
        }
        let (text, role) = if on {
            (format!("[{name}]"), Role::Key)
        } else {
            (name, Role::Muted)
        };
        let after = p.text(at, y + 2, end - at, &text, role);
        hits.push(
            Rect::new(at, y + 2, after - at, 1),
            HitTarget::Action(action),
        );
        at = after;
    }
    y + 4
}

/// Draws the page.
pub fn render(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let inner = area.inner;
    if inner.height < 5 || inner.width < 20 {
        return;
    }
    let (x, w) = (inner.x + 1, inner.width - 2);
    let Some(scope) = scope(state) else {
        p.text(
            x,
            inner.y,
            w,
            "Select a game from the scope list",
            Role::Muted,
        );
        return;
    };
    let top = header(state, &mut p, scope, x, inner.y, w, hits);
    let rest = Rect::new(
        inner.x,
        top,
        inner.width,
        inner.bottom().saturating_sub(top),
    );
    if let Some(e) = broken(state, scope) {
        let lines = wrap(&format!("Unavailable: {e}"), usize::from(w));
        for (y, line) in (rest.y..rest.bottom()).zip(lines) {
            p.text(x, y, w, &line, Role::Error);
        }
        return;
    }
    if aspect(state) == Aspect::Dlls {
        game_dlls::render(state, &mut p, hits, rest, x, w);
        return;
    }
    if state.profile.profiles.is_none() {
        let (text, role) = match &state.profile.load_error {
            Some(e) => (format!("Profiles failed to load: {e}"), Role::Error),
            None => ("Loading profiles...".to_owned(), Role::Muted),
        };
        p.text(x, rest.y, w, &text, role);
        return;
    }
    let (lines, start, end) = body(state, scope, usize::from(w));
    let h = usize::from(rest.height);
    let mut first = 0;
    if lines.len() > h {
        first = (end + 1).saturating_sub(h).min(start).min(lines.len() - h);
    }
    let focused = state.focus == Pane::Detail;
    let label_w = label_width();
    let glyphs = state.glyphs;
    for (n, line) in lines.iter().enumerate().skip(first).take(h) {
        let y = rest.y + u16::try_from(n - first).unwrap_or(0);
        match line {
            Line::Blank | Line::Taken => {}
            Line::Section(s) => {
                p.text(x, y, w, s.title(), Role::Title);
            }
            Line::Note(text, role) => {
                p.text(x + 4, y, w.saturating_sub(4), text, *role);
            }
            Line::Field(key) => {
                let Some(r) = resolved(state, scope, *key) else {
                    continue;
                };
                let field = key.field();
                let is_focus = *key == state.profile.field;
                let own = r.is_own();
                let mut value = words::value_text(field, &r);
                if is_focus && let Some(e) = open_editor(state) {
                    value = editor_value(state, scope, e);
                }
                let source = dotted(state, &words::source_text(field, &r));
                let text = format!(
                    "{}  {}  {source}",
                    crate::text::fit(field.label, label_w, glyphs.ellipsis),
                    crate::text::fit(&value, VALUE_W, glyphs.ellipsis)
                );
                let role = if is_focus {
                    if focused { Role::Key } else { Role::Strong }
                } else if own {
                    Role::Text
                } else {
                    Role::Muted
                };
                if is_focus {
                    p.text(x, y, 2, glyphs.cursor, Role::Key);
                }
                if own {
                    p.text(x + 2, y, 2, glyphs.own, Role::Own);
                }
                p.text(x + 4, y, w.saturating_sub(4), &text, role);
                let row = Rect::new(inner.x, y, inner.width, 1);
                if is_focus {
                    p.cursor_row(row);
                }
                hits.push(
                    row,
                    HitTarget::Row {
                        select: act(Action::SelectField(*key)),
                        open: act(Action::Edit),
                    },
                );
            }
            Line::Editor(rows) => {
                let room = u16::try_from(first + h - n).unwrap_or(0).min(*rows);
                let ew = editor_width(usize::from(w));
                let box_area = Rect::new(x + 4, y, ew, room);
                if let Some(e) = open_editor(state) {
                    match e {
                        Editor::Pick(e) => e.picker.render(&mut p, box_area),
                        Editor::Text(e) => e.editor.render(&mut p, box_area),
                    }
                }
            }
        }
    }
    let track = Rect::new(0, rest.y, 1, rest.height);
    p.thumb(area.border_x, track, first, h, lines.len());
}

/// The row's value while its editor is open: what Apply would save.
fn editor_value(state: &AppState, scope: Scope, editor: &Editor) -> String {
    let key = editor.key();
    match editor {
        Editor::Pick(e) => match e.values.get(e.picker.cursor).cloned().flatten() {
            Some(v) => words::display_value(key.field(), &v),
            None => follow_text(state, scope, key),
        },
        Editor::Text(e) => e.editor.input.text().to_owned(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::event::Msg as AnyMsg;
    use crate::testing::{key, state_with_games};
    use crate::update::update as step;

    fn loaded_state() -> AppState {
        let mut s = state_with_games(120, 40, 6);
        let cmds = s.start();
        let request = cmds
            .iter()
            .find_map(|c| match c {
                AnyCmd::Profile(Cmd::Load { request }) => Some(*request),
                _ => None,
            })
            .expect("a load");
        let mut all = OwnValues::new();
        all.set(FieldKey::DlssSrMode, Value::Text("balanced".into()))
            .unwrap();
        let game = s.games.games[0].app_id;
        let mut own = OwnValues::new();
        own.set(FieldKey::DlssSrMode, Value::Text("quality".into()))
            .unwrap();
        on_msg(
            &mut s,
            Msg::Loaded {
                request,
                result: Ok(Loaded {
                    all_games: Some((all, false)),
                    games: vec![LoadedGame {
                        app_id: game,
                        values: own,
                        read_only: false,
                    }],
                    broken: Vec::new(),
                }),
            },
        );
        s
    }

    fn save_token(cmds: &[AnyCmd]) -> u64 {
        cmds.iter()
            .find_map(|c| match c {
                AnyCmd::Save { token, .. } => Some(*token),
                _ => None,
            })
            .expect("a save")
    }

    #[test]
    fn space_follows_and_puts_the_own_value_back() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        s.profile.field = FieldKey::DlssSrMode;
        let scope = scope(&s).unwrap();
        let cmds = step(&mut s, key("space"));
        let token = save_token(&cmds);
        let r = resolved(&s, scope, FieldKey::DlssSrMode).unwrap();
        assert_eq!(r.value, Value::Text("balanced".into()), "shows at once");
        step(
            &mut s,
            crate::event::Event::Msg(AnyMsg::Saved {
                token,
                result: Ok(()),
            }),
        );
        assert_eq!(
            s.status.as_ref().unwrap().text,
            "Saved: Super resolution follows All games"
        );
        let cmds = step(&mut s, key("space"));
        let AnyCmd::Save { write, .. } = &cmds[0] else {
            panic!("{cmds:?}")
        };
        let Write::Profile { changes, .. } = write else {
            panic!()
        };
        assert_eq!(
            changes[&FieldKey::DlssSrMode],
            Some(Value::Text("quality".into()))
        );
    }

    #[test]
    fn a_failed_save_puts_the_stored_value_back() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        let scope = scope(&s).unwrap();
        s.profile.field = FieldKey::ProtonEnableHdr;
        let t1 = save_token(&step(&mut s, key("space")));
        // A second change before the first answer: it merges in the queue.
        let t2 = save_token(&step(&mut s, key("space")));
        let hdr = |s: &AppState| resolved(s, scope, FieldKey::ProtonEnableHdr).unwrap();
        assert!(hdr(&s).own.is_none(), "the second Space followed again");
        let failed = |t| {
            crate::event::Event::Msg(AnyMsg::Saved {
                token: t,
                result: Err("disk full".into()),
            })
        };
        step(&mut s, failed(t1));
        step(&mut s, failed(t2));
        assert_eq!(hdr(&s).own, None, "back to what is stored");
        assert_eq!(
            status_word(&s, scope),
            ("Not saved: disk full".into(), Role::Error)
        );
        assert!(
            s.status
                .as_ref()
                .unwrap()
                .text
                .starts_with("Not saved: HDR")
        );
        // A failure after an own value was set puts "follows" back.
        let t3 = save_token(&step(&mut s, key("space")));
        assert_eq!(hdr(&s).own, Some(Value::Bool(true)));
        step(&mut s, failed(t3));
        assert_eq!(hdr(&s).own, None);
    }

    #[test]
    fn the_picker_lists_follow_first_and_marks_the_stored_value() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        s.profile.field = FieldKey::DlssSrMode;
        step(&mut s, key("enter"));
        let Some(Editor::Pick(e)) = &s.profile.editor else {
            panic!("a picker")
        };
        let labels: Vec<&str> = e.picker.choices.iter().map(|c| c.label.as_str()).collect();
        assert_eq!(
            labels,
            [
                "Follow All games (balanced)",
                "none",
                "off",
                "ultra_performance",
                "performance",
                "balanced",
                "quality",
                "dlaa"
            ]
        );
        assert!(e.picker.choices[6].current);
        assert_eq!(e.picker.before.as_deref(), Some("quality"));
        assert!(editing(&s, Pane::Detail));
        // Letters type in the picker: p jumps, then Enter applies.
        step(&mut s, key("p"));
        let cmds = step(&mut s, key("enter"));
        assert!(s.profile.editor.is_none());
        let AnyCmd::Save { write, .. } = &cmds[0] else {
            panic!("{cmds:?}")
        };
        let Write::Profile { changes, .. } = write else {
            panic!()
        };
        assert_eq!(
            changes[&FieldKey::DlssSrMode],
            Some(Value::Text("performance".into()))
        );
    }

    #[test]
    fn smt_vrr_and_multi_frame_choices() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        let labels = |s: &mut AppState, k: FieldKey| {
            s.profile.editor = None;
            s.profile.field = k;
            update(s, Action::Edit);
            match &s.profile.editor {
                Some(Editor::Pick(e)) => e.picker.choices.iter().map(|c| c.label.clone()).collect(),
                other => panic!("{other:?}"),
            }
        };
        let smt: Vec<String> = labels(&mut s, FieldKey::CpuSmt);
        assert_eq!(smt, ["Follow All games (none)", "On", "Off"]);
        let vrr: Vec<String> = labels(&mut s, FieldKey::GpuVrr);
        assert!(!vrr.contains(&"none".to_owned()), "{vrr:?}");
        let mf: Vec<String> = labels(&mut s, FieldKey::DlssMultiFrame);
        assert_eq!(mf, ["Follow All games (none)", "0", "1", "2", "3", "4"]);
    }

    #[test]
    fn smt_left_unchanged_reads_unchanged_and_follow_removes_it() {
        // Decision 65: a game may store SMT's none as "unchanged".
        let mut s = state_with_games(120, 40, 6);
        let cmds = s.start();
        let request = cmds
            .iter()
            .find_map(|c| match c {
                AnyCmd::Profile(Cmd::Load { request }) => Some(*request),
                _ => None,
            })
            .expect("a load");
        let mut all = OwnValues::new();
        all.set(FieldKey::CpuSmt, Value::OptionalBool(Some(false)))
            .unwrap();
        let mut own = OwnValues::new();
        own.set(FieldKey::CpuSmt, Value::OptionalBool(None))
            .unwrap();
        let game = s.games.games[0].app_id;
        on_msg(
            &mut s,
            Msg::Loaded {
                request,
                result: Ok(Loaded {
                    all_games: Some((all, false)),
                    games: vec![LoadedGame {
                        app_id: game,
                        values: own,
                        read_only: false,
                    }],
                    broken: Vec::new(),
                }),
            },
        );
        s.focus = Pane::Detail;
        s.profile.field = FieldKey::CpuSmt;
        let scope = scope(&s).unwrap();
        let r = resolved(&s, scope, FieldKey::CpuSmt).unwrap();
        assert!(r.is_own());
        assert_eq!(words::value_text(FieldKey::CpuSmt.field(), &r), "unchanged");
        update(&mut s, Action::Edit);
        let Some(Editor::Pick(e)) = &s.profile.editor else {
            panic!("a picker")
        };
        let marked: Vec<&str> = e
            .picker
            .choices
            .iter()
            .filter(|c| c.current)
            .map(|c| c.label.as_str())
            .collect();
        assert_eq!(marked, ["unchanged"], "the stored value stays a choice");
        assert!(
            !e.picker.choices[0].current,
            "Follow is not the stored value"
        );
        s.profile.editor = None;
        let cmds = step(&mut s, key("space"));
        let AnyCmd::Save { write, .. } = &cmds[0] else {
            panic!("{cmds:?}")
        };
        let Write::Profile { changes, .. } = write else {
            panic!()
        };
        assert_eq!(changes[&FieldKey::CpuSmt], None, "Space follows");
    }

    #[test]
    fn the_number_editor_opens_empty_on_none_and_replaces_on_typing() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        s.profile.field = FieldKey::GpuClockOffset;
        step(&mut s, key("enter"));
        let Some(Editor::Text(e)) = &s.profile.editor else {
            panic!("a text editor")
        };
        assert_eq!(e.editor.input.text(), "", "none opens empty");
        assert_eq!(e.editor.before.as_deref(), Some("none"));
        // An empty Apply on none changes nothing and closes the editor.
        assert!(step(&mut s, key("enter")).is_empty());
        assert!(s.profile.editor.is_none());
        // A value that is not a number is refused in Go's words, and the
        // editor stays.
        step(&mut s, key("enter"));
        step(&mut s, key("x"));
        step(&mut s, key("enter"));
        let Some(Editor::Text(e)) = &s.profile.editor else {
            panic!("still open")
        };
        assert_eq!(e.editor.error.as_deref(), Some("invalid integer: x"));
        step(&mut s, key("ctrl-u"));
        for c in ["1", "5", "0"] {
            step(&mut s, key(c));
        }
        let cmds = step(&mut s, key("enter"));
        let token = save_token(&cmds);
        let scope = scope(&s).unwrap();
        assert_eq!(
            resolved(&s, scope, FieldKey::GpuClockOffset).unwrap().own,
            Some(Value::Int(150))
        );
        step(
            &mut s,
            crate::event::Event::Msg(AnyMsg::Saved {
                token,
                result: Ok(()),
            }),
        );
        // Editing again prefills 150; the first key replaces it.
        step(&mut s, key("enter"));
        step(&mut s, key("2"));
        let Some(Editor::Text(e)) = &s.profile.editor else {
            panic!()
        };
        assert_eq!(e.editor.input.text(), "2");
        step(&mut s, key("esc"));
        assert!(s.profile.editor.is_none(), "Esc cancels");
    }

    #[test]
    fn a_new_game_profile_gains_its_flag_after_the_save() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        step(&mut s, key("down"));
        // Game index 1 has no profile in the fixture (every third has).
        s.library.cursor = 2;
        let Some(Scope::Game(id)) = scope(&s) else {
            panic!()
        };
        assert!(!library::has_profile(&s, id));
        s.profile.field = FieldKey::ProtonEnableHdr;
        let token = save_token(&step(&mut s, key("space")));
        assert!(!library::has_profile(&s, id), "not before the save");
        step(
            &mut s,
            crate::event::Event::Msg(AnyMsg::Saved {
                token,
                result: Ok(()),
            }),
        );
        assert!(library::has_profile(&s, id));
    }

    #[test]
    fn whole_page_reset_asks_first_with_cancel_focused() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        update(&mut s, Action::ResetAll);
        let Some(Popup::Confirm(c)) = &s.popup else {
            panic!("a confirmation")
        };
        assert_eq!(c.focus, confirm::Focus::Cancel);
        assert_eq!(c.title, "Follow All games for all settings");
        let body: String = c.body.iter().flatten().map(|(t, _)| t.as_str()).collect();
        assert!(body.contains("This clears the 1 setting"), "{body}");
        assert!(step(&mut s, key("enter")).is_empty(), "Enter on Cancel");
        update(&mut s, Action::ResetAll);
        step(&mut s, key("right"));
        let cmds = step(&mut s, key("enter"));
        save_token(&cmds);
        let scope = scope(&s).unwrap();
        assert_eq!(own_keys(&s, scope).len(), 0);
    }

    #[test]
    fn apply_to_games_with_their_own_value_names_them_and_reports() {
        let mut s = loaded_state();
        s.library.cursor = 0;
        assert_eq!(scope(&s), Some(Scope::AllGames));
        s.focus = Pane::Detail;
        s.profile.field = FieldKey::DlssSrMode;
        assert!(apply_available(&s).is_ok());
        update(&mut s, Action::ApplyToOwn);
        let Some(Popup::Confirm(c)) = &s.popup else {
            panic!("a confirmation")
        };
        let body: String = c
            .body
            .iter()
            .flatten()
            .map(|(t, _)| t.as_str())
            .collect::<Vec<_>>()
            .join(" ");
        assert!(body.contains("Ashfall Courier"), "{body}");
        assert!(body.contains("(balanced)"), "{body}");
        step(&mut s, key("right"));
        let cmds = step(&mut s, key("enter"));
        let token = save_token(&cmds);
        step(
            &mut s,
            crate::event::Event::Msg(AnyMsg::Saved {
                token,
                result: Ok(()),
            }),
        );
        assert_eq!(
            s.status.as_ref().unwrap().text,
            "Saved: Super resolution: 1 game follows All games"
        );
        assert!(apply_available(&s).is_err());
    }

    #[test]
    fn the_vkd3d_notice_is_asked_for_and_drawn() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        s.profile.field = FieldKey::ProtonVkd3dHeap;
        let cmds = step(&mut s, key("space"));
        let request = cmds
            .iter()
            .find_map(|c| match c {
                AnyCmd::Profile(Cmd::Notice { request, .. }) => Some(*request),
                _ => None,
            })
            .expect("a notice request");
        let Some(Scope::Game(app_id)) = scope(&s) else {
            panic!()
        };
        on_msg(
            &mut s,
            Msg::Notice {
                request,
                app_id,
                notice: Some(
                    "⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: 575.64.3)"
                        .into(),
                ),
            },
        );
        let text = crate::testing::render_text(&mut s);
        assert!(
            text.contains("⚠ descriptor_heap requires NVIDIA driver 580.94.16+"),
            "{text}"
        );
        on_msg(
            &mut s,
            Msg::Notice {
                request,
                app_id,
                notice: Some(format!(
                    "{}{}",
                    spela_core::proton::INFO_GLYPH,
                    spela_core::proton::SKIP_DRIVER_NOT_DETECTED
                )),
            },
        );
        s.profile.notices.insert(
            app_id,
            Notice::Ready(Some(format!(
                "{}{}",
                spela_core::proton::INFO_GLYPH,
                spela_core::proton::SKIP_DRIVER_NOT_DETECTED
            ))),
        );
        let text = crate::testing::render_text(&mut s);
        assert!(text.contains("NVIDIA driver not detected"), "{text}");
        assert!(!text.contains('ⓘ'), "{text}");
    }

    #[test]
    fn preview_opens_read_only_and_asks_the_shell() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        let cmds = update(&mut s, Action::Preview);
        assert!(matches!(cmds[0], AnyCmd::Preview { .. }), "{cmds:?}");
        assert!(matches!(s.popup, Some(Popup::Preview(_))));
        s.close_popup();
        s.library.cursor = 0;
        let row = preview_row(&s);
        assert_eq!(row.unavailable.as_deref(), Some("select a game"));
    }

    #[test]
    fn right_from_cpu_opens_dlls_and_left_comes_back() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        s.profile.field = FieldKey::CpuGovernor;
        step(&mut s, key("right"));
        assert_eq!(aspect(&s), Aspect::Dlls);
        assert!(can_next(&s).is_err());
        let text = crate::testing::render_text(&mut s);
        assert!(
            text.contains("[DLLs]") && text.contains("DLL versions"),
            "{text}"
        );
        step(&mut s, key("left"));
        assert_eq!(aspect(&s), Aspect::Profile);
        assert_eq!(s.profile.field, FieldKey::CpuGovernor);
        // All games has no DLLs: Right from CPU is the end.
        s.library.cursor = 0;
        assert_eq!(can_next(&s), Err("last section".into()));
    }

    #[test]
    fn a_load_that_a_save_overtook_is_read_again() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        let cmds = rescanned(&mut s);
        let AnyCmd::Profile(Cmd::Load { request }) = cmds[0] else {
            panic!("{cmds:?}")
        };
        s.profile.field = FieldKey::ProtonEnableHdr;
        let token = save_token(&step(&mut s, key("space")));
        step(
            &mut s,
            crate::event::Event::Msg(AnyMsg::Saved {
                token,
                result: Ok(()),
            }),
        );
        // The stale read, from before the save.
        let again = on_msg(
            &mut s,
            Msg::Loaded {
                request,
                result: Ok(Loaded::default()),
            },
        );
        assert!(
            matches!(again[0], AnyCmd::Profile(Cmd::Load { .. })),
            "{again:?}"
        );
        let scope = scope(&s).unwrap();
        assert_eq!(
            resolved(&s, scope, FieldKey::ProtonEnableHdr).unwrap().own,
            Some(Value::Bool(true)),
            "the stale read is not shown"
        );
    }

    #[test]
    fn a_broken_scope_is_shown_and_never_edited() {
        let mut s = loaded_state();
        s.focus = Pane::Detail;
        let scope = scope(&s).unwrap();
        s.broken_profiles
            .insert(scope, "profile 9100010.yaml: bad".into());
        assert!(step(&mut s, key("space")).is_empty());
        let text = crate::testing::render_text(&mut s);
        assert!(
            text.contains("Unavailable: profile 9100010.yaml: bad"),
            "{text}"
        );
    }
}
