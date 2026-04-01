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

const (
	minSidebarWidth  = 25
	maxSidebarWidth  = 50
	sidebarRatio     = 0.30
	statusBarHeight  = 1
	messageBarHeight = 1
	headerHeight     = 7 // 6 lines for logo + 1 for bottom border
)

type LayoutModel struct {
	styles         *Styles
	header         HeaderModel
	sidebar        SidebarModel
	stack          NavStack
	statusBar      StatusBarModel
	messageBar     MessageBarModel
	help           HelpModel
	optionsModal   OptionsModalModel
	activeDialog   Dialog
	config         *config.Config
	db             *game.Database
	showHelp       bool
	showBatchMenu  bool
	batchGames     []*game.Game
	batchCursor    int
	batchMessage   string
	sidebarFocused bool
	width          int
	height         int
	sidebarWidth   int
	initCmd        tea.Cmd
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
		styles:         styles,
		header:         NewHeader(styles),
		sidebar:        sidebar,
		stack:          NewNavStack(newContentEntry(NewContent(styles))),
		statusBar:      NewStatusBar(styles),
		messageBar:     NewMessageBar(styles),
		help:           NewHelp(styles),
		optionsModal:   NewOptionsModal(styles),
		config:         cfg,
		db:             db,
		sidebarFocused: true,
		initCmd:        sidebarCmd,
	}
}

func (m LayoutModel) Init() tea.Cmd {
	return tea.Batch(m.header.Init(), m.initCmd)
}

