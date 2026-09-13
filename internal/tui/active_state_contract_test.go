package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

func TestActiveLayoutDestinationsRenderAndPreserveNavigationState(t *testing.T) {
	gameEntry := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	model := testLayoutWithGame(gameEntry)
	withDLLSection := func(section nav.DLLCatalogSection) nav.State {
		state := model.navState.SelectDestination(nav.DestinationDLLCatalog)
		state.DLLCatalogSection = section
		return state
	}
	withMonitorSection := func(section nav.MonitorSection) nav.State {
		state := model.navState.SelectDestination(nav.DestinationMonitor)
		state.MonitorSection = section
		return state
	}
	withSettingsSection := func(section nav.SettingsSection) nav.State {
		state := model.navState.SelectDestination(nav.DestinationSettings)
		state.SettingsSection = section
		return state
	}
	states := []struct {
		name    string
		state   nav.State
		content string
	}{
		{"game overview", model.navState.SelectAspect(nav.AspectOverview), "Cyberpunk 2077"},
		{"game profile", model.navState.SelectAspect(nav.AspectProfile), "HDR"},
		{"game DLLs", model.navState.SelectAspect(nav.AspectDLLs), "DLL versions"},
		{"DLL library", withDLLSection(nav.SectionDLLLibrary), "Inventory of DLL types"},
		{"DLL deployment", withDLLSection(nav.SectionDLLDeployment), "Deployment"},
		{"monitor GPU", withMonitorSection(nav.MonitorGPU), "GPU"},
		{"monitor CPU", withMonitorSection(nav.MonitorCPU), "CPU"},
		{"monitor alerts", withMonitorSection(nav.MonitorAlerts), "Alerts"},
		{"settings display", withSettingsSection(nav.SettingsDisplay), "Show hints"},
	}
	for _, test := range states {
		t.Run(test.name, func(t *testing.T) {
			*model.navState = test.state
			model.syncNavToComponents()
			for _, density := range []DensityMode{DensityStandard, DensityCompact, DensityFocused} {
				model.densityMode = density
				model.calculateDimensions()
				visible := stripANSI(model.View().Content)
				if !strings.Contains(visible, test.content) {
					t.Fatalf("density %d missing active content %q:\n%s", density, test.content, visible)
				}
				if model.pane.State().Destination != test.state.Destination {
					t.Fatalf("render changed destination from %v to %v", test.state.Destination, model.pane.State().Destination)
				}
			}
		})
	}
}

func TestOptionsModalEditsEveryCatalogOptionThroughKeyboardContract(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", state)
	modal := NewOptionsModal(NewStyles(DefaultTheme, true))
	modal.OpenEmbedded(config.Default())
	modal.SetSize(100, 40)

	for sectionIndex, section := range modal.sections {
		modal.sectionCursor = sectionIndex
		for optionIndex, option := range section.Options {
			modal.optionCursor = optionIndex
			before := modal.getConfigValue(option.Key)
			switch option.Kind {
			case config.KindPath:
				modal, _ = modal.Update(keyMsg("enter"))
				if !modal.Editing() || modal.getConfigValue(option.Key) != before {
					t.Fatalf("%s did not open an isolated field editor", option.Key)
				}
				modal.pathInput.SetValue("/contract/" + option.Key)
				modal, _ = modal.Update(keyMsg("enter"))
			case config.KindBool, config.KindEnum:
				modal, _ = modal.Update(keyMsg("enter"))
				if modal.Editing() || modal.getConfigValue(option.Key) != before {
					t.Fatalf("Enter changed cyclic setting %s or opened an editor", option.Key)
				}
				next, command := modal.Update(keyMsg("space"))
				modal = next
				if command != nil || modal.Editing() || option.Get(modal.config) != before {
					t.Fatalf("Space saved %s or opened an editor", option.Key)
				}
			default:
				t.Fatalf("add keyboard coverage for newly exposed option kind %v", option.Kind)
			}
			after := modal.getConfigValue(option.Key)
			if before == after {
				t.Errorf("keyboard edit did not change %s from %q", option.Key, before)
			}
			if view := stripANSI(modal.renderOptionsBody()); !strings.Contains(view, option.Label) {
				t.Errorf("option view missing active label %q:\n%s", option.Label, view)
			}
		}
	}
	if !modal.modified {
		t.Fatal("catalog edits did not mark settings modified")
	}
	next, command := modal.Update(keyMsg("ctrl+s"))
	modal = next
	if command == nil {
		t.Fatal("save key did not return a persistence command")
	}
	if message, ok := command().(optionsSavedMsg); !ok {
		t.Fatalf("save command returned %#v", message)
	}
}
