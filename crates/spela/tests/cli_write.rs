//! Runs the write commands in the sandbox, over Go-written XDG trees.
//!
//! Each test copies a case from `tests/fixtures/go` into the sandbox, with
//! `/fixture-root` replaced by the copy, and points `XDG_CONFIG_HOME`,
//! `XDG_DATA_HOME`, `XDG_CACHE_HOME` and `HOME` at it. The first command
//! migrates the Go files; the tests check that the Go originals stay
//! byte-identical and that running a command again changes nothing.

mod common;

use std::collections::BTreeMap;
use std::os::unix::fs::MetadataExt;
use std::path::{Path, PathBuf};

use common::{Run, Sandbox};

const ROOT_TOKEN: &str = "/fixture-root";

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../tests/fixtures/go")
}

/// A Go case copied into a sandbox.
struct Case {
    sandbox: Sandbox,
    root: PathBuf,
}

impl Case {
    /// Copies `tests/fixtures/go/<name>` into a new sandbox.
    fn new(name: &str) -> Case {
        let sandbox = Sandbox::new();
        let root = sandbox.root().join("case");
        copy(&fixtures().join(name), &root, &root);
        for dir in ["config/spela", "data/spela", "cache/spela", "home"] {
            std::fs::create_dir_all(root.join(dir)).unwrap();
        }
        Case { sandbox, root }
    }

    fn run(&self, args: &[&str]) -> Run {
        let dir = |d: &str| self.root.join(d).display().to_string();
        let (config, data, cache, home) = (dir("config"), dir("data"), dir("cache"), dir("home"));
        self.sandbox.run_with(
            args,
            &[
                ("XDG_CONFIG_HOME", &config),
                ("XDG_DATA_HOME", &data),
                ("XDG_CACHE_HOME", &cache),
                ("HOME", &home),
            ],
        )
    }

    /// Runs a command that must succeed and returns its stdout.
    fn ok(&self, args: &[&str]) -> String {
        let run = self.run(args);
        run.expect_code(0);
        run.stdout()
    }

    fn config(&self) -> PathBuf {
        self.root.join("config/spela")
    }

    fn profile(&self, app_id: u64) -> PathBuf {
        self.config().join(format!("profiles/{app_id}.toml"))
    }

    fn read(&self, path: &Path) -> String {
        std::fs::read_to_string(path).unwrap()
    }

    /// Every file under the case's config, data and cache folders, with its
    /// bytes and inode, so a second run that rewrites a file shows.
    fn files(&self) -> BTreeMap<PathBuf, (Vec<u8>, u64)> {
        let mut out = BTreeMap::new();
        for dir in ["config", "data", "cache"] {
            walk(&self.root.join(dir), &mut |path| {
                let meta = std::fs::metadata(path).unwrap();
                out.insert(
                    path.strip_prefix(&self.root).unwrap().to_owned(),
                    (std::fs::read(path).unwrap(), meta.ino()),
                );
            });
        }
        out
    }

    /// The Go files and their bytes: every YAML file, the Go caches and
    /// the backups.
    fn go_files(&self) -> BTreeMap<PathBuf, Vec<u8>> {
        self.files()
            .into_iter()
            .filter(|(path, _)| {
                let name = path.file_name().unwrap().to_string_lossy();
                name.ends_with(".yaml")
                    || name.ends_with(".yml")
                    || name == "manifest.json"
                    || name == "update-check.json"
                    || path.starts_with("data/spela/backups")
            })
            .map(|(path, (bytes, _))| (path, bytes))
            .collect()
    }
}

fn walk(dir: &Path, visit: &mut dyn FnMut(&Path)) {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if entry.file_type().unwrap().is_dir() {
            walk(&path, visit);
        } else {
            visit(&path);
        }
    }
}

