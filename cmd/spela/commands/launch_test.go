package commands

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

func TestRunLaunchDryRunSummarizesPreparationImpacts(t *testing.T) {
	withTempXDG(t)
	seedGameWithDLL(t)

	defaults := &profile.Profile{}
	if err := profile.SaveDefault(defaults); err != nil {
		t.Fatalf("SaveDefault: %v", err)
	}
	gameProfile := &profile.Profile{}
	gameProfile.Proton.VKD3DHeap = true
	gameProfile.DLSS.SROverride = true
	gameProfile.DLSS.SRMode = profile.DLSSModeQuality
	gameProfile.GPU.PowerLimit = 250
	gameProfile.Overlay.Enabled = true
	gameProfile.Overlay.Position = "top-right"
	for _, field := range []string{
		profile.FieldProtonVKD3DHeap,
		profile.FieldDLSSSROverride,
		profile.FieldDLSSSRMode,
		profile.FieldGPUPowerLimit,
		profile.FieldOverlayEnabled,
		profile.FieldOverlayPosition,
	} {
		gameProfile.MarkOverride(field)
	}
	if err := profile.Save(1091500, gameProfile); err != nil {
		t.Fatalf("Save profile: %v", err)
	}

	launchGameID = 0
	launchDryRun = true
	t.Cleanup(func() {
		launchGameID = 0
		launchDryRun = false
	})

	out := captureStdout(t, func() {
		if err := runLaunch(LaunchCmd, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatalf("runLaunch dry-run: %v", err)
		}
	})

	for _, want := range []string{
		"Profile impacts",
		"compatibility:",
		"environment:",
		"system_state:",
		"overlay:",
		"Environment",
		"PROTON_VKD3D_HEAP",
		"DLL",
		"no launch-time DLL file mutation planned",
		"nvngx_dlss.dll",
		"Hardware",
		"GPU power limit:",
		"Overlay",
		"collector IPC will be created before launch",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestRunLaunchDryRunSaysNoProfileSpecificMutation(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	defaults := &profile.Profile{}
	defaults.Proton.EnableHDR = true
	if err := profile.SaveDefault(defaults); err != nil {
		t.Fatalf("SaveDefault: %v", err)
	}

	launchGameID = 0
	launchDryRun = true
	t.Cleanup(func() {
		launchGameID = 0
		launchDryRun = false
	})

	out := captureStdout(t, func() {
		if err := runLaunch(LaunchCmd, []string{"Cyberpunk 2077"}); err != nil {
			t.Fatalf("runLaunch dry-run: %v", err)
		}
	})

	if !strings.Contains(out, "no profile-specific mutation planned") {
		t.Fatalf("expected no profile-specific mutation summary, got:\n%s", out)
	}
	if !strings.Contains(out, "PROTON_ENABLE_HDR") {
		t.Fatalf("expected inherited default environment to remain visible, got:\n%s", out)
	}
}

func TestRunLaunchRejectsDirectSteamURI(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	launchGameID = 0
	launchDryRun = false
	t.Cleanup(func() {
		launchGameID = 0
		launchDryRun = false
	})

	err := runLaunch(LaunchCmd, []string{"Cyberpunk 2077"})
	if err == nil {
		t.Fatal("runLaunch() error = nil, want direct Steam URI rejection")
	}
	if !strings.Contains(err.Error(), "spela %command%") {
		t.Fatalf("expected wrapper guidance in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "cannot track the game lifetime or cleanup coverage") {
		t.Fatalf("expected direct launch tracking warning, got %q", err.Error())
	}
}

func seedGameWithDLL(t *testing.T) {
	t.Helper()
	db := &game.Database{
		Games: map[uint64]*game.Game{
			1091500: {
				AppID: 1091500,
				Name:  "Cyberpunk 2077",
				DLLs: []game.DetectedDLL{{
					Path:    "/games/Cyberpunk 2077/bin/x64/nvngx_dlss.dll",
					Name:    "nvngx_dlss.dll",
					Type:    game.DLLTypeDLSS,
					Version: "3.7.0",
				}},
			},
		},
	}
	if err := db.Save(); err != nil {
		t.Fatalf("save database: %v", err)
	}
}
