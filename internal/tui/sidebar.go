package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jgabor/spela/internal/game"
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

type sidebarItemKind int

const (
	sidebarItemGame sidebarItemKind = iota
	sidebarItemDefaultProfile
)

type sidebarItem struct {
	kind sidebarItemKind
	game *game.Game
}

type SidebarModel struct {
	styles     *Styles
	services   *Services
	games      []*game.Game
	filtered   []sidebarItem
	cursor     int
	search     textinput.Model
	filters    FilterState
	sortMode   SortMode
	width      int
	height     int
	selected   map[uint64]bool
	selectMode bool
}

func NewSidebar(games []*game.Game, styles *Styles, svc *Services) (SidebarModel, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 30
	ti.SetWidth(20)

	m := SidebarModel{
		styles:   styles,
		services: svc,
		games:    games,
		search:   ti,
		sortMode: SortNameAsc,
		selected: make(map[uint64]bool),
	}
	m.applyFiltersAndSort()
	return m, m.selectCurrentItem()
}

func (m *SidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.search.SetWidth(width - 4)
}

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
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
					return m, m.selectCurrentItem()
				}
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if !m.selectMode {
					return m, m.selectCurrentItem()
				}
			}
		case "/":
			cmd = m.search.Focus()
			return m, cmd
		case "d":
			m.filters.hasDLLs = !m.filters.hasDLLs
			m.applyFiltersAndSort()
		case "P":
			// `p` is reserved for the Task 5 pin-field binding; profile
			// filter was displaced to `P` (shift+p) during the Task 3
			// keymap audit. Displacement documented in the help screen.
			m.filters.hasProfile = !m.filters.hasProfile
			m.applyFiltersAndSort()
		case "s":
			m.sortMode = (m.sortMode + 1) % 4
			m.applyFiltersAndSort()
		case "C":
			m.clearFilters()
		case "space":
			if m.cursor < len(m.filtered) {
				item := m.filtered[m.cursor]
				if item.kind != sidebarItemGame || item.game == nil {
					return m, nil
				}
				if !m.selectMode {
					m.selectMode = true
					m.selected[item.game.AppID] = true
				} else {
					if m.selected[item.game.AppID] {
						delete(m.selected, item.game.AppID)
					} else {
						m.selected[item.game.AppID] = true
					}
				}
			}
		case "a":
			if m.selectMode {
				for _, item := range m.filtered {
					if item.kind == sidebarItemGame && item.game != nil {
						m.selected[item.game.AppID] = true
					}
				}
			}
		case "A":
			if m.selectMode {
				for _, item := range m.filtered {
					if item.kind == sidebarItemGame && item.game != nil {
						delete(m.selected, item.game.AppID)
					}
				}
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
			if selected := m.SelectedItem(); selected != nil {
				if selected.kind == sidebarItemDefaultProfile {
					return m, func() tea.Msg {
						return defaultProfileConfirmedMsg{}
					}
				}
				if selected.game != nil {
					return m, func() tea.Msg {
						return gameConfirmedMsg{game: selected.game}
					}
				}
			}
		}
	}

	return m, nil
}

func (m *SidebarModel) clearFilters() {
	m.filters = FilterState{}
	m.search.SetValue("")
	m.sortMode = SortNameAsc
	m.applyFiltersAndSort()
}

func (m *SidebarModel) applyFiltersAndSort() {
	// Save current game identity before filtering (#8)
	var currentAppID uint64
	var hasCurrentGame bool
	if m.cursor < len(m.filtered) {
		item := m.filtered[m.cursor]
		if item.kind == sidebarItemGame && item.game != nil {
			currentAppID = item.game.AppID
			hasCurrentGame = true
		}
	}
	oldCursor := m.cursor

	query := strings.ToLower(m.search.Value())

	var filtered []*game.Game
	for _, g := range m.games {
		if query != "" && !strings.Contains(strings.ToLower(g.Name), query) {
			continue
		}
		if m.filters.hasDLLs && len(g.DLLs) == 0 {
			continue
		}
		if m.filters.hasProfile && !m.services.ProfileExists(g.AppID) {
			continue
		}
		filtered = append(filtered, g)
	}

	// Pre-compute hasProfile map before sorting (#21)
	hasProfileMap := make(map[uint64]bool, len(filtered))
	if m.sortMode == SortProfileFirst {
		for _, g := range filtered {
			hasProfileMap[g.AppID] = m.services.ProfileExists(g.AppID)
		}
	}

	switch m.sortMode {
	case SortNameAsc:
		slices.SortStableFunc(filtered, func(a, b *game.Game) int {
			return cmp.Compare(a.Name, b.Name)
		})
	case SortNameDesc:
		slices.SortStableFunc(filtered, func(a, b *game.Game) int {
			return cmp.Compare(b.Name, a.Name)
		})
	case SortDLLsFirst:
		slices.SortStableFunc(filtered, func(a, b *game.Game) int {
			aHas, bHas := len(a.DLLs) > 0, len(b.DLLs) > 0
			if aHas != bHas {
				if aHas {
					return -1
				}
				return 1
			}
			return cmp.Compare(a.Name, b.Name)
		})
	case SortProfileFirst:
		slices.SortStableFunc(filtered, func(a, b *game.Game) int {
			aHas, bHas := hasProfileMap[a.AppID], hasProfileMap[b.AppID]
			if aHas != bHas {
				if aHas {
					return -1
				}
				return 1
			}
			return cmp.Compare(a.Name, b.Name)
		})
	}

	// The default profile used to surface as the first sidebar item; with
	// Task 3's rail redesign it moved to the Defaults rail resource, so
	// the games sidebar is now games-only.
	items := make([]sidebarItem, 0, len(filtered))
	for _, g := range filtered {
		items = append(items, sidebarItem{kind: sidebarItemGame, game: g})
	}

	m.filtered = items

	// Restore cursor position (#8): find current game in new list, or clamp
	if hasCurrentGame {
		for i, item := range items {
			if item.kind == sidebarItemGame && item.game != nil && item.game.AppID == currentAppID {
				m.cursor = i
				return
			}
		}
	}
	if len(items) == 0 {
		m.cursor = 0
	} else {
		m.cursor = min(oldCursor, len(items)-1)
	}
}

