package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

const (
	cyberpunkAppID = 1091500
	eldenRingAppID = 1245620
)

// seedOwnValues stores a profile that sets HDR itself for each game, and
// Wayland too for the second, and sets HDR on All games. The page reads All
// games again, as it does when it is opened.
func seedOwnValues(t *testing.T, layout *LayoutModel) {
	t.Helper()
	defer layout.pane.refreshDefaultsDetail()
	if err := profile.SaveDefault(&profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}); err != nil {
		t.Fatal(err)
	}
	for appID, name := range map[uint64]string{cyberpunkAppID: "Cyberpunk 2077", eldenRingAppID: "Elden Ring"} {
		own := &profile.Profile{Name: name, Proton: profile.ProtonSettings{EnableHDR: false}}
		own.MarkOverride(profile.FieldProtonEnableHDR)
		if appID == eldenRingAppID {
			own.Proton.EnableWayland = true
			own.MarkOverride(profile.FieldProtonEnableWayland)
		}
		if err := profile.Save(appID, own); err != nil {
			t.Fatal(err)
		}
	}
}

// openActionsOn opens All games, focuses the field, and opens the Actions menu.
func openActionsOn(t *testing.T, layout *LayoutModel, field string) []BindingResolution {
	t.Helper()
	sessionSelect(t, layout, 0)
	layout.focus = FocusDetail
	focusField(t, layout.pane.profileDetail(), field)
	sessionKey(layout, "0")
	return layout.menuBindings()
}

func menuRow(items []BindingResolution, action KeyAction) (int, BindingResolution) {
	for index, item := range items {
		if item.Binding.Action == action {
			return index, item
		}
	}
	return -1, BindingResolution{}
}

func TestApplyToGamesWithOwnValueIsOfferedOnAllGamesWhenAGameSetsTheField(t *testing.T) {
	layout := sessionLayout(t)
	seedOwnValues(t, &layout)
	items := openActionsOn(t, &layout, profile.FieldProtonEnableHDR)
	_, row := menuRow(items, ActionApplyOwnValues)
	if row.Binding.Description != profile.ActionApplyToGamesWithOwnValue || !row.Available {
		t.Fatalf("on HDR the action = %q available %v (%s), want available", row.Binding.Description, row.Available, row.Reason)
	}

	sessionKey(&layout, "esc")
	focusField(t, layout.pane.profileDetail(), profile.FieldProtonEnableNGXUpdater)
	sessionKey(&layout, "0")
	_, row = menuRow(layout.menuBindings(), ActionApplyOwnValues)
	if row.Available || row.Reason != "no game has its own value" {
		t.Fatalf("on a field no game sets: available %v, reason %q", row.Available, row.Reason)
	}
}

func TestApplyToGamesWithOwnValueIsNotOnAGamePage(t *testing.T) {
	layout := sessionLayout(t)
	seedOwnValues(t, &layout)
	sessionSelect(t, &layout, cyberpunkAppID)
	layout.focus = FocusDetail
	sessionKey(&layout, "0")
	if index, _ := menuRow(layout.menuBindings(), ActionApplyOwnValues); index >= 0 {
		t.Fatal("a game page offers the All games action")
	}
}

func TestApplyToGamesWithOwnValueAsksThenFollowsOnlyThatField(t *testing.T) {
	layout := sessionLayout(t)
	seedOwnValues(t, &layout)
	items := openActionsOn(t, &layout, profile.FieldProtonEnableHDR)
	index, _ := menuRow(items, ActionApplyOwnValues)
	layout.actions.cursor = index
	sessionKey(&layout, "enter")

	dialog := layout.decision
	if dialog == nil || dialog.title != profile.ActionApplyToGamesWithOwnValue {
		t.Fatalf("the action did not ask first: %+v", dialog)
	}
	for _, want := range []string{"All games", "HDR: 2 games have their own value: Cyberpunk 2077, Elden Ring", "They will follow All games (On)", "cannot be undone"} {
		if !strings.Contains(dialog.body, want) {
			t.Errorf("dialog body is missing %q:\n%s", want, dialog.body)
		}
	}
	if stored, _ := profile.Load(cyberpunkAppID); !stored.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatal("asking changed a profile")
	}

	dialog.focus = 1
	command := sessionKey(&layout, "enter")
	if layout.decision != nil || !layout.savesPending() {
		t.Fatalf("confirming left the dialog open or did not mark the write: decision %v pending %v", layout.decision != nil, layout.savesPending())
	}
	var applied ownValuesAppliedMsg
	for _, message := range immediateMessages(command) {
		if result, ok := message.(ownValuesAppliedMsg); ok {
			applied = result
		}
	}
	if len(applied.outcome.Followed) != 2 || len(applied.outcome.Failed) != 0 {
		t.Fatalf("outcome = %+v, want both games followed", applied.outcome)
	}
	finish(&layout, applied)
	if layout.savesPending() {
		t.Fatal("the write is still marked as running")
	}
	if got := layout.messageBar.message; got != "Saved: HDR: 2 games follow All games" {
		t.Errorf("message = %q", got)
	}

	cyberpunk, _ := profile.Load(cyberpunkAppID)
	eldenRing, _ := profile.Load(eldenRingAppID)
	if cyberpunk.IsOverridden(profile.FieldProtonEnableHDR) || eldenRing.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Error("a game still sets HDR itself")
	}
	if !eldenRing.IsOverridden(profile.FieldProtonEnableWayland) || !eldenRing.Proton.EnableWayland {
		t.Error("another field of the game changed")
	}
	if got := layout.pane.defaultsDetail.FocusedOwnValueGameCount(); got != 0 {
		t.Errorf("the count on screen is still %d", got)
	}

	sessionSelect(t, &layout, eldenRingAppID)
	view := stripANSI(layout.pane.View(true))
	if !strings.Contains(view, "from All games") {
		t.Errorf("the game page opened after the action does not follow All games:\n%s", view)
	}
}

