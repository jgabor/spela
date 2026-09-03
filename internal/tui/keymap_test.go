package tui

import (
	"testing"

	"github.com/jgabor/spela/internal/nav"
)

func TestCanonicalKeymapReservesPrintableDestinationKeysForBrowse(t *testing.T) {
	tests := []struct {
		mode InputMode
		key  string
		want KeyAction
	}{
		{ModeBrowse, "1", ActionDestinationLibrary},
		{ModeBrowse, "4", ActionDestinationSettings},
		{ModeSearch, "1", ActionSearchInput},
		{ModeSearch, "q", ActionSearchInput},
		{ModeEdit, "2", ActionEditInput},
		{ModeEdit, "q", ActionEditInput},
		{ModeOverlay, "2", ActionNoOp},
	}
	for _, test := range tests {
		t.Run(string(test.mode)+"/"+test.key, func(t *testing.T) {
			got := CanonicalKeymap.Lookup(BindingContext{Mode: test.mode, Focus: FocusList}, test.key)
			if got.Binding.Action != test.want {
				t.Fatalf("Lookup() action = %q, want %q", got.Binding.Action, test.want)
			}
		})
	}
}

func TestKeymapModeAndFocusPrecedence(t *testing.T) {
	keymap := NewKeymap(
		KeyBinding{Mode: ModeAny, Scope: ScopeGlobal, Action: ActionQuit, Keys: []KeyLabel{key("x", "x")}},
		KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionEditSave, Keys: []KeyLabel{key("x", "x")}},
		KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionSearchInput, Keys: []KeyLabel{printable("Text")}},
	)

	if got := keymap.Lookup(BindingContext{Mode: ModeBrowse, Focus: FocusDetail}, "x").Binding.Action; got != ActionEditSave {
		t.Fatalf("focused exact-mode action = %q, want %q", got, ActionEditSave)
	}
	if got := keymap.Lookup(BindingContext{Mode: ModeBrowse, Focus: FocusList}, "x").Binding.Action; got != ActionQuit {
		t.Fatalf("global fallback action = %q, want %q", got, ActionQuit)
	}
	if got := keymap.Lookup(BindingContext{Mode: ModeSearch, Focus: FocusDetail}, "x").Binding.Action; got != ActionSearchInput {
		t.Fatalf("mode consumer action = %q, want %q", got, ActionSearchInput)
	}
}

func TestKeymapAvailabilityDrivesLookupAndHelp(t *testing.T) {
	disabled := func(BindingContext) (bool, string) { return false, "nothing selected" }
	binding := KeyBinding{
		Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail,
		Action: ActionEditSave, Description: "Save", Keys: []KeyLabel{key("s", "s")},
		Availability: disabled,
	}
	keymap := NewKeymap(binding)
	context := BindingContext{Mode: ModeBrowse, Focus: FocusDetail}

	resolution := keymap.Lookup(context, "s")
	if !resolution.Supported || resolution.Available || resolution.Reason != "nothing selected" {
		t.Fatalf("lookup availability = supported %t, available %t, reason %q", resolution.Supported, resolution.Available, resolution.Reason)
	}
	if help := keymap.HelpBindings(context); len(help) != 0 {
		t.Fatalf("help advertised unavailable binding: %#v", help)
	}
}

func TestCanonicalHelpOmitsUnavailableDestinationActions(t *testing.T) {
	dllCatalog := CanonicalKeymap.HelpBindings(BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationDLLCatalog})
	for _, action := range []KeyAction{ActionStartSearch, ActionRescanLibrary, ActionDetailInstall, ActionDetailUpdate, ActionDetailRestore} {
		if hasHelpAction(dllCatalog, action) {
			t.Fatalf("DLL Catalog help advertised unavailable action %q", action)
		}
	}
	deployment := BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationDLLCatalog, DLLSection: nav.SectionDLLDeployment}
	if got := CanonicalKeymap.Lookup(deployment, "U"); !got.Available || got.Binding.Action != ActionDetailUpdate {
		t.Fatalf("catalog update binding = %#v", got)
	}

	dllDetail := BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, GameScope: true, Aspect: nav.AspectDLLs, HasBackup: true}
	if got := CanonicalKeymap.Lookup(dllDetail, "f6"); !got.Available || got.Binding.Action != ActionDetailRestore {
		t.Fatalf("portable restore binding = %#v", got)
	}
	if got := CanonicalKeymap.Lookup(dllDetail, "ctrl+shift+r"); got.Supported {
		t.Fatalf("non-portable restore binding remains supported: %#v", got)
	}
	dllDetail.HasBackup = false
	if hasHelpAction(CanonicalKeymap.HelpBindings(dllDetail), ActionDetailRestore) {
		t.Fatal("help advertised restore without a backup")
	}
}

