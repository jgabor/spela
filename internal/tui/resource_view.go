package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// resourcePaneModel renders whichever resource is currently active on the
// rail. It holds substantive state only for ResourceGames (the existing
// sidebar + per-game detail); the other three resources are stubs in
// Task 3 and get filled in by Tasks 4-6.
//
// Design note: rather than collapse all four resources into a single
// ContentModel, each resource gets its own narrow shape here. Task 4 will
// expand Defaults into its own renderer; Task 6 the DLLs table and the
// Metrics widget relocation. For now, stubs are string literals rendered
// inside the resource pane.
type resourcePaneModel struct {
	styles  *Styles
	sidebar SidebarModel // ResourceGames: list of games on the left
	content ContentModel // ResourceGames: detail on the right
	// innerFocused flips true when the user tabs off the games sidebar into
	// the game detail. Meaningful only for ResourceGames. The other three
	// resources do not yet have internal focus.
	innerFocused bool
	width        int
	height       int
}

func newResourcePane(styles *Styles, sidebar SidebarModel, content ContentModel) resourcePaneModel {
	return resourcePaneModel{
		styles:  styles,
		sidebar: sidebar,
		content: content,
	}
}

func (p *resourcePaneModel) SetSize(width, height int) {
	p.width = width
	p.height = height
	// Inside ResourceGames we split the pane; sidebar gets ~30% of the
	// pane width. Other resources use full width.
	sidebarWidth := max(int(float64(width)*0.35), 22)
	sidebarWidth = min(sidebarWidth, 40)
	p.sidebar.SetSize(sidebarWidth-4, height)
	p.content.SetSize(width-sidebarWidth-4, height)
}

// View renders whichever resource's content is active.
func (p resourcePaneModel) View(active Resource, paneFocused bool) string {
	switch active {
	case ResourceGames:
		return p.renderGames(paneFocused)
	case ResourceDLLs:
		return p.renderStub("DLLs", "Library and deployment table render here (Task 6).")
	case ResourceDefaults:
		return p.renderStub("Defaults", "Default profile renders through the shared detail renderer (Task 4).")
	case ResourceMetrics:
		return p.renderStub("Metrics", "Thermal and sparkline widgets relocate here (Task 6).")
	}
	return p.renderStub("Unknown", "(unreachable)")
}

// renderGames renders the games list + per-game detail. This is the only
// resource with substantive wiring in Task 3.
func (p resourcePaneModel) renderGames(paneFocused bool) string {
	s := p.styles

	sidebarBorder := s.BorderColor(paneFocused && !p.innerFocused)
	detailBorder := s.BorderColor(paneFocused && p.innerFocused)

	sidebarStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(sidebarBorder).
		Padding(0, 1)
	detailStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(detailBorder).
		Padding(0, 1)

	sidebar := sidebarStyle.Render(p.sidebar.View())
	detail := detailStyle.Render(p.content.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)
}

// renderStub renders a placeholder for resources not yet implemented.
func (p resourcePaneModel) renderStub(title, body string) string {
	s := p.styles
	var b strings.Builder
	b.WriteString(s.Title.Render(title))
	b.WriteString("\n\n")
	b.WriteString(s.Dim.Render(body))
	b.WriteString("\n")
	return b.String()
}

// Update routes a message to the active resource's internal handlers. The
// rail layer handles all rail-level keys (1-4, j/k, enter) before this is
// called; by the time a message reaches Update, the rail has already said
// "not mine". Only ResourceGames has interactive internals in Task 3.
func (p resourcePaneModel) Update(msg tea.Msg, active Resource) (resourcePaneModel, tea.Cmd) {
	switch active {
	case ResourceGames:
		return p.updateGames(msg)
	}
	// Stubs ignore input for now.
	return p, nil
}

func (p resourcePaneModel) updateGames(msg tea.Msg) (resourcePaneModel, tea.Cmd) {
	if p.innerFocused {
		content, cmd := p.content.Update(msg)
		p.content = content
		return p, cmd
	}
	sidebar, cmd := p.sidebar.Update(msg)
	p.sidebar = sidebar
	return p, cmd
}

// SetInnerFocused flips the games resource's internal focus between the
// games sidebar (left inner pane) and the game detail (right inner pane).
func (p *resourcePaneModel) SetInnerFocused(v bool) {
	p.innerFocused = v
}

// InnerFocused reports whether the games-resource detail pane currently
// holds focus vs the games sidebar.
func (p resourcePaneModel) InnerFocused() bool {
	return p.innerFocused
}

// HasModalOpen reports whether any resource's internal state has a modal
// or editing session that should suppress rail hotkeys. Only meaningful
// for ResourceGames in Task 3.
func (p resourcePaneModel) HasModalOpen(active Resource) bool {
	if active == ResourceGames {
		return p.content.HasModalOpen() || p.sidebar.search.Focused() || p.sidebar.InSelectMode()
	}
	return false
}

// contentModel accessor: returns nil when we're not on ResourceGames.
func (p resourcePaneModel) contentModel() *ContentModel {
	return &p.content
}
