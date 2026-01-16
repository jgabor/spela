package ludusavi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type SaveInfo struct {
	Name     string   `json:"name"`
	Paths    []string `json:"paths"`
	Registry []string `json:"registry,omitempty"`
	Size     int64    `json:"size"`
	Backups  int      `json:"backups"`
}

type BackupResult struct {
	Overall struct {
		TotalGames     int   `json:"totalGames"`
		TotalBytes     int64 `json:"totalBytes"`
		ProcessedGames int   `json:"processedGames"`
		ProcessedBytes int64 `json:"processedBytes"`
	} `json:"overall"`
	Games map[string]GameBackupResult `json:"games"`
}

type GameBackupResult struct {
	Decision string                      `json:"decision"`
	Change   string                      `json:"change"`
	Files    map[string]FileBackupResult `json:"files"`
}

type FileBackupResult struct {
	Change string `json:"change"`
	Bytes  int64  `json:"bytes"`
}

func IsInstalled() bool {
	_, err := exec.LookPath("ludusavi")
	return err == nil
}

func GetVersion() (string, error) {
	cmd := exec.Command("ludusavi", "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func FindGame(gameName string) (*SaveInfo, error) {
	cmd := exec.Command("ludusavi", "find", "--api", gameName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ludusavi find failed: %s", stderr.String())
	}

	var result map[string]SaveInfo
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ludusavi output: %w", err)
	}

	for name, info := range result {
		info.Name = name
		return &info, nil
	}

	return nil, nil
}

func BackupGame(gameName string) (*BackupResult, error) {
	cmd := exec.Command("ludusavi", "backup", "--api", "--force", gameName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ludusavi backup failed: %s", stderr.String())
	}

	var result BackupResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ludusavi output: %w", err)
	}

	return &result, nil
}

func RestoreGame(gameName string) (*BackupResult, error) {
	cmd := exec.Command("ludusavi", "restore", "--api", "--force", gameName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ludusavi restore failed: %s", stderr.String())
	}

	var result BackupResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ludusavi output: %w", err)
	}

	return &result, nil
}

func ListBackups() ([]string, error) {
	cmd := exec.Command("ludusavi", "backups", "--api")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ludusavi backups failed: %s", stderr.String())
	}

	var games []string
	if err := json.Unmarshal(stdout.Bytes(), &games); err != nil {
		return nil, fmt.Errorf("failed to parse ludusavi output: %w", err)
	}

	return games, nil
}
