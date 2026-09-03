package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/nav"
)

// handleBatchMenuKeys handles key input when the batch-action menu is visible.
func (m LayoutModel) handleBatchMenuKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	if m.batchConfirmation != nil {
		confirm, cancel := m.batchConfirmation.update(msg)
		if cancel {
			m.batchConfirmation = nil
			m.batchMessage = dllCancellationResult("Selected-game DLL batch")
			return m, nil, true
		}
		if confirm && !m.batchBusy {
			targets := m.batchConfirmation.targets
			m.batchConfirmation = nil
			m.batchBusy = true
			m.batchMessage = "Updating DLLs..."
			return m, m.executeBatchAction(targets), true
		}
		return m, nil, true
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit, true
	case "esc", "escape":
		m.showBatchMenu = false
		m.batchGames = nil
		return m, nil, true
	case "up", "k":
		if m.batchCursor > 0 {
			m.batchCursor--
		}
	case "down", "j":
		if m.batchCursor < len(batchActions)-1 {
			m.batchCursor++
		}
	case "enter":
		if m.batchBusy {
			return m, nil, true
		}
		targets := latestDLLMutationTargets(m.batchGames, m.pane.dllsResource.manifest)
		if len(targets) == 0 {
			m.batchMessage = "No concrete DLL update targets available"
			return m, nil, true
		}
		m.batchConfirmation = newDLLMutationConfirmation("Confirm batch DLL update", targets, "each current DLL is backed up before replacement")
		return m, nil, true
	}
	return m, nil, true
}

// handleHelpKeys handles key input when the help overlay is visible.
func (m LayoutModel) handleHelpKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit, true
	case "?", "esc", "escape":
		m.showHelp = false
	case "j", "down":
		m.help.Move(1)
	case "k", "up":
		m.help.Move(-1)
	case "pgdown":
		m.help.Move(max(m.help.height-3, 1))
	case "pgup":
		m.help.Move(-max(m.help.height-3, 1))
	}
	return m, nil, true
}

func (m LayoutModel) handleGlobalKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	context := m.bindingContext()
	resolution := CanonicalKeymap.Lookup(context, msg.String())
	if resolution.Supported && resolution.Available {
		switch resolution.Binding.Action {
		case ActionSearchInput, ActionSearchDelete:
			var command tea.Cmd
			m.listPane, command, _ = m.listPane.Update(msg)
			m.pane.SetState(*m.navState)
			return m, command, true
		case ActionEditInput, ActionEditDelete, ActionEditCommit, ActionEditCancel:
			var command tea.Cmd
			m.pane, command = m.pane.Update(msg)
			return m, command, true
		}
	}
	if context.Mode == ModeSearch && (!resolution.Supported || !resolution.Available) {
		var command tea.Cmd
		m.listPane, command, _ = m.listPane.Update(msg)
		m.pane.SetState(*m.navState)
		return m, command, true
	}
	if context.Mode == ModeEdit && (!resolution.Supported || !resolution.Available) {
		var command tea.Cmd
		m.pane, command = m.pane.Update(msg)
		return m, command, true
	}
	if resolution.Supported && resolution.Available {
		switch resolution.Binding.Action {
		case ActionQuit:
			return m, tea.Quit, true
		case ActionShowHelp:
			m.help = NewHelp(m.styles)
			m.help.SetSize(m.helpWidth(), max(m.height-4, 3))
			m.showHelp = true
			return m, nil, true
		case ActionFocusNext:
			return m.handleTabKey(false), nil, true
		case ActionFocusPrevious:
			return m.handleTabKey(true), nil, true
		case ActionToggleCompact:
			if m.densityMode == DensityCompact {
				m.densityMode = DensityStandard
			} else {
				m.densityMode = DensityCompact
			}
			m.calculateDimensions()
			return m, nil, true
		case ActionToggleFocused:
			if m.densityMode == DensityFocused {
				m.densityMode = DensityStandard
			} else {
				m.densityMode = DensityFocused
			}
			m.calculateDimensions()
			return m, nil, true
		case ActionRescanLibrary:
			messageCommand := m.messageBar.SetMessage("Rescanning games...", MessageInfo)
			return m, tea.Batch(messageCommand, m.rescanGames()), true
		case ActionStartSearch:
			if m.navState.Destination == nav.DestinationLibrary {
				m.focus = FocusList
				m.inputMode = ModeSearch
				sidebar, command := m.listPane.sidebar.FocusSearch()
				m.listPane.sidebar = sidebar
				return m, command, true
			}
		case ActionSearchCancel:
			m.listPane.sidebar.search.SetValue("")
			m.listPane.sidebar.search.Blur()
			m.listPane.sidebar.applyFiltersAndSort()
			m.inputMode = ModeBrowse
			return m, m.listPane.sidebar.selectCurrentItem(), true
		case ActionSearchAccept:
			m.listPane.sidebar.search.Blur()
			m.inputMode = ModeBrowse
			return m, nil, true
		case ActionDetailPrevious:
			return m.selectAdjacentLibraryAspect(-1), nil, true
		case ActionDetailNext:
			return m.selectAdjacentLibraryAspect(1), nil, true
		case ActionEditCancel, ActionCancelDraft:
			if m.navState.Destination == nav.DestinationSettings {
				m.pane.settings.CancelDraft()
				m.inputMode = ModeBrowse
				return m, nil, true
			}
			if m.navState.Destination == nav.DestinationLibrary && m.navState.Aspect == nav.AspectProfile {
				if m.navState.Scope.Kind == nav.ScopeGlobal {
					m.pane.defaultsDetail.CancelDraft()
				} else {
					m.pane.content.detail.CancelDraft()
				}
				return m, nil, true
			}
		case ActionEditSave:
			if m.navState.Destination == nav.DestinationSettings {
				m.inputMode = ModeBrowse
				var command tea.Cmd
				m.pane.settings, command = m.pane.settings.save()
				return m, command, true
			}
		case ActionDestinationLibrary, ActionDestinationDLLs, ActionDestinationMonitor, ActionDestinationSettings:
			if m.pane.HasModalOpen() {
				return m, nil, false
			}
			destination := map[KeyAction]nav.Destination{
				ActionDestinationLibrary:  nav.DestinationLibrary,
				ActionDestinationDLLs:     nav.DestinationDLLCatalog,
				ActionDestinationMonitor:  nav.DestinationMonitor,
				ActionDestinationSettings: nav.DestinationSettings,
			}[resolution.Binding.Action]
			m.selectDestination(destination)
			return m, nil, true
		}
		if resolution.Binding.Scope == ScopeList {
			var command tea.Cmd
			var handled bool
			m, command, handled = m.updateVisibleList(msg)
			m.pane.SetState(*m.navState)
			return m, command, handled
		}
		if resolution.Binding.Scope == ScopeDetail {
			var command tea.Cmd
			m.pane, command = m.pane.Update(msg)
			return m, command, true
		}
	}
	if context.Mode == ModeBrowse {
		return m, nil, true
	}
	if next, cmd, handled := m.handleSystemKey(msg); handled {
		return next, cmd, true
	}
	return m.handleFocusAndResourceKey(msg)
}

