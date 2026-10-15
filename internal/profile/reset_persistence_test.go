package profile_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/profile"
)

// saveNonEmptyDefaults writes a defaults profile whose values are non-zero for
// every field the reset tests care about, so a phantom pin would be visible.
func saveNonEmptyDefaults(t *testing.T) *profile.Profile {
	t.Helper()
	defaults := &profile.Profile{
		GPU: profile.GPUSettings{
			ShaderCache:          true,
			ThreadedOptimization: true,
			ClockOffset:          150,
			FanSpeed:             60,
		},
	}
	if err := profile.SaveDefault(defaults); err != nil {
		t.Fatalf("SaveDefault: %v", err)
	}
	return defaults
}

func assertFullyInherited(t *testing.T, appID uint64, defaults *profile.Profile) {
	t.Helper()
	loaded, err := profile.Load(appID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load: expected a profile, got nil")
	}
	for _, field := range profile.AllFields() {
		if loaded.IsOverridden(field) {
			t.Errorf("%s: expected inherited after reload, got overridden", field)
		}
	}
	resolved := loaded.ResolveForApply(defaults)
	if resolved.GPU.ClockOffset != defaults.GPU.ClockOffset {
		t.Errorf("clock offset: expected %d from defaults, got %d", defaults.GPU.ClockOffset, resolved.GPU.ClockOffset)
	}
	if resolved.GPU.FanSpeed != defaults.GPU.FanSpeed {
		t.Errorf("fan speed: expected %d from defaults, got %d", defaults.GPU.FanSpeed, resolved.GPU.FanSpeed)
	}
	if !resolved.GPU.ThreadedOptimization {
		t.Error("threaded optimization: expected true from defaults, got false")
	}
	if !resolved.GPU.ShaderCache {
		t.Error("shader cache: expected true from defaults, got false")
	}
}

// Regression for F1: resetting the last override must reload as fully
// inherited, not as a legacy file that pins every field differing from the
// defaults.
func TestReset_LastOverrideReloadsFullyInherited(t *testing.T) {
	withTempXDG(t)
	defaults := saveNonEmptyDefaults(t)

	const appID = 1091500
	if err := profile.Mutate(appID, func(current, defaultProfile *profile.Profile) error {
		return current.Set(profile.FieldGPUClockOffset, 1500)
	}); err != nil {
		t.Fatalf("set override: %v", err)
	}
	if err := profile.Mutate(appID, func(current, defaultProfile *profile.Profile) error {
		return current.Reset(profile.FieldGPUClockOffset)
	}); err != nil {
		t.Fatalf("reset override: %v", err)
	}

	assertFullyInherited(t, appID, defaults)
}

// Regression for F1: Reset all clears the overrides map; the saved file must
// still say "no overrides" rather than lose the key.
func TestResetAll_ReloadsFullyInherited(t *testing.T) {
	withTempXDG(t)
	defaults := saveNonEmptyDefaults(t)

	const appID = 1091500
	if err := profile.Mutate(appID, func(current, defaultProfile *profile.Profile) error {
		if err := current.Set(profile.FieldGPUClockOffset, 1500); err != nil {
			return err
		}
		return current.Set(profile.FieldGPUFanSpeed, 0)
	}); err != nil {
		t.Fatalf("set overrides: %v", err)
	}
	if err := profile.Mutate(appID, func(current, defaultProfile *profile.Profile) error {
		current.ResetAll()
		return nil
	}); err != nil {
		t.Fatalf("reset all: %v", err)
	}

	assertFullyInherited(t, appID, defaults)
}

