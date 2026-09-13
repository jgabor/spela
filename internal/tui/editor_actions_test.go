package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func TestProfileEditorSaveValidatesBeforeRequestingPersistence(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme, true), &profile.Profile{})
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("invalid")
	if save, handled := detail.UpdateAction(ActionEditSave); save || !handled || !detail.Editing() || detail.Dirty() {
		t.Fatal("invalid Save escaped its editor or changed the document")
	}
	if !detail.EditorInputFocused() || detail.editor.Error() == nil {
		t.Fatal("invalid Save did not return a recoverable input with an error")
	}
	detail.editor.Set("275")
	if save, handled := detail.UpdateAction(ActionEditSave); !save || !handled || detail.Editing() || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("valid Save did not apply its field and request persistence")
	}
	if !detail.Dirty() {
		t.Fatal("save request marked the document saved before persistence completed")
	}
}

func TestProfileEditorTabControlsApplyCancelSaveAndWrap(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme, false), &profile.Profile{})
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("275")
	detail.UpdateAction(ActionFocusNext)
	if detail.EditorEnterLabel() != "apply to draft" || detail.EditorInputFocused() {
		t.Fatal("first Tab did not focus Apply")
	}
	if save, _ := detail.UpdateAction(ActionEditCommit); save || detail.RawProfile().GPU.PowerLimit != 275 || !detail.Dirty() {
		t.Fatal("Apply did not retain an unsaved draft")
	}

	focusField(t, &detail, profile.FieldGPUShaderCachePath)
	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("/cancelled")
	detail.UpdateAction(ActionFocusNext)
	detail.UpdateAction(ActionFocusNext)
	if detail.EditorEnterLabel() != "cancel edit" {
		t.Fatal("second Tab did not focus Cancel")
	}
	detail.UpdateAction(ActionEditCommit)
	if detail.Editing() || detail.RawProfile().GPU.ShaderCachePath != "" || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("Cancel discarded an earlier field draft or applied the active edit")
	}

	detail.UpdateAction(ActionDetailConfirm)
	detail.editor.Set("/saved")
	for range 3 {
		detail.UpdateAction(ActionFocusNext)
	}
	if detail.EditorEnterLabel() != "save" {
		t.Fatal("third Tab did not focus Save")
	}
	if save, _ := detail.UpdateAction(ActionEditCommit); !save || detail.RawProfile().GPU.ShaderCachePath != "/saved" || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("selectable Save did not request the same persistence as Ctrl+S")
	}
	detail.UpdateAction(ActionDetailConfirm)
	for range 4 {
		detail.UpdateAction(ActionFocusNext)
	}
	if !detail.EditorInputFocused() {
		t.Fatal("Tab did not wrap to Input")
	}
}

func TestProfileEarlierSaveKeepsLaterDraftAndFailedSaveKeepsBaseline(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme, true), &profile.Profile{})
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.BeginEdit()
	detail.editor.Set("275")
	detail.UpdateAction(ActionEditSave)
	saved := detail.RawProfile().Clone()
	detail.BeginEdit()
	detail.editor.Set("300")
	detail.UpdateAction(ActionEditCommit)
	detail.CompleteSaveSnapshot(saved, nil)
	if !detail.Dirty() || detail.RawProfile().GPU.PowerLimit != 300 || detail.persisted.GPU.PowerLimit != 275 {
		t.Fatal("earlier completion replaced or marked the later draft saved")
	}
	detail.CompleteSaveSnapshot(detail.RawProfile(), errors.New("read-only"))
	if !detail.Dirty() || detail.SaveError() == nil || detail.persisted.GPU.PowerLimit != 275 {
		t.Fatal("failed save advanced the baseline or lost the error")
	}
	detail.CancelDraft()
	if detail.RawProfile().GPU.PowerLimit != 275 || detail.Dirty() {
		t.Fatal("discard did not return to the latest successful snapshot")
	}
}

