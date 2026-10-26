//! spela's TUI: state, update and view, with no I/O.
//!
//! This crate never touches files, processes, the network or the
//! environment. The shell in the `spela` binary owns the terminal, the
//! threads and the backends, and feeds this crate events and results. A
//! safety test fails when this source names `std::fs`, `std::process`,
//! `std::net` or `std::env`, and when this crate depends on a backend
//! crate.
//!
//! Phase 4 fills it: `AppState`, `update`, `render`, the keymap, layout,
//! hit map, theme and text width.

#![warn(missing_docs)]
