package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func draftLayout(t *testing.T) LayoutModel {
	t.Helper()
	directory := t.TempDir()
	for _, entry := range []struct{ name, child string }{{"XDG_CONFIG_HOME", "config"}, {"XDG_DATA_HOME", "data"}, {"XDG_CACHE_HOME", "cache"}} {
		t.Setenv(entry.name, filepath.Join(directory, entry.child))
	}
	first := testGame("Cyberpunk 2077")
	second := testGame("Elden Ring", testDLL(game.DLLTypeDLSS, "3.8.10"))
	second.AppID = 1245620
	services := testServices()
	services.LoadProfile = profile.Load
	services.LoadDefaultProfile = profile.LoadDefault
	layout := NewLayout(testDatabase(first, second), services)
	sized, _ := layout.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	layout = sized.(LayoutModel)
	next := layout.listPane.sidebar.cloneSelection()
	next.cursor = 0
	layout, _ = layout.requestSidebar(next, true)
	return layout
}

func draftKey(layout *LayoutModel, key string) tea.Cmd {
	next, command := layout.Update(keyMsg(key))
	*layout = next.(LayoutModel)
	return command
}

func draftSelect(t *testing.T, layout *LayoutModel, appID uint64) tea.Cmd {
	t.Helper()
	next := layout.listPane.sidebar.cloneSelection()
	found := false
	for index, item := range next.filtered {
		if appID == 0 && item.kind == sidebarItemDefaultProfile || item.game != nil && item.game.AppID == appID {
			next.cursor, found = index, true
			break
		}
	}
	if !found {
		t.Fatalf("missing selection %d", appID)
	}
	updated, command := layout.requestSidebar(next, true)
	*layout = updated
	return command
}

func draftEditHDR(t *testing.T, layout *LayoutModel) {
	t.Helper()
	if layout.navState.Scope.Kind == nav.ScopeGame {
		layout.navState.Aspect = nav.AspectProfile
		layout.syncNavToComponents()
	}
	layout.focus = FocusDetail
	focusField(t, layout.pane.profileDetail(), profile.FieldProtonEnableHDR)
	draftKey(layout, "space")
	if layout.pane.profileDetail().Editing() || !layout.pane.profileDetail().Dirty() || !layout.pane.profileDetail().RawProfile().Proton.EnableHDR {
		t.Fatal("keyboard setup did not create an HDR draft")
	}
}

// Only execute commands returned by a Save choice. Layout result commands also
// contain notification timers and must not be treated as persistence work.
func draftSaveResults(t *testing.T, command tea.Cmd) []tea.Msg {
	t.Helper()
	if command == nil {
		t.Fatal("Save returned no command")
	}
	message := command()
	if batch, ok := message.(tea.BatchMsg); ok {
		var messages []tea.Msg
		for _, child := range batch {
			messages = append(messages, draftSaveResults(t, child)...)
		}
		return messages
	}
	switch message.(type) {
	case profileSaveMsg, optionsSavedMsg, optionsSaveErrorMsg:
		return []tea.Msg{message}
	default:
		t.Fatalf("unexpected Save result %T", message)
		return nil
	}
}

func draftComplete(layout *LayoutModel, message tea.Msg) tea.Cmd {
	next, _ := layout.handleAppMessages(message, nil)
	next, command := next.advanceDecision(message)
	*layout = next
	return command
}

func TestDraftNavigationSameItemReopenAndDestinationDetour(t *testing.T) {
	for _, appID := range []uint64{0, 1091500} {
		t.Run(map[bool]string{true: "root", false: "game"}[appID == 0], func(t *testing.T) {
			layout := draftLayout(t)
			if appID != 0 {
				draftSelect(t, &layout, appID)
			}
			draftEditHDR(t, &layout)
			draftKey(&layout, "down")
			field, cursor := layout.pane.profileDetail().FocusedField(), layout.pane.profileDetail().Cursor()
			for _, key := range []string{"4", "1", "enter"} {
				draftKey(&layout, key)
			}
			detail := layout.pane.profileDetail()
			if layout.decision != nil || !detail.Dirty() || !detail.RawProfile().Proton.EnableHDR || detail.FocusedField() != field || detail.Cursor() != cursor {
				t.Fatal("destination detour or same-item reopen lost draft/focus or prompted")
			}
			if layout.focus != FocusDetail || appID != 0 && layout.navState.Aspect != nav.AspectOverview {
				t.Fatal("reopening did not follow the scope's declared landing view")
			}
		})
	}
}

