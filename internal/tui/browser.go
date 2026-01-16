package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type BrowserModel struct {
	games    []*game.Game
	filtered []*game.Game
	cursor   int
	search   textinput.Model
	width    int
	height   int
}

func NewBrowser(games []*game.Game) BrowserModel {
	sorted := make([]*game.Game, len(games))
	copy(sorted, games)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	ti := textinput.New()
	ti.Placeholder = "Search games..."
	ti.CharLimit = 50
	ti.Width = 30

	return BrowserModel{
		games:    sorted,
		filtered: sorted,
		search:   ti,
	}
}

func (m BrowserModel) Init() tea.Cmd {
	return nil
}

func (m BrowserModel) Update(msg tea.Msg) (BrowserModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.search.Focused() {
			switch msg.String() {
			case "enter", "esc":
				m.search.Blur()
			default:
				m.search, cmd = m.search.Update(msg)
				m.filter()
			}
			return m, cmd
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "/":
			m.search.Focus()
			return m, textinput.Blink
		case "esc":
			if m.search.Value() != "" {
				m.search.SetValue("")
				m.filter()
			}
		}
	}

	return m, nil
}

func (m *BrowserModel) filter() {
	query := strings.ToLower(m.search.Value())
	if query == "" {
		m.filtered = m.games
		m.cursor = 0
		return
	}

	var filtered []*game.Game
	for _, g := range m.games {
		if strings.Contains(strings.ToLower(g.Name), query) {
			filtered = append(filtered, g)
		}
	}
	m.filtered = filtered
	m.cursor = 0
}

func (m BrowserModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Games"))
	b.WriteString("\n")

	if m.search.Focused() || m.search.Value() != "" {
		b.WriteString(m.search.View())
		b.WriteString("\n\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("No games found"))
		return b.String()
	}

	visibleCount := m.height - 8
	if visibleCount < 5 {
		visibleCount = 5
	}

	start := 0
	if m.cursor >= visibleCount {
		start = m.cursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := start; i < end; i++ {
		g := m.filtered[i]
		cursor := "  "
		style := normalStyle

		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		name := g.Name
		if len(name) > 40 {
			name = name[:37] + "..."
		}

		line := fmt.Sprintf("%s%-40s", cursor, name)

		if len(g.DLLs) > 0 {
			line += dlssStyle.Render(" [DLSS]")
		}

		hasProfile := profile.Exists(g.AppID)
		if hasProfile {
			line += dimStyle.Render(" [P]")
		}

		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n↑/↓ navigate • / search • enter select • m monitor • q quit"))

	return b.String()
}

func (m BrowserModel) Selected() *game.Game {
	if m.cursor < len(m.filtered) {
		return m.filtered[m.cursor]
	}
	return nil
}

func (m BrowserModel) SelectedStyle() lipgloss.Style {
	return selectedStyle
}
