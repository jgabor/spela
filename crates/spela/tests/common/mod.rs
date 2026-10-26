//! The sandbox every binary test runs in.
//!
//! Each run gets a cleared environment with HOME, every XDG variable and
//! TMPDIR under one temp root, so spela never reads or writes the user's
//! real files. `SPELA_BACKEND=fake` swaps hardware, privilege, display and
//! HTTP for the fake. PATH holds only the sandbox's `bin`, where stubs named
//! `pkexec`, `sudo`, `nvidia-smi`, `kscreen-doctor`, `systemctl` and
//! `scx_loader` write a marker and fail. The proxy variables point at a
//! closed loopback port, so a stray request fails fast instead of leaving
//! the machine. Dropping the sandbox fails the test if any marker exists.

#![allow(dead_code)]

use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};

/// Programs no test may ever run.
pub const FORBIDDEN: &[&str] = &[
    "pkexec",
    "sudo",
    "nvidia-smi",
    "kscreen-doctor",
    "systemctl",
    "scx_loader",
];

/// A loopback port nothing listens on: the discard port.
pub const CLOSED_PROXY: &str = "http://127.0.0.1:9";

/// A temp root with HOME, XDG dirs, TMPDIR and a stub-only PATH.
pub struct Sandbox {
    dir: tempfile::TempDir,
}

impl Sandbox {
    /// A fresh sandbox with the forbidden stubs on PATH.
    pub fn new() -> Sandbox {
        let dir = tempfile::Builder::new()
            .prefix("spela-test-")
            .tempdir()
            .unwrap();
        let s = Sandbox { dir };
        for sub in ["home", "bin", "markers", "tmp"] {
            std::fs::create_dir_all(s.root().join(sub)).unwrap();
        }
        for name in FORBIDDEN {
            s.tripwire(name);
        }
        s
    }

    /// The temp root. Everything spela touches must be under it.
    pub fn root(&self) -> &Path {
        self.dir.path()
    }

    /// `$HOME`.
    pub fn home(&self) -> PathBuf {
        self.root().join("home")
    }

    /// `$XDG_CONFIG_HOME/spela`.
    pub fn config_dir(&self) -> PathBuf {
        self.root().join("xdg-config/spela")
    }

    /// `$XDG_STATE_HOME/spela`.
    pub fn state_dir(&self) -> PathBuf {
        self.root().join("xdg-state/spela")
    }

    /// `$TMPDIR`.
    pub fn tmp(&self) -> PathBuf {
        self.root().join("tmp")
    }

    /// The folder on PATH.
    pub fn bin(&self) -> PathBuf {
        self.root().join("bin")
    }

    /// Puts an executable named `name` on PATH that writes a marker and
    /// fails if anything runs it. Returns its path.
    pub fn tripwire(&self, name: &str) -> PathBuf {
        let marker = self.root().join("markers").join(name);
        let script = format!(
            "#!/bin/sh\necho \"$0 $*\" >> '{}'\nexit 97\n",
            marker.display()
        );
        let path = self.bin().join(name);
        std::fs::write(&path, script).unwrap();
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o755)).unwrap();
        path
    }

    /// The names of tripwires that ran.
    pub fn markers(&self) -> Vec<String> {
        let mut names: Vec<String> = std::fs::read_dir(self.root().join("markers"))
            .unwrap()
            .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
            .collect();
        names.sort();
        names
    }

    /// Clears the markers, for the test that proves the tripwires work.
    pub fn take_markers(&self) -> Vec<String> {
        let names = self.markers();
        for n in &names {
            std::fs::remove_file(self.root().join("markers").join(n)).unwrap();
        }
        names
    }

    /// A `spela` command with the sandbox environment and nothing else.
    pub fn command(&self) -> Command {
        let mut cmd = Command::new(env!("CARGO_BIN_EXE_spela"));
        cmd.env_clear()
            .current_dir(self.root())
            .env("HOME", self.home())
            .env("XDG_CONFIG_HOME", self.root().join("xdg-config"))
            .env("XDG_DATA_HOME", self.root().join("xdg-data"))
            .env("XDG_CACHE_HOME", self.root().join("xdg-cache"))
            .env("XDG_STATE_HOME", self.root().join("xdg-state"))
            .env("XDG_RUNTIME_DIR", self.root().join("xdg-runtime"))
            .env("TMPDIR", self.tmp())
            .env("PATH", self.bin())
            .env("SPELA_BACKEND", "fake")
            .env("LC_ALL", "C");
        for proxy in [
            "HTTPS_PROXY",
            "HTTP_PROXY",
            "ALL_PROXY",
            "https_proxy",
            "http_proxy",
            "all_proxy",
        ] {
            cmd.env(proxy, CLOSED_PROXY);
        }
        cmd.env("NO_PROXY", "127.0.0.1,localhost")
            .env("no_proxy", "127.0.0.1,localhost")
            .stdin(std::process::Stdio::null());
        cmd
    }

    /// The same command as an `assert_cmd` command, for fluent checks.
    pub fn spela(&self) -> assert_cmd::Command {
        assert_cmd::Command::from_std(self.command())
    }

    /// Runs `spela args` with extra variables and returns its output.
    pub fn run_with(&self, args: &[&str], env: &[(&str, &str)]) -> Run {
        let mut cmd = self.command();
        cmd.args(args);
        for (k, v) in env {
            cmd.env(k, v);
        }
        Run(cmd.output().unwrap())
    }

    /// Runs `spela args` and returns its output.
    pub fn run(&self, args: &[&str]) -> Run {
        self.run_with(args, &[])
    }

    /// Fails unless `path` is under the root.
    pub fn assert_inside(&self, path: &Path) {
        assert!(
            path.starts_with(self.root()),
            "{} escaped the sandbox {}",
            path.display(),
            self.root().display()
        );
    }

    /// Every file and folder under the root, relative to it, sorted.
    pub fn tree(&self) -> Vec<String> {
        let mut out = Vec::new();
        walk(self.root(), self.root(), &mut out);
        out.sort();
        out
    }
}

