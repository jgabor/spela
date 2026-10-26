//! The root helper's core: the protocol, the checks, the apply and restore
//! rules, and the two loops (decision 30; architecture.md, Privilege model
//! and Protocol).
//!
//! The `spela` binary runs these loops as the hidden `apply-profile`
//! command under polkit, with NVML and `/sys`. The fake privilege runner
//! runs the same loops in process over pipes, against the fake GPU control
//! and a temp sysfs tree, so its tests check the helper the wrapper really
//! starts. Nothing here reads the environment, resolves an XDG path or
//! writes a log file: the helper runs as root with root's environment.
//!
//! | Module | Role |
//! |---|---|
//! | [`protocol`] | The JSON lines, version 1, parsed strictly |
//! | [`machine`] | What the helper changes: [`machine::Machine`] and the real [`machine::SystemMachine`] |
//! | [`serve`] | The `--serve` and `--once` loops |
//! | [`client`] | The parent's side: send, read, wait, restore |

pub mod client;
pub mod machine;
pub mod protocol;
pub mod serve;
