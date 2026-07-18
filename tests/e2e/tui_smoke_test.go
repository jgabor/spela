//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTUISmoke_Shell(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-shell-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI shell did not initialize correctly: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	if !strings.Contains(screen, "spela") {
		t.Error("expected 'spela' breadcrumbs in the screen output")
	}
	if !strings.Contains(screen, "Library") {
		t.Error("expected 'Library' destination in the screen output")
	}
	if !strings.Contains(screen, "Navigate") {
		t.Error("expected three-zone 'Navigate' column header")
	}
}

func TestTUISmoke_OpensOptionsAndHelpOverlays(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-overlay-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.SendKeys("4"); err != nil {
		t.Fatalf("failed to press 'o': %v", err)
	}
	if err := session.WaitForText("Settings", 2*time.Second); err != nil {
		t.Fatalf("Settings destination did not open: %v", err)
	}

	if err := session.SendKeys("1"); err != nil {
		t.Fatalf("failed to press '1': %v", err)
	}
	if err := session.WaitForText("Cyberpunk 2077", 2*time.Second); err != nil {
		t.Fatalf("failed to return to main view after closing settings: %v", err)
	}

	if err := session.SendKeys("?"); err != nil {
		t.Fatalf("failed to press '?': %v", err)
	}
	if err := session.WaitForText("Keyboard shortcuts", 2*time.Second); err != nil {
		if err2 := session.WaitForText("Help", 1*time.Second); err2 != nil {
			t.Fatalf("Help modal did not open: %v (alt error: %v)", err, err2)
		}
	}

	if err := session.SendKeys("escape"); err != nil {
		t.Fatalf("failed to press '1': %v", err)
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
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	for _, gameName := range []string{"Cyberpunk 2077", "Witcher", "Elden Ring"} {
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
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.FocusContext(); err != nil {
		t.Fatalf("failed to focus context: %v", err)
	}
	if err := session.SendKeys("/", "Cyber", "enter"); err != nil {
		t.Fatalf("failed to perform search: %v", err)
	}

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
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.FocusContext(); err != nil {
		t.Fatalf("failed to focus context: %v", err)
	}
	if err := session.SendKeys("/", "nonexistent game", "enter"); err != nil {
		t.Fatalf("failed to type search: %v", err)
	}

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
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.SelectGame("Cyberpunk"); err != nil {
		t.Fatalf("failed to select game: %v", err)
	}
	if err := session.SelectAspectOverview(); err != nil {
		t.Fatalf("failed to select overview aspect: %v", err)
	}

	if err := session.WaitForText("App ID:      1091500", 2*time.Second); err != nil {
		t.Fatalf("Cyberpunk 2077 details page did not load: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

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
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.SelectDefaultProfile(); err != nil {
		t.Fatalf("failed to select default profile: %v", err)
	}

	if err := session.WaitForBreadcrumb("All games", 2*time.Second); err != nil {
		t.Fatalf("default profile scope did not load: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	if !strings.Contains(screen, "HDR") {
		t.Error("expected default profile field 'HDR' to be visible")
	}
}

func TestTUISmoke_DLLAspect_ShowsVersionTable(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-dll-aspect-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.SelectGame("Cyberpunk"); err != nil {
		t.Fatalf("failed to select game: %v", err)
	}
	if err := session.SelectAspectDLLs(); err != nil {
		t.Fatalf("failed to select DLL aspect: %v", err)
	}
	if err := session.FocusContent(); err != nil {
		t.Fatalf("failed to focus content: %v", err)
	}

	if err := session.WaitForBreadcrumb("DLLs", 2*time.Second); err != nil {
		t.Fatalf("DLL aspect breadcrumb not shown: %v", err)
	}

	screen, err := session.Capture()
	if err != nil {
		t.Fatalf("failed to capture screen: %v", err)
	}

	for _, header := range []string{"DLSS", "3.7.0"} {
		if !strings.Contains(screen, header) {
			t.Errorf("expected %q in DLL aspect view", header)
		}
	}
}

func TestTUISmoke_DLLUpdate_WhenStale(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-dll-update-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.SelectGame("Cyberpunk"); err != nil {
		t.Fatalf("failed to select game: %v", err)
	}
	if err := session.SelectAspectDLLs(); err != nil {
		t.Fatalf("failed to select DLL aspect: %v", err)
	}
	if err := session.FocusContent(); err != nil {
		t.Fatalf("failed to focus content: %v", err)
	}

	// Wait for async manifest check to mark updates available.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		screen, err := session.Capture()
		if err != nil {
			t.Fatalf("capture failed: %v", err)
		}
		if strings.Contains(screen, "u:update") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := session.SendKeys("u"); err != nil {
		t.Fatalf("failed to press u: %v", err)
	}
	if err := session.WaitForText("Update DLLs?", 2*time.Second); err != nil {
		t.Fatalf("expected destructive confirmation prompt: %v", err)
	}
}

func TestTUISmoke_DLLUpdate_NoOpFeedback(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	writeCurrentManifest(t, te)

	session, err := NewSession("spela-dll-noop-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.SelectGame("Cyberpunk"); err != nil {
		t.Fatalf("failed to select game: %v", err)
	}
	if err := session.SelectAspectDLLs(); err != nil {
		t.Fatalf("failed to select DLL aspect: %v", err)
	}
	if err := session.FocusContent(); err != nil {
		t.Fatalf("failed to focus content: %v", err)
	}

	// Allow manifest check to complete with no stale DLLs.
	time.Sleep(500 * time.Millisecond)

	if err := session.SendKeys("u"); err != nil {
		t.Fatalf("failed to press u: %v", err)
	}
	if err := session.WaitForText("DLLs already up to date", 2*time.Second); err != nil {
		t.Fatalf("expected no-op feedback message: %v", err)
	}
}

func writeCurrentManifest(t *testing.T, te *TestEnvironment) {
	t.Helper()
	manifestPath := filepath.Join(te.CacheHome, "spela", "manifest.json")
	updatedAt := time.Now().Format(time.RFC3339)
	manifestJSON := fmt.Sprintf(`{
  "version": "1.0",
  "updated_at": "%s",
  "repository": "jgabor/spela",
  "dlls": {
    "dlss": [
      {
        "version": "3.7.0",
        "filename": "nvngx_dlss_3.7.0.dll",
        "url": "http://localhost:12345/dlls/nvngx_dlss_3.7.0.dll",
        "sha256": "abcdef1234567890",
        "size": 1024,
        "release_date": "2026-05-22T21:00:00Z"
      }
    ],
    "dlssg": [
      {
        "version": "3.7.0",
        "filename": "nvngx_dlssg_3.7.0.dll",
        "url": "http://localhost:12345/dlls/nvngx_dlssg_3.7.0.dll",
        "sha256": "abcdef1234567890",
        "size": 1024,
        "release_date": "2026-05-22T21:00:00Z"
      }
    ]
  }
}`, updatedAt)
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0o644); err != nil {
		t.Fatalf("failed to write matching manifest: %v", err)
	}
}
