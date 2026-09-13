package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// ViewDLLAspectFocused keeps the local controls visible and advertises only
// input that reaches this pane in its current focus state.
func (m ContentModel) ViewDLLAspectFocused(focused bool) string {
	if m.dllInstallState != DLLInstallNone {
		return m.renderDLLInstallDialog()
	}
	if m.game == nil {
		return m.styles.Dim.Render("Select a game from the scope list")
	}
	return m.renderDLLsFocused(focused)
}

// renderDLLInstallDialog keeps chooser controls visible even with verbose
// hints disabled. The active list and each local button have a distinct focus.
func (m ContentModel) renderDLLInstallDialog() string {
	if m.confirmation != nil {
		return m.confirmation.viewSized(m.styles, m.width, m.height)
	}
	if m.dllOperating || m.dllInstallState == DLLInstallDownloading {
		return m.styles.Title.Render("DLL operation") + "\n\n" + m.styles.Warning.Render(m.dllOperatingLabel) + "\n\nWork is running. Controls return when it finishes."
	}
	width, height := max(m.width, 20), max(m.height, 7)
	var builder strings.Builder
	builder.WriteString(m.styles.Title.Render("Install DLL"))
	builder.WriteByte('\n')
	labels := make([]string, 0)
	cursor := m.dllTypeCursor
	prompt := "Select a supported DLL family"
	if m.dllInstallState == DLLInstallSelectVersion {
		prompt, cursor = "Select "+dllFamilyName(m.selectedDLLType)+" version", m.dllVersionCursor
		for index, version := range m.dllVersions {
			label := version.Version
			if index == 0 {
				label += " (latest)"
			}
			labels = append(labels, label)
		}
	} else {
		for _, family := range m.dllTypes {
			labels = append(labels, dllFamilyName(family))
		}
	}
	builder.WriteString(m.styles.Dim.Render(truncate(prompt, width)) + "\n")
	capacity := max(height-5, 1)
	start, end := visibleRange(cursor, len(labels), capacity)
	if len(labels) == 0 {
		message := "Loading..."
		if m.dllInstallState == DLLInstallSelectVersion && m.dllVersionsLoaded {
			message = "No versions available"
		}
		builder.WriteString(m.styles.Dim.Render(message) + "\n")
		capacity--
	}
	for index := start; index < end; index++ {
		style, prefix := m.styles.Normal, "  "
		if index == cursor {
			style, prefix = m.styles.Selected, "> "
			if m.dllInstallControl == 0 {
				style = m.styles.FocusStyle()
			}
		}
		builder.WriteString(style.Render(truncate(prefix+labels[index], width)) + "\n")
	}
	for padding := end - start; padding < capacity; padding++ {
		builder.WriteByte('\n')
	}
	buttons := []string{"Cancel"}
	if m.dllInstallState == DLLInstallSelectVersion {
		buttons = []string{"Back", "Cancel"}
	}
	for index, label := range buttons {
		style := m.styles.Normal
		if m.dllInstallControl == index+1 {
			style = m.styles.FocusStyle()
		}
		builder.WriteString(style.Render("[ "+label+" ]") + "  ")
	}
	builder.WriteString("\n" + m.styles.Dim.Render(m.DLLOverlayHint()))
	return builder.String()
}

// renderDLLsFocused reserves room for direct actions before rendering versions.
// The action controls remain reachable when the terminal is short or hints are off.
func (m ContentModel) renderDLLsFocused(focused bool) string {
	if m.confirmation != nil {
		return m.confirmation.viewSized(m.styles, m.width, m.height)
	}
	if m.dllResultOpen {
		return dllResultView(m.styles, "DLL result", m.lastDLLResult, m.dllInstallControl == 0, m.scrollOffset, m.width, m.height)
	}
	if m.dllOperating {
		return m.styles.Title.Render("DLL operation") + "\n\n" + m.styles.Warning.Render(m.dllOperatingLabel) + "\n\nWork is running. Controls return when it finishes."
	}
	width, height := max(m.width, 20), max(m.height-3, 1)
	controls := m.renderDLLActions(focused, width)
	capacity := max(height-len(controls)-1, 0)
	var versions []string
	versions = append(versions, m.styles.Title.MarginBottom(0).Render("DLL versions"))
	if len(m.game.DLLs) == 0 {
		versions = append(versions, m.styles.Dim.Render("No managed DLL installed"))
		families := make([]string, 0, len(dllDisplayColumns))
		for _, info := range dllDisplayColumns {
			families = append(families, info.Label)
		}
		versions = append(versions, m.styles.Dim.Render("Supported families: "+strings.Join(families, ", ")))
	} else {
		for _, installed := range m.game.DLLs {
			version := installed.Version
			if version == "" {
				version = "unknown"
			}
			versions = append(versions, fmt.Sprintf("%s: %s", dllFamilyName(string(installed.Type)), version))
		}
		if m.hasBackup {
			versions = append(versions, m.styles.Dim.Render("Backup available"))
		}
	}
	if m.lastDLLResult != "" {
		versions = append(versions, m.styles.Dim.Render(m.lastDLLResult))
	}
	var lines []string
	for _, version := range versions {
		lines = append(lines, strings.Split(lipgloss.NewStyle().Width(width).Render(version), "\n")...)
	}
	if len(lines) > capacity {
		lines = lines[:capacity]
		if capacity > 0 {
			summary := fmt.Sprintf("%d installed DLL files", len(m.game.DLLs))
			if len(m.game.DLLs) == 0 {
				summary = "No managed DLL installed"
			}
			lines[capacity-1] = m.styles.Dim.Render(truncate(summary, width))
		}
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return strings.Join(append(lines, controls...), "\n")
}

func (m ContentModel) renderDLLActions(focused bool, width int) []string {
	lines := []string{m.styles.Title.MarginBottom(0).Render(truncate("DLL actions: "+m.game.Name, width))}
	selected := m.DLLSelectedAction()
	for _, action := range gameDLLActions {
		available, reason := m.DLLActionAvailability(action)
		prefix, style := "  ", m.styles.Normal
		if !available {
			style = m.styles.Dim
		}
		if action == selected && focused {
			prefix, style = "▸ ", m.styles.FocusStyle()
		}
		lines = append(lines, style.Render(truncate(prefix+m.dllActionTitle(action), width)))
		if !available {
			lines = append(lines, m.styles.Dim.Render(truncate("    Unavailable: "+reason, width)))
		}
	}
	explanation := "Choose a family and version, then review the target."
	switch selected {
	case ActionDetailUpdate:
		explanation = "Review each detected stale DLL before updating."
	case ActionDetailRestore:
		explanation = "Restore backed-up files. Newly added DLLs stay installed."
	}
	lines = append(lines, m.styles.Dim.Render(truncate(explanation, width)))
	hint := "Tab: Focus DLL controls"
	if focused {
		hint = "↑/↓ Choose action"
		if available, _ := m.DLLActionAvailability(selected); available {
			hint += " · Enter " + m.DLLSelectedActionName()
		}
	}
	lines = append(lines, m.styles.Dim.Render(truncate(hint, width)))
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

// renderProfile renders the profile detail section.
func (m ContentModel) renderProfile() string {
	var b strings.Builder
	if m.usingDefaultProfile {
		b.WriteString(m.styles.Dim.Render("Destination: this game · values inherited from Defaults"))
		b.WriteString("\n")
	}
	profileHeight := m.profileSectionHeight()
	if m.usingDefaultProfile {
		profileHeight--
	}
	m.detail.SetSize(m.width, profileHeight)
	b.WriteString(m.detail.View())
	return b.String()
}
