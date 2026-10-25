package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfiguredFoldersFallBackToXDGDefaults(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	defaults := map[string]string{
		"dll":    filepath.Join(root, "cache", "spela", "dlls"),
		"backup": filepath.Join(root, "data", "spela", "backups"),
		"shader": filepath.Join(root, "cache", "spela", "nvidia"),
	}
	check := func(name string, want map[string]string) {
		t.Helper()
		got := map[string]string{"dll": DLLCacheDir(), "backup": BackupFilesDir(), "shader": ShaderCacheDir()}
		for key := range want {
			if got[key] != want[key] {
				t.Errorf("%s: %s folder = %q, want %q", name, key, got[key], want[key])
			}
		}
	}
	check("missing config", defaults)

	configuration := Default()
	configuration.DLLCachePath, configuration.BackupPath, configuration.ShaderCache = "/custom/dlls/", "/custom/backups", "/custom/shaders"
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	check("configured", map[string]string{"dll": "/custom/dlls", "backup": "/custom/backups", "shader": "/custom/shaders"})
	if DefaultBackupDir() != defaults["backup"] {
		t.Errorf("DefaultBackupDir() = %q, want %q", DefaultBackupDir(), defaults["backup"])
	}

	configuration.DLLCachePath, configuration.BackupPath, configuration.ShaderCache = "dlls", "~/backups", ""
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	check("relative or empty", defaults)

	if err := os.WriteFile(filepath.Join(root, "config", "spela", "config.yaml"), []byte("dll_cache_path: [broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	check("unreadable config", defaults)
}

func TestValidateManifestURLAllowsHTTPSAndLoopbackHTTP(t *testing.T) {
	for _, valid := range []string{"https://example.test/manifest.json", "http://127.0.0.1:8080/m.json", "http://localhost/m.json", "http://[::1]/m.json"} {
		if err := ValidateManifestURL(valid); err != nil {
			t.Errorf("ValidateManifestURL(%q) = %v", valid, err)
		}
	}
	for _, invalid := range []string{"http://example.test/m.json", "ftp://example.test/m.json", "example.test/m.json", "https://", "file:///tmp/m.json"} {
		if err := ValidateManifestURL(invalid); err == nil {
			t.Errorf("ValidateManifestURL(%q) accepted", invalid)
		}
	}
}
