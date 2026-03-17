package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
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
}

type HeaderModel struct {
	gpuMetrics *gpu.GPUMetrics
	cpuMetrics *cpu.CPUMetrics
	width      int
}

func NewHeader() HeaderModel {
	return HeaderModel{}
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
		return metricsMsg{gpuMetrics: gpuMetrics, cpuMetrics: cpuMetrics}
	}
}

func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case headerTickMsg:
		return m, fetchMetrics()
	case metricsMsg:
		m.gpuMetrics = msg.gpuMetrics
		m.cpuMetrics = msg.cpuMetrics
		return m, tickHeader()
	}
	return m, nil
}

func (m HeaderModel) View() string {
	t := GetTheme()

	logoStyle := lipgloss.NewStyle().
		Foreground(t.Primary)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.TextDim)

	valueStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	// Build metrics lines
	var metricsLines []string

	// Line 1: GPU temp, util, power
	if m.gpuMetrics != nil {
		g := m.gpuMetrics
		line := labelStyle.Render("GPU: ") + valueStyle.Render(fmt.Sprintf("%d°C %d%% %.0fW", g.Temperature, g.Utilization, g.PowerDraw))
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

func (m HeaderModel) GPUMetrics() *gpu.GPUMetrics { return m.gpuMetrics }
func (m HeaderModel) CPUMetrics() *cpu.CPUMetrics { return m.cpuMetrics }