/// Copies a folder, replacing `/fixture-root` with `root` in text files.
fn copy(from: &Path, to: &Path, root: &Path) {
    std::fs::create_dir_all(to).unwrap();
    for entry in std::fs::read_dir(from).unwrap().flatten() {
        let target = to.join(entry.file_name());
        if entry.file_type().unwrap().is_dir() {
            copy(&entry.path(), &target, root);
            continue;
        }
        let bytes = std::fs::read(entry.path()).unwrap();
        let bytes = match String::from_utf8(bytes) {
            Ok(text) => text
                .replace(ROOT_TOKEN, &root.display().to_string())
                .into_bytes(),
            Err(e) => e.into_bytes(),
        };
        std::fs::write(&target, bytes).unwrap();
    }
}

fn lines(parts: &[&str]) -> String {
    let mut text = parts.join("\n");
    text.push('\n');
    text
}

#[test]
fn the_first_write_migrates_and_a_second_run_changes_nothing() {
    let case = Case::new("sandbox/main");
    let go = case.go_files();
    assert!(go.keys().any(|p| p.ends_with("9100010.yaml")), "{go:?}");
    let args = [
        "gpu",
        "set",
        "9100010",
        "--fan-speed",
        "0",
        "--power-limit",
        "200",
    ];
    let first = case.run(&args);
    first.expect_code(0);
    assert_eq!(first.stdout(), "Updated GPU profile for Glimmer Run\n");
    let notice = first.stderr();
    assert!(
        notice.contains("Migrated 9 files from Go spela, kept as backups only."),
        "{notice}"
    );
    assert!(
        notice.contains("GPU clock and memory offsets from Go spela were not carried over"),
        "{notice}"
    );
    let text = case.read(&case.profile(9100010));
    assert!(
        text.starts_with("# Migrated from 9100010.yaml (Go spela)"),
        "{text}"
    );
    for kept in [
        "name = \"Glimmer Run\"",
        "sr_mode = \"quality\"",
        "power_limit = 200",
        "fan_speed = 0",
    ] {
        assert!(text.contains(kept), "{kept}: {text}");
    }
    assert_eq!(case.go_files(), go, "the Go files stay byte-identical");

    let before = case.files();
    let second = case.run(&args);
    second.expect_code(0);
    assert_eq!(second.stdout(), "Updated GPU profile for Glimmer Run\n");
    assert_eq!(second.stderr(), "", "the notice comes once");
    assert_eq!(case.files(), before, "a second run writes nothing");
}

#[test]
fn a_tui_first_run_does_not_swallow_the_notice() {
    let case = Case::new("sandbox/main");
    let tui = case.run(&["tui"]);
    tui.expect_code(1);
    assert!(!tui.stderr().contains("Migrated"), "{}", tui.stderr());
    let list = case.run(&["profile", "list"]);
    list.expect_code(0);
    let notice = list.stderr();
    assert!(
        notice.contains("Migrated 9 files from Go spela"),
        "{notice}"
    );
    assert!(
        notice.contains("GPU clock and memory offsets from Go spela were not carried over"),
        "{notice}"
    );
}

#[test]
fn empty_flag_values_count_as_not_given_as_in_go() {
    let case = Case::new("sandbox/main");
    case.ok(&["gpu", "set", "9100010"]);
    let before = case.files();
    for args in [
        &["cpu", "set", "9100010", "--governor", ""][..],
        &["dlss", "set", "9100010", "--sr-mode", "", "--rr-preset", ""][..],
        &[
            "gpu",
            "set",
            "9100010",
            "--shader-cache-path",
            "",
            "--vrr",
            "",
        ][..],
        &["proton", "set", "9100010", "--hdr", ""][..],
        &["cpu", "set", "--all-games", "--smt", ""][..],
    ] {
        assert_eq!(
            case.ok(args),
            "No changes specified. Use --help to see available options.\n",
            "{args:?}"
        );
    }
    assert_eq!(case.files(), before, "nothing written");
    let out = case.ok(&["cpu", "set", "9100010", "--governor", "", "--smt", "on"]);
    assert_eq!(out, "Updated CPU profile for Glimmer Run\n");
    let text = case.read(&case.profile(9100010));
    assert!(!text.contains("governor"), "{text}");
    case.ok(&["cpu", "set", "9100010", "--governor", "performance"]);
    assert!(
        case.read(&case.profile(9100010))
            .contains("governor = \"performance\""),
        "a listed governor is stored"
    );
}

