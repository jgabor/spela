//! Screens and popups: one file each, registered here and nowhere else.
//!
//! A destination has two panes. Each pane belongs to one screen module:
//!
//! | Destination | List | Detail |
//! |---|---|---|
//! | 1 Library | [`library`] | [`profile`] (its DLLs section draws [`dlls`] and [`denylist`]) |
//! | 2 DLL Catalog | [`dlls`] | [`dlls`] |
//! | 3 Monitor | [`monitor`] | [`monitor`] |
//! | 4 Settings | [`settings`] | [`settings`] |
//!
//! The popups are [`actions`], [`confirm`], [`help`], [`palette`],
//! [`notice`] (the migration notice) and [`preview`] (Preview launch).
//!
//! # The screen contract
//!
//! Every screen module has the same shape, so it can grow in its own file:
//!
//! - `State` (with `Default`): its own state, a field of [`AppState`].
//! - `Action`, `Msg`, `Cmd`: what its keys run, what workers send back and
//!   what it asks the shell for. They travel wrapped as
//!   [`ScreenAction`], [`crate::Msg`] and [`crate::Cmd`] variants named
//!   after the screen.
//! - `title`, `bindings`, `editing`, `actions`, `palette`, `update`,
//!   `on_msg`, `saved`, `render` and `wheel`, with the signatures of the
//!   [`library`] module. A screen without a part returns nothing for it.
//!
//! A screen reads anything in [`AppState`] and changes its own `State` and
//! the shared parts it owns. It opens a popup with [`AppState::open`], a
//! confirmation with [`AppState::confirm`], says something on the status
//! line with [`AppState::say`], and saves with [`AppState::save`]. Keys
//! and Actions rows are [`Binding`]s and [`Row`]s it returns; the hint bar,
//! Help, Actions and the palette all read them, so they cannot drift from
//! dispatch.

pub mod actions;
pub mod confirm;
pub mod denylist;
pub mod dlls;
pub mod help;
pub mod library;
pub mod monitor;
pub mod notice;
pub mod palette;
pub mod preview;
pub mod profile;
pub mod settings;

use ratatui::Frame;

use crate::hit::HitMap;
use crate::keymap::{Binding, Hint, Key};
use crate::layout::{Layout, Mode, Pane, PaneArea};
use crate::state::AppState;
use crate::{Cmd, Msg};

/// A destination, in top bar order.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum Dest {
    /// 1 Library: games and their profiles.
    Library,
    /// 2 DLL Catalog: DLL families and deployments.
    Catalog,
    /// 3 Monitor: GPU, CPU and alerts.
    Monitor,
    /// 4 Settings.
    Settings,
}

impl Dest {
    /// Every destination, in order.
    pub const ALL: [Dest; 4] = [Dest::Library, Dest::Catalog, Dest::Monitor, Dest::Settings];

    /// The name in the top bar.
    pub fn name(self) -> &'static str {
        match self {
            Dest::Library => "Library",
            Dest::Catalog => "DLL Catalog",
            Dest::Monitor => "Monitor",
            Dest::Settings => "Settings",
        }
    }

    /// Its digit, 1 to 4.
    pub fn digit(self) -> char {
        match self {
            Dest::Library => '1',
            Dest::Catalog => '2',
            Dest::Monitor => '3',
            Dest::Settings => '4',
        }
    }
}

/// Everything a key, a click, an Actions row or a palette row can run.
#[derive(Clone, Debug, PartialEq)]
pub enum Action {
    /// Ctrl-C: quit at once, from anywhere.
    Interrupt,
    /// The Quit row and `Q`: quit, refused while a scan, DLL or hardware
    /// job runs. Pending saves do not refuse it.
    Quit,
    /// Esc: back one level. It never quits and never confirms.
    Back,
    /// `q`: back one level, then quit at the top level.
    BackOrQuit,
    /// `0`: the Actions popup for this context.
    Actions,
    /// `?`: Help for this context.
    Help,
    /// `:` and Ctrl-K: the command palette.
    Palette,
    /// `/`: Find in the current list.
    Find,
    /// `1` to `4`: go to a destination. Focus returns to List.
    Go(Dest),
    /// Tab: the next pane.
    NextPane,
    /// Shift-Tab: the previous pane.
    PrevPane,
    /// Ctrl-Z: stop to the shell.
    Suspend,
    /// A popup's own action.
    Popup(PopupAction),
    /// A screen's own action.
    Screen(ScreenAction),
}

