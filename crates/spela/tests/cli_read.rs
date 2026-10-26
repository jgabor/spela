//! The read-only commands of Phase 2, run as the real binary in the
//! sandbox: `scan`, `list`, `show`, `profile list` and `show`, `config
//! show`, the section shows, `cpu governor` and `cpu smt` with no
//! argument, `dll list`, `dll check-updates` and the deny list reads.
//!
//! Each test builds a small fictional Steam install under the sandbox HOME
//! (decision 43) and spela's files under the sandbox XDG dirs. The GPU and
//! CPU answers come from the fake backend.

mod common;

use std::path::{Path, PathBuf};

use common::Sandbox;
use serde_json::Value;

/// A fictional Steam install under the sandbox HOME.
struct Steam {
    root: PathBuf,
}

impl Steam {
    fn new(s: &Sandbox) -> Steam {
        let root = s.home().join(".steam/steam");
        std::fs::create_dir_all(root.join("steamapps/common")).unwrap();
        std::fs::write(
            root.join("steamapps/libraryfolders.vdf"),
            format!(
                "\"libraryfolders\"\n{{\n\t\"0\"\n\t{{\n\t\t\"path\"\t\t\"{}\"\n\t}}\n}}\n",
                root.display()
            ),
        )
        .unwrap();
        Steam { root }
    }

    /// An installed game with these files under its folder.
    fn game(&self, app_id: u64, name: &str, dir: &str, files: &[&str]) -> &Steam {
        std::fs::write(
            self.root.join(format!("steamapps/appmanifest_{app_id}.acf")),
            format!(
                "\"AppState\"\n{{\n\t\"appid\"\t\t\"{app_id}\"\n\t\"name\"\t\t\"{name}\"\n\t\"StateFlags\"\t\t\"4\"\n\t\"installdir\"\t\t\"{dir}\"\n}}\n"
            ),
        )
        .unwrap();
        let folder = self.root.join("steamapps/common").join(dir);
        std::fs::create_dir_all(&folder).unwrap();
        for file in files {
            let path = folder.join(file);
            std::fs::create_dir_all(path.parent().unwrap()).unwrap();
            std::fs::write(path, b"not a real DLL").unwrap();
        }
        self
    }

    fn common(&self, dir: &str) -> PathBuf {
        self.root.join("steamapps/common").join(dir)
    }
}

fn config_dir(s: &Sandbox) -> PathBuf {
    s.config_dir()
}

fn cache_dir(s: &Sandbox) -> PathBuf {
    s.root().join("xdg-cache/spela")
}

fn write(path: &Path, text: &str) {
    std::fs::create_dir_all(path.parent().unwrap()).unwrap();
    std::fs::write(path, text).unwrap();
}

fn json(run: &common::Run) -> Value {
    serde_json::from_str(&run.stdout()).unwrap_or_else(|e| panic!("{e}: {}", run.stdout()))
}

/// Three games: DLSS, XeSS only, and FSR only, plus a Steam tool.
fn three_games(s: &Sandbox) -> Steam {
    let steam = Steam::new(s);
    steam
        .game(9100410, "Amber Reach", "Amber", &["bin/nvngx_dlss.dll"])
        .game(9100420, "Cobalt Drift", "Cobalt", &["libxess.dll"])
        .game(
            9100430,
            "Birch Hollow",
            "Birch",
            &["amd_fidelityfx_dx12.dll"],
        )
        .game(9100440, "Lantern Path", "Lantern", &[])
        .game(1493710, "Proton Experimental", "Proton - Experimental", &[]);
    steam
}

