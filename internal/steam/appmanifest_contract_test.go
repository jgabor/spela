package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppManifestPrefixAndToolClassificationContracts(t *testing.T) {
	library := t.TempDir()
	manifestPath := filepath.Join(library, "steamapps", "appmanifest_1091500.acf")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `"AppState"
{
  "appid" "1091500"
  "name" "Cyberpunk 2077"
  "installdir" "Cyberpunk 2077"
  "StateFlags" "4"
  "LastUpdated" "1710000000"
  "SizeOnDisk" "70000000000"
}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseAppManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.AppID != 1091500 || parsed.Name != "Cyberpunk 2077" || parsed.InstallDir != "Cyberpunk 2077" || parsed.LibraryPath != library || parsed.FullInstallDir != filepath.Join(library, "steamapps", "common", "Cyberpunk 2077") || parsed.LastUpdated != 1710000000 || parsed.SizeOnDisk != 70000000000 || !parsed.IsFullyInstalled() {
		t.Fatalf("parsed app manifest = %+v", parsed)
	}
	parsed.StateFlags = 2
	if parsed.IsFullyInstalled() {
		t.Fatal("partial install reported fully installed")
	}

	for name, contents := range map[string]string{
		"missing AppState": `"Other" { "appid" "1" }`,
		"missing app ID":   `"AppState" { "name" "No ID" }`,
		"invalid app ID":   `"AppState" { "appid" "invalid" }`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "appmanifest.acf")
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			value, err := ParseAppManifest(path)
			if err != nil || value != nil {
				t.Fatalf("ParseAppManifest = %+v, %v", value, err)
			}
		})
	}
	if _, err := ParseAppManifest(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing app manifest did not fail")
	}

	compatData := filepath.Join(library, "steamapps", "compatdata")
	prefix := ScanProtonPrefix(compatData, 1091500)
	if prefix.IsValid || prefix.AppID != 1091500 {
		t.Fatalf("missing prefix = %+v", prefix)
	}
	if err := os.MkdirAll(prefix.DriveC, 0o755); err != nil {
		t.Fatal(err)
	}
	prefix = ScanProtonPrefix(compatData, 1091500)
	if !prefix.IsValid || prefix.Path != filepath.Join(compatData, "1091500") {
		t.Fatalf("valid prefix = %+v", prefix)
	}

	gameDirectory := t.TempDir()
	if IsTool(1091500, "Cyberpunk 2077", gameDirectory) {
		t.Fatal("normal game classified as tool")
	}
	if !IsTool(1493710, "Anything", gameDirectory) || !IsTool(1, "Proton Experimental", gameDirectory) {
		t.Fatal("known Steam tools were not classified")
	}
	if err := os.WriteFile(filepath.Join(gameDirectory, "toolmanifest.vdf"), []byte("tools"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsTool(1, "Unknown", gameDirectory) {
		t.Fatal("tool manifest was not classified")
	}
}
