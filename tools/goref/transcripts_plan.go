package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// transcriptPlan lists every command goref records. All of them only read.
func transcriptPlan(helpPaths [][]string) []transcriptCommand {
	var commands []transcriptCommand
	add := func(scenario, name, note string, args ...string) {
		commands = append(commands, transcriptCommand{Scenario: scenario, Name: name, Args: args, Note: note})
	}
	const mainBox = "sandbox/main"

	add(mainBox, "version", "", "--version")
	add(mainBox, "version-short", "cobra's -v", "-v")
	add(mainBox, "root", "no arguments prints the root help")
	add(mainBox, "root-help-flag", "", "--help")
	for _, path := range helpPaths {
		name := "help"
		if len(path) > 0 {
			name = "help-" + strings.Join(path, "-")
		}
		add(mainBox, name, "", append([]string{"help"}, path...)...)
	}
	add(mainBox, "unknown-command", "an unknown word that is not on PATH", "bogus")
	add(mainBox, "parent-dll", "a parent with no subcommand prints its help", "dll")
	add(mainBox, "parent-profile", "", "profile")
	add(mainBox, "parent-denylist", "", "denylist")

	add(mainBox, "list", "", "list")
	add(mainBox, "list-with-dlls", "", "list", "--with-dlls")
	add(mainBox, "list-json", "", "list", "--json")
	add(mainBox, "list-with-dlls-json", "", "list", "--with-dlls", "--json")

	add(mainBox, "show-id", "", "show", "9100010")
	add(mainBox, "show-name", "", "show", "Glimmer Run")
	add(mainBox, "show-name-with-colon", "", "show", "Paper Comet: Director's Cut")
	add(mainBox, "show-no-dlls", "", "show", "9100030")
	add(mainBox, "show-second-library", "", "show", "9100070")
	add(mainBox, "show-json", "", "show", "--json", "9100010")
	add(mainBox, "show-tool", "a Steam tool the scan dropped", "show", "9100040")
	add(mainBox, "show-pending-update", "StateFlags 6, which the scan dropped", "show", "9100050")
	add(mainBox, "show-missing", "", "show", "missing")
	add(mainBox, "show-case-mismatch", "names match exactly", "show", "glimmer run")
	add(mainBox, "show-no-args", "", "show")

	add(mainBox, "profile-list", "", "profile", "list")
	add(mainBox, "profile-show", "", "profile", "show", "9100010")
	add(mainBox, "profile-show-created", "a profile with only its name", "profile", "show", "9100020")
	add(mainBox, "profile-show-none", "a game with no profile", "profile", "show", "9100030")
	add(mainBox, "profile-show-by-name", "", "profile", "show", "Frostline Arena")
	add(mainBox, "profile-show-json", "the raw stored profile", "profile", "show", "--json", "9100010")
	add(mainBox, "profile-show-json-created", "", "profile", "show", "--json", "9100020")
	add(mainBox, "profile-show-json-none", "", "profile", "show", "--json", "9100030")
	add(mainBox, "profile-show-missing", "", "profile", "show", "missing")

	add(mainBox, "config-show", "", "config", "show")

	for _, section := range []string{"dlss", "gpu", "cpu", "proton", "overlay"} {
		add(mainBox, section+"-show", "", section, "show", "9100010")
		add(mainBox, section+"-show-json", "", section, "show", "--json", "9100010")
		add(mainBox, section+"-show-none", "a game with no profile: All games values", section, "show", "9100030")
		add(mainBox, section+"-show-json-none", "", section, "show", "--json", "9100030")
	}
	add(mainBox, "proton-show-ge", "", "proton", "show", "9100020")
	add(mainBox, "proton-show-unresolved-tool", "descriptor_heap on with a tool name Go cannot find: an info notice", "proton", "show", "9100070")
	add(mainBox, "proton-show-unsupported-tool", "descriptor_heap on with a GE build: a warning notice", "proton", "show", "9100060")

	add(mainBox, "dll-list", "map order: always marked unstable", "dll", "list")
	add(mainBox, "dll-list-game", "", "dll", "list", "9100010")
	add(mainBox, "dll-list-game-other-families", "", "dll", "list", "9100020")
	add(mainBox, "dll-list-game-none", "", "dll", "list", "9100030")
	add(mainBox, "dll-list-unknown-version", "", "dll", "list", "9100060")
	add(mainBox, "dll-check-updates", "map order: always marked unstable; manifest cache seeded, source matches the loopback URL, so no fetch", "dll", "check-updates")

	add(mainBox, "denylist-show", "", "denylist", "show")
	add(mainBox, "denylist-check-user-denied", "", "denylist", "check", "9100070")
	add(mainBox, "denylist-check-allowed", "", "denylist", "check", "9100010")
	add(mainBox, "denylist-check-force-allowed", "a built-in game the user force-allowed", "denylist", "check", "1172470")
	add(mainBox, "denylist-check-built-in", "", "denylist", "check", "252490")
	add(mainBox, "denylist-check-unknown-id", "an id not in the game database", "denylist", "check", "9100999")
	add(mainBox, "denylist-check-missing-name", "", "denylist", "check", "nope")

	add(mainBox, "launch-dry-run", "", "launch", "--dry-run", "9100010")
	add(mainBox, "launch-dry-run-created-profile", "", "launch", "--dry-run", "9100020")
	add(mainBox, "launch-dry-run-no-profile", "All games applies directly", "launch", "--dry-run", "9100030")
	add(mainBox, "launch-dry-run-denied", "", "launch", "--dry-run", "9100070")
	add(mainBox, "launch-dry-run-unknown-version", "", "launch", "--dry-run", "9100060")
	add(mainBox, "launch-dry-run-by-name", "", "launch", "--dry-run", "Glimmer Run")
	add(mainBox, "launch-dry-run-with-command", "two or more arguments: the summary shows them as the command", "launch", "--dry-run", "9100010", "--", "glimmer.exe", "-dx12")
	add(mainBox, "launch-dry-run-dash-argument", "a game argument that looks like a flag, without --", "launch", "--dry-run", "9100010", "glimmer.exe", "-dx12")
	add(mainBox, "launch-dry-run-game-id", "", "launch", "--dry-run", "--game-id", "9100020", "ignored")
	add(mainBox, "launch-dry-run-missing", "", "launch", "--dry-run", "missing")

	const broken = "sandbox/errors"
	add(broken, "profile-list", "profile list skips the broken files", "profile", "list")
	add(broken, "profile-show-broken-yaml", "", "profile", "show", "9100030")
	add(broken, "profile-show-invalid-vrr", "", "profile", "show", "9100060")
	add(broken, "profile-show-json-broken-yaml", "", "profile", "show", "--json", "9100030")
	add(broken, "show-broken-profile", "", "show", "9100030")
	add(broken, "dlss-show-invalid-vrr", "", "dlss", "show", "9100060")
	add(broken, "proton-show-broken-yaml", "", "proton", "show", "9100030")
	add(broken, "launch-dry-run-broken-yaml", "", "launch", "--dry-run", "9100030")
	add(broken, "launch-dry-run-invalid-vrr", "", "launch", "--dry-run", "9100060")
	add(broken, "launch-dry-run-fine", "a game whose profile still loads", "launch", "--dry-run", "9100010")

	const empty = "sandbox/empty"
	add(empty, "list", "empty database and no Steam: list tries to scan and fails", "list")
	add(empty, "list-json", "", "list", "--json")
	add(empty, "show", "", "show", "9100010")
	add(empty, "profile-list", "", "profile", "list")
	add(empty, "config-show", "the defaults", "config", "show")
	add(empty, "denylist-show", "the built-in list", "denylist", "show")
	add(empty, "denylist-check-built-in", "", "denylist", "check", "1172470")
	add(empty, "dll-list", "map order: always marked unstable", "dll", "list")
	add(empty, "launch-dry-run", "", "launch", "--dry-run", "9100010")

	const e2e = "e2e/library"
	add(e2e, "list-with-dlls", "", "list", "--with-dlls")
	add(e2e, "profile-list", "", "profile", "list")
	add(e2e, "profile-show", "", "profile", "show", "9100110")
	add(e2e, "dlss-show", "", "dlss", "show", "9100110")
	add(e2e, "launch-dry-run", "", "launch", "--dry-run", "9100110")
	add(e2e, "launch-dry-run-no-profile", "", "launch", "--dry-run", "9100130")
	return commands
}

