package profile_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/profile"
)

func TestApplyProton_VKD3DHeapEnabled(t *testing.T) {
	p := &profile.Profile{
		Proton: profile.ProtonSettings{VKD3DHeap: true},
	}
	e := env.New()
	p.Apply(e)

	if got := e.Get("PROTON_VKD3D_HEAP"); got != "1" {
		t.Errorf("PROTON_VKD3D_HEAP: expected %q, got %q", "1", got)
	}
	if got := e.Get("VKD3D_CONFIG"); got != "descriptor_heap" {
		t.Errorf("VKD3D_CONFIG: expected %q, got %q", "descriptor_heap", got)
	}
}

func TestApplyProton_VKD3DHeapDisabled(t *testing.T) {
	p := &profile.Profile{
		Proton: profile.ProtonSettings{VKD3DHeap: false},
	}
	e := env.New()
	p.Apply(e)

	if got := e.Get("PROTON_VKD3D_HEAP"); got != "" {
		t.Errorf("PROTON_VKD3D_HEAP: expected unset, got %q", got)
	}
	if got := e.Get("VKD3D_CONFIG"); got != "" {
		t.Errorf("VKD3D_CONFIG: expected unset, got %q", got)
	}
}

// TestProtonSettings_YAMLRoundTrip_VKD3DHeapAbsent verifies that existing
// profile YAMLs written before the vkd3d_heap field was introduced continue
// to load, default VKD3DHeap to false, and that re-saving such a profile
// round-trips cleanly without surfacing the zero-value field.
func TestProtonSettings_YAMLRoundTrip_VKD3DHeapAbsent(t *testing.T) {
	// Legacy YAML — no vkd3d_heap key.
	legacy := []byte(`name: test
proton:
  enable_wayland: true
  enable_hdr: true
`)

	var p profile.Profile
	if err := yaml.Unmarshal(legacy, &p); err != nil {
		t.Fatalf("unmarshal legacy YAML: %v", err)
	}
	if p.Proton.VKD3DHeap {
		t.Errorf("VKD3DHeap: expected default false for absent key, got true")
	}
	if !p.Proton.EnableWayland || !p.Proton.EnableHDR {
		t.Errorf("legacy fields lost in round-trip: %+v", p.Proton)
	}

	out, err := yaml.Marshal(&p)
	if err != nil {
		t.Fatalf("marshal after round-trip: %v", err)
	}

	// Re-load the re-marshaled YAML and confirm field still defaults to false.
	var p2 profile.Profile
	if err := yaml.Unmarshal(out, &p2); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if p2.Proton.VKD3DHeap {
		t.Errorf("VKD3DHeap: expected false after re-save, got true")
	}

	// omitempty should keep the key out of the serialized form when false.
	if containsKey(out, "vkd3d_heap") {
		t.Errorf("omitempty broken: vkd3d_heap appeared in marshaled output:\n%s", out)
	}

	// Now flip the field on and verify it survives a second round-trip.
	p2.Proton.VKD3DHeap = true
	out2, err := yaml.Marshal(&p2)
	if err != nil {
		t.Fatalf("marshal with VKD3DHeap=true: %v", err)
	}
	if !containsKey(out2, "vkd3d_heap") {
		t.Errorf("vkd3d_heap missing from marshaled output when true:\n%s", out2)
	}
	var p3 profile.Profile
	if err := yaml.Unmarshal(out2, &p3); err != nil {
		t.Fatalf("re-unmarshal with VKD3DHeap=true: %v", err)
	}
	if !p3.Proton.VKD3DHeap {
		t.Errorf("VKD3DHeap: expected true after round-trip, got false")
	}
}

// containsKey is a trivial substring check scoped to test use — sufficient
// for asserting presence/absence of a YAML key in marshaled output.
func containsKey(data []byte, key string) bool {
	needle := []byte(key + ":")
	for i := 0; i+len(needle) <= len(data); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if data[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

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
