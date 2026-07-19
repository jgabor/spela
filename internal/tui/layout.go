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
	primaryNavWidth     = 20
	contextNavWidth     = 28
	statusBarHeight     = 1
	messageBarHeight    = 1
	headerHeight        = 7 // 6 lines for logo + 1 for bottom border
	compactHeaderHeight = 3 // 2 metric lines + 1 bottom border
)

// LayoutModel is the three-zone shell: primary nav, context nav, content.
type LayoutModel struct {
	styles        *Styles
	services      *Services
	header        HeaderModel
	rail          RailModel
	contextNav    ContextNavModel
	pane          resourcePaneModel
	navState      *nav.State
	messageBar    MessageBarModel
	help          HelpModel
	config        *config.Config
	db            *game.Database
	showHelp      bool
	showBatchMenu bool
	batchGames    []*game.Game
	batchCursor   int
	batchMessage  string
	densityMode   DensityMode
	width         int
	height        int
	initCmd       tea.Cmd
}

func NewLayout(db *game.Database, svc *Services) LayoutModel {
	cfg, _ := svc.LoadConfig()
	if cfg == nil {
		cfg = config.Default()
	}

	// Single neon-accent dark theme (per .agentera/DECISIONS.md Decision 1).
	// Legacy config values (`theme: dark` / `theme: light` / `theme: default`)
	// are ignored and cleared so subsequent saves do not re-persist them.
	cfg.Theme = ""
	styles := NewStyles(DefaultTheme, cfg.ShowHints)

	games := db.List()
	sidebar, sidebarCmd := NewSidebar(games, styles, svc)
	content := NewContent(styles, cfg.ConfirmDestructive, svc)
	content.database = db
	pane := newResourcePane(styles, content)
	pane.dllsResource.database = db
	pane.setServices(svc)
	manifest, _ := dll.LoadManifest()
	pane.SetDLLsData(games, manifest)

	settings := NewOptionsModal(styles)
	settings.OpenEmbedded(cfg)
	pane.settings = settings

	state := nav.DefaultState()
	layout := LayoutModel{
		styles:     styles,
		services:   svc,
		header:     NewHeader(styles),
		rail:       NewRail(styles),
		pane:       pane,
		navState:   &state,
		messageBar: NewMessageBar(styles),
		help:       NewHelp(styles),
		config:     cfg,
		db:         db,
		initCmd:    sidebarCmd,
	}
	layout.pane.BindNavState(layout.navState)
	layout.contextNav = NewContextNav(styles, sidebar, layout.navState)
	layout.pane.loadGlobalScope()
	return layout
}

