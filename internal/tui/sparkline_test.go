package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

var testTheme = &DefaultTheme

// ---------------------------------------------------------------------------
// RenderSparkline tests
// ---------------------------------------------------------------------------

func TestRenderSparkline_ExactWidthMatchesSamples(t *testing.T) {
	values := make([]float64, 20)
	for i := range values {
		values[i] = float64(i) * 5 // 0..95
	}
	out := RenderSparkline(values, 20, 0, 100, testTheme)
	if w := lipgloss.Width(out); w != 20 {
		t.Errorf("expected visible width 20, got %d", w)
	}
}

func TestRenderSparkline_EmptyValues(t *testing.T) {
	out := RenderSparkline(nil, 10, 0, 100, testTheme)
	if w := lipgloss.Width(out); w != 10 {
		t.Errorf("expected visible width 10 for empty buffer, got %d", w)
	}
	// All characters should be baseline (▁) even though styled.
	raw := stripANSI(out)
	for _, r := range raw {
		if r != '▁' {
			t.Errorf("expected all baseline chars, found %c", r)
			break
		}
	}
}

func TestRenderSparkline_SingleSamplePadsLeft(t *testing.T) {
	out := RenderSparkline([]float64{50}, 8, 0, 100, testTheme)
	if w := lipgloss.Width(out); w != 8 {
		t.Errorf("expected visible width 8, got %d", w)
	}
}

func TestRenderSparkline_ValueExceedingMax(t *testing.T) {
	// Must not panic and should cap at full block.
	out := RenderSparkline([]float64{200}, 1, 0, 100, testTheme)
	raw := stripANSI(out)
	if !strings.ContainsRune(raw, '█') {
		t.Errorf("value exceeding max should render as full block, got %q", raw)
	}
}

func TestRenderSparkline_ValuesBelowMin(t *testing.T) {
	out := RenderSparkline([]float64{-10, -20}, 2, 0, 100, testTheme)
	raw := stripANSI(out)
	for _, r := range raw {
		if r != '▁' {
			t.Errorf("values below min should render as baseline, found %c", r)
			break
		}
	}
}

func TestRenderSparkline_WidthZero(t *testing.T) {
	out := RenderSparkline([]float64{50}, 0, 0, 100, testTheme)
	if out != "" {
		t.Errorf("width 0 should return empty string, got %q", out)
	}
}

func TestRenderSparkline_AllValuesAtMax(t *testing.T) {
	values := []float64{100, 100, 100, 100, 100}
	out := RenderSparkline(values, 5, 0, 100, testTheme)
	raw := stripANSI(out)
	for _, r := range raw {
		if r != '█' {
			t.Errorf("all-max values should render as full blocks, found %c", r)
			break
		}
	}
}

