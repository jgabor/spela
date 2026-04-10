package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/game"
)

// ---------------------------------------------------------------------------
// Global key bindings — happy path
// ---------------------------------------------------------------------------

func TestLayout_HelpToggle(t *testing.T) {
	m := testLayout()

	// ? opens help
	result, _ := sendKey(&m, "?")
	layout := result.(LayoutModel)
	if !layout.showHelp {
		t.Error("expected help to be shown")
	}

	// ? closes help
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

	// Tab should NOT toggle focus while help is shown
	focused := layout.sidebarFocused
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.sidebarFocused != focused {
		t.Error("expected help to block tab from toggling focus")
	}
}

func TestLayout_TabTogglesFocus(t *testing.T) {
	m := testLayout()
	if !m.sidebarFocused {
		t.Fatal("precondition: sidebar should be focused initially")
	}

	result, _ := sendKey(&m, "tab")
	layout := result.(LayoutModel)
	if layout.sidebarFocused {
		t.Error("expected tab to switch focus to content")
	}

	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if !layout.sidebarFocused {
		t.Error("expected tab to switch focus back to sidebar")
	}
}

func TestLayout_CtrlCQuits(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "ctrl+c")
	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from ctrl+c, got %T", msg)
	}
}

func TestLayout_QFromSidebarQuits(t *testing.T) {
	m := testLayout()
	_, cmd := sendKey(&m, "q")
	msg := execCmd(cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from q on sidebar, got %T", msg)
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

	// Toggle back
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
	m := testLayout()
	m.sidebarFocused = false // start from content

	result, _ := sendKey(&m, "ctrl+f")
	layout := result.(LayoutModel)
	if !layout.sidebarFocused {
		t.Error("expected ctrl+f to focus sidebar")
	}
	if !layout.sidebar.search.Focused() {
		t.Error("expected ctrl+f to activate search input")
	}
}

func TestLayout_OptionsModal(t *testing.T) {
	m := testLayout()
	// o from sidebar opens options
	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)
	if layout.activeDialog == nil {
		t.Fatal("expected options modal to open")
	}

	// Esc closes the modal (via dialog routing)
	result, _ = sendKey(&layout, "esc")
	layout = result.(LayoutModel)
	if layout.activeDialog != nil {
		t.Error("expected esc to close options modal")
	}
}

func TestLayout_OptionsFromContentIgnored(t *testing.T) {
	m := testLayout()
	m.sidebarFocused = false

	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)
	if layout.activeDialog != nil {
		t.Error("expected o from content to be ignored")
	}
}

func TestLayout_RescanFromSidebar(t *testing.T) {
	m := testLayout()

	_, cmd := sendKey(&m, "r")
	// r from sidebar should return a command (batch of message + rescan)
	if cmd == nil {
		t.Error("expected rescan command from r on sidebar")
	}
}

func TestLayout_RescanFromContentIgnored(t *testing.T) {
	m := testLayout()
	m.sidebarFocused = false

	_, cmd := sendKey(&m, "r")
	// r from content should not trigger rescan (may produce header tick cmds)
	msg := execCmd(cmd)
	// Should not be a rescan-related message
	if _, ok := msg.(rescanGamesMsg); ok {
		t.Error("expected r from content not to trigger rescan")
	}
}

// ---------------------------------------------------------------------------
// Jump keys (2/3/4) — require a game selected
// ---------------------------------------------------------------------------

func TestLayout_JumpKeys_WithGame(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testLayoutWithGame(g)
	m.sidebarFocused = true

	tests := []struct {
		key     string
		wantTab ContentTab
	}{
		{"2", TabDLLs},
		{"3", TabProfile},
		{"4", TabLaunch},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			layout := m // copy
			layout.sidebarFocused = true
			result, _ := sendKey(&layout, tt.key)
			layout = result.(LayoutModel)
			if layout.sidebarFocused {
				t.Error("expected jump key to switch focus to content")
			}
			cm := layout.contentModel()
			if cm.activeTab != tt.wantTab {
				t.Errorf("expected tab %d, got %d", tt.wantTab, cm.activeTab)
			}
		})
	}
}

