package commands

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

func TestGPUShowVRRInheritanceAndUnset(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	if err := profile.SaveDefault(&profile.Profile{GPU: profile.GPUSettings{VRR: "always"}}); err != nil {
		t.Fatal(err)
	}
	p := &profile.Profile{}
	for _, policy := range []string{"", "unset"} {
		if policy != "" {
			if err := p.Set(profile.FieldGPUVRR, policy); err != nil {
				t.Fatal(err)
			}
		}
		if err := profile.Save(1091500, p); err != nil {
			t.Fatal(err)
		}
		gpuShowJSON = false
		text := captureStdout(t, func() {
			if err := runGPUShow(nil, []string{"1091500"}); err != nil {
				t.Fatal(err)
			}
		})
		want := "always"
		if policy != "" {
			want = policy
		}
		if !strings.Contains(text, "KDE VRR:") || !strings.Contains(text, want) {
			t.Fatalf("missing VRR: %s", text)
		}
	}
}
