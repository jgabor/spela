//! DLSS, XeSS and FSR DLLs: detection, the manifest, versions, staleness and
//! backups.

pub mod backup;
pub mod detect;
mod go_json;
pub mod manifest;
pub mod stale;
pub mod version;
