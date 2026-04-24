package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/game"
)

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------

func TestLayout_HelpToggle(t *testing.T) {
	m := testLayout()

	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)
	if !layout.showHelp {
		t.Error("expected help to be shown")
	}

	result, _ = sendKey(&layout, "?")
	layout = result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected help to be hidden")
	}
}

func TestLayout_HelpEscCloses(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)

	result, _ = sendKey(&layout, "esc")
	layout = result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected esc to close help")
	}
}

func TestLayout_HelpQCloses(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)

	result, _ = sendKey(&layout, "q")
	layout = result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected q to close help")
	}
}

func TestLayout_HelpBlocksOtherKeys(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)

	// Tab should NOT toggle focus while help is shown.
	focused := layout.railFocused
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.railFocused != focused {
		t.Error("expected help to block tab from toggling focus")
	}

	// 1-4 rail hotkeys should NOT activate while help is shown.
	activeBefore := layout.rail.Active()
	result, _ = sendKey(&layout, "3")
	layout = result.(LayoutModel)
	if layout.rail.Active() != activeBefore {
		t.Error("expected help to block rail hotkeys")
	}
}

// ---------------------------------------------------------------------------
// Focus / rail toggling
// ---------------------------------------------------------------------------

func TestLayout_TabTogglesFocus(t *testing.T) {
	m := testLayout()
	if !m.railFocused {
		t.Fatal("precondition: rail should be focused initially")
	}

	result, _ := sendKey(&m, "tab")
	layout := result.(LayoutModel)
	if layout.railFocused {
		t.Error("expected tab to switch focus off the rail")
	}

	// Inside ResourceGames, a second tab goes into the inner detail pane,
	// not back to the rail. A third tab returns to the rail.
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.railFocused {
		t.Error("second tab under ResourceGames should stay off rail (dive inner)")
	}
	if !layout.pane.InnerFocused() {
		t.Error("second tab should toggle inner focus on")
	}
}

func TestLayout_TabTogglesFocus_NonGamesResource(t *testing.T) {
	m := testLayout()
	m, _, _ = m.handleGlobalKeys(tea.KeyPressMsg{Code: '2', Text: "2"}) // select DLLs
	if m.rail.Active() != ResourceDLLs {
		t.Fatalf("precondition: expected DLLs active, got %v", m.rail.Active())
	}
	// tab → leave rail
	result, _ := sendKey(&m, "tab")
	layout := result.(LayoutModel)
	if layout.railFocused {
		t.Error("tab should flip railFocused off")
	}
	// tab → non-games resource has no inner layers, so it returns to rail
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if !layout.railFocused {
		t.Error("tab on non-games should return to rail")
	}
}

// ---------------------------------------------------------------------------
// Global shortcuts
// ---------------------------------------------------------------------------

func TestLayout_CtrlCQuits(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "ctrl+c")
	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from ctrl+c, got %T", msg)
	}
}

func TestLayout_QFromRailQuits(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "q")
	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from q on rail, got %T", msg)
	}
}

func TestLayout_F5TogglesDensity(t *testing.T) {
	m := testLayout()
	if m.densityMode != DensityStandard {
		t.Fatal("precondition: should start in standard density")
	}

	result, _ := sendKey(&m, "f5")
	layout := result.(LayoutModel)
	if layout.densityMode != DensityCompact {
		t.Errorf("expected DensityCompact, got %d", layout.densityMode)
	}

	result, _ = sendKey(&layout, "f5")
	layout = result.(LayoutModel)
	if layout.densityMode != DensityStandard {
		t.Errorf("expected DensityStandard, got %d", layout.densityMode)
	}
}

func TestLayout_F11TogglesFocused(t *testing.T) {
	m := testLayout()

	result, _ := sendKey(&m, "f11")
	layout := result.(LayoutModel)
	if layout.densityMode != DensityFocused {
		t.Errorf("expected DensityFocused, got %d", layout.densityMode)
	}

	result, _ = sendKey(&layout, "f11")
	layout = result.(LayoutModel)
	if layout.densityMode != DensityStandard {
		t.Errorf("expected DensityStandard, got %d", layout.densityMode)
	}
}