func TestProfileResetRemovesFalseDirtinessButPreservesExplicitPins(t *testing.T) {
	for _, root := range []bool{false, true} {
		t.Run(map[bool]string{true: "root", false: "game"}[root], func(t *testing.T) {
			styles := NewStyles(DefaultTheme, true)
			detail := NewDetail(styles, &profile.Profile{}, &profile.Profile{})
			if root {
				detail = NewRootDetail(styles, &profile.Profile{})
			}
			focusField(t, &detail, profile.FieldProtonEnableHDR)
			detail.UpdateAction(ActionDetailConfirm)
			if detail.Editing() || detail.Dirty() {
				t.Fatal("Enter opened an editor or changed a cyclic field")
			}
			detail.UpdateAction(ActionDetailCycle)
			if !detail.Dirty() {
				t.Fatal("changed HDR did not mark the draft dirty")
			}
			detail.UpdateAction(ActionDetailReset)
			if detail.Dirty() || detail.RawProfile().Proton.EnableHDR {
				t.Fatal("reset back to the saved value retained nil-versus-empty map dirtiness")
			}
			// Cycling to explicit false must still create a concrete pin, even
			// though its value matches the currently inherited/system value.
			detail.UpdateAction(ActionDetailCycle)
			detail.UpdateAction(ActionDetailCycle)
			if !detail.Dirty() || !detail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) || detail.RawProfile().Proton.EnableHDR {
				t.Fatal("explicit false pin was mistaken for an unchanged inherited value")
			}
			detail.UpdateAction(ActionDetailResetAll)
			if detail.Dirty() {
				t.Fatal("reset-all did not return to the original draft baseline")
			}
		})
	}
}

func TestSettingsEditorSaveWritesExactPathAndAllPriorDraftChanges(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme, false))
	settings.OpenEmbedded(config.Default())
	settings.SyncNavSection(nav.SettingsDisplay)
	settings, _ = settings.UpdateAction(ActionDetailCycle)
	settings.SyncNavSection(nav.SettingsPaths)
	settings, _ = settings.UpdateAction(ActionDetailConfirm)
	path := "  /Spel bibliotek/åäö #1/[0]  "
	settings.pathInput.SetValue(path)
	var persisted *config.Config
	settings.SetSaveConfig(func(configuration *config.Config) error {
		persisted = configuration.Clone()
		return nil
	})
	settings, command := settings.UpdateAction(ActionEditSave)
	if command == nil || !settings.Busy() || settings.Editing() || !settings.Dirty() {
		t.Fatal("Ctrl+S did not submit the owning Settings draft")
	}
	message, ok := command().(optionsSavedMsg)
	if !ok || persisted.SteamPath != path || persisted.ShowHints {
		t.Fatalf("saved configuration = %#v, result = %#v", persisted, message)
	}
	settings.CompleteSave(message.config)
	if settings.Busy() || settings.Dirty() || settings.config.SteamPath != path {
		t.Fatal("successful save did not commit Settings coherently")
	}
}

func TestSettingsEditorCancelPreservesEarlierDraftAndIgnoresButtonsText(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme, true))
	settings.OpenEmbedded(config.Default())
	settings.SyncNavSection(nav.SettingsDisplay)
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
	if command != nil || settings.Editing() || settings.draft.SteamPath != "" || settings.draft.ShowHints || !settings.Dirty() {
		t.Fatal("Cancel applied its path or discarded the previous boolean draft")
	}
}

func TestSettingsBrowseArrowsEnterAndRemovedShortcutsCannotChangeValues(t *testing.T) {
	settings := NewOptionsModal(NewStyles(DefaultTheme, true))
	settings.OpenEmbedded(config.Default())
	for _, key := range []string{"left", "right", "enter", "h", "l", "s", "esc"} {
		next, command := settings.Update(keyMsg(key))
		settings = next
		if command != nil || settings.Editing() || settings.Dirty() || !settings.draft.ShowHints {
			t.Fatalf("Browse key %q changed Settings", key)
		}
	}
}

func TestEditorsKeepBasicControlsVisibleWithoutVerboseHints(t *testing.T) {
	styles := NewStyles(DefaultTheme, false)
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
		for _, control := range []string{"Input", "Apply", "Cancel", "Save", "Tab control", "Ctrl+S save"} {
			if !strings.Contains(view, control) {
				t.Fatalf("%s editor missing %q:\n%s", name, control, view)
			}
		}
		for _, removed := range []string{"Esc", "s:save", "r System", "Shift+R"} {
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
