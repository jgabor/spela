package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

type Focus int

const (
	FocusSidebar Focus = iota
	FocusContent
)

const (
	minSidebarWidth  = 25
	maxSidebarWidth  = 50
	sidebarRatio     = 0.30
	statusBarHeight  = 1
	messageBarHeight = 1
	headerHeight     = 7 // 6 lines for logo + 1 for bottom border
)

type LayoutModel struct {
	header        HeaderModel
	sidebar       SidebarModel
	content       ContentModel
	statusBar     StatusBarModel
	messageBar    MessageBarModel
	help          HelpModel
	optionsModal  OptionsModalModel
	config        *config.Config
	db            *game.Database
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
	if cfg == nil {
		cfg = config.Default()
	}
	SetShowHints(cfg.ShowHints)
	switch cfg.Theme {
	case "light":
		SetTheme(LightTheme)
	case "dark":
		SetTheme(DarkTheme)
	}

	games := db.List()
	return LayoutModel{
		header:       NewHeader(),
		sidebar:      NewSidebar(games),
		content:      NewContent(),
		statusBar:    NewStatusBar(),
		messageBar:   NewMessageBar(),
		help:         NewHelp(),
		optionsModal: NewOptionsModal(),
		config:       cfg,
		db:           db,
		focus:        FocusSidebar,
	}
}

