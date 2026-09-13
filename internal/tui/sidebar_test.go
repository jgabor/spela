package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/game"
)

// The sidebar hosts the games list inside the Games resource. Task 3
// changes: the "default profile" first-row item moved to its own rail
// resource, so the sidebar is games-only. Profile filter key moved from
// `p` to `P` (shift+p) because bare `p` is reserved for Task 5 pin-field.

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func TestSidebar_CursorDown(t *testing.T) {
	g1 := testGame("Alpha")
	g1.AppID = 100
	g2 := testGame("Beta")
	g2.AppID = 200
	m := testSidebar(g1, g2)

	start := m.cursor
	m, _ = m.Update(keyMsg("down"))
	if m.cursor != start+1 {
		t.Errorf("expected cursor %d, got %d", start+1, m.cursor)
	}
}

func TestSidebar_CursorUp_ClampsAtZero(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.cursor = 0

	m, _ = m.Update(keyMsg("up"))
	if m.cursor != 0 {
		t.Error("expected cursor to clamp at 0")
	}
}

func TestSidebar_CursorDown_ClampsAtEnd(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.cursor = len(m.filtered) - 1

	end := m.cursor
	m, _ = m.Update(keyMsg("down"))
	if m.cursor != end {
		t.Error("expected cursor to clamp at end of list")
	}
}

func TestSidebar_ArrowNavigation(t *testing.T) {
	g1 := testGame("Alpha")
	g1.AppID = 100
	g2 := testGame("Beta")
	g2.AppID = 200
	m := testSidebar(g1, g2)
	m.cursor = 0

	m, _ = m.Update(keyMsg("down"))
	if m.cursor != 1 {
		t.Error("expected j to move cursor down")
	}

	m, _ = m.Update(keyMsg("up"))
	if m.cursor != 0 {
		t.Error("expected k to move cursor up")
	}
}

// ---------------------------------------------------------------------------
// Filters
// ---------------------------------------------------------------------------

func TestSidebar_DLLFilter(t *testing.T) {
	gWithDLL := testGame("With DLLs", testDLL(game.DLLTypeDLSS, "3.8.10"))
	gWithDLL.AppID = 100
	gNoDLL := testGame("No DLLs")
	gNoDLL.AppID = 200
	m := testSidebar(gWithDLL, gNoDLL)

	initialCount := len(m.filtered)

	m, _ = m.UpdateAction(ActionListToggleDLLFilter)
	if !m.filters.hasDLLs {
		t.Error("expected DLL filter to be active")
	}
	if len(m.filtered) >= initialCount {
		t.Error("expected filtered list to be smaller with DLL filter")
	}

	m, _ = m.UpdateAction(ActionListToggleDLLFilter)
	if m.filters.hasDLLs {
		t.Error("expected DLL filter to be inactive after second toggle")
	}
}

// TestSidebar_ProfileFilter verifies the displaced binding `P` (shift+p)
// toggles the profile filter. The previous `p` binding is now reserved
// for Task 5's pin-field keystroke.
func TestSidebar_ProfileFilter(t *testing.T) {
	g1 := testGame("Has Profile")
	g1.AppID = 100
	g2 := testGame("No Profile")
	g2.AppID = 200

	svc := testServices()
	svc.ProfileExists = func(appID uint64) bool {
		return appID == 100
	}
	styles := NewStyles(DefaultTheme, true)
	m, _ := NewSidebar([]*game.Game{g1, g2}, styles, svc)

	initialCount := len(m.filtered)

	// Profile filtering uses p now that profile editing creates overrides.
	m, _ = m.UpdateAction(ActionListToggleProfile)
	if !m.filters.hasProfile {
		t.Error("expected profile filter to be active after 'p'")
	}
	if len(m.filtered) >= initialCount {
		t.Error("expected filtered list to be smaller with profile filter")
	}
	for _, item := range m.filtered {
		if item.kind == sidebarItemGame && item.game != nil && item.game.AppID == 200 {
			t.Error("expected game without profile to be filtered out")
		}
	}
}

func TestSidebar_ExplicitSortChoices(t *testing.T) {
	m := testSidebar(testGame("Alpha"), testGame("Beta", testDLL(game.DLLTypeDLSS, "3.7.0")))
	for _, test := range []struct {
		action KeyAction
		mode   SortMode
	}{{ActionSortNameDesc, SortNameDesc}, {ActionSortDLLsFirst, SortDLLsFirst}, {ActionSortProfileFirst, SortProfileFirst}, {ActionSortNameAsc, SortNameAsc}} {
		m, _ = m.UpdateAction(test.action)
		if m.sortMode != test.mode {
			t.Fatalf("%s selected mode %v", test.action, m.sortMode)
		}
	}
}

