package dll

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/game"
)

func TestOperationsInstallUpdateNoOpAndRestoreExactBytes(t *testing.T) {
	setupDLLTestDirs(t)
	original := []byte("original game DLL")
	first := []byte("first replacement")
	second := []byte("second replacement")
	server := payloadServer(t, map[string][]byte{"/first": first, "/second": second})
	saveOperationManifest(t, server.URL, map[string][]DLL{"dlss": {
		{Version: "2.0.0", Filename: "nvngx_dlss.dll", URL: server.URL + "/second", SHA256: checksum(second)},
		{Version: "1.0.0", Filename: "nvngx_dlss.dll", URL: server.URL + "/first", SHA256: checksum(first)},
	}})

	installDirectory := t.TempDir()
	targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
	if err := os.WriteFile(targetPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	entry := operationGame(1091500, installDirectory, targetPath, "")
	database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
	saveOperationDatabase(t, database)
	var stages []Stage
	result, err := Install(entry.AppID, "dlss", "1.0.0", func(event ProgressEvent) {
		stages = append(stages, event.Stage)
	})
	if err != nil || !result.FilesChanged || !result.MetadataPersisted {
		t.Fatalf("Install() = %+v, %v", result, err)
	}
	assertFileBytes(t, targetPath, first)
	backup, err := LoadBackup(entry.AppID)
	if err != nil || backup == nil || len(backup.Files) != 1 || backup.Files[0].OriginalPath != targetPath {
		t.Fatalf("backup metadata = %+v, %v", backup, err)
	}
	assertFileBytes(t, backup.Files[0].BackupPath, original)
	if !containsStages(stages, StageResolving, StageDownloading, StageInstalling, StageScanning, StageSaving) {
		t.Fatalf("install stages = %v", stages)
	}

	entry = result.Game
	entry.DLLs[0].Version = "1.0.0" // arbitrary fixtures are not PE version resources
	database.Games[entry.AppID] = entry
	saveOperationDatabase(t, database)
	result, err = Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", InstalledPath: targetPath}, nil)
	if err != nil || result.Outcome != OutcomeChanged || !result.MetadataPersisted {
		t.Fatalf("Update() = %+v, %v", result, err)
	}
	assertFileBytes(t, targetPath, second)
	assertFileBytes(t, backup.Files[0].BackupPath, original) // the first backup is reused

	entry = result.Game
	entry.DLLs[0].Version = "2.0.0"
	database.Games[entry.AppID] = entry
	saveOperationDatabase(t, database)
	result, err = Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", InstalledPath: targetPath}, nil)
	if err != nil || result.Outcome != OutcomeNoOp || result.FilesChanged {
		t.Fatalf("no-op Update() = %+v, %v", result, err)
	}

	result, err = Restore(entry.AppID, nil)
	if err != nil || !result.MetadataPersisted {
		t.Fatalf("Restore() = %+v, %v", result, err)
	}
	assertFileBytes(t, targetPath, original)
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_DATA_HOME"), "spela", "games.yaml")); err != nil {
		t.Fatalf("database was not persisted: %v", err)
	}
}

func TestOperationsRejectChecksumAndDenylistBeforeMutation(t *testing.T) {
	for _, test := range []struct {
		name  string
		appID uint64
		hash  string
		want  string
	}{
		{name: "checksum", appID: 1091500, hash: strings.Repeat("0", 64), want: "checksum mismatch"},
		{name: "denylist", appID: 1172470, hash: checksum([]byte("replacement")), want: "denied"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupDLLTestDirs(t)
			payload := []byte("replacement")
			server := payloadServer(t, map[string][]byte{"/dll": payload})
			saveOperationManifest(t, server.URL, map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", URL: server.URL + "/dll", SHA256: test.hash}}})
			installDirectory := t.TempDir()
			targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
			original := []byte("original")
			if err := os.WriteFile(targetPath, original, 0o644); err != nil {
				t.Fatal(err)
			}
			entry := operationGame(test.appID, installDirectory, targetPath, "1.0.0")
			database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
			saveOperationDatabase(t, database)
			result, err := Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", InstalledPath: targetPath}, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) || result.FilesChanged {
				t.Fatalf("Update() = %+v, %v; want %q pre-mutation failure", result, err, test.want)
			}
			assertFileBytes(t, targetPath, original)
		})
	}
}

