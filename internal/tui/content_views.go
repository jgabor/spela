package tui

import (
	"fmt"
	"strings"

	"github.com/jgabor/spela/internal/game"
)

// renderDefaultProfile renders the default-profile view.
func (m ContentModel) renderDefaultProfile() string {
	var b strings.Builder

	b.WriteString(m.styles.Title.Render("Default profile"))
	b.WriteString("\n\n")
	b.WriteString(m.renderProfile())

	return b.String()
}

// renderDLLInstallDialog renders the multi-step DLL install wizard.
func (m ContentModel) renderDLLInstallDialog() string {
	s := m.styles
	var b strings.Builder

	b.WriteString(s.Title.Render("Install DLL"))
	b.WriteString("\n\n")

	switch m.dllInstallState {
	case DLLInstallSelectType:
		b.WriteString(s.Dim.Render("Select DLL type:"))
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
				b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, strings.ToUpper(t))))
				b.WriteString("\n")
			}
		}

	case DLLInstallSelectVersion:
		b.WriteString(s.Dim.Render(fmt.Sprintf("Select %s version:", strings.ToUpper(m.selectedDLLType))))
		b.WriteString("\n\n")

		if len(m.dllVersions) == 0 {
			if m.dllVersionsLoaded {
				b.WriteString(s.Error.Render("No versions available"))
			} else {
				b.WriteString(s.Dim.Render("Loading..."))
			}
		} else {
			for i, v := range m.dllVersions {
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
				b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, label)))
				b.WriteString("\n")
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

// renderGameInfo renders the game name, App ID, install directory and prefix lines.
func (m ContentModel) renderGameInfo() string {
	var b strings.Builder

	b.WriteString(m.styles.Title.Render(m.game.Name))
	b.WriteString("\n\n")

	lines := 2 // title + blank line
	fmt.Fprintf(&b, "App ID:      %d\n", m.game.AppID)
	lines++
	fmt.Fprintf(&b, "Install Dir: %s\n", m.game.InstallDir)
	lines++

	if m.game.PrefixPath != "" {
		fmt.Fprintf(&b, "Prefix:      %s\n", m.game.PrefixPath)
		lines++
	}

	// Pad to fixed height
	for lines < headerSectionHeight {
		b.WriteString("\n")
		lines++
	}

	return b.String()
}

// renderDLLs renders the DLL versions section including any pending-action prompts.
func (m ContentModel) renderDLLs() string {
	s := m.styles
	var b strings.Builder

	sectionStyle := s.Title.Foreground(s.Theme.Secondary)

	b.WriteString(sectionStyle.Render("DLL versions"))
	b.WriteString("\n")

	if len(m.game.DLLs) == 0 {
		b.WriteString(s.Dim.Render("  No DLLs detected"))
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
			b.WriteString(s.Dim.Render(fmt.Sprintf("%-*s", columnWidth, col.columnName)))
		}
		b.WriteString("\n")

		// Version row
		b.WriteString("  ")
		for _, col := range dllDisplayColumns {
			version := dllVersions[col.dllType]
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
				prompt = "Update DLLs? [Y]es / any key to cancel"
			case PendingDLLRestore:
				prompt = "Restore original DLLs? [Y]es / any key to cancel"
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
				actions = append(actions, "R:restore")
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

	return b.String()
}

// renderProfile renders the profile detail section using the shared
// single-column grouped-by-subsystem DetailModel (Task 4 — Decision 1).
// Task 5 layers inheritance markers and reset/pin bindings on top.
func (m ContentModel) renderProfile() string {
	var b strings.Builder
	if m.usingDefaultProfile {
		b.WriteString(m.styles.Dim.Render("Using default profile values (inherited)"))
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