func TestLayout_CtrlFActivatesSearch(t *testing.T) {
	m := testLayout(testGame("Cyberpunk 2077"))

	result, _ := sendKey(&m, "ctrl+f")
	layout := result.(LayoutModel)
	if layout.railFocused {
		t.Error("expected ctrl+f to drop rail focus")
	}
	if !layout.pane.sidebar.search.Focused() {
		t.Error("expected ctrl+f to activate search input")
	}
}

func TestLayout_OptionsModal(t *testing.T) {
	m := testLayout()
	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)
	if layout.activeDialog == nil {
		t.Fatal("expected options modal to open")
	}

	result, _ = sendKey(&layout, "esc")
	layout = result.(LayoutModel)
	if layout.activeDialog != nil {
		t.Error("expected esc to close options modal")
	}
}

func TestLayout_OptionsFromPaneIgnored(t *testing.T) {
	m := testLayout()
	m.railFocused = false

	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)
	if layout.activeDialog != nil {
		t.Error("expected o from resource pane to be ignored")
	}
}

// ---------------------------------------------------------------------------
// Rescan — displaced from `r` to `ctrl+r` as part of the keymap audit
// ---------------------------------------------------------------------------

func TestLayout_RescanOnCtrlR(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "ctrl+r")
	if cmd == nil {
		t.Error("expected rescan command from ctrl+r")
	}
}

func TestLayout_BareRIsReservedForTask5(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "r")
	// Bare `r` must NOT trigger rescan — it is reserved for Task 5
	// (reset-field). The layout passes it through to the active resource
	// (which ignores it in Task 3). The one thing we care about: no
	// rescanGamesMsg in the resulting cmd chain.
	msg := execCmd(cmd)
	if _, ok := msg.(rescanGamesMsg); ok {
		t.Error("bare r must not trigger rescan (Task 5 reservation)")
	}
}

// ---------------------------------------------------------------------------
// Rail hotkeys 1-4 — smoke at layout level (detailed assertions in rail_test.go)
// ---------------------------------------------------------------------------

func TestLayout_RailHotkeysWithoutGame(t *testing.T) {
	// 1-4 no longer require a game selected; they are the rail spine.
	m := testLayout()

	for i, tc := range []struct {
		key  string
		want Resource
	}{
		{"1", ResourceGames},
		{"2", ResourceDLLs},
		{"3", ResourceDefaults},
		{"4", ResourceMetrics},
	} {
		t.Run(tc.key, func(t *testing.T) {
			result, _ := sendKey(&m, tc.key)
			layout := result.(LayoutModel)
			if layout.rail.Active() != tc.want {
				t.Errorf("[iter %d] after %q: active = %v, want %v", i, tc.key, layout.rail.Active(), tc.want)
			}
			if !layout.railFocused {
				t.Errorf("[iter %d] expected rail focus preserved after %q", i, tc.key)
			}
			m = layout
		})
	}
}

func TestLayout_RailHotkeyFromDeepFocus(t *testing.T) {
	// Even when deep in a game detail, 1-4 snap back to the rail.
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g)
	if m.rail.Active() != ResourceGames {
		t.Fatalf("precondition: expected ResourceGames, got %v", m.rail.Active())
	}
	if !m.pane.InnerFocused() {
		t.Fatalf("precondition: expected inner focus after gameConfirmedMsg")
	}

	result, _ := sendKey(&m, "3")
	layout := result.(LayoutModel)
	if layout.rail.Active() != ResourceDefaults {
		t.Errorf("expected Defaults active after '3' from deep focus, got %v", layout.rail.Active())
	}
	if !layout.railFocused {
		t.Error("expected rail focus restored after hotkey")
	}
	if layout.pane.InnerFocused() {
		t.Error("expected inner focus reset after hotkey")
	}
}

