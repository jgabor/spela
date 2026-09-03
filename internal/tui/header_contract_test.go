package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/overlay"
)

func TestHeaderAndMonitorRenderLiveUnavailableAndAlertStates(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	header := NewHeader(styles)
	header.SetWidth(120)
	if unavailable := stripANSI(header.View()); !strings.Contains(unavailable, "GPU: N/A") || !strings.Contains(unavailable, "CPU: N/A") {
		t.Fatalf("unavailable header state changed:\n%s", unavailable)
	}

	gpuMetrics := &gpu.GPUMetrics{
		Temperature: 92, PowerDraw: 275, PowerLimit: 300, Utilization: 98,
		MemoryUsed: 8192, MemoryTotal: 12288, GraphicsClock: 2800,
		MemoryClock: 10500, FanSpeed: 100,
		ThrottleReasons: &gpu.ThrottleReasons{ThermalHardware: true, PowerCap: true},
	}
	cpuMetrics := &cpu.CPUMetrics{
		Frequencies: []int{4200, 4300}, AverageFrequency: 4250,
		Utilization: 64.5, Governor: cpu.Governor("performance"), SMTEnabled: true,
		RAMUsedMB: 16384, RAMTotalMB: 32768,
	}
	alerts := []overlay.Alert{
		{Type: overlay.AlertPowerLimit, Severity: overlay.AlertWarning},
		{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertCritical},
		{Type: overlay.AlertFanMaximum, Severity: overlay.AlertWarning},
	}
	header, command := header.Update(metricsMsg{gpuMetrics: gpuMetrics, cpuMetrics: cpuMetrics, alerts: alerts})
	if command == nil || header.GPUMetrics() != gpuMetrics || header.CPUMetrics() != cpuMetrics {
		t.Fatal("metrics update did not schedule the next sample or retain the snapshot")
	}
	header.SetWidth(120)
	view := stripANSI(header.View())
	for _, fragment := range []string{"92°C", "98%", "275W", "4250MHz", "Throttling"} {
		if !strings.Contains(view, fragment) {
			t.Errorf("full header missing %q:\n%s", fragment, view)
		}
	}
	for _, width := range []int{80, 75, 50} {
		header.SetWidth(width)
		view := header.View()
		if got := lipgloss.Width(view); got > width {
			t.Errorf("header width %d rendered at %d", width, got)
		}
		if got := lipgloss.Height(view); got != headerHeight {
			t.Errorf("header width %d rendered %d lines", width, got)
		}
	}
	compact := stripANSI(header.ViewCompact())
	for _, fragment := range []string{"VRAM", "RAM", "Throttling"} {
		if !strings.Contains(compact, fragment) {
			t.Errorf("compact header missing %q:\n%s", fragment, compact)
		}
	}
	if next, command := header.Update(headerTickMsg{}); command == nil || next.GPUMetrics() != gpuMetrics {
		t.Fatal("header tick did not preserve state and request a sample")
	}

	temperature, utilization, power, cpuBuffer := NewMetricsBuffer(4), NewMetricsBuffer(4), NewMetricsBuffer(4), NewMetricsBuffer(4)
	for _, sample := range []float64{40, 55, 75, 92} {
		temperature.Push(sample)
		utilization.Push(sample)
		power.Push(sample * 3)
		cpuBuffer.Push(sample / 2)
	}
	monitor := NewMetricsResource(styles).SetData(gpuMetrics, cpuMetrics, alerts, temperature, utilization, power, cpuBuffer)
	for _, width := range []int{80, 24} {
		monitor.SetSize(width, 30)
		for section, heading := range []string{"GPU", "CPU", "Alerts"} {
			view := stripANSI(monitor.View(true, nav.MonitorSection(section)))
			if !strings.Contains(view, heading) {
				t.Errorf("monitor section %d missing %q:\n%s", section, heading, view)
			}
		}
	}
	empty := NewMetricsResource(styles)
	for section, fragment := range []string{"Loading metrics", "Loading metrics", "No active alerts"} {
		if view := stripANSI(empty.View(false, nav.MonitorSection(section))); !strings.Contains(view, fragment) {
			t.Errorf("empty monitor section %d missing %q:\n%s", section, fragment, view)
		}
	}
}

