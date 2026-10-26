//! Keeps spela's DLL manifest current. Runs in CI, not on users' machines.
//!
//! The port of Go's `tools/dlss-updater` (decision 14), with its flags,
//! output and exit codes:
//!
//! ```text
//! dlss-updater [-type dlss] [-manifest data/manifest.json] [-json]
//! ```
//!
//! scrapes the family's TechPowerUp page ([`scrape`], [`fetch`]) and says
//! whether its version is new, in text or as one JSON line with `type`,
//! `version`, `download_url`, `filename`, `is_new` and `source`. It exits 0
//! when the version is new, 2 when the manifest is up to date and 1 on an
//! error. A flag error exits 2 and `-h` exits 0, as Go's `flag` does.
//!
//! One change from Go (decision 8): new means newer by the shared version
//! compare than the highest valid version the manifest lists, the entry
//! spela itself picks. Go called any other string new, so an older listing
//! would have been added as the first entry, which Go clients take as the
//! latest.
//!
//! ```text
//! dlss-updater add -type dlss -version 310.6.0 -file nvngx_dlss.dll -url URL [-notes TEXT] [-manifest PATH]
//! ```
//!
//! adds a release to the manifest ([`add`]), the step the workflows did in
//! inline Python.

pub mod add;
pub mod fetch;
pub mod flags;
pub mod scrape;

use std::io::Write;
use std::path::{Path, PathBuf};

use jiff::Timestamp;
use serde::Serialize;
use spela_core::dll::manifest::Manifest;
use spela_core::dll::version;
use spela_core::games::DllType;

use flags::{Flag, FlagError, Kind, Parsed};
use scrape::{FetchError, Source};

/// The manifest every Go client fetches, relative to the repository root.
pub const DEFAULT_MANIFEST: &str = "data/manifest.json";

/// Exit codes.
pub mod exit {
    /// A new version, or a command that worked.
    pub const NEW: u8 = 0;
    /// An error.
    pub const FAILURE: u8 = 1;
    /// The manifest is up to date.
    pub const UP_TO_DATE: u8 = 2;
    /// A flag error, as Go's `flag` exits.
    pub const USAGE: u8 = 2;
}

/// The newest version a source lists, the JSON line the workflow reads.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct Latest {
    /// The family word, such as `dlssg`.
    #[serde(rename = "type")]
    pub dll_type: String,
    /// The version, such as `310.5.3`.
    pub version: String,
    /// Where the zip with the DLL is downloaded.
    pub download_url: String,
    /// The DLL's file name, such as `nvngx_dlssg.dll`.
    pub filename: String,
    /// Whether the version is newer than the manifest's latest.
    pub is_new: bool,
    /// Where it was found: `techpowerup.com`.
    pub source: String,
}

const MANIFEST_FLAG: Flag = Flag {
    name: "manifest",
    kind: Kind::Text,
    usage: "Path to manifest.json",
    default: DEFAULT_MANIFEST,
};

const CHECK_FLAGS: &[Flag] = &[
    MANIFEST_FLAG,
    Flag {
        name: "type",
        kind: Kind::Text,
        usage: "DLL type to check (dlss, dlssg, dlssd)",
        default: "dlss",
    },
    Flag {
        name: "json",
        kind: Kind::Bool,
        usage: "Output as JSON",
        default: "false",
    },
];

const ADD_FLAGS: &[Flag] = &[
    MANIFEST_FLAG,
    Flag {
        name: "type",
        kind: Kind::Text,
        usage: "DLL type to add (dlss, dlssg, dlssd, xess, fsr)",
        default: "",
    },
    Flag {
        name: "version",
        kind: Kind::Text,
        usage: "Version of the DLL, such as 310.5.3",
        default: "",
    },
    Flag {
        name: "file",
        kind: Kind::Text,
        usage: "Path to the DLL, for its SHA-256 and size",
        default: "",
    },
    Flag {
        name: "url",
        kind: Kind::Text,
        usage: "https URL clients download the DLL from, ending in its file name",
        default: "",
    },
    Flag {
        name: "notes",
        kind: Kind::Text,
        usage: "Notes for the entry, such as \"From techpowerup.com\"",
        default: "",
    },
];

