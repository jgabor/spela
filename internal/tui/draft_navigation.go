package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jgabor/spela/internal/nav"
)

type decisionDialog struct {
	title           string
	body            string
	choices         []string
	focus           int // -1 is the scrollable body; zero is always Cancel.
	offset          int
	saving          bool
	error           string
	includeSettings bool
	guard           bool
	apply           func(LayoutModel) (LayoutModel, tea.Cmd)
}

func (m LayoutModel) profileDirty() bool {
	return m.pane.defaultsDetail.Dirty() || m.pane.content.game != nil && m.pane.content.detail.Dirty()
}

func (m LayoutModel) dirtyDocumentNames(settings bool) []string {
	var names []string
	if m.pane.defaultsDetail.Dirty() {
		names = append(names, "All games (default profile)")
	}
	if m.pane.content.game != nil && m.pane.content.detail.Dirty() {
		names = append(names, m.pane.content.game.Name+" profile")
	}
	if settings && m.pane.settings.Dirty() {
		names = append(names, "Settings")
	}
	return names
}

func (m LayoutModel) guardDrafts(settings bool, apply func(LayoutModel) (LayoutModel, tea.Cmd)) LayoutModel {
	m.decision = &decisionDialog{title: "Unsaved changes", body: "Continue with changes to:\n" + strings.Join(m.dirtyDocumentNames(settings), "\n"), choices: []string{"Cancel", "Save and continue", "Discard and continue"}, includeSettings: settings, guard: true, apply: apply}
	return m
}

func sameSidebarScope(sidebar SidebarModel, state nav.State) bool {
	item := sidebar.SelectedItem()
	if item == nil {
		return state.Scope.Kind == nav.ScopeGame && state.Scope.AppID == 0
	}
	if item.kind == sidebarItemDefaultProfile {
		return state.Scope.Kind == nav.ScopeGlobal
	}
	return item.game != nil && state.Scope.Kind == nav.ScopeGame && item.game.AppID == state.Scope.AppID
}

func (m LayoutModel) requestSidebar(next SidebarModel, open bool) (LayoutModel, tea.Cmd) {
	pending := m.pane.content.profileSaves != nil && m.pane.content.profileSaves.active != nil
	if !next.selectMode && !sameSidebarScope(next, *m.navState) && (m.profileDirty() || pending) {
		guarded := m.guardDrafts(false, func(model LayoutModel) (LayoutModel, tea.Cmd) { return model.applySidebar(next, open) })
		if pending {
			guarded.decision.saving = true
		}
		return guarded, nil
	}
	return m.applySidebar(next, open)
}

func (m LayoutModel) applySidebar(next SidebarModel, open bool) (LayoutModel, tea.Cmd) {
	unchanged := sameSidebarScope(next, *m.navState)
	m.listPane.sidebar = next
	if next.selectMode {
		return m, nil
	}
	item := next.SelectedItem()
	if unchanged && !open {
		if item != nil && item.game != nil {
			m.pane.content.game = item.game
		}
		return m, nil
	}
	var command tea.Cmd
	if item == nil {
		m.pane.loadNoScope()
	} else if item.kind == sidebarItemDefaultProfile {
		m.pane.loadGlobalScope()
	} else {
		m.pane.loadGameScope(item.game)
		command = m.pane.content.LoadDLLUpdates()
	}
	if open {
		m.focus = FocusDetail
	}
	m.syncNavToComponents()
	return m, command
}

func (m LayoutModel) requestQuit() (LayoutModel, tea.Cmd) {
	if m.operationsPending() {
		return m, nil
	}
	quit := func(model LayoutModel) (LayoutModel, tea.Cmd) { return model, tea.Quit }
	if len(m.dirtyDocumentNames(true)) > 0 {
		return m.guardDrafts(true, quit), nil
	}
	return quit(m)
}

func (m LayoutModel) confirmDraftAction(action KeyAction) LayoutModel {
	title := "Discard draft"
	if action == ActionDetailResetAll {
		title = "Reset all fields"
	}
	name := "Settings"
	if m.navState.Destination == nav.DestinationLibrary {
		name = "All games (default profile)"
		if m.navState.Scope.Kind == nav.ScopeGame && m.pane.content.game != nil {
			name = m.pane.content.game.Name + " profile"
		}
	}
	m.decision = &decisionDialog{title: title, body: name + "\nThis changes the draft. Save persists it.", choices: []string{"Cancel", title}, apply: func(model LayoutModel) (LayoutModel, tea.Cmd) { return model.updatePaneAction(action) }}
	return m
}

