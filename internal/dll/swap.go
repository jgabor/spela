package dll

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/jgabor/spela/internal/xdg"
)

type Backup struct {
	AppID      uint64            `json:"app_id"`
	GameName   string            `json:"game_name"`
	CreatedAt  time.Time         `json:"created_at"`
	BackupPath string            `json:"backup_path"`
	Files      []BackedUpFile    `json:"files"`
}

type BackedUpFile struct {
	OriginalPath string `json:"original_path"`
	BackupPath   string `json:"backup_path"`
	DLLName      string `json:"dll_name"`
	Version      string `json:"version"`
}

func GetBackupDir(appID uint64) string {
	return xdg.DataPath(filepath.Join("backups", fmt.Sprintf("%d", appID)))
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
		return nil, err
	}

	return &backup, nil
}

func SaveBackup(backup *Backup) error {
	dir := GetBackupDir(backup.AppID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(GetBackupMetadataPath(backup.AppID), data, 0644)
}

func BackupExists(appID uint64) bool {
	backup, _ := LoadBackup(appID)
	return backup != nil
}

type GameDLL struct {
	Name    string
	Path    string
	Version string
}

func CreateBackup(appID uint64, gameName string, dlls []GameDLL) (*Backup, error) {
	if len(dlls) == 0 {
		return nil, fmt.Errorf("no DLLs to backup")
	}

	backupDir := GetBackupDir(appID)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	backup := &Backup{
		AppID:      appID,
		GameName:   gameName,
		CreatedAt:  time.Now(),
		BackupPath: backupDir,
	}

	for _, dll := range dlls {
		backupPath := filepath.Join(backupDir, filepath.Base(dll.Path))

		if err := copyFile(dll.Path, backupPath); err != nil {
			return nil, fmt.Errorf("failed to backup %s: %w", dll.Name, err)
		}

		backup.Files = append(backup.Files, BackedUpFile{
			OriginalPath: dll.Path,
			BackupPath:   backupPath,
			DLLName:      dll.Name,
			Version:      dll.Version,
		})
	}

	if err := SaveBackup(backup); err != nil {
		return nil, fmt.Errorf("failed to save backup metadata: %w", err)
	}

	return backup, nil
}

func RestoreBackup(appID uint64) error {
	backup, err := LoadBackup(appID)
	if err != nil {
		return err
	}
	if backup == nil {
		return fmt.Errorf("no backup found for app %d", appID)
	}

	for _, file := range backup.Files {
		if err := copyFile(file.BackupPath, file.OriginalPath); err != nil {
			return fmt.Errorf("failed to restore %s: %w", file.DLLName, err)
		}
	}

	return nil
}

func DeleteBackup(appID uint64) error {
	backupDir := GetBackupDir(appID)
	return os.RemoveAll(backupDir)
}

func SwapDLL(appID uint64, gameName string, dlls []GameDLL, dllName, cachePath string) error {
	var targetPath string
	for _, dll := range dlls {
		if dll.Name == dllName {
			targetPath = dll.Path
			break
		}
	}

	if targetPath == "" {
		return fmt.Errorf("DLL %s not found in game", dllName)
	}

	if !BackupExists(appID) {
		if _, err := CreateBackup(appID, gameName, dlls); err != nil {
			return fmt.Errorf("failed to create backup before swap: %w", err)
		}
	}

	if err := copyFile(cachePath, targetPath); err != nil {
		return fmt.Errorf("failed to swap DLL: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
