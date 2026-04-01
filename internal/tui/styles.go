package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Name string

	Primary     color.Color
	Secondary   color.Color
	Accent      color.Color
	Text        color.Color
	TextDim     color.Color
	TextMuted   color.Color
	Background  color.Color
	Border      color.Color
	BorderFocus color.Color

	Success color.Color
	Error   color.Color
	Warning color.Color

	SelectionFg color.Color
	SelectionBg color.Color

	// Thermal gradient stops (cool → hot → throttle).
	ThermalCold     color.Color
	ThermalCool     color.Color
	ThermalWarm     color.Color
	ThermalHot      color.Color
	ThermalCritical color.Color
	ThermalThrottle color.Color
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
	TextMuted:   lipgloss.Color("240"), // Dark gray
	Background:  lipgloss.Color("16"),  // Midnight Black
	Border:      lipgloss.Color("91"),  // Velvet Orchid
	BorderFocus: lipgloss.Color("133"), // Amethyst

	Success: lipgloss.Color("114"),
	Error:   lipgloss.Color("203"),
	Warning: lipgloss.Color("215"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("53"),  // Dark Amethyst

	ThermalCold:     lipgloss.Color("39"),  // Deep sky blue
	ThermalCool:     lipgloss.Color("50"),  // Cyan
	ThermalWarm:     lipgloss.Color("226"), // Yellow
	ThermalHot:      lipgloss.Color("208"), // Orange
	ThermalCritical: lipgloss.Color("196"), // Red
	ThermalThrottle: lipgloss.Color("201"), // Magenta
}

// DarkTheme uses deeper blacks and adjusted contrast for OLED/dark environments.
var DarkTheme = Theme{
	Name: "dark",

	Primary:     lipgloss.Color("133"), // Amethyst
	Secondary:   lipgloss.Color("69"),  // Royal Blue
	Accent:      lipgloss.Color("212"), // Pink Carnation
	Text:        lipgloss.Color("253"), // Near-white
	TextDim:     lipgloss.Color("240"), // Medium gray
	TextMuted:   lipgloss.Color("237"), // Darker gray
	Background:  lipgloss.Color("232"), // Deep charcoal (not quite black)
	Border:      lipgloss.Color("55"),  // Deep violet
	BorderFocus: lipgloss.Color("99"),  // Slate blue

	Success: lipgloss.Color("71"),
	Error:   lipgloss.Color("160"),
	Warning: lipgloss.Color("172"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("17"),  // Darkest blue

	ThermalCold:     lipgloss.Color("33"),  // Blue
	ThermalCool:     lipgloss.Color("44"),  // Dark cyan
	ThermalWarm:     lipgloss.Color("220"), // Gold
	ThermalHot:      lipgloss.Color("202"), // Dark orange
	ThermalCritical: lipgloss.Color("160"), // Dark red
	ThermalThrottle: lipgloss.Color("163"), // Dark magenta
}

// LightTheme uses light backgrounds with dark text for bright environments.
var LightTheme = Theme{
	Name: "light",

	Primary:     lipgloss.Color("91"),  // Velvet Orchid
	Secondary:   lipgloss.Color("26"),  // Medium blue
	Accent:      lipgloss.Color("162"), // Deep pink
	Text:        lipgloss.Color("235"), // Near-black
	TextDim:     lipgloss.Color("243"), // Dark gray
	TextMuted:   lipgloss.Color("249"), // Light gray
	Background:  lipgloss.Color("255"), // Ghost White
	Border:      lipgloss.Color("183"), // Light violet
	BorderFocus: lipgloss.Color("91"),  // Velvet Orchid

	Success: lipgloss.Color("28"),
	Error:   lipgloss.Color("160"),
	Warning: lipgloss.Color("130"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("91"),  // Velvet Orchid

	ThermalCold:     lipgloss.Color("27"),  // Dark blue
	ThermalCool:     lipgloss.Color("30"),  // Teal
	ThermalWarm:     lipgloss.Color("178"), // Dark yellow
	ThermalHot:      lipgloss.Color("166"), // Dark orange
	ThermalCritical: lipgloss.Color("124"), // Dark red
	ThermalThrottle: lipgloss.Color("125"), // Dark magenta
}

// Styles holds the active theme and all derived lipgloss styles.
// Shared by pointer across all TUI models for consistent rendering.
type Styles struct {
	Theme     Theme
	ShowHints bool

	Title    lipgloss.Style
	Selected lipgloss.Style
	Normal   lipgloss.Style
	Dim      lipgloss.Style
	DLSS     lipgloss.Style
	Error    lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
}

// NewStyles creates a Styles instance from the given theme and hint preference.
func NewStyles(theme Theme, showHints bool) *Styles {
	s := &Styles{Theme: theme, ShowHints: showHints}
	s.rebuild()
	return s
}

// SetTheme changes the active theme and rebuilds all derived styles.
func (s *Styles) SetTheme(t Theme) {
	s.Theme = t
	s.rebuild()
}

// SetShowHints updates the hint visibility preference.
func (s *Styles) SetShowHints(show bool) {
	s.ShowHints = show
}

func (s *Styles) rebuild() {
	t := s.Theme

	s.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		MarginBottom(1)

	s.Selected = lipgloss.NewStyle().
		Foreground(t.SelectionFg).
		Background(t.SelectionBg).
		Bold(true)

	s.Normal = lipgloss.NewStyle().
		Foreground(t.Text)

	s.Dim = lipgloss.NewStyle().
		Foreground(t.TextDim)

	s.DLSS = lipgloss.NewStyle().
		Foreground(t.Secondary)

	s.Error = lipgloss.NewStyle().
		Foreground(t.Error)

	s.Success = lipgloss.NewStyle().
		Foreground(t.Success)

	s.Warning = lipgloss.NewStyle().
		Foreground(t.Warning)
}

// RenderHint renders hint text in dim style, or returns empty if hints are disabled.
func (s *Styles) RenderHint(text string) string {
	if !s.ShowHints {
		return ""
	}
	return s.Dim.Render(text)
}

// BorderColor returns the appropriate border color for the given focus state.
func (s *Styles) BorderColor(focused bool) color.Color {
	if focused {
		return s.Theme.BorderFocus
	}
	return s.Theme.Border
}

// IndicatorStyle returns the style for a given state indicator.
func (s *Styles) IndicatorStyle(state StateIndicator) lipgloss.Style {
	switch state {
	case StateModified:
		return s.Warning
	case StateActive:
		return s.Success
	case StateDisabled:
		return s.Dim
	case StateError:
		return s.Error
	case StateSuccess:
		return s.Success
	default:
		return s.Normal
	}
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

// CLI color helper styles — immutable, computed once from DefaultTheme.
var (
	cliPrimaryStyle   = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)
	cliSecondaryStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary)
	cliDimStyle       = lipgloss.NewStyle().Foreground(DefaultTheme.TextDim)
	cliSuccessStyle   = lipgloss.NewStyle().Foreground(DefaultTheme.Success)
	cliErrorStyle     = lipgloss.NewStyle().Foreground(DefaultTheme.Error)
	cliAccentStyle    = lipgloss.NewStyle().Foreground(DefaultTheme.Accent)
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
