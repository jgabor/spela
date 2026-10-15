//go:build dev || production || bindings

package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

func TestGUIProfilePatchResetAllReloadsFullyInherited(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	defaults := &profile.Profile{
		GPU: profile.GPUSettings{ShaderCache: true, ThreadedOptimization: true, ClockOffset: 150, FanSpeed: 60},
	}
	if err := profile.SaveDefault(defaults); err != nil {
		t.Fatal(err)
	}
	boundary := defaultGUIApplicationBoundary(nil)

	if err := boundary.patchGameProfile(1091500, []ProfilePatch{
		{Field: profile.FieldGPUClockOffset, Operation: "set", Value: float64(1500)},
		{Field: profile.FieldGPUFanSpeed, Operation: "set", Value: float64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := boundary.patchGameProfile(1091500, []ProfilePatch{
		{Field: profile.FieldGPUClockOffset, Operation: "reset"},
		{Field: profile.FieldGPUFanSpeed, Operation: "reset"},
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := profile.Load(1091500)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range profile.AllFields() {
		if loaded.IsOverridden(field) {
			t.Errorf("%s: expected inherited after reload, got overridden", field)
		}
	}
	resolved := loaded.ResolveForApply(defaults)
	if resolved.GPU.ClockOffset != 150 || resolved.GPU.FanSpeed != 60 || !resolved.GPU.ThreadedOptimization {
		t.Fatalf("expected defaults to apply, got %+v", resolved.GPU)
	}
}

func TestGUIProfilePatchWithNoOperationsLeavesLegacyFileUnchanged(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := profile.SaveDefault(&profile.Profile{GPU: profile.GPUSettings{ClockOffset: 150}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configHome, "spela", "profiles", "1091500.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := defaultGUIApplicationBoundary(nil).patchGameProfile(1091500, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}\n" {
		t.Fatalf("a patch list with no operations rewrote the file:\n%s", data)
	}
}
