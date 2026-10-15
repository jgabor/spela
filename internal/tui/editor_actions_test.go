package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func TestProfileEditorApplyValidatesBeforeRequestingPersistence(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme), &profile.Profile{})
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("invalid")
	if changed, handled := detail.UpdateAction(ActionEditCommit); changed || !handled || !detail.Editing() || detail.Dirty() {
		t.Fatal("invalid Apply escaped its editor or changed the profile")
	}
	if !detail.EditorInputFocused() || detail.editor.Error() == nil {
		t.Fatal("invalid Apply did not return a recoverable input with an error")
	}
	detail.editor.Set("275")
	if changed, handled := detail.UpdateAction(ActionEditCommit); !changed || !handled || detail.Editing() || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("valid Apply did not set its field and request a save")
	}
	if got := detail.LastChange(); got != "Power limit 275 W (All games)" {
		t.Fatalf("change summary = %q", got)
	}
	if !detail.Dirty() {
		t.Fatal("the change counted as saved before the write finished")
	}
}

func TestProfileEditorTabControlsApplyAndCancelAndWrap(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme), &profile.Profile{})
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("275")
	detail.UpdateAction(ActionFocusNext)
	if detail.EditorEnterLabel() != "apply" || detail.EditorInputFocused() {
		t.Fatal("first Tab did not focus Apply")
	}
	if changed, _ := detail.UpdateAction(ActionEditCommit); !changed || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("Apply did not set the value and request a save")
	}

	focusField(t, &detail, profile.FieldGPUShaderCachePath)
	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("/cancelled")
	detail.UpdateAction(ActionFocusNext)
	detail.UpdateAction(ActionFocusNext)
	if detail.EditorEnterLabel() != "cancel edit" {
		t.Fatal("second Tab did not focus Cancel")
	}
	if changed, _ := detail.UpdateAction(ActionEditCommit); changed || detail.Editing() || detail.RawProfile().GPU.ShaderCachePath != "" || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("Cancel applied the active edit or lost the earlier change")
	}

	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("/escaped")
	if changed, handled := detail.UpdateAction(ActionEditCancel); changed || !handled || detail.Editing() || detail.RawProfile().GPU.ShaderCachePath != "" {
		t.Fatal("Esc did not cancel the edit")
	}

	detail.UpdateAction(ActionDetailConfirm)
	for range 3 {
		detail.UpdateAction(ActionFocusNext)
	}
	if !detail.EditorInputFocused() {
		t.Fatal("Tab did not wrap to Input after three controls")
	}
}

func TestProfileEarlierSaveKeepsLaterChangeAndFailedSaveReturnsToStoredValue(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme), &profile.Profile{})
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.BeginEdit()
	detail.editor.Set("275")
	detail.UpdateAction(ActionEditCommit)
	saved := detail.RawProfile().Clone()
	detail.BeginEdit()
	detail.editor.Set("300")
	detail.UpdateAction(ActionEditCommit)
	detail.CompleteSaveSnapshot(saved, nil)
	if !detail.Dirty() || detail.RawProfile().GPU.PowerLimit != 300 || detail.persisted.GPU.PowerLimit != 275 {
		t.Fatal("earlier completion replaced or marked the later change saved")
	}
	detail.CompleteSaveSnapshot(detail.RawProfile(), errors.New("read-only"))
	if !detail.Dirty() || detail.SaveError() == nil || detail.persisted.GPU.PowerLimit != 275 {
		t.Fatal("failed save advanced the baseline or lost the error")
	}
	detail.RevertToPersisted()
	if detail.RawProfile().GPU.PowerLimit != 275 || detail.Dirty() || detail.SaveError() == nil {
		t.Fatal("a failed save did not return the row to the stored value with its error")
	}
	if status := detail.SaveStatus(); !strings.HasPrefix(status, "Not saved: read-only") {
		t.Fatalf("status after a failed save = %q", status)
	}
}

