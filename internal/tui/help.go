package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type HelpSection struct {
	Title    string
	Bindings []HelpBinding
}

type HelpBinding struct {
	Key         string
	Description string
}

type HelpModel struct {
	styles   *Styles
	sections []HelpSection
	width    int
	height   int
}

func NewHelp(styles *Styles) HelpModel {
	return HelpModel{
		styles: styles,
		sections: []HelpSection{
			{
				Title: "Navigation",
				Bindings: []HelpBinding{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
					{"Tab", "Switch pane"},
					{"Enter", "Select"},
					{"Esc", "Clear/back"},
				},
			},
			{
				Title: "Sidebar filters",
				Bindings: []HelpBinding{
					{"/, Ctrl+F", "Search games"},
					{"d", "Toggle DLLs filter"},
					{"p", "Toggle profile filter"},
					{"s", "Cycle sort mode"},
					{"C", "Clear all filters"},
					{"r", "Rescan games"},
				},
			},
			{
				Title: "Content actions",
				Bindings: []HelpBinding{
					{"↑/k", "Previous setting"},
					{"↓/j", "Next setting"},
					{"←/h", "Decrease value"},
					{"→/l", "Increase value"},
					{"s", "Save profile"},
					{"L", "Launch game"},
					{"i", "Install DLL"},
					{"u", "Update DLLs"},
					{"R", "Restore DLLs"},
				},
			},
			{
				Title: "Batch operations",
				Bindings: []HelpBinding{
					{"Space", "Enter multi-select / toggle selection"},
					{"a", "Select all visible"},
					{"A", "Deselect all"},
					{"Esc", "Exit multi-select"},
					{"Enter", "Execute batch action"},
				},
			},
			{
				Title: "Indicators",
				Bindings: []HelpBinding{
					{"●", "Game has DLLs"},
					{"◆", "Game has profile"},
				},
			},
			{
				Title: "General",
				Bindings: []HelpBinding{
					{"?", "Toggle help"},
					{"o", "Options"},
					{"q", "Quit"},
					{"Ctrl+C", "Force quit"},
				},
			},
		},
	}
}

func (m *HelpModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	return m, nil
}

