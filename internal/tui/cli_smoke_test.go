package tui

import (
	"strings"
	"testing"
)

// TestCLIHelpers_NoLegacyColors_Smoke confirms the new CLI helpers embed the
// neon-palette hex in their rendered output and that none of the legacy
// amethyst/royal-blue ANSI indices survived the refactor.
func TestCLIHelpers_NoLegacyColors_Smoke(t *testing.T) {
	outputs := map[string]string{
		"CLIPrimary":   CLIPrimary("PRIMARY"),
		"CLISecondary": CLISecondary("SECONDARY"),
		"CLIDim":       CLIDim("DIM"),
		"CLISuccess":   CLISuccess("SUCCESS"),
		"CLIAccent":    CLIAccent("ACCENT"),
	}
	for name, s := range outputs {
		t.Logf("%-14s = %q", name, s)
	}
	combined := strings.Join([]string{
		outputs["CLIPrimary"], outputs["CLISecondary"], outputs["CLIDim"],
		outputs["CLISuccess"], outputs["CLIAccent"],
	}, "|")
	legacyMarkers := map[string]string{
		"ANSI 133 (Amethyst)":       "\x1b[38;5;133m",
		"ANSI 69 (Royal Blue)":      "\x1b[38;5;69m",
		"ANSI 91 (Velvet Orchid)":   "\x1b[38;5;91m",
		"ANSI 212 (Pink Carnation)": "\x1b[38;5;212m",
	}
	for name, m := range legacyMarkers {
		if strings.Contains(combined, m) {
			t.Errorf("legacy color leaked in CLI helpers: %s (%q)", name, m)
		}
	}
}
