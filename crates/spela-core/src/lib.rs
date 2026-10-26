//! spela's domain and its seams.
//!
//! Phase 1 holds the foundations every later part builds on:
//!
//! | Module | Role |
//! |---|---|
//! | [`env`] | The environment, read once. Nothing else reads `std::env` |
//! | [`paths`] | Config, data, cache and state directories, spec-strict XDG |
//! | [`clock`] | The injected clock |
//! | [`fs`] | Writing a file atomically, with fsync for user intent |
//! | [`lock`] | flock instance locks and store locks |
//! | [`hex`] | SHA-256 and hex helpers |
//! | [`backend`] | The traits for GPU, display, privilege and HTTP |
//! | [`fake`] | The fake behind every trait, shared by tests and `--demo` |

#![warn(missing_docs)]

pub mod backend;
pub mod clock;
pub mod env;
pub mod fake;
pub mod fs;
pub mod hex;
pub mod lock;
pub mod paths;

/// spela's version, the workspace version.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
