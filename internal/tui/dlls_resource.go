package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

// DLLsResourceModel renders the DLLs resource pane introduced by Task 6.
// Two sections are always visible at once:
//
//  1. Library: inventory of every DLL type the detector recognises, paired
//     with the latest manifest version and the number of cached payloads on
//     disk (plus the newest cached version string).
//  2. Deployment: a games × DLL-types matrix. Each cell shows the version of
//     that DLL currently installed in that game, or "-" when no DLL of that
//     type is installed. Cells carrying a version strictly older than the
//     newest cached payload are marked with the accent-override (magenta)
//     token, matching the inheritance-override marker used elsewhere.
//
// Rows are omitted entirely for DLL types that no tracked game installs —
// the matrix never shows empty columns. DLL types with zero detected
// installations across all games are dropped from the column set.
//
// Focus model: j/k (and arrow aliases) move the deployment row cursor
// across games. The rail owns 1-4 and tab; this pane only cares about
// row navigation and the update-all trigger.
//
// Keybindings:
//   - j / down: focus the next game row in the deployment table
//   - k / up:   focus the previous game row
//   - U or ctrl+u: update every stale cell to its latest cached version
//     in a single batched action; per-cell success/failure is reflected in
//     the lastBatchResult map after completion.
type DLLsResourceModel struct {
	styles   *Styles
	services *Services
	// games is the list the pane iterates for the deployment matrix. It is
	// a snapshot taken from the game database at construction / refresh
	// time — the pane does not watch the database for live changes.
	games []*game.Game
	// manifest caches the manifest consulted for "latest" versions. nil
	// when the manifest could not be loaded; the view degrades gracefully
	// by omitting latest-version cells.
	manifest *dll.Manifest
	// cached maps manifest-key → sorted-descending list of cached versions.
	// Populated via RefreshCached on construction. The first element (when
	// present) is the newest cached version.
	cached map[string][]string
	// typesInUse is the ordered list of manifest keys for DLL types that
	// at least one tracked game installs. Used for deployment columns and
	// derived from dll.KnownDLLTypes() filtered by installation.
	typesInUse []dll.KnownDLLTypeInfo
	// gameRowCursor: index into games that carry at least one installed
	// DLL matching typesInUse. Clamped on set-games and on construction.
	gameRowCursor int
	// deploymentGames: subset of games with at least one installed DLL.
	// These are the rows of the deployment matrix.
	deploymentGames []*game.Game
	// lastBatchResult maps "appID:manifestKey" to a short status snippet
	// (empty = not touched, "ok" = updated, "err: <reason>" = failed).
	// Rendered inline after a successful update-all cycle.
	lastBatchResult  map[string]string
	lastBatchSummary string
	// busy is true while an update-all batch is in flight.
	busy  bool
	width int
}

// dllsUpdateAllCompleteMsg is delivered when the batched update-all has
// finished. Results is keyed "appID:manifestKey". Each value is either
// empty (unchanged), "ok" (updated), or "err: <reason>" (failure).
type dllsUpdateAllCompleteMsg struct {
	results map[string]string
	summary string
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

func (m DLLsResourceModel) updateCachedDLL(req DLLUpdateRequest) error {
	if m.services != nil && m.services.UpdateCachedDLL != nil {
		return m.services.UpdateCachedDLL(req)
	}
	return defaultUpdateCachedDLL(req)
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
	m.width = width
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

// Update routes a key message. j/k navigate rows; U / ctrl+u triggers a
// batched update-all. Returns (model, cmd) consistent with other resource
// components.
func (m DLLsResourceModel) Update(msg tea.Msg) (DLLsResourceModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if m.gameRowCursor < len(m.deploymentGames)-1 {
				m.gameRowCursor++
			}
			return m, nil
		case "k", "up":
			if m.gameRowCursor > 0 {
				m.gameRowCursor--
			}
			return m, nil
		case "U", "ctrl+u":
			if m.busy {
				return m, nil
			}
			if !m.hasStaleCells() {
				return m, nil
			}
			m.busy = true
			return m, m.updateAllCmd()
		}
	case dllsUpdateAllCompleteMsg:
		m.busy = false
		m.lastBatchResult = msg.results
		m.lastBatchSummary = msg.summary
		// Freshly-downloaded payloads may have been cached; refresh so the
		// library section and stale markers reflect the new state.
		m = m.RefreshCached()
		return m, nil
	}
	return m, nil
}

