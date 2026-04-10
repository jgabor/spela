package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

// ---------------------------------------------------------------------------
// No-game guards
// ---------------------------------------------------------------------------

func TestContent_NoGame_DLLKeysIgnored(t *testing.T) {
	m := testContent(nil)

	for _, key := range []string{"i", "u", "R", "L"} {
		t.Run(key, func(t *testing.T) {
			result, cmd := m.Update(keyMsg(key))
			updated := result
			if cmd != nil {
				t.Errorf("expected no command from %s without game, got cmd", key)
			}
			_ = updated
		})
	}
}

func TestContent_NoGame_TabSwitchIgnored(t *testing.T) {
	m := testContent(nil)

	for _, key := range []string{"2", "3", "4"} {
		t.Run(key, func(t *testing.T) {
			result, _ := m.Update(keyMsg(key))
			if result.activeTab != TabDLLs {
				t.Errorf("expected tab to remain at default without game")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tab switching
// ---------------------------------------------------------------------------

func TestContent_TabSwitch(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)

	tests := []struct {
		key     string
		wantTab ContentTab
	}{
		{"2", TabDLLs},
		{"3", TabProfile},
		{"4", TabLaunch},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result, _ := m.Update(keyMsg(tt.key))
			if result.activeTab != tt.wantTab {
				t.Errorf("expected tab %d, got %d", tt.wantTab, result.activeTab)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Launch
// ---------------------------------------------------------------------------

func TestContent_Launch(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)

	result, cmd := m.Update(keyMsg("L"))
	if !result.launching {
		t.Error("expected launching to be true")
	}
	if cmd == nil {
		t.Error("expected launch command to be returned")
	}
}

func TestContent_Launch_DuplicatePrevented(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.launching = true // already launching

	_, cmd := m.Update(keyMsg("L"))
	if cmd != nil {
		t.Error("expected no command when launch already in progress")
	}
}

func TestContent_Launch_DefaultProfileIgnored(t *testing.T) {
	m := testContent(nil)
	m.defaultProfile = true

	_, cmd := m.Update(keyMsg("L"))
	if cmd != nil {
		t.Error("expected L to be ignored on default profile view")
	}
}

func TestContent_LaunchMsg_ClearsLaunching(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testContent(g)
	m.launching = true

	result, _ := m.Update(launchGameMsg{success: true})
	if result.launching {
		t.Error("expected launchGameMsg to clear launching flag")
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

	// u sets pending action, does NOT return a command
	result, cmd := m.Update(keyMsg("u"))
	if result.pendingAction != PendingDLLUpdate {
		t.Errorf("expected PendingDLLUpdate, got %d", result.pendingAction)
	}
	if cmd != nil {
		t.Error("expected no command when confirmation is pending")
	}

	// Y confirms and returns the update command
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

	// Any key other than Y cancels
	result, _ = result.Update(keyMsg("n"))
	if result.pendingAction != PendingNone {
		t.Error("expected pending action cleared on cancel")
	}
	if result.dllOperating {
		t.Error("expected dllOperating to remain false on cancel")
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
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testContent(g)
	m.hasBackup = true
	m.confirmDestructive = true

	result, cmd := m.Update(keyMsg("R"))
	if result.pendingAction != PendingDLLRestore {
		t.Errorf("expected PendingDLLRestore, got %d", result.pendingAction)
	}
	if cmd != nil {
		t.Error("expected no command when confirmation pending")
	}

	// Confirm
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

	_, cmd := m.Update(keyMsg("R"))
	if cmd != nil {
		t.Error("expected R to be ignored when no backup exists")
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

	// Navigate down
	result, _ := m.Update(keyMsg("down"))
	if result.dllTypeCursor != 1 {
		t.Errorf("expected cursor 1, got %d", result.dllTypeCursor)
	}

	// Navigate up
	result, _ = result.Update(keyMsg("up"))
	if result.dllTypeCursor != 0 {
		t.Errorf("expected cursor 0, got %d", result.dllTypeCursor)
	}

	// Clamp at 0
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
