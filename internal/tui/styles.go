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

// DarkTheme uses deeper blacks and adjusted contrast for OLED/dark environments.
var DarkTheme = Theme{
	Name: "dark",

	Primary:     lipgloss.Color("133"), // Amethyst
	Secondary:   lipgloss.Color("69"),  // Royal Blue
	Accent:      lipgloss.Color("212"), // Pink Carnation
	Text:        lipgloss.Color("253"), // Near-white
	TextDim:     lipgloss.Color("240"), // Medium gray
	Background:  lipgloss.Color("232"), // Deep charcoal (not quite black)
	Border:      lipgloss.Color("55"),  // Deep violet
	BorderFocus: lipgloss.Color("99"),  // Slate blue

	Success: lipgloss.Color("71"),
	Error:   lipgloss.Color("160"),
	Warning: lipgloss.Color("172"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("17"),  // Darkest blue
}

// LightTheme uses light backgrounds with dark text for bright environments.
var LightTheme = Theme{
	Name: "light",

	Primary:     lipgloss.Color("91"),  // Velvet Orchid
	Secondary:   lipgloss.Color("26"),  // Medium blue
	Accent:      lipgloss.Color("162"), // Deep pink
	Text:        lipgloss.Color("235"), // Near-black
	TextDim:     lipgloss.Color("243"), // Dark gray
	Background:  lipgloss.Color("255"), // Ghost White
	Border:      lipgloss.Color("183"), // Light violet
	BorderFocus: lipgloss.Color("91"),  // Velvet Orchid

	Success: lipgloss.Color("28"),
	Error:   lipgloss.Color("160"),
	Warning: lipgloss.Color("130"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("91"),  // Velvet Orchid
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

	dlssStyle = lipgloss.NewStyle().
		Foreground(t.Secondary)

	errorStyle = lipgloss.NewStyle().
		Foreground(t.Error)

	successStyle = lipgloss.NewStyle().
		Foreground(t.Success)

	warningStyle = lipgloss.NewStyle().
		Foreground(t.Warning)

	cliPrimaryStyle = lipgloss.NewStyle().Foreground(t.Primary)
	cliSecondaryStyle = lipgloss.NewStyle().Foreground(t.Secondary)
	cliDimStyle = lipgloss.NewStyle().Foreground(t.TextDim)
	cliSuccessStyle = lipgloss.NewStyle().Foreground(t.Success)
	cliErrorStyle = lipgloss.NewStyle().Foreground(t.Error)
	cliAccentStyle = lipgloss.NewStyle().Foreground(t.Accent)
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
	cliPrimaryStyle   lipgloss.Style
	cliSecondaryStyle lipgloss.Style
	cliDimStyle       lipgloss.Style
	cliSuccessStyle   lipgloss.Style
	cliErrorStyle     lipgloss.Style
	cliAccentStyle    lipgloss.Style
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
