package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jgabor/spela/internal/nav"
)

func (m LayoutModel) singlePane() bool {
	return m.width < 100 || m.height < 30 || m.densityMode == DensityFocused
}

func (m LayoutModel) headerBlock() string {
	if m.densityMode == DensityFocused {
		return ""
	}
	if m.width < 100 || m.height < 30 || m.densityMode == DensityCompact {
		return strings.TrimRight(m.header.ViewCompact(), "\n")
	}
	return strings.TrimRight(m.header.View(), "\n")
}

func (m LayoutModel) workspaceHeight() int {
	height := 0
	if header := m.headerBlock(); header != "" {
		height = lipgloss.Height(header)
	}
	return max(m.height-height-1-statusBarHeight-messageBarHeight, 3)
}

// boundedBox reserves both borders before sizing its content and pads every
// row to exactly the same terminal-cell width. No child can expand the frame.
func boundedBox(title, body string, width, height int, border color.Color) string {
	inside := max(width-2, 1)
	borderStyle := lipgloss.NewStyle().Foreground(border)
	label := ansi.Truncate(" "+title+" ", inside, "")
	var lines []string
	lines = append(lines, borderStyle.Render("╭"+label+strings.Repeat("─", max(inside-ansi.StringWidth(label), 0))+"╮"))
	bodyLines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for index := 0; index < max(height-2, 1); index++ {
		line := ""
		if index < len(bodyLines) {
			line = ansi.Truncate(bodyLines[index], inside, "…")
		}
		line += strings.Repeat(" ", max(inside-ansi.StringWidth(line), 0))
		lines = append(lines, borderStyle.Render("│")+line+borderStyle.Render("│"))
	}
	lines = append(lines, borderStyle.Render("╰"+strings.Repeat("─", inside)+"╯"))
	return strings.Join(lines, "\n")
}

func (m LayoutModel) renderWorkspace() string {
	context := m.bindingContext()
	height := m.workspaceHeight()
	listFocused := m.focus == FocusList && (context.Mode == ModeBrowse || context.Mode == ModeSearch)
	detailFocused := m.focus == FocusDetail
	var workspace string
	if m.singlePane() {
		body, title := m.visibleListView(listFocused), "List"
		if m.focus == FocusDetail {
			body, title = m.pane.View(detailFocused), "Detail"
		}
		workspace = boundedBox("▸ "+title, body, m.width, height, m.styles.BorderColor(true))
	} else {
		list := boundedBox(paneColumnTitle("List", listFocused), m.visibleListView(listFocused), m.listWidth(), height, m.styles.BorderColor(listFocused))
		detail := boundedBox(paneColumnTitle("Detail", detailFocused), m.pane.View(detailFocused), m.detailWidth(), height, m.styles.BorderColor(detailFocused))
		workspace = lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
	}
	var lines []string
	if header := m.headerBlock(); header != "" {
		lines = append(lines, header)
	}
	lines = append(lines, m.destinationBar.ViewWithKeys(context.Mode == ModeBrowse), workspace, ansi.Truncate(m.messageBar.View(), m.width, "…"))
	lines = append(lines, m.footerLines()...)
	return strings.Join(lines, "\n")
}

func (m LayoutModel) renderOverlay(content string) string {
	box := boundedBox("", content, max(m.width-4, 3), max(m.height-3, 3), m.styles.BorderColor(true))
	lines := strings.Split(box, "\n")
	for index := range lines {
		lines[index] = "  " + lines[index] + "  "
	}
	return m.destinationBar.ViewWithKeys(false) + "\n\n" + strings.Join(lines, "\n") + "\n"
}

func (m LayoutModel) footerLines() []string {
	if m.searchSession != nil && m.decision == nil {
		return []string{m.renderSearchControls(), m.styles.Dim.Render("Tab: next control  Enter: " + m.searchEnterLabel())}
	}
	if m.bindingContext().Mode == ModeOverlay {
		hint := ""
		if m.navState.Destination == nav.DestinationLibrary {
			hint = m.pane.content.DLLOverlayHint()
		}
		if m.navState.Destination == nav.DestinationDLLCatalog {
			hint = m.pane.dllsResource.DLLOverlayHint()
		}
		return []string{ansi.Truncate(hint, m.width, "…"), ""}
	}
	keys := m.statusKeys()
	lines := []string{""}
	for _, key := range keys {
		text := key.Key + ": " + key.Action
		index := len(lines) - 1
		separator := ""
		if lines[index] != "" {
			separator = "  "
		}
		if ansi.StringWidth(lines[index]+separator+text) > m.width {
			lines = append(lines, text)
		} else {
			lines[index] += separator + text
		}
	}
	for len(lines) < statusBarHeight {
		lines = append(lines, "")
	}
	for index := range lines {
		lines[index] = m.styles.Dim.Render(lines[index])
	}
	return lines[:statusBarHeight]
}
