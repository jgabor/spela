package tui

import (
	"fmt"
	"strings"
)

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

// renderDLLs is read-only. Applicable operations are exposed by the shell's
// contextual Actions menu rather than focus-blind edit and save shortcuts.
func (m ContentModel) renderDLLs() string {
	if m.confirmation != nil {
		return m.confirmation.viewSized(m.styles, m.width, m.height)
	}
	if m.dllResultOpen {
		return dllResultView(m.styles, "DLL result", m.lastDLLResult, m.dllInstallControl == 0, m.scrollOffset, m.width, m.height)
	}
	if m.dllOperating {
		return m.styles.Title.Render("DLL operation") + "\n\n" + m.styles.Warning.Render(m.dllOperatingLabel) + "\n\nWork is running. Controls return when it finishes."
	}
	var builder strings.Builder
	builder.WriteString(m.styles.Title.Render("DLL versions"))
	builder.WriteByte('\n')
	if len(m.game.DLLs) == 0 {
		builder.WriteString(m.styles.Dim.Render("No managed DLL installed"))
		builder.WriteString("\nSupported families: ")
		families := make([]string, 0, len(dllDisplayColumns))
		for _, info := range dllDisplayColumns {
			families = append(families, info.Label)
		}
		builder.WriteString(strings.Join(families, ", "))
	} else {
		for _, installed := range m.game.DLLs {
			version := installed.Version
			if version == "" {
				version = "unknown"
			}
			fmt.Fprintf(&builder, "%s: %s\n", dllFamilyName(string(installed.Type)), version)
		}
		if m.hasBackup {
			builder.WriteString(m.styles.Dim.Render("Backup available") + "\n")
		}
	}
	if m.lastDLLResult != "" {
		builder.WriteString("\n" + m.styles.Dim.Render(m.lastDLLResult))
	}
	return builder.String()
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
