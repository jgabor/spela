package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/profile"
)

func TestPickerLeadsWithFollowAndTheValueItWouldGive(t *testing.T) {
	defaults := &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}
	defaults.MarkOverride(profile.FieldProtonEnableHDR)
	game := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, defaults)
	focusField(t, &game, profile.FieldProtonEnableHDR)
	game.BeginEdit()
	labels, cursor := game.editor.ChoiceRows()
	if got := strings.Join(labels, "|"); got != "Follow All games (On)|On|Off" || cursor != 0 {
		t.Fatalf("game picker = %q at %d", got, cursor)
	}

	root := NewRootDetail(NewStyles(DefaultTheme), &profile.Profile{})
	focusField(t, &root, profile.FieldProtonEnableHDR)
	root.BeginEdit()
	if labels, _ := root.editor.ChoiceRows(); labels[0] != "Use system default (Off)" {
		t.Fatalf("All games picker starts with %q", labels[0])
	}
}

func TestPickerStartsOnTheOwnValueAndCanFollowAgain(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, &profile.Profile{})
	focusField(t, &detail, profile.FieldDLSSSRPreset)
	if err := detail.RawProfile().Set(profile.FieldDLSSSRPreset, "K"); err != nil {
		t.Fatal(err)
	}
	detail.rebuildResolved()
	detail.BeginEdit()
	labels, cursor := detail.editor.ChoiceRows()
	if labels[cursor] != "K" {
		t.Fatalf("picker opened on %q, want the own value K", labels[cursor])
	}
	if !pickOption(&detail, "f") || detail.RawProfile().IsOverridden(profile.FieldDLSSSRPreset) {
		t.Fatal("picking Follow All games did not clear the value")
	}
	if got := detail.LastChange(); got != "Resolution preset follows All games" {
		t.Fatalf("change summary = %q", got)
	}
}

func TestPickerTypeAheadJumpsToTheFirstMatchAndRestartsOnAMiss(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, &profile.Profile{})
	focusField(t, &detail, profile.FieldDLSSSRPreset)
	detail.BeginEdit()
	for _, step := range []struct{ text, want string }{{"k", "K"}, {"d", "default"}} {
		detail.editor.TypeAhead(step.text)
		if detail.editor.Value() != step.want {
			t.Fatalf("typing %q selected %q, want %q", step.text, detail.editor.Value(), step.want)
		}
	}
	detail.editor.TypeAhead("e")
	detail.editor.TypeAhead("f")
	if detail.editor.Value() != "default" {
		t.Fatalf("typing def selected %q", detail.editor.Value())
	}
	detail.editor.TypeAhead("z")
	detail.editor.TypeAhead("l")
	if detail.editor.Value() != "L" {
		t.Fatalf("a miss did not restart the search: selected %q", detail.editor.Value())
	}
}

func TestPickerFitsASmallPaneAndKeepsTheCurrentChoiceVisible(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, &profile.Profile{})
	detail.SetSize(70, 9)
	focusField(t, &detail, profile.FieldDLSSSRPreset)
	detail.BeginEdit()
	detail.editor.TypeAhead("m")
	view := stripANSI(detail.View())
	if lines := strings.Split(view, "\n"); len(lines) > 9 {
		t.Fatalf("picker is %d lines in a 9 line pane:\n%s", len(lines), view)
	}
	for _, text := range []string{"Edit Resolution preset", "> M", "Input", "Esc cancel", "Type to jump"} {
		if !strings.Contains(view, text) {
			t.Fatalf("picker missing %q:\n%s", text, view)
		}
	}
}

func TestPickerKeysReachTheEditorThroughTheLayout(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	layout.focus = FocusDetail
	focusField(t, &layout.pane.content.detail, profile.FieldDLSSSRPreset)
	sessionKey(&layout, "enter")
	if !layout.pane.content.detail.Editing() {
		t.Fatal("Enter did not open the picker")
	}
	sessionKey(&layout, "k")
	sessionKey(&layout, "down")
	sessionKey(&layout, "up")
	if got := layout.pane.content.detail.editor.Value(); got != "K" {
		t.Fatalf("after k, down, up the choice is %q, want K", got)
	}
	command := sessionKey(&layout, "enter")
	if command == nil || layout.pane.content.detail.RawProfile().DLSS.SRPreset != "K" {
		t.Fatal("Enter did not apply the picked value and request a save")
	}
}

func TestPickerRepeatedLetterMovesToTheNextMatch(t *testing.T) {
	for _, test := range []struct {
		field         string
		letter        string
		first, second string
	}{
		{profile.FieldDLSSSRPreset, "a", "auto", "A"},
		{profile.FieldDLSSSRPreset, "d", "default", "D"},
		{profile.FieldDLSSSRPreset, "f", "\x00follow", "F"},
		{profile.FieldDLSSRRPreset, "d", "default", "D"},
	} {
		t.Run(test.field+"/"+test.letter, func(t *testing.T) {
			detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, &profile.Profile{})
			focusField(t, &detail, test.field)
			detail.BeginEdit()
			detail.editor.TypeAhead(test.letter)
			if detail.editor.Value() != test.first {
				t.Fatalf("first %q selected %q, want %q", test.letter, detail.editor.Value(), test.first)
			}
			detail.editor.TypeAhead(test.letter)
			if detail.editor.Value() != test.second {
				t.Fatalf("second %q selected %q, want %q", test.letter, detail.editor.Value(), test.second)
			}
		})
	}
}

func TestPickerRepeatedLetterWrapsAndAMultiLetterSearchStillNarrows(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, &profile.Profile{})
	focusField(t, &detail, profile.FieldDLSSSRPreset)
	detail.BeginEdit()
	for _, letter := range []string{"a", "a", "a"} {
		detail.editor.TypeAhead(letter)
	}
	if detail.editor.Value() != "auto" {
		t.Fatalf("three a presses selected %q, want a wrap back to auto", detail.editor.Value())
	}
	detail.editor.TypeAhead("u")
	if detail.editor.Value() != "auto" {
		t.Fatalf("typing au selected %q", detail.editor.Value())
	}
}
