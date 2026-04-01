package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/overlay"
)

var logo = []string{
	"███████╗██████╗ ███████╗██╗      █████╗ ",
	"██╔════╝██╔══██╗██╔════╝██║     ██╔══██╗",
	"███████╗██████╔╝█████╗  ██║     ███████║",
	"╚════██║██╔═══╝ ██╔══╝  ██║     ██╔══██║",
	"███████║██║     ███████╗███████╗██║  ██║",
	"╚══════╝╚═╝     ╚══════╝╚══════╝╚═╝  ╚═╝",
}

type headerTickMsg struct{}

// metricsMsg delivers asynchronously fetched system metrics.
type metricsMsg struct {
	gpuMetrics *gpu.GPUMetrics
	cpuMetrics *cpu.CPUMetrics
	alerts     []overlay.Alert
}

type HeaderModel struct {
	styles     *Styles
	gpuMetrics *gpu.GPUMetrics
	cpuMetrics *cpu.CPUMetrics
	alerts     []overlay.Alert
	width      int

	// Rolling metrics buffers for sparkline rendering.
	tempBuffer  *MetricsBuffer
	utilBuffer  *MetricsBuffer
	powerBuffer *MetricsBuffer
	cpuBuffer   *MetricsBuffer
}

func NewHeader(styles *Styles) HeaderModel {
	return HeaderModel{
		styles:      styles,
		tempBuffer:  NewMetricsBuffer(20),
		utilBuffer:  NewMetricsBuffer(20),
		powerBuffer: NewMetricsBuffer(20),
		cpuBuffer:   NewMetricsBuffer(20),
	}
}

func (m *HeaderModel) SetWidth(width int) {
	m.width = width
}

func (m HeaderModel) Init() tea.Cmd {
	return tea.Batch(fetchMetrics(), tickHeader())
}

func tickHeader() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return headerTickMsg{}
	})
}

// fetchMetrics returns a command that reads GPU/CPU metrics off the main loop.
func fetchMetrics() tea.Cmd {
	return func() tea.Msg {
		gpuMetrics, _ := gpu.GetGPUMetrics()
		cpuMetrics, _ := cpu.GetCPUMetrics()
		var alerts []overlay.Alert
		if gpuMetrics != nil {
			input := overlay.AlertInput{
				Temperature:   gpuMetrics.Temperature,
				PowerDraw:     gpuMetrics.PowerDraw,
				PowerLimit:    gpuMetrics.PowerLimit,
				GraphicsClock: gpuMetrics.GraphicsClock,
				FanSpeed:      gpuMetrics.FanSpeed,
			}
			if r := gpuMetrics.ThrottleReasons; r != nil {
				input.ThrottleThermal = r.ThermalHardware || r.ThermalSoftware
				input.ThrottlePower = r.PowerCap || r.PowerBrake
			}
			alerts = overlay.Evaluate(input, overlay.DefaultThresholds())
		}
		return metricsMsg{gpuMetrics: gpuMetrics, cpuMetrics: cpuMetrics, alerts: alerts}
	}
}

func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case headerTickMsg:
		return m, fetchMetrics()
	case metricsMsg:
		m.gpuMetrics = msg.gpuMetrics
		m.cpuMetrics = msg.cpuMetrics
		m.alerts = msg.alerts
		if msg.gpuMetrics != nil {
			m.tempBuffer.Push(float64(msg.gpuMetrics.Temperature))
			m.utilBuffer.Push(float64(msg.gpuMetrics.Utilization))
			m.powerBuffer.Push(msg.gpuMetrics.PowerDraw)
		}
		if msg.cpuMetrics != nil {
			m.cpuBuffer.Push(msg.cpuMetrics.Utilization)
		}
		return m, tickHeader()
	}
	return m, nil
}