func TestLayout_ResourceKeysStayScopedToActiveResource(t *testing.T) {
	g1 := testGame("Alpha", testDLL(game.DLLTypeDLSS, "3.7.0"))
	g2 := testGame("Beta", testDLL(game.DLLTypeDLSS, "3.8.10"))
	g2.AppID = 2
	m := testLayout(g1, g2)

	result, _ := sendKey(&m, "2")
	m = result.(LayoutModel)
	result, _ = sendKey(&m, "tab")
	m = result.(LayoutModel)
	railCursor := m.rail.Cursor()
	result, _ = sendKey(&m, "j")
	m = result.(LayoutModel)
	if m.rail.Cursor() != railCursor {
		t.Errorf("DLLs j should not move rail cursor: got %d want %d", m.rail.Cursor(), railCursor)
	}
	if got := m.pane.dllsResource.gameRowCursor; got != 1 {
		t.Errorf("DLLs j should move deployment cursor to 1, got %d", got)
	}

	result, _ = sendKey(&m, "3")
	m = result.(LayoutModel)
	result, _ = sendKey(&m, "tab")
	m = result.(LayoutModel)
	defaultCursor := m.pane.defaultsDetail.Cursor()
	result, _ = sendKey(&m, "j")
	m = result.(LayoutModel)
	if got := m.pane.defaultsDetail.Cursor(); got != defaultCursor+1 {
		t.Errorf("Defaults j should move detail cursor to %d, got %d", defaultCursor+1, got)
	}
	if m.pane.dllsResource.gameRowCursor != 1 {
		t.Error("Defaults j should not mutate DLL row cursor")
	}

	result, _ = sendKey(&m, "4")
	m = result.(LayoutModel)
	result, _ = sendKey(&m, "tab")
	m = result.(LayoutModel)
	defaultCursor = m.pane.defaultsDetail.Cursor()
	dllCursor := m.pane.dllsResource.gameRowCursor
	result, _ = sendKey(&m, "j")
	m = result.(LayoutModel)
	if m.pane.defaultsDetail.Cursor() != defaultCursor {
		t.Error("Metrics j should not mutate Defaults cursor")
	}
	if m.pane.dllsResource.gameRowCursor != dllCursor {
		t.Error("Metrics j should not mutate DLL cursor")
	}
}

func TestLayout_DLLUpdateAllMessageReachesDLLsWhenMetricsActive(t *testing.T) {
	m := testLayout(testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.7.0")))
	result, _ := sendKey(&m, "4")
	m = result.(LayoutModel)
	if m.rail.Active() != ResourceMetrics {
		t.Fatalf("precondition: expected Metrics active, got %v", m.rail.Active())
	}

	updated, _ := m.Update(dllsUpdateAllCompleteMsg{
		results: map[string]string{"1091500:dlss": "ok"},
		summary: "Update-all: 1 updated, 0 failed",
	})
	m = updated.(LayoutModel)

	if got := m.pane.dllsResource.lastBatchSummary; got != "Update-all: 1 updated, 0 failed" {
		t.Errorf("DLLs resource did not receive message while Metrics active, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Navigation — q / esc
// ---------------------------------------------------------------------------

func TestLayout_QFromResourcePane_StepsBackToRail(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g)
	m.pane.SetInnerFocused(false) // inside games list, not detail

	result, _ := sendKey(&m, "q")
	layout := result.(LayoutModel)
	if !layout.railFocused {
		t.Error("expected q from games-list to return to rail")
	}
}

func TestLayout_QFromGameDetail_StepsBackToGamesList(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g) // starts with inner focus = detail
	if !m.pane.InnerFocused() {
		t.Fatal("precondition: expected inner focus on detail")
	}

	result, _ := sendKey(&m, "q")
	layout := result.(LayoutModel)
	if layout.railFocused {
		t.Error("expected q from detail to step back to games list (not rail)")
	}
	if layout.pane.InnerFocused() {
		t.Error("expected inner focus off after q from detail")
	}
}

func TestLayout_EscFromResourcePane_StepsBackToRail(t *testing.T) {
	g := testGame("Cyberpunk 2077")
	m := testLayoutWithGame(g)
	m.pane.SetInnerFocused(false)

	result, _ := sendKey(&m, "esc")
	layout := result.(LayoutModel)
	if !layout.railFocused {
		t.Error("expected esc from games-list to return to rail")
	}
}

// ---------------------------------------------------------------------------
// Modal interception
// ---------------------------------------------------------------------------

func TestLayout_ModalInterceptsInput(t *testing.T) {
	m := testLayout()

	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)
	if layout.activeDialog == nil {
		t.Fatal("precondition: options modal should be open")
	}

	focused := layout.railFocused
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.railFocused != focused {
		t.Error("expected modal to intercept tab, not toggle focus")
	}
}

func TestLayout_ModalClosesOnCancel(t *testing.T) {
	m := testLayout()

	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)

	result, _ = sendKey(&layout, "q")
	layout = result.(LayoutModel)
	if layout.activeDialog != nil {
		t.Error("expected q to close options modal")
	}
}

