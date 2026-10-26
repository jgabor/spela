//! The read-only commands against the Go sandbox fixtures, compared with
//! the Go CLI's text transcripts (`tests/fixtures/go/transcripts`).
//!
//! Each transcript in `index.json` is either compared or skipped with a
//! reason, so a new transcript cannot be ignored without saying why. Each
//! compared command runs the real binary on a fresh copy of its case, as
//! goref ran the Go binary: the case is copied into the sandbox root, its
//! `/fixture-root` token is replaced by that root, and HOME and the XDG
//! dirs point into it. Every Go file is migrated on that first run.
//!
//! The Go output is turned into the output spela must print by the
//! differences listed below, each with its reason, and then compared
//! exactly. Nothing else may differ.

mod common;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use common::Sandbox;
use serde_json::Value;

const FIXTURE_ROOT: &str = "/fixture-root";

fn repo() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../..")
}

fn transcripts() -> PathBuf {
    repo().join("tests/fixtures/go/transcripts")
}

// The differences every compared command has.

/// Go writes ANSI colour even to a pipe; spela writes it only to a
/// terminal (architecture, "Output, errors and exit codes";
/// phase0-checks.md).
fn strip_ansi(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    let mut chars = text.chars().peekable();
    while let Some(c) = chars.next() {
        if c == '\x1b' && chars.peek() == Some(&'[') {
            chars.next();
            for c in chars.by_ref() {
                if c == 'm' {
                    break;
                }
            }
            continue;
        }
        out.push(c);
    }
    out
}

/// cobra printed `Error: <message>` and then the usage text; spela prints
/// `error: <message>` and no usage (architecture, "Output, errors and exit
/// codes"; terminal-ux.md, CLI output).
fn go_error(stderr: &str) -> String {
    match stderr.lines().next() {
        Some(first) => match first.strip_prefix("Error: ") {
            Some(message) => format!("error: {message}\n"),
            None => stderr.to_owned(),
        },
        None => String::new(),
    }
}

/// Lines only spela writes to stderr, and why they are allowed:
/// - `hint:` follows an error where one helps (architecture, "Output,
///   errors and exit codes");
/// - the migration notice on the run that migrates (architecture, "What is
///   dropped, and how spela says so", item 3);
/// - a warning for each Go file that does not migrate (architecture,
///   "Failure handling").
fn spela_only_stderr(line: &str) -> bool {
    line.starts_with("hint: ")
        || line.starts_with("Migrated ")
        || line.starts_with("GPU clock and memory offsets from Go spela were not carried over.")
        || line.starts_with(" WARN cannot migrate a Go spela file: ")
}

/// Rows and sections decision 10 drops: the Overlay section,
/// `gpu.power_mizer` (Power mode) and `cpu.affinity` (Affinity).
fn drop_retired_rows(text: &str) -> String {
    let mut out = Vec::new();
    let mut in_overlay = false;
    for line in text.split_inclusive('\n') {
        let trimmed = line.trim();
        if trimmed == "Overlay" {
            in_overlay = true;
            continue;
        }
        if in_overlay {
            if trimmed.is_empty() {
                in_overlay = false;
            }
            continue;
        }
        if trimmed.starts_with("Power mode ") || trimmed.starts_with("Affinity ") {
            continue;
        }
        out.push(line);
    }
    out.concat()
}

// The differences of single commands.

