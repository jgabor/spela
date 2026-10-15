//go:build dev || production || bindings

package gui

import (
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

// The projection carries the shared words, so the graphical interface prints
// the same labels, sources, and effects as the terminal and the command line.
func TestGUIProfileSemanticsCarrySharedWords(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := profile.SaveDefault(&profile.Profile{GPU: profile.GPUSettings{ClockOffset: 150}}); err != nil {
		t.Fatal(err)
	}
	boundary := defaultGUIApplicationBoundary(nil)
	if err := boundary.patchGameProfile(1091500, []ProfilePatch{
		{Field: profile.FieldGPUClockOffset, Operation: "set", Value: float64(100)},
		{Field: profile.FieldProtonEnableHDR, Operation: "set", Value: true},
	}); err != nil {
		t.Fatal(err)
	}

	game := semanticsByField(boundary.profileView(appID(1091500))["semantics"].([]ProfileFieldSemantics))
	for field, want := range map[string]string{
		profile.FieldGPUClockOffset:         "this game · All games: 150 MHz",
		profile.FieldProtonEnableHDR:        "this game · All games: Off",
		profile.FieldGPUFanSpeed:            "system default",
		profile.FieldProtonVKD3DHeap:        "system default",
		profile.FieldDLSSSRMode:             "system default",
		profile.FieldProtonEnableNGXUpdater: "system default",
	} {
		if got := game[field].SourceText; got != want {
			t.Errorf("%s source text = %q, want %q", field, got, want)
		}
	}
	for _, descriptor := range profile.Fields() {
		item := game[descriptor.Key]
		if item.Label != descriptor.Label || item.Description != descriptor.Description {
			t.Errorf("%s: label %q, description %q do not come from the descriptor", descriptor.Key, item.Label, item.Description)
		}
	}

	allGames := semanticsByField(boundary.profileView(nil)["semantics"].([]ProfileFieldSemantics))
	if allGames[profile.FieldGPUClockOffset].SourceText != "set here" || allGames[profile.FieldGPUFanSpeed].SourceText != "system default" {
		t.Errorf("All games source text = %q and %q", allGames[profile.FieldGPUClockOffset].SourceText, allGames[profile.FieldGPUFanSpeed].SourceText)
	}
}

func TestGUIFieldActionsUseSharedActionWords(t *testing.T) {
	own := profile.FieldExplanation{Source: profile.ExplanationSourceOverride}
	inherited := profile.FieldExplanation{Source: profile.ExplanationSourceDefault}
	setHere := profile.FieldExplanation{Origin: profile.OriginSetHere}
	systemDefault := profile.FieldExplanation{Origin: profile.OriginSystemDefault}
	for _, test := range []struct {
		explanation profile.FieldExplanation
		allGames    bool
		want        string
	}{
		{own, false, profile.ActionFollowAllGames},
		{inherited, false, profile.ActionSetForThisGame},
		{setHere, true, profile.ActionUseSystemDefault},
		{systemDefault, true, profile.ActionSetHere},
	} {
		if got := fieldActionText(test.explanation, test.allGames); got != test.want {
			t.Errorf("action(%v, all games %t) = %q, want %q", test.explanation.Source, test.allGames, got, test.want)
		}
	}
}
