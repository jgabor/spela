package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
)

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
		if compact == standard || strings.Contains(stripANSI(compact), logo[0]) {
			t.Fatalf("%dx%d compact mode did not visibly reduce the header", size[0], size[1])
		}

		layout.densityMode = DensityFocused
		for _, focus := range []KeyFocus{FocusList, FocusDetail} {
			layout.focus = focus
			layout.calculateDimensions()
			view := layout.View().Content
			plainView := stripANSI(view)
			if !strings.Contains(plainView, "Library") || !strings.Contains(plainView, "spela › Library") {
				t.Fatalf("%dx%d focused %s view lacks orientation", size[0], size[1], focus)
			}
			if layout.listPane.width != size[0]-2 || layout.pane.width != size[0]-2 {
				t.Fatalf("%dx%d focused panes did not receive full width", size[0], size[1])
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
		if fixture.width >= len("q quit") && !strings.Contains(view, "q quit") {
			t.Fatalf("%dx%d view does not retain quit: %q", fixture.width, fixture.height, view)
		}
		if got := lipgloss.Width(view); got > fixture.width {
			t.Fatalf("%dx%d rendered width = %d", fixture.width, fixture.height, got)
		}
		if got := lipgloss.Height(view); got > fixture.height {
			t.Fatalf("%dx%d rendered height = %d", fixture.width, fixture.height, got)
		}
	}
}

func TestLayoutBelowMinimumQuitsAndRecoversAfterResize(t *testing.T) {
	layout := testLayout()
	model, _ := layout.Update(tea.WindowSizeMsg{Width: 79, Height: 23})
	layout = model.(LayoutModel)
	_, command := layout.Update(keyMsg("q"))
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("q did not remain available below minimum")
	}

	model, _ = layout.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	recovered := model.(LayoutModel).View().Content
	if strings.Contains(recovered, "Resize terminal") || !strings.Contains(recovered, "Library") {
		t.Fatalf("resized view did not recover: %q", recovered)
	}
}
