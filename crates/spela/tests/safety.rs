//! The hard rules in AGENTS.md, as tests that fail when someone breaks one.
//!
//! The source gates read every `crates/*/src` file and skip comments. The
//! privilege, environment and network gates read the inline test module
//! too, because unit tests run with the developer's real PATH, HOME and
//! XDG dirs. The production part of a file ends only at the column-0
//! `#[cfg(test)]` on the `mod tests` that AGENTS.md puts last, so an
//! indented `#[cfg(test)]` item hides nothing. Binary tests in
//! `crates/*/tests` and any `build.rs` are scanned for a forbidden program
//! named by an absolute path or run by its bare name, which the sandbox's
//! PATH tripwires cannot catch outside the sandbox.

mod common;

use std::os::unix::fs::PermissionsExt;

use std::path::{Path, PathBuf};

use common::{FORBIDDEN, Sandbox};

/// The workspace root.
fn workspace() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .unwrap()
}

/// Every `.rs` file under `crates/*/src`, relative to the workspace.
fn sources() -> Vec<(String, String)> {
    let out = read_all(|krate, files| collect(&krate.join("src"), files));
    assert!(out.len() > 10, "found too few sources: {out:?}");
    out
}

/// Every `.rs` file under `crates/*/tests`, and every `crates/*/build.rs`.
fn test_sources() -> Vec<(String, String)> {
    let out = read_all(|krate, files| {
        collect(&krate.join("tests"), files);
        let build = krate.join("build.rs");
        if build.is_file() {
            files.push(build);
        }
    });
    assert!(
        out.iter().any(|(p, _)| p == "crates/spela/tests/safety.rs"),
        "{out:?}"
    );
    out
}

fn read_all(find: impl Fn(&Path, &mut Vec<PathBuf>)) -> Vec<(String, String)> {
    let root = workspace();
    let mut files = Vec::new();
    for krate in std::fs::read_dir(root.join("crates")).unwrap() {
        find(&krate.unwrap().path(), &mut files);
    }
    let mut out: Vec<(String, String)> = files
        .into_iter()
        .map(|p| {
            let rel = p.strip_prefix(&root).unwrap().display().to_string();
            (rel, std::fs::read_to_string(&p).unwrap())
        })
        .collect();
    out.sort();
    out
}

fn collect(dir: &Path, out: &mut Vec<PathBuf>) {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect(&path, out);
        } else if path.extension().is_some_and(|e| e == "rs") {
            out.push(path);
        }
    }
}

/// Where a file's inline test module starts: the line index of a column-0
/// `#[cfg(test)]` whose next line starts with `mod `.
fn test_module_start(lines: &[&str]) -> Option<usize> {
    lines
        .windows(2)
        .position(|w| w[0] == "#[cfg(test)]" && w[1].starts_with("mod "))
}

/// The production lines of a file: everything before the inline test
/// module, with comments removed. Returns (line number, code).
fn code_lines(text: &str) -> Vec<(usize, String)> {
    let lines: Vec<&str> = text.lines().collect();
    let end = test_module_start(&lines).unwrap_or(lines.len());
    strip_comments(&lines[..end])
}

/// Every line of a file, the inline test module included, with comments
/// removed.
fn all_code_lines(text: &str) -> Vec<(usize, String)> {
    let lines: Vec<&str> = text.lines().collect();
    strip_comments(&lines)
}

fn strip_comments(lines: &[&str]) -> Vec<(usize, String)> {
    let mut out = Vec::new();
    for (i, line) in lines.iter().enumerate() {
        if line.trim_start().starts_with("//") {
            continue;
        }
        let code = match line.find(" //") {
            Some(at) => &line[..at],
            None => line,
        };
        out.push((i + 1, code.to_owned()));
    }
    out
}

fn has_word(line: &str, word: &str) -> bool {
    line.match_indices(word).any(|(at, _)| {
        let before = line[..at].chars().next_back();
        let after = line[at + word.len()..].chars().next();
        let edge = |c: Option<char>| c.is_none_or(|c| !(c.is_alphanumeric() || c == '_'));
        edge(before) && edge(after)
    })
}

/// Lines in `files` outside `allowed` where `bad` matches. `lines` picks
/// which lines of a file are read: [`code_lines`] or [`all_code_lines`].
fn violations(
    files: &[(String, String)],
    allowed: &[&str],
    lines: fn(&str) -> Vec<(usize, String)>,
    bad: impl Fn(&str) -> bool,
) -> Vec<String> {
    let mut found = Vec::new();
    for (path, text) in files {
        if allowed.contains(&path.as_str()) {
            continue;
        }
        for (n, line) in lines(text) {
            if bad(&line) {
                found.push(format!("{path}:{n}: {}", line.trim()));
            }
        }
    }
    found
}

