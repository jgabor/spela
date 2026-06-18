package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/nav"
)

// ContextNavModel renders the middle column (scope, aspects, or sections).
type ContextNavModel struct {
	styles   *Styles
	navState *nav.State
	sidebar  SidebarModel
	cursor   int
	width    int
	height   int
}

func NewContextNav(styles *Styles, sidebar SidebarModel, navState *nav.State) ContextNavModel {
	return ContextNavModel{
		styles:   styles,
		navState: navState,
		sidebar:  sidebar,
	}
}

func (m *ContextNavModel) SetState(s nav.State) {
	if m.navState != nil {
		*m.navState = s
	}
	m.syncCursorFromState()
}

func (m ContextNavModel) State() nav.State {
	if m.navState == nil {
		return nav.DefaultState()
	}
	return *m.navState
}

func (m *ContextNavModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.sidebar.SetSize(width-4, height)
}

func (m ContextNavModel) Update(msg tea.Msg) (ContextNavModel, tea.Cmd, bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil, false
	}

	switch m.State().Destination {
	case nav.DestinationLibrary:
		return m.updateLibrary(key)
	case nav.DestinationDLLCatalog:
		return m.updateSectionList(key, len(nav.DLLCatalogSectionLabels))
	case nav.DestinationMonitor:
		return m.updateSectionList(key, len(nav.MonitorSectionLabels))
	case nav.DestinationSettings:
		return m.updateSectionList(key, len(nav.SettingsSectionLabels))
	}
	return m, nil, false
}

func (m ContextNavModel) updateLibrary(key tea.KeyPressMsg) (ContextNavModel, tea.Cmd, bool) {
	if m.navState == nil {
		return m, nil, false
	}

	switch key.String() {
	case "1":
		next := m.State().SelectAspect(nav.AspectOverview)
		if m.navState.Scope.Kind == nav.ScopeGlobal {
			return m, nil, false
		}
		*m.navState = next
		return m, nil, true
	case "2":
		*m.navState = m.State().SelectAspect(nav.AspectProfile)
		return m, nil, true
	case "3":
		if m.navState.Scope.Kind == nav.ScopeGame {
			*m.navState = m.State().SelectAspect(nav.AspectDLLs)
			return m, nil, true
		}
		return m, nil, false
	}

	if m.navState.Aspect == nav.AspectProfile {
		switch key.String() {
		case "j", "down":
			if int(m.navState.ProfileSubsystem) < len(nav.ProfileSubsystemLabels)-1 {
				m.navState.ProfileSubsystem++
			}
			return m, nil, true
		case "k", "up":
			if m.navState.ProfileSubsystem > 0 {
				m.navState.ProfileSubsystem--
			}
			return m, nil, true
		}
	}

	// Scope list via sidebar (includes pinned default row).
	sidebar, cmd := m.sidebar.Update(key)
	m.sidebar = sidebar
	return m, cmd, cmd != nil
}

func (m ContextNavModel) updateSectionList(key tea.KeyPressMsg, count int) (ContextNavModel, tea.Cmd, bool) {
	switch key.String() {
	case "j", "down":
		m.cursor = min(m.cursor+1, count-1)
		m.applySectionCursor()
		return m, nil, true
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
		m.applySectionCursor()
		return m, nil, true
	}
	return m, nil, false
}

func (m *ContextNavModel) applySectionCursor() {
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

func (m *ContextNavModel) syncCursorFromState() {
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

// View renders the context column for the current destination.
func (m ContextNavModel) View(focused bool) string {
	s := m.styles
	var b strings.Builder

	title := contextTitle(m.State())
	titleStyle := lipgloss.NewStyle().Foreground(s.Theme.AccentOverride).Bold(true)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	switch m.State().Destination {
	case nav.DestinationLibrary:
		b.WriteString(m.renderLibraryContext(focused))
	default:
		b.WriteString(m.renderSectionList(focused))
	}

	return b.String()
}

func contextTitle(s nav.State) string {
	switch s.Destination {
	case nav.DestinationLibrary:
		return "Scope"
	case nav.DestinationDLLCatalog, nav.DestinationMonitor, nav.DestinationSettings:
		return "Sections"
	default:
		return "Context"
	}
}

func (m ContextNavModel) renderLibraryContext(focused bool) string {
	var b strings.Builder
	b.WriteString(m.sidebar.View())
	state := m.State()
	if state.Scope.Kind == nav.ScopeGame {
		b.WriteString("\n")
		b.WriteString(m.renderAspectTabs(focused))
	}
	if state.Aspect == nav.AspectProfile {
		b.WriteString("\n")
		b.WriteString(m.renderSubsystemList(focused))
	}
	return b.String()
}

func (m ContextNavModel) renderAspectTabs(focused bool) string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.Dim.Render("Aspect"))
	b.WriteString("\n")
	for i, label := range nav.AspectLabels {
		aspect := nav.Aspect(i)
		state := m.State()
		if state.Scope.Kind == nav.ScopeGlobal && aspect != nav.AspectProfile {
			continue
		}
		style := s.Dim
		if aspect == state.Aspect {
			style = lipgloss.NewStyle().Foreground(s.Theme.Accent)
		}
		prefix := fmt.Sprintf("%d ", i+1)
		b.WriteString(style.Render(prefix + label))
		b.WriteString("\n")
	}
	return b.String()
}

func (m ContextNavModel) renderSubsystemList(focused bool) string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.Dim.Render("Subsystem"))
	b.WriteString("\n")
	for i, label := range nav.ProfileSubsystemLabels {
		style := s.Dim
		if nav.ProfileSubsystem(i) == m.State().ProfileSubsystem {
			if focused {
				style = s.FocusStyle()
			} else {
				style = lipgloss.NewStyle().Foreground(s.Theme.Fg)
			}
		}
		b.WriteString(style.Render("  " + label))
		b.WriteString("\n")
	}
	return b.String()
}

func (m ContextNavModel) renderSectionList(focused bool) string {
	s := m.styles
	labels := sectionLabels(m.State().Destination)
	var b strings.Builder
	for i, label := range labels {
		style := s.Dim
		if i == m.cursor {
			if focused {
				style = s.FocusStyle()
			} else {
				style = lipgloss.NewStyle().Foreground(s.Theme.Fg)
			}
		}
		b.WriteString(style.Render("  " + label))
		b.WriteString("\n")
	}
	return b.String()
}

func sectionLabels(d nav.Destination) []string {
	switch d {
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
