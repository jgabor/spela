package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
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
	m.hasUpdates = true
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

func TestContent_ModalRoutesBeforePendingAction(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.pendingAction = PendingDLLUpdate
	m.dlssPresetModal.Open(profile.DLSSPresetDefault)

	result, cmd := m.Update(keyMsg("y"))
	if !result.dlssPresetModal.Visible() {
		t.Error("expected DLSS preset modal to remain open after unrelated key")
	}
	if result.pendingAction != PendingDLLUpdate {
		t.Errorf("modal routing should not consume pending action, got %d", result.pendingAction)
	}
	if result.dllOperating {
		t.Error("pending action must not start while modal is open")
	}
	if cmd != nil {
		t.Error("expected no command from unrelated modal key")
	}
}

func TestContent_Update_WithoutConfirmation(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasUpdates = true
	m.confirmDestructive = false

	result, cmd := m.Update(keyMsg("u"))
	if result.pendingAction != PendingNone {
		t.Error("expected no pending action when confirmation disabled")
	}
	if !result.dllOperating {
		t.Error("expected dllOperating to be true")
	}
	if cmd == nil {
		t.Error("expected update command returned directly")
	}
}

func TestContent_Update_NoUpdatesIgnored(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasUpdates = false

	_, cmd := m.Update(keyMsg("u"))
	if cmd != nil {
		t.Error("expected u to be ignored when no updates available")
	}
}

// ---------------------------------------------------------------------------
// DLL Restore — confirmation flow
// ---------------------------------------------------------------------------

func TestContent_Restore_WithConfirmation(t *testing.T) {
	// Task 5 displaced the restore DLLs binding from "R" to "ctrl+shift+r"
	// so bare Shift+R can reset the entire profile to inherited.
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasBackup = true
	m.confirmDestructive = true

	result, cmd := m.Update(keyMsg("ctrl+shift+r"))
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

	_, cmd := m.Update(keyMsg("ctrl+shift+r"))
	if cmd != nil {
		t.Error("expected ctrl+shift+r to be ignored when no backup exists")
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
	if result.dllInstallState != DLLInstallDownloading {
		t.Errorf("expected DLLInstallDownloading, got %d", result.dllInstallState)
	}
	if cmd == nil {
		t.Error("expected installSelectedDLL command")
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

	result, _ := m.Update(dllUpdateMsg{success: true, dlls: g.DLLs})
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

	result, _ := m.Update(dllRestoreMsg{success: true})
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
