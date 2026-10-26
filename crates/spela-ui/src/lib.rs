//! spela's TUI: state, update and view, with no I/O.
//!
//! This crate never touches files, processes, the network or the
//! environment. The shell in the `spela` binary owns the terminal, the
//! threads and the backends, and feeds this crate events and results. A
//! safety test fails when this source names `std::fs`, `std::process`,
//! `std::net` or `std::env`, and when this crate depends on a backend
//! crate.
//!
//! | Module | Role |
//! |---|---|
//! | [`state`] | [`AppState`]: what every screen shares, and one field per screen |
//! | [`event`] | [`Event`], [`Msg`] and [`Cmd`]: what reaches update, and what it asks the shell |
//! | [`update`] | [`update()`]: the one place state changes |
//! | [`view`] | [`render`]: the frame, the top bar, the status line and the hint bar |
//! | [`screens`] | One file per destination and popup, registered in one place |
//! | [`keymap`] | Keys, bindings and the resolver the hint bar and Help read too |
//! | [`layout`] | The frame's geometry: the 80x24 floor and the 100x30 breakpoint |
//! | [`hit`] | Click regions recorded while drawing |
//! | [`theme`] | `terminal`, `neon` and `mono`, and the glyph sets |
//! | [`paint`] | Drawing helpers the screens share |
//! | [`text`] | Display width, sanitising, truncation and wrapping |
//! | [`input`] | [`input::LineInput`], one line of editable text |
//! | [`editor`] | [`editor::Editor`], a one-line editor with Apply and Cancel, for Settings and the deny reason |
//! | [`fuzzy`] | The palette's matcher |
//! | [`picker`] | The choice picker with type-to-jump, for editors and Settings |
//! | [`perf`] | The key-to-flush ring |
//! | [`testing`] | Test support: fictional games, a manual clock, frames as text |

#![warn(missing_docs)]

pub mod editor;
pub mod event;
pub mod fuzzy;
pub mod hit;
pub mod input;
pub mod keymap;
pub mod layout;
pub mod paint;
pub mod perf;
pub mod picker;
pub mod screens;
pub mod state;
pub mod testing;
pub mod text;
pub mod theme;
pub mod update;
pub mod view;

pub use event::{Cmd, Event, Msg};
pub use state::{AppState, Startup};
pub use update::update;
pub use view::render;
