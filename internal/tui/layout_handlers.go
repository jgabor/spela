package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

// handleBatchMenuKeys handles key input when the batch-action menu is visible.
func (m LayoutModel) handleBatchMenuKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	action := CanonicalKeymap.Lookup(BindingContext{Mode: ModeOverlay}, msg.String()).Binding.Action
	if m.batchBusy {
		return m, nil, true
	}
	if m.batchConfirmation != nil {
		confirm, cancel := m.batchConfirmation.updateAction(action)
		if cancel {
			m.batchConfirmation = nil
			m.batchMessage = dllCancellationResult("Selected-game DLL batch")
			m.batchCloseFocused = false
			m.batchScrollOffset = 0
		}
		if confirm {
			targets := m.batchConfirmation.targets
			m.batchConfirmation = nil
			m.batchBusy = true
			m.batchMessage = "Updating DLLs..."
			return m, m.executeBatchAction(targets), true
		}
		return m, nil, true
	}
	if action == ActionOverlayFocusNext {
		m.batchCloseFocused = !m.batchCloseFocused
	}
	if m.batchMessage != "" && !m.batchCloseFocused {
		maximum := dllResultMaximumScroll(m.batchMessage, max(m.width-12, 1), max(m.height-10, 1))
		switch action {
		case ActionOverlayPrevious:
			m.batchScrollOffset = max(m.batchScrollOffset-1, 0)
		case ActionOverlayNext:
			m.batchScrollOffset = min(m.batchScrollOffset+1, maximum)
		}
	}
	if action == ActionOverlayConfirm {
		if m.batchCloseFocused {
			m.showBatchMenu = false
			m.batchGames = nil
			return m, nil, true
		}
		if m.batchMessage != "" {
			return m, nil, true
		}
		targets := latestDLLMutationTargets(m.batchGames, m.pane.dllsResource.manifest)
		if len(targets) == 0 {
			return m, nil, true
		}
		m.batchConfirmation = newDLLMutationConfirmation("Confirm batch DLL update", targets, "each current DLL is backed up before replacement")
	}
	return m, nil, true
}

// handleHelpKeys handles key input when the help overlay is visible.
func (m LayoutModel) handleHelpKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	action := CanonicalKeymap.Lookup(BindingContext{Mode: ModeOverlay}, msg.String()).Binding.Action
	switch action {
	case ActionOverlayFocusNext:
		m.help.closeFocused = !m.help.closeFocused
	case ActionOverlayConfirm:
		if m.help.closeFocused {
			m.showHelp = false
		}
	case ActionOverlayPrevious:
		if !m.help.closeFocused {
			m.help.Move(-1)
		}
	case ActionOverlayNext:
		if !m.help.closeFocused {
			m.help.Move(1)
		}
	}
	return m, nil, true
}

func (m LayoutModel) handleGlobalKeys(msg tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	next, command := m.routeKey(msg)
	return next, command, true
}

// jumpSection moves along the sections of the open profile page. Left and Right
// move the cursor to the first field of the previous or next section; Right from
// the last section opens the DLLs section of a game, and Left from it returns to
// the Overlay section. Nothing wraps.
func (m LayoutModel) jumpSection(delta int) LayoutModel {
	if m.navState.Destination != nav.DestinationLibrary {
		return m
	}
	game := m.navState.Scope.Kind == nav.ScopeGame
	if m.navState.Aspect == nav.AspectDLLs {
		if game && delta < 0 {
			*m.navState = m.navState.SelectAspect(nav.AspectProfile)
			m.pane.SetState(*m.navState)
			m.pane.content.detail.FocusSection("overlay")
		}
		return m
	}
	if m.pane.profileDetail().JumpSection(delta) {
		return m
	}
	if game && delta > 0 {
		*m.navState = m.navState.SelectAspect(nav.AspectDLLs)
		m.pane.SetState(*m.navState)
	}
	return m
}

func (m LayoutModel) bindingContext() BindingContext { return m.actionContext() }

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
		m.batchScrollOffset = 0

	case batchCompleteMsg:
		m.batchBusy = false
		m.batchScrollOffset = 0
		for _, item := range msg.batch.Items {
			m.pane.content.applyDLLResult(item.Result)
		}
		m.synchronizeDLLGameReferences()
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

	case ownValuesAppliedMsg:
		m, cmds = m.handleOwnValuesApplied(msg, cmds)

	case optionsSavedMsg:
		if msg.config == nil {
			break
		}
		cmds = append(cmds, m.pane.settings.CompleteSave(msg.config))
		m.config = msg.config.Clone()
		cmds = append(cmds, m.messageBar.SetMessage("Saved: "+msg.summary, MessageSuccess))

	case optionsSaveErrorMsg:
		m.pane.settings.FailSave(msg.err)
		m.quitWhenIdle = false
		cmds = append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Not saved: %v", msg.err), MessageError))

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
	m.synchronizeDLLGameReferences()
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
	m.pane.content = updated
	m.synchronizeDLLGameReferences()
	cmds = append(cmds, contentCmd)
	return m, cmds
}

