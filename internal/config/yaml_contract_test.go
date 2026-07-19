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
		LogLevel: "debug", ShaderCache: "/cache", CheckUpdates: true, ShowHints: true,
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
show_hints: true
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
	configuration.ShowHints = false
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
		"check_updates: false", "show_hints: false", "rescan_on_startup: false",
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
	if loaded.CheckUpdates || loaded.ShowHints || loaded.RescanOnStartup || loaded.AutoRefreshManifest || loaded.ManifestRefreshHours != 0 || loaded.ConfirmDestructive {
		t.Fatalf("explicit zero values did not round trip: %+v", loaded)
	}
	if loaded.AdditionalLibraryPaths != nil {
		t.Fatalf("omitted empty list loaded as %#v, want nil", loaded.AdditionalLibraryPaths)
	}
}
