package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jgabor/spela/internal/game"
)

type DLLStatusModel struct {
	game   *game.Game
	cursor int
}

func NewDLLStatus(g *game.Game) DLLStatusModel {
	return DLLStatusModel{
		game: g,
	}
}

func (m DLLStatusModel) Init() tea.Cmd {
	return nil
}

func (m DLLStatusModel) Update(msg tea.Msg) (DLLStatusModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.game.DLLs)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m DLLStatusModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("DLLs: %s", m.game.Name)))
	b.WriteString("\n\n")

	if len(m.game.DLLs) == 0 {
		b.WriteString(dimStyle.Render("No DLLs detected"))
		b.WriteString("\n")
	} else {
		for i, dll := range m.game.DLLs {
			cursor := "  "
			style := normalStyle
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}

			version := dll.Version
			if version == "" {
				version = "unknown"
			}

			line := fmt.Sprintf("%s%-24s %s", cursor, dll.Name, version)
			b.WriteString(style.Render(line))
			b.WriteString("\n")

			if i == m.cursor {
				b.WriteString(dimStyle.Render(fmt.Sprintf("      %s", dll.Path)))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString(helpStyle.Render("\n\n↑/↓ navigate • esc back • q quit"))

	return b.String()
}