/// A screen's action, by screen.
#[derive(Clone, Debug, PartialEq)]
pub enum ScreenAction {
    /// [`library`].
    Library(library::Action),
    /// [`profile`].
    Profile(profile::Action),
    /// [`settings`].
    Settings(settings::Action),
    /// [`monitor`].
    Monitor(monitor::Action),
    /// [`dlls`].
    Dlls(dlls::Action),
    /// [`denylist`].
    Denylist(denylist::Action),
}

/// A popup's action, by popup.
#[derive(Clone, Debug, PartialEq)]
pub enum PopupAction {
    /// [`actions`].
    Actions(actions::Action),
    /// [`confirm`].
    Confirm(confirm::Action),
    /// [`help`].
    Help(help::Action),
    /// [`palette`].
    Palette(palette::Action),
    /// [`notice`].
    Notice(notice::Action),
    /// [`preview`].
    Preview(preview::Action),
}

/// The open popup. One at a time: Help and a confirmation replace the
/// Actions popup they open from.
#[derive(Clone, Debug)]
pub enum Popup {
    /// [`actions`].
    Actions(actions::State),
    /// [`confirm`].
    Confirm(confirm::State),
    /// [`help`].
    Help(help::State),
    /// [`palette`].
    Palette(palette::State),
    /// [`notice`].
    Notice(notice::State),
    /// [`preview`].
    Preview(preview::State),
}

/// The screen that owns a save, so its result reaches it.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Owner {
    /// [`library`].
    Library,
    /// [`profile`].
    Profile,
    /// [`settings`].
    Settings,
    /// [`monitor`].
    Monitor,
    /// [`dlls`].
    Dlls,
    /// [`denylist`].
    Denylist,
}

/// What a palette row is, shown in its right column.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum RowKind {
    /// An Actions row.
    Action,
    /// A destination.
    Destination,
    /// A game.
    Game,
    /// A setting.
    Setting,
}

impl RowKind {
    /// The word the palette shows.
    pub fn word(self) -> &'static str {
        match self {
            RowKind::Action => "action",
            RowKind::Destination => "destination",
            RowKind::Game => "game",
            RowKind::Setting => "setting",
        }
    }
}

/// A row of the Actions popup or the palette.
#[derive(Clone, Debug, PartialEq)]
pub struct Row {
    /// The label, such as `Rescan games`.
    pub label: String,
    /// What Enter runs.
    pub action: Action,
    /// Why it cannot run now. The row stays, dimmed, with the reason.
    pub unavailable: Option<String>,
    /// The key that runs it outside the popup, shown on the right.
    pub key: Option<Key>,
    /// What it is, for the palette.
    pub kind: RowKind,
}

impl Row {
    /// An available Actions row with no key of its own.
    pub fn new(label: impl Into<String>, action: Action) -> Row {
        Row {
            label: label.into(),
            action,
            unavailable: None,
            key: None,
            kind: RowKind::Action,
        }
    }

    /// Shows the key that runs it outside the popup.
    pub fn key(mut self, key: Key) -> Row {
        self.key = Some(key);
        self
    }

    /// Marks it unavailable with a reason, or available with `Ok`.
    pub fn when(mut self, available: Result<(), String>) -> Row {
        self.unavailable = available.err();
        self
    }

    /// Sets what the palette calls it.
    pub fn kind(mut self, kind: RowKind) -> Row {
        self.kind = kind;
        self
    }
}