func TestLayout_JumpKey1_FocusesSidebar(t *testing.T) {
	m := testLayout()
	m.sidebarFocused = false
	m.densityMode = DensityStandard

	result, _ := sendKey(&m, "1")
	layout := result.(LayoutModel)
	if !layout.sidebarFocused {
		t.Error("expected 1 to focus sidebar")
	}
}

func TestLayout_JumpKey1_IgnoredInFocusedMode(t *testing.T) {
	m := testLayout()
	m.sidebarFocused = false
	m.densityMode = DensityFocused

	result, _ := sendKey(&m, "1")
	layout := result.(LayoutModel)
	if layout.sidebarFocused {
		t.Error("expected 1 to be ignored in focused density mode")
	}
}

func TestLayout_JumpKeys_NoGameIgnored(t *testing.T) {
	m := testLayout()
	m.sidebarFocused = true

	for _, key := range []string{"2", "3", "4"} {
		t.Run(key, func(t *testing.T) {
			result, _ := sendKey(&m, key)
			layout := result.(LayoutModel)
			if !layout.sidebarFocused {
				t.Errorf("expected %s to be ignored without game selected", key)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Navigation stack
// ---------------------------------------------------------------------------

func TestLayout_QFromContent_SingleEntry_ReturnsSidebar(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testLayoutWithGame(g)
	m.sidebarFocused = false

	if m.stack.Depth() != 1 {
		t.Fatalf("precondition: expected depth 1, got %d", m.stack.Depth())
	}

	result, _ := sendKey(&m, "q")
	layout := result.(LayoutModel)
	if !layout.sidebarFocused {
		t.Error("expected q from content (depth 1) to return to sidebar")
	}
}

func TestLayout_EscFromContent_SingleEntry_ReturnsSidebar(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testLayoutWithGame(g)
	m.sidebarFocused = false

	result, _ := sendKey(&m, "esc")
	layout := result.(LayoutModel)
	if !layout.sidebarFocused {
		t.Error("expected esc from content (depth 1) to return to sidebar")
	}
}

func TestLayout_NavStack_DeepPop(t *testing.T) {
	g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
	m := testLayoutWithGame(g)
	m.sidebarFocused = false

	// Push an additional entry onto the stack
	svc := testServices()
	extraContent := NewContent(m.styles, true, svc)
	m.stack.Push(newContentEntry(extraContent))

	if m.stack.Depth() != 2 {
		t.Fatalf("precondition: expected depth 2, got %d", m.stack.Depth())
	}

	result, _ := sendKey(&m, "q")
	layout := result.(LayoutModel)
	if layout.stack.Depth() != 1 {
		t.Errorf("expected q to pop stack to depth 1, got %d", layout.stack.Depth())
	}
	if layout.sidebarFocused {
		t.Error("expected to stay on content after popping (not at root)")
	}
}

func TestLayout_NavStack_RootCannotBePoped(t *testing.T) {
	m := testLayout()

	initialDepth := m.stack.Depth()
	m.stack.Pop()
	if m.stack.Depth() != initialDepth {
		t.Error("expected root entry to not be popable")
	}
}

// ---------------------------------------------------------------------------
// Modal interception
// ---------------------------------------------------------------------------

func TestLayout_ModalInterceptsInput(t *testing.T) {
	m := testLayout()

	// Open options modal
	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)
	if layout.activeDialog == nil {
		t.Fatal("precondition: options modal should be open")
	}

	// Tab should NOT toggle sidebar focus — modal intercepts it
	focused := layout.sidebarFocused
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	if layout.sidebarFocused != focused {
		t.Error("expected modal to intercept tab, not toggle focus")
	}
}

func TestLayout_ModalClosesOnCancel(t *testing.T) {
	m := testLayout()

	result, _ := sendKey(&m, "o")
	layout := result.(LayoutModel)

	// q closes the modal (OptionsModal handles q as cancel)
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

	// Can't go up from 0
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

	// ? should NOT open help while batch menu is shown
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
