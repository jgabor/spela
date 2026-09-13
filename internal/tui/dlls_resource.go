package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
)

// DLLsResourceModel renders cached DLL inventory and installed deployment.
// The explicit update action updates stale deployment cells in one batch.
type DLLsResourceModel struct {
	styles     *Styles
	services   *Services
	database   *game.Database
	games      []*game.Game
	manifest   *dll.Manifest
	cached     map[string][]string
	typesInUse []dll.KnownDLLTypeInfo

	typeCursor       int
	gameRowCursor    int
	deploymentGames  []*game.Game
	lastBatchResult  map[string]string
	lastBatchSummary string
	busy             bool
	confirmation     *dllMutationConfirmation
	resultOpen       bool
	resultBodyFocus  bool
	scrollOffset     int
	width            int
	height           int
}

// UpdateList accepts only the basic keys displayed for a focused list.
func (m DLLsResourceModel) UpdateList(key tea.KeyPressMsg, section nav.DLLCatalogSection) DLLsResourceModel {
	action := ActionNoOp
	if key.String() == "up" {
		action = ActionListPrevious
	} else if key.String() == "down" {
		action = ActionListNext
	}
	return m.UpdateListAction(action, section)
}

func (m DLLsResourceModel) UpdateListAction(action KeyAction, section nav.DLLCatalogSection) DLLsResourceModel {
	if m.HasModalOpen() {
		return m
	}
	delta := 0
	switch action {
	case ActionListNext:
		delta = 1
	case ActionListPrevious:
		delta = -1
	default:
		return m
	}
	m.scrollOffset = 0
	if section == nav.SectionDLLDeployment {
		if len(m.deploymentGames) > 0 {
			m.gameRowCursor = min(max(m.gameRowCursor+delta, 0), len(m.deploymentGames)-1)
		}
		return m
	}
	types := m.knownDLLTypes()
	if len(types) > 0 {
		m.typeCursor = min(max(m.typeCursor+delta, 0), len(types)-1)
	}
	return m
}

func (m DLLsResourceModel) ListView(focused bool, section nav.DLLCatalogSection) string {
	var builder strings.Builder
	sectionLabel := nav.DLLCatalogSectionLabels[int(section)]
	builder.WriteString(m.styles.Title.Render(sectionLabel))
	builder.WriteString("\n")
	if focused {
		builder.WriteString(m.styles.Dim.Render("←/→ Group"))
	}
	builder.WriteString("\n\n")
	if section == nav.SectionDLLDeployment {
		if len(m.deploymentGames) == 0 {
			return builder.String() + m.styles.Dim.Render("No deployed DLLs")
		}
		start, end := visibleRange(m.gameRowCursor, len(m.deploymentGames), max(m.height-5, 1))
		for index := start; index < end; index++ {
			entry := m.deploymentGames[index]
			builder.WriteString(m.listRow(m.deploymentRowLabel(entry), index == m.gameRowCursor, focused))
		}
		if len(m.deploymentGames) > end-start {
			builder.WriteString(m.styles.Dim.Render(fmt.Sprintf(" %d/%d", m.gameRowCursor+1, len(m.deploymentGames))))
		}
		return builder.String()
	}
	types := m.knownDLLTypes()
	if len(types) == 0 {
		return builder.String() + m.styles.Dim.Render("DLL catalog unavailable")
	}
	start, end := visibleRange(m.typeCursor, len(types), max(m.height-5, 1))
	for index := start; index < end; index++ {
		info := types[index]
		builder.WriteString(m.listRow(info.Label, index == m.typeCursor, focused))
	}
	return builder.String()
}

func (m DLLsResourceModel) listRow(label string, selected, focused bool) string {
	style := m.styles.Dim
	prefix := "  "
	if selected {
		prefix = "> "
		if focused {
			style = m.styles.FocusStyle()
		} else {
			style = m.styles.Selected
		}
	}
	return style.Render(prefix+label) + "\n"
}