func TestDraftNavigationScopeTransitionsResolveSaveDiscardCancel(t *testing.T) {
	for _, transition := range []struct {
		name     string
		from, to uint64
	}{{"root to game", 0, 1091500}, {"game to root", 1091500, 0}, {"game to game", 1091500, 1245620}} {
		for _, choice := range []string{"Cancel", "Save", "Discard"} {
			t.Run(transition.name+"/"+choice, func(t *testing.T) {
				layout := draftLayout(t)
				if transition.from != 0 {
					draftSelect(t, &layout, transition.from)
				}
				draftEditHDR(t, &layout)
				beforeScope, beforeCursor := layout.navState.Scope, layout.listPane.sidebar.cursor
				if command := draftSelect(t, &layout, transition.to); command != nil || layout.decision == nil || layout.decision.focus != 0 {
					t.Fatal("dirty transition was not held behind a default-Cancel decision")
				}
				if layout.navState.Scope != beforeScope || layout.listPane.sidebar.cursor != beforeCursor {
					t.Fatal("transition changed scope or cursor before the decision")
				}
				if choice != "Cancel" {
					layout, _ = layout.handleDecisionAction(ActionOverlayRight)
				}
				if choice == "Discard" {
					layout, _ = layout.handleDecisionAction(ActionOverlayRight)
				}
				var command tea.Cmd
				layout, command = layout.handleDecisionAction(ActionOverlayConfirm)
				if choice == "Save" {
					if layout.navState.Scope != beforeScope || layout.decision == nil || !layout.decision.saving {
						t.Fatal("Save advanced the selection before persistence")
					}
					for _, message := range draftSaveResults(t, command) {
						draftComplete(&layout, message)
					}
				}
				if choice == "Cancel" {
					if command != nil || layout.decision != nil || layout.navState.Scope != beforeScope || !layout.pane.profileDetail().Dirty() {
						t.Fatal("Cancel lost the original draft or scope")
					}
					return
				}
				if layout.decision != nil || layout.navState.Scope.AppID != transition.to || layout.profileDirty() {
					t.Fatalf("resolved transition left stale state: scope=%+v decision=%+v", layout.navState.Scope, layout.decision)
				}
				var persisted *profile.Profile
				if transition.from == 0 {
					persisted, _ = profile.LoadDefault()
				} else {
					persisted, _ = profile.Load(transition.from)
				}
				if saved := persisted != nil && persisted.Proton.EnableHDR; saved != (choice == "Save") {
					t.Fatalf("%s persistence = %v", choice, saved)
				}
			})
		}
	}
}

func TestDraftNavigationFailedSaveRetainsSelectionAndCanRetry(t *testing.T) {
	layout := draftLayout(t)
	draftEditHDR(t, &layout)
	beforeScope, beforeCursor := layout.navState.Scope, layout.listPane.sidebar.cursor
	draftSelect(t, &layout, 1091500)
	layout, _ = layout.handleDecisionAction(ActionOverlayRight)
	configurationHome := os.Getenv("XDG_CONFIG_HOME")
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", blocked)
	layout, command := layout.handleDecisionAction(ActionOverlayConfirm)
	for _, message := range draftSaveResults(t, command) {
		draftComplete(&layout, message)
	}
	if layout.decision == nil || layout.decision.saving || layout.decision.error == "" || !layout.pane.defaultsDetail.Dirty() || layout.navState.Scope != beforeScope || layout.listPane.sidebar.cursor != beforeCursor {
		t.Fatal("failed save lost the decision, error, original draft, or selection")
	}
	t.Setenv("XDG_CONFIG_HOME", configurationHome)
	layout, command = layout.handleDecisionAction(ActionOverlayConfirm)
	for _, message := range draftSaveResults(t, command) {
		draftComplete(&layout, message)
	}
	if layout.decision != nil || layout.navState.Scope.AppID != 1091500 || layout.profileDirty() {
		t.Fatal("retry did not save and complete the held transition")
	}
}

