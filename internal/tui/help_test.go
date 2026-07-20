package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderContextBar_EmptyKeys(t *testing.T) {
	if result := RenderContextBar(nil, 80, &DefaultTheme); result != "" {
		t.Errorf("expected empty string for nil keys, got %q", result)
	}
}

func TestRenderContextBar_ZeroWidth(t *testing.T) {
	keys := []ContextKey{{Key: "q", Action: "quit", Enabled: true}}
	if result := RenderContextBar(keys, 0, &DefaultTheme); result != "" {
		t.Errorf("expected empty string for zero width, got %q", result)
	}
}

func TestRenderContextBar_ContainsGlobalKeys(t *testing.T) {
	result := RenderContextBar(globalKeys, 200, &DefaultTheme)
	for _, key := range globalKeys {
		if !strings.Contains(result, key.Key) {
			t.Errorf("rendered bar missing global key %q", key.Key)
		}
	}
}

func TestRenderContextBar_DisabledKeyShowsReason(t *testing.T) {
	keys := []ContextKey{
		{Key: "R", Action: "restore", Enabled: false, Reason: "no backup"},
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}
	if result := RenderContextBar(keys, 200, &DefaultTheme); !strings.Contains(result, "no backup") {
		t.Error("rendered bar should contain disabled reason 'no backup'")
	}
}

func TestRenderContextBar_Truncation(t *testing.T) {
	keys := []ContextKey{
		{Key: "↑↓", Action: "navigate", Enabled: true},
		{Key: "/", Action: "search", Enabled: true},
		{Key: "d", Action: "DLLs", Enabled: true},
		{Key: "p", Action: "profile", Enabled: true},
		{Key: "s", Action: "sort", Enabled: true},
		{Key: "ctrl+r", Action: "rescan", Enabled: true},
		{Key: "enter", Action: "select", Enabled: true},
		{Key: "?", Action: "help", Enabled: true},
		{Key: "o", Action: "options", Enabled: true},
		{Key: "q", Action: "quit", Enabled: true},
	}
	result := RenderContextBar(keys, 40, &DefaultTheme)
	if !strings.Contains(result, "...") || !strings.Contains(result, "quit") {
		t.Errorf("narrow bar did not preserve truncation and global keys: %q", result)
	}
}

func TestRenderContextBar_WideEnoughNoTruncation(t *testing.T) {
	keys := append([]ContextKey{{Key: "a", Action: "act", Enabled: true}}, globalKeys...)
	if result := RenderContextBar(keys, 200, &DefaultTheme); strings.Contains(result, "...") {
		t.Error("wide bar should not contain ellipsis")
	}
}

func TestRenderContextBar_EnabledVsDisabledStyling(t *testing.T) {
	enabled := append([]ContextKey{{Key: "u", Action: "update", Enabled: true}}, globalKeys...)
	disabled := append([]ContextKey{{Key: "u", Action: "update", Reason: "busy"}}, globalKeys...)
	renderedEnabled := RenderContextBar(enabled, 200, &DefaultTheme)
	renderedDisabled := RenderContextBar(disabled, 200, &DefaultTheme)
	if renderedEnabled == renderedDisabled {
		t.Error("enabled and disabled keys should have different rendered output")
	}
	if lipgloss.Width(renderedDisabled) <= lipgloss.Width(renderedEnabled) {
		t.Error("disabled bar should be wider due to reason text")
	}
}

func TestRenderContextBar_GlobalKeysOnly(t *testing.T) {
	result := RenderContextBar(globalKeys, 200, &DefaultTheme)
	if result == "" || !strings.Contains(result, "shortcuts") || !strings.Contains(result, "quit") {
		t.Errorf("global-only bar = %q", result)
	}
}

func TestHelp_UsesCanonicalShellWithoutLegacyNavigation(t *testing.T) {
	out := strings.ToLower(NewHelp(NewStyles(DefaultTheme, true)).View())
	for _, want := range []string{"library", "dll catalog", "next pane", "keyboard shortcuts"} {
		if !strings.Contains(out, want) {
			t.Errorf("canonical help missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"rail", "primary", "context", "three-zone", "deferred"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("help retains legacy term %q:\n%s", forbidden, out)
		}
	}
}

func TestHelp_NoLaunchBinding(t *testing.T) {
	for _, line := range strings.Split(NewHelp(NewStyles(DefaultTheme, true)).View(), "\n") {
		if strings.Contains(strings.ToLower(line), "launch game") {
			t.Errorf("help line mentions launch game — must be removed:\n%s", line)
		}
	}
}
