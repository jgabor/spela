package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func TestProfileSpaceTogglesBooleansBetweenFollowingAndTheOppositeValue(t *testing.T) {
	for _, root := range []bool{true, false} {
		for _, test := range []struct {
			field     string
			inherited bool
		}{
			{profile.FieldProtonEnableHDR, true},
			{profile.FieldProtonVKD3DHeap, false},
		} {
			name := "game/"
			if root {
				name = "root/"
			}
			t.Run(name+test.field, func(t *testing.T) {
				defaults := &profile.Profile{}
				if err := defaults.Set(test.field, test.inherited); err != nil {
					t.Fatal(err)
				}
				detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, defaults)
				followed := test.inherited
				if root {
					detail = NewRootDetail(detail.styles, &profile.Profile{})
					followed = false
				}
				focusField(t, &detail, test.field)
				cursor := detail.Cursor()
				if !root && !strings.Contains(detail.View(), "from All games") {
					t.Fatal("inherited choice does not name All games as its source")
				}
				for round := 0; round < 2; round++ {
					changed, handled := detail.UpdateAction(ActionDetailCycle)
					actual, _ := profile.ReadField(detail.RawProfile(), test.field)
					if !handled || !changed || actual != !followed || !detail.RawProfile().IsOverridden(test.field) || detail.Editing() || !detail.Dirty() || detail.Cursor() != cursor {
						t.Fatalf("Space from following gave %v (changed %v), want the opposite %v set on the page", actual, changed, !followed)
					}
					if detail.persisted.IsOverridden(test.field) {
						t.Fatal("Space moved the saved baseline before the write finished")
					}
					changed, _ = detail.UpdateAction(ActionDetailCycle)
					if !changed || detail.RawProfile().IsOverridden(test.field) || detail.Dirty() || detail.Editing() {
						t.Fatal("Space from an own value did not return to following")
					}
				}
				if !root {
					actual, _ := profile.ReadField(detail.resolved, test.field)
					if actual != test.inherited {
						t.Fatalf("following resolved to %v, want %v", actual, test.inherited)
					}
				}
			})
		}
	}
}

func TestProfileSpaceOnOtherFieldsPicksOnceThenPutsTheValueBack(t *testing.T) {
	for _, test := range []struct {
		field     string
		inherited any
		typed     string
		want      any
	}{
		{profile.FieldGPUVRR, "always", "n", "never"},
		{profile.FieldDLSSMultiFrame, 3, "2", 2},
		{profile.FieldDLSSRRMode, "quality", "d", "dlaa"},
		{profile.FieldCPUSMT, true, "o", true},
	} {
		t.Run(test.field, func(t *testing.T) {
			defaults := &profile.Profile{}
			if err := defaults.Set(test.field, test.inherited); err != nil {
				t.Fatal(err)
			}
			detail := NewDetail(NewStyles(DefaultTheme), &profile.Profile{}, defaults)
			focusField(t, &detail, test.field)
			if changed, handled := detail.UpdateAction(ActionDetailCycle); changed || !handled || !detail.Editing() {
				t.Fatal("Space on a following field with no earlier value did not open the picker")
			}
			detail.editor.TypeAhead(test.typed)
			if changed, _ := detail.UpdateAction(ActionEditCommit); !changed || detail.Editing() {
				t.Fatal("applying the picked value did not change the profile")
			}
			if actual, _ := profile.ReadField(detail.RawProfile(), test.field); !reflect.DeepEqual(actual, test.want) || !detail.RawProfile().IsOverridden(test.field) {
				t.Fatalf("picked value = %#v, want %#v", actual, test.want)
			}
			if changed, _ := detail.UpdateAction(ActionDetailCycle); !changed || detail.RawProfile().IsOverridden(test.field) {
				t.Fatal("Space did not return the field to following")
			}
			if changed, _ := detail.UpdateAction(ActionDetailCycle); !changed || detail.Editing() {
				t.Fatal("Space did not put the earlier value back")
			}
			if actual, _ := profile.ReadField(detail.RawProfile(), test.field); !reflect.DeepEqual(actual, test.want) || !detail.RawProfile().IsOverridden(test.field) {
				t.Fatalf("restored value = %#v, want %#v", actual, test.want)
			}
		})
	}
}

func TestProfileSpaceAfterWriteFailureShowsTheStoredValueAndClearsTheStaleError(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme), &profile.Profile{})
	focusField(t, &detail, profile.FieldProtonEnableHDR)
	detail.UpdateAction(ActionDetailCycle)
	detail.CompleteSaveSnapshot(detail.RawProfile(), errors.New("read-only"))
	detail.RevertToPersisted()
	if detail.RawProfile().Proton.EnableHDR || detail.Dirty() || detail.SaveError() == nil {
		t.Fatal("a failed write did not return the row to the stored value with its error")
	}
	if changed, _ := detail.UpdateAction(ActionDetailCycle); !changed || !detail.RawProfile().Proton.EnableHDR || detail.SaveError() != nil || !detail.Dirty() {
		t.Fatal("Space after the failure did not repeat the change or kept a stale error")
	}
}

