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
