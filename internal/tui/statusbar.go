package tui

import (
	"github.com/charmbracelet/lipgloss"
)

type StatusBarModel struct {
	width int
}

func NewStatusBar() StatusBarModel {
	return StatusBarModel{}
}

func (m *StatusBarModel) SetWidth(width int) {
	m.width = width
}

func (m StatusBarModel) View() string {
	return m.ViewWithHelp("tab:switch • ?:help • q:quit")
}

func (m StatusBarModel) ViewWithHelp(contextHelp string) string {
	t := GetTheme()
	style := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Width(m.width).
		Padding(0, 1)

	return style.Render(contextHelp)
}
