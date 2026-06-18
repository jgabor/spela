package dll

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jgabor/spela/internal/denylist"
)

func setupDLLTestDirs(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv("XDG_CONFIG_HOME", base)
}

func TestCreateBackupRestoreRoundTrip(t *testing.T) {
	setupDLLTestDirs(t)

	installDir := t.TempDir()
	originalPath := filepath.Join(installDir, "nvngx_dlss.dll")
	original := []byte("original-dll-bytes")
	if err := os.WriteFile(originalPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	gameDLLs := []GameDLL{{
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

	if err := RestoreBackup(1091500); err != nil {
		t.Fatalf("RestoreBackup() error = %v", err)
	}

	restored, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("restored bytes = %q, want %q", restored, original)
	}
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

	err := SwapDLL(1172470, "Apex Legends", []GameDLL{{
		Name: "nvngx_dlss.dll",
		Path: targetPath,
	}}, "nvngx_dlss.dll", cachePath)
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

	if err := SwapDLL(1172470, "Apex Legends", []GameDLL{{
		Name: "nvngx_dlss.dll",
		Path: targetPath,
	}}, "nvngx_dlss.dll", cachePath); err != nil {
		t.Fatalf("SwapDLL() error = %v", err)
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

	if err := InstallDLL(1091500, "Cyberpunk 2077", installDir, nil, "nvngx_dlss.dll", cachePath); err != nil {
		t.Fatalf("InstallDLL() error = %v", err)
	}
	if !BackupExists(1091500) {
		t.Fatal("expected backup metadata after install")
	}

	if err := RestoreBackup(1091500); err != nil {
		t.Fatalf("RestoreBackup() error = %v", err)
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

	gotPath, err := EnsureCached(entry, "dlss")
	if err != nil {
		t.Fatalf("EnsureCached() error = %v", err)
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

	gotPath, err := EnsureCached(entry, "dlss")
	if err != nil {
		t.Fatalf("EnsureCached() error = %v", err)
	}
	if gotPath != cachePath {
		t.Fatalf("cache path = %q, want %q", gotPath, cachePath)
	}
}