func (m LayoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Always update header (metrics ticks).
	header, headerCmd := m.header.Update(msg)
	m.header = header
	cmds = append(cmds, headerCmd)

	// Handle navigation messages from stack entries.
	switch msg := msg.(type) {
	case pushMsg:
		msg.entry.SetSize(m.contentWidth(), m.contentHeight())
		m.stack.Push(msg.entry)
		m.sidebarFocused = false
		return m, nil
	case popMsg:
		m.stack.Pop()
		m.stack.Top().SetSize(m.contentWidth(), m.contentHeight())
		if msg.result != nil {
			return m.Update(msg.result)
		}
		return m, nil
	}

	// Route to active dialog first (intercepts all input).
	if m.activeDialog != nil {
		var cmd tea.Cmd
		m.activeDialog, cmd = m.activeDialog.Update(msg)
		if !m.activeDialog.Visible() {
			m.activeDialog = nil
		}
		return m, cmd
	}

	// Handle global overlays and keys.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.calculateDimensions()
		return m, tea.Batch(cmds...)

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
			if m.sidebarFocused && !m.sidebar.search.Focused() {
				m.optionsModal.SetSize(m.width, m.height)
				m.optionsModal.Open(m.config)
				m.activeDialog = &m.optionsModal
				return m, nil
			}
		case "ctrl+f":
			m.sidebarFocused = true
			var cmd tea.Cmd
			m.sidebar, cmd = m.sidebar.FocusSearch()
			return m, cmd
		case "q":
			if m.sidebarFocused && !m.sidebar.search.Focused() {
				return m, tea.Quit
			}
			if !m.sidebarFocused && !m.contentModel().HasModalOpen() {
				if m.stack.Depth() > 1 {
					m.stack.Pop()
					return m, nil
				}
				m.sidebarFocused = true
				return m, nil
			}
		case "esc":
			if !m.sidebarFocused && !m.contentModel().HasModalOpen() {
				if m.stack.Depth() > 1 {
					m.stack.Pop()
					return m, nil
				}
				m.sidebarFocused = true
				return m, nil
			}
		case "tab":
			m.sidebarFocused = !m.sidebarFocused
			return m, nil
		case "r":
			if m.sidebarFocused && !m.sidebar.search.Focused() {
				messageCmd := m.messageBar.SetMessage("Rescanning games...", MessageInfo)
				return m, tea.Batch(messageCmd, m.rescanGames())
			}
		}
	}

	// Route application messages that affect multiple components.
	switch msg := msg.(type) {
	case gameSelectedMsg:
		entry := m.contentEntryForGame(msg.game)
		m.stack.Replace(entry)
		return m, entry.model.LoadDLLUpdates()

	case gameConfirmedMsg:
		entry := m.contentEntryForGame(msg.game)
		m.stack.Replace(entry)
		m.sidebarFocused = false
		return m, entry.model.LoadDLLUpdates()

	case defaultProfileSelectedMsg:
		entry := m.contentEntryForDefaultProfile()
		m.stack.Replace(entry)
		return m, nil

	case defaultProfileConfirmedMsg:
		entry := m.contentEntryForDefaultProfile()
		m.stack.Replace(entry)
		m.sidebarFocused = false
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
		// Already handled above in header update.
		return m, tea.Batch(cmds...)

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
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
		return m, tea.Batch(messageCmd, contentCmd)

	case dllRestoreMsg:
		var msgType MessageType
		var message string
		if msg.success {
			message = "Original DLLs restored!"
			msgType = MessageSuccess
			if cm := m.contentModel(); cm.game != nil {
				detected, err := dll.ScanDirectory(cm.game.InstallDir)
				if err == nil {
					cm.game.DLLs = detected
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
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
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
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
		return m, tea.Batch(messageCmd, contentCmd)

	case dllTypesLoadedMsg:
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
		return m, contentCmd

	case dllVersionsLoadedMsg:
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
		return m, contentCmd

	case dllUpdatesCheckedMsg:
		updated, _ := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
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
		updated, contentCmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
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
		if cm := m.contentModel(); cm.game != nil && !cm.defaultProfile {
			if refreshed := msg.db.GetGame(cm.game.AppID); refreshed != nil {
				entry := m.contentEntryForGame(refreshed)
				m.stack.Replace(entry)
				contentCmd = entry.model.LoadDLLUpdates()
			} else {
				entry := m.contentEntryForGame(nil)
				m.stack.Replace(entry)
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
		updated, _ := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
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

	// Route input to focused component.
	if m.sidebarFocused {
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		updated, cmd := m.stack.Top().Update(msg)
		m.stack.Replace(updated)
		cmds = append(cmds, cmd)
	}

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

	m.header.SetWidth(m.width)
	m.sidebar.SetSize(sidebarInnerWidth, panelHeight)
	m.stack.Top().SetSize(m.contentWidth(), m.contentHeight())
	m.statusBar.SetWidth(m.width)
	m.messageBar.SetWidth(m.width)
}

// contentWidth returns the inner width available for the content area.
func (m LayoutModel) contentWidth() int {
	return m.width - m.sidebarWidth - 4
}

// contentHeight returns the inner height available for the content area.
func (m LayoutModel) contentHeight() int {
	return max(m.height-statusBarHeight-messageBarHeight-headerHeight-2, 5)
}

// contentModel returns the ContentModel from the current stack top.
// This is a convenience accessor for layout-level logic that needs
// to inspect content state (e.g. game, defaultProfile, HasModalOpen).
func (m LayoutModel) contentModel() *ContentModel {
	if entry, ok := m.stack.Top().(*contentEntry); ok {
		return &entry.model
	}
	return nil
}

// contentEntryForGame creates a new content entry configured for the given game.
func (m *LayoutModel) contentEntryForGame(g *game.Game) *contentEntry {
	content := NewContent(m.styles)
	content = content.SetGame(g)
	content.SetSize(m.contentWidth(), m.contentHeight())
	return newContentEntry(content)
}

// contentEntryForDefaultProfile creates a new content entry configured for the default profile.
func (m *LayoutModel) contentEntryForDefaultProfile() *contentEntry {
	content := NewContent(m.styles)
	content = content.SetDefaultProfile()
	content.SetSize(m.contentWidth(), m.contentHeight())
	return newContentEntry(content)
}

func (m LayoutModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	var content string

	if m.activeDialog != nil {
		content = m.activeDialog.View()
	} else if m.showBatchMenu {
		content = m.renderBatchOverlay()
	} else if m.showHelp {
		content = m.renderHelpOverlay()
	} else {
		content = m.renderMain()
	}

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
	contentView := truncateHeight(m.stack.Top().View(), innerHeight)

	sidebarStyle := lipgloss.NewStyle().
		Width(m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.BorderColor(m.sidebarFocused))

	contentStyle := lipgloss.NewStyle().
		Width(m.width - m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.BorderColor(!m.sidebarFocused))

	sidebar := sidebarStyle.Render(sidebarView)
	content := contentStyle.Render(contentView)

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	messageBar := m.messageBar.View()

	keys := ContextKeys(m.sidebarFocused, m.sidebar.search.Focused(), m.sidebar.InSelectMode(), m.contentModel(), m.styles.ShowHints)
	contextHelp := RenderContextBar(keys, m.width/2, &m.styles.Theme)
	crumbs := m.renderBreadcrumbs()
	statusBar := m.statusBar.ViewWithHelp(crumbs + "  " + contextHelp)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, messageBar, statusBar)
}

// renderBreadcrumbs renders the navigation breadcrumb trail for the status bar.
func (m LayoutModel) renderBreadcrumbs() string {
	crumbs := m.stack.Breadcrumbs()
	t := m.styles.Theme
	activeStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	trailStyle := lipgloss.NewStyle().Foreground(t.TextDim)
	sepStyle := lipgloss.NewStyle().Foreground(t.Border)

	parts := make([]string, 0, len(crumbs)*2+1)
	parts = append(parts, trailStyle.Render("spela"))
	for i, name := range crumbs {
		parts = append(parts, sepStyle.Render(" > "))
		if i == len(crumbs)-1 {
			parts = append(parts, activeStyle.Render(name))
		} else {
			parts = append(parts, trailStyle.Render(name))
		}
	}
	return strings.Join(parts, "")
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

func (m LayoutModel) renderBatchOverlay() string {
	overlayStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(2, 4)

	return overlayStyle.Render(m.renderBatchMenu())
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