func (m LayoutModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.header.Init()}
	if selected := m.sidebar.SelectedItem(); selected != nil {
		if selected.kind == sidebarItemDefaultProfile {
			cmds = append(cmds, func() tea.Msg {
				return defaultProfileSelectedMsg{}
			})
		} else if selected.game != nil {
			cmds = append(cmds, func() tea.Msg {
				return gameSelectedMsg{game: selected.game}
			})
		}
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
		if m.optionsModal.Visible() {
			var cmd tea.Cmd
			m.optionsModal, cmd = m.optionsModal.Update(msg)
			return m, cmd
		}

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
		case "o":
			if m.focus == FocusSidebar && !m.sidebar.search.Focused() {
				m.optionsModal.SetSize(m.width, m.height)
				m.optionsModal.Open(m.config)
				return m, nil
			}
		case "ctrl+f":
			m.focus = FocusSidebar
			var cmd tea.Cmd
			m.sidebar, cmd = m.sidebar.FocusSearch()
			return m, cmd
		case "q":
			if m.focus == FocusSidebar && !m.sidebar.search.Focused() {
				return m, tea.Quit
			} else if m.focus == FocusContent && !m.content.HasModalOpen() {
				m.focus = FocusSidebar
				return m, nil
			}
		case "esc":
			if m.focus == FocusContent && !m.content.HasModalOpen() {
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
		case "r":
			if m.focus == FocusSidebar && !m.sidebar.search.Focused() {
				messageCmd := m.messageBar.SetMessage("Rescanning games...", MessageInfo)
				return m, tea.Batch(messageCmd, m.rescanGames())
			}
		}
	}

	switch msg := msg.(type) {
	case gameSelectedMsg:
		m.content = m.content.SetGame(msg.game)
		return m, m.content.LoadDLLUpdates()

	case gameConfirmedMsg:
		m.content = m.content.SetGame(msg.game)
		m.focus = FocusContent
		return m, m.content.LoadDLLUpdates()

	case defaultProfileSelectedMsg:
		m.content = m.content.SetDefaultProfile()
		return m, nil

	case defaultProfileConfirmedMsg:
		m.content = m.content.SetDefaultProfile()
		m.focus = FocusContent
		return m, nil

	case batchActionRequestMsg:
		m.showBatchMenu = true
		m.batchGames = msg.selected
		m.batchCursor = 0
		m.batchMessage = ""
		return m, nil

	case batchCompleteMsg:
		m.batchMessage = msg.message
		cmd := m.messageBar.SetMessage(msg.message, MessageSuccess)
		return m, cmd

	case messageClearMsg:
		m.messageBar, _ = m.messageBar.Update(msg)
		return m, nil

	case dllUpdateMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "DLLs updated successfully!"
			msgType = MessageSuccess
		} else if msg.err != nil {
			message = fmt.Sprintf("Update failed: %v", msg.err)
			msgType = MessageError
		}
		messageCmd := m.messageBar.SetMessage(message, msgType)
		var contentCmd tea.Cmd
		m.content, contentCmd = m.content.Update(msg)
		return m, tea.Batch(messageCmd, contentCmd)

	case dllRestoreMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "Original DLLs restored!"
			msgType = MessageSuccess
		} else if msg.err != nil {
			message = fmt.Sprintf("Restore failed: %v", msg.err)
			msgType = MessageError
		}
		cmd := m.messageBar.SetMessage(message, msgType)
		m.content, _ = m.content.Update(msg)
		return m, cmd

	case dllInstallMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "DLL installed successfully!"
			msgType = MessageSuccess
		} else if msg.err != nil {
			message = fmt.Sprintf("Install failed: %v", msg.err)
			msgType = MessageError
		}
		messageCmd := m.messageBar.SetMessage(message, msgType)
		var contentCmd tea.Cmd
		m.content, contentCmd = m.content.Update(msg)
		return m, tea.Batch(messageCmd, contentCmd)

	case dllUpdatesCheckedMsg:
		m.content, _ = m.content.Update(msg)
		return m, nil

	case launchGameMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "Game launched!"
			msgType = MessageSuccess
		} else if msg.err != nil {
			message = fmt.Sprintf("Launch failed: %v", msg.err)
			msgType = MessageError
		}
		messageCmd := m.messageBar.SetMessage(message, msgType)
		m.content, _ = m.content.Update(msg)
		return m, messageCmd

	case rescanGamesMsg:
		if msg.err != nil {
			messageCmd := m.messageBar.SetMessage(fmt.Sprintf("Rescan failed: %v", msg.err), MessageError)
			return m, messageCmd
		}
		m.db = msg.db
		games := msg.db.List()
		m.sidebar = m.sidebar.SetGames(games)
		messageCmd := m.messageBar.SetMessage(
			fmt.Sprintf("Rescan complete: %d games found", len(games)),
			MessageSuccess,
		)
		var contentCmd tea.Cmd
		if m.content.game != nil && !m.content.defaultProfile {
			if refreshed := msg.db.GetGame(m.content.game.AppID); refreshed != nil {
				m.content = m.content.SetGame(refreshed)
				contentCmd = m.content.LoadDLLUpdates()
			} else {
				m.content = m.content.SetGame(nil)
			}
		}
		return m, tea.Batch(messageCmd, contentCmd)

	case profileSaveMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "Profile saved!"
			msgType = MessageSuccess
		} else if msg.err != nil {
			message = fmt.Sprintf("Error: %v", msg.err)
			msgType = MessageError
		}
		cmd := m.messageBar.SetMessage(message, msgType)
		m.content, _ = m.content.Update(msg)
		return m, cmd

	case optionsSavedMsg:
		m.config = msg.config
		cmd := m.messageBar.SetMessage("Options saved!", MessageSuccess)
		return m, cmd

	case optionsCancelledMsg:
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

	// Panel height: total minus header, status bar, message bar, and borders (2 for top+bottom)
	panelHeight := max(m.height-statusBarHeight-messageBarHeight-headerHeight-2, 5)

	// Inner dimensions account for border (2) and padding
	sidebarInnerWidth := m.sidebarWidth - 4 // -2 for borders, -2 for padding
	contentInnerWidth := m.width - m.sidebarWidth - 4

	m.header.SetWidth(m.width)
	m.sidebar.SetSize(sidebarInnerWidth, panelHeight)
	m.content.SetSize(contentInnerWidth, panelHeight)
	m.statusBar.SetWidth(m.width)
	m.messageBar.SetWidth(m.width)
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

	if m.optionsModal.Visible() {
		return m.renderOptionsOverlay()
	}

	header := m.header.View()

	// Calculate panel height: total height minus header, status bar, message bar, and borders (2 for top+bottom)
	panelHeight := max(m.height-statusBarHeight-messageBarHeight-headerHeight-2, 5)

	// Inner height is panel height minus the border lines
	innerHeight := panelHeight

	// Truncate sidebar and content views to fit within available height
	sidebarView := truncateHeight(m.sidebar.View(), innerHeight)
	contentView := truncateHeight(m.content.View(), innerHeight)

	sidebarStyle := lipgloss.NewStyle().
		Width(m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor(m.focus == FocusSidebar))

	contentStyle := lipgloss.NewStyle().
		Width(m.width - m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor(m.focus == FocusContent))

	sidebar := sidebarStyle.Render(sidebarView)
	content := contentStyle.Render(contentView)

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	messageBar := m.messageBar.View()

	contextHelp := ContextHelp(m.focus, m.sidebar.search.Focused(), m.sidebar.InSelectMode(), m.content.HasGameSelection())
	statusBar := m.statusBar.ViewWithHelp(contextHelp)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, messageBar, statusBar)
}

// truncateHeight limits content to a maximum number of lines.
func truncateHeight(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

func (m LayoutModel) renderHelpOverlay() string {
	overlayStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(2, 4)

	return overlayStyle.Render(m.help.View())
}

func (m LayoutModel) renderOptionsOverlay() string {
	return m.optionsModal.View()
}

type gameSelectedMsg struct {
	game *game.Game
}

type gameConfirmedMsg struct {
	game *game.Game
}

type defaultProfileSelectedMsg struct{}

type defaultProfileConfirmedMsg struct{}

type rescanGamesMsg struct {
	db  *game.Database
	err error
}

func (m LayoutModel) rescanGames() tea.Cmd {
	return func() tea.Msg {
		db, err := game.LoadDatabase()
		if err != nil {
			return rescanGamesMsg{err: err}
		}
		return rescanGamesMsg{db: db}
	}
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
}

func (m LayoutModel) executeBatchAction() tea.Cmd {
	games := m.batchGames

	return func() tea.Msg {
		return executeBatchDLLUpdate(games)
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
