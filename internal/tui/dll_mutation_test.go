package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func TestDLLMutationConfirmationSummaryAndCancel(t *testing.T) {
	entry := testGame("Fixture Game", testDLL(game.DLLTypeDLSS, "3.7.0"))
	entry.DLLs[0].Path = "/tmp/spela-fixture/nvngx_dlss.dll"
	confirmation := newDLLMutationConfirmation("Confirm DLL update", []dllMutationTarget{newDLLMutationTarget(entry, entry.DLLs[0], "3.8.0")}, "current DLL is backed up")

	view := stripANSI(confirmation.view(NewStyles(DefaultTheme, false)))
	for _, want := range []string{"Fixture Game", "DLSS", entry.DLLs[0].Path, "3.7.0 → 3.8.0", "Backup: current DLL is backed up"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}
	if confirm, cancel := confirmation.update(keyMsg("enter")); confirm || !cancel {
		t.Fatalf("initial Enter = confirm %v, cancel %v", confirm, cancel)
	}
	for _, key := range []string{"q", "esc", "y", "Y"} {
		if confirm, cancel := confirmation.update(keyMsg(key)); confirm || cancel {
			t.Fatalf("removed shortcut %s altered confirmation", key)
		}
	}
	confirmation.update(keyMsg("right"))
	if confirm, cancel := confirmation.update(keyMsg("enter")); !confirm || cancel {
		t.Fatalf("explicit Confirm = confirm %v, cancel %v", confirm, cancel)
	}
}

func TestLatestDLLMutationTargetsUseConcreteVersions(t *testing.T) {
	entry := testGame("Fixture Game", testDLL(game.DLLTypeDLSS, "3.7.0"), testDLL(game.DLLTypeXeSS, "1.3.1"))
	manifest := &dll.Manifest{DLLs: map[string][]dll.DLL{
		"dlss": {{Version: "3.8.0"}},
		"xess": {{Version: "1.3.1"}},
	}}
	targets := latestDLLMutationTargets([]*game.Game{entry}, manifest)
	if len(targets) != 1 || targets[0].path != entry.DLLs[0].Path || targets[0].targetVersion != "3.8.0" {
		t.Fatalf("targets = %+v", targets)
	}
}

func TestDLLMutationBusySuppressesDuplicate(t *testing.T) {
	entry := testGame("Fixture Game", testDLL(game.DLLTypeDLSS, "3.7.0"))
	model := makeDLLsResource([]*game.Game{entry}, map[string][]string{"dlss": {"3.8.0"}}, nil)
	model.busy = true

	next, command := model.UpdateAction(ActionDetailUpdate)
	if command != nil || !next.busy {
		t.Fatal("busy update-all accepted duplicate execution")
	}
}

func TestContentDLLCancellationRetainsAccurateOutcomeWithoutMutation(t *testing.T) {
	for _, action := range []string{"install", "update", "restore"} {
		for _, key := range []string{"enter"} {
			t.Run(action+"/"+key, func(t *testing.T) {
				entry, services, fixturePath, original, calls := newDLLCancellationFixture(t)
				content := NewContent(NewStyles(DefaultTheme, true), true, services)
				content.SetSize(100, 30)
				content.database = testDatabase(entry)
				content = content.SetGame(entry)
				content.lastDLLResult = "stale success or failure"

				switch action {
				case "install":
					content.dllInstallState = DLLInstallSelectVersion
					content.dllOperating = false
					content.selectedDLLType = "dlss"
					content.dllVersions = []dll.DLL{{Version: "3.8.0", Filename: filepath.Base(fixturePath)}}
				case "update":
					content.hasUpdates = true
					content.dllUpdateTargets = []dllMutationTarget{newDLLMutationTarget(entry, entry.DLLs[0], "3.8.0")}
				case "restore":
					content.hasBackup = true
				}

				openAction := map[string]KeyAction{"install": ActionOverlayConfirm, "update": ActionDetailUpdate, "restore": ActionDetailRestore}[action]
				content, command := content.UpdateDLLAction(openAction)
				if command != nil || content.confirmation == nil {
					t.Fatalf("%s did not stop at confirmation", action)
				}
				content, command = content.Update(keyMsg(key))
				if command != nil || content.confirmation != nil || content.pendingAction != PendingNone || content.dllOperating {
					t.Fatalf("cancel state = command %v, confirmation %v, pending %v, operating %v", command, content.confirmation != nil, content.pendingAction, content.dllOperating)
				}
				want := dllCancellationResult("DLL " + action)
				view := stripANSI(content.ViewDLLAspect())
				if !strings.Contains(view, want) || strings.Contains(view, "stale success or failure") {
					t.Fatalf("retained cancellation outcome is inaccurate:\n%s", view)
				}
				assertDLLFixtureUnchanged(t, fixturePath, original, *calls)
			})
		}
	}
}

