package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Name string

	// Brand colors — identical across all themes.
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color

	// Surface palette — background layers.
	SurfaceBase      color.Color
	SurfaceRaised    color.Color
	SurfaceOverlay   color.Color
	SurfaceHighlight color.Color

	// Text palette — progressive dimming.
	TextPrimary color.Color // high-contrast body text (alias: Text)
	TextDim     color.Color // hints, timestamps — dimmer than old TextDim
	TextMuted   color.Color // decorative borders, disabled elements

	// Legacy aliases — kept during transition.
	Text       color.Color // alias for TextPrimary
	Background color.Color // alias for SurfaceBase

	// Semantic colors.
	Border      color.Color
	BorderFocus color.Color
	Success     color.Color
	Error       color.Color
	Warning     color.Color

	SelectionFg color.Color
	SelectionBg color.Color

	// Thermal gradient — six stops for temperature/load visualisation.
	ThermalCold     color.Color // idle, below 30%
	ThermalCool     color.Color // light load, 30-45%
	ThermalWarm     color.Color // normal, 45-65%
	ThermalHot      color.Color // elevated, 65-80%
	ThermalCritical color.Color // high, 80-90%
	ThermalThrottle color.Color // danger, 90%+

	// Metric-specific tokens.
	MetricGPUClock   color.Color
	MetricCPUFreq    color.Color
	MetricDLLCurrent color.Color
	MetricDLLUpdate  color.Color
	MetricDLLMissing color.Color
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

	Primary:   lipgloss.Color("133"), // Amethyst
	Secondary: lipgloss.Color("69"),  // Royal Blue
	Accent:    lipgloss.Color("212"), // Pink Carnation

	SurfaceBase:      lipgloss.Color("16"),  // Midnight Black
	SurfaceRaised:    lipgloss.Color("234"), // Slightly lighter than base
	SurfaceOverlay:   lipgloss.Color("236"), // Modal/overlay background
	SurfaceHighlight: lipgloss.Color("238"), // Hover/active row

	TextPrimary: lipgloss.Color("255"), // Ghost White
	TextDim:     lipgloss.Color("245"), // Hints, timestamps
	TextMuted:   lipgloss.Color("240"), // Decorative borders, disabled

	Text:       lipgloss.Color("255"), // alias: TextPrimary
	Background: lipgloss.Color("16"),  // alias: SurfaceBase

	Border:      lipgloss.Color("91"),  // Velvet Orchid
	BorderFocus: lipgloss.Color("133"), // Amethyst
	Success:     lipgloss.Color("114"),
	Error:       lipgloss.Color("203"),
	Warning:     lipgloss.Color("215"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("53"),  // Dark Amethyst

	ThermalCold:     lipgloss.Color("69"),  // #5F87FF
	ThermalCool:     lipgloss.Color("75"),  // #5FAFFF
	ThermalWarm:     lipgloss.Color("114"), // #87D787
	ThermalHot:      lipgloss.Color("221"), // #FFD75F
	ThermalCritical: lipgloss.Color("209"), // #FF875F
	ThermalThrottle: lipgloss.Color("203"), // #FF5F5F

	MetricGPUClock:   lipgloss.Color("75"),  // same as ThermalCool
	MetricCPUFreq:    lipgloss.Color("141"), // #AF87FF
	MetricDLLCurrent: lipgloss.Color("69"),  // brand secondary
	MetricDLLUpdate:  lipgloss.Color("221"), // thermal hot
	MetricDLLMissing: lipgloss.Color("245"), // text dim
}

