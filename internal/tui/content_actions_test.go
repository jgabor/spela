package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func TestDLLConfirmationPinsButtonsAndScrollsBackupPolicy(t *testing.T) {
	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0"))
	var targets []dllMutationTarget
	for index := range 9 {
		target := newDLLMutationTarget(entry, entry.DLLs[0], "3.8.0")
		target.path = "/fixture/long-directory-name/" + strings.Repeat("nested/", index+1) + "nvngx_dlss.dll"
		targets = append(targets, target)
	}
	confirmation := newDLLMutationConfirmation("Confirm DLL updates", targets, "each original DLL is backed up before replacement")
	styles := NewStyles(DefaultTheme, false)
	view := stripANSI(confirmation.viewSized(styles, 54, 9))
	if strings.Count(view, "\n") >= 9 || !strings.Contains(view, "[ Cancel ]") || !strings.Contains(view, "[ Confirm ]") || !strings.Contains(view, "Tab Details") {
		t.Fatalf("short dialog clipped its controls:\n%s", view)
	}
	confirmation.updateAction(ActionOverlayFocusNext)
	seenPolicy := false
	for range 100 {
		view = stripANSI(confirmation.viewSized(styles, 54, 9))
		if strings.Contains(view, "Backup:") {
			seenPolicy = true
		}
		if !strings.Contains(view, "Cancel   Confirm") || !strings.Contains(view, "Tab Buttons") {
			t.Fatalf("body scroll lost the return path:\n%s", view)
		}
		confirmation.updateAction(ActionOverlayNext)
	}
	if !seenPolicy {
		t.Fatal("backup policy cannot be reached through displayed scrolling controls")
	}
	if confirm, cancel := confirmation.updateAction(ActionOverlayConfirm); confirm || cancel {
		t.Fatal("Enter in the scrollable body activated a hidden button")
	}
	confirmation.updateAction(ActionOverlayFocusNext)
	if confirm, cancel := confirmation.updateAction(ActionOverlayConfirm); confirm || !cancel {
		t.Fatal("returning from the body changed the initial Cancel choice")
	}
}

func TestDLLProgressCannotCancelOrDispatchAgainAndResultCanClose(t *testing.T) {
	entry, services, path, original, calls := newDLLCancellationFixture(t)
	content := NewContent(NewStyles(DefaultTheme, false), true, services).SetGame(entry)
	content.SetSize(70, 12)
	content.hasUpdates = true
	content.dllUpdateTargets = []dllMutationTarget{newDLLMutationTarget(entry, entry.DLLs[0], "3.8.0")}
	content, _ = content.UpdateDLLAction(ActionDetailUpdate)
	content, _ = content.UpdateDLLAction(ActionOverlayRight)
	content, command := content.UpdateDLLAction(ActionOverlayConfirm)
	if command == nil || !content.dllOperating || !content.HasModalOpen() {
		t.Fatal("explicit Confirm did not enter progress")
	}
	for _, action := range []KeyAction{ActionOverlayClose, ActionOverlayFocusNext, ActionOverlayConfirm, ActionDetailInstall, ActionDetailUpdate, ActionDetailRestore} {
		next, repeated := content.UpdateDLLAction(action)
		content = next
		if repeated != nil || !content.dllOperating || !content.HasModalOpen() {
			t.Fatalf("progress accepted action %q", action)
		}
	}
	if view := stripANSI(content.ViewDLLAspect()); strings.Contains(view, "Cancel") || strings.Contains(view, "Close") || !strings.Contains(view, "Work is running") {
		t.Fatalf("progress advertises false cancellation:\n%s", view)
	}
	// A queued command has not performed its write until executed.
	assertDLLFixtureUnchanged(t, path, original, *calls)
	message := command()
	content, _ = content.Update(message)
	if *calls != 1 || content.dllOperating || !content.dllResultOpen {
		t.Fatalf("completion state: calls=%d, busy=%v, result=%v", *calls, content.dllOperating, content.dllResultOpen)
	}
	if !strings.Contains(stripANSI(content.ViewDLLAspect()), "Tab Close") {
		t.Fatal("result has no basic-key Close path with verbose hints disabled")
	}
	content, _ = content.UpdateDLLAction(ActionOverlayFocusNext)
	content, command = content.UpdateDLLAction(ActionOverlayConfirm)
	if content.HasModalOpen() || command != nil || *calls != 1 {
		t.Fatal("result dismissal reran or retained the operation")
	}
}