func TestCatalogDLLCancellationReplacesStaleOutcomeWithoutMutation(t *testing.T) {
	for _, key := range []string{"enter"} {
		t.Run(key, func(t *testing.T) {
			entry, services, fixturePath, original, calls := newDLLCancellationFixture(t)
			resource := makeDLLsResourceWithServices([]*game.Game{entry}, map[string][]string{"dlss": {"3.8.0"}}, services)
			resource.lastBatchSummary = "stale update-all success or failure"
			resource.lastBatchResult = map[string]string{"1091500:dlss": "err: stale"}

			resource, command := resource.UpdateAction(ActionDetailUpdate)
			if command != nil || resource.confirmation == nil {
				t.Fatal("catalog update-all did not stop at confirmation")
			}
			resource, command = resource.Update(keyMsg(key))
			view := stripANSI(resource.View(true, nav.SectionDLLDeployment))
			if command != nil || resource.confirmation != nil || resource.busy || resource.lastBatchResult != nil {
				t.Fatalf("catalog cancel retained executable or stale state: %+v", resource)
			}
			if want := dllCancellationResult("DLL update-all"); !strings.Contains(view, want) || strings.Contains(view, "stale update-all success or failure") {
				t.Fatalf("catalog cancellation outcome is inaccurate:\n%s", view)
			}
			assertDLLFixtureUnchanged(t, fixturePath, original, *calls)
		})
	}
}

func TestSelectedGameBatchCancellationReplacesStaleOutcomeWithoutMutation(t *testing.T) {
	for _, key := range []string{"enter"} {
		t.Run(key, func(t *testing.T) {
			entry, services, fixturePath, original, calls := newDLLCancellationFixture(t)
			layout := NewLayout(testDatabase(entry), services)
			layout.width, layout.height = 120, 40
			layout.calculateDimensions()
			layout.batchMessage = "stale batch success or failure"
			opened, _ := layout.Update(batchActionRequestMsg{selected: []*game.Game{entry}})
			layout = opened.(LayoutModel)
			layout.pane.dllsResource.manifest = &dll.Manifest{DLLs: map[string][]dll.DLL{"dlss": {{Version: "3.8.0"}}}}

			result, command := sendKey(&layout, "enter")
			layout = result.(LayoutModel)
			if command != nil || layout.batchConfirmation == nil {
				t.Fatal("selected-game batch did not stop at confirmation")
			}
			result, command = sendKey(&layout, key)
			layout = result.(LayoutModel)
			view := stripANSI(layout.renderBatchMenu())
			if command != nil || layout.batchConfirmation != nil || layout.batchBusy || !layout.showBatchMenu {
				t.Fatalf("selected-game batch cancel retained executable state: %+v", layout)
			}
			if want := dllCancellationResult("Selected-game DLL batch"); !strings.Contains(view, want) || strings.Contains(view, "stale batch success or failure") {
				t.Fatalf("selected-game batch cancellation outcome is inaccurate:\n%s", view)
			}
			assertDLLFixtureUnchanged(t, fixturePath, original, *calls)
		})
	}
}

func newDLLCancellationFixture(t *testing.T) (*game.Game, *Services, string, []byte, *int) {
	t.Helper()
	directory := t.TempDir()
	fixturePath := filepath.Join(directory, "nvngx_dlss.dll")
	original := []byte("original fixture DLL")
	if err := os.WriteFile(fixturePath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	entry := testGame("Fixture Game", testDLL(game.DLLTypeDLSS, "3.7.0"))
	entry.InstallDir = directory
	entry.DLLs[0].Path = fixturePath
	calls := new(int)
	mutate := func() {
		(*calls)++
		_ = os.WriteFile(fixturePath, []byte("mutated"), 0o600)
	}
	services := testServices()
	services.LoadDLLBackup = func(appID uint64) (*dll.Backup, error) {
		return &dll.Backup{AppID: appID, Files: []dll.BackedUpFile{{OriginalPath: fixturePath, DLLName: filepath.Base(fixturePath)}}}, nil
	}
	services.BatchUpdateDLLs = func([]dll.UpdateRequest) dll.BatchResult {
		mutate()
		return dll.BatchResult{Updated: 1}
	}
	services.InstallDLL = func(uint64, string, string) (dll.Result, error) {
		mutate()
		return dll.Result{Outcome: dll.OutcomeChanged}, nil
	}
	services.RestoreDLLs = func(uint64) (dll.Result, error) {
		mutate()
		return dll.Result{Outcome: dll.OutcomeChanged}, nil
	}
	return entry, services, fixturePath, original, calls
}

func assertDLLFixtureUnchanged(t *testing.T, path string, original []byte, calls int) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || string(contents) != string(original) {
		t.Fatalf("cancelled mutation calls = %d, fixture = %q", calls, contents)
	}
}