// hasStaleCells reports whether the current deployment table contains any
// stale cell. Used to short-circuit update-all when nothing is actionable.
func (m DLLsResourceModel) hasStaleCells() bool {
	for _, g := range m.deploymentGames {
		for _, info := range m.typesInUse {
			installed := installedVersionFor(g, info.Type)
			if m.isStale(installed, info.ManifestKey) {
				return true
			}
		}
	}
	return false
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

// updateAllCmd captures a snapshot of the current stale cells and returns a
// tea.Cmd that performs the swap per cell, aggregating per-cell success /
// failure into dllsUpdateAllCompleteMsg.results.
func (m DLLsResourceModel) updateAllCmd() tea.Cmd {
	type staleCell struct {
		g           *game.Game
		typeInfo    dll.KnownDLLTypeInfo
		latest      string
		installedOn string // original filename in the game install dir
	}
	var cells []staleCell
	for _, g := range m.deploymentGames {
		for _, info := range m.typesInUse {
			installed := installedVersionFor(g, info.Type)
			if !m.isStale(installed, info.ManifestKey) {
				continue
			}
			// Find the detected DLL filename so SwapDLL can locate the
			// in-game target path. Fall back to the canonical filename
			// from KnownDLLTypes when the detection omitted it.
			installedName := info.Filename
			for _, d := range g.DLLs {
				if d.Type == info.Type {
					installedName = d.Name
					break
				}
			}
			cells = append(cells, staleCell{
				g:           g,
				typeInfo:    info,
				latest:      m.latestCached(info.ManifestKey),
				installedOn: installedName,
			})
		}
	}

	return func() tea.Msg {
		results := make(map[string]string, len(cells))
		succeeded := 0
		failed := 0
		for _, c := range cells {
			key := fmt.Sprintf("%d:%s", c.g.AppID, c.typeInfo.ManifestKey)
			if err := m.updateCachedDLL(DLLUpdateRequest{
				Game:          c.g,
				TypeInfo:      c.typeInfo,
				LatestVersion: c.latest,
				InstalledName: c.installedOn,
			}); err != nil {
				results[key] = fmt.Sprintf("err: %v", err)
				failed++
				continue
			}
			results[key] = "ok"
			succeeded++
		}
		summary := fmt.Sprintf("Update-all: %d updated, %d failed", succeeded, failed)
		return dllsUpdateAllCompleteMsg{results: results, summary: summary}
	}
}

// View renders the DLLs resource pane. Two sections are always visible:
// library at the top, deployment table below. A short help line beneath
// lists j/k and U.
func (m DLLsResourceModel) View(paneFocused bool) string {
	s := m.styles
	borderColor := s.BorderColor(paneFocused)

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(m.renderLibrary())
	b.WriteString("\n\n")
	b.WriteString(m.renderDeployment())
	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return box.Render(b.String())
}

// renderLibrary renders the library section: one row per DLL type, listing
// the latest manifest version, the newest cached version, and how many
// payloads are cached locally. Types with no manifest data and no cached
// payloads still appear — the library is an inventory, not a deployment
// status view.
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
		latestStr := "-"
		if m.manifest != nil {
			if latest := m.manifest.GetLatestDLL(info.ManifestKey); latest != nil {
				latestStr = latest.Version
			}
		}
		newestCached := m.latestCached(info.ManifestKey)
		if newestCached == "" {
			newestCached = "-"
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
		parts = append(parts, s.Dim.Render("j/k select row  U update-all"))
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
