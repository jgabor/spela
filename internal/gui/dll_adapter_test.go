//go:build dev || production || bindings

package gui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

func TestGUIBoundaryDLLDiscoveryAndRestoreSupportedAdapter(t *testing.T) {
	entry := &game.Game{
		AppID: 1091500, Name: "Cyberpunk 2077", InstallDir: "/game",
		DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Version: "3.7.0", Type: game.DLLTypeDLSS}},
	}
	manifest := &dll.Manifest{DLLs: map[string][]dll.DLL{
		"dlss":           {{Version: "3.8.10", Filename: "nvngx_dlss.dll"}, {Version: "3.7.0", Filename: "nvngx_dlss.dll"}},
		"nvngx_dlss.dll": {{Version: "3.8.10", Filename: "nvngx_dlss.dll"}},
		"xess":           {{Version: "1.3.1", Filename: "libxess.dll"}},
		"empty":          {},
	}}
	boundary := defaultGUIApplicationBoundary(&game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}})
	boundary.getManifest = func(bool, string) (*dll.Manifest, error) { return manifest, nil }

	types, err := boundary.listDLLInstallTypes(entry.AppID)
	if err != nil || !reflect.DeepEqual(types, []string{"dlss"}) {
		t.Fatalf("install types = %v, %v", types, err)
	}
	versions, err := boundary.listDLLVersions("dlss")
	if err != nil || !reflect.DeepEqual(versions, []string{"3.8.10", "3.7.0"}) {
		t.Fatalf("DLL versions = %v, %v", versions, err)
	}
	if _, err := boundary.listDLLVersions(""); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("empty DLL type error = %v", err)
	}
	if _, err := boundary.listDLLVersions("fsr"); err == nil || !strings.Contains(err.Error(), "no versions") {
		t.Fatalf("unknown DLL type error = %v", err)
	}

	updates := boundary.checkDLLUpdates(entry.AppID)
	if len(updates) != 1 || !updates[0].HasUpdate || updates[0].LatestVersion != "3.8.10" {
		t.Fatalf("DLL updates = %+v", updates)
	}
	entry.DLLs[0].Version = "3.8.10"
	if updates := boundary.checkDLLUpdates(entry.AppID); len(updates) != 1 || updates[0].HasUpdate {
		t.Fatalf("current DLL updates = %+v", updates)
	}

	var progress []string
	restored := false
	saved := false
	boundary.emitDLLProgress = func(stage string) { progress = append(progress, stage) }
	boundary.restoreBackup = func(appID uint64) error { restored = appID == entry.AppID; return nil }
	boundary.scanDLLDirectory = func(path string) ([]game.DetectedDLL, error) {
		return []game.DetectedDLL{{Version: "original"}}, nil
	}
	boundary.saveDatabase = func(*game.Database) error { saved = true; return nil }
	if err := boundary.restoreDLLs(entry.AppID); err != nil || !restored || !saved || entry.DLLs[0].Version != "original" || progress[len(progress)-1] != "" {
		t.Fatalf("restore = error %v, restored %v, saved %v, game %+v, progress %v", err, restored, saved, entry, progress)
	}
	boundary.restoreBackup = func(uint64) error { return errors.New("backup corrupt") }
	if err := boundary.restoreDLLs(entry.AppID); err == nil || !strings.Contains(err.Error(), "restore backup") {
		t.Fatalf("restore failure = %v", err)
	}
}

func TestGUIBoundaryDLLAdapterRejectsMissingDatabaseGameAndTypes(t *testing.T) {
	empty := defaultGUIApplicationBoundary(nil)
	if _, err := empty.listDLLInstallTypes(1); !errors.Is(err, ErrDatabaseNotLoaded) {
		t.Fatalf("missing database install types = %v", err)
	}
	if err := empty.restoreDLLs(1); !errors.Is(err, ErrDatabaseNotLoaded) {
		t.Fatalf("missing database restore = %v", err)
	}
	if updates := empty.checkDLLUpdates(1); len(updates) != 0 {
		t.Fatalf("missing database updates = %+v", updates)
	}
	database := &game.Database{Games: map[uint64]*game.Game{}}
	boundary := defaultGUIApplicationBoundary(database)
	if _, err := boundary.listDLLInstallTypes(1); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("missing game install types = %v", err)
	}
	if err := boundary.restoreDLLs(1); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("missing game restore = %v", err)
	}
}
