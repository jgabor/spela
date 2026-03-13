//go:build !wails
// +build !wails

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

func TestRunWrapperModeAppliesValidProfile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tempDir, "data"))

	profilePath := filepath.Join(tempDir, "config", "spela", "profiles", "1091500.yaml")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}
	profileData := "proton:\n  enable_hdr: true\n"
	if err := os.WriteFile(profilePath, []byte(profileData), 0o644); err != nil {
		t.Fatalf("failed to write valid profile: %v", err)
	}

	writeTestDatabase(t, tempDir)

	stdout, stderr := captureOutput(t, func() error {
		return runWrapperMode([]string{"SteamAppId=1091500", "/usr/bin/env"})
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

	runErr := fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = originalStdout
	os.Stderr = originalStderr

	stdoutBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}

	if runErr != nil {
		t.Fatalf("runWrapperMode returned error: %v", runErr)
	}

	return string(stdoutBytes), string(stderrBytes)
}
