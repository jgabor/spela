package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// transcriptCommand is one read-only Go command run in one sandbox.
type transcriptCommand struct {
	Scenario string   `json:"scenario"`
	Name     string   `json:"name"`
	Args     []string `json:"args"`
	Note     string   `json:"note,omitempty"`
}

// transcriptResult is one recorded run.
type transcriptResult struct {
	transcriptCommand
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr,omitempty"`
	Stable   bool   `json:"stable"`
	Redacted bool   `json:"redacted,omitempty"`
}

// readOnlyForms lists the command forms goref may run. Anything else is
// refused before the binary starts, so a typo cannot reach a command that
// changes hardware, game files or spela's own files.
var readOnlyForms = map[string][]string{
	"help": nil, "list": nil, "show": nil, "bogus": nil,
	"profile":  {"list", "show"},
	"config":   {"show"},
	"dlss":     {"show"},
	"gpu":      {"show"},
	"cpu":      {"show"},
	"proton":   {"show"},
	"overlay":  {"show"},
	"dll":      {"list", "check-updates"},
	"denylist": {"show", "check"},
	"launch":   nil,
}

// checkReadOnly refuses a command goref must not run in the sandbox case at
// scenarioDir. Flags after a bare "--" are game arguments, not spela's, so a
// --dry-run there does not count. dll check-updates may fetch the manifest,
// so it runs only where the case's config points dll_manifest_url at
// loopback.
func checkReadOnly(args []string, scenarioDir string) error {
	own := args
	if at := slices.Index(args, "--"); at >= 0 {
		own = args[:at]
	}
	var words []string
	for _, arg := range own {
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
		}
	}
	if len(words) == 0 {
		for _, arg := range args {
			if arg != "--help" && arg != "-h" && arg != "--version" && arg != "-v" {
				return fmt.Errorf("refusing flags %v", args)
			}
		}
		return nil
	}
	subcommands, ok := readOnlyForms[words[0]]
	if !ok {
		return fmt.Errorf("refusing command %v", args)
	}
	if words[0] == "launch" && !slices.Contains(own, "--dry-run") {
		return fmt.Errorf("refusing launch without --dry-run: %v", args)
	}
	if words[0] == "dll" && len(words) > 1 && words[1] == "check-updates" {
		if err := checkLoopbackManifest(scenarioDir); err != nil {
			return fmt.Errorf("refusing %v: %w", args, err)
		}
	}
	if subcommands != nil && (len(words) < 2 || !slices.Contains(subcommands, words[1])) {
		if len(words) == 1 && (words[0] == "dll" || words[0] == "profile" || words[0] == "denylist") {
			return nil // a parent with no subcommand prints its help
		}
		return fmt.Errorf("refusing %v", args)
	}
	return nil
}

