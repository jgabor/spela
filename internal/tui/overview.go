package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

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
	width    int
	height   int
	offset   int
}

func (m *OverviewModel) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m OverviewModel) Update(message tea.Msg) OverviewModel {
	key, ok := message.(tea.KeyPressMsg)
	if !ok {
		return m
	}
	switch key.String() {
	case "j", "down":
		m.offset++
	case "k", "up":
		m.offset = max(m.offset-1, 0)
	case "pgdown":
		m.offset += max(m.height-1, 1)
	case "pgup":
		m.offset = max(m.offset-max(m.height-1, 1), 0)
	}
	return m
}

func NewOverview(styles *Styles) OverviewModel {
	return OverviewModel{styles: styles}
}

func (m OverviewModel) SetGame(g *game.Game, svc *Services) OverviewModel {
	m.game = g
	m.offset = 0
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

	b.WriteString(s.Dim.Render("Location"))
	b.WriteString("\n")
	writeOverviewPath(&b, "Install directory", m.game.InstallDir, m.width)
	if m.game.PrefixPath != "" {
		writeOverviewPath(&b, "Proton prefix", m.game.PrefixPath, m.width)
	}

	overrideCount := 0
	if m.raw != nil {
		overrideCount = len(m.raw.Overrides)
	}
	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Status"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Profile overrides: %d\n", overrideCount)
	if len(m.game.DLLs) == 0 {
		b.WriteString("  DLLs: none detected\n")
	} else {
		b.WriteString("  DLLs:")
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
			if version != "-" {
				fmt.Fprintf(&b, " %s %s", col.columnName, version)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Steam launch options"))
	b.WriteString("\n  spela %command%\n")
	b.WriteString(s.Dim.Render(fmt.Sprintf("App ID %d", m.game.AppID)))
	b.WriteString("\n")

	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	visible := max(m.height, 1)
	m.offset = min(m.offset, max(len(lines)-visible, 0))
	end := min(m.offset+visible, len(lines))
	return strings.Join(lines[m.offset:end], "\n") + "\n"
}

func writeOverviewPath(b *strings.Builder, label, path string, width int) {
	fmt.Fprintf(b, "  %s:\n", label)
	const indent = "    "
	lineWidth := max(width-len(indent)-2, 20)
	for len(path) > lineWidth {
		fmt.Fprintf(b, "%s%s\n", indent, path[:lineWidth])
		path = path[lineWidth:]
	}
	fmt.Fprintf(b, "%s%s\n", indent, path)
}
