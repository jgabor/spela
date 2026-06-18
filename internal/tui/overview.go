package tui

import (
	"fmt"
	"strings"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

// OverviewModel renders the read-only Library › Overview aspect for a game.
type OverviewModel struct {
	styles   *Styles
	game     *game.Game
	profile  *profile.Profile
	defaults *profile.Profile
	raw      *profile.Profile
}

func NewOverview(styles *Styles) OverviewModel {
	return OverviewModel{styles: styles}
}

func (m OverviewModel) SetGame(g *game.Game, svc *Services) OverviewModel {
	m.game = g
	if g == nil || svc == nil {
		return m
	}
	m.raw, _ = svc.LoadProfile(g.AppID)
	m.defaults, _ = svc.LoadDefaultProfile()
	if m.raw != nil {
		m.profile = m.raw.ResolveForApply(m.defaults)
	} else {
		m.profile = m.defaults
	}
	return m
}

func (m OverviewModel) View() string {
	if m.game == nil {
		return m.styles.Dim.Render("Select a game from the scope list")
	}
	s := m.styles
	var b strings.Builder

	b.WriteString(s.Title.Render(m.game.Name))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "App ID:      %d\n", m.game.AppID)
	fmt.Fprintf(&b, "Install dir: %s\n", m.game.InstallDir)
	if m.game.PrefixPath != "" {
		fmt.Fprintf(&b, "Prefix:      %s\n", m.game.PrefixPath)
	}

	overrideCount := 0
	if m.raw != nil {
		overrideCount = len(m.raw.Overrides)
	}
	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Profile"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Overrides: %d\n", overrideCount)

	b.WriteString("\n")
	b.WriteString(s.Dim.Render("DLL status"))
	b.WriteString("\n")
	if len(m.game.DLLs) == 0 {
		b.WriteString("  (none detected)\n")
	} else {
		for _, col := range dllDisplayColumns {
			version := "-"
			for _, d := range m.game.DLLs {
				if d.Type == col.dllType {
					version = d.Version
					if version == "" {
						version = "?"
					}
					break
				}
			}
			fmt.Fprintf(&b, "  %-8s %s\n", col.columnName+":", version)
		}
	}

	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Launch via Steam: spela %command%"))
	b.WriteString("\n")

	return strings.TrimRight(b.String(), "\n") + "\n"
}
