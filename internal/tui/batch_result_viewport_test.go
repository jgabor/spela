package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestBatchFailureResultKeepsDetailsAndCloseReachable(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		for _, separator := range []string{"; ", "\n"} {
			format := "wrapped"
			if separator == "\n" {
				format = "multiline"
			}
			t.Run(fmt.Sprintf("%dx%d/%s", size.width, size.height, format), func(t *testing.T) {
				layout := testLayout(testGame("Fixture game"))
				updated, _ := layout.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
				layout = updated.(LayoutModel)
				layout.styles.SetShowHints(false)
				layout.showBatchMenu = true
				layout.batchMessage = strings.Repeat("/temporary/game/path/nvngx_dlss.dll: permission denied"+separator, 45) + "\nFINAL BATCH FAILURE"
				view := stripANSI(layout.View().Content)
				if strings.Contains(view, "FINAL BATCH FAILURE") {
					t.Fatal("fixture must overflow the result viewport")
				}
				for step := 0; step < 150 && !strings.Contains(view, "FINAL BATCH FAILURE"); step++ {
					if !strings.Contains(view, "Close") || !strings.Contains(view, "Tab") || !strings.Contains(view, "↓") {
						t.Fatalf("result details cannot be read using displayed controls:\n%s", view)
					}
					updated, _ = sendKey(layout, "down")
					layout = updated.(LayoutModel)
					view = stripANSI(layout.View().Content)
				}
				if !strings.Contains(view, "FINAL BATCH FAILURE") {
					t.Fatalf("final failure detail is unreachable:\n%s", view)
				}
				if strings.Contains(view, "Enter Close") {
					t.Fatal("body focus advertises activating the unfocused Close control")
				}
				updated, _ = sendKey(layout, "tab")
				layout = updated.(LayoutModel)
				view = stripANSI(layout.View().Content)
				if !strings.Contains(view, "Enter Close") || strings.Contains(view, "↓ Scroll") {
					t.Fatalf("focused Close has incorrect guidance:\n%s", view)
				}
				updated, _ = sendKey(layout, "enter")
				layout = updated.(LayoutModel)
				if layout.showBatchMenu || !strings.Contains(stripANSI(layout.View().Content), "0: Actions") {
					t.Fatal("displayed Close did not return to Browse")
				}
			})
		}
	}
}

func TestBatchWithoutConcreteTargetsKeepsAvailabilityTruthful(t *testing.T) {
	layout := testLayout(testGame("No DLLs"))
	selected, _ := sendKey(&layout, "space")
	layout = selected.(LayoutModel)
	action := CanonicalKeymap.Action(layout.bindingContext(), ActionBatchUpdate)
	if action.Available || action.Reason != "no updates for selected filtered games" {
		t.Fatalf("targetless update availability = %#v", action)
	}
	opened, _ := sendKey(&layout, "enter")
	layout = opened.(LayoutModel)
	view := stripANSI(layout.renderBatchMenu())
	if !strings.Contains(view, "No available updates") || strings.Contains(view, "Enter:") {
		t.Fatalf("targetless batch guidance is misleading: %s", view)
	}
	unchanged, command := sendKey(&layout, "enter")
	if command != nil || unchanged.(LayoutModel).batchConfirmation != nil || stripANSI(unchanged.(LayoutModel).renderBatchMenu()) != view {
		t.Fatal("unavailable batch Enter changed state")
	}
	closed, _ := sendKeys(unchanged, "tab", "enter")
	if closed.(LayoutModel).showBatchMenu {
		t.Fatal("targetless batch Close failed")
	}
}