/// Which screen draws a pane of a destination.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PaneOwner {
    /// [`library`].
    Library,
    /// [`profile`].
    Profile,
    /// [`dlls`].
    Dlls,
    /// [`monitor`].
    Monitor,
    /// [`settings`].
    Settings,
}

/// The screen that draws `pane` on `dest`.
pub fn owner(dest: Dest, pane: Pane) -> PaneOwner {
    match (dest, pane) {
        (Dest::Library, Pane::List) => PaneOwner::Library,
        (Dest::Library, Pane::Detail) => PaneOwner::Profile,
        (Dest::Catalog, _) => PaneOwner::Dlls,
        (Dest::Monitor, _) => PaneOwner::Monitor,
        (Dest::Settings, _) => PaneOwner::Settings,
    }
}

macro_rules! by_owner {
    ($owner:expr, $f:ident ( $($arg:expr),* )) => {
        match $owner {
            PaneOwner::Library => library::$f($($arg),*),
            PaneOwner::Profile => profile::$f($($arg),*),
            PaneOwner::Dlls => dlls::$f($($arg),*),
            PaneOwner::Monitor => monitor::$f($($arg),*),
            PaneOwner::Settings => settings::$f($($arg),*),
        }
    };
}

/// A pane's title, in its top border.
pub fn title(state: &AppState, pane: Pane) -> String {
    by_owner!(owner(state.dest, pane), title(state, pane))
}

/// Whether the screen in `pane` has a text input, editor or picker open.
/// Then only its own bindings and Ctrl-C work, and letters and digits
/// type (terminal-ux.md, option B).
pub fn editing(state: &AppState, pane: Pane) -> bool {
    deny_editing(state, pane) || by_owner!(owner(state.dest, pane), editing(state, pane))
}

/// The deny list's reason editor is open over Library's Detail. It lives
/// in the game's DLLs section, so it takes that pane's keys and draws
/// there.
fn deny_editing(state: &AppState, pane: Pane) -> bool {
    owner(state.dest, pane) == PaneOwner::Profile && denylist::editing(state, pane)
}

/// Whether the UI is in Browse: no popup and no editor. Digits show in the
/// top bar only then.
pub fn browsing(state: &AppState) -> bool {
    state.popup.is_none() && !editing(state, state.focus)
}

/// The bindings of the focused pane's screen.
pub fn screen_bindings(state: &AppState, pane: Pane) -> Vec<Binding> {
    if deny_editing(state, pane) {
        return denylist::bindings(state, pane);
    }
    by_owner!(owner(state.dest, pane), bindings(state, pane))
}

/// Every binding that is active now, in dispatch order: Ctrl-C first,
/// then the popup's, or the screen's between the global ones. The hint
/// bar, Help and dispatch all read this one list.
pub fn bindings(state: &AppState) -> Vec<Binding> {
    let mut out = vec![Binding::new(
        Key::ctrl('c'),
        Action::Interrupt,
        "Quit at once",
    )];
    if state.layout().mode == Mode::TooSmall {
        return out;
    }
    if let Some(popup) = &state.popup {
        out.extend(popup_bindings(state, popup));
        return out;
    }
    let pane = state.focus;
    if editing(state, pane) {
        out.extend(screen_bindings(state, pane));
        return out;
    }
    let (lead, tail) = global_bindings(state);
    out.extend(lead);
    out.extend(screen_bindings(state, pane));
    out.extend(tail);
    out
}

fn popup_bindings(state: &AppState, popup: &Popup) -> Vec<Binding> {
    match popup {
        Popup::Actions(p) => actions::bindings(state, p),
        Popup::Confirm(p) => confirm::bindings(state, p),
        Popup::Help(p) => help::bindings(state, p),
        Popup::Palette(p) => palette::bindings(state, p),
        Popup::Notice(p) => notice::bindings(state, p),
        Popup::Preview(p) => preview::bindings(state, p),
    }
}

