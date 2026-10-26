//! Wrapper mode end to end: the real binary wraps a small script from a
//! temp dir as the game, on the fake backends (`SPELA_BACKEND=fake`), in
//! the sandbox, so no polkit, `kscreen-doctor` or game ever runs.
//!
//! The script stands in for Steam's `%command%`. It records its environment
//! and, for the signal tests, says it is ready and then becomes
//! `/usr/bin/sleep`, so the signal spela forwards ends it.

mod common;

use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::time::{Duration, Instant};

use common::Sandbox;

const APP_ID: &str = "9100510";

/// A Steam library with one game, scanned into the cache, and All games
/// asking for VRR, a power limit and HDR. Returns the game script.
fn setup(s: &Sandbox, body: &str) -> PathBuf {
    let root = s.home().join(".steam/steam");
    let folder = root.join("steamapps/common/Lantern Path");
    std::fs::create_dir_all(&folder).unwrap();
    std::fs::write(
        root.join("steamapps/libraryfolders.vdf"),
        format!(
            "\"libraryfolders\"\n{{\n\t\"0\"\n\t{{\n\t\t\"path\"\t\t\"{}\"\n\t}}\n}}\n",
            root.display()
        ),
    )
    .unwrap();
    std::fs::write(
        root.join(format!("steamapps/appmanifest_{APP_ID}.acf")),
        format!(
            "\"AppState\"\n{{\n\t\"appid\"\t\t\"{APP_ID}\"\n\t\"name\"\t\t\"Lantern Path\"\n\t\"StateFlags\"\t\t\"4\"\n\t\"installdir\"\t\t\"Lantern Path\"\n}}\n"
        ),
    )
    .unwrap();
    let game = folder.join("lantern.sh");
    let env_file = s.root().join("game-env");
    std::fs::write(
        &game,
        format!(
            "#!/bin/sh\n/usr/bin/env > '{}'\n{body}\n",
            env_file.display()
        ),
    )
    .unwrap();
    std::fs::set_permissions(&game, std::fs::Permissions::from_mode(0o755)).unwrap();
    s.run(&["scan"]).expect_code(0);
    s.run(&[
        "gpu",
        "set",
        "--all-games",
        "--vrr",
        "always",
        "--power-limit",
        "250",
    ])
    .expect_code(0);
    s.run(&["proton", "set", "--all-games", "--hdr", "on"])
        .expect_code(0);
    game
}

/// The KDE Wayland session VRR needs, so the fake display is used.
const KDE: [(&str, &str); 2] = [
    ("XDG_SESSION_TYPE", "wayland"),
    ("XDG_CURRENT_DESKTOP", "KDE"),
];

fn log_text(s: &Sandbox) -> String {
    let dir = s.state_dir().join("log");
    let mut text = String::new();
    for entry in std::fs::read_dir(dir).into_iter().flatten().flatten() {
        text.push_str(&std::fs::read_to_string(entry.path()).unwrap_or_default());
    }
    text
}

fn game_env(s: &Sandbox) -> String {
    std::fs::read_to_string(s.root().join("game-env")).unwrap_or_default()
}

#[test]
fn the_game_runs_with_its_profile_and_its_exit_status() {
    let s = Sandbox::new();
    let game = setup(&s, "exit 7");
    let mut env: Vec<(&str, &str)> = KDE.to_vec();
    env.push(("SPELA_LOG", "info"));
    let run = s.run_with(&["PROTON_LOG=1", "--", game.to_str().unwrap()], &env);
    run.expect_code(7);
    let out = run.stdout();
    assert!(
        out.starts_with("Game: Lantern Path (AppID 9100510)\n"),
        "{out}"
    );
    assert!(
        out.contains(&format!("Command: {}\n", game.display())),
        "{out}"
    );
    assert!(out.contains("  GPU power limit:  250 W\n"), "{out}");
    assert!(
        out.contains("  KDE VRR:  always in the desktop session"),
        "{out}"
    );
    assert!(
        out.ends_with("Launching Lantern Path with All games settings...\n"),
        "{out}"
    );
    let env = game_env(&s);
    assert!(env.contains("PROTON_ENABLE_HDR=1\n"), "{env}");
    assert!(
        env.contains("PROTON_LOG=1\n"),
        "the wrapper's own token: {env}"
    );
    assert!(!env.contains("SPELA_OVERLAY_IPC"), "decision 10");
    let log = log_text(&s);
    assert!(log.contains("restore succeeded area=hardware"), "{log}");
    assert!(log.contains("restore succeeded area=\"KDE VRR\""), "{log}");
    assert!(
        log.find("area=hardware") < log.find("area=\"KDE VRR\""),
        "hardware first, then VRR: {log}"
    );
}

