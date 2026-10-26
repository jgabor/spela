//! `spela completion <shell>`: a static completion script (decision 57).

use std::io::Write;

use clap::CommandFactory;
use clap_complete::Shell;

use crate::cli::Cli;
use crate::error::CliError;

/// Prints the completion script for `shell`.
pub fn run(shell: Shell, out: &mut dyn Write) -> Result<(), CliError> {
    // clap_complete panics on a write error, so it writes to memory and the
    // copy to stdout handles a closed pipe.
    let mut buf = Vec::new();
    clap_complete::generate(shell, &mut visible_command(), "spela", &mut buf);
    out.write_all(&buf).map_err(CliError::from_stdout)
}

/// The command tree without hidden subcommands. clap_complete offers hidden
/// ones too, and `apply-profile`, `overlay` and `gui` are not for tab
/// completion.
fn visible_command() -> clap::Command {
    let root = Cli::command();
    clap::Command::new("spela")
        .version(spela_core::VERSION)
        .disable_version_flag(true)
        .args(root.get_arguments().cloned())
        .subcommands(root.get_subcommands().filter(|c| !c.is_hide_set()).cloned())
}