func (m HelpModel) View() string {
	s := m.styles
	t := s.Theme

	helpTitleStyle := s.Title.Foreground(t.Primary).MarginBottom(1)

	sectionStyle := s.Normal.
		Foreground(t.Secondary).
		Bold(true)

	keyStyle := s.Normal.
		Foreground(t.Accent).
		Width(10)

	descStyle := s.Normal.
		Foreground(t.Text)

	var b strings.Builder

	b.WriteString(helpTitleStyle.Render("Keyboard shortcuts"))
	b.WriteString("\n\n")

	for i, section := range m.sections {
		b.WriteString(sectionStyle.Render(section.Title))
		b.WriteString("\n")

		for _, binding := range section.Bindings {
			b.WriteString("  ")
			b.WriteString(keyStyle.Render(binding.Key))
			b.WriteString(descStyle.Render(binding.Description))
			b.WriteString("\n")
		}

		if i < len(m.sections)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Press ? or Esc to close"))

	return b.String()
}

// ContextKey represents a keybinding with its current state.
type ContextKey struct {
	Key     string // display text for the key (e.g., "u", "R", "tab")
	Action  string // short description (e.g., "update", "restore", "sidebar")
	Enabled bool   // false = dimmed with reason
	Reason  string // shown when disabled (e.g., "no backup")
}

// globalKeys are always appended to every context key set.
var globalKeys = []ContextKey{
	{Key: "?", Action: "help", Enabled: true},
	{Key: "o", Action: "options", Enabled: true},
	{Key: "q", Action: "quit", Enabled: true},
}

// ContextKeys returns the keybindings relevant to the current context.
func ContextKeys(focus Focus, searchFocused, selectMode bool, content *ContentModel, showHints bool) []ContextKey {
	if !showHints {
		return globalKeys
	}

	var keys []ContextKey

	switch {
	case searchFocused:
		keys = []ContextKey{
			{Key: "type", Action: "filter", Enabled: true},
			{Key: "enter", Action: "done", Enabled: true},
			{Key: "esc", Action: "cancel", Enabled: true},
		}

	case selectMode:
		keys = []ContextKey{
			{Key: "↑↓", Action: "navigate", Enabled: true},
			{Key: "space", Action: "toggle", Enabled: true},
			{Key: "a", Action: "all", Enabled: true},
			{Key: "A", Action: "none", Enabled: true},
			{Key: "enter", Action: "batch", Enabled: true},
			{Key: "esc", Action: "exit", Enabled: true},
		}

	case focus == FocusSidebar:
		keys = []ContextKey{
			{Key: "↑↓", Action: "navigate", Enabled: true},
			{Key: "/", Action: "search", Enabled: true},
			{Key: "d", Action: "DLLs", Enabled: true},
			{Key: "p", Action: "profile", Enabled: true},
			{Key: "s", Action: "sort", Enabled: true},
			{Key: "r", Action: "rescan", Enabled: true},
			{Key: "enter", Action: "select", Enabled: true},
		}

	default: // content focused
		keys = []ContextKey{
			{Key: "↑↓", Action: "navigate", Enabled: true},
			{Key: "←→", Action: "change", Enabled: true},
			{Key: "s", Action: "save", Enabled: true},
		}

		if content != nil && content.game != nil {
			keys = append(keys,
				ContextKey{Key: "L", Action: "launch", Enabled: !content.launching, Reason: "launching"},
				ContextKey{Key: "i", Action: "install", Enabled: !content.dllOperating, Reason: "busy"},
				ContextKey{
					Key: "u", Action: "update",
					Enabled: content.hasUpdates && content.hasBackup && !content.dllOperating,
					Reason:  reasonForUpdate(content),
				},
				ContextKey{
					Key: "R", Action: "restore",
					Enabled: content.hasBackup && !content.dllOperating,
					Reason:  "no backup",
				},
			)
		}

		keys = append(keys, ContextKey{Key: "tab", Action: "sidebar", Enabled: true})
	}

	keys = append(keys, globalKeys...)
	return keys
}

// reasonForUpdate returns the most relevant reason why the update key is disabled.
func reasonForUpdate(content *ContentModel) string {
	if content.dllOperating {
		return "busy"
	}
	if !content.hasUpdates {
		return "up to date"
	}
	if !content.hasBackup {
		return "no backup"
	}
	return ""
}

// contextKeySeparator is placed between rendered keys in the bar.
const contextKeySeparator = "  "

// RenderContextBar renders the keybinding bar from a slice of ContextKeys.
// Uses theme colors: Accent for keys, TextDim for actions, TextDim (dimmer) for disabled.
// Format: "key:action" with "..." truncation when bar exceeds width.
func RenderContextBar(keys []ContextKey, width int, theme *Theme) string {
	if len(keys) == 0 || width <= 0 {
		return ""
	}

	keyStyle := lipgloss.NewStyle().Foreground(theme.Accent)
	actionStyle := lipgloss.NewStyle().Foreground(theme.TextDim)
	disabledStyle := lipgloss.NewStyle().Foreground(theme.Border)

	// Render a single ContextKey to a styled string.
	renderKey := func(ck ContextKey) string {
		if ck.Enabled {
			return keyStyle.Render(ck.Key) + actionStyle.Render(":"+ck.Action)
		}
		text := ck.Key + ":" + ck.Action
		if ck.Reason != "" {
			text += " (" + ck.Reason + ")"
		}
		return disabledStyle.Render(text)
	}

	// Separate context keys from the global suffix.
	// Global keys are the last len(globalKeys) items.
	globalCount := len(globalKeys)
	if globalCount > len(keys) {
		globalCount = len(keys)
	}
	contextKeys := keys[:len(keys)-globalCount]
	suffixKeys := keys[len(keys)-globalCount:]

	// Pre-render the global suffix (always shown).
	suffixParts := make([]string, len(suffixKeys))
	for i, k := range suffixKeys {
		suffixParts[i] = renderKey(k)
	}
	suffix := strings.Join(suffixParts, contextKeySeparator)
	suffixWidth := lipgloss.Width(suffix)

	// If only global keys fit, return just those.
	if suffixWidth >= width {
		return suffix
	}

	// Render context keys, fitting as many as possible within the budget.
	ellipsis := "..."
	ellipsisWidth := len(ellipsis)
	budget := width - suffixWidth - len(contextKeySeparator) // space for separator before suffix

	var rendered []string
	usedWidth := 0
	truncated := false

	for i, ck := range contextKeys {
		part := renderKey(ck)
		partWidth := lipgloss.Width(part)

		// Account for separator between parts.
		separatorWidth := 0
		if i > 0 {
			separatorWidth = len(contextKeySeparator)
		}

		needed := partWidth + separatorWidth
		// Reserve space for ellipsis if there are more keys after this one.
		remaining := len(contextKeys) - i - 1
		reserveEllipsis := 0
		if remaining > 0 {
			reserveEllipsis = ellipsisWidth + len(contextKeySeparator)
		}

		if usedWidth+needed+reserveEllipsis > budget && remaining > 0 {
			// This key doesn't fit and there are more keys; truncate.
			truncated = true
			break
		}

		if usedWidth+needed > budget {
			// Last key or only key that doesn't fit.
			truncated = true
			break
		}

		rendered = append(rendered, part)
		usedWidth += needed
	}

	if truncated {
		rendered = append(rendered, ellipsis)
	}

	rendered = append(rendered, suffix)
	return strings.Join(rendered, contextKeySeparator)
}
