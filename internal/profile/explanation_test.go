package profile_test

import (
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

func TestExplainField_SourcesTrackLiveDefaults(t *testing.T) {
	defaults := &profile.Profile{
		Proton: profile.ProtonSettings{
			EnableHDR: true,
			VKD3DHeap: true,
		},
	}
	game := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: false}}
	game.MarkOverride(profile.FieldProtonVKD3DHeap)

	inherited, err := game.ExplainField(profile.FieldProtonEnableHDR, defaults)
	if err != nil {
		t.Fatalf("ExplainField inherited: %v", err)
	}
	if inherited.Source != profile.ExplanationSourceDefault || inherited.Value != true {
		t.Fatalf("HDR explanation: expected default true, got source=%s value=%v", inherited.Source, inherited.Value)
	}

	overridden, err := game.ExplainField(profile.FieldProtonVKD3DHeap, defaults)
	if err != nil {
		t.Fatalf("ExplainField override: %v", err)
	}
	if overridden.Source != profile.ExplanationSourceOverride || overridden.Value != false || overridden.DefaultValue != true {
		t.Fatalf("VKD3D explanation: expected override false with default true, got source=%s value=%v default=%v", overridden.Source, overridden.Value, overridden.DefaultValue)
	}

	unset, err := game.ExplainField(profile.FieldOverlayPosition, nil)
	if err != nil {
		t.Fatalf("ExplainField unset: %v", err)
	}
	if unset.Source != profile.ExplanationSourceUnset || unset.Value != "" {
		t.Fatalf("overlay position explanation: expected unset empty string, got source=%s value=%v", unset.Source, unset.Value)
	}

	defaults.Proton.EnableHDR = false
	changed, err := game.ExplainField(profile.FieldProtonEnableHDR, defaults)
	if err != nil {
		t.Fatalf("ExplainField changed default: %v", err)
	}
	if changed.Source != profile.ExplanationSourceDefault || changed.Value != false || changed.DefaultValue != false {
		t.Fatalf("changed default explanation: expected default false, got source=%s value=%v default=%v", changed.Source, changed.Value, changed.DefaultValue)
	}
}

func TestExplainField_RejectsUnknownField(t *testing.T) {
	if _, err := (&profile.Profile{}).ExplainField("gpu.unknown", nil); err == nil {
		t.Fatal("ExplainField: expected unknown field error")
	}
}

func TestFieldSemantics_ClassifyImpactAndRestoreCoverage(t *testing.T) {
	cases := []struct {
		field   string
		impact  profile.LaunchImpact
		restore profile.RestoreCoverage
	}{
		{profile.FieldDLSSSRMode, profile.LaunchImpactEnvironment, profile.RestoreCoverageEphemeralLaunch},
		{profile.FieldProtonVKD3DHeap, profile.LaunchImpactCompatibility, profile.RestoreCoverageEphemeralLaunch},
		{profile.FieldGPUPowerLimit, profile.LaunchImpactSystemState, profile.RestoreCoverageRestorableMutation},
		{profile.FieldOverlayEnabled, profile.LaunchImpactOverlay, profile.RestoreCoverageNotApplicable},
	}

	for _, tc := range cases {
		descriptor, ok := profile.Field(tc.field)
		if !ok {
			t.Fatalf("Field(%s) not found", tc.field)
		}
		if descriptor.Impact != tc.impact {
			t.Fatalf("Field(%s) impact: expected %s, got %s", tc.field, tc.impact, descriptor.Impact)
		}
		if descriptor.Restore != tc.restore {
			t.Fatalf("Field(%s) restore: expected %s, got %s", tc.field, tc.restore, descriptor.Restore)
		}
	}
}

func TestFieldSemantics_RejectUnknownField(t *testing.T) {
	if _, ok := profile.Field("gpu.unknown"); ok {
		t.Fatal("Field: expected unknown field rejection")
	}
}
