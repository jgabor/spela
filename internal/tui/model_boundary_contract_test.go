package tui

import (
	"path/filepath"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

func TestContentSupportedIdentityDefaultsAndNoSelectionOperations(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	services := testServices()
	styles := NewStyles(DefaultTheme, true)
	content := NewContent(styles, true, services)
	if content.Name() != "Details" || content.HasGameSelection() {
		t.Fatalf("empty content identity = %q", content.Name())
	}
	content.SetSize(80, 4)
	if content.profileSectionHeight() != 5 {
		t.Fatalf("minimum game profile height = %d", content.profileSectionHeight())
	}
	if message := content.updateDLLs()().(dllUpdateMsg); message.err == nil {
		t.Fatal("empty DLL update unexpectedly succeeded")
	}
	if message := content.restoreDLLs()().(dllRestoreMsg); message.err == nil {
		t.Fatal("empty DLL restore unexpectedly succeeded")
	}
	if content.saveResolvedProfile() != nil {
		t.Fatal("empty content returned a save command")
	}
	if message := content.LoadDLLUpdates()().(dllUpdatesCheckedMsg); message.hasUpdates || message.err != nil {
		t.Fatalf("empty DLL check = %+v", message)
	}

	entry := testGame("Cyberpunk 2077")
	content = content.SetGame(entry)
	if content.Name() != entry.Name || !content.HasGameSelection() {
		t.Fatalf("game content identity = %q", content.Name())
	}
}

func TestSidebarSupportedConfirmationAndNilItemContracts(t *testing.T) {
	entry := testGame("Cyberpunk 2077")
	sidebar := testSidebar(entry)
	if len(sidebar.filtered) < 2 {
		t.Fatalf("sidebar fixture items = %d", len(sidebar.filtered))
	}
	sidebar.cursor = 0
	_, command := sidebar.Update(keyMsg("enter"))
	if command == nil {
		t.Fatal("default profile confirmation returned no command")
	}
	if _, ok := command().(defaultProfileConfirmedMsg); !ok {
		t.Fatalf("default confirmation = %#v", command())
	}
	sidebar.cursor = 1
	_, command = sidebar.Update(keyMsg("enter"))
	if _, ok := command().(gameConfirmedMsg); !ok {
		t.Fatalf("game confirmation = %#v", command())
	}
	sidebar, _ = sidebar.Update(keyMsg("space"))
	_, command = sidebar.Update(keyMsg("enter"))
	if message, ok := command().(batchActionRequestMsg); !ok || len(message.selected) != 1 {
		t.Fatalf("batch confirmation = %#v", command())
	}

	nilItem := sidebarItem{kind: sidebarItemGame}
	if sidebar.itemName(nilItem) != "" || sidebar.itemIndicator(nilItem) != "" {
		t.Fatal("nil game sidebar item rendered content")
	}
	sidebar.filtered = []sidebarItem{nilItem}
	sidebar.cursor = 0
	if next, command := sidebar.Update(keyMsg("space")); command != nil || next.SelectionCount() != sidebar.SelectionCount() {
		t.Fatal("nil sidebar item changed selection")
	}
}

func TestDetailSupportedNilAndFocusBoundaryContracts(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	detail := NewDetail(styles, nil, nil)
	detail.SetActiveSubsystem("missing")
	if detail.FocusedField() != "" || detail.CycleFocusedField(1) {
		t.Fatal("empty subsystem exposed a focusable field")
	}
	if changed, err := detail.ResetFocused(); err != nil || changed {
		t.Fatalf("empty reset = %v, %v", changed, err)
	}
	if changed, err := detail.PinFocused(); err != nil || changed {
		t.Fatalf("empty pin = %v, %v", changed, err)
	}
	detail.raw = nil
	detail.rebuildResolved()
	if detail.resolved == nil || detail.IsOverridden(profile.FieldDLSSSRMode) || formatFieldValue(nil, "missing") != "(default)" || formatRootFieldValue(nil, "missing") != "(default)" {
		t.Fatal("nil detail fallback changed")
	}
	detail.styles = nil
	if detail.View() != "" {
		t.Fatal("styleless detail rendered content")
	}
}

func TestSidebarNilServiceIndicatorsRemainSafe(t *testing.T) {
	sidebar := testSidebar(&game.Game{AppID: 1, Name: "No metadata"})
	sidebar.services.ProfileExists = func(uint64) bool { return false }
	if got := sidebar.itemIndicator(sidebarItem{kind: sidebarItemGame, game: sidebar.games[0]}); got != "" {
		t.Fatalf("empty indicator = %q", got)
	}
}