func TestUpdateVerifiesCachedPayloadAgainstResolvedManifestBeforeMutation(t *testing.T) {
	for _, test := range []struct {
		name    string
		cached  []byte
		wantErr bool
	}{
		{name: "valid offline cache", cached: []byte("verified replacement")},
		{name: "corrupt offline cache", cached: []byte("truncated"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupDLLTestDirs(t)
			payload := []byte("verified replacement")
			saveOperationManifest(t, "offline fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", URL: "http://127.0.0.1:1/unavailable", SHA256: checksum(payload)}}})
			cachePath := GetDLLCachePath("dlss", "2.0.0")
			if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cachePath, test.cached, 0o644); err != nil {
				t.Fatal(err)
			}
			installDirectory := t.TempDir()
			targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
			original := []byte("original")
			if err := os.WriteFile(targetPath, original, 0o644); err != nil {
				t.Fatal(err)
			}
			entry := operationGame(1091500, installDirectory, targetPath, "1.0.0")
			database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
			saveOperationDatabase(t, database)
			result, err := Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", Version: "2.0.0", InstalledPath: targetPath, CachedOnly: true}, nil)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "checksum mismatch") || result.FilesChanged {
					t.Fatalf("Update() = %+v, %v", result, err)
				}
				assertFileBytes(t, targetPath, original)
				return
			}
			if err != nil || !result.FilesChanged {
				t.Fatalf("Update() = %+v, %v", result, err)
			}
			assertFileBytes(t, targetPath, payload)
		})
	}
}

func TestRestoreReportsFilesChangedWhenLaterReplacementFails(t *testing.T) {
	setupDLLTestDirs(t)
	installDirectory := t.TempDir()
	firstPath := filepath.Join(installDirectory, "first", "nvngx_dlss.dll")
	secondPath := filepath.Join(installDirectory, "second", "nvngx_dlss.dll")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	firstOriginal, secondOriginal := []byte("first original"), []byte("second original")
	if err := os.WriteFile(firstPath, firstOriginal, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, secondOriginal, 0o644); err != nil {
		t.Fatal(err)
	}
	detected := []game.DetectedDLL{
		{Name: "nvngx_dlss.dll", Path: firstPath, Type: game.DLLTypeDLSS, Version: "1.0.0"},
		{Name: "nvngx_dlss.dll", Path: secondPath, Type: game.DLLTypeDLSS, Version: "1.0.0"},
	}
	if _, err := CreateBackup(1091500, "fixture", detected); err != nil {
		t.Fatal(err)
	}
	assertWriteFile(t, firstPath, []byte("first changed"))
	assertWriteFile(t, secondPath, []byte("second changed"))
	entry := &game.Game{AppID: 1091500, Name: "fixture", InstallDir: installDirectory, DLLs: detected}
	database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
	replacements := 0
	result, err := restore(database, entry.AppID, nil, func(source, target string) error {
		replacements++
		if replacements == 2 {
			return errors.New("injected second replacement failure")
		}
		return copyFile(source, target)
	})
	var partial *PartialFailure
	if !errors.As(err, &partial) || partial.Stage != StageRestoring || !result.FilesChanged || result.MetadataPersisted {
		t.Fatalf("restoreOperation() = %+v, %#v", result, err)
	}
	assertFileBytes(t, firstPath, firstOriginal)
	assertFileBytes(t, secondPath, []byte("second changed"))
	if _, statErr := os.Stat(filepath.Join(os.Getenv("XDG_DATA_HOME"), "spela", "games.yaml")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("partial restore unexpectedly persisted metadata: %v", statErr)
	}
}

