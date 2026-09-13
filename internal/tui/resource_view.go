package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
	scrollOffset      int
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
	if p.defaultsDetail.Dirty() || p.defaultsDetail.Editing() {
		return
	}
	preserveField := p.defaultsDetail.FocusedField()
	preserveCursor := p.defaultsDetail.Cursor()
	var defaults *profile.Profile
	if p.services != nil && p.services.LoadDefaultProfile != nil {
		defaults, _ = p.services.LoadDefaultProfile()
		p.persistedDefaults = defaults.Clone()
	}
	if desired := p.content.profileSaves.latest(0); desired != nil {
		defaults = desired.Clone()
	}
	p.defaultsDetail = NewRootDetail(p.styles, defaults)
	p.defaultsDetail.SetSize(max(p.width-4, 1), max(p.height-5, 1))
	p.defaultsDetail.RestoreFocus(preserveField, preserveCursor)
}

func (p *resourcePaneModel) BindNavState(navState *nav.State) {
	p.navState = navState
}

func (p *resourcePaneModel) SetState(s nav.State) {
	if p.navState != nil {
		*p.navState = s
	}
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

func (p *resourcePaneModel) SetSize(width, height int) {
	p.width, p.height = width, height
	p.overview.SetSize(width-4, max(height-3, 1))
	p.content.SetSize(width-4, height)
	p.defaultsDetail.SetSize(width-4, max(height-5, 1))
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
	).SetStates(h.gpuState, h.cpuState)
}

// View renders the content pane for the current navigation state.
func (p resourcePaneModel) View(contentFocused bool) string {
	switch p.State().Destination {
	case nav.DestinationLibrary:
		return p.renderLibrary(contentFocused)
	case nav.DestinationDLLCatalog:
		return p.dllsResource.View(contentFocused, p.State().DLLCatalogSection)
	case nav.DestinationMonitor:
		return p.readOnlyView()
	case nav.DestinationSettings:
		return p.renderSettings(contentFocused)
	}
	return ""
}

func (p resourcePaneModel) renderLibrary(contentFocused bool) string {
	if p.content.HasModalOpen() {
		return p.content.ViewDLLAspectFocused(contentFocused)
	}
	s := p.styles

	var body strings.Builder
	if p.State().Scope.Kind == nav.ScopeGame && p.content.game != nil {
		for index, label := range nav.AspectLabels {
			if index > 0 {
				body.WriteString("  ")
			}
			if nav.Aspect(index) == p.State().Aspect {
				body.WriteString(s.Selected.Render("[" + label + "]"))
			} else {
				body.WriteString(s.Dim.Render(label))
			}
		}
		body.WriteString("\n")
		body.WriteString("\n")
	}
	switch p.State().Aspect {
	case nav.AspectOverview:
		if p.State().Scope.Kind != nav.ScopeGame {
			body.WriteString(s.Dim.Render("Overview is available for a selected game."))
		} else {
			body.WriteString(p.readOnlyView())
		}
	case nav.AspectDLLs:
		if p.State().Scope.Kind != nav.ScopeGame {
			body.WriteString(s.Dim.Render("DLL management requires a selected game."))
		} else {
			body.WriteString(p.content.ViewDLLAspectFocused(contentFocused))
		}
	case nav.AspectProfile:
		if p.State().Scope.Kind == nav.ScopeGlobal {
			body.WriteString(s.Title.Render("All games (default profile)"))
			body.WriteString("\n")
			body.WriteString(ansi.Truncate(s.Dim.Render("Root profile: inherited by games."), max(p.width-2, 1), "…"))
			body.WriteString("\n\n")
			body.WriteString(p.defaultsDetail.ViewFocused(contentFocused))
		} else if p.content.game == nil {
			body.WriteString(s.Dim.Render("Select a game from the scope list"))
		} else {
			detail := p.content.detail
			detail.SetSize(max(p.width-4, 1), max(p.height-3, 1))
			body.WriteString(detail.ViewFocused(contentFocused))
		}
	default:
		body.WriteString(s.Dim.Render("Select an aspect"))
	}

	return body.String()
}

