//! DLSS, XeSS and FSR DLLs.
//!
//! | Module | Role |
//! |---|---|
//! | [`detect`] | Which DLL families a game folder holds |
//! | [`version`] | The shared dotted version compare |
//! | [`manifest`] | The manifest's types, its cache and the DLL cache paths |
//! | [`fetch`] | The manifest under its refresh policy, and checksum-verified downloads |
//! | [`stale`] | The one answer to "is there an update" (decision 38) |
//! | [`backup`] | Reading `backup.json` and planning a restore |
//! | [`swap`] | Backups before a swap, the swap and the containment checks |
//! | [`ops`] | Install, update, batch update and restore |

pub mod backup;
pub mod detect;
pub mod fetch;
mod go_json;
pub mod manifest;
pub mod ops;
pub mod stale;
pub mod swap;
pub mod version;
