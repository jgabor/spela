package launcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
)

func TestWrapperModeSupportedExecutableFormsAndInvalidEnvironmentKeys(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"tool", "tool.exe", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	absolute := filepath.Join(bin, "tool")
	for _, test := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--flag"}, false},
		{[]string{absolute}, true},
		{[]string{"relative/tool"}, true},
		{[]string{"tool"}, true},
		{[]string{"tool.exe"}, true},
		{[]string{"notes.txt"}, false},
	} {
		if got := IsWrapperMode(test.args); got != test.want {
			t.Errorf("IsWrapperMode(%v) = %v", test.args, got)
		}
	}
	for _, assignment := range []string{"=value", "1KEY=value", "BAD-KEY=value", "missing-equals"} {
		if _, _, ok := splitEnvAssignment(assignment); ok {
			t.Errorf("invalid assignment %q accepted", assignment)
		}
	}
	invocation := ParseWrapperArguments([]string{"A=1", "--", "tool", "arg"})
	if invocation.Environment["A"] != "1" || len(invocation.Command) != 2 {
		t.Fatalf("wrapper invocation = %+v", invocation)
	}

	db := &game.Database{Games: map[uint64]*game.Game{7: {AppID: 7, Name: "Fixture"}}}
	t.Setenv("SteamAppId", "invalid")
	t.Setenv("SteamGameId", "7")
	if detected := DetectGameFromCommand(db, []string{"tool"}); detected == nil || detected.AppID != 7 {
		t.Fatalf("SteamGameId detection = %+v", detected)
	}
}

func TestLauncherCleanupLabelAndCompatibilitySkipContracts(t *testing.T) {
	launcher := New(&game.Game{AppID: 7, Name: "Fixture"})
	launcher.OnCleanupResult("", func() error { return nil })
	if len(launcher.cleanup) != 1 || launcher.cleanup[0].area != "cleanup" {
		t.Fatalf("cleanup steps = %+v", launcher.cleanup)
	}
	launcher.Profile = &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	launcher.VKD3DCompatibilityCheck = func(uint64) proton.CompatibilityResult {
		return proton.CompatibilityResult{ProtonOK: true, DriverOK: true, DriverSkip: "no NVIDIA driver"}
	}
	launcher.vkd3dPreflight()
	launcher.runCleanup()
	if len(launcher.cleanup) != 0 {
		t.Fatal("cleanup steps were not cleared")
	}
}
