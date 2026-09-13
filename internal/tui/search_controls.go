package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type searchSession struct {
	original SidebarModel
	control  int
}

func (m LayoutModel) startSearch() (LayoutModel, tea.Cmd) {
	m.searchSession = &searchSession{original: m.listPane.sidebar.cloneSelection()}
	m.inputMode = ModeSearch
	m.focus = FocusList
	m.listPane.sidebar.searching = true
	command := m.listPane.sidebar.search.Focus()
	return m, command
}

func (m LayoutModel) searchEnterLabel() string {
	if m.searchSession == nil {
		return "Apply search"
	}
	return []string{"Apply search", "Apply search", "Clear search", "Cancel search"}[m.searchSession.control]
}

func (m LayoutModel) handleSearchKey(key tea.KeyPressMsg, action KeyAction) (LayoutModel, tea.Cmd) {
	session := m.searchSession
	if session == nil {
		return m, nil
	}
	if action == ActionFocusNext {
		session.control = (session.control + 1) % 4
		if session.control == 0 {
			return m, m.listPane.sidebar.search.Focus()
		}
		m.listPane.sidebar.search.Blur()
		return m, nil
	}
	if action == ActionSearchAccept {
		next := m.listPane.sidebar
		if session.control == 2 {
			next.search.SetValue("")
			next.applyFiltersAndSort()
		}
		if session.control == 3 {
			next = session.original
		}
		next.search.Blur()
		next.searching = false
		// A guarded restoration also keeps Search open until the choice succeeds.
		if !sameSidebarScope(next, *m.navState) && m.profileDirty() {
			return m.guardDrafts(false, func(model LayoutModel) (LayoutModel, tea.Cmd) {
				model.searchSession = nil
				model.inputMode = ModeBrowse
				return model.applySidebar(next, false)
			}), nil
		}
		m.searchSession = nil
		m.inputMode = ModeBrowse
		return m.applySidebar(next, false)
	}
	if session.control != 0 {
		return m, nil
	}
	next := m.listPane.sidebar.cloneSelection()
	var command tea.Cmd
	next.search, command = next.search.Update(key)
	next.applyFiltersAndSort()
	updated, selectionCommand := m.requestSidebar(next, false)
	return updated, tea.Batch(command, selectionCommand)
}

func (m LayoutModel) renderSearchControls() string {
	var controls []string
	for index, label := range []string{"Input", "Apply", "Clear search", "Cancel"} {
		controls = append(controls, renderControl(label, index == m.searchSession.control, m.styles))
	}
	return strings.Join(controls, "  ")
}