func TestDLLChooserBackAndCancelNeverMutate(t *testing.T) {
	entry, services, path, original, calls := newDLLCancellationFixture(t)
	content := NewContent(NewStyles(DefaultTheme, false), true, services).SetGame(entry)
	content.SetSize(70, 12)
	content.dllInstallState = DLLInstallSelectVersion
	content.selectedDLLType = "dlss"
	content.dllTypes = []string{"dlss", "xess"}
	content.dllTypeCursor = 1
	content.dllVersions = []dll.DLL{{Version: "3.8.0"}, {Version: "3.7.0"}}
	content.dllVersionCursor = 1
	content, _ = content.UpdateDLLAction(ActionOverlayFocusNext)
	content, command := content.UpdateDLLAction(ActionOverlayConfirm)
	if content.dllInstallState != DLLInstallSelectType || content.dllTypeCursor != 1 || command != nil {
		t.Fatal("Back did not preserve family selection without dispatch")
	}
	content, _ = content.UpdateDLLAction(ActionOverlayFocusNext)
	content, command = content.UpdateDLLAction(ActionOverlayConfirm)
	if content.dllInstallState != DLLInstallNone || !content.dllResultOpen || command != nil {
		t.Fatal("Cancel did not produce a non-mutating result")
	}
	assertDLLFixtureUnchanged(t, path, original, *calls)
}

func TestDLLRestoreConfirmationIncludesMissingBackupTargets(t *testing.T) {
	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.8.0"))
	content := testContent(entry)
	content.hasBackup = true
	content.services.LoadDLLBackup = func(uint64) (*dll.Backup, error) {
		return &dll.Backup{Files: []dll.BackedUpFile{
			{OriginalPath: entry.DLLs[0].Path, DLLName: "nvngx_dlss.dll", Version: "3.7.0"},
			{OriginalPath: "/fixture/missing/libxess.dll", DLLName: "libxess.dll", Version: "1.3.0"},
		}}, nil
	}
	content, command := content.UpdateDLLAction(ActionDetailRestore)
	if command != nil || content.confirmation == nil || len(content.confirmation.targets) != 2 {
		t.Fatalf("restore confirmation omitted backup target: %+v", content.confirmation)
	}
	view := stripANSI(content.ViewDLLAspect())
	for _, want := range []string{"/fixture/missing/libxess.dll", "XeSS", "backup 1.3.0", "3.8.0 → backup 3.7.0"} {
		if !strings.Contains(view, want) {
			t.Fatalf("restore confirmation missing %q:\n%s", want, view)
		}
	}
}

func TestDLLRestoreBackupReadFailureDoesNotConfirm(t *testing.T) {
	content := testContent(testGame("Fixture"))
	content.hasBackup = true
	content.services.LoadDLLBackup = func(uint64) (*dll.Backup, error) { return nil, errors.New("metadata unreadable") }
	content, command := content.UpdateDLLAction(ActionDetailRestore)
	if command != nil || content.confirmation != nil || !content.dllResultOpen || !strings.Contains(content.lastDLLResult, "metadata unreadable") {
		t.Fatalf("backup failure did not remain a non-mutating visible result: %+v", content)
	}
}

