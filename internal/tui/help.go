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
				Title: "Rail",
				Bindings: []HelpBinding{
					{"1", "Games resource"},
					{"2", "DLLs resource"},
					{"3", "Defaults resource"},
					{"4", "Metrics resource"},
					{"↑/k", "Move rail cursor up"},
					{"↓/j", "Move rail cursor down"},
					{"Enter", "Activate rail selection"},
					{"Tab", "Toggle rail ↔ resource pane focus"},
				},
			},
			{
				Title: "Profile detail (Games + Defaults)",
				Bindings: []HelpBinding{
					{"↓/j", "Next field (crosses group boundaries)"},
					{"↑/k", "Previous field (crosses group boundaries)"},
					{"Tab", "Enter pane from rail / leave to rail"},
					{"Esc/q", "Return to rail from detail"},
				},
			},
			{
				Title: "Reserved for profile editing (Task 5)",
				Bindings: []HelpBinding{
					{"r", "Reset field to inherited"},
					{"Shift+R", "Reset entire profile to inherited"},
					{"p", "Pin current value as override"},
					{":", "Command palette (deferred)"},
				},
			},
			{
				Title: "Games list",
				Bindings: []HelpBinding{
					{"/, Ctrl+F", "Search games"},
					{"d", "Toggle DLLs filter"},
					{"P", "Toggle profile filter (shift+p)"},
					{"s", "Cycle sort mode"},
					{"C", "Clear all filters"},
					{"Ctrl+R", "Rescan games"},
				},
			},
			{
				Title: "Game detail",
				Bindings: []HelpBinding{
					{"↑/k", "Previous setting"},
					{"↓/j", "Next setting"},
					{"←/h", "Decrease value"},
					{"→/l", "Increase value"},
					{"s", "Save profile"},
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
					{"◆", "Game has profile / active resource"},
				},
			},
			{
				Title: "Displaced bindings (Task 3 keymap audit)",
				Bindings: []HelpBinding{
					{"r → Ctrl+R", "Rescan games moved so `r` can reset a profile field"},
					{"p → P", "Profile filter moved so `p` can pin a field"},
				},
			},
			{
				Title: "General",
				Bindings: []HelpBinding{
					{"?", "Toggle help"},
					{"o", "Options"},
					{"q", "Quit / step back"},
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
		Width(12)

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

// ContextKeys returns the keybindings relevant to the current context. The
// first argument used to be `sidebarFocused` in the previous shell; here
// it means `railFocused`. Kept positional so callers don't ripple.
func ContextKeys(railFocused bool, searchFocused, selectMode bool, content *ContentModel, showHints bool) []ContextKey {
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

	case railFocused:
		keys = []ContextKey{
			{Key: "↑↓", Action: "navigate", Enabled: true},
			{Key: "1-4", Action: "resource", Enabled: true},
			{Key: "enter", Action: "activate", Enabled: true},
			{Key: "tab", Action: "pane", Enabled: true},
		}

	default: // resource pane focused (games list / game detail)
		keys = []ContextKey{
			{Key: "↑↓", Action: "navigate", Enabled: true},
			{Key: "/", Action: "search", Enabled: true},
			{Key: "d", Action: "DLLs", Enabled: true},
			{Key: "P", Action: "profile", Enabled: true},
			{Key: "s", Action: "sort", Enabled: true},
			{Key: "ctrl+r", Action: "rescan", Enabled: true},
			{Key: "enter", Action: "select", Enabled: true},
		}

		if content != nil && content.game != nil {
			keys = append(keys,
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

		keys = append(keys, ContextKey{Key: "tab", Action: "rail", Enabled: true})
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
func RenderContextBar(keys []ContextKey, width int, theme *Theme) string {
	if len(keys) == 0 || width <= 0 {
		return ""
	}

	keyStyle := lipgloss.NewStyle().Foreground(theme.Accent)
	actionStyle := lipgloss.NewStyle().Foreground(theme.TextDim)
	disabledStyle := lipgloss.NewStyle().Foreground(theme.Border)

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

	globalCount := len(globalKeys)
	if globalCount > len(keys) {
		globalCount = len(keys)
	}
	contextKeys := keys[:len(keys)-globalCount]
	suffixKeys := keys[len(keys)-globalCount:]

	suffixParts := make([]string, len(suffixKeys))
	for i, k := range suffixKeys {
		suffixParts[i] = renderKey(k)
	}
	suffix := strings.Join(suffixParts, contextKeySeparator)
	suffixWidth := lipgloss.Width(suffix)

	if suffixWidth >= width {
		return suffix
	}

	ellipsis := "..."
	ellipsisWidth := len(ellipsis)
	budget := width - suffixWidth - len(contextKeySeparator)

	var rendered []string
	usedWidth := 0
	truncated := false

	for i, ck := range contextKeys {
		part := renderKey(ck)
		partWidth := lipgloss.Width(part)

		separatorWidth := 0
		if i > 0 {
			separatorWidth = len(contextKeySeparator)
		}

		needed := partWidth + separatorWidth
		remaining := len(contextKeys) - i - 1
		reserveEllipsis := 0
		if remaining > 0 {
			reserveEllipsis = ellipsisWidth + len(contextKeySeparator)
		}

		if usedWidth+needed+reserveEllipsis > budget && remaining > 0 {
			truncated = true
			break
		}

		if usedWidth+needed > budget {
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
