package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type DLLInstallState int

const (
	DLLInstallNone DLLInstallState = iota
	DLLInstallSelectType
	DLLInstallSelectVersion
	DLLInstallDownloading
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

// ContentModel renders the detail for the Games resource: the currently
// selected game's info, its detected DLLs, and its profile. The per-game
// detail is the only resource that has substantive content in Task 3;
// Tasks 4-5 flesh out profile inheritance rendering and Task 6 expands the
// DLLs and Metrics resources. The Launch-tab surface and ContentTab enum
// were removed as part of Task 3 (the shell redesign).
//
// Task 4 replaces the per-group ProfileWidget grid with a single-column
// grouped DetailModel rendering resolved (inheritance-aware) values. The
// legacy ProfileWidget is retained for now to keep the existing modal and
// save pipeline intact while Task 5 replaces in-place editing with r/p/
// shift+r bindings on the DetailModel.
type ContentModel struct {
	styles              *Styles
	services            *Services
	game                *game.Game
	defaultProfile      bool
	profile             *profile.Profile
	profileWidget       ProfileWidgetModel
	detail              DetailModel
	dlssPresetModal     DLSSPresetModalModel
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
	m.dllOperating = false
	m.scrollOffset = 0
	m.dllInstallState = DLLInstallNone
	m.profileHeight = m.profileSectionHeight()
	m.hasUpdates = false
	m.usingDefaultProfile = false

	if g != nil {
		rawProfile, _ := m.services.LoadProfile(g.AppID)
		defaults, _ := m.services.LoadDefaultProfile()
		m.usingDefaultProfile = rawProfile == nil
		widgetProfile := rawProfile
		if widgetProfile == nil {
			widgetProfile = defaults
		}
		m.profile = widgetProfile
		m.profileWidget = NewProfileWidget(g, widgetProfile, m.styles)
		if m.services != nil && m.services.VKD3DNotice != nil {
			appID := g.AppID
			noticeFn := m.services.VKD3DNotice
			m.profileWidget.SetVKD3DNoticeSource(func() string { return noticeFn(appID) })
		}
		m.detail = NewDetail(m.styles, rawProfile, defaults)
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

	p, _ := m.services.LoadDefaultProfile()
	m.profile = p
	m.profileWidget = NewDefaultProfileWidget(p, m.styles)
	if m.profile == nil {
		m.profile = m.profileWidget.profile
	}
	m.detail = NewRootDetail(m.styles, m.profile)

	return m
}

func (m *ContentModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.profileHeight = m.profileSectionHeight()
}

// profileSectionHeight returns the space allotted to the profile widget
// inside the detail view.
func (m ContentModel) profileSectionHeight() int {
	if m.defaultProfile {
		return max(m.height-3, 5)
	}
	// Game detail: header + dll section + blank
	used := headerSectionHeight + dllSectionHeight + 1
	return max(m.height-used, 5)
}

func (m ContentModel) Update(msg tea.Msg) (ContentModel, tea.Cmd) {
	if next, cmd, handled := m.updateBlockingFlow(msg); handled {
		return next, cmd
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if next, cmd, handled := m.updateContentKey(key); handled {
			return next, cmd
		}
	} else if next, cmd, handled := m.updateContentMessage(msg); handled {
		return next, cmd
	}

	var cmd tea.Cmd
	m.profileWidget, cmd = m.profileWidget.Update(msg)
	return m, cmd
}

// saveResolvedProfile emits a save command for the current game's raw
// profile after a Task 5 binding (r/R/p) has mutated it. The save target
// is the game profile on disk; the resolved inheritance view refreshes on
// the profileSaveMsg round-trip (same pipeline as the legacy widget save).
func (m ContentModel) saveResolvedProfile() tea.Cmd {
	if m.game == nil {
		return nil
	}
	appID := m.game.AppID
	raw := m.detail.RawProfile()
	if raw == nil {
		return nil
	}
	// Ensure the profile carries the game name so saving a freshly-inherited
	// profile doesn't lose identity metadata.
	if raw.Name == "" && m.game != nil {
		raw.Name = m.game.Name
	}
	toSave := *raw
	return func() tea.Msg {
		if err := profile.Save(appID, &toSave); err != nil {
			return profileSaveMsg{err: err}
		}
		return profileSaveMsg{success: true}
	}
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
		return m.styles.Dim.Render("Select a game from the list")
	}

	var b strings.Builder

	b.WriteString(m.renderGameInfo())
	b.WriteString("\n")
	b.WriteString(m.renderDLLs())
	b.WriteString("\n")
	b.WriteString(m.renderProfile())

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