func TestConcurrentExactPathUpdatesBackupAndRestoreDuplicateBasenames(t *testing.T) {
	setupDLLTestDirs(t)
	payload := []byte("replacement")
	saveOperationManifest(t, "fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", SHA256: checksum(payload)}}})
	cachePath := GetDLLCachePath("dlss", "2.0.0")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	assertWriteFile(t, cachePath, payload)
	installDirectory := t.TempDir()
	paths := []string{
		filepath.Join(installDirectory, "bin", "nvngx_dlss.dll"),
		filepath.Join(installDirectory, "plugins", "nvngx_dlss.dll"),
	}
	originals := map[string][]byte{paths[0]: []byte("bin original"), paths[1]: []byte("plugin original")}
	entry := &game.Game{AppID: 1091500, Name: "fixture", InstallDir: installDirectory}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		assertWriteFile(t, path, originals[path])
		entry.DLLs = append(entry.DLLs, game.DetectedDLL{Name: filepath.Base(path), Path: path, Type: game.DLLTypeDLSS, Version: "1.0.0"})
	}
	database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
	saveOperationDatabase(t, database)
	start := make(chan struct{})
	errorsChannel := make(chan error, len(paths))
	var wait sync.WaitGroup
	for _, path := range paths {
		path := path
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", Version: "2.0.0", InstalledPath: path, CachedOnly: true}, nil)
			errorsChannel <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range paths {
		assertFileBytes(t, path, payload)
	}
	backup, err := LoadBackup(entry.AppID)
	if err != nil || len(backup.Files) != 2 || backup.Files[0].BackupPath == backup.Files[1].BackupPath {
		t.Fatalf("backup = %+v, %v", backup, err)
	}
	for _, file := range backup.Files {
		assertFileBytes(t, file.BackupPath, originals[file.OriginalPath])
	}
	if _, err := Restore(entry.AppID, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		assertFileBytes(t, path, originals[path])
	}
}

func TestMutationFailsClosedWhenBackupMetadataCannotBeLoaded(t *testing.T) {
	for _, test := range []struct {
		name      string
		writeMeta func(*testing.T, string)
	}{
		{name: "malformed", writeMeta: func(t *testing.T, path string) { assertWriteFile(t, path, []byte("{")) }},
		{name: "unreadable", writeMeta: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupDLLTestDirs(t)
			payload := []byte("replacement")
			saveOperationManifest(t, "fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", SHA256: checksum(payload)}}})
			cachePath := GetDLLCachePath("dlss", "2.0.0")
			if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
				t.Fatal(err)
			}
			assertWriteFile(t, cachePath, payload)
			installDirectory := t.TempDir()
			targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
			original := []byte("original")
			assertWriteFile(t, targetPath, original)
			backupDirectory := GetBackupDir(1091500)
			if err := os.MkdirAll(backupDirectory, 0o755); err != nil {
				t.Fatal(err)
			}
			orphanBackup := filepath.Join(backupDirectory, "nvngx_dlss.dll")
			assertWriteFile(t, orphanBackup, []byte("do not overwrite"))
			test.writeMeta(t, GetBackupMetadataPath(1091500))
			entry := operationGame(1091500, installDirectory, targetPath, "1.0.0")
			database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
			saveOperationDatabase(t, database)
			result, err := Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", Version: "2.0.0", InstalledPath: targetPath, CachedOnly: true}, nil)
			if err == nil || result.FilesChanged {
				t.Fatalf("Update() = %+v, %v", result, err)
			}
			assertFileBytes(t, targetPath, original)
			assertFileBytes(t, orphanBackup, []byte("do not overwrite"))
			if backup, statusErr := LoadBackup(entry.AppID); backup != nil || statusErr == nil {
				t.Fatalf("LoadBackup() = %v, %v", backup, statusErr)
			}
		})
	}
}

