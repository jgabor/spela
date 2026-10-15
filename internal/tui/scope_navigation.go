package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

// decisionDialog is the confirmation shown before a change that cannot be
// undone with one key. Focus -1 is the scrollable body; zero is always Cancel.
type decisionDialog struct {
	title   string
	body    string
	choices []string
	focus   int
	offset  int
	apply   func(LayoutModel) (LayoutModel, tea.Cmd)
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

// applySidebar shows the selected scope. Every change is saved as it is made,
// so moving between games never asks anything.
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

// requestQuit quits at once, or after the writes in flight have finished.
// Other running work still blocks Quit.
func (m LayoutModel) requestQuit() (LayoutModel, tea.Cmd) {
	if m.operationsPending() {
		return m, nil
	}
	if m.savesPending() {
		m.quitWhenIdle = true
		return m, m.messageBar.SetMessage("Saving, then quitting", MessageInfo)
	}
	return m, tea.Quit
}

// confirmClearAll asks before every setting of the open profile returns to what
// it follows. The body says how many settings that is.
func (m LayoutModel) confirmClearAll() LayoutModel {
	title := profile.ClearAllAction(true)
	body := fmt.Sprintf("%s\n%s", profile.AllGamesName, profile.ClearAllBody(true, m.pane.defaultsDetail.ClearableCount()))
	if m.navState.Scope.Kind == nav.ScopeGame && m.pane.content.game != nil {
		title = profile.ClearAllAction(false)
		body = fmt.Sprintf("%s\n%s", m.pane.content.game.Name, profile.ClearAllBody(false, m.pane.content.detail.ClearableCount()))
	}
	m.decision = &decisionDialog{title: title, body: body, choices: []string{"Cancel", title}, apply: func(model LayoutModel) (LayoutModel, tea.Cmd) { return model.updatePaneAction(ActionDetailResetAll) }}
	return m
}

// ownValuesAppliedMsg reports that All games was applied to the games that set
// their own value for a field.
type ownValuesAppliedMsg struct {
	label   string
	outcome profile.FollowOutcome
}

// confirmApplyOwnValues asks before the focused All games field is applied to
// the games that set their own value for it. The dialog names those games and
// the value they will follow; confirming clears their own value.
func (m LayoutModel) confirmApplyOwnValues() LayoutModel {
	detail := m.pane.defaultsDetail
	field := detail.FocusedField()
	descriptor, ok := profile.Field(field)
	entries := m.pane.ownValueEntries()
	names := profile.CollectOwnValueGames(entries)[field]
	if !ok || len(names) == 0 {
		return m
	}
	value, _, _ := detail.fieldText(field)
	title := profile.ActionApplyToGamesWithOwnValue
	body := fmt.Sprintf("%s\n%s", profile.AllGamesName, profile.ApplyToOwnValueBody(descriptor.Label, value, names))
	m.decision = &decisionDialog{title: title, body: body, choices: []string{"Cancel", title}, apply: func(model LayoutModel) (LayoutModel, tea.Cmd) {
		model.applyingOwnValues = true
		follow := model.services.followAllGames
		return model, func() tea.Msg {
			return ownValuesAppliedMsg{label: descriptor.Label, outcome: follow(entries, []string{field})}
		}
	}}
	return m
}

func (m LayoutModel) handleDecisionAction(action KeyAction) (LayoutModel, tea.Cmd) {
	dialog := m.decision
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
		if dialog.focus < 0 {
			return m, nil
		}
		m.decision = nil
		if dialog.focus == 0 {
			return m, nil
		}
		return dialog.apply(m)
	}
	return m, nil
}

func (m LayoutModel) renderDecision() string {
	dialog := m.decision
	width, visible := max(m.width-10, 1), max(m.height-13, 1)
	lines := strings.Split(ansi.Wrap(dialog.body, width, ""), "\n")
	offset := min(dialog.offset, max(len(lines)-visible, 0))
	var builder strings.Builder
	builder.WriteString(m.styles.Title.Render(dialog.title) + "\n\n")
	builder.WriteString(strings.Join(lines[offset:min(offset+visible, len(lines))], "\n"))
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
