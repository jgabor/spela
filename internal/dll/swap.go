package dll

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jgabor/spela/internal/denylist"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/xdg"
)

type Backup struct {
	AppID      uint64         `json:"app_id"`
	GameName   string         `json:"game_name"`
	CreatedAt  time.Time      `json:"created_at"`
	BackupPath string         `json:"backup_path"`
	Files      []BackedUpFile `json:"files"`
}

type BackedUpFile struct {
	OriginalPath string `json:"original_path"`
	BackupPath   string `json:"backup_path"`
	DLLName      string `json:"dll_name"`
	Version      string `json:"version"`
}

func GetBackupDir(appID uint64) string {
	return xdg.DataPath(filepath.Join("backups", strconv.FormatUint(appID, 10)))
}

func GetBackupMetadataPath(appID uint64) string {
	return filepath.Join(GetBackupDir(appID), "backup.json")
}

func LoadBackup(appID uint64) (*Backup, error) {
	path := GetBackupMetadataPath(appID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var backup Backup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("parse backup metadata: %w", err)
	}
	if backup.AppID != appID {
		return nil, fmt.Errorf("invalid backup metadata: app ID is %d, want %d", backup.AppID, appID)
	}
	if len(backup.Files) == 0 {
		return nil, fmt.Errorf("invalid backup metadata: no files")
	}
	for index, file := range backup.Files {
		if file.OriginalPath == "" || file.BackupPath == "" {
			return nil, fmt.Errorf("invalid backup metadata: file %d has an empty path", index)
		}
	}

	return &backup, nil
}

