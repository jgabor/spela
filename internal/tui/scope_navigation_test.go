package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

// changeHDR turns HDR on from the keyboard on the open profile and returns the
// command that saves it. The change is written at once; nothing waits for Save.
func changeHDR(t *testing.T, layout *LayoutModel) tea.Cmd {
	t.Helper()
	if layout.navState.Scope.Kind == nav.ScopeGame {
		layout.navState.Aspect = nav.AspectProfile
		layout.syncNavToComponents()
	}
	layout.focus = FocusDetail
	focusField(t, layout.pane.profileDetail(), profile.FieldProtonEnableHDR)
	command := sessionKey(layout, "space")
	detail := layout.pane.profileDetail()
	if detail.Editing() || !detail.RawProfile().Proton.EnableHDR || command == nil {
		t.Fatal("Space did not turn HDR on and request a save")
	}
	return command
}

// immediateMessages runs a command and returns what it produces within a short
// time. Timers from the message bar are left behind instead of waited for.
func immediateMessages(command tea.Cmd) []tea.Msg {
	if command == nil {
		return nil
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- command() }()
	select {
	case message := <-result:
		if batch, ok := message.(tea.BatchMsg); ok {
			var messages []tea.Msg
			for _, child := range batch {
				messages = append(messages, immediateMessages(child)...)
			}
			return messages
		}
		return []tea.Msg{message}
	case <-time.After(150 * time.Millisecond):
		return nil
	}
}

// saveResults returns the save results a command produces.
func saveResults(t *testing.T, command tea.Cmd) []profileSaveMsg {
	t.Helper()
	var results []profileSaveMsg
	for _, message := range immediateMessages(command) {
		if result, ok := message.(profileSaveMsg); ok {
			results = append(results, result)
		}
	}
	if len(results) == 0 {
		t.Fatal("the command produced no save result")
	}
	return results
}

// finish delivers a message to the layout as the program would and returns the
// commands that follow.
func finish(layout *LayoutModel, message tea.Msg) tea.Cmd {
	updated, command := layout.Update(message)
	*layout = updated.(LayoutModel)
	return command
}

func TestAutosaveSameItemReopenKeepsFocusAndCursor(t *testing.T) {
	for _, appID := range []uint64{0, 1091500} {
		t.Run(map[bool]string{true: "root", false: "game"}[appID == 0], func(t *testing.T) {
			layout := sessionLayout(t)
			if appID != 0 {
				sessionSelect(t, &layout, appID)
			}
			changeHDR(t, &layout)
			sessionKey(&layout, "down")
			field, cursor := layout.pane.profileDetail().FocusedField(), layout.pane.profileDetail().Cursor()
			for _, key := range []string{"4", "1", "enter"} {
				sessionKey(&layout, key)
			}
			detail := layout.pane.profileDetail()
			if layout.decision != nil || !detail.RawProfile().Proton.EnableHDR || detail.FocusedField() != field || detail.Cursor() != cursor {
				t.Fatal("destination detour or same-item reopen lost the change or focus, or asked a question")
			}
			if layout.focus != FocusDetail || appID != 0 && layout.navState.Aspect != nav.AspectProfile {
				t.Fatal("reopening did not land on the profile")
			}
		})
	}
}

func TestAutosaveMovingBetweenScopesNeverAsksAndKeepsTheChange(t *testing.T) {
	for _, transition := range []struct {
		name     string
		from, to uint64
	}{{"root to game", 0, 1091500}, {"game to root", 1091500, 0}, {"game to game", 1091500, 1245620}} {
		for _, timing := range []string{"after the write", "during the write"} {
			t.Run(transition.name+"/"+timing, func(t *testing.T) {
				layout := sessionLayout(t)
				if transition.from != 0 {
					sessionSelect(t, &layout, transition.from)
				}
				results := saveResults(t, changeHDR(t, &layout))
				if timing == "after the write" {
					for _, result := range results {
						finish(&layout, result)
					}
				}
				sessionSelect(t, &layout, transition.to)
				if layout.decision != nil || layout.navState.Scope.AppID != transition.to {
					t.Fatalf("moving asked a question or stayed: scope=%+v decision=%+v", layout.navState.Scope, layout.decision)
				}
				if timing == "during the write" {
					for _, result := range results {
						finish(&layout, result)
					}
				}
				var persisted *profile.Profile
				if transition.from == 0 {
					persisted, _ = profile.LoadDefault()
				} else {
					persisted, _ = profile.Load(transition.from)
				}
				if persisted == nil || !persisted.Proton.EnableHDR {
					t.Fatal("the change did not reach the file")
				}
			})
		}
	}
}

