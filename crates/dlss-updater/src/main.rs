//! Keeps spela's DLL manifest current. Runs in CI, not on users' machines.
//!
//! The port of Go's `tools/dlss-updater` lands in Phase 7 (decision 14).
//! Until then this stub says so and fails, so a workflow that calls it by
//! mistake cannot pass.

use std::io::Write;
use std::process::ExitCode;

fn main() -> ExitCode {
    let _ = writeln!(
        std::io::stderr(),
        "error: dlss-updater {} is not yet implemented in this build",
        spela_core::VERSION
    );
    ExitCode::FAILURE
}
