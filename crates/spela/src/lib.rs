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
//! | [`router`] | Subcommand or wrapped game, decided before clap runs |
//! | [`error`] | [`error::CliError`] and the exit codes |
//! | [`output`] | `error:` and `hint:` lines, colour, the `--json` error object |
//! | [`logging`] | The rolling log file and stderr warnings |
//! | [`locks`] | The TUI lock and the launch lock |
//! | [`backends`] | The `SPELA_BACKEND` selector and the real backends |
//! | [`privilege`] | The real privilege backend, the only code that names its program |
//! | [`helper`] | The hidden `apply-profile` root helper |
//! | [`demo`] | `--demo`: a private temp dir and the fake backends |
//! | [`startup`] | The migration from Go spela before a command reads the store |
//! | [`text`] | Colour roles for command output |

#![warn(missing_docs)]

pub mod backends;
pub mod cli;
pub mod commands;
pub mod demo;
pub mod error;
pub mod helper;
pub mod locks;
pub mod logging;
pub mod output;
pub mod privilege;
pub mod router;
pub mod startup;
pub mod text;
