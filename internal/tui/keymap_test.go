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

func TestCanonicalHelpOmitsUnavailableDestinationKeys(t *testing.T) {
	catalog := BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationDLLCatalog}
	for _, action := range []KeyAction{ActionStartSearch, ActionRescanLibrary, ActionDetailInstall, ActionDetailUpdate, ActionDetailRestore} {
		if hasHelpAction(CanonicalKeymap.HelpBindings(catalog), action) {
			t.Fatalf("menu-only action %s became a keyboard shortcut", action)
		}
	}
	catalog.DLLSection = nav.SectionDLLDeployment
	catalog.DLLActions = map[KeyAction]actionAvailability{ActionDetailUpdate: {true, ""}}
	if result := CanonicalKeymap.Action(catalog, ActionDetailUpdate); !result.Available {
		t.Fatalf("eligible deployment update unavailable: %#v", result)
	}
	catalog.DLLActions[ActionDetailUpdate] = actionAvailability{false, "no stale deployments"}
	if result := CanonicalKeymap.Action(catalog, ActionDetailUpdate); result.Available || result.Reason != "no stale deployments" {
		t.Fatalf("availability mismatch: %#v", result)
	}
	for _, key := range []string{"U", "f6", "ctrl+shift+r", "/", "[", "]"} {
		if CanonicalKeymap.Lookup(catalog, key).Supported {
			t.Fatalf("retired shortcut %q remains", key)
		}
	}
}

func TestCanonicalHorizontalBindingsMatchExecutableContexts(t *testing.T) {
	tests := []struct {
		name           string
		context        BindingContext
		previous, next KeyAction
		available      bool
	}{
		{"Library List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationLibrary}, ActionListPreviousGroup, ActionListNextGroup, false},
		{"Catalog List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationDLLCatalog, ListGroups: true}, ActionListPreviousGroup, ActionListNextGroup, true},
		{"Settings List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationSettings, ListGroups: true}, ActionListPreviousGroup, ActionListNextGroup, true},
		{"Monitor List", BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: nav.DestinationMonitor}, ActionListPreviousGroup, ActionListNextGroup, false},
		{"Game Overview", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, GameScope: true, Aspect: nav.AspectOverview}, ActionDetailPrevious, ActionDetailNext, true},
		{"Game Profile", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, GameScope: true, Aspect: nav.AspectProfile}, ActionDetailPrevious, ActionDetailNext, true},
		{"Game DLLs", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, GameScope: true, Aspect: nav.AspectDLLs}, ActionDetailPrevious, ActionDetailNext, true},
		{"Root Profile", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, ProfileScope: true, Aspect: nav.AspectProfile}, ActionDetailPrevious, ActionDetailNext, false},
		{"Settings Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationSettings}, ActionDetailPrevious, ActionDetailNext, false},
		{"Monitor Detail", BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationMonitor}, ActionDetailPrevious, ActionDetailNext, false},
		{"Choice Editor", BindingContext{Mode: ModeEdit, Focus: FocusDetail, EditorInput: true, EditorKind: EditorChoice}, ActionDetailDecrease, ActionDetailIncrease, true},
		{"Overlay", BindingContext{Mode: ModeOverlay, Focus: FocusDetail}, ActionOverlayLeft, ActionOverlayRight, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, want := range map[string]KeyAction{"left": test.previous, "right": test.next} {
				result := CanonicalKeymap.Lookup(test.context, key)
				if !result.Supported || result.Binding.Action != want || result.Available != test.available {
					t.Errorf("%s result=%#v, want %s available=%t", key, result, want, test.available)
				}
				if test.context.Mode == ModeBrowse && hasHelpAction(CanonicalKeymap.HelpBindings(test.context), want) != test.available {
					t.Errorf("%s visibility differs from availability", key)
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
