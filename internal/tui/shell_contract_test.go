package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/nav"
)

func TestShellStartsInLibraryListWithNonFocusableDestinationBar(t *testing.T) {
	layout := testLayout()
	if got := layout.navState.Destination; got != nav.DestinationLibrary {
		t.Fatalf("destination = %v, want Library", got)
	}
	if got := layout.focus; got != FocusList {
		t.Fatalf("focus = %v, want List", got)
	}
	bar := layout.destinationBar.View()
	if !strings.Contains(bar, "Library") || !strings.Contains(bar, "DLL Catalog") || !strings.Contains(bar, "Monitor") || !strings.Contains(bar, "Settings") {
		t.Fatalf("destination bar is incomplete: %q", bar)
	}

	result, _ := sendKey(&layout, "tab")
	updated := result.(LayoutModel)
	if updated.focus != FocusDetail {
		t.Fatalf("Tab focus = %v, want Detail", updated.focus)
	}
	result, _ = sendKey(&updated, "shift+tab")
	updated = result.(LayoutModel)
	if updated.focus != FocusList {
		t.Fatalf("reverse Tab focus = %v, want List", updated.focus)
	}
}

func TestLibrarySearchSynchronizesDisplayedScopeBeforeEnter(t *testing.T) {
	alpha := testGame("Alpha")
	beta := testGame("Beta")
	beta.AppID = 2
	layout := testLayout(alpha, beta)

	model, _ := sendKey(&layout, "/")
	model, command := sendKey(model, "b")
	updated := model.(LayoutModel)
	message := execCmd(command)
	if batch, ok := message.(tea.BatchMsg); ok {
		for _, child := range batch {
			if child == nil {
				continue
			}
			if childMessage := child(); childMessage != nil {
				next, _ := updated.Update(childMessage)
				updated = next.(LayoutModel)
			}
		}
	} else if message != nil {
		next, _ := updated.Update(message)
		updated = next.(LayoutModel)
	}

	if got := updated.listPane.sidebar.search.Value(); got != "b" {
		t.Fatalf("search text = %q", got)
	}
	if got := updated.navState.Scope.AppID; got != beta.AppID {
		t.Fatalf("displayed scope AppID = %d, want %d before Enter", got, beta.AppID)
	}
	if updated.pane.content.game == nil || updated.pane.content.game.AppID != beta.AppID {
		t.Fatal("Detail did not update with the displayed search result")
	}
}

func TestLibraryDetailNavigationCannotMoveHiddenList(t *testing.T) {
	game := testGame("Cyberpunk 2077")
	layout := testLayoutWithGame(game)
	listCursor := layout.listPane.sidebar.cursor

	result, _ := sendKey(&layout, "]")
	updated := result.(LayoutModel)
	if updated.navState.Aspect != nav.AspectProfile {
		t.Fatalf("detail aspect = %v, want Profile", updated.navState.Aspect)
	}
	if updated.listPane.sidebar.cursor != listCursor {
		t.Fatal("Detail action moved hidden List cursor")
	}
}

func TestShellDestinationChangesFromEitherPaneAndReturnsToList(t *testing.T) {
	for _, focus := range []KeyFocus{FocusList, FocusDetail} {
		layout := testLayout()
		layout.focus = focus

		result, _ := sendKey(&layout, "4")
		updated := result.(LayoutModel)
		if got := updated.navState.Destination; got != nav.DestinationSettings {
			t.Fatalf("from %s destination = %v, want Settings", focus, got)
		}
		if got := updated.focus; got != FocusList {
			t.Fatalf("from %s focus = %v, want List", focus, got)
		}
	}
}

func TestShellOpeningGameUsesOverviewAndDetailFocus(t *testing.T) {
	game := testGame("Cyberpunk 2077")
	layout := testLayout(game)
	layout.navState.Aspect = nav.AspectProfile

	updated, _ := layout.Update(gameConfirmedMsg{game: game})
	result := updated.(LayoutModel)
	if result.navState.Aspect != nav.AspectOverview {
		t.Fatalf("opened game aspect = %v, want Overview", result.navState.Aspect)
	}
	if result.focus != FocusDetail {
		t.Fatalf("opened game focus = %v, want Detail", result.focus)
	}
}
