//! The release contract: the intents of Go's `tests/contracts`, ported for
//! the Rust release (docs/architecture.md, CI and Phase 7).
//!
//! - A `vX.Y.Z` tag is validated before anything uses it, and tag text never
//!   reaches a shell script except through env.
//! - The release builds one Linux x86_64 binary whose Cargo version is the
//!   tag, newer than v0.6.2, and publishes it as `spela-linux-amd64` with
//!   `checksums.txt` and the CHANGELOG section as its notes.
//! - The DLL workflows run the Rust updater and write real DLL file names.
//! - The AUR package and `just install` put the binary at /usr/bin/spela and
//!   the polkit policy beside it.
//!
//! Nothing here publishes, tags or reaches the network.

mod common;

use std::path::Path;
use std::process::{Command, Output};

use common::{clean_command, has_program, read, root};
use regex_lite::Regex;
use serde_json::Value;
use spela_core::dll::detect::CATALOGUE;

/// The workflow files.
fn workflow_names() -> Vec<String> {
    let mut names: Vec<String> = std::fs::read_dir(root().join(".github/workflows"))
        .unwrap()
        .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
        .filter(|n| n.ends_with(".yml") || n.ends_with(".yaml"))
        .collect();
    names.sort();
    assert!(names.len() >= 5, "{names:?}");
    names
}

fn workflow(name: &str) -> Value {
    let text = read(&format!(".github/workflows/{name}"));
    serde_saphyr::from_str(&text).unwrap_or_else(|e| panic!("{name}: {e}"))
}

fn steps<'a>(workflow: &'a Value, job: &str) -> &'a [Value] {
    workflow["jobs"][job]["steps"]
        .as_array()
        .unwrap_or_else(|| panic!("job {job} has no steps"))
}

fn step<'a>(workflow: &'a Value, job: &str, name: &str) -> &'a Value {
    steps(workflow, job)
        .iter()
        .find(|s| s["name"] == name)
        .unwrap_or_else(|| panic!("job {job} has no step {name:?}"))
}

fn run_of<'a>(workflow: &'a Value, job: &str, name: &str) -> &'a str {
    step(workflow, job, name)["run"].as_str().unwrap()
}

fn lines(text: &str) -> Vec<&str> {
    text.lines()
        .map(str::trim)
        .filter(|l| !l.is_empty())
        .collect()
}

/// Every `run` script of every job in a workflow, with its job and step.
fn scripts(workflow: &Value) -> Vec<(String, String, String)> {
    let mut out = Vec::new();
    for (job, body) in workflow["jobs"].as_object().unwrap() {
        for step in body["steps"]
            .as_array()
            .map(Vec::as_slice)
            .unwrap_or_default()
        {
            if let Some(run) = step["run"].as_str() {
                let name = step["name"].as_str().unwrap_or("(unnamed)");
                out.push((job.clone(), name.to_owned(), run.to_owned()));
            }
        }
    }
    out
}

/// Runs a workflow script with bash -e, as GitHub does, in `dir`.
fn bash(script: &str, dir: &Path, home: &Path, env: &[(&str, &str)]) -> Output {
    let mut command = clean_command("bash", home, None);
    command.args(["-e", "-c", script]).current_dir(dir);
    for (k, v) in env {
        command.env(k, v);
    }
    command.output().unwrap()
}

#[test]
fn every_workflow_is_a_mapping_with_jobs() {
    for name in workflow_names() {
        let w = workflow(&name);
        assert!(w.is_object(), "{name}");
        assert!(w["jobs"].is_object(), "{name} has no jobs");
        assert!(w.get("on").is_some(), "{name} has no trigger");
    }
}

#[test]
fn no_script_interpolates_an_expression() {
    // ${{ }} in a script is pasted in before bash parses it, so a tag, an
    // input or a scraped URL could run as code. Values go through env.
    for name in workflow_names() {
        for (job, step, run) in scripts(&workflow(&name)) {
            assert!(
                !run.contains("${{"),
                "{name}: {job}/{step} interpolates an expression into its script"
            );
        }
    }
}

