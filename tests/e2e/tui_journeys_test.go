//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const visibleTimeout = 10 * time.Second

func startTUI(t *testing.T, environment *TestEnvironment, width, height int) *Session {
	t.Helper()
	session, err := NewSession("spela-tui-e2e", width, height, testBinaryPath, []string{"tui"}, environment.Env, environment.TerminalControlRuntimeDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("cleanup Terminal Control session: %v", err)
		}
	})
	if err := session.WaitForText("Library", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	return session
}

func requireScreen(t *testing.T, session *Session, contains []string, excludes ...string) string {
	t.Helper()
	screen, err := session.Capture()
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range contains {
		if !strings.Contains(screen, text) {
			t.Errorf("rendered screen does not contain %q:\n%s", text, screen)
		}
	}
	for _, text := range excludes {
		if strings.Contains(screen, text) {
			t.Errorf("rendered screen unexpectedly contains %q:\n%s", text, screen)
		}
	}
	return screen
}

func TestTUIJourneyStandard(t *testing.T) {
	environment, cleanup := SetupTestEnvironment(t)
	t.Cleanup(cleanup)
	session := startTUI(t, environment, 120, 40)
	requireScreen(t, session, []string{"◆ [1] Library", "▸ List", "Cyberpunk 2077"})

	if err := session.SendKeys("/", "Cyber", "enter"); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("Cyberpunk 2077", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	requireScreen(t, session, []string{"Cyberpunk 2077"}, "Witcher", "Elden Ring")

	if err := session.SendKeys("enter"); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("Overview", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	requireScreen(t, session, []string{"▸ Detail", "spela › Library › Cyberpunk 2077 › Overview", "App ID 1091500"})

	if err := session.SendKeys("]"); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("Profile", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	requireScreen(t, session, []string{"spela › Library › Cyberpunk 2077 › Profile", "[Profile]", "▸ Detail"})

	if err := session.SendKeys("enter", "backspace", "backspace", "backspace", "backspace", "false", "enter"); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("Unsaved", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	requireScreen(t, session, []string{"Unsaved", "▸ Detail"})
	if err := session.SendKeys("s"); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("Profile saved!", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("false", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	requireScreen(t, session, []string{"HDR", "false"}, "Unsaved")

	if err := session.SendKeys("?"); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("Keyboard shortcuts", visibleTimeout); err != nil {
		t.Fatal(err)
	}
	requireScreen(t, session, []string{"Keyboard shortcuts", "Esc", "q"})
	if err := session.SendKeys("escape"); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitForText("Cyberpunk 2077", visibleTimeout); err != nil {
		t.Fatal(err)
	}
}

func TestTUIJourneysConstrained(t *testing.T) {
	t.Run("success and DLL cancellation", func(t *testing.T) {
		environment, cleanup := SetupTestEnvironment(t)
		t.Cleanup(cleanup)
		session := startTUI(t, environment, 80, 24)
		requireScreen(t, session, []string{"Library", "Cyberpunk 2077", "▸ List"})

		if err := session.SendKeys("enter"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("Overview", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		if err := session.SendKeys("]"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("Profile", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		if err := session.SendKeys("]"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("› DLLs", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		requireScreen(t, session, []string{"Cyberpunk 2077", "DLLs", "▸ Detail"})
		before, err := os.ReadFile(filepath.Join(environment.TempDir, "games", "Cyberpunk 2077", "bin", "x64", "nvngx_dlss.dll"))
		if err != nil {
			t.Fatal(err)
		}
		if err := session.SendKeys("i"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("Select a supported DLL family", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		if err := session.SendKeys("enter"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("Select DLSS version", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		if err := session.SendKeys("enter"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("Confirm DLL install", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		requireScreen(t, session, []string{"Confirm DLL install", "Enter/Y confirm", "Esc/q cancel", "DLSS"})
		if err := session.SendKeys("escape"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("cancelled — no operation ran", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(filepath.Join(environment.TempDir, "games", "Cyberpunk 2077", "bin", "x64", "nvngx_dlss.dll"))
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Fatal("cancelled DLL journey changed the fixture")
		}
	})

	t.Run("empty search", func(t *testing.T) {
		environment, cleanup := SetupTestEnvironment(t)
		t.Cleanup(cleanup)
		session := startTUI(t, environment, 80, 24)
		if err := session.SendKeys("/", "missing title", "enter"); err != nil {
			t.Fatal(err)
		}
		if err := session.WaitForText("No games match", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		requireScreen(t, session, []string{"No games match", "Esc, then C to clear", "Select a game"})
	})

	t.Run("startup error", func(t *testing.T) {
		environment, cleanup := SetupTestEnvironment(t)
		t.Cleanup(cleanup)
		if err := os.WriteFile(filepath.Join(environment.DataHome, "spela", "games.yaml"), []byte("games: [invalid"), 0o644); err != nil {
			t.Fatal(err)
		}
		session, err := NewSession("spela-tui-e2e-error", 80, 24, testBinaryPath, []string{"tui"}, environment.Env, environment.TerminalControlRuntimeDirectory)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := session.Close(); err != nil {
				t.Errorf("cleanup Terminal Control session: %v", err)
			}
		})
		if err := session.WaitForText("failed", visibleTimeout); err != nil {
			t.Fatal(err)
		}
		requireScreen(t, session, []string{"failed"})
	})
}