/// The Browse bindings every screen shares: `0 Actions` before the
/// screen's own, and the rest after them, in hint bar order.
pub fn global_bindings(state: &AppState) -> (Vec<Binding>, Vec<Binding>) {
    let single = !state.layout().two_panes();
    let lead = vec![Binding::new(Key::char('0'), Action::Actions, "Actions").hint(Hint::ALWAYS)];
    let mut tail = Vec::new();
    for dest in Dest::ALL {
        tail.push(Binding::new(
            Key::char(dest.digit()),
            Action::Go(dest),
            dest.name(),
        ));
    }
    let other = state.focus.other();
    let pane_label = if single {
        match other {
            Pane::List => "List",
            Pane::Detail => "Detail",
        }
    } else {
        "Pane"
    };
    tail.push(
        Binding::new(Key::TAB, Action::NextPane, pane_label).hint(if single {
            Hint::ALWAYS
        } else {
            30
        }),
    );
    tail.push(Binding::new(
        Key::BACK_TAB,
        Action::PrevPane,
        "Previous pane",
    ));
    let back = match state.focus {
        Pane::Detail => Binding::new(Key::ESC, Action::Back, "List").hint(10),
        Pane::List => Binding::new(Key::ESC, Action::Back, "Back").unavailable("nothing to close"),
    };
    tail.push(back);
    tail.push(Binding::new(Key::char('?'), Action::Help, "Help").hint(20));
    tail.push(Binding::new(Key::char(':'), Action::Palette, "Commands").also(Key::ctrl('k')));
    let find = find_available(state);
    tail.push(Binding::new(Key::char('/'), Action::Find, "Find").when(find));
    let q_label = match state.focus {
        Pane::Detail => "Back",
        Pane::List => "Quit",
    };
    tail.push(Binding::new(Key::char('q'), Action::BackOrQuit, q_label));
    tail.push(Binding::new(Key::char('Q'), Action::Quit, "Quit now"));
    tail.push(Binding::new(
        Key::ctrl('z'),
        Action::Suspend,
        "Suspend to the shell",
    ));
    (lead, tail)
}

/// Whether `/` has a list to find in: Library has Find games.
pub fn find_available(state: &AppState) -> Result<(), String> {
    match state.dest {
        Dest::Library => library::can_find(state),
        _ => Err("Library only".into()),
    }
}

/// The Actions rows for the focused pane, then Rescan, Help and Quit, as
/// in Go (TUI 7).
pub fn actions(state: &AppState, pane: Pane) -> Vec<Row> {
    let mut rows = by_owner!(owner(state.dest, pane), actions(state, pane));
    if state.dest == Dest::Library && pane == Pane::Detail {
        rows.push(library::rescan_row(state));
    }
    rows.push(Row::new("Help", Action::Help).key(Key::char('?')));
    rows.push(
        Row::new("Quit", Action::Quit)
            .key(Key::char('q'))
            .when(state.can_quit()),
    );
    rows
}

/// Every palette row: destinations, then each screen's rows (Actions
/// rows, games, settings).
pub fn palette(state: &AppState) -> Vec<Row> {
    let mut rows: Vec<Row> = Dest::ALL
        .iter()
        .map(|d| {
            Row::new(format!("Go to {}", d.name()), Action::Go(*d))
                .key(Key::char(d.digit()))
                .kind(RowKind::Destination)
        })
        .collect();
    rows.extend(library::palette(state));
    rows.extend(profile::palette(state));
    rows.extend(dlls::palette(state));
    rows.extend(monitor::palette(state));
    rows.extend(settings::palette(state));
    rows.extend(denylist::palette(state));
    rows.push(Row::new("Help", Action::Help).key(Key::char('?')));
    rows.push(
        Row::new("Quit", Action::Quit)
            .key(Key::char('Q'))
            .when(state.can_quit()),
    );
    rows
}

/// Runs a screen's or a popup's action.
pub fn run(state: &mut AppState, action: ScreenAction) -> Vec<Cmd> {
    match action {
        ScreenAction::Library(a) => library::update(state, a),
        ScreenAction::Profile(a) => profile::update(state, a),
        ScreenAction::Settings(a) => settings::update(state, a),
        ScreenAction::Monitor(a) => monitor::update(state, a),
        ScreenAction::Dlls(a) => dlls::update(state, a),
        ScreenAction::Denylist(a) => denylist::update(state, a),
    }
}

