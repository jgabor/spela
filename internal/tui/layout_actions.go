package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jgabor/spela/internal/nav"
)

func (m LayoutModel) operationsPending() bool {
	saves := m.pane.content.profileSaves
	return saves != nil && (saves.active != nil || len(saves.queued) > 0) || m.rescanBusy || m.pane.settings.Busy() || m.batchBusy || m.pane.content.dllOperating || m.pane.dllsResource.busy || m.pane.content.dllInstallState == DLLInstallDownloading
}

func (m LayoutModel) actionContext() BindingContext {
	c := BindingContext{Mode: m.inputMode, Focus: m.focus, Destination: m.navState.Destination, GameScope: m.navState.Scope.Kind == nav.ScopeGame && m.pane.content.game != nil, Aspect: m.navState.Aspect, DLLSection: m.navState.DLLCatalogSection, HasBackup: m.pane.content.hasBackup, Busy: m.operationsPending()}
	c.Busy = c.Busy || m.pendingRescan != nil
	c.ProfileScope = m.navState.Scope.Kind == nav.ScopeGlobal || c.GameScope
	c.ListGroups = c.Destination == nav.DestinationSettings || c.Destination == nav.DestinationDLLCatalog
	c.SelectedCount = len(m.listPane.sidebar.selected)
	c.VisibleSelectedCount = len(m.listPane.sidebar.SelectedGames())
	c.SelectedUpdateCount = len(latestDLLMutationTargets(m.listPane.sidebar.SelectedGames(), m.pane.dllsResource.manifest))
	switch c.Destination {
	case nav.DestinationLibrary:
		c.CanMoveList, c.CanOpen = len(m.listPane.sidebar.filtered) > 0, m.listPane.sidebar.SelectedItem() != nil
		c.CanSelect = m.listPane.sidebar.Selected() != nil
		c.HasFields = c.ProfileScope && c.Aspect == nav.AspectProfile
		c.Editable = c.HasFields
		c.Scrollable = c.Aspect == nav.AspectOverview && m.pane.detailScrollMaximum() > 0
		c.SaveAvailable = c.HasFields && c.Focus == FocusDetail
		if c.HasFields {
			detail := m.pane.profileDetail()
			c.Dirty, c.EditorInput, c.EditorEnterLabel, c.EditorKind = detail.Dirty(), detail.EditorInputFocused(), detail.EditorEnterLabel(), detail.EditorInputKind()
		}
	case nav.DestinationSettings:
		c.CanMoveList, c.CanOpen = m.pane.settings.getCurrentOption() != nil, m.pane.settings.getCurrentOption() != nil
		c.Editable = c.CanOpen && !m.pane.settings.Busy()
		c.SaveAvailable = !m.pane.settings.Busy()
		c.Dirty = m.pane.settings.Dirty()
		c.EditorInput, c.EditorEnterLabel, c.EditorKind = m.pane.settings.EditorInputFocused(), m.pane.settings.EditorEnterLabel(), m.pane.settings.EditorInputKind()
	case nav.DestinationDLLCatalog:
		count := len(m.pane.dllsResource.knownDLLTypes())
		if c.DLLSection == nav.SectionDLLDeployment {
			count = len(m.pane.dllsResource.deploymentGames)
		}
		c.CanMoveList, c.CanOpen = count > 0, count > 0
		c.Scrollable = m.pane.dllsResource.DetailScrollable(c.DLLSection)
	case nav.DestinationMonitor:
		c.CanMoveList, c.CanOpen = true, true
		c.Scrollable = m.pane.detailScrollMaximum() > 0
	}
	c.CanSwitchPane = c.Focus == FocusDetail || c.CanOpen || c.HasFields || c.Scrollable
	c.DLLActions = make(map[KeyAction]actionAvailability)
	c.DLLLabels = make(map[KeyAction]string)
	for _, action := range []KeyAction{ActionDetailInstall, ActionDetailUpdate, ActionDetailRestore} {
		available, reason := m.pane.content.DLLActionAvailability(action)
		label := m.pane.content.DLLActionLabel(action)
		if c.Destination == nav.DestinationDLLCatalog {
			available, reason = m.pane.dllsResource.DLLActionAvailability(action, c.DLLSection)
			label = m.pane.dllsResource.DLLActionLabel(action)
		}
		c.DLLActions[action] = actionAvailability{available, reason}
		c.DLLLabels[action] = label
	}
	if m.pane.Editing() {
		c.Mode = ModeEdit
	}
	if m.searchSession != nil {
		c.Mode = ModeSearch
		c.EditorInput = m.searchSession.control == 0
		c.EditorEnterLabel = m.searchEnterLabel()
	}
	if m.actions != nil || m.decision != nil || m.showHelp || m.showBatchMenu || m.pane.content.HasModalOpen() && c.Destination == nav.DestinationLibrary || m.pane.dllsResource.HasModalOpen() && c.Destination == nav.DestinationDLLCatalog {
		c.Mode = ModeOverlay
	}
	return c
}

