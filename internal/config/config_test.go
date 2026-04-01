package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.LogLevel != LogLevelInfo {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, LogLevelInfo)
	}
	if !cfg.CheckUpdates {
		t.Error("CheckUpdates = false, want true")
	}
	if !cfg.ShowHints {
		t.Error("ShowHints = false, want true")
	}
	if !cfg.RescanOnStartup {
		t.Error("RescanOnStartup = false, want true")
	}
	if cfg.ManifestRefreshHours != 24 {
		t.Errorf("ManifestRefreshHours = %d, want 24", cfg.ManifestRefreshHours)
	}
	if !cfg.ConfirmDestructive {
		t.Error("ConfirmDestructive = false, want true")
	}
}

func TestLoadMissingFile(t *testing.T) {
	setupTestEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	defaults := Default()
	if cfg.LogLevel != defaults.LogLevel {
		t.Errorf("LogLevel = %q, want default %q", cfg.LogLevel, defaults.LogLevel)
	}
	if cfg.ShowHints != defaults.ShowHints {
		t.Errorf("ShowHints = %v, want default %v", cfg.ShowHints, defaults.ShowHints)
	}
	if cfg.CheckUpdates != defaults.CheckUpdates {
		t.Errorf("CheckUpdates = %v, want default %v", cfg.CheckUpdates, defaults.CheckUpdates)
	}
}

func TestRoundtrip(t *testing.T) {
	setupTestEnv(t)

	original := &Config{
		LogLevel:               LogLevelDebug,
		ShaderCache:            "/tmp/shaders",
		CheckUpdates:           false,
		ShowHints:              false,
		RescanOnStartup:        false,
		AutoUpdateDLLs:         true,
		SteamPath:              "/opt/steam",
		AdditionalLibraryPaths: []string{"/mnt/games", "/mnt/ssd"},
		DLLCachePath:           "/tmp/dlls",
		BackupPath:             "/tmp/backups",
		DLLManifestURL:         "https://example.com/manifest.json",
		AutoRefreshManifest:    false,
		ManifestRefreshHours:   48,
		PreferredDLLSource:     "github",
		Theme:                  "dark",
		CompactMode:            true,
		ConfirmDestructive:     false,
	}

	if err := original.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.LogLevel != original.LogLevel {
		t.Errorf("LogLevel = %q, want %q", loaded.LogLevel, original.LogLevel)
	}
	if loaded.ShaderCache != original.ShaderCache {
		t.Errorf("ShaderCache = %q, want %q", loaded.ShaderCache, original.ShaderCache)
	}
	if loaded.CheckUpdates != original.CheckUpdates {
		t.Errorf("CheckUpdates = %v, want %v", loaded.CheckUpdates, original.CheckUpdates)
	}
	if loaded.ShowHints != original.ShowHints {
		t.Errorf("ShowHints = %v, want %v", loaded.ShowHints, original.ShowHints)
	}
	if loaded.RescanOnStartup != original.RescanOnStartup {
		t.Errorf("RescanOnStartup = %v, want %v", loaded.RescanOnStartup, original.RescanOnStartup)
	}
	if loaded.AutoUpdateDLLs != original.AutoUpdateDLLs {
		t.Errorf("AutoUpdateDLLs = %v, want %v", loaded.AutoUpdateDLLs, original.AutoUpdateDLLs)
	}
	if loaded.SteamPath != original.SteamPath {
		t.Errorf("SteamPath = %q, want %q", loaded.SteamPath, original.SteamPath)
	}
	if len(loaded.AdditionalLibraryPaths) != len(original.AdditionalLibraryPaths) {
		t.Fatalf("AdditionalLibraryPaths len = %d, want %d", len(loaded.AdditionalLibraryPaths), len(original.AdditionalLibraryPaths))
	}
	for i, p := range loaded.AdditionalLibraryPaths {
		if p != original.AdditionalLibraryPaths[i] {
			t.Errorf("AdditionalLibraryPaths[%d] = %q, want %q", i, p, original.AdditionalLibraryPaths[i])
		}
	}
	if loaded.DLLCachePath != original.DLLCachePath {
		t.Errorf("DLLCachePath = %q, want %q", loaded.DLLCachePath, original.DLLCachePath)
	}
	if loaded.BackupPath != original.BackupPath {
		t.Errorf("BackupPath = %q, want %q", loaded.BackupPath, original.BackupPath)
	}
	if loaded.DLLManifestURL != original.DLLManifestURL {
		t.Errorf("DLLManifestURL = %q, want %q", loaded.DLLManifestURL, original.DLLManifestURL)
	}
	if loaded.AutoRefreshManifest != original.AutoRefreshManifest {
		t.Errorf("AutoRefreshManifest = %v, want %v", loaded.AutoRefreshManifest, original.AutoRefreshManifest)
	}
	if loaded.ManifestRefreshHours != original.ManifestRefreshHours {
		t.Errorf("ManifestRefreshHours = %d, want %d", loaded.ManifestRefreshHours, original.ManifestRefreshHours)
	}
	if loaded.PreferredDLLSource != original.PreferredDLLSource {
		t.Errorf("PreferredDLLSource = %q, want %q", loaded.PreferredDLLSource, original.PreferredDLLSource)
	}
	if loaded.Theme != original.Theme {
		t.Errorf("Theme = %q, want %q", loaded.Theme, original.Theme)
	}
	if loaded.CompactMode != original.CompactMode {
		t.Errorf("CompactMode = %v, want %v", loaded.CompactMode, original.CompactMode)
	}
	if loaded.ConfirmDestructive != original.ConfirmDestructive {
		t.Errorf("ConfirmDestructive = %v, want %v", loaded.ConfirmDestructive, original.ConfirmDestructive)
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	setupTestEnv(t)

	configDir := filepath.Join(t.TempDir(), "spela")

	// Override XDG_CONFIG_HOME again to point to a dir we control for the write
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(configDir))

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	malformed := []byte("{{not: valid: yaml: [unterminated")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), malformed, 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want YAML parse error")
	}
}

func TestClone(t *testing.T) {
	original := &Config{
		LogLevel:               LogLevelDebug,
		SteamPath:              "/opt/steam",
		AdditionalLibraryPaths: []string{"/mnt/a", "/mnt/b"},
		ShowHints:              true,
	}

	clone := original.Clone()

	// Verify values match
	if clone.LogLevel != original.LogLevel {
		t.Errorf("Clone LogLevel = %q, want %q", clone.LogLevel, original.LogLevel)
	}
	if clone.SteamPath != original.SteamPath {
		t.Errorf("Clone SteamPath = %q, want %q", clone.SteamPath, original.SteamPath)
	}

	// Verify slice independence
	clone.AdditionalLibraryPaths[0] = "/modified"
	if original.AdditionalLibraryPaths[0] == "/modified" {
		t.Error("Clone shares slice with original — not a deep copy")
	}

	// Verify field independence
	clone.ShowHints = false
	if !original.ShowHints {
		t.Error("Clone mutation affected original — not a deep copy")
	}
}
