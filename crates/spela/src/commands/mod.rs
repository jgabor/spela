//! One file per command group, so each can be owned and filled on its own.
//!
//! [`crate::cli`] parses the arguments and dispatches to the `run` function
//! here. A command a later phase fills returns
//! [`crate::error::CliError::not_implemented`] until then. Each file owns
//! its own argument types: replacing a placeholder [`crate::cli::Rest`]
//! touches only that file.
//!
//! | File | Commands |
//! |---|---|
//! | [`scan`], [`list`], [`show`] | `scan`, `list`, `show <game>` |
//! | [`launch`] | `launch <game> --dry-run` |
//! | [`profile`] | `profile list`, `create`, `show`, `reset`, `delete`, `follow` |
//! | [`config`] | `config show`, `set` |
//! | [`dll`] | `dll list`, `check-updates`, `update`, `install`, `cache`, `restore` |
//! | [`dlss`], [`proton`] | `show`, `set`, `reset` |
//! | [`gpu`] | `gpu info`, `reset`, `set`, `show`, `profile-reset` |
//! | [`cpu`] | `cpu info`, `governor`, `smt`, `set`, `show`, `profile-reset` |
//! | [`denylist`] | `denylist show`, `check`, `allow`, `deny`, `clear` |
//! | [`profile_read`], [`config_read`], [`dll_read`], [`denylist_read`], [`cpu_read`] | The read subcommands of those groups, apart from their write side |
//! | [`read`] | What the read commands share: config, Steam, the game cache, profiles |
//! | [`tui`] | `tui`, and `--demo` with no command |
//! | [`completion`] | `completion <shell>` |
//! | [`removed`] | the reserved `overlay` and `gui` (decision 45) |
//! | [`profile_write`], [`section_write`], [`config_write`], [`denylist_write`] | the stored-profile, config and deny list writes |
//! | [`dll_write`] | `dll install`, `update` and `restore` |
//! | [`target`] | what the writes share: `<game>` or `--all-games`, exit codes |
//!
//! The hidden `apply-profile` root helper is [`crate::helper`].

pub mod completion;
pub mod config;
pub mod config_read;
pub mod config_write;
pub mod cpu;
pub mod cpu_read;
pub mod denylist;
pub mod denylist_read;
pub mod denylist_write;
pub mod dll;
pub mod dll_read;
pub mod dll_write;
pub mod dlss;
pub mod gpu;
pub mod launch;
pub mod list;
pub mod profile;
pub mod profile_read;
pub mod profile_write;
pub mod proton;
pub mod read;
pub mod removed;
pub mod scan;
pub mod section_write;
pub mod show;
pub mod target;
pub mod tui;
