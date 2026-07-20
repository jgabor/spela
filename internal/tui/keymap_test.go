package tui

import "testing"

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
	help := keymap.HelpBindings(context)
	if len(help) != 1 || help[0].Binding.Action != resolution.Binding.Action || help[0].Available != resolution.Available || help[0].Reason != resolution.Reason {
		t.Fatalf("help binding does not reflect lookup: %#v vs %#v", help, resolution)
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
