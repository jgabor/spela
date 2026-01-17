package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

type Focus int

const (
	FocusSidebar Focus = iota
	FocusContent
)

const (
	minSidebarWidth = 25
	maxSidebarWidth = 50
	sidebarRatio    = 0.30
	statusBarHeight = 1
	headerHeight    = 7 // 6 lines for logo + 1 for bottom border
)

type LayoutModel struct {
	header        HeaderModel
	sidebar       SidebarModel
	content       ContentModel
	statusBar     StatusBarModel
	help          HelpModel
	showHelp      bool
	showBatchMenu bool
	batchGames    []*game.Game
	batchCursor   int
	batchMessage  string
	focus         Focus
	width         int
	height        int
	sidebarWidth  int
}

func NewLayout(db *game.Database) LayoutModel {
	cfg, _ := config.Load()
	if cfg != nil {
		SetShowHints(cfg.ShowHints)
	}

	games := db.List()
	return LayoutModel{
		header:    NewHeader(),
		sidebar:   NewSidebar(games),
		content:   NewContent(),
		statusBar: NewStatusBar(),
		help:      NewHelp(),
		focus:     FocusSidebar,
	}
}

func (m LayoutModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.header.Init()}
	if g := m.sidebar.Selected(); g != nil {
		cmds = append(cmds, func() tea.Msg {
			return gameSelectedMsg{game: g}
		})
	}
	return tea.Batch(cmds...)
}

func (m LayoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.calculateDimensions()

	case tea.KeyMsg:
		if m.showBatchMenu {
			switch msg.String() {
			case "esc", "q":
				m.showBatchMenu = false
				m.batchGames = nil
				return m, nil
			case "up", "k":
				if m.batchCursor > 0 {
					m.batchCursor--
				}
			case "down", "j":
				if m.batchCursor < 2 {
					m.batchCursor++
				}
			case "enter":
				return m, m.executeBatchAction()
			}
			return m, nil
		}

		if m.showHelp {
			switch msg.String() {
			case "?", "esc", "q":
				m.showHelp = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = true
			return m, nil
		case "q":
			if m.focus == FocusSidebar && !m.sidebar.search.Focused() {
				return m, tea.Quit
			} else if m.focus == FocusContent {
				m.focus = FocusSidebar
				return m, nil
			}
		case "tab":
			if m.focus == FocusSidebar {
				m.focus = FocusContent
			} else {
				m.focus = FocusSidebar
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case gameSelectedMsg:
		m.content = m.content.SetGame(msg.game)
		return m, nil

	case batchActionRequestMsg:
		m.showBatchMenu = true
		m.batchGames = msg.selected
		m.batchCursor = 0
		m.batchMessage = ""
		return m, nil

	case batchCompleteMsg:
		m.batchMessage = msg.message
		return m, nil
	}

	if m.focus == FocusSidebar {
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		m.content, cmd = m.content.Update(msg)
		cmds = append(cmds, cmd)
	}

	var headerCmd tea.Cmd
	m.header, headerCmd = m.header.Update(msg)
	cmds = append(cmds, headerCmd)

	return m, tea.Batch(cmds...)
}

func (m *LayoutModel) calculateDimensions() {
	m.sidebarWidth = int(float64(m.width) * sidebarRatio)
	m.sidebarWidth = max(m.sidebarWidth, minSidebarWidth)
	m.sidebarWidth = min(m.sidebarWidth, maxSidebarWidth)

	// Total height minus header, status bar, and borders (2 for sidebar/content top+bottom)
	contentHeight := max(m.height-statusBarHeight-headerHeight-2, 5)

	m.header.SetWidth(m.width)
	m.sidebar.SetSize(m.sidebarWidth-3, contentHeight) // -3 for left+right borders and padding
	m.content.SetSize(m.width-m.sidebarWidth-4, contentHeight)
	m.statusBar.SetWidth(m.width)
}

func (m LayoutModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.showBatchMenu {
		return m.renderBatchOverlay()
	}

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	header := m.header.View()

	// Height accounts for header, status bar, and own top+bottom borders
	panelHeight := m.height - statusBarHeight - headerHeight - 2

	sidebarStyle := lipgloss.NewStyle().
		Width(m.sidebarWidth - 1).
		Height(panelHeight)

	contentStyle := lipgloss.NewStyle().
		Width(m.width - m.sidebarWidth - 2).
		Height(panelHeight)

	sidebarStyle = sidebarStyle.BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor(m.focus == FocusSidebar))
	contentStyle = contentStyle.BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor(m.focus == FocusContent))

	sidebar := sidebarStyle.Render(m.sidebar.View())
	content := contentStyle.Render(m.content.View())

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	contextHelp := ContextHelp(m.focus, m.sidebar.search.Focused())
	statusBar := m.statusBar.ViewWithHelp(contextHelp)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, statusBar)
}

