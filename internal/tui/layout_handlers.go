package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
)

// handleBatchMenuKeys handles key input when the batch-action menu is visible.
// Returns (model, cmd, handled). When handled is true the caller should return immediately.
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
// Returns (model, cmd, handled). When handled is true the caller should return immediately.
func (m LayoutModel) handleHelpKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch msg.String() {
	case "?", "esc", "q":
		m.showHelp = false
	}
	return m, nil, true
}

// handleGlobalKeys handles the main key bindings that apply globally when no overlay is active.
// Returns (model, cmd, handled). When handled is true the caller should return immediately.
func (m LayoutModel) handleGlobalKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
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
	case "1":
		if !m.sidebarFocused && !m.sidebar.search.Focused() && m.densityMode != DensityFocused {
			m.sidebarFocused = true
			return m, nil, true
		}
	case "2", "3", "4":
		// Global jump keys — switch to content tab from anywhere.
		cm := m.contentModel()
		if cm != nil && cm.game != nil && !cm.defaultProfile && !cm.profileWidget.Editing() {
			m.sidebarFocused = false
			switch msg.String() {
			case "2":
				cm.activeTab = TabDLLs
			case "3":
				cm.activeTab = TabProfile
			case "4":
				cm.activeTab = TabLaunch
			}
			return m, nil, true
		}
	case "?":
		m.showHelp = true
		return m, nil, true
	case "o":
		if m.sidebarFocused && !m.sidebar.search.Focused() {
			m.optionsModal.SetSize(m.width, m.height)
			m.optionsModal.Open(m.config)
			m.activeDialog = &m.optionsModal
			return m, nil, true
		}
	case "ctrl+f":
		m.sidebarFocused = true
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.FocusSearch()
		return m, cmd, true
	case "q":
		if m.sidebarFocused && !m.sidebar.search.Focused() {
			return m, tea.Quit, true
		}
		if !m.sidebarFocused && !m.contentModel().HasModalOpen() {
			if m.stack.Depth() > 1 {
				m.stack.Pop()
				return m, nil, true
			}
			m.sidebarFocused = true
			return m, nil, true
		}
	case "esc":
		if !m.sidebarFocused && !m.contentModel().HasModalOpen() {
			if m.stack.Depth() > 1 {
				m.stack.Pop()
				return m, nil, true
			}
			m.sidebarFocused = true
			return m, nil, true
		}
	case "tab":
		m.sidebarFocused = !m.sidebarFocused
		return m, nil, true
	case "r":
		if m.sidebarFocused && !m.sidebar.search.Focused() {
			messageCmd := m.messageBar.SetMessage("Rescanning games...", MessageInfo)
			return m, tea.Batch(messageCmd, m.rescanGames()), true
		}
	}
	return m, nil, false
}

// handleAppMessages routes application-level messages that affect multiple components.
// It takes the current cmds accumulator and returns an updated model and cmds slice.
func (m LayoutModel) handleAppMessages(msg tea.Msg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	switch msg := msg.(type) {
	case gameSelectedMsg:
		entry := m.contentEntryForGame(msg.game)
		m.stack.Replace(entry)
		cmds = append(cmds, entry.model.LoadDLLUpdates())

	case gameConfirmedMsg:
		entry := m.contentEntryForGame(msg.game)
		m.stack.Replace(entry)
		m.sidebarFocused = false
		cmds = append(cmds, entry.model.LoadDLLUpdates())

	case defaultProfileSelectedMsg:
		entry := m.contentEntryForDefaultProfile()
		m.stack.Replace(entry)

	case defaultProfileConfirmedMsg:
		entry := m.contentEntryForDefaultProfile()
		m.stack.Replace(entry)
		m.sidebarFocused = false

	case batchActionRequestMsg:
		m.showBatchMenu = true
		m.batchGames = msg.selected
		m.batchCursor = 0
		m.batchMessage = ""

	case batchCompleteMsg:
		m.batchMessage = msg.message
		cmds = append(cmds, m.messageBar.SetMessage(msg.message, MessageSuccess))

	case messageClearMsg:
		m.messageBar, _ = m.messageBar.Update(msg)

	case flashTickMsg:
		var cmd tea.Cmd
		m.messageBar, cmd = m.messageBar.Update(msg)
		cmds = append(cmds, cmd)

	case metricsMsg:
		// Already handled in header update; nothing more to do.

	case dllUpdateMsg:
		m, cmds = m.handleDLLUpdateMsg(msg, cmds)

	case dllRestoreMsg:
		m, cmds = m.handleDLLRestoreMsg(msg, cmds)

	case dllInstallMsg:
		m, cmds = m.handleDLLInstallMsg(msg, cmds)

	case dllTypesLoadedMsg:
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
		cmds = append(cmds, contentCmd)

	case dllVersionsLoadedMsg:
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
		cmds = append(cmds, contentCmd)

	case dllUpdatesCheckedMsg:
		updated, _ := m.stack.Top().Update(msg)
		m.stack.Replace(updated)

	case launchGameMsg:
		m, cmds = m.handleLaunchGameMsg(msg, cmds)

	case rescanGamesMsg:
		m, cmds = m.handleRescanGamesMsg(msg, cmds)

	case profileSaveMsg:
		m, cmds = m.handleProfileSaveMsg(msg, cmds)

	case optionsSavedMsg:
		m.config = msg.config
		cmds = append(cmds, m.messageBar.SetMessage("Options saved!", MessageSuccess))

	case optionsSaveErrorMsg:
		cmds = append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Failed to save options: %v", msg.err), MessageError))

	case optionsCancelledMsg:
		// nothing to do
	}

	return m, cmds
}

