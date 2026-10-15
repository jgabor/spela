package tui

import (
	"unicode"
	"unicode/utf8"

	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
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
	ActionShowActions         KeyAction = "show-actions"
	ActionEditToggle          KeyAction = "edit-toggle"
	ActionOverlayFocusNext    KeyAction = "overlay-focus-next"
	ActionOverlayLeft         KeyAction = "overlay-left"
	ActionOverlayRight        KeyAction = "overlay-right"
	ActionSortNameAsc         KeyAction = "sort-name-asc"
	ActionSortNameDesc        KeyAction = "sort-name-desc"
	ActionSortDLLsFirst       KeyAction = "sort-dlls-first"
	ActionSortProfileFirst    KeyAction = "sort-profile-first"
	ActionLayoutStandard      KeyAction = "layout-standard"
	ActionLayoutCompact       KeyAction = "layout-compact"
	ActionLayoutFocused       KeyAction = "layout-focused"
	ActionBatchUpdate         KeyAction = "batch-update"
	ActionSearchClear         KeyAction = "search-clear"
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
	ActionDetailCycle         KeyAction = "detail-cycle"
	ActionDetailReset         KeyAction = "detail-reset"
	ActionDetailResetAll      KeyAction = "detail-reset-all"
	ActionApplyOwnValues      KeyAction = "detail-apply-own-values"
	ActionDetailInstall       KeyAction = "detail-install"
	ActionDetailUpdate        KeyAction = "detail-update"
	ActionDetailRestore       KeyAction = "detail-restore"
	ActionStartSearch         KeyAction = "start-search"
	ActionSearchInput         KeyAction = "search-input"
	ActionSearchDelete        KeyAction = "search-delete"
	ActionSearchAccept        KeyAction = "search-accept"
	ActionSearchCancel        KeyAction = "search-cancel"
	ActionEditInput           KeyAction = "edit-input"
	ActionEditDelete          KeyAction = "edit-delete"
	ActionEditCommit          KeyAction = "edit-commit"
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
	Mode             InputMode
	Focus            KeyFocus
	Destination      nav.Destination
	GameScope        bool
	Aspect           nav.Aspect
	HasBackup        bool
	DLLSection       nav.DLLCatalogSection
	ListGroups       bool
	DetailAdjustable bool
	ProfileScope     bool
	CanMoveList      bool
	CanOpen          bool
	// FindRow is true while the List cursor is on the Find row.
	FindRow       bool
	CanSelect     bool
	CanSwitchPane bool
	Editable      bool
	CanCycle      bool
	// FieldOwn is true when the focused profile field holds a value of its own.
	FieldOwn bool
	// OwnValueGames is the number of games that set their own value for the
	// focused All games field. SavesPending is true while a write is unfinished.
	OwnValueGames int
	SavesPending  bool
	// CanJumpPrevious and CanJumpNext say whether a section lies in that
	// direction, so Left and Right are advertised only when they move.
	CanJumpPrevious, CanJumpNext bool
	DLLActionCount               int
	DLLSelectedAction            KeyAction
	DLLSelectedName              string
	HasFields                    bool
	Scrollable                   bool
	Busy                         bool
	HasUpdates                   bool
	SelectedCount                int
	VisibleSelectedCount         int
	SelectedUpdateCount          int
	EditorKind                   EditorValueKind
	DLLActions                   map[KeyAction]actionAvailability
	DLLLabels                    map[KeyAction]string
	EditorInput                  bool
	EditorEnterLabel             string
}

// Availability lets behavior and help make the same enabled/disabled
// decision. A nil function means the binding is available.
type actionAvailability struct {
	available bool
	reason    string
}

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
	Menu         bool
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
				binding = describeBinding(binding, context)
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

// sectionJumpPreviousAvailable allows Left on a profile page that has an earlier
// section, and on the DLLs section of a game, where Left returns to the profile.
func sectionJumpPreviousAvailable(context BindingContext) (bool, string) {
	if context.Destination != nav.DestinationLibrary || !context.ProfileScope {
		return false, "select a profile"
	}
	if !context.CanJumpPrevious {
		return false, "first section"
	}
	return true, ""
}

// sectionJumpNextAvailable allows Right on a profile page that has a later
// section; on a game the last one is DLLs.
func sectionJumpNextAvailable(context BindingContext) (bool, string) {
	if context.Destination != nav.DestinationLibrary || !context.ProfileScope {
		return false, "select a profile"
	}
	if !context.CanJumpNext {
		return false, "last section"
	}
	return true, ""
}

func libraryAvailable(context BindingContext) (bool, string) {
	if context.Destination != nav.DestinationLibrary {
		return false, "Library only"
	}
	return true, ""
}

