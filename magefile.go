//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	binaryName  = "spela"
	frontendDir = "internal/gui/frontend"
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

// Build builds the unified spela binary (frontend + backend)
func Build() error {
	mg.Deps(FrontendBuild)

	ldf, err := ldflags()
	if err != nil {
		return err
	}
	return sh.RunV("go", "build", "-buildvcs=false", "-trimpath", "-tags", "embed_assets,production,webkit2_41", "-ldflags", ldf, "-o", binaryName, "./cmd/spela")
}

// GenNavJS generates GUI nav constants from internal/nav.
func GenNavJS() error {
	out := filepath.Join(frontendDir, "src", "lib", "navContract.generated.js")
	return sh.RunV("go", "run", "./tools/gen-nav-js", out)
}

// FrontendBindings regenerates Wails frontend bindings
func FrontendBindings() error {
	return runInDir(filepath.Join("cmd", "spela"), "go", "tool", "wails", "build", "-s", "-nopackage", "-m", "-tags", "wails,webkit2_41")
}

// FrontendBuild builds the Svelte frontend
func FrontendBuild() error {
	mg.Deps(GenNavJS, FrontendBindings)
	if err := runInDir(frontendDir, "bun", "install", "--frozen-lockfile"); err != nil {
		return err
	}
	return runInDir(frontendDir, "bun", "run", "build")
}

func runInDir(dir string, cmd string, args ...string) error {
	return runInDirWithEnv(dir, nil, cmd, args...)
}

func runInDirWithEnv(dir string, environment []string, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Dir = dir
	if environment != nil {
		c.Env = environment
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func runWithEnv(environment []string, cmd string, args ...string) error {
	return runInDirWithEnv("", environment, cmd, args...)
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
	realHome, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve dependency cache home: %w", err)
	}
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
	values["BUN_INSTALL_CACHE_DIR"] = filepath.Join(realHome, ".bun", "install", "cache")
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

// Test runs all default Go tests plus GUI backend tests behind build tags.
func Test() error {
	if err := sh.RunV("go", "test", "-v", "./..."); err != nil {
		return err
	}
	return sh.RunV("go", "test", "-tags", "dev,webkit2_41", "-v", "./internal/gui")
}

// TestFrontend runs frontend tests
func TestFrontend() error {
	mg.Deps(GenNavJS, FrontendBindings)
	return runInDir(frontendDir, "bun", "run", "test")
}

// Coverage runs the canonical Go and frontend line-coverage measurement.
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
	defer os.RemoveAll(filepath.Join(frontendDir, "coverage"))

	if err := runWithEnv(environment, "go", "run", "./tools/gen-nav-js", filepath.Join(frontendDir, "src", "lib", "navContract.generated.js")); err != nil {
		return err
	}
	if err := runInDirWithEnv(filepath.Join("cmd", "spela"), environment, "go", "tool", "wails", "build", "-s", "-nopackage", "-m", "-tags", "wails,webkit2_41"); err != nil {
		return err
	}
	if err := runInDirWithEnv(frontendDir, environment, "bun", "install", "--frozen-lockfile"); err != nil {
		return err
	}
	if err := runInDirWithEnv(frontendDir, environment, "bun", "run", "build"); err != nil {
		return err
	}

	coverPackages := "./cmd/...,./internal/...,./tools/..."
	testPackages := []string{"./cmd/...", "./internal/...", "./tools/...", "./tests/contracts/..."}
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
	if err := runGoCoverage("gui", testPackages, "dev", "webkit2_41"); err != nil {
		return err
	}
	taggedPackages := []string{"./cmd/spela", "./internal/gui"}
	if err := runGoCoverage("production", taggedPackages, "wails", "production", "webkit2_41"); err != nil {
		return err
	}
	if err := runGoCoverage("embedded", taggedPackages, "wails", "production", "embed_assets", "webkit2_41"); err != nil {
		return err
	}
	if err := runInDirWithEnv(frontendDir, environment, "bun", "run", "test:coverage"); err != nil {
		return err
	}
	if err := runWithEnv(environment, "go", "run", "./tools/coverage-inventory",
		"--profile", filepath.Join(coverageDir, "go-default.out"),
		"--profile", filepath.Join(coverageDir, "go-gui.out"),
		"--profile", filepath.Join(coverageDir, "go-production.out"),
		"--profile", filepath.Join(coverageDir, "go-embedded.out")); err != nil {
		return err
	}
	return runWithEnv(environment, "python3", "scripts/merge-coverage.py",
		"--go", filepath.Join(coverageDir, "go-default.out"),
		"--go", filepath.Join(coverageDir, "go-gui.out"),
		"--go", filepath.Join(coverageDir, "go-production.out"),
		"--go", filepath.Join(coverageDir, "go-embedded.out"),
		"--frontend", filepath.Join(frontendDir, "coverage", "lcov.info"),
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

// TestTUIE2E runs the TUI end-to-end integration tests using tmux.
func TestTUIE2E() error {
	return sh.RunV("go", "test", "-tags", "e2e", "-v", "-count=1", "./tests/e2e/...")
}

// TestE2E runs Playwright e2e tests and TUI E2E tests
func TestE2E() error {
	mg.Deps(GenNavJS, FrontendBindings, TestTUIE2E)
	return runInDir(frontendDir, "bun", "run", "test:e2e")
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
	mg.Deps(FrontendBuild)

	ldf, err := ldflags()
	if err != nil {
		return err
	}
	if err := sh.RunV("go", "install", "-tags", "embed_assets,production,webkit2_41", "-ldflags", ldf, "./cmd/spela"); err != nil {
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
	if err := sh.Rm(filepath.Join(frontendDir, "dist")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Dev starts development mode with Vite hot-reload
func Dev() error {
	if err := runInDir(frontendDir, "bun", "install", "--frozen-lockfile"); err != nil {
		return err
	}

	fmt.Println("Starting Vite dev server in background...")
	viteCmd := exec.Command("bun", "run", "dev")
	viteCmd.Dir = frontendDir
	viteCmd.Stdout = os.Stdout
	viteCmd.Stderr = os.Stderr
	if err := viteCmd.Start(); err != nil {
		return err
	}

	fmt.Println("Building Go binary with dev tag...")
	ldf, err := ldflags()
	if err != nil {
		viteCmd.Process.Kill()
		return err
	}
	if err := sh.RunV("go", "build", "-tags", "dev,webkit2_41", "-ldflags", ldf, "-o", binaryName, "./cmd/spela"); err != nil {
		viteCmd.Process.Kill()
		return err
	}

	fmt.Println("Starting spela...")
	return sh.RunV("./"+binaryName, "gui")
}

// DevStop stops the Vite dev server
func DevStop() error {
	return sh.Run("pkill", "-f", "bun run dev")
}