#[test]
fn a_bad_setter_value_under_json_prints_the_error_object() {
    let case = Case::new("sandbox/main");
    let run = case.run(&["--json", "dlss", "set", "9100010", "--sr-mode", "bogus"]);
    run.expect_code(2);
    assert_eq!(run.stderr(), "");
    let json: serde_json::Value = serde_json::from_str(&run.stdout()).unwrap();
    assert_eq!(json["schema_version"], 1);
    assert_eq!(json["error"]["code"], "usage");
    let message = json["error"]["message"].as_str().unwrap();
    assert!(
        message.starts_with(
            "invalid value 'bogus' for '--sr-mode <VALUE>': dlss.sr_mode: unsupported value"
        ),
        "{message}"
    );
    // clap's own usage errors keep clap's format.
    let run = case.run(&["--json", "dlss", "set", "9100010", "--nope"]);
    run.expect_code(2);
    assert!(
        run.stderr().starts_with("error: unexpected argument"),
        "{}",
        run.stderr()
    );
    assert_eq!(run.stdout(), "");
}

#[test]
fn set_flags_store_values_follow_on_default_and_imply_overrides() {
    let case = Case::new("sandbox/main");
    // Quiet Orchard has no profile: setting creates one with its name.
    case.ok(&[
        "dlss",
        "set",
        "9100030",
        "--sr-mode",
        "dlaa",
        "--multi-frame",
        "2",
        "--indicator",
    ]);
    assert_eq!(
        case.read(&case.profile(9100030)),
        lines(&[
            "version = 1",
            "name = \"Quiet Orchard\"",
            "",
            "[dlss]",
            "sr_mode = \"dlaa\"",
            "sr_override = true",
            "fg_override = true",
            "multi_frame = 2",
            "indicator = true",
        ])
    );
    let out = case.ok(&[
        "dlss",
        "set",
        "Quiet Orchard",
        "--sr-mode",
        "default",
        "--sr-override",
        "off",
        "--sr-preset",
        "default",
        "--indicator=default",
    ]);
    assert_eq!(out, "Updated DLSS configuration for Quiet Orchard\n");
    let text = case.read(&case.profile(9100030));
    assert!(!text.contains("sr_mode"), "default follows: {text}");
    assert!(!text.contains("indicator"), "{text}");
    assert!(text.contains("sr_override = false"), "{text}");
    assert!(
        text.contains("sr_preset = \"default\""),
        "a preset stores default: {text}"
    );

    case.ok(&[
        "gpu",
        "set",
        "9100030",
        "--clock-offset",
        "-100",
        "--vrr",
        "always",
        "--shader-cache-path",
        "/games/cache",
        "--threaded-opt",
        "no",
    ]);
    case.ok(&[
        "cpu",
        "set",
        "9100030",
        "--smt",
        "off",
        "--governor",
        "powersave",
    ]);
    case.ok(&[
        "proton",
        "set",
        "9100030",
        "--hdr",
        "yes",
        "--vkd3d-heap",
        "1",
    ]);
    let text = case.read(&case.profile(9100030));
    for line in [
        "clock_offset = -100",
        "vrr = \"always\"",
        "shader_cache_path = \"/games/cache\"",
        "threaded_optimization = false",
        "governor = \"powersave\"",
        "smt = false",
        "enable_hdr = true",
        "vkd3d_heap = true",
    ] {
        assert!(text.contains(line), "{line}: {text}");
    }
    case.ok(&[
        "gpu",
        "set",
        "9100030",
        "--shader-cache-path",
        "default",
        "--fan-speed",
        "default",
    ]);
    assert!(
        !case
            .read(&case.profile(9100030))
            .contains("shader_cache_path")
    );
    case.ok(&[
        "cpu",
        "set",
        "9100030",
        "--smt",
        "default",
        "--governor",
        "default",
    ]);
    assert!(!case.read(&case.profile(9100030)).contains("[cpu]"));

    let none = case.ok(&["proton", "set", "9100030"]);
    assert_eq!(
        none,
        "No changes specified. Use --help to see available options.\n"
    );
}