// checkLoopbackManifest fails unless the case's config.yaml sets
// dll_manifest_url to a loopback URL.
func checkLoopbackManifest(scenarioDir string) error {
	data, err := os.ReadFile(filepath.Join(scenarioDir, "config", "spela", "config.yaml"))
	if err != nil {
		return fmt.Errorf("no config.yaml, so the manifest URL is Go's remote default: %w", err)
	}
	var config struct {
		ManifestURL string `yaml:"dll_manifest_url"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}
	parsed, err := url.Parse(config.ManifestURL)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("dll_manifest_url %q is not a URL", config.ManifestURL)
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("dll_manifest_url %q is not loopback", config.ManifestURL)
	}
	return nil
}

// mapOrder reports the commands whose output order comes from a Go map
// (dll list without a game and dll check-updates range over the game
// database's map). Their order is not part of the contract, so they are
// always marked unstable, whatever two runs show.
func mapOrder(args []string) bool {
	return slices.Equal(args, []string{"dll", "list"}) || slices.Equal(args, []string{"dll", "check-updates"})
}

func runTranscripts(arguments []string) error {
	flags := flag.NewFlagSet("transcripts", flag.ContinueOnError)
	binary := flags.String("bin", "", "Go spela binary built from this tree")
	fixtures := flags.String("fixtures", filepath.Join("tests", "fixtures", "go"), "fixture folder")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *binary == "" {
		return errors.New("-bin is required")
	}
	bin, err := filepath.Abs(*binary)
	if err != nil {
		return err
	}
	fixtureDir, err := filepath.Abs(*fixtures)
	if err != nil {
		return err
	}
	outDir := filepath.Join(fixtureDir, "transcripts")

	runner := &transcriptRunner{bin: bin, fixtures: fixtureDir}
	helpPaths, err := runner.discoverHelp()
	if err != nil {
		return err
	}
	commands := transcriptPlan(helpPaths)

	var results []transcriptResult
	for _, command := range commands {
		if err := checkReadOnly(command.Args, filepath.Join(fixtureDir, filepath.FromSlash(command.Scenario))); err != nil {
			return err
		}
		first, err := runner.run(command)
		if err != nil {
			return fmt.Errorf("%s/%s: %w", command.Scenario, command.Name, err)
		}
		second, err := runner.run(command)
		if err != nil {
			return fmt.Errorf("%s/%s: %w", command.Scenario, command.Name, err)
		}
		first.Stable = !mapOrder(command.Args) && first.ExitCode == second.ExitCode && first.Stdout == second.Stdout && first.Stderr == second.Stderr
		results = append(results, first)
	}

	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	return writeTranscripts(outDir, bin, results)
}

type transcriptRunner struct {
	bin      string
	fixtures string
}

// run copies the scenario's fixture case into a fresh folder, runs the
// command there with a closed environment, and swaps the folder for
// FixtureRoot in the output.
func (r *transcriptRunner) run(command transcriptCommand) (transcriptResult, error) {
	result := transcriptResult{transcriptCommand: command}
	root, err := os.MkdirTemp(guardRoot, "run-")
	if err != nil {
		return result, err
	}
	defer func() { _ = os.RemoveAll(root) }()
	if err := copyRelocated(filepath.Join(r.fixtures, filepath.FromSlash(command.Scenario)), root, FixtureRoot, root); err != nil {
		return result, err
	}
	for _, dir := range []string{"empty-bin", "runtime", "home", "config", "data", "cache"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return result, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, r.bin, command.Args...)
	process.Dir = root
	process.Env = sandboxEnvironment(root)
	process.Stdin = nil
	var stdout, stderr bytes.Buffer
	process.Stdout = &stdout
	process.Stderr = &stderr
	runErr := process.Run()
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		result.ExitCode = exitErr.ExitCode()
	default:
		return result, runErr
	}
	result.Stdout = strings.ReplaceAll(stdout.String(), root, FixtureRoot)
	result.Stderr = strings.ReplaceAll(stderr.String(), root, FixtureRoot)
	result.Stdout, result.Redacted = redactMachineValues(result.Stdout)
	var redacted bool
	result.Stderr, redacted = redactMachineValues(result.Stderr)
	result.Redacted = result.Redacted || redacted
	return result, nil
}

// sandboxEnvironment is the whole environment of a transcript run. PATH is
// an empty folder, so no subcommand name can resolve to a program on this
// machine (Go's wrapper detection runs before command parsing), and pkexec,
// nvidia-smi and kscreen-doctor cannot be found. The proxy variables point
// at a closed loopback port with no NO_PROXY, so any request the Go binary
// makes to a remote host fails at once instead of leaving the machine.
func sandboxEnvironment(root string) []string {
	return []string{
		"HTTP_PROXY=" + closedProxy,
		"HTTPS_PROXY=" + closedProxy,
		"http_proxy=" + closedProxy,
		"https_proxy=" + closedProxy,
		"HOME=" + filepath.Join(root, "home"),
		"USER=fixture",
		"LOGNAME=fixture",
		"PATH=" + filepath.Join(root, "empty-bin"),
		"XDG_CONFIG_HOME=" + filepath.Join(root, "config"),
		"XDG_DATA_HOME=" + filepath.Join(root, "data"),
		"XDG_CACHE_HOME=" + filepath.Join(root, "cache"),
		"XDG_RUNTIME_DIR=" + filepath.Join(root, "runtime"),
		"LANG=C.UTF-8",
		"TERM=dumb",
		"TZ=UTC",
	}
}

// closedProxy is a loopback port nothing listens on: the discard port.
const closedProxy = "http://127.0.0.1:9"

// driverVersions match the NVIDIA driver version in Go's descriptor heap
// notices, "(detected: 580.1.2)" and "(detected: <proton>, driver 580.1.2)".
// It is the only machine value the read-only commands print, and only when
// the driver is below the minimum or a check is skipped.
var driverVersions = []*regexp.Regexp{
	regexp.MustCompile(`(\(detected: )([0-9]+\.[0-9]+(\.[0-9]+)?)(\))`),
	regexp.MustCompile(`(, driver )([0-9]+\.[0-9]+(\.[0-9]+)?)(\))`),
}

func redactMachineValues(text string) (string, bool) {
	replaced := text
	for _, pattern := range driverVersions {
		replaced = pattern.ReplaceAllString(replaced, "${1}<driver>${4}")
	}
	return replaced, replaced != text
}

// Cobra pads names to at least 11 columns, so a long name has one space.
var availableCommand = regexp.MustCompile(`^\s{2}(\S+)\s+\S`)

// discoverHelp walks Go's own help output from the root and returns every
// command path, so the help transcripts follow what the binary registers.
func (r *transcriptRunner) discoverHelp() ([][]string, error) {
	var paths [][]string
	queue := [][]string{{}}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		paths = append(paths, path)
		result, err := r.run(transcriptCommand{Scenario: "sandbox/main", Name: "discover", Args: append([]string{"help"}, path...)})
		if err != nil {
			return nil, err
		}
		inCommands := false
		for _, line := range strings.Split(result.Stdout, "\n") {
			if strings.HasPrefix(line, "Available Commands:") {
				inCommands = true
				continue
			}
			if inCommands {
				if strings.TrimSpace(line) == "" {
					break
				}
				match := availableCommand.FindStringSubmatch(line)
				if match == nil {
					continue
				}
				if match[1] == "help" && len(path) > 0 {
					continue
				}
				queue = append(queue, append(slices.Clone(path), match[1]))
			}
		}
		if len(path) == 0 {
			// The hidden command is not listed.
			queue = append(queue, []string{"apply-profile"})
		}
	}
	return paths, nil
}