func (m LayoutModel) routeKey(key tea.KeyPressMsg) (LayoutModel, tea.Cmd) {
	// Terminal interruption is deliberately outside the normal command set.
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	c := m.bindingContext()
	resolution := CanonicalKeymap.Lookup(c, key.String())
	if m.decision != nil {
		return m.handleDecisionAction(resolution.Binding.Action)
	}
	if m.actions != nil {
		return m.handleActionsKey(resolution.Binding.Action)
	}
	if m.showHelp {
		next, command, _ := m.handleHelpKeys(key)
		return next, command
	}
	if m.showBatchMenu {
		next, command, _ := m.handleBatchMenuKeys(key)
		return next, command
	}
	if c.Mode == ModeOverlay {
		return m.updatePaneAction(resolution.Binding.Action)
	}
	if c.Mode == ModeSearch {
		return m.handleSearchKey(key, resolution.Binding.Action)
	}
	if c.Mode == ModeEdit {
		if isEditorTextKey(key, c) {
			var command tea.Cmd
			m.pane, command = m.pane.UpdateEditorInput(key)
			return m, command
		}
		if resolution.Supported && resolution.Available {
			return m.updatePaneAction(resolution.Binding.Action)
		}
		return m, nil
	}
	if !resolution.Supported || !resolution.Available {
		return m, nil
	}
	return m.dispatchAction(resolution.Binding.Action)
}

func isEditorTextKey(key tea.KeyPressMsg, context BindingContext) bool {
	if !context.EditorInput || context.EditorKind == EditorBool || context.EditorKind == EditorChoice {
		return false
	}
	if key.Text != "" || isPrintableKey(key.String()) {
		return true
	}
	switch key.String() {
	case "backspace", "delete", "home", "end", "left", "right", "space":
		return true
	}
	return false
}

func (m LayoutModel) updatePaneAction(action KeyAction) (LayoutModel, tea.Cmd) {
	wasCatalogModal := m.pane.dllsResource.HasModalOpen()
	wasEditing := m.pane.Editing()
	var command tea.Cmd
	m.pane, command = m.pane.UpdateAction(action)
	if !wasEditing && m.pane.Editing() {
		m.messageBar.Clear()
	}
	if m.navState.Destination == nav.DestinationDLLCatalog {
		isCatalogModal := m.pane.dllsResource.HasModalOpen()
		if !wasCatalogModal && isCatalogModal {
			returnFocus := m.focus
			m.dllCatalogReturnFocus = &returnFocus
			m.focus = FocusDetail
			m.calculateDimensions()
		} else if wasCatalogModal && !isCatalogModal && m.dllCatalogReturnFocus != nil {
			m.focus = *m.dllCatalogReturnFocus
			m.dllCatalogReturnFocus = nil
			m.calculateDimensions()
		}
	}
	return m, command
}

func (m LayoutModel) handleActionsKey(action KeyAction) (LayoutModel, tea.Cmd) {
	items := m.menuBindings()
	if len(items) > 0 {
		m.actions.cursor = min(m.actions.cursor, len(items)-1)
	}
	switch action {
	case ActionOverlayFocusNext:
		m.actions.closeFocused = !m.actions.closeFocused
	case ActionOverlayPrevious:
		if !m.actions.closeFocused && len(items) > 0 {
			m.actions.cursor = (m.actions.cursor + len(items) - 1) % len(items)
		}
	case ActionOverlayNext:
		if !m.actions.closeFocused && len(items) > 0 {
			m.actions.cursor = (m.actions.cursor + 1) % len(items)
		}
	case ActionOverlayConfirm:
		if m.actions.closeFocused {
			m.actions = nil
			return m, nil
		}
		if len(items) > 0 && items[m.actions.cursor].Available {
			action = items[m.actions.cursor].Binding.Action
			m.actions = nil
			return m.dispatchAction(action)
		}
	}
	return m, nil
}

