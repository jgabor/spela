//! `update(&mut AppState, Event) -> Vec<Cmd>`: the one place state
//! changes.
//!
//! Keys resolve through the composed keymap ([`crate::screens::bindings`])
//! in update itself, as mejl's code does. Clicks resolve through the last
//! frame's hit map to the same actions. Worker results go to their screen.
//! update sets `state.dirty` when something visible changed, so the shell
//! draws only then: a tick whose sample shows nothing new is not drawn
//! (architecture.md, wake-up policy).

use crossterm::event::{MouseButton, MouseEvent, MouseEventKind};
use jiff::SignedDuration;

use crate::event::{Cmd, Event, Msg};
use crate::hit::HitTarget;
use crate::keymap::{self, Key, Resolution};
use crate::layout::{Mode, Pane};
use crate::screens::{self, Action, Dest, Popup, actions, help, palette};
use crate::state::{AppState, Level};

/// Two clicks on the same target within this time are a double click.
pub const DOUBLE_CLICK: SignedDuration = SignedDuration::from_millis(400);

/// Applies one event and returns what the shell should do.
pub fn update(state: &mut AppState, event: Event) -> Vec<Cmd> {
    // Below 80x24 the resize prompt shows only the size, so nothing but a
    // resize changes what is drawn. State still changes under it, and the
    // next resize draws it.
    let was_dirty = state.dirty;
    let resize = matches!(event, Event::Resize(..));
    let cmds = apply(state, event);
    if !resize && state.layout().mode == Mode::TooSmall {
        state.dirty = was_dirty;
    }
    cmds
}

fn apply(state: &mut AppState, event: Event) -> Vec<Cmd> {
    match event {
        Event::Resize(w, h) => {
            state.size = (w, h);
            state.dirty = true;
            if state.layout().two_panes() || state.focus == Pane::List {
                // Nothing to fix: the focused pane shows.
            }
            Vec::new()
        }
        Event::Key(key) => on_key(state, key),
        Event::Mouse(m) => on_mouse(state, m),
        Event::Paste(text) => on_paste(state, &text),
        Event::Tick => {
            if state.wants_metrics() && !state.metrics.polling {
                state.metrics.polling = true;
                vec![Cmd::PollMetrics]
            } else {
                Vec::new()
            }
        }
        Event::Wake => {
            let now = state.clock.now();
            if state.status.as_ref().is_some_and(|m| m.until <= now) {
                state.status = None;
                // A popup covers the status line, so its message clears
                // without a draw.
                if state.popup.is_none() {
                    state.dirty = true;
                }
            }
            Vec::new()
        }
        Event::Msg(msg) => on_msg(state, msg),
    }
}

fn on_key(state: &mut AppState, key: Key) -> Vec<Cmd> {
    let bindings = screens::bindings(state);
    match keymap::resolve(&bindings, &key) {
        Resolution::Run(action) => run(state, action),
        Resolution::Unavailable(_) | Resolution::Unbound => Vec::new(),
    }
}

fn on_paste(state: &mut AppState, text: &str) -> Vec<Cmd> {
    // A paste types into the focused input, if there is one, and never
    // fires a key.
    let bindings = screens::bindings(state);
    let Some(typed) = bindings
        .iter()
        .find_map(|b| b.typed.filter(|_| b.available()))
    else {
        return Vec::new();
    };
    let mut cmds = Vec::new();
    for c in text.chars() {
        let c = if c.is_control() { ' ' } else { c };
        cmds.extend(run(state, typed(c)));
    }
    cmds
}

