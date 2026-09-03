package tui

import (
	"fmt"
	"strings"

	"github.com/jgabor/spela/internal/game"
)

// renderDLLInstallDialog renders the multi-step DLL install wizard.
func (m ContentModel) renderDLLInstallDialog() string {
	if m.confirmation != nil {
		return m.confirmation.view(m.styles)
	}
	s := m.styles
	var b strings.Builder

	b.WriteString(s.Title.Render("Install DLL"))
	b.WriteString("\n\n")

	switch m.dllInstallState {
	case DLLInstallSelectType:
		b.WriteString(s.Dim.Render("Select a supported DLL family:"))
		b.WriteString("\n\n")

		if len(m.dllTypes) == 0 {
			b.WriteString(s.Dim.Render("Loading..."))
		} else {
			for i, t := range m.dllTypes {
				cursor := "  "
				style := s.Normal
				if i == m.dllTypeCursor {
					cursor = "> "
					style = s.Selected
				}
				b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, dllFamilyName(t))))
				b.WriteString("\n")
			}
		}

	case DLLInstallSelectVersion:
		b.WriteString(s.Dim.Render(fmt.Sprintf("Select %s version:", dllFamilyName(m.selectedDLLType))))
		b.WriteString("\n\n")

		if len(m.dllVersions) == 0 {
			if m.dllVersionsLoaded {
				b.WriteString(s.Error.Render("No versions available"))
			} else {
				b.WriteString(s.Dim.Render("Loading..."))
			}
		} else {
			start, end := visibleRange(m.dllVersionCursor, len(m.dllVersions), max(m.height-8, 1))
			for i := start; i < end; i++ {
				v := m.dllVersions[i]
				cursor := "  "
				style := s.Normal
				if i == m.dllVersionCursor {
					cursor = "> "
					style = s.Selected
				}
				label := v.Version
				if i == 0 {
					label += " (latest)"
				}
				b.WriteString(style.Render(truncate(fmt.Sprintf("%s%s", cursor, label), max(m.width-2, 1))))
				b.WriteString("\n")
			}
			if len(m.dllVersions) > end-start {
				b.WriteString(s.Dim.Render(fmt.Sprintf(" %d/%d", m.dllVersionCursor+1, len(m.dllVersions))))
			}
		}

	case DLLInstallDownloading:
		b.WriteString(s.Dim.Render("Installing DLL..."))
	}

	if hint := s.RenderHint("\n\n↑/↓ select • enter confirm • esc cancel"); hint != "" {
		b.WriteString(hint)
	}

	return b.String()
}

// renderDLLs renders the DLL versions section including any pending-action prompts.
func (m ContentModel) renderDLLs() string {
	if m.confirmation != nil {
		return m.confirmation.view(m.styles)
	}
	s := m.styles
	var b strings.Builder

	sectionStyle := s.Title.Foreground(s.Theme.Secondary)

	b.WriteString(sectionStyle.Render("DLL versions"))
	b.WriteString("\n")

	if len(m.game.DLLs) == 0 {
		b.WriteString(s.Dim.Render("  No managed DLL installed"))
		b.WriteString("\n")
		b.WriteString(s.Normal.Render("  i: install a supported family"))
		b.WriteString("\n")
		families := make([]string, 0, len(dllDisplayColumns))
		for _, info := range dllDisplayColumns {
			families = append(families, info.Label)
		}
		b.WriteString(s.Dim.Render("  Choices: " + strings.Join(families, ", ")))
		b.WriteString("\n")
	} else {
		// Build DLL type -> version mapping using DLLType constants directly
		dllVersions := make(map[game.DLLType]string)
		for _, d := range m.game.DLLs {
			version := d.Version
			if version == "" {
				version = "?"
			}
			dllVersions[d.Type] = version
		}

		// Column layout: type headers then versions
		columnWidth := 10

		// Header row
		b.WriteString("  ")
		for _, col := range dllDisplayColumns {
			b.WriteString(s.Dim.Render(fmt.Sprintf("%-*s", columnWidth, col.Label)))
		}
		b.WriteString("\n")

		// Version row
		b.WriteString("  ")
		for _, col := range dllDisplayColumns {
			version := dllVersions[col.Type]
			if version == "" {
				version = "-"
			}
			b.WriteString(s.DLSS.Render(fmt.Sprintf("%-*s", columnWidth, version)))
		}
		b.WriteString("\n")

		if m.pendingAction != PendingNone {
			var prompt string
			switch m.pendingAction {
			case PendingDLLUpdate:
				prompt = "Update DLLs? [Y]es • Esc cancel • q/Ctrl+C quit"
			case PendingDLLRestore:
				prompt = "Restore original DLLs? [Y]es • Esc cancel • q/Ctrl+C quit"
			}
			b.WriteString(s.Warning.Render("  " + prompt))
			b.WriteString("\n")
		} else if m.dllOperating {
			b.WriteString(s.Warning.Render("  ⟳ " + m.dllOperatingLabel))
			b.WriteString("\n")
		} else if s.ShowHints {
			var actions []string
			if m.hasUpdates {
				actions = append(actions, "u:update")
			}
			if m.hasBackup {
				actions = append(actions, "F6:restore")
			}
			if m.hasBackup {
				actions = append(actions, "(backup exists)")
			}

			if len(actions) > 0 {
				b.WriteString(s.RenderHint("  " + strings.Join(actions, " • ")))
				b.WriteString("\n")
			}
		}
	}
	if m.lastDLLResult != "" {
		b.WriteString(s.Dim.Render("  " + m.lastDLLResult))
		b.WriteString("\n")
	}

	return b.String()
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
