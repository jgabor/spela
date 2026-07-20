package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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
			layout.width = fixture.width
			layout.height = fixture.height
			layout.calculateDimensions()

			view := layout.View().Content
			if got := lipgloss.Width(view); got > fixture.width {
				t.Fatalf("rendered width = %d, viewport width = %d", got, fixture.width)
			}
			if got := lipgloss.Height(view); got > fixture.height {
				t.Fatalf("rendered height = %d, viewport height = %d\n%s", got, fixture.height, view)
			}
		})
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
		if got := lipgloss.Width(view); got > fixture.width {
			t.Fatalf("%dx%d rendered width = %d", fixture.width, fixture.height, got)
		}
		if got := lipgloss.Height(view); got > fixture.height {
			t.Fatalf("%dx%d rendered height = %d", fixture.width, fixture.height, got)
		}
	}
}