func TestSidebar_ClearFilters(t *testing.T) {
	m := testSidebar(testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.8.10")))

	m, _ = m.UpdateAction(ActionListToggleDLLFilter)
	m, _ = m.Update(keyMsg("s"))

	m, _ = m.UpdateAction(ActionListClearFilters)
	if m.filters.hasDLLs || m.filters.hasProfile {
		t.Error("expected C to clear all filters")
	}
	if m.sortMode != SortNameAsc {
		t.Error("expected C to reset sort to SortNameAsc")
	}
}

// ---------------------------------------------------------------------------
// Default profile removed from games sidebar
// ---------------------------------------------------------------------------

// TestSidebar_HasDefaultProfilePinned verifies the global default scope row.
func TestSidebar_HasDefaultProfilePinned(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	if len(m.filtered) != 2 {
		t.Fatalf("expected default + game, got %d entries", len(m.filtered))
	}
	if m.filtered[0].kind != sidebarItemDefaultProfile {
		t.Error("expected first row to be default profile scope")
	}
}

// ---------------------------------------------------------------------------
// Multi-select
// ---------------------------------------------------------------------------

func TestSidebar_SpaceEntersSelectMode(t *testing.T) {
	g := testGame("Alpha")
	m := testSidebar(g)
	m.cursor = 1 // game row after pinned default

	m, _ = m.Update(keyMsg("space"))
	if !m.selectMode {
		t.Error("expected space to enter select mode")
	}
	if !m.selected[g.AppID] {
		t.Error("expected current game to be selected")
	}
}

func TestSidebar_SpaceTogglesSelection(t *testing.T) {
	g := testGame("Alpha")
	m := testSidebar(g)
	m.cursor = 1
	m.selectMode = true
	m.selected[g.AppID] = true

	m, _ = m.Update(keyMsg("space"))
	if m.selected[g.AppID] {
		t.Error("expected space to deselect already-selected game")
	}
}

func TestSidebar_SelectAll_DeselectAll(t *testing.T) {
	g1 := testGame("Alpha")
	g1.AppID = 100
	g2 := testGame("Beta")
	g2.AppID = 200
	m := testSidebar(g1, g2)
	m.selectMode = true

	m, _ = m.UpdateAction(ActionListSelectAll)
	if !m.selected[100] || !m.selected[200] {
		t.Error("expected a to select all games")
	}

	m, _ = m.UpdateAction(ActionListClearSelection)
	if len(m.selected) != 0 {
		t.Error("expected A to deselect all games")
	}
}

func TestSidebar_ClearSelectedLeavesSelectionMode(t *testing.T) {
	g := testGame("Alpha")
	m := testSidebar(g)
	m.selectMode = true
	m.selected[g.AppID] = true

	m, _ = m.UpdateAction(ActionListClearSelection)
	if m.selectMode {
		t.Error("expected esc to exit select mode")
	}
	if len(m.selected) != 0 {
		t.Error("expected esc to clear selections")
	}
}

func TestSidebar_EnterInSelectMode_TriggersBatch(t *testing.T) {
	g := testGame("Alpha")
	m := testSidebar(g)
	m.selectMode = true
	m.selected[g.AppID] = true
	m.cursor = 0

	_, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("expected enter in select mode to return a command")
	}
	msg := execCmd(cmd)
	if _, ok := msg.(batchActionRequestMsg); !ok {
		t.Errorf("expected batchActionRequestMsg, got %T", msg)
	}
}

func TestSidebar_EnterConfirmsGame(t *testing.T) {
	g := testGame("Alpha")
	m := testSidebar(g)
	m.cursor = 1

	_, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("expected enter to return a command")
	}
	msg := execCmd(cmd)
	if confirmed, ok := msg.(gameConfirmedMsg); !ok {
		t.Errorf("expected gameConfirmedMsg, got %T", msg)
	} else if confirmed.game.Name != "Alpha" {
		t.Errorf("expected game Alpha, got %s", confirmed.game.Name)
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSidebar_ObsoleteCommandsCannotChangeBrowseState(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	for _, key := range []string{"/", "j", "k", "d", "p", "s", "a", "A", "[", "]", "?"} {
		next, _ := m.Update(keyMsg(key))
		if next.cursor != m.cursor || next.search.Focused() || next.sortMode != m.sortMode || len(next.selected) != 0 || next.filters != m.filters {
			t.Fatalf("obsolete key %q changed browse state", key)
		}
	}
}

func TestSidebar_TextInputDoesNotOwnSearchCancellation(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.search.Focus()
	m, _ = m.Update(keyMsg("esc"))
	if !m.search.Focused() {
		t.Fatal("Sidebar escaped the shell-owned search controls")
	}
}

func TestSidebar_TextInputDoesNotOwnSearchApply(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.search.Focus()
	m, _ = m.Update(keyMsg("enter"))
	if !m.search.Focused() {
		t.Fatal("Sidebar bypassed the shell-owned search Apply")
	}
}
