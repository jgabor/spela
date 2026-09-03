package tui

import (
	"unicode"
	"unicode/utf8"

	"github.com/jgabor/spela/internal/nav"
)

// InputMode identifies the input consumer that gets the first opportunity to
// handle a key. Modes are deliberately independent from focus: a search field
// can retain its owning pane while Search still owns keyboard input.
type InputMode string

const (
	ModeAny     InputMode = ""
	ModeBrowse  InputMode = "browse"
	ModeSearch  InputMode = "search"
	ModeEdit    InputMode = "edit"
	ModeOverlay InputMode = "overlay"
)

// KeyFocus is the visibly focused part of the two-pane workspace.
type KeyFocus string

const (
	FocusAny    KeyFocus = ""
	FocusList   KeyFocus = "list"
	FocusDetail KeyFocus = "detail"
)

// BindingScope documents which part of the interface owns an action. It is
// used both by dispatch and by help rendering.
type BindingScope string

const (
	ScopeGlobal  BindingScope = "global"
	ScopeList    BindingScope = "list"
	ScopeDetail  BindingScope = "detail"
	ScopeSearch  BindingScope = "search"
	ScopeEdit    BindingScope = "edit"
	ScopeOverlay BindingScope = "overlay"
)

// KeyAction is a semantic operation. Widgets dispatch actions rather than
// independently interpreting raw key strings.
type KeyAction string

const (
	ActionNoOp                KeyAction = "no-op"
	ActionQuit                KeyAction = "quit"
	ActionShowHelp            KeyAction = "show-help"
	ActionDestinationLibrary  KeyAction = "destination-library"
	ActionDestinationDLLs     KeyAction = "destination-dlls"
	ActionDestinationMonitor  KeyAction = "destination-monitor"
	ActionDestinationSettings KeyAction = "destination-settings"
	ActionFocusNext           KeyAction = "focus-next"
	ActionFocusPrevious       KeyAction = "focus-previous"
	ActionToggleCompact       KeyAction = "toggle-compact"
	ActionToggleFocused       KeyAction = "toggle-focused"
	ActionRescanLibrary       KeyAction = "rescan-library"
	ActionListPrevious        KeyAction = "list-previous"
	ActionListNext            KeyAction = "list-next"
	ActionListSelect          KeyAction = "list-select"
	ActionListPreviousGroup   KeyAction = "list-previous-group"
	ActionListNextGroup       KeyAction = "list-next-group"
	ActionListToggleDLLFilter KeyAction = "list-toggle-dll-filter"
	ActionListToggleProfile   KeyAction = "list-toggle-profile-filter"
	ActionListSort            KeyAction = "list-sort"
	ActionListClearFilters    KeyAction = "list-clear-filters"
	ActionListMultiSelect     KeyAction = "list-multi-select"
	ActionListSelectAll       KeyAction = "list-select-all"
	ActionListClearSelection  KeyAction = "list-clear-selection"
	ActionDetailPrevious      KeyAction = "detail-previous"
	ActionDetailNext          KeyAction = "detail-next"
	ActionDetailPreviousItem  KeyAction = "detail-previous-item"
	ActionDetailNextItem      KeyAction = "detail-next-item"
	ActionDetailDecrease      KeyAction = "detail-decrease"
	ActionDetailIncrease      KeyAction = "detail-increase"
	ActionDetailConfirm       KeyAction = "detail-confirm"
	ActionDetailSave          KeyAction = "detail-save"
	ActionDetailReset         KeyAction = "detail-reset"
	ActionDetailResetAll      KeyAction = "detail-reset-all"
	ActionDetailInstall       KeyAction = "detail-install"
	ActionDetailUpdate        KeyAction = "detail-update"
	ActionDetailRestore       KeyAction = "detail-restore"
	ActionCancelDraft         KeyAction = "cancel-draft"
	ActionStartSearch         KeyAction = "start-search"
	ActionSearchInput         KeyAction = "search-input"
	ActionSearchDelete        KeyAction = "search-delete"
	ActionSearchAccept        KeyAction = "search-accept"
	ActionSearchCancel        KeyAction = "search-cancel"
	ActionEditInput           KeyAction = "edit-input"
	ActionEditDelete          KeyAction = "edit-delete"
	ActionEditCommit          KeyAction = "edit-commit"
	ActionEditSave            KeyAction = "edit-save"
	ActionEditCancel          KeyAction = "edit-cancel"
	ActionOverlayPrevious     KeyAction = "overlay-previous"
	ActionOverlayNext         KeyAction = "overlay-next"
	ActionOverlayConfirm      KeyAction = "overlay-confirm"
	ActionOverlayClose        KeyAction = "overlay-close"
)

