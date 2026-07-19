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
}