func TestAutosaveFailedWriteReturnsTheRowToTheStoredValueAndCanBeRepeated(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	configurationHome := os.Getenv("XDG_CONFIG_HOME")
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", blocked)
	for _, result := range saveResults(t, changeHDR(t, &layout)) {
		if result.err == nil {
			t.Fatal("writing under a plain file succeeded")
		}
		finish(&layout, result)
	}
	detail := layout.pane.profileDetail()
	if detail.RawProfile().Proton.EnableHDR || detail.Dirty() || detail.SaveError() == nil {
		t.Fatal("a failed write kept the unsaved value or lost the error")
	}
	if status := detail.SaveStatus(); status == "Saved at once" || status == "Saving…" {
		t.Fatalf("header status after a failed write = %q", status)
	}

	t.Setenv("XDG_CONFIG_HOME", configurationHome)
	for _, result := range saveResults(t, sessionKey(&layout, "space")) {
		finish(&layout, result)
	}
	saved, err := profile.Load(1091500)
	if err != nil || saved == nil || !saved.Proton.EnableHDR || layout.pane.profileDetail().SaveError() != nil {
		t.Fatalf("repeating the change did not save: profile=%#v err=%v", saved, err)
	}
}

func TestAutosaveFilterAndSearchNeverAskWhileAWriteRuns(t *testing.T) {
	for _, operation := range []string{"filter", "search"} {
		t.Run(operation, func(t *testing.T) {
			layout := sessionLayout(t)
			sessionSelect(t, &layout, 1091500)
			changeHDR(t, &layout)
			layout.focus = FocusList
			if operation == "filter" {
				layout, _ = layout.updateListAction(ActionListToggleDLLFilter)
			} else {
				layout, _ = layout.startSearch()
				sessionKey(&layout, "Z")
			}
			if layout.decision != nil {
				t.Fatal("filter or search asked a question")
			}
		})
	}
}

func TestSearchCancelRestoresQueryAndSelection(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	next := layout.listPane.sidebar.cloneSelection()
	next.search.SetValue("Cyber")
	next.applyFiltersAndSort()
	layout, _ = layout.applySidebar(next, false)
	layout, _ = layout.startSearch()
	for _, key := range []string{"home", "X", "tab", "tab", "tab", "enter"} {
		sessionKey(&layout, key)
	}
	selected := layout.listPane.sidebar.Selected()
	if layout.searchSession != nil || layout.listPane.sidebar.search.Value() != "Cyber" || layout.navState.Scope.AppID != 1091500 || selected == nil || selected.AppID != 1091500 {
		t.Fatalf("search Cancel lost query/selection: query=%q scope=%+v", layout.listPane.sidebar.search.Value(), layout.navState.Scope)
	}
}