// shellQuote quotes an argument for the command lines in COMMANDS.md.
func shellQuote(text string) string {
	if text == "" {
		return "''"
	}
	if strings.ContainsAny(text, " '\"$&;:|<>()*?!\\`") {
		return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
	}
	return text
}

func writeTranscripts(outDir, bin string, results []transcriptResult) error {
	var index []map[string]any
	var table strings.Builder
	unstable := 0
	for _, result := range results {
		base := filepath.Join(outDir, filepath.FromSlash(result.Scenario), result.Name)
		if err := writeText(base+".out", result.Stdout); err != nil {
			return err
		}
		entry := map[string]any{
			"scenario":  result.Scenario,
			"name":      result.Name,
			"args":      result.Args,
			"exit_code": result.ExitCode,
			"stdout":    result.Scenario + "/" + result.Name + ".out",
			"stable":    result.Stable,
		}
		if result.Stderr != "" {
			if err := writeText(base+".err", result.Stderr); err != nil {
				return err
			}
			entry["stderr"] = result.Scenario + "/" + result.Name + ".err"
		}
		if result.Note != "" {
			entry["note"] = result.Note
		}
		if result.Redacted {
			entry["redacted"] = "NVIDIA driver version replaced by <driver>"
		}
		if !result.Stable {
			unstable++
		}
		index = append(index, entry)

		quoted := make([]string, 0, len(result.Args)+1)
		quoted = append(quoted, "spela-go")
		for _, arg := range result.Args {
			quoted = append(quoted, shellQuote(arg))
		}
		flagsText := ""
		if !result.Stable {
			flagsText += " unstable"
		}
		if result.Redacted {
			flagsText += " redacted"
		}
		files := "`" + result.Name + ".out`"
		if result.Stderr != "" {
			files += ", `" + result.Name + ".err`"
		}
		fmt.Fprintf(&table, "| `%s` | `%s` | %d | %s |%s |\n", result.Scenario, strings.ReplaceAll(strings.Join(quoted, " "), "|", `\|`), result.ExitCode, files, flagsText)
	}
	if err := writeJSON(filepath.Join(outDir, "index.json"), index); err != nil {
		return err
	}
	content := strings.ReplaceAll(commandsHeader, "{COUNT}", fmt.Sprint(len(results)))
	content = strings.ReplaceAll(content, "{UNSTABLE}", fmt.Sprint(unstable))
	content += "| Sandbox | Command line | Exit | Files | Notes |\n|---|---|---|---|---|\n" + table.String()
	if err := writeText(filepath.Join(outDir, "COMMANDS.md"), content); err != nil {
		return err
	}
	fmt.Printf("wrote %d transcripts (%d unstable) to %s with %s\n", len(results), unstable, outDir, bin)
	return nil
}

