package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

// Content tests focus on the games-resource detail pane — the only
// interactive surface inside ContentModel after Task 3. The previous
// ContentTab enum (DLLs/Profile/Launch) and the Launch tab itself are
// gone; all DLL actions live directly in the single game-detail view.

// ---------------------------------------------------------------------------
// No-game guards
// ---------------------------------------------------------------------------

func TestContent_NoGame_DLLKeysIgnored(t *testing.T) {
	m := testContent(nil)

	// L is no longer a binding (Launch tab removed).
	for _, key := range []string{"i", "u", "R"} {
		t.Run(key, func(t *testing.T) {
			result, cmd := m.Update(keyMsg(key))
			if cmd != nil {
				t.Errorf("expected no command from %s without game, got cmd", key)
			}
			_ = result
		})
	}
}

// ---------------------------------------------------------------------------
// DLL Update — confirmation flow
// ---------------------------------------------------------------------------

func TestContent_Update_WithConfirmation(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	var requests []dll.UpdateRequest
	m.services.BatchUpdateDLLs = func(received []dll.UpdateRequest) dll.BatchResult {
		requests = append(requests, received...)
		return dll.BatchResult{Updated: len(received)}
	}
	m.hasUpdates = true
	m.dllUpdateTargets = []dllMutationTarget{newDLLMutationTarget(g, g.DLLs[0], "3.9.0")}
	m.confirmDestructive = true

	result, cmd := m.Update(keyMsg("u"))
	if result.pendingAction != PendingDLLUpdate {
		t.Errorf("expected PendingDLLUpdate, got %d", result.pendingAction)
	}
	if cmd != nil {
		t.Error("expected no command when confirmation is pending")
	}

	result, cmd = result.Update(keyMsg("Y"))
	if result.pendingAction != PendingNone {
		t.Error("expected pending action cleared after confirmation")
	}
	if !result.dllOperating {
		t.Error("expected dllOperating to be true after confirmation")
	}
	if cmd == nil {
		t.Error("expected update command after Y confirmation")
	}
	execCmd(cmd)
	if len(requests) != 1 || requests[0].Version != "3.9.0" || requests[0].InstalledPath != g.DLLs[0].Path {
		t.Fatalf("executed targets = %+v", requests)
	}
}

func TestContent_Update_ConfirmationCancelled(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasUpdates = true
	m.confirmDestructive = true

	result, _ := m.Update(keyMsg("u"))
	if result.pendingAction != PendingDLLUpdate {
		t.Fatal("precondition: should be pending")
	}

	result, _ = result.Update(keyMsg("n"))
	if result.pendingAction != PendingNone {
		t.Error("expected pending action cleared on cancel")
	}
	if result.dllOperating {
		t.Error("expected dllOperating to remain false on cancel")
	}
}

func TestContent_PerGameConfirmationEnterAndYExecute(t *testing.T) {
	for _, action := range []string{"update", "restore"} {
		for _, key := range []string{"enter", "y", "Y"} {
			t.Run(action+"/"+key, func(t *testing.T) {
				entry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
				content := testContent(entry)
				if action == "update" {
					content.hasUpdates = true
					content.dllUpdateTargets = []dllMutationTarget{newDLLMutationTarget(entry, entry.DLLs[0], "3.9.0")}
					content, _ = content.Update(keyMsg("u"))
				} else {
					content.hasBackup = true
					content, _ = content.Update(keyMsg("f6"))
				}
				content, command := content.Update(keyMsg(key))
				if command == nil || !content.dllOperating || content.pendingAction != PendingNone {
					t.Fatalf("state = operating %v, pending %v, command %v", content.dllOperating, content.pendingAction, command)
				}
			})
		}
	}
}

func TestContent_Update_ConfigCannotBypassConfirmation(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasUpdates = true
	m.confirmDestructive = false

	result, cmd := m.Update(keyMsg("u"))
	if result.pendingAction != PendingDLLUpdate {
		t.Error("expected confirmation even when persisted setting is false")
	}
	if result.dllOperating {
		t.Error("operation started before confirmation")
	}
	if cmd != nil {
		t.Error("unexpected update command before confirmation")
	}
}

func TestContent_Update_NoUpdatesFeedback(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasUpdates = false

	_, cmd := m.Update(keyMsg("u"))
	if cmd == nil {
		t.Error("expected feedback command when no updates available")
	}
}

// ---------------------------------------------------------------------------
// DLL Restore — confirmation flow
// ---------------------------------------------------------------------------

func TestContent_Restore_WithConfirmation(t *testing.T) {
	// F6 is distinguishable on terminals that cannot distinguish Ctrl+Shift+R,
	// leaving bare Shift+R available to reset the entire profile to inherited.
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasBackup = true
	m.confirmDestructive = true

	result, cmd := m.Update(keyMsg("f6"))
	if result.pendingAction != PendingDLLRestore {
		t.Errorf("expected PendingDLLRestore, got %d", result.pendingAction)
	}
	if cmd != nil {
		t.Error("expected no command when confirmation pending")
	}

	result, cmd = result.Update(keyMsg("y"))
	if !result.dllOperating {
		t.Error("expected dllOperating after Y")
	}
	if cmd == nil {
		t.Error("expected restore command after confirmation")
	}
}

func TestContent_Restore_NoBackupIgnored(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasBackup = false

	_, cmd := m.Update(keyMsg("f6"))
	if cmd != nil {
		t.Error("expected F6 to be ignored when no backup exists")
	}
}

// ---------------------------------------------------------------------------
// DLL Install wizard
// ---------------------------------------------------------------------------

