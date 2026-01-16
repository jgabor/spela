package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HelpSection struct {
	Title    string
	Bindings []HelpBinding
}

type HelpBinding struct {
	Key         string
	Description string
}

type HelpModel struct {
	sections []HelpSection
	width    int
	height   int
}

func NewHelp() HelpModel {
	return HelpModel{
		sections: []HelpSection{
			{
				Title: "Navigation",
				Bindings: []HelpBinding{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
					{"Tab", "Switch pane"},
					{"Enter", "Select game"},
					{"Esc", "Clear/back"},
				},
			},
			{
				Title: "Sidebar filters",
				Bindings: []HelpBinding{
					{"/", "Search games"},
					{"d", "Toggle DLLs filter"},
					{"p", "Toggle profile filter"},
					{"s", "Cycle sort mode"},
					{"C", "Clear all filters"},
				},
			},
			{
				Title: "Content actions",
				Bindings: []HelpBinding{
					{"↑/k", "Previous setting"},
					{"↓/j", "Next setting"},
					{"←/h", "Decrease value"},
					{"→/l", "Increase value"},
					{"s", "Save profile"},
					{"u", "Update DLLs"},
					{"R", "Restore DLLs"},
				},
			},
			{
				Title: "General",
				Bindings: []HelpBinding{
					{"?", "Toggle help"},
					{"q", "Quit"},
					{"Ctrl+C", "Force quit"},
				},
			},
		},
	}
}

func (m *HelpModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	return m, nil
}

func (m HelpModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("76")).
		Width(10)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	var b strings.Builder

	b.WriteString(titleStyle.Render("Keyboard shortcuts"))
	b.WriteString("\n\n")

	for i, section := range m.sections {
		b.WriteString(sectionStyle.Render(section.Title))
		b.WriteString("\n")

		for _, binding := range section.Bindings {
			b.WriteString("  ")
			b.WriteString(keyStyle.Render(binding.Key))
			b.WriteString(descStyle.Render(binding.Description))
			b.WriteString("\n")
		}

		if i < len(m.sections)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press ? or Esc to close"))

	return b.String()
}

func ContextHelp(focus Focus, searchFocused bool) string {
	var hints []string

	if searchFocused {
		hints = []string{"type:filter", "enter/esc:done"}
	} else if focus == FocusSidebar {
		hints = []string{"↑↓:navigate", "/:search", "d:DLLs", "p:profile", "s:sort", "enter:select"}
	} else {
		hints = []string{"↑↓:navigate", "←→:change", "s:save", "u:update", "R:restore", "tab:sidebar"}
	}

	hints = append(hints, "?:help", "q:quit")
	return strings.Join(hints, " • ")
}
