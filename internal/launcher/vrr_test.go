package launcher

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/profile"
)

// The PATH-local doctor records commands only; no real session tool is invoked.
func prepareVRRFixture(t *testing.T, launcher *Launcher) func() {
	t.Helper()
	directory := t.TempDir()
	record := filepath.Join(directory, "commands")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$VRR_COMMANDS"
if [ "$1" = '-o' ]; then
  printf 'Output: 8 DP-3 11111111-2222-3333-4444-555555555555\n\tenabled\n\tconnected\n\tpriority 1\n\tVrr: Never\n'
fi
`
	if err := os.WriteFile(filepath.Join(directory, "kscreen-doctor"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+":"+os.Getenv("PATH"))
	t.Setenv("VRR_COMMANDS", record)
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	launcher.Profile = &profile.Profile{GPU: profile.GPUSettings{VRR: "always"}}
	return func() {
		t.Helper()
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatal(err)
		}
		want := "-o\noutput.11111111-2222-3333-4444-555555555555.vrrpolicy.always\n-o\noutput.11111111-2222-3333-4444-555555555555.vrrpolicy.never\n"
		if string(data) != want {
			t.Fatalf("VRR lifecycle commands = %s, want %s", data, want)
		}
	}
}

func TestVRRTrackedLaunchCleanup(t *testing.T) {
	for _, command := range []string{"/bin/true", "/bin/false", "/nonexistent-spela-test-command"} {
		t.Run(command, func(t *testing.T) {
			launcher := New(nil)
			check := prepareVRRFixture(t, launcher)
			if err := launcher.Prepare(); err != nil {
				t.Fatal(err)
			}
			err := launcher.Launch([]string{command})
			if (err != nil) != (command != "/bin/true") {
				t.Fatalf("launch error = %v", err)
			}
			check()
		})
	}
}

func TestVRRPrepareFailureRestores(t *testing.T) {
	launcher := New(&game.Game{AppID: 1091500, InstallDir: t.TempDir()})
	check := prepareVRRFixture(t, launcher)
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", blocked)
	launcher.Profile.Overlay.Enabled = true
	if err := launcher.Prepare(); err == nil {
		t.Fatal("expected overlay setup failure")
	}
	check()
}

func TestVRRDryRunDoesNotInvokeDoctor(t *testing.T) {
	launcher := New(nil)
	prepareVRRFixture(t, launcher)
	launcher.Profile.ApplyEnv(launcher.Environment)
	if _, err := os.Stat(os.Getenv("VRR_COMMANDS")); !os.IsNotExist(err) {
		t.Fatalf("dry run invoked doctor: %v", err)
	}
}

func TestVRRUnavailableWarnsAndLaunchContinues(t *testing.T) {
	var logs bytes.Buffer
	restore := logging.SetHandler(slog.NewTextHandler(&logs, nil))
	t.Cleanup(restore)
	launcher := New(nil)
	prepareVRRFixture(t, launcher)
	t.Setenv("PATH", t.TempDir())
	if err := launcher.Prepare(); err != nil {
		t.Fatal(err)
	}
	if err := launcher.Launch([]string{"/bin/true"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "failed to apply KDE VRR policy") {
		t.Fatal("missing warning", logs.String())
	}
}