// KeyLabel contains the value used for matching and the label rendered in
// status/help UI. Printable matches any single printable rune and is intended
// for text-entry consumers.
type KeyLabel struct {
	Key       string
	Label     string
	Printable bool
}

// BindingContext is the complete state needed to select a binding.
type BindingContext struct {
	Mode        InputMode
	Focus       KeyFocus
	Destination nav.Destination
	GameScope   bool
	Aspect      nav.Aspect
	HasBackup   bool
}

// Availability lets behavior and help make the same enabled/disabled
// decision. A nil function means the binding is available.
type Availability func(BindingContext) (available bool, reason string)

// KeyBinding is the canonical description of one user action.
type KeyBinding struct {
	Mode         InputMode
	Focus        KeyFocus
	Scope        BindingScope
	Action       KeyAction
	Description  string
	Keys         []KeyLabel
	Availability Availability
}

// BindingResolution is returned for every key, including unsupported keys.
// Supported distinguishes an intentional binding from the explicit no-op.
type BindingResolution struct {
	Binding   KeyBinding
	Key       string
	Available bool
	Reason    string
	Supported bool
}

// Keymap is the single source of truth for keyboard behavior and help.
type Keymap struct {
	bindings []KeyBinding
}

func NewKeymap(bindings ...KeyBinding) Keymap {
	return Keymap{bindings: append([]KeyBinding(nil), bindings...)}
}

// Lookup applies input precedence before global and focus-specific bindings.
// Exact-mode bindings win over ModeAny bindings; within either group, the
// visibly focused pane wins over global scope.
func (k Keymap) Lookup(context BindingContext, key string) BindingResolution {
	if key == "escape" {
		key = "esc"
	}
	var unavailable *BindingResolution
	for _, exactMode := range []bool{true, false} {
		for _, exactFocus := range []bool{true, false} {
			for _, binding := range k.bindings {
				if (binding.Mode == context.Mode) != exactMode || (!exactMode && binding.Mode != ModeAny) {
					continue
				}
				if (binding.Focus == context.Focus) != exactFocus || (!exactFocus && binding.Focus != FocusAny) {
					continue
				}
				if !bindingMatchesKey(binding, key) {
					continue
				}
				available, reason := bindingAvailability(binding, context)
				resolution := BindingResolution{Binding: binding, Key: key, Available: available, Reason: reason, Supported: true}
				if available {
					return resolution
				}
				if unavailable == nil {
					unavailable = &resolution
				}
			}
		}
	}
	if unavailable != nil {
		return *unavailable
	}

	return BindingResolution{
		Binding:   KeyBinding{Mode: context.Mode, Focus: context.Focus, Action: ActionNoOp},
		Key:       key,
		Available: true,
	}
}

// HelpBindings returns actions that Lookup can execute in the current context.
func (k Keymap) HelpBindings(context BindingContext) []BindingResolution {
	results := make([]BindingResolution, 0, len(k.bindings))
	for _, binding := range k.bindings {
		if binding.Mode != ModeAny && binding.Mode != context.Mode {
			continue
		}
		if binding.Focus != FocusAny && binding.Focus != context.Focus {
			continue
		}
		available, reason := bindingAvailability(binding, context)
		if !available {
			continue
		}
		results = append(results, BindingResolution{
			Binding: binding, Available: available, Reason: reason, Supported: true,
		})
	}
	return results
}

