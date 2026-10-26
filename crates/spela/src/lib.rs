//! The spela binary's building blocks.
//!
//! Everything the `spela` executable does lives here as a library, so each
//! piece can be tested without a terminal, real hardware or the user's real
//! directories. `main.rs` only calls [`cli::main`].
//!
//! | Module | Role |
//! |---|---|
//! | [`cli`] | Arguments, commands, the process entry point and the [`cli::run`] seam |
//! | [`commands`] | One file per command group, with its arguments and its `run` |
//! | [`wrapper`] | Wrapper mode: `spela %command%` runs a game with its profile |
//! | [`router`] | Subcommand or wrapped game, decided before clap runs |
//! | [`error`] | [`error::CliError`] and the exit codes |
//! | [`json`] | `--json` output, schema version 1 |
//! | [`output`] | `error:` and `hint:` lines, colour, the `--json` error object |
//! | [`logging`] | The rolling log file and stderr warnings |
//! | [`locks`] | The TUI lock and the launch lock |
//! | [`backends`] | The `SPELA_BACKEND` selector and the real backends |
//! | [`display`] | The real KDE display backend, `kscreen-doctor` |
//! | [`privilege`] | The real privilege backend, the only code that names its program |
//! | [`helper`] | The hidden `apply-profile` root helper |
//! | [`plan`] | What the dry run, the wrapper and Preview launch share to build a launch plan |
//! | [`migration`] | The migration from Go spela, run before a command reads the store |
//! | [`demo`] | `--demo`: a private temp dir and the fake backends |
//! | [`text`] | Colour roles for command output |
//! | [`tui`] | The TUI shell: terminal, threads and the loop around `spela-ui` |

#![warn(missing_docs)]

pub mod backends;
pub mod cli;
pub mod commands;
pub mod demo;
pub mod display;
pub mod error;
pub mod helper;
pub mod json;
pub mod locks;
pub mod logging;
pub mod migration;
pub mod output;
pub mod plan;
pub mod privilege;
pub mod router;
pub mod text;
pub mod tui;
pub mod wrapper;
