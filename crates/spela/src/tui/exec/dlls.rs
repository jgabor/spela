//! The dlls screen's commands, run off the UI thread: loads on the job
//! pool, writes through `Workers::writes`, results back as
//! `Msg::Dlls`.

use spela_ui::screens::dlls::Cmd;

use super::Workers;

/// Runs one of the screen's commands. It has none yet.
pub fn run(_w: &mut Workers, cmd: Cmd) {
    match cmd {}
}