func (m DLLsResourceModel) deploymentRowLabel(entry *game.Game) string {
	status := "current"
	for _, installed := range entry.DLLs {
		for _, info := range m.typesInUse {
			if installed.Type == info.Type && m.isStale(installed.Version, info.ManifestKey) {
				status = "stale"
				break
			}
		}
	}
	// The catalog list pane is 20 cells wide at the supported 80-column floor.
	// Keep the status on the same navigable row instead of relying on wrapping.
	available := 20
	suffix := " · " + status
	return truncate(entry.Name, max(available-len([]rune(suffix)), 1)) + suffix
}

type dllsUpdateAllCompleteMsg struct {
	results map[string]string
	summary string
	games   map[uint64]*game.Game
}

// NewDLLsResource constructs an empty DLLs resource. Use SetGames and
// RefreshCached before rendering.
func NewDLLsResource(styles *Styles, services *Services) DLLsResourceModel {
	return DLLsResourceModel{
		styles:   styles,
		services: services,
		cached:   make(map[string][]string),
	}
}

func (m DLLsResourceModel) knownDLLTypes() []dll.KnownDLLTypeInfo {
	if m.services != nil && m.services.KnownDLLTypes != nil {
		return m.services.KnownDLLTypes()
	}
	return dll.KnownDLLTypes()
}

func (m DLLsResourceModel) listCachedDLLs(manifestKey string) ([]string, error) {
	if m.services != nil && m.services.ListCachedDLLs != nil {
		return m.services.ListCachedDLLs(manifestKey)
	}
	return dll.ListCachedVersions(manifestKey)
}

func (m DLLsResourceModel) batchUpdateDLLs(requests []dll.UpdateRequest) dll.BatchResult {
	if m.services != nil && m.services.BatchUpdateDLLs != nil {
		return m.services.BatchUpdateDLLs(requests)
	}
	return dll.BatchUpdate(requests, nil)
}

// SetGames replaces the tracked game list and recomputes which DLL types
// have at least one install. Called on construction and after a rescan.
func (m DLLsResourceModel) SetGames(games []*game.Game) DLLsResourceModel {
	m.games = games
	m.typesInUse = m.computeTypesInUse(games)
	m.deploymentGames = gamesWithAnyDLL(games, m.typesInUse)
	if m.gameRowCursor >= len(m.deploymentGames) {
		m.gameRowCursor = max(len(m.deploymentGames)-1, 0)
	}
	return m
}

// SetManifest attaches a manifest consulted for latest-version cells. Safe
// to call with nil.
func (m DLLsResourceModel) SetManifest(man *dll.Manifest) DLLsResourceModel {
	m.manifest = man
	return m
}

// RefreshCached re-reads the per-type cache directory for every known DLL
// type and stores the sorted-descending version list under its manifest
// key. Called on construction and after an update-all batch completes (new
// versions may have just been downloaded).
func (m DLLsResourceModel) RefreshCached() DLLsResourceModel {
	types := m.knownDLLTypes()
	cached := make(map[string][]string, len(types))
	for _, info := range types {
		versions, _ := m.listCachedDLLs(info.ManifestKey)
		sort.Slice(versions, func(i, j int) bool {
			return dll.CompareVersions(versions[i], versions[j]) > 0
		})
		cached[info.ManifestKey] = versions
	}
	m.cached = cached
	return m
}

// SetSize stores the allotted width for rendering.
func (m *DLLsResourceModel) SetSize(width, height int) {
	m.width, m.height = width, height
}

func visibleRange(cursor, count, capacity int) (int, int) {
	capacity = max(capacity, 1)
	start := max(cursor-capacity+1, 0)
	start = min(start, max(count-capacity, 0))
	return start, min(start+capacity, count)
}

// computeTypesInUse returns the ordered list of DLL types that at least one
// tracked game installs, preserving the KnownDLLTypes order.
func (m DLLsResourceModel) computeTypesInUse(games []*game.Game) []dll.KnownDLLTypeInfo {
	installed := make(map[game.DLLType]bool)
	for _, g := range games {
		for _, d := range g.DLLs {
			installed[d.Type] = true
		}
	}
	var out []dll.KnownDLLTypeInfo
	for _, info := range m.knownDLLTypes() {
		if installed[info.Type] {
			out = append(out, info)
		}
	}
	return out
}

