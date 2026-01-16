package profile_test

import (
	"testing"

	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/profile"
)

func TestModelPresetSelection(t *testing.T) {
	tests := []struct {
		name     string
		mode     profile.DLSSMode
		model    profile.DLSSModelPreset
		expected string
	}{
		{"Auto + Ultra Perf -> L", profile.DLSSModeUltraPerformance, profile.DLSSModelPresetAuto, "render_preset_l"},
		{"Auto + Performance -> M", profile.DLSSModePerformance, profile.DLSSModelPresetAuto, "render_preset_m"},
		{"Auto + Balanced -> K", profile.DLSSModeBalanced, profile.DLSSModelPresetAuto, "render_preset_k"},
		{"Auto + Quality -> K", profile.DLSSModeQuality, profile.DLSSModelPresetAuto, "render_preset_k"},
		{"Auto + DLAA -> K", profile.DLSSModeDLAA, profile.DLSSModelPresetAuto, "render_preset_k"},
		{"Explicit K", profile.DLSSModePerformance, profile.DLSSModelPresetK, "render_preset_k"},
		{"Explicit L", profile.DLSSModeBalanced, profile.DLSSModelPresetL, "render_preset_l"},
		{"Explicit M", profile.DLSSModeQuality, profile.DLSSModelPresetM, "render_preset_m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &profile.Profile{
				DLSS: profile.DLSSSettings{
					SRMode:        tt.mode,
					SRModelPreset: tt.model,
					SROverride:    true,
				},
			}
			e := env.New()
			p.Apply(e)
			actual := e.Get("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION")
			if actual != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, actual)
			}
		})
	}
}