func TestApplyToGamesWithOwnValueCancelChangesNothing(t *testing.T) {
	layout := sessionLayout(t)
	seedOwnValues(t, &layout)
	items := openActionsOn(t, &layout, profile.FieldProtonEnableHDR)
	index, _ := menuRow(items, ActionApplyOwnValues)
	layout.actions.cursor = index
	sessionKey(&layout, "enter")
	layout.decision.focus = 0
	if command := sessionKey(&layout, "enter"); command != nil || layout.decision != nil || layout.applyingOwnValues {
		t.Fatalf("Cancel started work or left the dialog: command %v decision %v applying %v", command != nil, layout.decision != nil, layout.applyingOwnValues)
	}
	if stored, _ := profile.Load(cyberpunkAppID); !stored.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatal("Cancel changed a profile")
	}
}

func TestApplyToGamesWithOwnValueNamesAGameThatFailsAndKeepsTheRest(t *testing.T) {
	layout := sessionLayout(t)
	seedOwnValues(t, &layout)
	layout.services.FollowAllGames = func(entries []profile.OwnValueEntry, fields []string) profile.FollowOutcome {
		return profile.FollowOutcome{Followed: []string{"Cyberpunk 2077"}, Failed: []string{"Elden Ring"}, Errors: map[string]error{"Elden Ring": errors.New("disk full")}}
	}
	layout.pane.services = layout.services
	items := openActionsOn(t, &layout, profile.FieldProtonEnableHDR)
	index, _ := menuRow(items, ActionApplyOwnValues)
	layout.actions.cursor = index
	sessionKey(&layout, "enter")
	layout.decision.focus = 1
	var applied ownValuesAppliedMsg
	for _, message := range immediateMessages(sessionKey(&layout, "enter")) {
		if result, ok := message.(ownValuesAppliedMsg); ok {
			applied = result
		}
	}
	finish(&layout, applied)
	if got, want := layout.messageBar.message, "Not saved for Elden Ring (disk full). HDR: 1 game follows All games"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if layout.messageBar.messageType != MessageError {
		t.Error("a partial failure is not shown as an error")
	}
}

func TestApplyToGamesWithOwnValueWaitsForSaves(t *testing.T) {
	layout := sessionLayout(t)
	seedOwnValues(t, &layout)
	openActionsOn(t, &layout, profile.FieldProtonEnableHDR)
	sessionKey(&layout, "esc")
	layout.pane.content.profileSaves.active = &profileSaveRequest{appID: cyberpunkAppID}
	sessionKey(&layout, "0")
	_, row := menuRow(layout.menuBindings(), ActionApplyOwnValues)
	if row.Available || row.Reason != "saving" {
		t.Fatalf("with a save running the action = available %v, reason %q", row.Available, row.Reason)
	}
	if layout.navState.Scope.Kind != nav.ScopeGlobal {
		t.Fatal("test left All games")
	}
}

func TestQuitWaitsForAnApplyToGamesWithOwnValueInFlight(t *testing.T) {
	layout := sessionLayout(t)
	layout.applyingOwnValues = true
	next, _ := layout.requestQuit()
	if !next.quitWhenIdle {
		t.Fatal("Quit did not wait for the write")
	}
	finish(&next, ownValuesAppliedMsg{label: "HDR"})
	if next.applyingOwnValues {
		t.Fatal("the write is still marked as running after it finished")
	}
}
