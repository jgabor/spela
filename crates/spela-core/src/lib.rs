//! spela's domain and its seams.
//!
//! The foundations:
//!
//! | Module | Role |
//! |---|---|
//! | [`env`] | The environment, read once. Nothing else reads `std::env` |
//! | [`paths`] | Config, data, cache and state directories, spec-strict XDG, and every file in them |
//! | [`clock`] | The injected clock |
//! | [`fs`] | Writing a file atomically, with fsync for user intent |
//! | [`lock`] | flock instance locks and store locks |
//! | [`hex`] | SHA-256 and hex helpers |
//! | [`backend`] | The traits for GPU, CPU reads, display, privilege and HTTP |
//! | [`fake`] | The fake behind every trait, shared by tests and `--demo` |
//!
//! The domain:
//!
//! | Module | Role |
//! |---|---|
//! | [`profile`] | The field table, profiles, All games inheritance and its words |
//! | [`config`] | `config.toml`: schema, defaults, validation and settings |
//! | [`games`] | The game cache and finding a game |
//! | [`steam`] | Steam libraries, app manifests and VDF |
//! | [`proton`] | The Proton resolver |
//! | [`pe`] | PE version reading |
//! | [`dll`] | DLL detection, the manifest and its fetch, versions, staleness, backups, and install, update and restore |
//! | [`denylist`] | The anti-cheat deny list |
//! | [`release`] | The release check against GitHub's tags |
//! | [`migrate`] | Reading Go spela's YAML and migrating it |
//! | [`metrics`] | CPU and memory metrics from procfs text |
//! | [`alerts`] | The GPU alert evaluator |
//! | [`launch`] | Launch planning: the environment, the hardware request, the dry-run summary, game detection and KDE VRR |
//! | [`kscreen`] | Reading `kscreen-doctor -o` |

#![warn(missing_docs)]

pub mod alerts;
pub mod backend;
pub mod clock;
pub mod config;
pub mod denylist;
pub mod dll;
pub mod env;
pub mod fake;
pub mod fs;
pub mod games;
pub mod hex;
pub mod kscreen;
pub mod launch;
pub mod lock;
pub mod metrics;
pub mod migrate;
pub mod paths;
pub mod pe;
pub mod profile;
pub mod proton;
pub mod release;
pub mod steam;

/// spela's version, the workspace version.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
