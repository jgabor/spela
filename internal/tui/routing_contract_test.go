package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
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

func TestTextEditorKeepsPrintableNavigationKeys(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme, true), nil, nil)
	focusField(t, &detail, profile.FieldGPUShaderCachePath)
	if !detail.BeginEdit() {
		t.Fatal("could not start text editor")
	}
	for _, key := range []string{"h", "l", "x"} {
		detail.UpdateEditor(keyMsg(key))
	}
	if !strings.HasSuffix(detail.editor.Value(), "hlx") {
		t.Fatalf("text editor value = %q, want printable suffix hlx", detail.editor.Value())
	}
	detail.UpdateEditor(keyMsg("enter"))
	if detail.Editing() || !strings.HasSuffix(detail.RawProfile().GPU.ShaderCachePath, "hlx") {
		t.Fatal("printable text could not be committed")
	}
}

func TestProfileDraftEscapeRestoresPersistedState(t *testing.T) {
	layout := testLayoutWithGame(testGame("Cyberpunk 2077"))
	layout.focus = FocusDetail
	*layout.navState = layout.navState.SelectAspect(nav.AspectProfile)
	layout.syncNavToComponents()
	detail := &layout.pane.content.detail
	focusField(t, detail, profile.FieldGPUShaderCachePath)
	detail.BeginEdit()
	detail.editor.Set("draft")
	detail.UpdateEditor(keyMsg("enter"))

	model, _ := sendKey(&layout, "esc")
	updated := model.(LayoutModel)
	if updated.pane.content.detail.Dirty() {
		t.Fatal("escape retained unsaved profile draft")
	}
}

func TestDestinationChangeClearsMultiSelection(t *testing.T) {
	layout := testLayoutWithGame(testGame("Cyberpunk 2077"))
	layout.listPane.sidebar.selectMode = true
	layout.listPane.sidebar.selected[layout.listPane.sidebar.games[0].AppID] = true

	model, _ := sendKey(&layout, "2")
	updated := model.(LayoutModel)
	if updated.listPane.sidebar.selectMode || len(updated.listPane.sidebar.selected) != 0 {
		t.Fatal("destination change retained hidden multi-selection")
	}
	if updated.navState.Destination != nav.DestinationDLLCatalog || updated.pane.State().Destination != nav.DestinationDLLCatalog {
		t.Fatal("destination and detail state diverged")
	}
}

func TestClearingFinalMarkedGameReturnsToBrowsing(t *testing.T) {
	layout := testLayoutWithGame(testGame("Cyberpunk 2077"))
	sidebar := layout.listPane.sidebar
	sidebar.cursor = 1
	sidebar, _ = sidebar.Update(keyMsg("space"))
	sidebar, command := sidebar.Update(keyMsg("space"))
	if sidebar.selectMode || len(sidebar.selected) != 0 {
		t.Fatal("clearing final mark retained multi-select mode")
	}
	if command == nil {
		t.Fatal("clearing final mark did not restore highlighted item detail")
	}
}

func TestEmptySearchClearsStaleGameDetail(t *testing.T) {
	layout := testLayoutWithGame(testGame("Cyberpunk 2077"))
	layout.focus = FocusList
	model, _ := sendKey(&layout, "/")
	searching := model.(LayoutModel)
	for _, key := range []string{"n", "o", "m", "a", "t", "c", "h"} {
		model, _ = sendKey(&searching, key)
		searching = model.(LayoutModel)
	}
	command := searching.listPane.sidebar.selectCurrentItem()
	if command == nil {
		t.Fatal("empty search did not produce explicit no-item transition")
	}
	updated, _ := searching.handleAppMessages(command(), nil)
	if updated.pane.content.game != nil || updated.navState.Scope.GameName != "" {
		t.Fatal("empty search retained stale game detail")
	}
}

func TestBelowMinimumViewportSuppressesHiddenWorkspaceInput(t *testing.T) {
	layout := testLayout()
	layout.width = minimumTerminalWidth - 1
	layout.height = minimumTerminalHeight
	before := *layout.navState

	for _, key := range []string{"2", "/", "tab", "ctrl+r"} {
		model, command := sendKey(&layout, key)
		updated := model.(LayoutModel)
		if command != nil {
			t.Fatalf("below-minimum key %q launched a command", key)
		}
		if *updated.navState != before || updated.focus != layout.focus || updated.inputMode != layout.inputMode {
			t.Fatalf("below-minimum key %q changed hidden workspace state", key)
		}
	}

	_, command := sendKey(&layout, "q")
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("below-minimum q did not quit")
	}
}
