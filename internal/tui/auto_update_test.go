package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

func TestStartupAutoUpdateWaitsForTheStartupScan(t *testing.T) {
	services := testServices()
	services.LoadConfig = func() (*config.Config, error) {
		configuration := config.Default()
		configuration.AutoUpdateDLLs = true
		return configuration, nil
	}
	calls := 0
	services.AutoUpdateDLLs = func() dll.AutoUpdateResult {
		calls++
		return dll.AutoUpdateResult{}
	}
	// An empty library starts with a scan, so the update must follow it.
	layout := NewLayout(testDatabase(), services)
	if !layout.autoUpdateAfterRescan {
		t.Fatal("the startup update must wait for the startup scan")
	}
	settled, commands := layout.handleRescanGamesMsg(rescanGamesMsg{requestID: layout.rescanRequestID, db: testDatabase(testGame("Scanned"))}, nil)
	if settled.autoUpdateAfterRescan || len(commands) < 2 {
		t.Fatalf("finished scan did not start the update: pending %v, %d commands", settled.autoUpdateAfterRescan, len(commands))
	}
	if message, ok := settled.autoUpdateDLLs()().(autoUpdateMsg); !ok || calls != 1 {
		t.Fatalf("update command = %#v after %d calls", message, calls)
	}
	settled.rescanRequestID++
	_, again := settled.handleRescanGamesMsg(rescanGamesMsg{requestID: settled.rescanRequestID, db: testDatabase(testGame("Again"))}, nil)
	if len(again) != len(commands)-1 {
		t.Fatalf("a later rescan must not run the update again: %d commands, first scan had %d", len(again), len(commands))
	}

	services.LoadConfig = func() (*config.Config, error) { return config.Default(), nil }
	if off := NewLayout(testDatabase(), services); off.autoUpdateAfterRescan {
		t.Fatal("auto-update off must not schedule an update")
	}
}

func TestAutoUpdateResultRefreshesGamesAndShowsTheNotice(t *testing.T) {
	outdated := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "1.0.0"))
	layout := testLayout(outdated)
	updated := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "2.0.0"))
	result := dll.AutoUpdateResult{Batch: dll.BatchResult{
		Updated: 1,
		Items:   []dll.BatchItem{{Result: dll.Result{Outcome: dll.OutcomeChanged, FilesChanged: true, Game: updated}}},
	}}
	layout, _ = layout.handleAutoUpdateMsg(autoUpdateMsg{result: result}, nil)
	if got := layout.db.GetGame(outdated.AppID); got == nil || got.DLLs[0].Version != "2.0.0" {
		t.Fatalf("library still has the old DLL: %+v", got)
	}
	if view := stripANSI(layout.View().Content); !strings.Contains(view, "DLL auto-update: updated 1 DLL") {
		t.Fatalf("notice missing from the screen:\n%s", view)
	}

	quiet := testLayout(outdated)
	quiet, commands := quiet.handleAutoUpdateMsg(autoUpdateMsg{}, nil)
	if len(commands) != 0 || strings.Contains(stripANSI(quiet.View().Content), "auto-update") {
		t.Fatal("a current library must start without a message")
	}
}
