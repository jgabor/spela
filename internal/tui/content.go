package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
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
}

type dllUpdateMsg struct {
	success bool
	err     error
}

type dllRestoreMsg struct {
	success bool
	err     error
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
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

		for _, d := range m.game.DLLs {
			latest := manifest.GetLatestDLL(d.Name)
			if latest == nil {
				continue
			}

			cachePath, err := dll.DownloadDLL(latest, d.Name)
			if err != nil {
				return dllUpdateMsg{err: fmt.Errorf("download failed: %w", err)}
			}

			if err := dll.SwapDLL(m.game.AppID, m.game.Name, gameDLLs, d.Name, cachePath); err != nil {
				return dllUpdateMsg{err: fmt.Errorf("swap failed: %w", err)}
			}
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

	var b strings.Builder

	b.WriteString(m.renderGameInfo())
	b.WriteString("\n")
	b.WriteString(m.renderDLLs())
	b.WriteString("\n")
	b.WriteString(m.renderProfile())

	if m.message != "" {
		b.WriteString("\n")
		if strings.HasPrefix(m.message, "Error") || strings.HasPrefix(m.message, "Update failed") || strings.HasPrefix(m.message, "Restore failed") {
			b.WriteString(errorStyle.Render(m.message))
		} else if m.dllOperating {
			b.WriteString(dimStyle.Render(m.message))
		} else {
			b.WriteString(successStyle.Render(m.message))
		}
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

	var actions []string
	if len(m.game.DLLs) > 0 && !m.dllOperating {
		actions = append(actions, "u:update")
	}
	if m.hasBackup && !m.dllOperating {
		actions = append(actions, "R:restore")
	}
	if m.hasBackup {
		actions = append(actions, dimStyle.Render("(backup exists)"))
	}

	if len(actions) > 0 {
		b.WriteString(dimStyle.Render("  " + strings.Join(actions, " • ")))
		b.WriteString("\n")
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
		b.WriteString(dimStyle.Render("  (modified) "))
		b.WriteString(dimStyle.Render("s:save"))
		b.WriteString("\n")
	}

	b.WriteString(dimStyle.Render("  ↑↓:navigate • ←→:change"))
	b.WriteString("\n")

	return b.String()
}
