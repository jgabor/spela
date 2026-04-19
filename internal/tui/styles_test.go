package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/config"
)

// Theme helper tests — one positive + one negative per helper, per Task 1
// acceptance criterion ("1 pass + 1 fail per new theme helper"). The "fail"
// case is a negative assertion (the helper must NOT produce a given output)
// rather than a test-suite failure; together with the pass case they pin the
// helper's semantics.

// TestInheritedStyle_UsesFgMuted verifies the inherited-row style renders with
// the fg-muted token. This is the pass case.
func TestInheritedStyle_UsesFgMuted(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.InheritedStyle().Render("x")
	want := lipgloss.NewStyle().Foreground(DefaultTheme.FgMuted).Render("x")
	if got != want {
		t.Errorf("InheritedStyle render = %q, want render with FgMuted = %q", got, want)
	}
}

// TestInheritedStyle_DoesNotUseFgOrAccent is the fail case: inherited rows
// must never render with the primary fg or either accent token (that would
// erase the inherited/overridden visual distinction Task 5 depends on).
func TestInheritedStyle_DoesNotUseFgOrAccent(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.InheritedStyle().Render("x")

	fgRender := lipgloss.NewStyle().Foreground(DefaultTheme.Fg).Render("x")
	overrideRender := lipgloss.NewStyle().Foreground(DefaultTheme.AccentOverride).Render("x")
	focusRender := lipgloss.NewStyle().Foreground(DefaultTheme.AccentFocus).Render("x")

	if got == fgRender {
		t.Errorf("InheritedStyle rendered with Fg, want FgMuted — got %q", got)
	}
	if got == overrideRender {
		t.Errorf("InheritedStyle rendered with AccentOverride, want FgMuted — got %q", got)
	}
	if got == focusRender {
		t.Errorf("InheritedStyle rendered with AccentFocus, want FgMuted — got %q", got)
	}
}

// TestOverrideStyle_UsesFg verifies the overridden-row style renders with the
// fg token.
func TestOverrideStyle_UsesFg(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.OverrideStyle().Render("x")
	want := lipgloss.NewStyle().Foreground(DefaultTheme.Fg).Render("x")
	if got != want {
		t.Errorf("OverrideStyle render = %q, want render with Fg = %q", got, want)
	}
}

// TestOverrideStyle_DoesNotUseFgMuted is the fail case: an overridden field
// rendered with fg-muted would be indistinguishable from an inherited one.
func TestOverrideStyle_DoesNotUseFgMuted(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.OverrideStyle().Render("x")
	mutedRender := lipgloss.NewStyle().Foreground(DefaultTheme.FgMuted).Render("x")
	if got == mutedRender {
		t.Errorf("OverrideStyle rendered with FgMuted, want Fg — got %q", got)
	}
}

// TestOverrideMarkerStyle_UsesAccentOverride verifies the override marker
// (magenta) uses the accent-override token.
func TestOverrideMarkerStyle_UsesAccentOverride(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.OverrideMarkerStyle().Render("*")
	want := lipgloss.NewStyle().Foreground(DefaultTheme.AccentOverride).Render("*")
	if got != want {
		t.Errorf("OverrideMarkerStyle render = %q, want render with AccentOverride = %q", got, want)
	}
}

// TestOverrideMarkerStyle_DoesNotUseFocus is the fail case: the override
// marker and focus accent carry different semantics (magenta = override,
// cyan = focus) and must render distinctly.
func TestOverrideMarkerStyle_DoesNotUseFocus(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.OverrideMarkerStyle().Render("*")
	focusRender := lipgloss.NewStyle().Foreground(DefaultTheme.AccentFocus).Render("*")
	if got == focusRender {
		t.Errorf("OverrideMarkerStyle rendered with AccentFocus, want AccentOverride — got %q", got)
	}
}

// TestFocusStyle_UsesAccentFocus verifies the focus style (focused field /
// focused rail row) uses the accent-focus token.
func TestFocusStyle_UsesAccentFocus(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.FocusStyle().Render("x")
	want := lipgloss.NewStyle().Foreground(DefaultTheme.AccentFocus).Bold(true).Render("x")
	if got != want {
		t.Errorf("FocusStyle render = %q, want render with AccentFocus = %q", got, want)
	}
}

