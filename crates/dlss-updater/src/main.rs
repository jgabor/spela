//! Keeps spela's DLL manifest current. Runs in CI, not on users' machines.
//! See the library for what each command does.

use std::process::ExitCode;

use dlss_updater::fetch::Client;
use jiff::Timestamp;

fn main() -> ExitCode {
    let mut args = std::env::args_os().map(|a| a.to_string_lossy().into_owned());
    let program = args.next().unwrap_or_else(|| "dlss-updater".to_owned());
    let args: Vec<String> = args.collect();
    let client = Client::new();
    let code = dlss_updater::run(
        &program,
        &args,
        &mut std::io::stdout().lock(),
        &mut std::io::stderr().lock(),
        &|source| client.latest(source),
        Timestamp::now(),
    );
    ExitCode::from(code)
}
