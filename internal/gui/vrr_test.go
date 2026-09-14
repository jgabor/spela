//go:build dev || production || bindings

package gui

import (
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

func TestGUIVRRPatchPersistenceAndReset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	boundary := defaultGUIApplicationBoundary(nil)
	if err := boundary.patchDefaultProfile([]ProfilePatch{{Field: profile.FieldGPUVRR, Operation: "set", Value: "always"}}); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"unset", "automatic", "always", "never"} {
		if err := boundary.patchGameProfile(1091500, []ProfilePatch{{Field: profile.FieldGPUVRR, Operation: "set", Value: policy}}); err != nil {
			t.Fatal(err)
		}
		view := boundary.profileView(appID(1091500))
		if view["vrr"] != policy {
			t.Fatalf("view = %+v", view)
		}
		semantics := semanticsByField(view["semantics"].([]ProfileFieldSemantics))
		assertProfileSemantic(t, semantics[profile.FieldGPUVRR], "override", "system_state", "restorable_mutation")
	}
	if err := boundary.patchGameProfile(1091500, []ProfilePatch{{Field: profile.FieldGPUVRR, Operation: "set", Value: "adaptive"}}); err == nil {
		t.Fatal("accepted invalid policy")
	}
	if boundary.profileView(appID(1091500))["vrr"] != "never" {
		t.Fatal("failed patch changed saved state")
	}
	if err := boundary.patchGameProfile(1091500, []ProfilePatch{{Field: profile.FieldGPUVRR, Operation: "reset"}}); err != nil {
		t.Fatal(err)
	}
	view := boundary.profileView(appID(1091500))
	if view["vrr"] != "always" {
		t.Fatalf("reset did not inherit: %+v", view)
	}
	semantics := semanticsByField(view["semantics"].([]ProfileFieldSemantics))
	assertProfileSemantic(t, semantics[profile.FieldGPUVRR], "default", "system_state", "restorable_mutation")
}
