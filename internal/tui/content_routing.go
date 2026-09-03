package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
)

func (m ContentModel) updateBlockingFlow(msg tea.Msg) (ContentModel, tea.Cmd, bool) {
	if m.confirmation != nil {
		key, ok := msg.(tea.KeyPressMsg)
		if !ok {
			return m, nil, true
		}
		confirm, cancel := m.confirmation.update(key)
		if cancel {
			m.confirmation = nil
			if m.pendingAction != PendingNone {
				if m.pendingAction == PendingDLLUpdate {
					m.lastDLLResult = dllCancellationResult("DLL update")
				} else {
					m.lastDLLResult = dllCancellationResult("DLL restore")
				}
				m.pendingAction = PendingNone
			} else {
				m.lastDLLResult = dllCancellationResult("DLL install")
				m.dllInstallState = DLLInstallNone
				m.dllOperating = false
			}
			return m, nil, true
		}
		if !confirm {
			return m, nil, true
		}
		m.confirmation = nil
		if m.pendingAction == PendingNone {
			m.lastDLLResult = ""
			m.dllInstallState = DLLInstallDownloading
			return m, m.installSelectedDLL(), true
		}
	}
	if m.dllInstallState != DLLInstallNone {
		m, cmd := m.updateDLLInstall(msg)
		return m, cmd, true
	}
	if m.pendingAction != PendingNone {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			return m.updatePendingAction(key)
		}
	}
	return m, nil, false
}

func (m ContentModel) updatePendingAction(msg tea.KeyPressMsg) (ContentModel, tea.Cmd, bool) {
	switch msg.String() {
	case "esc", "escape", "q":
		if m.pendingAction == PendingDLLUpdate {
			m.lastDLLResult = dllCancellationResult("DLL update")
		} else {
			m.lastDLLResult = dllCancellationResult("DLL restore")
		}
		m.pendingAction = PendingNone
		return m, nil, true
	case "enter", "y", "Y":
		action := m.pendingAction
		m.pendingAction = PendingNone
		m.lastDLLResult = ""
		switch action {
		case PendingDLLUpdate:
			m.dllOperating = true
			m.dllOperatingLabel = "Updating DLLs..."
			return m, m.updateDLLs(), true
		case PendingDLLRestore:
			m.dllOperating = true
			m.dllOperatingLabel = "Restoring DLLs..."
			return m, m.restoreDLLs(), true
		}
	default:
		m.pendingAction = PendingNone
	}
	return m, nil, true
}

func (m ContentModel) updateContentMessage(msg tea.Msg) (ContentModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case profileSaveMsg:
		if m.game != nil && msg.request.appID == m.game.AppID {
			m.detail.CompleteSave(msg.err)
		}
		if msg.err == nil && m.game != nil && msg.request.appID == m.game.AppID {
			m.persistedProfile = msg.request.desired.Clone()
			m.usingDefaultProfile = false
		}
		return m, m.profileSaves.complete(msg), true
	case dllUpdateMsg:
		return m.updateDLLUpdateMsg(msg)
	case dllRestoreMsg:
		return m.updateDLLRestoreMsg(msg), nil, true
	case dllUpdatesCheckedMsg:
		if msg.err == nil {
			m.hasUpdates = msg.hasUpdates
			m.dllUpdateTargets = msg.targets
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m ContentModel) updateDLLUpdateMsg(msg dllUpdateMsg) (ContentModel, tea.Cmd, bool) {
	m.dllOperating = false
	for _, item := range msg.batch.Items {
		m.applyDLLResult(item.Result)
	}
	m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
	m.hasUpdates = false
	return m, m.LoadDLLUpdates(), true
}

func (m ContentModel) updateDLLRestoreMsg(msg dllRestoreMsg) ContentModel {
	m.dllOperating = false
	if msg.result.Game != nil {
		m.applyDLLResult(msg.result)
		m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
	}
	return m
}

func (m *ContentModel) applyDLLResult(result dll.Result) {
	if result.Game == nil {
		return
	}
	if m.database != nil {
		m.database.Games[result.Game.AppID] = result.Game
	}
	if m.game != nil && m.game.AppID == result.Game.AppID {
		m.game = result.Game
	}
}

func (m ContentModel) updateContentKey(msg tea.KeyPressMsg) (ContentModel, tea.Cmd, bool) {
	if m.updateDetailNavigation(msg) {
		return m, nil, true
	}
	if next, cmd, handled := m.updateProfileKey(msg); handled {
		return next, cmd, true
	}
	return m.updateDLLKey(msg)
}

func (m *ContentModel) updateDetailNavigation(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "j", "down", "k", "up":
		detail, _, handled := m.detail.Update(msg)
		if handled {
			m.detail = detail
			return true
		}
	}
	return false
}

func (m ContentModel) updateProfileKey(msg tea.KeyPressMsg) (ContentModel, tea.Cmd, bool) {
	if m.game == nil || m.dllOperating {
		return m, nil, false
	}
	switch msg.String() {
	case "r":
		changed, err := m.detail.ResetFocused()
		if err == nil && changed {
			return m, m.saveResolvedProfile(), true
		}
		return m, nil, true
	case "R":
		if m.detail.ResetAll() {
			return m, m.saveResolvedProfile(), true
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m ContentModel) updateDLLKey(msg tea.KeyPressMsg) (ContentModel, tea.Cmd, bool) {
	switch msg.String() {
	case "i":
		if m.game != nil && !m.dllOperating {
			m.dllOperating = true
			m.dllOperatingLabel = "Installing DLL..."
			m.dllInstallState = DLLInstallSelectType
			m.dllTypeCursor = 0
			return m, m.loadDLLTypes(), true
		}
	case "u":
		if m.game == nil || len(m.game.DLLs) == 0 || m.dllOperating {
			return m, nil, false
		}
		if !m.hasUpdates {
			m.lastDLLResult = "DLLs already up to date"
			return m, func() tea.Msg {
				return contentNoticeMsg{text: "DLLs already up to date", messageType: MessageInfo}
			}, true
		}
		m.pendingAction = PendingDLLUpdate
		m.confirmation = newDLLMutationConfirmation("Confirm DLL update", m.dllUpdateTargets, "current DLL is backed up before replacement")
		return m, nil, true
	case "f6":
		if m.game != nil && m.hasBackup && !m.dllOperating {
			m.pendingAction = PendingDLLRestore
			targets := make([]dllMutationTarget, 0, len(m.game.DLLs))
			for _, installed := range m.game.DLLs {
				targets = append(targets, newDLLMutationTarget(m.game, installed, "backup"))
			}
			m.confirmation = newDLLMutationConfirmation("Confirm DLL restore", targets, "existing backup is restored; current DLL is not backed up again")
			return m, nil, true
		}
	}
	return m, nil, false
}
