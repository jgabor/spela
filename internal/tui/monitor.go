package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
)

type MonitorModel struct {
	gpuMetrics *gpu.GPUMetrics
	cpuMetrics *cpu.CPUMetrics
	gpuName    string
	cpuName    string
	err        error
	width      int
	height     int
}

type metricsTickMsg struct{}

func NewMonitor() MonitorModel {
	m := MonitorModel{}

	gpuInfo, err := gpu.GetGPUInfo()
	if err == nil {
		m.gpuName = gpuInfo["name"]
	}

	cpuInfo, err := cpu.GetCPUInfo()
	if err == nil {
		m.cpuName = cpuInfo["model"]
	}

	m.refreshMetrics()
	return m
}

func (m *MonitorModel) refreshMetrics() {
	m.gpuMetrics, _ = gpu.GetGPUMetrics()
	m.cpuMetrics, _ = cpu.GetCPUMetrics()
}

func (m MonitorModel) Init() tea.Cmd {
	return tickMetrics()
}

func tickMetrics() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return metricsTickMsg{}
	})
}

func (m MonitorModel) Update(msg tea.Msg) (MonitorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case metricsTickMsg:
		m.refreshMetrics()
		return m, tickMetrics()
	}

	return m, nil
}

func (m MonitorModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("System monitor"))
	b.WriteString("\n\n")

	b.WriteString(dlssStyle.Render("GPU"))
	b.WriteString("\n")

	if m.gpuName != "" {
		b.WriteString(dimStyle.Render(m.gpuName))
		b.WriteString("\n")
	}

	if m.gpuMetrics != nil {
		g := m.gpuMetrics

		b.WriteString(fmt.Sprintf("  Temperature:   %d°C\n", g.Temperature))
		b.WriteString(fmt.Sprintf("  Utilization:   %d%%\n", g.Utilization))
		b.WriteString(fmt.Sprintf("  Power:         %.0f / %.0f W\n", g.PowerDraw, g.PowerLimit))
		b.WriteString(fmt.Sprintf("  Memory:        %d / %d MB\n", g.MemoryUsed, g.MemoryTotal))
		b.WriteString(fmt.Sprintf("  Graphics clk:  %d MHz\n", g.GraphicsClock))
		b.WriteString(fmt.Sprintf("  Memory clk:    %d MHz\n", g.MemoryClock))
	} else {
		b.WriteString(dimStyle.Render("  No GPU metrics available"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dlssStyle.Render("CPU"))
	b.WriteString("\n")

	if m.cpuName != "" {
		b.WriteString(dimStyle.Render(m.cpuName))
		b.WriteString("\n")
	}

	if m.cpuMetrics != nil {
		c := m.cpuMetrics

		b.WriteString(fmt.Sprintf("  Avg frequency: %d MHz\n", c.AverageFrequency))
		b.WriteString(fmt.Sprintf("  Governor:      %s\n", c.Governor))

		smtStatus := "disabled"
		if c.SMTEnabled {
			smtStatus = "enabled"
		}
		b.WriteString(fmt.Sprintf("  SMT:           %s\n", smtStatus))
	} else {
		b.WriteString(dimStyle.Render("  No CPU metrics available"))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n\nesc back • q quit"))

	return b.String()
}
