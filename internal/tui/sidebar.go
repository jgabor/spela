package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type SortMode int

const (
	SortNameAsc SortMode = iota
	SortNameDesc
	SortDLLsFirst
	SortProfileFirst
)

var sortModeNames = []string{"A-Z", "Z-A", "DLLs", "Profile"}

type FilterState struct {
	hasDLLs    bool
	hasProfile bool
}

func (f FilterState) IsActive() bool {
	return f.hasDLLs || f.hasProfile
}

type SidebarModel struct {
	games      []*game.Game
	filtered   []*game.Game
	cursor     int
	search     textinput.Model
	filters    FilterState
	sortMode   SortMode
	width      int
	height     int
	selected   map[uint64]bool
	selectMode bool
}

func NewSidebar(games []*game.Game) SidebarModel {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 30
	ti.Width = 20

	m := SidebarModel{
		games:    games,
		search:   ti,
		sortMode: SortNameAsc,
		selected: make(map[uint64]bool),
	}
	m.applyFiltersAndSort()
	return m
}

func (m *SidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.search.Width = width - 4
}

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.search.Focused() {
			switch msg.String() {
			case "enter", "esc":
				m.search.Blur()
			default:
				m.search, cmd = m.search.Update(msg)
				m.applyFiltersAndSort()
			}
			return m, cmd
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if !m.selectMode {
					return m, m.selectCurrentGame()
				}
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if !m.selectMode {
					return m, m.selectCurrentGame()
				}
			}
		case "/":
			m.search.Focus()
			return m, textinput.Blink
		case "d":
			m.filters.hasDLLs = !m.filters.hasDLLs
			m.applyFiltersAndSort()
		case "p":
			m.filters.hasProfile = !m.filters.hasProfile
			m.applyFiltersAndSort()
		case "s":
			m.sortMode = (m.sortMode + 1) % 4
			m.applyFiltersAndSort()
		case "C":
			m.clearFilters()
		case "v":
			m.selectMode = !m.selectMode
			if !m.selectMode {
				m.selected = make(map[uint64]bool)
			}
		case " ":
			if m.selectMode && m.cursor < len(m.filtered) {
				g := m.filtered[m.cursor]
				if m.selected[g.AppID] {
					delete(m.selected, g.AppID)
				} else {
					m.selected[g.AppID] = true
				}
			}
		case "a":
			if m.selectMode {
				for _, g := range m.filtered {
					m.selected[g.AppID] = true
				}
			}
		case "A":
			if m.selectMode {
				m.selected = make(map[uint64]bool)
			}
		case "esc":
			if m.selectMode {
				m.selectMode = false
				m.selected = make(map[uint64]bool)
			} else if m.search.Value() != "" {
				m.search.SetValue("")
				m.applyFiltersAndSort()
			} else if m.filters.IsActive() {
				m.clearFilters()
			}
		case "enter":
			if m.selectMode && len(m.selected) > 0 {
				return m, func() tea.Msg {
					return batchActionRequestMsg{selected: m.SelectedGames()}
				}
			}
			if selected := m.Selected(); selected != nil {
				return m, func() tea.Msg {
					return gameSelectedMsg{game: selected}
				}
			}
		}
	}

	return m, nil
}

func (m *SidebarModel) clearFilters() {
	m.filters = FilterState{}
	m.search.SetValue("")
	m.applyFiltersAndSort()
}

func (m *SidebarModel) applyFiltersAndSort() {
	query := strings.ToLower(m.search.Value())

	var filtered []*game.Game
	for _, g := range m.games {
		if query != "" && !strings.Contains(strings.ToLower(g.Name), query) {
			continue
		}
		if m.filters.hasDLLs && len(g.DLLs) == 0 {
			continue
		}
		if m.filters.hasProfile && !profile.Exists(g.AppID) {
			continue
		}
		filtered = append(filtered, g)
	}

	switch m.sortMode {
	case SortNameAsc:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Name < filtered[j].Name
		})
	case SortNameDesc:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Name > filtered[j].Name
		})
	case SortDLLsFirst:
		sort.Slice(filtered, func(i, j int) bool {
			iHas := len(filtered[i].DLLs) > 0
			jHas := len(filtered[j].DLLs) > 0
			if iHas != jHas {
				return iHas
			}
			return filtered[i].Name < filtered[j].Name
		})
	case SortProfileFirst:
		sort.Slice(filtered, func(i, j int) bool {
			iHas := profile.Exists(filtered[i].AppID)
			jHas := profile.Exists(filtered[j].AppID)
			if iHas != jHas {
				return iHas
			}
			return filtered[i].Name < filtered[j].Name
		})
	}

	m.filtered = filtered
	if m.cursor >= len(filtered) {
		m.cursor = 0
	}
}

