package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

// dllsSectionTitle is the last entry of the section strip on a game page.
const dllsSectionTitle = "DLLs"

// profileHeader renders the three header lines above a profile page: what is
// open and whether it is saved, which settings the game sets itself, and the
// section strip that Left and Right move along.
func (p resourcePaneModel) profileHeader() string {
	s := p.styles
	width := max(p.width-2, 1)
	detail := p.profileDetail()
	global := p.State().Scope.Kind == nav.ScopeGlobal

	status := detail.SaveStatus()
	statusStyle := s.Dim
	switch {
	case detail.SaveError() != nil:
		statusStyle = s.Error
	case detail.Dirty():
		statusStyle = s.Warning
	}

	var first, second string
	if global {
		first = s.Title.MarginBottom(0).Render(profile.AllGamesName) + s.Dim.Render(" · ") + statusStyle.Render(status)
		second = s.Dim.Render("Games follow these values unless they set their own.")
	} else {
		g := p.content.game
		name := fmt.Sprintf("%s · App ID %d", g.Name, g.AppID)
		tail := statusStyle.Render(status)
		first = s.Title.MarginBottom(0).Render(name) + s.Dim.Render(" · ") + tail
		if versions := dllVersionSummary(g); ansi.StringWidth(name+" · DLLs: "+versions+" · "+status) <= width {
			first = s.Title.MarginBottom(0).Render(name) + s.Dim.Render(" · DLLs: "+versions+" · ") + tail
		}
		own := detail.OwnSettings()
		if len(own) == 0 {
			second = s.Dim.Render("Own settings: none, every value follows All games")
		} else {
			second = s.Dim.Render(fmt.Sprintf("Own settings (%d): %s", len(own), strings.Join(own, " · ")))
		}
	}
	lines := []string{first, second, p.sectionStrip(detail)}
	for index, line := range lines {
		lines[index] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

// sectionStrip names the sections of the page with the current one marked. The
// DLLs section exists for a game only.
func (p resourcePaneModel) sectionStrip(detail *DetailModel) string {
	s := p.styles
	global := p.State().Scope.Kind == nav.ScopeGlobal
	current := detail.CurrentSection()
	if !global && p.State().Aspect == nav.AspectDLLs {
		current = ""
	}
	var entries []string
	render := func(title string, selected bool) {
		if selected {
			entries = append(entries, s.Selected.Render("["+title+"]"))
		} else {
			entries = append(entries, s.Dim.Render(title))
		}
	}
	for _, section := range detailSectionOrder {
		render(detailSectionTitle[section], section == current)
	}
	if !global {
		render(dllsSectionTitle, p.State().Aspect == nav.AspectDLLs)
	}
	return strings.Join(entries, "  ")
}

// dllVersionSummary lists the detected DLL families and versions in one line.
func dllVersionSummary(g *game.Game) string {
	if g == nil || len(g.DLLs) == 0 {
		return "none detected"
	}
	var parts []string
	for _, column := range dllDisplayColumns {
		for _, detected := range g.DLLs {
			if detected.Type != column.Type {
				continue
			}
			version := detected.Version
			if version == "" {
				version = "?"
			}
			parts = append(parts, column.Label+" "+version)
			break
		}
	}
	if len(parts) == 0 {
		return "none detected"
	}
	return strings.Join(parts, ", ")
}