/// How spela's output differs from Go's for one command.
#[derive(Debug, Clone, Copy)]
enum Change {
    /// This stdout line replaces Go's.
    Line {
        go: &'static str,
        spela: &'static str,
    },
    /// spela exits with this code.
    Exit(i32),
    /// spela's error line replaces Go's, before the hint.
    Error(&'static str),
    /// With `--json` a failure prints only the error object on stdout,
    /// with this code, and nothing on stderr.
    JsonError(&'static str),
    /// The JSON is the new schema; the same facts are checked instead of
    /// bytes.
    Json,
    /// Go failed where spela prints the profile object.
    JsonInsteadOfError,
    /// The config prints as TOML; the same values are checked instead.
    Toml,
    /// Go printed games in map order; spela sorts them by name. The game
    /// blocks are compared as a set, and spela's must be sorted.
    SortedBlocks,
    /// A clap usage error: exit 2 and clap's own text.
    ClapUsage,
}

/// One allowed difference: which commands, what changes, and why.
struct Allowed {
    cases: &'static [&'static str],
    change: Change,
    why: &'static str,
}

const ALLOWED: &[Allowed] = &[
    Allowed {
        cases: &["sandbox/main/profile-show"],
        change: Change::Line {
            go: "  Clock offset           150 MHz    this game · All games: none",
            spela: "  Clock offset           none       system default",
        },
        why: "decision 31: Go's clock_offset was a clock lock, so the migration records it and does not carry it over",
    },
    Allowed {
        cases: &["sandbox/main/profile-show"],
        change: Change::Line {
            go: "  Memory offset          500 MHz    this game · All games: none",
            spela: "  Memory offset          none       system default",
        },
        why: "decision 31: Go's memory_offset was a clock limit, so it is not carried over",
    },
    Allowed {
        cases: &["sandbox/main/gpu-show"],
        change: Change::Line {
            go: "Clock offset           150 MHz    this game · All games: none",
            spela: "Clock offset           none       system default",
        },
        why: "decision 31, as in profile show",
    },
    Allowed {
        cases: &["sandbox/main/gpu-show"],
        change: Change::Line {
            go: "Memory offset          500 MHz    this game · All games: none",
            spela: "Memory offset          none       system default",
        },
        why: "decision 31, as in profile show",
    },
    Allowed {
        cases: &["sandbox/main/proton-show-unresolved-tool"],
        change: Change::Line {
            go: "    ⓘ could not resolve active Proton for this game; skipping compatibility check",
            spela: "    ⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ (detected: GE-Proton11-7-x86_64)",
        },
        why: "architecture, Proton: the resolver reads each tool's internal name from compatibilitytool.vdf, so GE-Proton11-7-x86_64 now finds its GE-Proton11-7 folder (phase0-checks.md, Proton tool folders)",
    },
    Allowed {
        cases: &["e2e/library/profile-show"],
        change: Change::Line {
            go: "  Resolution preset    quality   this game · All games: default",
            spela: "  Resolution preset    default   from All games",
        },
        why: "decision 8 and architecture, Profiles and inheritance: every enum is checked on load; Go stored sr_preset \"quality\", which is no preset, so the migration records it as not_supported instead of carrying it over",
    },
    Allowed {
        cases: &["e2e/library/dlss-show"],
        change: Change::Line {
            go: "Resolution preset    quality   this game · All games: default",
            spela: "Resolution preset    default   from All games",
        },
        why: "decision 8, as in profile show",
    },
    Allowed {
        cases: &[
            "sandbox/main/list-json",
            "sandbox/main/list-with-dlls-json",
            "sandbox/main/show-json",
            "sandbox/main/profile-show-json",
            "sandbox/main/profile-show-json-created",
            "sandbox/main/dlss-show-json",
            "sandbox/main/dlss-show-json-none",
            "sandbox/main/gpu-show-json",
            "sandbox/main/gpu-show-json-none",
            "sandbox/main/cpu-show-json",
            "sandbox/main/cpu-show-json-none",
            "sandbox/main/proton-show-json",
            "sandbox/main/proton-show-json-none",
        ],
        change: Change::Json,
        why: "decision 7: --json is the new snake_case schema version 1; the section shows print the profile object with both views",
    },
    Allowed {
        cases: &["sandbox/main/profile-show-json-none"],
        change: Change::JsonInsteadOfError,
        why: "architecture, JSON output: profile show --json works without a profile file, where Go failed",
    },
    Allowed {
        cases: &["sandbox/main/profile-show-json-none"],
        change: Change::Exit(0),
        why: "architecture, JSON output: profile show --json works without a profile file",
    },
    Allowed {
        cases: &["sandbox/main/config-show", "sandbox/empty/config-show"],
        change: Change::Toml,
        why: "decision 6: the config is TOML; theme and compact_mode are gone (decisions 3 and 46), and the new keys and [ui] are printed",
    },
    Allowed {
        cases: &["sandbox/main/dll-list", "sandbox/main/dll-check-updates"],
        change: Change::SortedBlocks,
        why: "architecture, Commands: dll list and check-updates sort games by name, where Go used map order",
    },
    Allowed {
        cases: &["sandbox/main/show-no-args"],
        change: Change::ClapUsage,
        why: "architecture, Output, errors and exit codes: clap's usage errors keep clap's format and exit 2",
    },
    Allowed {
        cases: &[
            "sandbox/errors/profile-show-broken-yaml",
            "sandbox/errors/proton-show-broken-yaml",
        ],
        change: Change::Error(
            "error: cannot migrate Go spela's profile /fixture-root/config/spela/profiles/9100030.yaml: line 2 column 7: expected mapping start",
        ),
        why: "architecture, Failure handling and Profiles and inheritance: a Go profile that does not migrate is a broken profile; the error names the file, and the YAML reader is serde-saphyr, not yaml.v3 (Reading YAML)",
    },
    Allowed {
        cases: &[
            "sandbox/errors/profile-show-invalid-vrr",
            "sandbox/errors/dlss-show-invalid-vrr",
        ],
        change: Change::Error(
            "error: cannot migrate Go spela's profile /fixture-root/config/spela/profiles/9100060.yaml: gpu.vrr: unsupported value \"sometimes\" (valid: , unset, automatic, always, never)",
        ),
        why: "architecture, Failure handling: the error names the file and keeps Go's value message",
    },
    Allowed {
        cases: &[
            "sandbox/errors/profile-show-broken-yaml",
            "sandbox/errors/proton-show-broken-yaml",
            "sandbox/errors/profile-show-invalid-vrr",
            "sandbox/errors/dlss-show-invalid-vrr",
            "sandbox/errors/profile-show-json-broken-yaml",
        ],
        change: Change::Exit(3),
        why: "architecture, Output, errors and exit codes: a profile file that is invalid exits 3",
    },
    Allowed {
        cases: &["sandbox/errors/profile-show-json-broken-yaml"],
        change: Change::JsonError("config_invalid"),
        why: "architecture, JSON output: with --json a failure prints only the error object on stdout",
    },
    Allowed {
        cases: &["sandbox/empty/list-json"],
        change: Change::JsonError("no_steam"),
        why: "architecture, JSON output: with --json a failure prints only the error object on stdout",
    },
];

/// Transcripts that are not compared here, and why.
fn skipped(name: &str) -> Option<&'static str> {
    let reasons: &[(&str, &str)] = &[
        (
            "version",
            "decision 48: clap's `spela 0.7.0`; tests/cli.rs pins it",
        ),
        ("root", "clap's help replaces cobra's; tests/cli.rs pins it"),
        ("help", "clap's help replaces cobra's; tests/cli.rs pins it"),
        (
            "unknown-command",
            "clap's usage error, exit 2; tests/cli.rs pins it",
        ),
        (
            "parent-",
            "a parent prints clap's help; tests/cli.rs pins it",
        ),
        (
            "overlay-",
            "decisions 10 and 45: the overlay command is removed",
        ),
        (
            "launch-",
            "launch --dry-run is not one of the read commands built here",
        ),
    ];
    reasons
        .iter()
        .find(|(prefix, _)| name.starts_with(prefix))
        .map(|(_, why)| *why)
}

