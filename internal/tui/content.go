package tui

import (
	"fmt"
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

	dllRequestID      uint64
	dllInstallControl int
	dllResultOpen     bool
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
	appID      uint64
	hasUpdates bool
	targets    []dllMutationTarget
	err        error
}

type dllInstallMsg struct {
	requestID uint64
	result    dll.Result
	err       error
}

type dllTypesLoadedMsg struct {
	requestID uint64
	types     []string
}

func NewContent(styles *Styles, confirmDestructive bool, svc *Services) ContentModel {
	return ContentModel{
		styles:       styles,
		services:     svc,
		profileSaves: &profileSaveState{},
	}
}

func (m ContentModel) SetGame(g *game.Game) ContentModel {
	if m.game != nil && g != nil && m.game.AppID == g.AppID && (m.detail.Dirty() || m.detail.Editing()) {
		m.game = g
		m.hasBackup = m.services.BackupExists(g.AppID)
		return m
	}
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
	return m.confirmation != nil || m.dllOperating || m.dllResultOpen || m.dllInstallState != DLLInstallNone || m.pendingAction != PendingNone
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
	requestID := m.dllRequestID
	return func() tea.Msg {
		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllInstallMsg{requestID: requestID, err: err}
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
			return dllInstallMsg{requestID: requestID, err: fmt.Errorf("no supported DLL types detected in game")}
		}

		return dllTypesLoadedMsg{requestID: requestID, types: filteredTypes}
	}
}

func (m ContentModel) loadDLLVersions() tea.Cmd {
	dllType := m.selectedDLLType
	requestID := m.dllRequestID
	return func() tea.Msg {
		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllInstallMsg{requestID: requestID, err: err}
		}
		versions := manifest.DLLs[dllType]
		return dllVersionsLoadedMsg{requestID: requestID, versions: versions}
	}
}

func (m ContentModel) LoadDLLUpdates() tea.Cmd {
	var detected []game.DetectedDLL
	var appID uint64
	if m.game != nil {
		appID = m.game.AppID
		detected = append(detected, m.game.DLLs...)
	}
	return func() tea.Msg {
		if len(detected) == 0 {
			return dllUpdatesCheckedMsg{appID: appID, hasUpdates: false}
		}

		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllUpdatesCheckedMsg{appID: appID, err: fmt.Errorf("failed to fetch manifest: %w", err)}
		}

		targets := latestDLLMutationTargets([]*game.Game{m.game}, manifest)
		return dllUpdatesCheckedMsg{appID: appID, hasUpdates: len(targets) > 0, targets: targets}
	}
}

type dllVersionsLoadedMsg struct {
	requestID uint64
	versions  []dll.DLL
}

func (m ContentModel) installSelectedDLL() tea.Cmd {
	dllType := m.selectedDLLType
	dllInfo := m.dllVersions[m.dllVersionCursor]
	appID := m.game.AppID

	requestID := m.dllRequestID
	return func() tea.Msg {
		result, err := m.services.installDLL(appID, dllType, dllInfo.Version)
		return dllInstallMsg{requestID: requestID, result: result, err: err}
	}
}
