package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

// TestRenderProbe_Task4 emits ANSI-rendered output of the Games and
// Defaults resources so a human verifier can confirm the Task 4 acceptance
// criteria visually. Gated by SPELA_RENDER_PROBE=1 so it does not run in
// normal test suites.
//
//	SPELA_RENDER_PROBE=1 go test ./internal/tui/ -run TestRenderProbe_Task4 -v
func TestRenderProbe_Task4(t *testing.T) {
	if os.Getenv("SPELA_RENDER_PROBE") == "" {
		t.Skip("set SPELA_RENDER_PROBE=1 to see the render output")
	}

	// Shared fixture: a defaults profile with several fields set so the
	// resolved game-profile view surfaces inherited values, and a game
	// profile that pins HDR=true explicitly plus inherits everything else.
	defaults := &profile.Profile{
		Name: "Defaults",
		Proton: profile.ProtonSettings{
			EnableHDR:     false,
			EnableWayland: true,
			VKD3DHeap:     true,
		},
		DLSS: profile.DLSSSettings{
			SRMode:   profile.DLSSModeQuality,
			SRPreset: profile.DLSSPresetK,
		},
		GPU: profile.GPUSettings{
			PowerLimit: 350,
		},
		Overlay: profile.OverlaySettings{
			Enabled:  true,
			Position: "top-left",
		},
	}
	gameProfile := &profile.Profile{
		Name: "Cyberpunk 2077",
		Proton: profile.ProtonSettings{
			EnableHDR: true,
		},
		Overrides: map[string]bool{
			profile.FieldProtonEnableHDR: true,
		},
	}

	styles := NewStyles(DefaultTheme, true)
	svc := testServices()
	svc.LoadDefaultProfile = func() (*profile.Profile, error) { return defaults, nil }
	svc.LoadProfile = func(appID uint64) (*profile.Profile, error) { return gameProfile, nil }

	db := testDatabase(&game.Game{
		AppID: 1091500,
		Name:  "Cyberpunk 2077",
	})
	m := NewLayout(db, svc)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	layout := sized.(LayoutModel)

	// --- Games resource ---
	_ = styles
	// Press 1 to ensure Games is active.
	result, _ := sendKey(&layout, "1")
	layout = result.(LayoutModel)
	// Confirm game selection so detail pane is populated.
	gSelect, _ := layout.Update(gameSelectedMsg{game: &game.Game{AppID: 1091500, Name: "Cyberpunk 2077"}})
	layout = gSelect.(LayoutModel)
	gConfirm, _ := layout.Update(gameConfirmedMsg{game: &game.Game{AppID: 1091500, Name: "Cyberpunk 2077"}})
	layout = gConfirm.(LayoutModel)
	// Now tab into detail pane so j/k moves field focus.
	// (Cycle test — not strictly needed for rendered output.)

	gamesView := layout.pane.View(ResourceGames, true)
	fmt.Fprintln(os.Stderr, "═════════════════════ GAMES RESOURCE ═════════════════════")
	fmt.Fprintln(os.Stderr, gamesView)
	fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════════════════════")

	// --- Defaults resource ---
	result, _ = sendKey(&layout, "3")
	layout = result.(LayoutModel)
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	defaultsView := layout.pane.View(ResourceDefaults, true)
	fmt.Fprintln(os.Stderr, "═════════════════════ DEFAULTS RESOURCE ═════════════════════")
	fmt.Fprintln(os.Stderr, defaultsView)
	fmt.Fprintln(os.Stderr, "══════════════════════════════════════════════════════════════")

	// Press j twice on defaults, then render again to show focus movement.
	result, _ = sendKey(&layout, "j")
	layout = result.(LayoutModel)
	result, _ = sendKey(&layout, "j")
	layout = result.(LayoutModel)
	defaultsAfterJJ := layout.pane.View(ResourceDefaults, true)
	fmt.Fprintln(os.Stderr, "═══════════════ DEFAULTS RESOURCE AFTER 2× j ══════════════════")
	fmt.Fprintln(os.Stderr, defaultsAfterJJ)
	fmt.Fprintln(os.Stderr, "══════════════════════════════════════════════════════════════")
	fmt.Fprintf(os.Stderr, "defaultsDetail.Cursor() = %d (focused field = %q)\n",
		layout.pane.defaultsDetail.Cursor(),
		layout.pane.defaultsDetail.FocusedField())

	// Cross group boundary check: press j many times.
	initialCursor := layout.pane.defaultsDetail.Cursor()
	protonFieldCount := len(profile.SectionFields("proton"))
	// Move past proton into dlss.
	for i := 0; i <= protonFieldCount; i++ {
		result, _ = sendKey(&layout, "j")
		layout = result.(LayoutModel)
	}
	fmt.Fprintf(os.Stderr,
		"After %d more j presses: cursor went from %d to %d; focused field = %q\n",
		protonFieldCount+1,
		initialCursor,
		layout.pane.defaultsDetail.Cursor(),
		layout.pane.defaultsDetail.FocusedField())

	// Sanity: focused field should be a DLSS field (crossed boundary).
	if !strings.HasPrefix(layout.pane.defaultsDetail.FocusedField(), "dlss.") {
		t.Errorf("expected j presses to cross into dlss section, got focus on %q",
			layout.pane.defaultsDetail.FocusedField())
	}
}
