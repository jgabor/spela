//! Runs the real `spela` executable in the sandbox.

mod common;

use common::Sandbox;
use predicates::prelude::*;

const VERSION_LINE: &str = concat!("spela ", env!("CARGO_PKG_VERSION"), "\n");

#[test]
fn version_flags_print_clap_s_line() {
    let s = Sandbox::new();
    for flag in ["--version", "-V", "-v"] {
        s.spela()
            .arg(flag)
            .assert()
            .success()
            .stdout(VERSION_LINE)
            .stderr("");
    }
    assert!(!s.state_dir().exists(), "version writes nothing");
}

#[test]
fn no_arguments_prints_help_and_exits_0() {
    let s = Sandbox::new();
    let bare = s.run(&[]);
    bare.expect_code(0);
    let help = s.run(&["--help"]);
    help.expect_code(0);
    assert_eq!(bare.stdout(), help.stdout());
    assert!(bare.stderr().is_empty());
    insta::assert_snapshot!("help", help.stdout());
}

#[test]
fn a_parent_without_a_subcommand_prints_its_help() {
    let s = Sandbox::new();
    let run = s.run(&["dll"]);
    run.expect_code(0);
    insta::assert_snapshot!("dll_help", run.stdout());
}

#[test]
fn unknown_commands_are_usage_errors() {
    let s = Sandbox::new();
    s.spela()
        .arg("frobnicate")
        .assert()
        .code(2)
        .stdout("")
        .stderr(predicate::str::contains(
            "unrecognized subcommand 'frobnicate'",
        ));
    s.spela()
        .args(["--log-level", "loud", "scan"])
        .assert()
        .code(2);
}

#[test]
fn unbuilt_commands_say_so_and_exit_1() {
    let s = Sandbox::new();
    for (args, name) in [
        (&["gpu", "info"][..], "gpu info"),
        (&["profile", "create", "Some Game"][..], "profile create"),
        (&["dll", "update", "--all"][..], "dll update"),
        (
            &["gpu", "set", "x", "--clock-offset", "-100"][..],
            "gpu set",
        ),
        (&["launch", "x", "--dry-run"][..], "launch --dry-run"),
    ] {
        let run = s.run(args);
        run.expect_code(1);
        assert_eq!(
            run.stderr(),
            format!("error: 'spela {name}' is not yet implemented in this build\n"),
            "{args:?}"
        );
        assert!(run.stdout().is_empty());
    }
    assert!(!s.state_dir().exists(), "CLI commands write no log file");
}

#[test]
fn json_failures_print_only_an_error_object() {
    let s = Sandbox::new();
    for args in [
        &["--json", "gpu", "info"][..],
        &["gpu", "info", "--json"][..],
    ] {
        let run = s.run(args);
        run.expect_code(1);
        assert!(run.stderr().is_empty(), "{}", run.stderr());
        insta::assert_snapshot!("json_not_implemented", run.stdout());
    }
}

#[test]
fn global_flags_after_a_positional_still_count() {
    let s = Sandbox::new();
    // --json after a game name still gives the error object. The sandbox
    // has no Steam, so the empty cache cannot be scanned.
    let run = s.run(&["show", "x", "--json"]);
    run.expect_code(1);
    assert!(run.stderr().is_empty(), "{}", run.stderr());
    let error: serde_json::Value = serde_json::from_str(&run.stdout()).unwrap();
    assert_eq!(error["schema_version"], 1, "{error}");
    assert_eq!(error["error"]["code"], "no_steam", "{error}");
    assert_eq!(
        error["error"]["message"],
        "failed to refresh game database: could not find Steam installation"
    );
    // --demo after a game name selects the demo, not the real paths.
    let run = s.run_with(&["show", "x", "--demo"], &[("SPELA_LOG", "info")]);
    run.expect_code(1);
    assert!(run.stderr().contains("demo=true"), "{}", run.stderr());
    assert!(run.stderr().contains("backend=Fake"), "{}", run.stderr());
    // --log-level after a positional wins over SPELA_LOG.
    let run = s.run_with(
        &["profile", "show", "x", "--log-level", "error"],
        &[("SPELA_LOG", "info")],
    );
    assert!(!run.stderr().contains("spela starting"), "{}", run.stderr());
    s.run(&["show", "x", "--log-level", "loud"]).expect_code(2);
}

