//! Clicks give the same actions as keys (terminal-ux.md, mouse): the
//! frame's hit map turns a click on a destination, a hint, a row or a
//! popup button into that action, a double click opens a row like Enter,
//! right click opens Actions and the wheel moves the pane under it.

use crossterm::event::{KeyModifiers, MouseButton, MouseEvent, MouseEventKind};
use spela_ui::event::Event;
use spela_ui::layout::Pane;
use spela_ui::screens::{Dest, Popup, library};
use spela_ui::testing::{render_text, state_with_games};
use spela_ui::update;

fn mouse(kind: MouseEventKind, column: u16, row: u16) -> Event {
    Event::Mouse(MouseEvent {
        kind,
        column,
        row,
        modifiers: KeyModifiers::NONE,
    })
}

fn click(column: u16, row: u16) -> Event {
    mouse(MouseEventKind::Down(MouseButton::Left), column, row)
}

#[test]
fn a_destination_click_goes_there() {
    let mut s = state_with_games(120, 40, 5);
    let text = render_text(&mut s);
    let x = text.lines().next().unwrap().find("Monitor").unwrap();
    update(
        &mut s,
        click(
            u16::try_from(text.lines().next().unwrap()[..x].chars().count()).unwrap(),
            0,
        ),
    );
    assert_eq!(s.dest, Dest::Monitor);
}

#[test]
fn a_hint_click_runs_its_key() {
    let mut s = state_with_games(120, 40, 5);
    let text = render_text(&mut s);
    let bar = text.lines().nth(39).unwrap();
    let at = bar.find("? Help").unwrap();
    update(
        &mut s,
        click(u16::try_from(bar[..at].chars().count()).unwrap(), 39),
    );
    assert!(matches!(s.popup, Some(Popup::Help(_))));
    // A click outside the popup closes it.
    render_text(&mut s);
    update(&mut s, click(0, 38));
    assert!(s.popup.is_none());
}

#[test]
fn rows_select_and_double_clicks_open() {
    let mut s = state_with_games(120, 40, 5);
    render_text(&mut s);
    // Screen row 5 is list row 2, the second game: Find is row 2 and All
    // games row 3 on screen.
    update(&mut s, click(5, 5));
    assert_eq!(s.library.cursor, 2);
    assert_eq!(s.focus, Pane::List);
    render_text(&mut s);
    update(&mut s, click(5, 5));
    assert_eq!(s.focus, Pane::Detail, "a double click opens, like Enter");
    let _ = library::selected_game(&s).unwrap();
}

#[test]
fn right_click_opens_actions_and_the_wheel_moves() {
    let mut s = state_with_games(120, 40, 12);
    render_text(&mut s);
    update(&mut s, mouse(MouseEventKind::ScrollDown, 5, 10));
    assert_eq!(s.library.cursor, 4, "three rows down from the first game");
    render_text(&mut s);
    update(
        &mut s,
        mouse(MouseEventKind::Down(MouseButton::Right), 5, 4),
    );
    assert!(matches!(s.popup, Some(Popup::Actions(_))));
    assert_eq!(s.library.cursor, 1, "the row under the pointer");
}

#[test]
fn mouse_off_ignores_clicks() {
    let mut s = state_with_games(120, 40, 5);
    s.config.ui.mouse = false;
    render_text(&mut s);
    update(&mut s, click(5, 5));
    assert_eq!(s.library.cursor, 1);
}
