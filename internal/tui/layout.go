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
	"github.com/jgabor/spela/internal/nav"
)

// DensityMode controls the information density of the TUI layout.
type DensityMode int

const (
	DensityStandard DensityMode = iota
	DensityCompact
	DensityFocused
)

const (
	listPaneWidth         = 32
	minimumTerminalWidth  = 80
	minimumTerminalHeight = 24
	statusBarHeight       = 2
	messageBarHeight      = 1
	headerHeight          = 7 // 6 lines for logo + 1 for bottom border
	compactHeaderHeight   = 3 // 2 metric lines + 1 bottom border
)

// LayoutModel owns the non-focusable destination bar and the two-pane
// List/Detail workspace.
type LayoutModel struct {
	dllCatalogReturnFocus *KeyFocus
	styles                *Styles
	services              *Services
	header                HeaderModel
	destinationBar        DestinationBarModel
	listPane              ListPaneModel
	pane                  resourcePaneModel
	navState              *nav.State
	focus                 KeyFocus
	inputMode             InputMode
	messageBar            MessageBarModel
	help                  HelpModel
	config                *config.Config
	db                    *game.Database
	pendingRescan         *rescanGamesMsg
	rescanBusy            bool
	rescanRequestID       uint64
	decision              *decisionDialog
	searchSession         *searchSession
	batchCloseFocused     bool
	batchScrollOffset     int
	actions               *actionMenu
	showHelp              bool
	showBatchMenu         bool
	batchGames            []*game.Game
	batchCursor           int
	batchMessage          string
	batchConfirmation     *dllMutationConfirmation
	batchBusy             bool
	densityMode           DensityMode
	width                 int
	height                int
	initCmd               tea.Cmd
}

func NewLayout(db *game.Database, svc *Services) LayoutModel {
	cfg, _ := svc.LoadConfig()
	if cfg == nil {
		cfg = config.Default()
	}

	styles := NewStyles(DefaultTheme, cfg.ShowHints)

	games := db.List()
	sidebar, _ := NewSidebar(games, styles, svc)
	content := NewContent(styles, true, svc)
	content.database = db
	pane := newResourcePane(styles, content)
	pane.dllsResource.database = db
	pane.setServices(svc)
	manifest, _ := dll.LoadManifest()
	pane.SetDLLsData(games, manifest)

	settings := NewOptionsModal(styles)
	settings.SetSaveConfig(svc.SaveConfig)
	settings.OpenEmbedded(cfg)
	pane.settings = settings

	state := nav.DefaultState()
	layout := LayoutModel{
		styles:         styles,
		services:       svc,
		header:         NewHeader(styles),
		destinationBar: NewDestinationBar(styles),
		pane:           pane,
		navState:       &state,
		focus:          FocusList,
		inputMode:      ModeBrowse,
		messageBar:     NewMessageBar(styles),
		help:           NewHelp(styles),
		config:         cfg,
		db:             db,
	}
	layout.pane.BindNavState(layout.navState)
	if cfg.RescanOnStartup || len(db.Games) == 0 {
		layout.rescanBusy = true
		layout.rescanRequestID = 1
	}
	layout.listPane = NewListPane(styles, sidebar, layout.navState)
	if selected := sidebar.Selected(); selected != nil {
		layout.pane.loadGameScope(selected)
	} else {
		layout.pane.loadNoScope()
	}
	return layout
}

func (m LayoutModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.header.Init(), m.initCmd, m.pane.content.LoadDLLUpdates()}
	if m.config.RescanOnStartup || len(m.db.Games) == 0 {
		cmds = append(
			cmds,
			m.messageBar.SetMessage("Scanning games...", MessageInfo),
			m.rescanGames(),
		)
	}
	return tea.Batch(cmds...)
}

