package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/launcher"
	"github.com/jgabor/spela/internal/profile"
)

type DLLInstallState int

const (
	DLLInstallNone DLLInstallState = iota
	DLLInstallSelectType
	DLLInstallSelectVersion
	DLLInstallDownloading
)

type ContentTab int

const (
	TabDLLs ContentTab = iota
	TabProfile
	TabLaunch
)

// Fixed heights for content sections to prevent layout shifts.
const (
	headerSectionHeight = 5 // name + app ID + install + prefix + blank
	dllSectionHeight    = 5 // title + DLL columns (2 rows) + hint + blank
)

// dllDisplayColumns defines the ordered list of DLL types to display and their column headers.
var dllDisplayColumns = []struct {
	dllType    game.DLLType
	columnName string
}{
	{game.DLLTypeDLSS, "DLSS"},
	{game.DLLTypeDLSSG, "DLSS-G"},
	{game.DLLTypeDLSSD, "DLSS-D"},
	{game.DLLTypeXeSS, "XESS"},
	{game.DLLTypeFSR, "FSR"},
}

// PendingAction identifies a destructive operation awaiting confirmation.
type PendingAction int

const (
	PendingNone PendingAction = iota
	PendingDLLUpdate
	PendingDLLRestore
)

type ContentModel struct {
	styles              *Styles
	services            *Services
	game                *game.Game
	defaultProfile      bool
	profile             *profile.Profile
	profileWidget       ProfileWidgetModel
	dlssPresetModal     DLSSPresetModalModel
	activeTab           ContentTab
	confirmDestructive  bool
	pendingAction       PendingAction
	width               int
	height              int
	profileHeight       int
	dllOperating        bool
	dllOperatingLabel   string
	hasBackup           bool
	hasUpdates          bool
	usingDefaultProfile bool
	launching           bool
	scrollOffset        int

	dllInstallState   DLLInstallState
	dllTypes          []string
	dllTypeCursor     int
	dllVersions       []dll.DLL
	dllVersionCursor  int
	dllVersionsLoaded bool
	selectedDLLType   string
}

type dllUpdateMsg struct {
	success bool
	err     error
	dlls    []game.DetectedDLL
}

type dllRestoreMsg struct {
	success bool
	err     error
}

type dllUpdatesCheckedMsg struct {
	hasUpdates bool
	err        error
}

type launchGameMsg struct {
	success bool
	err     error
}

type dllInstallMsg struct {
	success bool
	err     error
	dlls    []game.DetectedDLL
}

type dllTypesLoadedMsg struct {
	types []string
}

// Name returns the display name for breadcrumb rendering.
func (m ContentModel) Name() string {
	if m.defaultProfile {
		return "Default Profile"
	}
	if m.game != nil {
		return m.game.Name
	}
	return "Details"
}

func NewContent(styles *Styles, confirmDestructive bool, svc *Services) ContentModel {
	return ContentModel{
		styles:             styles,
		services:           svc,
		confirmDestructive: confirmDestructive,
		dlssPresetModal:    NewDLSSPresetModal(styles),
	}
}

func (m ContentModel) SetGame(g *game.Game) ContentModel {
	m.game = g
	m.defaultProfile = false
	m.activeTab = TabDLLs
	m.dllOperating = false
	m.scrollOffset = 0
	m.dllInstallState = DLLInstallNone
	m.profileHeight = m.profileSectionHeight()
	m.hasUpdates = false
	m.usingDefaultProfile = false
	m.launching = false

	if g != nil {
		p, inherited := m.loadEffectiveProfile(g.AppID)
		m.profile = p
		m.usingDefaultProfile = inherited
		m.profileWidget = NewProfileWidget(g, p, m.styles)
		if m.services != nil && m.services.VKD3DNotice != nil {
			appID := g.AppID
			noticeFn := m.services.VKD3DNotice
			m.profileWidget.SetVKD3DNoticeSource(func() string { return noticeFn(appID) })
		}
		m.hasBackup = m.services.BackupExists(g.AppID)
	}

	return m
}

func (m ContentModel) SetDefaultProfile() ContentModel {
	m.game = nil
	m.defaultProfile = true
	m.dllOperating = false
	m.scrollOffset = 0
	m.dllInstallState = DLLInstallNone
	m.hasBackup = false
	m.profileHeight = m.profileSectionHeight()
	m.hasUpdates = false
	m.usingDefaultProfile = false
	m.launching = false

	p, _ := m.services.LoadDefaultProfile()
	m.profile = p
	m.profileWidget = NewDefaultProfileWidget(p, m.styles)
	if m.profile == nil {
		m.profile = m.profileWidget.profile
	}

	return m
}

