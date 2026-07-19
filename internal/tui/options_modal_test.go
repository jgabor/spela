package tui

import (
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
)

func TestOptionsModal_EmbeddedShowsSingleSection(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(config.Default())
	modal.SyncNavSection(nav.SettingsLogging)

	view := modal.renderOptionsBody()
	if !strings.Contains(view, "Log level") {
		t.Fatalf("expected logging option in view, got %q", view)
	}
	if strings.Contains(view, "Steam path") {
		t.Fatal("paths section should not render when logging section is active")
	}
}

func TestOptionsModal_ShowHintsTakesEffectBeforeSave(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	modal := NewOptionsModal(styles)
	configuration := config.Default()
	modal.OpenEmbedded(configuration)
	modal.SyncNavSection(nav.SettingsDisplay)
	modal.optionCursor = 1 // Show hints follows Theme in the canonical display section.

	modal.cycleValue(1)

	if configuration.ShowHints || styles.ShowHints {
		t.Fatalf("ShowHints immediate state = config %v, styles %v; want both false", configuration.ShowHints, styles.ShowHints)
	}
}

func TestOptionsModal_CompactWidthKeepsValuesOnOptionRows(t *testing.T) {
	modal := NewOptionsModal(NewStyles(DefaultTheme, false))
	modal.OpenEmbedded(config.Default())
	modal.SetSize(34, 14)

	view := stripANSI(modal.renderOptionsBody())
	for _, row := range []string{"Theme: default", "Show hints: true", "Compact mode: false", "Confirm destr…: true"} {
		if !strings.Contains(view, row) {
			t.Errorf("compact settings missing row %q:\n%s", row, view)
		}
	}
	if strings.Contains(view, "Use the default, dark, or light theme.") {
		t.Fatal("compact settings should omit the long description")
	}
}
