package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func layoutWithSettings(t *testing.T, change func(*config.Config), games ...*game.Game) LayoutModel {
	t.Helper()
	services := testServices()
	services.LoadConfig = func() (*config.Config, error) {
		configuration := config.Default()
		configuration.RescanOnStartup = false
		change(configuration)
		return configuration, nil
	}
	return NewLayout(testDatabase(games...), services)
}

func TestConfirmDestructiveOffStartsEveryUpdateAtOnce(t *testing.T) {
	entry := testGame("Fixture", testDLL(game.DLLTypeDLSS, "3.7.0"))
	off := func(configuration *config.Config) { configuration.ConfirmDestructive = false }

	layout := layoutWithSettings(t, off, entry)
	if !layout.pane.content.skipUpdateConfirmation || !layout.pane.dllsResource.skipUpdateConfirmation {
		t.Fatal("the setting did not reach the game and catalog views")
	}
	layout.width, layout.height = 120, 40
	layout.selectDestination(nav.DestinationDLLCatalog)
	layout.navState.DLLCatalogSection = nav.SectionDLLDeployment
	layout.pane.dllsResource.cached = map[string][]string{"dlss": {"3.8.0"}}
	layout.calculateDimensions()
	layout, command := layout.updatePaneAction(ActionDetailUpdate)
	if layout.pane.dllsResource.confirmation != nil || !layout.pane.dllsResource.busy || command == nil {
		t.Fatal("catalog update-all still asked first")
	}

	batch := layoutWithSettings(t, off, entry)
	batch.showBatchMenu, batch.batchGames = true, []*game.Game{entry}
	batch.pane.dllsResource.manifest = &dll.Manifest{DLLs: map[string][]dll.DLL{"dlss": {{Version: "3.9.0"}}}}
	batch, command, _ = batch.handleBatchMenuAction(ActionOverlayConfirm)
	if batch.batchConfirmation != nil || !batch.batchBusy || command == nil {
		t.Fatal("selected-game batch update still asked first")
	}

	on := layoutWithSettings(t, func(*config.Config) {}, entry)
	on.showBatchMenu, on.batchGames = true, []*game.Game{entry}
	on.pane.dllsResource.manifest = batch.pane.dllsResource.manifest
	on, command, _ = on.handleBatchMenuAction(ActionOverlayConfirm)
	if on.batchConfirmation == nil || on.batchBusy || command != nil {
		t.Fatal("with the setting on, the batch update must ask first")
	}
}

func TestCompactModeSetsTheLayoutAtStartupAndWhenChanged(t *testing.T) {
	layout := layoutWithSettings(t, func(configuration *config.Config) { configuration.CompactMode = true })
	if layout.densityMode != DensityCompact {
		t.Fatalf("startup layout = %v, want compact", layout.densityMode)
	}
	if standard := layoutWithSettings(t, func(*config.Config) {}); standard.densityMode != DensityStandard {
		t.Fatalf("startup layout without Compact mode = %v", standard.densityMode)
	}

	save := func(change func(*config.Config)) {
		previous := layout.config
		next := previous.Clone()
		change(next)
		layout.config = next
		layout.applyDisplaySettings(previous)
	}
	layout.densityMode = DensityFocused
	save(func(configuration *config.Config) { configuration.LogLevel = "debug" })
	if layout.densityMode != DensityFocused {
		t.Fatal("an unrelated save replaced the layout picked from Actions")
	}
	save(func(configuration *config.Config) { configuration.CompactMode = false })
	if layout.densityMode != DensityStandard {
		t.Fatalf("turning Compact mode off = %v", layout.densityMode)
	}
	save(func(configuration *config.Config) { configuration.CompactMode = true })
	if layout.densityMode != DensityCompact {
		t.Fatalf("turning Compact mode on = %v", layout.densityMode)
	}
	save(func(configuration *config.Config) { configuration.ConfirmDestructive = false })
	if !layout.pane.content.skipUpdateConfirmation {
		t.Fatal("a saved Confirm destructive actions change did not reach the views")
	}
}