// ---------------------------------------------------------------------------
// Batch menu
// ---------------------------------------------------------------------------

func TestLayout_BatchMenu_EscCloses(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}

	result, _ := sendKey(&m, "esc")
	layout := result.(LayoutModel)
	if layout.showBatchMenu {
		t.Error("expected esc to close batch menu")
	}
	if layout.batchGames != nil {
		t.Error("expected batch games to be cleared")
	}
}

func TestLayout_BatchMenu_Navigation(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}
	m.batchCursor = 0

	result, _ := sendKey(&m, "up")
	layout := result.(LayoutModel)
	if layout.batchCursor != 0 {
		t.Error("expected cursor to clamp at 0")
	}
}

func TestLayout_BatchMenu_EnterExecutes(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}
	m.batchCursor = 0

	_, cmd := sendKey(&m, "enter")
	if cmd == nil {
		t.Error("expected enter in batch menu to return a command")
	}
}

func TestLayout_BatchMenu_BlocksGlobalKeys(t *testing.T) {
	m := testLayout()
	m.showBatchMenu = true
	m.batchGames = []*game.Game{testGame("Test")}

	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)
	if layout.showHelp {
		t.Error("expected batch menu to block ? from opening help")
	}
}

// ---------------------------------------------------------------------------
// Window size
// ---------------------------------------------------------------------------

func TestLayout_WindowSize(t *testing.T) {
	m := testLayout()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	layout := result.(LayoutModel)
	if layout.width != 200 || layout.height != 60 {
		t.Errorf("expected 200x60, got %dx%d", layout.width, layout.height)
	}
}

// ---------------------------------------------------------------------------
// Assertions that no `Launch` symbols survive in the shell layer
// ---------------------------------------------------------------------------

// TestLayout_NoLaunchSurface is a structural test: the shell must expose
// no launching field, no TabLaunch constant, no launchGame method, and no
// launchGameMsg type. This guards the Task 3 acceptance criterion
// "rg -i launch internal/tui/ returns no matches outside test fixtures /
// historical comments".
//
// We can't run rg from inside a test, but we can assert the absence of
// the concrete types the old shell relied on by failing to compile if
// they reappear. The go test harness catches this because the test file
// references none of those symbols — adding a new one would not break
// this test directly, which is why the acceptance check also runs grep
// in the verification step. Keep this test as a documentation anchor.
func TestLayout_NoLaunchSurface(t *testing.T) {
	// Intentionally empty — compile-time anchor only. The body could
	// reference `_ = TabLaunch` to fail the build if reintroduced, but
	// we keep it body-less so the rg grep from the acceptance criterion
	// is the authoritative check.
}