func (m LayoutModel) selectAdjacentLibraryAspect(delta int) LayoutModel {
	if m.navState.Destination != nav.DestinationLibrary || m.navState.Scope.Kind != nav.ScopeGame {
		return m
	}
	aspects := []nav.Aspect{nav.AspectOverview, nav.AspectProfile, nav.AspectDLLs}
	index := 0
	for candidate, aspect := range aspects {
		if aspect == m.navState.Aspect {
			index = candidate
			break
		}
	}
	index = (index + delta + len(aspects)) % len(aspects)
	*m.navState = m.navState.SelectAspect(aspects[index])
	m.pane.SetState(*m.navState)
	return m
}

func (m LayoutModel) bindingContext() BindingContext {
	context := BindingContext{
		Mode: m.inputMode, Focus: m.focus, Destination: m.navState.Destination, GameScope: m.navState.Scope.Kind == nav.ScopeGame, Aspect: m.navState.Aspect, HasBackup: m.pane.content.hasBackup, DLLSection: m.navState.DLLCatalogSection,
	}
	if m.pane.Editing() {
		context.Mode = ModeEdit
		return context
	}
	if m.showHelp || m.showBatchMenu || m.pane.content.HasModalOpen() {
		context.Mode = ModeOverlay
		return context
	}
	if m.navState.Destination == nav.DestinationSettings && m.pane.settings.editingPath {
		context.Mode = ModeEdit
		return context
	}
	if m.navState.Destination == nav.DestinationLibrary && m.listPane.sidebar.search.Focused() {
		context.Mode = ModeSearch
	}
	return context
}

func (m LayoutModel) handleSystemKey(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit, true
	case "f5":
		if m.densityMode == DensityCompact {
			m.densityMode = DensityStandard
		} else {
			m.densityMode = DensityCompact
		}
		m.calculateDimensions()
		return m, nil, true
	case "f11":
		if m.densityMode == DensityFocused {
			m.densityMode = DensityStandard
		} else {
			m.densityMode = DensityFocused
		}
		m.calculateDimensions()
		return m, nil, true
	case "?":
		if m.pane.HasModalOpen() {
			return m, nil, false
		}
		m.showHelp = true
		return m, nil, true
	}
	return m, nil, false
}

