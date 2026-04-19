package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme is the single neon-accent dark palette. All UI components consume
// theme tokens; no component hardcodes a hex value. The layout (Task 3),
// inheritance rendering (Task 5), and any future variant selection must
// read from this struct rather than introducing new colors.
type Theme struct {
	Name string

	// Canonical neon-accent tokens (source of truth for Tasks 3-5).
	Bg             color.Color // page background
	Fg             color.Color // primary foreground, overridden rows
	FgMuted        color.Color // inherited rows, hints, dim secondary text
	AccentOverride color.Color // magenta — override markers, section titles
	AccentFocus    color.Color // cyan — focus rings, focused borders, selection
	Border         color.Color // idle pane borders

	// Backwards-compat aliases — all resolve to canonical tokens above so the
	// rest of the TUI (header, sparklines, help bar, messagebar, modals) keeps
	// working while the shell/resource rewrite is in flight. Tasks 3-6 can
	// delete these once their consumers migrate to the canonical tokens.
	Primary     color.Color // alias AccentOverride (section titles)
	Secondary   color.Color // alias AccentFocus (DLSS group accents, etc.)
	Accent      color.Color // alias AccentOverride
	Text        color.Color // alias Fg
	TextPrimary color.Color // alias Fg
	TextDim     color.Color // alias FgMuted
	TextMuted   color.Color // alias FgMuted
	Background  color.Color // alias Bg
	BorderFocus color.Color // alias AccentFocus

	SurfaceBase      color.Color // alias Bg
	SurfaceRaised    color.Color // one step up from Bg
	SurfaceOverlay   color.Color // two steps up — modals
	SurfaceHighlight color.Color // three steps up — active row

	Success color.Color
	Error   color.Color
	Warning color.Color

	SelectionFg color.Color
	SelectionBg color.Color

	// Thermal gradient — six stops for temperature/load visualisation.
	// Preserved from prior theme because header/sparkline widgets consume
	// these directly. Tuned to sit visually inside the neon palette.
	ThermalCold     color.Color // idle, below 30%
	ThermalCool     color.Color // light load, 30-45%
	ThermalWarm     color.Color // normal, 45-65%
	ThermalHot      color.Color // elevated, 65-80%
	ThermalCritical color.Color // high, 80-90%
	ThermalThrottle color.Color // danger, 90%+

	// Metric-specific tokens for sparklines / status pills.
	MetricGPUClock   color.Color
	MetricCPUFreq    color.Color
	MetricDLLCurrent color.Color
	MetricDLLUpdate  color.Color
	MetricDLLMissing color.Color
}

// Neon-accent dark palette (per .agentera/DECISIONS.md Decision 1).
//
// Canonical hex values — these are the only hex literals in the TUI. Every
// component must reference a theme field, never a hex string.
const (
	hexBg             = "#0a0a14" // page background
	hexFg             = "#e8e8f0" // primary foreground / overridden
	hexFgMuted        = "#6a6a80" // inherited, dim
	hexAccentOverride = "#ff5fd2" // magenta — override marker
	hexAccentFocus    = "#5ff0ff" // cyan — focus / selection
	hexBorder         = "#202030" // idle pane border

	// Derived surface stops for modals and highlights. Kept close to bg so
	// the neon accents carry the visual weight, not the surfaces.
	hexSurfaceRaised    = "#12121e"
	hexSurfaceOverlay   = "#1a1a28"
	hexSurfaceHighlight = "#242438"

	// Semantic status colors tuned for the dark palette.
	hexSuccess = "#7ce38b"
	hexError   = "#ff6e6e"
	hexWarning = "#ffd866"

	// Thermal gradient stops, cool→hot, chosen to sit inside the neon palette.
	hexThermalCold     = "#5ff0ff" // cyan (same family as AccentFocus)
	hexThermalCool     = "#5fbaff"
	hexThermalWarm     = "#7ce38b"
	hexThermalHot      = "#ffd866"
	hexThermalCritical = "#ff9e5f"
	hexThermalThrottle = "#ff6e6e"
)

