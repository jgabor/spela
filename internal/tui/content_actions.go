package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
)

var gameDLLActions = [...]KeyAction{ActionDetailInstall, ActionDetailUpdate, ActionDetailRestore}

// DLLActionCount includes unavailable controls, so their reasons stay reachable.
func (m ContentModel) DLLActionCount() int {
	if m.game == nil {
		return 0
	}
	return len(gameDLLActions)
}

func (m ContentModel) DLLSelectedAction() KeyAction {
	if m.DLLActionCount() == 0 {
		return ""
	}
	return gameDLLActions[min(max(m.dllActionCursor, 0), len(gameDLLActions)-1)]
}

func (m ContentModel) DLLSelectedActionName() string {
	return gameDLLActionName(m.DLLSelectedAction())
}

func gameDLLActionName(action KeyAction) string {
	switch action {
	case ActionDetailInstall:
		return "Install DLL"
	case ActionDetailUpdate:
		return "Update DLLs"
	case ActionDetailRestore:
		return "Restore originals"
	}
	return ""
}

func (m ContentModel) dllActionTitle(action KeyAction) string {
	name := gameDLLActionName(action)
	switch action {
	case ActionDetailUpdate:
		unit := "files"
		if len(m.dllUpdateTargets) == 1 {
			unit = "file"
		}
		return fmt.Sprintf("%s (%d %s)", name, len(m.dllUpdateTargets), unit)
	case ActionDetailRestore:
		if count, known := m.dllBackupFileCount(); known {
			unit := "files"
			if count == 1 {
				unit = "file"
			}
			return fmt.Sprintf("%s (%d %s)", name, count, unit)
		}
	}
	return name
}

func (m ContentModel) dllBackupFileCount() (int, bool) {
	if m.game == nil || !m.hasBackup {
		return 0, true
	}
	backup, err := m.services.loadDLLBackup(m.game.AppID)
	if err != nil || backup == nil {
		return 0, false
	}
	return len(backup.Files), true
}

// DLLActionAvailability describes the current target and pending state for the
// shell Actions menu. Dispatch checks it again before opening a workflow.
func (m ContentModel) DLLActionAvailability(action KeyAction) (bool, string) {
	if m.game == nil {
		return false, "select a game"
	}
	if m.HasModalOpen() {
		return false, "finish the current DLL operation"
	}
	switch action {
	case ActionDetailInstall:
		return true, ""
	case ActionDetailUpdate:
		if !m.hasUpdates || len(m.dllUpdateTargets) == 0 {
			return false, "no stale DLLs for this game"
		}
		return true, ""
	case ActionDetailRestore:
		if !m.hasBackup {
			return false, "no backup for this game"
		}
		if count, known := m.dllBackupFileCount(); known && count == 0 {
			return false, "backup has no original files"
		}
		return true, ""
	}
	return false, "not a DLL action"
}

func (m ContentModel) DLLActionLabel(action KeyAction) string {
	name := "selected game"
	if m.game != nil {
		name = m.game.Name
	}
	switch action {
	case ActionDetailInstall:
		return "Install DLL: " + name
	case ActionDetailUpdate:
		return fmt.Sprintf("Update stale DLLs: %s (%d)", name, len(m.dllUpdateTargets))
	case ActionDetailRestore:
		label := "Restore originals: " + name
		if count, known := m.dllBackupFileCount(); known {
			unit := "files"
			if count == 1 {
				unit = "file"
			}
			label += fmt.Sprintf(" (%d %s)", count, unit)
		}
		return label
	}
	return ""
}

// DLLOverlayHint describes the active local focus, not commands belonging to
// the underlying Profile or DLL view. It remains essential with hints disabled.
func (m ContentModel) DLLOverlayHint() string {
	if m.confirmation != nil {
		return m.confirmation.hint()
	}
	if m.dllOperating {
		return "Work is running"
	}
	if m.dllResultOpen {
		return dllResultHint(m.lastDLLResult, m.dllInstallControl == 0, m.width, m.height)
	}
	if m.dllInstallState != DLLInstallNone {
		if m.dllInstallControl != 0 {
			return "Enter Activate · Tab Next control"
		}
		if m.dllInstallState == DLLInstallSelectType && len(m.dllTypes) == 0 {
			return "Tab Cancel"
		}
		if m.dllInstallState == DLLInstallSelectVersion && len(m.dllVersions) == 0 {
			return "Tab Back/Cancel"
		}
		return "↑/↓ Select · Enter Choose · Tab Buttons"
	}
	return ""
}

