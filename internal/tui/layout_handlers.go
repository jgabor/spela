package tui

import (
	"fmt"
	"strings"

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

func (m LayoutModel) handleGlobalKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	if next, cmd, handled := m.handleSystemKey(msg); handled {
		return next, cmd, true
	}
	if next, cmd, handled := m.handleRailHotkey(msg); handled {
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
	switch msg.String() {
	case "1", "2", "3", "4":
		if m.pane.HasModalOpen(m.rail.Active()) {
			return m, nil, false
		}
		rail := m.rail
		if rail.SelectHotkey(msg.String()) {
			m.rail = rail
			m.railFocused = true
			m.pane.SetInnerFocused(false)
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m LayoutModel) handleFocusAndResourceKey(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch msg.String() {
	case "o":
		if m.railFocused {
			m.optionsModal.SetSize(m.width, m.height)
			m.optionsModal.Open(m.config)
			m.activeDialog = &m.optionsModal
			return m, nil, true
		}
	case "ctrl+f":
		if m.rail.Active() == ResourceGames {
			m.railFocused = false
			m.pane.SetInnerFocused(false)
			sidebar, cmd := m.pane.sidebar.FocusSearch()
			m.pane.sidebar = sidebar
			return m, cmd, true
		}
	case "ctrl+r":
		messageCmd := m.messageBar.SetMessage("Rescanning games...", MessageInfo)
		return m, tea.Batch(messageCmd, m.rescanGames()), true
	case "q":
		return m.handleBackOrQuitKey()
	case "esc":
		return m.handleBackKey()
	case "tab":
		return m.handleTabKey(), nil, true
	}
	return m, nil, false
}

func (m LayoutModel) handleBackOrQuitKey() (LayoutModel, tea.Cmd, bool) {
	if m.railFocused {
		return m, tea.Quit, true
	}
	return m.handleBackKey()
}

func (m LayoutModel) handleBackKey() (LayoutModel, tea.Cmd, bool) {
	if m.railFocused || m.pane.HasModalOpen(m.rail.Active()) {
		return m, nil, false
	}
	if m.rail.Active() == ResourceGames && m.pane.InnerFocused() {
		m.pane.SetInnerFocused(false)
		return m, nil, true
	}
	m.railFocused = true
	m.pane.SetInnerFocused(false)
	return m, nil, true
}

func (m LayoutModel) handleTabKey() LayoutModel {
	if m.railFocused {
		m.railFocused = false
		switch m.rail.Active() {
		case ResourceDefaults, ResourceDLLs, ResourceMetrics:
			m.pane.SetInnerFocused(true)
		}
		return m
	}
	if m.rail.Active() == ResourceGames {
		m.pane.SetInnerFocused(!m.pane.InnerFocused())
		return m
	}
	m.railFocused = true
	m.pane.SetInnerFocused(false)
	return m
}

// handleAppMessages routes application-level messages that affect multiple components.
// It takes the current cmds accumulator and returns an updated model and cmds slice.
func (m LayoutModel) handleAppMessages(msg tea.Msg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	switch msg := msg.(type) {
	case gameSelectedMsg:
		content := m.contentForGame(msg.game)
		m.pane.content = content
		m.pane.content.SetSize(m.paneWidth(), m.paneHeight())
		cmds = append(cmds, m.pane.content.LoadDLLUpdates())

	case gameConfirmedMsg:
		content := m.contentForGame(msg.game)
		m.pane.content = content
		m.pane.content.SetSize(m.paneWidth(), m.paneHeight())
		m.pane.SetInnerFocused(true)
		m.railFocused = false
		cmds = append(cmds, m.pane.content.LoadDLLUpdates())

	case defaultProfileSelectedMsg:
		// Defaults is now its own rail resource; sidebar no longer emits
		// this for Task 3, but we keep the handler as a no-op to stay
		// compatible with legacy tests that still dispatch it.
		// (Task 4 will wire the Defaults resource for real.)

	case defaultProfileConfirmedMsg:
		// Same as above — no-op in Task 3.

	case batchActionRequestMsg:
		m.showBatchMenu = true
		m.batchGames = msg.selected
		m.batchCursor = 0
		m.batchMessage = ""

	case batchCompleteMsg:
		m.batchMessage = msg.message
		cmds = append(cmds, m.messageBar.SetMessage(msg.message, MessageSuccess))

	case dllsUpdateAllCompleteMsg:
		// Refresh the in-memory games list in case DLL versions changed,
		// then flash a summary in the message bar. The DLLs resource also
		// handles this message in its own Update to refresh the cached
		// version index; routing both paths keeps responsibilities local.
		msgType := MessageSuccess
		for _, v := range msg.results {
			if strings.HasPrefix(v, "err:") {
				msgType = MessageError
				break
			}
		}
		cmds = append(cmds, m.messageBar.SetMessage(msg.summary, msgType))
		if err := m.db.Save(); err != nil {
			cmds = append(cmds, m.messageBar.SetMessage(
				fmt.Sprintf("Update-all: saved partial state: %v", err), MessageError))
		}

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
	updated, contentCmd := m.pane.content.Update(msg)
	m.pane.content = updated
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLRestoreMsg(msg dllRestoreMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	var msgType MessageType
	var message string
	if msg.success {
		message = "Original DLLs restored!"
		msgType = MessageSuccess
		if cm := m.contentModel(); cm != nil && cm.game != nil {
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
	updated, contentCmd := m.pane.content.Update(msg)
	m.pane.content = updated
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
	updated, contentCmd := m.pane.content.Update(msg)
	m.pane.content = updated
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
	m.pane.sidebar = m.pane.sidebar.SetGames(games)
	// Refresh the DLLs resource so its library + deployment sections
	// reflect any newly detected DLLs after a rescan.
	manifest, _ := dll.LoadManifest()
	m.pane.SetDLLsData(games, manifest)
	cmds = append(cmds, m.messageBar.SetMessage(
		fmt.Sprintf("Rescan complete: %d games found", len(games)),
		MessageSuccess,
	))
	if cm := m.contentModel(); cm != nil && cm.game != nil && !cm.defaultProfile {
		if refreshed := msg.db.GetGame(cm.game.AppID); refreshed != nil {
			content := m.contentForGame(refreshed)
			m.pane.content = content
			cmds = append(cmds, m.pane.content.LoadDLLUpdates())
		} else {
			content := m.contentForGame(nil)
			m.pane.content = content
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
	// Refresh the defaults-root DetailModel so Defaults view reflects any
	// newly saved defaults immediately. Cheap (one reflective profile read).
	m.pane.refreshDefaultsDetail()
	return m, cmds
}