func (m LayoutModel) renderHelpOverlay() string {
	overlayStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(2, 4)

	return overlayStyle.Render(m.help.View())
}

type gameSelectedMsg struct {
	game *game.Game
}

func Run(db *game.Database) error {
	p := tea.NewProgram(NewLayout(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type batchCompleteMsg struct {
	message string
}

var batchActions = []string{
	"Update all DLLs",
	"Apply Performance preset",
	"Apply Balanced preset",
	"Apply Quality preset",
}

func (m LayoutModel) executeBatchAction() tea.Cmd {
	games := m.batchGames
	action := m.batchCursor

	return func() tea.Msg {
		if action == 0 {
			return executeBatchDLLUpdate(games)
		}

		var preset profile.Preset
		switch action {
		case 1:
			preset = profile.PresetPerformance
		case 2:
			preset = profile.PresetBalanced
		case 3:
			preset = profile.PresetQuality
		}

		succeeded := 0
		for _, g := range games {
			p := profile.FromPreset(preset)
			p.Name = g.Name
			if err := profile.Save(g.AppID, p); err == nil {
				succeeded++
			}
		}

		return batchCompleteMsg{
			message: fmt.Sprintf("Applied %s preset to %d/%d games", preset, succeeded, len(games)),
		}
	}
}

func executeBatchDLLUpdate(games []*game.Game) batchCompleteMsg {
	manifest, err := dll.GetManifest(false, "")
	if err != nil {
		return batchCompleteMsg{message: fmt.Sprintf("Failed to load manifest: %v", err)}
	}

	succeeded := 0
	failed := 0

	for _, g := range games {
		if len(g.DLLs) == 0 {
			continue
		}

		var gameDLLs []dll.GameDLL
		for _, d := range g.DLLs {
			gameDLLs = append(gameDLLs, dll.GameDLL{
				Name:    d.Name,
				Path:    d.Path,
				Version: d.Version,
			})
		}

		gameUpdated := false
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
				failed++
				continue
			}

			if err := dll.SwapDLL(g.AppID, g.Name, gameDLLs, d.Name, cachePath); err != nil {
				failed++
				continue
			}
			gameUpdated = true
		}

		if gameUpdated {
			succeeded++
		}
	}

	if failed > 0 {
		return batchCompleteMsg{
			message: fmt.Sprintf("Updated %d games, %d failed", succeeded, failed),
		}
	}
	return batchCompleteMsg{
		message: fmt.Sprintf("Updated DLLs for %d/%d games", succeeded, len(games)),
	}
}

func (m LayoutModel) renderBatchOverlay() string {
	overlayStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(2, 4)

	return overlayStyle.Render(m.renderBatchMenu())
}

func (m LayoutModel) renderBatchMenu() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render(fmt.Sprintf("Batch action (%d games)", len(m.batchGames))))
	b.WriteString("\n\n")

	for i, action := range batchActions {
		cursor := "  "
		style := normalStyle
		if i == m.batchCursor {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, action)))
		b.WriteString("\n")
	}

	if m.batchMessage != "" {
		b.WriteString("\n")
		b.WriteString(successStyle.Render(m.batchMessage))
		b.WriteString("\n")
	}

	if hint := RenderHint("\n\n↑/↓ select • enter execute • esc cancel"); hint != "" {
		b.WriteString(hint)
	}

	return b.String()
}
