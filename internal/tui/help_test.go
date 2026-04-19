package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/game"
)

// ---------------------------------------------------------------------------
// ContextKeys — the first argument now means railFocused (renamed from
// sidebarFocused during the Task 3 shell rewrite).
// ---------------------------------------------------------------------------

func TestContextKeys_RailFocused(t *testing.T) {
	keys := ContextKeys(true, false, false, nil, true)

	assertHasKey(t, keys, "1-4", "resource")
	assertHasKey(t, keys, "enter", "activate")
	assertHasKey(t, keys, "tab", "pane")
	assertHasGlobalKeys(t, keys)
}

func TestContextKeys_SearchFocused(t *testing.T) {
	keys := ContextKeys(true, true, false, nil, true)

	assertHasKey(t, keys, "type", "filter")
	assertHasKey(t, keys, "enter", "done")
	assertHasKey(t, keys, "esc", "cancel")
	assertHasGlobalKeys(t, keys)

	// Should NOT have rail-specific keys while search is focused.
	assertMissingKey(t, keys, "1-4")
}

func TestContextKeys_SelectMode(t *testing.T) {
	keys := ContextKeys(true, false, true, nil, true)

	assertHasKey(t, keys, "space", "toggle")
	assertHasKey(t, keys, "a", "all")
	assertHasKey(t, keys, "A", "none")
	assertHasKey(t, keys, "enter", "batch")
	assertHasKey(t, keys, "esc", "exit")
	assertHasGlobalKeys(t, keys)
}

func TestContextKeys_ResourcePaneFocusedWithGame(t *testing.T) {
	content := &ContentModel{
		game:       &game.Game{AppID: 1091500},
		hasBackup:  true,
		hasUpdates: true,
	}
	keys := ContextKeys(false, false, false, content, true)

	// Launch is gone.
	assertMissingKey(t, keys, "L")

	assertHasKey(t, keys, "i", "install")
	assertHasKey(t, keys, "u", "update")
	// Task 5 displaced Restore from "R" (now reset-all profile) to "ctrl+shift+r".
	assertHasKey(t, keys, "ctrl+shift+r", "restore")
	assertHasKey(t, keys, "r", "reset-field")
	assertHasKey(t, keys, "R", "reset-all")
	assertHasKey(t, keys, "p", "pin")
	assertHasKey(t, keys, "tab", "rail")
	assertHasKey(t, keys, "ctrl+r", "rescan")
	// The profile filter moved from `p` to `P` during the keymap audit.
	assertHasKey(t, keys, "P", "profile")
	assertHasGlobalKeys(t, keys)
}

func TestContextKeys_ResourcePaneFocusedNoGame(t *testing.T) {
	content := &ContentModel{}
	keys := ContextKeys(false, false, false, content, true)

	assertHasKey(t, keys, "↑↓", "navigate")
	assertHasKey(t, keys, "tab", "rail")
	assertMissingKey(t, keys, "L")
	assertMissingKey(t, keys, "u")
	assertMissingKey(t, keys, "ctrl+shift+r")
}

func TestContextKeys_DisabledRestoreNoBackup(t *testing.T) {
	content := &ContentModel{
		game:       &game.Game{AppID: 1091500},
		hasBackup:  false,
		hasUpdates: true,
	}
	keys := ContextKeys(false, false, false, content, true)

	// Task 5 displaced restore from "R" to "ctrl+shift+r".
	k := findKey(keys, "ctrl+shift+r")
	if k == nil {
		t.Fatal("expected restore key to be present")
	}
	if k.Enabled {
		t.Error("restore should be disabled when no backup exists")
	}
	if k.Reason != "no backup" {
		t.Errorf("restore reason should be 'no backup', got %q", k.Reason)
	}
}

func TestContextKeys_DisabledUpdateNoUpdates(t *testing.T) {
	content := &ContentModel{
		game:       &game.Game{AppID: 1091500},
		hasBackup:  true,
		hasUpdates: false,
	}
	keys := ContextKeys(false, false, false, content, true)

	k := findKey(keys, "u")
	if k == nil {
		t.Fatal("expected update key to be present")
	}
	if k.Enabled {
		t.Error("update should be disabled when no updates available")
	}
	if k.Reason != "up to date" {
		t.Errorf("update reason should be 'up to date', got %q", k.Reason)
	}
}

