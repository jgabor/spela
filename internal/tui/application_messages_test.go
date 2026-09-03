package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func TestLayoutApplicationMessagesMaintainCrossComponentState(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", stateRoot+"/config")
	t.Setenv("XDG_DATA_HOME", stateRoot+"/data")
	t.Setenv("XDG_CACHE_HOME", stateRoot+"/cache")
	entry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.7.0"))
	layout := testLayoutWithGame(entry)

	updated, commands := layout.handleAppMessages(batchActionRequestMsg{selected: []*game.Game{entry}}, nil)
	if !updated.showBatchMenu || len(updated.batchGames) != 1 || len(commands) != 0 {
		t.Fatalf("batch request state = %+v, commands %d", updated, len(commands))
	}
	updated, commands = updated.handleAppMessages(batchCompleteMsg{message: "Updated 1 game"}, commands)
	if updated.batchMessage != "Updated 1 game" || len(commands) == 0 {
		t.Fatalf("batch completion = message %q, commands %d", updated.batchMessage, len(commands))
	}
	updated, _ = updated.handleAppMessages(batchCompleteMsg{message: "Updated 0 games, 1 failed", failed: true}, nil)
	if updated.messageBar.messageType != MessageError {
		t.Fatal("failed TUI batch was posted as success")
	}
	updated, _ = updated.handleAppMessages(batchCompleteMsg{message: "Selected games are already current", batch: dll.BatchResult{Unchanged: 1}}, nil)
	if updated.messageBar.messageType != MessageInfo || !strings.Contains(updated.messageBar.message, "already current") {
		t.Fatalf("all-current batch message = %q, %v", updated.messageBar.message, updated.messageBar.messageType)
	}

	configuration := updated.config
	updated.pane.settings.saving, updated.pane.settings.modified = true, true
	updated, commands = updated.handleAppMessages(optionsSavedMsg{config: configuration.Clone()}, nil)
	if !configsEqual(updated.config, configuration) || !configsEqual(updated.pane.settings.config, configuration) || updated.pane.settings.saving || updated.pane.settings.modified || len(commands) == 0 {
		t.Fatal("options save did not preserve config ownership, reset state, and announce success")
	}
	updated.pane.settings.saving, updated.pane.settings.modified = true, true
	updated, commands = updated.handleAppMessages(optionsSaveErrorMsg{err: errors.New("read-only")}, nil)
	if updated.pane.settings.saving || !updated.pane.settings.modified || len(commands) == 0 {
		t.Fatal("options save failure did not preserve retry state and announce error")
	}

	updated.pane.content.dllOperating = true
	updatedGame := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	updated, commands = updated.handleAppMessages(dllUpdateMsg{batch: dll.BatchResult{Updated: 1, Items: []dll.BatchItem{{Result: dll.Result{Outcome: dll.OutcomeChanged, Game: updatedGame}}}}}, nil)
	if updated.pane.content.dllOperating || updated.pane.content.game.DLLs[0].Version != "3.8.10" || len(commands) == 0 {
		t.Fatal("DLL update success did not synchronize content and message state")
	}
	updated, _ = updated.handleAppMessages(dllUpdateMsg{batch: dll.BatchResult{Unchanged: 1}}, nil)
	if updated.messageBar.messageType != MessageInfo || updated.messageBar.message != "DLLs already up to date" {
		t.Fatalf("DLL no-op message = %q, %v", updated.messageBar.message, updated.messageBar.messageType)
	}
	updated.pane.content.dllOperating = true
	updated, commands = updated.handleAppMessages(dllRestoreMsg{err: errors.New("backup missing")}, nil)
	if updated.pane.content.dllOperating || len(commands) == 0 {
		t.Fatal("DLL restore failure did not clear operation and announce error")
	}
	updated.pane.content.dllOperating = true
	updated.pane.content.dllInstallState = DLLInstallDownloading
	updated, commands = updated.handleAppMessages(dllInstallMsg{err: errors.New("download failed")}, nil)
	if updated.pane.content.dllOperating || len(commands) == 0 {
		t.Fatal("DLL install failure did not clear operation and announce error")
	}

	replacement := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.9.0"))
	rescanned := testDatabase(replacement)
	updated, commands = updated.handleAppMessages(rescanGamesMsg{db: rescanned}, nil)
	if updated.db != rescanned || updated.pane.content.game.DLLs[0].Version != "3.9.0" || len(commands) < 2 {
		t.Fatalf("rescan synchronization = db %v, version %s, commands %d", updated.db == rescanned, updated.pane.content.game.DLLs[0].Version, len(commands))
	}
	updated, commands = updated.handleAppMessages(rescanGamesMsg{err: errors.New("Steam unavailable")}, nil)
	if len(commands) == 0 {
		t.Fatal("rescan failure did not announce error")
	}
	updated, commands = updated.handleAppMessages(profileSaveMsg{}, nil)
	if len(commands) == 0 {
		t.Fatal("profile save did not announce success")
	}
	updated, commands = updated.handleAppMessages(contentNoticeMsg{text: "notice", messageType: MessageInfo}, nil)
	if len(commands) == 0 {
		t.Fatal("content notice did not reach message bar")
	}

	view := stripANSI(updated.View().Content)
	if !strings.Contains(view, "Cyberpunk 2077") {
		t.Fatalf("message transitions lost active game:\n%s", view)
	}

	updated, _ = updated.handleAppMessages(defaultProfileSelectedMsg{}, nil)
	if updated.navState.Scope.Kind != nav.ScopeGlobal {
		t.Fatal("default profile selection did not restore global context scope")
	}
	updated, _ = updated.handleAppMessages(defaultProfileConfirmedMsg{}, nil)
	if updated.focus != FocusDetail {
		t.Fatal("default profile confirmation did not focus Detail")
	}
	updated, commands = updated.handleAppMessages(gameSelectedMsg{game: entry}, nil)
	updated, _ = updated.handleAppMessages(gameConfirmedMsg{game: entry}, commands)
	if updated.focus != FocusDetail || updated.pane.content.game != entry {
		t.Fatal("game messages did not synchronize selected content")
	}
	updated.pane.content.dllInstallState = DLLInstallSelectType
	updated, commands = updated.handleAppMessages(dllTypesLoadedMsg{types: []string{"dlss"}}, nil)
	updated.pane.content.dllInstallState = DLLInstallSelectVersion
	updated, commands = updated.handleAppMessages(dllVersionsLoadedMsg{}, commands)
	updated.pane.content.dllInstallState = DLLInstallNone
	updated, _ = updated.handleAppMessages(dllUpdatesCheckedMsg{hasUpdates: true}, commands)
	if !updated.pane.content.hasUpdates || len(updated.pane.content.dllTypes) != 1 {
		t.Fatal("DLL loading messages did not synchronize content")
	}
	updated, commands = updated.handleAppMessages(dllRestoreMsg{result: dll.Result{Outcome: dll.OutcomeChanged}}, nil)
	updated, commands = updated.handleAppMessages(dllInstallMsg{result: dll.Result{Outcome: dll.OutcomeChanged}}, commands)
	updated, commands = updated.handleAppMessages(profileSaveMsg{err: errors.New("read-only")}, commands)
	if len(commands) < 3 {
		t.Fatalf("success/error messages did not schedule notices: %d", len(commands))
	}
	updated, _ = updated.handleAppMessages(messageClearMsg{}, nil)
	updated, _ = updated.handleAppMessages(flashTickMsg{}, nil)

	updated.showBatchMenu = true
	updated.batchCursor = 1
	updated, _, _ = updated.handleBatchMenuKeys(keyMsg("up"))
	updated, _, _ = updated.handleBatchMenuKeys(keyMsg("down"))
	updated, _, _ = updated.handleBatchMenuKeys(keyMsg("esc"))
	if updated.showBatchMenu {
		t.Fatal("batch escape did not close menu")
	}
	updated.showHelp = true
	updated.showBatchMenu = true
	updated.batchGames = []*game.Game{entry}
	if modalView := stripANSI(updated.View().Content); !strings.Contains(modalView, "Keyboard") || !strings.Contains(modalView, "Batch action") {
		t.Fatalf("stacked layout overlays missing:\n%s", modalView)
	}
}

