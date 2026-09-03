package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	height   int
	width    int
	offset   int
}

func (m *HelpModel) SetHeight(height int) {
	m.SetSize(m.width, height)
}

func (m *HelpModel) SetSize(width, height int) {
	m.width = width
	m.height = max(height, 3)
	m.clampOffset()
}

func (m *HelpModel) Move(delta int) {
	m.offset += delta
	m.clampOffset()
}

func (m *HelpModel) clampOffset() {
	maximum := max(len(strings.Split(m.content(), "\n"))-(m.height-2), 0)
	m.offset = min(max(m.offset, 0), maximum)
}

func NewHelp(styles *Styles) HelpModel {
	return NewHelpForContext(styles, BindingContext{Mode: ModeBrowse, Focus: FocusList})
}

func NewHelpForContext(styles *Styles, context BindingContext) HelpModel {
	return HelpModel{
		styles:   styles,
		sections: []HelpSection{canonicalHelpSection(context)},
	}
}

func canonicalHelpSection(context BindingContext) HelpSection {
	section := HelpSection{Title: "Shell"}
	for _, resolution := range CanonicalKeymap.HelpBindings(context) {
		binding := resolution.Binding
		labels := make([]string, 0, len(binding.Keys))
		for _, candidate := range binding.Keys {
			labels = append(labels, candidate.Label)
		}
		description := binding.Description
		if !resolution.Available && resolution.Reason != "" {
			description += " (" + resolution.Reason + ")"
		}
		section.Bindings = append(section.Bindings, HelpBinding{
			Key: strings.Join(labels, " / "), Description: description,
		})
	}
	return section
}

func (m HelpModel) View() string {
	lines := strings.Split(m.content(), "\n")
	if m.width > 0 {
		for index := range lines {
			lines[index] = ansi.Truncate(lines[index], m.width, "…")
		}
	}
	if m.height == 0 {
		return strings.Join(lines, "\n") + m.styles.Dim.Render("?, q, Esc close • Ctrl+C quits")
	}
	visible := max(m.height-2, 1)
	if len(lines) <= visible {
		return strings.Join(lines, "\n") + "\n" + m.styles.Dim.Render("?, q, Esc close • Ctrl+C quits")
	}
	end := min(m.offset+visible, len(lines))
	position := m.styles.Dim.Render(fmt.Sprintf("↑/↓ scroll  %d-%d/%d", m.offset+1, end, len(lines)))
	return strings.Join(lines[m.offset:end], "\n") + "\n" + position + "\n" + m.styles.Dim.Render("?, q, Esc close • Ctrl+C quits")
}

func (m HelpModel) content() string {
	s := m.styles
	t := s.Theme

	helpTitleStyle := s.Title.Foreground(t.Primary).MarginBottom(1)

	sectionStyle := s.Normal.
		Foreground(t.Secondary).
		Bold(true)

	keyStyle := s.Normal.
		Foreground(t.Accent).
		Width(18)

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

	return b.String()
}

// ContextKey represents a keybinding with its current state.
type ContextKey struct {
	Key     string // display text for the key (e.g., "u", "R", "tab")
	Action  string // short description (e.g., "update", "restore", "sidebar")
	Enabled bool   // false = dimmed with reason
	Reason  string // shown when disabled (e.g., "no backup")
}

// globalKeys are projected from the canonical behavior keymap and appended to
// every legacy pane context until those panes also publish canonical actions.
var globalKeys = canonicalGlobalContextKeys()

func canonicalGlobalContextKeys() []ContextKey {
	var keys []ContextKey
	for _, resolution := range CanonicalKeymap.HelpBindings(BindingContext{Mode: ModeBrowse, Focus: FocusList}) {
		binding := resolution.Binding
		if binding.Scope != ScopeGlobal || (binding.Action != ActionShowHelp && binding.Action != ActionQuit) {
			continue
		}
		for _, candidate := range binding.Keys {
			if candidate.Key == "ctrl+c" {
				continue
			}
			keys = append(keys, ContextKey{
				Key: candidate.Label, Action: strings.ToLower(binding.Description), Enabled: resolution.Available, Reason: resolution.Reason,
			})
		}
	}
	return keys
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

	ellipsis := "..."
	ellipsisWidth := len(ellipsis)

	var rendered []string
	usedWidth := 0
	truncated := false

	for i, ck := range keys {
		part := renderKey(ck)
		partWidth := lipgloss.Width(part)

		separatorWidth := 0
		if i > 0 {
			separatorWidth = len(contextKeySeparator)
		}

		needed := partWidth + separatorWidth
		remaining := len(keys) - i - 1
		reserveEllipsis := 0
		if remaining > 0 {
			reserveEllipsis = ellipsisWidth + len(contextKeySeparator)
		}

		if usedWidth+needed+reserveEllipsis > width && remaining > 0 {
			truncated = true
			break
		}

		if usedWidth+needed > width {
			truncated = true
			break
		}

		rendered = append(rendered, part)
		usedWidth += needed
	}

	if truncated {
		rendered = append(rendered, ellipsis)
	}

	return strings.Join(rendered, contextKeySeparator)
}