func (p resourcePaneModel) renderSettings(contentFocused bool) string {
	s := p.styles

	var b strings.Builder
	b.WriteString(s.Title.Render("Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.settings.DetailViewFocused(contentFocused))
	return b.String()
}

// Update routes input to the destination Detail pane.
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
		return p.updateProfileDetail(msg)
	case nav.AspectOverview:
		if p.State().Scope.Kind == nav.ScopeGame {
			p.overview = p.overview.Update(msg)
		}
		return p, nil
	case nav.AspectDLLs:
		if p.State().Scope.Kind == nav.ScopeGame {
			content, cmd := p.content.Update(msg)
			p.content = content
			return p, cmd
		}
	}
	return p, nil
}

func (p resourcePaneModel) updateProfileDetail(msg tea.Msg) (resourcePaneModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	detail := p.profileDetail()
	context := BindingContext{Mode: ModeBrowse, Focus: FocusDetail, Destination: nav.DestinationLibrary, ProfileScope: true, GameScope: p.State().Scope.Kind == nav.ScopeGame, Aspect: nav.AspectProfile, HasFields: true, Editable: !detail.CanCycleFocusedField(), CanCycle: detail.CanCycleFocusedField(), Dirty: detail.Dirty(), SaveAvailable: true}
	if p.Editing() {
		context.Mode = ModeEdit
		context.EditorInput, context.EditorKind, context.EditorEnterLabel = detail.EditorInputFocused(), detail.EditorInputKind(), detail.EditorEnterLabel()
	}
	resolution := CanonicalKeymap.Lookup(context, key.String())
	if resolution.Binding.Action == ActionEditInput || !resolution.Supported && p.Editing() {
		return p.UpdateEditorInput(key)
	}
	if resolution.Supported && resolution.Available {
		return p.UpdateAction(resolution.Binding.Action)
	}
	return p, nil
}

// HasModalOpen reports modals that should suppress global hotkeys.
func (p resourcePaneModel) HasModalOpen() bool {
	destination := p.State().Destination
	return destination == nav.DestinationLibrary && p.content.HasModalOpen() ||
		destination == nav.DestinationDLLCatalog && p.dllsResource.HasModalOpen() ||
		destination == nav.DestinationSettings && p.settings.Editing()
}

func (p resourcePaneModel) HasDLLMutationConfirmation() bool {
	switch p.State().Destination {
	case nav.DestinationLibrary:
		return p.content.confirmation != nil
	case nav.DestinationDLLCatalog:
		return p.dllsResource.confirmation != nil
	default:
		return false
	}
}

func (p resourcePaneModel) Editing() bool {
	if p.State().Destination == nav.DestinationSettings {
		return p.settings.Editing()
	}
	if p.State().Destination == nav.DestinationLibrary && p.State().Aspect == nav.AspectProfile {
		if p.State().Scope.Kind == nav.ScopeGlobal {
			return p.defaultsDetail.Editing()
		}
		return p.content.detail.Editing()
	}
	return false
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
	p.defaultsDetail.CompleteSaveSnapshot(message.request.desired, message.err)
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
	p.scrollOffset = 0
	if p.defaultsDetail.raw == nil {
		p.refreshDefaultsDetail()
	}
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
	}).SelectAspect(nav.AspectOverview)
	p.content = p.content.SetGame(g)
	p.scrollOffset = 0
	p.overview = p.overview.SetGame(g, p.services)
}

func (p *resourcePaneModel) loadNoScope() {
	*p.navState = p.State().SelectScope(nav.Scope{Kind: nav.ScopeGame})
	p.content = p.content.SetGame(nil)
	p.overview = p.overview.SetGame(nil, p.services)
}
