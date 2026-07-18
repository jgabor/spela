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

func TestSteamLibraryDiscoveryAndScanSupportedLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	steamRoot := filepath.Join(home, ".local", "share", "Steam")
	library := filepath.Join(home, "Games", "SteamLibrary")
	if err := os.MkdirAll(filepath.Join(steamRoot, "steamapps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindSteamPath(); got != steamRoot {
		t.Fatalf("FindSteamPath = %q, want %q", got, steamRoot)
	}
	libraryVDF := `"libraryfolders"
{
  "0"
  {
    "path" "` + steamRoot + `"
    "label" "main"
    "apps"
    {
      "1091500" "700"
    }
  }
  "1"
  {
    "path" "` + library + `"
    "label" "games"
  }
  "ignored" "value"
}`
	if err := os.WriteFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"), []byte(libraryVDF), 0o644); err != nil {
		t.Fatal(err)
	}
	libraries, err := GetLibraries(steamRoot)
	byLabel := make(map[string]Library, len(libraries))
	for _, item := range libraries {
		byLabel[item.Label] = item
	}
	if err != nil || len(libraries) != 2 || byLabel["main"].Apps["1091500"] != "700" || byLabel["games"].Path != library {
		t.Fatalf("GetLibraries = %+v, %v", libraries, err)
	}

	steamapps := filepath.Join(library, "steamapps")
	gameDirectory := filepath.Join(steamapps, "common", "Cyberpunk 2077")
	prefixDrive := filepath.Join(steamapps, "compatdata", "1091500", "pfx", "drive_c")
	if err := os.MkdirAll(gameDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(prefixDrive, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDirectory, "nvngx_dlss.dll"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, manifest := range map[string]string{
		"appmanifest_1091500.acf": "\"AppState\"\n{\n\"appid\" \"1091500\"\n\"name\" \"Cyberpunk 2077\"\n\"installdir\" \"Cyberpunk 2077\"\n\"StateFlags\" \"4\"\n}",
		"appmanifest_2.acf":       "\"AppState\"\n{\n\"appid\" \"2\"\n\"name\" \"Partial\"\n\"installdir\" \"Partial\"\n\"StateFlags\" \"2\"\n}",
		"appmanifest_1493710.acf": "\"AppState\"\n{\n\"appid\" \"1493710\"\n\"name\" \"Proton Experimental\"\n\"installdir\" \"Proton\"\n\"StateFlags\" \"4\"\n}",
	} {
		if err := os.WriteFile(filepath.Join(steamapps, name), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	games, err := ScanLibrary(Library{Path: library})
	if err != nil || len(games) != 1 || games[0].AppID != 1091500 || games[0].PrefixPath == "" || len(games[0].DLLs) != 1 {
		t.Fatalf("ScanLibrary = %+v, %v", games, err)
	}
	database, err := ScanLibraries(steamRoot, []string{library, library, ""})
	if err != nil || database == nil || database.GetGame(1091500) == nil {
		t.Fatalf("ScanLibraries = %+v, %v", database, err)
	}
	resolved, err := librariesToScan(steamRoot, []string{library, library, ""})
	if err != nil || len(resolved) != 2 {
		t.Fatalf("librariesToScan = %+v, %v", resolved, err)
	}
}
