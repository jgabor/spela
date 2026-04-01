package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	styles        *Styles
	header        HeaderModel
	sidebar       SidebarModel
	content       ContentModel
	statusBar     StatusBarModel
	messageBar    MessageBarModel
	help          HelpModel
	optionsModal  OptionsModalModel
	activeDialog  Dialog
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
	initCmd       tea.Cmd
}

func NewLayout(db *game.Database) LayoutModel {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Default()
	}

	theme := DefaultTheme
	switch cfg.Theme {
	case "light":
		theme = LightTheme
	case "dark":
		theme = DarkTheme
	}
	styles := NewStyles(theme, cfg.ShowHints)

	games := db.List()
	sidebar, sidebarCmd := NewSidebar(games, styles)
	return LayoutModel{
		styles:       styles,
		header:       NewHeader(styles),
		sidebar:      sidebar,
		content:      NewContent(styles),
		statusBar:    NewStatusBar(styles),
		messageBar:   NewMessageBar(styles),
		help:         NewHelp(styles),
		optionsModal: NewOptionsModal(styles),
		config:       cfg,
		db:           db,
		focus:        FocusSidebar,
		initCmd:      sidebarCmd,
	}
}

func (m LayoutModel) Init() tea.Cmd {
	return tea.Batch(m.header.Init(), m.initCmd)
}