func (m *ContentModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.profileHeight = m.profileSectionHeight()
}

func (m ContentModel) profileSectionHeight() int {
	if m.defaultProfile {
		return max(m.height-3, 5)
	}
	return m.tabContentHeight()
}

func (m ContentModel) tabContentHeight() int {
	// Total height minus game info header minus tab bar (2 lines: bar + blank).
	used := headerSectionHeight + 2
	return max(m.height-used, 5)
}

func (m ContentModel) Update(msg tea.Msg) (ContentModel, tea.Cmd) {
	var cmds []tea.Cmd

	if m.dlssPresetModal.Visible() {
		var cmd tea.Cmd
		m.dlssPresetModal, cmd = m.dlssPresetModal.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	if m.dllInstallState != DLLInstallNone {
		return m.updateDLLInstall(msg)
	}

	// Handle pending confirmation for destructive operations.
	if m.pendingAction != PendingNone {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				action := m.pendingAction
				m.pendingAction = PendingNone
				switch action {
				case PendingDLLUpdate:
					m.dllOperating = true
					m.dllOperatingLabel = "Updating DLLs..."
					return m, m.updateDLLs()
				case PendingDLLRestore:
					m.dllOperating = true
					m.dllOperatingLabel = "Restoring DLLs..."
					return m, m.restoreDLLs()
				}
			default:
				// Any other key cancels.
				m.pendingAction = PendingNone
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case openDLSSPresetModalMsg:
		m.dlssPresetModal.SetSize(m.width, m.height)
		m.dlssPresetModal.Open(msg.currentPreset)
		return m, nil

	case dlssPresetSelectedMsg:
		m.profileWidget.SetDLSSPreset(msg.preset)
		return m, nil

	case dlssPresetCancelledMsg:
		return m, nil

	case tea.KeyPressMsg:
		// Tab switching (only when no modal/editing is active).
		if m.game != nil && !m.defaultProfile && !m.profileWidget.Editing() {
			switch msg.String() {
			case "2":
				m.activeTab = TabDLLs
				return m, nil
			case "3":
				m.activeTab = TabProfile
				return m, nil
			case "4":
				m.activeTab = TabLaunch
				return m, nil
			}
		}

		switch msg.String() {
		case "L":
			if m.game != nil && !m.defaultProfile && !m.launching {
				m.launching = true
				return m, m.launchGame()
			}
		case "i":
			if m.game != nil && !m.dllOperating {
				m.dllOperating = true
				m.dllOperatingLabel = "Installing DLL..."
				m.dllInstallState = DLLInstallSelectType
				m.dllTypeCursor = 0
				return m, m.loadDLLTypes()
			}
		case "u":
			if m.game != nil && len(m.game.DLLs) > 0 && m.hasUpdates && !m.dllOperating {
				if m.confirmDestructive {
					m.pendingAction = PendingDLLUpdate
					return m, nil
				}
				m.dllOperating = true
				m.dllOperatingLabel = "Updating DLLs..."
				return m, m.updateDLLs()
			}
		case "R":
			if m.game != nil && m.hasBackup && !m.dllOperating {
				if m.confirmDestructive {
					m.pendingAction = PendingDLLRestore
					return m, nil
				}
				m.dllOperating = true
				m.dllOperatingLabel = "Restoring DLLs..."
				return m, m.restoreDLLs()
			}
		}

	case profileSaveMsg:
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
		return m, nil

	case dllUpdateMsg:
		m.dllOperating = false
		if msg.success && msg.dlls != nil && m.game != nil {
			m.game.DLLs = msg.dlls
			m.game.ScannedAt = time.Now()
		}
		m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
		if msg.success {
			m.hasUpdates = false
			return m, m.LoadDLLUpdates()
		}
		return m, nil

	case dllRestoreMsg:
		m.dllOperating = false
		if msg.success {
			m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
		}
		return m, nil

	case dllUpdatesCheckedMsg:
		if msg.err == nil {
			m.hasUpdates = msg.hasUpdates
		}
		return m, nil

	case launchGameMsg:
		m.launching = false
		return m, nil
	}

	var cmd tea.Cmd
	m.profileWidget, cmd = m.profileWidget.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ContentModel) updateDLLs() tea.Cmd {
	g := m.game
	return func() tea.Msg {
		if g == nil || len(g.DLLs) == 0 {
			return dllUpdateMsg{err: fmt.Errorf("no game or DLLs selected")}
		}

		manifest, err := dll.LoadManifest()
		if err != nil {
			return dllUpdateMsg{err: fmt.Errorf("failed to load manifest: %w", err)}
		}
		if manifest == nil {
			manifest, err = dll.UpdateManifest("")
			if err != nil {
				return dllUpdateMsg{err: fmt.Errorf("failed to fetch manifest: %w", err)}
			}
		}

		gameDLLs := dll.GameDLLsFromDetected(g.DLLs)

		updatedCount := 0
		for _, d := range g.DLLs {
			dllType := strings.ToLower(string(d.Type))
			latest := manifest.GetLatestDLL(dllType)
			if latest == nil {
				continue
			}

			if d.Version != "" && !dll.IsNewer(d.Version, latest.Version) {
				continue
			}

			cachePath, err := dll.DownloadDLL(latest, dllType)
			if err != nil {
				return dllUpdateMsg{err: fmt.Errorf("download %s failed: %w", dllType, err)}
			}

			if err := dll.SwapDLL(g.AppID, g.Name, gameDLLs, d.Name, cachePath); err != nil {
				return dllUpdateMsg{err: fmt.Errorf("swap %s failed: %w", dllType, err)}
			}
			updatedCount++
		}

		if updatedCount == 0 {
			return dllUpdateMsg{err: fmt.Errorf("no updates available")}
		}

		detected, err := dll.ScanDirectory(g.InstallDir)
		if err != nil {
			return dllUpdateMsg{err: err}
		}

		return dllUpdateMsg{success: true, dlls: detected}
	}
}

func (m ContentModel) restoreDLLs() tea.Cmd {
	g := m.game
	return func() tea.Msg {
		if g == nil {
			return dllRestoreMsg{err: fmt.Errorf("no game selected")}
		}

		if err := dll.RestoreBackup(g.AppID); err != nil {
			return dllRestoreMsg{err: err}
		}

		return dllRestoreMsg{success: true}
	}
}

func (m ContentModel) HasModalOpen() bool {
	return m.dlssPresetModal.Visible() || m.dllInstallState != DLLInstallNone || m.profileWidget.Editing() || m.pendingAction != PendingNone
}

func (m ContentModel) HasGameSelection() bool {
	return m.game != nil
}

func (m ContentModel) View() string {
	if m.dlssPresetModal.Visible() {
		return m.dlssPresetModal.View()
	}

	if m.dllInstallState != DLLInstallNone {
		return m.renderDLLInstallDialog()
	}

	if m.defaultProfile {
		return m.renderDefaultProfile()
	}

	if m.game == nil {
		return m.styles.Dim.Render("Select a game from the sidebar")
	}

	var b strings.Builder

	b.WriteString(m.renderGameInfo())
	b.WriteString("\n")
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	switch m.activeTab {
	case TabDLLs:
		b.WriteString(m.renderDLLs())
	case TabProfile:
		b.WriteString(m.renderProfile())
	case TabLaunch:
		b.WriteString(m.renderLaunch())
	}

	return b.String()
}

func (m ContentModel) loadEffectiveProfile(appID uint64) (*profile.Profile, bool) {
	p, _ := m.services.LoadProfile(appID)
	if p != nil {
		return p, false
	}
	defaultProfile, _ := m.services.LoadDefaultProfile()
	if defaultProfile == nil {
		return nil, false
	}
	return defaultProfile, true
}

func (m ContentModel) loadDLLTypes() tea.Cmd {
	g := m.game
	return func() tea.Msg {
		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllInstallMsg{err: err}
		}

		validTypes := make(map[string]bool, len(g.DLLs))
		for _, d := range g.DLLs {
			validTypes[strings.ToLower(string(d.Type))] = true
		}

		allTypes := manifest.ListDLLNames()
		filteredTypes := make([]string, 0, len(allTypes))
		for _, t := range allTypes {
			if len(manifest.DLLs[t]) == 0 {
				continue
			}
			if len(validTypes) > 0 && !validTypes[t] {
				continue
			}
			filteredTypes = append(filteredTypes, t)
		}

		if len(filteredTypes) == 0 {
			return dllInstallMsg{err: fmt.Errorf("no supported DLL types detected in game")}
		}

		return dllTypesLoadedMsg{types: filteredTypes}
	}
}

