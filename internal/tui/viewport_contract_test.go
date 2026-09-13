package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
)

func TestFreshMinimumLayoutClosesLongPathDetailBorder(t *testing.T) {
	entry := testGame("Cyberpunk 2077")
	entry.InstallDir = "/tmp/" + strings.Repeat("deterministic-long-install-path/", 8)
	entry.PrefixPath = "/tmp/" + strings.Repeat("deterministic-long-prefix-path/", 8)
	layout := testLayoutWithGame(entry)
	layout.width, layout.height = minimumTerminalWidth, minimumTerminalHeight
	layout.calculateDimensions()

	view := stripANSI(layout.View().Content)
	lines := strings.Split(view, "\n")
	detailTop := -1
	for index, line := range lines {
		line = strings.TrimRight(line, " ")
		if strings.Contains(line, "Detail") && strings.HasSuffix(line, "╮") {
			detailTop = index
			break
		}
	}
	if detailTop < 0 {
		t.Fatalf("80x24 Detail top border is missing:\n%s", view)
	}
	for _, line := range lines[detailTop+1:] {
		line = strings.TrimRight(line, " ")
		if strings.HasPrefix(line, "╰") && strings.HasSuffix(line, "╯") {
			return
		}
	}
	t.Fatalf("80x24 long-path Detail bottom border is missing:\n%s", view)
}

func TestLayoutViewportContract(t *testing.T) {
	t.Parallel()

	for _, fixture := range []struct {
		name   string
		width  int
		height int
	}{
		{name: "normal", width: 120, height: 40},
		{name: "minimum", width: minimumTerminalWidth, height: minimumTerminalHeight},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			layout := testLayout()
			layout.header.gpuMetrics = &gpu.GPUMetrics{Temperature: 32, PowerDraw: 4, Utilization: 1, MemoryUsed: 2200, MemoryTotal: 12000}
			layout.header.cpuMetrics = &cpu.CPUMetrics{AverageFrequency: 5200, Utilization: 4, RAMUsedMB: 18000, RAMTotalMB: 32000}
			layout.width = fixture.width
			layout.height = fixture.height
			for _, density := range []DensityMode{DensityStandard, DensityCompact, DensityFocused} {
				layout.densityMode = density
				layout.calculateDimensions()

				view := layout.View().Content
				if got := lipgloss.Width(view); got > fixture.width {
					t.Fatalf("density %d rendered width = %d, viewport width = %d", density, got, fixture.width)
				}
				if got := lipgloss.Height(view); got > fixture.height {
					t.Fatalf("density %d rendered height = %d, viewport height = %d\n%s", density, got, fixture.height, view)
				}
			}
		})
	}
}

func TestLayoutFeedbackRemainsWithinViewport(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}} {
		for _, messageType := range []MessageType{MessageInfo, MessageSuccess, MessageError} {
			layout := testLayout()
			layout.width, layout.height = size[0], size[1]
			layout.calculateDimensions()
			layout.messageBar.SetMessage(strings.Repeat("feedback ", 20), messageType)

			view := layout.View().Content
			if got := lipgloss.Width(view); got > size[0] {
				t.Fatalf("%dx%d message type %d width = %d", size[0], size[1], messageType, got)
			}
			if got := lipgloss.Height(view); got > size[1] {
				t.Fatalf("%dx%d message type %d height = %d", size[0], size[1], messageType, got)
			}
			if !strings.Contains(view, "feedback") || !strings.Contains(view, "Library") {
				t.Fatalf("%dx%d feedback displaced context", size[0], size[1])
			}
		}
	}
}