impl Default for Sandbox {
    fn default() -> Self {
        Self::new()
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        if !std::thread::panicking() {
            let ran = self.markers();
            assert!(ran.is_empty(), "forbidden programs ran: {ran:?}");
        }
    }
}

fn walk(root: &Path, dir: &Path, out: &mut Vec<String>) {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        out.push(path.strip_prefix(root).unwrap().display().to_string());
        if entry.file_type().is_ok_and(|t| t.is_dir()) {
            walk(root, &path, out);
        }
    }
}

/// One finished run.
pub struct Run(pub Output);

impl Run {
    /// The exit code.
    pub fn code(&self) -> Option<i32> {
        self.0.status.code()
    }

    /// stdout as text.
    pub fn stdout(&self) -> String {
        String::from_utf8_lossy(&self.0.stdout).into_owned()
    }

    /// stderr as text.
    pub fn stderr(&self) -> String {
        String::from_utf8_lossy(&self.0.stderr).into_owned()
    }

    /// Fails unless the exit code is `code`, showing both streams.
    pub fn expect_code(&self, code: i32) -> &Self {
        assert_eq!(
            self.code(),
            Some(code),
            "stdout: {}\nstderr: {}",
            self.stdout(),
            self.stderr()
        );
        assert!(!self.stderr().contains("panicked"), "{}", self.stderr());
        self
    }
}

/// The token Go's absolute paths carry in `tests/fixtures/go` (its README).
pub const FIXTURE_ROOT: &str = "/fixture-root";

/// `tests/fixtures/go`.
pub fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../tests/fixtures/go")
}

/// Copies a fixture folder into `to`, replacing [`FIXTURE_ROOT`] with `root`
/// in text files. A file with a NUL byte or that is not UTF-8, such as a PE
/// stub, is copied byte for byte.
pub fn copy_fixture(from: &Path, to: &Path, root: &str) {
    std::fs::create_dir_all(to).unwrap();
    for entry in std::fs::read_dir(from).unwrap() {
        let entry = entry.unwrap();
        let target = to.join(entry.file_name());
        let kind = entry.file_type().unwrap();
        if kind.is_dir() {
            copy_fixture(&entry.path(), &target, root);
            continue;
        }
        let bytes = std::fs::read(entry.path()).unwrap();
        let bytes = match String::from_utf8(bytes) {
            Ok(text) if !text.contains('\0') => text.replace(FIXTURE_ROOT, root).into_bytes(),
            Ok(text) => text.into_bytes(),
            Err(e) => e.into_bytes(),
        };
        std::fs::write(&target, bytes).unwrap();
    }
}

/// Writes an executable script through `/bin/cp`, so the test process
/// never holds a write descriptor to it that a parallel test's fork could
/// inherit, which would make running it fail with "text file busy".
pub fn write_script(path: &Path, text: impl AsRef<[u8]>) {
    let source = path.with_file_name(format!(
        "{}.text",
        path.file_name().unwrap().to_string_lossy()
    ));
    std::fs::write(&source, text).unwrap();
    let copied = Command::new("/bin/cp")
        .arg(&source)
        .arg(path)
        .status()
        .unwrap();
    assert!(copied.success());
    std::fs::remove_file(&source).unwrap();
    std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o755)).unwrap();
}