#[test]
fn launch_without_dry_run_points_at_steam_and_exits_2() {
    let s = Sandbox::new();
    let run = s.run(&["launch", "4000001"]);
    run.expect_code(2);
    assert_eq!(
        run.stderr(),
        "error: direct Steam URI launch cannot track the game lifetime or cleanup coverage\n\
         hint: set the game's Steam launch options to `spela %command%` instead\n"
    );
}

#[test]
fn reserved_names_never_launch_a_program_on_path() {
    // A program on PATH with each name. The Go wrapper ran /usr/bin/overlay.
    let s = Sandbox::new();
    for name in ["overlay", "gui", "scan"] {
        s.tripwire(name);
    }
    for name in ["overlay", "gui"] {
        for args in [
            vec![name],
            vec![name, "set", "x"],
            vec!["--demo", name],
            vec![name, "--help"],
            vec![name, "-h"],
            vec!["help", name],
            vec![name, "--log-level", "loud"],
        ] {
            let run = s.run(&args);
            run.expect_code(2);
            assert!(
                run.stderr().starts_with(&format!(
                    "error: the {name} command was removed in 0.7.0\nhint: "
                )),
                "{args:?}: {}",
                run.stderr()
            );
        }
    }
    s.run(&["scan"]).expect_code(1);
    // The sandbox's drop checks that no tripwire ran.
}

#[test]
fn wrapped_games_route_to_the_wrapper() {
    let s = Sandbox::new();
    s.tripwire("mygame");
    let script = s.tripwire("launch.sh");
    let script = script.to_str().unwrap();
    let wrapper = "error: running a game through spela is not yet implemented in this build\n";
    for args in [
        vec!["mygame", "--fullscreen"],
        vec!["--", "mygame"],
        vec!["FOO=1", "--", "mygame"],
        vec!["FOO=1", "BAR=2", script],
        vec![script, "-x"],
    ] {
        let run = s.run(&args);
        run.expect_code(1);
        assert_eq!(run.stderr(), wrapper, "{args:?}");
    }
    // A wrapper run logs to the file, inside the sandbox.
    let log_dir = s.state_dir().join("log");
    assert!(log_dir.is_dir(), "{:?}", s.tree());
    s.assert_inside(&log_dir);

    // Not a program: clap reports it.
    s.run(&["FOO=1", "--", "missing"]).expect_code(2);
    s.run(&["missing"]).expect_code(2);

    // The extension check ignores case, as Go's does.
    for name in ["Game.EXE", "run.SH"] {
        s.tripwire(name);
        let run = s.run(&[name]);
        run.expect_code(1);
        assert_eq!(run.stderr(), wrapper, "{name}");
    }
}

#[test]
fn apply_profile_refuses_go_flags_and_writes_nothing() {
    let s = Sandbox::new();
    for args in [
        &["apply-profile", "--reset"][..],
        &[
            "apply-profile",
            "--clock-offset",
            "100",
            "--power-limit",
            "250",
        ][..],
        &["apply-profile"][..],
        &["apply-profile", "--help"][..],
        &["apply-profile", "-h"][..],
        &["apply-profile", "--once"][..],
        &["apply-profile", "--serve", "--once", "x"][..],
        &["apply-profile", "--serve", "--reset"][..],
    ] {
        s.spela()
            .args(args)
            .assert()
            .code(2)
            .stdout("")
            .stderr(
                "apply-profile: this is the spela 0.7 helper; the Go helper's flags are not supported\n",
            );
    }
    s.spela()
        .args(["apply-profile", "--serve"])
        .assert()
        .code(1)
        .stderr("apply-profile: not yet implemented in this build\n");
    let tree = s.tree();
    assert!(
        !tree.iter().any(|p| p.starts_with("xdg-")),
        "the helper resolves no XDG path: {tree:?}"
    );
}