func TestHeaderSuccessiveSamplesKeepMetricColumnAndReplaceValues(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	for _, width := range []int{80, 120} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			header := NewHeader(styles)
			header.SetWidth(width)
			samples := []metricsMsg{
				{
					gpuMetrics: &gpu.GPUMetrics{Temperature: 100, PowerDraw: 307, PowerLimit: 300, Utilization: 100, MemoryUsed: 8192, MemoryTotal: 12288},
					cpuMetrics: &cpu.CPUMetrics{AverageFrequency: 5365, Utilization: 100, RAMUsedMB: 16384, RAMTotalMB: 32768},
					alerts:     []overlay.Alert{{Type: overlay.AlertPowerLimit, Severity: overlay.AlertWarning}},
				},
				{
					gpuMetrics: &gpu.GPUMetrics{Temperature: 9, PowerDraw: 48, PowerLimit: 300, Utilization: 9, MemoryUsed: 512, MemoryTotal: 12288},
					cpuMetrics: &cpu.CPUMetrics{AverageFrequency: 842, Utilization: 9, RAMUsedMB: 1024, RAMTotalMB: 32768},
				},
				{
					gpuMetrics: &gpu.GPUMetrics{Temperature: 105, PowerDraw: 1234, PowerLimit: 300, Utilization: 100, MemoryUsed: 10240, MemoryTotal: 12288},
					cpuMetrics: &cpu.CPUMetrics{AverageFrequency: 12345, Utilization: 100, RAMUsedMB: 24576, RAMTotalMB: 32768},
				},
			}
			frequencies := []string{"5365MHz", "842MHz", "12345MHz"}
			cpuColumn := -1
			for index, sample := range samples {
				header, _ = header.Update(sample)
				view := stripANSI(header.View())
				line := lineContaining(view, "CPU:")
				if line == "" || !strings.Contains(line, frequencies[index]) {
					t.Fatalf("sample %d CPU line = %q", index, line)
				}
				for previous := range index {
					if strings.Contains(view, frequencies[previous]) {
						t.Fatalf("sample %d retained %q:\n%s", index, frequencies[previous], view)
					}
				}
				if column := strings.Index(line, "CPU:"); cpuColumn < 0 {
					cpuColumn = column
				} else if column != cpuColumn {
					t.Fatalf("sample %d moved CPU column from %d to %d:\n%s", index, cpuColumn, column, view)
				}
				for _, renderedLine := range strings.Split(view, "\n") {
					if got := lipgloss.Width(renderedLine); got != width {
						t.Fatalf("sample %d line width = %d, want %d: %q", index, got, width, renderedLine)
					}
				}
			}
		})
	}
}

func lineContaining(view, fragment string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, fragment) {
			return line
		}
	}
	return ""
}

func TestMonitorUsesTextMarkersAndDistinctMetricStates(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	for _, state := range []metricState{metricLoading, metricUnsupported, metricFailed, metricStale} {
		monitor := NewMetricsResource(styles).SetStates(state, state)
		view := stripANSI(monitor.View(true, nav.MonitorGPU))
		if !strings.Contains(view, "› GPU") || !strings.Contains(view, string(state)) {
			t.Fatalf("monitor state %q was not explicit:\n%s", state, view)
		}
		if strings.Count(view, "GPU") != 1 || strings.Contains(view, "header sample loop") {
			t.Fatalf("monitor repeated its heading or exposed developer copy:\n%s", view)
		}
	}
}

func TestMonitorAlertNamesCauseAndRecovery(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	alert := overlay.Alert{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertCritical, Message: "GPU thermal throttling at 92°C", Suggestion: "Reduce the GPU power limit"}
	view := stripANSI(NewMetricsResource(styles).SetData(nil, nil, []overlay.Alert{alert}, nil, nil, nil, nil).View(true, nav.MonitorAlerts))
	for _, text := range []string{"› Alerts", "GPU thermal throttling at 92°C", "Recovery: Reduce the GPU power limit"} {
		if !strings.Contains(view, text) {
			t.Fatalf("alert missing %q:\n%s", text, view)
		}
	}
}

func TestAlertLabelsCoverSupportedAlertKinds(t *testing.T) {
	tests := []struct {
		alert overlay.Alert
		want  string
	}{
		{overlay.Alert{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertWarning}, "High temp"},
		{overlay.Alert{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertCritical}, "Throttling"},
		{overlay.Alert{Type: overlay.AlertPowerLimit}, "Power limited"},
		{overlay.Alert{Type: overlay.AlertFanMaximum}, "Fan max"},
		{overlay.Alert{Type: overlay.AlertType(99)}, ""},
	}
	for _, test := range tests {
		if got := alertLabel(&test.alert); got != test.want {
			t.Errorf("alertLabel(%+v) = %q, want %q", test.alert, got, test.want)
		}
	}
	if highestSeverityAlert(nil) != nil {
		t.Fatal("empty alerts should not have a highest severity")
	}
}
