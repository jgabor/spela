package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

// ---------------------------------------------------------------------------
// Fake services
// ---------------------------------------------------------------------------

// testServices returns a Services struct with no-op fakes for all dependencies.
// Individual fields can be overridden after creation.
func testServices() *Services {
	return &Services{
		LoadConfig: func() (*config.Config, error) {
			return config.Default(), nil
		},
		LoadProfile: func(appID uint64) (*profile.Profile, error) {
			return nil, nil
		},
		LoadDefaultProfile: func() (*profile.Profile, error) {
			return &profile.Profile{}, nil
		},
		ProfileExists: func(appID uint64) bool {
			return false
		},
		BackupExists: func(appID uint64) bool {
			return false
		},
	}
}

// ---------------------------------------------------------------------------
// Test data factories
// ---------------------------------------------------------------------------

// testGame creates a game with the given name and optional DLLs.
func testGame(name string, dlls ...game.DetectedDLL) *game.Game {
	return &game.Game{
		AppID:      1091500,
		Name:       name,
		InstallDir: "/tmp/test/steamapps/common/" + name,
		DLLs:       dlls,
	}
}

// testDLL creates a DetectedDLL of the given type with a version.
func testDLL(dllType game.DLLType, version string) game.DetectedDLL {
	names := map[game.DLLType]string{
		game.DLLTypeDLSS:  "nvngx_dlss.dll",
		game.DLLTypeDLSSG: "nvngx_dlssg.dll",
		game.DLLTypeDLSSD: "nvngx_dlssd.dll",
	}
	name := names[dllType]
	if name == "" {
		name = string(dllType) + ".dll"
	}
	return game.DetectedDLL{
		Path:    "/tmp/test/" + name,
		Name:    name,
		Type:    dllType,
		Version: version,
	}
}

// testDatabase creates a Database with the given games.
func testDatabase(games ...*game.Game) *game.Database {
	db := &game.Database{
		Games: make(map[uint64]*game.Game, len(games)),
	}
	for _, g := range games {
		db.Games[g.AppID] = g
	}
	return db
}

// ---------------------------------------------------------------------------
// Model factories
// ---------------------------------------------------------------------------

// testLayout creates a LayoutModel with a default config and the given games.
// The model has a standard terminal size (120x40) and sidebar focused.
func testLayout(games ...*game.Game) LayoutModel {
	svc := testServices()
	db := testDatabase(games...)
	m := NewLayout(db, svc)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return sized.(LayoutModel)
}

// testLayoutWithGame creates a LayoutModel with one game already selected.
// Returns the layout after simulating game selection from the sidebar.
func testLayoutWithGame(g *game.Game) LayoutModel {
	svc := testServices()
	svc.LoadProfile = func(appID uint64) (*profile.Profile, error) {
		return &profile.Profile{}, nil
	}

	db := testDatabase(g)
	m := NewLayout(db, svc)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	layout := sized.(LayoutModel)

	// Simulate game selection: sidebar dispatches gameSelectedMsg,
	// then gameConfirmedMsg to switch focus to content.
	selected, _ := layout.Update(gameSelectedMsg{game: g})
	layout = selected.(LayoutModel)
	confirmed, _ := layout.Update(gameConfirmedMsg{game: g})
	layout = confirmed.(LayoutModel)

	return layout
}

// testContent creates a ContentModel with fake services and optional game.
func testContent(g *game.Game) ContentModel {
	svc := testServices()
	svc.LoadProfile = func(appID uint64) (*profile.Profile, error) {
		return &profile.Profile{}, nil
	}
	styles := NewStyles(DefaultTheme, true)
	m := NewContent(styles, true, svc)
	if g != nil {
		m = m.SetGame(g)
	}
	return m
}

// testSidebar creates a SidebarModel with the given games.
func testSidebar(games ...*game.Game) SidebarModel {
	svc := testServices()
	styles := NewStyles(DefaultTheme, true)
	m, _ := NewSidebar(games, styles, svc)
	return m
}

// ---------------------------------------------------------------------------
// Key sequence helpers
// ---------------------------------------------------------------------------

// sendKey sends a single key press through a model's Update and returns the result.
func sendKey(m tea.Model, key string) (tea.Model, tea.Cmd) {
	return m.Update(keyMsg(key))
}