func (m SidebarModel) View() string {
	s := m.styles
	var b strings.Builder

	titleLine := "Games"
	if m.selectMode {
		titleLine = fmt.Sprintf("Select (%d)", len(m.selected))
	} else if m.sortMode != SortNameAsc {
		titleLine += " [" + sortModeNames[m.sortMode] + "]"
	}
	b.WriteString(s.Title.Render(titleLine))
	b.WriteString("\n")

	if m.filters.IsActive() {
		var activeFilters []string
		if m.filters.hasDLLs {
			activeFilters = append(activeFilters, "●DLLs")
		}
		if m.filters.hasProfile {
			activeFilters = append(activeFilters, "◆Profile")
		}
		b.WriteString(s.DLSS.Render(strings.Join(activeFilters, " ")))
		b.WriteString("\n")
	}

	if m.search.Focused() || m.search.Value() != "" {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(s.Dim.Render("No games found"))
		return b.String()
	}

	headerLines := 2
	if m.filters.IsActive() {
		headerLines++
	}
	if m.search.Focused() || m.search.Value() != "" {
		headerLines++
	}

	// Footer takes 3 lines: optional scroll info, legend, and multi-select hint
	footerLines := 3
	visibleCount := max(m.height-headerLines-footerLines, 3)

	start := 0
	if m.cursor >= visibleCount {
		start = m.cursor - visibleCount + 1
	}
	end := min(start+visibleCount, len(m.filtered))

	maxNameWidth := max(m.width-10, 10)

	for i := start; i < end; i++ {
		item := m.filtered[i]
		prefix := "  "
		style := s.Normal

		if m.selectMode {
			// In select mode: checkmark for selected, cursor for current unselected
			if item.kind == sidebarItemGame && item.game != nil {
				if m.selected[item.game.AppID] {
					prefix = "✓ "
				} else if i == m.cursor {
					prefix = "> "
				}
			}
			if i == m.cursor {
				style = s.Selected
			}
		} else {
			// Normal mode: cursor for current item
			if i == m.cursor {
				prefix = "> "
				style = s.Selected
			}
		}

		name := m.itemName(item)
		name = ansi.Truncate(name, maxNameWidth, "...")

		line := fmt.Sprintf("%s%s", prefix, name)

		b.WriteString(style.Render(line))
		if indicator := m.itemIndicator(item); indicator != "" {
			b.WriteString(indicator)
		}
		b.WriteString("\n")
	}

	if len(m.filtered) > visibleCount {
		scrollInfo := fmt.Sprintf(" %d/%d", m.cursor+1, len(m.filtered))
		b.WriteString(s.Dim.Render(scrollInfo))
		b.WriteString("\n")
	}

	// Legend for status icons
	legend := s.DLSS.Render("●") + s.Dim.Render(" DLLs  ") + s.DLSS.Render("◆") + s.Dim.Render(" profile")
	b.WriteString(legend)
	b.WriteString("\n")
	if !m.selectMode && s.ShowHints {
		b.WriteString(s.Dim.Render("space:multi-select"))
	}

	return b.String()
}

func (m SidebarModel) Selected() *game.Game {
	if selected := m.SelectedItem(); selected != nil && selected.kind == sidebarItemGame {
		return selected.game
	}
	return nil
}

func (m SidebarModel) SelectedItem() *sidebarItem {
	if m.cursor < len(m.filtered) {
		return &m.filtered[m.cursor]
	}
	return nil
}

func (m SidebarModel) SetGames(games []*game.Game) SidebarModel {
	m.games = games
	m.applyFiltersAndSort()
	if m.cursor >= len(m.filtered) {
		m.cursor = max(len(m.filtered)-1, 0)
	}
	return m
}

func (m SidebarModel) selectCurrentItem() tea.Cmd {
	if selected := m.SelectedItem(); selected != nil {
		if selected.kind == sidebarItemDefaultProfile {
			return func() tea.Msg {
				return defaultProfileSelectedMsg{}
			}
		}
		if selected.game != nil {
			return func() tea.Msg {
				return gameSelectedMsg{game: selected.game}
			}
		}
	}
	return nil
}

func (m SidebarModel) SelectedGames() []*game.Game {
	var games []*game.Game
	for _, item := range m.filtered {
		if item.kind != sidebarItemGame || item.game == nil {
			continue
		}
		if m.selected[item.game.AppID] {
			games = append(games, item.game)
		}
	}
	return games
}

func (m SidebarModel) itemName(item sidebarItem) string {
	if item.kind == sidebarItemDefaultProfile {
		return "Default profile"
	}
	if item.game == nil {
		return ""
	}
	return item.game.Name
}

func (m SidebarModel) itemIndicator(item sidebarItem) string {
	if item.kind == sidebarItemDefaultProfile {
		return m.styles.Dim.Render(" Default")
	}
	if item.game == nil {
		return ""
	}

	indicators := ""
	if len(item.game.DLLs) > 0 {
		indicators += " ●"
	}
	if m.services.ProfileExists(item.game.AppID) {
		indicators += " ◆"
	}
	if indicators == "" {
		return ""
	}
	return m.styles.DLSS.Render(indicators)
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

func (m SidebarModel) FocusSearch() (SidebarModel, tea.Cmd) {
	cmd := m.search.Focus()
	return m, cmd
}