func (m LayoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Always update header (metrics ticks).
	header, headerCmd := m.header.Update(msg)
	m.header = header
	cmds = append(cmds, headerCmd)
	// Relay the header's latest sample buffers and snapshot to the Metrics
	// resource so its sparklines and gauges reflect the same data the
	// header renders. No separate polling loop (Task 6 relocation preserves
	// the single metrics source-of-truth).
	m.pane.SetMetricsData(m.header)

	// Resize is the only message that reaches the hidden workspace while the
	// terminal is below the supported minimum.
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = msg.Width, msg.Height
		m.calculateDimensions()
		return m, tea.Batch(cmds...)
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if m.width < minimumTerminalWidth || m.height < minimumTerminalHeight {
			if key.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, tea.Batch(cmds...)
		}
		next, command := m.routeKey(key)
		cmds = append(cmds, command)
		if next.pendingRescan != nil && next.bindingContext().Mode == ModeBrowse {
			pending := *next.pendingRescan
			next.pendingRescan = nil
			next, cmds = next.handleRescanGamesMsg(pending, cmds)
		}
		return next, tea.Batch(cmds...)
	}

	// Route application messages that affect multiple components.
	m, cmds = m.handleAppMessages(msg, cmds)
	switch msg.(type) {
	case profileSaveMsg, optionsSavedMsg, optionsSaveErrorMsg, dllTypesLoadedMsg, dllVersionsLoadedMsg, dllUpdateMsg, dllRestoreMsg, dllInstallMsg, dllUpdatesCheckedMsg, gameSelectedMsg, gameConfirmedMsg, defaultProfileSelectedMsg, defaultProfileConfirmedMsg, noLibrarySelectionMsg, rescanGamesMsg, batchCompleteMsg, batchActionRequestMsg:
	default:
		var command tea.Cmd
		m.pane, command = m.pane.Update(msg)
		cmds = append(cmds, command)
		if _, completed := msg.(dllsUpdateAllCompleteMsg); completed {
			m.synchronizeDLLGameReferences()
			cmds = append(cmds, m.pane.content.LoadDLLUpdates())
		}
	}
	next, command := m.advanceDecision(msg)
	cmds = append(cmds, command)
	if next.pendingRescan != nil && next.bindingContext().Mode == ModeBrowse {
		pending := *next.pendingRescan
		next.pendingRescan = nil
		next, cmds = next.handleRescanGamesMsg(pending, cmds)
	}
	return next, tea.Batch(cmds...)
}

func (m *LayoutModel) selectDestination(destination nav.Destination) {
	m.navState.Destination = destination
	m.focus = FocusList
	m.inputMode = ModeBrowse
	m.syncNavToComponents()
	if m.navState.Destination == nav.DestinationSettings {
		m.pane.settings.OpenEmbedded(m.config)
	}
}

func (m *LayoutModel) syncNavToComponents() {
	m.destinationBar.SetActive(m.navState.Destination)
	m.listPane.SetState(*m.navState)
	m.pane.SetState(*m.navState)
}

func (m *LayoutModel) calculateDimensions() {
	m.header.SetWidth(m.width)
	m.destinationBar.SetWidth(m.width)
	contentHeight := max(m.workspaceHeight()-2, 1)
	if m.singlePane() {
		m.listPane.SetSize(m.width, contentHeight)
		m.pane.SetSize(max(m.width-2, 1), contentHeight)
	} else {
		m.listPane.SetSize(m.listWidth(), contentHeight)
		m.pane.SetSize(max(m.detailWidth()-2, 1), contentHeight)
	}
	m.messageBar.SetWidth(m.width)
	m.help.SetSize(max(m.width-10, 1), max(m.height-10, 3))
}

func (m LayoutModel) listWidth() int   { return min(listPaneWidth, max(m.width/3, 24)) }
func (m LayoutModel) detailWidth() int { return max(m.width-m.listWidth(), 1) }

func (m LayoutModel) View() tea.View {
	content := "Loading..."
	if m.width > 0 && m.height > 0 {
		if m.width < minimumTerminalWidth || m.height < minimumTerminalHeight {
			content = m.renderResizePrompt()
		} else if m.actions != nil {
			content = m.renderOverlay(m.renderActions())
		} else if m.decision != nil {
			content = m.renderOverlay(m.renderDecision())
		} else if m.showHelp {
			content = m.renderOverlay(m.help.View())
		} else if m.showBatchMenu {
			content = m.renderOverlay(m.renderBatchMenu())
		} else {
			content = m.renderMain()
		}
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m LayoutModel) renderResizePrompt() string {
	lines := []string{
		"Resize terminal",
		fmt.Sprintf("Spela needs at least %dx%d", minimumTerminalWidth, minimumTerminalHeight),
		fmt.Sprintf("Current size: %dx%d", m.width, m.height),
	}
	for index := range lines {
		lines[index] = truncatePlainText(lines[index], m.width)
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func truncatePlainText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width])
}

func (m LayoutModel) renderMain() string { return m.renderWorkspace() }

func (m LayoutModel) visibleListView(focused bool) string {
	switch m.navState.Destination {
	case nav.DestinationDLLCatalog:
		return m.pane.dllsResource.ListView(focused, m.navState.DLLCatalogSection)
	case nav.DestinationSettings:
		return m.pane.settings.ListView(focused)
	default:
		return m.listPane.View(focused)
	}
}

func (m LayoutModel) renderCanonicalStatus() string { return strings.Join(m.footerLines(), "\n") }

// renderBreadcrumbs renders the navigation breadcrumb trail for the status bar.
func (m LayoutModel) renderBreadcrumbs() string {
	t := m.styles.Theme
	activeStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	trailStyle := lipgloss.NewStyle().Foreground(t.TextDim)
	sepStyle := lipgloss.NewStyle().Foreground(t.Border)

	segments := m.navState.Breadcrumb()
	parts := []string{trailStyle.Render("spela")}
	for i, segment := range segments {
		parts = append(parts, sepStyle.Render(" › "))
		if i == len(segments)-1 {
			parts = append(parts, activeStyle.Render(segment))
		} else {
			parts = append(parts, trailStyle.Render(segment))
		}
	}
	return strings.Join(parts, "")
}