func TestContent_InstallWizard_Start(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)

	result, cmd := m.Update(keyMsg("i"))
	if result.dllInstallState != DLLInstallSelectType {
		t.Errorf("expected DLLInstallSelectType, got %d", result.dllInstallState)
	}
	if !result.dllOperating {
		t.Error("expected dllOperating to be true")
	}
	if cmd == nil {
		t.Error("expected loadDLLTypes command")
	}
}

func TestContent_InstallWizard_AlreadyOperating(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllOperating = true

	result, cmd := m.Update(keyMsg("i"))
	if result.dllInstallState != DLLInstallNone {
		t.Error("expected i to be ignored when already operating")
	}
	if cmd != nil {
		t.Error("expected no command when already operating")
	}
}

func TestContent_InstallWizard_TypeSelection(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllInstallState = DLLInstallSelectType
	m.dllOperating = true
	m.dllTypes = []string{"dlss", "dlssg", "dlssd"}
	m.dllTypeCursor = 0

	result, _ := m.Update(keyMsg("down"))
	if result.dllTypeCursor != 1 {
		t.Errorf("expected cursor 1, got %d", result.dllTypeCursor)
	}

	result, _ = result.Update(keyMsg("up"))
	if result.dllTypeCursor != 0 {
		t.Errorf("expected cursor 0, got %d", result.dllTypeCursor)
	}

	result, _ = result.Update(keyMsg("up"))
	if result.dllTypeCursor != 0 {
		t.Error("expected cursor to clamp at 0")
	}
}

func TestContent_InstallWizard_TypeToVersion(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllInstallState = DLLInstallSelectType
	m.dllOperating = true
	m.dllTypes = []string{"dlss"}
	m.dllTypeCursor = 0

	result, cmd := m.Update(keyMsg("enter"))
	if result.dllInstallState != DLLInstallSelectVersion {
		t.Errorf("expected DLLInstallSelectVersion, got %d", result.dllInstallState)
	}
	if result.selectedDLLType != "dlss" {
		t.Errorf("expected selectedDLLType 'dlss', got %q", result.selectedDLLType)
	}
	if cmd == nil {
		t.Error("expected loadDLLVersions command")
	}
}

func TestContent_InstallWizard_VersionToDownload(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllInstallState = DLLInstallSelectVersion
	m.dllOperating = true
	m.dllVersions = []dll.DLL{{Version: "3.9.0"}, {Version: "3.8.10"}}
	m.dllVersionCursor = 0

	result, cmd := m.Update(keyMsg("enter"))
	if result.confirmation == nil || cmd != nil {
		t.Fatal("expected install confirmation before execution")
	}
	result, cmd = result.Update(keyMsg("enter"))
	if result.dllInstallState != DLLInstallDownloading {
		t.Errorf("expected DLLInstallDownloading, got %d", result.dllInstallState)
	}
	if cmd == nil {
		t.Error("expected installSelectedDLL command")
	}
}

func TestContent_InstallConfirmationListsOnlySelectedFamilyAndPath(t *testing.T) {
	entry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"), testDLL(game.DLLTypeXeSS, "1.3.1"))
	content := testContent(entry)
	content.dllInstallState = DLLInstallSelectVersion
	content.dllOperating = true
	content.selectedDLLType = "dlss"
	content.dllVersions = []dll.DLL{{Version: "3.9.0", Filename: entry.DLLs[0].Name}}

	content, command := content.Update(keyMsg("enter"))
	if command != nil || content.confirmation == nil {
		t.Fatal("expected install confirmation before execution")
	}
	view := stripANSI(content.ViewDLLAspect())
	for _, want := range []string{"DLSS", entry.DLLs[0].Path, "3.8.10 → 3.9.0"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, entry.DLLs[1].Path) || strings.Contains(view, "1.3.1") {
		t.Fatalf("confirmation included unselected XeSS DLL:\n%s", view)
	}
}

func TestContent_InstallWizard_Cancel(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllInstallState = DLLInstallSelectType
	m.dllOperating = true
	m.dllTypes = []string{"dlss"}

	result, _ := m.Update(keyMsg("esc"))
	if result.dllInstallState != DLLInstallNone {
		t.Error("expected esc to cancel install wizard")
	}
	if result.dllOperating {
		t.Error("expected dllOperating to be false after cancel")
	}
}

func TestContent_InstallWizard_QCancel(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllInstallState = DLLInstallSelectVersion
	m.dllOperating = true
	m.dllVersions = []dll.DLL{{Version: "3.9.0"}}

	result, _ := m.Update(keyMsg("q"))
	if result.dllInstallState != DLLInstallNone {
		t.Error("expected q to cancel install wizard")
	}
}

// ---------------------------------------------------------------------------
// Message handlers
// ---------------------------------------------------------------------------

func TestContent_DLLUpdateMsg_ClearsOperating(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllOperating = true

	result, _ := m.Update(dllUpdateMsg{batch: dll.BatchResult{Updated: 1, Items: []dll.BatchItem{{Result: dll.Result{Outcome: dll.OutcomeChanged, Game: g}}}}})
	if result.dllOperating {
		t.Error("expected dllOperating to be cleared")
	}
	if result.hasUpdates {
		t.Error("expected hasUpdates to be false after successful update")
	}
}

func TestContent_DLLRestoreMsg_ClearsOperating(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.dllOperating = true

	result, _ := m.Update(dllRestoreMsg{result: dll.Result{Outcome: dll.OutcomeChanged}})
	if result.dllOperating {
		t.Error("expected dllOperating to be cleared after restore")
	}
}

func TestContent_DLLUpdatesCheckedMsg(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasUpdates = false

	result, _ := m.Update(dllUpdatesCheckedMsg{hasUpdates: true})
	if !result.hasUpdates {
		t.Error("expected hasUpdates to be set to true")
	}
}