func TestOperationsReturnStructuredPartialFailuresAfterMutation(t *testing.T) {
	t.Run("scan", func(t *testing.T) {
		setupDLLTestDirs(t)
		payload := []byte("replacement")
		cachePath := GetDLLCachePath("dlss", "2.0.0")
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			t.Fatal(err)
		}
		assertWriteFile(t, cachePath, payload)
		actualDirectory := t.TempDir()
		movedDirectory := filepath.Join(t.TempDir(), "moved")
		targetPath := filepath.Join(actualDirectory, "nvngx_dlss.dll")
		assertWriteFile(t, targetPath, []byte("original"))
		entry := operationGame(1091500, actualDirectory, targetPath, "1.0.0")
		saveOperationManifest(t, "fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", SHA256: checksum(payload)}}})
		result, err := update(&game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}, UpdateRequest{AppID: entry.AppID, DLLType: "dlss", Version: "2.0.0", InstalledPath: targetPath, CachedOnly: true}, func(event ProgressEvent) {
			if event.Stage == StageScanning {
				if renameErr := os.Rename(actualDirectory, movedDirectory); renameErr != nil {
					t.Fatal(renameErr)
				}
			}
		})
		assertPartialFailure(t, result, err, StageScanning, "scan install directory")
		if len(entry.DLLs) != 1 || entry.DLLs[0].Version != "1.0.0" {
			t.Fatalf("scan failure replaced prior metadata: %+v", entry.DLLs)
		}
		assertFileBytes(t, filepath.Join(movedDirectory, filepath.Base(targetPath)), payload)
	})
	t.Run("save", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
		t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
		t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
		installDirectory := t.TempDir()
		targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
		assertWriteFile(t, targetPath, []byte("replacement"))
		entry := operationGame(1091500, installDirectory, targetPath, "1.0.0")
		saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}})
		dataDirectory := filepath.Join(os.Getenv("XDG_DATA_HOME"), "spela")
		if err := os.Chmod(dataDirectory, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dataDirectory, 0o755) })
		result, err := mutate(nil, func(database *game.Database) (Result, error) {
			return refresh(database.GetGame(entry.AppID), Result{}, nil)
		})
		assertPartialFailure(t, result, err, StageSaving, "save game database")
		if result.Game == nil {
			t.Fatal("save failure discarded scanned metadata")
		}
	})
}

func assertPartialFailure(t *testing.T, result Result, err error, stage Stage, message string) {
	t.Helper()
	var partial *PartialFailure
	if !errors.As(err, &partial) || partial.Stage != stage || !result.FilesChanged || result.MetadataPersisted {
		t.Fatalf("operation = %+v, %#v", result, err)
	}
	if !strings.Contains(err.Error(), "DLL files changed but metadata did not fully persist") || !strings.Contains(err.Error(), message) {
		t.Fatalf("partial failure = %q", err)
	}
}

func TestBatchUpdateContinuesAfterError(t *testing.T) {
	setupDLLTestDirs(t)
	payload := []byte("batch replacement")
	cachePath := GetDLLCachePath("dlss", "2.0.0")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	saveOperationManifest(t, "fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", SHA256: checksum(payload)}}})
	makeEntry := func(appID uint64) (*game.Game, string) {
		directory := t.TempDir()
		path := filepath.Join(directory, "nvngx_dlss.dll")
		if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		return operationGame(appID, directory, path, "1.0.0"), path
	}
	denied, deniedPath := makeEntry(1172470)
	allowed, allowedPath := makeEntry(1091500)
	database := &game.Database{Games: map[uint64]*game.Game{denied.AppID: denied, allowed.AppID: allowed}}
	saveOperationDatabase(t, database)
	batch := BatchUpdate([]UpdateRequest{
		{AppID: denied.AppID, DLLType: "dlss", Version: "2.0.0", InstalledPath: deniedPath, CachedOnly: true},
		{AppID: allowed.AppID, DLLType: "dlss", Version: "2.0.0", InstalledPath: allowedPath, CachedOnly: true},
	}, nil)
	if batch.Updated != 1 || batch.Failed != 1 || len(batch.Items) != 2 {
		t.Fatalf("BatchUpdate() = %+v", batch)
	}
	assertFileBytes(t, deniedPath, []byte("original"))
	assertFileBytes(t, allowedPath, payload)
}

