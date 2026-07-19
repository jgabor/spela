package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

// resourcePaneModel renders the content column for the active destination.
type resourcePaneModel struct {
	styles            *Styles
	services          *Services
	navState          *nav.State
	content           ContentModel
	defaultsDetail    DetailModel
	persistedDefaults *profile.Profile
	overview          OverviewModel
	dllsResource      DLLsResourceModel
	metricsView       MetricsResourceModel
	settings          OptionsModalModel
	width             int
	height            int
}

func newResourcePane(styles *Styles, content ContentModel) resourcePaneModel {
	if content.profileSaves == nil {
		content.profileSaves = &profileSaveState{}
	}
	return resourcePaneModel{
		styles:       styles,
		content:      content,
		overview:     NewOverview(styles),
		dllsResource: NewDLLsResource(styles, nil),
		metricsView:  NewMetricsResource(styles),
		settings:     NewOptionsModal(styles),
	}
}

func (p *resourcePaneModel) setServices(svc *Services) {
	p.services, p.dllsResource.services = svc, svc
	p.refreshDefaultsDetail()
}

func (p *resourcePaneModel) refreshDefaultsDetail() {
	preserveField := p.defaultsDetail.FocusedField()
	preserveCursor := p.defaultsDetail.Cursor()
	preserveSubsystem := p.defaultsDetail.activeSubsystem
	var defaults *profile.Profile
	if p.services != nil && p.services.LoadDefaultProfile != nil {
		defaults, _ = p.services.LoadDefaultProfile()
		p.persistedDefaults = defaults.Clone()
	}
	if desired := p.content.profileSaves.latest(0); desired != nil {
		defaults = desired.Clone()
	}
	p.defaultsDetail = NewRootDetail(p.styles, defaults)
	p.defaultsDetail.SetActiveSubsystem(preserveSubsystem)
	p.defaultsDetail.RestoreFocus(preserveField, preserveCursor)
}

func (p *resourcePaneModel) BindNavState(navState *nav.State) {
	p.navState = navState
}

func (p *resourcePaneModel) SetState(s nav.State) {
	if p.navState != nil {
		*p.navState = s
	}
	p.applyProfileSubsystem()
	if p.navState != nil && p.navState.Destination == nav.DestinationSettings {
		p.settings.SyncNavSection(p.navState.SettingsSection)
	}
}

func (p resourcePaneModel) State() nav.State {
	if p.navState == nil {
		return nav.DefaultState()
	}
	return *p.navState
}

func (p *resourcePaneModel) applyProfileSubsystem() {
	state := p.State()
	key := state.ProfileSubsystem.Key()
	if state.Scope.Kind == nav.ScopeGlobal {
		p.defaultsDetail.SetActiveSubsystem(key)
	} else {
		p.content.detail.SetActiveSubsystem(key)
	}
}

func (p *resourcePaneModel) SetSize(width, height int) {
	p.width, p.height = width, height
	p.content.SetSize(width-4, height)
	p.defaultsDetail.SetSize(width-4, height)
	p.dllsResource.SetSize(width-4, height)
	p.metricsView.SetSize(width-4, height)
	p.settings.SetSize(width, height)
}

func (p *resourcePaneModel) SetDLLsData(games []*game.Game, manifest *dll.Manifest) {
	p.dllsResource = p.dllsResource.SetGames(games).SetManifest(manifest).RefreshCached()
}

func (p *resourcePaneModel) SetMetricsData(h HeaderModel) {
	p.metricsView = p.metricsView.SetData(
		h.gpuMetrics,
		h.cpuMetrics,
		h.alerts,
		h.tempBuffer,
		h.utilBuffer,
		h.powerBuffer,
		h.cpuBuffer,
	)
}

// View renders the content pane for the current navigation state.
func (p resourcePaneModel) View(contentFocused bool) string {
	switch p.State().Destination {
	case nav.DestinationLibrary:
		return p.renderLibrary(contentFocused)
	case nav.DestinationDLLCatalog:
		return p.dllsResource.View(contentFocused, p.State().DLLCatalogSection)
	case nav.DestinationMonitor:
		return p.metricsView.View(contentFocused, p.State().MonitorSection)
	case nav.DestinationSettings:
		return p.renderSettings(contentFocused)
	}
	return ""
}

func (p resourcePaneModel) renderLibrary(contentFocused bool) string {
	s := p.styles
	borderColor := s.BorderColor(contentFocused)
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	var body strings.Builder
	switch p.State().Aspect {
	case nav.AspectOverview:
		if p.State().Scope.Kind != nav.ScopeGame {
			body.WriteString(s.Dim.Render("Overview is available for a selected game."))
		} else {
			body.WriteString(p.overview.View())
		}
	case nav.AspectDLLs:
		if p.State().Scope.Kind != nav.ScopeGame {
			body.WriteString(s.Dim.Render("DLL management requires a selected game."))
		} else {
			body.WriteString(p.content.ViewDLLAspect())
		}
	case nav.AspectProfile:
		if p.State().Scope.Kind == nav.ScopeGlobal {
			body.WriteString(s.Title.Render("All games (default profile)"))
			body.WriteString("\n")
			body.WriteString(s.Dim.Render("Root profile — fields here feed games by inheritance."))
			body.WriteString("\n\n")
			body.WriteString(p.defaultsDetail.View())
		} else if p.content.game == nil {
			body.WriteString(s.Dim.Render("Select a game from the scope list"))
		} else {
			body.WriteString(p.content.ViewProfileAspect())
		}
	default:
		body.WriteString(s.Dim.Render("Select an aspect"))
	}

	return boxStyle.Render(body.String())
}