func paneColumnTitle(name string, focused bool) string {
	if focused {
		return "▸ " + name
	}
	return name
}

// buildTopBorder builds a rounded top border line with a styled title embedded.
func buildTopBorder(title string, totalWidth int, borderColor color.Color) string {
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	cornerLeft := borderStyle.Render("╭")
	cornerRight := borderStyle.Render("╮")
	titleStr := " " + title + " "

	titleVisualWidth := lipgloss.Width(titleStr)
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

// Modal rendering constants.
const (
	cascadeOffsetX = 2
	cascadeOffsetY = 1
	minModalWidth  = 40
	maxModalWidth  = 70
)

// renderModalBox wraps content in a bordered, styled modal box.
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

type gameSelectedMsg struct {
	game *game.Game
}

type gameConfirmedMsg struct {
	game *game.Game
}

type defaultProfileSelectedMsg struct{}

type noLibrarySelectionMsg struct{}

type defaultProfileConfirmedMsg struct{}

type rescanGamesMsg struct {
	db        *game.Database
	err       error
	requestID uint64
}

func (m LayoutModel) rescanGames() tea.Cmd {
	cfg := m.config.Clone()
	requestID := m.rescanRequestID
	scanGames := m.services.ScanGames
	return func() tea.Msg {
		db, err := scanGames(cfg)
		if err != nil {
			return rescanGamesMsg{err: err, requestID: requestID}
		}
		return rescanGamesMsg{db: db, requestID: requestID}
	}
}

func Run(db *game.Database) error {
	p := tea.NewProgram(NewLayout(db, DefaultServices()))
	_, err := p.Run()
	return err
}

type batchCompleteMsg struct {
	message string
	failed  bool
	batch   dll.BatchResult
}

func (m LayoutModel) executeBatchAction(targets []dllMutationTarget) tea.Cmd {
	requests := dllUpdateRequests(targets, false)
	requested := len(m.batchGames)
	return func() tea.Msg {
		return summarizeBatchDLLUpdate(m.services.BatchUpdateDLLs(requests), requested)
	}
}

func executeBatchDLLUpdate(appIDs []uint64) batchCompleteMsg {
	return summarizeBatchDLLUpdate(dll.UpdateGames(appIDs, "", nil), len(appIDs))
}

func summarizeBatchDLLUpdate(batch dll.BatchResult, requested int) batchCompleteMsg {
	updatedGames := make(map[uint64]bool)
	for _, item := range batch.Items {
		if item.Err == nil && item.Result.Outcome == dll.OutcomeChanged && item.Result.Game != nil {
			updatedGames[item.Result.Game.AppID] = true
		}
	}
	message := fmt.Sprintf("Updated DLLs for %d/%d games", len(updatedGames), requested)
	if batch.Updated == 0 && batch.Failed == 0 {
		message = "Selected games are already current"
	}
	if batch.Failed > 0 {
		message = formatDLLBatch(batch)
	}
	return batchCompleteMsg{message: message, failed: batch.Failed > 0, batch: batch}
}

func (m LayoutModel) renderBatchContent() string {
	return m.renderModalBox("", m.renderBatchMenu(), 0.40)
}

func (m LayoutModel) renderBatchMenu() string {
	if m.batchConfirmation != nil {
		return m.batchConfirmation.viewSized(m.styles, max(m.width-12, 1), max(m.height-10, 1))
	}
	title := fmt.Sprintf("Batch action (%d visible games)", len(m.batchGames))
	if m.batchBusy {
		return m.styles.Title.Render(title) + "\n\nUpdating DLLs... Please wait."
	}
	if m.batchMessage != "" {
		return dllResultView(m.styles, title, m.batchMessage, !m.batchCloseFocused, m.batchScrollOffset, max(m.width-12, 1), max(m.height-10, 1))
	}
	count := len(latestDLLMutationTargets(m.batchGames, m.pane.dllsResource.manifest))
	body := renderControl(fmt.Sprintf("Update selected games (%d DLLs)", count), !m.batchCloseFocused, m.styles)
	hint := "Tab: next control"
	if count == 0 {
		body += "\n" + m.styles.Dim.Render("No available updates for these visible selected games")
	}
	if m.batchCloseFocused {
		hint += "  Enter: close"
	} else if count > 0 {
		hint += "  Enter: open confirmation"
	}
	return m.styles.Title.Render(title) + "\n\n" + body + "\n\n" + renderControl("Close", m.batchCloseFocused, m.styles) + "\n" + m.styles.Dim.Render(hint)
}
