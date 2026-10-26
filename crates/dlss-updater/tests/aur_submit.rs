//! `scripts/aur-submit.sh` against fake ssh and git, with the real makepkg:
//! a dry run first, publish only with `--publish`, and only as `jgabor`.
//!
//! The port of Go's `tests/contracts/aur_submit_test.go`. Real makepkg runs
//! the PKGBUILD's source extraction and `pkgver()`. The fake git serves the
//! upstream and AUR repositories from local folders and records a commit or
//! push instead of making it, so nothing reaches the network or the AUR.

mod common;

use std::path::{Path, PathBuf};
use std::process::Command;

use common::{clean_command, find_program, read, root};

struct Scenario {
    dir: tempfile::TempDir,
    git: PathBuf,
}

impl Scenario {
    fn path(&self, name: &str) -> PathBuf {
        self.dir.path().join(name)
    }

    fn write(&self, name: &str, text: &str) {
        let path = self.path(name);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(&path, text).unwrap();
        if name.starts_with("bin/") {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o755)).unwrap();
        }
    }

    fn read(&self, name: &str) -> String {
        std::fs::read_to_string(self.path(name)).unwrap_or_default()
    }

    /// Runs the real git in one of the scenario's repositories.
    fn git(&self, repo: &str, args: &[&str]) -> String {
        let out = Command::new(&self.git)
            .arg("-C")
            .arg(self.path(repo))
            .args(args)
            .env_clear()
            .env("PATH", common::system_path())
            .env("HOME", self.path("home"))
            .env("GIT_CONFIG_GLOBAL", "/dev/null")
            .env("GIT_CONFIG_SYSTEM", "/dev/null")
            .env("GIT_AUTHOR_NAME", "Test")
            .env("GIT_AUTHOR_EMAIL", "test@example.invalid")
            .env("GIT_COMMITTER_NAME", "Test")
            .env("GIT_COMMITTER_EMAIL", "test@example.invalid")
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "git {args:?}: {}",
            String::from_utf8_lossy(&out.stderr)
        );
        String::from_utf8(out.stdout).unwrap().trim().to_owned()
    }

    fn entries(&self, name: &str) -> Vec<String> {
        let mut names: Vec<String> = std::fs::read_dir(self.path(name))
            .unwrap()
            .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
            .collect();
        names.sort();
        names
    }
}

const FAKE_SSH: &str = r#"#!/bin/bash
printf '%s\n' "$*" >> "$FIXTURE/ssh.log"
if [[ $SCENARIO == wrong-account ]]; then
  printf 'Welcome to AUR, other!\n'
else
  printf 'Welcome to AUR, jgabor!\n'
fi
"#;