const commandsHeader = `# Go command transcripts

The output of the read-only commands of the Go binary, run against the
sandbox cases in this folder's parent. {COUNT} commands; {UNSTABLE} of them
are marked unstable, because two runs gave different output or the output
comes in Go map order (see below).

## Regenerate

Build the Go binary outside the repository, then run goref from the
repository root:

` + "```bash" + `
GOPROXY=off GOFLAGS=-mod=mod go build -o /tmp/spela-go-ref/spela-go ./cmd/spela
GOPROXY=off GOFLAGS=-mod=mod go run ./tools/goref transcripts -bin /tmp/spela-go-ref/spela-go
` + "```" + `

The build is the CLI and TUI only: the GUI and its Wails build tag were
removed in Phase 1, so it needs no frontend. Its version is ` + "`dev`" + `.

## How each command runs

For each command and each of two runs, goref copies the sandbox case into a
fresh temp folder ` + "`$ROOT`" + `, replaces ` + "`/fixture-root`" + ` with ` + "`$ROOT`" + ` in its
text files, and runs the binary from ` + "`$ROOT`" + ` with exactly this environment
and no stdin:

` + "```bash" + `
env -i HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 \
  http_proxy=http://127.0.0.1:9 https_proxy=http://127.0.0.1:9 \
  HOME=$ROOT/home USER=fixture LOGNAME=fixture PATH=$ROOT/empty-bin \
  XDG_CONFIG_HOME=$ROOT/config XDG_DATA_HOME=$ROOT/data \
  XDG_CACHE_HOME=$ROOT/cache XDG_RUNTIME_DIR=$ROOT/runtime \
  LANG=C.UTF-8 TERM=dumb TZ=UTC \
  /tmp/spela-go-ref/spela-go ARGS...
` + "```" + `

Then ` + "`$ROOT`" + ` is replaced by ` + "`/fixture-root`" + ` in stdout and stderr.

- PATH is an empty folder. Go checks for wrapper mode before it parses
  commands, and treats any first word that is a program on PATH as a game to
  launch (on this machine ` + "`spela overlay`" + ` would run ` + "`/sbin/overlay`" + `). With an
  empty PATH no word resolves, and pkexec, nvidia-smi and kscreen-doctor
  cannot be found either.
- Help is only read through ` + "`help <command>`" + `, never ` + "`<command> --help`" + `.
  The help paths come from walking Go's own "Available Commands" lists.
- goref refuses any command outside a fixed read-only list
  (` + "`checkReadOnly`" + ` in ` + "`tools/goref/transcripts.go`" + `), and ` + "`launch`" + ` without
  ` + "`--dry-run`" + ` before any ` + "`--`" + `.
- Commands that change hardware, game files or spela's files, ` + "`scan`" + ` and
  ` + "`tui`" + ` are not run. ` + "`gpu info`" + `, ` + "`cpu info`" + ` and the other live readers
  are not recorded, because their output is this machine's.
- The proxy variables point at ` + "`http://127.0.0.1:9`" + `, a closed loopback
  port, with no ` + "`NO_PROXY`" + `, so a request to a remote host fails at once.
- The sandboxes set ` + "`rescan_on_startup`" + `, ` + "`check_updates`" + ` and
  ` + "`auto_refresh_manifest`" + ` to false and ` + "`dll_manifest_url`" + ` to
  ` + "`http://127.0.0.1:9/`" + `, where nothing listens. The manifest cache is seeded
  from ` + "`data/manifest.json`" + ` with that URL as its source, so no fetch is tried.
  The empty sandbox has no config, so ` + "`dll check-updates`" + ` is not run there:
  goref refuses ` + "`dll check-updates`" + ` in any case whose ` + "`dll_manifest_url`" + ` is
  not loopback.
- ` + "`dll list`" + ` without a game and ` + "`dll check-updates`" + ` print games in Go map
  order, so they are always marked unstable. Their order is not part of the
  contract; compare them as sets of lines.
- The descriptor heap notice that ` + "`proton show`" + ` prints depends on the machine
  that runs goref: on the NVIDIA driver NVML reports and whether it meets
  580.94.16. On the machine that wrote these transcripts the driver met the
  minimum, so the notices name only the Proton build. A driver version in a
  notice (` + "`(detected: <ver>)`" + ` or ` + "`, driver <ver>)`" + `) is replaced by ` + "`<driver>`" + `
  and the run is marked redacted.
- Go writes 24-bit ANSI color codes on every run, even to a pipe and with
  ` + "`NO_COLOR`" + ` set. The transcripts keep them; strip ` + "`\\x1b\\[[0-9;]*m`" + ` to compare
  text.

## Files

` + "`index.json`" + ` lists every command with its arguments, exit code, stdout file,
stderr file (only when stderr was not empty) and whether two runs matched.
Each command has ` + "`<sandbox>/<name>.out`" + ` and, when stderr was not empty,
` + "`<sandbox>/<name>.err`" + `.

## Commands

`
