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
	styles       *Styles
	sections     []HelpSection
	height       int
	width        int
	offset       int
	closeFocused bool
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
	maximum := max(len(strings.Split(m.content(), "\n"))-(m.height-3), 0)
	m.offset = min(max(m.offset, 0), maximum)
}

func NewHelp(styles *Styles) HelpModel {
	return NewHelpForContext(styles, BindingContext{Mode: ModeBrowse, Focus: FocusList})
}

func NewHelpForContext(styles *Styles, context BindingContext) HelpModel {
	current := HelpSection{Title: fmt.Sprintf("After closing Help: %s · %s", context.Destination, context.Focus)}
	for _, resolution := range CanonicalKeymap.HelpBindings(context) {
		binding := describeBinding(resolution.Binding, context)
		if binding.Mode == ModeAny || len(binding.Keys) == 0 {
			continue
		}
		var labels []string
		for _, candidate := range binding.Keys {
			if !candidate.Printable {
				labels = append(labels, candidate.Label)
			}
		}
		if len(labels) > 0 {
			current.Bindings = append(current.Bindings, HelpBinding{Key: strings.Join(labels, " "), Description: binding.Description})
		}
	}
	menu := HelpSection{Title: "Actions in this context (open with 0 after closing Help)"}
	for _, resolution := range CanonicalKeymap.MenuBindings(context) {
		description := resolution.Binding.Description
		if !resolution.Available {
			description += " (" + resolution.Reason + ")"
		}
		menu.Bindings = append(menu.Bindings, HelpBinding{Key: "Actions", Description: description})
	}
	sections := []HelpSection{current, menu}
	for _, mode := range []InputMode{ModeBrowse, ModeEdit, ModeSearch, ModeOverlay} {
		section := HelpSection{Title: "Reference: " + string(mode) + " controls"}
		for _, binding := range CanonicalKeymap.bindings {
			if binding.Mode != mode || len(binding.Keys) == 0 {
				continue
			}
			var labels []string
			for _, candidate := range binding.Keys {
				labels = append(labels, candidate.Label)
			}
			description := binding.Description
			if binding.Focus != FocusAny {
				description += " (" + string(binding.Focus) + " pane)"
			}
			if binding.Availability != nil {
				_, reason := binding.Availability(context)
				if reason != "" {
					description += "; " + reason
				}
			}
			section.Bindings = append(section.Bindings, HelpBinding{Key: strings.Join(labels, " "), Description: description})
		}
		sections = append(sections, section)
	}
	return HelpModel{styles: styles, sections: sections}
}

func (m HelpModel) View() string {
	lines := strings.Split(m.content(), "\n")
	visible := max(m.height-3, 1)
	offset := min(m.offset, max(len(lines)-visible, 0))
	end := min(offset+visible, len(lines))
	body := strings.Join(lines[offset:end], "\n")
	hint := "Tab: next control"
	if m.closeFocused {
		hint += "  Enter: close"
	} else if len(lines) > visible {
		hint += "  ↑ ↓: scroll"
	}
	return body + "\n" + renderControl("Close", m.closeFocused, m.styles) + "\n" + m.styles.Dim.Render(hint)
}

func (m HelpModel) content() string {
	var lines []string
	width := m.width
	if width <= 0 {
		width = 60
	}
	lines = append(lines, m.styles.Title.Render("Keyboard shortcuts"), "Reference only. Close Help before using these controls.", "")
	for _, section := range m.sections {
		lines = append(lines, m.styles.Title.Render(section.Title))
		for _, binding := range section.Bindings {
			text := "  " + binding.Key + ": " + binding.Description
			lines = append(lines, ansi.Hardwrap(text, width, true))
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ContextKey represents a keybinding with its current state.
type ContextKey struct {
	Key     string // display text for the key (e.g., "u", "R", "tab")
	Action  string // short description (e.g., "update", "restore", "sidebar")
	Enabled bool   // false = dimmed with reason
	Reason  string // shown when disabled (e.g., "no backup")
}

// globalKeys project the basic entry point into the Actions menu.
var globalKeys = canonicalGlobalContextKeys()

func canonicalGlobalContextKeys() []ContextKey {
	var keys []ContextKey
	for _, resolution := range CanonicalKeymap.HelpBindings(BindingContext{Mode: ModeBrowse, Focus: FocusList}) {
		binding := resolution.Binding
		if binding.Scope != ScopeGlobal || binding.Action != ActionShowActions {
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