/// One transcript.
#[derive(Debug)]
struct Case {
    id: String,
    scenario: String,
    args: Vec<String>,
    exit: i32,
    stdout: String,
    stderr: String,
}

fn cases() -> Vec<(Case, Option<&'static str>)> {
    let index: Value =
        serde_json::from_str(&std::fs::read_to_string(transcripts().join("index.json")).unwrap())
            .unwrap();
    let read = |rel: Option<&str>| {
        rel.map(|r| std::fs::read_to_string(transcripts().join(r)).unwrap())
            .unwrap_or_default()
    };
    index
        .as_array()
        .unwrap()
        .iter()
        .map(|entry| {
            let scenario = entry["scenario"].as_str().unwrap().to_owned();
            let name = entry["name"].as_str().unwrap();
            let case = Case {
                id: format!("{scenario}/{name}"),
                args: entry["args"]
                    .as_array()
                    .map(|a| a.iter().map(|v| v.as_str().unwrap().to_owned()).collect())
                    .unwrap_or_default(),
                exit: entry["exit_code"].as_i64().unwrap() as i32,
                stdout: strip_ansi(&read(entry["stdout"].as_str())),
                stderr: strip_ansi(&read(entry["stderr"].as_str())),
                scenario,
            };
            (case, skipped(name))
        })
        .collect()
}

