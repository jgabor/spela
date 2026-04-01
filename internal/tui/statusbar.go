package tui

import (
	"charm.land/lipgloss/v2"
)

type StatusBarModel struct {
	styles *Styles
	width  int
}

func NewStatusBar(styles *Styles) StatusBarModel {
	return StatusBarModel{styles: styles}
}

func (m *StatusBarModel) SetWidth(width int) {
	m.width = width
}

func (m StatusBarModel) View() string {
	return m.ViewWithHelp("tab:switch • ?:help • q:quit")
}

func (m StatusBarModel) ViewWithHelp(contextHelp string) string {
	t := m.styles.Theme
	style := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Width(m.width).
		Padding(0, 1)

	return style.Render(contextHelp)
}
