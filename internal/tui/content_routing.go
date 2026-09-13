package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
)

func (m ContentModel) updateBlockingFlow(msg tea.Msg) (ContentModel, tea.Cmd, bool) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if m.HasModalOpen() {
			next, command := m.UpdateDLLAction(dllOverlayAction(key))
			return next, command, true
		}
		return m, nil, false
	}
	// Completions must reach their owner even while a dialog or progress view
	// owns keyboard input. Otherwise a dispatched write can remain busy forever.
	if next, command, handled := m.updateContentMessage(msg); handled {
		return next, command, true
	}
	switch msg.(type) {
	case dllTypesLoadedMsg, dllVersionsLoadedMsg, dllInstallMsg:
		next, command := m.updateDLLInstall(msg)
		return next, command, true
	}
	return m, nil, false
}

func (m ContentModel) updateContentMessage(msg tea.Msg) (ContentModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case profileSaveMsg:
		if m.game != nil && msg.request.appID == m.game.AppID {
			m.detail.CompleteSaveSnapshot(msg.request.desired, msg.err)
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
		if m.game == nil || msg.appID != m.game.AppID {
			return m, nil, true
		}
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
	m.dllResultOpen, m.dllInstallControl, m.scrollOffset = true, 0, 0
	if msg.err != nil {
		m.lastDLLResult = "DLL update failed: " + msg.err.Error()
	} else {
		m.lastDLLResult = fmt.Sprintf("DLL update: %d updated, %d current, %d failed", msg.batch.Updated, msg.batch.Unchanged, msg.batch.Failed)
	}
	for _, item := range msg.batch.Items {
		if item.Err != nil {
			m.lastDLLResult += "\n" + item.Err.Error()
		}
		m.applyDLLResult(item.Result)
	}
	m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
	m.hasUpdates = false
	return m, m.LoadDLLUpdates(), true
}

func (m ContentModel) updateDLLRestoreMsg(msg dllRestoreMsg) ContentModel {
	m.dllOperating = false
	m.dllResultOpen, m.dllInstallControl, m.scrollOffset = true, 0, 0
	if msg.err != nil {
		m.lastDLLResult = "DLL restore failed: " + msg.err.Error()
	} else if msg.result.Outcome == dll.OutcomeNoOp {
		m.lastDLLResult = "DLL restore: already current"
	} else {
		m.lastDLLResult = "DLL restore completed"
	}
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

// Browse commands are semantic actions owned by resourcePaneModel. Content
// only consumes basic local overlay controls through updateBlockingFlow.
func (m ContentModel) updateContentKey(_ tea.KeyPressMsg) (ContentModel, tea.Cmd, bool) {
	return m, nil, false
}
