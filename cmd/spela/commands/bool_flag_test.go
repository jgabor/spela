package commands

import "testing"

func TestParseBoolFlag_AcceptsTrueFalseForms(t *testing.T) {
	for _, raw := range []string{"true", "1", "yes", "on", "false", "0", "no", "off"} {
		if _, err := parseBoolFlag(raw); err != nil {
			t.Fatalf("parseBoolFlag(%q): %v", raw, err)
		}
	}
}

func TestParseBoolFlag_RejectsInvalidText(t *testing.T) {
	if _, err := parseBoolFlag("maybe"); err == nil {
		t.Fatal("parseBoolFlag(maybe): expected error, got nil")
	}
}
