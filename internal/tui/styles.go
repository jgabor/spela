package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name string

	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Accent      lipgloss.Color
	Text        lipgloss.Color
	TextDim     lipgloss.Color
	Background  lipgloss.Color
	Border      lipgloss.Color
	BorderFocus lipgloss.Color

	Success lipgloss.Color
	Error   lipgloss.Color
	Warning lipgloss.Color

	SelectionFg lipgloss.Color
	SelectionBg lipgloss.Color
}

var DefaultTheme = Theme{
	Name: "default",

	Primary:     lipgloss.Color("205"),
	Secondary:   lipgloss.Color("76"),
	Accent:      lipgloss.Color("229"),
	Text:        lipgloss.Color("252"),
	TextDim:     lipgloss.Color("241"),
	Background:  lipgloss.Color("236"),
	Border:      lipgloss.Color("241"),
	BorderFocus: lipgloss.Color("205"),

	Success: lipgloss.Color("76"),
	Error:   lipgloss.Color("196"),
	Warning: lipgloss.Color("214"),

	SelectionFg: lipgloss.Color("229"),
	SelectionBg: lipgloss.Color("57"),
}

var DarkTheme = Theme{
	Name: "dark",

	Primary:     lipgloss.Color("141"),
	Secondary:   lipgloss.Color("114"),
	Accent:      lipgloss.Color("223"),
	Text:        lipgloss.Color("255"),
	TextDim:     lipgloss.Color("245"),
	Background:  lipgloss.Color("234"),
	Border:      lipgloss.Color("238"),
	BorderFocus: lipgloss.Color("141"),

	Success: lipgloss.Color("114"),
	Error:   lipgloss.Color("203"),
	Warning: lipgloss.Color("215"),

	SelectionFg: lipgloss.Color("234"),
	SelectionBg: lipgloss.Color("141"),
}

var (
	activeTheme = DefaultTheme
	showHints   = true
)

func SetTheme(t Theme) {
	activeTheme = t
	rebuildStyles()
}

func GetTheme() Theme {
	return activeTheme
}

func SetShowHints(show bool) {
	showHints = show
}

func ShowHints() bool {
	return showHints
}

func RenderHint(text string) string {
	if !showHints {
		return ""
	}
	return dimStyle.Render(text)
}

var (
	titleStyle    lipgloss.Style
	selectedStyle lipgloss.Style
	normalStyle   lipgloss.Style
	dimStyle      lipgloss.Style
	helpStyle     lipgloss.Style
	dlssStyle     lipgloss.Style
	errorStyle    lipgloss.Style
	successStyle  lipgloss.Style
	warningStyle  lipgloss.Style
)

func init() {
	rebuildStyles()
}

func rebuildStyles() {
	t := activeTheme

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
		Foreground(t.SelectionFg).
		Background(t.SelectionBg).
		Bold(true)

	normalStyle = lipgloss.NewStyle().
		Foreground(t.Text)

	dimStyle = lipgloss.NewStyle().
		Foreground(t.TextDim)

	helpStyle = lipgloss.NewStyle().
		Foreground(t.TextDim).
		MarginTop(1)

	dlssStyle = lipgloss.NewStyle().
		Foreground(t.Secondary)

	errorStyle = lipgloss.NewStyle().
		Foreground(t.Error)

	successStyle = lipgloss.NewStyle().
		Foreground(t.Success)

	warningStyle = lipgloss.NewStyle().
		Foreground(t.Warning)
}

type StateIndicator int

const (
	StateNormal StateIndicator = iota
	StateModified
	StateActive
	StateDisabled
	StateLoading
	StateError
	StateSuccess
)

func IndicatorIcon(state StateIndicator) string {
	switch state {
	case StateModified:
		return "●"
	case StateActive:
		return "▶"
	case StateDisabled:
		return "○"
	case StateLoading:
		return "⟳"
	case StateError:
		return "✗"
	case StateSuccess:
		return "✓"
	default:
		return ""
	}
}

func IndicatorStyle(state StateIndicator) lipgloss.Style {
	switch state {
	case StateModified:
		return warningStyle
	case StateActive:
		return successStyle
	case StateDisabled:
		return dimStyle
	case StateError:
		return errorStyle
	case StateSuccess:
		return successStyle
	default:
		return normalStyle
	}
}

func BorderColor(focused bool) lipgloss.Color {
	if focused {
		return activeTheme.BorderFocus
	}
	return activeTheme.Border
}
