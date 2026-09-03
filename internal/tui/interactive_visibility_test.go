package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/profile"
)

func TestProfileFocusStaysVisibleWhileEditingAndResizing(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme, true), &profile.Profile{}, &profile.Profile{})
	for _, height := range []int{40, 12} {
		detail.SetSize(70, height)
		for detail.Cursor() < detail.FieldCount()-1 {
			detail, _, _ = detail.Update(keyMsg("down"))
			label := detail.rows[detail.focusableRows[detail.Cursor()]].label
			if !strings.Contains(stripANSI(detail.View()), label) || lipgloss.Height(detail.View()) > height {
				t.Fatalf("cursor %d (%s) is not visible within height %d:\n%s", detail.Cursor(), label, height, stripANSI(detail.View()))
			}
		}
	}
	wantField := detail.FocusedField()
	if !detail.BeginEdit() {
		t.Fatal("last field did not enter edit mode")
	}
	detail.editor.Set("true")
	detail.SetSize(50, 8)
	focusedLabel := detail.rows[detail.focusableRows[detail.Cursor()]].label
	if detail.FocusedField() != wantField || !detail.Editing() || !strings.Contains(stripANSI(detail.View()), focusedLabel) {
		t.Fatal("resize lost the focused edit context")
	}
	detail.UpdateEditor(keyMsg("esc"))
	if detail.Editing() || detail.FocusedField() != wantField {
		t.Fatal("cancel lost the focused field")
	}
	detail.BeginEdit()
	detail.UpdateEditor(keyMsg("enter"))
	if detail.Editing() || detail.FocusedField() != wantField {
		t.Fatal("successful edit lost the focused field")
	}
}

func TestHelpFitsAndScrollsToEveryLineWithCloseGuidance(t *testing.T) {
	help := NewHelp(NewStyles(DefaultTheme, true))
	help.SetHeight(100)
	if !strings.Contains(strings.ToLower(stripANSI(help.View())), "dll catalog") {
		t.Fatal("fitting help omitted a shortcut")
	}
	help.SetHeight(10)
	help.Move(1000)
	view := stripANSI(help.View())
	if !strings.Contains(view, "? / Esc close • q / Ctrl+C quit") || !strings.Contains(view, "↑/↓ scroll") || lipgloss.Height(help.View()) > 10 {
		t.Fatalf("overflowing help did not expose its bounded final position:\n%s", view)
	}
}

func TestOverflowingCollectionsKeepCursorOnOneVisibleRow(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	games := make([]*game.Game, 20)
	for index := range games {
		games[index] = &game.Game{AppID: uint64(index + 1), Name: fmt.Sprintf("Game %02d", index), DLLs: []game.DetectedDLL{{Type: game.DLLTypeDLSS}}}
	}
	resource := NewDLLsResource(styles, nil).SetGames(games)
	resource.SetSize(30, 8)
	resource.gameRowCursor = len(games) - 1
	view := stripANSI(resource.ListView(true, nav.SectionDLLDeployment))
	if strings.Count(view, "> Game 19") != 1 || lipgloss.Height(resource.ListView(true, nav.SectionDLLDeployment)) > 8 {
		t.Fatalf("deployment cursor is not one bounded row:\n%s", view)
	}

	content := NewContent(styles, false, testServices())
	content.SetSize(30, 10)
	content.dllInstallState = DLLInstallSelectVersion
	for index := 0; index < 20; index++ {
		content.dllVersions = append(content.dllVersions, dll.DLL{Version: fmt.Sprintf("1.0.%d", index)})
	}
	content.dllVersionCursor = 19
	view = stripANSI(content.renderDLLInstallDialog())
	if strings.Count(view, "> 1.0.19") != 1 || !strings.Contains(view, "20/20") {
		t.Fatalf("version cursor is not one visible row:\n%s", view)
	}
}
