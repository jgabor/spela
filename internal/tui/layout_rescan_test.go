package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
)

func TestLayout_InitRescanOnStartup(t *testing.T) {
	svc := testServices()
	svc.LoadConfig = func() (*config.Config, error) {
		cfg := config.Default()
		cfg.RescanOnStartup = true
		return cfg, nil
	}
	svc.ScanGames = func(cfg *config.Config) (*game.Database, error) {
		return testDatabase(testGame("Rescanned")), nil
	}

	m := NewLayout(testDatabase(), svc)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected startup rescan command")
	}
}

func TestLayout_RescanUsesScanGamesService(t *testing.T) {
	svc := testServices()
	scanned := testDatabase(testGame("Rescanned"))
	svc.ScanGames = func(cfg *config.Config) (*game.Database, error) {
		return scanned, nil
	}

	m := testLayout()
	m.services = svc
	msg := execCmd(m.rescanGames())
	rescanMsg, ok := msg.(rescanGamesMsg)
	if !ok {
		t.Fatalf("expected rescanGamesMsg, got %T", msg)
	}
	if rescanMsg.err != nil {
		t.Fatalf("rescanGamesMsg error = %v", rescanMsg.err)
	}
	if len(rescanMsg.db.Games) != 1 {
		t.Fatalf("expected scanned database, got %d games", len(rescanMsg.db.Games))
	}
}