func bindingAvailability(binding KeyBinding, context BindingContext) (bool, string) {
	if binding.Availability == nil {
		return true, ""
	}
	return binding.Availability(context)
}

func bindingMatchesKey(binding KeyBinding, key string) bool {
	for _, candidate := range binding.Keys {
		if candidate.Key == key || (candidate.Printable && isPrintableKey(key)) {
			return true
		}
	}
	return false
}

func isPrintableKey(key string) bool {
	r, size := utf8.DecodeRuneInString(key)
	return r != utf8.RuneError && size == len(key) && unicode.IsPrint(r)
}

func key(key, label string) KeyLabel { return KeyLabel{Key: key, Label: label} }

func printable(label string) KeyLabel { return KeyLabel{Label: label, Printable: true} }

func libraryGameAvailable(context BindingContext) (bool, string) {
	if context.Destination != nav.DestinationLibrary || !context.GameScope {
		return false, "select a game"
	}
	return true, ""
}

func libraryAvailable(context BindingContext) (bool, string) {
	if context.Destination != nav.DestinationLibrary {
		return false, "Library only"
	}
	return true, ""
}

func libraryProfileAvailable(context BindingContext) (bool, string) {
	if available, reason := libraryGameAvailable(context); !available {
		return false, reason
	}
	if context.Aspect != nav.AspectProfile {
		return false, "open Profile"
	}
	return true, ""
}

func libraryDLLAvailable(context BindingContext) (bool, string) {
	if available, reason := libraryGameAvailable(context); !available {
		return false, reason
	}
	if context.Aspect != nav.AspectDLLs {
		return false, "open DLLs"
	}
	return true, ""
}

func libraryDLLRestoreAvailable(context BindingContext) (bool, string) {
	if available, reason := libraryDLLAvailable(context); !available {
		return false, reason
	}
	if !context.HasBackup {
		return false, "no backup"
	}
	return true, ""
}

func settingsAvailable(context BindingContext) (bool, string) {
	if context.Destination != nav.DestinationSettings {
		return false, "Settings only"
	}
	return true, ""
}

