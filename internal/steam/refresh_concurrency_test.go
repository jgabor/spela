package steam

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

func TestRescanAndDLLMutationPreserveFreshGameSet(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_RUNTIME_DIR"} {
		t.Setenv(name, filepath.Join(root, name))
	}
	installDirectory := filepath.Join(root, "game")
	targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
	newTargetPath := filepath.Join(installDirectory, "nested", "nvngx_dlss.dll")
	addedDirectory := filepath.Join(root, "added-game")
	addedTargetPath := filepath.Join(addedDirectory, "libxess.dll")
	for _, directory := range []string{installDirectory, filepath.Dir(newTargetPath), addedDirectory} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{targetPath, newTargetPath, addedTargetPath} {
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tracked := &game.Game{AppID: 1, Name: "tracked", InstallDir: installDirectory, DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: targetPath, Type: game.DLLTypeDLSS, Version: "1.0.0"}}}
	if _, err := game.Transaction(func(current *game.Database) (bool, error) {
		current.Games = map[uint64]*game.Game{1: tracked, 2: {AppID: 2, Name: "removed"}}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	payload := []byte("new")
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	if err := dll.SaveManifest(&dll.Manifest{UpdatedAt: time.Now(), DLLs: map[string][]dll.DLL{
		"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", SHA256: digest}},
		"xess": {{Version: "2.0.0", Filename: "libxess.dll", SHA256: digest}},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, dllType := range []string{"dlss", "xess"} {
		cachePath := dll.GetDLLCachePath(dllType, "2.0.0")
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cachePath, payload, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	scanStarted := make(chan struct{})
	releaseScan := make(chan struct{})
	rescanDone := make(chan error, 1)
	go func() {
		_, err := rescan(config.Default(), func(string, []string) (*game.Database, error) {
			close(scanStarted)
			<-releaseScan
			discovered := &game.Game{AppID: 1, Name: "tracked", InstallDir: installDirectory, DLLs: []game.DetectedDLL{
				{Name: "nvngx_dlss.dll", Path: targetPath, Type: game.DLLTypeDLSS, Version: "1.0.0"},
				{Name: "nvngx_dlss.dll", Path: newTargetPath, Type: game.DLLTypeDLSS, Version: "1.0.0"},
			}}
			return &game.Database{Games: map[uint64]*game.Game{
				1: discovered,
				3: {AppID: 3, Name: "added", InstallDir: addedDirectory, DLLs: []game.DetectedDLL{{Name: "libxess.dll", Path: addedTargetPath, Type: game.DLLTypeXeSS, Version: "1.0.0"}}},
			}}, nil
		})
		rescanDone <- err
	}()
	<-scanStarted
	mutationDone := make(chan dll.BatchResult, 1)
	go func() {
		mutationDone <- dll.UpdateGames([]uint64{1, 3}, "", nil)
	}()
	select {
	case result := <-mutationDone:
		t.Fatalf("DLL mutation bypassed the active rescan transaction: %+v", result)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseScan)
	if err := <-rescanDone; err != nil {
		t.Fatal(err)
	}
	if result := <-mutationDone; result.Updated != 3 || result.Failed != 0 {
		t.Fatalf("fresh-state update = %+v", result)
	}

	database, err := game.LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if database.GetGame(1) == nil || database.GetGame(3) == nil || database.GetGame(2) != nil {
		t.Fatalf("final games = %+v", database.Games)
	}
	for _, path := range []string{targetPath, newTargetPath, addedTargetPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, payload) {
			t.Fatalf("updated DLL %s = %q", path, data)
		}
	}
}
