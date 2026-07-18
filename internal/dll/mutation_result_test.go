package dll

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
)

func TestRefreshGameAfterMutationDefinesSuccessAndPartialOutcomes(t *testing.T) {
	entry := &game.Game{InstallDir: "/game", DLLs: []game.DetectedDLL{{Version: "old"}}}
	scanError := errors.New("scan denied")
	saveCalled := false
	err := RefreshGameAfterMutation(entry, "install", func(path string) ([]game.DetectedDLL, error) {
		if path != "/game" {
			t.Fatalf("scan path = %q", path)
		}
		return nil, scanError
	}, func() error { saveCalled = true; return nil })
	if !errors.Is(err, scanError) || !strings.Contains(err.Error(), "scan install directory") || saveCalled || entry.DLLs[0].Version != "old" {
		t.Fatalf("post-mutation scan outcome = error %v, save %v, game %+v", err, saveCalled, entry)
	}

	saveError := errors.New("disk full")
	err = RefreshGameAfterMutation(entry, "install", func(string) ([]game.DetectedDLL, error) {
		return []game.DetectedDLL{{Version: "new"}}, nil
	}, func() error { return saveError })
	if !errors.Is(err, saveError) || !strings.Contains(err.Error(), "save game database after install") || entry.DLLs[0].Version != "new" || entry.ScannedAt.IsZero() {
		t.Fatalf("post-mutation save outcome = error %v, game %+v", err, entry)
	}

	err = RefreshGameAfterMutation(entry, "update", func(string) ([]game.DetectedDLL, error) {
		return []game.DetectedDLL{{Version: "latest"}}, nil
	}, func() error { return nil })
	if err != nil || entry.DLLs[0].Version != "latest" {
		t.Fatalf("successful mutation refresh = error %v, game %+v", err, entry)
	}
}