// gamesWithAnyDLL returns the subset of games that install at least one DLL
// matching typesInUse. Preserves input order.
func gamesWithAnyDLL(games []*game.Game, typesInUse []dll.KnownDLLTypeInfo) []*game.Game {
	if len(typesInUse) == 0 {
		return nil
	}
	known := make(map[game.DLLType]bool, len(typesInUse))
	for _, t := range typesInUse {
		known[t.Type] = true
	}
	var out []*game.Game
	for _, g := range games {
		for _, d := range g.DLLs {
			if known[d.Type] {
				out = append(out, g)
				break
			}
		}
	}
	return out
}

// latestCached returns the newest cached version for the given manifest key,
// or "" if nothing has been downloaded yet.
func (m DLLsResourceModel) latestCached(manifestKey string) string {
	versions := m.cached[manifestKey]
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}

// isStale reports whether the installed version is strictly older than the
// newest cached version. Returns false when either side is missing or when
// versions are equal. Uses dll.IsNewer for ordering.
func (m DLLsResourceModel) isStale(installed, manifestKey string) bool {
	if installed == "" {
		return false
	}
	latest := m.latestCached(manifestKey)
	if latest == "" {
		return false
	}
	return dll.IsNewer(installed, latest)
}

// HasModalOpen separates an active workflow from ordinary read-only detail.
func (m DLLsResourceModel) HasModalOpen() bool {
	return m.confirmation != nil || m.busy || m.resultOpen
}

func (m DLLsResourceModel) DLLOverlayHint() string {
	if m.confirmation != nil {
		return m.confirmation.hint()
	}
	if m.busy {
		return "Work is running"
	}
	if m.resultOpen {
		return dllResultHint(m.batchResultMessage(), m.resultBodyFocus, m.width, m.height)
	}
	return ""
}

func (m DLLsResourceModel) DLLActionAvailability(action KeyAction, section nav.DLLCatalogSection) (bool, string) {
	if action != ActionDetailUpdate {
		return false, "not a catalog action"
	}
	if section != nav.SectionDLLDeployment {
		return false, "open Deployment"
	}
	if m.HasModalOpen() {
		return false, "finish the current DLL operation"
	}
	if !m.hasStaleCells() {
		return false, "no stale cached deployments"
	}
	return true, ""
}

func (m DLLsResourceModel) UpdateTargetCount() int {
	return len(m.staleDLLMutationTargets())
}

func (m DLLsResourceModel) DLLActionLabel(action KeyAction) string {
	if action == ActionDetailUpdate {
		unit := "DLLs"
		if m.UpdateTargetCount() == 1 {
			unit = "DLL"
		}
		return fmt.Sprintf("Update all stale deployments (%d %s)", m.UpdateTargetCount(), unit)
	}
	return ""
}

// UpdateAction is usable from either pane. The owning destination supplies the
// section availability gate; confirmations and progress capture all input.
func (m DLLsResourceModel) UpdateAction(action KeyAction) (DLLsResourceModel, tea.Cmd) {
	if m.confirmation != nil {
		confirm, cancel := m.confirmation.updateAction(action)
		if cancel {
			m.confirmation = nil
			m.lastBatchResult = nil
			m.lastBatchSummary = dllCancellationResult("DLL update-all")
			m.resultOpen, m.resultBodyFocus, m.scrollOffset = true, true, 0
			return m, nil
		}
		if confirm && !m.busy {
			targets := append([]dllMutationTarget(nil), m.confirmation.targets...)
			m.confirmation = nil
			if len(targets) == 0 {
				m.lastBatchSummary = "DLL update-all unavailable: no current target"
				m.resultOpen, m.resultBodyFocus = true, true
				return m, nil
			}
			m.busy = true
			m.lastBatchSummary = "Updating DLLs..."
			return m, m.updateAllCmd(targets)
		}
		return m, nil
	}
	if m.busy {
		return m, nil
	}
	if m.resultOpen {
		switch action {
		case ActionOverlayFocusNext:
			m.resultBodyFocus = !m.resultBodyFocus
		case ActionOverlayPrevious:
			if m.resultBodyFocus {
				m.scrollOffset = max(m.scrollOffset-1, 0)
			}
		case ActionOverlayNext:
			if m.resultBodyFocus {
				m.scrollOffset = min(m.scrollOffset+1, dllResultMaximumScroll(m.batchResultMessage(), m.width, m.height))
			}
		case ActionOverlayClose:
			m.resultOpen = false
		case ActionOverlayConfirm:
			if !m.resultBodyFocus {
				m.resultOpen = false
			}
		}
		return m, nil
	}
	if action == ActionDetailUpdate && m.hasStaleCells() {
		m.scrollOffset = 0
		m.confirmation = newDLLMutationConfirmation("Confirm all stale DLL updates", m.staleDLLMutationTargets(), "each current DLL is backed up before replacement")
	}
	return m, nil
}

