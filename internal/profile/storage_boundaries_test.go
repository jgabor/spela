package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileStorageSupportedFilesystemAndListingBoundaries(t *testing.T) {
	t.Run("read errors", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", root)
		if err := os.MkdirAll(profilePath(1), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(1); err == nil {
			t.Fatal("profile directory unexpectedly loaded as a file")
		}
		if err := os.MkdirAll(defaultProfilePath(), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadDefault(); err == nil {
			t.Fatal("default profile directory unexpectedly loaded as a file")
		}
	})

	t.Run("list filters unsupported entries", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", root)
		if err := EnsureProfilesDir(); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(profilesDir(), "directory.yaml"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Save(3, &Profile{Name: "Fixture"}); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string]string{
			"notes.txt":    "ignored",
			"invalid.yaml": "name: ignored\n",
			"2.yaml":       "not: [yaml",
		} {
			if err := os.WriteFile(filepath.Join(profilesDir(), name), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		profiles, err := List()
		if err != nil || len(profiles) != 1 || profiles[3] == nil {
			t.Fatalf("listed profiles = %+v, %v", profiles, err)
		}
	})

	t.Run("directory creation errors", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", root)
		parent := filepath.Dir(profilesDir())
		if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(parent, []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Save(1, &Profile{}); err == nil {
			t.Fatal("game profile save unexpectedly created through a file")
		}
		if err := SaveDefault(&Profile{}); err == nil {
			t.Fatal("default profile save unexpectedly created through a file")
		}
	})
}