func TestProfileResetRemovesFalseDirtinessButPreservesExplicitPins(t *testing.T) {
	for _, root := range []bool{false, true} {
		t.Run(map[bool]string{true: "root", false: "game"}[root], func(t *testing.T) {
			styles := NewStyles(DefaultTheme)
			detail := NewDetail(styles, &profile.Profile{}, &profile.Profile{})
			if root {
				detail = NewRootDetail(styles, &profile.Profile{})
			}
			focusField(t, &detail, profile.FieldProtonEnableHDR)
			detail.UpdateAction(ActionDetailConfirm)
			if !detail.Editing() || detail.editor.Kind() != EditorChoice || detail.Dirty() {
				t.Fatal("Enter did not open a picker without changing the field")
			}
			detail.UpdateAction(ActionEditCancel)
			detail.UpdateAction(ActionDetailCycle)
			if !detail.Dirty() {
				t.Fatal("changed HDR did not mark the profile as not yet saved")
			}
			detail.UpdateAction(ActionDetailReset)
			if detail.Dirty() || detail.RawProfile().Proton.EnableHDR {
				t.Fatal("reset back to the saved value retained nil-versus-empty map dirtiness")
			}
			// Picking Off must still create a concrete pin, even though its value
			// matches the currently inherited/system value.
			detail.UpdateAction(ActionDetailConfirm)
			if !pickOption(&detail, "of") {
				t.Fatal("picking Off did not change the profile")
			}
			if !detail.Dirty() || !detail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) || detail.RawProfile().Proton.EnableHDR {
				t.Fatal("explicit false pin was mistaken for an unchanged inherited value")
			}
			detail.UpdateAction(ActionDetailResetAll)
			if detail.Dirty() {
				t.Fatal("reset-all did not return to the original baseline")
			}
		})
	}
}

func TestSettingsEditorApplySavesExactPathAndKeepsEarlierChanges(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme))
	settings.OpenEmbedded(config.Default())
	var persisted *config.Config
	settings.SetSaveConfig(func(configuration *config.Config) error {
		persisted = configuration.Clone()
		return nil
	})
	settings.SyncNavSection(nav.SettingsStartup)
	settings, command := settings.UpdateAction(ActionDetailCycle)
	if command == nil || !settings.Saving() {
		t.Fatal("Space did not save the option at once")
	}
	saved, ok := command().(optionsSavedMsg)
	if !ok || persisted.RescanOnStartup || saved.summary != "Re-scan on startup Off" {
		t.Fatalf("first save = %#v (%#v)", persisted, saved)
	}
	if next := settings.CompleteSave(saved.config); next != nil || settings.Saving() || settings.Dirty() {
		t.Fatal("a finished write left Settings busy or dirty")
	}

	settings.SyncNavSection(nav.SettingsPaths)
	settings, _ = settings.UpdateAction(ActionDetailConfirm)
	path := "  /Spel bibliotek/åäö #1/[0]  "
	settings.pathInput.SetValue(path)
	settings, command = settings.UpdateAction(ActionEditCommit)
	if command == nil || !settings.Saving() || settings.Editing() {
		t.Fatal("Apply did not save the path at once")
	}
	message, ok := command().(optionsSavedMsg)
	if !ok || persisted.SteamPath != path || persisted.RescanOnStartup {
		t.Fatalf("saved configuration = %#v, result = %#v", persisted, message)
	}
	settings.CompleteSave(message.config)
	if settings.Saving() || settings.Dirty() || settings.config.SteamPath != path {
		t.Fatal("successful save did not commit Settings coherently")
	}
}

func TestSettingsChangeDuringAWriteIsWrittenByTheNextOne(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme))
	settings.OpenEmbedded(config.Default())
	var writes []*config.Config
	settings.SetSaveConfig(func(configuration *config.Config) error {
		writes = append(writes, configuration.Clone())
		return nil
	})
	settings.SyncNavSection(nav.SettingsStartup)
	settings, first := settings.UpdateAction(ActionDetailCycle)
	settings, second := settings.UpdateAction(ActionDetailCycle)
	if first == nil || second != nil || !settings.Saving() {
		t.Fatal("a change during a write started a second write or was refused")
	}
	message := first().(optionsSavedMsg)
	next := settings.CompleteSave(message.config)
	if next == nil || !settings.Saving() || !settings.Dirty() {
		t.Fatal("the change made during the write was not scheduled")
	}
	last := next().(optionsSavedMsg)
	settings.CompleteSave(last.config)
	if len(writes) != 2 || writes[0].RescanOnStartup || !writes[1].RescanOnStartup || settings.Dirty() {
		t.Fatalf("writes = %#v", writes)
	}
}