func TestDLLCatalogActionAvailabilityAndResultDetail(t *testing.T) {
	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0"))
	resource := makeDLLsResource([]*game.Game{entry}, map[string][]string{"dlss": {"3.8.0"}}, nil)
	if available, reason := resource.DLLActionAvailability(ActionDetailUpdate, nav.SectionDLLLibrary); available || reason != "open Deployment" {
		t.Fatalf("catalog Library update availability = %v, %q", available, reason)
	}
	if available, reason := resource.DLLActionAvailability(ActionDetailUpdate, nav.SectionDLLDeployment); !available || reason != "" || resource.UpdateTargetCount() != 1 {
		t.Fatalf("catalog Deployment update availability = %v, %q", available, reason)
	}
	resource, _ = resource.Update(dllsUpdateAllCompleteMsg{summary: "Update-all: 0 updated, 0 current, 1 failed", results: map[string]string{"1091500:dlss": "err: copy denied"}})
	view := stripANSI(resource.View(false, nav.SectionDLLDeployment))
	if !strings.Contains(view, "Fixture · DLSS: copy denied") || !strings.Contains(view, "Tab Close") {
		t.Fatalf("catalog failure hides its cause or Close control:\n%s", view)
	}
	resource, _ = resource.UpdateAction(ActionOverlayFocusNext)
	resource, command := resource.UpdateAction(ActionOverlayConfirm)
	if resource.HasModalOpen() || command != nil {
		t.Fatal("catalog Close did not dismiss the result")
	}
}