// UpdateDLLAction executes an explicit action; it never synthesizes old shortcut
// keys. Choosers and confirmations own input until dismissed or completed.
func (m ContentModel) UpdateDLLAction(action KeyAction) (ContentModel, tea.Cmd) {
	if m.confirmation != nil {
		confirm, cancel := m.confirmation.updateAction(action)
		if cancel {
			operation := "DLL install"
			switch m.pendingAction {
			case PendingDLLUpdate:
				operation = "DLL update"
			case PendingDLLRestore:
				operation = "DLL restore"
			}
			return m.finishDLLFlow(dllCancellationResult(operation)), nil
		}
		if !confirm || m.dllOperating {
			return m, nil
		}
		if len(m.confirmation.targets) == 0 || m.game == nil {
			return m.finishDLLFlow("DLL operation unavailable: no current target"), nil
		}
		if m.pendingAction == PendingDLLRestore {
			backup, err := m.services.loadDLLBackup(m.game.AppID)
			if err != nil {
				return m.finishDLLFlow("Cannot read DLL backup: " + err.Error()), nil
			}
			if backup == nil || len(backup.Files) == 0 {
				m.hasBackup = false
				return m.finishDLLFlow("DLL restore unavailable: no backup files"), nil
			}
			currentTargets := m.backupMutationTargets(backup)
			if !slices.Equal(currentTargets, m.confirmation.targets) {
				m.confirmation = newDLLMutationConfirmation("Backup changed: review restore", currentTargets, "existing backup is restored; current DLL is not backed up again")
				return m, nil
			}
		}
		targets := append([]dllMutationTarget(nil), m.confirmation.targets...)
		m.confirmation = nil
		m.lastDLLResult = ""
		m.dllOperating = true
		switch m.pendingAction {
		case PendingDLLUpdate:
			m.pendingAction = PendingNone
			m.dllOperatingLabel = "Updating DLLs..."
			m.dllUpdateTargets = targets
			return m, m.updateDLLs()
		case PendingDLLRestore:
			m.pendingAction = PendingNone
			m.dllOperatingLabel = "Restoring DLLs..."
			return m, m.restoreDLLs()
		default:
			m.dllRequestID++
			m.dllInstallState = DLLInstallDownloading
			m.dllOperatingLabel = "Installing DLL..."
			return m, m.installSelectedDLL()
		}
	}
	if m.dllOperating {
		// No cancellation API exists for a dispatched write. Keep its progress
		// visible and reject duplicate dispatch until the completion arrives.
		return m, nil
	}
	if m.dllResultOpen {
		switch action {
		case ActionOverlayFocusNext:
			m.dllInstallControl = (m.dllInstallControl + 1) % 2
		case ActionOverlayPrevious:
			if m.dllInstallControl == 0 {
				m.scrollOffset = max(m.scrollOffset-1, 0)
			}
		case ActionOverlayNext:
			if m.dllInstallControl == 0 {
				m.scrollOffset = min(m.scrollOffset+1, dllResultMaximumScroll(m.lastDLLResult, m.width, m.height))
			}
		case ActionOverlayClose:
			m.dllResultOpen = false
		case ActionOverlayConfirm:
			if m.dllInstallControl == 1 {
				m.dllResultOpen = false
			}
		}
		return m, nil
	}
	if m.dllInstallState != DLLInstallNone {
		return m.updateDLLChooserAction(action)
	}
	switch action {
	case ActionDetailPreviousItem:
		m.dllActionCursor = max(m.dllActionCursor-1, 0)
		return m, nil
	case ActionDetailNextItem:
		m.dllActionCursor = min(m.dllActionCursor+1, max(m.DLLActionCount()-1, 0))
		return m, nil
	case ActionDetailConfirm:
		action = m.DLLSelectedAction()
	}
	if available, _ := m.DLLActionAvailability(action); !available {
		return m, nil
	}
	m.dllInstallControl, m.scrollOffset = 0, 0
	m.lastDLLResult = ""
	switch action {
	case ActionDetailInstall:
		m.dllRequestID++
		m.dllInstallState = DLLInstallSelectType
		m.dllTypeCursor = 0
		m.dllTypes, m.dllVersions = nil, nil
		return m, m.loadDLLTypes()
	case ActionDetailUpdate:
		m.pendingAction = PendingDLLUpdate
		m.confirmation = newDLLMutationConfirmation("Confirm DLL update", m.dllUpdateTargets, "current DLL is backed up before replacement")
	case ActionDetailRestore:
		backup, err := m.services.loadDLLBackup(m.game.AppID)
		if err != nil {
			return m.finishDLLFlow("Cannot read DLL backup: " + err.Error()), nil
		}
		if backup == nil || len(backup.Files) == 0 {
			m.hasBackup = false
			return m.finishDLLFlow("DLL restore unavailable: no backup files"), nil
		}
		m.pendingAction = PendingDLLRestore
		targets := m.backupMutationTargets(backup)
		m.confirmation = newDLLMutationConfirmation("Confirm DLL restore", targets, "existing backup is restored; current DLL is not backed up again")
	}
	return m, nil
}

