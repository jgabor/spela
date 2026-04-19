package tui

import (
	"image/color"
	"testing"
)

// toRGBA is a test helper that converts any color.Color to color.RGBA for comparison.
func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

// colorsEqual compares two colors by their RGBA representations with a tolerance
// of +/- 1 per channel to account for rounding in the 16-to-8-bit conversion.
func colorsEqual(a, b color.Color) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return absDiff(ar>>8, br>>8) <= 1 &&
		absDiff(ag>>8, bg>>8) <= 1 &&
		absDiff(ab>>8, bb>>8) <= 1
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// allThemes returns every built-in theme for parametric testing. The
// Default/Dark/Light triad collapsed into a single neon-accent dark theme
// per .agentera/DECISIONS.md Decision 1; the helper is retained so thermal
// tests stay parametric if additional palettes are introduced later.
func allThemes() []Theme {
	return []Theme{DefaultTheme}
}

func TestThermalColorExactStopBoundaries(t *testing.T) {
	for _, theme := range allThemes() {
		t.Run(theme.Name, func(t *testing.T) {
			// Map each stop ratio to its expected theme color.
			expectations := []struct {
				ratio    float64
				expected color.Color
				label    string
			}{
				{0.00, theme.ThermalCold, "ThermalCold (0%)"},
				{0.30, theme.ThermalCool, "ThermalCool (30%)"},
				{0.50, theme.ThermalWarm, "ThermalWarm (50%)"},
				{0.70, theme.ThermalHot, "ThermalHot (70%)"},
				{0.85, theme.ThermalCritical, "ThermalCritical (85%)"},
				{1.00, theme.ThermalThrottle, "ThermalThrottle (100%)"},
			}

			for _, expect := range expectations {
				// Use min=0, max=100 so the ratio equals value/100.
				value := expect.ratio * 100.0
				got := ThermalColor(value, 0, 100, &theme)

				if !colorsEqual(got, expect.expected) {
					t.Errorf("%s: ThermalColor(%v, 0, 100) = %v, want %v",
						expect.label, value, toRGBA(got), toRGBA(expect.expected))
				}
			}
		})
	}
}

func TestThermalColorBelowMinClampsToThermalCold(t *testing.T) {
	for _, theme := range allThemes() {
		t.Run(theme.Name, func(t *testing.T) {
			got := ThermalColor(-50, 0, 100, &theme)
			if !colorsEqual(got, theme.ThermalCold) {
				t.Errorf("below-min: ThermalColor(-50, 0, 100) = %v, want ThermalCold %v",
					toRGBA(got), toRGBA(theme.ThermalCold))
			}
		})
	}
}

func TestThermalColorAboveMaxClampsToThermalThrottle(t *testing.T) {
	for _, theme := range allThemes() {
		t.Run(theme.Name, func(t *testing.T) {
			got := ThermalColor(200, 0, 100, &theme)
			if !colorsEqual(got, theme.ThermalThrottle) {
				t.Errorf("above-max: ThermalColor(200, 0, 100) = %v, want ThermalThrottle %v",
					toRGBA(got), toRGBA(theme.ThermalThrottle))
			}
		})
	}
}

func TestThermalColorMidRangeInterpolation(t *testing.T) {
	// 15% is halfway between Cold (0%) and Cool (30%).
	// The result must not equal either stop exactly (unless they happen to be identical).
	for _, theme := range allThemes() {
		t.Run(theme.Name, func(t *testing.T) {
			got := ThermalColor(15, 0, 100, &theme)

			coldRGBA := toRGBA(theme.ThermalCold)
			coolRGBA := toRGBA(theme.ThermalCool)
			gotRGBA := toRGBA(got)

			// Verify each channel falls between the two stop colors.
			assertBetween(t, "R", gotRGBA.R, coldRGBA.R, coolRGBA.R)
			assertBetween(t, "G", gotRGBA.G, coldRGBA.G, coolRGBA.G)
			assertBetween(t, "B", gotRGBA.B, coldRGBA.B, coolRGBA.B)
		})
	}
}

// assertBetween checks that v is between a and b (inclusive, regardless of order).
// Uses int arithmetic to avoid uint8 overflow at boundary values (0, 255).
func assertBetween(t *testing.T, channel string, v, a, b uint8) {
	t.Helper()
	vi, ai, bi := int(v), int(a), int(b)
	lo, hi := ai, bi
	if lo > hi {
		lo, hi = hi, lo
	}
	// Allow +/- 1 for rounding.
	if vi < lo-1 || vi > hi+1 {
		t.Errorf("channel %s: %d not between %d and %d", channel, vi, lo, hi)
	}
}