const PRIVILEGE_MODULE: &str = "crates/spela/src/privilege.rs";
const ENV_MODULE: &str = "crates/spela-core/src/env.rs";
/// The one place outside the env module that touches `std::env`: `main`
/// reads its arguments.
const ARGS_READ: &str = "std::env::args_os()";
/// The modules that may build an HTTP agent or open a socket: the binary's
/// backend and the updater's fetch module (both arrive in later phases).
const HTTP_MODULES: &[&str] = &[
    "crates/spela/src/http.rs",
    "crates/dlss-updater/src/fetch.rs",
];

/// Names `pkexec` or `sudo`.
fn names_privilege(line: &str) -> bool {
    line.contains("pkexec") || has_word(line, "sudo")
}

/// Touches the process environment. `env!` is a compile-time macro and is
/// fine.
fn reads_env(line: &str) -> bool {
    const READS: &[&str] = &[
        "env::var(",
        "env::var_os(",
        "env::vars(",
        "env::vars_os(",
        "env::set_var(",
        "env::remove_var(",
        "env::home_dir(",
        "env::temp_dir(",
    ];
    READS.iter().any(|r| line.contains(r))
        || line.replace(ARGS_READ, "").contains("std::env")
        || (line.contains("std::{") && has_word(line, "env"))
}

/// Builds an HTTP agent or opens a socket.
fn uses_network(line: &str) -> bool {
    line.contains("ureq")
        || line.contains("std::net")
        || has_word(line, "TcpStream")
        || has_word(line, "UdpSocket")
        || (line.contains("std::{") && has_word(line, "net"))
}

/// Runs a forbidden program in a way the sandbox's PATH cannot catch: by an
/// absolute path such as `/usr/bin/pkexec`, or by its bare name from a
/// test that may not run in the sandbox.
fn runs_forbidden(line: &str) -> bool {
    FORBIDDEN.iter().any(|name| {
        let by_path = line.match_indices(name).any(|(at, _)| {
            let before = line[..at].chars().next_back();
            let after = line[at + name.len()..].chars().next();
            before == Some('/')
                && after.is_none_or(|c| !(c.is_alphanumeric() || c == '_' || c == '-'))
        });
        by_path || line.contains(&format!("Command::new(\"{name}\")"))
    })
}

#[test]
fn only_the_privilege_module_names_pkexec_or_sudo() {
    let found = violations(
        &sources(),
        &[PRIVILEGE_MODULE],
        all_code_lines,
        names_privilege,
    );
    assert!(
        found.is_empty(),
        "privilege outside {PRIVILEGE_MODULE}:\n{}",
        found.join("\n")
    );
    // The allowlisted module really is the one that names it.
    let privilege = sources()
        .into_iter()
        .find(|(p, _)| p == PRIVILEGE_MODULE)
        .expect("the privilege module exists");
    assert!(
        code_lines(&privilege.1)
            .iter()
            .any(|(_, l)| l.contains("\"pkexec\""))
    );
}

#[test]
fn only_the_env_module_reads_the_environment() {
    let found = violations(&sources(), &[ENV_MODULE], all_code_lines, reads_env);
    assert!(
        found.is_empty(),
        "environment reads outside {ENV_MODULE}:\n{}",
        found.join("\n")
    );
}

#[test]
fn tests_and_build_scripts_never_run_a_forbidden_program() {
    let found = violations(&test_sources(), &[], all_code_lines, runs_forbidden);
    let inline = violations(&sources(), &[], all_code_lines, runs_forbidden);
    assert!(
        found.is_empty() && inline.is_empty(),
        "forbidden programs run outside the sandbox:\n{}",
        [found, inline].concat().join("\n")
    );
}

#[test]
fn only_the_http_modules_build_an_agent() {
    let found = violations(&sources(), HTTP_MODULES, all_code_lines, uses_network);
    assert!(
        found.is_empty(),
        "HTTP outside {HTTP_MODULES:?}:\n{}",
        found.join("\n")
    );
}