func (m LayoutModel) handleDecisionAction(action KeyAction) (LayoutModel, tea.Cmd) {
	dialog := m.decision
	if dialog.saving {
		return m, nil
	}
	switch action {
	case ActionOverlayFocusNext:
		dialog.focus++
		if dialog.focus >= len(dialog.choices) {
			dialog.focus = -1
		}
	case ActionOverlayLeft:
		if dialog.focus >= 0 {
			dialog.focus = (dialog.focus + len(dialog.choices) - 1) % len(dialog.choices)
		}
	case ActionOverlayRight:
		if dialog.focus >= 0 {
			dialog.focus = (dialog.focus + 1) % len(dialog.choices)
		}
	case ActionOverlayPrevious:
		if dialog.focus < 0 {
			dialog.offset = max(dialog.offset-1, 0)
		}
	case ActionOverlayNext:
		if dialog.focus < 0 {
			dialog.offset++
		}
	case ActionOverlayConfirm:
		if dialog.focus == 0 {
			m.decision = nil
			return m, nil
		}
		if dialog.focus < 0 {
			return m, nil
		}
		if !dialog.guard {
			m.decision = nil
			return dialog.apply(m)
		}
		if dialog.focus == 2 {
			m.pane.defaultsDetail.CancelDraft()
			if m.pane.content.game != nil {
				m.pane.content.detail.CancelDraft()
			}
			if dialog.includeSettings {
				m.pane.settings.CancelDraft()
			}
			m.decision = nil
			return dialog.apply(m)
		}
		dialog.saving = true
		dialog.error = ""
		var commands []tea.Cmd
		if m.pane.defaultsDetail.Dirty() {
			commands = append(commands, m.pane.saveDefaultProfile())
		}
		if m.pane.content.game != nil && m.pane.content.detail.Dirty() {
			commands = append(commands, m.pane.content.saveResolvedProfile())
		}
		if dialog.includeSettings && m.pane.settings.Dirty() {
			var command tea.Cmd
			m.pane.settings, command = m.pane.settings.UpdateAction(ActionDetailSave)
			commands = append(commands, command)
		}
		return m, tea.Batch(commands...)
	}
	return m, nil
}

func (m LayoutModel) advanceDecision(message tea.Msg) (LayoutModel, tea.Cmd) {
	if m.decision == nil || !m.decision.saving {
		return m, nil
	}
	var err error
	switch message := message.(type) {
	case profileSaveMsg:
		err = message.err
	case optionsSaveErrorMsg:
		err = message.err
	}
	if err != nil {
		m.decision.error = err.Error()
	}
	if m.operationsPending() {
		return m, nil
	}
	m.decision.saving = false
	if m.decision.error != "" {
		return m, nil
	}
	if len(m.dirtyDocumentNames(m.decision.includeSettings)) > 0 {
		m.decision.error = "Changes remain unsaved. Save again or Cancel."
		return m, nil
	}
	apply := m.decision.apply
	m.decision = nil
	return apply(m)
}

func (m LayoutModel) renderDecision() string {
	dialog := m.decision
	width, visible := max(m.width-10, 1), max(m.height-13, 1)
	body := dialog.body
	if dialog.error != "" {
		body += "\nSave failed: " + dialog.error
	}
	lines := strings.Split(ansi.Hardwrap(body, width, true), "\n")
	offset := min(dialog.offset, max(len(lines)-visible, 0))
	var builder strings.Builder
	builder.WriteString(m.styles.Title.Render(dialog.title) + "\n\n")
	builder.WriteString(strings.Join(lines[offset:min(offset+visible, len(lines))], "\n"))
	if dialog.saving {
		builder.WriteString("\n\nSaving changes... Please wait.")
		return builder.String()
	}
	builder.WriteString("\n\n")
	var buttons []string
	for index, choice := range dialog.choices {
		buttons = append(buttons, renderControl(choice, index == dialog.focus, m.styles))
	}
	builder.WriteString(strings.Join(buttons, "  ") + "\n")
	if dialog.focus < 0 {
		if len(lines) > visible {
			builder.WriteString("↑ ↓: scroll  ")
		}
		builder.WriteString("Tab: next control")
	} else {
		fmt.Fprintf(&builder, "Tab: next control  ← →: choose  Enter: %s", dialog.choices[dialog.focus])
	}
	return builder.String()
}
