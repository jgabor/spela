package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
)

func TestRescanSerializesAdmissionAndBlocksNormalQuit(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			layout := testLayout(testGame("Initial"))
			layout, command := layout.dispatchAction(ActionRescanLibrary)
			if command == nil || !layout.rescanBusy || !layout.operationsPending() {
				t.Fatal("dispatched Rescan is not tracked as pending work")
			}
			requestID := layout.rescanRequestID
			for _, action := range []KeyAction{ActionRescanLibrary, ActionQuit} {
				resolution := CanonicalKeymap.Action(layout.bindingContext(), action)
				if resolution.Available || resolution.Reason == "" {
					t.Fatalf("%s is available or lacks an explanation during Rescan", action)
				}
				next, duplicate := layout.dispatchAction(action)
				if duplicate != nil || next.rescanRequestID != requestID {
					t.Fatalf("%s dispatched while the first Rescan is pending", action)
				}
			}
			if _, command := layout.requestQuit(); command != nil {
				t.Fatal("normal Quit bypassed the active Rescan")
			}
			completion := rescanGamesMsg{requestID: requestID, db: testDatabase(testGame("Rescanned"))}
			if failed {
				completion.db = nil
				completion.err = errors.New("fixture scan failed")
			}
			updated, _ := layout.Update(completion)
			layout = updated.(LayoutModel)
			if layout.rescanBusy || layout.operationsPending() {
				t.Fatal("settled Rescan still blocks work")
			}
			if failed && !strings.Contains(stripANSI(layout.View().Content), "fixture scan failed") {
				t.Fatal("failed Rescan lost its visible error")
			}
			if resolution := CanonicalKeymap.Action(layout.bindingContext(), ActionRescanLibrary); !resolution.Available {
				t.Fatal("settled Rescan cannot be retried")
			}
			_, command = layout.requestQuit()
			if command == nil {
				t.Fatal("normal Quit remains blocked after completion")
			}
			if _, ok := command().(tea.QuitMsg); !ok {
				t.Fatal("normal Quit returned a different action")
			}
		})
	}
}

func TestRescanCommandOwnsConfigurationSnapshot(t *testing.T) {
	layout := testLayout(testGame("Initial"))
	layout.config.SteamPath = "/fixture/original-steam"
	layout.config.AdditionalLibraryPaths = []string{"/fixture/original-library"}
	layout.services.ScanGames = func(configuration *config.Config) (*game.Database, error) {
		if configuration.SteamPath != "/fixture/original-steam" || configuration.AdditionalLibraryPaths[0] != "/fixture/original-library" {
			t.Fatalf("queued scan observed later configuration changes: %#v", configuration)
		}
		return testDatabase(), nil
	}
	layout, _ = layout.dispatchAction(ActionRescanLibrary)
	requestID := layout.rescanRequestID
	command := layout.rescanGames()
	layout.config.SteamPath = "/fixture/later-steam"
	layout.config.AdditionalLibraryPaths[0] = "/fixture/later-library"
	message, ok := command().(rescanGamesMsg)
	if !ok || message.err != nil || message.requestID != requestID {
		t.Fatalf("scan completion lost its request identity: %#v", message)
	}
}

func TestRescanIgnoresOlderGenerationAfterPermittedNewRequest(t *testing.T) {
	layout := testLayout(testGame("Initial"))
	layout, _ = layout.dispatchAction(ActionRescanLibrary)
	firstID := layout.rescanRequestID
	firstDatabase := testDatabase(testGame("First scan"))
	updated, _ := layout.Update(rescanGamesMsg{requestID: firstID, db: firstDatabase})
	layout = updated.(LayoutModel)
	layout, command := layout.dispatchAction(ActionRescanLibrary)
	if command == nil || layout.rescanRequestID == firstID {
		t.Fatal("next Rescan was not admitted after the first completed")
	}
	secondID := layout.rescanRequestID
	stale := rescanGamesMsg{requestID: firstID, db: testDatabase(testGame("Stale scan"))}
	updated, _ = layout.Update(stale)
	layout = updated.(LayoutModel)
	if !layout.rescanBusy || layout.db != firstDatabase || layout.pane.content.game.Name != "First scan" {
		t.Fatal("stale completion replaced data or cleared the newer operation")
	}
	latest := testDatabase(testGame("Latest scan"))
	updated, _ = layout.Update(rescanGamesMsg{requestID: secondID, db: latest})
	layout = updated.(LayoutModel)
	if layout.rescanBusy || layout.db != latest || layout.pane.content.game.Name != "Latest scan" {
		t.Fatal("current completion did not replace the scan data")
	}
	updated, _ = layout.Update(rescanGamesMsg{requestID: firstID, err: errors.New("stale failure")})
	layout = updated.(LayoutModel)
	if layout.db != latest || strings.Contains(stripANSI(layout.View().Content), "stale failure") {
		t.Fatal("stale failure replaced the current data or outcome")
	}
}
