package tui

import (
	"fmt"
	"path/filepath"
	"strings"

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

// dllDisplayColumns is the canonical ordered family catalog used by every TUI view.
var dllDisplayColumns = dll.KnownDLLTypes()

func dllFamilyName(manifestKey string) string {
	for _, info := range dllDisplayColumns {
		if info.ManifestKey == strings.ToLower(manifestKey) {
			return info.Label
		}
	}
	return manifestKey
}

// PendingAction identifies a destructive operation awaiting confirmation.
type PendingAction int

const (
	PendingNone PendingAction = iota
	PendingDLLUpdate
	PendingDLLRestore
)

// ContentModel renders the detail for the Games resource: the currently
// selected game's info, its detected DLLs, and its profile.
type ContentModel struct {
	styles              *Styles
	services            *Services
	database            *game.Database
	game                *game.Game
	detail              DetailModel
	persistedProfile    *profile.Profile
	profileSaves        *profileSaveState
	confirmDestructive  bool // persisted compatibility; TUI mutations always confirm
	pendingAction       PendingAction
	confirmation        *dllMutationConfirmation
	width               int
	height              int
	dllOperating        bool
	dllOperatingLabel   string
	lastDLLResult       string
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
	dllUpdateTargets  []dllMutationTarget
}

type contentNoticeMsg struct {
	text        string
	messageType MessageType
}

type profileSaveMsg struct {
	err     error
	request profileSaveRequest
}

type profileSaveRequest struct {
	appID           uint64
	before, desired *profile.Profile
}

type profileSaveState struct {
	active *profileSaveRequest
	queued []profileSaveRequest
}

func (s *profileSaveState) start(request profileSaveRequest) tea.Cmd {
	if s.active != nil {
		last := len(s.queued) - 1
		if last >= 0 && s.queued[last].appID == request.appID {
			request.before = s.queued[last].before
			s.queued[last] = request
		} else {
			s.queued = append(s.queued, request)
		}
		return nil
	}
	s.active = &request
	return func() tea.Msg {
		var err error
		merge := func(current *profile.Profile) error {
			return profile.MergeChanges(current, request.before, request.desired)
		}
		if request.appID == 0 {
			err = profile.MutateDefault(merge)
		} else {
			err = profile.Mutate(request.appID, func(current, _ *profile.Profile) error {
				return merge(current)
			})
		}
		return profileSaveMsg{err: err, request: request}
	}
}

func (s *profileSaveState) complete(message profileSaveMsg) tea.Cmd {
	s.active = nil
	if message.err != nil {
		for index := range s.queued {
			if s.queued[index].appID == message.request.appID {
				s.queued[index].before = message.request.before.Clone()
				break
			}
		}
	}
	if len(s.queued) == 0 {
		return nil
	}
	request := s.queued[0]
	s.queued = s.queued[1:]
	return s.start(request)
}

func (s *profileSaveState) latest(appID uint64) *profile.Profile {
	for index := len(s.queued) - 1; index >= 0; index-- {
		if s.queued[index].appID == appID {
			return s.queued[index].desired
		}
	}
	if s.active != nil && s.active.appID == appID {
		return s.active.desired
	}
	return nil
}

type dllUpdateMsg struct {
	batch dll.BatchResult
	err   error
}

type dllRestoreMsg struct {
	result dll.Result
	err    error
}

type dllUpdatesCheckedMsg struct {
	hasUpdates bool
	targets    []dllMutationTarget
	err        error
}

type dllInstallMsg struct {
	result dll.Result
	err    error
}

type dllTypesLoadedMsg struct {
	types []string
}

func NewContent(styles *Styles, confirmDestructive bool, svc *Services) ContentModel {
	return ContentModel{
		styles:       styles,
		services:     svc,
		profileSaves: &profileSaveState{},
	}
}

func (m ContentModel) SetGame(g *game.Game) ContentModel {
	m.game, m.dllOperating = g, false
	m.scrollOffset, m.dllInstallState = 0, DLLInstallNone
	m.hasUpdates, m.usingDefaultProfile, m.lastDLLResult = false, false, ""
	m.dllUpdateTargets = nil

	if g != nil {
		rawProfile, _ := m.services.LoadProfile(g.AppID)
		defaults, _ := m.services.LoadDefaultProfile()
		if desired := m.profileSaves.latest(0); desired != nil {
			defaults = desired.Clone()
		}
		m.usingDefaultProfile = rawProfile == nil
		m.persistedProfile = rawProfile.Clone()
		if desired := m.profileSaves.latest(g.AppID); desired != nil {
			rawProfile, m.usingDefaultProfile = desired.Clone(), false
		}
		m.detail = NewDetail(m.styles, rawProfile, defaults)
		m.detail.SetSize(max(m.width, 1), m.height)
		m.hasBackup = m.services.BackupExists(g.AppID)
	}

	return m
}

func (m *ContentModel) SetSize(width, height int) {
	m.width, m.height = width, height
}

// profileSectionHeight returns the space below the Library aspect selector.
func (m ContentModel) profileSectionHeight() int {
	return max(m.height-3, 5)
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

	return m, nil
}

// saveResolvedProfile emits a save command for the current game's raw profile.
func (m *ContentModel) saveResolvedProfile() tea.Cmd {
	if m.game == nil {
		return nil
	}
	raw := m.detail.RawProfile()
	if raw == nil {
		return nil
	}
	// Ensure the profile carries the game name so saving a freshly-inherited
	// profile doesn't lose identity metadata.
	if raw.Name == "" && m.game != nil {
		raw.Name = m.game.Name
	}
	before := m.persistedProfile
	if desired := m.profileSaves.latest(m.game.AppID); desired != nil {
		before = desired
	}
	return m.profileSaves.start(profileSaveRequest{appID: m.game.AppID, before: before.Clone(), desired: raw.Clone()})
}

func (m ContentModel) updateDLLs() tea.Cmd {
	if m.game == nil {
		return func() tea.Msg { return dllUpdateMsg{err: fmt.Errorf("no game selected")} }
	}
	requests := dllUpdateRequests(m.dllUpdateTargets, false)
	return func() tea.Msg {
		return dllUpdateMsg{batch: m.services.BatchUpdateDLLs(requests)}
	}
}

func (m ContentModel) restoreDLLs() tea.Cmd {
	if m.game == nil {
		return func() tea.Msg { return dllRestoreMsg{err: fmt.Errorf("no game selected")} }
	}
	appID := m.game.AppID
	return func() tea.Msg {
		result, err := m.services.restoreDLLs(appID)
		return dllRestoreMsg{result: result, err: err}
	}
}

func (m ContentModel) HasModalOpen() bool {
	return m.dllInstallState != DLLInstallNone || m.pendingAction != PendingNone
}

// ViewProfileAspect renders Library › Profile for the selected game.
func (m ContentModel) ViewProfileAspect() string {
	if m.game == nil {
		return m.styles.Dim.Render("Select a game from the scope list")
	}
	return m.renderProfile()
}

// ViewDLLAspect renders Library › DLLs for the selected game.
func (m ContentModel) ViewDLLAspect() string {
	if m.dllInstallState != DLLInstallNone {
		return m.renderDLLInstallDialog()
	}
	if m.game == nil {
		return m.styles.Dim.Render("Select a game from the scope list")
	}
	return m.renderDLLs()
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
				selected := m.dllVersions[m.dllVersionCursor]
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
				return m, nil
			}
		}

	case dllTypesLoadedMsg:
		m.dllTypes = msg.types
		return m, nil

	case dllInstallMsg:
		m.dllInstallState = DLLInstallNone
		m.dllOperating = false
		if msg.result.Game != nil {
			m.applyDLLResult(msg.result)
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
	var detected []game.DetectedDLL
	if m.game != nil {
		detected = append(detected, m.game.DLLs...)
	}
	return func() tea.Msg {
		if len(detected) == 0 {
			return dllUpdatesCheckedMsg{hasUpdates: false}
		}

		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllUpdatesCheckedMsg{err: fmt.Errorf("failed to fetch manifest: %w", err)}
		}

		targets := latestDLLMutationTargets([]*game.Game{m.game}, manifest)
		return dllUpdatesCheckedMsg{hasUpdates: len(targets) > 0, targets: targets}
	}
}

type dllVersionsLoadedMsg struct {
	versions []dll.DLL
}

func (m ContentModel) installSelectedDLL() tea.Cmd {
	dllType := m.selectedDLLType
	dllInfo := m.dllVersions[m.dllVersionCursor]
	appID := m.game.AppID

	return func() tea.Msg {
		result, err := m.services.installDLL(appID, dllType, dllInfo.Version)
		return dllInstallMsg{result: result, err: err}
	}
}