const FAKE_GIT: &str = r#"#!/bin/bash
set -eu
if [[ $1 == clone && $2 == ssh://test-aur/spela-git.git ]]; then
  exec "$REAL_GIT" clone "$FIXTURE/aur" "$3"
fi
if [[ $1 == clone && $* == *https://github.com/jgabor/spela.git* ]]; then
  [[ $SCENARIO != download-failure ]] || exit 42
  arguments=("$@")
  for index in "${!arguments[@]}"; do
    [[ ${arguments[index]} != https://github.com/jgabor/spela.git ]] || arguments[index]="$FIXTURE/upstream"
  done
  exec "$REAL_GIT" "${arguments[@]}"
fi
for argument in "$@"; do
  if [[ $argument == commit || $argument == push ]]; then
    printf '%s\n' "$argument" >> "$FIXTURE/publication.log"
    "$REAL_GIT" -C "$2" diff --cached --name-only > "$FIXTURE/staged"
    "$REAL_GIT" -C "$2" diff --cached > "$FIXTURE/diff"
    cp "$2/PKGBUILD" "$2/.SRCINFO" "$FIXTURE/"
    exit 0
  fi
done
exec "$REAL_GIT" "$@"
"#;

const FAKE_MAKEPKG: &str = r#"#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/makepkg.log"
if [[ $1 == --printsrcinfo && $SCENARIO == srcinfo-failure ]]; then exit 43; fi
exec "$REAL_MAKEPKG" "$@"
"#;

fn is_root() -> bool {
    Command::new("id")
        .arg("-u")
        .output()
        .map(|o| String::from_utf8_lossy(&o.stdout).trim() == "0")
        .unwrap_or(true)
}

/// A command with the scenario's fakes first on PATH and its own HOME.
fn command(s: &Scenario, program: &str, scenario: &str, makepkg: &Path) -> Command {
    let mut command = clean_command(program, &s.path("home-root"), Some(&s.path("bin")));
    let fixture = s.dir.path().to_str().unwrap();
    command
        .env("FIXTURE", fixture)
        .env("REAL_GIT", &s.git)
        .env("REAL_MAKEPKG", makepkg)
        .env("SCENARIO", scenario)
        .env("TMPDIR", s.path("temporary"))
        .env("MAKEPKG_CONF", s.path("makepkg.conf"))
        .env("SRCDEST", s.path("outside"))
        .env("BUILDDIR", s.path("outside"))
        .env("AUR_SSH_TARGET", "test-aur")
        .env("AUR_COMMIT_NAME", "Test")
        .env("AUR_COMMIT_EMAIL", "test@example.invalid")
        .env("GIT_CONFIG_GLOBAL", "/dev/null")
        .env("GIT_CONFIG_SYSTEM", "/dev/null")
        .env("BASH_ENV", "/dev/null");
    command
}

fn run_scenario(scenario: &str, git: &Path, makepkg: &Path) {
    let s = Scenario {
        dir: tempfile::tempdir().unwrap(),
        git: git.to_owned(),
    };
    let pkgbuild = read("pkg/aur/PKGBUILD");

    s.write("upstream/content", "remote source\n");
    s.git("upstream", &["init", "-q", "-b", "master"]);
    s.git("upstream", &["add", "."]);
    s.git("upstream", &["commit", "-q", "-m", "fixture"]);
    s.git("upstream", &["tag", "v1.2.3"]);
    s.write("upstream/content", "new remote commit\n");
    s.git("upstream", &["commit", "-q", "-am", "advance"]);
    let version = format!(
        "1.2.3.r1.g{}",
        s.git("upstream", &["rev-parse", "--short=7", "HEAD"])
    );
    if scenario == "pkgver-failure" {
        s.git("upstream", &["tag", "-d", "v1.2.3"]);
    }

    // The checkout the script runs from: its history and tags must not
    // decide the version.
    s.write("local/pkg/aur/PKGBUILD", &pkgbuild);
    s.write("local/pkg/aur/.SRCINFO", "original metadata\n");
    s.write("local/pkg/aur/src/sentinel", "original source\n");
    s.git("local", &["init", "-q", "-b", "master"]);
    s.git("local", &["add", "."]);
    s.git(
        "local",
        &["commit", "-q", "-m", "local history is unrelated"],
    );
    s.git("local", &["tag", "v99.0.0"]);

    s.write("aur/PKGBUILD", &pkgbuild);
    s.write("aur/.SRCINFO", "old metadata\n");
    // The AUR contents must not be usable as a source cache.
    s.write("aur/spela/sentinel", "not upstream\n");
    s.git("aur", &["init", "-q", "-b", "master"]);
    s.git("aur", &["add", "."]);
    s.git("aur", &["commit", "-q", "-m", "old metadata"]);

    s.write("temporary/.keep", "");
    s.write("outside/sentinel", "untouched\n");
    let fixture = s.dir.path().display();
    s.write(
        "makepkg.conf",
        &format!(
            "source /etc/makepkg.conf\nSRCDEST=\"{fixture}/outside\"\nBUILDDIR=\"{fixture}/outside\"\nPKGDEST=\"{fixture}/outside\"\nSRCPKGDEST=\"{fixture}/outside\"\nLOGDEST=\"{fixture}/outside\"\n"
        ),
    );
    s.write("bin/ssh", FAKE_SSH);
    s.write("bin/git", FAKE_GIT);
    s.write("bin/makepkg", FAKE_MAKEPKG);

    if scenario == "current" {
        let current: String = pkgbuild
            .lines()
            .map(|l| {
                if l.starts_with("pkgver=") {
                    format!("pkgver={version}\n")
                } else {
                    format!("{l}\n")
                }
            })
            .collect();
        s.write("aur/PKGBUILD", &current);
        let out = command(&s, makepkg.to_str().unwrap(), scenario, makepkg)
            .arg("--printsrcinfo")
            .current_dir(s.path("aur"))
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        s.write("aur/.SRCINFO", &String::from_utf8(out.stdout).unwrap());
        s.git("aur", &["commit", "-q", "-am", "current metadata"]);
    }

    let mut submit = command(&s, "bash", scenario, makepkg);
    submit.arg(root().join("scripts/aur-submit.sh"));
    if scenario != "preview" {
        submit.arg("--publish");
    }
    let out = submit.current_dir(s.path("local")).output().unwrap();
    let output = format!(
        "{}{}",
        String::from_utf8_lossy(&out.stdout),
        String::from_utf8_lossy(&out.stderr)
    );
    let should_fail = scenario.contains("failure") || scenario == "wrong-account";
    assert_eq!(!out.status.success(), should_fail, "{scenario}: {output}");

    if scenario == "publish" {
        assert_eq!(s.read("publication.log"), "commit\npush\n", "{output}");
        assert_eq!(s.read("staged"), ".SRCINFO\nPKGBUILD\n");
        assert!(
            s.read("PKGBUILD").contains(&format!("pkgver={version}\n")),
            "the published PKGBUILD has the upstream version"
        );
        assert!(
            s.read(".SRCINFO")
                .contains(&format!("pkgver = {version}\n"))
        );
    } else {
        assert_eq!(
            s.read("publication.log"),
            "",
            "{scenario}: {}",
            s.read("diff")
        );
    }
    if scenario == "preview" {
        for expected in [
            format!("+pkgver={version}"),
            format!("+\tpkgver = {version}"),
            "Dry run only.".to_owned(),
        ] {
            assert!(output.contains(&expected), "missing {expected:?}\n{output}");
        }
    }
    if scenario == "current" {
        assert!(output.contains("already current"), "{output}");
    }
    if scenario == "wrong-account" {
        assert_eq!(
            s.read("makepkg.log"),
            "",
            "prepared despite the wrong account"
        );
    } else {
        // prepare() would fetch crates: the script must skip it.
        let log = s.read("makepkg.log");
        assert!(
            log.lines()
                .filter(|l| l.contains("--nobuild"))
                .all(|l| l.contains("--noprepare")),
            "{log}"
        );
    }
    assert!(s.read("ssh.log").contains("-T test-aur"), "SSH target lost");
    assert_eq!(
        s.git("local", &["status", "--porcelain"]),
        "",
        "the checkout changed"
    );
    assert_eq!(
        s.entries("outside"),
        ["sentinel"],
        "makepkg wrote outside its temp dir"
    );
    assert_eq!(
        s.entries("temporary"),
        [".keep"],
        "the temp dir was not cleaned"
    );
}

#[test]
fn aur_submit_previews_first_and_publishes_only_as_jgabor() {
    let (Some(makepkg), Some(git)) = (find_program("makepkg"), find_program("git")) else {
        eprintln!("skipped: needs makepkg and git");
        return;
    };
    if is_root() {
        eprintln!("skipped: makepkg refuses to run as root");
        return;
    }
    for scenario in [
        "preview",
        "publish",
        "current",
        "download-failure",
        "pkgver-failure",
        "srcinfo-failure",
        "wrong-account",
    ] {
        run_scenario(scenario, &git, &makepkg);
    }
}
