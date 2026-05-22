package e2e

import (
	"strings"
	"testing"
	"time"
)

func TestTUISmoke_Shell(t *testing.T) {
	// Setup isolated test environment
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	// Start RMUX session running 'spela tui'
	session, err := NewSession("spela-shell-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() {
		_ = session.Close()
	}()

	// Verify that the main UI has initialized and shows a mock game
	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI shell did not initialize correctly: %v", err)
	}

	// Verify breadcrumbs in status bar
	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	if !strings.Contains(screen, "spela") {
		t.Error("expected 'spela' logo/breadcrumbs in the screen output")
	}
	if !strings.Contains(screen, "Games · resource") {
		t.Error("expected 'Games · resource' header to be visible")
	}
}

func TestTUISmoke_OpensOptionsAndHelpOverlays(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-overlay-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() {
		_ = session.Close()
	}()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	// 1. Open Options Modal using key 'o'
	if err := session.SendKeys("o"); err != nil {
		t.Fatalf("failed to press 'o': %v", err)
	}
	if err := session.WaitForText("Options", 2*time.Second); err != nil {
		t.Fatalf("Options modal did not open: %v", err)
	}

	// 2. Close Options Modal using key 'escape'
	if err := session.SendKeys("escape"); err != nil {
		t.Fatalf("failed to press 'escape': %v", err)
	}
	if err := session.WaitForText("Cyberpunk 2077", 2*time.Second); err != nil {
		t.Fatalf("failed to return to main view after closing options: %v", err)
	}

	// 3. Open Help Modal using key '?'
	if err := session.SendKeys("?"); err != nil {
		t.Fatalf("failed to press '?': %v", err)
	}
	if err := session.WaitForText("Keyboard shortcuts", 2*time.Second); err != nil {
		// Fallback to "Help" or navigation texts if the title differs slightly
		if err2 := session.WaitForText("Help", 1*time.Second); err2 != nil {
			t.Fatalf("Help modal did not open: %v (alt error: %v)", err, err2)
		}
	}

	// 4. Close Help Modal using key 'escape'
	if err := session.SendKeys("escape"); err != nil {
		t.Fatalf("failed to press 'escape': %v", err)
	}
	if err := session.WaitForText("Cyberpunk 2077", 2*time.Second); err != nil {
		t.Fatalf("failed to return to main view after closing help: %v", err)
	}
}

func TestTUISmoke_GameList(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-list-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() {
		_ = session.Close()
	}()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	// Check if all mock games appear in the sidebar list
	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	expectedGames := []string{"Cyberpunk 2077", "The Witcher 3", "Elden Ring"}
	for _, gameName := range expectedGames {
		if !strings.Contains(screen, gameName) {
			t.Errorf("expected game %q to be in sidebar list", gameName)
		}
	}
}

func TestTUISmoke_FiltersGamesBySearch(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-search-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() {
		_ = session.Close()
	}()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	// Focus sidebar using 'tab' then search bar using '/' and type 'Cyber'
	if err := session.SendKeys("tab", "/", "Cyber", "enter"); err != nil {
		t.Fatalf("failed to perform search: %v", err)
	}

	// Let the list update and verify Witcher and Elden Ring are filtered out
	if err := session.WaitForText("Cyberpunk 2077", 2*time.Second); err != nil {
		t.Fatalf("Cyberpunk 2077 did not remain visible: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	if strings.Contains(screen, "The Witcher 3: Wild Hunt") {
		t.Error("The Witcher 3: Wild Hunt should have been filtered out")
	}
	if strings.Contains(screen, "Elden Ring") {
		t.Error("Elden Ring should have been filtered out")
	}
}

func TestTUISmoke_ShowsEmptyState(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-empty-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() {
		_ = session.Close()
	}()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	// Focus sidebar using 'tab' then search for non-existent game
	if err := session.SendKeys("tab", "/", "nonexistent game", "enter"); err != nil {
		t.Fatalf("failed to type search: %v", err)
	}

	// Expect the empty state message
	if err := session.WaitForText("No games found", 2*time.Second); err != nil {
		t.Fatalf("empty state message 'No games found' not shown: %v", err)
	}
}

func TestTUISmoke_GameDetail(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-detail-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() {
		_ = session.Close()
	}()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	// Focus sidebar using 'tab' then confirm selection of first game (Cyberpunk 2077) by hitting enter
	if err := session.SendKeys("tab", "enter"); err != nil {
		t.Fatalf("failed to press enter to view details: %v", err)
	}

	// Verify the game details header/content
	if err := session.WaitForText("App ID:      1091500", 2*time.Second); err != nil {
		t.Fatalf("Cyberpunk 2077 details page did not load: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	// Check for metadata representation
	if !strings.Contains(screen, "1091500") {
		t.Error("expected App ID '1091500' to be visible in game details")
	}
	if !strings.Contains(screen, "3.7.0") {
		t.Error("expected DLL version '3.7.0' to be visible")
	}
}

func TestTUISmoke_DefaultProfile(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-defaults-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() {
		_ = session.Close()
	}()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	// Press hotkey '3' to switch to the Defaults rail item
	if err := session.SendKeys("3"); err != nil {
		t.Fatalf("failed to press '3': %v", err)
	}

	// Verify breadcrumbs and pane title reflect Defaults
	if err := session.WaitForText("Defaults · resource", 2*time.Second); err != nil {
		t.Fatalf("Defaults resource view did not load: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	// Check that we see default settings rows (like SR mode, Wayland, HDR)
	if !strings.Contains(screen, "SR mode") {
		t.Error("expected default profile field 'SR mode' to be visible")
	}
	if !strings.Contains(screen, "HDR") {
		t.Error("expected default profile field 'HDR' to be visible")
	}
}
