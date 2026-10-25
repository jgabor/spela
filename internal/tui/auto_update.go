package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
)

// autoUpdateMsg carries the result of the startup DLL update.
type autoUpdateMsg struct{ result dll.AutoUpdateResult }

// autoUpdateDLLs runs the startup DLL update off the UI thread.
func (m LayoutModel) autoUpdateDLLs() tea.Cmd {
	run := m.services.AutoUpdateDLLs
	if run == nil {
		return nil
	}
	return func() tea.Msg { return autoUpdateMsg{result: run()} }
}

// handleAutoUpdateMsg applies changed games to every view and shows the
// notice. A library that was already current gets no message.
func (m LayoutModel) handleAutoUpdateMsg(msg autoUpdateMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	for _, item := range msg.result.Batch.Items {
		m.pane.content.applyDLLResult(item.Result)
	}
	if msg.result.Batch.Updated > 0 {
		m.synchronizeDLLGameReferences()
		cmds = append(cmds, m.pane.content.LoadDLLUpdates())
	}
	notice := msg.result.Notice()
	if notice == "" {
		return m, cmds
	}
	messageType := MessageSuccess
	if msg.result.Err != nil || msg.result.Batch.Failed > 0 {
		messageType = MessageError
	} else if msg.result.Batch.Updated == 0 {
		messageType = MessageInfo
	}
	return m, append(cmds, m.messageBar.SetMessage(notice, messageType))
}
