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

func TestProfileSpaceCyclesPreserveInheritanceAndExplicitValues(t *testing.T) {
	for _, root := range []bool{true, false} {
		for _, test := range []struct {
			field     string
			inherited any
			values    []any
		}{
			{profile.FieldProtonEnableHDR, true, []any{true, false}},
			{profile.FieldProtonVKD3DHeap, true, []any{true, false}},
			{profile.FieldCPUSMT, true, []any{true, false, nil}},
			{profile.FieldGPUVRR, "always", []any{"unset", "automatic", "always", "never"}},
			{profile.FieldDLSSMultiFrame, 3, []any{0, 1, 2, 3, 4}},
			{profile.FieldDLSSRRMode, "quality", []any{"off", "ultra_performance", "performance", "balanced", "quality", "dlaa", ""}},
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
				detail := NewDetail(NewStyles(DefaultTheme, false), &profile.Profile{}, defaults)
				if root {
					detail = NewRootDetail(detail.styles, &profile.Profile{})
				}
				focusField(t, &detail, test.field)
				cursor := detail.Cursor()
				if !detail.CanCycleFocusedField() {
					t.Fatal("finite field cannot cycle")
				}
				if !root && (!strings.Contains(detail.View(), "(default)") || !strings.Contains(detail.View(), "Default:")) {
					t.Fatal("inherited choice does not show default and its effective value")
				}
				for _, expected := range test.values {
					save, handled := detail.UpdateAction(ActionDetailCycle)
					actual, err := profile.ReadField(detail.RawProfile(), test.field)
					if err != nil || !reflect.DeepEqual(actual, expected) || !detail.RawProfile().IsOverridden(test.field) {
						t.Fatalf("cycle value = %#v, want explicit %#v (error %v)", actual, expected, err)
					}
					if !handled || save || detail.Editing() || !detail.Dirty() || detail.Cursor() != cursor {
						t.Fatal("Space saved, opened an editor, lost focus or failed to retain the draft")
					}
					if detail.persisted.IsOverridden(test.field) {
						t.Fatal("Space changed the saved baseline")
					}
				}
				detail.UpdateAction(ActionDetailCycle)
				if detail.RawProfile().IsOverridden(test.field) || detail.Dirty() || detail.Editing() {
					t.Fatal("full rotation did not return to an unchanged inherited/default draft")
				}
				if !root {
					actual, _ := profile.ReadField(detail.resolved, test.field)
					expected, _ := profile.ReadField(defaults, test.field)
					if !reflect.DeepEqual(actual, expected) {
						t.Fatalf("return to inheritance resolved to %#v, want %#v", actual, expected)
					}
				}
			})
		}
	}
}

func TestProfileSpaceAfterSaveFailureKeepsDraftAndClearsStaleError(t *testing.T) {
	detail := NewRootDetail(NewStyles(DefaultTheme, true), &profile.Profile{})
	focusField(t, &detail, profile.FieldProtonEnableHDR)
	detail.UpdateAction(ActionDetailCycle)
	detail.CompleteSaveSnapshot(detail.RawProfile(), errors.New("read-only"))
	if !detail.Dirty() || detail.SaveError() == nil {
		t.Fatal("failed save did not retain the draft and error")
	}
	detail.UpdateAction(ActionDetailCycle)
	if detail.RawProfile().Proton.EnableHDR || !detail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) || detail.SaveError() != nil || !detail.Dirty() {
		t.Fatal("Space after failure lost the explicit false draft or kept a stale error")
	}
	detail.UpdateAction(ActionCancelDraft)
	if detail.Dirty() || detail.RawProfile().IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatal("discard after cycling did not restore the saved baseline")
	}
}

func TestSpaceFieldBindingMatchesActiveFocusAndFieldKind(t *testing.T) {
	layout := testLayout()
	layout.pane.loadGlobalScope()
	focusField(t, &layout.pane.defaultsDetail, profile.FieldProtonEnableHDR)
	layout.focus = FocusList
	if hasHelpAction(CanonicalKeymap.HelpBindings(layout.bindingContext()), ActionDetailCycle) {
		t.Fatal("inactive Detail advertised Space for cycling")
	}
	updated, _ := sendKey(&layout, "space")
	layout = updated.(LayoutModel)
	if layout.pane.defaultsDetail.Dirty() {
		t.Fatal("List Space changed the profile")
	}
	layout.focus = FocusDetail
	context := layout.bindingContext()
	if !hasHelpAction(CanonicalKeymap.HelpBindings(context), ActionDetailCycle) || CanonicalKeymap.Lookup(context, "enter").Available {
		t.Fatal("finite field did not advertise Space as its only value control")
	}
	updated, _ = sendKey(&layout, "enter")
	layout = updated.(LayoutModel)
	if layout.pane.Editing() || layout.pane.defaultsDetail.Dirty() {
		t.Fatal("undisplayed Enter opened or changed a choice field")
	}
	for _, field := range []string{profile.FieldGPUPowerLimit, profile.FieldGPUShaderCachePath} {
		focusField(t, &layout.pane.defaultsDetail, field)
		context = layout.bindingContext()
		if hasHelpAction(CanonicalKeymap.HelpBindings(context), ActionDetailCycle) || !CanonicalKeymap.Lookup(context, "enter").Available {
			t.Fatal("free input did not advertise Enter and hide Space")
		}
		updated, _ = sendKey(&layout, "space")
		layout = updated.(LayoutModel)
		if layout.pane.defaultsDetail.Dirty() || layout.pane.Editing() {
			t.Fatal("undisplayed Space changed a free input")
		}
	}
}

func TestSettingsSpaceCyclesWithoutOpeningEditor(t *testing.T) {
	for _, key := range []string{"show_hints", "rescan_on_startup"} {
		t.Run(key, func(t *testing.T) {
			settings := NewOptionsModal(NewStyles(DefaultTheme, false))
			settings.OpenEmbedded(config.Default())
			option := *config.OptionByKey(key)
			settings.sections = []config.Section{{Options: []config.Option{option}}}
			settings.sectionCursor, settings.optionCursor = 0, 0
			original := option.Get(settings.config)
			settings, command := settings.UpdateAction(ActionDetailCycle)
			if command != nil || settings.Editing() || !settings.Dirty() || option.Get(settings.config) != original || option.Get(settings.draft) == original {
				t.Fatalf("Space did not change only the Settings draft: editing=%t dirty=%t saved=%q draft=%q error=%v", settings.Editing(), settings.Dirty(), option.Get(settings.config), option.Get(settings.draft), settings.saveError)
			}
			for index := 1; index < len(option.Choices); index++ {
				settings, _ = settings.UpdateAction(ActionDetailCycle)
			}
			if settings.Dirty() || option.Get(settings.draft) != original {
				t.Fatal("full Settings rotation did not return to its saved value")
			}
			settings.saving = true
			settings, _ = settings.UpdateAction(ActionDetailCycle)
			if settings.Dirty() {
				t.Fatal("Space changed Settings during a save")
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
