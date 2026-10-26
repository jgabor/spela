//! Test support, for this crate's tests, the TUI snapshot tests and the
//! screen agents' tests: a state on fictional games with a manual clock,
//! keys by name, and a frame drawn on ratatui's `TestBackend` as text.
//!
//! Nothing here does I/O: the games are built in memory.

use std::path::PathBuf;
use std::sync::Arc;

use ratatui::Terminal;
use ratatui::backend::TestBackend;
use spela_core::clock::ManualClock;
use spela_core::config::Config;
use spela_core::games::{DetectedDll, DllType, Game, GameCache};

use crate::event::Event;
use crate::keymap::Key;
use crate::state::{AppState, Startup};

/// The time every test state starts at.
pub const NOW: &str = "2026-10-26T17:02:10Z";

/// Fictional game names, sorted (decision 43).
pub const NAMES: &[&str] = &[
    "Ashfall Courier",
    "Brightwater Keep",
    "Cinder Valley Racing",
    "Deepline Salvage",
    "Ember Tide",
    "Game 1 Remastered",
    "Glasshouse Detective",
    "Hollow Crown Tactics: Definitive Edition",
    "Ironleaf Expedition",
    "Lantern Drift",
    "Moonwell Farmstead",
    "Nightglass Protocol",
];

/// A manual clock at [`NOW`].
pub fn clock() -> Arc<ManualClock> {
    Arc::new(ManualClock::new(NOW.parse().expect("a valid time")))
}

/// `n` fictional games, with DLLs on every other one, app ids from
/// 9100010 up.
pub fn games(n: usize) -> GameCache {
    let mut cache = GameCache::default();
    for i in 0..n {
        let name = NAMES
            .get(i)
            .map_or_else(|| format!("Fixture Game {i:03}"), |s| (*s).to_owned());
        let app_id = 9_100_010 + 10 * i as u64;
        let dir = PathBuf::from("/fixture/steamapps/common").join(&name);
        let dlls = if i % 2 == 0 {
            vec![DetectedDll {
                path: dir.join("nvngx_dlss.dll"),
                name: "nvngx_dlss.dll".into(),
                dll_type: DllType::Dlss,
                version: Some("310.4.0".into()),
            }]
        } else {
            Vec::new()
        };
        cache.upsert(Game {
            app_id,
            name,
            install_dir: dir,
            library_path: PathBuf::from("/fixture"),
            prefix_path: None,
            scanned_at: NOW.parse().expect("a valid time"),
            dlls,
        });
    }
    cache
}

/// A state at `width` by `height` on `n` games, with every third game
/// owning a profile, as `spela tui` would start it.
pub fn state_with_games(width: u16, height: u16, n: usize) -> AppState {
    let cache = games(n);
    let profiles = cache.games.iter().step_by(3).map(|g| g.app_id).collect();
    AppState::new(Startup {
        size: (width, height),
        config: Config::default(),
        games: cache,
        profiles,
        broken_profiles: Default::default(),
        notice: Vec::new(),
        warnings: Vec::new(),
        no_color: false,
        demo: false,
        clock: clock(),
    })
}

/// A key event by name: `q`, `Q`, `enter`, `esc`, `shift-tab`, `ctrl-k`.
pub fn key(name: &str) -> Event {
    Event::Key(name.parse::<Key>().expect("a key name"))
}

/// Draws a frame and returns its text, one line per row.
pub fn render_text(state: &mut AppState) -> String {
    let (w, h) = state.size;
    let mut terminal = Terminal::new(TestBackend::new(w, h)).expect("a test terminal");
    terminal
        .draw(|f| crate::view::render(state, f))
        .expect("a frame");
    let buf = terminal.backend().buffer();
    let mut out = String::new();
    for y in 0..h {
        let mut line = String::new();
        for x in 0..w {
            line.push_str(buf[(x, y)].symbol());
        }
        out.push_str(line.trim_end());
        out.push('\n');
    }
    out
}
