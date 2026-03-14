package denylist

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "spela", "lists"), func() {}
}

func TestLoadDenyListDefault(t *testing.T) {
	setupTestDir(t)

	list, err := LoadDenyList()
	if err != nil {
		t.Fatalf("LoadDenyList() error = %v", err)
	}
	if list == nil {
		t.Fatal("LoadDenyList() returned nil")
	}
	if len(list.Entries) == 0 {
		t.Error("default deny list should have entries")
	}
}

func TestLoadOverridesEmpty(t *testing.T) {
	setupTestDir(t)

	overrides, err := LoadOverrides()
	if err != nil {
		t.Fatalf("LoadOverrides() error = %v", err)
	}
	if overrides == nil {
		t.Fatal("LoadOverrides() returned nil")
	}
	if len(overrides.Allowed) != 0 || len(overrides.Denied) != 0 {
		t.Error("fresh overrides should be empty")
	}
}

func TestIsDeniedDefaultList(t *testing.T) {
	setupTestDir(t)

	denied, reason := IsDenied(1172470) // Apex Legends
	if !denied {
		t.Error("Apex Legends should be denied by default")
	}
	if reason == "" {
		t.Error("denied entry should have a reason")
	}

	denied, _ = IsDenied(99999999)
	if denied {
		t.Error("unknown app should not be denied")
	}
}

func TestAllowOverridesDenyList(t *testing.T) {
	setupTestDir(t)

	if err := Allow(1172470); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}

	denied, _ := IsDenied(1172470)
	if denied {
		t.Error("allowed app should not be denied")
	}
}

func TestDenyAndRemoveDeny(t *testing.T) {
	setupTestDir(t)

	if err := Deny(12345, "Test Game", "test reason"); err != nil {
		t.Fatalf("Deny() error = %v", err)
	}

	denied, reason := IsDenied(12345)
	if !denied {
		t.Error("denied app should be denied")
	}
	if reason != "test reason" {
		t.Errorf("reason = %q, want %q", reason, "test reason")
	}

	if err := RemoveDeny(12345); err != nil {
		t.Fatalf("RemoveDeny() error = %v", err)
	}

	denied, _ = IsDenied(12345)
	if denied {
		t.Error("removed deny should no longer be denied")
	}
}

func TestDenyUpdatesExistingEntry(t *testing.T) {
	setupTestDir(t)

	if err := Deny(12345, "Old Name", "old reason"); err != nil {
		t.Fatalf("Deny() error = %v", err)
	}
	if err := Deny(12345, "New Name", "new reason"); err != nil {
		t.Fatalf("Deny() error = %v", err)
	}

	overrides, err := LoadOverrides()
	if err != nil {
		t.Fatalf("LoadOverrides() error = %v", err)
	}

	count := 0
	for _, e := range overrides.Denied {
		if e.AppID == 12345 {
			count++
			if e.Name != "New Name" {
				t.Errorf("name = %q, want %q", e.Name, "New Name")
			}
			if e.Reason != "new reason" {
				t.Errorf("reason = %q, want %q", e.Reason, "new reason")
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry for appID, got %d", count)
	}
}

func TestAllowIdempotent(t *testing.T) {
	setupTestDir(t)

	if err := Allow(12345); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if err := Allow(12345); err != nil {
		t.Fatalf("Allow() second call error = %v", err)
	}

	overrides, err := LoadOverrides()
	if err != nil {
		t.Fatalf("LoadOverrides() error = %v", err)
	}

	count := 0
	for _, id := range overrides.Allowed {
		if id == 12345 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 allowed entry, got %d", count)
	}
}

func TestRemoveAllowAndRemoveDenyNoOp(t *testing.T) {
	setupTestDir(t)

	if err := RemoveAllow(99999); err != nil {
		t.Errorf("RemoveAllow() on missing entry should not error, got %v", err)
	}
	if err := RemoveDeny(99999); err != nil {
		t.Errorf("RemoveDeny() on missing entry should not error, got %v", err)
	}
}

func TestLoadDenyListFromFile(t *testing.T) {
	dir, _ := setupTestDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	custom := DenyList{
		Entries: []Entry{
			{AppID: 1, Name: "Custom Game", Reason: "custom"},
		},
	}
	data, _ := yaml.Marshal(custom)
	if err := os.WriteFile(filepath.Join(dir, "denylist.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := LoadDenyList()
	if err != nil {
		t.Fatalf("LoadDenyList() error = %v", err)
	}
	if len(list.Entries) != 1 || list.Entries[0].Name != "Custom Game" {
		t.Errorf("expected custom deny list, got %+v", list)
	}
}