func (m SidebarModel) View() string {
	var b strings.Builder

	titleLine := "Games"
	if m.selectMode {
		titleLine = fmt.Sprintf("Select (%d)", len(m.selected))
	} else if m.sortMode != SortNameAsc {
		titleLine += " [" + sortModeNames[m.sortMode] + "]"
	}
	b.WriteString(titleStyle.Render(titleLine))
	b.WriteString("\n")

	if m.filters.IsActive() {
		var activeFilters []string
		if m.filters.hasDLLs {
			activeFilters = append(activeFilters, "●DLLs")
		}
		if m.filters.hasProfile {
			activeFilters = append(activeFilters, "◆Profile")
		}
		b.WriteString(dlssStyle.Render(strings.Join(activeFilters, " ")))
		b.WriteString("\n")
	}

	if m.search.Focused() || m.search.Value() != "" {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("No games found"))
		return b.String()
	}

	headerLines := 2
	if m.filters.IsActive() {
		headerLines++
	}
	if m.search.Focused() || m.search.Value() != "" {
		headerLines++
	}

	visibleCount := max(m.height-headerLines-2, 3)

	start := 0
	if m.cursor >= visibleCount {
		start = m.cursor - visibleCount + 1
	}
	end := min(start+visibleCount, len(m.filtered))

	maxNameWidth := max(m.width-10, 10)

	for i := start; i < end; i++ {
		g := m.filtered[i]
		cursor := "  "
		style := normalStyle

		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		checkbox := ""
		if m.selectMode {
			if m.selected[g.AppID] {
				checkbox = "[✓] "
			} else {
				checkbox = "[ ] "
			}
		}

		name := g.Name
		effectiveMaxWidth := maxNameWidth
		if m.selectMode {
			effectiveMaxWidth -= 4
		}
		if len(name) > effectiveMaxWidth {
			name = name[:effectiveMaxWidth-3] + "..."
		}

		line := fmt.Sprintf("%s%s%s", cursor, checkbox, name)

		indicators := ""
		if len(g.DLLs) > 0 {
			indicators += " ●"
		}
		if profile.Exists(g.AppID) {
			indicators += " ◆"
		}

		b.WriteString(style.Render(line))
		if indicators != "" {
			b.WriteString(dlssStyle.Render(indicators))
		}
		b.WriteString("\n")
	}

	if len(m.filtered) > visibleCount {
		scrollInfo := fmt.Sprintf(" %d/%d", m.cursor+1, len(m.filtered))
		b.WriteString(dimStyle.Render(scrollInfo))
	}

	return b.String()
}

func (m SidebarModel) Selected() *game.Game {
	if m.cursor < len(m.filtered) {
		return m.filtered[m.cursor]
	}
	return nil
}

func (m SidebarModel) selectCurrentGame() tea.Cmd {
	if g := m.Selected(); g != nil {
		return func() tea.Msg {
			return gameSelectedMsg{game: g}
		}
	}
	return nil
}

func (m SidebarModel) SelectedGames() []*game.Game {
	var games []*game.Game
	for _, g := range m.games {
		if m.selected[g.AppID] {
			games = append(games, g)
		}
	}
	return games
}

func (m SidebarModel) SelectionCount() int {
	return len(m.selected)
}

func (m SidebarModel) InSelectMode() bool {
	return m.selectMode
}

type batchActionRequestMsg struct {
	selected []*game.Game
}