func TestDraftNavigationFilterAndSearchAreAtomicWhenDraftWouldBeReplaced(t *testing.T) {
	for _, operation := range []string{"filter", "search"} {
		t.Run(operation, func(t *testing.T) {
			layout := draftLayout(t)
			draftSelect(t, &layout, 1091500)
			draftEditHDR(t, &layout)
			layout.focus = FocusList
			beforeScope, beforeCursor := layout.navState.Scope, layout.listPane.sidebar.cursor
			if operation == "filter" {
				layout, _ = layout.updateListAction(ActionListToggleDLLFilter)
			} else {
				layout, _ = layout.startSearch()
				draftKey(&layout, "Z")
			}
			if layout.decision == nil || layout.navState.Scope != beforeScope || layout.listPane.sidebar.cursor != beforeCursor || layout.listPane.sidebar.filters.IsActive() || layout.listPane.sidebar.search.Value() != "" {
				t.Fatal("filter/search replaced dirty scope or changed controls before confirmation")
			}
			layout, _ = layout.handleDecisionAction(ActionOverlayConfirm)
			if layout.decision != nil || !layout.pane.content.detail.Dirty() || layout.navState.Scope != beforeScope {
				t.Fatal("Cancel lost the dirty original scope")
			}
			if operation == "search" {
				for _, key := range []string{"tab", "tab", "tab", "enter"} {
					draftKey(&layout, key)
				}
				if layout.searchSession != nil || layout.inputMode != ModeBrowse || layout.listPane.sidebar.search.Value() != "" {
					t.Fatal("search Cancel did not restore Browse with the original query")
				}
			}
		})
	}
}

func TestDraftNavigationSearchCancelRestoresQueryAndSelection(t *testing.T) {
	layout := draftLayout(t)
	draftSelect(t, &layout, 1091500)
	next := layout.listPane.sidebar.cloneSelection()
	next.search.SetValue("Cyber")
	next.applyFiltersAndSort()
	layout, _ = layout.requestSidebar(next, false)
	layout, _ = layout.startSearch()
	for _, key := range []string{"home", "X", "tab", "tab", "tab", "enter"} {
		draftKey(&layout, key)
	}
	selected := layout.listPane.sidebar.Selected()
	if layout.searchSession != nil || layout.listPane.sidebar.search.Value() != "Cyber" || layout.navState.Scope.AppID != 1091500 || selected == nil || selected.AppID != 1091500 {
		t.Fatalf("search Cancel lost query/selection: query=%q scope=%+v", layout.listPane.sidebar.search.Value(), layout.navState.Scope)
	}
}

func TestDraftNavigationQuitSavesBothDocumentsAndStopsOnAnyFailure(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{true: "Settings failure", false: "success"}[failure], func(t *testing.T) {
			layout := draftLayout(t)
			draftEditHDR(t, &layout)
			layout.pane.settings.SyncNavSection(nav.SettingsDisplay)
			layout.pane.settings.cycleValue(1)
			if failure {
				layout.pane.settings.SetSaveConfig(func(*config.Config) error { return errors.New("read-only Settings") })
			}
			layout, command := layout.requestQuit()
			if command != nil || layout.decision == nil || !strings.Contains(layout.decision.body, "Settings") || !strings.Contains(layout.decision.body, "default profile") {
				t.Fatal("Quit did not name both unsaved documents")
			}
			layout, _ = layout.handleDecisionAction(ActionOverlayRight)
			layout, command = layout.handleDecisionAction(ActionOverlayConfirm)
			messages := draftSaveResults(t, command)
			if len(messages) != 2 {
				t.Fatalf("Quit dispatched %d writes, want 2", len(messages))
			}
			if command := draftComplete(&layout, messages[0]); command != nil || layout.decision == nil || !layout.decision.saving {
				t.Fatal("Quit advanced before both writes completed")
			}
			command = draftComplete(&layout, messages[1])
			if failure {
				if command != nil || layout.decision == nil || layout.decision.error == "" || !layout.pane.settings.Dirty() || layout.pane.defaultsDetail.Dirty() {
					t.Fatal("failed Settings save exited or lost the remaining draft")
				}
			} else {
				if layout.decision != nil || command == nil {
					t.Fatal("successful saves did not complete Quit")
				}
				if _, ok := command().(tea.QuitMsg); !ok {
					t.Fatal("Quit continuation returned another action")
				}
			}
		})
	}
}