#[test]
fn spela_ui_does_no_io() {
    let ui: Vec<(String, String)> = sources()
        .into_iter()
        .filter(|(p, _)| p.starts_with("crates/spela-ui/"))
        .collect();
    assert!(!ui.is_empty());
    const BANNED: &[&str] = &["fs", "process", "net", "env"];
    let found = violations(&ui, &[], all_code_lines, |line| {
        BANNED.iter().any(|m| line.contains(&format!("std::{m}")))
            || (line.contains("std::{") && BANNED.iter().any(|m| has_word(line, m)))
    });
    assert!(found.is_empty(), "I/O in spela-ui:\n{}", found.join("\n"));

    // The dependency list is the rule a tool can check.
    let manifest = std::fs::read_to_string(workspace().join("crates/spela-ui/Cargo.toml")).unwrap();
    for dep in [
        "nvml-wrapper",
        "ureq",
        "libc",
        "rustix",
        "signal-hook",
        "tempfile",
        "walkdir",
        "which",
        "etcetera",
        "wait-timeout",
    ] {
        assert!(
            !manifest.lines().any(|l| l.trim_start().starts_with(dep)),
            "spela-ui depends on {dep}"
        );
    }
}

#[test]
fn gates_catch_what_they_look_for() {
    let file = |text: &str| vec![("crates/x/src/lib.rs".to_owned(), text.to_owned())];
    let sample = file(
        "// pkexec in a comment is fine\n\
         fn a() { let p = \"pkexec\"; } // trailing pkexec comment\n\
         let pseudo = 1;\n\
         std::process::Command::new(\"sudo\");\n\
         let v = std::env::var(\"X\");\n\
         use std::{fs, io};\n\
         #[cfg(test)]\n\
         mod tests { fn t() { let _ = \"pkexec\"; std::env::var(\"Y\"); } }\n",
    );
    let pk = violations(&sample, &[], code_lines, names_privilege);
    assert_eq!(pk.len(), 2, "{pk:?}");
    let pk_all = violations(&sample, &[], all_code_lines, names_privilege);
    assert_eq!(pk_all.len(), 3, "the test module counts too: {pk_all:?}");
    let env = violations(&sample, &[], code_lines, |l| l.contains("env::var("));
    assert_eq!(env.len(), 1, "{env:?}");
    let grouped = violations(&sample, &[], code_lines, |l| {
        l.contains("std::{") && has_word(l, "fs")
    });
    assert_eq!(grouped.len(), 1, "{grouped:?}");
    assert!(!has_word("pseudo", "sudo"));

    // An indented #[cfg(test)] item does not end the production code.
    let hidden = file(
        "impl Colors {\n    #[cfg(test)]\n    fn probe() {}\n}\n\
         fn run() { std::process::Command::new(\"pkexec\"); }\n",
    );
    assert_eq!(code_lines(&hidden[0].1).len(), 5);
    assert_eq!(
        violations(&hidden, &[], code_lines, names_privilege).len(),
        1
    );

    // Imported or grouped env reads, and the args read main may do.
    for (line, bad) in [
        ("use std::env::var;", true),
        ("use std::env;", true),
        ("use std::{env, fs};", true),
        ("let home = ::std::env::home_dir();", true),
        ("let args = std::env::args_os().skip(1);", false),
        ("let dir = env!(\"CARGO_MANIFEST_DIR\");", false),
    ] {
        assert_eq!(reads_env(line), bad, "{line}");
    }

    // Sockets anywhere, not only an agent.
    for (line, bad) in [
        ("std::net::TcpStream::connect(\"203.0.113.1:443\")", true),
        ("use std::net::UdpSocket;", true),
        ("use std::{io, net};", true),
        ("let s = TcpStream::connect(addr);", true),
        ("let agent = ureq::agent();", true),
        ("let network = 1;", false),
    ] {
        assert_eq!(uses_network(line), bad, "{line}");
    }

    // A forbidden program by absolute path, or by bare name in a test.
    let usr_bin = format!("Command::new(\"/usr/bin/{}\")", FORBIDDEN[0]);
    let bare = format!("std::process::Command::new(\"{}\")", FORBIDDEN[1]);
    let bin = format!("let p = \"/bin/{}\";", FORBIDDEN[1]);
    for (line, bad) in [
        (usr_bin.as_str(), true),
        (bare.as_str(), true),
        (bin.as_str(), true),
        ("Command::new(s.bin().join(name))", false),
        ("let p = \"/usr/bin/sudoku\";", false),
    ] {
        assert_eq!(runs_forbidden(line), bad, "{line}");
    }

    // The test-module layout check.
    let ok = "fn a() {}\n\n#[cfg(test)]\nmod tests {\n    fn t() {}\n}\n";
    assert!(test_module_problems(ok).is_empty());
    for bad in [
        "#[cfg(test)]\nmod tests {\n}\nfn after() {}\n",
        "impl A {\n    #[cfg(test)]\n    fn probe() {}\n}\n#[cfg(test)]\nmod tests {\n}\n",
        "fn a() {\n    #[cfg(test)]\n    let x = 1;\n}\n",
        "#[cfg(any(test, feature = \"x\"))]\nfn probe() {}\n",
        "#[cfg(test)]\nfn helper() {}\n",
    ] {
        assert!(!test_module_problems(bad).is_empty(), "{bad}");
    }
}