func containsQuit(messages []tea.Msg) bool {
	for _, message := range messages {
		if _, ok := message.(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}

func TestQuitWaitsForWritesInFlightThenQuits(t *testing.T) {
	layout := sessionLayout(t)
	results := saveResults(t, changeHDR(t, &layout))
	layout, command := layout.requestQuit()
	if !layout.quitWhenIdle || containsQuit(immediateMessages(command)) {
		t.Fatal("Quit did not wait for the write")
	}
	var after []tea.Msg
	for _, result := range results {
		after = append(after, immediateMessages(finish(&layout, result))...)
	}
	if !containsQuit(after) {
		t.Fatal("Quit did not follow the finished write")
	}
	saved, err := profile.LoadDefault()
	if err != nil || !saved.Proton.EnableHDR {
		t.Fatalf("the write was lost on the way out: profile=%#v err=%v", saved, err)
	}
}

func TestQuitIsImmediateWhenNothingIsPending(t *testing.T) {
	layout := sessionLayout(t)
	layout, command := layout.requestQuit()
	if layout.quitWhenIdle || !containsQuit(immediateMessages(command)) {
		t.Fatal("Quit with nothing pending did not quit at once")
	}
}

func TestQuitWaitsForSettingsWriteAndRefusesOtherWork(t *testing.T) {
	for _, operation := range []string{"Settings", "game DLL", "catalog DLL", "batch DLL", "install DLL"} {
		t.Run(operation, func(t *testing.T) {
			layout := sessionLayout(t)
			switch operation {
			case "Settings":
				layout.pane.settings.saving = true
			case "game DLL":
				layout.pane.content.dllOperating = true
			case "catalog DLL":
				layout.pane.dllsResource.busy = true
			case "batch DLL":
				layout.batchBusy = true
			case "install DLL":
				layout.pane.content.dllInstallState = DLLInstallDownloading
			}
			layout, command := layout.requestQuit()
			if containsQuit(immediateMessages(command)) {
				t.Fatal("Quit passed unfinished work")
			}
			if layout.quitWhenIdle != (operation == "Settings") {
				t.Fatalf("quitWhenIdle = %v for %s", layout.quitWhenIdle, operation)
			}
		})
	}
}

func TestAutosaveOlderResultPreservesLaterChanges(t *testing.T) {
	for _, appID := range []uint64{0, 1091500} {
		t.Run(map[bool]string{true: "root", false: "game"}[appID == 0], func(t *testing.T) {
			layout := sessionLayout(t)
			if appID != 0 {
				sessionSelect(t, &layout, appID)
			}
			first := saveResults(t, changeHDR(t, &layout))
			sessionKey(&layout, "down")
			sessionKey(&layout, "space")
			for _, result := range first {
				finish(&layout, result)
			}
			detail := layout.pane.profileDetail()
			if !detail.Dirty() || !detail.RawProfile().Proton.EnableWayland || !detail.RawProfile().Proton.EnableHDR || detail.persisted.Proton.EnableWayland {
				t.Fatal("the earlier result replaced or marked the later change saved")
			}
		})
	}
}

func TestRescanDuringAWriteIsAppliedWithoutAskingAndKeepsTheChange(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	results := saveResults(t, changeHDR(t, &layout))
	replacement := testDatabase(layout.db.GetGame(1245620))
	layout, _ = layout.handleRescanGamesMsg(rescanGamesMsg{db: replacement}, nil)
	if layout.decision != nil || layout.db != replacement {
		t.Fatal("the rescan was held or asked a question")
	}
	for _, result := range results {
		finish(&layout, result)
	}
	saved, err := profile.Load(1091500)
	if err != nil || saved == nil || !saved.Proton.EnableHDR {
		t.Fatalf("the rescan lost the change: profile=%#v err=%v", saved, err)
	}
}

func TestRescanSameItemRetainsProfileViewAndFocus(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	changeHDR(t, &layout)
	sessionKey(&layout, "down")
	field := layout.pane.content.detail.FocusedField()
	refreshed := *layout.db.GetGame(1091500)
	refreshed.InstallDir = filepath.Join(t.TempDir(), "new location")
	layout, _ = layout.handleRescanGamesMsg(rescanGamesMsg{db: testDatabase(&refreshed, layout.db.GetGame(1245620))}, nil)
	if layout.decision != nil || layout.navState.Aspect != nav.AspectProfile || layout.pane.content.detail.FocusedField() != field || !layout.pane.content.detail.RawProfile().Proton.EnableHDR || layout.pane.content.game.InstallDir != refreshed.InstallDir {
		t.Fatal("same-item rescan lost the Profile view, the change, or the focus, or failed to refresh metadata")
	}
}

func TestRescanWaitsForActiveEditor(t *testing.T) {
	for _, choice := range []string{"Apply", "Cancel"} {
		t.Run(choice, func(t *testing.T) {
			layout := sessionLayout(t)
			sessionSelect(t, &layout, 1091500)
			layout.navState.Aspect = nav.AspectProfile
			layout.syncNavToComponents()
			layout.focus = FocusDetail
			focusField(t, &layout.pane.content.detail, profile.FieldGPUShaderCachePath)
			sessionKey(&layout, "enter")
			const typed = "/rescan edit 01234"
			for _, character := range typed {
				sessionKey(&layout, string(character))
			}
			beforeDatabase, beforeScope := layout.db, layout.navState.Scope
			replacement := testDatabase(layout.db.GetGame(1245620))
			updated, _ := layout.Update(rescanGamesMsg{db: replacement})
			layout = updated.(LayoutModel)
			if layout.pendingRescan == nil || layout.db != beforeDatabase || layout.navState.Scope != beforeScope || !layout.pane.content.detail.Editing() || layout.decision != nil {
				t.Fatal("rescan result replaced or interrupted the active field editor")
			}
			if choice == "Cancel" {
				sessionKey(&layout, "tab")
				sessionKey(&layout, "tab")
			}
			command := sessionKey(&layout, "enter")
			if layout.pendingRescan != nil || layout.decision != nil || layout.db != replacement {
				t.Fatal("leaving the editor did not let the pending rescan complete")
			}
			if choice == "Apply" {
				for _, result := range saveResults(t, command) {
					if result.err != nil {
						t.Fatalf("saving the applied value failed: %v", result.err)
					}
				}
				saved, err := profile.Load(1091500)
				if err != nil || saved == nil || saved.GPU.ShaderCachePath != typed {
					t.Fatalf("applied value did not reach the file: profile=%#v err=%v", saved, err)
				}
			} else if saved, _ := profile.Load(1091500); saved != nil && saved.GPU.ShaderCachePath != "" {
				t.Fatal("cancelled edit reached the file")
			}
		})
	}
}

func TestAutosaveAnnouncesWhatWasSavedAndTheHeaderFollowsTheWrite(t *testing.T) {
	layout := sessionLayout(t)
	sessionSelect(t, &layout, 1091500)
	results := saveResults(t, changeHDR(t, &layout))
	if status := layout.pane.profileDetail().SaveStatus(); status != "Saving…" {
		t.Fatalf("status while the write runs = %q", status)
	}
	for _, result := range results {
		finish(&layout, result)
	}
	if layout.messageBar.message != "Saved: HDR On" || layout.messageBar.messageType != MessageSuccess {
		t.Fatalf("message after the write = %q (%v)", layout.messageBar.message, layout.messageBar.messageType)
	}
	if status := layout.pane.profileDetail().SaveStatus(); status != "Saved at once" {
		t.Fatalf("status after the write = %q", status)
	}

	sessionSelect(t, &layout, 0)
	for _, result := range saveResults(t, changeHDR(t, &layout)) {
		finish(&layout, result)
	}
	if layout.messageBar.message != "Saved: HDR On (All games)" {
		t.Fatalf("All games message = %q", layout.messageBar.message)
	}
}