func (m HeaderModel) View() string {
	t := m.styles.Theme

	logoStyle := lipgloss.NewStyle().
		Foreground(t.Primary)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.TextDim)

	valueStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	freqStyle := lipgloss.NewStyle().
		Foreground(t.MetricCPUFreq)

	// Determine sparkline width based on available space.
	logoWidth := lipgloss.Width(logo[0])
	sparklineWidth := 20
	metricsAvail := m.width - logoWidth - 10 // 10 for padding/spacing
	if metricsAvail < 50 {
		sparklineWidth = 10
	}
	if metricsAvail < 35 {
		sparklineWidth = 0
	}

	gaugeWidth := 12

	// Build metrics lines
	var metricsLines []string

	// Line 1: GPU temp, sparkline, util, power, alert
	if m.gpuMetrics != nil {
		g := m.gpuMetrics

		tempStyle := ThermalStyle(float64(g.Temperature), 30, 95, &t)
		powerRatio := g.PowerDraw
		if g.PowerLimit > 0 {
			powerRatio = g.PowerDraw / g.PowerLimit * 100
		}
		powerStyle := ThermalStyle(powerRatio, 0, 100, &t)

		line := labelStyle.Render("GPU: ") +
			tempStyle.Render(fmt.Sprintf("%d°C", g.Temperature))

		if sparklineWidth > 0 {
			line += " " + RenderSparkline(m.tempBuffer.Values(), sparklineWidth, 30, 95, &t)
		}

		line += "  " +
			valueStyle.Render(fmt.Sprintf("%d%%", g.Utilization)) +
			" " +
			powerStyle.Render(fmt.Sprintf("%.0fW", g.PowerDraw))

		if highest := highestSeverityAlert(m.alerts); highest != nil {
			var icon string
			switch highest.Severity {
			case overlay.AlertCritical:
				icon = "✗"
			case overlay.AlertWarning:
				icon = "⚠"
			}
			if icon != "" {
				var alertColor float64
				if highest.Severity == overlay.AlertCritical {
					alertColor = 100
				} else {
					alertColor = 75
				}
				alertStyle := ThermalStyle(alertColor, 0, 100, &t)
				line += " " + alertStyle.Render(icon+" "+alertLabel(highest))
			}
		}

		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("GPU: ")+valueStyle.Render("N/A"))
	}

	// Line 2: VRAM gauge
	if m.gpuMetrics != nil {
		g := m.gpuMetrics
		vramUsedGB := float64(g.MemoryUsed) / 1024.0
		vramTotalGB := float64(g.MemoryTotal) / 1024.0
		gauge := RenderGauge(float64(g.MemoryUsed), 0, float64(g.MemoryTotal), gaugeWidth, &t)
		line := labelStyle.Render("VRAM: ") +
			gauge +
			"  " +
			valueStyle.Render(fmt.Sprintf("%.1f/%.1f GB", vramUsedGB, vramTotalGB))
		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("VRAM: ")+valueStyle.Render("N/A"))
	}

	// Line 3: CPU util, sparkline, freq
	if m.cpuMetrics != nil {
		c := m.cpuMetrics

		utilStyle := ThermalStyle(c.Utilization, 0, 100, &t)
		line := labelStyle.Render("CPU: ") +
			utilStyle.Render(fmt.Sprintf("%.0f%%", c.Utilization))

		if sparklineWidth > 0 {
			line += " " + RenderSparkline(m.cpuBuffer.Values(), sparklineWidth, 0, 100, &t)
		}

		line += "  " + freqStyle.Render(fmt.Sprintf("%dMHz", c.AverageFrequency))
		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("CPU: ")+valueStyle.Render("N/A"))
	}

	// Line 4: RAM gauge
	if m.cpuMetrics != nil {
		c := m.cpuMetrics
		ramUsedGB := float64(c.RAMUsedMB) / 1024.0
		ramTotalGB := float64(c.RAMTotalMB) / 1024.0
		gauge := RenderGauge(float64(c.RAMUsedMB), 0, float64(c.RAMTotalMB), gaugeWidth, &t)
		line := labelStyle.Render("RAM: ") +
			gauge +
			"  " +
			valueStyle.Render(fmt.Sprintf("%.1f/%.1f GB", ramUsedGB, ramTotalGB))
		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("RAM: ")+valueStyle.Render("N/A"))
	}

	// Calculate widths for layout spacing.
	metricsWidth := 0
	for _, line := range metricsLines {
		w := lipgloss.Width(line)
		if w > metricsWidth {
			metricsWidth = w
		}
	}

	// Build combined lines
	var lines []string
	numLines := max(len(logo), len(metricsLines))

	spacing := max(m.width-logoWidth-metricsWidth-4, 2)
	spacer := strings.Repeat(" ", spacing)

	for i := range numLines {
		var logoLine, metricsLine string

		if i < len(logo) {
			logoLine = logoStyle.Render(logo[i])
		} else {
			logoLine = strings.Repeat(" ", logoWidth)
		}

		if i < len(metricsLines) {
			metricsLine = metricsLines[i]
		}

		lines = append(lines, logoLine+spacer+metricsLine)
	}

	content := strings.Join(lines, "\n")

	headerStyle := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(t.Border)

	return headerStyle.Render(content)
}

func (m HeaderModel) GPUMetrics() *gpu.GPUMetrics { return m.gpuMetrics }
func (m HeaderModel) CPUMetrics() *cpu.CPUMetrics { return m.cpuMetrics }

func highestSeverityAlert(alerts []overlay.Alert) *overlay.Alert {
	var highest *overlay.Alert
	for i := range alerts {
		if highest == nil || alerts[i].Severity > highest.Severity {
			highest = &alerts[i]
		}
	}
	return highest
}

func alertLabel(a *overlay.Alert) string {
	switch a.Type {
	case overlay.AlertThermalThrottle:
		if a.Severity == overlay.AlertCritical {
			return "Throttling"
		}
		return "High temp"
	case overlay.AlertPowerLimit:
		return "Power limited"
	case overlay.AlertFanMaximum:
		return "Fan max"
	default:
		return ""
	}
}
