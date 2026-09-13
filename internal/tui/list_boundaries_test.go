package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
)

func TestDLLResourceSizingAndTruncationBoundaryContracts(t *testing.T) {
	resource := NewDLLsResource(NewStyles(DefaultTheme, true), nil)
	resource.width = 20
	resource.typesInUse = nil
	if resource.typeColumnWidth() != 8 {
		t.Fatalf("empty type column width = %d", resource.typeColumnWidth())
	}
	resource.width = 0
	resource.typesInUse = resource.knownDLLTypes()[:1]
	if resource.typeColumnWidth() != 12 {
		t.Fatalf("default type column width = %d", resource.typeColumnWidth())
	}
	for _, test := range []struct {
		got, want string
	}{
		{shortenLabel("long", 1), "l"},
		{shortenLabel("long", 3), "lo…"},
		{truncate("long", 0), ""},
		{truncate("long", 1), "l"},
		{truncate("long", 3), "lo…"},
		{padRight("long", 2), "long"},
	} {
		if test.got != test.want {
			t.Errorf("text helper = %q, want %q", test.got, test.want)
		}
	}
}

func TestSidebarSupportedSortSearchAndSelectionBoundaryContracts(t *testing.T) {
	first := &game.Game{AppID: 1, Name: "Alpha"}
	second := &game.Game{AppID: 2, Name: "Beta", DLLs: []game.DetectedDLL{{Type: game.DLLTypeDLSS}}}
	third := &game.Game{AppID: 3, Name: "Gamma"}
	services := testServices()
	services.ProfileExists = func(appID uint64) bool { return appID == 1 }
	sidebar, _ := NewSidebar([]*game.Game{third, second, first}, NewStyles(DefaultTheme, true), services)
	for _, mode := range []SortMode{SortNameDesc, SortDLLsFirst, SortProfileFirst} {
		sidebar.sortMode = mode
		sidebar.applyFiltersAndSort()
		if view := stripANSI(sidebar.View()); view == "" || !strings.Contains(view, sortModeNames[mode]) {
			t.Fatalf("sort mode %d view:\n%s", mode, view)
		}
	}
	sidebar.search.SetValue("missing")
	sidebar.search.Blur()
	sidebar.applyFiltersAndSort()
	sidebar, _ = sidebar.UpdateAction(ActionListClearFilters)
	if sidebar.search.Value() != "" {
		t.Fatal("Clear filters did not clear inactive search")
	}
	sidebar.filters.hasDLLs = true
	sidebar, _ = sidebar.UpdateAction(ActionListClearFilters)
	if sidebar.filters.IsActive() {
		t.Fatal("Clear filters did not clear active filters")
	}
	sidebar.cursor = 0
	if command := sidebar.selectCurrentItem(); command == nil {
		t.Fatal("default selection returned no command")
	}
	sidebar.cursor = 1
	if command := sidebar.selectCurrentItem(); command == nil {
		t.Fatal("game selection returned no command")
	}
	sidebar.cursor = len(sidebar.filtered)
	command := sidebar.selectCurrentItem()
	if command == nil {
		t.Fatal("out-of-range selection returned no clearing command")
	}
	if _, ok := command().(noLibrarySelectionMsg); !ok {
		t.Fatal("out-of-range selection did not clear stale detail")
	}
}
