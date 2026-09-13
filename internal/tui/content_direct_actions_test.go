package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func TestDLLDirectActionsOpenTheSelectedWorkflow(t *testing.T) {
	for _, action := range gameDLLActions {
		t.Run(string(action), func(t *testing.T) {
			entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0"))
			content := testContent(entry)
			content.hasBackup, content.hasUpdates = true, true
			content.dllUpdateTargets = []dllMutationTarget{newDLLMutationTarget(entry, entry.DLLs[0], "3.8.0")}
			for content.DLLSelectedAction() != action {
				content, _ = content.UpdateDLLAction(ActionDetailNextItem)
			}
			content, command := content.UpdateDLLAction(ActionDetailConfirm)
			switch action {
			case ActionDetailInstall:
				if command == nil || content.dllInstallState != DLLInstallSelectType {
					t.Fatal("Enter did not open the selected install chooser")
				}
			case ActionDetailUpdate, ActionDetailRestore:
				if command != nil || content.confirmation == nil || content.confirmation.confirmSelected || len(content.confirmation.targets) != 1 {
					t.Fatal("Enter did not open a safe confirmation for the selected action")
				}
				content, command = content.UpdateDLLAction(ActionOverlayConfirm)
				if command != nil || content.confirmation != nil || !content.dllResultOpen || !strings.Contains(content.lastDLLResult, "cancelled") {
					t.Fatal("default confirmation choice did not cancel without dispatch")
				}
				content, _ = content.UpdateDLLAction(ActionOverlayFocusNext)
				content, command = content.UpdateDLLAction(ActionOverlayConfirm)
				if command != nil || content.HasModalOpen() || content.DLLSelectedAction() != action {
					t.Fatal("Close did not return to the initiating DLL action")
				}
			}
		})
	}
}

func TestDLLDirectDisabledActionsRemainVisibleAndDoNotRun(t *testing.T) {
	content := testContent(testGame("Empty fixture"))
	if content.DLLActionCount() != 3 || content.DLLSelectedActionName() != "Install DLL" {
		t.Fatal("DLL action selection did not start at Install")
	}
	for _, action := range []KeyAction{ActionDetailUpdate, ActionDetailRestore} {
		content, _ = content.UpdateDLLAction(ActionDetailNextItem)
		if content.DLLSelectedAction() != action {
			t.Fatal("arrow navigation skipped an unavailable action")
		}
		available, reason := content.DLLActionAvailability(action)
		view := stripANSI(content.ViewDLLAspectFocused(true))
		if available || reason == "" || !strings.Contains(view, reason) || !strings.Contains(view, "▸ "+content.dllActionTitle(action)) || strings.Contains(view, "Enter ") {
			t.Fatalf("unavailable selected action has misleading controls:\n%s", view)
		}
		updated, command := content.UpdateDLLAction(ActionDetailConfirm)
		if command != nil || updated.HasModalOpen() || updated.DLLSelectedAction() != action {
			t.Fatal("Enter dispatched an unavailable DLL action")
		}
	}
	content, _ = content.UpdateDLLAction(ActionDetailNextItem)
	if content.DLLSelectedAction() != ActionDetailRestore {
		t.Fatal("Down moved beyond the final visible action")
	}
}

func TestDLLDirectControlsFitBelowLongVersionContent(t *testing.T) {
	var installed []game.DetectedDLL
	for range 30 {
		installed = append(installed, testDLL(game.DLLTypeDLSS, "3.7.0"))
	}
	content := testContent(testGame("A long game name with many installed DLL copies", installed...))
	for _, size := range [][2]int{{72, 17}, {82, 30}} {
		content.SetSize(size[0], size[1])
		view := stripANSI(content.ViewDLLAspectFocused(true))
		for _, expected := range []string{"Install DLL", "Update DLLs (0 files)", "Restore originals (0 files)", "no stale DLLs", "no backup", "↑/↓ Choose action", "Enter Install DLL"} {
			if !strings.Contains(view, expected) {
				t.Fatalf("%dx%d lost essential control %q:\n%s", size[0], size[1], expected, view)
			}
		}
		if lines := strings.Count(view, "\n") + 1; lines > size[1]-3 {
			t.Fatalf("DLL view has %d lines in a %d-line body:\n%s", lines, size[1]-3, view)
		}
	}
}