/// Runs the updater with `args`, the words after the program name.
/// `fetch` finds a source's newest version, and `now` stamps a release that
/// `add` writes. Returns the exit code.
pub fn run(
    program: &str,
    args: &[String],
    out: &mut dyn Write,
    err: &mut dyn Write,
    fetch: &dyn Fn(&Source) -> Result<Latest, FetchError>,
    now: Timestamp,
) -> u8 {
    if args.first().is_some_and(|a| a == "add") {
        let program = format!("{program} add");
        return match parse(&program, ADD_FLAGS, &args[1..], err) {
            Ok(parsed) => add_command(&program, &parsed, out, err, now),
            Err(code) => code,
        };
    }
    match parse(program, CHECK_FLAGS, args, err) {
        Ok(parsed) => check(&parsed, out, err, fetch),
        Err(code) => code,
    }
}

/// Parses the flags, printing help or a flag error. Words after the flags
/// are an error: Go ignored them, and a mistyped command should not check.
fn parse(
    program: &str,
    flags: &[Flag],
    args: &[String],
    err: &mut dyn Write,
) -> Result<Parsed, u8> {
    let usage = flags::usage(program, flags);
    let problem = match flags::parse(flags, args) {
        Ok(parsed) => match parsed.rest.first() {
            None => return Ok(parsed),
            Some(word) => format!("unexpected argument: {word}"),
        },
        Err(FlagError::Help) => {
            let _ = write!(err, "{usage}");
            return Err(exit::NEW);
        }
        Err(FlagError::Usage(problem)) => problem,
    };
    let _ = write!(err, "{problem}\n{usage}");
    Err(exit::USAGE)
}

fn check(
    parsed: &Parsed,
    out: &mut dyn Write,
    err: &mut dyn Write,
    fetch: &dyn Fn(&Source) -> Result<Latest, FetchError>,
) -> u8 {
    let name = parsed.text("type");
    let Some(source) = scrape::find_source(name) else {
        let _ = writeln!(err, "Unknown DLL type: {name}");
        return exit::FAILURE;
    };
    let mut latest = match fetch(&source) {
        Ok(latest) => latest,
        Err(e) => {
            let _ = writeln!(err, "Failed to fetch latest version: {e}");
            return exit::FAILURE;
        }
    };
    let current = match current_version(Path::new(parsed.text("manifest")), source.dll_type) {
        Ok(current) => current,
        Err(e) => {
            let _ = writeln!(err, "Warning: could not read manifest: {e}");
            None
        }
    };
    latest.is_new = current
        .as_deref()
        .is_none_or(|current| version::is_newer(current, &latest.version));

    let label = name.to_uppercase();
    let written = if parsed.on("json") {
        serde_json::to_string(&latest)
            .map_err(std::io::Error::other)
            .and_then(|line| writeln!(out, "{line}"))
    } else if latest.is_new {
        let mut text = format!(
            "New {label} version available: {}\nDownload URL: {}\nFilename: {}\nSource: {}\n",
            latest.version, latest.download_url, latest.filename, latest.source
        );
        if let Some(current) = &current {
            text.push_str(&format!("Current version: {current}\n"));
        }
        write!(out, "{text}")
    } else {
        match current.as_deref() {
            Some(current) if version::compare(current, &latest.version).is_gt() => writeln!(
                out,
                "{label} is up to date: the manifest has {current}, newer than the listed {}",
                latest.version
            ),
            _ => writeln!(out, "{label} is up to date: {}", latest.version),
        }
    };
    if let Err(e) = written {
        let _ = writeln!(err, "Failed to write the result: {e}");
        return exit::FAILURE;
    }
    if latest.is_new {
        exit::NEW
    } else {
        exit::UP_TO_DATE
    }
}

