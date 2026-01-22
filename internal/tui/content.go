package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/env"
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

type ContentModel struct {
	game                *game.Game
	defaultProfile      bool
	profile             *profile.Profile
	profileWidget       ProfileWidgetModel
	dlssPresetModal     DLSSPresetModalModel
	width               int
	height              int
	profileHeight       int
	dllOperating        bool
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
}

type dllTypesLoadedMsg struct {
	types []string
}

func NewContent() ContentModel {
	return ContentModel{
		dlssPresetModal: NewDLSSPresetModal(),
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
	m.launching = false

	if g != nil {
		p, inherited := loadEffectiveProfile(g.AppID)
		m.profile = p
		m.usingDefaultProfile = inherited
		m.profileWidget = NewProfileWidget(g, p)
		m.hasBackup = dll.BackupExists(g.AppID)
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

	p, _ := profile.LoadDefault()
	m.profile = p
	m.profileWidget = NewDefaultProfileWidget(p)
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
	extraLines := 0
	if m.usingDefaultProfile {
		extraLines = 1
	}
	return max(m.height-headerSectionHeight-dllSectionHeight-2-extraLines, 5)
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

	case tea.KeyMsg:
		switch msg.String() {
		case "l":
			if m.game != nil && !m.defaultProfile && !m.launching {
				m.launching = true
				return m, m.launchGame()
			}
		case "i":
			if m.game != nil && !m.dllOperating {
				m.dllInstallState = DLLInstallSelectType
				m.dllTypeCursor = 0
				return m, m.loadDLLTypes()
			}
		case "u":
			if m.game != nil && len(m.game.DLLs) > 0 && m.hasUpdates && !m.dllOperating {
				m.dllOperating = true
				return m, m.updateDLLs()
			}
		case "R":
			if m.game != nil && m.hasBackup && !m.dllOperating {
				m.dllOperating = true
				return m, m.restoreDLLs()
			}
		}

	case profileSaveMsg:
		if msg.success {
			if m.defaultProfile {
				p, _ := profile.LoadDefault()
				m.profile = p
			} else if m.game != nil {
				p, inherited := loadEffectiveProfile(m.game.AppID)
				m.profile = p
				m.usingDefaultProfile = inherited
				m.profileHeight = m.profileSectionHeight()
			}
		}
		return m, nil

	case dllUpdateMsg:
		m.dllOperating = false
		m.hasBackup = m.game != nil && dll.BackupExists(m.game.AppID)
		if msg.success {
			m.hasUpdates = false
			return m, m.LoadDLLUpdates()
		}
		return m, nil

	case dllRestoreMsg:
		m.dllOperating = false
		if msg.success {
			m.hasBackup = m.game != nil && dll.BackupExists(m.game.AppID)
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
	return func() tea.Msg {
		if m.game == nil || len(m.game.DLLs) == 0 {
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

		var gameDLLs []dll.GameDLL
		for _, d := range m.game.DLLs {
			gameDLLs = append(gameDLLs, dll.GameDLL{
				Name:    d.Name,
				Path:    d.Path,
				Version: d.Version,
			})
		}

		updatedCount := 0
		for _, d := range m.game.DLLs {
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

			if err := dll.SwapDLL(m.game.AppID, m.game.Name, gameDLLs, d.Name, cachePath); err != nil {
				return dllUpdateMsg{err: fmt.Errorf("swap %s failed: %w", dllType, err)}
			}
			updatedCount++
		}

		if updatedCount == 0 {
			return dllUpdateMsg{err: fmt.Errorf("no updates available")}
		}

		detected, err := dll.ScanDirectory(m.game.InstallDir)
		if err != nil {
			return dllUpdateMsg{err: err}
		}

		m.game.DLLs = detected
		m.game.ScannedAt = time.Now()

		return dllUpdateMsg{success: true}
	}
}

func (m ContentModel) restoreDLLs() tea.Cmd {
	return func() tea.Msg {
		if m.game == nil {
			return dllRestoreMsg{err: fmt.Errorf("no game selected")}
		}

		if err := dll.RestoreBackup(m.game.AppID); err != nil {
			return dllRestoreMsg{err: err}
		}

		return dllRestoreMsg{success: true}
	}
}

func (m ContentModel) HasModalOpen() bool {
	return m.dlssPresetModal.Visible() || m.dllInstallState != DLLInstallNone || m.profileWidget.Editing()
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
		return dimStyle.Render("Select a game from the sidebar")
	}

	var b strings.Builder

	b.WriteString(m.renderGameInfo())
	b.WriteString("\n")
	b.WriteString(m.renderDLLs())
	b.WriteString("\n")
	b.WriteString(m.renderProfile())

	return b.String()
}

func (m ContentModel) renderDefaultProfile() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Default profile"))
	b.WriteString("\n\n")
	b.WriteString(m.renderProfile())

	return b.String()
}

func (m ContentModel) renderDLLInstallDialog() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Install DLL"))
	b.WriteString("\n\n")

	switch m.dllInstallState {
	case DLLInstallSelectType:
		b.WriteString(dimStyle.Render("Select DLL type:"))
		b.WriteString("\n\n")

		if len(m.dllTypes) == 0 {
			b.WriteString(dimStyle.Render("Loading..."))
		} else {
			for i, t := range m.dllTypes {
				cursor := "  "
				style := normalStyle
				if i == m.dllTypeCursor {
					cursor = "> "
					style = selectedStyle
				}
				b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, strings.ToUpper(t))))
				b.WriteString("\n")
			}
		}

	case DLLInstallSelectVersion:
		b.WriteString(dimStyle.Render(fmt.Sprintf("Select %s version:", strings.ToUpper(m.selectedDLLType))))
		b.WriteString("\n\n")

		if len(m.dllVersions) == 0 {
			if m.dllVersionsLoaded {
				b.WriteString(errorStyle.Render("No versions available"))
			} else {
				b.WriteString(dimStyle.Render("Loading..."))
			}
		} else {
			for i, v := range m.dllVersions {
				cursor := "  "
				style := normalStyle
				if i == m.dllVersionCursor {
					cursor = "> "
					style = selectedStyle
				}
				label := v.Version
				if i == 0 {
					label += " (latest)"
				}
				b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, label)))
				b.WriteString("\n")
			}
		}

	case DLLInstallDownloading:
		b.WriteString(dimStyle.Render("Installing DLL..."))
	}

	if hint := RenderHint("\n\n↑/↓ select • enter confirm • esc cancel"); hint != "" {
		b.WriteString(hint)
	}

	return b.String()
}

