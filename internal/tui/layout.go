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
	statusBarHeight       = 1
	messageBarHeight      = 1
	headerHeight          = 7 // 6 lines for logo + 1 for bottom border
	compactHeaderHeight   = 3 // 2 metric lines + 1 bottom border
)

// LayoutModel owns the non-focusable destination bar and the two-pane
// List/Detail workspace.
type LayoutModel struct {
	styles         *Styles
	services       *Services
	header         HeaderModel
	destinationBar DestinationBarModel
	listPane       ListPaneModel
	pane           resourcePaneModel
	navState       *nav.State
	focus          KeyFocus
	inputMode      InputMode
	messageBar     MessageBarModel
	help           HelpModel
	config         *config.Config
	db             *game.Database
	showHelp       bool
	showBatchMenu  bool
	batchGames     []*game.Game
	batchCursor    int
	batchMessage   string
	densityMode    DensityMode
	width          int
	height         int
	initCmd        tea.Cmd
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
		initCmd:        sidebarCmd,
	}
	layout.pane.BindNavState(layout.navState)
	layout.listPane = NewListPane(styles, sidebar, layout.navState)
	if selected := sidebar.Selected(); selected != nil {
		layout.pane.loadGameScope(selected)
	} else {
		layout.pane.loadNoScope()
	}
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

	// Resize is the only message that reaches the hidden workspace while the
	// terminal is below the supported minimum.
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = msg.Width, msg.Height
		m.calculateDimensions()
		return m, tea.Batch(cmds...)
	}
	if m.width < minimumTerminalWidth || m.height < minimumTerminalHeight {
		if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "q" || key.String() == "ctrl+c") {
			return m, tea.Quit
		}
		return m, tea.Batch(cmds...)
	}

	// Handle key overlays and globals at supported sizes.
	if msg, ok := msg.(tea.KeyPressMsg); ok {
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

	// Route keys only to the visibly focused pane; non-key messages continue to
	// the Detail host because it owns asynchronous domain operations.
	if key, ok := msg.(tea.KeyPressMsg); ok {
		var cmd tea.Cmd
		if m.focus == FocusList {
			var handled bool
			m, cmd, handled = m.updateVisibleList(key)
			m.pane.SetState(*m.navState)
			if handled {
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		} else {
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

func (m *LayoutModel) selectDestination(destination nav.Destination) {
	m.listPane.sidebar.selectMode = false
	m.listPane.sidebar.selected = make(map[uint64]bool)
	*m.navState = m.navState.SelectDestination(destination)
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
	headerH := headerHeight
	switch m.densityMode {
	case DensityCompact:
		headerH = compactHeaderHeight
	case DensityFocused:
		headerH = 0
	}

	panelHeight := max(m.height-headerH-statusBarHeight-messageBarHeight-2, 5)

	m.header.SetWidth(m.width)
	m.destinationBar.SetWidth(m.width)
	if m.densityMode == DensityFocused {
		m.listPane.SetSize(max(m.width-2, 1), panelHeight)
		m.pane.SetSize(max(m.width-2, 1), panelHeight)
	} else {
		m.listPane.SetSize(m.listWidth(), panelHeight)
		m.pane.SetSize(m.detailWidth(), panelHeight)
	}
	m.messageBar.SetWidth(m.width)
	m.help.SetSize(m.helpWidth(), max(m.height-4, 3))
}

func (m LayoutModel) helpWidth() int {
	modalWidth := min(max(int(float64(m.width)*0.55), minModalWidth), maxModalWidth)
	return max(modalWidth-4, 1)
}

func (m LayoutModel) listWidth() int   { return min(listPaneWidth, max(m.width/3, 24)) }
func (m LayoutModel) detailWidth() int { return max(m.width-m.listWidth(), 1) }

// contentModel returns the ContentModel inside ResourceGames for layout-
// level logic (e.g. HasModalOpen checks). Returns nil when not applicable.
func (m LayoutModel) renderStatusBar(text string) string {
	return lipgloss.NewStyle().Foreground(m.styles.Theme.TextDim).
		Width(max(m.width-2, 1)).Padding(0, 1).MaxHeight(1).Render(text)
}

func (m LayoutModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}
	if m.width < minimumTerminalWidth || m.height < minimumTerminalHeight {
		view := tea.NewView(m.renderResizePrompt())
		view.AltScreen = true
		return view
	}

	mainContent := m.renderMain()
	if !m.showHelp && !m.showBatchMenu {
		view := tea.NewView(mainContent)
		view.AltScreen = true
		return view
	}

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

func (m LayoutModel) renderResizePrompt() string {
	lines := []string{
		"Resize terminal",
		"q quit",
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
	return m.renderSplit(m.header.View(), headerHeight)
}

func (m LayoutModel) renderSplit(header string, renderedHeaderHeight int) string {
	panelHeight := max(m.height-renderedHeaderHeight-statusBarHeight-messageBarHeight-2, 5)
	listFocused := m.focus == FocusList
	detailFocused := m.focus == FocusDetail
	listWidth := m.listWidth()
	detailWidth := m.detailWidth()

	listView := truncateHeight(m.visibleListView(listFocused), panelHeight)
	detailView := truncateHeight(m.pane.View(detailFocused), panelHeight)
	listBorder := m.styles.BorderColor(listFocused)
	detailBorder := m.styles.BorderColor(detailFocused)

	listStyle := lipgloss.NewStyle().Width(listWidth - 2).Height(panelHeight).
		MaxHeight(panelHeight).
		BorderStyle(lipgloss.RoundedBorder()).BorderTop(false).BorderLeft(true).BorderRight(true).BorderBottom(true).BorderForeground(listBorder)
	detailStyle := lipgloss.NewStyle().Width(detailWidth - 2).Height(panelHeight).
		MaxHeight(panelHeight).
		BorderStyle(lipgloss.RoundedBorder()).BorderTop(false).BorderLeft(true).BorderRight(true).BorderBottom(true).BorderForeground(detailBorder)

	listBox := buildTopBorder(paneColumnTitle("List", listFocused), listWidth-2, listBorder) + "\n" + listStyle.Render(listView)
	detailBox := buildTopBorder(paneColumnTitle("Detail", detailFocused), detailWidth-2, detailBorder) + "\n" + detailStyle.Render(detailView)
	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, listBox, detailBox)
	statusHelp := m.renderCanonicalStatus()
	crumbs := m.renderBreadcrumbs()
	focusLabel := "List"
	if m.focus == FocusDetail {
		focusLabel = "Detail"
	}
	statusBar := m.renderStatusBar(crumbs + "  [" + focusLabel + "]  " + statusHelp)

	return lipgloss.JoinVertical(lipgloss.Left, header, m.destinationBar.View(), mainArea, m.messageBar.View(), statusBar)
}

func (m LayoutModel) renderCompact() string {
	return m.renderSplit(m.header.ViewCompact(), compactHeaderHeight)
}

func (m LayoutModel) renderFocused() string {
	contentHeight := max(m.height-statusBarHeight-messageBarHeight-3, 5)
	var paneView string
	if m.focus == FocusList {
		paneView = truncateHeight(m.visibleListView(true), contentHeight)
	} else {
		paneView = truncateHeight(m.pane.View(true), contentHeight)
	}

	paneBorderColor := m.styles.BorderColor(true)

	paneStyle := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(contentHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(paneBorderColor)

	paneBox := paneStyle.Render(paneView)

	messageBar := m.messageBar.View()

	focusLabel := "List"
	if m.focus == FocusDetail {
		focusLabel = "Detail"
	}
	statusBar := m.renderStatusBar(m.renderBreadcrumbs() + "  [" + focusLabel + "]  " + m.renderCanonicalStatus())

	return lipgloss.JoinVertical(lipgloss.Left, m.destinationBar.View(), paneBox, messageBar, statusBar)
}

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

func (m LayoutModel) updateVisibleList(key tea.KeyPressMsg) (LayoutModel, tea.Cmd, bool) {
	switch m.navState.Destination {
	case nav.DestinationDLLCatalog:
		if key.String() == "enter" {
			m.focus = FocusDetail
			return m, nil, true
		}
		if key.String() == "h" || key.String() == "left" || key.String() == "l" || key.String() == "right" {
			if m.navState.DLLCatalogSection == nav.SectionDLLLibrary {
				m.navState.DLLCatalogSection = nav.SectionDLLDeployment
			} else {
				m.navState.DLLCatalogSection = nav.SectionDLLLibrary
			}
			return m, nil, true
		}
		m.pane.dllsResource = m.pane.dllsResource.UpdateList(key, m.navState.DLLCatalogSection)
		return m, nil, true
	case nav.DestinationSettings:
		if key.String() == "enter" {
			m.focus = FocusDetail
			return m, nil, true
		}
		if key.String() == "h" || key.String() == "left" || key.String() == "l" || key.String() == "right" {
			sectionCount := len(m.pane.settings.sections)
			if sectionCount > 0 {
				delta := 1
				if key.String() == "h" || key.String() == "left" {
					delta = -1
				}
				m.pane.settings.sectionCursor = (m.pane.settings.sectionCursor + delta + sectionCount) % sectionCount
				m.pane.settings.optionCursor = 0
				m.navState.SettingsSection = nav.SettingsSection(m.pane.settings.sectionCursor)
			}
			return m, nil, true
		}
		m.pane.settings.UpdateList(key)
		return m, nil, true
	default:
		var command tea.Cmd
		var handled bool
		m.listPane, command, handled = m.listPane.Update(key)
		return m, command, handled
	}
}

func (m LayoutModel) renderCanonicalStatus() string {
	if m.showHelp {
		return ""
	}
	resolutions := CanonicalKeymap.HelpBindings(m.bindingContext())
	contextKeys := make([]ContextKey, 0, len(resolutions))
	global := make([]ContextKey, 0, len(globalKeys))
	for _, resolution := range resolutions {
		var labels []string
		for _, candidate := range resolution.Binding.Keys {
			if !candidate.Printable && candidate.Key != "ctrl+c" {
				labels = append(labels, candidate.Label)
			}
		}
		if len(labels) == 0 {
			continue
		}
		key := ContextKey{Key: strings.Join(labels, "/"), Action: strings.ToLower(resolution.Binding.Description), Enabled: true}
		if resolution.Binding.Scope == ScopeGlobal {
			if resolution.Binding.Action == ActionShowHelp || resolution.Binding.Action == ActionQuit {
				global = append(global, key)
			}
		} else {
			contextKeys = append(contextKeys, key)
		}
	}
	keys := append(contextKeys, global...)
	return RenderContextBar(keys, m.width/2, &m.styles.Theme)
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

type noLibrarySelectionMsg struct{}

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