#[test]
fn tui_takes_its_lock_and_logs_inside_the_sandbox() {
    let s = Sandbox::new();
    let run = s.run(&["tui"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: 'spela tui' is not yet implemented in this build\n"
    );
    let lock = s.state_dir().join("locks/tui.lock");
    assert!(lock.is_file(), "{:?}", s.tree());
    s.assert_inside(&lock);
    let logs: Vec<_> = std::fs::read_dir(s.state_dir().join("log"))
        .unwrap()
        .map(|e| e.unwrap().path())
        .collect();
    assert_eq!(logs.len(), 1, "{logs:?}");
    let text = std::fs::read_to_string(&logs[0]).unwrap();
    assert!(text.contains("spela starting"), "{text}");
    assert!(text.contains("took the TUI lock"), "{text}");
    assert!(
        text.contains("backend=Fake"),
        "SPELA_BACKEND=fake was read: {text}"
    );
    assert!(!text.contains('\x1b'), "no ANSI in the file");
}

#[test]
fn a_held_tui_lock_exits_5_with_the_pid() {
    let s = Sandbox::new();
    let lock = s.state_dir().join("locks/tui.lock");
    let held = spela_core::lock::InstanceLock::acquire(&lock).unwrap();
    let run = s.run(&["tui"]);
    run.expect_code(5);
    assert_eq!(
        run.stderr(),
        format!(
            "error: spela is already running (PID: {})\n",
            std::process::id()
        )
    );
    drop(held);
    s.run(&["tui"]).expect_code(1);
}

#[test]
fn relative_xdg_values_are_ignored() {
    let s = Sandbox::new();
    let run = s.run_with(&["tui"], &[("XDG_STATE_HOME", "relative-state")]);
    run.expect_code(1);
    let lock = s.home().join(".local/state/spela/locks/tui.lock");
    assert!(lock.is_file(), "{:?}", s.tree());
    assert!(!s.root().join("relative-state").exists());
}

#[test]
fn demo_runs_in_a_private_temp_dir_that_goes_away() {
    let s = Sandbox::new();
    std::fs::create_dir_all(s.config_dir()).unwrap();
    std::fs::write(s.config_dir().join("config.toml"), "version = 1\n").unwrap();
    let run = s.run(&["--demo"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: 'spela tui' is not yet implemented in this build\n"
    );
    assert!(
        !s.state_dir().exists(),
        "the demo locks and logs in its own dir"
    );
    let left: Vec<_> = std::fs::read_dir(s.tmp()).unwrap().collect();
    assert!(left.is_empty(), "demo dir removed: {left:?}");
    assert_eq!(
        std::fs::read_to_string(s.config_dir().join("config.toml")).unwrap(),
        "version = 1\n",
        "the real config is never written"
    );
}

#[test]
fn backend_selector_typos_fail() {
    let s = Sandbox::new();
    let run = s.run_with(&["scan"], &[("SPELA_BACKEND", "fkae")]);
    run.expect_code(1);
    assert!(
        run.stderr()
            .contains("unknown SPELA_BACKEND value \"fkae\""),
        "{}",
        run.stderr()
    );
}

#[test]
fn log_level_comes_from_the_flag_then_spela_log() {
    let s = Sandbox::new();
    assert!(!s.run(&["scan"]).stderr().contains("spela starting"));
    let from_env = s.run_with(&["scan"], &[("SPELA_LOG", "info")]);
    assert!(
        from_env.stderr().contains("spela starting"),
        "{}",
        from_env.stderr()
    );
    let flag_wins = s.run_with(&["--log-level", "error", "scan"], &[("SPELA_LOG", "info")]);
    assert!(!flag_wins.stderr().contains("spela starting"));
    let bad = s.run_with(&["scan"], &[("SPELA_LOG", "spela=loud")]);
    assert!(
        bad.stderr().contains("warning: invalid log filter"),
        "{}",
        bad.stderr()
    );
    // Not a terminal, so no colour, whatever NO_COLOR says.
    assert!(!from_env.stderr().contains('\x1b'));
}

#[test]
fn completion_scripts_are_static() {
    let s = Sandbox::new();
    for shell in ["bash", "zsh", "fish"] {
        s.spela()
            .args(["completion", shell])
            .assert()
            .success()
            .stdout(predicate::str::contains("check-updates"))
            .stdout(predicate::str::contains("apply-profile").not())
            .stdout(predicate::str::contains("overlay").not());
    }
}

#[test]
fn a_closed_stdout_is_a_quiet_exit_0() {
    let s = Sandbox::new();
    for args in [&["--help"][..], &["completion", "bash"][..], &["dll"][..]] {
        let (reader, writer) = std::io::pipe().unwrap();
        drop(reader);
        let mut cmd = s.command();
        cmd.args(args)
            .stdout(writer)
            .stderr(std::process::Stdio::piped());
        let out = cmd.output().unwrap();
        let stderr = String::from_utf8_lossy(&out.stderr);
        assert_eq!(out.status.code(), Some(0), "{args:?}: {stderr}");
        assert!(stderr.is_empty(), "{args:?}: {stderr}");
    }
}
