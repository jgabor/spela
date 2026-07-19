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
	switch msg.String() {
	case "esc", "q":
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
		return m, m.executeBatchAction(), true
	}
	return m, nil, true
}

// handleHelpKeys handles key input when the help overlay is visible.
func (m LayoutModel) handleHelpKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch msg.String() {
	case "?", "esc", "q":
		m.showHelp = false
	}
	return m, nil, true
}

func (m LayoutModel) handleGlobalKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	if next, cmd, handled := m.handleSystemKey(msg); handled {
		return next, cmd, true
	}
	if next, cmd, handled := m.handleRailHotkey(msg); handled {
		return next, cmd, true
	}
	if next, cmd, handled := m.handleAspectHotkey(msg); handled {
		return next, cmd, true
	}
	return m.handleFocusAndResourceKey(msg)
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
		m.showHelp = true
		return m, nil, true
	}
	return m, nil, false
}

func (m LayoutModel) handleRailHotkey(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	if m.navState.Zone != nav.ZonePrimary {
		return m, nil, false
	}
	switch msg.String() {
	case "1", "2", "3", "4":
		if m.pane.HasModalOpen() {
			return m, nil, false
		}
		rail := m.rail
		if rail.SelectHotkey(msg.String()) {
			m.rail = rail
			m.syncNavFromRail()
			m.navState.Zone = nav.ZonePrimary
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m LayoutModel) handleAspectHotkey(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	if m.navState.Zone != nav.ZoneContent {
		return m, nil, false
	}
	if m.navState.Destination != nav.DestinationLibrary || m.navState.Scope.Kind != nav.ScopeGame {
		return m, nil, false
	}
	if m.pane.HasModalOpen() {
		return m, nil, false
	}
	switch msg.String() {
	case "1":
		if m.navState.Scope.Kind == nav.ScopeGlobal {
			return m, nil, false
		}
		*m.navState = m.navState.SelectAspect(nav.AspectOverview)
		m.syncNavToComponents()
		return m, nil, true
	case "2":
		*m.navState = m.navState.SelectAspect(nav.AspectProfile)
		m.syncNavToComponents()
		return m, nil, true
	case "3":
		*m.navState = m.navState.SelectAspect(nav.AspectDLLs)
		m.syncNavToComponents()
		return m, nil, true
	}
	return m, nil, false
}

func (m LayoutModel) handleFocusAndResourceKey(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+f", "/":
		if m.navState.Destination == nav.DestinationLibrary {
			m.navState.Zone = nav.ZoneContext
			sidebar, cmd := m.contextNav.sidebar.FocusSearch()
			m.contextNav.sidebar = sidebar
			return m, cmd, true
		}
	case "ctrl+r":
		messageCmd := m.messageBar.SetMessage("Rescanning games...", MessageInfo)
		return m, tea.Batch(messageCmd, m.rescanGames()), true
	case "q":
		return m.handleBackOrQuitKey()
	case "esc":
		if m.pane.HasModalOpen() {
			pane, cmd := m.pane.Update(msg)
			m.pane = pane
			return m, cmd, true
		}
		return m.handleBackKey()
	case "tab":
		return m.handleTabKey(), nil, true
	}
	return m, nil, false
}

func (m LayoutModel) handleBackOrQuitKey() (LayoutModel, tea.Cmd, bool) {
	if m.navState.Zone == nav.ZonePrimary {
		return m, tea.Quit, true
	}
	return m.handleBackKey()
}

func (m LayoutModel) handleBackKey() (LayoutModel, tea.Cmd, bool) {
	if m.pane.HasModalOpen() {
		return m, nil, false
	}
	if m.navState.Zone == nav.ZoneContent {
		*m.navState = m.navState.PrevZone()
		m.contextNav.SetState(*m.navState)
		m.pane.SetState(*m.navState)
		return m, nil, true
	}
	if m.navState.Zone == nav.ZoneContext {
		*m.navState = m.navState.PrevZone()
		m.contextNav.SetState(*m.navState)
		return m, nil, true
	}
	return m, nil, false
}

func (m LayoutModel) handleTabKey() LayoutModel {
	if m.pane.HasModalOpen() {
		return m
	}
	*m.navState = m.navState.NextZone()
	if m.navState.Zone == nav.ZoneContext {
		m.contextNav.SetState(*m.navState)
	}
	m.pane.SetState(*m.navState)
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
		m.navState.Zone = nav.ZoneContent
		m.syncNavToComponents()
		cmds = append(cmds, m.pane.content.LoadDLLUpdates())

	case defaultProfileSelectedMsg:
		m.pane.loadGlobalScope()
		m.syncNavToComponents()

	case defaultProfileConfirmedMsg:
		m.pane.loadGlobalScope()
		m.navState.Zone = nav.ZoneContent
		m.syncNavToComponents()

	case batchActionRequestMsg:
		m.showBatchMenu = true
		m.batchGames = msg.selected
		m.batchCursor = 0
		m.batchMessage = ""

	case batchCompleteMsg:
		for _, item := range msg.batch.Items {
			m.pane.content.applyDLLResult(item.Result)
		}
		if m.db != nil {
			games := m.db.List()
			m.pane.dllsResource = m.pane.dllsResource.SetGames(games)
			m.contextNav.sidebar = m.contextNav.sidebar.SetGames(games)
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
		m.config = msg.config
		cmds = append(cmds, m.messageBar.SetMessage("Settings saved!", MessageSuccess))

	case optionsSaveErrorMsg:
		cmds = append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Failed to save settings: %v", msg.err), MessageError))

	case optionsCancelledMsg:
		// no-op for embedded settings
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
	m.pane.content = updated
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLRestoreMsg(msg dllRestoreMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.err == nil {
		message = "Original DLLs restored!"
		msgType = MessageSuccess
	} else if msg.err != nil {
		message = fmt.Sprintf("Restore failed: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.pane.content.Update(msg)
	m.pane.content = updated
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLInstallMsg(msg dllInstallMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.err == nil {
		message = "DLL installed successfully!"
		msgType = MessageSuccess
	} else if msg.err != nil {
		message = fmt.Sprintf("Install failed: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.pane.content.Update(msg)
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
	m.contextNav.sidebar = m.contextNav.sidebar.SetGames(games)
	manifest, _ := dll.LoadManifest()
	m.pane.SetDLLsData(games, manifest)
	cmds = append(cmds, m.messageBar.SetMessage(
		fmt.Sprintf("Rescan complete: %d games found", len(games)),
		MessageSuccess,
	))
	if cm := m.contentModel(); cm != nil && cm.game != nil {
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
	if msg.success {
		message = "Profile saved!"
		msgType = MessageSuccess
	} else if msg.err != nil {
		message = fmt.Sprintf("Error: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, _ := m.pane.content.Update(msg)
	m.pane.content = updated
	m.pane.refreshDefaultsDetail()
	return m, cmds
}