func TestSettingsFailedWriteReturnsToTheStoredValue(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme))
	settings.OpenEmbedded(config.Default())
	settings.SetSaveConfig(func(*config.Config) error { return errors.New("read-only") })
	settings.SyncNavSection(nav.SettingsStartup)
	settings, command := settings.UpdateAction(ActionDetailCycle)
	failure, ok := command().(optionsSaveErrorMsg)
	if !ok {
		t.Fatalf("write result = %T", command())
	}
	settings.FailSave(failure.err)
	if settings.Saving() || settings.Dirty() || !settings.draft.RescanOnStartup || settings.SaveError() == nil {
		t.Fatal("a failed write kept the unsaved value or lost the error")
	}
	if view := stripANSI(settings.DetailView()); !strings.Contains(view, "Not saved: read-only") {
		t.Fatalf("Settings view hides the failure:\n%s", view)
	}
}

func TestSettingsEditorCancelPreservesEarlierDraftAndIgnoresButtonsText(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme))
	settings.OpenEmbedded(config.Default())
	settings.SyncNavSection(nav.SettingsStartup)
	settings.cycleValue(1)
	settings.SyncNavSection(nav.SettingsPaths)
	settings, _ = settings.UpdateAction(ActionDetailConfirm)
	settings.pathInput.SetValue("/draft")
	settings, _ = settings.UpdateAction(ActionFocusNext)
	settings, _ = settings.UpdateAction(ActionFocusNext)
	settings, _ = settings.UpdateEditorInput(keyMsg("4"))
	if settings.pathInput.Value() != "/draft" || settings.EditorInputFocused() {
		t.Fatal("button focus allowed printable input into the path")
	}
	settings, command := settings.UpdateAction(ActionEditCommit)
	if command != nil || settings.Editing() || settings.draft.SteamPath != "" || settings.draft.RescanOnStartup || !settings.Dirty() {
		t.Fatal("Cancel applied its path or discarded the previous boolean draft")
	}
}

func TestSettingsBrowseArrowsEnterAndRemovedShortcutsCannotChangeValues(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme))
	settings.OpenEmbedded(config.Default())
	for _, key := range []string{"left", "right", "enter", "h", "l", "s", "esc"} {
		next, command := settings.Update(keyMsg(key))
		settings = next
		if command != nil || settings.Editing() || settings.Dirty() || !settings.draft.RescanOnStartup {
			t.Fatalf("Browse key %q changed Settings", key)
		}
	}
}

func TestEditorsKeepBasicControlsVisibleWithoutVerboseHints(t *testing.T) {
	styles := NewStyles(DefaultTheme)
	detail := NewRootDetail(styles, &profile.Profile{})
	detail.SetSize(68, 9)
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.UpdateAction(ActionDetailConfirm)
	settings := NewOptionsModal(styles)
	settings.OpenEmbedded(config.Default())
	settings.SetSize(68, 9)
	settings.SyncNavSection(nav.SettingsPaths)
	settings, _ = settings.UpdateAction(ActionDetailConfirm)
	for name, view := range map[string]string{"profile": detail.View(), "settings": settings.DetailView()} {
		view = stripANSI(view)
		for _, control := range []string{"Input", "Apply", "Cancel", "Tab control", "Esc cancel"} {
			if !strings.Contains(view, control) {
				t.Fatalf("%s editor missing %q:\n%s", name, control, view)
			}
		}
		for _, removed := range []string{"Save", "Ctrl+S", "s:save", "r System", "Shift+R"} {
			if strings.Contains(view, removed) {
				t.Fatalf("%s editor retains removed control %q", name, removed)
			}
		}
	}
}

func TestEditorHostTextUsesRuneCursorAndProtectsLocalButtonFocus(t *testing.T) {
	var editor EditorHost
	editor.Begin(EditorSpec{Kind: EditorText}, "å0")
	editor.UpdateInput(keyMsg("left"))
	editor.UpdateInput(keyMsg("/"))
	editor.Toggle()
	if editor.Value() != "å/ 0" {
		t.Fatalf("text insertion at rune cursor = %q", editor.Value())
	}
	editor.UpdateInput(keyMsg("backspace"))
	editor.UpdateInput(keyMsg("delete"))
	if editor.Value() != "å/" {
		t.Fatalf("text deletion at rune cursor = %q", editor.Value())
	}
	editor.FocusNext()
	if editor.UpdateInput(keyMsg("4")) || editor.Value() != "å/" {
		t.Fatal("button focus accepted text")
	}
}
