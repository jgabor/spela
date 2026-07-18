package tui

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func TestLayoutConstructionDialogAndRenderingBoundaryContracts(t *testing.T) {
	services := testServices()
	services.LoadConfig = func() (*config.Config, error) { return nil, errors.New("missing") }
	services.ScanGames = func(*config.Config) (*game.Database, error) { return nil, errors.New("offline") }
	layout := NewLayout(testDatabase(), services)
	if layout.config == nil || layout.Init() == nil {
		t.Fatal("default layout configuration or init command missing")
	}
	if message, ok := layout.rescanGames()().(rescanGamesMsg); !ok || message.err == nil {
		t.Fatalf("rescan error message = %#v", message)
	}

	dialog := NewOptionsModal(layout.styles)
	dialog.Open(config.Default())
	layout.activeDialog = &dialog
	model, _ := layout.Update(keyMsg("esc"))
	layout = model.(LayoutModel)
	if layout.activeDialog != nil {
		t.Fatal("closed active dialog remained attached")
	}

	if zoneLabel(nav.Zone(99)) != "Primary" || zoneLabel(nav.ZoneContext) != "Context" || zoneLabel(nav.ZoneContent) != "Content" {
		t.Fatal("zone label fallback changed")
	}
	if zoneColumnTitle("Scope", true) != "▸ Scope" || zoneColumnTitle("Scope", false) != "Scope" {
		t.Fatal("zone title focus marker changed")
	}
	if border := buildTopBorder("a title wider than its box", 4, color.White); border == "" {
		t.Fatal("narrow top border rendered empty")
	}
	if got := truncateHeight("one\ntwo\nthree", 2); got != "one\ntwo" {
		t.Fatalf("truncated content = %q", got)
	}
	if got := truncateHeight("one\ntwo", 3); got != "one\ntwo" {
		t.Fatalf("short content changed = %q", got)
	}
	layout.width, layout.height = 30, 10
	if modal := stripANSI(layout.renderModalBox("Title", "Body", 0.1)); !strings.Contains(modal, "Title") || !strings.Contains(modal, "Body") {
		t.Fatalf("modal rendering:\n%s", modal)
	}
	layout.batchGames = []*game.Game{{AppID: 1, Name: "Empty"}}
	if menu := stripANSI(layout.renderBatchContent()); !strings.Contains(menu, "Batch action") {
		t.Fatalf("batch rendering:\n%s", menu)
	}
}