#[test]
fn scan_counts_every_dll_family_and_writes_the_cache() {
    let s = Sandbox::new();
    three_games(&s);
    let run = s.run(&["scan"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "Found 4 games\n\
         Games with DLSS/FSR/XeSS: 3\n\
         \x20 - Amber Reach (9100410)\n\
         \x20     nvngx_dlss.dll: unknown\n\
         \x20 - Birch Hollow (9100430)\n\
         \x20     amd_fidelityfx_dx12.dll: unknown\n\
         \x20 - Cobalt Drift (9100420)\n\
         \x20     libxess.dll: unknown\n",
        "XeSS-only and FSR-only games count (decision 8), by name; the tool is dropped"
    );
    let cache: Value =
        serde_json::from_str(&std::fs::read_to_string(cache_dir(&s).join("games.json")).unwrap())
            .unwrap();
    assert_eq!(cache["games"].as_array().unwrap().len(), 4);
}

#[test]
fn scan_json_lists_games_without_tools_in_schema_1() {
    let s = Sandbox::new();
    three_games(&s);
    let run = s.run(&["scan", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["schema_version"], 1);
    let names: Vec<&str> = v["games"]
        .as_array()
        .unwrap()
        .iter()
        .map(|g| g["name"].as_str().unwrap())
        .collect();
    assert_eq!(
        names,
        [
            "Amber Reach",
            "Birch Hollow",
            "Cobalt Drift",
            "Lantern Path"
        ]
    );
    let amber = &v["games"][0];
    assert_eq!(amber["app_id"], 9100410);
    assert_eq!(amber["prefix_path"], Value::Null);
    assert_eq!(amber["has_profile"], false);
    assert_eq!(amber["dlls"][0]["type"], "dlss");
    assert_eq!(amber["dlls"][0]["version"], Value::Null);
    assert!(
        amber["scanned_at"].as_str().unwrap().ends_with('Z'),
        "{amber}"
    );
    assert_eq!(v["games"][3]["dlls"], serde_json::json!([]));
}

#[test]
fn list_scans_only_an_empty_cache() {
    let s = Sandbox::new();
    let steam = Steam::new(&s);
    steam.game(9100410, "Amber Reach", "Amber", &[]);
    let run = s.run(&["list"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "Amber Reach (9100410)\n",
        "the empty cache was scanned"
    );
    // A new game appears in Steam. The cache is not empty, so list and
    // show keep to it until `spela scan` (decision 24).
    steam.game(9100420, "Cobalt Drift", "Cobalt", &[]);
    assert_eq!(s.run(&["list"]).stdout(), "Amber Reach (9100410)\n");
    s.run(&["show", "9100420"]).expect_code(1);
    s.run(&["scan"]).expect_code(0);
    assert_eq!(
        s.run(&["list"]).stdout(),
        "Amber Reach (9100410)\nCobalt Drift (9100420)\n"
    );
}

#[test]
fn an_empty_library_lists_nothing_and_json_is_an_empty_list() {
    let s = Sandbox::new();
    Steam::new(&s);
    let run = s.run(&["list"]);
    run.expect_code(0);
    assert_eq!(run.stdout(), "No games found. Run 'spela scan' first.\n");
    let run = s.run(&["list", "--json"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "{\n  \"schema_version\": 1,\n  \"games\": []\n}\n",
        "Go printed the text even with --json (decision 8)"
    );
}

#[test]
fn no_steam_is_go_s_error_with_a_hint() {
    let s = Sandbox::new();
    let run = s.run(&["list"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: failed to refresh game database: could not find Steam installation\n\
         hint: set steam_path with 'spela config set steam_path <folder>'\n"
    );
    let run = s.run(&["scan", "--json"]);
    run.expect_code(1);
    assert_eq!(json(&run)["error"]["code"], "no_steam");
}

#[test]
fn list_with_dlls_and_show_print_go_s_lines() {
    let s = Sandbox::new();
    let steam = three_games(&s);
    s.run(&["scan"]).expect_code(0);
    let run = s.run(&["list", "--with-dlls"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "Amber Reach (9100410)\n  nvngx_dlss.dll: unknown\n\
         Birch Hollow (9100430)\n  amd_fidelityfx_dx12.dll: unknown\n\
         Cobalt Drift (9100420)\n  libxess.dll: unknown\n\
         Lantern Path (9100440)\n"
    );
    let run = s.run(&["show", "Amber Reach"]);
    run.expect_code(0);
    let amber = steam.common("Amber");
    assert_eq!(
        run.stdout(),
        format!(
            "Name:  Amber Reach\nApp ID: 9100410\nInstall:  {}\n\n\
             Detected DLLs:\n  nvngx_dlss.dll: unknown\n    {}\n\nProfile: no\n",
            amber.display(),
            amber.join("bin/nvngx_dlss.dll").display()
        )
    );
    let run = s.run(&["show", "amber reach"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: game not found: amber reach\nhint: run 'spela scan' to refresh the game list\n",
        "names match exactly, as in Go"
    );
    let run = s.run(&["--json", "show", "nope"]);
    run.expect_code(1);
    assert_eq!(json(&run)["error"]["code"], "game_not_found");
}

#[test]
fn show_json_has_the_game_and_its_profile_flag() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    write(
        &config_dir(&s).join("profiles/9100420.toml"),
        "version = 1\nname = \"Cobalt Drift\"\n",
    );
    let run = s.run(&["show", "9100420", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["schema_version"], 1);
    assert_eq!(v["game"]["name"], "Cobalt Drift");
    assert_eq!(v["game"]["has_profile"], true);
    assert_eq!(v["game"]["dlls"][0]["type"], "xess");
}

fn profiles(s: &Sandbox) {
    write(
        &config_dir(s).join("profiles/all-games.toml"),
        "version = 1\n\n[proton]\nenable_wayland = true\n\n[cpu]\ngovernor = \"performance\"\n",
    );
    write(
        &config_dir(s).join("profiles/9100410.toml"),
        "version = 1\nname = \"Amber Reach\"\n\n[proton]\nenable_hdr = true\nvkd3d_heap = true\n\n\
         [gpu]\npower_limit = 250\nthreaded_optimization = false\n",
    );
}

#[test]
fn profile_show_resolves_every_field_with_its_source() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    profiles(&s);
    let run = s.run(&["profile", "show", "9100410"]);
    run.expect_code(0);
    let text = run.stdout();
    assert!(
        text.starts_with("Profile for Amber Reach:\n\nProton\n"),
        "{text}"
    );
    for line in [
        "  HDR          On   this game · All games: Off\n",
        "  Wayland      On   from All games\n",
        "  NGX updater  Off  system default\n",
        "  Power limit            250 W  this game · All games: none\n",
        "  Threaded optimization  Off    this game · same as All games\n",
        "  Governor  performance  from All games\n",
        "  SMT       none         system default\n",
    ] {
        assert!(text.contains(line), "{line:?} in\n{text}");
    }
    for gone in ["Overlay", "Power mode", "Affinity"] {
        assert!(!text.contains(gone), "{gone} is gone (decision 10)");
    }
    for retired in spela_core::profile::words::RETIRED {
        assert!(!text.contains(retired), "{retired}");
    }
    let run = s.run(&["profile", "show", "Lantern Path"]);
    run.expect_code(0);
    assert!(
        run.stdout().contains(
            "No profile for Lantern Path; every value follows All games or the system default.\n\n"
        ),
        "{}",
        run.stdout()
    );
}

#[test]
fn profile_show_json_works_without_a_profile_and_for_all_games() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    profiles(&s);
    let run = s.run(&["profile", "show", "9100440", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["scope"], "game");
    assert_eq!(v["app_id"], 9100440);
    assert_eq!(v["name"], "Lantern Path");
    assert_eq!(v["has_profile"], false, "Go failed here; spela resolves");
    assert_eq!(v["fields"].as_array().unwrap().len(), 25);
    let run = s.run(&["profile", "show", "--all-games", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    assert_eq!(v["scope"], "all_games");
    assert_eq!(v["app_id"], Value::Null);
    assert_eq!(v["name"], Value::Null);
    assert_eq!(v["has_profile"], true);
    let origins: Vec<&str> = v["fields"]
        .as_array()
        .unwrap()
        .iter()
        .map(|f| f["origin"].as_str().unwrap())
        .collect();
    assert!(
        origins
            .iter()
            .all(|o| *o == "set_here" || *o == "system_default")
    );
    assert_eq!(origins.iter().filter(|o| **o == "set_here").count(), 2);
    let run = s.run(&["profile", "show", "--all-games"]);
    run.expect_code(0);
    assert!(
        run.stdout().contains("  Wayland      On   set here\n"),
        "{}",
        run.stdout()
    );
}

#[test]
fn section_shows_print_their_section_only() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    profiles(&s);
    let run = s.run(&["gpu", "show", "9100410"]);
    run.expect_code(0);
    let text = run.stdout();
    assert!(
        text.starts_with("GPU profile for Amber Reach:\n\nClock offset "),
        "{text}"
    );
    assert_eq!(text.lines().count(), 2 + 8, "{text}");
    let run = s.run(&["cpu", "show", "9100410", "--json"]);
    run.expect_code(0);
    let v = json(&run);
    let keys: Vec<&str> = v["fields"]
        .as_array()
        .unwrap()
        .iter()
        .map(|f| f["key"].as_str().unwrap())
        .collect();
    assert_eq!(keys, ["cpu.governor", "cpu.smt"]);
    assert_eq!(v["fields"][0]["all_games_value"], "performance");
    let run = s.run(&["dlss", "show", "--all-games"]);
    run.expect_code(0);
    assert!(
        run.stdout()
            .starts_with("DLSS configuration for All games:\n\n"),
        "{}",
        run.stdout()
    );
}

#[test]
fn proton_show_checks_the_descriptor_heap_when_it_is_on() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    profiles(&s);
    // No config.vdf maps a tool, so the check cannot run.
    let run = s.run(&["proton", "show", "9100410"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "Proton profile for Amber Reach:\n\n\
         HDR          On   this game · All games: Off\n\
         Wayland      On   from All games\n\
         NGX updater  Off  system default\n\
         VKD3D heap   On   this game · All games: Off\n\
         \x20   ⓘ could not resolve active Proton for this game; skipping compatibility check\n"
    );
    // vkd3d_heap off: no notice.
    let run = s.run(&["proton", "show", "9100420"]);
    run.expect_code(0);
    assert!(!run.stdout().contains("compatibility"), "{}", run.stdout());
}

#[test]
fn profile_list_names_games_by_cache_name() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    assert_eq!(s.run(&["profile", "list"]).stdout(), "No profiles found.\n");
    profiles(&s);
    write(
        &config_dir(&s).join("profiles/9100499.toml"),
        "version = 1\n",
    );
    write(
        &config_dir(&s).join("profiles/9100430.toml"),
        "version = 1\n[gpu]\nvrr = \"sometimes\"\n",
    );
    let run = s.run(&["profile", "list"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "9100499 (9100499): custom\nAmber Reach (9100410): Amber Reach\n"
    );
    assert!(
        run.stderr().contains("9100430"),
        "a broken profile is named: {}",
        run.stderr()
    );
}

#[test]
fn a_broken_profile_exits_3_and_names_the_file() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    let path = config_dir(&s).join("profiles/9100410.toml");
    write(&path, "version = 1\n[gpu]\npower_limit = \"lots\"\n");
    for args in [
        &["profile", "show", "9100410"][..],
        &["gpu", "show", "9100410"][..],
    ] {
        let run = s.run(args);
        run.expect_code(3);
        assert!(
            run.stderr().contains(&path.display().to_string()),
            "{}",
            run.stderr()
        );
    }
    let run = s.run(&["profile", "show", "9100410", "--json"]);
    run.expect_code(3);
    assert_eq!(json(&run)["error"]["code"], "config_invalid");
    // show still works and says the game has a profile.
    assert!(
        s.run(&["show", "9100410"])
            .stdout()
            .ends_with("Profile: yes\n")
    );
}

#[test]
fn go_files_migrate_first_and_a_broken_go_config_exits_3() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    write(
        &config_dir(&s).join("profiles/9100410.yaml"),
        "name: Amber Reach\ngpu:\n    power_limit: 200\noverrides:\n    gpu.power_limit: true\n",
    );
    let run = s.run(&["gpu", "show", "9100410"]);
    run.expect_code(0);
    assert!(
        run.stderr().starts_with("Migrated 1 file from Go spela"),
        "{}",
        run.stderr()
    );
    assert!(
        run.stdout().contains("Power limit            200 W"),
        "{}",
        run.stdout()
    );
    // The second run has nothing to migrate.
    assert_eq!(s.run(&["gpu", "show", "9100410"]).stderr(), "");
    let go_config = config_dir(&s).join("config.yaml");
    write(&go_config, "log_level: [broken\n");
    let run = s.run(&["list"]);
    run.expect_code(3);
    let err = run.stderr();
    assert!(err.contains(&go_config.display().to_string()), "{err}");
    assert!(
        err.ends_with("hint: Fix the YAML or remove the file to restore defaults.\n"),
        "{err}"
    );
}

#[test]
fn demo_never_migrates_or_reads_the_user_s_steam() {
    let s = Sandbox::new();
    three_games(&s);
    write(
        &config_dir(&s).join("profiles/9100410.yaml"),
        "name: Amber Reach\n",
    );
    let run = s.run(&["--demo", "list"]);
    assert!(!run.stdout().contains("Amber Reach"), "{}", run.stdout());
    assert!(!run.stderr().contains("Migrated"), "{}", run.stderr());
    assert!(!config_dir(&s).join("profiles/9100410.toml").exists());
}

#[test]
fn demo_cli_commands_run_on_the_built_in_games_and_hardware() {
    let s = Sandbox::new();
    let run = s.run(&["--demo", "list"]);
    run.expect_code(0);
    assert!(
        run.stdout().contains("Ashfall Courier (9100110)"),
        "{}",
        run.stdout()
    );
    // `dll list` never scans, so the demo's own cache must hold its games.
    let run = s.run(&["--demo", "dll", "list"]);
    run.expect_code(0);
    assert!(run.stdout().contains("nvngx_dlss.dll"), "{}", run.stdout());
    let run = s.run(&["--demo", "launch", "--dry-run", "Ashfall Courier"]);
    run.expect_code(0);
    assert!(
        run.stdout()
            .contains("Game: Ashfall Courier (AppID 9100110)"),
        "{}",
        run.stdout()
    );
    // The live CPU commands change only the demo's fictional sysfs tree.
    let run = s.run(&["--demo", "cpu", "governor", "performance"]);
    run.expect_code(0);
    assert!(
        run.stdout().contains("Governor set to performance"),
        "{}",
        run.stdout()
    );
    let run = s.run(&["--demo", "cpu", "smt", "off"]);
    run.expect_code(0);
    assert!(run.stdout().contains("SMT disabled"), "{}", run.stdout());
    // `cpu info` reads the demo's fictional procfs and sysfs files.
    let run = s.run(&["--demo", "cpu", "info"]);
    run.expect_code(0);
    assert!(
        run.stdout().contains("Demo CPU 8-Core Processor"),
        "{}",
        run.stdout()
    );
    assert!(run.stdout().contains("powersave"), "{}", run.stdout());
}

#[test]
fn config_show_prints_every_key_as_toml() {
    let s = Sandbox::new();
    write(
        &config_dir(&s).join("config.toml"),
        "check_updates = false\n[ui]\ntheme = \"mono\"\n",
    );
    let run = s.run(&["config", "show"]);
    run.expect_code(0);
    let text = run.stdout();
    let parsed: toml::Table = text.parse().unwrap_or_else(|e| panic!("{e}\n{text}"));
    assert_eq!(parsed["check_updates"].as_bool(), Some(false));
    assert_eq!(parsed["ui"]["theme"].as_str(), Some("mono"));
    assert_eq!(parsed["manifest_refresh_hours"].as_integer(), Some(24));
    let shader = s.root().join("xdg-cache/spela/nvidia");
    assert_eq!(
        parsed["shader_cache"].as_str(),
        Some(shader.to_str().unwrap()),
        "an empty shader_cache prints the folder it means"
    );
    assert!(
        text.starts_with("version = 1\nlog_level = \"info\"\n"),
        "{text}"
    );
    write(
        &config_dir(&s).join("config.toml"),
        "log_level = \"loud\"\n",
    );
    let run = s.run(&["config", "show"]);
    run.expect_code(3);
    assert!(
        run.stderr().contains("unsupported log level: loud"),
        "{}",
        run.stderr()
    );
}

#[test]
fn dll_list_shows_every_family_and_one_game() {
    let s = Sandbox::new();
    let steam = three_games(&s);
    s.run(&["scan"]).expect_code(0);
    let run = s.run(&["dll", "list"]);
    run.expect_code(0);
    let stdout = run.stdout();
    let names: Vec<&str> = stdout.lines().filter(|l| !l.starts_with(' ')).collect();
    assert_eq!(
        names,
        [
            "Amber Reach (9100410)",
            "Birch Hollow (9100430)",
            "Cobalt Drift (9100420)"
        ],
        "XeSS-only and FSR-only games are listed (decision 8)"
    );
    let run = s.run(&["dll", "list", "Cobalt Drift"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        format!(
            "Cobalt Drift (9100420)\n  libxess.dll: unknown\n    {}\n",
            steam.common("Cobalt").join("libxess.dll").display()
        )
    );
    assert_eq!(
        s.run(&["dll", "list", "9100440"]).stdout(),
        "Lantern Path (9100440)\n"
    );
}

#[test]
fn dll_check_updates_falls_back_to_the_cached_manifest() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    // The fake network has nothing, and there is no cache to fall back to.
    let run = s.run(&["dll", "check-updates"]);
    run.expect_code(1);
    assert!(
        run.stderr().contains("error: failed to fetch the DLL manifest from https://raw.githubusercontent.com/jgabor/spela/main/data/manifest.json: "),
        "{}",
        run.stderr()
    );
    write(
        &cache_dir(&s).join("manifest-v1.json"),
        r#"{"version":"1","updated_at":"2026-10-26T15:00:00Z","dlls":{
            "dlss":[{"version":"310.5.3","url":"https://example.invalid/a","sha256":"aa","filename":"nvngx_dlss.dll"}],
            "xess":[{"version":"1.3.1","url":"https://example.invalid/b","sha256":"bb","filename":"libxess.dll"}]
        },"fetched_at":"2026-10-26T15:00:00Z","source_url":"http://127.0.0.1:9/"}"#,
    );
    // The cache came from another URL, so the policy fetches; the fetch
    // fails and the cache stands in, with a warning.
    let run = s.run(&["dll", "check-updates"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "Amber Reach (9100410)\n  nvngx_dlss.dll: unknown -> 310.5.3\n\
         Cobalt Drift (9100420)\n  libxess.dll: unknown -> 1.3.1\n",
        "XeSS counts; FSR has no manifest entry"
    );
    assert!(
        run.stderr()
            .contains("using the cached DLL manifest: cannot fetch"),
        "{}",
        run.stderr()
    );
}

#[test]
fn denylist_show_and_check_read_the_lists() {
    let s = Sandbox::new();
    three_games(&s);
    s.run(&["scan"]).expect_code(0);
    write(
        &config_dir(&s).join("lists/overrides.toml"),
        "version = 1\nallowed = [1172470]\n\n[[denied]]\napp_id = 9100420\nname = \"Cobalt Drift\"\nreason = \"kernel anti-cheat\"\n",
    );
    let run = s.run(&["denylist", "show"]);
    run.expect_code(0);
    let text = run.stdout();
    assert!(text.starts_with("Denied games (anti-cheat):\n  Apex Legends (1172470): Easy Anti-Cheat [FORCE-ALLOWED]\n"), "{text}");
    assert!(
        text.contains("  PUBG: Battlegrounds (578080): Anti-Cheat\n"),
        "{text}"
    );
    assert!(
        text.ends_with("\nUser-denied games:\n  Cobalt Drift (9100420): kernel anti-cheat\n"),
        "{text}"
    );
    assert_eq!(
        s.run(&["denylist", "check", "Cobalt Drift"]).stdout(),
        "Cobalt Drift is DENIED: kernel anti-cheat\nUse 'spela denylist allow' to force-allow (not recommended)\n"
    );
    assert_eq!(
        s.run(&["denylist", "check", "9100410"]).stdout(),
        "Amber Reach is allowed for DLL swapping\n"
    );
    assert_eq!(
        s.run(&["denylist", "check", "252490"]).stdout(),
        "252490 is DENIED: Easy Anti-Cheat\nUse 'spela denylist allow' to force-allow (not recommended)\n",
        "an unknown AppID is checked as it is"
    );
    s.run(&["denylist", "check", "nope"]).expect_code(1);
    // --json is accepted, but these commands have no JSON results in
    // schema version 1, so success prints the text.
    let run = s.run(&["denylist", "show", "--json"]);
    run.expect_code(0);
    assert!(run.stdout().starts_with("Denied games"), "{}", run.stdout());
}

#[test]
fn deny_lists_fail_closed() {
    let s = Sandbox::new();
    let overrides = config_dir(&s).join("lists/overrides.toml");
    write(&overrides, "allowed = [\"x\"\n");
    for args in [
        &["denylist", "show"][..],
        &["denylist", "check", "252490"][..],
    ] {
        let run = s.run(args);
        run.expect_code(3);
        assert!(
            run.stderr().contains(&overrides.display().to_string()),
            "{}",
            run.stderr()
        );
        assert!(run.stdout().is_empty());
    }
    std::fs::remove_file(&overrides).unwrap();
    // A Go list that does not migrate leaves no TOML behind. The built-in
    // list alone would let the user's denials through, so it is an error.
    let go_list = config_dir(&s).join("lists/overrides.yaml");
    write(&go_list, "denied: [broken\n");
    let run = s.run(&["denylist", "check", "9100420"]);
    run.expect_code(3);
    assert!(
        run.stderr().contains(&go_list.display().to_string()),
        "{}",
        run.stderr()
    );
    let run = s.run(&["--json", "denylist", "show"]);
    run.expect_code(3);
    assert_eq!(json(&run)["error"]["code"], "list_invalid");
}

#[test]
fn cpu_governor_and_smt_read_the_fake_cpu() {
    let s = Sandbox::new();
    let run = s.run(&["cpu", "governor"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "Current governor: powersave\nAvailable: [performance powersave]\n"
    );
    let run = s.run(&["cpu", "smt"]);
    run.expect_code(0);
    assert_eq!(run.stdout(), "SMT: true\n");
}

#[test]
fn gpu_and_cpu_info_read_the_fake() {
    let s = Sandbox::new();
    let run = s.run(&["gpu", "info"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "GPU:  Demo GPU 4080\nDriver:  580.95.05\nVRAM:  16376 MB\nTemp:  41°C\n\
         Power:  22.4 W / 320 W limit (100–360 W range)\nCore:  285 MHz\nMem:  10501 MHz\n\
         Fan:  0%\nLoad:  3%\n"
    );
    let run = s.run(&["cpu", "info"]);
    run.expect_code(0);
    assert_eq!(
        run.stdout(),
        "CPU:  Demo CPU 8-Core Processor\nCores:  16\n\
         Governor:  powersave (performance, powersave available)\nSMT:  enabled\n\
         Freq:  3420 MHz avg\nLoad:  12%\nRAM:  9800 / 32000 MB\n"
    );
    // The sandbox's drop checks that neither ran nvidia-smi, systemctl or
    // scx_loader.
}

#[test]
fn the_live_commands_run_the_fake_helper_and_take_the_launch_lock() {
    let s = Sandbox::new();
    let run = s.run(&["gpu", "reset"]);
    run.expect_code(0);
    assert_eq!(run.stdout(), "GPU clocks reset to default\n");
    // A typo is a usage error; Go turned SMT off.
    let run = s.run(&["cpu", "smt", "of"]);
    run.expect_code(2);
    assert_eq!(
        run.stderr(),
        "error: invalid SMT value: of (use on or off)\n"
    );
    // A governor sysfs does not list is refused before any prompt.
    let run = s.run(&["cpu", "governor", "turbo"]);
    run.expect_code(1);
    assert_eq!(
        run.stderr(),
        "error: failed to set governor: CPU governor \"turbo\" is not available \
         (available: performance, powersave)\n"
    );
    let lock = s.state_dir().join("locks");
    assert!(lock.is_dir(), "the live commands took the launch lock");
}

/// A game cache written by hand with fixed times, so the JSON snapshots
/// stay the same on every run.
fn fixed_cache(s: &Sandbox) {
    let lib = s.root().join("lib");
    let common = lib.join("steamapps/common");
    let cache = serde_json::json!({
        "version": 1,
        "updated_at": "2026-10-26T15:02:11Z",
        "games": [
            {
                "app_id": 9100410,
                "name": "Amber Reach",
                "install_dir": common.join("Amber"),
                "library_path": lib,
                "prefix_path": lib.join("steamapps/compatdata/9100410"),
                "scanned_at": "2026-10-26T17:02:10.5+02:00",
                "dlls": [
                    {"path": common.join("Amber/bin/nvngx_dlssg.dll"), "name": "nvngx_dlssg.dll", "type": "dlssg", "version": null},
                    {"path": common.join("Amber/bin/nvngx_dlss.dll"), "name": "nvngx_dlss.dll", "type": "dlss", "version": "310.5.3"}
                ]
            },
            {
                "app_id": 9100420,
                "name": "Cobalt Drift",
                "install_dir": common.join("Cobalt"),
                "library_path": lib,
                "scanned_at": "2026-10-26T15:02:10Z",
                "dlls": [
                    {"path": common.join("Cobalt/libxess.dll"), "name": "libxess.dll", "type": "xess", "version": "1.3.1"}
                ]
            }
        ]
    });
    write(&cache_dir(s).join("games.json"), &cache.to_string());
}

#[test]
fn json_output_is_pinned_by_snapshots() {
    let s = Sandbox::new();
    fixed_cache(&s);
    profiles(&s);
    let root = s.root().to_str().unwrap().to_owned();
    for (name, args) in [
        ("list_json", &["list", "--json"][..]),
        ("show_json", &["show", "9100410", "--json"][..]),
        (
            "profile_show_json",
            &["profile", "show", "9100410", "--json"][..],
        ),
        (
            "profile_show_all_games_json",
            &["profile", "show", "--all-games", "--json"][..],
        ),
        ("gpu_show_json", &["gpu", "show", "9100420", "--json"][..]),
    ] {
        let run = s.run(args);
        run.expect_code(0);
        assert!(run.stderr().is_empty(), "{name}: {}", run.stderr());
        insta::assert_snapshot!(name, run.stdout().replace(&root, "[ROOT]"));
    }
}

#[test]
fn list_and_show_load_the_config_even_with_a_full_cache() {
    let s = Sandbox::new();
    fixed_cache(&s);
    write(
        &config_dir(&s).join("config.toml"),
        "check_updates = \"maybe\"\n",
    );
    for args in [&["list"][..], &["show", "9100410"][..]] {
        let run = s.run(args);
        run.expect_code(3);
        assert!(run.stderr().contains("config.toml"), "{}", run.stderr());
    }
    // The commands that never read the config in Go still work.
    s.run(&["profile", "show", "9100410"]).expect_code(0);
}