// CanonicalKeymap defines the shell-level actions shared by routing and help.
// Pane-specific actions can be appended when their replacement widgets land.
var CanonicalKeymap = NewKeymap(
	KeyBinding{Mode: ModeAny, Scope: ScopeGlobal, Action: ActionQuit, Description: "Quit", Keys: []KeyLabel{key("ctrl+c", "Ctrl+C")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionQuit, Description: "Quit", Keys: []KeyLabel{key("q", "q")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionShowHelp, Description: "Keyboard shortcuts", Keys: []KeyLabel{key("?", "?")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationLibrary, Description: "Library", Keys: []KeyLabel{key("1", "1")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationDLLs, Description: "DLL Catalog", Keys: []KeyLabel{key("2", "2")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationMonitor, Description: "Monitor", Keys: []KeyLabel{key("3", "3")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationSettings, Description: "Settings", Keys: []KeyLabel{key("4", "4")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionFocusNext, Description: "Next pane", Keys: []KeyLabel{key("tab", "Tab")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionFocusPrevious, Description: "Previous pane", Keys: []KeyLabel{key("shift+tab", "Shift+Tab")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionToggleCompact, Description: "Toggle compact layout", Keys: []KeyLabel{key("f5", "F5")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionToggleFocused, Description: "Toggle focused layout", Keys: []KeyLabel{key("f11", "F11")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionRescanLibrary, Description: "Rescan games", Keys: []KeyLabel{key("ctrl+r", "Ctrl+R")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailPrevious, Description: "Previous detail view", Keys: []KeyLabel{key("[", "[")}, Availability: libraryGameAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailNext, Description: "Next detail view", Keys: []KeyLabel{key("]", "]")}, Availability: libraryGameAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListPrevious, Description: "Previous item", Keys: []KeyLabel{key("up", "↑"), key("k", "k")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListNext, Description: "Next item", Keys: []KeyLabel{key("down", "↓"), key("j", "j")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListSelect, Description: "Open selection", Keys: []KeyLabel{key("enter", "Enter")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListPreviousGroup, Description: "Previous group", Keys: []KeyLabel{key("left", "←"), key("h", "h")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListNextGroup, Description: "Next group", Keys: []KeyLabel{key("right", "→"), key("l", "l")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListToggleDLLFilter, Description: "Toggle DLL filter", Keys: []KeyLabel{key("d", "d")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListToggleProfile, Description: "Toggle profile filter", Keys: []KeyLabel{key("p", "p")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListSort, Description: "Change sort", Keys: []KeyLabel{key("s", "s")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListClearFilters, Description: "Clear filters", Keys: []KeyLabel{key("C", "C")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListMultiSelect, Description: "Toggle selection", Keys: []KeyLabel{key("space", "Space")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListSelectAll, Description: "Select all", Keys: []KeyLabel{key("a", "a")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListClearSelection, Description: "Clear selection", Keys: []KeyLabel{key("A", "A")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailPreviousItem, Description: "Previous field", Keys: []KeyLabel{key("up", "↑"), key("k", "k")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailNextItem, Description: "Next field", Keys: []KeyLabel{key("down", "↓"), key("j", "j")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailDecrease, Description: "Decrease value", Keys: []KeyLabel{key("left", "←"), key("h", "h")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailIncrease, Description: "Increase value", Keys: []KeyLabel{key("right", "→"), key("l", "l")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailConfirm, Description: "Edit or confirm", Keys: []KeyLabel{key("enter", "Enter")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailSave, Description: "Save changes", Keys: []KeyLabel{key("s", "s")}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionCancelDraft, Description: "Discard changes", Keys: []KeyLabel{key("esc", "Esc")}, Availability: libraryProfileAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionCancelDraft, Description: "Discard changes", Keys: []KeyLabel{key("esc", "Esc")}, Availability: settingsAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailReset, Description: "Reset field", Keys: []KeyLabel{key("r", "r")}, Availability: libraryProfileAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailResetAll, Description: "Reset all fields", Keys: []KeyLabel{key("R", "R")}, Availability: libraryProfileAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailInstall, Description: "Install DLL", Keys: []KeyLabel{key("i", "i")}, Availability: libraryDLLAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailUpdate, Description: "Update stale DLLs", Keys: []KeyLabel{key("u", "u"), key("U", "U"), key("ctrl+u", "Ctrl+U")}, Availability: libraryDLLAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailRestore, Description: "Restore DLLs", Keys: []KeyLabel{key("f6", "F6")}, Availability: libraryDLLRestoreAvailable},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionStartSearch, Description: "Search", Keys: []KeyLabel{key("/", "/"), key("ctrl+f", "Ctrl+F")}, Availability: libraryAvailable},
	KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionSearchCancel, Description: "Cancel search", Keys: []KeyLabel{key("esc", "Esc")}},
	KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionSearchAccept, Description: "Accept search", Keys: []KeyLabel{key("enter", "Enter")}},
	KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionSearchDelete, Description: "Delete character", Keys: []KeyLabel{key("backspace", "Backspace")}},
	KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionSearchInput, Description: "Search text", Keys: []KeyLabel{printable("Text")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditCancel, Description: "Discard edits", Keys: []KeyLabel{key("esc", "Esc")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditCommit, Description: "Apply value to draft", Keys: []KeyLabel{key("enter", "Enter"), key("ctrl+s", "Ctrl+S")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditDelete, Description: "Delete character", Keys: []KeyLabel{key("backspace", "Backspace")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditInput, Description: "Edit value", Keys: []KeyLabel{printable("Text")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayClose, Description: "Close overlay", Keys: []KeyLabel{key("esc", "Esc")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayConfirm, Description: "Confirm", Keys: []KeyLabel{key("enter", "Enter")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayPrevious, Description: "Previous option", Keys: []KeyLabel{key("up", "Up"), key("k", "k")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayNext, Description: "Next option", Keys: []KeyLabel{key("down", "Down"), key("j", "j")}},
)
