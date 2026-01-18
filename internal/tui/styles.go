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

// Spela color palette (from logo)
// Midnight Black: #000000 (16)
// Dark Amethyst:  #200748 (53)
// Velvet Orchid:  #64297D (91)
// Amethyst:       #9C41AA (133)
// Pink Carnation: #FA76C2 (212)
// Dusk Blue:      #3D58A1 (62)
// Royal Blue:     #566EDC (69)
// Ghost White:    #F5F5FD (255)

var DefaultTheme = Theme{
	Name: "default",

	Primary:     lipgloss.Color("133"), // Amethyst
	Secondary:   lipgloss.Color("69"),  // Royal Blue
	Accent:      lipgloss.Color("212"), // Pink Carnation
	Text:        lipgloss.Color("255"), // Ghost White
	TextDim:     lipgloss.Color("145"), // Light purple
	Background:  lipgloss.Color("16"),  // Midnight Black
	Border:      lipgloss.Color("91"),  // Velvet Orchid
	BorderFocus: lipgloss.Color("133"), // Amethyst

	Success: lipgloss.Color("114"),
	Error:   lipgloss.Color("203"),
	Warning: lipgloss.Color("215"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("53"),  // Dark Amethyst
}

var DarkTheme = DefaultTheme

var LightTheme = Theme{
	Name: "light",

	Primary:     lipgloss.Color("133"), // Amethyst
	Secondary:   lipgloss.Color("62"),  // Dusk Blue
	Accent:      lipgloss.Color("212"), // Pink Carnation
	Text:        lipgloss.Color("53"),  // Dark Amethyst
	TextDim:     lipgloss.Color("91"),  // Velvet Orchid
	Background:  lipgloss.Color("255"), // Ghost White
	Border:      lipgloss.Color("140"), // Light orchid
	BorderFocus: lipgloss.Color("133"), // Amethyst

	Success: lipgloss.Color("34"),
	Error:   lipgloss.Color("160"),
	Warning: lipgloss.Color("172"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("133"), // Amethyst
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

// CLI color helper styles using the spela theme
var (
	cliPrimaryStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("133")) // Amethyst
	cliSecondaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))  // Royal Blue
	cliDimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("145")) // Light purple
	cliSuccessStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	cliErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	cliAccentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")) // Pink Carnation
)

func CLIPrimary(text string) string {
	return cliPrimaryStyle.Render(text)
}

func CLISecondary(text string) string {
	return cliSecondaryStyle.Render(text)
}

func CLIDim(text string) string {
	return cliDimStyle.Render(text)
}

func CLISuccess(text string) string {
	return cliSuccessStyle.Render(text)
}

func CLIError(text string) string {
	return cliErrorStyle.Render(text)
}

func CLIAccent(text string) string {
	return cliAccentStyle.Render(text)
}
