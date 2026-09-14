package profile

import (
	"strings"
	"testing"
)

func TestVRRPersistenceAndInheritance(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defaults := &Profile{GPU: GPUSettings{VRR: "always"}}
	if err := SaveDefault(defaults); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"", "unset", "automatic", "always", "never"} {
		t.Run("policy="+policy, func(t *testing.T) {
			p := &Profile{}
			if err := p.Set(FieldGPUVRR, policy); err != nil {
				t.Fatal(err)
			}
			if err := Save(1091500, p); err != nil {
				t.Fatal(err)
			}
			loaded, err := Load(1091500)
			if err != nil {
				t.Fatal(err)
			}
			if !loaded.IsOverridden(FieldGPUVRR) || loaded.ResolveForApply(defaults).GPU.VRR != policy {
				t.Fatalf("lost override: %+v", loaded)
			}
			if err := loaded.Reset(FieldGPUVRR); err != nil {
				t.Fatal(err)
			}
			if err := Save(1091500, loaded); err != nil {
				t.Fatal(err)
			}
			loaded, err = Load(1091500)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.IsOverridden(FieldGPUVRR) || loaded.ResolveForApply(defaults).GPU.VRR != "always" {
				t.Fatalf("lost inheritance: %+v", loaded)
			}
		})
	}
}

func TestVRRLegacyAndInvalidValues(t *testing.T) {
	defaults := &Profile{GPU: GPUSettings{VRR: "unset"}}
	for _, policy := range []string{"", "unset", "automatic", "always", "never"} {
		p, err := unmarshalProfileYAML([]byte("gpu:\n  vrr: \"" + policy + "\"\n"))
		if err != nil {
			t.Fatal(err)
		}
		migrateInheritance(p, defaults)
		if p.IsOverridden(FieldGPUVRR) != (policy != "") {
			t.Fatalf("legacy %q: %+v", policy, p)
		}
	}
	p, err := unmarshalProfileYAML([]byte("name: old profile\n"))
	if err != nil {
		t.Fatal(err)
	}
	migrateInheritance(p, nil)
	if p.ResolveForApply(nil).GPU.VRR != "" {
		t.Fatal("unconfigured VRR changed")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, err := range []error{
		(&Profile{}).Set(FieldGPUVRR, "adaptive"),
		Save(1091500, &Profile{GPU: GPUSettings{VRR: "adaptive"}}),
		func() error { _, err := unmarshalProfileYAML([]byte("gpu:\n  vrr: adaptive\n")); return err }(),
	} {
		if err == nil || !strings.Contains(err.Error(), "gpu.vrr") {
			t.Fatalf("unclear validation: %v", err)
		}
	}
}
