package tui

import (
	"errors"
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

func TestOptionsModal_SaveUsesInvocationSnapshot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	configuration := config.Default()
	modal := NewOptionsModal(NewStyles(DefaultTheme, true))
	modal.OpenEmbedded(configuration)
	modal.modified = true

	modal, command := modal.save()
	configuration.ShowHints = false
	if _, ok := command().(optionsSavedMsg); !ok {
		t.Fatal("save command did not report success")
	}
	persisted, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.ShowHints || !modal.saving {
		t.Fatalf("persisted live mutation or lost pending state: show hints=%v saving=%v", persisted.ShowHints, modal.saving)
	}
}

func TestOptionsModal_SavePendingSerializesEditsAndRecovers(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	configuration := config.Default()
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(configuration)
	modal.SyncNavSection(nav.SettingsDisplay)
	modal.optionCursor = 1
	modal.modified = true

	modal, first := modal.save()
	before := configuration.ShowHints
	modal, edit := modal.Update(keyMsg("right"))
	modal, second := modal.Update(keyMsg("s"))
	if first == nil || edit != nil || second != nil || configuration.ShowHints != before {
		t.Fatal("an edit or second save passed an in-flight save")
	}

	layout := LayoutModel{config: configuration, pane: resourcePaneModel{settings: modal}, messageBar: NewMessageBar(styles)}
	layout, _ = layout.handleAppMessages(optionsSaveErrorMsg{err: errors.New("read-only")}, nil)
	if layout.pane.settings.saving || !layout.pane.settings.modified {
		t.Fatal("failed save was not left retryable")
	}
	layout.pane.settings, second = layout.pane.settings.Update(keyMsg("s"))
	if second == nil || !layout.pane.settings.saving {
		t.Fatal("failed save could not be retried")
	}
	layout, _ = layout.handleAppMessages(optionsSavedMsg{}, nil)
	if layout.pane.settings.saving || layout.pane.settings.modified || layout.config != layout.pane.settings.config {
		t.Fatal("successful save did not reset state with coherent config ownership")
	}
	layout.pane.settings, _ = layout.pane.settings.Update(keyMsg("right"))
	if configuration.ShowHints == before {
		t.Fatal("normal input remained blocked after save completion")
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
