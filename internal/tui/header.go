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
}

func NewHeader(styles *Styles) HeaderModel {
	return HeaderModel{styles: styles}
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

	// Build metrics lines
	var metricsLines []string

	// Line 1: GPU temp, util, power (with alert coloring)
	if m.gpuMetrics != nil {
		g := m.gpuMetrics

		tempStyle := valueStyle
		powerStyle := valueStyle
		for _, a := range m.alerts {
			if a.Type == overlay.AlertThermalThrottle {
				if a.Severity == overlay.AlertCritical {
					tempStyle = lipgloss.NewStyle().Foreground(t.Error)
				} else {
					tempStyle = lipgloss.NewStyle().Foreground(t.Warning)
				}
			}
			if a.Type == overlay.AlertPowerLimit {
				powerStyle = lipgloss.NewStyle().Foreground(t.Warning)
			}
		}

		line := labelStyle.Render("GPU: ") +
			tempStyle.Render(fmt.Sprintf("%d°C", g.Temperature)) +
			valueStyle.Render(fmt.Sprintf(" %d%% ", g.Utilization)) +
			powerStyle.Render(fmt.Sprintf("%.0fW", g.PowerDraw))

		if g.FanSpeed > 0 {
			line += valueStyle.Render(fmt.Sprintf(" %d%%fan", g.FanSpeed))
		}

		if highest := highestSeverityAlert(m.alerts); highest != nil {
			var alertStyle lipgloss.Style
			var icon string
			switch highest.Severity {
			case overlay.AlertCritical:
				alertStyle = lipgloss.NewStyle().Foreground(t.Error)
				icon = "✗"
			case overlay.AlertWarning:
				alertStyle = lipgloss.NewStyle().Foreground(t.Warning)
				icon = "⚠"
			}
			if icon != "" {
				line += " " + alertStyle.Render(icon+" "+alertLabel(highest))
			}
		}

		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("GPU: ")+valueStyle.Render("N/A"))
	}

	// Line 2: VRAM
	if m.gpuMetrics != nil {
		g := m.gpuMetrics
		vramUsedGB := float64(g.MemoryUsed) / 1024.0
		vramTotalGB := float64(g.MemoryTotal) / 1024.0
		line := labelStyle.Render("VRAM: ") + valueStyle.Render(fmt.Sprintf("%.1f/%.1f GB", vramUsedGB, vramTotalGB))
		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("VRAM: ")+valueStyle.Render("N/A"))
	}

	// Line 3: CPU util and freq
	if m.cpuMetrics != nil {
		c := m.cpuMetrics
		line := labelStyle.Render("CPU: ") + valueStyle.Render(fmt.Sprintf("%.0f%% %dMHz", c.Utilization, c.AverageFrequency))
		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("CPU: ")+valueStyle.Render("N/A"))
	}

	// Line 4: RAM
	if m.cpuMetrics != nil {
		c := m.cpuMetrics
		ramUsedGB := float64(c.RAMUsedMB) / 1024.0
		ramTotalGB := float64(c.RAMTotalMB) / 1024.0
		line := labelStyle.Render("RAM: ") + valueStyle.Render(fmt.Sprintf("%.1f/%.1f GB", ramUsedGB, ramTotalGB))
		metricsLines = append(metricsLines, line)
	} else {
		metricsLines = append(metricsLines, labelStyle.Render("RAM: ")+valueStyle.Render("N/A"))
	}

	// Calculate widths
	logoWidth := lipgloss.Width(logo[0])
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

// ViewCompact renders a condensed 2-line header with metrics only (no logo).
func (m HeaderModel) ViewCompact() string {
	t := m.styles.Theme
	labelStyle := lipgloss.NewStyle().Foreground(t.TextDim)
	valueStyle := lipgloss.NewStyle().Foreground(t.Text)

	var lines []string

	// Line 1: GPU temp util power | CPU util freq
	var line1 strings.Builder
	if m.gpuMetrics != nil {
		g := m.gpuMetrics
		tempStyle := m.alertStyleFor(overlay.AlertThermalThrottle, valueStyle)
		powerStyle := m.alertStyleFor(overlay.AlertPowerLimit, valueStyle)

		line1.WriteString(labelStyle.Render("GPU "))
		line1.WriteString(tempStyle.Render(fmt.Sprintf("%d\u00b0C", g.Temperature)))
		line1.WriteString(valueStyle.Render(fmt.Sprintf(" %d%% ", g.Utilization)))
		line1.WriteString(powerStyle.Render(fmt.Sprintf("%.0fW", g.PowerDraw)))
	} else {
		line1.WriteString(labelStyle.Render("GPU ") + valueStyle.Render("N/A"))
	}
	line1.WriteString("  ")
	if m.cpuMetrics != nil {
		c := m.cpuMetrics
		line1.WriteString(labelStyle.Render("CPU "))
		line1.WriteString(valueStyle.Render(fmt.Sprintf("%.0f%% %dMHz", c.Utilization, c.AverageFrequency)))
	} else {
		line1.WriteString(labelStyle.Render("CPU ") + valueStyle.Render("N/A"))
	}
	lines = append(lines, line1.String())

	// Line 2: VRAM used/total | RAM used/total | alerts
	var line2 strings.Builder
	if m.gpuMetrics != nil {
		g := m.gpuMetrics
		vramUsed := float64(g.MemoryUsed) / 1024.0
		vramTotal := float64(g.MemoryTotal) / 1024.0
		line2.WriteString(labelStyle.Render("VRAM "))
		line2.WriteString(valueStyle.Render(fmt.Sprintf("%.1f/%.1fG", vramUsed, vramTotal)))
	}
	line2.WriteString("  ")
	if m.cpuMetrics != nil {
		c := m.cpuMetrics
		ramUsed := float64(c.RAMUsedMB) / 1024.0
		ramTotal := float64(c.RAMTotalMB) / 1024.0
		line2.WriteString(labelStyle.Render("RAM "))
		line2.WriteString(valueStyle.Render(fmt.Sprintf("%.1f/%.1fG", ramUsed, ramTotal)))
	}
	if highest := highestSeverityAlert(m.alerts); highest != nil {
		var icon string
		var alertStyle lipgloss.Style
		switch highest.Severity {
		case overlay.AlertCritical:
			icon = "\u2717"
			alertStyle = lipgloss.NewStyle().Foreground(t.Error)
		case overlay.AlertWarning:
			icon = "\u26a0"
			alertStyle = lipgloss.NewStyle().Foreground(t.Warning)
		}
		if icon != "" {
			line2.WriteString("  " + alertStyle.Render(icon+" "+alertLabel(highest)))
		}
	}
	lines = append(lines, line2.String())

	content := strings.Join(lines, "\n")

	headerStyle := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(t.Border)

	return headerStyle.Render(content)
}

// alertStyleFor returns a styled variant if any alert of the given type exists, else the fallback.
func (m HeaderModel) alertStyleFor(alertType overlay.AlertType, fallback lipgloss.Style) lipgloss.Style {
	t := m.styles.Theme
	for _, a := range m.alerts {
		if a.Type == alertType {
			if a.Severity == overlay.AlertCritical {
				return lipgloss.NewStyle().Foreground(t.Error)
			}
			return lipgloss.NewStyle().Foreground(t.Warning)
		}
	}
	return fallback
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
