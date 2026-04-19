package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/overlay"
)

// MetricsResourceModel renders the Metrics resource pane — the relocation
// target from Task 6. It reuses the existing thermal gradient and sparkline
// renderers unchanged; the pane simply arranges them inside the resource
// body. The underlying MetricsBuffer / ThermalColor / RenderSparkline /
// RenderGauge primitives are shared with HeaderModel and their state-machine
// tests are not touched.
//
// Data source: this resource does not fetch metrics itself. HeaderModel
// already owns the 2-second tick loop and the rolling MetricsBuffer; the
// layout hands those buffers and the latest snapshot to MetricsResource each
// render. This keeps the single metrics polling loop in one place and
// preserves existing sparkline behaviour bit-for-bit.
type MetricsResourceModel struct {
	styles      *Styles
	gpuMetrics  *gpu.GPUMetrics
	cpuMetrics  *cpu.CPUMetrics
	alerts      []overlay.Alert
	tempBuffer  *MetricsBuffer
	utilBuffer  *MetricsBuffer
	powerBuffer *MetricsBuffer
	cpuBuffer   *MetricsBuffer
	width       int
	height      int
}

// NewMetricsResource constructs an empty Metrics pane. Buffers and metrics
// must be wired via SetData before rendering.
func NewMetricsResource(styles *Styles) MetricsResourceModel {
	return MetricsResourceModel{styles: styles}
}

