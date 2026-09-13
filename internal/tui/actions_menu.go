package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/jgabor/spela/internal/nav"
)

// actionMenu owns only its cursor and Close control. Availability is refreshed
// from the shell before both rendering and dispatch.
type actionMenu struct {
	cursor       int
	closeFocused bool
}

func actionRelevant(action KeyAction, context BindingContext) bool {
	switch action {
	case ActionStartSearch, ActionListToggleDLLFilter, ActionListToggleProfile,
		ActionListClearFilters, ActionListSelectAll, ActionListClearSelection,
		ActionSortNameAsc, ActionSortNameDesc, ActionSortDLLsFirst, ActionSortProfileFirst,
		ActionBatchUpdate:
		return context.Destination == nav.DestinationLibrary && context.Focus == FocusList
	case ActionRescanLibrary:
		return context.Destination == nav.DestinationLibrary
	case ActionDetailReset, ActionDetailResetAll:
		return context.Destination == nav.DestinationLibrary && context.ProfileScope && context.Aspect == nav.AspectProfile && context.Focus == FocusDetail
	case ActionDetailInstall, ActionDetailRestore:
		return context.Destination == nav.DestinationLibrary && context.GameScope && context.Aspect == nav.AspectDLLs && context.Focus == FocusDetail
	case ActionDetailUpdate:
		return context.Destination == nav.DestinationDLLCatalog && context.DLLSection == nav.SectionDLLDeployment ||
			context.Destination == nav.DestinationLibrary && context.GameScope && context.Aspect == nav.AspectDLLs && context.Focus == FocusDetail
	case ActionDetailSave, ActionCancelDraft:
		return context.SaveAvailable
	case ActionDetailConfirm:
		return context.Editable || context.DLLActionCount > 0
	default:
		return true
	}
}

func (k Keymap) MenuBindings(context BindingContext) []BindingResolution {
	var items []BindingResolution
	for _, binding := range k.bindings {
		// DLL workflows already have their own named menu entries.
		if binding.Action == ActionDetailConfirm && context.DLLActionCount > 0 {
			continue
		}
		if !binding.Menu || binding.Mode != context.Mode || (binding.Focus != FocusAny && binding.Focus != context.Focus) || !actionRelevant(binding.Action, context) {
			continue
		}
		items = append(items, k.Action(context, binding.Action))
	}
	return items
}

func (m LayoutModel) menuBindings() []BindingResolution {
	context := m.bindingContext()
	context.Mode = ModeBrowse
	return CanonicalKeymap.MenuBindings(context)
}

func (m LayoutModel) renderActions() string {
	items := m.menuBindings()
	width, height := max(m.width-8, 1), max(m.height-8, 1)
	cursor := min(m.actions.cursor, max(len(items)-1, 0))
	visible := max(height-5, 1)
	start := max(cursor-visible+1, 0)
	end := min(start+visible, len(items))
	var lines []string
	lines = append(lines, m.styles.Title.Render(fmt.Sprintf("Actions · %s · %s", m.navState.Destination, m.focus)), "")
	for index := start; index < end; index++ {
		item := items[index]
		label := item.Binding.Description
		if item.Binding.Action == ActionBatchUpdate {
			label += fmt.Sprintf(" (%d visible)", len(m.listPane.sidebar.SelectedGames()))
		}
		if !item.Available {
			label += " (" + item.Reason + ")"
		}
		prefix, style := "  ", m.styles.Normal
		if !item.Available {
			style = m.styles.Dim
		}
		if index == cursor && !m.actions.closeFocused {
			prefix, style = "▸ ", m.styles.Selected
		}
		lines = append(lines, style.Render(ansi.Truncate(prefix+label, width, "…")))
	}
	lines = append(lines, "", renderControl("Close", m.actions.closeFocused, m.styles))
	hint := "Tab: next control"
	if !m.actions.closeFocused {
		hint += "  ↑ ↓: choose"
		if len(items) > 0 && items[cursor].Available {
			hint += "  Enter: run"
		}
	} else {
		hint += "  Enter: close"
	}
	lines = append(lines, m.styles.Dim.Render(hint))
	return strings.Join(lines, "\n")
}

func renderControl(label string, focused bool, styles *Styles) string {
	if focused {
		return styles.Selected.Render("▸ " + label)
	}
	return styles.Normal.Render("  " + label)
}
