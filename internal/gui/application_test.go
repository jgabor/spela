//go:build dev || production || bindings

package gui

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
)

func TestGUIBoundaryProfilePassUsesGameProfile(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadProfile = func(appID uint64) (*profile.Profile, error) {
		if appID != 1091500 {
			t.Fatalf("unexpected appID: %d", appID)
		}
		return &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}, nil
	}
	boundary.loadDefaultProfile = func() (*profile.Profile, error) {
		t.Fatal("default profile should not load when game profile exists")
		return nil, nil
	}

	info := boundary.getProfile(1091500)
	if info == nil {
		t.Fatal("expected profile info")
	}
	if !info.EnableHDR || info.InheritedFromDefault {
		t.Fatalf("expected game profile HDR override, got %+v", *info)
	}
}

func TestGUIBoundaryProfileFailReturnsNil(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadProfile = func(uint64) (*profile.Profile, error) {
		return nil, errors.New("profile read failed")
	}

	if info := boundary.getProfile(1091500); info != nil {
		t.Fatalf("expected nil profile on load failure, got %+v", *info)
	}
}

func TestGUIBoundaryCompatibilityPassUsesDomainNotice(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(nil)
	boundary.loadConfig = func() (*config.Config, error) {
		return &config.Config{SteamPath: "/steam/root"}, nil
	}
	boundary.compatibilityNotice = func(appID uint64, deps proton.NoticeDeps) string {
		if appID != 1091500 {
			t.Fatalf("unexpected appID: %d", appID)
		}
		if deps.SteamRoot != "/steam/root" {
			t.Fatalf("expected configured Steam root, got %q", deps.SteamRoot)
		}
		return "⚠ descriptor_heap requires Proton-CachyOS 9.0+"
	}

	got := boundary.vkd3dHeapCompatibilityNotice(1091500)
	if !strings.Contains(got, "descriptor_heap requires") {
		t.Fatalf("expected domain compatibility notice, got %q", got)
	}
}

func TestGUIBoundaryLaunchFailExplainsSteamLaunchOptions(t *testing.T) {
	app := &App{db: &game.Database{Games: map[uint64]*game.Game{
		1091500: {AppID: 1091500, Name: "Cyberpunk 2077"},
	}}}

	err := app.LaunchGame(1091500)
	if err == nil {
		t.Fatal("expected direct launch rejection")
	}
	message := err.Error()
	if !strings.Contains(message, "cannot track the game lifetime") || !strings.Contains(message, "spela %command%") {
		t.Fatalf("expected Steam launch-option guidance, got %q", message)
	}
}

func TestGUIBoundaryDLLPassInstallsThroughBoundary(t *testing.T) {
	gameEntry := &game.Game{
		AppID:      1091500,
		Name:       "Cyberpunk 2077",
		InstallDir: "/games/cyberpunk",
		DLLs: []game.DetectedDLL{{
			Path:    "/games/cyberpunk/nvngx_dlss.dll",
			Name:    "nvngx_dlss.dll",
			Type:    game.DLLTypeDLSS,
			Version: "3.7.0",
		}},
	}
	boundary := defaultGUIApplicationBoundary(&game.Database{Games: map[uint64]*game.Game{1091500: gameEntry}})
	var progress []string
	boundary.emitDLLProgress = func(stage string) { progress = append(progress, stage) }
	boundary.getManifest = func(forceUpdate bool, manifestURL string) (*dll.Manifest, error) {
		if forceUpdate || manifestURL != "" {
			t.Fatalf("unexpected manifest request: force=%v url=%q", forceUpdate, manifestURL)
		}
		return &dll.Manifest{DLLs: map[string][]dll.DLL{
			"dlss": {{Version: "3.8.10", Filename: "nvngx_dlss.dll"}},
		}}, nil
	}
	boundary.ensureDLLCached = func(target *dll.DLL, dllName string) (string, error) {
		if target.Version != "3.8.10" || dllName != "dlss" {
			t.Fatalf("unexpected cache target: %+v %q", *target, dllName)
		}
		return "/cache/dlss/3.8.10.dll", nil
	}
	boundary.installDLL = func(appID uint64, gameName, installDir string, gameDLLs []dll.GameDLL, dllName, cachePath string) error {
		if appID != 1091500 || gameName != "Cyberpunk 2077" || installDir != "/games/cyberpunk" {
			t.Fatalf("unexpected install target: %d %q %q", appID, gameName, installDir)
		}
		if len(gameDLLs) != 1 || gameDLLs[0].Version != "3.7.0" {
			t.Fatalf("expected existing game DLL state, got %+v", gameDLLs)
		}
		if dllName != "nvngx_dlss.dll" || cachePath != "/cache/dlss/3.8.10.dll" {
			t.Fatalf("unexpected install payload: %q %q", dllName, cachePath)
		}
		return nil
	}
	boundary.scanDLLDirectory = func(dir string) ([]game.DetectedDLL, error) {
		if dir != "/games/cyberpunk" {
			t.Fatalf("unexpected scan dir: %q", dir)
		}
		return []game.DetectedDLL{{Name: "nvngx_dlss.dll", Type: game.DLLTypeDLSS, Version: "3.8.10"}}, nil
	}
	saved := false
	boundary.saveDatabase = func(db *game.Database) error {
		saved = true
		return nil
	}

	if err := boundary.installDLLVersion(1091500, "dlss", "3.8.10"); err != nil {
		t.Fatal(err)
	}
	if !saved {
		t.Fatal("expected database save")
	}
	if got := gameEntry.DLLs[0].Version; got != "3.8.10" {
		t.Fatalf("expected scanned DLL version to replace game state, got %q", got)
	}
	expectedProgress := []string{
		"Resolving manifest",
		"Downloading dlss 3.8.10",
		"Installing nvngx_dlss.dll",
		"Scanning install directory",
		"Saving database",
		"",
	}
	if strings.Join(progress, "|") != strings.Join(expectedProgress, "|") {
		t.Fatalf("expected progress %q, got %q", expectedProgress, progress)
	}
}

func TestGUIBoundaryDLLFailPreservesManifestError(t *testing.T) {
	boundary := defaultGUIApplicationBoundary(&game.Database{Games: map[uint64]*game.Game{
		1091500: {AppID: 1091500, Name: "Cyberpunk 2077"},
	}})
	var progress []string
	boundary.emitDLLProgress = func(stage string) { progress = append(progress, stage) }
	boundary.getManifest = func(bool, string) (*dll.Manifest, error) {
		return nil, errors.New("manifest offline")
	}

	err := boundary.installDLLVersion(1091500, "dlss", "latest")
	if err == nil {
		t.Fatal("expected DLL install failure")
	}
	if !strings.Contains(err.Error(), "load DLL manifest: manifest offline") {
		t.Fatalf("expected wrapped manifest error, got %q", err.Error())
	}
	if strings.Join(progress, "|") != "Resolving manifest|" {
		t.Fatalf("expected resolving stage and cleanup, got %q", progress)
	}
}

func TestGUILoggingPassUsesCentralizedHandler(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	gamesPath := filepath.Join(dataHome, "spela", "games.yaml")
	if err := os.MkdirAll(gamesPath, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	restore := logging.SetHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(restore)

	app := NewApp()
	app.startup(context.Background())

	logs := buf.String()
	if !strings.Contains(logs, "failed to load game database") || !strings.Contains(logs, "level=ERROR") {
		t.Fatalf("expected GUI startup log through centralized handler, got %q", logs)
	}
}
