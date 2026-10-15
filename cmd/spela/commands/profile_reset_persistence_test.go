package commands

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

// Regression for F1 through the CLI: setting one field on a fresh game and
// resetting it must leave every field inherited after a reload, even when the
// defaults profile holds non-zero values.
func TestProtonResetLastOverride_ReloadsFullyInherited(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
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

	protonSetHDR = "true"
	protonSetWayland = ""
	protonSetNGXUpdater = ""
	protonSetVKD3DHeap = ""
	t.Cleanup(func() { protonSetHDR = "" })
	if err := runProtonSet(protonSetCmd, []string{"1091500"}); err != nil {
		t.Fatalf("runProtonSet: %v", err)
	}
	captureStdout(t, func() {
		if err := runProtonReset(nil, []string{"1091500", "hdr"}); err != nil {
			t.Fatalf("runProtonReset: %v", err)
		}
	})

	gpuShowJSON, dlssShowJSON, protonShowJSON = false, false, false
	for name, show := range map[string]func() error{
		"gpu show":    func() error { return runGPUShow(nil, []string{"1091500"}) },
		"proton show": func() error { return runProtonShowWithNotice([]string{"1091500"}, func(uint64) string { return "" }) },
		"dlss show":   func() error { return runDLSSShow(nil, []string{"1091500"}) },
	} {
		out := captureStdout(t, func() {
			if err := show(); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
		if !strings.Contains(out, "from All games") && !strings.Contains(out, "system default") {
			t.Fatalf("%s: expected fields to follow All games or the system default, got:\n%s", name, out)
		}
		if strings.Contains(out, "this game") {
			t.Errorf("%s: expected no overridden field after the reset, got:\n%s", name, out)
		}
	}

	loaded, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, field := range profile.AllFields() {
		if loaded.IsOverridden(field) {
			t.Errorf("%s: expected inherited after reload, got overridden", field)
		}
	}
	if resolved := loaded.ResolveForApply(defaults); resolved.GPU.ClockOffset != 150 || resolved.GPU.FanSpeed != 60 {
		t.Errorf("expected defaults to apply, got clock offset %d, fan speed %d",
			resolved.GPU.ClockOffset, resolved.GPU.FanSpeed)
	}
}
