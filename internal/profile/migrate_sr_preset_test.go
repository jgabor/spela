package profile

import "testing"

func TestMigrateSRPreset_LegacyModelWins(t *testing.T) {
	p := &Profile{
		DLSS: DLSSSettings{
			SRPreset: DLSSPresetA,
		},
	}
	migrateSRPreset(p, "l")
	if p.DLSS.SRPreset != DLSSPresetL {
		t.Fatalf("expected L, got %q", p.DLSS.SRPreset)
	}
}

func TestMigrateSRPreset_LegacyAuto(t *testing.T) {
	p := &Profile{}
	migrateSRPreset(p, "auto")
	if p.DLSS.SRPreset != DLSSPresetAuto {
		t.Fatalf("expected auto, got %q", p.DLSS.SRPreset)
	}
}

func TestMigrateSRPreset_NormalizesLowercase(t *testing.T) {
	p := &Profile{
		DLSS: DLSSSettings{
			SRPreset: DLSSPreset("l"),
		},
	}
	migrateSRPreset(p, "")
	if p.DLSS.SRPreset != DLSSPresetL {
		t.Fatalf("expected L, got %q", p.DLSS.SRPreset)
	}
}

func TestMigrateSRPreset_OverrideMap(t *testing.T) {
	p := &Profile{
		DLSS: DLSSSettings{},
		Overrides: map[string]bool{
			"dlss.sr_model_preset": true,
		},
	}
	migrateSRPreset(p, "k")
	if !p.Overrides[FieldDLSSSRPreset] {
		t.Fatal("expected sr_preset override to be set")
	}
	if p.Overrides["dlss.sr_model_preset"] {
		t.Fatal("expected legacy override key to be removed")
	}
	if p.DLSS.SRPreset != DLSSPresetK {
		t.Fatalf("expected K, got %q", p.DLSS.SRPreset)
	}
}

func TestUnmarshalProfileYAML_LegacyField(t *testing.T) {
	data := []byte(`dlss:
  sr_preset: A
  sr_model_preset: l
`)
	p, err := unmarshalProfileYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.DLSS.SRPreset != DLSSPresetL {
		t.Fatalf("expected L from legacy model preset, got %q", p.DLSS.SRPreset)
	}
}

func TestNormalizeSRPreset(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"auto", "auto"},
		{"default", "default"},
		{"l", "L"},
		{"K", "K"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeSRPreset(tt.in)
		if string(got) != tt.want {
			t.Errorf("NormalizeSRPreset(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