func (m ContentModel) renderGameInfo() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(m.game.Name))
	b.WriteString("\n\n")

	lines := 2 // title + blank line
	fmt.Fprintf(&b, "App ID:      %d\n", m.game.AppID)
	lines++
	fmt.Fprintf(&b, "Install Dir: %s\n", m.game.InstallDir)
	lines++

	if m.game.PrefixPath != "" {
		fmt.Fprintf(&b, "Prefix:      %s\n", m.game.PrefixPath)
		lines++
	}

	// Pad to fixed height
	for lines < headerSectionHeight {
		b.WriteString("\n")
		lines++
	}

	return b.String()
}

func (m ContentModel) renderDLLs() string {
	var b strings.Builder

	t := GetTheme()
	sectionStyle := titleStyle.Foreground(t.Secondary)

	b.WriteString(sectionStyle.Render("DLL versions"))
	b.WriteString("\n")
	lines := 1 // section title

	if len(m.game.DLLs) == 0 {
		b.WriteString(dimStyle.Render("  No DLLs detected"))
		b.WriteString("\n")
		lines++
	} else {
		// Build DLL type -> version mapping
		dllVersions := make(map[string]string)
		for _, d := range m.game.DLLs {
			version := d.Version
			if version == "" {
				version = "?"
			}
			dllType := strings.ToUpper(string(d.Type))
			dllVersions[dllType] = version
		}

		// Column layout: type headers then versions
		columnWidth := 12
		dllTypes := []string{"DLSS", "DLSS-G", "XESS", "FSR"}

		// Header row
		b.WriteString("  ")
		for _, dllType := range dllTypes {
			b.WriteString(dimStyle.Render(fmt.Sprintf("%-*s", columnWidth, dllType)))
		}
		b.WriteString("\n")
		lines++

		// Version row
		b.WriteString("  ")
		for _, dllType := range dllTypes {
			version := dllVersions[dllType]
			if version == "" {
				version = "-"
			}
			b.WriteString(dlssStyle.Render(fmt.Sprintf("%-*s", columnWidth, version)))
		}
		b.WriteString("\n")
		lines++

		if ShowHints() {
			var actions []string
			if !m.dllOperating && m.hasUpdates {
				actions = append(actions, "u:update")
			}
			if m.hasBackup && !m.dllOperating {
				actions = append(actions, "R:restore")
			}
			if m.hasBackup {
				actions = append(actions, "(backup exists)")
			}

			if len(actions) > 0 {
				b.WriteString(RenderHint("  " + strings.Join(actions, " • ")))
				b.WriteString("\n")
				lines++
			}
		}
	}

	// Pad to fixed height
	for lines < dllSectionHeight {
		b.WriteString("\n")
		lines++
	}

	return b.String()
}