// SetSize stores the pane dimensions so the sparkline width can adapt.
func (m *MetricsResourceModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetData attaches the latest metrics and rolling buffers. Called by the
// layout each render.
func (m MetricsResourceModel) SetData(
	g *gpu.GPUMetrics,
	c *cpu.CPUMetrics,
	alerts []overlay.Alert,
	temp, util, power, cpuBuf *MetricsBuffer,
) MetricsResourceModel {
	m.gpuMetrics = g
	m.cpuMetrics = c
	m.alerts = alerts
	m.tempBuffer = temp
	m.utilBuffer = util
	m.powerBuffer = power
	m.cpuBuffer = cpuBuf
	return m
}

// View renders the metrics pane. Layout is arranged so the thermal-coloured
// temperature and utilisation lines sit above the VRAM / RAM gauges, with
// any active alerts listed beneath. Sparkline widths scale with the
// available pane width.
func (m MetricsResourceModel) View(paneFocused bool) string {
	s := m.styles
	t := s.Theme

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.BorderColor(paneFocused)).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(s.Title.Render("Metrics"))
	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Live GPU and CPU telemetry. Sparklines and gauges share the header sample loop."))
	b.WriteString("\n\n")

	sparklineWidth := 30
	if m.width > 0 {
		sparklineWidth = max(10, min(40, m.width-30))
	}
	gaugeWidth := 20
	if m.width > 0 {
		gaugeWidth = max(8, min(24, m.width-20))
	}

	labelStyle := lipgloss.NewStyle().Foreground(t.TextDim)
	valueStyle := lipgloss.NewStyle().Foreground(t.Text)
	freqStyle := lipgloss.NewStyle().Foreground(t.MetricCPUFreq)

	// --- GPU block ---
	b.WriteString(m.sectionHeader("GPU"))
	b.WriteString("\n")
	if m.gpuMetrics != nil {
		g := m.gpuMetrics

		tempStyle := ThermalStyle(float64(g.Temperature), 30, 95, &t)
		powerRatio := g.PowerDraw
		if g.PowerLimit > 0 {
			powerRatio = g.PowerDraw / g.PowerLimit * 100
		}
		powerStyle := ThermalStyle(powerRatio, 0, 100, &t)

		// Temp line w/ sparkline
		line := labelStyle.Render("Temp   ") +
			tempStyle.Render(fmt.Sprintf("%d°C", g.Temperature))
		if m.tempBuffer != nil {
			line += "  " + RenderSparkline(m.tempBuffer.Values(), sparklineWidth, 30, 95, &t)
		}
		b.WriteString(line)
		b.WriteString("\n")

		// Utilisation line w/ sparkline
		line = labelStyle.Render("Util   ") +
			valueStyle.Render(fmt.Sprintf("%d%%", g.Utilization))
		if m.utilBuffer != nil {
			line += "  " + RenderSparkline(m.utilBuffer.Values(), sparklineWidth, 0, 100, &t)
		}
		b.WriteString(line)
		b.WriteString("\n")

		// Power line w/ sparkline
		line = labelStyle.Render("Power  ") +
			powerStyle.Render(fmt.Sprintf("%.0fW / %.0fW", g.PowerDraw, g.PowerLimit))
		if m.powerBuffer != nil {
			line += "  " + RenderSparkline(m.powerBuffer.Values(), sparklineWidth, 0, max100(g.PowerLimit), &t)
		}
		b.WriteString(line)
		b.WriteString("\n")

		// VRAM gauge
		vramGauge := RenderGauge(float64(g.MemoryUsed), 0, float64(g.MemoryTotal), gaugeWidth, &t)
		vramUsedGB := float64(g.MemoryUsed) / 1024.0
		vramTotalGB := float64(g.MemoryTotal) / 1024.0
		b.WriteString(labelStyle.Render("VRAM   "))
		b.WriteString(vramGauge)
		b.WriteString("  ")
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f / %.1f GB", vramUsedGB, vramTotalGB)))
		b.WriteString("\n")

		if g.GraphicsClock > 0 {
			b.WriteString(labelStyle.Render("Clock  "))
			b.WriteString(freqStyle.Render(fmt.Sprintf("%d MHz", g.GraphicsClock)))
			if g.MemoryClock > 0 {
				b.WriteString("   ")
				b.WriteString(labelStyle.Render("Mem  "))
				b.WriteString(freqStyle.Render(fmt.Sprintf("%d MHz", g.MemoryClock)))
			}
			b.WriteString("\n")
		}
		if g.FanSpeed > 0 {
			fanStyle := ThermalStyle(float64(g.FanSpeed), 0, 100, &t)
			b.WriteString(labelStyle.Render("Fan    "))
			b.WriteString(fanStyle.Render(fmt.Sprintf("%d%%", g.FanSpeed)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(labelStyle.Render("Temp   ") + valueStyle.Render("N/A"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// --- CPU block ---
	b.WriteString(m.sectionHeader("CPU"))
	b.WriteString("\n")
	if m.cpuMetrics != nil {
		c := m.cpuMetrics
		utilStyle := ThermalStyle(c.Utilization, 0, 100, &t)
		line := labelStyle.Render("Util   ") +
			utilStyle.Render(fmt.Sprintf("%.0f%%", c.Utilization))
		if m.cpuBuffer != nil {
			line += "  " + RenderSparkline(m.cpuBuffer.Values(), sparklineWidth, 0, 100, &t)
		}
		b.WriteString(line)
		b.WriteString("\n")

		b.WriteString(labelStyle.Render("Freq   "))
		b.WriteString(freqStyle.Render(fmt.Sprintf("%d MHz", c.AverageFrequency)))
		b.WriteString("\n")

		ramGauge := RenderGauge(float64(c.RAMUsedMB), 0, float64(c.RAMTotalMB), gaugeWidth, &t)
		ramUsedGB := float64(c.RAMUsedMB) / 1024.0
		ramTotalGB := float64(c.RAMTotalMB) / 1024.0
		b.WriteString(labelStyle.Render("RAM    "))
		b.WriteString(ramGauge)
		b.WriteString("  ")
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f / %.1f GB", ramUsedGB, ramTotalGB)))
		b.WriteString("\n")

		if c.Governor != "" {
			b.WriteString(labelStyle.Render("Governor "))
			b.WriteString(valueStyle.Render(string(c.Governor)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(labelStyle.Render("Util   ") + valueStyle.Render("N/A"))
		b.WriteString("\n")
	}

	// --- Alerts ---
	if len(m.alerts) > 0 {
		b.WriteString("\n")
		b.WriteString(m.sectionHeader("Alerts"))
		b.WriteString("\n")
		for _, a := range m.alerts {
			icon := "⚠"
			style := s.Warning
			if a.Severity == overlay.AlertCritical {
				icon = "✗"
				style = s.Error
			}
			b.WriteString(style.Render(fmt.Sprintf("%s %s", icon, alertLabel(&a))))
			b.WriteString("\n")
		}
	}

	return box.Render(b.String())
}

func (m MetricsResourceModel) sectionHeader(name string) string {
	return lipgloss.NewStyle().
		Foreground(m.styles.Theme.AccentOverride).
		Bold(true).
		Render(name)
}

// max100 returns v clamped to a minimum of 100 — sparkline ranges need a
// positive max even when PowerLimit is not yet known.
func max100(v float64) float64 {
	if v <= 0 {
		return 100
	}
	return v
}