/// Runs a popup's action.
pub fn run_popup(state: &mut AppState, action: PopupAction) -> Vec<Cmd> {
    match action {
        PopupAction::Actions(a) => actions::update(state, a),
        PopupAction::Confirm(a) => confirm::update(state, a),
        PopupAction::Help(a) => help::update(state, a),
        PopupAction::Palette(a) => palette::update(state, a),
        PopupAction::Notice(a) => notice::update(state, a),
        PopupAction::Preview(a) => preview::update(state, a),
    }
}

/// Hands a screen's message to it.
pub fn on_msg(state: &mut AppState, msg: Msg) -> Vec<Cmd> {
    match msg {
        Msg::Library(m) => library::on_msg(state, m),
        Msg::Profile(m) => profile::on_msg(state, m),
        Msg::Settings(m) => settings::on_msg(state, m),
        Msg::Monitor(m) => monitor::on_msg(state, m),
        Msg::Dlls(m) => dlls::on_msg(state, m),
        Msg::Denylist(m) => denylist::on_msg(state, m),
        // The shell-level messages are handled in crate::update.
        _ => Vec::new(),
    }
}

/// Hands a save's result to the screen that asked for it.
pub fn saved(
    state: &mut AppState,
    owner: Owner,
    token: u64,
    result: Result<(), String>,
) -> Vec<Cmd> {
    match owner {
        Owner::Library => library::saved(state, token, result),
        Owner::Profile => profile::saved(state, token, result),
        Owner::Settings => settings::saved(state, token, result),
        Owner::Monitor => monitor::saved(state, token, result),
        Owner::Dlls => dlls::saved(state, token, result),
        Owner::Denylist => denylist::saved(state, token, result),
    }
}

/// Draws a pane's content inside its borders.
pub fn render_pane(state: &AppState, frame: &mut Frame, area: PaneArea, hits: &mut HitMap) {
    by_owner!(
        owner(state.dest, area.pane),
        render(state, frame, area, hits)
    );
    if deny_editing(state, area.pane) {
        denylist::render(state, frame, area, hits);
    }
}

/// Draws the open popup over the dimmed workspace.
pub fn render_popup(state: &AppState, frame: &mut Frame, layout: &Layout, hits: &mut HitMap) {
    match &state.popup {
        Some(Popup::Actions(p)) => actions::render(state, p, frame, layout, hits),
        Some(Popup::Confirm(p)) => confirm::render(state, p, frame, layout, hits),
        Some(Popup::Help(p)) => help::render(state, p, frame, layout, hits),
        Some(Popup::Palette(p)) => palette::render(state, p, frame, layout, hits),
        Some(Popup::Notice(p)) => notice::render(state, p, frame, layout, hits),
        Some(Popup::Preview(p)) => preview::render(state, p, frame, layout, hits),
        None => {}
    }
}

/// The mouse wheel over a pane.
pub fn wheel(state: &mut AppState, pane: Pane, down: bool) -> Vec<Cmd> {
    by_owner!(owner(state.dest, pane), wheel(state, pane, down))
}

/// Text in a pane's bottom border, such as the Library legend.
pub fn footer(state: &AppState, pane: Pane) -> Option<String> {
    match owner(state.dest, pane) {
        PaneOwner::Library => library::footer(state),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn every_pane_has_one_owner() {
        assert_eq!(owner(Dest::Library, Pane::List), PaneOwner::Library);
        assert_eq!(owner(Dest::Library, Pane::Detail), PaneOwner::Profile);
        assert_eq!(owner(Dest::Settings, Pane::List), PaneOwner::Settings);
        let digits: String = Dest::ALL.iter().map(|d| d.digit()).collect();
        assert_eq!(digits, "1234");
    }
}
