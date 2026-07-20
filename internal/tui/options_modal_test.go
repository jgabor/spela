package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
)

func TestSettingsDraftIsolationCancelAndSuccessfulCommit(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	live := config.Default()
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(live)
	modal.SyncNavSection(nav.SettingsDisplay)
	modal.optionCursor = 1
	modal.cycleValue(1)
	if !live.ShowHints || !modal.config.ShowHints || modal.draft.ShowHints || !styles.ShowHints {
		t.Fatal("Settings edit escaped the draft boundary")
	}

	modal.CancelDraft()
	if !modal.draft.ShowHints || modal.modified {
		t.Fatal("Settings cancel did not restore the saved value")
	}

	modal.cycleValue(1)
	modal.SetSaveConfig(func(*config.Config) error { return nil })
	modal, command := modal.save()
	message, ok := command().(optionsSavedMsg)
	if !ok || message.config.ShowHints {
		t.Fatalf("save message = %#v", message)
	}
	layout := LayoutModel{styles: styles, config: live, pane: resourcePaneModel{settings: modal}, messageBar: NewMessageBar(styles)}
	layout, _ = layout.handleAppMessages(message, nil)
	if layout.config.ShowHints || layout.pane.settings.draft.ShowHints || styles.ShowHints {
		t.Fatal("successful Settings save was not committed and kept visible")
	}
}

func TestSettingsSaveFailureRetainsDraftFocusAndError(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	live := config.Default()
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(live)
	modal.SyncNavSection(nav.SettingsDisplay)
	modal.optionCursor = 1
	modal.cycleValue(1)
	modal.SetSaveConfig(func(*config.Config) error { return errors.New("read-only") })
	wantSection, wantOption := modal.sectionCursor, modal.optionCursor
	modal, command := modal.save()
	message := command().(optionsSaveErrorMsg)
	layout := LayoutModel{styles: styles, config: live, pane: resourcePaneModel{settings: modal}, messageBar: NewMessageBar(styles)}
	layout, _ = layout.handleAppMessages(message, nil)
	settings := layout.pane.settings
	if settings.saving || !settings.modified || settings.draft.ShowHints || !settings.config.ShowHints {
		t.Fatal("failed Settings save lost or committed its draft")
	}
	if settings.sectionCursor != wantSection || settings.optionCursor != wantOption || settings.saveError == nil {
		t.Fatal("failed Settings save lost focus or inline error")
	}
	if view := stripANSI(settings.DetailView()); !strings.Contains(view, "Draft retained") || !strings.Contains(view, "read-only") {
		t.Fatalf("failed Settings Detail view:\n%s", view)
	}
}

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
	desired := layout.pane.settings.draft.Clone()
	layout, _ = layout.handleAppMessages(optionsSavedMsg{config: desired}, nil)
	if layout.pane.settings.saving || layout.pane.settings.modified || !configsEqual(layout.config, layout.pane.settings.config) {
		t.Fatal("successful save did not reset state with coherent config ownership")
	}
	committed := layout.config.ShowHints
	layout.pane.settings, _ = layout.pane.settings.Update(keyMsg("right"))
	if layout.config.ShowHints != committed {
		t.Fatal("post-save draft edit mutated committed config")
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

	if !configuration.ShowHints || !styles.ShowHints || modal.draft.ShowHints {
		t.Fatalf("ShowHints draft boundary = live %v, styles %v, draft %v", configuration.ShowHints, styles.ShowHints, modal.draft.ShowHints)
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
