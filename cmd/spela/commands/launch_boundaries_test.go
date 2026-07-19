package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

func TestLaunchSummaryProjectsEverySupportedHardwareAndDLLOutcome(t *testing.T) {
	state := withTempXDG(t)
	writable := filepath.Join(state, "writable.dll")
	readOnly := filepath.Join(state, "readonly.dll")
	if err := os.WriteFile(writable, []byte("dll"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readOnly, []byte("dll"), 0o444); err != nil {
		t.Fatal(err)
	}
	entry := &game.Game{AppID: 7, Name: "Fixture", DLLs: []game.DetectedDLL{
		{Name: "b.dll", Type: game.DLLTypeDLSS, Version: "1", Path: writable},
		{Name: "a.dll", Type: game.DLLTypeDLSS, Version: "", Path: readOnly},
		{Name: "x.dll", Type: game.DLLTypeXeSS, Version: "2", Path: filepath.Join(state, "missing.dll")},
	}}
	smt := true
	effective := &profile.Profile{
		GPU:     profile.GPUSettings{ClockOffset: 100, MemoryOffset: 500, PowerLimit: 350, FanSpeed: 70},
		CPU:     profile.CPUSettings{Governor: "performance", SMT: &smt},
		Overlay: profile.OverlaySettings{Enabled: true, Position: "top-right"},
	}
	raw := &profile.Profile{Overrides: map[string]bool{profile.FieldGPUClockOffset: true}}
	output := captureStdout(t, func() { PrintLaunchSummary(entry, raw, nil, effective, []string{"game", "arg"}) })
	for _, fragment := range []string{"GPU clock offset", "GPU memory offset", "GPU power limit", "GPU fan speed", "CPU governor", "CPU SMT", "write bit present", "no write bit", "path not accessible", "collector IPC"} {
		if !strings.Contains(output, fragment) {
			t.Errorf("launch summary missing %q:\n%s", fragment, output)
		}
	}
	if summaryValue(nil) != "unset" || summaryValue("   ") != "unset" {
		t.Fatal("empty launch summary values changed")
	}
}

func TestRunLaunchSupportedLookupAndLoadErrors(t *testing.T) {
	state := withTempXDG(t)
	gamesPath := filepath.Join(state, "data", "spela", "games.yaml")
	if err := os.MkdirAll(filepath.Dir(gamesPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gamesPath, []byte("not: [yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runLaunch(nil, []string{"Fixture"}); err == nil || !strings.Contains(err.Error(), "load game database") {
		t.Fatalf("database error = %v", err)
	}
	if err := os.Remove(gamesPath); err != nil {
		t.Fatal(err)
	}
	seedGame(t, "Fixture", 7)
	launchGameID = 999
	if err := runLaunch(nil, []string{"Fixture"}); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("game ID lookup error = %v", err)
	}
	launchGameID = 0
	profilePath := filepath.Join(state, "config", "spela", "profiles", "7.yaml")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte("not: [yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runLaunch(nil, []string{"Fixture"}); err == nil || !strings.Contains(err.Error(), "failed to load profile") {
		t.Fatalf("profile load error = %v", err)
	}
}