func TestRenderSparkline_MoreValuesThanWidth(t *testing.T) {
	// Should use only the most recent `width` values.
	values := []float64{0, 0, 0, 100, 100, 100}
	out := RenderSparkline(values, 3, 0, 100, testTheme)
	if w := lipgloss.Width(out); w != 3 {
		t.Errorf("expected visible width 3, got %d", w)
	}
	raw := stripANSI(out)
	for _, r := range raw {
		if r != '█' {
			t.Errorf("most recent 3 values are all 100; expected full blocks, found %c", r)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// RenderGauge tests
// ---------------------------------------------------------------------------

func TestRenderGauge_MidValue(t *testing.T) {
	out := RenderGauge(53, 0, 100, 12, testTheme)
	if !strings.Contains(out, "53%") {
		t.Errorf("gauge should contain percentage text, got %q", out)
	}
	if !strings.HasPrefix(stripANSI(out), "[") || !strings.Contains(stripANSI(out), "]") {
		t.Errorf("gauge should have brackets, got %q", stripANSI(out))
	}
	// Bar interior should be exactly 12 characters wide.
	raw := stripANSI(out)
	barStart := strings.Index(raw, "[") + 1
	barEnd := strings.Index(raw, "]")
	barContent := raw[barStart:barEnd]
	if w := lipgloss.Width(barContent); w != 12 {
		t.Errorf("bar interior should be 12 wide, got %d", w)
	}
}

func TestRenderGauge_ZeroPercent(t *testing.T) {
	out := RenderGauge(0, 0, 100, 10, testTheme)
	if !strings.Contains(out, "0%") {
		t.Errorf("0%% gauge should contain '0%%', got %q", out)
	}
	raw := stripANSI(out)
	barStart := strings.Index(raw, "[") + 1
	barEnd := strings.Index(raw, "]")
	barContent := raw[barStart:barEnd]
	// All should be empty characters.
	for _, r := range barContent {
		if r != '░' && r != ' ' {
			t.Errorf("0%% gauge should be all empty, found %c", r)
			break
		}
	}
}

func TestRenderGauge_HundredPercent(t *testing.T) {
	out := RenderGauge(100, 0, 100, 10, testTheme)
	if !strings.Contains(out, "100%") {
		t.Errorf("100%% gauge should contain '100%%', got %q", out)
	}
	raw := stripANSI(out)
	barStart := strings.Index(raw, "[") + 1
	barEnd := strings.Index(raw, "]")
	barContent := raw[barStart:barEnd]
	for _, r := range barContent {
		if r != '█' {
			t.Errorf("100%% gauge should be all full blocks, found %c (U+%04X)", r, r)
			break
		}
	}
}

func TestRenderGauge_WidthZero(t *testing.T) {
	out := RenderGauge(50, 0, 100, 0, testTheme)
	// Should return just the percentage.
	if !strings.Contains(out, "50%") {
		t.Errorf("width-0 gauge should return percentage, got %q", out)
	}
	if strings.Contains(out, "[") {
		t.Errorf("width-0 gauge should have no brackets, got %q", out)
	}
}

func TestRenderGauge_ContainsPercentage(t *testing.T) {
	out := RenderGauge(75, 0, 100, 8, testTheme)
	if !strings.Contains(out, "75%") {
		t.Errorf("gauge should contain '75%%', got %q", out)
	}
}

// ---------------------------------------------------------------------------
// RenderGaugeMini tests
// ---------------------------------------------------------------------------

func TestRenderGaugeMini_NoBracketsOrPercentage(t *testing.T) {
	out := RenderGaugeMini(50, 0, 100, 10, testTheme)
	raw := stripANSI(out)
	if strings.Contains(raw, "[") || strings.Contains(raw, "]") || strings.Contains(raw, "%") {
		t.Errorf("mini gauge should have no brackets or percentage, got %q", raw)
	}
}

func TestRenderGaugeMini_WidthZero(t *testing.T) {
	out := RenderGaugeMini(50, 0, 100, 0, testTheme)
	if out != "" {
		t.Errorf("mini gauge width 0 should return empty, got %q", out)
	}
}

func TestRenderGaugeMini_CorrectVisibleWidth(t *testing.T) {
	out := RenderGaugeMini(50, 0, 100, 15, testTheme)
	if w := lipgloss.Width(out); w != 15 {
		t.Errorf("mini gauge should be 15 wide, got %d", w)
	}
}

// ---------------------------------------------------------------------------
// MetricsBuffer tests
// ---------------------------------------------------------------------------

func TestMetricsBuffer_PushLessThanCapacity(t *testing.T) {
	buf := NewMetricsBuffer(10)
	buf.Push(1.0)
	buf.Push(2.0)
	buf.Push(3.0)

	values := buf.Values()
	expected := []float64{1.0, 2.0, 3.0}
	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(values))
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestMetricsBuffer_PushMoreThanCapacity(t *testing.T) {
	buf := NewMetricsBuffer(3)
	for i := range 7 {
		buf.Push(float64(i))
	}
	// Should have [4, 5, 6] — the last 3 values.
	values := buf.Values()
	expected := []float64{4.0, 5.0, 6.0}
	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(values))
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestMetricsBuffer_PushExactlyCapacity(t *testing.T) {
	buf := NewMetricsBuffer(5)
	for i := range 5 {
		buf.Push(float64(i + 10))
	}
	values := buf.Values()
	expected := []float64{10, 11, 12, 13, 14}
	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(values))
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %f, want %f", i, v, expected[i])
		}
	}
	if buf.Len() != 5 {
		t.Errorf("Len() = %d, want 5", buf.Len())
	}
}

func TestMetricsBuffer_EmptyBuffer(t *testing.T) {
	buf := NewMetricsBuffer(5)
	if buf.Len() != 0 {
		t.Errorf("empty buffer Len() = %d, want 0", buf.Len())
	}
	values := buf.Values()
	if values != nil {
		t.Errorf("empty buffer Values() should be nil, got %v", values)
	}
}

func TestMetricsBuffer_LastEmpty(t *testing.T) {
	buf := NewMetricsBuffer(5)
	_, ok := buf.Last()
	if ok {
		t.Error("Last() on empty buffer should return false")
	}
}

func TestMetricsBuffer_LastAfterPushes(t *testing.T) {
	buf := NewMetricsBuffer(5)
	buf.Push(42.0)
	buf.Push(99.0)

	val, ok := buf.Last()
	if !ok {
		t.Fatal("Last() should return true after pushes")
	}
	if val != 99.0 {
		t.Errorf("Last() = %f, want 99.0", val)
	}
}

func TestMetricsBuffer_LastAfterWrap(t *testing.T) {
	buf := NewMetricsBuffer(3)
	buf.Push(1)
	buf.Push(2)
	buf.Push(3)
	buf.Push(4) // wraps, overwrites 1

	val, ok := buf.Last()
	if !ok {
		t.Fatal("Last() should return true")
	}
	if val != 4 {
		t.Errorf("Last() = %f, want 4", val)
	}
}

func TestMetricsBuffer_Cap(t *testing.T) {
	buf := NewMetricsBuffer(42)
	if buf.Cap() != 42 {
		t.Errorf("Cap() = %d, want 42", buf.Cap())
	}
}

func TestMetricsBuffer_PanicOnZeroCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewMetricsBuffer(0) should panic")
		}
	}()
	NewMetricsBuffer(0)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stripANSI removes ANSI escape sequences to get raw visible text.
// This is intentionally simple — sufficient for block characters in tests.
func stripANSI(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