func (m LayoutModel) handleFocusAndResourceKey(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+f", "/":
		if m.navState.Destination == nav.DestinationLibrary {
			m.focus = FocusList
			m.inputMode = ModeSearch
			sidebar, cmd := m.listPane.sidebar.FocusSearch()
			m.listPane.sidebar = sidebar
			return m, cmd, true
		}
	case "ctrl+r":
		messageCmd := m.messageBar.SetMessage("Rescanning games...", MessageInfo)
		return m, tea.Batch(messageCmd, m.rescanGames()), true
	case "esc":
		if m.pane.HasModalOpen() {
			pane, cmd := m.pane.Update(msg)
			m.pane = pane
			return m, cmd, true
		}
		if m.inputMode == ModeSearch {
			m.listPane.sidebar.search.SetValue("")
			m.listPane.sidebar.search.Blur()
			m.listPane.sidebar.applyFiltersAndSort()
			m.inputMode = ModeBrowse
			return m, nil, true
		}
		return m, nil, false
	case "tab":
		return m.handleTabKey(false), nil, true
	case "shift+tab":
		return m.handleTabKey(true), nil, true
	}
	return m, nil, false
}

func (m LayoutModel) handleTabKey(_ bool) LayoutModel {
	if m.pane.HasModalOpen() {
		return m
	}
	if m.focus == FocusList {
		m.focus = FocusDetail
	} else {
		m.focus = FocusList
	}
	return m
}

// handleAppMessages routes application-level messages that affect multiple components.
func (m LayoutModel) handleAppMessages(msg tea.Msg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	switch msg := msg.(type) {
	case gameSelectedMsg:
		m.pane.loadGameScope(msg.game)
		m.syncNavToComponents()
		cmds = append(cmds, m.pane.content.LoadDLLUpdates())

	case gameConfirmedMsg:
		m.pane.loadGameScope(msg.game)
		m.focus = FocusDetail
		m.syncNavToComponents()
		cmds = append(cmds, m.pane.content.LoadDLLUpdates())

	case defaultProfileSelectedMsg:
		m.pane.loadGlobalScope()
		m.syncNavToComponents()

	case noLibrarySelectionMsg:
		m.pane.loadNoScope()
		m.syncNavToComponents()

	case defaultProfileConfirmedMsg:
		m.pane.loadGlobalScope()
		m.focus = FocusDetail
		m.syncNavToComponents()

	case batchActionRequestMsg:
		m.showBatchMenu = true
		m.batchGames = msg.selected
		m.batchCursor = 0
		m.batchMessage = ""

	case batchCompleteMsg:
		m.batchBusy = false
		for _, item := range msg.batch.Items {
			m.pane.content.applyDLLResult(item.Result)
		}
		if m.db != nil {
			games := m.db.List()
			m.pane.dllsResource = m.pane.dllsResource.SetGames(games)
			m.listPane.sidebar = m.listPane.sidebar.SetGames(games)
		}
		m.batchMessage = msg.message
		messageType := MessageSuccess
		if msg.failed {
			messageType = MessageError
		} else if msg.batch.Updated == 0 {
			messageType = MessageInfo
		}
		cmds = append(cmds, m.messageBar.SetMessage(msg.message, messageType))

	case dllsUpdateAllCompleteMsg:
		msgType := MessageSuccess
		if strings.Contains(msg.summary, "already current") {
			msgType = MessageInfo
		}
		for _, v := range msg.results {
			if strings.HasPrefix(v, "err:") {
				msgType = MessageError
				break
			}
		}
		cmds = append(cmds, m.messageBar.SetMessage(msg.summary, msgType))

	case contentNoticeMsg:
		cmds = append(cmds, m.messageBar.SetMessage(msg.text, msg.messageType))

	case messageClearMsg:
		m.messageBar, _ = m.messageBar.Update(msg)

	case flashTickMsg:
		var cmd tea.Cmd
		m.messageBar, cmd = m.messageBar.Update(msg)
		cmds = append(cmds, cmd)

	case metricsMsg:
		// handled in header

	case dllUpdateMsg:
		m, cmds = m.handleDLLUpdateMsg(msg, cmds)

	case dllRestoreMsg:
		m, cmds = m.handleDLLRestoreMsg(msg, cmds)

	case dllInstallMsg:
		m, cmds = m.handleDLLInstallMsg(msg, cmds)

	case dllTypesLoadedMsg:
		updated, contentCmd := m.pane.content.Update(msg)
		m.pane.content = updated
		cmds = append(cmds, contentCmd)

	case dllVersionsLoadedMsg:
		updated, contentCmd := m.pane.content.Update(msg)
		m.pane.content = updated
		cmds = append(cmds, contentCmd)

	case dllUpdatesCheckedMsg:
		updated, _ := m.pane.content.Update(msg)
		m.pane.content = updated

	case rescanGamesMsg:
		m, cmds = m.handleRescanGamesMsg(msg, cmds)

	case profileSaveMsg:
		m, cmds = m.handleProfileSaveMsg(msg, cmds)

	case optionsSavedMsg:
		if msg.config == nil {
			break
		}
		m.pane.settings.CompleteSave(msg.config)
		m.config = msg.config.Clone()
		if m.styles != nil {
			m.styles.SetShowHints(m.config.ShowHints)
		}
		cmds = append(cmds, m.messageBar.SetMessage("Settings saved!", MessageSuccess))

	case optionsSaveErrorMsg:
		m.pane.settings.saving = false
		m.pane.settings.saveError = msg.err
		cmds = append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Failed to save settings: %v", msg.err), MessageError))

	}

	return m, cmds
}