#[test]
fn all_games_is_edited_with_all_games() {
    let case = Case::new("sandbox/main");
    let out = case.ok(&["dlss", "set", "--all-games", "--sr-mode", "balanced"]);
    assert_eq!(out, "Updated DLSS configuration for All games\n");
    let all = case.config().join("profiles/all-games.toml");
    let text = case.read(&all);
    assert!(
        text.starts_with("# Migrated from default.yaml (Go spela)"),
        "{text}"
    );
    assert!(text.contains("sr_mode = \"balanced\""), "{text}");
    assert!(
        text.contains("governor = \"performance\""),
        "Go's value kept: {text}"
    );
    let out = case.ok(&["cpu", "profile-reset", "--all-games", "governor"]);
    assert_eq!(out, "All games: Governor now uses the system default.\n");
    let out = case.ok(&["cpu", "profile-reset", "--all-games", "governor"]);
    assert_eq!(
        out,
        "All games: Governor already uses the system default.\n"
    );
    let out = case.ok(&["dlss", "reset", "--all-games"]);
    assert_eq!(
        out,
        "All games now uses the system default for all DLSS settings (3 cleared).\n"
    );
    let out = case.ok(&["profile", "reset", "--all-games"]);
    assert_eq!(
        out,
        "All games now uses the system default for all settings (2 cleared).\n"
    );
    let before = case.files();
    let out = case.ok(&["profile", "reset", "--all-games"]);
    assert_eq!(
        out,
        "All games already uses the system default for all settings.\n"
    );
    assert_eq!(case.files(), before);
    let run = case.run(&["gpu", "set", "9100010", "--all-games", "--fan-speed", "1"]);
    run.expect_code(2);
}

#[test]
fn resets_keep_go_s_texts() {
    let case = Case::new("sandbox/main");
    let out = case.ok(&["proton", "reset", "9100010", "hdr", "wayland"]);
    assert_eq!(
        out,
        lines(&[
            "Glimmer Run: HDR now follows All games.",
            "Glimmer Run: Wayland already follows All games.",
        ])
    );
    let out = case.ok(&["gpu", "profile-reset", "9100030", "fan-speed"]);
    assert_eq!(
        out,
        "No profile for Quiet Orchard; Fan speed already follows All games.\n"
    );
    let out = case.ok(&["dlss", "reset", "9100010"]);
    assert!(
        out.starts_with("Glimmer Run now follows All games for all DLSS settings ("),
        "{out}"
    );
    let run = case.run(&["proton", "reset", "9100010", "bogus"]);
    run.expect_code(2);
    assert_eq!(
        run.stderr(),
        "error: unknown proton field \"bogus\" (valid: hdr, wayland, ngx_updater, vkd3d_heap)\n"
    );
    let run = case.run(&["cpu", "profile-reset", "9100010", "affinity"]);
    run.expect_code(2);
    assert_eq!(run.stderr(), "error: unknown CPU field \"affinity\"\n");
    let out = case.ok(&["profile", "reset", "9100010"]);
    assert!(
        out.starts_with("Glimmer Run now follows All games for all settings ("),
        "{out}"
    );
    let text = case.read(&case.profile(9100010));
    assert!(
        text.contains("name = \"Glimmer Run\""),
        "the name stays: {text}"
    );
    let out = case.ok(&["profile", "reset", "9100010"]);
    assert_eq!(
        out,
        "Glimmer Run already follows All games for all settings.\n"
    );
    let out = case.ok(&["profile", "reset", "9100030"]);
    assert_eq!(
        out,
        "No profile for Quiet Orchard; it already follows All games for all settings.\n"
    );
}

