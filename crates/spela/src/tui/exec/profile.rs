//! The profile screen's commands, run off the UI thread: loads on the job
//! pool, writes through `Workers::writes`, results back as
//! `Msg::Profile`.

use spela_ui::screens::profile::Cmd;

use super::Workers;

/// Runs one of the screen's commands. It has none yet.
pub fn run(_w: &mut Workers, cmd: Cmd) {
    match cmd {}
}
