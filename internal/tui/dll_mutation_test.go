package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
)

func TestDLLMutationConfirmationSummaryAndCancel(t *testing.T) {
	entry := testGame("Fixture Game", testDLL(game.DLLTypeDLSS, "3.7.0"))
	entry.DLLs[0].Path = "/tmp/spela-fixture/nvngx_dlss.dll"
	confirmation := newDLLMutationConfirmation("Confirm DLL update", []*game.Game{entry}, "3.8.0", "current DLL is backed up")

	view := stripANSI(confirmation.view(NewStyles(DefaultTheme, false)))
	for _, want := range []string{"Fixture Game", entry.DLLs[0].Path, "3.7.0 → 3.8.0", "Backup: current DLL is backed up"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}
	if confirm, cancel := confirmation.update(keyMsg("q")); confirm || !cancel {
		t.Fatalf("q = confirm %v, cancel %v", confirm, cancel)
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
