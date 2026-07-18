//go:build dev || production || bindings

package gui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

func TestGUIBoundaryDLLAdapterErrorContextContracts(t *testing.T) {
	failure := errors.New("fixture failure")
	entry := &game.Game{AppID: 1, Name: "Fixture", InstallDir: "/game", DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Type: game.DLLTypeDLSS, Version: "3.7.0"}}}
	boundary := defaultGUIApplicationBoundary(&game.Database{Games: map[uint64]*game.Game{1: entry}})
	boundary.getManifest = func(bool, string) (*dll.Manifest, error) { return nil, failure }
	if _, err := boundary.listDLLInstallTypes(1); !errors.Is(err, failure) {
		t.Fatalf("install types manifest error = %v", err)
	}
	if _, err := boundary.listDLLVersions("dlss"); !errors.Is(err, failure) {
		t.Fatalf("versions manifest error = %v", err)
	}

	boundary.getManifest = func(bool, string) (*dll.Manifest, error) {
		return &dll.Manifest{DLLs: map[string][]dll.DLL{"empty": {}, "dlss": {{Version: "3.8.10", Filename: "nvngx_dlss.dll"}}}}, nil
	}
	entry.DLLs = []game.DetectedDLL{{Name: "unsupported.dll", Type: game.DLLType("unsupported")}}
	if _, err := boundary.listDLLInstallTypes(1); err == nil || !strings.Contains(err.Error(), "no supported") {
		t.Fatalf("unsupported install types error = %v", err)
	}
	if _, err := boundary.listDLLVersions("missing"); err == nil || !strings.Contains(err.Error(), "no versions") {
		t.Fatalf("missing versions error = %v", err)
	}
	if err := boundary.installDLLVersion(1, "", "latest"); err == nil || !strings.Contains(err.Error(), "type is required") {
		t.Fatalf("missing install type error = %v", err)
	}
	if err := boundary.installDLLVersion(2, "dlss", "latest"); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("missing install game error = %v", err)
	}
	if err := boundary.installDLLVersion(1, "missing", ""); err == nil || !strings.Contains(err.Error(), "no version") {
		t.Fatalf("missing latest version error = %v", err)
	}

	entry.DLLs = []game.DetectedDLL{{Name: "nvngx_dlss.dll", Type: game.DLLTypeDLSS, Version: "3.7.0"}}
	boundary.ensureDLLCached = func(*dll.DLL, string) (string, error) { return "", failure }
	if err := boundary.installDLLVersion(1, "dlss", "3.8.10"); err == nil || !strings.Contains(err.Error(), "download 3.8.10") {
		t.Fatalf("install cache error = %v", err)
	}
	boundary.ensureDLLCached = func(*dll.DLL, string) (string, error) { return "/cache", nil }
	boundary.installDLL = func(uint64, string, string, []dll.GameDLL, string, string) error { return failure }
	if err := boundary.installDLLVersion(1, "dlss", "3.8.10"); err == nil || !strings.Contains(err.Error(), "install DLL") {
		t.Fatalf("install mutation error = %v", err)
	}

	boundary.getManifest = func(bool, string) (*dll.Manifest, error) { return nil, failure }
	if err := boundary.updateDLLs(1); err == nil || !strings.Contains(err.Error(), "load DLL manifest") {
		t.Fatalf("update manifest error = %v", err)
	}
	boundary.getManifest = func(bool, string) (*dll.Manifest, error) {
		return &dll.Manifest{DLLs: map[string][]dll.DLL{"dlss": {{Version: "3.8.10", Filename: "nvngx_dlss.dll"}}}}, nil
	}
	boundary.ensureDLLCached = func(*dll.DLL, string) (string, error) { return "", failure }
	if err := boundary.updateDLLs(1); err == nil || !strings.Contains(err.Error(), "download dlss") {
		t.Fatalf("update cache error = %v", err)
	}
	boundary.ensureDLLCached = func(*dll.DLL, string) (string, error) { return "/cache", nil }
	boundary.swapDLL = func(uint64, string, []dll.GameDLL, string, string) error { return failure }
	if err := boundary.updateDLLs(1); err == nil || !strings.Contains(err.Error(), "swap nvngx_dlss.dll") {
		t.Fatalf("update swap error = %v", err)
	}

	boundary.swapDLL = func(uint64, string, []dll.GameDLL, string, string) error { return nil }
	boundary.scanDLLDirectory = func(string) ([]game.DetectedDLL, error) { return nil, failure }
	boundary.saveDatabase = func(*game.Database) error { return failure }
	if err := boundary.updateDLLs(1); err == nil || !strings.Contains(err.Error(), "save game database") {
		t.Fatalf("update save error = %v", err)
	}
	boundary.restoreBackup = func(uint64) error { return nil }
	if err := boundary.restoreDLLs(1); err == nil || !strings.Contains(err.Error(), "save game database") {
		t.Fatalf("restore save error = %v", err)
	}
}