func (p resourcePaneModel) renderSettings(contentFocused bool) string {
	s := p.styles
	borderColor := s.BorderColor(contentFocused)
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(s.Title.Render("Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.settings.renderOptionsBody())
	return boxStyle.Render(b.String())
}

// Update routes input to the content column when ZoneContent is active.
func (p resourcePaneModel) Update(msg tea.Msg) (resourcePaneModel, tea.Cmd) {
	if m, ok := msg.(dllsUpdateAllCompleteMsg); ok {
		next, cmd := p.dllsResource.Update(m)
		p.dllsResource = next
		return p, cmd
	}

	switch p.State().Destination {
	case nav.DestinationLibrary:
		return p.updateLibrary(msg)
	case nav.DestinationDLLCatalog:
		next, cmd := p.dllsResource.Update(msg)
		p.dllsResource = next
		return p, cmd
	case nav.DestinationSettings:
		var cmd tea.Cmd
		p.settings, cmd = p.settings.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p resourcePaneModel) updateLibrary(msg tea.Msg) (resourcePaneModel, tea.Cmd) {
	if p.content.HasModalOpen() {
		content, cmd := p.content.Update(msg)
		p.content = content
		return p, cmd
	}

	switch p.State().Aspect {
	case nav.AspectProfile:
		if p.State().Scope.Kind == nav.ScopeGlobal {
			if key, ok := msg.(tea.KeyPressMsg); ok {
				switch key.String() {
				case "left", "h":
					if p.defaultsDetail.CycleFocusedField(-1) {
						return p, p.saveDefaultProfile()
					}
					return p, nil
				case "right", "l":
					if p.defaultsDetail.CycleFocusedField(1) {
						return p, p.saveDefaultProfile()
					}
					return p, nil
				case "r":
					if changed, err := p.defaultsDetail.ResetFocused(); err == nil && changed {
						return p, p.saveDefaultProfile()
					}
					return p, nil
				case "R":
					if p.defaultsDetail.ResetAll() {
						return p, p.saveDefaultProfile()
					}
					return p, nil
				}
			}
			detail, cmd, _ := p.defaultsDetail.Update(msg)
			p.defaultsDetail = detail
			return p, cmd
		}
		content, cmd := p.content.Update(msg)
		p.content = content
		return p, cmd
	case nav.AspectDLLs, nav.AspectOverview:
		if p.State().Scope.Kind == nav.ScopeGame {
			content, cmd := p.content.Update(msg)
			p.content = content
			return p, cmd
		}
	}
	return p, nil
}

// HasModalOpen reports modals that should suppress global hotkeys.
func (p resourcePaneModel) HasModalOpen() bool {
	destination := p.State().Destination
	return destination == nav.DestinationLibrary && p.content.HasModalOpen() ||
		destination == nav.DestinationSettings && p.settings.editingPath
}

func (p resourcePaneModel) contentModel() *ContentModel {
	if p.State().Destination == nav.DestinationLibrary && p.State().Scope.Kind == nav.ScopeGame {
		return &p.content
	}
	return nil
}

// saveDefaultProfile persists the root defaults profile asynchronously,
// emitting profileSaveMsg on completion (handled by the layout).
func (p *resourcePaneModel) saveDefaultProfile() tea.Cmd {
	raw := p.defaultsDetail.RawProfile()
	if raw == nil {
		return nil
	}
	before := p.persistedDefaults
	if desired := p.content.profileSaves.latest(0); desired != nil {
		before = desired
	}
	return p.content.profileSaves.start(profileSaveRequest{before: before.Clone(), desired: raw.Clone()})
}

func (p *resourcePaneModel) completeDefaultSave(message profileSaveMsg) tea.Cmd {
	if message.err == nil && message.request.desired != nil {
		p.persistedDefaults = message.request.desired.Clone()
	}
	return p.content.profileSaves.complete(message)
}

// loadGlobalScope prepares default profile content.
func (p *resourcePaneModel) loadGlobalScope() {
	if p.navState == nil {
		return
	}
	*p.navState = p.State().SelectScope(nav.Scope{Kind: nav.ScopeGlobal})
	*p.navState = p.State().SelectAspect(nav.AspectProfile)
	p.refreshDefaultsDetail()
	p.applyProfileSubsystem()
}

// loadGameScope prepares per-game content.
func (p *resourcePaneModel) loadGameScope(g *game.Game) {
	if p.navState == nil {
		return
	}
	*p.navState = p.State().SelectScope(nav.Scope{
		Kind:     nav.ScopeGame,
		GameName: g.Name,
		AppID:    g.AppID,
	})
	p.content = p.content.SetGame(g)
	p.overview = p.overview.SetGame(g, p.services)
	p.applyProfileSubsystem()
}