func TestCanonicalHorizontalBindingsMatchExecutableContexts(t *testing.T) {
	tests := []struct {
		name      string
		context   BindingContext
		hAction   KeyAction
		lAction   KeyAction
		available bool
		supported bool
	}{
		{"Library List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationLibrary}, ActionListPreviousGroup, ActionListNextGroup, false, true},
		{"DLL Catalog List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationDLLCatalog, ListGroups: true}, ActionListPreviousGroup, ActionListNextGroup, true, true},
		{"Monitor List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationMonitor}, ActionListPreviousGroup, ActionListNextGroup, false, true},
		{"Settings List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationSettings, ListGroups: true}, ActionListPreviousGroup, ActionListNextGroup, true, true},
		{"game Overview Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, GameScope: true, Aspect: nav.AspectOverview}, ActionDetailDecrease, ActionDetailIncrease, false, true},
		{"game Profile Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, GameScope: true, Aspect: nav.AspectProfile}, ActionDetailDecrease, ActionDetailIncrease, false, true},
		{"game DLL Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, GameScope: true, Aspect: nav.AspectDLLs}, ActionDetailDecrease, ActionDetailIncrease, false, true},
		{"adjustable default Profile Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, Aspect: nav.AspectProfile, DetailAdjustable: true}, ActionDetailDecrease, ActionDetailIncrease, true, true},
		{"non-adjustable default Profile Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, Aspect: nav.AspectProfile}, ActionDetailDecrease, ActionDetailIncrease, false, true},
		{"DLL Catalog Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationDLLCatalog}, ActionDetailDecrease, ActionDetailIncrease, false, true},
		{"Monitor Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationMonitor}, ActionDetailDecrease, ActionDetailIncrease, false, true},
		{"adjustable Settings Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationSettings, DetailAdjustable: true}, ActionDetailDecrease, ActionDetailIncrease, true, true},
		{"path Settings Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationSettings}, ActionDetailDecrease, ActionDetailIncrease, false, true},
		{"Search input", BindingContext{Mode: ModeSearch, Focus: FocusList, Destination: nav.DestinationLibrary}, ActionSearchInput, ActionSearchInput, true, true},
		{"Edit input", BindingContext{Mode: ModeEdit, Focus: FocusDetail, Destination: nav.DestinationSettings}, ActionEditInput, ActionEditInput, true, true},
		{"Overlay", BindingContext{Mode: ModeOverlay, Focus: FocusDetail}, ActionNoOp, ActionNoOp, true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, wantAction := range map[string]KeyAction{"h": test.hAction, "l": test.lAction} {
				resolution := CanonicalKeymap.Lookup(test.context, key)
				if resolution.Supported != test.supported || resolution.Available != test.available || resolution.Binding.Action != wantAction {
					t.Errorf("%s resolution = %#v, want action %q, available %t, supported %t", key, resolution, wantAction, test.available, test.supported)
				}
				if test.context.Mode == ModeBrowse {
					visible := hasHelpAction(CanonicalKeymap.HelpBindings(test.context), wantAction)
					if visible != test.available {
						t.Errorf("%s help visibility = %t, want %t", key, visible, test.available)
					}
				}
			}
		})
	}
}

func TestKeymapUnsupportedKeyIsExplicitNoOp(t *testing.T) {
	resolution := CanonicalKeymap.Lookup(BindingContext{Mode: ModeOverlay, Focus: FocusDetail}, "f12")
	if resolution.Supported {
		t.Fatal("unsupported key reported as supported")
	}
	if resolution.Binding.Action != ActionNoOp {
		t.Fatalf("unsupported action = %q, want explicit %q", resolution.Binding.Action, ActionNoOp)
	}
	if resolution.Binding.Mode != ModeOverlay || resolution.Binding.Focus != FocusDetail {
		t.Fatalf("no-op lost routing context: %#v", resolution.Binding)
	}
}

func TestCanonicalHelpUsesModeAndVisibleFocus(t *testing.T) {
	search := CanonicalKeymap.HelpBindings(BindingContext{Mode: ModeSearch, Focus: FocusList})
	if hasHelpAction(search, ActionDestinationLibrary) || !hasHelpAction(search, ActionSearchInput) {
		t.Fatalf("search help contains wrong actions: %#v", search)
	}

	focusMap := NewKeymap(
		KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionSearchAccept, Keys: []KeyLabel{key("enter", "Enter")}},
		KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionEditSave, Keys: []KeyLabel{key("s", "s")}},
	)
	listHelp := focusMap.HelpBindings(BindingContext{Mode: ModeBrowse, Focus: FocusList})
	if !hasHelpAction(listHelp, ActionSearchAccept) || hasHelpAction(listHelp, ActionEditSave) {
		t.Fatalf("help ignored visible focus: %#v", listHelp)
	}
}

func hasHelpAction(bindings []BindingResolution, action KeyAction) bool {
	for _, binding := range bindings {
		if binding.Binding.Action == action {
			return true
		}
	}
	return false
}