/// Copies a fixture case into `to`, replacing the root token in text files.
fn copy_case(from: &Path, to: &Path, root: &str) {
    std::fs::create_dir_all(to).unwrap();
    for entry in std::fs::read_dir(from).unwrap() {
        let entry = entry.unwrap();
        let target = to.join(entry.file_name());
        let kind = entry.file_type().unwrap();
        if kind.is_dir() {
            copy_case(&entry.path(), &target, root);
        } else if kind.is_file() {
            let bytes = std::fs::read(entry.path()).unwrap();
            let bytes = match String::from_utf8(bytes) {
                Ok(text) => text.replace(FIXTURE_ROOT, root).into_bytes(),
                Err(e) => e.into_bytes(),
            };
            std::fs::write(&target, bytes).unwrap();
        }
    }
}

/// spela's output for one case, with the sandbox root put back as the
/// token.
struct Output {
    exit: i32,
    stdout: String,
    stderr: String,
}

fn run_spela(case: &Case) -> Output {
    let s = Sandbox::new();
    let root = s.root().to_str().unwrap().to_owned();
    copy_case(
        &repo().join("tests/fixtures/go").join(&case.scenario),
        s.root(),
        &root,
    );
    let config = s.root().join("config");
    let data = s.root().join("data");
    let cache = s.root().join("cache");
    let args: Vec<&str> = case.args.iter().map(String::as_str).collect();
    let run = s.run_with(
        &args,
        &[
            ("XDG_CONFIG_HOME", config.to_str().unwrap()),
            ("XDG_DATA_HOME", data.to_str().unwrap()),
            ("XDG_CACHE_HOME", cache.to_str().unwrap()),
        ],
    );
    assert!(
        !run.stderr().contains("panicked"),
        "{}: {}",
        case.id,
        run.stderr()
    );
    Output {
        exit: run.code().unwrap(),
        stdout: run.stdout().replace(&root, FIXTURE_ROOT),
        stderr: run.stderr().replace(&root, FIXTURE_ROOT),
    }
}

fn changes(id: &str) -> Vec<Change> {
    ALLOWED
        .iter()
        .filter(|a| a.cases.contains(&id))
        .map(|a| a.change)
        .collect()
}

/// Compares one case. Returns the problems found.
fn compare(case: &Case, out: &Output) -> Vec<String> {
    let changes = changes(&case.id);
    let has = |f: &dyn Fn(&Change) -> bool| changes.iter().any(f);
    let mut problems = Vec::new();
    let mut fail = |what: String| problems.push(format!("{}: {what}", case.id));

    let exit = changes
        .iter()
        .find_map(|c| match c {
            Change::Exit(code) => Some(*code),
            _ => None,
        })
        .unwrap_or(if has(&|c| matches!(c, Change::ClapUsage)) {
            2
        } else {
            case.exit
        });
    if out.exit != exit {
        fail(format!("exit {} where {exit} is expected", out.exit));
    }

    let stderr: String = out
        .stderr
        .split_inclusive('\n')
        .filter(|l| !spela_only_stderr(l))
        .collect();

    if has(&|c| matches!(c, Change::ClapUsage)) {
        if !out.stderr.contains("error:") || !out.stdout.is_empty() {
            fail(format!("not a clap usage error: {}", out.stderr));
        }
        return problems;
    }
    if let Some(code) = changes.iter().find_map(|c| match c {
        Change::JsonError(code) => Some(*code),
        _ => None,
    }) {
        let v: Value = serde_json::from_str(&out.stdout).unwrap_or_default();
        if v["schema_version"] != 1 || v["error"]["code"] != code {
            fail(format!("not the {code} error object: {}", out.stdout));
        }
        let message = go_error(&case.stderr);
        let message = message.trim_end().strip_prefix("error: ").unwrap_or("");
        if !case.id.ends_with("broken-yaml") && v["error"]["message"] != message {
            fail(format!(
                "message {} where {message:?}",
                v["error"]["message"]
            ));
        }
        if !stderr.is_empty() {
            fail(format!("stderr is not empty: {stderr}"));
        }
        return problems;
    }

    let expected_stderr = if has(&|c| matches!(c, Change::JsonInsteadOfError)) {
        String::new()
    } else {
        changes
            .iter()
            .find_map(|c| match c {
                Change::Error(line) => Some(format!("{line}\n")),
                _ => None,
            })
            .unwrap_or_else(|| go_error(&case.stderr))
    };
    if stderr != expected_stderr {
        fail(format!(
            "stderr\n--- spela\n{stderr}--- expected\n{expected_stderr}"
        ));
    }

    if has(&|c| matches!(c, Change::Json | Change::JsonInsteadOfError)) {
        problems.extend(json_facts(case, &out.stdout));
        return problems;
    }
    if has(&|c| matches!(c, Change::Toml)) {
        problems.extend(toml_facts(case, &out.stdout));
        return problems;
    }

    let mut expected = drop_retired_rows(&case.stdout);
    for change in &changes {
        if let Change::Line { go, spela } = change {
            let go = format!("{go}\n");
            if !expected.contains(&go) {
                problems.push(format!(
                    "{}: the allowed line {go:?} is not in Go's output",
                    case.id
                ));
            }
            expected = expected.replace(&go, &format!("{spela}\n"));
        }
    }
    if has(&|c| matches!(c, Change::SortedBlocks)) {
        let (mut ours, mut theirs) = (blocks(&out.stdout), blocks(&expected));
        let headers: Vec<&String> = ours.iter().map(|b| &b[0]).collect();
        if !headers.is_sorted() {
            problems.push(format!(
                "{}: games are not sorted by name: {headers:?}",
                case.id
            ));
        }
        ours.sort();
        theirs.sort();
        if ours != theirs {
            problems.push(format!(
                "{}: stdout\n--- spela\n{}--- expected, in any order\n{expected}",
                case.id, out.stdout
            ));
        }
        return problems;
    }
    if out.stdout != expected {
        problems.push(format!(
            "{}: stdout\n--- spela\n{}--- expected\n{expected}",
            case.id, out.stdout
        ));
    }
    problems
}