func TestSpaceAndEnterBindingsFollowFocusAndFieldState(t *testing.T) {
	layout := testLayout()
	layout.pane.loadGlobalScope()
	focusField(t, &layout.pane.defaultsDetail, profile.FieldProtonEnableHDR)
	layout.focus = FocusList
	if hasHelpAction(CanonicalKeymap.HelpBindings(layout.bindingContext()), ActionDetailCycle) {
		t.Fatal("inactive Detail advertised Space for the field")
	}
	layout.focus = FocusDetail
	context := layout.bindingContext()
	space := CanonicalKeymap.Lookup(context, "space")
	if !space.Available || space.Binding.Description != profile.ActionSetHere {
		t.Fatalf("Space on a field at the system default = %q (available %v), want %q", space.Binding.Description, space.Available, profile.ActionSetHere)
	}
	if enter := CanonicalKeymap.Lookup(context, "enter"); !enter.Available || enter.Binding.Description != "Edit value" {
		t.Fatalf("Enter on a boolean = %q (available %v)", enter.Binding.Description, enter.Available)
	}
	updated, _ := sendKey(&layout, "space")
	layout = updated.(LayoutModel)
	if !layout.pane.defaultsDetail.RawProfile().Proton.EnableHDR {
		t.Fatal("Space did not set the value")
	}
	if space := CanonicalKeymap.Lookup(layout.bindingContext(), "space"); space.Binding.Description != profile.ActionUseSystemDefault {
		t.Fatalf("Space on an own value = %q, want %q", space.Binding.Description, profile.ActionUseSystemDefault)
	}

	updated, _ = sendKey(&layout, "enter")
	layout = updated.(LayoutModel)
	if !layout.pane.defaultsDetail.Editing() || layout.pane.defaultsDetail.editor.Kind() != EditorChoice {
		t.Fatal("Enter on a boolean did not open the picker")
	}
	updated, _ = sendKey(&layout, "esc")
	layout = updated.(LayoutModel)
	if layout.pane.defaultsDetail.Editing() {
		t.Fatal("Esc did not close the picker")
	}

	for _, field := range []string{profile.FieldGPUPowerLimit, profile.FieldGPUShaderCachePath} {
		focusField(t, &layout.pane.defaultsDetail, field)
		updated, _ = sendKey(&layout, "space")
		layout = updated.(LayoutModel)
		detail := layout.pane.defaultsDetail
		if !detail.Editing() || detail.editor.Kind() == EditorChoice || detail.RawProfile().IsOverridden(field) {
			t.Fatalf("Space on %s with no earlier value did not open its editor", field)
		}
		updated, _ = sendKey(&layout, "esc")
		layout = updated.(LayoutModel)
	}
}

func TestSettingsSpaceCyclesWithoutOpeningEditor(t *testing.T) {
	for _, key := range []string{"rescan_on_startup", "rescan_on_startup"} {
		t.Run(key, func(t *testing.T) {
			settings := NewOptionsModal(NewStyles(DefaultTheme))
			settings.OpenEmbedded(config.Default())
			option := *config.OptionByKey(key)
			settings.sections = []config.Section{{Options: []config.Option{option}}}
			settings.sectionCursor, settings.optionCursor = 0, 0
			original := option.Get(settings.config)
			settings, command := settings.UpdateAction(ActionDetailCycle)
			if command == nil || settings.Editing() || !settings.Dirty() || option.Get(settings.config) != original || option.Get(settings.draft) == original {
				t.Fatalf("Space did not save the new value or moved the stored one early: editing=%t dirty=%t saved=%q draft=%q error=%v", settings.Editing(), settings.Dirty(), option.Get(settings.config), option.Get(settings.draft), settings.saveError)
			}
			for index := 1; index < len(option.Choices); index++ {
				settings, _ = settings.UpdateAction(ActionDetailCycle)
			}
			if settings.Dirty() || option.Get(settings.draft) != original {
				t.Fatal("full Settings rotation did not return to its saved value")
			}
		})
	}
}

func TestDLLDirectKeyHintsFollowFocusSelectionAndAvailability(t *testing.T) {
	layout := testLayoutWithGame(testGame("Fixture"))
	layout.navState.Aspect = nav.AspectDLLs
	layout.syncNavToComponents()
	layout.focus = FocusList
	if hasHelpAction(CanonicalKeymap.HelpBindings(layout.bindingContext()), ActionDetailConfirm) || hasHelpAction(CanonicalKeymap.HelpBindings(layout.bindingContext()), ActionDetailNextItem) {
		t.Fatal("List focus advertised Detail action controls")
	}
	layout.focus = FocusDetail
	context := layout.bindingContext()
	resolution := CanonicalKeymap.Lookup(context, "enter")
	if !resolution.Available || resolution.Binding.Description != "Install DLL" || !hasHelpAction(CanonicalKeymap.HelpBindings(context), ActionDetailNextItem) {
		t.Fatal("focused DLL Detail did not advertise direct Install and action selection")
	}
	updated, _ := sendKey(&layout, "down")
	layout = updated.(LayoutModel)
	context = layout.bindingContext()
	if layout.pane.content.DLLSelectedAction() != ActionDetailUpdate || CanonicalKeymap.Lookup(context, "enter").Available || hasHelpAction(CanonicalKeymap.HelpBindings(context), ActionDetailConfirm) {
		t.Fatal("disabled Update kept an active Enter hint")
	}
	updated, command := sendKey(&layout, "enter")
	layout = updated.(LayoutModel)
	if command != nil || layout.pane.content.HasModalOpen() {
		t.Fatal("undisplayed Enter dispatched disabled Update")
	}
}