// Update consumes completions and basic overlay controls. Browse commands enter
// through UpdateAction rather than through old letter or modifier shortcuts.
func (m DLLsResourceModel) Update(msg tea.Msg) (DLLsResourceModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.HasModalOpen() {
			return m.UpdateAction(dllOverlayAction(msg))
		}
	case dllsUpdateAllCompleteMsg:
		m.busy = false
		m.resultOpen, m.resultBodyFocus, m.scrollOffset = true, true, 0
		for appID, updated := range msg.games {
			if m.database != nil {
				m.database.Games[appID] = updated
			}
			for index, entry := range m.games {
				if entry.AppID == appID {
					m.games[index] = updated
				}
			}
		}
		m = m.SetGames(m.games)
		m.lastBatchResult = msg.results
		m.lastBatchSummary = msg.summary
		m = m.RefreshCached()
	}
	return m, nil
}

// hasStaleCells reports whether the current deployment table contains any
// stale cell. Used to short-circuit update-all when nothing is actionable.
func (m DLLsResourceModel) hasStaleCells() bool {
	for _, g := range m.deploymentGames {
		for _, info := range m.typesInUse {
			for _, detected := range g.DLLs {
				if detected.Type == info.Type && m.isStale(detected.Version, info.ManifestKey) {
					return true
				}
			}
		}
	}
	return false
}

func (m DLLsResourceModel) staleDLLMutationTargets() []dllMutationTarget {
	var targets []dllMutationTarget
	for _, entry := range m.deploymentGames {
		for _, installed := range entry.DLLs {
			for _, info := range m.typesInUse {
				if installed.Type == info.Type && m.isStale(installed.Version, info.ManifestKey) {
					targets = append(targets, newDLLMutationTarget(entry, installed, m.latestCached(info.ManifestKey)))
				}
			}
		}
	}
	return targets
}

// installedVersionFor returns the version string of the installed DLL of
// the given type for this game, or "" when not installed.
func installedVersionFor(g *game.Game, t game.DLLType) string {
	for _, d := range g.DLLs {
		if d.Type == t {
			return d.Version
		}
	}
	return ""
}

func (m DLLsResourceModel) updateAllCmd(targets []dllMutationTarget) tea.Cmd {
	requests := dllUpdateRequests(targets, true)
	var keys []string
	for _, target := range targets {
		keys = append(keys, fmt.Sprintf("%d:%s", target.appID, target.manifestKey))
	}

	return func() tea.Msg {
		batch := m.batchUpdateDLLs(requests)
		results := make(map[string]string, len(requests))
		games := make(map[uint64]*game.Game)
		for index, item := range batch.Items {
			if item.Result.Game != nil {
				games[item.Result.Game.AppID] = item.Result.Game
			}
			if index >= len(keys) {
				continue
			}
			if item.Err != nil {
				results[keys[index]] = fmt.Sprintf("err: %v", item.Err)
			} else if item.Result.Outcome != dll.OutcomeNoOp {
				results[keys[index]] = "ok"
			}
		}
		summary := fmt.Sprintf("Update-all: %d updated, %d current, %d failed", batch.Updated, batch.Unchanged, batch.Failed)
		if batch.Updated == 0 && batch.Failed == 0 {
			summary = "Update-all: already current"
		}
		return dllsUpdateAllCompleteMsg{results: results, summary: summary, games: games}
	}
}

