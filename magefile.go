//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
)

const (
	binaryName  = "spela"
	coverageDir = "coverage"
)

var Default = Build

func version() (string, error) {
	if value := os.Getenv("SPELA_VERSION"); value != "" {
		return value, nil
	}
	out, err := sh.Output("git", "describe", "--tags", "--always", "--dirty")
	if err != nil || out == "" {
		return "dev", nil
	}
	return out, nil
}

func ldflags() (string, error) {
	v, err := version()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("-s -w -X main.version=%s", v), nil
}

// Build builds the spela CLI and TUI binary
func Build() error {
	ldf, err := ldflags()
	if err != nil {
		return err
	}
	return sh.RunV("go", "build", "-buildvcs=false", "-trimpath", "-ldflags", ldf, "-o", binaryName, "./cmd/spela")
}

func runWithEnv(environment []string, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Env = environment
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func environmentWith(base []string, values map[string]string) []string {
	result := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := values[key]; !replaced {
			result = append(result, entry)
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func coverageEnvironment() ([]string, func(), error) {
	stateRoot, err := os.MkdirTemp("", "spela-coverage-state-")
	if err != nil {
		return nil, nil, fmt.Errorf("create isolated coverage state: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(stateRoot) }

	directories := map[string]string{
		"HOME":                   filepath.Join(stateRoot, "home"),
		"XDG_CONFIG_HOME":        filepath.Join(stateRoot, "config"),
		"XDG_CACHE_HOME":         filepath.Join(stateRoot, "cache"),
		"XDG_DATA_HOME":          filepath.Join(stateRoot, "data"),
		"XDG_STATE_HOME":         filepath.Join(stateRoot, "state"),
		"XDG_RUNTIME_DIR":        filepath.Join(stateRoot, "runtime"),
		"TMPDIR":                 filepath.Join(stateRoot, "tmp"),
		"STEAM_COMPAT_DATA_PATH": filepath.Join(stateRoot, "steam-compat"),
	}
	for name, directory := range directories {
		mode := os.FileMode(0o755)
		if name == "XDG_RUNTIME_DIR" {
			mode = 0o700
		}
		if err := os.MkdirAll(directory, mode); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("create isolated %s: %w", name, err)
		}
	}

	values := directories
	values["GOFLAGS"] = strings.TrimSpace(strings.Join([]string{os.Getenv("GOFLAGS"), "-buildvcs=false"}, " "))
	for _, variable := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		value := os.Getenv(variable)
		if value == "" {
			value, err = sh.Output("go", "env", variable)
			if err != nil {
				cleanup()
				return nil, nil, fmt.Errorf("resolve %s: %w", variable, err)
			}
		}
		values[variable] = strings.TrimSpace(value)
	}
	return environmentWith(os.Environ(), values), cleanup, nil
}

// Test runs all default Go tests.
func Test() error {
	return sh.RunV("go", "test", "-v", "./...")
}

// Coverage runs the canonical Go line-coverage measurement.
func Coverage() error {
	if err := os.RemoveAll(coverageDir); err != nil {
		return err
	}
	if err := os.MkdirAll(coverageDir, 0o755); err != nil {
		return err
	}
	environment, cleanup, err := coverageEnvironment()
	if err != nil {
		return err
	}
	defer cleanup()

	coverPackages := "./cmd/...,./internal/...,./tools/..."
	testPackages := []string{"./cmd/...", "./internal/...", "./tools/..."}
	runGoCoverage := func(name string, packages []string, tags ...string) error {
		args := []string{"test", "-covermode=atomic", "-coverpkg=" + coverPackages,
			"-coverprofile=" + filepath.Join(coverageDir, "go-"+name+".out")}
		if len(tags) > 0 {
			args = append(args, "-tags", strings.Join(tags, ","))
		}
		args = append(args, "-count=1")
		return runWithEnv(environment, "go", append(args, packages...)...)
	}
	if err := runGoCoverage("default", testPackages); err != nil {
		return err
	}
	if err := runGoCoverage("e2e", []string{"./internal/tui"}, "e2e"); err != nil {
		return err
	}
	if err := runWithEnv(environment, "go", "run", "./tools/coverage-inventory",
		"--profile", filepath.Join(coverageDir, "go-default.out"),
		"--profile", filepath.Join(coverageDir, "go-e2e.out")); err != nil {
		return err
	}
	return runWithEnv(environment, "python3", "scripts/merge-coverage.py",
		"--go", filepath.Join(coverageDir, "go-default.out"),
		"--go", filepath.Join(coverageDir, "go-e2e.out"),
		"--output", filepath.Join(coverageDir, "lcov.info"))
}

// CoverageCheck measures canonical coverage and enforces the simplification gate.
func CoverageCheck() error {
	if err := sh.RunV("python3", "-m", "unittest", "tests/test_merge_coverage.py"); err != nil {
		return err
	}
	if err := Coverage(); err != nil {
		return err
	}
	return sh.RunV("python3", "scripts/check-coverage.py", filepath.Join(coverageDir, "lcov.info"), "--minimum", "90")
}

// TestTUIE2E runs the TUI end-to-end integration tests using Terminal Control.
func TestTUIE2E() error {
	return sh.RunV("go", "test", "-tags", "e2e", "-v", "-count=1", "./tests/e2e/...")
}

// TestE2E runs the end-to-end tests: the TUI E2E tests
func TestE2E() error {
	return TestTUIE2E()
}

// Lint runs golangci-lint
func Lint() error {
	return sh.RunV("golangci-lint", "run")
}

// Install installs the binary to GOPATH/bin and /usr/bin. Steam game
// launches run /bin/sh with PATH=/usr/bin:/bin inside the runtime
// supervisor, so a user-shell PATH entry (~/.bin, GOPATH/bin) is not
// enough for `spela %command%` in Steam launch options.
func Install() error {
	ldf, err := ldflags()
	if err != nil {
		return err
	}
	if err := sh.RunV("go", "install", "-ldflags", ldf, "./cmd/spela"); err != nil {
		return err
	}

	goBin, err := sh.Output("go", "env", "GOPATH")
	if err != nil {
		return fmt.Errorf("resolve GOPATH after install: %w", err)
	}
	binaryPath := filepath.Join(strings.TrimSpace(goBin), "bin", binaryName)
	return sh.RunV("sudo", "install", "-m755", binaryPath, "/usr/bin/"+binaryName)
}

// Clean removes build artifacts
func Clean() error {
	if err := sh.Rm(binaryName); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