func (m ContentModel) renderProfile() string {
	var b strings.Builder
	if m.usingDefaultProfile {
		b.WriteString(dimStyle.Render("Using default profile values"))
		b.WriteString("\n")
	}
	m.profileWidget.SetSize(m.width, m.profileHeight)
	b.WriteString(m.profileWidget.View())
	return b.String()
}

func loadEffectiveProfile(appID uint64) (*profile.Profile, bool) {
	p, _ := profile.Load(appID)
	if p != nil {
		return p, false
	}
	defaultProfile, _ := profile.LoadDefault()
	if defaultProfile == nil {
		return nil, false
	}
	return defaultProfile, true
}

func (m ContentModel) loadDLLTypes() tea.Cmd {
	return func() tea.Msg {
		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllInstallMsg{err: err}
		}

		validTypes := make(map[string]bool)
		if len(m.game.DLLs) > 0 {
			validTypes = make(map[string]bool, len(m.game.DLLs))
			for _, d := range m.game.DLLs {
				validTypes[strings.ToLower(string(d.Type))] = true
			}
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

		sort.Strings(filteredTypes)
		return dllTypesLoadedMsg{types: filteredTypes}
	}
}

func (m ContentModel) updateDLLInstall(msg tea.Msg) (ContentModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.dllInstallState = DLLInstallNone
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
			m.hasBackup = m.game != nil && dll.BackupExists(m.game.AppID)
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
		environment := env.New()
		if p != nil {
			p.Apply(environment)
		}

		cmd := exec.Command("steam", fmt.Sprintf("steam://rungameid/%d", g.AppID))
		environment.ApplyToCmd(cmd)
		if err := cmd.Start(); err != nil {
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

		var gameDLLs []dll.GameDLL
		for _, d := range g.DLLs {
			gameDLLs = append(gameDLLs, dll.GameDLL{
				Name:    d.Name,
				Path:    d.Path,
				Version: d.Version,
			})
		}

		targetName := dllInfo.Filename
		if err := dll.InstallDLL(g.AppID, g.Name, g.InstallDir, gameDLLs, targetName, cachePath); err != nil {
			return dllInstallMsg{err: err}
		}

		detected, err := dll.ScanDirectory(g.InstallDir)
		if err != nil {
			return dllInstallMsg{err: err}
		}

		g.DLLs = detected
		g.ScannedAt = time.Now()

		return dllInstallMsg{success: true}
	}
}
