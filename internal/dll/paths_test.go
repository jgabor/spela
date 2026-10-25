package dll

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
)

func saveTestConfig(t *testing.T, change func(*config.Config)) {
	t.Helper()
	configuration := config.Default()
	change(configuration)
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestDLLCacheFollowsConfiguredPath(t *testing.T) {
	setupDLLTestDirs(t)
	if got, want := GetDLLCachePath("dlss", "2.0.0"), filepath.Join(os.Getenv("XDG_CACHE_HOME"), "spela", "dlls", "dlss", "2.0.0.dll"); got != want {
		t.Fatalf("default cache path = %q, want %q", got, want)
	}

	custom := t.TempDir()
	saveTestConfig(t, func(c *config.Config) { c.DLLCachePath = custom })
	if got, want := GetDLLCachePath("dlss", "2.0.0"), filepath.Join(custom, "dlss", "2.0.0.dll"); got != want {
		t.Fatalf("configured cache path = %q, want %q", got, want)
	}
	if err := os.MkdirAll(GetDLLCacheDir("dlss"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GetDLLCachePath("dlss", "2.0.0"), []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	if versions, err := ListCachedVersions("dlss"); err != nil || len(versions) != 1 || versions[0] != "2.0.0" {
		t.Fatalf("ListCachedVersions() = %v, %v; want [2.0.0]", versions, err)
	}

	saveTestConfig(t, func(c *config.Config) { c.DLLCachePath = "relative/dlls" })
	if got := GetDLLCachePath("dlss", "2.0.0"); !strings.HasPrefix(got, os.Getenv("XDG_CACHE_HOME")) {
		t.Fatalf("relative configured path was used: %q", got)
	}
}

func TestBackupPathChangeKeepsEarlierBackupsRestorable(t *testing.T) {
	setupDLLTestDirs(t)
	const appID = 1091500
	installDirectory := t.TempDir()
	targetPath := filepath.Join(installDirectory, "nvngx_dlss.dll")
	original := []byte("original-dll-bytes")
	if err := os.WriteFile(targetPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	dlls := []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: targetPath, Version: "3.7.0"}}

	first := t.TempDir()
	saveTestConfig(t, func(c *config.Config) { c.BackupPath = first })
	backup, err := CreateBackup(appID, "fixture", dlls)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	firstFile := backup.Files[0].BackupPath
	if !strings.HasPrefix(firstFile, first+string(filepath.Separator)) {
		t.Fatalf("backup file %q is not in configured folder %q", firstFile, first)
	}
	if strings.HasPrefix(GetBackupMetadataPath(appID), first) {
		t.Fatalf("backup metadata %q moved with the backup files", GetBackupMetadataPath(appID))
	}

	// The game now holds a swapped DLL, and the user picks another folder.
	if err := os.WriteFile(targetPath, []byte("swapped"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := t.TempDir()
	saveTestConfig(t, func(c *config.Config) { c.BackupPath = second })
	if !BackupExists(appID) {
		t.Fatal("earlier backup hidden after changing the backup path")
	}
	backup, err = CreateBackup(appID, "fixture", dlls)
	if err != nil {
		t.Fatalf("CreateBackup() after path change error = %v", err)
	}
	if len(backup.Files) != 1 || backup.Files[0].BackupPath != firstFile {
		t.Fatalf("backup files = %+v, want the earlier entry reused", backup.Files)
	}
	if entries, _ := os.ReadDir(GetBackupDir(appID)); len(entries) != 0 {
		t.Fatalf("swapped DLL was copied into the new folder: %v", entries)
	}
	if _, err := restoreBackup(appID, copyFile); err != nil {
		t.Fatalf("restoreBackup() error = %v", err)
	}
	assertFileBytes(t, targetPath, original)

	if err := os.Remove(firstFile); err != nil {
		t.Fatal(err)
	}
	if _, err := restoreBackup(appID, copyFile); err == nil || !strings.Contains(err.Error(), "cannot restore nvngx_dlss.dll") || !strings.Contains(err.Error(), filepath.Base(firstFile)) {
		t.Fatalf("restoreBackup() with missing backup file error = %v", err)
	}
}

func TestNewBackupFilesUseCurrentBackupPath(t *testing.T) {
	setupDLLTestDirs(t)
	installDirectory := t.TempDir()
	firstTarget := filepath.Join(installDirectory, "nvngx_dlss.dll")
	secondTarget := filepath.Join(installDirectory, "nvngx_dlssg.dll")
	for _, path := range []string{firstTarget, secondTarget} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	first, second := t.TempDir(), t.TempDir()
	saveTestConfig(t, func(c *config.Config) { c.BackupPath = first })
	if _, err := CreateBackup(1, "fixture", []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: firstTarget}}); err != nil {
		t.Fatal(err)
	}
	saveTestConfig(t, func(c *config.Config) { c.BackupPath = second })
	backup, err := CreateBackup(1, "fixture", []game.DetectedDLL{{Name: "nvngx_dlssg.dll", Path: secondTarget}})
	if err != nil {
		t.Fatal(err)
	}
	if len(backup.Files) != 2 || !strings.HasPrefix(backup.Files[0].BackupPath, first) || !strings.HasPrefix(backup.Files[1].BackupPath, second) {
		t.Fatalf("backup files = %+v, want one in each folder", backup.Files)
	}
}