func TestDLLChooserIgnoresCanceledRequestResults(t *testing.T) {
	content := testContent(testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0")))
	content, _ = content.UpdateDLLAction(ActionDetailInstall)
	oldRequest := content.dllRequestID
	content, _ = content.UpdateDLLAction(ActionOverlayFocusNext)
	content, _ = content.UpdateDLLAction(ActionOverlayConfirm)
	content, _ = content.UpdateDLLAction(ActionOverlayFocusNext)
	content, _ = content.UpdateDLLAction(ActionOverlayConfirm)
	content, _ = content.UpdateDLLAction(ActionDetailInstall)
	currentRequest := content.dllRequestID
	if currentRequest == oldRequest {
		t.Fatal("new chooser reused canceled request identity")
	}
	content, _ = content.Update(dllTypesLoadedMsg{requestID: oldRequest, types: []string{"xess"}})
	content, _ = content.Update(dllInstallMsg{requestID: oldRequest, err: errors.New("stale failure")})
	if len(content.dllTypes) != 0 || content.dllInstallState != DLLInstallSelectType || content.dllResultOpen {
		t.Fatal("canceled request changed the new chooser")
	}
	content, _ = content.Update(dllTypesLoadedMsg{requestID: currentRequest, types: []string{"dlss"}})
	content, _ = content.UpdateDLLAction(ActionOverlayConfirm)
	versionRequest := content.dllRequestID
	content, _ = content.Update(dllVersionsLoadedMsg{requestID: currentRequest, versions: []dll.DLL{{Version: "wrong"}}})
	if len(content.dllVersions) != 0 {
		t.Fatal("old family request replaced current version choices")
	}
	content, _ = content.Update(dllVersionsLoadedMsg{requestID: versionRequest, versions: []dll.DLL{{Version: "3.8.0"}}})
	if len(content.dllVersions) != 1 || content.dllVersions[0].Version != "3.8.0" {
		t.Fatal("current version request was not applied")
	}
}

func TestDLLUpdateCheckIgnoresPreviousGameTargets(t *testing.T) {
	first := testGame("First", testDLL(game.DLLTypeDLSS, "3.7.0"))
	second := testGame("Second", testDLL(game.DLLTypeXeSS, "1.3.0"))
	second.AppID = 2
	content := testContent(first).SetGame(second)
	content, _ = content.Update(dllUpdatesCheckedMsg{appID: first.AppID, hasUpdates: true, targets: []dllMutationTarget{newDLLMutationTarget(first, first.DLLs[0], "3.8.0")}})
	if content.hasUpdates || len(content.dllUpdateTargets) != 0 {
		t.Fatal("previous game's update check changed the current game's targets")
	}
	content, _ = content.Update(dllUpdatesCheckedMsg{appID: second.AppID, hasUpdates: true, targets: []dllMutationTarget{newDLLMutationTarget(second, second.DLLs[0], "1.4.0")}})
	if !content.hasUpdates || len(content.dllUpdateTargets) != 1 || content.dllUpdateTargets[0].appID != second.AppID {
		t.Fatal("current game's update check was not applied")
	}
}

func TestDLLRestoreRechecksTargetsBeforeDispatch(t *testing.T) {
	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.8.0"))
	content := testContent(entry)
	content.hasBackup = true
	backup := &dll.Backup{Files: []dll.BackedUpFile{{OriginalPath: entry.DLLs[0].Path, DLLName: "nvngx_dlss.dll", Version: "3.7.0"}}}
	content.services.LoadDLLBackup = func(uint64) (*dll.Backup, error) { return backup, nil }
	content, _ = content.UpdateDLLAction(ActionDetailRestore)
	backup.Files = append(backup.Files, dll.BackedUpFile{OriginalPath: "/fixture/extra/libxess.dll", DLLName: "libxess.dll", Version: "1.3.0"})
	content, _ = content.UpdateDLLAction(ActionOverlayRight)
	content, command := content.UpdateDLLAction(ActionOverlayConfirm)
	if command != nil || content.dllOperating || content.confirmation == nil || content.confirmation.confirmSelected || len(content.confirmation.targets) != 2 {
		t.Fatal("changed restore targets were dispatched without a new review")
	}
	if !strings.Contains(stripANSI(content.ViewDLLAspect()), "Backup changed") {
		t.Fatal("updated target review did not explain why confirmation was reset")
	}
}

func TestDLLCatalogListOperationRevealsDetailAndRestoresFocus(t *testing.T) {
	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0"))
	layout := NewLayout(testDatabase(entry), testServices())
	layout.width, layout.height = 80, 24
	layout.selectDestination(nav.DestinationDLLCatalog)
	layout.navState.DLLCatalogSection = nav.SectionDLLDeployment
	layout.pane.dllsResource.cached = map[string][]string{"dlss": {"3.8.0"}}
	layout.focus = FocusList
	layout.calculateDimensions()
	layout, command := layout.updatePaneAction(ActionDetailUpdate)
	if command != nil || layout.focus != FocusDetail || layout.dllCatalogReturnFocus == nil || *layout.dllCatalogReturnFocus != FocusList {
		t.Fatalf("Catalog List mutation did not capture focus and reveal Detail: %+v", layout.dllCatalogReturnFocus)
	}
	view := stripANSI(layout.View().Content)
	for _, want := range []string{"Confirm all stale DLL updates", "1 DLL target(s)", "[ Cancel ]", "[ Confirm ]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("compact Catalog confirmation hides %q:\n%s", want, view)
		}
	}
	layout, _ = layout.updatePaneAction(ActionOverlayConfirm)
	if layout.focus != FocusDetail || !strings.Contains(stripANSI(layout.View().Content), "DLL update-all cancelled") {
		t.Fatal("cancellation result did not remain visible in Detail")
	}
	layout, _ = layout.updatePaneAction(ActionOverlayFocusNext)
	layout, _ = layout.updatePaneAction(ActionOverlayConfirm)
	if layout.focus != FocusList || layout.dllCatalogReturnFocus != nil || layout.pane.dllsResource.HasModalOpen() {
		t.Fatal("closing the result did not restore the original List focus")
	}
	if view = stripANSI(layout.View().Content); !strings.Contains(view, "▸ List") || !strings.Contains(view, "Deployment") {
		t.Fatalf("compact Catalog return view is not the original List:\n%s", view)
	}
}

func TestDLLActionLabelsNameTargets(t *testing.T) {
	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0"))
	content := testContent(entry)
	content.hasBackup = true
	content.dllUpdateTargets = []dllMutationTarget{newDLLMutationTarget(entry, entry.DLLs[0], "3.8.0")}
	for _, action := range []KeyAction{ActionDetailInstall, ActionDetailUpdate, ActionDetailRestore} {
		if label := content.DLLActionLabel(action); !strings.Contains(label, "Fixture") {
			t.Fatalf("%s label has no target name: %q", action, label)
		}
	}
	if label := content.DLLActionLabel(ActionDetailRestore); !strings.Contains(label, "1 file") {
		t.Fatalf("restore label has no backup target count: %q", label)
	}
	resource := makeDLLsResource([]*game.Game{entry}, map[string][]string{"dlss": {"3.8.0"}}, nil)
	if label := resource.DLLActionLabel(ActionDetailUpdate); !strings.Contains(label, "1 DLL") {
		t.Fatalf("catalog update label has no target count: %q", label)
	}
}