/// Runs an action: the shell's own, a popup's or a screen's.
pub fn run(state: &mut AppState, action: Action) -> Vec<Cmd> {
    state.dirty = true;
    match action {
        Action::Interrupt => {
            state.quit = true;
            vec![Cmd::Quit]
        }
        Action::Quit => quit(state),
        Action::Back => {
            back(state);
            Vec::new()
        }
        Action::BackOrQuit => {
            if state.focus == Pane::Detail {
                back(state);
                Vec::new()
            } else {
                quit(state)
            }
        }
        Action::Actions => {
            let pane = state.focus;
            let rows = screens::actions(state, pane);
            let title = format!(
                "Actions {} {} {} {}",
                state.glyphs.dot,
                state.dest.name(),
                state.glyphs.dot,
                pane.word()
            );
            state.open(Popup::Actions(actions::State::new(title, rows)));
            Vec::new()
        }
        Action::Help => {
            help::open(state);
            Vec::new()
        }
        Action::Palette => {
            let rows = screens::palette(state);
            state.open(Popup::Palette(palette::State::new(rows)));
            Vec::new()
        }
        Action::Find => match state.dest {
            Dest::Library => screens::library::find(state),
            _ => Vec::new(),
        },
        Action::Go(dest) => {
            state.close_popup();
            state.dest = dest;
            state.focus = Pane::List;
            Vec::new()
        }
        Action::NextPane | Action::PrevPane => {
            state.focus = state.focus.other();
            Vec::new()
        }
        Action::Suspend => vec![Cmd::Suspend],
        Action::Popup(a) => screens::run_popup(state, a),
        Action::Screen(a) => screens::run(state, a),
    }
}

fn quit(state: &mut AppState) -> Vec<Cmd> {
    match state.can_quit() {
        Ok(()) => {
            state.quit = true;
            vec![Cmd::Quit]
        }
        Err(reason) => {
            state.close_popup();
            state.say(Level::Error, reason);
            Vec::new()
        }
    }
}

/// Back one level: close the popup, then Detail to List. On List in
/// Browse it does nothing. It never quits and never confirms (decision 2).
pub fn back(state: &mut AppState) {
    if state.popup.is_some() {
        state.close_popup();
    } else if state.focus == Pane::Detail {
        state.focus = Pane::List;
        state.dirty = true;
    }
}

fn on_mouse(state: &mut AppState, m: MouseEvent) -> Vec<Cmd> {
    if state.layout().mode == Mode::TooSmall || !state.config.ui.mouse {
        return Vec::new();
    }
    let (x, y) = (m.column, m.row);
    match m.kind {
        MouseEventKind::ScrollDown | MouseEventKind::ScrollUp => {
            let down = m.kind == MouseEventKind::ScrollDown;
            if state.popup.is_some() {
                let key = if down { Key::DOWN } else { Key::UP };
                return on_key(state, key);
            }
            match state.hits.pane_at(x, y) {
                Some(pane) => {
                    state.dirty = true;
                    screens::wheel(state, pane, down)
                }
                None => Vec::new(),
            }
        }
        MouseEventKind::Down(MouseButton::Right) => {
            if state.popup.is_some() {
                return Vec::new();
            }
            if let Some(pane) = state.hits.pane_at(x, y) {
                if let Some(HitTarget::Row { select, .. }) = state.hits.hit(x, y).cloned() {
                    state.focus = pane;
                    run(state, select);
                }
                state.focus = pane;
                return run(state, Action::Actions);
            }
            Vec::new()
        }
        MouseEventKind::Down(MouseButton::Left) => {
            let Some(target) = state.hits.hit(x, y).cloned() else {
                return Vec::new();
            };
            let now = state.clock.now();
            let double = state
                .last_click
                .as_ref()
                .is_some_and(|(t, at)| *t == target && now.duration_since(*at) <= DOUBLE_CLICK);
            state.last_click = Some((target.clone(), now));
            if let Some(pane) = state.hits.pane_at(x, y)
                && state.popup.is_none()
            {
                state.focus = pane;
                state.dirty = true;
            }
            match target {
                HitTarget::Action(a) => run(state, a),
                HitTarget::Row { select, open } => {
                    let mut cmds = run(state, select);
                    if double {
                        state.last_click = None;
                        cmds.extend(run(state, open));
                    }
                    cmds
                }
                HitTarget::Backdrop => {
                    back(state);
                    Vec::new()
                }
                HitTarget::Pane(_) | HitTarget::Popup => Vec::new(),
            }
        }
        _ => Vec::new(),
    }
}