// Regression for F1: a profile created empty and then edited once must keep
// the edit as its only override.
func TestCreateThenFirstEdit_KeepsOtherFieldsInherited(t *testing.T) {
	withTempXDG(t)
	defaults := saveNonEmptyDefaults(t)

	const appID = 1091500
	created, err := profile.Create(appID, &profile.Profile{Name: "Cyberpunk 2077"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !created {
		t.Fatal("Create: expected the profile to be created")
	}
	if err := profile.Mutate(appID, func(current, defaultProfile *profile.Profile) error {
		return current.Set(profile.FieldGPUClockOffset, 1500)
	}); err != nil {
		t.Fatalf("first edit: %v", err)
	}

	loaded, err := profile.Load(appID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.IsOverridden(profile.FieldGPUClockOffset) {
		t.Error("clock offset: expected overridden after the first edit")
	}
	for _, field := range profile.AllFields() {
		if field != profile.FieldGPUClockOffset && loaded.IsOverridden(field) {
			t.Errorf("%s: expected inherited, got overridden", field)
		}
	}
	resolved := loaded.ResolveForApply(defaults)
	if resolved.GPU.FanSpeed != 60 || !resolved.GPU.ThreadedOptimization {
		t.Errorf("untouched fields must follow defaults, got fan speed %d, threaded optimization %t",
			resolved.GPU.FanSpeed, resolved.GPU.ThreadedOptimization)
	}
}

// A saved profile always carries the overrides key, so an empty override set
// is distinguishable from a legacy file on the next load.
func TestSave_AlwaysWritesOverridesKey(t *testing.T) {
	withTempXDG(t)

	const appID = 1091500
	if err := profile.Save(appID, &profile.Profile{Name: "empty"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles", "1091500.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved profile: %v", err)
	}
	if !slices.Contains(strings.Split(string(data), "\n"), "overrides: {}") {
		t.Errorf("expected an `overrides: {}` line, got:\n%s", data)
	}
}

// Files that lack the overrides key are still legacy and still migrate; the
// F1 fix must not remove migration for them.
func TestLoad_FileWithoutOverridesKeyStillMigrates(t *testing.T) {
	withTempXDG(t)
	defaults := saveNonEmptyDefaults(t)

	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := []byte("name: legacy\ngpu:\n  clock_offset: 150\n  fan_speed: 80\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "1091500.yaml"), legacy, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	loaded, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.IsOverridden(profile.FieldGPUClockOffset) {
		t.Error("clock offset equals the default: expected inherited after migration")
	}
	if !loaded.IsOverridden(profile.FieldGPUFanSpeed) {
		t.Error("fan speed differs from the default: expected overridden after migration")
	}
	if got := loaded.ResolveForApply(defaults).GPU.FanSpeed; got != 80 {
		t.Errorf("fan speed: expected 80, got %d", got)
	}
}

// The overrides key distinguishes "nothing pinned" from "legacy file": a nil
// or empty map marshals as `overrides: {}`, that loads as a non-nil empty map,
// and an absent key loads as nil.
func TestOverridesKey_MarshalAndLoadContract(t *testing.T) {
	for name, overrides := range map[string]map[string]bool{
		"nil map":   nil,
		"empty map": {},
	} {
		data, err := yaml.Marshal(&profile.Profile{Name: "contract", Overrides: overrides})
		if err != nil {
			t.Fatalf("%s: Marshal: %v", name, err)
		}
		if !slices.Contains(strings.Split(string(data), "\n"), "overrides: {}") {
			t.Errorf("%s: expected an `overrides: {}` line, got:\n%s", name, data)
		}
		var loaded profile.Profile
		if err := yaml.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("%s: Unmarshal: %v", name, err)
		}
		if loaded.Overrides == nil {
			t.Errorf("%s: `overrides: {}` must load as a non-nil map", name)
		}
	}

	var absent profile.Profile
	if err := yaml.Unmarshal([]byte("name: legacy\n"), &absent); err != nil {
		t.Fatalf("Unmarshal absent: %v", err)
	}
	if absent.Overrides != nil {
		t.Errorf("an absent key must load as nil, got %v", absent.Overrides)
	}
}

// Reset, ResetAll, and Create never leave leaf values behind a nil overrides
// map, which the always-written key would otherwise turn into silent loss.
func TestResetAndCreate_NeverLeaveLeavesWithoutOverrides(t *testing.T) {
	pinned := func() *profile.Profile {
		p := &profile.Profile{Name: "pinned"}
		if err := p.Set(profile.FieldGPUClockOffset, 1500); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := p.Set(profile.FieldGPUFanSpeed, 80); err != nil {
			t.Fatalf("Set: %v", err)
		}
		return p
	}
	assertConsistent := func(label string, p *profile.Profile) {
		t.Helper()
		for _, field := range profile.AllFields() {
			value, err := profile.ReadField(p, field)
			if err != nil {
				t.Fatalf("%s: ReadField %s: %v", label, field, err)
			}
			if !p.IsOverridden(field) && value != nil && !reflect.ValueOf(value).IsZero() {
				t.Errorf("%s: %s holds %v but is not overridden", label, field, value)
			}
		}
	}

	reset := pinned()
	if err := reset.Reset(profile.FieldGPUClockOffset); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	assertConsistent("Reset", reset)

	resetAll := pinned()
	resetAll.ResetAll()
	assertConsistent("ResetAll", resetAll)

	withTempXDG(t)
	if _, err := profile.Create(1091500, &profile.Profile{Name: "created"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	created, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertConsistent("Create", created)
}

func writeLegacyEmptyProfile(t *testing.T, appID string) string {
	t.Helper()
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(cfgDir, appID+".yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	return path
}

// A mutation that changes nothing must not rewrite a legacy file, because the
// save would freeze the in-memory migrated pins as deliberate overrides.
func TestMutate_UnchangedLegacyProfileIsNotRewritten(t *testing.T) {
	withTempXDG(t)
	saveNonEmptyDefaults(t)
	path := writeLegacyEmptyProfile(t, "1091500")

	exists, err := profile.MutateExisting(1091500, func(current, defaults *profile.Profile) error {
		return current.Reset(profile.FieldGPUMemoryOffset)
	})
	if err != nil {
		t.Fatalf("MutateExisting: %v", err)
	}
	if !exists {
		t.Fatal("MutateExisting: expected the profile to exist")
	}
	if err := profile.Mutate(1091500, func(current, defaults *profile.Profile) error { return nil }); err != nil {
		t.Fatalf("Mutate: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if string(data) != "{}\n" {
		t.Errorf("unchanged mutation rewrote the file:\n%s", data)
	}
}

// A mutation that creates a missing profile still writes it, with the key.
func TestMutate_CreatesMissingProfileWithOverridesKey(t *testing.T) {
	withTempXDG(t)

	if err := profile.Mutate(1091500, func(current, defaults *profile.Profile) error { return nil }); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles", "1091500.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected the profile to be created: %v", err)
	}
	if !slices.Contains(strings.Split(string(data), "\n"), "overrides: {}") {
		t.Errorf("expected an `overrides: {}` line, got:\n%s", data)
	}

	exists, err := profile.MutateExisting(2, func(current, defaults *profile.Profile) error { return nil })
	if err != nil || exists {
		t.Fatalf("MutateExisting on a missing profile: exists=%t err=%v", exists, err)
	}
	if profile.Exists(2) {
		t.Error("MutateExisting must not create a missing profile")
	}
}

// A mutation that changes a field is written, and the first edit of a legacy
// file freezes its migrated state (the documented residual risk).
func TestMutate_ChangedProfileIsWritten(t *testing.T) {
	withTempXDG(t)
	saveNonEmptyDefaults(t)
	writeLegacyEmptyProfile(t, "1091500")

	if err := profile.Mutate(1091500, func(current, defaults *profile.Profile) error {
		return current.Set(profile.FieldGPUClockOffset, 1500)
	}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	loaded, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.IsOverridden(profile.FieldGPUClockOffset) || loaded.GPU.ClockOffset != 1500 {
		t.Errorf("expected the edit to persist, got %+v", loaded.GPU)
	}
}
