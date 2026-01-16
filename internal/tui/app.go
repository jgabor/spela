package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type view int

const (
	viewBrowser view = iota
	viewDetail
	viewProfile
	viewMonitor
	viewDLLStatus
)

type Model struct {
	browser       BrowserModel
	profileEditor ProfileEditorModel
	monitor       MonitorModel
	dllStatus     DLLStatusModel
	currentView   view
	selected      *game.Game
	profile       *profile.Profile
	width         int
	height        int
	err           error
}

func NewModel(db *game.Database) Model {
	games := db.List()
	return Model{
		browser:     NewBrowser(games),
		currentView: viewBrowser,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.browser.width = msg.Width
		m.browser.height = msg.Height

	case profileSaveMsg:
		if msg.success {
			m.profileEditor.SetMessage("Profile saved!")
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.currentView == viewBrowser {
				return m, tea.Quit
			}
			m.currentView = viewBrowser
			return m, nil

		case "enter":
			if m.currentView == viewBrowser {
				m.selected = m.browser.Selected()
				if m.selected != nil {
					m.currentView = viewDetail
					p, _ := profile.Load(m.selected.AppID)
					m.profile = p
				}
			}
			return m, nil

		case "p":
			if m.currentView == viewDetail && m.selected != nil {
				m.profileEditor = NewProfileEditor(m.selected, m.profile)
				m.currentView = viewProfile
			}
			return m, nil

		case "d":
			if m.currentView == viewDetail && m.selected != nil && len(m.selected.DLLs) > 0 {
				m.dllStatus = NewDLLStatus(m.selected)
				m.currentView = viewDLLStatus
			}
			return m, nil

		case "m":
			if m.currentView == viewBrowser {
				m.monitor = NewMonitor()
				m.currentView = viewMonitor
				return m, m.monitor.Init()
			}
			return m, nil

		case "esc":
			if m.currentView == viewProfile || m.currentView == viewDLLStatus {
				m.currentView = viewDetail
				return m, nil
			}
			if m.currentView != viewBrowser {
				m.currentView = viewBrowser
				return m, nil
			}
		}
	}

	switch m.currentView {
	case viewBrowser:
		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd
	case viewProfile:
		var cmd tea.Cmd
		m.profileEditor, cmd = m.profileEditor.Update(msg)
		return m, cmd
	case viewMonitor:
		var cmd tea.Cmd
		m.monitor, cmd = m.monitor.Update(msg)
		return m, cmd
	case viewDLLStatus:
		var cmd tea.Cmd
		m.dllStatus, cmd = m.dllStatus.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	switch m.currentView {
	case viewDetail:
		return m.viewDetail()
	case viewProfile:
		return m.profileEditor.View()
	case viewMonitor:
		return m.monitor.View()
	case viewDLLStatus:
		return m.dllStatus.View()
	default:
		return m.browser.View()
	}
}

func (m Model) viewDetail() string {
	if m.selected == nil {
		return "No game selected"
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(m.selected.Name))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("App ID:      %d\n", m.selected.AppID))
	b.WriteString(fmt.Sprintf("Install Dir: %s\n", m.selected.InstallDir))

	if m.selected.PrefixPath != "" {
		b.WriteString(fmt.Sprintf("Prefix:      %s\n", m.selected.PrefixPath))
	}

	if len(m.selected.DLLs) > 0 {
		b.WriteString("\n")
		b.WriteString(dlssStyle.Render("Detected DLLs:"))
		b.WriteString("\n")
		for _, d := range m.selected.DLLs {
			version := d.Version
			if version == "" {
				version = "unknown"
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n", d.Name, version))
		}
	}

	if m.profile != nil {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("Profile: "))
		b.WriteString(string(m.profile.Preset))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("No profile configured"))
		b.WriteString("\n")
	}

	help := "\np edit profile"
	if len(m.selected.DLLs) > 0 {
		help += " • d view DLLs"
	}
	help += " • esc back • q quit"
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func Run(db *game.Database) error {
	p := tea.NewProgram(NewModel(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
