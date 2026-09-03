package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
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
	for _, key := range []string{"q", "esc"} {
		if confirm, cancel := confirmation.update(keyMsg(key)); confirm || !cancel {
			t.Fatalf("%s = confirm %v, cancel %v", key, confirm, cancel)
		}
	}
	for _, key := range []string{"enter", "y", "Y"} {
		if confirm, cancel := confirmation.update(keyMsg(key)); !confirm || cancel {
			t.Fatalf("%s = confirm %v, cancel %v", key, confirm, cancel)
		}
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

	next, command := model.Update(keyMsg("U"))
	if command != nil || !next.busy {
		t.Fatal("busy update-all accepted duplicate execution")
	}
}