/// The highest valid version of a family in the manifest file, `None` when
/// it lists none. One leading `v` is dropped first, as Go's updater and the
/// shared compare do, so a `v310.5.3` entry still counts as 310.5.3.
pub fn current_version(path: &Path, dll_type: DllType) -> Result<Option<String>, String> {
    let text = std::fs::read_to_string(path)
        .map_err(|e| format!("cannot read {}: {e}", path.display()))?;
    let manifest = Manifest::from_json(&text)
        .map_err(|e| format!("{} does not parse: {e}", path.display()))?;
    Ok(manifest
        .entries(dll_type)
        .iter()
        .map(|e| e.version.strip_prefix('v').unwrap_or(&e.version))
        .filter(|v| version::is_valid(v))
        .reduce(|best, v| {
            if version::compare(v, best).is_gt() {
                v
            } else {
                best
            }
        })
        .map(str::to_owned))
}

fn add_command(
    program: &str,
    parsed: &Parsed,
    out: &mut dyn Write,
    err: &mut dyn Write,
    now: Timestamp,
) -> u8 {
    for required in ["type", "version", "file", "url"] {
        if parsed.text(required).is_empty() {
            let usage = flags::usage(program, ADD_FLAGS);
            let _ = write!(err, "missing -{required}\n{usage}");
            return exit::USAGE;
        }
    }
    let name = parsed.text("type");
    let Some(dll_type) = DllType::from_name(name) else {
        let _ = writeln!(err, "Unknown DLL type: {name}");
        return exit::FAILURE;
    };
    let release = add::Release {
        dll_type,
        version: parsed.text("version").to_owned(),
        file: PathBuf::from(parsed.text("file")),
        url: parsed.text("url").to_owned(),
        notes: parsed.text("notes").to_owned(),
    };
    let manifest = Path::new(parsed.text("manifest"));
    let added = match add::add(manifest, &release, now) {
        Ok(added) => added,
        Err(e) => {
            let _ = writeln!(err, "Failed to add the release: {e}");
            return exit::FAILURE;
        }
    };
    let label = name.to_uppercase();
    let verb = if added.replaced { "Replaced" } else { "Added" };
    let mut line = format!(
        "{verb} {label} {} in {}.",
        release.version,
        manifest.display()
    );
    if version::compare(&added.latest, &release.version).is_ne() {
        line.push_str(&format!(" The latest {label} is still {}.", added.latest));
    }
    let _ = writeln!(out, "{line}");
    exit::NEW
}

#[cfg(test)]
mod tests {
    use super::*;

    struct Run {
        code: u8,
        out: String,
        err: String,
    }

    fn run_with(args: &[&str], fetch: &dyn Fn(&Source) -> Result<Latest, FetchError>) -> Run {
        let args: Vec<String> = args.iter().map(|a| (*a).to_owned()).collect();
        let (mut out, mut err) = (Vec::new(), Vec::new());
        let code = run(
            "dlss-updater",
            &args,
            &mut out,
            &mut err,
            fetch,
            "2026-10-26T12:00:00Z".parse().unwrap(),
        );
        Run {
            code,
            out: String::from_utf8(out).unwrap(),
            err: String::from_utf8(err).unwrap(),
        }
    }

    fn listing(version: &'static str) -> impl Fn(&Source) -> Result<Latest, FetchError> {
        move |source| {
            Ok(Latest {
                dll_type: source.dll_type.as_str().into(),
                version: version.into(),
                download_url: "https://download.example.test/dlss.zip?a=1&b=2".into(),
                filename: source.filename().into(),
                is_new: false,
                source: "fixture".into(),
            })
        }
    }

    fn no_fetch(_: &Source) -> Result<Latest, FetchError> {
        panic!("this run must not fetch")
    }

    fn manifest(text: &str) -> (tempfile::TempDir, String) {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("manifest.json");
        std::fs::write(&path, text).unwrap();
        (dir, path.display().to_string())
    }

    const USAGE: &str = "Usage of dlss-updater:\n  -json\n    \tOutput as JSON\n  -manifest string\n    \tPath to manifest.json (default \"data/manifest.json\")\n  -type string\n    \tDLL type to check (dlss, dlssg, dlssd) (default \"dlss\")\n";

