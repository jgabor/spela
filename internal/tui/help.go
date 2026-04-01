package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
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
	styles   *Styles
	sections []HelpSection
	width    int
	height   int
}

func NewHelp(styles *Styles) HelpModel {
	return HelpModel{
		styles: styles,
		sections: []HelpSection{
			{
				Title: "Navigation",
				Bindings: []HelpBinding{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
					{"Tab", "Switch pane"},
					{"Enter", "Select"},
					{"Esc", "Clear/back"},
				},
			},
			{
				Title: "Sidebar filters",
				Bindings: []HelpBinding{
					{"/, Ctrl+F", "Search games"},
					{"d", "Toggle DLLs filter"},
					{"p", "Toggle profile filter"},
					{"s", "Cycle sort mode"},
					{"C", "Clear all filters"},
					{"r", "Rescan games"},
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
					{"L", "Launch game"},
					{"i", "Install DLL"},
					{"u", "Update DLLs"},
					{"R", "Restore DLLs"},
				},
			},
			{
				Title: "Batch operations",
				Bindings: []HelpBinding{
					{"Space", "Enter multi-select / toggle selection"},
					{"a", "Select all visible"},
					{"A", "Deselect all"},
					{"Esc", "Exit multi-select"},
					{"Enter", "Execute batch action"},
				},
			},
			{
				Title: "Indicators",
				Bindings: []HelpBinding{
					{"●", "Game has DLLs"},
					{"◆", "Game has profile"},
				},
			},
			{
				Title: "General",
				Bindings: []HelpBinding{
					{"?", "Toggle help"},
					{"o", "Options"},
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
	s := m.styles
	t := s.Theme

	helpTitleStyle := s.Title.Foreground(t.Primary).MarginBottom(1)

	sectionStyle := s.Normal.
		Foreground(t.Secondary).
		Bold(true)

	keyStyle := s.Normal.
		Foreground(t.Accent).
		Width(10)

	descStyle := s.Normal.
		Foreground(t.Text)

	var b strings.Builder

	b.WriteString(helpTitleStyle.Render("Keyboard shortcuts"))
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
	b.WriteString(s.Dim.Render("Press ? or Esc to close"))

	return b.String()
}

func ContextHelp(focus Focus, searchFocused, selectMode, hasGameSelection bool, showHints bool) string {
	if !showHints {
		return "?:help • q:quit"
	}

	var hints []string

	if searchFocused {
		hints = []string{"type:filter", "enter/esc:done"}
	} else if selectMode {
		hints = []string{"↑↓:navigate", "space:toggle", "a:all", "A:none", "enter:batch", "esc:exit"}
	} else if focus == FocusSidebar {
		hints = []string{"↑↓:navigate", "/:search", "d:DLLs", "p:profile", "s:sort", "r:rescan", "enter:select"}
	} else {
		hints = []string{"↑↓:navigate", "←→:change", "s:save"}
		if hasGameSelection {
			hints = append(hints, "L:launch", "i:install", "u:update", "R:restore")
		}
		hints = append(hints, "tab:sidebar")
	}

	hints = append(hints, "?:help", "o:options", "q:quit")
	return strings.Join(hints, " • ")
}
