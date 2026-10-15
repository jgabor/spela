package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

// Regression for F1 through the TUI save path: Reset all on a game profile,
// saved through the same pipeline as the Save action, reloads as fully
// inherited under a non-empty defaults profile.
func TestResetAllSaveReloadsFullyInherited(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defaults := &profile.Profile{
		GPU: profile.GPUSettings{ShaderCache: true, ThreadedOptimization: true, ClockOffset: 150, FanSpeed: 60},
	}
	if err := profile.SaveDefault(defaults); err != nil {
		t.Fatal(err)
	}
	const appID = 1091500
	if err := profile.Mutate(appID, func(current, _ *profile.Profile) error {
		if err := current.Set(profile.FieldGPUClockOffset, 1500); err != nil {
			return err
		}
		return current.Set(profile.FieldGPUFanSpeed, 0)
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := profile.Load(appID)
	if err != nil {
		t.Fatal(err)
	}

	before := stored.Clone()
	detail := NewDetail(NewStyles(DefaultTheme), stored, defaults)
	if _, handled := detail.UpdateAction(ActionDetailResetAll); !handled {
		t.Fatal("Reset all was not handled")
	}
	if len(detail.RawProfile().Overrides) != 0 {
		t.Fatalf("Reset all kept overrides: %v", detail.RawProfile().Overrides)
	}

	saves := &profileSaveState{}
	command := saves.start(profileSaveRequest{appID: appID, before: before, desired: detail.RawProfile().Clone()})
	message, ok := command().(profileSaveMsg)
	if !ok || message.err != nil {
		t.Fatalf("save failed: %+v", message)
	}

	reloaded, err := profile.Load(appID)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range profile.AllFields() {
		if reloaded.IsOverridden(field) {
			t.Errorf("%s: expected inherited after reload, got overridden", field)
		}
	}
	if resolved := reloaded.ResolveForApply(defaults); resolved.GPU.ClockOffset != 150 || resolved.GPU.FanSpeed != 60 {
		t.Fatalf("expected defaults to apply, got %+v", resolved.GPU)
	}
}