func (m ContentModel) backupMutationTargets(backup *dll.Backup) []dllMutationTarget {
	targets := make([]dllMutationTarget, 0, len(backup.Files))
	for _, file := range backup.Files {
		family := file.DLLName
		for _, info := range dllDisplayColumns {
			if strings.EqualFold(file.DLLName, info.Filename) {
				family = info.Label
				break
			}
		}
		version := "backup"
		if file.Version != "" {
			version += " " + file.Version
		}
		target := dllMutationTarget{appID: m.game.AppID, gameName: m.game.Name, family: family, path: file.OriginalPath, targetVersion: version}
		for _, installed := range m.game.DLLs {
			if installed.Path == file.OriginalPath {
				target.currentVersion = installed.Version
				break
			}
		}
		targets = append(targets, target)
	}
	return targets
}

func (m ContentModel) finishDLLFlow(message string) ContentModel {
	m.dllRequestID++
	m.confirmation, m.pendingAction = nil, PendingNone
	m.dllOperating, m.dllResultOpen = false, true
	m.dllInstallState, m.dllInstallControl = DLLInstallNone, 0
	m.scrollOffset, m.lastDLLResult = 0, message
	return m
}

func (m ContentModel) updateDLLChooserAction(action KeyAction) (ContentModel, tea.Cmd) {
	controls := 2 // List, Cancel.
	if m.dllInstallState == DLLInstallSelectVersion {
		controls = 3 // List, Back, Cancel.
	}
	switch action {
	case ActionOverlayFocusNext:
		m.dllInstallControl = (m.dllInstallControl + 1) % controls
	case ActionOverlayPrevious:
		if m.dllInstallControl == 0 {
			if m.dllInstallState == DLLInstallSelectType {
				m.dllTypeCursor = max(m.dllTypeCursor-1, 0)
			} else {
				m.dllVersionCursor = max(m.dllVersionCursor-1, 0)
			}
		}
	case ActionOverlayNext:
		if m.dllInstallControl == 0 {
			if m.dllInstallState == DLLInstallSelectType {
				m.dllTypeCursor = min(m.dllTypeCursor+1, max(len(m.dllTypes)-1, 0))
			} else {
				m.dllVersionCursor = min(m.dllVersionCursor+1, max(len(m.dllVersions)-1, 0))
			}
		}
	case ActionOverlayClose:
		return m.finishDLLFlow(dllCancellationResult("DLL install")), nil
	case ActionOverlayConfirm:
		if m.dllInstallControl == controls-1 {
			return m.finishDLLFlow(dllCancellationResult("DLL install")), nil
		}
		if m.dllInstallState == DLLInstallSelectVersion && m.dllInstallControl == 1 {
			m.dllRequestID++
			m.dllInstallState, m.dllInstallControl = DLLInstallSelectType, 0
			return m, nil
		}
		if m.dllInstallState == DLLInstallSelectType && len(m.dllTypes) > 0 {
			m.dllRequestID++
			m.selectedDLLType = m.dllTypes[min(m.dllTypeCursor, len(m.dllTypes)-1)]
			m.dllInstallState, m.dllVersionCursor = DLLInstallSelectVersion, 0
			m.dllVersionsLoaded, m.dllVersions = false, nil
			return m, m.loadDLLVersions()
		}
		if m.dllInstallState == DLLInstallSelectVersion && len(m.dllVersions) > 0 && m.game != nil {
			selected := m.dllVersions[min(m.dllVersionCursor, len(m.dllVersions)-1)]
			path := filepath.Join(m.game.InstallDir, selected.Filename)
			currentVersion := ""
			for _, installed := range m.game.DLLs {
				if installed.Name == selected.Filename {
					path, currentVersion = installed.Path, installed.Version
					break
				}
			}
			m.confirmation = newDLLMutationConfirmation("Confirm DLL install", []dllMutationTarget{{
				appID:          m.game.AppID,
				gameName:       m.game.Name,
				family:         dllFamilyName(m.selectedDLLType),
				manifestKey:    m.selectedDLLType,
				path:           path,
				currentVersion: currentVersion,
				targetVersion:  selected.Version,
			}}, "original DLL is backed up before replacement")
		}
	}
	return m, nil
}

func (m ContentModel) updateDLLInstall(msg tea.Msg) (ContentModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.UpdateDLLAction(dllOverlayAction(msg))
	case dllTypesLoadedMsg:
		if msg.requestID == m.dllRequestID && m.dllInstallState == DLLInstallSelectType {
			m.dllTypes = msg.types
		}
	case dllVersionsLoadedMsg:
		if msg.requestID == m.dllRequestID && m.dllInstallState == DLLInstallSelectVersion {
			m.dllVersions, m.dllVersionsLoaded = msg.versions, true
		}
	case dllInstallMsg:
		if msg.requestID != m.dllRequestID || m.dllInstallState == DLLInstallNone && !m.dllOperating {
			return m, nil
		}
		message := "DLL install completed"
		if msg.err != nil {
			message = "DLL install failed: " + msg.err.Error()
		} else if msg.result.Outcome == dll.OutcomeNoOp {
			message = "DLL install: already current"
		}
		m = m.finishDLLFlow(message)
		if msg.result.Game != nil {
			m.applyDLLResult(msg.result)
			m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
			return m, m.LoadDLLUpdates()
		}
	}
	return m, nil
}