func TestBatchSaveFailureLeavesNoOpSuccessful(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	installDirectory := t.TempDir()
	currentPath := filepath.Join(installDirectory, "current", "nvngx_dlss.dll")
	stalePath := filepath.Join(installDirectory, "stale", "nvngx_dlss.dll")
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatal(err)
	}
	assertWriteFile(t, currentPath, []byte("current"))
	assertWriteFile(t, stalePath, []byte("stale"))
	entry := &game.Game{AppID: 1, Name: "fixture", InstallDir: installDirectory, DLLs: []game.DetectedDLL{
		{Name: "nvngx_dlss.dll", Path: currentPath, Type: game.DLLTypeDLSS, Version: "2.0.0"},
		{Name: "nvngx_dlss.dll", Path: stalePath, Type: game.DLLTypeDLSS, Version: "1.0.0"},
	}}
	saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}})
	if _, err := CreateBackup(entry.AppID, entry.Name, entry.DLLs); err != nil {
		t.Fatal(err)
	}
	payload := []byte("replacement")
	cachePath := GetDLLCachePath("dlss", "2.0.0")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	assertWriteFile(t, cachePath, payload)
	saveOperationManifest(t, "fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", SHA256: checksum(payload)}}})
	dataDirectory := filepath.Join(os.Getenv("XDG_DATA_HOME"), "spela")
	if err := os.Chmod(dataDirectory, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dataDirectory, 0o755) })

	batch := BatchUpdate([]UpdateRequest{
		{AppID: entry.AppID, DLLType: "dlss", InstalledPath: currentPath},
		{AppID: entry.AppID, DLLType: "dlss", InstalledPath: stalePath},
	}, nil)
	if batch.Unchanged != 1 || batch.Failed != 1 || batch.Updated != 0 {
		t.Fatalf("BatchUpdate() = %+v", batch)
	}
	if batch.Items[0].Err != nil || batch.Items[0].Result.Outcome != OutcomeNoOp {
		t.Fatalf("no-op item = %+v", batch.Items[0])
	}
	var partial *PartialFailure
	if !errors.As(batch.Items[1].Err, &partial) || partial.Stage != StageSaving || !batch.Items[1].Result.FilesChanged {
		t.Fatalf("changed item = %+v", batch.Items[1])
	}
}