// DarkTheme uses deeper blacks and adjusted contrast for OLED/dark environments.
var DarkTheme = Theme{
	Name: "dark",

	Primary:   lipgloss.Color("133"), // Amethyst
	Secondary: lipgloss.Color("69"),  // Royal Blue
	Accent:    lipgloss.Color("212"), // Pink Carnation

	SurfaceBase:      lipgloss.Color("232"), // Deep charcoal
	SurfaceRaised:    lipgloss.Color("233"), // Slightly lighter
	SurfaceOverlay:   lipgloss.Color("235"), // Modal/overlay
	SurfaceHighlight: lipgloss.Color("237"), // Active row

	TextPrimary: lipgloss.Color("253"), // Near-white
	TextDim:     lipgloss.Color("243"), // Hints, timestamps
	TextMuted:   lipgloss.Color("238"), // Decorative borders, disabled

	Text:       lipgloss.Color("253"), // alias: TextPrimary
	Background: lipgloss.Color("232"), // alias: SurfaceBase

	Border:      lipgloss.Color("55"), // Deep violet
	BorderFocus: lipgloss.Color("99"), // Slate blue
	Success:     lipgloss.Color("71"),
	Error:       lipgloss.Color("160"),
	Warning:     lipgloss.Color("172"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("17"),  // Darkest blue

	ThermalCold:     lipgloss.Color("69"),  // #5F87FF
	ThermalCool:     lipgloss.Color("75"),  // #5FAFFF
	ThermalWarm:     lipgloss.Color("114"), // #87D787
	ThermalHot:      lipgloss.Color("221"), // #FFD75F
	ThermalCritical: lipgloss.Color("209"), // #FF875F
	ThermalThrottle: lipgloss.Color("203"), // #FF5F5F

	MetricGPUClock:   lipgloss.Color("75"),
	MetricCPUFreq:    lipgloss.Color("141"),
	MetricDLLCurrent: lipgloss.Color("69"),
	MetricDLLUpdate:  lipgloss.Color("221"),
	MetricDLLMissing: lipgloss.Color("243"),
}

// LightTheme uses light backgrounds with dark text for bright environments.
var LightTheme = Theme{
	Name: "light",

	Primary:   lipgloss.Color("91"),  // Velvet Orchid
	Secondary: lipgloss.Color("26"),  // Medium blue
	Accent:    lipgloss.Color("162"), // Deep pink

	SurfaceBase:      lipgloss.Color("255"), // Ghost White
	SurfaceRaised:    lipgloss.Color("254"), // Slightly dimmer
	SurfaceOverlay:   lipgloss.Color("253"), // Modal/overlay
	SurfaceHighlight: lipgloss.Color("252"), // Active row

	TextPrimary: lipgloss.Color("235"), // Near-black
	TextDim:     lipgloss.Color("243"), // Hints, timestamps
	TextMuted:   lipgloss.Color("249"), // Decorative borders, disabled

	Text:       lipgloss.Color("235"), // alias: TextPrimary
	Background: lipgloss.Color("255"), // alias: SurfaceBase

	Border:      lipgloss.Color("183"), // Light violet
	BorderFocus: lipgloss.Color("91"),  // Velvet Orchid
	Success:     lipgloss.Color("28"),
	Error:       lipgloss.Color("160"),
	Warning:     lipgloss.Color("130"),

	SelectionFg: lipgloss.Color("255"), // Ghost White
	SelectionBg: lipgloss.Color("91"),  // Velvet Orchid

	ThermalCold:     lipgloss.Color("26"),  // Darker blue for light bg
	ThermalCool:     lipgloss.Color("32"),  // Medium blue
	ThermalWarm:     lipgloss.Color("28"),  // Forest green
	ThermalHot:      lipgloss.Color("172"), // Orange
	ThermalCritical: lipgloss.Color("166"), // Dark orange
	ThermalThrottle: lipgloss.Color("160"), // Red

	MetricGPUClock:   lipgloss.Color("32"),
	MetricCPUFreq:    lipgloss.Color("91"),
	MetricDLLCurrent: lipgloss.Color("26"),
	MetricDLLUpdate:  lipgloss.Color("172"),
	MetricDLLMissing: lipgloss.Color("243"),
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
	Muted    lipgloss.Style
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

	s.Muted = lipgloss.NewStyle().
		Foreground(t.TextMuted)

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

// CLI color helper styles — immutable, computed once from DefaultTheme.
var (
	cliPrimaryStyle   = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)
	cliSecondaryStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary)
	cliDimStyle       = lipgloss.NewStyle().Foreground(DefaultTheme.TextDim)
	cliSuccessStyle   = lipgloss.NewStyle().Foreground(DefaultTheme.Success)
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

func CLIAccent(text string) string {
	return cliAccentStyle.Render(text)
}
