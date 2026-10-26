//! The `spela` executable. All logic lives in the library; see `cli`.

fn main() -> std::process::ExitCode {
    spela::cli::main()
}