func TestOperationValidationAndExactTargetErrors(t *testing.T) {
	setupDLLTestDirs(t)
	entry := &game.Game{AppID: 1091500, Name: "fixture", InstallDir: t.TempDir()}
	database := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}}
	saveOperationDatabase(t, database)
	if _, err := Install(entry.AppID, "", "", nil); err == nil {
		t.Fatal("install accepted an empty DLL type")
	}
	saveOperationManifest(t, "fixture", map[string][]DLL{})
	if _, err := Install(entry.AppID, "dlss", "latest", nil); err == nil || !strings.Contains(err.Error(), "available in manifest") {
		t.Fatalf("missing install version error = %v", err)
	}

	firstPath := filepath.Join(entry.InstallDir, "one", "nvngx_dlss.dll")
	secondPath := filepath.Join(entry.InstallDir, "two", "nvngx_dlss.dll")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		assertWriteFile(t, path, []byte("original"))
		entry.DLLs = append(entry.DLLs, game.DetectedDLL{Name: filepath.Base(path), Path: path, Type: game.DLLTypeDLSS, Version: "1.0.0"})
	}
	payload := []byte("replacement")
	saveOperationManifest(t, "fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: "nvngx_dlss.dll", SHA256: checksum(payload)}}})
	cachePath := GetDLLCachePath("dlss", "2.0.0")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	assertWriteFile(t, cachePath, payload)
	saveOperationDatabase(t, database)
	if _, err := Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", Version: "2.0.0", CachedOnly: true}, nil); err == nil || !strings.Contains(err.Error(), "path not found") {
		t.Fatalf("ambiguous update error = %v", err)
	}
	if _, err := Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", InstalledPath: filepath.Join(entry.InstallDir, "missing.dll")}, nil); err == nil || !strings.Contains(err.Error(), "path not found") {
		t.Fatalf("missing exact path error = %v", err)
	}
	if _, err := Update(UpdateRequest{AppID: entry.AppID, DLLType: "dlss", InstalledPath: firstPath, CachedOnly: true}, nil); err == nil || !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("missing cached version error = %v", err)
	}
	if _, err := Install(entry.AppID, "dlss", "2.0.0", nil); err == nil || !strings.Contains(err.Error(), "exact path") {
		t.Fatalf("ambiguous install error = %v", err)
	}
}

func TestInstallRejectsManifestPathEscapeBeforeCacheOrMutation(t *testing.T) {
	for _, test := range []struct {
		name     string
		filename func(string) string
		cached   bool
	}{
		{name: "cached parent traversal", filename: func(string) string { return "../outside.dll" }, cached: true},
		{name: "remote absolute path", filename: func(outside string) string { return outside }},
		{name: "windows separator", filename: func(string) string { return `..\\outside.dll` }, cached: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupDLLTestDirs(t)
			installDirectory := t.TempDir()
			outside := filepath.Join(filepath.Dir(installDirectory), "outside.dll")
			entry := &game.Game{AppID: 1091500, Name: "fixture", InstallDir: installDirectory}
			saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}})
			payload := []byte("replacement")
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				requests++
				_, _ = writer.Write(payload)
			}))
			defer server.Close()
			saveOperationManifest(t, "fixture", map[string][]DLL{"dlss": {{Version: "2.0.0", Filename: test.filename(outside), URL: server.URL, SHA256: checksum(payload)}}})
			if test.cached {
				cachePath := GetDLLCachePath("dlss", "2.0.0")
				if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
					t.Fatal(err)
				}
				assertWriteFile(t, cachePath, payload)
			}
			result, err := Install(entry.AppID, "dlss", "2.0.0", nil)
			if err == nil || !strings.Contains(err.Error(), "manifest filename") || result.FilesChanged {
				t.Fatalf("Install() = %+v, %v", result, err)
			}
			if requests != 0 {
				t.Fatalf("invalid filename triggered %d remote requests", requests)
			}
			if _, statErr := os.Stat(outside); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("outside path was touched: %v", statErr)
			}
			if backup, statusErr := LoadBackup(entry.AppID); backup != nil || statusErr != nil {
				t.Fatalf("backup was touched: %v, %v", backup, statusErr)
			}
		})
	}
}

func TestOperationDoesNotResurrectGameRemovedBeforeLock(t *testing.T) {
	setupDLLTestDirs(t)
	removed := operationGame(1091500, t.TempDir(), filepath.Join(t.TempDir(), "nvngx_dlss.dll"), "1.0.0")
	request := UpdateRequest{AppID: removed.AppID, DLLType: "dlss", InstalledPath: removed.DLLs[0].Path}
	saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{removed.AppID: removed}})
	saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{42: {AppID: 42, Name: "survivor"}}})
	if _, err := Update(request, nil); err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("removed game update error = %v", err)
	}
	reloaded, err := game.LoadDatabase()
	if err != nil || reloaded.GetGame(removed.AppID) != nil || reloaded.GetGame(42) == nil {
		t.Fatalf("database resurrected removed game: %+v, %v", reloaded, err)
	}
}

