package dll

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/game"
)

func setupDLLTestDirs(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("XDG_RUNTIME_DIR", base)
}

func TestCreateBackupRestoreRoundTrip(t *testing.T) {
	setupDLLTestDirs(t)

	installDir := t.TempDir()
	originalPath := filepath.Join(installDir, "nvngx_dlss.dll")
	original := []byte("original-dll-bytes")
	if err := os.WriteFile(originalPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	gameDLLs := []game.DetectedDLL{{
		Name:    "nvngx_dlss.dll",
		Path:    originalPath,
		Version: "3.7.0",
	}}
	if _, err := CreateBackup(1091500, "Cyberpunk 2077", gameDLLs); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	if err := os.WriteFile(originalPath, []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := restoreBackup(1091500, copyFile); err != nil {
		t.Fatalf("restoreBackup() error = %v", err)
	}

	restored, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("restored bytes = %q, want %q", restored, original)
	}
}

func TestLegacyBasenameBackupMetadataRestores(t *testing.T) {
	setupDLLTestDirs(t)
	installDirectory := t.TempDir()
	targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
	backupDirectory := GetBackupDir(1091500)
	if err := os.MkdirAll(backupDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(backupDirectory, filepath.Base(targetPath))
	if err := os.WriteFile(targetPath, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveBackup(&Backup{AppID: 1091500, GameName: "fixture", BackupPath: backupDirectory, Files: []BackedUpFile{{OriginalPath: targetPath, BackupPath: legacyPath, DLLName: filepath.Base(targetPath)}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := restoreBackup(1091500, copyFile); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, targetPath, []byte("legacy original"))
}

func TestSwapDLLDenied(t *testing.T) {
	setupDLLTestDirs(t)

	installDir := t.TempDir()
	targetPath := filepath.Join(installDir, "nvngx_dlss.dll")
	if err := os.WriteFile(targetPath, []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(t.TempDir(), "cache.dll")
	if err := os.WriteFile(cachePath, []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := swapDLLAtPath(1172470, "Apex Legends", []game.DetectedDLL{{
		Name: "nvngx_dlss.dll",
		Path: targetPath,
	}}, targetPath, cachePath)
	if err == nil {
		t.Fatal("expected denylist error")
	}
	if got := err.Error(); !bytes.Contains([]byte(got), []byte("denied")) {
		t.Fatalf("error = %q, want deny message", got)
	}
}

func TestSwapDLLAllowedOverride(t *testing.T) {
	setupDLLTestDirs(t)

	if err := denylist.Allow(1172470); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}

	installDir := t.TempDir()
	targetPath := filepath.Join(installDir, "nvngx_dlss.dll")
	if err := os.WriteFile(targetPath, []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(t.TempDir(), "cache.dll")
	cached := []byte("cached-dll")
	if err := os.WriteFile(cachePath, cached, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := swapDLLAtPath(1172470, "Apex Legends", []game.DetectedDLL{{
		Name: "nvngx_dlss.dll",
		Path: targetPath,
	}}, targetPath, cachePath); err != nil {
		t.Fatalf("swapDLLAtPath() error = %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, cached) {
		t.Fatalf("swapped bytes = %q, want %q", got, cached)
	}
}

func TestInstallDLLBacksUpExistingTargetWithoutDetectedDLLs(t *testing.T) {
	setupDLLTestDirs(t)

	installDir := t.TempDir()
	targetPath := filepath.Join(installDir, "nvngx_dlss.dll")
	original := []byte("original-dll")
	if err := os.WriteFile(targetPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(t.TempDir(), "cache.dll")
	if err := os.WriteFile(cachePath, []byte("new-dll"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installDLLAtPath(1091500, "Cyberpunk 2077", nil, targetPath, "", "nvngx_dlss.dll", cachePath); err != nil {
		t.Fatalf("installDLLAtPath() error = %v", err)
	}
	if !BackupExists(1091500) {
		t.Fatal("expected backup metadata after install")
	}

	if _, err := restoreBackup(1091500, copyFile); err != nil {
		t.Fatalf("restoreBackup() error = %v", err)
	}
	restored, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("restored bytes = %q, want %q", restored, original)
	}
}

func TestCopyFileLeavesDestinationIntactOnSourceMissing(t *testing.T) {
	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "nvngx_dlss.dll")
	original := []byte("keep-me")
	if err := os.WriteFile(dstPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(filepath.Join(dstDir, "missing.dll"), dstPath); err == nil {
		t.Fatal("expected copy error for missing source")
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("destination mutated to %q", got)
	}
	if _, err := os.Stat(dstPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("expected no leftover temp file")
	}
}

func TestEnsureCachedReDownloadsCorruptCache(t *testing.T) {
	setupDLLTestDirs(t)

	payload := []byte("valid-dll-payload")
	hash := sha256.Sum256(payload)
	sha := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)

	entry := &DLL{
		Version: "3.8.10",
		URL:     server.URL,
		SHA256:  sha,
	}
	cachePath := GetDLLCachePath("dlss", entry.Version)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, err := acquireDLL(entry, "dlss", true, nil)
	if err != nil {
		t.Fatalf("acquireDLL() error = %v", err)
	}
	if gotPath != cachePath {
		t.Fatalf("cache path = %q, want %q", gotPath, cachePath)
	}

	got, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("cache payload = %q, want %q", got, payload)
	}
}

func TestEnsureCachedUsesValidCache(t *testing.T) {
	setupDLLTestDirs(t)

	payload := []byte("cached-valid")
	hash := sha256.Sum256(payload)
	sha := hex.EncodeToString(hash[:])

	entry := &DLL{
		Version: "3.8.10",
		URL:     "http://example.invalid/should-not-be-called",
		SHA256:  sha,
	}
	cachePath := GetDLLCachePath("dlss", entry.Version)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, err := acquireDLL(entry, "dlss", false, nil)
	if err != nil {
		t.Fatalf("acquireDLL() error = %v", err)
	}
	if gotPath != cachePath {
		t.Fatalf("cache path = %q, want %q", gotPath, cachePath)
	}
}

func TestCacheFailureBoundaries(t *testing.T) {
	t.Run("cache directory is file", func(t *testing.T) {
		setupDLLTestDirs(t)
		cacheDirectory := GetDLLCacheDir("dlss")
		if err := os.MkdirAll(filepath.Dir(cacheDirectory), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cacheDirectory, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ListCachedVersions("dlss"); err == nil {
			t.Fatal("ListCachedVersions accepted a file as a directory")
		}
	})
	t.Run("cached DLL is directory", func(t *testing.T) {
		setupDLLTestDirs(t)
		entry := &DLL{Version: "1", SHA256: checksum([]byte("valid"))}
		if err := os.MkdirAll(GetDLLCachePath("dlss", entry.Version), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := acquireDLL(entry, "dlss", false, nil); err == nil || !strings.Contains(err.Error(), "read cached DLL") {
			t.Fatalf("directory cache error = %v", err)
		}
	})
	t.Run("cache parent is file", func(t *testing.T) {
		setupDLLTestDirs(t)
		blocked := filepath.Join(t.TempDir(), "cache")
		if err := os.WriteFile(blocked, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XDG_CACHE_HOME", blocked)
		entry := &DLL{Version: "1", URL: "http://example.invalid", SHA256: checksum(nil)}
		if _, err := acquireDLL(entry, "dlss", true, nil); err == nil || !strings.Contains(err.Error(), "create cache directory") {
			t.Fatalf("blocked cache error = %v", err)
		}
	})
	t.Run("invalid download URL", func(t *testing.T) {
		setupDLLTestDirs(t)
		entry := &DLL{Version: "1", URL: "://invalid", SHA256: checksum(nil)}
		if _, err := acquireDLL(entry, "dlss", true, nil); err == nil || !strings.Contains(err.Error(), "download DLL") {
			t.Fatalf("invalid URL error = %v", err)
		}
	})
}

func TestBackupAndAtomicWriteFailureBoundaries(t *testing.T) {
	t.Run("backup source missing or nonregular", func(t *testing.T) {
		setupDLLTestDirs(t)
		for index, source := range []string{filepath.Join(t.TempDir(), "missing.dll"), t.TempDir()} {
			_, err := CreateBackup(uint64(index+1), "game", []game.DetectedDLL{{Name: "test.dll", Path: source}})
			if err == nil || !strings.Contains(err.Error(), "cannot backup") {
				t.Fatalf("CreateBackup(%q) error = %v", source, err)
			}
		}
	})
	t.Run("existing backup file missing", func(t *testing.T) {
		setupDLLTestDirs(t)
		const appID = 10
		missing := filepath.Join(t.TempDir(), "missing-backup.dll")
		if err := SaveBackup(&Backup{AppID: appID, Files: []BackedUpFile{{OriginalPath: "/game/test.dll", BackupPath: missing, DLLName: "test.dll"}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateBackup(appID, "game", []game.DetectedDLL{{Name: "test.dll", Path: "/game/test.dll"}}); err == nil || !strings.Contains(err.Error(), "existing backup") {
			t.Fatalf("CreateBackup() error = %v", err)
		}
	})
	t.Run("orphan backup destination", func(t *testing.T) {
		setupDLLTestDirs(t)
		const appID = 11
		source := filepath.Join(t.TempDir(), "test.dll")
		if err := os.WriteFile(source, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(GetBackupDir(appID), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(uniqueBackupPath(GetBackupDir(appID), source), []byte("orphan"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateBackup(appID, "game", []game.DetectedDLL{{Name: "test.dll", Path: source}}); err == nil || !strings.Contains(err.Error(), "without metadata") {
			t.Fatalf("CreateBackup() error = %v", err)
		}
	})
	t.Run("restore preflight", func(t *testing.T) {
		setupDLLTestDirs(t)
		const appID = 12
		backupPath := filepath.Join(t.TempDir(), "backup.dll")
		if err := os.WriteFile(backupPath, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name, original, backup, want string
		}{
			{"missing backup", filepath.Join(t.TempDir(), "game.dll"), filepath.Join(t.TempDir(), "missing.dll"), "cannot restore"},
			{"missing target parent", filepath.Join(t.TempDir(), "missing", "game.dll"), backupPath, "target directory is unavailable"},
			{"target directory", t.TempDir(), backupPath, "target is a directory"},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				if err := SaveBackup(&Backup{AppID: appID, Files: []BackedUpFile{{OriginalPath: test.original, BackupPath: test.backup, DLLName: "test.dll"}}}); err != nil {
					t.Fatal(err)
				}
				if _, err := restoreBackup(appID, copyFile); err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("restoreBackup() error = %v", err)
				}
			})
		}
	})
	t.Run("atomic writer callback and close errors", func(t *testing.T) {
		directory := t.TempDir()
		sentinel := errors.New("write failed")
		if err := writeAtomically(filepath.Join(directory, "callback"), 0o644, func(io.Writer) error { return sentinel }); !errors.Is(err, sentinel) {
			t.Fatalf("callback error = %v", err)
		}
		if err := writeAtomically(filepath.Join(directory, "closed"), 0o644, func(writer io.Writer) error { return writer.(*os.File).Close() }); err == nil {
			t.Fatal("writeAtomically accepted a closed temporary file")
		}
		if err := writeAtomically(filepath.Join(directory, "missing", "file"), 0o644, func(io.Writer) error { return nil }); err == nil {
			t.Fatal("writeAtomically accepted a missing parent directory")
		}
	})
}

func TestSwapInstallAndRestoreFailureBoundaries(t *testing.T) {
	setupDLLTestDirs(t)
	if err := swapDLLAtPath(1, "game", nil, "", "missing-cache"); err == nil || !strings.Contains(err.Error(), "target path is required") {
		t.Fatalf("empty swap target error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "game.dll")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := swapDLLAtPath(2, "game", []game.DetectedDLL{{Name: "game.dll", Path: target}}, target, filepath.Join(t.TempDir(), "missing.dll")); err == nil || !strings.Contains(err.Error(), "failed to swap") {
		t.Fatalf("missing swap source error = %v", err)
	}
	if err := installDLLAtPath(3, "game", nil, filepath.Join(t.TempDir(), "new.dll"), "", "new.dll", filepath.Join(t.TempDir(), "missing.dll")); err == nil || !strings.Contains(err.Error(), "failed to install") {
		t.Fatalf("missing install source error = %v", err)
	}
	if err := installDLLAtPath(1172470, "Apex Legends", nil, target, "", "game.dll", target); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("denied install error = %v", err)
	}
	const malformedAppID = 4
	if err := os.MkdirAll(GetBackupDir(malformedAppID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GetBackupMetadataPath(malformedAppID), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := restoreBackup(malformedAppID, copyFile); err == nil {
		t.Fatal("restore accepted malformed backup metadata")
	}
	temporary := filepath.Join(t.TempDir(), "temporary")
	if err := os.WriteFile(temporary, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	removeFiles([]string{temporary, filepath.Join(t.TempDir(), "missing")})
	if _, err := os.Stat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed temporary file stat error = %v", err)
	}
}

func TestBackupMetadataAndPathPrimitiveValidation(t *testing.T) {
	setupDLLTestDirs(t)
	if _, err := CreateBackup(1, "fixture", nil); err == nil {
		t.Fatal("empty backup was accepted")
	}
	backupDirectory := GetBackupDir(1)
	if err := os.MkdirAll(backupDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []string{
		`{"app_id":2,"files":[{"original_path":"/a","backup_path":"/b"}]}`,
		`{"app_id":1,"files":[]}`,
		`{"app_id":1,"files":[{"original_path":"","backup_path":"/b"}]}`,
	} {
		if err := os.WriteFile(GetBackupMetadataPath(1), []byte(metadata), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadBackup(1); err == nil {
			t.Fatalf("invalid metadata was accepted: %s", metadata)
		}
	}
}
