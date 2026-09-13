package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/nav"
)

// ListPaneModel owns destination-local collection state. It is one of the two
// focusable shell panes; destination navigation itself is rendered separately
// and never receives focus.
type ListPaneModel struct {
	styles   *Styles
	navState *nav.State
	sidebar  SidebarModel
	cursor   int
	width    int
	height   int
}

func NewListPane(styles *Styles, sidebar SidebarModel, navState *nav.State) ListPaneModel {
	return ListPaneModel{styles: styles, navState: navState, sidebar: sidebar}
}

func (m *ListPaneModel) SetState(state nav.State) {
	if m.navState != nil {
		*m.navState = state
	}
	m.syncCursorFromState()
}

func (m ListPaneModel) State() nav.State {
	if m.navState == nil {
		return nav.DefaultState()
	}
	return *m.navState
}

func (m *ListPaneModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.sidebar.SetSize(max(width-2, 1), height)
}

func (m ListPaneModel) Update(message tea.Msg) (ListPaneModel, tea.Cmd, bool) {
	key, ok := message.(tea.KeyPressMsg)
	if !ok {
		return m, nil, false
	}
	if m.State().Destination == nav.DestinationLibrary {
		sidebar, command := m.sidebar.Update(key)
		m.sidebar = sidebar
		// Search input is handled even when textinput has no command.
		return m, command, m.sidebar.search.Focused() || command != nil
	}
	return m.updateSectionList(key, len(sectionLabels(m.State().Destination)))
}

func (m ListPaneModel) updateSectionList(key tea.KeyPressMsg, count int) (ListPaneModel, tea.Cmd, bool) {
	if count == 0 {
		return m, nil, false
	}
	switch key.String() {
	case "down":
		m.cursor = min(m.cursor+1, count-1)
	case "up":
		m.cursor = max(m.cursor-1, 0)
	default:
		return m, nil, false
	}
	m.applySectionCursor()
	return m, nil, true
}

func (m *ListPaneModel) applySectionCursor() {
	if m.navState == nil {
		return
	}
	switch m.navState.Destination {
	case nav.DestinationDLLCatalog:
		m.navState.DLLCatalogSection = nav.DLLCatalogSection(m.cursor)
	case nav.DestinationMonitor:
		m.navState.MonitorSection = nav.MonitorSection(m.cursor)
	case nav.DestinationSettings:
		m.navState.SettingsSection = nav.SettingsSection(m.cursor)
	}
}

func (m *ListPaneModel) syncCursorFromState() {
	if m.navState == nil {
		m.cursor = 0
		return
	}
	switch m.navState.Destination {
	case nav.DestinationDLLCatalog:
		m.cursor = int(m.navState.DLLCatalogSection)
	case nav.DestinationMonitor:
		m.cursor = int(m.navState.MonitorSection)
	case nav.DestinationSettings:
		m.cursor = int(m.navState.SettingsSection)
	default:
		m.cursor = 0
	}
}

func (m ListPaneModel) View(focused bool) string {
	if m.State().Destination == nav.DestinationLibrary {
		return m.sidebar.View()
	}
	labels := sectionLabels(m.State().Destination)
	var builder strings.Builder
	for index, label := range labels {
		style := m.styles.Dim
		prefix := "  "
		if index == m.cursor {
			if focused {
				style = m.styles.FocusStyle()
				prefix = "▸ "
			} else {
				style = lipgloss.NewStyle().Foreground(m.styles.Theme.Fg)
			}
		}
		builder.WriteString(style.Render(prefix + label))
		builder.WriteString("\n")
	}
	if len(labels) == 0 {
		return m.styles.Dim.Render("No items")
	}
	return builder.String()
}

func sectionLabels(destination nav.Destination) []string {
	switch destination {
	case nav.DestinationDLLCatalog:
		return nav.DLLCatalogSectionLabels
	case nav.DestinationMonitor:
		return nav.MonitorSectionLabels
	case nav.DestinationSettings:
		return nav.SettingsSectionLabels
	default:
		return nil
	}
}