func (m ContentModel) updateDLLInstall(msg tea.Msg) (ContentModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			m.dllInstallState = DLLInstallNone
			m.dllOperating = false
			return m, nil
		case "up", "k":
			if m.dllInstallState == DLLInstallSelectType && m.dllTypeCursor > 0 {
				m.dllTypeCursor--
			} else if m.dllInstallState == DLLInstallSelectVersion && m.dllVersionCursor > 0 {
				m.dllVersionCursor--
			}
		case "down", "j":
			if m.dllInstallState == DLLInstallSelectType && m.dllTypeCursor < len(m.dllTypes)-1 {
				m.dllTypeCursor++
			} else if m.dllInstallState == DLLInstallSelectVersion && m.dllVersionCursor < len(m.dllVersions)-1 {
				m.dllVersionCursor++
			}
		case "enter":
			if m.dllInstallState == DLLInstallSelectType && len(m.dllTypes) > 0 {
				m.selectedDLLType = m.dllTypes[m.dllTypeCursor]
				m.dllInstallState = DLLInstallSelectVersion
				m.dllVersionCursor = 0
				m.dllVersionsLoaded = false
				return m, m.loadDLLVersions()
			} else if m.dllInstallState == DLLInstallSelectVersion && len(m.dllVersions) > 0 {
				m.dllInstallState = DLLInstallDownloading
				return m, m.installSelectedDLL()
			}
		}

	case dllTypesLoadedMsg:
		m.dllTypes = msg.types
		return m, nil

	case dllInstallMsg:
		m.dllInstallState = DLLInstallNone
		m.dllOperating = false
		if msg.success {
			if msg.dlls != nil && m.game != nil {
				m.game.DLLs = msg.dlls
				m.game.ScannedAt = time.Now()
			}
			m.hasBackup = m.game != nil && m.services.BackupExists(m.game.AppID)
			return m, m.LoadDLLUpdates()
		}
		return m, nil

	case dllVersionsLoadedMsg:
		m.dllVersions = msg.versions
		m.dllVersionsLoaded = true
		return m, nil
	}

	return m, nil
}

