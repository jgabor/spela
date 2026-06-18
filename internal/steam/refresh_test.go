package steam

import (
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
)

func TestRefreshIfNeeded_SkipsWhenPopulatedAndRescanDisabled(t *testing.T) {
	existing := &game.Database{
		Games: map[uint64]*game.Game{
			1: {AppID: 1, Name: "Alpha"},
		},
	}
	cfg := config.Default()
	cfg.RescanOnStartup = false

	got, err := RefreshIfNeeded(existing, cfg)
	if err != nil {
		t.Fatalf("RefreshIfNeeded() error = %v", err)
	}
	if got != existing {
		t.Fatal("expected existing database to be returned unchanged")
	}
}
