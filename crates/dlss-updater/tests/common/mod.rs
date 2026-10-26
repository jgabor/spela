//! Helpers for the release contract tests: the repository root and a clean
//! environment for the shell scripts they run.

#![allow(dead_code)]

use std::ffi::OsString;
use std::path::{Path, PathBuf};
use std::process::Command;

/// The repository root.
pub fn root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap()
}

/// A file in the repository, as text.
pub fn read(path: &str) -> String {
    std::fs::read_to_string(root().join(path)).unwrap_or_else(|e| panic!("{path}: {e}"))
}

/// The real `PATH`, for finding bash, git, jq, just and makepkg.
pub fn system_path() -> OsString {
    std::env::var_os("PATH").unwrap_or_default()
}

/// Whether a program is on the real `PATH`.
pub fn has_program(name: &str) -> bool {
    find_program(name).is_some()
}

/// A program's path on the real `PATH`.
pub fn find_program(name: &str) -> Option<PathBuf> {
    std::env::split_paths(&system_path())
        .map(|dir| dir.join(name))
        .find(|p| p.is_file())
}

/// A command with a cleared environment: the real `PATH` behind `extra_path`,
/// and `HOME`, the XDG dirs and `TMPDIR` under `home_root`, so no script
/// reads or writes the user's files.
pub fn clean_command(program: &str, home_root: &Path, extra_path: Option<&Path>) -> Command {
    let mut path = Vec::new();
    if let Some(extra) = extra_path {
        path.push(extra.to_owned());
    }
    path.extend(std::env::split_paths(&system_path()));
    let mut command = Command::new(program);
    command.env_clear();
    command.env("PATH", std::env::join_paths(path).unwrap());
    command.env("LANG", "C");
    command.env("LC_ALL", "C");
    for (key, dir) in [
        ("HOME", "home"),
        ("XDG_CONFIG_HOME", "config"),
        ("XDG_DATA_HOME", "data"),
        ("XDG_CACHE_HOME", "cache"),
        ("XDG_STATE_HOME", "state"),
        ("XDG_RUNTIME_DIR", "runtime"),
        ("TMPDIR", "tmp"),
    ] {
        let dir = home_root.join(dir);
        std::fs::create_dir_all(&dir).unwrap();
        command.env(key, dir);
    }
    command
}
