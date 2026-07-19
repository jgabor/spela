package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

func TestShowAndGPUInfoSupportedTextAndFallbackErrors(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	configuration := config.Default()
	configuration.RescanOnStartup = false
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	entry := &game.Game{AppID: 7, Name: "Fixture", InstallDir: "/games/fixture", PrefixPath: "/prefix", DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: "/games/fixture/nvngx_dlss.dll", Type: game.DLLTypeDLSS}}}
	saveCommandDatabase(t, &game.Database{Games: map[uint64]*game.Game{7: entry}})
	if err := profile.Save(7, &profile.Profile{Name: "Fixture"}); err != nil {
		t.Fatal(err)
	}
	showJSON = false
	if output := captureStdout(t, func() {
		if err := runShow(nil, []string{"Fixture"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "Prefix:") || !strings.Contains(output, "unknown") || !strings.Contains(output, "yes") {
		t.Fatalf("show output:\n%s", output)
	}

	bin := t.TempDir()
	nvidiaSMI := filepath.Join(bin, "nvidia-smi")
	if err := os.WriteFile(nvidiaSMI, []byte("#!/bin/sh\nprintf 'RTX 5090, 590.1, 32768, 55, 250.5\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if output := captureStdout(t, func() {
		if err := runGPUInfo(nil, nil); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "RTX 5090") || !strings.Contains(output, "250.5 W") {
		t.Fatalf("GPU info output:\n%s", output)
	}
	if err := os.WriteFile(nvidiaSMI, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runGPUInfo(nil, nil); err == nil || !strings.Contains(err.Error(), "failed to get GPU info") {
		t.Fatalf("GPU info error = %v", err)
	}
}

func TestDLLCommandSupportedEmptyMissingAndCurrentContracts(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_CACHE_HOME", state+"/cache")
	empty := &game.Database{Games: map[uint64]*game.Game{}}
	saveCommandDatabase(t, empty)
	if output := captureStdout(t, func() {
		if err := runDLLList(nil, nil); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "No games") {
		t.Fatalf("empty DLL list output = %q", output)
	}
	if err := runDLLList(nil, []string{"missing"}); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("missing DLL list game error = %v", err)
	}

	entry := &game.Game{AppID: 7, Name: "Fixture", InstallDir: state + "/game", DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: state + "/game/nvngx_dlss.dll", Type: game.DLLTypeDLSS, Version: "3.9.0"}}}
	saveCommandDatabase(t, &game.Database{Games: map[uint64]*game.Game{7: entry}})
	manifest := &dll.Manifest{Version: "1", UpdatedAt: time.Now(), DLLs: map[string][]dll.DLL{"dlss": {{Version: "3.9.0", Filename: "nvngx_dlss.dll"}}}}
	if err := dll.SaveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if err := runDLLUpdate(nil, []string{"missing", "dlss"}); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("missing DLL update game error = %v", err)
	}
	if err := runDLLUpdate(nil, []string{"Fixture", "xess"}); err == nil || !strings.Contains(err.Error(), "does not have") {
		t.Fatalf("missing DLL type error = %v", err)
	}
	if output := captureStdout(t, func() {
		if err := runDLLUpdate(nil, []string{"Fixture", "dlss"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.Contains(output, "already at the latest") {
		t.Fatalf("current DLL output = %q", output)
	}
	if err := runDLLRestore(nil, []string{"missing"}); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("missing restore game error = %v", err)
	}
	if err := runDLLRestore(nil, []string{"Fixture"}); err == nil || !strings.Contains(err.Error(), "no backup") {
		t.Fatalf("missing backup error = %v", err)
	}
}