func TestBatchPreservesSuccessBeforeLaterChecksumFailure(t *testing.T) {
	setupDLLTestDirs(t)
	installDirectory := t.TempDir()
	dlssPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
	dlssgPath := filepath.Join(installDirectory, "nvngx_dlssg.dll")
	assertWriteFile(t, dlssPath, []byte("old dlss"))
	assertWriteFile(t, dlssgPath, []byte("old dlssg"))
	entry := &game.Game{AppID: 1091500, Name: "fixture", InstallDir: installDirectory, DLLs: []game.DetectedDLL{
		{Name: filepath.Base(dlssPath), Path: dlssPath, Type: game.DLLTypeDLSS, Version: "1.0.0"},
		{Name: filepath.Base(dlssgPath), Path: dlssgPath, Type: game.DLLTypeDLSSG, Version: "1.0.0"},
	}}
	saveOperationDatabase(t, &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}})
	dlssPayload, dlssgPayload := []byte("new dlss"), []byte("new dlssg")
	saveOperationManifest(t, "fixture", map[string][]DLL{
		"dlss":  {{Version: "2.0.0", Filename: filepath.Base(dlssPath), SHA256: checksum(dlssPayload)}},
		"dlssg": {{Version: "2.0.0", Filename: filepath.Base(dlssgPath), SHA256: checksum(dlssgPayload)}},
	})
	for dllType, payload := range map[string][]byte{"dlss": dlssPayload, "dlssg": []byte("corrupt")} {
		cachePath := GetDLLCachePath(dllType, "2.0.0")
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			t.Fatal(err)
		}
		assertWriteFile(t, cachePath, payload)
	}
	batch := BatchUpdate([]UpdateRequest{
		{AppID: entry.AppID, DLLType: "dlss", Version: "2.0.0", InstalledPath: dlssPath, CachedOnly: true},
		{AppID: entry.AppID, DLLType: "dlssg", Version: "2.0.0", InstalledPath: dlssgPath, CachedOnly: true},
	}, nil)
	if batch.Updated != 1 || batch.Failed != 1 || batch.Items[0].Err != nil || batch.Items[1].Err == nil {
		t.Fatalf("BatchUpdate() = %+v", batch)
	}
	assertFileBytes(t, dlssPath, dlssPayload)
	assertFileBytes(t, dlssgPath, []byte("old dlssg"))
}

func TestMutationSetupAndContainmentFailures(t *testing.T) {
	t.Run("malformed database", func(t *testing.T) {
		setupDLLTestDirs(t)
		path := filepath.Join(os.Getenv("XDG_DATA_HOME"), "spela", "games.yaml")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Update(UpdateRequest{AppID: 1}, nil); err == nil || !strings.Contains(err.Error(), "load game database") {
			t.Fatalf("Update() error = %v", err)
		}
		batch := BatchUpdate([]UpdateRequest{{AppID: 1}, {AppID: 2}}, nil)
		if batch.Failed != 2 || len(batch.Items) != 2 {
			t.Fatalf("BatchUpdate() = %+v", batch)
		}
	})
	t.Run("runtime directory blocked", func(t *testing.T) {
		setupDLLTestDirs(t)
		blocked := filepath.Join(t.TempDir(), "blocked")
		assertWriteFile(t, blocked, nil)
		t.Setenv("XDG_RUNTIME_DIR", blocked)
		if _, err := Update(UpdateRequest{AppID: 1}, nil); err == nil {
			t.Fatal("operation lock accepted a file as its parent directory")
		}
	})
	t.Run("lock path is directory", func(t *testing.T) {
		setupDLLTestDirs(t)
		lockPath := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "spela", "games.lock")
		if err := os.MkdirAll(lockPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Update(UpdateRequest{AppID: 1}, nil); err == nil {
			t.Fatal("operation opened a directory as its lock file")
		}
	})
	if err := pathWithin("", "/tmp/outside"); err == nil {
		t.Fatal("empty install root was accepted")
	}
	if err := pathWithin(t.TempDir(), "/tmp/outside"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("outside target error = %v", err)
	}
	if cloneGame(nil) != nil {
		t.Fatal("nil game clone was non-nil")
	}
}

