//! The wall-clock budgets of terminal-ux.md (budgets), measured on release
//! builds and reported, not gated (decision 55; architecture.md,
//! performance budgets). Debug builds say nothing about speed, so these
//! are ignored in the normal gate; CI runs them in a release build:
//!
//! ```text
//! cargo test --release --locked -p spela --test perf -- --ignored --nocapture
//! ```
//!
//! Each test asserts only its deterministic counts. The times go to stderr
//! with their budget and buckets. The first-frame budget is reported by the
//! pty scenario in `terminal_pty.rs`, which spawns the real binary.

use std::time::{Duration, Instant};

use crossterm::event::{Event as TermEvent, KeyCode, KeyEvent, KeyModifiers};
use ratatui::backend::CrosstermBackend;
use ratatui::layout::Rect;
use ratatui::{Terminal, TerminalOptions, Viewport};
use spela::tui::app::App;
use spela::tui::events::{LoopEvent, TimedEvent};
use spela::tui::exec::Recorder;
use spela::tui::host::RecordingHost;
use spela_ui::screens::{self, palette};
use spela_ui::testing::state_with_games;

const GAMES: usize = 1_000;

fn key(code: KeyCode) -> LoopEvent<spela_ui::Msg> {
    LoopEvent::Input(vec![TimedEvent {
        event: TermEvent::Key(KeyEvent::new(code, KeyModifiers::NONE)),
        at: Instant::now(),
    }])
}

fn report(what: &str, budget: Duration, p: Duration, samples: usize) {
    let verdict = if p <= budget { "within" } else { "OVER" };
    eprintln!("perf: {what}: {p:?} over {samples} samples, budget {budget:?}: {verdict}");
}

/// Key to bytes flushed, p99 under 8 ms: the real loop with the crossterm
/// backend writing to a byte sink plays 1,000 keys over Library, a profile
/// field, Settings and the palette on 1,000 games.
#[test]
#[ignore = "a release-mode budget report; CI runs it with --release --ignored"]
fn key_to_flush_over_a_thousand_games() {
    let backend = CrosstermBackend::new(std::io::sink());
    let terminal = Terminal::with_options(
        backend,
        TerminalOptions {
            viewport: Viewport::Fixed(Rect::new(0, 0, 120, 40)),
        },
    )
    .unwrap();
    let mut app = App::new(
        state_with_games(120, 40, GAMES),
        terminal,
        Box::new(RecordingHost::default()),
        Box::new(Recorder::default()),
    );
    app.start().unwrap();
    use KeyCode::{Char, Down, Enter, Esc, Up};
    let round = [
        Down,
        Down,
        Down,
        Up,
        Enter,
        Down,
        Down,
        Esc,
        Char('4'),
        Down,
        Down,
        Char('1'),
        Char(':'),
        Char('s'),
        Char('o'),
        Char('r'),
        Esc,
        Char('0'),
        Down,
        Esc,
    ];
    let mut sent = 0;
    while sent < 1_000 {
        for code in round {
            app.handle_batch(vec![key(code)]).unwrap();
            sent += 1;
        }
    }
    let s = app.state.perf.summary();
    // A key that changes nothing draws nothing and so has no sample, such
    // as Esc on the List.
    assert!(s.total > 0 && s.total <= 1_000, "{}", s.total);
    report("key to flush p99", Duration::from_millis(8), s.p99, s.count);
    eprintln!("perf: key to flush buckets: {:?}", s.buckets);
}

/// The palette's match per keystroke, under 1 ms over every Actions row,
/// game and setting with 1,000 games.
#[test]
#[ignore = "a release-mode budget report; CI runs it with --release --ignored"]
fn palette_match_per_keystroke_over_a_thousand_games() {
    let state = state_with_games(120, 40, GAMES);
    let rows = screens::palette(&state);
    assert!(rows.len() > GAMES, "every game is a row");
    let mut popup = palette::State::new(rows);
    let mut times = Vec::new();
    for round in 0..20 {
        popup.input.clear();
        for c in "fixture game 9"
            .chars()
            .chain(std::iter::once(char::from(b'0' + round % 10)))
        {
            popup.input.handle(&spela_ui::keymap::Key::char(c));
            let at = Instant::now();
            popup.refilter();
            times.push(at.elapsed());
        }
    }
    times.sort_unstable();
    let p99 = times[(times.len() * 99).div_ceil(100) - 1];
    report(
        "palette match p99",
        Duration::from_millis(1),
        p99,
        times.len(),
    );
}
