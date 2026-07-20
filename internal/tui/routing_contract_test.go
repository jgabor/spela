package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/nav"
)

func TestBrowseDestinationShortcutWorksFromVisiblePane(t *testing.T) {
	layout := testLayout()
	layout.focus = FocusDetail

	result, _ := sendKey(&layout, "2")
	updated := result.(LayoutModel)
	if got := updated.navState.Destination; got != nav.DestinationDLLCatalog {
		t.Fatalf("destination = %v, want DLL Catalog", got)
	}
}

func TestSearchConsumesPrintableDestinationShortcut(t *testing.T) {
	layout := testLayout()
	layout.focus = FocusList
	layout.inputMode = ModeSearch
	layout.listPane.sidebar.search.Focus()

	result, _ := sendKey(&layout, "2")
	updated := result.(LayoutModel)
	if got := updated.navState.Destination; got != nav.DestinationLibrary {
		t.Fatalf("search input changed destination to %v", got)
	}
	if got := updated.listPane.sidebar.search.Value(); got != "2" {
		t.Fatalf("search value = %q, want reserved printable character", got)
	}
}

func TestUnsupportedKeyDoesNotChangeNavigationState(t *testing.T) {
	layout := testLayout()
	layout.focus = FocusDetail
	before := *layout.navState

	result, command := sendKey(&layout, "f12")
	updated := result.(LayoutModel)
	if command != nil {
		t.Fatal("unsupported key launched a command")
	}
	if got := *updated.navState; got != before {
		t.Fatalf("unsupported key changed navigation state: before %#v after %#v", before, got)
	}
}

func TestSearchConsumesReservedKeysBeforeGlobals(t *testing.T) {
	layout := testLayout()
	model, _ := sendKey(&layout, "/")
	searching := model.(LayoutModel)

	for _, key := range []string{"?", "/", "tab", "ctrl+r"} {
		beforeFocus := searching.focus
		model, _ = sendKey(&searching, key)
		searching = model.(LayoutModel)
		if searching.showHelp {
			t.Fatalf("Search key %q opened help", key)
		}
		if searching.focus != beforeFocus {
			t.Fatalf("Search key %q changed pane focus", key)
		}
	}
	if got := searching.listPane.sidebar.search.Value(); got != "?/" {
		t.Fatalf("search value = %q, want reserved printable keys", got)
	}
}

func TestProfileEditConsumesPrintableDestinationShortcut(t *testing.T) {
	layout := testLayout()
	layout.focus = FocusDetail
	if !layout.pane.defaultsDetail.BeginEdit() {
		t.Fatal("could not start profile editor")
	}

	model, _ := layout.Update(keyMsg("4"))
	updated := model.(LayoutModel)
	if updated.navState.Destination != nav.DestinationLibrary {
		t.Fatalf("edit input changed destination to %s", updated.navState.Destination)
	}
	if !updated.pane.defaultsDetail.Editing() || !strings.HasSuffix(updated.pane.defaultsDetail.editor.Value(), "4") {
		t.Fatal("printable destination key did not stay with the active editor")
	}
}

func TestBelowMinimumViewportSuppressesHiddenWorkspaceInput(t *testing.T) {
	layout := testLayout()
	layout.width = minimumTerminalWidth - 1
	layout.height = minimumTerminalHeight
	before := *layout.navState

	for _, key := range []string{"2", "/", "tab", "ctrl+r", "q"} {
		model, command := sendKey(&layout, key)
		updated := model.(LayoutModel)
		if command != nil {
			t.Fatalf("below-minimum key %q launched a command", key)
		}
		if *updated.navState != before || updated.focus != layout.focus || updated.inputMode != layout.inputMode {
			t.Fatalf("below-minimum key %q changed hidden workspace state", key)
		}
	}
}
