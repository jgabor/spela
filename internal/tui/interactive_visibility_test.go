package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/overlay"
	"github.com/jgabor/spela/internal/profile"
)

func TestProfileFocusStaysVisibleWhileEditingAndResizing(t *testing.T) {
	detail := NewDetail(NewStyles(DefaultTheme, true), &profile.Profile{}, &profile.Profile{})
	for _, height := range []int{40, 12} {
		detail.SetSize(70, height)
		detail.RestoreFocus("", 0)
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
	detail.UpdateAction(ActionFocusNext)
	detail.UpdateAction(ActionFocusNext)
	if !strings.Contains(stripANSI(detail.View()), "Cancel") {
		t.Fatal("local Cancel control is not visible")
	}
	detail.UpdateAction(ActionEditCommit)
	if detail.Editing() || detail.FocusedField() != wantField {
		t.Fatal("cancel lost the focused field")
	}
	detail.BeginEdit()
	detail.UpdateEditor(keyMsg("enter"))
	if detail.Editing() || detail.FocusedField() != wantField {
		t.Fatal("successful edit lost the focused field")
	}
}

func TestConstrainedProfileAndMonitorViewsStayComplete(t *testing.T) {
	t.Run("profile", func(t *testing.T) {
		layout := testLayoutWithGame(testGame("Cyberpunk 2077"))
		*layout.navState = layout.navState.SelectAspect(nav.AspectProfile)
		layout.focus = FocusDetail
		layout.width, layout.height = 80, 24
		layout.calculateDimensions()
		focusField(t, &layout.pane.content.detail, profile.FieldOverlayToggleKey)

		view := stripANSI(layout.View().Content)
		for _, want := range []string{"Toggle key", "Effect:"} {
			if !strings.Contains(view, want) {
				t.Fatalf("80x24 profile omitted %q:\n%s", want, view)
			}
		}
	})

	t.Run("monitor", func(t *testing.T) {
		alert := overlay.Alert{
			Type:       overlay.AlertThermalThrottle,
			Severity:   overlay.AlertCritical,
			Message:    "GPU thermal throttling at 92°C",
			Suggestion: "Approaching thermal limit — consider reducing power limit by 25W",
		}
		monitor := NewMetricsResource(NewStyles(DefaultTheme, true)).SetData(
			&gpu.GPUMetrics{Temperature: 92, PowerDraw: 275, PowerLimit: 300, Utilization: 98, MemoryUsed: 8192, MemoryTotal: 12288},
			&cpu.CPUMetrics{AverageFrequency: 4250, Utilization: 64.5, RAMUsedMB: 16384, RAMTotalMB: 32768},
			[]overlay.Alert{alert}, nil, nil, nil, nil,
		)
		monitor.SetSize(50, 13) // Detail content available in the 80x24 standard layout.
		sections := []struct {
			section      nav.MonitorSection
			unbrokenText string
		}{
			{nav.MonitorGPU, "8.0 / 12.0 GB"},
			{nav.MonitorCPU, "16.0 / 32.0 GB"},
			{nav.MonitorAlerts, "Recovery:"},
		}
		for _, test := range sections {
			view := monitor.View(true, test.section)
			lines := strings.Split(stripANSI(view), "\n")
			if lipgloss.Width(view) > 50 {
				t.Fatalf("80x24 monitor section %d exceeded its width:\n%s", test.section, stripANSI(view))
			}
			if !strings.HasSuffix(lines[0], "╮") || !strings.HasSuffix(lines[len(lines)-1], "╯") {
				t.Fatalf("80x24 monitor section %d broke its border:\n%s", test.section, stripANSI(view))
			}
			if !strings.Contains(stripANSI(view), test.unbrokenText) {
				t.Fatalf("80x24 monitor section %d wrapped %q:\n%s", test.section, test.unbrokenText, stripANSI(view))
			}
		}
		alerts := stripANSI(monitor.View(true, nav.MonitorAlerts))
		for _, want := range []string{"Recovery:", "consider reducing power limit by 25W"} {
			if !strings.Contains(alerts, want) {
				t.Fatalf("80x24 alerts omitted %q:\n%s", want, alerts)
			}
		}
	})
}

func TestHelpFitsAndScrollsToEveryLineWithCloseGuidance(t *testing.T) {
	help := NewHelp(NewStyles(DefaultTheme, true))
	help.SetHeight(100)
	if !strings.Contains(strings.ToLower(stripANSI(help.View())), "close help") {
		t.Fatal("fitting help omitted a shortcut")
	}
	help.SetHeight(5)
	help.Move(1000)
	view := stripANSI(help.View())
	if !strings.Contains(view, "Close") || !strings.Contains(view, "Tab: next control") || !strings.Contains(view, "↑ ↓: scroll") || lipgloss.Height(help.View()) > 5 {
		t.Fatalf("overflowing help did not expose its bounded final position:\n%s", view)
	}
	lastOffset := help.offset
	help.Move(1)
	if help.offset != lastOffset {
		t.Fatal("Help scroll moved beyond its last bounded position")
	}
	help.closeFocused = true
	if view := stripANSI(help.View()); !strings.Contains(view, "Enter: close") || strings.Contains(view, "↑ ↓: scroll") {
		t.Fatalf("Close control did not own the active guidance:\n%s", view)
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
	content.dllVersionsLoaded = true
	content.selectedDLLType = "dlss"
	for index := 0; index < 20; index++ {
		content.dllVersions = append(content.dllVersions, dll.DLL{Version: fmt.Sprintf("1.0.%d", index)})
	}
	content.dllVersionCursor = 19
	view = stripANSI(content.renderDLLInstallDialog())
	if strings.Count(view, "> 1.0.19") != 1 || strings.Contains(view, "1.0.0\n") || lipgloss.Height(content.renderDLLInstallDialog()) > 10 {
		t.Fatalf("version cursor is not one visible row:\n%s", view)
	}
}