func (m LayoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Route to active dialog first
	if m.activeDialog != nil {
		var cmd tea.Cmd
		m.activeDialog, cmd = m.activeDialog.Update(msg)
		if !m.activeDialog.Visible() {
			m.activeDialog = nil
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.calculateDimensions()

	case tea.KeyPressMsg:
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
				if m.batchCursor < len(batchActions)-1 {
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
				m.activeDialog = &m.optionsModal
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

	case metricsMsg:
		m.header, _ = m.header.Update(msg)
		return m, nil

	case dllUpdateMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "DLLs updated successfully!"
			msgType = MessageSuccess
			if err := m.db.Save(); err != nil {
				message = fmt.Sprintf("DLLs updated but failed to save database: %v", err)
				msgType = MessageError
			}
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
			// Re-scan DLLs to update versions after restore
			if m.content.game != nil {
				detected, err := dll.ScanDirectory(m.content.game.InstallDir)
				if err == nil {
					m.content.game.DLLs = detected
				}
			}
			if err := m.db.Save(); err != nil {
				message = fmt.Sprintf("DLLs restored but failed to save database: %v", err)
				msgType = MessageError
			}
		} else if msg.err != nil {
			message = fmt.Sprintf("Restore failed: %v", msg.err)
			msgType = MessageError
		}
		messageCmd := m.messageBar.SetMessage(message, msgType)
		var contentCmd tea.Cmd
		m.content, contentCmd = m.content.Update(msg)
		return m, tea.Batch(messageCmd, contentCmd)

	case dllInstallMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "DLL installed successfully!"
			msgType = MessageSuccess
			if err := m.db.Save(); err != nil {
				message = fmt.Sprintf("DLL installed but failed to save database: %v", err)
				msgType = MessageError
			}
		} else if msg.err != nil {
			message = fmt.Sprintf("Install failed: %v", msg.err)
			msgType = MessageError
		}
		messageCmd := m.messageBar.SetMessage(message, msgType)
		var contentCmd tea.Cmd
		m.content, contentCmd = m.content.Update(msg)
		return m, tea.Batch(messageCmd, contentCmd)

	case dllTypesLoadedMsg:
		var contentCmd tea.Cmd
		m.content, contentCmd = m.content.Update(msg)
		return m, contentCmd

	case dllVersionsLoadedMsg:
		var contentCmd tea.Cmd
		m.content, contentCmd = m.content.Update(msg)
		return m, contentCmd

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
		var contentCmd tea.Cmd
		m.content, contentCmd = m.content.Update(msg)
		return m, tea.Batch(messageCmd, contentCmd)

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

	case optionsSaveErrorMsg:
		cmd := m.messageBar.SetMessage(fmt.Sprintf("Failed to save options: %v", msg.err), MessageError)
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

func (m LayoutModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	// Main content always renders as the base layer.
	mainContent := m.renderMain()
	mainLayer := lipgloss.NewLayer(mainContent)

	compositor := lipgloss.NewCompositor(mainLayer)

	// Track how many modal layers are active for cascading offsets.
	modalCount := 0

	if m.showHelp {
		helpContent := m.renderHelpContent()
		helpLayer := m.positionModalLayer(helpContent, "help", 10, modalCount)
		compositor.AddLayers(helpLayer)
		modalCount++
	}

	if m.showBatchMenu {
		batchContent := m.renderBatchContent()
		batchLayer := m.positionModalLayer(batchContent, "batch", 20, modalCount)
		compositor.AddLayers(batchLayer)
		modalCount++
	}

	if m.activeDialog != nil {
		dialogContent := m.activeDialog.View()
		dialogLayer := m.positionModalLayer(dialogContent, "dialog", 30, modalCount)
		compositor.AddLayers(dialogLayer)
	}

	content := compositor.Render()

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m LayoutModel) renderMain() string {
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
		BorderForeground(m.styles.BorderColor(m.focus == FocusSidebar))

	contentStyle := lipgloss.NewStyle().
		Width(m.width - m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.BorderColor(m.focus == FocusContent))

	sidebar := sidebarStyle.Render(sidebarView)
	content := contentStyle.Render(contentView)

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	messageBar := m.messageBar.View()

	contextHelp := ContextHelp(m.focus, m.sidebar.search.Focused(), m.sidebar.InSelectMode(), m.content.HasGameSelection(), m.styles.ShowHints)
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

func (m LayoutModel) renderHelpContent() string {
	return m.renderModalBox("", m.help.View(), 0.55)
}

// Modal rendering constants.
const (
	cascadeOffsetX = 2
	cascadeOffsetY = 1
	minModalWidth  = 40
	maxModalWidth  = 70
)

// renderModalBox wraps content in a bordered, styled modal box.
// When title is non-empty it is rendered as a bold header above the content.
func (m LayoutModel) renderModalBox(title, content string, widthRatio float64) string {
	t := m.styles.Theme
	modalWidth := int(float64(m.width) * widthRatio)
	modalWidth = max(modalWidth, minModalWidth)
	modalWidth = min(modalWidth, maxModalWidth)

	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2)

	var body string
	if title != "" {
		titleStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		body = titleStyle.Render(title) + "\n\n" + content
	} else {
		body = content
	}

	return modalStyle.Render(body)
}

// positionModalLayer creates a centered Layer for a modal string, applying a
// cascading offset based on how many modals are already stacked.
func (m LayoutModel) positionModalLayer(content, id string, zIndex, stackIndex int) *lipgloss.Layer {
	layer := lipgloss.NewLayer(content)
	contentWidth := lipgloss.Width(content)
	contentHeight := lipgloss.Height(content)

	x := centerX(m.width, contentWidth) + stackIndex*cascadeOffsetX
	y := centerY(m.height, contentHeight) + stackIndex*cascadeOffsetY

	return layer.
		X(x).
		Y(y).
		Z(zIndex).
		ID(id)
}

func centerX(totalWidth, contentWidth int) int {
	return max((totalWidth-contentWidth)/2, 0)
}

func centerY(totalHeight, contentHeight int) int {
	return max((totalHeight-contentHeight)/2, 0)
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
	p := tea.NewProgram(NewLayout(db))
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

		gameDLLs := dll.GameDLLsFromDetected(g.DLLs)

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

func (m LayoutModel) renderBatchContent() string {
	return m.renderModalBox("", m.renderBatchMenu(), 0.40)
}

func (m LayoutModel) renderBatchMenu() string {
	var b strings.Builder

	s := m.styles

	b.WriteString(s.Title.Render(fmt.Sprintf("Batch action (%d games)", len(m.batchGames))))
	b.WriteString("\n\n")

	for i, action := range batchActions {
		cursor := "  "
		style := s.Normal
		if i == m.batchCursor {
			cursor = "> "
			style = s.Selected
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, action)))
		b.WriteString("\n")
	}

	if m.batchMessage != "" {
		b.WriteString("\n")
		b.WriteString(s.Success.Render(m.batchMessage))
		b.WriteString("\n")
	}

	if hint := s.RenderHint("\n\n↑/↓ select • enter execute • esc cancel"); hint != "" {
		b.WriteString(hint)
	}

	return b.String()
}