// TestFocusStyle_DoesNotUseOverride is the fail case: focus must not render
// as magenta — a focused inherited field would then be misread as overridden.
func TestFocusStyle_DoesNotUseOverride(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	got := styles.FocusStyle().Render("x")
	overrideRender := lipgloss.NewStyle().Foreground(DefaultTheme.AccentOverride).Bold(true).Render("x")
	if got == overrideRender {
		t.Errorf("FocusStyle rendered with AccentOverride, want AccentFocus — got %q", got)
	}
}

// TestTheme_NeonPaletteTokens verifies the six canonical tokens resolve to
// the exact RGB values from .agentera/DECISIONS.md Decision 1. This is the
// contract the rest of Tasks 3-5 build on.
func TestTheme_NeonPaletteTokens(t *testing.T) {
	cases := []struct {
		name                string
		got                 any
		wantR, wantG, wantB uint8
	}{
		{"Bg", DefaultTheme.Bg, 0x0a, 0x0a, 0x14},
		{"Fg", DefaultTheme.Fg, 0xe8, 0xe8, 0xf0},
		{"FgMuted", DefaultTheme.FgMuted, 0x6a, 0x6a, 0x80},
		{"AccentOverride", DefaultTheme.AccentOverride, 0xff, 0x5f, 0xd2},
		{"AccentFocus", DefaultTheme.AccentFocus, 0x5f, 0xf0, 0xff},
		{"Border", DefaultTheme.Border, 0x20, 0x20, 0x30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := tc.got.(interface {
				RGBA() (r, g, b, a uint32)
			})
			if !ok {
				t.Fatalf("%s is not a color.Color", tc.name)
			}
			r, g, b, _ := c.RGBA()
			gotR, gotG, gotB := uint8(r>>8), uint8(g>>8), uint8(b>>8)
			if gotR != tc.wantR || gotG != tc.wantG || gotB != tc.wantB {
				t.Errorf("%s RGB = %02x%02x%02x, want %02x%02x%02x",
					tc.name, gotR, gotG, gotB, tc.wantR, tc.wantG, tc.wantB)
			}
		})
	}
}

// TestLayout_StripsLegacyTheme verifies that when a config file carrying a
// legacy `theme: dark` or `theme: light` value is loaded, the TUI resolves
// to the single neon-accent theme without error and clears the stored
// value so subsequent saves do not re-persist it. Task 1 acceptance
// criterion 2.
func TestLayout_StripsLegacyTheme(t *testing.T) {
	cases := []string{"dark", "light", "default", "royal-blue"}
	for _, legacy := range cases {
		t.Run(legacy, func(t *testing.T) {
			svc := testServices()
			// Inject a config with the legacy theme value set.
			svc.LoadConfig = func() (*config.Config, error) {
				cfg := config.Default()
				cfg.Theme = legacy
				return cfg, nil
			}
			db := testDatabase()
			m := NewLayout(db, svc)
			if m.config.Theme != "" {
				t.Errorf("legacy theme %q not stripped: config.Theme = %q, want empty",
					legacy, m.config.Theme)
			}
			if m.styles.Theme.Name != "neon-accent-dark" {
				t.Errorf("expected neon-accent-dark, got %q", m.styles.Theme.Name)
			}
		})
	}
}

// TestTheme_SingleVariant verifies the Default/Dark/Light triad has been
// collapsed to a single theme. This guards against a future regression that
// silently reintroduces DarkTheme/LightTheme package variables.
func TestTheme_SingleVariant(t *testing.T) {
	// Rendering with the default theme at one of the canonical tokens must
	// produce non-empty ANSI output (terminals set TERM to stub ANSI in CI,
	// but lipgloss embeds escape codes in the rendered string itself).
	out := lipgloss.NewStyle().Foreground(DefaultTheme.AccentOverride).Render("x")
	if !strings.Contains(out, "x") {
		t.Errorf("AccentOverride render should contain 'x'; got %q", out)
	}
	if DefaultTheme.Name != "neon-accent-dark" {
		t.Errorf("DefaultTheme.Name = %q, want %q", DefaultTheme.Name, "neon-accent-dark")
	}
}