func TestOperationHelperFailureBoundaries(t *testing.T) {
	setupDLLTestDirs(t)
	sentinel := errors.New("sentinel")
	if !errors.Is(&PartialFailure{Err: sentinel}, sentinel) {
		t.Fatal("PartialFailure did not unwrap its cause")
	}
	if _, err := install(&game.Database{Games: map[uint64]*game.Game{}}, 1, "dlss", "latest", nil); err == nil {
		t.Fatal("install accepted a missing game")
	}
	manifestPath := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "spela", ManifestCacheFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	assertWriteFile(t, manifestPath, []byte("{"))
	if _, err := resolveManifestDLL("dlss", "latest"); err == nil || !strings.Contains(err.Error(), "load DLL manifest") {
		t.Fatalf("malformed manifest error = %v", err)
	}
	saveOperationManifest(t, "", map[string][]DLL{})
	if _, err := resolveManifestDLL("dlss", "missing"); err == nil || !strings.Contains(err.Error(), "available in manifest") {
		t.Fatalf("missing manifest version error = %v", err)
	}
	installDirectory := t.TempDir()
	outside := filepath.Join(t.TempDir(), "nvngx_dlss.dll")
	entry := &game.Game{InstallDir: installDirectory, DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: outside, Type: game.DLLTypeDLSS}}}
	if _, err := detectedDLL(entry, "dlss", outside); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("outside detected DLL error = %v", err)
	}
	if err := validateManifestEntry(&DLL{Filename: "nvngx_dlss.dll"}, "dlss"); err == nil || !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("missing manifest version error = %v", err)
	}
	if err := validateManifestChecksum(&DLL{Version: "1", SHA256: "bad"}, "dlss"); err == nil {
		t.Fatal("invalid checksum was accepted")
	}
	if _, _, err := installTarget(entry, "nvngx_dlss.dll"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("outside install target error = %v", err)
	}
	target, version, err := installTarget(&game.Game{InstallDir: installDirectory}, "new.dll")
	if err != nil || target != filepath.Join(installDirectory, "new.dll") || version != "" {
		t.Fatalf("new install target = %q, %q, %v", target, version, err)
	}
}

func operationGame(appID uint64, installDirectory, targetPath, version string) *game.Game {
	return &game.Game{AppID: appID, Name: fmt.Sprintf("game-%d", appID), InstallDir: installDirectory, DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: targetPath, Type: game.DLLTypeDLSS, Version: version}}}
}

func payloadServer(t *testing.T, payloads map[string][]byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		payload, ok := payloads[request.URL.Path]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server
}

func saveOperationManifest(t *testing.T, repository string, entries map[string][]DLL) {
	t.Helper()
	if err := SaveManifest(&Manifest{Version: "test", UpdatedAt: time.Now(), Repository: repository, DLLs: entries}); err != nil {
		t.Fatal(err)
	}
}

func saveOperationDatabase(t *testing.T, database *game.Database) {
	t.Helper()
	if _, err := game.Transaction(func(current *game.Database) (bool, error) {
		*current = *database
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func checksum(payload []byte) string {
	hash := sha256.Sum256(payload)
	return fmt.Sprintf("%x", hash)
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s bytes = %q, want %q", path, got, want)
	}
}

func assertWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsStages(got []Stage, want ...Stage) bool {
	for _, stage := range want {
		found := false
		for _, candidate := range got {
			if candidate == stage {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
