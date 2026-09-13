package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/nav"
)

type DestinationBarModel struct {
	styles *Styles
	active nav.Destination
	width  int
}

func NewDestinationBar(styles *Styles) DestinationBarModel {
	return DestinationBarModel{styles: styles, active: nav.DestinationLibrary}
}

func (m *DestinationBarModel) SetActive(destination nav.Destination) { m.active = destination }
func (m *DestinationBarModel) SetWidth(width int)                    { m.width = width }

func (m DestinationBarModel) View() string { return m.ViewWithKeys(true) }

func (m DestinationBarModel) ViewWithKeys(showKeys bool) string {
	entries := []struct {
		destination nav.Destination
		key         string
	}{
		{nav.DestinationLibrary, "1"},
		{nav.DestinationDLLCatalog, "2"},
		{nav.DestinationMonitor, "3"},
		{nav.DestinationSettings, "4"},
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		label := fmt.Sprintf("%s %s", entry.key, entry.destination)
		if !showKeys {
			label = entry.destination.String()
		}
		style := m.styles.Dim
		if entry.destination == m.active {
			style = lipgloss.NewStyle().Foreground(m.styles.Theme.AccentFocus).Bold(true)
			label = "◆ " + label
		}
		parts = append(parts, style.Render(label))
	}
	return lipgloss.NewStyle().MaxWidth(max(m.width, 1)).Render(strings.Join(parts, "  "))
}
