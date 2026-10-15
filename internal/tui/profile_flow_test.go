package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func TestGameChangeKeepsTheFieldUnderTheCursor(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	layout.focus = FocusDetail
	focusField(t, &layout.pane.content.detail, profile.FieldGPUClockOffset)

	sessionSelect(t, &layout, 1245620)
	if got := layout.pane.content.detail.FocusedField(); got != profile.FieldGPUClockOffset {
		t.Fatalf("field after a game change = %q, want %q", got, profile.FieldGPUClockOffset)
	}
	if layout.navState.Aspect != nav.AspectProfile {
		t.Fatalf("aspect after a game change = %v, want Profile", layout.navState.Aspect)
	}
}

func TestLeftAndRightJumpBetweenSectionsAndReachDLLs(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	layout.focus = FocusDetail
	detail := func() *DetailModel { return layout.pane.profileDetail() }

	if got := detail().FocusedField(); got != profile.FieldProtonEnableHDR {
		t.Fatalf("a game opens on %q, want the first field", got)
	}
	sessionKey(&layout, "left")
	if got := detail().CurrentSection(); got != "proton" {
		t.Fatalf("Left from the first section moved to %q", got)
	}
	want := []string{profile.FieldDLSSSRMode, profile.FieldGPUClockOffset, profile.FieldCPUGovernor, profile.FieldOverlayEnabled}
	for _, field := range want {
		sessionKey(&layout, "right")
		if got := detail().FocusedField(); got != field {
			t.Fatalf("Right moved to %q, want %q", got, field)
		}
	}
	sessionKey(&layout, "right")
	if layout.navState.Aspect != nav.AspectDLLs {
		t.Fatalf("Right from the last section left aspect %v, want DLLs", layout.navState.Aspect)
	}
	sessionKey(&layout, "right")
	if layout.navState.Aspect != nav.AspectDLLs {
		t.Fatal("Right moved past the DLLs section")
	}
	sessionKey(&layout, "left")
	if layout.navState.Aspect != nav.AspectProfile || detail().FocusedField() != profile.FieldOverlayEnabled {
		t.Fatalf("Left from DLLs = %v at %q, want Profile at the first Overlay field", layout.navState.Aspect, detail().FocusedField())
	}
}

func TestAllGamesSectionJumpsStopAtTheLastSection(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 0)
	layout.focus = FocusDetail
	for range 6 {
		sessionKey(&layout, "right")
	}
	if got := layout.pane.defaultsDetail.CurrentSection(); got != "overlay" {
		t.Fatalf("All games ended in section %q, want overlay", got)
	}
	if layout.navState.Scope.Kind != nav.ScopeGlobal || layout.navState.Aspect != nav.AspectProfile {
		t.Fatalf("All games left the profile page: scope %v aspect %v", layout.navState.Scope.Kind, layout.navState.Aspect)
	}
}

func TestHeaderNamesTheGameItsOwnSettingsAndTheSections(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	view := stripANSI(layout.pane.View(true))
	for _, text := range []string{"Cyberpunk 2077", "App ID 1091500", "Saved at once", "Own settings: none", "[Proton]", "DLSS", "GPU", "CPU", "Overlay", "DLLs"} {
		if !strings.Contains(view, text) {
			t.Fatalf("header missing %q:\n%s", text, view)
		}
	}
	sessionSelect(t, &layout, 0)
	view = stripANSI(layout.pane.View(true))
	if strings.Contains(view, "DLLs") {
		t.Fatalf("the All games page offers a DLLs section:\n%s", view)
	}
}

func TestActionsMenuStartsOnTheFirstRowThatCanRun(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	layout.focus = FocusDetail
	focusField(t, layout.pane.profileDetail(), profile.FieldProtonEnableHDR)

	sessionKey(&layout, "0")
	items := layout.menuBindings()
	if items[0].Binding.Action != ActionDetailReset || items[0].Available || items[0].Reason != "nothing to clear" {
		t.Fatalf("Follow on a row that already follows = %+v, want unavailable with a reason", items[0])
	}
	if got := items[layout.actions.cursor].Binding.Action; got != ActionDetailResetAll {
		t.Fatalf("the menu opened on %q, want the first row that can run", got)
	}
	sessionKey(&layout, "tab")
	sessionKey(&layout, "enter")

	changeHDR(t, &layout)
	sessionKey(&layout, "0")
	items = layout.menuBindings()
	if !items[0].Available || layout.actions.cursor != 0 {
		t.Fatalf("Follow on a row with its own value = %+v at cursor %d, want the first row", items[0], layout.actions.cursor)
	}
}

func TestSectionKeysInTheKeyBarMatchWhatTheyDo(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	layout.focus = FocusDetail
	advertised := func(action KeyAction) bool {
		return hasHelpAction(CanonicalKeymap.HelpBindings(layout.bindingContext()), action)
	}
	if advertised(ActionDetailPrevious) || !advertised(ActionDetailNext) {
		t.Fatal("the first section advertised Left, or hid Right")
	}
	for range 4 {
		sessionKey(&layout, "right")
	}
	if !advertised(ActionDetailPrevious) || !advertised(ActionDetailNext) {
		t.Fatal("the last profile section must advertise Left and Right (to DLLs)")
	}
	sessionKey(&layout, "right")
	if layout.navState.Aspect != nav.AspectDLLs || !advertised(ActionDetailPrevious) || advertised(ActionDetailNext) {
		t.Fatalf("the DLLs section advertised a Right that does nothing (aspect %v)", layout.navState.Aspect)
	}

	sessionSelect(t, &layout, 0)
	layout.focus = FocusDetail
	for range 4 {
		sessionKey(&layout, "right")
	}
	if !advertised(ActionDetailPrevious) || advertised(ActionDetailNext) {
		t.Fatal("the last section of All games advertised a Right that does nothing")
	}
}

func TestQuitAfterAFailedWriteStaysOpenWithTheError(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", blocked)
	results := saveResults(t, changeHDR(t, &layout))
	layout, _ = layout.requestQuit()
	if !layout.quitWhenIdle {
		t.Fatal("Quit did not wait for the write")
	}
	var after []tea.Msg
	for _, result := range results {
		after = append(after, immediateMessages(finish(&layout, result))...)
	}
	if containsQuit(after) || layout.quitWhenIdle {
		t.Fatal("the program quit over a write that failed, hiding the error")
	}
	if status := layout.pane.profileDetail().SaveStatus(); !strings.HasPrefix(status, "Not saved:") {
		t.Fatalf("status after the failed write = %q", status)
	}
}
