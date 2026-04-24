package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m ContentModel) updateBlockingFlow(msg tea.Msg) (ContentModel, tea.Cmd, bool) {
	if m.dlssPresetModal.Visible() {
		var cmd tea.Cmd
		m.dlssPresetModal, cmd = m.dlssPresetModal.Update(msg)
		return m, cmd, true
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
	case "y", "Y":
		action := m.pendingAction
		m.pendingAction = PendingNone
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
	case openDLSSPresetModalMsg:
		m.dlssPresetModal.SetSize(m.width, m.height)
		m.dlssPresetModal.Open(msg.currentPreset)
		return m, nil, true
	case dlssPresetSelectedMsg:
		m.profileWidget.SetDLSSPreset(msg.preset)
		return m, nil, true
	case dlssPresetCancelledMsg:
		return m, nil, true
	case profileSaveMsg:
		return m.updateProfileSaveMsg(msg), nil, true
	case dllUpdateMsg:
		return m.updateDLLUpdateMsg(msg)
	case dllRestoreMsg:
		return m.updateDLLRestoreMsg(msg), nil, true
	case dllUpdatesCheckedMsg:
		if msg.err == nil {
			m.hasUpdates = msg.hasUpdates
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m ContentModel) updateProfileSaveMsg(msg profileSaveMsg) ContentModel {
	if msg.success {
		if m.defaultProfile {
			p, _ := m.services.LoadDefaultProfile()
			m.profile = p
		} else if m.game != nil {
			p, inherited := m.loadEffectiveProfile(m.game.AppID)
			m.profile = p
			m.usingDefaultProfile = inherited
			m.profileHeight = m.profileSectionHeight()
		}
	}
	return m
}

func (m ContentModel) updateDLLUpdateMsg(msg dllUpdateMsg) (ContentModel, tea.Cmd, bool) {
	m.dllOperating = false
	if msg.success && msg.dlls != nil && m.game != nil {
		m.game.DLLs = msg.dlls
		m.game.ScannedAt = time.Now()
	}
	m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
	if msg.success {
		m.hasUpdates = false
		return m, m.LoadDLLUpdates(), true
	}
	return m, nil, true
}

func (m ContentModel) updateDLLRestoreMsg(msg dllRestoreMsg) ContentModel {
	m.dllOperating = false
	if msg.success {
		m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
	}
	return m
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
		if !m.profileWidget.Editing() {
			detail, _, handled := m.detail.Update(msg)
			if handled {
				m.detail = detail
				return true
			}
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
	case "p":
		changed, err := m.detail.PinFocused()
		if err == nil && changed {
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
		if m.game != nil && len(m.game.DLLs) > 0 && m.hasUpdates && !m.dllOperating {
			if m.confirmDestructive {
				m.pendingAction = PendingDLLUpdate
				return m, nil, true
			}
			m.dllOperating = true
			m.dllOperatingLabel = "Updating DLLs..."
			return m, m.updateDLLs(), true
		}
	case "ctrl+shift+r":
		if m.game != nil && m.hasBackup && !m.dllOperating {
			if m.confirmDestructive {
				m.pendingAction = PendingDLLRestore
				return m, nil, true
			}
			m.dllOperating = true
			m.dllOperatingLabel = "Restoring DLLs..."
			return m, m.restoreDLLs(), true
		}
	}
	return m, nil, false
}