func SaveBackup(backup *Backup) error {
	dir := GetBackupDir(backup.AppID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomically(GetBackupMetadataPath(backup.AppID), data, 0o644)
}

func BackupExists(appID uint64) bool {
	backup, err := LoadBackup(appID)
	return err == nil && backup != nil
}

func CreateBackup(appID uint64, gameName string, dlls []game.DetectedDLL) (*Backup, error) {
	if len(dlls) == 0 {
		return nil, fmt.Errorf("no DLLs to backup")
	}

	backupDir := GetBackupDir(appID)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	backup, err := LoadBackup(appID)
	if err != nil {
		return nil, fmt.Errorf("load existing backup metadata: %w", err)
	}
	if backup == nil {
		backup = &Backup{
			AppID:      appID,
			GameName:   gameName,
			CreatedAt:  time.Now(),
			BackupPath: backupDir,
		}
	}

	existing := make(map[string]BackedUpFile, len(backup.Files))
	for _, file := range backup.Files {
		existing[filepath.Clean(file.OriginalPath)] = file
		if err := preflightRegularFile(file.BackupPath); err != nil {
			return nil, fmt.Errorf("existing backup for %s is unreadable: %w", file.OriginalPath, err)
		}
	}
	for _, dll := range dlls {
		if _, ok := existing[filepath.Clean(dll.Path)]; ok {
			continue
		}
		if err := preflightRegularFile(dll.Path); err != nil {
			return nil, fmt.Errorf("cannot backup %s: %w", dll.Name, err)
		}
	}

	var created []string
	for _, dll := range dlls {
		if _, ok := existing[filepath.Clean(dll.Path)]; ok {
			continue
		}
		backupPath := uniqueBackupPath(backupDir, dll.Path)
		if _, err := os.Lstat(backupPath); err == nil {
			return nil, fmt.Errorf("backup destination already exists without metadata: %s", backupPath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect backup destination: %w", err)
		}

		if err := copyFile(dll.Path, backupPath); err != nil {
			removeFiles(created)
			return nil, fmt.Errorf("failed to backup %s: %w", dll.Name, err)
		}
		created = append(created, backupPath)

		backup.Files = append(backup.Files, BackedUpFile{
			OriginalPath: dll.Path,
			BackupPath:   backupPath,
			DLLName:      dll.Name,
			Version:      dll.Version,
		})
	}

	if err := SaveBackup(backup); err != nil {
		removeFiles(created)
		return nil, fmt.Errorf("failed to save backup metadata: %w", err)
	}

	return backup, nil
}

func restoreBackup(appID uint64, replace func(string, string) error) (int, error) {
	backup, err := LoadBackup(appID)
	if err != nil {
		return 0, err
	}
	if backup == nil {
		return 0, fmt.Errorf("no backup found for app %d", appID)
	}

	for _, file := range backup.Files {
		if err := preflightRegularFile(file.BackupPath); err != nil {
			return 0, fmt.Errorf("cannot restore %s: %w", file.DLLName, err)
		}
		parent, err := os.Stat(filepath.Dir(file.OriginalPath))
		if err != nil || !parent.IsDir() {
			return 0, fmt.Errorf("cannot restore %s: target directory is unavailable", file.DLLName)
		}
		if info, statErr := os.Stat(file.OriginalPath); statErr == nil && info.IsDir() {
			return 0, fmt.Errorf("cannot restore %s: target is a directory", file.DLLName)
		} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return 0, fmt.Errorf("cannot restore %s: %w", file.DLLName, statErr)
		}
	}

	changed := 0
	for _, file := range backup.Files {
		if err := replace(file.BackupPath, file.OriginalPath); err != nil {
			return changed, fmt.Errorf("failed to restore %s: %w", file.DLLName, err)
		}
		changed++
	}

	return changed, nil
}

func guardSwap(appID uint64) error {
	if denied, reason := denylist.IsDenied(appID); denied {
		return fmt.Errorf("DLL swap denied for app %d: %s", appID, reason)
	}
	return nil
}

func swapDLLAtPath(appID uint64, gameName string, dlls []game.DetectedDLL, targetPath, cachePath string) error {
	if err := guardSwap(appID); err != nil {
		return err
	}
	if targetPath == "" {
		return fmt.Errorf("DLL target path is required")
	}
	if _, err := CreateBackup(appID, gameName, dlls); err != nil {
		return fmt.Errorf("failed to create backup before swap: %w", err)
	}

	if err := copyFile(cachePath, targetPath); err != nil {
		return fmt.Errorf("failed to swap DLL: %w", err)
	}

	return nil
}

func installDLLAtPath(appID uint64, gameName string, dlls []game.DetectedDLL, targetPath, targetVersion, dllName, cachePath string) error {
	if err := guardSwap(appID); err != nil {
		return err
	}
	backupDLLs := append([]game.DetectedDLL(nil), dlls...)
	if _, err := os.Stat(targetPath); err == nil {
		found := false
		for _, candidate := range backupDLLs {
			found = found || filepath.Clean(candidate.Path) == filepath.Clean(targetPath)
		}
		if !found {
			backupDLLs = append(backupDLLs, game.DetectedDLL{Name: dllName, Path: targetPath, Version: targetVersion})
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect install target: %w", err)
	}
	if len(backupDLLs) > 0 {
		if _, err := CreateBackup(appID, gameName, backupDLLs); err != nil {
			return fmt.Errorf("failed to create backup before install: %w", err)
		}
	}

	if err := copyFile(cachePath, targetPath); err != nil {
		return fmt.Errorf("failed to install DLL: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	mode := fs.FileMode(0o644)
	if info, statErr := source.Stat(); statErr == nil {
		mode = info.Mode().Perm()
	}
	if info, statErr := os.Stat(dst); statErr == nil {
		mode = info.Mode().Perm()
	}
	return writeAtomically(dst, mode, func(destination io.Writer) error {
		_, err := io.Copy(destination, source)
		return err
	})
}

func writeFileAtomically(path string, data []byte, mode fs.FileMode) error {
	return writeAtomically(path, mode, func(destination io.Writer) error {
		_, err := destination.Write(data)
		return err
	})
}

func writeAtomically(path string, mode fs.FileMode, write func(io.Writer) error) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".spela-write-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := write(temporary); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func uniqueBackupPath(directory, originalPath string) string {
	digest := sha256.Sum256([]byte(filepath.Clean(originalPath)))
	return filepath.Join(directory, fmt.Sprintf("%x-%s", digest, filepath.Base(originalPath)))
}

func preflightRegularFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

func removeFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