/// What is wrong with a file's test-module layout: every `#[cfg(...test...)]`
/// must be the single column-0 one on the last item, `mod tests`.
fn test_module_problems(text: &str) -> Vec<String> {
    let lines: Vec<&str> = text.lines().collect();
    let marks: Vec<usize> = lines
        .iter()
        .enumerate()
        .filter(|(_, l)| {
            let t = l.trim_start();
            t.starts_with("#[cfg") && has_word(t, "test")
        })
        .map(|(i, _)| i)
        .collect();
    let at = match marks.as_slice() {
        [] => return Vec::new(),
        [at] => *at,
        _ => return vec!["more than one #[cfg(test)] item".to_owned()],
    };
    if lines[at] != "#[cfg(test)]" || !lines.get(at + 1).is_some_and(|l| l.starts_with("mod ")) {
        return vec!["#[cfg(test)] is not on a column-0 module".to_owned()];
    }
    // Everything after the module line is inside the module: indented or
    // blank, up to the closing brace on the last line.
    let last = lines.iter().rposition(|l| !l.trim().is_empty()).unwrap();
    let inside = &lines[at + 2..last];
    if lines[last] != "}"
        || inside
            .iter()
            .any(|l| !l.is_empty() && !l.starts_with(char::is_whitespace))
    {
        return vec!["the test module is not last".to_owned()];
    }
    Vec::new()
}

#[test]
fn inline_test_modules_come_last() {
    let bad: Vec<String> = sources()
        .into_iter()
        .flat_map(|(path, text)| {
            test_module_problems(&text)
                .into_iter()
                .map(move |p| format!("{path}: {p}"))
        })
        .collect();
    assert!(bad.is_empty(), "{}", bad.join("\n"));
}

#[test]
fn tripwires_catch_a_call() {
    let s = Sandbox::new();
    for name in FORBIDDEN {
        // Another test thread may fork while a stub is still open for
        // writing, and the exec then fails with "text file busy" until that
        // child execs. Retry that one error briefly.
        let mut tries = 0;
        let status = loop {
            match std::process::Command::new(s.bin().join(name))
                .arg("--probe")
                .status()
            {
                Err(e) if e.kind() == std::io::ErrorKind::ExecutableFileBusy && tries < 50 => {
                    tries += 1;
                    std::thread::sleep(std::time::Duration::from_millis(10));
                }
                other => break other.unwrap(),
            }
        };
        assert_eq!(status.code(), Some(97), "{name}");
    }
    let mut expected: Vec<String> = FORBIDDEN.iter().map(|s| (*s).to_owned()).collect();
    expected.sort();
    assert_eq!(s.take_markers(), expected);
}

#[test]
fn the_sandbox_env_stays_inside_its_root() {
    let s = Sandbox::new();
    let cmd = s.command();
    let mut seen = Vec::new();
    for (key, value) in cmd.get_envs() {
        let key = key.to_string_lossy().into_owned();
        let Some(value) = value else { continue };
        let value = Path::new(value);
        if ["HOME", "PATH", "TMPDIR"].contains(&key.as_str()) || key.starts_with("XDG_") {
            s.assert_inside(value);
            seen.push(key);
        }
    }
    seen.sort();
    assert_eq!(
        seen,
        [
            "HOME",
            "PATH",
            "TMPDIR",
            "XDG_CACHE_HOME",
            "XDG_CONFIG_HOME",
            "XDG_DATA_HOME",
            "XDG_RUNTIME_DIR",
            "XDG_STATE_HOME"
        ]
    );
    let backend = cmd
        .get_envs()
        .find(|(k, _)| *k == "SPELA_BACKEND")
        .and_then(|(_, v)| v);
    assert_eq!(backend, Some(std::ffi::OsStr::new("fake")));
}