func (m LayoutModel) handleDLLUpdateMsg(msg dllUpdateMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	message, msgType := "DLLs already up to date", MessageInfo
	if msg.err != nil {
		message, msgType = fmt.Sprintf("Update failed: %v", msg.err), MessageError
	} else if msg.batch.Failed > 0 {
		message, msgType = formatDLLBatch(msg.batch), MessageError
	} else if msg.batch.Updated > 0 {
		message, msgType = formatDLLBatch(msg.batch), MessageSuccess
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.pane.content.Update(msg)
	updated.lastDLLResult = message
	m.pane.content = updated
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLRestoreMsg(msg dllRestoreMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.err != nil {
		message = fmt.Sprintf("Restore failed: %v", msg.err)
		msgType = MessageError
	} else if msg.result.Outcome == dll.OutcomeNoOp {
		message = "Restore: already current"
		msgType = MessageInfo
	} else {
		message = "Original DLLs restored!"
		msgType = MessageSuccess
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.pane.content.Update(msg)
	updated.lastDLLResult = message
	m.pane.content = updated
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLInstallMsg(msg dllInstallMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.err != nil {
		message = fmt.Sprintf("Install failed: %v", msg.err)
		msgType = MessageError
	} else if msg.result.Outcome == dll.OutcomeNoOp {
		message = "Install: already current"
		msgType = MessageInfo
	} else {
		message = "DLL installed successfully!"
		msgType = MessageSuccess
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.pane.content.Update(msg)
	updated.lastDLLResult = message
	m.pane.content = updated
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func formatDLLBatch(batch dll.BatchResult) string {
	message := fmt.Sprintf("DLL update: %d updated, %d current, %d failed", batch.Updated, batch.Unchanged, batch.Failed)
	for _, item := range batch.Items {
		if item.Err != nil {
			message += fmt.Sprintf("; %s: %v", item.Path, item.Err)
		}
	}
	return message
}

func (m LayoutModel) handleRescanGamesMsg(msg rescanGamesMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	if msg.err != nil {
		cmds = append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Rescan failed: %v", msg.err), MessageError))
		return m, cmds
	}
	m.db = msg.db
	m.pane.content.database = msg.db
	m.pane.dllsResource.database = msg.db
	games := msg.db.List()
	m.listPane.sidebar = m.listPane.sidebar.SetGames(games)
	manifest, _ := dll.LoadManifest()
	m.pane.SetDLLsData(games, manifest)
	cmds = append(cmds, m.messageBar.SetMessage(
		fmt.Sprintf("Rescan complete: %d games found", len(games)),
		MessageSuccess,
	))
	if cm := m.pane.contentModel(); cm != nil && cm.game != nil {
		if refreshed := msg.db.GetGame(cm.game.AppID); refreshed != nil {
			m.pane.loadGameScope(refreshed)
			*m.navState = m.pane.State()
			m.syncNavToComponents()
			cmds = append(cmds, m.pane.content.LoadDLLUpdates())
		}
	}
	return m, cmds
}

func (m LayoutModel) handleProfileSaveMsg(msg profileSaveMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.err == nil {
		message = "Profile saved!"
		msgType = MessageSuccess
	} else if msg.err != nil {
		message = fmt.Sprintf("Error: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	if msg.request.appID == 0 {
		cmds = append(cmds, m.pane.completeDefaultSave(msg))
	} else {
		updated, cmd := m.pane.content.Update(msg)
		m.pane.content = updated
		cmds = append(cmds, cmd)
	}
	return m, cmds
}