func (m LayoutModel) handleDLLInstallMsg(msg dllInstallMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	if msg.requestID != m.pane.content.dllRequestID || m.pane.content.dllInstallState == DLLInstallNone && !m.pane.content.dllOperating {
		return m, cmds
	}
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
	m.pane.content = updated
	m.synchronizeDLLGameReferences()
	cmds = append(cmds, contentCmd)
	return m, cmds
}

// DLL services replace game records after a mutation. All views must use those
// records without reloading or replacing either profile's working copy.
func (m *LayoutModel) synchronizeDLLGameReferences() {
	if m.db == nil {
		return
	}
	games := m.db.List()
	m.listPane.sidebar = m.listPane.sidebar.SetGames(games)
	m.pane.dllsResource = m.pane.dllsResource.SetGames(games)
	if m.pane.content.game != nil {
		if updated := m.db.GetGame(m.pane.content.game.AppID); updated != nil {
			m.pane.content.game = updated
			m.pane.content.hasBackup = m.services.BackupExists(updated.AppID)
		}
	}
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
	if msg.requestID != m.rescanRequestID {
		return m, cmds
	}
	m.rescanBusy = false
	if msg.err != nil {
		return m, append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Rescan failed: %v", msg.err), MessageError))
	}
	if msg.db == nil {
		return m, append(cmds, m.messageBar.SetMessage("Rescan failed: no database returned", MessageError))
	}
	if m.bindingContext().Mode != ModeBrowse {
		m.pendingRescan = &msg
		return m, cmds
	}
	next := m.listPane.sidebar.cloneSelection().SetGames(msg.db.List())
	apply := func(model LayoutModel) (LayoutModel, tea.Cmd) {
		model.db = msg.db
		model.pane.content.database = msg.db
		model.pane.dllsResource.database = msg.db
		games := msg.db.List()
		manifest, _ := dll.LoadManifest()
		model.pane.SetDLLsData(games, manifest)
		var selectionCommand tea.Cmd
		model, selectionCommand = model.applySidebar(next, false)
		if model.pane.content.game != nil {
			if refreshed := msg.db.GetGame(model.pane.content.game.AppID); refreshed != nil {
				model.pane.content = model.pane.content.SetGame(refreshed)
				selectionCommand = tea.Batch(selectionCommand, model.pane.content.LoadDLLUpdates())
			}
		}
		return model, tea.Batch(selectionCommand, model.messageBar.SetMessage(fmt.Sprintf("Rescan complete: %d games found", len(games)), MessageSuccess))
	}
	nextModel, command := apply(m)
	return nextModel, append(cmds, command)
}

// handleOwnValuesApplied reports what applying All games to the games with
// their own value did, and reads the games again so the count is current.
func (m LayoutModel) handleOwnValuesApplied(msg ownValuesAppliedMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	m.applyingOwnValues = false
	m.pane.refreshOwnValueGames()
	m.pane.defaultsDetail.SetOwnValueGames(m.pane.ownValueGames)
	kind := MessageSuccess
	text := "Saved: " + profile.FollowOutcomeText(msg.label, msg.outcome)
	if len(msg.outcome.Failed) > 0 {
		kind, text = MessageError, profile.FollowOutcomeText(msg.label, msg.outcome)
		// A failed write keeps the program open, so the error can be read.
		m.quitWhenIdle = false
	}
	return m, append(cmds, m.messageBar.SetMessage(text, kind))
}

func (m LayoutModel) handleProfileSaveMsg(msg profileSaveMsg, cmds []tea.Cmd) (LayoutModel, []tea.Cmd) {
	summary := msg.request.summary
	if summary == "" {
		summary = "profile"
	}
	shown := msg.request.appID == 0 || m.pane.content.game != nil && m.pane.content.game.AppID == msg.request.appID
	if !shown && msg.request.desired != nil && msg.request.desired.Name != "" {
		summary = msg.request.desired.Name + ": " + summary
	}
	if msg.err == nil {
		cmds = append(cmds, m.messageBar.SetMessage("Saved: "+summary, MessageSuccess))
	} else {
		cmds = append(cmds, m.messageBar.SetMessage(fmt.Sprintf("Not saved: %s: %v", summary, msg.err), MessageError))
		// A failed write keeps the program open, so the error can be read.
		m.quitWhenIdle = false
	}
	if msg.request.appID == 0 {
		cmds = append(cmds, m.pane.completeDefaultSave(msg))
	} else {
		updated, cmd := m.pane.content.Update(msg)
		m.pane.content = updated
		cmds = append(cmds, cmd)
	}
	return m, cmds
}
