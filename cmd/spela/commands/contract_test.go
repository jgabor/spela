package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/profile"
)

func TestConfigCLITextAndErrorContract(t *testing.T) {
	withTempXDG(t)
	out := captureStdout(t, func() {
		ConfigCmd.SetArgs([]string{"set", "check_updates", "false"})
		if err := ConfigCmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if out != "Set check_updates = false\n" {
		t.Fatalf("config set output = %q", out)
	}
	loaded, err := config.Load()
	if err != nil || loaded.CheckUpdates {
		t.Fatalf("config set persistence = %+v, %v", loaded, err)
	}
	ConfigCmd.SetArgs([]string{"set", "not_a_key", "value"})
	if err := ConfigCmd.Execute(); err == nil || err.Error() != "unknown config key: not_a_key" {
		t.Fatalf("unknown-key error = %v", err)
	}
}

func TestProfileCLITextJSONAndErrorContract(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)
	p := &profile.Profile{Name: "Quality", Proton: profile.ProtonSettings{EnableHDR: true}}
	p.MarkOverride(profile.FieldProtonEnableHDR)
	if err := profile.Save(1091500, p); err != nil {
		t.Fatal(err)
	}

	text := captureStdout(t, func() {
		ProfileCmd.SetArgs([]string{"show", "1091500"})
		if err := ProfileCmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	for _, fragment := range []string{"Profile for", "Cyberpunk 2077", "DLSS", "HDR:", "true", "Overlay"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("profile text missing %q:\n%s", fragment, text)
		}
	}

	encoded := captureStdout(t, func() {
		ProfileCmd.SetArgs([]string{"show", "Cyberpunk 2077", "--json"})
		if err := ProfileCmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	var decoded map[string]any
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("profile --json output is not JSON: %v\n%s", err, encoded)
	}
	if decoded["Name"] != "Quality" {
		t.Fatalf("profile JSON Name = %#v", decoded["Name"])
	}
	if _, ok := decoded["Overrides"]; !ok {
		t.Fatalf("profile JSON keys changed: %#v", decoded)
	}

	ProfileCmd.SetArgs([]string{"show", "missing"})
	if err := ProfileCmd.Execute(); err == nil || err.Error() != "game not found: missing" {
		t.Fatalf("missing-game error = %v", err)
	}
}

func TestProfileCommandRejectsMissingArgument(t *testing.T) {
	ProfileCmd.SetArgs([]string{"show"})
	if err := ProfileCmd.Execute(); err == nil || err.Error() != "accepts 1 arg(s), received 0" {
		t.Fatalf("missing argument error = %v", err)
	}
}