    #[test]
    fn help_and_flag_errors_keep_go_s_text_and_codes() {
        // tools/dlss-updater/main_test.go, TestRunPreservesFlagHelpAndParseExitSemantics.
        for help in ["-h", "--help"] {
            let r = run_with(&[help], &no_fetch);
            assert_eq!((r.code, r.out.as_str(), r.err.as_str()), (0, "", USAGE));
        }
        let r = run_with(&["--invalid"], &no_fetch);
        assert_eq!(r.code, 2);
        assert_eq!(r.out, "");
        assert_eq!(
            r.err,
            format!("flag provided but not defined: -invalid\n{USAGE}")
        );
        let r = run_with(&["-json", "now"], &no_fetch);
        assert_eq!(r.code, 2);
        assert!(
            r.err
                .starts_with("unexpected argument: now\nUsage of dlss-updater:"),
            "{}",
            r.err
        );
    }

    #[test]
    fn new_current_unknown_and_failing_runs() {
        // tools/dlss-updater/main_test.go, TestRunCommandOutputExitAndFailureContracts.
        let (_dir, path) = manifest(r#"{"dlls":{"dlss":[{"version":"3.8.10"}]}}"#);
        let r = run_with(&["--manifest", &path, "--type", "dlss"], &listing("3.9.0"));
        assert_eq!(r.code, 0, "{}", r.err);
        assert_eq!(r.err, "");
        assert_eq!(
            r.out,
            "New DLSS version available: 3.9.0\nDownload URL: https://download.example.test/dlss.zip?a=1&b=2\nFilename: nvngx_dlss.dll\nSource: fixture\nCurrent version: 3.8.10\n"
        );

        let r = run_with(&["--manifest", &path, "--json"], &listing("3.8.10"));
        assert_eq!(r.code, 2);
        assert_eq!(
            r.out,
            "{\"type\":\"dlss\",\"version\":\"3.8.10\",\"download_url\":\"https://download.example.test/dlss.zip?a=1&b=2\",\"filename\":\"nvngx_dlss.dll\",\"is_new\":false,\"source\":\"fixture\"}\n"
        );

        let r = run_with(&["-manifest", &path], &listing("3.8.10"));
        assert_eq!(
            (r.code, r.out.as_str()),
            (2, "DLSS is up to date: 3.8.10\n")
        );

        let r = run_with(&["--type", "unknown"], &no_fetch);
        assert_eq!((r.code, r.err.as_str()), (1, "Unknown DLL type: unknown\n"));
        let r = run_with(&["--type", "xess"], &no_fetch);
        assert_eq!(r.code, 1, "XeSS has no source");

        let offline = |_: &Source| Err(FetchError::Network("offline".into()));
        let r = run_with(&[], &offline);
        assert_eq!(
            (r.code, r.err.as_str()),
            (1, "Failed to fetch latest version: offline\n")
        );
    }

    #[test]
    fn a_v_prefixed_manifest_version_counts_as_its_number() {
        // Go's getCurrentVersion trims one leading `v`.
        let (_dir, path) = manifest(r#"{"dlls":{"dlss":[{"version":"v310.5.3"}]}}"#);
        let r = run_with(&["-manifest", &path], &listing("310.5.3"));
        assert_eq!(
            (r.code, r.out.as_str()),
            (2, "DLSS is up to date: 310.5.3\n"),
            "{}",
            r.err
        );
    }

    #[test]
    fn an_older_listing_is_never_new() {
        let (_dir, path) = manifest(
            r#"{"dlls":{"dlssg":[{"version":"310.4.0"},{"version":"310.5.3"},{"version":"999-beta"}]}}"#,
        );
        let r = run_with(&["-type", "dlssg", "-manifest", &path], &listing("310.5.0"));
        assert_eq!(r.code, 2, "lower than the highest, though not the first");
        assert_eq!(
            r.out,
            "DLSSG is up to date: the manifest has 310.5.3, newer than the listed 310.5.0\n"
        );
        let r = run_with(
            &["-type", "dlssg", "-manifest", &path, "-json"],
            &listing("310.5.3.0"),
        );
        assert_eq!(r.code, 2, "an equal version with a fourth part");
        assert!(r.out.contains("\"is_new\":false"));
        let r = run_with(
            &["-type", "dlssg", "-manifest", &path],
            &listing("310.10.0"),
        );
        assert_eq!(r.code, 0, "numbers, not strings");
        assert!(r.out.contains("Current version: 310.5.3\n"), "{}", r.out);
    }

    #[test]
    fn an_unreadable_manifest_warns_and_counts_as_new() {
        let dir = tempfile::tempdir().unwrap();
        let missing = dir.path().join("none.json").display().to_string();
        let r = run_with(&["-manifest", &missing, "-json"], &listing("1.0.0"));
        assert_eq!(r.code, 0);
        assert!(
            r.err
                .starts_with("Warning: could not read manifest: cannot read "),
            "{}",
            r.err
        );
        assert!(r.out.contains("\"is_new\":true"));

        let (_dir, broken) = manifest("{");
        let r = run_with(&["-manifest", &broken], &listing("1.0.0"));
        assert_eq!(r.code, 0);
        assert!(r.err.contains("does not parse"), "{}", r.err);
        assert!(!r.out.contains("Current version"), "{}", r.out);

        let (_dir, empty) = manifest(r#"{"dlls":{"dlss":[]}}"#);
        let r = run_with(&["-manifest", &empty], &listing("1.0.0"));
        assert_eq!((r.code, r.err.as_str()), (0, ""));
    }

    #[test]
    fn add_writes_the_entry_and_says_where_it_went() {
        let (dir, path) = manifest(r#"{"version":"1","dlls":{"dlss":[{"version":"310.5.3"}]}}"#);
        let dll = dir.path().join("nvngx_dlss.dll");
        std::fs::write(&dll, b"dll").unwrap();
        let dll = dll.display().to_string();
        let url =
            "https://github.com/jgabor/spela/releases/download/dll-dlss-v310.6.0/nvngx_dlss.dll";
        let args = |version: &'static str| {
            vec![
                "add",
                "-type",
                "dlss",
                "-version",
                version,
                "-file",
                &dll,
                "-url",
                url,
                "-manifest",
                &path,
                "-notes",
                "From techpowerup.com",
            ]
            .into_iter()
            .map(str::to_owned)
            .collect::<Vec<String>>()
        };
        let run_args = |a: Vec<String>| {
            let a: Vec<&str> = a.iter().map(String::as_str).collect();
            run_with(&a, &no_fetch)
        };
        let r = run_args(args("310.6.0"));
        assert_eq!((r.code, r.err.as_str()), (0, ""));
        assert_eq!(r.out, format!("Added DLSS 310.6.0 in {path}.\n"));
        let r = run_args(args("310.6.0"));
        assert_eq!(r.out, format!("Replaced DLSS 310.6.0 in {path}.\n"));
        let r = run_args(args("310.1.0"));
        assert_eq!(
            r.out,
            format!("Added DLSS 310.1.0 in {path}. The latest DLSS is still 310.6.0.\n")
        );
        assert_eq!(
            current_version(Path::new(&path), DllType::Dlss)
                .unwrap()
                .as_deref(),
            Some("310.6.0")
        );

        let r = run_with(&["add", "-type", "dlss"], &no_fetch);
        assert_eq!(r.code, 2);
        assert!(
            r.err
                .starts_with("missing -version\nUsage of dlss-updater add:\n"),
            "{}",
            r.err
        );
        let r = run_with(&["add", "-h"], &no_fetch);
        assert_eq!(r.code, 0);
        assert!(r.err.contains("  -url string\n"), "{}", r.err);
        let r = run_with(
            &[
                "add", "-type", "nope", "-version", "1", "-file", "x", "-url", "y",
            ],
            &no_fetch,
        );
        assert_eq!((r.code, r.err.as_str()), (1, "Unknown DLL type: nope\n"));
        let r = run_with(
            &[
                "add",
                "-type",
                "dlss",
                "-version",
                "1",
                "-file",
                "/nonexistent/x.dll",
                "-url",
                url,
            ],
            &no_fetch,
        );
        assert_eq!(r.code, 1);
        assert!(
            r.err.starts_with("Failed to add the release: "),
            "{}",
            r.err
        );
    }
}