func (m LayoutModel) dispatchAction(action KeyAction) (LayoutModel, tea.Cmd) {
	context := m.bindingContext()
	resolution := CanonicalKeymap.Action(context, action)
	if !resolution.Supported || !resolution.Available || !actionRelevant(action, context) {
		return m, nil
	}
	switch action {
	case ActionShowActions:
		m.actions = &actionMenu{}
		return m, nil
	case ActionShowHelp:
		m.help = NewHelpForContext(m.styles, context)
		m.help.SetSize(max(m.width-10, 1), max(m.height-10, 3))
		m.showHelp = true
		return m, nil
	case ActionQuit:
		return m.requestQuit()
	case ActionDestinationLibrary, ActionDestinationDLLs, ActionDestinationMonitor, ActionDestinationSettings:
		destination := map[KeyAction]nav.Destination{ActionDestinationLibrary: nav.DestinationLibrary, ActionDestinationDLLs: nav.DestinationDLLCatalog, ActionDestinationMonitor: nav.DestinationMonitor, ActionDestinationSettings: nav.DestinationSettings}[action]
		m.selectDestination(destination)
		return m, nil
	case ActionFocusNext:
		return m.handleTabKey(false), nil
	case ActionLayoutStandard, ActionLayoutCompact, ActionLayoutFocused:
		m.densityMode = map[KeyAction]DensityMode{ActionLayoutStandard: DensityStandard, ActionLayoutCompact: DensityCompact, ActionLayoutFocused: DensityFocused}[action]
		m.calculateDimensions()
		return m, nil
	case ActionDetailPrevious:
		return m.selectAdjacentLibraryAspect(-1), nil
	case ActionDetailNext:
		return m.selectAdjacentLibraryAspect(1), nil
	case ActionRescanLibrary:
		m.rescanBusy = true
		m.rescanRequestID++
		return m, tea.Batch(m.messageBar.SetMessage("Rescanning games...", MessageInfo), m.rescanGames())
	case ActionStartSearch:
		return m.startSearch()
	case ActionCancelDraft, ActionDetailResetAll:
		return m.confirmDraftAction(action), nil
	}
	if resolution.Binding.Scope == ScopeList {
		return m.updateListAction(action)
	}
	return m.updatePaneAction(action)
}

func (m LayoutModel) updateListAction(action KeyAction) (LayoutModel, tea.Cmd) {
	switch m.navState.Destination {
	case nav.DestinationLibrary:
		if action == ActionListSelect && len(m.listPane.sidebar.SelectedGames()) > 0 || action == ActionBatchUpdate {
			m.showBatchMenu = true
			m.batchGames = m.listPane.sidebar.SelectedGames()
			m.batchCursor = 0
			m.batchMessage = ""
			m.batchCloseFocused = false
			m.batchScrollOffset = 0
			return m, nil
		}
		next, _ := m.listPane.sidebar.cloneSelection().UpdateAction(action)
		return m.requestSidebar(next, action == ActionListSelect)
	case nav.DestinationDLLCatalog:
		if action == ActionListSelect {
			m.focus = FocusDetail
			return m, nil
		}
		switch action {
		case ActionListPreviousGroup, ActionListNextGroup:
			m.navState.DLLCatalogSection = (m.navState.DLLCatalogSection + 1) % 2
		default:
			m.pane.dllsResource = m.pane.dllsResource.UpdateListAction(action, m.navState.DLLCatalogSection)
		}
	case nav.DestinationSettings:
		if action == ActionListSelect {
			m.focus = FocusDetail
			return m, nil
		}
		switch action {
		case ActionListPreviousGroup, ActionListNextGroup:
			count := len(m.pane.settings.sections)
			if count > 0 {
				delta := 1
				if action == ActionListPreviousGroup {
					delta = -1
				}
				m.pane.settings.sectionCursor = (m.pane.settings.sectionCursor + delta + count) % count
				m.pane.settings.optionCursor = 0
				m.navState.SettingsSection = nav.SettingsSection(m.pane.settings.sectionCursor)
			}
		case ActionListPrevious:
			m.pane.settings.moveCursor(-1)
		case ActionListNext:
			m.pane.settings.moveCursor(1)
		}
	case nav.DestinationMonitor:
		if action == ActionListSelect {
			m.focus = FocusDetail
			return m, nil
		}
		if action == ActionListPrevious {
			m.listPane.cursor = max(m.listPane.cursor-1, 0)
		}
		if action == ActionListNext {
			m.listPane.cursor = min(m.listPane.cursor+1, len(nav.MonitorSectionLabels)-1)
		}
		m.listPane.applySectionCursor()
		m.pane.scrollOffset = 0
	}
	return m, nil
}

func (m LayoutModel) statusKeys() []ContextKey {
	context := m.bindingContext()
	var keys []ContextKey
	for _, item := range CanonicalKeymap.HelpBindings(context) {
		binding := describeBinding(item.Binding, context)
		if strings.HasPrefix(string(binding.Action), "destination-") || binding.Action == ActionQuit || len(binding.Keys) == 0 {
			continue
		}
		var labels []string
		for _, key := range binding.Keys {
			if !key.Printable {
				labels = append(labels, key.Label)
			}
		}
		if len(labels) > 0 {
			keys = append(keys, ContextKey{Key: strings.Join(labels, " "), Action: binding.Description, Enabled: true})
		}
	}
	return keys
}
