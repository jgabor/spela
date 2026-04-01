package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// sparklineRunes maps normalized [0..7] intensity to eighth-block characters.
var sparklineRunes = [8]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// leftBlockRunes maps sub-character eighths for gauge partial fill.
// Index 0 is empty (space), index 8 is full block.
var leftBlockRunes = [9]rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

const (
	gaugeEmpty = '░' // U+2591 light shade — used for unfilled gauge region
	gaugeFull  = '█' // U+2588 full block — used for filled gauge region
)

// RenderSparkline renders a thermally-colored sparkline from a slice of values.
// Each character uses eighth-block characters (▁▂▃▄▅▆▇█) and is individually
// colored via its thermal gradient position within [min, max].
//
// If len(values) < width, baseline characters (▁) are left-padded.
// If len(values) > width, only the most recent `width` values are shown.
// Width <= 0 returns an empty string.
func RenderSparkline(values []float64, width int, min, max float64, theme *Theme) string {
	if width <= 0 {
		return ""
	}

	// Determine which values to render, with left-padding if needed.
	padCount := 0
	displayValues := values
	if len(values) > width {
		displayValues = values[len(values)-width:]
	} else if len(values) < width {
		padCount = width - len(values)
	}

	// Build the raw rune string and collect per-character styles.
	var raw strings.Builder
	raw.Grow(width * 3) // UTF-8 block chars are 3 bytes each
	ranges := make([]lipgloss.Range, 0, width)

	// Left-pad with baseline characters in the coldest thermal color.
	coldStyle := lipgloss.NewStyle().Foreground(theme.ThermalCold)
	for i := range padCount {
		raw.WriteRune(sparklineRunes[0])
		ranges = append(ranges, lipgloss.NewRange(i, i+1, coldStyle))
	}

	// Render each value as a sparkline character.
	for i, value := range displayValues {
		ratio := normalizeAndClamp(value, min, max)
		var runeIndex int
		switch {
		case ratio <= 0:
			runeIndex = 0
		case ratio >= 1:
			runeIndex = 7
		default:
			runeIndex = int(ratio * 7)
		}

		raw.WriteRune(sparklineRunes[runeIndex])

		thermalColor := ThermalColor(value, min, max, theme)
		style := lipgloss.NewStyle().Foreground(thermalColor)
		pos := padCount + i
		ranges = append(ranges, lipgloss.NewRange(pos, pos+1, style))
	}

	return lipgloss.StyleRanges(raw.String(), ranges...)
}

// RenderGauge renders a block-character progress bar with sub-character
// precision using left-block characters: [████████░░░░░░░] 53%
//
// The filled portion uses thermal coloring based on the value's position
// within [min, max]. The empty portion uses theme.TextMuted.
// Width refers to the bar interior only (excluding brackets and percentage).
// Width <= 0 returns the percentage text only.
func RenderGauge(value, min, max float64, width int, theme *Theme) string {
	percentage := formatPercentage(value, min, max)
	if width <= 0 {
		return percentage
	}

	bar := renderGaugeBar(value, min, max, width, theme)
	return "[" + bar + "] " + percentage
}

// RenderGaugeMini renders a compact gauge without brackets or percentage text.
// Used in compact density mode. Width <= 0 returns an empty string.
func RenderGaugeMini(value, min, max float64, width int, theme *Theme) string {
	if width <= 0 {
		return ""
	}
	return renderGaugeBar(value, min, max, width, theme)
}

// renderGaugeBar builds the interior of a gauge (filled + partial + empty).
func renderGaugeBar(value, min, max float64, width int, theme *Theme) string {
	ratio := normalizeAndClamp(value, min, max)

	totalEighths := int(ratio * float64(width) * 8)
	fullBlocks := totalEighths / 8
	partialIndex := totalEighths % 8
	emptyBlocks := width - fullBlocks
	if partialIndex > 0 {
		emptyBlocks--
	}
	// Clamp: if fullBlocks fills the entire width, no partial or empty.
	if fullBlocks >= width {
		fullBlocks = width
		partialIndex = 0
		emptyBlocks = 0
	}

	thermalColor := ThermalColor(value, min, max, theme)
	filledStyle := lipgloss.NewStyle().Foreground(thermalColor)
	emptyStyle := lipgloss.NewStyle().Foreground(theme.TextMuted)

	var bar strings.Builder
	bar.Grow(width * 3)

	// Filled portion.
	filled := strings.Repeat(string(gaugeFull), fullBlocks)
	if partialIndex > 0 {
		filled += string(leftBlockRunes[partialIndex])
	}
	bar.WriteString(filledStyle.Render(filled))

	// Empty portion.
	if emptyBlocks > 0 {
		empty := strings.Repeat(string(gaugeEmpty), emptyBlocks)
		bar.WriteString(emptyStyle.Render(empty))
	}

	return bar.String()
}

// formatPercentage computes the display percentage string.
func formatPercentage(value, min, max float64) string {
	ratio := normalizeAndClamp(value, min, max)
	return fmt.Sprintf("%.0f%%", ratio*100)
}
