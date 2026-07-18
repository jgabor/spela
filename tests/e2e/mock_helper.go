//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestEnvironment represents the paths and environment configuration for an isolated test run.
type TestEnvironment struct {
	TempDir    string
	ConfigHome string
	DataHome   string
	CacheHome  string
	Env        []string
}

// SetupTestEnvironment creates a temporary directory and writes isolated config/profile/database/manifest files.
func SetupTestEnvironment(t *testing.T) (*TestEnvironment, func()) {
	t.Helper()

	// 1. Create a parent test_run_data folder in the worktree if it does not exist
	baseDir, err := filepath.Abs(filepath.Join(".", "test_run_data"))
	if err != nil {
		t.Fatalf("failed to get absolute path for test_run_data base directory: %v", err)
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("failed to create test_run_data base directory: %v", err)
	}

	// 2. Create an isolated subdirectory for this specific test
	tempDir, err := os.MkdirTemp(baseDir, "run-*")
	if err != nil {
		t.Fatalf("failed to create temporary test run directory: %v", err)
	}

	configHome := filepath.Join(tempDir, "config")
	dataHome := filepath.Join(tempDir, "data")
	cacheHome := filepath.Join(tempDir, "cache")

	// Ensure subdirectories exist
	runtimeDir := filepath.Join(tempDir, "runtime")
	for _, dir := range []string{configHome, dataHome, cacheHome, runtimeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create XDG subdirectory %q: %v", dir, err)
		}
	}

	// Build isolated environment variables list
	env := []string{
		"XDG_RUNTIME_DIR=" + filepath.Join(tempDir, "runtime"),
		"XDG_CONFIG_HOME=" + configHome,
		"XDG_DATA_HOME=" + dataHome,
		"XDG_CACHE_HOME=" + cacheHome,
	}

	te := &TestEnvironment{
		TempDir:    tempDir,
		ConfigHome: configHome,
		DataHome:   dataHome,
		CacheHome:  cacheHome,
		Env:        env,
	}

	// 3. Write mock game database (games.yaml)
	cyberpunkDir := filepath.Join(tempDir, "games", "Cyberpunk 2077")
	witcherDir := filepath.Join(tempDir, "games", "The Witcher 3")
	eldenRingDir := filepath.Join(tempDir, "games", "ELDEN RING")

	// Create directories for DLL layout
	if err := os.MkdirAll(filepath.Join(cyberpunkDir, "bin", "x64"), 0o755); err != nil {
		t.Fatalf("failed to create cyberpunk bin dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(witcherDir, "bin"), 0o755); err != nil {
		t.Fatalf("failed to create witcher bin dir: %v", err)
	}
	if err := os.MkdirAll(eldenRingDir, 0o755); err != nil {
		t.Fatalf("failed to create elden ring dir: %v", err)
	}

	// Write mock dummy DLLs
	writeDummyFile(t, filepath.Join(cyberpunkDir, "bin", "x64", "nvngx_dlss.dll"), "DLSS 3.7.0")
	writeDummyFile(t, filepath.Join(cyberpunkDir, "bin", "x64", "nvngx_dlssg.dll"), "DLSSG 3.7.0")
	writeDummyFile(t, filepath.Join(witcherDir, "bin", "nvngx_dlss.dll"), "DLSS 3.5.0")

	// Write games.yaml database
	dbPath := filepath.Join(dataHome, "spela", "games.yaml")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("failed to create spela data dir: %v", err)
	}

	gamesData := map[string]interface{}{
		"games": map[uint64]interface{}{
			1091500: map[string]interface{}{
				"app_id":       1091500,
				"name":         "Cyberpunk 2077",
				"install_dir":  cyberpunkDir,
				"prefix_path":  filepath.Join(tempDir, "compatdata", "1091500", "pfx"),
				"scanned_at":   time.Now().Format(time.RFC3339),
				"library_path": filepath.Join(tempDir, "games"),
				"dlls": []map[string]interface{}{
					{
						"path":    filepath.Join(cyberpunkDir, "bin", "x64", "nvngx_dlss.dll"),
						"name":    "nvngx_dlss.dll",
						"type":    "dlss",
						"version": "3.7.0",
					},
					{
						"path":    filepath.Join(cyberpunkDir, "bin", "x64", "nvngx_dlssg.dll"),
						"name":    "nvngx_dlssg.dll",
						"type":    "dlssg",
						"version": "3.7.0",
					},
				},
			},
			292030: map[string]interface{}{
				"app_id":       292030,
				"name":         "The Witcher 3: Wild Hunt",
				"install_dir":  witcherDir,
				"prefix_path":  filepath.Join(tempDir, "compatdata", "292030", "pfx"),
				"scanned_at":   time.Now().Format(time.RFC3339),
				"library_path": filepath.Join(tempDir, "games"),
				"dlls": []map[string]interface{}{
					{
						"path":    filepath.Join(witcherDir, "bin", "nvngx_dlss.dll"),
						"name":    "nvngx_dlss.dll",
						"type":    "dlss",
						"version": "3.5.0",
					},
				},
			},
			1245620: map[string]interface{}{
				"app_id":       1245620,
				"name":         "Elden Ring",
				"install_dir":  eldenRingDir,
				"prefix_path":  filepath.Join(tempDir, "compatdata", "1245620", "pfx"),
				"scanned_at":   time.Now().Format(time.RFC3339),
				"library_path": filepath.Join(tempDir, "games"),
				"dlls":         []interface{}{},
			},
		},
		"updated_at": time.Now().Format(time.RFC3339),
	}

	writeYAML(t, dbPath, gamesData)

	// 4. Write mock configuration (config.yaml)
	configPath := filepath.Join(configHome, "spela", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("failed to create spela config dir: %v", err)
	}

	configData := map[string]interface{}{
		"log_level":              "info",
		"shader_cache":           filepath.Join(tempDir, "nvidia"),
		"check_updates":          false,
		"show_hints":             true,
		"rescan_on_startup":      false,
		"auto_update_dlls":       false,
		"auto_refresh_manifest":  false,
		"manifest_refresh_hours": 24,
		"preferred_dll_source":   "techpowerup",
		"compact_mode":           false,
		"confirm_destructive":    true,
	}

	writeYAML(t, configPath, configData)

	// 5. Write default profile (profiles/default.yaml)
	profilesDir := filepath.Join(configHome, "spela", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}

	defaultProfilePath := filepath.Join(profilesDir, "default.yaml")
	defaultProfileData := map[string]interface{}{
		"dlss": map[string]interface{}{
			"sr_mode":     "balanced",
			"sr_preset":   "balanced",
			"sr_override": false,
			"fg_enabled":  false,
			"fg_override": false,
		},
		"proton": map[string]interface{}{
			"enable_hdr":         false,
			"enable_wayland":     false,
			"enable_ngx_updater": false,
		},
	}

	writeYAML(t, defaultProfilePath, defaultProfileData)

	// 6. Write Cyberpunk 2077 game profile (profiles/1091500.yaml)
	cyberpunkProfilePath := filepath.Join(profilesDir, "1091500.yaml")
	cyberpunkProfileData := map[string]interface{}{
		"dlss": map[string]interface{}{
			"sr_mode":     "balanced",
			"sr_preset":   "quality",
			"sr_override": false,
			"fg_enabled":  true,
			"fg_override": true,
		},
		"proton": map[string]interface{}{
			"enable_hdr":         true,
			"enable_wayland":     false,
			"enable_ngx_updater": false,
		},
		"overrides": map[string]bool{
			"dlss.sr_mode":              true,
			"dlss.sr_preset":            true,
			"dlss.sr_override":          true,
			"dlss.fg_enabled":           true,
			"dlss.fg_override":          true,
			"proton.enable_hdr":         true,
			"proton.enable_wayland":     true,
			"proton.enable_ngx_updater": true,
		},
	}

	writeYAML(t, cyberpunkProfilePath, cyberpunkProfileData)

	// 7. Write mock DLL manifest JSON (manifest.json)
	manifestPath := filepath.Join(cacheHome, "spela", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("failed to create spela cache dir: %v", err)
	}

	manifestUpdatedAt := time.Now().Format(time.RFC3339)
	manifestJSON := fmt.Sprintf(`{
  "version": "1.0",
  "updated_at": "%s",
  "repository": "jgabor/spela",
  "dlls": {
    "dlss": [
      {
        "version": "3.8.0",
        "filename": "nvngx_dlss_3.8.0.dll",
        "url": "http://localhost:12345/dlls/nvngx_dlss_3.8.0.dll",
        "sha256": "abcdef1234567890",
        "size": 1024,
        "release_date": "2026-05-22T21:00:00Z"
      }
    ],
    "dlssg": [
      {
        "version": "3.7.0",
        "filename": "nvngx_dlssg_3.7.0.dll",
        "url": "http://localhost:12345/dlls/nvngx_dlssg_3.7.0.dll",
        "sha256": "abcdef1234567890",
        "size": 1024,
        "release_date": "2026-05-22T21:00:00Z"
      }
    ]
  }
}`, manifestUpdatedAt)

	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0o644); err != nil {
		t.Fatalf("failed to write manifest JSON: %v", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return te, cleanup
}

func writeYAML(t *testing.T, path string, data interface{}) {
	t.Helper()
	bytes, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal YAML for path %s: %v", path, err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatalf("failed to write YAML file %s: %v", path, err)
	}
}

func writeDummyFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write dummy file %s: %v", path, err)
	}
}
