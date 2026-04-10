package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

// DensityMode controls the information density of the TUI layout.
type DensityMode int

const (
	DensityStandard DensityMode = iota
	DensityCompact
	DensityFocused
)

const (
	minSidebarWidth     = 25
	maxSidebarWidth     = 50
	sidebarRatio        = 0.30
	statusBarHeight     = 1
	messageBarHeight    = 1
	headerHeight        = 7 // 6 lines for logo + 1 for bottom border
	compactHeaderHeight = 3 // 2 metric lines + 1 bottom border
)

type LayoutModel struct {
	styles         *Styles
	services       *Services
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
	densityMode    DensityMode
	width          int
	height         int
	sidebarWidth   int
	initCmd        tea.Cmd
}

func NewLayout(db *game.Database, svc *Services) LayoutModel {
	cfg, _ := svc.LoadConfig()
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
	sidebar, sidebarCmd := NewSidebar(games, styles, svc)
	return LayoutModel{
		styles:         styles,
		services:       svc,
		header:         NewHeader(styles),
		sidebar:        sidebar,
		stack:          NewNavStack(newContentEntry(NewContent(styles, cfg.ConfirmDestructive, svc))),
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
		case "f5":
			if m.densityMode == DensityCompact {
				m.densityMode = DensityStandard
			} else {
				m.densityMode = DensityCompact
			}
			m.calculateDimensions()
			return m, nil
		case "f11":
			if m.densityMode == DensityFocused {
				m.densityMode = DensityStandard
			} else {
				m.densityMode = DensityFocused
			}
			m.calculateDimensions()
			return m, nil
		case "1":
			if !m.sidebarFocused && !m.sidebar.search.Focused() && m.densityMode != DensityFocused {
				m.sidebarFocused = true
				return m, nil
			}
		case "2", "3", "4":
			// Global jump keys — switch to content tab from anywhere.
			cm := m.contentModel()
			if cm != nil && cm.game != nil && !cm.defaultProfile && !cm.profileWidget.Editing() {
				m.sidebarFocused = false
				switch msg.String() {
				case "2":
					cm.activeTab = TabDLLs
				case "3":
					cm.activeTab = TabProfile
				case "4":
					cm.activeTab = TabLaunch
				}
				return m, nil
			}
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

	case flashTickMsg:
		var cmd tea.Cmd
		m.messageBar, cmd = m.messageBar.Update(msg)
		return m, cmd

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
	// Sidebar width adjusts in compact mode.
	if m.densityMode == DensityCompact {
		m.sidebarWidth = max(int(float64(m.width)*0.25), minSidebarWidth)
	} else {
		m.sidebarWidth = int(float64(m.width) * sidebarRatio)
		m.sidebarWidth = max(m.sidebarWidth, minSidebarWidth)
		m.sidebarWidth = min(m.sidebarWidth, maxSidebarWidth)
	}

	// Panel height depends on header size.
	headerH := headerHeight
	switch m.densityMode {
	case DensityCompact:
		headerH = compactHeaderHeight
	case DensityFocused:
		headerH = 0
	}

	panelHeight := max(m.height-statusBarHeight-messageBarHeight-headerH-2, 5)

	// Inner dimensions account for border (2) and padding.
	sidebarInnerWidth := m.sidebarWidth - 4

	m.header.SetWidth(m.width)
	m.sidebar.SetSize(sidebarInnerWidth, panelHeight)
	m.stack.Top().SetSize(m.contentWidth(), m.contentHeight())
	m.statusBar.SetWidth(m.width)
	m.messageBar.SetWidth(m.width)
}

// contentWidth returns the inner width available for the content area.
func (m LayoutModel) contentWidth() int {
	if m.densityMode == DensityFocused {
		return m.width - 4
	}
	return m.width - m.sidebarWidth - 4
}

// contentHeight returns the inner height available for the content area.
func (m LayoutModel) contentHeight() int {
	headerH := headerHeight
	switch m.densityMode {
	case DensityCompact:
		headerH = compactHeaderHeight
	case DensityFocused:
		headerH = 0
	}
	return max(m.height-statusBarHeight-messageBarHeight-headerH-2, 5)
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
	content := NewContent(m.styles, m.config.ConfirmDestructive, m.services)
	content = content.SetGame(g)
	content.SetSize(m.contentWidth(), m.contentHeight())
	return newContentEntry(content)
}

// contentEntryForDefaultProfile creates a new content entry configured for the default profile.
func (m *LayoutModel) contentEntryForDefaultProfile() *contentEntry {
	content := NewContent(m.styles, m.config.ConfirmDestructive, m.services)
	content = content.SetDefaultProfile()
	content.SetSize(m.contentWidth(), m.contentHeight())
	return newContentEntry(content)
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
	switch m.densityMode {
	case DensityFocused:
		return m.renderFocused()
	case DensityCompact:
		return m.renderCompact()
	default:
		return m.renderStandard()
	}
}

func (m LayoutModel) renderStandard() string {
	t := m.styles.Theme
	header := m.header.View()

	panelHeight := max(m.height-statusBarHeight-messageBarHeight-headerHeight-2, 5)
	innerHeight := panelHeight

	sidebarView := truncateHeight(m.sidebar.View(), innerHeight)
	contentView := truncateHeight(m.stack.Top().View(), innerHeight)

	sidebarBorderColor := m.styles.BorderColor(m.sidebarFocused)
	contentBorderColor := m.styles.BorderColor(!m.sidebarFocused)

	sidebarStyle := lipgloss.NewStyle().
		Width(m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(sidebarBorderColor)

	contentStyle := lipgloss.NewStyle().
		Width(m.width - m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(contentBorderColor)

	sidebar := sidebarStyle.Render(sidebarView)
	content := contentStyle.Render(contentView)

	// Build custom top borders with jump-key titles.
	jumpKeyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	sidebarTitle := jumpKeyStyle.Render("[1]") + labelStyle.Render(" Games")
	contentTitle := jumpKeyStyle.Render("[2]") + labelStyle.Render(" Details")

	sidebarTopBorder := buildTopBorder(sidebarTitle, m.sidebarWidth-2, sidebarBorderColor)
	contentTopBorder := buildTopBorder(contentTitle, m.width-m.sidebarWidth-2, contentBorderColor)

	sidebar = sidebarTopBorder + "\n" + sidebar
	content = contentTopBorder + "\n" + content

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	messageBar := m.messageBar.View()

	keys := ContextKeys(m.sidebarFocused, m.sidebar.search.Focused(), m.sidebar.InSelectMode(), m.contentModel(), m.styles.ShowHints)
	contextHelp := RenderContextBar(keys, m.width/2, &m.styles.Theme)
	crumbs := m.renderBreadcrumbs()
	statusBar := m.statusBar.ViewWithHelp(crumbs + "  " + contextHelp)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, messageBar, statusBar)
}

func (m LayoutModel) renderCompact() string {
	t := m.styles.Theme
	header := m.header.ViewCompact()

	panelHeight := max(m.height-statusBarHeight-messageBarHeight-compactHeaderHeight-2, 5)
	innerHeight := panelHeight

	sidebarView := truncateHeight(m.sidebar.View(), innerHeight)
	contentView := truncateHeight(m.stack.Top().View(), innerHeight)

	sidebarBorderColor := m.styles.BorderColor(m.sidebarFocused)
	contentBorderColor := m.styles.BorderColor(!m.sidebarFocused)

	sidebarStyle := lipgloss.NewStyle().
		Width(m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(sidebarBorderColor)

	contentStyle := lipgloss.NewStyle().
		Width(m.width - m.sidebarWidth - 2).
		Height(innerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(contentBorderColor)

	sidebar := sidebarStyle.Render(sidebarView)
	content := contentStyle.Render(contentView)

	// Jump-key titles.
	jumpKeyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	sidebarTitle := jumpKeyStyle.Render("[1]") + labelStyle.Render(" Games")
	contentTitle := jumpKeyStyle.Render("[2]") + labelStyle.Render(" Details")

	sidebarTopBorder := buildTopBorder(sidebarTitle, m.sidebarWidth-2, sidebarBorderColor)
	contentTopBorder := buildTopBorder(contentTitle, m.width-m.sidebarWidth-2, contentBorderColor)

	sidebar = sidebarTopBorder + "\n" + sidebar
	content = contentTopBorder + "\n" + content

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	messageBar := m.messageBar.View()

	keys := ContextKeys(m.sidebarFocused, m.sidebar.search.Focused(), m.sidebar.InSelectMode(), m.contentModel(), m.styles.ShowHints)
	contextHelp := RenderContextBar(keys, m.width/2, &m.styles.Theme)
	crumbs := m.renderBreadcrumbs()
	statusBar := m.statusBar.ViewWithHelp(crumbs + "  " + contextHelp)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, messageBar, statusBar)
}

func (m LayoutModel) renderFocused() string {
	contentHeight := max(m.height-statusBarHeight-messageBarHeight-2, 5)

	contentView := truncateHeight(m.stack.Top().View(), contentHeight)

	contentBorderColor := m.styles.BorderColor(true)

	contentStyle := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(contentHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(contentBorderColor)

	content := contentStyle.Render(contentView)

	messageBar := m.messageBar.View()

	escHint := lipgloss.NewStyle().Foreground(m.styles.Theme.TextDim).Render("F11:exit focused  ?:help  q:quit")
	statusBar := m.statusBar.ViewWithHelp(escHint)

	return lipgloss.JoinVertical(lipgloss.Left, content, messageBar, statusBar)
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

// buildTopBorder builds a rounded top border line with a styled title embedded.
// The title is placed after the opening corner, and the remaining width is filled with dashes.
func buildTopBorder(title string, totalWidth int, borderColor color.Color) string {
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	cornerLeft := borderStyle.Render("╭")
	cornerRight := borderStyle.Render("╮")
	titleStr := " " + title + " "

	titleVisualWidth := lipgloss.Width(titleStr)
	// Fill remaining width: totalWidth - 1 (left corner) - titleWidth - 1 (right corner)
	fillWidth := totalWidth - titleVisualWidth - 2
	if fillWidth < 0 {
		fillWidth = 0
	}
	fill := borderStyle.Render(strings.Repeat("─", fillWidth))

	return cornerLeft + titleStr + fill + cornerRight
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
	p := tea.NewProgram(NewLayout(db, DefaultServices()))
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
