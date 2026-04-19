package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Resource identifies which of the four peer resource surfaces the right
// pane currently displays. The rail is the permanent left-side navigator
// that selects among them (per .agentera/DECISIONS.md Decision 1).
type Resource int

const (
	ResourceGames Resource = iota
	ResourceDLLs
	ResourceDefaults
	ResourceMetrics
)

// resourceEntry is one row on the rail.
type resourceEntry struct {
	resource Resource
	hotkey   string // "1".."4" — also the display glyph
	label    string
}

// railEntries is the ordered list of rail rows. Order is load-bearing:
// hotkeys 1-4 map to index 0-3 by construction, and the acceptance tests
// assert exactly this order.
var railEntries = []resourceEntry{
	{resource: ResourceGames, hotkey: "1", label: "Games"},
	{resource: ResourceDLLs, hotkey: "2", label: "DLLs"},
	{resource: ResourceDefaults, hotkey: "3", label: "Defaults"},
	{resource: ResourceMetrics, hotkey: "4", label: "Metrics"},
}

// String returns the display label for a resource. Kept tiny so test
// assertions can read "Games" etc. rather than integer literals.
func (r Resource) String() string {
	for _, e := range railEntries {
		if e.resource == r {
			return e.label
		}
	}
	return "unknown"
}

// RailModel is the left-rail navigator. It owns:
//   - the cursor position (for j/k navigation)
//   - the currently *active* resource (may differ from cursor while the
//     user is navigating before pressing enter)
//
// The rail is always rendered in the shell's left gutter. Hotkeys 1-4 set
// both cursor AND active resource in one keystroke; j/k + enter sets them
// over two keystrokes.
type RailModel struct {
	styles *Styles
	cursor int      // 0..len(railEntries)-1
	active Resource // currently displayed in the right pane
	width  int
	height int
}

// NewRail creates a rail with the games resource active and cursor at 0.
func NewRail(styles *Styles) RailModel {
	return RailModel{
		styles: styles,
		cursor: 0,
		active: ResourceGames,
	}
}

// Active reports the currently active resource (shown in the right pane).
func (m RailModel) Active() Resource { return m.active }

// Cursor reports the current rail cursor position.
func (m RailModel) Cursor() int { return m.cursor }

// SetSize stores the rail's allotted render dimensions.
func (m *RailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SelectHotkey matches a key against the rail's 1-4 hotkeys. On match it
// sets BOTH cursor and active resource and returns true. On miss the rail
// state is untouched and the caller must route the key elsewhere.
func (m *RailModel) SelectHotkey(key string) bool {
	for i, e := range railEntries {
		if e.hotkey == key {
			m.cursor = i
			m.active = e.resource
			return true
		}
	}
	return false
}

// Update routes a keypress. Return value handled=true means the rail
// consumed the key; handled=false means the caller should route it to
// whatever currently owns focus beyond the rail itself.
//
// Contract:
//   - "j" / "down" moves cursor down (clamped)
//   - "k" / "up"   moves cursor up   (clamped)
//   - "enter"      promotes cursor to active resource
//   - "1".."4"     sets both cursor and active in one stroke
func (m RailModel) Update(msg tea.Msg) (RailModel, tea.Cmd, bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil, false
	}
	switch key.String() {
	case "j", "down":
		if m.cursor < len(railEntries)-1 {
			m.cursor++
		}
		return m, nil, true
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil, true
	case "enter":
		m.active = railEntries[m.cursor].resource
		return m, nil, true
	case "1", "2", "3", "4":
		if m.SelectHotkey(key.String()) {
			return m, nil, true
		}
	}
	return m, nil, false
}

// View renders the rail. Focused rail rows render with the accent-focus
// (cyan) token via FocusStyle; idle rows render muted. The active resource
// (which may differ from the cursor mid-navigation) carries a small glyph.
func (m RailModel) View(railFocused bool) string {
	s := m.styles
	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Foreground(s.Theme.AccentOverride).
		Bold(true)
	b.WriteString(titleStyle.Render("Resources"))
	b.WriteString("\n\n")

	for i, e := range railEntries {
		// Build the line: "[hotkey] label"  with an active glyph when that
		// resource is the one currently driving the right pane.
		glyph := "  "
		if e.resource == m.active {
			glyph = s.OverrideMarkerStyle().Render("◆ ")
		}

		hotkey := lipgloss.NewStyle().
			Foreground(s.Theme.AccentOverride).
			Bold(true).
			Render(fmt.Sprintf("[%s]", e.hotkey))

		label := e.label
		switch {
		case railFocused && i == m.cursor:
			label = s.FocusStyle().Render(label)
		case e.resource == m.active:
			label = lipgloss.NewStyle().Foreground(s.Theme.Fg).Render(label)
		default:
			label = s.Dim.Render(label)
		}

		b.WriteString(glyph)
		b.WriteString(hotkey)
		b.WriteString(" ")
		b.WriteString(label)
		b.WriteString("\n")
	}

	if s.ShowHints {
		b.WriteString("\n")
		b.WriteString(s.Dim.Render("j/k move"))
		b.WriteString("\n")
		b.WriteString(s.Dim.Render("enter pick"))
		b.WriteString("\n")
		b.WriteString(s.Dim.Render("1-4 jump"))
	}

	return b.String()
}
