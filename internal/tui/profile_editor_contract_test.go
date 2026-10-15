package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func TestProfileEditorCreatesExactOverrideFromInheritedValue(t *testing.T) {
	styles := NewStyles(DefaultTheme)
	defaults := &profile.Profile{Proton: profile.ProtonSettings{EnableHDR: true}}
	detail := NewDetail(styles, nil, defaults)
	focusField(t, &detail, profile.FieldProtonEnableHDR)

	if !detail.BeginEdit() || !pickOption(&detail, "o") {
		t.Fatal("inherited field did not enter and commit through EditorHost")
	}
	if !detail.Dirty() || !detail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) || !detail.RawProfile().Proton.EnableHDR {
		t.Fatalf("concrete inherited edit did not create exact override: %+v", detail.RawProfile())
	}
}

func TestProfileEditorKeepsInvalidInputAndReportsFailedWrites(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, &profile.Profile{})
	focusField(t, &detail, profile.FieldGPUPowerLimit)
	detail.BeginEdit()
	detail.editor.Set("not-a-number")
	detail.UpdateEditor(keyMsg("enter"))
	if !detail.Editing() || detail.editor.Error() == nil || detail.Dirty() {
		t.Fatal("invalid value did not remain recoverable without mutating the draft")
	}
	detail.editor.Set("275")
	detail.UpdateEditor(keyMsg("enter"))
	if detail.Editing() || !detail.Dirty() || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("corrected integer did not apply to the draft")
	}

	detail.CompleteSave(errors.New("disk full"))
	if !detail.Dirty() || detail.SaveError() == nil || detail.RawProfile().GPU.PowerLimit != 275 {
		t.Fatal("persistence failure did not retain draft and error")
	}
	detail.CompleteSave(nil)
	if detail.Dirty() || detail.SaveError() != nil {
		t.Fatal("successful persistence did not advance the visible baseline")
	}
}

func TestProfileAndSettingsUseSharedEditorHost(t *testing.T) {
	profileDetail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, &profile.Profile{})
	if !profileDetail.BeginEdit() || !profileDetail.editor.Active() {
		t.Fatal("profile did not activate EditorHost")
	}

	settings := NewOptionsModal(NewStyles(DefaultTheme))
	settings.OpenEmbedded(config.Default())
	settings.SyncNavSection(nav.SettingsPaths)
	settings.startPathEditing()
	if !settings.editor.Active() || !settings.editingPath {
		t.Fatal("settings path did not activate EditorHost")
	}
	settings, _ = settings.UpdateAction(ActionEditCancel)
	if settings.editor.Active() || settings.editingPath {
		t.Fatal("settings cancellation did not close EditorHost")
	}

	root := NewRootDetail(NewStyles(DefaultTheme), &profile.Profile{})
	if !strings.Contains(stripANSI(root.View()), "system default") {
		t.Fatal("root reset semantics are not labelled system default")
	}
}
