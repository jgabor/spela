package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/game"
)

func TestContextKeys_SidebarFocused(t *testing.T) {
	keys := ContextKeys(FocusSidebar, false, false, nil, true)

	assertHasKey(t, keys, "/", "search")
	assertHasKey(t, keys, "d", "DLLs")
	assertHasKey(t, keys, "s", "sort")
	assertHasKey(t, keys, "r", "rescan")
	assertHasGlobalKeys(t, keys)
}

func TestContextKeys_SearchFocused(t *testing.T) {
	keys := ContextKeys(FocusSidebar, true, false, nil, true)

	assertHasKey(t, keys, "type", "filter")
	assertHasKey(t, keys, "enter", "done")
	assertHasKey(t, keys, "esc", "cancel")
	assertHasGlobalKeys(t, keys)

	// Should NOT have sidebar-specific keys.
	assertMissingKey(t, keys, "/")
	assertMissingKey(t, keys, "d")
}

func TestContextKeys_SelectMode(t *testing.T) {
	keys := ContextKeys(FocusSidebar, false, true, nil, true)

	assertHasKey(t, keys, "space", "toggle")
	assertHasKey(t, keys, "a", "all")
	assertHasKey(t, keys, "A", "none")
	assertHasKey(t, keys, "enter", "batch")
	assertHasKey(t, keys, "esc", "exit")
	assertHasGlobalKeys(t, keys)
}

func TestContextKeys_ContentFocusedWithGame(t *testing.T) {
	content := &ContentModel{
		game:       &game.Game{AppID: 1091500},
		hasBackup:  true,
		hasUpdates: true,
	}
	keys := ContextKeys(FocusContent, false, false, content, true)

	assertHasKey(t, keys, "L", "launch")
	assertHasKey(t, keys, "i", "install")
	assertHasKey(t, keys, "u", "update")
	assertHasKey(t, keys, "R", "restore")
	assertHasKey(t, keys, "tab", "sidebar")
	assertHasGlobalKeys(t, keys)

	// All game keys should be enabled with backup and updates.
	for _, k := range keys {
		if k.Key == "u" || k.Key == "R" || k.Key == "L" || k.Key == "i" {
			if !k.Enabled {
				t.Errorf("key %q should be enabled when game has backup and updates", k.Key)
			}
		}
	}
}

func TestContextKeys_ContentFocusedNoGame(t *testing.T) {
	content := &ContentModel{}
	keys := ContextKeys(FocusContent, false, false, content, true)

	// Should have basic content keys but not game-specific ones.
	assertHasKey(t, keys, "↑↓", "navigate")
	assertHasKey(t, keys, "←→", "change")
	assertHasKey(t, keys, "s", "save")
	assertHasKey(t, keys, "tab", "sidebar")
	assertMissingKey(t, keys, "L")
	assertMissingKey(t, keys, "u")
	assertMissingKey(t, keys, "R")
}

func TestContextKeys_DisabledRestoreNoBackup(t *testing.T) {
	content := &ContentModel{
		game:       &game.Game{AppID: 1091500},
		hasBackup:  false,
		hasUpdates: true,
	}
	keys := ContextKeys(FocusContent, false, false, content, true)

	k := findKey(keys, "R")
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
	keys := ContextKeys(FocusContent, false, false, content, true)

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
	keys := ContextKeys(FocusContent, false, false, content, true)

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

func TestContextKeys_DisabledLaunchLaunching(t *testing.T) {
	content := &ContentModel{
		game:      &game.Game{AppID: 1091500},
		launching: true,
	}
	keys := ContextKeys(FocusContent, false, false, content, true)

	k := findKey(keys, "L")
	if k == nil {
		t.Fatal("expected launch key to be present")
	}
	if k.Enabled {
		t.Error("launch should be disabled when already launching")
	}
	if k.Reason != "launching" {
		t.Errorf("launch reason should be 'launching', got %q", k.Reason)
	}
}

func TestContextKeys_HintsDisabled(t *testing.T) {
	keys := ContextKeys(FocusSidebar, false, false, nil, false)

	// Should only have global keys when hints are off.
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
		{
			name:    "busy",
			content: &ContentModel{dllOperating: true, hasUpdates: true, hasBackup: true},
			want:    "busy",
		},
		{
			name:    "up to date",
			content: &ContentModel{hasUpdates: false, hasBackup: true},
			want:    "up to date",
		},
		{
			name:    "no backup",
			content: &ContentModel{hasUpdates: true, hasBackup: false},
			want:    "no backup",
		},
		{
			name:    "no reason when all good",
			content: &ContentModel{hasUpdates: true, hasBackup: true},
			want:    "",
		},
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
	keys := ContextKeys(FocusSidebar, false, false, nil, true)
	result := RenderContextBar(keys, 200, theme)

	// The rendered output should contain the text of each global key.
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

	// The reason text should appear in the output.
	if !strings.Contains(result, "no backup") {
		t.Error("rendered bar should contain disabled reason 'no backup'")
	}
}

func TestRenderContextBar_Truncation(t *testing.T) {
	theme := &DefaultTheme
	// Create many keys so they definitely exceed a narrow width.
	keys := []ContextKey{
		{Key: "↑↓", Action: "navigate", Enabled: true},
		{Key: "/", Action: "search", Enabled: true},
		{Key: "d", Action: "DLLs", Enabled: true},
		{Key: "p", Action: "profile", Enabled: true},
		{Key: "s", Action: "sort", Enabled: true},
		{Key: "r", Action: "rescan", Enabled: true},
		{Key: "enter", Action: "select", Enabled: true},
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}

	// Use a narrow width that can't fit all keys.
	result := RenderContextBar(keys, 40, theme)
	if !strings.Contains(result, "...") {
		t.Error("narrow bar should contain ellipsis truncation")
	}

	// Global keys should still be present even when truncated.
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

	// Enabled and disabled versions should render differently.
	if resultEnabled == resultDisabled {
		t.Error("enabled and disabled keys should have different rendered output")
	}

	// The visual width should differ because disabled has the reason appended.
	enabledWidth := lipgloss.Width(resultEnabled)
	disabledWidth := lipgloss.Width(resultDisabled)
	if disabledWidth <= enabledWidth {
		t.Errorf("disabled bar (width=%d) should be wider than enabled (width=%d) due to reason text",
			disabledWidth, enabledWidth)
	}
}

func TestRenderContextBar_GlobalKeysOnly(t *testing.T) {
	theme := &DefaultTheme
	// When showHints=false, ContextKeys returns only globalKeys.
	keys := ContextKeys(FocusSidebar, false, false, nil, false)
	result := RenderContextBar(keys, 200, theme)

	// Should render something non-empty.
	if result == "" {
		t.Error("global-only bar should not be empty")
	}
	// Should contain all global key actions.
	if !strings.Contains(result, "help") {
		t.Error("bar should contain 'help'")
	}
	if !strings.Contains(result, "quit") {
		t.Error("bar should contain 'quit'")
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
