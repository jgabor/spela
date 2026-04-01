package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// thermalStop maps a normalized ratio position [0.0, 1.0] to a theme color accessor.
type thermalStop struct {
	ratio    float64
	getColor func(*Theme) color.Color
}

// thermalStops defines the six breakpoints of the thermal gradient.
// Positions are chosen so idle stays blue longest, then transitions
// rapidly through green/yellow/orange/red as load increases.
var thermalStops = []thermalStop{
	{0.00, func(t *Theme) color.Color { return t.ThermalCold }},
	{0.30, func(t *Theme) color.Color { return t.ThermalCool }},
	{0.50, func(t *Theme) color.Color { return t.ThermalWarm }},
	{0.70, func(t *Theme) color.Color { return t.ThermalHot }},
	{0.85, func(t *Theme) color.Color { return t.ThermalCritical }},
	{1.00, func(t *Theme) color.Color { return t.ThermalThrottle }},
}

// ThermalColor returns a color interpolated through the thermal gradient
// based on where value falls in the [min, max] range.
// Values below min clamp to ThermalCold; values above max clamp to ThermalThrottle.
func ThermalColor(value, min, max float64, t *Theme) color.Color {
	ratio := normalizeAndClamp(value, min, max)

	// Walk the stops to find the surrounding pair.
	for i := 1; i < len(thermalStops); i++ {
		if ratio <= thermalStops[i].ratio {
			lower := thermalStops[i-1]
			upper := thermalStops[i]

			segmentRange := upper.ratio - lower.ratio
			if segmentRange == 0 {
				return lower.getColor(t)
			}
			segmentRatio := (ratio - lower.ratio) / segmentRange
			return lerpColor(lower.getColor(t), upper.getColor(t), segmentRatio)
		}
	}

	// Fallback: fully clamped to the last stop.
	return thermalStops[len(thermalStops)-1].getColor(t)
}

// ThermalStyle returns a lipgloss.Style with foreground set to the thermal color
// for the given value within [min, max].
func ThermalStyle(value, min, max float64, t *Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ThermalColor(value, min, max, t))
}

// normalizeAndClamp maps value into [0.0, 1.0] relative to [min, max],
// clamping at both ends. When min >= max the result is 0.0 (cold).
func normalizeAndClamp(value, min, max float64) float64 {
	if min >= max {
		return 0.0
	}
	ratio := (value - min) / (max - min)
	if ratio < 0.0 {
		return 0.0
	}
	if ratio > 1.0 {
		return 1.0
	}
	return ratio
}

// lerpColor linearly interpolates between two colors in RGBA space.
// The ratio parameter should be in [0.0, 1.0] where 0.0 returns colorA
// and 1.0 returns colorB.
func lerpColor(colorA, colorB color.Color, ratio float64) color.Color {
	rA, gA, bA, _ := colorA.RGBA()
	rB, gB, bB, _ := colorB.RGBA()

	// RGBA() returns pre-multiplied 16-bit values; scale to 8-bit.
	return color.RGBA{
		R: lerpByte(rA, rB, ratio),
		G: lerpByte(gA, gB, ratio),
		B: lerpByte(bA, bB, ratio),
		A: 0xFF,
	}
}

// lerpByte interpolates between two 16-bit pre-multiplied channel values,
// returning an 8-bit result.
func lerpByte(a, b uint32, ratio float64) uint8 {
	a8 := float64(a >> 8)
	b8 := float64(b >> 8)
	return uint8(a8 + (b8-a8)*ratio)
}