/// Text split into game blocks: a header line and its indented lines.
fn blocks(text: &str) -> Vec<Vec<String>> {
    let mut out: Vec<Vec<String>> = Vec::new();
    for line in text.lines() {
        if line.starts_with(' ') {
            if let Some(last) = out.last_mut() {
                last.push(line.to_owned());
            }
        } else {
            out.push(vec![line.to_owned()]);
        }
    }
    out
}

/// Go's JSON names for the kept profile fields.
const GO_FIELDS: &[(&str, &str, &str)] = &[
    ("proton.enable_hdr", "Proton", "EnableHDR"),
    ("proton.enable_wayland", "Proton", "EnableWayland"),
    ("proton.enable_ngx_updater", "Proton", "EnableNGXUpdater"),
    ("proton.vkd3d_heap", "Proton", "VKD3DHeap"),
    ("dlss.sr_mode", "DLSS", "SRMode"),
    ("dlss.sr_preset", "DLSS", "SRPreset"),
    ("dlss.sr_override", "DLSS", "SROverride"),
    ("dlss.rr_mode", "DLSS", "RRMode"),
    ("dlss.rr_preset", "DLSS", "RRPreset"),
    ("dlss.rr_override", "DLSS", "RROverride"),
    ("dlss.fg_enabled", "DLSS", "FGEnabled"),
    ("dlss.fg_override", "DLSS", "FGOverride"),
    ("dlss.multi_frame", "DLSS", "MultiFrame"),
    ("dlss.indicator", "DLSS", "Indicator"),
    ("dlss.fg_indicator", "DLSS", "FGIndicator"),
    ("gpu.clock_offset", "GPU", "ClockOffset"),
    ("gpu.vrr", "GPU", "VRR"),
    ("gpu.memory_offset", "GPU", "MemoryOffset"),
    ("gpu.power_limit", "GPU", "PowerLimit"),
    ("gpu.fan_speed", "GPU", "FanSpeed"),
    ("gpu.shader_cache", "GPU", "ShaderCache"),
    ("gpu.shader_cache_path", "GPU", "ShaderCachePath"),
    ("gpu.threaded_optimization", "GPU", "ThreadedOptimization"),
    ("cpu.governor", "CPU", "Governor"),
    ("cpu.smt", "CPU", "SMT"),
];

/// The offsets decision 31 does not carry over: their own values are gone.
const NOT_CARRIED: &[&str] = &["gpu.clock_offset", "gpu.memory_offset"];

/// Go's fields that decision 10 drops, which spela must not print.
const RETIRED_GO_FIELDS: &[&str] = &["PowerMizer", "Affinity"];