func listGroupsAvailable(context BindingContext) (bool, string) {
	if !context.ListGroups {
		return false, "no groups in this list"
	}
	return true, ""
}

func libraryProfileAvailable(context BindingContext) (bool, string) {
	if context.Destination != nav.DestinationLibrary || !context.ProfileScope {
		return false, "select a profile"
	}
	if context.Aspect != nav.AspectProfile {
		return false, "open Profile"
	}
	return true, ""
}

// fieldResetAvailable offers Follow on a profile row only when the row holds a
// value of its own; otherwise there is nothing to clear.
func fieldResetAvailable(context BindingContext) (bool, string) {
	if available, reason := libraryProfileAvailable(context); !available {
		return false, reason
	}
	if !context.FieldOwn {
		return false, "nothing to clear"
	}
	return true, ""
}

// applyOwnValuesAvailable offers the All games action only on a field some game
// sets itself, and not while a write is unfinished, so the games it lists are
// the games on disk.
func applyOwnValuesAvailable(context BindingContext) (bool, string) {
	if available, reason := libraryProfileAvailable(context); !available {
		return false, reason
	}
	switch {
	case context.GameScope:
		return false, "open " + profile.AllGamesName
	case context.SavesPending:
		return false, "saving"
	case context.OwnValueGames == 0:
		return false, "no game has its own value"
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

func dllUpdateAvailable(context BindingContext) (bool, string) {
	if !actionRelevant(ActionDetailUpdate, context) {
		return false, "open a DLL view"
	}
	if state, ok := context.DLLActions[ActionDetailUpdate]; ok {
		return state.available, state.reason
	}
	if context.Destination == nav.DestinationDLLCatalog {
		if context.DLLSection != nav.SectionDLLDeployment {
			return false, "open Deployment"
		}
		return true, ""
	}
	return libraryDLLAvailable(context)
}

func libraryDLLRestoreAvailable(context BindingContext) (bool, string) {
	if state, ok := context.DLLActions[ActionDetailRestore]; ok {
		return state.available, state.reason
	}
	if available, reason := libraryDLLAvailable(context); !available {
		return false, reason
	}
	if !context.HasBackup {
		return false, "no backup"
	}
	return true, ""
}

// CanonicalKeymap defines commands, menu entries, and visible key hints together.
var CanonicalKeymap = NewKeymap(
	KeyBinding{Mode: ModeAny, Scope: ScopeGlobal, Action: ActionQuit, Description: "Interrupt", Keys: []KeyLabel{key("ctrl+c", "Ctrl+C")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionShowActions, Description: "Actions", Keys: []KeyLabel{key("0", "0")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationLibrary, Description: "Library", Keys: []KeyLabel{key("1", "1")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationDLLs, Description: "DLL Catalog", Keys: []KeyLabel{key("2", "2")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationMonitor, Description: "Monitor", Keys: []KeyLabel{key("3", "3")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDestinationSettings, Description: "Settings", Keys: []KeyLabel{key("4", "4")}},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionFocusNext, Description: "Switch pane", Keys: []KeyLabel{key("tab", "Tab")}, Availability: func(c BindingContext) (bool, string) { return c.CanSwitchPane, "no other eligible pane" }},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailPrevious, Description: "Previous section", Keys: []KeyLabel{key("left", "←")}, Availability: sectionJumpPreviousAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailNext, Description: "Next section", Keys: []KeyLabel{key("right", "→")}, Availability: sectionJumpNextAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListPrevious, Description: "Previous item", Keys: []KeyLabel{key("up", "↑")}, Availability: func(c BindingContext) (bool, string) { return c.CanMoveList, "no items" }},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListNext, Description: "Next item", Keys: []KeyLabel{key("down", "↓")}, Availability: func(c BindingContext) (bool, string) { return c.CanMoveList, "no items" }},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListSelect, Description: "Open selection", Keys: []KeyLabel{key("enter", "Enter")}, Availability: func(c BindingContext) (bool, string) { return c.CanOpen, "no selection" }},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListPreviousGroup, Description: "Previous group", Keys: []KeyLabel{key("left", "←")}, Availability: listGroupsAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListNextGroup, Description: "Next group", Keys: []KeyLabel{key("right", "→")}, Availability: listGroupsAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListMultiSelect, Description: "Select game", Keys: []KeyLabel{key("space", "Space")}, Availability: func(c BindingContext) (bool, string) {
		return c.Destination == nav.DestinationLibrary && c.CanSelect, "select a game row"
	}},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailPreviousItem, Description: "Previous field", Keys: []KeyLabel{key("up", "↑")}, Availability: detailMovementAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailNextItem, Description: "Next field", Keys: []KeyLabel{key("down", "↓")}, Availability: detailMovementAvailable},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailReset, Description: "Reset field", Availability: fieldResetAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailResetAll, Description: "Reset all fields", Availability: libraryProfileAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionApplyOwnValues, Description: profile.ActionApplyToGamesWithOwnValue, Availability: applyOwnValuesAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailConfirm, Description: "Edit value", Keys: []KeyLabel{key("enter", "Enter")}, Availability: func(c BindingContext) (bool, string) {
		if c.DLLActionCount > 0 {
			availability := c.DLLActions[c.DLLSelectedAction]
			return availability.available, availability.reason
		}
		return c.Editable, "no editable field"
	}, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailCycle, Description: "Next value", Keys: []KeyLabel{key("space", "Space")}, Availability: func(c BindingContext) (bool, string) { return c.CanCycle, "no field" }},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionStartSearch, Description: "Search games", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListToggleDLLFilter, Description: "Toggle DLL filter", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListToggleProfile, Description: "Toggle profile filter", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListClearFilters, Description: "Clear filters (reset sort to A-Z)", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionSortNameAsc, Description: "Sort: A-Z", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionSortNameDesc, Description: "Sort: Z-A", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionSortDLLsFirst, Description: "Sort: DLLs first", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionSortProfileFirst, Description: "Sort: profiles first", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListSelectAll, Description: "Select all filtered games", Availability: libraryAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionListClearSelection, Description: "Clear selected filtered games", Availability: func(c BindingContext) (bool, string) {
		return c.Destination == nav.DestinationLibrary && c.VisibleSelectedCount > 0, "no selected filtered games"
	}, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusList, Scope: ScopeList, Action: ActionBatchUpdate, Description: "Update selected games", Availability: func(c BindingContext) (bool, string) {
		if c.Busy {
			return false, "wait for pending work"
		}
		return c.Destination == nav.DestinationLibrary && c.VisibleSelectedCount > 0 && c.SelectedUpdateCount > 0, "no updates for selected filtered games"
	}, Menu: true},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionRescanLibrary, Description: "Rescan games", Availability: func(c BindingContext) (bool, string) {
		if c.Busy {
			return false, "wait for pending work"
		}
		return libraryAvailable(c)
	}, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailInstall, Description: "Install DLL", Availability: func(c BindingContext) (bool, string) {
		if state, ok := c.DLLActions[ActionDetailInstall]; ok {
			return state.available, state.reason
		}
		return libraryDLLAvailable(c)
	}, Menu: true},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionDetailUpdate, Description: "Update stale DLLs", Availability: dllUpdateAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Focus: FocusDetail, Scope: ScopeDetail, Action: ActionDetailRestore, Description: "Restore DLL backups", Availability: libraryDLLRestoreAvailable, Menu: true},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionLayoutStandard, Description: "Layout: Standard", Menu: true},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionLayoutCompact, Description: "Layout: Compact", Menu: true},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionLayoutFocused, Description: "Layout: Focused", Menu: true},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionShowHelp, Description: "Help", Menu: true},
	KeyBinding{Mode: ModeBrowse, Scope: ScopeGlobal, Action: ActionQuit, Description: "Quit", Availability: func(c BindingContext) (bool, string) { return !c.Busy, "wait for pending operations" }, Menu: true},
	KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionFocusNext, Description: "Next control", Keys: []KeyLabel{key("tab", "Tab")}},
	KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionSearchAccept, Description: "Apply search", Keys: []KeyLabel{key("enter", "Enter")}},
	KeyBinding{Mode: ModeSearch, Scope: ScopeSearch, Action: ActionSearchInput, Description: "Search text", Keys: []KeyLabel{printable("Text")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionFocusNext, Description: "Next control", Keys: []KeyLabel{key("tab", "Tab")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditCancel, Description: "Cancel edit", Keys: []KeyLabel{key("esc", "Esc")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditCommit, Description: "Apply", Keys: []KeyLabel{key("enter", "Enter")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionDetailDecrease, Description: "Previous choice", Keys: []KeyLabel{key("left", "←")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionDetailIncrease, Description: "Next choice", Keys: []KeyLabel{key("right", "→")}},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionDetailPreviousItem, Description: "Previous choice", Keys: []KeyLabel{key("up", "↑")}, Availability: editorChoiceAvailable},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionDetailNextItem, Description: "Next choice", Keys: []KeyLabel{key("down", "↓")}, Availability: editorChoiceAvailable},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditToggle, Description: "Toggle", Keys: []KeyLabel{key("space", "Space")}, Availability: func(c BindingContext) (bool, string) { return c.EditorInput, "focus the input" }},
	KeyBinding{Mode: ModeEdit, Scope: ScopeEdit, Action: ActionEditInput, Description: "Edit text", Keys: []KeyLabel{printable("Text")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayFocusNext, Description: "Next control", Keys: []KeyLabel{key("tab", "Tab")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayConfirm, Description: "Activate", Keys: []KeyLabel{key("enter", "Enter")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayPrevious, Description: "Previous", Keys: []KeyLabel{key("up", "↑")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayNext, Description: "Next", Keys: []KeyLabel{key("down", "↓")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayLeft, Description: "Previous button", Keys: []KeyLabel{key("left", "←")}},
	KeyBinding{Mode: ModeOverlay, Scope: ScopeOverlay, Action: ActionOverlayRight, Description: "Next button", Keys: []KeyLabel{key("right", "→")}},
)

func detailMovementAvailable(c BindingContext) (bool, string) {
	return c.HasFields || c.Scrollable || c.DLLActionCount > 0, "no fields, actions or overflowing content"
}

func describeBinding(binding KeyBinding, c BindingContext) KeyBinding {
	switch binding.Action {
	case ActionDetailDecrease, ActionDetailIncrease:
		if c.Mode == ModeEdit && !c.EditorInput {
			binding.Description = "Choose button"
		} else if c.Mode == ModeEdit && c.EditorKind != EditorBool && c.EditorKind != EditorChoice {
			binding.Description = "Move cursor"
		}
	case ActionEditToggle:
		if c.EditorKind != EditorBool && c.EditorKind != EditorChoice {
			binding.Description = "Type space"
		}

	case ActionListSelect:
		if c.Destination == nav.DestinationLibrary && c.FindRow {
			binding.Description = "Find games"
		} else if c.Destination == nav.DestinationLibrary && c.VisibleSelectedCount > 0 {
			binding.Description = "Batch actions"
		}
	case ActionDetailPreviousItem:
		if c.DLLActionCount > 0 && c.Mode == ModeBrowse {
			binding.Description = "Previous action"
		} else if !c.HasFields && c.Mode == ModeBrowse {
			binding.Description = "Scroll up"
		}
	case ActionDetailNextItem:
		if c.DLLActionCount > 0 && c.Mode == ModeBrowse {
			binding.Description = "Next action"
		} else if !c.HasFields && c.Mode == ModeBrowse {
			binding.Description = "Scroll down"
		}
	case ActionDetailConfirm:
		if c.DLLActionCount > 0 {
			binding.Description = c.DLLSelectedName
		}
	case ActionDetailCycle:
		if c.Destination == nav.DestinationLibrary && c.HasFields {
			switch {
			case c.FieldOwn && c.GameScope:
				binding.Description = profile.ActionFollowAllGames
			case c.FieldOwn:
				binding.Description = profile.ActionUseSystemDefault
			case c.GameScope:
				binding.Description = profile.ActionSetForThisGame
			default:
				binding.Description = profile.ActionSetHere
			}
		}
	case ActionDetailReset:
		if !c.GameScope {
			binding.Description = profile.ActionUseSystemDefault
		} else {
			binding.Description = profile.ActionFollowAllGames
		}
	case ActionDetailResetAll:
		if !c.GameScope {
			binding.Description = profile.ActionUseSystemDefaultForAllSettings
		} else {
			binding.Description = profile.ActionFollowAllGamesForAllSettings
		}
	case ActionDetailUpdate, ActionDetailInstall, ActionDetailRestore:
		if label := c.DLLLabels[binding.Action]; label != "" {
			binding.Description = label
		} else if binding.Action == ActionDetailUpdate && c.Destination == nav.DestinationDLLCatalog {
			binding.Description = "Update all stale deployments"
		}
	case ActionEditCommit, ActionSearchAccept:
		if c.EditorEnterLabel != "" {
			binding.Description = c.EditorEnterLabel
		}
	}
	return binding
}

// Action returns the same scoped availability used by keys and menu invocation.
func (k Keymap) Action(context BindingContext, action KeyAction) BindingResolution {
	for _, binding := range k.bindings {
		if binding.Action != action || binding.Mode != context.Mode || (binding.Focus != FocusAny && binding.Focus != context.Focus) {
			continue
		}
		binding = describeBinding(binding, context)
		available, reason := bindingAvailability(binding, context)
		return BindingResolution{Binding: binding, Available: available, Reason: reason, Supported: true}
	}
	return BindingResolution{Binding: KeyBinding{Action: ActionNoOp}, Reason: "unavailable in this context"}
}

func editorChoiceAvailable(c BindingContext) (bool, string) {
	return c.EditorInput && (c.EditorKind == EditorBool || c.EditorKind == EditorChoice), "focus a choice input"
}
