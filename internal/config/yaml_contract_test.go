package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigYAMLContract(t *testing.T) {
	setupTestEnv(t)
	configuration := &Config{
		LogLevel: "debug", ShaderCache: "/cache", CheckUpdates: true,
		RescanOnStartup: true, AutoUpdateDLLs: true, SteamPath: "/steam",
		AdditionalLibraryPaths: []string{"/games"}, DLLCachePath: "/dlls", BackupPath: "/backups",
		DLLManifestURL: "https://example.test/manifest.json", AutoRefreshManifest: true,
		ManifestRefreshHours: 12, PreferredDLLSource: "github", Theme: "dark",
		CompactMode: true, ConfirmDestructive: true,
	}
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `log_level: debug
shader_cache: /cache
check_updates: true
rescan_on_startup: true
auto_update_dlls: true
steam_path: /steam
additional_library_paths:
    - /games
dll_cache_path: /dlls
backup_path: /backups
dll_manifest_url: https://example.test/manifest.json
auto_refresh_manifest: true
manifest_refresh_hours: 12
preferred_dll_source: github
theme: dark
compact_mode: true
confirm_destructive: true
`
	if string(data) != want {
		t.Fatalf("persisted config contract changed (-want +got):\nwant:\n%s\ngot:\n%s", want, data)
	}
	if strings.Contains(string(data), "LogLevel") {
		t.Fatal("persisted config must use snake_case YAML keys")
	}
}

func TestConfigYAMLExplicitZeroContract(t *testing.T) {
	setupTestEnv(t)
	configuration := Default()
	configuration.CheckUpdates = false
	configuration.RescanOnStartup = false
	configuration.AutoRefreshManifest = false
	configuration.ManifestRefreshHours = 0
	configuration.ConfirmDestructive = false
	configuration.AdditionalLibraryPaths = []string{}
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"check_updates: false", "rescan_on_startup: false",
		"auto_refresh_manifest: false", "manifest_refresh_hours: 0", "confirm_destructive: false",
	} {
		if !strings.Contains(string(data), line+"\n") {
			t.Errorf("explicit zero line %q missing from:\n%s", line, data)
		}
	}
	if strings.Contains(string(data), "additional_library_paths:") {
		t.Fatalf("explicit empty list must remain omitted under omitempty compatibility:\n%s", data)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CheckUpdates || loaded.RescanOnStartup || loaded.AutoRefreshManifest || loaded.ManifestRefreshHours != 0 || loaded.ConfirmDestructive {
		t.Fatalf("explicit zero values did not round trip: %+v", loaded)
	}
	if loaded.AdditionalLibraryPaths != nil {
		t.Fatalf("omitted empty list loaded as %#v, want nil", loaded.AdditionalLibraryPaths)
	}
}

// The retired show_hints key may still sit in an existing config.yaml. It must
// load without error, and saving the configuration drops it.
func TestConfigLoadIgnoresRetiredShowHintsKey(t *testing.T) {
	setupTestEnv(t)
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("log_level: debug\nshow_hints: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("a config with the retired show_hints key must load: %v", err)
	}
	if loaded.LogLevel != "debug" {
		t.Errorf("other keys must still load, log level = %q", loaded.LogLevel)
	}
	if err := loaded.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "show_hints") {
		t.Errorf("saving should drop the retired key:\n%s", data)
	}
}