#[test]
fn every_command_runs_without_reaching_a_tripwire() {
    let s = Sandbox::new();
    for args in [
        &["gpu", "reset"][..],
        &["cpu", "governor", "performance"][..],
        &["cpu", "smt", "off"][..],
        &["gpu", "info"][..],
        &["tui"][..],
        &["--demo"][..],
        &["apply-profile", "--serve"][..],
    ] {
        let run = s.run(args);
        assert!(!run.stderr().contains("panicked"), "{args:?}");
    }
    // The sandbox's drop fails the test if any tripwire ran.
}

#[test]
fn no_command_builds_the_real_backends_in_the_sandbox() {
    // A cleared env does not stop the loader from finding the real NVML, so
    // the sandbox's SPELA_BACKEND=fake must reach every command. Each run
    // logs the backend it built; `tui` logs to its file, checked in
    // tests/cli.rs. Unit tests that call `backends::real()` panic.
    let s = Sandbox::new();
    for args in [
        &["scan"][..],
        &["list"][..],
        &["show", "x"][..],
        &["launch", "x", "--dry-run"][..],
        &["profile", "show", "x"][..],
        &["config", "show"][..],
        &["dll"][..],
        &["dll", "check-updates"][..],
        &["dll", "update", "--all"][..],
        &["dlss", "set", "x", "--sr-preset", "default"][..],
        &["gpu", "info"][..],
        &["gpu", "reset"][..],
        &["cpu", "governor", "performance"][..],
        &["cpu", "smt", "off"][..],
        &["proton", "show", "x"][..],
        &["denylist", "allow", "x"][..],
        &["completion", "bash"][..],
        &["overlay"][..],
    ] {
        let run = s.run_with(args, &[("SPELA_LOG", "info")]);
        let err = run.stderr();
        assert!(err.contains("backend=Fake"), "{args:?}: {err}");
        assert!(!err.contains("backend=Real"), "{args:?}: {err}");
    }
}

#[test]
fn two_processes_never_interleave_lines_in_one_log_file() {
    // TUI and wrapper runs at once, all appending to the same daily file.
    // This covers short lines, the kind spela writes; tracing-appender
    // writes each event with one append.
    const EACH: usize = 12;
    let s = Sandbox::new();
    // A stand-in game that exits at once.
    let game = s.bin().join("mygame");
    std::fs::write(&game, "#!/bin/sh\nexit 0\n").unwrap();
    std::fs::set_permissions(&game, std::fs::Permissions::from_mode(0o755)).unwrap();
    let mut children = Vec::new();
    for _ in 0..EACH {
        for args in [&["mygame"][..], &["tui"][..]] {
            let mut cmd = s.command();
            cmd.args(args)
                .stdout(std::process::Stdio::null())
                .stderr(std::process::Stdio::piped());
            children.push(cmd.spawn().unwrap());
        }
    }
    let stderr: Vec<String> = children
        .into_iter()
        .map(|child| {
            String::from_utf8_lossy(&child.wait_with_output().unwrap().stderr).into_owned()
        })
        .collect();
    // A warning here means a process could not log to the file at all.
    let warnings: Vec<&String> = stderr.iter().filter(|e| e.contains("warning")).collect();
    assert!(warnings.is_empty(), "{warnings:?}");
    let mut text = String::new();
    for entry in std::fs::read_dir(s.state_dir().join("log")).unwrap() {
        text.push_str(&std::fs::read_to_string(entry.unwrap().path()).unwrap());
    }
    let whole = |line: &str| {
        let (stamp, rest) = line.split_once(' ').unwrap_or((line, ""));
        stamp.len() >= 20
            && stamp.as_bytes()[..4].iter().all(u8::is_ascii_digit)
            && stamp.as_bytes()[10] == b'T'
            && stamp.ends_with('Z')
            && ["INFO main spela::cli: ", "INFO main spela::wrapper: "]
                .iter()
                .any(|p| rest.trim_start().starts_with(p))
            && !rest.contains("Z  INFO")
    };
    let bad: Vec<&str> = text.lines().filter(|l| !whole(l)).collect();
    assert!(bad.is_empty(), "broken lines:\n{}", bad.join("\n"));
    let count = |needle: &str| text.lines().filter(|l| l.contains(needle)).count();
    assert_eq!(count("spela wrapping a game"), EACH, "{text}");
    assert_eq!(count("spela starting"), EACH, "{text}");
}
