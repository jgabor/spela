package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func TestListPaneSupportedStateTransitionsAndViews(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	entry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	sidebar := testSidebar(entry)
	state := nav.DefaultState()
	list := NewListPane(styles, sidebar, &state)
	list.SetSize(30, 20)
	if view := stripANSI(list.View(true)); !strings.Contains(view, "All games") || !strings.Contains(view, "Cyberpunk 2077") {
		t.Fatalf("Library List view:\n%s", view)
	}

	for _, destination := range []nav.Destination{nav.DestinationDLLCatalog, nav.DestinationMonitor, nav.DestinationSettings} {
		state = state.SelectDestination(destination)
		list.SetState(state)
		for _, key := range []string{"down", "j", "up", "k"} {
			next, _, handled := list.Update(keyMsg(key))
			list = next
			if !handled {
				t.Errorf("destination %v key %s was not handled", destination, key)
			}
		}
		if view := stripANSI(list.View(true)); view == "" || strings.Contains(view, "unknown") {
			t.Errorf("destination %v List view:\n%s", destination, view)
		}
	}
	list.navState = nil
	if list.State().Destination != nav.DestinationLibrary {
		t.Fatal("nil navigation state did not fall back to default")
	}
	if _, _, handled := list.Update("not a key"); handled {
		t.Fatal("non-key message was unexpectedly handled")
	}
}

func TestOptionsModalSupportedSaveFailureAndInlineSections(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	original := config.Default()
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(original)
	modal.SetSize(100, 40)
	if !strings.Contains(stripANSI(modal.renderOptionsBody()), "Show hints") {
		t.Fatal("settings destination did not render")
	}
	modal.cycleValue(1)
	if !modal.modified {
		t.Fatal("option cycle did not mark modal modified")
	}
	modal.OpenEmbedded(config.Default())
	modal.SyncNavSection(nav.SettingsSection(-1))
	modal.SyncNavSection(nav.SettingsSection(99))
	for _, key := range []string{"down", "j", "up", "k", "left", "h", "right", "l"} {
		next, _ := modal.Update(keyMsg(key))
		modal = next
	}
	if _, command := modal.Update(keyMsg("esc")); command != nil {
		t.Fatal("embedded escape should not emit a command")
	}
	for section := range nav.SettingsSectionLabels {
		modal.SyncNavSection(nav.SettingsSection(section))
		view := stripANSI(modal.renderOptionsBody())
		if !strings.Contains(view, modal.sections[section].Options[0].Label) {
			t.Errorf("inline section %d missing heading:\n%s", section, view)
		}
	}
	modal.sectionCursor = len(modal.sections)
	if modal.getCurrentOption() != nil {
		t.Fatal("out-of-range section unexpectedly returned an option")
	}
	modal.sectionCursor = 0
	modal.optionCursor = len(modal.sections[0].Options)
	if modal.getCurrentOption() != nil {
		t.Fatal("out-of-range option unexpectedly returned an option")
	}
	modal.SyncNavSection(nav.SettingsPaths)
	modal.optionCursor = 0
	next, _ := modal.Update(keyMsg("enter"))
	modal = next
	if !modal.editingPath {
		t.Fatal("path option did not enter editor")
	}
	modal.pathInput.SetValue("/cancelled")
	next, _ = modal.Update(keyMsg("esc"))
	modal = next
	if modal.editingPath || modal.config.SteamPath != "" {
		t.Fatal("path editor escape did not cancel edit")
	}
	next, _ = modal.Update(keyMsg("enter"))
	modal = next
	modal.pathInput.SetValue("/steam")
	next, _ = modal.Update(keyMsg("enter"))
	modal = next
	if modal.editingPath || modal.draft.SteamPath != "/steam" || modal.config.SteamPath != "" {
		t.Fatal("path editor did not confirm edit")
	}

	state := t.TempDir()
	configHomeFile := filepath.Join(state, "config-file")
	if err := os.WriteFile(configHomeFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", configHomeFile)
	modal.OpenEmbedded(config.Default())
	modal.modified = true
	_, command := modal.Update(keyMsg("s"))
	if command == nil {
		t.Fatal("save failure path did not return command")
	}
	message, ok := command().(optionsSaveErrorMsg)
	if !ok || message.err == nil || !errors.Is(message.err, os.ErrNotExist) && !strings.Contains(message.err.Error(), "not a directory") {
		t.Fatalf("save failure message = %#v", message)
	}
}

func TestSidebarSupportedRenderedSelectionFilterAndBatchStates(t *testing.T) {
	services := testServices()
	services.ProfileExists = func(appID uint64) bool { return appID == 2 }
	styles := NewStyles(DefaultTheme, true)
	first := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.8.10"))
	first.AppID = 1
	second := testGame("Beta")
	second.AppID = 2
	sidebar, _ := NewSidebar([]*game.Game{second, first}, styles, services)
	sidebar.SetSize(30, 12)
	view := stripANSI(sidebar.View())
	for _, fragment := range []string{"All games", "Alpha", "Beta", "●", "◆"} {
		if !strings.Contains(view, fragment) {
			t.Errorf("sidebar missing %q:\n%s", fragment, view)
		}
	}

	for _, key := range []string{"down", "space", "down", "space", "a", "A", "d", "p", "s", "s", "s", "s", "C"} {
		next, _ := sidebar.Update(keyMsg(key))
		sidebar = next
		_ = sidebar.View()
	}
	if sidebar.InSelectMode() && sidebar.SelectionCount() != 0 {
		t.Fatalf("batch selection after clear = %d", sidebar.SelectionCount())
	}
	sidebar, _ = sidebar.FocusSearch()
	if !sidebar.search.Focused() {
		t.Fatal("search focus contract failed")
	}
	sidebar, _ = sidebar.Update(keyMsg("z"))
	if view := stripANSI(sidebar.View()); !strings.Contains(view, "No games found") {
		t.Fatalf("empty filtered sidebar:\n%s", view)
	}
	sidebar, _ = sidebar.Update(keyMsg("esc"))
	if sidebar.search.Focused() {
		t.Fatal("escape did not leave sidebar search")
	}
	empty := sidebar.SetGames(nil)
	if empty.Selected() != nil || empty.SelectedItem() != nil {
		t.Fatalf("empty sidebar selection = game %+v item %+v", empty.Selected(), empty.SelectedItem())
	}
}