fn on_msg(state: &mut AppState, msg: Msg) -> Vec<Cmd> {
    match msg {
        Msg::Metrics(sample) => {
            let before = metrics_view(state);
            state.metrics.record(*sample);
            if metrics_view(state) != before {
                state.dirty = true;
            }
            Vec::new()
        }
        Msg::ScanDone {
            request,
            result,
            profiles,
        } => screens::library::scan_done(state, request, result, profiles),
        Msg::Saved { token, result } => {
            let Some(owner) = state.saves.remove(&token) else {
                return Vec::new();
            };
            state.dirty = true;
            if result.is_ok() {
                state.saved_once = true;
            }
            screens::saved(state, owner, token, result)
        }
        Msg::ReleaseNotice(notice) => {
            state.release = notice;
            state.dirty = true;
            Vec::new()
        }
        Msg::AutoUpdateNotice(text) => {
            state.say(Level::Info, text);
            Vec::new()
        }
        Msg::HardwareDone { token, result } => {
            screens::monitor::hardware_done(state, token, result)
        }
        Msg::PreviewReady { request, result } => {
            screens::preview::ready(state, request, result);
            Vec::new()
        }
        other => {
            state.dirty = true;
            screens::on_msg(state, other)
        }
    }
}

/// What the frame shows of the metrics, at display precision and with
/// each part's role: the top bar, and on Monitor its panes. A sample that
/// leaves it the same is not drawn.
fn metrics_view(state: &AppState) -> String {
    let mut key: String = crate::view::metrics_parts(state, usize::MAX)
        .into_iter()
        .map(|(t, r)| format!("{t}\u{0}{r:?}\u{0}"))
        .collect();
    if state.dest == Dest::Monitor {
        key.push_str(&screens::monitor::view_key(state));
    }
    key
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testing::{key, state_with_games};

    #[test]
    fn digits_switch_destination_and_return_focus_to_list() {
        let mut s = state_with_games(120, 40, 3);
        s.focus = Pane::Detail;
        update(&mut s, key("4"));
        assert_eq!(s.dest, Dest::Settings);
        assert_eq!(s.focus, Pane::List);
    }

    #[test]
    fn esc_backs_out_and_never_quits() {
        let mut s = state_with_games(120, 40, 3);
        s.focus = Pane::Detail;
        update(&mut s, key("esc"));
        assert_eq!(s.focus, Pane::List);
        let cmds = update(&mut s, key("esc"));
        assert!(cmds.is_empty() && !s.quit);
        update(&mut s, key("0"));
        assert!(matches!(s.popup, Some(Popup::Actions(_))));
        update(&mut s, key("esc"));
        assert!(s.popup.is_none() && !s.quit);
    }

    #[test]
    fn q_backs_then_quits_and_ctrl_c_always_quits() {
        let mut s = state_with_games(120, 40, 3);
        s.focus = Pane::Detail;
        assert!(update(&mut s, key("q")).is_empty());
        assert_eq!(s.focus, Pane::List);
        assert_eq!(update(&mut s, key("q")), vec![Cmd::Quit]);
        let mut small = state_with_games(60, 20, 1);
        assert_eq!(
            update(&mut small, key("q")),
            Vec::new(),
            "below 80x24 only Ctrl-C"
        );
        assert_eq!(update(&mut small, key("ctrl-c")), vec![Cmd::Quit]);
    }

    #[test]
    fn quit_waits_for_a_running_scan() {
        let mut s = state_with_games(120, 40, 3);
        s.jobs.scan = Some(7);
        assert!(update(&mut s, key("Q")).is_empty());
        assert!(!s.quit);
        assert_eq!(
            s.status.as_ref().unwrap().text,
            "wait for pending operations"
        );
    }

    #[test]
    fn a_tick_polls_once_until_the_answer() {
        let mut s = state_with_games(120, 40, 3);
        s.dirty = false;
        assert_eq!(update(&mut s, Event::Tick), vec![Cmd::PollMetrics]);
        assert!(update(&mut s, Event::Tick).is_empty(), "one poll in flight");
        assert!(!s.dirty, "a tick alone draws nothing");
    }
}