// DefaultTheme is the one and only TUI theme — neon-accent dark. The former
// Default/Dark/Light triad has been collapsed; legacy `theme: dark` /
// `theme: light` values in config.yaml are ignored at load and stripped on
// next save so they do not re-persist.
var DefaultTheme = Theme{
	Name: "neon-accent-dark",

	// Canonical tokens.
	Bg:             lipgloss.Color(hexBg),
	Fg:             lipgloss.Color(hexFg),
	FgMuted:        lipgloss.Color(hexFgMuted),
	AccentOverride: lipgloss.Color(hexAccentOverride),
	AccentFocus:    lipgloss.Color(hexAccentFocus),
	Border:         lipgloss.Color(hexBorder),

	// Aliases for legacy consumers (header, sparkline, help bar, modals, etc.).
	Primary:     lipgloss.Color(hexAccentOverride),
	Secondary:   lipgloss.Color(hexAccentFocus),
	Accent:      lipgloss.Color(hexAccentOverride),
	Text:        lipgloss.Color(hexFg),
	TextPrimary: lipgloss.Color(hexFg),
	TextDim:     lipgloss.Color(hexFgMuted),
	TextMuted:   lipgloss.Color(hexFgMuted),
	Background:  lipgloss.Color(hexBg),
	BorderFocus: lipgloss.Color(hexAccentFocus),

	SurfaceBase:      lipgloss.Color(hexBg),
	SurfaceRaised:    lipgloss.Color(hexSurfaceRaised),
	SurfaceOverlay:   lipgloss.Color(hexSurfaceOverlay),
	SurfaceHighlight: lipgloss.Color(hexSurfaceHighlight),

	Success: lipgloss.Color(hexSuccess),
	Error:   lipgloss.Color(hexError),
	Warning: lipgloss.Color(hexWarning),

	SelectionFg: lipgloss.Color(hexBg),
	SelectionBg: lipgloss.Color(hexAccentFocus),

	ThermalCold:     lipgloss.Color(hexThermalCold),
	ThermalCool:     lipgloss.Color(hexThermalCool),
	ThermalWarm:     lipgloss.Color(hexThermalWarm),
	ThermalHot:      lipgloss.Color(hexThermalHot),
	ThermalCritical: lipgloss.Color(hexThermalCritical),
	ThermalThrottle: lipgloss.Color(hexThermalThrottle),

	MetricGPUClock:   lipgloss.Color(hexAccentFocus),
	MetricCPUFreq:    lipgloss.Color(hexAccentOverride),
	MetricDLLCurrent: lipgloss.Color(hexAccentFocus),
	MetricDLLUpdate:  lipgloss.Color(hexThermalHot),
	MetricDLLMissing: lipgloss.Color(hexFgMuted),
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

// SetTheme changes the active theme and rebuilds all derived styles. With the
// single-theme collapse this is effectively a no-op in production, but the
// method is kept so tests can swap in alternate palettes.
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
		Foreground(t.Fg)

	s.Dim = lipgloss.NewStyle().
		Foreground(t.FgMuted)

	s.Muted = lipgloss.NewStyle().
		Foreground(t.FgMuted)

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
// Focused borders use AccentFocus (cyan); idle borders use Border.
func (s *Styles) BorderColor(focused bool) color.Color {
	if focused {
		return s.Theme.AccentFocus
	}
	return s.Theme.Border
}

// InheritedStyle returns the style for a profile field that inherits from the
// default profile — rendered with the fg-muted token and no override marker.
// Task 5 consumes this for per-field inheritance rendering.
func (s *Styles) InheritedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.Theme.FgMuted)
}

// OverrideStyle returns the style for a profile field that overrides the
// default — rendered with the fg token plus the accent-override marker color
// consumed via OverrideMarkerStyle.
// Task 5 consumes this for per-field inheritance rendering.
func (s *Styles) OverrideStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.Theme.Fg)
}

// OverrideMarkerStyle returns the style for the visual marker next to an
// overridden field (magenta, AccentOverride). Paired with OverrideStyle.
func (s *Styles) OverrideMarkerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.Theme.AccentOverride)
}

// FocusStyle returns the style for the currently focused field / row — uses
// the accent-focus token (cyan). Used by the shell rail and resource views.
func (s *Styles) FocusStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.Theme.AccentFocus).Bold(true)
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

// CLI color helper styles — immutable, computed once from DefaultTheme. With
// the single-theme collapse these resolve to canonical neon tokens:
//   - CLIPrimary  → AccentOverride (magenta)
//   - CLISecondary→ AccentFocus (cyan)
//   - CLIDim      → FgMuted
//   - CLISuccess  → Success
//   - CLIAccent   → AccentOverride
var (
	cliPrimaryStyle   = lipgloss.NewStyle().Foreground(DefaultTheme.AccentOverride)
	cliSecondaryStyle = lipgloss.NewStyle().Foreground(DefaultTheme.AccentFocus)
	cliDimStyle       = lipgloss.NewStyle().Foreground(DefaultTheme.FgMuted)
	cliSuccessStyle   = lipgloss.NewStyle().Foreground(DefaultTheme.Success)
	cliAccentStyle    = lipgloss.NewStyle().Foreground(DefaultTheme.AccentOverride)
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
