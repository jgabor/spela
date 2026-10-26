package dll

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
)

func TestAutoUpdateSkipsDeniedRunningAndUnknownDLLs(t *testing.T) {
	setupDLLTestDirs(t)
	replacement := []byte("replacement")
	server := payloadServer(t, map[string][]byte{"/dll": replacement})
	saveOperationManifest(t, server.URL, map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", URL: server.URL + "/dll", SHA256: checksum(replacement)}}})

	original := []byte("original")
	database := &game.Database{Games: map[uint64]*game.Game{}}
	paths := map[uint64]string{}
	for appID, version := range map[uint64]string{
		1091500: "1.0.0", // outdated
		1172470: "1.0.0", // outdated but deny-listed
		2000000: "1.0.0", // outdated but running
		3000000: "",      // version unknown
		4000000: "2.0.0", // current
	} {
		installDirectory := t.TempDir()
		paths[appID] = filepath.Join(installDirectory, "nvngx_dlss.dll")
		assertWriteFile(t, paths[appID], original)
		database.Games[appID] = operationGame(appID, installDirectory, paths[appID], version)
	}
	saveOperationDatabase(t, database)

	var checked []uint64
	running := func(games []*game.Game) map[uint64]bool {
		for _, entry := range games {
			checked = append(checked, entry.AppID)
		}
		return map[uint64]bool{2000000: true}
	}
	result := AutoUpdate(running)
	if result.Err != nil || result.Batch.Updated != 1 || result.Batch.Failed != 0 {
		t.Fatalf("AutoUpdate() = %+v", result)
	}
	if len(checked) != 2 {
		t.Fatalf("only games with an outdated, allowed DLL need the running check, got %v", checked)
	}
	if notice := result.Notice(); notice != "DLL auto-update: updated 1 DLL; skipped game-2000000 while running" {
		t.Fatalf("notice = %q", notice)
	}
	assertFileBytes(t, paths[1091500], replacement)
	for _, appID := range []uint64{1172470, 2000000, 3000000, 4000000} {
		assertFileBytes(t, paths[appID], original)
	}
	if backup, err := LoadBackup(1091500); err != nil || backup == nil {
		t.Fatalf("the auto-update swap must keep a backup: %+v, %v", backup, err)
	}

	// The swapped fixture has no version resource, so its version is unknown
	// now; the game that stopped running is updated on the next start.
	result = AutoUpdate(func([]*game.Game) map[uint64]bool { return nil })
	if notice := result.Notice(); notice != "DLL auto-update: updated 1 DLL" {
		t.Fatalf("second start notice = %q", notice)
	}
	assertFileBytes(t, paths[2000000], replacement)
	if notice := AutoUpdate(func([]*game.Game) map[uint64]bool { return nil }).Notice(); notice != "" {
		t.Fatalf("a current library must start quietly, got %q", notice)
	}
}

func TestAutoUpdateReportsAFailedSwapAndAMissingManifest(t *testing.T) {
	setupDLLTestDirs(t)
	server := payloadServer(t, map[string][]byte{})
	saveOperationManifest(t, server.URL, map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", URL: server.URL + "/missing", SHA256: checksum([]byte("x"))}}})
	installDirectory := t.TempDir()
	path := filepath.Join(installDirectory, "nvngx_dlss.dll")
	assertWriteFile(t, path, []byte("original"))
	saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{1091500: operationGame(1091500, installDirectory, path, "1.0.0")}})
	result := AutoUpdate(func([]*game.Game) map[uint64]bool { return nil })
	if notice := result.Notice(); result.Batch.Failed != 1 || !strings.HasPrefix(notice, "DLL auto-update: 1 failed (") {
		t.Fatalf("failed swap = %+v, notice %q", result, notice)
	}
	assertFileBytes(t, path, []byte("original"))

	if err := os.Remove(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "spela", ManifestCacheFile)); err != nil {
		t.Fatal(err)
	}
	failing, _ := manifestServer(t)
	saveManifestSettings(t, failing.URL+"/failure", true, 24)
	result = AutoUpdate(func([]*game.Game) map[uint64]bool { t.Fatal("no game is checked without a manifest"); return nil })
	if result.Err == nil || !strings.HasPrefix(result.Notice(), "DLL auto-update skipped: load DLL manifest") {
		t.Fatalf("missing manifest = %+v, notice %q", result, result.Notice())
	}
}

func TestAutoUpdateStopsWhenTheDenyListCannotBeRead(t *testing.T) {
	setupDLLTestDirs(t)
	replacement := []byte("replacement")
	server := payloadServer(t, map[string][]byte{"/dll": replacement})
	saveOperationManifest(t, server.URL, map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", URL: server.URL + "/dll", SHA256: checksum(replacement)}}})
	installDirectory := t.TempDir()
	path := filepath.Join(installDirectory, "nvngx_dlss.dll")
	assertWriteFile(t, path, []byte("original"))
	saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{1091500: operationGame(1091500, installDirectory, path, "1.0.0")}})
	lists := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "lists")
	if err := os.MkdirAll(lists, 0o755); err != nil {
		t.Fatal(err)
	}
	assertWriteFile(t, filepath.Join(lists, "denylist.yaml"), []byte("entries: [unclosed"))

	result := AutoUpdate(func([]*game.Game) map[uint64]bool { t.Fatal("no game is checked without a deny list"); return nil })
	if result.Err == nil || !strings.HasPrefix(result.Notice(), "DLL auto-update skipped: check deny list: read ") {
		t.Fatalf("broken deny list = %+v, notice %q", result, result.Notice())
	}
	assertFileBytes(t, path, []byte("original"))
}

func TestAutoUpdateNoticeCountsAndNames(t *testing.T) {
	result := AutoUpdateResult{
		Batch:   BatchResult{Updated: 3, Failed: 2, Items: []BatchItem{{}, {Err: errors.New("disk full")}, {Err: errors.New("later")}}},
		Running: []string{"Alpha", "Beta"},
	}
	if notice := result.Notice(); notice != "DLL auto-update: updated 3 DLLs; 2 failed (disk full); skipped Alpha, Beta while running" {
		t.Fatalf("notice = %q", notice)
	}
}