func TestPerGameDLLOutcomesRemainAfterMessageBarClears(t *testing.T) {
	entry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	partial := &dll.PartialFailure{
		Result: dll.Result{Outcome: dll.OutcomeChanged, FilesChanged: true, Game: entry},
		Stage:  dll.StageSaving,
		Err:    errors.New("disk full"),
	}
	tests := []struct {
		name    string
		message interface{}
		want    string
	}{
		{name: "success", message: dllUpdateMsg{batch: dll.BatchResult{Updated: 1, Items: []dll.BatchItem{{Result: dll.Result{Outcome: dll.OutcomeChanged, Game: entry}}}}}, want: "DLL update: 1 updated, 0 current, 0 failed"},
		{name: "no-op", message: dllUpdateMsg{batch: dll.BatchResult{Unchanged: 1}}, want: "DLLs already up to date"},
		{name: "denied", message: dllInstallMsg{err: errors.New("DLL swap denied for app 1091500")}, want: "Install failed: DLL swap denied"},
		{name: "partial", message: dllRestoreMsg{result: partial.Result, err: partial}, want: "DLL files changed but metadata did not fully persist"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := testLayoutWithGame(entry)
			updated, _ := layout.handleAppMessages(test.message, nil)
			timestamp := updated.messageBar.timestamp
			updated, _ = updated.handleAppMessages(messageClearMsg{timestamp: timestamp}, nil)
			if updated.messageBar.HasMessage() {
				t.Fatal("message bar did not clear")
			}
			view := stripANSI(updated.pane.content.ViewDLLAspect())
			if !strings.Contains(view, test.want) {
				t.Fatalf("retained outcome missing %q:\n%s", test.want, view)
			}
		})
	}
}