func TestCompactAndFocusedLayoutsUseAvailableSpace(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}} {
		layout := testLayout(testGame("Cyberpunk 2077"))
		layout.width, layout.height = size[0], size[1]

		layout.densityMode = DensityStandard
		layout.calculateDimensions()
		standard := layout.View().Content
		layout.densityMode = DensityCompact
		layout.calculateDimensions()
		compact := layout.View().Content
		if strings.Contains(stripANSI(compact), logo[0]) || size[0] >= 100 && size[1] >= 30 && compact == standard {
			t.Fatalf("%dx%d compact mode did not visibly reduce the header", size[0], size[1])
		}

		layout.densityMode = DensityFocused
		for _, focus := range []KeyFocus{FocusList, FocusDetail} {
			layout.focus = focus
			layout.calculateDimensions()
			view := layout.View().Content
			plainView := stripANSI(view)
			title := "List"
			if focus == FocusDetail {
				title = "Detail"
			}
			if !strings.Contains(plainView, "Library") || !strings.Contains(plainView, "▸ "+title) || !strings.Contains(plainView, "0: Actions") || !strings.Contains(plainView, "Tab: Switch pane") {
				t.Fatalf("%dx%d focused %s view lacks orientation", size[0], size[1], focus)
			}
			if layout.listPane.sidebar.width != size[0]-2 || layout.pane.width != size[0]-2 {
				t.Fatalf("%dx%d focused panes did not receive full content width", size[0], size[1])
			}
			if strings.Count(plainView, "╭ ▸ ") != 1 || lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
				t.Fatalf("%dx%d focused view exposed extra panes or exceeded its viewport:\n%s", size[0], size[1], plainView)
			}
		}
	}
}

func TestLayoutBelowMinimumRendersBoundedResizePrompt(t *testing.T) {
	t.Parallel()

	for _, fixture := range []struct {
		width  int
		height int
	}{
		{width: minimumTerminalWidth - 1, height: minimumTerminalHeight},
		{width: minimumTerminalWidth, height: minimumTerminalHeight - 1},
		{width: 12, height: 3},
	} {
		layout := testLayout()
		layout.width = fixture.width
		layout.height = fixture.height

		view := layout.View().Content
		if !strings.Contains(view, "Resize") {
			t.Fatalf("%dx%d view does not explain recovery: %q", fixture.width, fixture.height, view)
		}
		for _, hiddenControl := range []string{"q quit", "0: Actions", "Tab:", "Enter:"} {
			if strings.Contains(view, hiddenControl) {
				t.Fatalf("%dx%d resize prompt advertises hidden input %q: %q", fixture.width, fixture.height, hiddenControl, view)
			}
		}
		if got := lipgloss.Width(view); got > fixture.width {
			t.Fatalf("%dx%d rendered width = %d", fixture.width, fixture.height, got)
		}
		if got := lipgloss.Height(view); got > fixture.height {
			t.Fatalf("%dx%d rendered height = %d", fixture.width, fixture.height, got)
		}
	}
}

func TestLayoutBelowMinimumSuspendsInputAndRecoversAfterResize(t *testing.T) {
	layout := testLayoutWithGame(testGame("Cyberpunk 2077"))
	beforeScope, beforeFocus := layout.navState.Scope, layout.focus
	model, _ := layout.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	layout = model.(LayoutModel)
	for _, key := range []string{"q", "0", "1", "4", "tab", "enter", "space", "down"} {
		model, command := layout.Update(keyMsg(key))
		layout = model.(LayoutModel)
		if command != nil || layout.navState.Scope != beforeScope || layout.focus != beforeFocus || layout.actions != nil || layout.decision != nil {
			t.Fatalf("hidden workspace accepted %q below the supported size", key)
		}
	}
	_, command := layout.Update(keyMsg("ctrl+c"))
	if command == nil {
		t.Fatal("emergency terminal interrupt was lost below minimum")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("terminal interrupt did not quit below minimum")
	}

	model, _ = layout.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	recovered := model.(LayoutModel).View().Content
	if strings.Contains(recovered, "Resize terminal") || !strings.Contains(recovered, "Library") || !strings.Contains(recovered, "0: Actions") {
		t.Fatalf("resized view did not recover: %q", recovered)
	}
}