func TestDraftNavigationQuitWaitsForPendingWrites(t *testing.T) {
	for _, operation := range []string{"active profile", "queued profile", "Settings", "game DLL", "catalog DLL", "batch DLL", "install DLL"} {
		t.Run(operation, func(t *testing.T) {
			layout := draftLayout(t)
			switch operation {
			case "active profile":
				layout.pane.content.profileSaves.active = &profileSaveRequest{appID: 999}
			case "queued profile":
				layout.pane.content.profileSaves.queued = []profileSaveRequest{{appID: 999}}
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
			if command != nil || layout.decision != nil {
				t.Fatal("Quit passed a dispatched or queued write")
			}
		})
	}
}

func TestDraftNavigationOlderSaveResultPreservesLaterEdits(t *testing.T) {
	for _, appID := range []uint64{0, 1091500} {
		t.Run(map[bool]string{true: "root", false: "game"}[appID == 0], func(t *testing.T) {
			layout := draftLayout(t)
			if appID != 0 {
				draftSelect(t, &layout, appID)
			}
			draftEditHDR(t, &layout)
			messages := draftSaveResults(t, draftKey(&layout, "ctrl+s"))
			for _, key := range []string{"down", "space"} {
				draftKey(&layout, key)
			}
			for _, message := range messages {
				draftComplete(&layout, message)
			}
			detail := layout.pane.profileDetail()
			if !detail.Dirty() || !detail.RawProfile().Proton.EnableWayland || !detail.RawProfile().Proton.EnableHDR || detail.persisted.Proton.EnableWayland {
				t.Fatal("earlier layout save result replaced or marked later edits saved")
			}
		})
	}
}

func TestDraftNavigationRescanReplacementRequiresDecision(t *testing.T) {
	layout := draftLayout(t)
	draftSelect(t, &layout, 1091500)
	draftEditHDR(t, &layout)
	beforeDatabase, beforeScope, beforeCursor := layout.db, layout.navState.Scope, layout.listPane.sidebar.cursor
	replacement := testDatabase(layout.db.GetGame(1245620))
	layout, _ = layout.handleRescanGamesMsg(rescanGamesMsg{db: replacement}, nil)
	if layout.decision == nil || layout.db != beforeDatabase || layout.navState.Scope != beforeScope || layout.listPane.sidebar.cursor != beforeCursor {
		t.Fatal("rescan replaced dirty selection data before Save/Discard/Cancel")
	}
	layout, _ = layout.handleDecisionAction(ActionOverlayConfirm)
	if !layout.pane.content.detail.Dirty() || layout.db != beforeDatabase || layout.navState.Scope != beforeScope {
		t.Fatal("Cancel did not retain the pre-rescan draft and selection")
	}
}

func TestDraftNavigationRescanSameItemRetainsProfileView(t *testing.T) {
	layout := draftLayout(t)
	draftSelect(t, &layout, 1091500)
	draftEditHDR(t, &layout)
	draftKey(&layout, "down")
	field := layout.pane.content.detail.FocusedField()
	refreshed := *layout.db.GetGame(1091500)
	refreshed.InstallDir = filepath.Join(t.TempDir(), "new location")
	layout, _ = layout.handleRescanGamesMsg(rescanGamesMsg{db: testDatabase(&refreshed, layout.db.GetGame(1245620))}, nil)
	if layout.decision != nil || layout.navState.Aspect != nav.AspectProfile || layout.pane.content.detail.FocusedField() != field || !layout.pane.content.detail.Dirty() || layout.pane.content.game.InstallDir != refreshed.InstallDir {
		t.Fatal("same-item rescan lost Profile view/draft/focus or failed to refresh metadata")
	}
}

func TestDraftNavigationRescanWaitsForActiveEditor(t *testing.T) {
	for _, choice := range []string{"Apply", "Cancel"} {
		t.Run(choice, func(t *testing.T) {
			layout := draftLayout(t)
			draftSelect(t, &layout, 1091500)
			layout.navState.Aspect = nav.AspectProfile
			layout.syncNavToComponents()
			layout.focus = FocusDetail
			focusField(t, &layout.pane.content.detail, profile.FieldGPUShaderCachePath)
			draftKey(&layout, "enter")
			const draftPath = "/rescan draft 01234"
			for _, character := range draftPath {
				draftKey(&layout, string(character))
			}
			beforeDatabase, beforeScope := layout.db, layout.navState.Scope
			replacement := testDatabase(layout.db.GetGame(1245620))
			updated, _ := layout.Update(rescanGamesMsg{db: replacement})
			layout = updated.(LayoutModel)
			if layout.pendingRescan == nil || layout.db != beforeDatabase || layout.navState.Scope != beforeScope || !layout.pane.content.detail.Editing() || layout.decision != nil {
				t.Fatal("rescan result replaced or interrupted the active field editor")
			}
			if choice == "Cancel" {
				draftKey(&layout, "tab")
				draftKey(&layout, "tab")
			}
			draftKey(&layout, "enter")
			if layout.pendingRescan != nil {
				t.Fatal("leaving the editor did not resume the pending rescan")
			}
			if choice == "Apply" {
				if layout.decision == nil || layout.db != beforeDatabase || layout.navState.Scope != beforeScope || !layout.pane.content.detail.Dirty() || layout.pane.content.detail.RawProfile().GPU.ShaderCachePath != draftPath {
					t.Fatal("applied editor value was replaced without a draft decision")
				}
			} else if layout.decision != nil || layout.db != replacement || layout.navState.Scope == beforeScope || layout.profileDirty() {
				t.Fatal("cancelled editor did not allow the pending rescan to complete")
			}
		})
	}
}