func (m LayoutModel) handleDLLUpdateMsg(msg dllUpdateMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.success {
		message = "DLLs updated successfully!"
		msgType = MessageSuccess
		if err := m.db.Save(); err != nil {
			message = fmt.Sprintf("DLLs updated but failed to save database: %v", err)
			msgType = MessageError
		}
	} else if msg.err != nil {
		message = fmt.Sprintf("Update failed: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.stack.Top().Update(msg)
	m.stack.Replace(updated)
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLRestoreMsg(msg dllRestoreMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.success {
		message = "Original DLLs restored!"
		msgType = MessageSuccess
		if cm := m.contentModel(); cm.game != nil {
			detected, err := dll.ScanDirectory(cm.game.InstallDir)
			if err == nil {
				cm.game.DLLs = detected
			}
		}
		if err := m.db.Save(); err != nil {
			message = fmt.Sprintf("DLLs restored but failed to save database: %v", err)
			msgType = MessageError
		}
	} else if msg.err != nil {
		message = fmt.Sprintf("Restore failed: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.stack.Top().Update(msg)
	m.stack.Replace(updated)
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLInstallMsg(msg dllInstallMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.success {
		message = "DLL installed successfully!"
		msgType = MessageSuccess
		if err := m.db.Save(); err != nil {
			message = fmt.Sprintf("DLL installed but failed to save database: %v", err)
			msgType = MessageError
		}
	} else if msg.err != nil {
		message = fmt.Sprintf("Install failed: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.stack.Top().Update(msg)
	m.stack.Replace(updated)
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleLaunchGameMsg(msg launchGameMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.success {
		message = "Game launched!"
		msgType = MessageSuccess
	} else if msg.err != nil {
		message = fmt.Sprintf("Launch failed: %v", msg.err)
		msgType = MessageError
	}
	cmds = append(cmds, m.messageBar.SetMessage(message, msgType))
	updated, contentCmd := m.stack.Top().Update(msg)
	m.stack.Replace(updated)
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleRescanGamesMsg(msg rescanGamesMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	if msg.err != nil {
		cmds = append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Rescan failed: %v", msg.err), MessageError))
		return m, cmds
	}
	m.db = msg.db
	games := msg.db.List()
	m.sidebar = m.sidebar.SetGames(games)
	cmds = append(cmds, m.messageBar.SetMessage(
		fmt.Sprintf("Rescan complete: %d games found", len(games)),
		MessageSuccess,
	))
	if cm := m.contentModel(); cm.game != nil && !cm.defaultProfile {
		if refreshed := msg.db.GetGame(cm.game.AppID); refreshed != nil {
			entry := m.contentEntryForGame(refreshed)
			m.stack.Replace(entry)
			cmds = append(cmds, entry.model.LoadDLLUpdates())
		} else {
			entry := m.contentEntryForGame(nil)
			m.stack.Replace(entry)
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
	updated, _ := m.stack.Top().Update(msg)
	m.stack.Replace(updated)
	return m, cmds
}