func (m ContentModel) loadDLLVersions() tea.Cmd {
	dllType := m.selectedDLLType
	return func() tea.Msg {
		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllInstallMsg{err: err}
		}
		versions := manifest.DLLs[dllType]
		return dllVersionsLoadedMsg{versions: versions}
	}
}

func (m ContentModel) LoadDLLUpdates() tea.Cmd {
	g := m.game
	return func() tea.Msg {
		if g == nil || len(g.DLLs) == 0 {
			return dllUpdatesCheckedMsg{hasUpdates: false}
		}

		manifest, err := dll.LoadManifest()
		if err != nil {
			return dllUpdatesCheckedMsg{err: fmt.Errorf("failed to load manifest: %w", err)}
		}
		if manifest == nil {
			manifest, err = dll.UpdateManifest("")
			if err != nil {
				return dllUpdatesCheckedMsg{err: fmt.Errorf("failed to fetch manifest: %w", err)}
			}
		}

		for _, d := range g.DLLs {
			dllType := strings.ToLower(string(d.Type))
			latest := manifest.GetLatestDLL(dllType)
			if latest == nil {
				continue
			}
			if d.Version != "" && !dll.IsNewer(d.Version, latest.Version) {
				continue
			}
			return dllUpdatesCheckedMsg{hasUpdates: true}
		}

		return dllUpdatesCheckedMsg{hasUpdates: false}
	}
}

func (m ContentModel) launchGame() tea.Cmd {
	g := m.game
	return func() tea.Msg {
		if g == nil {
			return launchGameMsg{err: fmt.Errorf("no game selected")}
		}

		p, _ := profile.LoadEffective(g.AppID)

		l := launcher.New(g)
		l.Profile = p
		l.Prepare()

		steamURL := fmt.Sprintf("steam://rungameid/%d", g.AppID)
		if err := l.Launch([]string{"steam", steamURL}); err != nil {
			return launchGameMsg{err: fmt.Errorf("failed to launch game: %w", err)}
		}

		return launchGameMsg{success: true}
	}
}

type dllVersionsLoadedMsg struct {
	versions []dll.DLL
}

func (m ContentModel) installSelectedDLL() tea.Cmd {
	dllType := m.selectedDLLType
	dllInfo := m.dllVersions[m.dllVersionCursor]
	g := m.game

	return func() tea.Msg {
		cachePath, err := dll.DownloadDLL(&dllInfo, dllType)
		if err != nil {
			return dllInstallMsg{err: err}
		}

		gameDLLs := dll.GameDLLsFromDetected(g.DLLs)

		targetName := dllInfo.Filename
		if err := dll.InstallDLL(g.AppID, g.Name, g.InstallDir, gameDLLs, targetName, cachePath); err != nil {
			return dllInstallMsg{err: err}
		}

		detected, err := dll.ScanDirectory(g.InstallDir)
		if err != nil {
			return dllInstallMsg{err: err}
		}

		return dllInstallMsg{success: true, dlls: detected}
	}
}