func TestContextKeys_DisabledUpdateBusy(t *testing.T) {
	content := &ContentModel{
		game:         &game.Game{AppID: 1091500},
		hasBackup:    true,
		hasUpdates:   true,
		dllOperating: true,
	}
	keys := ContextKeys(false, false, false, content, true)

	k := findKey(keys, "u")
	if k == nil {
		t.Fatal("expected update key to be present")
	}
	if k.Enabled {
		t.Error("update should be disabled when DLL operating")
	}
	if k.Reason != "busy" {
		t.Errorf("update reason should be 'busy', got %q", k.Reason)
	}
}

func TestContextKeys_HintsDisabled(t *testing.T) {
	keys := ContextKeys(true, false, false, nil, false)

	if len(keys) != len(globalKeys) {
		t.Errorf("expected %d keys (global only), got %d", len(globalKeys), len(keys))
	}
	assertHasGlobalKeys(t, keys)
}

func TestReasonForUpdate(t *testing.T) {
	tests := []struct {
		name    string
		content *ContentModel
		want    string
	}{
		{"busy", &ContentModel{dllOperating: true, hasUpdates: true, hasBackup: true}, "busy"},
		{"up to date", &ContentModel{hasUpdates: false, hasBackup: true}, "up to date"},
		{"no backup", &ContentModel{hasUpdates: true, hasBackup: false}, "no backup"},
		{"no reason when all good", &ContentModel{hasUpdates: true, hasBackup: true}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reasonForUpdate(tt.content)
			if got != tt.want {
				t.Errorf("reasonForUpdate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RenderContextBar
// ---------------------------------------------------------------------------

func TestRenderContextBar_EmptyKeys(t *testing.T) {
	theme := &DefaultTheme
	result := RenderContextBar(nil, 80, theme)
	if result != "" {
		t.Errorf("expected empty string for nil keys, got %q", result)
	}
}

func TestRenderContextBar_ZeroWidth(t *testing.T) {
	theme := &DefaultTheme
	keys := []ContextKey{{Key: "q", Action: "quit", Enabled: true}}
	result := RenderContextBar(keys, 0, theme)
	if result != "" {
		t.Errorf("expected empty string for zero width, got %q", result)
	}
}

func TestRenderContextBar_ContainsGlobalKeys(t *testing.T) {
	theme := &DefaultTheme
	keys := ContextKeys(true, false, false, nil, true)
	result := RenderContextBar(keys, 200, theme)

	for _, gk := range globalKeys {
		if !strings.Contains(result, gk.Key) {
			t.Errorf("rendered bar missing global key %q", gk.Key)
		}
	}
}

func TestRenderContextBar_DisabledKeyShowsReason(t *testing.T) {
	theme := &DefaultTheme
	keys := []ContextKey{
		{Key: "R", Action: "restore", Enabled: false, Reason: "no backup"},
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}
	result := RenderContextBar(keys, 200, theme)

	if !strings.Contains(result, "no backup") {
		t.Error("rendered bar should contain disabled reason 'no backup'")
	}
}

func TestRenderContextBar_Truncation(t *testing.T) {
	theme := &DefaultTheme
	keys := []ContextKey{
		{Key: "↑↓", Action: "navigate", Enabled: true},
		{Key: "/", Action: "search", Enabled: true},
		{Key: "d", Action: "DLLs", Enabled: true},
		{Key: "P", Action: "profile", Enabled: true},
		{Key: "s", Action: "sort", Enabled: true},
		{Key: "ctrl+r", Action: "rescan", Enabled: true},
		{Key: "enter", Action: "select", Enabled: true},
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}

	result := RenderContextBar(keys, 40, theme)
	if !strings.Contains(result, "...") {
		t.Error("narrow bar should contain ellipsis truncation")
	}

	if !strings.Contains(result, "quit") {
		t.Error("global keys should be present even when truncated")
	}
}

func TestRenderContextBar_WideEnoughNoTruncation(t *testing.T) {
	theme := &DefaultTheme
	keys := []ContextKey{
		{Key: "a", Action: "act", Enabled: true},
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}

	result := RenderContextBar(keys, 200, theme)
	if strings.Contains(result, "...") {
		t.Error("wide bar should not contain ellipsis")
	}
}

func TestRenderContextBar_EnabledVsDisabledStyling(t *testing.T) {
	theme := &DefaultTheme
	enabledKey := ContextKey{Key: "u", Action: "update", Enabled: true}
	disabledKey := ContextKey{Key: "u", Action: "update", Enabled: false, Reason: "busy"}

	keysEnabled := []ContextKey{
		enabledKey,
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}
	keysDisabled := []ContextKey{
		disabledKey,
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}

	resultEnabled := RenderContextBar(keysEnabled, 200, theme)
	resultDisabled := RenderContextBar(keysDisabled, 200, theme)

	if resultEnabled == resultDisabled {
		t.Error("enabled and disabled keys should have different rendered output")
	}

	enabledWidth := lipgloss.Width(resultEnabled)
	disabledWidth := lipgloss.Width(resultDisabled)
	if disabledWidth <= enabledWidth {
		t.Errorf("disabled bar (width=%d) should be wider than enabled (width=%d) due to reason text",
			disabledWidth, enabledWidth)
	}
}

func TestRenderContextBar_GlobalKeysOnly(t *testing.T) {
	theme := &DefaultTheme
	keys := ContextKeys(true, false, false, nil, false)
	result := RenderContextBar(keys, 200, theme)

	if result == "" {
		t.Error("global-only bar should not be empty")
	}
	if !strings.Contains(result, "help") {
		t.Error("bar should contain 'help'")
	}
	if !strings.Contains(result, "quit") {
		t.Error("bar should contain 'quit'")
	}
}

// ---------------------------------------------------------------------------
// Help model — Task 3 keymap documentation
// ---------------------------------------------------------------------------

// TestHelp_DocumentsDisplacedBindings verifies the help screen explicitly
// calls out the keymap audit moves (r → ctrl+r and p → P) so the user can
// discover the displacement.
func TestHelp_DocumentsDisplacedBindings(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	h := NewHelp(styles)

	out := strings.ToLower(h.View())
	for _, want := range []string{"displaced", "ctrl+r", "rescan", "pin"} {
		if !strings.Contains(out, want) {
			t.Errorf("help view missing %q — keymap displacement must be documented:\n%s", want, out)
		}
	}
}

// TestHelp_NoLaunchBinding guards that the help screen does not advertise a
// Launch binding now that the Launch tab is removed.
func TestHelp_NoLaunchBinding(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	h := NewHelp(styles)
	out := h.View()
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		low := strings.ToLower(line)
		if strings.Contains(low, "launch game") {
			t.Errorf("help line mentions launch game — must be removed:\n%s", line)
		}
	}
}

// --- test helpers ---

func findKey(keys []ContextKey, key string) *ContextKey {
	for i := range keys {
		if keys[i].Key == key {
			return &keys[i]
		}
	}
	return nil
}

func assertHasKey(t *testing.T, keys []ContextKey, key, action string) {
	t.Helper()
	for _, k := range keys {
		if k.Key == key && k.Action == action {
			return
		}
	}
	t.Errorf("expected key %q with action %q not found in keys", key, action)
}

func assertMissingKey(t *testing.T, keys []ContextKey, key string) {
	t.Helper()
	for _, k := range keys {
		if k.Key == key {
			t.Errorf("key %q should not be present, but found with action %q", key, k.Action)
			return
		}
	}
}

func assertHasGlobalKeys(t *testing.T, keys []ContextKey) {
	t.Helper()
	for _, gk := range globalKeys {
		assertHasKey(t, keys, gk.Key, gk.Action)
	}
}