func (m LayoutModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.header.Init(), m.initCmd}
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

	// Handle window resize and key overlays/globals.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.calculateDimensions()
		return m, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		if m.showBatchMenu {
			var cmd tea.Cmd
			m, cmd, _ = m.handleBatchMenuKeys(msg)
			return m, cmd
		}
		if m.showHelp {
			var cmd tea.Cmd
			m, cmd, _ = m.handleHelpKeys(msg)
			return m, cmd
		}
		if updated, cmd, handled := m.handleGlobalKeys(msg); handled {
			return updated, cmd
		}
	}

	// Route application messages that affect multiple components.
	m, cmds = m.handleAppMessages(msg, cmds)
	if _, handled := msg.(profileSaveMsg); handled {
		return m, tea.Batch(cmds...)
	}

	// Route keys to the focused zone; always route non-key msgs to content.
	if key, ok := msg.(tea.KeyPressMsg); ok {
		var cmd tea.Cmd
		var handled bool
		switch m.navState.Zone {
		case nav.ZonePrimary:
			m.rail, cmd, handled = m.rail.Update(key)
			if handled {
				m.syncNavFromRail()
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		case nav.ZoneContext:
			m.contextNav, cmd, handled = m.contextNav.Update(key)
			m.pane.SetState(*m.navState)
			if handled {
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		case nav.ZoneContent:
			m.pane, cmd = m.pane.Update(key)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
	} else {
		var cmd tea.Cmd
		m.pane, cmd = m.pane.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *LayoutModel) syncNavFromRail() {
	*m.navState = m.navState.SelectDestination(m.rail.Active())
	m.syncNavToComponents()
	if m.navState.Destination == nav.DestinationSettings {
		m.pane.settings.OpenEmbedded(m.config)
	}
}

func (m *LayoutModel) syncNavToComponents() {
	m.rail.SyncFromState(*m.navState)
	m.contextNav.SetState(*m.navState)
	m.contextNav.syncCursorFromState()
	m.pane.SetState(*m.navState)
}

func (m *LayoutModel) calculateDimensions() {
	headerH := headerHeight
	switch m.densityMode {
	case DensityCompact:
		headerH = compactHeaderHeight
	case DensityFocused:
		headerH = 0
	}

	panelHeight := max(m.height-statusBarHeight-messageBarHeight-headerH-2, 5)

	m.header.SetWidth(m.width)
	m.rail.SetSize(primaryNavWidth-4, panelHeight)
	m.contextNav.SetSize(contextNavWidth, panelHeight)
	m.pane.SetSize(m.contentWidth(), panelHeight)
	m.messageBar.SetWidth(m.width)
}

func (m LayoutModel) contentWidth() int {
	if m.densityMode == DensityFocused {
		return m.width - 4
	}
	return m.width - primaryNavWidth - contextNavWidth - 4
}

// contentModel returns the ContentModel inside ResourceGames for layout-
// level logic (e.g. HasModalOpen checks). Returns nil when not applicable.
func (m LayoutModel) renderStatusBar(text string) string {
	return lipgloss.NewStyle().Foreground(m.styles.Theme.TextDim).
		Width(m.width).Padding(0, 1).Render(text)
}

func (m LayoutModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	mainContent := m.renderMain()
	mainLayer := lipgloss.NewLayer(mainContent)

	compositor := lipgloss.NewCompositor(mainLayer)

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
	header := m.header.View()

	panelHeight := max(m.height-statusBarHeight-messageBarHeight-headerHeight-2, 5)

	primaryFocused := m.navState.Zone == nav.ZonePrimary
	contextFocused := m.navState.Zone == nav.ZoneContext
	contentFocused := m.navState.Zone == nav.ZoneContent

	primaryView := truncateHeight(m.rail.View(primaryFocused), panelHeight)
	contextView := truncateHeight(m.contextNav.View(contextFocused), panelHeight)
	contentView := truncateHeight(m.pane.View(contentFocused), panelHeight)

	primaryBorder := m.styles.BorderColor(primaryFocused)
	contextBorder := m.styles.BorderColor(contextFocused)
	contentBorder := m.styles.BorderColor(contentFocused)

	primaryStyle := lipgloss.NewStyle().Width(primaryNavWidth - 2).Height(panelHeight).
		BorderStyle(lipgloss.RoundedBorder()).BorderTop(false).BorderForeground(primaryBorder)
	contextStyle := lipgloss.NewStyle().Width(contextNavWidth - 2).Height(panelHeight).
		BorderStyle(lipgloss.RoundedBorder()).BorderTop(false).BorderForeground(contextBorder)
	contentStyle := lipgloss.NewStyle().Width(m.contentWidth() - 2).Height(panelHeight).
		BorderStyle(lipgloss.RoundedBorder()).BorderTop(false).BorderForeground(contentBorder)

	primaryBox := primaryStyle.Render(primaryView)
	contextBox := contextStyle.Render(contextView)
	contentBox := contentStyle.Render(contentView)

	primaryBox = buildTopBorder(zoneColumnTitle("Navigate", primaryFocused), primaryNavWidth-2, primaryBorder) + "\n" + primaryBox
	contextBox = buildTopBorder(zoneColumnTitle("Context", contextFocused), contextNavWidth-2, contextBorder) + "\n" + contextBox
	contentBox = buildTopBorder(zoneColumnTitle("Content", contentFocused), m.contentWidth()-2, contentBorder) + "\n" + contentBox

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, primaryBox, contextBox, contentBox)

	messageBar := m.messageBar.View()

	hints := nav.ContentHints{}
	if cm := m.pane.contentModel(); cm != nil {
		hints = nav.ContentHints{
			HasUpdates:   cm.hasUpdates,
			HasBackup:    cm.hasBackup,
			DLLOperating: cm.dllOperating,
		}
	}
	contextHelp := RenderNavContextBar(m.navState.ContextKeys(m.styles.ShowHints, hints), m.width/2, &m.styles.Theme)
	crumbs := m.renderBreadcrumbs()
	statusBar := m.renderStatusBar(crumbs + "  " + m.renderZoneIndicator() + "  " + contextHelp)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, messageBar, statusBar)
}

func (m LayoutModel) renderCompact() string {
	return m.renderStandard()
}

func (m LayoutModel) renderFocused() string {
	contentHeight := max(m.height-statusBarHeight-messageBarHeight-2, 5)
	paneView := truncateHeight(m.pane.View(true), contentHeight)

	paneBorderColor := m.styles.BorderColor(true)

	paneStyle := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(contentHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(paneBorderColor)

	paneBox := paneStyle.Render(paneView)

	messageBar := m.messageBar.View()

	escHint := lipgloss.NewStyle().Foreground(m.styles.Theme.TextDim).Render("F11:exit focused  ?:help  q:quit")
	statusBar := m.renderStatusBar(escHint)

	return lipgloss.JoinVertical(lipgloss.Left, paneBox, messageBar, statusBar)
}

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

func (m LayoutModel) renderZoneIndicator() string {
	label := zoneLabel(m.navState.Zone)
	style := lipgloss.NewStyle().Foreground(m.styles.Theme.AccentOverride).Bold(true)
	return style.Render("[" + label + "]")
}

func zoneLabel(z nav.Zone) string {
	switch z {
	case nav.ZonePrimary:
		return "Primary"
	case nav.ZoneContext:
		return "Context"
	case nav.ZoneContent:
		return "Content"
	default:
		return "Primary"
	}
}

func zoneColumnTitle(name string, focused bool) string {
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

// positionModalLayer creates a centered Layer for a modal string.
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
	cfg := m.config
	scanGames := m.services.ScanGames
	return func() tea.Msg {
		db, err := scanGames(cfg)
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
	failed  bool
	batch   dll.BatchResult
}

var batchActions = []string{
	"Update all DLLs",
}

func (m LayoutModel) executeBatchAction() tea.Cmd {
	appIDs := make([]uint64, len(m.batchGames))
	for index, entry := range m.batchGames {
		appIDs[index] = entry.AppID
	}
	return func() tea.Msg {
		return executeBatchDLLUpdate(appIDs)
	}
}

func executeBatchDLLUpdate(appIDs []uint64) batchCompleteMsg {
	batch := dll.UpdateGames(appIDs, "", nil)
	updatedGames := make(map[uint64]bool)
	for _, item := range batch.Items {
		if item.Err == nil && item.Result.Outcome == dll.OutcomeChanged && item.Result.Game != nil {
			updatedGames[item.Result.Game.AppID] = true
		}
	}
	message := fmt.Sprintf("Updated DLLs for %d/%d games", len(updatedGames), len(appIDs))
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
