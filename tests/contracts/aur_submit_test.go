package contracts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Real makepkg exercises VCS extraction/pkgver and configuration precedence;
// only remote transport and publication are replaced. No network is used.
func TestAURSubmit(t *testing.T) {
	makepkg, err := exec.LookPath("makepkg")
	if err != nil || os.Geteuid() == 0 {
		t.Skip("requires makepkg and a non-root user")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := contractRepositoryRoot(t)
	original, err := os.ReadFile(filepath.Join(root, "pkg/aur/PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"preview", "publish", "current", "download-failure", "pkgver-failure", "srcinfo-failure", "wrong-account"} {
		t.Run(scenario, func(t *testing.T) {
			directory := t.TempDir()
			write := func(name, text string) {
				t.Helper()
				path := filepath.Join(directory, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				mode := os.FileMode(0o644)
				if strings.HasPrefix(name, "bin/") {
					mode = 0o755
				}
				if err := os.WriteFile(path, []byte(text), mode); err != nil {
					t.Fatal(err)
				}
			}
			runGit := func(location string, arguments ...string) string {
				t.Helper()
				command := exec.Command(git, append([]string{"-C", filepath.Join(directory, location)}, arguments...)...)
				command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v\n%s", arguments, err, output)
				}
				return strings.TrimSpace(string(output))
			}
			write("upstream/content", "remote source\n")
			runGit("upstream", "init", "-b", "master")
			runGit("upstream", "add", ".")
			runGit("upstream", "commit", "-m", "fixture")
			runGit("upstream", "tag", "v1.2.3")
			write("upstream/content", "new remote commit\n")
			runGit("upstream", "commit", "-am", "advance")
			version := "1.2.3.r1.g" + runGit("upstream", "rev-parse", "--short=7", "HEAD")
			if scenario == "pkgver-failure" {
				runGit("upstream", "tag", "-d", "v1.2.3")
			}
			write("local/pkg/aur/PKGBUILD", string(original))
			write("local/pkg/aur/.SRCINFO", "original metadata\n")
			write("local/pkg/aur/src/sentinel", "original source\n")
			runGit("local", "init", "-b", "master")
			runGit("local", "add", ".")
			runGit("local", "commit", "-m", "local history is unrelated")
			runGit("local", "tag", "v99.0.0")
			write("aur/PKGBUILD", string(original))
			write("aur/.SRCINFO", "old metadata\n")
			// AUR contents must not be usable as a source cache.
			write("aur/spela/sentinel", "not upstream\n")
			runGit("aur", "init", "-b", "master")
			runGit("aur", "add", ".")
			runGit("aur", "commit", "-m", "old metadata")
			write("temporary/.keep", "")
			write("outside/sentinel", "untouched\n")
			write("makepkg.conf", "source /etc/makepkg.conf\n"+`SRCDEST="$FIXTURE/outside"
BUILDDIR="$FIXTURE/outside"
PKGDEST="$FIXTURE/outside"
SRCPKGDEST="$FIXTURE/outside"
LOGDEST="$FIXTURE/outside"
`)
			write("bin/ssh", `#!/bin/bash
printf '%s\n' "$*" >> "$FIXTURE/ssh.log"
if [[ $SCENARIO == wrong-account ]]; then
  printf 'Welcome to AUR, other!\n'
else
  printf 'Welcome to AUR, jgabor!\n'
fi
`)
			write("bin/git", `#!/bin/bash
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
`)
			write("bin/makepkg", `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/makepkg.log"
if [[ $1 == --printsrcinfo && $SCENARIO == srcinfo-failure ]]; then exit 43; fi
exec "$REAL_MAKEPKG" "$@"
`)
			environment := append(os.Environ(), "PATH="+filepath.Join(directory, "bin")+":"+os.Getenv("PATH"), "FIXTURE="+directory, "REAL_GIT="+git, "REAL_MAKEPKG="+makepkg, "SCENARIO="+scenario, "TMPDIR="+filepath.Join(directory, "temporary"), "MAKEPKG_CONF="+filepath.Join(directory, "makepkg.conf"), "SRCDEST="+filepath.Join(directory, "outside"), "BUILDDIR="+filepath.Join(directory, "outside"), "AUR_SSH_TARGET=test-aur", "AUR_COMMIT_NAME=Test", "AUR_COMMIT_EMAIL=test@example.invalid", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
			environment = append(environment, "BASH_ENV=/dev/null")
			if scenario == "current" {
				lines := strings.Split(string(original), "\n")
				for index, line := range lines {
					if strings.HasPrefix(line, "pkgver=") {
						lines[index] = "pkgver=" + version
					}
				}
				write("aur/PKGBUILD", strings.Join(lines, "\n"))
				command := exec.Command(makepkg, "--printsrcinfo")
				command.Dir = filepath.Join(directory, "aur")
				command.Env = environment
				output, err := command.Output()
				if err != nil {
					t.Fatal(err)
				}
				write("aur/.SRCINFO", string(output))
				runGit("aur", "commit", "-am", "current metadata")
			}
			arguments := []string{filepath.Join(root, "scripts/aur-submit.sh")}
			if scenario != "preview" {
				arguments = append(arguments, "--publish")
			}
			command := exec.Command("bash", arguments...)
			command.Dir = filepath.Join(directory, "local")
			command.Env = environment
			output, err := command.CombinedOutput()
			failure := strings.Contains(scenario, "failure") || scenario == "wrong-account"
			if (err != nil) != failure {
				t.Fatalf("error = %v\n%s", err, output)
			}
			read := func(name string) string {
				t.Helper()
				data, err := os.ReadFile(filepath.Join(directory, name))
				if os.IsNotExist(err) {
					return ""
				}
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			if scenario == "publish" {
				if read("publication.log") != "commit\npush\n" || read("staged") != ".SRCINFO\nPKGBUILD\n" {
					t.Fatalf("unexpected publication: %q staged: %q", read("publication.log"), read("staged"))
				}
				if !strings.Contains(read("PKGBUILD"), "pkgver="+version+"\n") || !strings.Contains(read(".SRCINFO"), "pkgver = "+version+"\n") {
					t.Fatal("published metadata does not match remote version")
				}
			} else if read("publication.log") != "" {
				t.Fatalf("unexpected publication: %s", read("diff"))
			}
			if scenario == "preview" {
				for _, expected := range []string{"+pkgver=" + version, "+\tpkgver = " + version, "Dry run only."} {
					if !strings.Contains(string(output), expected) {
						t.Fatalf("missing %q\n%s", expected, output)
					}
				}
			}
			if scenario == "current" && !strings.Contains(string(output), "already current") {
				t.Fatalf("not a no-op: %s", output)
			}
			if scenario == "wrong-account" && read("makepkg.log") != "" {
				t.Fatal("prepared despite account rejection")
			}
			if !strings.Contains(read("ssh.log"), "-T test-aur") {
				t.Fatal("SSH target override lost")
			}
			if status := runGit("local", "status", "--porcelain"); status != "" {
				t.Fatalf("original tree changed: %s", status)
			}
			for name, expected := range map[string]string{"outside": "sentinel", "temporary": ".keep"} {
				entries, err := os.ReadDir(filepath.Join(directory, name))
				if err != nil || len(entries) != 1 || entries[0].Name() != expected {
					t.Fatalf("%s not isolated/cleaned: %v, %v", name, entries, err)
				}
			}
		})
	}
}
