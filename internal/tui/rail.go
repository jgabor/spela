package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/nav"
)

// primaryEntry is one row on the primary navigator.
type primaryEntry struct {
	destination nav.Destination
	hotkey      string
	label       string
}

// primaryEntries is the ordered list of primary nav rows.
var primaryEntries = []primaryEntry{
	{destination: nav.DestinationLibrary, hotkey: "1", label: "Library"},
	{destination: nav.DestinationDLLCatalog, hotkey: "2", label: "DLL Catalog"},
	{destination: nav.DestinationMonitor, hotkey: "3", label: "Monitor"},
	{destination: nav.DestinationSettings, hotkey: "4", label: "Settings"},
}

// RailModel is the primary (left) navigator in the three-zone shell.
type RailModel struct {
	styles *Styles
	cursor int
	active nav.Destination
	width  int
	height int
}

// NewRail creates primary nav with Library active.
func NewRail(styles *Styles) RailModel {
	return RailModel{
		styles: styles,
		cursor: 0,
		active: nav.DestinationLibrary,
	}
}

// Active reports the current destination shown in context + content panes.
func (m RailModel) Active() nav.Destination { return m.active }

// Cursor reports the cursor row index.
func (m RailModel) Cursor() int { return m.cursor }

// SetActive sets the active destination and aligns the cursor.
func (m *RailModel) SetActive(d nav.Destination) {
	m.active = d
	for i, e := range primaryEntries {
		if e.destination == d {
			m.cursor = i
			return
		}
	}
}

// SetSize stores render dimensions.
func (m *RailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SyncFromState aligns rail highlight with canonical navigation state.
func (m *RailModel) SyncFromState(s nav.State) {
	m.SetActive(s.Destination)
}

// SelectHotkey matches 1-4 and activates the destination.
func (m *RailModel) SelectHotkey(key string) bool {
	for i, e := range primaryEntries {
		if e.hotkey == key {
			m.cursor = i
			m.active = e.destination
			return true
		}
	}
	return false
}

// Update handles primary-nav keys when ZonePrimary is focused.
func (m RailModel) Update(msg tea.Msg) (RailModel, tea.Cmd, bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil, false
	}
	switch key.String() {
	case "j", "down":
		if m.cursor < len(primaryEntries)-1 {
			m.cursor++
		}
		return m, nil, true
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil, true
	case "enter":
		m.active = primaryEntries[m.cursor].destination
		return m, nil, true
	case "1", "2", "3", "4":
		if m.SelectHotkey(key.String()) {
			return m, nil, true
		}
	}
	return m, nil, false
}

// View renders the primary navigation column.
func (m RailModel) View(focused bool) string {
	s := m.styles
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(s.Theme.AccentOverride).
		Bold(true)
	b.WriteString(titleStyle.Render("Navigate"))
	b.WriteString("\n\n")

	for i, e := range primaryEntries {
		glyph := "  "
		if e.destination == m.active {
			glyph = s.OverrideMarkerStyle().Render("◆ ")
		}

		hotkey := lipgloss.NewStyle().
			Foreground(s.Theme.AccentOverride).
			Bold(true).
			Render(fmt.Sprintf("[%s]", e.hotkey))

		label := e.label
		switch {
		case focused && i == m.cursor:
			label = s.FocusStyle().Render(label)
		case e.destination == m.active:
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
