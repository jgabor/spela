package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/game"
)

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

func TestSidebar_JK_Navigation(t *testing.T) {
	g1 := testGame("Alpha")
	g1.AppID = 100
	g2 := testGame("Beta")
	g2.AppID = 200
	m := testSidebar(g1, g2)
	m.cursor = 0

	m, _ = m.Update(keyMsg("j"))
	if m.cursor != 1 {
		t.Error("expected j to move cursor down")
	}

	m, _ = m.Update(keyMsg("k"))
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

	m, _ = m.Update(keyMsg("d"))
	if !m.filters.hasDLLs {
		t.Error("expected DLL filter to be active")
	}
	if len(m.filtered) >= initialCount {
		t.Error("expected filtered list to be smaller with DLL filter")
	}

	m, _ = m.Update(keyMsg("d"))
	if m.filters.hasDLLs {
		t.Error("expected DLL filter to be inactive after second toggle")
	}
}

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

	m, _ = m.Update(keyMsg("p"))
	if !m.filters.hasProfile {
		t.Error("expected profile filter to be active")
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

func TestSidebar_SortCycles(t *testing.T) {
	m := testSidebar(testGame("Alpha"))

	if m.sortMode != SortNameAsc {
		t.Fatal("precondition: should start with SortNameAsc")
	}

	m, _ = m.Update(keyMsg("s"))
	if m.sortMode != SortNameDesc {
		t.Errorf("expected SortNameDesc, got %d", m.sortMode)
	}

	m, _ = m.Update(keyMsg("s"))
	if m.sortMode != SortDLLsFirst {
		t.Errorf("expected SortDLLsFirst, got %d", m.sortMode)
	}

	m, _ = m.Update(keyMsg("s"))
	if m.sortMode != SortProfileFirst {
		t.Errorf("expected SortProfileFirst, got %d", m.sortMode)
	}

	m, _ = m.Update(keyMsg("s"))
	if m.sortMode != SortNameAsc {
		t.Errorf("expected SortNameAsc after full cycle, got %d", m.sortMode)
	}
}

func TestSidebar_ClearFilters(t *testing.T) {
	m := testSidebar(testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.8.10")))

	m, _ = m.Update(keyMsg("d"))
	m, _ = m.Update(keyMsg("s"))

	m, _ = m.Update(keyMsg("C"))
	if m.filters.hasDLLs || m.filters.hasProfile {
		t.Error("expected C to clear all filters")
	}
	if m.sortMode != SortNameAsc {
		t.Error("expected C to reset sort to SortNameAsc")
	}
}

// ---------------------------------------------------------------------------
// Multi-select
// ---------------------------------------------------------------------------

func TestSidebar_SpaceEntersSelectMode(t *testing.T) {
	g := testGame("Alpha")
	m := testSidebar(g)
	m.cursor = 1 // game item
	if m.filtered[1].kind != sidebarItemGame {
		t.Fatal("precondition: cursor should be on a game item")
	}

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

func TestSidebar_SpaceOnDefaultProfileIgnored(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.cursor = 0

	m, _ = m.Update(keyMsg("space"))
	if m.selectMode {
		t.Error("expected space on default profile to be ignored")
	}
}

func TestSidebar_SelectAll_DeselectAll(t *testing.T) {
	g1 := testGame("Alpha")
	g1.AppID = 100
	g2 := testGame("Beta")
	g2.AppID = 200
	m := testSidebar(g1, g2)
	m.selectMode = true

	m, _ = m.Update(keyMsg("a"))
	if !m.selected[100] || !m.selected[200] {
		t.Error("expected a to select all games")
	}

	m, _ = m.Update(keyMsg("A"))
	if len(m.selected) != 0 {
		t.Error("expected A to deselect all games")
	}
}

func TestSidebar_EscExitsSelectMode(t *testing.T) {
	g := testGame("Alpha")
	m := testSidebar(g)
	m.selectMode = true
	m.selected[g.AppID] = true

	m, _ = m.Update(keyMsg("esc"))
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
	m.cursor = 1

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

func TestSidebar_EnterOnDefaultProfile(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.cursor = 0

	_, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("expected enter to return a command")
	}
	msg := execCmd(cmd)
	if _, ok := msg.(defaultProfileConfirmedMsg); !ok {
		t.Errorf("expected defaultProfileConfirmedMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSidebar_SlashActivatesSearch(t *testing.T) {
	m := testSidebar(testGame("Alpha"))

	m, _ = m.Update(keyMsg("/"))
	if !m.search.Focused() {
		t.Error("expected / to activate search")
	}
}

func TestSidebar_SearchEscBlurs(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.search.Focus()

	m, _ = m.Update(keyMsg("esc"))
	if m.search.Focused() {
		t.Error("expected esc to blur search")
	}
}

func TestSidebar_SearchEnterBlurs(t *testing.T) {
	m := testSidebar(testGame("Alpha"))
	m.search.Focus()

	m, _ = m.Update(keyMsg("enter"))
	if m.search.Focused() {
		t.Error("expected enter to blur search")
	}
}