fn json_facts(case: &Case, stdout: &str) -> Vec<String> {
    let mut problems = Vec::new();
    let ours: Value = match serde_json::from_str(stdout) {
        Ok(v) => v,
        Err(e) => return vec![format!("{}: not JSON ({e}): {stdout}", case.id)],
    };
    let mut check = |ok: bool, what: String| {
        if !ok {
            problems.push(format!("{}: {what}", case.id));
        }
    };
    check(
        stdout.starts_with("{\n  \"schema_version\": 1,"),
        "schema_version is not the first key".into(),
    );
    let theirs: Value = serde_json::from_str(&case.stdout).unwrap_or(Value::Null);
    let command = case.args.iter().find(|a| !a.starts_with('-')).unwrap();
    match command.as_str() {
        "list" => {
            let ours = ours["games"].as_array().unwrap();
            let theirs = theirs.as_array().unwrap();
            check(
                ours.len() == theirs.len(),
                format!("{} games where {}", ours.len(), theirs.len()),
            );
            for (o, t) in ours.iter().zip(theirs) {
                check_game(o, t, &mut check);
            }
        }
        "show" => check_game(&ours["game"], &theirs, &mut check),
        "profile" if theirs.is_null() => {
            // Go failed: there is no profile file. spela resolves anyway.
            check(
                ours["has_profile"] == false,
                format!("has_profile {}", ours["has_profile"]),
            );
            check(
                ours["fields"].as_array().unwrap().len() == 25,
                "not 25 fields".into(),
            );
            let all_follow = ours["fields"]
                .as_array()
                .unwrap()
                .iter()
                .all(|f| f["own_value"].is_null());
            check(
                all_follow,
                "a game without a profile has an own value".into(),
            );
        }
        "profile" => {
            // Go printed the raw stored profile: an own value is a field its
            // overrides mark true.
            check(
                ours["name"] == theirs["Name"],
                format!("name {}", ours["name"]),
            );
            check(ours["has_profile"] == true, "has_profile".into());
            for (key, section, name) in GO_FIELDS {
                let field = find_field(&ours, key);
                let marked = theirs["Overrides"][*key] == true;
                let expected = if marked && !NOT_CARRIED.contains(key) {
                    theirs[*section][*name].clone()
                } else {
                    Value::Null
                };
                check(
                    field["own_value"] == expected,
                    format!("{key} own_value {} where {expected}", field["own_value"]),
                );
            }
        }
        _ => {
            // A section show: Go printed the resolved section.
            let fields = ours["fields"].as_array().unwrap();
            let go = theirs.as_object().unwrap();
            let kept = go
                .keys()
                .filter(|k| !RETIRED_GO_FIELDS.contains(&k.as_str()))
                .count();
            check(
                fields.len() == kept,
                format!("{} fields where {kept}", fields.len()),
            );
            for (name, value) in go {
                if RETIRED_GO_FIELDS.contains(&name.as_str()) {
                    continue;
                }
                let Some((key, _, _)) = GO_FIELDS.iter().find(|(_, _, n)| n == name) else {
                    check(false, format!("Go field {name} has no key"));
                    continue;
                };
                let expected = if NOT_CARRIED.contains(key) {
                    Value::from(0)
                } else {
                    value.clone()
                };
                let field = find_field(&ours, key);
                check(
                    field["value"] == expected,
                    format!("{key} value {} where {expected}", field["value"]),
                );
            }
        }
    }
    problems
}

fn find_field<'a>(profile: &'a Value, key: &str) -> &'a Value {
    profile["fields"]
        .as_array()
        .unwrap()
        .iter()
        .find(|f| f["key"] == key)
        .unwrap_or(&Value::Null)
}