func (m DLLsResourceModel) View(_ bool, section nav.DLLCatalogSection) string {
	if m.confirmation != nil {
		return m.confirmation.viewSized(m.styles, m.width, m.height)
	}
	if m.busy {
		return m.styles.Title.Render("DLL update-all") + "\n\n" + m.styles.Warning.Render("Updating DLLs...") + "\n\nWork is running. Controls return when it finishes."
	}
	if m.resultOpen {
		return dllResultView(m.styles, "DLL update-all result", m.batchResultMessage(), m.resultBodyFocus, m.scrollOffset, m.width, m.height)
	}
	lines := m.detailLines(section)
	start := min(max(m.scrollOffset, 0), max(len(lines)-max(m.height, 1), 0))
	return strings.Join(lines[start:min(start+max(m.height, 1), len(lines))], "\n")
}

func (m DLLsResourceModel) batchResultMessage() string {
	var builder strings.Builder
	builder.WriteString(m.lastBatchSummary)
	keys := make([]string, 0, len(m.lastBatchResult))
	for key, result := range m.lastBatchResult {
		if strings.HasPrefix(result, "err:") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		label := key
		for _, entry := range m.games {
			if strings.HasPrefix(key, fmt.Sprintf("%d:", entry.AppID)) {
				label = entry.Name + " · " + dllFamilyName(strings.SplitN(key, ":", 2)[1])
				break
			}
		}
		builder.WriteString("\n\n" + label + ": " + strings.TrimPrefix(m.lastBatchResult[key], "err: "))
	}
	return builder.String()
}

func (m DLLsResourceModel) detailLines(section nav.DLLCatalogSection) []string {
	body := m.renderSelectedDetail(section)
	if m.lastBatchSummary != "" {
		body += "\n\n" + m.styles.Dim.Render(m.lastBatchSummary)
	}
	return strings.Split(lipgloss.NewStyle().Width(max(m.width, 20)).Render(body), "\n")
}

func (m DLLsResourceModel) DetailScrollable(section nav.DLLCatalogSection) bool {
	return len(m.detailLines(section)) > max(m.height, 1)
}

func (m DLLsResourceModel) ScrollDetail(action KeyAction, section nav.DLLCatalogSection) DLLsResourceModel {
	if m.HasModalOpen() {
		return m
	}
	switch action {
	case ActionDetailPreviousItem:
		m.scrollOffset = max(m.scrollOffset-1, 0)
	case ActionDetailNextItem:
		m.scrollOffset = min(m.scrollOffset+1, max(len(m.detailLines(section))-max(m.height, 1), 0))
	}
	return m
}

func (m DLLsResourceModel) renderSelectedDetail(section nav.DLLCatalogSection) string {
	if section == nav.SectionDLLDeployment {
		if len(m.deploymentGames) == 0 {
			return m.styles.Dim.Render("No deployed DLL selected")
		}
		entry := m.deploymentGames[min(m.gameRowCursor, len(m.deploymentGames)-1)]
		var builder strings.Builder
		builder.WriteString(m.styles.Title.Render("Deployment"))
		builder.WriteString("\n")
		builder.WriteString(m.styles.Selected.Render(truncate(entry.Name, max(m.width-2, 20))))
		builder.WriteString("\n\n")
		for _, installed := range entry.DLLs {
			version := installed.Version
			if version == "" {
				version = "unknown"
			}
			status := "current"
			for _, info := range m.typesInUse {
				if info.Type == installed.Type && m.isStale(installed.Version, info.ManifestKey) {
					status = "stale"
				}
			}
			if installed.Version == "" {
				status = "unknown"
			}
			fmt.Fprintf(&builder, "%s  %s · %s\n", dllFamilyName(string(installed.Type)), version, status)
		}
		if m.hasStaleCells() {
			builder.WriteString("\n")
			builder.WriteString(m.styles.Dim.Render(fmt.Sprintf("%d stale DLL target(s) can be updated", m.UpdateTargetCount())))
		}
		return builder.String()
	}
	types := m.knownDLLTypes()
	if len(types) == 0 {
		return m.styles.Dim.Render("DLL catalog unavailable")
	}
	info := types[min(m.typeCursor, len(types)-1)]
	latest := "unavailable"
	if m.manifest != nil {
		if entry := m.manifest.GetLatestDLL(info.ManifestKey); entry != nil {
			latest = entry.Version
		}
	}
	cached := m.cached[info.ManifestKey]
	cachedStatus := "unavailable"
	if len(cached) > 0 {
		cachedStatus = strings.Join(cached, ", ")
	}
	return fmt.Sprintf("%s\n%s\n\nFile: %s\nLatest: %s\nCached: %s", m.styles.Title.Render("Inventory of DLL types"), m.styles.Selected.Render(info.Label), info.Filename, latest, cachedStatus)
}