#[test]
fn every_stop_signal_reaches_the_game_and_restores_run() {
    for (signal, code) in [
        (libc::SIGTERM, 143),
        (libc::SIGINT, 130),
        (libc::SIGHUP, 129),
    ] {
        let s = Sandbox::new();
        let ready = s.root().join("ready");
        let game = setup(
            &s,
            &format!(
                "/usr/bin/touch '{}'\nexec /usr/bin/sleep 30",
                ready.display()
            ),
        );
        let mut cmd = s.command();
        cmd.arg(&game)
            .envs(KDE)
            .stdout(Stdio::null())
            .stderr(Stdio::piped());
        let child = cmd.spawn().unwrap();
        wait_for(&ready);
        // SAFETY: kill only sends a signal.
        unsafe { libc::kill(child.id() as libc::pid_t, signal) };
        let output = child.wait_with_output().unwrap();
        assert_eq!(
            output.status.code(),
            Some(code),
            "signal {signal}: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        let log = log_text(&s);
        assert!(log.contains("forwarding a signal to the game"), "{log}");
        assert!(log.contains("restore succeeded area=hardware"), "{log}");
        assert!(log.contains("restore succeeded area=\"KDE VRR\""), "{log}");
    }
}

fn wait_for(path: &Path) {
    let start = Instant::now();
    while !path.exists() {
        assert!(
            start.elapsed() < Duration::from_secs(20),
            "{} never appeared",
            path.display()
        );
        std::thread::sleep(Duration::from_millis(20));
    }
}

#[test]
fn a_missing_game_cache_never_blocks_a_launch() {
    let s = Sandbox::new();
    let game = setup(&s, "exit 0");
    std::fs::remove_file(s.root().join("xdg-cache/spela/games.json")).unwrap();
    let run = s.run_with(&[game.to_str().unwrap()], &[("SteamAppId", APP_ID)]);
    run.expect_code(0);
    assert!(
        run.stderr()
            .contains("the game cache is empty or unreadable"),
        "{}",
        run.stderr()
    );
    assert!(run.stdout().contains("Game: Lantern Path (AppID 9100510)"));
    assert!(game_env(&s).contains("PROTON_ENABLE_HDR=1\n"));
}

#[test]
fn an_unknown_game_runs_with_no_profile() {
    let s = Sandbox::new();
    let script = s.root().join("tool.sh");
    std::fs::write(&script, "#!/bin/sh\nexit 4\n").unwrap();
    std::fs::set_permissions(&script, std::fs::Permissions::from_mode(0o755)).unwrap();
    let run = s.run(&[script.to_str().unwrap()]);
    run.expect_code(4);
    assert_eq!(run.stdout(), "Launching command...\n");
    assert!(
        run.stderr()
            .contains("Warning: could not detect game for wrapper invocation")
    );
}

#[test]
fn demo_never_runs_a_game() {
    let s = Sandbox::new();
    let script = s.root().join("tool.sh");
    std::fs::write(&script, "#!/bin/sh\n/usr/bin/touch ran\n").unwrap();
    std::fs::set_permissions(&script, std::fs::Permissions::from_mode(0o755)).unwrap();
    let run = s.run(&["--demo", script.to_str().unwrap()]);
    run.expect_code(2);
    assert!(
        run.stderr().contains("--demo never runs a game"),
        "{}",
        run.stderr()
    );
    assert!(!s.root().join("ran").exists());
}

#[test]
fn a_launch_that_cannot_start_restores_and_exits_1() {
    let s = Sandbox::new();
    setup(&s, "exit 0");
    let missing = s
        .home()
        .join(".steam/steam/steamapps/common/Lantern Path/gone.sh");
    let mut env: Vec<(&str, &str)> = KDE.to_vec();
    env.push(("SPELA_LOG", "info"));
    let run = s.run_with(&[missing.to_str().unwrap()], &env);
    run.expect_code(1);
    assert!(
        run.stderr().contains("error: cannot start "),
        "{}",
        run.stderr()
    );
    let log = log_text(&s);
    assert!(log.contains("restore succeeded area=hardware"), "{log}");
}