#[test]
fn create_and_delete_report_what_happened() {
    let case = Case::new("sandbox/main");
    assert_eq!(
        case.ok(&["profile", "create", "Quiet Orchard"]),
        "Created profile for Quiet Orchard\n"
    );
    assert_eq!(
        case.read(&case.profile(9100030)),
        "version = 1\nname = \"Quiet Orchard\"\n"
    );
    let run = case.run(&["profile", "create", "9100030"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: profile already exists for Quiet Orchard\n"
    );
    assert_eq!(
        case.ok(&["profile", "delete", "9100010"]),
        "Deleted profile for Glimmer Run\n"
    );
    assert_eq!(
        case.ok(&["profile", "delete", "9100010"]),
        "No profile for Glimmer Run\n",
        "Go reported a deletion here"
    );
    // The migration record keeps the Go profile from coming back.
    assert_eq!(
        case.ok(&["profile", "reset", "9100010"]),
        "No profile for Glimmer Run; it already follows All games for all settings.\n"
    );
    assert!(!case.profile(9100010).exists());
    assert!(
        case.config().join("profiles/9100010.yaml").exists(),
        "Go's file stays"
    );
    let run = case.run(&["profile", "delete", "missing"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: game not found: missing\nhint: run 'spela scan' to refresh the game list\n"
    );
}

#[test]
fn follow_makes_games_with_their_own_value_follow() {
    let case = Case::new("sandbox/main");
    case.ok(&["dlss", "set", "9100030", "--sr-mode", "dlaa"]);
    let out = case.ok(&["profile", "follow", "dlss.sr_mode", "proton.hdr"]);
    assert_eq!(
        out,
        lines(&[
            "Super resolution: 2 games follow All games",
            "HDR: 1 game follows All games",
        ])
    );
    for app_id in [9100010, 9100030] {
        let text = case.read(&case.profile(app_id));
        assert!(!text.contains("sr_mode"), "{text}");
    }
    let before = case.files();
    let out = case.ok(&["profile", "follow", "dlss.sr_mode"]);
    assert_eq!(out, "Super resolution: 0 games follow All games\n");
    assert_eq!(case.files(), before, "nothing to follow writes nothing");

    case.ok(&["gpu", "set", "9100030", "--fan-speed", "40"]);
    case.ok(&["gpu", "set", "9100060", "--fan-speed", "50"]);
    let out = case.ok(&[
        "profile",
        "follow",
        "gpu.fan-speed",
        "--games",
        "9100060,Glimmer Run",
    ]);
    // Glimmer Run's Go profile set a fan speed too; Quiet Orchard is not
    // listed, so it keeps its own.
    assert_eq!(out, "Fan speed: 2 games follow All games\n");
    assert!(case.read(&case.profile(9100030)).contains("fan_speed = 40"));
    assert!(!case.read(&case.profile(9100060)).contains("fan_speed"));
    assert!(!case.read(&case.profile(9100010)).contains("fan_speed"));
    let run = case.run(&["profile", "follow", "gpu.power_mizer"]);
    run.expect_code(2);
    assert_eq!(
        run.stderr(),
        "error: unknown profile field: \"gpu.power_mizer\"\n"
    );
}

#[test]
fn bad_values_are_usage_errors_that_write_nothing() {
    let case = Case::new("sandbox/main");
    // A run with no flags migrates and changes nothing else.
    case.ok(&["gpu", "set", "9100010"]);
    let before = case.files();
    for (args, text) in [
        (
            &["dlss", "set", "9100010", "--rr-preset", "auto"][..],
            "dlss.rr_preset: unsupported value \"auto\"",
        ),
        (
            &["dlss", "set", "9100010", "--sr-preset", "G"][..],
            "dlss.sr_preset: unsupported value \"G\"",
        ),
        (
            &["proton", "set", "9100010", "--hdr", "maybe"][..],
            "invalid bool \"maybe\" (use true/false)",
        ),
        (
            &["cpu", "set", "9100010", "--smt", "sometimes"][..],
            "invalid SMT value: sometimes (use on, off, or default)",
        ),
        (
            &["gpu", "set", "9100010", "--fan-speed", "loud"][..],
            "gpu.fan_speed: invalid integer \"loud\"",
        ),
        (
            &["gpu", "set", "9100010", "--vrr", "sometimes"][..],
            "gpu.vrr: unsupported value \"sometimes\"",
        ),
        (
            &["dlss", "set", "9100010", "--multi-frame", "5"][..],
            "dlss.multi_frame: unsupported value \"5\"",
        ),
        (
            &["cpu", "set", "9100010", "--governor", "turbo"][..],
            "error: --governor: CPU governor \"turbo\" is not available (available: performance, powersave)\n",
        ),
        (
            &["cpu", "set", "--all-games", "--governor", "conservative"][..],
            "error: --governor: CPU governor \"conservative\" is not available (available: performance, powersave)\n",
        ),
        (
            &["gpu", "set", "9100010", "--power-limit", ""][..],
            "gpu.power_limit: invalid integer \"\"",
        ),
        (
            &["config", "set", "check_updates", "perhaps"][..],
            "invalid boolean for check_updates: perhaps",
        ),
        (
            &["config", "set", "compact_mode", "true"][..],
            "unknown config key: compact_mode",
        ),
    ] {
        let run = case.run(args);
        run.expect_code(2);
        assert!(run.stderr().contains(text), "{args:?}: {}", run.stderr());
    }
    assert_eq!(case.files(), before, "nothing written");
}

#[test]
fn config_set_takes_every_settings_key() {
    let case = Case::new("sandbox/main");
    for (key, value) in [
        ("log_level", "debug"),
        ("check_updates", "on"),
        ("rescan_on_startup", "no"),
        ("auto_update_dlls", "yes"),
        ("confirm_destructive", "false"),
        ("steam_path", "/games/steam"),
        ("additional_library_paths", "/games/a:/games/b c"),
        ("dll_cache_path", "/games/dlls"),
        ("backup_path", "/games/backups"),
        ("shader_cache", "/games/shaders"),
        ("dll_manifest_url", "https://example.invalid/manifest.json"),
        ("auto_refresh_manifest", "off"),
        ("manifest_refresh_hours", "48"),
        ("ui.theme", "mono"),
        ("ui.mouse", "0"),
        ("ui.ascii_glyphs", "1"),
    ] {
        let out = case.ok(&["config", "set", key, value]);
        assert_eq!(out, format!("Set {key} = {value}\n"));
    }
    let text = case.read(&case.config().join("config.toml"));
    assert!(
        text.starts_with("# Migrated from config.yaml (Go spela)"),
        "{text}"
    );
    for line in [
        "log_level = \"debug\"",
        "additional_library_paths = [\"/games/a\", \"/games/b c\"]",
        "manifest_refresh_hours = 48",
        "[ui]",
        "theme = \"mono\"",
        "ascii_glyphs = true",
    ] {
        assert!(text.contains(line), "{line}: {text}");
    }
    let before = case.files();
    case.ok(&["config", "set", "ui.theme", "mono"]);
    assert_eq!(case.files(), before, "the same value writes nothing");
}

#[test]
fn denylist_writes_clear_each_other() {
    let case = Case::new("sandbox/main");
    let out = case.ok(&["denylist", "allow", "Frostline Arena"]);
    assert_eq!(
        out,
        lines(&[
            "Force-allowed Frostline Arena (9100070)",
            "WARNING: Swapping DLLs in anti-cheat games may result in bans!",
        ])
    );
    let overrides = case.config().join("lists/overrides.toml");
    let text = case.read(&overrides);
    assert!(
        text.starts_with("# Migrated from overrides.yaml (Go spela)"),
        "{text}"
    );
    assert!(text.contains("allowed = [1172470, 9100070]"), "{text}");
    assert!(
        !text.contains("[[denied]]"),
        "allow clears the deny: {text}"
    );
    let out = case.ok(&["denylist", "deny", "9100070", "--reason", "anti-cheat"]);
    assert_eq!(out, "Denied Frostline Arena (9100070): anti-cheat\n");
    let text = case.read(&overrides);
    assert!(
        text.contains("allowed = [1172470]"),
        "deny clears the allow: {text}"
    );
    let out = case.ok(&["denylist", "deny", "9100999"]);
    assert_eq!(out, "Denied 9100999 (9100999): user-specified\n");
    let out = case.ok(&["denylist", "clear", "9100070"]);
    assert_eq!(
        out,
        "Cleared your allow or deny for Frostline Arena (9100070)\n"
    );
    let before = case.files();
    let out = case.ok(&["denylist", "clear", "9100070"]);
    assert_eq!(
        out,
        "Frostline Arena (9100070) has no allow or deny of yours\n"
    );
    assert_eq!(case.files(), before, "nothing to clear writes nothing");
    // An AppID the cache does not know is named by its digits, as in Go and
    // in `denylist check`, even when the built-in list names it.
    let out = case.ok(&["denylist", "allow", "252490"]);
    assert!(out.starts_with("Force-allowed 252490 (252490)\n"), "{out}");
    let out = case.ok(&["denylist", "deny", "252490"]);
    assert_eq!(out, "Denied 252490 (252490): user-specified\n");
    let out = case.ok(&["denylist", "check", "252490"]);
    assert!(
        out.starts_with("252490 is DENIED: user-specified\n"),
        "{out}"
    );
    let run = case.run(&["denylist", "deny", "18446744073709551615"]);
    run.expect_code(2);
    assert_eq!(
        run.stderr(),
        "error: app id 18446744073709551615 is too large for TOML\n"
    );
    let run = case.run(&["denylist", "allow", "nope"]);
    run.expect_code(1);
    assert!(run.stderr().starts_with("error: game not found: nope\n"));
}

#[test]
fn writes_to_a_go_profile_that_failed_to_migrate_are_refused() {
    let case = Case::new("sandbox/errors");
    let go = case.go_files();
    for args in [
        &["gpu", "set", "9100030", "--fan-speed", "1"][..],
        &["profile", "create", "9100030"][..],
        &["profile", "reset", "9100060"][..],
        &["dlss", "reset", "9100060", "sr_mode"][..],
    ] {
        let run = case.run(args);
        run.expect_code(3);
        let err = run.stderr();
        assert!(
            err.contains("error: cannot migrate Go spela's profile "),
            "{err}"
        );
        assert!(
            err.contains("hint: Fix the YAML or remove the file"),
            "{err}"
        );
    }
    assert!(!case.profile(9100030).exists());
    assert!(!case.profile(9100060).exists());
    let run = case.run(&["profile", "follow", "gpu.fan_speed"]);
    run.expect_code(1);
    let out = run.stdout();
    assert!(
        out.starts_with("Not saved for Paper Comet: Director's Cut (cannot migrate "),
        "{out}"
    );
    assert!(out.contains("Quiet Orchard (cannot migrate "), "{out}");
    assert!(!case.profile(9100030).exists());
    // The profiles that did migrate still take writes.
    case.ok(&["gpu", "set", "9100010", "--fan-speed", "1"]);
    assert_eq!(case.go_files(), go, "the Go files stay byte-identical");
}

#[test]
fn a_go_config_that_does_not_migrate_exits_3() {
    let case = Case::new("config/broken");
    for args in [
        &["config", "set", "check_updates", "false"][..],
        &["profile", "reset", "--all-games"][..],
    ] {
        let run = case.run(args);
        run.expect_code(3);
        let err = run.stderr();
        assert!(
            err.contains("error: cannot migrate Go spela's config file "),
            "{err}"
        );
        assert!(
            err.contains("hint: Fix the YAML or remove the file to restore defaults."),
            "{err}"
        );
    }
    assert!(!case.config().join("config.toml").exists());
}

#[test]
fn a_go_overrides_file_that_does_not_migrate_blocks_deny_list_writes() {
    let case = Case::new("sandbox/empty");
    let lists = case.config().join("lists");
    std::fs::create_dir_all(&lists).unwrap();
    std::fs::write(lists.join("overrides.yaml"), "allowed: [\n").unwrap();
    for args in [
        &["denylist", "allow", "9100010"][..],
        &["denylist", "deny", "9100010"][..],
        &["denylist", "clear", "9100010"][..],
        &["denylist", "show"][..],
        &["denylist", "check", "9100010"][..],
    ] {
        let run = case.run(args);
        run.expect_code(3);
        let err = run.stderr();
        assert!(
            err.contains("error: cannot migrate Go spela's deny list "),
            "{err}"
        );
        assert!(
            err.contains("hint: Fix the YAML or the folder's permissions")
                && !err.contains("remove the file"),
            "the hint never says to delete a deny list: {err}"
        );
    }
    assert!(!lists.join("overrides.toml").exists());
}

#[test]
fn an_empty_home_finds_no_game_and_writes_no_profile() {
    let case = Case::new("sandbox/empty");
    let run = case.run(&["gpu", "set", "9100010", "--fan-speed", "1"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: game not found: 9100010\nhint: run 'spela scan' to refresh the game list\n"
    );
    assert!(!case.config().join("profiles").exists());
    // All games needs no game.
    case.ok(&["gpu", "set", "--all-games", "--fan-speed", "30"]);
    assert_eq!(
        case.read(&case.config().join("profiles/all-games.toml")),
        "version = 1\n\n[gpu]\nfan_speed = 30\n"
    );
}

#[test]
fn every_write_run_twice_changes_nothing_the_second_time() {
    let case = Case::new("sandbox/main");
    let go = case.go_files();
    for args in [
        &[
            "gpu",
            "set",
            "9100060",
            "--fan-speed",
            "0",
            "--vrr",
            "never",
        ][..],
        &["dlss", "set", "--all-games", "--rr-preset", "K"][..],
        &[
            "cpu",
            "set",
            "9100070",
            "--smt",
            "on",
            "--governor",
            "default",
        ][..],
        &["proton", "reset", "9100010", "hdr"][..],
        &["gpu", "profile-reset", "9100010"][..],
        &["profile", "reset", "9100060"][..],
        &["profile", "follow", "dlss.sr_mode", "cpu.smt"][..],
        &["profile", "delete", "9100020"][..],
        &["denylist", "allow", "1517290"][..],
        &["denylist", "deny", "9100010", "--reason", "testing"][..],
        &["denylist", "clear", "9100070"][..],
        &[
            "config",
            "set",
            "additional_library_paths",
            "/games/a:/games/b",
        ][..],
    ] {
        case.ok(args);
        let before = case.files();
        case.ok(args);
        assert_eq!(case.files(), before, "{args:?} wrote again");
    }
    let before = case.files();
    case.run(&["profile", "create", "9100010"]).expect_code(1);
    assert_eq!(case.files(), before);
    assert_eq!(case.go_files(), go, "the Go files stay byte-identical");
}

#[test]
fn a_go_file_that_keeps_failing_is_retried_without_rewriting_anything() {
    let case = Case::new("sandbox/errors");
    let args = ["gpu", "set", "9100010", "--fan-speed", "1"];
    case.ok(&args);
    let before = case.files();
    assert!(before.contains_key(Path::new("data/spela/migration.toml")));
    let again = case.run(&args);
    again.expect_code(0);
    assert!(
        !again.stderr().contains("Migrated"),
        "no notice: {}",
        again.stderr()
    );
    assert_eq!(
        case.files(),
        before,
        "the record and the files stay as they are"
    );
}

#[test]
fn yml_and_other_names_go_never_loaded_are_skipped() {
    let case = Case::new("profiles/file-names");
    let go = case.go_files();
    let out = case.ok(&["profile", "follow", "gpu.fan_speed"]);
    assert_eq!(out, "Fan speed: 0 games follow All games\n");
    let profiles: Vec<String> = std::fs::read_dir(case.config().join("profiles"))
        .unwrap()
        .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
        .filter(|n| n.ends_with(".toml"))
        .collect();
    assert_eq!(profiles, ["9100128.toml"], "only <appid>.yaml migrates");
    let record = case.read(&case.root.join("data/spela/migration.toml"));
    for name in ["9100129.yml", "default.yml", "09100130.yaml", "notes.yaml"] {
        let entry = record
            .split("[[source]]")
            .find(|e| e.contains(&format!("/{name}\"")))
            .unwrap_or_else(|| panic!("{name} is not in the record: {record}"));
        assert!(entry.contains("result = \"skipped\""), "{name}: {entry}");
    }
    assert_eq!(case.go_files(), go, "the Go files stay byte-identical");
}