#[test]
fn every_action_is_pinned_to_a_commit() {
    let pinned = Regex::new(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$").unwrap();
    for name in workflow_names() {
        let text = read(&format!(".github/workflows/{name}"));
        assert!(!text.contains("@latest"), "{name} uses @latest");
        let w = workflow(&name);
        for (job, body) in w["jobs"].as_object().unwrap() {
            let mut uses: Vec<&str> = body["uses"].as_str().into_iter().collect();
            for step in body["steps"]
                .as_array()
                .map(Vec::as_slice)
                .unwrap_or_default()
            {
                uses.extend(step["uses"].as_str());
            }
            for action in uses {
                assert!(
                    action.starts_with("./") || pinned.is_match(action),
                    "{name}: {job} uses {action}, not a commit"
                );
            }
        }
    }
}

#[test]
fn the_release_publishes_one_binary_with_checksums_and_notes() {
    // tests/contracts/release_contract_test.go, TestReleaseWorkflowArtifactContract.
    let w = workflow("release.yml");
    assert_eq!(w["on"], serde_json::json!({"push": {"tags": ["v*"]}}));
    assert_eq!(w["permissions"]["contents"], "write");
    let jobs: Vec<&String> = w["jobs"].as_object().unwrap().keys().collect();
    assert_eq!(jobs, ["build", "gate", "validate-tag"]);

    // The full gate runs first.
    assert_eq!(w["jobs"]["gate"]["uses"], "./.github/workflows/rust.yml");
    assert_eq!(w["jobs"]["gate"]["needs"], "validate-tag");
    assert_eq!(
        w["jobs"]["build"]["needs"],
        serde_json::json!(["validate-tag", "gate"])
    );
    let rust = workflow("rust.yml");
    assert!(
        rust["on"].get("workflow_call").is_some(),
        "rust.yml can be called"
    );
    for job in ["gate", "msrv", "deny"] {
        assert!(rust["jobs"].get(job).is_some(), "rust.yml has no {job} job");
    }

    let names: Vec<&str> = steps(&w, "build")
        .iter()
        .filter_map(|s| s["name"].as_str())
        .collect();
    assert_eq!(
        names,
        [
            "Install Rust",
            "Check the version against the tag",
            "Build the release binary",
            "Create checksums",
            "Extract release notes from CHANGELOG.md",
            "Create Release"
        ]
    );
    assert_eq!(
        lines(run_of(&w, "build", "Build the release binary")),
        [
            r#"SPELA_VERSION="${RELEASE_TAG#v}" cargo build --release --locked -p spela --target x86_64-unknown-linux-gnu"#,
            "cp target/x86_64-unknown-linux-gnu/release/spela spela-linux-amd64",
        ]
    );
    assert_eq!(
        run_of(&w, "build", "Create checksums").trim(),
        "sha256sum spela-linux-amd64 > checksums.txt"
    );
    assert_eq!(
        run_of(&w, "build", "Extract release notes from CHANGELOG.md").trim(),
        r#"sh scripts/release-notes.sh "$RELEASE_TAG" > release_notes.md"#
    );
    let release = step(&w, "build", "Create Release");
    assert!(
        release["uses"]
            .as_str()
            .unwrap()
            .starts_with("softprops/action-gh-release@")
    );
    assert_eq!(
        lines(release["with"]["files"].as_str().unwrap()),
        ["spela-linux-amd64", "checksums.txt"]
    );
    assert_eq!(release["with"]["body_path"], "release_notes.md");

    // No Go build, and no AUR publication from a versioned release: the AUR
    // package is the VCS package spela-git.
    let text = read(".github/workflows/release.yml");
    for gone in ["setup-go", "go tool", "mage", "aur"] {
        assert!(
            !text.to_lowercase().contains(gone),
            "release.yml mentions {gone}"
        );
    }
}

#[test]
fn the_tag_is_validated_before_use() {
    // tests/contracts/release_contract_test.go, TestReleaseTagIsValidatedBeforeUse.
    let w = workflow("release.yml");
    let text = read(".github/workflows/release.yml");
    assert_eq!(
        text.matches("${{ github.ref_name }}").count(),
        1,
        "github.ref_name is read only by the validation's env"
    );
    let validate = &w["jobs"]["validate-tag"];
    let validate_steps = validate["steps"].as_array().unwrap();
    assert_eq!(validate_steps.len(), 1);
    let only = &validate_steps[0];
    assert_eq!(only["id"], "validate");
    assert_eq!(only["env"]["RELEASE_TAG"], "${{ github.ref_name }}");
    assert_eq!(
        validate["outputs"]["release_tag"],
        "${{ steps.validate.outputs.release_tag }}"
    );
    assert!(
        only["run"]
            .as_str()
            .unwrap()
            .contains(r"^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$")
    );
    assert_eq!(
        w["jobs"]["build"]["env"]["RELEASE_TAG"],
        "${{ needs.validate-tag.outputs.release_tag }}"
    );
    for (job, body) in w["jobs"].as_object().unwrap() {
        if let Some(uses) = body["uses"].as_str() {
            assert!(!uses.contains("ref_name"), "{job}");
        }
        for step in body["steps"]
            .as_array()
            .map(Vec::as_slice)
            .unwrap_or_default()
        {
            for key in ["run", "uses"] {
                let value = step[key].as_str().unwrap_or_default();
                assert!(!value.contains("github.ref_name"), "{job}: {value}");
            }
        }
    }
}

#[test]
fn the_tag_validation_rejects_shell_syntax() {
    // tests/contracts/release_contract_test.go, TestReleaseTagValidationRejectsShellSyntax.
    let w = workflow("release.yml");
    let script = w["jobs"]["validate-tag"]["steps"][0]["run"]
        .as_str()
        .unwrap()
        .to_owned();
    // No dot-prefixed folder in the marker path, so the payloads stay valid
    // Git ref names.
    let temp = || {
        tempfile::Builder::new()
            .prefix("spela-tag-")
            .tempdir()
            .unwrap()
    };
    let validate = |tag: &str| {
        let state = temp();
        let output = state.path().join("github-output");
        let run = bash(
            &script,
            state.path(),
            state.path(),
            &[
                ("RELEASE_TAG", tag),
                ("GITHUB_OUTPUT", output.to_str().unwrap()),
            ],
        );
        let written = std::fs::read_to_string(&output).ok();
        (run.status.success(), written, state)
    };
    for tag in ["v1.2.3", "v0.0.0", "v0.7.0", "v10.20.30"] {
        let (ok, written, _) = validate(tag);
        assert!(ok, "{tag} rejected");
        assert_eq!(
            written.as_deref(),
            Some(format!("release_tag={tag}\n").as_str())
        );
    }
    for tag in [
        "",
        "1.2.3",
        "v01.2.3",
        "v1.2.3-rc1",
        "v1.2",
        "v1.2.3.4",
        "v1.2.3+meta",
        "V1.2.3",
        "v1.2.3\n",
    ] {
        let (ok, written, _) = validate(tag);
        assert!(!ok, "{tag:?} accepted");
        assert_eq!(written, None, "{tag:?} reached the output");
    }

    for payload in [
        "v1.2.3$(touch${IFS}MARKER)",
        r#"v1.2.3"$(touch${IFS}MARKER)""#,
        "v1.2.3`touch${IFS}MARKER`",
    ] {
        let state = temp();
        let marker = state.path().join("injected");
        let payload = payload.replace("MARKER", marker.to_str().unwrap());
        if !payload.contains('`') {
            let check = Command::new("git")
                .args(["check-ref-format", &format!("refs/tags/{payload}")])
                .output()
                .unwrap();
            assert!(check.status.success(), "{payload} is not a valid Git ref");
        }
        let output = state.path().join("github-output");
        let run = bash(
            &script,
            state.path(),
            state.path(),
            &[
                ("RELEASE_TAG", &payload),
                ("GITHUB_OUTPUT", output.to_str().unwrap()),
            ],
        );
        assert!(!run.status.success(), "{payload} accepted");
        assert!(!marker.exists(), "{payload} ran its substitution");
        assert!(!output.exists(), "{payload} reached the output");
    }
}

#[test]
fn the_cargo_version_must_be_the_tag_and_newer_than_v0_6_2() {
    if !has_program("jq") {
        eprintln!("skipped: needs jq");
        return;
    }
    let w = workflow("release.yml");
    let script = run_of(&w, "build", "Check the version against the tag").to_owned();
    let check = |dir: &Path, tag: &str| {
        let home = tempfile::tempdir().unwrap();
        // The toolchain's own cargo, so the check needs no rustup state.
        let bin = home.path().join("bin");
        std::fs::create_dir_all(&bin).unwrap();
        std::os::unix::fs::symlink(env!("CARGO"), bin.join("cargo")).unwrap();
        let mut command = clean_command("bash", home.path(), Some(&bin));
        command
            .args(["-e", "-o", "pipefail", "-c", &script])
            .current_dir(dir)
            .env("RELEASE_TAG", tag)
            .env("CARGO_HOME", home.path().join("cargo"));
        let out = command.output().unwrap();
        (
            out.status.success(),
            String::from_utf8_lossy(&out.stderr).into_owned(),
        )
    };

    // This workspace, at its own version.
    let (ok, err) = check(&root(), &format!("v{}", spela_core::VERSION));
    assert!(ok, "{err}");
    let (ok, err) = check(&root(), "v99.0.0");
    assert!(!ok);
    assert!(err.contains("does not match the Cargo version"), "{err}");

    // A crate named spela at other versions.
    for (version, tag, want) in [
        ("0.7.0", "v0.7.0", true),
        ("0.10.0", "v0.10.0", true),
        ("1.0.0", "v1.0.0", true),
        ("0.6.3", "v0.6.3", true),
        ("0.7.0", "v0.7.1", false),
        ("0.6.2", "v0.6.2", false),
        ("0.6.1", "v0.6.1", false),
        ("0.5.10", "v0.5.10", false),
    ] {
        let dir = tempfile::tempdir().unwrap();
        std::fs::create_dir_all(dir.path().join("src")).unwrap();
        std::fs::write(dir.path().join("src/main.rs"), "fn main() {}\n").unwrap();
        std::fs::write(
            dir.path().join("Cargo.toml"),
            format!("[package]\nname = \"spela\"\nversion = \"{version}\"\nedition = \"2021\"\n"),
        )
        .unwrap();
        let (ok, err) = check(dir.path(), tag);
        assert_eq!(ok, want, "{version} tagged {tag}: {err}");
    }
}

#[test]
fn release_notes_are_the_exact_changelog_section() {
    // tests/contracts/release_contract_test.go, TestReleaseNotesAreExactChangelogSection.
    let home = tempfile::tempdir().unwrap();
    let notes = |tag: &str| {
        let mut command = clean_command("sh", home.path(), None);
        command
            .args(["scripts/release-notes.sh", tag])
            .current_dir(root());
        command.output().unwrap()
    };

    let out = notes(&format!("v{}", spela_core::VERSION));
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let text = String::from_utf8(out.stdout).unwrap();
    assert!(text.contains("rewritten in Rust"), "{text}");
    for crossed in [
        "## [",
        "This release makes the profile",
        "Selectable TUI Actions menu",
    ] {
        assert!(
            !text.contains(crossed),
            "the notes cross into another section: {crossed:?}"
        );
    }

    let out = notes("v0.6.0");
    assert!(out.status.success());
    let text = String::from_utf8(out.stdout).unwrap();
    assert!(text.contains("This minor release makes the trusted profile loop visible"));
    for crossed in ["## [0.6.0]", "## [0.5.1]", "This patch release hardens"] {
        assert!(!text.contains(crossed), "{crossed}");
    }

    assert!(
        !notes("v999.0.0").status.success(),
        "a version with no section"
    );
    for tag in [
        "",
        "0.6.0",
        "v0.6",
        "v0.6.0.1",
        "v01.2.3",
        "v1.02.3",
        "v1.2.03",
        "v[0].6.0",
        "v0.*.0",
        "v0x6x0",
        "v1.2.3-rc.1",
        "v1.2.3+meta",
    ] {
        assert!(
            !notes(tag).status.success(),
            "malformed tag {tag:?} gave notes"
        );
    }
}

#[test]
fn the_manifest_workflow_runs_the_rust_updater() {
    let w = workflow("dll-manifest.yml");
    let text = read(".github/workflows/dll-manifest.yml");
    for gone in ["setup-go", "tools/dlss-updater", "python"] {
        assert!(!text.contains(gone), "dll-manifest.yml still uses {gone}");
    }
    assert!(
        !Regex::new(r"\bgo (build|run|tool)")
            .unwrap()
            .is_match(&text),
        "dll-manifest.yml still builds with Go"
    );
    assert_eq!(w["on"]["schedule"][0]["cron"], "0 6 * * *");
    assert_eq!(w["on"]["push"]["branches"], serde_json::json!(["main"]));
    assert_eq!(w["permissions"]["contents"], "write");

    // The matrix is the updater's sources.
    let job = "check-updates";
    let matrix: Vec<&str> = w["jobs"][job]["strategy"]["matrix"]["include"]
        .as_array()
        .unwrap()
        .iter()
        .map(|m| m["dll_type"].as_str().unwrap())
        .collect();
    let sources: Vec<&str> = dlss_updater::scrape::sources()
        .iter()
        .map(|s| s.dll_type.as_str())
        .collect();
    assert_eq!(matrix, sources);

    assert_eq!(
        run_of(&w, job, "Build the updater").trim(),
        "cargo build --locked -p dlss-updater"
    );
    let check = step(&w, job, "Check for ${{ matrix.name }} updates");
    assert_eq!(check["id"], "check");
    let check = check["run"].as_str().unwrap();
    assert!(check.contains(
        r#"./target/debug/dlss-updater -json -type "$DLL_TYPE" -manifest data/manifest.json"#
    ));
    assert!(check.contains("0) has_update=true ;;"), "{check}");
    assert!(check.contains("2) has_update=false ;;"), "{check}");
    let add = run_of(&w, job, "Add ${{ matrix.name }} to the manifest");
    assert!(
        add.contains("./target/debug/dlss-updater add -type \"$DLL_TYPE\""),
        "{add}"
    );
    assert!(add.contains("git push origin main"), "{add}");
}

#[test]
fn the_manual_dll_release_writes_real_file_names() {
    let w = workflow("dll-release.yml");
    let options: Vec<&str> = w["on"]["workflow_dispatch"]["inputs"]["dll_type"]["options"]
        .as_array()
        .unwrap()
        .iter()
        .map(|o| o.as_str().unwrap())
        .collect();
    let catalogue: Vec<&str> = CATALOGUE.iter().map(|f| f.dll_type.as_str()).collect();
    assert_eq!(options, catalogue);
    let validate = run_of(&w, "release-dll", "Validate inputs");
    for family in &CATALOGUE {
        let line = format!("{}) FILENAME={} ;;", family.dll_type, family.filename);
        assert!(validate.contains(&line), "missing {line:?}");
    }
    let text = read(".github/workflows/dll-release.yml");
    assert!(
        !text.contains("python"),
        "dll-release.yml edits the manifest in Python"
    );
    assert!(!text.contains(".dll'"), "a file name built from the type");
    let add = run_of(&w, "release-dll", "Add to the manifest");
    assert!(add.contains("dlss-updater add"), "{add}");
    assert!(add.contains("/$FILENAME\""), "{add}");
}

#[test]
fn the_aur_package_builds_with_cargo_and_installs_binary_and_policy() {
    // tests/contracts/release_contract_test.go, TestReleaseBuildInputsContract,
    // for the AUR half.
    let pkgbuild = read("pkg/aur/PKGBUILD");
    for line in [
        "pkgname=spela-git",
        "source=(\"$_pkgname::git+$url.git\")",
        "git describe --long --abbrev=7 --tags --match 'v[0-9]*'",
        "makedepends=('cargo' 'git')",
        "cargo fetch --locked --target",
        "cargo build --frozen --release -p \"$_pkgname\"",
        "install -Dm755 \"target/release/$_pkgname\" \"$pkgdir/usr/bin/$_pkgname\"",
        "install -Dm644 data/polkit/io.github.spela.policy \"$pkgdir/usr/share/polkit-1/actions/io.github.spela.policy\"",
        "install -Dm644 LICENSE \"$pkgdir/usr/share/licenses/$pkgname/LICENSE\"",
    ] {
        assert!(pkgbuild.contains(line), "PKGBUILD lacks {line:?}");
    }
    for gone in [
        "go tool",
        "mage",
        "'go",
        "bun",
        "webkit2gtk",
        "gtk3",
        "CGO_ENABLED",
    ] {
        assert!(!pkgbuild.contains(gone), "PKGBUILD still has {gone:?}");
    }
    let srcinfo = read("pkg/aur/.SRCINFO");
    assert!(srcinfo.starts_with("pkgbase = spela-git\n"));
    assert!(srcinfo.contains("\npkgname = spela-git\n"));

    // aur-submit.sh reads only pkgver(), so it skips prepare(), which
    // fetches crates.
    let submit = read("scripts/aur-submit.sh");
    assert!(submit.contains("makepkg --nobuild --noprepare --nodeps"));
    assert!(submit.contains("expected_account=jgabor"));
    assert!(submit.contains("Welcome to AUR, ${expected_account}!"));

    if !has_program("makepkg") {
        eprintln!("skipped the .SRCINFO check: needs makepkg");
        return;
    }
    let home = tempfile::tempdir().unwrap();
    let dir = home.path().join("aur");
    std::fs::create_dir_all(&dir).unwrap();
    std::fs::write(dir.join("PKGBUILD"), &pkgbuild).unwrap();
    let mut command = clean_command("makepkg", home.path(), None);
    let out = command
        .arg("--printsrcinfo")
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert_eq!(
        String::from_utf8(out.stdout).unwrap(),
        srcinfo,
        ".SRCINFO is not makepkg --printsrcinfo of the PKGBUILD"
    );
}

#[test]
fn just_install_places_the_binary_and_the_policy() {
    if !has_program("just") {
        eprintln!("skipped: needs just");
        return;
    }
    let home = tempfile::tempdir().unwrap();
    let work = home.path().join("work");
    let policy = read("data/polkit/io.github.spela.policy");
    std::fs::create_dir_all(work.join("data/polkit")).unwrap();
    std::fs::write(work.join("data/polkit/io.github.spela.policy"), &policy).unwrap();
    let staged = home.path().join("root");
    let install = || {
        let mut command = clean_command("just", home.path(), None);
        command
            .arg("--justfile")
            .arg(root().join("justfile"))
            .arg("--working-directory")
            .arg(&work)
            .arg("install")
            .arg(&staged);
        command.output().unwrap()
    };

    let out = install();
    assert!(!out.status.success(), "installs without a release build");
    assert!(
        String::from_utf8_lossy(&out.stderr).contains("run 'just build-release' first"),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(!staged.exists(), "a failed install wrote files");

    std::fs::create_dir_all(work.join("target/release")).unwrap();
    std::fs::write(work.join("target/release/spela"), "#!/bin/sh\n").unwrap();
    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(
        work.join("target/release/spela"),
        std::fs::Permissions::from_mode(0o700),
    )
    .unwrap();
    let out = install();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let mode = |p: &Path| std::fs::metadata(p).unwrap().permissions().mode() & 0o7777;
    let binary = staged.join("usr/bin/spela");
    assert_eq!(std::fs::read_to_string(&binary).unwrap(), "#!/bin/sh\n");
    assert_eq!(mode(&binary), 0o755);
    let installed = staged.join("usr/share/polkit-1/actions/io.github.spela.policy");
    assert_eq!(std::fs::read_to_string(&installed).unwrap(), policy);
    assert_eq!(mode(&installed), 0o644);

    let justfile = read("justfile");
    assert!(
        !justfile.lines().any(|l| l.trim_start().starts_with("sudo")),
        "the recipe never runs sudo itself"
    );
}