/// One game: Go's object against spela's.
fn check_game(ours: &Value, theirs: &Value, check: &mut dyn FnMut(bool, String)) {
    let same = |a: &Value, b: &Value| {
        // Go writes an empty string where spela writes null.
        a == b || (a.is_null() && b == "")
    };
    for (o, t) in [
        ("app_id", "AppID"),
        ("name", "Name"),
        ("install_dir", "InstallDir"),
        ("library_path", "LibraryPath"),
        ("prefix_path", "PrefixPath"),
    ] {
        check(
            same(&ours[o], &theirs[t]),
            format!("{o} {} where {}", ours[o], theirs[t]),
        );
    }
    let go_time: jiff::Timestamp = theirs["ScannedAt"].as_str().unwrap().parse().unwrap();
    check(
        ours["scanned_at"] == go_time.to_string(),
        format!("scanned_at {} where {go_time}", ours["scanned_at"]),
    );
    let dll = |d: &Value, keys: [&str; 4]| -> Vec<String> {
        keys.iter()
            .map(|k| match &d[*k] {
                Value::String(s) => s.clone(),
                _ => String::new(),
            })
            .collect()
    };
    let mut ours_dlls: Vec<Vec<String>> = ours["dlls"]
        .as_array()
        .unwrap()
        .iter()
        .map(|d| dll(d, ["path", "name", "type", "version"]))
        .collect();
    let mut theirs_dlls: Vec<Vec<String>> = theirs["DLLs"]
        .as_array()
        .map(|a| {
            a.iter()
                .map(|d| dll(d, ["Path", "Name", "Type", "Version"]))
                .collect()
        })
        .unwrap_or_default();
    let types: Vec<&String> = ours_dlls.iter().map(|d| &d[2]).collect();
    let order = ["dlss", "dlssg", "dlssd", "xess", "fsr"];
    let ranks: Vec<usize> = types
        .iter()
        .map(|t| order.iter().position(|o| o == t).unwrap())
        .collect();
    check(
        ranks.is_sorted(),
        format!("DLLs not in catalogue order: {types:?}"),
    );
    ours_dlls.sort();
    theirs_dlls.sort();
    check(
        ours_dlls == theirs_dlls,
        format!("DLLs {ours_dlls:?} where {theirs_dlls:?}"),
    );
}

fn toml_facts(case: &Case, stdout: &str) -> Vec<String> {
    let mut problems = Vec::new();
    let ours: toml::Table = match stdout.parse() {
        Ok(t) => t,
        Err(e) => return vec![format!("{}: not TOML ({e}): {stdout}", case.id)],
    };
    let go: BTreeMap<&str, &str> = case
        .stdout
        .lines()
        .filter_map(|l| l.split_once(": "))
        .collect();
    for (key, value) in &go {
        if ["theme", "compact_mode"].contains(key) {
            if ours.contains_key(*key) {
                problems.push(format!("{}: {key} is still printed", case.id));
            }
            continue;
        }
        let shown = match ours.get(*key) {
            Some(toml::Value::String(s)) => s.clone(),
            Some(other) => other.to_string(),
            None => {
                problems.push(format!("{}: {key} is missing", case.id));
                continue;
            }
        };
        if shown != *value {
            problems.push(format!(
                "{}: {key} = {shown} where Go says {value}",
                case.id
            ));
        }
    }
    for key in ["version", "steam_path", "additional_library_paths", "ui"] {
        if !ours.contains_key(key) {
            problems.push(format!("{}: {key} is missing", case.id));
        }
    }
    problems
}

#[test]
fn read_commands_match_the_go_transcripts() {
    let mut compared = 0;
    let mut problems = Vec::new();
    for (case, skip) in cases() {
        if skip.is_some() {
            continue;
        }
        compared += 1;
        let out = run_spela(&case);
        problems.extend(compare(&case, &out));
    }
    assert!(
        problems.is_empty(),
        "{} of {compared} commands differ:\n\n{}",
        problems.len(),
        problems.join("\n\n")
    );
    assert!(compared >= 70, "only {compared} transcripts compared");
}

#[test]
fn every_allowed_difference_names_a_compared_transcript() {
    let ids: Vec<String> = cases()
        .into_iter()
        .filter(|(_, skip)| skip.is_none())
        .map(|(c, _)| c.id)
        .collect();
    for allowed in ALLOWED {
        assert!(!allowed.why.is_empty());
        for case in allowed.cases {
            assert!(
                ids.iter().any(|id| id == case),
                "{case} is not a compared transcript"
            );
        }
    }
}

#[test]
fn every_transcript_is_compared_or_skipped_with_a_reason() {
    let all = cases();
    assert_eq!(
        all.len(),
        161,
        "index.json changed: look at the new entries"
    );
    for (case, skip) in &all {
        if let Some(why) = skip {
            assert!(!why.is_empty(), "{}", case.id);
        }
    }
}