func TestDLLDirectControlsAdvertiseOnlyTheFocusedPaneKeys(t *testing.T) {
	content := testContent(testGame("Fixture"))
	view := stripANSI(content.ViewDLLAspectFocused(false))
	if !strings.Contains(view, "Install DLL") || !strings.Contains(view, "Tab: Focus DLL controls") {
		t.Fatalf("inactive pane hides its controls or focus path:\n%s", view)
	}
	for _, inactiveKey := range []string{"↑/↓", "Enter ", "▸ "} {
		if strings.Contains(view, inactiveKey) {
			t.Fatalf("inactive DLL pane advertises %q:\n%s", inactiveKey, view)
		}
	}
}

func TestDLLDirectRestoreWithAnEmptyBackupIsUnavailable(t *testing.T) {
	content := testContent(testGame("Empty backup"))
	content.hasBackup = true
	content, _ = content.UpdateDLLAction(ActionDetailNextItem)
	content, _ = content.UpdateDLLAction(ActionDetailNextItem)
	available, reason := content.DLLActionAvailability(content.DLLSelectedAction())
	if available || reason != "backup has no original files" {
		t.Fatalf("empty restore availability = %v, %q", available, reason)
	}
	view := stripANSI(content.ViewDLLAspectFocused(true))
	if !strings.Contains(view, "Restore originals (0 files)") || !strings.Contains(view, reason) || strings.Contains(view, "Enter ") {
		t.Fatalf("empty restore has misleading controls:\n%s", view)
	}
	content, command := content.UpdateDLLAction(ActionDetailConfirm)
	if command != nil || content.HasModalOpen() {
		t.Fatal("empty restore opened a workflow")
	}
}

func TestDLLDirectSelectionFollowsGameScope(t *testing.T) {
	first := testGame("First")
	content := testContent(first)
	content, _ = content.UpdateDLLAction(ActionDetailNextItem)
	content = content.SetGame(first)
	if content.DLLSelectedAction() != ActionDetailUpdate {
		t.Fatal("refreshing the same game reset its DLL action")
	}
	second := testGame("Second")
	second.AppID++
	content = content.SetGame(second)
	if content.DLLSelectedAction() != ActionDetailInstall {
		t.Fatal("changing game scope retained another game's selected action")
	}
	content = content.SetGame(nil)
	if content.DLLActionCount() != 0 || content.DLLSelectedAction() != "" || content.DLLSelectedActionName() != "" {
		t.Fatal("global scope exposed per-game DLL actions")
	}
}

func TestDLLDirectControlsRouteDisplayedKeysWithHintsDisabled(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0"), testDLL(game.DLLTypeDLSSG, "3.7.0"))
		layout := testLayoutWithGame(entry)
		layout.rescanBusy = false
		layout.config.ShowHints = false
		layout.styles.SetShowHints(false)
		layout.width, layout.height = size[0], size[1]
		layout.navState.Aspect = nav.AspectDLLs
		layout.focus = FocusDetail
		layout.pane.content.hasUpdates = true
		layout.pane.content.dllUpdateTargets = []dllMutationTarget{newDLLMutationTarget(entry, entry.DLLs[0], "3.8.0")}
		layout.calculateDimensions()
		view := stripANSI(layout.View().Content)
		for _, expected := range []string{"DLSS: 3.7.0", "DLSS-G: 3.7.0", "Install DLL", "Update DLLs (1 file)", "Restore originals", "no backup", "↑", "↓", "Enter"} {
			if !strings.Contains(view, expected) {
				t.Fatalf("%dx%d hints-off DLL view hides %q:\n%s", size[0], size[1], expected, view)
			}
		}
		next, _ := sendKey(layout, "down")
		layout = next.(LayoutModel)
		if layout.pane.content.DLLSelectedAction() != ActionDetailUpdate {
			t.Fatalf("%dx%d displayed Down did not select Update DLLs", size[0], size[1])
		}
		next, command := sendKey(layout, "enter")
		layout = next.(LayoutModel)
		if command != nil || layout.pane.content.confirmation == nil || layout.pane.content.confirmation.confirmSelected {
			t.Fatalf("displayed Enter did not open the selected safe confirmation: command=%t, confirmation=%+v, action=%q", command != nil, layout.pane.content.confirmation, layout.pane.content.DLLSelectedAction())
		}
	}
}