// sendKeys sends multiple key presses sequentially and returns the final model
// and all accumulated commands.
func sendKeys(m tea.Model, keys ...string) (tea.Model, []tea.Cmd) {
	var cmds []tea.Cmd
	for _, key := range keys {
		var cmd tea.Cmd
		m, cmd = m.Update(keyMsg(key))
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, cmds
}

// keyMsg builds a tea.KeyPressMsg that will produce the given string from
// its String() method. This mirrors how bubbletea v2 represents key events:
//
//   - Printable single characters (e.g. "a", "?", "1"): Code = the rune,
//     Text = the character.
//   - Special keys (e.g. "enter", "tab", "f5"): Code = the constant,
//     Text = "".
//   - Modifier combos (e.g. "ctrl+c", "ctrl+f"): Code = the base rune,
//     Mod = the modifier flag(s), Text = "".
func keyMsg(key string) tea.KeyPressMsg {
	var mod tea.KeyMod
	remaining := key
	for {
		if strings.HasPrefix(remaining, "ctrl+") {
			mod |= tea.ModCtrl
			remaining = remaining[len("ctrl+"):]
		} else if strings.HasPrefix(remaining, "alt+") {
			mod |= tea.ModAlt
			remaining = remaining[len("alt+"):]
		} else if strings.HasPrefix(remaining, "shift+") {
			mod |= tea.ModShift
			remaining = remaining[len("shift+"):]
		} else {
			break
		}
	}

	if code, ok := specialKeys[remaining]; ok {
		return tea.KeyPressMsg{Code: code, Mod: mod}
	}

	if r, size := utf8.DecodeRuneInString(remaining); size == len(remaining) && size > 0 {
		text := remaining
		if mod != 0 {
			text = ""
		}
		return tea.KeyPressMsg{Code: r, Text: text, Mod: mod}
	}

	return tea.KeyPressMsg{Code: tea.KeyExtended, Text: remaining, Mod: mod}
}

var specialKeys = map[string]rune{
	"enter":     tea.KeyEnter,
	"esc":       tea.KeyEscape,
	"escape":    tea.KeyEscape,
	"tab":       tea.KeyTab,
	"space":     tea.KeySpace,
	"backspace": tea.KeyBackspace,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
	"home":      tea.KeyHome,
	"end":       tea.KeyEnd,
	"pgup":      tea.KeyPgUp,
	"pgdown":    tea.KeyPgDown,
	"delete":    tea.KeyDelete,
	"insert":    tea.KeyInsert,
	"f1":        tea.KeyF1,
	"f2":        tea.KeyF2,
	"f3":        tea.KeyF3,
	"f4":        tea.KeyF4,
	"f5":        tea.KeyF5,
	"f6":        tea.KeyF6,
	"f7":        tea.KeyF7,
	"f8":        tea.KeyF8,
	"f9":        tea.KeyF9,
	"f10":       tea.KeyF10,
	"f11":       tea.KeyF11,
	"f12":       tea.KeyF12,
}

// ---------------------------------------------------------------------------
// Cmd execution helper
// ---------------------------------------------------------------------------

// execCmd executes a tea.Cmd and returns the resulting message.
// Returns nil if the cmd is nil.
func execCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// ---------------------------------------------------------------------------
// Smoke tests
// ---------------------------------------------------------------------------

func TestFactories_Smoke(t *testing.T) {
	t.Run("testLayout creates without panicking", func(t *testing.T) {
		m := testLayout(testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10")))
		if m.width != 120 {
			t.Errorf("expected width 120, got %d", m.width)
		}
		if m.height != 40 {
			t.Errorf("expected height 40, got %d", m.height)
		}
		if !m.sidebarFocused {
			t.Error("expected sidebar to be focused by default")
		}
	})

	t.Run("testLayoutWithGame selects the game", func(t *testing.T) {
		g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
		withGame := testLayoutWithGame(g)
		cm := withGame.contentModel()
		if cm == nil || cm.game == nil {
			t.Fatal("expected game to be selected in content")
		}
		if cm.game.Name != "Cyberpunk 2077" {
			t.Errorf("expected Cyberpunk 2077, got %s", cm.game.Name)
		}
		if withGame.sidebarFocused {
			t.Error("expected content to be focused after game confirmation")
		}
	})

	t.Run("sendKey toggles help", func(t *testing.T) {
		m := testLayout(testGame("Cyberpunk 2077"))
		result, _ := sendKey(&m, "?")
		layout := result.(LayoutModel)
		if !layout.showHelp {
			t.Error("expected help to be shown after pressing ?")
		}
	})

	t.Run("keyMsg produces correct String output", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"a", "a"},
			{"?", "?"},
			{"enter", "enter"},
			{"esc", "esc"},
			{"tab", "tab"},
			{"space", "space"},
			{"f5", "f5"},
			{"f11", "f11"},
			{"up", "up"},
			{"down", "down"},
			{"ctrl+c", "ctrl+c"},
			{"ctrl+f", "ctrl+f"},
		}
		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				msg := keyMsg(tt.input)
				got := msg.String()
				if got != tt.want {
					t.Errorf("keyMsg(%q).String() = %q, want %q", tt.input, got, tt.want)
				}
			})
		}
	})

	t.Run("testContent with game", func(t *testing.T) {
		g := testGame("Cyberpunk 2077", testDLL(game.DLLTypeDLSS, "3.8.10"))
		content := testContent(g)
		if content.game == nil {
			t.Fatal("expected game in content")
		}
		if content.game.Name != "Cyberpunk 2077" {
			t.Errorf("expected Cyberpunk 2077, got %s", content.game.Name)
		}
	})

	t.Run("testContent without game", func(t *testing.T) {
		content := testContent(nil)
		if content.game != nil {
			t.Error("expected nil game when none provided")
		}
	})

	t.Run("testSidebar with games", func(t *testing.T) {
		g := testGame("Cyberpunk 2077")
		sidebar := testSidebar(g)
		if len(sidebar.games) != 1 {
			t.Errorf("expected 1 game, got %d", len(sidebar.games))
		}
	})

	t.Run("testDatabase builds map correctly", func(t *testing.T) {
		g := testGame("Cyberpunk 2077")
		db := testDatabase(g)
		if len(db.Games) != 1 {
			t.Errorf("expected 1 game in database, got %d", len(db.Games))
		}
		if db.Games[1091500] == nil {
			t.Error("expected game at AppID 1091500")
		}
	})

	t.Run("sendKeys accumulates commands", func(t *testing.T) {
		m := testLayout(testGame("Cyberpunk 2077"))
		result, cmds := sendKeys(&m, "?", "?")
		layout := result.(LayoutModel)
		// Help toggled on then off
		if layout.showHelp {
			t.Error("expected help to be off after two presses")
		}
		_ = cmds
	})

	t.Run("execCmd handles nil", func(t *testing.T) {
		if execCmd(nil) != nil {
			t.Error("expected nil from execCmd(nil)")
		}
	})
}