func TestThermalColorMinEqualsMax(t *testing.T) {
	// When min == max the function should not panic and should return ThermalCold
	// (the normalizeAndClamp function returns 0.0 in this case).
	for _, theme := range allThemes() {
		t.Run(theme.Name, func(t *testing.T) {
			got := ThermalColor(50, 50, 50, &theme)
			if !colorsEqual(got, theme.ThermalCold) {
				t.Errorf("min==max: ThermalColor(50, 50, 50) = %v, want ThermalCold %v",
					toRGBA(got), toRGBA(theme.ThermalCold))
			}
		})
	}
}

func TestThermalColorMinGreaterThanMax(t *testing.T) {
	// When min > max, treat identically to min == max: clamp to cold.
	for _, theme := range allThemes() {
		t.Run(theme.Name, func(t *testing.T) {
			got := ThermalColor(50, 100, 0, &theme)
			if !colorsEqual(got, theme.ThermalCold) {
				t.Errorf("min>max: ThermalColor(50, 100, 0) = %v, want ThermalCold %v",
					toRGBA(got), toRGBA(theme.ThermalCold))
			}
		})
	}
}

func TestThermalStyleReturnsValidStyle(t *testing.T) {
	for _, theme := range allThemes() {
		t.Run(theme.Name, func(t *testing.T) {
			style := ThermalStyle(60, 0, 100, &theme)
			rendered := style.Render("60%")
			if rendered == "" {
				t.Error("ThermalStyle rendered empty string")
			}
		})
	}
}

func TestNormalizeAndClamp(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{"midpoint", 50, 0, 100, 0.5},
		{"at min", 0, 0, 100, 0.0},
		{"at max", 100, 0, 100, 1.0},
		{"below min", -10, 0, 100, 0.0},
		{"above max", 150, 0, 100, 1.0},
		{"min equals max", 50, 50, 50, 0.0},
		{"min greater than max", 50, 100, 0, 0.0},
		{"quarter point", 25, 0, 100, 0.25},
		{"custom range", 75, 50, 100, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAndClamp(tt.value, tt.min, tt.max)
			if got != tt.expected {
				t.Errorf("normalizeAndClamp(%v, %v, %v) = %v, want %v",
					tt.value, tt.min, tt.max, got, tt.expected)
			}
		})
	}
}

func TestLerpColor(t *testing.T) {
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// Lerp at 0.0 should return black.
	gotZero := toRGBA(lerpColor(black, white, 0.0))
	if gotZero.R != 0 || gotZero.G != 0 || gotZero.B != 0 {
		t.Errorf("lerpColor(black, white, 0.0) = %v, want black", gotZero)
	}

	// Lerp at 1.0 should return white.
	gotOne := toRGBA(lerpColor(black, white, 1.0))
	if gotOne.R != 255 || gotOne.G != 255 || gotOne.B != 255 {
		t.Errorf("lerpColor(black, white, 1.0) = %v, want white", gotOne)
	}

	// Lerp at 0.5 should return mid-gray (~127 or 128).
	gotHalf := toRGBA(lerpColor(black, white, 0.5))
	if gotHalf.R < 126 || gotHalf.R > 129 {
		t.Errorf("lerpColor(black, white, 0.5) R=%d, want ~127", gotHalf.R)
	}
}

func TestThermalColorCustomRange(t *testing.T) {
	// Test with a real-world GPU temperature range: 30-95 degrees.
	theme := DefaultTheme

	// At 30 degrees (min), should be ThermalCold.
	gotCold := ThermalColor(30, 30, 95, &theme)
	if !colorsEqual(gotCold, theme.ThermalCold) {
		t.Errorf("temp=30: got %v, want ThermalCold %v",
			toRGBA(gotCold), toRGBA(theme.ThermalCold))
	}

	// At 95 degrees (max), should be ThermalThrottle.
	gotHot := ThermalColor(95, 30, 95, &theme)
	if !colorsEqual(gotHot, theme.ThermalThrottle) {
		t.Errorf("temp=95: got %v, want ThermalThrottle %v",
			toRGBA(gotHot), toRGBA(theme.ThermalThrottle))
	}

	// At 62.5 degrees (50% of range), should be ThermalWarm.
	midTemp := 30 + 0.5*(95-30)
	gotWarm := ThermalColor(midTemp, 30, 95, &theme)
	if !colorsEqual(gotWarm, theme.ThermalWarm) {
		t.Errorf("temp=%.1f: got %v, want ThermalWarm %v",
			midTemp, toRGBA(gotWarm), toRGBA(theme.ThermalWarm))
	}
}
