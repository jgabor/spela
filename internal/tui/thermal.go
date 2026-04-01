package tui

import "image/color"

// thermalStops defines the gradient breakpoints as (normalized position, color field).
// The gradient flows: cold → cool → warm → hot → critical → throttle.
var thermalStops = [6]float64{0.0, 0.2, 0.4, 0.6, 0.8, 1.0}

// normalizeAndClamp maps value into [0.0, 1.0] within [min, max].
// Values at or below min return 0; values at or above max return 1.
// If min == max, returns 0.
func normalizeAndClamp(value, min, max float64) float64 {
	if max <= min {
		return 0
	}
	ratio := (value - min) / (max - min)
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

// thermalColors returns the six gradient stop colors from the theme.
func thermalColors(theme *Theme) [6]color.Color {
	return [6]color.Color{
		theme.ThermalCold,
		theme.ThermalCool,
		theme.ThermalWarm,
		theme.ThermalHot,
		theme.ThermalCritical,
		theme.ThermalThrottle,
	}
}

// ThermalColor returns a color from the thermal gradient for a given value
// within [min, max]. The gradient interpolates across six stops from cold
// (blue) through warm (yellow/orange) to critical/throttle (red/magenta).
func ThermalColor(value, min, max float64, theme *Theme) color.Color {
	ratio := normalizeAndClamp(value, min, max)
	colors := thermalColors(theme)

	// Find which two stops we're between.
	for i := 0; i < len(thermalStops)-1; i++ {
		if ratio <= thermalStops[i+1] {
			segmentLength := thermalStops[i+1] - thermalStops[i]
			if segmentLength <= 0 {
				return colors[i]
			}
			t := (ratio - thermalStops[i]) / segmentLength
			return lerpColor(colors[i], colors[i+1], t)
		}
	}
	return colors[len(colors)-1]
}

// lerpColor linearly interpolates between two colors in RGBA space.
func lerpColor(a, b color.Color, t float64) color.Color {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return color.NRGBA{
		R: uint8((float64(ar)/257*(1-t) + float64(br)/257*t)),
		G: uint8((float64(ag)/257*(1-t) + float64(bg)/257*t)),
		B: uint8((float64(ab)/257*(1-t) + float64(bb)/257*t)),
		A: uint8((float64(aa)/257*(1-t) + float64(ba)/257*t)),
	}
}
