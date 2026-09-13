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
	searching  bool
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
	// Start on a game overview instead of the global profile editor.
	if len(games) > 0 {
		m.cursor = 1
	}
	return m, m.selectCurrentItem()
}

func (m *SidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.search.SetWidth(width - 4)
}

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.search.Focused() {
		var command tea.Cmd
		m.search, command = m.search.Update(key)
		m.applyFiltersAndSort()
		return m, command
	}
	resolution := CanonicalKeymap.Lookup(BindingContext{Mode: ModeBrowse, Focus: FocusList, Destination: 0, CanMoveList: len(m.filtered) > 0, CanOpen: m.SelectedItem() != nil, CanSelect: m.Selected() != nil}, key.String())
	if !resolution.Supported || !resolution.Available || resolution.Binding.Scope != ScopeList {
		return m, nil
	}
	return m.UpdateAction(resolution.Binding.Action)
}

// UpdateAction changes only List-owned state. The shell guards any resulting
// profile owner change before publishing the new List and Detail together.
func (m SidebarModel) UpdateAction(action KeyAction) (SidebarModel, tea.Cmd) {
	switch action {
	case ActionListPrevious:
		m.cursor = max(m.cursor-1, 0)
	case ActionListNext:
		m.cursor = min(m.cursor+1, max(len(m.filtered)-1, 0))
	case ActionListToggleDLLFilter:
		m.filters.hasDLLs = !m.filters.hasDLLs
		m.applyFiltersAndSort()
	case ActionListToggleProfile:
		m.filters.hasProfile = !m.filters.hasProfile
		m.applyFiltersAndSort()
	case ActionSortNameAsc, ActionSortNameDesc, ActionSortDLLsFirst, ActionSortProfileFirst:
		m.sortMode = map[KeyAction]SortMode{ActionSortNameAsc: SortNameAsc, ActionSortNameDesc: SortNameDesc, ActionSortDLLsFirst: SortDLLsFirst, ActionSortProfileFirst: SortProfileFirst}[action]
		m.applyFiltersAndSort()
	case ActionListSort:
		m.sortMode = (m.sortMode + 1) % 4
		m.applyFiltersAndSort()
	case ActionListClearFilters:
		m.clearFilters()
	case ActionListMultiSelect:
		if item := m.Selected(); item != nil {
			if m.selected[item.AppID] {
				delete(m.selected, item.AppID)
			} else {
				m.selected[item.AppID] = true
			}
			m.selectMode = len(m.selected) > 0
			m.applyFiltersAndSort()
		}
	case ActionListSelectAll:
		for _, item := range m.filtered {
			if item.game != nil {
				m.selected[item.game.AppID] = true
			}
		}
		m.selectMode = len(m.selected) > 0
		m.applyFiltersAndSort()
	case ActionListClearSelection:
		for _, item := range m.filtered {
			if item.game != nil {
				delete(m.selected, item.game.AppID)
			}
		}
		m.selectMode = len(m.selected) > 0
		m.applyFiltersAndSort()
	case ActionListSelect, ActionBatchUpdate:
		if len(m.SelectedGames()) > 0 {
			return m, func() tea.Msg { return batchActionRequestMsg{selected: m.SelectedGames()} }
		}
		if item := m.SelectedItem(); item != nil {
			if item.kind == sidebarItemDefaultProfile {
				return m, func() tea.Msg { return defaultProfileConfirmedMsg{} }
			}
			return m, func() tea.Msg { return gameConfirmedMsg{game: item.game} }
		}
	default:
		return m, nil
	}
	if !m.selectMode {
		return m, m.selectCurrentItem()
	}
	return m, nil
}

func (m SidebarModel) cloneSelection() SidebarModel {
	selected := make(map[uint64]bool, len(m.selected))
	for id, value := range m.selected {
		selected[id] = value
	}
	m.selected = selected
	// The text input stores a rune slice. A struct copy alone would let edits
	// overwrite the query retained by a pending navigation or search decision.
	position := m.search.Position()
	m.search.SetValue(m.search.Value())
	m.search.SetCursor(position)
	return m
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

	items := make([]sidebarItem, 0, len(filtered)+1)
	if len(m.games) > 0 && query == "" && !m.filters.IsActive() && !m.selectMode {
		items = append(items, sidebarItem{kind: sidebarItemDefaultProfile})
	}
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

	titleLine := "Scope"
	if m.selectMode {
		titleLine = fmt.Sprintf("Selected: %d total, %d visible", len(m.selected), len(m.SelectedGames()))
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

	if m.searching || m.search.Focused() || m.search.Value() != "" {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	if len(m.games) == 0 && m.search.Value() == "" {
		b.WriteString("No games found\n")
		b.WriteString(s.Dim.Render("Use Actions to rescan or Settings for paths"))
		return b.String()
	}

	if len(m.filtered) == 0 {
		if query := m.search.Value(); query != "" {
			fmt.Fprintf(&b, "No games match %q\n", query)
			b.WriteString(s.Dim.Render("Use Clear search or Cancel below"))
		} else {
			b.WriteString("No games match filters\n")
			b.WriteString(s.Dim.Render("Clear filters in Actions"))
		}
		return b.String()
	}

	headerLines := 2
	if m.filters.IsActive() {
		headerLines++
	}
	if m.searching || m.search.Focused() || m.search.Value() != "" {
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
	present := make(map[uint64]bool, len(games))
	for _, item := range games {
		present[item.AppID] = true
	}
	for id := range m.selected {
		if !present[id] {
			delete(m.selected, id)
		}
	}
	m.selectMode = len(m.selected) > 0
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
	return func() tea.Msg { return noLibrarySelectionMsg{} }
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
		return "All games (default)"
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