func (m DLLsResourceModel) renderLibrary() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.Title.Render("Library"))
	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Inventory of DLL types Spela can manage."))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Foreground(s.Theme.Fg).Bold(true).Width(9)
	keyStyle := s.Dim
	valueStyle := lipgloss.NewStyle().Foreground(s.Theme.Fg)
	// Header row
	header := labelStyle.Render("Type") +
		keyStyle.Render("key        ") +
		keyStyle.Render("latest         ") +
		keyStyle.Render("cached (newest)") +
		keyStyle.Render("  count")
	b.WriteString(header)
	b.WriteString("\n")

	for _, info := range m.knownDLLTypes() {
		latestStr := "unavailable"
		if m.manifest != nil {
			if latest := m.manifest.GetLatestDLL(info.ManifestKey); latest != nil {
				latestStr = latest.Version
			}
		}
		newestCached := m.latestCached(info.ManifestKey)
		if newestCached == "" {
			newestCached = "unavailable"
		}
		count := len(m.cached[info.ManifestKey])

		line := labelStyle.Render(info.Label) +
			keyStyle.Render(padRight(info.ManifestKey, 11)) +
			valueStyle.Render(padRight(latestStr, 15)) +
			valueStyle.Render(padRight(newestCached, 15)) +
			valueStyle.Render(fmt.Sprintf("  %d", count))
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// renderDeployment renders the games × DLL-types matrix. Rows are only the
// games that install at least one known DLL; columns are only DLL types that
// at least one tracked game installs. Stale cells carry a magenta marker
// using the accent-override (override) token. Empty cells render "-".
func (m DLLsResourceModel) renderDeployment() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.Title.Render("Deployment"))
	b.WriteString("\n")

	if len(m.typesInUse) == 0 || len(m.deploymentGames) == 0 {
		b.WriteString(s.Dim.Render("No tracked game installs a managed DLL yet."))
		return b.String()
	}
	b.WriteString(s.Dim.Render("Installed version of each DLL type per game. Magenta marker = stale."))
	b.WriteString("\n\n")

	// Column widths — cap labels tightly on narrow terminals. The row label
	// (game name) is cushioned; DLL-type headers contract before the row.
	nameCol := m.gameColumnWidth()
	typeCol := m.typeColumnWidth()

	// Header row: game name column + one cell per DLL type.
	nameHeaderStyle := lipgloss.NewStyle().
		Foreground(s.Theme.FgMuted).
		Bold(true).
		Width(nameCol)
	typeHeaderStyle := lipgloss.NewStyle().
		Foreground(s.Theme.FgMuted).
		Bold(true).
		Width(typeCol).
		Align(lipgloss.Center)

	b.WriteString(nameHeaderStyle.Render("Game"))
	for _, info := range m.typesInUse {
		b.WriteString(typeHeaderStyle.Render(shortenLabel(info.Label, typeCol-1)))
	}
	b.WriteString("\n")

	// Body rows
	for idx, g := range m.deploymentGames {
		focused := idx == m.gameRowCursor
		nameStyle := lipgloss.NewStyle().Width(nameCol)
		if focused {
			nameStyle = nameStyle.Foreground(s.Theme.AccentFocus).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(s.Theme.Fg)
		}
		b.WriteString(nameStyle.Render(truncate(g.Name, nameCol-1)))

		for _, info := range m.typesInUse {
			b.WriteString(m.renderCell(g, info, typeCol))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderCell renders one deployment cell. Cells carry the installed version
// when present, or "-" when absent. Stale cells prepend a magenta "◆" using
// the accent-override token; freshly-updated cells from the last batch
// prepend a "✓" in the Success color; failed cells render in Error.
func (m DLLsResourceModel) renderCell(g *game.Game, info dll.KnownDLLTypeInfo, width int) string {
	s := m.styles
	installed := installedVersionFor(g, info.Type)
	cellStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
	key := fmt.Sprintf("%d:%s", g.AppID, info.ManifestKey)
	batchStatus := m.lastBatchResult[key]

	if installed == "" {
		return cellStyle.Foreground(s.Theme.FgMuted).Render("-")
	}

	label := installed
	prefix := ""
	switch {
	case strings.HasPrefix(batchStatus, "err:"):
		prefix = s.OverrideMarkerStyle().Render("✗ ")
		label = installed
		cellStyle = cellStyle.Foreground(s.Theme.Error)
	case batchStatus == "ok":
		prefix = s.Success.Render("✓ ")
		cellStyle = cellStyle.Foreground(s.Theme.Fg)
	case m.isStale(installed, info.ManifestKey):
		prefix = s.OverrideMarkerStyle().Render("◆ ")
		cellStyle = cellStyle.Foreground(s.Theme.Fg)
	default:
		cellStyle = cellStyle.Foreground(s.Theme.Fg)
	}
	return cellStyle.Render(prefix + truncate(label, width-3))
}

// renderFooter renders the keybindings and last-batch summary.
func (m DLLsResourceModel) renderFooter() string {
	s := m.styles
	var parts []string
	if s.ShowHints {
		parts = append(parts, s.Dim.Render("Read-only deployment status"))
	}
	if m.busy {
		parts = append(parts, s.Warning.Render("updating..."))
	}
	if !m.busy && m.lastBatchResult != nil {
		status := m.lastBatchSummary
		if status == "" {
			status = "update-all finished"
		}
		if batchHasFailures(m.lastBatchResult) {
			parts = append(parts, s.Warning.Render(status))
		} else {
			parts = append(parts, s.Success.Render(status))
		}
	}
	return strings.Join(parts, "  ")
}

func batchHasFailures(results map[string]string) bool {
	for _, status := range results {
		if strings.HasPrefix(status, "err:") {
			return true
		}
	}
	return false
}

// gameColumnWidth returns the rendered width for the row-label column. The
// column contracts to fit game names while respecting the total pane width.
// Wide terminals get 28; narrow ones 16.
func (m DLLsResourceModel) gameColumnWidth() int {
	if m.width <= 0 {
		return 20
	}
	// Reserve at least 8 chars per type column plus 2 padding for borders.
	reserved := len(m.typesInUse) * 10
	remain := m.width - reserved - 4
	w := min(28, max(remain, 12))
	return w
}

// typeColumnWidth returns the rendered width per DLL-type column.
func (m DLLsResourceModel) typeColumnWidth() int {
	if len(m.typesInUse) == 0 {
		return 8
	}
	if m.width <= 0 {
		return 12
	}
	name := m.gameColumnWidth()
	remain := m.width - name - 4
	per := remain / len(m.typesInUse)
	return max(8, min(14, per))
}

// shortenLabel truncates a label to fit within width columns. If the label
// already fits it is returned unchanged. Otherwise the first width-1 runes
// are kept and suffixed with "…".
func shortenLabel(label string, width int) string {
	if width <= 0 || len(label) <= width {
		return label
	}
	if width == 1 {
		return label[:1]
	}
	return label[:width-1] + "…"
}

// truncate trims s to maxWidth runes, appending "…" when trimming occurs.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return string(runes[:1])
	}
	return string(runes[:maxWidth-1]) + "…"
}

// padRight right-pads s with spaces to width runes.
func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}
