package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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

type ContentModel struct {
	game          *game.Game
	profile       *profile.Profile
	profileEditor ProfileEditorModel
	width         int
	height        int
	message       string
	dllOperating  bool
	hasBackup     bool
	scrollOffset  int

	dllInstallState  DLLInstallState
	dllTypes         []string
	dllTypeCursor    int
	dllVersions      []dll.DLL
	dllVersionCursor int
	selectedDLLType  string
}

type dllUpdateMsg struct {
	success bool
	err     error
}

type dllRestoreMsg struct {
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
	return ContentModel{}
}

func (m ContentModel) SetGame(g *game.Game) ContentModel {
	m.game = g
	m.message = ""
	m.dllOperating = false
	m.scrollOffset = 0

	if g != nil {
		p, _ := profile.Load(g.AppID)
		m.profile = p
		m.profileEditor = NewProfileEditor(g, p)
		m.hasBackup = dll.BackupExists(g.AppID)
	}

	return m
}

func (m *ContentModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m ContentModel) Update(msg tea.Msg) (ContentModel, tea.Cmd) {
	var cmds []tea.Cmd

	if m.dllInstallState != DLLInstallNone {
		return m.updateDLLInstall(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "i":
			if m.game != nil && !m.dllOperating {
				m.dllInstallState = DLLInstallSelectType
				m.dllTypeCursor = 0
				m.message = "Loading DLL types..."
				return m, m.loadDLLTypes()
			}
		case "u":
			if m.game != nil && len(m.game.DLLs) > 0 && !m.dllOperating {
				m.dllOperating = true
				m.message = "Updating DLLs..."
				return m, m.updateDLLs()
			}
		case "R":
			if m.game != nil && m.hasBackup && !m.dllOperating {
				m.dllOperating = true
				m.message = "Restoring original DLLs..."
				return m, m.restoreDLLs()
			}
		}

	case profileSaveMsg:
		if msg.success {
			m.message = "Profile saved!"
			if m.game != nil {
				p, _ := profile.Load(m.game.AppID)
				m.profile = p
			}
		} else if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
		}
		return m, nil

	case dllUpdateMsg:
		m.dllOperating = false
		if msg.success {
			m.message = "DLLs updated successfully!"
		} else if msg.err != nil {
			m.message = fmt.Sprintf("Update failed: %v", msg.err)
		}
		m.hasBackup = m.game != nil && dll.BackupExists(m.game.AppID)
		return m, nil

	case dllRestoreMsg:
		m.dllOperating = false
		if msg.success {
			m.message = "Original DLLs restored!"
			m.hasBackup = false
		} else if msg.err != nil {
			m.message = fmt.Sprintf("Restore failed: %v", msg.err)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.profileEditor, cmd = m.profileEditor.Update(msg)
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

func (m ContentModel) View() string {
	if m.game == nil {
		return dimStyle.Render("Select a game from the sidebar")
	}

	if m.dllInstallState != DLLInstallNone {
		return m.renderDLLInstallDialog()
	}

	var b strings.Builder

	b.WriteString(m.renderGameInfo())
	b.WriteString("\n")
	b.WriteString(m.renderDLLs())
	b.WriteString("\n")
	b.WriteString(m.renderProfile())

	if m.message != "" {
		b.WriteString("\n")
		if strings.HasPrefix(m.message, "Error") || strings.HasPrefix(m.message, "Update failed") || strings.HasPrefix(m.message, "Restore failed") || strings.HasPrefix(m.message, "Install failed") {
			b.WriteString(errorStyle.Render(m.message))
		} else if m.dllOperating {
			b.WriteString(dimStyle.Render(m.message))
		} else {
			b.WriteString(successStyle.Render(m.message))
		}
	}

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
			b.WriteString(dimStyle.Render("Loading..."))
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

	b.WriteString(fmt.Sprintf("App ID:      %d\n", m.game.AppID))
	b.WriteString(fmt.Sprintf("Install Dir: %s\n", m.game.InstallDir))

	if m.game.PrefixPath != "" {
		b.WriteString(fmt.Sprintf("Prefix:      %s\n", m.game.PrefixPath))
	}

	return b.String()
}

func (m ContentModel) renderDLLs() string {
	var b strings.Builder

	t := GetTheme()
	sectionStyle := titleStyle.Foreground(t.Secondary)

	b.WriteString(sectionStyle.Render("DLLs"))
	b.WriteString("\n")

	if len(m.game.DLLs) == 0 {
		b.WriteString(dimStyle.Render("  No DLSS DLLs detected"))
		b.WriteString("\n")
		return b.String()
	}

	for _, d := range m.game.DLLs {
		version := d.Version
		if version == "" {
			version = "unknown"
		}
		b.WriteString(fmt.Sprintf("  %s: %s\n", d.Name, version))
	}

	if ShowHints() {
		var actions []string
		if len(m.game.DLLs) > 0 && !m.dllOperating {
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
		}
	}

	return b.String()
}

func (m ContentModel) renderProfile() string {
	var b strings.Builder

	t := GetTheme()
	sectionStyle := titleStyle.Foreground(t.Secondary)

	b.WriteString(sectionStyle.Render("Profile"))
	b.WriteString("\n")

	for i, field := range m.profileEditor.fields {
		cursor := "  "
		style := normalStyle
		if i == m.profileEditor.cursor {
			cursor = "> "
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%-16s: ", cursor, field.label)
		b.WriteString(style.Render(line))
		b.WriteString(dlssStyle.Render(field.value))
		b.WriteString("\n")

		if i == m.profileEditor.cursor && field.description != "" {
			b.WriteString(dimStyle.Render("    " + field.description))
			b.WriteString("\n")
		}
	}

	if m.profileEditor.Modified() {
		b.WriteString(RenderHint("  (modified) s:save"))
		if ShowHints() {
			b.WriteString("\n")
		}
	}

	if hint := RenderHint("  ↑↓:navigate • ←→:change"); hint != "" {
		b.WriteString(hint)
		b.WriteString("\n")
	}

	return b.String()
}

func (m ContentModel) loadDLLTypes() tea.Cmd {
	gameDLLTypes := make(map[string]bool)
	for _, d := range m.game.DLLs {
		gameDLLTypes[strings.ToLower(string(d.Type))] = true
	}

	return func() tea.Msg {
		manifest, err := dll.GetManifest(false, "")
		if err != nil {
			return dllInstallMsg{err: err}
		}

		allTypes := manifest.ListDLLNames()
		var filteredTypes []string
		for _, t := range allTypes {
			if gameDLLTypes[t] {
				filteredTypes = append(filteredTypes, t)
			}
		}

		if len(filteredTypes) == 0 {
			return dllInstallMsg{err: fmt.Errorf("no supported DLL types detected in game")}
		}

		return dllTypesLoadedMsg{types: filteredTypes}
	}
}

func (m ContentModel) updateDLLInstall(msg tea.Msg) (ContentModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.dllInstallState = DLLInstallNone
			m.message = ""
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
				return m, m.loadDLLVersions()
			} else if m.dllInstallState == DLLInstallSelectVersion && len(m.dllVersions) > 0 {
				m.dllInstallState = DLLInstallDownloading
				m.message = "Installing DLL..."
				return m, m.installSelectedDLL()
			}
		}

	case dllTypesLoadedMsg:
		m.dllTypes = msg.types
		m.message = ""
		return m, nil

	case dllInstallMsg:
		m.dllInstallState = DLLInstallNone
		m.dllOperating = false
		if msg.success {
			m.message = "DLL installed successfully!"
			m.hasBackup = m.game != nil && dll.BackupExists(m.game.AppID)
		} else if msg.err != nil {
			m.message = fmt.Sprintf("Install failed: %v", msg.err)
		}
		return m, nil

	case dllVersionsLoadedMsg:
		m.dllVersions = msg.versions
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
		if err := dll.SwapDLL(g.AppID, g.Name, gameDLLs, targetName, cachePath); err != nil {
			return dllInstallMsg{err: err}
		}

		return dllInstallMsg{success: true}
	}
}
