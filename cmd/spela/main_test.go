//go:build !wails

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWrapperModeIgnoresInvalidProfile(t *testing.T) {
	t.Setenv("SteamAppId", "")

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tempDir, "data"))

	profilePath := filepath.Join(tempDir, "config", "spela", "profiles", "1091500.yaml")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}
	if err := os.WriteFile(profilePath, []byte("proton: [unterminated\n"), 0o644); err != nil {
		t.Fatalf("failed to write invalid profile: %v", err)
	}

	writeTestDatabase(t, tempDir)

	stdout, stderr := captureOutput(t, func() error {
		return runWrapperMode([]string{"SteamAppId=1091500", "/usr/bin/env"})
	})

	if stderr == "" || !strings.Contains(stderr, "Warning: failed to load profile for Test Game:") {
		t.Fatalf("expected profile warning in stderr, got %q", stderr)
	}
	if strings.Contains(stderr, "failed to load profile:") {
		t.Fatalf("expected wrapper mode to continue after invalid profile, got fatal error %q", stderr)
	}
	if !strings.Contains(stdout, "Launching Test Game (no profile)...") {
		t.Fatalf("expected launch message without profile, got %q", stdout)
	}
	if strings.Contains(stdout, "PROTON_ENABLE_HDR=1") {
		t.Fatalf("expected invalid profile to be ignored, got stdout %q", stdout)
	}
}

func TestRunWrapperModePreparesLaunchAndPreservesWrapperEnv(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tempDir, "data"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tempDir, "runtime"))

	profilePath := filepath.Join(tempDir, "config", "spela", "profiles", "1091500.yaml")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}
	profileData := "proton:\n  enable_hdr: true\noverlay:\n  enabled: true\n  position: top-right\n"
	if err := os.WriteFile(profilePath, []byte(profileData), 0o644); err != nil {
		t.Fatalf("failed to write valid profile: %v", err)
	}

	writeTestDatabase(t, tempDir)

	stdout, stderr := captureOutput(t, func() error {
		return runWrapperMode([]string{"SteamAppId=1091500", "SPELA_USER_SETTING=keep", "/usr/bin/env"})
	})

	if stderr != "" {
		t.Fatalf("expected no stderr for valid profile, got %q", stderr)
	}
	if !strings.Contains(stdout, "Launching Test Game with profile...") {
		t.Fatalf("expected profile launch message, got %q", stdout)
	}
	if !strings.Contains(stdout, "PROTON_ENABLE_HDR=1") {
		t.Fatalf("expected profile environment to be applied, got %q", stdout)
	}
	if !strings.Contains(stdout, "SPELA_USER_SETTING=keep") {
		t.Fatalf("expected wrapper environment to be preserved, got %q", stdout)
	}
	ipcPath := envValue(stdout, "SPELA_OVERLAY_IPC")
	if ipcPath == "" {
		t.Fatalf("expected wrapper launch to prepare overlay IPC, got %q", stdout)
	}
	if _, err := os.Stat(ipcPath); !os.IsNotExist(err) {
		t.Fatalf("expected overlay IPC cleanup after launch, stat err = %v", err)
	}
}

func TestRootCommandExecutesConfigWiringAndStreams(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", filepath.Join(state, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(state, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(state, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(state, "data"))

	execute := func(args ...string) (string, string, error) {
		return captureOutputResult(t, func() error {
			rootCmd.SetArgs(args)
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(os.Stderr)
			return rootCmd.Execute()
		})
	}

	stdout, stderr, err := execute("config", "set", "check_updates", "false")
	if err != nil || stdout != "Set check_updates = false\n" || stderr != "" {
		t.Fatalf("config set command = stdout %q, stderr %q, error %v", stdout, stderr, err)
	}
	stdout, stderr, err = execute("config", "show")
	if err != nil || !strings.Contains(stdout, "check_updates: false") || stderr != "" {
		t.Fatalf("config show command = stdout %q, stderr %q, error %v", stdout, stderr, err)
	}
	stdout, stderr, err = execute("config", "set", "not_a_key", "value")
	if err == nil || err.Error() != "unknown config key: not_a_key" || !strings.Contains(stdout, "Usage:") || !strings.Contains(stderr, "unknown config key: not_a_key") {
		t.Fatalf("invalid config command = stdout %q, stderr %q, error %v", stdout, stderr, err)
	}
}

func TestRootCommandHelpUsesSuccessfulCobraExecution(t *testing.T) {
	stdout, stderr, err := captureOutputResult(t, func() error {
		rootCmd.SetArgs([]string{})
		rootCmd.SetOut(os.Stdout)
		rootCmd.SetErr(os.Stderr)
		return rootCmd.Execute()
	})
	if err != nil || stderr != "" {
		t.Fatalf("root help = stderr %q, error %v", stderr, err)
	}
	for _, fragment := range []string{"Linux gaming optimization tool", "Available Commands:", "profile", "config"} {
		if !strings.Contains(stdout, fragment) {
			t.Errorf("root help missing %q:\n%s", fragment, stdout)
		}
	}
}

func writeTestDatabase(t *testing.T, tempDir string) {
	t.Helper()

	dataPath := filepath.Join(tempDir, "data", "spela", "games.yaml")
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	data := "games:\n  1091500:\n    app_id: 1091500\n    name: Test Game\n    install_dir: /usr/bin\n"
	if err := os.WriteFile(dataPath, []byte(data), 0o644); err != nil {
		t.Fatalf("failed to write database: %v", err)
	}
}

func captureOutput(t *testing.T, fn func() error) (string, string) {
	stdout, stderr, runErr := captureOutputResult(t, fn)
	if runErr != nil {
		t.Fatalf("runWrapperMode returned error: %v", runErr)
	}
	return stdout, stderr
}

func captureOutputResult(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	runErr := fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	stdoutBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	_ = stdoutReader.Close()

	stderrBytes, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}
	_ = stderrReader.Close()

	return string(stdoutBytes), string(stderrBytes), runErr
}

func envValue(output, key string) string {
	for _, line := range strings.Split(output, "\n") {
		if value, ok := strings.CutPrefix(line, key+"="); ok {
			return value
		}
	}
	return ""
}
