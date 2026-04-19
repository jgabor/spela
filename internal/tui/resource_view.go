package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// resourcePaneModel renders whichever resource is currently active on the
// rail. In Task 4 both ResourceGames and ResourceDefaults have substantive
// wiring; ResourceDLLs and ResourceMetrics remain stubs until Task 6.
//
// Focus mechanics per resource:
//
//   - ResourceGames: innerFocused=false → games sidebar, innerFocused=true →
//     per-game detail (the DetailModel inside ContentModel). The rail's
//     `tab`/`enter` transfers focus from the rail into the pane; then `tab`
//     flips between sidebar and detail. j/k in the rail navigates the rail;
//     j/k in the sidebar navigates the game list; j/k in the detail moves
//     field focus across group boundaries.
//   - ResourceDefaults: renders a root DetailModel directly (no sidebar —
//     there's only one defaults profile). innerFocused toggles between the
//     rail and the detail pane. j/k in the detail moves field focus.
//   - ResourceDLLs / ResourceMetrics: stubs for Task 6.
type resourcePaneModel struct {
	styles         *Styles
	services       *Services
	sidebar        SidebarModel // ResourceGames: list of games on the left
	content        ContentModel // ResourceGames: detail on the right
	defaultsDetail DetailModel  // ResourceDefaults: root detail renderer
	// innerFocused flips true when the user tabs off the rail into the
	// resource pane. Meaningful for ResourceGames (sidebar vs detail) and
	// ResourceDefaults (detail pane). Ignored for the stub resources.
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

// setServices wires the Services dependency so the pane can lazily refresh
// the defaults-root DetailModel (e.g. after the user edits defaults). Only
// called from LayoutModel construction.
func (p *resourcePaneModel) setServices(svc *Services) {
	p.services = svc
	p.refreshDefaultsDetail()
}

// refreshDefaultsDetail rebuilds the defaults-root DetailModel from the
// current defaults on disk. Called on construction and after a defaults
// save so the pane reflects fresh values. Safe to call with nil services.
func (p *resourcePaneModel) refreshDefaultsDetail() {
	if p.services == nil || p.services.LoadDefaultProfile == nil {
		p.defaultsDetail = NewRootDetail(p.styles, nil)
		return
	}
	defaults, _ := p.services.LoadDefaultProfile()
	p.defaultsDetail = NewRootDetail(p.styles, defaults)
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
	p.defaultsDetail.SetSize(width-4, height)
}

// View renders whichever resource's content is active.
func (p resourcePaneModel) View(active Resource, paneFocused bool) string {
	switch active {
	case ResourceGames:
		return p.renderGames(paneFocused)
	case ResourceDLLs:
		return p.renderStub("DLLs", "Library and deployment table render here (Task 6).")
	case ResourceDefaults:
		return p.renderDefaults(paneFocused)
	case ResourceMetrics:
		return p.renderStub("Metrics", "Thermal and sparkline widgets relocate here (Task 6).")
	}
	return p.renderStub("Unknown", "(unreachable)")
}

// renderDefaults renders the defaults-root DetailModel inside the resource
// pane. isRoot=true on the renderer suppresses inheritance markers and the
// reset/pin keybindings (Task 4 acceptance).
func (p resourcePaneModel) renderDefaults(paneFocused bool) string {
	s := p.styles
	borderColor := s.BorderColor(paneFocused && p.innerFocused)

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	var body strings.Builder
	body.WriteString(s.Title.Render("Default profile"))
	body.WriteString("\n")
	body.WriteString(s.Dim.Render("Root profile — fields here feed games by inheritance."))
	body.WriteString("\n\n")
	body.WriteString(p.defaultsDetail.View())

	return boxStyle.Render(body.String())
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
// "not mine" (or focus is already inside the pane).
func (p resourcePaneModel) Update(msg tea.Msg, active Resource) (resourcePaneModel, tea.Cmd) {
	switch active {
	case ResourceGames:
		return p.updateGames(msg)
	case ResourceDefaults:
		return p.updateDefaults(msg)
	}
	// DLLs and Metrics stubs ignore input until Task 6.
	return p, nil
}

// updateDefaults routes input to the defaults-root DetailModel. j/k moves
// field focus across group-header boundaries per Task 4 acceptance. The
// detail renderer itself enforces isRoot=true — no inheritance markers, no
// reset/pin bindings.
func (p resourcePaneModel) updateDefaults(msg tea.Msg) (resourcePaneModel, tea.Cmd) {
	detail, cmd, _ := p.defaultsDetail.Update(msg)
	p.defaultsDetail = detail
	return p, cmd
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
// for ResourceGames in Task 4; the Defaults detail has no modal yet (Task 5
// will gate reset/pin confirmations if any are introduced).
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
