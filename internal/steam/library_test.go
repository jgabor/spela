package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSteamPathPrefersConfigured(t *testing.T) {
	const configured = "/opt/custom-steam"
	if got := ResolveSteamPath(configured); got != configured {
		t.Fatalf("ResolveSteamPath() = %q, want %q", got, configured)
	}
}

func TestLibrariesToScanIncludesAdditionalPaths(t *testing.T) {
	root := t.TempDir()
	steamPath := filepath.Join(root, "steam")
	if err := os.MkdirAll(filepath.Join(steamPath, "steamapps"), 0o755); err != nil {
		t.Fatalf("mkdir steamapps: %v", err)
	}
	writeLibraryFoldersVDF(t, steamPath)

	additional := filepath.Join(root, "extra-library")
	if err := os.MkdirAll(filepath.Join(additional, "steamapps"), 0o755); err != nil {
		t.Fatalf("mkdir additional steamapps: %v", err)
	}

	libraries, err := librariesToScan(steamPath, []string{additional, additional})
	if err != nil {
		t.Fatalf("librariesToScan() error = %v", err)
	}
	if len(libraries) != 2 {
		t.Fatalf("librariesToScan() len = %d, want 2", len(libraries))
	}
}

func writeLibraryFoldersVDF(t *testing.T, steamPath string) {
	t.Helper()
	content := `"libraryfolders"
{
	"0"
	{
		"path"		"` + steamPath + `"
	}
}`
	path := filepath.Join(steamPath, "steamapps", "libraryfolders.vdf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write libraryfolders.vdf: %v", err)
	}
}
